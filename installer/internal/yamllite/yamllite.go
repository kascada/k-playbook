// Package yamllite liest die Teilmenge von YAML, die k-playbook in
// Konfigurations- und Manifestdateien tatsächlich vorfindet: Blockabbildungen,
// Blocklisten, einfache Flow-Listen und -Abbildungen sowie Blockskalare.
//
// Warum kein YAML-Paket: das Werkzeug kommt bisher ohne YAML-Abhängigkeit aus —
// die K-PLAYBOOK.yaml wird zeilenweise gelesen —, und für das Versionsinventar
// ist die Zeilennummer jedes Fundes Pflicht: sie ist Teil der Herkunft
// (docs/versionsinventar.md, „Datenmodell einer Inventarzeile"). Die üblichen
// Pakete geben sie nur über Umwege heraus.
//
// Zwei Leser benutzen dieses Paket: der Sammler des Versionsinventars und der
// Leser der Quellenkonfiguration. Es hängt deshalb von nichts ab außer der
// Standardbibliothek — sonst entstünde ein Importzyklus zwischen `project` und
// dem Sammler.
//
// Was fehlt: Anker und Aliase, Merge-Keys, mehrere Dokumente je Datei und
// komplexe Schlüssel. Was davon vorkommt, bleibt als Rohtext stehen, statt den
// Lauf abzubrechen; was gar nicht deutbar ist, wird als Fehler gemeldet — ein
// stilles Leerergebnis gibt es nicht.
package yamllite

import (
	"fmt"
	"strconv"
	"strings"
)

// Kind unterscheidet die drei Knotenarten.
type Kind int

const (
	// Scalar ist ein einzelner Wert; ein Schlüssel ohne Wert ist ein leerer Scalar.
	Scalar Kind = iota
	// Mapping ist eine Abbildung mit erhaltener Schlüsselreihenfolge.
	Mapping
	// Sequence ist eine Liste.
	Sequence
)

// Node ist ein Knoten des gelesenen Baums. Line ist die 1-basierte Zeile, in
// der der Knoten beginnt — bei einem Skalar die Zeile seines Wertes, bei einer
// Abbildung die ihres ersten Schlüssels.
type Node struct {
	Kind   Kind
	Line   int
	Value  string
	Keys   []string
	Fields map[string]*Node
	Items  []*Node
}

// Get folgt einem Schlüsselpfad und liefert nil, sobald ein Schritt ins Leere
// führt. Alle Zugriffe sind nil-fest, damit Aufrufer nicht bei jedem Schritt
// prüfen müssen.
func (n *Node) Get(path ...string) *Node {
	current := n
	for _, key := range path {
		if current == nil || current.Kind != Mapping {
			return nil
		}
		current = current.Fields[key]
	}
	return current
}

// Str liefert den Skalarwert oder den leeren String.
func (n *Node) Str() string {
	if n == nil || n.Kind != Scalar {
		return ""
	}
	return n.Value
}

// Int liefert den Skalarwert als ganze Zahl.
func (n *Node) Int() (int, bool) {
	if n == nil || n.Kind != Scalar {
		return 0, false
	}
	value, err := strconv.Atoi(strings.TrimSpace(n.Value))
	if err != nil {
		return 0, false
	}
	return value, true
}

// At liefert die Zeilennummer, nil-fest.
func (n *Node) At() int {
	if n == nil {
		return 0
	}
	return n.Line
}

// Items liefert die Listeneinträge; für alles andere nil.
func (n *Node) List() []*Node {
	if n == nil || n.Kind != Sequence {
		return nil
	}
	return n.Items
}

// MapKeys liefert die Schlüssel einer Abbildung in Dateireihenfolge. Die
// Reihenfolge ist Teil des Vertrags: das Inventar wird deterministisch
// gerendert, und eine Schlüsselreihenfolge aus einer Go-Map wäre es nicht.
func (n *Node) MapKeys() []string {
	if n == nil || n.Kind != Mapping {
		return nil
	}
	return n.Keys
}

// Parse liest ein Dokument. Ein zweites Dokument in derselben Datei wird nicht
// gelesen — die Quellen des Versionsinventars kennen keines.
func Parse(data []byte) (*Node, error) {
	lines, err := scanLines(string(data))
	if err != nil {
		return nil, err
	}
	parser := &parser{lines: lines}
	parser.skipDocumentStart()

	first, ok := parser.peek()
	if !ok {
		return &Node{Kind: Mapping, Fields: map[string]*Node{}}, nil
	}
	root := parser.parseBlock(first.indent)
	if len(parser.problems) > 0 {
		return nil, fmt.Errorf("%s", strings.Join(parser.problems, "; "))
	}
	return root, nil
}

type sourceLine struct {
	num    int
	indent int
	text   string
	raw    string
	blank  bool
}

