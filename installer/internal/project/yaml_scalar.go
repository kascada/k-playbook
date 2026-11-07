package project

import (
	"strconv"
	"strings"
)

// yamlScalar schreibt einen Wert so, dass jeder YAML-Leser ihn als genau
// diese Zeichenkette liest. Ein schlichter Wert bleibt schlicht; was YAML
// anders deuten könnte — führende Sonderzeichen, „: " oder „ #" im Text,
// Leerraum an den Enden, Werte, die wie true, null oder eine Zahl aussehen —
// wird in doppelte Anführungszeichen gesetzt. yamllite liest solche Werte
// über strconv.Unquote zurück, deshalb strconv.Quote zum Schreiben.
//
// Genutzt vom Schreibweg der Dokument-Operationen (docedit.go) und von den
// Todo-Dokumenten (todo_doc.go).
func yamlScalar(value string) string {
	if yamlNeedsQuotes(value) {
		return strconv.Quote(value)
	}
	return value
}

func yamlNeedsQuotes(value string) bool {
	if value == "" || value != strings.TrimSpace(value) {
		return true
	}
	if strings.ContainsAny(value[:1], "-?:,[]{}#&*!|>'\"%@`") {
		return true
	}
	if strings.Contains(value, ": ") || strings.Contains(value, " #") || strings.HasSuffix(value, ":") || strings.ContainsAny(value, "\t\r\n") {
		return true
	}
	switch strings.ToLower(value) {
	case "true", "false", "yes", "no", "on", "off", "null", "~":
		return true
	}
	if _, err := strconv.ParseFloat(value, 64); err == nil {
		return true
	}
	return false
}
