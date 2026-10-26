// Package accesstest gibt Tests eine eigene GitHub-Zugangsdatei, damit kein
// Test die persönliche ~/.config/k-playbook/github.yaml des Rechners liest
// oder — bei fehlender Datei — dort die Vorlage anlegt. Sobald die Quelle
// github in der Umgebungsprüfung registriert ist, erreicht jedes health.Check
// und jedes Briefing den Leser; jedes Testpaket, das dorthin kommt, schirmt
// sich in TestMain mit Isolate ab, wie sharetest.Isolate es für
// ~/.local/share/k-playbook tut. Unterprozesse erben die Umgebung und damit
// den Override.
package accesstest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/githubaccess"
)

// Isolate setzt den Override K_PLAYBOOK_GITHUB_CONFIG auf eine Datei in einem
// leeren Verzeichnis und gibt die Aufräumfunktion zurück. Gedacht für
// TestMain: Danach legt ein Test, der den Leser erreicht, höchstens dort die
// Vorlage an, nie am echten Ort.
func Isolate() (cleanup func(), err error) {
	dir, err := os.MkdirTemp("", "k-playbook-github-leer-")
	if err != nil {
		return nil, err
	}
	if err := os.Setenv(githubaccess.EnvConfig, filepath.Join(dir, githubaccess.FileName)); err != nil {
		os.RemoveAll(dir)
		return nil, err
	}
	return func() { os.RemoveAll(dir) }, nil
}

// Empty gibt einem Test eine vorhandene, leere Zugangsdatei (services: {})
// und setzt den Override darauf. Die Quelle github der Umgebungsprüfung
// schweigt dann — für Tests, die die Einträge der anderen Quellen zählen.
// Liefert den Pfad, falls der Test die Datei danach füllen will.
func Empty(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), githubaccess.FileName)
	if err := os.WriteFile(path, []byte("services: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(githubaccess.EnvConfig, path)
	return path
}
