package inventory

import "testing"

// Body trennt den Frontmatter-Block ab und deutet den Rumpf nicht: was nach
// der zweiten `---`-Zeile steht, kommt unverändert zurück.
func TestBodyTrenntDasFrontmatterAb(t *testing.T) {
	data := []byte("---\ntitle: Versionsinventar\ninventory:\n  entries: 3\n---\n\n# Versionsinventar\n\n| a | b |\n")

	got := string(Body(data))
	if got != "# Versionsinventar\n\n| a | b |\n" {
		t.Errorf("Body = %q", got)
	}
}

// Ohne Frontmatter ist der Rumpf die ganze Datei — auch dann, wenn ein
// öffnendes `---` kein schließendes findet.
func TestBodyOhneFrontmatterIstDieGanzeDatei(t *testing.T) {
	for name, data := range map[string]string{
		"kein Block":       "# Versionsinventar\n",
		"nur öffnend":      "---\ntitle: x\n# Rumpf\n",
		"Trenner im Rumpf": "# Versionsinventar\n\n---\n",
	} {
		if got := string(Body([]byte(data))); got != data {
			t.Errorf("%s: Body = %q, erwartet die Datei selbst", name, got)
		}
	}
}
