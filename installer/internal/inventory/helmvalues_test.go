package inventory

import (
	"strings"
	"testing"
)

const redisValues = "helm-chart-generic:\n" +
	"  redis:\n" +
	"    enabled: true\n" +
	"    standalone:\n" +
	"      tag: v7.4.11\n" +
	"      resources:\n" +
	"        limits:\n" +
	"          cpu: 256m\n" +
	"    repository: registry.example.io\n"

func helmValuesConfig(entries string) string {
	return "schema_version: 2\nhelm_values:\n" + entries
}

// notesFor liefert die Hinweise, die einen Text enthalten.
func notesFor(result Result, fragment string) []Note {
	var found []Note
	for _, note := range result.Notes {
		if strings.Contains(note.Text, fragment) {
			found = append(found, note)
		}
	}
	return found
}

// Der Anlassfall: Compose lokal mit `redis:7.4.11-alpine`, Helm mit einem Tag,
// den erst die Konfiguration einem Gegenstand zuordnet. Beide landen in
// derselben Gruppe, und die Abweichung ist umgebungsbedingt.
func TestKonfigurierterHelmWertLandetInDerComposeGruppe(t *testing.T) {
	result := collectFiles(t, map[string]string{
		"docker-compose.yml": "services:\n  redis:\n    image: redis:7.4.11-alpine\n",
		"helm/values.yaml":   redisValues,
		"version-sources.yaml": helmValuesConfig("  - path: helm/values.yaml\n" +
			"    key: helm-chart-generic.redis.standalone.tag\n" +
			"    item: container/redis\n"),
	})

	helm := find(t, result, "container/redis", "helm/values.yaml")
	if helm.Version != "v7.4.11" || helm.Pin != PinExact || helm.Context != EnvDeployment ||
		helm.SourceKey != "helm-chart-generic.redis.standalone.tag" || helm.SourceLine != 5 ||
		helm.KindOfThing != ThingImage || helm.Note != configuredValueNote {
		t.Errorf("Helm-Zeile = %+v", helm)
	}
	var deviation *Deviation
	for index := range result.Deviations {
		if result.Deviations[index].Group == "container/redis" {
			deviation = &result.Deviations[index]
		}
	}
	if deviation == nil || deviation.Art != DeviationEnvironmental || len(deviation.Entries) != 2 {
		t.Fatalf("Abweichungen = %+v", result.Deviations)
	}
	if len(result.Notes) != 0 || len(result.Rejections) != 0 {
		t.Errorf("Hinweise %+v, Ablehnungen %+v", result.Notes, result.Rejections)
	}

	rendered := Render(result, "2026-09-16T00:00:00Z")
	if !strings.Contains(rendered, "| `v7.4.11` | exact | deployment | `helm/values.yaml:5` · `helm-chart-generic.redis.standalone.tag` · konfiguriert in version-sources.yaml |") {
		t.Errorf("Herkunft mit Schlüsselpfad und Zeile fehlt:\n%s", rendered)
	}
}

// Die Zeile trägt den Kontext der Quelle, als die die Datei gelesen wird —
// auch ein `env` aus `sources`.
func TestKonfigurierterHelmWertNimmtDenKontextDerQuelle(t *testing.T) {
	result := collectFiles(t, map[string]string{
		"helm/values.yaml": redisValues,
		"version-sources.yaml": "schema_version: 2\n" +
			"sources:\n  - path: helm/values.yaml\n    kind: helm\n    env: dev\n" +
			"helm_values:\n  - path: helm/values.yaml\n    key: helm-chart-generic.redis.standalone.tag\n    item: container/redis\n",
	})
	entry := find(t, result, "container/redis", "helm/values.yaml")
	if entry.Context != EnvDev || entry.ContextOrigin != ContextConfigured {
		t.Errorf("Kontext = %+v", entry)
	}
}

