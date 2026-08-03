package webui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/projects"
)

func TestCurrentProjectRootFindsNearestPlaybookConfig(t *testing.T) {
	root := t.TempDir()
	projectRoot := filepath.Join(root, "project")
	workdir := filepath.Join(projectRoot, "app")
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		t.Fatalf("create workdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, projects.ConfigFileName), []byte("schema_version: 1\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	got := currentProjectRoot(workdir, filepath.Join(root, "k-playbook"))
	if got != projectRoot {
		t.Fatalf("expected %s, got %s", projectRoot, got)
	}
}

func TestCanEditProjectInContainerOnlyAllowsCurrentProject(t *testing.T) {
	root := t.TempDir()
	current := filepath.Join(root, "current")
	other := filepath.Join(root, "other")

	runtime := runtimeStatus{InsideContainer: true, CurrentProject: current}
	if !canEditProject(runtime, current) {
		t.Fatalf("expected current project to be editable")
	}
	if canEditProject(runtime, other) {
		t.Fatalf("expected other project to be read-only in container runtime")
	}
}
