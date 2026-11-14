package scripts

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// Die beiden Skripte nennen ihr Verzeichnis nach dem, was es ist: die Skripte
// unter ~/.local/share/k-playbook/scripts/, nicht mehr „Installation“. Das
// JSON-Feld playbookDir entfällt, weil es niemand liest (Task 154). Wörter,
// die das Installieren von Werkzeugen meinen, bleiben.
func TestPreflightNenntSkriptverzeichnisStattInstallation(t *testing.T) {
	scripts := []struct {
		name string
		env  []string
		args []string
	}{
		{name: "install-base-tools.sh", args: []string{"--preflight"}},
		{
			name: "install-security-tools.sh",
			env:  []string{"K_SECURITY_TOOLS_MATRIX=" + writeMatrix(t, securityMatrix)},
			args: []string{"--preflight", "--bin-dir", t.TempDir()},
		},
	}

	for _, script := range scripts {
		t.Run(script.name, func(t *testing.T) {
			path := scriptPath(t, script.name)
			dir, err := filepath.EvalSymlinks(filepath.Dir(path))
			if err != nil {
				t.Fatalf("Skriptverzeichnis auflösen: %v", err)
			}

			text := runScript(t, path, script.env, script.args...)
			if text.code != 0 {
				t.Fatalf("Preflight brach ab:\n%s", text.all())
			}
			wantLine := false
			for _, line := range strings.Split(text.stdout, "\n") {
				if strings.HasPrefix(line, "Installation: ") {
					t.Errorf("Preflight nennt das Verzeichnis noch Installation: %q", line)
				}
				if strings.HasPrefix(line, "Skripte:") && strings.TrimSpace(strings.TrimPrefix(line, "Skripte:")) == dir {
					wantLine = true
				}
			}
			if !wantLine {
				t.Errorf("Preflight nennt das Skriptverzeichnis %s nicht:\n%s", dir, text.stdout)
			}

			jsonArgs := append([]string{"--json"}, script.args[1:]...)
			raw := runScript(t, path, script.env, jsonArgs...)
			if raw.code != 0 {
				t.Fatalf("--json brach ab:\n%s", raw.all())
			}
			var fields map[string]any
			if err := json.Unmarshal([]byte(raw.stdout), &fields); err != nil {
				t.Fatalf("--json nicht lesbar: %v\n%s", err, raw.stdout)
			}
			if _, found := fields["playbookDir"]; found {
				t.Errorf("--json trägt noch playbookDir: %s", raw.stdout)
			}
		})
	}
}
