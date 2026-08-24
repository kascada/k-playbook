package review

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// ManifestGoModule ist das Manifest, an dem ein Go-Modul erkannt wird.
//
// Welches Manifest gesucht wird, ist ein Parameter von FindModules und keine
// Eigenschaft der Suche: pip-audit steht später vor derselben Frage, und der
// Mechanismus soll dann nicht neu entstehen.
const ManifestGoModule = "go.mod"

// moduleSearchExcluded sind Verzeichnisse, in denen ein Manifest kein Modul
// des Projekts bezeichnet. Der Ausschluss gilt über den Verzeichnisnamen,
// gleich auf welcher Ebene er auftaucht.
//
// Anders als die Ausschlüsse der Scan-Jobs steht diese Liste im Code und nicht
// in scanners.tsv. Dort stehen sie, weil sie zum Aufruf eines Werkzeugs
// gehören und jedes Werkzeug sie anders schreibt. Die Modulsuche ruft kein
// Werkzeug auf — sie sucht die Gegenstände, über die anschließend aufgefächert
// wird. Es gibt keinen Aufruf, in dem sie stehen könnte.
var moduleSearchExcluded = map[string]bool{
	// Die Installationskopie ist ein eigener Clone mit eigenem Modul. Ihr Code
	// ist ausgeliefertes Material, nicht Code des Projekts.
	"k-playbook": true,
	// Ergebnisse, Tasks und Notizen — hier ganz und nicht nur results/ wie in
	// den Scans: Projektcode liegt dort keiner, ein Modul hätte nichts zu
	// suchen.
	"k-playbook-local": true,
	// Eingebundene Fremdquellen. Ihr Manifest gehört dem, von dem sie stammen.
	"vendor":       true,
	"node_modules": true,
	// Der Fall, der leicht durchrutscht: in Go-Repos liegen unter testdata/
	// echte go.mod-Dateien als Prüfmaterial. Ein Werkzeug darauf loszulassen
	// prüfte die Testdaten, nicht das Projekt.
	"testdata": true,
}

// FindModules liefert die Verzeichnisse unter root, die manifest enthalten —
// relativ zu root, mit / getrennt, die Wurzel selbst als „.".
//
// Sortiert, damit zwei Läufe über denselben Baum dieselbe Reihenfolge und
// damit dieselben Job-Namen ergeben.
//
// Ein Lesefehler wird durchgereicht und nicht übergangen: nach einer
// abgebrochenen Suche ist gerade unbekannt, ob es ein Modul gibt — ein leeres
// Ergebnis behauptete, es gebe keins.
func FindModules(root string, manifest string) ([]string, error) {
	if root == "" {
		return nil, errors.New("kein Verzeichnis angegeben")
	}
	if manifest == "" {
		return nil, errors.New("kein Manifest angegeben")
	}

	found := []string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		if path != root && skipModuleDir(entry.Name()) {
			return fs.SkipDir
		}

		// Stat statt eines zweiten Verzeichniseintrags: ein Manifest, das ein
		// Verzeichnis ist, ist keins.
		info, err := os.Stat(filepath.Join(path, manifest))
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}

		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		found = append(found, filepath.ToSlash(relative))
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(found)
	return found, nil
}

// pythonManifestPatterns sind die Dateinamensmuster, an denen ein
// Python-Manifest für pip-audit erkannt wird. Sie laufen gegen den Dateinamen,
// nicht gegen den Pfad (path.Match), und decken damit requirements.txt und
// requirements-dev.txt gemeinsam ab — die Namen sind nicht abzählbar.
//
// pyproject.toml, setup.py, Pipfile und poetry.lock stehen bewusst nicht
// dabei: pip-audit -r erwartet eine Datei im requirements.txt-Format, während
// ein pyproject.toml-Projekt über den positionalen project_path-Parameter
// geprüft wird. Das ist ein anderer Aufrufpfad, der pip installieren und
// auflösen lässt — Netzwerkzugriff und eine andere Laufzeit als bei -r — und
// verdient eine eigene Abwägung (Task 026, Abschnitt „Kontext").
var pythonManifestPatterns = []string{"requirements*.txt", "constraints*.txt"}

