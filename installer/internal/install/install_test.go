package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kascada/k-playbook/installer/payload"
)

func TestDiscoverWalksUpward(t *testing.T) {
	root := t.TempDir()
	deep := filepath.Join(root, "src", "pkg", "inner")
	mustMkdirAll(t, deep)
	mustMkdirAll(t, filepath.Join(root, PlaybookDirName))
	mustWrite(t, filepath.Join(root, PlaybookDirName, ConfigFileName), "schema_version: 2\n")

	got, err := Discover(deep)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if want := filepath.Join(root, PlaybookDirName); !sameDir(got, want) {
		t.Errorf("Discover = %s, want %s", got, want)
	}
}

// Standing inside the playbook directory must resolve to that directory, not to
// a parent — the old code needed an explicit guard for this.
func TestDiscoverFromInsidePlaybookDir(t *testing.T) {
	root := t.TempDir()
	playbook := filepath.Join(root, PlaybookDirName)
	mustMkdirAll(t, playbook)
	mustWrite(t, filepath.Join(playbook, ConfigFileName), "schema_version: 2\n")

	got, err := Discover(playbook)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if !sameDir(got, playbook) {
		t.Errorf("Discover = %s, want %s", got, playbook)
	}
}

// Discovery must not escape into an unrelated parent project through the
// worktree boundary.
func TestDiscoverStopsAtWorktreeRoot(t *testing.T) {
	outer := t.TempDir()
	mustMkdirAll(t, filepath.Join(outer, PlaybookDirName))
	mustWrite(t, filepath.Join(outer, PlaybookDirName, ConfigFileName), "schema_version: 2\n")

	inner := filepath.Join(outer, "vendor", "other")
	mustMkdirAll(t, filepath.Join(inner, ".git"))
	mustMkdirAll(t, filepath.Join(inner, "src"))

	if _, err := Discover(filepath.Join(inner, "src")); err == nil {
		t.Error("Discover fand ein Playbook jenseits der Worktree-Grenze")
	}
}

func TestInitProducesContract(t *testing.T) {
	root := t.TempDir()
	mustMkdirAll(t, filepath.Join(root, ".git"))

	result, err := Init(root, Options{Now: time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if !result.ConfigWritten {
		t.Error("K-PLAYBOOK.yaml wurde nicht geschrieben")
	}

	config, err := ReadConfig(result.PlaybookDir)
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}
	if config.SchemaVersion != SchemaVersion || config.Layout != LayoutName {
		t.Errorf("schema_version=%d layout=%s", config.SchemaVersion, config.Layout)
	}
	if config.RepoRoot != ".." {
		t.Errorf("repo_root = %q, want ..", config.RepoRoot)
	}
	for _, entry := range PathKeys {
		value, ok := config.Paths[entry.Key]
		if !ok {
			t.Errorf("paths.%s fehlt", entry.Key)
			continue
		}
		if strings.HasPrefix(value, PlaybookDirName+"/") {
			t.Errorf("paths.%s = %q traegt noch das k-playbook/-Praefix", entry.Key, value)
		}
		if value == DistDirName || strings.HasPrefix(value, DistDirName+"/") {
			t.Errorf("paths.%s = %q zeigt in die Installation", entry.Key, value)
		}
	}

	for _, name := range []string{"commands/k-review.md", "rules/docs-sync.md", "bin/k-check"} {
		if _, err := os.Stat(filepath.Join(result.DistDir, name)); err != nil {
			t.Errorf("_dist/%s fehlt", name)
		}
	}
	info, err := os.Stat(filepath.Join(result.DistDir, "bin", "k-check"))
	if err != nil || info.Mode()&0o111 == 0 {
		t.Error("bin/k-check ist nicht ausfuehrbar")
	}

	gitignore := mustRead(t, filepath.Join(root, ".gitignore"))
	if !strings.Contains(gitignore, PlaybookDirName+"/"+DistDirName+"/") {
		t.Errorf(".gitignore enthaelt die _dist-Regel nicht:\n%s", gitignore)
	}
}

// The central promise of the layout: update replaces _dist wholesale and touches
// nothing beside it.
func TestUpdateReplacesOnlyDist(t *testing.T) {
	root := t.TempDir()
	result, err := Init(root, Options{})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	stray := filepath.Join(result.DistDir, "rules", "handmade.md")
	mustWrite(t, stray, "edited in place\n")
	owned := filepath.Join(result.PlaybookDir, "enforcement", "team.md")
	mustWrite(t, owned, "project rule\n")
	todo := filepath.Join(result.PlaybookDir, "TODO.md")
	mustWrite(t, todo, "my todos\n")

	if _, err := Update(result.PlaybookDir); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if _, err := os.Stat(stray); err == nil {
		t.Error("handgeschriebene Datei in _dist hat das Update ueberlebt")
	}
	if got := mustRead(t, owned); got != "project rule\n" {
		t.Errorf("projekteigene Regel veraendert: %q", got)
	}
	if got := mustRead(t, todo); got != "my todos\n" {
		t.Errorf("TODO.md veraendert: %q", got)
	}
}

// Restore is the git-clone case: _dist is gone and the assistant symlinks dangle.
func TestRestoreAfterClone(t *testing.T) {
	root := t.TempDir()
	result, err := Init(root, Options{})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := os.RemoveAll(result.DistDir); err != nil {
		t.Fatal(err)
	}

	if _, err := Restore(result.PlaybookDir); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if _, err := os.Stat(filepath.Join(result.DistDir, "commands", "k-review.md")); err != nil {
		t.Error("_dist wurde nicht wiederhergestellt")
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "commands", "k-review.md")); err != nil {
		t.Error("Assistant-Symlink zeigt nach restore weiterhin ins Leere")
	}
}

