package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/inventory"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// Der Subkommando-Weg über die Fixture des Sammlers: runInventory sammelt nur
// die drei Pfade zusammen und ruft inventory.Run — die Datei muss deshalb
// byteweise das sein, was die Fachlogik für dieselben Optionen rendert. Der
// API-Weg wird in internal/webui gegen dieselbe Fachlogik geprüft; zusammen
// belegen beide Tests, dass Subkommando und API dieselbe Ausgabe erzeugen.
func fixtureProject(t *testing.T, name string) string {
	t.Helper()

	source, err := filepath.Abs(filepath.Join("..", "..", "internal", "inventory", "testdata", "projekte", name))
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

	before, err := os.Getwd()
	if err != nil {
		t.Fatalf("Arbeitsverzeichnis: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("nach %s wechseln: %v", root, err)
	}
	t.Cleanup(func() { os.Chdir(before) })
	return root
}

func fixtureOptions(root string) inventory.Options {
	localDir := project.LocalDir(root)
	return inventory.Options{
		ProjectDir:    root,
		SourcesFile:   filepath.Join(localDir, project.VersionSourcesFileName),
		InventoryFile: inventory.FilePath(localDir),
	}
}

func TestSubkommandoUeberDieFixtureErgibtDieDateiDerFachlogik(t *testing.T) {
	root := fixtureProject(t, "vollstaendig")

	if err := runInventory(nil); err != nil {
		t.Fatalf("runInventory: %v", err)
	}
	options := fixtureOptions(root)
	written, err := os.ReadFile(options.InventoryFile)
	if err != nil {
		t.Fatalf("Inventar lesen: %v", err)
	}
	status := inventory.ReadStatus(options.InventoryFile)
	if status.Problem != "" || status.GeneratedBy != inventory.GeneratedBy {
		t.Fatalf("Status = %+v", status)
	}

	result, err := inventory.Collect(options)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if expected := inventory.Render(result, status.GeneratedAt); expected != string(written) {
		t.Error("die Datei des Subkommandos ist nicht das Rendering der Fachlogik")
	}
	if status.Deviations != len(result.Deviations) || status.SourcesRead != len(result.Sources) {
		t.Errorf("Status %+v, Fachlogik %d Abweichungen, %d Quellen", status, len(result.Deviations), len(result.Sources))
	}
}

// Ein abgelehnter Pfad erzeugt auf dem CLI-Weg dieselbe Ablehnung wie über die
// API: beide kommen aus derselben Vertrauensgrenze, und der Bericht des
// Subkommandos nennt angefragten Pfad und Grund im Wortlaut der Fachlogik.
func TestSubkommandoNenntAbgelehntePfadeWieDieFachlogik(t *testing.T) {
	root := fixtureProject(t, "dreikontexte")
	outside := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(outside); err == nil {
		outside = resolved
	}
	foreign := filepath.Join(outside, "values.yaml")
	if err := os.WriteFile(foreign, []byte("image: nginx:1.27\n"), 0o644); err != nil {
		t.Fatalf("schreiben: %v", err)
	}
	options := fixtureOptions(root)
	if err := os.MkdirAll(filepath.Dir(options.SourcesFile), 0o755); err != nil {
		t.Fatalf("anlegen: %v", err)
	}
	if err := os.WriteFile(options.SourcesFile,
		[]byte("schema_version: 1\nsources:\n  - path: "+foreign+"\n    kind: helm\n    env: deployment\n"), 0o644); err != nil {
		t.Fatalf("schreiben: %v", err)
	}

	result, err := inventory.Collect(options)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(result.Rejections) != 1 {
		t.Fatalf("Ablehnungen = %+v", result.Rejections)
	}

	var out strings.Builder
	_, outcome, err := inventory.Run(options)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	printInventory(&out, options, result, outcome)
	if !strings.Contains(out.String(), describeRejection(result.Rejections[0])) {
		t.Errorf("der Bericht nennt die Ablehnung nicht im Wortlaut:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "Abgelehnte Quellen:           1") {
		t.Errorf("der Bericht zählt die Ablehnung nicht:\n%s", out.String())
	}
}
