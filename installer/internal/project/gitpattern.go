package project

import (
	"regexp"
	"strings"
)

// Muster im Stil einer .gitignore, so weit der Scope der Docs-Erzeuger sie
// braucht (Task 179): * und ? innerhalb eines Pfadteils, ** über Pfadteile,
// [a-z]-Klassen, ein Schrägstrich am Ende (nur Verzeichnisse), ein
// Schrägstrich am Anfang oder in der Mitte (verankert), ! (wieder
// einschließen) und # (Kommentar). Kein Anspruch auf jede Eigenheit von git:
// verschachtelte .gitignore gelten nach bestem Vermögen.

// ignoreRule ist ein übersetztes Muster.
type ignoreRule struct {
	source  string
	re      *regexp.Regexp
	negate  bool
	dirOnly bool
}

// compileIgnoreRule übersetzt ein Muster. ok ist false für leere Zeilen,
// Kommentare und Muster, die sich nicht übersetzen lassen. unanchored lässt
// ein Muster auch dann auf jeder Ebene greifen, wenn es einen Schrägstrich in
// der Mitte trägt (tests/fixtures/).
func compileIgnoreRule(pattern string, unanchored bool) (ignoreRule, bool) {
	text := strings.TrimRight(pattern, " \t\r\n")
	if strings.TrimSpace(text) == "" || strings.HasPrefix(text, "#") {
		return ignoreRule{}, false
	}
	rule := ignoreRule{source: strings.TrimSpace(pattern)}
	if strings.HasPrefix(text, "\\#") || strings.HasPrefix(text, "\\!") {
		text = text[1:]
	} else if strings.HasPrefix(text, "!") {
		rule.negate = true
		text = text[1:]
	}
	if strings.HasSuffix(text, "/") {
		rule.dirOnly = true
		text = strings.TrimRight(text, "/")
	}
	if text == "" {
		return ignoreRule{}, false
	}
	anchored := strings.Contains(text, "/") && !unanchored
	text = strings.TrimPrefix(text, "/")
	var b strings.Builder
	b.WriteString("^")
	if !anchored && !strings.HasPrefix(text, "**/") {
		b.WriteString("(?:.*/)?")
	}
	for i := 0; i < len(text); i++ {
		c := text[i]
		switch {
		case c == '*' && strings.HasPrefix(text[i:], "**/"):
			b.WriteString("(?:.*/)?")
			i += 2
		case c == '*' && strings.HasPrefix(text[i:], "**") && i+2 == len(text):
			b.WriteString(".*")
			i++
		case c == '*':
			b.WriteString("[^/]*")
		case c == '?':
			b.WriteString("[^/]")
		case c == '[':
			end := strings.IndexByte(text[i+1:], ']')
			if end < 0 {
				b.WriteString(`\[`)
				continue
			}
			class := text[i+1 : i+1+end]
			class = strings.ReplaceAll(class, `\`, `\\`)
			if strings.HasPrefix(class, "!") {
				class = "^" + class[1:]
			}
			b.WriteString("[" + class + "]")
			i += end + 1
		case c == '\\' && i+1 < len(text):
			i++
			b.WriteString(regexp.QuoteMeta(string(text[i])))
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	b.WriteString("$")
	re, err := regexp.Compile(b.String())
	if err != nil {
		return ignoreRule{}, false
	}
	rule.re = re
	return rule, true
}

// compileIgnoreRules übersetzt eine Liste und lässt Unbrauchbares aus.
func compileIgnoreRules(patterns []string, unanchored bool) []ignoreRule {
	rules := make([]ignoreRule, 0, len(patterns))
	for _, pattern := range patterns {
		if rule, ok := compileIgnoreRule(pattern, unanchored); ok {
			rules = append(rules, rule)
		}
	}
	return rules
}

// ignoreVerdict wertet die Regeln für einen Pfad aus, relativ und mit
// Schrägstrichen. Ein Pfad ist ausgenommen, wenn er oder ein Verzeichnis
// darüber ausgenommen ist; die letzte passende Regel entscheidet. Zurück
// kommt die Regel, die ausgenommen hat, sonst false.
func ignoreVerdict(rules []ignoreRule, rel string, isDir bool) (ignoreRule, bool) {
	parts := strings.Split(strings.Trim(rel, "/"), "/")
	for i := range parts {
		prefix := strings.Join(parts[:i+1], "/")
		dir := i < len(parts)-1 || isDir
		var hit ignoreRule
		excluded := false
		for _, rule := range rules {
			if rule.dirOnly && !dir {
				continue
			}
			if rule.re.MatchString(prefix) {
				excluded = !rule.negate
				hit = rule
			}
		}
		if excluded {
			return hit, true
		}
	}
	return ignoreRule{}, false
}
