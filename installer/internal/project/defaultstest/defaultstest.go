// Package defaultstest schirmt Tests anderer Pakete von der öffentlichen
// Collection k-playbook am Node dieses Rechners ab: Isolate setzt
// project.OpenDefaultsStore auf einen Store im Speicher (kephtest), gefüllt
// beim Öffnen aus dem Baum, den ein Test unter k-playbook/defaults/ seines
// Projekts anlegt — dem Lieferumfang, wie der Release ihn packt. Gelesen wird
// über den Store, nicht über die Platte. Nur Tests importieren das Paket; das
// Paket project selbst hat dafür eine eigene Fassung (defaults_test_helper_test.go),
// weil es dieses Paket nicht importieren kann.
package defaultstest

import (
	"fmt"
	"path/filepath"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
	"github.com/kascada/k-playbook/installer/internal/kephalaion/kephtest"
	"github.com/kascada/k-playbook/installer/internal/project"
	"github.com/kascada/k-playbook/installer/internal/versionsources"
)

// Address ist die Adresse der öffentlichen Collection in Tests.
const Address = "test:k-playbook"

// ProjectAddress ist die Adresse der Collection des Projekts in Tests.
const ProjectAddress = "test:projekt"

// CollectionDir ist das Verzeichnis im Projekt eines Tests, aus dem Isolate
// die Collection des Projekts baut: test-collection/k-playbook/reviews/x.md
// ist dort keph:/k-playbook/reviews/x.md. Das Programm liest es nie.
const CollectionDir = "test-collection"

// Isolate setzt den Store ein und gibt die Rücknahme zurück — für TestMain.
// Ebenso für die projekteigene Ebene in der Collection des Projekts
// (project.OpenProjectLayerStore): ein Store im Speicher, gefüllt beim Öffnen
// aus CollectionDir des Projekts; fehlt es, ist die Collection leer. Ebenso
// die Quellenkonfiguration des Versionsinventars
// (project.OpenVersionSourcesReader), die wie der echte Weg nur bei bereiter
// Anbindung öffnet.
func Isolate() func() {
	original := project.OpenDefaultsStore
	originalProject := project.OpenProjectLayerStore
	originalSources := project.OpenVersionSourcesReader
	project.OpenVersionSourcesReader = func(projectDir string, status kephalaion.Status) (versionsources.Reader, error) {
		if !status.OK {
			return nil, fmt.Errorf("Kephalaion ist für dieses Projekt nicht bereit (%s): %s", status.State, status.Message)
		}
		return kephtest.Collection(filepath.Join(projectDir, CollectionDir))
	}
	project.OpenDefaultsStore = func(projectDir string) (project.DefaultsStore, string, error) {
		store, err := kephtest.Defaults(filepath.Join(project.PlaybookDir(projectDir), "defaults"))
		if err != nil {
			return nil, Address, err
		}
		return store, Address, nil
	}
	project.OpenProjectLayerStore = func(projectDir string) (project.DefaultsStore, string, error) {
		store, err := kephtest.Collection(filepath.Join(projectDir, CollectionDir))
		if err != nil {
			return nil, ProjectAddress, err
		}
		return store, ProjectAddress, nil
	}
	return func() {
		project.OpenDefaultsStore = original
		project.OpenProjectLayerStore = originalProject
		project.OpenVersionSourcesReader = originalSources
	}
}
