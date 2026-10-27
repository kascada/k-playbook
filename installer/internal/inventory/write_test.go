package inventory

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
	"github.com/kascada/k-playbook/installer/internal/kephalaion/kephtest"
)

// newRunProject baut ein kleines Projekt mit einer Quelle und liefert die
// Optionen eines Laufs. Das Inventar landet nicht auf der Platte, sondern im
// Store, den der Test dazu anlegt (newRunStore).
func newRunProject(t *testing.T) Options {
	t.Helper()

	root := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	if err := os.WriteFile(filepath.Join(root, "requirements.txt"), []byte("redis==5.0.1\n"), 0o644); err != nil {
		t.Fatalf("schreiben: %v", err)
	}
	return Options{
		ProjectDir:  root,
		SourcesFile: filepath.Join(root, "k-playbook-local", "version-sources.yaml"),
	}
}

// newRunStore ist die Collection im Speicher, in die ein Lauf schreibt.
func newRunStore(t *testing.T) *kephtest.Store {
	t.Helper()
	return kephtest.New()
}

// runInto erhebt und schreibt in den Store — der Weg jedes Einstiegs.
func runInto(t *testing.T, store Store, options Options) (Result, Outcome, error) {
	t.Helper()
	return Run(context.Background(), store, options)
}

// storedInventory ist der Bestand im Store; fehlt er, scheitert der Test.
func storedInventory(t *testing.T, store *kephtest.Store) string {
	t.Helper()
	content, ok := store.Content(DocName)
	if !ok {
		t.Fatalf("%s liegt nicht im Store: %v", DocName, store.Names())
	}
	return content
}

// Der erste Lauf legt das Dokument unter dem einen Namen an — über create,
// nicht write —, und der Outcome nennt den Ort als keph:-Verweis.
func TestErsterLaufLegtDasDokumentInDerCollectionAn(t *testing.T) {
	options := newRunProject(t)
	store := newRunStore(t)
	options.Now = func() time.Time { return time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC) }

	_, outcome, err := runInto(t, store, options)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !outcome.Written || outcome.Path != DocRef || outcome.At != "2026-09-05T12:00:00Z" || outcome.Revision == 0 {
		t.Errorf("Outcome = %+v", outcome)
	}
	if outcome.Path != "keph:/docs/generated/versions/inventory.md" {
		t.Errorf("der Verweis lautet %q", outcome.Path)
	}
	if store.Names()[0] != DocName || len(store.Names()) != 1 {
		t.Errorf("im Store liegt %v, erwartet nur %s", store.Names(), DocName)
	}
	if len(store.Calls) != 2 || store.Calls[0] != "read "+DocName || store.Calls[1] != "create "+DocName {
		t.Errorf("Aufrufe = %v, erwartet read und create", store.Calls)
	}
	if !strings.Contains(storedInventory(t, store), "redis") {
		t.Error("das Dokument trägt die Erhebung nicht")
	}
}

// Die Byte-Stabilitätsregel: ein zweiter Lauf ohne Quelländerung fasst das
// Dokument nicht an — Zeitstempel und Revision eingeschlossen.
func TestZweiterLaufOhneQuellaenderungFasstDasDokumentNichtAn(t *testing.T) {
	options := newRunProject(t)
	store := newRunStore(t)
	options.Now = func() time.Time { return time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC) }

	_, first, err := runInto(t, store, options)
	if err != nil {
		t.Fatalf("erster Lauf: %v", err)
	}
	if !first.Written {
		t.Fatal("der erste Lauf muss schreiben")
	}
	before := storedInventory(t, store)
	calls := len(store.Calls)

	// Der zweite Lauf hat eine andere Uhr: stünde der Zeitstempel des Laufs im
	// Dokument, änderte es sich jedes Mal.
	options.Now = func() time.Time { return time.Date(2026, 12, 24, 18, 0, 0, 0, time.UTC) }
	_, second, err := runInto(t, store, options)
	if err != nil {
		t.Fatalf("zweiter Lauf: %v", err)
	}
	if second.Written {
		t.Error("ohne inhaltliche Änderung darf gar nicht geschrieben werden")
	}
	if after := storedInventory(t, store); after != before {
		t.Error("das Dokument muss byte-identisch bleiben")
	}
	if second.At != first.At {
		t.Errorf("der Zeitstempel bleibt der der letzten inhaltlichen Änderung: %q gegen %q", second.At, first.At)
	}
	if second.Revision != first.Revision {
		t.Errorf("die Revision bleibt: %d gegen %d", second.Revision, first.Revision)
	}
	if got := store.Calls[calls:]; len(got) != 1 || got[0] != "read "+DocName {
		t.Errorf("der zweite Lauf darf nur lesen: %v", got)
	}
}

