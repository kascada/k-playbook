package project

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
	"github.com/kascada/k-playbook/installer/internal/kephalaion/kephtest"
)

// docsOrdersNow setzt die Uhr des Programms, solange der Test läuft.
func docsOrdersNow(t *testing.T, at time.Time) {
	t.Helper()
	previous := now
	now = func() time.Time { return at }
	t.Cleanup(func() { now = previous })
}

// docsOrdersClock ist 3 Stunden nach dem Zeitstempel, den kephtest an jedes
// Dokument setzt: Ein Auftrag mit diesem Stempel ist reif.
func docsOrdersClock() time.Time {
	stamp, _ := time.Parse(time.RFC3339Nano, kephtest.Stamp)
	return stamp.Add(3 * time.Hour)
}

func rawOrder(session, date string, pfade ...string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "---\ntitle: \"Nachzug %s\"\nerzeuger: code\npfade:\n", pfade[0])
	for _, p := range pfade {
		fmt.Fprintf(&b, "  - %s\n", p)
	}
	fmt.Fprintf(&b, "anlass: \"Anlass für %s\"\nsitzung: %s\nentstanden: %s\n---\n", pfade[0], session, date)
	return b.String()
}

func newDocsOrders(t *testing.T) (*DocsOrders, *kephtest.Store) {
	t.Helper()
	store := kephtest.New()
	orders, err := NewDocsOrders(store, "kamran")
	if err != nil {
		t.Fatal(err)
	}
	return orders, store
}

// putOrder legt einen Auftrag mit einem Zeitpunkt der letzten Änderung an.
func putOrder(store *kephtest.Store, name, content string, updated time.Time) {
	store.Put(name, content)
	store.Docs[name].Updated = updated.UTC().Format(time.RFC3339Nano)
}

// Der Lauf von code: reife und wartende rohe Aufträge, Zusammenlegen
// abhängiger Pfade (gleich oder darin, auch über mehrere hinweg), die volle
// Lauf nimmt die wartenden auf, Wiederaufnahme nur mit zerlegten.
func TestDocsOrdersCode(t *testing.T) {
	at := docsOrdersClock()
	docsOrdersNow(t, at)
	orders, store := newDocsOrders(t)
	old := at.Add(-5 * time.Hour)
	putOrder(store, "queue/docs/001-a.md", rawOrder("s1", "2026-11-20", "installer/a"), old)
	putOrder(store, "queue/docs/002-b.md", rawOrder("s2", "2026-11-18", "installer/a/b"), old)
	putOrder(store, "queue/docs/003-c.md", rawOrder("s3", "2026-11-21", "installer/a/b/c", "docs"), old)
	putOrder(store, "queue/docs/004-d.md", rawOrder("s4", "2026-11-22", "other"), old)
	putOrder(store, "queue/docs/005-wartet.md", rawOrder("s5", "2026-11-23", "waiting/dir"), at.Add(-30*time.Minute))
	store.Put("queue/docs/done/006-fertig.md", rawOrder("s6", "2026-11-01", "x"))
	store.Put("queue/docs/007-tools.md", strings.Replace(rawOrder("s7", "2026-11-20", "go.mod"), "erzeuger: code", "erzeuger: tools", 1))
	store.Put("queue/docs/008-kaputt.md", "kein Frontmatter\n")

	result, err := orders.Orders(context.Background(), "code")
	if err != nil {
		t.Fatal(err)
	}
	if result.Run != DocsRunPartial || result.NothingToDo || result.WaitMinutes != 120 {
		t.Fatalf("Lauf: %+v", result)
	}
	if len(result.Orders) != 5 || len(result.Problems) != 1 || !strings.Contains(result.Problems[0], "008-kaputt.md") {
		t.Errorf("Aufträge %d, Probleme %v", len(result.Orders), result.Problems)
	}
	if len(result.RawOrders) != 2 {
		t.Fatalf("zusammengelegt: %+v", result.RawOrders)
	}
	first := result.RawOrders[0]
	if len(first.Refs) != 3 || first.Entstanden != "2026-11-18" || strings.Join(first.Pfade, ",") != "installer/a,installer/a/b,installer/a/b/c,docs" {
		t.Errorf("erste Gruppe: %+v", first)
	}
	if len(first.Anlaesse) != 3 || first.Anlaesse[0].Anlass != "Anlass für installer/a" || first.Anlaesse[0].Datum != "2026-11-20" {
		t.Errorf("Anlässe: %+v", first.Anlaesse)
	}
	if second := result.RawOrders[1]; len(second.Refs) != 1 || second.Pfade[0] != "other" {
		t.Errorf("zweite Gruppe: %+v", second)
	}
	if len(result.Waiting) != 1 || result.Waiting[0].Ref != "keph:/queue/docs/005-wartet.md" || result.Waiting[0].RemainingMinutes != 90 {
		t.Errorf("wartend: %+v", result.Waiting)
	}

	// Ein roher Auftrag mit . nimmt alle rohen auf, auch die wartenden.
	putOrder(store, "queue/docs/009-voll.md", rawOrder("s9", "2026-11-23", "."), at.Add(-time.Minute))
	result, err = orders.Orders(context.Background(), "code")
	if err != nil {
		t.Fatal(err)
	}
	if result.Run != DocsRunFull || len(result.Waiting) != 0 || len(result.RawOrders) != 1 {
		t.Fatalf("voller Lauf: %+v", result)
	}
	if raw := result.RawOrders[0]; len(raw.Refs) != 6 || len(raw.Pfade) != 1 || raw.Pfade[0] != "." || raw.Entstanden != "2026-11-18" {
		t.Errorf("voller Lauf, Gruppe: %+v", raw)
	}

	// Nur wartende: kein Lauf. Nur zerlegte: Wiederaufnahme.
	only, ostore := newDocsOrders(t)
	putOrder(ostore, "queue/docs/001-w.md", rawOrder("s1", "2026-11-23", "x"), at.Add(-10*time.Minute))
	result, err = only.Orders(context.Background(), "code")
	if err != nil || result.Run != DocsRunNone || !result.NothingToDo || !strings.Contains(result.Reason, "wartende") || result.Waiting[0].RemainingMinutes != 110 {
		t.Errorf("nur wartende: %+v %v", result, err)
	}
	ostore.Put("queue/docs/002-z.md", "---\ntitle: \"z\"\nerzeuger: code\ndokument: 01-z.md\naktion: umschreiben\npfade: [ \"p\" ]\nanlass: \"a\"\nsitzung: s\nentstanden: 2026-11-10\n---\n\n2026-11-10 — Eins\n2026-11-11 — Zwei\n")
	result, err = only.Orders(context.Background(), "code")
	if err != nil || result.Run != DocsRunResume || len(result.Waiting) != 1 {
		t.Fatalf("wartend und zerlegt: %+v %v", result, err)
	}
	delete(ostore.Docs, "queue/docs/001-w.md")
	result, err = only.Orders(context.Background(), "code")
	if err != nil || result.Run != DocsRunResume || len(result.SplitOrders) != 1 || len(result.RawOrders) != 0 {
		t.Fatalf("Wiederaufnahme: %+v %v", result, err)
	}
	if split := result.SplitOrders[0]; split.Dokument != "01-z.md" || split.Aktion != "umschreiben" || len(split.Lines) != 2 || split.Pfade[0] != "p" {
		t.Errorf("zerlegt: %+v", split)
	}

	// Ohne Aufträge, und tools: nur die Liste.
	empty, _ := newDocsOrders(t)
	result, err = empty.Orders(context.Background(), "code")
	if err != nil || !result.NothingToDo || result.Run != DocsRunNone {
		t.Errorf("leer: %+v %v", result, err)
	}
	tools, err := orders.Orders(context.Background(), "tools")
	if err != nil || len(tools.Orders) != 1 || tools.Run != "" || tools.RawOrders != nil || tools.NothingToDo {
		t.Errorf("tools: %+v %v", tools, err)
	}
	if _, err := orders.Orders(context.Background(), "libs"); DocRefErrorCode(err) != DocCodeInvalidInput {
		t.Errorf("unbekannter Erzeuger: %v", err)
	}
}

