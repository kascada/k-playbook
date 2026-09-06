package versionsources

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "version-sources.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("schreiben: %v", err)
	}
	return path
}

func TestReadLiestGueltigeDatei(t *testing.T) {
	path := writeConfig(t, `# Kommentar
schema_version: 1

roots:
  - /srv/deploy

sources:
  - path: /srv/deploy/values-prod.yaml
    kind: helm
    env: deployment
    note: Produktionswerte aus dem Deployment-Repo
  - path: extra/requirements.txt
    kind: auto
    env: ci
    optional: true
`)
	config, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !config.Present || config.SchemaVersion != 1 {
		t.Fatalf("Zustand = %+v", config)
	}
	if len(config.Roots) != 1 || config.Roots[0] != "/srv/deploy" {
		t.Errorf("roots = %v", config.Roots)
	}
	if len(config.Sources) != 2 {
		t.Fatalf("sources = %d Einträge", len(config.Sources))
	}
	first := config.Sources[0]
	if first.Path != "/srv/deploy/values-prod.yaml" || first.Kind != "helm" || first.Env != "deployment" {
		t.Errorf("erster Eintrag = %+v", first)
	}
	if first.Note != "Produktionswerte aus dem Deployment-Repo" || first.Optional {
		t.Errorf("Note oder Optional falsch: %+v", first)
	}
	if !config.Sources[1].Optional {
		t.Errorf("optional: true wurde nicht gelesen: %+v", config.Sources[1])
	}
	if len(config.Rejections) != 0 || len(config.Valid()) != 2 {
		t.Errorf("gültige Datei darf nichts ablehnen: %+v", config.Rejections)
	}
}

// Eine fehlende Datei ist kein Fehler: es gelten die Standardquellen unterhalb
// der Projektwurzel. Der Zustand bleibt trotzdem ablesbar.
func TestReadMeldetFehlendeDateiAlsZustand(t *testing.T) {
	path := filepath.Join(t.TempDir(), "version-sources.yaml")

	config, err := Read(path)
	if err != nil {
		t.Fatalf("eine fehlende Datei darf kein Fehler sein: %v", err)
	}
	if config.Present {
		t.Errorf("Present = true für eine fehlende Datei")
	}
	if config.Path != path {
		t.Errorf("Path = %q, erwartet %q — auch eine fehlende Datei muss ihren Ort nennen", config.Path, path)
	}
	if len(config.Sources) != 0 || len(config.Roots) != 0 {
		t.Errorf("fehlende Datei darf nichts liefern: %+v", config)
	}
}

func TestReadBrichtBeiDefektemFormatAb(t *testing.T) {
	cases := map[string]string{
		"nicht lesbares YAML": "schema_version: 1\nroots: [/srv/a\n",
		"fremde Fassung":      "schema_version: 7\n",
		"fehlende Fassung":    "roots: []\n",
		"kein Abbildungskopf": "- eins\n- zwei\n",
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			config, err := Read(writeConfig(t, content))
			if err == nil {
				t.Fatalf("erwartet wurde ein Fehler, keiner kam: %+v", config)
			}
			if !config.Present {
				t.Errorf("eine defekte Datei ist trotzdem vorhanden: %+v", config)
			}
			if len(config.Sources) != 0 || len(config.Roots) != 0 {
				t.Errorf("bei einem Abbruch darf nichts halb gelesen sein: %+v", config)
			}
		})
	}
}

// Ein unbekanntes Label bricht den Lauf nicht ab, verschwindet aber auch nicht:
// der Eintrag wird abgelehnt, sichtbar, mit dem gefundenen Wert und den
// gültigen.
func TestReadLehntUnbekannteLabelUndArtenSichtbarAb(t *testing.T) {
	config, err := Read(writeConfig(t, `schema_version: 1
sources:
  - path: a.yaml
    kind: helm
    env: produktion
  - path: b.txt
    kind: cobol
    env: ci
  - path: c.yaml
    kind: helm
    env: deployment
`))
	if err != nil {
		t.Fatalf("ein Eintragsfehler darf den Lauf nicht abbrechen: %v", err)
	}
	if len(config.Rejections) != 2 {
		t.Fatalf("Ablehnungen = %d, erwartet 2: %+v", len(config.Rejections), config.Rejections)
	}
	if !strings.Contains(config.Rejections[0].Reason, "produktion") ||
		!strings.Contains(config.Rejections[0].Reason, "deployment") {
		t.Errorf("die Ablehnung muss gefundenen und gültige Werte nennen: %q", config.Rejections[0].Reason)
	}
	if !strings.Contains(config.Rejections[1].Reason, "cobol") {
		t.Errorf("die Ablehnung muss die gefundene Quellart nennen: %q", config.Rejections[1].Reason)
	}
	if config.Rejections[0].Line == 0 {
		t.Errorf("die Ablehnung muss die Zeile nennen: %+v", config.Rejections[0])
	}
	// Die Datei bleibt vollständig ablesbar; nur der Sammler arbeitet mit
	// weniger Einträgen.
	if len(config.Sources) != 3 {
		t.Errorf("sources = %d, erwartet 3 — die Kontextausgabe zeigt die Datei wie sie ist", len(config.Sources))
	}
	if len(config.Valid()) != 1 || config.Valid()[0].Path != "c.yaml" {
		t.Errorf("Valid() = %+v", config.Valid())
	}
}

func TestReadLiestLeereKonfigurationDerVorlage(t *testing.T) {
	config, err := Read(writeConfig(t, "schema_version: 1\n\nroots: []\n\nsources: []\n"))
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !config.Present || config.SchemaVersion != 1 {
		t.Fatalf("Zustand = %+v", config)
	}
	if len(config.Roots) != 0 || len(config.Sources) != 0 {
		t.Errorf("leere Listen heißen „nichts konfiguriert\": %+v", config)
	}
}

// `exclude:` nennt die Bereiche, die die Standarderkennung übergeht. Ein
// absolutes Muster hinge vom Rechner ab und wird sichtbar abgelehnt, statt
// stillschweigend nichts zu treffen.
func TestReadLiestAusschlussmusterUndLehntAbsoluteAb(t *testing.T) {
	path := writeConfig(t, `schema_version: 1
exclude:
  - tests/fixtures/**
  - /srv/deploy
  - ""
`)

	config, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(config.Exclude) != 1 || config.Exclude[0] != "tests/fixtures/**" {
		t.Fatalf("exclude = %v", config.Exclude)
	}
	if len(config.Rejections) != 2 {
		t.Fatalf("Ablehnungen = %+v", config.Rejections)
	}
	if !strings.Contains(config.Rejections[0].Reason, "relativ zur Projektwurzel") {
		t.Errorf("Grund = %q", config.Rejections[0].Reason)
	}
	if !strings.Contains(config.Rejections[1].Reason, "leeres Ausschlussmuster") {
		t.Errorf("Grund = %q", config.Rejections[1].Reason)
	}
}
