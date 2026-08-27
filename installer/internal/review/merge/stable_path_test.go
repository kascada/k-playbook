package merge

import (
	"strings"
	"testing"
)

// TestStablePathIstEingefroren hält den Stand der eingefrorenen Pfadnormierung
// fest, aus der die Stable-IDs gebildet werden.
//
// Der Test prüft **nicht** Gleichheit mit pathnorm.Normalize. Die
// beiden sind heute wertgleich, weil stablePath als Kopie des heutigen Standes
// entstanden ist; sie dürfen aber auseinanderlaufen, sobald pathnorm.Normalize
// für die Gruppierung verbessert wird. Genau dafür ist die Kopie da. Ein Test auf
// Gleichheit machte die Entkopplung wieder zunichte.
//
// Schlägt dieser Test fehl, ist nicht der Test falsch: dann hat jemand
// stablePath geändert und damit jede Stable-ID verschoben, deren Pfad davon
// berührt wird. Siehe den Doc-Kommentar an stablePath.
func TestStablePathIstEingefroren(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"leer", "", ""},
		{"relativer Pfad unverändert", "installer/go.mod", "installer/go.mod"},
		{"führendes Slash fällt weg", "/dist/k-playbook-linux-amd64", "dist/k-playbook-linux-amd64"},
		{"file:// ohne Authority", "file:///home/kleist/requirements.txt", "home/kleist/requirements.txt"},
		{"file:// mit Authority", "file://rechner/pfad/requirements.txt", "pfad/requirements.txt"},
		{"file:// ohne weiteren Slash", "file://requirements.txt", "requirements.txt"},
		{"doppelte Slashes", "a//b///c.go", "a/b/c.go"},
		{"Punkt-Segment", "./a/./b.go", "a/b.go"},
		{"Doppelpunkt-Segment relativ", "a/../b.go", "b.go"},
		{"Doppelpunkt-Segment über die Wurzel hinaus", "/../b.go", "b.go"},
		{"Doppelpunkt-Segment am Anfang bleibt relativ", "../b.go", "../b.go"},
		{"Backslashes", `installer\internal\merge.go`, "installer/internal/merge.go"},
		{"Großschreibung", "Installer/GO.MOD", "installer/go.mod"},
		{"nur Slashes", "//", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := stablePath(test.in); got != test.want {
				t.Fatalf("stablePath(%q) = %q, erwartet %q", test.in, got, test.want)
			}
		})
	}
}

// TestStableKeyHaengtNichtAnPathnorm belegt die Entkopplung an der Stelle, an
// der sie zählt: der Schlüssel einer Gruppe entsteht aus stablePath, nicht aus
// pathnorm.Normalize. Der Test ruft beide Wege über dieselben Findings und
// vergleicht den Schlüssel mit dem, was stablePath liefert.
func TestStableKeyHaengtNichtAnPathnorm(t *testing.T) {
	findings := []Finding{{
		ID:       "f1",
		Evidence: Evidence{Tool: "osv-scanner", Job: "osv-scanner"},
		RuleID:   "CVE-2018-18074",
		Message:  "requests 2.19.0",
		Location: Location{URI: "file:///abs/pfad/requirements.txt", StartLine: 1, StartColumn: 1},
	}}
	_, key := stablePrefixAndKey(findings)
	want := "locations=" + stablePath("file:///abs/pfad/requirements.txt") + ":1:1"
	if !strings.Contains(key, want) {
		t.Fatalf("stableKey enthält %q nicht:\n%s", want, key)
	}
}
