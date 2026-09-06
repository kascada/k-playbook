package inventory

import (
	"sort"
	"strings"
)

// lineFinder findet die Zeile, in der ein JSON-Schlüssel steht.
//
// encoding/json gibt keine Zeilennummern heraus, und die Herkunft eines Fundes
// soll ohne Suche wiederauffindbar sein. Gefunden wird der Schlüssel in seiner
// Anführungszeichen-Schreibweise; ist er mehrdeutig oder gar nicht da, bleibt
// die Zeile leer — dann trägt der SourceKey die Auffindbarkeit allein, wie es
// der Vertrag vorsieht.
type lineFinder struct {
	lines []string
}

func newLineFinder(data []byte) *lineFinder {
	return &lineFinder{lines: strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")}
}

// find liefert die 1-basierte Zeile des ersten Vorkommens von "key" ab der
// Zeile after (1-basiert, 0 heißt: von vorn). 0 bedeutet „nicht gefunden".
func (f *lineFinder) find(key string, after int) int {
	needle := `"` + key + `"`
	start := 0
	if after > 1 {
		// after ist 1-basiert; der Anker selbst darf mitgesucht werden, weil
		// `"engines": { "node": ">=20" }` in einer Zeile stehen kann.
		start = after - 1
	}
	for index := start; index < len(f.lines); index++ {
		if strings.Contains(f.lines[index], needle) {
			return index + 1
		}
	}
	return 0
}

// sortedKeys liefert die Schlüssel einer Map in stabiler Reihenfolge. Das
// Inventar wird deterministisch gerendert; eine Go-Map-Reihenfolge wäre es
// nicht — auch dann nicht, wenn nachher sowieso sortiert wird, denn bei
// gleichen Sortierschlüsseln entschiede sonst der Zufall.
func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// cell macht aus einem Wert eine Markdown-Tabellenzelle: leere Werte werden zum
// Gedankenstrich, ein `|` im Wert würde sonst die Spalte sprengen.
func cell(value string) string {
	value = strings.ReplaceAll(value, "|", "\\|")
	value = strings.ReplaceAll(value, "\n", " ")
	if strings.TrimSpace(value) == "" {
		return "—"
	}
	return value
}

// code setzt einen Wert in Backticks, leere Werte bleiben der Gedankenstrich.
func code(value string) string {
	if strings.TrimSpace(value) == "" {
		return "—"
	}
	return "`" + strings.ReplaceAll(value, "|", "\\|") + "`"
}
