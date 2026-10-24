package project

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
)

const legacyTodosJSON = `{
  "schemaVersion": 1,
  "nextId": 6,
  "migratedOn": "2026-09-08",
  "todos": [
    {"id": 1, "text": "Offen ohne Datum", "created": ""},
    {"id": 3, "text": "Erledigt aus der Markdown-Zeit", "created": "", "done": "2026-09-08", "doneMigrated": true},
    {"id": 4, "text": "Mit Herkunft", "created": "2026-09-09", "origin": "Task 055: Etappe 6"},
    {"id": 5, "text": "Von Hand erledigt", "created": "2026-09-10", "done": "2026-09-12"}
  ]
}`

func TestMigrateTodosErsterLauf(t *testing.T) {
	root := todoProject(t)
	store := newFakeTodoStore()
	useTodoStore(t, store)
	writeProjectFile(t, TodoFile(root), legacyTodosJSON)
	ctx := context.Background()

	result, err := MigrateTodos(ctx, root)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if result.Mode != TodoMigrationFirst || len(result.Created) != 4 || len(result.Skipped) != 0 ||
		result.RenamedTo != "k-playbook-local/data/todos.json.migriert" {
		t.Fatalf("Ergebnis: %+v", result)
	}
	want := map[string]string{
		"todos/001-offen-ohne-datum.md":                    "---\ncreated: \"\"\n---\n\nOffen ohne Datum\n",
		"todos/done/003-erledigt-aus-der-markdown-zeit.md": "---\ncreated: \"\"\ndone: 2026-09-08\ndoneMigrated: true\n---\n\nErledigt aus der Markdown-Zeit\n",
		"todos/004-mit-herkunft.md":                        "---\norigin: \"Task 055: Etappe 6\"\ncreated: 2026-09-09\n---\n\nMit Herkunft\n",
		"todos/done/005-von-hand-erledigt.md":              "---\ncreated: 2026-09-10\ndone: 2026-09-12\n---\n\nVon Hand erledigt\n",
	}
	for name, content := range want {
		doc, ok := store.docs[name]
		if !ok || doc.content != content {
			t.Errorf("%s: %+v, erwartet %q", name, doc, content)
		}
	}
	if fileExists(TodoFile(root)) || !fileExists(TodoMigratedFile(root)) {
		t.Fatal("todos.json nicht nach todos.json.migriert umbenannt")
	}

	// Nach der Übernahme: Felder wie vorher, Schreiben frei.
	all, notice, err := ListAllTodos(ctx, root)
	if err != nil || notice.Hint != "" || len(all) != 4 {
		t.Fatalf("Liste: %+v %+v %v", all, notice, err)
	}
	wantTodos := []Todo{
		{ID: 1, Text: "Offen ohne Datum", Created: ""},
		{ID: 3, Text: "Erledigt aus der Markdown-Zeit", Created: "", Done: "2026-09-08", DoneMigrated: true},
		{ID: 4, Text: "Mit Herkunft", Created: "2026-09-09", Origin: "Task 055: Etappe 6"},
		{ID: 5, Text: "Von Hand erledigt", Created: "2026-09-10", Done: "2026-09-12"},
	}
	for i := range wantTodos {
		if all[i] != wantTodos[i] {
			t.Errorf("Todo %d: %+v, erwartet %+v", i, all[i], wantTodos[i])
		}
	}
	if added, _, err := AddTodo(ctx, root, "Danach", ""); err != nil || added.ID != 6 {
		t.Fatalf("anlegen nach der Übernahme: %+v %v", added, err)
	}

	// Noch einmal: nichts mehr zu tun.
	again, err := MigrateTodos(ctx, root)
	if err != nil || again.Mode != TodoMigrationNone || len(again.Created) != 0 {
		t.Fatalf("zweiter Aufruf: %+v %v", again, err)
	}
}

func TestMigrateTodosWiederholbarNachAbbruch(t *testing.T) {
	root := todoProject(t)
	store := newFakeTodoStore()
	useTodoStore(t, store)
	writeProjectFile(t, TodoFile(root), legacyTodosJSON)
	ctx := context.Background()

	// Der dritte create scheitert offen: zwei sind angelegt, die Datei bleibt.
	failing := &failNthCreate{fakeTodoStore: store, n: 3}
	useTodoStore(t, failing)
	result, err := MigrateTodos(ctx, root)
	wantCode(t, err, DocCodeOutcomeUnknown)
	if len(result.Created) != 2 || result.RenamedTo != "" || !fileExists(TodoFile(root)) {
		t.Fatalf("Abbruch: %+v", result)
	}

	useTodoStore(t, store)
	result, err = MigrateTodos(ctx, root)
	if err != nil || len(result.Skipped) != 2 || len(result.Created) != 2 || result.RenamedTo == "" {
		t.Fatalf("Wiederholung: %+v %v", result, err)
	}
	if len(store.docs) != 4 {
		t.Fatalf("doppelt angelegt: %d Dokumente", len(store.docs))
	}
}

