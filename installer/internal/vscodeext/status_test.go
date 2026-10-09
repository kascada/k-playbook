package vscodeext

import (
	"errors"
	"path/filepath"
	"testing"
)

// Zwei Installationen in verschiedenen Fassungen: die Frage „nachzuziehen?"
// bezieht sich auf das Verzeichnis, das die gewählte CLI beschreibt — und die
// zweite, abweichende Fassung wird nicht weggerechnet, sondern genannt.
func TestStatusNenntZielUndAbweichendeZweitfassung(t *testing.T) {
	home := t.TempDir()
	version := eingebettet(t)
	server := filepath.Join(home, ".vscode-server", "extensions")
	lokal := filepath.Join(home, ".vscode", "extensions")
	schreibe(t, filepath.Join(server, "extensions.json"), manifest(eintrag(ID, version)))
	schreibe(t, filepath.Join(lokal, "extensions.json"), manifest(eintrag(ID, "0.0.9")))
	codeServer(t, home, 0, "sollte nicht aufgerufen werden")

	keinPATH := func(string) (string, error) { return "", errors.New("nicht im PATH") }
	status := StatusFor(home, func(string) string { return "" }, keinPATH)

	if status.CLI == nil || status.CLI.Kind != KindServer {
		t.Fatalf("CLI = %+v", status.CLI)
	}
	if status.Target != server {
		t.Errorf("Target = %q, erwartet %q", status.Target, server)
	}
	if status.NeedsInstall {
		t.Error("das Verzeichnis der CLI trägt die passende Fassung, es ist nichts nachzuziehen")
	}
	if len(status.Divergent) != 1 || status.Divergent[0].Dir != lokal {
		t.Fatalf("Divergent = %+v, erwartet %s", status.Divergent, lokal)
	}
	if status.Divergent[0].Version != "0.0.9" {
		t.Errorf("Divergent-Fassung = %q", status.Divergent[0].Version)
	}
}

// Ohne CLI bleibt die Frage „irgendwo": von Hand über die VSIX installiert
// der Benutzer in ein Verzeichnis, das hier niemand kennt.
func TestStatusOhneCLIFragtIrgendwo(t *testing.T) {
	home := t.TempDir()
	schreibe(t, filepath.Join(home, ".vscode", "extensions", "extensions.json"),
		manifest(eintrag(ID, "0.0.9")))

	keinPATH := func(string) (string, error) { return "", errors.New("nicht im PATH") }
	status := StatusFor(home, func(string) string { return "" }, keinPATH)

	if status.CLI != nil || status.Target != "" {
		t.Fatalf("CLI = %+v, Target = %q", status.CLI, status.Target)
	}
	if !status.NeedsInstall {
		t.Error("eine abweichende Fassung gilt ohne CLI nicht als nachzuziehen")
	}
	if len(status.Divergent) != 0 {
		t.Errorf("Divergent = %+v, ohne Ziel gibt es kein Daneben", status.Divergent)
	}
}
