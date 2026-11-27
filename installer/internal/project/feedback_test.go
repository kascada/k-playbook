package project

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/kascada/k-playbook/installer/internal/kephalaion/kephtest"
)

// useFeedbackStore setzt für den Test einen Store im Speicher und eine feste
// Version ein.
func useFeedbackStore(t *testing.T) *kephtest.Store {
	t.Helper()
	store := kephtest.New()
	original, originalVersion := OpenFeedbackStore, FeedbackVersion
	OpenFeedbackStore = func(string) (FeedbackStore, FeedbackIdentity, error) {
		return store, FeedbackIdentity{User: "kamran", Account: "kamran-wsl", Address: "vm:k-playbook-dev"}, nil
	}
	FeedbackVersion = func() string { return "v0.16.60" }
	t.Cleanup(func() { OpenFeedbackStore, FeedbackVersion = original, originalVersion })
	return store
}

func validFeedbackOptions() FeedbackCreateOptions {
	return FeedbackCreateOptions{Title: "Sync hängt: Replica läuft nach #2", Art: "fehler", Dringlichkeit: "hoch",
		Betrifft: "kephalaion", Body: "Was passiert ist.\n\nWas erwartet war.", Von: "ki", Client: "claude-code",
		Modell: "claude-opus-5-5", Sitzung: "20261127-141205-619d6d"}
}

// Bauen und Lesen sind ein Paar: Was composeFeedbackDoc schreibt, liest
// parseFeedbackDoc unverändert zurück, auch einen Titel mit „: “, „ #“ und
// Anführungszeichen.
func TestFeedbackDocBauenUndLesen(t *testing.T) {
	fields := map[string]string{
		"title": `Titel: mit "Zitat" und #3 \ Rest`, "art": "wunsch", "dringlichkeit": "niedrig", "betrifft": "k-playbook",
		"status": "offen", "von": "mensch", "user": "kamran", "account": "kamran-wsl", "sitzung": "s-1",
		"version": "v0.16.60",
	}
	content := composeFeedbackDoc("007", fields, nil, "Text der Meldung.\n")
	if !strings.Contains(content, `title: "Titel: mit \"Zitat\" und #3 \\ Rest"`+"\n") {
		t.Fatalf("title nicht gequotet:\n%s", content)
	}
	if !strings.Contains(content, "art: wunsch\n") || strings.Contains(content, `art: "`) {
		t.Errorf("Aufzählung gequotet oder fehlt:\n%s", content)
	}
	if strings.Contains(content, "client:") || strings.Contains(content, "modell:") {
		t.Errorf("leere Felder geschrieben:\n%s", content)
	}
	if !strings.Contains(content, "\n# Meldung 007 — Titel: mit \"Zitat\" und #3 \\ Rest\n\nText der Meldung.\n") ||
		strings.HasSuffix(content, "\n\n") {
		t.Errorf("Überschrift oder Rumpf falsch:\n%s", content)
	}
	got := feedbackFromDoc(parseFeedbackDoc(content))
	if got.Title != fields["title"] || got.Art != "wunsch" || got.Von != "mensch" || got.Sitzung != "s-1" ||
		got.Version != "v0.16.60" || got.Body != "Text der Meldung." || got.Problem != "" {
		t.Fatalf("zurückgelesen: %+v", got)
	}
}

// Fremde Felder bleiben beim Lesen und neuen Schreiben erhalten; ein
// unlesbarer Kopf ist kein Fehler, sondern ein Problem.
func TestFeedbackDocBehaeltFremdesUndMeldetKaputtes(t *testing.T) {
	doc := parseFeedbackDoc("---\ntitle: \"A\"\nstatus: offen\nnotiz:\n  - von Hand\n---\n\n# Meldung 001 — A\n\nText\n")
	if doc.fields["status"] != "offen" || len(doc.extra) != 2 || doc.extra[0] != "notiz:" {
		t.Fatalf("fremde Felder: %+v", doc)
	}
	again := composeFeedbackDoc("001", doc.fields, doc.extra, doc.body)
	if !strings.Contains(again, "notiz:\n  - von Hand\n---") {
		t.Errorf("fremdes Feld verloren:\n%s", again)
	}
	broken := parseFeedbackDoc("---\ntitle: \"A\"\nnotiz: von Hand\n  zweite: zeile\n---\nText\n")
	if broken.problem == "" || broken.body != "Text" {
		t.Errorf("kaputter Kopf: %+v", broken)
	}
	if none := parseFeedbackDoc("nur Text\n"); none.problem != "kein Kopf" || none.body != "nur Text" {
		t.Errorf("ohne Kopf: %+v", none)
	}
}

