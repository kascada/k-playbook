package project

import (
	"path/filepath"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/kephalaion/kephtest"
)

// testDefaultsAddress ist die Adresse der öffentlichen Collection in Tests.
const testDefaultsAddress = "test:k-playbook"

// realOpenDefaultsStore ist OpenDefaultsStore, wie das Programm es hat — für
// Tests des echten Wegs über die Anbindung.
var realOpenDefaultsStore = OpenDefaultsStore

// isolateDefaults setzt für das ganze Paket eine öffentliche Collection im
// Speicher ein (TestMain): Kein Test fragt den Node dieses Rechners. Gefüllt
// wird sie beim Öffnen aus dem Baum, den ein Test unter k-playbook/defaults/
// seines Projekts anlegt — dem Lieferumfang, wie der Release ihn packt; fehlt
// er, steht nur VERSION darin. Gelesen wird über den Store, nicht über die
// Platte.
func isolateDefaults() func() {
	original := OpenDefaultsStore
	OpenDefaultsStore = func(projectDir string) (DefaultsStore, string, error) {
		store, err := kephtest.Defaults(filepath.Join(PlaybookDir(projectDir), defaultsDirName))
		if err != nil {
			return nil, testDefaultsAddress, err
		}
		return store, testDefaultsAddress, nil
	}
	return func() { OpenDefaultsStore = original }
}

// stubDefaultsStore setzt für einen Test einen festen Store ein.
func stubDefaultsStore(t *testing.T, open func(projectDir string) (DefaultsStore, string, error)) {
	t.Helper()
	original := OpenDefaultsStore
	OpenDefaultsStore = open
	t.Cleanup(func() { OpenDefaultsStore = original })
}

// repoDefaultsSource ist das echte defaults/ des Repos als mitgelieferte
// Ebene — so, wie der Hub es nach dem Release trägt.
func repoDefaultsSource(t *testing.T) DocSource {
	t.Helper()
	repo, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	store, err := kephtest.Defaults(filepath.Join(repo, defaultsDirName))
	if err != nil {
		t.Fatalf("defaults/ des Repos laden: %v", err)
	}
	return NewKephDefaultsSource(func() (DefaultsStore, string, error) { return store, testDefaultsAddress, nil })
}
