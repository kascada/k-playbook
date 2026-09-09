package mcpserver

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// Die vier Todo-Werkzeuge sind dünne Hüllen über denselben Funktionen in
// project/, die auch das Subkommando `k-playbook todo` und die Oberfläche
// benutzen. Zweite Fachlogik gibt es hier nicht.
//
// Tragende Schicht ist das Subkommando: der MCP-Server wird pro Projekt
// registriert und kann fehlen, das Binary ist mit der Installation da.
const (
	todoToolList   = "k_playbook_todo_list"
	todoToolAdd    = "k_playbook_todo_add"
	todoToolUpdate = "k_playbook_todo_update"
	todoToolDelete = "k_playbook_todo_delete"
)

type todoBaseInput struct {
	ProjectDir string `json:"projectDir" jsonschema:"Pflicht. Verzeichnis, ab dem aufwärts nach K-PLAYBOOK.yaml gesucht wird. Relative Pfade werden relativ zum Arbeitsverzeichnis des MCP-Servers aufgelöst."`
}

type todoListInput struct {
	todoBaseInput
	IncludeDone bool `json:"includeDone,omitempty" jsonschema:"Nimmt die erledigten Einträge mit auf. Ohne Angabe kommen nur die offenen."`
}

type todoAddInput struct {
	todoBaseInput
	Text   string `json:"text" jsonschema:"Pflicht. Der Text des neuen Eintrags."`
	Origin string `json:"origin,omitempty" jsonschema:"Optionale Herkunftsnotiz, etwa \"Task 026\"."`
}

type todoUpdateInput struct {
	todoBaseInput
	ID   int     `json:"id" jsonschema:"Pflicht. Kennung des Eintrags."`
	Text *string `json:"text,omitempty" jsonschema:"Neuer Text. Ohne Angabe bleibt der bestehende."`
	Done *bool   `json:"done,omitempty" jsonschema:"true hakt ab, false öffnet wieder. Ohne Angabe bleibt der Erledigt-Zustand."`
}

type todoDeleteInput struct {
	todoBaseInput
	ID int `json:"id" jsonschema:"Pflicht. Kennung des Eintrags. Löschen entfernt ihn endgültig; zum Abhaken dient k_playbook_todo_update."`
}

// todoEnvelope ist die Antwortform aller vier Werkzeuge.
//
// migrated meldet, dass dieser Aufruf eine vorhandene TODO.md übersetzt hat;
// hint meldet eine zurückgebliebene TODO.md. Beides ist kein Fehler — der
// Zugriffsweg scheitert daran nicht.
type todoEnvelope struct {
	OK         bool               `json:"ok"`
	Tool       string             `json:"tool"`
	ProjectDir string             `json:"projectDir"`
	Todos      []project.Todo     `json:"todos,omitempty"`
	Todo       *project.Todo      `json:"todo,omitempty"`
	Migrated   int                `json:"migrated,omitempty"`
	Hint       string             `json:"hint,omitempty"`
	Error      *todoErrorEnvelope `json:"error,omitempty"`
}

type todoErrorEnvelope struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func addTodoTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name: todoToolList,
		Description: "Listet die Todos des Projekts aus k-playbook-local/data/todos.json. " +
			"Ohne includeDone nur die offenen. Hülle über `k-playbook todo list`.",
	}, todoListTool)
	mcp.AddTool(server, &mcp.Tool{
		Name: todoToolAdd,
		Description: "Legt ein Todo an und vergibt die nächste Kennung. " +
			"Hülle über `k-playbook todo add`.",
	}, todoAddTool)
	mcp.AddTool(server, &mcp.Tool{
		Name: todoToolUpdate,
		Description: "Ändert Text oder Erledigt-Zustand eines Todos. Abhaken behält den Eintrag. " +
			"Hülle über `k-playbook todo update`.",
	}, todoUpdateTool)
	mcp.AddTool(server, &mcp.Tool{
		Name: todoToolDelete,
		Description: "Entfernt ein Todo endgültig — der Fall für Fehleingaben, nicht für Erledigtes. " +
			"Hülle über `k-playbook todo delete`.",
	}, todoDeleteTool)
}

