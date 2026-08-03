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
	config := MinimalConfig(time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC), RemediationModeDirectAllowed)
	expected := `schema_version: 1
layout: fixed-project-k-playbook

k_playbook:
  repo: ~/dev/k-playbook

project:
  repo_root: .
  vcs: git

setup:
  updated_at: 2026-07-30

remediation:
  mode: direct-allowed
  target: .
  grouping: true
  quick_wins: true
  branch_prefix: remediation/
  pr_required: false
  direct_fixes: true
`

	if config != expected {
		t.Fatalf("unexpected minimal config:\n%s", config)
	}
}

func TestMinimalConfigTaskBranchPR(t *testing.T) {
	config := MinimalConfig(time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC), RemediationModeTaskBranchPR)
	for _, expected := range []string{
		"mode: task-branch-pr\n",
		"pr_required: true\n",
		"direct_fixes: false\n",
	} {
		if !strings.Contains(config, expected) {
			t.Fatalf("config does not contain %q:\n%s", expected, config)
		}
	}
}

func TestEnsureConfigCreatesMinimalConfig(t *testing.T) {
	root := t.TempDir()
	legacyPath := filepath.Join(root, LegacyConfigMarkdownFileName)
	if err := os.WriteFile(legacyPath, []byte("# Legacy\n"), 0o644); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}

	created, err := EnsureConfig(root, RemediationModeDirectAllowed)
	if err != nil {
		t.Fatalf("EnsureConfig failed: %v", err)
	}
	if !created {
		t.Fatal("expected config to be created")
	}
	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Fatalf("expected legacy config to be removed, stat err: %v", err)
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
		"project:\n",
		"repo_root: ",
		"vcs: ",
		"setup:\n",
		"updated_at: ",
		"remediation:\n",
		"mode: direct-allowed\n",
		"direct_fixes: true\n",
	} {
		if !strings.Contains(content, expected) {
			t.Fatalf("config does not contain %q:\n%s", expected, content)
		}
	}

	created, err = EnsureConfig(root, RemediationModeTaskBranchPR)
	if err != nil {
		t.Fatalf("second EnsureConfig failed: %v", err)
	}
	if created {
		t.Fatal("expected existing config to be left unchanged")
	}
}

func TestEnsureConfigRemovesLegacyMarkdownWhenConfigExists(t *testing.T) {
	root := t.TempDir()
	if _, err := EnsureConfig(root, RemediationModeDirectAllowed); err != nil {
		t.Fatalf("EnsureConfig failed: %v", err)
	}
	legacyPath := filepath.Join(root, LegacyConfigMarkdownFileName)
	if err := os.WriteFile(legacyPath, []byte("# Legacy\n"), 0o644); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}

	created, err := EnsureConfig(root, RemediationModeTaskBranchPR)
	if err != nil {
		t.Fatalf("second EnsureConfig failed: %v", err)
	}
	if created {
		t.Fatal("expected existing config to be left unchanged")
	}
	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Fatalf("expected legacy config to be removed, stat err: %v", err)
	}
}

func TestMinimalConfigForProjectSupportsNoGit(t *testing.T) {
	config := MinimalConfigForProject(
		time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC),
		RemediationModeDirectAllowed,
		ProjectRootConfig{RepoRoot: ".", VCS: string(ProjectVCSNone)},
	)
	for _, expected := range []string{
		"project:\n",
		"repo_root: .\n",
		"vcs: none\n",
	} {
		if !strings.Contains(config, expected) {
			t.Fatalf("config does not contain %q:\n%s", expected, config)
		}
	}
}

func TestUpdateProjectRootWritesNoGitDecision(t *testing.T) {
	root := t.TempDir()
	if _, err := EnsureConfig(root, RemediationModeDirectAllowed); err != nil {
		t.Fatalf("EnsureConfig failed: %v", err)
	}

	if err := UpdateProjectRoot(root, ".", ProjectVCSNone); err != nil {
		t.Fatalf("UpdateProjectRoot failed: %v", err)
	}
	contentBytes, err := os.ReadFile(filepath.Join(root, ConfigFileName))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	content := string(contentBytes)
	for _, expected := range []string{
		"project:\n",
		"repo_root: .\n",
		"vcs: none\n",
	} {
		if !strings.Contains(content, expected) {
			t.Fatalf("config does not contain %q:\n%s", expected, content)
		}
	}

	status := CheckProjectRoot(root)
	if !status.OK || status.VCS != string(ProjectVCSNone) || status.RepoRoot != "." {
		t.Fatalf("unexpected project root status: %#v", status)
	}
}

