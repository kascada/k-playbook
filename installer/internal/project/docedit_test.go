package project

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
)

const editTask = "user/kamran/tasks/100-a.md"

// noEditWait schaltet das Warten auf die Replica für einen Test ab.
func noEditWait(t *testing.T) {
	t.Helper()
	original := docEditWait
	docEditWait = func(int) time.Duration { return 0 }
	t.Cleanup(func() { docEditWait = original })
}

func newEditDocs(t *testing.T, content string) (*UserDocs, *fakeDocStore, *fakeDoc) {
	t.Helper()
	noEditWait(t)
	store := newFakeDocStore("kamran")
	doc := store.put(editTask, content)
	return newTestUserDocs(t, store, DocKinds), store, doc
}

func TestDocAppendRegel(t *testing.T) {
	cases := []struct{ name, content, text, want string }{
		{"leer", "", "Eintrag", "Eintrag\n"},
		{"ohne Schlussumbruch", "# Titel", "Eintrag\n\n\n", "# Titel\n\nEintrag\n"},
		{"normal", "# Titel\n", "Eintrag\nzweite Zeile", "# Titel\n\nEintrag\nzweite Zeile\n"},
	}
	for _, c := range cases {
		docs, store, doc := newEditDocs(t, c.content)
		before := doc.revision
		edited, err := docs.Append(context.Background(), "keph:/user/~/tasks/100-a.md", c.text, DocEditOptions{})
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if doc.content != c.want {
			t.Errorf("%s: Inhalt %q, erwartet %q", c.name, doc.content, c.want)
		}
		if !edited.Changed || edited.Revision != doc.revision || edited.Revision == before ||
			edited.Ref != "keph:/"+editTask || edited.Size != int64(len(c.want)) {
			t.Errorf("%s: Antwort %+v", c.name, edited)
		}
		if len(store.writes) != 1 {
			t.Errorf("%s: %d write", c.name, len(store.writes))
		}
	}
	docs, store, _ := newEditDocs(t, "x\n")
	_, err := docs.Append(context.Background(), "keph:/"+editTask, " \n", DocEditOptions{})
	wantCode(t, err, DocCodeInvalidInput)
	if len(store.reads) != 0 {
		t.Fatal("leerer Text und doch gelesen")
	}
}

func TestDocEditSinceRevision(t *testing.T) {
	docs, store, doc := newEditDocs(t, "# A\n")
	ctx := context.Background()

	// Gleich: geschrieben.
	edited, err := docs.Append(ctx, "keph:/"+editTask, "eins", DocEditOptions{SinceRevision: doc.revision})
	if err != nil || !edited.Changed {
		t.Fatalf("gleich: %+v %v", edited, err)
	}
	// Direkt danach mit der Revision aus dem ersten: kein foreign_change.
	edited, err = docs.Append(ctx, "keph:/"+editTask, "zwei", DocEditOptions{SinceRevision: edited.Revision})
	if err != nil || !edited.Changed {
		t.Fatalf("hintereinander: %+v %v", edited, err)
	}

	// Ein anderer schreibt: foreign_change mit Angaben, nichts geschrieben.
	seen := edited.Revision
	doc.content += "\nfremd\n"
	store.revision++
	doc.revision, doc.by = store.revision, "anna"
	writes := len(store.writes)
	_, err = docs.Append(ctx, "keph:/"+editTask, "drei", DocEditOptions{SinceRevision: seen})
	wantCode(t, err, DocCodeForeignChange)
	change, ok := DocForeignChangeOf(err)
	if !ok || change.Revision != doc.revision || change.SinceRevision != seen || change.UpdatedBy != "anna" ||
		change.UpdatedAt == "" || !strings.HasSuffix(change.Tail, "fremd\n") {
		t.Fatalf("Angaben: %+v", change)
	}
	if len(store.writes) != writes {
		t.Fatal("foreign_change und doch geschrieben")
	}
}

