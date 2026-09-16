package webui

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// Der Wächter hält die Gestaltung der Oberfläche an einer Stelle: Farben,
// Schrift, Radien, Schatten und Abstände stehen als Variablen in :root, der
// Rest des Stylesheets verweist nur per var(--…) darauf. Welche Ausnahmen
// gelten und warum Breiten und Rahmenstärken bewusst ungeprüft bleiben, steht
// in installer/docs/architecture.md.
const styleRules = `installer/docs/architecture.md, Abschnitt „Gestaltung der Oberfläche"`

type styleViolation struct {
	line  int
	prop  string
	value string
	why   string
}

func (v styleViolation) String() string {
	return fmt.Sprintf("styles.css:%d: %s: %s (%s)", v.line, v.prop, v.value, v.why)
}

type styleDeclaration struct {
	line  int
	prop  string
	value string
}

var (
	styleRootBlock     = regexp.MustCompile(`:root\s*\{[^}]*\}`)
	styleHexColor      = regexp.MustCompile(`#[0-9a-fA-F]{3,8}\b`)
	styleColorFunction = regexp.MustCompile(`(?i)\b(rgba?|hsla?|hwb|lab|lch|oklab|oklch|color|color-mix)\(`)
	styleLength        = regexp.MustCompile(`(^|[^\w-])-?(\d+\.?\d*|\.\d+)(px|rem|em)\b`)
	styleVariable      = regexp.MustCompile(`var\(--[\w-]+\)`)
	styleWord          = regexp.MustCompile(`-*[a-z][a-z-]*`)
	styleSpacingProp   = regexp.MustCompile(`^(padding|margin|gap|row-gap|column-gap|scroll-margin|scroll-padding)(-[a-z-]+)?$`)
)

// Diese Eigenschaften tragen nur Verweise auf Variablen. Daneben sind die
// CSS-weiten Schlüsselwörter erlaubt und je Eigenschaft die hier genannten.
var styleVariableOnly = map[string][]string{
	"font":           {"inherit"},
	"font-size":      nil,
	"font-family":    nil,
	"font-weight":    nil,
	"line-height":    nil,
	"letter-spacing": nil,
	"border-radius":  {"0"},
	"box-shadow":     {"none"},
}

var styleWideKeywords = map[string]bool{"inherit": true, "initial": true, "unset": true, "revert": true}

var styleNamedColors = map[string]bool{}

func init() {
	for _, name := range strings.Fields(`aliceblue antiquewhite aqua aquamarine azure beige bisque black
		blanchedalmond blue blueviolet brown burlywood cadetblue chartreuse chocolate coral
		cornflowerblue cornsilk crimson cyan darkblue darkcyan darkgoldenrod darkgray darkgreen
		darkgrey darkkhaki darkmagenta darkolivegreen darkorange darkorchid darkred darksalmon
		darkseagreen darkslateblue darkslategray darkslategrey darkturquoise darkviolet deeppink
		deepskyblue dimgray dimgrey dodgerblue firebrick floralwhite forestgreen fuchsia gainsboro
		ghostwhite gold goldenrod gray green greenyellow grey honeydew hotpink indianred indigo
		ivory khaki lavender lavenderblush lawngreen lemonchiffon lightblue lightcoral lightcyan
		lightgoldenrodyellow lightgray lightgreen lightgrey lightpink lightsalmon lightseagreen
		lightskyblue lightslategray lightslategrey lightsteelblue lightyellow lime limegreen linen
		magenta maroon mediumaquamarine mediumblue mediumorchid mediumpurple mediumseagreen
		mediumslateblue mediumspringgreen mediumturquoise mediumvioletred midnightblue mintcream
		mistyrose moccasin navajowhite navy oldlace olive olivedrab orange orangered orchid
		palegoldenrod palegreen paleturquoise palevioletred papayawhip peachpuff peru pink plum
		powderblue purple rebeccapurple red rosybrown royalblue saddlebrown salmon sandybrown
		seagreen seashell sienna silver skyblue slateblue slategray slategrey snow springgreen
		steelblue tan teal thistle tomato turquoise violet wheat white whitesmoke yellow
		yellowgreen`) {
		styleNamedColors[name] = true
	}
}

// blankStyle ersetzt alles außer Zeilenumbrüchen durch Leerzeichen, damit die
// Zeilennummern des übrigen Stylesheets stimmen.
func blankStyle(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c != '\n' {
			b[i] = ' '
		}
	}
	return string(b)
}

