package project

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
)

// docTree legt zwei Ebenen als Beispielbaum an: dist und local, je eine Karte
// von Pfad zu Inhalt. Ein Pfad, der auf „/" endet, wird als leeres
// Verzeichnis angelegt.
func docTree(t *testing.T, dist map[string]string, local map[string]string) (DocLayers, string, string) {
	t.Helper()

	root := t.TempDir()
	distDir := filepath.Join(root, "k-playbook")
	localDir := filepath.Join(root, "k-playbook-local")
	writeDocTree(t, distDir, dist)
	writeDocTree(t, localDir, local)
	return DocLayers{Dist: NewDiskDocSource(distDir), Local: NewDiskDocSource(localDir)}, distDir, localDir
}

func writeDocTree(t *testing.T, dir string, files map[string]string) {
	t.Helper()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("%s anlegen: %v", dir, err)
	}
	for rel, content := range files {
		target := filepath.Join(dir, filepath.FromSlash(rel))
		if strings.HasSuffix(rel, "/") {
			if err := os.MkdirAll(target, 0o755); err != nil {
				t.Fatalf("%s anlegen: %v", target, err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatalf("%s anlegen: %v", filepath.Dir(target), err)
		}
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			t.Fatalf("%s schreiben: %v", target, err)
		}
	}
}

func mustResolveDoc(t *testing.T, layers DocLayers, ref string) DocResult {
	t.Helper()

	result, err := ResolveDocRef(layers, ref)
	if err != nil {
		t.Fatalf("%s: %v", ref, err)
	}
	return result
}

func partNames(parts []DocPart) []string {
	names := make([]string, 0, len(parts))
	for _, part := range parts {
		names = append(names, part.Name)
	}
	return names
}

func expectDocError(t *testing.T, layers DocLayers, ref string, code string) error {
	t.Helper()

	_, err := ResolveDocRef(layers, ref)
	if err == nil {
		t.Fatalf("%s: Fehler erwartet, keiner gemeldet", ref)
	}
	if got := DocRefErrorCode(err); got != code {
		t.Fatalf("%s: Code %q, erwartet %q (%v)", ref, got, code, err)
	}
	return err
}

func TestParseDocRefZerlegtDieForm(t *testing.T) {
	cases := []struct {
		raw    string
		scheme string
		path   string
		dir    bool
	}{
		{"k-playbook:/instructions/always/", "k-playbook", "instructions/always", true},
		{"  k-playbook:/instructions/on-demand/x.md ", "k-playbook", "instructions/on-demand/x.md", false},
		{"project:/scripts/123.sh", "project", "scripts/123.sh", false},
		{"keph:/manual/abc.md", "keph", "manual/abc.md", false},
		{"k-playbook:/", "k-playbook", "", true},
	}
	for _, c := range cases {
		ref, err := ParseDocRef(c.raw)
		if err != nil {
			t.Fatalf("%q: %v", c.raw, err)
		}
		if ref.Scheme != c.scheme || ref.Path != c.path || ref.Dir != c.dir {
			t.Errorf("%q zerlegt zu %+v", c.raw, ref)
		}
	}

	for _, raw := range []string{
		"", "instructions/always/", "local:/x.md", "k-playbook:instructions/x.md",
		"k-playbook:/instructions//x.md", "k-playbook:/instructions/../rules/x.md",
		"k-playbook:/instructions/./x.md", "k-playbook:/instructions/.versteckt.md",
		`k-playbook:/instructions\x.md`,
	} {
		_, err := ParseDocRef(raw)
		if DocRefErrorCode(err) != DocCodeInvalidInput {
			t.Errorf("%q: erwartet invalid_input, bekommen %v", raw, err)
		}
	}
}

func TestDocRefNurMitgeliefert(t *testing.T) {
	layers, _, _ := docTree(t, map[string]string{
		"instructions/always/010-a.md":    "## A\n",
		"instructions/always/100-b.md":    "## B\n",
		"instructions/on-demand/thema.md": "---\ntitle: Thema\ndescription: D\n---\n\n## Thema\n",
	}, nil)

	dir := mustResolveDoc(t, layers, "k-playbook:/instructions/always/")
	if dir.Content != "## A\n\n## B\n" {
		t.Errorf("Zusammenbau: %q", dir.Content)
	}
	for _, part := range dir.Parts {
		if part.Origin != "dist" || part.Disabled {
			t.Errorf("Teil %+v", part)
		}
	}

	file := mustResolveDoc(t, layers, "k-playbook:/instructions/on-demand/thema.md")
	if file.Content != "---\ntitle: Thema\ndescription: D\n---\n\n## Thema\n" {
		t.Errorf("Einzeldatei nicht unverändert mit Frontmatter: %q", file.Content)
	}
	if file.Directory || file.Disabled || len(file.Parts) != 1 || file.Parts[0].Origin != "dist" {
		t.Errorf("Einzeldatei: %+v", file)
	}
}

func TestDocRefNurLokal(t *testing.T) {
	layers, _, _ := docTree(t, nil, map[string]string{
		"instructions/always/1000-eigen.md": "## Eigen\n\nText.\n",
		"instructions/on-demand/eigen.md":   "## Eigen auf Abruf\n\nText.\n",
	})

	dir := mustResolveDoc(t, layers, "k-playbook:/instructions/always/")
	if dir.Content != "## Eigen\n\nText.\n" || len(dir.Parts) != 1 || dir.Parts[0].Origin != "local" {
		t.Errorf("Zusammenbau nur lokal: %+v", dir)
	}
	file := mustResolveDoc(t, layers, "k-playbook:/instructions/on-demand/eigen.md")
	if file.Content != "## Eigen auf Abruf\n\nText.\n" || file.Parts[0].Origin != "local" {
		t.Errorf("Einzeldatei nur lokal: %+v", file)
	}
}

func TestDocRefLokalErsetzt(t *testing.T) {
	layers, _, _ := docTree(t, map[string]string{
		"instructions/always/100-b.md":    "## B mitgeliefert\n",
		"instructions/on-demand/thema.md": "## Mitgeliefert\n",
	}, map[string]string{
		"instructions/always/100-b.md":    "## B lokal\n\nText.\n",
		"instructions/on-demand/thema.md": "## Lokal\n\nText.\n",
	})

	dir := mustResolveDoc(t, layers, "k-playbook:/instructions/always/")
	if dir.Content != "## B lokal\n\nText.\n" {
		t.Errorf("Ersetzen im Zusammenbau: %q", dir.Content)
	}
	if !reflect.DeepEqual(dir.Parts, []DocPart{{Name: "100-b.md", Origin: "override"}}) {
		t.Errorf("Teile: %+v", dir.Parts)
	}
	file := mustResolveDoc(t, layers, "k-playbook:/instructions/on-demand/thema.md")
	if file.Content != "## Lokal\n\nText.\n" || file.Parts[0].Origin != "override" {
		t.Errorf("Ersetzen der Einzeldatei: %+v", file)
	}
}

func TestDocRefLeerLokalSchaltetAb(t *testing.T) {
	layers, _, _ := docTree(t, map[string]string{
		"instructions/always/010-a.md":    "## A\n",
		"instructions/always/100-b.md":    "## B\n",
		"instructions/on-demand/thema.md": "## Thema\n",
	}, map[string]string{
		"instructions/always/100-b.md":    "# Abgeschaltet: hier nicht gebraucht.\n",
		"instructions/on-demand/thema.md": "\n# Abgeschaltet\n\n",
	})

	dir := mustResolveDoc(t, layers, "k-playbook:/instructions/always/")
	if dir.Content != "## A\n" {
		t.Errorf("abgeschalteter Teil im Inhalt: %q", dir.Content)
	}
	want := []DocPart{{Name: "010-a.md", Origin: "dist"}, {Name: "100-b.md", Origin: "override", Disabled: true}}
	if !reflect.DeepEqual(dir.Parts, want) {
		t.Errorf("Teile: %+v", dir.Parts)
	}

	// Abgeschaltete Einzeldatei: kein Fehler, leerer Inhalt, disabled mit Hinweis.
	file := mustResolveDoc(t, layers, "k-playbook:/instructions/on-demand/thema.md")
	if file.Content != "" || !file.Disabled || file.Hint == "" {
		t.Errorf("abgeschaltete Einzeldatei: %+v", file)
	}
	if len(file.Parts) != 1 || !file.Parts[0].Disabled || file.Parts[0].Origin != "override" {
		t.Errorf("Teil der abgeschalteten Einzeldatei: %+v", file.Parts)
	}
}

// Eine Datei nur aus Überschriften gilt wie bei isEmptyFile als leer und
// schaltet ab — die Folge steht in docs/verweisschema.md.
func TestDocRefNurUeberschriftenSchaltetAb(t *testing.T) {
	layers, _, _ := docTree(t, map[string]string{
		"instructions/always/100-b.md": "## B\n",
	}, map[string]string{
		"instructions/always/100-b.md": "## Nur eine Überschrift\n",
	})
	dir := mustResolveDoc(t, layers, "k-playbook:/instructions/always/")
	if dir.Content != "" || !dir.Parts[0].Disabled {
		t.Errorf("Datei nur aus Überschriften: %+v", dir)
	}
}

func TestDocRefOrdnungNumerischUeberBeideEbenen(t *testing.T) {
	layers, _, _ := docTree(t, map[string]string{
		"instructions/always/20-a.md":  "zwanzig\n",
		"instructions/always/100-b.md": "hundert\n",
		"instructions/always/100-a.md": "hundert-a\n",
	}, map[string]string{
		"instructions/always/050-dazwischen.md": "fünfzig\n",
		"instructions/always/1000-hinten.md":    "tausend\n",
	})

	dir := mustResolveDoc(t, layers, "k-playbook:/instructions/always/")
	wantNames := []string{"20-a.md", "050-dazwischen.md", "100-a.md", "100-b.md", "1000-hinten.md"}
	if got := partNames(dir.Parts); !reflect.DeepEqual(got, wantNames) {
		t.Errorf("Reihenfolge %v, erwartet %v", got, wantNames)
	}
	if dir.Content != "zwanzig\n\nfünfzig\n\nhundert-a\n\nhundert\n\ntausend\n" {
		t.Errorf("Inhalt: %q", dir.Content)
	}
}

