package webui

import (
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
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/kascada/k-playbook/installer/internal/pathcontract"
	"github.com/kascada/k-playbook/installer/internal/projects"
	"github.com/kascada/k-playbook/installer/internal/store"
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

func Run() error {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("GUI-Port oeffnen: %w", err)
	}

	server := &http.Server{Handler: routes()}
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
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			return fmt.Errorf("GUI-Server stoppen: %w", err)
		}
		return nil
	case err := <-serverErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("GUI-Server: %w", err)
	}
}

func routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/status", statusHandler)
	mux.HandleFunc("POST /api/repair-path", repairPathHandler)
	mux.HandleFunc("GET /api/projects", projectsHandler)
	mux.HandleFunc("GET /api/projects/scan", scanProjectsHandler)
	mux.HandleFunc("POST /api/projects", addProjectHandler)
	mux.HandleFunc("POST /api/projects/scan", saveScannedProjectsHandler)
	mux.HandleFunc("/", staticHandler)

	return mux
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

func scanProjectsHandler(w http.ResponseWriter, r *http.Request) {
	candidates, err := projects.ScanDefaultDev()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	sortProjects(candidates)

	writeJSON(w, http.StatusOK, candidates)
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
