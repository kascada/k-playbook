package inventory

import (
	"strings"
	"testing"
)

// lockJSON baut eine package-lock.json mit einem Wurzelpaket und den
// aufgelösten Versionen der direkten Abhängigkeiten.
func lockJSON(section string, versions map[string]string) string {
	var declared, resolved []string
	for _, name := range sortedKeys(versions) {
		declared = append(declared, `"`+name+`":"*"`)
		resolved = append(resolved, `"node_modules/`+name+`":{"version":"`+versions[name]+`"}`)
	}
	return `{"lockfileVersion":3,"packages":{"":{"` + section + `":{` + strings.Join(declared, ",") + `}},` +
		strings.Join(resolved, ",") + `}}`
}

func deviationFor(result Result, group string) *Deviation {
	for index := range result.Deviations {
		if result.Deviations[index].Group == group {
			return &result.Deviations[index]
		}
	}
	return nil
}

func requireDeviation(t *testing.T, result Result, group string, art string, rows int) *Deviation {
	t.Helper()
	deviation := deviationFor(result, group)
	if deviation == nil {
		t.Fatalf("%s: keine Abweichung, erwartet %s mit %d Zeilen", group, art, rows)
	}
	if deviation.Art != art || len(deviation.Entries) != rows {
		t.Fatalf("%s: %s mit %d Zeilen, erwartet %s mit %d: %+v", group, deviation.Art, len(deviation.Entries), art, rows, deviation.Entries)
	}
	return deviation
}

func requireNoDeviation(t *testing.T, result Result, group string) {
	t.Helper()
	if deviation := deviationFor(result, group); deviation != nil {
		t.Fatalf("%s: unerwartete Abweichung %s: %+v", group, deviation.Art, deviation.Entries)
	}
}

// Ein stimmiges Paar ist keine Abweichung; die Zeilen stehen trotzdem beide in
// der Kontexttabelle.
func TestStimmigesPackageLockPaarIstKeineAbweichung(t *testing.T) {
	result := collectFiles(t, map[string]string{
		"package.json":      `{"devDependencies":{"vite":"^4.1.1"}}`,
		"package-lock.json": lockJSON("devDependencies", map[string]string{"vite": "4.1.1"}),
	})

	requireNoDeviation(t, result, "node/vite")
	if len(result.Deviations) != 0 {
		t.Errorf("Abweichungen = %+v", result.Deviations)
	}
	if manifest, lock := find(t, result, "node/vite", "package.json"), find(t, result, "node/vite", "package-lock.json"); manifest.Note != "" || lock.Note != "" {
		t.Errorf("ein stimmiges Paar bekommt keinen Hinweis: %+v / %+v", manifest, lock)
	}
	rendered := Render(result, "2026-09-16T00:00:00Z")
	for _, row := range []string{"| `node/vite` | package | `4.1.1` | exact |", "| `node/vite` | package | `^4.1.1` | range |"} {
		if !strings.Contains(rendered, row) {
			t.Errorf("Zeile %q fehlt in der Kontexttabelle", row)
		}
	}
}

// Eine Lock-Version außerhalb des Bereichs ist immer eine Abweichung, und die
// Lock-Zeile nennt den Grund.
func TestLockVersionAusserhalbDesRangeIstWiderspruechlich(t *testing.T) {
	result := collectFiles(t, map[string]string{
		"app/package.json":      "{\n  \"dependencies\": {\n    \"alpinejs\": \"^3.17.1\"\n  }\n}\n",
		"app/package-lock.json": lockJSON("dependencies", map[string]string{"alpinejs": "3.14.8"}),
	})

	requireDeviation(t, result, "node/alpinejs", DeviationConflicting, 2)
	lock := find(t, result, "node/alpinejs", "app/package-lock.json")
	want := "Lock-Version 3.14.8 erfüllt den Range ^3.17.1 nicht (app/package.json:3, dependencies.alpinejs)"
	if lock.Note != want {
		t.Errorf("Hinweis = %q, erwartet %q", lock.Note, want)
	}
	if !strings.Contains(Render(result, "2026-09-16T00:00:00Z"), want) {
		t.Error("der Hinweis steht nicht in der Inventardatei")
	}
}

// yarn.lock bleibt unverändert: Manifest-Bereich gegen Lock-Version ist eine
// Abweichung, ohne Hinweis.
func TestYarnLockVerhaeltSichUnveraendert(t *testing.T) {
	result := collectFiles(t, map[string]string{
		"package.json": `{"dependencies":{"alpinejs":"^3.17.1"}}`,
		"yarn.lock":    "alpinejs@^3.17.1:\n  version \"3.17.1\"\n",
	})

	deviation := requireDeviation(t, result, "node/alpinejs", DeviationConflicting, 2)
	for _, entry := range deviation.Entries {
		if entry.Note != "" || entry.Manifest != "" {
			t.Errorf("yarn.lock ist nicht Teil der Paarregel: %+v", entry)
		}
	}
}

