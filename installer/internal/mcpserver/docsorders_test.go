package mcpserver

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/kephalaion/kephtest"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// newDocsOrdersProject legt ein Projekt an, dessen Collection im Speicher
// liegt.
func newDocsOrdersProject(t *testing.T) (string, *kephtest.Store) {
	t.Helper()
	root := t.TempDir()
	if err := project.CreateConfig(root, "."); err != nil {
		t.Fatalf("Konfiguration anlegen: %v", err)
	}
	store := kephtest.New()
	original := openDocsOrders
	openDocsOrders = func(string) (*project.DocsOrders, error) { return project.NewDocsOrders(store, "kamran") }
	t.Cleanup(func() { openDocsOrders = original })
	return root, store
}

const mcpRawOrder = "---\ntitle: \"Nachzug\"\nerzeuger: code\npfade:\n  - installer/a\nanlass: \"Anlass\"\nsitzung: s1\nentstanden: 2026-11-20\n---\n"

// docs_scope, docs_orders, docs_order_add und docs_orders_split über MCP: die
// Antwort trägt die Felder der Fachlogik, Fehler tragen ihren Code.
func TestDocsOrderToolsAblauf(t *testing.T) {
	root, store := newDocsOrdersProject(t)
	ctx := context.Background()
	store.Put("queue/docs/001-a.md", mcpRawOrder)

	result, _, _ := docsOrdersTool(ctx, nil, docsOrdersInput{ProjectDir: root, Producer: "code"})
	orders := decodeDocsWork[docsOrdersEnvelope](t, result)
	if result.IsError || !orders.OK || orders.Tool != docsToolOrders || orders.DocsOrdersResult == nil || orders.Run != project.DocsRunPartial ||
		len(orders.RawOrders) != 1 || orders.RawOrders[0].Refs[0] != "keph:/queue/docs/001-a.md" {
		t.Fatalf("orders: %+v", orders)
	}
	result, _, _ = docsOrdersTool(ctx, nil, docsOrdersInput{ProjectDir: root, Producer: "libs"})
	if bad := decodeDocsWork[docsOrdersEnvelope](t, result); !result.IsError || bad.Error == nil || bad.Error.Code != project.DocCodeInvalidInput {
		t.Errorf("unbekannter Erzeuger: %+v", bad)
	}

	store.Put("docs/generated/code/01-a.md", "x")
	add := docsOrderAddInput{ProjectDir: root, Producer: "code", Paths: []string{"installer/b"}, Anlass: "Neu", Session: "S1"}
	result, _, _ = docsOrderAddTool(ctx, nil, add)
	created := decodeDocsWork[docsOrderAddEnvelope](t, result)
	if result.IsError || !created.OK || created.DocsOrderAddResult == nil || created.Status != project.DocsOrderCreated || created.Tool != docsToolOrderAdd {
		t.Fatalf("order_add: %+v", created)
	}
	result, _, _ = docsOrderAddTool(ctx, nil, add)
	if again := decodeDocsWork[docsOrderAddEnvelope](t, result); again.Status != project.DocsOrderCovered || again.Ref != created.Ref {
		t.Errorf("order_add gedeckt: %+v", again)
	}
	add.Session = ""
	result, _, _ = docsOrderAddTool(ctx, nil, add)
	if bad := decodeDocsWork[docsOrderAddEnvelope](t, result); !result.IsError || bad.Error == nil || bad.Error.Code != project.DocCodeInvalidInput {
		t.Errorf("order_add ohne Sitzung: %+v", bad)
	}

	result, _, _ = docsOrdersSplitTool(ctx, nil, docsOrdersSplitInput{ProjectDir: root, Session: "V1",
		Documents: []project.DocsSplitDocument{{Dokument: "01-a.md", Aktion: "umschreiben", Pfade: []string{"installer/a"},
			Anlaesse: []project.DocsOrderLine{{Datum: "2026-11-20", Anlass: "Anlass"}}}},
		Raw: []string{"keph:/queue/docs/001-a.md"}})
	split := decodeDocsWork[docsOrdersSplitEnvelope](t, result)
	if result.IsError || !split.OK || split.DocsSplitResult == nil || len(split.Documents) != 1 || split.Documents[0].Status != project.DocsOrderCreated ||
		len(split.ClosedRaw) != 1 {
		t.Fatalf("orders_split: %+v", split)
	}
	if _, ok := store.Content("queue/docs/done/001-a.md"); !ok {
		t.Errorf("roher Auftrag nicht in done/: %v", store.Names())
	}
	result, _, _ = docsOrdersSplitTool(ctx, nil, docsOrdersSplitInput{ProjectDir: root, Session: "V1",
		Documents: []project.DocsSplitDocument{{Dokument: "README.md", Aktion: "neu", Pfade: []string{"a"}, Anlass: "x"}}})
	if bad := decodeDocsWork[docsOrdersSplitEnvelope](t, result); !result.IsError || bad.Error == nil || bad.Error.Code != project.DocCodeInvalidInput {
		t.Errorf("orders_split ungültig: %+v", bad)
	}
}

