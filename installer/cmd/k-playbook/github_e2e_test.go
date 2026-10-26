package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/githubaccess"
)

// githubFile legt eine Zugangsdatei mit content an und macht sie über den
// Override zur Datei des Tests; ohne content fehlt die Datei. HOME zeigt
// auf ein eigenes Verzeichnis, damit ~ in den Pfaden vorhersagbar aufgelöst
// wird und die echte Datei des Rechners nie im Spiel ist.
func githubFile(t *testing.T, content string) (path string, home string) {
	t.Helper()
	home = t.TempDir()
	path = filepath.Join(t.TempDir(), githubaccess.FileName)
	t.Setenv("HOME", home)
	t.Setenv(githubaccess.EnvConfig, path)
	if content != "" {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return path, home
}

const githubValid = `services:
  website:
    repo: acme/website
    access:
      - type: gh
        login: alice
  backend:
    repo: acme-labs/backend
    access:
      - type: ssh
        key: ~/.ssh/id_ed25519_acme_labs
      - type: token
        file: ~/.config/github-tokens/acme-labs.token
        disabled: true
  playbook:
    repo: example/k-playbook
    access:
      - type: ssh
        host: github-example
      - type: public
`

const githubBroken = `services:
  website:
    repo: acme/website
    access:
      - type: gh
        login: alice
  backend:
    repo: acme-labs/backend
    access:
      - type: ssh
        key: ~/.ssh/id_ed25519_acme_labs
      - type: token
        file: tokens/acme-labs.token
`

// runGitHubCaptured führt das Subkommando aus und fängt stdout ab.
func runGitHubCaptured(t *testing.T, args ...string) (string, error) {
	t.Helper()

	file, err := os.Create(filepath.Join(t.TempDir(), "stdout"))
	if err != nil {
		t.Fatalf("Ausgabedatei: %v", err)
	}
	before := os.Stdout
	os.Stdout = file
	runErr := runGitHub(args)
	os.Stdout = before
	file.Close()

	stdout, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatalf("Ausgabe lesen: %v", err)
	}
	return string(stdout), runErr
}

// decodeJSON liest die Ausgabe als Objekt mit rohen Feldern.
func decodeJSON(t *testing.T, stdout string) map[string]json.RawMessage {
	t.Helper()
	var top map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stdout), &top); err != nil {
		t.Fatalf("kein JSON: %v\n%s", err, stdout)
	}
	return top
}

func TestGitHubCheckOhneProblemeIstEineZeile(t *testing.T) {
	path, home := githubFile(t, githubValid)

	stdout, err := runGitHubCaptured(t, "check")
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if stdout != path+": 3 Dienste, keine Probleme\n" {
		t.Errorf("Ausgabe: %q", stdout)
	}

	stdout, err = runGitHubCaptured(t, "check", "--json")
	if err != nil {
		t.Fatalf("check --json: %v", err)
	}
	top := decodeJSON(t, stdout)
	if len(top) != 4 || string(top["file"]) != `"`+path+`"` || string(top["created"]) != "false" || string(top["problems"]) != "[]" {
		t.Errorf("JSON oben: %v", top)
	}
	// services: Dateireihenfolge, ~ aufgelöst, disabled nur wo wahr.
	compact := strings.Join(strings.Fields(string(top["services"])), "")
	want := strings.Join(strings.Fields(`{"website":{"repo":"acme/website","access":[{"type":"gh","login":"alice"}]},
		"backend":{"repo":"acme-labs/backend","access":[{"type":"ssh","key":"`+home+`/.ssh/id_ed25519_acme_labs"},
		{"type":"token","file":"`+home+`/.config/github-tokens/acme-labs.token","disabled":true}]},
		"playbook":{"repo":"example/k-playbook","access":[{"type":"ssh","host":"github-example"},{"type":"public"}]}}`), "")
	if compact != want {
		t.Errorf("services:\n%s\nerwartet:\n%s", compact, want)
	}
}

