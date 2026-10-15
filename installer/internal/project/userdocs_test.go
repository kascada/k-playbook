package project

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
)

// fakeDoc ist ein Dokument im gefälschten Kephalaion.
type fakeDoc struct {
	by    string
	title string
	// hidden fehlt in der Replica (list), lebt aber am Hub (create) — der
	// Rückstand der Replica.
	hidden bool
}

// fakeDocStore ist ein Kephalaion im Speicher: create und rename prüfen den
// Namen wie der Hub, list liest nur, was nicht hidden ist.
type fakeDocStore struct {
	docs map[string]*fakeDoc
	user string
	// fail lässt jeden Aufruf mit diesem Fehler scheitern.
	fail error
	// createFail lässt nur create scheitern.
	createFail error
	creates    []string
	renames    [][2]string
	lists      []kephalaion.ListRequest
}

func newFakeDocStore(user string, names ...string) *fakeDocStore {
	store := &fakeDocStore{docs: map[string]*fakeDoc{}, user: user}
	for _, name := range names {
		store.docs[name] = &fakeDoc{by: user}
	}
	return store
}

func (s *fakeDocStore) List(_ context.Context, req kephalaion.ListRequest) ([]kephalaion.Entry, error) {
	s.lists = append(s.lists, req)
	if s.fail != nil {
		return nil, s.fail
	}
	var out []kephalaion.Entry
	for name, doc := range s.docs {
		if doc.hidden {
			continue
		}
		if req.Path != "" && !strings.HasPrefix(name, req.Path+"/") {
			continue
		}
		if !req.Recursive && strings.Contains(strings.TrimPrefix(name, req.Path+"/"), "/") {
			continue
		}
		entry := kephalaion.Entry{Kind: "document", Name: name,
			Created: &kephalaion.Stamp{At: "2026-10-14T10:00:00.000Z", By: doc.by},
			Updated: &kephalaion.Stamp{At: "2026-10-14T11:00:00.000Z", By: doc.by}}
		if req.Frontmatter && doc.title != "" {
			entry.Frontmatter = map[string]any{"title": doc.title}
		}
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (s *fakeDocStore) Create(_ context.Context, name, _ string) (kephalaion.Document, error) {
	s.creates = append(s.creates, name)
	if s.fail != nil {
		return kephalaion.Document{}, s.fail
	}
	if s.createFail != nil {
		return kephalaion.Document{}, s.createFail
	}
	if _, ok := s.docs[name]; ok {
		return kephalaion.Document{}, &kephalaion.ToolError{Tool: "create", Code: kephalaion.CodeNameTaken, Message: "belegt"}
	}
	s.docs[name] = &fakeDoc{by: s.user}
	return kephalaion.Document{Name: name}, nil
}

func (s *fakeDocStore) Rename(_ context.Context, name, newName string) (kephalaion.Document, error) {
	s.renames = append(s.renames, [2]string{name, newName})
	if s.fail != nil {
		return kephalaion.Document{}, s.fail
	}
	doc, ok := s.docs[name]
	if !ok {
		return kephalaion.Document{}, &kephalaion.ToolError{Tool: "rename", Code: kephalaion.CodeNotFound, Message: "fehlt"}
	}
	if _, ok := s.docs[newName]; ok {
		return kephalaion.Document{}, &kephalaion.ToolError{Tool: "rename", Code: kephalaion.CodeNameTaken, Message: "Ziel belegt"}
	}
	delete(s.docs, name)
	s.docs[newName] = doc
	return kephalaion.Document{Name: newName}, nil
}

func newTestUserDocs(t *testing.T, store *fakeDocStore, kinds []DocKind) *UserDocs {
	t.Helper()
	docs, err := NewUserDocs(kinds, store, "kamran")
	if err != nil {
		t.Fatalf("NewUserDocs: %v", err)
	}
	return docs
}

func wantCode(t *testing.T, err error, code string) {
	t.Helper()
	if got := DocRefErrorCode(err); got != code {
		t.Fatalf("erwartet Code %s, bekommen %q (%v)", code, got, err)
	}
}

func TestDocKindsTabelleIstGueltig(t *testing.T) {
	if err := ValidateDocKinds(DocKinds); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(DocKindNames(DocKinds), ","); got != "tasks,workbench,findings" {
		t.Fatalf("Sorten: %s", got)
	}
}

func TestDocKindsQueueOhneUserbereich(t *testing.T) {
	bad := append(append([]DocKind{}, DocKinds...),
		DocKind{Kind: "notes", Dir: "queue/notes", UserArea: true, Counting: DocCountDir, Start: 1})
	err := ValidateDocKinds(bad)
	if err == nil || !strings.Contains(err.Error(), "queue/") {
		t.Fatalf("Ablage unter queue/ mit Userbereich angenommen: %v", err)
	}
	// Jeder Eintrag der Tabelle unter queue/ ist ohne Userbereich.
	for _, kind := range DocKinds {
		if strings.HasPrefix(kind.Dir, "queue/") && kind.UserArea {
			t.Errorf("Sorte %s: queue/ mit Userbereich", kind.Kind)
		}
	}
}

func TestDocKindsWeitereFehler(t *testing.T) {
	base := DocKind{Kind: "x", Dir: "user/~/x", UserArea: true, Counting: DocCountDir, Done: true, Start: 1}
	cases := map[string]func(k *DocKind){
		"Userbereich ohne user/~/": func(k *DocKind) { k.Dir = "manual/x" },
		"user/~/ ohne Userbereich": func(k *DocKind) { k.UserArea = false },
		"alle User ohne Userbereich": func(k *DocKind) {
			k.Dir, k.UserArea, k.Counting = "manual/x", false, DocCountAllUsers
		},
		"Startnummer 0":   func(k *DocKind) { k.Start = 0 },
		"Ablage auf done": func(k *DocKind) { k.Dir = "user/~/done" },
		"leeres Segment":  func(k *DocKind) { k.Dir = "user/~//x" },
	}
	for name, change := range cases {
		kind := base
		change(&kind)
		if err := ValidateDocKinds([]DocKind{kind}); err == nil {
			t.Errorf("%s: angenommen", name)
		}
	}
	if err := ValidateDocKinds([]DocKind{base, base}); err == nil {
		t.Error("doppelte Sorte angenommen")
	}
}

func TestNormalizeDocName(t *testing.T) {
	cases := map[string]string{
		"Userbereich Dokumente":         "userbereich-dokumente",
		"  Größe für Übergänge!  ":      "groesse-fuer-uebergaenge",
		"schon-passend.md":              "schon-passend",
		"a__b..c":                       "a-b-c",
		strings.Repeat("abcdefghij", 8): strings.Repeat("abcdefghij", 5),
	}
	for in, want := range cases {
		got, err := NormalizeDocName(in)
		if err != nil || got != want {
			t.Errorf("%q: %q %v, erwartet %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "   ", "!!!", ".md", "–"} {
		_, err := NormalizeDocName(in)
		wantCode(t, err, DocCodeInvalidInput)
	}
}

func TestDocCreateStartnummer(t *testing.T) {
	store := newFakeDocStore("kamran")
	docs := newTestUserDocs(t, store, DocKinds)

	for kind, want := range map[string]string{
		"tasks":     "keph:/user/kamran/tasks/100-erste.md",
		"workbench": "keph:/user/kamran/workbench/001-erste.md",
		"findings":  "keph:/queue/findings/001-erste.md",
	} {
		created, err := docs.Create(context.Background(), kind, "Erste")
		if err != nil || created.Ref != want {
			t.Errorf("%s: %+v %v, erwartet %s", kind, created, err, want)
		}
	}

	// Höchstwert unter der Startnummer: die Startnummer.
	store = newFakeDocStore("kamran", "user/kamran/tasks/042-alt.md")
	created, err := newTestUserDocs(t, store, DocKinds).Create(context.Background(), "tasks", "neu")
	if err != nil || created.Number != 100 {
		t.Fatalf("unter Start: %+v %v", created, err)
	}
	// Höchstwert ab der Startnummer: Höchstwert plus eins.
	store = newFakeDocStore("kamran", "user/kamran/tasks/100-alt.md")
	created, err = newTestUserDocs(t, store, DocKinds).Create(context.Background(), "tasks", "neu")
	if err != nil || created.Number != 101 || created.Name != "101-neu.md" || created.User != "kamran" {
		t.Fatalf("ab Start: %+v %v", created, err)
	}
	// Bei Sorten ohne Userbereich kein User in der Antwort.
	created, err = newTestUserDocs(t, newFakeDocStore("kamran"), DocKinds).Create(context.Background(), "findings", "b")
	if err != nil || created.User != "" {
		t.Fatalf("findings: %+v %v", created, err)
	}
}

func TestDocCreateZaehltUeberUserUndDone(t *testing.T) {
	store := newFakeDocStore("kamran",
		"user/anna/tasks/done/120-fertig.md",
		"user/kamran/tasks/105-offen.md",
		"user/kamran/workbench/300-andere-sorte.md",
		"user/anna/tasks/sub/500-zu-tief.md",
		"user/kamran/tasks/notiz.md",
	)
	docs := newTestUserDocs(t, store, DocKinds)

	created, err := docs.Create(context.Background(), "tasks", "Nächste Aufgabe")
	if err != nil {
		t.Fatal(err)
	}
	if created.Ref != "keph:/user/kamran/tasks/121-naechste-aufgabe.md" {
		t.Fatalf("Verweis %s", created.Ref)
	}
	// Die Workbench zählt ebenfalls projektweit: 300 aus kamran, dazu anna.
	store.docs["user/anna/workbench/done/310-x.md"] = &fakeDoc{by: "anna"}
	created, err = docs.Create(context.Background(), "workbench", "stand")
	if err != nil || created.Number != 311 {
		t.Fatalf("workbench: %+v %v", created, err)
	}
	if store.lists[0].Path != "user" || !store.lists[0].Recursive {
		t.Fatalf("Zählbereich nicht projektweit gelesen: %+v", store.lists[0])
	}
}

func TestDocCreateNameTakenWiederholt(t *testing.T) {
	store := newFakeDocStore("kamran")
	// Am Hub belegt, in der Replica noch nicht zu sehen.
	store.docs["user/kamran/tasks/100-a.md"] = &fakeDoc{by: "kamran", hidden: true}
	store.docs["user/kamran/tasks/101-a.md"] = &fakeDoc{by: "kamran", hidden: true}
	docs := newTestUserDocs(t, store, DocKinds)

	created, err := docs.Create(context.Background(), "tasks", "a")
	if err != nil || created.Number != 102 {
		t.Fatalf("%+v %v", created, err)
	}
	if len(store.creates) != 3 {
		t.Fatalf("Versuche: %v", store.creates)
	}

	// Gleiche Nummer, anderer Name: erlaubt, kein Konflikt.
	store = newFakeDocStore("kamran")
	store.docs["user/kamran/tasks/100-a.md"] = &fakeDoc{hidden: true}
	created, err = newTestUserDocs(t, store, DocKinds).Create(context.Background(), "tasks", "b")
	if err != nil || created.Number != 100 {
		t.Fatalf("gleiche Nummer: %+v %v", created, err)
	}

	// Immer belegt: nach einer festen Zahl von Versuchen ein Fehler.
	store = newFakeDocStore("kamran")
	store.createFail = &kephalaion.ToolError{Tool: "create", Code: kephalaion.CodeNameTaken, Message: "belegt"}
	_, err = newTestUserDocs(t, store, DocKinds).Create(context.Background(), "tasks", "a")
	wantCode(t, err, DocCodeNameTaken)
	if len(store.creates) != docCreateAttempts {
		t.Fatalf("Versuche %d, erwartet %d", len(store.creates), docCreateAttempts)
	}
}

func TestDocCreateUngueltig(t *testing.T) {
	docs := newTestUserDocs(t, newFakeDocStore("kamran"), DocKinds)
	_, err := docs.Create(context.Background(), "tasks", "???")
	wantCode(t, err, DocCodeInvalidInput)
	_, err = docs.Create(context.Background(), "notes", "a")
	wantCode(t, err, DocCodeInvalidInput)
	if err == nil || !strings.Contains(err.Error(), "tasks, workbench, findings") {
		t.Fatalf("Meldung nennt die Sorten nicht: %v", err)
	}
}

func TestDocListEigeneAlleUndDone(t *testing.T) {
	store := newFakeDocStore("kamran",
		"user/kamran/tasks/100-a.md",
		"user/kamran/tasks/done/101-b.md",
		"user/anna/tasks/102-c.md",
	)
	store.docs["user/kamran/tasks/100-a.md"].title = "Titel A"
	store.docs["user/anna/tasks/102-c.md"].by = "anna"
	docs := newTestUserDocs(t, store, DocKinds)

	list, err := docs.List(context.Background(), DocListOptions{Kind: "tasks"})
	if err != nil || list.User != "kamran" || len(list.Items) != 1 {
		t.Fatalf("eigene: %+v %v", list, err)
	}
	item := list.Items[0]
	if item.Ref != "keph:/user/kamran/tasks/100-a.md" || item.Title != "Titel A" || item.User != "kamran" ||
		item.Created == "" || item.Updated == "" || item.Done {
		t.Fatalf("Eintrag: %+v", item)
	}

	list, _ = docs.List(context.Background(), DocListOptions{Kind: "tasks", Done: true})
	if len(list.Items) != 2 || !list.Items[1].Done || list.Items[1].Title != "101-b.md" {
		t.Fatalf("mit done: %+v", list.Items)
	}
	list, _ = docs.List(context.Background(), DocListOptions{Kind: "tasks", User: "*"})
	if len(list.Items) != 2 || list.Items[0].User != "anna" || list.User != "*" {
		t.Fatalf("alle: %+v", list)
	}
	list, _ = docs.List(context.Background(), DocListOptions{Kind: "tasks", User: "anna"})
	if len(list.Items) != 1 || list.Items[0].User != "anna" {
		t.Fatalf("anna: %+v", list)
	}
	_, err = docs.List(context.Background(), DocListOptions{Kind: "tasks", User: "../x"})
	wantCode(t, err, DocCodeInvalidInput)
}

func TestDocListOhneUserbereich(t *testing.T) {
	store := newFakeDocStore("kamran", "queue/findings/001-a.md", "queue/findings/002-b.md")
	store.docs["queue/findings/002-b.md"].by = "anna"
	docs := newTestUserDocs(t, store, DocKinds)

	list, err := docs.List(context.Background(), DocListOptions{Kind: "findings", User: "kamran"})
	if err != nil || len(list.Items) != 2 || list.User != "" || list.Hint == "" {
		t.Fatalf("%+v %v", list, err)
	}
	if list.Items[0].User != "kamran" || list.Items[1].User != "anna" {
		t.Fatalf("User aus den Autor-Metadaten: %+v", list.Items)
	}
}

func TestDocDoneUndReopen(t *testing.T) {
	store := newFakeDocStore("kamran", "user/kamran/tasks/100-a.md")
	docs := newTestUserDocs(t, store, DocKinds)

	moved, err := docs.Done(context.Background(), "keph:/user/~/tasks/100-a.md")
	if err != nil || moved.Ref != "keph:/user/kamran/tasks/done/100-a.md" || moved.From != "keph:/user/kamran/tasks/100-a.md" {
		t.Fatalf("done: %+v %v", moved, err)
	}
	moved, err = docs.Reopen(context.Background(), moved.Ref)
	if err != nil || moved.Ref != "keph:/user/kamran/tasks/100-a.md" || moved.Kind != "tasks" {
		t.Fatalf("reopen: %+v %v", moved, err)
	}
}

func TestDocDoneAbgelehnt(t *testing.T) {
	store := newFakeDocStore("kamran",
		"user/kamran/tasks/100-a.md",
		"user/kamran/tasks/done/100-a.md",
		"user/kamran/tasks/done/101-b.md",
		"user/anna/tasks/102-c.md",
		"queue/findings/001-x.md",
	)
	docs := newTestUserDocs(t, store, DocKinds)
	ctx := context.Background()

	_, err := docs.Done(ctx, "keph:/user/anna/tasks/102-c.md")
	wantCode(t, err, DocCodeForeignUser)
	_, err = docs.Done(ctx, "keph:/queue/findings/001-x.md")
	wantCode(t, err, DocCodeDoneUnsupported)
	_, err = docs.Done(ctx, "keph:/user/~/tasks/done/101-b.md")
	wantCode(t, err, DocCodeWrongDir)
	_, err = docs.Reopen(ctx, "keph:/user/~/tasks/100-a.md")
	wantCode(t, err, DocCodeWrongDir)
	_, err = docs.Done(ctx, "keph:/manual/x.md")
	wantCode(t, err, DocCodeWrongDir)
	_, err = docs.Done(ctx, "keph:/user/~/tasks/sub/100-a.md")
	wantCode(t, err, DocCodeWrongDir)
	_, err = docs.Done(ctx, "k-playbook:/instructions/x.md")
	wantCode(t, err, DocCodeInvalidInput)
	_, err = docs.Done(ctx, "keph:/user/~/tasks/")
	wantCode(t, err, DocCodeInvalidInput)
	_, err = docs.Done(ctx, "keph:/user/../tasks/1.md")
	wantCode(t, err, DocCodeInvalidInput)
	if len(store.renames) != 0 {
		t.Fatalf("abgelehnt und doch verschoben: %v", store.renames)
	}

	// Belegtes Ziel: name_taken, nichts überschrieben.
	_, err = docs.Done(ctx, "keph:/user/kamran/tasks/100-a.md")
	wantCode(t, err, DocCodeNameTaken)
	if _, ok := store.docs["user/kamran/tasks/100-a.md"]; !ok {
		t.Fatal("Quelle verschwunden")
	}
	// Dokument fehlt: not_found von Kephalaion.
	_, err = docs.Done(ctx, "keph:/user/kamran/tasks/999-fehlt.md")
	wantCode(t, err, DocCodeNotFound)
}

func TestDocKephalaionNichtErreichbar(t *testing.T) {
	store := newFakeDocStore("kamran", "user/kamran/tasks/100-a.md")
	store.fail = &kephalaion.ToolError{Tool: "list", Code: kephalaion.CodeNodeUnreachable, Message: "keine Verbindung"}
	docs := newTestUserDocs(t, store, DocKinds)
	ctx := context.Background()

	_, err := docs.Create(ctx, "tasks", "a")
	wantCode(t, err, DocCodeKephalaionUnavailable)
	_, err = docs.List(ctx, DocListOptions{Kind: "tasks"})
	wantCode(t, err, DocCodeKephalaionUnavailable)
	_, err = docs.Done(ctx, "keph:/user/~/tasks/100-a.md")
	wantCode(t, err, DocCodeKephalaionUnavailable)

	// Ein anderer Code von Kephalaion bleibt erkennbar.
	store.fail = &kephalaion.ToolError{Tool: "rename", Code: "forbidden", Message: "kein Recht"}
	_, err = docs.Done(ctx, "keph:/user/~/tasks/100-a.md")
	wantCode(t, err, DocCodeKephalaionError)
	if !strings.Contains(err.Error(), "forbidden") {
		t.Fatalf("Code von Kephalaion fehlt in der Meldung: %v", err)
	}
}

func TestOpenUserDocsOhneAnbindung(t *testing.T) {
	original := kephalaionCheck
	kephalaionCheck = func(string) kephalaion.Status {
		return kephalaion.Status{State: kephalaion.StateNotConfigured, Message: "Keine Collection eingetragen."}
	}
	t.Cleanup(func() { kephalaionCheck = original })

	_, err := OpenUserDocs(t.TempDir())
	wantCode(t, err, DocCodeKephalaionUnavailable)
}

// TestDocZusaetzlicheSorteNurUeberTabelle zeigt, dass eine neue Sorte ein
// Eintrag in der Tabelle ist: anlegen, auflisten, erledigen, wiederöffnen,
// ohne eine Zeile Code.
func TestDocZusaetzlicheSorteNurUeberTabelle(t *testing.T) {
	kinds := append(append([]DocKind{}, DocKinds...),
		DocKind{Kind: "notes", Dir: "user/~/sammlung/notes", UserArea: true, Counting: DocCountDir, Done: true, Start: 5},
		DocKind{Kind: "ideas", Dir: "queue/ideas", UserArea: false, Counting: DocCountDir, Done: true, Start: 1},
	)
	store := newFakeDocStore("kamran", "user/anna/sammlung/notes/050-fremd.md")
	docs := newTestUserDocs(t, store, kinds)
	ctx := context.Background()

	// Zählbereich nur das eigene Verzeichnis: annas 050 zählt nicht.
	created, err := docs.Create(ctx, "notes", "Notiz")
	if err != nil || created.Ref != "keph:/user/kamran/sammlung/notes/005-notiz.md" {
		t.Fatalf("anlegen: %+v %v", created, err)
	}
	list, err := docs.List(ctx, DocListOptions{Kind: "notes"})
	if err != nil || len(list.Items) != 1 || list.Items[0].Ref != created.Ref {
		t.Fatalf("auflisten: %+v %v", list, err)
	}
	moved, err := docs.Done(ctx, created.Ref)
	if err != nil || moved.Ref != "keph:/user/kamran/sammlung/notes/done/005-notiz.md" {
		t.Fatalf("erledigen: %+v %v", moved, err)
	}
	if _, err := docs.Reopen(ctx, moved.Ref); err != nil {
		t.Fatalf("wiederöffnen: %v", err)
	}

	// Ohne Userbereich, mit done/.
	created, err = docs.Create(ctx, "ideas", "Idee")
	if err != nil || created.Ref != "keph:/queue/ideas/001-idee.md" {
		t.Fatalf("ideas: %+v %v", created, err)
	}
	if moved, err := docs.Done(ctx, created.Ref); err != nil || moved.Ref != "keph:/queue/ideas/done/001-idee.md" {
		t.Fatalf("ideas erledigen: %+v %v", moved, err)
	}
}
