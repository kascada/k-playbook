package inventory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSources(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("anlegen: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("schreiben: %v", err)
	}
}

// Eine fehlende Quellenkonfiguration ist kein Fehler: es gelten die
// Standardquellen unterhalb der Projektwurzel.
func TestFehlendeQuellenkonfigurationIstKeinFehler(t *testing.T) {
	options := newRunProject(t)
	result, err := Collect(options)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(result.Sources) == 0 {
		t.Error("ohne Konfiguration gelten die Standardquellen")
	}
	if result.ConfiguredSources != 0 {
		t.Errorf("konfigurierte Zusatzquellen = %d", result.ConfiguredSources)
	}
}

// Der Erhebungslauf bricht bei defekter Konfiguration ab — anders als
// `k-playbook context`, der sie als Zustand ausgibt und weiterläuft.
func TestDefekteQuellenkonfigurationBrichtDenLaufAb(t *testing.T) {
	for name, content := range map[string]string{
		"nicht lesbares YAML": "schema_version: 1\nroots: [/srv\n",
		"fremde Fassung":      "schema_version: 2\n",
	} {
		t.Run(name, func(t *testing.T) {
			options := newRunProject(t)
			writeSources(t, options.SourcesFile, content)

			if _, err := Collect(options); err == nil {
				t.Fatal("erwartet wurde ein Abbruch")
			}
			if _, _, err := Run(options); err == nil {
				t.Fatal("Run muss denselben Abbruch weitergeben")
			}
			if _, err := os.Stat(options.InventoryFile); !os.IsNotExist(err) {
				t.Error("bei einem Abbruch wird nichts geschrieben")
			}
		})
	}
}

// Ein unbekanntes Umgebungslabel lehnt den Eintrag ab, nicht den Lauf — und die
// Ablehnung steht sichtbar im Ergebnis.
func TestUnbekanntesLabelLehntDenEintragSichtbarAb(t *testing.T) {
	options := newRunProject(t)
	writeSources(t, options.SourcesFile, `schema_version: 1
sources:
  - path: requirements.txt
    kind: python
    env: produktion
`)
	result, err := Collect(options)
	if err != nil {
		t.Fatalf("der Lauf darf weitergehen: %v", err)
	}
	if len(result.Rejections) != 1 {
		t.Fatalf("Ablehnungen = %+v", result.Rejections)
	}
	if !strings.Contains(result.Rejections[0].Reason, "produktion") {
		t.Errorf("der gefundene Wert muss in der Meldung stehen: %q", result.Rejections[0].Reason)
	}
	if !strings.Contains(result.Rejections[0].Reason, "version-sources.yaml") {
		t.Errorf("die Meldung muss auf die Konfigurationsdatei zeigen: %q", result.Rejections[0].Reason)
	}
	if result.ConfiguredSources != 1 {
		t.Errorf("ein abgelehnter Eintrag zählt weiterhin als konfiguriert: %d", result.ConfiguredSources)
	}
}

// Eine konfigurierte, aber fehlende Quelle ist ein sichtbarer Hinweis — es sei
// denn, der Eintrag trägt optional: true.
func TestFehlendeKonfigurierteQuelleIstEinHinweis(t *testing.T) {
	options := newRunProject(t)
	writeSources(t, options.SourcesFile, `schema_version: 1
sources:
  - path: gibtesnicht.txt
    kind: python
    env: ci
  - path: auchnicht.txt
    kind: python
    env: ci
    optional: true
`)
	result, err := Collect(options)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(result.Notes) != 1 {
		t.Fatalf("Hinweise = %+v", result.Notes)
	}
	if !strings.Contains(result.Notes[0].Source, "gibtesnicht.txt") {
		t.Errorf("der Hinweis muss die Quelle nennen: %+v", result.Notes[0])
	}
}

