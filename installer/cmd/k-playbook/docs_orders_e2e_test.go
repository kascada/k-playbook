package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/kephalaion/kephtest"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// useCLIDocsOrders setzt eine Collection im Speicher hinter docs orders ein.
func useCLIDocsOrders(t *testing.T) *kephtest.Store {
	t.Helper()
	store := kephtest.New()
	original := openDocsOrders
	openDocsOrders = func(string) (*project.DocsOrders, error) { return project.NewDocsOrders(store, "kamran") }
	t.Cleanup(func() { openDocsOrders = original })
	return store
}

func cliRawOrder(session string, pfade ...string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "---\ntitle: \"Nachzug\"\nerzeuger: code\npfade:\n")
	for _, p := range pfade {
		fmt.Fprintf(&b, "  - %s\n", p)
	}
	fmt.Fprintf(&b, "anlass: \"Anlass\"\nsitzung: %s\nentstanden: 2026-11-20\n---\n", session)
	return b.String()
}

// withStdin setzt stdin für die Dauer des Aufrufs.
func withStdin(t *testing.T, text string) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "stdin")
	if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	in, err := os.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	before := os.Stdin
	os.Stdin = in
	t.Cleanup(func() { os.Stdin = before; in.Close() })
}

// docs orders: Liste und --json, ein Erzeuger ist Pflicht und muss bekannt
// sein.
func TestDocsOrdersSubkommando(t *testing.T) {
	todoProjectRoot(t)
	store := useCLIDocsOrders(t)

	stdout, _, err := runDocsCaptured(t, "orders", "code")
	if err != nil || !strings.Contains(stdout, "Nichts zu tun: keine offenen Aufträge für code") {
		t.Fatalf("leer: %q %v", stdout, err)
	}
	store.Put("queue/docs/001-a.md", cliRawOrder("s1", "installer/a"))
	store.Put("queue/docs/002-b.md", cliRawOrder("s2", "installer/a/x"))
	stdout, _, err = runDocsCaptured(t, "orders", "code", "--json")
	var result project.DocsOrdersResult
	if err != nil || json.Unmarshal([]byte(stdout), &result) != nil || result.Run != project.DocsRunPartial ||
		len(result.RawOrders) != 1 || len(result.RawOrders[0].Refs) != 2 || result.WaitMinutes != 120 {
		t.Fatalf("json: %q %v", stdout, err)
	}
	stdout, _, err = runDocsCaptured(t, "orders", "code")
	if err != nil || !strings.Contains(stdout, "Lauf: Teil-Lauf") || !strings.Contains(stdout, "keph:/queue/docs/001-a.md\t2026-11-20\troh, reif\tAnlass") ||
		!strings.Contains(stdout, "roh 1: installer/a, installer/a/x (2026-11-20) aus 2 Auftrag/Aufträgen") {
		t.Errorf("Text: %q %v", stdout, err)
	}
	store.Put("queue/docs/003-ohne.md", "---\ntitle: \"t\"\nerzeuger: code\nanlass: \"a\"\nsitzung: s3\nentstanden: 2026-11-20\n---\n")
	stdout, _, err = runDocsCaptured(t, "orders", "code")
	if err != nil || !strings.Contains(stdout, "keph:/queue/docs/003-ohne.md\t2026-11-20\troh, reif, unvollständig (ohne Pfad)\ta") ||
		!strings.Contains(stdout, "Problem: keph:/queue/docs/003-ohne.md: ohne gültigen Pfad — unvollständig") {
		t.Errorf("unvollständig: %q %v", stdout, err)
	}
	if _, _, err := runDocsCaptured(t, "orders"); err == nil || !strings.Contains(err.Error(), "docs orders <code|tools>") {
		t.Errorf("ohne Erzeuger: %v", err)
	}
	if _, _, err := runDocsCaptured(t, "orders", "libs"); err == nil || !strings.Contains(err.Error(), "(invalid_input)") {
		t.Errorf("unbekannter Erzeuger: %v", err)
	}
	if _, _, err := runDocsCaptured(t, "orders", "code", "--weg"); err == nil || !strings.Contains(err.Error(), "unbekannte Option --weg") {
		t.Errorf("unbekannte Option: %v", err)
	}
}

