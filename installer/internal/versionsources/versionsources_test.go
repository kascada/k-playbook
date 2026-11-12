package versionsources

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
	"github.com/kascada/k-playbook/installer/internal/kephalaion/kephtest"
)

// doc baut das Dokument: frontmatter zwischen zwei Zeilen ---, darunter ein
// Rumpf. Die erste Zeile des Frontmatters ist Zeile 2 des Dokuments.
func doc(frontmatter string) string {
	return "---\n" + frontmatter + "---\n\nErklärung der Einstellungen.\n"
}

func parse(t *testing.T, frontmatter string) (Config, error) {
	t.Helper()
	return Parse([]byte(doc(frontmatter)))
}

func TestParseLiestGueltigesDokument(t *testing.T) {
	config, err := parse(t, `# Kommentar
title: "Quellen des Versionsinventars"
schema_version: 3

sources:
  - path: deploy/values-prod.yaml
    kind: helm
    env: deployment
    note: Produktionswerte aus dem Deployment-Repo
  - path: extra/requirements.txt
    kind: auto
    env: ci
    optional: true
`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !config.Present || config.SchemaVersion != SchemaVersion || config.Ref != DocRef {
		t.Fatalf("Zustand = %+v", config)
	}
	if len(config.Sources) != 2 {
		t.Fatalf("sources = %d Einträge", len(config.Sources))
	}
	first := config.Sources[0]
	if first.Path != "deploy/values-prod.yaml" || first.Kind != "helm" || first.Env != "deployment" {
		t.Errorf("erster Eintrag = %+v", first)
	}
	if first.Note != "Produktionswerte aus dem Deployment-Repo" || first.Optional {
		t.Errorf("Note oder Optional falsch: %+v", first)
	}
	// Zeile 7 des Dokuments: --- ist Zeile 1, das Frontmatter beginnt in 2.
	if first.Line != 7 {
		t.Errorf("Zeile = %d, erwartet 7 (gezählt ab dem Anfang des Dokuments)", first.Line)
	}
	if !config.Sources[1].Optional {
		t.Errorf("optional: true wurde nicht gelesen: %+v", config.Sources[1])
	}
	if len(config.Rejections) != 0 || len(config.Valid()) != 2 {
		t.Errorf("gültiges Dokument darf nichts ablehnen: %+v", config.Rejections)
	}
}

// Ein fehlendes Dokument ist kein Fehler: es gelten die Standardquellen
// unterhalb der Projektwurzel. Der Zustand bleibt trotzdem ablesbar.
func TestLoadMeldetFehlendesDokumentAlsZustand(t *testing.T) {
	config, err := Load(context.Background(), kephtest.New())
	if err != nil {
		t.Fatalf("ein fehlendes Dokument darf kein Fehler sein: %v", err)
	}
	if config.Present {
		t.Errorf("Present = true für ein fehlendes Dokument")
	}
	if config.Ref != DocRef {
		t.Errorf("Ref = %q, erwartet %q — auch ein fehlendes Dokument muss seinen Ort nennen", config.Ref, DocRef)
	}
	if len(config.Sources) != 0 || len(config.Exclude) != 0 {
		t.Errorf("fehlendes Dokument darf nichts liefern: %+v", config)
	}
}

// Gelesen wird der Rohtext aus der Collection, mit demselben Parser wie
// jede Konfiguration — Kommentare und verschachtelte Listen im Frontmatter
// bleiben lesbar.
func TestLoadLiestRohtextAusDerCollection(t *testing.T) {
	store := kephtest.New()
	store.Put(DocName, doc(`schema_version: 3
# Kommentar vor der Liste
exclude:
  # Kommentar in der Liste
  - "**/testdata/**"
helm_values:
  - path: charts/app/values.yaml   # Zeilenendkommentar
    key: images[0].tag
    item: container/app
`))
	config, err := Load(context.Background(), store)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !config.Present || len(config.Exclude) != 1 || config.Exclude[0] != "**/testdata/**" {
		t.Fatalf("Zustand = %+v", config)
	}
	if len(config.ValidHelmValues()) != 1 || config.HelmValues[0].Path != "charts/app/values.yaml" {
		t.Errorf("helm_values = %+v", config.HelmValues)
	}
}

// Ist die Collection nicht erreichbar, ist das ein eigener Zustand — kein
// „fehlt“, kein Rückfall auf die Standardquellen.
func TestLoadMeldetNichtErreichbar(t *testing.T) {
	store := kephtest.New()
	store.Fail["read"] = errors.New("Node antwortet nicht")
	config, err := Load(context.Background(), store)
	if err == nil || !IsUnavailable(err) {
		t.Fatalf("erwartet UnavailableError, bekam %v", err)
	}
	if !strings.Contains(err.Error(), DocRef) || config.Present {
		t.Errorf("Meldung = %q, Zustand = %+v", err, config)
	}
	if _, err := Load(context.Background(), nil); !IsUnavailable(err) {
		t.Errorf("ohne Zugang erwartet UnavailableError, bekam %v", err)
	}
}

func TestLoadLehntVerzeichnisAb(t *testing.T) {
	reader := readerFunc(func(context.Context, string) (kephalaion.ReadResult, error) {
		return kephalaion.ReadResult{Kind: kephalaion.ReadKindDirectory}, nil
	})
	config, err := Load(context.Background(), reader)
	if err == nil || IsUnavailable(err) || !config.Present {
		t.Fatalf("ein Verzeichnis ist ein defekter Zustand: %+v, %v", config, err)
	}
}

func TestParseBrichtBeiDefektemFormatAb(t *testing.T) {
	cases := map[string]string{
		"nicht lesbares YAML":  doc("schema_version: 3\nexclude: [a\n"),
		"fremde Fassung":       doc("schema_version: 7\n"),
		"alte Fassung 2":       doc("schema_version: 2\n"),
		"alte Fassung 1":       doc("schema_version: 1\n"),
		"fehlende Fassung":     doc("exclude: []\n"),
		"kein Abbildungskopf":  doc("- eins\n- zwei\n"),
		"kein Frontmatter":     "schema_version: 3\n",
		"Frontmatter offen":    "---\nschema_version: 3\n",
		"roots mit Eintrag":    doc("schema_version: 3\nroots:\n  - /srv/deploy\n"),
		"roots leer als Liste": doc("schema_version: 3\nroots: []\n"),
		"roots ohne Wert":      doc("schema_version: 3\nroots:\n"),
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			config, err := Parse([]byte(content))
			if err == nil {
				t.Fatalf("erwartet wurde ein Fehler, keiner kam: %+v", config)
			}
			if IsUnavailable(err) {
				t.Errorf("ein defektes Dokument ist nicht „nicht erreichbar“: %v", err)
			}
			if !config.Present {
				t.Errorf("ein defektes Dokument ist trotzdem vorhanden: %+v", config)
			}
			if len(config.Sources) != 0 || len(config.Exclude) != 0 {
				t.Errorf("bei einem Abbruch darf nichts halb gelesen sein: %+v", config)
			}
			if !strings.Contains(err.Error(), DocRef) {
				t.Errorf("die Meldung muss den Ort nennen: %v", err)
			}
		})
	}
}

