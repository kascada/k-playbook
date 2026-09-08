package main

import (
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
// können, ohne Fließtext zu deuten. migrated und hint stehen daneben, weil ein
// Zugriff nebenbei migrieren kann und weil eine zurückgebliebene TODO.md ein
// Hinweis ist und kein Fehler.
type todoListOutput struct {
	Todos    []project.Todo `json:"todos"`
	Migrated int            `json:"migrated,omitempty"`
	Hint     string         `json:"hint,omitempty"`
}

// todoEntryOutput ist die Antwort von `todo add`, `todo update` und
// `todo delete`: der betroffene Eintrag.
type todoEntryOutput struct {
	Todo     project.Todo `json:"todo"`
	Migrated int          `json:"migrated,omitempty"`
	Hint     string       `json:"hint,omitempty"`
}

// todoImportOutput ist die Antwort von `todo import`.
type todoImportOutput struct {
	Imported int    `json:"imported"`
	Migrated int    `json:"migrated,omitempty"`
	Hint     string `json:"hint,omitempty"`
}

// runTodo führt die Todo-Verwaltung aus.
//
// Ohne Unterbefehl und mit --help gibt es nur die Kurzhilfe: dieser Aufruf
// fasst ausdrücklich keine Daten an — weder data/todos.json noch eine
// TODO.md —, damit er als Rauchtest nach einem Release folgenlos bleibt und
// die Migration nicht anläuft.
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
	case "import":
		return runTodoImport(args[1:])
	default:
		printTodoUsage(os.Stderr)
		return fmt.Errorf("unbekannter Unterbefehl: todo %s", args[0])
	}
}

func runTodoList(args []string) error {
	done := false
	all := false
	for _, arg := range args {
		switch arg {
		case "--done":
			done = true
		case "--all":
			all = true
		default:
			return fmt.Errorf("unbekanntes Argument: %s — erwartet wird todo list [--done] [--all]", arg)
		}
	}

	projectDir, err := todoProjectDir()
	if err != nil {
		return err
	}

	var todos []project.Todo
	var notice project.TodoNotice
	switch {
	case all:
		todos, notice, err = project.ListAllTodos(projectDir)
	case done:
		todos, notice, err = project.ListDoneTodos(projectDir)
	default:
		todos, notice, err = project.ListTodos(projectDir)
	}
	if err != nil {
		return err
	}

	return printTodoJSON(todoListOutput{Todos: todos, Migrated: notice.Migrated, Hint: notice.Hint})
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

	todo, notice, err := project.AddTodo(projectDir, text, origin)
	if err != nil {
		return err
	}
	return printTodoJSON(todoEntryOutput{Todo: todo, Migrated: notice.Migrated, Hint: notice.Hint})
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

	todo, notice, err := project.UpdateTodo(projectDir, id, text, done)
	if err != nil {
		return err
	}
	return printTodoJSON(todoEntryOutput{Todo: todo, Migrated: notice.Migrated, Hint: notice.Hint})
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

	todo, notice, err := project.DeleteTodo(projectDir, id)
	if err != nil {
		return err
	}
	return printTodoJSON(todoEntryOutput{Todo: todo, Migrated: notice.Migrated, Hint: notice.Hint})
}

func runTodoImport(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("erwartet wird todo import <pfad>")
	}

	projectDir, err := todoProjectDir()
	if err != nil {
		return err
	}

	count, notice, err := project.ImportTodos(projectDir, args[0])
	if err != nil {
		return err
	}
	return printTodoJSON(todoImportOutput{Imported: count, Migrated: notice.Migrated, Hint: notice.Hint})
}

// todoProjectDir löst das Projekt auf. Wie bei inventory wird nur der Anker
// gebraucht: die Todos liegen unter k-playbook-local/, nicht im Katalog der
// Installation.
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

Die Ablage ist k-playbook-local/data/todos.json; geschrieben wird sie nur über
diesen Weg, über die MCP-Werkzeuge oder über die Oberfläche. Jede Ausgabe ist
JSON auf stdout, Fehler stehen auf stderr.

Unterbefehle:
  list [--done] [--all]
            Listet die offenen Einträge. --done nur die erledigten, --all beide.
  add <text> [--origin <herkunft>]
            Legt einen Eintrag an und vergibt die nächste Kennung.
  update <id> [--text <text>] [--done] [--reopen]
            Ändert den Text, hakt ab (--done) oder öffnet wieder (--reopen).
  delete <id>
            Entfernt einen Eintrag. Die Kennung wird nie wieder vergeben.
  import <pfad>
            Hängt die Einträge einer Markdown-Checkliste an und entfernt sie.

Ohne Unterbefehl und mit --help steht hier nur diese Übersicht: der Aufruf
liest und schreibt dabei nichts.
`)
}
