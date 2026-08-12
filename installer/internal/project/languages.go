package project

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// DefaultLanguages gilt, solange project.languages fehlt.
//
// Anders als bei tools.gh gibt es hier kein unknown: eine leere Auswahl waere
// nicht dasselbe wie eine offene Frage, sondern hiesse "keine sprachgebundenen
// Tools" — und das ist eine Aussage, die niemand getroffen hat. Python ist die
// Vorauswahl, weil es die haeufigste Sprache in den Projekten ist, die
// k-playbook nutzen.
var DefaultLanguages = []string{"python"}

// languagePattern begrenzt, was als Sprachname in die Konfiguration darf. Der
// Wert wandert als Kommandozeilenargument in das Preflight-Skript, also darf er
// nichts enthalten, was dort eine eigene Bedeutung haette.
var languagePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9+#-]*$`)

// ValidLanguage meldet, ob ein Sprachname zulaessig ist.
func ValidLanguage(language string) bool {
	return languagePattern.MatchString(language)
}

// ReadLanguages liest project.languages. Das zweite Ergebnis meldet, ob der
// Schluessel ueberhaupt dastand — fehlt er, gilt DefaultLanguages.
func ReadLanguages(projectDir string) ([]string, bool, error) {
	data, err := os.ReadFile(ConfigPath(projectDir))
	if err != nil {
		return DefaultLanguages, false, err
	}
	return parseLanguages(string(data))
}

// parseLanguages liest die Liste zeilenweise, wie der Rest der Konfiguration.
// Beide YAML-Schreibweisen werden verstanden: die Flussform in einer Zeile und
// die Blockform mit Spiegelstrichen. Geschrieben wird immer die Blockform.
func parseLanguages(content string) ([]string, bool, error) {
	inProject := false
	listIndent := -1
	found := false
	languages := []string{}

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := lineIndent(line)

		if indent == 0 {
			// Ein neuer Block auf oberster Ebene beendet die Liste.
			if listIndent >= 0 {
				break
			}
			inProject = trimmed == "project:"
			continue
		}
		if !inProject {
			continue
		}

		if listIndent >= 0 {
			if strings.HasPrefix(trimmed, "- ") || trimmed == "-" {
				if value := cleanLanguage(strings.TrimPrefix(trimmed, "-")); value != "" {
					languages = append(languages, value)
				}
				continue
			}
			// Etwas anderes auf gleicher oder geringerer Tiefe: die Liste ist zu Ende.
			if indent <= listIndent {
				break
			}
			continue
		}

		key, value, hasColon := strings.Cut(trimmed, ":")
		if !hasColon || strings.TrimSpace(key) != "languages" {
			continue
		}
		found = true
		value = strings.TrimSpace(value)
		if value == "" {
			listIndent = indent
			continue
		}
		// Flussform: languages: [python, go]
		for _, item := range strings.Split(strings.Trim(value, "[]"), ",") {
			if cleaned := cleanLanguage(item); cleaned != "" {
				languages = append(languages, cleaned)
			}
		}
		break
	}

	if !found {
		return DefaultLanguages, false, nil
	}
	for _, language := range languages {
		if !ValidLanguage(language) {
			return DefaultLanguages, true, fmt.Errorf("project.languages enthaelt den unzulaessigen Wert %q; erlaubt sind Kleinbuchstaben, Ziffern und - + #", language)
		}
	}
	return languages, true, nil
}

func cleanLanguage(value string) string {
	return strings.ToLower(strings.TrimSpace(strings.Trim(strings.TrimSpace(value), `"'`)))
}

// SetLanguages schreibt project.languages.
//
// Ersetzt wird nur dieser Schluessel; repo_root, vcs und alles andere im
// project-Block bleiben unangetastet.
func SetLanguages(projectDir string, languages []string) error {
	cleaned := []string{}
	for _, language := range languages {
		language = cleanLanguage(language)
		if language == "" {
			continue
		}
		if !ValidLanguage(language) {
			return fmt.Errorf("unzulaessiger Sprachname: %q", language)
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

// languagesBlock rendert den Schluessel samt Erklaerung.
//
// Der Kommentar steht bewusst *innerhalb* des Blocks, unter dem Schluessel:
// alles darueber liegt ausserhalb dessen, was beim Schreiben ersetzt wird, und
// wuerde sich mit jedem Umschalten ein weiteres Mal ansammeln.
func languagesBlock(languages []string) string {
	var builder strings.Builder
	if len(languages) == 0 {
		builder.WriteString("  languages: []\n")
		return builder.String()
	}
	builder.WriteString("  languages:\n")
	builder.WriteString("    # Sie entscheiden, welche Security-Tools gebraucht werden;\n")
	builder.WriteString("    # sprachunabhaengige gelten immer.\n")
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