// Ein Auftrag ohne lesbaren Zeitpunkt gilt als reif; die Frist rechnet mit der
// Uhr des Programms, nicht mit dem Datum im Kopf.
func TestDocsOrdersFrist(t *testing.T) {
	at := docsOrdersClock()
	docsOrdersNow(t, at)
	orders, store := newDocsOrders(t)
	putOrder(store, "queue/docs/001-a.md", rawOrder("s", "2026-11-23", "a"), at.Add(-119*time.Minute))
	putOrder(store, "queue/docs/002-b.md", rawOrder("s", "2026-11-23", "b"), at.Add(-120*time.Minute))
	store.Docs["queue/docs/002-b.md"].Updated = ""
	result, err := orders.Orders(context.Background(), "code")
	if err != nil {
		t.Fatal(err)
	}
	waiting := map[string]bool{}
	for _, order := range result.Orders {
		waiting[order.Ref] = order.Waiting
	}
	if !waiting["keph:/queue/docs/001-a.md"] || waiting["keph:/queue/docs/002-b.md"] {
		t.Errorf("Frist: %v", waiting)
	}
	if result.Waiting[0].RemainingMinutes != 1 {
		t.Errorf("Restzeit: %+v", result.Waiting)
	}
	putOrder(store, "queue/docs/003-c.md", rawOrder("s", "2026-11-23", "c"), at.Add(-120*time.Minute))
	result, _ = orders.Orders(context.Background(), "code")
	for _, order := range result.Orders {
		if order.Ref == "keph:/queue/docs/003-c.md" && order.Waiting {
			t.Error("genau zwei Stunden alt wartet noch")
		}
	}
}

// Ohne lesbare letzte Änderung rechnet die Wartezeit mit dem Anlegezeitpunkt;
// fehlt auch der, ist der Auftrag reif, steht unter problems und wird
// trotzdem abgearbeitet — auch wenn entstanden im Kopf von heute ist.
func TestDocsOrdersFristOhneAenderung(t *testing.T) {
	at := docsOrdersClock()
	docsOrdersNow(t, at)
	today := at.Format("2006-01-02")
	orders, store := newDocsOrders(t)
	put := func(name, pfad, created, updated string) {
		store.Put(name, rawOrder("s", today, pfad))
		store.Docs[name].Created = created
		store.Docs[name].Updated = updated
	}
	stamp := func(d time.Duration) string { return at.Add(-d).UTC().Format(time.RFC3339Nano) }
	put("queue/docs/001-frisch.md", "a", stamp(30*time.Minute), "")
	put("queue/docs/002-alt.md", "b", stamp(DocsOrderWait+time.Minute), "")
	put("queue/docs/003-kaputt-frisch.md", "c", stamp(30*time.Minute), "gestern")
	put("queue/docs/004-kaputt-alt.md", "d", stamp(DocsOrderWait), "gestern")
	put("queue/docs/005-ohne.md", "e", "", "")
	put("queue/docs/006-ohne-kaputt.md", "f", "nie", "gestern")
	result, err := orders.Orders(context.Background(), "code")
	if err != nil {
		t.Fatal(err)
	}
	waiting := map[string]bool{}
	for _, order := range result.Orders {
		waiting[strings.TrimPrefix(order.Ref, "keph:/queue/docs/")] = order.Waiting
	}
	want := map[string]bool{
		"001-frisch.md": true, "002-alt.md": false, "003-kaputt-frisch.md": true,
		"004-kaputt-alt.md": false, "005-ohne.md": false, "006-ohne-kaputt.md": false,
	}
	for name, expected := range want {
		if got, ok := waiting[name]; !ok || got != expected {
			t.Errorf("%s: wartet %v (da: %v), erwartet %v", name, got, ok, expected)
		}
	}
	for _, item := range result.Waiting {
		if item.Ref == "keph:/queue/docs/001-frisch.md" && item.RemainingMinutes != 90 {
			t.Errorf("Restzeit aus dem Anlegezeitpunkt: %+v", item)
		}
	}
	problems := strings.Join(result.Problems, "\n")
	for _, name := range []string{"005-ohne.md", "006-ohne-kaputt.md"} {
		if !strings.Contains(problems, "keph:/queue/docs/"+name+": ohne lesbaren Zeitpunkt") ||
			!strings.Contains(problems, "gilt als reif, wird abgearbeitet") {
			t.Errorf("%s fehlt unter problems: %q", name, problems)
		}
	}
	for _, name := range []string{"001-frisch.md", "002-alt.md", "003-kaputt-frisch.md", "004-kaputt-alt.md"} {
		if strings.Contains(problems, name) {
			t.Errorf("%s steht unter problems: %q", name, problems)
		}
	}
	taken := map[string]bool{}
	for _, raw := range result.RawOrders {
		for _, ref := range raw.Refs {
			taken[strings.TrimPrefix(ref, "keph:/queue/docs/")] = true
		}
	}
	for name, waits := range want {
		if taken[name] == waits {
			t.Errorf("%s: in rawOrders %v, wartet %v", name, taken[name], waits)
		}
	}
	if result.Run != DocsRunPartial {
		t.Errorf("Lauf: %s", result.Run)
	}
}