// An existing real .claude/commands directory is project-owned and must survive.
func TestLinkAssistantKeepsOwnCommands(t *testing.T) {
	root := t.TempDir()
	own := filepath.Join(root, ".claude", "commands")
	mustMkdirAll(t, own)
	mustWrite(t, filepath.Join(own, "my-command.md"), "own\n")

	result, err := Init(root, Options{})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if got := mustRead(t, filepath.Join(own, "my-command.md")); got != "own\n" {
		t.Errorf("eigener Command wurde veraendert: %q", got)
	}
	if _, err := os.Stat(filepath.Join(own, "k-review.md")); err != nil {
		t.Error("mitgelieferter Command wurde nicht einzeln verlinkt")
	}
	_ = result
}

func TestMigrateConfigText(t *testing.T) {
	source := `schema_version: 1
layout: fixed-project-k-playbook

k_playbook:
  repo: ~/dev/k-playbook

paths:
  playbook: k-playbook
  tasks: k-playbook/tasks
  todo: k-playbook/TODO.md
  docs: k-playbook/docs

project:
  repo_root: ./app
  vcs: git

setup:
  updated_at: 2026-07-30

tools:
  codeql:
    languages:
      - python
`

	got, changes := migrateConfigText(source, "9.9.9", time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC))

	for _, want := range []string{
		"schema_version: 2",
		"layout: " + LayoutName,
		"  dist: " + DistDirName,
		"  version: 9.9.9",
		"  tasks: tasks",
		"  todo: TODO.md",
		"  docs: docs",
		"  commands: commands",
		"  repo_root: ../app",
		"  updated_at: 2026-08-08",
		"overlay:",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("fehlt in der migrierten Config: %q\n---\n%s", want, got)
		}
	}
	for _, unwanted := range []string{"repo: ~/dev/k-playbook", "playbook: k-playbook", "k-playbook/tasks", "fixed-project-k-playbook"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("Altlast in der migrierten Config: %q", unwanted)
		}
	}
	// Unknown blocks must survive untouched.
	if !strings.Contains(got, "tools:") || !strings.Contains(got, "      - python") {
		t.Errorf("unbekannter tools-Block ging verloren:\n%s", got)
	}
	if len(changes) == 0 {
		t.Error("keine Aenderungen berichtet")
	}
}

// Migrating twice must not corrupt an already-migrated project.
func TestMigrateIsIdempotent(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ConfigFileName), `schema_version: 1
layout: fixed-project-k-playbook

k_playbook:
  repo: ~/dev/k-playbook

paths:
  playbook: k-playbook
  tasks: k-playbook/tasks

project:
  repo_root: .
  vcs: git
`)

	if _, err := Migrate(root, false); err != nil {
		t.Fatalf("erste Migration: %v", err)
	}
	first := mustRead(t, filepath.Join(root, PlaybookDirName, ConfigFileName))

	result, err := Migrate(root, false)
	if err != nil {
		t.Fatalf("zweite Migration: %v", err)
	}
	if len(result.Changes) != 0 {
		t.Errorf("zweite Migration meldete Aenderungen: %v", result.Changes)
	}
	if second := mustRead(t, filepath.Join(root, PlaybookDirName, ConfigFileName)); second != first {
		t.Errorf("zweite Migration hat die Config veraendert:\n%s", second)
	}
}

// Extract removes its destination before writing, so a destination that would
// wipe the filesystem root or a home directory must be refused outright.
func TestExtractRefusesDangerousDestinations(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("kein Home-Verzeichnis ermittelbar")
	}
	for _, dest := range []string{"", "/", home} {
		if err := payload.Extract(dest); err == nil {
			t.Errorf("Extract(%q) wurde nicht abgelehnt", dest)
		}
	}
}

// Init on a non-existent directory must fail instead of creating a stray tree.
func TestInitRequiresExistingProject(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "gibt-es-nicht")
	if _, err := Init(missing, Options{}); err == nil {
		t.Error("Init auf ein nicht existierendes Verzeichnis wurde nicht abgelehnt")
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func sameDir(a string, b string) bool {
	resolvedA, errA := filepath.EvalSymlinks(a)
	resolvedB, errB := filepath.EvalSymlinks(b)
	if errA == nil && errB == nil {
		return resolvedA == resolvedB
	}
	return a == b
}
