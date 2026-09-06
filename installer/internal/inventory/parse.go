package inventory

import (
	"fmt"
	"strings"
)

// fileContext ist eine Quelle, die die Vertrauensgrenze freigegeben und
// gelesen hat.
type fileContext struct {
	// Display ist der Wert für Entry.SourceFile: projektrelativ, oder absolut,
	// wenn die Datei außerhalb der Projektwurzel liegt.
	Display string
	// Base ist der Dateiname ohne Verzeichnis; die Parser verzweigen darüber.
	Base string
	// Kind ist die aufgelöste Quellart, nie `auto`.
	Kind      string
	Env       string
	EnvOrigin string
	Data      []byte
	// Direct enthält die direkten Paketnamen eines zugehörigen Manifests. Es ist
	// nur für Lockfiles gesetzt; nil bedeutet, dass keine gültige Referenzmenge
	// ermittelt werden konnte.
	Direct map[string]string
}

// collector sammelt die Funde einer Datei und füllt die Felder, die für alle
// Einträge derselben Quelle gleich sind. Kein Parser setzt Context, SourceFile
// oder Group selbst — sonst liefe eine Quelle irgendwann unter einem anderen
// Label als die Tabelle im Vertrag sagt.
type collector struct {
	file    fileContext
	entries []Entry
	notes   []Note
}

func (c *collector) add(entry Entry) {
	entry.Name = normalizeName(entry.Ecosystem, entry.Name)
	if entry.Name == "" {
		return
	}
	if entry.Pin == "" {
		entry.Pin = classifyPin(entry.Version)
	}
	if entry.VersionNormalized == "" {
		entry.VersionNormalized = normalizeVersion(entry.Version, entry.Pin)
	}
	// Wo `unknown` steht, gehört ein Hinweis dazu, der sagt warum — so der
	// Vertrag. Die Parser setzen ihn, wo sie den Grund genauer kennen; diese
	// Zeile schließt die Lücke für alle übrigen Fälle, statt sie jedem Parser
	// einzeln zu überlassen und beim nächsten wieder zu vergessen.
	if entry.Pin == PinUnknown && entry.Note == "" {
		entry.Note = unknownReason(entry.Version)
	}
	entry.Context = c.file.Env
	entry.ContextOrigin = c.file.EnvOrigin
	entry.SourceFile = c.file.Display
	entry.Group = groupKey(entry.Ecosystem, entry.Name)
	c.entries = append(c.entries, entry)
}

func (c *collector) note(format string, args ...any) {
	c.notes = append(c.notes, Note{Source: c.file.Display, Text: fmt.Sprintf(format, args...)})
}

// text liefert den Inhalt als Zeilen, 1-basiert adressierbar über den Index+1.
func (c *collector) lines() []string {
	return strings.Split(strings.ReplaceAll(string(c.file.Data), "\r\n", "\n"), "\n")
}

// parseFile liest eine freigegebene Quelle.
//
// Eine bekannte, aber defekte Datei erzeugt einen sichtbaren Hinweis und keine
// erfundenen Einträge; eine unbekannte erzeugt gar nichts — nur was gesucht
// wird, kann fehlen.
func parseFile(file fileContext) ([]Entry, []Note) {
	collect := &collector{file: file}
	switch file.Kind {
	case KindPython:
		parsePython(collect)
	case KindGo:
		parseGo(collect)
	case KindNode:
		parseNode(collect)
	case KindRust:
		parseRust(collect)
	case KindRuby:
		parseRuby(collect)
	case KindPHP:
		parsePHP(collect)
	case KindJava:
		parseJava(collect)
	case KindElixir:
		parseElixir(collect)
	case KindDockerfile:
		parseDockerfile(collect)
	case KindCompose:
		parseCompose(collect)
	case KindDevcontainer:
		parseDevcontainer(collect)
	case KindHelm:
		parseHelm(collect)
	case KindCI:
		parseCI(collect)
	case KindToolVersions:
		parseToolVersions(collect)
	default:
		collect.note("unbekannte Quellart %q — die Datei wurde nicht ausgewertet", file.Kind)
	}
	return collect.entries, collect.notes
}

// resolveKind macht aus `auto` die Art am Dateinamen. Ein ausdrücklich
// gesetztes `kind` gilt unverändert: wer es hinschreibt, weiß mehr über die
// Datei als ihr Name verrät.
func resolveKind(kind string, requested string) (string, bool) {
	if kind != "" && kind != KindAuto {
		return kind, true
	}
	return detectKind(strings.ReplaceAll(requested, "\\", "/"))
}
