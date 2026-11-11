package mcpserver

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kascada/k-playbook/installer/internal/kephalaion/kephtest"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// newDocsWorkProject legt ein Projekt an, dessen Collection im Speicher
// liegt (kephtest.Store statt des Nodes).
func newDocsWorkProject(t *testing.T) (string, *kephtest.Store) {
	t.Helper()
	root := t.TempDir()
	if err := project.CreateConfig(root, "."); err != nil {
		t.Fatalf("Konfiguration anlegen: %v", err)
	}
	store := kephtest.New()
	templates := repoDocsTemplates(t)
	original := openDocsWork
	openDocsWork = func(projectDir string) (*project.DocsWork, error) {
		return project.NewDocsWork(projectDir, store, "vm:projekt").WithTemplates(templates), nil
	}
	t.Cleanup(func() { openDocsWork = original })
	return root, store
}

// decodeDocsWork liest die Antwort eines Werkzeugs als JSON.
func decodeDocsWork[T any](t *testing.T, result *mcp.CallToolResult) T {
	t.Helper()
	if len(result.Content) != 1 {
		t.Fatalf("erwartet genau einen Inhaltsblock, bekommen %d", len(result.Content))
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("Inhalt ist kein Text: %#v", result.Content[0])
	}
	var envelope T
	if err := json.Unmarshal([]byte(text.Text), &envelope); err != nil {
		t.Fatalf("Antwort ist kein JSON: %v — %s", err, text.Text)
	}
	return envelope
}

