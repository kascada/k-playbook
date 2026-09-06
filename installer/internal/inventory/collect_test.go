package inventory

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const fixtureProject = "testdata/projekte/vollstaendig"

func collectFixture(t *testing.T) Result {
	t.Helper()

	result, err := Collect(Options{ProjectDir: fixtureProject})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	return result
}

// find sucht genau eine Aussage. Zwei Treffer sind ein Fehler des Tests, nicht
// des Sammlers: dann prüft er nicht das, was er zu prüfen glaubt.
func find(t *testing.T, result Result, group string, file string) Entry {
	t.Helper()

	var found []Entry
	for _, entry := range result.Entries {
		if entry.Group == group && entry.SourceFile == file {
			found = append(found, entry)
		}
	}
	if len(found) != 1 {
		t.Fatalf("%s aus %s: %d Treffer, erwartet 1", group, file, len(found))
	}
	return found[0]
}

func TestCollectLiestAlleQuellartenDerFixture(t *testing.T) {
	result := collectFixture(t)

	wanted := []string{
		".devcontainer/devcontainer.json", ".github/workflows/ci.yml", ".python-version",
		"Dockerfile", "charts/app/Chart.lock", "charts/app/Chart.yaml",
		"charts/app/values-prod.yaml", "charts/app/values.yaml", "docker-compose.yml",
		"go.mod", "package.json", "pyproject.toml", "requirements.txt",
	}
	read := map[string]SourceRead{}
	for _, source := range result.Sources {
		read[source.File] = source
	}
	for _, file := range wanted {
		if _, ok := read[file]; !ok {
			t.Errorf("%s wurde nicht ausgewertet", file)
		}
	}
	if len(result.Rejections) != 0 || len(result.Notes) != 0 {
		t.Errorf("die Fixture ist vollständig lesbar: %+v / %+v", result.Rejections, result.Notes)
	}
}

// Jede dargestellte Version trägt eine eindeutige Herkunft: Datei plus
// Abschnitt, und wo die Aussage genau einer Zeile zuzuordnen ist, auch die
// Zeile.
func TestJederEintragTraegtHerkunft(t *testing.T) {
	result := collectFixture(t)

	for _, entry := range result.Entries {
		if entry.SourceFile == "" || entry.SourceKey == "" {
			t.Errorf("Herkunft unvollständig: %+v", entry)
		}
		if entry.Group != entry.Ecosystem+"/"+entry.Name {
			t.Errorf("Gruppenschlüssel = %q, erwartet %q/%q", entry.Group, entry.Ecosystem, entry.Name)
		}
		if entry.Context == "" || entry.ContextOrigin == "" || entry.Pin == "" {
			t.Errorf("Pflichtfeld fehlt: %+v", entry)
		}
		if entry.Pin == PinUnknown && entry.Note == "" {
			t.Errorf("wo unknown steht, gehört ein Hinweis dazu: %+v", entry)
		}
	}
}

func TestKontextlabelKommenAusDerQuellart(t *testing.T) {
	result := collectFixture(t)

	cases := map[string]string{
		"pyproject.toml":                  EnvLokal,
		"go.mod":                          EnvLokal,
		"docker-compose.yml":              EnvDev,
		"Dockerfile":                      EnvDeployment,
		"charts/app/values.yaml":          EnvDeployment,
		".github/workflows/ci.yml":        EnvCI,
		".devcontainer/devcontainer.json": EnvDevcontainer,
	}
	for _, source := range result.Sources {
		want, known := cases[source.File]
		if !known {
			continue
		}
		if source.Env != want {
			t.Errorf("%s trägt %q, erwartet %q", source.File, source.Env, want)
		}
	}
	for _, entry := range result.Entries {
		if entry.ContextOrigin != ContextDefault {
			t.Errorf("ohne Konfiguration ist die Herkunft des Labels `default`: %+v", entry)
		}
	}
}

