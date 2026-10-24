package mcpserver

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kascada/k-playbook/installer/internal/kephalaion/kephtest"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// newTodoProject legt ein k-playbook-Projekt an, dessen Todos in einer
// Collection im Speicher liegen (kephtest.Store statt des Nodes).
func newTodoProject(t *testing.T) (string, *kephtest.Store) {
	t.Helper()

	root := t.TempDir()
	if err := project.CreateConfig(root, "."); err != nil {
		t.Fatalf("Konfiguration anlegen: %v", err)
	}
	if err := os.MkdirAll(project.LocalDir(root), 0o755); err != nil {
		t.Fatalf("k-playbook-local anlegen: %v", err)
	}
	store := kephtest.New()
	original := project.OpenTodoStore
	project.OpenTodoStore = func(string) (project.TodoStore, string, error) { return store, "kamran", nil }
	t.Cleanup(func() { project.OpenTodoStore = original })
	return root, store
}

func decodeTodoEnvelope(t *testing.T, result *mcp.CallToolResult) todoEnvelope {
	t.Helper()

	if len(result.Content) != 1 {
		t.Fatalf("erwartet genau einen Inhaltsblock, bekommen %d", len(result.Content))
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("Inhalt ist kein Text: %#v", result.Content[0])
	}
	envelope := todoEnvelope{}
	if err := json.Unmarshal([]byte(text.Text), &envelope); err != nil {
		t.Fatalf("Antwort ist kein JSON: %v — %s", err, text.Text)
	}
	return envelope
}

func TestTodoListTrenntOffeneUndErledigteUndSucht(t *testing.T) {
	root, store := newTodoProject(t)
	store.Put("todos/001-offen.md", "Offen und gesucht\n")
	store.Put("todos/done/002-erledigt.md", "---\ndone: 2026-10-01\n---\n\nErledigt und gesucht\n")
	store.Put("todos/003-anders.md", "Anders\n")
	ctx := context.Background()
	base := todoBaseInput{ProjectDir: root}

	result, _, _ := todoListTool(ctx, nil, todoListInput{todoBaseInput: base})
	envelope := decodeTodoEnvelope(t, result)
	if !envelope.OK || len(envelope.Todos) != 2 || envelope.Todos[0].ID != 1 || envelope.Todos[1].ID != 3 {
		t.Fatalf("offene Einträge: %+v %+v", envelope.Todos, envelope.Error)
	}
	result, _, _ = todoListTool(ctx, nil, todoListInput{todoBaseInput: base, IncludeDone: true})
	if envelope := decodeTodoEnvelope(t, result); len(envelope.Todos) != 3 || envelope.Todos[1].Done != "2026-10-01" {
		t.Fatalf("alle Einträge: %+v", envelope.Todos)
	}
	result, _, _ = todoListTool(ctx, nil, todoListInput{todoBaseInput: base, Search: "GESUCHT"})
	if envelope := decodeTodoEnvelope(t, result); len(envelope.Todos) != 1 || envelope.Todos[0].ID != 1 {
		t.Fatalf("Suche über offene: %+v", envelope.Todos)
	}
	result, _, _ = todoListTool(ctx, nil, todoListInput{todoBaseInput: base, Search: "gesucht", IncludeDone: true})
	if envelope := decodeTodoEnvelope(t, result); len(envelope.Todos) != 2 {
		t.Fatalf("Suche über alle: %+v", envelope.Todos)
	}
}

