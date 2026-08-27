package merge

import "strings"

// stablePath ist die **eingefrorene** Pfadnormierung der Stable-IDs.
//
// Sie ist eine Kopie der Gruppierungs-Normierung im Stand vom 2026-08-27 und
// ruft absichtlich nichts aus pathnorm auf — weder Normalize noch dessen
// Hilfsfunktionen. Genau diese Aufrufkette hat Task 027 gerissen: dort wurde
// die damals in dedupe.go liegende normalizePath für die *Gruppierung*
// verbessert, und weil stablePrefixAndKey dieselbe Funktion rief, verschoben
// sich unbemerkt die Stable-IDs. Gemessen an einem Lauf mit 74 Gruppen traf es
// 38 davon — allein dadurch, dass das Auflösen der Segmente das führende `/`
// entfernt.
//
// Die Trennung ist der Preis dafür, dass eine Stable-ID hält, was ihr Name
// verspricht: sie darf sich ändern, wenn sich der Befund ändert, nicht wenn
// sich die Schreibweise eines Pfades ändert. pathnorm.Normalize darf und soll
// weiterlaufen — sein Doc-Kommentar nennt selbst, was es noch nicht kann
// (absoluter gegen relativen Pfad unterhalb des Scan-Ziels). Jede solche
// Verbesserung wirkt dann auf Gruppierung und Fundort-Vergleich, ohne den
// Vertrag von review-input.json anzufassen.
//
// **Diese Funktion wird nicht nebenbei geändert.** Eine Änderung hier verschiebt
// jede Stable-ID, deren Pfad davon berührt wird, und entwertet damit alle
// known-decisions-Einträge, die auf `stableId` matchen. Wer sie ändert, trifft
// dieselbe Entscheidung wie dieser Task noch einmal, dokumentiert sie in
// docs/review-runs.md und zieht commands/_review-run/review-input-contract.md
// nach. TestStablePathIstEingefroren hält den Stand fest.
//
// Alle vier Stellen, an denen ein Pfad in Schlüssel oder Klasse einer Gruppe
// eingeht, rufen sie: die Zeilen `locations` und `dependencies` in
// locationKeyLines, die Zeile `paths` in aiKeyLines und aiClassApplies. Eine
// halbe Entkopplung wäre keine — bliebe eine Stelle an pathnorm.Normalize
// hängen, bräche der Vertrag genau dort weiter.
func stablePath(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	path = stablePathTrimScheme(path)
	return strings.ToLower(stablePathCollapse(path))
}

// stablePathTrimScheme schneidet ein `file://`-Präfix samt Authority ab.
// Eingefroren wie stablePath; siehe trimFileScheme in internal/pathnorm für
// die Herleitung.
func stablePathTrimScheme(path string) string {
	if len(path) < len("file://") || !strings.EqualFold(path[:len("file://")], "file://") {
		return path
	}
	rest := path[len("file://"):]
	if slash := strings.Index(rest, "/"); slash >= 0 {
		return rest[slash:]
	}
	return rest
}

// stablePathCollapse löst `.`, `..` und doppelte Slashes auf und entfernt den
// führenden `/`. Eingefroren wie stablePath; siehe collapse in
// internal/pathnorm für die Herleitung, insbesondere den Verzicht auf
// filepath.Clean.
func stablePathCollapse(path string) string {
	rooted := strings.HasPrefix(path, "/")
	segments := []string{}
	for _, segment := range strings.Split(path, "/") {
		switch segment {
		case "", ".":
			continue
		case "..":
			if len(segments) > 0 && segments[len(segments)-1] != ".." {
				segments = segments[:len(segments)-1]
				continue
			}
			if rooted {
				continue
			}
			segments = append(segments, "..")
		default:
			segments = append(segments, segment)
		}
	}
	return strings.Join(segments, "/")
}