func TestPythonManifestUndLockfileStehenNebeneinander(t *testing.T) {
	result := collectFixture(t)

	manifest := find(t, result, "python/fastapi", "pyproject.toml")
	if manifest.Version != ">=0.109" || manifest.Pin != PinRange || manifest.Scope != "main" {
		t.Errorf("Manifest = %+v", manifest)
	}
	direct := find(t, result, "python/fastapi", "requirements.txt")
	if direct.Version != "==0.110.0" || direct.Pin != PinExact || direct.VersionNormalized != "0.110.0" {
		t.Errorf("requirements.txt = %+v", direct)
	}
	if direct.SourceLine == 0 {
		t.Errorf("eine Zeile aus requirements.txt ist genau einer Zeile zuzuordnen: %+v", direct)
	}
	build := find(t, result, "python/hatchling", "pyproject.toml")
	if build.Scope != "build" {
		t.Errorf("build-system.requires gehört in den Scope build: %+v", build)
	}
}

func TestGoModuleUndErsetzungen(t *testing.T) {
	result := collectFixture(t)

	module := find(t, result, "go/github.com/spf13/cobra", "go.mod")
	if module.Pin != PinExact || module.Version != "v1.8.0" {
		t.Errorf("require = %+v", module)
	}
	replaced := find(t, result, "go/example.com/lokal", "go.mod")
	if replaced.Pin != PinLocal || !strings.Contains(replaced.Note, "Arbeitsbaum") {
		t.Errorf("replace auf einen Pfad ist local: %+v", replaced)
	}
	runtimeEntry := find(t, result, "runtime/go", "go.mod")
	if runtimeEntry.KindOfThing != ThingRuntime {
		t.Errorf("die go-Direktive ist ein runtime-Eintrag: %+v", runtimeEntry)
	}
	for _, entry := range result.Entries {
		if entry.SourceFile == "go.sum" {
			t.Errorf("go.sum wiederholt go.mod und wird nicht geführt: %+v", entry)
		}
	}
}

func TestContainerQuellenUnterscheidenDigestStageUndVariable(t *testing.T) {
	result := collectFixture(t)

	digest := find(t, result, "container/redis", "Dockerfile")
	if digest.Pin != PinDigest || digest.Digest == "" || digest.Version != "7.2.4" {
		t.Errorf("`@sha256:` macht daraus digest, der Tag bleibt in version: %+v", digest)
	}
	stage := find(t, result, "container/builder", "Dockerfile")
	if stage.Pin != PinLocal || !strings.Contains(stage.Note, "Stage") {
		t.Errorf("FROM auf eine Stage derselben Datei ist local: %+v", stage)
	}
	variable := find(t, result, "container/ghcr.io/example/base", "Dockerfile")
	if variable.Pin != PinUnknown || !strings.Contains(variable.Note, "nicht auflösbar") {
		t.Errorf("nicht auflösbare Variable = %+v", variable)
	}
	fromArg := find(t, result, "container/python", "Dockerfile")
	if fromArg.Version != "3.12-slim" || !strings.Contains(fromArg.Note, "ARG") {
		t.Errorf("ARG mit Default in derselben Datei wird aufgelöst: %+v", fromArg)
	}
	pinned := find(t, result, "python/poetry", "Dockerfile")
	if pinned.Version != "1.8.2" || pinned.KindOfThing != ThingTool {
		t.Errorf("eine wörtlich gepinnte RUN-Zeile wird geführt: %+v", pinned)
	}
	build := find(t, result, "container/app", "docker-compose.yml")
	if build.Pin != PinLocal {
		t.Errorf("services.<name>.build ohne image ist local: %+v", build)
	}
}

