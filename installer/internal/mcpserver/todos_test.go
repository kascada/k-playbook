package mcpserver

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// newTodoProject legt ein k-playbook-Projekt mit einer zu migrierenden
// TODO.md an. Der Dateiname steht hier bewusst: die Markdown-Ablage ist nur
// noch Quelle der Migration.
func newTodoProject(t *testing.T, legacy string) string {
	t.Helper()

	root := t.TempDir()
	if err := project.CreateConfig(root, "."); err != nil {
		t.Fatalf("Konfiguration anlegen: %v", err)
	}
	if err := os.MkdirAll(project.LocalDir(root), 0o755); err != nil {
		t.Fatalf("k-playbook-local anlegen: %v", err)
	}
	if legacy != "" {
		if err := os.WriteFile(project.LegacyTodoFile(root), []byte(legacy), 0o644); err != nil {
			t.Fatalf("TODO.md anlegen: %v", err)
		}
	}
	return root
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

func TestTodoListMigriertUndTrenntOffeneUndErledigte(t *testing.T) {
	root := newTodoProject(t, "- [ ] Offen\n- [x] Erledigt\n")

	result, _, err := todoListTool(context.Background(), nil, todoListInput{todoBaseInput: todoBaseInput{ProjectDir: root}})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	envelope := decodeTodoEnvelope(t, result)
	if !envelope.OK {
		t.Fatalf("list fehlgeschlagen: %#v", envelope.Error)
	}
	if envelope.Migrated != 2 {
		t.Errorf("Migrated = %d, erwartet 2", envelope.Migrated)
	}
	if len(envelope.Todos) != 1 || envelope.Todos[0].Text != "Offen" {
		t.Fatalf("offene Einträge: %+v", envelope.Todos)
	}

	result, _, err = todoListTool(context.Background(), nil, todoListInput{
		todoBaseInput: todoBaseInput{ProjectDir: root}, IncludeDone: true,
	})
	if err != nil {
		t.Fatalf("list includeDone: %v", err)
	}
	envelope = decodeTodoEnvelope(t, result)
	if len(envelope.Todos) != 2 {
		t.Fatalf("alle Einträge: %+v", envelope.Todos)
	}
	if envelope.Migrated != 0 {
		t.Errorf("Migrated = %d, erwartet 0 — es war nichts mehr zu migrieren", envelope.Migrated)
	}
}

func TestTodoAddLehntLeerenTextAb(t *testing.T) {
	root := newTodoProject(t, "")

	result, _, err := todoAddTool(context.Background(), nil, todoAddInput{
		todoBaseInput: todoBaseInput{ProjectDir: root}, Text: "  ",
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	envelope := decodeTodoEnvelope(t, result)
	if envelope.OK || envelope.Error == nil {
		t.Fatalf("leerer Text wurde angenommen: %#v", envelope)
	}
	if !result.IsError {
		t.Error("Ergebnis ist nicht als Werkzeugfehler markiert")
	}
}

func TestTodoUpdateHaktAbUndOeffnetWieder(t *testing.T) {
	root := newTodoProject(t, "")

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
	root := newTodoProject(t, "- [ ] Erster\n")

	done := true
	result, _, err := todoUpdateTool(context.Background(), nil, todoUpdateInput{
		todoBaseInput: todoBaseInput{ProjectDir: root}, ID: 99, Done: &done,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if envelope := decodeTodoEnvelope(t, result); envelope.OK {
		t.Errorf("unbekannte Kennung wurde angenommen: %#v", envelope)
	}

	result, _, err = todoDeleteTool(context.Background(), nil, todoDeleteInput{
		todoBaseInput: todoBaseInput{ProjectDir: root}, ID: 99,
	})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if envelope := decodeTodoEnvelope(t, result); envelope.OK {
		t.Errorf("unbekannte Kennung wurde angenommen: %#v", envelope)
	}
}

// Löschen entfernt den Eintrag; die Kennung bleibt verbraucht.
func TestTodoDeleteHaeltNextIdFest(t *testing.T) {
	root := newTodoProject(t, "- [ ] Erster\n- [ ] Zweiter\n")

	result, _, err := todoDeleteTool(context.Background(), nil, todoDeleteInput{
		todoBaseInput: todoBaseInput{ProjectDir: root}, ID: 2,
	})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	deleted := decodeTodoEnvelope(t, result)
	if !deleted.OK || deleted.Todo.ID != 2 {
		t.Fatalf("gelöschter Eintrag: %#v", deleted)
	}

	result, _, err = todoAddTool(context.Background(), nil, todoAddInput{
		todoBaseInput: todoBaseInput{ProjectDir: root}, Text: "Dritter",
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	added := decodeTodoEnvelope(t, result)
	if added.Todo.ID != 3 {
		t.Fatalf("ID = %d, erwartet 3 — eine gelöschte Kennung wurde neu vergeben", added.Todo.ID)
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
