package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/kephalaion/kephtest"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// todoGUIProject legt ein Projekt an, macht es zum Arbeitsverzeichnis und
// legt seine Todos in eine Collection im Speicher.
func todoGUIProject(t *testing.T) (string, *kephtest.Store) {
	t.Helper()
	root := t.TempDir()
	if err := project.CreateConfig(root, "."); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(project.LocalDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	chdir(t, root)
	store := kephtest.New()
	original := project.OpenTodoStore
	project.OpenTodoStore = func(string) (project.TodoStore, string, error) { return store, "kamran", nil }
	t.Cleanup(func() { project.OpenTodoStore = original })
	return root, store
}

func serveTodos(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	routes(&serverState{}).ServeHTTP(recorder, request)
	return recorder
}

func decodeJSONBody[T any](t *testing.T, recorder *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(recorder.Body.Bytes(), &out); err != nil {
		t.Fatalf("Antwort nicht lesbar: %v — %s", err, recorder.Body.String())
	}
	return out
}

func TestTodoEndpunkteAnlegenAbhakenSuchen(t *testing.T) {
	_, store := todoGUIProject(t)

	added := decodeJSONBody[todoResponse](t, serveTodos(t, http.MethodPost, "/api/todos", `{"text":"Über die GUI"}`))
	if !added.OK || added.Todo == nil || added.Todo.ID != 1 {
		t.Fatalf("anlegen: %+v", added)
	}
	if _, ok := store.Content("todos/001-ueber-die-gui.md"); !ok {
		t.Fatalf("Dokument fehlt: %v", store.Names())
	}
	open := decodeJSONBody[todosResponse](t, serveTodos(t, http.MethodGet, "/api/todos", ""))
	if !open.Available || len(open.Todos) != 1 || open.Message != "" {
		t.Fatalf("offene: %+v", open)
	}

	done := decodeJSONBody[todoResponse](t, serveTodos(t, http.MethodPost, "/api/todos/1/done", ""))
	if !done.OK || done.Todo.Done == "" {
		t.Fatalf("abhaken: %+v", done)
	}
	doneList := decodeJSONBody[todosResponse](t, serveTodos(t, http.MethodGet, "/api/todos/done", ""))
	if len(doneList.Todos) != 1 || doneList.Todos[0].ID != 1 {
		t.Fatalf("erledigte: %+v", doneList)
	}
	found := decodeJSONBody[todosResponse](t, serveTodos(t, http.MethodGet, "/api/todos/search?q=gui", ""))
	if len(found.Todos) != 1 || found.Todos[0].Done == "" {
		t.Fatalf("Suche findet Erledigte nicht: %+v", found)
	}
	reopened := decodeJSONBody[todoResponse](t, serveTodos(t, http.MethodPost, "/api/todos/1/reopen", ""))
	if !reopened.OK || reopened.Todo.Done != "" {
		t.Fatalf("wiederöffnen: %+v", reopened)
	}

	missing := decodeJSONBody[todoResponse](t, serveTodos(t, http.MethodPost, "/api/todos/9/done", ""))
	if missing.OK || missing.Code != "not_found" {
		t.Fatalf("unbekannte Kennung: %+v", missing)
	}
	if recorder := serveTodos(t, http.MethodPost, "/api/todos/x/done", ""); recorder.Code != http.StatusBadRequest {
		t.Fatalf("ungültige Kennung: %d", recorder.Code)
	}
	empty := decodeJSONBody[todoResponse](t, serveTodos(t, http.MethodPost, "/api/todos", `{"text":"  "}`))
	if empty.OK || empty.Code != "invalid_input" {
		t.Fatalf("leerer Text: %+v", empty)
	}
}

func TestTodoEndpunkteMeldenZustaende(t *testing.T) {
	root, store := todoGUIProject(t)
	if err := os.MkdirAll(project.TodoDataDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(project.TodoFile(root), []byte(`{"todos":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	list := decodeJSONBody[todosResponse](t, serveTodos(t, http.MethodGet, "/api/todos", ""))
	if list.Message != "" || !strings.Contains(list.Hint, "k-playbook todo migrate") {
		t.Fatalf("Liste vor der Übernahme: %+v", list)
	}
	rejected := decodeJSONBody[todoResponse](t, serveTodos(t, http.MethodPost, "/api/todos", `{"text":"neu"}`))
	if rejected.OK || rejected.Code != project.TodoCodeNotMigrated || rejected.Hint == "" {
		t.Fatalf("anlegen vor der Übernahme: %+v", rejected)
	}
	if len(store.Names()) != 0 {
		t.Fatalf("trotz Sperre angelegt: %v", store.Names())
	}

	project.OpenTodoStore = func(string) (project.TodoStore, string, error) {
		_, err := project.OpenUserDocs(root)
		return nil, "", err
	}
	unavailable := decodeJSONBody[todosResponse](t, serveTodos(t, http.MethodGet, "/api/todos", ""))
	if unavailable.Code != project.DocCodeKephalaionUnavailable || unavailable.Message == "" || !unavailable.Available {
		t.Fatalf("ohne Kephalaion: %+v", unavailable)
	}
}

func TestTodoPostFremderHerkunftAbgewiesen(t *testing.T) {
	_, store := todoGUIProject(t)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/todos", strings.NewReader(`{"text":"x"}`))
	request.Header.Set("Origin", "http://fremd.example")
	routes(&serverState{}).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden || len(store.Names()) != 0 {
		t.Fatalf("fremde Herkunft: %d %v", recorder.Code, store.Names())
	}
}
