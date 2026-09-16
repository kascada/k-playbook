package inventory

import (
	"regexp"
	"strconv"
	"strings"
)

// Die npm-Range-Prüfung für die Paarregel aus package.json und
// package-lock.json. Sie folgt der Semantik des npm-Pakets `semver` ohne
// Optionen und kommt ohne Abhängigkeit aus.
//
// Sie ist die einzige Stelle, an der das Inventar einen Bereich auswertet, und
// sie entscheidet nur über die Einteilung einer Abweichung. `version`,
// `versionNormalized` und `pin` einer Zeile bleiben davon unberührt.
//
// Was sie nicht sicher lesen kann, meldet sie als nicht prüfbar, statt zu
// raten: npm-Aliase, workspace:, link:, file:, Git-URLs, Kurzformen wie
// owner/repo, Tarball-URLs, Dist-Tags wie `latest` und jede andere Angabe, die
// kein gültiger Bereich ist.

// semver ist eine vollständige Version. Build-Metadaten werden verworfen: sie
// zählen für den Vergleich nicht.
type semver struct {
	major, minor, patch uint64
	pre                 []string
}

var semverPattern = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)

// parseSemver liest eine vollständige Version wie `3.17.1` oder `1.0.0-beta.2`.
func parseSemver(raw string) (semver, bool) {
	match := semverPattern.FindStringSubmatch(strings.TrimSpace(raw))
	if match == nil {
		return semver{}, false
	}
	var version semver
	var err error
	if version.major, err = strconv.ParseUint(match[1], 10, 64); err != nil {
		return semver{}, false
	}
	if version.minor, err = strconv.ParseUint(match[2], 10, 64); err != nil {
		return semver{}, false
	}
	if version.patch, err = strconv.ParseUint(match[3], 10, 64); err != nil {
		return semver{}, false
	}
	if match[4] != "" {
		version.pre = strings.Split(match[4], ".")
	}
	return version, true
}

