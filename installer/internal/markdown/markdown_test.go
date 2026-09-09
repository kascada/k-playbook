package markdown

import (
	"bytes"
	"strings"
	"testing"
)

// Die drei Eigenschaften, auf die sich beide Seiten verlassen: die Oberfläche
// beim Rendern der Doku, das Wissenstor beim Bilden seiner Anker. Sie sind hier
// festgenagelt, weil eine Änderung an New() sonst nur an einer stummen
// Fehlfunktion auffiele — ein Sprung aus einem Suchtreffer, der ins Leere geht.
func TestNewBildetUeberschriftenIDs(t *testing.T) {
	html := rendern(t, "## Freigabe\n")

	if !strings.Contains(html, `id="freigabe"`) {
		t.Errorf("keine Überschriften-Id gebildet: %s", html)
	}
}

// Nicht-ASCII fällt ersatzlos heraus. Das ist Goldmarks Verhalten, nicht unser
// Wunsch — festgehalten, damit ein Wechsel der Fassung auffällt, statt jeden
// Anker der deutschen Doku still zu brechen.
func TestNewVerwirftNichtASCIIAusIDs(t *testing.T) {
	html := rendern(t, "## Überschriften\n")

	if !strings.Contains(html, `id="berschriften"`) {
		t.Errorf(`erwartet id="berschriften": %s`, html)
	}
}

// Die Eindeutigkeit zählt über den ganzen Lauf, nicht je Abschnitt. Darauf
// beruht die Festlegung des Wissenstors, jede Datei einmal ganz zu parsen und
// erst danach zu chunken.
func TestNewZaehltWiederholteUeberschriftenDurch(t *testing.T) {
	html := rendern(t, "## Ablauf\n\ntext\n\n## Ablauf\n\ntext\n\n## Ablauf\n")

	for _, want := range []string{`id="ablauf"`, `id="ablauf-1"`, `id="ablauf-2"`} {
		if !strings.Contains(html, want) {
			t.Errorf("erwartet %s: %s", want, html)
		}
	}
}

// Rohes HTML bleibt abgeschaltet. Die Doku landet ungefiltert im Browser; ein
// Wechsel auf goldmark.WithRendererOptions(html.WithUnsafe()) wäre hier zu
// sehen und nicht erst an einer fremden Datei, die Skript mitbringt.
func TestNewLaesstRohesHTMLStehen(t *testing.T) {
	html := rendern(t, "<script>alert(1)</script>\n")

	if strings.Contains(html, "<script>") {
		t.Errorf("rohes HTML wurde durchgereicht: %s", html)
	}
}

// GFM ist an: die mitgelieferte Doku benutzt Tabellen.
func TestNewRendertGFMTabellen(t *testing.T) {
	html := rendern(t, "| a | b |\n|---|---|\n| 1 | 2 |\n")

	if !strings.Contains(html, "<table>") {
		t.Errorf("keine Tabelle gerendert: %s", html)
	}
}

func rendern(t *testing.T, quelle string) string {
	t.Helper()

	buf := bytes.Buffer{}
	if err := New().Convert([]byte(quelle), &buf); err != nil {
		t.Fatalf("konvertieren: %v", err)
	}
	return buf.String()
}
