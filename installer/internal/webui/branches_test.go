package webui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kascada/k-playbook/installer/internal/branches"
	"github.com/kascada/k-playbook/installer/internal/github"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// Die Handler-Tests legen ihr Repo selbst an. Umgeschaltet und geholt wird nur
// dort, nie in einem echten Arbeitsrepo.

func branchesGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// newBranchesProject legt ein Projekt mit repo_root: . an, dessen Repo einen
// Remote mit main und feature/x hat, und macht es zum Arbeitsverzeichnis.
// Zurück kommen Projekt- und Remote-Verzeichnis.
func newBranchesProject(t *testing.T, gitSection string) (string, string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git fehlt")
	}
	config := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(config, []byte("[init]\n\tdefaultBranch = main\n[commit]\n\tgpgsign = false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", config)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.invalid")
	t.Setenv("GIT_COMMITTER_NAME", "Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.invalid")

	base := t.TempDir()
	origin := filepath.Join(base, "origin.git")
	seed := filepath.Join(base, "seed")
	root := filepath.Join(base, "projekt")
	branchesGit(t, base, "init", "-q", "--bare", "-b", "main", origin)
	branchesGit(t, base, "init", "-q", "-b", "main", seed)
	if err := os.WriteFile(filepath.Join(seed, "README.md"), []byte("eins\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	branchesGit(t, seed, "add", "README.md")
	branchesGit(t, seed, "commit", "-q", "-m", "eins")
	branchesGit(t, seed, "branch", "feature/x")
	branchesGit(t, seed, "remote", "add", "origin", origin)
	branchesGit(t, seed, "push", "-q", "origin", "--all")
	branchesGit(t, base, "clone", "-q", origin, root)

	if err := project.CreateConfig(root, "."); err != nil {
		t.Fatalf("Konfiguration anlegen: %v", err)
	}
	if gitSection != "" {
		path := project.ConfigPath(root)
		content, _ := os.ReadFile(path)
		if err := os.WriteFile(path, append(content, []byte("\n"+gitSection)...), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Die Konfiguration gehört nicht zum Stand, der hier geprüft wird: sie
	// liegt im Repo und machte den Arbeitsbaum sonst schmutzig.
	if err := os.WriteFile(filepath.Join(root, ".git", "info", "exclude"), []byte(project.ConfigFileName+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	chdir(t, root)
	return root, seed
}

func getBranches(t *testing.T) branches.Listing {
	t.Helper()
	recorder := httptest.NewRecorder()
	routes(&serverState{}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/branches", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("Status %d: %s", recorder.Code, recorder.Body.String())
	}
	var listing branches.Listing
	if err := json.Unmarshal(recorder.Body.Bytes(), &listing); err != nil {
		t.Fatalf("Antwort nicht lesbar: %v", err)
	}
	return listing
}

func TestBranchesAPIOhneProjekt(t *testing.T) {
	chdir(t, t.TempDir())
	listing := getBranches(t)
	if listing.State != branches.StateNoProject || listing.Message == "" || listing.Groups == nil {
		t.Errorf("listing = %+v", listing)
	}
}

// Die Liste steht ohne gh da; der Default-Branch kommt aus origin/HEAD.
func TestBranchesAPIListeOhneGH(t *testing.T) {
	newBranchesProject(t, "")
	listing := getBranches(t)
	if listing.State != branches.StateOK {
		t.Fatalf("State = %q: %s", listing.State, listing.Message)
	}
	if listing.Default.Name != "main" || listing.Default.Source != "remote-head" {
		t.Errorf("Default = %+v", listing.Default)
	}
	if listing.Switch.Offered || listing.Switch.Status != project.GitSwitchUnknown {
		t.Errorf("Switch = %+v, erwartet unknown ohne Angebot", listing.Switch)
	}
	if listing.GitHub.Enabled {
		t.Errorf("GitHub = %+v", listing.GitHub)
	}
}

// „Mit gh" heißt tools.gh.status = enabled. Bei unknown und disabled kommen keine
// GitHub-Daten hinzu, auch wenn gh installiert und angemeldet ist — und es startet
// kein gh-Prozess.
func TestBranchesAPIOhneEntscheidungKeinGH(t *testing.T) {
	for _, status := range []project.GHStatus{project.GHUnknown, project.GHDisabled} {
		t.Run(string(status), func(t *testing.T) {
			root, _ := newBranchesProject(t, "")
			if err := project.SetGHStatus(root, status); err != nil {
				t.Fatal(err)
			}
			bin := t.TempDir()
			if err := os.WriteFile(filepath.Join(bin, "gh"), []byte("#!/bin/sh\nexit 99\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			gitPath, _ := exec.LookPath("git")
			t.Setenv("PATH", bin+string(os.PathListSeparator)+filepath.Dir(gitPath))
			t.Setenv("GH_CONFIG_DIR", t.TempDir())
			t.Setenv("GH_TOKEN", "nur-fuer-die-vorpruefung")

			before := newGitHubClient
			newGitHubClient = func(dir string) *github.Client {
				t.Errorf("gh-Client angelegt, obwohl tools.gh.status = %s", status)
				return before(dir)
			}
			t.Cleanup(func() { newGitHubClient = before })

			listing := getBranches(t)
			if listing.State != branches.StateOK || listing.GitHub.Enabled || listing.Default.Source != "remote-head" {
				t.Errorf("listing: State=%q GitHub=%+v Default=%+v", listing.State, listing.GitHub, listing.Default)
			}
		})
	}
}

// ghFake beantwortet die gh-Aufrufe der Seite und reicht git an das echte git.
type ghFake struct {
	calls []string
}

func (f *ghFake) Run(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	call := name + " " + strings.Join(args, " ")
	f.calls = append(f.calls, call)
	if name != "gh" {
		return github.ExecRunner{}.Run(ctx, dir, name, args...)
	}
	switch {
	case strings.HasPrefix(call, "gh repo view"):
		return []byte(`{"nameWithOwner":"acme/app","defaultBranchRef":{"name":"main"}}`), nil
	case strings.HasPrefix(call, "gh api graphql"):
		return []byte(`{"data":{"repository":{"defaultBranchRef":{"name":"main"},"open":{"nodes":[{"number":5,"title":"X","state":"OPEN","headRefName":"feature/x","baseRefName":"main"}]},"closed":{"nodes":[]}}}}`), nil
	case strings.HasPrefix(call, "gh api repos/acme/app/environments"):
		return []byte(`{"environments":[{"name":"prod"}]}`), nil
	case strings.HasPrefix(call, "gh api repos/acme/app/deployments"):
		return []byte(`[]`), nil
	}
	return nil, fmt.Errorf("unerwarteter Aufruf: %s", call)
}

// Mit gh kommen Default-Branch, PRs und Environments dazu.
func TestBranchesAPIMitGH(t *testing.T) {
	fake := &ghFake{}
	newBranchesProject(t, "")
	enableGitHubInCurrentProject(t, fake)

	listing := getBranches(t)
	if !listing.GitHub.Enabled || listing.GitHub.State != github.StateOK || listing.Default.Source != "github" {
		t.Fatalf("GitHub = %+v, Default = %+v", listing.GitHub, listing.Default)
	}
	var pull *branches.Pull
	for _, group := range listing.Groups {
		for _, branch := range group.Branches {
			if branch.Name == "feature/x" {
				pull = branch.Pull
			}
		}
	}
	if pull == nil || pull.Number != 5 {
		t.Errorf("PR an feature/x = %+v", pull)
	}
	found := false
	for _, env := range listing.Environments {
		if env.Name == "prod" && env.OnGitHub && env.Branch == "main" && env.Suggested {
			found = true
		}
	}
	if !found {
		t.Errorf("Environments = %+v", listing.Environments)
	}
}

// ghBlockingFake lässt jeden gh-Aufruf bis zum Ablauf seines Kontexts hängen und
// reicht git an das echte git.
type ghBlockingFake struct{}

func (ghBlockingFake) Run(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	if name != "gh" {
		return github.ExecRunner{}.Run(ctx, dir, name, args...)
	}
	return blockingRunner{}.Run(ctx, dir, name, args...)
}

// Ein langsames GitHub nimmt der lokalen Liste nicht die Zeit: gh hat ein
// eigenes, kürzeres Budget, die git-Liste ein eigenes. Läuft gh in die
// Zeitgrenze, steht die Liste trotzdem da, und die Antwort sagt, dass die
// GitHub-Daten fehlen.
func TestBranchesAPIGitHubZeitgrenzeLaesstListeStehen(t *testing.T) {
	newBranchesProject(t, "")
	enableGitHubInCurrentProject(t, ghBlockingFake{})
	setBranchesBudgets(t, 200*time.Millisecond, 20*time.Second)

	started := time.Now()
	listing := getBranches(t)
	elapsed := time.Since(started)

	if listing.State != branches.StateOK {
		t.Fatalf("State = %q: %s", listing.State, listing.Message)
	}
	if len(listing.Groups) == 0 || listing.Head.Branch != "main" {
		t.Errorf("lokale Liste fehlt: Head = %+v, Groups = %+v", listing.Head, listing.Groups)
	}
	if !listing.GitHub.Enabled || listing.GitHub.State != github.StateTimeout || listing.GitHub.Message == "" {
		t.Errorf("GitHub = %+v, erwartet Zeitgrenze mit Satz", listing.GitHub)
	}
	if listing.Default.Name != "main" || listing.Default.Source != "remote-head" || !strings.Contains(listing.Default.Reason, "GitHub") {
		t.Errorf("Default = %+v", listing.Default)
	}
	if elapsed > 5*time.Second {
		t.Errorf("Antwort nach %s", elapsed)
	}
}

// enableGitHubInCurrentProject setzt tools.gh.status = enabled im Projekt des
// Arbeitsverzeichnisses und leitet jeden gh-Aufruf an runner.
func enableGitHubInCurrentProject(t *testing.T, runner github.Runner) {
	t.Helper()
	environment := project.Detect()
	if err := project.SetGHStatus(environment.ProjectDir, project.GHEnabled); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte("#!/bin/sh\nexit 99\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	gitPath, _ := exec.LookPath("git")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+filepath.Dir(gitPath))
	t.Setenv("GH_CONFIG_DIR", t.TempDir())
	t.Setenv("GH_TOKEN", "nur-fuer-die-vorpruefung")
	before := newGitHubClient
	newGitHubClient = func(dir string) *github.Client {
		return &github.Client{Runner: runner, Dir: dir, Timeout: github.DefaultTimeout}
	}
	t.Cleanup(func() { newGitHubClient = before })
}

// Fetch ist ein eigener POST und holt, was auf dem Remote neu ist.
func TestBranchesFetch(t *testing.T) {
	_, seed := newBranchesProject(t, "")
	branchesGit(t, seed, "branch", "neu")
	branchesGit(t, seed, "push", "-q", "origin", "neu")

	before := getBranches(t)
	for _, group := range before.Groups {
		for _, branch := range group.Branches {
			if branch.Name == "neu" {
				t.Fatal("neu steht schon vor dem Fetch in der Liste — die Liste hat ungefragt geholt")
			}
		}
	}

	recorder := httptest.NewRecorder()
	routes(&serverState{}).ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/branches/fetch", nil))
	var result branches.FetchResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil || !result.OK {
		t.Fatalf("Fetch: %v %s", err, recorder.Body.String())
	}
	if result.Fetch.State != branches.FieldKnown || result.Command != "git fetch --all --prune" {
		t.Errorf("Fetch = %+v", result)
	}

	after := getBranches(t)
	found := false
	for _, group := range after.Groups {
		for _, branch := range group.Branches {
			if branch.Name == "neu" && branch.RemoteOnly {
				found = true
			}
		}
	}
	if !found {
		t.Error("neu fehlt nach dem Fetch")
	}
}

// Die POSTs stehen hinter der Herkunftsprüfung wie alle anderen.
func TestBranchesPostsPruefenHerkunft(t *testing.T) {
	newBranchesProject(t, "")
	for _, path := range []string{"/api/branches/fetch", "/api/branches/switch"} {
		request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
		request.Host = "127.0.0.1:4000"
		request.Header.Set("Origin", "http://fremd.example")
		recorder := httptest.NewRecorder()
		routes(&serverState{}).ServeHTTP(recorder, request)
		if recorder.Code != http.StatusForbidden {
			t.Errorf("%s: Status %d, erwartet 403", path, recorder.Code)
		}
	}
}

// noProcesses ist eine leere Prozessquelle: Die Tests laufen selbst in einem
// Projekt, in dem Sitzungen offen sein können, und prüfen hier nicht die
// Sitzungserkennung.
type noProcesses struct{}

func (noProcesses) Processes() ([]branches.Process, error) { return nil, nil }

func useNoProcesses(t *testing.T) {
	t.Helper()
	before := branchesProcesses
	branchesProcesses = noProcesses{}
	t.Cleanup(func() { branchesProcesses = before })
}

func getSwitchCheck(t *testing.T, query string) (int, branches.SwitchCheck) {
	t.Helper()
	recorder := httptest.NewRecorder()
	routes(&serverState{}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/branches/switch-check?"+query, nil))
	var result branches.SwitchCheck
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("Antwort nicht lesbar: %v: %s", err, recorder.Body.String())
	}
	return recorder.Code, result
}

func TestBranchesSwitchCheckAPI(t *testing.T) {
	newBranchesProject(t, "git:\n  switch: offer\n")
	useNoProcesses(t)

	status, result := getSwitchCheck(t, "target=feature/x")
	if status != http.StatusOK || result.State != branches.StateOK || !result.Offered || result.Stamp == "" {
		t.Fatalf("Status %d, result = %+v", status, result)
	}
	if result.Command != "git switch --no-overwrite-ignore --track origin/feature/x" {
		t.Errorf("Command = %q", result.Command)
	}

	status, result = getSwitchCheck(t, "target=--force")
	if status != http.StatusBadRequest || result.State != branches.StateInvalidTarget {
		t.Errorf("Status %d, State %q", status, result.State)
	}
}

// Ohne git: gilt unknown, und die Freigabe blockiert mit Verweis auf git.switch.
func TestBranchesSwitchCheckOhneFreigabe(t *testing.T) {
	newBranchesProject(t, "")
	useNoProcesses(t)
	_, result := getSwitchCheck(t, "target=feature/x")
	if result.Offered {
		t.Fatal("ohne git.switch angeboten")
	}
	for _, check := range result.Checks {
		if check.ID == "freigabe" && (check.Result != branches.ResultBlocked || !strings.Contains(check.Reason, "git.switch")) {
			t.Errorf("freigabe = %+v", check)
		}
	}
}

func postSwitch(t *testing.T, body string) (int, branches.SwitchResult) {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/branches/switch", strings.NewReader(body))
	routes(&serverState{}).ServeHTTP(recorder, request)
	var result branches.SwitchResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("Antwort nicht lesbar: %v: %s", err, recorder.Body.String())
	}
	return recorder.Code, result
}

// Ende zu Ende über die Endpunkte: prüfen, umschalten; ein schmutziger
// Arbeitsbaum blockiert, und ein POST mit altem Stempel führt nichts aus.
func TestBranchesUmschaltenUeberAPI(t *testing.T) {
	root, _ := newBranchesProject(t, "git:\n  switch: offer\n  allow:\n    - feature/*\n    - main\n")
	useNoProcesses(t)

	_, check := getSwitchCheck(t, "target=feature/x")
	if !check.Offered {
		t.Fatalf("nicht angeboten: %+v", check.Checks)
	}
	status, result := postSwitch(t, `{"target":"feature/x","stamp":"`+check.Stamp+`"}`)
	if status != http.StatusOK || !result.OK || result.After.Branch != "feature/x" {
		t.Fatalf("Status %d, result = %+v", status, result)
	}

	// Zurück geht es nur mit sauberem Arbeitsbaum.
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("schmutzig\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, check = getSwitchCheck(t, "target=main")
	if check.Offered {
		t.Fatal("schmutziger Arbeitsbaum angeboten")
	}
	status, result = postSwitch(t, `{"target":"main","stamp":"`+check.Stamp+`"}`)
	if status != http.StatusConflict || result.Executed || result.Reason != branches.SwitchReasonBlocked {
		t.Errorf("Status %d, result = %+v", status, result)
	}
	if got := branchesGit(t, root, "branch", "--show-current"); got != "feature/x" {
		t.Errorf("ausgecheckt = %q", got)
	}

	status, result = postSwitch(t, `{"target":"main","stamp":"alt"}`)
	if status != http.StatusConflict || result.Executed || result.Reason != branches.SwitchReasonStamp {
		t.Errorf("alter Stempel: Status %d, result = %+v", status, result)
	}
	if status, _ := postSwitch(t, `{"target":"-f","stamp":"x"}`); status != http.StatusBadRequest {
		t.Errorf("ungültiges Ziel: Status %d", status)
	}
}

// setBranchesBudgets setzt die Budgets von GET /api/branches für einen Test.
func setBranchesBudgets(t *testing.T, gh time.Duration, list time.Duration) {
	t.Helper()
	beforeGitHub, beforeList := branchesGitHubBudget, branchesBudget
	branchesGitHubBudget, branchesBudget = gh, list
	t.Cleanup(func() { branchesGitHubBudget, branchesBudget = beforeGitHub, beforeList })
}