func TestDocRefDateiOhneZifferWirdGemeldet(t *testing.T) {
	layers, _, _ := docTree(t, map[string]string{
		"instructions/always/010-a.md":  "## A\n",
		"instructions/always/notiz.md":  "## Notiz\n",
		"instructions/always/README.md": "# Liesmich\n\nText.\n",
		"instructions/always/sub/":      "",
		"instructions/always/sub/1.md":  "## Unter\n",
	}, map[string]string{
		"instructions/always/eigen.md": "## Eigen\n\nText.\n",
	})

	dir := mustResolveDoc(t, layers, "k-playbook:/instructions/always/")
	if dir.Content != "## A\n" {
		t.Errorf("Inhalt: %q", dir.Content)
	}
	if got := partNames(dir.Parts); !reflect.DeepEqual(got, []string{"010-a.md"}) {
		t.Errorf("Teile: %v", got)
	}
	if len(dir.Problems) != 2 {
		t.Fatalf("Probleme: %v", dir.Problems)
	}
	joined := strings.Join(dir.Problems, "\n")
	for _, name := range []string{"notiz.md", "eigen.md"} {
		if !strings.Contains(joined, name) {
			t.Errorf("%s fehlt in den Problemen: %v", name, dir.Problems)
		}
	}
	if strings.Contains(joined, "README") || strings.Contains(joined, "sub") {
		t.Errorf("README oder Unterverzeichnis gemeldet: %v", dir.Problems)
	}
}

func TestDocRefFrontmatterWirdBeimZusammenbauEntfernt(t *testing.T) {
	layers, _, _ := docTree(t, map[string]string{
		"instructions/always/010-a.md": "---\ntitle: A\n---\n\n## A\n",
		"instructions/always/020-b.md": "## B\n\n<!-- NACHZIEHEN: bleibt stehen -->\n",
	}, nil)

	dir := mustResolveDoc(t, layers, "k-playbook:/instructions/always/")
	if dir.Content != "## A\n\n## B\n\n<!-- NACHZIEHEN: bleibt stehen -->\n" {
		t.Errorf("Inhalt: %q", dir.Content)
	}
}

func TestDocRefVerzeichnisOhneTeileIstLeer(t *testing.T) {
	// Lauter abgeschaltete Teile.
	layers, _, _ := docTree(t, map[string]string{
		"instructions/always/010-a.md": "## A\n",
	}, map[string]string{
		"instructions/always/010-a.md": "",
	})
	dir := mustResolveDoc(t, layers, "k-playbook:/instructions/always/")
	if dir.Content != "" || dir.Disabled {
		t.Errorf("lauter abgeschaltete Teile: %+v", dir)
	}
	if !reflect.DeepEqual(dir.Parts, []DocPart{{Name: "010-a.md", Origin: "override", Disabled: true}}) {
		t.Errorf("Teile: %+v", dir.Parts)
	}

	// Keine Teile: das Verzeichnis gibt es, es enthält nur die README.
	layers, _, _ = docTree(t, map[string]string{
		"instructions/always/README.md": "# Liesmich\n\nText.\n",
	}, nil)
	dir = mustResolveDoc(t, layers, "k-playbook:/instructions/always/")
	if dir.Content != "" || len(dir.Parts) != 0 || dir.Parts == nil {
		t.Errorf("keine Teile: %+v", dir)
	}
}

func TestDocRefUnbekannterVerweisIstFehler(t *testing.T) {
	layers, distDir, _ := docTree(t, map[string]string{
		"instructions/always/010-a.md": "## A\n",
	}, nil)
	// Neben der Quelle liegt eine Datei gleichen Namens — sie darf nicht
	// gefunden werden: kein Rückfall auf die Platte außerhalb der Ebenen.
	writeDocTree(t, filepath.Dir(distDir), map[string]string{
		"instructions/on-demand/fehlt.md": "## Daneben\n",
	})

	err := expectDocError(t, layers, "k-playbook:/instructions/on-demand/fehlt.md", DocCodeNotFound)
	if !strings.Contains(err.Error(), "unbekannt") {
		t.Errorf("Meldung: %v", err)
	}
	// Ein unbekanntes Verzeichnis steht nicht in der Liste: der Dateiname
	// fehlt, entschieden ohne zu lesen.
	expectDocError(t, layers, "k-playbook:/instructions/gibt-es-nicht/", DocCodeInvalidInput)
	// Verzeichnis ohne „/" am Ende.
	expectDocError(t, layers, "k-playbook:/instructions/always", DocCodeInvalidInput)
}

// Ein Verweis, der eine Datei wie ein Verzeichnis behandelt, ist als
// Einzeldatei (…/a.md/x.md) unbekannt, in jeder Ebene — das System meldet dafür
// ENOTDIR, nicht „gibt es nicht“. Als Verzeichnis (…/a.md/) steht er nicht in
// der Liste der zusammengebauten Verzeichnisse: der Dateiname fehlt.
func TestDocRefDateiAlsVerzeichnis(t *testing.T) {
	layers, _, _ := docTree(t, map[string]string{
		"instructions/always/010-a.md":    "## A\n",
		"instructions/always/100-b.md":    "## B mitgeliefert\n",
		"instructions/on-demand/thema.md": "## Thema\n",
	}, map[string]string{
		"instructions/always/100-b.md":     "## B lokal\n",
		"instructions/always/1000-eins.md": "## Eigen\n",
	})

	for _, file := range []string{
		"always/010-a.md",     // nur mitgeliefert
		"always/1000-eins.md", // nur lokal
		"always/100-b.md",     // in beiden Ebenen
		"on-demand/thema.md",  // lokal fehlt schon das Verzeichnis
	} {
		ref := "k-playbook:/instructions/" + file
		err := expectDocError(t, layers, ref+"/x.md", DocCodeNotFound)
		if strings.Contains(err.Error(), "not a directory") {
			t.Errorf("%s/x.md: Systemfehler statt Meldung: %v", ref, err)
		}
		err = expectDocError(t, layers, ref+"/", DocCodeInvalidInput)
		if !strings.Contains(err.Error(), "Dateiname fehlt") || !strings.Contains(err.Error(), "ohne „/“ am Ende: "+ref) {
			t.Errorf("%s/: Meldung: %v", ref, err)
		}
		// Auch tiefer unter der Datei.
		expectDocError(t, layers, ref+"/tiefer/x.md", DocCodeNotFound)
		err = expectDocError(t, layers, ref+"/tiefer/", DocCodeInvalidInput)
		if !strings.Contains(err.Error(), "Dateiname fehlt") || !strings.Contains(err.Error(), "gemeint ist etwa "+ref) {
			t.Errorf("%s/tiefer/: Meldung: %v", ref, err)
		}
	}
}

// Nur Verzeichnisse aus docAssembledDirs werden zusammengebaut. Jeder andere
// Verzeichnis-Verweis ist unvollständig — auch wenn es das Verzeichnis lokal
// und mitgeliefert mit Dateien gibt. Einzeldateien bleiben überall lesbar.
func TestDocRefNurListeWirdZusammengebaut(t *testing.T) {
	layers, _, _ := docTree(t, map[string]string{
		"instructions/always/010-a.md":    "## A\n",
		"instructions/on-demand/thema.md": "## Thema\n",
		"instructions/on-demand/010-x.md": "## X\n",
		"instructions/oben.md":            "## Oben\n",
	}, map[string]string{
		"instructions/on-demand/eigen.md":   "## Eigen\n\nText.\n",
		"instructions/on-demand/1000-y.md":  "## Y\n\nText.\n",
		"instructions/always/1000-eigen.md": "## Eigen immer\n\nText.\n",
	})

	for ref, example := range map[string]string{
		"k-playbook:/instructions/on-demand/": "k-playbook:/instructions/on-demand/<name>.md",
		"k-playbook:/instructions/":           "k-playbook:/instructions/on-demand/<name>.md",
		"k-playbook:/instructions/always/x/":  "k-playbook:/instructions/always/x/<name>.md",
	} {
		err := expectDocError(t, layers, ref, DocCodeInvalidInput)
		if !strings.Contains(err.Error(), "Dateiname fehlt") || !strings.Contains(err.Error(), example) {
			t.Errorf("%s: Meldung ohne „Dateiname fehlt“ oder Beispiel %s: %v", ref, example, err)
		}
		if !strings.Contains(err.Error(), "k-playbook:/instructions/always/") {
			t.Errorf("%s: Meldung nennt das zusammengebaute Verzeichnis nicht: %v", ref, err)
		}
	}

	dir := mustResolveDoc(t, layers, "k-playbook:/instructions/always/")
	if dir.Content != "## A\n\n## Eigen immer\n\nText.\n" {
		t.Errorf("always/ baut nicht mehr zusammen: %q", dir.Content)
	}

	for ref, content := range map[string]string{
		"k-playbook:/instructions/on-demand/thema.md":   "## Thema\n",
		"k-playbook:/instructions/on-demand/eigen.md":   "## Eigen\n\nText.\n",
		"k-playbook:/instructions/always/010-a.md":      "## A\n",
		"k-playbook:/instructions/always/1000-eigen.md": "## Eigen immer\n\nText.\n",
		"k-playbook:/instructions/oben.md":              "## Oben\n",
	} {
		file := mustResolveDoc(t, layers, ref)
		if file.Content != content || file.Directory {
			t.Errorf("%s: %+v", ref, file)
		}
	}

	// Außerhalb der Sorten bleibt es unsupported, auch mit „/“ am Ende; eine
	// Sorte ohne Dateinamen ist unvollständig.
	expectDocError(t, layers, "k-playbook:/checks/", DocCodeUnsupported)
	expectDocError(t, layers, "k-playbook:/rules/", DocCodeInvalidInput)
}

// Die Prüfung der Liste liest nichts: auch eine Quelle, die bei jedem Zugriff
// scheitert, ergibt invalid_input statt eines Umgebungsfehlers.
func TestDocRefListePrueftOhneZuLesen(t *testing.T) {
	layers := DocLayers{Dist: failingDocSource{}, Local: failingDocSource{}}
	expectDocError(t, layers, "k-playbook:/instructions/on-demand/", DocCodeInvalidInput)
}

