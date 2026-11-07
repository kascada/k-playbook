// Package markdown trägt die eine Goldmark-Konfiguration, mit der k-playbook
// Markdown liest.
//
// Die Oberfläche rendert damit die Doku. parser.WithAutoHeadingID erzeugt die
// Überschriften-Ids, gegen die ein Verweis mit Anker aufgelöst wird — liest
// ein zweiter Leser dieselben Dateien mit einer anderen Option, laufen Sprünge
// stumm ins Leere. Deshalb steht die Konstruktion genau einmal, hier.
//
// Gemessen gegen goldmark v1.8.5 und in markdown_test.go festgehalten:
// Nicht-ASCII fällt aus den Ids ersatzlos heraus („Überschriften" →
// „berschriften"), und die Eindeutigkeit zählt über den ganzen Parser-Lauf
// („Ablauf" dreimal → ablauf, ablauf-1, ablauf-2).
package markdown

import (
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
)

// New liefert die Goldmark-Instanz. GFM deckt Tabellen und Aufgabenlisten ab,
// die in den mitgelieferten Dateien vorkommen; Überschriften bekommen eine ID,
// damit Verweise innerhalb einer Datei funktionieren.
//
// Rohes HTML aus der Quelle bleibt bewusst abgeschaltet — das ist die
// Voreinstellung von goldmark und genau richtig für Text, der einfach im
// Browser landet.
func New() goldmark.Markdown {
	return goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	)
}
