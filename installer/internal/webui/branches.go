package webui

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/kascada/k-playbook/installer/internal/branches"
	"github.com/kascada/k-playbook/installer/internal/github"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// Die Seite /branches zeigt die Branches des Code-Repos, ihre Umgebungen und die
// Worktrees. Gearbeitet wird in project.RepoRootDir, nicht im Hauptverzeichnis:
// bei einem Projekt mit repo_root: omni-gw ist das omni-gw, und die
// Konfiguration liegt außerhalb des Repos, das umgeschaltet wird.
//
// Ungefragt läuft kein git fetch. Die Liste nennt, wie alt der letzte ist;
// holen ist ein eigener POST. Umschalten gibt es nur nach Vorprüfung und
// Bestätigung, und der Server prüft unmittelbar davor erneut.

// branchesBudget ist die Frist der lokalen git-Aufrufe von GET /api/branches
// und der Vorprüfung. Eine Variable, damit ein Test sie verkürzen kann.
var branchesBudget = github.DefaultTimeout

// branchesGitHubBudget ist die eigene, kürzere Frist der gh-Abfragen von
// GET /api/branches. Sie teilen sich die Frist nicht mit der git-Liste: sonst
// verbrauchte ein langsames GitHub die Zeit, und die ganze Seite scheiterte an
// einer lokalen Liste, die in Millisekunden dastünde. Läuft sie ab, steht die
// Liste ohne GitHub-Daten da, und github.state sagt warum.
var branchesGitHubBudget = 10 * time.Second

// branchesReadRunner und branchesActionRunner sind die Nähte für Tests. Lesende
// Aufrufe gehen über den Runner der GitHub-Ansicht; Fetch und Switch brauchen
// auch stderr und laufen über den CombinedRunner.
var (
	branchesReadRunner   github.Runner = github.ExecRunner{}
	branchesActionRunner github.Runner = branches.CombinedRunner{}
)

// branchesProcesses ist die Prozessquelle der Sitzungsprüfung, in Tests
// austauschbar.
var branchesProcesses branches.ProcessSource = branches.ProcSource{}

// branchesActionMu serialisiert Fetch und Switch: zwei Fenster, die zugleich
// umschalten, sollen nicht gegeneinander laufen.
var branchesActionMu sync.Mutex

// branchesProject ist das aufgelöste Projekt einer Anfrage.
type branchesProject struct {
	projectDir string
	repoDir    string
	settings   project.GitSettings
	// settingsErr ist ein Fehler im Abschnitt git:. Die Liste steht trotzdem da,
	// Umschalten wird dann nicht angeboten.
	settingsErr error
}

// resolveBranchesProject ermittelt Projekt und Code-Repo wie die übrigen
// Handler: über project.Detect() aus dem Arbeitsverzeichnis des Servers.
func resolveBranchesProject() (branchesProject, string, string, bool) {
	environment := project.Detect()
	if !environment.Installed {
		return branchesProject{}, branches.StateNoProject, "Keine " + project.ConfigFileName + " gefunden. Ohne Projekt gibt es kein Code-Repo, dessen Branches hier stehen könnten.", false
	}
	config, err := project.ReadConfig(environment.ProjectDir)
	if err != nil {
		return branchesProject{}, branches.StateError, "Die Datei " + project.ConfigFileName + " ließ sich nicht lesen: " + err.Error(), false
	}
	resolved := branchesProject{
		projectDir: environment.ProjectDir,
		repoDir:    project.RepoRootDir(environment.ProjectDir, config),
	}
	if config.VCS != "" && config.VCS != "git" {
		return resolved, branches.StateNoVCS, "Das Projekt nutzt keine Versionskontrolle mit git (project.vcs: " + config.VCS + ").", false
	}
	resolved.settings, resolved.settingsErr = project.ReadGitSettings(environment.ProjectDir)
	return resolved, "", "", true
}