// roots wird mit Zeile abgelehnt; die Zeile zählt ab dem Anfang des
// Dokuments.
func TestParseNenntZeileVonRoots(t *testing.T) {
	_, err := parse(t, "schema_version: 3\nroots: []\n")
	if err == nil || !strings.Contains(err.Error(), "Zeile 3") || !strings.Contains(err.Error(), "roots") {
		t.Fatalf("Meldung = %v", err)
	}
}

// Ein unbekanntes Label bricht den Lauf nicht ab, verschwindet aber auch nicht:
// der Eintrag wird abgelehnt, sichtbar, mit dem gefundenen Wert und den
// gültigen. Ebenso eine absolute Quelle.
func TestParseLehntUnbekannteLabelArtenUndAbsolutePfadeSichtbarAb(t *testing.T) {
	config, err := parse(t, `schema_version: 3
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
  - path: /srv/deploy/values.yaml
    kind: helm
    env: deployment
  - path: C:\deploy\values.yaml
    kind: helm
    env: deployment
`)
	if err != nil {
		t.Fatalf("ein Eintragsfehler darf den Lauf nicht abbrechen: %v", err)
	}
	if len(config.Rejections) != 4 {
		t.Fatalf("Ablehnungen = %d, erwartet 4: %+v", len(config.Rejections), config.Rejections)
	}
	if !strings.Contains(config.Rejections[0].Reason, "produktion") ||
		!strings.Contains(config.Rejections[0].Reason, "deployment") {
		t.Errorf("die Ablehnung muss gefundenen und gültige Werte nennen: %q", config.Rejections[0].Reason)
	}
	if !strings.Contains(config.Rejections[1].Reason, "cobol") {
		t.Errorf("die Ablehnung muss die gefundene Quellart nennen: %q", config.Rejections[1].Reason)
	}
	for _, rejection := range config.Rejections[2:] {
		if !strings.Contains(rejection.Reason, "absoluter Pfad") {
			t.Errorf("absolute Quelle: Grund = %q", rejection.Reason)
		}
	}
	if config.Rejections[0].Line != 4 {
		t.Errorf("die Ablehnung muss die Zeile im Dokument nennen: %+v", config.Rejections[0])
	}
	// Das Dokument bleibt vollständig ablesbar; nur der Sammler arbeitet mit
	// weniger Einträgen.
	if len(config.Sources) != 5 {
		t.Errorf("sources = %d, erwartet 5 — die Kontextausgabe zeigt das Dokument wie es ist", len(config.Sources))
	}
	if len(config.Valid()) != 1 || config.Valid()[0].Path != "c.yaml" {
		t.Errorf("Valid() = %+v", config.Valid())
	}
}

