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
	mux.HandleFunc("POST /api/shutdown", state.shutdownHandler)
	mux.HandleFunc("/", staticHandler)

	return mux
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
