package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/health"
)

// Exit-Codes von `k-playbook health`: 0 ohne Fehler — Warnungen zählen nicht —,
// 1 mit mindestens einem Fehler, 2 bei Aufruffehlern (unbekannte Option, kein
// Projekt).
const (
	healthExitProblems = 1
	healthExitUsage    = 2
)

// runHealth gibt die Umgebungsprüfung aus — dieselbe Fachlogik, die das
// MCP-Werkzeug k_playbook_health als Hülle anbietet und die das Briefing als
// Abschnitt health trägt: Fehler und Warnungen über die Umgebung des
// Projekts, je Eintrag Stufe, Bereich, Satz und Abhilfe. Ohne --json der
// Klartext, derselbe Block wie im Briefing; mit --json der Report.
func runHealth(args []string) error {
	asJSON := false
	for _, arg := range args {
		switch arg {
		case "--json":
			asJSON = true
		case "-h", "--help", "help":
			printHealthUsage(os.Stdout)
			return nil
		default:
			return exitError{code: healthExitUsage, err: fmt.Errorf("unbekannte Option %s — erwartet wird health [--json]", arg)}
		}
	}

	projectDir, err := todoProjectDir()
	if err != nil {
		return exitError{code: healthExitUsage, err: err}
	}
	report := health.Check(context.Background(), projectDir)

	if asJSON {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			return err
		}
	} else if err := writeHealthText(os.Stdout, report); err != nil {
		return err
	}
	if !report.OK {
		// Die Liste steht schon auf stdout; der Exit-Code sagt es dem Skript.
		return exitError{code: healthExitProblems}
	}
	return nil
}

// writeHealthText rendert den Report zum Lesen: ohne Problem eine Zeile,
// sonst eine Zeile je Eintrag mit Stufe, Bereich, Satz und Abhilfe. Dieselbe
// Form steht im Klartext des Briefings.
func writeHealthText(out io.Writer, report health.Report) error {
	_, err := io.WriteString(out, healthText(report))
	return err
}

func healthText(report health.Report) string {
	if len(report.Problems) == 0 {
		return "Umgebung: keine Probleme\n"
	}
	var b strings.Builder
	b.WriteString("Umgebung:\n")
	for _, problem := range report.Problems {
		fmt.Fprintf(&b, "  [%s] %s: %s", problem.Level, problem.Area, problem.Message)
		if problem.Hint != "" {
			fmt.Fprintf(&b, " — %s", problem.Hint)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// exitError trägt einen Exit-Code zu einem Fehler. Ohne err schweigt main und
// setzt nur den Code — für den Fall, dass die Ausgabe schon alles sagt.
type exitError struct {
	code int
	err  error
}

func (e exitError) Error() string {
	if e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e exitError) Unwrap() error { return e.err }

// exitCodeOf nennt den Exit-Code zu einem Fehler von run: den eines
// exitError, sonst 1.
func exitCodeOf(err error) int {
	var coded exitError
	if errors.As(err, &coded) {
		return coded.code
	}
	return 1
}

func printHealthUsage(out io.Writer) {
	fmt.Fprint(out, `k-playbook health — Fehler und Warnungen über die Umgebung des Projekts

Aufruf:
  k-playbook health [--json]

Prüft die Umgebung des Projekts und nennt alles, was nicht stimmt — nicht nur
das erste Problem. Je Eintrag Stufe (error oder warning), Bereich, ein Satz
und die Abhilfe; nie ein Token oder ein Wert aus einer Datei (Dienst- und
Schlüsselnamen dürfen vorkommen). Repariert wird nichts — außer dass der
Bereich github die fehlende Zugangsdatei einmal als Vorlage anlegt. Die
Bereiche heute:

  kephalaion      eingetragen, aber nicht ok: error; nicht eingetragen: warning
  gh              das Projekt setzt gh voraus (tools.gh.status enabled), aber
                  die CLI fehlt oder ist nicht angemeldet: warning
  baseTools       je fehlendem Basis-Werkzeug eine warning
  versionSources  k-playbook-local/version-sources.yaml da, aber nicht lesbar
                  oder von unbekannter Fassung: error
  github          die persönliche GitHub-Zugangsdatei ~/.config/k-playbook/
                  github.yaml sagt etwas Falsches: je Problem ein error mit
                  dem Pfad in der Datei vorn im Satz; Datei fehlte und wurde
                  gerade als Vorlage angelegt: einmal warning

Ohne --json der Klartext: ohne Problem die Zeile „Umgebung: keine Probleme“,
sonst „Umgebung:“ und je Eintrag „[stufe] bereich: satz — abhilfe“ — derselbe
Block, den das Briefing trägt. Mit --json der Report {ok, problems}, je Eintrag
level, area, message, hint; ok ist false, sobald ein Eintrag error trägt.

Exit-Code 0 ohne Fehler (Warnungen zählen nicht), 1 mit mindestens einem
Fehler, 2 bei Aufruffehlern. Gesucht wird das Projekt ab dem
Arbeitsverzeichnis aufwärts. Dasselbe wie das MCP-Werkzeug k_playbook_health.
`)
}
