package project

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// Greift die Grenze von docsScopeMaxGitignores, nennt das Ergebnis die Zahl
// der nicht gelesenen Dateien; sonst bleibt das Feld 0 (Task 184).
func TestDocsScopeGitignoreGrenze(t *testing.T) {
	root := t.TempDir()
	if err := CreateConfig(root, "."); err != nil {
		t.Fatal(err)
	}
	write := func(n int) {
		for i := 0; i < n; i++ {
			dir := filepath.Join(root, fmt.Sprintf("d%03d", i))
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("x/\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	write(docsScopeMaxGitignores - 1)
	result, err := DocsScope(context.Background(), root, DocsScopeOptions{})
	if err != nil || result.GitignoreTruncated != 0 || len(result.Exclude.Gitignore) != docsScopeMaxGitignores-1 {
		t.Fatalf("unter der Grenze: %d %d %v", result.GitignoreTruncated, len(result.Exclude.Gitignore), err)
	}
	write(docsScopeMaxGitignores + 5)
	result, err = DocsScope(context.Background(), root, DocsScopeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Exclude.Gitignore) != docsScopeMaxGitignores || result.GitignoreTruncated != 5 {
		t.Errorf("Grenze: gelesen %d, nicht gelesen %d", len(result.Exclude.Gitignore), result.GitignoreTruncated)
	}
}

// alwaysExcludedAbs schließt .k-playbook/ und k-playbook/ unter project.dir
// und unter project.repoRoot aus, keph-backup/ und .git/ auf jeder Ebene.
func TestDocsScopeImmerAusgeschlossen(t *testing.T) {
	s := &docsScope{projectDir: "/r/proj"}
	for abs, want := range map[string]bool{
		"/r/proj/.k-playbook/x.md":   true,
		"/r/proj/k-playbook/x.md":    true,
		"/r/.k-playbook/x.md":        true,
		"/r/k-playbook":              true,
		"/r/proj/a/keph-backup/x.md": true,
		"/r/proj/a/.git/config":      true,
		"/r/proj/a/k-playbook/x.md":  false,
		"/r/proj/src/a.go":           false,
		"/r/other/.k-playbook/x.md":  false,
		"/r/proj/k-playbook-x/y.md":  false,
	} {
		if got := s.alwaysExcludedAbs(abs, "/r"); got != want {
			t.Errorf("%s: %v, erwartet %v", abs, got, want)
		}
	}
}
