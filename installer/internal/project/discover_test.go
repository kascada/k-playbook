package project

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newInstallation legt ein Projekt mit Anker an.
func newInstallation(t *testing.T, projectDir string) {
	t.Helper()

	writeFile(t, ConfigPath(projectDir), "schema_version: 2\n")
}

// resolved gleicht Symlinks an, damit Vergleiche unter /tmp auf macOS greifen.
func resolved(t *testing.T, dir string) string {
	t.Helper()

	path, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("EvalSymlinks(%s): %v", dir, err)
	}
	return path
}

func TestDiscoverFindetAnkerImVerzeichnisSelbst(t *testing.T) {
	root := t.TempDir()
	newInstallation(t, root)

	found, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if want := resolved(t, root); found != want {
		t.Errorf("Discover = %q, erwartet %q", found, want)
	}
}

func TestDiscoverFindetAnkerAusTieferUnterebene(t *testing.T) {
	root := t.TempDir()
	newInstallation(t, root)

	deep := filepath.Join(root, "src", "paket", "unter")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatalf("Unterverzeichnis anlegen: %v", err)
	}

	found, err := Discover(deep)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if want := resolved(t, root); found != want {
		t.Errorf("Discover = %q, erwartet %q", found, want)
	}
}

// Ein Verzeichnis unterhalb des Hauptverzeichnisses kann ein eigener
// Git-Worktree sein — das Repository des Projekts (project.repo_root). Ein
// Abbruch am Worktree-Root würde den Anker eine Ebene darüber verdecken.
func TestDiscoverLaeuftUeberGitWorktreeHinaus(t *testing.T) {
	root := t.TempDir()
	newInstallation(t, root)

	code := filepath.Join(root, "code")
	if err := os.MkdirAll(filepath.Join(code, ".git"), 0o755); err != nil {
		t.Fatalf(".git in code/ anlegen: %v", err)
	}

	found, err := Discover(code)
	if err != nil {
		t.Fatalf("Discover aus code/ heraus: %v", err)
	}
	if want := resolved(t, root); found != want {
		t.Errorf("Discover = %q, erwartet %q", found, want)
	}
}

func TestDiscoverMeldetKeinenFund(t *testing.T) {
	root := t.TempDir()

	_, err := Discover(root)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Fehler = %v, erwartet %v", err, ErrNotFound)
	}
}

func TestDetectFromLiefertProjektverzeichnis(t *testing.T) {
	root := t.TempDir()
	newInstallation(t, root)

	environment := DetectFrom(root)
	if !environment.Installed {
		t.Fatal("Installed = false, erwartet true")
	}
	if want := resolved(t, root); environment.ProjectDir != want {
		t.Errorf("ProjectDir = %q, erwartet %q", environment.ProjectDir, want)
	}
}

// Die Config allein macht ein Verzeichnis zum Projekt. Die Umgebung trägt
// kein Feld zu einer Installation im Projekt (Task 157).
func TestDetectFromOhnePlaybookVerzeichnis(t *testing.T) {
	root := t.TempDir()
	writeFile(t, ConfigPath(root), "schema_version: 2\n")

	environment := DetectFrom(root)
	if !environment.Installed {
		t.Fatal("Installed = false, erwartet true")
	}
	assertNoPlaybookField := func(environment Environment) {
		t.Helper()
		encoded, err := json.Marshal(environment)
		if err != nil {
			t.Fatalf("Environment kodieren: %v", err)
		}
		for _, gone := range []string{"playbookPresent", "playbookDir"} {
			if strings.Contains(string(encoded), gone) {
				t.Errorf("Environment trägt noch %s: %s", gone, encoded)
			}
		}
	}
	assertNoPlaybookField(environment)
}

func TestDetectFromOhneInstallation(t *testing.T) {
	environment := DetectFrom(t.TempDir())
	if environment.Installed {
		t.Error("Installed = true, erwartet false")
	}
	if environment.ProjectDir != "" {
		t.Errorf("ProjectDir = %q, erwartet leer", environment.ProjectDir)
	}
}

// Ein Verzeichnis wird als Anker nur anerkannt, wenn K-PLAYBOOK.yaml eine Datei
// ist; ein gleichnamiges Verzeichnis darf nicht zählen.
func TestDiscoverIgnoriertGleichnamigesVerzeichnis(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(ConfigPath(root), 0o755); err != nil {
		t.Fatalf("Verzeichnis anlegen: %v", err)
	}

	if _, err := Discover(root); !errors.Is(err, ErrNotFound) {
		t.Errorf("Fehler = %v, erwartet %v", err, ErrNotFound)
	}
}

// Ein Unterverzeichnis kann ein .k-playbook/ ohne Konfiguration tragen. Das
// Verzeichnis allein ist kein Anker, sonst endete die Suche dort.
func TestDiscoverUebergehtKPlaybookVerzeichnisOhneKonfiguration(t *testing.T) {
	root := t.TempDir()
	newInstallation(t, root)

	unten := filepath.Join(root, "unten")
	writeFile(t, filepath.Join(unten, ProjectDirName, ".gitignore"), "/share\n")

	found, err := Discover(unten)
	if err != nil {
		t.Fatalf("Discover aus unten/ heraus: %v", err)
	}
	if want := resolved(t, root); found != want {
		t.Errorf("Discover = %q, erwartet %q", found, want)
	}
}

// Ein K-PLAYBOOK.yaml direkt im Hauptverzeichnis ist kein Anker: das Projekt
// gilt als nicht eingerichtet, und die Datei bleibt, wo sie ist.
func TestDiscoverKenntKeinenAnkerImHauptverzeichnis(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ConfigFileName), "schema_version: 3\n")

	if _, err := Discover(root); !errors.Is(err, ErrNotFound) {
		t.Errorf("Fehler = %v, erwartet %v", err, ErrNotFound)
	}
	if !fileExists(filepath.Join(root, ConfigFileName)) {
		t.Error("die Datei im Hauptverzeichnis wurde verschoben oder gelöscht")
	}
	if fileExists(ConfigPath(root)) {
		t.Error("unter .k-playbook/ ist eine Konfiguration entstanden")
	}
}
