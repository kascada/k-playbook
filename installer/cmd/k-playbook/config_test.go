package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/project"
)

func TestRunConfigCreateLegtAnkerImAktuellenVerzeichnisAn(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf("Repository anlegen: %v", err)
	}

	before, err := os.Getwd()
	if err != nil {
		t.Fatalf("Arbeitsverzeichnis lesen: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("ins Testprojekt wechseln: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(before) })

	if err := runConfig([]string{"create"}); err != nil {
		t.Fatalf("runConfig: %v", err)
	}

	content, err := os.ReadFile(project.ConfigPath(root))
	if err != nil {
		t.Fatalf("Konfiguration lesen: %v", err)
	}
	for _, want := range []string{"schema_version: 3", "repo_root: .", "vcs: git"} {
		if !strings.Contains(string(content), want) {
			t.Errorf("Konfiguration enthält %q nicht:\n%s", want, content)
		}
	}
}

// Eine Datei am früheren Ort direkt im Hauptverzeichnis zählt als vorhandener
// Anker: Sie zieht um und bleibt unverändert.
func TestRunConfigCreateLehntVorhandenenAnkerAb(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, project.ConfigFileName), []byte("eigenständig\n"), 0o644); err != nil {
		t.Fatalf("Konfiguration anlegen: %v", err)
	}

	if err := runConfig([]string{"create", root}); err == nil {
		t.Fatal("runConfig hat einen vorhandenen Anker nicht abgelehnt")
	}
	content, err := os.ReadFile(project.ConfigPath(root))
	if err != nil {
		t.Fatalf("Konfiguration lesen: %v", err)
	}
	if string(content) != "eigenständig\n" {
		t.Errorf("vorhandene Konfiguration wurde verändert: %q", content)
	}
}
