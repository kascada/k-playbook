package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Jede mitgelieferte Hülle unter share/commands/ trägt argument-hint als
// Zeichenkette in doppelten Anführungszeichen. Unquotiert liest YAML einen
// Wert wie „[anzahl]“ als Liste, und zwei Klammerpaare hintereinander
// („[a] [b]“) machen das ganze Frontmatter ungültig — OpenCode 2 überspringt
// einen solchen Command dann ganz (so bei /k-docs bis v0.16.47).
func TestCommandHuellenArgumentHintIstZeichenkette(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "share", "commands")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(string(data), "\n")
		if len(lines) == 0 || lines[0] != "---" {
			continue
		}
		for _, line := range lines[1:] {
			if line == "---" {
				break
			}
			value, ok := strings.CutPrefix(line, "argument-hint:")
			if !ok {
				continue
			}
			value = strings.TrimSpace(value)
			if !strings.HasPrefix(value, `"`) || !strings.HasSuffix(value, `"`) {
				t.Errorf("%s: argument-hint ohne doppelte Anführungszeichen: %s", entry.Name(), line)
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("keine Hülle mit argument-hint gefunden")
	}
}
