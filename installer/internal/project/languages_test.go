package project

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const configWithoutLanguages = `schema_version: 3

project:
  # Ort des Projekt-Repositorys, relativ zu dieser Datei.
  repo_root: .
  vcs: git

tools:
  gh:
    status: enabled
`

func TestReadLanguagesOhneEintragLiefertVorauswahl(t *testing.T) {
	root := writeConfig(t, configWithoutLanguages)

	languages, configured, err := ReadLanguages(root)
	if err != nil {
		t.Fatalf("ReadLanguages: %v", err)
	}
	if configured {
		t.Error("fehlender Schlüssel wurde als konfiguriert gemeldet")
	}
	if !slices.Equal(languages, DefaultLanguages) {
		t.Errorf("Languages = %v, erwartet %v", languages, DefaultLanguages)
	}
}

func TestReadLanguagesBlockform(t *testing.T) {
	root := writeConfig(t, `schema_version: 3

project:
  repo_root: .
  languages:
    - python
    - go
  vcs: git

tools:
  gh:
    status: enabled
`)

	languages, configured, err := ReadLanguages(root)
	if err != nil {
		t.Fatalf("ReadLanguages: %v", err)
	}
	if !configured {
		t.Error("vorhandener Schlüssel wurde nicht als konfiguriert gemeldet")
	}
	if !slices.Equal(languages, []string{"python", "go"}) {
		t.Errorf("Languages = %v, erwartet [python go]", languages)
	}
}

// Von Hand geschriebene Konfigurationen dürfen die Flussform nutzen, auch wenn
// das Werkzeug selbst die Blockform schreibt.
func TestReadLanguagesFlussform(t *testing.T) {
	root := writeConfig(t, "schema_version: 3\n\nproject:\n  languages: [python, go]\n  vcs: git\n")

	languages, _, err := ReadLanguages(root)
	if err != nil {
		t.Fatalf("ReadLanguages: %v", err)
	}
	if !slices.Equal(languages, []string{"python", "go"}) {
		t.Errorf("Languages = %v, erwartet [python go]", languages)
	}
}

func TestReadLanguagesLeereListe(t *testing.T) {
	root := writeConfig(t, "schema_version: 3\n\nproject:\n  languages: []\n  vcs: git\n")

	languages, configured, err := ReadLanguages(root)
	if err != nil {
		t.Fatalf("ReadLanguages: %v", err)
	}
	if !configured {
		t.Error("leere Liste wurde nicht als konfiguriert gemeldet")
	}
	if len(languages) != 0 {
		t.Errorf("Languages = %v, erwartet leer", languages)
	}
}

// Der Wert wandert als Kommandozeilenargument in das Preflight-Skript. Was dort
// eine eigene Bedeutung hätte, darf gar nicht erst gelesen werden.
func TestReadLanguagesMeldetUnzulaessigenWert(t *testing.T) {
	root := writeConfig(t, "schema_version: 3\n\nproject:\n  languages:\n    - \"go; rm -rf /\"\n")

	if _, _, err := ReadLanguages(root); err == nil {
		t.Error("unzulässiger Sprachname wurde nicht gemeldet")
	}
}

func TestSetLanguagesLaesstDenRestStehen(t *testing.T) {
	root := writeConfig(t, configWithoutLanguages)

	if err := SetLanguages(root, []string{"python", "go"}); err != nil {
		t.Fatalf("SetLanguages: %v", err)
	}

	data, err := os.ReadFile(ConfigPath(root))
	if err != nil {
		t.Fatalf("Konfiguration lesen: %v", err)
	}
	content := string(data)
	for _, want := range []string{"repo_root: .", "vcs: git", "status: enabled", "- python", "- go"} {
		if !strings.Contains(content, want) {
			t.Errorf("%q fehlt in der geschriebenen Konfiguration:\n%s", want, content)
		}
	}

	languages, configured, err := ReadLanguages(root)
	if err != nil {
		t.Fatalf("ReadLanguages: %v", err)
	}
	if !configured || !slices.Equal(languages, []string{"python", "go"}) {
		t.Errorf("Languages = %v (configured %v), erwartet [python go]", languages, configured)
	}
}