// failNthCreate lässt den n-ten create offen scheitern.
type failNthCreate struct {
	*fakeTodoStore
	n     int
	count int
}

func (s *failNthCreate) Create(ctx context.Context, name, content string) (kephalaion.Document, error) {
	s.count++
	if s.count == s.n {
		return kephalaion.Document{}, &kephalaion.ToolError{Tool: "create", Code: kephalaion.CodeOutcomeUnknown, Message: "offen"}
	}
	return s.fakeTodoStore.Create(ctx, name, content)
}

func TestMigrateTodosBrichtVorDemAnlegenAb(t *testing.T) {
	stamp := "2026-10-01T12:00:00.000Z"
	cases := map[string]struct {
		name, content, want string
	}{
		"fremde Nummer": {"todos/007-neu.md", "Neu angelegt\n", "007-neu.md trägt die Nummer 7"},
		"anderer Text":  {"todos/004-x.md", "Anderer Text\n", "Eintrag 4"},
	}
	for label, c := range cases {
		t.Run(label, func(t *testing.T) {
			root := todoProject(t)
			store := newFakeTodoStore()
			useTodoStore(t, store)
			writeProjectFile(t, TodoFile(root), legacyTodosJSON)
			store.put(c.name, c.content, stamp, stamp)
			store.calls = nil
			result, err := MigrateTodos(context.Background(), root)
			if err == nil || !strings.Contains(err.Error(), c.want) || !strings.Contains(err.Error(), "nichts angelegt") {
				t.Fatalf("Abbruch: %v", err)
			}
			if len(store.writes()) != 0 || len(result.Created) != 0 || !fileExists(TodoFile(root)) {
				t.Fatalf("trotz Abbruch geschrieben: %v %+v", store.writes(), result)
			}
		})
	}
}

func TestMigrateTodosNachzuegler(t *testing.T) {
	root := todoProject(t)
	store := newFakeTodoStore()
	useTodoStore(t, store)
	writeProjectFile(t, TodoFile(root), legacyTodosJSON)
	ctx := context.Background()
	if _, err := MigrateTodos(ctx, root); err != nil {
		t.Fatal(err)
	}
	migrated, err := os.ReadFile(TodoMigratedFile(root))
	if err != nil {
		t.Fatal(err)
	}

	// Ein altes Binary legt danach ein neues data/todos.json ab id 1 an.
	writeProjectFile(t, TodoFile(root), `{"schemaVersion":1,"nextId":3,"todos":[
		{"id":1,"text":"Offen ohne Datum","created":""},
		{"id":2,"text":"Vom alten Binary","created":"2026-10-20","done":"2026-10-20"}]}`)
	list, notice, err := ListTodos(ctx, root)
	if err != nil || len(list) != 2 || !strings.Contains(notice.Hint, "migrate") {
		t.Fatalf("Nachzügler wird gemeldet: %+v %q %v", list, notice.Hint, err)
	}
	result, err := MigrateTodos(ctx, root)
	if err != nil || result.Mode != TodoMigrationLatecomer {
		t.Fatalf("Nachzügler: %+v %v", result, err)
	}
	if len(result.Skipped) != 1 || result.Skipped[0].SourceID != 1 || len(result.Created) != 1 ||
		result.Created[0].ID != 6 || !strings.HasPrefix(result.Created[0].Name, "todos/done/006-") {
		t.Fatalf("Nachzügler-Ergebnis: %+v", result)
	}
	if !strings.HasPrefix(result.RenamedTo, "k-playbook-local/data/todos.json.nachzuegler-"+today()) {
		t.Fatalf("Name des Nachzüglers: %q", result.RenamedTo)
	}
	if after, _ := os.ReadFile(TodoMigratedFile(root)); string(after) != string(migrated) {
		t.Fatal("todos.json.migriert überschrieben")
	}

	// Ein zweiter Nachzügler am selben Tag bekommt einen eigenen Namen.
	writeProjectFile(t, TodoFile(root), `{"todos":[{"id":1,"text":"Noch einer","created":""}]}`)
	second, err := MigrateTodos(ctx, root)
	if err != nil || second.RenamedTo == result.RenamedTo || !strings.HasSuffix(second.RenamedTo, "-2") {
		t.Fatalf("zweiter Nachzügler: %+v %v", second, err)
	}
}

func TestMigrateTodosOhneDatei(t *testing.T) {
	root := todoProject(t)
	store := newFakeTodoStore()
	useTodoStore(t, store)
	result, err := MigrateTodos(context.Background(), root)
	if err != nil || result.Mode != TodoMigrationNone || len(store.calls) != 0 {
		t.Fatalf("ohne Datei: %+v %v %v", result, err, store.calls)
	}
	writeProjectFile(t, TodoFile(root), `{"todos":[{"id":1,"text":"a"},{"id":1,"text":"b"}]}`)
	if _, err := MigrateTodos(context.Background(), root); err == nil || !strings.Contains(err.Error(), "zweimal") {
		t.Fatalf("doppelte Kennung: %v", err)
	}
}
