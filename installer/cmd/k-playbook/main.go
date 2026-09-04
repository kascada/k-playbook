// Command k-playbook ist das Werkzeug für die projektlokale k-playbook-Installation.
//
// Ohne Argument startet es die lokale Oberfläche. Das Unterkommando `context`
// gibt den aufgelösten Arbeitsstand als JSON aus, `mcp` bietet dieselbe Auskunft
// einem Assistenten als MCP-Werkzeug an, `scan` führt die Werkzeug-Einträge
// eines Review-Laufs aus, `merge` fasst einen Lauf als Review-Input zusammen,
// und `stop` beendet den Hintergrunddienst der Oberfläche.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/kascada/k-playbook/installer/internal/guiproc"
	"github.com/kascada/k-playbook/installer/internal/legacy"
	"github.com/kascada/k-playbook/installer/internal/mcpserver"
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
		if guiproc.ServeMode() {
			// Der abgekoppelte Server, gestartet vom argumentlosen Aufruf.
			// Keine Wirt-Pflege und kein Browser: beides gehört zum Aufruf,
			// der jedes Mal läuft — hier liefe es nur beim allerersten Start.
			// Die Marke wird gelöscht, damit kein Kindprozess des Servers sie
			// erbt und selbst zum Server wird.
			os.Unsetenv(guiproc.ServeEnv)
			return webui.Serve()
		}
		return runGUI()
	}

	switch args[0] {
	case "config":
		return runConfig(args[1:])
	case "context":
		// Ohne Wirt-Pflege: deren Ausgabe würde die JSON-Ausgabe stören.
		// Die Assistenten-Verlinkung zieht ContextForDir dabei nach — das ist
		// projektbezogen und gehört deshalb hierher, nicht in den Startpfad.
		return printContext()
	case "mcp":
		// Ohne Wirt-Pflege: stdout trägt den JSON-RPC-Strom, und eine einzige
		// fremde Zeile daneben macht die Verbindung unbrauchbar.
		return mcpserver.Run(context.Background())
	case "scan":
		// Ohne Wirt-Pflege: ein Scan liest nur und soll den Host nicht nebenbei
		// anfassen, während er läuft.
		return runScan(args[1:])
	case "merge":
		// Wie scan: ein Merge arbeitet auf einem vorhandenen Lauf und fasst dessen
		// Artefakte zusammen, ohne den Host nebenbei anzufassen.
		return runMerge(args[1:])
	case "stop":
		// Ohne Wirt-Pflege: wer beendet, will nichts einrichten.
		return runStop(os.Stdout)
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		printUsage()
		return fmt.Errorf("unbekanntes Kommando: %s", args[0])
	}
}

// printContext gibt den Arbeitsstand aus: Pfade, Konfiguration und die
// aufgelösten Kataloge. Damit muss ein Command die Overlay-Regeln nicht selbst
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

Ohne Argument:  startet die lokale Oberfläche im Browser. Der Server läuft
                dazu als Hintergrunddienst je Projekt weiter; ein zweiter
                Aufruf findet ihn und öffnet nur den Browser.

Unterkommandos:
  config create [--repo-root <pfad>] [hauptverzeichnis]
             Legt K-PLAYBOOK.yaml explizit an. Ohne Hauptverzeichnis wird das
             Arbeitsverzeichnis verwendet; eine vorhandene Datei bleibt immer
             unverändert.
  context   Gibt den aufgelösten Arbeitsstand als JSON aus: Pfade,
            Konfiguration und die effektiven Kataloge für rules, reviews
            und checks. Gesucht wird ab dem Arbeitsverzeichnis aufwärts.
            Zieht dabei die Assistenten-Verlinkung auf den Katalog nach;
            was sich geändert hat, steht unter "links".
  mcp       Startet einen MCP-Server über stdin/stdout, der dieselbe Auskunft
            als Werkzeug anbietet. Gedacht für den Aufruf durch einen
            Assistenten, nicht für die Hand.
  scan      Führt die Werkzeug-Einträge eines Review-Laufs aus:
            k-playbook scan <lauf> [eintrag …]. Ohne Eintragsangabe alle, die
            noch nicht gelaufen sind. Die Namen sind die der Werkzeuge. Das
            Kommando kehrt zurück, wenn alle Einträge durch sind; der
            Fortschritt steht währenddessen unter entries/.
  merge     Fasst einen bestehenden Review-Lauf zusammen:
            k-playbook merge <lauf>. Das Ergebnis wird als Review-Input in das
            Laufverzeichnis geschrieben.
  stop      Beendet den Hintergrunddienst der Oberfläche für dieses Projekt.
            Ohne laufenden Server eine Auskunft, kein Fehler; eine verwaiste
            Laufzeitdatei wird dabei entfernt.
  help      Diese Übersicht.
`)
}

// cleanUpLegacy räumt die host-globale Registrierung des alten Modells weg.
// Sie läuft bei jedem Start und meldet sich nur, wenn tatsächlich etwas
// wegfällt — auf einem sauberen Rechner bleibt sie still. Ein Fehler dabei
// hält den Start nicht auf: die Oberfläche ist auch mit Altlasten bedienbar.
func cleanUpLegacy() {
	removals, err := legacy.RemoveGlobalLinks()
	if len(removals) > 0 {
		fmt.Printf("Alte globale Verlinkung entfernt (%d):\n", len(removals))
		for _, removal := range removals {
			fmt.Printf("  - %s\n", removal)
		}
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Hinweis: alte globale Verlinkung nicht vollständig entfernt: %v\n", err)
	}
}