// Ungültige Werte kommen gesammelt als invalid_input, und es wird nichts
// angelegt.
func TestFeedbackCreateWeistUngueltigesAb(t *testing.T) {
	store := useFeedbackStore(t)
	ctx := context.Background()
	cases := map[string]func(*FeedbackCreateOptions){
		"art":                func(o *FeedbackCreateOptions) { o.Art = "bug" },
		"dringlichkeit":      func(o *FeedbackCreateOptions) { o.Dringlichkeit = "sofort" },
		"betrifft":           func(o *FeedbackCreateOptions) { o.Betrifft = "omni" },
		"von":                func(o *FeedbackCreateOptions) { o.Von = "" },
		"title fehlt":        func(o *FeedbackCreateOptions) { o.Title = "  " },
		"title mehrzeilig":   func(o *FeedbackCreateOptions) { o.Title = "a\nb" },
		"body fehlt":         func(o *FeedbackCreateOptions) { o.Body = "\n" },
		"client bei mensch":  func(o *FeedbackCreateOptions) { o.Von = "mensch" },
		"sitzung mehrzeilig": func(o *FeedbackCreateOptions) { o.Sitzung = "a\nb" },
	}
	for name, mutate := range cases {
		opts := validFeedbackOptions()
		mutate(&opts)
		_, err := CreateFeedback(ctx, "", opts)
		if DocRefErrorCode(err) != DocCodeInvalidInput {
			t.Errorf("%s: erwartet invalid_input, bekommen %v", name, err)
		}
	}
	opts := validFeedbackOptions()
	opts.Art, opts.Betrifft = "x", "y"
	_, err := CreateFeedback(ctx, "", opts)
	if err == nil || !strings.Contains(err.Error(), "art") || !strings.Contains(err.Error(), "betrifft") {
		t.Errorf("Verstöße nicht gesammelt: %v", err)
	}
	if len(store.Docs) != 0 {
		t.Fatalf("angelegt trotz Fehler: %v", store.Names())
	}
}

// Anlegen schreibt den fertigen Inhalt in einem Zug unter der nächsten
// Nummer aus feedback/ samt done/; den Kopf füllt das Programm.
func TestFeedbackCreateLegtInEinemZugAn(t *testing.T) {
	store := useFeedbackStore(t)
	store.Put("feedback/done/004-alt.md", "---\ntitle: \"Alt\"\n---\n")
	created, err := CreateFeedback(context.Background(), "", validFeedbackOptions())
	if err != nil {
		t.Fatal(err)
	}
	if created.Number != 5 || created.Name != "005-sync-haengt-replica-laeuft-nach-2.md" ||
		created.Ref != "keph:/feedback/005-sync-haengt-replica-laeuft-nach-2.md" || created.Revision == 0 {
		t.Fatalf("Antwort: %+v", created)
	}
	creates := 0
	for _, call := range store.Calls {
		if strings.HasPrefix(call, "create ") {
			creates++
		}
		if strings.HasPrefix(call, "write ") {
			t.Errorf("zweiter Schreibaufruf: %s", call)
		}
	}
	if creates != 1 {
		t.Errorf("create-Aufrufe: %v", store.Calls)
	}
	content, _ := store.Content("feedback/005-sync-haengt-replica-laeuft-nach-2.md")
	want := "---\n" +
		"title: \"Sync hängt: Replica läuft nach #2\"\n" +
		"art: fehler\ndringlichkeit: hoch\nbetrifft: kephalaion\nstatus: offen\nvon: ki\n" +
		"user: \"kamran\"\naccount: \"kamran-wsl\"\nclient: \"claude-code\"\nmodell: \"claude-opus-5-5\"\n" +
		"sitzung: \"20261127-141205-619d6d\"\nversion: \"v0.16.60\"\n" +
		"---\n\n# Meldung 005 — Sync hängt: Replica läuft nach #2\n\nWas passiert ist.\n\nWas erwartet war.\n"
	if content != want {
		t.Fatalf("Inhalt:\n%s\nerwartet:\n%s", content, want)
	}
	if f := created.Feedback; f.User != "kamran" || f.Account != "kamran-wsl" || f.Status != "offen" || f.Version != "v0.16.60" {
		t.Errorf("Kopf der Antwort: %+v", f)
	}
}

