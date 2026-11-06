package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/githubaccess"
)

// githubFile legt eine Zugangsdatei mit content an und macht sie über den
// Override zur Datei des Tests; ohne content fehlt die Datei. HOME zeigt
// auf ein eigenes Verzeichnis, damit ~ in den Pfaden vorhersagbar aufgelöst
// wird und die echte Datei des Rechners nie im Spiel ist. Für die
// Verfügbarkeit bekommt der Test eine eigene gh-Konfiguration mit dem
// Account alice und einen PATH, in dem gh nichts tut und ssh -G nur den
// Alias github-example nach github.com auflöst — der Rechner, auf dem der
// Test läuft, spielt keine Rolle.
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

	ghDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(ghDir, "hosts.yml"), []byte("github.com:\n    user: alice\n    oauth_token: gho_geheim\n    users:\n        alice:\n            oauth_token: gho_geheim\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GH_CONFIG_DIR", ghDir)
	bin := t.TempDir()
	ssh := "#!/bin/sh\ncase \"$2\" in github-example|github.com) echo \"hostname github.com\" ;; *) echo \"hostname $2\" ;; esac\n"
	for name, body := range map[string]string{"gh": "#!/bin/sh\nexit 0\n", "ssh": ssh} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	return path, home
}

// githubKeyMissing ist der Grund, den die Verfügbarkeit für den fehlenden
// Schlüssel von backend nennt.
func githubKeyMissing(home string) string {
	return "Die Schlüsseldatei fehlt — den Pfad in key prüfen oder den Schlüssel nach " + home + "/.ssh/id_ed25519_acme_labs legen."
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
	// backend hat nur einen fehlenden Schlüssel und ein deaktiviertes Token:
	// der eine Dienst ohne verfügbaren Zugang, und die eine Zeile darunter.
	// Je Dienst dazu die Wahl von run, nach den Zeilen zum Dienst.
	if stdout != path+": 3 Dienste, 1 ohne verfügbaren Zugang, keine Probleme\n"+
		"website run: git über gh alice, api über gh\n"+
		"services.backend.access[0]: nicht verfügbar — "+githubKeyMissing(home)+"\n"+
		"backend run: kein Zugang\n"+
		"playbook run: git über ssh host github-example, api: keiner\n" {
		t.Errorf("Ausgabe: %q", stdout)
	}

	stdout, err = runGitHubCaptured(t, "check", "--json")
	if err != nil {
		t.Fatalf("check --json: %v", err)
	}
	top := decodeJSON(t, stdout)
	if len(top) != 4 || string(top["file"]) != `"`+path+`"` || string(top["present"]) != "true" || string(top["problems"]) != "[]" {
		t.Errorf("JSON oben: %s", stdout)
	}
	// services: Dateireihenfolge, ~ aufgelöst, disabled nur wo wahr, je
	// Zugang state und bei unavailable der reason.
	var services map[string]json.RawMessage
	if err := json.Unmarshal(top["services"], &services); err != nil {
		t.Fatalf("services: %v", err)
	}
	compact := func(raw json.RawMessage) string { return strings.Join(strings.Fields(string(raw)), "") }
	reason, _ := json.Marshal(githubKeyMissing(home))
	for name, want := range map[string]string{
		"website":  `{"repo":"acme/website","access":[{"type":"gh","login":"alice","state":"available"}],"selected":{"read":{"git":0,"api":0},"write":{"git":0,"api":0}}}`,
		"backend":  `{"repo":"acme-labs/backend","access":[{"type":"ssh","key":"` + home + `/.ssh/id_ed25519_acme_labs","state":"unavailable","reason":` + string(reason) + `},{"type":"token","file":"` + home + `/.config/github-tokens/acme-labs.token","disabled":true,"state":"disabled"}],"selected":{"read":{"git":null,"api":null},"write":{"git":null,"api":null}}}`,
		"playbook": `{"repo":"example/k-playbook","access":[{"type":"ssh","host":"github-example","state":"available"},{"type":"public","state":"available"}],"selected":{"read":{"git":0,"api":null},"write":{"git":0,"api":null}}}`,
	} {
		if got := compact(services[name]); got != strings.Join(strings.Fields(want), "") {
			t.Errorf("services.%s:\n%s\nerwartet:\n%s", name, got, want)
		}
	}
	all := compact(top["services"])
	if !(strings.Index(all, `"website":`) < strings.Index(all, `"backend":`) && strings.Index(all, `"backend":`) < strings.Index(all, `"playbook":`)) {
		t.Errorf("services nicht in Dateireihenfolge: %s", all)
	}
}

