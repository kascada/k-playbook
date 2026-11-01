package project

import (
	"strconv"
	"strings"
)

// Lesen in Teilen (Task 104): ein Zeilenbereich oder ein Abschnitt nach
// Überschrift aus einer Einzeldatei, nach dem Einsetzen der Includes. Beides
// steht in DocReadOptions und schließt sich aus.

// DocCodeSectionNotFound: Die Überschrift von DocReadOptions.Section kommt im
// Dokument nicht vor — außerhalb von Code-Blöcken und Frontmatter.
const DocCodeSectionNotFound = "section_not_found"

// docLineRange ist ein Zeilenbereich, 1-basiert, einschließlich; to 0 heißt
// bis zum Ende.
type docLineRange struct {
	from, to int
}

// parseDocLines liest einen Zeilenbereich von-bis oder von-.
func parseDocLines(text string) (docLineRange, error) {
	text = strings.TrimSpace(text)
	fail := func() (docLineRange, error) {
		return docLineRange{}, docRefErrorf(DocCodeInvalidInput,
			"lines %q: erwartet von-bis oder von- (1-basiert, einschließlich), etwa 10-20 oder 10-", text)
	}
	fromText, toText, found := strings.Cut(text, "-")
	if !found {
		return fail()
	}
	from, err := strconv.Atoi(fromText)
	if err != nil || from < 1 {
		return fail()
	}
	r := docLineRange{from: from}
	if toText != "" {
		to, err := strconv.Atoi(toText)
		if err != nil || to < from {
			return fail()
		}
		r.to = to
	}
	return r, nil
}

// docLines zerlegt einen Inhalt in Zeilen samt Zeilenumbruch; ein Umbruch am
// Ende eröffnet keine leere letzte Zeile.
func docLines(content string) []string {
	lines := strings.SplitAfter(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// docLineCount ist die Zahl der Zeilen eines Inhalts.
func docLineCount(content string) int {
	return len(docLines(content))
}

// sliceDocLines liefert die Zeilen des Bereichs, Byte für Byte. Ein Bereich
// über das Ende hinaus endet am Ende; beginnt er dahinter, ist er leer.
func sliceDocLines(content string, r docLineRange) string {
	lines := docLines(content)
	if r.from > len(lines) {
		return ""
	}
	to := r.to
	if to == 0 || to > len(lines) {
		to = len(lines)
	}
	return strings.Join(lines[r.from-1:to], "")
}

// docHeadingLevel ist die Ebene einer ATX-Überschrift (# bis ######, danach
// Leerraum oder Zeilenende); 0 ohne Überschrift.
func docHeadingLevel(line string) int {
	line = strings.TrimRight(line, "\r\n")
	n := 0
	for n < len(line) && line[n] == '#' {
		n++
	}
	if n == 0 || n > 6 {
		return 0
	}
	if n < len(line) && line[n] != ' ' && line[n] != '\t' {
		return 0
	}
	return n
}

// validateDocSection prüft, dass section eine Überschriftenzeile ist.
func validateDocSection(section string) (string, error) {
	heading := strings.TrimSpace(section)
	if docHeadingLevel(heading) == 0 {
		return "", docRefErrorf(DocCodeInvalidInput,
			"section %q ist keine Überschriftenzeile — erwartet etwa „## Intent“, genau wie sie im Dokument steht", section)
	}
	return heading, nil
}

// docSection liefert den Abschnitt ab der ersten Zeile, die genau heading ist,
// bis vor die nächste Überschrift gleicher oder höherer Ebene. Überschriften
// in Code-Blöcken und im Frontmatter zählen nicht. ok ist false ohne Treffer.
func docSection(content, heading string) (string, bool) {
	lines := docLines(content)
	level := docHeadingLevel(heading)
	start, stop := -1, len(lines)
	var fenceChar byte
	fenceLen := 0
	for i := frontmatterLines(lines); i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\r\n")
		if char, n, rest := fenceMarker(line); n > 0 {
			switch {
			case fenceLen == 0:
				fenceChar, fenceLen = char, n
			case char == fenceChar && n >= fenceLen && strings.TrimSpace(rest) == "":
				fenceLen = 0
			}
			continue
		}
		if fenceLen > 0 {
			continue
		}
		lineLevel := docHeadingLevel(line)
		if lineLevel == 0 {
			continue
		}
		if start < 0 {
			if strings.TrimRight(line, " \t") == heading {
				start = i
			}
			continue
		}
		if lineLevel <= level {
			stop = i
			break
		}
	}
	if start < 0 {
		return "", false
	}
	return strings.Join(lines[start:stop], ""), true
}