// Die fehlende Datei: Vorlage angelegt, eine warning, Exit 0; der zweite
// Aufruf meldet eine leere Datei ohne Problem.
func TestGitHubCheckLegtVorlageAnUndEndetMitNull(t *testing.T) {
	path, _ := githubFile(t, "")

	stdout, err := runGitHubCaptured(t, "check")
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if !strings.HasPrefix(stdout, "[warning] file: Die Datei wurde gerade angelegt und ist leer") || !strings.Contains(stdout, " — ") || strings.Count(stdout, "\n") != 1 {
		t.Errorf("Ausgabe: %q", stdout)
	}
	info, statErr := os.Stat(path)
	if statErr != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("Vorlage: %v %v", info, statErr)
	}

	stdout, err = runGitHubCaptured(t, "check", "--json")
	if err != nil {
		t.Fatalf("zweiter Aufruf: %v", err)
	}
	top := decodeJSON(t, stdout)
	if string(top["created"]) != "false" || string(top["services"]) != "{}" || string(top["problems"]) != "[]" {
		t.Errorf("zweiter Aufruf: %v", top)
	}

	stdout, err = runGitHubCaptured(t, "check")
	if err != nil || stdout != path+": keine Dienste, keine Probleme\n" {
		t.Errorf("leere Datei: %q %v", stdout, err)
	}
}

func TestGitHubCheckMitFehlerEndetMitEins(t *testing.T) {
	path, _ := githubFile(t, githubBroken)

	stdout, err := runGitHubCaptured(t, "check")
	if exitCodeOf(err) != githubExitProblems || err.Error() != "" {
		t.Fatalf("Fehler: %v (Code %d)", err, exitCodeOf(err))
	}
	if stdout != "[error] services.backend.access[1]: Der Pfad in file ist relativ. — Absoluten Pfad oder ~/… eintragen.\n" {
		t.Errorf("Ausgabe: %q", stdout)
	}

	stdout, err = runGitHubCaptured(t, "check", "--json")
	if exitCodeOf(err) != githubExitProblems {
		t.Fatalf("--json: %v", err)
	}
	top := decodeJSON(t, stdout)
	if string(top["file"]) != `"`+path+`"` || string(top["created"]) != "false" || string(top["services"]) != "{}" {
		t.Errorf("JSON: %v", top)
	}
	var problems []map[string]string
	if err := json.Unmarshal(top["problems"], &problems); err != nil || len(problems) != 1 {
		t.Fatalf("problems: %s", top["problems"])
	}
	want := map[string]string{"level": "error", "path": "services.backend.access[1]", "message": "Der Pfad in file ist relativ.", "hint": "Absoluten Pfad oder ~/… eintragen."}
	for key, value := range want {
		if problems[0][key] != value {
			t.Errorf("problems[0].%s = %q, erwartet %q", key, problems[0][key], value)
		}
	}
	if len(problems[0]) != len(want) {
		t.Errorf("problems[0] hat fremde Felder: %v", problems[0])
	}
}

// show: je Dienst ein Block; mit Dienst nur dieser; JSON ohne problems. Text
// und JSON tragen dieselbe Information.
func TestGitHubShowZeigtDieKonfiguration(t *testing.T) {
	path, home := githubFile(t, githubValid)

	stdout, err := runGitHubCaptured(t, "show")
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	want := "Datei: " + path + "\n" +
		"\nwebsite: acme/website\n  1. gh  login: alice\n" +
		"\nbackend: acme-labs/backend\n  1. ssh  key: " + home + "/.ssh/id_ed25519_acme_labs\n  2. token  file: " + home + "/.config/github-tokens/acme-labs.token  (disabled)\n" +
		"\nplaybook: example/k-playbook\n  1. ssh  host: github-example\n  2. public\n"
	if stdout != want {
		t.Errorf("Ausgabe:\n%s\nerwartet:\n%s", stdout, want)
	}

	stdout, err = runGitHubCaptured(t, "show", "backend")
	if err != nil {
		t.Fatalf("show backend: %v", err)
	}
	if !strings.Contains(stdout, "backend: acme-labs/backend") || strings.Contains(stdout, "website") || strings.Contains(stdout, "playbook") {
		t.Errorf("show backend:\n%s", stdout)
	}

	stdout, err = runGitHubCaptured(t, "show", "--json")
	if err != nil {
		t.Fatalf("show --json: %v", err)
	}
	top := decodeJSON(t, stdout)
	if len(top) != 3 || string(top["file"]) != `"`+path+`"` || string(top["created"]) != "false" {
		t.Errorf("JSON oben: %v", top)
	}
	if _, has := top["problems"]; has {
		t.Error("show --json trägt problems")
	}
	var services map[string]json.RawMessage
	if err := json.Unmarshal(top["services"], &services); err != nil || len(services) != 3 {
		t.Errorf("services: %s", top["services"])
	}

	stdout, err = runGitHubCaptured(t, "show", "backend", "--json")
	if err != nil {
		t.Fatalf("show backend --json: %v", err)
	}
	services = nil
	if err := json.Unmarshal(decodeJSON(t, stdout)["services"], &services); err != nil || len(services) != 1 || services["backend"] == nil {
		t.Errorf("services: %s", stdout)
	}
}