// pnpm-lock.yaml bleibt unverändert: die Zeile trägt den specifier, derselbe
// Bereich ist keine Abweichung, ein anderer schon.
func TestPnpmLockVerhaeltSichUnveraendert(t *testing.T) {
	lock := func(specifier string) string {
		return "importers:\n  .:\n    dependencies:\n      alpinejs:\n        specifier: " + specifier + "\n        version: 3.17.1\n"
	}

	same := collectFiles(t, map[string]string{
		"package.json":   `{"dependencies":{"alpinejs":"^3.17.1"}}`,
		"pnpm-lock.yaml": lock("^3.17.1"),
	})
	requireNoDeviation(t, same, "node/alpinejs")

	different := collectFiles(t, map[string]string{
		"package.json":   `{"dependencies":{"alpinejs":"^3.17.1"}}`,
		"pnpm-lock.yaml": lock("^3.14.8"),
	})
	deviation := requireDeviation(t, different, "node/alpinejs", DeviationConflicting, 2)
	for _, entry := range deviation.Entries {
		if entry.Note != "" {
			t.Errorf("pnpm-lock.yaml bekommt keinen Range-Hinweis: %+v", entry)
		}
	}
}

// npm-Workspaces bleiben unverändert: packages["<member>"] wird nicht gelesen,
// und das Mitglieds-Manifest ohne eigenes Lockfile hat keinen Partner.
func TestNpmWorkspaceVerhaeltSichUnveraendert(t *testing.T) {
	result := collectFiles(t, map[string]string{
		"package.json":            `{"workspaces":["packages/a"],"dependencies":{"lodash":"^4.17.0"}}`,
		"packages/a/package.json": `{"dependencies":{"lodash":"^4.17.0"}}`,
		"package-lock.json": `{"lockfileVersion":3,"packages":{` +
			`"":{"workspaces":["packages/a"],"dependencies":{"lodash":"^4.17.0"}},` +
			`"packages/a":{"dependencies":{"lodash":"^4.17.0"}},` +
			`"node_modules/lodash":{"version":"4.17.21"}}}`,
	})

	if rows := len(entriesFrom(result, "package-lock.json")); rows != 1 {
		t.Errorf("aus dem Lockfile nur die Wurzel: %d Zeilen", rows)
	}
	deviation := requireDeviation(t, result, "node/lodash", DeviationConflicting, 3)
	for _, entry := range deviation.Entries {
		if entry.Note != "" {
			t.Errorf("kein Range-Hinweis erwartet: %+v", entry)
		}
	}
}

// Mehrere Zeilen eines Pakets aus demselben package.json gehören zu einem
// Paar; jeder Bereich wird gegen die Lock-Version geprüft.
func TestPeerUndDevDeklarationBildenEinPaar(t *testing.T) {
	result := collectFiles(t, map[string]string{
		"package.json":      `{"peerDependencies":{"alpinejs":">=3"},"devDependencies":{"alpinejs":"^3.17.1"}}`,
		"package-lock.json": lockJSON("devDependencies", map[string]string{"alpinejs": "3.17.1"}),
	})
	if rows := len(entriesFrom(result, "package.json")); rows != 2 {
		t.Fatalf("Manifest-Zeilen = %d, erwartet 2", rows)
	}
	requireNoDeviation(t, result, "node/alpinejs")

	violated := collectFiles(t, map[string]string{
		"package.json":      `{"peerDependencies":{"alpinejs":">=4"},"devDependencies":{"alpinejs":"^3.17.1"}}`,
		"package-lock.json": lockJSON("devDependencies", map[string]string{"alpinejs": "3.17.1"}),
	})
	requireDeviation(t, violated, "node/alpinejs", DeviationConflicting, 3)
	lock := find(t, violated, "node/alpinejs", "package-lock.json")
	if !strings.Contains(lock.Note, "erfüllt den Range >=4 nicht") || strings.Contains(lock.Note, "^3.17.1") {
		t.Errorf("Hinweis = %q, erwartet nur den verletzten Bereich", lock.Note)
	}
}

// Zwei stimmige Paare mit verschiedenen Bereichen sind eine Abweichung, auch
// wenn beide auf dieselbe Version aufgelöst sind.
func TestZweiStimmigePaareMitVerschiedenenRanges(t *testing.T) {
	result := collectFiles(t, map[string]string{
		"package.json":                       `{"dependencies":{"alpinejs":"^3.14.8"}}`,
		"package-lock.json":                  lockJSON("dependencies", map[string]string{"alpinejs": "3.17.1"}),
		"theme/static_src/package.json":      `{"dependencies":{"alpinejs":"^3.17.1"}}`,
		"theme/static_src/package-lock.json": lockJSON("dependencies", map[string]string{"alpinejs": "3.17.1"}),
	})
	deviation := requireDeviation(t, result, "node/alpinejs", DeviationConflicting, 4)
	for _, entry := range deviation.Entries {
		if entry.Note != "" {
			t.Errorf("stimmige Paare bekommen keinen Hinweis: %+v", entry)
		}
	}
}