func TestDocEditTail20Zeilen(t *testing.T) {
	var b strings.Builder
	for i := 1; i <= 30; i++ {
		fmt.Fprintf(&b, "Zeile %d\n", i)
	}
	docs, _, doc := newEditDocs(t, b.String())
	_, err := docs.Append(context.Background(), "keph:/"+editTask, "x", DocEditOptions{SinceRevision: doc.revision + 5})
	change, ok := DocForeignChangeOf(err)
	if !ok || strings.Count(change.Tail, "\n") != 20 || !strings.HasPrefix(change.Tail, "Zeile 11\n") {
		t.Fatalf("Tail: %+v", change)
	}
}

func TestDocEditReplicaLaeuftNach(t *testing.T) {
	docs, store, doc := newEditDocs(t, "# A\n")
	ctx := context.Background()
	edited, err := docs.Append(ctx, "keph:/"+editTask, "eins", DocEditOptions{})
	if err != nil {
		t.Fatal(err)
	}
	// Die Replica zeigt noch zweimal den Stand vor dem eigenen Schreiben.
	doc.lagReads, doc.lagContent, doc.lagRevision = 2, "# A\n", edited.Revision-1
	reads := len(store.reads)
	edited, err = docs.Append(ctx, "keph:/"+editTask, "zwei", DocEditOptions{SinceRevision: edited.Revision})
	if err != nil || !edited.Changed {
		t.Fatalf("nachlaufende Replica: %+v %v", edited, err)
	}
	if got := len(store.reads) - reads; got != 3 {
		t.Fatalf("%d read, erwartet 3", got)
	}
	if !strings.HasSuffix(doc.content, "eins\n\nzwei\n") {
		t.Fatalf("Inhalt: %q", doc.content)
	}

	// Bleibt sie über alle Versuche zurück: foreign_change.
	doc.lagReads, doc.lagContent, doc.lagRevision = 10, "# A\n", 1
	_, err = docs.Append(ctx, "keph:/"+editTask, "drei", DocEditOptions{SinceRevision: edited.Revision})
	wantCode(t, err, DocCodeForeignChange)
}

func TestDocEditStaleRevision(t *testing.T) {
	ctx := context.Background()

	// Ohne since_revision: neu lesen und wiederholen.
	docs, store, doc := newEditDocs(t, "# A\n")
	store.staleWrites = 2
	edited, err := docs.Append(ctx, "keph:/"+editTask, "eins", DocEditOptions{})
	if err != nil || !edited.Changed || doc.content != "# A\n\neins\n" || len(store.writes) != 3 {
		t.Fatalf("ohne since: %+v %v, %d write", edited, err, len(store.writes))
	}
	// Mehr als drei Wiederholungen: stale_revision.
	store.staleWrites = 10
	_, err = docs.Append(ctx, "keph:/"+editTask, "zwei", DocEditOptions{})
	wantCode(t, err, DocCodeStaleRevision)

	// Mit since_revision: foreign_change, keine Wiederholung.
	docs, store, doc = newEditDocs(t, "# A\n")
	store.staleWrites = 1
	_, err = docs.Append(ctx, "keph:/"+editTask, "eins", DocEditOptions{SinceRevision: doc.revision})
	wantCode(t, err, DocCodeForeignChange)
	if len(store.writes) != 1 || doc.content != "# A\n" {
		t.Fatalf("mit since: %d write, Inhalt %q", len(store.writes), doc.content)
	}
}

func TestDocEditUnveraendertSchreibtNicht(t *testing.T) {
	docs, store, doc := newEditDocs(t, "---\ntitle: A\n---\n\nText\n")
	before := doc.revision
	edited, err := docs.Replace(context.Background(), "keph:/"+editTask, "Text", "Text", DocEditOptions{})
	if err != nil || edited.Changed || edited.Revision != before || len(store.writes) != 0 {
		t.Fatalf("replace gleich: %+v %v", edited, err)
	}
	value := "A"
	edited, err = docs.SetField(context.Background(), "keph:/"+editTask, "title", &value, DocEditOptions{})
	if err != nil || edited.Changed || len(store.writes) != 0 {
		t.Fatalf("set gleich: %+v %v", edited, err)
	}
	edited, err = docs.SetField(context.Background(), "keph:/"+editTask, "status", nil, DocEditOptions{})
	if err != nil || edited.Changed || len(store.writes) != 0 {
		t.Fatalf("unset fehlend: %+v %v", edited, err)
	}
}

