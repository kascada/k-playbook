package project

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// newInstallation legt ein Projekt mit Anker und Installation an.
func newInstallation(t *testing.T, projectDir string) {
	t.Helper()

	if err := os.MkdirAll(PlaybookDir(projectDir), 0o755); err != nil {
		t.Fatalf("Installation anlegen: %v", err)
	}
	if err := os.WriteFile(ConfigPath(projectDir), []byte("schema_version: 2\n"), 0o644); err != nil {
		t.Fatalf("Config anlegen: %v", err)
	}
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

// Die Installation ist ein eigener Clone und damit ein eigener Git-Worktree.
// Ein Abbruch am Worktree-Root wuerde den Anker eine Ebene darueber verdecken.
func TestDiscoverLaeuftUeberGitWorktreeHinaus(t *testing.T) {
	root := t.TempDir()
	newInstallation(t, root)

	playbook := PlaybookDir(root)
	if err := os.MkdirAll(filepath.Join(playbook, ".git"), 0o755); err != nil {
		t.Fatalf(".git in der Installation anlegen: %v", err)
	}

	found, err := Discover(playbook)
	if err != nil {
		t.Fatalf("Discover aus der Installation heraus: %v", err)
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

func TestDetectFromLiefertBeideVerzeichnisse(t *testing.T) {
	root := t.TempDir()
	newInstallation(t, root)

	environment := DetectFrom(root)
	if !environment.Installed {
		t.Fatal("Installed = false, erwartet true")
	}
	if want := resolved(t, root); environment.ProjectDir != want {
		t.Errorf("ProjectDir = %q, erwartet %q", environment.ProjectDir, want)
	}
	if want := PlaybookDir(resolved(t, root)); environment.PlaybookDir != want {
		t.Errorf("PlaybookDir = %q, erwartet %q", environment.PlaybookDir, want)
	}
	if !environment.PlaybookPresent {
		t.Error("PlaybookPresent = false, erwartet true")
	}
}

// Nach einem frischen Clone des Projekts ist die Config da, die Installation
// aber noch nicht.
func TestDetectFromMeldetFehlendeInstallation(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(ConfigPath(root), []byte("schema_version: 2\n"), 0o644); err != nil {
		t.Fatalf("Config anlegen: %v", err)
	}

	environment := DetectFrom(root)
	if !environment.Installed {
		t.Fatal("Installed = false, erwartet true")
	}
	if environment.PlaybookPresent {
		t.Error("PlaybookPresent = true, erwartet false")
	}
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
// ist; ein gleichnamiges Verzeichnis darf nicht zaehlen.
func TestDiscoverIgnoriertGleichnamigesVerzeichnis(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(ConfigPath(root), 0o755); err != nil {
		t.Fatalf("Verzeichnis anlegen: %v", err)
	}

	if _, err := Discover(root); !errors.Is(err, ErrNotFound) {
		t.Errorf("Fehler = %v, erwartet %v", err, ErrNotFound)
	}
}
