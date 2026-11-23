package project

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func listRefs(l DocList) []string {
	var refs []string
	for _, item := range l.Items {
		refs = append(refs, item.Ref)
	}
	return refs
}

func pagedStore() (*fakeDocStore, *UserDocs) {
	store := newFakeDocStore("kamran",
		"user/kamran/tasks/007-sieben.md",
		"user/kamran/tasks/008-acht-Alpha.md",
		"user/kamran/tasks/done/007-doppelt.md",
		"user/kamran/tasks/done/009-neun.md",
		"user/kamran/tasks/010-zehn.md",
		"user/kamran/tasks/ohne-nummer.md",
	)
	return store, nil
}

func TestDocListNumberUndName(t *testing.T) {
	store, _ := pagedStore()
	docs := newTestUserDocs(t, store, DocKinds)
	ctx := context.Background()

	// führende Nullen: 7 und 007 sind gleich; mehrere Treffer, auch unter done/.
	list, err := docs.List(ctx, DocListOptions{Kind: "tasks", Done: true, Number: 7})
	if err != nil || len(list.Items) != 2 || list.Total != 2 {
		t.Fatalf("number 7: %+v %v", list, err)
	}
	// Ohne done nur das offene.
	list, _ = docs.List(ctx, DocListOptions{Kind: "tasks", Number: 7})
	if len(list.Items) != 1 || list.Items[0].Ref != "keph:/user/kamran/tasks/007-sieben.md" {
		t.Fatalf("number 7 offen: %+v", list)
	}
	// Ein Dokument ohne führende Ziffern trifft nie.
	list, _ = docs.List(ctx, DocListOptions{Kind: "tasks", Done: true, Number: 99})
	if len(list.Items) != 0 || list.Total != 0 {
		t.Fatalf("number 99: %+v", list)
	}
	// name: Teilstring, ohne Groß-/Kleinschreibung, im Dateinamen.
	list, _ = docs.List(ctx, DocListOptions{Kind: "tasks", Done: true, Name: "ALPHA"})
	if len(list.Items) != 1 || !strings.HasSuffix(list.Items[0].Ref, "008-acht-Alpha.md") {
		t.Fatalf("name: %+v", list)
	}
	list, _ = docs.List(ctx, DocListOptions{Kind: "tasks", Done: true, Name: "ohne-"})
	if len(list.Items) != 1 {
		t.Fatalf("name ohne Nummer: %+v", list)
	}
	// Der Pfad zählt nicht, nur der Dateiname.
	list, _ = docs.List(ctx, DocListOptions{Kind: "tasks", Done: true, Name: "done"})
	if len(list.Items) != 0 {
		t.Fatalf("name im Pfad: %+v", list)
	}
	_, err = docs.List(ctx, DocListOptions{Kind: "tasks", Number: -1})
	wantCode(t, err, DocCodeInvalidInput)
}

func TestDocListNumberSorteOhneNummern(t *testing.T) {
	kinds := append(append([]DocKind{}, DocKinds...),
		DocKind{Kind: "plain", Dir: "queue/plain", Counting: DocCountDir, Start: 1, Unnumbered: true})
	store := newFakeDocStore("kamran", "queue/plain/abc.md")
	docs := newTestUserDocs(t, store, kinds)
	_, err := docs.List(context.Background(), DocListOptions{Kind: "plain", Number: 1})
	wantCode(t, err, DocCodeInvalidInput)
	list, err := docs.List(context.Background(), DocListOptions{Kind: "plain", Name: "abc"})
	if err != nil || len(list.Items) != 1 {
		t.Fatalf("plain name: %+v %v", list, err)
	}
}

func numberedStore(n int) *fakeDocStore {
	var names []string
	for i := 1; i <= n; i++ {
		names = append(names, fmt.Sprintf("queue/findings/%03d-f.md", i))
	}
	return newFakeDocStore("kamran", names...)
}

