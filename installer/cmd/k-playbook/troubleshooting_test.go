package main

import (
	"bytes"
	"strings"
	"testing"
)

// `k-playbook troubleshooting` ist bis zu seinem eigenen Task ein Platzhalter:
// Er sagt, wofür er da ist und dass er noch nicht umgesetzt ist, nennt die
// Befehle, die bis dahin helfen, und endet mit 0.
func TestTroubleshootingIstPlatzhalter(t *testing.T) {
	var out bytes.Buffer
	if err := runTroubleshooting(&out, nil); err != nil {
		t.Fatalf("Fehler: %v", err)
	}
	text := out.String()
	for _, want := range []string{TroubleshootingCommand, "Kephalaion", "Noch nicht umgesetzt", "k-playbook health"} {
		if !strings.Contains(text, want) {
			t.Errorf("Ausgabe ohne %q:\n%s", want, text)
		}
	}
	if lines := strings.Count(text, "\n"); lines > 12 {
		t.Errorf("Ausgabe zu lang (%d Zeilen):\n%s", lines, text)
	}

	out.Reset()
	if err := runTroubleshooting(&out, []string{"--help"}); err != nil || out.Len() == 0 {
		t.Errorf("--help: %v, %q", err, out.String())
	}
	if err := runTroubleshooting(&out, []string{"--kaputt"}); err == nil {
		t.Error("unbekannte Option ohne Fehler")
	}
}
