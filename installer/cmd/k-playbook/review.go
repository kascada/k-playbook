package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/kascada/k-playbook/installer/internal/project"
	"github.com/kascada/k-playbook/installer/internal/review/merge"
	"github.com/kascada/k-playbook/installer/internal/review/workdir"
)

// reviewUsage ist die Kurzhilfe von `k-playbook review`.
const reviewUsage = `k-playbook review — der Weg eines eigenständigen Reviews (/k-review)

  k-playbook review workdir <rezept> [--dir <pfad>]
      Bestimmt das Arbeitsverzeichnis eines Reviews. Ohne --dir legt es ein
      neues Verzeichnis unmittelbar unter /tmp an ($TMPDIR gilt nicht), mit
      Projekt, Rezept, Zeitstempel und einem zufälligen Teil im Namen und den
      Rechten 0700. Mit --dir nimmt es den
      Pfad, legt ihn mit 0700 an, wenn er fehlt, und warnt, wenn er im
      Projekt liegt oder für andere zugänglich ist.

  k-playbook review merge <verzeichnis> [--check]
      Fasst die Fundlisten <eintrag>.sarif im Arbeitsverzeichnis zusammen:
      prüft jede gegen den Vertrag ihres Rezepts (Werkzeugname, audit.ruleIds,
      audit.scope.paths) und schreibt review-input.json und review-input.md
      dorthin, mit den Rechten 0600. Bekannte Entscheidungen gleicht dieser
      Weg nicht ab. Mit --check wird nur geprüft und nichts geschrieben.

Die Ausgabe ist JSON auf stdout. Einen Audit-Lauf legt keiner der beiden
Befehle an; scan und merge bleiben gesperrt.
`

// runReview verteilt die Unterbefehle von `k-playbook review`.
func runReview(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprint(os.Stderr, reviewUsage)
		return nil
	}
	switch args[0] {
	case "workdir":
		return runReviewWorkdir(os.Stdout, args[1:])
	case "merge":
		return runReviewMerge(os.Stdout, args[1:])
	default:
		fmt.Fprint(os.Stderr, reviewUsage)
		return fmt.Errorf("unbekannter Unterbefehl von review: %s", args[0])
	}
}

func runReviewWorkdir(out io.Writer, args []string) error {
	recipe, dir := "", ""
	for index := 0; index < len(args); index++ {
		switch arg := args[index]; {
		case arg == "--dir":
			if index+1 >= len(args) {
				return errors.New("--dir braucht einen Pfad")
			}
			index++
			dir = args[index]
		case strings.HasPrefix(arg, "--dir="):
			dir = strings.TrimPrefix(arg, "--dir=")
		case strings.HasPrefix(arg, "-"):
			return fmt.Errorf("unbekannte Option: %s", arg)
		case recipe == "":
			recipe = arg
		default:
			return errors.New("erwartet genau ein Rezept: k-playbook review workdir <rezept> [--dir <pfad>]")
		}
	}
	if recipe == "" {
		return errors.New("erwartet ein Rezept: k-playbook review workdir <rezept> [--dir <pfad>]")
	}
	environment := project.Detect()
	if !environment.Installed {
		return fmt.Errorf("keine %s gefunden — gesucht ab %s aufwärts",
			project.ConfigFileName, project.DisplayPath(environment.SearchedFrom))
	}
	prepared, err := workdir.Prepare(environment.ProjectDir, recipe, dir, time.Now())
	if err != nil {
		return err
	}
	return writeReviewJSON(out, prepared)
}

func runReviewMerge(out io.Writer, args []string) error {
	dir, check := "", false
	for _, arg := range args {
		switch {
		case arg == "--check":
			check = true
		case strings.HasPrefix(arg, "-"):
			return fmt.Errorf("unbekannte Option: %s", arg)
		case dir == "":
			dir = arg
		default:
			return errors.New("erwartet genau ein Arbeitsverzeichnis: k-playbook review merge <verzeichnis> [--check]")
		}
	}
	if dir == "" {
		return errors.New("erwartet ein Arbeitsverzeichnis: k-playbook review merge <verzeichnis> [--check]")
	}
	environment := project.Detect()
	if !environment.Installed {
		return fmt.Errorf("keine %s gefunden — gesucht ab %s aufwärts",
			project.ConfigFileName, project.DisplayPath(environment.SearchedFrom))
	}
	output, err := workdir.Merge(environment.ProjectDir, dir, check, kPlaybookVersion(), merge.SeverityCatalog(project.SharedDir()))
	if err != nil {
		return err
	}
	return writeReviewJSON(out, output)
}

func writeReviewJSON(out io.Writer, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, string(data))
	return err
}
