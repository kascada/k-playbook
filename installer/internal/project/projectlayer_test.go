package project

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
	"github.com/kascada/k-playbook/installer/internal/kephalaion/kephtest"
	"github.com/kascada/k-playbook/installer/internal/versionsources"
)

// projectLayerStore baut die Collection eines Projekts im Speicher: die
// übergebenen Dateien unter k-playbook/, dazu ein Dokument außerhalb, das die
// Ebene nie sehen darf.
func projectLayerStore(files map[string]string) *kephtest.Store {
	store := kephtest.New()
	store.Put("docs/manual/thema.md", "# Thema\n\nText.\n")
	for rel, content := range files {
		store.Put(ProjectLayerDirName+"/"+rel, content)
	}
	return store
}

// projectLayers sind die Ebenen eines Projekts mit beiden Collections im
// Speicher: das Mitgelieferte aus dist, die projekteigene Ebene aus project.
func projectLayers(t *testing.T, dist *kephtest.Store, project DefaultsStore) (DocLayers, string) {
	t.Helper()
	root := t.TempDir()
	calls := 0
	open := func() (DefaultsStore, string, error) { return project, "vm:projekt", nil }
	return DocLayers{
		Dist:  NewKephDefaultsSource(openStore(dist, &calls)),
		Local: newProjectLocalDocSource(root, NewKephProjectLayerSource(open)),
	}, root
}

// Ersetzen, abschalten, ergänzen — für alle drei Sorten gleich, mit dem Ort
// in der Collection als keph:-Verweis.
func TestProjectLayerUeberlagertWieJedeEbene(t *testing.T) {
	dist := defaultsStore(map[string]string{
		"instructions/on-demand/muster.md":  "---\ntitle: Muster\n---\n\nMitgeliefert.\n",
		"instructions/on-demand/release.md": "Release mitgeliefert.\n",
		"includes/baustein.md":              "Baustein mitgeliefert.\n",
		"reviews/review-code.md":            "# Code\n\nMitgeliefert.\n",
		"reviews/review-tech.md":            "# Tech\n\nMitgeliefert.\n",
	})
	project := projectLayerStore(map[string]string{
		"instructions/on-demand/muster.md":  "---\ntitle: Muster eigen\n---\n\nEigen.\n",
		"instructions/on-demand/release.md": "",
		"instructions/on-demand/eigen.md":   "Nur eigen.\n",
		"includes/baustein.md":              "Baustein eigen.\n",
		"reviews/review-tech.md":            "",
		"reviews/review-eigen.md":           "# Eigen\n\nText.\n",
	})
	layers, _ := projectLayers(t, dist, project)

	cases := []struct {
		ref, content, origin, location string
		disabled                       bool
	}{
		{"k-playbook:/instructions/on-demand/muster.md", "---\ntitle: Muster eigen\n---\n\nEigen.\n", DocOriginOverride,
			"keph:/k-playbook/instructions/on-demand/muster.md", false},
		{"k-playbook:/instructions/on-demand/release.md", "", DocOriginOverride,
			"keph:/k-playbook/instructions/on-demand/release.md", true},
		{"k-playbook:/instructions/on-demand/eigen.md", "Nur eigen.\n", DocOriginLocal,
			"keph:/k-playbook/instructions/on-demand/eigen.md", false},
		{"k-playbook:/includes/baustein.md", "Baustein eigen.\n", DocOriginOverride, "keph:/k-playbook/includes/baustein.md", false},
		{"k-playbook:/reviews/review-code.md", "# Code\n\nMitgeliefert.\n", DocOriginDist, "", false},
		{"k-playbook:/reviews/review-tech.md", "", DocOriginOverride, "keph:/k-playbook/reviews/review-tech.md", true},
		{"k-playbook:/reviews/review-eigen.md", "# Eigen\n\nText.\n", DocOriginLocal, "keph:/k-playbook/reviews/review-eigen.md", false},
	}
	for _, c := range cases {
		result, err := ResolveDocRef(layers, c.ref)
		if err != nil {
			t.Errorf("%s: %v", c.ref, err)
			continue
		}
		part := result.Parts[0]
		if result.Content != c.content || part.Origin != c.origin || part.Location != c.location || result.Disabled != c.disabled {
			t.Errorf("%s: Inhalt %q, Herkunft %s, Ort %q, abgeschaltet %v; erwartet %q, %s, %q, %v",
				c.ref, result.Content, part.Origin, part.Location, result.Disabled, c.content, c.origin, c.location, c.disabled)
		}
		if c.disabled && !strings.Contains(result.Hint, c.location) {
			t.Errorf("%s: Hinweis nennt den Ort nicht: %q", c.ref, result.Hint)
		}
	}

	// Ein Include auf einen projekteigenen Baustein nennt dessen Ort.
	project.Put(ProjectLayerDirName+"/instructions/on-demand/mit-include.md", "Vorn.\n\ninclude: k-playbook:/includes/baustein.md\n")
	layers, _ = projectLayers(t, dist, project)
	result, err := ResolveDocRef(layers, "k-playbook:/instructions/on-demand/mit-include.md")
	if err != nil {
		t.Fatalf("mit-include.md: %v", err)
	}
	if !strings.Contains(result.Content, "Baustein eigen.") || len(result.Includes) != 1 ||
		result.Includes[0].Location != "keph:/k-playbook/includes/baustein.md" {
		t.Errorf("Include: %q %+v", result.Content, result.Includes)
	}
}

