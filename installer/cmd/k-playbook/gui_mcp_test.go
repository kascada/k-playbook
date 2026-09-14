package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// Die Startmeldung hängt an der geschriebenen Form: in einer erfassten Datei
// steht nach der Korrektur der bloße Name, und die Meldung nennt ihn samt
// Dock/Finder-Hinweis. In einer nicht erfassten Datei bleibt der absolute Pfad,
// und der Hinweis fehlt.
func TestStartmeldungNenntDiePortableForm(t *testing.T) {
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
			writeRepairTestFile(t, filepath.Join(root, ".mcp.json"),
				`{"mcpServers":{"`+project.MCPServerKey+`":{"command":"k-playbook/bin/k-playbook","args":["mcp"]}}}`+"\n", 0o644)
			if tracked {
				runRepairTestGit(t, root, "add", ".mcp.json")
				runRepairTestGit(t, root, "commit", "-qm", "Registrierung")
			}
			t.Chdir(root)

			out := captureStdout(t, repairMCPRegistration)

			want := installed
			if tracked {
				want = project.InstalledCommandName
			}
			if !strings.Contains(out, "Veraltete MCP-Registrierung korrigiert: .mcp.json -> "+want+"\n") {
				t.Errorf("Korrekturzeile fehlt oder nennt eine andere Form:\n%s", out)
			}
			hint := strings.Contains(out, "bloße Name") && strings.Contains(out, "Dock oder Finder")
			if hint != tracked {
				t.Errorf("Hinweis auf die portable Form = %v, erwartet %v:\n%s", hint, tracked, out)
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
