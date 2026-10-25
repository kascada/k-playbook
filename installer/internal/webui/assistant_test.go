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
	"github.com/kascada/k-playbook/installer/internal/sharedir/sharetest"
)

// assistantProject baut ein Projekt auf und macht es zum Arbeitsverzeichnis:
// die Handler leiten ihr Projekt über project.Detect() daraus ab.
func assistantProject(t *testing.T) string {
	t.Helper()

	root := newProject(t)
	writeFile(t, project.ConfigPath(root),
		"schema_version: "+project.SchemaVersion+"\n\nproject:\n  repo_root: .\n  vcs: git\n")

	before, err := os.Getwd()
	if err != nil {
		t.Fatalf("Arbeitsverzeichnis: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("nach %s wechseln: %v", root, err)
	}
	t.Cleanup(func() { os.Chdir(before) })
	return root
}

func getAssistant(t *testing.T) assistantResponse {
	t.Helper()

	recorder := httptest.NewRecorder()
	assistantHandler(recorder, httptest.NewRequest(http.MethodGet, "/api/assistant", nil))

	var response assistantResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Antwort nicht lesbar: %v — %s", err, recorder.Body.String())
	}
	return response
}

// Der Statusabruf der Karte richtet ein, was sich einrichten lässt. Sonst
// stünde die Abweichung nur da und bliebe, bis jemand sie wegklickt.
func TestAssistantHandlerHeiltBeimLesen(t *testing.T) {
	root := assistantProject(t)

	// Geprüft wird die Verlinkung, nicht response.OK: das verlangt zusätzlich
	// eine AGENTS.md mit dem Anstoß, und die anzulegen bleibt dem Knopf.
	response := getAssistant(t)
	if !project.LinksOK(response.Entries) {
		t.Errorf("nach dem Lesen muss die Verlinkung stehen, war: %+v", response.Entries)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "commands", "k-test.md")); err != nil {
		t.Errorf("der Command ist nicht registriert: %v", err)
	}
	if response.Message == "" {
		t.Error("was eingerichtet wurde, gehört in die Meldung")
	}

	// Beim zweiten Abruf gibt es nichts mehr zu melden.
	if again := getAssistant(t); again.Message != "" {
		t.Errorf("Message = %q, erwartet leer", again.Message)
	}
}

// Ein umbenannter Command hinterlässt einen Link ins Leere und eine Lücke.
// Beides soll der nächste Blick auf die Karte bereits behoben haben.
func TestAssistantHandlerZiehtUmbenennungNach(t *testing.T) {
	root := assistantProject(t)
	getAssistant(t)

	commands := filepath.Join(sharetest.Dir(t), "commands")
	if err := os.Rename(filepath.Join(commands, "k-test.md"), filepath.Join(commands, "k-task-test.md")); err != nil {
		t.Fatalf("umbenennen: %v", err)
	}

	response := getAssistant(t)
	if !project.LinksOK(response.Entries) {
		t.Errorf("die Umbenennung muss nachgezogen sein, war: %+v", response.Entries)
	}
	if response.Message == "" {
		t.Error("die Bilanz gehört in die Meldung")
	}
	if _, err := os.Lstat(filepath.Join(root, ".claude", "commands", "k-test.md")); !os.IsNotExist(err) {
		t.Error("der verwaiste Link muss verschwunden sein")
	}
}

// Die Karte nennt, was das Einrichten an AGENTS.md nachgezogen hat: den neuen
// Aufruf beim ersetzten Anstoß und die entfernten Blöcke mit dem Grund. Der
// Hinweis auf einen Block ohne Grenze kommt als Fehler des Einrichtens in den
// Text und steht deshalb nicht noch einmal hier.
func TestDescribeSetupNenntNachzugAnAgents(t *testing.T) {
	setup := project.AssistantSetup{
		RootRefreshed:        true,
		RootAnstossRefreshed: true,
		RootBlocksRemoved:    []string{"Session-Memory-Block", "Befunde-Block"},
	}
	setup.Root.Hint = "das Ende des abgelösten Befunde-Blocks ließ sich nicht bestimmen"

	text := describeSetup(setup)
	if strings.Contains(text, setup.Root.Hint) {
		t.Errorf("der Hinweis stünde doppelt im Text: %s", text)
	}
	for _, erwartet := range []string{
		"auf den aktuellen Aufruf gebracht (k-playbook briefing)",
		"Session-Memory-Block und Befunde-Block aus AGENTS.md entfernt; der Inhalt kommt mit dem Briefing.",
	} {
		if !strings.Contains(text, erwartet) {
			t.Errorf("%q fehlt in: %s", erwartet, text)
		}
	}
	if strings.Contains(text, "k-playbook context") {
		t.Errorf("der abgelöste Aufruf steht in der Meldung: %s", text)
	}
}
