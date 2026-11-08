package webui

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/github"
	"github.com/kascada/k-playbook/installer/internal/githubaccess"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// routeFakeToken ist ein Fake-Token. Er darf nur in GH_TOKEN/GITHUB_TOKEN der
// Kindumgebung stehen — nie im JSON, in einer Meldung, im Log oder in einem
// Argument.
const routeFakeToken = "ghp_fake_ui_token_126"

// serviceProject legt ein Projekt an, dessen
// Repo das Remote remote hat, und eine Zugangsdatei mit config. Im PATH
// liegen ein gh, das nie laufen darf, und git. Zurück kommt das
// Projektverzeichnis.
func serviceProject(t *testing.T, remote string, config string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("das gh im PATH ist ein Shell-Skript")
	}
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git fehlt")
	}
	root := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", "-b", "main", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	if remote != "" {
		if out, err := exec.Command("git", "-C", root, "remote", "add", "origin", remote).CombinedOutput(); err != nil {
			t.Fatalf("git remote add: %v\n%s", err, out)
		}
	}
	if err := project.CreateConfig(root, "."); err != nil {
		t.Fatalf("Konfiguration anlegen: %v", err)
	}
	chdir(t, root)

	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte("#!/bin/sh\necho \"gh gestartet: $*\" >&2\nexit 99\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+filepath.Dir(gitPath))
	t.Setenv("GH_CONFIG_DIR", t.TempDir())
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")

	if config != "" {
		path := filepath.Join(t.TempDir(), githubaccess.FileName)
		if err := os.WriteFile(path, []byte(config), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv(githubaccess.EnvConfig, path)
	}
	return root
}

// serviceTokenFile legt eine Token-Datei mit routeFakeToken an.
func serviceTokenFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "k-playbook.token")
	if err := os.WriteFile(path, []byte(routeFakeToken+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// envRunner antwortet wie slowRunner ohne Pause und hält je Aufruf
// Argumente und Umgebung fest. failRuns lässt `gh run list` mit 401
// scheitern, damit auch eine Fehlermeldung durch den Weg geht.
type envRunner struct {
	mu       sync.Mutex
	calls    [][]string
	envs     [][]string
	failRuns bool
}

func (e *envRunner) Run(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	return e.RunEnv(ctx, dir, nil, name, args...)
}

func (e *envRunner) RunEnv(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
	e.mu.Lock()
	e.calls = append(e.calls, append([]string{name}, args...))
	e.envs = append(e.envs, env)
	e.mu.Unlock()
	if e.failRuns && name == "gh" && len(args) > 1 && args[0] == "run" {
		return nil, &github.CommandError{Name: name, Args: args, ExitCode: 1, Stderr: "HTTP 401: Bad credentials (https://api.github.com/graphql)", Err: os.ErrPermission}
	}
	if name == "gh" && len(args) > 0 && args[0] == "pr" {
		return []byte(`[]`), nil
	}
	if name == "gh" && len(args) > 1 && args[0] == "api" && args[1] == "graphql" {
		return []byte(`{"data":{"repository":{"open":{"nodes":[]},"closed":{"nodes":[]},"merged":{"nodes":[]}}}}`), nil
	}
	return slowRunner{}.Run(ctx, dir, name, args...)
}

// recordRunner setzt runner als Runner jedes Clients ein und übernimmt die
// Umgebung, die der Handler mitgibt.
func recordRunner(t *testing.T, runner github.Runner) {
	t.Helper()
	before := newGitHubClient
	newGitHubClient = func(dir string, env []string) *github.Client {
		return &github.Client{Runner: runner, Dir: dir, Timeout: github.DefaultTimeout, Env: env}
	}
	t.Cleanup(func() { newGitHubClient = before })
}

// forbidClient lässt den Test scheitern, sobald ein gh-Client entsteht.
func forbidClient(t *testing.T) {
	t.Helper()
	before := newGitHubClient
	newGitHubClient = func(dir string, env []string) *github.Client {
		t.Errorf("gh-Client angelegt, obwohl kein gh starten darf")
		return &github.Client{Runner: blockingRunner{}, Dir: dir}
	}
	t.Cleanup(func() { newGitHubClient = before })
}

// captureLogs leitet log und os.Stderr in eine Datei um; die Funktion liefert
// danach, was dort ankam.
func captureLogs(t *testing.T) func() string {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "log")
	if err != nil {
		t.Fatal(err)
	}
	stderr, writer := os.Stderr, log.Writer()
	os.Stderr = file
	log.SetOutput(file)
	restore := func() {
		os.Stderr = stderr
		log.SetOutput(writer)
	}
	t.Cleanup(restore)
	return func() string {
		restore()
		data, _ := os.ReadFile(file.Name())
		return string(data)
	}
}

// getGitHubRaw ruft einen Endpunkt der GitHub-Ansicht und liefert den Rumpf.
func getGitHubRaw(t *testing.T, path string) []byte {
	t.Helper()
	recorder := httptest.NewRecorder()
	routes(&serverState{}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("%s: Status %d", path, recorder.Code)
	}
	return recorder.Body.Bytes()
}

// githubRouteAnswer sind die Felder, die der Weg in einer Antwort setzt.
type githubRouteAnswer struct {
	github.Result
	Via *github.Via `json:"via"`
}

func decodeRouteAnswer(t *testing.T, path string, body []byte) githubRouteAnswer {
	t.Helper()
	var answer githubRouteAnswer
	if err := json.Unmarshal(body, &answer); err != nil {
		t.Fatalf("%s: Antwort nicht lesbar: %v", path, err)
	}
	return answer
}

// Mit Dienst liest die Seite über dessen Zugang: gh bekommt die
// Kindumgebung mit dem Token des Dienstes, und die Kopfzeile nennt Dienst
// und Zugang in der Form der Zeile von run. Der Token steht weder im JSON
// eines Endpunkts noch in einer Meldung noch im Log noch in einem Argument.
func TestGitHubSeiteUeberDienst(t *testing.T) {
	tokenFile := serviceTokenFile(t)
	serviceProject(t, "https://github.com/kascada/k-playbook.git",
		"services:\n  k-playbook:\n    repo: kascada/k-playbook\n    access:\n      - type: token\n        file: "+tokenFile+"\n")
	runner := &envRunner{failRuns: true}
	recordRunner(t, runner)
	logs := captureLogs(t)

	bodies := map[string][]byte{}
	for _, path := range githubEndpoints {
		bodies[path] = getGitHubRaw(t, path)
	}
	logged := logs()

	for path, body := range bodies {
		if strings.Contains(string(body), routeFakeToken) {
			t.Errorf("%s: Token im JSON: %s", path, body)
		}
	}
	if strings.Contains(logged, routeFakeToken) {
		t.Errorf("Token im Log: %s", logged)
	}

	overview := decodeRouteAnswer(t, "/api/github/overview", bodies["/api/github/overview"])
	if overview.State != github.StateOK {
		t.Fatalf("Kopfzeile: %+v", overview.Result)
	}
	want := "github run k-playbook: git über token " + tokenFile + ", api über token"
	if overview.Via == nil || overview.Via.Kind != github.ViaService || overview.Via.Service != "k-playbook" || overview.Via.Line != want {
		t.Errorf("via: %+v, erwartet die Zeile %q", overview.Via, want)
	}
	// Ein Fehler über den Dienst nennt nicht die Zugangsdatei als Abhilfe —
	// es gibt sie ja — und fällt nicht auf das aktive Konto zurück.
	runs := decodeRouteAnswer(t, "/api/github/runs", bodies["/api/github/runs"])
	if runs.State != github.StateBadCredentials || runs.AccessLink != "" || strings.Contains(runs.Message, routeFakeToken) {
		t.Errorf("Läufe mit 401: %+v", runs.Result)
	}

	ghCalls := 0
	for index, call := range runner.calls {
		for _, arg := range call {
			if strings.Contains(arg, routeFakeToken) {
				t.Errorf("Token im Argument: %q", call)
			}
		}
		if call[0] != "gh" {
			continue
		}
		ghCalls++
		found := false
		for _, entry := range runner.envs[index] {
			if entry == "GH_TOKEN="+routeFakeToken {
				found = true
			}
		}
		if !found {
			t.Errorf("gh ohne das Token des Dienstes: %q", call)
		}
	}
	if ghCalls == 0 {
		t.Error("kein gh-Aufruf")
	}
}

// Ohne api-Kanal, ohne verfügbaren Zugang und bei fehlerhafter Zugangsdatei
// startet kein gh: Die Seite zeigt die Meldung des Weges samt Verweis auf
// die Zugangsdatei, und es gibt keinen Rückfall auf das aktive Konto.
func TestGitHubSeiteOhneGHProzess(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "fehlt.token")
	cases := []struct {
		name    string
		config  string
		want    github.State
		message string
		via     bool
	}{
		{"kein api-Kanal", "services:\n  k-playbook:\n    repo: kascada/k-playbook\n    access:\n      - type: public\n",
			github.StateServiceNoAPI, "Der Dienst k-playbook hat keinen api-Zugang", true},
		{"kein verfügbarer Zugang", "services:\n  k-playbook:\n    repo: kascada/k-playbook\n    access:\n      - type: token\n        file: " + missing + "\n",
			github.StateServiceNoAccess, "Die Token-Datei fehlt", true},
		{"Zugangsdatei fehlerhaft", "services:\n  kaputt:\n    access:\n      - type: gh\n        login: alice\n",
			github.StateAccessFileError, "ist fehlerhaft", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			serviceProject(t, "https://github.com/kascada/k-playbook.git", c.config)
			forbidClient(t)
			for _, path := range githubEndpoints {
				answer := decodeRouteAnswer(t, path, getGitHubRaw(t, path))
				if answer.State != c.want || !strings.Contains(answer.Message, c.message) || answer.AccessLink != github.AccessPage {
					t.Errorf("%s: %+v", path, answer.Result)
				}
				if path == "/api/github/overview" && (answer.Via != nil) != c.via {
					t.Errorf("%s: via %+v", path, answer.Via)
				}
			}
		})
	}
}

