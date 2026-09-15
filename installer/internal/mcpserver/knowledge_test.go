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

// newKnowledgeProject legt ein k-playbook-Projekt mit einer kleinen
// Wissensablage an: eine flache README (source root) und ein Dokument unter
// manual/. findings/ entsteht erst durch das Schreiben.
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
	if hit.Path != "manual/ablauf.md" || hit.Heading != "Freigabe" || hit.Kind != "manual" || hit.Rank != 1 || hit.Anchor != "freigabe" {
		t.Errorf("Treffer: %+v", hit)
	}
	if !strings.HasPrefix(hit.Excerpt, "Die Freigabe") {
		t.Errorf("Excerpt = %q, erwartet den Anfang des Chunks", hit.Excerpt)
	}

	// Der Filter auf die Art grenzt ein: unter root gibt es keinen Wächter.
	result, _, err = knowledgeSearchTool(context.Background(), nil, knowledgeSearchInput{
		knowledgeBaseInput: knowledgeBaseInput{ProjectDir: root}, Query: "Wächter", Kind: project.KnowledgeRootKind,
	})
	if err != nil {
		t.Fatalf("search kind: %v", err)
	}
	if envelope = decodeKnowledgeEnvelope(t, result); !envelope.OK || len(knowledgeHitsOf(envelope)) != 0 || envelope.Kind != "root" {
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
	if knowledgeEntriesOf(envelope)[0].Path != "README.md" || knowledgeEntriesOf(envelope)[0].Kind != project.KnowledgeRootKind || knowledgeEntriesOf(envelope)[0].Title != "Index" {
		t.Errorf("erster Eintrag: %+v", knowledgeEntriesOf(envelope)[0])
	}
	if knowledgeEntriesOf(envelope)[1].Path != "manual/ablauf.md" || knowledgeEntriesOf(envelope)[1].Kind != "manual" || knowledgeEntriesOf(envelope)[1].Title != "Ablauf" {
		t.Errorf("zweiter Eintrag: %+v", knowledgeEntriesOf(envelope)[1])
	}

	result, _, err = knowledgeListTool(context.Background(), nil, knowledgeListInput{
		knowledgeBaseInput: knowledgeBaseInput{ProjectDir: root}, Kind: "manual",
	})
	if err != nil {
		t.Fatalf("list kind: %v", err)
	}
	if envelope = decodeKnowledgeEnvelope(t, result); len(knowledgeEntriesOf(envelope)) != 1 || knowledgeEntriesOf(envelope)[0].Kind != "manual" {
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

// Die Hülle reicht Erzeuger, Felder und Queue an project.Knowledge.Write
// durch und meldet den Pfad, den Write berechnet hat. Danach kennen read,
// search, list und status das Dokument unter genau diesem Pfad.
func TestKnowledgeWriteSchreibtMitErzeugerUndFeldern(t *testing.T) {
	root := newKnowledgeProject(t)

	result, _, err := knowledgeWriteTool(context.Background(), nil, knowledgeWriteInput{
		knowledgeBaseInput: knowledgeBaseInput{ProjectDir: root},
		Producer:           "session",
		knowledgeDocumentInput: knowledgeDocumentInput{
			Path: "findings/sitzung.md", Title: "Gelernt", Subject: "Release", Origin: "Task 056", State: "reviewed",
			Body: "# Gelernt\n\n## Erkenntnis\n\nDer Wächter verlangt ein Release.\n",
		},
	})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	written := decodeKnowledgeEnvelope(t, result)
	if !written.OK || !written.Written || written.Path != "findings/sitzung.md" || written.Producer != "session" {
		t.Fatalf("Umschlag: %#v", written)
	}
	if _, err := os.Stat(filepath.Join(project.KnowledgeDir(root), "findings", "sitzung.md")); err != nil {
		t.Fatalf("Datei fehlt unter findings/: %v", err)
	}

	// Lesen unter dem gemeldeten Pfad, mit dem erzeugten Frontmatter.
	result, _, err = knowledgeReadTool(context.Background(), nil, knowledgeReadInput{
		knowledgeBaseInput: knowledgeBaseInput{ProjectDir: root}, Path: written.Path,
	})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	read := decodeKnowledgeEnvelope(t, result)
	if !read.OK || read.Content == nil || !strings.HasPrefix(*read.Content, "---\ntitle: Gelernt\nsubject: Release\norigin: Task 056\nstate: reviewed\nformat: markdown\nupdated: ") {
		t.Fatalf("gelesener Inhalt: %#v", read)
	}

	// Suchen findet das neue Dokument unter findings.
	result, _, err = knowledgeSearchTool(context.Background(), nil, knowledgeSearchInput{
		knowledgeBaseInput: knowledgeBaseInput{ProjectDir: root}, Query: "Release", Kind: "findings",
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	searched := decodeKnowledgeEnvelope(t, result)
	if len(knowledgeHitsOf(searched)) != 1 || knowledgeHitsOf(searched)[0].Path != "findings/sitzung.md" || knowledgeHitsOf(searched)[0].Heading != "Erkenntnis" {
		t.Fatalf("Treffer nach dem Schreiben: %+v", knowledgeHitsOf(searched))
	}

	// Auflisten kennt drei Dateien, Status zählt findings mit.
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
	if findings := status.Status.ByKind["findings"]; findings.Files != 1 || findings.Chunks == 0 {
		t.Errorf("byKind.findings = %+v", findings)
	}
	if status.Status.Stale {
		t.Errorf("das Tor selbst hat nicht am Tor vorbei geschrieben: %+v", *status.Status)
	}
}

// Jede Eingabe-Ablehnung aus project/ kommt als invalid_input zurück —
// Erzeuger, Ziel, Felder, Rumpf und ein unbekannter Queue-Eintrag —, und
// nichts davon hinterlässt eine Datei.
func TestKnowledgeWriteWehrtErzeugerZielUndFelderAb(t *testing.T) {
	root := newKnowledgeProject(t)

	good := knowledgeDocumentInput{Path: "findings/neu.md", Title: "Neu", Subject: "Test", Origin: "Test", State: "raw", Body: "# Neu\n"}
	cases := []struct {
		name  string
		input knowledgeWriteInput
	}{
		{"herausführender Pfad", knowledgeWriteInput{Producer: "session", knowledgeDocumentInput: withPath(good, "../manual/neu.md")}},
		{"absoluter Pfad", knowledgeWriteInput{Producer: "session", knowledgeDocumentInput: withPath(good, filepath.Join(root, "neu.md"))}},
		{"keine Markdown-Datei", knowledgeWriteInput{Producer: "session", knowledgeDocumentInput: withPath(good, "findings/neu.txt")}},
		{"fremdes Verzeichnis", knowledgeWriteInput{Producer: "session", knowledgeDocumentInput: withPath(good, "manual/neu.md")}},
		{"unbekannter Erzeuger", knowledgeWriteInput{Producer: "gate", knowledgeDocumentInput: good}},
		{"Generator", knowledgeWriteInput{Producer: "docs-code", knowledgeDocumentInput: withPath(good, "code/neu.md")}},
		{"leerer body", knowledgeWriteInput{Producer: "session", knowledgeDocumentInput: withBody(good, "  \n")}},
		{"Kopf im body", knowledgeWriteInput{Producer: "session", knowledgeDocumentInput: withBody(good, "---\ntitle: X\n---\n# Neu\n")}},
		{"state superseded", knowledgeWriteInput{Producer: "session", knowledgeDocumentInput: withState(good, "superseded")}},
		{"fehlender title", knowledgeWriteInput{Producer: "session", knowledgeDocumentInput: withTitle(good, " ")}},
		{"unbekannter Queue-Eintrag", knowledgeWriteInput{Producer: "session", knowledgeDocumentInput: good, Queue: "gibt-es-nicht"}},
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
	for _, rel := range []string{"manual/neu.md", "findings/neu.md", "code/neu.md"} {
		if _, err := os.Stat(filepath.Join(project.KnowledgeDir(root), filepath.FromSlash(rel))); !os.IsNotExist(err) {
			t.Errorf("%s ist trotz Ablehnung entstanden: %v", rel, err)
		}
	}
}

func withPath(doc knowledgeDocumentInput, path string) knowledgeDocumentInput {
	doc.Path = path
	return doc
}

func withBody(doc knowledgeDocumentInput, body string) knowledgeDocumentInput {
	doc.Body = body
	return doc
}

func withState(doc knowledgeDocumentInput, state string) knowledgeDocumentInput {
	doc.State = state
	return doc
}

func withTitle(doc knowledgeDocumentInput, title string) knowledgeDocumentInput {
	doc.Title = title
	return doc
}

// publish tauscht das Generatorverzeichnis über die Hülle; supersede löst ein
// Dokument ab. Beide melden Eingabefehler aus project/ als invalid_input.
func TestKnowledgePublishUndSupersedeUeberDieHuelle(t *testing.T) {
	root := newKnowledgeProject(t)
	doc := func(path string) knowledgeDocumentInput {
		return knowledgeDocumentInput{Path: path, Title: "Code", Subject: "Quelle", Origin: "/k-docs-code", State: "condensed", Body: "# " + path + "\n"}
	}

	result, _, err := knowledgePublishTool(context.Background(), nil, knowledgePublishInput{
		knowledgeBaseInput: knowledgeBaseInput{ProjectDir: root}, Producer: "docs-code",
		Documents: []knowledgeDocumentInput{doc("links.md"), doc("tief/unten.md")},
	})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	published := decodeKnowledgeEnvelope(t, result)
	if !published.OK || published.Publish == nil || published.Publish.Written != 2 || published.Publish.Removed != 0 || published.Publish.Dir != "code/" || published.Producer != "docs-code" {
		t.Fatalf("Umschlag: %#v", published)
	}
	for _, rel := range []string{"code/links.md", "code/tief/unten.md"} {
		if _, err := os.Stat(filepath.Join(project.KnowledgeDir(root), filepath.FromSlash(rel))); err != nil {
			t.Errorf("%s fehlt: %v", rel, err)
		}
	}

	// Ein zweiter Satz ohne tief/unten.md entfernt es.
	result, _, err = knowledgePublishTool(context.Background(), nil, knowledgePublishInput{
		knowledgeBaseInput: knowledgeBaseInput{ProjectDir: root}, Producer: "docs-code",
		Documents: []knowledgeDocumentInput{doc("links.md")},
	})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if published = decodeKnowledgeEnvelope(t, result); published.Publish == nil || published.Publish.Removed != 1 {
		t.Errorf("zweiter Satz: %#v", published)
	}

	// Kein Generator, ungültiger Satz: invalid_input.
	for name, input := range map[string]knowledgePublishInput{
		"kein Generator": {Producer: "session", Documents: []knowledgeDocumentInput{doc("x.md")}},
		"ungültig":       {Producer: "docs-code", Documents: []knowledgeDocumentInput{withState(doc("x.md"), "superseded")}},
	} {
		input.ProjectDir = root
		result, _, err := knowledgePublishTool(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if envelope := decodeKnowledgeEnvelope(t, result); envelope.OK || envelope.Error == nil || envelope.Error.Code != "invalid_input" {
			t.Errorf("%s angenommen: %#v", name, envelope)
		}
	}

	// supersede: manual/ablauf.md wird durch findings/nachfolger.md abgelöst.
	// Ein Nachfolger unter code/ wäre abgewiesen (Task 063, Entscheidung 4).
	result, _, err = knowledgeWriteTool(context.Background(), nil, knowledgeWriteInput{
		knowledgeBaseInput: knowledgeBaseInput{ProjectDir: root}, Producer: "session",
		knowledgeDocumentInput: knowledgeDocumentInput{Path: "findings/nachfolger.md", Title: "N", Subject: "S", Origin: "O", State: "condensed", Body: "# N\n"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if written := decodeKnowledgeEnvelope(t, result); !written.OK {
		t.Fatalf("Nachfolger schreiben: %#v", written)
	}
	result, _, err = knowledgeSupersedeTool(context.Background(), nil, knowledgeSupersedeInput{
		knowledgeBaseInput: knowledgeBaseInput{ProjectDir: root}, Path: "manual/ablauf.md", Successor: "findings/nachfolger.md", Reason: "Ersetzt",
	})
	if err != nil {
		t.Fatalf("supersede: %v", err)
	}
	superseded := decodeKnowledgeEnvelope(t, result)
	if !superseded.OK || !superseded.Superseded || superseded.Path != "manual/ablauf.md" || superseded.Successor != "findings/nachfolger.md" {
		t.Fatalf("Umschlag: %#v", superseded)
	}
	result, _, err = knowledgeReadTool(context.Background(), nil, knowledgeReadInput{knowledgeBaseInput: knowledgeBaseInput{ProjectDir: root}, Path: "manual/ablauf.md"})
	if err != nil {
		t.Fatal(err)
	}
	if read := decodeKnowledgeEnvelope(t, result); read.Content == nil || !strings.Contains(*read.Content, "state: superseded\nsuccessor: findings/nachfolger.md\nsuperseded_reason: Ersetzt\n") {
		t.Errorf("abgelöst gelesen: %#v", read)
	}
	result, _, err = knowledgeSupersedeTool(context.Background(), nil, knowledgeSupersedeInput{
		knowledgeBaseInput: knowledgeBaseInput{ProjectDir: root}, Path: "manual/ablauf.md", Successor: "findings/fehlt.md", Reason: "x",
	})
	if err != nil {
		t.Fatal(err)
	}
	if envelope := decodeKnowledgeEnvelope(t, result); envelope.OK || envelope.Error == nil || envelope.Error.Code != "invalid_input" {
		t.Errorf("fehlender Nachfolger angenommen: %#v", envelope)
	}
}

// Ein leerer Satz über die Hülle — documents: [] oder das Feld weggelassen
// (nil) — ist ein Eingabefehler und leert das Generatorverzeichnis nicht. Die
// Kommandozeile war über den Lader geschützt; hier zählt der Wächter im Kern.
func TestKnowledgePublishLeererSatzUeberDieHuelle(t *testing.T) {
	root := newKnowledgeProject(t)
	doc := knowledgeDocumentInput{Path: "links.md", Title: "Code", Subject: "Quelle", Origin: "/k-docs-code", State: "condensed", Body: "# Links\n"}
	result, _, err := knowledgePublishTool(context.Background(), nil, knowledgePublishInput{
		knowledgeBaseInput: knowledgeBaseInput{ProjectDir: root}, Producer: "docs-code", Documents: []knowledgeDocumentInput{doc},
	})
	if err != nil {
		t.Fatal(err)
	}
	if published := decodeKnowledgeEnvelope(t, result); !published.OK {
		t.Fatalf("erster Lauf: %#v", published)
	}
	target := filepath.Join(project.KnowledgeDir(root), "code", "links.md")

	for name, documents := range map[string][]knowledgeDocumentInput{"leer": {}, "weggelassen": nil} {
		result, _, err := knowledgePublishTool(context.Background(), nil, knowledgePublishInput{
			knowledgeBaseInput: knowledgeBaseInput{ProjectDir: root}, Producer: "docs-code", Documents: documents,
		})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		envelope := decodeKnowledgeEnvelope(t, result)
		if envelope.OK || envelope.Error == nil || envelope.Error.Code != "invalid_input" || !result.IsError {
			t.Errorf("%s angenommen: %#v", name, envelope)
		}
		if _, err := os.Stat(target); err != nil {
			t.Errorf("%s: code/links.md nach leerem Satz weg: %v", name, err)
		}
	}
}

// Eingang und Warteschlange über die Hüllen: put mit content und mit file,
// list mit Notiz, read nur für Text; queue add → list → write mit queue →
// leer; drop.
func TestKnowledgeInboxUndQueueUeberDieHuelle(t *testing.T) {
	root := newKnowledgeProject(t)
	base := knowledgeBaseInput{ProjectDir: root}

	result, _, err := knowledgeInboxPutTool(context.Background(), nil, knowledgeInboxPutInput{
		knowledgeBaseInput: base, Source: "chat", Name: "standup.md", Content: "# Standup\n\nKennwort.\n", Note: "Vom 12.9.",
	})
	if err != nil {
		t.Fatal(err)
	}
	put := decodeKnowledgeEnvelope(t, result)
	if !put.OK || !put.Written || put.Path != "chat/standup.md" || put.Source != "chat" {
		t.Fatalf("inbox_put: %#v", put)
	}
	binary := filepath.Join(root, "seite.pdf")
	if err := os.WriteFile(binary, []byte{0x25, 0x50, 0x44, 0x46}, 0o644); err != nil {
		t.Fatal(err)
	}
	result, _, err = knowledgeInboxPutTool(context.Background(), nil, knowledgeInboxPutInput{knowledgeBaseInput: base, Source: "scan", Name: "seite.pdf", File: binary})
	if err != nil {
		t.Fatal(err)
	}
	if put = decodeKnowledgeEnvelope(t, result); !put.OK || put.Path != "scan/seite.pdf" {
		t.Fatalf("inbox_put file: %#v", put)
	}
	for name, input := range map[string]knowledgeInboxPutInput{
		"ohne Inhalt":      {Source: "chat", Name: "leer.md"},
		"content und file": {Source: "chat", Name: "beides.md", Content: "x", File: binary},
		"belegter Name":    {Source: "chat", Name: "standup.md", Content: "x"},
		"Ausbruch":         {Source: "chat", Name: "../x.md", Content: "x"},
	} {
		input.ProjectDir = root
		result, _, err := knowledgeInboxPutTool(context.Background(), nil, input)
		if err != nil {
			t.Fatal(err)
		}
		if envelope := decodeKnowledgeEnvelope(t, result); envelope.OK || envelope.Error == nil || envelope.Error.Code != "invalid_input" {
			t.Errorf("%s angenommen: %#v", name, envelope)
		}
	}

	result, _, err = knowledgeInboxListTool(context.Background(), nil, knowledgeInboxListInput{knowledgeBaseInput: base})
	if err != nil {
		t.Fatal(err)
	}
	listed := decodeKnowledgeEnvelope(t, result)
	if !listed.OK || listed.Inbox == nil || len(*listed.Inbox) != 2 || (*listed.Inbox)[0].Note != "Vom 12.9." || (*listed.Inbox)[1].Format != "pdf" {
		t.Fatalf("inbox_list: %#v", listed)
	}
	if got := string(rawKnowledgeJSON(t, result)["inbox"]); !strings.HasPrefix(got, "[") {
		t.Errorf("inbox = %s", got)
	}

	result, _, err = knowledgeInboxReadTool(context.Background(), nil, knowledgeInboxReadInput{knowledgeBaseInput: base, Path: "chat/standup.md"})
	if err != nil {
		t.Fatal(err)
	}
	if read := decodeKnowledgeEnvelope(t, result); !read.OK || read.Content == nil || *read.Content != "# Standup\n\nKennwort.\n" {
		t.Errorf("inbox_read: %#v", read)
	}
	result, _, err = knowledgeInboxReadTool(context.Background(), nil, knowledgeInboxReadInput{knowledgeBaseInput: base, Path: "scan/seite.pdf"})
	if err != nil {
		t.Fatal(err)
	}
	if read := decodeKnowledgeEnvelope(t, result); read.OK || read.Error == nil || !strings.Contains(read.Error.Message, "kein Textformat") {
		t.Errorf("PDF gelesen: %#v", read)
	}

	result, _, err = knowledgeQueueAddTool(context.Background(), nil, knowledgeQueueAddInput{knowledgeBaseInput: base, Origin: "chat/standup.md", Target: "findings", Reason: "Befund"})
	if err != nil {
		t.Fatal(err)
	}
	added := decodeKnowledgeEnvelope(t, result)
	if !added.OK || !added.Added || !strings.HasSuffix(added.ID, "-chat-standup-md") {
		t.Fatalf("queue_add: %#v", added)
	}
	result, _, err = knowledgeQueueListTool(context.Background(), nil, knowledgeQueueListInput{knowledgeBaseInput: base})
	if err != nil {
		t.Fatal(err)
	}
	queue := decodeKnowledgeEnvelope(t, result)
	if !queue.OK || queue.Queue == nil || len(*queue.Queue) != 1 || (*queue.Queue)[0].ID != added.ID || (*queue.Queue)[0].Target != "findings/" {
		t.Fatalf("queue_list: %#v", queue)
	}

	result, _, err = knowledgeWriteTool(context.Background(), nil, knowledgeWriteInput{
		knowledgeBaseInput: base, Producer: "session", Queue: added.ID,
		knowledgeDocumentInput: knowledgeDocumentInput{Path: "findings/standup.md", Title: "Standup", Subject: "Team", Origin: "chat/standup.md", State: "condensed", Sources: []string{"chat/standup.md"}, Body: "# Standup\n"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if written := decodeKnowledgeEnvelope(t, result); !written.OK {
		t.Fatalf("write mit queue: %#v", written)
	}
	result, _, err = knowledgeQueueListTool(context.Background(), nil, knowledgeQueueListInput{knowledgeBaseInput: base})
	if err != nil {
		t.Fatal(err)
	}
	if queue = decodeKnowledgeEnvelope(t, result); queue.Queue == nil || len(*queue.Queue) != 0 {
		t.Errorf("Eintrag nach write noch da: %#v", queue)
	}
	if got := string(rawKnowledgeJSON(t, result)["queue"]); got != "[]" {
		t.Errorf("queue = %s, erwartet []", got)
	}

	result, _, err = knowledgeQueueAddTool(context.Background(), nil, knowledgeQueueAddInput{knowledgeBaseInput: base, Origin: "mail/x.eml", Target: "extracted/", Reason: "x"})
	if err != nil {
		t.Fatal(err)
	}
	id := decodeKnowledgeEnvelope(t, result).ID
	result, _, err = knowledgeQueueDropTool(context.Background(), nil, knowledgeQueueDropInput{knowledgeBaseInput: base, ID: id, Reason: "doch nicht"})
	if err != nil {
		t.Fatal(err)
	}
	if dropped := decodeKnowledgeEnvelope(t, result); !dropped.OK || !dropped.Dropped || dropped.ID != id {
		t.Errorf("queue_drop: %#v", dropped)
	}
	result, _, err = knowledgeQueueDropTool(context.Background(), nil, knowledgeQueueDropInput{knowledgeBaseInput: base, ID: id})
	if err != nil {
		t.Fatal(err)
	}
	if dropped := decodeKnowledgeEnvelope(t, result); dropped.OK || dropped.Error == nil || dropped.Error.Code != "invalid_input" {
		t.Errorf("zweiter drop angenommen: %#v", dropped)
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
		knowledgeBaseInput: knowledgeBaseInput{ProjectDir: root}, Kind: "gibtesnicht",
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
// denyWrite nimmt einem Verzeichnis das Schreibrecht und gibt es am Ende des
// Tests zurück, damit t.TempDir() aufräumen kann. Greifen die Rechte nicht —
// als root, auf manchen Dateisystemen —, bewiese der Test nichts und wird
// übersprungen. Dasselbe Muster wie in project.
func denyWrite(t *testing.T, dir string) {
	t.Helper()
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("%s: %v", dir, err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("%s sperren: %v", dir, err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, info.Mode().Perm()) })

	probe := filepath.Join(dir, ".schreibprobe")
	if err := os.WriteFile(probe, []byte("x"), 0o644); err == nil {
		_ = os.Remove(probe)
		t.Skip("das entzogene Schreibrecht greift hier nicht")
	}
}

// denyRead nimmt einer Datei das Leserecht, mit derselben Vorsichtsmaßnahme.
func denyRead(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatalf("%s sperren: %v", path, err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, info.Mode().Perm()) })

	if _, err := os.ReadFile(path); err == nil {
		t.Skip("das entzogene Leserecht greift hier nicht")
	}
}

// Der Fehlercode sagt, ob die Argumente oder die Umgebung falsch sind:
// invalid_input nur für Eingabefehler aus project/ (auch „nicht vorhanden"),
// write_failed für alles andere bei schreibenden, read_failed bei lesenden
// Hüllen. Vorher meldete jede schreibende Hülle jeden Fehler als
// invalid_input — ein Aufrufer, der daran entscheidet, korrigierte bei einem
// nicht beschreibbaren knowledge/ endlos.
func TestKnowledgeFehlercodesUnterscheidenEingabeUndUmgebung(t *testing.T) {
	code := func(t *testing.T, result *mcp.CallToolResult, err error) string {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		envelope := decodeKnowledgeEnvelope(t, result)
		if envelope.OK || envelope.Error == nil || !result.IsError {
			t.Fatalf("kein Fehler: %#v", envelope)
		}
		return envelope.Error.Code
	}
	good := knowledgeDocumentInput{Path: "findings/neu.md", Title: "Neu", Subject: "Test", Origin: "Test", State: "raw", Body: "# Neu\n"}

	t.Run("nicht beschreibbares knowledge/ ist write_failed", func(t *testing.T) {
		root := newKnowledgeProject(t)
		denyWrite(t, project.KnowledgeDir(root))
		base := knowledgeBaseInput{ProjectDir: root}

		result, _, err := knowledgeWriteTool(context.Background(), nil, knowledgeWriteInput{knowledgeBaseInput: base, Producer: "session", knowledgeDocumentInput: good})
		if got := code(t, result, err); got != "write_failed" {
			t.Errorf("write: code = %q, erwartet write_failed", got)
		}
		result, _, err = knowledgePublishTool(context.Background(), nil, knowledgePublishInput{
			knowledgeBaseInput: base, Producer: "docs-code", Documents: []knowledgeDocumentInput{withPath(good, "neu.md")},
		})
		if got := code(t, result, err); got != "write_failed" {
			t.Errorf("publish: code = %q, erwartet write_failed", got)
		}
	})

	t.Run("falscher Erzeuger ist invalid_input", func(t *testing.T) {
		root := newKnowledgeProject(t)
		result, _, err := knowledgeWriteTool(context.Background(), nil, knowledgeWriteInput{
			knowledgeBaseInput: knowledgeBaseInput{ProjectDir: root}, Producer: "gate", knowledgeDocumentInput: good,
		})
		if got := code(t, result, err); got != "invalid_input" {
			t.Errorf("code = %q, erwartet invalid_input", got)
		}
	})

	t.Run("fehlender Pfad bei read ist invalid_input", func(t *testing.T) {
		root := newKnowledgeProject(t)
		result, _, err := knowledgeReadTool(context.Background(), nil, knowledgeReadInput{knowledgeBaseInput: knowledgeBaseInput{ProjectDir: root}, Path: "manual/fehlt.md"})
		if got := code(t, result, err); got != "invalid_input" {
			t.Errorf("code = %q, erwartet invalid_input", got)
		}
	})

	t.Run("vorhandener, unlesbarer Pfad bei read ist read_failed", func(t *testing.T) {
		root := newKnowledgeProject(t)
		denyRead(t, filepath.Join(project.KnowledgeDir(root), "manual", "ablauf.md"))
		result, _, err := knowledgeReadTool(context.Background(), nil, knowledgeReadInput{knowledgeBaseInput: knowledgeBaseInput{ProjectDir: root}, Path: "manual/ablauf.md"})
		if got := code(t, result, err); got != "read_failed" {
			t.Errorf("code = %q, erwartet read_failed", got)
		}
	})

	t.Run("unbekannter Queue-Eintrag und fehlender Nachfolger sind invalid_input", func(t *testing.T) {
		root := newKnowledgeProject(t)
		base := knowledgeBaseInput{ProjectDir: root}
		result, _, err := knowledgeQueueDropTool(context.Background(), nil, knowledgeQueueDropInput{knowledgeBaseInput: base, ID: "gibt-es-nicht"})
		if got := code(t, result, err); got != "invalid_input" {
			t.Errorf("queue_drop: code = %q, erwartet invalid_input", got)
		}
		result, _, err = knowledgeSupersedeTool(context.Background(), nil, knowledgeSupersedeInput{knowledgeBaseInput: base, Path: "manual/ablauf.md", Successor: "manual/fehlt.md", Reason: "x"})
		if got := code(t, result, err); got != "invalid_input" {
			t.Errorf("supersede: code = %q, erwartet invalid_input", got)
		}
	})
}

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

// Ein Dokumentpfad mit vorangestelltem Generatorverzeichnis ist über die Hülle
// invalid_input, und der Bestand bleibt (Task 063, Entscheidung 1).
func TestKnowledgePublishPfadMitErzeugerverzeichnisUeberDieHuelle(t *testing.T) {
	root := newKnowledgeProject(t)
	doc := func(path string) knowledgeDocumentInput {
		return knowledgeDocumentInput{Path: path, Title: "Code", Subject: "Quelle", Origin: "/k-docs-code", State: "condensed", Body: "# " + path + "\n"}
	}
	base := knowledgeBaseInput{ProjectDir: root}
	result, _, err := knowledgePublishTool(context.Background(), nil, knowledgePublishInput{knowledgeBaseInput: base, Producer: "docs-code", Documents: []knowledgeDocumentInput{doc("links.md")}})
	if err != nil {
		t.Fatal(err)
	}
	if published := decodeKnowledgeEnvelope(t, result); !published.OK {
		t.Fatalf("erster Lauf: %#v", published)
	}

	result, _, err = knowledgePublishTool(context.Background(), nil, knowledgePublishInput{knowledgeBaseInput: base, Producer: "docs-code", Documents: []knowledgeDocumentInput{doc("code/zztest-dup.md")}})
	if err != nil {
		t.Fatal(err)
	}
	envelope := decodeKnowledgeEnvelope(t, result)
	if envelope.OK || envelope.Error == nil || envelope.Error.Code != "invalid_input" || !strings.Contains(envelope.Error.Message, "relativ zu code/") {
		t.Errorf("angenommen oder falscher Code: %#v", envelope)
	}
	if _, err := os.Stat(filepath.Join(project.KnowledgeDir(root), "code", "links.md")); err != nil {
		t.Errorf("Bestand weg: %v", err)
	}
	if _, err := os.Stat(filepath.Join(project.KnowledgeDir(root), "code", "code")); err == nil {
		t.Error("code/code/ entstanden")
	}
}

// supersede über die Hülle: die Antwort nennt den bereinigten Nachfolger, die
// neuen Abweisungen (Task 063, Entscheidungen 2 bis 4 und 7) sind
// invalid_input.
func TestKnowledgeSupersedeUeberDieHuelle(t *testing.T) {
	root := newKnowledgeProject(t)
	base := knowledgeBaseInput{ProjectDir: root}
	write := func(producer, path, state string) {
		t.Helper()
		result, _, err := knowledgeWriteTool(context.Background(), nil, knowledgeWriteInput{
			knowledgeBaseInput: base, Producer: producer,
			knowledgeDocumentInput: knowledgeDocumentInput{Path: path, Title: "T", Subject: "S", Origin: "O", State: state, Body: "# " + path + "\n"},
		})
		if err != nil {
			t.Fatal(err)
		}
		if envelope := decodeKnowledgeEnvelope(t, result); !envelope.OK {
			t.Fatalf("write %s: %#v", path, envelope)
		}
	}
	write("session", "findings/y.md", "condensed")
	write("session", "findings/z.md", "condensed")
	write("session", "findings/roh.md", "raw")
	result, _, err := knowledgePublishTool(context.Background(), nil, knowledgePublishInput{knowledgeBaseInput: base, Producer: "docs-code",
		Documents: []knowledgeDocumentInput{{Path: "links.md", Title: "C", Subject: "S", Origin: "O", State: "condensed", Body: "# C\n"}}})
	if err != nil {
		t.Fatal(err)
	}
	if envelope := decodeKnowledgeEnvelope(t, result); !envelope.OK {
		t.Fatalf("publish: %#v", envelope)
	}

	result, _, err = knowledgeSupersedeTool(context.Background(), nil, knowledgeSupersedeInput{knowledgeBaseInput: base, Path: "manual/ablauf.md", Successor: "./findings/y.md", Reason: "Ersetzt"})
	if err != nil {
		t.Fatal(err)
	}
	if envelope := decodeKnowledgeEnvelope(t, result); !envelope.OK || envelope.Path != "manual/ablauf.md" || envelope.Successor != "findings/y.md" {
		t.Errorf("Umschlag: %#v", envelope)
	}

	for name, input := range map[string]knowledgeSupersedeInput{
		"Nachfolger unter code/": {Path: "findings/z.md", Successor: "code/links.md", Reason: "x"},
		"Nachfolger README":      {Path: "findings/z.md", Successor: "README.md", Reason: "x"},
		"Nachfolger raw":         {Path: "findings/z.md", Successor: "findings/roh.md", Reason: "x"},
		"Ziel unter code/":       {Path: "code/links.md", Successor: "findings/z.md", Reason: "x"},
		"Ziel README":            {Path: "README.md", Successor: "findings/z.md", Reason: "x"},
		"schon abgelöst":         {Path: "manual/ablauf.md", Successor: "findings/z.md", Reason: "x"},
		"Nachfolger abgelöst":    {Path: "findings/z.md", Successor: "manual/ablauf.md", Reason: "x"},
	} {
		input.knowledgeBaseInput = base
		result, _, err := knowledgeSupersedeTool(context.Background(), nil, input)
		if err != nil {
			t.Fatal(err)
		}
		if envelope := decodeKnowledgeEnvelope(t, result); envelope.OK || envelope.Error == nil || envelope.Error.Code != "invalid_input" {
			t.Errorf("%s: %#v", name, envelope)
		}
	}

	result, _, err = knowledgeWriteTool(context.Background(), nil, knowledgeWriteInput{
		knowledgeBaseInput: base, Producer: "person",
		knowledgeDocumentInput: knowledgeDocumentInput{Path: "manual/ablauf.md", Title: "T", Subject: "S", Origin: "O", State: "condensed", Body: "# Neu\n"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if envelope := decodeKnowledgeEnvelope(t, result); envelope.OK || envelope.Error == nil || envelope.Error.Code != "invalid_input" {
		t.Errorf("write auf abgelöstes Dokument: %#v", envelope)
	}
	content, err := os.ReadFile(filepath.Join(project.KnowledgeDir(root), "manual", "ablauf.md"))
	if err != nil || !strings.Contains(string(content), "state: superseded\nsuccessor: findings/y.md\n") {
		t.Errorf("Ablösung nicht erhalten: %v\n%s", err, content)
	}
}

// inbox_put mit einer Datei am Quellpfad ist invalid_input, nicht write_failed;
// .markdown ist über inbox_read lesbar (Task 063, Etappe 5).
func TestKnowledgeInboxQuellpfadUndMarkdownUeberDieHuelle(t *testing.T) {
	root := newKnowledgeProject(t)
	base := knowledgeBaseInput{ProjectDir: root}
	readme := filepath.Join(project.InboxDir(root), "README.md")
	if err := os.MkdirAll(filepath.Dir(readme), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(readme, []byte("# inbox\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, _, err := knowledgeInboxPutTool(context.Background(), nil, knowledgeInboxPutInput{knowledgeBaseInput: base, Source: "README.md", Name: "zztest.md", Content: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if envelope := decodeKnowledgeEnvelope(t, result); envelope.OK || envelope.Error == nil || envelope.Error.Code != "invalid_input" {
		t.Errorf("Quelle README.md: %#v", envelope)
	}

	result, _, err = knowledgeInboxPutTool(context.Background(), nil, knowledgeInboxPutInput{knowledgeBaseInput: base, Source: "chat", Name: "zztest.markdown", Content: "# Z\n"})
	if err != nil {
		t.Fatal(err)
	}
	if envelope := decodeKnowledgeEnvelope(t, result); !envelope.OK {
		t.Fatalf("inbox_put: %#v", envelope)
	}
	result, _, err = knowledgeInboxReadTool(context.Background(), nil, knowledgeInboxReadInput{knowledgeBaseInput: base, Path: "chat/zztest.markdown"})
	if err != nil {
		t.Fatal(err)
	}
	if envelope := decodeKnowledgeEnvelope(t, result); !envelope.OK || envelope.Content == nil || *envelope.Content != "# Z\n" {
		t.Errorf("inbox_read .markdown: %#v", envelope)
	}

	// inbox_list nennt .markdown mit dem Format markdown (Task 064, Etappe 6).
	result, _, err = knowledgeInboxListTool(context.Background(), nil, knowledgeInboxListInput{knowledgeBaseInput: base, Source: "chat"})
	if err != nil {
		t.Fatal(err)
	}
	envelope := decodeKnowledgeEnvelope(t, result)
	if !envelope.OK || envelope.Inbox == nil || len(*envelope.Inbox) != 1 {
		t.Fatalf("inbox_list: %#v", envelope)
	}
	if entry := (*envelope.Inbox)[0]; entry.Path != "chat/zztest.markdown" || entry.Format != "markdown" {
		t.Errorf("inbox_list .markdown: %+v", entry)
	}
}

// read stellt ein verwaistes .<dir>-alt-* zurück und meldet das im hint (Task
// 064, Entscheidung 4) — ohne das Feld käme die Meldung über MCP nie an.
func TestKnowledgeReadMeldetRueckstellungImHint(t *testing.T) {
	root := newKnowledgeProject(t)
	base := knowledgeBaseInput{ProjectDir: root}
	result, _, err := knowledgePublishTool(context.Background(), nil, knowledgePublishInput{knowledgeBaseInput: base, Producer: "docs-code",
		Documents: []knowledgeDocumentInput{{Path: "links.md", Title: "C", Subject: "S", Origin: "O", State: "condensed", Body: "# C\n\nAltstand.\n"}}})
	if err != nil {
		t.Fatal(err)
	}
	if envelope := decodeKnowledgeEnvelope(t, result); !envelope.OK {
		t.Fatalf("publish: %#v", envelope)
	}
	dir := project.KnowledgeDir(root)
	if err := os.Rename(filepath.Join(dir, "code"), filepath.Join(dir, ".code-alt-222222")); err != nil {
		t.Fatal(err)
	}

	result, _, err = knowledgeReadTool(context.Background(), nil, knowledgeReadInput{knowledgeBaseInput: base, Path: "code/links.md"})
	if err != nil {
		t.Fatal(err)
	}
	envelope := decodeKnowledgeEnvelope(t, result)
	if !envelope.OK || envelope.Content == nil || !strings.Contains(*envelope.Content, "Altstand.") {
		t.Fatalf("read nach abgebrochenem publish: %#v", envelope)
	}
	if !strings.Contains(envelope.Hint, "zurückgestellt") || !strings.Contains(envelope.Hint, ".code-alt-222222") {
		t.Errorf("hint ohne Rückstellung: %q", envelope.Hint)
	}
	if _, err := os.Stat(filepath.Join(dir, "code", "links.md")); err != nil {
		t.Errorf("code/ steht nicht wieder: %v", err)
	}
}
