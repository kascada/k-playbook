package project

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
)

// fakeTodoDoc ist ein Dokument im gefälschten Kephalaion der Todo-Tests.
type fakeTodoDoc struct {
	content  string
	revision int64
	created  string
	updated  string
}

// fakeTodoStore ist ein Kephalaion im Speicher mit Inhalt und Revisionen:
// create prüft den Namen, write und delete die Revision, rename das Ziel —
// wie der Hub.
type fakeTodoStore struct {
	docs     map[string]*fakeTodoDoc
	revision int64
	// failOn lässt das Werkzeug mit diesem Namen mit dem Fehler scheitern.
	failOn map[string]error
	calls  []string
}

func newFakeTodoStore() *fakeTodoStore {
	return &fakeTodoStore{docs: map[string]*fakeTodoDoc{}, failOn: map[string]error{}}
}

// put legt ein Dokument direkt an, ohne Aufruf.
func (s *fakeTodoStore) put(name, content, created, updated string) {
	s.revision++
	s.docs[name] = &fakeTodoDoc{content: content, revision: s.revision, created: created, updated: updated}
}

func (s *fakeTodoStore) record(call string) error {
	s.calls = append(s.calls, call)
	tool, _, _ := strings.Cut(call, " ")
	return s.failOn[tool]
}

func (s *fakeTodoStore) writes() []string {
	var out []string
	for _, call := range s.calls {
		if !strings.HasPrefix(call, "list") && !strings.HasPrefix(call, "read") {
			out = append(out, call)
		}
	}
	return out
}

