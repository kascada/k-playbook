package main

import (
	"bytes"
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
		field := c.field
		if got := briefingGitHubLine(&field); got != c.want {
			t.Errorf("%s:\n%s\nerwartet\n%s", name, got, c.want)
		}
	}
	if got := briefingGitHubLine(nil); got != "" {
		t.Errorf("ohne Objekt: %q", got)
	}
}

// Im Klartext steht die Zeile nach Projektdoku und vor dem Hinweis auf den
// Teilabruf; im JSON des Briefings stehen nur die Dienste dieses Repos
// (github.projectServices), das volle Objekt samt services liefert der
// Teilabruf environment. Weder Klartext noch JSON — auch nicht der Teilabruf —
// tragen einen Token-Wert — nicht aus dem Remote, nicht aus der Token-Datei —,
// und repo ist trotzdem gefüllt.
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
	partText, err := runBriefingCaptured(t, "--part", "environment")
	if err != nil {
		t.Fatalf("briefing --part environment: %v", err)
	}
	partJSON, err := runBriefingCaptured(t, "--part", "environment", "--json")
	if err != nil {
		t.Fatalf("briefing --part environment --json: %v", err)
	}
	for _, output := range []string{text, asJSON, partText, partJSON} {
		for _, secret := range []string{"ghp_remotegeheim", "x-access-token", "ghp_dateigeheim"} {
			if strings.Contains(output, secret) {
				t.Errorf("Ausgabe trägt %s:\n%s", secret, output)
			}
		}
	}
	line := "Projektdoku: keph:/docs/\nGitHub: dieses Repo = Dienst k-playbook — run: git über token " + tokenFile + ", api über token\n" + briefingEnvironmentHint
	if !strings.HasSuffix(text, line) {
		t.Errorf("Zeile fehlt oder steht falsch:\n%s", text)
	}
	var briefing project.Briefing
	if err := json.Unmarshal([]byte(asJSON), &briefing); err != nil {
		t.Fatal(err)
	}
	if len(briefing.GitHub.ProjectServices) != 1 || briefing.GitHub.ProjectServices[0] != "k-playbook" {
		t.Errorf("github.projectServices: %+v", briefing.GitHub)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(asJSON), &raw); err != nil {
		t.Fatal(err)
	}
	if string(compactJSON(t, raw["github"])) != `{"projectServices":["k-playbook"]}` {
		t.Errorf("github im JSON des Briefings: %s", raw["github"])
	}
	field := environmentOf(t, partJSON).GitHub
	if field == nil || field.Repo != "kascada/k-playbook" || field.Remote != "https://github.com/kascada/k-playbook.git" ||
		len(field.ProjectServices) != 1 || len(field.Services) != 1 || field.Services[0].Selected == nil {
		t.Errorf("environment.github: %+v", field)
	}
	if !strings.Contains(partText, "\n  github:\n") || !strings.Contains(partText, "    repo: kascada/k-playbook\n") {
		t.Errorf("github im Klartext des Teilabrufs:\n%s", partText)
	}
}

// compactJSON nimmt Einrückung und Umbrüche aus einem JSON-Wert.
func compactJSON(t *testing.T, raw json.RawMessage) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := json.Compact(&b, raw); err != nil {
		t.Fatalf("kein JSON: %v\n%s", err, raw)
	}
	return b.Bytes()
}

// environmentOf liest die JSON-Ausgabe des Teilabrufs environment.
func environmentOf(t *testing.T, output string) project.BriefingEnvironment {
	t.Helper()
	var decoded briefingEnvironmentOutput
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatalf("Teilabruf ist kein JSON: %v\n%s", err, output)
	}
	return decoded.Environment
}

// Fehlt die Zugangsdatei, legt das Briefing nichts an — weder der Abschnitt
// github noch die Umgebungsprüfung desselben Briefings, auch nicht der
// Teilabruf environment: Keine Datei und kein Verzeichnis entstehen, die
// Umgebung nennt nichts im Bereich github, und die Zeile sagt „GitHub: keine
// Zugangsdatei“; im Teilabruf ist present falsch. Ein symbolischer Link ohne Ziel an der Stelle der Datei
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
	if len(briefing.GitHub.ProjectServices) != 0 {
		t.Errorf("github.projectServices ohne Datei: %+v", briefing.GitHub)
	}
	partJSON, err := runBriefingCaptured(t, "--part", "environment", "--json")
	if err != nil {
		t.Fatalf("briefing --part environment --json: %v", err)
	}
	field := environmentOf(t, partJSON).GitHub
	if field == nil || field.Present || field.File != config || len(field.Problems) != 0 || len(field.Services) != 0 || field.Repo != "kascada/k-playbook" {
		t.Errorf("environment.github ohne Datei: %+v", field)
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
