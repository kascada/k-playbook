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