// Ohne Dienst bleibt es beim Bisherigen. Scheitert es an gh oder seiner
// Anmeldung, nennt die Seite die Zugangsdatei als Abhilfe, mit „Anlegen“
// und der Hilfe durch eine KI-Sitzung.
func TestGitHubSeiteOhneDienstNenntDieZugangsdatei(t *testing.T) {
	t.Run("gh fehlt", func(t *testing.T) {
		serviceProject(t, "https://github.com/kascada/k-playbook.git", "")
		// Nur git im PATH, kein gh — auch nicht das des Rechners.
		gitPath, _ := exec.LookPath("git")
		onlyGit := t.TempDir()
		if err := os.Symlink(gitPath, filepath.Join(onlyGit, "git")); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", onlyGit)
		forbidClient(t)
		answer := decodeRouteAnswer(t, "/api/github/overview", getGitHubRaw(t, "/api/github/overview"))
		if answer.State != github.StateNotInstalled || answer.AccessLink != github.AccessPage {
			t.Fatalf("Kopfzeile: %+v", answer.Result)
		}
		for _, want := range []string{"Zugangsdatei", "„Anlegen“", "KI-Sitzung"} {
			if !strings.Contains(answer.Message, want) {
				t.Errorf("Meldung ohne %s: %q", want, answer.Message)
			}
		}
		if answer.Via == nil || answer.Via.Kind != github.ViaAccount {
			t.Errorf("via: %+v", answer.Via)
		}
	})
	t.Run("401", func(t *testing.T) {
		runner := &envRunner{failRuns: true}
		enableGitHub(t, runner)
		answer := decodeRouteAnswer(t, "/api/github/runs", getGitHubRaw(t, "/api/github/runs"))
		if answer.State != github.StateBadCredentials || answer.AccessLink != github.AccessPage || !strings.Contains(answer.Message, "KI-Sitzung") {
			t.Errorf("Läufe: %+v", answer.Result)
		}
		for _, env := range runner.envs {
			if env != nil {
				t.Errorf("ohne Dienst mit eigener Umgebung: %q", env)
			}
		}
		overview := decodeRouteAnswer(t, "/api/github/overview", getGitHubRaw(t, "/api/github/overview"))
		if overview.Via == nil || overview.Via.Kind != github.ViaAccount {
			t.Errorf("via ohne Dienst: %+v", overview.Via)
		}
	})
}