// Jeder Eintrag, der keine Zeile ergibt, hinterlässt einen Hinweis mit Grund.
func TestKonfigurierterHelmWertOhneZeileErzeugtHinweis(t *testing.T) {
	cases := []struct {
		name   string
		files  map[string]string
		path   string
		key    string
		source string
		reason string
	}{
		{"Schlüssel fehlt", map[string]string{"helm/values.yaml": redisValues},
			"helm/values.yaml", "helm-chart-generic.redis.cluster.tag", "helm/values.yaml", "der Schlüssel fehlt in der Datei"},
		{"Wert ohne Skalar", map[string]string{"helm/values.yaml": redisValues},
			"helm/values.yaml", "helm-chart-generic.redis.standalone", "helm/values.yaml", "der Schlüssel hat keinen Skalar"},
		{"Index außerhalb der Liste", map[string]string{"helm/values.yaml": "list:\n  - tag: 1\n"},
			"helm/values.yaml", "list[3].tag", "helm/values.yaml", "der Schlüssel fehlt in der Datei"},
		{"andere Quellart", map[string]string{"docker-compose.yml": "services:\n  redis:\n    image: redis:7\n"},
			"docker-compose.yml", "services.redis.image", "docker-compose.yml", "die Datei wird als compose gelesen, nicht als Helm-values"},
		{"Chart.yaml", map[string]string{"helm/Chart.yaml": "name: app\nversion: 1.0.0\n"},
			"helm/Chart.yaml", "version", "helm/Chart.yaml", "die Datei wird als Chart.yaml gelesen, nicht als Helm-values"},
		{"keine Quelle", map[string]string{"helm/prod.yaml": redisValues},
			"helm/prod.yaml", "helm-chart-generic.redis.standalone.tag", "helm/prod.yaml", "die Datei ist keine Quelle des Inventars"},
		{"nicht gefunden", map[string]string{},
			"helm/values.yaml", "a.tag", "helm/values.yaml", "die Datei liegt nicht auf der Platte"},
		{"ausgeschlossen", map[string]string{"k-playbook/helm/values.yaml": redisValues},
			"k-playbook/helm/values.yaml", "helm-chart-generic.redis.standalone.tag", "k-playbook/helm/values.yaml", "Ausschlussregel `k-playbook/**`"},
		{"nicht auswertbar", map[string]string{"helm/values.yaml": "a:\n  b: [\n"},
			"helm/values.yaml", "a.b", "helm/values.yaml", "die Datei ist nicht auswertbar"},
		{"Muster ohne Treffer", map[string]string{},
			"helm/values-*.yaml", "a.tag", "helm/values-*.yaml", "das Muster trifft keine Datei"},
		{"außerhalb der Wurzeln", map[string]string{},
			"/srv/deploy/values.yaml", "a.tag", "/srv/deploy/values.yaml", "außerhalb der erlaubten Wurzeln"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			files := map[string]string{}
			for name, content := range tc.files {
				files[name] = content
			}
			files["version-sources.yaml"] = helmValuesConfig("  - path: " + tc.path + "\n    key: " + tc.key + "\n    item: container/redis\n")
			result := collectFiles(t, files)

			for _, entry := range result.Entries {
				if entry.Note == configuredValueNote {
					t.Errorf("unerwartete Zeile: %+v", entry)
				}
			}
			notes := notesFor(result, "konfigurierter Helm-Wert "+tc.key+" → container/redis (version-sources.yaml, Zeile 3) nicht angewandt: ")
			if len(notes) != 1 {
				t.Fatalf("Hinweise = %+v, erwartet genau einen", result.Notes)
			}
			if notes[0].Source != tc.source || !strings.Contains(notes[0].Text, tc.reason) {
				t.Errorf("Hinweis = %+v, erwartet Quelle %q und Grund %q", notes[0], tc.source, tc.reason)
			}
			for _, source := range result.Sources {
				if source.Unevaluable && tc.name != "nicht auswertbar" {
					t.Errorf("ein wirkungsloser Helm-Wert setzt keinen Zustand: %+v", source)
				}
			}
		})
	}
}

