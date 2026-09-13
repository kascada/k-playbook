package project

import "strings"

// yamlParent ist ein offener Schlüssel über der aktuellen Zeile, mit seiner
// Einrückung.
type yamlParent struct {
	indent int
	key    string
}

// parseYAMLList liest die Liste unter genau einem Schlüsselpfad, zeilenweise
// und ohne YAML-Parser — wie der Rest der Konfiguration, damit die Datei beim
// Zurückschreiben eines anderen Blocks unangetastet bleibt.
//
// Verstanden werden beide Schreibweisen: die Flussform in einer Zeile
// (`languages: [python, go]`) und die Blockform mit Spiegelstrichen. Der Pfad
// wird genau geprüft: `tools.mcp.required` trifft `required` nur als direktes
// Kind von `mcp` unter `tools`, nicht in beliebiger Tiefe darunter.
//
// Zurück kommen die Einträge, nur von Leerraum befreit — Anführungszeichen,
// Groß- und Kleinschreibung und Gültigkeit sind Sache des Aufrufers. Das
// zweite Ergebnis meldet, ob der Schlüssel dastand; eine leere Liste ist
// etwas anderes als keine.
func parseYAMLList(content string, path ...string) ([]string, bool) {
	if len(path) == 0 {
		return []string{}, false
	}

	// Ein Spiegelstrich einer fremden Liste steht als "-" in parents, damit
	// ein gleichnamiger Schlüssel darunter nicht als Treffer gilt.
	var parents []yamlParent
	listIndent := -1
	items := []string{}

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := lineIndent(line)
		dash := strings.HasPrefix(trimmed, "- ") || trimmed == "-"

		// Innerhalb der Blockliste zählen nur Spiegelstriche, die nicht
		// weniger eingerückt sind als der Schlüssel. Alles andere auf gleicher
		// oder geringerer Tiefe beendet sie; tiefer Eingerücktes gehört zu
		// einem Eintrag und wird übergangen.
		if listIndent >= 0 {
			if dash && indent >= listIndent {
				if item := strings.TrimSpace(strings.TrimPrefix(trimmed, "-")); item != "" {
					items = append(items, item)
				}
				continue
			}
			if indent <= listIndent {
				return items, true
			}
			continue
		}

		for len(parents) > 0 && parents[len(parents)-1].indent >= indent {
			parents = parents[:len(parents)-1]
		}
		if dash {
			parents = append(parents, yamlParent{indent: indent, key: "-"})
			continue
		}

		key, value, hasColon := strings.Cut(trimmed, ":")
		if !hasColon {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if strings.HasPrefix(value, "#") {
			value = ""
		}

		depth := len(parents)
		if depth == len(path)-1 && key == path[depth] && yamlParentsMatch(parents, path) {
			if value == "" {
				listIndent = indent
				continue
			}
			return flowListItems(value), true
		}
		if value == "" {
			parents = append(parents, yamlParent{indent: indent, key: key})
		}
	}

	if listIndent >= 0 {
		return items, true
	}
	return []string{}, false
}

// yamlParentsMatch meldet, ob die offenen Schlüssel genau der Anfang des
// Pfads sind.
func yamlParentsMatch(parents []yamlParent, path []string) bool {
	for index, parent := range parents {
		if parent.key != path[index] {
			return false
		}
	}
	return true
}

// flowListItems zerlegt die Flussform `[a, b]` in ihre Einträge.
func flowListItems(value string) []string {
	items := []string{}
	for _, item := range strings.Split(strings.Trim(value, "[]"), ",") {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}
	return items
}