// always/ wird aus beiden Ebenen zusammengebaut, numerisch sortiert über die
// Ebenen hinweg, je Teil mit Herkunft und Ort.
func TestProjectLayerBautAlwaysZusammen(t *testing.T) {
	dist := defaultsStore(map[string]string{
		"instructions/always/010-a.md":  "## A\n",
		"instructions/always/100-b.md":  "## B\n",
		"instructions/always/200-c.md":  "## C\n",
		"instructions/always/README.md": "# Liesmich\n",
	})
	project := projectLayerStore(map[string]string{
		"instructions/always/100-b.md":  "## B eigen\n\nText.\n",
		"instructions/always/200-c.md":  "",
		"instructions/always/20-x.md":   "## X eigen\n\nText.\n",
		"instructions/always/1000-y.md": "## Y eigen\n\nText.\n",
	})
	layers, _ := projectLayers(t, dist, project)

	result, err := ResolveDocRef(layers, "k-playbook:/instructions/always/")
	if err != nil {
		t.Fatalf("always/: %v", err)
	}
	if want := "## A\n\n## X eigen\n\nText.\n\n## B eigen\n\nText.\n\n## Y eigen\n\nText.\n"; result.Content != want {
		t.Errorf("Inhalt:\n%q\nerwartet\n%q", result.Content, want)
	}
	want := []DocPart{
		{Name: "010-a.md", Origin: DocOriginDist},
		{Name: "20-x.md", Origin: DocOriginLocal, Location: "keph:/k-playbook/instructions/always/20-x.md"},
		{Name: "100-b.md", Origin: DocOriginOverride, Location: "keph:/k-playbook/instructions/always/100-b.md"},
		{Name: "200-c.md", Origin: DocOriginOverride, Location: "keph:/k-playbook/instructions/always/200-c.md", Disabled: true},
		{Name: "1000-y.md", Origin: DocOriginLocal, Location: "keph:/k-playbook/instructions/always/1000-y.md"},
	}
	if !reflect.DeepEqual(result.Parts, want) {
		t.Errorf("Teile:\n%+v\nerwartet\n%+v", result.Parts, want)
	}
}

// Die Quellenkonfiguration des Versionsinventars liegt an der Wurzel von
// keph:/k-playbook/ (versionsources.DocName, Task 144), neben den Sorten,
// aber in keiner: Die Überlagerung greift sie weder als Sorte noch als Datei
// der projekteigenen Ebene auf.
func TestProjectLayerGreiftInventoryMdNichtAuf(t *testing.T) {
	dist := defaultsStore(map[string]string{
		"instructions/always/010-a.md":     "## A\n",
		"instructions/on-demand/muster.md": "Muster.\n",
		"reviews/review-code.md":           "# Code\n",
	})
	project := projectLayerStore(map[string]string{
		"inventory.md": "---\nschema_version: 3\nexclude:\n  - \"**/testdata/**\"\n---\n\nQuellen.\n",
	})
	if _, ok := project.Content(versionsources.DocName); !ok {
		t.Fatalf("Testaufbau: %s fehlt im Store: %v", versionsources.DocName, project.Names())
	}
	layers, root := projectLayers(t, dist, project)

	always, err := ResolveDocRef(layers, "k-playbook:/instructions/always/")
	if err != nil || always.Content != "## A\n" || len(always.Parts) != 1 {
		t.Errorf("always/: %+v, %v", always, err)
	}
	if _, err := ResolveDocRef(layers, "k-playbook:/inventory.md"); err == nil {
		t.Error("k-playbook:/inventory.md darf nicht aufgelöst werden — es ist keine Sorte und kein Teil der Ebene")
	}
	for _, kind := range catalogKinds() {
		entries, err := resolveCatalog(root, layers, kind)
		if err != nil {
			t.Errorf("Katalog %s: %v", kind.name, err)
			continue
		}
		for _, entry := range entries {
			if strings.Contains(entry.Name, "inventory") || strings.Contains(entry.Path, "inventory") {
				t.Errorf("Katalog %s greift inventory.md auf: %+v", kind.name, entry)
			}
		}
	}
	if isProjectLayerKind("inventory.md") || isProjectLayerKind("/inventory.md") {
		t.Error("inventory.md ist keine Sorte der projekteigenen Ebene")
	}
}

