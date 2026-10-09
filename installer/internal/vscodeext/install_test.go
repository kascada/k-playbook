package vscodeext

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// fakeCLI legt ein `#!/bin/sh` an, das seine Argumente in marker schreibt und
// mit exitCode endet. Gegen das echte `code` wird nie getestet: die
// Remote-CLI ignoriert --extensions-dir und schriebe in die laufende
// Umgebung.
func fakeCLI(t *testing.T, marker string, exitCode int, message string) CLI {
	t.Helper()
	path := filepath.Join(t.TempDir(), "code-server")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" > " + marker + "\n" +
		"printf '" + message + "\\n'\n" +
		"exit " + strconv.Itoa(exitCode) + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return CLI{Path: path, Kind: KindServer}
}

func TestInstallRuftCLIMitForce(t *testing.T) {
	erlaubeInstall(t)
	marker := filepath.Join(t.TempDir(), "argumente")
	cli := fakeCLI(t, marker, 0, "Extension was successfully installed.")

	output, err := Install(context.Background(), cli, []byte("VSIX-Inhalt"))
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if !strings.Contains(output, "successfully installed") {
		t.Errorf("Ausgabe = %q", output)
	}

	data, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("Marker lesen: %v", err)
	}
	argumente := strings.Fields(strings.TrimSpace(string(data)))
	if len(argumente) != 3 || argumente[0] != "--install-extension" || argumente[2] != "--force" {
		t.Fatalf("Argumente = %v", argumente)
	}
	// Die temporäre VSIX ist nach dem Lauf weg.
	if _, err := os.Stat(argumente[1]); !os.IsNotExist(err) {
		t.Errorf("%s liegt noch: %v", argumente[1], err)
	}
	if filepath.Ext(argumente[1]) != ".vsix" {
		t.Errorf("Dateiendung von %s", argumente[1])
	}
}

// Eine scheiternde CLI: der Fehler nennt den Aufruf, und ihre Ausgabe kommt
// mit zurück — sie ist das Einzige, was den Grund nennt.
func TestInstallScheiterndeCLI(t *testing.T) {
	erlaubeInstall(t)
	marker := filepath.Join(t.TempDir(), "argumente")
	cli := fakeCLI(t, marker, 1, "Unable to install extension")

	output, err := Install(context.Background(), cli, []byte("VSIX-Inhalt"))
	if err == nil {
		t.Fatal("Install ohne Fehler")
	}
	if !strings.Contains(err.Error(), "--install-extension") {
		t.Errorf("Fehler = %v", err)
	}
	if !strings.Contains(output, "Unable to install extension") {
		t.Errorf("Ausgabe = %q", output)
	}
}

func TestInstallOhneVSIX(t *testing.T) {
	cli := fakeCLI(t, filepath.Join(t.TempDir(), "argumente"), 0, "")

	if _, err := Install(context.Background(), cli, nil); err == nil ||
		!strings.Contains(err.Error(), "ohne die VS-Code-Erweiterung") {
		t.Errorf("Install = %v", err)
	}
}

// Ein abgebrochener Kontext bricht den Aufruf ab, statt die Frist abzuwarten.
func TestInstallAbgebrochenerKontext(t *testing.T) {
	erlaubeInstall(t)
	cli := fakeCLI(t, filepath.Join(t.TempDir(), "argumente"), 0, "")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := Install(ctx, cli, []byte("VSIX-Inhalt")); err == nil {
		t.Error("Install ohne Fehler")
	}
}
