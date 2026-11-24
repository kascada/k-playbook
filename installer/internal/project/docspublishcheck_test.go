package project

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/kephalaion/kephtest"
)

// Prüfmodus von publish (Task 195): Er schreibt nichts und meldet Plan,
// Ablehnungen, geänderte hints und absehbare Konflikte gemeinsam.

// docsTreeSnapshot liest alle Dateien unter dir mit Inhalt.
func docsTreeSnapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(dir, func(file string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, file)
		files[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// storeSnapshot sind Name und Revision jedes Dokuments im Store.
func storeSnapshot(store *kephtest.Store) map[string]int64 {
	out := map[string]int64{}
	for name, doc := range store.Docs {
		out[name] = doc.Revision
	}
	return out
}

// Im Prüfmodus bleiben Kephalaion, Arbeitsverzeichnis und Merkdatei
// unverändert; Ablehnungen und geänderte hints stehen gemeinsam in der
// Antwort, je Dokument der geplante Vorgang, die README im Vokabular von
// docs readme --check.
func TestDocsPublishPruefmodusSchreibtNichts(t *testing.T) {
	root, store, work, revisions := checkoutFixture(t)
	writeWorkFile(t, root, "02-aendern.md", docsDoc("Geändert"))
	removeWorkFile(t, root, "03-weg.md")
	writeWorkFile(t, root, "sub/04-hints.md", docsDoc("Hints", "hints: [ \"Anders\" ]\n"))
	writeWorkFile(t, root, "sub/05-neu.md", docsDoc("Neu"))
	writeWorkFile(t, root, "README.md", docsDoc("Liesmich"))
	writeWorkFile(t, root, "notiz.txt", "x")
	workBefore := docsTreeSnapshot(t, DocsWorkDir(root, "code"))
	recordBefore, err := os.ReadFile(DocsRecordFile(root, "code"))
	if err != nil {
		t.Fatal(err)
	}
	storeBefore := storeSnapshot(store)
	calls := len(store.Calls)

	result, err := work.Publish(context.Background(), "code", DocsPublishOptions{Check: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := writeCalls(store, calls); len(got) != 0 {
		t.Errorf("geschrieben: %v", got)
	}
	if got := storeSnapshot(store); !reflect.DeepEqual(got, storeBefore) {
		t.Errorf("Kephalaion verändert: %v", got)
	}
	if got := docsTreeSnapshot(t, DocsWorkDir(root, "code")); !reflect.DeepEqual(got, workBefore) {
		t.Errorf("Arbeitsverzeichnis verändert: %v", got)
	}
	if got, _ := os.ReadFile(DocsRecordFile(root, "code")); string(got) != string(recordBefore) {
		t.Errorf("Merkdatei verändert: %s", got)
	}

	want := map[string]string{"01-bleibt.md": DocsUnchanged, "02-aendern.md": DocsWritten, "03-weg.md": DocsDeleted,
		"sub/04-hints.md": DocsWritten, "sub/05-neu.md": DocsCreated}
	if got := statuses(result); !reflect.DeepEqual(got, want) {
		t.Errorf("Zustände: %v", got)
	}
	for _, item := range result.Docs {
		if item.Path == "02-aendern.md" && (item.Base != revisions["02-aendern.md"] || item.Revision != item.Base) {
			t.Errorf("Revision im Prüfmodus: %+v", item)
		}
	}
	if len(result.Problems) != 2 || !strings.HasPrefix(result.Problems[0], "README.md: ") || !strings.HasPrefix(result.Problems[1], "notiz.txt: ") {
		t.Errorf("Ablehnungen: %v", result.Problems)
	}
	if !reflect.DeepEqual(result.HintsChanged, []string{"sub/04-hints.md"}) {
		t.Errorf("hints: %v", result.HintsChanged)
	}
	if !result.Check || result.Cleaned || len(result.Open) != 0 {
		t.Errorf("Antwort: %+v", result)
	}
	if result.Readme.Status != DocsReadmeMissing || result.Readme.Ref != "keph:/docs/generated/code/README.md" {
		t.Errorf("README: %+v", result.Readme)
	}
}

// Je ein absehbarer Konflikt für created, written und deleted; ein lokal
// unverändertes, in Kephalaion neueres Dokument bleibt unchanged.
func TestDocsPublishPruefmodusKonflikte(t *testing.T) {
	root, store, work, revisions := checkoutFixture(t)
	writeWorkFile(t, root, "02-aendern.md", docsDoc("Geändert"))
	removeWorkFile(t, root, "03-weg.md")
	writeWorkFile(t, root, "sub/04-hints.md", docsDoc("Hints neu", "hints: [ \"Auf .devcontainer/ achten\" ]\n"))
	writeWorkFile(t, root, "06-neu.md", docsDoc("Neu"))
	store.Put("docs/generated/code/01-bleibt.md", docsDoc("Bleibt fremd"))
	store.Put("docs/generated/code/02-aendern.md", docsDoc("Ändern fremd"))
	store.Put("docs/generated/code/03-weg.md", docsDoc("Weg fremd"))
	delete(store.Docs, "docs/generated/code/sub/04-hints.md")
	store.Put("docs/generated/code/06-neu.md", docsDoc("Neu fremd"))
	calls := len(store.Calls)

	result, err := work.Publish(context.Background(), "code", DocsPublishOptions{Check: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := writeCalls(store, calls); len(got) != 0 {
		t.Errorf("geschrieben: %v", got)
	}
	codes := map[string]string{}
	for _, item := range result.Docs {
		codes[item.Path] = item.Status + " " + item.Code
		switch item.Path {
		case "02-aendern.md", "03-weg.md", "06-neu.md":
			if item.Revision != store.Docs["docs/generated/"+"code/"+item.Path].Revision || item.Message == "" {
				t.Errorf("Konflikt ohne aktuelle Revision oder Meldung: %+v", item)
			}
		case "01-bleibt.md":
			if item.Revision != revisions["01-bleibt.md"] {
				t.Errorf("unverändert: %+v", item)
			}
		}
	}
	want := map[string]string{
		"01-bleibt.md":    DocsUnchanged + " ",
		"02-aendern.md":   DocsConflict + " " + DocCodeStaleRevision,
		"03-weg.md":       DocsConflict + " " + DocCodeStaleRevision,
		"sub/04-hints.md": DocsConflict + " " + DocCodeNotFound,
		"06-neu.md":       DocsConflict + " " + DocCodeNameTaken,
	}
	if !reflect.DeepEqual(codes, want) {
		t.Errorf("Zustände: %v", codes)
	}
	if len(result.Open) != 0 || result.Cleaned {
		t.Errorf("Antwort: %+v", result)
	}
}

// Bei unverändertem Bestand in Kephalaion liefert ein publish danach, was der
// Prüfmodus angekündigt hat; die README missing wird created.
func TestDocsPublishPruefmodusKuendigtAn(t *testing.T) {
	root, _, work, _ := checkoutFixture(t)
	writeWorkFile(t, root, "02-aendern.md", docsDoc("Geändert"))
	removeWorkFile(t, root, "03-weg.md")
	writeWorkFile(t, root, "sub/05-neu.md", docsDoc("Neu"))

	check, err := work.Publish(context.Background(), "code", DocsPublishOptions{Check: true})
	if err != nil {
		t.Fatal(err)
	}
	result, err := work.Publish(context.Background(), "code", DocsPublishOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(statuses(check), statuses(result)) {
		t.Errorf("angekündigt %v, geschehen %v", statuses(check), statuses(result))
	}
	if check.Readme.Status != DocsReadmeMissing || result.Readme.Status != DocsCreated {
		t.Errorf("README: angekündigt %s, geschehen %s", check.Readme.Status, result.Readme.Status)
	}
	if result.Check || !result.Cleaned {
		t.Errorf("publish: %+v", result)
	}
}

// Ohne Prüfmodus bleibt die Reihenfolge der Ablehnungen: erst invalid_input,
// dann hints_changed; der Prüfmodus meldet beides ohne Fehler.
func TestDocsPublishPruefmodusAblehnungenOhneFehler(t *testing.T) {
	root, _, work, _ := checkoutFixture(t)
	writeWorkFile(t, root, "notiz.txt", "x")
	removeWorkFile(t, root, "sub/04-hints.md")

	if _, err := work.Publish(context.Background(), "code", DocsPublishOptions{}); DocRefErrorCode(err) != DocCodeInvalidInput {
		t.Errorf("publish: %v", err)
	}
	result, err := work.Publish(context.Background(), "code", DocsPublishOptions{Check: true})
	if err != nil || len(result.Problems) != 1 || !reflect.DeepEqual(result.HintsChanged, []string{"sub/04-hints.md"}) {
		t.Errorf("Prüfmodus: %+v, %v", result, err)
	}
	if err := os.Remove(filepath.Join(DocsWorkDir(root, "code"), "notiz.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := work.Publish(context.Background(), "code", DocsPublishOptions{}); DocRefErrorCode(err) != DocCodeHintsChanged {
		t.Errorf("publish ohne allow_hints: %v", err)
	}
}