func scanLines(content string) ([]sourceLine, error) {
	raw := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	lines := make([]sourceLine, 0, len(raw))
	for index, text := range raw {
		stripped := strings.TrimRight(stripComment(text), " \t")
		if strings.TrimSpace(stripped) == "" {
			lines = append(lines, sourceLine{num: index + 1, raw: text, blank: true})
			continue
		}
		indent := 0
		for indent < len(stripped) && (stripped[indent] == ' ' || stripped[indent] == '\t') {
			if stripped[indent] == '\t' {
				return nil, fmt.Errorf("Zeile %d: Tabulator in der Einrückung", index+1)
			}
			indent++
		}
		lines = append(lines, sourceLine{num: index + 1, indent: indent, text: stripped, raw: text})
	}
	return lines, nil
}

// stripComment entfernt einen Kommentar. Ein `#` mitten im Wort bleibt stehen —
// `image: foo#bar` ist kein Kommentar, `image: foo # bar` schon.
func stripComment(line string) string {
	single, double := false, false
	for index := 0; index < len(line); index++ {
		char := line[index]
		switch {
		case single:
			if char == '\'' {
				single = false
			}
		case double:
			if char == '\\' {
				index++
			} else if char == '"' {
				double = false
			}
		case char == '\'':
			single = true
		case char == '"':
			double = true
		case char == '#':
			if index == 0 || line[index-1] == ' ' || line[index-1] == '\t' {
				return line[:index]
			}
		}
	}
	return line
}

type parser struct {
	lines    []sourceLine
	index    int
	problems []string
}

func (p *parser) peek() (sourceLine, bool) {
	for p.index < len(p.lines) && p.lines[p.index].blank {
		p.index++
	}
	if p.index >= len(p.lines) {
		return sourceLine{}, false
	}
	return p.lines[p.index], true
}

func (p *parser) skipDocumentStart() {
	if line, ok := p.peek(); ok && strings.TrimSpace(line.text) == "---" {
		p.index++
	}
}

func (p *parser) note(format string, args ...any) {
	p.problems = append(p.problems, fmt.Sprintf(format, args...))
}

func (p *parser) parseBlock(indent int) *Node {
	line, ok := p.peek()
	if !ok || line.indent < indent {
		return &Node{Kind: Mapping, Fields: map[string]*Node{}}
	}
	if isSequenceItem(line.text) {
		return p.parseSequence(indent)
	}
	return p.parseMapping(indent)
}

func (p *parser) parseMapping(indent int) *Node {
	node := &Node{Kind: Mapping, Fields: map[string]*Node{}}
	for {
		line, ok := p.peek()
		if !ok || line.indent < indent || isDocumentBreak(line.text) {
			break
		}
		if isSequenceItem(line.text) && line.indent == indent {
			break
		}
		if line.indent > indent {
			p.note("Zeile %d: Einrückung passt zu keiner offenen Ebene", line.num)
			p.index++
			continue
		}
		key, rest, found := splitKey(line.text)
		if !found {
			p.note("Zeile %d: weder Schlüssel noch Listeneintrag: %q", line.num, strings.TrimSpace(line.text))
			p.index++
			continue
		}
		if node.Line == 0 {
			node.Line = line.num
		}
		p.index++
		child := p.parseValue(line, indent, rest)
		if _, seen := node.Fields[key]; !seen {
			node.Keys = append(node.Keys, key)
		}
		node.Fields[key] = child
	}
	return node
}

func (p *parser) parseValue(keyLine sourceLine, indent int, rest string) *Node {
	rest = strings.TrimSpace(rest)
	switch {
	case rest == "":
		next, ok := p.peek()
		if !ok {
			return &Node{Kind: Scalar, Line: keyLine.num}
		}
		if next.indent > indent && !isDocumentBreak(next.text) {
			return p.parseBlock(next.indent)
		}
		if next.indent == indent && isSequenceItem(next.text) {
			return p.parseSequence(indent)
		}
		return &Node{Kind: Scalar, Line: keyLine.num}
	case rest[0] == '|' || rest[0] == '>':
		return p.parseBlockScalar(keyLine, indent, rest[0] == '>')
	case rest[0] == '[' || rest[0] == '{':
		node, err := parseFlow(rest, keyLine.num)
		if err != nil {
			p.note("Zeile %d: %v", keyLine.num, err)
			return &Node{Kind: Scalar, Line: keyLine.num, Value: rest}
		}
		return node
	default:
		return &Node{Kind: Scalar, Line: keyLine.num, Value: unquote(rest)}
	}
}

