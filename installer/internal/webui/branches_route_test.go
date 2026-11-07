package webui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/branches"
	"github.com/kascada/k-playbook/installer/internal/github"
	"github.com/kascada/k-playbook/installer/internal/githubaccess"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// branchesServiceProject ist newBranchesProject mit einem GitHub-Remote für
// origin, tools.gh.status: enabled, einem gh im PATH, das nie laufen darf,
// und der Zugangsdatei config (leer: keine).
func branchesServiceProject(t *testing.T, config string) string {
	t.Helper()
	root, _ := newBranchesProject(t, "")
	branchesGit(t, root, "remote", "set-url", "origin", "https://github.com/kascada/k-playbook.git")
	if err := project.SetGHStatus(root, project.GHEnabled); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte("#!/bin/sh\necho \"gh gestartet: $*\" >&2\nexit 99\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	gitPath, _ := exec.LookPath("git")
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

// fetchRecorder ersetzt den Runner von Fetch: Er hält Argumente und Umgebung
// fest und lässt den Fetch mit der Ausgabe stderr scheitern, wenn fail
// gesetzt ist.
type fetchRecorder struct {
	mu    sync.Mutex
	calls [][]string
	envs  [][]string
	fail  bool
}

func (f *fetchRecorder) Run(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	return f.RunEnv(ctx, dir, nil, name, args...)
}

func (f *fetchRecorder) RunEnv(_ context.Context, _ string, env []string, name string, args ...string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, append([]string{name}, args...))
	f.envs = append(f.envs, env)
	if f.fail {
		output := "fatal: Authentication failed for 'https://github.com/kascada/k-playbook.git/'"
		return []byte(output), &github.CommandError{Name: name, Args: args, ExitCode: 128, Stderr: output, Err: os.ErrPermission}
	}
	return []byte("From https://github.com/kascada/k-playbook\n"), nil
}

func useFetchRecorder(t *testing.T, recorder *fetchRecorder) {
	t.Helper()
	before := branchesActionRunner
	branchesActionRunner = recorder
	t.Cleanup(func() { branchesActionRunner = before })
}

// postFetchRaw ruft POST /api/branches/fetch und liefert Status und Rumpf.
func postFetchRaw(t *testing.T) (int, []byte) {
	t.Helper()
	recorder := httptest.NewRecorder()
	routes(&serverState{}).ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/branches/fetch", nil))
	return recorder.Code, recorder.Body.Bytes()
}

// getBranchesRaw ruft GET /api/branches und liefert den Rumpf.
func getBranchesRaw(t *testing.T) []byte {
	t.Helper()
	recorder := httptest.NewRecorder()
	routes(&serverState{}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/branches", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("Status %d: %s", recorder.Code, recorder.Body.String())
	}
	return recorder.Body.Bytes()
}

func envHas(env []string, entry string) bool {
	for _, value := range env {
		if value == entry {
			return true
		}
	}
	return false
}

// Mit Dienst laufen die gh-Abfragen und der Fetch über dessen Zugang: gh
// und git bekommen die Kindumgebung mit dem Token, der git-Kanal lenkt das
// Dienst-Repo um, und die Seite nennt Dienst und Zugang. Der Token steht
// weder im JSON von /branches und vom Fetch noch in Meldungen (auch nicht in
// CommandError.Args/Stderr eines gescheiterten Fetch) noch im Log noch in
// einem Argument.
func TestBranchesUeberDienst(t *testing.T) {
	tokenFile := serviceTokenFile(t)
	branchesServiceProject(t, "services:\n  k-playbook:\n    repo: kascada/k-playbook\n    access:\n      - type: token\n        file: "+tokenFile+"\n")
	ghRunner := &envRunner{}
	recordRunner(t, ghRunner)
	fetch := &fetchRecorder{}
	useFetchRecorder(t, fetch)
	logs := captureLogs(t)

	listingBody := getBranchesRaw(t)
	code, fetchBody := postFetchRaw(t)
	fetch.fail = true
	failCode, failBody := postFetchRaw(t)
	logged := logs()

	for name, body := range map[string][]byte{"/api/branches": listingBody, "Fetch": fetchBody, "Fetch gescheitert": failBody} {
		if strings.Contains(string(body), routeFakeToken) {
			t.Errorf("%s: Token im JSON: %s", name, body)
		}
	}
	if strings.Contains(logged, routeFakeToken) {
		t.Errorf("Token im Log: %s", logged)
	}

	var listing branches.Listing
	if err := json.Unmarshal(listingBody, &listing); err != nil {
		t.Fatal(err)
	}
	want := "github run k-playbook: git über token " + tokenFile + ", api über token"
	if listing.State != branches.StateOK || listing.Via == nil || listing.Via.Kind != github.ViaService || listing.Via.Line != want || listing.FetchBlock != nil {
		t.Errorf("Liste: State=%q Via=%+v FetchBlock=%+v", listing.State, listing.Via, listing.FetchBlock)
	}
	if !listing.GitHub.Enabled {
		t.Errorf("GitHub-Daten nicht über den Dienst gelesen: %+v", listing.GitHub)
	}
	ghCalls := 0
	for index, call := range ghRunner.calls {
		if call[0] == "gh" {
			ghCalls++
			if !envHas(ghRunner.envs[index], "GH_TOKEN="+routeFakeToken) {
				t.Errorf("gh ohne das Token des Dienstes: %q", call)
			}
		}
	}
	if ghCalls == 0 {
		t.Error("kein gh-Aufruf")
	}

	var result branches.FetchResult
	if err := json.Unmarshal(fetchBody, &result); err != nil || code != http.StatusOK || !result.OK || result.Via != want {
		t.Errorf("Fetch: %d %+v %v", code, result, err)
	}
	if failCode != http.StatusOK {
		t.Errorf("gescheiterter Fetch: Status %d", failCode)
	}
	if len(fetch.calls) != 2 {
		t.Fatalf("Fetch-Aufrufe: %q", fetch.calls)
	}
	for index, call := range fetch.calls {
		for _, arg := range call {
			if strings.Contains(arg, routeFakeToken) {
				t.Errorf("Token im Argument: %q", call)
			}
		}
		env := fetch.envs[index]
		if !envHas(env, "GH_TOKEN="+routeFakeToken) || !envHas(env, "GIT_TERMINAL_PROMPT=0") {
			t.Errorf("Fetch ohne die Umgebung des Dienstes")
		}
		redirected := false
		for _, entry := range env {
			if strings.HasPrefix(entry, "GIT_CONFIG_VALUE_") && strings.Contains(entry, "github.com/kascada/k-playbook") {
				redirected = true
			}
		}
		if !redirected {
			t.Error("der git-Kanal lenkt das Dienst-Repo nicht um")
		}
	}
}

// Ohne api-Kanal startet kein gh, der Fetch läuft über den git-Kanal und
// ohne Token in der Umgebung.
func TestBranchesOhneAPIKanal(t *testing.T) {
	branchesServiceProject(t, "services:\n  k-playbook:\n    repo: kascada/k-playbook\n    access:\n      - type: public\n")
	forbidClient(t)
	fetch := &fetchRecorder{}
	useFetchRecorder(t, fetch)

	listing := getBranches(t)
	if listing.State != branches.StateOK || listing.GitHub.Enabled || listing.GitHub.State != github.StateServiceNoAPI || listing.FetchBlock != nil {
		t.Errorf("Liste: State=%q GitHub=%+v FetchBlock=%+v", listing.State, listing.GitHub, listing.FetchBlock)
	}
	if !strings.Contains(listing.GitHub.Message, "Der Dienst k-playbook hat keinen api-Zugang") {
		t.Errorf("Meldung: %q", listing.GitHub.Message)
	}
	code, body := postFetchRaw(t)
	var result branches.FetchResult
	if err := json.Unmarshal(body, &result); err != nil || code != http.StatusOK || !result.OK {
		t.Fatalf("Fetch: %d %s", code, body)
	}
	if len(fetch.envs) != 1 || fetch.envs[0] == nil {
		t.Fatalf("Fetch ohne die Umgebung des Dienstes: %q", fetch.envs)
	}
	for _, entry := range fetch.envs[0] {
		if strings.HasPrefix(entry, "GH_TOKEN=") || strings.HasPrefix(entry, "GITHUB_TOKEN=") {
			t.Errorf("Token-Variable ohne api-Kanal: %s", entry)
		}
	}
}

// Ohne verfügbaren Zugang und bei fehlerhafter Zugangsdatei weder gh noch
// Fetch; die lokalen Branches stehen trotzdem da.
func TestBranchesOhneGHUndFetch(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "fehlt.token")
	cases := []struct {
		name   string
		config string
		want   github.State
	}{
		{"kein verfügbarer Zugang", "services:\n  k-playbook:\n    repo: kascada/k-playbook\n    access:\n      - type: token\n        file: " + missing + "\n", github.StateServiceNoAccess},
		{"Zugangsdatei fehlerhaft", "services:\n  kaputt:\n    access:\n      - type: gh\n        login: alice\n", github.StateAccessFileError},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			branchesServiceProject(t, c.config)
			forbidClient(t)
			fetch := &fetchRecorder{}
			useFetchRecorder(t, fetch)

			listing := getBranches(t)
			if listing.State != branches.StateOK || len(listing.Groups) == 0 {
				t.Fatalf("lokale Liste fehlt: State=%q Groups=%d", listing.State, len(listing.Groups))
			}
			if listing.GitHub.Enabled || listing.GitHub.State != c.want || listing.GitHub.AccessLink != github.AccessPage {
				t.Errorf("GitHub: %+v", listing.GitHub)
			}
			if listing.FetchBlock == nil || listing.FetchBlock.State != c.want || listing.FetchBlock.AccessLink != github.AccessPage {
				t.Errorf("FetchBlock: %+v", listing.FetchBlock)
			}

			code, body := postFetchRaw(t)
			var result branches.FetchResult
			if err := json.Unmarshal(body, &result); err != nil || code != http.StatusConflict || result.OK || !strings.Contains(result.Message, "git fetch läuft nicht") {
				t.Errorf("Fetch: %d %s", code, body)
			}
			if len(fetch.calls) != 0 {
				t.Errorf("Fetch gestartet: %q", fetch.calls)
			}
		})
	}
}

// Ohne Dienst läuft der Fetch wie bisher, ohne eigene Umgebung. Scheitert
// er, nennt die Antwort die Zugangsdatei als Abhilfe.
func TestBranchesOhneDienstWieBisher(t *testing.T) {
	branchesServiceProject(t, "")
	fetch := &fetchRecorder{fail: true}
	useFetchRecorder(t, fetch)

	listing := getBranches(t)
	if listing.Via == nil || listing.Via.Kind != github.ViaAccount || listing.FetchBlock != nil {
		t.Errorf("Liste: Via=%+v FetchBlock=%+v", listing.Via, listing.FetchBlock)
	}
	_, body := postFetchRaw(t)
	var result branches.FetchResult
	if err := json.Unmarshal(body, &result); err != nil || result.OK || result.AccessLink != github.AccessPage || !strings.Contains(result.Message, "KI-Sitzung") || result.Via != "" {
		t.Errorf("Fetch: %s", body)
	}
	if len(fetch.envs) != 1 || fetch.envs[0] != nil {
		t.Errorf("Fetch ohne Dienst mit eigener Umgebung: %q", fetch.envs)
	}
}
