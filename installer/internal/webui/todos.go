package webui

import (
	"net/http"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// todosResponse ist eine Liste von Todos — offene oder erledigte.
//
// Message ist das Feld des Fehlerfalls: die Seite blendet bei jedem nichtleeren
// Message die Liste aus und setzt die Pille auf „Nicht lesbar". Hint ist
// daneben der Hinweis eines gelungenen Zugriffs — etwa eine zurückgebliebene
// Datei der früheren Markdown-Ablage — und lässt Liste und Zählpille stehen.
type todosResponse struct {
	Available bool           `json:"available"`
	Todos     []project.Todo `json:"todos"`
	Message   string         `json:"message"`
	Hint      string         `json:"hint"`
}

func todosHandler(w http.ResponseWriter, r *http.Request) {
	environment := project.Detect()
	if !environment.Installed {
		writeJSON(w, http.StatusOK, todosResponse{})
		return
	}

	todos, notice, err := project.ListTodos(environment.ProjectDir)
	if err != nil {
		writeJSON(w, http.StatusOK, todosResponse{Available: true, Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, todosResponse{Available: true, Todos: todos, Hint: notice.Hint})
}

// doneTodosHandler liefert die abgehakten Todos. Eigener Endpunkt, damit die
// Seite sie erst beim Aufklappen holt — wie bei den erledigten Tasks.
func doneTodosHandler(w http.ResponseWriter, r *http.Request) {
	environment := project.Detect()
	if !environment.Installed {
		writeJSON(w, http.StatusOK, todosResponse{})
		return
	}

	todos, notice, err := project.ListDoneTodos(environment.ProjectDir)
	if err != nil {
		writeJSON(w, http.StatusOK, todosResponse{Available: true, Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, todosResponse{Available: true, Todos: todos, Hint: notice.Hint})
}
