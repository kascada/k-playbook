package inventory

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// envIndex ordnet ein Umgebungslabel in die feste Abschnittsreihenfolge ein.
// Ein unbekanntes Label kann hier nicht ankommen — die Quellenkonfiguration
// lehnt es ab, bevor eine Quelle daraus entsteht.
func envIndex(env string) int {
	for index, known := range EnvOrder {
		if known == env {
			return index
		}
	}
	return len(EnvOrder)
}

// sortEntries stellt die stabile Reihenfolge her: Kontexte in der festen
// Reihenfolge, darin nach ecosystem, name, sourceFile, sourceLine.
//
// sourceKey ist der letzte Vergleichsschlüssel und steht nicht im Vertrag: er
// entscheidet die Fälle, in denen zwei Aussagen aus derselben Zeile stammen —
// zwei gepinnte Werkzeuge in einer RUN-Zeile etwa. Ohne ihn entschiede dort die
// Reihenfolge des Parsers, und „zwei Läufe erzeugen dieselbe Datei" wäre nur
// fast wahr.
func sortEntries(entries []Entry) {
	sort.SliceStable(entries, func(left, right int) bool {
		a, b := entries[left], entries[right]
		return entryLess(a, b)
	})
}

func entryLess(a Entry, b Entry) bool {
	if index := envIndex(a.Context) - envIndex(b.Context); index != 0 {
		return index < 0
	}
	for _, pair := range [][2]string{
		{a.Ecosystem, b.Ecosystem},
		{a.Name, b.Name},
		{a.SourceFile, b.SourceFile},
	} {
		if pair[0] != pair[1] {
			return pair[0] < pair[1]
		}
	}
	if a.SourceLine != b.SourceLine {
		return a.SourceLine < b.SourceLine
	}
	if a.SourceKey != b.SourceKey {
		return a.SourceKey < b.SourceKey
	}
	return a.Version < b.Version
}

// buildDeviations bündelt die Aussagen je Gegenstand und markiert die
// beteiligten Einträge.
//
// Eine Abweichung wird nie aufgelöst, zusammengefasst oder auf einen
// „richtigen" Wert reduziert. Sie wird ausgewiesen, mit allen beteiligten
// Zeilen und deren Herkunft. Dass ein stimmiges Paar aus package.json und
// package-lock.json als eine Aussage zählt, gilt nur für die Einteilung; die
// Abweichung trägt trotzdem jede Zeile.
func buildDeviations(entries []Entry) []Deviation {
	groups := map[string][]int{}
	var order []string
	for index, entry := range entries {
		if _, seen := groups[entry.Group]; !seen {
			order = append(order, entry.Group)
		}
		groups[entry.Group] = append(groups[entry.Group], index)
	}
	sort.Strings(order)

	var deviations []Deviation
	for _, group := range order {
		indexes := groups[group]
		units := statements(entries, indexes)
		if len(units) < 2 || sameStatement(units) {
			continue
		}
		art := DeviationEnvironmental
		if conflicting(units) {
			art = DeviationConflicting
		}
		deviation := Deviation{Group: group, Art: art}
		for _, index := range indexes {
			entries[index].Deviation = group
			deviation.Entries = append(deviation.Entries, entries[index])
		}
		sortEntries(deviation.Entries)
		deviations = append(deviations, deviation)
	}

	// widersprüchlich zuerst: das ist der Fall, der eine Frage aufwirft.
	sort.SliceStable(deviations, func(left, right int) bool {
		a, b := deviations[left], deviations[right]
		if a.Art != b.Art {
			return a.Art == DeviationConflicting
		}
		return a.Group < b.Group
	})
	return deviations
}

// statement ist eine Aussage für die Einteilung: eine einzelne Zeile oder ein
// stimmiges Paar. key vergleicht zwei Aussagen; ein Paar ist nie dieselbe
// Aussage wie eine einzelne Zeile.
type statement struct {
	context string
	key     string
}

