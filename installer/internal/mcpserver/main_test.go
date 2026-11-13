package mcpserver

import (
	"fmt"
	"os"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/githubaccess/accesstest"
	"github.com/kascada/k-playbook/installer/internal/kephalaion/kephtest"
	"github.com/kascada/k-playbook/installer/internal/project/defaultstest"
	"github.com/kascada/k-playbook/installer/internal/release/releasetest"
)

// TestMain schirmt das Paket von der persönlichen GitHub-Zugangsdatei des
// Rechners ab: k_playbook_health und k_playbook_briefing erreichen deren
// Leser, und der läse sonst die Datei dieses Rechners.
func TestMain(m *testing.M) {
	cleanup, err := accesstest.Isolate()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	cleanupDefaults := defaultstest.Isolate()
	// Kein Test fragt das echte kephalaion dieses Rechners (Task 138).
	cleanupBinary := kephtest.IsolateBinary()
	// Und keiner das echte GitHub nach dem neuesten Release-Tag (Task 149):
	// k_playbook_health und k_playbook_briefing erreichen über die
	// Umgebungsprüfung die Release-Quelle, sobald das Programm eine Version
	// trägt.
	cleanupRelease := releasetest.Isolate()
	code := m.Run()
	cleanupRelease()
	cleanupBinary()
	cleanupDefaults()
	cleanup()
	os.Exit(code)
}