// project: liest eine Datei des Projekts ab seiner Wurzel: eine Ebene, keine
// Überlagerung, kein Zusammenbau, eine leere Datei ist leerer Inhalt.
func TestDocRefProjekt(t *testing.T) {
	layers, _, _ := docTree(t, map[string]string{
		"instructions/on-demand/thema.md": "## Thema\n",
	}, nil)
	root := t.TempDir()
	writeDocTree(t, root, map[string]string{
		"scripts/123.sh":                     "#!/bin/sh\necho hallo\n",
		"k-playbook-local/docs/README.md":    "# Doku\n",
		"k-playbook-local/docs/leer.md":      "",
		"k-playbook-local/docs/kommentar.md": "# nur Überschrift\n",
		"ordner/":                            "",
	})
	layers.Project = NewDiskDocSource(root)

	file := mustResolveDoc(t, layers, "project:/scripts/123.sh")
	if file.Content != "#!/bin/sh\necho hallo\n" || file.Directory || file.Disabled || file.Ref != "project:/scripts/123.sh" {
		t.Errorf("Projektdatei: %+v", file)
	}
	if !reflect.DeepEqual(file.Parts, []DocPart{{Name: "123.sh", Origin: DocOriginProject}}) {
		t.Errorf("parts: %+v", file.Parts)
	}
	if doc := mustResolveDoc(t, layers, "project:/k-playbook-local/docs/README.md"); doc.Content != "# Doku\n" {
		t.Errorf("Datei unter k-playbook-local/: %+v", doc)
	}
	// Leer heißt leer, nicht abgeschaltet: es gibt keine zweite Ebene, die
	// eine leere Datei abschalten könnte.
	for _, ref := range []string{"project:/k-playbook-local/docs/leer.md", "project:/k-playbook-local/docs/kommentar.md"} {
		doc := mustResolveDoc(t, layers, ref)
		if doc.Disabled || doc.Hint != "" || len(doc.Parts) != 1 || doc.Parts[0].Disabled {
			t.Errorf("%s: als abgeschaltet gemeldet: %+v", ref, doc)
		}
	}
	if doc := mustResolveDoc(t, layers, "project:/k-playbook-local/docs/kommentar.md"); doc.Content != "# nur Überschrift\n" {
		t.Errorf("Inhalt der Kommentar-Datei: %q", doc.Content)
	}

	expectDocError(t, layers, "project:/scripts/fehlt.sh", DocCodeNotFound)
	expectDocError(t, layers, "project:/scripts/123.sh/x.md", DocCodeNotFound)
	// Ein Verzeichnis: ohne „/" am Ende ein Eingabefehler, mit „/" fehlt der
	// Dateiname — project: baut nichts zusammen.
	expectDocError(t, layers, "project:/ordner", DocCodeInvalidInput)
	for _, ref := range []string{"project:/ordner/", "project:/scripts/", "project:/"} {
		err := expectDocError(t, layers, ref, DocCodeInvalidInput)
		if !strings.Contains(err.Error(), "Dateiname fehlt") {
			t.Errorf("%s: %v", ref, err)
		}
	}
	// Aus der Wurzel führt kein Pfad: .. ist schon in der Form ungültig,
	// und die Quelle liest über fs.FS.
	expectDocError(t, layers, "project:/../geheim.md", DocCodeInvalidInput)
	expectDocError(t, layers, "project:/.git/config", DocCodeInvalidInput)
	// Ohne Projektquelle gibt es keine Projektdatei.
	layers.Project = nil
	expectDocError(t, layers, "project:/scripts/123.sh", DocCodeNotFound)
}

// fakeKephStore ist ein Kephalaion im Speicher für keph:-Verweise: read
// liefert ein Dokument, ein Verzeichnis (ein Dokument darunter) oder none.
type fakeKephStore struct {
	docs map[string]string
	// fail lässt jeden Aufruf mit diesem Fehler scheitern.
	fail  error
	reads []string
}

func (s *fakeKephStore) Read(_ context.Context, name string) (kephalaion.ReadResult, error) {
	s.reads = append(s.reads, name)
	if s.fail != nil {
		return kephalaion.ReadResult{}, s.fail
	}
	out := kephalaion.ReadResult{Kind: kephalaion.ReadKindNone, Address: "vm:k-playbook-dev", Name: name}
	if content, ok := s.docs[name]; ok {
		out.Kind, out.Content = kephalaion.ReadKindDocument, &content
		return out, nil
	}
	for other := range s.docs {
		if strings.HasPrefix(other, name+"/") {
			out.Kind = kephalaion.ReadKindDirectory
			break
		}
	}
	return out, nil
}

// kephLayers sind Ebenen mit einem gefälschten Zugang zur Collection.
func kephLayers(t *testing.T, store *fakeKephStore, user string) DocLayers {
	t.Helper()
	layers, _, _ := docTree(t, map[string]string{"instructions/always/010-a.md": "## A\n"}, nil)
	layers.Keph = func() (KephDocs, error) { return KephDocs{Store: store, User: user}, nil }
	return layers
}

// keph: liest ein Dokument der Collection über den Node: eine Ebene, ~ steht
// für den eigenen User, none ist not_found, ein Verzeichnis ein Eingabefehler.
func TestDocRefKeph(t *testing.T) {
	store := &fakeKephStore{docs: map[string]string{
		"manual/abc.md":                "---\ntitle: ABC\n---\n\n# ABC\n",
		"user/kamran/tasks/100-a.md":   "# A\n",
		"user/kamran/tasks/leer.md":    "",
		"user/anna/tasks/104-fremd.md": "# fremd\n",
	}}
	layers := kephLayers(t, store, "kamran")

	doc := mustResolveDoc(t, layers, "keph:/manual/abc.md")
	if doc.Content != "---\ntitle: ABC\n---\n\n# ABC\n" || doc.Directory || doc.Disabled || doc.Ref != "keph:/manual/abc.md" {
		t.Errorf("Dokument: %+v", doc)
	}
	if !reflect.DeepEqual(doc.Parts, []DocPart{{Name: "abc.md", Origin: DocOriginKeph}}) {
		t.Errorf("parts: %+v", doc.Parts)
	}
	// ~ als ganzer Pfadteil wird ersetzt; der Verweis in der Antwort trägt
	// den echten Namen.
	own := mustResolveDoc(t, layers, "keph:/user/~/tasks/100-a.md")
	if own.Content != "# A\n" || own.Ref != "keph:/user/kamran/tasks/100-a.md" {
		t.Errorf("~: %+v", own)
	}
	if store.reads[len(store.reads)-1] != "user/kamran/tasks/100-a.md" {
		t.Errorf("read mit %q statt mit dem echten Namen", store.reads[len(store.reads)-1])
	}
	// Fremde Dokumente sind lesbar — Lesen ist kein Verschieben.
	if fremd := mustResolveDoc(t, layers, "keph:/user/anna/tasks/104-fremd.md"); fremd.Content != "# fremd\n" {
		t.Errorf("fremdes Dokument: %+v", fremd)
	}
	// Leer heißt leer, nicht abgeschaltet.
	if leer := mustResolveDoc(t, layers, "keph:/user/~/tasks/leer.md"); leer.Disabled || leer.Content != "" {
		t.Errorf("leeres Dokument: %+v", leer)
	}
	// ~ nur als ganzer Pfadteil; ~kamran bleibt stehen und ist unbekannt.
	expectDocError(t, layers, "keph:/user/~kamran/tasks/100-a.md", DocCodeNotFound)
	expectDocError(t, layers, "keph:/manual/fehlt.md", DocCodeNotFound)
	// Ein Verzeichnis: ohne „/" ein Eingabefehler (keph: baut nichts
	// zusammen), mit „/" fehlt der Dateiname.
	expectDocError(t, layers, "keph:/user/kamran/tasks", DocCodeInvalidInput)
	err := expectDocError(t, layers, "keph:/user/~/tasks/", DocCodeInvalidInput)
	if !strings.Contains(err.Error(), "Dateiname fehlt") {
		t.Errorf("keph: mit / am Ende: %v", err)
	}
	expectDocError(t, layers, "keph:/", DocCodeInvalidInput)
}

func TestDocRefKephNichtBereit(t *testing.T) {
	// Kein Zugang in den Ebenen.
	layers, _, _ := docTree(t, map[string]string{"instructions/always/010-a.md": "## A\n"}, nil)
	expectDocError(t, layers, "keph:/manual/abc.md", DocCodeKephalaionUnavailable)

	// Der Zugang scheitert: kephalaion.ok ist nicht wahr.
	layers.Keph = func() (KephDocs, error) {
		return KephDocs{}, docRefErrorf(DocCodeKephalaionUnavailable, "Kephalaion ist für dieses Projekt nicht bereit (not-configured): kein Block")
	}
	err := expectDocError(t, layers, "keph:/manual/abc.md", DocCodeKephalaionUnavailable)
	if !strings.Contains(err.Error(), "not-configured") {
		t.Errorf("Meldung des Zugangs verloren: %v", err)
	}
	// Ohne gültigen User lässt sich ~ nicht ersetzen; ein Verweis ohne ~
	// geht trotzdem.
	store := &fakeKephStore{docs: map[string]string{"manual/abc.md": "# ABC\n"}}
	layers = kephLayers(t, store, "")
	expectDocError(t, layers, "keph:/user/~/tasks/100-a.md", DocCodeKephalaionUnavailable)
	mustResolveDoc(t, layers, "keph:/manual/abc.md")

	// Fehler des Clients gehen durch kephError.
	for code, want := range map[string]string{
		kephalaion.CodeNodeUnreachable: DocCodeKephalaionUnavailable,
		kephalaion.CodeInternal:        DocCodeKephalaionUnavailable,
		"":                             DocCodeKephalaionError,
		"forbidden":                    DocCodeKephalaionError,
	} {
		store.fail = &kephalaion.ToolError{Tool: "read", Code: code, Message: "Meldung"}
		err := expectDocError(t, layers, "keph:/manual/abc.md", want)
		if !strings.Contains(err.Error(), "Meldung") {
			t.Errorf("%q: Meldung verloren: %v", code, err)
		}
	}
	// Der Zugang wird je Ebenen höchstens einmal geöffnet.
	opened := 0
	layers.Keph = memoKephDocs(func() (KephDocs, error) {
		opened++
		return KephDocs{Store: &fakeKephStore{docs: map[string]string{"a.md": "A", "b.md": "B"}}, User: "kamran"}, nil
	})
	mustResolveDoc(t, layers, "keph:/a.md")
	mustResolveDoc(t, layers, "keph:/b.md")
	if opened != 1 {
		t.Errorf("Zugang %d-mal geöffnet", opened)
	}
}

