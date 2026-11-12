package main

import (
	"fmt"
	"io"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// TroubleshootingCommand ist der Aufruf, auf den Meldungen zu einer kaputten
// Umgebung als nächsten Schritt zeigen — das Briefing, wenn Kephalaion nicht
// erreichbar ist, und die Abhilfe der Gesundheitsprüfung dazu.
const TroubleshootingCommand = project.TroubleshootingCommand

// runTroubleshooting ist vorerst ein Platzhalter (Task 144): Er sagt, wofür
// der Befehl da ist und dass er noch nicht umgesetzt ist, und endet mit 0.
// Die eigentliche Fehlersuche — zuerst Kephalaion, dann Schritt für Schritt
// der nächste Fehler der Umgebung — baut ein eigener Task. Er liest und
// schreibt nichts, auch kein Projekt wird gesucht: Er muss gerade dann
// laufen, wenn die Umgebung kaputt ist.
func runTroubleshooting(out io.Writer, args []string) error {
	for _, arg := range args {
		switch arg {
		case "-h", "--help", "help":
		default:
			return fmt.Errorf("unbekannte Option %s — erwartet wird %s", arg, TroubleshootingCommand)
		}
	}
	fmt.Fprint(out, `k-playbook troubleshooting — Hilfe bei Fehlern der Umgebung

Gedacht ist: Der Befehl prüft zuerst, ob Kephalaion erreichbar ist, und sucht
dann Schritt für Schritt den nächsten Fehler der Umgebung, je mit dem, was zu
tun ist.

Noch nicht umgesetzt. Bis dahin: k-playbook health nennt alle Fehler und
Warnungen mit Abhilfe, kephalaion status den Zustand von Kephalaion auf diesem
Rechner.
`)
	return nil
}