// Zweimal Schreiben darf den Block ersetzen und nicht ein zweites Mal anhängen.
func TestSetLanguagesErsetztStattAnzuhaengen(t *testing.T) {
	root := writeConfig(t, configWithoutLanguages)

	if err := SetLanguages(root, []string{"python", "go"}); err != nil {
		t.Fatalf("SetLanguages: %v", err)
	}
	if err := SetLanguages(root, []string{"go"}); err != nil {
		t.Fatalf("SetLanguages: %v", err)
	}

	data, _ := os.ReadFile(ConfigPath(root))
	if count := strings.Count(string(data), "languages:"); count != 1 {
		t.Errorf("%d languages-Blöcke, erwartet 1:\n%s", count, data)
	}

	languages, _, err := ReadLanguages(root)
	if err != nil {
		t.Fatalf("ReadLanguages: %v", err)
	}
	if !slices.Equal(languages, []string{"go"}) {
		t.Errorf("Languages = %v, erwartet [go]", languages)
	}
}

// Wiederholtes Umschalten darf die Datei nicht wachsen lassen: weder durch
// gestapelte Kommentare noch durch das Aufzehren der Leerzeile vor dem nächsten
// Block. Beides ist beim Bauen der Sprachauswahl aufgefallen.
func TestSetLanguagesBleibtStabilBeiWiederholtemSchreiben(t *testing.T) {
	root := writeConfig(t, configWithoutLanguages)

	if err := SetLanguages(root, []string{"python"}); err != nil {
		t.Fatalf("SetLanguages: %v", err)
	}
	first, err := os.ReadFile(ConfigPath(root))
	if err != nil {
		t.Fatalf("Konfiguration lesen: %v", err)
	}

	for range 3 {
		if err := SetLanguages(root, []string{"python"}); err != nil {
			t.Fatalf("SetLanguages: %v", err)
		}
	}
	again, err := os.ReadFile(ConfigPath(root))
	if err != nil {
		t.Fatalf("Konfiguration lesen: %v", err)
	}

	if string(first) != string(again) {
		t.Errorf("Datei hat sich beim erneuten Schreiben verändert:\n--- erst ---\n%s\n--- dann ---\n%s", first, again)
	}
	if !strings.Contains(string(again), "\n\ntools:") {
		t.Errorf("Leerzeile vor dem nächsten Block ging verloren:\n%s", again)
	}
}

// Die leere Auswahl wird als Flussform geschrieben. Sie muss beim nächsten Mal
// wiedergefunden werden, sonst entstünde ein zweiter languages-Schlüssel.
func TestSetLanguagesErsetztAuchDieLeereListe(t *testing.T) {
	root := writeConfig(t, configWithoutLanguages)

	if err := SetLanguages(root, nil); err != nil {
		t.Fatalf("SetLanguages: %v", err)
	}
	if err := SetLanguages(root, []string{"go"}); err != nil {
		t.Fatalf("SetLanguages: %v", err)
	}

	data, _ := os.ReadFile(ConfigPath(root))
	if count := strings.Count(string(data), "languages:"); count != 1 {
		t.Errorf("%d languages-Schlüssel, erwartet 1:\n%s", count, data)
	}

	languages, _, err := ReadLanguages(root)
	if err != nil {
		t.Fatalf("ReadLanguages: %v", err)
	}
	if !slices.Equal(languages, []string{"go"}) {
		t.Errorf("Languages = %v, erwartet [go]", languages)
	}
}

func TestSetLanguagesWeistUnzulaessigenWertAb(t *testing.T) {
	root := writeConfig(t, configWithoutLanguages)

	if err := SetLanguages(root, []string{"go; rm -rf /"}); err == nil {
		t.Error("unzulässiger Sprachname wurde geschrieben")
	}
}

