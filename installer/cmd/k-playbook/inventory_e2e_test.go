package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/inventory"
	"github.com/kascada/k-playbook/installer/internal/kephalaion/kephtest"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// Der Subkommando-Weg über die Fixture des Sammlers: runInventory sammelt nur
// die Pfade und den Zugang zusammen und ruft inventory.Run — das Dokument in
// der Collection muss deshalb byteweise das sein, was die Fachlogik für
// dieselben Optionen rendert. Der API-Weg wird in internal/webui gegen
// dieselbe Fachlogik geprüft; zusammen belegen beide Tests, dass Subkommando
// und API dieselbe Ausgabe erzeugen. Die Collection liegt im Speicher.
func fixtureProject(t *testing.T, name string) (string, *kephtest.Store) {
	t.Helper()
	store := kephtest.New()
	original := project.OpenInventoryStore
	project.OpenInventoryStore = func(string) (inventory.Store, error) { return store, nil }
	t.Cleanup(func() { project.OpenInventoryStore = original })

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
	// Die README-Vorlage von versions/ im Lieferumfang des Projekts — dort
	// liest der abgeschirmte Store (defaultstest) ihn. Die Installation
	// k-playbook/ gehört nie zum Inventar.
	putVersionsTemplate(t, root)

	before, err := os.Getwd()
	if err != nil {
		t.Fatalf("Arbeitsverzeichnis: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("nach %s wechseln: %v", root, err)
	}
	t.Cleanup(func() { os.Chdir(before) })
	return root, store
}

// putVersionsTemplate legt die Vorlage k-playbook:/docs/generated/versions/README.md
// aus dem Repo in den Lieferumfang des Projekts.
func putVersionsTemplate(t *testing.T, root string) {
	t.Helper()
	rel := filepath.Join("docs", "generated", "versions", "README.md")
	data, err := os.ReadFile(filepath.Join(repoDefaultsDir(t), rel))
	if err != nil {
		t.Fatalf("Vorlage lesen: %v", err)
	}
	target := filepath.Join(project.PlaybookDir(root), "defaults", rel)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func fixtureOptions(root string) inventory.Options {
	localDir := project.LocalDir(root)
	return inventory.Options{
		ProjectDir:  root,
		SourcesFile: filepath.Join(localDir, project.VersionSourcesFileName),
	}
}

func TestSubkommandoUeberDieFixtureErgibtDieDateiDerFachlogik(t *testing.T) {
	root, store := fixtureProject(t, "vollstaendig")

	out := captureStdout(t, func() {
		if err := runInventory(nil); err != nil {
			t.Fatalf("runInventory: %v", err)
		}
	})
	if !strings.Contains(out, "Geschrieben: "+inventory.DocRef) {
		t.Errorf("der Bericht nennt den keph:-Verweis nicht:\n%s", out)
	}
	// Nach dem Inventar die README des Verzeichnisses aus ihrer Vorlage.
	if !strings.Contains(out, "README: created keph:/docs/generated/versions/README.md") {
		t.Errorf("der Bericht nennt die README nicht:\n%s", out)
	}
	if readme, ok := store.Content("docs/generated/versions/README.md"); !ok || !strings.Contains(readme, "title: \"Version inventory\"") {
		t.Errorf("README von versions/: %q", readme)
	}
	options := fixtureOptions(root)
	written, ok := store.Content(inventory.DocName)
	if !ok {
		t.Fatalf("%s liegt nicht in der Collection: %v", inventory.DocName, store.Names())
	}
	status := inventory.ReadStatus(context.Background(), store)
	if status.Problem != "" || status.GeneratedBy != inventory.GeneratedBy {
		t.Fatalf("Status = %+v", status)
	}

	result, err := inventory.Collect(options)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if expected := inventory.Render(result, status.GeneratedAt); expected != written {
		t.Error("das Dokument des Subkommandos ist nicht das Rendering der Fachlogik")
	}
	if _, err := os.Stat(filepath.Join(project.LocalDir(root), "docs")); !os.IsNotExist(err) {
		t.Error("das Subkommando hat auf der Platte geschrieben")
	}
	if status.Deviations != len(result.Deviations) || status.SourcesRead != len(result.Sources) {
		t.Errorf("Status %+v, Fachlogik %d Abweichungen, %d Quellen", status, len(result.Deviations), len(result.Sources))
	}
}

// Ein abgelehnter Pfad erzeugt auf dem CLI-Weg dieselbe Ablehnung wie über die
// API: beide kommen aus derselben Vertrauensgrenze. Geprüft wird die echte
// Ausgabe von runInventory gegen Pfad und Grund als Literale — nicht gegen die
// Funktion, die den Bericht selbst formatiert.
func TestSubkommandoNenntAbgelehntePfadeWieDieFachlogik(t *testing.T) {
	root, _ := fixtureProject(t, "dreikontexte")
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

	out := captureStdout(t, func() {
		if err := runInventory(nil); err != nil {
			t.Fatalf("runInventory: %v", err)
		}
	})
	if !strings.Contains(out, foreign+": liegt außerhalb der erlaubten Wurzeln (") {
		t.Errorf("der Bericht nennt Pfad und Grund der Ablehnung nicht:\n%s", out)
	}
	if !strings.Contains(out, "Abgelehnte Quellen:           1") {
		t.Errorf("der Bericht zählt die Ablehnung nicht:\n%s", out)
	}
}

// captureStdout fängt ab, was fn nach os.Stdout schreibt — runInventory kennt
// keinen anderen Ausgang, und genau der soll geprüft werden.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe: %v", err)
	}
	before := os.Stdout
	os.Stdout = writer
	defer func() { os.Stdout = before }()

	done := make(chan string)
	go func() {
		var buffer strings.Builder
		_, _ = io.Copy(&buffer, reader)
		done <- buffer.String()
	}()
	fn()
	writer.Close()
	return <-done
}