// docs order add: angelegt, gedeckt, ergänzt, übersprungen — auch ohne
// Kephalaion (skipped, kein Fehler).
func TestDocsOrderAddSubkommando(t *testing.T) {
	todoProjectRoot(t)
	store := useCLIDocsOrders(t)
	args := func(extra ...string) []string {
		return append([]string{"order", "add", "code", "--sitzung", "S1", "--anlass", "Neuer Weg"}, extra...)
	}

	stdout, _, err := runDocsCaptured(t, args("--pfad", "installer/a")...)
	if err != nil || !strings.HasPrefix(stdout, "skipped\tcode\t") || !strings.Contains(stdout, "no_generated_docs") {
		t.Fatalf("ohne erzeugte Doku: %q %v", stdout, err)
	}
	store.Put("docs/generated/code/01-a.md", "x")
	stdout, _, err = runDocsCaptured(t, append(args("--json", "--pfad", "installer/a", "installer/b"), "--title=Mein Titel")...)
	var added project.DocsOrderAddResult
	if err != nil || json.Unmarshal([]byte(stdout), &added) != nil || added.Status != project.DocsOrderCreated || len(added.Pfade) != 2 {
		t.Fatalf("angelegt: %q %v", stdout, err)
	}
	if content, _ := store.Content(strings.TrimPrefix(added.Ref, "keph:/")); !strings.Contains(content, "title: \"Mein Titel\"") || !strings.Contains(content, "sitzung: S1") {
		t.Errorf("Inhalt: %s", content)
	}
	stdout, _, err = runDocsCaptured(t, args("--pfad=installer/a/c")...)
	if err != nil || !strings.HasPrefix(stdout, "covered\tcode\t"+added.Ref) {
		t.Errorf("gedeckt: %q %v", stdout, err)
	}
	stdout, _, err = runDocsCaptured(t, args("--pfad", "lib/z")...)
	if err != nil || !strings.HasPrefix(stdout, "extended\tcode\t"+added.Ref) {
		t.Errorf("ergänzt: %q %v", stdout, err)
	}

	for name, bad := range map[string][]string{
		"ohne Pfad":    {"order", "add", "code", "--sitzung", "S", "--anlass", "x"},
		"ohne Sitzung": {"order", "add", "code", "--pfad", "a", "--anlass", "x"},
		"ohne Anlass":  {"order", "add", "code", "--pfad", "a", "--sitzung", "S"},
		"ohne Wert":    {"order", "add", "code", "--pfad", "a", "--anlass"},
		"Erzeuger":     {"order", "add", "libs", "--pfad", "a", "--sitzung", "S", "--anlass", "x"},
		"Unterbefehl":  {"order", "weg"},
		"Option":       {"order", "add", "code", "--weg", "x"},
	} {
		if _, _, err := runDocsCaptured(t, bad...); err == nil {
			t.Errorf("%s: kein Fehler", name)
		}
	}

	// Ohne bereite Anbindung wird nicht eingereiht; das ist kein Fehler.
	original := openDocsOrders
	openDocsOrders = func(string) (*project.DocsOrders, error) {
		return nil, project.DocsOrderOpenError("Kephalaion ist nicht bereit")
	}
	t.Cleanup(func() { openDocsOrders = original })
	stdout, _, err = runDocsCaptured(t, args("--json", "--pfad", "a")...)
	var skipped project.DocsOrderAddResult
	if err != nil || json.Unmarshal([]byte(stdout), &skipped) != nil || skipped.Status != project.DocsOrderSkipped ||
		skipped.Reason != project.DocsReasonKephalaionUnavailable || !strings.Contains(skipped.Message, "nicht bereit") {
		t.Errorf("ohne Kephalaion: %q %v", stdout, err)
	}
	// Alles andere ist ein Fehler, auch bei orders.
	if _, _, err := runDocsCaptured(t, "orders", "code"); err == nil || !strings.Contains(err.Error(), "(kephalaion_unavailable)") {
		t.Errorf("orders ohne Kephalaion: %v", err)
	}
}