// Fehlt k-playbook/ in der Collection, ist die Ebene leer, ohne Fehler: es
// gilt das Mitgelieferte.
func TestProjectLayerOhneVerzeichnisIstLeer(t *testing.T) {
	dist := defaultsStore(map[string]string{
		"instructions/always/010-a.md":     "## A\n",
		"instructions/on-demand/muster.md": "Muster.\n",
	})
	layers, _ := projectLayers(t, dist, projectLayerStore(nil))

	always, err := ResolveDocRef(layers, "k-playbook:/instructions/always/")
	if err != nil || always.Content != "## A\n" {
		t.Errorf("always/: %+v, %v", always, err)
	}
	muster, err := ResolveDocRef(layers, "k-playbook:/instructions/on-demand/muster.md")
	if err != nil || muster.Parts[0].Origin != DocOriginDist {
		t.Errorf("muster.md: %+v, %v", muster, err)
	}
	if _, err := ResolveDocRef(layers, "k-playbook:/reviews/review-gibtsnicht.md"); DocRefErrorCode(err) != DocCodeNotFound {
		t.Errorf("unbekanntes Rezept: %v, erwartet not_found", err)
	}
}

// Ein Fehler der Collection ist kephalaion_unavailable — beim Öffnen, beim
// Auflisten und beim Lesen —, kein stilles „nicht vorhanden“.
func TestProjectLayerFehlerIstNichtErreichbar(t *testing.T) {
	dist := defaultsStore(map[string]string{"instructions/on-demand/muster.md": "Muster.\n"})

	listFails := projectLayerStore(map[string]string{"instructions/on-demand/eigen.md": "Eigen.\n"})
	listFails.Fail["list"] = &kephalaion.ToolError{Tool: "list", Message: "vm:projekt nicht lesbar"}
	readFails := projectLayerStore(map[string]string{"instructions/on-demand/eigen.md": "Eigen.\n"})
	readFails.Fail["read"] = &kephalaion.ToolError{Tool: "read", Code: kephalaion.CodeNodeUnreachable, Message: "weg"}

	for name, local := range map[string]DocSource{
		"öffnen": NewKephProjectLayerSource(func() (DefaultsStore, string, error) {
			return nil, "vm:projekt", docRefErrorf(DocCodeKephalaionUnavailable, "nicht bereit")
		}),
		"auflisten": NewKephProjectLayerSource(func() (DefaultsStore, string, error) { return listFails, "vm:projekt", nil }),
		"lesen":     NewKephProjectLayerSource(func() (DefaultsStore, string, error) { return readFails, "vm:projekt", nil }),
	} {
		calls := 0
		layers := DocLayers{Dist: NewKephDefaultsSource(openStore(dist, &calls)), Local: newProjectLocalDocSource(t.TempDir(), local)}
		ref := "k-playbook:/instructions/on-demand/eigen.md"
		if name != "lesen" {
			// Auch ein Verweis, den nur das Mitgelieferte hat, fällt nicht
			// auf das Mitgelieferte allein zurück.
			ref = "k-playbook:/instructions/on-demand/muster.md"
		}
		_, err := ResolveDocRef(layers, ref)
		if DocRefErrorCode(err) != DocCodeKephalaionUnavailable {
			t.Errorf("%s: %v, erwartet kephalaion_unavailable", name, err)
			continue
		}
		if name != "lesen" && !strings.Contains(err.Error(), "keph:/k-playbook/") {
			t.Errorf("%s: Meldung nennt den Ort nicht: %v", name, err)
		}
		if strings.Contains(err.Error(), "fetch_release") {
			t.Errorf("%s: fetch_release gilt nur für das Mitgelieferte: %v", name, err)
		}
	}
}

