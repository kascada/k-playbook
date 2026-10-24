package webui

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// Die Todo-Endpunkte sind Hüllen über project/todos.go, dieselbe Fachlogik wie
// `k-playbook todo` und die MCP-Werkzeuge. Die Todos liegen in der
// Kephalaion-Collection des Projekts; ohne erreichbares Kephalaion gibt es
// keine, und die Antwort sagt das mit code kephalaion_unavailable.

// maxTodoRequest begrenzt den Körper eines POST auf /api/todos.
const maxTodoRequest = 1 << 16

// todosResponse ist eine Liste von Todos — offene, erledigte oder Treffer
// einer Suche.
//
// Message ist das Feld des Fehlerfalls: die Seite blendet bei jedem nichtleeren
// Message die Liste aus. Code ist dessen Code (kephalaion_unavailable,
// todos_not_migrated, …), damit die Seite den Zustand benennen kann. Hint ist
// daneben der Hinweis eines Zugriffs — etwa ein noch nicht übernommenes
// data/todos.json — und lässt Liste und Zählpille stehen.
type todosResponse struct {
	Available bool           `json:"available"`
	Todos     []project.Todo `json:"todos"`
	Message   string         `json:"message"`
	Code      string         `json:"code,omitempty"`
	Hint      string         `json:"hint"`
}

// todoResponse ist die Antwort eines schreibenden Endpunkts: der betroffene
// Eintrag oder der Fehler mit Code.
type todoResponse struct {
	OK      bool          `json:"ok"`
	Todo    *project.Todo `json:"todo,omitempty"`
	Message string        `json:"message,omitempty"`
	Code    string        `json:"code,omitempty"`
	Hint    string        `json:"hint,omitempty"`
}

// todoErrorCode ist der Code eines Fehlers der Fachlogik, sonst failed.
func todoErrorCode(err error) string {
	if code := project.DocRefErrorCode(err); code != "" {
		return code
	}
	return "failed"
}

// writeTodoList ruft eine Liste der Fachlogik und schreibt die Antwort.
func writeTodoList(w http.ResponseWriter, r *http.Request,
	list func(projectDir string) ([]project.Todo, project.TodoNotice, error)) {
	environment := project.Detect()
	if !environment.Installed {
		writeJSON(w, http.StatusOK, todosResponse{})
		return
	}
	todos, notice, err := list(environment.ProjectDir)
	if err != nil {
		writeJSON(w, http.StatusOK, todosResponse{Available: true, Message: err.Error(), Code: todoErrorCode(err), Hint: notice.Hint})
		return
	}
	writeJSON(w, http.StatusOK, todosResponse{Available: true, Todos: todos, Hint: notice.Hint})
}

func todosHandler(w http.ResponseWriter, r *http.Request) {
	writeTodoList(w, r, func(dir string) ([]project.Todo, project.TodoNotice, error) {
		return project.ListTodos(r.Context(), dir)
	})
}

// doneTodosHandler liefert die abgehakten Todos. Eigener Endpunkt, damit die
// Seite sie erst beim Aufklappen holt — wie bei den erledigten Tasks.
func doneTodosHandler(w http.ResponseWriter, r *http.Request) {
	writeTodoList(w, r, func(dir string) ([]project.Todo, project.TodoNotice, error) {
		return project.ListDoneTodos(r.Context(), dir)
	})
}

// searchTodosHandler sucht über offene und erledigte Todos (?q=…).
func searchTodosHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	writeTodoList(w, r, func(dir string) ([]project.Todo, project.TodoNotice, error) {
		return project.SearchTodos(r.Context(), dir, query)
	})
}

// writeTodoChange führt einen schreibenden Vorgang aus und schreibt die
// Antwort. Ein Fehler der Fachlogik ist eine Antwort mit ok false und Code,
// kein HTTP-Fehler: die Seite zeigt ihn an der Eingabe.
func writeTodoChange(w http.ResponseWriter, change func(projectDir string) (project.Todo, project.TodoNotice, error)) {
	environment := project.Detect()
	if !environment.Installed {
		writeJSON(w, http.StatusOK, todoResponse{Message: "Keine Projektkonfiguration gefunden.", Code: "project_not_found"})
		return
	}
	todo, notice, err := change(environment.ProjectDir)
	if err != nil {
		writeJSON(w, http.StatusOK, todoResponse{Message: err.Error(), Code: todoErrorCode(err), Hint: notice.Hint})
		return
	}
	writeJSON(w, http.StatusOK, todoResponse{OK: true, Todo: &todo, Hint: notice.Hint})
}

// addTodoHandler legt ein Todo an: {"text": "…", "origin": "…"}.
func addTodoHandler(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Text   string `json:"text"`
		Origin string `json:"origin"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxTodoRequest)).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, todoResponse{Message: "Anfrage nicht lesbar.", Code: "invalid_input"})
		return
	}
	writeTodoChange(w, func(dir string) (project.Todo, project.TodoNotice, error) {
		return project.AddTodo(r.Context(), dir, request.Text, request.Origin)
	})
}

// todoDoneHandler hakt ein Todo ab, todoReopenHandler öffnet es wieder.
func todoDoneHandler(w http.ResponseWriter, r *http.Request)   { setTodoDone(w, r, true) }
func todoReopenHandler(w http.ResponseWriter, r *http.Request) { setTodoDone(w, r, false) }

func setTodoDone(w http.ResponseWriter, r *http.Request, done bool) {
	id, err := strconv.Atoi(strings.TrimSpace(r.PathValue("id")))
	if err != nil || id < 1 {
		writeJSON(w, http.StatusBadRequest, todoResponse{Message: "Keine gültige Kennung.", Code: "invalid_input"})
		return
	}
	writeTodoChange(w, func(dir string) (project.Todo, project.TodoNotice, error) {
		return project.UpdateTodo(r.Context(), dir, id, nil, &done)
	})
}
