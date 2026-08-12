// Command k-playbook ist das Werkzeug für die projektlokale k-playbook-Installation.
//
// Ohne Argument startet es die lokale Oberfläche. Das Unterkommando `context`
// gibt den aufgelösten Arbeitsstand als JSON aus.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/kascada/k-playbook/installer/internal/hostinstall"
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
		mirrorHostInstall()
		return webui.Run()
	}

	switch args[0] {
	case "context":
		// Ohne cleanUpLegacy: dessen Ausgabe würde die JSON-Ausgabe stören.
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
  context   Gibt den aufgelösten Arbeitsstand als JSON aus: Pfade,
            Konfiguration und die effektiven Kataloge für rules, reviews
            und checks. Gesucht wird ab dem Arbeitsverzeichnis aufwärts.
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