func TestParseLiestLeereKonfiguration(t *testing.T) {
	config, err := parse(t, "schema_version: 3\n\nsources: []\n\nexclude: []\n")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !config.Present || config.SchemaVersion != SchemaVersion {
		t.Fatalf("Zustand = %+v", config)
	}
	if len(config.Exclude) != 0 || len(config.Sources) != 0 {
		t.Errorf("leere Listen heißen „nichts konfiguriert\": %+v", config)
	}
}

// `exclude:` nennt die Bereiche, die die Standarderkennung übergeht. Ein
// absolutes Muster hinge vom Rechner ab und wird sichtbar abgelehnt, statt
// stillschweigend nichts zu treffen.
func TestParseLiestAusschlussmusterUndLehntAbsoluteAb(t *testing.T) {
	config, err := parse(t, `schema_version: 3
exclude:
  - tests/fixtures/**
  - /srv/deploy
  - ""
`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
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

// `helm_values:` trägt je Eintrag Datei, Schlüsselpfad und Gegenstand.
// Ungültige Einträge werden sichtbar abgelehnt, bleiben aber in der Liste.
func TestParseLiestHelmValues(t *testing.T) {
	config, err := parse(t, `schema_version: 3
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
  - path: /srv/values.yaml
    key: a.tag
    item: container/x
`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if config.SchemaVersion != SchemaVersion || len(config.HelmValues) != 8 {
		t.Fatalf("Zustand = %+v", config)
	}
	first := config.HelmValues[0]
	if first.Path != "omni-gw/helm/values.yaml" || first.Key != "helm-chart-generic.redis.standalone.tag" ||
		first.Item != "container/redis" || first.ItemName() != "redis" || first.Line != 4 || !first.Valid {
		t.Errorf("erster Eintrag = %+v", first)
	}
	valid := config.ValidHelmValues()
	if len(valid) != 2 || valid[1].Key != "services[0].tag" {
		t.Errorf("gültige Einträge = %+v", valid)
	}
	wanted := []string{"kein `path`", "kein `key`", "ungültiger Schlüsselpfad", "ungültiger Gegenstand", "kein `item`", "absoluter Pfad"}
	if len(config.Rejections) != len(wanted) {
		t.Fatalf("Ablehnungen = %+v", config.Rejections)
	}
	for index, fragment := range wanted {
		if !strings.Contains(config.Rejections[index].Reason, fragment) {
			t.Errorf("Ablehnung %d = %q, erwartet %q", index, config.Rejections[index].Reason, fragment)
		}
	}
}

// Dass kephalaion.ReadResult die Antwort trägt, ist die Schnittstelle zum
// Store; ein Dokument ohne Inhalt ist ein leeres Dokument ohne Frontmatter.
func TestLoadLeeresDokumentIstDefekt(t *testing.T) {
	reader := readerFunc(func(context.Context, string) (kephalaion.ReadResult, error) {
		return kephalaion.ReadResult{Kind: kephalaion.ReadKindDocument}, nil
	})
	config, err := Load(context.Background(), reader)
	if err == nil || IsUnavailable(err) || !config.Present {
		t.Fatalf("leeres Dokument: %+v, %v", config, err)
	}
}

type readerFunc func(context.Context, string) (kephalaion.ReadResult, error)

func (f readerFunc) Read(ctx context.Context, name string) (kephalaion.ReadResult, error) {
	return f(ctx, name)
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