func TestSetLanguagesEntferntDoppelte(t *testing.T) {
	root := writeConfig(t, configWithoutLanguages)

	if err := SetLanguages(root, []string{"python", "Python", " python "}); err != nil {
		t.Fatalf("SetLanguages: %v", err)
	}

	languages, _, err := ReadLanguages(root)
	if err != nil {
		t.Fatalf("ReadLanguages: %v", err)
	}
	if !slices.Equal(languages, []string{"python"}) {
		t.Errorf("Languages = %v, erwartet [python]", languages)
	}
}

// writeToolMatrix legt eine Tool-Matrix in die Installation des Projekts, die
// genau die genannten Sprachen kennt. Ohne sie gäbe es keine Menge, auf die
// die Erkennung begrenzt werden könnte.
func writeToolMatrix(t *testing.T, root string, languages ...string) {
	t.Helper()

	content := "name\tlanguages\ngitleaks\t*\n"
	for _, language := range languages {
		content += "werkzeug-" + language + "\t" + language + "\n"
	}
	writeFile(t, ToolMatrix(root), content)
}

// Ein Projekt ohne project.languages bekommt die Sprachen, die seine Manifeste
// belegen — je Sprache eines, dazu das gemischte Projekt.
func TestDetectLanguagesAusManifesten(t *testing.T) {
	cases := []struct {
		name      string
		manifests []string
		want      []string
	}{
		{name: "Go", manifests: []string{"go.mod"}, want: []string{"go"}},
		{name: "Python über pyproject.toml", manifests: []string{"pyproject.toml"}, want: []string{"python"}},
		{name: "Python über requirements.txt", manifests: []string{"requirements.txt"}, want: []string{"python"}},
		{name: "Python mehrfach belegt, einmal genannt", manifests: []string{"setup.py", "setup.cfg", "Pipfile"}, want: []string{"python"}},
		{name: "JavaScript", manifests: []string{"package.json"}, want: []string{"javascript"}},
		{name: "TypeScript neben JavaScript", manifests: []string{"package.json", "tsconfig.json"}, want: []string{"javascript", "typescript"}},
		{name: "gemischt", manifests: []string{"go.mod", "pyproject.toml", "package.json"}, want: []string{"go", "python", "javascript"}},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			root := writeConfig(t, configWithoutLanguages)
			writeToolMatrix(t, root, "go", "python", "javascript", "typescript")
			for _, name := range test.manifests {
				writeFile(t, filepath.Join(root, name), "")
			}

			if got := DetectLanguages(root); !slices.Equal(got, test.want) {
				t.Errorf("DetectLanguages = %v, erwartet %v", got, test.want)
			}

			languages, configured, err := ReadLanguages(root)
			if err != nil {
				t.Fatalf("ReadLanguages: %v", err)
			}
			if configured {
				t.Error("erkannte Sprachen wurden als konfiguriert gemeldet")
			}
			if !slices.Equal(languages, test.want) {
				t.Errorf("ReadLanguages = %v, erwartet %v", languages, test.want)
			}
		})
	}
}

// Keine Rekursion: ein Manifest in einem Unterverzeichnis zählt nicht. Sonst
// träfe die Erkennung node_modules/, vendor/ und Monorepo-Rauschen.
func TestDetectLanguagesSuchtNichtRekursiv(t *testing.T) {
	root := writeConfig(t, configWithoutLanguages)
	writeToolMatrix(t, root, "go", "python", "javascript")
	writeFile(t, filepath.Join(root, "dienst", "go.mod"), "")
	writeFile(t, filepath.Join(root, "node_modules", "paket", "package.json"), "")

	if got := DetectLanguages(root); len(got) != 0 {
		t.Errorf("DetectLanguages = %v, erwartet nichts", got)
	}
	languages, configured, err := ReadLanguages(root)
	if err != nil {
		t.Fatalf("ReadLanguages: %v", err)
	}
	if configured || !slices.Equal(languages, DefaultLanguages) {
		t.Errorf("ReadLanguages = %v (configured %v), erwartet %v", languages, configured, DefaultLanguages)
	}
}

