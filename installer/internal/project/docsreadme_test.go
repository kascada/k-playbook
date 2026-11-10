package project

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
	"github.com/kascada/k-playbook/installer/internal/kephalaion/kephtest"
	"github.com/kascada/k-playbook/installer/internal/yamllite"
)

// docs/ ist eine Sorte nur der mitgelieferten Ebene (Task 137): die Vorlage
// kommt mit Frontmatter und Herkunft dist; eine lokale Datei gleichen Namens
// gewinnt nicht, eine leere schaltet nicht ab, eine nur lokale gibt es nicht.
// Mit „/“ am Ende fehlt der Dateiname.
func TestDocRefSorteDocsNurMitgeliefert(t *testing.T) {
	const vorlage = "---\ntitle: \"Code\"\ndescription: \"Aus dem Code.\"\n---\n\n# Code\n"
	layers, _, _ := docTree(t, map[string]string{
		"docs/generated/code/README.md": vorlage,
		"docs/manual/README.md":         "---\ntitle: \"Manual\"\ndescription: \"Von Hand.\"\n---\n",
	}, map[string]string{
		"docs/generated/code/README.md": "---\ntitle: lokal\n---\n",
		"docs/manual/README.md":         "",
		"docs/eigen/README.md":          "---\ntitle: eigen\n---\n",
	})

	got := mustResolveDoc(t, layers, "k-playbook:/docs/generated/code/README.md")
	if got.Content != vorlage || got.Disabled || !reflect.DeepEqual(got.Parts, []DocPart{{Name: "README.md", Origin: DocOriginDist}}) {
		t.Errorf("Vorlage: %+v", got)
	}
	manual := mustResolveDoc(t, layers, "k-playbook:/docs/manual/README.md")
	if manual.Disabled || manual.Parts[0].Origin != DocOriginDist || !strings.Contains(manual.Content, "Von Hand.") {
		t.Errorf("leere lokale Datei schaltet ab: %+v", manual)
	}
	err := expectDocError(t, layers, "k-playbook:/docs/eigen/README.md", DocCodeNotFound)
	if !strings.Contains(err.Error(), "mitgeliefert nicht vorhanden") {
		t.Errorf("nur lokal: %v", err)
	}
	expectDocError(t, layers, "k-playbook:/docs/", DocCodeInvalidInput)
	expectDocError(t, layers, "k-playbook:/docs/generated/", DocCodeInvalidInput)

	// Kein Katalog, keine Lesersorte: docs/ steht in keinem Briefing.
	if slices.Contains(docReaderKinds, docDocsRoot) {
		t.Error("docs/ ist Lesersorte")
	}
	for _, kind := range catalogKinds() {
		if kind.name == docDocsRoot {
			t.Error("docs/ im Katalog")
		}
	}
	// Die Meldung zu unsupported nennt nur die Lesersorten.
	err = expectDocError(t, layers, "k-playbook:/checks/check_x.sh", DocCodeUnsupported)
	if strings.Contains(err.Error(), "docs/") {
		t.Errorf("docs/ in der Meldung: %v", err)
	}
}

// Über die Ebenen eines Projekts: eine Datei unter k-playbook-local/docs/
// gewinnt nicht; die Vorlage kommt aus <hub>:k-playbook/defaults/docs/.
func TestDocRefDocsLokaleDateiGewinntNicht(t *testing.T) {
	root := t.TempDir()
	writeDocTree(t, filepath.Join(root, PlaybookDirName, defaultsDirName), map[string]string{
		"docs/manual/README.md": "---\ntitle: \"Manual\"\n---\n\nmitgeliefert\n",
	})
	writeDocTree(t, LocalDir(root), map[string]string{
		"docs/manual/README.md": "---\ntitle: lokal\n---\n\nlokal\n",
	})
	got := mustResolveDoc(t, ProjectDocLayers(root), "k-playbook:/docs/manual/README.md")
	if !strings.Contains(got.Content, "mitgeliefert") || got.Parts[0].Origin != DocOriginDist {
		t.Errorf("lokale Datei gewinnt: %+v", got)
	}
}

