package webui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// newProject baut ein Zielprojekt mit Installation darin auf.
func newProject(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	writeFile(t, filepath.Join(root, project.PlaybookDirName, "commands", "k-test.md"), "test\n")
	writeFile(t, filepath.Join(root, project.PlaybookDirName, "skills", "beispiel", "SKILL.md"), "# Beispiel\n")
	return root
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("%s anlegen: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("%s schreiben: %v", path, err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s lesen: %v", path, err)
	}
	return string(data)
}

// Das Aktualisieren bekommt denselben Ablauf wie das Einrichten. Ohne das
// bliebe ein Projekt mit nur echter CLAUDE.md über „Aktualisieren" für immer
// unverändert.
func TestRelinkAfterUpdateRichtetEin(t *testing.T) {
	root := newProject(t)
	writeFile(t, filepath.Join(root, "CLAUDE.md"), "# Unser Projekt\n")

	_, message := relinkAfterUpdate(root, "Aktualisiert.")

	agents := filepath.Join(root, project.RootInstructionsFile)
	content := readFile(t, agents)
	if !strings.Contains(content, "# Unser Projekt") {
		t.Errorf("der mitgebrachte Inhalt fehlt in AGENTS.md:\n%s", content)
	}
	if !strings.Contains(content, "bin/k-playbook context") {
		t.Errorf("der Anstoß fehlt in AGENTS.md:\n%s", content)
	}

	destination, err := os.Readlink(filepath.Join(root, "CLAUDE.md"))
	if err != nil || destination != project.RootInstructionsFile {
		t.Errorf("CLAUDE.md zeigt auf %q, %v", destination, err)
	}
	if !project.LinksOK(project.CheckLinks(root)) {
		t.Errorf("nach dem Aktualisieren nicht eingerichtet: %+v", project.CheckLinks(root))
	}

	// Die Umbenennung gehört in den Antworttext des Updates, nicht nur in den
	// des Einrichtens.
	if !strings.Contains(message, "umbenannt") {
		t.Errorf("die Umbenennung fehlt im Antworttext: %s", message)
	}
}

// Ein Projekt ohne beide Dateien bekommt AGENTS.md erstmals aus der Vorlage.
// Das ist neues Verhalten für diesen Einstieg und deshalb gewollt benannt.
func TestRelinkAfterUpdateLegtWurzeldateiAn(t *testing.T) {
	root := newProject(t)

	_, message := relinkAfterUpdate(root, "Aktualisiert.")

	content := readFile(t, filepath.Join(root, project.RootInstructionsFile))
	if !strings.Contains(content, "bin/k-playbook context") {
		t.Errorf("der Anstoß fehlt in AGENTS.md:\n%s", content)
	}
	if !strings.Contains(message, project.RootInstructionsFile) {
		t.Errorf("die angelegte Wurzeldatei fehlt im Antworttext: %s", message)
	}

	// Ein zweiter Lauf ändert nichts mehr.
	relinkAfterUpdate(root, "Aktualisiert.")
	if readFile(t, filepath.Join(root, project.RootInstructionsFile)) != content {
		t.Error("der zweite Lauf hat AGENTS.md verändert")
	}
}

// Ein Konflikt gehört ebenfalls in den Antworttext — er bedeutet, dass Claude
// Code das Playbook bis zur Handarbeit nicht kennt.
func TestRelinkAfterUpdateMeldetKonflikt(t *testing.T) {
	root := newProject(t)
	writeFile(t, filepath.Join(root, project.RootInstructionsFile), "# A\n")
	writeFile(t, filepath.Join(root, "CLAUDE.md"), "# eigenständig\n")

	_, message := relinkAfterUpdate(root, "Aktualisiert.")

	if !strings.Contains(message, "sieht Claude Code den Anstoß nicht") {
		t.Errorf("der Konflikt fehlt im Antworttext: %s", message)
	}
	if content := readFile(t, filepath.Join(root, "CLAUDE.md")); content != "# eigenständig\n" {
		t.Errorf("CLAUDE.md wurde verändert: %q", content)
	}
}