// docsOrderAdd ruft Add mit den üblichen Eingaben.
func docsOrderAdd(t *testing.T, orders *DocsOrders, producer, session, anlass string, pfade ...string) DocsOrderAddResult {
	t.Helper()
	result, err := orders.Add(context.Background(), DocsOrderAddInput{Producer: producer, Pfade: pfade, Anlass: anlass, Sitzung: session})
	if err != nil {
		t.Fatalf("Add %v: %v", pfade, err)
	}
	return result
}

// Einreihen: nichts ohne erzeugte Doku, ein Auftrag je Sitzung und Erzeuger,
// Ergänzen vereinigt die Pfade, ein deckender Auftrag schreibt nichts, der
// volle Lauf gilt ohne Doku und doppelt sich nicht.
func TestDocsOrderAdd(t *testing.T) {
	docsOrdersNow(t, docsOrdersClock())
	orders, store := newDocsOrders(t)

	skipped := docsOrderAdd(t, orders, "code", "S1", "Neuer Weg", "installer/a")
	if skipped.Status != DocsOrderSkipped || skipped.Reason != DocsReasonNoGeneratedDocs || len(store.Names()) != 0 {
		t.Fatalf("ohne erzeugte Doku: %+v %v", skipped, store.Names())
	}
	store.Put("docs/generated/code/README.md", "x")
	if again := docsOrderAdd(t, orders, "code", "S1", "Neuer Weg", "installer/a"); again.Status != DocsOrderSkipped {
		t.Errorf("README.md ist keine Doku: %+v", again)
	}
	store.Put("docs/generated/code/01-a.md", "---\ntitle: A\ndescription: D\n---\n")

	created := docsOrderAdd(t, orders, "code", "S1", `Neuer "Weg": gebaut`, "installer/a/", "./installer/a")
	if created.Status != DocsOrderCreated || created.Ref != "keph:/queue/docs/001-code-neuer-weg-gebaut.md" || len(created.Pfade) != 1 {
		t.Fatalf("angelegt: %+v", created)
	}
	content, _ := store.Content("queue/docs/001-code-neuer-weg-gebaut.md")
	for _, line := range []string{
		"title: \"Neuer \\\"Weg\\\": gebaut\"", "erzeuger: code", "pfade:\n  - installer/a\n", "anlass: \"Neuer \\\"Weg\\\": gebaut\"", "sitzung: S1", "entstanden: " + docsOrdersClock().Format("2006-01-02") + "\n---\n",
	} {
		if !strings.Contains(content, line) {
			t.Errorf("Kopf ohne %q:\n%s", line, content)
		}
	}

	callsBefore := len(store.Calls)
	covered := docsOrderAdd(t, orders, "code", "S1", "Noch einer", "installer/a/b")
	if covered.Status != DocsOrderCovered || covered.Ref != created.Ref {
		t.Fatalf("gedeckt: %+v", covered)
	}
	for _, call := range store.Calls[callsBefore:] {
		if strings.HasPrefix(call, "write") || strings.HasPrefix(call, "create") {
			t.Errorf("gedeckt, aber geschrieben: %s", call)
		}
	}

	extended := docsOrderAdd(t, orders, "code", "S1", "Zweiter Anlass", "docs/x", "installer/a/c")
	if extended.Status != DocsOrderExtended || extended.Ref != created.Ref || extended.Revision <= created.Revision ||
		strings.Join(extended.Pfade, ",") != "installer/a,docs/x,installer/a/c" {
		t.Fatalf("ergänzt: %+v", extended)
	}
	content, _ = store.Content("queue/docs/001-code-neuer-weg-gebaut.md")
	day := docsOrdersClock().Format("2006-01-02")
	if !strings.Contains(content, "pfade:\n  - installer/a\n  - docs/x\n  - installer/a/c\nanlass:") ||
		!strings.HasSuffix(content, "---\n\n"+day+" — Zweiter Anlass\n") || !strings.Contains(content, "title: \"Neuer") {
		t.Errorf("ergänzter Auftrag:\n%s", content)
	}
	docsOrderAdd(t, orders, "code", "S1", "Dritter", "lib/y")
	content, _ = store.Content("queue/docs/001-code-neuer-weg-gebaut.md")
	if !strings.HasSuffix(content, day+" — Zweiter Anlass\n"+day+" — Dritter\n") {
		t.Errorf("weiterer Anlass:\n%s", content)
	}

	other := docsOrderAdd(t, orders, "code", "S2", "Andere Sitzung", "lib/y")
	if other.Status != DocsOrderCreated || other.Ref == created.Ref {
		t.Errorf("andere Sitzung: %+v", other)
	}

	// tools: erst nach erzeugter Doku, ein Auftrag je Sitzung genügt.
	if tools := docsOrderAdd(t, orders, "tools", "S1", "Neue Abhängigkeit", "go.mod"); tools.Status != DocsOrderSkipped || tools.Reason != DocsReasonNoGeneratedDocs {
		t.Errorf("tools ohne Doku: %+v", tools)
	}
	store.Put("docs/generated/libs/yamllite.md", "x")
	tools := docsOrderAdd(t, orders, "tools", "S1", "Neue Abhängigkeit", "go.mod")
	if tools.Status != DocsOrderCreated {
		t.Fatalf("tools: %+v", tools)
	}
	if again := docsOrderAdd(t, orders, "tools", "S1", "Noch eine", "go.sum"); again.Status != DocsOrderCovered || again.Ref != tools.Ref {
		t.Errorf("tools doppelt: %+v", again)
	}

	// Der volle Lauf: auch ohne erzeugte Doku, einmal.
	fresh, fstore := newDocsOrders(t)
	full := docsOrderAdd(t, fresh, "code", "S1", "Voller Lauf", ".", "src")
	if full.Status != DocsOrderCreated || len(full.Pfade) != 1 || full.Pfade[0] != "." {
		t.Fatalf("voller Lauf: %+v", full)
	}
	if again := docsOrderAdd(t, fresh, "code", "S9", "Voller Lauf", "."); again.Status != DocsOrderCovered || again.Ref != full.Ref {
		t.Errorf("voller Lauf doppelt: %+v", again)
	}
	if len(fstore.Names()) != 1 {
		t.Errorf("Aufträge: %v", fstore.Names())
	}
}

