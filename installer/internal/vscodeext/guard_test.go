package vscodeext

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// erlaubeInstall hebt die Marke für diesen Test auf.
//
// Nur zusammen mit einer Fake-CLI in isoliertem HOME: aufgehoben ist die
// Abdichtung, die verhindert, dass ein Testlauf in das VS Code des Rechners
// eingreift. Jeder Aufrufer muss sicher sein, dass die CLI, die er stellt,
// nicht die echte ist.
func erlaubeInstall(t *testing.T) {
	t.Helper()
	t.Setenv(EnvNoInstall, "")
}

// Die Marke steht in jedem Test-Binary von selbst — gesetzt in init(), damit
// sie gilt, bevor ein Testlauf den ersten Prozess startet, und damit sie jedes
// Kind über die Umgebung erbt.
func TestMarkeStehtInJedemTestBinary(t *testing.T) {
	if os.Getenv(EnvNoInstall) == "" {
		t.Fatalf("%s ist im Testlauf nicht gesetzt", EnvNoInstall)
	}
	if !installBlocked() {
		t.Error("installBlocked() ist falsch, obwohl die Marke steht")
	}
}

// Gesperrt ruft Install keine CLI: die Fake-CLI schreibt ihre Argumente
// nirgends hin, und der Fehler nennt die Variable.
func TestInstallGesperrtRuftKeineCLI(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "argumente")
	cli := fakeCLI(t, marker, 0, "sollte nicht aufgerufen werden")

	_, err := Install(context.Background(), cli, []byte("VSIX-Inhalt"))
	if !errors.Is(err, ErrNoInstall) {
		t.Fatalf("Install = %v, erwartet %v", err, ErrNoInstall)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Errorf("die CLI wurde gerufen: %s liegt (%v)", marker, err)
	}
}

// Gesperrt sucht EnsureInstalled nicht einmal nach einer CLI: nichts im Log,
// nichts aufgerufen — aber ein Vermerk, damit die Statusanzeige den Zustand
// nennen kann und er nicht wie „noch nicht gelaufen" aussieht.
func TestEnsureInstalledGesperrt(t *testing.T) {
	home, lockDir := vorbereiten(t)
	t.Setenv(EnvNoInstall, "1")
	codeServer(t, home, 0, "sollte nicht aufgerufen werden")

	var log bytes.Buffer
	EnsureInstalled(context.Background(), lockDir, &log)

	if log.Len() != 0 {
		t.Errorf("Log = %q, erwartet still", log.String())
	}
	refresh := LastRefresh()
	if refresh == nil || refresh.Action != ActionBlocked {
		t.Fatalf("Nachzug = %+v, erwartet %q", refresh, ActionBlocked)
	}
	if refresh.CLI != nil {
		t.Errorf("es wurde eine CLI gewählt: %+v", refresh.CLI)
	}
}