// Eine geänderte Quelle führt zu einer aktualisierten Inventur, einem
// fortgeschriebenen Zeitstempel, einem write mit der gelesenen Revision als
// base_revision und — weil eine zweite Quelle desselben Projekts weiter das
// Alte sagt — zu einer sichtbaren Abweichung.
func TestGeaenderteQuelleSchreibtNeuMitNeuemZeitstempelUndZeigtDieAbweichung(t *testing.T) {
	options := newRunProject(t)
	store := newRunStore(t)
	// Dieselbe Aussage aus einer zweiten Quelle: solange beide dasselbe sagen,
	// ist das keine Abweichung.
	if err := os.WriteFile(filepath.Join(options.ProjectDir, "pyproject.toml"),
		[]byte("[project]\nname = \"beispiel\"\ndependencies = [\n  \"redis==5.0.1\",\n]\n"), 0o644); err != nil {
		t.Fatalf("schreiben: %v", err)
	}
	options.Now = func() time.Time { return time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC) }
	first, firstOutcome, err := runInto(t, store, options)
	if err != nil {
		t.Fatalf("erster Lauf: %v", err)
	}
	if len(first.Deviations) != 0 {
		t.Fatalf("gleiche version und gleicher pin ergeben keine Abweichung: %+v", first.Deviations)
	}

	if err := os.WriteFile(filepath.Join(options.ProjectDir, "requirements.txt"),
		[]byte("redis==5.2.0\n"), 0o644); err != nil {
		t.Fatalf("schreiben: %v", err)
	}
	options.Now = func() time.Time { return time.Date(2026, 12, 24, 18, 0, 0, 0, time.UTC) }
	second, outcome, err := runInto(t, store, options)
	if err != nil {
		t.Fatalf("zweiter Lauf: %v", err)
	}
	if !outcome.Written {
		t.Fatal("eine geänderte Quelle muss das Dokument neu schreiben")
	}
	if !strings.HasPrefix(outcome.At, "2026-12-24") || outcome.At == firstOutcome.At {
		t.Errorf("der Zeitstempel muss fortgeschrieben werden: %q gegen %q", outcome.At, firstOutcome.At)
	}
	if outcome.Revision <= firstOutcome.Revision {
		t.Errorf("die Revision muss steigen: %d nach %d", firstOutcome.Revision, outcome.Revision)
	}
	want := "write " + DocName + " @" + itoa(firstOutcome.Revision)
	if last := store.Calls[len(store.Calls)-1]; last != want {
		t.Errorf("letzter Aufruf %q, erwartet %q — write mit der gelesenen Revision", last, want)
	}
	if len(second.Deviations) != 1 || second.Deviations[0].Group != "python/redis" {
		t.Fatalf("die Änderung muss eine Abweichung erzeugen: %+v", second.Deviations)
	}
	if second.Deviations[0].Art != DeviationConflicting {
		t.Errorf("beide Quellen sind lokal, das ist widersprüchlich: %q", second.Deviations[0].Art)
	}
	content := storedInventory(t, store)
	for _, needle := range []string{"5.2.0", "5.0.1", "deviations: 1",
		"### widersprüchlich — `python/redis`"} {
		if !strings.Contains(content, needle) {
			t.Errorf("im Dokument fehlt %q", needle)
		}
	}
}