// Eingaben ohne Anlass, Sitzung, Pfad oder mit einem Pfad aus dem Projekt
// heraus sind invalid_input; geschrieben wird nichts.
func TestDocsOrderAddEingaben(t *testing.T) {
	orders, store := newDocsOrders(t)
	store.Put("docs/generated/code/01-a.md", "x")
	for _, in := range []DocsOrderAddInput{
		{Producer: "code", Pfade: []string{"a"}, Anlass: " ", Sitzung: "S"},
		{Producer: "code", Pfade: []string{"a"}, Anlass: "x"},
		{Producer: "code", Anlass: "x", Sitzung: "S"},
		{Producer: "code", Pfade: []string{"../a"}, Anlass: "x", Sitzung: "S"},
		{Producer: "code", Pfade: []string{"/a"}, Anlass: "x", Sitzung: "S"},
		{Producer: "code", Pfade: []string{" "}, Anlass: "x", Sitzung: "S"},
		{Producer: "libs", Pfade: []string{"a"}, Anlass: "x", Sitzung: "S"},
	} {
		if _, err := orders.Add(context.Background(), in); DocRefErrorCode(err) != DocCodeInvalidInput {
			t.Errorf("%+v: %v", in, err)
		}
	}
	if len(store.Names()) != 1 {
		t.Errorf("geschrieben: %v", store.Names())
	}
	unavailable := DocsOrderAddUnavailable("code", &DocRefError{Code: DocCodeKephalaionUnavailable, err: fmt.Errorf("nicht bereit")})
	if unavailable.Status != DocsOrderSkipped || unavailable.Reason != DocsReasonKephalaionUnavailable || unavailable.Message != "nicht bereit" {
		t.Errorf("ohne Kephalaion: %+v", unavailable)
	}
}

// staleOnce lässt das erste Write am Revisionsvergleich scheitern, weil ein
// anderer vorher geschrieben hat.
type staleOnce struct {
	*kephtest.Store
	name   string
	extra  string
	nWrite int
}

func (s *staleOnce) Write(ctx context.Context, name, content string, base int64) (kephalaion.Document, error) {
	s.nWrite++
	if s.nWrite == 1 && name == s.name {
		current, _ := s.Store.Content(name)
		s.Store.Put(name, current+s.extra)
	}
	return s.Store.Write(ctx, name, content, base)
}

// Bei stale_revision liest das Ergänzen neu und ergänzt auf dem neuen Stand.
func TestDocsOrderAddStaleRevision(t *testing.T) {
	docsOrdersNow(t, docsOrdersClock())
	store := kephtest.New()
	store.Put("docs/generated/code/01-a.md", "x")
	wrapped := &staleOnce{Store: store, name: "queue/docs/001-code-erster.md", extra: "2026-11-01 — Fremder Anlass\n"}
	orders, err := NewDocsOrders(wrapped, "kamran")
	if err != nil {
		t.Fatal(err)
	}
	docsOrderAdd(t, orders, "code", "S1", "Erster", "a")
	result := docsOrderAdd(t, orders, "code", "S1", "Zweiter", "b")
	if result.Status != DocsOrderExtended || wrapped.nWrite < 2 {
		t.Fatalf("stale: %+v, Schreibversuche %d", result, wrapped.nWrite)
	}
	content, _ := store.Content("queue/docs/001-code-erster.md")
	if !strings.Contains(content, "Fremder Anlass") || !strings.Contains(content, "— Zweiter") || !strings.Contains(content, "  - b\n") {
		t.Errorf("nach stale:\n%s", content)
	}
}

