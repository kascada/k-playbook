package webui

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kascada/k-playbook/installer/internal/pathcontract"
	"github.com/kascada/k-playbook/installer/internal/projects"
	"github.com/kascada/k-playbook/installer/internal/store"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
)

//go:embed static/*
var staticFiles embed.FS

type apiError struct {
	Error string `json:"error"`
}

type projectRequest struct {
	Path        string `json:"path"`
	Environment string `json:"environment"`
	Selected    *bool  `json:"selected"`
}

type saveScannedRequest struct {
	Projects []projectRequest `json:"projects"`
}

type projectsResponse struct {
	Version  int           `json:"version"`
	Projects []projectView `json:"projects"`
}

type projectView struct {
	store.Project
	Setup projectCommandStatus `json:"setup"`
	Docs  projectCommandStatus `json:"docs"`
}

type projectCommandStatus struct {
	OK      bool   `json:"ok"`
	Path    string `json:"path"`
	Command string `json:"command"`
	Message string `json:"message"`
}

type gitPullResult struct {
	Output string `json:"output"`
}

type gitStatusResult struct {
	OK              bool   `json:"ok"`
	UpdateAvailable bool   `json:"updateAvailable"`
	Current         string `json:"current"`
	Remote          string `json:"remote"`
	Branch          string `json:"branch"`
	RemoteName      string `json:"remoteName"`
	Message         string `json:"message"`
}

type docEntry struct {
	Path  string `json:"path"`
	Title string `json:"title"`
}

type docContent struct {
	Path    string `json:"path"`
	Title   string `json:"title"`
	Content string `json:"content"`
	HTML    string `json:"html"`
}

type opencodeStatus struct {
	OK                       bool     `json:"ok"`
	ClaudeOK                 bool     `json:"claudeOk"`
	CommandDir               string   `json:"commandDir"`
	ClaudeCommandsDir        string   `json:"claudeCommandsDir"`
	ClaudeSkillsDir          string   `json:"claudeSkillsDir"`
	ConfigFile               string   `json:"configFile"`
	RepoCommands             int      `json:"repoCommands"`
	LinkedCommands           int      `json:"linkedCommands"`
	ClaudeLinkedCommands     int      `json:"claudeLinkedCommands"`
	RepoSkills               int      `json:"repoSkills"`
	ClaudeLinkedSkills       int      `json:"claudeLinkedSkills"`
	MissingCommands          []string `json:"missingCommands"`
	ClaudeMissingCommands    []string `json:"claudeMissingCommands"`
	ClaudeMissingSkills      []string `json:"claudeMissingSkills"`
	WrongCommands            []string `json:"wrongCommands"`
	ClaudeWrongCommands      []string `json:"claudeWrongCommands"`
	ClaudeWrongSkills        []string `json:"claudeWrongSkills"`
	NonSymlinkCommands       []string `json:"nonSymlinkCommands"`
	ClaudeNonSymlinkCommands []string `json:"claudeNonSymlinkCommands"`
	ClaudeNonSymlinkSkills   []string `json:"claudeNonSymlinkSkills"`
	StaleCommands            []string `json:"staleCommands"`
	ClaudeStaleCommands      []string `json:"claudeStaleCommands"`
	ClaudeStaleSkills        []string `json:"claudeStaleSkills"`
	SkillsPathOK             bool     `json:"skillsPathOk"`
	ConfigExists             bool     `json:"configExists"`
	ConfigEditable           bool     `json:"configEditable"`
	RestartRequired          bool     `json:"restartRequired"`
	ManualConfigSnippet      string   `json:"manualConfigSnippet"`
}

type opencodeInstallResult struct {
	Status  opencodeStatus `json:"status"`
	Changed bool           `json:"changed"`
	Message string         `json:"message"`
}

type devcontainerStatus struct {
	OK           bool                      `json:"ok"`
	HasProjects  bool                      `json:"hasProjects"`
	ProjectCount int                       `json:"projectCount"`
	ReadyCount   int                       `json:"readyCount"`
	Missing      []devcontainerProjectInfo `json:"missing"`
	MountEntry   string                    `json:"mountEntry"`
	Message      string                    `json:"message"`
}

type devcontainerProjectInfo struct {
	Path    string   `json:"path"`
	Missing []string `json:"missing"`
}

type devcontainerInstallRequest struct {
	Path string `json:"path"`
}

type devcontainerInstallResult struct {
	Status  devcontainerStatus `json:"status"`
	Changed bool               `json:"changed"`
	Output  string             `json:"output"`
	Message string             `json:"message"`
}

type securityToolSpec struct {
	Name     string
	Role     string
	Required bool
}

type securityToolStatus struct {
	OK              bool               `json:"ok"`
	ScopeOK         bool               `json:"scopeOk"`
	MissingRequired int                `json:"missingRequired"`
	Tools           []securityToolInfo `json:"tools"`
	VirtualEnv      string             `json:"virtualEnv"`
	PathWarnings    []string           `json:"pathWarnings"`
	Message         string             `json:"message"`
}

type securityToolInfo struct {
	Name     string `json:"name"`
	Role     string `json:"role"`
	Required bool   `json:"required"`
	Present  bool   `json:"present"`
	Path     string `json:"path"`
	Version  string `json:"version"`
}

type serverState struct {
	shutdown       func()
	mu             sync.Mutex
	lastClientSeen time.Time
	clientGoneAt   time.Time
}

const (
	clientGoneShutdownDelay = 10 * time.Second
	clientHeartbeatTimeout  = 30 * time.Second
	clientMonitorInterval   = 2 * time.Second
	toolVersionTimeout      = 2 * time.Second
)

