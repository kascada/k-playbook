package projects

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const projectLocalConfig = `schema_version: 2
layout: project-local

k_playbook:
  dist: _dist
  version: 0.4.0

paths:
  tasks: tasks
  todo: TODO.md

project:
  repo_root: ..
  vcs: git
`

const legacyConfig = `schema_version: 1
layout: fixed-project-k-playbook

k_playbook:
  repo: ~/dev/k-playbook

paths:
  playbook: k-playbook
  tasks: k-playbook/tasks

project:
  repo_root: .
  vcs: git
`

// The legacy write paths must refuse a migrated project. Without this, using the
// GUI on a project that was already migrated would write a second config in the
// project root and recreate the v1 directory layout.
func TestLegacyWritesRefuseMigratedProject(t *testing.T) {
	root := t.TempDir()
	playbook := filepath.Join(root, "k-playbook")
	if err := os.MkdirAll(playbook, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(playbook, "K-PLAYBOOK.yaml"), []byte(projectLocalConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	cases := map[string]error{
		"EnsureConfig":             firstError(EnsureConfig(root, RemediationModeDirectAllowed)),
		"EnsureConfigDefaults":     firstError(EnsureConfigDefaults(root)),
		"UpdateRemediationMode":    UpdateRemediationMode(root, RemediationModeTaskFirst),
		"UpdateProjectRoot":        UpdateProjectRoot(root, ".", ProjectVCSGit),
		"CompleteProjectStructure": structureError(CompleteProjectStructure(root)),
	}
	for name, err := range cases {
		if err == nil {
			t.Errorf("%s hat ein migriertes Projekt nicht abgelehnt", name)
			continue
		}
		if !errors.Is(err, ErrProjectLocalLayout) {
			t.Errorf("%s: unerwarteter Fehler: %v", name, err)
		}
	}

	// Nothing may have been written into the project root.
	if _, err := os.Stat(filepath.Join(root, "K-PLAYBOOK.yaml")); err == nil {
		t.Error("es wurde eine zweite K-PLAYBOOK.yaml im Projekt-Root angelegt")
	}
	for _, dir := range []string{"tasks", "reviews", "docs"} {
		if _, err := os.Stat(filepath.Join(playbook, dir)); err == nil {
			t.Errorf("k-playbook/%s wurde vom Legacy-Pfad angelegt", dir)
		}
	}
}

// A genuine v1 project must still be writable, otherwise the guard would block
// the very projects the GUI is there for.
func TestLegacyWritesAllowLegacyProject(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "K-PLAYBOOK.yaml"), []byte(legacyConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := UpdateRemediationMode(root, RemediationModeTaskFirst); err != nil {
		t.Errorf("UpdateRemediationMode auf einem v1-Projekt: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "K-PLAYBOOK.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "mode: task-first") {
		t.Error("Remediation-Modus wurde nicht geschrieben")
	}
}

func TestIsProjectLocalConfig(t *testing.T) {
	cases := map[string]bool{
		projectLocalConfig:                   true,
		legacyConfig:                         false,
		"layout: project-local\n":            true,
		"schema_version: 3\n":                true,
		"schema_version: 1\nlayout: x\n":     false,
		"":                                   false,
		"# schema_version: 2 im Kommentar\n": false,
	}
	for text, want := range cases {
		if got := isProjectLocalConfig(text); got != want {
			t.Errorf("isProjectLocalConfig(%q) = %v, want %v", configHead(text), got, want)
		}
	}
}

func firstError(_ bool, err error) error { return err }

func structureError(_ StructureStatus, err error) error { return err }

func configHead(text string) string {
	if index := strings.IndexByte(text, '\n'); index >= 0 {
		return text[:index]
	}
	return text
}