func TestDocReplace(t *testing.T) {
	ctx := context.Background()
	docs, store, doc := newEditDocs(t, "a b a\nc\n")

	_, err := docs.Replace(ctx, "keph:/"+editTask, "x", "y", DocEditOptions{})
	wantCode(t, err, DocCodeTextNotFound)
	_, err = docs.Replace(ctx, "keph:/"+editTask, "a", "y", DocEditOptions{})
	wantCode(t, err, DocCodeTextNotUnique)
	if !strings.Contains(err.Error(), "2-mal") {
		t.Fatalf("Anzahl fehlt: %v", err)
	}
	_, err = docs.Replace(ctx, "keph:/"+editTask, "", "y", DocEditOptions{})
	wantCode(t, err, DocCodeInvalidInput)
	if len(store.writes) != 0 {
		t.Fatal("abgelehnt und doch geschrieben")
	}

	edited, err := docs.Replace(ctx, "keph:/"+editTask, "b a\n", "", DocEditOptions{})
	if err != nil || !edited.Changed || doc.content != "a c\n" {
		t.Fatalf("ein Treffer: %+v %v %q", edited, err, doc.content)
	}
}

func TestDocReplaceUeberlappend(t *testing.T) {
	ctx := context.Background()
	docs, store, _ := newEditDocs(t, "# A\nx\nx\nx\n")

	// "x\nx\n" steht an zwei Stellen (überlappend); strings.Count sähe nur eine.
	_, err := docs.Replace(ctx, "keph:/"+editTask, "x\nx\n", "y\n", DocEditOptions{})
	wantCode(t, err, DocCodeTextNotUnique)
	if !strings.Contains(err.Error(), "2-mal") {
		t.Fatalf("Anzahl fehlt: %v", err)
	}
	if len(store.writes) != 0 {
		t.Fatal("abgelehnt und doch geschrieben")
	}

	cases := []struct {
		s, sub string
		want   int
	}{
		{"x\nx\nx\n", "x\nx\n", 2},
		{"aaaa", "aa", 3},
		{"a b a\nc\n", "a", 2},
		{"abc", "d", 0},
		{"abc", "abc", 1},
	}
	for _, c := range cases {
		if got := countOverlapping(c.s, c.sub); got != c.want {
			t.Errorf("countOverlapping(%q, %q) = %d, want %d", c.s, c.sub, got, c.want)
		}
	}
}

func TestDocSetFieldRegel(t *testing.T) {
	str := func(s string) *string { return &s }
	cases := []struct {
		name    string
		content string
		key     string
		value   *string
		want    string
	}{
		{"ohne Frontmatter", "# Titel\n", "status", str("offen"), "---\nstatus: offen\n---\n\n# Titel\n"},
		{"leer", "", "status", str("offen"), "---\nstatus: offen\n---\n"},
		{"neues Feld", "---\ntitle: A\nz: 1\n---\nRumpf  \n", "status", str("offen"),
			"---\ntitle: A\nz: 1\nstatus: offen\n---\nRumpf  \n"},
		{"bestehendes Feld", "---\ntitle: A\nstatus: alt\nz: 1\n---\n\nRumpf\n", "status", str("neu"),
			"---\ntitle: A\nstatus: neu\nz: 1\n---\n\nRumpf\n"},
		{"Sonderzeichen", "---\ntitle: A\n---\n", "title", str(`a: "b" #c`),
			"---\ntitle: \"a: \\\"b\\\" #c\"\n---\n"},
		{"Zahl als Text", "---\n---\n", "version", str("1.10"), "---\nversion: \"1.10\"\n---\n"},
		{"unset mit Folgezeilen", "---\ntitle: A\ntags:\n  - a\n\n- b\nz: 1\n---\nRumpf\n", "tags", nil,
			"---\ntitle: A\nz: 1\n---\nRumpf\n"},
		{"ersetzen mit Block", "---\ndesc: |\n  eins\n\n  zwei\nz: 1\n---\n", "desc", str("kurz"),
			"---\ndesc: kurz\nz: 1\n---\n"},
		{"unset", "---\ntitle: A\nstatus: x\n---\n", "status", nil, "---\ntitle: A\n---\n"},
		{"ähnlicher Name bleibt", "---\nstatus2: x\n---\n", "status", str("y"), "---\nstatus2: x\nstatus: y\n---\n"},
	}
	for _, c := range cases {
		if got := setDocField(c.content, c.key, c.value); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}

	docs, store, doc := newEditDocs(t, "# T\n")
	value := "erledigt"
	edited, err := docs.SetField(context.Background(), "keph:/"+editTask, "status", &value, DocEditOptions{})
	if err != nil || !edited.Changed || doc.content != "---\nstatus: erledigt\n---\n\n# T\n" {
		t.Fatalf("SetField: %+v %v %q", edited, err, doc.content)
	}
	for _, key := range []string{"Status", "1a", "", "a b", "a.b"} {
		_, err := docs.SetField(context.Background(), "keph:/"+editTask, key, &value, DocEditOptions{})
		wantCode(t, err, DocCodeInvalidInput)
	}
	if len(store.writes) != 1 {
		t.Fatalf("%d write", len(store.writes))
	}
}

