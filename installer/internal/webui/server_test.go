package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/guiproc"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// chdir macht ein Verzeichnis zum Arbeitsverzeichnis des Tests: die Handler
// leiten ihr Projekt über project.Detect() daraus ab.
func chdir(t *testing.T, dir string) {
	t.Helper()

	before, err := os.Getwd()
	if err != nil {
		t.Fatalf("Arbeitsverzeichnis: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("nach %s wechseln: %v", dir, err)
	}
	t.Cleanup(func() { os.Chdir(before) })
}

// getPage holt eine Seite über routes() — also über denselben Mux wie im
// Betrieb, samt Zuordnung von Pfad zu Handler und Vorlage.
func getPage(t *testing.T, path string) (int, string) {
	t.Helper()

	recorder := httptest.NewRecorder()
	routes(&serverState{}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	return recorder.Code, recorder.Body.String()
}

// Jede Seite trägt dasselbe Fragment der linken Spalte, und darin ist genau
// ein Bereich markiert. Ohne diesen Test fällt eine vergessene area-Übergabe,
// ein umbenanntes {{define "sidebar"}} oder eine Seite ohne
// {{template "sidebar" .}} erst im Browser auf.
//
// Geprüft wird Vorhandensein und Markierung, nicht die Vollständigkeit der
// Spalte: was später zusätzlich hineingesetzt wird, darf den Test nicht
// brechen.
func TestSeitenTragenDieLinkeSpalte(t *testing.T) {
	root := t.TempDir()
	if err := project.CreateConfig(root, "."); err != nil {
		t.Fatalf("Konfiguration anlegen: %v", err)
	}
	chdir(t, root)

	tests := []struct {
		path string
		// markiert ist der Eintrag des aktiven Bereichs, samt aria-current:
		// "page" nur auf der offenen Seite selbst.
		markiert string
		// fileIndex sagt, ob das Blockmenü den Dateiindex trägt und deshalb
		// auch schmal stehen bleibt.
		fileIndex bool
	}{
		{path: "/", markiert: `<a class="area-nav-item active" href="/" aria-current="page">`},
		{path: "/workflows", markiert: `<a class="area-nav-item active" href="/workflows" aria-current="page">`},
		{path: "/docs", markiert: `<a class="area-nav-item active" href="/docs" aria-current="page">`, fileIndex: true},
		// /mcp ist die Detailseite des Setup-Blocks: der Bereich ist aktiv,
		// die Startseite darunter ist aber nicht offen.
		{path: "/mcp", markiert: `<a class="area-nav-item active" href="/" aria-current="true">`},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			status, body := getPage(t, test.path)
			if status != http.StatusOK {
				t.Fatalf("Status = %d, erwartet %d", status, http.StatusOK)
			}
			if !strings.Contains(body, `class="area-nav-item`) {
				t.Error("der Umschalter fehlt")
			}
			if !strings.Contains(body, `id="block-nav"`) {
				t.Error("das Blockmenü fehlt")
			}
			if count := strings.Count(body, "area-nav-item active"); count != 1 {
				t.Errorf("markierte Bereiche = %d, erwartet genau 1", count)
			}
			if !strings.Contains(body, test.markiert) {
				t.Errorf("der markierte Eintrag ist nicht %s", test.markiert)
			}
			if got := strings.Contains(body, "block-nav file-index"); got != test.fileIndex {
				t.Errorf("Modifier file-index = %v, erwartet %v", got, test.fileIndex)
			}
		})
	}
}

// Ohne Konfiguration führt der Umschalter nur nach Setup: Workflows und Docs
// hätten dort nichts zu zeigen.
func TestUmschalterOhneInstallation(t *testing.T) {
	chdir(t, t.TempDir())

	status, body := getPage(t, "/")
	if status != http.StatusOK {
		t.Fatalf("Status = %d, erwartet %d", status, http.StatusOK)
	}
	if count := strings.Count(body, `class="area-nav-item`); count != 1 {
		t.Errorf("Einträge im Umschalter = %d, erwartet genau 1", count)
	}
	if strings.Contains(body, `href="/workflows"`) || strings.Contains(body, `href="/docs"`) {
		t.Error("der Umschalter führt nach Workflows oder Docs, obwohl nichts eingerichtet ist")
	}
}

// /api/health nennt Schlüssel, Version und PID: daran erkennt ein CLI-Aufruf
// den Server als seinen eigenen. Der Schlüssel ist das aufgelöste ProjectDir,
// die Version die vom Start.
func TestHealthNenntSchluesselVersionUndPID(t *testing.T) {
	root := t.TempDir()
	if err := project.CreateConfig(root, "."); err != nil {
		t.Fatalf("Konfiguration anlegen: %v", err)
	}
	chdir(t, root)

	recorder := httptest.NewRecorder()
	routes(&serverState{version: "v1.2.3"}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("Status = %d, erwartet %d", recorder.Code, http.StatusOK)
	}

	var health guiproc.Health
	if err := json.Unmarshal(recorder.Body.Bytes(), &health); err != nil {
		t.Fatalf("Antwort lesen: %v", err)
	}
	if health.Status != "ok" {
		t.Errorf("status = %q", health.Status)
	}
	want, err := guiproc.Key()
	if err != nil {
		t.Fatalf("Schlüssel: %v", err)
	}
	if health.Key != want {
		t.Errorf("key = %q, erwartet %q", health.Key, want)
	}
	if health.Version != "v1.2.3" {
		t.Errorf("version = %q", health.Version)
	}
	if health.PID != os.Getpid() {
		t.Errorf("pid = %d, erwartet %d", health.PID, os.Getpid())
	}
}
