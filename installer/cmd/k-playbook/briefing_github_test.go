package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/githubaccess"
	"github.com/kascada/k-playbook/installer/internal/health"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// githubSection baut einen Rest, der nur context.github trägt.
func githubSection(t *testing.T, field project.ContextGitHub) project.BriefingContext {
	t.Helper()
	raw, err := json.Marshal(field)
	if err != nil {
		t.Fatal(err)
	}
	return project.BriefingContext{Available: true, Fields: []project.BriefingContextField{{Name: "github", Value: raw}}}
}

func TestBriefingGitHubLine(t *testing.T) {
	zero, one := 0, 1
	selected := &githubaccess.Selection{
		Read:  githubaccess.ChannelIndex{Git: &zero, API: &one},
		Write: githubaccess.ChannelIndex{Git: &zero, API: &one},
	}
	services := githubaccess.Services{
		{Name: "k-playbook", Repo: "kascada/k-playbook", Selected: selected, Access: []githubaccess.Access{
			{Type: "ssh", Host: "github-kamranbycloud", State: githubaccess.StateAvailable},
			{Type: "gh", Login: "KamranByCloud", State: githubaccess.StateAvailable},
		}},
		{Name: "spiegel", Repo: "kascada/k-playbook", Selected: &githubaccess.Selection{
			Read:  githubaccess.ChannelIndex{Git: &zero},
			Write: githubaccess.ChannelIndex{},
		}, Access: []githubaccess.Access{{Type: "public", State: githubaccess.StateAvailable}}},
		{Name: "andere", Repo: "kascada/andere"},
	}
	base := project.ContextGitHub{File: "/x/github.yaml", Present: true, Problems: []githubaccess.Problem{}, Services: services,
		Remote: "git@github-kamranbycloud:kascada/k-playbook.git", Repo: "kascada/k-playbook"}

	cases := map[string]struct {
		field project.ContextGitHub
		want  string
	}{
		"ein Dienst": {func() project.ContextGitHub { f := base; f.ProjectServices = []string{"k-playbook"}; return f }(),
			"GitHub: dieses Repo = Dienst k-playbook — run: git über ssh host github-kamranbycloud, api über gh KamranByCloud"},
		"zwei Dienste": {func() project.ContextGitHub {
			f := base
			f.ProjectServices = []string{"k-playbook", "spiegel"}
			return f
		}(),
			"GitHub: dieses Repo = Dienste k-playbook, spiegel — k-playbook run: git über ssh host github-kamranbycloud, api über gh KamranByCloud; spiegel run: git über public, api: keiner; mit --write: kein Zugang"},
		"kein Dienst": {func() project.ContextGitHub {
			f := base
			f.Repo, f.ProjectServices = "kascada/foo", []string{}
			return f
		}(), "GitHub: kein Dienst für kascada/foo (Dienste: k-playbook, spiegel, andere)"},
		"keine Datei": {project.ContextGitHub{File: "/x/github.yaml", Repo: "kascada/k-playbook"}, "GitHub: keine Zugangsdatei"},
		"kein Remote": {project.ContextGitHub{File: "/x/github.yaml", Present: true}, "GitHub: kein GitHub-Remote erkannt (Dienste: keine)"},
		"Datei kaputt": {project.ContextGitHub{File: "/x/github.yaml", Present: true, Problems: []githubaccess.Problem{{Level: githubaccess.LevelError, Path: "file", Message: "kaputt"}}},
			"GitHub: die Zugangsdatei hat Fehler — k-playbook github check nennt sie"},
	}
	for name, c := range cases {
		if got := briefingGitHubLine(githubSection(t, c.field)); got != c.want {
			t.Errorf("%s:\n%s\nerwartet\n%s", name, got, c.want)
		}
	}
	if got := briefingGitHubLine(project.BriefingContext{Available: true}); got != "" {
		t.Errorf("ohne Feld: %q", got)
	}
}