func TestDocEditGeltungsbereich(t *testing.T) {
	noEditWait(t)
	store := newFakeDocStore("kamran")
	store.put("user/anna/tasks/101-b.md", "x\n")
	store.put("user/kamran/tasks/done/102-c.md", "x\n")
	store.put("user/kamran/workbench/001-w.md", "x\n")
	store.put("queue/findings/003-f.md", "x\n")
	store.put("todos/004-t.md", "x\n")
	store.put("docs/index.md", "x\n")
	store.put("docs/generated/code/a.md", "x\n")
	docs := newTestUserDocs(t, store, DocKinds)
	ctx := context.Background()

	_, err := docs.Append(ctx, "keph:/user/anna/tasks/101-b.md", "y", DocEditOptions{})
	wantCode(t, err, DocCodeForeignUser)
	for _, ref := range []string{
		"keph:/todos/004-t.md", "keph:/docs/index.md", "keph:/docs/generated/code/a.md", "keph:/docs/manual",
		"project:/README.md", "k-playbook:/rules/a.md",
		"keph:/user/~/tasks/", "keph:/user/~/tasks/sub/100-a.md",
	} {
		_, err := docs.Append(ctx, ref, "y", DocEditOptions{})
		wantCode(t, err, DocCodeInvalidInput)
	}
	if len(store.reads) != 0 || len(store.writes) != 0 {
		t.Fatalf("abgelehnt und doch gelesen/geschrieben: %v %v", store.reads, store.writes)
	}

	for _, ref := range []string{
		"keph:/user/~/tasks/done/102-c.md", "keph:/user/kamran/workbench/001-w.md", "keph:/queue/findings/003-f.md",
	} {
		if _, err := docs.Append(ctx, ref, "y", DocEditOptions{}); err != nil {
			t.Errorf("%s: %v", ref, err)
		}
	}
	_, err = docs.Append(ctx, "keph:/user/~/tasks/999-fehlt.md", "y", DocEditOptions{})
	wantCode(t, err, DocCodeNotFound)
	_, err = docs.Append(ctx, "keph:/user/~/tasks/done/102-c.md", "y", DocEditOptions{SinceRevision: -1})
	wantCode(t, err, DocCodeInvalidInput)
}