func todoListTool(ctx context.Context, req *mcp.CallToolRequest, input todoListInput) (*mcp.CallToolResult, any, error) {
	return wrapTodoTool(todoToolList, input.ProjectDir, func(projectDir string) *mcp.CallToolResult {
		todos, notice, err := listTodosFor(projectDir, input.IncludeDone)
		if err != nil {
			return todoErrorResult(todoToolList, projectDir, "read_failed", err)
		}
		return todoResult(todoEnvelope{
			OK: true, Tool: todoToolList, ProjectDir: projectDir,
			Todos: todos, Migrated: notice.Migrated, Hint: notice.Hint,
		}, false)
	}), nil, nil
}

func listTodosFor(projectDir string, includeDone bool) ([]project.Todo, project.TodoNotice, error) {
	if includeDone {
		return project.ListAllTodos(projectDir)
	}
	return project.ListTodos(projectDir)
}

func todoAddTool(ctx context.Context, req *mcp.CallToolRequest, input todoAddInput) (*mcp.CallToolResult, any, error) {
	return wrapTodoTool(todoToolAdd, input.ProjectDir, func(projectDir string) *mcp.CallToolResult {
		todo, notice, err := project.AddTodo(projectDir, input.Text, input.Origin)
		if err != nil {
			return todoErrorResult(todoToolAdd, projectDir, "invalid_input", err)
		}
		return todoResult(todoEnvelope{
			OK: true, Tool: todoToolAdd, ProjectDir: projectDir,
			Todo: &todo, Migrated: notice.Migrated, Hint: notice.Hint,
		}, false)
	}), nil, nil
}

func todoUpdateTool(ctx context.Context, req *mcp.CallToolRequest, input todoUpdateInput) (*mcp.CallToolResult, any, error) {
	return wrapTodoTool(todoToolUpdate, input.ProjectDir, func(projectDir string) *mcp.CallToolResult {
		if input.Text == nil && input.Done == nil {
			return todoErrorResult(todoToolUpdate, projectDir, "invalid_input",
				fmt.Errorf("nichts zu ändern: erwartet wird text oder done"))
		}
		todo, notice, err := project.UpdateTodo(projectDir, input.ID, input.Text, input.Done)
		if err != nil {
			return todoErrorResult(todoToolUpdate, projectDir, "invalid_input", err)
		}
		return todoResult(todoEnvelope{
			OK: true, Tool: todoToolUpdate, ProjectDir: projectDir,
			Todo: &todo, Migrated: notice.Migrated, Hint: notice.Hint,
		}, false)
	}), nil, nil
}

func todoDeleteTool(ctx context.Context, req *mcp.CallToolRequest, input todoDeleteInput) (*mcp.CallToolResult, any, error) {
	return wrapTodoTool(todoToolDelete, input.ProjectDir, func(projectDir string) *mcp.CallToolResult {
		todo, notice, err := project.DeleteTodo(projectDir, input.ID)
		if err != nil {
			return todoErrorResult(todoToolDelete, projectDir, "invalid_input", err)
		}
		return todoResult(todoEnvelope{
			OK: true, Tool: todoToolDelete, ProjectDir: projectDir,
			Todo: &todo, Migrated: notice.Migrated, Hint: notice.Hint,
		}, false)
	}), nil, nil
}

// wrapTodoTool löst das Projekt auf und übergibt dessen Hauptverzeichnis.
// Aufgelöst wird über resolveProjectDir, gemeinsam mit den übrigen
// Werkzeugfamilien.
func wrapTodoTool(tool string, inputDir string, fn func(projectDir string) *mcp.CallToolResult) *mcp.CallToolResult {
	projectDir, err := resolveProjectDir(inputDir)
	if err != nil {
		return todoErrorResult(tool, projectDir, "project_not_found", err)
	}
	return fn(projectDir)
}

func todoErrorResult(tool string, projectDir string, code string, err error) *mcp.CallToolResult {
	return todoResult(todoEnvelope{
		OK:         false,
		Tool:       tool,
		ProjectDir: projectDir,
		Error:      &todoErrorEnvelope{Code: code, Message: err.Error()},
	}, true)
}

func todoResult(envelope todoEnvelope, toolError bool) *mcp.CallToolResult {
	return jsonToolResult(envelope, toolError)
}
