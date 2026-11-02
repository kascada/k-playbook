package main

import (
	"fmt"
	"os"

	"github.com/kascada/k-playbook/installer/internal/guiproc"
	"github.com/kascada/k-playbook/installer/internal/sharedir"
)

// ensureShare bringt ~/.local/share/k-playbook/ auf den Stand dieses
// Programms, bevor das Kommando läuft.
//
// Anders als die Wirt-Pflege läuft es bei jedem Aufruf: Das Verzeichnis gilt
// für den ganzen Rechner, und welcher Weg ein neues Programm hingelegt hat —
// bin/install, make dev-install, k-playbook upgrade, ein Container mit eigener
// Installation —, soll keine Rolle spielen. Im Normalfall ist es eine kleine
// Datei lesen und vergleichen. Es schreibt nie nach stdout, das bei mcp den
// JSON-RPC-Strom und bei context, briefing und read die Antwort trägt; ein
// Fehler ist ein Hinweis auf stderr und hält das Kommando nicht auf.
//
// In main und nicht in run, damit die Tests, die run direkt rufen, nicht in
// das echte Home schreiben. Ausgenommen sind version und help: Die Oberfläche
// fragt mit `version` eine Datei am Installationsziel ab, bevor sie
// entscheidet, ob sie installiert — dieser Aufruf soll nichts anfassen —,
// und github run, das außerhalb der Umgebung seines Befehls nichts ändert.
func ensureShare(args []string) {
	if len(args) > 0 {
		switch args[0] {
		case "version", "help", "-h", "--help":
			return
		case "github":
			// github run richtet nur die Umgebung eines Befehls ein und
			// schreibt sonst nichts (docs/github-access.md, What is set up);
			// die mitgelieferten Dateien braucht es nicht.
			if len(args) > 1 && args[1] == "run" {
				return
			}
		}
	}
	if _, err := sharedir.Ensure(guiproc.OwnVersion()); err != nil {
		fmt.Fprintf(os.Stderr, "Hinweis: mitgelieferte Dateien nicht entpackt: %v\n", err)
	}
}
