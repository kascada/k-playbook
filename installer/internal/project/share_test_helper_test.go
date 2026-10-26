package project

import (
	"fmt"
	"os"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/githubaccess/accesstest"
	"github.com/kascada/k-playbook/installer/internal/sharedir/sharetest"
)

// TestMain schirmt das Paket vom echten ~/.local/share/k-playbook ab: Ohne
// eigenes XDG_DATA_HOME läse jeder Test, der den Katalog auflöst, die
// mitgelieferten Commands dieses Rechners. Ebenso von der persönlichen
// GitHub-Zugangsdatei: Jedes health.Check — also jedes Briefing — erreicht
// deren Leser, und der legte am echten Ort die Vorlage an.
func TestMain(m *testing.M) {
	cleanup, err := sharetest.Isolate()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	cleanupGitHub, err := accesstest.Isolate()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	code := m.Run()
	cleanupGitHub()
	cleanup()
	os.Exit(code)
}

// shipped ist das mitgelieferte Verzeichnis dieses Tests, das, was sonst
// ~/.local/share/k-playbook ist.
func shipped(t *testing.T) string {
	t.Helper()
	return sharetest.Dir(t)
}