// Ohne bereite Anbindung bricht das Subkommando mit kephalaion_unavailable
// und dem Grund ab, bevor es eine Quelle liest; auf der Platte entsteht
// nichts.
func TestSubkommandoOhneKephalaionBrichtAb(t *testing.T) {
	root, _ := fixtureProject(t, "vollstaendig")
	project.OpenInventoryStore = func(string) (inventory.Store, error) {
		_, err := project.OpenUserDocs(root)
		return nil, err
	}

	err := runInventory(nil)
	if err == nil {
		t.Fatal("ohne Kephalaion muss das Subkommando abbrechen")
	}
	if !strings.Contains(err.Error(), "(kephalaion_unavailable)") || !strings.Contains(err.Error(), "nicht bereit") {
		t.Errorf("der Fehler nennt Code und Grund nicht: %v", err)
	}
	if _, err := os.Stat(filepath.Join(project.LocalDir(root), "docs")); !os.IsNotExist(err) {
		t.Error("einen Rückfall auf die Platte gibt es nicht")
	}
}

// Fehlt die Vorlage auf dem Hub (Versatz Binary/Hub), scheitert nur der
// README-Schritt: Das Inventar ist geschrieben, der Bericht nennt die README
// mit template_missing, und der Lauf endet danach mit einem Fehler. Eine
// leere README entsteht nicht.
func TestSubkommandoOhneVorlageSchreibtDasInventar(t *testing.T) {
	root, store := fixtureProject(t, "vollstaendig")
	if err := os.RemoveAll(filepath.Join(project.PlaybookDir(root), "defaults", "docs")); err != nil {
		t.Fatal(err)
	}

	var runErr error
	out := captureStdout(t, func() { runErr = runInventory(nil) })
	if runErr == nil || !strings.Contains(runErr.Error(), "Inventar geschrieben") || !strings.Contains(runErr.Error(), "(template_missing)") {
		t.Fatalf("Fehler: %v", runErr)
	}
	if !strings.Contains(out, "Geschrieben: "+inventory.DocRef) || !strings.Contains(out, "README: failed keph:/docs/generated/versions/README.md — template_missing") {
		t.Errorf("Bericht:\n%s", out)
	}
	if _, ok := store.Content(inventory.DocName); !ok {
		t.Error("Inventar nicht geschrieben")
	}
	if _, ok := store.Content("docs/generated/versions/README.md"); ok {
		t.Error("README trotz fehlender Vorlage angelegt")
	}
}
