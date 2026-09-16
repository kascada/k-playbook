package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/versionsources"
)

func versionSourcesPath(root string) string {
	return filepath.Join(LocalDir(root), VersionSourcesFileName)
}

func TestBuildContextMeldetFehlendeQuellenkonfiguration(t *testing.T) {
	root := newContextProject(t)

	context, err := BuildContext(root)
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}
	state := context.VersionSources
	if state == nil {
		t.Fatal("versionSources fehlt in der Ausgabe")
	}
	if state.Present {
		t.Errorf("Present = true für eine fehlende Datei")
	}
	if state.Path != versionSourcesPath(root) {
		t.Errorf("Path = %q — auch eine fehlende Datei muss ihren Ort nennen", state.Path)
	}
	if state.Error != "" || len(state.Roots) != 0 || len(state.Sources) != 0 {
		t.Errorf("fehlende Datei ist kein Fehler: %+v", state)
	}
}

func TestBuildContextGibtQuellenkonfigurationAus(t *testing.T) {
	root := newContextProject(t)
	write(t, versionSourcesPath(root), `schema_version: 1
roots:
  - /srv/deploy
sources:
  - path: /srv/deploy/values-prod.yaml
    kind: helm
    env: deployment
    note: Produktionswerte aus dem Deployment-Repo
exclude:
  - tests/fixtures/**
`)

	context, err := BuildContext(root)
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}
	state := context.VersionSources
	if !state.Present || state.SchemaVersion != 1 || state.Error != "" {
		t.Fatalf("Zustand = %+v", state)
	}
	if len(state.Roots) != 1 || state.Roots[0] != "/srv/deploy" {
		t.Errorf("roots = %v", state.Roots)
	}
	if len(state.Sources) != 1 {
		t.Fatalf("sources = %+v", state.Sources)
	}
	source := state.Sources[0]
	if source.Path != "/srv/deploy/values-prod.yaml" || source.Kind != "helm" || source.Env != "deployment" {
		t.Errorf("Eintrag = %+v", source)
	}
	if source.Note != "Produktionswerte aus dem Deployment-Repo" {
		t.Errorf("note = %q", source.Note)
	}
	if len(state.Exclude) != 1 || state.Exclude[0] != "tests/fixtures/**" {
		t.Errorf("exclude = %v — der Zustand muss vollständig aus context kommen", state.Exclude)
	}
	if len(state.HelmValues) != 0 {
		t.Errorf("helmValues = %+v, erwartet leer", state.HelmValues)
	}
}

// helm_values steht in der Kontextausgabe wie in der Datei — unter Fassung 1
// auch dann, wenn der Sammler den Abschnitt ablehnt.
func TestBuildContextGibtHelmValuesAus(t *testing.T) {
	for _, version := range []string{"1", "2"} {
		t.Run("Fassung "+version, func(t *testing.T) {
			root := newContextProject(t)
			write(t, versionSourcesPath(root), "schema_version: "+version+`
helm_values:
  - path: omni-gw/helm/values.yaml
    key: helm-chart-generic.redis.standalone.tag
    item: container/redis
`)
			context, err := BuildContext(root)
			if err != nil {
				t.Fatalf("BuildContext: %v", err)
			}
			state := context.VersionSources
			if state.Error != "" || len(state.HelmValues) != 1 {
				t.Fatalf("Zustand = %+v", state)
			}
			value := state.HelmValues[0]
			if value.Path != "omni-gw/helm/values.yaml" || value.Key != "helm-chart-generic.redis.standalone.tag" || value.Item != "container/redis" {
				t.Errorf("Eintrag = %+v", value)
			}
			data, err := json.Marshal(state)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), `"helmValues":[{"path":"omni-gw/helm/values.yaml","key":"helm-chart-generic.redis.standalone.tag","item":"container/redis"}]`) {
				t.Errorf("JSON = %s", data)
			}
		})
	}
}

// Eine defekte Zusatzkonfiguration darf nicht jeden Command lahmlegen: der
// Kontextaufruf steht am Anfang jedes Commands. Sichtbar bleibt der Zustand
// trotzdem — dafür ist `error` da.
func TestBuildContextBrichtBeiDefekterQuellenkonfigurationNichtAb(t *testing.T) {
	root := newContextProject(t)
	write(t, versionSourcesPath(root), "schema_version: 1\nroots: [/srv\n")

	context, err := BuildContext(root)
	if err != nil {
		t.Fatalf("der Kontextaufruf darf nicht abbrechen: %v", err)
	}
	state := context.VersionSources
	if !state.Present || state.Error == "" {
		t.Fatalf("Zustand = %+v", state)
	}
	if len(state.Roots) != 0 || len(state.Sources) != 0 {
		t.Errorf("bei einem Fehler bleibt nichts halb gelesen: %+v", state)
	}
}

// Die Vorlage, die /k-gui anlegt, muss von demselben Leser gelesen werden
// können, den der Sammler benutzt — sonst legte das Werkzeug eine Datei an, die
// es selbst ablehnt.
func TestVersionSourcesVorlageIstEineGueltigeLeereKonfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), VersionSourcesFileName)
	if err := os.WriteFile(path, []byte(versionSourcesTemplate()), 0o644); err != nil {
		t.Fatalf("schreiben: %v", err)
	}

	config, err := versionsources.Read(path)
	if err != nil {
		t.Fatalf("die Vorlage muss lesbar sein: %v", err)
	}
	// Die Vorlage bleibt bei Fassung 1: 2 braucht nur, wer helm_values
	// einträgt, und eine neu angelegte Datei soll ältere Installationen
	// desselben Projekts nicht aussperren.
	if config.SchemaVersion != versionsources.MinSchemaVersion {
		t.Errorf("schema_version = %d", config.SchemaVersion)
	}
	if len(config.Roots) != 0 || len(config.Sources) != 0 || len(config.Exclude) != 0 {
		t.Errorf("die Vorlage ist leer: %+v", config)
	}
	if len(config.Rejections) != 0 {
		t.Errorf("die Vorlage darf nichts enthalten, was der Sammler ablehnt: %+v", config.Rejections)
	}
	if !strings.Contains(versionSourcesTemplate(), "handgepflegt") {
		t.Error("der erklärende Kommentar der Vorlage fehlt")
	}
}
