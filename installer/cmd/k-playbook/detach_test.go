package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/kascada/k-playbook/installer/internal/guiproc"
)

// Der abgekoppelte Start gegen einen echten Prozess: das Test-Binary startet
// sich selbst im Servermodus (siehe TestMain), der Aufruf wartet auf
// Laufzeitdatei und /api/health, und stop beendet den Server wieder.
func TestSpawnServerUndStop(t *testing.T) {
	stopEnvironment(t)

	var out bytes.Buffer
	record, err := spawnServer(&out)
	if err != nil {
		t.Fatalf("spawnServer: %v", err)
	}
	t.Cleanup(func() { _ = guiproc.Terminate(record.PID) })

	key, err := guiproc.Key()
	if err != nil {
		t.Fatalf("Schlüssel: %v", err)
	}
	if record.Key != key {
		t.Errorf("Schlüssel der Datei = %q, erwartet %q", record.Key, key)
	}
	if !guiproc.IdentityMatches(record.PID, time.Unix(record.StartTime, 0)) {
		t.Error("die Startzeit in der Datei passt nicht zum Prozess")
	}
	health, err := guiproc.ProbeHealth(record.Addr)
	if err != nil {
		t.Fatalf("/api/health: %v", err)
	}
	if health.Key != key || health.PID != record.PID {
		t.Errorf("Health = %+v", health)
	}

	// Ein zweiter Blick findet ihn als laufend, nicht als etwas Neues.
	finding, err := guiproc.Inspect(key, guiproc.OwnVersion(), guiproc.DefaultInspector())
	if err != nil || finding.Status != guiproc.StatusRunning {
		t.Errorf("Inspect = %s, %v; erwartet %s", finding.Status, err, guiproc.StatusRunning)
	}

	out.Reset()
	if err := runStop(&out); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if !strings.Contains(out.String(), "Server beendet") {
		t.Errorf("stop-Ausgabe: %q", out.String())
	}
	if !guiproc.WaitForExit(finding.Path, record.PID, 5*time.Second) {
		t.Error("der Server lebt nach stop weiter")
	}
	if _, ok, _ := guiproc.Read(finding.Path); ok {
		t.Error("die Laufzeitdatei liegt nach stop noch")
	}
}
