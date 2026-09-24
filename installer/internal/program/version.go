// Package program kümmert sich um die Programmdatei selbst: ob das laufende
// Programm zum Clone passt, ob `k-playbook` im PATH auf das Installationsziel
// zeigt, welche Version eine installierte Datei trägt, und wie das passende
// Release-Programm über den Bootstrap des Clones nachinstalliert wird.
//
// Heruntergeladen wird hier nichts. Der Download ist Sache von
// k-playbook/bin/install, das die Datei gegen SHA256SUMS prüft und atomar
// ersetzt; dieses Paket ruft es nur auf und prüft danach die Datei.
package program

import (
	"strconv"
	"strings"
)

// Order ist das Ergebnis eines Versionsvergleichs aus Sicht der ersten
// Version.
type Order int

const (
	// Unknown: mindestens eine der beiden Versionen ist nicht lesbar. Das ist
	// kein Nachweis für irgendetwas und löst beim laufenden Programm nichts aus.
	Unknown Order = iota
	// Older: die erste Version ist älter als die zweite.
	Older
	// Equal: beide sind gleich.
	Equal
	// Newer: die erste Version ist neuer — der Normalfall im Entwicklungsrepo
	// nach `make dev-install`, wenn der Clone noch dem letzten Release folgt.
	Newer
)

func (o Order) String() string {
	switch o {
	case Older:
		return "älter"
	case Equal:
		return "gleich"
	case Newer:
		return "neuer"
	}
	return "unbekannt"
}

// Compare ordnet version gegenüber reference ein, semantisch nach SemVer.
//
// Warum semantisch und nicht als Zeichenkette: „v0.10.0" ist neuer als
// „v0.9.4", obwohl es lexikalisch davor steht, und genau solche Sprünge
// kommen bei jedem Minor-Release vor.
//
// Warum nicht schlicht „ungleich": im Entwicklungsrepo ist das laufende
// Programm regelmäßig neuer als der Clone. Ein Vergleich auf Ungleichheit
// meldete dort bei jedem Start ein Update und installierte auf Knopfdruck ein
// älteres Programm über das neuere. Deshalb zählt allein „älter".
//
// Gelesen wird die Form, die VERSION und die gestempelte Version tragen:
// optionales „v", drei Zahlen, optional ein Vorabkennzeichen hinter „-".
// Build-Metadaten hinter „+" zählen nach SemVer nicht. Was sich so nicht lesen
// lässt — leer, „dev", eine Commit-Kennung —, ergibt Unknown.
func Compare(version string, reference string) Order {
	a, okA := parse(version)
	b, okB := parse(reference)
	if !okA || !okB {
		return Unknown
	}
	switch c := a.compare(b); {
	case c < 0:
		return Older
	case c > 0:
		return Newer
	}
	return Equal
}

type semver struct {
	core [3]int
	pre  []string
}

func parse(text string) (semver, bool) {
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "v")
	if i := strings.IndexByte(text, '+'); i >= 0 {
		text = text[:i]
	}
	core, pre, hasPre := strings.Cut(text, "-")

	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return semver{}, false
	}
	var result semver
	for i, part := range parts {
		number, ok := parseNumber(part)
		if !ok {
			return semver{}, false
		}
		result.core[i] = number
	}
	if hasPre {
		result.pre = strings.Split(pre, ".")
		for _, identifier := range result.pre {
			if identifier == "" {
				return semver{}, false
			}
		}
	}
	return result, true
}

// parseNumber liest eine nichtnegative Dezimalzahl ohne Vorzeichen.
// strconv.Atoi allein nähme „+1" und „-1" an.
func parseNumber(text string) (int, bool) {
	if text == "" {
		return 0, false
	}
	for _, r := range text {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	number, err := strconv.Atoi(text)
	return number, err == nil
}

func (a semver) compare(b semver) int {
	for i := range a.core {
		if a.core[i] != b.core[i] {
			if a.core[i] < b.core[i] {
				return -1
			}
			return 1
		}
	}
	// Eine Vorabversion ist älter als die Freigabe derselben Nummer.
	switch {
	case len(a.pre) == 0 && len(b.pre) == 0:
		return 0
	case len(a.pre) == 0:
		return 1
	case len(b.pre) == 0:
		return -1
	}
	for i := 0; i < len(a.pre) && i < len(b.pre); i++ {
		if c := compareIdentifier(a.pre[i], b.pre[i]); c != 0 {
			return c
		}
	}
	switch {
	case len(a.pre) < len(b.pre):
		return -1
	case len(a.pre) > len(b.pre):
		return 1
	}
	return 0
}

// compareIdentifier vergleicht zwei Teile eines Vorabkennzeichens nach SemVer:
// Zahlen numerisch, Zahlen vor Text, Text lexikalisch.
func compareIdentifier(a string, b string) int {
	numberA, isNumberA := parseNumber(a)
	numberB, isNumberB := parseNumber(b)
	switch {
	case isNumberA && isNumberB:
		switch {
		case numberA < numberB:
			return -1
		case numberA > numberB:
			return 1
		}
		return 0
	case isNumberA:
		return -1
	case isNumberB:
		return 1
	}
	return strings.Compare(a, b)
}
