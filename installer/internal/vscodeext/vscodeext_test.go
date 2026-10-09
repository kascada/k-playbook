package vscodeext

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

// quellDir ist die Quelle der Erweiterung, relativ zu diesem Paket.
const quellDir = "../../vscode"

// zipMit baut eine VSIX im Speicher mit den gegebenen Einträgen.
func zipMit(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range entries {
		file, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestVersionAusVSIX(t *testing.T) {
	data := zipMit(t, map[string]string{"extension/package.json": `{"name":"x","version":"0.1.0"}`})
	version, err := Version(data)
	if err != nil || version != "0.1.0" {
		t.Fatalf("Version = %q, %v", version, err)
	}
}

func TestVersionUnbrauchbareVSIX(t *testing.T) {
	fälle := map[string][]byte{
		"kein Zip":               []byte("kein Zip"),
		"ohne package.json":      zipMit(t, map[string]string{"extension/extension.js": "//"}),
		"kaputtes JSON":          zipMit(t, map[string]string{"extension/package.json": "{"}),
		"package.json ohne Zahl": zipMit(t, map[string]string{"extension/package.json": `{"name":"x"}`}),
	}
	for name, data := range fälle {
		if _, err := Version(data); err == nil {
			t.Errorf("%s: Version ohne Fehler", name)
		}
	}
}

// fromFS liest nur eine nicht leere Datei am vereinbarten Ort. Eine leere
// Datei zählt als fehlend: so sieht ein Binary aus, das ohne Erweiterung
// gebaut wurde, und eine leere Datei an die CLI zu geben wäre schlechter als
// eine klare Meldung.
func TestFromFS(t *testing.T) {
	if _, ok := fromFS(fstest.MapFS{}); ok {
		t.Error("leeres FS gilt als vorhanden")
	}
	if _, ok := fromFS(fstest.MapFS{vsixPath: &fstest.MapFile{Data: nil}}); ok {
		t.Error("leere Datei gilt als vorhanden")
	}
	data, ok := fromFS(fstest.MapFS{vsixPath: &fstest.MapFile{Data: []byte("Inhalt")}})
	if !ok || string(data) != "Inhalt" {
		t.Errorf("fromFS = %q, %v", data, ok)
	}
}

// Die eingebettete VSIX ist die eingecheckte; ihre Version ist die aus
// installer/vscode/package.json.
func TestEingebetteteVersionIstDieDerQuelle(t *testing.T) {
	version, ok, err := EmbeddedVersion()
	if !ok {
		t.Fatal("keine eingebettete VSIX — make vscode-vsix fehlt")
	}
	if err != nil {
		t.Fatalf("EmbeddedVersion: %v", err)
	}
	if version != quellVersion(t) {
		t.Errorf("eingebettet %q, Quelle %q — make vscode-vsix nachziehen", version, quellVersion(t))
	}
}

func quellVersion(t *testing.T) string {
	t.Helper()
	var manifest struct {
		Version string `json:"version"`
	}
	data, err := os.ReadFile(filepath.Join(quellDir, "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	return manifest.Version
}

// TestVSIXPasstZurQuelle ist der Wächter über die eingecheckte VSIX: sie wird
// nicht bei jedem Build erzeugt, also könnte sie zur Quelle unter
// installer/vscode/ hinterherhängen, ohne dass es auffällt. Dieser Test macht
// `make test` rot, wenn `make vscode-vsix` nach einer Änderung ausblieb.
//
// Verglichen wird in beide Richtungen: jeder Eintrag der VSIX gegen seine
// Quelldatei und jede mitzuliefernde Quelldatei gegen die VSIX. Nur
// package.json wird inhaltlich verglichen — vsce darf sie umformatieren —,
// alles andere byteweise.
func TestVSIXPasstZurQuelle(t *testing.T) {
	data, ok := VSIX()
	if !ok {
		t.Fatal("keine eingebettete VSIX — make vscode-vsix fehlt")
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("VSIX lesen: %v", err)
	}

	gesehen := map[string]bool{}
	for _, entry := range reader.File {
		if entry.FileInfo().IsDir() {
			continue
		}
		rel, inhalt := strings.CutPrefix(entry.Name, "extension/")
		if !inhalt {
			// extension.vsixmanifest und [Content_Types].xml erzeugt vsce.
			continue
		}
		quelle := quellDateiZu(rel)
		gesehen[quelle] = true

		gepackt := leseEintrag(t, entry)
		original, err := os.ReadFile(filepath.Join(quellDir, quelle))
		if err != nil {
			t.Errorf("%s steckt in der VSIX, fehlt aber in der Quelle: %v", entry.Name, err)
			continue
		}
		if rel == "package.json" {
			vergleicheJSON(t, entry.Name, gepackt, original)
			continue
		}
		if !bytes.Equal(gepackt, original) {
			t.Errorf("%s weicht von %s/%s ab — make vscode-vsix nachziehen", entry.Name, quellDir, quelle)
		}
	}

	for _, quelle := range mitzuliefern(t) {
		if !gesehen[quelle] {
			t.Errorf("%s/%s fehlt in der VSIX — make vscode-vsix nachziehen", quellDir, quelle)
		}
	}
}

// quellDateiZu bildet einen VSIX-Eintrag auf seine Quelldatei ab. vsce
// schreibt die Readme klein; alles andere behält seinen Namen.
func quellDateiZu(rel string) string {
	if strings.EqualFold(rel, "readme.md") {
		return "README.md"
	}
	return rel
}

// mitzuliefern sind die Quelldateien, die in die VSIX gehören: alles unter
// installer/vscode/ außer dem, was .vscodeignore ausnimmt.
func mitzuliefern(t *testing.T) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(quellDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(quellDir, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			if rel == "test" || rel == "node_modules" {
				return fs.SkipDir
			}
			return nil
		}
		switch {
		case strings.HasPrefix(rel, "."), strings.HasSuffix(rel, ".vsix"):
			return nil
		}
		out = append(out, rel)
		return nil
	})
	if err != nil {
		t.Fatalf("%s durchgehen: %v", quellDir, err)
	}
	return out
}

func leseEintrag(t *testing.T, entry *zip.File) []byte {
	t.Helper()
	file, err := entry.Open()
	if err != nil {
		t.Fatalf("%s öffnen: %v", entry.Name, err)
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("%s lesen: %v", entry.Name, err)
	}
	return data
}

func vergleicheJSON(t *testing.T, name string, gepackt []byte, original []byte) {
	t.Helper()
	var a, b any
	if err := json.Unmarshal(gepackt, &a); err != nil {
		t.Fatalf("%s aus der VSIX: %v", name, err)
	}
	if err := json.Unmarshal(original, &b); err != nil {
		t.Fatalf("package.json der Quelle: %v", err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Errorf("%s weicht inhaltlich von %s/package.json ab — make vscode-vsix nachziehen", name, quellDir)
	}
}
