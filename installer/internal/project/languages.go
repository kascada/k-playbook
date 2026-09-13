package project

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// DefaultLanguages gilt, solange project.languages fehlt und die Erkennung
// aus den Manifesten (DetectLanguages) leer ausfällt.
//
// Anders als bei tools.gh gibt es hier kein unknown: eine leere Auswahl wäre
// nicht dasselbe wie eine offene Frage, sondern hieße "keine sprachgebundenen
// Tools" — und das ist eine Aussage, die niemand getroffen hat. Python ist die
// Vorauswahl, weil es die häufigste Sprache in den Projekten ist, die
// k-playbook nutzen.
var DefaultLanguages = []string{"python"}

// languageManifest ordnet eine Manifestdatei der Sprache zu, die sie belegt.
type languageManifest struct {
	file     string
	language string
}

// languageManifests sind die Dateien, an denen ein Projekt seine Sprachen
// verrät. Die Reihenfolge ist die der erkannten Liste; ein Projekt mit mehreren
// Manifesten derselben Sprache nennt sie einmal.
//
// tsconfig.json steht eigens: ein TypeScript-Projekt trägt daneben fast
// immer eine package.json, und dann sind beide Sprachen gemeint — die
// Tool-Matrix unterscheidet sie.
var languageManifests = []languageManifest{
	{file: "go.mod", language: "go"},
	{file: "pyproject.toml", language: "python"},
	{file: "setup.py", language: "python"},
	{file: "setup.cfg", language: "python"},
	{file: "Pipfile", language: "python"},
	{file: "requirements.txt", language: "python"},
	{file: "package.json", language: "javascript"},
	{file: "tsconfig.json", language: "typescript"},
}

// DetectLanguages erkennt die Sprachen eines Projekts aus seinen Manifesten,
// begrenzt auf die Sprachen, die die Tool-Matrix kennt. Leer, wenn nichts zu
// erkennen ist — dann gilt DefaultLanguages.
//
// Keine Rekursion. Geprüft wird nur die oberste Ebene von project.repo_root
// und, falls verschieden, die oberste Ebene des Hauptverzeichnisses: ein
// rekursiver Lauf träfe node_modules/, vendor/ und Monorepo-Rauschen und
// liefe bei jedem `k-playbook context` mit. Eine Ausschlussliste braucht es
// so nicht — die beiden k-playbook-Verzeichnisse sind Verzeichnisse, keine
// Manifeste. Ist die Konfiguration nicht lesbar, gilt das Hauptverzeichnis
// allein.
//
// Die Begrenzung auf die Tool-Matrix hat einen Grund: eine Sprache, für die
// kein Tool zuständig ist, wäre in der Auswahl der Oberfläche eine tote
// Option. Ohne lesbare Matrix — keine Installation — steht die Erkennung für
// sich; sie ist dann die einzige Auskunft, die es gibt.
func DetectLanguages(projectDir string) []string {
	dirs := []string{projectDir}
	if config, err := ReadConfig(projectDir); err == nil {
		if repoRoot := RepoRootDir(projectDir, config); filepath.Clean(repoRoot) != filepath.Clean(projectDir) {
			dirs = append([]string{repoRoot}, dirs...)
		}
	}

	detected := []string{}
	for _, manifest := range languageManifests {
		if containsString(detected, manifest.language) {
			continue
		}
		for _, dir := range dirs {
			if fileExists(filepath.Join(dir, manifest.file)) {
				detected = append(detected, manifest.language)
				break
			}
		}
	}

	known, err := ReadToolLanguages(projectDir)
	if err != nil {
		return detected
	}
	languages := []string{}
	for _, language := range detected {
		if containsString(known, language) {
			languages = append(languages, language)
		}
	}
	return languages
}

// languagePattern begrenzt, was als Sprachname in die Konfiguration darf. Der
// Wert wandert als Kommandozeilenargument in das Preflight-Skript, also darf er
// nichts enthalten, was dort eine eigene Bedeutung hätte.
var languagePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9+#-]*$`)

