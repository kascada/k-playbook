package webui

import (
	"bytes"
	"net/http"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// tasksResponse ist eine Liste von Tasks — offene oder erledigte.
type tasksResponse struct {
	Available bool           `json:"available"`
	Tasks     []project.Task `json:"tasks"`
	Message   string         `json:"message"`
}

// taskResponse ist eine einzelne Task, fertig gerendert.
type taskResponse struct {
	Available bool   `json:"available"`
	Path      string `json:"path"`
	Title     string `json:"title"`
	HTML      string `json:"html"`
	Message   string `json:"message"`
}

func tasksHandler(w http.ResponseWriter, r *http.Request) {
	environment := project.Detect()
	if !environment.Installed {
		writeJSON(w, http.StatusOK, tasksResponse{})
		return
	}

	tasks, err := project.ListTasks(environment.ProjectDir)
	if err != nil {
		writeJSON(w, http.StatusOK, tasksResponse{Available: true, Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, tasksResponse{Available: true, Tasks: tasks})
}

// doneTasksHandler liefert die erledigten Tasks. Eigener Endpunkt, weil die
// Seite sie erst beim Aufklappen holt: jede Datei wird für die Liste einmal
// gelesen, und done/ wächst mit jedem Lauf weiter.
func doneTasksHandler(w http.ResponseWriter, r *http.Request) {
	environment := project.Detect()
	if !environment.Installed {
		writeJSON(w, http.StatusOK, tasksResponse{})
		return
	}

	tasks, err := project.ListDoneTasks(environment.ProjectDir)
	if err != nil {
		writeJSON(w, http.StatusOK, tasksResponse{Available: true, Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, tasksResponse{Available: true, Tasks: tasks})
}

func taskFileHandler(w http.ResponseWriter, r *http.Request) {
	environment := project.Detect()
	if !environment.Installed {
		writeJSON(w, http.StatusOK, taskResponse{})
		return
	}

	task, content, err := project.ReadTask(environment.ProjectDir, r.URL.Query().Get("path"))
	if err != nil {
		writeJSON(w, http.StatusOK, taskResponse{Available: true, Message: err.Error()})
		return
	}

	var rendered bytes.Buffer
	if err := markdown.Convert(content, &rendered); err != nil {
		writeJSON(w, http.StatusOK, taskResponse{
			Available: true,
			Path:      task.Path,
			Title:     task.Title,
			Message:   "Markdown konnte nicht gerendert werden: " + err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, taskResponse{
		Available: true,
		Path:      task.Path,
		Title:     task.Title,
		HTML:      rendered.String(),
	})
}
