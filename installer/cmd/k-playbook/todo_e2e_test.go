package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// todoProject legt ein leeres k-playbook-Projekt an und macht es zum
// Arbeitsverzeichnis — das Subkommando sucht die K-PLAYBOOK.yaml von dort
// aufwärts.
func todoProject(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	if err := project.CreateConfig(root, "."); err != nil {
		t.Fatalf("Konfiguration anlegen: %v", err)
	}
	if err := os.MkdirAll(project.LocalDir(root), 0o755); err != nil {
		t.Fatalf("k-playbook-local anlegen: %v", err)
	}

	before, err := os.Getwd()
	if err != nil {
		t.Fatalf("Arbeitsverzeichnis: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("nach %s wechseln: %v", root, err)
	}
	t.Cleanup(func() { os.Chdir(before) })
	return root
}

// runTodoCaptured führt das Subkommando aus und fängt dabei stdout ab.
func runTodoCaptured(t *testing.T, args ...string) (string, error) {
	t.Helper()

	file, err := os.CreateTemp(t.TempDir(), "stdout-*")
	if err != nil {
		t.Fatalf("Ausgabedatei: %v", err)
	}
	before := os.Stdout
	os.Stdout = file
	runErr := runTodo(args)
	os.Stdout = before
	if err := file.Close(); err != nil {
		t.Fatalf("Ausgabedatei schließen: %v", err)
	}

	content, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatalf("Ausgabe lesen: %v", err)
	}
	return string(content), runErr
}

func decodeTodoOutput[T any](t *testing.T, output string) T {
	t.Helper()

	var decoded T
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatalf("Ausgabe ist kein JSON: %v — %s", err, output)
	}
	return decoded
}

// --help ist der Rauchtest nach einem Release: Exit 0, Kurzhilfe, und die
// Daten bleiben unangetastet — eine vorhandene TODO.md liegt danach
// unverändert da und die Migration ist nicht angelaufen.
func TestTodoHilfeIstFolgenlos(t *testing.T) {
	root := todoProject(t)
	legacy := project.LegacyTodoFile(root)
	if err := os.WriteFile(legacy, []byte("- [ ] Unangetastet\n"), 0o644); err != nil {
		t.Fatalf("TODO.md anlegen: %v", err)
	}

	for _, args := range [][]string{{}, {"--help"}, {"help"}} {
		output, err := runTodoCaptured(t, args...)
		if err != nil {
			t.Fatalf("todo %v: %v", args, err)
		}
		if !strings.Contains(output, "Unterbefehle:") {
			t.Errorf("todo %v ohne Kurzhilfe:\n%s", args, output)
		}
	}

	content, err := os.ReadFile(legacy)
	if err != nil {
		t.Fatalf("TODO.md fehlt nach der Hilfe: %v", err)
	}
	if string(content) != "- [ ] Unangetastet\n" {
		t.Errorf("TODO.md wurde verändert: %q", content)
	}
	if pathExistsForTest(project.TodoFile(root)) {
		t.Error("die Hilfe hat data/todos.json angelegt")
	}
}

func TestTodoAddListUndAbhaken(t *testing.T) {
	todoProject(t)

	output, err := runTodoCaptured(t, "add", "Erster", "Punkt", "--origin", "Task 053")
	if err != nil {
		t.Fatalf("todo add: %v", err)
	}
	added := decodeTodoOutput[todoEntryOutput](t, output)
	if added.Todo.ID != 1 || added.Todo.Text != "Erster Punkt" || added.Todo.Origin != "Task 053" {
		t.Fatalf("angelegter Eintrag: %+v", added.Todo)
	}

	if _, err := runTodoCaptured(t, "add", "Zweiter"); err != nil {
		t.Fatalf("todo add: %v", err)
	}

	output, err = runTodoCaptured(t, "update", "1", "--done")
	if err != nil {
		t.Fatalf("todo update --done: %v", err)
	}
	updated := decodeTodoOutput[todoEntryOutput](t, output)
	if updated.Todo.Done == "" || updated.Todo.DoneMigrated {
		t.Fatalf("abgehakter Eintrag: %+v", updated.Todo)
	}

	output, err = runTodoCaptured(t, "list")
	if err != nil {
		t.Fatalf("todo list: %v", err)
	}
	open := decodeTodoOutput[todoListOutput](t, output)
	if len(open.Todos) != 1 || open.Todos[0].ID != 2 {
		t.Fatalf("offene Einträge: %+v", open.Todos)
	}

	output, err = runTodoCaptured(t, "list", "--done")
	if err != nil {
		t.Fatalf("todo list --done: %v", err)
	}
	done := decodeTodoOutput[todoListOutput](t, output)
	if len(done.Todos) != 1 || done.Todos[0].ID != 1 {
		t.Fatalf("erledigte Einträge: %+v", done.Todos)
	}

	output, err = runTodoCaptured(t, "list", "--all")
	if err != nil {
		t.Fatalf("todo list --all: %v", err)
	}
	all := decodeTodoOutput[todoListOutput](t, output)
	if len(all.Todos) != 2 {
		t.Fatalf("alle Einträge: %+v", all.Todos)
	}

	output, err = runTodoCaptured(t, "update", "1", "--reopen")
	if err != nil {
		t.Fatalf("todo update --reopen: %v", err)
	}
	reopened := decodeTodoOutput[todoEntryOutput](t, output)
	if reopened.Todo.Done != "" {
		t.Fatalf("wieder geöffneter Eintrag: %+v", reopened.Todo)
	}
}

