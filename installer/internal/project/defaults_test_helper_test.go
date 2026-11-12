package project

import (
	"path/filepath"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
	"github.com/kascada/k-playbook/installer/internal/kephalaion/kephtest"
	"github.com/kascada/k-playbook/installer/internal/versionsources"
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
//
// Ebenso die Collection des Projekts für die projekteigene Ebene
// (OpenProjectLayerStore) und die Quellenkonfiguration des
// Versionsinventars (OpenVersionSourcesReader): gefüllt aus
// testCollectionDir des Projekts, leer ohne. Wie der echte Weg öffnet der
// Leser nur bei bereiter Anbindung.
func isolateDefaults() func() {
	original := OpenDefaultsStore
	originalProject := OpenProjectLayerStore
	originalSources := OpenVersionSourcesReader
	OpenVersionSourcesReader = func(projectDir string, status kephalaion.Status) (versionsources.Reader, error) {
		if !status.OK {
			return nil, docRefErrorf(DocCodeKephalaionUnavailable,
				"Kephalaion ist für dieses Projekt nicht bereit (%s): %s", status.State, status.Message)
		}
		return kephtest.Collection(filepath.Join(projectDir, testCollectionDir))
	}
	OpenDefaultsStore = func(projectDir string) (DefaultsStore, string, error) {
		store, err := kephtest.Defaults(filepath.Join(PlaybookDir(projectDir), defaultsDirName))
		if err != nil {
			return nil, testDefaultsAddress, err
		}
		return store, testDefaultsAddress, nil
	}
	OpenProjectLayerStore = func(projectDir string) (DefaultsStore, string, error) {
		store, err := kephtest.Collection(filepath.Join(projectDir, testCollectionDir))
		if err != nil {
			return nil, testProjectAddress, err
		}
		return store, testProjectAddress, nil
	}
	return func() {
		OpenDefaultsStore = original
		OpenProjectLayerStore = originalProject
		OpenVersionSourcesReader = originalSources
	}
}

// realOpenProjectLayerStore ist OpenProjectLayerStore, wie das Programm es hat.
var realOpenProjectLayerStore = OpenProjectLayerStore

// testProjectAddress ist die Adresse der Collection des Projekts in Tests.
const testProjectAddress = "test:projekt"

// testCollectionDir ist das Verzeichnis im Projekt eines Tests, aus dem
// isolateDefaults die Collection des Projekts baut:
// test-collection/k-playbook/reviews/x.md ist dort keph:/k-playbook/reviews/x.md.
// Das Programm liest es nie.
const testCollectionDir = "test-collection"

// stubProjectLayerStore setzt für einen Test einen festen Store der
// Collection des Projekts ein.
func stubProjectLayerStore(t *testing.T, open func(projectDir string) (DefaultsStore, string, error)) {
	t.Helper()
	original := OpenProjectLayerStore
	OpenProjectLayerStore = open
	t.Cleanup(func() { OpenProjectLayerStore = original })
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
