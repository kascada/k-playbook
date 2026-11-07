package project

import (
	"strconv"
	"testing"
)

// Werte, die YAML anders läse — ein Doppelpunkt mit Leerzeichen, ein
// führender Stern, eine Zahl, „true" —, werden zitiert und kommen über
// strconv.Unquote zeichengleich zurück; schlichte Werte bleiben schlicht.
func TestYamlScalarZitiertHeikleWerte(t *testing.T) {
	for _, value := range []string{"Sitzung 42", "Ablauf", "k-playbook v0.7.0", "ä ö ü ß"} {
		if yamlScalar(value) != value {
			t.Errorf("%q wurde zitiert: %s", value, yamlScalar(value))
		}
	}
	for _, value := range []string{"", " x", "x ", "- a", "a: b", "a #b", "true", "null", "3.5", "[a]", "\"q\"", "a:", "Release: was der Wächter prüft", "*Sterne* und #Rauten", "2026"} {
		quoted := yamlScalar(value)
		if quoted == value {
			t.Errorf("%q wurde nicht zitiert", value)
			continue
		}
		back, err := strconv.Unquote(quoted)
		if err != nil || back != value {
			t.Errorf("%q kommt als %q zurück (%v)", value, back, err)
		}
	}
}
