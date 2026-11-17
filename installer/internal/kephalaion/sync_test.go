package kephalaion

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSyncHub(t *testing.T) {
	fake := newFakeClient(t)
	fake.set(t, "Hub vm: 1 Seite\n  k-playbook-dev: abgeglichen, 0 Zeilen, Revision 7664\n", "", 0)
	if err := SyncHub(context.Background(), "vm"); err != nil {
		t.Fatalf("Abgleich: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(fake.dir, "args"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Fields(string(data)); strings.Join(got, " ") != "node sync vm" {
		t.Fatalf("Aufruf: %v", got)
	}
}

func TestSyncHubScheitert(t *testing.T) {
	fake := newFakeClient(t)
	var syncErr *SyncError

	// Grund auf stderr.
	fake.set(t, "", "Fehler: Hub nicht erreichbar\n", 1)
	err := SyncHub(context.Background(), "vm")
	if !errors.As(err, &syncErr) || !strings.Contains(err.Error(), "Exit 1: Fehler: Hub nicht erreichbar") ||
		!strings.Contains(err.Error(), "kephalaion node sync vm") {
		t.Fatalf("stderr: %v", err)
	}

	// Ohne stderr: die letzte Zeile von stdout.
	fake.set(t, "Hub vm: 1 Seite\n  k-playbook-dev: Fehler beim Abgleich\n", "", 1)
	err = SyncHub(context.Background(), "vm")
	if !errors.As(err, &syncErr) || !strings.Contains(err.Error(), "k-playbook-dev: Fehler beim Abgleich") {
		t.Fatalf("stdout: %v", err)
	}
}

func TestSyncHubOhneBinary(t *testing.T) {
	original := LookupBinary
	LookupBinary = func() (string, bool) { return "", false }
	t.Cleanup(func() { LookupBinary = original })
	err := SyncHub(context.Background(), "vm")
	var syncErr *SyncError
	if !errors.As(err, &syncErr) || !strings.Contains(err.Error(), "nicht installiert") || !strings.Contains(err.Error(), InstallCommand) {
		t.Fatalf("ohne Binary: %v", err)
	}
}

func TestSyncHubUngueltigerHub(t *testing.T) {
	fake := newFakeClient(t)
	err := SyncHub(context.Background(), "--config")
	var syncErr *SyncError
	if !errors.As(err, &syncErr) {
		t.Fatalf("ungültiger Hub: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(fake.dir, "args")); !os.IsNotExist(statErr) {
		t.Fatal("Binary trotz ungültigem Hub aufgerufen")
	}
}

func TestSyncHubZeitgrenze(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kephalaion")
	writeFile(t, path, "#!/bin/sh\nexec sleep 10\n")
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
	original, originalTimeout := LookupBinary, SyncTimeout
	LookupBinary = func() (string, bool) { return path, true }
	SyncTimeout = 200 * time.Millisecond
	t.Cleanup(func() { LookupBinary, SyncTimeout = original, originalTimeout })
	err := SyncHub(context.Background(), "vm")
	if err == nil || !strings.Contains(err.Error(), "nicht innerhalb von") {
		t.Fatalf("Zeitgrenze: %v", err)
	}
}