var securityToolSpecs = []securityToolSpec{
	{Name: "gitleaks", Role: "Secret-Scanning", Required: true},
	{Name: "trufflehog", Role: "Tiefes Secret-Scanning", Required: true},
	{Name: "pip-audit", Role: "Python Dependency-CVEs", Required: true},
	{Name: "trivy", Role: "Filesystem/Container/IaC-CVEs", Required: true},
	{Name: "syft", Role: "SBOM-Erzeugung", Required: true},
	{Name: "grype", Role: "SBOM-/Dependency-CVE-Auswertung", Required: true},
	{Name: "docker", Role: "Optionaler Fallback fuer Container-Images", Required: false},
}

var markdown = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
)

func Run() error {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("GUI-Port oeffnen: %w", err)
	}

	ctx, stop := context.WithCancel(context.Background())
	state := &serverState{shutdown: stop}
	server := &http.Server{Handler: routes(state)}
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.Serve(listener)
	}()
	go state.monitorClient(ctx)

	url := "http://" + listener.Addr().String() + "/"
	fmt.Printf("k-playbook Installer GUI: %s\n", url)
	fmt.Println("Zum Beenden Ctrl+C druecken.")
	if err := openBrowser(url); err != nil {
		fmt.Printf("Browser konnte nicht automatisch geoeffnet werden: %v\n", err)
	}

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	defer signal.Stop(interrupt)

	select {
	case <-interrupt:
		stop()
	case <-ctx.Done():
	case err := <-serverErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("GUI-Server: %w", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("GUI-Server stoppen: %w", err)
	}

	select {
	case err := <-serverErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("GUI-Server: %w", err)
	default:
		return nil
	}
}

func routes(state *serverState) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/status", statusHandler)
	mux.HandleFunc("POST /api/repair-path", repairPathHandler)
	mux.HandleFunc("GET /api/projects", projectsHandler)
	mux.HandleFunc("DELETE /api/projects", removeProjectHandler)
	mux.HandleFunc("GET /api/projects/scan", scanProjectsHandler)
	mux.HandleFunc("POST /api/projects", addProjectHandler)
	mux.HandleFunc("POST /api/projects/scan", saveScannedProjectsHandler)
	mux.HandleFunc("GET /api/git/status", gitStatusHandler)
	mux.HandleFunc("POST /api/git/pull", gitPullHandler)
	mux.HandleFunc("GET /api/docs", docsHandler)
	mux.HandleFunc("GET /api/docs/file", docFileHandler)
	mux.HandleFunc("GET /api/opencode/status", opencodeStatusHandler)
	mux.HandleFunc("POST /api/opencode/install", opencodeInstallHandler)
	mux.HandleFunc("GET /api/security-tools/status", securityToolsStatusHandler)
	mux.HandleFunc("GET /api/devcontainer/status", devcontainerStatusHandler)
	mux.HandleFunc("POST /api/devcontainer/install", devcontainerInstallHandler)
	mux.HandleFunc("GET /api/health", state.healthHandler)
	mux.HandleFunc("POST /api/client-gone", state.clientGoneHandler)
	mux.HandleFunc("POST /api/shutdown", state.shutdownHandler)
	mux.HandleFunc("/", staticHandler)

	return mux
}

func (state *serverState) healthHandler(w http.ResponseWriter, r *http.Request) {
	state.noteClientSeen()
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (state *serverState) clientGoneHandler(w http.ResponseWriter, r *http.Request) {
	state.noteClientGone()
	w.WriteHeader(http.StatusNoContent)
}

func (state *serverState) noteClientSeen() {
	state.mu.Lock()
	defer state.mu.Unlock()
	state.lastClientSeen = time.Now()
	state.clientGoneAt = time.Time{}
}

func (state *serverState) noteClientGone() {
	state.mu.Lock()
	defer state.mu.Unlock()
	state.clientGoneAt = time.Now()
}

func (state *serverState) monitorClient(ctx context.Context) {
	ticker := time.NewTicker(clientMonitorInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			if state.shouldShutdownForMissingClient(now) {
				state.shutdown()
				return
			}
		}
	}
}

func (state *serverState) shouldShutdownForMissingClient(now time.Time) bool {
	state.mu.Lock()
	defer state.mu.Unlock()

	if !state.clientGoneAt.IsZero() && now.Sub(state.clientGoneAt) >= clientGoneShutdownDelay {
		return true
	}
	return !state.lastClientSeen.IsZero() && now.Sub(state.lastClientSeen) >= clientHeartbeatTimeout
}

func opencodeStatusHandler(w http.ResponseWriter, r *http.Request) {
	status, err := checkOpenCode()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusOK, status)
}

func opencodeInstallHandler(w http.ResponseWriter, r *http.Request) {
	changed, err := installOpenCode()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	status, err := checkOpenCode()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	status.RestartRequired = changed

	message := "OpenCode-Registrierung ist aktuell."
	if changed {
		message = "OpenCode-Registrierung aktualisiert. OpenCode danach neu starten."
	}
	writeJSON(w, http.StatusOK, opencodeInstallResult{Status: status, Changed: changed, Message: message})
}

func securityToolsStatusHandler(w http.ResponseWriter, r *http.Request) {
	status := checkSecurityTools()
	writeJSON(w, http.StatusOK, status)
}

func devcontainerStatusHandler(w http.ResponseWriter, r *http.Request) {
	status, err := checkDevcontainers()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusOK, status)
}

func devcontainerInstallHandler(w http.ResponseWriter, r *http.Request) {
	var request devcontainerInstallRequest
	if r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("Request lesen: %w", err))
			return
		}
	}

	output, changed, err := installDevcontainers(r.Context(), request.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	status, err := checkDevcontainers()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	message := "DevContainer-Integration ist aktuell."
	if changed {
		message = "DevContainer-Integration aktualisiert. DevContainer danach neu bauen oder neu starten."
	}
	writeJSON(w, http.StatusOK, devcontainerInstallResult{Status: status, Changed: changed, Output: output, Message: message})
}

