package project

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
	"github.com/kascada/k-playbook/installer/internal/kephalaion/kephtest"
)

// Die Erzeugertabelle ist gültig und trägt in diesem Stand code und libs
// (Task 120); ein unbekannter Erzeuger ist invalid_input und nennt die
// erlaubten. README-Texte trägt sie seit Task 137 nicht mehr.
func TestDocsProducersTabelle(t *testing.T) {
	if err := ValidateDocsProducers(DocsProducers); err != nil {
		t.Fatalf("Tabelle ungültig: %v", err)
	}
	if got := DocsProducerNames(); !reflect.DeepEqual(got, []string{"code", "libs"}) {
		t.Errorf("Erzeuger: %v", got)
	}
	code, err := DocsProducerByName("code")
	if err != nil {
		t.Fatal(err)
	}
	if code.Ref() != "keph:/docs/generated/code/" || code.docName("a/01-x.md") != "docs/generated/code/a/01-x.md" {
		t.Errorf("Orte: %s, %s", code.Ref(), code.docName("a/01-x.md"))
	}
	libs, err := DocsProducerByName("libs")
	if err != nil {
		t.Fatal(err)
	}
	if libs.Ref() != "keph:/docs/generated/libs/" || libs.docName("python-celery.md") != "docs/generated/libs/python-celery.md" {
		t.Errorf("Orte: %s, %s", libs.Ref(), libs.docName("python-celery.md"))
	}
	_, err = DocsProducerByName("tools")
	if DocRefErrorCode(err) != DocCodeInvalidInput || !strings.Contains(err.Error(), "code, libs") {
		t.Errorf("unbekannter Erzeuger: %v (%s)", err, DocRefErrorCode(err))
	}

	for _, bad := range [][]DocsProducer{
		{{Name: "Code"}},
		{{Name: "a.b"}},
		{{Name: "code"}, {Name: "code"}},
	} {
		if err := ValidateDocsProducers(bad); err == nil {
			t.Errorf("Tabelle %+v gilt als gültig", bad)
		}
	}
}

// Jeder Erzeuger hat seine README-Vorlage unter defaults/docs/generated/ —
// publish schreibt sie von dort. Die Vorlage von libs/ nennt, was die alte
// Tabelle nannte.
func TestDocsProducerVorlagen(t *testing.T) {
	layers := repoDefaultsLayers(t)
	for _, producer := range DocsProducers {
		if producer.templateDir() != "generated/"+producer.Name {
			t.Errorf("%s: Vorlagenverzeichnis %q", producer.Name, producer.templateDir())
		}
		mustResolveDoc(t, layers, DocsTemplateRef(producer.templateDir()))
	}
	libs := readRepoDefault(t, "docs/generated/libs/README.md")
	for _, needle := range []string{"/k-docs-tools", "keph:/docs/generated/versions/inventory.md", "`sources`", "`hints`",
		"`k-playbook docs checkout libs`", "`k-playbook docs publish libs`"} {
		if !strings.Contains(libs, needle) {
			t.Errorf("Vorlage libs nennt %q nicht", needle)
		}
	}
}

// Pfadregeln im Arbeitsverzeichnis: nur .md, keine README.md, jedes Segment
// nach der Namensregel von Kephalaion.
func TestDocsRelPath(t *testing.T) {
	for _, ok := range []string{"01-overview.md", "sub/02-teil.md", "a_b.c-d.md"} {
		if problem := docsRelPath(ok); problem != "" {
			t.Errorf("%s abgelehnt: %s", ok, problem)
		}
	}
	for rel, want := range map[string]string{
		"README.md":        "README.md",
		"sub/readme.md":    "README.md",
		"notiz.txt":        ".md",
		"Gross.md":         "Namensregel",
		"sub dir/x.md":     "Namensregel",
		".versteckt.md":    "Namensregel",
		"../x.md":          "unterhalb",
		"/x.md":            "unterhalb",
		"a//b.md":          "unterhalb",
		"":                 "unterhalb",
		"_privat/x.md":     "Namensregel",
		"x.md/../y.md":     "unterhalb",
		"tief/tiefer/z.md": "",
	} {
		problem := docsRelPath(rel)
		if want == "" {
			if problem != "" {
				t.Errorf("%q abgelehnt: %s", rel, problem)
			}
			continue
		}
		if !strings.Contains(problem, want) {
			t.Errorf("%q: Ablehnung %q, erwartet mit %q", rel, problem, want)
		}
	}
}