// Zwei stimmige Paare mit demselben Bereich, aber verschiedenen Lock-Versionen
// sind eine Abweichung: die Aussage umfasst die Lock-Version.
func TestZweiStimmigePaareMitVerschiedenenLockVersionen(t *testing.T) {
	result := collectFiles(t, map[string]string{
		"a/package.json":      `{"dependencies":{"alpinejs":"^3.14.8"}}`,
		"a/package-lock.json": lockJSON("dependencies", map[string]string{"alpinejs": "3.14.8"}),
		"b/package.json":      `{"dependencies":{"alpinejs":"^3.14.8"}}`,
		"b/package-lock.json": lockJSON("dependencies", map[string]string{"alpinejs": "3.17.1"}),
	})
	requireDeviation(t, result, "node/alpinejs", DeviationConflicting, 4)

	equal := collectFiles(t, map[string]string{
		"a/package.json":      `{"dependencies":{"alpinejs":"^3.14.8"}}`,
		"a/package-lock.json": lockJSON("dependencies", map[string]string{"alpinejs": "3.17.1"}),
		"b/package.json":      `{"dependencies":{"alpinejs":"^3.14.8"}}`,
		"b/package-lock.json": lockJSON("dependencies", map[string]string{"alpinejs": "3.17.1"}),
	})
	requireNoDeviation(t, equal, "node/alpinejs")
}

// Ein package.json ohne Lockfile gegen ein stimmiges Paar mit demselben
// Bereich bleibt eine Abweichung: seine Auflösung ist unbekannt.
func TestManifestOhneLockfileGegenStimmigesPaar(t *testing.T) {
	result := collectFiles(t, map[string]string{
		"package.json":           `{"dependencies":{"alpinejs":"^3.17.1"}}`,
		"package-lock.json":      lockJSON("dependencies", map[string]string{"alpinejs": "3.17.1"}),
		"docs/demo/package.json": `{"dependencies":{"alpinejs":"^3.17.1"}}`,
	})
	requireDeviation(t, result, "node/alpinejs", DeviationConflicting, 3)
}

// Ein nicht prüfbarer Bereich lässt das Paar in seine Zeilen zerfallen, ohne
// Hinweis — wie vor der Paarregel.
func TestNichtPruefbareRangeBleibtAbweichung(t *testing.T) {
	for _, declaration := range []string{"latest", "github:owner/repo", "npm:other@^1.0.0", "file:../lib"} {
		t.Run(declaration, func(t *testing.T) {
			result := collectFiles(t, map[string]string{
				"package.json":      `{"dependencies":{"thing":"` + declaration + `"}}`,
				"package-lock.json": lockJSON("dependencies", map[string]string{"thing": "1.0.0"}),
			})
			deviation := requireDeviation(t, result, "node/thing", DeviationConflicting, 2)
			for _, entry := range deviation.Entries {
				if entry.Note != "" && entry.Pin != PinUnknown {
					t.Errorf("kein Range-Hinweis erwartet: %+v", entry)
				}
				if strings.Contains(entry.Note, "Lock-Version") {
					t.Errorf("kein Range-Hinweis erwartet: %+v", entry)
				}
			}
		})
	}
}

// Die Paarregel gilt nur in einem Kontext: sind Manifest und Lockfile
// verschiedenen Umgebungen zugeordnet, bleibt es bei zwei Aussagen.
func TestPaarNurImSelbenKontext(t *testing.T) {
	result := collectFiles(t, map[string]string{
		"package.json":      `{"dependencies":{"alpinejs":"^3.17.1"}}`,
		"package-lock.json": lockJSON("dependencies", map[string]string{"alpinejs": "3.17.1"}),
		"version-sources.yaml": "schema_version: 1\nsources:\n" +
			"  - path: package-lock.json\n    kind: node\n    env: ci\n",
	})
	requireDeviation(t, result, "node/alpinejs", DeviationEnvironmental, 2)
}

// Nicht-Node-Paare bleiben unverändert: Manifest gegen Lockfile ist dort eine
// Abweichung, sobald die Zeilen verschieden sind.
func TestNichtNodePaarVerhaeltSichUnveraendert(t *testing.T) {
	result := collectFiles(t, map[string]string{
		"Cargo.toml":     "[dependencies]\nserde = \"1\"\n",
		"Cargo.lock":     "[[package]]\nname = \"serde\"\nversion = \"1.0.200\"\n",
		"pyproject.toml": "[project]\ndependencies = [\"requests>=2.31\"]\n",
		"poetry.lock":    "[[package]]\nname = \"requests\"\nversion = \"2.31.0\"\n",
	})
	requireDeviation(t, result, "rust/serde", DeviationConflicting, 2)
	requireDeviation(t, result, "python/requests", DeviationConflicting, 2)
	for _, entry := range result.Entries {
		if entry.Manifest != "" || strings.Contains(entry.Note, "Lock-Version") {
			t.Errorf("Nicht-Node-Zeile ist nicht Teil der Paarregel: %+v", entry)
		}
	}
}
