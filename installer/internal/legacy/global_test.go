package legacy

import (
	"os"
	"path/filepath"
	"testing"
)

// newHome baut ein Home-Verzeichnis mit einer alten Basisinstallation daneben.
func newHome(t *testing.T) (string, string) {
	t.Helper()

	root := t.TempDir()
	home := filepath.Join(root, "home")
	playbook := filepath.Join(root, "dev", playbookSegment)

	for _, dir := range []string{filepath.Join(playbook, "commands"), filepath.Join(playbook, "skills")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("%s anlegen: %v", dir, err)
		}
	}
	if err := os.WriteFile(filepath.Join(playbook, "commands", "k-test.md"), []byte("test\n"), 0o644); err != nil {
		t.Fatalf("Beispieldatei anlegen: %v", err)
	}
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("Home anlegen: %v", err)
	}
	return home, playbook
}

func symlink(t *testing.T, source string, target string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("%s anlegen: %v", filepath.Dir(target), err)
	}
	if err := os.Symlink(source, target); err != nil {
		t.Fatalf("%s verlinken: %v", target, err)
	}
}

func TestEntferntVerzeichnisSymlink(t *testing.T) {
	home, playbook := newHome(t)
	link := filepath.Join(home, ".claude", "commands")
	symlink(t, filepath.Join(playbook, "commands"), link)

	removals, err := removeGlobalLinks(home)
	if err != nil {
		t.Fatalf("removeGlobalLinks: %v", err)
	}
	if len(removals) != 1 || removals[0].Path != link {
		t.Fatalf("Removals = %+v, erwartet nur %s", removals, link)
	}
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Errorf("%s besteht weiter", link)
	}
}

func TestEntferntEinzelLinksUndLeeresVerzeichnis(t *testing.T) {
	home, playbook := newHome(t)
	dir := filepath.Join(home, ".config", "opencode", "command")
	symlink(t, filepath.Join(playbook, "commands", "k-test.md"), filepath.Join(dir, "k-test.md"))
	// Ein toter Link zaehlt genauso: die Datei gibt es im Repo nicht mehr.
	symlink(t, filepath.Join(playbook, "commands", "k-weg.md"), filepath.Join(dir, "k-weg.md"))

	removals, err := removeGlobalLinks(home)
	if err != nil {
		t.Fatalf("removeGlobalLinks: %v", err)
	}
	if len(removals) != 3 {
		t.Fatalf("Removals = %+v, erwartet zwei Links und das leere Verzeichnis", removals)
	}
	if _, err := os.Lstat(dir); !os.IsNotExist(err) {
		t.Errorf("%s besteht weiter", dir)
	}
}