// Eine konfigurierte Quelle überschreibt das Label der Standarderkennung —
// dieselbe Datei, ausdrücklich gesetzter Kontext.
func TestKonfigurierteQuelleUeberschreibtDasLabel(t *testing.T) {
	options := newRunProject(t)
	writeSources(t, options.SourcesFile, `schema_version: 1
sources:
  - path: requirements.txt
    kind: auto
    env: deployment
    note: Stand des Deployments
`)
	result, err := Collect(options)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	entry := find(t, result, "python/redis", "requirements.txt")
	if entry.Context != EnvDeployment || entry.ContextOrigin != ContextConfigured {
		t.Errorf("Kontext = %q/%q, erwartet deployment/configured", entry.Context, entry.ContextOrigin)
	}
	if len(result.Sources) != 1 || !result.Sources[0].Configured {
		t.Errorf("Quellen = %+v", result.Sources)
	}
}

// Eine bekannte, aber defekte Quelldatei erzeugt einen sichtbaren Hinweis; die
// übrigen Quellen werden trotzdem erhoben.
func TestDefekteQuelldateiIstEinHinweisUndKeinAbbruch(t *testing.T) {
	options := newRunProject(t)
	if err := os.WriteFile(filepath.Join(options.ProjectDir, "docker-compose.yml"),
		[]byte("services:\n\t- kaputt\n"), 0o644); err != nil {
		t.Fatalf("schreiben: %v", err)
	}
	result, err := Collect(options)
	if err != nil {
		t.Fatalf("eine defekte Quelldatei darf den Lauf nicht abbrechen: %v", err)
	}
	found := false
	for _, note := range result.Notes {
		if strings.Contains(note.Source, "docker-compose.yml") {
			found = true
		}
	}
	if !found {
		t.Errorf("die defekte Datei muss als Hinweis erscheinen: %+v", result.Notes)
	}
	if len(result.Entries) == 0 {
		t.Error("die übrigen Quellen werden weiter erhoben")
	}
}

// Ein unbekannter Dateityp unterhalb der Projektwurzel wird stillschweigend
// übergangen: nur was gesucht wird, kann fehlen.
func TestUnbekannterDateitypWirdUebergangen(t *testing.T) {
	options := newRunProject(t)
	if err := os.WriteFile(filepath.Join(options.ProjectDir, "notizen.txt"), []byte("egal\n"), 0o644); err != nil {
		t.Fatalf("schreiben: %v", err)
	}
	result, err := Collect(options)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(result.Notes) != 0 || len(result.Rejections) != 0 {
		t.Errorf("kein Hinweis erwartet: %+v / %+v", result.Notes, result.Rejections)
	}
}

// Die Standardquellen werden ergänzt, nicht ersetzt: es gibt keinen Schalter,
// der die Standarderkennung abschaltet.
func TestZusatzquelleErgaenztDieStandardquellen(t *testing.T) {
	options := newRunProject(t)
	extra := filepath.Join(t.TempDir(), "deploy")
	if resolved, err := filepath.EvalSymlinks(filepath.Dir(extra)); err == nil {
		extra = filepath.Join(resolved, "deploy")
	}
	if err := os.MkdirAll(extra, 0o755); err != nil {
		t.Fatalf("anlegen: %v", err)
	}
	if err := os.WriteFile(filepath.Join(extra, "values-prod.yaml"),
		[]byte("image:\n  repository: ghcr.io/example/app\n  tag: 9.9.9\n"), 0o644); err != nil {
		t.Fatalf("schreiben: %v", err)
	}
	writeSources(t, options.SourcesFile, "schema_version: 1\nroots:\n  - "+extra+
		"\nsources:\n  - path: "+extra+"/*.yaml\n    kind: helm\n    env: deployment\n")

	result, err := Collect(options)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(result.Rejections) != 0 {
		t.Fatalf("Ablehnungen = %+v", result.Rejections)
	}
	if len(result.Sources) != 2 {
		t.Fatalf("Quellen = %+v", result.Sources)
	}
	entry := find(t, result, "container/ghcr.io/example/app", filepath.ToSlash(filepath.Join(extra, "values-prod.yaml")))
	if entry.Version != "9.9.9" || entry.Context != EnvDeployment {
		t.Errorf("Zusatzquelle = %+v", entry)
	}
	if !filepath.IsAbs(entry.SourceFile) {
		t.Errorf("außerhalb der Projektwurzel steht der absolute Pfad: %q", entry.SourceFile)
	}
}
