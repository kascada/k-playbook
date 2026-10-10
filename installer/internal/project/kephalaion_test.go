package project

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
)

// Der Block kephalaion steht in jeder Kontextausgabe und lässt sie nie
// scheitern — auch nicht bei unlesbarer Datei oder fehlendem Node.
func TestContextForDirTraegtKephalaion(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	root := newContextProject(t)

	built, err := ContextForDir(root)
	if err != nil {
		t.Fatalf("ContextForDir: %v", err)
	}
	if built.Kephalaion == nil || built.Kephalaion.State != kephalaion.StateNotConfigured || built.Kephalaion.Configured {
		t.Fatalf("ohne Eintrag: %+v", built.Kephalaion)
	}

	local := filepath.Join(LocalDir(root), kephalaion.LocalFileName)
	write(t, local, "kephalaion:\n  hub: vm\n  collection: k-playbook\n  account: kamran-wsl\n")
	built, err = ContextForDir(root)
	if err != nil {
		t.Fatalf("ContextForDir: %v", err)
	}
	if built.Kephalaion.State != kephalaion.StateNoKephalaion || built.Kephalaion.Address != "vm:k-playbook" {
		t.Fatalf("ohne Kephalaion: %+v", built.Kephalaion)
	}

	write(t, local, "kephalaion:\n  hub: [kaputt\n")
	built, err = ContextForDir(root)
	if err != nil {
		t.Fatalf("unlesbare Datei bricht den Kontext ab: %v", err)
	}
	if built.Kephalaion.State != kephalaion.StateInvalidEntry {
		t.Fatalf("unlesbar: %+v", built.Kephalaion)
	}
}

// BuildContext fragt den Node nicht; das Feld bleibt leer.
func TestBuildContextFragtKephalaionNicht(t *testing.T) {
	called := false
	original := kephalaionCheck
	kephalaionCheck = func(string) kephalaion.Status { called = true; return kephalaion.Status{} }
	t.Cleanup(func() { kephalaionCheck = original })

	built, err := BuildContext(newContextProject(t))
	if err != nil {
		t.Fatal(err)
	}
	if called || built.Kephalaion != nil {
		t.Fatal("BuildContext hat die Anbindung geprüft")
	}
}

// Eine Zeile in K-PLAYBOOK.local.yaml, die nur aus einem Token besteht,
// erscheint nicht in der Kontextausgabe — höchstens Zeilennummer und Art des
// Fehlers.
func TestContextForDirZitiertKeinenDateiinhalt(t *testing.T) {
	const token = "keph_CONTEXTLEAKCANARYabcdefghijklmnopqrstuvw1234"
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := newContextProject(t)
	write(t, filepath.Join(LocalDir(root), kephalaion.LocalFileName),
		"kephalaion:\n  hub: vm\n  collection: k-playbook\n"+token+"\n  account: kamran-wsl\n")

	built, err := ContextForDir(root)
	if err != nil {
		t.Fatalf("ContextForDir: %v", err)
	}
	data, err := json.Marshal(built)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "CONTEXTLEAKCANARY") {
		t.Fatalf("Dateiinhalt in der Kontextausgabe: %s", built.Kephalaion.Message)
	}
	if built.Kephalaion.State != kephalaion.StateInvalidEntry || !strings.Contains(built.Kephalaion.Message, "Zeile 4") {
		t.Fatalf("Zustand: %+v", built.Kephalaion)
	}
}