func TestDocListSeitenAmRand(t *testing.T) {
	docs := newTestUserDocs(t, numberedStore(5), DocKinds)
	ctx := context.Background()

	first, err := docs.List(ctx, DocListOptions{Kind: "findings", Limit: 2})
	if err != nil || len(first.Items) != 2 || first.Total != 5 || !first.Truncated || first.Next == nil ||
		first.Next.After != "keph:/queue/findings/002-f.md" || first.Next.Hint == "" {
		t.Fatalf("erste: %+v %v", first, err)
	}
	mid, _ := docs.List(ctx, DocListOptions{Kind: "findings", Limit: 2, After: first.Next.After})
	if got := listRefs(mid); len(got) != 2 || got[0] != "keph:/queue/findings/003-f.md" || mid.Total != 5 || !mid.Truncated {
		t.Fatalf("mittlere: %+v", mid)
	}
	last, _ := docs.List(ctx, DocListOptions{Kind: "findings", Limit: 2, After: mid.Next.After})
	if got := listRefs(last); len(got) != 1 || got[0] != "keph:/queue/findings/005-f.md" || last.Truncated || last.Next != nil || last.Total != 5 {
		t.Fatalf("letzte: %+v", last)
	}
	empty, err := docs.List(ctx, DocListOptions{Kind: "findings", Limit: 2, After: "keph:/queue/findings/005-f.md"})
	if err != nil || len(empty.Items) != 0 || empty.Truncated || empty.Total != 5 {
		t.Fatalf("leere: %+v %v", empty, err)
	}
	// after eines Dokuments, das es nicht (mehr) gibt: Seite setzt dahinter an.
	gone, err := docs.List(ctx, DocListOptions{Kind: "findings", After: "keph:/queue/findings/003-weg.md"})
	if err != nil || len(gone.Items) != 2 || gone.Items[0].Ref != "keph:/queue/findings/004-f.md" {
		t.Fatalf("after ohne Dokument: %+v %v", gone, err)
	}
	// after ist kein Verweis der Sorte.
	for _, bad := range []string{"quatsch", "keph:/user/kamran/tasks/001-a.md", "project:/x"} {
		_, err = docs.List(ctx, DocListOptions{Kind: "findings", After: bad})
		wantCode(t, err, DocCodeInvalidInput)
	}
	// Ohne limit keine Grenze.
	all, _ := docs.List(ctx, DocListOptions{Kind: "findings"})
	if len(all.Items) != 5 || all.Truncated || all.Total != 5 {
		t.Fatalf("ohne limit: %+v", all)
	}
}

func TestDocListLimitGrenzenUndCompact(t *testing.T) {
	docs := newTestUserDocs(t, numberedStore(3), DocKinds)
	ctx := context.Background()
	for _, opts := range []DocListOptions{
		{Kind: "findings", Limit: -1},
		{Kind: "findings", Limit: 201, Compact: true},
		{Kind: "findings", Limit: 51},
	} {
		_, err := docs.List(ctx, opts)
		wantCode(t, err, DocCodeInvalidInput)
	}
	if _, err := docs.List(ctx, DocListOptions{Kind: "findings", Limit: 200, Compact: true}); err != nil {
		t.Fatalf("limit 200 compact: %v", err)
	}
	full, _ := docs.List(ctx, DocListOptions{Kind: "findings"})
	raw, _ := json.Marshal(full.Items[0])
	if !strings.Contains(string(raw), `"user"`) || !strings.Contains(string(raw), `"created"`) || !strings.Contains(string(raw), `"updated"`) {
		t.Fatalf("voller Eintrag: %s", raw)
	}
	compact, _ := docs.List(ctx, DocListOptions{Kind: "findings", Compact: true})
	raw, _ = json.Marshal(compact.Items[0])
	if string(raw) != `{"ref":"keph:/queue/findings/001-f.md","title":"001-f.md","done":false}` {
		t.Fatalf("kompakter Eintrag: %s", raw)
	}
}

func TestDocListZeichenobergrenze(t *testing.T) {
	var names []string
	for i := 1; i <= 40; i++ {
		names = append(names, fmt.Sprintf("queue/findings/%03d-f.md", i))
	}
	store := newFakeDocStore("kamran", names...)
	long := strings.Repeat("x", 1500)
	for _, name := range names {
		store.docs[name].title = long
	}
	docs := newTestUserDocs(t, store, DocKinds)
	list, err := docs.List(context.Background(), DocListOptions{Kind: "findings", Limit: 50})
	if err != nil || len(list.Items) >= 40 || len(list.Items) < 1 || !list.Truncated || list.Next == nil || list.Total != 40 {
		t.Fatalf("Zeichenobergrenze: %d Einträge %+v %v", len(list.Items), list.Next, err)
	}
	raw, _ := json.MarshalIndent(list.Items, "  ", "  ")
	if len(raw) > DocListMaxChars+len(long) {
		t.Fatalf("zu groß: %d", len(raw))
	}
	// Ein einzelner Eintrag über der Grenze kommt trotzdem.
	big := newFakeDocStore("kamran", "queue/findings/001-f.md", "queue/findings/002-f.md")
	huge := strings.Repeat("y", DocListMaxChars+10)
	big.docs["queue/findings/001-f.md"].title = huge
	docs = newTestUserDocs(t, big, DocKinds)
	list, _ = docs.List(context.Background(), DocListOptions{Kind: "findings", Limit: 10})
	if len(list.Items) != 1 || !list.Truncated || list.Next.After != "keph:/queue/findings/001-f.md" {
		t.Fatalf("mindestens ein Eintrag: %+v", list)
	}
}

func TestDocListTotalMitUndOhneAfter(t *testing.T) {
	docs := newTestUserDocs(t, numberedStore(6), DocKinds)
	ctx := context.Background()
	a, _ := docs.List(ctx, DocListOptions{Kind: "findings", Name: "f"})
	b, _ := docs.List(ctx, DocListOptions{Kind: "findings", Name: "f", After: "keph:/queue/findings/004-f.md"})
	if a.Total != 6 || b.Total != 6 || len(b.Items) != 2 {
		t.Fatalf("total: %d %d %d", a.Total, b.Total, len(b.Items))
	}
}
