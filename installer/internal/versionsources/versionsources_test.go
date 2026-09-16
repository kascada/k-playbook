package versionsources

import (
	"os"
	"path/filepath"
	"strconv"
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

// `helm_values:` wird unter Fassung 2 gelesen; jeder Eintrag trägt Datei,
// Schlüsselpfad und Gegenstand. Ungültige Einträge werden sichtbar abgelehnt,
// bleiben aber in der Liste.
func TestReadLiestHelmValuesUnterFassung2(t *testing.T) {
	config, err := Read(writeConfig(t, `schema_version: 2
helm_values:
  - path: omni-gw/helm/values.yaml
    key: helm-chart-generic.redis.standalone.tag
    item: container/redis
  - path: helm/values-*.yaml
    key: services[0].tag
    item: container/app
  - key: a.tag
    item: container/x
  - path: values.yaml
    item: container/x
  - path: values.yaml
    key: a..tag
    item: container/x
  - path: values.yaml
    key: a.tag
    item: node/alpinejs
  - path: values.yaml
    key: a.tag
`))
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if config.SchemaVersion != 2 || len(config.HelmValues) != 7 {
		t.Fatalf("Zustand = %+v", config)
	}
	first := config.HelmValues[0]
	if first.Path != "omni-gw/helm/values.yaml" || first.Key != "helm-chart-generic.redis.standalone.tag" ||
		first.Item != "container/redis" || first.ItemName() != "redis" || first.Line != 3 || !first.Valid {
		t.Errorf("erster Eintrag = %+v", first)
	}
	valid := config.ValidHelmValues()
	if len(valid) != 2 || valid[1].Key != "services[0].tag" {
		t.Errorf("gültige Einträge = %+v", valid)
	}
	wanted := []string{"kein `path`", "kein `key`", "ungültiger Schlüsselpfad", "ungültiger Gegenstand", "kein `item`"}
	if len(config.Rejections) != len(wanted) {
		t.Fatalf("Ablehnungen = %+v", config.Rejections)
	}
	for index, fragment := range wanted {
		if !strings.Contains(config.Rejections[index].Reason, fragment) {
			t.Errorf("Ablehnung %d = %q, erwartet %q", index, config.Rejections[index].Reason, fragment)
		}
	}
}

// Unter Fassung 1 wird `helm_values` nicht angewandt, sondern einmal sichtbar
// abgelehnt: ein älteres Binary überginge den Abschnitt still, und genau das
// soll eine Datei, die sich auf ihn verlässt, nicht erlauben.
func TestHelmValuesUnterFassung1WerdenAbgelehnt(t *testing.T) {
	config, err := Read(writeConfig(t, `schema_version: 1
helm_values:
  - path: values.yaml
    key: redis.standalone.tag
    item: container/redis
`))
	if err != nil {
		t.Fatalf("Fassung 1 bleibt lesbar: %v", err)
	}
	if len(config.HelmValues) != 1 || config.HelmValues[0].Valid {
		t.Errorf("der Eintrag bleibt sichtbar, gilt aber nicht: %+v", config.HelmValues)
	}
	if len(config.ValidHelmValues()) != 0 {
		t.Errorf("unter Fassung 1 wird nichts angewandt: %+v", config.ValidHelmValues())
	}
	if len(config.Rejections) != 1 || config.Rejections[0].Path != "helm_values" || config.Rejections[0].Line != 3 ||
		!strings.Contains(config.Rejections[0].Reason, "schema_version: 2") {
		t.Errorf("Ablehnungen = %+v", config.Rejections)
	}

	empty, err := Read(writeConfig(t, "schema_version: 1\nhelm_values: []\n"))
	if err != nil || len(empty.Rejections) != 0 {
		t.Errorf("eine leere Liste unter Fassung 1 ist kein Befund: %+v, %v", empty.Rejections, err)
	}
}

func TestParseKeyPath(t *testing.T) {
	for key, want := range map[string]string{
		"a":                    "a",
		"a.b.c":                "a|b|c",
		"helm-chart-generic.x": "helm-chart-generic|x",
		"services[0].tag":      "services[0]|tag",
		"matrix[1][2].tag":     "matrix[1][2]|tag",
		"":                     "",
		"a..b":                 "",
		".a":                   "",
		"a.":                   "",
		"[0].a":                "",
		"a[x]":                 "",
		"a[-1]":                "",
		"a[]":                  "",
		"a[0]b":                "",
		"a[0":                  "",
		"a]":                   "",
	} {
		steps, ok := ParseKeyPath(key)
		got := ""
		if ok {
			var parts []string
			for _, step := range steps {
				part := step.Key
				for _, index := range step.Indexes {
					part += "[" + strconv.Itoa(index) + "]"
				}
				parts = append(parts, part)
			}
			got = strings.Join(parts, "|")
		}
		if got != want {
			t.Errorf("ParseKeyPath(%q) = %q (%v), erwartet %q", key, got, ok, want)
		}
	}
}