// docs orders split: JSON von stdin, zerlegte Aufträge, rohe nach done/, ein
// Fehler endet mit Exit 1 nach der Ausgabe.
func TestDocsOrdersSplitSubkommando(t *testing.T) {
	todoProjectRoot(t)
	store := useCLIDocsOrders(t)
	store.Put("queue/docs/001-a.md", cliRawOrder("s1", "installer/a"))

	withStdin(t, `{"sitzung":"V1","documents":[{"dokument":"11-x.md","aktion":"umschreiben","pfade":["installer/a"],"anlaesse":[{"datum":"2026-11-20","anlass":"Anlass"}]}],"raw":["keph:/queue/docs/001-a.md"]}`)
	stdout, _, err := runDocsCaptured(t, "orders", "split", "--json")
	var result project.DocsSplitResult
	if err != nil || json.Unmarshal([]byte(stdout), &result) != nil || len(result.Documents) != 1 || result.Documents[0].Status != project.DocsOrderCreated ||
		len(result.ClosedRaw) != 1 || result.ClosedRaw[0] != "keph:/queue/docs/001-a.md" {
		t.Fatalf("split: %q %v", stdout, err)
	}
	if _, ok := store.Content("queue/docs/done/001-a.md"); !ok {
		t.Errorf("roher Auftrag nicht in done/: %v", store.Names())
	}

	if _, _, err := runDocsCaptured(t, "orders", "split"); err == nil || !strings.Contains(err.Error(), "--json ist Pflicht") {
		t.Errorf("ohne --json: %v", err)
	}
	withStdin(t, `{"sitzung":"V1","unbekannt":1}`)
	if _, _, err := runDocsCaptured(t, "orders", "split", "--json"); err == nil || !strings.Contains(err.Error(), "(invalid_input)") {
		t.Errorf("unbekanntes Feld: %v", err)
	}
	withStdin(t, `{"documents":[]}`)
	if _, _, err := runDocsCaptured(t, "orders", "split", "--json"); err == nil || !strings.Contains(err.Error(), "(invalid_input)") {
		t.Errorf("ohne Sitzung: %v", err)
	}

	store.Fail["create"] = fmt.Errorf("kaputt")
	withStdin(t, `{"sitzung":"V1","documents":[{"dokument":"12-y.md","aktion":"neu","pfade":["a"],"anlass":"x"}]}`)
	stdout, _, err = runDocsCaptured(t, "orders", "split", "--json")
	if err == nil || !strings.Contains(err.Error(), "12-y.md") || !strings.Contains(stdout, `"status": "failed"`) {
		t.Errorf("failed: %q %v", stdout, err)
	}
}

// docs scope: die Zeilen des Preflights, --json, --check, ein ungültiger
// Zusatzpfad.
func TestDocsScopeSubkommando(t *testing.T) {
	root := todoProjectRoot(t)
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("*.tmp\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "a.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	doubled := filepath.Base(root) + "/src/a.go"

	stdout, _, err := runDocsCaptured(t, "scope", "src", "--check", "src/a.go", "x.tmp", ".env", "keph-backup/z.md", doubled)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Scope         " + root + " (project.repoRoot)\n", "Zusätzlich    src\n", "Immer außen   /.k-playbook/, keph-backup/, .git/\n",
		"Versteckt     ausgeschlossen (.*)\n", "Aus K-PLAYBOOK.yaml  ", "Ausschlüsse   venv/, env/, node_modules/", ".gitignore (1)",
		"im Scope\tsrc/a.go\n", "ausgeschlossen\tx.tmp\tgitignore\t.gitignore: *.tmp\tfehlt\n", "ausgeschlossen\t.env\thidden\t.\tfehlt\n",
		"ausgeschlossen\tkeph-backup/z.md\talways\tkeph-backup/\tfehlt\n", "im Scope\t" + doubled + "\tfehlt\tVorschlag: src/a.go\n",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("Ausgabe ohne %q:\n%s", want, stdout)
		}
	}
	stdout, _, err = runDocsCaptured(t, "scope", "--json")
	var result project.DocsScopeResult
	if err != nil || json.Unmarshal([]byte(stdout), &result) != nil || result.ScanRoot != root || result.Extra != nil || !result.Exclude.Hidden.Excluded {
		t.Errorf("json: %q %v", stdout, err)
	}
	for name, args := range map[string][]string{
		"fehlt":      {"scope", "gibt-es-nicht"},
		"außerhalb":  {"scope", ".."},
		"always":     {"scope", ".k-playbook"},
		"zwei Pfade": {"scope", "src", "docs"},
		"check leer": {"scope", "--check"},
		"Option":     {"scope", "--weg"},
	} {
		if _, _, err := runDocsCaptured(t, args...); err == nil {
			t.Errorf("%s: kein Fehler", name)
		}
	}
}

// docs order add ohne --json: Berichtigungen und fehlende Pfade je eine Zeile
// nach der Statuszeile.
func TestPrintDocsOrderAddBerichtigt(t *testing.T) {
	var out strings.Builder
	printDocsOrderAdd(&out, project.DocsOrderAddResult{
		Producer: "code", Status: project.DocsOrderCreated, Ref: "keph:/queue/docs/001-a.md", Revision: 7,
		Corrected: []project.DocsPathCorrection{{From: "repo/a.go", To: "a.go"}}, Missing: []string{"weg.go"},
	})
	if want := "created\tcode\tkeph:/queue/docs/001-a.md\t7\nberichtigt\trepo/a.go → a.go\nfehlt\tweg.go\n"; out.String() != want {
		t.Errorf("Ausgabe %q, erwartet %q", out.String(), want)
	}
}