func TestDocRefNichtUnterstuetzt(t *testing.T) {
	layers, _, _ := docTree(t, map[string]string{
		"instructions/always/010-a.md": "## A\n",
		"checks/check_x.sh":            "#!/bin/sh\n",
	}, nil)

	// Außerhalb der Sorten aus docRefKinds abgelehnt, auch wenn es die Datei
	// gibt: checks sind Skripte, kein Lesestoff; Skills liegen beim
	// Assistenten.
	expectDocError(t, layers, "k-playbook:/checks/check_x.sh", DocCodeUnsupported)
	expectDocError(t, layers, "k-playbook:/skills/ks-befunde/SKILL.md", DocCodeUnsupported)
	expectDocError(t, layers, "k-playbook:/", DocCodeUnsupported)
	expectDocError(t, layers, "k-playbook:/instructionsx/a.md", DocCodeUnsupported)
	expectDocError(t, layers, "k-playbook:/rulesx/befunde.md", DocCodeUnsupported)
	// rules/ ist eine Sorte: der Verweis darauf ist gültig, aber ohne
	// Dateinamen unvollständig — kein unsupported mehr.
	err := expectDocError(t, layers, "k-playbook:/rules/", DocCodeInvalidInput)
	if !strings.Contains(err.Error(), "Dateiname fehlt") || !strings.Contains(err.Error(), "k-playbook:/rules/<name>.md") {
		t.Errorf("rules/ mit / am Ende: %v", err)
	}
	// .. ist schon in der Form ungültig.
	expectDocError(t, layers, "k-playbook:/instructions/../rules/befunde.md", DocCodeInvalidInput)
}

func TestDocRefUmgebungsfehlerHatKeinenCode(t *testing.T) {
	layers := DocLayers{Dist: failingDocSource{}, Local: failingDocSource{}}
	_, err := ResolveDocRef(layers, "k-playbook:/instructions/always/")
	if err == nil || DocRefErrorCode(err) != "" {
		t.Fatalf("Umgebungsfehler: %v (Code %q)", err, DocRefErrorCode(err))
	}
}

type failingDocSource struct{}

func (failingDocSource) List(string) ([]DocSourceEntry, error) { return nil, errors.New("kaputt") }
func (failingDocSource) Read(string) ([]byte, error)           { return nil, errors.New("kaputt") }

// Der mitgelieferte Stand im Repo lässt sich auflösen: 010-verweise.md steht
// vorn, und die Muster-Datei unter on-demand trägt Frontmatter.
func TestDocRefMitgelieferterStand(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("Repo-Wurzel: %v", err)
	}
	layers := DocLayers{Dist: NewShippedDocSource(root), Local: NewDiskDocSource(t.TempDir())}

	always := mustResolveDoc(t, layers, "k-playbook:/instructions/always/")
	if len(always.Parts) == 0 || always.Parts[0].Name != "010-verweise.md" {
		t.Errorf("erster Teil: %+v", always.Parts)
	}
	if len(always.Problems) != 0 {
		t.Errorf("Probleme im mitgelieferten Stand: %v", always.Problems)
	}

	muster := mustResolveDoc(t, layers, "k-playbook:/instructions/on-demand/muster.md")
	if !strings.HasPrefix(muster.Content, "---\ntitle: ") || !strings.Contains(muster.Content, "\ndescription: ") {
		t.Errorf("Muster ohne Frontmatter title/description: %q", muster.Content)
	}
}

// rules, reviews und guidelines sind Sorten wie instructions: Einzeldateien,
// lokal vor mitgeliefert, leer lokal schaltet ab. Nur instructions/always/
// wird zusammengebaut.
func TestDocRefSortenRulesReviewsGuidelines(t *testing.T) {
	layers, _, _ := docTree(t, map[string]string{
		"rules/befunde.md":          "# Regel: Befunde\n\nText.\n",
		"rules/docs-sync.md":        "# Regel: Docs\n\nmitgeliefert\n",
		"reviews/review-code.md":    "---\nname: review-code\n---\n# Code\n",
		"reviews/review-tech.md":    "# Tech\n\nText.\n",
		"guidelines/betrieb.md":     "# Betrieb\n\nText.\n",
		"instructions/always/01.md": "## A\n",
	}, map[string]string{
		"rules/docs-sync.md":            "# Regel: Docs\n\nlokal\n",
		"rules/eigene.md":               "# Eigene Regel\n\nText.\n",
		"reviews/review-tech.md":        "# Abgeschaltet: kein Tech-Review.\n",
		"guidelines/oberflaeche.md":     "# Oberfläche\n\nText.\n",
		"guidelines/betrieb.md":         "",
		"guidelines/README.md":          "# Liesmich\n",
		"instructions/on-demand/x.md":   "## X\n",
		"instructions/always/README.md": "# Liesmich\n",
	})

	for ref, want := range map[string]DocPart{
		"k-playbook:/rules/befunde.md":          {Name: "befunde.md", Origin: DocOriginDist},
		"k-playbook:/rules/docs-sync.md":        {Name: "docs-sync.md", Origin: DocOriginOverride},
		"k-playbook:/rules/eigene.md":           {Name: "eigene.md", Origin: DocOriginLocal},
		"k-playbook:/reviews/review-code.md":    {Name: "review-code.md", Origin: DocOriginDist},
		"k-playbook:/reviews/review-tech.md":    {Name: "review-tech.md", Origin: DocOriginOverride, Disabled: true},
		"k-playbook:/guidelines/betrieb.md":     {Name: "betrieb.md", Origin: DocOriginOverride, Disabled: true},
		"k-playbook:/guidelines/oberflaeche.md": {Name: "oberflaeche.md", Origin: DocOriginLocal},
	} {
		result := mustResolveDoc(t, layers, ref)
		if result.Directory || len(result.Parts) != 1 || !reflect.DeepEqual(result.Parts[0], want) {
			t.Errorf("%s: %+v, erwartet Teil %+v", ref, result, want)
		}
		if result.Disabled != want.Disabled || (want.Disabled && result.Content != "") || (!want.Disabled && result.Content == "") {
			t.Errorf("%s: disabled %v, content %q", ref, result.Disabled, result.Content)
		}
	}
	if got := mustResolveDoc(t, layers, "k-playbook:/rules/docs-sync.md").Content; got != "# Regel: Docs\n\nlokal\n" {
		t.Errorf("Überlagerung liefert nicht die lokale Fassung: %q", got)
	}
	if got := mustResolveDoc(t, layers, "k-playbook:/reviews/review-code.md").Content; !strings.HasPrefix(got, "---\nname: review-code\n") {
		t.Errorf("Einzeldatei ohne Frontmatter: %q", got)
	}

	expectDocError(t, layers, "k-playbook:/rules/fehlt.md", DocCodeNotFound)
	expectDocError(t, layers, "k-playbook:/guidelines/fehlt.md", DocCodeNotFound)
	// Mit „/" am Ende fehlt der Dateiname: keine der drei Sorten wird
	// zusammengebaut.
	for _, ref := range []string{"k-playbook:/rules/", "k-playbook:/reviews/", "k-playbook:/guidelines/"} {
		err := expectDocError(t, layers, ref, DocCodeInvalidInput)
		if !strings.Contains(err.Error(), "Dateiname fehlt") {
			t.Errorf("%s: %v", ref, err)
		}
	}
}

// commands ist eine Sorte wie rules: der Text eines Commands als Einzeldatei,
// lokal vor mitgeliefert, leer lokal schaltet ab; Namensräume bleiben Pfade.
// Mit „/" am Ende fehlt der Dateiname, und checks/ bleibt unsupported (Task
// 092).
func TestDocRefSorteCommands(t *testing.T) {
	layers, _, _ := docTree(t, map[string]string{
		"commands/k-upgrade.md":       "---\ndescription: Info\n---\n\n# k-upgrade\n\nmitgeliefert\n",
		"commands/k-gui.md":           "# k-gui\n\nmitgeliefert\n",
		"commands/k-danke.md":         "# k-danke\n",
		"commands/_shared/context.md": "# Kontext\n",
		"commands/README.md":          "# Liesmich\n",
		"checks/check_x.sh":           "#!/bin/sh\n",
	}, map[string]string{
		"commands/k-gui.md":   "# k-gui\n\nlokal\n",
		"commands/k-danke.md": "",
		"commands/eigener.md": "# eigener\n\nText.\n",
	})

	for ref, want := range map[string]DocPart{
		"k-playbook:/commands/k-upgrade.md":       {Name: "k-upgrade.md", Origin: DocOriginDist},
		"k-playbook:/commands/k-gui.md":           {Name: "k-gui.md", Origin: DocOriginOverride},
		"k-playbook:/commands/k-danke.md":         {Name: "k-danke.md", Origin: DocOriginOverride, Disabled: true},
		"k-playbook:/commands/eigener.md":         {Name: "eigener.md", Origin: DocOriginLocal},
		"k-playbook:/commands/_shared/context.md": {Name: "context.md", Origin: DocOriginDist},
	} {
		result := mustResolveDoc(t, layers, ref)
		if result.Directory || len(result.Parts) != 1 || !reflect.DeepEqual(result.Parts[0], want) {
			t.Errorf("%s: %+v, erwartet Teil %+v", ref, result, want)
		}
		if result.Disabled != want.Disabled || (want.Disabled && result.Content != "") || (!want.Disabled && result.Content == "") {
			t.Errorf("%s: disabled %v, content %q", ref, result.Disabled, result.Content)
		}
	}
	// Die Einzeldatei kommt unverändert, mit Frontmatter.
	if got := mustResolveDoc(t, layers, "k-playbook:/commands/k-upgrade.md").Content; !strings.HasPrefix(got, "---\ndescription: Info\n") {
		t.Errorf("Command ohne Frontmatter: %q", got)
	}
	if got := mustResolveDoc(t, layers, "k-playbook:/commands/k-gui.md").Content; got != "# k-gui\n\nlokal\n" {
		t.Errorf("Überlagerung liefert nicht die lokale Fassung: %q", got)
	}

	expectDocError(t, layers, "k-playbook:/commands/fehlt.md", DocCodeNotFound)
	err := expectDocError(t, layers, "k-playbook:/commands/", DocCodeInvalidInput)
	if !strings.Contains(err.Error(), "Dateiname fehlt") || !strings.Contains(err.Error(), "k-playbook:/commands/<name>.md") {
		t.Errorf("commands/ mit / am Ende: %v", err)
	}
	expectDocError(t, layers, "k-playbook:/commands/_shared/", DocCodeInvalidInput)
	expectDocError(t, layers, "k-playbook:/checks/check_x.sh", DocCodeUnsupported)
}

