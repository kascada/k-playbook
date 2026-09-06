package inventory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withInstallation legt neben dem Projekt die Installation an — dieselbe Lage
// wie in jedem Zielprojekt: `k-playbook/` liegt neben den eigenen Quellen und
// bringt eigene mit.
func withInstallation(t *testing.T) Options {
	t.Helper()

	options := newRunProject(t)
	installation := filepath.Join(options.ProjectDir, InstallationDirName, "installer")
	if err := os.MkdirAll(installation, 0o755); err != nil {
		t.Fatalf("anlegen: %v", err)
	}
	if err := os.WriteFile(filepath.Join(installation, "go.mod"),
		[]byte("module example.com/werkzeug\n\ngo 1.26\n\nrequire example.com/fremd v9.9.9\n"), 0o644); err != nil {
		t.Fatalf("schreiben: %v", err)
	}
	return options
}

// Die Installation ist keine Standardquelle: ihre Versionsangaben beschreiben
// das Werkzeug, nicht das Projekt, und sie werden beim nächsten Update
// ausgetauscht.
func TestInstallationIstKeineStandardquelle(t *testing.T) {
	options := withInstallation(t)

	result, err := Collect(options)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	for _, source := range result.Sources {
		if strings.HasPrefix(source.File, InstallationDirName+"/") {
			t.Errorf("die Installation wurde als Standardquelle gelesen: %+v", source)
		}
	}
	for _, entry := range result.Entries {
		if entry.Group == "go/example.com/fremd" {
			t.Errorf("ein Eintrag aus der Installation: %+v", entry)
		}
	}
}

// Der Ausschluss ist nicht still: er steht mit Muster, Herkunft, Grund und der
// Zahl der übergangenen Quellen in der Inventardatei.
func TestAusschlussDerInstallationStehtSichtbarInDerDatei(t *testing.T) {
	options := withInstallation(t)

	result, err := Collect(options)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	var rule Exclusion
	for _, exclusion := range result.Exclusions {
		if exclusion.Origin == ExclusionInstallation {
			rule = exclusion
		}
	}
	if rule.Pattern != InstallationDirName+"/**" {
		t.Fatalf("die feste Regel fehlt im Ergebnis: %+v", result.Exclusions)
	}
	if rule.Skipped != 1 {
		t.Errorf("übergangene Quellen = %d, erwartet 1", rule.Skipped)
	}

	rendered := Render(result, "2026-09-05T12:00:00+02:00")
	for _, needle := range []string{
		"## Nicht durchsuchte Bereiche", "`" + InstallationDirName + "/**`",
		ExclusionInstallation, "sources-excluded: 1", "- Nicht durchsuchte Quellen: 1",
	} {
		if !strings.Contains(rendered, needle) {
			t.Errorf("in der Datei fehlt %q", needle)
		}
	}
}

// Gesperrt ist nichts: wer eine Quelle aus einem ausgeschlossenen Bereich im
// Inventar haben will, schreibt sie in `sources:`. Der Ausschluss wirkt nur auf
// die Standarderkennung.
func TestAusdruecklicheQuelleSchlaegtDenAusschluss(t *testing.T) {
	options := withInstallation(t)
	writeSources(t, options.SourcesFile, `schema_version: 1
sources:
  - path: k-playbook/installer/go.mod
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
		t.Error("eine ausdrücklich konfigurierte Quelle wird gelesen, auch aus der Installation")
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

	writeSources(t, options.SourcesFile, `schema_version: 1
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
	writeSources(t, options.SourcesFile, `schema_version: 1
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
		{"k-playbook/**", "k-playbook/installer/go.mod", true},
		{"k-playbook/**", "k-playbook/go.mod", true},
		// Ein Nachbar mit gleichem Präfix ist nicht gemeint.
		{"k-playbook/**", "k-playbook-local/go.mod", false},
		{"k-playbook/**", "installer/k-playbook/go.mod", false},
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
