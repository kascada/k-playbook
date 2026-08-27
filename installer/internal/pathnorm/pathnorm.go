// Package pathnorm bringt Pfadangaben verschiedener Herkunft auf eine
// Schreibweise.
//
// Es gibt genau eine solche Funktion, und sie liegt hier, weil zwei Pakete
// dieselbe Frage stellen: `review/merge` fragt beim Gruppieren, ob zwei Funde
// an derselben Stelle liegen, und `knowndecisions` fragt beim Matchen, ob ein
// `pathGlob` auf den Fundort passt. Beide lasen früher denselben SARIF-Pfad mit
// je eigener Kopie — und die Kopien sind über die Zeit auseinandergelaufen:
// `knowndecisions` kannte `file://` nur ohne Authority und löste `..`, `./`
// mitten im Pfad und doppelte Slashes gar nicht auf. Wo `merge` zwei Funde
// zusammenzog, traf eine Decision danach nur den einen von beiden — sichtbar
// als Teildeckung einer Gruppe, ohne dass irgendwo etwas fehlschlug.
//
// **Nicht hier liegt die Normierung der Stable-IDs.** `review/merge` hält dafür
// eine eigene, ausdrücklich eingefrorene Kopie (`stablePath`). Das ist kein
// Rückfall in die alte Doppelung, sondern ihr Gegenteil: diese Funktion darf
// sich weiterentwickeln, weil an ihr nur Gruppierung und Matching hängen — beides
// Fragen des aktuellen Laufs. Der Vertrag von `review-input.json` hängt an der
// eingefrorenen Fassung und bleibt davon unberührt. Wer hier etwas verbessert,
// entscheidet in einem zweiten, ausdrücklichen Schritt, ob `stablePath`
// mitziehen soll; siehe `docs/review-runs.md`.
package pathnorm

import "strings"

// Normalize bleibt rein und einparametrig: das Scan-Ziel wird bewusst nicht
// durchgereicht — `merge.GroupFindings` als einzige Einstiegsstelle kennt es
// nicht, und `knowndecisions` normiert auch Glob-Muster, für die es gar kein
// Ziel gibt.
//
// Normiert wird deshalb nur, was ohne Zielkenntnis geht:
//
//   - Backslashes zu `/`,
//   - `file://` weg, mit und ohne Host-Teil (`file:///pfad`, `file://host/pfad`),
//   - `.`- und `..`-Segmente sowie doppelte Slashes auflösen,
//   - führendes `/` entfernen (`/requirements.txt` → `requirements.txt`),
//   - Groß-/Kleinschreibung angleichen.
//
// Was damit ausdrücklich **nicht** zusammenfindet: ein absoluter Pfad unterhalb
// des Scan-Ziels und derselbe Pfad relativ dazu. `/abs/projekt/a.txt` und
// `a.txt` bleiben verschieden, weil ohne das Ziel nicht zu entscheiden ist, wo
// der gemeinsame Teil aufhört. Genau daran scheitert die Zusammenführung von
// osv-scanner mit den übrigen Werkzeugen — der Grund, aus dem das Manifest nicht
// im harten Dependency-Schlüssel steht.
func Normalize(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	path = trimFileScheme(path)
	return strings.ToLower(collapse(path))
}

// trimFileScheme schneidet ein `file://`-Präfix ab. Der Teil zwischen den
// Doppelslashes und dem nächsten `/` ist die Authority und gehört nicht zum
// Pfad; bei `file:///pfad` ist sie leer, bei `file://host/pfad` steht dort ein
// Rechnername. Fehlt ein weiterer `/`, ist die ganze Restzeichenkette der Pfad
// — eine formal falsche, aber vorkommende Schreibweise, bei der Wegwerfen mehr
// schadete als Behalten.
func trimFileScheme(path string) string {
	if len(path) < len("file://") || !strings.EqualFold(path[:len("file://")], "file://") {
		return path
	}
	rest := path[len("file://"):]
	if slash := strings.Index(rest, "/"); slash >= 0 {
		return rest[slash:]
	}
	return rest
}

// collapse löst `.`, `..` und doppelte Slashes auf und liefert den Pfad ohne
// führenden `/`.
//
// Kein filepath.Clean: das arbeitet mit dem Trennzeichen des laufenden Systems
// und ließe eine unter Windows erzeugte SARIF-Datei anders normieren als
// dieselbe unter Linux. Hier ist `/` gesetzt, gleich auf welchem System.
func collapse(path string) string {
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
			// Über die Wurzel hinaus führt kein Weg; ein relativer Pfad behält
			// sein `..`, weil es dort etwas bezeichnet.
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
