package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/inventory"
)

// Gezählt werden Gruppen, nicht Zeilen; die widersprüchlichen stehen daneben,
// weil sie die Frage aufwerfen.
func TestDescribeDeviationsZaehltGruppenUndNenntWidersprueche(t *testing.T) {
	fälle := []struct {
		name       string
		deviations []inventory.Deviation
		want       string
	}{
		{name: "keine", want: "0"},
		{
			name: "nur umgebungsbedingt",
			deviations: []inventory.Deviation{
				{Art: inventory.DeviationEnvironmental},
				{Art: inventory.DeviationEnvironmental},
			},
			want: "2",
		},
		{
			name: "gemischt",
			deviations: []inventory.Deviation{
				{Art: inventory.DeviationConflicting},
				{Art: inventory.DeviationEnvironmental},
			},
			want: "2 (davon 1 widersprüchlich)",
		},
	}
	for _, fall := range fälle {
		if got := describeDeviations(fall.deviations); got != fall.want {
			t.Errorf("%s: %q, erwartet %q", fall.name, got, fall.want)
		}
	}
}

// Eine Ablehnung nennt den aufgelösten Pfad, damit erkennbar ist, was
// tatsächlich gelesen worden wäre.
func TestDescribeRejectionNenntDenAufgeloestenPfad(t *testing.T) {
	got := describeRejection(inventory.Rejection{
		Requested: "docs/link.yaml", Resolved: "/etc/passwd", Reason: "außerhalb der erlaubten Wurzeln",
	})
	for _, want := range []string{"docs/link.yaml", "/etc/passwd", "außerhalb"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q fehlt in %q", want, got)
		}
	}

	same := describeRejection(inventory.Rejection{Requested: "/srv/x", Reason: "nicht lesbar"})
	if strings.Contains(same, "→") {
		t.Errorf("ohne aufgelösten Pfad kein Pfeil: %q", same)
	}
}

// Ablehnungen und Hinweise stehen vollständig im Bericht, nicht nur als Zahl:
// eine Lücke, die niemand sieht, ist schlimmer als ein Fehler.
func TestPrintInventoryFuehrtAblehnungenUndHinweiseAus(t *testing.T) {
	var out bytes.Buffer
	printInventory(&out,
		inventory.Options{ProjectDir: "/projekt", InventoryFile: "/projekt/k-playbook-local/docs/versions/inventory.md"},
		inventory.Result{
			Rejections: []inventory.Rejection{{Requested: "/srv/deploy/values.yaml", Reason: "keine freigegebene Wurzel"}},
			Notes:      []inventory.Note{{Source: "Chart.yaml", Text: "defektes YAML"}},
		},
		inventory.Outcome{Path: "/projekt/k-playbook-local/docs/versions/inventory.md", Written: true, At: "2026-09-05T12:00:00Z"})

	text := out.String()
	for _, want := range []string{"/srv/deploy/values.yaml", "keine freigegebene Wurzel", "Chart.yaml", "defektes YAML", "Geschrieben:"} {
		if !strings.Contains(text, want) {
			t.Errorf("%q fehlt im Bericht:\n%s", want, text)
		}
	}
}

// Ein Lauf ohne inhaltliche Änderung sagt ausdrücklich, dass die Datei
// unangetastet blieb — sonst liest sich der alte Zeitstempel wie ein Fehlschlag.
func TestPrintInventoryMeldetDenUnveraendertenLauf(t *testing.T) {
	var out bytes.Buffer
	printInventory(&out, inventory.Options{ProjectDir: "/projekt"}, inventory.Result{},
		inventory.Outcome{Path: "/projekt/k-playbook-local/docs/versions/inventory.md", At: "2026-09-01T08:00:00Z"})

	text := out.String()
	if !strings.Contains(text, "Unverändert:") {
		t.Errorf("der unveränderte Lauf wird nicht benannt:\n%s", text)
	}
	if strings.Contains(text, "Geschrieben:") {
		t.Errorf("ohne Schreiben darf nichts als geschrieben gemeldet werden:\n%s", text)
	}
}

// Auch der Ausschluss steht im Bericht: er ist keine Ablehnung, aber eine
// Stelle, an der nicht gesucht wurde — und ein Filter, den niemand sieht, wäre
// eine Lücke.
func TestPrintInventoryNenntNichtDurchsuchteBereiche(t *testing.T) {
	var out bytes.Buffer
	printInventory(&out, inventory.Options{ProjectDir: "/projekt"},
		inventory.Result{
			Exclusions: []inventory.Exclusion{
				{Pattern: "k-playbook/**", Origin: "installation", Reason: "Clone des Werkzeugs", Skipped: 3},
				{Pattern: "tests/fixtures/**", Origin: "configured", Reason: "ausgenommen", Skipped: 0},
			},
		},
		inventory.Outcome{Path: "/projekt/k-playbook-local/docs/versions/inventory.md", At: "2026-09-05T12:00:00Z"})

	text := out.String()
	for _, want := range []string{"Nicht durchsuchte Quellen:    3", "k-playbook/**",
		"installation", "Clone des Werkzeugs"} {
		if !strings.Contains(text, want) {
			t.Errorf("%q fehlt im Bericht:\n%s", want, text)
		}
	}
	// Eine Regel, die nichts getroffen hat, macht den Bericht nicht länger.
	if strings.Contains(text, "tests/fixtures/**") {
		t.Errorf("eine Regel ohne Treffer gehört nicht in die Aufzählung:\n%s", text)
	}
}