// Zerlegte Aufträge: anlegen mit Kopf und Rumpf, abgelehnte schließen, rohe
// nach done/, ergänzen statt verdoppeln, ein Fehler hält die rohen offen.
func TestDocsOrdersSplit(t *testing.T) {
	at := docsOrdersClock()
	docsOrdersNow(t, at)
	orders, store := newDocsOrders(t)
	store.Put("queue/docs/001-a.md", rawOrder("s1", "2026-11-18", "installer/a"))
	store.Put("queue/docs/002-b.md", rawOrder("s2", "2026-11-20", "docs"))
	store.Put("queue/docs/003-nix.md", rawOrder("s3", "2026-11-21", "vendor"))

	result, err := orders.Split(context.Background(), DocsSplitInput{
		Sitzung: "V1",
		Documents: []DocsSplitDocument{
			{Dokument: "11-installation.md", Aktion: "umschreiben", Pfade: []string{"installer/a"},
				Anlaesse: []DocsOrderLine{{Datum: "2026-11-18", Anlass: "Anlass A"}, {Datum: "2026-11-20", Anlass: "Anlass B"}}},
			{Dokument: "12-neu.md", Aktion: "neu", Pfade: []string{"docs/"}, Anlaesse: []DocsOrderLine{{Datum: "2026-11-20", Anlass: "Anlass B"}}, Abgelehnt: true},
			{Dokument: "07-alt.md", Aktion: "entfällt", Pfade: []string{"installer/a"}, Anlass: "Thema weg", Entstanden: "2026-11-18"},
		},
		Raw: []string{"keph:/queue/docs/001-a.md", "keph:/queue/docs/002-b.md", "keph:/queue/docs/003-nix.md"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Documents) != 3 || len(result.ClosedRaw) != 3 || len(result.Raw) != 3 {
		t.Fatalf("Ergebnis: %+v", result)
	}
	first := result.Documents[0]
	if first.Status != DocsOrderCreated || first.Aktion != "umschreiben" || first.Closed || !strings.HasPrefix(first.Ref, "keph:/queue/docs/") {
		t.Errorf("erstes Dokument: %+v", first)
	}
	name := strings.TrimPrefix(first.Ref, "keph:/")
	content, ok := store.Content(name)
	if !ok || !strings.HasSuffix(name, "-code-11-installation.md") {
		t.Fatalf("Auftrag fehlt: %s %v", name, store.Names())
	}
	for _, want := range []string{
		"title: \"11-installation umschreiben\"\n", "erzeuger: code\ndokument: 11-installation.md\naktion: umschreiben\n", "pfade:\n  - installer/a\n",
		"anlass: \"Anlass A; Anlass B\"\n", "sitzung: V1\nentstanden: 2026-11-18\n---\n\n2026-11-18 — Anlass A\n2026-11-20 — Anlass B\n",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("Kopf ohne %q:\n%s", want, content)
		}
	}
	second := result.Documents[1]
	if second.Status != DocsOrderCreated || !second.Closed || !strings.Contains(second.DoneRef, "queue/docs/done/") {
		t.Errorf("abgelehnt: %+v", second)
	}
	if _, ok := store.Content(strings.TrimPrefix(second.Ref, "keph:/")); ok {
		t.Error("abgelehnter Auftrag liegt noch offen")
	}
	if third := result.Documents[2]; third.Aktion != "entfaellt" || third.Status != DocsOrderCreated {
		t.Errorf("entfällt: %+v", third)
	}
	for _, raw := range result.Raw {
		if raw.Status != DocsRawClosed || !strings.Contains(raw.DoneRef, "queue/docs/done/") {
			t.Errorf("roh: %+v", raw)
		}
	}
	if _, ok := store.Content("queue/docs/001-a.md"); ok {
		t.Error("roher Auftrag nicht verschoben")
	}

	// Zweiter Lauf: ergänzt statt zu verdoppeln; ohne Neues nichts.
	again, err := orders.Split(context.Background(), DocsSplitInput{
		Sitzung: "V2",
		Documents: []DocsSplitDocument{
			{Dokument: "11-installation.md", Aktion: "umschreiben", Pfade: []string{"installer/b", "installer/a"},
				Anlaesse: []DocsOrderLine{{Datum: "2026-11-20", Anlass: "Anlass B"}, {Datum: "2026-11-17", Anlass: "Anlass C"}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	doc := again.Documents[0]
	if doc.Status != DocsOrderExtended || doc.Ref != first.Ref || doc.Revision <= first.Revision {
		t.Fatalf("ergänzt: %+v", doc)
	}
	content, _ = store.Content(name)
	if !strings.Contains(content, "pfade:\n  - installer/a\n  - installer/b\n") || !strings.Contains(content, "entstanden: 2026-11-17\n") ||
		strings.Count(content, "Anlass B") != 2 /* anlass im Kopf und eine Zeile */ || !strings.HasSuffix(content, "2026-11-17 — Anlass C\n") ||
		!strings.Contains(content, "sitzung: V1\n") {
		t.Errorf("ergänzter Auftrag:\n%s", content)
	}
	same, err := orders.Split(context.Background(), DocsSplitInput{
		Sitzung: "V3",
		Documents: []DocsSplitDocument{{Dokument: "11-installation.md", Aktion: "umschreiben", Pfade: []string{"installer/a"},
			Anlaesse: []DocsOrderLine{{Datum: "2026-11-17", Anlass: "Anlass C"}}}},
	})
	if err != nil || same.Documents[0].Status != "skipped" || same.Documents[0].Reason != DocsReasonNothingToAdd {
		t.Errorf("nichts zu ergänzen: %+v %v", same, err)
	}
}

// Ein zerlegter Auftrag, der nicht geschrieben werden kann, hält die rohen
// offen; ein roher, den ein anderer Lauf schon geschlossen hat, ist gone.
func TestDocsOrdersSplitFehler(t *testing.T) {
	docsOrdersNow(t, docsOrdersClock())
	orders, store := newDocsOrders(t)
	store.Put("queue/docs/001-a.md", rawOrder("s1", "2026-11-18", "a"))
	input := DocsSplitInput{Sitzung: "V1", Raw: []string{"keph:/queue/docs/001-a.md"},
		Documents: []DocsSplitDocument{{Dokument: "01-a.md", Aktion: "neu", Pfade: []string{"a"}, Anlass: "x"}}}

	store.Fail["create"] = &kephalaion.ToolError{Tool: "create", Code: kephalaion.CodeUnreachable, Message: "weg"}
	result, err := orders.Split(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if result.Documents[0].Status != DocsSplitFailed || result.Documents[0].Code != DocCodeKephalaionUnavailable ||
		result.Raw[0].Status != DocsRawKept || len(result.ClosedRaw) != 0 {
		t.Fatalf("Fehler: %+v", result)
	}
	if _, ok := store.Content("queue/docs/001-a.md"); !ok {
		t.Error("roher Auftrag trotz Fehler verschoben")
	}

	delete(store.Fail, "create")
	input.Raw = append(input.Raw, "keph:/queue/docs/099-weg.md")
	result, err = orders.Split(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if result.Documents[0].Status != DocsOrderCreated || result.Raw[0].Status != DocsRawClosed || result.Raw[1].Status != DocsRawGone {
		t.Errorf("gone: %+v", result)
	}
}

// Eingaben werden geprüft, bevor etwas geschrieben wird.
func TestDocsOrdersSplitEingaben(t *testing.T) {
	orders, store := newDocsOrders(t)
	store.Put("queue/docs/001-a.md", rawOrder("s1", "2026-11-18", "a"))
	store.Put("queue/docs/002-z.md", "---\ntitle: \"z\"\nerzeuger: code\ndokument: 01-z.md\naktion: neu\npfade: [ \"p\" ]\nanlass: \"a\"\nsitzung: s\nentstanden: 2026-11-10\n---\n")
	good := DocsSplitDocument{Dokument: "01-a.md", Aktion: "neu", Pfade: []string{"a"}, Anlass: "x"}
	before := len(store.Names())
	for name, in := range map[string]DocsSplitInput{
		"ohne Sitzung":          {Documents: []DocsSplitDocument{good}},
		"README":                {Sitzung: "V", Documents: []DocsSplitDocument{{Dokument: "README.md", Aktion: "neu", Pfade: []string{"a"}, Anlass: "x"}}},
		"Großbuchstaben":        {Sitzung: "V", Documents: []DocsSplitDocument{{Dokument: "01-A.md", Aktion: "neu", Pfade: []string{"a"}, Anlass: "x"}}},
		"Verzeichnis":           {Sitzung: "V", Documents: []DocsSplitDocument{{Dokument: "x/01-a.md", Aktion: "neu", Pfade: []string{"a"}, Anlass: "x"}}},
		"unbekannte Aktion":     {Sitzung: "V", Documents: []DocsSplitDocument{{Dokument: "01-a.md", Aktion: "löschen", Pfade: []string{"a"}, Anlass: "x"}}},
		"umschreiben abgelehnt": {Sitzung: "V", Documents: []DocsSplitDocument{{Dokument: "01-a.md", Aktion: "umschreiben", Pfade: []string{"a"}, Anlass: "x", Abgelehnt: true}}},
		"ohne Pfad":             {Sitzung: "V", Documents: []DocsSplitDocument{{Dokument: "01-a.md", Aktion: "neu", Anlass: "x"}}},
		"ohne Anlass":           {Sitzung: "V", Documents: []DocsSplitDocument{{Dokument: "01-a.md", Aktion: "neu", Pfade: []string{"a"}}}},
		"doppelt":               {Sitzung: "V", Documents: []DocsSplitDocument{good, good}},
		"roh: Task":             {Sitzung: "V", Raw: []string{"keph:/user/kamran/tasks/001-x.md"}},
		"roh: done":             {Sitzung: "V", Raw: []string{"keph:/queue/docs/done/001-x.md"}},
		"roh: Verzeichnis":      {Sitzung: "V", Raw: []string{"keph:/queue/docs/"}},
		"roh: zerlegt":          {Sitzung: "V", Raw: []string{"keph:/queue/docs/002-z.md"}},
	} {
		if _, err := orders.Split(context.Background(), in); DocRefErrorCode(err) != DocCodeInvalidInput {
			t.Errorf("%s: %v (%s)", name, err, DocRefErrorCode(err))
		}
	}
	if len(store.Names()) != before {
		t.Errorf("geschrieben: %v", store.Names())
	}
	// Ohne Dokument gehen die rohen trotzdem nach done/ (alle Pfade ausgenommen).
	result, err := orders.Split(context.Background(), DocsSplitInput{Sitzung: "V", Raw: []string{"keph:/queue/docs/001-a.md"}})
	if err != nil || len(result.ClosedRaw) != 1 || len(result.Documents) != 0 {
		t.Errorf("ohne Dokument: %+v %v", result, err)
	}
}

// Sitzung und entstanden können keinen Kopf einschleusen: die Sitzung wird,
// wenn sie nicht schlicht ist, gequotet, entstanden muss ein Datum sein.
func TestDocsOrdersKopfEinschleusen(t *testing.T) {
	docsOrdersNow(t, docsOrdersClock())
	orders, store := newDocsOrders(t)
	for _, in := range []DocsSplitInput{
		{Sitzung: "V", Documents: []DocsSplitDocument{{Dokument: "01-a.md", Aktion: "neu", Pfade: []string{"a"}, Anlass: "x", Entstanden: "2026-11-10\nsitzung: x"}}},
		{Sitzung: "V", Documents: []DocsSplitDocument{{Dokument: "01-a.md", Aktion: "neu", Pfade: []string{"a"}, Anlass: "x", Entstanden: "gestern"}}},
		{Sitzung: "V", Documents: []DocsSplitDocument{{Dokument: "01-a.md", Aktion: "neu", Pfade: []string{"a"}, Anlaesse: []DocsOrderLine{{Datum: "x: y", Anlass: "x"}}}}},
	} {
		if _, err := orders.Split(context.Background(), in); DocRefErrorCode(err) != DocCodeInvalidInput {
			t.Errorf("%+v: %v", in, err)
		}
	}
	if len(store.Names()) != 0 {
		t.Errorf("geschrieben: %v", store.Names())
	}

	evil := "S1\nerzeuger: tools: x"
	result, err := orders.Split(context.Background(), DocsSplitInput{Sitzung: evil,
		Documents: []DocsSplitDocument{{Dokument: "01-a.md", Aktion: "neu", Pfade: []string{"a"}, Anlass: "x", Entstanden: "2026-11-10"}}})
	if err != nil || result.Documents[0].Status != DocsOrderCreated {
		t.Fatalf("Split: %+v %v", result, err)
	}
	content, _ := store.Content(strings.TrimPrefix(result.Documents[0].Ref, "keph:/"))
	if !strings.Contains(content, "sitzung: \"S1 erzeuger: tools: x\"\n") || strings.Count(content, "\nerzeuger:") != 1 {
		t.Errorf("Kopf: %s", content)
	}
	if !strings.Contains(content, "entstanden: 2026-11-10\n") {
		t.Errorf("entstanden: %s", content)
	}
}

// Ein offener Auftrag ohne gültigen Pfad ist unvollständig: Er bleibt in der
// Liste, trägt incomplete und steht unter problems. Ein weggelassener
// ungültiger Pfad steht mit Grund unter problems. Beides nur für Aufträge des
// gefragten Erzeugers (Task 193).
func TestDocsOrdersUnvollstaendig(t *testing.T) {
	at := docsOrdersClock()
	docsOrdersNow(t, at)
	orders, store := newDocsOrders(t)
	old := at.Add(-5 * time.Hour)
	head := func(producer, pfade string) string {
		return "---\ntitle: \"t\"\nerzeuger: " + producer + "\n" + pfade + "anlass: \"a\"\nsitzung: s\nentstanden: 2026-11-20\n---\n"
	}
	putOrder(store, "queue/docs/001-ohne.md", head("code", ""), old)
	putOrder(store, "queue/docs/002-leer.md", head("code", "pfade: []\n"), old)
	putOrder(store, "queue/docs/003-ungueltig.md", head("code", "pfade:\n  - /abs/pfad\n  - ../draussen\n"), old)
	putOrder(store, "queue/docs/004-gemischt.md", head("code", "pfade:\n  - installer/a\n  - /abs/b\n"), old)
	putOrder(store, "queue/docs/005-zerlegt.md", "---\ntitle: \"z\"\nerzeuger: code\ndokument: 01-z.md\naktion: umschreiben\nanlass: \"a\"\nsitzung: s\nentstanden: 2026-11-10\n---\n", old)
	putOrder(store, "queue/docs/006-tools.md", head("tools", "pfade:\n  - /abs/tools\n"), old)
	putOrder(store, "queue/docs/007-tools-ohne.md", head("tools", ""), old)

	result, err := orders.Orders(context.Background(), "code")
	if err != nil {
		t.Fatal(err)
	}
	incomplete := map[string]bool{}
	for _, order := range result.Orders {
		incomplete[strings.TrimPrefix(order.Ref, "keph:/queue/docs/")] = order.Incomplete
	}
	want := map[string]bool{"001-ohne.md": true, "002-leer.md": true, "003-ungueltig.md": true, "004-gemischt.md": false, "005-zerlegt.md": true}
	if len(incomplete) != len(want) {
		t.Errorf("Aufträge: %v", incomplete)
	}
	for name, expected := range want {
		if got, ok := incomplete[name]; !ok || got != expected {
			t.Errorf("%s: incomplete %v (da: %v), erwartet %v", name, got, ok, expected)
		}
	}
	problems := strings.Join(result.Problems, "\n")
	for _, name := range []string{"001-ohne.md", "002-leer.md", "003-ungueltig.md", "005-zerlegt.md"} {
		if !strings.Contains(problems, "keph:/queue/docs/"+name+": ohne gültigen Pfad — unvollständig") {
			t.Errorf("%s: Zeile „unvollständig“ fehlt: %q", name, problems)
		}
	}
	if strings.Contains(problems, "004-gemischt.md: ohne gültigen Pfad") {
		t.Errorf("gemischter Auftrag als unvollständig gemeldet: %q", problems)
	}
	for _, line := range []string{
		`keph:/queue/docs/003-ungueltig.md: Pfad weggelassen — "/abs/pfad"`,
		`keph:/queue/docs/003-ungueltig.md: Pfad weggelassen — "../draussen" führt aus project.repoRoot heraus`,
		`keph:/queue/docs/004-gemischt.md: Pfad weggelassen — "/abs/b"`,
	} {
		if !strings.Contains(problems, line) {
			t.Errorf("Zeile %q fehlt: %q", line, problems)
		}
	}
	if strings.Contains(problems, "tools") {
		t.Errorf("Zeilen zu tools in der Antwort für code: %q", problems)
	}
	// Ein unvollständiger roher Auftrag bleibt eine eigene Gruppe.
	groups := 0
	for _, raw := range result.RawOrders {
		if len(raw.Pfade) == 0 {
			groups++
		}
	}
	if groups != 3 {
		t.Errorf("Gruppen ohne Pfad: %d, %+v", groups, result.RawOrders)
	}

	tools, err := orders.Orders(context.Background(), "tools")
	if err != nil {
		t.Fatal(err)
	}
	problems = strings.Join(tools.Problems, "\n")
	if !strings.Contains(problems, "006-tools.md: Pfad weggelassen") || !strings.Contains(problems, "007-tools-ohne.md: ohne gültigen Pfad") ||
		strings.Contains(problems, "003-ungueltig.md") || tools.NothingToDo {
		t.Errorf("tools: %+v", tools)
	}
}

// docsOrdersRepo legt ein Repo-Verzeichnis namens myrepo mit Dateien an und
// gibt seinen Pfad zurück.
func docsOrdersRepo(t *testing.T, files ...string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "myrepo")
	for _, file := range files {
		full := filepath.Join(root, filepath.FromSlash(file))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// Berichtigt wird nur der eindeutige Fall: das Repo-Verzeichnis ein zweites
// Mal vorangestellt, den Pfad gibt es so nicht, ohne das erste Segment schon.
func TestDocsRepoPrefixFix(t *testing.T) {
	root := docsOrdersRepo(t, "installer/a.go", "myrepo/both.go", "both.go", "lib/x.go")
	cases := []struct {
		in, want string
		ok       bool
	}{
		{"myrepo/installer/a.go", "installer/a.go", true},
		{"myrepo/installer", "installer", true},
		{"myrepo/both.go", "", false},       // beide Schreibweisen
		{"myrepo/nope.go", "", false},       // keine
		{"other/installer/a.go", "", false}, // erstes Segment nicht der Basisname
		{"installer/a.go", "", false},       // gibt es
		{"myrepo", "", false},               // kein Rest
		{".", "", false},
	}
	for _, c := range cases {
		got, ok := docsRepoPrefixFix(root, c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("%s: %q %v, erwartet %q %v", c.in, got, ok, c.want, c.ok)
		}
	}
	if _, ok := docsRepoPrefixFix("", "myrepo/installer/a.go"); ok {
		t.Error("ohne repoRoot berichtigt")
	}
}

// load berichtigt die Pfade jedes Auftrags, Orders nennt die Berichtigung nur
// für den gefragten Erzeuger; Add berichtigt die Eingabe, ein berichtigter
// Pfad ist vom offenen Auftrag gedeckt, ein fehlender, nicht berichtigbarer
// wird unverändert angenommen und unter missing genannt (Task 193).
func TestDocsOrdersBerichtigung(t *testing.T) {
	at := docsOrdersClock()
	docsOrdersNow(t, at)
	orders, store := newDocsOrders(t)
	orders.repoRoot = docsOrdersRepo(t, "installer/a.go", "installer/b.go", "go.mod")
	old := at.Add(-5 * time.Hour)
	putOrder(store, "queue/docs/001-doppelt.md", rawOrder("S1", "2026-11-20", "myrepo/installer/a.go", "installer/a.go", "geloescht.go"), old)
	store.Put("queue/docs/002-tools.md", strings.Replace(rawOrder("S2", "2026-11-20", "myrepo/go.mod"), "erzeuger: code", "erzeuger: tools", 1))
	store.Put("docs/generated/code/01-a.md", "---\ntitle: A\ndescription: D\n---\n")

	result, err := orders.Orders(context.Background(), "code")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Orders) != 1 || strings.Join(result.Orders[0].Pfade, ",") != "installer/a.go,geloescht.go" {
		t.Fatalf("berichtigte Pfade: %+v", result.Orders)
	}
	problems := strings.Join(result.Problems, "\n")
	if !strings.Contains(problems, `keph:/queue/docs/001-doppelt.md: Pfad "myrepo/installer/a.go" berichtigt zu "installer/a.go"`) {
		t.Errorf("Berichtigung fehlt: %q", problems)
	}
	if strings.Contains(problems, "go.mod") || strings.Contains(problems, "geloescht.go") {
		t.Errorf("fremder Erzeuger oder gelöschter Pfad unter problems: %q", problems)
	}
	content, _ := store.Content("queue/docs/001-doppelt.md")
	if !strings.Contains(content, "myrepo/installer/a.go") {
		t.Error("Auftrag in der Queue umgeschrieben")
	}
	tools, _ := orders.Orders(context.Background(), "tools")
	if len(tools.Orders) != 1 || tools.Orders[0].Pfade[0] != "go.mod" || !strings.Contains(strings.Join(tools.Problems, "\n"), "berichtigt zu \"go.mod\"") {
		t.Errorf("tools: %+v", tools)
	}

	covered := docsOrderAdd(t, orders, "code", "S1", "Noch einmal", "myrepo/installer/a.go")
	if covered.Status != DocsOrderCovered || len(covered.Corrected) != 1 ||
		covered.Corrected[0] != (DocsPathCorrection{From: "myrepo/installer/a.go", To: "installer/a.go"}) || covered.Missing != nil {
		t.Errorf("gedeckt nach Berichtigung: %+v", covered)
	}

	extended := docsOrderAdd(t, orders, "code", "S1", "Weg", "myrepo/installer/b.go", "neu/fehlt.go")
	if extended.Status != DocsOrderExtended || strings.Join(extended.Missing, ",") != "neu/fehlt.go" ||
		strings.Join(extended.Pfade, ",") != "installer/a.go,geloescht.go,installer/b.go,neu/fehlt.go" {
		t.Errorf("ergänzt: %+v", extended)
	}
	content, _ = store.Content("queue/docs/001-doppelt.md")
	if strings.Contains(content, "  - myrepo/") || !strings.Contains(content, "  - neu/fehlt.go\n") {
		t.Errorf("ergänzter Auftrag trägt nicht die berichtigten Pfade:\n%s", content)
	}

	created := docsOrderAdd(t, orders, "code", "S3", "Gelöscht", "weg/datei.go")
	if created.Status != DocsOrderCreated || strings.Join(created.Pfade, ",") != "weg/datei.go" ||
		strings.Join(created.Missing, ",") != "weg/datei.go" || created.Corrected != nil {
		t.Errorf("fehlender Pfad: %+v", created)
	}
}
