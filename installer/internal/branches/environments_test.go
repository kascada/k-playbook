package branches

import (
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/github"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// deploymentFixture ist die Lage aus omni-gw, nachgebaut: stage trägt einen
// eigenen Commit, und das letzte Deployment nach prod stammt aus stage — der
// Name legt für prod dagegen den Default-Branch nahe.
func deploymentFixture(t *testing.T) (fixture, string, string) {
	t.Helper()
	f := newFixture(t)
	run(t, f.seed, "switch", "-q", "stage")
	commit(t, f.seed, "stage.txt", "stage\n", "Stand für stage")
	run(t, f.seed, "push", "-q", "origin", "stage")
	run(t, f.work, "fetch", "-q", "origin")
	stageSHA := run(t, f.work, "rev-parse", "origin/stage")
	mainSHA := run(t, f.work, "rev-parse", "origin/main")
	return f, stageSHA, mainSHA
}

func environmentByName(t *testing.T, listing Listing, name string) Environment {
	t.Helper()
	for _, env := range listing.Environments {
		if env.Name == name {
			return env
		}
	}
	t.Fatalf("Umgebung %s fehlt: %+v", name, listing.Environments)
	return Environment{}
}

func gitHubWith(deployments ...github.Deployment) GitHubData {
	return GitHubData{
		Enabled:      true,
		Result:       github.Result{State: github.StateOK},
		Environments: []string{"dev", "prod", "copilot"},
		Deployments:  deployments,
	}
}

// Ohne gh: nur Namensvorschläge, als Vorschlag gekennzeichnet und mit Quelle.
func TestUmgebungenAusNamen(t *testing.T) {
	f := newFixture(t)
	listing := List(testContext(t), f.options())
	var names []string
	for _, env := range listing.Environments {
		names = append(names, env.Name+"="+env.Branch+"/"+env.Source)
		if !env.Suggested {
			t.Errorf("%s ist nicht als Vorschlag gekennzeichnet", env.Name)
		}
	}
	if got := strings.Join(names, ","); got != "dev=development/name,stage=stage/name,prod=main/name" {
		t.Errorf("Umgebungen = %s", got)
	}
}

// Mit gh geht das Deployment in den Vorschlag ein. Ref ist ein Tag, der nicht
// gedeutet wird; entschieden wird am SHA. Weicht der Name ab, ist das ein Befund.
func TestUmgebungenAusDeployment(t *testing.T) {
	f, stageSHA, mainSHA := deploymentFixture(t)
	options := f.options()
	options.GitHub = gitHubWith(
		github.Deployment{Environment: "prod", Ref: stageSHA[:7] + "-stage", SHA: stageSHA, CreatedAt: "2026-09-10T15:13:15Z"},
		// Ref nennt einen vorhandenen Branch: dann ist es dieser.
		github.Deployment{Environment: "dev", Ref: "development", SHA: mainSHA, CreatedAt: "2026-09-16T18:08:15Z"},
	)
	listing := List(testContext(t), options)

	prod := environmentByName(t, listing, "prod")
	if prod.Branch != "stage" || prod.Source != SourceDeployment || !prod.Suggested || prod.NameSuggestion != "main" || !prod.OnGitHub {
		t.Errorf("prod = %+v", prod)
	}
	if prod.Deployment == nil || !prod.Deployment.Resolved || prod.Deployment.InBranch != "contained" || prod.Deployment.Ahead != 0 {
		t.Errorf("prod.Deployment = %+v", prod.Deployment)
	}
	if len(prod.Findings) != 1 || prod.Findings[0].Kind != "name-differs" {
		t.Errorf("prod.Findings = %+v", prod.Findings)
	}

	dev := environmentByName(t, listing, "dev")
	if dev.Branch != "development" || dev.Source != SourceDeployment {
		t.Errorf("dev = %+v", dev)
	}
	if dev.Deployment == nil || dev.Deployment.InBranch != "contained" || dev.Deployment.Ahead != 1 || !strings.Contains(dev.Deployment.Message, "1 Commit weiter") {
		t.Errorf("dev.Deployment = %+v", dev.Deployment)
	}

	copilot := environmentByName(t, listing, "copilot")
	if copilot.Branch != "" || copilot.Message == "" {
		t.Errorf("copilot = %+v, erwartet keine Zuordnung mit Satz", copilot)
	}

	// Die Umgebung steht auch an der Liste, als Vorschlag gekennzeichnet.
	stage, _ := findBranch(t, listing, "stage")
	var labels []string
	for _, label := range stage.Environments {
		labels = append(labels, label.Name+"/"+label.Source)
		if !label.Suggested {
			t.Errorf("stage trägt %s nicht als Vorschlag", label.Name)
		}
	}
	if strings.Join(labels, ",") != "stage/name,prod/deployment" {
		t.Errorf("stage.Environments = %v", labels)
	}
}

// Festgelegt ist prod: main, ausgerollt wurde aus stage. Das ist ein sichtbarer
// Befund, kein Fehler, und die Festlegung bleibt wirksam.
func TestUmgebungenAbweichungFestlegungDeployment(t *testing.T) {
	f, stageSHA, _ := deploymentFixture(t)
	options := f.options()
	options.Settings = project.GitSettings{Switch: project.GitSwitchOffer, Configured: true, Environments: []project.GitEnvironment{{Name: "prod", Branch: "main"}}}
	options.GitHub = gitHubWith(github.Deployment{Environment: "prod", Ref: stageSHA[:7] + "-stage", SHA: stageSHA})
	listing := List(testContext(t), options)

	prod := environmentByName(t, listing, "prod")
	if prod.Branch != "main" || prod.Source != SourceConfig || prod.Suggested || prod.DeploymentSuggestion != "stage" {
		t.Errorf("prod = %+v", prod)
	}
	kinds := []string{}
	for _, finding := range prod.Findings {
		kinds = append(kinds, finding.Kind)
	}
	if strings.Join(kinds, ",") != "deployment-differs,deployment-not-in-branch" {
		t.Errorf("Befunde = %v", prod.Findings)
	}
	if prod.Deployment == nil || prod.Deployment.InBranch != "not-contained" {
		t.Errorf("prod.Deployment = %+v", prod.Deployment)
	}
	// Die Festlegung steht als erste Umgebung.
	if listing.Environments[0].Name != "prod" {
		t.Errorf("Reihenfolge = %+v", listing.Environments)
	}
}

// Ein SHA, den das Repo nicht kennt, ist ein Satz und kein Fehler.
func TestUmgebungenDeploymentLokalUnbekannt(t *testing.T) {
	f := newFixture(t)
	options := f.options()
	options.GitHub = gitHubWith(github.Deployment{Environment: "prod", Ref: "v9", SHA: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"})
	listing := List(testContext(t), options)
	prod := environmentByName(t, listing, "prod")
	if prod.Source != SourceName || prod.Deployment == nil || prod.Deployment.Resolved || !strings.Contains(prod.Deployment.Message, "nicht vorhanden") {
		t.Errorf("prod = %+v / %+v", prod, prod.Deployment)
	}
}

// Ohne gh zählen mitgegebene Deployments nicht.
func TestUmgebungenOhneGHOhneDeployments(t *testing.T) {
	f, stageSHA, _ := deploymentFixture(t)
	options := f.options()
	data := gitHubWith(github.Deployment{Environment: "prod", SHA: stageSHA})
	data.Enabled = false
	options.GitHub = data
	listing := List(testContext(t), options)
	prod := environmentByName(t, listing, "prod")
	if prod.Source != SourceName || prod.Deployment != nil || prod.OnGitHub {
		t.Errorf("prod = %+v", prod)
	}
}

// Ein älteres Deployment steckt in mehreren langlebigen Branches. Enthält der
// Branch des Namens den Stand, gewinnt er — nicht der mit dem geringsten
// Abstand, in den nur zuletzt gemergt wurde.
func TestUmgebungenNameGewinntBeiMehrerenTraegern(t *testing.T) {
	f := newFixture(t)
	old := run(t, f.work, "rev-parse", "origin/main")
	// stage zieht weit voraus, main bekommt nur einen Commit.
	run(t, f.seed, "switch", "-q", "stage")
	for index := 0; index < 3; index++ {
		commit(t, f.seed, "stage.txt", strings.Repeat("s", index+1)+"\n", "stage weiter")
	}
	run(t, f.seed, "push", "-q", "origin", "stage")
	run(t, f.work, "fetch", "-q", "origin")

	options := f.options()
	options.GitHub = gitHubWith(github.Deployment{Environment: "stage", Ref: old[:7] + "-stage", SHA: old})
	listing := List(testContext(t), options)
	stage := environmentByName(t, listing, "stage")
	if stage.Branch != "stage" || stage.Source != SourceDeployment || len(stage.Findings) != 0 {
		t.Errorf("stage = %+v", stage)
	}
	if stage.Deployment == nil || stage.Deployment.Ahead != 3 {
		t.Errorf("stage.Deployment = %+v", stage.Deployment)
	}
}

// Festgelegte Umgebungen, die sich nur in der Schreibweise unterscheiden, legt
// die Liste zu einer zusammen. Der Parser weist das ab; kommt es trotzdem
// herein, zählen für die Ordnung die tatsächlich angelegten Einträge. Vorher
// schnitt list[configured:] an der Zahl der Festlegungen: mit vier Schreibweisen
// über das Ende der Liste hinaus, mit zweien an der falschen Stelle.
func TestUmgebungenDoppeltInAndererSchreibweise(t *testing.T) {
	tests := []struct {
		name   string
		names  []string
		github GitHubData
		want   string
	}{
		{
			name:  "mehr Festlegungen als Einträge",
			names: []string{"Prod", "prod", "PROD", "pRoD"},
			want:  "Prod=main/config,dev=development/name,stage=stage/name",
		},
		{
			name:   "Ordnung der übrigen",
			names:  []string{"Prod", "prod"},
			github: GitHubData{Enabled: true, Result: github.Result{State: github.StateOK}, Environments: []string{"zeta", "dev"}},
			want:   "Prod=main/config,dev=development/name,stage=stage/name,zeta=/",
		},
	}
	f := newFixture(t)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			options := f.options()
			options.GitHub = test.github
			options.Settings = project.GitSettings{Switch: project.GitSwitchOffer, Allow: []string{}}
			for _, name := range test.names {
				options.Settings.Environments = append(options.Settings.Environments, project.GitEnvironment{Name: name, Branch: "main"})
			}
			listing := List(testContext(t), options)
			if listing.State != StateOK {
				t.Fatalf("State = %q: %s", listing.State, listing.Message)
			}
			var names []string
			for _, env := range listing.Environments {
				names = append(names, env.Name+"="+env.Branch+"/"+env.Source)
			}
			if got := strings.Join(names, ","); got != test.want {
				t.Errorf("Umgebungen = %s, erwartet %s", got, test.want)
			}
		})
	}
}
