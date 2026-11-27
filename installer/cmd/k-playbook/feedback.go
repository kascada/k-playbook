package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// runFeedback führt `k-playbook feedback` aus (Task 198), das Gegenstück der
// MCP-Werkzeuge k_playbook_feedback_*, mit derselben Fachlogik. Anders als
// über MCP verlangt create --von ki|mensch: Die Kommandozeile ruft auch eine
// KI über die Shell auf. Ohne Unterbefehl und mit --help gibt es nur die
// Kurzhilfe, ohne Daten anzufassen.
func runFeedback(args []string) error {
	if len(args) == 0 {
		printFeedbackUsage(os.Stdout)
		return nil
	}
	switch args[0] {
	case "help", "-h", "--help":
		printFeedbackUsage(os.Stdout)
		return nil
	case "create":
		return runFeedbackCreate(args[1:], os.Stdin)
	case "list":
		return runFeedbackList(args[1:])
	case "update":
		return runFeedbackUpdate(args[1:])
	default:
		printFeedbackUsage(os.Stderr)
		return fmt.Errorf("unbekannter Unterbefehl: feedback %s", args[0])
	}
}

// feedbackFlagValue liest den Wert hinter einem Flag.
func feedbackFlagValue(args []string, index *int) (string, error) {
	flag := args[*index]
	if *index+1 >= len(args) {
		return "", fmt.Errorf("%s erwartet einen Wert", flag)
	}
	*index++
	return args[*index], nil
}

const feedbackCreateCall = "feedback create --title <titel> --art fehler|wunsch --dringlichkeit hoch|mittel|niedrig " +
	"--betrifft k-playbook|kephalaion|projekt --von ki|mensch [--client <client>] [--modell <modell>] [--sitzung <id>] (Text von stdin)"

func runFeedbackCreate(args []string, stdin io.Reader) error {
	opts := project.FeedbackCreateOptions{}
	targets := map[string]*string{
		"--title": &opts.Title, "--art": &opts.Art, "--dringlichkeit": &opts.Dringlichkeit, "--betrifft": &opts.Betrifft,
		"--von": &opts.Von, "--client": &opts.Client, "--modell": &opts.Modell, "--sitzung": &opts.Sitzung,
	}
	for index := 0; index < len(args); index++ {
		if args[index] == "-h" || args[index] == "--help" {
			printFeedbackUsage(os.Stdout)
			return nil
		}
		target, ok := targets[args[index]]
		if !ok {
			return fmt.Errorf("unbekanntes Argument: %s — erwartet wird %s", args[index], feedbackCreateCall)
		}
		value, err := feedbackFlagValue(args, &index)
		if err != nil {
			return err
		}
		*target = value
	}
	if strings.TrimSpace(opts.Von) == "" {
		return fmt.Errorf("--von ki|mensch ist Pflicht: %s", feedbackCreateCall)
	}
	body, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("Text von stdin lesen: %w", err)
	}
	opts.Body = string(body)

	projectDir, err := todoProjectDir()
	if err != nil {
		return err
	}
	created, err := project.CreateFeedback(context.Background(), projectDir, opts)
	if err != nil {
		return docError(err)
	}
	return printTodoJSON(created)
}

func runFeedbackList(args []string) error {
	opts := project.FeedbackListOptions{}
	for index := 0; index < len(args); index++ {
		var err error
		switch args[index] {
		case "--all":
			opts.IncludeDone = true
		case "--all-collections":
			opts.AllCollections = true
		case "--art":
			opts.Art, err = feedbackFlagValue(args, &index)
		case "--betrifft":
			opts.Betrifft, err = feedbackFlagValue(args, &index)
		case "--status":
			opts.Status, err = feedbackFlagValue(args, &index)
		default:
			return fmt.Errorf("unbekanntes Argument: %s — erwartet wird feedback list [--all] [--art <art>] [--betrifft <betreff>] [--status <status>] [--all-collections]", args[index])
		}
		if err != nil {
			return err
		}
	}
	projectDir, err := todoProjectDir()
	if err != nil {
		return err
	}
	listing, err := project.ListFeedback(context.Background(), projectDir, opts)
	if err != nil {
		return docError(err)
	}
	return printTodoJSON(listing)
}