func TestSmokeSkipsGHWhenProjectHasNoGit(t *testing.T) {
	root := t.TempDir()
	if _, err := EnsureConfig(root, RemediationModeDirectAllowed); err != nil {
		t.Fatalf("EnsureConfig failed: %v", err)
	}
	if err := UpdateProjectRoot(root, ".", ProjectVCSNone); err != nil {
		t.Fatalf("UpdateProjectRoot failed: %v", err)
	}

	result, err := Smoke(root)
	if err != nil {
		t.Fatalf("Smoke failed: %v", err)
	}
	if !result.OK {
		t.Fatalf("expected smoke OK for no-git project: %#v", result)
	}
	if len(result.Checks) < 2 {
		t.Fatalf("expected config and gh checks: %#v", result.Checks)
	}
	if result.Checks[1].Name != "gh" || !strings.Contains(result.Checks[1].Message, "uebersprungen") {
		t.Fatalf("expected gh skip check, got %#v", result.Checks[1])
	}
}

func TestSmokeReportsConfigErrorWhenRepoRootMissing(t *testing.T) {
	root := t.TempDir()
	if _, err := EnsureConfig(root, RemediationModeDirectAllowed); err != nil {
		t.Fatalf("EnsureConfig failed: %v", err)
	}

	result, err := Smoke(root)
	if err != nil {
		t.Fatalf("Smoke failed: %v", err)
	}
	if result.OK {
		t.Fatalf("expected smoke warning/error for missing repo root: %#v", result)
	}
	if result.Checks[0].Name != "config" || result.Checks[0].Severity != SmokeSeverityError {
		t.Fatalf("expected config error, got %#v", result.Checks[0])
	}
}

func TestUpdateRemediationModeReplacesExistingBlock(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ConfigFileName)
	initial := `schema_version: 1
layout: fixed-project-k-playbook

k_playbook:
  repo: ~/dev/k-playbook

setup:
  updated_at: 2026-07-30

remediation:
  mode: direct-allowed
  target: .
  grouping: true
  quick_wins: true
  branch_prefix: remediation/
  pr_required: false
  direct_fixes: true

custom:
  keep: true
`
	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if err := UpdateRemediationMode(root, RemediationModeTaskBranchPR); err != nil {
		t.Fatalf("UpdateRemediationMode failed: %v", err)
	}
	contentBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	content := string(contentBytes)
	for _, expected := range []string{
		"mode: task-branch-pr\n",
		"pr_required: true\n",
		"direct_fixes: false\n",
		"custom:\n",
		"keep: true\n",
	} {
		if !strings.Contains(content, expected) {
			t.Fatalf("config does not contain %q:\n%s", expected, content)
		}
	}
}

func TestEnsureConfigDefaultsAddsMissingRemediationBlock(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ConfigFileName)
	legacyPath := filepath.Join(root, LegacyConfigMarkdownFileName)
	initial := `schema_version: 1
layout: fixed-project-k-playbook

k_playbook:
  repo: ~/dev/k-playbook

setup:
  updated_at: 2026-07-30

custom:
  keep: true
`
	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(legacyPath, []byte("# Legacy\n"), 0o644); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}

	changed, err := EnsureConfigDefaults(root)
	if err != nil {
		t.Fatalf("EnsureConfigDefaults failed: %v", err)
	}
	if !changed {
		t.Fatal("expected defaults to be added")
	}
	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Fatalf("expected legacy config to be removed, stat err: %v", err)
	}
	contentBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	content := string(contentBytes)
	for _, expected := range []string{
		"remediation:\n",
		"mode: direct-allowed\n",
		"direct_fixes: true\n",
		"custom:\n",
		"keep: true\n",
	} {
		if !strings.Contains(content, expected) {
			t.Fatalf("config does not contain %q:\n%s", expected, content)
		}
	}

	changed, err = EnsureConfigDefaults(root)
	if err != nil {
		t.Fatalf("second EnsureConfigDefaults failed: %v", err)
	}
	if changed {
		t.Fatal("expected second defaults call to be unchanged")
	}
}

