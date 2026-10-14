package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// readProject legt ein Projekt mit beiden Ebenen unter instructions/ an und
// macht es zum Arbeitsverzeichnis.
func readProject(t *testing.T) string {
	t.Helper()

	root := todoProject(t)
	files := map[string]string{
		"k-playbook/instructions/always/010-a.md":        "---\ntitle: A\n---\n\n## A\n",
		"k-playbook/instructions/always/100-b.md":        "## B mitgeliefert\n",
		"k-playbook/instructions/always/200-c.md":        "## C\n",
		"k-playbook/instructions/on-demand/thema.md":     "---\ntitle: Thema\ndescription: D\n---\n\n## Thema\n",
		"k-playbook-local/instructions/always/100-b.md":  "## B lokal\n\nText.\n",
		"k-playbook-local/instructions/always/200-c.md":  "# Abgeschaltet\n",
		"k-playbook-local/instructions/always/notiz.md":  "## Notiz\n\nText.\n",
		"k-playbook-local/instructions/on-demand/aus.md": "",
		"k-playbook/instructions/on-demand/aus.md":       "## Aus\n",
	}
	for rel, content := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("%s anlegen: %v", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("%s schreiben: %v", rel, err)
		}
	}
	return root
}

// runReadCaptured führt das Subkommando aus und fängt stdout und stderr ab.
func runReadCaptured(t *testing.T, args ...string) (string, string, error) {
	t.Helper()

	dir := t.TempDir()
	stdoutFile, err := os.Create(filepath.Join(dir, "stdout"))
	if err != nil {
		t.Fatalf("Ausgabedatei: %v", err)
	}
	stderrFile, err := os.Create(filepath.Join(dir, "stderr"))
	if err != nil {
		t.Fatalf("Fehlerdatei: %v", err)
	}
	beforeOut, beforeErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = stdoutFile, stderrFile
	runErr := runRead(args)
	os.Stdout, os.Stderr = beforeOut, beforeErr
	stdoutFile.Close()
	stderrFile.Close()

	stdout, err := os.ReadFile(stdoutFile.Name())
	if err != nil {
		t.Fatalf("Ausgabe lesen: %v", err)
	}
	stderr, err := os.ReadFile(stderrFile.Name())
	if err != nil {
		t.Fatalf("Fehlerausgabe lesen: %v", err)
	}
	return string(stdout), string(stderr), runErr
}

func TestReadBautVerzeichnisZusammen(t *testing.T) {
	readProject(t)

	stdout, stderr, err := runReadCaptured(t, "k-playbook:/instructions/always/")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if stdout != "## A\n\n## B lokal\n\nText.\n" {
		t.Errorf("stdout: %q", stdout)
	}
	if !strings.Contains(stderr, "notiz.md") {
		t.Errorf("Datei ohne Ziffer nicht auf stderr gemeldet: %q", stderr)
	}
}

func TestReadAbgeschalteteDateiIstKeinFehler(t *testing.T) {
	readProject(t)

	stdout, stderr, err := runReadCaptured(t, "k-playbook:/instructions/on-demand/aus.md")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if stdout != "" {
		t.Errorf("stdout bei abgeschalteter Datei: %q", stdout)
	}
	if !strings.Contains(stderr, "abgeschaltet") {
		t.Errorf("Hinweis fehlt auf stderr: %q", stderr)
	}
}

func TestReadJSON(t *testing.T) {
	readProject(t)

	stdout, _, err := runReadCaptured(t, "k-playbook:/instructions/on-demand/thema.md", "--json")
	if err != nil {
		t.Fatalf("read --json: %v", err)
	}
	var result project.DocResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("kein JSON: %v — %s", err, stdout)
	}
	if result.Content != "---\ntitle: Thema\ndescription: D\n---\n\n## Thema\n" || result.Disabled || result.Directory {
		t.Errorf("Antwort: %+v", result)
	}
	if len(result.Parts) != 1 || result.Parts[0].Origin != "dist" {
		t.Errorf("parts: %+v", result.Parts)
	}
}

func TestReadUnbekannterVerweisIstFehler(t *testing.T) {
	readProject(t)

	_, _, err := runReadCaptured(t, "k-playbook:/instructions/on-demand/fehlt.md")
	if err == nil || !strings.Contains(err.Error(), "not_found") {
		t.Fatalf("unbekannter Verweis: %v", err)
	}
	// Datei wie ein Verzeichnis behandelt: unbekannt, kein Systemfehler.
	_, _, err = runReadCaptured(t, "k-playbook:/instructions/always/010-a.md/x.md")
	if err == nil || !strings.Contains(err.Error(), "not_found") || strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("…/010-a.md/x.md: %v", err)
	}
	// Mit „/" am Ende fehlt der Dateiname — wie bei einem Verzeichnis, das
	// nicht zusammengebaut wird.
	for _, ref := range []string{
		"k-playbook:/instructions/always/010-a.md/",
		"k-playbook:/instructions/always/100-b.md/",
		"k-playbook:/instructions/on-demand/",
	} {
		_, _, err = runReadCaptured(t, ref)
		if err == nil || !strings.Contains(err.Error(), "(invalid_input)") || !strings.Contains(err.Error(), "Dateiname fehlt") {
			t.Fatalf("%s: %v", ref, err)
		}
	}
	_, _, err = runReadCaptured(t, "keph:/manual/abc.md")
	if err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("keph: %v", err)
	}
}

func TestReadHilfeIstFolgenlos(t *testing.T) {
	for _, args := range [][]string{{}, {"--help"}} {
		stdout, _, err := runReadCaptured(t, args...)
		if err != nil || !strings.Contains(stdout, "k-playbook read <verweis>") {
			t.Errorf("read %v: %v — %q", args, err, stdout)
		}
	}
}
