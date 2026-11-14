package main

import (
	"bytes"
	"encoding/json"
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

	"github.com/kascada/k-playbook/installer/internal/release/releasetest"
)

// Ein SIGTERM an `k-playbook upgrade` oder ein Strg-C (SIGINT an die Gruppe
// des Terminals), während das install.sh von Kephalaion läuft: Erst endet die
// Prozessgruppe des Skripts samt seinem Enkel — dem Download —, dann das
// Programm durch dasselbe Signal. Es meldet nichts mehr und läuft nicht mit
// dem nächsten Schritt weiter (Task 152).
//
// Das Ersatz-install.sh startet einen langlebigen Enkel und wartet auf ihn;
// das Test-Binary läuft als k-playbook (mainArgsEnv), sucht kephalaion nicht
// auf diesem Rechner (kephtest.IsolateBinary in TestMain) und lädt das Skript
// von der Ersatzquelle.
func TestUpgradeSignalBeendetInstallSkriptUndProgramm(t *testing.T) {
	for _, tc := range []struct {
		name  string
		sig   syscall.Signal
		group bool
	}{
		{"SIGTERM an das Programm", syscall.SIGTERM, false},
		{"Strg-C an die Gruppe", syscall.SIGINT, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := todoProjectRoot(t)
			t.Setenv("HOME", t.TempDir())
			marks := t.TempDir()
			child, grandchild := filepath.Join(marks, "kind"), filepath.Join(marks, "enkel")
			releasetest.Serve(t).SetKephalaionInstall(fmt.Sprintf(
				"sleep 300 &\necho $! > %q.tmp && mv %q.tmp %q\necho $$ > %q.tmp && mv %q.tmp %q\nwait\n",
				grandchild, grandchild, grandchild, child, child, child))

			raw, err := json.Marshal([]string{"upgrade"})
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(os.Args[0])
			cmd.Env = append(os.Environ(), mainArgsEnv+"="+string(raw))
			cmd.Dir = root
			cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }()

			pids := map[string]int{}
			for name, path := range map[string]string{"Kind": child, "Enkel": grandchild} {
				waitForMarker(t, cmd, path, &stderr)
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
				if err != nil {
					t.Fatal(err)
				}
				pids[name] = pid
			}

			target := cmd.Process.Pid
			if tc.group {
				target = -target
			}
			if err := syscall.Kill(target, tc.sig); err != nil {
				t.Fatal(err)
			}
			if code := waitWithin(t, cmd, tc.sig.String()); code != 128+int(tc.sig) {
				t.Errorf("Ende mit %d, erwartet %d (durch %s); stdout %q, stderr %q", code, 128+int(tc.sig), tc.sig, stdout.String(), stderr.String())
			}
			if stdout.Len() != 0 || stderr.Len() != 0 {
				t.Errorf("das Programm meldete nach dem Signal noch etwas: stdout %q, stderr %q", stdout.String(), stderr.String())
			}
			assertProcessesGone(t, pids)
		})
	}
}

// assertProcessesGone prüft, dass alle pids binnen zwei Sekunden nicht mehr
// laufen — ein Zombie, den init noch einsammeln muss, zählt nicht —, und
// beendet sonst selbst, was übrig ist.
func assertProcessesGone(t *testing.T, pids map[string]int) {
	t.Helper()
	gone := func(pid int) bool {
		if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
			return true
		}
		data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
		if err != nil {
			return false
		}
		end := strings.LastIndexByte(string(data), ')')
		fields := strings.Fields(string(data)[end+1:])
		return end >= 0 && len(fields) > 0 && fields[0] == "Z"
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		all := true
		for _, pid := range pids {
			all = all && gone(pid)
		}
		if all {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	for name, pid := range pids {
		if !gone(pid) {
			t.Errorf("%s des install.sh (PID %d) läuft nach dem Signal weiter", name, pid)
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	}
}
