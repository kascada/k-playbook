package webui

import (
	"path/filepath"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/project"
	"github.com/kascada/k-playbook/installer/internal/sharedir/sharetest"
)

func TestPreflightBlockedByVenv(t *testing.T) {
	for _, message := range []string{
		"ERROR: Ein Python-venv ist aktiv (/tmp/projekt/.venv).",
		"ERROR: PATH enthält ein typisches Projekt-venv (/tmp/projekt/.venv/bin).",
	} {
		if !preflightBlockedByVenv(message) {
			t.Errorf("preflightBlockedByVenv(%q) = false, erwartet true", message)
		}
	}

	if preflightBlockedByVenv("ERROR: Security-Tool-Matrix fehlt") {
		t.Error("nicht-venv-Fehler wurde als venv-Blockade erkannt")
	}
}

func TestFallbackToolInstallCommand(t *testing.T) {
	root := t.TempDir()
	want := "bash '" + filepath.Join(sharetest.Dir(t), project.ToolScriptPath) + "' --languages python,go --install missing --method venv"

	got := fallbackToolInstallCommand(root, []string{"python", "go"}, " --method venv")
	if got != want {
		t.Errorf("fallbackToolInstallCommand = %q, erwartet %q", got, want)
	}
}