// <hub>:k-playbook ist lesbar, die Collection des Projekts nicht: Briefing
// und read scheitern mit kephalaion_unavailable, statt das Mitgelieferte
// allein zu liefern; die Kataloge der Kontextausgabe bleiben leer und nennen
// den Grund.
func TestProjectLayerHubLesbarProjektNicht(t *testing.T) {
	root := newBriefingProject(t)
	stubBriefingDocs(t, unavailableDocs)
	stubProjectLayerStore(t, func(string) (DefaultsStore, string, error) {
		store := kephtest.New()
		store.Fail["list"] = &kephalaion.ToolError{Tool: "list", Message: "keine Rechte an test:projekt"}
		return store, testProjectAddress, nil
	})

	if _, err := BuildBriefing(context.Background(), root); DocRefErrorCode(err) != DocCodeKephalaionUnavailable {
		t.Errorf("BuildBriefing: %v, erwartet kephalaion_unavailable", err)
	}
	if _, err := ResolveDocRef(ProjectDocLayers(root), "k-playbook:/instructions/always/"); DocRefErrorCode(err) != DocCodeKephalaionUnavailable {
		t.Errorf("always/: %v, erwartet kephalaion_unavailable", err)
	}
	if _, err := ResolveDocRef(ProjectDocLayers(root), "k-playbook:/instructions/on-demand/muster.md"); DocRefErrorCode(err) != DocCodeKephalaionUnavailable {
		t.Errorf("muster.md: %v, erwartet kephalaion_unavailable", err)
	}
	built, err := BuildContext(root)
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}
	if len(built.Catalogs["reviews"]) != 0 || !strings.Contains(built.CatalogsProblem, "keine Rechte") {
		t.Errorf("Katalog reviews %+v, Problem %q", built.Catalogs["reviews"], built.CatalogsProblem)
	}
}

// Nur instructions/, includes/ und reviews/ kommen aus der Collection;
// commands/ bleibt in .k-playbook/commands/, guidelines/ in
// k-playbook-local/. Der frühere Ort k-playbook-local/instructions/,
// includes/ und reviews/ wird nicht mehr gelesen.
func TestProjectLocalDocSourceWaehltDenOrt(t *testing.T) {
	root := t.TempDir()
	writeDocTree(t, root, map[string]string{
		".k-playbook/commands/k-eigen.md":                  "Command.\n",
		"k-playbook-local/guidelines/stil.md":              "Stil.\n",
		"k-playbook-local/instructions/on-demand/alt.md":   "Alt.\n",
		"k-playbook-local/includes/alt.md":                 "Alt.\n",
		"k-playbook-local/reviews/review-alt.md":           "Alt.\n",
		"test-collection/k-playbook/reviews/review-neu.md": "Neu.\n",
		"test-collection/k-playbook/includes/neu.md":       "Neu.\n",
		"test-collection/k-playbook/commands/k-eigen.md":   "falsch\n",
		"test-collection/k-playbook/guidelines/stil.md":    "falsch\n",
	})
	store, err := kephtest.Collection(filepath.Join(root, testCollectionDir))
	if err != nil {
		t.Fatal(err)
	}
	source := newProjectLocalDocSource(root, NewKephProjectLayerSource(func() (DefaultsStore, string, error) {
		return store, testProjectAddress, nil
	}))

	for file, want := range map[string]string{
		"commands/k-eigen.md":           "Command.\n",
		"guidelines/stil.md":            "Stil.\n",
		"reviews/review-neu.md":         "Neu.\n",
		"includes/neu.md":               "Neu.\n",
		"reviews/review-alt.md":         "",
		"includes/alt.md":               "",
		"instructions/on-demand/alt.md": "",
	} {
		data, err := source.Read(file)
		switch {
		case want == "" && err == nil:
			t.Errorf("%s: vom alten Ort gelesen: %q", file, data)
		case want != "" && (err != nil || string(data) != want):
			t.Errorf("%s: %q, %v; erwartet %q", file, data, err, want)
		}
	}
	locator := source.(DocLocator)
	for file, want := range map[string]string{
		"reviews/review-neu.md": "keph:/k-playbook/reviews/review-neu.md",
		"commands/k-eigen.md":   "",
		"guidelines/stil.md":    "",
	} {
		if got := locator.Locate(file); got != want {
			t.Errorf("Locate(%s) = %q, erwartet %q", file, got, want)
		}
	}
}
