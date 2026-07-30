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
	"runtime"
	"sort"
	"strings"
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

type gitPullResult struct {
	Output string `json:"output"`
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

type serverState struct {
	shutdown func()
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
	state := serverState{shutdown: stop}
	server := &http.Server{Handler: routes(state)}
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.Serve(listener)
	}()

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

func routes(state serverState) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/status", statusHandler)
	mux.HandleFunc("POST /api/repair-path", repairPathHandler)
	mux.HandleFunc("GET /api/projects", projectsHandler)
	mux.HandleFunc("GET /api/projects/scan", scanProjectsHandler)
	mux.HandleFunc("POST /api/projects", addProjectHandler)
	mux.HandleFunc("POST /api/projects/scan", saveScannedProjectsHandler)
	mux.HandleFunc("POST /api/git/pull", gitPullHandler)
	mux.HandleFunc("GET /api/docs", docsHandler)
	mux.HandleFunc("GET /api/docs/file", docFileHandler)
	mux.HandleFunc("GET /api/opencode/status", opencodeStatusHandler)
	mux.HandleFunc("POST /api/opencode/install", opencodeInstallHandler)
	mux.HandleFunc("GET /api/health", healthHandler)
	mux.HandleFunc("POST /api/shutdown", state.shutdownHandler)
	mux.HandleFunc("/", staticHandler)

	return mux
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
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

func (state serverState) shutdownHandler(w http.ResponseWriter, r *http.Request) {
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

	writeJSON(w, http.StatusOK, file)
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

	writeJSON(w, http.StatusOK, file)
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

	writeJSON(w, http.StatusOK, file)
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