// Frontmatter-Prüfung: title und description Pflicht, hints fehlt, leer oder
// Liste von Sätzen.
func TestParseDocsMeta(t *testing.T) {
	meta, problems := parseDocsMeta([]byte("---\ntitle: T\ndescription: D, mit: Doppelpunkt\n" +
		"generated: { by: k-docs-code, at: 2026-11-01T10:00:00Z }\n" +
		"hints: [ \"Eins, mit Komma\", 'zwei: drei' ]\nquestions:\n  - Wie geht das?\n---\n\n# T\n"))
	if len(problems) != 0 || meta.Title != "T" || meta.Description != "D, mit: Doppelpunkt" ||
		!reflect.DeepEqual(meta.Hints, []string{"Eins, mit Komma", "zwei: drei"}) {
		t.Errorf("gültiges Frontmatter: %+v, %v", meta, problems)
	}
	meta, problems = parseDocsMeta([]byte("---\ntitle: T\ndescription: D\nhints:\n  - a\n  - b\n---\n"))
	if len(problems) != 0 || !reflect.DeepEqual(meta.Hints, []string{"a", "b"}) {
		t.Errorf("Blockliste: %+v, %v", meta, problems)
	}
	for _, empty := range []string{"hints: []\n", "hints:\n", ""} {
		meta, problems = parseDocsMeta([]byte("---\ntitle: T\ndescription: D\n" + empty + "---\n"))
		if len(problems) != 0 || meta.Hints != nil {
			t.Errorf("%q: %+v, %v", empty, meta, problems)
		}
	}
	for content, want := range map[string]string{
		"# Ohne\n":                   "Frontmatter fehlt",
		"---\ndescription: D\n---\n": "title fehlt",
		"---\ntitle: T\n---\n":       "description fehlt",
		"---\ntitle: T\ndescription: D\nhints: x\n---\n":                   "keine Liste",
		"---\ntitle: T\ndescription: D\nhints:\n  - a: b\n    c: d\n---\n": "anderes als Sätze",
	} {
		_, problems := parseDocsMeta([]byte(content))
		if len(problems) == 0 || !strings.Contains(strings.Join(problems, "; "), want) {
			t.Errorf("%q: Probleme %v, erwartet %q", content, problems, want)
		}
	}

	if got := docsHintsOfContent([]byte("kein Frontmatter")); got != nil {
		t.Errorf("hints ohne Frontmatter: %v", got)
	}
	if !sameHints(nil, []string{}) || sameHints([]string{"a"}, []string{"b"}) || !sameHints([]string{"a"}, []string{"a"}) {
		t.Error("sameHints vergleicht falsch")
	}
}

