package main

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/kascada/k-playbook/installer/internal/guiproc"
)

// stopEnvironment macht ein leeres Verzeichnis zum Projekt (Schlüssel =
// Arbeitsverzeichnis) und ein eigenes Laufzeitverzeichnis.
func stopEnvironment(t *testing.T) {
	t.Helper()

	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	before, err := os.Getwd()
	if err != nil {
		t.Fatalf("Arbeitsverzeichnis: %v", err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("wechseln: %v", err)
	}
	t.Cleanup(func() { os.Chdir(before) })
}

// Ohne Laufzeitdatei ist stop eine Auskunft und kein Fehler.
func TestStopOhneServer(t *testing.T) {
	stopEnvironment(t)

	var out bytes.Buffer
	if err := runStop(&out); err != nil {
		t.Fatalf("Fehler: %v", err)
	}
	if !strings.Contains(out.String(), "Kein Server") {
		t.Errorf("Ausgabe: %q", out.String())
	}
}

// Eine verwaiste Datei — der Prozess ist tot — wird gelöscht, Ende 0.
func TestStopEntferntVerwaisteDatei(t *testing.T) {
	stopEnvironment(t)

	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Skipf("Kindprozess: %v", err)
	}
	key, err := guiproc.Key()
	if err != nil {
		t.Fatalf("Schlüssel: %v", err)
	}
	registration, err := guiproc.Register(guiproc.Record{
		Key:       key,
		Addr:      "127.0.0.1:1",
		PID:       cmd.Process.Pid,
		StartTime: time.Now().Unix(),
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	var out bytes.Buffer
	if err := runStop(&out); err != nil {
		t.Fatalf("Fehler: %v", err)
	}
	if !strings.Contains(out.String(), "verwaiste Laufzeitdatei entfernt") {
		t.Errorf("Ausgabe: %q", out.String())
	}
	if _, ok, _ := guiproc.Read(registration.Path()); ok {
		t.Error("die verwaiste Datei liegt noch")
	}
}
