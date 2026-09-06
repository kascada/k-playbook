package inventory

import "strings"

// Ein kleiner TOML-Leser für genau das, was in Manifesten und Lockfiles steht:
// Tabellen, Tabellenlisten, Schlüssel-Wert-Paare, Zeichenketten, Listen und
// Inline-Tabellen.
//
// Wie beim YAML-Leser gilt: die Zeilennummer jedes Fundes ist Pflicht, weil sie
// Teil der Herkunft ist. Verschachtelte Werte werden bewusst als Rohtext
// weitergereicht — mehr braucht keiner der Parser, und mehr zu deuten hieße,
// eine Interpretation zu treffen, die das Inventar nicht treffen soll.

type tomlEntry struct {
	Key  string
	Raw  string
	Line int
}

type tomlTable struct {
	Name    string
	Line    int
	Array   bool
	Keys    []string
	Entries map[string]tomlEntry
}

func (t *tomlTable) set(entry tomlEntry) {
	if _, seen := t.Entries[entry.Key]; !seen {
		t.Keys = append(t.Keys, entry.Key)
	}
	t.Entries[entry.Key] = entry
}

func (t *tomlTable) get(key string) (tomlEntry, bool) {
	if t == nil {
		return tomlEntry{}, false
	}
	entry, ok := t.Entries[key]
	return entry, ok
}

type tomlDoc struct {
	Tables []*tomlTable
}

// table liefert die erste Tabelle dieses Namens.
func (d *tomlDoc) table(name string) *tomlTable {
	for _, table := range d.Tables {
		if table.Name == name {
			return table
		}
	}
	return nil
}

// tables liefert alle Tabellen dieses Namens — die Form, in der Lockfiles ihre
// Pakete führen (`[[package]]`).
func (d *tomlDoc) tables(name string) []*tomlTable {
	var found []*tomlTable
	for _, table := range d.Tables {
		if table.Name == name {
			found = append(found, table)
		}
	}
	return found
}

func parseTOML(data []byte) *tomlDoc {
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	doc := &tomlDoc{}
	current := &tomlTable{Entries: map[string]tomlEntry{}}
	doc.Tables = append(doc.Tables, current)

	for index := 0; index < len(lines); index++ {
		trimmed := strings.TrimSpace(stripTOMLComment(lines[index]))
		if trimmed == "" {
			continue
		}
		switch {
		case strings.HasPrefix(trimmed, "[["):
			name := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "[["), "]]"))
			current = &tomlTable{Name: unquoteTOMLKey(name), Line: index + 1, Array: true, Entries: map[string]tomlEntry{}}
			doc.Tables = append(doc.Tables, current)
		case strings.HasPrefix(trimmed, "["):
			name := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "["), "]"))
			current = &tomlTable{Name: unquoteTOMLKey(name), Line: index + 1, Entries: map[string]tomlEntry{}}
			doc.Tables = append(doc.Tables, current)
		default:
			key, value, found := strings.Cut(trimmed, "=")
			if !found {
				continue
			}
			start := index + 1
			accumulated := strings.TrimSpace(value)
			for !tomlBalanced(accumulated) && index+1 < len(lines) {
				index++
				accumulated += "\n" + strings.TrimSpace(stripTOMLComment(lines[index]))
			}
			current.set(tomlEntry{Key: unquoteTOMLKey(strings.TrimSpace(key)), Raw: accumulated, Line: start})
		}
	}
	return doc
}

func stripTOMLComment(line string) string {
	single, double := false, false
	for index := 0; index < len(line); index++ {
		char := line[index]
		switch {
		case single:
			if char == '\'' {
				single = false
			}
		case double:
			if char == '\\' {
				index++
			} else if char == '"' {
				double = false
			}
		case char == '\'':
			single = true
		case char == '"':
			double = true
		case char == '#':
			return line[:index]
		}
	}
	return line
}

func tomlBalanced(value string) bool {
	depth := 0
	single, double := false, false
	for index := 0; index < len(value); index++ {
		char := value[index]
		switch {
		case single:
			if char == '\'' {
				single = false
			}
		case double:
			if char == '\\' {
				index++
			} else if char == '"' {
				double = false
			}
		case char == '\'':
			single = true
		case char == '"':
			double = true
		case char == '[' || char == '{':
			depth++
		case char == ']' || char == '}':
			depth--
		}
	}
	return depth <= 0 && !single && !double
}

func unquoteTOMLKey(key string) string {
	key = strings.TrimSpace(key)
	if len(key) >= 2 && (key[0] == '"' || key[0] == '\'') && key[len(key)-1] == key[0] {
		return key[1 : len(key)-1]
	}
	return key
}

// tomlString liefert den Inhalt einer Zeichenkette. Ist der Wert keine,
// bleibt ok false — dann steht dort eine Liste, eine Inline-Tabelle oder eine
// Zahl.
func tomlString(raw string) (string, bool) {
	value := strings.TrimSpace(raw)
	for _, quote := range []string{`"""`, `'''`} {
		if len(value) >= 6 && strings.HasPrefix(value, quote) && strings.HasSuffix(value, quote) {
			return strings.TrimSpace(value[3 : len(value)-3]), true
		}
	}
	if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
		return value[1 : len(value)-1], true
	}
	return "", false
}

type tomlValue struct {
	Raw  string
	Line int
}

// tomlArray zerlegt eine Liste und hält je Element die Zeile fest, in der es
// steht. Mehrzeilige Listen sind in pyproject.toml der Normalfall.
func tomlArray(raw string, startLine int) []tomlValue {
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, "[") {
		return nil
	}
	inner := trimmed[1:]
	if closing := strings.LastIndex(inner, "]"); closing >= 0 {
		inner = inner[:closing]
	}
	return splitTOMLList(inner, startLine)
}

// tomlInline zerlegt eine Inline-Tabelle in Rohwerte je Schlüssel.
func tomlInline(raw string) map[string]string {
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, "{") {
		return nil
	}
	inner := trimmed[1:]
	if closing := strings.LastIndex(inner, "}"); closing >= 0 {
		inner = inner[:closing]
	}
	fields := map[string]string{}
	for _, part := range splitTOMLList(inner, 0) {
		key, value, found := strings.Cut(part.Raw, "=")
		if !found {
			continue
		}
		fields[unquoteTOMLKey(key)] = strings.TrimSpace(value)
	}
	return fields
}

func splitTOMLList(inner string, startLine int) []tomlValue {
	var values []tomlValue
	var builder strings.Builder
	depth := 0
	single, double := false, false
	line := startLine
	elementLine := startLine
	started := false

	flush := func() {
		text := strings.TrimSpace(builder.String())
		if text != "" {
			values = append(values, tomlValue{Raw: text, Line: elementLine})
		}
		builder.Reset()
		started = false
	}

	for index := 0; index < len(inner); index++ {
		char := inner[index]
		if char == '\n' && !single && !double {
			line++
			builder.WriteByte(' ')
			continue
		}
		if !started && char != ' ' && char != '\t' {
			started = true
			elementLine = line
		}
		switch {
		case single:
			if char == '\'' {
				single = false
			}
		case double:
			if char == '\\' {
				builder.WriteByte(char)
				index++
				if index < len(inner) {
					builder.WriteByte(inner[index])
				}
				continue
			} else if char == '"' {
				double = false
			}
		case char == '\'':
			single = true
		case char == '"':
			double = true
		case char == '[' || char == '{':
			depth++
		case char == ']' || char == '}':
			depth--
		case char == ',' && depth == 0:
			flush()
			continue
		}
		builder.WriteByte(char)
	}
	flush()
	return values
}