func (s *fakeTodoStore) List(_ context.Context, req kephalaion.ListRequest) ([]kephalaion.Entry, error) {
	if err := s.record("list " + req.Path); err != nil {
		return nil, err
	}
	var out []kephalaion.Entry
	for name, doc := range s.docs {
		if !strings.HasPrefix(name, req.Path+"/") {
			continue
		}
		out = append(out, kephalaion.Entry{Kind: "document", Name: name, Revision: doc.revision,
			Created: &kephalaion.Stamp{At: doc.created, By: "kamran"}, Updated: &kephalaion.Stamp{At: doc.updated, By: "kamran"}})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (s *fakeTodoStore) Read(_ context.Context, name string) (kephalaion.ReadResult, error) {
	if err := s.record("read " + name); err != nil {
		return kephalaion.ReadResult{}, err
	}
	doc, ok := s.docs[name]
	if !ok {
		return kephalaion.ReadResult{Kind: kephalaion.ReadKindNone, Name: name}, nil
	}
	content := doc.content
	return kephalaion.ReadResult{Kind: kephalaion.ReadKindDocument, Name: name, Revision: doc.revision, Content: &content,
		Created: &kephalaion.Stamp{At: doc.created}, Updated: &kephalaion.Stamp{At: doc.updated}}, nil
}

func (s *fakeTodoStore) Create(_ context.Context, name, content string) (kephalaion.Document, error) {
	if err := s.record("create " + name); err != nil {
		return kephalaion.Document{}, err
	}
	if _, ok := s.docs[name]; ok {
		return kephalaion.Document{}, &kephalaion.ToolError{Tool: "create", Code: kephalaion.CodeNameTaken, Message: "belegt"}
	}
	s.put(name, content, "2026-10-20T08:00:00.000Z", "2026-10-20T08:00:00.000Z")
	return kephalaion.Document{Name: name, Revision: s.revision, Created: &kephalaion.Stamp{At: "2026-10-20T08:00:00.000Z"}}, nil
}

func (s *fakeTodoStore) Write(_ context.Context, name, content string, base int64) (kephalaion.Document, error) {
	if err := s.record(fmt.Sprintf("write %s @%d", name, base)); err != nil {
		return kephalaion.Document{}, err
	}
	doc, ok := s.docs[name]
	if !ok {
		return kephalaion.Document{}, &kephalaion.ToolError{Tool: "write", Code: kephalaion.CodeNotFound, Message: "fehlt"}
	}
	if base > 0 && base != doc.revision {
		return kephalaion.Document{}, &kephalaion.ToolError{Tool: "write", Code: kephalaion.CodeStaleRevision,
			Message: fmt.Sprintf("Revision %d, nicht %d", doc.revision, base)}
	}
	s.revision++
	doc.content, doc.revision, doc.updated = content, s.revision, "2026-10-20T09:00:00.000Z"
	return kephalaion.Document{Name: name, Revision: doc.revision}, nil
}

func (s *fakeTodoStore) Rename(_ context.Context, name, newName string) (kephalaion.Document, error) {
	if err := s.record("rename " + name + " " + newName); err != nil {
		return kephalaion.Document{}, err
	}
	doc, ok := s.docs[name]
	if !ok {
		return kephalaion.Document{}, &kephalaion.ToolError{Tool: "rename", Code: kephalaion.CodeNotFound, Message: "fehlt"}
	}
	if _, ok := s.docs[newName]; ok {
		return kephalaion.Document{}, &kephalaion.ToolError{Tool: "rename", Code: kephalaion.CodeNameTaken, Message: "Ziel belegt"}
	}
	delete(s.docs, name)
	s.revision++
	doc.revision, doc.updated = s.revision, "2026-10-20T10:00:00.000Z"
	s.docs[newName] = doc
	return kephalaion.Document{Name: newName, Revision: doc.revision}, nil
}

func (s *fakeTodoStore) Delete(_ context.Context, name string, base int64) (kephalaion.Document, error) {
	if err := s.record(fmt.Sprintf("delete %s @%d", name, base)); err != nil {
		return kephalaion.Document{}, err
	}
	doc, ok := s.docs[name]
	if !ok {
		return kephalaion.Document{}, &kephalaion.ToolError{Tool: "delete", Code: kephalaion.CodeNotFound, Message: "fehlt"}
	}
	if base > 0 && base != doc.revision {
		return kephalaion.Document{}, &kephalaion.ToolError{Tool: "delete", Code: kephalaion.CodeStaleRevision, Message: "veraltet"}
	}
	delete(s.docs, name)
	s.revision++
	return kephalaion.Document{Name: name, Revision: s.revision, Deleted: true}, nil
}

// useTodoStore setzt den gefälschten Store für alle Todo-Funktionen ein.
func useTodoStore(t *testing.T, store TodoStore) {
	t.Helper()
	original := OpenTodoStore
	OpenTodoStore = func(string) (TodoStore, string, error) { return store, "kamran", nil }
	t.Cleanup(func() { OpenTodoStore = original })
}

// todoProject legt ein leeres Projekt an.
func todoProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(LocalDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func writeProjectFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func boolPtr(value bool) *bool { return &value }

func TestTodosAnlegenAbhakenWiederoeffnenLoeschen(t *testing.T) {
	root := todoProject(t)
	store := newFakeTodoStore()
	useTodoStore(t, store)
	ctx := context.Background()

	first, notice, err := AddTodo(ctx, root, "  Erster Punkt mit Ümläut und mehr als sechs Wörtern im Text ", "Task 097")
	if err != nil || notice.Hint != "" {
		t.Fatalf("AddTodo: %v %+v", err, notice)
	}
	if first.ID != 1 || first.Text != "Erster Punkt mit Ümläut und mehr als sechs Wörtern im Text" || first.Origin != "Task 097" ||
		first.Created != stampDate("2026-10-20T08:00:00.000Z") || first.Done != "" {
		t.Fatalf("erstes Todo: %+v", first)
	}
	name := "todos/001-erster-punkt-mit-uemlaeut-und-mehr.md"
	doc, ok := store.docs[name]
	if !ok {
		t.Fatalf("Dokument fehlt; vorhanden: %v", store.calls)
	}
	if doc.content != "---\norigin: Task 097\n---\n\nErster Punkt mit Ümläut und mehr als sechs Wörtern im Text\n" {
		t.Fatalf("Inhalt: %q", doc.content)
	}
	second, _, err := AddTodo(ctx, root, "Zweiter", "")
	if err != nil || second.ID != 2 {
		t.Fatalf("zweites Todo: %+v %v", second, err)
	}
	if store.docs["todos/002-zweiter.md"].content != "Zweiter\n" {
		t.Fatalf("Inhalt ohne Felder: %q", store.docs["todos/002-zweiter.md"].content)
	}

	open, _, err := ListTodos(ctx, root)
	if err != nil || len(open) != 2 || open[0].ID != 1 || open[1].ID != 2 {
		t.Fatalf("offene: %+v %v", open, err)
	}

	// Abhaken: erst done: schreiben, dann verschieben.
	store.calls = nil
	done, _, err := UpdateTodo(ctx, root, 1, nil, boolPtr(true))
	if err != nil || done.Done != today() || done.DoneMigrated {
		t.Fatalf("abhaken: %+v %v", done, err)
	}
	writes := store.writes()
	if len(writes) != 2 || !strings.HasPrefix(writes[0], "write "+name+" @") ||
		writes[1] != "rename "+name+" todos/done/001-erster-punkt-mit-uemlaeut-und-mehr.md" {
		t.Fatalf("Reihenfolge beim Abhaken: %v", writes)
	}
	doneDoc := store.docs["todos/done/001-erster-punkt-mit-uemlaeut-und-mehr.md"]
	if doneDoc == nil || !strings.Contains(doneDoc.content, "done: "+today()+"\n") {
		t.Fatalf("done: fehlt: %+v", doneDoc)
	}
	doneList, _, err := ListDoneTodos(ctx, root)
	if err != nil || len(doneList) != 1 || doneList[0].ID != 1 || doneList[0].Done != today() {
		t.Fatalf("erledigte: %+v %v", doneList, err)
	}
	if open, _, _ := ListTodos(ctx, root); len(open) != 1 || open[0].ID != 2 {
		t.Fatalf("offen nach Abhaken: %+v", open)
	}

	// Wiederöffnen: erst zurück, dann das Feld entfernen.
	store.calls = nil
	reopened, _, err := UpdateTodo(ctx, root, 1, nil, boolPtr(false))
	if err != nil || reopened.Done != "" {
		t.Fatalf("wiederöffnen: %+v %v", reopened, err)
	}
	writes = store.writes()
	if len(writes) != 2 || !strings.HasPrefix(writes[0], "rename todos/done/") || !strings.HasPrefix(writes[1], "write "+name) {
		t.Fatalf("Reihenfolge beim Wiederöffnen: %v", writes)
	}
	if strings.Contains(store.docs[name].content, "done:") {
		t.Fatalf("done: nicht entfernt: %q", store.docs[name].content)
	}

	neu := "Erster Punkt, neu gefasst"
	changed, _, err := UpdateTodo(ctx, root, 1, &neu, nil)
	if err != nil || changed.Text != neu || changed.Origin != "Task 097" {
		t.Fatalf("Text ändern: %+v %v", changed, err)
	}
	if !strings.HasSuffix(store.docs[name].content, "\n\n"+neu+"\n") {
		t.Fatalf("Text nicht geschrieben: %q", store.docs[name].content)
	}

	// Löschen mit der Revision aus dem Lesen; die höchste Nummer wird danach
	// wieder vergeben.
	store.calls = nil
	removed, _, err := DeleteTodo(ctx, root, 2)
	if err != nil || removed.ID != 2 || removed.Text != "Zweiter" {
		t.Fatalf("löschen: %+v %v", removed, err)
	}
	if writes := store.writes(); len(writes) != 1 || !strings.HasPrefix(writes[0], "delete todos/002-zweiter.md @") ||
		strings.HasSuffix(writes[0], "@0") {
		t.Fatalf("delete: %v", writes)
	}
	again, _, err := AddTodo(ctx, root, "Nach dem Löschen", "")
	if err != nil || again.ID != 2 {
		t.Fatalf("Nummer nach dem Löschen: %+v %v", again, err)
	}

	_, _, err = UpdateTodo(ctx, root, 99, nil, boolPtr(true))
	wantCode(t, err, DocCodeNotFound)
	_, _, err = DeleteTodo(ctx, root, 99)
	wantCode(t, err, DocCodeNotFound)
	_, _, err = AddTodo(ctx, root, "   ", "")
	wantCode(t, err, DocCodeInvalidInput)
	blank := " "
	_, _, err = UpdateTodo(ctx, root, 1, &blank, nil)
	wantCode(t, err, DocCodeInvalidInput)
	_, _, err = UpdateTodo(ctx, root, 1, nil, nil)
	wantCode(t, err, DocCodeInvalidInput)
}

func TestTodosCreatedUndDoneNachDenRegeln(t *testing.T) {
	root := todoProject(t)
	store := newFakeTodoStore()
	useTodoStore(t, store)
	ctx := context.Background()
	stamp := "2026-10-01T12:00:00.000Z"
	store.put("todos/001-leer.md", "---\ncreated: \"\"\n---\n\nÜbernommen ohne Datum\n", stamp, stamp)
	store.put("todos/002-datum.md", "---\ncreated: 2026-09-08\n---\n\nÜbernommen mit Datum\n", stamp, stamp)
	store.put("todos/003-neu.md", "Neu angelegt\n", stamp, stamp)
	store.put("todos/004-rest.md", "---\ndone: 2026-10-02\ndoneMigrated: true\n---\n\nÜbrig gebliebenes done\n", stamp, stamp)
	store.put("todos/done/005-ohne.md", "Erledigt ohne done\n", stamp, "2026-10-05T12:00:00.000Z")
	store.put("todos/done/006-mig.md", "---\ndone: 2026-09-08\ndoneMigrated: true\n---\n\nMigriert erledigt\n", stamp, stamp)
	store.put("todos/README.md", "kein Todo\n", stamp, stamp)

	all, _, err := ListAllTodos(ctx, root)
	if err != nil || len(all) != 6 {
		t.Fatalf("alle: %+v %v", all, err)
	}
	want := []Todo{
		{ID: 1, Text: "Übernommen ohne Datum", Created: ""},
		{ID: 2, Text: "Übernommen mit Datum", Created: "2026-09-08"},
		{ID: 3, Text: "Neu angelegt", Created: stampDate(stamp)},
		{ID: 4, Text: "Übrig gebliebenes done", Created: stampDate(stamp)},
		{ID: 5, Text: "Erledigt ohne done", Created: stampDate(stamp), Done: stampDate("2026-10-05T12:00:00.000Z")},
		{ID: 6, Text: "Migriert erledigt", Created: stampDate(stamp), Done: "2026-09-08", DoneMigrated: true},
	}
	for i := range want {
		if all[i] != want[i] {
			t.Errorf("Todo %d: %+v, erwartet %+v", i+1, all[i], want[i])
		}
	}

	// Das nächste Schreiben bereinigt: done: unter todos/ fällt weg, unter
	// done/ kommt das gezeigte Datum dazu.
	text4 := "Übrig gebliebenes done, bereinigt"
	if _, _, err := UpdateTodo(ctx, root, 4, &text4, nil); err != nil {
		t.Fatal(err)
	}
	if c := store.docs["todos/004-rest.md"].content; c != text4+"\n" {
		t.Fatalf("done: unter todos/ nicht bereinigt: %q", c)
	}
	text5 := "Erledigt ohne done, bereinigt"
	updated, _, err := UpdateTodo(ctx, root, 5, &text5, nil)
	if err != nil || updated.Done != stampDate("2026-10-05T12:00:00.000Z") {
		t.Fatalf("Todo 5: %+v %v", updated, err)
	}
	if c := store.docs["todos/done/005-ohne.md"].content; !strings.Contains(c, "done: "+stampDate("2026-10-05T12:00:00.000Z")) {
		t.Fatalf("done: unter done/ nicht nachgetragen: %q", c)
	}
	// Abhaken eines schon Erledigten setzt das heutige Datum und entfernt
	// doneMigrated, ohne zu verschieben.
	store.calls = nil
	redone, _, err := UpdateTodo(ctx, root, 6, nil, boolPtr(true))
	if err != nil || redone.Done != today() || redone.DoneMigrated {
		t.Fatalf("Todo 6: %+v %v", redone, err)
	}
	if writes := store.writes(); len(writes) != 1 || !strings.HasPrefix(writes[0], "write todos/done/006-mig.md") {
		t.Fatalf("Todo 6: %v", writes)
	}
	// Übernommenes created bleibt beim Abhaken stehen, auch leer.
	if done1, _, err := UpdateTodo(ctx, root, 1, nil, boolPtr(true)); err != nil || done1.Created != "" {
		t.Fatalf("Todo 1: %+v %v", done1, err)
	}
	if c := store.docs["todos/done/001-leer.md"].content; !strings.Contains(c, "created: \"\"\n") {
		t.Fatalf("leeres created verloren: %q", c)
	}
}

func TestTodosSuche(t *testing.T) {
	root := todoProject(t)
	store := newFakeTodoStore()
	useTodoStore(t, store)
	stamp := "2026-10-01T12:00:00.000Z"
	store.put("todos/001-a.md", "Kephalaion anbinden\n", stamp, stamp)
	store.put("todos/002-b.md", "GUI aufräumen\n", stamp, stamp)
	store.put("todos/done/003-c.md", "---\ndone: 2026-10-02\n---\n\nDie kephalaion-Doku\n", stamp, stamp)

	found, _, err := SearchTodos(context.Background(), root, "  KEPHALAION ")
	if err != nil || len(found) != 2 || found[0].ID != 1 || found[1].ID != 3 || found[1].Done != "2026-10-02" {
		t.Fatalf("Suche: %+v %v", found, err)
	}
	if found, _, _ := SearchTodos(context.Background(), root, "nirgends"); len(found) != 0 {
		t.Fatalf("Suche ohne Treffer: %+v", found)
	}
	if found, _, _ := SearchTodos(context.Background(), root, ""); len(found) != 3 {
		t.Fatalf("leere Suche: %+v", found)
	}
}

func TestTodosNichtUebernommenSperrtSchreiben(t *testing.T) {
	root := todoProject(t)
	store := newFakeTodoStore()
	useTodoStore(t, store)
	store.put("todos/001-a.md", "Schon in der Collection\n", "2026-10-01T12:00:00.000Z", "2026-10-01T12:00:00.000Z")
	writeProjectFile(t, TodoFile(root), `{"schemaVersion":1,"nextId":2,"todos":[{"id":1,"text":"alt","created":""}]}`)
	ctx := context.Background()

	todos, notice, err := ListAllTodos(ctx, root)
	if err != nil || len(todos) != 1 || todos[0].Text != "Schon in der Collection" {
		t.Fatalf("Liste liest data/todos.json oder scheitert: %+v %v", todos, err)
	}
	if !strings.Contains(notice.Hint, "k-playbook todo migrate") || !strings.Contains(notice.Hint, TodoCodeNotMigrated) {
		t.Fatalf("Hinweis: %q", notice.Hint)
	}
	store.calls = nil
	_, notice, err = AddTodo(ctx, root, "neu", "")
	wantCode(t, err, TodoCodeNotMigrated)
	if !strings.Contains(notice.Hint, "migrate") || !strings.Contains(err.Error(), "k-playbook todo migrate") {
		t.Fatalf("Ablehnung ohne Ausweg: %v / %q", err, notice.Hint)
	}
	_, _, err = UpdateTodo(ctx, root, 1, nil, boolPtr(true))
	wantCode(t, err, TodoCodeNotMigrated)
	_, _, err = DeleteTodo(ctx, root, 1)
	wantCode(t, err, TodoCodeNotMigrated)
	if len(store.calls) != 0 {
		t.Fatalf("trotz Sperre Kephalaion gefragt: %v", store.calls)
	}
}

func TestTodosTodoMdWirdNurGemeldet(t *testing.T) {
	root := todoProject(t)
	useTodoStore(t, newFakeTodoStore())
	writeProjectFile(t, LegacyTodoFile(root), "- [ ] Aus der Markdown-Datei\n")

	todos, notice, err := ListTodos(context.Background(), root)
	if err != nil || len(todos) != 0 {
		t.Fatalf("Liste: %+v %v", todos, err)
	}
	if !strings.Contains(notice.Hint, "TODO.md") || !strings.Contains(notice.Hint, "nicht übernommen") ||
		strings.Contains(notice.Hint, "import") {
		t.Fatalf("Hinweis: %q", notice.Hint)
	}
	if _, err := os.Stat(LegacyTodoFile(root)); err != nil {
		t.Fatalf("TODO.md angefasst: %v", err)
	}
	// Schreiben ist nicht gesperrt: TODO.md wird nicht übernommen.
	if _, _, err := AddTodo(context.Background(), root, "neu", ""); err != nil {
		t.Fatalf("AddTodo neben TODO.md: %v", err)
	}
}

func TestTodosOhneKephalaion(t *testing.T) {
	root := todoProject(t)
	original := OpenTodoStore
	OpenTodoStore = func(string) (TodoStore, string, error) {
		return nil, "", docRefErrorf(DocCodeKephalaionUnavailable, "Kephalaion ist für dieses Projekt nicht bereit")
	}
	t.Cleanup(func() { OpenTodoStore = original })
	ctx := context.Background()
	_, _, err := ListTodos(ctx, root)
	wantCode(t, err, DocCodeKephalaionUnavailable)
	_, _, err = AddTodo(ctx, root, "x", "")
	wantCode(t, err, DocCodeKephalaionUnavailable)
	_, _, err = SearchTodos(ctx, root, "x")
	wantCode(t, err, DocCodeKephalaionUnavailable)

	// Der echte Weg ohne Anbindung: dasselbe.
	OpenTodoStore = original
	_, _, err = ListTodos(ctx, root)
	wantCode(t, err, DocCodeKephalaionUnavailable)
}

func TestTodosZweiterSchrittScheitert(t *testing.T) {
	root := todoProject(t)
	store := newFakeTodoStore()
	useTodoStore(t, store)
	stamp := "2026-10-01T12:00:00.000Z"
	store.put("todos/001-a.md", "A\n", stamp, stamp)
	store.failOn["rename"] = &kephalaion.ToolError{Tool: "rename", Code: kephalaion.CodeOutcomeUnknown, Message: "Ausgang offen"}

	_, _, err := UpdateTodo(context.Background(), root, 1, nil, boolPtr(true))
	wantCode(t, err, DocCodeOutcomeUnknown)
	if !strings.Contains(err.Error(), "done/") || !strings.Contains(err.Error(), "k-playbook todo list --all") {
		t.Fatalf("Meldung: %v", err)
	}
	// Der erste Schritt steht; die Liste übergeht das Feld.
	if !strings.Contains(store.docs["todos/001-a.md"].content, "done: ") {
		t.Fatal("done: nicht geschrieben")
	}
	delete(store.failOn, "rename")
	open, _, err := ListTodos(context.Background(), root)
	if err != nil || len(open) != 1 || open[0].Done != "" {
		t.Fatalf("offen mit übrig gebliebenem done: %+v %v", open, err)
	}
}

func TestTodosVeralteteRevision(t *testing.T) {
	root := todoProject(t)
	store := newFakeTodoStore()
	useTodoStore(t, store)
	stamp := "2026-10-01T12:00:00.000Z"
	store.put("todos/001-a.md", "A\n", stamp, stamp)
	// Zwischen read und write ändert ein anderer Rechner das Dokument.
	stale := &staleTodoStore{fakeTodoStore: store}
	useTodoStore(t, stale)
	text := "B"
	_, _, err := UpdateTodo(context.Background(), root, 1, &text, nil)
	wantCode(t, err, DocCodeStaleRevision)
	if store.docs["todos/001-a.md"].content != "von anderswo\n" {
		t.Fatalf("fremde Änderung überschrieben: %q", store.docs["todos/001-a.md"].content)
	}
}

// staleTodoStore ändert ein Dokument nach jedem read, wie ein anderer Rechner.
type staleTodoStore struct{ *fakeTodoStore }

func (s *staleTodoStore) Read(ctx context.Context, name string) (kephalaion.ReadResult, error) {
	result, err := s.fakeTodoStore.Read(ctx, name)
	if doc, ok := s.docs[name]; ok {
		s.revision++
		doc.content, doc.revision = "von anderswo\n", s.revision
	}
	return result, err
}

func TestTodosMehrdeutigeKennung(t *testing.T) {
	root := todoProject(t)
	store := newFakeTodoStore()
	useTodoStore(t, store)
	stamp := "2026-10-01T12:00:00.000Z"
	store.put("todos/005-a.md", "A\n", stamp, stamp)
	store.put("todos/005-b.md", "B\n", stamp, stamp)
	_, _, err := DeleteTodo(context.Background(), root, 5)
	wantCode(t, err, DocCodeInvalidInput)
	if !strings.Contains(err.Error(), "mehrdeutig") || len(store.writes()) != 0 {
		t.Fatalf("mehrdeutig: %v %v", err, store.writes())
	}
}

func TestTodoShortName(t *testing.T) {
	cases := map[string]string{
		"Einen Punkt erledigen":                          "einen-punkt-erledigen",
		"`k-playbook todo` umbauen: eins zwei drei vier": "k-playbook-todo-umbauen-eins-zwei-drei",
		"???": "todo",
		"Ein sehr langes Wortungetüm Donaudampfschifffahrtsgesellschaftskapitänsmütze": "ein-sehr-langes-wortungetuem-donaudampfs",
	}
	for text, want := range cases {
		got := todoShortName(text)
		if got != want || len(got) > todoMaxShortName || !docNamePattern.MatchString(docFileName(999, got)) {
			t.Errorf("todoShortName(%q) = %q, erwartet %q", text, got, want)
		}
	}
}

// TestAddTodoUeberDenEchtenClient geht über kephalaion.Client gegen den
// gefälschten Node: create mit request_id und dem Inhalt des Formats.
func TestAddTodoUeberDenEchtenClient(t *testing.T) {
	root := todoProject(t)
	node := &kephNode{answer: func(_ int, call kephCall, w http.ResponseWriter, id int) {
		kephOK(w, id, call.name)
	}}
	client := newKephClient(t, node)
	original := OpenTodoStore
	OpenTodoStore = func(string) (TodoStore, string, error) { return client, "kamran", nil }
	t.Cleanup(func() { OpenTodoStore = original })

	todo, _, err := AddTodo(context.Background(), root, "Über den Node", "Test")
	if err != nil || todo.ID != 1 {
		t.Fatalf("AddTodo: %+v %v", todo, err)
	}
	calls := node.recorded()
	if len(calls) != 1 || calls[0].tool != "create" || calls[0].name != "todos/001-ueber-den-node.md" ||
		calls[0].requestID == "" || calls[0].content != "---\norigin: Test\n---\n\nÜber den Node\n" {
		t.Fatalf("Aufrufe: %+v", calls)
	}
}
