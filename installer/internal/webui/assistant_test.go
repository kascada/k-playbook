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

// Geheilt wird nur unter der Bedingung des Briefings (Task 191): Bei einer
// Fassung, die dieses Werkzeug nicht versteht, und bei einem unzulässigen
// Wert in project.languages liest die Karte den Zustand, verlinkt aber
// nichts — und sagt, warum.
func TestAssistantHandlerHeiltNichtOhneAkzeptierteKonfiguration(t *testing.T) {
	for _, test := range []struct {
		name, config, reason string
	}{
		{"unbekannte Fassung", "schema_version: 99\n\nproject:\n  repo_root: .\n  vcs: git\n", "schema_version"},
		{"ältere Fassung", "schema_version: 1\n\nproject:\n  repo_root: .\n  vcs: git\n", "schema_version"},
		{"unzulässiges project.languages", "schema_version: " + project.SchemaVersion +
			"\n\nproject:\n  repo_root: .\n  vcs: git\n  languages: [go, \"c sharp\"]\n", "project.languages"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := assistantProject(t)
			writeFile(t, project.ConfigPath(root), test.config)
			before := treeListing(t, root)

			response := getAssistant(t)
			if project.LinksOK(response.Entries) {
				t.Errorf("die Verlinkung steht, obwohl nichts nachgezogen werden durfte: %+v", response.Entries)
			}
			if len(response.Entries) == 0 {
				t.Error("der Zustand wird trotzdem gelesen")
			}
			if !strings.Contains(response.Message, "nicht nachgezogen") || !strings.Contains(response.Message, test.reason) {
				t.Errorf("Message nennt den Grund nicht: %q", response.Message)
			}
			for _, name := range []string{".claude", ".opencode", "CLAUDE.md", "AGENTS.md"} {
				if _, err := os.Lstat(filepath.Join(root, name)); !os.IsNotExist(err) {
					t.Errorf("%s wurde angelegt (%v)", name, err)
				}
			}
			if after := treeListing(t, root); strings.Join(before, "\n") != strings.Join(after, "\n") {
				t.Errorf("der Abruf hat das Projekt verändert:\nvorher:  %v\nnachher: %v", before, after)
			}

			// Mit der akzeptierten Konfiguration zieht derselbe Abruf nach.
			writeFile(t, project.ConfigPath(root),
				"schema_version: "+project.SchemaVersion+"\n\nproject:\n  repo_root: .\n  vcs: git\n")
			if response := getAssistant(t); !project.LinksOK(response.Entries) {
				t.Errorf("mit akzeptierter Konfiguration muss die Verlinkung stehen: %+v", response.Entries)
			}
		})
	}
}

// treeListing listet alle Einträge unter root, Links ohne ihnen zu folgen.
func treeListing(t *testing.T, root string) []string {
	t.Helper()
	var entries []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		entries = append(entries, path+" "+entry.Type().String())
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return entries
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

// Die Seiten „Assistenten“ und „MCP“ zeigen ihren Zustand ohne Installation
// im Projekt, statt mit „Installationsverzeichnis … fehlt“ abzubrechen.
func TestAssistentUndMCPOhneInstallation(t *testing.T) {
	assistantProject(t)

	assistant := getAssistant(t)
	// Die Umgebung in der Antwort nennt keinen Ort einer Installation im
	// Projekt mehr (Task 157).
	if encoded, err := json.Marshal(assistant.Environment); err != nil {
		t.Fatalf("Environment kodieren: %v", err)
	} else if strings.Contains(string(encoded), "playbookDir") {
		t.Errorf("Environment trägt playbookDir: %s", encoded)
	}
	if strings.Contains(assistant.Message, "Installationsverzeichnis") {
		t.Errorf("Assistenten: Message = %q, erwartet keine Sperre", assistant.Message)
	}
	if len(assistant.Entries) == 0 || assistant.Root.Path == "" {
		t.Errorf("Assistenten: kein Zustand gemessen: %+v", assistant)
	}

	recorder := httptest.NewRecorder()
	mcpHandler(recorder, httptest.NewRequest(http.MethodGet, "/api/mcp", nil))
	var mcp mcpResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &mcp); err != nil {
		t.Fatalf("Antwort nicht lesbar: %v — %s", err, recorder.Body.String())
	}
	if strings.Contains(mcp.Message, "Installationsverzeichnis") {
		t.Errorf("MCP: Message = %q, erwartet keine Sperre", mcp.Message)
	}
	if len(mcp.Entries) == 0 {
		t.Errorf("MCP: kein Zustand gemessen: %+v", mcp)
	}
}
