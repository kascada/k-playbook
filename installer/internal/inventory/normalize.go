package inventory

import (
	"regexp"
	"strings"
)

// Normalisiert wird nur, was Vergleichbarkeit herstellt. Die Rohangabe bleibt
// immer erhalten — `Entry.Version` ist wortgleich die Quelle.

// exactVersion trifft eine einzelne, vollständige Versionsangabe. Ein führendes
// `v` ist erlaubt, weil Go-Module und Git-Tags es schreiben.
var exactVersion = regexp.MustCompile(`^v?\d+(\.\d+)*([-+._][0-9A-Za-z][0-9A-Za-z.\-+]*)?$`)

// digestValue trifft die beiden Formen, die der Vertrag als Digest gelten
// lässt: `sha256:` mit 64 Hexzeichen und den vollen 40-stelligen Commit-SHA.
// Kurz-SHAs werden nicht verlängert und gelten als `unknown`.
var (
	digestValue = regexp.MustCompile(`^sha256:[0-9a-fA-F]{64}$`)
	commitSHA   = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)
	// shortSHA trifft die abgeschnittenen Formen, die deshalb `unknown` sind.
	shortSHA = regexp.MustCompile(`^[0-9a-fA-F]{7,39}$`)
)

// rangeMarker sind die Zeichen, an denen eine Angabe ein Bereich wird.
const rangeMarker = "^~><=*|,"

// unresolved sind die Formen, in denen eine Quelle auf etwas verweist, das erst
// zur Laufzeit einen Wert bekommt.
func unresolved(value string) bool {
	return strings.Contains(value, "${") || strings.Contains(value, "$(") ||
		strings.Contains(value, "%{") || strings.HasPrefix(value, "$")
}

// classifyPin ordnet eine Versionsangabe in die Taxonomie ein.
//
// `floating` und `unknown` werden nicht verwechselt: `floating` heißt „bewusst
// nicht gepinnt", `unknown` heißt „nicht ermittelbar".
func classifyPin(raw string) string {
	value := strings.TrimSpace(raw)
	switch {
	case value == "" || value == "*" || value == "x" || value == "X":
		return PinFloating
	case unresolved(value):
		return PinUnknown
	case digestValue.MatchString(value) || commitSHA.MatchString(value):
		return PinDigest
	}
	// `==` fällt vor der Bereichsprüfung weg, sonst läse sein `=` als Marker.
	stripped := strings.TrimSpace(strings.TrimPrefix(value, "=="))
	// Vor exact geprüft: `1.2.x` sieht wie eine vollständige Version aus, ist
	// aber ein Bereich.
	if strings.HasSuffix(stripped, ".x") || strings.HasSuffix(stripped, ".*") {
		return PinRange
	}
	if exactVersion.MatchString(stripped) {
		return PinExact
	}
	if strings.ContainsAny(value, rangeMarker) || strings.Contains(value, " - ") {
		return PinRange
	}
	return PinUnknown
}

// normalizeVersion bildet die vergleichbare Fassung. Sie entsteht nur bei
// `pin: exact`: bei einem Bereich wäre sie eine Interpretation, und das
// Inventar interpretiert nicht.
func normalizeVersion(raw string, pin string) string {
	if pin != PinExact {
		return ""
	}
	value := strings.TrimSpace(raw)
	value = strings.TrimSpace(strings.TrimPrefix(value, "=="))
	value = strings.TrimPrefix(value, "v")
	return value
}

// normalizeName bringt den Gegenstand auf seine kanonische Schreibweise.
func normalizeName(ecosystem string, name string) string {
	value := strings.ToLower(strings.TrimSpace(name))
	if ecosystem == EcoPython {
		// PEP 503: `_` und `.` sind im Paketnamen dasselbe wie `-`.
		value = strings.NewReplacer("_", "-", ".", "-").Replace(value)
	}
	return value
}

// groupKey ist der Schlüssel der Abweichungsbildung. Er ist ökosystemlokal: ein
// Python-`redis` und ein Image `redis` sind zwei Gegenstände.
func groupKey(ecosystem string, name string) string {
	return ecosystem + "/" + name
}

// imageReference ist eine zerlegte Container-Image-Referenz.
type imageReference struct {
	Name   string
	Tag    string
	Digest string
}

// parseImage zerlegt `registry/namespace/name:tag@sha256:…`.
//
// Ein fehlendes `docker.io/library` wird nicht ergänzt: sonst sähen zwei
// gleiche Aussagen verschieden aus, je nachdem wer sie geschrieben hat.
func parseImage(reference string) imageReference {
	value := strings.TrimSpace(reference)
	result := imageReference{}
	if at := strings.Index(value, "@"); at >= 0 {
		result.Digest = strings.TrimSpace(value[at+1:])
		value = value[:at]
	}
	// Der Tag-Doppelpunkt steht hinter dem letzten `/`; davor wäre er ein Port.
	if colon := strings.LastIndex(value, ":"); colon >= 0 && colon > strings.LastIndex(value, "/") {
		result.Tag = strings.TrimSpace(value[colon+1:])
		value = value[:colon]
	}
	result.Name = strings.ToLower(strings.TrimSpace(value))
	return result
}

