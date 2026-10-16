package guiproc

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// KeyFrom folgt dem übergebenen Verzeichnis und nicht dem eigenen
// Arbeitsverzeichnis: ein Unterverzeichnis eines Projekts bildet den Schlüssel
// des Projekts, ein Verzeichnis ohne Installation den eigenen. Darauf beruht
// das MCP-Werkzeug, dessen Bezug sein projectDir ist.
func TestKeyFromFolgtDemUebergebenenVerzeichnis(t *testing.T) {
	projectDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectDir, project.ConfigFileName),
		[]byte("schema_version: 1\n"), 0o644); err != nil {
		t.Fatalf("Anker schreiben: %v", err)
	}
	sub := filepath.Join(projectDir, "unterverzeichnis")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatalf("%s anlegen: %v", sub, err)
	}

	if got := KeyFrom(sub); got != canonical(projectDir) {
		t.Errorf("Schlüssel aus dem Unterverzeichnis = %q, erwartet %q", got, canonical(projectDir))
	}

	ohne := t.TempDir()
	if got := KeyFrom(ohne); got != canonical(ohne) {
		t.Errorf("Schlüssel ohne Installation = %q, erwartet %q", got, canonical(ohne))
	}
	if KeyFrom(ohne) == KeyFrom(sub) {
		t.Error("zwei verschiedene Verzeichnisse, ein Schlüssel")
	}
}

// resolveLocation bildet dieselben Pfade wie Locate, legt aber nichts an.
func TestResolveLocationBildetDiePfadeOhneAnzulegen(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(root, "run"))

	location, err := resolveLocation("/home/x/projekt")
	if err != nil {
		t.Fatalf("Fehler: %v", err)
	}
	if location.Dir != filepath.Join(root, "run", dirName) {
		t.Errorf("Verzeichnis = %q", location.Dir)
	}
	if _, err := os.Stat(location.Dir); !os.IsNotExist(err) {
		t.Errorf("%s wurde angelegt: %v", location.Dir, err)
	}

	created, err := Locate("/home/x/projekt")
	if err != nil {
		t.Fatalf("Locate: %v", err)
	}
	if created != location {
		t.Errorf("Locate bildet andere Pfade: %+v gegen %+v", created, location)
	}
}

// StatusForDir braucht ein auflösbares Laufzeitverzeichnis; ohne jede Quelle
// dafür ist das ein Fehler und keine Auskunft „kein Dienst".
func TestStatusForDirOhneAufloesbaresVerzeichnis(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("HOME", "")

	if _, err := StatusForDir(t.TempDir()); err == nil {
		t.Error("kein Fehler, obwohl kein Laufzeitverzeichnis auflösbar ist")
	}
}