func stripStyleComments(css string) string {
	var out strings.Builder
	for {
		start := strings.Index(css, "/*")
		if start < 0 {
			out.WriteString(css)
			return out.String()
		}
		end := strings.Index(css[start+2:], "*/")
		if end < 0 {
			out.WriteString(css[:start])
			out.WriteString(blankStyle(css[start:]))
			return out.String()
		}
		end += start + 4
		out.WriteString(css[:start])
		out.WriteString(blankStyle(css[start:end]))
		css = css[end:]
	}
}

// styleDeclarations zerlegt das Stylesheet in Deklarationen samt Zeile. Was vor
// einer öffnenden Klammer steht, ist ein Selektor oder eine @-Regel wie der Kopf
// von @media und wird nicht geprüft.
func styleDeclarations(css string) []styleDeclaration {
	var out []styleDeclaration
	line, depth, start, startLine := 1, 0, 0, 1
	for i := 0; i < len(css); i++ {
		switch css[i] {
		case '\n':
			line++
		case '{':
			depth++
			start, startLine = i+1, line
		case '}', ';':
			if depth > 0 {
				if d, ok := parseStyleDeclaration(css[start:i], startLine); ok {
					out = append(out, d)
				}
			}
			if css[i] == '}' {
				depth--
			}
			start, startLine = i+1, line
		}
	}
	return out
}

func parseStyleDeclaration(chunk string, line int) (styleDeclaration, bool) {
	lead := len(chunk) - len(strings.TrimLeft(chunk, " \t\r\n"))
	line += strings.Count(chunk[:lead], "\n")
	text := strings.TrimSpace(chunk)
	colon := strings.Index(text, ":")
	if text == "" || colon < 0 {
		return styleDeclaration{}, false
	}
	prop := strings.ToLower(strings.TrimSpace(text[:colon]))
	value := strings.Join(strings.Fields(text[colon+1:]), " ")
	value = strings.TrimSpace(strings.TrimSuffix(value, "!important"))
	return styleDeclaration{line: line, prop: prop, value: value}, true
}

func checkStyleDeclaration(d styleDeclaration) []styleViolation {
	var found []styleViolation
	add := func(why string) {
		found = append(found, styleViolation{line: d.line, prop: d.prop, value: d.value, why: why})
	}
	plain := styleVariable.ReplaceAllString(d.value, "")

	if styleHexColor.MatchString(plain) || styleColorFunction.MatchString(plain) || hasNamedStyleColor(plain) {
		add("feste Farbe")
	}
	if keywords, ok := styleVariableOnly[d.prop]; ok && !onlyStyleVariables(plain, keywords) {
		add("ohne var(--…)")
	}
	if styleSpacingProp.MatchString(d.prop) && styleLength.MatchString(plain) {
		add("fester Abstand")
	}
	if strings.HasPrefix(d.prop, "--") && styleLength.MatchString(plain) {
		add("feste Länge in einer Variable außerhalb von :root")
	}
	return found
}

func hasNamedStyleColor(value string) bool {
	for _, word := range styleWord.FindAllString(strings.ToLower(value), -1) {
		if styleNamedColors[word] {
			return true
		}
	}
	return false
}

func onlyStyleVariables(plain string, keywords []string) bool {
	rest := strings.Fields(strings.ReplaceAll(plain, ",", " "))
	if len(rest) == 0 {
		return true
	}
	if len(rest) > 1 {
		return false
	}
	if styleWideKeywords[rest[0]] {
		return true
	}
	for _, k := range keywords {
		if rest[0] == k {
			return true
		}
	}
	return false
}

// findStyleViolations meldet jeden festen Gestaltungswert außerhalb von :root.
// Der zweite Rückgabewert sagt, ob überhaupt ein :root-Block gefunden wurde.
func findStyleViolations(css string) ([]styleViolation, bool) {
	css = stripStyleComments(css)
	if !styleRootBlock.MatchString(css) {
		return nil, false
	}
	css = styleRootBlock.ReplaceAllStringFunc(css, blankStyle)
	var found []styleViolation
	for _, d := range styleDeclarations(css) {
		found = append(found, checkStyleDeclaration(d)...)
	}
	return found, true
}

func TestStylesheetKeepsDesignValuesInRoot(t *testing.T) {
	raw, err := staticFiles.ReadFile("static/styles.css")
	if err != nil {
		t.Fatalf("styles.css nicht eingebettet: %v", err)
	}
	found, ok := findStyleViolations(string(raw))
	if !ok {
		t.Fatalf("styles.css hat keinen :root-Block; die Gestaltungswerte gehören dorthin (%s)", styleRules)
	}
	for _, v := range found {
		t.Errorf("%s: fester Wert außerhalb von :root. Als Variable in :root anlegen und per var(--…) verweisen; Regeln und Ausnahmen: %s", v, styleRules)
	}
}

