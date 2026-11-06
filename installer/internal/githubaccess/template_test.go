package githubaccess

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Fehlt die Datei, legt Load nichts an: Present falsch, keine Dienste, kein
// Problem, der Hinweis auf init — und weder Datei noch Verzeichnis entstehen.
// Im JSON steht present und hint, kein created.
func TestLoadLegtBeiFehlenderDateiNichtsAn(t *testing.T) {
	dir, _ := isolate(t)
	path := filepath.Join(dir, "tief", "k-playbook", FileName)

	result := Load(path)
	if result.Present || len(result.Problems) != 0 || result.HasErrors() || result.Services == nil || len(result.Services) != 0 {
		t.Fatalf("fehlende Datei: %+v", result)
	}
	if result.File != path || result.Hint != MissingHint || !strings.Contains(MissingHint, "k-playbook github init") {
		t.Errorf("Pfad oder Hinweis: %+v", result)
	}
	if _, err := os.Lstat(filepath.Join(dir, "tief")); !os.IsNotExist(err) {
		t.Errorf("Verzeichnis angelegt: %v", err)
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Errorf("Datei angelegt: %v", err)
	}

	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatal(err)
	}
	if _, ok := top["created"]; ok || string(top["present"]) != "false" || string(top["hint"]) != `"`+MissingHint+`"` || string(top["problems"]) != "[]" || string(top["services"]) != "{}" {
		t.Errorf("JSON: %s", raw)
	}

	// Eine vorhandene Datei trägt present wahr und keinen Hinweis.
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("services: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if again := Load(path); !again.Present || again.Hint != "" || len(again.Problems) != 0 {
		t.Errorf("vorhandene Datei: %+v", again)
	}
}

// Die Vorlage selbst liest sich ohne Probleme und nennt jede Zugangsart als
// Beispiel.
func TestVorlageLiestSichOhneProbleme(t *testing.T) {
	dir, _ := isolate(t)
	path := filepath.Join(dir, FileName)
	if err := os.WriteFile(path, []byte(Template), 0o600); err != nil {
		t.Fatal(err)
	}
	result := Load(path)
	if len(result.Problems) != 0 || len(result.Services) != 0 || !result.Present {
		t.Errorf("Vorlage: %+v", result)
	}
	for _, want := range []string{"services: {}", "type: gh", "type: token", "type: ssh", "key: ", "host: ", "type: public", "disabled: true", "docs/github-access.md"} {
		if !strings.Contains(Template, want) {
			t.Errorf("Vorlage ohne %q", want)
		}
	}
}

// Init legt die Vorlage an: Verzeichnis 0700 (auch die Zwischenstufen),
// Datei 0600, Inhalt Template, Created wahr, kein Problem. Der zweite Aufruf
// findet die Datei und lässt sie unverändert — auch eine, die inzwischen
// ausgefüllt wurde oder Fehler hat.
func TestInitLegtVorlageAnUndLaesstVorhandeneStehen(t *testing.T) {
	dir, _ := isolate(t)
	path := filepath.Join(dir, "tief", "k-playbook", FileName)

	result := Init(path)
	if !result.Created || result.HasErrors() || result.File != path || result.Problems == nil {
		t.Fatalf("erster Aufruf: %+v", result)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("Datei: %v %v", info, err)
	}
	for _, d := range []string{filepath.Dir(path), filepath.Dir(filepath.Dir(path))} {
		if info, err := os.Stat(d); err != nil || info.Mode().Perm() != 0o700 {
			t.Errorf("Verzeichnis %s: %v %v", d, info, err)
		}
	}
	content, _ := os.ReadFile(path)
	if string(content) != Template {
		t.Errorf("Inhalt ist nicht die Vorlage:\n%s", content)
	}
	if loaded := Load(path); !loaded.Present || len(loaded.Problems) != 0 || len(loaded.Services) != 0 {
		t.Errorf("Load nach Init: %+v", loaded)
	}

	// Vorhanden, auch mit Fehlern: nichts geschrieben, kein Problem.
	own := "services:\n  kaputt: [\n"
	if err := os.WriteFile(path, []byte(own), 0o600); err != nil {
		t.Fatal(err)
	}
	again := Init(path)
	if again.Created || again.HasErrors() {
		t.Errorf("zweiter Aufruf: %+v", again)
	}
	if content, _ := os.ReadFile(path); string(content) != own {
		t.Errorf("vorhandene Datei überschrieben:\n%s", content)
	}
}