func (state *serverState) shutdownHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "shutting_down"})
	go func() {
		time.Sleep(150 * time.Millisecond)
		state.shutdown()
	}()
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	result, err := pathcontract.Check()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func repairPathHandler(w http.ResponseWriter, r *http.Request) {
	result, err := pathcontract.Check()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if !result.OK {
		if err := pathcontract.Repair(result); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
	}

	result, err = pathcontract.Check()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func projectsHandler(w http.ResponseWriter, r *http.Request) {
	file, err := store.LoadProjects()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusOK, projectResponse(file))
}

func removeProjectHandler(w http.ResponseWriter, r *http.Request) {
	var request projectRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("Request lesen: %w", err))
		return
	}
	path, err := projects.NormalizePath(request.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	file, err := store.LoadProjects()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	file, removed := store.RemoveProject(file, path)
	if !removed {
		writeError(w, http.StatusNotFound, fmt.Errorf("Projekt nicht gespeichert: %s", path))
		return
	}
	if err := store.SaveProjects(file); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, projectResponse(file))
}

func gitStatusHandler(w http.ResponseWriter, r *http.Request) {
	status, err := checkGitStatus(r.Context())
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusOK, status)
}

func gitPullHandler(w http.ResponseWriter, r *http.Request) {
	root, err := repoRoot()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "pull", "--ff-only")
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		writeError(w, http.StatusGatewayTimeout, fmt.Errorf("git pull Timeout"))
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("git pull --ff-only: %w\n%s", err, strings.TrimSpace(string(output))))
		return
	}

	writeJSON(w, http.StatusOK, gitPullResult{Output: strings.TrimSpace(string(output))})
}

func checkGitStatus(parent context.Context) (gitStatusResult, error) {
	root, err := repoRoot()
	if err != nil {
		return gitStatusResult{}, err
	}

	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()

	branch, err := gitOutput(ctx, root, "branch", "--show-current")
	if err != nil {
		return gitStatusResult{}, err
	}
	if branch == "" {
		return gitStatusResult{Message: "Kein aktiver Git-Branch erkannt."}, nil
	}

	remoteName, err := gitOutput(ctx, root, "config", "--get", "branch."+branch+".remote")
	if err != nil || remoteName == "" {
		return gitStatusResult{Branch: branch, Message: "Kein Git-Upstream fuer diesen Branch konfiguriert."}, nil
	}

	mergeRef, err := gitOutput(ctx, root, "config", "--get", "branch."+branch+".merge")
	if err != nil || mergeRef == "" {
		return gitStatusResult{Branch: branch, RemoteName: remoteName, Message: "Kein Git-Upstream fuer diesen Branch konfiguriert."}, nil
	}
	remoteBranch := strings.TrimPrefix(mergeRef, "refs/heads/")

	current, err := gitOutput(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		return gitStatusResult{}, err
	}
	remote, err := gitOutput(ctx, root, "ls-remote", "--heads", remoteName, remoteBranch)
	if ctx.Err() == context.DeadlineExceeded {
		return gitStatusResult{}, fmt.Errorf("git update check Timeout")
	}
	if err != nil {
		return gitStatusResult{}, err
	}

	remoteCommit := firstField(remote)
	if remoteCommit == "" {
		return gitStatusResult{Branch: branch, RemoteName: remoteName, Current: current, Message: "Remote-Branch nicht gefunden."}, nil
	}

	updateAvailable := current != remoteCommit
	remoteKnownLocally := false
	remoteAncestorOfCurrent := false
	if updateAvailable && gitObjectExists(ctx, root, remoteCommit) {
		remoteKnownLocally = true
		updateAvailable = gitIsAncestor(ctx, root, current, remoteCommit)
		if !updateAvailable {
			remoteAncestorOfCurrent = gitIsAncestor(ctx, root, remoteCommit, current)
		}
	}

	status := gitStatusResult{
		OK:              true,
		UpdateAvailable: updateAvailable,
		Current:         current,
		Remote:          remoteCommit,
		Branch:          branch,
		RemoteName:      remoteName,
	}
	if status.UpdateAvailable {
		status.Message = "Neue k-playbook-Version verfuegbar."
	} else if current != remoteCommit && remoteKnownLocally && remoteAncestorOfCurrent {
		status.Message = "Lokaler Stand ist neuer als Remote."
	} else if current != remoteCommit && remoteKnownLocally {
		status.Message = "Remote unterscheidet sich, aber ist kein Fast-Forward-Update."
	} else {
		status.Message = "k-playbook ist aktuell."
	}
	return status, nil
}

func gitObjectExists(ctx context.Context, root string, object string) bool {
	cmd := exec.CommandContext(ctx, "git", "cat-file", "-e", object)
	cmd.Dir = root
	return cmd.Run() == nil
}

func gitIsAncestor(ctx context.Context, root string, ancestor string, descendant string) bool {
	cmd := exec.CommandContext(ctx, "git", "merge-base", "--is-ancestor", ancestor, descendant)
	cmd.Dir = root
	return cmd.Run() == nil
}

