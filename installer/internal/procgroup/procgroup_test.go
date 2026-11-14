package procgroup

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// callerEnv schaltet das Test-Binary in den Betrieb als Aufrufer: Es
// installiert NotifyContext und startet über Run ein Skript mit einem
// langlebigen Enkel (groupScript), das Verzeichnis für dessen Marken steht in
// der Variablen. Kehrt Run zurück, meldet der Aufrufer das auf stderr und
// endet mit 0 — genau das darf nach einem Signal nicht geschehen.
const callerEnv = "K_PLAYBOOK_TEST_PROCGROUP_CALLER"

// groupScript schreibt die eigene PID nach $1/kind und die eines
// langlebigen Enkels nach $1/enkel und wartet auf den Enkel. Der Enkel hält
// stdout offen, wie ein Download des install.sh die Pipe offen hielte.
const groupScript = `sleep 300 &
echo $! > "$1/enkel.tmp" && mv "$1/enkel.tmp" "$1/enkel"
echo $$ > "$1/kind.tmp" && mv "$1/kind.tmp" "$1/kind"
wait`

func TestMain(m *testing.M) {
	if dir, ok := os.LookupEnv(callerEnv); ok {
		os.Unsetenv(callerEnv)
		ctx, stop := NotifyContext(context.Background())
		defer stop()
		cmd := Command(ctx, "sh", "-c", groupScript, "_", dir)
		var output bytes.Buffer
		cmd.Stdout, cmd.Stderr = &output, &output
		err := Run(cmd)
		HoldIfEnding()
		fmt.Fprintf(os.Stderr, "weitergelaufen nach Run: %v\n", err)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// waitForPIDs wartet, bis das Skript Kind und Enkel gemeldet hat.
func waitForPIDs(t *testing.T, dir string) (child int, grandchild int) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		child, childErr := readPID(filepath.Join(dir, "kind"))
		grandchild, grandchildErr := readPID(filepath.Join(dir, "enkel"))
		if childErr == nil && grandchildErr == nil {
			return child, grandchild
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("das Skript hat Kind und Enkel nicht gemeldet")
	return 0, 0
}

func readPID(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

// alive sagt, ob pid noch läuft. Ein Zombie läuft nicht mehr: Er wartet nur
// darauf, dass init ihn einsammelt.
func alive(pid int) bool {
	if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
		return false
	}
	if data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid)); err == nil {
		// Feld 3 nach dem Namen in Klammern ist der Zustand.
		if end := strings.LastIndexByte(string(data), ')'); end >= 0 {
			fields := strings.Fields(string(data)[end+1:])
			if len(fields) > 0 && fields[0] == "Z" {
				return false
			}
		}
	}
	return true
}

