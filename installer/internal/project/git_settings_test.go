package project

import (
	"strings"
	"testing"
)

// Ohne Abschnitt gilt switch: unknown — ausdrücklich, nicht als stilles Nein —,
// und configured bleibt false, damit die Oberfläche die offene Entscheidung
// nennen kann.
func TestGitSettingsOhneAbschnitt(t *testing.T) {
	settings, err := parseGitSettings("schema_version: 3\n\nproject:\n  repo_root: .\n")
	if err != nil {
		t.Fatalf("parseGitSettings: %v", err)
	}
	if settings.Switch != GitSwitchUnknown || settings.Configured {
		t.Errorf("ohne Abschnitt = %+v, erwartet unknown und nicht konfiguriert", settings)
	}
	if settings.Allow == nil || settings.Environments == nil {
		t.Errorf("Listen sind nil statt leer: %+v", settings)
	}
}

func TestGitSettingsLiestAbschnitt(t *testing.T) {
	content := `schema_version: 3

project:
  repo_root: omni-gw

git:
  # Umschalten aus der Oberfläche: unknown, offer oder off.
  switch: offer
  allow:
    - development
    - "stage"
    - remediation/*
  environments:
    dev: development
    stage: stage
    prod: master

tools:
  gh:
    status: enabled
`
	settings, err := parseGitSettings(content)
	if err != nil {
		t.Fatalf("parseGitSettings: %v", err)
	}
	if !settings.Configured || settings.Switch != GitSwitchOffer {
		t.Errorf("Switch = %q, Configured = %v", settings.Switch, settings.Configured)
	}
	if got := strings.Join(settings.Allow, ","); got != "development,stage,remediation/*" {
		t.Errorf("Allow = %q", got)
	}
	var envs []string
	for _, env := range settings.Environments {
		envs = append(envs, env.Name+"="+env.Branch)
	}
	if got := strings.Join(envs, ","); got != "dev=development,stage=stage,prod=master" {
		t.Errorf("Environments = %q, erwartet Dateireihenfolge", got)
	}
}

// Die Flussform wird ebenso gelesen, und ein Branch darf mehrere Umgebungen
// bedienen — in omni-gw geht stage einmal nach stage, einmal nach prod.
func TestGitSettingsFlussformUndGeteilterBranch(t *testing.T) {
	settings, err := parseGitSettings("git:\n  switch: off\n  allow: [main, 'feature/*']\n  environments:\n    stage: stage\n    prod: stage\n")
	if err != nil {
		t.Fatalf("parseGitSettings: %v", err)
	}
	if settings.Switch != GitSwitchOff || strings.Join(settings.Allow, ",") != "main,feature/*" || len(settings.Environments) != 2 {
		t.Errorf("settings = %+v", settings)
	}
}

// Ein leerer Abschnitt steht da, entscheidet aber nichts.
func TestGitSettingsLeererAbschnitt(t *testing.T) {
	settings, err := parseGitSettings("git:\n\ntools:\n  gh:\n    status: enabled\n")
	if err != nil {
		t.Fatalf("parseGitSettings: %v", err)
	}
	if !settings.Configured || settings.Switch != GitSwitchUnknown {
		t.Errorf("settings = %+v", settings)
	}
}