// Dieselbe Sorte darf nicht über zwei Wege verschieden herauskommen: Name,
// Herkunft und Abschalten eines Eintrags sind über k_playbook_doc_read
// (ResolveDocRef) dieselben wie unter catalogs der Kontextausgabe, und der
// Inhalt hinter dem Verweis ist die Datei, die der Katalog nennt.
//
// commands hat keinen Katalog in der Kontextausgabe; sein zweiter Weg ist
// die Registry (ResolveRegistry), die die Hülle beim Assistenten verlinkt.
// Name, Herkunft und Abschalten sind auch dort dieselben. Der Inhalt ist es
// nur bei einer lokalen Datei: mitgeliefert liest die Registry die Hülle
// oben in commands/, der Verweis den Text in defaults/commands/ — zwei
// Dateien, mit Absicht (Task 092).
func TestDocRefUndKatalogeStimmenUeberein(t *testing.T) {
	root := newContextProject(t)
	files := map[string]string{
		"k-playbook/rules/befunde.md":              "# Befunde\n\nText.\n",
		"k-playbook/rules/docs-sync.md":            "# Docs\n\nmitgeliefert\n",
		"k-playbook/rules/README.md":               "# Liesmich\n",
		"k-playbook/reviews/review-code.md":        "---\nname: review-code\naudit:\n  enabled: true\n---\n# Code\n",
		"k-playbook/reviews/review-tech.md":        "# Tech\n",
		"k-playbook/reviews/review-sast.md":        "# SAST\n",
		"k-playbook/reviews/notiz.md":              "# kein Rezept\n",
		"k-playbook-local/rules/docs-sync.md":      "# Docs\n\nlokal\n",
		"k-playbook-local/rules/eigene.md":         "# Eigene\n\nText.\n",
		"k-playbook-local/rules/aus.md":            "",
		"k-playbook-local/reviews/review-tech.md":  "# Abgeschaltet.\n",
		"k-playbook-local/reviews/review-eigen.md": "# Eigen\n\nText.\n",
		"k-playbook-local/reviews/.versteckt.md":   "# versteckt\n",
	}
	for rel, content := range files {
		write(t, filepath.Join(root, filepath.FromSlash(rel)), content)
	}

	context, err := BuildContext(root)
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}
	layers := ProjectDocLayers(root)
	for _, kind := range []string{"rules", "reviews"} {
		entries := context.Catalogs[kind]
		if len(entries) == 0 {
			t.Fatalf("Katalog %s ist leer", kind)
		}
		for _, entry := range entries {
			ref := "k-playbook:/" + kind + "/" + entry.Name
			result := mustResolveDoc(t, layers, ref)
			part := result.Parts[0]
			if part.Name != entry.Name || part.Origin != entry.Origin || part.Disabled != entry.Disabled || result.Disabled != entry.Disabled {
				t.Errorf("%s: Verweis %+v, Katalog %+v", ref, part, entry)
			}
			if entry.Disabled {
				continue
			}
			data, err := os.ReadFile(entry.Path)
			if err != nil {
				t.Fatalf("%s lesen: %v", entry.Path, err)
			}
			if result.Content != string(data) {
				t.Errorf("%s: Inhalt %q, Katalogdatei %s hat %q", ref, result.Content, entry.Path, data)
			}
		}
	}
	// Was der Katalog nicht führt, führt auch der Verweis nicht als Eintrag
	// der Sorte — README.md und Dotfiles kommen über keinen der beiden Wege.
	if _, ok := entryByName(context.Catalogs["rules"], "README.md"); ok {
		t.Error("README.md im Katalog")
	}
	if _, ok := entryByName(context.Catalogs["reviews"], ".versteckt.md"); ok {
		t.Error("Dotfile im Katalog")
	}
	if _, ok := entryByName(context.Catalogs["reviews"], "notiz.md"); ok {
		t.Error("Datei ohne Sortenpräfix im Katalog")
	}
	// Die Modi der Rezepte kommen aus demselben Inhalt.
	code, ok := entryByName(context.Catalogs["reviews"], "review-code.md")
	if !ok || code.Audit == nil || !code.Audit.Enabled {
		t.Errorf("review-code: %+v", code)
	}

	// commands: Registry und Verweis.
	for rel, content := range map[string]string{
		"k-playbook/commands/k-upgrade.md":          "---\ndescription: Hülle\n---\n\n# k-upgrade\n\nHülle\n",
		"k-playbook/defaults/commands/k-upgrade.md": "# k-upgrade\n\nText\n",
		"k-playbook/commands/k-gui.md":              "# k-gui\n\nHülle\n",
		"k-playbook/defaults/commands/k-gui.md":     "# k-gui\n\nText\n",
		"k-playbook/commands/k-danke.md":            "# k-danke\n",
		"k-playbook/defaults/commands/k-danke.md":   "# k-danke\n\nText\n",
		"k-playbook-local/commands/k-gui.md":        "# k-gui\n\nlokal\n",
		"k-playbook-local/commands/k-danke.md":      "",
		"k-playbook-local/commands/eigener.md":      "# eigener\n\nText.\n",
	} {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		write(t, full, content)
	}
	if _, ok := context.Catalogs["commands"]; ok {
		t.Error("commands im Katalog der Kontextausgabe")
	}
	registry := ResolveRegistry(root, KindCommands)
	if len(registry) != 4 {
		t.Fatalf("Registry: %+v", registry)
	}
	for _, entry := range registry {
		ref := "k-playbook:/commands/" + entry.Name
		result := mustResolveDoc(t, layers, ref)
		part := result.Parts[0]
		if part.Name != entry.Name || part.Origin != entry.Origin || part.Disabled != entry.Disabled || result.Disabled != entry.Disabled {
			t.Errorf("%s: Verweis %+v, Registry %+v", ref, part, entry)
		}
		if entry.Disabled {
			continue
		}
		data, err := os.ReadFile(entry.Path)
		if err != nil {
			t.Fatalf("%s lesen: %v", entry.Path, err)
		}
		switch entry.Origin {
		case "dist":
			if result.Content == string(data) || !strings.HasSuffix(result.Content, "Text\n") {
				t.Errorf("%s: Verweis liefert %q, erwartet den Text aus defaults/, nicht die Hülle %s", ref, result.Content, entry.Path)
			}
		default:
			if result.Content != string(data) {
				t.Errorf("%s: Inhalt %q, Registry-Datei %s hat %q", ref, result.Content, entry.Path, data)
			}
		}
	}
}

