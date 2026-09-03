package guiproc

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// fakeChild macht einen beliebigen Prozess zum Child, damit Await ohne das
// eigene Binary prüfbar ist.
func fakeChild(t *testing.T, name string, args ...string) *Child {
	t.Helper()

	cmd := exec.Command(name, args...)
	if err := cmd.Start(); err != nil {
		t.Skipf("%s starten: %v", name, err)
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	return &Child{Process: cmd.Process, Exited: exited, LogPath: filepath.Join(t.TempDir(), "x.log")}
}

// Await ist zufrieden, sobald die Datei die PID des Kindes trägt und der
// Server dahinter mit demselben Schlüssel und derselben PID antwortet.
func TestAwaitFindetDasEigeneKind(t *testing.T) {
	child := fakeChild(t, "sleep", "10")
	file := filepath.Join(t.TempDir(), "r.json")

	// Erst eine fremde Datei — die eines gleichzeitigen Starts —, dann die
	// eigene. Nur die zweite zählt.
	if err := writeExclusive(file, Record{Key: "/p", Addr: "a:1", PID: child.Process.Pid + 100000}); err != nil {
		t.Fatalf("Datei: %v", err)
	}
	go func() {
		time.Sleep(200 * time.Millisecond)
		_ = os.Remove(file)
		_ = writeExclusive(file, Record{Key: "/p", Addr: "a:2", PID: child.Process.Pid, StartTime: 5})
	}()

	probe := func(addr string) (Health, error) {
		return Health{Status: "ok", Key: "/p", PID: child.Process.Pid}, nil
	}
	record, err := child.Await("/p", file, 3*time.Second, probe)
	if err != nil {
		t.Fatalf("Fehler: %v", err)
	}
	if record.Addr != "a:2" || record.PID != child.Process.Pid {
		t.Errorf("Record = %+v", record)
	}
}

// Endet das Kind, bevor es antwortet, sagt Await das — und wartet nicht bis
// zur Zeitgrenze.
func TestAwaitMeldetVorzeitigesEnde(t *testing.T) {
	child := fakeChild(t, "false")
	begin := time.Now()

	_, err := child.Await("/p", filepath.Join(t.TempDir(), "r.json"), 5*time.Second, ProbeHealth)
	if !errors.Is(err, ErrChildExited) {
		t.Fatalf("Fehler = %v, erwartet ErrChildExited", err)
	}
	if time.Since(begin) > 2*time.Second {
		t.Error("Await hat bis zur Zeitgrenze gewartet")
	}
}

// Ohne Datei und Antwort läuft die Zeitgrenze ab.
func TestAwaitZeitgrenze(t *testing.T) {
	child := fakeChild(t, "sleep", "10")

	_, err := child.Await("/p", filepath.Join(t.TempDir(), "r.json"), 300*time.Millisecond, ProbeHealth)
	if !errors.Is(err, ErrStartupTimeout) {
		t.Fatalf("Fehler = %v, erwartet ErrStartupTimeout", err)
	}
}

// Die Servermarke gilt nur, wenn sie genau "1" ist.
func TestServeMode(t *testing.T) {
	t.Setenv(ServeEnv, "")
	if ServeMode() {
		t.Error("leer gilt als Servermodus")
	}
	t.Setenv(ServeEnv, "1")
	if !ServeMode() {
		t.Error("1 gilt nicht als Servermodus")
	}
}
