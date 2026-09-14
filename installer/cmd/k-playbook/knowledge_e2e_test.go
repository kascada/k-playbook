package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// knowledgeProject legt ein Projekt mit einem kleinen Wissensverzeichnis an
// und macht es zum Arbeitsverzeichnis.
func knowledgeProject(t *testing.T) string {
	t.Helper()
	root := todoProject(t)
	for rel, content := range map[string]string{
		"README.md":         "# Index\n\nDer Einstieg.\n",
		"manual/release.md": "# Release\n\nWie ein Release entsteht.\n\n## Der Weg\n\nTag setzen, CI abwarten.\n",
	} {
		path := filepath.Join(project.KnowledgeDir(root), filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// runKnowledgeCaptured führt das Subkommando aus und fängt stdout ab.
func runKnowledgeCaptured(t *testing.T, args ...string) (string, error) {
	t.Helper()

	file, err := os.CreateTemp(t.TempDir(), "stdout-*")
	if err != nil {
		t.Fatalf("Ausgabedatei: %v", err)
	}
	before := os.Stdout
	os.Stdout = file
	runErr := runKnowledge(args)
	os.Stdout = before
	if err := file.Close(); err != nil {
		t.Fatalf("Ausgabedatei schließen: %v", err)
	}
	content, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatalf("Ausgabe lesen: %v", err)
	}
	return string(content), runErr
}

// --help ist der Rauchtest nach einem Release: Exit 0, Kurzhilfe, und der
// Index wird dabei nicht angelegt. Hilfe an Optionsposition gewinnt vor jeder
// Prüfung des Parsers — auch hinter einer vollständigen Option, hinter einer
// unbekannten und vor fehlenden Pflichtoptionen —, sonst wäre sie genau dann
// nicht zu haben, wenn man sie braucht. Das nackte help gilt nach knowledge
// und nach einer Gruppe.
func TestKnowledgeHilfeIstFolgenlos(t *testing.T) {
	root := knowledgeProject(t)

	for _, args := range [][]string{
		{}, {"--help"}, {"help"}, {"-h"},
		{"write", "--help"}, {"write", "--title", "x", "--help"}, {"write", "--schnabeltier", "--help"},
		{"inbox", "put", "-h"}, {"inbox", "help"}, {"queue", "help"}, {"queue", "add", "--origin", "x", "--help"},
	} {
		output, err := runKnowledgeCaptured(t, args...)
		if err != nil {
			t.Fatalf("knowledge %v: %v", args, err)
		}
		if !strings.Contains(output, "Unterbefehle:") {
			t.Errorf("knowledge %v ohne Kurzhilfe:\n%s", args, output)
		}
	}
	if pathExistsForTest(project.KnowledgeIndexFile(root)) {
		t.Error("die Hilfe hat den Index angelegt")
	}
	if _, err := runKnowledgeCaptured(t, "schnabeltier"); err == nil {
		t.Error("unbekannter Unterbefehl wurde angenommen")
	}
	for _, rel := range []string{"findings/x.md", "findings/-h"} {
		if pathExistsForTest(filepath.Join(project.KnowledgeDir(root), filepath.FromSlash(rel))) {
			t.Errorf("die Hilfe hat %s geschrieben", rel)
		}
	}
}

// help, -h und --help sind nur dort Hilfe, wo der Parser einen Optionsnamen
// erwartet — nie als Wert einer Option, und das Wort help nie als Suchbegriff
// oder Positionsargument. Die drei Fälle aus dem Befund: search help sucht
// nach „help", queue drop … --reason help löscht, write … --title -h
// schreibt mit dem Titel „-h".
func TestKnowledgeHilfeNurAnOptionsposition(t *testing.T) {
	root := knowledgeProject(t)

	output, err := runKnowledgeCaptured(t, "search", "help", "--json")
	if err != nil {
		t.Fatalf("search help: %v", err)
	}
	if strings.Contains(output, "Unterbefehle:") {
		t.Fatalf("search help zeigt die Hilfe statt zu suchen:\n%s", output)
	}
	if search := decodeTodoOutput[knowledgeSearchOutput](t, output); search.Query != "help" || search.Hits == nil {
		t.Errorf("search help = %+v", search)
	}

	output, err = runKnowledgeCaptured(t, "queue", "add", "--origin", "chat/x.md", "--target", "extracted/", "--reason", "Test", "--json")
	if err != nil {
		t.Fatalf("queue add: %v", err)
	}
	id := decodeTodoOutput[knowledgeQueueAddOutput](t, output).ID
	entry := filepath.Join(project.QueueDir(root), id+".md")
	output, err = runKnowledgeCaptured(t, "queue", "drop", id, "--reason", "help")
	if err != nil {
		t.Fatalf("queue drop --reason help: %v", err)
	}
	if strings.Contains(output, "Unterbefehle:") || pathExistsForTest(entry) {
		t.Errorf("queue drop --reason help hat nicht gelöscht:\n%s", output)
	}

	body := filepath.Join(root, "rumpf.md")
	if err := os.WriteFile(body, []byte("# Rumpf\n\nText.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	output, err = runKnowledgeCaptured(t, "write", "findings/strich.md", "--producer", "session", "--title", "-h",
		"--subject", "Hilfe", "--origin", "Test", "--state", "raw", "--file", body)
	if err != nil {
		t.Fatalf("write --title -h: %v", err)
	}
	if strings.Contains(output, "Unterbefehle:") {
		t.Fatalf("write --title -h zeigt die Hilfe statt zu schreiben:\n%s", output)
	}
	content, err := os.ReadFile(filepath.Join(project.KnowledgeDir(root), "findings", "strich.md"))
	if err != nil || !strings.Contains(string(content), "title: \"-h\"\n") {
		t.Errorf("findings/strich.md: %v\n%s", err, content)
	}
}

func TestKnowledgeStatusSearchListReadWrite(t *testing.T) {
	root := knowledgeProject(t)

	output, err := runKnowledgeCaptured(t, "status", "--json")
	if err != nil {
		t.Fatalf("knowledge status: %v", err)
	}
	status := decodeTodoOutput[project.KnowledgeStatus](t, output)
	if status.IndexKind != "bm25" || status.FileCount != 2 || status.ChunkCount != 2 || status.Stale {
		t.Errorf("status = %+v", status)
	}
	if status.ByKind["root"].Files != 1 || status.ByKind["root"].Chunks != 0 || status.ByKind["manual"].Chunks != 2 {
		t.Errorf("byKind = %+v", status.ByKind)
	}
	if !strings.Contains(output, `"builtAt": "`) {
		t.Errorf("builtAt fehlt oder ist kein String:\n%s", output)
	}
	if !pathExistsForTest(project.KnowledgeIndexFile(root)) {
		t.Error("status hat den Index nicht angelegt")
	}

	output, err = runKnowledgeCaptured(t, "status")
	if err != nil || !strings.Contains(output, "Dateien:  2, Chunks: 2") || !strings.Contains(output, "Drift:    keine") {
		t.Errorf("status lesbar: %v\n%s", err, output)
	}

	output, err = runKnowledgeCaptured(t, "search", "CI", "abwarten", "--json")
	if err != nil {
		t.Fatalf("knowledge search: %v", err)
	}
	search := decodeTodoOutput[knowledgeSearchOutput](t, output)
	if search.Query != "CI abwarten" || len(search.Hits) != 1 || search.Hits[0].Rank != 1 || search.Hits[0].Heading != "Der Weg" || search.Hits[0].Anchor != "der-weg" {
		t.Errorf("search = %+v", search)
	}
	if strings.Contains(output, "score") {
		t.Errorf("score im Vertrag:\n%s", output)
	}
	output, err = runKnowledgeCaptured(t, "search", "CI", "--kind", "root")
	if err != nil || !strings.Contains(output, "Keine Treffer") {
		t.Errorf("search mit Filter: %v\n%s", err, output)
	}
	if _, err := runKnowledgeCaptured(t, "search", "CI", "--source", "root"); err == nil {
		t.Error("--source wurde angenommen; der Filter heißt --kind")
	}
	if _, err := runKnowledgeCaptured(t, "search", "CI", "--limit", "0"); err == nil {
		t.Error("--limit 0 wurde angenommen")
	}
	if _, err := runKnowledgeCaptured(t, "search", "--json"); err == nil {
		t.Error("leere Anfrage wurde angenommen")
	}

	output, err = runKnowledgeCaptured(t, "list", "--json")
	if err != nil {
		t.Fatalf("knowledge list: %v", err)
	}
	list := decodeTodoOutput[knowledgeListOutput](t, output)
	if len(list.Entries) != 2 || list.Entries[0].Path != "README.md" || list.Entries[1].Title != "Release" || list.Entries[1].Kind != "manual" {
		t.Errorf("list = %+v", list)
	}

	output, err = runKnowledgeCaptured(t, "read", "manual/release.md")
	if err != nil || !strings.HasPrefix(output, "# Release\n") {
		t.Errorf("read: %v\n%s", err, output)
	}
	output, err = runKnowledgeCaptured(t, "read", "manual/release.md", "--json")
	if err != nil {
		t.Fatalf("read --json: %v", err)
	}
	read := decodeTodoOutput[knowledgeReadOutput](t, output)
	if read.Path != "manual/release.md" || !strings.Contains(read.Content, "## Der Weg") {
		t.Errorf("read = %+v", read)
	}
	if _, err := runKnowledgeCaptured(t, "read", "../k-playbook.md"); err == nil {
		t.Error("Ausbruch beim Lesen wurde angenommen")
	}

	source := filepath.Join(root, "befund.md")
	if err := os.WriteFile(source, []byte("# Befund\n\nDer Wächter prüft VERSION.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	output, err = runKnowledgeCaptured(t, "write", "findings/sitzung/befund.md", "--producer", "session",
		"--title", "Befund", "--subject", "Release", "--origin", "Sitzung 42", "--state", "reviewed",
		"--source", "chat/a.md", "--source", "chat/b.md", "--file", source, "--json")
	if err != nil {
		t.Fatalf("knowledge write: %v", err)
	}
	written := decodeTodoOutput[knowledgeWriteOutput](t, output)
	if written.Path != "findings/sitzung/befund.md" || !written.Written || written.Producer != "session" {
		t.Errorf("write = %+v", written)
	}
	content, err := os.ReadFile(filepath.Join(project.KnowledgeDir(root), "findings", "sitzung", "befund.md"))
	if err != nil || !strings.HasPrefix(string(content), "---\ntitle: Befund\nsubject: Release\norigin: Sitzung 42\nstate: reviewed\nformat: markdown\nsources:\n  - chat/a.md\n  - chat/b.md\nupdated: ") {
		t.Errorf("geschriebene Datei: %v\n%s", err, content)
	}
	for name, args := range map[string][]string{
		"ohne --producer": {"write", "findings/x.md", "--title", "X", "--subject", "X", "--origin", "X", "--state", "raw", "--file", source},
		"ohne --title":    {"write", "findings/x.md", "--producer", "session", "--subject", "X", "--origin", "X", "--state", "raw", "--file", source},
		"Ausbruch":        {"write", "../x.md", "--producer", "session", "--title", "X", "--subject", "X", "--origin", "X", "--state", "raw", "--file", source},
		"fremdes Ziel":    {"write", "manual/x.md", "--producer", "session", "--title", "X", "--subject", "X", "--origin", "X", "--state", "raw", "--file", source},
		"Generator":       {"write", "code/x.md", "--producer", "docs-code", "--title", "X", "--subject", "X", "--origin", "X", "--state", "raw", "--file", source},
		"--title zweimal": {"write", "findings/x.md", "--producer", "session", "--title", "X", "--title", "Y", "--subject", "X", "--origin", "X", "--state", "raw", "--file", source},
	} {
		if _, err := runKnowledgeCaptured(t, args...); err == nil {
			t.Errorf("%s wurde angenommen", name)
		}
	}

	output, err = runKnowledgeCaptured(t, "search", "Wächter", "--kind", "findings", "--json")
	if err != nil {
		t.Fatalf("search nach write: %v", err)
	}
	search = decodeTodoOutput[knowledgeSearchOutput](t, output)
	if len(search.Hits) != 1 || search.Hits[0].Path != "findings/sitzung/befund.md" || search.Hits[0].Kind != "findings" || search.Hits[0].Origin != "Sitzung 42" || search.Hits[0].State != "reviewed" {
		t.Errorf("search nach write = %+v", search)
	}

	output, err = runKnowledgeCaptured(t, "status", "--json")
	if err != nil {
		t.Fatalf("status nach write: %v", err)
	}
	status = decodeTodoOutput[project.KnowledgeStatus](t, output)
	if status.Stale || status.FileCount != 3 {
		t.Errorf("Write als Drift gemeldet oder nicht gezählt: %+v", status)
	}
}

// publish --from liest ein Verzeichnis mit Kopf-Dateien und tauscht das
// Generatorverzeichnis; supersede löst ein Dokument ab.
func TestKnowledgePublishUndSupersede(t *testing.T) {
	root := knowledgeProject(t)
	from := filepath.Join(root, "erzeugt")
	for rel, content := range map[string]string{
		"links.md":      "---\ntitle: Verlinkung\nsubject: Symlinks\norigin: /k-docs-code\nstate: condensed\n---\n# Verlinkung\n\nKennwortchunk.\n",
		"tief/unten.md": "---\ntitle: Unten\nsubject: Tiefe\norigin: /k-docs-code\nstate: condensed\n---\n# Unten\n",
	} {
		full := filepath.Join(from, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	output, err := runKnowledgeCaptured(t, "publish", "--producer", "docs-code", "--from", from, "--json")
	if err != nil {
		t.Fatalf("knowledge publish: %v", err)
	}
	published := decodeTodoOutput[project.KnowledgePublishResult](t, output)
	if published.Producer != "docs-code" || published.Dir != "code/" || published.Written != 2 || published.Removed != 0 {
		t.Errorf("publish = %+v", published)
	}
	content, err := os.ReadFile(filepath.Join(project.KnowledgeDir(root), "code", "links.md"))
	if err != nil || !strings.HasPrefix(string(content), "---\ntitle: Verlinkung\nsubject: Symlinks\norigin: /k-docs-code\nstate: condensed\nformat: markdown\nupdated: ") {
		t.Errorf("code/links.md: %v\n%s", err, content)
	}
	output, err = runKnowledgeCaptured(t, "search", "Kennwortchunk", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if search := decodeTodoOutput[knowledgeSearchOutput](t, output); len(search.Hits) != 1 || search.Hits[0].Path != "code/links.md" {
		t.Errorf("search nach publish = %+v", search)
	}
	for name, args := range map[string][]string{
		"kein Generator":        {"publish", "--producer", "session", "--from", from},
		"ohne --from":           {"publish", "--producer", "docs-code"},
		"fehlendes Verzeichnis": {"publish", "--producer", "docs-code", "--from", filepath.Join(root, "fehlt")},
	} {
		if _, err := runKnowledgeCaptured(t, args...); err == nil {
			t.Errorf("%s wurde angenommen", name)
		}
	}

	// Ein Nachfolger unter code/ ist abgewiesen (Task 063, Entscheidung 4);
	// abgelöst wird auf ein Befunddokument. Die Antwort nennt den bereinigten
	// Nachfolger.
	if _, err := runKnowledgeCaptured(t, "supersede", "manual/release.md", "--successor", "code/links.md", "--reason", "Ersetzt"); err == nil {
		t.Error("Nachfolger unter code/ wurde angenommen")
	}
	nachfolger := filepath.Join(project.KnowledgeDir(root), "findings", "nachfolger.md")
	if err := os.MkdirAll(filepath.Dir(nachfolger), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(nachfolger, []byte("---\ntitle: N\nsubject: S\norigin: O\nstate: condensed\n---\n# N\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	output, err = runKnowledgeCaptured(t, "supersede", "manual/release.md", "--successor", "./findings/nachfolger.md", "--reason", "Ersetzt", "--json")
	if err != nil {
		t.Fatalf("knowledge supersede: %v", err)
	}
	superseded := decodeTodoOutput[knowledgeSupersedeOutput](t, output)
	if superseded.Path != "manual/release.md" || superseded.Successor != "findings/nachfolger.md" || !superseded.Superseded {
		t.Errorf("supersede = %+v", superseded)
	}
	content, err = os.ReadFile(filepath.Join(project.KnowledgeDir(root), "manual", "release.md"))
	if err != nil || !strings.Contains(string(content), "state: superseded\nsuccessor: findings/nachfolger.md\nsuperseded_reason: Ersetzt\n") {
		t.Errorf("manual/release.md: %v\n%s", err, content)
	}
	if _, err := runKnowledgeCaptured(t, "supersede", "manual/release.md", "--successor", "findings/fehlt.md", "--reason", "x"); err == nil {
		t.Error("fehlender Nachfolger wurde angenommen")
	}
	if _, err := runKnowledgeCaptured(t, "supersede", "manual/release.md", "--successor", "findings/nachfolger.md"); err == nil {
		t.Error("supersede ohne --reason wurde angenommen")
	}
}

// Eingang und Warteschlange über das Subkommando: put/list/read, add/list/drop
// und die Übernahme über write --queue.
func TestKnowledgeInboxUndQueue(t *testing.T) {
	root := knowledgeProject(t)
	raw := filepath.Join(root, "standup.md")
	if err := os.WriteFile(raw, []byte("# Standup\n\nKennwort.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	output, err := runKnowledgeCaptured(t, "inbox", "put", "chat", "standup.md", "--file", raw, "--note", "Vom 12.9.", "--json")
	if err != nil {
		t.Fatalf("inbox put: %v", err)
	}
	put := decodeTodoOutput[knowledgeInboxPutOutput](t, output)
	if put.Path != "chat/standup.md" || put.Source != "chat" || !put.Written {
		t.Errorf("put = %+v", put)
	}
	if _, err := runKnowledgeCaptured(t, "inbox", "put", "chat", "standup.md", "--file", raw); err == nil {
		t.Error("belegter Name wurde angenommen")
	}
	if _, err := runKnowledgeCaptured(t, "inbox", "put", "chat", "../x.md", "--file", raw); err == nil {
		t.Error("Ausbruch wurde angenommen")
	}
	if _, err := runKnowledgeCaptured(t, "inbox", "schnabeltier"); err == nil {
		t.Error("unbekannter inbox-Unterbefehl wurde angenommen")
	}

	output, err = runKnowledgeCaptured(t, "inbox", "list", "--json")
	if err != nil {
		t.Fatalf("inbox list: %v", err)
	}
	list := decodeTodoOutput[knowledgeInboxListOutput](t, output)
	if len(list.Entries) != 1 || list.Entries[0].Path != "chat/standup.md" || list.Entries[0].Format != "markdown" || list.Entries[0].Note != "Vom 12.9." {
		t.Errorf("list = %+v", list)
	}
	output, err = runKnowledgeCaptured(t, "inbox", "list", "scan")
	if err != nil || !strings.Contains(output, "leer") {
		t.Errorf("inbox list scan: %v\n%s", err, output)
	}
	output, err = runKnowledgeCaptured(t, "inbox", "read", "chat/standup.md")
	if err != nil || output != "# Standup\n\nKennwort.\n" {
		t.Errorf("inbox read: %v\n%q", err, output)
	}
	if _, err := runKnowledgeCaptured(t, "inbox", "read", "chat/standup.md.note"); err == nil {
		t.Error("Notiz gelesen")
	}

	output, err = runKnowledgeCaptured(t, "queue", "add", "--origin", "chat/standup.md", "--target", "findings", "--reason", "Befund", "--json")
	if err != nil {
		t.Fatalf("queue add: %v", err)
	}
	added := decodeTodoOutput[knowledgeQueueAddOutput](t, output)
	if !added.Added || !strings.HasSuffix(added.ID, "-chat-standup-md") {
		t.Errorf("add = %+v", added)
	}
	if _, err := runKnowledgeCaptured(t, "queue", "add", "--origin", "x", "--target", "../docs", "--reason", "x"); err == nil {
		t.Error("Ausbruch im target wurde angenommen")
	}
	output, err = runKnowledgeCaptured(t, "queue", "list", "--json")
	if err != nil {
		t.Fatalf("queue list: %v", err)
	}
	queue := decodeTodoOutput[knowledgeQueueListOutput](t, output)
	if len(queue.Entries) != 1 || queue.Entries[0].ID != added.ID || queue.Entries[0].Target != "findings/" || queue.Entries[0].Reason != "Befund" {
		t.Errorf("queue = %+v", queue)
	}

	if _, err := runKnowledgeCaptured(t, "write", "findings/standup.md", "--producer", "session", "--title", "Standup", "--subject", "Team",
		"--origin", "chat/standup.md", "--state", "condensed", "--source", "chat/standup.md", "--queue", added.ID, "--file", raw); err != nil {
		t.Fatalf("write mit --queue: %v", err)
	}
	output, err = runKnowledgeCaptured(t, "queue", "list")
	if err != nil || !strings.Contains(output, "Nichts offen") {
		t.Errorf("queue nach write: %v\n%s", err, output)
	}
	if !pathExistsForTest(filepath.Join(project.InboxDir(root), "chat", "standup.md")) {
		t.Error("die Übernahme hat das Rohstück aus dem Eingang entfernt")
	}

	output, err = runKnowledgeCaptured(t, "queue", "add", "--origin", "mail/x.eml", "--target", "extracted/", "--reason", "x", "--json")
	if err != nil {
		t.Fatal(err)
	}
	id := decodeTodoOutput[knowledgeQueueAddOutput](t, output).ID
	output, err = runKnowledgeCaptured(t, "queue", "drop", id, "--reason", "doch nicht", "--json")
	if err != nil {
		t.Fatalf("queue drop: %v", err)
	}
	if dropped := decodeTodoOutput[knowledgeQueueDropOutput](t, output); dropped.ID != id || !dropped.Dropped {
		t.Errorf("drop = %+v", dropped)
	}
	if _, err := runKnowledgeCaptured(t, "queue", "drop", id); err == nil {
		t.Error("zweiter drop wurde angenommen")
	}
}

// runKnowledgeStreams führt das Subkommando aus und fängt stdout und stderr
// getrennt ab: die Antwort gehört auf stdout, der Hinweis auf stderr.
func runKnowledgeStreams(t *testing.T, args ...string) (string, string, error) {
	t.Helper()

	capture := func(target **os.File) (*os.File, func() string) {
		file, err := os.CreateTemp(t.TempDir(), "strom-*")
		if err != nil {
			t.Fatalf("Ausgabedatei: %v", err)
		}
		before := *target
		*target = file
		return before, func() string {
			*target = before
			if err := file.Close(); err != nil {
				t.Fatalf("Ausgabedatei schließen: %v", err)
			}
			content, err := os.ReadFile(file.Name())
			if err != nil {
				t.Fatalf("Ausgabe lesen: %v", err)
			}
			return string(content)
		}
	}

	_, readOut := capture(&os.Stdout)
	_, readErr := capture(&os.Stderr)
	runErr := runKnowledge(args)
	return readOut(), readErr(), runErr
}

// Ein nicht beschreibbares cache/ stoppt das Subkommando nicht: die Antwort
// steht auf stdout, und was dabei nicht ging, steht auf stderr. Beides
// getrennt, damit --json für sich lesbar bleibt.
func TestKnowledgeHinweisStehtAufStderr(t *testing.T) {
	root := knowledgeProject(t)
	cache := filepath.Join(project.LocalDir(root), project.CacheDirName)
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatalf("%s anlegen: %v", cache, err)
	}
	info, err := os.Stat(cache)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(cache, 0o555); err != nil {
		t.Fatalf("%s sperren: %v", cache, err)
	}
	t.Cleanup(func() { _ = os.Chmod(cache, info.Mode().Perm()) })
	if err := os.WriteFile(filepath.Join(cache, ".schreibprobe"), []byte("x"), 0o644); err == nil {
		t.Skip("das entzogene Schreibrecht greift hier nicht")
	}

	stdout, stderr, err := runKnowledgeStreams(t, "status", "--json")
	if err != nil {
		t.Fatalf("status trotz vollständigem Index im Speicher gescheitert: %v", err)
	}
	if !strings.Contains(stdout, `"chunkCount": 2`) {
		t.Errorf("die Antwort fehlt auf stdout:\n%s", stdout)
	}
	if !strings.Contains(stderr, "Hinweis:") {
		t.Errorf("der Fehlschlag am Cache wurde still verschluckt; stderr:\n%s", stderr)
	}
	if strings.Contains(stdout, "Hinweis:") {
		t.Errorf("der Hinweis steht in der JSON-Antwort:\n%s", stdout)
	}
}

// publish --from mit einer Datei unter <dir>/code/: der Pfad trägt das
// Generatorverzeichnis und wird abgewiesen, statt nach knowledge/code/code/ zu
// gehen (Task 063, Entscheidung 1).
func TestKnowledgePublishPfadMitErzeugerverzeichnisUeberFrom(t *testing.T) {
	root := knowledgeProject(t)
	write := func(dir, rel string) {
		full := filepath.Join(root, dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("---\ntitle: X\nsubject: Y\norigin: /k-docs-code\nstate: condensed\n---\n# X\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("gut", "links.md")
	write("falsch", "code/overview.md")

	if _, err := runKnowledgeCaptured(t, "publish", "--producer", "docs-code", "--from", filepath.Join(root, "gut")); err != nil {
		t.Fatalf("erster Lauf: %v", err)
	}
	_, err := runKnowledgeCaptured(t, "publish", "--producer", "docs-code", "--from", filepath.Join(root, "falsch"))
	if err == nil || !strings.Contains(err.Error(), "relativ zu code/") {
		t.Errorf("abgewiesen mit Konvention erwartet: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(project.KnowledgeDir(root), "code", "code")); statErr == nil {
		t.Error("knowledge/code/code/ entstanden")
	}
	if _, statErr := os.Stat(filepath.Join(project.KnowledgeDir(root), "code", "links.md")); statErr != nil {
		t.Errorf("Bestand weg: %v", statErr)
	}
}
