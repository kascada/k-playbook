package project

import (
	"context"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/health"
)

func frontmatterProblems(root string) []health.Problem {
	return healthFrontmatter(context.Background(), root)
}

// Die zwei Fälle aus omni, die in OpenCode 2 einen Command unsichtbar machen.
func TestHealthFrontmatterMeldetOmniFaelle(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ProjectDirName, "commands", "k-docs.md"),
		"---\ndescription: Docs\nargument-hint: [status|inventory|code|tools|alle] [zusatz-pfad]\n---\n\n# Docs\n")
	writeFile(t, filepath.Join(root, ProjectDirName, "skills", "omni-backlog", skillFileName),
		"---\nname: omni-backlog\ndescription: Nutzen wenn: der Backlog bearbeitet wird\n---\n\n# Skill\n")

	problems := frontmatterProblems(root)
	if len(problems) != 2 {
		t.Fatalf("%d Probleme, erwartet 2: %+v", len(problems), problems)
	}
	for _, p := range problems {
		if p.Level != health.LevelError || p.Area != health.AreaFrontmatter {
			t.Errorf("Problem = %+v, erwartet Stufe error im Bereich frontmatter", p)
		}
	}
	joined := problems[0].Message + problems[1].Message
	for _, want := range []string{".k-playbook/commands/k-docs.md", ".k-playbook/skills/omni-backlog/SKILL.md"} {
		if !strings.Contains(joined, want) {
			t.Errorf("Meldungen nennen %s nicht: %s", want, joined)
		}
	}
}

func TestHealthFrontmatterPruefteFragmenteAberNichtAbgeschaltetes(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ProjectDirName, "commands", "_shared", "kaputt.md"), "---\nx: a: b\n---\ntext\n")
	writeFile(t, filepath.Join(root, ProjectDirName, "commands", "_shared", "ohne.md"), "# ohne Frontmatter\n")
	writeFile(t, filepath.Join(root, ProjectDirName, "commands", "leer.md"), "")
	writeFile(t, filepath.Join(root, ProjectDirName, "commands", "gut.md"), "---\nargument-hint: \"[a|b] [c]\"\n---\n")

	problems := frontmatterProblems(root)
	if len(problems) != 1 || !strings.Contains(problems[0].Message, "_shared/kaputt.md") {
		t.Fatalf("Probleme = %+v, erwartet genau das kaputte Fragment", problems)
	}
}

// Jede Markdown-Datei unter defaults/ — den Texten, die über Kephalaion
// ausgeliefert werden — hat ein lesbares Frontmatter. Geprüft wird nur
// defaults/; share/, das das Binary selbst mitbringt, deckt dieser Test nicht
// ab.
func TestFrontmatterDerAuslieferungIstGueltig(t *testing.T) {
	defaults := filepath.Join("..", "..", "..", "defaults")
	count := 0
	err := filepath.WalkDir(defaults, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".md") {
			return err
		}
		count++
		if err := CheckFrontmatter(path); err != nil {
			t.Errorf("%s: %v", path, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if count == 0 {
		t.Fatal("keine Dateien unter defaults/ gefunden")
	}
}
