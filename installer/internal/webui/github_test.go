package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/github"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// getGitHubAPI ruft einen der GitHub-Endpunkte über routes() — denselben Mux
// wie im Betrieb — und liest Zustand und Erklärung aus der Antwort.
func getGitHubAPI(t *testing.T, path string) github.Result {
	t.Helper()

	recorder := httptest.NewRecorder()
	routes(&serverState{}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("%s: Status %d, erwartet 200", path, recorder.Code)
	}
	var result github.Result
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("%s: Antwort nicht lesbar: %v", path, err)
	}
	return result
}

// githubEndpoints sind alle Endpunkte der Ansicht. Jeder steht hinter
// derselben Vorprüfung, und jeder muss sie einhalten.
var githubEndpoints = []string{
	"/api/github/overview",
	"/api/github/pulls",
	"/api/github/runs",
	"/api/github/runs/35002675449/failure",
}

// Die Vorprüfung entscheidet aus Dateien, bevor irgendein Subprozess startet.
// Steht tools.gh.status nicht auf enabled, ruft der Server gh gar nicht auf —
// das ist die Zusage des Zustands, nicht bloß eine andere Beschriftung.
func TestGitHubAPIFragtOhneEntscheidungNicht(t *testing.T) {
	tests := []struct {
		name   string
		status project.GHStatus
		want   github.State
	}{
		{name: "abgeschaltet", status: project.GHDisabled, want: github.StateDisabled},
		{name: "unentschieden", status: project.GHUnknown, want: github.StateUndecided},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			root := t.TempDir()
			if err := project.CreateConfig(root, "."); err != nil {
				t.Fatalf("Konfiguration anlegen: %v", err)
			}
			if err := project.SetGHStatus(root, testCase.status); err != nil {
				t.Fatalf("tools.gh.status setzen: %v", err)
			}
			chdir(t, root)

			for _, path := range githubEndpoints {
				result := getGitHubAPI(t, path)
				if result.State != testCase.want {
					t.Errorf("%s: Zustand %q, erwartet %q", path, result.State, testCase.want)
				}
				if result.Message == "" {
					t.Errorf("%s: Zustand ohne Erklärung", path)
				}
			}
		})
	}
}

// Ohne K-PLAYBOOK.yaml gibt es kein Projekt, auf das sich die Ansicht beziehen
// könnte. Auch das ist ein erklärter Zustand und keine leere Seite.
func TestGitHubAPIOhneProjekt(t *testing.T) {
	chdir(t, t.TempDir())

	for _, path := range githubEndpoints {
		result := getGitHubAPI(t, path)
		if result.State != github.StateNoProject {
			t.Errorf("%s: Zustand %q, erwartet %q", path, result.State, github.StateNoProject)
		}
		if result.Message == "" {
			t.Errorf("%s: Zustand ohne Erklärung", path)
		}
	}
}
