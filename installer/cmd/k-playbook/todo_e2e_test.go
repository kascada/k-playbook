package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/kephalaion/kephtest"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// todoProject legt ein leeres k-playbook-Projekt an und macht es zum
// Arbeitsverzeichnis — das Subkommando sucht die K-PLAYBOOK.yaml von dort
// aufwärts. Die Todos liegen in einer Collection im Speicher.
func todoProject(t *testing.T) string {
	t.Helper()
	todoProjectStore(t)
	return todoProjectRoot(t)
}

// todoProjectStore setzt eine Collection im Speicher für die Todos ein.
func todoProjectStore(t *testing.T) *kephtest.Store {
	t.Helper()
	store := kephtest.New()
	original := project.OpenTodoStore
	project.OpenTodoStore = func(string) (project.TodoStore, string, error) { return store, "kamran", nil }
	t.Cleanup(func() { project.OpenTodoStore = original })
	return store
}

func todoProjectRoot(t *testing.T) string {
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
// unverändert da, und die Collection wird nicht gefragt.
func TestTodoHilfeIstFolgenlos(t *testing.T) {
	store := todoProjectStore(t)
	root := todoProjectRoot(t)
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
	if len(store.Calls) != 0 {
		t.Errorf("die Hilfe hat Kephalaion gefragt: %v", store.Calls)
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

// Löschen entfernt den Eintrag; die höchste Nummer wird danach wieder
// vergeben.
func TestTodoDeleteGibtHoechsteNummerFrei(t *testing.T) {
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
	if added.Todo.ID != 2 {
		t.Fatalf("ID = %d, erwartet 2", added.Todo.ID)
	}
}

func TestTodoListSucht(t *testing.T) {
	store := todoProjectStore(t)
	todoProjectRoot(t)
	store.Put("todos/001-a.md", "Kephalaion anbinden\n")
	store.Put("todos/002-b.md", "GUI\n")
	store.Put("todos/done/003-c.md", "---\ndone: 2026-10-01\n---\n\nkephalaion-Doku\n")

	for args, want := range map[string]int{"list --search KEPHALAION": 1, "list --search kephalaion --all": 2,
		"list --search kephalaion --done": 1, "list --search nichts": 0} {
		output, err := runTodoCaptured(t, strings.Fields(args)...)
		if err != nil {
			t.Fatalf("%s: %v", args, err)
		}
		if got := decodeTodoOutput[todoListOutput](t, output); len(got.Todos) != want {
			t.Errorf("%s: %+v, erwartet %d", args, got.Todos, want)
		}
	}
	if _, err := runTodoCaptured(t, "list", "--search"); err == nil {
		t.Error("--search ohne Wert angenommen")
	}
}

// import gibt es nicht mehr; vor der Übernahme von data/todos.json sind
// Schreibvorgänge mit Code abgelehnt, die Liste geht.
func TestTodoOhneImportUndNichtUebernommen(t *testing.T) {
	root := todoProject(t)
	if _, err := runTodoCaptured(t, "import", "x.md"); err == nil || !strings.Contains(err.Error(), "unbekannter Unterbefehl") {
		t.Fatalf("import: %v", err)
	}
	if err := os.MkdirAll(project.TodoDataDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(project.TodoFile(root), []byte(`{"todos":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	output, err := runTodoCaptured(t, "list")
	if err != nil || !strings.Contains(decodeTodoOutput[todoListOutput](t, output).Hint, "k-playbook todo migrate") {
		t.Fatalf("list: %v %s", err, output)
	}
	if _, err := runTodoCaptured(t, "add", "neu"); err == nil || !strings.Contains(err.Error(), project.TodoCodeNotMigrated) {
		t.Fatalf("add vor der Übernahme: %v", err)
	}
}

func TestTodoMigrateUebernimmt(t *testing.T) {
	store := todoProjectStore(t)
	root := todoProjectRoot(t)
	if err := os.MkdirAll(project.TodoDataDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(project.TodoFile(root), []byte(`{"schemaVersion":1,"nextId":3,"todos":[
		{"id":2,"text":"Zweiter","created":"2026-09-01","done":"2026-09-02","origin":"Test"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	output, err := runTodoCaptured(t, "migrate")
	if err != nil {
		t.Fatalf("todo migrate: %v", err)
	}
	result := decodeTodoOutput[project.TodoMigration](t, output)
	if result.Mode != "first" || len(result.Created) != 1 || result.Created[0].Name != "todos/done/002-zweiter.md" {
		t.Fatalf("Ergebnis: %+v", result)
	}
	if _, ok := store.Content("todos/done/002-zweiter.md"); !ok || pathExistsForTest(project.TodoFile(root)) {
		t.Fatalf("nicht übernommen: %v", store.Names())
	}
	if _, err := runTodoCaptured(t, "migrate", "x"); err == nil {
		t.Error("migrate mit Argument angenommen")
	}
}

func pathExistsForTest(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