func TestDevcontainerFuehrtImageUndFeatures(t *testing.T) {
	result := collectFixture(t)

	image := find(t, result, "container/mcr.microsoft.com/devcontainers/base", ".devcontainer/devcontainer.json")
	if image.KindOfThing != ThingImage || image.Context != EnvDevcontainer {
		t.Errorf("image = %+v", image)
	}
	feature := find(t, result, "container/ghcr.io/devcontainers/features/python", ".devcontainer/devcontainer.json")
	if feature.KindOfThing != ThingTool || feature.Version != "3.12" {
		t.Errorf("die Version steht hinter dem Doppelpunkt: %+v", feature)
	}
	inField := find(t, result, "container/ghcr.io/devcontainers/features/node", ".devcontainer/devcontainer.json")
	if inField.Version != "20.11.1" {
		t.Errorf("die Version aus dem Feld version gewinnt: %+v", inField)
	}
}

func TestHelmChartLockUndValues(t *testing.T) {
	result := collectFixture(t)

	chart := find(t, result, "helm/app", "charts/app/Chart.yaml")
	if chart.KindOfThing != ThingChart || chart.Version != "1.4.0" {
		t.Errorf("Chart.yaml version = %+v", chart)
	}
	app := find(t, result, "runtime/app", "charts/app/Chart.yaml")
	if app.KindOfThing != ThingRuntime || app.Version != "2.3.1" {
		t.Errorf("appVersion ist ein runtime-Eintrag: %+v", app)
	}
	declared := find(t, result, "helm/postgresql", "charts/app/Chart.yaml")
	resolved := find(t, result, "helm/postgresql", "charts/app/Chart.lock")
	if declared.KindOfThing != ThingChartDependency || resolved.KindOfThing != ThingChartDependency {
		t.Errorf("dependencies sind chart-dependency: %+v / %+v", declared, resolved)
	}
	if declared.Version == resolved.Version {
		t.Errorf("die Fixture soll Chart.yaml und Chart.lock auseinanderlaufen lassen")
	}
	repository := find(t, result, "container/ghcr.io/example/app", "charts/app/values.yaml")
	if repository.Version != "1.4.0" {
		t.Errorf("image.repository + image.tag = %+v", repository)
	}
	sidecar := find(t, result, "container/redis", "charts/app/values.yaml")
	if sidecar.SourceKey != "sidecar.image" {
		t.Errorf("ein einzelner image-String zählt auch: %+v", sidecar)
	}
	// Andere Schlüssel werden nicht geraten.
	for _, entry := range result.Entries {
		if entry.SourceFile == "charts/app/values.yaml" && entry.SourceKey == "replicaCount" {
			t.Errorf("replicaCount ist keine Versionsaussage: %+v", entry)
		}
	}
}

func TestCIActionsUndSetupRuntimes(t *testing.T) {
	result := collectFixture(t)

	tag := find(t, result, "ci/actions/checkout", ".github/workflows/ci.yml")
	if tag.Pin != PinExact || tag.KindOfThing != ThingAction {
		t.Errorf("ein Tag ist exact: %+v", tag)
	}
	branch := find(t, result, "ci/actions/setup-node", ".github/workflows/ci.yml")
	if branch.Pin != PinFloating {
		t.Errorf("ein Branch ist floating: %+v", branch)
	}
	sha := find(t, result, "ci/actions/setup-python", ".github/workflows/ci.yml")
	if sha.Pin != PinDigest || sha.Digest == "" {
		t.Errorf("ein voller Commit-SHA ist digest: %+v", sha)
	}
	runtimeEntry := find(t, result, "runtime/python", ".github/workflows/ci.yml")
	if runtimeEntry.Version != "3.11" || runtimeEntry.Context != EnvCI {
		t.Errorf("setup-python liefert zusätzlich einen runtime-Eintrag: %+v", runtimeEntry)
	}
	image := find(t, result, "container/python", ".github/workflows/ci.yml")
	if image.KindOfThing != ThingImage {
		t.Errorf("container: ist ein image-Eintrag: %+v", image)
	}
	// Ein Ausdruck, der erst zur Laufzeit einen Wert bekommt, ist `unknown` —
	// und trägt den Grund bei sich, statt eine Einordnung ohne Begründung zu
	// hinterlassen.
	expression := find(t, result, "runtime/go", ".github/workflows/ci.yml")
	if expression.Pin != PinUnknown {
		t.Errorf("ein GitHub-Ausdruck ist nicht ermittelbar: %+v", expression)
	}
	if !strings.Contains(expression.Note, "nicht auflösbar") {
		t.Errorf("zu jedem unknown gehört ein Grund: %+v", expression)
	}
}