// ValidLanguage meldet, ob ein Sprachname zulässig ist.
func ValidLanguage(language string) bool {
	return languagePattern.MatchString(language)
}

// ReadLanguages liest project.languages. Das zweite Ergebnis meldet, ob der
// Schlüssel überhaupt dastand. Fehlt er, gilt die Erkennung aus den
// Manifesten und, wenn die leer ausfällt, DefaultLanguages — configured bleibt
// in beiden Fällen false: erkannt ist nicht entschieden, geschrieben wird
// erst auf ausdrückliche Wahl.
//
// Die Erkennung sitzt hier und nicht in der Oberfläche, damit Oberfläche,
// `k-playbook context` und der Tool-Preflight dieselbe Antwort geben.
func ReadLanguages(projectDir string) ([]string, bool, error) {
	data, err := os.ReadFile(ConfigPath(projectDir))
	if err != nil {
		return DefaultLanguages, false, err
	}
	languages, configured, err := parseLanguages(string(data))
	if err != nil || configured {
		return languages, configured, err
	}
	if detected := DetectLanguages(projectDir); len(detected) > 0 {
		return detected, false, nil
	}
	return DefaultLanguages, false, nil
}

// parseLanguages liest project.languages über parseYAMLList — Fluss- und
// Blockform — und bereinigt und prüft die Einträge. Geschrieben wird immer die
// Blockform.
func parseLanguages(content string) ([]string, bool, error) {
	items, found := parseYAMLList(content, "project", "languages")
	if !found {
		return DefaultLanguages, false, nil
	}
	languages := []string{}
	for _, item := range items {
		if cleaned := cleanLanguage(item); cleaned != "" {
			languages = append(languages, cleaned)
		}
	}
	for _, language := range languages {
		if !ValidLanguage(language) {
			return DefaultLanguages, true, fmt.Errorf("project.languages enthält den unzulässigen Wert %q; erlaubt sind Kleinbuchstaben, Ziffern und - + #", language)
		}
	}
	return languages, true, nil
}

func cleanLanguage(value string) string {
	return strings.ToLower(strings.TrimSpace(strings.Trim(strings.TrimSpace(value), `"'`)))
}

// SetLanguages schreibt project.languages.
//
// Ersetzt wird nur dieser Schlüssel; repo_root, vcs und alles andere im
// project-Block bleiben unangetastet.
func SetLanguages(projectDir string, languages []string) error {
	cleaned := []string{}
	for _, language := range languages {
		language = cleanLanguage(language)
		if language == "" {
			continue
		}
		if !ValidLanguage(language) {
			return fmt.Errorf("unzulässiger Sprachname: %q", language)
		}
		if !containsString(cleaned, language) {
			cleaned = append(cleaned, language)
		}
	}

	path := ConfigPath(projectDir)
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	updated := replaceNestedBlock(string(data), "project", "languages", languagesBlock(cleaned))
	return os.WriteFile(path, []byte(updated), 0o644)
}

// languagesBlock rendert den Schlüssel samt Erklärung.
//
// Der Kommentar steht bewusst *innerhalb* des Blocks, unter dem Schlüssel:
// alles darüber liegt außerhalb dessen, was beim Schreiben ersetzt wird, und
// würde sich mit jedem Umschalten ein weiteres Mal ansammeln.
func languagesBlock(languages []string) string {
	var builder strings.Builder
	if len(languages) == 0 {
		builder.WriteString("  languages: []\n")
		return builder.String()
	}
	builder.WriteString("  languages:\n")
	builder.WriteString("    # Sie entscheiden, welche Security-Tools gebraucht werden;\n")
	builder.WriteString("    # sprachunabhängige gelten immer.\n")
	for _, language := range languages {
		fmt.Fprintf(&builder, "    - %s\n", language)
	}
	return builder.String()
}

func containsString(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
