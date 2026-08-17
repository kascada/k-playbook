package project

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// legacyConfig ist eine Konfiguration aus Modell 1: zentrale Basisinstallation,
// paths.* zeigen ins k-playbook-Verzeichnis.
const legacyConfig = `schema_version: 1
layout: fixed-project-k-playbook

k_playbook:
  repo: ~/dev/k-playbook

paths:
  playbook: k-playbook
  tasks: k-playbook/tasks
  todo: k-playbook/TODO.md

project:
  repo_root: code
  vcs: git
`

func TestResetConfigSichertAlteDateiWeg(t *testing.T) {
	root := t.TempDir()
	writeFile(t, ConfigPath(root), legacyConfig)

	result, err := ResetConfig(root, "code")
	if err != nil {
		t.Fatalf("ResetConfig: %v", err)
	}

	if result.PreviousVersion != "1" {
		t.Errorf("PreviousVersion = %q, erwartet %q", result.PreviousVersion, "1")
	}
	if want := ConfigPath(root) + ".v1-alt"; result.BackupPath != want {
		t.Errorf("BackupPath = %q, erwartet %q", result.BackupPath, want)
	}

	backup, err := os.ReadFile(result.BackupPath)
	if err != nil {
		t.Fatalf("Sicherung lesen: %v", err)
	}
	if string(backup) != legacyConfig {
		t.Error("die Sicherung trägt nicht den alten Inhalt")
	}

	config, err := ReadConfig(root)
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}
	if err := CheckSchema(config); err != nil {
		t.Errorf("die neue Datei besteht die eigene Prüfung nicht: %v", err)
	}
	if config.RepoRoot != "code" {
		t.Errorf("RepoRoot = %q, erwartet %q", config.RepoRoot, "code")
	}
}

// Eine zweite Sicherung darf die erste nicht überschreiben: sie kann die
// einzigen Werte enthalten, die es noch gibt.
func TestResetConfigUeberschreibtSicherungNicht(t *testing.T) {
	root := t.TempDir()
	writeFile(t, ConfigPath(root), legacyConfig)
	writeFile(t, ConfigPath(root)+".v1-alt", "aus einem früheren Versuch\n")

	result, err := ResetConfig(root, ".")
	if err != nil {
		t.Fatalf("ResetConfig: %v", err)
	}
	if want := ConfigPath(root) + ".v1-alt-2"; result.BackupPath != want {
		t.Errorf("BackupPath = %q, erwartet %q", result.BackupPath, want)
	}

	first, err := os.ReadFile(ConfigPath(root) + ".v1-alt")
	if err != nil {
		t.Fatalf("erste Sicherung lesen: %v", err)
	}
	if string(first) != "aus einem früheren Versuch\n" {
		t.Error("die erste Sicherung wurde überschrieben")
	}
}

// Liegen Projektinhalte in der Installation, wird nichts geschrieben: sie
// müssen zuerst umziehen, sonst nimmt das nächste Update sie mit.
func TestResetConfigBlocktBeiProjektinhalten(t *testing.T) {
	root := t.TempDir()
	writeFile(t, ConfigPath(root), legacyConfig)
	writeFile(t, filepath.Join(root, PlaybookDirName, "tasks", "001-etwas.md"), "# Aufgabe\n")

	_, err := ResetConfig(root, ".")
	if err == nil {
		t.Fatal("Projektinhalte in der Installation wurden übergangen")
	}

	var legacy *LegacyContentError
	if !errors.As(err, &legacy) {
		t.Fatalf("Fehlertyp = %T, erwartet *LegacyContentError", err)
	}
	if want := filepath.Join(PlaybookDirName, "tasks"); !contains(legacy.Paths, want) {
		t.Errorf("Paths = %v, erwartet einen Eintrag %q", legacy.Paths, want)
	}

	data, err := os.ReadFile(ConfigPath(root))
	if err != nil {
		t.Fatalf("Konfiguration lesen: %v", err)
	}
	if string(data) != legacyConfig {
		t.Error("die alte Datei wurde trotz Blockade angefasst")
	}
}

// Eine aktuelle Konfiguration ist kein Fall für das Zurücksetzen.
func TestResetConfigLehntAktuelleFassungAb(t *testing.T) {
	root := t.TempDir()
	if err := CreateConfig(root, "."); err != nil {
		t.Fatalf("CreateConfig: %v", err)
	}

	if _, err := ResetConfig(root, "."); err == nil {
		t.Error("eine aktuelle Konfiguration wurde zurückgesetzt")
	}
}

// Eine Fassung neuer als die eigene ebenfalls nicht: dort ist die Installation
// hinterher, und das Zurücksetzen würde die neuere Datei wegwerfen.
func TestResetConfigLehntNeuereFassungAb(t *testing.T) {
	root := t.TempDir()
	writeFile(t, ConfigPath(root), "schema_version: 9\n")

	if _, err := ResetConfig(root, "."); err == nil {
		t.Error("eine neuere Fassung wurde zurückgesetzt")
	}
}

// Der paths-Block nennt die Orte genau; zeigt einer aus der Installation
// heraus, ist der Inhalt dort nicht gefährdet.
func TestLegacyContentIgnoriertPfadeAusserhalb(t *testing.T) {
	root := t.TempDir()
	writeFile(t, ConfigPath(root), `schema_version: 1

paths:
  playbook: k-playbook
  tasks: eigene/tasks
`)
	writeFile(t, filepath.Join(root, "eigene", "tasks", "001-etwas.md"), "# Aufgabe\n")
	if err := os.MkdirAll(filepath.Join(root, PlaybookDirName), 0o755); err != nil {
		t.Fatalf("Installation anlegen: %v", err)
	}

	if found := LegacyContent(root); len(found) > 0 {
		t.Errorf("LegacyContent = %v, erwartet nichts", found)
	}
}

// Ohne Installationsverzeichnis gibt es nichts zu verlieren.
func TestLegacyContentOhneInstallation(t *testing.T) {
	root := t.TempDir()
	writeFile(t, ConfigPath(root), legacyConfig)

	if found := LegacyContent(root); len(found) > 0 {
		t.Errorf("LegacyContent = %v, erwartet nichts", found)
	}
}

func TestSchemaState(t *testing.T) {
	cases := map[string]SchemaStatus{
		SchemaVersion: SchemaOK,
		"":            SchemaMissing,
		"1":           SchemaOutdated,
		"2":           SchemaOutdated,
		"9":           SchemaNewer,
	}

	for version, want := range cases {
		if got := SchemaState(Config{SchemaVersion: version}); got != want {
			t.Errorf("SchemaState(%q) = %q, erwartet %q", version, got, want)
		}
	}

	if !SchemaOutdated.Resettable() || !SchemaMissing.Resettable() {
		t.Error("eine zu alte Datei muss sich zurücksetzen lassen")
	}
	if SchemaNewer.Resettable() || SchemaOK.Resettable() {
		t.Error("nur abgelöste Fassungen dürfen zurückgesetzt werden")
	}
}

// Die Meldung muss den Ausweg nennen, sonst endet sie bei der Diagnose.
func TestCheckSchemaNenntDenAusweg(t *testing.T) {
	err := CheckSchema(Config{SchemaVersion: "1"})
	if err == nil {
		t.Fatal("schema_version 1 wurde akzeptiert")
	}
	if !strings.Contains(err.Error(), "k-playbook gui") {
		t.Errorf("Meldung nennt den Weg nicht: %v", err)
	}
}
