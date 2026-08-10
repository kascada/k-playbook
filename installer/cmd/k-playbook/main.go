// Command k-playbook ist das Werkzeug fuer die projektlokale k-playbook-Installation.
//
// Derzeit ein Geruest: es startet die lokale GUI und zeigt eine leere Seite.
// Der bisherige Stand liegt unter installer/_old/ als Nachschlagewerk.
package main

import (
	"fmt"
	"os"

	"github.com/kascada/k-playbook/installer/internal/webui"
)

func main() {
	if err := webui.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Fehler: %v\n", err)
		os.Exit(1)
	}
}
