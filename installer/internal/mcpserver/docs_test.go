package mcpserver

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
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
		"k-playbook/defaults/instructions/always/010-a.md":    "---\ntitle: A\n---\n\n## A\n",
		"k-playbook/defaults/instructions/always/100-b.md":    "## B mitgeliefert\n",
		"k-playbook/defaults/instructions/always/200-c.md":    "## C\n",
		"k-playbook/defaults/instructions/always/README.md":   "# Liesmich\n\nText.\n",
		"k-playbook/defaults/instructions/on-demand/thema.md": "---\ntitle: Thema\ndescription: D\n---\n\n## Thema\n",
		"k-playbook/defaults/instructions/on-demand/aus.md":   "## Aus\n",
		"k-playbook-local/instructions/always/100-b.md":       "## B lokal\n\nText.\n",
		"k-playbook-local/instructions/always/200-c.md":       "# Abgeschaltet\n",
		"k-playbook-local/instructions/always/notiz.md":       "## Notiz\n\nText.\n",
		"k-playbook-local/instructions/always/1000-eins.md":   "## Eigen\n\nText.\n",
		"k-playbook-local/instructions/on-demand/aus.md":      "",
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
		"project:/scripts/123.sh":                       project.DocCodeNotFound,
		"project:/k-playbook/":                          project.DocCodeInvalidInput,
		// Ohne Anbindung des Projekts; der Node wird nicht gefragt.
		"keph:/manual/abc.md":                    project.DocCodeKephalaionUnavailable,
		"keph:/manual/":                          project.DocCodeInvalidInput,
		"k-playbook:/checks/check_x.sh":          project.DocCodeUnsupported,
		"k-playbook:/rules/":                     project.DocCodeInvalidInput,
		"k-playbook:/rules/fehlt.md":             project.DocCodeNotFound,
		"k-playbook:/instructions/../rules/x.md": project.DocCodeInvalidInput,
		"instructions/always/":                   project.DocCodeInvalidInput,
		"":                                       project.DocCodeInvalidInput,
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

// memDocStore ist ein Kephalaion im Speicher für die Hüllen-Tests.
type memDocStore struct {
	docs map[string]string
	// writeFail lässt create und rename mit diesem Fehler scheitern.
	writeFail error
}

