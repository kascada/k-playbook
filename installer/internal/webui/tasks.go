package webui

import (
	"bytes"
	"net/http"

	"github.com/kascada/k-playbook/installer/internal/project"
	"github.com/kascada/k-playbook/installer/internal/review"
)

// workflowsResponse sind die beiden Zahlen des Workflow-Blocks. Er verweist nur
// weiter, deshalb genügt je Ziel, wie viel dort liegt.
type workflowsResponse struct {
	Available bool   `json:"available"`
	Reviews   int    `json:"reviews"`
	Tasks     int    `json:"tasks"`
	Message   string `json:"message"`
}

// tasksResponse ist die Liste der offenen Tasks.
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

// workflowsHandler zählt beide Seiten, ohne sie zu laden. Bewusst nicht über
// /api/reviews: das stellt einen ganzen Lauf zusammen und prüft dafür jedes
// Werkzeug — viel zu viel Arbeit für eine Zahl auf einem Knopf.
func workflowsHandler(w http.ResponseWriter, r *http.Request) {
	environment := project.Detect()
	if !environment.Installed {
		writeJSON(w, http.StatusOK, workflowsResponse{})
		return
	}

	response := workflowsResponse{Available: true}

	runs, err := review.ListRuns(project.LocalDir(environment.ProjectDir))
	if err != nil {
		response.Message = err.Error()
	}
	response.Reviews = len(runs)

	tasks, err := project.ListTasks(environment.ProjectDir)
	if err != nil && response.Message == "" {
		response.Message = err.Error()
	}
	response.Tasks = len(tasks)

	writeJSON(w, http.StatusOK, response)
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
