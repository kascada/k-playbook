package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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

	content, err := os.ReadFile(filepath.Join(root, "K-PLAYBOOK.yaml"))
	if err != nil {
		t.Fatalf("Konfiguration lesen: %v", err)
	}
	for _, want := range []string{"schema_version: 3", "repo_root: .", "vcs: git"} {
		if !strings.Contains(string(content), want) {
			t.Errorf("Konfiguration enthält %q nicht:\n%s", want, content)
		}
	}
}

func TestRunConfigCreateLehntVorhandenenAnkerAb(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "K-PLAYBOOK.yaml")
	if err := os.WriteFile(path, []byte("eigenständig\n"), 0o644); err != nil {
		t.Fatalf("Konfiguration anlegen: %v", err)
	}

	if err := runConfig([]string{"create", root}); err == nil {
		t.Fatal("runConfig hat einen vorhandenen Anker nicht abgelehnt")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Konfiguration lesen: %v", err)
	}
	if string(content) != "eigenständig\n" {
		t.Errorf("vorhandene Konfiguration wurde verändert: %q", content)
	}
}