// Der Wächter selbst: er findet jede Sorte fester Werte und lässt die
// erlaubten Ausnahmen stehen — @media-Köpfe, 0, auto, Prozentwerte, Breiten,
// Rahmenstärken und CSS-weite Schlüsselwörter.
func TestStylesheetGuardFindsFixedValues(t *testing.T) {
	css := `:root {
  --accent: #2f5d50;
  --space-4: 8px;
}
/* #ffffff in einem Kommentar zählt nicht */
@media (max-width: 820px) {
  .a {
    width: min(100% - 22px, 1120px);
    padding: var(--space-4) 0;
  }
}
.b {
  color: #fff;
  background: rgba(0, 0, 0, 0.5);
  border: 2px solid var(--accent);
  border-color: white;
  box-shadow: inset 0 0 0 1px var(--accent);
  font-size: 0.9rem;
  margin: 0 auto;
  gap: 50%;
  padding: 0.12em 0;
  background-color: color-mix(in srgb, var(--accent) 12%, transparent);
  font: inherit;
  white-space: nowrap;
  --local: 24px;
  letter-spacing: var(--tracking, 0.1em);
}
`
	want := []string{
		"13:color", "14:background", "16:border-color", "17:box-shadow", "18:font-size",
		"21:padding", "22:background-color", "25:--local", "26:letter-spacing",
	}
	found, ok := findStyleViolations(css)
	if !ok {
		t.Fatal(":root-Block im Beispiel nicht erkannt")
	}
	var got []string
	for _, v := range found {
		got = append(got, fmt.Sprintf("%d:%s", v.line, v.prop))
	}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("Wächter meldet\n  %v\nerwartet\n  %v", got, want)
	}
}

// Nur ein Knopf sieht aus wie ein Knopf: Beschriftungen tragen weder Fläche
// noch Rahmen, Schatten oder Knopfhöhe. Der Zustand steckt in der Schriftfarbe
// und im Punkt davor.
const labelRules = `k-playbook-local/guidelines/oberflaeche-gestaltung.md, Abschnitt „Nur ein Knopf sieht aus wie ein Knopf"; ausgeliefert in installer/docs/architecture.md, Abschnitt „Gestaltung der Oberfläche"`

// styleLabelClasses sind die Klassen reiner Beschriftungen. Getroffen wird nur
// der ganze Klassenname: .pill-row ist ein Behälter und zählt nicht als .pill.
//
// Eine neue Beschriftung gehört in diese Liste, sonst prüft der Wächter sie
// nicht. Die fünf hinter den beiden ersten kommen von der Seite /github:
// Quell- und Ziel-Branch eines PR, seine Kennzahlenzeile, Branch oder Tag
// eines Laufs, dessen Kennzahlenzeile und die Zahl der Tests einer Ursache.
// Die Zustandsmarken der Seite — Recht, CI-Stand, Fork, Ziel ≠ Default —
// nutzen .pill und sind darüber schon abgedeckt.
var styleLabelClasses = []string{
	"pill", "version-badge",
	"pr-branches", "pr-meta", "run-ref", "run-meta", "failure-count",
}

// styleLabelDot ist der einzige Selektor, der eine Fläche tragen darf — der
// Punkt vor der Zustandsmarke. Rahmen, Schatten und Höhe bleiben ihm verboten.
const styleLabelDot = ".pill::before"

type styleRule struct {
	selector string
	decls    []styleDeclaration
}

// styleRulesOf zerlegt das Stylesheet in Regeln mit Selektor und
// Deklarationen. @-Regeln wie @media sind nur Hülle; die Regeln darin zählen
// wie alle anderen. Zeichenketten beachtet der Parser nicht: ein ; oder { in
// einem content-Wert brächte ihn aus dem Tritt.
func styleRulesOf(css string) []styleRule {
	css = stripStyleComments(css)
	var rules []styleRule
	var open []int // Index der Regel je offener Klammer, -1 für @-Regeln
	line, start, startLine := 1, 0, 1
	for i := 0; i < len(css); i++ {
		switch css[i] {
		case '\n':
			line++
		case '{':
			prelude := strings.Join(strings.Fields(css[start:i]), " ")
			if strings.HasPrefix(prelude, "@") {
				open = append(open, -1)
			} else {
				rules = append(rules, styleRule{selector: prelude})
				open = append(open, len(rules)-1)
			}
			start, startLine = i+1, line
		case '}', ';':
			if n := len(open); n > 0 && open[n-1] >= 0 {
				if d, ok := parseStyleDeclaration(css[start:i], startLine); ok {
					rules[open[n-1]].decls = append(rules[open[n-1]].decls, d)
				}
			}
			if css[i] == '}' && len(open) > 0 {
				open = open[:len(open)-1]
			}
			start, startLine = i+1, line
		}
	}
	return rules
}