// imageEntry baut den Eintrag zu einer Image-Referenz. Steht neben dem Digest
// noch ein Tag, bleibt er in `version` — die Bindung ist trotzdem der Digest.
func imageEntry(reference string) (name string, version string, pin string, digest string) {
	parsed := parseImage(reference)
	name = parsed.Name
	version = parsed.Tag
	digest = parsed.Digest
	switch {
	case digest != "":
		if !digestValue.MatchString(digest) {
			return name, version, PinUnknown, digest
		}
		return name, version, PinDigest, digest
	case unresolved(reference):
		return name, version, PinUnknown, ""
	case version == "" || movingTag(version):
		return name, version, PinFloating, ""
	}
	// Ein hingeschriebener Tag ist eine Festlegung, auch wenn er keine
	// Versionsnummer ist: `ubuntu-22.04` bindet so fest wie `1.2.3`. Das ist
	// dieselbe Lesart, die der Vertrag für CI-Actions ausdrücklich trifft
	// („ein Tag `exact`"), und sie verhindert ein `unknown` ohne Begründung.
	return name, version, PinExact, ""
}

// movingTag sind die Tags, die ausdrücklich nicht festlegen sollen.
func movingTag(tag string) bool {
	switch strings.ToLower(strings.TrimSpace(tag)) {
	case "latest", "main", "master", "edge", "stable", "nightly", "dev", "devel":
		return true
	}
	return false
}

// requirement ist eine zerlegte Python-Anforderung.
type requirement struct {
	Name string
	// Version ist der Rest hinter dem Namen, wortgleich: Extras und Marker
	// bleiben darin erhalten, gehen aber nicht in den Namen ein.
	Version string
	Pin     string
	Note    string
}

var requirementName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*`)

// parseRequirement liest eine Zeile im Format von requirements.txt oder PEP 508.
func parseRequirement(spec string) (requirement, bool) {
	value := strings.TrimSpace(spec)
	if index := strings.Index(value, " #"); index >= 0 {
		value = strings.TrimSpace(value[:index])
	}
	if value == "" || strings.HasPrefix(value, "#") {
		return requirement{}, false
	}
	if strings.HasPrefix(value, "-e ") || strings.HasPrefix(value, "--editable ") {
		target := strings.TrimSpace(strings.SplitN(value, " ", 2)[1])
		name := lastPathSegment(target)
		if name == "." {
			// `-e .` ist das Projekt selbst. Es hat keinen Gegenstandsnamen und
			// keine Version; als Zeile stünde dort ein Gegenstand ohne Namen.
			return requirement{}, false
		}
		return requirement{Name: normalizeName(EcoPython, name), Pin: PinLocal,
			Note: "Editable-Installation aus " + target}, true
	}
	if strings.HasPrefix(value, "-") {
		// -r, -c, --index-url und Verwandte sind keine Versionsaussagen.
		return requirement{}, false
	}
	match := requirementName.FindString(value)
	if match == "" {
		return requirement{}, false
	}
	rest := strings.TrimSpace(value[len(match):])
	entry := requirement{Name: normalizeName(EcoPython, match), Version: rest}

	constraint := rest
	if semicolon := strings.Index(constraint, ";"); semicolon >= 0 {
		constraint = constraint[:semicolon]
	}
	if bracket := strings.Index(constraint, "["); bracket >= 0 {
		if closing := strings.Index(constraint, "]"); closing > bracket {
			constraint = constraint[:bracket] + constraint[closing+1:]
		}
	}
	constraint = strings.TrimSpace(constraint)
	if strings.HasPrefix(constraint, "@") {
		entry.Pin = PinLocal
		entry.Note = "direkte Referenz statt Version: " + strings.TrimSpace(strings.TrimPrefix(constraint, "@"))
		return entry, true
	}
	entry.Pin = classifyPin(constraint)
	if entry.Pin == PinUnknown && constraint != "" {
		entry.Note = "Versionsangabe nicht deutbar: " + constraint
	}
	return entry, true
}

func lastPathSegment(path string) string {
	value := strings.TrimSuffix(strings.ReplaceAll(path, "\\", "/"), "/")
	if index := strings.LastIndex(value, "/"); index >= 0 {
		value = value[index+1:]
	}
	if value == "" || value == "." {
		return "."
	}
	return value
}

// unknownReason begründet ein `unknown`. Der Vertrag verlangt zu jedem `unknown`
// einen Hinweis, der sagt warum; ohne ihn stünde in der Inventarzeile eine
// Einordnung ohne Grund, und der Leser müsste die Quelle selbst aufschlagen.
func unknownReason(version string) string {
	value := strings.TrimSpace(version)
	switch {
	case value == "":
		return "keine deutbare Versionsangabe gefunden"
	case unresolved(value):
		return "Wert aus Variable, nicht auflösbar: " + value
	case shortSHA.MatchString(value):
		return "Kurz-SHA; er wird nicht verlängert: " + value
	default:
		return "Versionsangabe nicht deutbar: " + value
	}
}
