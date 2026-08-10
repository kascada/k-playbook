package webui

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

func TestBrowserFallbackHintOnlyForDevcontainer(t *testing.T) {
	url := "http://127.0.0.1:12345/"

	if got := browserFallbackHint(runtimeStatus{}, url); got != "" {
		t.Fatalf("expected no host hint, got %q", got)
	}
	if got := browserFallbackHint(runtimeStatus{InsideContainer: true}, url); got != "" {
		t.Fatalf("expected no generic container hint, got %q", got)
	}
	got := browserFallbackHint(runtimeStatus{InsideContainer: true, InsideDevcontainer: true}, url)
	if !strings.Contains(got, "DevContainer erkannt") || !strings.Contains(got, url) {
		t.Fatalf("expected DevContainer hint with URL, got %q", got)
	}
}

func TestIsAllowedDocPathAllowsRootReadmeAndDocsOnly(t *testing.T) {
	allowed := []string{
		"README.md",
		"docs/README.md",
		"docs/commands.md",
	}
	for _, path := range allowed {
		if !isAllowedDocPath(path) {
			t.Fatalf("expected %s to be allowed", path)
		}
	}

	rejected := []string{
		".",
		"../README.md",
		"AGENTS.md",
		"commands/k-gui.md",
	}
	for _, path := range rejected {
		if isAllowedDocPath(path) {
			t.Fatalf("expected %s to be rejected", path)
		}
	}
}

func TestInstallBinaryFileCopiesExecutableAtomically(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source-binary")
	destination := filepath.Join(root, "bin", "k-playbook-installer")

	if err := os.WriteFile(source, []byte("installer"), 0o755); err != nil {
		t.Fatalf("write source: %v", err)
	}

	if err := installBinaryFile(source, destination); err != nil {
		t.Fatalf("install binary: %v", err)
	}
	data, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("read destination: %v", err)
	}
	if string(data) != "installer" {
		t.Fatalf("expected destination content to match source, got %q", string(data))
	}
	info, err := os.Stat(destination)
	if err != nil {
		t.Fatalf("stat destination: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o755 {
		t.Fatalf("expected destination mode 0755, got %v", info.Mode().Perm())
	}
}

func TestInstallerDistChangedDetectsAddedAndChangedAssets(t *testing.T) {
	if installerDistChanged(map[string]string{"a": "1"}, map[string]string{"a": "1"}) {
		t.Fatalf("expected identical hashes to be unchanged")
	}
	if !installerDistChanged(map[string]string{"a": "1"}, map[string]string{"a": "2"}) {
		t.Fatalf("expected changed hash to be detected")
	}
	if !installerDistChanged(map[string]string{"a": "1"}, map[string]string{"a": "1", "b": "2"}) {
		t.Fatalf("expected added asset to be detected")
	}
}

func TestSyncInstallerBinariesMirrorsAllDistAssetsAndWrapper(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink behavior is platform-specific")
	}
	root := t.TempDir()
	oldHome := t.TempDir()
	t.Setenv("HOME", oldHome)

	distDir := filepath.Join(root, "dist")
	templateDir := filepath.Join(root, "scripts", "templates")
	if err := os.MkdirAll(distDir, 0o755); err != nil {
		t.Fatalf("create dist dir: %v", err)
	}
	if err := os.MkdirAll(templateDir, 0o755); err != nil {
		t.Fatalf("create template dir: %v", err)
	}
	assets := map[string]string{
		"k-playbook-installer-linux-amd64":  "linux-amd64",
		"k-playbook-installer-darwin-arm64": "darwin-arm64",
	}
	for name, content := range assets {
		if err := os.WriteFile(filepath.Join(distDir, name), []byte(content), 0o755); err != nil {
			t.Fatalf("write dist asset: %v", err)
		}
	}
	wrapperTemplate := filepath.Join(templateDir, "k-playbook-installer-wrapper.sh")
	if err := os.WriteFile(wrapperTemplate, []byte("#!/usr/bin/env bash\n"), 0o755); err != nil {
		t.Fatalf("write wrapper template: %v", err)
	}

	installed, changed, err := syncInstallerBinaries(root)
	if err != nil {
		t.Fatalf("sync installer binaries: %v", err)
	}
	if !changed {
		t.Fatalf("expected first sync to install files")
	}
	if len(installed) != 4 {
		t.Fatalf("expected two assets, wrapper and global link to be installed, got %d: %v", len(installed), installed)
	}
	for name, content := range assets {
		path := filepath.Join(root, "bin", name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read mirrored asset %s: %v", name, err)
		}
		if string(data) != content {
			t.Fatalf("expected mirrored asset %s content %q, got %q", name, content, string(data))
		}
	}
	link := filepath.Join(oldHome, ".local", "bin", "k-playbook-installer")
	target, err := os.Readlink(link)
	if err != nil {
		t.Fatalf("read global symlink: %v", err)
	}
	if target != filepath.Join(root, "bin", "k-playbook-installer") {
		t.Fatalf("expected global link target to be repo wrapper, got %s", target)
	}

	installed, changed, err = syncInstallerBinaries(root)
	if err != nil {
		t.Fatalf("second sync installer binaries: %v", err)
	}
	if changed || len(installed) != 0 {
		t.Fatalf("expected second sync to be unchanged, got changed=%v installed=%v", changed, installed)
	}
}

// Der PATH-Eintrag ist der host-weite Installationsort. Ein Repo- oder Projektpfad
// darf dort nie landen: ein Binary bedient alle Projekte, und deren Pfade sind
// verschieden.
func TestEnsureLauncherPathAddsInstallDirToProfile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/bash")
	t.Setenv("PATH", "/usr/bin:/bin")

	changed, message, err := ensureLauncherPath()
	if err != nil {
		t.Fatalf("ensure launcher path: %v", err)
	}
	if !changed {
		t.Fatalf("expected profile to be changed, message=%q", message)
	}

	profile := filepath.Join(home, ".profile")
	data, err := os.ReadFile(profile)
	if err != nil {
		t.Fatalf("read profile: %v", err)
	}
	expectedPath := "${HOME}/.local/bin"
	if !strings.Contains(string(data), expectedPath) {
		t.Fatalf("expected profile to contain %s, got %q", expectedPath, string(data))
	}

	changed, message, err = ensureLauncherPath()
	if err != nil {
		t.Fatalf("second ensure launcher path: %v", err)
	}
	if changed {
		t.Fatalf("expected existing profile entry to be unchanged, message=%q", message)
	}
}

func TestEnsureLauncherPathDoesNothingWhenPathAlreadyContainsInstallDir(t *testing.T) {
	home := t.TempDir()
	expectedPath := filepath.Join(home, ".local", "bin")
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/bash")
	t.Setenv("PATH", strings.Join([]string{"/usr/bin", expectedPath, "/bin"}, string(os.PathListSeparator)))

	changed, message, err := ensureLauncherPath()
	if err != nil {
		t.Fatalf("ensure launcher path: %v", err)
	}
	if changed || message != "" {
		t.Fatalf("expected no change and no message, got changed=%v message=%q", changed, message)
	}
	if _, err := os.Stat(filepath.Join(home, ".profile")); !os.IsNotExist(err) {
		t.Fatalf("expected profile to stay absent, stat err=%v", err)
	}
}