// Die fehlende Datei: kein Problem, Exit 0, eine Zeile mit dem Hinweis auf
// init — und keine Datei entsteht, auch nicht beim zweiten Aufruf. Im JSON
// present false, hint, problems leer, kein created. Erst init legt sie an;
// danach meldet check eine leere Datei ohne Problem.
func TestGitHubCheckOhneDateiLegtNichtsAnUndEndetMitNull(t *testing.T) {
	path, _ := githubFile(t, "")

	for range 2 {
		stdout, err := runGitHubCaptured(t, "check")
		if err != nil {
			t.Fatalf("check: %v", err)
		}
		if stdout != path+": keine Zugangsdatei — k-playbook github init legt sie an\n" {
			t.Errorf("Ausgabe: %q", stdout)
		}
		if _, statErr := os.Lstat(path); !os.IsNotExist(statErr) {
			t.Fatalf("Datei angelegt: %v", statErr)
		}
	}

	stdout, err := runGitHubCaptured(t, "check", "--json")
	if err != nil {
		t.Fatalf("check --json: %v", err)
	}
	top := decodeJSON(t, stdout)
	if _, has := top["created"]; has || string(top["present"]) != "false" || string(top["hint"]) != `"`+githubaccess.MissingHint+`"` ||
		string(top["services"]) != "{}" || string(top["problems"]) != "[]" {
		t.Errorf("JSON: %s", stdout)
	}
	if _, statErr := os.Lstat(path); !os.IsNotExist(statErr) {
		t.Fatalf("Datei angelegt: %v", statErr)
	}

	if _, err := runGitHubCaptured(t, "init"); err != nil {
		t.Fatalf("init: %v", err)
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
	if string(top["file"]) != `"`+path+`"` || string(top["present"]) != "true" || string(top["services"]) != "{}" {
		t.Errorf("JSON: %s", stdout)
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
		"\nwebsite: acme/website\n  1. gh  login: alice  [verfügbar]\n  run: git über gh alice, api über gh\n" +
		"\nbackend: acme-labs/backend\n  1. ssh  key: " + home + "/.ssh/id_ed25519_acme_labs  [nicht verfügbar — " + githubKeyMissing(home) + "]\n  2. token  file: " + home + "/.config/github-tokens/acme-labs.token  [deaktiviert]\n  run: kein Zugang\n" +
		"\nplaybook: example/k-playbook\n  1. ssh  host: github-example  [verfügbar]\n  2. public  [verfügbar]\n  run: git über ssh host github-example, api: keiner\n"
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
	if len(top) != 3 || string(top["file"]) != `"`+path+`"` || string(top["present"]) != "true" {
		t.Errorf("JSON oben: %s", stdout)
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

// show und test ohne Datei: die Datei und der Hinweis auf init, Exit 0,
// auch mit einem Dienstnamen; im JSON present false und hint, bei test ok
// true ohne Dienste. Keine Datei entsteht, und GitHub wird nicht gefragt.
func TestGitHubShowUndTestOhneDatei(t *testing.T) {
	path, _ := githubFile(t, "")
	want := "Datei: " + path + "\nKeine Zugangsdatei — k-playbook github init legt sie an.\n"

	for _, args := range [][]string{{"show"}, {"show", "website"}, {"test"}, {"test", "website"}} {
		stdout, err := runGitHubCaptured(t, args...)
		if err != nil || stdout != want {
			t.Errorf("%v: %q %v", args, stdout, err)
		}
	}
	stdout, err := runGitHubCaptured(t, "show", "--json")
	top := decodeJSON(t, stdout)
	if err != nil || len(top) != 4 || string(top["present"]) != "false" || string(top["hint"]) != `"`+githubaccess.MissingHint+`"` || string(top["services"]) != "{}" {
		t.Errorf("show --json: %s %v", stdout, err)
	}
	stdout, err = runGitHubCaptured(t, "test", "--json")
	top = decodeJSON(t, stdout)
	if err != nil || string(top["present"]) != "false" || string(top["ok"]) != "true" || string(top["hint"]) != `"`+githubaccess.MissingHint+`"` || string(top["services"]) != "{}" {
		t.Errorf("test --json: %s %v", stdout, err)
	}
	if _, statErr := os.Lstat(path); !os.IsNotExist(statErr) {
		t.Errorf("Datei angelegt: %v", statErr)
	}
}

func TestGitHubAufruffehlerEndenMitZwei(t *testing.T) {
	githubFile(t, githubValid)

	for _, args := range [][]string{{"nope"}, {"check", "--nein"}, {"check", "website"}, {"show", "a", "b"}, {"init", "--nein"}, {"init", "website"}} {
		stdout, err := runGitHubCaptured(t, args...)
		if exitCodeOf(err) != githubExitUsage || err.Error() == "" {
			t.Errorf("%v: %v", args, err)
		}
		if stdout != "" {
			t.Errorf("%v: Ausgabe: %q", args, stdout)
		}
	}

	stdout, err := runGitHubCaptured(t, "--help")
	if err != nil || !strings.Contains(stdout, "k-playbook github check [--json]") || !strings.Contains(stdout, "k-playbook github init [--json]") {
		t.Errorf("Hilfe: %v %q", err, stdout)
	}
}

// init legt die Vorlage an (0600, Inhalt Template), Exit 0; der zweite
// Aufruf lässt die Datei stehen und sagt das, ebenfalls Exit 0. Mit --json
// das Objekt {file, created, problems}.
func TestGitHubInitLegtVorlageAnUndLaesstVorhandeneStehen(t *testing.T) {
	path, _ := githubFile(t, "")

	stdout, err := runGitHubCaptured(t, "init")
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if !strings.HasPrefix(stdout, path+": angelegt (Verzeichnis 0700, Datei 0600)") || strings.Count(stdout, "\n") != 1 {
		t.Errorf("Ausgabe: %q", stdout)
	}
	info, statErr := os.Stat(path)
	if statErr != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("Vorlage: %v %v", info, statErr)
	}
	if content, _ := os.ReadFile(path); string(content) != githubaccess.Template {
		t.Errorf("Inhalt:\n%s", content)
	}

	own := githubValid
	if err := os.WriteFile(path, []byte(own), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, err = runGitHubCaptured(t, "init")
	if err != nil || stdout != path+": schon vorhanden, nichts geändert\n" {
		t.Errorf("zweiter Aufruf: %q %v", stdout, err)
	}
	stdout, err = runGitHubCaptured(t, "init", "--json")
	if err != nil {
		t.Fatalf("init --json: %v", err)
	}
	top := decodeJSON(t, stdout)
	if len(top) != 3 || string(top["file"]) != `"`+path+`"` || string(top["created"]) != "false" || string(top["problems"]) != "[]" {
		t.Errorf("JSON: %s", stdout)
	}
	if content, _ := os.ReadFile(path); string(content) != own {
		t.Errorf("vorhandene Datei überschrieben:\n%s", content)
	}
}

// Lässt sich die Vorlage nicht anlegen (schreibgeschütztes Verzeichnis wie
// im Container), endet init mit 1 und nennt, was auf dem Host anzulegen ist.
func TestGitHubInitImSchreibgeschuetztenVerzeichnis(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("als root greift der Schreibschutz nicht")
	}
	githubFile(t, "")
	locked := filepath.Join(t.TempDir(), "gesperrt")
	if err := os.Mkdir(locked, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o700) })
	path := filepath.Join(locked, githubaccess.FileName)
	t.Setenv(githubaccess.EnvConfig, path)

	stdout, err := runGitHubCaptured(t, "init")
	if exitCodeOf(err) != githubExitProblems {
		t.Fatalf("Exit: %v (Code %d)", err, exitCodeOf(err))
	}
	if !strings.HasPrefix(stdout, "[error] file: Die Datei lässt sich nicht anlegen") || !strings.Contains(stdout, "Auf dem Host anlegen: "+path) {
		t.Errorf("Ausgabe: %q", stdout)
	}
	stdout, err = runGitHubCaptured(t, "init", "--json")
	top := decodeJSON(t, stdout)
	if exitCodeOf(err) != githubExitProblems || string(top["created"]) != "false" || !strings.Contains(string(top["problems"]), "Auf dem Host anlegen") {
		t.Errorf("JSON: %s (%v)", stdout, err)
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Errorf("Datei entstanden: %v", err)
	}
}

// githubTestFile ist eine Datei für test ohne API-Zugänge: der Alias
// github-example (Fake-ssh grüßt als Deploy Key, Fake-git liest), public
// (Fake-git), ein fehlender Schlüssel (skipped) und ein deaktiviertes Token.
// gh und token blieben außen vor — sie gingen an die echte API.
const githubTestable = `services:
  playbook:
    repo: example/k-playbook
    access:
      - type: ssh
        host: github-example
      - type: public
  backend:
    repo: acme-labs/backend
    access:
      - type: ssh
        key: ~/.ssh/id_ed25519_acme_labs
      - type: token
        file: ~/.config/github-tokens/acme-labs.token
        disabled: true
`

// githubTestTools ergänzt den PATH aus githubFile um ein ssh, das -T mit
// GitHubs Gruß beantwortet, und ein git, dessen ls-remote für URLs mit
// „missing“ scheitert. ok sagt, ob der Gruß kommt.
func githubTestTools(t *testing.T, ok bool) {
	t.Helper()
	bin := os.Getenv("PATH")
	greet := "echo \"Hi example/k-playbook! You've successfully authenticated, but GitHub does not provide shell access.\" >&2; exit 1"
	if !ok {
		greet = "echo \"git@github.com: Permission denied (publickey).\" >&2; exit 255"
	}
	ssh := "#!/bin/sh\nif [ \"$1\" = \"-G\" ]; then case \"$2\" in github-example|github.com) echo \"hostname github.com\" ;; *) echo \"hostname $2\" ;; esac; exit 0; fi\n" + greet + "\n"
	git := "#!/bin/sh\ncase \"$*\" in *missing*) echo \"ERROR: Repository not found.\" >&2; exit 128 ;; esac\necho \"abc\tHEAD\"\n"
	for name, body := range map[string]string{"ssh": ssh, "git": git} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

// test: je Dienst ein Block, je Zugang eine Zeile mit Ergebnis; JSON nach
// Vertrag; Exit 0, wenn kein getesteter Zugang failed ist.
func TestGitHubTestZeigtDieErgebnisse(t *testing.T) {
	path, home := githubFile(t, githubTestable)
	githubTestTools(t, true)

	stdout, err := runGitHubCaptured(t, "test")
	if err != nil {
		t.Fatalf("test: %v", err)
	}
	want := "Datei: " + path + "\n" +
		"\nplaybook: example/k-playbook\n" +
		"  1. ssh  host: github-example  → ok — Identität example/k-playbook, lesen ja, schreiben unbekannt, Ablauf keiner\n" +
		"  2. public  → ok — Identität anonymous, lesen ja, schreiben nein, Ablauf keiner\n" +
		"  run: git über ssh host github-example → ok, api: keiner\n" +
		"\nbackend: acme-labs/backend\n" +
		"  1. ssh  key: " + home + "/.ssh/id_ed25519_acme_labs  → übersprungen: nicht verfügbar — " + githubKeyMissing(home) + "\n" +
		"  2. token  file: " + home + "/.config/github-tokens/acme-labs.token  → übersprungen: deaktiviert\n" +
		"  run: kein Zugang\n"
	if stdout != want {
		t.Errorf("Ausgabe:\n%s\nerwartet:\n%s", stdout, want)
	}

	stdout, err = runGitHubCaptured(t, "test", "playbook", "--json")
	if err != nil {
		t.Fatalf("test --json: %v", err)
	}
	top := decodeJSON(t, stdout)
	if len(top) != 4 || string(top["file"]) != `"`+path+`"` || string(top["present"]) != "true" || string(top["ok"]) != "true" {
		t.Errorf("JSON oben: %s", stdout)
	}
	compact := strings.Join(strings.Fields(string(top["services"])), "")
	if compact != `{"playbook":{"repo":"example/k-playbook","access":[{"type":"ssh","host":"github-example","result":"ok","identity":"example/k-playbook","read":"yes","write":"unknown","expires":"none"},{"type":"public","result":"ok","identity":"anonymous","read":"yes","write":"no","expires":"none"}],"selected":{"read":{"git":0,"api":null},"write":{"git":0,"api":null}},"selectedOk":{"read":{"git":true,"api":null},"write":{"git":true,"api":null}}}}` {
		t.Errorf("services: %s", compact)
	}
}

// Ein failed macht Exit 1 und ok false; die Meldung ist SSHs stderr.
func TestGitHubTestMitFehlschlagEndetMitEins(t *testing.T) {
	githubFile(t, githubTestable)
	githubTestTools(t, false)

	stdout, err := runGitHubCaptured(t, "test", "playbook")
	if exitCodeOf(err) != githubExitProblems || err.Error() != "" {
		t.Fatalf("Fehler: %v", err)
	}
	if !strings.Contains(stdout, "  1. ssh  host: github-example  → fehlgeschlagen: ssh -T git@github-example: git@github.com: Permission denied (publickey).\n") || !strings.Contains(stdout, "  2. public  → ok") {
		t.Errorf("Ausgabe:\n%s", stdout)
	}
	stdout, err = runGitHubCaptured(t, "test", "--json")
	if exitCodeOf(err) != githubExitProblems || string(decodeJSON(t, stdout)["ok"]) != "false" {
		t.Errorf("--json: %v %s", err, stdout)
	}
}

// test mit unbekanntem Dienst ist ein Aufruffehler wie bei show; mit einer
// defekten Datei die Probleme wie check, Exit 1, nichts getestet.
func TestGitHubTestAufruffehlerUndDefekteDatei(t *testing.T) {
	githubFile(t, githubTestable)
	githubTestTools(t, true)
	stdout, err := runGitHubCaptured(t, "test", "nope")
	if exitCodeOf(err) != githubExitUsage || !strings.Contains(err.Error(), "unbekannter Dienst nope") || !strings.Contains(err.Error(), "playbook, backend") || stdout != "" {
		t.Errorf("unbekannter Dienst: %v %q", err, stdout)
	}
	if stdout, err := runGitHubCaptured(t, "test", "a", "b"); exitCodeOf(err) != githubExitUsage || stdout != "" {
		t.Errorf("zwei Dienste: %v %q", err, stdout)
	}

	githubFile(t, githubBroken)
	checkText, _ := runGitHubCaptured(t, "check")
	stdout, err = runGitHubCaptured(t, "test")
	if exitCodeOf(err) != githubExitProblems || stdout != checkText {
		t.Errorf("defekte Datei: %v\n%s", err, stdout)
	}
}

// githubSelectionFile hat Dienste, an denen sich die Wahl von run mit und
// ohne --write unterscheidet, einen mit getrenntem api-Kanal und einen ohne
// verfügbaren Zugang. gh liefert im PATH von githubFile kein Token: test
// meldet es failed, ohne die API zu fragen.
const githubSelectionFile = `services:
  playbook:
    repo: example/k-playbook
    access:
      - type: ssh
        host: github-example
      - type: public
  offen:
    repo: acme/offen
    access:
      - type: public
      - type: ssh
        key: ~/.ssh/id_offen
  nurpublic:
    repo: acme/nurpublic
    access:
      - type: public
  mitgh:
    repo: acme/mitgh
    access:
      - type: ssh
        host: github-example
      - type: gh
        login: alice
  aus:
    repo: acme/aus
    access:
      - type: ssh
        key: ~/.ssh/fehlt
`

// githubSelectionView ist, was ein Test aus dem JSON von check, show oder
// test über die Wahl eines Dienstes liest: die Zugänge (bei test mit
// result), selected und bei test selectedOk.
type githubSelectionView struct {
	name     string
	accesses []githubaccess.AccessResult
	selected *githubaccess.Selection
	ok       *githubaccess.SelectionOK
}

// githubSelectionViews liest services aus dem JSON von check/show (report
// false) oder test (report true) in Dateireihenfolge.
func githubSelectionViews(t *testing.T, stdout string, report bool) []githubSelectionView {
	t.Helper()
	raw := decodeJSON(t, stdout)["services"]
	var views []githubSelectionView
	if report {
		var services githubaccess.ServiceResults
		if err := json.Unmarshal(raw, &services); err != nil {
			t.Fatalf("services: %v", err)
		}
		for _, service := range services {
			views = append(views, githubSelectionView{service.Name, service.Access, service.Selected, service.SelectedOK})
		}
		return views
	}
	var services githubaccess.Services
	if err := json.Unmarshal(raw, &services); err != nil {
		t.Fatalf("services: %v", err)
	}
	for _, service := range services {
		accesses := make([]githubaccess.AccessResult, len(service.Access))
		for index, access := range service.Access {
			accesses[index] = githubaccess.AccessResult{Type: access.Type, Login: access.Login, File: access.File, Key: access.Key, Host: access.Host}
		}
		views = append(views, githubSelectionView{name: service.Name, accesses: accesses, selected: service.Selected})
	}
	return views
}

// githubExpectedChoice baut den Text einer Wahl aus Indizes und Zugängen nach
// der Tabelle im Vertrag (Output and exit codes), unabhängig vom Code, der
// ihn schreibt: „git über …, api über …“, ohne Zugang „kein Zugang“. Mit
// results hängt jeder gewählte Kanal „→ <ergebnis>“ an.
func githubExpectedChoice(accesses []githubaccess.AccessResult, index githubaccess.ChannelIndex, home string, results bool) string {
	if index.Git == nil && index.API == nil {
		return "kein Zugang"
	}
	tilde := func(path string) string {
		if rest, ok := strings.CutPrefix(path, home+"/"); ok {
			return "~/" + rest
		}
		return path
	}
	part := func(channel string, at *int, short bool) string {
		if at == nil {
			return channel + ": keiner"
		}
		access := accesses[*at]
		what := access.Type
		switch {
		case short:
		case access.Type == "gh":
			what = "gh " + access.Login
		case access.Type == "token":
			what = "token " + tilde(access.File)
		case access.Type == "ssh" && access.Key != "":
			what = "ssh key " + tilde(access.Key)
		case access.Type == "ssh":
			what = "ssh host " + access.Host
		}
		text := channel + " über " + what
		if results {
			text += " → " + map[githubaccess.Outcome]string{githubaccess.ResultOK: "ok", githubaccess.ResultFailed: "fehlgeschlagen", githubaccess.ResultSkipped: "übersprungen"}[access.Result]
		}
		return text
	}
	same := index.Git != nil && index.API != nil && *index.Git == *index.API
	return part("git", index.Git, false) + ", " + part("api", index.API, same)
}

// githubExpectedLines sind die Zeilen, die der Text zu selected tragen
// muss: run, und wo --write abweicht, mit --write — jeweils mit prefix.
func githubExpectedLines(views []githubSelectionView, home string, results bool, prefix func(name string) string) []string {
	var lines []string
	for _, view := range views {
		read := githubExpectedChoice(view.accesses, view.selected.Read, home, results)
		lines = append(lines, prefix(view.name)+"run: "+read)
		if write := githubExpectedChoice(view.accesses, view.selected.Write, home, results); !view.selected.Write.Equal(view.selected.Read) {
			lines = append(lines, prefix(view.name)+"mit --write: "+write)
		}
	}
	return lines
}

// githubSelectionTextLines zieht aus einem Text die Zeilen zur Wahl von run:
// bei show und test eingerückt, bei check mit dem Dienstnamen vorn.
func githubSelectionTextLines(text string) []string {
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimPrefix(line, "  ")
		if _, rest, found := strings.Cut(line, " "); found && !strings.HasPrefix(line, " ") && (strings.HasPrefix(rest, "run: ") || strings.HasPrefix(rest, "mit --write: ")) {
			lines = append(lines, line)
		} else if strings.HasPrefix(line, "  ") && (strings.HasPrefix(trimmed, "run: ") || strings.HasPrefix(trimmed, "mit --write: ")) {
			lines = append(lines, line)
		}
	}
	return lines
}

func describeChannelIndex(index githubaccess.ChannelIndex) string {
	part := func(value *int) string {
		if value == nil {
			return "null"
		}
		return strconv.Itoa(*value)
	}
	return part(index.Git) + "/" + part(index.API)
}

func describeChannelOK(ok githubaccess.ChannelOK) string {
	part := func(value *bool) string {
		if value == nil {
			return "null"
		}
		return strconv.FormatBool(*value)
	}
	return part(ok.Git) + "/" + part(ok.API)
}

// selected in check, show und test: dieselbe Wahl in allen dreien, nach der
// Regel von run, und der Text nennt für read und --write dieselben Zugänge
// wie das JSON. test meldet dazu selectedOk aus den Ergebnissen.
func TestGitHubSelectedInCheckShowUndTest(t *testing.T) {
	_, home := githubFile(t, githubSelectionFile)
	githubTestTools(t, true)
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".ssh", "id_offen"), []byte("schlüssel\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	outputs := []string{}
	capture := func(args ...string) string {
		t.Helper()
		stdout, err := runGitHubCaptured(t, args...)
		if err != nil && exitCodeOf(err) != githubExitProblems {
			t.Fatalf("%v: %v", args, err)
		}
		outputs = append(outputs, stdout)
		return stdout
	}

	show := githubSelectionViews(t, capture("show", "--json"), false)
	want := map[string]string{
		"playbook":  "read 0/null write 0/null",
		"offen":     "read 0/null write 1/null",
		"nurpublic": "read 0/null write null/null",
		"mitgh":     "read 0/1 write 0/1",
		"aus":       "read null/null write null/null",
	}
	if len(show) != len(want) {
		t.Fatalf("Dienste: %+v", show)
	}
	for _, view := range show {
		if view.selected == nil {
			t.Fatalf("%s ohne selected", view.name)
		}
		if got := "read " + describeChannelIndex(view.selected.Read) + " write " + describeChannelIndex(view.selected.Write); got != want[view.name] {
			t.Errorf("%s: selected %s, erwartet %s", view.name, got, want[view.name])
		}
	}

	// show: Text und JSON nennen dieselben Zugänge.
	indent := func(string) string { return "  " }
	showText := capture("show")
	if got, expected := githubSelectionTextLines(showText), githubExpectedLines(show, home, false, indent); strings.Join(got, "\n") != strings.Join(expected, "\n") {
		t.Errorf("show-Text:\n%s\nerwartet aus JSON:\n%s", strings.Join(got, "\n"), strings.Join(expected, "\n"))
	}
	for _, line := range []string{
		"  run: git über public, api: keiner\n  mit --write: git über ssh key ~/.ssh/id_offen, api: keiner\n",
		"  run: git über public, api: keiner\n  mit --write: kein Zugang\n",
		"  run: git über ssh host github-example, api über gh alice\n",
		"  run: kein Zugang\n",
	} {
		if !strings.Contains(showText, line) {
			t.Errorf("show-Text ohne %q:\n%s", line, showText)
		}
	}

	// check: dieselbe Wahl, im Text mit dem Dienstnamen vorn.
	check := githubSelectionViews(t, capture("check", "--json"), false)
	byName := func(name string) string { return name + " " }
	if got, expected := githubSelectionTextLines(capture("check")), githubExpectedLines(check, home, false, byName); strings.Join(got, "\n") != strings.Join(expected, "\n") {
		t.Errorf("check-Text:\n%s\nerwartet aus JSON:\n%s", strings.Join(got, "\n"), strings.Join(expected, "\n"))
	}
	for index := range check {
		if !reflect.DeepEqual(check[index].selected, show[index].selected) {
			t.Errorf("%s: check %+v, show %+v", check[index].name, check[index].selected, show[index].selected)
		}
	}

	// test: dieselbe Wahl wie show, dazu selectedOk aus den Ergebnissen; der
	// Text hängt an jeden gewählten Kanal sein Ergebnis.
	report := githubSelectionViews(t, capture("test", "--json"), true)
	for index, view := range report {
		if !reflect.DeepEqual(view.selected, show[index].selected) {
			t.Errorf("%s: test %+v, show %+v", view.name, view.selected, show[index].selected)
		}
		githubCheckSelectedOK(t, view)
	}
	if got, expected := githubSelectionTextLines(capture("test")), githubExpectedLines(report, home, true, indent); strings.Join(got, "\n") != strings.Join(expected, "\n") {
		t.Errorf("test-Text:\n%s\nerwartet aus JSON:\n%s", strings.Join(got, "\n"), strings.Join(expected, "\n"))
	}
	if got := describeChannelOK(report[3].ok.Read); report[3].name != "mitgh" || got != "true/false" {
		t.Errorf("mitgh selectedOk.read %s: ssh ok, gh ohne Token failed", got)
	}

	for _, output := range outputs {
		if strings.Contains(output, "gho_geheim") {
			t.Errorf("Token in der Ausgabe:\n%s", output)
		}
	}
}

// githubCheckSelectedOK prüft selectedOk eines Dienstes gegen die
// Ergebnisse der gewählten Zugänge: true genau bei ok, null wo selected null.
func githubCheckSelectedOK(t *testing.T, view githubSelectionView) {
	t.Helper()
	if view.ok == nil {
		t.Fatalf("%s ohne selectedOk", view.name)
	}
	pairs := []struct {
		index *int
		ok    *bool
	}{
		{view.selected.Read.Git, view.ok.Read.Git}, {view.selected.Read.API, view.ok.Read.API},
		{view.selected.Write.Git, view.ok.Write.Git}, {view.selected.Write.API, view.ok.Write.API},
	}
	for _, pair := range pairs {
		switch {
		case pair.index == nil && pair.ok != nil:
			t.Errorf("%s: selectedOk gesetzt, wo selected null ist", view.name)
		case pair.index != nil && (pair.ok == nil || *pair.ok != (view.accesses[*pair.index].Result == githubaccess.ResultOK)):
			t.Errorf("%s: selectedOk passt nicht zum Ergebnis von access[%d] (%s)", view.name, *pair.index, view.accesses[*pair.index].Result)
		}
	}
}

// Der gewählte Zugang ist failed, ein späterer ok: test wählt nicht neu —
// selected bleibt, selectedOk ist false, und der Text sagt es am Kanal.
func TestGitHubTestGewaehlterZugangFailed(t *testing.T) {
	_, home := githubFile(t, githubTestable)
	githubTestTools(t, false)

	show, err := runGitHubCaptured(t, "show", "playbook", "--json")
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	stdout, err := runGitHubCaptured(t, "test", "playbook", "--json")
	if exitCodeOf(err) != githubExitProblems {
		t.Fatalf("test --json: %v", err)
	}
	before, after := githubSelectionViews(t, show, false)[0], githubSelectionViews(t, stdout, true)[0]
	if !reflect.DeepEqual(before.selected, after.selected) {
		t.Errorf("selected vor dem Test %+v, im Report %+v", before.selected, after.selected)
	}
	if after.accesses[0].Result != githubaccess.ResultFailed || after.accesses[1].Result != githubaccess.ResultOK {
		t.Fatalf("Ergebnisse: %+v", after.accesses)
	}
	if got := describeChannelIndex(after.selected.Read) + " " + describeChannelOK(after.ok.Read) + " " + describeChannelOK(after.ok.Write); got != "0/null false/null false/null" {
		t.Errorf("selected/selectedOk: %s", got)
	}
	githubCheckSelectedOK(t, after)

	text, _ := runGitHubCaptured(t, "test", "playbook")
	if got, expected := githubSelectionTextLines(text), githubExpectedLines([]githubSelectionView{after}, home, true, func(string) string { return "  " }); strings.Join(got, "\n") != strings.Join(expected, "\n") {
		t.Errorf("test-Text:\n%s\nerwartet aus JSON:\n%s", strings.Join(got, "\n"), strings.Join(expected, "\n"))
	}
	if !strings.Contains(text, "  run: git über ssh host github-example → fehlgeschlagen, api: keiner\n") {
		t.Errorf("test-Text:\n%s", text)
	}
}
