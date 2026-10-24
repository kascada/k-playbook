package mcpserver

import (
	"context"
	"fmt"
	"strings"

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

// todoBaseInput trägt projectDir. Im Vertrag ist das Feld Pflicht, im Schema
// optional (omitempty): sonst weist die Schemaprüfung des SDK einen Aufruf ohne
// projectDir ab, bevor die Hülle läuft, und der Aufrufer bekommt nur deren
// nackten Text statt des Umschlags. Geprüft wird in checkProjectDir.
type todoBaseInput struct {
	ProjectDir string `json:"projectDir,omitempty" jsonschema:"Pflicht. Verzeichnis, ab dem aufwärts nach K-PLAYBOOK.yaml gesucht wird. Relative Pfade werden relativ zum Arbeitsverzeichnis des MCP-Servers aufgelöst."`
}

type todoListInput struct {
	todoBaseInput
	IncludeDone bool   `json:"includeDone,omitempty" jsonschema:"Nimmt die erledigten Einträge mit auf. Ohne Angabe kommen nur die offenen."`
	Search      string `json:"search,omitempty" jsonschema:"Nur Einträge, deren Text diesen Suchtext enthält, ohne Rücksicht auf Groß- und Kleinschreibung. Gilt für die Liste, die includeDone wählt."`
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
// hint meldet ein noch nicht übernommenes data/todos.json oder eine
// zurückgebliebene TODO.md. Das ist kein Fehler — eine Liste kommt trotzdem;
// hint steht auch neben einem Fehler, etwa neben todos_not_migrated.
type todoEnvelope struct {
	OK         bool               `json:"ok"`
	Tool       string             `json:"tool"`
	ProjectDir string             `json:"projectDir"`
	Todos      []project.Todo     `json:"todos,omitempty"`
	Todo       *project.Todo      `json:"todo,omitempty"`
	Hint       string             `json:"hint,omitempty"`
	Error      *todoErrorEnvelope `json:"error,omitempty"`
}

type todoErrorEnvelope struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// todoStateHelp ist der gemeinsame Teil der Beschreibungen: Ablage und
// Zustände.
const todoStateHelp = " Die Todos liegen als Dokumente in der Kephalaion-Collection des Projekts (todos/, erledigt " +
	"todos/done/). kephalaion_unavailable heißt: Kephalaion ist nicht erreichbar, es gibt dann keine Todos und " +
	"keinen Rückfall auf die Platte. hint meldet ein noch nicht übernommenes data/todos.json (dann " +
	"`k-playbook todo migrate`) oder eine zurückgebliebene TODO.md, die nicht übernommen wird."

// todoWriteHelp ergänzt die Beschreibungen der schreibenden Werkzeuge.
const todoWriteHelp = " Solange data/todos.json besteht, ist das abgelehnt: todos_not_migrated. outcome_unknown: " +
	"Der Vorgang kann angekommen sein — nicht wiederholen, sondern mit k_playbook_todo_list (includeDone) nachsehen."

func addTodoTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name: todoToolList,
		Description: "Listet die Todos des Projekts, nach Kennung. Ohne includeDone nur die offenen; search filtert " +
			"über den Text." + todoStateHelp + " Hülle über `k-playbook todo list`.",
	}, todoListTool)
	mcp.AddTool(server, &mcp.Tool{
		Name: todoToolAdd,
		Description: "Legt ein Todo an und vergibt die nächste Kennung: höchste Nummer unter todos/ samt done/ plus " +
			"eins. Nach dem Löschen des Todos mit der höchsten Nummer wird diese wieder vergeben." + todoStateHelp +
			todoWriteHelp + " Hülle über `k-playbook todo add`.",
	}, todoAddTool)
	mcp.AddTool(server, &mcp.Tool{
		Name: todoToolUpdate,
		Description: "Ändert Text oder Erledigt-Zustand eines Todos. Abhaken behält den Eintrag und verschiebt ihn " +
			"nach todos/done/; stale_revision: Das Todo hat sich seit dem Lesen geändert, neu listen und noch einmal." +
			todoStateHelp + todoWriteHelp + " Hülle über `k-playbook todo update`.",
	}, todoUpdateTool)
	mcp.AddTool(server, &mcp.Tool{
		Name: todoToolDelete,
		Description: "Entfernt ein Todo endgültig — der Fall für Fehleingaben, nicht für Erledigtes." + todoStateHelp +
			todoWriteHelp + " Hülle über `k-playbook todo delete`.",
	}, todoDeleteTool)
}

func todoListTool(ctx context.Context, req *mcp.CallToolRequest, input todoListInput) (*mcp.CallToolResult, any, error) {
	return wrapTodoTool(todoToolList, input.ProjectDir, func(projectDir string) *mcp.CallToolResult {
		todos, notice, err := listTodosFor(ctx, projectDir, input.IncludeDone, input.Search)
		if err != nil {
			return todoErrorResult(todoToolList, projectDir, todoErrorCode(err, "read_failed"), err, notice)
		}
		return todoResult(todoEnvelope{
			OK: true, Tool: todoToolList, ProjectDir: projectDir,
			Todos: todos, Hint: notice.Hint,
		}, false)
	}), nil, nil
}

