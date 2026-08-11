// Command k-playbook ist das Werkzeug fuer die projektlokale k-playbook-Installation.
//
// Ohne Argument startet es die lokale Oberflaeche. Das Unterkommando `context`
// gibt den aufgeloesten Arbeitsstand als JSON aus.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/kascada/k-playbook/installer/internal/legacy"
	"github.com/kascada/k-playbook/installer/internal/project"
	"github.com/kascada/k-playbook/installer/internal/webui"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Fehler: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		cleanUpLegacy()
		return webui.Run()
	}

	switch args[0] {
	case "context":
		// Ohne cleanUpLegacy: dessen Ausgabe wuerde die JSON-Ausgabe stoeren.
		return printContext()
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		printUsage()
		return fmt.Errorf("unbekanntes Kommando: %s", args[0])
	}
}

// printContext gibt den Arbeitsstand aus: Pfade, Konfiguration und die
// aufgeloesten Kataloge. Damit muss ein Command die Overlay-Regeln nicht selbst
// anwenden.
func printContext() error {
	workdir, err := os.Getwd()
	if err != nil {
		return err
	}

	context, err := project.ContextForDir(workdir)
	if err != nil {
		return err
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(context)
}

func printUsage() {
	fmt.Fprint(os.Stderr, `k-playbook

Ohne Argument:  startet die lokale Oberflaeche im Browser.

Unterkommandos:
  context   Gibt den aufgeloesten Arbeitsstand als JSON aus: Pfade,
            Konfiguration und die effektiven Kataloge fuer rules, reviews
            und checks. Gesucht wird ab dem Arbeitsverzeichnis aufwaerts.
  help      Diese Uebersicht.
`)
}

// cleanUpLegacy raeumt die host-globale Registrierung des alten Modells weg.
// Sie laeuft bei jedem Start und meldet sich nur, wenn tatsaechlich etwas
// wegfaellt — auf einem sauberen Rechner bleibt sie still. Ein Fehler dabei
// haelt den Start nicht auf: die Oberflaeche ist auch mit Altlasten bedienbar.
func cleanUpLegacy() {
	removals, err := legacy.RemoveGlobalLinks()
	if len(removals) > 0 {
		fmt.Printf("Alte globale Verlinkung entfernt (%d):\n", len(removals))
		for _, removal := range removals {
			fmt.Printf("  - %s\n", removal)
		}
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Hinweis: alte globale Verlinkung nicht vollstaendig entfernt: %v\n", err)
	}
}