// FindPythonManifests liefert die Python-Manifeste unter root — relativ zu
// root, mit / getrennt und sortiert.
//
// Anders als FindModules liefert sie Dateien und keine Verzeichnisse, und das
// ist der Unterschied, um den es geht: Go kennt ein go.mod je Verzeichnis, in
// einem Python-Verzeichnis liegen requirements.txt und requirements-dev.txt
// nebeneinander. Über Verzeichnisse aufgefächert verdeckte das eine das
// andere; über Dateien bekommt jedes Manifest seinen eigenen Job.
//
// Deshalb ist das hier eine eigene Funktion und kein Parameter von
// FindModules: dort ist der Rückgabewert ein Verzeichnis, in das ein Job
// hineinwechselt (WorkdirModule), hier ist er ein Pfad, den das Werkzeug als
// Argument bekommt (WorkdirModuleFile, siehe scanners.go).
//
// Ausgeschlossen wird wie bei der Go-Suche über moduleSearchExcluded und
// skipModuleDir — nicht über die engere Liste aus candidates.go: gesucht
// werden die Gegenstände des Projekts, nicht was ein Werkzeug hätte sehen
// können.
//
// Ein Lesefehler wird durchgereicht und nicht übergangen — aus demselben
// Grund wie bei FindModules: nach einer abgebrochenen Suche ist gerade
// unbekannt, ob es ein Manifest gibt, und ein leeres Ergebnis behauptete, es
// gebe keins.
func FindPythonManifests(root string) ([]string, error) {
	if root == "" {
		return nil, errors.New("kein Verzeichnis angegeben")
	}

	found := []string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path != root && skipModuleDir(entry.Name()) {
				return fs.SkipDir
			}
			return nil
		}
		// Nur reguläre Dateien: ein Symlink zeigt entweder auf etwas, das
		// ohnehin schon gefunden ist, oder aus dem Ziel heraus.
		if !entry.Type().IsRegular() || !matchesPythonManifest(entry.Name()) {
			return nil
		}

		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		found = append(found, filepath.ToSlash(relative))
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(found)
	return found, nil
}

// matchesPythonManifest meldet, ob ein Dateiname eines der Muster trifft.
//
// Der Fehler von path.Match trifft nur ein fehlerhaftes Muster; die Muster
// stehen hier im Code und werden vom Test abgedeckt.
func matchesPythonManifest(name string) bool {
	for _, pattern := range pythonManifestPatterns {
		if matched, err := path.Match(pattern, name); err == nil && matched {
			return true
		}
	}
	return false
}

// skipModuleDir meldet, ob ein Verzeichnis für die Modulsuche ausfällt.
//
// Verzeichnisse mit führendem Punkt fallen als Gruppe heraus: dort liegen
// Versionsverwaltung und Werkzeug-Caches, kein Projektcode. Sie einzeln
// aufzuzählen hieße, .git, .venv und was sonst noch kommt nachzupflegen.
func skipModuleDir(name string) bool {
	return strings.HasPrefix(name, ".") || moduleSearchExcluded[name]
}

// jobNameForModule leitet den Job-Namen eines aufgefächerten Aufrufs ab und
// hält ihn von den schon vergebenen frei.
//
// Der Name wird zu einem Dateinamen unter raw/ und muss ValidEntryName()
// genügen. Zwei Module können auf dasselbe Suffix führen — a/b und a-b etwa —,
// und dann überschriebe der zweite Job die Datei des ersten. taken wächst mit:
// der Aufrufer reicht dieselbe Menge durch alle Module eines Eintrags.
func jobNameForModule(taken map[string]bool, job string, module string) string {
	name := job + "-" + moduleSuffix(module)
	unique := name
	for round := 2; taken[unique]; round++ {
		unique = fmt.Sprintf("%s-%d", name, round)
	}
	taken[unique] = true
	return unique
}

// moduleSuffix macht aus einem Modulpfad ein Namensstück: Pfadtrenner werden
// zu -, alles, was ValidEntryName() nicht zulässt, fällt weg.
func moduleSuffix(module string) string {
	allowed := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		case r == '.' || r == '_' || r == '-':
			return r
		case r == '/':
			return '-'
		default:
			return -1
		}
	}, module)

	// Vorn verlangt ValidEntryName() einen Buchstaben oder eine Ziffer.
	allowed = strings.TrimLeft(allowed, "._-")
	if allowed == "" {
		// Die Wurzel selbst („.") und Pfade, von denen nichts Zulässiges übrig
		// bleibt, brauchen trotzdem einen Namen.
		return "root"
	}
	return allowed
}