func styleLabelPatterns(classes []string) []*regexp.Regexp {
	var out []*regexp.Regexp
	for _, c := range classes {
		out = append(out, regexp.MustCompile(`\.`+regexp.QuoteMeta(c)+`([^\w-]|$)`))
	}
	return out
}

// labelShapeProperty sagt, ob eine Eigenschaft eine Beschriftung zur Fläche,
// zum Rahmen, zum Schatten oder auf Knopfhöhe brächte. border-radius ist
// allein keine Form und bleibt erlaubt; ein Rahmen mit none oder 0 zeichnet
// nichts.
func labelShapeProperty(prop, value string, dot bool) bool {
	switch {
	case prop == "background" || prop == "background-color":
		return !dot
	case strings.HasPrefix(prop, "background-"):
		return true
	case prop == "border" || strings.HasPrefix(prop, "border-"):
		if strings.HasSuffix(prop, "-radius") {
			return false
		}
		return value != "none" && value != "0"
	case prop == "box-shadow", prop == "min-height":
		return true
	}
	return false
}

// findLabelShapes meldet jede Deklaration, die einer Beschriftung Fläche,
// Rahmen, Schatten oder Knopfhöhe gibt — in jeder Regel, deren Selektor eine
// Beschriftungsklasse trifft, Pseudo-Elemente eingeschlossen.
func findLabelShapes(css string, classes []string) []string {
	patterns := styleLabelPatterns(classes)
	var found []string
	for _, rule := range styleRulesOf(css) {
		for _, sel := range strings.Split(rule.selector, ",") {
			sel = strings.TrimSpace(sel)
			hit := false
			for _, p := range patterns {
				if p.MatchString(sel) {
					hit = true
					break
				}
			}
			if !hit {
				continue
			}
			for _, d := range rule.decls {
				if labelShapeProperty(d.prop, d.value, sel == styleLabelDot) {
					found = append(found, fmt.Sprintf("styles.css:%d: %s { %s: %s }", d.line, sel, d.prop, d.value))
				}
			}
		}
	}
	return found
}

func TestStylesheetKeepsLabelsFlat(t *testing.T) {
	raw, err := staticFiles.ReadFile("static/styles.css")
	if err != nil {
		t.Fatalf("styles.css nicht eingebettet: %v", err)
	}
	for _, v := range findLabelShapes(string(raw), styleLabelClasses) {
		t.Errorf("%s: Beschriftung mit Fläche, Rahmen, Schatten oder Knopfhöhe. Nur was sich anklicken lässt, sieht aus wie ein Knopf; den Zustand über Schriftfarbe und Punkt zeigen (%s)", v, labelRules)
	}
}

// Der Beschriftungs-Wächter selbst: er trifft ganze Klassennamen auch in
// Selektorlisten, @media und Pseudo-Elementen und lässt nur den Punkt in
// .pill::before eine Fläche tragen.
func TestLabelGuardFindsButtonShapes(t *testing.T) {
	css := `.pill {
  color: var(--muted);
  background: var(--accent);
  border-radius: var(--radius-sm);
}
.pill::before {
  background: currentColor;
  border: 1px solid var(--line);
}
.pill::after { background-color: var(--warn); }
.pill-row, .pillow { background: var(--card); min-height: 32px; }
.card, .section-head .pill.ok:hover {
  box-shadow: var(--shadow);
}
@media (max-width: 820px) {
  .version-badge {
    border-bottom: 1px solid var(--line);
    border: none;
    min-height: 32px;
  }
}
`
	want := []string{
		"styles.css:3: .pill { background: var(--accent) }",
		"styles.css:8: .pill::before { border: 1px solid var(--line) }",
		"styles.css:10: .pill::after { background-color: var(--warn) }",
		"styles.css:13: .section-head .pill.ok:hover { box-shadow: var(--shadow) }",
		"styles.css:17: .version-badge { border-bottom: 1px solid var(--line) }",
		"styles.css:19: .version-badge { min-height: 32px }",
	}
	got := findLabelShapes(css, styleLabelClasses)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("Beschriftungs-Wächter meldet\n  %s\nerwartet\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}
