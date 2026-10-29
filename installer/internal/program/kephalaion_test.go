package program

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// kephalaionClone baut ein Projekt mit Clone, dessen bin/install-kephalaion
// einen Marker schreibt, bei install ein kephalaion nach ~/.local/bin legt
// und mit exitCode endet. Gibt Hauptverzeichnis und Marker zurück.
func kephalaionClone(t *testing.T, install bool, exitCode int) (string, string) {
	t.Helper()

	projectDir := t.TempDir()
	marker := filepath.Join(t.TempDir(), "install-kephalaion-gelaufen")
	place := ""
	if install {
		place = "printf '#!/bin/sh\\n' > \"$HOME/.local/bin/kephalaion\"\nchmod 755 \"$HOME/.local/bin/kephalaion\"\n"
	}
	writeExecutable(t, filepath.Join(projectDir, project.PlaybookDirName, "bin", "install-kephalaion"), fmt.Sprintf(
		"#!/bin/sh\necho \"Installiere aus $(pwd)\"\n: > %q\n%sexit %d\n", marker, place, exitCode))
	return projectDir, marker
}

// Fehlt kephalaion, läuft bin/install-kephalaion aus dem Hauptverzeichnis;
// Erfolg heißt, danach liegt kephalaion im PATH.
func TestInstallKephalaionInstalliertFehlendes(t *testing.T) {
	home, _ := isolate(t)
	projectDir, marker := kephalaionClone(t, true, 0)

	result, err := InstallKephalaion(projectDir)
	if err != nil {
		t.Fatalf("InstallKephalaion: %v\n%s", err, result.Output)
	}
	if !ran(marker) || !result.Installed {
		t.Fatalf("bin/install-kephalaion ist nicht gelaufen: %+v", result)
	}
	if want := filepath.Join(home, ".local", "bin", "kephalaion"); result.Path != want {
		t.Errorf("Path = %q, erwartet %q", result.Path, want)
	}
	resolved, _ := filepath.EvalSymlinks(projectDir)
	if !strings.Contains(result.Output, projectDir) && !strings.Contains(result.Output, resolved) {
		t.Errorf("bin/install-kephalaion lief nicht im Hauptverzeichnis: %q", result.Output)
	}
}

// Ein vorhandenes kephalaion bleibt unangetastet; das Skript läuft nicht.
func TestInstallKephalaionLaesstVorhandenesStehen(t *testing.T) {
	home, _ := isolate(t)
	present := filepath.Join(home, ".local", "bin", "kephalaion")
	writeExecutable(t, present, "#!/bin/sh\n")
	projectDir, marker := kephalaionClone(t, true, 0)

	result, err := InstallKephalaion(projectDir)
	if err != nil || ran(marker) || result.Installed || result.Path != present {
		t.Errorf("Ergebnis = %+v, Fehler %v, gelaufen %v", result, err, ran(marker))
	}
}

// Exit ≠ 0, Exit 0 ohne kephalaion im PATH und ein Clone ohne Skript sind je
// ein Fehler.
func TestInstallKephalaionFehler(t *testing.T) {
	isolate(t)

	projectDir, _ := kephalaionClone(t, false, 4)
	result, err := InstallKephalaion(projectDir)
	if err == nil || !strings.Contains(err.Error(), "Exit-Code 4") || !strings.Contains(err.Error(), project.KephalaionInstallCommand) || !strings.Contains(result.Output, "Installiere aus") {
		t.Errorf("Exit 4: Ergebnis = %+v, Fehler %v", result, err)
	}

	projectDir, _ = kephalaionClone(t, false, 0)
	if _, err := InstallKephalaion(projectDir); err == nil || !strings.Contains(err.Error(), "nicht im PATH") {
		t.Errorf("Exit 0 ohne Binary: Fehler %v", err)
	}

	empty := t.TempDir()
	if err := os.MkdirAll(filepath.Join(empty, project.PlaybookDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallKephalaion(empty); err == nil || !strings.Contains(err.Error(), "kennt "+project.KephalaionInstallCommand+" noch nicht") {
		t.Errorf("ohne Skript: Fehler %v", err)
	}
}