// applySettingsError sperrt das Umschalten, wenn der Abschnitt git: fehlerhaft
// ist, und sagt warum.
func applySettingsError(policy *branches.SwitchPolicy, err error) {
	if err == nil {
		return
	}
	policy.Offered = false
	policy.Message = "Der Abschnitt git: in " + project.ConfigFileName + " ist fehlerhaft, deshalb wird nicht umgeschaltet: " + err.Error()
}

// branchesGitHub beschafft die GitHub-Daten. „Mit gh" heißt tools.gh.status =
// enabled und gh bereit; bei unknown oder disabled startet kein gh-Prozess,
// auch wenn gh installiert ist — dieselbe Vorprüfung wie auf /github.
func branchesGitHub(ctx context.Context) branches.GitHubData {
	client, result, ok := githubClient()
	if !ok {
		return branches.GitHubData{Result: result, Environments: []string{}, Deployments: []github.Deployment{}}
	}
	data := branches.GitHubData{Enabled: true, Environments: []string{}, Deployments: []github.Deployment{}}

	repo, defaultBranch, result := client.RepoDefault(ctx)
	data.Result = result
	if result.State != github.StateOK {
		return data
	}
	data.Repo, data.DefaultBranch = repo, defaultBranch

	// PRs und Environments samt Deployments sind unabhängig voneinander und
	// laufen nebeneinander; das Budget teilen sie sich.
	var wait sync.WaitGroup
	wait.Add(2)
	go func() {
		defer wait.Done()
		pulls := client.FetchPulls(ctx, repo)
		data.Notes.Pulls = pulls.Result
		if pulls.State == github.StateOK {
			data.Pulls = pulls.Open
		}
	}()
	go func() {
		defer wait.Done()
		names, result := client.FetchEnvironments(ctx, repo)
		data.Notes.Environments = result
		data.Environments = names
		deployments, result := client.FetchLatestDeployments(ctx, repo, names)
		data.Notes.Deployments = result
		data.Deployments = deployments
	}()
	wait.Wait()
	return data
}

// branchesHandler ist GET /api/branches: Liste, Umgebungen und Worktrees.
//
// Die GitHub-Daten laufen vor der Liste, nicht neben ihr: die Liste ordnet nach
// dem Default-Branch von GitHub und hängt PRs und Deployments an, braucht die
// Daten also als Eingabe. Jede Seite hat ihr eigenes Budget; die git-Liste
// beginnt ihres erst, wenn gh fertig oder abgebrochen ist.
func branchesHandler(w http.ResponseWriter, r *http.Request) {
	resolved, state, message, ok := resolveBranchesProject()
	if !ok {
		writeJSON(w, http.StatusOK, emptyListing(resolved, state, message))
		return
	}
	ghCtx, ghCancel := context.WithTimeout(r.Context(), branchesGitHubBudget)
	gh := branchesGitHub(ghCtx)
	ghCancel()

	ctx, cancel := context.WithTimeout(r.Context(), branchesBudget)
	defer cancel()
	listing := branches.List(ctx, branches.Options{
		ProjectDir: resolved.projectDir,
		RepoDir:    resolved.repoDir,
		Settings:   resolved.settings,
		GitHub:     gh,
		Runner:     branchesReadRunner,
	})
	applySettingsError(&listing.Switch, resolved.settingsErr)
	writeGitHubJSON(w, r, listing)
}

// emptyListing ist die Antwort ohne Liste: ein Zustand mit Satz, die Listen
// leer und nicht null.
func emptyListing(resolved branchesProject, state string, message string) branches.Listing {
	return branches.Listing{
		State:        state,
		Message:      message,
		ProjectDir:   resolved.projectDir,
		RepoDir:      resolved.repoDir,
		Switch:       branches.SwitchPolicy{Status: project.GitSwitchUnknown, Allow: []string{}},
		GitHub:       branches.GitHubData{Environments: []string{}, Deployments: []github.Deployment{}},
		Groups:       []branches.Group{},
		Worktrees:    []branches.Worktree{},
		Environments: []branches.Environment{},
	}
}