// Die Merkdatei kommt so zurück, wie sie geschrieben wurde; eine kaputte ist
// invalid_input, eine fehlende kein Fehler.
func TestDocsRecordRundweg(t *testing.T) {
	file := DocsRecordFile(t.TempDir(), "code")
	if !strings.HasSuffix(filepath.ToSlash(file), ".k-playbook/work/docs/code.checkout.json") {
		t.Errorf("Merkdatei: %s", file)
	}
	if _, ok, err := readDocsRecord(file); ok || err != nil {
		t.Fatalf("fehlende Merkdatei: %v, %v", ok, err)
	}
	record := docsRecord{Producer: "code", Collection: "vm:p", CheckedOut: "2026-11-01T10:00:00Z"}
	record.set(docsRecordItem{Path: "b.md", Revision: 2, Hash: docsHash([]byte("b"))})
	record.set(docsRecordItem{Path: "a.md", Revision: 1, Hash: docsHash([]byte("a")), Hints: []string{"h"}})
	record.set(docsRecordItem{Path: "b.md", Revision: 3, Hash: "x"})
	if err := writeDocsRecord(file, record); err != nil {
		t.Fatal(err)
	}
	got, ok, err := readDocsRecord(file)
	if err != nil || !ok || !reflect.DeepEqual(got, record) {
		t.Fatalf("Rundweg: %+v, %v, %v", got, ok, err)
	}
	if len(got.Docs) != 2 || got.Docs[0].Path != "a.md" || got.Docs[1].Revision != 3 {
		t.Errorf("Einträge: %+v", got.Docs)
	}
	got.remove("a.md")
	if _, found := got.find("a.md"); found || len(got.Docs) != 1 {
		t.Errorf("remove: %+v", got.Docs)
	}

	if err := os.WriteFile(file, []byte("{kaputt"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readDocsRecord(file); DocRefErrorCode(err) != DocCodeInvalidInput {
		t.Errorf("kaputte Merkdatei: %v", err)
	}
}

// Ohne bereite Anbindung gibt es keinen Zugang: kephalaion_unavailable.
func TestOpenDocsWorkOhneAnbindungIstKephalaionUnavailable(t *testing.T) {
	original := kephalaionCheck
	kephalaionCheck = func(string) kephalaion.Status {
		return kephalaion.Status{State: kephalaion.StateNotConfigured, Message: "Keine Collection eingetragen."}
	}
	t.Cleanup(func() { kephalaionCheck = original })

	work, err := OpenDocsWork(t.TempDir())
	if work != nil || DocRefErrorCode(err) != DocCodeKephalaionUnavailable {
		t.Fatalf("Zugang %v, Fehler %v", work, err)
	}
}

// .k-playbook/.gitignore bekommt eine Zeile genau einmal, Vorhandenes bleibt.
func TestEnsureProjectIgnored(t *testing.T) {
	root := t.TempDir()
	if err := ensureProjectIgnored(root, "/work", "# Kopf\n"); err != nil {
		t.Fatal(err)
	}
	if err := ensureShareIgnored(root); err != nil {
		t.Fatal(err)
	}
	if err := ensureProjectIgnored(root, "/work", "# Kopf\n"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, ProjectDirName, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "# Kopf\n/work\n/share\n" {
		t.Errorf(".gitignore: %q", data)
	}
}

// docsWorkFixture ist ein Projektverzeichnis mit einem Store im Speicher und
// fester Uhr.
func docsWorkFixture(t *testing.T) (string, *kephtest.Store, *DocsWork) {
	t.Helper()
	root := t.TempDir()
	store := kephtest.New()
	work := NewDocsWork(root, store, "vm:projekt").WithTemplates(repoDefaultsSource(t))
	work.now = func() time.Time { return time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC) }
	return root, store, work
}

func readWorkFile(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(DocsWorkDir(root, "code"), filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("%s lesen: %v", rel, err)
	}
	return string(data)
}

// Ein leeres Erzeuger-Verzeichnis: Arbeitsverzeichnis und Merkdatei
// entstehen trotzdem, /work steht in .k-playbook/.gitignore.
func TestDocsCheckoutLeer(t *testing.T) {
	root, _, work := docsWorkFixture(t)

	result, err := work.Checkout(context.Background(), "code", DocsCheckoutOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.State != DocsStateCheckedOut || len(result.Docs) != 0 || result.Ref != "keph:/docs/generated/code/" ||
		result.Dir != DocsWorkDir(root, "code") || result.Record != DocsRecordFile(root, "code") {
		t.Errorf("Antwort: %+v", result)
	}
	if info, err := os.Stat(result.Dir); err != nil || !info.IsDir() {
		t.Errorf("Arbeitsverzeichnis fehlt: %v", err)
	}
	record, ok, err := readDocsRecord(result.Record)
	if err != nil || !ok || record.Producer != "code" || record.Collection != "vm:projekt" ||
		record.CheckedOut != "2026-11-01T12:00:00Z" || len(record.Docs) != 0 {
		t.Errorf("Merkdatei: %+v, %v, %v", record, ok, err)
	}
	ignore, err := os.ReadFile(filepath.Join(root, ProjectDirName, ".gitignore"))
	if err != nil || !strings.Contains(string(ignore), "\n/work\n") {
		t.Errorf(".gitignore: %q, %v", ignore, err)
	}
}

// Vorhandener Bestand mit Unterverzeichnis: geholt werden alle .md außer
// README.md, mit gleichem relativem Pfad; die Merkdatei trägt Revision, Hash
// und hints. Andere Verzeichnisse unter docs/ bleiben draußen.
func TestDocsCheckoutBestand(t *testing.T) {
	root, store, work := docsWorkFixture(t)
	overview := "---\ntitle: Überblick\ndescription: D\nhints: [ \"Auf .devcontainer/ achten\" ]\n---\n\n# Überblick\n"
	store.Put("docs/generated/code/00-overview.md", overview)
	store.Put("docs/generated/code/sub/10-teil.md", "---\ntitle: Teil\ndescription: D\n---\n")
	store.Put("docs/generated/code/README.md", "---\ntitle: R\n---\n")
	store.Put("docs/generated/code/bild.png", "PNG")
	store.Put("docs/generated/codex/fremd.md", "fremd")
	store.Put("docs/generated/versions/inventory.md", "inventar")
	store.Put("docs/manual/hand.md", "hand")

	result, err := work.Checkout(context.Background(), "code", DocsCheckoutOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, doc := range result.Docs {
		paths = append(paths, doc.Path)
		if doc.Ref != "keph:/docs/generated/code/"+doc.Path || doc.Revision == 0 {
			t.Errorf("Eintrag: %+v", doc)
		}
	}
	if !reflect.DeepEqual(paths, []string{"00-overview.md", "sub/10-teil.md"}) {
		t.Fatalf("geholt: %v", paths)
	}
	if got := readWorkFile(t, root, "00-overview.md"); got != overview {
		t.Errorf("Inhalt: %q", got)
	}
	readWorkFile(t, root, "sub/10-teil.md")
	for _, absent := range []string{"README.md", "bild.png"} {
		if _, err := os.Stat(filepath.Join(result.Dir, absent)); !os.IsNotExist(err) {
			t.Errorf("%s im Arbeitsverzeichnis", absent)
		}
	}
	record, _, _ := readDocsRecord(result.Record)
	item, ok := record.find("00-overview.md")
	if !ok || item.Revision != result.Docs[0].Revision || item.Hash != docsHash([]byte(overview)) ||
		!reflect.DeepEqual(item.Hints, []string{"Auf .devcontainer/ achten"}) {
		t.Errorf("Merkdatei-Eintrag: %+v", item)
	}
	if item, _ := record.find("sub/10-teil.md"); item.Hints != nil {
		t.Errorf("hints ohne Feld: %+v", item)
	}
}

// Liegt ein Arbeitsverzeichnis schon da, wird nichts geholt und nichts
// überschrieben; die Antwort nennt Alter und Anzahl Dateien. --discard
// verwirft es und holt neu.
func TestDocsCheckoutVorhandenUndDiscard(t *testing.T) {
	root, store, work := docsWorkFixture(t)
	store.Put("docs/generated/code/01-a.md", "---\ntitle: A\ndescription: D\n---\n")
	if _, err := work.Checkout(context.Background(), "code", DocsCheckoutOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(DocsWorkDir(root, "code"), "01-a.md"), []byte("lokal"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(DocsWorkDir(root, "code"), "02-neu.md"), []byte("neu"), 0o644); err != nil {
		t.Fatal(err)
	}
	store.Put("docs/generated/code/01-a.md", "---\ntitle: A2\ndescription: D\n---\n")
	work.now = func() time.Time { return time.Date(2026, 11, 1, 13, 30, 0, 0, time.UTC) }
	calls := len(store.Calls)

	result, err := work.Checkout(context.Background(), "code", DocsCheckoutOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.State != DocsStateExisting || result.Existing == nil || result.Existing.Files != 2 ||
		result.Existing.AgeMinutes != 90 || result.Existing.Since != "2026-11-01T12:00:00Z" || !result.Existing.Record {
		t.Errorf("vorhandenes Arbeitsverzeichnis: %+v %+v", result, result.Existing)
	}
	if len(store.Calls) != calls {
		t.Errorf("Kephalaion gefragt: %v", store.Calls[calls:])
	}
	if got := readWorkFile(t, root, "01-a.md"); got != "lokal" {
		t.Errorf("überschrieben: %q", got)
	}

	// Nur die Merkdatei ohne Arbeitsverzeichnis zählt ebenso.
	if err := os.RemoveAll(DocsWorkDir(root, "code")); err != nil {
		t.Fatal(err)
	}
	result, err = work.Checkout(context.Background(), "code", DocsCheckoutOptions{})
	if err != nil || result.State != DocsStateExisting || result.Existing.Files != 0 {
		t.Errorf("nur Merkdatei: %+v, %v", result, err)
	}

	result, err = work.Checkout(context.Background(), "code", DocsCheckoutOptions{Discard: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.State != DocsStateCheckedOut || !result.Discarded || len(result.Docs) != 1 {
		t.Errorf("--discard: %+v", result)
	}
	if got := readWorkFile(t, root, "01-a.md"); !strings.Contains(got, "title: A2") {
		t.Errorf("nicht neu geholt: %q", got)
	}
	if _, err := os.Stat(filepath.Join(result.Dir, "02-neu.md")); !os.IsNotExist(err) {
		t.Error("02-neu.md nach --discard noch da")
	}
	record, _, _ := readDocsRecord(result.Record)
	if record.CheckedOut != "2026-11-01T13:30:00Z" {
		t.Errorf("Merkdatei nicht neu: %+v", record)
	}
}

// Ohne Kephalaion schreibt checkout nichts — kein Arbeitsverzeichnis, keine
// Merkdatei, kein .gitignore; mit --discard bleibt das Vorhandene liegen.
func TestDocsCheckoutOhneKephalaion(t *testing.T) {
	root, store, work := docsWorkFixture(t)
	store.Fail["list"] = &kephalaion.ToolError{Tool: "list", Code: kephalaion.CodeNodeUnreachable, Message: "Node weg"}

	_, err := work.Checkout(context.Background(), "code", DocsCheckoutOptions{})
	if DocRefErrorCode(err) != DocCodeKephalaionUnavailable {
		t.Fatalf("Fehler: %v (%s)", err, DocRefErrorCode(err))
	}
	if _, err := os.Stat(filepath.Join(root, ProjectDirName)); !os.IsNotExist(err) {
		t.Errorf(".k-playbook angelegt: %v", err)
	}

	if err := os.MkdirAll(DocsWorkDir(root, "code"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(DocsWorkDir(root, "code"), "x.md"), []byte("lokal"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := work.Checkout(context.Background(), "code", DocsCheckoutOptions{Discard: true}); DocRefErrorCode(err) != DocCodeKephalaionUnavailable {
		t.Fatalf("--discard ohne Kephalaion: %v", err)
	}
	if got := readWorkFile(t, root, "x.md"); got != "lokal" {
		t.Errorf("Vorhandenes verworfen: %q", got)
	}

	if _, err := work.Checkout(context.Background(), "tools", DocsCheckoutOptions{}); DocRefErrorCode(err) != DocCodeInvalidInput {
		t.Errorf("unbekannter Erzeuger: %v", err)
	}
}

func writeWorkFile(t *testing.T, root, rel, content string) {
	t.Helper()
	writeWorkFileFor(t, root, "code", rel, content)
}

func writeWorkFileFor(t *testing.T, root, producer, rel, content string) {
	t.Helper()
	file := filepath.Join(DocsWorkDir(root, producer), filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func removeWorkFile(t *testing.T, root, rel string) {
	t.Helper()
	if err := os.Remove(filepath.Join(DocsWorkDir(root, "code"), filepath.FromSlash(rel))); err != nil {
		t.Fatal(err)
	}
}

func docsDoc(title string, extra ...string) string {
	return "---\ntitle: " + title + "\ndescription: Beschreibung von " + title + "\n" + strings.Join(extra, "") + "---\n\n# " + title + "\n"
}

// statuses fasst die Antwort als pfad → Zustand zusammen.
func statuses(result DocsPublishResult) map[string]string {
	out := map[string]string{}
	for _, item := range result.Docs {
		out[item.Path] = item.Status
	}
	return out
}

// writeCalls sind die Aufrufe von create, write und delete ab einer Stelle.
func writeCalls(store *kephtest.Store, from int) []string {
	var calls []string
	for _, call := range store.Calls[from:] {
		if strings.HasPrefix(call, "create ") || strings.HasPrefix(call, "write ") || strings.HasPrefix(call, "delete ") {
			calls = append(calls, call)
		}
	}
	return calls
}

// checkoutFixture holt einen Bestand mit vier Dokumenten.
func checkoutFixture(t *testing.T) (string, *kephtest.Store, *DocsWork, map[string]int64) {
	t.Helper()
	root, store, work := docsWorkFixture(t)
	store.Put("docs/generated/code/01-bleibt.md", docsDoc("Bleibt"))
	store.Put("docs/generated/code/02-aendern.md", docsDoc("Ändern"))
	store.Put("docs/generated/code/03-weg.md", docsDoc("Weg"))
	store.Put("docs/generated/code/sub/04-hints.md", docsDoc("Hints", "hints: [ \"Auf .devcontainer/ achten\" ]\n"))
	result, err := work.Checkout(context.Background(), "code", DocsCheckoutOptions{})
	if err != nil {
		t.Fatal(err)
	}
	revisions := map[string]int64{}
	for _, doc := range result.Docs {
		revisions[doc.Path] = doc.Revision
	}
	return root, store, work, revisions
}

// Neu, geändert, gelöscht und unverändert in einem Lauf; danach die README
// aus der Tabelle, Arbeitsverzeichnis und Merkdatei sind weg.
func TestDocsPublishAbgleich(t *testing.T) {
	root, store, work, revisions := checkoutFixture(t)
	writeWorkFile(t, root, "02-aendern.md", docsDoc("Geändert"))
	removeWorkFile(t, root, "03-weg.md")
	writeWorkFile(t, root, "sub/05-neu.md", docsDoc("Neu", "questions: [ \"Wie fange ich an?\" ]\n"))
	calls := len(store.Calls)

	result, err := work.Publish(context.Background(), "code", DocsPublishOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"01-bleibt.md": DocsUnchanged, "02-aendern.md": DocsWritten, "03-weg.md": DocsDeleted,
		"sub/04-hints.md": DocsUnchanged, "sub/05-neu.md": DocsCreated}
	if got := statuses(result); !reflect.DeepEqual(got, want) {
		t.Errorf("Zustände: %v", got)
	}
	if got := writeCalls(store, calls); !reflect.DeepEqual(got, []string{
		fmt.Sprintf("write docs/generated/code/02-aendern.md @%d", revisions["02-aendern.md"]),
		fmt.Sprintf("delete docs/generated/code/03-weg.md @%d", revisions["03-weg.md"]),
		"create docs/generated/code/sub/05-neu.md",
		"create docs/generated/code/README.md",
	}) {
		t.Errorf("Aufrufe: %v", got)
	}
	for _, item := range result.Docs {
		if item.Status != DocsDeleted && item.Revision == 0 {
			t.Errorf("ohne Revision: %+v", item)
		}
		if item.Ref != "keph:/docs/generated/code/"+item.Path {
			t.Errorf("Verweis: %+v", item)
		}
	}
	if content, _ := store.Content("docs/generated/code/02-aendern.md"); !strings.Contains(content, "title: Geändert") {
		t.Errorf("nicht geschrieben: %q", content)
	}
	if _, ok := store.Content("docs/generated/code/03-weg.md"); ok {
		t.Error("03-weg.md nicht gelöscht")
	}
	codeReadme := readRepoDefault(t, "docs/generated/code/README.md")
	if content, _ := store.Content("docs/generated/code/README.md"); content != codeReadme {
		t.Errorf("README: %q", content)
	}
	if result.Readme.Status != DocsCreated || result.Readme.Revision == 0 || !result.Cleaned || len(result.Open) != 0 || len(result.HintsChanged) != 0 {
		t.Errorf("Antwort: %+v", result)
	}
	for _, gone := range []string{DocsWorkDir(root, "code"), DocsRecordFile(root, "code")} {
		if _, err := os.Stat(gone); !os.IsNotExist(err) {
			t.Errorf("%s nicht entfernt", gone)
		}
	}

	// Ein zweiter Lauf ohne Änderung schreibt nichts, auch die README nicht.
	if _, err := work.Checkout(context.Background(), "code", DocsCheckoutOptions{}); err != nil {
		t.Fatal(err)
	}
	calls = len(store.Calls)
	result, err = work.Publish(context.Background(), "code", DocsPublishOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got := writeCalls(store, calls); len(got) != 0 || result.Readme.Status != DocsUnchanged || !result.Cleaned {
		t.Errorf("zweiter Lauf: %v, %+v", got, result)
	}

	// Weicht die README ab, wird sie mit ihrer Revision neu geschrieben.
	store.Put("docs/generated/code/README.md", "---\ntitle: alt\n---\n")
	if _, err := work.Checkout(context.Background(), "code", DocsCheckoutOptions{}); err != nil {
		t.Fatal(err)
	}
	result, err = work.Publish(context.Background(), "code", DocsPublishOptions{})
	if err != nil || result.Readme.Status != DocsWritten {
		t.Errorf("README abweichend: %+v, %v", result.Readme, err)
	}
	if content, _ := store.Content("docs/generated/code/README.md"); content != codeReadme {
		t.Errorf("README: %q", content)
	}
}

// Fehlt die Vorlage auf dem Hub (Versatz Binary/Hub, Task 133), scheitert nur
// der README-Schritt: Die Dokumente sind veröffentlicht, die README ist
// failed mit template_missing, keine leere README entsteht, und aufgeräumt
// wird trotzdem.
func TestDocsPublishOhneVorlage(t *testing.T) {
	root, store, work, _ := checkoutFixture(t)
	empty, _, _ := docTree(t, map[string]string{"reviews/review-x.md": "x"}, nil)
	work.WithTemplates(empty.Dist)
	writeWorkFile(t, root, "02-aendern.md", docsDoc("Geändert"))

	result, err := work.Publish(context.Background(), "code", DocsPublishOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got := statuses(result); got["02-aendern.md"] != DocsWritten {
		t.Errorf("Zustände: %v", got)
	}
	if result.Readme.Status != DocsFailed || result.Readme.Code != DocCodeTemplateMissing || result.Readme.Ref != "keph:/docs/generated/code/README.md" {
		t.Errorf("README: %+v", result.Readme)
	}
	if _, ok := store.Content("docs/generated/code/README.md"); ok {
		t.Error("README ohne Vorlage angelegt")
	}
	if !result.Cleaned || len(result.Open) != 0 {
		t.Errorf("nicht aufgeräumt: %+v", result)
	}
	if _, err := os.Stat(DocsWorkDir(root, "code")); !os.IsNotExist(err) {
		t.Errorf("Arbeitsverzeichnis liegt noch: %v", err)
	}
}

// Nach einem aufgeräumten publish sind auch .k-playbook/work/docs/ und
// .k-playbook/work/ weg, wenn sie leer sind (Task 124); liegt dort noch das
// Arbeitsverzeichnis eines anderen Erzeugers, bleiben sie. Ein publish mit
// offenem Konflikt räumt nichts ab, checkout mit discard ebenso wenig.
func TestDocsPublishRaeumtLeereElternAb(t *testing.T) {
	root, store, work := docsWorkFixture(t)
	store.Put("docs/generated/code/01-a.md", docsDoc("A"))
	store.Put("docs/generated/libs/go.md", docsDoc("Go"))
	docsRoot := filepath.Dir(DocsWorkDir(root, "code"))
	workRoot := filepath.Dir(docsRoot)

	for _, name := range []string{"code", "libs"} {
		if _, err := work.Checkout(context.Background(), name, DocsCheckoutOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	// checkout mit discard räumt nur das eigene Arbeitsverzeichnis ab.
	if _, err := work.Checkout(context.Background(), "code", DocsCheckoutOptions{Discard: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(DocsWorkDir(root, "code")); err != nil {
		t.Fatalf("discard: %v", err)
	}

	result, err := work.Publish(context.Background(), "code", DocsPublishOptions{})
	if err != nil || !result.Cleaned {
		t.Fatalf("code: %+v %v", result, err)
	}
	if _, err := os.Stat(DocsWorkDir(root, "libs")); err != nil {
		t.Fatalf("Arbeitsverzeichnis libs entfernt: %v", err)
	}
	if _, err := os.Stat(docsRoot); err != nil {
		t.Fatalf("%s entfernt, obwohl libs noch liegt: %v", docsRoot, err)
	}

	// Ein offener Konflikt: nichts wird aufgeräumt.
	writeWorkFileFor(t, root, "libs", "go.md", docsDoc("Go lokal"))
	store.Put("docs/generated/libs/go.md", docsDoc("Go fremd"))
	result, err = work.Publish(context.Background(), "libs", DocsPublishOptions{})
	if err != nil || result.Cleaned || len(result.Open) != 1 {
		t.Fatalf("Konflikt: %+v %v", result, err)
	}
	if _, err := os.Stat(docsRoot); err != nil {
		t.Fatalf("%s nach Konflikt entfernt: %v", docsRoot, err)
	}

	// Geklärt und neu geholt: Das letzte publish räumt beide Eltern ab.
	if _, err := work.Checkout(context.Background(), "libs", DocsCheckoutOptions{Discard: true}); err != nil {
		t.Fatal(err)
	}
	result, err = work.Publish(context.Background(), "libs", DocsPublishOptions{})
	if err != nil || !result.Cleaned {
		t.Fatalf("libs: %+v %v", result, err)
	}
	for _, gone := range []string{docsRoot, workRoot} {
		if _, err := os.Stat(gone); !os.IsNotExist(err) {
			t.Errorf("%s nicht entfernt: %v", gone, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, ProjectDirName)); err != nil {
		t.Errorf(".k-playbook/ selbst entfernt: %v", err)
	}
}

// Ein Dokument, das seit dem checkout in Kephalaion geändert wurde, wird
// gemeldet, nicht geschrieben; ein fehlgeschlagener Aufruf ebenso. Die
// übrigen gehen durch, Arbeitsverzeichnis und Merkdatei bleiben, und ein
// zweites publish nach der Klärung schreibt nur noch den Rest.
func TestDocsPublishKonfliktUndZweitesPublish(t *testing.T) {
	root, store, work, revisions := checkoutFixture(t)
	writeWorkFile(t, root, "01-bleibt.md", docsDoc("Bleibt lokal"))
	writeWorkFile(t, root, "02-aendern.md", docsDoc("Geändert"))
	writeWorkFile(t, root, "06-neu.md", docsDoc("Neu"))
	store.Put("docs/generated/code/01-bleibt.md", docsDoc("Bleibt fremd"))
	fremd, _ := store.Content("docs/generated/code/01-bleibt.md")
	store.Fail["create"] = &kephalaion.ToolError{Tool: "create", Code: kephalaion.CodeUnreachable, Message: "Hub weg"}

	result, err := work.Publish(context.Background(), "code", DocsPublishOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got := statuses(result); got["01-bleibt.md"] != DocsConflict || got["02-aendern.md"] != DocsWritten || got["06-neu.md"] != DocsFailed {
		t.Fatalf("Zustände: %v", got)
	}
	for _, item := range result.Docs {
		switch item.Path {
		case "01-bleibt.md":
			if item.Code != DocCodeStaleRevision || item.Base != revisions["01-bleibt.md"] || item.Revision <= item.Base {
				t.Errorf("Konflikt: %+v", item)
			}
		case "06-neu.md":
			if item.Code != DocCodeKephalaionUnavailable {
				t.Errorf("fehlgeschlagen: %+v", item)
			}
		}
	}
	if content, _ := store.Content("docs/generated/code/01-bleibt.md"); content != fremd {
		t.Errorf("fremde Änderung überschrieben: %q", content)
	}
	if result.Cleaned || !reflect.DeepEqual(result.Open, []string{"01-bleibt.md", "06-neu.md"}) || result.Readme.Status != DocsFailed {
		t.Errorf("Antwort: %+v", result)
	}
	record, ok, _ := readDocsRecord(DocsRecordFile(root, "code"))
	if item, _ := record.find("02-aendern.md"); !ok || item.Hash != docsHash([]byte(docsDoc("Geändert"))) || item.Revision == revisions["02-aendern.md"] {
		t.Errorf("Merkdatei nach dem Schreiben: %+v", item)
	}
	if item, _ := record.find("01-bleibt.md"); item.Revision != revisions["01-bleibt.md"] {
		t.Errorf("Merkdatei beim Konflikt verändert: %+v", item)
	}

	// Klärung: die eigene Änderung an 01 wird verworfen (zurück auf den
	// Stand beim checkout), der Hub ist wieder da.
	writeWorkFile(t, root, "01-bleibt.md", docsDoc("Bleibt"))
	delete(store.Fail, "create")
	calls := len(store.Calls)
	result, err = work.Publish(context.Background(), "code", DocsPublishOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got := writeCalls(store, calls); !reflect.DeepEqual(got, []string{"create docs/generated/code/06-neu.md", "create docs/generated/code/README.md"}) {
		t.Errorf("zweites publish: %v", got)
	}
	if got := statuses(result); got["02-aendern.md"] != DocsUnchanged || got["06-neu.md"] != DocsCreated || got["01-bleibt.md"] != DocsUnchanged {
		t.Errorf("Zustände: %v", got)
	}
	if !result.Cleaned || len(result.Open) != 0 {
		t.Errorf("nicht aufgeräumt: %+v", result)
	}
}

// outcome_unknown: der Vorgang kann angekommen sein; gemeldet als
// fehlgeschlagen mit dem Code, nicht wiederholt, Arbeitsverzeichnis bleibt.
func TestDocsPublishOutcomeUnknown(t *testing.T) {
	root, store, work, _ := checkoutFixture(t)
	writeWorkFile(t, root, "02-aendern.md", docsDoc("Geändert"))
	store.Fail["write"] = &kephalaion.ToolError{Tool: "write", Code: kephalaion.CodeOutcomeUnknown, Message: "offen"}
	calls := len(store.Calls)

	result, err := work.Publish(context.Background(), "code", DocsPublishOptions{})
	if err != nil {
		t.Fatal(err)
	}
	item := result.Docs[1]
	if item.Path != "02-aendern.md" || item.Status != DocsFailed || item.Code != DocCodeOutcomeUnknown {
		t.Errorf("Eintrag: %+v", item)
	}
	writes := 0
	for _, call := range writeCalls(store, calls) {
		if strings.HasPrefix(call, "write docs/generated/code/02") {
			writes++
		}
	}
	if writes != 1 {
		t.Errorf("write %d-mal aufgerufen, erwartet einmal", writes)
	}
	if result.Cleaned || !reflect.DeepEqual(result.Open, []string{"02-aendern.md"}) {
		t.Errorf("Antwort: %+v", result)
	}
	if _, err := os.Stat(DocsRecordFile(root, "code")); err != nil {
		t.Errorf("Merkdatei weg: %v", err)
	}
}

// Ablehnungen im Arbeitsverzeichnis schreiben nichts: README.md, Dateien
// ohne .md, fehlendes Frontmatter oder fehlende description.
func TestDocsPublishAblehnungen(t *testing.T) {
	for name, prepare := range map[string]func(t *testing.T, root string){
		"README": func(t *testing.T, root string) { writeWorkFile(t, root, "README.md", docsDoc("Liesmich")) },
		"txt":    func(t *testing.T, root string) { writeWorkFile(t, root, "notiz.txt", "x") },
		"ohne Frontmatter": func(t *testing.T, root string) {
			writeWorkFile(t, root, "02-aendern.md", "# Ohne Frontmatter\n")
		},
		"ohne description": func(t *testing.T, root string) {
			writeWorkFile(t, root, "07-neu.md", "---\ntitle: Neu\n---\n")
		},
		"Name": func(t *testing.T, root string) { writeWorkFile(t, root, "Gross/x.md", docsDoc("X")) },
	} {
		t.Run(name, func(t *testing.T) {
			root, store, work, _ := checkoutFixture(t)
			writeWorkFile(t, root, "01-bleibt.md", docsDoc("Bleibt geändert"))
			prepare(t, root)
			calls := len(store.Calls)
			_, err := work.Publish(context.Background(), "code", DocsPublishOptions{AllowHints: true})
			if DocRefErrorCode(err) != DocCodeInvalidInput || !strings.Contains(err.Error(), "nichts veröffentlicht") {
				t.Fatalf("Fehler: %v (%s)", err, DocRefErrorCode(err))
			}
			if got := writeCalls(store, calls); len(got) != 0 {
				t.Errorf("geschrieben: %v", got)
			}
			if _, err := os.Stat(DocsRecordFile(root, "code")); err != nil {
				t.Errorf("Merkdatei weg: %v", err)
			}
		})
	}
}

// hints: geändert, Dokument mit hints gelöscht oder neu angelegt — ohne
// allow_hints wird nichts geschrieben, mit allow_hints geschrieben und
// gemeldet.
func TestDocsPublishHints(t *testing.T) {
	for name, prepare := range map[string]func(t *testing.T, root string){
		"geändert": func(t *testing.T, root string) {
			writeWorkFile(t, root, "sub/04-hints.md", docsDoc("Hints", "hints: [ \"Anders\" ]\n"))
		},
		"entfernt": func(t *testing.T, root string) { writeWorkFile(t, root, "sub/04-hints.md", docsDoc("Hints")) },
		"gelöscht": func(t *testing.T, root string) { removeWorkFile(t, root, "sub/04-hints.md") },
		"umbenannt": func(t *testing.T, root string) {
			removeWorkFile(t, root, "sub/04-hints.md")
			writeWorkFile(t, root, "sub/04-hinweise.md", docsDoc("Hints", "hints: [ \"Auf .devcontainer/ achten\" ]\n"))
		},
		"neu mit hints": func(t *testing.T, root string) {
			writeWorkFile(t, root, "08-neu.md", docsDoc("Neu", "hints: [ \"Neu\" ]\n"))
		},
		"neu auf hints eines anderen": func(t *testing.T, root string) {
			writeWorkFile(t, root, "02-aendern.md", docsDoc("Ändern", "hints: [ \"Neu\" ]\n"))
		},
	} {
		t.Run(name, func(t *testing.T) {
			root, store, work, _ := checkoutFixture(t)
			prepare(t, root)
			calls := len(store.Calls)
			_, err := work.Publish(context.Background(), "code", DocsPublishOptions{})
			if DocRefErrorCode(err) != DocCodeHintsChanged || !strings.Contains(err.Error(), "allow_hints") {
				t.Fatalf("ohne allow_hints: %v (%s)", err, DocRefErrorCode(err))
			}
			if got := writeCalls(store, calls); len(got) != 0 {
				t.Errorf("ohne allow_hints geschrieben: %v", got)
			}

			result, err := work.Publish(context.Background(), "code", DocsPublishOptions{AllowHints: true})
			if err != nil {
				t.Fatal(err)
			}
			if len(result.HintsChanged) == 0 || !result.Cleaned {
				t.Errorf("mit allow_hints: %+v", result)
			}
			for _, rel := range result.HintsChanged {
				if status := statuses(result)[rel]; status == DocsUnchanged || status == "" {
					t.Errorf("%s gemeldet, aber %q", rel, status)
				}
			}
		})
	}

	// Ein geändertes Dokument mit unveränderten hints braucht kein
	// allow_hints.
	root, _, work, _ := checkoutFixture(t)
	writeWorkFile(t, root, "sub/04-hints.md", docsDoc("Hints neu geschrieben", "hints:\n  - Auf .devcontainer/ achten\n"))
	result, err := work.Publish(context.Background(), "code", DocsPublishOptions{})
	if err != nil || statuses(result)["sub/04-hints.md"] != DocsWritten || len(result.HintsChanged) != 0 {
		t.Errorf("hints unverändert: %+v, %v", result, err)
	}
}

// Ohne checkout, ohne Arbeitsverzeichnis oder mit einer Merkdatei aus einer
// anderen Collection wird nichts veröffentlicht.
func TestDocsPublishOhneCheckout(t *testing.T) {
	root, store, work := docsWorkFixture(t)
	if _, err := work.Publish(context.Background(), "code", DocsPublishOptions{}); DocRefErrorCode(err) != DocCodeNotCheckedOut {
		t.Errorf("ohne checkout: %v", err)
	}
	if _, err := work.Checkout(context.Background(), "code", DocsCheckoutOptions{}); err != nil {
		t.Fatal(err)
	}
	other := NewDocsWork(root, store, "vm:anderes")
	if _, err := other.Publish(context.Background(), "code", DocsPublishOptions{}); DocRefErrorCode(err) != DocCodeInvalidInput {
		t.Errorf("andere Collection: %v", err)
	}
	if err := os.RemoveAll(DocsWorkDir(root, "code")); err != nil {
		t.Fatal(err)
	}
	calls := len(store.Calls)
	if _, err := work.Publish(context.Background(), "code", DocsPublishOptions{}); DocRefErrorCode(err) != DocCodeNotCheckedOut {
		t.Errorf("ohne Arbeitsverzeichnis: %v", err)
	}
	if len(store.Calls) != calls {
		t.Errorf("Kephalaion gefragt: %v", store.Calls[calls:])
	}
}