// assertGone prüft, dass alle pids binnen kurzer Frist beendet sind, und
// räumt sonst selbst auf.
func assertGone(t *testing.T, within time.Duration, pids map[string]int) {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		running := false
		for _, pid := range pids {
			running = running || alive(pid)
		}
		if !running {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	for name, pid := range pids {
		if alive(pid) {
			t.Errorf("%s (PID %d) läuft %s danach noch", name, pid, within)
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	}
}

// Ein SIGINT oder SIGTERM an die Gruppe des Aufrufers — Strg-C im Terminal
// bzw. das Beenden von außen — erreicht die eigene Gruppe von Run nicht
// unmittelbar. Der Handler aus NotifyContext beendet sie: binnen kurzer Frist
// laufen weder Kind noch Enkel, und danach endet der Aufrufer durch dasselbe
// Signal, ohne dass Run zurückkehrt und er weiterläuft oder etwas meldet.
func TestSignalBeendetErstDieGruppeDannDenAufrufer(t *testing.T) {
	for _, sig := range []syscall.Signal{syscall.SIGTERM, syscall.SIGINT} {
		t.Run(sig.String(), func(t *testing.T) {
			dir := t.TempDir()
			caller := exec.Command(os.Args[0], "-test.run=^$")
			caller.Env = append(os.Environ(), callerEnv+"="+dir)
			// Eine eigene Gruppe für den Aufrufer, wie sie ein Terminal einem
			// Vordergrundauftrag gibt.
			caller.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
			var stderr bytes.Buffer
			caller.Stderr = &stderr
			if err := caller.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = syscall.Kill(-caller.Process.Pid, syscall.SIGKILL) }()
			ended := make(chan error, 1)
			go func() { ended <- caller.Wait() }()

			child, grandchild := waitForPIDs(t, dir)
			if err := syscall.Kill(-caller.Process.Pid, sig); err != nil {
				t.Fatal(err)
			}

			select {
			case err := <-ended:
				var exitErr *exec.ExitError
				if !errors.As(err, &exitErr) {
					t.Fatalf("Aufrufer endete mit %v, erwartet: durch %s; stderr %q", err, sig, stderr.String())
				}
				status, ok := exitErr.Sys().(syscall.WaitStatus)
				if !ok || !status.Signaled() || status.Signal() != sig {
					t.Errorf("Aufrufer endete mit %v, erwartet: durch %s; stderr %q", err, sig, stderr.String())
				}
			case <-time.After(10 * time.Second):
				t.Fatalf("der Aufrufer läuft 10 s nach %s weiter; stderr %q", sig, stderr.String())
			}
			if stderr.Len() != 0 {
				t.Errorf("der Aufrufer meldete etwas: %q", stderr.String())
			}
			assertGone(t, 2*time.Second, map[string]int{"Kind": child, "Enkel": grandchild})
		})
	}
}

// Endet der Kontext durch Abbruch (nicht durch eine Frist), endet die ganze
// Gruppe, auch der Enkel, und Run kehrt zurück. Das ist der Weg, über den der
// Kontext aus NotifyContext eine Gruppe erreicht — für runKephalaionInstall
// wie für jeden anderen Aufrufer von Run.
func TestAbbruchBeendetDieGanzeGruppe(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := Command(ctx, "sh", "-c", groupScript, "_", dir)
	var output bytes.Buffer
	cmd.Stdout = &output
	returned := make(chan error, 1)
	go func() { returned <- Run(cmd) }()

	child, grandchild := waitForPIDs(t, dir)
	start := time.Now()
	cancel()
	select {
	case err := <-returned:
		if err == nil {
			t.Error("Run ohne Fehler nach dem Abbruch")
		}
		if elapsed := time.Since(start); elapsed > time.Second {
			t.Errorf("Run kehrte erst %s nach dem Abbruch zurück", elapsed)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Run kehrt nach dem Abbruch nicht zurück")
	}
	assertGone(t, 2*time.Second, map[string]int{"Kind": child, "Enkel": grandchild})
}

// Läuft die Frist ab, kehrt Run mit ihr zurück — nicht erst nach Frist plus
// WaitDelay, obwohl der Enkel die Ausgabe offen hielte —, und der Enkel ist
// beendet.
func TestFristKehrtMitDerFristZurueck(t *testing.T) {
	before := WaitDelay
	WaitDelay = 10 * time.Second
	t.Cleanup(func() { WaitDelay = before })

	dir := t.TempDir()
	const timeout = 300 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := Command(ctx, "sh", "-c", groupScript, "_", dir)
	var output bytes.Buffer
	cmd.Stdout = &output

	start := time.Now()
	err := Run(cmd)
	elapsed := time.Since(start)
	if err == nil || ctx.Err() != context.DeadlineExceeded {
		t.Fatalf("Run = %v, Kontext %v", err, ctx.Err())
	}
	if elapsed > timeout+time.Second {
		t.Errorf("Run kehrte erst nach %s zurück, Frist %s", elapsed, timeout)
	}
	child, grandchild := waitForPIDs(t, dir)
	assertGone(t, 2*time.Second, map[string]int{"Kind": child, "Enkel": grandchild})
}

// Ohne Frist und Abbruch läuft der Prozess zu Ende, und Run gibt sein
// Ergebnis zurück.
func TestRunOhneAbbruch(t *testing.T) {
	cmd := Command(context.Background(), "sh", "-c", "echo ausgabe; exit 3")
	var output bytes.Buffer
	cmd.Stdout = &output
	err := Run(cmd)
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 3 {
		t.Errorf("Run = %v, erwartet Exit-Code 3", err)
	}
	if output.String() != "ausgabe\n" {
		t.Errorf("Ausgabe = %q", output.String())
	}
	groups.mu.Lock()
	defer groups.mu.Unlock()
	if len(groups.running) != 0 {
		t.Errorf("noch angemeldete Gruppen: %v", groups.running)
	}
}

// Ohne Signal endet der Kontext aus NotifyContext erst mit stop.
func TestNotifyContextEndetMitStop(t *testing.T) {
	ctx, stop := NotifyContext(context.Background())
	if ctx.Err() != nil {
		t.Fatalf("Kontext vor stop beendet: %v", ctx.Err())
	}
	stop()
	if ctx.Err() == nil {
		t.Error("Kontext nach stop nicht beendet")
	}
	stop()
}
