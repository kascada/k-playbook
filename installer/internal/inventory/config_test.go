package inventory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/versionsources"
)

// configKey ist in collectFiles der Schlüssel, unter dem ein Test das
// Frontmatter der Quellenkonfiguration statt einer Datei angibt.
const configKey = "@quellenkonfiguration"

// sourcesDoc baut das Dokument der Quellenkonfiguration
// (versionsources.DocRef) aus seinem Frontmatter.
func sourcesDoc(frontmatter string) string {
	return "---\n" + frontmatter + "---\n\nQuellen des Versionsinventars.\n"
}

// sourcesConfig wertet das Frontmatter aus, wie Run es aus der Collection
// täte; ein Fehler lässt den Test scheitern.
func sourcesConfig(t *testing.T, frontmatter string) versionsources.Config {
	t.Helper()
	config, err := versionsources.Parse([]byte(sourcesDoc(frontmatter)))
	if err != nil {
		t.Fatalf("Quellenkonfiguration: %v", err)
	}
	return config
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
// `k-playbook context`, der sie als Zustand ausgibt und weiterläuft. Ebenso,
// wenn die Collection nicht erreichbar ist: kein Rückfall auf die
// Standardquellen.
func TestDefekteQuellenkonfigurationBrichtDenLaufAb(t *testing.T) {
	for name, content := range map[string]string{
		"nicht lesbares YAML": sourcesDoc("schema_version: 3\nexclude: [a\n"),
		"fremde Fassung":      sourcesDoc("schema_version: 2\n"),
		"roots":               sourcesDoc("schema_version: 3\nroots: []\n"),
		"kein Frontmatter":    "schema_version: 3\n",
	} {
		t.Run(name, func(t *testing.T) {
			options := newRunProject(t)
			store := newRunStore(t)
			store.Put(versionsources.DocName, content)
			if _, _, err := runInto(t, store, options); err == nil {
				t.Fatal("Run muss abbrechen")
			}
			if len(store.Names()) != 1 {
				t.Errorf("bei einem Abbruch wird nichts geschrieben: %v", store.Names())
			}
			for _, call := range store.Calls {
				if !strings.HasPrefix(call, "read") {
					t.Errorf("bei einem Abbruch wird nur die Konfiguration gelesen: %v", store.Calls)
				}
			}
		})
	}
	t.Run("nicht erreichbar", func(t *testing.T) {
		options := newRunProject(t)
		store := newRunStore(t)
		store.Fail["read"] = os.ErrDeadlineExceeded
		_, _, err := runInto(t, store, options)
		if err == nil || !versionsources.IsUnavailable(err) {
			t.Fatalf("erwartet „nicht erreichbar“, bekam %v", err)
		}
		if len(store.Names()) != 0 {
			t.Errorf("nichts geschrieben: %v", store.Names())
		}
	})
}

// Run liest die Konfiguration aus der Collection: Der Ausschluss im
// Dokument wirkt, ohne dass eine Datei auf der Platte liegt.
func TestRunLiestDieKonfigurationAusDerCollection(t *testing.T) {
	options := newRunProject(t)
	if err := os.MkdirAll(filepath.Join(options.ProjectDir, "fixtures"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(options.ProjectDir, "fixtures", "requirements.txt"), []byte("django==1.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := newRunStore(t)
	store.Put(versionsources.DocName, sourcesDoc("schema_version: 3\nexclude:\n  # Fixtures\n  - \"fixtures/**\"\n"))
	result, _, err := runInto(t, store, options)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	skipped := 0
	for _, exclusion := range result.Exclusions {
		if exclusion.Pattern == "fixtures/**" {
			skipped = exclusion.Skipped
		}
	}
	if skipped != 1 {
		t.Errorf("Ausschluss aus der Collection: %+v", result.Exclusions)
	}
}

// Ein unbekanntes Umgebungslabel lehnt den Eintrag ab, nicht den Lauf — und die
// Ablehnung steht sichtbar im Ergebnis.
func TestUnbekanntesLabelLehntDenEintragSichtbarAb(t *testing.T) {
	options := newRunProject(t)
	options.Config = sourcesConfig(t, `schema_version: 3
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
	if !strings.Contains(result.Rejections[0].Reason, versionsources.DocRef+", Zeile 4") {
		t.Errorf("die Meldung muss auf das Dokument und die Zeile darin zeigen: %q", result.Rejections[0].Reason)
	}
	if result.ConfiguredSources != 1 {
		t.Errorf("ein abgelehnter Eintrag zählt weiterhin als konfiguriert: %d", result.ConfiguredSources)
	}
}

// Eine konfigurierte, aber fehlende Quelle ist ein sichtbarer Hinweis — es sei
// denn, der Eintrag trägt optional: true.
func TestFehlendeKonfigurierteQuelleIstEinHinweis(t *testing.T) {
	options := newRunProject(t)
	options.Config = sourcesConfig(t, `schema_version: 3
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
	options.Config = sourcesConfig(t, `schema_version: 3
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
	extra := filepath.Join(options.ProjectDir, "deploy")
	if err := os.MkdirAll(extra, 0o755); err != nil {
		t.Fatalf("anlegen: %v", err)
	}
	if err := os.WriteFile(filepath.Join(extra, "values-prod.yaml"),
		[]byte("image:\n  repository: ghcr.io/example/app\n  tag: 9.9.9\n"), 0o644); err != nil {
		t.Fatalf("schreiben: %v", err)
	}
	options.Config = sourcesConfig(t, "schema_version: 3\nsources:\n  - path: deploy/*.yaml\n    kind: helm\n    env: deployment\n")

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
	entry := find(t, result, "container/ghcr.io/example/app", "deploy/values-prod.yaml")
	if entry.Version != "9.9.9" || entry.Context != EnvDeployment {
		t.Errorf("Zusatzquelle = %+v", entry)
	}
}

// Eine absolute Quelle wird abgelehnt, auch wenn sie unter der Projektwurzel
// liegt: Die Konfiguration gilt auf jedem Rechner, ein absoluter Pfad nur auf
// einem. Außerhalb der Projektwurzel gibt es nichts mehr freizugeben.
func TestAbsoluteQuelleWirdAbgelehnt(t *testing.T) {
	options := newRunProject(t)
	inside := filepath.ToSlash(filepath.Join(options.ProjectDir, "requirements.txt"))
	options.Config = sourcesConfig(t, "schema_version: 3\nsources:\n  - path: "+inside+
		"\n    kind: python\n    env: deployment\n  - path: ../draussen/values.yaml\n    kind: helm\n    env: deployment\n")

	result, err := Collect(options)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(result.Rejections) != 2 {
		t.Fatalf("Ablehnungen = %+v", result.Rejections)
	}
	reasons := result.Rejections[0].Reason + " / " + result.Rejections[1].Reason
	if !strings.Contains(reasons, "absoluter Pfad") || !strings.Contains(reasons, "außerhalb der Projektwurzel") {
		t.Errorf("Gründe = %s", reasons)
	}
	entry := find(t, result, "python/redis", "requirements.txt")
	if entry.ContextOrigin == ContextConfigured {
		t.Errorf("die abgelehnte absolute Quelle darf das Label nicht setzen: %+v", entry)
	}
}
