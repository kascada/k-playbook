package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// Eine Doku-Datei kann einen Frontmatter-Block tragen — wissensablage.md tut
// es. Er gehört nicht in die Ansicht: Goldmark läse ihn sonst als Trennlinie
// mit anschließender Überschrift, und die Datei begänne mit ihren eigenen
// Kopfdaten statt mit ihrem Text.
func TestDokuOhneFrontmatterGerendert(t *testing.T) {
	root := t.TempDir()
	if err := project.CreateConfig(root, "."); err != nil {
		t.Fatalf("Konfiguration anlegen: %v", err)
	}
	docs := project.DocsDir(root)
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatalf("Doku-Verzeichnis anlegen: %v", err)
	}
	datei := "wissensablage.md"
	inhalt := "---\ntitle: Knowledge Storage\ndescription: Weg des Wissens.\n---\n\n# Knowledge Storage\n\nWohin das Wissen fließt.\n"
	if err := os.WriteFile(filepath.Join(docs, datei), []byte(inhalt), 0o644); err != nil {
		t.Fatalf("Datei schreiben: %v", err)
	}
	chdir(t, root)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/docs/file?path="+datei, nil)
	routes(&serverState{}).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("Status = %d, erwartet %d", recorder.Code, http.StatusOK)
	}

	var response docResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Antwort lesen: %v", err)
	}
	if response.Message != "" {
		t.Fatalf("Meldung = %q, erwartet keine", response.Message)
	}
	if strings.Contains(response.HTML, "title:") || strings.Contains(response.HTML, "description:") {
		t.Errorf("das Frontmatter steht in der Ansicht: %s", response.HTML)
	}
	if !strings.Contains(response.HTML, "<h1") || !strings.Contains(response.HTML, "Knowledge Storage") {
		t.Errorf("die Überschrift der Datei fehlt: %s", response.HTML)
	}
	// Der Titel kommt weiterhin aus der ersten Überschrift, nicht aus dem
	// abgetrennten Block.
	if response.Title != "Knowledge Storage" {
		t.Errorf("Titel = %q", response.Title)
	}
}