func TestTodoWerkzeugeMeldenZustaende(t *testing.T) {
	root, store := newTodoProject(t)
	ctx := context.Background()
	base := todoBaseInput{ProjectDir: root}
	if err := os.MkdirAll(project.TodoDataDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(project.TodoFile(root), []byte(`{"todos":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	result, _, _ := todoListTool(ctx, nil, todoListInput{todoBaseInput: base})
	envelope := decodeTodoEnvelope(t, result)
	if !envelope.OK || !strings.Contains(envelope.Hint, "k-playbook todo migrate") {
		t.Fatalf("Liste neben data/todos.json: %+v", envelope)
	}
	result, _, _ = todoAddTool(ctx, nil, todoAddInput{todoBaseInput: base, Text: "neu"})
	envelope = decodeTodoEnvelope(t, result)
	if envelope.OK || envelope.Error == nil || envelope.Error.Code != project.TodoCodeNotMigrated || envelope.Hint == "" || !result.IsError {
		t.Fatalf("add vor der Übernahme: %+v", envelope)
	}
	if len(store.Names()) != 0 {
		t.Fatalf("trotz Sperre angelegt: %v", store.Names())
	}

	project.OpenTodoStore = func(string) (project.TodoStore, string, error) {
		_, err := project.OpenUserDocs(root)
		return nil, "", err
	}
	result, _, _ = todoListTool(ctx, nil, todoListInput{todoBaseInput: base})
	envelope = decodeTodoEnvelope(t, result)
	if envelope.OK || envelope.Error == nil || envelope.Error.Code != project.DocCodeKephalaionUnavailable {
		t.Fatalf("ohne Kephalaion: %+v", envelope)
	}
}

func TestTodoAddLehntLeerenTextAb(t *testing.T) {
	root, _ := newTodoProject(t)

	result, _, err := todoAddTool(context.Background(), nil, todoAddInput{
		todoBaseInput: todoBaseInput{ProjectDir: root}, Text: "  ",
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	envelope := decodeTodoEnvelope(t, result)
	if envelope.OK || envelope.Error == nil || envelope.Error.Code != "invalid_input" {
		t.Fatalf("leerer Text wurde angenommen: %#v", envelope)
	}
	if !result.IsError {
		t.Error("Ergebnis ist nicht als Werkzeugfehler markiert")
	}
}

func TestTodoUpdateHaktAbUndOeffnetWieder(t *testing.T) {
	root, store := newTodoProject(t)

	result, _, err := todoAddTool(context.Background(), nil, todoAddInput{
		todoBaseInput: todoBaseInput{ProjectDir: root}, Text: "Erster", Origin: "Task 053",
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	added := decodeTodoEnvelope(t, result)
	if !added.OK || added.Todo == nil || added.Todo.ID != 1 || added.Todo.Origin != "Task 053" {
		t.Fatalf("angelegter Eintrag: %#v", added)
	}

	done := true
	result, _, err = todoUpdateTool(context.Background(), nil, todoUpdateInput{
		todoBaseInput: todoBaseInput{ProjectDir: root}, ID: 1, Done: &done,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	updated := decodeTodoEnvelope(t, result)
	if !updated.OK || updated.Todo.Done == "" || updated.Todo.DoneMigrated {
		t.Fatalf("abgehakter Eintrag: %#v", updated.Todo)
	}
	if names := store.Names(); len(names) != 1 || names[0] != "todos/done/001-erster.md" {
		t.Fatalf("nicht nach done/ verschoben: %v", names)
	}

	done = false
	result, _, err = todoUpdateTool(context.Background(), nil, todoUpdateInput{
		todoBaseInput: todoBaseInput{ProjectDir: root}, ID: 1, Done: &done,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	reopened := decodeTodoEnvelope(t, result)
	if !reopened.OK || reopened.Todo.Done != "" {
		t.Fatalf("wieder geöffneter Eintrag: %#v", reopened.Todo)
	}
}

func TestTodoUpdateUndDeleteMeldenUnbekannteKennung(t *testing.T) {
	root, store := newTodoProject(t)
	store.Put("todos/001-erster.md", "Erster\n")

	done := true
	result, _, err := todoUpdateTool(context.Background(), nil, todoUpdateInput{
		todoBaseInput: todoBaseInput{ProjectDir: root}, ID: 99, Done: &done,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if envelope := decodeTodoEnvelope(t, result); envelope.OK || envelope.Error.Code != "not_found" {
		t.Errorf("unbekannte Kennung wurde angenommen: %#v", envelope)
	}

	result, _, err = todoDeleteTool(context.Background(), nil, todoDeleteInput{
		todoBaseInput: todoBaseInput{ProjectDir: root}, ID: 99,
	})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if envelope := decodeTodoEnvelope(t, result); envelope.OK || envelope.Error.Code != "not_found" {
		t.Errorf("unbekannte Kennung wurde angenommen: %#v", envelope)
	}
}

// Löschen entfernt den Eintrag; die höchste Nummer wird danach wieder
// vergeben (Task 097: eine Löschmarke sperrt keinen Namen).
func TestTodoDeleteGibtHoechsteNummerFrei(t *testing.T) {
	root, store := newTodoProject(t)
	store.Put("todos/001-erster.md", "Erster\n")
	store.Put("todos/002-zweiter.md", "Zweiter\n")

	result, _, err := todoDeleteTool(context.Background(), nil, todoDeleteInput{
		todoBaseInput: todoBaseInput{ProjectDir: root}, ID: 2,
	})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	deleted := decodeTodoEnvelope(t, result)
	if !deleted.OK || deleted.Todo.ID != 2 || deleted.Todo.Text != "Zweiter" {
		t.Fatalf("gelöschter Eintrag: %#v", deleted)
	}

	result, _, err = todoAddTool(context.Background(), nil, todoAddInput{
		todoBaseInput: todoBaseInput{ProjectDir: root}, Text: "Dritter",
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	added := decodeTodoEnvelope(t, result)
	if added.Todo.ID != 2 {
		t.Fatalf("ID = %d, erwartet 2", added.Todo.ID)
	}
}

func TestTodoToolOhneProjektDir(t *testing.T) {
	result, _, err := todoListTool(context.Background(), nil, todoListInput{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	envelope := decodeTodoEnvelope(t, result)
	if envelope.OK || envelope.Error == nil || envelope.Error.Code != "invalid_input" {
		t.Fatalf("erwartet invalid_input, bekommen %#v", envelope)
	}
	if !strings.Contains(envelope.Error.Message, "nichts ausgeführt") {
		t.Errorf("Meldung sagt nicht, dass nichts ausgeführt wurde: %q", envelope.Error.Message)
	}
}