// Ohne bereite Anbindung: orders und orders_split melden kephalaion_unavailable,
// order_add reiht nicht ein und meldet skipped.
func TestDocsOrderToolsOhneKephalaion(t *testing.T) {
	root, _ := newDocsOrdersProject(t)
	openDocsOrders = func(string) (*project.DocsOrders, error) {
		return nil, project.DocsOrderOpenError("Kephalaion ist nicht bereit")
	}
	ctx := context.Background()
	result, _, _ := docsOrdersTool(ctx, nil, docsOrdersInput{ProjectDir: root, Producer: "code"})
	if bad := decodeDocsWork[docsOrdersEnvelope](t, result); !result.IsError || bad.Error == nil || bad.Error.Code != project.DocCodeKephalaionUnavailable {
		t.Errorf("orders: %+v", bad)
	}
	result, _, _ = docsOrdersSplitTool(ctx, nil, docsOrdersSplitInput{ProjectDir: root, Session: "V"})
	if bad := decodeDocsWork[docsOrdersSplitEnvelope](t, result); !result.IsError || bad.Error == nil || bad.Error.Code != project.DocCodeKephalaionUnavailable {
		t.Errorf("orders_split: %+v", bad)
	}
	result, _, _ = docsOrderAddTool(ctx, nil, docsOrderAddInput{ProjectDir: root, Producer: "code", Paths: []string{"a"}, Anlass: "x", Session: "S"})
	skipped := decodeDocsWork[docsOrderAddEnvelope](t, result)
	if result.IsError || !skipped.OK || skipped.Status != project.DocsOrderSkipped || skipped.Reason != project.DocsReasonKephalaionUnavailable {
		t.Errorf("order_add: %+v", skipped)
	}
}

// docs_scope über MCP: Scope, Zusatzpfad und check; ein ungültiger Pfad ist
// invalid_input.
func TestDocsScopeTool(t *testing.T) {
	root, _ := newDocsOrdersProject(t)
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	result, _, _ := docsScopeTool(ctx, nil, docsScopeInput{ProjectDir: root, Path: "src", Check: []string{"src/a.go", "node_modules/x", ".env"}})
	scope := decodeDocsWork[docsScopeEnvelope](t, result)
	if result.IsError || !scope.OK || scope.Tool != docsToolScope || scope.DocsScopeResult == nil || scope.Extra == nil || scope.Extra.Path != "src" ||
		len(scope.Checks) != 3 || scope.Checks[0].Excluded || scope.Checks[1].Reason != "further" || scope.Checks[2].Reason != "hidden" {
		t.Fatalf("scope: %+v", scope)
	}
	if !strings.HasSuffix(scope.ScanRoot, filepath.Base(root)) || len(scope.Exclude.Always) != 3 {
		t.Errorf("scope: %+v", scope.Exclude)
	}
	result, _, _ = docsScopeTool(ctx, nil, docsScopeInput{ProjectDir: root, Path: ".k-playbook"})
	if bad := decodeDocsWork[docsScopeEnvelope](t, result); !result.IsError || bad.Error == nil || bad.Error.Code != project.DocCodeInvalidInput {
		t.Errorf("immer ausgeschlossen: %+v", bad)
	}
}
