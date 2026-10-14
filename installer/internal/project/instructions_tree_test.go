package project

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// TestInstructionsGleichstand sichert, dass instructions/ oben im Repo und die
// Kopie defaults/instructions/ dieselben Dateien mit demselben Inhalt tragen.
//
// instructions/ wird laufend gepflegt — anders als der übrige Inhalt von
// defaults/ ist die Kopie nicht eingefroren. Die Installation liest die
// oberste Ebene, der Hub das Archiv aus defaults/; weichen die beiden ab,
// bekämen Installation und Hub verschiedene Anweisungen. pack-defaults.sh
// prüft dasselbe beim Packen, dieser Test schon im normalen Testlauf.
func TestInstructionsGleichstand(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("Repo-Wurzel: %v", err)
	}
	top := filepath.Join(root, "instructions")
	copied := filepath.Join(root, "defaults", "instructions")

	topFiles := treeFiles(t, top)
	copiedFiles := treeFiles(t, copied)
	if len(topFiles) == 0 {
		t.Fatalf("%s enthält keine Dateien", top)
	}

	for _, rel := range topFiles {
		left, err := os.ReadFile(filepath.Join(top, rel))
		if err != nil {
			t.Fatalf("%s lesen: %v", rel, err)
		}
		right, err := os.ReadFile(filepath.Join(copied, rel))
		if err != nil {
			t.Errorf("defaults/instructions/%s fehlt oder ist unlesbar: %v", rel, err)
			continue
		}
		if !bytes.Equal(left, right) {
			t.Errorf("instructions/%s und defaults/instructions/%s weichen voneinander ab", rel, rel)
		}
	}
	known := map[string]bool{}
	for _, rel := range topFiles {
		known[rel] = true
	}
	for _, rel := range copiedFiles {
		if !known[rel] {
			t.Errorf("defaults/instructions/%s fehlt in instructions/", rel)
		}
	}
}

// treeFiles listet alle Dateien unter dir, relativ und mit Schrägstrich,
// sortiert.
func treeFiles(t *testing.T, dir string) []string {
	t.Helper()

	var files []string
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatalf("%s durchlaufen: %v", dir, err)
	}
	sort.Strings(files)
	return files
}
