package inventory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withFremdquelle legt im Projekt ein Verzeichnis werkzeug/ mit eigener
// go.mod an und schließt es über `exclude:` von der Standarderkennung aus.
func withFremdquelle(t *testing.T) Options {
	t.Helper()

	options := newRunProject(t)
	installation := filepath.Join(options.ProjectDir, "werkzeug", "installer")
	if err := os.MkdirAll(installation, 0o755); err != nil {
		t.Fatalf("anlegen: %v", err)
	}
	if err := os.WriteFile(filepath.Join(installation, "go.mod"),
		[]byte("module example.com/werkzeug\n\ngo 1.26\n\nrequire example.com/fremd v9.9.9\n"), 0o644); err != nil {
		t.Fatalf("schreiben: %v", err)
	}
	return options
}

// Gesperrt ist nichts: wer eine Quelle aus einem ausgeschlossenen Bereich im
// Inventar haben will, schreibt sie in `sources:`. Der Ausschluss wirkt nur auf
// die Standarderkennung.
func TestAusdruecklicheQuelleSchlaegtDenAusschluss(t *testing.T) {
	options := withFremdquelle(t)
	options.Config = sourcesConfig(t, `schema_version: 3
exclude:
  - werkzeug/**
sources:
  - path: werkzeug/installer/go.mod
    kind: go
    env: lokal
    note: Abhängigkeiten des Werkzeugs, ausdrücklich gewollt
`)

	result, err := Collect(options)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	found := false
	for _, entry := range result.Entries {
		if entry.Group == "go/example.com/fremd" {
			found = true
			if entry.ContextOrigin != ContextConfigured {
				t.Errorf("eine konfigurierte Quelle trägt ein gesetztes Label: %+v", entry)
			}
		}
	}
	if !found {
		t.Error("eine ausdrücklich konfigurierte Quelle wird gelesen, auch aus einem ausgeschlossenen Bereich")
	}
}

// Ein Muster aus `exclude:` übergeht die Standarderkennung genauso — und ist
// genauso sichtbar.
func TestExcludeMusterUebergehtQuellenSichtbar(t *testing.T) {
	options := newRunProject(t)
	fixtures := filepath.Join(options.ProjectDir, "tests", "fixtures", "beispiel")
	if err := os.MkdirAll(fixtures, 0o755); err != nil {
		t.Fatalf("anlegen: %v", err)
	}
	if err := os.WriteFile(filepath.Join(fixtures, "requirements.txt"),
		[]byte("redis==1.0.0\n"), 0o644); err != nil {
		t.Fatalf("schreiben: %v", err)
	}

	// Ohne Ausschluss ist die Fixture eine Quelle — und erzeugt eine
	// Abweichung, die nichts über das Projekt sagt.
	before, err := Collect(options)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(before.Deviations) != 1 {
		t.Fatalf("ohne Ausschluss steht die Fixture im Inventar: %+v", before.Deviations)
	}

	options.Config = sourcesConfig(t, `schema_version: 3
exclude:
  - tests/fixtures/**
`)
	after, err := Collect(options)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(after.Deviations) != 0 {
		t.Errorf("mit Ausschluss bleibt die Fixture draußen: %+v", after.Deviations)
	}
	for _, source := range after.Sources {
		if strings.HasPrefix(source.File, "tests/fixtures/") {
			t.Errorf("ausgeschlossene Quelle gelesen: %+v", source)
		}
	}
	var rule Exclusion
	for _, exclusion := range after.Exclusions {
		if exclusion.Origin == ExclusionConfigured {
			rule = exclusion
		}
	}
	if rule.Pattern != "tests/fixtures/**" || rule.Skipped != 1 {
		t.Errorf("der konfigurierte Ausschluss steht nicht sichtbar im Ergebnis: %+v", after.Exclusions)
	}
}

// Ein absolutes Ausschlussmuster hinge vom Rechner ab und träfe auf einem
// anderen nichts — stillschweigend. Es wird deshalb sichtbar abgelehnt.
func TestAbsolutesAusschlussmusterWirdAbgelehnt(t *testing.T) {
	options := newRunProject(t)
	options.Config = sourcesConfig(t, `schema_version: 3
exclude:
  - /srv/deploy/**
`)

	result, err := Collect(options)
	if err != nil {
		t.Fatalf("ein abgelehntes Muster bricht den Lauf nicht ab: %v", err)
	}
	if len(result.Rejections) != 1 {
		t.Fatalf("Ablehnungen = %+v", result.Rejections)
	}
	if !strings.Contains(result.Rejections[0].Reason, "relativ zur Projektwurzel") {
		t.Errorf("Grund = %q", result.Rejections[0].Reason)
	}
	for _, exclusion := range result.Exclusions {
		if exclusion.Origin == ExclusionConfigured {
			t.Errorf("ein abgelehntes Muster gilt nicht: %+v", exclusion)
		}
	}
}

func TestMatchExcludeVergleichtSegmentweise(t *testing.T) {
	fälle := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"werkzeug/**", "werkzeug/installer/go.mod", true},
		{"werkzeug/**", "werkzeug/go.mod", true},
		// Ein Nachbar mit gleichem Präfix ist nicht gemeint.
		{"werkzeug/**", "werkzeug-notizen/go.mod", false},
		{"werkzeug/**", "installer/werkzeug/go.mod", false},
		// Ohne Wildcard trifft das Muster den Pfad selbst und alles darunter.
		{"testdata", "testdata/projekt/go.mod", true},
		{"testdata", "testdata2/projekt/go.mod", false},
		{"tests/*/go.mod", "tests/eins/go.mod", true},
		{"tests/*/go.mod", "tests/eins/zwei/go.mod", false},
		{"**/testdata/**", "a/b/testdata/c/go.mod", true},
		{"", "go.mod", false},
	}
	for _, fall := range fälle {
		if got := matchExclude(fall.pattern, fall.path); got != fall.want {
			t.Errorf("matchExclude(%q, %q) = %v, erwartet %v",
				fall.pattern, fall.path, got, fall.want)
		}
	}
}
