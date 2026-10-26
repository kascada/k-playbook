package mcpserver

import (
	"fmt"
	"os"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/githubaccess/accesstest"
)

// TestMain schirmt das Paket von der persönlichen GitHub-Zugangsdatei des
// Rechners ab: k_playbook_health und k_playbook_briefing erreichen deren
// Leser, und der legte bei fehlender Datei am echten Ort die Vorlage an.
func TestMain(m *testing.M) {
	cleanup, err := accesstest.Isolate()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	code := m.Run()
	cleanup()
	os.Exit(code)
}