func TestTodoLehntUnbekannteKennungUndLeerenTextAb(t *testing.T) {
	todoProject(t)

	if _, err := runTodoCaptured(t, "add", "   "); err == nil {
		t.Error("leerer Text wurde angenommen")
	}
	if _, err := runTodoCaptured(t, "update", "99", "--done"); err == nil {
		t.Error("unbekannte Kennung wurde angenommen")
	}
	if _, err := runTodoCaptured(t, "delete", "99"); err == nil {
		t.Error("unbekannte Kennung wurde angenommen")
	}
	if _, err := runTodoCaptured(t, "update", "keine-zahl", "--done"); err == nil {
		t.Error("nichtnumerische Kennung wurde angenommen")
	}
	if _, err := runTodoCaptured(t, "schnabeltier"); err == nil {
		t.Error("unbekannter Unterbefehl wurde angenommen")
	}
}

// Löschen entfernt den Eintrag; die Kennung bleibt verbraucht.
func TestTodoDeleteHaeltNextIdFest(t *testing.T) {
	todoProject(t)

	for _, text := range []string{"Erster", "Zweiter"} {
		if _, err := runTodoCaptured(t, "add", text); err != nil {
			t.Fatalf("todo add %s: %v", text, err)
		}
	}
	if _, err := runTodoCaptured(t, "delete", "2"); err != nil {
		t.Fatalf("todo delete: %v", err)
	}

	output, err := runTodoCaptured(t, "add", "Dritter")
	if err != nil {
		t.Fatalf("todo add: %v", err)
	}
	added := decodeTodoOutput[todoEntryOutput](t, output)
	if added.Todo.ID != 3 {
		t.Fatalf("ID = %d, erwartet 3 — eine gelöschte Kennung wurde neu vergeben", added.Todo.ID)
	}
}

// Der Import hängt an ein bestehendes Dokument an und entfernt die Quelle.
func TestTodoImportHaengtAnBestehendesDokumentAn(t *testing.T) {
	root := todoProject(t)

	if _, err := runTodoCaptured(t, "add", "Bestehend"); err != nil {
		t.Fatalf("todo add: %v", err)
	}

	markdown := filepath.Join(root, "zulieferung.md")
	if err := os.WriteFile(markdown, []byte("- [ ] Importiert offen\n- [x] Importiert erledigt\n"), 0o644); err != nil {
		t.Fatalf("Markdown anlegen: %v", err)
	}

	output, err := runTodoCaptured(t, "import", markdown)
	if err != nil {
		t.Fatalf("todo import: %v", err)
	}
	imported := decodeTodoOutput[todoImportOutput](t, output)
	if imported.Imported != 2 {
		t.Fatalf("imported = %d, erwartet 2", imported.Imported)
	}
	if pathExistsForTest(markdown) {
		t.Error("die importierte Datei liegt noch da")
	}

	output, err = runTodoCaptured(t, "list", "--all")
	if err != nil {
		t.Fatalf("todo list --all: %v", err)
	}
	all := decodeTodoOutput[todoListOutput](t, output)
	if len(all.Todos) != 3 {
		t.Fatalf("Bestand nach dem Import: %+v", all.Todos)
	}
	if all.Todos[1].ID != 2 || all.Todos[2].ID != 3 {
		t.Errorf("Kennungen nach dem Import: %+v", all.Todos)
	}
	if !all.Todos[2].DoneMigrated {
		t.Errorf("importierter erledigter Eintrag ohne doneMigrated: %+v", all.Todos[2])
	}
}

func pathExistsForTest(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