func TestGitSettingsFehlerfaelle(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "unbekannter Wert", content: "git:\n  switch: yes\n", want: "git.switch"},
		{name: "unbekannter Schlüssel", content: "git:\n  swich: offer\n", want: "git.swich"},
		{name: "leerer allow-Eintrag", content: "git:\n  allow:\n    - \"\"\n", want: "leeren Eintrag"},
		{name: "allow ist keine Liste", content: "git:\n  allow: main\n", want: "Liste"},
		{name: "unzulässiges Muster", content: "git:\n  allow:\n    - \"feature branch\"\n", want: "unzulässige Muster"},
		{name: "Umgebung ohne Branch", content: "git:\n  environments:\n    prod:\n", want: "git.environments.prod nennt keinen Branch"},
		{name: "leerer Umgebungsname", content: "git:\n  environments:\n    \"\": main\n", want: "Umgebungsnamen"},
		{name: "unzulässiger Branch", content: "git:\n  environments:\n    prod: -main\n", want: "unzulässigen Branch-Namen"},
		{name: "environments ist keine Abbildung", content: "git:\n  environments:\n    - main\n", want: "Abbildung"},
		// Umgebungen werden ohne Rücksicht auf Groß- und Kleinschreibung
		// zusammengelegt; zwei Schreibweisen desselben Namens sind eine Dublette.
		{name: "Umgebung doppelt in anderer Schreibweise", content: "git:\n  environments:\n    Prod: master\n    prod: main\n", want: "git.environments nennt die Umgebung prod doppelt"},
		{name: "Umgebung doppelt in Großbuchstaben", content: "git:\n  environments:\n    prod: master\n    dev: development\n    PROD: main\n", want: "(prod und PROD)"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseGitSettings(test.content)
			if err == nil {
				t.Fatalf("kein Fehler, erwartet %q", test.want)
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Errorf("Fehler = %q, erwartet darin %q", err, test.want)
			}
		})
	}
}

// Ein Fehler im Abschnitt bricht den Kontext ab wie ein unbekannter gh-Status:
// ein Tippfehler soll nicht wie „nicht entschieden" aussehen. Ohne Abschnitt
// steht der Standard in der Ausgabe.
func TestBuildContextLiestGitAbschnitt(t *testing.T) {
	root := newContextProject(t)

	context, err := BuildContext(root)
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}
	if context.Git.Configured || context.Git.Switch != GitSwitchUnknown {
		t.Errorf("Git ohne Abschnitt = %+v", context.Git)
	}

	write(t, ConfigPath(root), "schema_version: 3\n\nproject:\n  repo_root: .\n\ngit:\n  switch: offer\n  environments:\n    prod: main\n")
	context, err = BuildContext(root)
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}
	if !context.Git.Configured || context.Git.Switch != GitSwitchOffer || len(context.Git.Environments) != 1 {
		t.Errorf("Git = %+v", context.Git)
	}

	write(t, ConfigPath(root), "schema_version: 3\n\ngit:\n  switch: maybe\n")
	if _, err := BuildContext(root); err == nil {
		t.Error("ein unbekannter git.switch bricht den Kontext nicht ab")
	}
}

func TestBranchAllowed(t *testing.T) {
	allow := []string{"development", "stage", "remediation/*"}
	tests := []struct {
		branch string
		want   bool
	}{
		{"development", true},
		{"stage", true},
		{"stages", false},
		{"remediation/OMN-1", true},
		{"remediation/a/b", true},
		{"remediation", false},
		{"master", false},
	}
	for _, test := range tests {
		if got, _ := BranchAllowed(allow, test.branch); got != test.want {
			t.Errorf("BranchAllowed(%q) = %v, erwartet %v", test.branch, got, test.want)
		}
	}
	if ok, _ := BranchAllowed(nil, "beliebig"); !ok {
		t.Error("eine leere Liste erlaubt nicht jeden Branch")
	}
	if ok, _ := BranchAllowed([]string{"*-hotfix-*"}, "x-hotfix-1"); !ok {
		t.Error("mehrere * im Muster treffen nicht")
	}
}

func TestValidBranchName(t *testing.T) {
	for _, name := range []string{"main", "remediation/OMN-382-test", "release-1.2", "feature_x"} {
		if !ValidBranchName(name) {
			t.Errorf("%q abgelehnt", name)
		}
	}
	for _, name := range []string{"", "-f", "a..b", "a b", "a~1", "a^", "a:b", "x.lock", "a/", "/a", ".a", "a/.b", "@", "a@{1}", "a*", "a\\b"} {
		if ValidBranchName(name) {
			t.Errorf("%q angenommen", name)
		}
	}
}