// Die mitgelieferte Ebene liest instructions/ und commands/ aus defaults/,
// die übrigen Sorten von oben — es gibt nur eine Fassung der Anweisungen,
// keine Kopie; bei commands/ liegt oben die Hülle, in defaults/ der Text.
func TestShippedDocSourceLiestInstructionsAusDefaults(t *testing.T) {
	playbook := t.TempDir()
	for rel, content := range map[string]string{
		"defaults/instructions/always/010-a.md": "## A\n",
		"defaults/instructions/on-demand/x.md":  "## X\n",
		"instructions/always/999-oben.md":       "## Oben\n",
		"rules/befunde.md":                      "# Regel\n",
		"defaults/rules/befunde.md":             "# Regel aus defaults\n",
		"commands/k-upgrade.md":                 "# k-upgrade\n\nHülle\n",
		"defaults/commands/k-upgrade.md":        "# k-upgrade\n\nText\n",
		"commands/k-gui.md":                     "# k-gui\n\nnur oben\n",
	} {
		full := filepath.Join(playbook, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	layers := DocLayers{Dist: NewShippedDocSource(playbook), Local: NewDiskDocSource(t.TempDir())}

	always := mustResolveDoc(t, layers, "k-playbook:/instructions/always/")
	if len(always.Parts) != 1 || always.Parts[0].Name != "010-a.md" {
		t.Errorf("always aus defaults/ erwartet, bekommen: %+v", always.Parts)
	}
	if x := mustResolveDoc(t, layers, "k-playbook:/instructions/on-demand/x.md"); x.Content != "## X\n" {
		t.Errorf("on-demand aus defaults/: %q", x.Content)
	}
	if rule := mustResolveDoc(t, layers, "k-playbook:/rules/befunde.md"); rule.Content != "# Regel\n" {
		t.Errorf("rules weiter von oben erwartet: %q", rule.Content)
	}
	if cmd := mustResolveDoc(t, layers, "k-playbook:/commands/k-upgrade.md"); cmd.Content != "# k-upgrade\n\nText\n" {
		t.Errorf("commands aus defaults/ erwartet, nicht die Hülle: %q", cmd.Content)
	}
	// Ein Command, der nur oben als Hülle liegt, hat keinen Text: not_found,
	// kein Rückfall auf die Hülle.
	expectDocError(t, layers, "k-playbook:/commands/k-gui.md", DocCodeNotFound)
}

// includeTree sind Ebenen mit Bausteinen für Include-Tests: Einzeldateien
// unter rules/ und on-demand/, ein zusammengebautes Verzeichnis.
func includeTree(t *testing.T, local map[string]string) DocLayers {
	t.Helper()
	layers, _, _ := docTree(t, map[string]string{
		"rules/haupt.md":                  "---\ntitle: Haupt\n---\n\n# Haupt\n\ninclude: k-playbook:/rules/baustein.md\n\nEnde.\n",
		"rules/baustein.md":               "---\ntitle: Baustein\n---\n\n## Baustein\n\nText.\n",
		"rules/relativ.md":                "# Relativ\n\ninclude: teile/stueck.md\n",
		"rules/teile/stueck.md":           "Stück.\n",
		"rules/hoch.md":                   "include: teile/../baustein.md\n",
		"rules/verschachtelt.md":          "# V\n\ninclude: haupt.md\n",
		"rules/codeblock.md":              "# Code\n\n```markdown\ninclude: k-playbook:/rules/baustein.md\n```\n\n~~~\ninclude: baustein.md\n~~~\n\ninclude: baustein.md\n",
		"rules/zyklus-a.md":               "A\n\ninclude: zyklus-b.md\n",
		"rules/zyklus-b.md":               "B\n\ninclude: zyklus-a.md\n",
		"rules/tot.md":                    "# Tot\n\ninclude: fehlt.md\n",
		"rules/zwei-argumente.md":         "include: a.md b.md\n",
		"rules/nicht-sorte.md":            "include: k-playbook:/checks/check_x.sh\n",
		"rules/verzeichnis.md":            "include: k-playbook:/instructions/on-demand/\n",
		"rules/always.md":                 "# Always\n\ninclude: k-playbook:/instructions/always/\n",
		"instructions/always/010-a.md":    "## A\n\ninclude: k-playbook:/rules/baustein.md\n",
		"instructions/always/020-b.md":    "## B\n",
		"instructions/on-demand/thema.md": "## Thema\n",
		"checks/check_x.sh":               "#!/bin/sh\n",
	}, local)
	return layers
}

func includeRefs(includes []DocInclude) []string {
	refs := make([]string, 0, len(includes))
	for _, inc := range includes {
		refs = append(refs, inc.Ref)
	}
	return refs
}

func TestDocRefIncludeSetztRumpfEin(t *testing.T) {
	layers := includeTree(t, nil)

	// Das Hauptdokument behält sein Frontmatter; eingesetzt wird der Rumpf
	// des Ziels ohne dessen Frontmatter.
	doc := mustResolveDoc(t, layers, "k-playbook:/rules/haupt.md")
	if doc.Content != "---\ntitle: Haupt\n---\n\n# Haupt\n\n## Baustein\n\nText.\n\nEnde.\n" {
		t.Errorf("Inhalt: %q", doc.Content)
	}
	if !reflect.DeepEqual(doc.Includes, []DocInclude{{Ref: "k-playbook:/rules/baustein.md", Origin: DocOriginDist, Depth: 1}}) {
		t.Errorf("includes: %+v", doc.Includes)
	}
	if len(doc.Parts) != 1 || doc.Parts[0].Name != "haupt.md" {
		t.Errorf("parts bleiben die Teile der Datei: %+v", doc.Parts)
	}

	// Relativ gegen das Verzeichnis des einbindenden Dokuments, auch aus dem
	// Unterverzeichnis heraus nicht mit „..“.
	rel := mustResolveDoc(t, layers, "k-playbook:/rules/relativ.md")
	if rel.Content != "# Relativ\n\nStück.\n" {
		t.Errorf("relativ: %q", rel.Content)
	}
	if !reflect.DeepEqual(includeRefs(rel.Includes), []string{"k-playbook:/rules/teile/stueck.md"}) {
		t.Errorf("includes relativ: %+v", rel.Includes)
	}

	// Verschachtelt: Tiefe 2, Frontmatter des eingebundenen Hauptdokuments
	// verschwindet, weil nur der Rumpf eingesetzt wird.
	nested := mustResolveDoc(t, layers, "k-playbook:/rules/verschachtelt.md")
	if nested.Content != "# V\n\n# Haupt\n\n## Baustein\n\nText.\n\nEnde.\n" {
		t.Errorf("verschachtelt: %q", nested.Content)
	}
	if len(nested.Includes) != 2 || nested.Includes[0].Depth != 1 || nested.Includes[1].Depth != 2 {
		t.Errorf("Tiefen: %+v", nested.Includes)
	}

	// In Code-Blöcken bleibt die Zeile stehen, beide Zaunarten.
	code := mustResolveDoc(t, layers, "k-playbook:/rules/codeblock.md")
	if code.Content != "# Code\n\n```markdown\ninclude: k-playbook:/rules/baustein.md\n```\n\n~~~\ninclude: baustein.md\n~~~\n\n## Baustein\n\nText.\n" {
		t.Errorf("codeblock: %q", code.Content)
	}
	if len(code.Includes) != 1 {
		t.Errorf("includes im Code-Block gezählt: %+v", code.Includes)
	}

	// Ohne Include-Zeile ist includes leer, nie nil.
	if plain := mustResolveDoc(t, layers, "k-playbook:/rules/baustein.md"); plain.Includes == nil || len(plain.Includes) != 0 {
		t.Errorf("includes ohne Include: %#v", plain.Includes)
	}
}

func TestDocRefIncludeRohtextUndAbgeschaltet(t *testing.T) {
	layers := includeTree(t, map[string]string{
		"rules/baustein.md": "# Abgeschaltet: kein Baustein.\n",
	})

	// Raw: die Zeile bleibt, includes ist leer.
	raw, err := ResolveDocRefWith(context.Background(), layers, "k-playbook:/rules/haupt.md", DocReadOptions{Raw: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(raw.Content, "\ninclude: k-playbook:/rules/baustein.md\n") || len(raw.Includes) != 0 || raw.Includes == nil {
		t.Errorf("raw: %+v", raw)
	}
	// Ein abgeschaltetes Ziel setzt nichts ein, kein Fehler; die Zeile fällt
	// weg, und includes nennt das Ziel als abgeschaltet.
	doc := mustResolveDoc(t, layers, "k-playbook:/rules/haupt.md")
	if doc.Content != "---\ntitle: Haupt\n---\n\n# Haupt\n\n\nEnde.\n" {
		t.Errorf("abgeschaltetes Ziel: %q", doc.Content)
	}
	if !reflect.DeepEqual(doc.Includes, []DocInclude{{Ref: "k-playbook:/rules/baustein.md", Origin: DocOriginOverride, Depth: 1, Disabled: true}}) {
		t.Errorf("includes: %+v", doc.Includes)
	}
	// Eine lokale Fassung des Bausteins gewinnt — dieselbe Überlagerung wie
	// beim Verweis selbst.
	layers = includeTree(t, map[string]string{"rules/baustein.md": "## Lokal\n\nText.\n"})
	doc = mustResolveDoc(t, layers, "k-playbook:/rules/haupt.md")
	if !strings.Contains(doc.Content, "## Lokal\n") || doc.Includes[0].Origin != DocOriginOverride {
		t.Errorf("lokaler Baustein: %+v", doc)
	}
	// Auch das einbindende Dokument lokal: relativ zählt im selben Präfix,
	// die Auflösung des Ziels ist die normale.
	layers = includeTree(t, map[string]string{"rules/eigen.md": "# Eigen\n\ninclude: baustein.md\n"})
	if doc = mustResolveDoc(t, layers, "k-playbook:/rules/eigen.md"); doc.Content != "# Eigen\n\n## Baustein\n\nText.\n" {
		t.Errorf("lokales Dokument mit Include: %q", doc.Content)
	}
}

func TestDocRefIncludeFehlerTragenDieKette(t *testing.T) {
	layers := includeTree(t, nil)

	err := expectDocError(t, layers, "k-playbook:/rules/zyklus-a.md", DocCodeIncludeCycle)
	if !strings.Contains(err.Error(), "Include-Kette: k-playbook:/rules/zyklus-a.md → k-playbook:/rules/zyklus-b.md → k-playbook:/rules/zyklus-a.md") {
		t.Errorf("Zyklus ohne Kette: %v", err)
	}
	// Die Kette steht genau einmal in der Meldung.
	if strings.Count(err.Error(), "Include-Kette") != 1 {
		t.Errorf("Kette mehrfach: %v", err)
	}
	err = expectDocError(t, layers, "k-playbook:/rules/tot.md", DocCodeNotFound)
	if !strings.Contains(err.Error(), "k-playbook:/rules/fehlt.md ist unbekannt") ||
		!strings.Contains(err.Error(), "Include-Kette: k-playbook:/rules/tot.md → k-playbook:/rules/fehlt.md") {
		t.Errorf("totes Ziel: %v", err)
	}
	err = expectDocError(t, layers, "k-playbook:/rules/zwei-argumente.md", DocCodeInvalidInput)
	if !strings.Contains(err.Error(), "genau ein Argument") || !strings.Contains(err.Error(), "Include-Kette") {
		t.Errorf("zwei Argumente: %v", err)
	}
	err = expectDocError(t, layers, "k-playbook:/rules/nicht-sorte.md", DocCodeUnsupported)
	if !strings.Contains(err.Error(), "k-playbook:/rules/nicht-sorte.md → k-playbook:/checks/check_x.sh") {
		t.Errorf("Ziel außerhalb der Sorten: %v", err)
	}
	err = expectDocError(t, layers, "k-playbook:/rules/verzeichnis.md", DocCodeInvalidInput)
	if !strings.Contains(err.Error(), "Dateiname fehlt") {
		t.Errorf("Verzeichnis ohne Zusammenbau als Ziel: %v", err)
	}
	// Aus der Sorte führt kein relativer Pfad heraus: „..“ ist ungültig.
	err = expectDocError(t, layers, "k-playbook:/rules/hoch.md", DocCodeInvalidInput)
	if !strings.Contains(err.Error(), `".." ist im Pfad nicht erlaubt`) || !strings.Contains(err.Error(), "Include-Kette") {
		t.Errorf("..: %v", err)
	}

	// Tiefe: 00 bindet 01 ein … 10 bindet 11 ein; 11 läge auf Tiefe 11.
	chain := map[string]string{}
	for i := 0; i <= 11; i++ {
		content := fmt.Sprintf("Stufe %02d\n", i)
		if i < 11 {
			content += fmt.Sprintf("\ninclude: zu-tief-%02d.md\n", i+1)
		}
		chain[fmt.Sprintf("rules/zu-tief-%02d.md", i)] = content
	}
	layers, _, _ = docTree(t, chain, nil)
	err = expectDocError(t, layers, "k-playbook:/rules/zu-tief-00.md", DocCodeIncludeTooDeep)
	if !strings.Contains(err.Error(), "k-playbook:/rules/zu-tief-10.md → k-playbook:/rules/zu-tief-11.md") {
		t.Errorf("zu tief: %v", err)
	}
	// Von 01 aus reicht die Kette nur bis Tiefe 10: kein Fehler.
	deep := mustResolveDoc(t, layers, "k-playbook:/rules/zu-tief-01.md")
	if len(deep.Includes) != 10 || deep.Includes[9].Depth != 10 || !strings.HasSuffix(deep.Content, "Stufe 11\n") {
		t.Errorf("Tiefe 10 erlaubt: %d Includes, Inhalt %q", len(deep.Includes), deep.Content)
	}
	// Mit Raw gibt es keinen Fehler: nichts wird eingesetzt.
	if _, err := ResolveDocRefWith(context.Background(), includeTree(t, nil), "k-playbook:/rules/zyklus-a.md", DocReadOptions{Raw: true}); err != nil {
		t.Errorf("raw mit Zyklus: %v", err)
	}
}

// Zwei Fehlerebenen: in einem zusammengebauten Verzeichnis landet der
// Include-Fehler eines Teils unter problems, der Teil fehlt, die übrigen
// bleiben; die Teile setzen ihre Includes für sich ein.
func TestDocRefIncludeImZusammengebautenVerzeichnis(t *testing.T) {
	layers := includeTree(t, map[string]string{
		"instructions/always/1000-kaputt.md": "## Kaputt\n\ninclude: gibt-es-nicht.md\n",
		"instructions/always/1100-eigen.md":  "## Eigen\n\ninclude: k-playbook:/instructions/on-demand/thema.md\n",
	})

	dir := mustResolveDoc(t, layers, "k-playbook:/instructions/always/")
	if dir.Content != "## A\n\n## Baustein\n\nText.\n\n## B\n\n## Eigen\n\n## Thema\n" {
		t.Errorf("Zusammenbau mit Includes: %q", dir.Content)
	}
	if !reflect.DeepEqual(partNames(dir.Parts), []string{"010-a.md", "020-b.md", "1100-eigen.md"}) {
		t.Errorf("Teile: %+v", dir.Parts)
	}
	if len(dir.Problems) != 1 || !strings.Contains(dir.Problems[0], "1000-kaputt.md") ||
		!strings.Contains(dir.Problems[0], "Include-Kette: k-playbook:/instructions/always/ → k-playbook:/instructions/always/1000-kaputt.md → k-playbook:/instructions/always/gibt-es-nicht.md") ||
		!strings.Contains(dir.Problems[0], "nicht aufgenommen") {
		t.Errorf("problems: %v", dir.Problems)
	}
	if !reflect.DeepEqual(includeRefs(dir.Includes), []string{"k-playbook:/rules/baustein.md", "k-playbook:/instructions/on-demand/thema.md"}) ||
		dir.Includes[0].Depth != 1 {
		t.Errorf("includes: %+v", dir.Includes)
	}

	// Raw: Zeilen bleiben, kein Include-Problem.
	raw, err := ResolveDocRefWith(context.Background(), layers, "k-playbook:/instructions/always/", DocReadOptions{Raw: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(raw.Content, "include: gibt-es-nicht.md") || len(raw.Problems) != 0 || len(raw.Includes) != 0 {
		t.Errorf("raw: %+v", raw)
	}

	// Ein Verzeichnis als Include-Ziel einer Datei: sein Text wird
	// eingesetzt, sein Problem gemeldet, es erscheint ohne Herkunft in
	// includes, seine eigenen Includes dahinter.
	file := mustResolveDoc(t, layers, "k-playbook:/rules/always.md")
	if file.Content != "# Always\n\n## A\n\n## Baustein\n\nText.\n\n## B\n\n## Eigen\n\n## Thema\n" {
		t.Errorf("Verzeichnis als Ziel: %q", file.Content)
	}
	if len(file.Problems) != 1 || !strings.Contains(file.Problems[0], "1000-kaputt.md") {
		t.Errorf("problems des eingebundenen Verzeichnisses: %v", file.Problems)
	}
	if !reflect.DeepEqual(file.Includes, []DocInclude{
		{Ref: "k-playbook:/instructions/always/", Depth: 1},
		{Ref: "k-playbook:/rules/baustein.md", Origin: DocOriginDist, Depth: 2},
		{Ref: "k-playbook:/instructions/on-demand/thema.md", Origin: DocOriginDist, Depth: 2},
	}) {
		t.Errorf("includes: %+v", file.Includes)
	}
	// Ein Teil, der das eigene Verzeichnis einbindet: Zyklus, nur dieser
	// Teil fällt aus.
	layers = includeTree(t, map[string]string{"instructions/always/1000-selbst.md": "## Selbst\n\ninclude: k-playbook:/instructions/always/\n"})
	dir = mustResolveDoc(t, layers, "k-playbook:/instructions/always/")
	if len(dir.Problems) != 1 || !strings.Contains(dir.Problems[0], "include_cycle") && !strings.Contains(dir.Problems[0], "Zyklus") || len(dir.Parts) != 2 {
		t.Errorf("Zyklus über das Verzeichnis: %+v", dir)
	}
}

// project: als einbindendes Dokument und als Ziel, mit angelegten Dateien.
func TestDocRefIncludeProjekt(t *testing.T) {
	layers := includeTree(t, nil)
	root := t.TempDir()
	writeDocTree(t, root, map[string]string{
		"docs/haupt.md":        "# Projekt\n\ninclude: teile/stueck.md\n\ninclude: k-playbook:/rules/baustein.md\n",
		"docs/teile/stueck.md": "Projektstück.\n",
		"docs/leer.md":         "",
		"docs/leer-ziel.md":    "# L\n\ninclude: leer.md\n",
		"docs/kaputt.md":       "include: fehlt.md\n",
	})
	layers.Project = NewDiskDocSource(root)

	doc := mustResolveDoc(t, layers, "project:/docs/haupt.md")
	if doc.Content != "# Projekt\n\nProjektstück.\n\n## Baustein\n\nText.\n" {
		t.Errorf("project mit Includes: %q", doc.Content)
	}
	if !reflect.DeepEqual(doc.Includes, []DocInclude{
		{Ref: "project:/docs/teile/stueck.md", Origin: DocOriginProject, Depth: 1},
		{Ref: "k-playbook:/rules/baustein.md", Origin: DocOriginDist, Depth: 1},
	}) {
		t.Errorf("includes: %+v", doc.Includes)
	}
	// Ein leeres Projektziel setzt nichts ein und ist nicht abgeschaltet.
	leer := mustResolveDoc(t, layers, "project:/docs/leer-ziel.md")
	if leer.Content != "# L\n\n" || leer.Includes[0].Disabled {
		t.Errorf("leeres Ziel: %+v", leer)
	}
	err := expectDocError(t, layers, "project:/docs/kaputt.md", DocCodeNotFound)
	if !strings.Contains(err.Error(), "project:/docs/kaputt.md → project:/docs/fehlt.md") {
		t.Errorf("Kette: %v", err)
	}
	// Aus k-playbook: heraus ein project:-Ziel.
	layers2 := includeTree(t, map[string]string{"rules/projekt.md": "# P\n\ninclude: project:/docs/teile/stueck.md\n"})
	layers2.Project = layers.Project
	if p := mustResolveDoc(t, layers2, "k-playbook:/rules/projekt.md"); p.Content != "# P\n\nProjektstück.\n" || p.Includes[0].Origin != DocOriginProject {
		t.Errorf("project: als Ziel: %+v", p)
	}
}

// keph: als einbindendes Dokument und als Ziel, ~ relativ.
func TestDocRefIncludeKeph(t *testing.T) {
	store := &fakeKephStore{docs: map[string]string{
		"user/kamran/tasks/100-a.md":   "---\ntitle: A\n---\n\n# A\n\ninclude: notizen.md\n\ninclude: k-playbook:/rules/baustein.md\n",
		"user/kamran/tasks/notizen.md": "Notiz.\n\ninclude: keph:/user/~/tasks/100-a.md\n",
		"manual/x.md":                  "X\n",
	}}
	layers := includeTree(t, map[string]string{"rules/keph.md": "# K\n\ninclude: keph:/manual/x.md\n"})
	layers.Keph = func() (KephDocs, error) { return KephDocs{Store: store, User: "kamran"}, nil }

	// notizen.md bindet über ~ das Hauptdokument wieder ein: Zyklus, erkannt
	// am echten Namen.
	err := expectDocError(t, layers, "keph:/user/~/tasks/100-a.md", DocCodeIncludeCycle)
	if !strings.Contains(err.Error(), "keph:/user/kamran/tasks/notizen.md") {
		t.Errorf("Zyklus über keph: %v", err)
	}
	store.docs["user/kamran/tasks/notizen.md"] = "Notiz.\n"
	doc := mustResolveDoc(t, layers, "keph:/user/~/tasks/100-a.md")
	if doc.Content != "---\ntitle: A\n---\n\n# A\n\nNotiz.\n\n## Baustein\n\nText.\n" {
		t.Errorf("keph mit Includes: %q", doc.Content)
	}
	if doc.Includes[0].Ref != "keph:/user/kamran/tasks/notizen.md" || doc.Includes[0].Origin != DocOriginKeph {
		t.Errorf("includes: %+v", doc.Includes)
	}
	if k := mustResolveDoc(t, layers, "k-playbook:/rules/keph.md"); k.Content != "# K\n\nX\n" {
		t.Errorf("keph: als Ziel: %q", k.Content)
	}
	// Ohne Zugang scheitert das Include mit kephalaion_unavailable und Kette.
	layers.Keph = nil
	err = expectDocError(t, layers, "k-playbook:/rules/keph.md", DocCodeKephalaionUnavailable)
	if !strings.Contains(err.Error(), "k-playbook:/rules/keph.md → keph:/manual/x.md") {
		t.Errorf("Kette: %v", err)
	}
}

// includes/ ist eine Sorte: mitgeliefert aus defaults/, lokal ersetzt, leer
// schaltet ab; mit „/“ am Ende fehlt der Dateiname. Geprüft mit testeigenen
// Ebenen — defaults/includes/ trägt nur die README und keinen Baustein.
func TestDocRefSorteIncludes(t *testing.T) {
	layers, _, _ := docTree(t, map[string]string{
		"includes/context.md": "---\ntitle: Kontext\n---\n\n## Kontext\n\nMitgeliefert.\n",
		"includes/ersetzt.md": "## Ersetzt\n\nMitgeliefert.\n",
		"includes/aus.md":     "## Aus\n\nMitgeliefert.\n",
		"includes/README.md":  "# Liesmich\n",
		"commands/k-danke.md": "# k-danke\n\ninclude: k-playbook:/includes/context.md\n\ninclude: k-playbook:/includes/aus.md\n",
		"commands/relativ.md": "include: ../includes/context.md\n",
		"testdata/x/fall.md":  "## Fall\n",
	}, map[string]string{
		"includes/ersetzt.md": "## Ersetzt\n\nLokal.\n",
		"includes/aus.md":     "",
		"includes/eigen.md":   "## Eigen\n\nLokal.\n",
	})

	for ref, want := range map[string]DocPart{
		"k-playbook:/includes/context.md": {Name: "context.md", Origin: DocOriginDist},
		"k-playbook:/includes/ersetzt.md": {Name: "ersetzt.md", Origin: DocOriginOverride},
		"k-playbook:/includes/aus.md":     {Name: "aus.md", Origin: DocOriginOverride, Disabled: true},
		"k-playbook:/includes/eigen.md":   {Name: "eigen.md", Origin: DocOriginLocal},
		"k-playbook:/testdata/x/fall.md":  {Name: "fall.md", Origin: DocOriginDist},
	} {
		result := mustResolveDoc(t, layers, ref)
		if len(result.Parts) != 1 || !reflect.DeepEqual(result.Parts[0], want) || result.Disabled != want.Disabled {
			t.Errorf("%s: %+v, erwartet Teil %+v", ref, result, want)
		}
	}
	if got := mustResolveDoc(t, layers, "k-playbook:/includes/ersetzt.md").Content; got != "## Ersetzt\n\nLokal.\n" {
		t.Errorf("lokale Fassung: %q", got)
	}
	// Ein Command bindet Bausteine ein; der abgeschaltete setzt nichts ein.
	cmd := mustResolveDoc(t, layers, "k-playbook:/commands/k-danke.md")
	if cmd.Content != "# k-danke\n\n## Kontext\n\nMitgeliefert.\n\n" {
		t.Errorf("Command mit Bausteinen: %q", cmd.Content)
	}
	if !reflect.DeepEqual(cmd.Includes, []DocInclude{
		{Ref: "k-playbook:/includes/context.md", Origin: DocOriginDist, Depth: 1},
		{Ref: "k-playbook:/includes/aus.md", Origin: DocOriginOverride, Depth: 1, Disabled: true},
	}) {
		t.Errorf("includes: %+v", cmd.Includes)
	}
	// Relativ führt kein Pfad aus der Sorte: der absolute Verweis ist nötig.
	expectDocError(t, layers, "k-playbook:/commands/relativ.md", DocCodeInvalidInput)

	expectDocError(t, layers, "k-playbook:/includes/fehlt.md", DocCodeNotFound)
	err := expectDocError(t, layers, "k-playbook:/includes/", DocCodeInvalidInput)
	if !strings.Contains(err.Error(), "Dateiname fehlt") || !strings.Contains(err.Error(), "k-playbook:/includes/<name>.md") {
		t.Errorf("includes/ mit / am Ende: %v", err)
	}
	expectDocError(t, layers, "k-playbook:/testdata/", DocCodeInvalidInput)
	expectDocError(t, layers, "k-playbook:/testdata/x/", DocCodeInvalidInput)
	// Die Sorten erscheinen nicht in der Hilfe der unsupported-Meldung, und
	// die Kataloge führen sie nicht.
	err = expectDocError(t, layers, "k-playbook:/checks/check_x.sh", DocCodeUnsupported)
	if strings.Contains(err.Error(), "includes") || strings.Contains(err.Error(), "testdata") {
		t.Errorf("Lesersorten in der Meldung: %v", err)
	}
	for _, kind := range catalogKinds() {
		if kind.name == docIncludesRoot || kind.name == docTestdataRoot {
			t.Errorf("%s im Katalog", kind.name)
		}
	}
}

// repoDefaultsLayers liest das echte defaults/ des Repos als mitgelieferte
// Ebene — so, wie eine Installation es nach dem Release liest.
func repoDefaultsLayers(t *testing.T) DocLayers {
	t.Helper()
	repo, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(repo, "defaults", "testdata", "includes", "README.md")); err != nil {
		t.Fatalf("defaults/testdata/includes/ im Repo nicht gefunden: %v", err)
	}
	return DocLayers{Dist: NewShippedDocSource(repo), Local: NewDiskDocSource(t.TempDir())}
}

// Die festen Fälle in defaults/testdata/includes/: jeder einmal mit und
// einmal ohne Includes. Nach dem Release liefert
// `k-playbook read k-playbook:/testdata/includes/<fall>.md` dasselbe.
func TestDocRefTestdataIncludes(t *testing.T) {
	layers := repoDefaultsLayers(t)
	const base = "k-playbook:/testdata/includes/"
	baustein := "## Baustein\n\nDieser Absatz kommt aus baustein.md."
	raw := func(ref string) DocResult {
		t.Helper()
		result, err := ResolveDocRefWith(context.Background(), layers, ref, DocReadOptions{Raw: true})
		if err != nil {
			t.Fatalf("%s roh: %v", ref, err)
		}
		if len(result.Includes) != 0 {
			t.Errorf("%s roh: includes %+v", ref, result.Includes)
		}
		return result
	}

	cases := []struct {
		file     string
		content  string
		includes []DocInclude
	}{
		{"einfach.md", "# Einfach\n\n" + baustein + "\n\nEnde.\n",
			[]DocInclude{{Ref: base + "baustein.md", Origin: DocOriginDist, Depth: 1}}},
		{"baustein.md", "---\ntitle: Baustein\ndescription: Darf im eingesetzten Text nicht erscheinen.\n---\n\n" + baustein + "\n", []DocInclude{}},
		{"verschachtelt.md", "# Verschachtelt\n\n# Einfach\n\n" + baustein + "\n\nEnde.\n",
			[]DocInclude{{Ref: base + "einfach.md", Origin: DocOriginDist, Depth: 1}, {Ref: base + "baustein.md", Origin: DocOriginDist, Depth: 2}}},
		{"relativ.md", "# Relativ\n\nEin Stück aus dem Unterverzeichnis teile/.\n",
			[]DocInclude{{Ref: base + "teile/stueck.md", Origin: DocOriginDist, Depth: 1}}},
		{"codeblock.md", "# Codeblock\n\nSo sieht eine Include-Zeile aus:\n\n```markdown\ninclude: k-playbook:/includes/beispiel.md\n```\n\nUnd mit Tilden:\n\n~~~\ninclude: baustein.md\n~~~\n\nDanach ein echtes Include:\n\n" + baustein + "\n",
			[]DocInclude{{Ref: base + "baustein.md", Origin: DocOriginDist, Depth: 1}}},
	}
	for _, c := range cases {
		ref := base + c.file
		got := mustResolveDoc(t, layers, ref)
		if got.Content != c.content {
			t.Errorf("%s:\n%q\nerwartet\n%q", ref, got.Content, c.content)
		}
		if !reflect.DeepEqual(got.Includes, c.includes) {
			t.Errorf("%s includes: %+v, erwartet %+v", ref, got.Includes, c.includes)
		}
		if len(got.Problems) != 0 || got.Disabled || got.Directory {
			t.Errorf("%s: %+v", ref, got)
		}
		// Roh: die Datei, wie sie auf der Platte liegt.
		data, err := os.ReadFile(filepath.Join("..", "..", "..", "defaults", "testdata", "includes", filepath.FromSlash(c.file)))
		if err != nil {
			t.Fatal(err)
		}
		if r := raw(ref); r.Content != string(data) {
			t.Errorf("%s roh: %q, Datei %q", ref, r.Content, data)
		}
	}

	// Zu tief: ab 00 liegt 11 auf Tiefe 11; ab 01 reicht die Kette genau
	// bis Tiefe 10.
	err := expectDocError(t, layers, base+"zu-tief-00.md", DocCodeIncludeTooDeep)
	if !strings.Contains(err.Error(), base+"zu-tief-10.md → "+base+"zu-tief-11.md") {
		t.Errorf("zu tief: %v", err)
	}
	deep := mustResolveDoc(t, layers, base+"zu-tief-01.md")
	wantDeep := "Stufe 01"
	for i := 2; i <= 11; i++ {
		wantDeep += fmt.Sprintf("\n\nStufe %02d", i)
	}
	if deep.Content != wantDeep+"\n" || len(deep.Includes) != 10 || deep.Includes[9].Depth != 10 || deep.Includes[9].Ref != base+"zu-tief-11.md" {
		t.Errorf("zu-tief-01: %q, includes %+v", deep.Content, deep.Includes)
	}
	raw(base + "zu-tief-00.md")

	// Zyklus und totes Ziel: Fehler mit Kette; roh lesbar.
	err = expectDocError(t, layers, base+"zyklus-a.md", DocCodeIncludeCycle)
	if !strings.Contains(err.Error(), base+"zyklus-a.md → "+base+"zyklus-b.md → "+base+"zyklus-a.md") {
		t.Errorf("Zyklus: %v", err)
	}
	expectDocError(t, layers, base+"zyklus-b.md", DocCodeIncludeCycle)
	err = expectDocError(t, layers, base+"tot.md", DocCodeNotFound)
	if !strings.Contains(err.Error(), base+"tot.md → "+base+"gibt-es-nicht.md") {
		t.Errorf("tot: %v", err)
	}
	for _, file := range []string{"zyklus-a.md", "zyklus-b.md", "tot.md"} {
		if r := raw(base + file); !strings.Contains(r.Content, "include: ") {
			t.Errorf("%s roh ohne Include-Zeile: %q", file, r.Content)
		}
	}
	// Die README zählt nicht als Fall, ist aber lesbar; das Verzeichnis mit
	// „/“ wird nicht zusammengebaut.
	mustResolveDoc(t, layers, base+"README.md")
	expectDocError(t, layers, base, DocCodeInvalidInput)
}
