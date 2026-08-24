package review

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// modulBaum legt unter einem frischen Verzeichnis die angegebenen Manifeste an.
// Der Pfad ist der des Manifests, mit / getrennt.
func modulBaum(t *testing.T, manifeste ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, pfad := range manifeste {
		voll := filepath.Join(root, filepath.FromSlash(pfad))
		if err := os.MkdirAll(filepath.Dir(voll), 0o755); err != nil {
			t.Fatalf("%s anlegen: %v", pfad, err)
		}
		if err := os.WriteFile(voll, []byte("module beispiel\n"), 0o644); err != nil {
			t.Fatalf("%s schreiben: %v", pfad, err)
		}
	}
	return root
}

func TestFindModulesLiefertSortiertUndRelativ(t *testing.T) {
	root := modulBaum(t, "go.mod", "dienste/b/go.mod", "dienste/a/go.mod")

	module, err := FindModules(root, ManifestGoModule)
	if err != nil {
		t.Fatalf("FindModules: %v", err)
	}
	// Die Wurzel selbst steht als „.", und die Reihenfolge steht fest: aus ihr
	// entstehen die Job-Namen.
	want := []string{".", "dienste/a", "dienste/b"}
	if strings.Join(module, ",") != strings.Join(want, ",") {
		t.Errorf("Module = %v, erwartet %v", module, want)
	}
}

// Kein Modul ist kein Fehler: es fehlt der Gegenstand, nicht das Werkzeug.
func TestFindModulesOhneManifest(t *testing.T) {
	module, err := FindModules(modulBaum(t, "quelle/beispiel.go"), ManifestGoModule)
	if err != nil {
		t.Fatalf("FindModules: %v", err)
	}
	if len(module) != 0 {
		t.Errorf("Module = %v, erwartet keins", module)
	}
}

// Die Ausschlüsse: ausgeliefertes Material, Projekteigenes ohne Code,
// Fremdquellen — und testdata/, wo in Go-Repos echte go.mod-Dateien als
// Prüfmaterial liegen.
func TestFindModulesUeberspringtAusschluesse(t *testing.T) {
	root := modulBaum(t,
		"k-playbook/installer/go.mod",
		"k-playbook-local/tasks/go.mod",
		"installer/internal/review/testdata/kaputt/go.mod",
		"vendor/fremd/go.mod",
		"web/node_modules/paket/go.mod",
		".git/hooks/go.mod",
		"installer/go.mod",
	)

	module, err := FindModules(root, ManifestGoModule)
	if err != nil {
		t.Fatalf("FindModules: %v", err)
	}
	if len(module) != 1 || module[0] != "installer" {
		t.Errorf("Module = %v, erwartet nur [installer]", module)
	}
}

// Ein Manifest, das ein Verzeichnis ist, ist keins.
func TestFindModulesIgnoriertVerzeichnisAlsManifest(t *testing.T) {
	root := modulBaum(t, "seltsam/go.mod/inhalt.txt")

	module, err := FindModules(root, ManifestGoModule)
	if err != nil {
		t.Fatalf("FindModules: %v", err)
	}
	if len(module) != 0 {
		t.Errorf("Module = %v, erwartet keins", module)
	}
}

