package projects

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kascada/k-playbook/installer/internal/store"
)

func TestMinimalConfig(t *testing.T) {
	config := MinimalConfig(time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC))
	expected := `schema_version: 1
layout: fixed-project-k-playbook

k_playbook:
  repo: ~/dev/k-playbook

setup:
  updated_at: 2026-07-30
`

	if config != expected {
		t.Fatalf("unexpected minimal config:\n%s", config)
	}
}

func TestEnsureConfigCreatesMinimalConfig(t *testing.T) {
	root := t.TempDir()

	created, err := EnsureConfig(root)
	if err != nil {
		t.Fatalf("EnsureConfig failed: %v", err)
	}
	if !created {
		t.Fatal("expected config to be created")
	}

	data, err := os.ReadFile(filepath.Join(root, ConfigFileName))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	content := string(data)
	for _, expected := range []string{
		"schema_version: 1\n",
		"layout: fixed-project-k-playbook\n",
		"repo: ~/dev/k-playbook\n",
		"setup:\n",
		"updated_at: ",
	} {
		if !strings.Contains(content, expected) {
			t.Fatalf("config does not contain %q:\n%s", expected, content)
		}
	}

	created, err = EnsureConfig(root)
	if err != nil {
		t.Fatalf("second EnsureConfig failed: %v", err)
	}
	if created {
		t.Fatal("expected existing config to be left unchanged")
	}
}

func TestDetectEnvironmentMapsPlainAndDevContainer(t *testing.T) {
	plain := t.TempDir()
	if err := os.WriteFile(filepath.Join(plain, "go.mod"), []byte("module example\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.Mkdir(filepath.Join(plain, ".venv"), 0o755); err != nil {
		t.Fatalf("create .venv: %v", err)
	}

	environment, detected := DetectEnvironment(plain)
	if environment != store.EnvironmentPlain {
		t.Fatalf("expected plain, got %s", environment)
	}
	if len(detected) == 0 {
		t.Fatal("expected detected markers")
	}

	devcontainer := t.TempDir()
	if err := os.Mkdir(filepath.Join(devcontainer, ".devcontainer"), 0o755); err != nil {
		t.Fatalf("create .devcontainer: %v", err)
	}
	if err := os.WriteFile(filepath.Join(devcontainer, ".devcontainer", "devcontainer.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write devcontainer.json: %v", err)
	}

	environment, detected = DetectEnvironment(devcontainer)
	if environment != store.EnvironmentDevContainer {
		t.Fatalf("expected devcontainer, got %s", environment)
	}
	if len(detected) != 1 || detected[0] != ".devcontainer/devcontainer.json" {
		t.Fatalf("unexpected devcontainer markers: %#v", detected)
	}
}
