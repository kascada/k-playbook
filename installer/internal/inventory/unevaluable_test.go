package inventory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// sourceByFile sucht die Zeile einer Quelle in der Quellentabelle.
func sourceByFile(t *testing.T, result Result, file string) SourceRead {
	t.Helper()
	for _, source := range result.Sources {
		if source.File == file {
			return source
		}
	}
	t.Fatalf("%s steht nicht unter den ausgewerteten Quellen: %+v", file, result.Sources)
	return SourceRead{}
}

// Eine defekte Compose-Datei ist als Ganzes nicht auswertbar. Der Zustand steht
// an der Quelle, die Übersicht zählt ihn, und der Hinweis mit dem Grund bleibt.
func TestDefekteComposeDateiIstNichtAuswertbar(t *testing.T) {
	result := collectFiles(t, map[string]string{
		"docker-compose.yml": "services:\n  web:\n    image: nginx:1\n   kaputt: [\n",
	})

	source := sourceByFile(t, result, "docker-compose.yml")
	if !source.Unevaluable || source.Entries != 0 {
		t.Fatalf("Quelle = %+v, erwartet nicht auswertbar ohne Einträge", source)
	}
	if got := len(result.UnevaluableSources()); got != 1 {
		t.Errorf("nicht auswertbare Quellen = %d, erwartet 1", got)
	}
	if !noteContains(result, "nicht lesbares YAML") {
		t.Errorf("der Hinweis mit dem Grund fehlt: %+v", result.Notes)
	}

	rendered := Render(result, "2026-09-16T00:00:00Z")
	for _, want := range []string{
		"  sources-unevaluable: 1\n",
		"- Nicht auswertbare Quellen: 1\n",
		"| Datei | Quellart | Label | Einträge | Zustand | Note |\n",
		"| `docker-compose.yml` | compose | dev | 0 | nicht auswertbar | — |\n",
		"- `docker-compose.yml`: nicht lesbares YAML",
	} {
		if !strings.Contains(rendered, want) {
			t.Errorf("Inventardatei enthält %q nicht:\n%s", want, rendered)
		}
	}
}

// Ohne packages[""] sind die direkten Abhängigkeiten nicht zu erkennen: das
// Lockfile ist nicht auswertbar, das Manifest daneben sehr wohl.
func TestPackageLockOhneWurzelpaketIstNichtAuswertbar(t *testing.T) {
	result := collectFiles(t, map[string]string{
		"package.json":      `{"dependencies":{"alpinejs":"^3.17.1"}}`,
		"package-lock.json": `{"lockfileVersion":3,"packages":{"node_modules/alpinejs":{"version":"3.17.1"}}}`,
	})

	if lock := sourceByFile(t, result, "package-lock.json"); !lock.Unevaluable {
		t.Errorf("Lockfile = %+v, erwartet nicht auswertbar", lock)
	}
	if manifest := sourceByFile(t, result, "package.json"); manifest.Unevaluable {
		t.Errorf("Manifest = %+v, erwartet ausgewertet", manifest)
	}
	if !noteContains(result, "kein Wurzelpaket") {
		t.Errorf("Hinweis fehlt: %+v", result.Notes)
	}
}

// Fehlt das Manifest eines yarn.lock oder ist es ausgeschlossen, steht der
// Zustand am Lockfile — der Quelle, die deshalb nichts liefert —, nicht am
// Manifest.
func TestYarnLockOhneManifestIstNichtAuswertbar(t *testing.T) {
	yarn := "alpinejs@^3.17.1:\n  version \"3.17.1\"\n"

	t.Run("Manifest fehlt", func(t *testing.T) {
		result := collectFiles(t, map[string]string{"yarn.lock": yarn})
		if lock := sourceByFile(t, result, "yarn.lock"); !lock.Unevaluable {
			t.Errorf("Lockfile = %+v, erwartet nicht auswertbar", lock)
		}
		if !noteContains(result, "zugehöriges Manifest", "fehlt") {
			t.Errorf("Hinweis fehlt: %+v", result.Notes)
		}
	})

	t.Run("Manifest ausgeschlossen", func(t *testing.T) {
		result := collectFiles(t, map[string]string{
			"version-sources.yaml": "schema_version: 1\nexclude:\n  - package.json\n",
			"package.json":         `{"dependencies":{"alpinejs":"^3.17.1"}}`,
			"yarn.lock":            yarn,
		})
		if lock := sourceByFile(t, result, "yarn.lock"); !lock.Unevaluable {
			t.Errorf("Lockfile = %+v, erwartet nicht auswertbar", lock)
		}
		for _, source := range result.Sources {
			if source.File == "package.json" {
				t.Errorf("das ausgeschlossene Manifest darf nicht als Quelle erscheinen: %+v", source)
			}
		}
		if got := len(result.UnevaluableSources()); got != 1 {
			t.Errorf("nicht auswertbare Quellen = %d, erwartet 1", got)
		}
	})
}

