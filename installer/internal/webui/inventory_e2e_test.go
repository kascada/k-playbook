package webui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/inventory"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// Die Fixtures des Sammlers decken die Breite der Quelltypen ab — Python, Go,
// Node, Container, DevContainer, Compose, Helm, CI. Das Dev-Repo kann das
// nicht: es trägt nur Go- und CI-Quellen. Hier wird deshalb dieselbe Fixture
// über die API erhoben statt über das Subkommando, und das Ergebnis verglichen.
//
// Die Fixture wird kopiert, weil ein Projekt einen Anker braucht und in
// testdata/ nichts geschrieben wird.
func fixtureProject(t *testing.T, name string) string {
	t.Helper()

	source, err := filepath.Abs(filepath.Join("..", "inventory", "testdata", "projekte", name))
	if err != nil {
		t.Fatalf("Fixture auflösen: %v", err)
	}
	root := filepath.Join(t.TempDir(), name)
	if resolved, err := filepath.EvalSymlinks(filepath.Dir(root)); err == nil {
		root = filepath.Join(resolved, name)
	}
	if err := os.CopyFS(root, os.DirFS(source)); err != nil {
		t.Fatalf("Fixture kopieren: %v", err)
	}
	if err := project.CreateConfig(root, "."); err != nil {
		t.Fatalf("Konfiguration anlegen: %v", err)
	}
	chdir(t, root)
	return root
}

// Der API-Weg und der Weg der Fachlogik — den das Subkommando nimmt — erzeugen
// dieselbe Datei. Verglichen wird byteweise: die Erhebung über die Fachlogik,
// gerendert mit dem Zeitstempel, den die API geschrieben hat, muss die Datei
// der API sein. Das ist genau der Vergleich, mit dem inventory.Write die
// Byte-Stabilität entscheidet.
func TestVollstaendigeFixtureUeberDieAPIErgibtDieselbeDateiWieDieFachlogik(t *testing.T) {
	root := fixtureProject(t, "vollstaendig")

	code, response := postInventory(t)
	if code != 200 || !response.OK || !response.Outcome.Written {
		t.Fatalf("Anstoß: Status %d, Antwort %+v", code, response)
	}
	written, err := os.ReadFile(response.Outcome.Path)
	if err != nil {
		t.Fatalf("Inventar lesen: %v", err)
	}

	options := inventoryOptions(root)
	result, err := inventory.Collect(options)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if expected := inventory.Render(result, response.Status.GeneratedAt); expected != string(written) {
		t.Errorf("API-Datei und Fachlogik-Rendering unterscheiden sich:\n--- API\n%s\n--- Fachlogik\n%s", written, expected)
	}
	if response.Summary.Entries != len(result.Entries) || response.Summary.Sources != len(result.Sources) ||
		response.Summary.Deviations != len(result.Deviations) {
		t.Errorf("Summary %+v, Fachlogik %d/%d/%d", response.Summary, len(result.Entries), len(result.Sources), len(result.Deviations))
	}

	// Die Breite der Quelltypen ist über die API sichtbar, nicht nur im Test
	// des Sammlers.
	kinds := map[string]bool{}
	for _, source := range result.Sources {
		kinds[source.Kind] = true
	}
	for _, kind := range []string{
		inventory.KindPython, inventory.KindGo, inventory.KindNode, inventory.KindDockerfile,
		inventory.KindCompose, inventory.KindDevcontainer, inventory.KindHelm, inventory.KindCI,
	} {
		if !kinds[kind] {
			t.Errorf("Quellart %s fehlt in der Erhebung: %v", kind, kinds)
		}
	}
	if response.Status.SourcesRead != len(result.Sources) {
		t.Errorf("Status.SourcesRead = %d, Fachlogik %d", response.Status.SourcesRead, len(result.Sources))
	}
}

// Der Fall mit bewusst verschiedenen lokalen, DevContainer- und
// Deployment-Versionen: die Abweichung erscheint in der Antwort — also in der
// Anzeige — und in der Datei, mit allen drei Aussagen nebeneinander.
func TestDreiKontexteAbweichungErscheintInAntwortUndDatei(t *testing.T) {
	fixtureProject(t, "dreikontexte")

	_, response := postInventory(t)
	if !response.OK {
		t.Fatalf("Anstoß: %+v", response)
	}
	if response.Summary.Deviations < 1 || response.Status.Deviations != response.Summary.Deviations {
		t.Errorf("Abweichungen: Summary %d, Status %d", response.Summary.Deviations, response.Status.Deviations)
	}
	if response.Summary.Conflicting != 0 {
		t.Errorf("drei Kontexte sind umgebungsbedingt, nicht widersprüchlich: %+v", response.Summary)
	}

	content, err := os.ReadFile(response.Outcome.Path)
	if err != nil {
		t.Fatalf("Inventar lesen: %v", err)
	}
	text := string(content)
	for _, needle := range []string{"python/poetry", inventory.DeviationEnvironmental, "==1.8.4", "1.8.2", "1.7.1"} {
		if !strings.Contains(text, needle) {
			t.Errorf("in der Datei fehlt %q", needle)
		}
	}

	// Und gerendert im Bereich selbst.
	var file inventoryFileResponse
	getJSON(t, "/api/inventory/file", &file)
	if !strings.Contains(file.HTML, "python/poetry") || !strings.Contains(file.HTML, inventory.DeviationEnvironmental) {
		t.Error("die Abweichung steht nicht in der gerenderten Datei")
	}
}