// Ein Dienst, der nicht in der Datei steht: Aufruffehler mit den bekannten
// Diensten, Exit 2, kein JSON-Objekt.
func TestGitHubShowUnbekannterDienstEndetMitZwei(t *testing.T) {
	githubFile(t, githubValid)

	for _, args := range [][]string{{"show", "nope"}, {"show", "nope", "--json"}} {
		stdout, err := runGitHubCaptured(t, args...)
		if exitCodeOf(err) != githubExitUsage || !strings.Contains(err.Error(), "unbekannter Dienst nope") || !strings.Contains(err.Error(), "website, backend, playbook") {
			t.Errorf("%v: %v", args, err)
		}
		if stdout != "" {
			t.Errorf("%v: Ausgabe statt Fehler: %q", args, stdout)
		}
	}
}

// show bei einer Datei mit error: die Probleme wie check, in Text und JSON,
// Exit 1.
func TestGitHubShowBeiFehlerhafterDateiZeigtDieProbleme(t *testing.T) {
	githubFile(t, githubBroken)

	checkText, _ := runGitHubCaptured(t, "check")
	checkJSON, _ := runGitHubCaptured(t, "check", "--json")

	for _, args := range [][]string{{"show"}, {"show", "backend"}} {
		stdout, err := runGitHubCaptured(t, args...)
		if exitCodeOf(err) != githubExitProblems || err.Error() != "" || stdout != checkText {
			t.Errorf("%v: %v\n%s", args, err, stdout)
		}
	}
	stdout, err := runGitHubCaptured(t, "show", "--json")
	if exitCodeOf(err) != githubExitProblems || stdout != checkJSON {
		t.Errorf("show --json: %v\n%s", err, stdout)
	}
}

// Die frisch angelegte Vorlage bei show: Datei, Warnung, keine Dienste; JSON
// mit created true.
func TestGitHubShowNachVorlage(t *testing.T) {
	path, _ := githubFile(t, "")

	stdout, err := runGitHubCaptured(t, "show")
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if !strings.HasPrefix(stdout, "Datei: "+path+"\n[warning] file: Die Datei wurde gerade angelegt") || !strings.HasSuffix(stdout, "Keine Dienste eingetragen.\n") {
		t.Errorf("Ausgabe:\n%s", stdout)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("Vorlage fehlt: %v", err)
	}
}

func TestGitHubAufruffehlerEndenMitZwei(t *testing.T) {
	githubFile(t, githubValid)

	for _, args := range [][]string{{"nope"}, {"check", "--nein"}, {"check", "website"}, {"show", "a", "b"}} {
		stdout, err := runGitHubCaptured(t, args...)
		if exitCodeOf(err) != githubExitUsage || err.Error() == "" {
			t.Errorf("%v: %v", args, err)
		}
		if stdout != "" {
			t.Errorf("%v: Ausgabe: %q", args, stdout)
		}
	}

	stdout, err := runGitHubCaptured(t, "--help")
	if err != nil || !strings.Contains(stdout, "k-playbook github check [--json]") {
		t.Errorf("Hilfe: %v %q", err, stdout)
	}
}
