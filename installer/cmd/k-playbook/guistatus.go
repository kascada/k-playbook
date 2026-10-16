package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/kascada/k-playbook/installer/internal/guiproc"
)

// guiUsage ist die Hilfe des Unterkommandos.
const guiUsage = `k-playbook gui

Auskunft über die Oberfläche dieses Projekts, ohne sie anzufassen. Gestartet
wird hier nichts: die Oberfläche startet der argumentlose Aufruf "k-playbook".

Unterkommandos:
  status   Nennt Zustand und Adresse des Dienstes als JSON: "state" mit einem
           von none, running, otherVersion, orphaned, unresponsive und dazu,
           soweit gemessen, "url", "pid", "version" und "build". Nicht
           gemessene Felder fehlen in der Antwort. Ohne Dienst steht dort
           {"state": "none"}, und das ist eine Auskunft und kein Fehler.
           Nebenwirkungsfrei: es startet nichts, beendet nichts und entfernt
           auch keine verwaiste Laufzeitdatei — das bleibt "k-playbook stop".
`

// runGUICommand führt das Unterkommando `gui` aus.
//
// Ohne Unterkommando endet es mit Fehler, anders als `vscode` (runVSCode gibt
// dann nil zurück): der naheliegende Sinn eines nackten `gui` wäre, die
// Oberfläche zu starten — und das tut dieses Unterkommando nicht, das ist der
// argumentlose Aufruf des Programms.
func runGUICommand(args []string) error {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, guiUsage)
		return errors.New("gui erwartet ein Unterkommando: status")
	}
	switch args[0] {
	case "help", "-h", "--help":
		fmt.Fprint(os.Stdout, guiUsage)
		return nil
	case "status":
		return runGUIStatus(os.Stdout, args[1:])
	default:
		fmt.Fprint(os.Stderr, guiUsage)
		return fmt.Errorf("unbekanntes Kommando: gui %s", args[0])
	}
}

// runGUIStatus rendert den Wertesatz aus guiproc.StatusForDir als JSON. Die
// CLI setzt ihr Arbeitsverzeichnis ein und gibt aus, sonst nichts:
// zusammengesetzt wird die Antwort in guiproc, damit das MCP-Werkzeug keine
// zweite Fassung derselben Antwort bildet.
func runGUIStatus(out io.Writer, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("gui status erwartet keine Argumente: %s", args[0])
	}

	workdir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("Arbeitsverzeichnis: %w", err)
	}
	report, err := guiproc.StatusForDir(workdir)
	if err != nil {
		return err
	}

	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}
