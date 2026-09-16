package webui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

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

// enableGitHub legt ein Projekt mit tools.gh.status: enabled an, in dem die
// Vorprüfung durchgeht, und setzt runner als Runner jedes Clients ein.
//
// Die Vorprüfung liest nur Dateien und die Umgebung: ein ausführbares `gh` im
// PATH und ein Token in GH_TOKEN genügen ihr. Aufgerufen wird dieses `gh` nie —
// jeder Subprozess geht an den eingesetzten Runner.
func enableGitHub(t *testing.T, runner github.Runner) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("das gh im PATH ist ein Shell-Skript")
	}

	root := t.TempDir()
	if err := project.CreateConfig(root, "."); err != nil {
		t.Fatalf("Konfiguration anlegen: %v", err)
	}
	if err := project.SetGHStatus(root, project.GHEnabled); err != nil {
		t.Fatalf("tools.gh.status setzen: %v", err)
	}
	chdir(t, root)

	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte("#!/bin/sh\nexit 99\n"), 0o755); err != nil {
		t.Fatalf("gh anlegen: %v", err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("GH_CONFIG_DIR", t.TempDir())
	t.Setenv("GH_TOKEN", "nur-fuer-die-vorpruefung")

	before := newGitHubClient
	newGitHubClient = func(dir string) *github.Client {
		return &github.Client{Runner: runner, Dir: dir, Timeout: github.DefaultTimeout}
	}
	t.Cleanup(func() { newGitHubClient = before })
}

// setGitHubBudgets setzt das Budget aller Endpunkte für einen Test.
func setGitHubBudgets(t *testing.T, budget time.Duration) {
	t.Helper()
	before := githubBudgets
	githubBudgets.Overview = budget
	githubBudgets.Pulls = budget
	githubBudgets.Runs = budget
	githubBudgets.Failure = budget
	t.Cleanup(func() { githubBudgets = before })
}

// blockingRunner antwortet nie. Er kehrt erst zurück, wenn der Kontext abläuft,
// und meldet dann, was ExecRunner in diesem Fall meldet: einen getöteten
// Prozess.
type blockingRunner struct{}

func (blockingRunner) Run(ctx context.Context, _ string, name string, args ...string) ([]byte, error) {
	<-ctx.Done()
	return nil, &github.CommandError{Name: name, Args: args, ExitCode: -1, Err: errors.New("signal: killed")}
}

// slowRunner antwortet richtig, aber jeder Aufruf dauert delay. Einzeln bleibt
// jeder weit unter der Frist je Subprozess; erst ihre Summe überschreitet das
// Budget.
type slowRunner struct {
	delay time.Duration
}

func (s slowRunner) Run(ctx context.Context, _ string, name string, args ...string) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, &github.CommandError{Name: name, Args: args, ExitCode: -1, Err: errors.New("signal: killed")}
	case <-time.After(s.delay):
	}
	call := name + " " + strings.Join(args, " ")
	switch {
	case strings.HasPrefix(call, "gh repo view"):
		return []byte(`{"nameWithOwner":"kascada/k-playbook","defaultBranchRef":{"name":"main"},"viewerPermission":"READ"}`), nil
	case strings.HasPrefix(call, "gh api user"):
		return []byte(`{"login":"kkl_kmde"}`), nil
	case strings.HasPrefix(call, "gh run list"):
		return []byte(`[]`), nil
	case strings.HasPrefix(call, "git remote"):
		return []byte("git@github.com:kascada/k-playbook.git\n"), nil
	case strings.HasPrefix(call, "git describe"):
		return []byte("v0.9.0\n"), nil
	case strings.HasPrefix(call, "git rev-list"):
		return []byte("3\n"), nil
	case strings.HasPrefix(call, "git log"):
		return []byte("2026-09-15T19:35:12+02:00\n"), nil
	}
	return nil, fmt.Errorf("unerwarteter Aufruf: %s", call)
}

// Die Frist gilt je Endpunkt, nicht je Subprozess. Antwortet gh nie, ist jede
// Anfrage nach ihrem Budget beendet — hier 100 ms statt der 20 s, die jeder
// einzelne Aufruf für sich hätte.
func TestGitHubAPIEndetNachDemBudget(t *testing.T) {
	enableGitHub(t, blockingRunner{})
	const budget = 100 * time.Millisecond
	setGitHubBudgets(t, budget)

	for _, path := range githubEndpoints {
		started := time.Now()
		result := getGitHubAPI(t, path)
		elapsed := time.Since(started)

		if elapsed > budget+time.Second {
			t.Errorf("%s: Antwort nach %s, Budget %s", path, elapsed, budget)
		}
		if result.State != github.StateTimeout {
			t.Errorf("%s: Zustand %q, erwartet %q (%s)", path, result.State, github.StateTimeout, result.Message)
		}
	}
}

// Die Kopfzeile ruft sieben Subprozesse nacheinander. Jeder bleibt einzeln
// unter jeder Frist, zusammen brauchten sie 1,05 s — das Budget von 300 ms
// beendet die Anfrage trotzdem.
func TestGitHubKopfzeileHaeltDasBudgetUeberAlleAufrufe(t *testing.T) {
	enableGitHub(t, slowRunner{delay: 150 * time.Millisecond})
	const budget = 300 * time.Millisecond
	setGitHubBudgets(t, budget)

	started := time.Now()
	result := getGitHubAPI(t, "/api/github/overview")
	elapsed := time.Since(started)

	if elapsed >= 7*150*time.Millisecond {
		t.Fatalf("Antwort nach %s: die Aufrufe liefen ohne Budget durch", elapsed)
	}
	if elapsed > budget+500*time.Millisecond {
		t.Errorf("Antwort nach %s, Budget %s", elapsed, budget)
	}
	// Der erste Aufruf ist gelungen: das Repo steht fest, der Zustand bleibt ok.
	if result.State != github.StateOK {
		t.Errorf("Zustand %q, erwartet ok (%s)", result.State, result.Message)
	}
}

// Bricht der Browser die Anfrage ab, antwortet der Endpunkt still: kein
// Zeitfehler, kein Fehler mit roher gh-Zeile — gar keine Antwort, denn es hört
// niemand mehr zu.
func TestGitHubAPIBeantwortetAbgebrocheneAnfrageNicht(t *testing.T) {
	enableGitHub(t, blockingRunner{})
	setGitHubBudgets(t, 5*time.Second)

	for _, path := range githubEndpoints {
		ctx, cancel := context.WithCancel(context.Background())
		time.AfterFunc(20*time.Millisecond, cancel)
		recorder := httptest.NewRecorder()

		started := time.Now()
		routes(&serverState{}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil).WithContext(ctx))
		if elapsed := time.Since(started); elapsed > time.Second {
			t.Errorf("%s: Handler lief nach dem Abbruch %s weiter", path, elapsed)
		}
		if body := recorder.Body.String(); body != "" {
			t.Errorf("%s: Antwort auf eine abgebrochene Anfrage: %s", path, body)
		}
		cancel()
	}
}