// statements bildet die Aussagen einer Gruppe.
//
// Ein Paar entsteht nur zwischen einer Zeile aus package-lock.json und allen
// Zeilen desselben Pakets aus dem package.json im selben Verzeichnis und
// demselben Kontext. Stimmig ist es, wenn die Lock-Version jeden Bereich
// erfüllt; dann ist es eine Aussage aus der Menge der Deklarationen und der
// Lock-Version. Verletzt die Lock-Version einen Bereich, bekommt die Lock-Zeile
// den Hinweis und das Paar zerfällt in seine Zeilen — wie auch dann, wenn ein
// Bereich oder die Lock-Version nicht prüfbar ist.
func statements(entries []Entry, indexes []int) []statement {
	paired := map[int]bool{}
	var units []statement
	for _, lockIndex := range indexes {
		lock := entries[lockIndex]
		if lock.Manifest == "" || lock.Ecosystem != EcoNode {
			continue
		}
		var declarations []int
		for _, index := range indexes {
			entry := entries[index]
			if index != lockIndex && entry.Manifest == "" && entry.SourceFile == lock.Manifest && entry.Context == lock.Context {
				declarations = append(declarations, index)
			}
		}
		if len(declarations) == 0 {
			continue
		}
		checkable := true
		var violations []string
		for _, index := range declarations {
			declaration := entries[index]
			satisfied, ok := npmSatisfies(declaration.Version, lock.Version)
			switch {
			case !ok:
				checkable = false
			case !satisfied:
				violations = append(violations, rangeViolation(lock, declaration))
			}
		}
		if len(violations) > 0 {
			note := strings.Join(violations, "; ")
			if entries[lockIndex].Note != "" {
				note = entries[lockIndex].Note + "; " + note
			}
			entries[lockIndex].Note = note
			continue
		}
		if !checkable {
			continue
		}
		declared := make([]string, 0, len(declarations))
		for _, index := range declarations {
			declared = append(declared, rowKey(entries[index]))
			paired[index] = true
		}
		sort.Strings(declared)
		paired[lockIndex] = true
		units = append(units, statement{context: lock.Context,
			key: "paar\x00" + strings.Join(declared, "\x01") + "\x02" + rowKey(lock)})
	}
	for _, index := range indexes {
		if !paired[index] {
			units = append(units, statement{context: entries[index].Context, key: "zeile\x00" + rowKey(entries[index])})
		}
	}
	return units
}

// rangeViolation ist der Hinweis an der Lock-Zeile, wortgleich wie im Vertrag.
func rangeViolation(lock Entry, declaration Entry) string {
	location := declaration.SourceFile
	if declaration.SourceLine > 0 {
		location += ":" + strconv.Itoa(declaration.SourceLine)
	}
	return fmt.Sprintf("Lock-Version %s erfüllt den Range %s nicht (%s, %s)",
		lock.Version, declaration.Version, location, declaration.SourceKey)
}

// rowKey ist die Aussage einer einzelnen Zeile: version und pin.
func rowKey(entry Entry) string {
	return entry.Version + "\x00" + entry.Pin
}

// sameStatement: sagen alle Aussagen dasselbe, gibt es keine Abweichung.
func sameStatement(units []statement) bool {
	for _, unit := range units[1:] {
		if unit.key != units[0].key {
			return false
		}
	}
	return true
}

// conflicting: sagen zwei Aussagen desselben Kontexts Verschiedenes, ist die
// Abweichung widersprüchlich — zwei Compose-Dateien derselben Umgebung,
// Chart.yaml gegen Chart.lock, zwei Manifeste mit verschiedener Deklaration,
// ein Manifest gegen ein Lockfile außerhalb der Paarregel. Tragen sie
// verschiedene Kontexte, ist sie umgebungsbedingt und meist Absicht.
func conflicting(units []statement) bool {
	perContext := map[string]string{}
	for _, unit := range units {
		if previous, seen := perContext[unit.context]; seen && previous != unit.key {
			return true
		}
		perContext[unit.context] = unit.key
	}
	return false
}

func sortSources(sources []SourceRead) {
	sort.SliceStable(sources, func(left, right int) bool {
		if sources[left].File != sources[right].File {
			return sources[left].File < sources[right].File
		}
		return sources[left].Kind < sources[right].Kind
	})
}

func sortRejections(rejections []Rejection) {
	sort.SliceStable(rejections, func(left, right int) bool {
		a, b := rejections[left], rejections[right]
		if a.Requested != b.Requested {
			return a.Requested < b.Requested
		}
		return a.Reason < b.Reason
	})
}

func sortNotes(notes []Note) {
	sort.SliceStable(notes, func(left, right int) bool {
		a, b := notes[left], notes[right]
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		return a.Text < b.Text
	})
}

func contextsPresent(entries []Entry) []string {
	seen := map[string]bool{}
	for _, entry := range entries {
		seen[entry.Context] = true
	}
	var present []string
	for _, env := range EnvOrder {
		if seen[env] {
			present = append(present, env)
		}
	}
	return present
}
