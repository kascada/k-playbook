package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// todoListOutput ist die Antwort von `todo list`.
//
// Die Ausgabe ist JSON auf stdout wie bei `context`: /k-todo soll sie lesen
// können, ohne Fließtext zu deuten. hint steht daneben, weil ein nicht
// übernommenes data/todos.json oder eine zurückgebliebene TODO.md ein
// Hinweis ist und kein Fehler.
type todoListOutput struct {
	Todos []project.Todo `json:"todos"`
	Hint  string         `json:"hint,omitempty"`
}

// todoEntryOutput ist die Antwort von `todo add`, `todo update` und
// `todo delete`: der betroffene Eintrag.
type todoEntryOutput struct {
	Todo project.Todo `json:"todo"`
	Hint string       `json:"hint,omitempty"`
}

// runTodo führt die Todo-Verwaltung aus.
//
// Ohne Unterbefehl und mit --help gibt es nur die Kurzhilfe: dieser Aufruf
// fasst ausdrücklich keine Daten an — weder die Collection noch
// data/todos.json —, damit er als Rauchtest nach einem Release folgenlos
// bleibt.
func runTodo(args []string) error {
	if len(args) == 0 {
		printTodoUsage(os.Stdout)
		return nil
	}

	switch args[0] {
	case "help", "-h", "--help":
		printTodoUsage(os.Stdout)
		return nil
	case "list":
		return runTodoList(args[1:])
	case "add":
		return runTodoAdd(args[1:])
	case "update":
		return runTodoUpdate(args[1:])
	case "delete":
		return runTodoDelete(args[1:])
	case "migrate":
		return runTodoMigrate(args[1:])
	default:
		printTodoUsage(os.Stderr)
		return fmt.Errorf("unbekannter Unterbefehl: todo %s", args[0])
	}
}

func runTodoList(args []string) error {
	done := false
	all := false
	search := ""
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--done":
			done = true
		case "--all":
			all = true
		case "--search":
			if index+1 >= len(args) {
				return fmt.Errorf("--search erwartet einen Wert")
			}
			index++
			search = args[index]
		default:
			return fmt.Errorf("unbekanntes Argument: %s — erwartet wird todo list [--done] [--all] [--search <text>]", args[index])
		}
	}

	projectDir, err := todoProjectDir()
	if err != nil {
		return err
	}

	ctx := context.Background()
	var todos []project.Todo
	var notice project.TodoNotice
	switch {
	case strings.TrimSpace(search) != "":
		var found []project.Todo
		found, notice, err = project.SearchTodos(ctx, projectDir, search)
		todos = []project.Todo{}
		for _, todo := range found {
			if all || (todo.Done != "") == done {
				todos = append(todos, todo)
			}
		}
	case all:
		todos, notice, err = project.ListAllTodos(ctx, projectDir)
	case done:
		todos, notice, err = project.ListDoneTodos(ctx, projectDir)
	default:
		todos, notice, err = project.ListTodos(ctx, projectDir)
	}
	if err != nil {
		return todoError(err, notice)
	}

	return printTodoJSON(todoListOutput{Todos: todos, Hint: notice.Hint})
}

// todoError meldet einen Fehler der Fachlogik mit seinem Code; ein Hinweis
// daneben geht nach stderr, stdout bleibt leer.
func todoError(err error, notice project.TodoNotice) error {
	if notice.Hint != "" {
		fmt.Fprintf(os.Stderr, "Hinweis: %s\n", notice.Hint)
	}
	return docError(err)
}

func runTodoAdd(args []string) error {
	origin := ""
	parts := []string{}
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--origin":
			if index+1 >= len(args) {
				return fmt.Errorf("--origin erwartet einen Wert")
			}
			index++
			origin = args[index]
		default:
			parts = append(parts, args[index])
		}
	}

	text := strings.TrimSpace(strings.Join(parts, " "))
	if text == "" {
		return fmt.Errorf("leerer Text: erwartet wird todo add <text> [--origin <herkunft>]")
	}

	projectDir, err := todoProjectDir()
	if err != nil {
		return err
	}

	todo, notice, err := project.AddTodo(context.Background(), projectDir, text, origin)
	if err != nil {
		return todoError(err, notice)
	}
	return printTodoJSON(todoEntryOutput{Todo: todo, Hint: notice.Hint})
}

