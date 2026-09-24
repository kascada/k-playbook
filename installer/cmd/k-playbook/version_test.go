package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/buildinfo"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// `k-playbook version` gibt nur die Version aus: eine Zeile, nichts davor und
// nichts danach. Die Oberfläche liest sie an der Datei am Installationsziel.
func TestVersionGibtNurDieVersionAus(t *testing.T) {
	before := buildinfo.Version
	t.Cleanup(func() { buildinfo.Version = before })

	buildinfo.Version = "v0.9.4"
	var out bytes.Buffer
	if err := runVersion(&out); err != nil {
		t.Fatalf("Fehler: %v", err)
	}
	if out.String() != "v0.9.4\n" {
		t.Errorf("Ausgabe = %q, erwartet genau die Version", out.String())
	}

	// Ohne gestempelte Version keine erfundene Angabe, sondern ein Fehler.
	buildinfo.Version = ""
	out.Reset()
	if err := runVersion(&out); err == nil {
		t.Error("ohne Version kein Fehler")
	}
	if out.Len() != 0 {
		t.Errorf("ohne Version steht etwas auf stdout: %q", out.String())
	}
}

// Startet `k-playbook` aus einem Programm, das älter ist als der Clone, steht
// genau eine Zeile im Terminal. Gleich, neuer und unbekannt bleiben still.
func TestStartAusAelteremProgrammMeldetEineZeile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, project.ConfigFileName), []byte("schema_version: 3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, project.PlaybookDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, project.PlaybookDirName, project.VersionFileName), []byte("v0.9.4\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(before) })

	var out bytes.Buffer
	noteOutdatedProgram(&out, "v0.9.3")
	line := out.String()
	if strings.Count(line, "\n") != 1 {
		t.Errorf("nicht genau eine Zeile: %q", line)
	}
	for _, want := range []string{"v0.9.3", "v0.9.4", "Programm aktualisieren", project.BootstrapCommand} {
		if !strings.Contains(line, want) {
			t.Errorf("die Zeile nennt %q nicht: %q", want, line)
		}
	}

	for _, running := range []string{"v0.9.4", "v0.10.0", "", "dev"} {
		out.Reset()
		noteOutdatedProgram(&out, running)
		if out.Len() != 0 {
			t.Errorf("laufend %q: %q", running, out.String())
		}
	}
}