// Die Handdoku unter docs/manual/ (DocEditAreas) erreichen append und replace,
// beliebig tief und ohne Userbereich; set nicht, und außerhalb des Bereichs
// bleibt docs/ gesperrt. Entschieden wird ohne zu lesen.
func TestDocEditHanddoku(t *testing.T) {
	noEditWait(t)
	store := newFakeDocStore("kamran")
	page := store.put("docs/manual/local-dev-start.md", "---\ntitle: Start\n---\n\nAlt eins.\n")
	store.put("docs/manual/tief/unten.md", "x\n")
	docs := newTestUserDocs(t, store, DocKinds)
	ctx := context.Background()

	edited, err := docs.Replace(ctx, "keph:/docs/manual/local-dev-start.md", "Alt eins.", "Neu eins.",
		DocEditOptions{SinceRevision: page.revision})
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if !edited.Changed || edited.Ref != "keph:/docs/manual/local-dev-start.md" || edited.Revision != page.revision {
		t.Fatalf("Antwort: %+v (Dokument jetzt Revision %d)", edited, page.revision)
	}
	if page.content != "---\ntitle: Start\n---\n\nNeu eins.\n" {
		t.Fatalf("Inhalt: %q", page.content)
	}
	if _, err := docs.Append(ctx, "keph:/docs/manual/tief/unten.md", "y", DocEditOptions{}); err != nil {
		t.Fatalf("append tief: %v", err)
	}

	before := len(store.reads)
	value := "v"
	_, err = docs.SetField(ctx, "keph:/docs/manual/local-dev-start.md", "title", &value, DocEditOptions{})
	wantCode(t, err, DocCodeInvalidInput)
	if !strings.Contains(err.Error(), "Handdoku") || !strings.Contains(err.Error(), "append und replace") {
		t.Fatalf("Meldung nennt den Bereich nicht: %v", err)
	}
	_, err = docs.Append(ctx, "keph:/docs/manual/", "y", DocEditOptions{})
	wantCode(t, err, DocCodeInvalidInput)
	if len(store.reads) != before {
		t.Fatalf("abgelehnt und doch gelesen: %v", store.reads[before:])
	}

	_, err = docs.Replace(ctx, "keph:/docs/manual/fehlt.md", "a", "b", DocEditOptions{})
	wantCode(t, err, DocCodeNotFound)
	_, err = docs.Replace(ctx, "keph:/docs/manual/local-dev-start.md", "Neu eins.", "x",
		DocEditOptions{SinceRevision: page.revision + 5})
	wantCode(t, err, DocCodeForeignChange)
}

func TestDocEditBereicheValidierung(t *testing.T) {
	cases := map[string][]DocEditArea{
		"ohne Operation": {{Dir: "docs/manual", Label: "x"}},
		"Sorte darunter": {{Dir: "queue", Label: "x", Append: true}},
		"in einer Sorte": {{Dir: "queue/docs/sub", Label: "x", Append: true}},
		"gleich Sorte":   {{Dir: "todos", Label: "x", Append: true}},
		"doppelt":        {{Dir: "docs/manual", Label: "x", Append: true}, {Dir: "docs/manual/a", Label: "y", Append: true}},
		"User-Segment":   {{Dir: "docs/~", Label: "x", Append: true}},
		"leeres Segment": {{Dir: "docs//manual", Label: "x", Append: true}},
	}
	for name, areas := range cases {
		if err := validateDocEditAreas(areas, DocKinds); err == nil {
			t.Errorf("%s: kein Fehler", name)
		}
	}
	if err := validateDocEditAreas(DocEditAreas, DocKinds); err != nil {
		t.Fatalf("DocEditAreas: %v", err)
	}
}

func TestDocEditKephalaionFehler(t *testing.T) {
	docs, store, _ := newEditDocs(t, "x\n")
	ctx := context.Background()
	store.writeFail = &kephalaion.ToolError{Tool: "write", Code: kephalaion.CodeOutcomeUnknown, Message: "Ausgang offen"}
	_, err := docs.Append(ctx, "keph:/"+editTask, "y", DocEditOptions{})
	wantCode(t, err, DocCodeOutcomeUnknown)
	if !strings.Contains(err.Error(), "nicht noch einmal") && !strings.Contains(err.Error(), "Nicht noch einmal") {
		t.Fatalf("Hinweis fehlt: %v", err)
	}
	store.writeFail = &kephalaion.ToolError{Tool: "write", Code: kephalaion.CodeUnreachable, Message: "Hub weg"}
	_, err = docs.Append(ctx, "keph:/"+editTask, "y", DocEditOptions{})
	wantCode(t, err, DocCodeKephalaionUnavailable)
	store.writeFail = nil
	store.fail = &kephalaion.ToolError{Tool: "read", Code: kephalaion.CodeNodeUnreachable, Message: "Node weg"}
	_, err = docs.Append(ctx, "keph:/"+editTask, "y", DocEditOptions{})
	wantCode(t, err, DocCodeKephalaionUnavailable)
}