// ListDocsTemplates nennt die Verzeichnisse mit README.md unter docs/,
// sortiert; docs/README.md oben und andere Dateien zählen nicht. Ohne docs/
// ist die Liste leer, kein Fehler.
func TestListDocsTemplates(t *testing.T) {
	layers, _, _ := docTree(t, map[string]string{
		"docs/README.md":                "# oben\n",
		"docs/generated/README.md":      "x",
		"docs/generated/code/README.md": "x",
		"docs/generated/code/notiz.md":  "x",
		"docs/manual/README.md":         "x",
		"docs/leer/sub/README.md":       "x",
		"docs/ohne/notiz.md":            "x",
	}, nil)
	got, err := ListDocsTemplates(layers.Dist)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"generated", "generated/code", "leer/sub", "manual"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Vorlagen: %v, erwartet %v", got, want)
	}

	empty, _, _ := docTree(t, map[string]string{"reviews/review-x.md": "x"}, nil)
	got, err = ListDocsTemplates(empty.Dist)
	if err != nil || got == nil || len(got) != 0 {
		t.Errorf("ohne docs/: %v, %v", got, err)
	}

	if _, err := ListDocsTemplates(failingDocSource{}); err == nil {
		t.Error("Fehler der Ebene verschluckt")
	}
}

// Das Argument von docs readme: Pfad relativ zu docs/ oder ein Verweis auf
// Verzeichnis oder README; docs/ selbst und docs/README.md sind ungültig.
func TestCleanDocsDir(t *testing.T) {
	for arg, want := range map[string]string{
		"generated/code":                           "generated/code",
		"generated/code/":                          "generated/code",
		"manual":                                   "manual",
		"keph:/docs/generated/code/":               "generated/code",
		"keph:/docs/generated/code/README.md":      "generated/code",
		"k-playbook:/docs/sources/README.md":       "sources",
		"  k-playbook:/docs/generated/versions/  ": "generated/versions",
		"generated/code/README.md":                 "generated/code",
	} {
		got, err := CleanDocsDir(arg)
		if err != nil || got != want {
			t.Errorf("%q: %q, %v — erwartet %q", arg, got, err, want)
		}
	}
	for _, arg := range []string{"", "/", "README.md", "keph:/docs/", "keph:/docs/README.md", "keph:/manual/x/",
		"k-playbook:/docsx/a/", "../x", "generated/../x", "project:/docs/manual/"} {
		if got, err := CleanDocsDir(arg); err == nil {
			t.Errorf("%q gilt als gültig: %q", arg, got)
		} else if DocRefErrorCode(err) != DocCodeInvalidInput {
			t.Errorf("%q: Code %q (%v)", arg, DocRefErrorCode(err), err)
		}
	}
}