func gitOutput(ctx context.Context, root string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

func firstField(value string) string {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func docsHandler(w http.ResponseWriter, r *http.Request) {
	root, err := repoRoot()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	docsRoot := filepath.Join(root, "docs")
	entries := []docEntry{}
	err = filepath.WalkDir(docsRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.ToLower(filepath.Ext(path)) != ".md" {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		entries = append(entries, docEntry{Path: filepath.ToSlash(rel), Title: docTitle(path)})
		return nil
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("Docs lesen: %w", err))
		return
	}
	sort.Slice(entries, func(i int, j int) bool {
		return entries[i].Path < entries[j].Path
	})

	writeJSON(w, http.StatusOK, entries)
}

func docFileHandler(w http.ResponseWriter, r *http.Request) {
	root, err := repoRoot()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	rel := filepath.Clean(r.URL.Query().Get("path"))
	if rel == "." || filepath.IsAbs(rel) || strings.HasPrefix(rel, "..") || !strings.HasPrefix(filepath.ToSlash(rel), "docs/") {
		writeError(w, http.StatusBadRequest, fmt.Errorf("ungueltiger Docs-Pfad"))
		return
	}
	path := filepath.Join(root, rel)
	if strings.ToLower(filepath.Ext(path)) != ".md" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("nur Markdown-Dateien werden angezeigt"))
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("Doc lesen: %w", err))
		return
	}

	rendered, err := renderMarkdown(data)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("Markdown rendern: %w", err))
		return
	}

	writeJSON(w, http.StatusOK, docContent{Path: filepath.ToSlash(rel), Title: docTitle(path), Content: string(data), HTML: rendered})
}

func scanProjectsHandler(w http.ResponseWriter, r *http.Request) {
	candidates, err := scanProjects(r.URL.Query().Get("root"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	sortProjects(candidates)

	writeJSON(w, http.StatusOK, candidates)
}

func scanProjects(root string) ([]store.Project, error) {
	switch root {
	case "", "dev":
		return projects.ScanDefaultDev()
	case "home":
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("user home ermitteln: %w", err)
		}
		return projects.Scan(home)
	default:
		return nil, fmt.Errorf("ungueltiger Scan-Root: %s", root)
	}
}

func addProjectHandler(w http.ResponseWriter, r *http.Request) {
	var request projectRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("Request lesen: %w", err))
		return
	}

	project, err := projectFromRequest(request)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	file, err := store.LoadProjects()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	file = store.UpsertProject(file, project)
	if err := store.SaveProjects(file); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, projectResponse(file))
}

func saveScannedProjectsHandler(w http.ResponseWriter, r *http.Request) {
	var request saveScannedRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("Request lesen: %w", err))
		return
	}

	file, err := store.LoadProjects()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	for _, selected := range request.Projects {
		if selected.Selected != nil && !*selected.Selected {
			continue
		}
		project, err := projectFromRequest(selected)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		file = store.UpsertProject(file, project)
	}

	if err := store.SaveProjects(file); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, projectResponse(file))
}

func projectResponse(file store.ProjectsFile) projectsResponse {
	response := projectsResponse{Version: file.Version, Projects: make([]projectView, 0, len(file.Projects))}
	for _, project := range file.Projects {
		response.Projects = append(response.Projects, projectView{Project: project, Setup: checkProjectSetup(project.Path), Docs: checkProjectDocs(project.Path)})
	}
	return response
}

func checkProjectSetup(projectPath string) projectCommandStatus {
	status := projectCommandStatus{Command: "/k-setup"}
	for _, name := range []string{"K-PLAYBOOK.MD", "K-PLAYBOOK.md"} {
		path := filepath.Join(projectPath, name)
		info, err := os.Stat(path)
		if err == nil && !info.IsDir() {
			status.OK = true
			status.Path = path
			status.Message = name + " vorhanden."
			return status
		}
	}

	status.Path = filepath.Join(projectPath, "K-PLAYBOOK.MD")
	status.Message = "K-PLAYBOOK.MD fehlt."
	return status
}

func checkProjectDocs(projectPath string) projectCommandStatus {
	status := projectCommandStatus{Command: "/k-code2docs", Path: filepath.Join(projectPath, "docs")}
	hasDocs, err := hasMarkdownFiles(status.Path)
	if err != nil {
		status.Message = "docs-Verzeichnis fehlt oder ist nicht lesbar."
		return status
	}
	if !hasDocs {
		status.Message = "docs-Verzeichnis enthaelt noch keine Markdown-Dateien."
		return status
	}

	status.OK = true
	status.Message = "docs-Verzeichnis enthaelt Markdown-Dateien."
	return status
}

func hasMarkdownFiles(root string) (bool, error) {
	info, err := os.Stat(root)
	if err != nil {
		return false, err
	}
	if !info.IsDir() {
		return false, fmt.Errorf("kein Verzeichnis: %s", root)
	}

	found := false
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || entry.Name() == "node_modules" || entry.Name() == ".venv" || entry.Name() == "venv" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.ToLower(filepath.Ext(path)) == ".md" {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil && !errors.Is(err, filepath.SkipAll) {
		return false, err
	}
	return found, nil
}

func projectFromRequest(request projectRequest) (store.Project, error) {
	project, err := projects.ProjectFromPath(request.Path)
	if err != nil {
		return store.Project{}, err
	}

	if request.Environment != "" {
		environment := store.ProjectEnvironment(request.Environment)
		if !isValidEnvironment(environment) {
			return store.Project{}, fmt.Errorf("ungueltige Umgebung: %s", request.Environment)
		}
		project.Environment = environment
	}
	if request.Selected != nil {
		project.Selected = *request.Selected
	} else {
		project.Selected = true
	}

	return project, nil
}

func isValidEnvironment(environment store.ProjectEnvironment) bool {
	switch environment {
	case store.EnvironmentUnknown, store.EnvironmentPlain, store.EnvironmentVenv, store.EnvironmentDevContainer:
		return true
	default:
		return false
	}
}

func repoRoot() (string, error) {
	result, err := pathcontract.Check()
	if err != nil {
		return "", err
	}
	if !result.OK {
		return "", fmt.Errorf("Pfadvertrag nicht erfuellt: %s", result.Code)
	}

	root := result.Expected
	if realRoot, err := filepath.EvalSymlinks(root); err == nil {
		root = realRoot
	}
	if !pathcontract.IsKPlaybookRoot(root) {
		return "", fmt.Errorf("k-playbook-Repo nicht sicher erkannt: %s", root)
	}

	return root, nil
}

func docTitle(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}

	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
}