// Die Sorte steht in der Tabelle mit eigenen Werkzeugen; die doc-Werkzeuge
// erreichen sie nicht.
func TestDocKindFeedbackNurUeberEigeneWerkzeuge(t *testing.T) {
	var feedback DocKind
	for _, kind := range DocKinds {
		if kind.Kind == FeedbackKindName {
			feedback = kind
		}
	}
	want := DocKind{Kind: "feedback", Dir: "feedback", UserArea: false, Counting: DocCountDir, Done: true, Start: 1, OwnTools: true}
	if feedback != want {
		t.Fatalf("Sorte feedback: %+v", feedback)
	}
	for _, name := range DocKindNames(DocKinds) {
		if name == FeedbackKindName {
			t.Fatal("feedback steht in der kind-Liste der doc-Werkzeuge")
		}
	}
	store := newFakeDocStore("kamran", "feedback/001-a.md")
	docs := newTestUserDocs(t, store, DocKinds)
	_, err := docs.Create(context.Background(), "feedback", "x")
	wantCode(t, err, DocCodeInvalidInput)
	if !strings.Contains(err.Error(), "k-playbook feedback") {
		t.Errorf("Meldung nennt den Weg nicht: %v", err)
	}
	_, err = docs.Done(context.Background(), "keph:/feedback/001-a.md")
	wantCode(t, err, DocCodeInvalidInput)
}

// putFeedback legt eine Meldung mit Kopf direkt im Store an.
func putFeedback(store *kephtest.Store, name, art, betrifft, status string) {
	number := name[strings.LastIndex(name, "/")+1:][:3]
	store.Put(name, composeFeedbackDoc(number, map[string]string{"title": "Meldung " + number, "art": art,
		"dringlichkeit": "mittel", "betrifft": betrifft, "status": status, "von": "mensch"}, nil, "Text "+number))
}

// Bei von: ki sind client, modell und sitzung Pflicht, bei von: mensch nicht.
func TestFeedbackCreatePflichtfelderJeHerkunft(t *testing.T) {
	useFeedbackStore(t)
	ctx := context.Background()
	opts := validFeedbackOptions()
	opts.Modell, opts.Client = "", ""
	_, err := CreateFeedback(ctx, "", opts)
	wantCode(t, err, DocCodeInvalidInput)
	if !strings.Contains(err.Error(), "client, modell") {
		t.Errorf("Meldung nennt die fehlenden Felder nicht: %v", err)
	}
	opts = validFeedbackOptions()
	opts.Von, opts.Client, opts.Modell, opts.Sitzung = "mensch", "", "", ""
	created, err := CreateFeedback(ctx, "", opts)
	if err != nil || created.Feedback.Von != "mensch" || created.Feedback.Client != "" {
		t.Fatalf("mensch ohne Sitzung: %+v %v", created, err)
	}
}

// Die Liste nimmt ohne includeDone nur die offenen; die Filter greifen, ein
// Filter auf erledigt sucht auch in done/.
func TestFeedbackListFiltert(t *testing.T) {
	store := useFeedbackStore(t)
	putFeedback(store, "feedback/001-a.md", "fehler", "kephalaion", "offen")
	putFeedback(store, "feedback/002-b.md", "wunsch", "k-playbook", "angenommen")
	putFeedback(store, "feedback/done/003-c.md", "fehler", "projekt", "erledigt")
	store.Put("feedback/notiz.md", "ohne Nummer\n")
	ctx := context.Background()

	listing, err := ListFeedback(ctx, "", FeedbackListOptions{})
	list := listing.Items
	if err != nil || len(list) != 2 || list[0].Number != 1 || list[1].Status != "angenommen" || list[0].Body != "Text 001" {
		t.Fatalf("offene: %+v %v", list, err)
	}
	if list[0].Ref != "keph:/feedback/001-a.md" || list[0].Revision == 0 || list[0].Done {
		t.Errorf("Eintrag: %+v", list[0])
	}
	listing, _ = ListFeedback(ctx, "", FeedbackListOptions{IncludeDone: true, Art: "fehler"})
	list = listing.Items
	if len(list) != 2 || list[1].Ref != "keph:/feedback/done/003-c.md" || !list[1].Done {
		t.Errorf("fehler samt done: %+v", list)
	}
	listing, _ = ListFeedback(ctx, "", FeedbackListOptions{Status: "erledigt"})
	list = listing.Items
	if len(list) != 1 || list[0].Number != 3 {
		t.Errorf("status erledigt: %+v", list)
	}
	listing, _ = ListFeedback(ctx, "", FeedbackListOptions{Betrifft: "k-playbook"})
	list = listing.Items
	if len(list) != 1 || list[0].Number != 2 {
		t.Errorf("betrifft: %+v", list)
	}
	_, err = ListFeedback(ctx, "", FeedbackListOptions{Status: "zu"})
	wantCode(t, err, DocCodeInvalidInput)
}