// Auch die Ablehnungen sind Inhalt: ändert sich die Menge der abgelehnten
// Quellen, ändert sich das Dokument.
func TestAblehnungenSindInhalt(t *testing.T) {
	options := newRunProject(t)
	store := newRunStore(t)
	options.Now = func() time.Time { return time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC) }
	if _, _, err := runInto(t, store, options); err != nil {
		t.Fatalf("erster Lauf: %v", err)
	}

	writeSources(t, options.SourcesFile, `schema_version: 1
sources:
  - path: ../draussen/values.yaml
    kind: helm
    env: deployment
`)
	options.Now = func() time.Time { return time.Date(2026, 12, 24, 18, 0, 0, 0, time.UTC) }
	result, outcome, err := runInto(t, store, options)
	if err != nil {
		t.Fatalf("zweiter Lauf: %v", err)
	}
	if !outcome.Written {
		t.Fatal("eine neue Ablehnung ändert das Dokument")
	}
	if len(result.Rejections) != 1 {
		t.Fatalf("Ablehnungen = %+v", result.Rejections)
	}
	content := storedInventory(t, store)
	if !strings.Contains(content, "außerhalb der erlaubten Wurzeln") {
		t.Error("die Ablehnung muss im Dokument stehen, nicht nur im Ergebnis")
	}
	if !strings.Contains(content, "rejected: 1") {
		t.Error("das Frontmatter muss die Ablehnung zählen")
	}
}

// Liegt das Dokument da, sein Frontmatter aber defekt, ist das ein sichtbarer
// Befund; der Lauf erhebt neu und schreibt, weil ein Vergleich nicht möglich
// ist — mit der Revision des defekten Bestands als base_revision.
func TestDefektesFrontmatterWirdGemeldetUndNeuGeschrieben(t *testing.T) {
	options := newRunProject(t)
	store := newRunStore(t)
	store.Put(DocName, "# Versionsinventar\n\nvon Hand\n")

	_, outcome, err := runInto(t, store, options)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if outcome.Problem == "" {
		t.Error("ein defektes Frontmatter ist ein sichtbarer Befund")
	}
	if !outcome.Written {
		t.Error("ohne Vergleichsmöglichkeit wird neu geschrieben")
	}
	if last := store.Calls[len(store.Calls)-1]; !strings.HasPrefix(last, "write "+DocName+" @1") {
		t.Errorf("neu geschrieben wird mit write auf der gelesenen Revision, nicht mit create: %q", last)
	}
	if status := ReadStatus(context.Background(), store); status.Problem != "" {
		t.Errorf("nach dem Lauf muss der Stand sauber sein: %q", status.Problem)
	}
}

// Hat inzwischen jemand anders geschrieben, lehnt der Hub den write mit
// stale_revision ab. Das ist ein sichtbarer Fehler; es wird nichts
// überschrieben und nichts als geschrieben gemeldet.
func TestStaleRevisionIstEinSichtbarerFehler(t *testing.T) {
	options := newRunProject(t)
	store := newRunStore(t)
	store.Put(DocName, Render(Result{}, "2026-09-01T08:00:00Z"))
	// Die Erhebung unterscheidet sich vom Bestand, also will der Lauf
	// schreiben; der Hub lehnt ab.
	stale := &kephalaion.ToolError{Tool: "write", Code: kephalaion.CodeStaleRevision, Message: "aktuelle Revision 7"}
	store.Fail["write"] = stale

	_, outcome, err := runInto(t, store, options)
	if err == nil {
		t.Fatal("stale_revision muss als Fehler zurückkommen")
	}
	if kephalaion.ErrorCode(err) != kephalaion.CodeStaleRevision {
		t.Errorf("der Code von Kephalaion geht verloren: %v", err)
	}
	if !errors.Is(err, stale) {
		t.Errorf("der Fehler des Stores muss erhalten bleiben: %v", err)
	}
	if outcome.Written {
		t.Error("nach einer Ablehnung darf nichts als geschrieben gelten")
	}
	if content, _ := store.Content(DocName); !strings.Contains(content, "2026-09-01T08:00:00Z") {
		t.Error("der Bestand im Store wurde trotz Ablehnung verändert")
	}
}

