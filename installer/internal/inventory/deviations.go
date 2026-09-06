package inventory

import (
	"sort"
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
// Zeilen und deren Herkunft.
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
		if len(indexes) < 2 {
			continue
		}
		if sameStatement(entries, indexes) {
			continue
		}
		art := DeviationEnvironmental
		if conflicting(entries, indexes) {
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

// sameStatement: haben alle Zeilen dieselbe version und denselben pin, gibt es
// keine Abweichung.
func sameStatement(entries []Entry, indexes []int) bool {
	first := statement(entries[indexes[0]])
	for _, index := range indexes[1:] {
		if statement(entries[index]) != first {
			return false
		}
	}
	return true
}

func statement(entry Entry) string {
	return entry.Version + "\x00" + entry.Pin
}

// conflicting: tragen zwei abweichende Zeilen denselben Kontext, ist die
// Abweichung widersprüchlich — Manifest gegen Lockfile, zwei Compose-Dateien
// derselben Umgebung, Chart.yaml gegen Chart.lock. Tragen sie verschiedene, ist
// sie umgebungsbedingt und meist Absicht.
func conflicting(entries []Entry, indexes []int) bool {
	perContext := map[string]string{}
	for _, index := range indexes {
		entry := entries[index]
		if previous, seen := perContext[entry.Context]; seen && previous != statement(entry) {
			return true
		}
		perContext[entry.Context] = statement(entry)
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
