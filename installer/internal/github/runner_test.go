package github

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

// Hält ein Kindprozess die Pipes offen, kehrt ExecRunner trotzdem kurz nach der
// Frist zurück. Der Abbruch tötet nur den Prozess selbst; ohne WaitDelay
// wartete Run, bis auch das Kind endet — hier 5 s.
func TestExecRunnerKehrtTrotzOffenerPipesZurueck(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("braucht sh")
	}
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh fehlt")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	started := time.Now()
	_, err := ExecRunner{}.Run(ctx, t.TempDir(), "sh", "-c", "sleep 5 & wait")
	elapsed := time.Since(started)

	if err == nil {
		t.Fatal("kein Fehler, obwohl die Frist abgelaufen ist")
	}
	if elapsed > 100*time.Millisecond+waitDelay+2*time.Second {
		t.Errorf("Rückkehr nach %s: Run wartete auf den Kindprozess", elapsed)
	}
	var commandErr *CommandError
	if !errors.As(err, &commandErr) {
		t.Errorf("Fehler %T, erwartet *CommandError", err)
	}
}
