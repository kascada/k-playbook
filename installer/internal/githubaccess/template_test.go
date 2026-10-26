package githubaccess

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Fehlt die Datei, legt Load die Vorlage an: Verzeichnis 0700, Datei 0600,
// Created wahr, genau eine warning, kein error — und keine Dienste. Der
// zweite Aufruf findet eine vorhandene, leere Datei und meldet nichts.
func TestLoadLegtVorlageEinmalAn(t *testing.T) {
	dir, _ := isolate(t)
	path := filepath.Join(dir, "tief", "k-playbook", FileName)

	result := Load(path)
	if !result.Created || len(result.Problems) != 1 {
		t.Fatalf("erster Aufruf: %+v", result)
	}
	warning := result.Problems[0]
	if warning.Level != LevelWarning || warning.Path != "file" || !strings.Contains(warning.Message, "gerade angelegt") || !strings.Contains(warning.Hint, path) {
		t.Errorf("Warnung: %+v", warning)
	}
	if result.HasErrors() || result.Services == nil || len(result.Services) != 0 {
		t.Errorf("Konfiguration nach dem Anlegen: %+v", result.Services)
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

	again := Load(path)
	if again.Created || len(again.Problems) != 0 || len(again.Services) != 0 {
		t.Errorf("zweiter Aufruf: %+v", again)
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
	if len(result.Problems) != 0 || len(result.Services) != 0 || result.Created {
		t.Errorf("Vorlage: %+v", result)
	}
	for _, want := range []string{"services: {}", "type: gh", "type: token", "type: ssh", "key: ", "host: ", "type: public", "disabled: true", "docs/github-access.md"} {
		if !strings.Contains(Template, want) {
			t.Errorf("Vorlage ohne %q", want)
		}
	}
}

// Lässt sich die Vorlage nicht anlegen — das Verzeichnis ist schreibgeschützt
// wie ein read-only Mount im Container —, ist das ein error, der den Pfad
// nennt, der auf dem Host anzulegen ist. Created bleibt falsch.
func TestLoadMeldetNichtAnlegbareVorlage(t *testing.T) {
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
	result := Load(path)
	if result.Created || len(result.Problems) != 1 {
		t.Fatalf("Datei: %+v", result)
	}
	problem := result.Problems[0]
	if problem.Level != LevelError || problem.Path != "file" || !strings.Contains(problem.Message, "lässt sich nicht anlegen") || !strings.Contains(problem.Hint, "Auf dem Host anlegen: "+path) {
		t.Errorf("Datei: %+v", problem)
	}
	if result.Services != nil {
		t.Errorf("Services trotz error: %+v", result.Services)
	}

	// Verzeichnis unter dem gesperrten.
	nested := filepath.Join(locked, "k-playbook", FileName)
	result = Load(nested)
	if result.Created || len(result.Problems) != 1 || !strings.Contains(result.Problems[0].Message, "Verzeichnis") || !strings.Contains(result.Problems[0].Hint, nested) {
		t.Errorf("Verzeichnis: %+v", result)
	}
}

// Ist der Pfad ein symbolischer Link ohne Ziel, sieht ReadFile keine Datei
// und O_EXCL einen belegten Pfad. Load darf dann nicht erneut in sich selbst
// laufen (das endete früher im Stack Overflow), sondern meldet einen error,
// der Link und Ziel nennt; der Link bleibt stehen. Sobald das Ziel da ist,
// liest Load es wie jede vorhandene Datei.
func TestLoadMeldetHaengendenSymlinkStattEndlosZuLesen(t *testing.T) {
	dir, _ := isolate(t)
	target := filepath.Join(dir, "ziel", FileName)
	path := filepath.Join(dir, FileName)
	if err := os.Symlink(target, path); err != nil {
		t.Skipf("kein Symlink möglich: %v", err)
	}

	// Rekursion ohne Abbruch endete hier mit fatal error: stack overflow,
	// das kein recover fängt — der Testlauf selbst ist die Messung.
	result := Load(path)
	if result.Created || len(result.Problems) != 1 {
		t.Fatalf("hängender Link: %+v", result)
	}
	problem := result.Problems[0]
	if problem.Level != LevelError || problem.Path != "file" || !strings.Contains(problem.Message, "symbolischer Link") || !strings.Contains(problem.Message, target) || !strings.Contains(problem.Hint, path) {
		t.Errorf("Problem: %+v", problem)
	}
	if result.Services != nil {
		t.Errorf("Services trotz error: %+v", result.Services)
	}
	if info, err := os.Lstat(path); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("Link angefasst: %v %v", info, err)
	}

	// Mit vorhandenem Ziel ist der Link eine gewöhnliche Datei.
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("services: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	again := Load(path)
	if again.Created || len(again.Problems) != 0 || len(again.Services) != 0 {
		t.Errorf("mit Ziel: %+v", again)
	}
}

// Ist der Override gesetzt, entsteht die Vorlage nur dort — nie am
// Standardort.
func TestVorlageEntstehtNurAmOverride(t *testing.T) {
	dir, home := isolate(t)
	override := filepath.Join(dir, FileName)
	path, err := Path()
	if err != nil || path != override {
		t.Fatalf("Path: %q %v", path, err)
	}
	Load(path)
	if _, err := os.Stat(override); err != nil {
		t.Errorf("Vorlage fehlt am Override: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "k-playbook")); !os.IsNotExist(err) {
		t.Errorf("Standardort angefasst: %v", err)
	}
}