func TestLaesstFremdesInRuhe(t *testing.T) {
	home, _ := newHome(t)
	dir := filepath.Join(home, ".claude", "commands")
	other := filepath.Join(home, "eigenes", "mein.md")
	symlink(t, other, filepath.Join(dir, "fremd.md"))

	eigen := filepath.Join(dir, "eigen.md")
	if err := os.WriteFile(eigen, []byte("eigen\n"), 0o644); err != nil {
		t.Fatalf("eigene Datei anlegen: %v", err)
	}

	removals, err := removeGlobalLinks(home)
	if err != nil {
		t.Fatalf("removeGlobalLinks: %v", err)
	}
	if len(removals) != 0 {
		t.Fatalf("Removals = %+v, erwartet keine", removals)
	}
	if _, err := os.Lstat(eigen); err != nil {
		t.Errorf("eigene Datei entfernt: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(dir, "fremd.md")); err != nil {
		t.Errorf("fremder Link entfernt: %v", err)
	}
}

func TestLaeuftAufSauberemHomeDurch(t *testing.T) {
	home, _ := newHome(t)

	removals, err := removeGlobalLinks(home)
	if err != nil {
		t.Fatalf("removeGlobalLinks: %v", err)
	}
	if len(removals) != 0 {
		t.Fatalf("Removals = %+v, erwartet keine", removals)
	}
}

func TestEntferntSkillsPathAusConfig(t *testing.T) {
	home, _ := newHome(t)
	config := filepath.Join(home, ".config", "opencode", "opencode.jsonc")
	if err := os.MkdirAll(filepath.Dir(config), 0o755); err != nil {
		t.Fatalf("Config-Verzeichnis anlegen: %v", err)
	}
	content := "{\n  \"$schema\": \"https://opencode.ai/config.json\",\n  \"skills\": {\n    \"paths\": [\"~/dev/k-playbook\"]\n  }\n}\n"
	if err := os.WriteFile(config, []byte(content), 0o644); err != nil {
		t.Fatalf("Config schreiben: %v", err)
	}

	removals, err := removeGlobalLinks(home)
	if err != nil {
		t.Fatalf("removeGlobalLinks: %v", err)
	}
	if len(removals) != 1 || removals[0].Path != config {
		t.Fatalf("Removals = %+v, erwartet nur %s", removals, config)
	}

	data, err := os.ReadFile(config)
	if err != nil {
		t.Fatalf("Config lesen: %v", err)
	}
	want := "{\n  \"$schema\": \"https://opencode.ai/config.json\"\n}\n"
	if string(data) != want {
		t.Errorf("Config = %q, erwartet %q", string(data), want)
	}
}

func TestStripSkillsBlock(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
		changed bool
	}{
		{
			name:    "einziger Eintrag",
			content: "{\n  \"skills\": {\n    \"paths\": [\"~/dev/k-playbook\"]\n  }\n}\n",
			want:    "{\n}\n",
			changed: true,
		},
		{
			name:    "Eintrag in der Mitte",
			content: "{\n  \"skills\": {\"paths\": [\"~/dev/k-playbook\"]},\n  \"theme\": \"dark\"\n}\n",
			want:    "{\n  \"theme\": \"dark\"\n}\n",
			changed: true,
		},
		{
			name:    "nachtraeglich eingefuegte Form",
			content: "{\n  \"theme\": \"dark\"\n  ,\"skills\": {\n    \"paths\": [\"~/dev/k-playbook\"]\n  }\n}\n",
			want:    "{\n  \"theme\": \"dark\"\n}\n",
			changed: true,
		},
		{
			name:    "Kommentare bleiben",
			content: "{\n  // eigener Hinweis\n  \"theme\": \"dark\",\n  \"skills\": {\"paths\": [\"~/dev/k-playbook\"]}\n}\n",
			want:    "{\n  // eigener Hinweis\n  \"theme\": \"dark\"\n}\n",
			changed: true,
		},
		{
			name:    "fremder skills-Pfad bleibt",
			content: "{\n  \"skills\": {\"paths\": [\"~/dev/eigene-skills\"]}\n}\n",
			want:    "{\n  \"skills\": {\"paths\": [\"~/dev/eigene-skills\"]}\n}\n",
			changed: false,
		},
		{
			name:    "skills nur als Wert genannt",
			content: "{\n  \"note\": \"skills liegen unter ~/dev/k-playbook\"\n}\n",
			want:    "{\n  \"note\": \"skills liegen unter ~/dev/k-playbook\"\n}\n",
			changed: false,
		},
		{
			name:    "verschachteltes skills bleibt",
			content: "{\n  \"agent\": {\"skills\": {\"paths\": [\"~/dev/k-playbook\"]}}\n}\n",
			want:    "{\n  \"agent\": {\"skills\": {\"paths\": [\"~/dev/k-playbook\"]}}\n}\n",
			changed: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, changed := stripSkillsBlock(test.content)
			if changed != test.changed {
				t.Fatalf("changed = %v, erwartet %v", changed, test.changed)
			}
			if got != test.want {
				t.Errorf("Ergebnis = %q, erwartet %q", got, test.want)
			}
		})
	}
}

func TestHasPlaybookSegment(t *testing.T) {
	tests := map[string]bool{
		"/home/x/dev/k-playbook/commands":       true,
		"/home/x/projekt/k-playbook/skills":     true,
		"/home/x/dev/k-playbook-local/commands": false,
		"/home/x/dev/eigenes/commands":          false,
	}

	for path, want := range tests {
		if got := hasPlaybookSegment(path); got != want {
			t.Errorf("hasPlaybookSegment(%q) = %v, erwartet %v", path, got, want)
		}
	}
}
