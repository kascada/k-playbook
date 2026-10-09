package vscodeext

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// serverBinary legt ~/.vscode-server/bin/<commit>/<rel> als ausführbare Datei
// an und setzt die Änderungszeit des Commit-Verzeichnisses — danach wählt
// ChooseCLI.
func serverBinary(t *testing.T, home string, commit string, rel string, age time.Duration) string {
	t.Helper()
	path := filepath.Join(home, ".vscode-server", "bin", commit, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	when := time.Now().Add(-age)
	if err := os.Chtimes(filepath.Join(home, ".vscode-server", "bin", commit), when, when); err != nil {
		t.Fatal(err)
	}
	return path
}

// nichtGefunden ist ein LookPath, das nichts findet.
func nichtGefunden(string) (string, error) { return "", errors.New("nicht im PATH") }

func umgebung(paare map[string]string) func(string) string {
	return func(key string) string { return paare[key] }
}

func TestChooseCLIOhneAlles(t *testing.T) {
	_, err := ChooseCLI(t.TempDir(), umgebung(nil), nichtGefunden)
	if !errors.Is(err, ErrNoCLI) {
		t.Errorf("ChooseCLI = %v, erwartet ErrNoCLI", err)
	}
}

// Mit lebendem Socket gewinnt die Remote-CLI, auch wenn code-server daneben
// liegt.
func TestChooseCLIRemoteBeiLebendemSocket(t *testing.T) {
	home := t.TempDir()
	remote := serverBinary(t, home, "aaa", filepath.Join("bin", "remote-cli", "code"), 0)
	serverBinary(t, home, "aaa", filepath.Join("bin", "code-server"), 0)
	socket := filepath.Join(home, "ipc.sock")
	if err := os.WriteFile(socket, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	cli, err := ChooseCLI(home, umgebung(map[string]string{"VSCODE_IPC_HOOK_CLI": socket}), nichtGefunden)
	if err != nil {
		t.Fatalf("ChooseCLI: %v", err)
	}
	if cli.Kind != KindRemote || cli.Path != remote {
		t.Errorf("ChooseCLI = %+v, erwartet %s", cli, remote)
	}
}

// Ein Socket, der nicht mehr da ist, zählt nicht: genau der Fall eines
// Dienstes, der aus einem geschlossenen VS-Code-Terminal gestartet wurde.
func TestChooseCLIVeralteterSocketFaelltAufCodeServer(t *testing.T) {
	home := t.TempDir()
	serverBinary(t, home, "aaa", filepath.Join("bin", "remote-cli", "code"), 0)
	server := serverBinary(t, home, "aaa", filepath.Join("bin", "code-server"), 0)

	cli, err := ChooseCLI(home, umgebung(map[string]string{
		"VSCODE_IPC_HOOK_CLI": filepath.Join(home, "gibt-es-nicht.sock"),
	}), nichtGefunden)
	if err != nil {
		t.Fatalf("ChooseCLI: %v", err)
	}
	if cli.Kind != KindServer || cli.Path != server {
		t.Errorf("ChooseCLI = %+v, erwartet %s", cli, server)
	}
}

// Liegen mehrere Server-Fassungen nebeneinander, gilt die jüngste.
func TestChooseCLINeuesterServer(t *testing.T) {
	home := t.TempDir()
	serverBinary(t, home, "alt", filepath.Join("bin", "code-server"), 48*time.Hour)
	neu := serverBinary(t, home, "neu", filepath.Join("bin", "code-server"), time.Minute)

	cli, err := ChooseCLI(home, umgebung(nil), nichtGefunden)
	if err != nil {
		t.Fatalf("ChooseCLI: %v", err)
	}
	if cli.Path != neu {
		t.Errorf("ChooseCLI = %+v, erwartet %s", cli, neu)
	}
}

// Ohne Server bleibt der Desktop-`code` aus dem PATH.
func TestChooseCLIDesktop(t *testing.T) {
	home := t.TempDir()
	desktop := filepath.Join(home, "bin", "code")

	cli, err := ChooseCLI(home, umgebung(nil), func(string) (string, error) { return desktop, nil })
	if err != nil {
		t.Fatalf("ChooseCLI: %v", err)
	}
	if cli.Kind != KindDesktop || cli.Path != desktop {
		t.Errorf("ChooseCLI = %+v", cli)
	}
}

// Vorsichtsregel: ein `code` unterhalb von /mnt/ wird nicht genommen. Nicht,
// weil er nachweislich falsch installierte — er leitet gemessen an die
// WSL-Seite weiter —, sondern weil die Weiterleitung an eine windows-seitig
// installierte Erweiterung hängt und nebenbei einen Server nachlädt.
func TestChooseCLIKeinCodeUnterMnt(t *testing.T) {
	for _, path := range []string{
		"/mnt/c/Users/x/AppData/Local/Programs/Microsoft VS Code/bin/code",
		"/mnt",
		"/mnt/c/../c/code",
	} {
		_, err := ChooseCLI(t.TempDir(), umgebung(nil), func(string) (string, error) { return path, nil })
		if !errors.Is(err, ErrNoCLI) {
			t.Errorf("%s: ChooseCLI = %v, erwartet ErrNoCLI", path, err)
		}
	}
}