func renderMarkdown(data []byte) (string, error) {
	var buffer bytes.Buffer
	if err := markdown.Convert(data, &buffer); err != nil {
		return "", err
	}

	return buffer.String(), nil
}

func checkOpenCode() (opencodeStatus, error) {
	root, err := repoRoot()
	if err != nil {
		return opencodeStatus{}, err
	}

	commandDir, configFile, claudeCommandsDir, claudeSkillsDir, err := assistantPaths()
	if err != nil {
		return opencodeStatus{}, err
	}
	commands, err := repoCommands(root)
	if err != nil {
		return opencodeStatus{}, err
	}
	skills, err := repoSkills(root)
	if err != nil {
		return opencodeStatus{}, err
	}

	status := opencodeStatus{
		CommandDir:          commandDir,
		ClaudeCommandsDir:   claudeCommandsDir,
		ClaudeSkillsDir:     claudeSkillsDir,
		ConfigFile:          configFile,
		RepoCommands:        len(commands),
		RepoSkills:          len(skills),
		ManualConfigSnippet: manualOpenCodeConfigSnippet(),
	}

	status.LinkedCommands, status.MissingCommands, status.WrongCommands, status.NonSymlinkCommands = linkCheck(commandDir, commands, commandLinkName)
	status.ClaudeLinkedCommands, status.ClaudeMissingCommands, status.ClaudeWrongCommands, status.ClaudeNonSymlinkCommands = claudeCommandLinkCheck(claudeCommandsDir, commands, root)
	status.ClaudeLinkedSkills, status.ClaudeMissingSkills, status.ClaudeWrongSkills, status.ClaudeNonSymlinkSkills = linkCheck(claudeSkillsDir, skills, skillLinkName)

	status.StaleCommands = staleOpenCodeCommandLinks(commandDir, root)
	status.ClaudeStaleCommands = staleClaudeCommandLinks(claudeCommandsDir, root)
	status.ClaudeStaleSkills = staleClaudeSkillLinks(claudeSkillsDir, root)
	status.ConfigExists, status.SkillsPathOK, status.ConfigEditable = inspectOpenCodeConfig(configFile)
	status.ClaudeOK = status.RepoCommands > 0 && status.ClaudeLinkedCommands == status.RepoCommands && status.ClaudeLinkedSkills == status.RepoSkills && len(status.ClaudeMissingCommands) == 0 && len(status.ClaudeWrongCommands) == 0 && len(status.ClaudeNonSymlinkCommands) == 0 && len(status.ClaudeStaleCommands) == 0 && len(status.ClaudeMissingSkills) == 0 && len(status.ClaudeWrongSkills) == 0 && len(status.ClaudeNonSymlinkSkills) == 0 && len(status.ClaudeStaleSkills) == 0
	openCodeOK := status.RepoCommands > 0 && status.LinkedCommands == status.RepoCommands && len(status.MissingCommands) == 0 && len(status.WrongCommands) == 0 && len(status.NonSymlinkCommands) == 0 && len(status.StaleCommands) == 0 && status.SkillsPathOK
	status.OK = openCodeOK && status.ClaudeOK

	return status, nil
}

func installOpenCode() (bool, error) {
	root, err := repoRoot()
	if err != nil {
		return false, err
	}
	commandDir, configFile, claudeCommandsDir, claudeSkillsDir, err := assistantPaths()
	if err != nil {
		return false, err
	}
	commands, err := repoCommands(root)
	if err != nil {
		return false, err
	}
	if len(commands) == 0 {
		return false, fmt.Errorf("keine commands/k-*.md gefunden")
	}
	skills, err := repoSkills(root)
	if err != nil {
		return false, err
	}

	changed := false
	if err := os.MkdirAll(commandDir, 0o755); err != nil {
		return false, fmt.Errorf("OpenCode-Command-Verzeichnis anlegen: %w", err)
	}
	commandChanged, err := ensureLinks(commandDir, commands, commandLinkName)
	if err != nil {
		return false, err
	}
	changed = changed || commandChanged
	claudeCommandChanged, err := ensureClaudeCommandLinks(claudeCommandsDir, commands, root)
	if err != nil {
		return changed, err
	}
	changed = changed || claudeCommandChanged
	if err := os.MkdirAll(claudeSkillsDir, 0o755); err != nil {
		return false, fmt.Errorf("Claude-Skill-Verzeichnis anlegen: %w", err)
	}
	claudeSkillChanged, err := ensureLinks(claudeSkillsDir, skills, skillLinkName)
	if err != nil {
		return changed, err
	}
	changed = changed || claudeSkillChanged
	for _, stale := range staleOpenCodeCommandLinks(commandDir, root) {
		if err := os.Remove(filepath.Join(commandDir, stale)); err != nil && !os.IsNotExist(err) {
			return changed, fmt.Errorf("verwaisten Command-Link entfernen: %w", err)
		}
		changed = true
	}
	for _, stale := range staleClaudeCommandLinks(claudeCommandsDir, root) {
		if err := os.Remove(filepath.Join(claudeCommandsDir, stale)); err != nil && !os.IsNotExist(err) {
			return changed, fmt.Errorf("verwaisten Claude-Command-Link entfernen: %w", err)
		}
		changed = true
	}
	for _, stale := range staleClaudeSkillLinks(claudeSkillsDir, root) {
		if err := os.Remove(filepath.Join(claudeSkillsDir, stale)); err != nil && !os.IsNotExist(err) {
			return changed, fmt.Errorf("verwaisten Claude-Skill-Link entfernen: %w", err)
		}
		changed = true
	}

	configChanged, err := ensureOpenCodeSkillsPath(configFile)
	if err != nil {
		return changed, err
	}
	return changed || configChanged, nil
}

