package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckLocalMeldetFehlendeStruktur(t *testing.T) {
	root := t.TempDir()

	statuses := CheckLocal(root)
	if LocalOK(statuses) {
		t.Fatal("leeres Projekt gilt als vollstaendig")
	}
	for _, status := range statuses {
		if status.Present {
			t.Errorf("%s als vorhanden gemeldet", status.Path)
		}
	}
}

func TestCreateLocalLegtStrukturAn(t *testing.T) {
	root := t.TempDir()

	statuses, err := CreateLocal(root)
	if err != nil {
		t.Fatalf("CreateLocal: %v", err)
	}
	if !LocalOK(statuses) {
		t.Fatalf("nach CreateLocal unvollstaendig: %+v", statuses)
	}

	local := LocalDir(root)
	for _, name := range []string{"rules", "reviews", "checks", "results", "guidelines", "commands", "tasks"} {
		if !isDir(filepath.Join(local, name)) {
			t.Errorf("%s fehlt", name)
		}
	}
	if !isDir(filepath.Join(local, "tasks", "done")) {
		t.Error("tasks/done fehlt")
	}
	if !fileExists(filepath.Join(local, "TODO.md")) {
		t.Error("TODO.md fehlt")
	}
}

// Git speichert keine leeren Verzeichnisse; ohne README waeren sie nach einem
// Clone des Projekts verschwunden.
func TestCreateLocalLegtReadmesAn(t *testing.T) {
	root := t.TempDir()

	if _, err := CreateLocal(root); err != nil {
		t.Fatalf("CreateLocal: %v", err)
	}

	readme := filepath.Join(LocalDir(root), "rules", "README.md")
	content, err := os.ReadFile(readme)
	if err != nil {
		t.Fatalf("README fehlt: %v", err)
	}
	if !strings.Contains(string(content), "Enforcement-Regeln") {
		t.Errorf("README ohne Zweckbeschreibung:\n%s", content)
	}
}

func TestCreateLocalUeberschreibtNichts(t *testing.T) {
	root := t.TempDir()
	local := LocalDir(root)

	// Ein Projekt, das schon Inhalte hat.
	if err := os.MkdirAll(filepath.Join(local, "rules"), 0o755); err != nil {
		t.Fatalf("Verzeichnis anlegen: %v", err)
	}
	eigen := filepath.Join(local, "rules", "README.md")
	if err := os.WriteFile(eigen, []byte("# eigene Beschreibung\n"), 0o644); err != nil {
		t.Fatalf("README anlegen: %v", err)
	}
	todo := filepath.Join(local, "TODO.md")
	if err := os.WriteFile(todo, []byte("- offener Punkt\n"), 0o644); err != nil {
		t.Fatalf("TODO.md anlegen: %v", err)
	}

	if _, err := CreateLocal(root); err != nil {
		t.Fatalf("CreateLocal: %v", err)
	}

	for path, want := range map[string]string{
		eigen: "# eigene Beschreibung\n",
		todo:  "- offener Punkt\n",
	} {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s lesen: %v", path, err)
		}
		if string(content) != want {
			t.Errorf("%s wurde veraendert: %q", path, content)
		}
	}
}

func TestCreateLocalIstIdempotent(t *testing.T) {
	root := t.TempDir()

	if _, err := CreateLocal(root); err != nil {
		t.Fatalf("erster Lauf: %v", err)
	}
	statuses, err := CreateLocal(root)
	if err != nil {
		t.Fatalf("zweiter Lauf: %v", err)
	}
	if !LocalOK(statuses) {
		t.Errorf("zweiter Lauf unvollstaendig: %+v", statuses)
	}
}

// Die drei Overlay-Sorten muessen ein lokales Gegenstueck haben, sonst greift
// die Overlay-Aufloesung ins Leere.
func TestLocalStructureDecktOverlaySortenAb(t *testing.T) {
	vorhanden := map[string]bool{}
	for _, entry := range LocalStructure() {
		vorhanden[entry.Path] = true
	}

	for _, kind := range []string{"rules", "reviews", "checks"} {
		if !vorhanden[kind] {
			t.Errorf("Overlay-Sorte %s fehlt in der lokalen Struktur", kind)
		}
	}
}

// priv/ bleibt versioniert, sein Inhalt nicht. Die .gitignore liegt im
// Verzeichnis selbst, damit die Projekt-.gitignore unangetastet bleibt.
func TestCreateLocalSchuetztPrivVerzeichnis(t *testing.T) {
	root := t.TempDir()

	if _, err := CreateLocal(root); err != nil {
		t.Fatalf("CreateLocal: %v", err)
	}

	priv := filepath.Join(LocalDir(root), "priv")
	if !isDir(priv) {
		t.Fatal("priv/ fehlt")
	}

	content, err := os.ReadFile(filepath.Join(priv, ".gitignore"))
	if err != nil {
		t.Fatalf(".gitignore fehlt: %v", err)
	}
	for _, want := range []string{"*", "!.gitignore", "!README.md"} {
		if !strings.Contains(string(content), want) {
			t.Errorf(".gitignore enthaelt %q nicht:\n%s", want, content)
		}
	}
}

// Nur priv/ bekommt eine .gitignore; die uebrigen Verzeichnisse sind normaler
// Projektinhalt.
func TestCreateLocalSchuetztNurPriv(t *testing.T) {
	root := t.TempDir()

	if _, err := CreateLocal(root); err != nil {
		t.Fatalf("CreateLocal: %v", err)
	}

	for _, name := range []string{"rules", "reviews", "checks", "results"} {
		if pathExists(filepath.Join(LocalDir(root), name, ".gitignore")) {
			t.Errorf("%s hat eine .gitignore, sollte aber versioniert sein", name)
		}
	}
}
