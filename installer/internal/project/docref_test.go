package project

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
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

func TestDocRefNichtUnterstuetzt(t *testing.T) {
	layers, _, _ := docTree(t, map[string]string{
		"instructions/always/010-a.md": "## A\n",
		"checks/check_x.sh":            "#!/bin/sh\n",
	}, nil)

	expectDocError(t, layers, "project:/scripts/123.sh", DocCodeUnsupported)
	expectDocError(t, layers, "keph:/manual/abc.md", DocCodeUnsupported)
	// Außerhalb der Sorten aus docRefKinds abgelehnt, auch wenn es die Datei
	// gibt: checks sind Skripte, kein Lesestoff.
	expectDocError(t, layers, "k-playbook:/checks/check_x.sh", DocCodeUnsupported)
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

// Dieselbe Sorte darf nicht über zwei Wege verschieden herauskommen: Name,
// Herkunft und Abschalten eines Eintrags sind über k_playbook_doc_read
// (ResolveDocRef) dieselben wie unter catalogs der Kontextausgabe, und der
// Inhalt hinter dem Verweis ist die Datei, die der Katalog nennt.
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
}

// Die mitgelieferte Ebene liest instructions/ aus defaults/, die übrigen
// Sorten von oben — es gibt nur eine Fassung der Anweisungen, keine Kopie.
func TestShippedDocSourceLiestInstructionsAusDefaults(t *testing.T) {
	playbook := t.TempDir()
	for rel, content := range map[string]string{
		"defaults/instructions/always/010-a.md": "## A\n",
		"defaults/instructions/on-demand/x.md":  "## X\n",
		"instructions/always/999-oben.md":       "## Oben\n",
		"rules/befunde.md":                      "# Regel\n",
		"defaults/rules/befunde.md":             "# Regel aus defaults\n",
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
}
