package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// runTask führt `k-playbook task` aus (Task 178): zurzeit nur `create`, das
// Gegenstück des MCP-Werkzeugs k_playbook_task_create. Ohne Unterbefehl und
// mit --help gibt es nur die Kurzhilfe, ohne Daten anzufassen.
func runTask(args []string) error {
	if len(args) == 0 {
		printTaskUsage(os.Stdout)
		return nil
	}
	switch args[0] {
	case "help", "-h", "--help":
		printTaskUsage(os.Stdout)
		return nil
	case "create":
		return runTaskCreate(args[1:], os.Stdin)
	default:
		printTaskUsage(os.Stderr)
		return fmt.Errorf("unbekannter Unterbefehl: task %s", args[0])
	}
}

func runTaskCreate(args []string, stdin io.Reader) error {
	opts := project.TaskCreateOptions{}
	for index := 0; index < len(args); index++ {
		value := func(flag string) (string, error) {
			if index+1 >= len(args) {
				return "", fmt.Errorf("%s erwartet einen Wert", flag)
			}
			index++
			return args[index], nil
		}
		var err error
		switch args[index] {
		case "--title":
			opts.Title, err = value("--title")
		case "--name":
			opts.Name, err = value("--name")
		case "--dry-run":
			opts.DryRun = true
		case "--no-intent":
			opts.NoIntent = true
		case "-h", "--help":
			printTaskUsage(os.Stdout)
			return nil
		default:
			return fmt.Errorf("unbekanntes Argument: %s — erwartet wird task create --title <titel> --name <kurzname> [--dry-run] [--no-intent] (Rumpf von stdin)", args[index])
		}
		if err != nil {
			return err
		}
	}
	if strings.TrimSpace(opts.Title) == "" || strings.TrimSpace(opts.Name) == "" {
		return fmt.Errorf("--title und --name sind Pflicht: task create --title <titel> --name <kurzname> [--dry-run] [--no-intent] (Rumpf von stdin)")
	}
	body, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("Rumpf von stdin lesen: %w", err)
	}
	opts.Body = string(body)

	projectDir, err := todoProjectDir()
	if err != nil {
		return err
	}
	created, err := project.CreateTaskVia(context.Background(), projectDir, opts, openDocs)
	if err != nil {
		return docError(err)
	}
	if created.Warnings == nil {
		created.Warnings = []string{}
	}
	return printTodoJSON(created)
}

func printTaskUsage(out io.Writer) {
	fmt.Fprint(out, `k-playbook task — Tasks des Projekts

Aufruf:
  k-playbook task create --title <titel> --name <kurzname> [--dry-run] [--no-intent]   (Rumpf von stdin)

create prüft, nummeriert und legt einen Task in einem Zug in der Collection
des Projekts an (keph:/user/~/tasks/): Frontmatter mit title, die Überschrift
"# Task <NNN> — <Titel>" und den Rumpf von stdin (Markdown ohne Frontmatter und
ohne H1). Strukturverstöße kommen gesammelt als invalid_input, nichts wird
geschrieben. --dry-run liefert dasselbe Ergebnis samt gerendertem Text, legt
aber nichts an; --no-intent hebt die Pflicht zu einem Abschnitt Intent auf.
Die Ausgabe ist JSON auf stdout. Aufbau eines Tasks:
k-playbook:/instructions/on-demand/tasks.md.
`)
}
