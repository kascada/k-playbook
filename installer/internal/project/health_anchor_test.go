package project

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/health"
)

// anchorRepo legt ein Projekt an, dessen Hauptverzeichnis ein Git-Repository
// ist, mit Anker über CreateConfig (project.vcs: git). Committet ist nichts.
func anchorRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git nicht verfügbar")
	}
	root := t.TempDir()
	run(t, root, "git", "init", "-q", "-b", "main")
	run(t, root, "git", "config", "user.email", "test@example.com")
	run(t, root, "git", "config", "user.name", "Test")
	if err := CreateConfig(root, "."); err != nil {
		t.Fatalf("Konfiguration anlegen: %v", err)
	}
	return root
}

func anchorProblemsOf(t *testing.T, root string) []health.Problem {
	t.Helper()
	return healthAnchor(context.Background(), root)
}

func TestHealthAnchorNichtVersioniertWarnt(t *testing.T) {
	root := anchorRepo(t)
	writeFile(t, filepath.Join(root, "README.md"), "x\n")
	run(t, root, "git", "add", "README.md")
	run(t, root, "git", "commit", "-q", "-m", "erster Stand")

	problems := anchorProblemsOf(t, root)
	if len(problems) != 1 {
		t.Fatalf("Probleme = %+v, erwartet eines", problems)
	}
	got := problems[0]
	if got.Level != health.LevelWarning || got.Area != health.AreaAnchor ||
		!strings.Contains(got.Message, ".k-playbook/K-PLAYBOOK.yaml ist weder versioniert noch ignoriert") ||
		strings.Contains(got.Message, "umgezogen") ||
		!strings.Contains(got.Hint, "git add .k-playbook/K-PLAYBOOK.yaml") || !strings.Contains(got.Hint, ".gitignore") {
		t.Errorf("Eintrag: %+v", got)
	}
	if report := health.Check(context.Background(), root); !report.OK {
		t.Errorf("Warnung lässt OK nicht wahr: %+v", report)
	}
}

// Ein frisch angelegtes Projekt vor dem ersten Commit ist derselbe Zustand:
// die Meldung passt, weil sie keinen Umzug behauptet.
func TestHealthAnchorNeuesProjektOhneCommitWarnt(t *testing.T) {
	root := anchorRepo(t)

	problems := anchorProblemsOf(t, root)
	if len(problems) != 1 || !strings.Contains(problems[0].Message, "weder versioniert noch ignoriert") {
		t.Fatalf("Probleme = %+v, erwartet eine Zustandsmeldung", problems)
	}
}

func TestHealthAnchorVersioniertSchweigt(t *testing.T) {
	root := anchorRepo(t)
	run(t, root, "git", "add", ".k-playbook/K-PLAYBOOK.yaml")
	run(t, root, "git", "commit", "-q", "-m", "Anker")

	if problems := anchorProblemsOf(t, root); len(problems) != 0 {
		t.Errorf("Probleme = %+v, erwartet keines", problems)
	}
}

// Nur in den Index aufgenommen, noch nicht committet: versioniert genug.
func TestHealthAnchorImIndexSchweigt(t *testing.T) {
	root := anchorRepo(t)
	run(t, root, "git", "add", ".k-playbook/K-PLAYBOOK.yaml")

	if problems := anchorProblemsOf(t, root); len(problems) != 0 {
		t.Errorf("Probleme = %+v, erwartet keines", problems)
	}
}

func TestHealthAnchorIgnoriertSchweigt(t *testing.T) {
	root := anchorRepo(t)
	writeFile(t, filepath.Join(root, ".gitignore"), "/.k-playbook/K-PLAYBOOK.yaml\n")

	if problems := anchorProblemsOf(t, root); len(problems) != 0 {
		t.Errorf("Probleme = %+v, erwartet keines", problems)
	}
}

func TestHealthAnchorOhneGitSchweigt(t *testing.T) {
	root := t.TempDir()
	writeFile(t, ConfigPath(root), "schema_version: 3\n\nproject:\n  repo_root: .\n  vcs: git\n")

	if problems := anchorProblemsOf(t, root); len(problems) != 0 {
		t.Errorf("Probleme = %+v, erwartet keines", problems)
	}
}

func TestHealthAnchorOhneVCSGitSchweigt(t *testing.T) {
	root := anchorRepo(t)
	writeFile(t, ConfigPath(root), "schema_version: 3\n\nproject:\n  repo_root: .\n  vcs: none\n")

	if problems := anchorProblemsOf(t, root); len(problems) != 0 {
		t.Errorf("Probleme = %+v, erwartet keines", problems)
	}
}

// Der Anker lag am früheren Ort und war committet; Discover hat ihn
// verschoben, der Index kennt noch den alten Pfad. Der neue Ort ist schon
// aufgenommen, damit nur der Umzugsbefund spricht.
func TestHealthAnchorAlterOrtNochImIndexWarnt(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git nicht verfügbar")
	}
	root := t.TempDir()
	run(t, root, "git", "init", "-q", "-b", "main")
	run(t, root, "git", "config", "user.email", "test@example.com")
	run(t, root, "git", "config", "user.name", "Test")
	writeFile(t, legacyConfigPath(root), "schema_version: 3\n\nproject:\n  repo_root: .\n  vcs: git\n")
	run(t, root, "git", "add", ConfigFileName)
	run(t, root, "git", "commit", "-q", "-m", "alter Ort")

	if _, err := Discover(root); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if _, err := os.Lstat(legacyConfigPath(root)); !os.IsNotExist(err) {
		t.Fatalf("alter Ort nicht geräumt: %v", err)
	}
	run(t, root, "git", "add", ".k-playbook/K-PLAYBOOK.yaml")

	problems := anchorProblemsOf(t, root)
	if len(problems) != 1 {
		t.Fatalf("Probleme = %+v, erwartet eines", problems)
	}
	got := problems[0]
	if got.Level != health.LevelWarning || got.Area != health.AreaAnchor ||
		!strings.Contains(got.Message, "früheren Ort") || !strings.Contains(got.Message, "umgezogen") ||
		!strings.Contains(got.Hint, "git rm --cached K-PLAYBOOK.yaml") || !strings.Contains(got.Hint, "committen") {
		t.Errorf("Eintrag: %+v", got)
	}

	// Ohne das git add sprechen beide Befunde.
	run(t, root, "git", "rm", "-q", "--cached", ".k-playbook/K-PLAYBOOK.yaml")
	if problems := anchorProblemsOf(t, root); len(problems) != 2 {
		t.Errorf("Probleme = %+v, erwartet zwei", problems)
	}
}