func TestAbweichungenZaehlenGruppenUndTrennenDieArten(t *testing.T) {
	result := collectFixture(t)

	byGroup := map[string]Deviation{}
	for _, deviation := range result.Deviations {
		byGroup[deviation.Group] = deviation
	}

	// Manifest gegen requirements.txt, beide lokal.
	if got := byGroup["python/fastapi"].Art; got != DeviationConflicting {
		t.Errorf("python/fastapi = %q, erwartet widersprüchlich", got)
	}
	// Chart.yaml gegen Chart.lock, beide deployment.
	if got := byGroup["helm/postgresql"].Art; got != DeviationConflicting {
		t.Errorf("helm/postgresql = %q, erwartet widersprüchlich", got)
	}
	// Dockerfile (deployment) gegen CI-Container (ci).
	if got := byGroup["container/python"].Art; got != DeviationEnvironmental {
		t.Errorf("container/python = %q, erwartet umgebungsbedingt", got)
	}
	// Gleiche Aussage aus zwei Quellen ist keine Abweichung.
	if _, exists := byGroup["python/redis"]; exists {
		t.Errorf("gleiche version und gleicher pin ergeben keine Abweichung")
	}

	// Gezählt werden Gruppen, nicht Zeilen.
	for _, deviation := range result.Deviations {
		if len(deviation.Entries) < 2 {
			t.Errorf("eine Abweichung braucht mindestens zwei Aussagen: %+v", deviation)
		}
		for _, entry := range deviation.Entries {
			if entry.Deviation != deviation.Group {
				t.Errorf("die beteiligte Zeile muss auf ihre Abweichung zeigen: %+v", entry)
			}
		}
	}
	// widersprüchlich steht vor umgebungsbedingt.
	seenEnvironmental := false
	for _, deviation := range result.Deviations {
		if deviation.Art == DeviationEnvironmental {
			seenEnvironmental = true
			continue
		}
		if seenEnvironmental {
			t.Errorf("widersprüchlich gehört nach vorn: %+v", result.Deviations)
			break
		}
	}
}

func TestCollectIstDeterministisch(t *testing.T) {
	first := Render(collectFixture(t), "2026-09-05T12:00:00+02:00")
	second := Render(collectFixture(t), "2026-09-05T12:00:00+02:00")
	if first != second {
		t.Error("zwei Läufe über denselben Bestand müssen dieselbe Datei erzeugen")
	}
}

func TestRenderTraegtVollstaendigesFrontmatter(t *testing.T) {
	result := collectFixture(t)
	rendered := Render(result, "2026-09-05T12:00:00+02:00")

	for _, needle := range []string{
		"type: Version Inventory",
		"title: Versionsinventar",
		"description: Vollständige Übersicht",
		"tags: [versions, inventory, dependencies]",
		"generated: { by: k-doc-inventory, at: 2026-09-05T12:00:00+02:00 }",
		"## Übersicht", "## Abweichungen", "## Ausgewertete Quellen",
		"## Abgelehnte Quellen und Hinweise",
	} {
		if !strings.Contains(rendered, needle) {
			t.Errorf("im Rumpf fehlt %q", needle)
		}
	}

	path := filepath.Join(t.TempDir(), "inventory.md")
	if err := os.WriteFile(path, []byte(rendered), 0o644); err != nil {
		t.Fatalf("schreiben: %v", err)
	}
	status := ReadStatus(path)
	if status.Problem != "" {
		t.Fatalf("das erzeugte Frontmatter muss vollständig sein: %q", status.Problem)
	}
	if status.Entries != len(result.Entries) || status.Deviations != len(result.Deviations) {
		t.Errorf("Status = %+v, Ergebnis = %d/%d", status, len(result.Entries), len(result.Deviations))
	}
	if status.SourcesRead != len(result.Sources) || status.GeneratedBy != GeneratedBy {
		t.Errorf("Status = %+v", status)
	}
}