// Im Klartext steht die Zeile nach Projektdoku und vor der Trennzeile, im
// Rest github ohne services; weder Klartext noch JSON tragen einen
// Token-Wert — nicht aus dem Remote, nicht aus der Token-Datei —, und repo
// ist trotzdem gefüllt.
func TestBriefingGitHubOhneTokenWert(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git nicht verfügbar")
	}
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	root := briefingProject(t)
	run := func(args ...string) {
		t.Helper()
		command := exec.Command("git", args...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}
	run("init", "-b", "main")
	run("remote", "add", "origin", "https://x-access-token:ghp_remotegeheim@github.com/kascada/k-playbook.git")
	tokenFile := filepath.Join(t.TempDir(), "x.token")
	if err := os.WriteFile(tokenFile, []byte("ghp_dateigeheim\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(t.TempDir(), githubaccess.FileName)
	if err := os.WriteFile(config, []byte("services:\n  k-playbook:\n    repo: kascada/k-playbook\n    access:\n      - type: token\n        file: "+tokenFile+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(githubaccess.EnvConfig, config)

	text, err := runBriefingCaptured(t)
	if err != nil {
		t.Fatalf("briefing: %v", err)
	}
	asJSON, err := runBriefingCaptured(t, "--json")
	if err != nil {
		t.Fatalf("briefing --json: %v", err)
	}
	for _, output := range []string{text, asJSON} {
		for _, secret := range []string{"ghp_remotegeheim", "x-access-token", "ghp_dateigeheim"} {
			if strings.Contains(output, secret) {
				t.Errorf("Ausgabe trägt %s:\n%s", secret, output)
			}
		}
	}
	line := "Projektdoku: keph:/docs/\nGitHub: dieses Repo = Dienst k-playbook — run: git über token " + tokenFile + ", api über token\n\n" + briefingContextSeparator
	if !strings.Contains(text, line) {
		t.Errorf("Zeile fehlt oder steht falsch:\n%s", text)
	}
	_, rest, _ := strings.Cut(text, briefingContextSeparator)
	if !strings.Contains(rest, "\n  github:\n") || !strings.Contains(rest, "    repo: kascada/k-playbook\n") || strings.Contains(rest, "services") {
		t.Errorf("github im Rest:\n%s", rest)
	}
	var briefing project.Briefing
	if err := json.Unmarshal([]byte(asJSON), &briefing); err != nil {
		t.Fatal(err)
	}
	raw, _ := briefing.Context.Field("github")
	var field project.ContextGitHub
	if err := json.Unmarshal(raw, &field); err != nil {
		t.Fatalf("context.github: %v\n%s", err, raw)
	}
	if field.Repo != "kascada/k-playbook" || field.Remote != "https://github.com/kascada/k-playbook.git" ||
		len(field.ProjectServices) != 1 || len(field.Services) != 1 || field.Services[0].Selected == nil {
		t.Errorf("context.github: %s", raw)
	}
}

// Fehlt die Zugangsdatei, legt das Briefing nichts an — weder das Feld
// context.github noch die Umgebungsprüfung desselben Briefings: Keine Datei
// und kein Verzeichnis entstehen, die Umgebung nennt nichts im Bereich
// github, und die Zeile sagt „GitHub: keine Zugangsdatei“; im JSON ist
// present falsch. Ein symbolischer Link ohne Ziel an der Stelle der Datei
// ist dagegen belegt: Die Zeile nennt Fehler, und die Umgebungsprüfung
// meldet einen error; auch dort entsteht nichts.
func TestBriefingLegtBeiFehlenderZugangsdateiNichtsAn(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git nicht verfügbar")
	}
	root := briefingProject(t)
	for _, args := range [][]string{{"init", "-b", "main"}, {"remote", "add", "origin", "https://github.com/kascada/k-playbook.git"}} {
		command := exec.Command("git", args...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}
	dir := filepath.Join(t.TempDir(), "fehlt")
	config := filepath.Join(dir, githubaccess.FileName)
	t.Setenv(githubaccess.EnvConfig, config)

	text, err := runBriefingCaptured(t)
	if err != nil {
		t.Fatalf("briefing: %v", err)
	}
	if _, err := os.Lstat(dir); !os.IsNotExist(err) {
		t.Fatalf("Verzeichnis der Zugangsdatei angelegt: %v", err)
	}
	if !strings.Contains(text, "\nGitHub: keine Zugangsdatei\n") {
		t.Errorf("Zeile ohne Zugangsdatei:\n%s", text)
	}
	if strings.Contains(text, "] github: ") || strings.Contains(text, "angelegt") {
		t.Errorf("Umgebung nennt die fehlende Zugangsdatei:\n%s", text)
	}

	asJSON, err := runBriefingCaptured(t, "--json")
	if err != nil {
		t.Fatalf("briefing --json: %v", err)
	}
	var briefing project.Briefing
	if err := json.Unmarshal([]byte(asJSON), &briefing); err != nil {
		t.Fatal(err)
	}
	for _, problem := range briefing.Health.Problems {
		if problem.Area == health.AreaGitHub {
			t.Errorf("Umgebung mit Eintrag github: %+v", problem)
		}
	}
	raw, _ := briefing.Context.Field("github")
	var field project.ContextGitHub
	if err := json.Unmarshal(raw, &field); err != nil {
		t.Fatalf("context.github: %v\n%s", err, raw)
	}
	if field.Present || field.File != config || len(field.Problems) != 0 || len(field.Services) != 0 || field.Repo != "kascada/k-playbook" {
		t.Errorf("context.github ohne Datei: %s", raw)
	}
	if _, err := os.Lstat(dir); !os.IsNotExist(err) {
		t.Fatalf("Verzeichnis der Zugangsdatei angelegt: %v", err)
	}

	// Symlink ohne Ziel: belegt, nicht fehlend.
	linkDir := t.TempDir()
	link := filepath.Join(linkDir, githubaccess.FileName)
	target := filepath.Join(linkDir, "ziel", githubaccess.FileName)
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("kein Symlink möglich: %v", err)
	}
	t.Setenv(githubaccess.EnvConfig, link)
	text, err = runBriefingCaptured(t)
	if err != nil {
		t.Fatalf("briefing: %v", err)
	}
	if !strings.Contains(text, "\nGitHub: die Zugangsdatei hat Fehler — k-playbook github check nennt sie\n") || !strings.Contains(text, "[error] github: file: Die Datei ist nicht lesbar: Der Pfad ist ein symbolischer Link") {
		t.Errorf("Symlink ohne Ziel:\n%s", text)
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		t.Errorf("Ziel des Links angelegt: %v", err)
	}
}
