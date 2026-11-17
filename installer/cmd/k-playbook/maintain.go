package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// maintainOptions sind die Schalter von `k-playbook maintain`.
type maintainOptions struct {
	only   []string
	dryRun bool
	commit bool
	help   bool
}

const maintainCall = "maintain [--only <schritt>[,<schritt>…]] [--dry-run] [--commit]"

// parseMaintainArgs liest die Schalter. --only darf mehrfach stehen; die
// Schritte werden an Kommas getrennt.
func parseMaintainArgs(args []string) (maintainOptions, error) {
	options := maintainOptions{}
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "--dry-run":
			options.dryRun = true
		case arg == "--commit":
			options.commit = true
		case arg == "-h" || arg == "--help" || arg == "help":
			options.help = true
		case arg == "--only" || strings.HasPrefix(arg, "--only="):
			value, found := strings.CutPrefix(arg, "--only=")
			if !found {
				if index+1 >= len(args) {
					return options, errors.New("--only ohne Schritt — erwartet wird " + maintainCall)
				}
				index++
				value = args[index]
			}
			if options.only == nil {
				options.only = []string{}
			}
			options.only = append(options.only, strings.Split(value, ",")...)
		default:
			return options, fmt.Errorf("unbekannte Option %s — erwartet wird %s", arg, maintainCall)
		}
	}
	return options, nil
}

// runMaintain ist `k-playbook maintain`: der Wartungslauf ohne KI und ohne
// Rückfrage, damit ihn später ein Cron-Job aufrufen kann. Exit-Code 0, wenn
// kein Schritt scheitert (übersprungen zählt nicht), 1 bei einem Fehler, 2 bei
// einem Aufruffehler oder einem unbekannten Schritt.
func runMaintain(args []string) error {
	options, err := parseMaintainArgs(args)
	if err != nil {
		return exitError{code: 2, err: err}
	}
	if options.help {
		printMaintainUsage(os.Stdout)
		return nil
	}
	environment := project.Detect()
	if !environment.Installed {
		return fmt.Errorf("keine %s gefunden — gesucht ab %s aufwärts",
			project.ConfigFileName, project.DisplayPath(environment.SearchedFrom))
	}
	return maintain(context.Background(), environment.ProjectDir, options, os.Stdout, os.Stderr)
}

// maintain führt den Lauf aus und schreibt je Schritt Status und Grund, zuletzt
// das Gesamtergebnis.
func maintain(ctx context.Context, projectDir string, options maintainOptions, out, errOut io.Writer) error {
	result, err := project.RunMaintenance(ctx, projectDir, project.MaintenanceOptions{
		Only: options.only, DryRun: options.dryRun, Commit: options.commit, Out: out, ErrOut: errOut,
	})
	var selection *project.MaintenanceSelectionError
	if errors.As(err, &selection) {
		return exitError{code: 2, err: err}
	}
	if err != nil {
		return err
	}

	mode := ""
	if options.dryRun {
		mode = ", Vorschau ohne Schreiben"
	}
	if len(result.Steps) == 0 {
		fmt.Fprintf(out, "Wartung: keine Schritte ausgewählt (%s)%s.\n", result.Selection, mode)
	}
	for _, step := range result.Steps {
		fmt.Fprintf(out, "%s: %s — %s\n", step.Step, step.Status, step.Reason)
		for _, note := range step.Notes {
			fmt.Fprintf(out, "  %s\n", note)
		}
	}
	failed := 0
	for _, step := range result.Steps {
		if step.Status == project.MaintenanceFailed {
			failed++
		}
	}
	if result.Failed {
		fmt.Fprintf(out, "Ergebnis: %s (%d von %d Schritten; Auswahl: %s%s)\n",
			project.MaintenanceFailed, failed, len(result.Steps), result.Selection, mode)
		return exitError{code: 1}
	}
	fmt.Fprintf(out, "Ergebnis: %s (Auswahl: %s%s)\n", project.MaintenanceOK, result.Selection, mode)
	return nil
}

func printMaintainUsage(out io.Writer) {
	var steps strings.Builder
	for _, step := range project.MaintenanceSteps() {
		fmt.Fprintf(&steps, "  %-12s %s\n", step.Name, step.Summary)
	}
	fmt.Fprintf(out, `k-playbook maintain — Wartungslauf des Projekts, ohne KI und ohne Rückfrage

Aufruf:
  k-playbook %s

Schritte, in dieser festen Reihenfolge:
%s
Ohne --only läuft das Tagesprogramm aus maintenance.daily in K-PLAYBOOK.yaml;
fehlt das Feld, gilt %s. --only wählt Schritte gezielt, mehrere durch
Komma getrennt; ein unbekannter Name in --only oder maintenance.daily wird
vor jedem Schreiben abgelehnt.

  --dry-run  zeigt, was geschähe: ruft den Export mit --dry-run auf, legt
             keine Markierung an, ändert weder Dateien noch Git und nennt, ob
             ein Commit angefordert wäre.
  --commit   committet nach einem vollständigen Export (Exit 0) nur die
             Änderungen im Sicherungsziel; schaltet eine deaktivierte
             Sicherung nicht ein. Ohne Schalter entscheidet
             maintenance.backup.git.commit. Nie ein Push.

Je Schritt eine Zeile mit Status (erfolgreich, übersprungen, Fehler) und
Grund, zuletzt das Gesamtergebnis. Exit-Code 0 ohne Fehler (übersprungen ist
keiner), 1, sobald ein Schritt scheitert, 2 bei Aufruffehlern und unbekannten
Schritten. Vertrag: k-playbook:/docs/handbook/k-playbook-format.md, Abschnitt
maintenance.
`, maintainCall, steps.String(), strings.Join(project.DefaultMaintenanceDaily, ", "))
}