// Ohne erreichbare Collection gibt es kein Inventar: Run bricht mit dem
// Fehler des Stores ab, legt nichts auf der Platte ab und meldet nichts als
// geschrieben.
func TestOhneErreichbareCollectionGibtEsKeinInventar(t *testing.T) {
	options := newRunProject(t)
	store := newRunStore(t)
	down := &kephalaion.ToolError{Tool: "read", Code: kephalaion.CodeNodeUnreachable, Message: "Node antwortet nicht"}
	store.Fail["read"] = down

	_, outcome, err := runInto(t, store, options)
	if !errors.Is(err, down) {
		t.Fatalf("erwartet den Fehler des Stores, bekommen: %v", err)
	}
	if outcome.Written || outcome.Path != DocRef {
		t.Errorf("Outcome = %+v", outcome)
	}
	if len(store.Names()) != 0 {
		t.Errorf("trotz Abbruch liegt etwas im Store: %v", store.Names())
	}
	if _, err := os.Stat(filepath.Join(options.ProjectDir, "k-playbook-local", "docs")); !os.IsNotExist(err) {
		t.Error("einen Rückfall auf die Platte gibt es nicht")
	}
	if _, _, err := Run(context.Background(), nil, options); err == nil {
		t.Error("ohne Store muss Run abbrechen")
	}
}

// Der Stand aus der Collection: fehlt das Dokument, ist das ein definierter
// Zustand; ist die Collection nicht erreichbar, ein anderer — und keiner von
// beiden wird mit dem anderen verwechselt.
func TestReadStatusUnterscheidetFehltUndNichtErreichbar(t *testing.T) {
	store := newRunStore(t)
	status := ReadStatus(context.Background(), store)
	if status.Present || status.Unavailable || status.Problem != "" {
		t.Errorf("ein fehlendes Dokument ist ein definierter Zustand, kein Fehler: %+v", status)
	}
	if status.Path != DocRef {
		t.Errorf("Path = %q", status.Path)
	}

	store.Fail["read"] = &kephalaion.ToolError{Tool: "read", Code: kephalaion.CodeNodeUnreachable, Message: "Node antwortet nicht"}
	status = ReadStatus(context.Background(), store)
	if !status.Unavailable || status.Present || !strings.Contains(status.Message, "Node antwortet nicht") {
		t.Errorf("nicht erreichbar ist nicht „fehlt“: %+v", status)
	}
	if status = ReadStatus(context.Background(), nil); !status.Unavailable {
		t.Errorf("ohne Store: %+v", status)
	}
	if status = UnavailableStatus("nicht bereit"); !status.Unavailable || status.Message != "nicht bereit" || status.Path != DocRef {
		t.Errorf("UnavailableStatus = %+v", status)
	}
}

func TestReadStatusMeldetUnvollstaendigesFrontmatter(t *testing.T) {
	store := newRunStore(t)
	store.Put(DocName, "---\ngenerated: { by: k-doc-inventory, at: x }\n---\n\n# Rumpf\n")
	status := ReadStatus(context.Background(), store)
	if !status.Present || status.Revision == 0 {
		t.Errorf("Status = %+v", status)
	}
	if !strings.Contains(status.Problem, "title") || !strings.Contains(status.Problem, "description") {
		t.Errorf("`generated.by` allein genügt nicht: %q", status.Problem)
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