func (s *memDocStore) List(_ context.Context, req kephalaion.ListRequest) ([]kephalaion.Entry, error) {
	var out []kephalaion.Entry
	for name, by := range s.docs {
		if strings.HasPrefix(name, req.Path+"/") {
			out = append(out, kephalaion.Entry{Kind: "document", Name: name,
				Created: &kephalaion.Stamp{At: "2026-10-14T10:00:00.000Z", By: by}})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (s *memDocStore) Create(_ context.Context, name, _ string) (kephalaion.Document, error) {
	if s.writeFail != nil {
		return kephalaion.Document{}, s.writeFail
	}
	if _, ok := s.docs[name]; ok {
		return kephalaion.Document{}, &kephalaion.ToolError{Tool: "create", Code: kephalaion.CodeNameTaken, Message: "belegt"}
	}
	s.docs[name] = "kamran"
	return kephalaion.Document{Name: name}, nil
}

func (s *memDocStore) Rename(_ context.Context, name, newName string) (kephalaion.Document, error) {
	if s.writeFail != nil {
		return kephalaion.Document{}, s.writeFail
	}
	by, ok := s.docs[name]
	if !ok {
		return kephalaion.Document{}, &kephalaion.ToolError{Tool: "rename", Code: kephalaion.CodeNotFound, Message: "fehlt"}
	}
	if _, ok := s.docs[newName]; ok {
		return kephalaion.Document{}, &kephalaion.ToolError{Tool: "rename", Code: kephalaion.CodeNameTaken, Message: "belegt"}
	}
	delete(s.docs, name)
	s.docs[newName] = by
	return kephalaion.Document{Name: newName}, nil
}

// useMemDocs ersetzt den Zugang zu Kephalaion durch einen im Speicher.
func useMemDocs(t *testing.T, names ...string) *memDocStore {
	t.Helper()
	store := &memDocStore{docs: map[string]string{}}
	for _, name := range names {
		store.docs[name] = "anna"
	}
	original := openUserDocs
	openUserDocs = func(string) (*project.UserDocs, error) {
		return project.NewUserDocs(project.DocKinds, store, "kamran")
	}
	t.Cleanup(func() { openUserDocs = original })
	return store
}

func decodeEnvelope(t *testing.T, result *mcp.CallToolResult, target any) {
	t.Helper()
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("Inhalt ist kein Text: %#v", result.Content[0])
	}
	if err := json.Unmarshal([]byte(text.Text), target); err != nil {
		t.Fatalf("Antwort ist kein JSON: %v — %s", err, text.Text)
	}
}

func TestDocCreateListDoneReopenUeberHuellen(t *testing.T) {
	root := newDocProject(t)
	useMemDocs(t, "user/anna/tasks/104-fremd.md", "queue/findings/007-alt.md")
	ctx := context.Background()

	result, _, _ := docCreateTool(ctx, nil, docCreateInput{ProjectDir: root, Kind: "tasks", Name: "Neue Aufgabe"})
	var created struct {
		OK     bool   `json:"ok"`
		Tool   string `json:"tool"`
		Ref    string `json:"ref"`
		Number int    `json:"number"`
		User   string `json:"user"`
	}
	decodeEnvelope(t, result, &created)
	if result.IsError || !created.OK || created.Tool != docToolCreate || created.Ref != "keph:/user/kamran/tasks/105-neue-aufgabe.md" ||
		created.Number != 105 || created.User != "kamran" {
		t.Fatalf("create: %+v", created)
	}

	result, _, _ = docListTool(ctx, nil, docListInput{ProjectDir: root, Kind: "tasks", User: "*"})
	var list struct {
		OK    bool                  `json:"ok"`
		User  string                `json:"user"`
		Items []project.DocListItem `json:"items"`
	}
	decodeEnvelope(t, result, &list)
	if !list.OK || list.User != "*" || len(list.Items) != 2 {
		t.Fatalf("list: %+v", list)
	}

	result, _, _ = docListTool(ctx, nil, docListInput{ProjectDir: root, Kind: "findings"})
	decodeEnvelope(t, result, &list)
	if len(list.Items) != 1 || list.Items[0].User != "anna" {
		t.Fatalf("findings: %+v", list)
	}

	result, _, _ = docDoneTool(ctx, nil, docMoveInput{ProjectDir: root, Ref: "keph:/user/~/tasks/105-neue-aufgabe.md"})
	var moved struct {
		OK   bool   `json:"ok"`
		Tool string `json:"tool"`
		Ref  string `json:"ref"`
		From string `json:"from"`
	}
	decodeEnvelope(t, result, &moved)
	if result.IsError || moved.Tool != docToolDone || moved.Ref != "keph:/user/kamran/tasks/done/105-neue-aufgabe.md" {
		t.Fatalf("done: %+v", moved)
	}
	result, _, _ = docReopenTool(ctx, nil, docMoveInput{ProjectDir: root, Ref: moved.Ref})
	decodeEnvelope(t, result, &moved)
	if result.IsError || moved.Tool != docToolReopen || moved.Ref != "keph:/user/kamran/tasks/105-neue-aufgabe.md" {
		t.Fatalf("reopen: %+v", moved)
	}
}

func TestDocHuellenFehlercodes(t *testing.T) {
	root := newDocProject(t)
	useMemDocs(t, "user/anna/tasks/104-fremd.md", "queue/findings/007-alt.md")
	ctx := context.Background()

	cases := []struct {
		name   string
		result func() *mcp.CallToolResult
		code   string
	}{
		{"fremder User", func() *mcp.CallToolResult {
			r, _, _ := docDoneTool(ctx, nil, docMoveInput{ProjectDir: root, Ref: "keph:/user/anna/tasks/104-fremd.md"})
			return r
		}, project.DocCodeForeignUser},
		{"ohne done/", func() *mcp.CallToolResult {
			r, _, _ := docDoneTool(ctx, nil, docMoveInput{ProjectDir: root, Ref: "keph:/queue/findings/007-alt.md"})
			return r
		}, project.DocCodeDoneUnsupported},
		{"reopen außerhalb done/", func() *mcp.CallToolResult {
			r, _, _ := docReopenTool(ctx, nil, docMoveInput{ProjectDir: root, Ref: "keph:/user/~/tasks/100-x.md"})
			return r
		}, project.DocCodeWrongDir},
		{"ungültiger Name", func() *mcp.CallToolResult {
			r, _, _ := docCreateTool(ctx, nil, docCreateInput{ProjectDir: root, Kind: "tasks", Name: "!!"})
			return r
		}, project.DocCodeInvalidInput},
		{"ohne projectDir", func() *mcp.CallToolResult {
			r, _, _ := docListTool(ctx, nil, docListInput{Kind: "tasks"})
			return r
		}, "invalid_input"},
	}
	for _, c := range cases {
		result := c.result()
		var envelope struct {
			OK    bool                    `json:"ok"`
			Ref   string                  `json:"ref"`
			Error *knowledgeErrorEnvelope `json:"error"`
		}
		decodeEnvelope(t, result, &envelope)
		if !result.IsError || envelope.OK || envelope.Error == nil || envelope.Error.Code != c.code || envelope.Ref != "" {
			t.Errorf("%s: erwartet %s, bekommen %+v", c.name, c.code, envelope)
		}
	}
}

func TestDocHuellenOffenerAusgangUndNodeCodes(t *testing.T) {
	root := newDocProject(t)
	store := useMemDocs(t, "user/kamran/tasks/100-a.md")
	ctx := context.Background()
	cases := map[string]string{
		kephalaion.CodeOutcomeUnknown:  project.DocCodeOutcomeUnknown,
		kephalaion.CodeUnreachable:     project.DocCodeKephalaionUnavailable,
		kephalaion.CodeUnsupported:     project.DocCodeKephalaionUnavailable,
		kephalaion.CodeInternal:        project.DocCodeKephalaionUnavailable,
		kephalaion.CodeNodeUnreachable: project.DocCodeKephalaionUnavailable,
	}
	for code, want := range cases {
		store.writeFail = &kephalaion.ToolError{Tool: "create", Code: code, Message: "Meldung"}
		r1, _, _ := docCreateTool(ctx, nil, docCreateInput{ProjectDir: root, Kind: "tasks", Name: "b"})
		r2, _, _ := docDoneTool(ctx, nil, docMoveInput{ProjectDir: root, Ref: "keph:/user/~/tasks/100-a.md"})
		for i, result := range []*mcp.CallToolResult{r1, r2} {
			var envelope struct {
				OK    bool                    `json:"ok"`
				Error *knowledgeErrorEnvelope `json:"error"`
			}
			decodeEnvelope(t, result, &envelope)
			if !result.IsError || envelope.OK || envelope.Error == nil || envelope.Error.Code != want {
				t.Errorf("%s, Aufruf %d: erwartet %s, bekommen %+v", code, i, want, envelope.Error)
				continue
			}
			if want == project.DocCodeOutcomeUnknown && !strings.Contains(envelope.Error.Message, "k_playbook_doc_list") {
				t.Errorf("%s: Hinweis zum Nachsehen fehlt: %s", code, envelope.Error.Message)
			}
		}
	}
}

func TestDocHuellenOhneKephalaion(t *testing.T) {
	// Ein Projekt ohne Anbindung: der echte Zugang, kein Node wird gefragt.
	root := newDocProject(t)
	ctx := context.Background()
	results := []*mcp.CallToolResult{}
	r, _, _ := docCreateTool(ctx, nil, docCreateInput{ProjectDir: root, Kind: "tasks", Name: "a"})
	results = append(results, r)
	r, _, _ = docListTool(ctx, nil, docListInput{ProjectDir: root, Kind: "tasks"})
	results = append(results, r)
	r, _, _ = docDoneTool(ctx, nil, docMoveInput{ProjectDir: root, Ref: "keph:/user/~/tasks/100-a.md"})
	results = append(results, r)
	r, _, _ = docReopenTool(ctx, nil, docMoveInput{ProjectDir: root, Ref: "keph:/user/~/tasks/done/100-a.md"})
	results = append(results, r)
	for i, result := range results {
		var envelope struct {
			Error *knowledgeErrorEnvelope `json:"error"`
		}
		decodeEnvelope(t, result, &envelope)
		if !result.IsError || envelope.Error == nil || envelope.Error.Code != project.DocCodeKephalaionUnavailable {
			t.Errorf("Aufruf %d: %+v", i, envelope.Error)
		}
	}
}

func TestDocKindSchemaAusDerTabelle(t *testing.T) {
	for _, schema := range []*jsonschema.Schema{docKindSchema[docCreateInput](), docKindSchema[docListInput]()} {
		var got []string
		for _, value := range schema.Properties["kind"].Enum {
			got = append(got, value.(string))
		}
		if !reflect.DeepEqual(got, project.DocKindNames(project.DocKinds)) {
			t.Fatalf("enum %v", got)
		}
		for _, name := range got {
			if name == project.TodoKindName {
				t.Fatalf("todos steht im enum der doc-Werkzeuge: %v", got)
			}
		}
	}
	if _, ok := docKindSchema[docMoveInput]().Properties["kind"]; ok {
		t.Fatal("done/reopen haben kein kind")
	}
}

// Includes über die Hülle: eingesetzt, mit includes: false roh, Fehler mit
// Code und Kette, includes in der Antwort.
func TestDocReadIncludes(t *testing.T) {
	root := newDocProject(t)
	for rel, content := range map[string]string{
		"k-playbook/defaults/includes/context.md":          "## Kontext\n\nBaustein.\n",
		"k-playbook/defaults/commands/k-danke.md":          "# k-danke\n\ninclude: k-playbook:/includes/context.md\n",
		"k-playbook/defaults/commands/zyklus.md":           "include: zyklus.md\n",
		"k-playbook/defaults/instructions/always/300-d.md": "## D\n\ninclude: k-playbook:/includes/context.md\n",
	} {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	call := func(ref string, includes *bool) docEnvelope {
		t.Helper()
		result, _, err := docReadTool(context.Background(), nil, docReadInput{ProjectDir: root, Ref: ref, Includes: includes})
		if err != nil {
			t.Fatalf("doc_read: %v", err)
		}
		var envelope docEnvelope
		decodeEnvelope(t, result, &envelope)
		return envelope
	}
	no := false
	yes := true

	envelope := call("k-playbook:/commands/k-danke.md", nil)
	if !envelope.OK || deref(envelope.Content) != "# k-danke\n\n## Kontext\n\nBaustein.\n" {
		t.Fatalf("eingesetzt: %#v", envelope)
	}
	if envelope.Includes == nil || !reflect.DeepEqual(*envelope.Includes, []project.DocInclude{{Ref: "k-playbook:/includes/context.md", Origin: "dist", Depth: 1}}) {
		t.Errorf("includes: %+v", envelope.Includes)
	}
	if e := call("k-playbook:/commands/k-danke.md", &yes); deref(e.Content) != deref(envelope.Content) {
		t.Errorf("includes: true weicht vom Standard ab: %q", deref(e.Content))
	}
	rawEnvelope := call("k-playbook:/commands/k-danke.md", &no)
	if deref(rawEnvelope.Content) != "# k-danke\n\ninclude: k-playbook:/includes/context.md\n" || rawEnvelope.Includes == nil || len(*rawEnvelope.Includes) != 0 {
		t.Errorf("includes: false: %#v", rawEnvelope)
	}
	// Ohne Include ist includes [] — immer da.
	if e := call("k-playbook:/instructions/on-demand/thema.md", nil); e.Includes == nil || len(*e.Includes) != 0 {
		t.Errorf("includes ohne Include: %#v", e.Includes)
	}
	// Das Verzeichnis setzt je Teil ein.
	if e := call("k-playbook:/instructions/always/", nil); !strings.Contains(deref(e.Content), "## D\n\n## Kontext\n\nBaustein.\n") || len(*e.Includes) != 1 {
		t.Errorf("Verzeichnis: %#v", e)
	}
	errEnvelope := call("k-playbook:/commands/zyklus.md", nil)
	if errEnvelope.OK || errEnvelope.Error == nil || errEnvelope.Error.Code != project.DocCodeIncludeCycle ||
		!strings.Contains(errEnvelope.Error.Message, "Include-Kette: k-playbook:/commands/zyklus.md → k-playbook:/commands/zyklus.md") {
		t.Errorf("Zyklus: %#v", errEnvelope.Error)
	}
	if errEnvelope.Includes != nil || errEnvelope.Content != nil {
		t.Errorf("Fehlerantwort trägt Inhalt: %#v", errEnvelope)
	}
	if e := call("k-playbook:/commands/zyklus.md", &no); !e.OK {
		t.Errorf("Zyklus roh: %#v", e.Error)
	}
	// Die Sorten includes/ und testdata/ erscheinen nicht in der Beschreibung.
	for _, text := range []string{docReadSchemaText(t), docReadDescription} {
		if strings.Contains(text, "includes/") || strings.Contains(text, "testdata/") {
			t.Errorf("Lesersorten in der Werkzeugbeschreibung: %q", text)
		}
		if strings.Contains(text, "unsupported gemeldet") {
			t.Errorf("Beschreibung nennt project:/keph: noch als unsupported: %q", text)
		}
	}
}

func docReadSchemaText(t *testing.T) string {
	t.Helper()
	schema, err := jsonschema.For[docReadInput](nil)
	if err != nil {
		t.Fatal(err)
	}
	var text strings.Builder
	for _, prop := range schema.Properties {
		text.WriteString(prop.Description)
		text.WriteString("\n")
	}
	if _, ok := schema.Properties["includes"]; !ok {
		t.Error("Schema ohne includes")
	}
	return text.String()
}