// Ein Glob über zwei Dateien, der Schlüssel nur in einer: eine Zeile und ein
// Hinweis für die andere Datei.
func TestKonfigurierterHelmWertMitGlobMeldetJedeDatei(t *testing.T) {
	result := collectFiles(t, map[string]string{
		"helm/values-prod.yaml":  redisValues,
		"helm/values-stage.yaml": "helm-chart-generic:\n  redis:\n    enabled: false\n",
		"version-sources.yaml": helmValuesConfig("  - path: helm/values-*.yaml\n" +
			"    key: helm-chart-generic.redis.standalone.tag\n" +
			"    item: container/redis\n"),
	})

	find(t, result, "container/redis", "helm/values-prod.yaml")
	for _, entry := range result.Entries {
		if entry.SourceFile == "helm/values-stage.yaml" {
			t.Errorf("unerwartete Zeile: %+v", entry)
		}
	}
	notes := notesFor(result, "nicht angewandt")
	if len(notes) != 1 || notes[0].Source != "helm/values-stage.yaml" || !strings.Contains(notes[0].Text, "der Schlüssel fehlt in der Datei") {
		t.Errorf("Hinweise = %+v", result.Notes)
	}
}

// Unter schema_version 1 wird der Abschnitt sichtbar abgelehnt und ergibt
// keine Zeile.
func TestHelmValuesUnterFassung1ErgebenKeineZeile(t *testing.T) {
	result := collectFiles(t, map[string]string{
		"helm/values.yaml": redisValues,
		"version-sources.yaml": "schema_version: 1\nhelm_values:\n  - path: helm/values.yaml\n" +
			"    key: helm-chart-generic.redis.standalone.tag\n    item: container/redis\n",
	})
	for _, entry := range result.Entries {
		if entry.Group == "container/redis" {
			t.Errorf("unter Fassung 1 darf keine Zeile entstehen: %+v", entry)
		}
	}
	if len(result.Rejections) != 1 || !strings.Contains(result.Rejections[0].Reason, "helm_values verlangt schema_version: 2") ||
		!strings.Contains(result.Rejections[0].Reason, "Zeile 3") {
		t.Errorf("Ablehnungen = %+v", result.Rejections)
	}
}

// Die Pin-Regeln der Container-Tags gelten für den konfigurierten Wert.
func TestKonfigurierterHelmWertPinRegeln(t *testing.T) {
	result := collectFiles(t, map[string]string{
		"helm/values.yaml": "a:\n  tag: \"\"\nb:\n  tag: latest\nc:\n  tag: ${TAG}\nd:\n  tag: ubuntu-22.04\n",
		"version-sources.yaml": helmValuesConfig(
			"  - path: helm/values.yaml\n    key: a.tag\n    item: container/a\n" +
				"  - path: helm/values.yaml\n    key: b.tag\n    item: container/b\n" +
				"  - path: helm/values.yaml\n    key: c.tag\n    item: container/c\n" +
				"  - path: helm/values.yaml\n    key: d.tag\n    item: container/Registry.Example.io/D\n"),
	})
	for group, pin := range map[string]string{
		"container/a": PinFloating, "container/b": PinFloating, "container/c": PinUnknown,
		"container/registry.example.io/d": PinExact,
	} {
		entry := find(t, result, group, "helm/values.yaml")
		if entry.Pin != pin {
			t.Errorf("%s: Pin %q, erwartet %q", group, entry.Pin, pin)
		}
		if pin == PinUnknown && !strings.Contains(entry.Note, "nicht auflösbar") {
			t.Errorf("%s: unknown braucht einen Grund: %+v", group, entry)
		}
	}
}

// Das Inventar bleibt deterministisch, auch mit Hinweisen aus Globs.
func TestKonfigurierteHelmWerteSindDeterministisch(t *testing.T) {
	files := map[string]string{
		"helm/values-a.yaml": "x: 1\n",
		"helm/values-b.yaml": "x: 1\n",
		"helm/values-c.yaml": redisValues,
		"version-sources.yaml": helmValuesConfig("  - path: helm/values-*.yaml\n" +
			"    key: helm-chart-generic.redis.standalone.tag\n    item: container/redis\n"),
	}
	first := Render(collectFiles(t, files), "2026-09-16T00:00:00Z")
	for range 5 {
		if again := Render(collectFiles(t, files), "2026-09-16T00:00:00Z"); again != first {
			t.Fatalf("zwei Läufe ergeben verschiedene Dateien")
		}
	}
}