// Die Vorlagen im Repo (Task 137): unter defaults/docs/ liegen nur README.md,
// keine oben unter docs/; jede trägt title und description und kein
// generated (daran erkennt Kephalaion seine Mahnung). Lesbar über
// k-playbook:/docs/<pfad>/README.md mit dem Inhalt der Datei.
func TestDefaultsDocsVorlagen(t *testing.T) {
	repo, err := filepath.Abs(filepath.Join("..", "..", "..", "defaults", "docs"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(repo, "README.md")); !os.IsNotExist(err) {
		t.Errorf("defaults/docs/README.md darf es nicht geben: docs/README.md ist Kephalaions (%v)", err)
	}
	err = filepath.WalkDir(repo, func(file string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		if entry.Name() != "README.md" {
			t.Errorf("%s: unter defaults/docs/ liegen nur Vorlagen README.md", file)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	layers := repoDefaultsLayers(t)
	dirs, err := ListDocsTemplates(layers.Dist)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"generated", "generated/code", "generated/libs", "generated/versions", "manual", "sources"}
	if !reflect.DeepEqual(dirs, want) {
		t.Errorf("Vorlagen: %v, erwartet %v", dirs, want)
	}
	for _, dir := range dirs {
		ref := DocsTemplateRef(dir)
		got := mustResolveDoc(t, layers, ref)
		if got.Content != readRepoDefault(t, "docs/"+dir+"/README.md") || len(got.Includes) != 0 {
			t.Errorf("%s: Inhalt weicht von der Datei ab", ref)
		}
		meta, problems := parseDocsMeta([]byte(got.Content))
		if len(problems) != 0 || meta.Title == "" || meta.Description == "" {
			t.Errorf("%s: Kopf %+v, Probleme %v", ref, meta, problems)
		}
		block, _ := frontmatterOf(got.Content)
		root, err := yamllite.Parse([]byte(block))
		if err != nil {
			t.Fatalf("%s: %v", ref, err)
		}
		if root.Get("generated") != nil {
			t.Errorf("%s trägt generated — Kephalaion hielte sie für seine Mahnung", ref)
		}
		if strings.ContainsAny(meta.Description, "\n") || strings.Count(strings.TrimSuffix(meta.Description, "."), ". ") > 0 {
			t.Errorf("%s: description ist nicht ein Satz: %q", ref, meta.Description)
		}
		if !strings.Contains(got.Content, "`"+ref+"`") {
			t.Errorf("%s nennt ihre eigene Vorlage nicht", ref)
		}
	}
}

// frontmatterOf ist der Frontmatter-Block eines Dokuments.
func frontmatterOf(content string) (string, bool) {
	lines := strings.Split(content, "\n")
	n := frontmatterLines(lines)
	if n == 0 {
		return "", false
	}
	return strings.Join(lines[1:n-1], "\n"), true
}

// docsReadmeFixture ist eine Collection im Speicher und die Vorlagen des
// Repos als mitgelieferte Ebene.
func docsReadmeFixture(t *testing.T) (*kephtest.Store, *DocsReadmes) {
	t.Helper()
	store := kephtest.New()
	return store, NewDocsReadmes(store, repoDefaultsSource(t))
}

// statusesOf sind die Zustände je Verzeichnis.
func statusesOf(result DocsReadmeResult) map[string]string {
	got := map[string]string{}
	for _, item := range result.Readmes {
		got[item.Dir] = item.Status
	}
	return got
}

// Alle Vorlagen: eine fehlende README wird angelegt, eine gleiche bleibt, eine
// von Hand geänderte und Kephalaions Mahnung (generated) werden mit ihrer
// Revision überschrieben. docs/README.md oben und ein Verzeichnis ohne
// Vorlage bleiben unberührt. Ein zweiter Lauf schreibt nichts.
func TestDocsReadmesAlle(t *testing.T) {
	store, readmes := docsReadmeFixture(t)
	code := readRepoDefault(t, "docs/generated/code/README.md")
	store.Put("docs/README.md", "# Kephalaions Anleitung\n")
	store.Put("docs/generated/code/README.md", code)
	store.Put("docs/manual/README.md", "---\ntitle: Von Hand\ndescription: geändert\n---\n")
	store.Put("docs/sources/README.md", "---\ntitle: sources\ndescription: fehlt noch\ngenerated: { by: kephalaion, at: 2026-11-10T10:00:00Z }\n---\n")
	store.Put("docs/manual/sub/README.md", "---\ntitle: Unterordner\n---\n")
	manualRev := store.Docs["docs/manual/README.md"].Revision
	sourcesRev := store.Docs["docs/sources/README.md"].Revision
	calls := len(store.Calls)

	result, err := readmes.All(context.Background(), DocsReadmeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"generated": DocsCreated, "generated/code": DocsUnchanged, "generated/libs": DocsCreated,
		"generated/versions": DocsCreated, "manual": DocsWritten, "sources": DocsWritten}
	if got := statusesOf(result); !reflect.DeepEqual(got, want) {
		t.Errorf("Zustände: %v", got)
	}
	if got := writeCalls(store, calls); !reflect.DeepEqual(got, []string{
		"create docs/generated/README.md",
		"create docs/generated/libs/README.md",
		"create docs/generated/versions/README.md",
		fmt.Sprintf("write docs/manual/README.md @%d", manualRev),
		fmt.Sprintf("write docs/sources/README.md @%d", sourcesRev),
	}) {
		t.Errorf("Aufrufe: %v", got)
	}
	for _, item := range result.Readmes {
		if item.Ref != "keph:/docs/"+item.Dir+"/README.md" || item.Template != "k-playbook:/docs/"+item.Dir+"/README.md" || item.Revision == 0 {
			t.Errorf("Eintrag: %+v", item)
		}
		if content, _ := store.Content("docs/" + item.Dir + "/README.md"); content != readRepoDefault(t, "docs/"+item.Dir+"/README.md") {
			t.Errorf("%s: Inhalt ist nicht die Vorlage: %q", item.Dir, content)
		}
		if item.Dir == "manual" && item.Base != manualRev {
			t.Errorf("manual: base %d, erwartet %d", item.Base, manualRev)
		}
	}
	if content, _ := store.Content("docs/README.md"); content != "# Kephalaions Anleitung\n" {
		t.Errorf("docs/README.md angefasst: %q", content)
	}
	if content, _ := store.Content("docs/manual/sub/README.md"); content != "---\ntitle: Unterordner\n---\n" {
		t.Errorf("Verzeichnis ohne Vorlage angefasst: %q", content)
	}
	if result.Failed() != 0 || result.Hint != "" || result.Check {
		t.Errorf("Antwort: %+v", result)
	}

	calls = len(store.Calls)
	again, err := readmes.All(context.Background(), DocsReadmeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got := writeCalls(store, calls); len(got) != 0 {
		t.Errorf("zweiter Lauf schreibt: %v", got)
	}
	for _, item := range again.Readmes {
		if item.Status != DocsUnchanged {
			t.Errorf("zweiter Lauf: %+v", item)
		}
	}
}

// Check schreibt nichts und meldet missing, differs und unchanged.
func TestDocsReadmesCheck(t *testing.T) {
	store, readmes := docsReadmeFixture(t)
	store.Put("docs/generated/code/README.md", readRepoDefault(t, "docs/generated/code/README.md"))
	store.Put("docs/manual/README.md", "---\ntitle: alt\n---\n")
	calls := len(store.Calls)

	result, err := readmes.All(context.Background(), DocsReadmeOptions{Check: true})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"generated": DocsReadmeMissing, "generated/code": DocsUnchanged, "generated/libs": DocsReadmeMissing,
		"generated/versions": DocsReadmeMissing, "manual": DocsReadmeDiffers, "sources": DocsReadmeMissing}
	if got := statusesOf(result); !reflect.DeepEqual(got, want) || !result.Check {
		t.Errorf("Zustände: %v (check %v)", got, result.Check)
	}
	if got := writeCalls(store, calls); len(got) != 0 {
		t.Errorf("Check schreibt: %v", got)
	}
}

// Eine einzelne Vorlage; fehlt sie, ist das template_missing und nichts wird
// geschrieben, auch keine leere README. Eine Vorlage mit generated oder ohne
// description ist unbrauchbar (invalid_input).
func TestDocsReadmesEineUndFehlendeVorlage(t *testing.T) {
	store := kephtest.New()
	layers, _, _ := docTree(t, map[string]string{
		"docs/manual/README.md":    "---\ntitle: \"Manual\"\ndescription: \"By hand.\"\n---\n\n# Manual\n",
		"docs/generiert/README.md": "---\ntitle: \"G\"\ndescription: \"D.\"\ngenerated: { by: x, at: y }\n---\n",
		"docs/kopflos/README.md":   "# ohne Kopf\n",
		"docs/leer/README.md":      "",
	}, nil)
	readmes := NewDocsReadmes(store, layers.Dist)
	ctx := context.Background()

	item := readmes.One(ctx, "manual", DocsReadmeOptions{})
	if item.Status != DocsCreated || item.Revision == 0 {
		t.Errorf("manual: %+v", item)
	}
	for dir, code := range map[string]string{
		"generated/code": DocCodeTemplateMissing,
		"generiert":      DocCodeInvalidInput,
		"kopflos":        DocCodeInvalidInput,
		"leer":           DocCodeInvalidInput,
	} {
		calls := len(store.Calls)
		item := readmes.One(ctx, dir, DocsReadmeOptions{})
		if item.Status != DocsFailed || item.Code != code || item.Message == "" {
			t.Errorf("%s: %+v, erwartet failed mit %s", dir, item, code)
		}
		if got := writeCalls(store, calls); len(got) != 0 {
			t.Errorf("%s: geschrieben %v", dir, got)
		}
		if _, ok := store.Content("docs/" + dir + "/README.md"); ok {
			t.Errorf("%s: README angelegt", dir)
		}
	}
}

// Ohne Vorlagen auf dem Hub: nichts zu tun, mit Hinweis. Lassen sich die
// Vorlagen nicht auflisten, ist das der Fehler. Scheitert eine README am
// Store, laufen die übrigen weiter.
func TestDocsReadmesOhneVorlagenUndFehler(t *testing.T) {
	store := kephtest.New()
	empty, _, _ := docTree(t, map[string]string{"reviews/review-x.md": "x"}, nil)
	result, err := NewDocsReadmes(store, empty.Dist).All(context.Background(), DocsReadmeOptions{})
	if err != nil || len(result.Readmes) != 0 || !strings.Contains(result.Hint, "keine Vorlagen") {
		t.Errorf("ohne Vorlagen: %+v, %v", result, err)
	}
	if _, err := NewDocsReadmes(store, failingDocSource{}).All(context.Background(), DocsReadmeOptions{}); err == nil {
		t.Error("Fehler beim Auflisten verschluckt")
	}

	store, readmes := docsReadmeFixture(t)
	store.Put("docs/manual/README.md", "---\ntitle: alt\n---\n")
	store.Fail["create"] = &kephalaion.ToolError{Tool: "create", Code: kephalaion.CodeUnreachable, Message: "Hub weg"}
	result, err = readmes.All(context.Background(), DocsReadmeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	got := statusesOf(result)
	if got["manual"] != DocsWritten || got["generated"] != DocsFailed || result.Failed() != 5 {
		t.Errorf("Zustände: %v, failed %d", got, result.Failed())
	}
	for _, item := range result.Readmes {
		if item.Status == DocsFailed && item.Code != DocCodeKephalaionUnavailable {
			t.Errorf("Code: %+v", item)
		}
	}
}

// Ohne Anbindung kein Zugang: kephalaion_unavailable, kein Rückfall.
func TestOpenDocsReadmesOhneAnbindung(t *testing.T) {
	original := kephalaionCheck
	kephalaionCheck = func(string) kephalaion.Status {
		return kephalaion.Status{State: kephalaion.StateNotConfigured, Message: "Keine Collection eingetragen."}
	}
	t.Cleanup(func() { kephalaionCheck = original })

	readmes, err := OpenDocsReadmes(t.TempDir())
	if readmes != nil || DocRefErrorCode(err) != DocCodeKephalaionUnavailable {
		t.Errorf("ohne Anbindung: %v, %v", readmes, err)
	}
}

// stubSetupDocsReadmes setzt hinter dem README-Schritt des Einrichtens eine
// Collection im Speicher mit den gegebenen Vorlagen ein.
func stubSetupDocsReadmes(t *testing.T, store *kephtest.Store, templates DocSource) {
	t.Helper()
	original := openSetupDocsReadmes
	openSetupDocsReadmes = func(context.Context, string) (*DocsReadmes, error) { return NewDocsReadmes(store, templates), nil }
	t.Cleanup(func() { openSetupDocsReadmes = original })
}

// Das Einrichten schreibt am Ende alle README aus den Vorlagen; das Ergebnis
// steht im eigenen Feld Readmes, nicht im Fehlerwert. Ein zweites Einrichten
// lässt sie unverändert.
func TestApplyAssistantSetupSchreibtReadmes(t *testing.T) {
	store := kephtest.New()
	stubSetupDocsReadmes(t, store, repoDefaultsSource(t))
	root := t.TempDir()
	if err := CreateConfig(root, "."); err != nil {
		t.Fatal(err)
	}

	setup, err := ApplyAssistantSetup(root)
	if err != nil {
		t.Fatalf("ApplyAssistantSetup: %v", err)
	}
	if len(setup.Readmes.Readmes) != 6 || !setup.Readmes.Changed() || setup.Readmes.Hint != "" {
		t.Fatalf("Readmes: %+v", setup.Readmes)
	}
	if text := DescribeSetupReadmes(setup.Readmes); text != "README unter keph:/docs/: 6 angelegt." {
		t.Errorf("Satz: %q", text)
	}
	for _, dir := range []string{"manual", "sources"} {
		if _, ok := store.Content("docs/" + dir + "/README.md"); !ok {
			t.Errorf("README von %s/ fehlt", dir)
		}
	}

	store.Put("docs/manual/README.md", "---\ntitle: alt\n---\n")
	setup, err = ApplyAssistantSetup(root)
	if err != nil {
		t.Fatal(err)
	}
	if text := DescribeSetupReadmes(setup.Readmes); text != "README unter keph:/docs/: 1 aus der Vorlage überschrieben, 5 unverändert." {
		t.Errorf("Satz: %q", text)
	}
}

// Ohne bereites Kephalaion wird nichts versucht: ein Hinweis, kein Fehler des
// Einrichtens. Fehlt eine Vorlage, nennt der Satz die README mit Code.
func TestApplyAssistantSetupReadmesOhneKephalaionUndVorlage(t *testing.T) {
	original := kephalaionCheck
	kephalaionCheck = func(string) kephalaion.Status {
		return kephalaion.Status{State: kephalaion.StateNotConfigured, Message: "Keine Collection eingetragen."}
	}
	t.Cleanup(func() { kephalaionCheck = original })
	root := t.TempDir()
	if err := CreateConfig(root, "."); err != nil {
		t.Fatal(err)
	}
	setup, err := ApplyAssistantSetup(root)
	if err != nil {
		t.Fatalf("ApplyAssistantSetup: %v", err)
	}
	text := DescribeSetupReadmes(setup.Readmes)
	if len(setup.Readmes.Readmes) != 0 || !strings.Contains(text, "nicht versucht") || !strings.Contains(text, "nicht bereit") ||
		strings.HasSuffix(text, "..") {
		t.Errorf("ohne Kephalaion: %+v — %q", setup.Readmes, text)
	}

	store := kephtest.New()
	layers, _, _ := docTree(t, map[string]string{
		"docs/manual/README.md": "---\ntitle: \"Manual\"\ndescription: \"By hand.\"\n---\n",
		"docs/kaputt/README.md": "# ohne Kopf\n",
	}, nil)
	stubSetupDocsReadmes(t, store, layers.Dist)
	setup, err = ApplyAssistantSetup(root)
	if err != nil {
		t.Fatal(err)
	}
	if text := DescribeSetupReadmes(setup.Readmes); text != "README unter keph:/docs/: 1 angelegt; nicht geschrieben: keph:/docs/kaputt/README.md (invalid_input)." {
		t.Errorf("Satz: %q", text)
	}

	empty, _, _ := docTree(t, map[string]string{"reviews/review-x.md": "x"}, nil)
	stubSetupDocsReadmes(t, store, empty.Dist)
	setup, _ = ApplyAssistantSetup(root)
	if text := DescribeSetupReadmes(setup.Readmes); !strings.Contains(text, "keine Vorlagen") {
		t.Errorf("ohne Vorlagen: %q", text)
	}
}

// Ein Hub, der nicht antwortet, hält das Einrichten nur bis zum Zeitlimit
// auf: danach ein Hinweis, die Aufrufe am Node bekommen den abgelaufenen
// Kontext.
func TestApplyAssistantSetupReadmesZeitlimit(t *testing.T) {
	originalTimeout, originalOpen := docsReadmeSetupTimeout, openSetupDocsReadmes
	t.Cleanup(func() { docsReadmeSetupTimeout, openSetupDocsReadmes = originalTimeout, originalOpen })
	docsReadmeSetupTimeout = 50 * time.Millisecond
	released := make(chan struct{})
	openSetupDocsReadmes = func(ctx context.Context, _ string) (*DocsReadmes, error) {
		<-ctx.Done()
		close(released)
		return nil, ctx.Err()
	}
	root := t.TempDir()
	if err := CreateConfig(root, "."); err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	setup, err := ApplyAssistantSetup(root)
	if err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("Einrichten hat %s gedauert", elapsed)
	}
	if !strings.Contains(setup.Readmes.Hint, "abgebrochen") || !strings.Contains(setup.Readmes.Hint, "50ms") {
		t.Errorf("Hinweis: %+v", setup.Readmes)
	}
	select {
	case <-released:
	case <-time.After(5 * time.Second):
		t.Error("der Schritt bekam den abgelaufenen Kontext nicht")
	}
}
