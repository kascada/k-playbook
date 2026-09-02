// Command k-playbook ist das Werkzeug für die projektlokale k-playbook-Installation.
//
// Ohne Argument startet es die lokale Oberfläche. Das Unterkommando `context`
// gibt den aufgelösten Arbeitsstand als JSON aus, `mcp` bietet dieselbe Auskunft
// einem Assistenten als MCP-Werkzeug an, `scan` führt die Werkzeug-Einträge
// eines Review-Laufs aus, und `merge` fasst einen Lauf als Review-Input zusammen.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/kascada/k-playbook/installer/internal/hostinstall"
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
		cleanUpLegacy()
		mirrorHostInstall()
		return webui.Run()
	}

	switch args[0] {
	case "config":
		return runConfig(args[1:])
	case "context":
		// Ohne cleanUpLegacy: dessen Ausgabe würde die JSON-Ausgabe stören.
		// Die Assistenten-Verlinkung zieht ContextForDir dabei nach — das ist
		// projektbezogen und gehört deshalb hierher, nicht in cleanUpLegacy,
		// das die Host-Installation aufräumt.
		return printContext()
	case "mcp":
		// Ohne cleanUpLegacy und mirrorHostInstall, aus demselben Grund wie bei
		// context — hier sogar zwingend: stdout trägt den JSON-RPC-Strom, und der
		// bleibt über die ganze Sitzung offen. Eine einzige Zeile daneben macht
		// die Verbindung unbrauchbar.
		return mcpserver.Run(context.Background())
	case "scan":
		// Ohne cleanUpLegacy und mirrorHostInstall: beides gehört zum Start der
		// Oberfläche. Ein Scan liest nur und soll den Host nicht nebenbei
		// anfassen, während er läuft.
		return runScan(args[1:])
	case "merge":
		// Wie scan: ein Merge arbeitet auf einem vorhandenen Lauf und fasst dessen
		// Artefakte zusammen, ohne die Host-Installation anzufassen.
		return runMerge(args[1:])
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

Ohne Argument:  startet die lokale Oberfläche im Browser.

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

// mirrorHostInstall hält die host-weite Kopie auf dem Stand dieses Aufrufs,
// damit `k-playbook` aus jedem Verzeichnis startbar ist.
//
// Nur hier, nicht bei `context`: dessen JSON darf keine Beigaben bekommen, und
// die Spiegelung braucht ein Git-Kommando, das den häufigen Kontextaufrufen
// der Commands nichts bringt.
//
// Wie cleanUpLegacy meldet sie sich nur, wenn etwas passiert ist, und ein
// Fehler hält den Start nicht auf: die Oberfläche läuft auch ohne Kopie.
func mirrorHostInstall() {
	result, err := hostinstall.Mirror()
	if len(result.Copied) > 0 {
		fmt.Printf("Host-weite Kopie aktualisiert (%d):\n", len(result.Copied))
		for _, copied := range result.Copied {
			fmt.Printf("  - %s\n", copied)
		}
	}
	if result.Link != "" {
		fmt.Printf("Verlinkt: %s\n", result.Link)
	}
	// Ohne Bedingung auf result: der PATH stimmt auch dann noch nicht, wenn
	// diesmal nichts zu spiegeln war. Sonst sähe man den Hinweis genau einmal.
	if status := hostinstall.CheckPath(); status.Export != "" {
		fmt.Printf("Hinweis: %s liegt nicht im PATH. Diese Zeile ins Shell-Profil:\n", status.Dir)
		fmt.Printf("  %s\n", status.Export)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Hinweis: host-weite Kopie nicht aktualisiert: %v\n", err)
	}
}