// Eine gelesene Datei ohne Fundstellen ist ausgewertet — „nichts gefunden" ist
// eine andere Aussage als „nicht auswertbar".
func TestDateiOhneFundstellenIstAusgewertet(t *testing.T) {
	result := collectFiles(t, map[string]string{
		".github/workflows/leer.yml": "name: leer\non: push\njobs: {}\n",
	})

	source := sourceByFile(t, result, ".github/workflows/leer.yml")
	if source.Unevaluable || source.Entries != 0 {
		t.Errorf("Quelle = %+v, erwartet ausgewertet ohne Einträge", source)
	}
	rendered := Render(result, "2026-09-16T00:00:00Z")
	if !strings.Contains(rendered, "| `.github/workflows/leer.yml` | ci | ci | 0 | ausgewertet | — |\n") {
		t.Errorf("Zustandszelle fehlt:\n%s", rendered)
	}
	if !strings.Contains(rendered, "- Nicht auswertbare Quellen: 0\n") {
		t.Errorf("Zähler fehlt:\n%s", rendered)
	}
}

// Ein inhaltlicher Hinweis ist kein Auswertungsfehler: die Quelle wurde
// ausgewertet, der Hinweis betrifft eine Aussage darin.
func TestInhaltlicherHinweisSetztKeinenZustand(t *testing.T) {
	result := collectFiles(t, map[string]string{
		"setup.py": "from setuptools import setup\nsetup(name='x', install_requires=REQUIREMENTS)\n",
	})

	if !noteContains(result, "install_requires") {
		t.Fatalf("der inhaltliche Hinweis fehlt: %+v", result.Notes)
	}
	if source := sourceByFile(t, result, "setup.py"); source.Unevaluable {
		t.Errorf("Quelle = %+v, erwartet ausgewertet", source)
	}
}

// JSON-Ergebnis, Datei und Status tragen denselben Zustand und dieselbe Zahl.
func TestZustandNichtAuswertbarIstInJSONUndDateiGleich(t *testing.T) {
	options := newRunProject(t)
	options.Now = func() time.Time { return time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC) }
	if err := os.WriteFile(filepath.Join(options.ProjectDir, "compose.yaml"), []byte("services: [\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, _, err := Run(options)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Sources []struct {
			File        string `json:"file"`
			Unevaluable bool   `json:"unevaluable"`
		} `json:"sources"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	jsonCount := 0
	for _, source := range decoded.Sources {
		if source.Unevaluable {
			jsonCount++
			if source.File != "compose.yaml" {
				t.Errorf("falsche Quelle im JSON: %+v", source)
			}
		}
	}

	status := ReadStatus(options.InventoryFile)
	if status.Problem != "" {
		t.Fatalf("Frontmatter: %s", status.Problem)
	}
	if jsonCount != 1 || status.SourcesUnevaluable != 1 || len(result.UnevaluableSources()) != 1 {
		t.Errorf("JSON %d, Frontmatter %d, Ergebnis %d — erwartet überall 1",
			jsonCount, status.SourcesUnevaluable, len(result.UnevaluableSources()))
	}
}

// Ein Bestand ohne den Schlüssel sources-unevaluable ist unvollständig und wird
// beim nächsten Lauf neu geschrieben, statt still 0 zu melden.
func TestFrontmatterOhneUnevaluableIstUnvollstaendig(t *testing.T) {
	rendered := Render(Result{}, "2026-09-16T00:00:00Z")
	old := strings.Replace(rendered, "  sources-unevaluable: 0\n", "", 1)
	status := Status{}
	fillStatus(&status, []byte(old))
	if !strings.Contains(status.Problem, "inventory.sources-unevaluable") {
		t.Errorf("Problem = %q", status.Problem)
	}
}