// erledigt und abgelehnt verschieben nach done/, offen holt zurück,
// angenommen bleibt liegen; der Rest des Dokuments bleibt unverändert.
func TestFeedbackUpdateVerschiebt(t *testing.T) {
	store := useFeedbackStore(t)
	created, err := CreateFeedback(context.Background(), "", validFeedbackOptions())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	before, _ := store.Content("feedback/" + created.Name)

	updated, err := UpdateFeedback(ctx, "", "5", "angenommen", created.Revision)
	if err == nil || DocRefErrorCode(err) != DocCodeNotFound {
		t.Fatalf("falsche Nummer: %+v %v", updated, err)
	}
	updated, err = UpdateFeedback(ctx, "", "1", "angenommen", created.Revision)
	if err != nil || !updated.Changed || updated.Feedback.Done || updated.Feedback.Ref != created.Ref {
		t.Fatalf("angenommen: %+v %v", updated, err)
	}
	after, _ := store.Content("feedback/" + created.Name)
	if after != strings.Replace(before, "status: offen\n", "status: angenommen\n", 1) {
		t.Errorf("mehr als der Status geändert:\n%s", after)
	}

	done, err := UpdateFeedback(ctx, "", created.Ref, "erledigt", updated.Feedback.Revision)
	if err != nil || done.Feedback.Ref != "keph:/feedback/done/"+created.Name || !done.Feedback.Done || done.From != created.Ref {
		t.Fatalf("erledigt: %+v %v", done, err)
	}
	if _, ok := store.Content("feedback/" + created.Name); ok {
		t.Error("liegt noch unter feedback/")
	}
	again, err := UpdateFeedback(ctx, "", "#1", "erledigt", 0)
	if err != nil || again.Changed {
		t.Errorf("gleicher Status schreibt: %+v %v", again, err)
	}
	reopened, err := UpdateFeedback(ctx, "", done.Feedback.Ref, "offen", done.Feedback.Revision)
	if err != nil || reopened.Feedback.Ref != created.Ref || reopened.Feedback.Done {
		t.Fatalf("offen: %+v %v", reopened, err)
	}
	if content, _ := store.Content("feedback/" + created.Name); content != before {
		t.Errorf("nach dem Rückweg nicht wie vorher:\n%s", content)
	}
	rejected, err := UpdateFeedback(ctx, "", "1", "abgelehnt", 0)
	if err != nil || !rejected.Feedback.Done {
		t.Errorf("abgelehnt: %+v %v", rejected, err)
	}
}

// Eine abweichende since_revision ist foreign_change, eine ungültige
// Eingabe invalid_input; geschrieben wird nichts.
func TestFeedbackUpdateFremdeAenderung(t *testing.T) {
	store := useFeedbackStore(t)
	putFeedback(store, "feedback/001-a.md", "fehler", "kephalaion", "offen")
	origWait := docEditWait
	docEditWait = func(int) time.Duration { return 0 }
	t.Cleanup(func() { docEditWait = origWait })
	ctx := context.Background()

	_, err := UpdateFeedback(ctx, "", "1", "erledigt", 999)
	wantCode(t, err, DocCodeForeignChange)
	change, ok := DocForeignChangeOf(err)
	if !ok || change.SinceRevision != 999 || change.Ref != "keph:/feedback/001-a.md" {
		t.Errorf("Angaben: %+v", change)
	}
	_, err = UpdateFeedback(ctx, "", "1", "fertig", 0)
	wantCode(t, err, DocCodeInvalidInput)
	_, err = UpdateFeedback(ctx, "", "keph:/todos/001-a.md", "offen", 0)
	wantCode(t, err, DocCodeWrongDir)
	for _, call := range store.Calls {
		if strings.HasPrefix(call, "write ") || strings.HasPrefix(call, "rename ") {
			t.Errorf("geschrieben: %s", call)
		}
	}
}