// Ohne erkennbares Manifest bleibt es bei der Vorauswahl.
func TestDetectLanguagesOhneManifestFaelltAufVorauswahl(t *testing.T) {
	root := writeConfig(t, configWithoutLanguages)
	writeToolMatrix(t, root, "go", "python")
	writeFile(t, filepath.Join(root, "README.md"), "# Projekt\n")

	if got := DetectLanguages(root); len(got) != 0 {
		t.Errorf("DetectLanguages = %v, erwartet nichts", got)
	}
	languages, configured, err := ReadLanguages(root)
	if err != nil {
		t.Fatalf("ReadLanguages: %v", err)
	}
	if configured || !slices.Equal(languages, DefaultLanguages) {
		t.Errorf("ReadLanguages = %v (configured %v), erwartet %v", languages, configured, DefaultLanguages)
	}
}

// Ein gesetztes project.languages sticht die Erkennung: entschieden schlägt
// erkannt.
func TestReadLanguagesKonfiguriertStichtErkennung(t *testing.T) {
	root := writeConfig(t, "schema_version: 3\n\nproject:\n  repo_root: .\n  languages:\n    - go\n  vcs: git\n")
	writeToolMatrix(t, root, "go", "python")
	writeFile(t, filepath.Join(root, "pyproject.toml"), "")

	languages, configured, err := ReadLanguages(root)
	if err != nil {
		t.Fatalf("ReadLanguages: %v", err)
	}
	if !configured {
		t.Error("gesetzter Schlüssel wurde nicht als konfiguriert gemeldet")
	}
	if !slices.Equal(languages, []string{"go"}) {
		t.Errorf("ReadLanguages = %v, erwartet [go]", languages)
	}
}

// Liegt das Repository neben dem Hauptverzeichnis, zählt dessen oberste Ebene
// mit — und nur die.
func TestDetectLanguagesLiestRepoRoot(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "playbook")
	writeFile(t, ConfigPath(root), "schema_version: 3\n\nproject:\n  repo_root: ../code\n  vcs: git\n")
	writeToolMatrix(t, root, "go", "python")
	writeFile(t, filepath.Join(base, "code", "go.mod"), "")
	writeFile(t, filepath.Join(base, "code", "tief", "pyproject.toml"), "")

	if got := DetectLanguages(root); !slices.Equal(got, []string{"go"}) {
		t.Errorf("DetectLanguages = %v, erwartet [go]", got)
	}
}

// Die Tool-Matrix begrenzt das Ergebnis: eine Sprache, für die kein Tool
// zuständig ist, wäre in der Auswahl eine tote Option.
func TestDetectLanguagesBegrenztAufToolMatrix(t *testing.T) {
	root := writeConfig(t, configWithoutLanguages)
	writeToolMatrix(t, root, "python")
	writeFile(t, filepath.Join(root, "go.mod"), "")
	writeFile(t, filepath.Join(root, "pyproject.toml"), "")

	if got := DetectLanguages(root); !slices.Equal(got, []string{"python"}) {
		t.Errorf("DetectLanguages = %v, erwartet [python]", got)
	}
}

// Ohne lesbare Matrix — keine Installation — steht die Erkennung für sich.
func TestDetectLanguagesOhneMatrixBleibtUnbegrenzt(t *testing.T) {
	root := writeConfig(t, configWithoutLanguages)
	writeFile(t, filepath.Join(root, "go.mod"), "")

	if got := DetectLanguages(root); !slices.Equal(got, []string{"go"}) {
		t.Errorf("DetectLanguages = %v, erwartet [go]", got)
	}
}