const feedbackUpdateCall = "feedback update <keph:-verweis|nummer> --status offen|angenommen|erledigt|abgelehnt [--since <revision>]"

func runFeedbackUpdate(args []string) error {
	target, status := "", ""
	var since int64
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--status":
			value, err := feedbackFlagValue(args, &index)
			if err != nil {
				return err
			}
			status = value
		case "--since":
			value, err := feedbackFlagValue(args, &index)
			if err != nil {
				return err
			}
			since, err = strconv.ParseInt(value, 10, 64)
			if err != nil || since < 1 {
				return fmt.Errorf("--since erwartet eine Revision, bekommen %q", value)
			}
		default:
			if strings.HasPrefix(args[index], "--") || target != "" {
				return fmt.Errorf("unbekanntes Argument: %s — erwartet wird %s", args[index], feedbackUpdateCall)
			}
			target = args[index]
		}
	}
	if target == "" || status == "" {
		return fmt.Errorf("erwartet wird %s", feedbackUpdateCall)
	}
	projectDir, err := todoProjectDir()
	if err != nil {
		return err
	}
	updated, err := project.UpdateFeedback(context.Background(), projectDir, target, status, since)
	if err != nil {
		if change, ok := project.DocForeignChangeOf(err); ok {
			return fmt.Errorf("%w (%s; revision %d)", err, project.DocCodeForeignChange, change.Revision)
		}
		return docError(err)
	}
	return printTodoJSON(updated)
}

func printFeedbackUsage(out io.Writer) {
	fmt.Fprint(out, `k-playbook feedback — Meldungen des Projekts

Eine Meldung ist ein Fehler oder Wunsch, den jemand anderes erledigen soll —
an k-playbook, an Kephalaion oder am Projekt selbst. Sie liegt als Dokument in
der Kephalaion-Collection des Projekts: offen unter feedback/, erledigt und
abgelehnt unter feedback/done/. Gelesen und geschrieben wird sie nur über
diesen Weg, die MCP-Werkzeuge k_playbook_feedback_* oder /k-feedback. Ohne
erreichbares Kephalaion gibt es keine Meldungen (kephalaion_unavailable).
Jede Ausgabe ist JSON auf stdout, Fehler stehen mit ihrem Code auf stderr.

Unterbefehle:
  create --title <titel> --art fehler|wunsch --dringlichkeit hoch|mittel|niedrig
         --betrifft k-playbook|kephalaion|projekt --von ki|mensch
         [--client <client>] [--modell <modell>] [--sitzung <id>]   (Text von stdin)
            Legt eine Meldung mit der nächsten Nummer an. --von ist Pflicht.
            Bei --von ki sind --client, --modell und --sitzung Pflicht (die
            Sitzung aus der Zeile Sitzung: des Briefings); bei --von mensch
            stehen --client und --modell nicht, --sitzung ist wahlweise.
            User, Account, Version und status offen setzt das Programm.
  list [--all] [--art <art>] [--betrifft <betreff>] [--status <status>] [--all-collections]
            Listet die Meldungen der eigenen Collection mit Kopf und Text
            (items); --all nimmt feedback/done/ mit. --all-collections zählt
            dazu in jeder anderen Collection aller Hubs, die der Account des
            jeweiligen Hubs lesen darf (ohne die eigene und die öffentliche
            k-playbook), die offenen Meldungen nach betrifft (collections);
            eine Collection, die beim Lesen scheitert, steht mit ihrem
            Fehler da.
  update <keph:-verweis|nummer> --status offen|angenommen|erledigt|abgelehnt [--since <revision>]
            Setzt den Status. erledigt und abgelehnt verschieben nach
            feedback/done/, offen und angenommen holen zurück. Mit --since
            wird nichts geschrieben, wenn die Meldung inzwischen eine andere
            Revision hat (foreign_change).

Ohne Unterbefehl und mit --help steht hier nur diese Übersicht: der Aufruf
liest und schreibt dabei nichts.
`)
}