// compareSemver vergleicht nach den Regeln von SemVer 2.0: eine Version ohne
// Vorabkennung ist größer als dieselbe mit; Kennungen aus Ziffern werden
// numerisch verglichen und sind kleiner als alphanumerische.
func compareSemver(a, b semver) int {
	for _, pair := range [][2]uint64{{a.major, b.major}, {a.minor, b.minor}, {a.patch, b.patch}} {
		if pair[0] != pair[1] {
			if pair[0] < pair[1] {
				return -1
			}
			return 1
		}
	}
	switch {
	case len(a.pre) == 0 && len(b.pre) == 0:
		return 0
	case len(a.pre) == 0:
		return 1
	case len(b.pre) == 0:
		return -1
	}
	for index := 0; index < len(a.pre) && index < len(b.pre); index++ {
		left, right := a.pre[index], b.pre[index]
		if left == right {
			continue
		}
		leftNumber, leftErr := strconv.ParseUint(left, 10, 64)
		rightNumber, rightErr := strconv.ParseUint(right, 10, 64)
		switch {
		case leftErr == nil && rightErr == nil:
			if leftNumber < rightNumber {
				return -1
			}
			return 1
		case leftErr == nil:
			return -1
		case rightErr == nil:
			return 1
		case left < right:
			return -1
		default:
			return 1
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

// comparator ist eine einzelne Bedingung. any trifft jede Version bis auf die
// Vorabregel; op ist `>=`, `>`, `<`, `<=` oder `=`.
type comparator struct {
	any     bool
	op      string
	version semver
}

func (c comparator) test(version semver) bool {
	if c.any {
		return true
	}
	cmp := compareSemver(version, c.version)
	switch c.op {
	case ">=":
		return cmp >= 0
	case ">":
		return cmp > 0
	case "<":
		return cmp < 0
	case "<=":
		return cmp <= 0
	default:
		return cmp == 0
	}
}

// none trifft keine Version: `>1.x` und `<1.x` ohne Hauptversion, wie npm sie
// auflöst.
var none = comparator{op: "<", version: semver{pre: []string{"0"}}}

// npmRange sind die Alternativen (`||`), jede eine Menge von Bedingungen, die
// alle gelten müssen.
type npmRange [][]comparator

// npmSatisfies prüft eine Lock-Version gegen eine Deklaration aus
// package.json. checkable ist false, wenn der Bereich oder die Version nicht
// sicher gelesen werden kann; satisfied ist dann ohne Bedeutung.
func npmSatisfies(declaration string, lockVersion string) (satisfied bool, checkable bool) {
	version, ok := parseSemver(lockVersion)
	if !ok {
		return false, false
	}
	parsed, ok := parseNPMRange(declaration)
	if !ok {
		return false, false
	}
	return parsed.test(version), true
}

func (r npmRange) test(version semver) bool {
	for _, set := range r {
		if testSet(set, version) {
			return true
		}
	}
	return false
}

// testSet ist `testSet` aus npm-semver: alle Bedingungen müssen gelten, und
// eine Vorabversion trifft nur, wenn eine Bedingung selbst eine Vorabversion
// derselben major.minor.patch nennt. Deshalb trifft `*` keine `1.0.0-beta`
// und `^1.2.3` keine `2.0.0-beta`.
func testSet(set []comparator, version semver) bool {
	for _, condition := range set {
		if !condition.test(version) {
			return false
		}
	}
	if len(version.pre) == 0 {
		return true
	}
	for _, condition := range set {
		if condition.any || len(condition.version.pre) == 0 {
			continue
		}
		allowed := condition.version
		if allowed.major == version.major && allowed.minor == version.minor && allowed.patch == version.patch {
			return true
		}
	}
	return false
}

// uncheckablePrefixes sind die Formen, die eine Deklaration ausdrücklich zu
// etwas anderem als einem Bereich machen. Sie werden vor dem Lesen erkannt,
// damit nichts davon zufällig als Bereich durchgeht.
var uncheckablePrefixes = []string{"npm:", "workspace:", "link:", "file:", "portal:", "patch:",
	"git:", "git+", "github:", "gitlab:", "bitbucket:", "gist:", "http:", "https:"}

// parseNPMRange liest einen npm-Bereich. false heißt: nicht prüfbar.
func parseNPMRange(raw string) (npmRange, bool) {
	value := strings.TrimSpace(raw)
	for _, prefix := range uncheckablePrefixes {
		if strings.HasPrefix(strings.ToLower(value), prefix) {
			return nil, false
		}
	}
	// Eine URL, eine Kurzform owner/repo oder ein Pfad ist kein Bereich.
	if strings.ContainsAny(value, "/:#@\\") {
		return nil, false
	}
	var result npmRange
	for _, alternative := range strings.Split(value, "||") {
		set, ok := parseAlternative(strings.TrimSpace(alternative))
		if !ok {
			return nil, false
		}
		result = append(result, set)
	}
	return result, true
}

// operatorSpace fasst `>= 1.2.3` zu `>=1.2.3` zusammen, wie npm es tut.
var operatorSpace = regexp.MustCompile(`(<=|>=|~>|<|>|=|~|\^)\s+`)

// hyphenRange trifft `1.2.3 - 2.3.4`.
var hyphenRange = regexp.MustCompile(`^(\S+)\s+-\s+(\S+)$`)

func parseAlternative(text string) ([]comparator, bool) {
	if text == "" {
		return []comparator{{any: true}}, true
	}
	text = operatorSpace.ReplaceAllString(text, "$1")
	if match := hyphenRange.FindStringSubmatch(text); match != nil {
		return parseHyphen(match[1], match[2])
	}
	var set []comparator
	for _, token := range strings.Fields(text) {
		conditions, ok := parseToken(token)
		if !ok {
			return nil, false
		}
		set = append(set, conditions...)
	}
	return set, true
}

// partial ist eine Versionsangabe, in der Teile fehlen oder `x`/`*` sind.
type partial struct {
	major, minor, patch string
	pre                 string
}

var partialPattern = regexp.MustCompile(`^v?(\d+|[xX*])(?:\.(\d+|[xX*])(?:\.(\d+|[xX*])(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?)?)?$`)

func parsePartial(text string) (partial, bool) {
	match := partialPattern.FindStringSubmatch(text)
	if match == nil {
		return partial{}, false
	}
	value := partial{major: match[1], minor: match[2], patch: match[3], pre: match[4]}
	// Eine Vorabkennung gehört nur an eine vollständige Version.
	if value.pre != "" && (isX(value.major) || isX(value.minor) || isX(value.patch)) {
		return partial{}, false
	}
	return value, true
}

func isX(part string) bool {
	return part == "" || part == "x" || part == "X" || part == "*"
}

func number(part string) (uint64, bool) {
	value, err := strconv.ParseUint(part, 10, 64)
	return value, err == nil
}

// build baut eine Version aus Zahlen und einer optionalen Vorabkennung.
func build(major, minor, patch uint64, pre string) semver {
	version := semver{major: major, minor: minor, patch: patch}
	if pre != "" {
		version.pre = strings.Split(pre, ".")
	}
	return version
}

// numbers liefert die vorhandenen Teile als Zahlen; ein x-Teil ist 0.
func (p partial) numbers() (major, minor, patch uint64, ok bool) {
	if major, ok = number(p.major); !ok {
		return 0, 0, 0, false
	}
	if !isX(p.minor) {
		if minor, ok = number(p.minor); !ok {
			return 0, 0, 0, false
		}
	}
	if !isX(p.patch) {
		if patch, ok = number(p.patch); !ok {
			return 0, 0, 0, false
		}
	}
	return major, minor, patch, true
}

func parseToken(token string) ([]comparator, bool) {
	switch {
	case strings.HasPrefix(token, "~>"):
		return tilde(token[2:])
	case strings.HasPrefix(token, "~"):
		return tilde(token[1:])
	case strings.HasPrefix(token, "^"):
		return caret(token[1:])
	}
	op := ""
	for _, candidate := range []string{">=", "<=", ">", "<", "="} {
		if strings.HasPrefix(token, candidate) {
			op = candidate
			break
		}
	}
	rest := token[len(op):]
	if rest == "" {
		return nil, false
	}
	return xRange(op, rest)
}

// tilde ist `replaceTilde` aus npm-semver: `~1.2.3` heißt
// `>=1.2.3 <1.3.0-0`, `~1.2` `>=1.2.0 <1.3.0-0`, `~1` `>=1.0.0 <2.0.0-0`.
func tilde(text string) ([]comparator, bool) {
	value, ok := parsePartial(text)
	if !ok {
		return nil, false
	}
	if isX(value.major) {
		return []comparator{{any: true}}, true
	}
	major, minor, patch, ok := value.numbers()
	if !ok {
		return nil, false
	}
	if isX(value.minor) {
		return []comparator{
			{op: ">=", version: build(major, 0, 0, "")},
			{op: "<", version: build(major+1, 0, 0, "0")},
		}, true
	}
	if isX(value.patch) {
		patch = 0
	}
	return []comparator{
		{op: ">=", version: build(major, minor, patch, value.pre)},
		{op: "<", version: build(major, minor+1, 0, "0")},
	}, true
}

// caret ist `replaceCaret` aus npm-semver: die erste Stelle ungleich 0 bleibt
// fest — `^1.2.3` heißt `>=1.2.3 <2.0.0-0`, `^0.2.3` `>=0.2.3 <0.3.0-0`,
// `^0.0.3` `>=0.0.3 <0.0.4-0`.
func caret(text string) ([]comparator, bool) {
	value, ok := parsePartial(text)
	if !ok {
		return nil, false
	}
	if isX(value.major) {
		return []comparator{{any: true}}, true
	}
	major, minor, patch, ok := value.numbers()
	if !ok {
		return nil, false
	}
	switch {
	case isX(value.minor):
		return []comparator{
			{op: ">=", version: build(major, 0, 0, "")},
			{op: "<", version: build(major+1, 0, 0, "0")},
		}, true
	case isX(value.patch):
		upper := build(major+1, 0, 0, "0")
		if major == 0 {
			upper = build(major, minor+1, 0, "0")
		}
		return []comparator{{op: ">=", version: build(major, minor, 0, "")}, {op: "<", version: upper}}, true
	}
	upper := build(major+1, 0, 0, "0")
	if major == 0 {
		upper = build(0, minor+1, 0, "0")
		if minor == 0 {
			upper = build(0, 0, patch+1, "0")
		}
	}
	return []comparator{{op: ">=", version: build(major, minor, patch, value.pre)}, {op: "<", version: upper}}, true
}

// xRange ist `replaceXRange` aus npm-semver: Vergleiche und exakte Angaben mit
// fehlenden oder x-Teilen.
func xRange(op string, text string) ([]comparator, bool) {
	value, ok := parsePartial(text)
	if !ok {
		return nil, false
	}
	anyX := isX(value.major) || isX(value.minor) || isX(value.patch)
	if op == "=" && anyX {
		op = ""
	}
	if isX(value.major) {
		if op == ">" || op == "<" {
			return []comparator{none}, true
		}
		return []comparator{{any: true}}, true
	}
	major, minor, patch, ok := value.numbers()
	if !ok {
		return nil, false
	}
	switch {
	case op != "" && anyX:
		if isX(value.minor) {
			minor = 0
		}
		patch = 0
		pre := ""
		switch op {
		case ">":
			op = ">="
			if isX(value.minor) {
				major, minor = major+1, 0
			} else {
				minor++
			}
		case "<=":
			op = "<"
			if isX(value.minor) {
				major++
			} else {
				minor++
			}
		}
		if op == "<" {
			pre = "0"
		}
		return []comparator{{op: op, version: build(major, minor, patch, pre)}}, true
	case isX(value.minor):
		return []comparator{
			{op: ">=", version: build(major, 0, 0, "")},
			{op: "<", version: build(major+1, 0, 0, "0")},
		}, true
	case isX(value.patch):
		return []comparator{
			{op: ">=", version: build(major, minor, 0, "")},
			{op: "<", version: build(major, minor+1, 0, "0")},
		}, true
	}
	if op == "" {
		op = "="
	}
	return []comparator{{op: op, version: build(major, minor, patch, value.pre)}}, true
}

// parseHyphen ist `hyphenReplace` aus npm-semver: `1.2.3 - 2.3.4` heißt
// `>=1.2.3 <=2.3.4`, mit denselben Regeln für fehlende Teile.
func parseHyphen(fromText string, toText string) ([]comparator, bool) {
	from, ok := parsePartial(fromText)
	if !ok {
		return nil, false
	}
	to, ok := parsePartial(toText)
	if !ok {
		return nil, false
	}
	var set []comparator
	if !isX(from.major) {
		major, minor, patch, ok := from.numbers()
		if !ok {
			return nil, false
		}
		switch {
		case isX(from.minor):
			set = append(set, comparator{op: ">=", version: build(major, 0, 0, "")})
		case isX(from.patch):
			set = append(set, comparator{op: ">=", version: build(major, minor, 0, "")})
		default:
			set = append(set, comparator{op: ">=", version: build(major, minor, patch, from.pre)})
		}
	}
	if !isX(to.major) {
		major, minor, patch, ok := to.numbers()
		if !ok {
			return nil, false
		}
		switch {
		case isX(to.minor):
			set = append(set, comparator{op: "<", version: build(major+1, 0, 0, "0")})
		case isX(to.patch):
			set = append(set, comparator{op: "<", version: build(major, minor+1, 0, "0")})
		default:
			set = append(set, comparator{op: "<=", version: build(major, minor, patch, to.pre)})
		}
	}
	if len(set) == 0 {
		set = append(set, comparator{any: true})
	}
	return set, true
}
