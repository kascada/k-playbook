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
// können, ohne Fließtext zu deuten.
type todoListOutput struct {
	Todos []project.Todo `json:"todos"`
}

// todoEntryOutput ist die Antwort von `todo add`, `todo update` und
// `todo delete`: der betroffene Eintrag.
type todoEntryOutput struct {
	Todo project.Todo `json:"todo"`
}

// runTodo führt die Todo-Verwaltung aus.
//
// Ohne Unterbefehl und mit --help gibt es nur die Kurzhilfe: dieser Aufruf
// fasst ausdrücklich keine Daten an, damit er als Rauchtest nach einem
// Release folgenlos bleibt.
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
	switch {
	case strings.TrimSpace(search) != "":
		var found []project.Todo
		found, err = project.SearchTodos(ctx, projectDir, search)
		todos = []project.Todo{}
		for _, todo := range found {
			if all || (todo.Done != "") == done {
				todos = append(todos, todo)
			}
		}
	case all:
		todos, err = project.ListAllTodos(ctx, projectDir)
	case done:
		todos, err = project.ListDoneTodos(ctx, projectDir)
	default:
		todos, err = project.ListTodos(ctx, projectDir)
	}
	if err != nil {
		return docError(err)
	}

	return printTodoJSON(todoListOutput{Todos: todos})
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

	todo, err := project.AddTodo(context.Background(), projectDir, text, origin)
	if err != nil {
		return docError(err)
	}
	return printTodoJSON(todoEntryOutput{Todo: todo})
}

func runTodoUpdate(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("erwartet wird todo update <id> [--text <text>] [--origin <herkunft>] [--done] [--reopen]")
	}

	id, err := parseTodoID(args[0])
	if err != nil {
		return err
	}

	var text *string
	var done *bool
	var origin *string
	for index := 1; index < len(args); index++ {
		switch args[index] {
		case "--text":
			if index+1 >= len(args) {
				return fmt.Errorf("--text erwartet einen Wert")
			}
			index++
			value := args[index]
			text = &value
		case "--origin":
			if index+1 >= len(args) {
				return fmt.Errorf("--origin erwartet einen Wert (ein leerer Wert \"\" entfernt die Herkunft)")
			}
			index++
			value := args[index]
			origin = &value
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
			return fmt.Errorf("unbekanntes Argument: %s — erwartet wird todo update <id> [--text <text>] [--origin <herkunft>] [--done] [--reopen]", args[index])
		}
	}
	if text == nil && done == nil && origin == nil {
		return fmt.Errorf("nichts zu ändern: erwartet wird mindestens --text, --origin, --done oder --reopen")
	}

	projectDir, err := todoProjectDir()
	if err != nil {
		return err
	}

	todo, err := project.UpdateTodo(context.Background(), projectDir, id, text, done, origin)
	if err != nil {
		return docError(err)
	}
	return printTodoJSON(todoEntryOutput{Todo: todo})
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

	todo, err := project.DeleteTodo(context.Background(), projectDir, id)
	if err != nil {
		return docError(err)
	}
	return printTodoJSON(todoEntryOutput{Todo: todo})
}

// todoProjectDir löst das Projekt auf. Wie bei inventory wird nur der Anker
// gebraucht: Die Anbindung an die Collection steht in der Projektkonfiguration,
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
  update <id> [--text <text>] [--origin <herkunft>] [--done] [--reopen]
            Ändert den Text, setzt oder ändert die Herkunft (--origin; ein
            leerer Wert --origin "" entfernt sie), hakt ab (--done) oder
            öffnet wieder (--reopen). Was nicht angegeben ist, bleibt.
  delete <id>
            Entfernt einen Eintrag.

Ohne Unterbefehl und mit --help steht hier nur diese Übersicht: der Aufruf
liest und schreibt dabei nichts.
`)
}