func TestReadRemediationMode(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ConfigFileName), []byte(MinimalConfig(time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC), RemediationModeTaskFirst)), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	mode, found, err := ReadRemediationMode(root)
	if err != nil {
		t.Fatalf("ReadRemediationMode failed: %v", err)
	}
	if !found {
		t.Fatal("expected remediation mode to be found")
	}
	if mode != RemediationModeTaskFirst {
		t.Fatalf("expected task-first, got %s", mode)
	}
}

func TestCompleteProjectStructure(t *testing.T) {
	root := t.TempDir()

	status, err := CheckProjectStructure(root)
	if err != nil {
		t.Fatalf("CheckProjectStructure failed: %v", err)
	}
	if status.OK {
		t.Fatal("expected incomplete structure")
	}
	if len(status.Missing) == 0 {
		t.Fatal("expected missing paths")
	}

	status, err = CompleteProjectStructure(root)
	if err != nil {
		t.Fatalf("CompleteProjectStructure failed: %v", err)
	}
	if !status.OK {
		t.Fatalf("expected complete structure, missing: %#v", status.Missing)
	}

	for _, path := range []string{
		"k-playbook/tasks",
		"k-playbook/tasks/done",
		"k-playbook/checks",
		"k-playbook/reviews",
		"k-playbook/guidelines",
		"k-playbook/enforcement",
		"k-playbook/docs",
	} {
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatalf("expected %s: %v", path, err)
		}
		if !info.IsDir() {
			t.Fatalf("expected %s to be a directory", path)
		}
	}
	for _, path := range []string{
		"k-playbook/TODO.md",
		"k-playbook/reviews/known-decisions.md",
	} {
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatalf("expected %s: %v", path, err)
		}
		if info.IsDir() {
			t.Fatalf("expected %s to be a file", path)
		}
	}
}

func TestStatusReturnsGUIBackedProjectFields(t *testing.T) {
	root := t.TempDir()
	if _, err := EnsureConfig(root, RemediationModeTaskFirst); err != nil {
		t.Fatalf("EnsureConfig failed: %v", err)
	}
	if _, err := CompleteProjectStructure(root); err != nil {
		t.Fatalf("CompleteProjectStructure failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "k-playbook", "docs", "README.md"), []byte("# Docs\n"), 0o644); err != nil {
		t.Fatalf("write docs: %v", err)
	}

	status, err := Status(root)
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if status.Path != root {
		t.Fatalf("expected path %s, got %s", root, status.Path)
	}
	if !status.Setup.OK {
		t.Fatalf("expected setup OK: %#v", status.Setup)
	}
	if !status.Structure.OK {
		t.Fatalf("expected structure OK: %#v", status.Structure)
	}
	if !status.Docs.OK {
		t.Fatalf("expected docs OK: %#v", status.Docs)
	}
	if !status.Remediation.OK || status.Remediation.Mode != string(RemediationModeTaskFirst) {
		t.Fatalf("unexpected remediation status: %#v", status.Remediation)
	}
	if status.ProjectRoot.OK {
		t.Fatalf("expected project root error while repo_root is unset: %#v", status.ProjectRoot)
	}
}

func TestCheckReviewsAllowsGlobalOnlyReviewRecipes(t *testing.T) {
	root := t.TempDir()
	if _, err := CompleteProjectStructure(root); err != nil {
		t.Fatalf("CompleteProjectStructure failed: %v", err)
	}

	status := CheckReviews(root)
	if !status.OK {
		t.Fatalf("expected reviews OK without local review recipes: %#v", status)
	}
	if status.Reviews != 0 {
		t.Fatalf("expected no local review recipes, got %d", status.Reviews)
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

func TestResolveStatusPathUsesNearestParentConfig(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	nested := filepath.Join(project, "app", "service")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("create nested project path: %v", err)
	}
	if err := os.WriteFile(filepath.Join(project, ConfigFileName), []byte("schema_version: 1\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	resolved, err := ResolveStatusPath(nested)
	if err != nil {
		t.Fatalf("ResolveStatusPath failed: %v", err)
	}
	if resolved != project {
		t.Fatalf("expected %s, got %s", project, resolved)
	}
}