func checkSecurityTools() securityToolStatus {
	status := securityToolStatus{
		ScopeOK:      true,
		VirtualEnv:   os.Getenv("VIRTUAL_ENV"),
		PathWarnings: projectVenvPathEntries(os.Getenv("PATH")),
	}
	if status.VirtualEnv != "" || len(status.PathWarnings) > 0 {
		status.ScopeOK = false
	}

	for _, spec := range securityToolSpecs {
		info := securityToolInfo{Name: spec.Name, Role: spec.Role, Required: spec.Required}
		if path, err := exec.LookPath(spec.Name); err == nil {
			info.Present = true
			info.Path = path
			info.Version = securityToolVersion(spec.Name)
		} else if spec.Required {
			status.MissingRequired++
		}
		status.Tools = append(status.Tools, info)
	}

	status.OK = status.ScopeOK && status.MissingRequired == 0
	switch {
	case !status.ScopeOK:
		status.Message = "Aktives oder im PATH sichtbares Projekt-venv gefunden. Security-Tools erst nach deactivate/PATH-Bereinigung host-global bewerten."
	case status.MissingRequired > 0:
		status.Message = fmt.Sprintf("%d Pflicht-Tools fehlen. Installation spaeter separat klaeren.", status.MissingRequired)
	default:
		status.Message = "Alle Pflicht-Tools sind vorhanden."
	}

	return status
}

func securityToolVersion(tool string) string {
	args := []string{"--version"}
	if tool == "gitleaks" || tool == "syft" || tool == "grype" {
		args = []string{"version"}
	}
	output, timedOut := securityToolVersionOutput(tool, args...)
	if len(output) == 0 && len(args) == 1 && args[0] == "version" {
		output, timedOut = securityToolVersionOutput(tool, "--version")
	}
	if timedOut {
		return "Timeout bei Versionsabfrage"
	}

	line := strings.TrimSpace(string(output))
	if line == "" {
		return "Version unbekannt"
	}
	if index := strings.IndexByte(line, '\n'); index >= 0 {
		line = line[:index]
	}
	return line
}

func securityToolVersionOutput(tool string, args ...string) ([]byte, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), toolVersionTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, tool, args...)
	output, _ := cmd.CombinedOutput()
	return output, ctx.Err() == context.DeadlineExceeded
}

func projectVenvPathEntries(value string) []string {
	warnings := []string{}
	for _, entry := range filepath.SplitList(value) {
		if isProjectVenvPath(entry) {
			warnings = append(warnings, entry)
		}
	}
	return warnings
}

func isProjectVenvPath(path string) bool {
	path = filepath.ToSlash(strings.TrimRight(path, `/\\`))
	if path == "" {
		return false
	}
	parts := strings.Split(path, "/")
	for _, part := range parts {
		switch part {
		case ".venv", "venv", "env":
			return true
		}
	}
	return false
}

func checkDevcontainers() (devcontainerStatus, error) {
	file, err := store.LoadProjects()
	if err != nil {
		return devcontainerStatus{}, err
	}

	status := devcontainerStatus{MountEntry: devcontainerMountEntry()}
	for _, project := range file.Projects {
		if !project.Selected || project.Environment != store.EnvironmentDevContainer {
			continue
		}
		status.HasProjects = true
		status.ProjectCount++

		missing := missingDevcontainerIntegration(project.Path)
		if len(missing) == 0 {
			status.ReadyCount++
			continue
		}
		status.Missing = append(status.Missing, devcontainerProjectInfo{Path: project.Path, Missing: missing})
	}

	status.OK = status.HasProjects && status.ReadyCount == status.ProjectCount
	if !status.HasProjects {
		status.Message = "Kein gespeichertes DevContainer-Projekt ausgewaehlt."
	} else if status.OK {
		status.Message = "Alle DevContainer-Projekte binden ~/dev/k-playbook ein."
	} else {
		status.Message = fmt.Sprintf("%d von %d DevContainer-Projekten brauchen die k-playbook-Integration.", len(status.Missing), status.ProjectCount)
	}

	return status, nil
}

func installDevcontainers(ctx context.Context, projectPath string) (string, bool, error) {
	root, err := repoRoot()
	if err != nil {
		return "", false, err
	}
	file, err := store.LoadProjects()
	if err != nil {
		return "", false, err
	}

	script := filepath.Join(root, "scripts", "install-devcontainer-k-playbook.sh")
	if _, err := os.Stat(script); err != nil {
		return "", false, fmt.Errorf("DevContainer-Installationsscript pruefen: %w", err)
	}
	if strings.TrimSpace(projectPath) != "" {
		projectPath, err = projects.NormalizePath(projectPath)
		if err != nil {
			return "", false, err
		}
	}

	var output strings.Builder
	changed := false
	found := false
	for _, project := range file.Projects {
		if !project.Selected || project.Environment != store.EnvironmentDevContainer {
			continue
		}
		if projectPath != "" && project.Path != projectPath {
			continue
		}
		found = true
		if len(missingDevcontainerIntegration(project.Path)) == 0 {
			continue
		}

		commandCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		cmd := exec.CommandContext(commandCtx, script, project.Path)
		cmd.Dir = root
		data, err := cmd.CombinedOutput()
		cancel()
		if output.Len() > 0 {
			output.WriteString("\n")
		}
		output.WriteString(strings.TrimSpace(string(data)))
		if commandCtx.Err() == context.DeadlineExceeded {
			return output.String(), changed, fmt.Errorf("DevContainer-Installation Timeout: %s", project.Path)
		}
		if err != nil {
			return output.String(), changed, fmt.Errorf("DevContainer-Integration fuer %s: %w\n%s", project.Path, err, strings.TrimSpace(string(data)))
		}
		changed = true
	}
	if projectPath != "" && !found {
		return output.String(), changed, fmt.Errorf("gespeichertes DevContainer-Projekt nicht gefunden: %s", projectPath)
	}

	return output.String(), changed, nil
}