// Lässt sich die Vorlage nicht anlegen — das Verzeichnis ist schreibgeschützt
// wie ein read-only Mount im Container —, ist das ein error, der den Pfad
// nennt, der auf dem Host anzulegen ist. Created bleibt falsch, nichts
// entsteht. Load meldet denselben Pfad dagegen nur als fehlend.
func TestInitMeldetNichtAnlegbareVorlage(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("als root ist jedes Verzeichnis beschreibbar")
	}
	dir, _ := isolate(t)
	locked := filepath.Join(dir, "nur-lesen")
	if err := os.Mkdir(locked, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o700) })

	// Datei direkt im gesperrten Verzeichnis.
	path := filepath.Join(locked, FileName)
	if loaded := Load(path); loaded.Present || len(loaded.Problems) != 0 {
		t.Errorf("Load im gesperrten Verzeichnis: %+v", loaded)
	}
	result := Init(path)
	if result.Created || len(result.Problems) != 1 {
		t.Fatalf("Datei: %+v", result)
	}
	problem := result.Problems[0]
	if problem.Level != LevelError || problem.Path != "file" || !strings.Contains(problem.Message, "lässt sich nicht anlegen") || !strings.Contains(problem.Hint, "Auf dem Host anlegen: "+path) {
		t.Errorf("Datei: %+v", problem)
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Errorf("Datei entstanden: %v", err)
	}

	// Verzeichnis unter dem gesperrten.
	nested := filepath.Join(locked, "k-playbook", FileName)
	result = Init(nested)
	if result.Created || len(result.Problems) != 1 || !strings.Contains(result.Problems[0].Message, "Verzeichnis") || !strings.Contains(result.Problems[0].Hint, nested) {
		t.Errorf("Verzeichnis: %+v", result)
	}
}

// Ist der Pfad ein symbolischer Link ohne Ziel, meldet os.ReadFile ihn wie
// eine fehlende Datei. Load unterscheidet ihn trotzdem — Present wahr, ein
// error, der Link und Ziel nennt —, und Init legt dort nichts an, sondern
// meldet denselben error; der Link bleibt stehen. Sobald das Ziel da ist,
// liest Load es wie jede vorhandene Datei.
func TestHaengenderSymlinkIstBelegtNichtFehlend(t *testing.T) {
	dir, _ := isolate(t)
	target := filepath.Join(dir, "ziel", FileName)
	path := filepath.Join(dir, FileName)
	if err := os.Symlink(target, path); err != nil {
		t.Skipf("kein Symlink möglich: %v", err)
	}

	result := Load(path)
	if !result.Present || result.Hint != "" || len(result.Problems) != 1 {
		t.Fatalf("hängender Link: %+v", result)
	}
	problem := result.Problems[0]
	if problem.Level != LevelError || problem.Path != "file" || !strings.Contains(problem.Message, "symbolischer Link") || !strings.Contains(problem.Message, target) || !strings.Contains(problem.Hint, path) {
		t.Errorf("Problem: %+v", problem)
	}
	if result.Services != nil {
		t.Errorf("Services trotz error: %+v", result.Services)
	}

	initResult := Init(path)
	if initResult.Created || len(initResult.Problems) != 1 || initResult.Problems[0].Message != problem.Message {
		t.Errorf("Init auf hängendem Link: %+v", initResult)
	}
	if info, err := os.Lstat(path); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("Link angefasst: %v %v", info, err)
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		t.Errorf("Ziel angelegt: %v", err)
	}

	// Mit vorhandenem Ziel ist der Link eine gewöhnliche Datei.
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("services: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	again := Load(path)
	if !again.Present || len(again.Problems) != 0 || len(again.Services) != 0 {
		t.Errorf("mit Ziel: %+v", again)
	}
	if initAgain := Init(path); initAgain.Created || initAgain.HasErrors() {
		t.Errorf("Init mit Ziel: %+v", initAgain)
	}
}

// Liegt an der Stelle der Datei ein Verzeichnis, ist der Pfad belegt: Init
// meldet einen error und legt nichts an.
func TestInitMeldetVerzeichnisAmPfad(t *testing.T) {
	dir, _ := isolate(t)
	path := filepath.Join(dir, FileName)
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	result := Init(path)
	if result.Created || len(result.Problems) != 1 || result.Problems[0].Level != LevelError || !strings.Contains(result.Problems[0].Message, "Verzeichnis") || !strings.Contains(result.Problems[0].Hint, path) {
		t.Errorf("Verzeichnis am Pfad: %+v", result)
	}
	if entries, err := os.ReadDir(path); err != nil || len(entries) != 0 {
		t.Errorf("Verzeichnis verändert: %v %v", entries, err)
	}
}

// Ist der Override gesetzt, legt Init nur dort an — nie am Standardort; Load
// legt an keinem von beiden etwas an.
func TestInitNurAmOverride(t *testing.T) {
	dir, home := isolate(t)
	override := filepath.Join(dir, FileName)
	path, err := Path()
	if err != nil || path != override {
		t.Fatalf("Path: %q %v", path, err)
	}
	Load(path)
	if _, err := os.Stat(override); !os.IsNotExist(err) {
		t.Errorf("Load hat am Override angelegt: %v", err)
	}
	if result := Init(path); !result.Created {
		t.Fatalf("Init: %+v", result)
	}
	if _, err := os.Stat(override); err != nil {
		t.Errorf("Vorlage fehlt am Override: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "k-playbook")); !os.IsNotExist(err) {
		t.Errorf("Standardort angefasst: %v", err)
	}
}
