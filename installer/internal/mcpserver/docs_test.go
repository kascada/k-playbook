package mcpserver

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// newDocProject legt ein Projekt mit beiden Ebenen unter instructions/ an:
// mitgeliefert in k-playbook/, lokal in k-playbook-local/.
func newDocProject(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	if err := project.CreateConfig(root, "."); err != nil {
		t.Fatalf("Konfiguration anlegen: %v", err)
	}
	files := map[string]string{
		"k-playbook/instructions/always/010-a.md":           "---\ntitle: A\n---\n\n## A\n",
		"k-playbook/instructions/always/100-b.md":           "## B mitgeliefert\n",
		"k-playbook/instructions/always/200-c.md":           "## C\n",
		"k-playbook/instructions/always/README.md":          "# Liesmich\n\nText.\n",
		"k-playbook/instructions/on-demand/thema.md":        "---\ntitle: Thema\ndescription: D\n---\n\n## Thema\n",
		"k-playbook/instructions/on-demand/aus.md":          "## Aus\n",
		"k-playbook-local/instructions/always/100-b.md":     "## B lokal\n\nText.\n",
		"k-playbook-local/instructions/always/200-c.md":     "# Abgeschaltet\n",
		"k-playbook-local/instructions/always/notiz.md":     "## Notiz\n\nText.\n",
		"k-playbook-local/instructions/always/1000-eins.md": "## Eigen\n\nText.\n",
		"k-playbook-local/instructions/on-demand/aus.md":    "",
	}
	for rel, content := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("%s anlegen: %v", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("%s schreiben: %v", rel, err)
		}
	}
	return root
}

func callDocRead(t *testing.T, projectDir string, ref string) (docEnvelope, *mcp.CallToolResult) {
	t.Helper()

	result, _, err := docReadTool(context.Background(), nil, docReadInput{ProjectDir: projectDir, Ref: ref})
	if err != nil {
		t.Fatalf("doc_read: %v", err)
	}
	if len(result.Content) != 1 {
		t.Fatalf("erwartet genau einen Inhaltsblock, bekommen %d", len(result.Content))
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("Inhalt ist kein Text: %#v", result.Content[0])
	}
	envelope := docEnvelope{}
	if err := json.Unmarshal([]byte(text.Text), &envelope); err != nil {
		t.Fatalf("Antwort ist kein JSON: %v — %s", err, text.Text)
	}
	return envelope, result
}

func TestDocReadBautVerzeichnisZusammen(t *testing.T) {
	root := newDocProject(t)

	envelope, result := callDocRead(t, root, "k-playbook:/instructions/always/")
	if result.IsError || !envelope.OK || envelope.Tool != docToolRead || envelope.ProjectDir != root {
		t.Fatalf("Umschlag: %#v", envelope)
	}
	if envelope.Directory == nil || !*envelope.Directory {
		t.Errorf("directory fehlt: %#v", envelope)
	}
	if envelope.Content == nil || *envelope.Content != "## A\n\n## B lokal\n\nText.\n\n## Eigen\n\nText.\n" {
		t.Errorf("content: %q", deref(envelope.Content))
	}
	if envelope.Disabled == nil || *envelope.Disabled {
		t.Errorf("disabled beim Verzeichnis: %v", envelope.Disabled)
	}
	want := []project.DocPart{
		{Name: "010-a.md", Origin: "dist"},
		{Name: "100-b.md", Origin: "override"},
		{Name: "200-c.md", Origin: "override", Disabled: true},
		{Name: "1000-eins.md", Origin: "local"},
	}
	if envelope.Parts == nil || !reflect.DeepEqual(*envelope.Parts, want) {
		t.Errorf("parts: %+v", envelope.Parts)
	}
	if envelope.Problems == nil || len(*envelope.Problems) != 1 || !strings.Contains((*envelope.Problems)[0], "notiz.md") {
		t.Errorf("problems: %v", envelope.Problems)
	}
}

