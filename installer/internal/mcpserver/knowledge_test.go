package mcpserver

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// newKnowledgeProject legt ein k-playbook-Projekt mit einem kleinen
// Wissensverzeichnis an: eine flache README (source root) und ein Dokument
// unter manual/. learned/ entsteht erst durch das Schreiben.
func newKnowledgeProject(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	if err := project.CreateConfig(root, "."); err != nil {
		t.Fatalf("Konfiguration anlegen: %v", err)
	}
	docs := project.KnowledgeDir(root)
	files := map[string]string{
		"README.md": "---\ntitle: Index\n---\n# Projektwissen\n\nEinstieg in die Doku.\n",
		"manual/ablauf.md": "# Ablauf\n\n## Freigabe\n\nDie Freigabe braucht ein Review und einen Wächter.\n" +
			"\n## Überschriften\n\nUmlaute im Anker.\n",
	}
	for rel, content := range files {
		full := filepath.Join(docs, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("%s anlegen: %v", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("%s schreiben: %v", rel, err)
		}
	}
	return root
}

func decodeKnowledgeEnvelope(t *testing.T, result *mcp.CallToolResult) knowledgeEnvelope {
	t.Helper()

	if len(result.Content) != 1 {
		t.Fatalf("erwartet genau einen Inhaltsblock, bekommen %d", len(result.Content))
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("Inhalt ist kein Text: %#v", result.Content[0])
	}
	envelope := knowledgeEnvelope{}
	if err := json.Unmarshal([]byte(text.Text), &envelope); err != nil {
		t.Fatalf("Antwort ist kein JSON: %v — %s", err, text.Text)
	}
	return envelope
}

func TestKnowledgeSearchLiefertVertragsfelder(t *testing.T) {
	root := newKnowledgeProject(t)

	result, _, err := knowledgeSearchTool(context.Background(), nil, knowledgeSearchInput{
		knowledgeBaseInput: knowledgeBaseInput{ProjectDir: root}, Query: "Wächter",
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	envelope := decodeKnowledgeEnvelope(t, result)
	if !envelope.OK || envelope.Tool != knowledgeToolSearch || envelope.ProjectDir != root {
		t.Fatalf("Umschlag: %#v", envelope)
	}
	if envelope.Query != "Wächter" {
		t.Errorf("Query = %q, erwartet Wächter", envelope.Query)
	}
	if len(knowledgeHitsOf(envelope)) != 1 {
		t.Fatalf("Treffer: %+v", knowledgeHitsOf(envelope))
	}
	hit := knowledgeHitsOf(envelope)[0]
	if hit.Path != "manual/ablauf.md" || hit.Heading != "Freigabe" || hit.Source != "manual" || hit.Rank != 1 || hit.Anchor != "freigabe" {
		t.Errorf("Treffer: %+v", hit)
	}
	if !strings.HasPrefix(hit.Excerpt, "Die Freigabe") {
		t.Errorf("Excerpt = %q, erwartet den Anfang des Chunks", hit.Excerpt)
	}

	// Der Herkunftsfilter grenzt ein: unter root gibt es keinen Wächter.
	result, _, err = knowledgeSearchTool(context.Background(), nil, knowledgeSearchInput{
		knowledgeBaseInput: knowledgeBaseInput{ProjectDir: root}, Query: "Wächter", Source: project.KnowledgeRootSource,
	})
	if err != nil {
		t.Fatalf("search source: %v", err)
	}
	if envelope = decodeKnowledgeEnvelope(t, result); !envelope.OK || len(knowledgeHitsOf(envelope)) != 0 || envelope.Source != "root" {
		t.Errorf("gefilterte Suche: %#v", envelope)
	}
}

func TestKnowledgeSearchLehntLeereAnfrageAb(t *testing.T) {
	root := newKnowledgeProject(t)

	result, _, err := knowledgeSearchTool(context.Background(), nil, knowledgeSearchInput{
		knowledgeBaseInput: knowledgeBaseInput{ProjectDir: root}, Query: "  ",
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	envelope := decodeKnowledgeEnvelope(t, result)
	if envelope.OK || envelope.Error == nil || envelope.Error.Code != "invalid_input" {
		t.Fatalf("leere Anfrage wurde angenommen: %#v", envelope)
	}
	if !result.IsError {
		t.Error("Ergebnis ist nicht als Werkzeugfehler markiert")
	}
}

func TestKnowledgeListNenntReadmeZuerst(t *testing.T) {
	root := newKnowledgeProject(t)

	result, _, err := knowledgeListTool(context.Background(), nil, knowledgeListInput{
		knowledgeBaseInput: knowledgeBaseInput{ProjectDir: root},
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	envelope := decodeKnowledgeEnvelope(t, result)
	if !envelope.OK || len(knowledgeEntriesOf(envelope)) != 2 {
		t.Fatalf("Einträge: %#v", envelope)
	}
	if knowledgeEntriesOf(envelope)[0].Path != "README.md" || knowledgeEntriesOf(envelope)[0].Source != project.KnowledgeRootSource || knowledgeEntriesOf(envelope)[0].Title != "Projektwissen" {
		t.Errorf("erster Eintrag: %+v", knowledgeEntriesOf(envelope)[0])
	}
	if knowledgeEntriesOf(envelope)[1].Path != "manual/ablauf.md" || knowledgeEntriesOf(envelope)[1].Source != "manual" || knowledgeEntriesOf(envelope)[1].Title != "Ablauf" {
		t.Errorf("zweiter Eintrag: %+v", knowledgeEntriesOf(envelope)[1])
	}

	result, _, err = knowledgeListTool(context.Background(), nil, knowledgeListInput{
		knowledgeBaseInput: knowledgeBaseInput{ProjectDir: root}, Source: "manual",
	})
	if err != nil {
		t.Fatalf("list source: %v", err)
	}
	if envelope = decodeKnowledgeEnvelope(t, result); len(knowledgeEntriesOf(envelope)) != 1 || knowledgeEntriesOf(envelope)[0].Source != "manual" {
		t.Errorf("gefilterte Liste: %#v", knowledgeEntriesOf(envelope))
	}
}

func TestKnowledgeReadLiefertMarkdown(t *testing.T) {
	root := newKnowledgeProject(t)

	result, _, err := knowledgeReadTool(context.Background(), nil, knowledgeReadInput{
		knowledgeBaseInput: knowledgeBaseInput{ProjectDir: root}, Path: "manual/ablauf.md",
	})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	envelope := decodeKnowledgeEnvelope(t, result)
	if !envelope.OK || envelope.Path != "manual/ablauf.md" || envelope.Content == nil {
		t.Fatalf("Umschlag: %#v", envelope)
	}
	if !strings.HasPrefix(*envelope.Content, "# Ablauf") {
		t.Errorf("Content = %q, erwartet Markdown", *envelope.Content)
	}
}

func TestKnowledgeReadWehrtPfadeAb(t *testing.T) {
	root := newKnowledgeProject(t)
	outside := filepath.Join(root, "k-playbook-local", "geheim.md")
	if err := os.WriteFile(outside, []byte("# Geheim\n"), 0o644); err != nil {
		t.Fatalf("Datei außerhalb anlegen: %v", err)
	}

	for _, path := range []string{"", "../geheim.md", outside, "manual/ablauf.txt"} {
		result, _, err := knowledgeReadTool(context.Background(), nil, knowledgeReadInput{
			knowledgeBaseInput: knowledgeBaseInput{ProjectDir: root}, Path: path,
		})
		if err != nil {
			t.Fatalf("read %q: %v", path, err)
		}
		envelope := decodeKnowledgeEnvelope(t, result)
		if envelope.OK || envelope.Error == nil || envelope.Error.Code != "invalid_input" || !result.IsError {
			t.Errorf("Pfad %q wurde angenommen: %#v", path, envelope)
		}
	}
}

func TestKnowledgeWriteSchreibtNurNachLearned(t *testing.T) {
	root := newKnowledgeProject(t)

	result, _, err := knowledgeWriteTool(context.Background(), nil, knowledgeWriteInput{
		knowledgeBaseInput: knowledgeBaseInput{ProjectDir: root},
		Path:               "sitzung.md",
		Content:            "# Gelernt\n\n## Erkenntnis\n\nDer Wächter verlangt ein Release.\n",
		Source:             "Task 055",
	})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	written := decodeKnowledgeEnvelope(t, result)
	if !written.OK || !written.Written || written.Path != "learned/sitzung.md" || written.Source != "Task 055" {
		t.Fatalf("Umschlag: %#v", written)
	}
	if _, err := os.Stat(filepath.Join(project.KnowledgeLearnedDir(root), "sitzung.md")); err != nil {
		t.Fatalf("Datei fehlt unter learned/: %v", err)
	}

	// Lesen unter dem gemeldeten Pfad, mit Herkunftsvermerk im Frontmatter.
	result, _, err = knowledgeReadTool(context.Background(), nil, knowledgeReadInput{
		knowledgeBaseInput: knowledgeBaseInput{ProjectDir: root}, Path: written.Path,
	})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	read := decodeKnowledgeEnvelope(t, result)
	if !read.OK || read.Content == nil || !strings.Contains(*read.Content, "source: Task 055") {
		t.Fatalf("gelesener Inhalt: %#v", read)
	}

	// Suchen findet das neue Dokument unter der Herkunft learned.
	result, _, err = knowledgeSearchTool(context.Background(), nil, knowledgeSearchInput{
		knowledgeBaseInput: knowledgeBaseInput{ProjectDir: root}, Query: "Release", Source: project.KnowledgeLearnedDirName,
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	searched := decodeKnowledgeEnvelope(t, result)
	if len(knowledgeHitsOf(searched)) != 1 || knowledgeHitsOf(searched)[0].Path != "learned/sitzung.md" || knowledgeHitsOf(searched)[0].Heading != "Erkenntnis" {
		t.Fatalf("Treffer nach dem Schreiben: %+v", knowledgeHitsOf(searched))
	}

	// Auflisten kennt drei Dateien, Status zählt learned mit.
	result, _, err = knowledgeListTool(context.Background(), nil, knowledgeListInput{
		knowledgeBaseInput: knowledgeBaseInput{ProjectDir: root},
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if listed := decodeKnowledgeEnvelope(t, result); len(knowledgeEntriesOf(listed)) != 3 {
		t.Fatalf("Einträge nach dem Schreiben: %+v", knowledgeEntriesOf(listed))
	}
	result, _, err = knowledgeStatusTool(context.Background(), nil, knowledgeStatusInput{
		knowledgeBaseInput: knowledgeBaseInput{ProjectDir: root},
	})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	status := decodeKnowledgeEnvelope(t, result)
	if !status.OK || status.Status == nil {
		t.Fatalf("Umschlag: %#v", status)
	}
	if status.Status.IndexKind != project.KnowledgeIndexKind || status.Status.FileCount != 3 || status.Status.ChunkCount == 0 {
		t.Errorf("Status: %+v", *status.Status)
	}
	if learned := status.Status.BySource[project.KnowledgeLearnedDirName]; learned.Files != 1 || learned.Chunks == 0 {
		t.Errorf("bySource.learned = %+v", learned)
	}
	if status.Status.Stale {
		t.Errorf("das Tor selbst hat nicht am Tor vorbei geschrieben: %+v", *status.Status)
	}
}

func TestKnowledgeWriteWehrtPfadeUndLeerenInhaltAb(t *testing.T) {
	root := newKnowledgeProject(t)

	cases := []struct {
		name  string
		input knowledgeWriteInput
	}{
		{"herausführender Pfad", knowledgeWriteInput{Path: "../manual/neu.md", Content: "# Neu\n", Source: "Test"}},
		{"absoluter Pfad", knowledgeWriteInput{Path: filepath.Join(root, "neu.md"), Content: "# Neu\n", Source: "Test"}},
		{"keine Markdown-Datei", knowledgeWriteInput{Path: "neu.txt", Content: "# Neu\n", Source: "Test"}},
		{"leerer Inhalt", knowledgeWriteInput{Path: "neu.md", Content: "  \n", Source: "Test"}},
		{"fehlende Herkunft", knowledgeWriteInput{Path: "neu.md", Content: "# Neu\n", Source: " "}},
	}
	for _, tc := range cases {
		tc.input.ProjectDir = root
		result, _, err := knowledgeWriteTool(context.Background(), nil, tc.input)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		envelope := decodeKnowledgeEnvelope(t, result)
		if envelope.OK || envelope.Error == nil || envelope.Error.Code != "invalid_input" || !result.IsError {
			t.Errorf("%s wurde angenommen: %#v", tc.name, envelope)
		}
	}
	if _, err := os.Stat(filepath.Join(project.KnowledgeDir(root), "manual", "neu.md")); !os.IsNotExist(err) {
		t.Errorf("Datei außerhalb von learned/ entstanden: %v", err)
	}
}

func TestKnowledgeToolOhneProjektDir(t *testing.T) {
	result, _, err := knowledgeStatusTool(context.Background(), nil, knowledgeStatusInput{})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	envelope := decodeKnowledgeEnvelope(t, result)
	if envelope.OK || envelope.Error == nil || envelope.Error.Code != "project_not_found" {
		t.Fatalf("erwartet project_not_found, bekommen %#v", envelope)
	}
}

// rawKnowledgeJSON gibt die Antwort als rohe Schlüssel zurück. Nach dem
// Dekodieren in den Umschlag sind ein fehlender Schlüssel und eine leere Liste
// nicht mehr zu unterscheiden — genau darum geht es hier.
func rawKnowledgeJSON(t *testing.T, result *mcp.CallToolResult) map[string]json.RawMessage {
	t.Helper()

	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("Inhalt ist kein Text: %#v", result.Content[0])
	}
	raw := map[string]json.RawMessage{}
	if err := json.Unmarshal([]byte(text.Text), &raw); err != nil {
		t.Fatalf("Antwort ist kein JSON-Objekt: %v — %s", err, text.Text)
	}
	return raw
}

// Eine trefferlose Suche antwortet mit "hits": [], eine leere Auflistung mit
// "entries": [] — so wie `k-playbook knowledge --json`, das der Umschlag als
// Vorbild nennt. Ein fehlender Schlüssel wäre für einen Aufrufer, der die
// Länge liest, etwas anderes als eine leere Liste.
func TestKnowledgeLeereErgebnisseBleibenImJSON(t *testing.T) {
	root := newKnowledgeProject(t)

	result, _, err := knowledgeSearchTool(context.Background(), nil, knowledgeSearchInput{
		knowledgeBaseInput: knowledgeBaseInput{ProjectDir: root}, Query: "Wortdasnirgendsvorkommt",
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if envelope := decodeKnowledgeEnvelope(t, result); !envelope.OK || len(knowledgeHitsOf(envelope)) != 0 {
		t.Fatalf("erwartet eine trefferlose, gelungene Suche: %#v", envelope)
	}
	if got := string(rawKnowledgeJSON(t, result)["hits"]); got != "[]" {
		t.Errorf(`hits = %s, erwartet []`, got)
	}

	result, _, err = knowledgeListTool(context.Background(), nil, knowledgeListInput{
		knowledgeBaseInput: knowledgeBaseInput{ProjectDir: root}, Source: "gibtesnicht",
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if got := string(rawKnowledgeJSON(t, result)["entries"]); got != "[]" {
		t.Errorf(`entries = %s, erwartet []`, got)
	}

	// Gegenprobe: ein Werkzeug, das weder sucht noch auflistet, trägt die
	// beiden Schlüssel gar nicht erst — null wäre ein vorgetäuschtes leeres
	// Ergebnis.
	result, _, err = knowledgeStatusTool(context.Background(), nil, knowledgeStatusInput{
		knowledgeBaseInput: knowledgeBaseInput{ProjectDir: root},
	})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	raw := rawKnowledgeJSON(t, result)
	if _, ok := raw["hits"]; ok {
		t.Errorf("status tr\u00e4gt hits, erwartet keinen Schl\u00fcssel")
	}
	if _, ok := raw["entries"]; ok {
		t.Errorf("status tr\u00e4gt entries, erwartet keinen Schl\u00fcssel")
	}
}

// knowledgeHitsOf und knowledgeEntriesOf lesen die Zeigerfelder des Umschlags
// nil-sicher: nur das Werkzeug, das sie beantwortet, setzt sie.
func knowledgeHitsOf(envelope knowledgeEnvelope) []project.Hit {
	if envelope.Hits == nil {
		return nil
	}
	return *envelope.Hits
}

func knowledgeEntriesOf(envelope knowledgeEnvelope) []project.KnowledgeEntry {
	if envelope.Entries == nil {
		return nil
	}
	return *envelope.Entries
}
