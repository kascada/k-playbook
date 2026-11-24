package project

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
	"github.com/kascada/k-playbook/installer/internal/versionsources"
)

// okStatus ist eine bereite Anbindung, wie KephalaionStatus sie meldete.
var okStatus = kephalaion.Status{Configured: true, State: kephalaion.StateOK, OK: true, Message: "Angemeldet."}

// versionSourcesDoc ist der Ort des Dokuments in der Collection eines
// Testprojekts (isolateDefaults baut sie aus testCollectionDir).
func versionSourcesDoc(root string) string {
	return filepath.Join(root, testCollectionDir, versionsources.DocName)
}

func TestVersionSourcesMeldetFehlendesDokument(t *testing.T) {
	root := newContextProject(t)

	state := versionSourcesWithStatus(root, okStatus)
	if state.State != VersionSourcesMissing || state.Present {
		t.Errorf("Zustand = %+v, erwartet missing", state)
	}
	if state.Ref != versionsources.DocRef {
		t.Errorf("Ref = %q — auch ein fehlendes Dokument muss seinen Ort nennen", state.Ref)
	}
	if state.Error != "" || len(state.Sources) != 0 {
		t.Errorf("fehlendes Dokument ist kein Fehler: %+v", state)
	}
}

func TestVersionSourcesGibtKonfigurationAus(t *testing.T) {
	root := newContextProject(t)
	write(t, versionSourcesDoc(root), `---
title: "Quellen des Versionsinventars"
schema_version: 3
sources:
  - path: deploy/values-prod.yaml
    kind: helm
    env: deployment
    note: Produktionswerte aus dem Deployment-Repo
exclude:
  # Testfixtures
  - tests/fixtures/**
helm_values:
  - path: omni-gw/helm/values.yaml
    key: helm-chart-generic.redis.standalone.tag
    item: container/redis
---

Erklärung.
`)

	state := versionSourcesWithStatus(root, okStatus)
	if state.State != VersionSourcesPresent || !state.Present || state.SchemaVersion != 3 || state.Error != "" {
		t.Fatalf("Zustand = %+v", state)
	}
	if len(state.Sources) != 1 {
		t.Fatalf("sources = %+v", state.Sources)
	}
	source := state.Sources[0]
	if source.Path != "deploy/values-prod.yaml" || source.Kind != "helm" || source.Env != "deployment" ||
		source.Note != "Produktionswerte aus dem Deployment-Repo" {
		t.Errorf("Eintrag = %+v", source)
	}
	if len(state.Exclude) != 1 || state.Exclude[0] != "tests/fixtures/**" {
		t.Errorf("exclude = %v — der Zustand muss vollständig aus context kommen", state.Exclude)
	}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"ref":"keph:/k-playbook/inventory.md"`, `"state":"present"`,
		`"helmValues":[{"path":"omni-gw/helm/values.yaml","key":"helm-chart-generic.redis.standalone.tag","item":"container/redis"}]`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("JSON ohne %s: %s", want, data)
		}
	}
	if strings.Contains(string(data), `"roots"`) || strings.Contains(string(data), `"path":"/`) {
		t.Errorf("roots und ein Pfad auf der Platte gibt es nicht mehr: %s", data)
	}
}

// Eine defekte Zusatzkonfiguration darf nicht jeden Command lahmlegen: der
// Kontextaufruf steht am Anfang jedes Commands. Sichtbar bleibt der Zustand
// trotzdem — dafür sind state und error da.
func TestVersionSourcesDefektIstZustand(t *testing.T) {
	for name, content := range map[string]string{
		"nicht lesbar": "---\nschema_version: 3\nexclude: [a\n---\n",
		"roots":        "---\nschema_version: 3\nroots: []\n---\n",
		"Fassung 2":    "---\nschema_version: 2\n---\n",
	} {
		t.Run(name, func(t *testing.T) {
			root := newContextProject(t)
			write(t, versionSourcesDoc(root), content)
			state := versionSourcesWithStatus(root, okStatus)
			if state.State != VersionSourcesInvalid || !state.Present || state.Error == "" {
				t.Fatalf("Zustand = %+v", state)
			}
			if len(state.Sources) != 0 || len(state.Exclude) != 0 {
				t.Errorf("bei einem Fehler bleibt nichts halb gelesen: %+v", state)
			}
		})
	}
}

// Ist Kephalaion nicht bereit, ist das ein eigener Zustand — nicht „fehlt“.
func TestVersionSourcesNichtErreichbarFaelltNichtZurueck(t *testing.T) {
	root := newContextProject(t)
	write(t, versionSourcesDoc(root), "---\nschema_version: 3\nexclude:\n  - x/**\n---\n")

	state := versionSourcesWithStatus(root, kephalaion.Status{State: kephalaion.StateNodeUnreachable, Message: "Node antwortet nicht."})
	if state.State != VersionSourcesUnavailable || state.Present || !strings.Contains(state.Error, "Node antwortet nicht.") {
		t.Fatalf("Zustand = %+v", state)
	}
	if len(state.Exclude) != 0 {
		t.Errorf("kein Rückfall: %+v", state)
	}
}

// Der Teilabruf environment trägt versionSources aus der Collection, mit dem
// Status der Anbindung.
func TestBriefingEnvironmentTraegtVersionSources(t *testing.T) {
	root := newContextProject(t)
	stubKephalaionCheck(t, okStatus)
	write(t, versionSourcesDoc(root), "---\nschema_version: 3\nexclude:\n  - \"**/testdata/**\"\n---\n")

	sources := BuildBriefingEnvironment(root, root).VersionSources
	if sources == nil || sources.State != VersionSourcesPresent ||
		len(sources.Exclude) != 1 || sources.Exclude[0] != "**/testdata/**" {
		t.Fatalf("versionSources = %+v", sources)
	}
}