// branchesFetchHandler ist POST /api/branches/fetch: der einzige Fetch der Seite,
// und nur auf Knopfdruck.
func branchesFetchHandler(w http.ResponseWriter, r *http.Request) {
	resolved, state, message, ok := resolveBranchesProject()
	if !ok {
		writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "state": state, "message": message})
		return
	}
	branchesActionMu.Lock()
	defer branchesActionMu.Unlock()
	writeJSON(w, http.StatusOK, branches.Fetch(r.Context(), branchesActionRunner, resolved.repoDir))
}

// checkOptionsFor baut die Vorgaben einer Vorprüfung. Die Sitzungsprüfung nimmt
// den eigenen Prozess aus: der Server arbeitet selbst im Projektverzeichnis.
func checkOptionsFor(resolved branchesProject, target string, remote string) branches.CheckOptions {
	return branches.CheckOptions{
		ProjectDir:    resolved.projectDir,
		RepoDir:       resolved.repoDir,
		Settings:      resolved.settings,
		SettingsError: resolved.settingsErr,
		Target:        target,
		Remote:        remote,
		Runner:        branchesReadRunner,
		Processes:     branchesProcesses,
		OwnPID:        os.Getpid(),
	}
}

// branchesSwitchCheckHandler ist GET /api/branches/switch-check?target=<branch>:
// jeder Prüfpunkt mit Ergebnis und Begründung, dazu der Prüfstempel. Sie liest
// nur.
func branchesSwitchCheckHandler(w http.ResponseWriter, r *http.Request) {
	target := r.URL.Query().Get("target")
	remote := r.URL.Query().Get("remote")
	resolved, state, message, ok := resolveBranchesProject()
	if !ok {
		writeJSON(w, http.StatusOK, branches.SwitchCheck{State: state, Message: message, Target: target, Checks: []branches.Check{}})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), branchesBudget)
	defer cancel()
	result := branches.RunCheck(ctx, checkOptionsFor(resolved, target, remote))
	status := http.StatusOK
	if result.State == branches.StateInvalidTarget {
		status = http.StatusBadRequest
	}
	writeJSON(w, status, result)
}

// switchRequest ist der Rumpf von POST /api/branches/switch.
type switchRequest struct {
	Target string `json:"target"`
	Remote string `json:"remote"`
	Stamp  string `json:"stamp"`
}

// branchesSwitchHandler ist POST /api/branches/switch. Der Server wiederholt die
// Vorprüfung; stimmt der Stempel nicht oder blockiert etwas, wird nichts
// ausgeführt und die neue Prüfung zurückgegeben (409). Sonst läuft genau der
// genannte git-Befehl, und die Antwort trägt vorher, nachher und die Ausgabe.
func branchesSwitchHandler(w http.ResponseWriter, r *http.Request) {
	var request switchRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, branches.SwitchResult{Message: "Anfrage nicht lesbar: " + err.Error()})
		return
	}
	resolved, state, message, ok := resolveBranchesProject()
	if !ok {
		writeJSON(w, http.StatusConflict, branches.SwitchResult{Reason: branches.SwitchReasonState, Message: "Nicht umgeschaltet: " + message, Check: branches.SwitchCheck{State: state, Message: message, Checks: []branches.Check{}}})
		return
	}

	branchesActionMu.Lock()
	defer branchesActionMu.Unlock()
	// Bewusst nicht der Kontext der Anfrage: ein geschlossener Tab soll einen
	// laufenden git switch nicht mittendrin abbrechen. Die Frist setzt Switch.
	ctx := context.WithoutCancel(r.Context())
	result := branches.Switch(ctx, checkOptionsFor(resolved, request.Target, request.Remote), request.Stamp, branchesActionRunner)

	status := http.StatusOK
	switch {
	case result.Check.State == branches.StateInvalidTarget:
		status = http.StatusBadRequest
	case !result.Executed:
		status = http.StatusConflict
	}
	writeJSON(w, status, result)
}