// Der Fall mit bewusst verschiedenen Versionen desselben Gegenstands in
// lokaler, DevContainer- und Deployment-Quelle. Er steht in einer eigenen
// Fixture, weil er genau eine Frage stellt: stehen die drei Aussagen
// nebeneinander, jede mit ihrem Kontext und ihrer Herkunft?
func TestDreiKontexteStehenNebeneinanderStattZusammengefasst(t *testing.T) {
	result, err := Collect(Options{ProjectDir: "testdata/projekte/dreikontexte"})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	wanted := map[string]string{
		EnvLokal:        "==1.8.4",
		EnvDevcontainer: "1.8.2",
		EnvDeployment:   "1.7.1",
	}
	seen := map[string]Entry{}
	for _, entry := range result.Entries {
		if entry.Group != "python/poetry" {
			continue
		}
		if previous, twice := seen[entry.Context]; twice {
			t.Fatalf("zwei Aussagen im selben Kontext: %+v / %+v", previous, entry)
		}
		seen[entry.Context] = entry
	}
	for context, version := range wanted {
		entry, ok := seen[context]
		if !ok {
			t.Fatalf("im Kontext %s fehlt die Aussage; gefunden: %+v", context, seen)
		}
		if entry.Version != version {
			t.Errorf("%s = %q, erwartet %q", context, entry.Version, version)
		}
		if entry.SourceFile == "" || entry.SourceKey == "" {
			t.Errorf("%s ohne Herkunft: %+v", context, entry)
		}
	}

	var deviation Deviation
	for _, candidate := range result.Deviations {
		if candidate.Group == "python/poetry" {
			deviation = candidate
		}
	}
	if deviation.Art != DeviationEnvironmental {
		t.Fatalf("verschiedene Umgebungen ergeben eine umgebungsbedingte Abweichung: %+v", deviation)
	}
	if len(deviation.Entries) != 3 {
		t.Errorf("alle drei Aussagen gehören in die Abweichung: %+v", deviation.Entries)
	}
	// Und sie wird nicht aufgelöst: alle drei Versionen stehen in der Datei.
	rendered := Render(result, "2026-09-05T12:00:00+02:00")
	for _, version := range []string{"1.8.4", "1.8.2", "1.7.1"} {
		if !strings.Contains(rendered, version) {
			t.Errorf("%s fehlt in der Inventardatei", version)
		}
	}
}

// Jede dargestellte Version trägt eine eindeutige Herkunft: keine zwei Zeilen
// derselben Umgebungstabelle sind über Gegenstand und Herkunft ununterscheidbar,
// und jede Herkunft steht so in der Datei, dass sie ohne Suche wiederzufinden
// ist.
func TestJedeDargestellteVersionTraegtEineEindeutigeHerkunft(t *testing.T) {
	result := collectFixture(t)

	type fingerprint struct{ context, group, file, key, version string }
	seen := map[fingerprint]Entry{}
	for _, entry := range result.Entries {
		key := fingerprint{entry.Context, entry.Group, entry.SourceFile, entry.SourceKey, entry.Version}
		if previous, twice := seen[key]; twice {
			t.Errorf("zwei nicht unterscheidbare Zeilen: %+v / %+v", previous, entry)
		}
		seen[key] = entry
	}

	rendered := Render(result, "2026-09-05T12:00:00+02:00")
	for _, entry := range result.Entries {
		location := entry.SourceFile
		if entry.SourceLine > 0 {
			location += ":" + strconv.Itoa(entry.SourceLine)
		}
		if !strings.Contains(rendered, "`"+location+"`") {
			t.Errorf("die Herkunft %q fehlt in der Datei", location)
		}
		if !strings.Contains(rendered, "`"+entry.SourceKey+"`") {
			t.Errorf("der Schlüssel %q fehlt in der Datei", entry.SourceKey)
		}
	}
}
