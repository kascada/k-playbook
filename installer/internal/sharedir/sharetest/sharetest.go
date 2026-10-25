// Package sharetest gibt Tests ein eigenes ~/.local/share/k-playbook, damit
// kein Test die mitgelieferten Dateien des Rechners liest, auf dem er läuft.
package sharetest

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// Isolate setzt XDG_DATA_HOME auf ein leeres Verzeichnis und gibt die
// Aufräumfunktion zurück. Gedacht für TestMain: Danach sieht jeder Test des
// Pakets ein leeres mitgeliefertes Verzeichnis, solange er nicht Dir ruft.
func Isolate() (cleanup func(), err error) {
	empty, err := os.MkdirTemp("", "k-playbook-share-leer-")
	if err != nil {
		return nil, err
	}
	if err := os.Setenv("XDG_DATA_HOME", empty); err != nil {
		os.RemoveAll(empty)
		return nil, err
	}
	return func() { os.RemoveAll(empty) }, nil
}

var dirs sync.Map // *testing.T -> string

// Dir ist das mitgelieferte Verzeichnis dieses Tests. Jeder Test bekommt ein
// eigenes, derselbe Test bei jedem Aufruf dasselbe.
func Dir(t *testing.T) string {
	t.Helper()
	if dir, ok := dirs.Load(t); ok {
		return dir.(string)
	}
	base := t.TempDir()
	t.Setenv("XDG_DATA_HOME", base)
	dir := filepath.Join(base, "k-playbook")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	dirs.Store(t, dir)
	t.Cleanup(func() { dirs.Delete(t) })
	return dir
}