func (p *parser) parseSequence(indent int) *Node {
	node := &Node{Kind: Sequence}
	for {
		line, ok := p.peek()
		if !ok || line.indent != indent || !isSequenceItem(line.text) {
			break
		}
		if node.Line == 0 {
			node.Line = line.num
		}
		trimmed := line.text[line.indent:]
		offset := 1
		for offset < len(trimmed) && trimmed[offset] == ' ' {
			offset++
		}
		content := strings.TrimSpace(trimmed[offset:])
		itemIndent := line.indent + offset
		p.index++

		switch {
		case content == "":
			next, ok := p.peek()
			if ok && next.indent > indent {
				node.Items = append(node.Items, p.parseBlock(next.indent))
			} else {
				node.Items = append(node.Items, &Node{Kind: Scalar, Line: line.num})
			}
		case content[0] == '[' || content[0] == '{':
			item, err := parseFlow(content, line.num)
			if err != nil {
				p.note("Zeile %d: %v", line.num, err)
				item = &Node{Kind: Scalar, Line: line.num, Value: content}
			}
			node.Items = append(node.Items, item)
		default:
			if key, rest, found := splitKey(content); found {
				node.Items = append(node.Items, p.parseItemMapping(line, itemIndent, key, rest))
			} else {
				node.Items = append(node.Items, &Node{Kind: Scalar, Line: line.num, Value: unquote(content)})
			}
		}
	}
	return node
}

// parseItemMapping liest einen Listeneintrag, dessen erster Schlüssel schon auf
// der Strichzeile steht. Die weiteren Schlüssel desselben Eintrags stehen
// darunter auf der Spalte des ersten.
func (p *parser) parseItemMapping(line sourceLine, itemIndent int, key string, rest string) *Node {
	node := &Node{Kind: Mapping, Line: line.num, Fields: map[string]*Node{}}
	node.Keys = append(node.Keys, key)
	node.Fields[key] = p.parseValue(line, itemIndent, rest)

	siblings := p.parseMapping(itemIndent)
	for _, siblingKey := range siblings.Keys {
		if _, seen := node.Fields[siblingKey]; !seen {
			node.Keys = append(node.Keys, siblingKey)
		}
		node.Fields[siblingKey] = siblings.Fields[siblingKey]
	}
	return node
}

func (p *parser) parseBlockScalar(keyLine sourceLine, indent int, folded bool) *Node {
	var parts []string
	blockIndent := -1
	for p.index < len(p.lines) {
		line := p.lines[p.index]
		if line.blank {
			parts = append(parts, "")
			p.index++
			continue
		}
		if line.indent <= indent {
			break
		}
		if blockIndent < 0 {
			blockIndent = line.indent
		}
		text := strings.TrimRight(line.raw, " \t")
		if len(text) >= blockIndent {
			text = text[blockIndent:]
		} else {
			text = strings.TrimSpace(text)
		}
		parts = append(parts, text)
		p.index++
	}
	for len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	separator := "\n"
	if folded {
		separator = " "
	}
	return &Node{Kind: Scalar, Line: keyLine.num, Value: strings.Join(parts, separator)}
}

func isSequenceItem(text string) bool {
	trimmed := strings.TrimSpace(text)
	if !strings.HasPrefix(trimmed, "-") {
		return false
	}
	return len(trimmed) == 1 || trimmed[1] == ' '
}

func isDocumentBreak(text string) bool {
	trimmed := strings.TrimSpace(text)
	return trimmed == "---" || trimmed == "..."
}

// splitKey trennt `schlüssel: rest`. Der Doppelpunkt zählt nur außerhalb von
// Anführungszeichen und Flow-Klammern und nur, wenn ein Leerzeichen oder das
// Zeilenende folgt — sonst wäre `image: foo:1.2` zweimal geteilt.
func splitKey(text string) (string, string, bool) {
	single, double := false, false
	depth := 0
	for index := 0; index < len(text); index++ {
		char := text[index]
		switch {
		case single:
			if char == '\'' {
				single = false
			}
		case double:
			if char == '\\' {
				index++
			} else if char == '"' {
				double = false
			}
		case char == '\'':
			single = true
		case char == '"':
			double = true
		case char == '[' || char == '{':
			depth++
		case char == ']' || char == '}':
			if depth > 0 {
				depth--
			}
		case char == ':' && depth == 0:
			if index+1 >= len(text) || text[index+1] == ' ' || text[index+1] == '\t' {
				key := strings.TrimSpace(text[:index])
				if key == "" {
					return "", "", false
				}
				return unquote(key), text[index+1:], true
			}
		}
	}
	return "", "", false
}

func unquote(value string) string {
	value = strings.TrimSpace(value)
	if len(value) < 2 {
		return value
	}
	if value[0] == '"' && value[len(value)-1] == '"' {
		if unquoted, err := strconv.Unquote(value); err == nil {
			return unquoted
		}
		return value[1 : len(value)-1]
	}
	if value[0] == '\'' && value[len(value)-1] == '\'' {
		return strings.ReplaceAll(value[1:len(value)-1], "''", "'")
	}
	return value
}