func runTodoUpdate(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("erwartet wird todo update <id> [--text <text>] [--done] [--reopen]")
	}

	id, err := parseTodoID(args[0])
	if err != nil {
		return err
	}

	var text *string
	var done *bool
	for index := 1; index < len(args); index++ {
		switch args[index] {
		case "--text":
			if index+1 >= len(args) {
				return fmt.Errorf("--text erwartet einen Wert")
			}
			index++
			value := args[index]
			text = &value
		case "--done":
			if done != nil && !*done {
				return fmt.Errorf("--done und --reopen schließen einander aus")
			}
			value := true
			done = &value
		case "--reopen":
			if done != nil && *done {
				return fmt.Errorf("--done und --reopen schließen einander aus")
			}
			value := false
			done = &value
		default:
			return fmt.Errorf("unbekanntes Argument: %s — erwartet wird todo update <id> [--text <text>] [--done] [--reopen]", args[index])
		}
	}
	if text == nil && done == nil {
		return fmt.Errorf("nichts zu ändern: erwartet wird mindestens --text, --done oder --reopen")
	}

	projectDir, err := todoProjectDir()
	if err != nil {
		return err
	}

	todo, notice, err := project.UpdateTodo(context.Background(), projectDir, id, text, done)
	if err != nil {
		return todoError(err, notice)
	}
	return printTodoJSON(todoEntryOutput{Todo: todo, Hint: notice.Hint})
}

func runTodoDelete(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("erwartet wird todo delete <id>")
	}

	id, err := parseTodoID(args[0])
	if err != nil {
		return err
	}

	projectDir, err := todoProjectDir()
	if err != nil {
		return err
	}

	todo, notice, err := project.DeleteTodo(context.Background(), projectDir, id)
	if err != nil {
		return todoError(err, notice)
	}
	return printTodoJSON(todoEntryOutput{Todo: todo, Hint: notice.Hint})
}

// runTodoMigrate übernimmt data/todos.json in die Collection. Die Ausgabe
// nennt angelegte und übersprungene Einträge — auch bei einem Abbruch, dann
// mit dem Fehler auf stderr.
func runTodoMigrate(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("erwartet wird todo migrate ohne Argumente")
	}
	projectDir, err := todoProjectDir()
	if err != nil {
		return err
	}
	result, err := project.MigrateTodos(context.Background(), projectDir)
	if printErr := printTodoJSON(result); printErr != nil && err == nil {
		err = printErr
	}
	if err != nil {
		return docError(err)
	}
	return nil
}

// todoProjectDir löst das Projekt auf. Wie bei inventory wird nur der Anker
// gebraucht: die Anbindung an die Collection steht unter k-playbook-local/,
// nicht im Katalog der Installation.
func todoProjectDir() (string, error) {
	environment := project.Detect()
	if !environment.Installed {
		return "", fmt.Errorf("keine %s gefunden — gesucht ab %s aufwärts",
			project.ConfigFileName, project.DisplayPath(environment.SearchedFrom))
	}
	return environment.ProjectDir, nil
}

func parseTodoID(value string) (int, error) {
	id, err := strconv.Atoi(strings.TrimPrefix(strings.TrimSpace(value), "#"))
	if err != nil || id < 1 {
		return 0, fmt.Errorf("keine gültige Kennung: %s", value)
	}
	return id, nil
}

func printTodoJSON(payload any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(payload)
}

func printTodoUsage(out io.Writer) {
	fmt.Fprint(out, `k-playbook todo — Todos des Projekts

Die Todos liegen als je ein Dokument in der Kephalaion-Collection des
Projekts: offen unter todos/, abgehakt unter todos/done/. Geschrieben werden
sie über diesen Weg, über die MCP-Werkzeuge k_playbook_todo_* oder über die
Oberfläche. Ohne erreichbares Kephalaion gibt es keine Todos
(kephalaion_unavailable). Jede Ausgabe ist JSON auf stdout, Fehler stehen mit
ihrem Code auf stderr.

Unterbefehle:
  list [--done] [--all] [--search <text>]
            Listet die offenen Einträge. --done nur die erledigten, --all beide;
            --search nur die, deren Text <text> enthält (Groß- und
            Kleinschreibung egal).
  add <text> [--origin <herkunft>]
            Legt einen Eintrag an und vergibt die nächste Kennung. Nach dem
            Löschen des Eintrags mit der höchsten Kennung wird diese wieder
            vergeben.
  update <id> [--text <text>] [--done] [--reopen]
            Ändert den Text, hakt ab (--done) oder öffnet wieder (--reopen).
  delete <id>
            Entfernt einen Eintrag.
  migrate   Übernimmt k-playbook-local/data/todos.json in die Collection und
            benennt die Datei danach um. Solange sie besteht, sind add, update
            und delete gesperrt (todos_not_migrated).

Ohne Unterbefehl und mit --help steht hier nur diese Übersicht: der Aufruf
liest und schreibt dabei nichts.
`)
}
