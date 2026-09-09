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
// Index wird dabei nicht angelegt.
func TestKnowledgeHilfeIstFolgenlos(t *testing.T) {
	root := knowledgeProject(t)

	for _, args := range [][]string{{}, {"--help"}, {"help"}, {"-h"}} {
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
}

func TestKnowledgeStatusSearchListReadWrite(t *testing.T) {
	root := knowledgeProject(t)

	output, err := runKnowledgeCaptured(t, "status", "--json")
	if err != nil {
		t.Fatalf("knowledge status: %v", err)
	}
	status := decodeTodoOutput[project.KnowledgeStatus](t, output)
	if status.IndexKind != "bm25" || status.FileCount != 2 || status.ChunkCount != 3 || status.Stale {
		t.Errorf("status = %+v", status)
	}
	if status.BySource["root"].Files != 1 || status.BySource["manual"].Chunks != 2 {
		t.Errorf("bySource = %+v", status.BySource)
	}
	if !strings.Contains(output, `"builtAt": "`) {
		t.Errorf("builtAt fehlt oder ist kein String:\n%s", output)
	}
	if !pathExistsForTest(project.KnowledgeIndexFile(root)) {
		t.Error("status hat den Index nicht angelegt")
	}

	output, err = runKnowledgeCaptured(t, "status")
	if err != nil || !strings.Contains(output, "Dateien:  2, Chunks: 3") || !strings.Contains(output, "Drift:    keine") {
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
	output, err = runKnowledgeCaptured(t, "search", "CI", "--source", "root")
	if err != nil || !strings.Contains(output, "Keine Treffer") {
		t.Errorf("search mit Filter: %v\n%s", err, output)
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
	if len(list.Entries) != 2 || list.Entries[0].Path != "README.md" || list.Entries[1].Title != "Release" || list.Entries[1].Source != "manual" {
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
	output, err = runKnowledgeCaptured(t, "write", "sitzung/befund.md", "--source", "Sitzung 42", "--file", source, "--json")
	if err != nil {
		t.Fatalf("knowledge write: %v", err)
	}
	written := decodeTodoOutput[knowledgeWriteOutput](t, output)
	if written.Path != "learned/sitzung/befund.md" || !written.Written || written.Source != "Sitzung 42" {
		t.Errorf("write = %+v", written)
	}
	content, err := os.ReadFile(filepath.Join(project.KnowledgeLearnedDir(root), "sitzung", "befund.md"))
	if err != nil || !strings.HasPrefix(string(content), "---\nsource: Sitzung 42\n---\n") {
		t.Errorf("geschriebene Datei: %v\n%s", err, content)
	}
	if _, err := runKnowledgeCaptured(t, "write", "x.md", "--file", source); err == nil {
		t.Error("write ohne --source wurde angenommen")
	}
	if _, err := runKnowledgeCaptured(t, "write", "../x.md", "--source", "s", "--file", source); err == nil {
		t.Error("Ausbruch beim Schreiben wurde angenommen")
	}

	output, err = runKnowledgeCaptured(t, "search", "Wächter", "--source", "learned", "--json")
	if err != nil {
		t.Fatalf("search nach write: %v", err)
	}
	search = decodeTodoOutput[knowledgeSearchOutput](t, output)
	if len(search.Hits) != 1 || search.Hits[0].Path != "learned/sitzung/befund.md" || search.Hits[0].Source != "learned" {
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
	if !strings.Contains(stdout, `"chunkCount": 3`) {
		t.Errorf("die Antwort fehlt auf stdout:\n%s", stdout)
	}
	if !strings.Contains(stderr, "Hinweis:") {
		t.Errorf("der Fehlschlag am Cache wurde still verschluckt; stderr:\n%s", stderr)
	}
	if strings.Contains(stdout, "Hinweis:") {
		t.Errorf("der Hinweis steht in der JSON-Antwort:\n%s", stdout)
	}
}