// listTodosFor wählt die Liste: offene oder alle, mit search nur die
// Treffer daraus.
func listTodosFor(ctx context.Context, projectDir string, includeDone bool, search string) ([]project.Todo, project.TodoNotice, error) {
	if strings.TrimSpace(search) != "" {
		found, notice, err := project.SearchTodos(ctx, projectDir, search)
		if err != nil || includeDone {
			return found, notice, err
		}
		open := []project.Todo{}
		for _, todo := range found {
			if todo.Done == "" {
				open = append(open, todo)
			}
		}
		return open, notice, nil
	}
	if includeDone {
		return project.ListAllTodos(ctx, projectDir)
	}
	return project.ListTodos(ctx, projectDir)
}

// todoErrorCode ist der Code eines Fehlers der Fachlogik
// (kephalaion_unavailable, todos_not_migrated, not_found, …); ohne Code gilt
// fallback.
func todoErrorCode(err error, fallback string) string {
	if code := project.DocRefErrorCode(err); code != "" {
		return code
	}
	return fallback
}

func todoAddTool(ctx context.Context, req *mcp.CallToolRequest, input todoAddInput) (*mcp.CallToolResult, any, error) {
	return wrapTodoTool(todoToolAdd, input.ProjectDir, func(projectDir string) *mcp.CallToolResult {
		todo, notice, err := project.AddTodo(ctx, projectDir, input.Text, input.Origin)
		if err != nil {
			return todoErrorResult(todoToolAdd, projectDir, todoErrorCode(err, "failed"), err, notice)
		}
		return todoResult(todoEnvelope{
			OK: true, Tool: todoToolAdd, ProjectDir: projectDir,
			Todo: &todo, Hint: notice.Hint,
		}, false)
	}), nil, nil
}

func todoUpdateTool(ctx context.Context, req *mcp.CallToolRequest, input todoUpdateInput) (*mcp.CallToolResult, any, error) {
	return wrapTodoTool(todoToolUpdate, input.ProjectDir, func(projectDir string) *mcp.CallToolResult {
		if input.Text == nil && input.Done == nil {
			return todoErrorResult(todoToolUpdate, projectDir, "invalid_input",
				fmt.Errorf("nichts zu ändern: erwartet wird text oder done"), project.TodoNotice{})
		}
		todo, notice, err := project.UpdateTodo(ctx, projectDir, input.ID, input.Text, input.Done)
		if err != nil {
			return todoErrorResult(todoToolUpdate, projectDir, todoErrorCode(err, "failed"), err, notice)
		}
		return todoResult(todoEnvelope{
			OK: true, Tool: todoToolUpdate, ProjectDir: projectDir,
			Todo: &todo, Hint: notice.Hint,
		}, false)
	}), nil, nil
}

func todoDeleteTool(ctx context.Context, req *mcp.CallToolRequest, input todoDeleteInput) (*mcp.CallToolResult, any, error) {
	return wrapTodoTool(todoToolDelete, input.ProjectDir, func(projectDir string) *mcp.CallToolResult {
		todo, notice, err := project.DeleteTodo(ctx, projectDir, input.ID)
		if err != nil {
			return todoErrorResult(todoToolDelete, projectDir, todoErrorCode(err, "failed"), err, notice)
		}
		return todoResult(todoEnvelope{
			OK: true, Tool: todoToolDelete, ProjectDir: projectDir,
			Todo: &todo, Hint: notice.Hint,
		}, false)
	}), nil, nil
}

// wrapTodoTool löst das Projekt auf und übergibt dessen Hauptverzeichnis.
// Aufgelöst wird über resolveProjectDir, gemeinsam mit den übrigen
// Werkzeugfamilien.
func wrapTodoTool(tool string, inputDir string, fn func(projectDir string) *mcp.CallToolResult) *mcp.CallToolResult {
	projectDir, err := resolveProjectDir(inputDir)
	if err != nil {
		return todoErrorResult(tool, projectDir, projectDirErrorCode(err), err, project.TodoNotice{})
	}
	return fn(projectDir)
}

func todoErrorResult(tool string, projectDir string, code string, err error, notice project.TodoNotice) *mcp.CallToolResult {
	return todoResult(todoEnvelope{
		OK:         false,
		Tool:       tool,
		ProjectDir: projectDir,
		Hint:       notice.Hint,
		Error:      &todoErrorEnvelope{Code: code, Message: err.Error()},
	}, true)
}

func todoResult(envelope todoEnvelope, toolError bool) *mcp.CallToolResult {
	return jsonToolResult(envelope, toolError)
}