func TestDocReadEinzeldatei(t *testing.T) {
	root := newDocProject(t)

	envelope, _ := callDocRead(t, root, "k-playbook:/instructions/on-demand/thema.md")
	if !envelope.OK || deref(envelope.Content) != "---\ntitle: Thema\ndescription: D\n---\n\n## Thema\n" {
		t.Fatalf("Einzeldatei: %#v", envelope)
	}
	if envelope.Directory == nil || *envelope.Directory {
		t.Errorf("directory bei Einzeldatei: %v", envelope.Directory)
	}

	// Abgeschaltet: kein Fehler, leerer Inhalt, disabled mit Hinweis.
	envelope, result := callDocRead(t, root, "k-playbook:/instructions/on-demand/aus.md")
	if result.IsError || !envelope.OK {
		t.Fatalf("abgeschaltete Datei als Fehler: %#v", envelope)
	}
	if envelope.Content == nil || *envelope.Content != "" || envelope.Disabled == nil || !*envelope.Disabled || envelope.Hint == "" {
		t.Errorf("abgeschaltete Datei: %#v", envelope)
	}
}

func TestDocReadFehlercodes(t *testing.T) {
	root := newDocProject(t)

	for ref, code := range map[string]string{
		"k-playbook:/instructions/on-demand/fehlt.md": project.DocCodeNotFound,
		// Datei wie ein Verzeichnis behandelt, mitgeliefert und lokal.
		"k-playbook:/instructions/always/010-a.md/x.md": project.DocCodeNotFound,
		"k-playbook:/instructions/always/100-b.md/x.md": project.DocCodeNotFound,
		// Mit „/" am Ende fehlt der Dateiname: nicht in der Liste der
		// zusammengebauten Verzeichnisse.
		"k-playbook:/instructions/always/010-a.md/":     project.DocCodeInvalidInput,
		"k-playbook:/instructions/always/1000-eins.md/": project.DocCodeInvalidInput,
		"k-playbook:/instructions/on-demand/":           project.DocCodeInvalidInput,
		"project:/scripts/123.sh":                       project.DocCodeUnsupported,
		"keph:/manual/abc.md":                           project.DocCodeUnsupported,
		"k-playbook:/rules/befunde.md":                  project.DocCodeUnsupported,
		"k-playbook:/instructions/../rules/x.md":        project.DocCodeInvalidInput,
		"instructions/always/":                          project.DocCodeInvalidInput,
		"":                                              project.DocCodeInvalidInput,
	} {
		envelope, result := callDocRead(t, root, ref)
		if !result.IsError || envelope.OK || envelope.Error == nil || envelope.Error.Code != code {
			t.Errorf("%q: erwartet %s, bekommen %#v", ref, code, envelope)
			continue
		}
		if envelope.Content != nil || envelope.Parts != nil {
			t.Errorf("%q: Fehlerantwort trägt Inhalt: %#v", ref, envelope)
		}
	}

	envelope, _ := callDocRead(t, root, "k-playbook:/instructions/on-demand/")
	if envelope.Error == nil || !strings.Contains(envelope.Error.Message, "Dateiname fehlt") {
		t.Errorf("on-demand/: Meldung ohne „Dateiname fehlt“: %#v", envelope.Error)
	}

	envelope, _ = callDocRead(t, "", "k-playbook:/instructions/always/")
	if envelope.Error == nil || envelope.Error.Code != "invalid_input" {
		t.Errorf("ohne projectDir: %#v", envelope)
	}
	envelope, _ = callDocRead(t, t.TempDir(), "k-playbook:/instructions/always/")
	if envelope.Error == nil || envelope.Error.Code != "project_not_found" {
		t.Errorf("ohne Projekt: %#v", envelope)
	}
}

func deref(value *string) string {
	if value == nil {
		return "<nil>"
	}
	return *value
}