// checkout und publish über MCP: holen, vorhandenes Arbeitsverzeichnis
// melden, veröffentlichen, hints nur mit allow_hints.
func TestDocsWorkToolsAblauf(t *testing.T) {
	root, store := newDocsWorkProject(t)
	store.Put("docs/generated/code/01-a.md", "---\ntitle: A\ndescription: D\nhints: [ \"Alt\" ]\n---\n")
	ctx := context.Background()

	result, _, _ := docsCheckoutTool(ctx, nil, docsCheckoutInput{ProjectDir: root, Producer: "code"})
	checkout := decodeDocsWork[docsCheckoutEnvelope](t, result)
	if result.IsError || !checkout.OK || checkout.Tool != docsToolCheckout || checkout.DocsCheckoutResult == nil ||
		checkout.State != project.DocsStateCheckedOut || len(checkout.Docs) != 1 || checkout.Docs[0].Ref != "keph:/docs/generated/code/01-a.md" {
		t.Fatalf("checkout: %+v", checkout)
	}
	result, _, _ = docsCheckoutTool(ctx, nil, docsCheckoutInput{ProjectDir: root, Producer: "code"})
	checkout = decodeDocsWork[docsCheckoutEnvelope](t, result)
	if result.IsError || checkout.State != project.DocsStateExisting || checkout.Existing == nil || checkout.Existing.Files != 1 {
		t.Fatalf("checkout vorhanden: %+v", checkout)
	}

	file := filepath.Join(project.DocsWorkDir(root, "code"), "01-a.md")
	if err := os.WriteFile(file, []byte("---\ntitle: A\ndescription: D\nhints: [ \"Neu\" ]\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, _, _ = docsPublishTool(ctx, nil, docsPublishInput{ProjectDir: root, Producer: "code"})
	publish := decodeDocsWork[docsPublishEnvelope](t, result)
	if !result.IsError || publish.OK || publish.Error == nil || publish.Error.Code != project.DocCodeHintsChanged {
		t.Fatalf("publish ohne allow_hints: %+v", publish)
	}
	result, _, _ = docsPublishTool(ctx, nil, docsPublishInput{ProjectDir: root, Producer: "code", AllowHints: true})
	publish = decodeDocsWork[docsPublishEnvelope](t, result)
	if result.IsError || !publish.OK || publish.DocsPublishResult == nil || !publish.Cleaned ||
		len(publish.HintsChanged) != 1 || publish.Docs[0].Status != project.DocsWritten || publish.Readme.Status != project.DocsCreated {
		t.Fatalf("publish mit allow_hints: %+v", publish)
	}
	if content, _ := store.Content("docs/generated/code/01-a.md"); !strings.Contains(content, "Neu") {
		t.Errorf("nicht geschrieben: %q", content)
	}

	// Ein publish mit Konflikt ist kein Fehler des Werkzeugs.
	docsCheckoutTool(ctx, nil, docsCheckoutInput{ProjectDir: root, Producer: "code"})
	if err := os.WriteFile(file, []byte("---\ntitle: A lokal\ndescription: D\nhints: [ \"Neu\" ]\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	store.Put("docs/generated/code/01-a.md", "---\ntitle: A fremd\ndescription: D\n---\n")
	result, _, _ = docsPublishTool(ctx, nil, docsPublishInput{ProjectDir: root, Producer: "code"})
	publish = decodeDocsWork[docsPublishEnvelope](t, result)
	if result.IsError || !publish.OK || publish.Cleaned || len(publish.Open) != 1 || publish.Docs[0].Status != project.DocsConflict {
		t.Fatalf("publish mit Konflikt: %+v", publish)
	}

	// discard verwirft und holt neu.
	result, _, _ = docsCheckoutTool(ctx, nil, docsCheckoutInput{ProjectDir: root, Producer: "code", Discard: true})
	checkout = decodeDocsWork[docsCheckoutEnvelope](t, result)
	if result.IsError || !checkout.Discarded || checkout.State != project.DocsStateCheckedOut {
		t.Fatalf("discard: %+v", checkout)
	}
}

// Fehlercodes wie bei den doc-Werkzeugen: projectDir, unbekannter Erzeuger,
// ohne Kephalaion, ohne checkout.
func TestDocsWorkToolsFehler(t *testing.T) {
	root, _ := newDocsWorkProject(t)
	ctx := context.Background()

	result, _, _ := docsCheckoutTool(ctx, nil, docsCheckoutInput{Producer: "code"})
	if envelope := decodeDocsWork[docsCheckoutEnvelope](t, result); !result.IsError || envelope.Error.Code != "invalid_input" {
		t.Errorf("ohne projectDir: %+v", envelope)
	}
	result, _, _ = docsCheckoutTool(ctx, nil, docsCheckoutInput{ProjectDir: root, Producer: "tools"})
	if envelope := decodeDocsWork[docsCheckoutEnvelope](t, result); !result.IsError || envelope.Error.Code != project.DocCodeInvalidInput {
		t.Errorf("unbekannter Erzeuger: %+v", envelope)
	}
	result, _, _ = docsPublishTool(ctx, nil, docsPublishInput{ProjectDir: root, Producer: "code"})
	if envelope := decodeDocsWork[docsPublishEnvelope](t, result); !result.IsError || envelope.Error.Code != project.DocCodeNotCheckedOut {
		t.Errorf("ohne checkout: %+v", envelope)
	}

	openDocsWork = project.OpenDocsWork
	for _, call := range []func() *mcp.CallToolResult{
		func() *mcp.CallToolResult {
			r, _, _ := docsCheckoutTool(ctx, nil, docsCheckoutInput{ProjectDir: root, Producer: "code"})
			return r
		},
		func() *mcp.CallToolResult {
			r, _, _ := docsPublishTool(ctx, nil, docsPublishInput{ProjectDir: root, Producer: "code"})
			return r
		},
	} {
		result := call()
		envelope := decodeDocsWork[docsPublishEnvelope](t, result)
		if !result.IsError || envelope.Error == nil || envelope.Error.Code != project.DocCodeKephalaionUnavailable {
			t.Errorf("ohne Kephalaion: %+v", envelope)
		}
	}
	if _, err := os.Stat(filepath.Join(root, project.ProjectDirName, "work")); !os.IsNotExist(err) {
		t.Errorf("ohne Kephalaion geschrieben: %v", err)
	}
}

// Das Schema nennt die Erzeuger der Tabelle als erlaubte Werte.
func TestDocsWorkToolsSchema(t *testing.T) {
	for _, schema := range []any{docsProducerSchema[docsCheckoutInput](), docsProducerSchema[docsPublishInput]()} {
		data, err := json.Marshal(schema)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), `"erzeuger":{"type":"string","description":`) && !strings.Contains(string(data), `"enum":["code","libs"]`) {
			t.Errorf("Schema ohne Erzeuger: %s", data)
		}
		if !strings.Contains(string(data), `"enum":["code","libs"]`) {
			t.Errorf("Schema ohne enum: %s", data)
		}
	}
}

// exclude_add über MCP: trägt ein, meldet Verworfenes unter rejected, lässt
// die Datei bei nichts Neuem unberührt, und die Kontextausgabe trägt die
// Liste danach. Ohne Muster ist das invalid_input.
func TestDocsExcludeAddToolAblauf(t *testing.T) {
	root := t.TempDir()
	if err := project.CreateConfig(root, "."); err != nil {
		t.Fatalf("Konfiguration anlegen: %v", err)
	}
	ctx := context.Background()

	result, _, _ := docsExcludeAddTool(ctx, nil, docsExcludeAddInput{ProjectDir: root, Patterns: []string{"docs/", "../weg"}})
	added := decodeDocsWork[docsExcludeEnvelope](t, result)
	if result.IsError || !added.OK || added.Tool != docsToolExcludeAdd || added.DocsExcludeResult == nil || !added.Changed ||
		len(added.Added) != 1 || added.Added[0] != "docs/" || len(added.Exclude) != 1 ||
		len(added.Rejected) != 1 || added.Rejected[0].Pattern != "../weg" || added.Rejected[0].Reason == "" {
		t.Fatalf("exclude_add: %+v", added)
	}
	before, err := os.ReadFile(project.ConfigPath(root))
	if err != nil {
		t.Fatal(err)
	}

	result, _, _ = docsExcludeAddTool(ctx, nil, docsExcludeAddInput{ProjectDir: root, Patterns: []string{"docs/"}})
	again := decodeDocsWork[docsExcludeEnvelope](t, result)
	if result.IsError || !again.OK || again.Changed || len(again.Added) != 0 || len(again.Exclude) != 1 {
		t.Fatalf("exclude_add Duplikat: %+v", again)
	}
	after, err := os.ReadFile(project.ConfigPath(root))
	if err != nil || string(after) != string(before) {
		t.Fatalf("Datei bei nichts Neuem geändert: %v\n%s", err, after)
	}

	built, err := project.BuildContext(root)
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}
	if len(built.Docs.Exclude) != 1 || built.Docs.Exclude[0] != "docs/" {
		t.Errorf("context.docs.exclude = %v", built.Docs.Exclude)
	}

	result, _, _ = docsExcludeAddTool(ctx, nil, docsExcludeAddInput{ProjectDir: root})
	empty := decodeDocsWork[docsExcludeEnvelope](t, result)
	if !result.IsError || empty.OK || empty.Error == nil || empty.Error.Code != project.DocCodeInvalidInput {
		t.Fatalf("exclude_add ohne Muster: %+v", empty)
	}
	if !strings.Contains(string(after), "docs:") {
		t.Errorf("Block fehlt:\n%s", after)
	}
}

// exclude_add mit Zeilenumbruch im Muster: verworfen unter rejected, die
// Datei trägt außerhalb des Blocks byte-gleich nur das gültige Muster dazu,
// und die gemeldete Liste ist die, die die Kontextausgabe danach liest.
func TestDocsExcludeAddToolVerwirftZeilenumbruch(t *testing.T) {
	root := t.TempDir()
	content := "schema_version: 3\ndocs:\n  exclude:\n    - keep/\ntail: x\n"
	if err := os.MkdirAll(filepath.Dir(project.ConfigPath(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(project.ConfigPath(root), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	result, _, _ := docsExcludeAddTool(context.Background(), nil,
		docsExcludeAddInput{ProjectDir: root, Patterns: []string{"a\nd", "neu/"}})
	added := decodeDocsWork[docsExcludeEnvelope](t, result)
	if result.IsError || !added.OK || added.DocsExcludeResult == nil || !added.Changed ||
		strings.Join(added.Added, ",") != "neu/" || strings.Join(added.Exclude, ",") != "keep/,neu/" ||
		len(added.Rejected) != 1 || added.Rejected[0].Pattern != "a\nd" || !strings.Contains(added.Rejected[0].Reason, "Steuerzeichen") {
		t.Fatalf("exclude_add: %+v", added)
	}
	after, err := os.ReadFile(project.ConfigPath(root))
	if want := "schema_version: 3\ndocs:\n  exclude:\n    - keep/\n    - neu/\ntail: x\n"; err != nil || string(after) != want {
		t.Fatalf("Datei danach: %v\n%q\nerwartet:\n%q", err, after, want)
	}
	built, err := project.BuildContext(root)
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}
	if strings.Join(built.Docs.Exclude, ",") != strings.Join(added.Exclude, ",") || len(built.Docs.Problems) != 0 {
		t.Errorf("context.docs = %+v; gemeldet %q", built.Docs, added.Exclude)
	}
}

// repoDocsTemplates ist das defaults/ des Repos als mitgelieferte Ebene.
func repoDocsTemplates(t *testing.T) project.DocSource {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("Ort der Testdatei unbekannt")
	}
	store, err := kephtest.Defaults(filepath.Join(filepath.Dir(file), "..", "..", "..", "defaults"))
	if err != nil {
		t.Fatalf("defaults/ des Repos laden: %v", err)
	}
	return project.NewKephDefaultsSource(func() (project.DefaultsStore, string, error) { return store, "test:k-playbook", nil })
}

// k_playbook_docs_readme: check meldet ohne zu schreiben, ohne pfad alle,
// mit pfad die eine; ein ungültiger pfad und ein Projekt ohne Anbindung sind
// Fehler mit Code.
func TestDocsReadmeTool(t *testing.T) {
	root := t.TempDir()
	if err := project.CreateConfig(root, "."); err != nil {
		t.Fatal(err)
	}
	store := kephtest.New()
	templates := repoDocsTemplates(t)
	original := openDocsReadmes
	openDocsReadmes = func(string) (*project.DocsReadmes, error) { return project.NewDocsReadmes(store, templates), nil }
	t.Cleanup(func() { openDocsReadmes = original })
	ctx := context.Background()

	result, _, _ := docsReadmeTool(ctx, nil, docsReadmeInput{ProjectDir: root, Check: true})
	envelope := decodeDocsWork[docsReadmeEnvelope](t, result)
	if result.IsError || !envelope.OK || envelope.Tool != docsToolReadme || envelope.DocsReadmeResult == nil || !envelope.Check ||
		len(envelope.Readmes) != 7 || envelope.Readmes[0].Status != project.DocsReadmeMissing || len(store.Names()) != 0 {
		t.Fatalf("check: %+v", envelope)
	}
	result, _, _ = docsReadmeTool(ctx, nil, docsReadmeInput{ProjectDir: root, Path: "generated/code"})
	envelope = decodeDocsWork[docsReadmeEnvelope](t, result)
	if result.IsError || len(envelope.Readmes) != 1 || envelope.Readmes[0].Status != project.DocsCreated {
		t.Fatalf("eine: %+v", envelope)
	}
	result, _, _ = docsReadmeTool(ctx, nil, docsReadmeInput{ProjectDir: root})
	envelope = decodeDocsWork[docsReadmeEnvelope](t, result)
	if result.IsError || len(envelope.Readmes) != 7 || len(store.Names()) != 7 {
		t.Fatalf("alle: %+v, %v", envelope, store.Names())
	}

	result, _, _ = docsReadmeTool(ctx, nil, docsReadmeInput{ProjectDir: root, Path: "keph:/docs/README.md"})
	if envelope := decodeDocsWork[docsReadmeEnvelope](t, result); !result.IsError || envelope.Error.Code != project.DocCodeInvalidInput {
		t.Errorf("docs/README.md: %+v", envelope)
	}
	// Ohne Anbindung im Projekt: der echte Zugang meldet kephalaion_unavailable,
	// ohne den Node zu fragen.
	openDocsReadmes = project.OpenDocsReadmes
	result, _, _ = docsReadmeTool(ctx, nil, docsReadmeInput{ProjectDir: root})
	if envelope := decodeDocsWork[docsReadmeEnvelope](t, result); !result.IsError || envelope.Error.Code != project.DocCodeKephalaionUnavailable {
		t.Errorf("ohne Anbindung: %+v", envelope)
	}
}
