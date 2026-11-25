package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/procgroup"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// Exit-Codes von `k-playbook findings check`: 0 bereit, 1 nicht bereit, 2 bei
// Aufruffehlern (unbekannte Option, kein Projekt).
const (
	findingsExitNotReady = 1
	findingsExitUsage    = 2
)

// checkFindings ist austauschbar, damit Tests den Node nicht fragen.
var checkFindings = project.CheckFindings

// runFindings ist `k-playbook findings check [--json]`: die Vorprüfung der
// Befund-Queue vor /k-befunde-einordnen — dieselbe Fachlogik, die das
// MCP-Werkzeug k_playbook_findings_check als Hülle anbietet.
func runFindings(args []string) error {
	if len(args) == 0 {
		printFindingsUsage(os.Stdout)
		return nil
	}
	switch args[0] {
	case "-h", "--help", "help":
		printFindingsUsage(os.Stdout)
		return nil
	case "check":
	default:
		printFindingsUsage(os.Stderr)
		return exitError{code: findingsExitUsage, err: fmt.Errorf("unbekannter Unterbefehl: findings %s", args[0])}
	}
	asJSON := false
	for _, arg := range args[1:] {
		switch arg {
		case "--json":
			asJSON = true
		case "-h", "--help":
			printFindingsUsage(os.Stdout)
			return nil
		default:
			return exitError{code: findingsExitUsage, err: fmt.Errorf("unbekannte Option %s — erwartet wird findings check [--json]", arg)}
		}
	}

	projectDir, err := todoProjectDir()
	if err != nil {
		return exitError{code: findingsExitUsage, err: err}
	}
	ctx, stop := procgroup.NotifyContext(context.Background())
	defer stop()
	check := checkFindings(ctx, projectDir)

	if asJSON {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(check); err != nil {
			return err
		}
	} else if _, err := io.WriteString(os.Stdout, findingsCheckText(check)); err != nil {
		return err
	}
	if !check.Ready {
		// Die Gründe stehen schon auf stdout; der Exit-Code sagt es dem Skript.
		return exitError{code: findingsExitNotReady}
	}
	return nil
}

// findingsCheckText ist das Ergebnis zum Lesen: Zustand, Abgleich, Rechte,
// Bestand je Nutzer, Übersprungene und die Probleme.
func findingsCheckText(check project.FindingsCheck) string {
	var b strings.Builder
	if check.Ready {
		b.WriteString("Befund-Queue: bereit\n")
	} else {
		b.WriteString("Befund-Queue: nicht bereit\n")
	}
	if check.Sync != nil {
		fmt.Fprintf(&b, "Abgleich: %s", check.Sync.At)
		if check.Sync.Revision != nil {
			fmt.Fprintf(&b, ", Revision %d", *check.Sync.Revision)
		}
		b.WriteString("\n")
	}
	if len(check.Rights) > 0 {
		fmt.Fprintf(&b, "Rechte: %s\n", strings.Join(check.Rights, ", "))
	}
	fmt.Fprintf(&b, "Befunde: %d%s\n", check.Total, findingsByUserText(check.ByUser))
	if len(check.Skipped) > 0 {
		fmt.Fprintf(&b, "Übersprungen: %d\n", len(check.Skipped))
		for _, s := range check.Skipped {
			fmt.Fprintf(&b, "  %s (%s)\n", s.Name, s.Reason)
		}
	}
	if len(check.Problems) > 0 {
		b.WriteString("Probleme:\n")
		for _, p := range check.Problems {
			fmt.Fprintf(&b, "  [%s] %s\n", p.Kind, p.Message)
		}
	}
	return b.String()
}

// findingsByUserText ist „ (kamran 200, anna 26)“, nach Zahl absteigend,
// leer ohne Befunde.
func findingsByUserText(byUser map[string]int) string {
	if len(byUser) == 0 {
		return ""
	}
	users := make([]string, 0, len(byUser))
	for user := range byUser {
		users = append(users, user)
	}
	sort.Slice(users, func(i, j int) bool {
		if byUser[users[i]] != byUser[users[j]] {
			return byUser[users[i]] > byUser[users[j]]
		}
		return users[i] < users[j]
	})
	parts := make([]string, len(users))
	for i, user := range users {
		parts[i] = fmt.Sprintf("%s %d", user, byUser[user])
	}
	return " (" + strings.Join(parts, ", ") + ")"
}

func printFindingsUsage(out io.Writer) {
	fmt.Fprint(out, `k-playbook findings — die Befund-Queue des Projekts (keph:/queue/findings/)

Aufruf:
  k-playbook findings check [--json]

Die Vorprüfung vor /k-befunde-einordnen. Sie fordert zuerst beim Node den
Abgleich der Replica an (MCP-Werkzeug sync, Node ab Kephalaion v0.5.0; kein
Prozess und keine Datenbank des Nodes auf diesem Rechner, geht also auch in
einem reinen Client wie dem DevContainer), prüft dann die Rechte auf die Collection (read,
write, supersede), dann jeden Eintrag unter queue/findings/ — jeder muss ein
Befund nach dem Namensmuster <nummer>-<kurzname>.md sein, direkt in der
Ablage — und liest zuletzt jeden Befund. Sie meldet alles auf einmal, nicht
nur das erste Problem, und zählt den Bestand auch, wenn sie nicht bereit ist.

Ein leerer oder in den letzten sechs Stunden geänderter Befund wird
übersprungen (skipped, mit Grund „leer“ bzw. „jünger als 6 h“), nicht als
Problem gezählt; er kommt in einem späteren Lauf dran. Eine leere Queue ist bereit.

Geschrieben wird nichts, außer der Replica des Nodes, die der Abgleich
nachzieht.

Ohne --json der Klartext: Zustand, Abgleich, Rechte, Zahl je Nutzer,
Übersprungene und Probleme. Mit --json {ready, problems, sync, rights, total,
byUser, skipped, findings}; problems je mit kind (kephalaion, sync, right,
name, subdir, unreadable, frontmatter), name und message; findings und skipped
je mit name (voller Name in der Collection), ref, revision, user, created,
updated, ziel, thema, skipped dazu reason — beide getrennt und nach Alter sortiert,
ältester zuerst.

Exit-Code 0 bereit, 1 nicht bereit, 2 bei Aufruffehlern. Gesucht wird das
Projekt ab dem Arbeitsverzeichnis aufwärts. Dasselbe wie das MCP-Werkzeug
k_playbook_findings_check.
`)
}