// Ein Lesefehler wird durchgereicht: nach einer abgebrochenen Suche ist
// unbekannt, ob es ein Modul gibt — ein leeres Ergebnis behauptete, es gebe
// keins.
func TestFindModulesMeldetLesefehler(t *testing.T) {
	root := modulBaum(t, "gesperrt/go.mod")
	gesperrt := filepath.Join(root, "gesperrt")
	if err := os.Chmod(gesperrt, 0o000); err != nil {
		t.Fatalf("Rechte setzen: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(gesperrt, 0o755) })

	if _, err := FindModules(root, ManifestGoModule); err == nil {
		t.Error("die abgebrochene Suche wurde als leeres Ergebnis gemeldet")
	}
}

func TestFindModulesBrauchtVerzeichnisUndManifest(t *testing.T) {
	if _, err := FindModules("", ManifestGoModule); err == nil {
		t.Error("ohne Verzeichnis angenommen")
	}
	if _, err := FindModules(t.TempDir(), ""); err == nil {
		t.Error("ohne Manifest angenommen")
	}
	if _, err := FindModules(filepath.Join(t.TempDir(), "gibtesnicht"), ManifestGoModule); err == nil {
		t.Error("ein fehlendes Verzeichnis wurde als leeres Ergebnis gemeldet")
	}
}

// Der abgeleitete Name wird zu einem Dateinamen unter raw/ und muss deshalb
// ValidEntryName() genügen.
func TestModuleSuffixErgibtGueltigeNamen(t *testing.T) {
	fälle := map[string]string{
		".":              "root",
		"installer":      "installer",
		"dienste/api":    "dienste-api",
		"_intern/modul":  "intern-modul",
		"mit raum/modul": "mitraum-modul",
		"öäü":            "root",
	}
	for modul, want := range fälle {
		if got := moduleSuffix(modul); got != want {
			t.Errorf("moduleSuffix(%q) = %q, erwartet %q", modul, got, want)
		}
		if !ValidEntryName("govulncheck-" + moduleSuffix(modul)) {
			t.Errorf("aus %q entsteht kein gültiger Job-Name", modul)
		}
	}
}

// Zwei Module können auf dasselbe Suffix führen. Ohne Unterscheidung
// überschriebe der zweite Job die Datei des ersten.
func TestJobNameForModuleHaeltNamenAuseinander(t *testing.T) {
	taken := map[string]bool{"govulncheck": true}

	erster := jobNameForModule(taken, "govulncheck", "dienst/api")
	zweiter := jobNameForModule(taken, "govulncheck", "dienst-api")
	if erster != "govulncheck-dienst-api" {
		t.Errorf("erster Name = %q", erster)
	}
	if zweiter == erster {
		t.Fatalf("beide Module heißen %q", erster)
	}
	if !ValidEntryName(zweiter) {
		t.Errorf("zweiter Name %q taugt nicht als Dateiname", zweiter)
	}
}

// Der Punkt der dateibasierten Suche: zwei Manifeste im selben Verzeichnis
// bleiben zwei Gegenstände. Über Verzeichnisse aufgefächert verdeckte das eine
// das andere.
func TestFindPythonManifestsLiefertDateienSortiert(t *testing.T) {
	root := modulBaum(t,
		"requirements.txt", "requirements-dev.txt", "constraints.txt",
		"dienste/b/requirements.txt", "dienste/a/requirements-test.txt",
	)

	manifeste, err := FindPythonManifests(root)
	if err != nil {
		t.Fatalf("FindPythonManifests: %v", err)
	}
	want := []string{
		"constraints.txt",
		"dienste/a/requirements-test.txt",
		"dienste/b/requirements.txt",
		"requirements-dev.txt",
		"requirements.txt",
	}
	if strings.Join(manifeste, ",") != strings.Join(want, ",") {
		t.Errorf("Manifeste = %v, erwartet %v", manifeste, want)
	}
}

// pyproject.toml & Co. bleiben außen vor: pip-audit -r erwartet eine Datei im
// requirements.txt-Format, ein pyproject.toml-Projekt liefe über den
// positionalen project_path-Parameter — ein anderer Aufrufpfad.
func TestFindPythonManifestsNimmtNurRequirementsUndConstraints(t *testing.T) {
	root := modulBaum(t, "pyproject.toml", "setup.py", "setup.cfg", "Pipfile", "poetry.lock", "requirements.txt")

	manifeste, err := FindPythonManifests(root)
	if err != nil {
		t.Fatalf("FindPythonManifests: %v", err)
	}
	if len(manifeste) != 1 || manifeste[0] != "requirements.txt" {
		t.Errorf("Manifeste = %v, erwartet nur requirements.txt", manifeste)
	}
}

// Ausgeschlossen wird über moduleSearchExcluded — dieselbe Liste wie bei der
// Go-Suche, nicht die engere aus candidates.go.
func TestFindPythonManifestsUeberspringtAusschluesse(t *testing.T) {
	root := modulBaum(t,
		"requirements.txt",
		"k-playbook/requirements.txt",
		"k-playbook-local/requirements.txt",
		"vendor/requirements.txt",
		"node_modules/requirements.txt",
		"testdata/requirements.txt",
		".venv/requirements.txt",
	)

	manifeste, err := FindPythonManifests(root)
	if err != nil {
		t.Fatalf("FindPythonManifests: %v", err)
	}
	if len(manifeste) != 1 || manifeste[0] != "requirements.txt" {
		t.Errorf("Manifeste = %v, erwartet nur das Manifest der Wurzel", manifeste)
	}
}

// Ein Verzeichnis, das wie ein Manifest heißt, ist keins.
func TestFindPythonManifestsIgnoriertVerzeichnisAlsManifest(t *testing.T) {
	root := modulBaum(t, "requirements.txt/liste.txt")

	manifeste, err := FindPythonManifests(root)
	if err != nil {
		t.Fatalf("FindPythonManifests: %v", err)
	}
	if len(manifeste) != 0 {
		t.Errorf("Manifeste = %v, erwartet keins", manifeste)
	}
}

// Ein Lesefehler wird durchgereicht: nach einer abgebrochenen Suche ist
// unbekannt, ob es ein Manifest gibt.
func TestFindPythonManifestsMeldetLesefehler(t *testing.T) {
	root := modulBaum(t, "gesperrt/requirements.txt")
	gesperrt := filepath.Join(root, "gesperrt")
	if err := os.Chmod(gesperrt, 0o000); err != nil {
		t.Fatalf("Rechte setzen: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(gesperrt, 0o755) })

	if _, err := FindPythonManifests(root); err == nil {
		t.Error("die abgebrochene Suche wurde als leeres Ergebnis gemeldet")
	}
}

func TestFindPythonManifestsBrauchtVerzeichnis(t *testing.T) {
	if _, err := FindPythonManifests(""); err == nil {
		t.Error("ohne Verzeichnis angenommen")
	}
	if _, err := FindPythonManifests(filepath.Join(t.TempDir(), "gibtesnicht")); err == nil {
		t.Error("ein fehlendes Verzeichnis wurde als leeres Ergebnis gemeldet")
	}
}