func missingDevcontainerIntegration(projectPath string) []string {
	devcontainerJSON := filepath.Join(projectPath, ".devcontainer", "devcontainer.json")
	setupScript := filepath.Join(projectPath, ".devcontainer", "setup-k-playbook.sh")
	missing := []string{}

	data, err := os.ReadFile(devcontainerJSON)
	if err != nil {
		return []string{".devcontainer/devcontainer.json fehlt oder ist nicht lesbar"}
	}
	content := string(data)
	if !strings.Contains(content, devcontainerMountEntry()) {
		missing = append(missing, "Bind-Mount ~/dev/k-playbook -> /workspaces/k-playbook")
	}
	if !hasDevcontainerCommand(content, "postCreateCommand", "sudo bash .devcontainer/setup-k-playbook.sh --install-security-tools") {
		missing = append(missing, "postCreateCommand fuer setup-k-playbook.sh")
	}
	if !hasDevcontainerCommand(content, "postStartCommand", "sudo bash .devcontainer/setup-k-playbook.sh") {
		missing = append(missing, "postStartCommand fuer setup-k-playbook.sh")
	}
	if info, err := os.Stat(setupScript); err != nil || info.IsDir() || info.Mode()&0o111 == 0 {
		missing = append(missing, ".devcontainer/setup-k-playbook.sh fehlt oder ist nicht ausfuehrbar")
	}

	return missing
}

func devcontainerMountEntry() string {
	return "source=${localEnv:HOME}/dev/k-playbook,target=/workspaces/k-playbook,type=bind"
}

func hasDevcontainerCommand(content string, name string, desired string) bool {
	pattern := regexp.MustCompile(`"` + regexp.QuoteMeta(name) + `"\s*:\s*"((?:\\.|[^"\\])*)"`)
	match := pattern.FindStringSubmatch(content)
	if len(match) != 2 {
		return false
	}

	var value string
	if err := json.Unmarshal([]byte(`"`+match[1]+`"`), &value); err != nil {
		return false
	}
	for _, part := range strings.Split(value, "&&") {
		if strings.TrimSpace(part) == desired {
			return true
		}
	}
	return false
}

func assistantPaths() (string, string, string, string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", "", "", fmt.Errorf("user home ermitteln: %w", err)
	}
	configDir := filepath.Join(home, ".config", "opencode")
	commandDir := filepath.Join(configDir, "command")
	claudeCommandsDir := filepath.Join(home, ".claude", "commands")
	claudeSkillsDir := filepath.Join(home, ".claude", "skills")
	jsonc := filepath.Join(configDir, "opencode.jsonc")
	json := filepath.Join(configDir, "opencode.json")
	if _, err := os.Stat(jsonc); err == nil {
		return commandDir, jsonc, claudeCommandsDir, claudeSkillsDir, nil
	}
	if _, err := os.Stat(json); err == nil {
		return commandDir, json, claudeCommandsDir, claudeSkillsDir, nil
	}
	return commandDir, jsonc, claudeCommandsDir, claudeSkillsDir, nil
}

func repoCommands(root string) ([]string, error) {
	commands, err := filepath.Glob(filepath.Join(root, "commands", "k-*.md"))
	if err != nil {
		return nil, fmt.Errorf("Commands suchen: %w", err)
	}
	sort.Strings(commands)
	return commands, nil
}

func repoSkills(root string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(root, "ks-*", "SKILL.md"))
	if err != nil {
		return nil, fmt.Errorf("Skills suchen: %w", err)
	}
	skills := make([]string, 0, len(matches))
	for _, match := range matches {
		skills = append(skills, filepath.Dir(match))
	}
	sort.Strings(skills)
	return skills, nil
}

func linkCheck(dir string, targets []string, linkName func(string) string) (int, []string, []string, []string) {
	linked := 0
	missing := []string{}
	wrong := []string{}
	nonSymlink := []string{}
	for _, target := range targets {
		name := linkName(target)
		link := filepath.Join(dir, name)
		info, err := os.Lstat(link)
		if err != nil {
			if os.IsNotExist(err) {
				missing = append(missing, name)
				continue
			}
			wrong = append(wrong, name)
			continue
		}
		if info.Mode()&os.ModeSymlink == 0 {
			nonSymlink = append(nonSymlink, name)
			continue
		}
		resolved, err := filepath.EvalSymlinks(link)
		if err != nil || !samePath(resolved, target) {
			wrong = append(wrong, name)
			continue
		}
		linked++
	}
	return linked, missing, wrong, nonSymlink
}

func claudeCommandLinkCheck(dir string, commands []string, root string) (int, []string, []string, []string) {
	if directorySymlinkTargetOK(dir, filepath.Join(root, "commands")) {
		return len(commands), []string{}, []string{}, []string{}
	}

	return linkCheck(dir, commands, commandLinkName)
}

func ensureClaudeCommandLinks(dir string, commands []string, root string) (bool, error) {
	commandsRoot := filepath.Join(root, "commands")
	if directorySymlinkTargetOK(dir, commandsRoot) {
		return false, nil
	}

	info, err := os.Lstat(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			return false, fmt.Errorf("Claude-Command-Verzeichnis pruefen: %w", err)
		}
		if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
			return false, fmt.Errorf("Claude-Verzeichnis anlegen: %w", err)
		}
		return true, os.Symlink(commandsRoot, dir)
	}

	if info.Mode()&os.ModeSymlink != 0 {
		if err := os.Remove(dir); err != nil {
			return false, fmt.Errorf("falschen Claude-Command-Link entfernen: %w", err)
		}
		return true, os.Symlink(commandsRoot, dir)
	}

	if !info.IsDir() {
		return false, nil
	}

	return ensureLinks(dir, commands, commandLinkName)
}

