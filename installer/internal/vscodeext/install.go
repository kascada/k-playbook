package vscodeext

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// InstallTimeout begrenzt einen Aufruf der CLI. Gemessen dauert der Weg über
// code-server rund eine Sekunde; die Frist ist nur gegen eine CLI, die auf
// etwas wartet, das nie kommt.
const InstallTimeout = 2 * time.Minute

// NoVSIX ist die Meldung eines Binarys ohne eingebettete Erweiterung.
const NoVSIX = "dieses Programm ist ohne die VS-Code-Erweiterung gebaut"

// ViaSudo sagt, ob dieser Aufruf als root über sudo läuft. root ohne sudo —
// der Normalfall in einem Container — ist der Benutzer, der VS Code
// verwendet, und darf installieren.
func ViaSudo() bool {
	return os.Geteuid() == 0 && os.Getenv("SUDO_USER") != ""
}

// ErrSudo heißt: über sudo wird nicht installiert. Die Erweiterung gehört
// dem Benutzer, der den Editor bedient; unter root entstünden Dateien, die er
// nicht mehr ändern kann.
var ErrSudo = errors.New("nicht über sudo: die Erweiterung gehört dem Benutzer, der VS Code bedient")

// Install schreibt die VSIX in eine temporäre Datei und ruft
// `<cli> --install-extension <datei> --force`. Zurück kommt die Ausgabe der
// CLI — sie ist im Fehlerfall das Einzige, was den Grund nennt, und gehört
// deshalb ins Log und in die Statusanzeige.
//
// --force auch über eine höhere Fassung: maßgeblich ist, was das Programm
// mitbringt, nicht was zufällig schon liegt.
func Install(ctx context.Context, cli CLI, data []byte) (output string, err error) {
	if len(data) == 0 {
		return "", errors.New(NoVSIX)
	}
	// Die Abdichtung sitzt hier, an der einzigen Stelle, die wirklich eine
	// CLI ausführt: so greift sie für den Nachzug des Dienstes, für
	// `k-playbook vscode install` und für jeden künftigen Aufrufer gleich.
	if installBlocked() {
		return "", ErrNoInstall
	}
	if ViaSudo() {
		return "", ErrSudo
	}

	file, err := os.CreateTemp("", "k-playbook-workspace-tools-*.vsix")
	if err != nil {
		return "", fmt.Errorf("temporäre VSIX anlegen: %w", err)
	}
	defer func() { _ = os.Remove(file.Name()) }()
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return "", fmt.Errorf("temporäre VSIX schreiben: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("temporäre VSIX schließen: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, InstallTimeout)
	defer cancel()

	var collected bytes.Buffer
	command := exec.CommandContext(ctx, cli.Path, "--install-extension", file.Name(), "--force")
	command.Stdout = &collected
	command.Stderr = &collected
	// Ohne stdin: die CLI darf unter keinen Umständen auf eine Eingabe
	// warten. Der Dienst hat kein Terminal, an dem jemand antworten könnte.
	command.Stdin = nil

	runErr := command.Run()
	output = strings.TrimSpace(collected.String())
	if runErr != nil {
		if ctx.Err() != nil {
			return output, fmt.Errorf("%s antwortet nicht, nach %s abgebrochen", cli.Path, InstallTimeout)
		}
		return output, fmt.Errorf("%s --install-extension ist gescheitert: %w", cli.Path, runErr)
	}
	return output, nil
}
