package yamllite

import (
	"fmt"
	"strings"
)

// parseFlow liest die einzeilige Schreibweise `[a, b]` und `{k: v}`. Sie kommt
// in den gelesenen Quellen vor — `tags: [versions, inventory]` im Frontmatter,
// `generated: { by: …, at: … }` ebendort, `roots: []` in der leeren
// Quellenkonfiguration.
//
// Eine unbalancierte Klammer ist ein Fehler und kein halb gelesener Wert: eine
// defekte Konfiguration halb zu deuten hieße, eine andere Vertrauensgrenze
// anzuwenden als die aufgeschriebene.
func parseFlow(text string, line int) (*Node, error) {
	parser := &flowParser{text: text, line: line}
	node, err := parser.parseValue()
	if err != nil {
		return nil, err
	}
	parser.skipSpace()
	if parser.index < len(parser.text) {
		return nil, fmt.Errorf("überzähliges Zeichen %q nach dem Flow-Wert", parser.text[parser.index])
	}
	return node, nil
}

type flowParser struct {
	text  string
	index int
	line  int
}

func (f *flowParser) skipSpace() {
	for f.index < len(f.text) && (f.text[f.index] == ' ' || f.text[f.index] == '\t') {
		f.index++
	}
}

func (f *flowParser) parseValue() (*Node, error) {
	f.skipSpace()
	if f.index >= len(f.text) {
		return &Node{Kind: Scalar, Line: f.line}, nil
	}
	switch f.text[f.index] {
	case '[':
		return f.parseSequence()
	case '{':
		return f.parseMapping()
	}
	return &Node{Kind: Scalar, Line: f.line, Value: unquote(f.readUntil(false))}, nil
}

func (f *flowParser) parseSequence() (*Node, error) {
	f.index++ // [
	node := &Node{Kind: Sequence, Line: f.line}
	for {
		f.skipSpace()
		if f.index >= len(f.text) {
			return nil, fmt.Errorf("nicht geschlossene Flow-Liste")
		}
		if f.text[f.index] == ']' {
			f.index++
			return node, nil
		}
		item, err := f.parseValue()
		if err != nil {
			return nil, err
		}
		node.Items = append(node.Items, item)
		f.skipSpace()
		if f.index < len(f.text) && f.text[f.index] == ',' {
			f.index++
			continue
		}
		if f.index >= len(f.text) || f.text[f.index] != ']' {
			return nil, fmt.Errorf("nicht geschlossene Flow-Liste")
		}
	}
}

func (f *flowParser) parseMapping() (*Node, error) {
	f.index++ // {
	node := &Node{Kind: Mapping, Line: f.line, Fields: map[string]*Node{}}
	for {
		f.skipSpace()
		if f.index >= len(f.text) {
			return nil, fmt.Errorf("nicht geschlossene Flow-Abbildung")
		}
		if f.text[f.index] == '}' {
			f.index++
			return node, nil
		}
		key := unquote(f.readUntil(true))
		f.skipSpace()
		if f.index >= len(f.text) || f.text[f.index] != ':' {
			return nil, fmt.Errorf("Schlüssel %q ohne Doppelpunkt in der Flow-Abbildung", key)
		}
		f.index++
		value, err := f.parseValue()
		if err != nil {
			return nil, err
		}
		if _, seen := node.Fields[key]; !seen {
			node.Keys = append(node.Keys, key)
		}
		node.Fields[key] = value
		f.skipSpace()
		if f.index < len(f.text) && f.text[f.index] == ',' {
			f.index++
			continue
		}
		if f.index >= len(f.text) || f.text[f.index] != '}' {
			return nil, fmt.Errorf("nicht geschlossene Flow-Abbildung")
		}
	}
}

// readUntil liest einen Skalar bis zum nächsten Trennzeichen. stopAtColon gilt
// beim Schlüssel einer Flow-Abbildung.
func (f *flowParser) readUntil(stopAtColon bool) string {
	start := f.index
	single, double := false, false
	for f.index < len(f.text) {
		char := f.text[f.index]
		switch {
		case single:
			if char == '\'' {
				single = false
			}
		case double:
			if char == '\\' {
				f.index++
			} else if char == '"' {
				double = false
			}
		case char == '\'':
			single = true
		case char == '"':
			double = true
		case char == ',' || char == ']' || char == '}':
			return strings.TrimSpace(f.text[start:f.index])
		case stopAtColon && char == ':':
			return strings.TrimSpace(f.text[start:f.index])
		}
		f.index++
	}
	return strings.TrimSpace(f.text[start:f.index])
}