func ensureLinks(dir string, targets []string, linkName func(string) string) (bool, error) {
	changed := false
	for _, target := range targets {
		link := filepath.Join(dir, linkName(target))
		needsUpdate, canReplace := linkStatus(link, target)
		if needsUpdate && canReplace {
			if err := os.Remove(link); err != nil && !os.IsNotExist(err) {
				return changed, fmt.Errorf("alten Link entfernen: %w", err)
			}
			if err := os.Symlink(target, link); err != nil {
				return changed, fmt.Errorf("Link anlegen: %w", err)
			}
			changed = true
		}
	}
	return changed, nil
}

func commandLinkName(path string) string {
	return filepath.Base(path)
}

func skillLinkName(path string) string {
	return filepath.Base(path)
}

func staleOpenCodeCommandLinks(commandDir string, root string) []string {
	entries, err := os.ReadDir(commandDir)
	if err != nil {
		return []string{}
	}
	stale := []string{}
	commandsRoot := filepath.Join(root, "commands")
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "k-") || !strings.HasSuffix(name, ".md") {
			continue
		}
		path := filepath.Join(commandDir, name)
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			continue
		}
		target, err := os.Readlink(path)
		if err != nil {
			continue
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(commandDir, target)
		}
		target = filepath.Clean(target)
		if isWithin(target, commandsRoot) {
			if _, err := os.Stat(target); os.IsNotExist(err) {
				stale = append(stale, name)
			}
		}
	}
	sort.Strings(stale)
	return stale
}

func staleClaudeCommandLinks(commandDir string, root string) []string {
	if directorySymlinkTargetOK(commandDir, filepath.Join(root, "commands")) {
		return []string{}
	}

	return staleOpenCodeCommandLinks(commandDir, root)
}

func staleClaudeSkillLinks(skillsDir string, root string) []string {
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return []string{}
	}
	stale := []string{}
	for _, entry := range entries {
		path := filepath.Join(skillsDir, entry.Name())
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			continue
		}
		target, err := os.Readlink(path)
		if err != nil {
			continue
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(skillsDir, target)
		}
		target = filepath.Clean(target)
		if isWithin(target, root) {
			if _, err := os.Stat(target); os.IsNotExist(err) {
				stale = append(stale, entry.Name())
			}
		}
	}
	sort.Strings(stale)
	return stale
}

func inspectOpenCodeConfig(path string) (bool, bool, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, false, true
		}
		return true, false, false
	}
	content := string(data)
	return true, strings.Contains(content, "~/dev/k-playbook"), configCanBeAutoEdited(content)
}

func ensureOpenCodeSkillsPath(path string) (bool, error) {
	configExists, skillsOK, editable := inspectOpenCodeConfig(path)
	if skillsOK {
		return false, nil
	}
	if !editable {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, fmt.Errorf("OpenCode-Konfig-Verzeichnis anlegen: %w", err)
	}
	if !configExists {
		return true, os.WriteFile(path, []byte(manualOpenCodeConfigSnippet()), 0o644)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("OpenCode-Konfig lesen: %w", err)
	}
	content := strings.TrimSpace(string(data))
	if content == "{}" || content == "" {
		return true, os.WriteFile(path, []byte(manualOpenCodeConfigSnippet()), 0o644)
	}
	insert := "\n  ,\"skills\": {\n    \"paths\": [\"~/dev/k-playbook\"]\n  }\n"
	index := strings.LastIndex(content, "}")
	if index < 0 {
		return false, fmt.Errorf("OpenCode-Konfig nicht sicher automatisch editierbar: %s", path)
	}
	updated := content[:index] + insert + content[index:] + "\n"
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return false, fmt.Errorf("OpenCode-Konfig schreiben: %w", err)
	}
	return true, nil
}

func configCanBeAutoEdited(content string) bool {
	return !strings.Contains(content, "skills") || strings.TrimSpace(content) == "{}" || strings.TrimSpace(content) == ""
}

func linkStatus(link string, target string) (bool, bool) {
	info, err := os.Lstat(link)
	if err != nil {
		return true, true
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return true, false
	}
	resolved, err := filepath.EvalSymlinks(link)
	return err != nil || !samePath(resolved, target), true
}

func directorySymlinkTargetOK(path string, target string) bool {
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		return false
	}
	resolved, err := filepath.EvalSymlinks(path)
	return err == nil && samePath(resolved, target)
}

func manualOpenCodeConfigSnippet() string {
	return "{\n  \"$schema\": \"https://opencode.ai/config.json\",\n  \"skills\": {\n    \"paths\": [\"~/dev/k-playbook\"]\n  }\n}\n"
}

func samePath(left string, right string) bool {
	leftReal, leftErr := filepath.EvalSymlinks(left)
	rightReal, rightErr := filepath.EvalSymlinks(right)
	if leftErr == nil {
		left = leftReal
	}
	if rightErr == nil {
		right = rightReal
	}
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	if leftErr == nil {
		left = leftAbs
	}
	if rightErr == nil {
		right = rightAbs
	}
	return filepath.Clean(left) == filepath.Clean(right)
}

func isWithin(path string, root string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func staticHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		data, err := staticFiles.ReadFile("static/index.html")
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(data)
		return
	}

	if !strings.HasPrefix(r.URL.Path, "/static/") {
		http.NotFound(w, r)
		return
	}

	sub, err := fs.Sub(staticFiles, ".")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	http.FileServer(http.FS(sub)).ServeHTTP(w, r)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, apiError{Error: err.Error()})
}

func sortProjects(values []store.Project) {
	sort.Slice(values, func(i int, j int) bool {
		return values[i].Path < values[j].Path
	})
}

func openBrowser(url string) error {
	var command string
	var args []string

	switch runtime.GOOS {
	case "darwin":
		command = "open"
		args = []string{url}
	case "windows":
		command = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	default:
		command = "xdg-open"
		args = []string{url}
	}

	return exec.Command(command, args...).Start()
}
