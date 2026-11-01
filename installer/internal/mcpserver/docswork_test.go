package mcpserver

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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
	original := openDocsWork
	openDocsWork = func(projectDir string) (*project.DocsWork, error) {
		return project.NewDocsWork(projectDir, store, "vm:projekt"), nil
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
	result, _, _ = docsCheckoutTool(ctx, nil, docsCheckoutInput{ProjectDir: root, Producer: "libs"})
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
		if !strings.Contains(string(data), `"erzeuger":{"type":"string","description":`) && !strings.Contains(string(data), `"enum":["code"]`) {
			t.Errorf("Schema ohne Erzeuger: %s", data)
		}
		if !strings.Contains(string(data), `"enum":["code"]`) {
			t.Errorf("Schema ohne enum: %s", data)
		}
	}
}
