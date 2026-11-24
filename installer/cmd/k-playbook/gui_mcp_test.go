package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// Der Start ergänzt einen fehlenden Eintrag nur in einer nicht erfassten
// Datei und meldet ihn samt dem Hinweis, dass die Datei einen
// rechnerbezogenen Pfad trägt. In einer erfassten Datei schreibt er nichts.
func TestStartErgaenztNurNichtErfassteDatei(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("kein git im Pfad")
	}

	for _, tracked := range []bool{true, false} {
		name := "nicht erfasst"
		if tracked {
			name = "erfasst"
		}
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			installed := filepath.Join(home, ".local", "bin", project.InstalledCommandName)
			writeRepairTestFile(t, installed, "#!/bin/sh\n", 0o755)

			root := t.TempDir()
			runRepairTestGit(t, root, "init", "--quiet")
			runRepairTestGit(t, root, "config", "user.email", "test@example.invalid")
			runRepairTestGit(t, root, "config", "user.name", "Test")
			writeRepairTestFile(t, project.ConfigPath(root),
				"schema_version: "+project.SchemaVersion+"\n\nproject:\n  repo_root: .\n  vcs: git\n", 0o644)
			before := `{"mcpServers":{}}` + "\n"
			writeRepairTestFile(t, filepath.Join(root, ".mcp.json"), before, 0o644)
			if tracked {
				runRepairTestGit(t, root, "add", ".mcp.json")
				runRepairTestGit(t, root, "commit", "-qm", "Registrierung")
			}
			t.Chdir(root)

			out := captureStdout(t, repairMCPRegistration)

			written := strings.Contains(out, "MCP-Registrierung eingetragen: .mcp.json\n")
			if written == tracked {
				t.Errorf("eingetragen = %v bei erfasst = %v:\n%s", written, tracked, out)
			}
			hint := strings.Contains(out, "rechnerbezogenen Pfad")
			if hint == tracked {
				t.Errorf("Hinweis auf den rechnerbezogenen Pfad = %v bei erfasst = %v:\n%s", hint, tracked, out)
			}
			content, err := os.ReadFile(filepath.Join(root, ".mcp.json"))
			if err != nil {
				t.Fatal(err)
			}
			if tracked && string(content) != before {
				t.Errorf("die erfasste Datei wurde verändert:\n%s", content)
			}
			if !tracked && !strings.Contains(string(content), installed) {
				t.Errorf("der absolute Pfad fehlt in der ergänzten Datei:\n%s", content)
			}
		})
	}
}

func writeRepairTestFile(t *testing.T, path string, content string, mode os.FileMode) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("%s anlegen: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatalf("%s schreiben: %v", path, err)
	}
}

func runRepairTestGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}
