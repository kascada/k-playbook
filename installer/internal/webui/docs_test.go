package webui

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/kephalaion/kephtest"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// Eine Doku-Datei kann einen Frontmatter-Block tragen — die meisten unter
// docs/ tun es. Er gehört nicht in die Ansicht: Goldmark läse ihn sonst als Trennlinie
// mit anschließender Überschrift, und die Datei begänne mit ihren eigenen
// Kopfdaten statt mit ihrem Text.
func TestDokuOhneFrontmatterGerendert(t *testing.T) {
	root := t.TempDir()
	if err := project.CreateConfig(root, "."); err != nil {
		t.Fatalf("Konfiguration anlegen: %v", err)
	}
	datei := "beispiel.md"
	inhalt := "---\ntitle: Beispiel\ndescription: Eine Seite mit Kopf.\n---\n\n# Beispiel\n\nDer Text der Seite.\n"
	stubKPlaybookHandbook(t, map[string]string{"docs/handbook/" + datei: inhalt})
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
	if !strings.Contains(response.HTML, "<h1") || !strings.Contains(response.HTML, "Beispiel") {
		t.Errorf("die Überschrift der Datei fehlt: %s", response.HTML)
	}
	// Der Titel kommt weiterhin aus der ersten Überschrift, nicht aus dem
	// abgetrennten Block.
	if response.Title != "Beispiel" {
		t.Errorf("Titel = %q", response.Title)
	}
}

// stubKPlaybookHandbook setzt für einen Test das Handbuch von k-playbook aus
// einer öffentlichen Collection im Speicher ein: VERSION und die gegebenen
// Dateien unter defaults/. Kein Test fragt den Node dieses Rechners.
func stubKPlaybookHandbook(t *testing.T, files map[string]string) {
	t.Helper()
	store := kephtest.New()
	store.Put("defaults/VERSION", "v0.0.0-test\n")
	for name, content := range files {
		store.Put("defaults/"+name, content)
	}
	original := openKPlaybookHandbook
	openKPlaybookHandbook = func(string) project.Handbook {
		source := project.NewKephDefaultsSource(func() (project.DefaultsStore, string, error) {
			return store, "test:k-playbook", nil
		})
		return project.KPlaybookHandbookFrom(source, "test:k-playbook")
	}
	t.Cleanup(func() { openKPlaybookHandbook = original })
}

// getDocsList fragt eine der beiden Listen-APIs ab.
func getDocsList(t *testing.T, path string) docsResponse {
	t.Helper()
	recorder := httptest.NewRecorder()
	routes(&serverState{}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("%s: Status = %d", path, recorder.Code)
	}
	var response docsResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("%s: Antwort lesen: %v", path, err)
	}
	return response
}

// handbookFixture legt ein Projekt an und wechselt hinein.
func handbookFixture(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	if err := project.CreateConfig(root, "."); err != nil {
		t.Fatalf("Konfiguration anlegen: %v", err)
	}
	chdir(t, root)
}

// /api/docs: das Handbuch von k-playbook mit overview.md vorn; fehlt es im
// Lieferumfang oder ist Kephalaion nicht erreichbar, eine Meldung statt der
// Liste, unterschieden über code.
func TestAPIDocsHandbuchVonKPlaybook(t *testing.T) {
	handbookFixture(t)

	stubKPlaybookHandbook(t, map[string]string{
		"docs/handbook/overview.md": "# Overview\n",
		"docs/handbook/faq.md":      "# FAQ\n",
	})
	response := getDocsList(t, "/api/docs")
	if !response.Available || response.Message != "" || len(response.Docs) != 2 || response.Docs[0].Path != "overview.md" {
		t.Errorf("vorhanden: %+v", response)
	}

	stubKPlaybookHandbook(t, map[string]string{"docs/manual/README.md": "# Vorlage\n"})
	response = getDocsList(t, "/api/docs")
	if response.Code != project.HandbookCodeMissing || len(response.Docs) != 0 || !strings.Contains(response.Message, "fetch_release") {
		t.Errorf("Lieferumfang ohne Handbuch: %+v", response)
	}

	original := openKPlaybookHandbook
	openKPlaybookHandbook = func(string) project.Handbook {
		source := project.NewKephDefaultsSource(func() (project.DefaultsStore, string, error) {
			return nil, "test:k-playbook", errors.New("Kephalaion ist für dieses Projekt nicht bereit")
		})
		return project.KPlaybookHandbookFrom(source, "test:k-playbook")
	}
	t.Cleanup(func() { openKPlaybookHandbook = original })
	response = getDocsList(t, "/api/docs")
	if response.Code != project.DocCodeKephalaionUnavailable || len(response.Docs) != 0 || response.Message == "" {
		t.Errorf("ohne Kephalaion: %+v", response)
	}
}

// /api/handbook: das Handbuch des Projekts aus keph:/docs/manual/handbook/,
// README.md vorn; ohne Verzeichnis oder ohne Kephalaion eine Meldung.
func TestAPIHandbookDesProjekts(t *testing.T) {
	handbookFixture(t)
	stub := func(open func() (project.DefaultsStore, string, error)) {
		original := openProjectHandbook
		openProjectHandbook = func(string) project.Handbook {
			return project.ProjectHandbookFrom(project.NewKephProjectHandbookSource(open), "test:projekt")
		}
		t.Cleanup(func() { openProjectHandbook = original })
	}

	store := kephtest.New()
	store.Put("docs/manual/handbook/README.md", "---\ntitle: \"Project handbook\"\n---\n\n# Project handbook\n")
	store.Put("docs/manual/handbook/betrieb.md", "# Betrieb\n")
	stub(func() (project.DefaultsStore, string, error) { return store, "test:projekt", nil })
	response := getDocsList(t, "/api/handbook")
	if response.Message != "" || len(response.Docs) != 2 || response.Docs[0].Path != "README.md" {
		t.Errorf("mit README: %+v", response)
	}
	recorder := httptest.NewRecorder()
	routes(&serverState{}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/handbook/file?path=README.md", nil))
	var page docResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &page); err != nil || page.Message != "" ||
		!strings.Contains(page.HTML, "Project handbook") || strings.Contains(page.HTML, "title:") {
		t.Errorf("Startseite: %+v %v", page, err)
	}

	empty := kephtest.New()
	stub(func() (project.DefaultsStore, string, error) { return empty, "test:projekt", nil })
	response = getDocsList(t, "/api/handbook")
	if response.Code != project.HandbookCodeMissing || len(response.Docs) != 0 {
		t.Errorf("ohne Verzeichnis: %+v", response)
	}

	stub(func() (project.DefaultsStore, string, error) {
		return nil, "test:projekt", errors.New("Kephalaion ist für dieses Projekt nicht bereit")
	})
	response = getDocsList(t, "/api/handbook")
	if response.Code != project.DocCodeKephalaionUnavailable || len(response.Docs) != 0 {
		t.Errorf("ohne Kephalaion: %+v", response)
	}
}
