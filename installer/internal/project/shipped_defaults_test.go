package project

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/inventory"
	"github.com/kascada/k-playbook/installer/internal/kephalaion"
)

// Tests gegen den Arbeitsstand defaults/ des Repos (Task 094): Was dort liegt,
// kommt mit dem nächsten Release in jede Installation; die Installation
// daneben trägt noch den vorigen Stand und taugt nicht als Abnahme.

// readRepoDefault liest eine Datei aus defaults/ des Repos.
func readRepoDefault(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "defaults", filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("defaults/%s lesen: %v", rel, err)
	}
	return string(data)
}

// Der Always-Teil 030-briefing.md steht im zusammengebauten Verzeichnis
// zwischen den Verweisen und dem Projektwissen, ohne Frontmatter und ohne
// Problem — und bleibt kürzer als 010-verweise.md, die Obergrenze des Tasks.
func TestDefaultsAlwaysTraegtBriefingTeil(t *testing.T) {
	layers := repoDefaultsLayers(t)

	always := mustResolveDoc(t, layers, "k-playbook:/instructions/always/")
	names := partNames(always.Parts)
	index := map[string]int{}
	for i, name := range names {
		index[name] = i
	}
	for _, name := range []string{"010-verweise.md", "030-briefing.md", "100-projektwissen-zuerst.md"} {
		if _, ok := index[name]; !ok {
			t.Fatalf("Teil %s fehlt: %v", name, names)
		}
	}
	if !(index["010-verweise.md"] < index["030-briefing.md"] && index["030-briefing.md"] < index["100-projektwissen-zuerst.md"]) {
		t.Errorf("Reihenfolge: %v", names)
	}
	if len(always.Problems) != 0 {
		t.Errorf("Probleme im mitgelieferten Stand: %v", always.Problems)
	}

	verweise := strings.Index(always.Content, "## Verweise")
	briefing := strings.Index(always.Content, "## Das Briefing")
	wissen := strings.Index(always.Content, "## Projektwissen zuerst")
	if verweise < 0 || briefing < 0 || wissen < 0 || !(verweise < briefing && briefing < wissen) {
		t.Errorf("Überschriften nicht in der Reihenfolge Verweise, Briefing, Projektwissen:\n%s", always.Content)
	}
	if strings.Contains(always.Content, "title: Das Briefing") {
		t.Errorf("Frontmatter des Teils im zusammengebauten Text:\n%s", always.Content)
	}
	for _, want := range []string{
		"`context.now.date`",
		"`k-playbook:/instructions/on-demand/briefing-felder.md`",
		"`k-playbook:/guidelines/betrieb.md`",
	} {
		if !strings.Contains(always.Content, want) {
			t.Errorf("Teil nennt nicht %s", want)
		}
	}

	lines := func(rel string) int { return strings.Count(readRepoDefault(t, rel), "\n") }
	if got, max := lines("instructions/always/030-briefing.md"), lines("instructions/always/010-verweise.md"); got >= max {
		t.Errorf("030-briefing.md hat %d Zeilen, Obergrenze sind weniger als die %d von 010-verweise.md", got, max)
	}
}

// Das Blatt on-demand/briefing-felder.md nennt jedes Feld des Briefings:
// jeden Schlüssel der obersten Ebene und jeden Schlüssel von context — als
// `name` bzw. `context.name`, Unterfelder zählen nicht. Die Schlüssel kommen
// auf demselben Weg zustande wie das Briefing selbst (Marshal eines gebauten
// Briefings, briefingContextFields auf BuildContext), nicht durch Reflexion
// über Context: die würde guidelines und catalogs.rules verlangen, die das
// Briefing herausnimmt. Optionale Felder werden gesetzt, damit sie in der
// Liste sind.
func TestDefaultsBriefingFelderBlattIstVollstaendig(t *testing.T) {
	sheet := readRepoDefault(t, "instructions/on-demand/briefing-felder.md")
	if !strings.HasPrefix(sheet, "---\ntitle: ") || !strings.Contains(sheet, "\ndescription: ") {
		t.Errorf("Blatt ohne Frontmatter title/description")
	}
	names := func(key string) []string { return []string{"`" + key + "`", "`" + key + "."} }
	mentions := func(key string) bool {
		for _, form := range names(key) {
			if strings.Contains(sheet, form) {
				return true
			}
		}
		return false
	}

	root := newBriefingProject(t)
	stubBriefingDocs(t, unavailableDocs)
	stubKephalaionCheck(t, kephalaion.Status{State: kephalaion.StateNotConfigured, Message: "Keine Collection eingetragen."})

	briefing, err := BuildBriefing(context.Background(), root)
	if err != nil {
		t.Fatalf("BuildBriefing: %v", err)
	}
	top := topLevelKeys(t, briefing)
	if len(top) < 5 {
		t.Fatalf("Briefing mit %d Feldern oben: %v", len(top), top)
	}
	for key := range top {
		if !mentions(key) {
			t.Errorf("Blatt nennt das Feld `%s` nicht", key)
		}
	}

	built, err := BuildContext(root)
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}
	if built.VersionSources == nil {
		built.VersionSources = &VersionSources{}
	}
	built.Kephalaion = &kephalaion.Status{}
	built.Links = &ContextLinks{}
	built.GitHub = &ContextGitHub{}
	fields, err := briefingContextFields(built)
	if err != nil {
		t.Fatalf("briefingContextFields: %v", err)
	}
	keys := []string{"available", "message"}
	for _, field := range fields {
		keys = append(keys, field.Name)
	}
	if len(keys) < 10 {
		t.Fatalf("context mit %d Schlüsseln: %v", len(keys), keys)
	}
	for _, key := range keys {
		if !mentions("context." + key) {
			t.Errorf("Blatt nennt `context.%s` nicht", key)
		}
	}
	// Die abgedeckten Felder stehen nicht als context.-Feld im Blatt.
	for _, covered := range []string{"context.guidelines", "context.catalogs.rules", "context.catalogs.reviews"} {
		if mentions(covered) {
			t.Errorf("Blatt nennt `%s`, das das Briefing herausnimmt", covered)
		}
	}
}

// Die Command-Module in defaults/commands/_shared/ sind über die Sorte
// commands erreichbar, tragen Frontmatter title und description und kommen als
// Einzeldatei unverändert; erster-schritt.md bleibt wenige Zeilen. includes/
// trägt weiterhin keinen Baustein — wie seine README sagt.
func TestDefaultsCommandsSharedIncludes(t *testing.T) {
	layers := repoDefaultsLayers(t)

	for _, name := range []string{"erster-schritt.md", "gh.md", "base-tools.md", "docs.md"} {
		ref := "k-playbook:/commands/_shared/" + name
		got := mustResolveDoc(t, layers, ref)
		if got.Directory || got.Disabled || len(got.Parts) != 1 || got.Parts[0].Origin != DocOriginDist || len(got.Problems) != 0 || len(got.Includes) != 0 {
			t.Errorf("%s: %+v", ref, got)
		}
		if got.Content != readRepoDefault(t, "commands/_shared/"+name) {
			t.Errorf("%s: Inhalt weicht von der Datei ab:\n%q", ref, got.Content)
		}
		title, description := docTitleAndDescription(name, []byte(got.Content))
		if title == "" || title == name || description == "" {
			t.Errorf("%s: Frontmatter title/description fehlt (title %q, description %q)", ref, title, description)
		}
	}

	shared := mustResolveDoc(t, layers, "k-playbook:/commands/_shared/erster-schritt.md")
	body := strings.TrimSpace(string(inventoryBody(t, shared.Content)))
	if lines := strings.Count(body, "\n") + 1; lines > 8 {
		t.Errorf("erster-schritt.md hat %d Zeilen Rumpf, soll wenige bleiben:\n%s", lines, body)
	}
	for _, want := range []string{"`k_playbook_briefing`", "`k-playbook briefing --json`", "`documents.always.content`"} {
		if !strings.Contains(body, want) {
			t.Errorf("erster-schritt.md nennt nicht %s", want)
		}
	}
	if strings.Contains(body, "k-playbook context") {
		t.Errorf("erster-schritt.md ruft noch `k-playbook context`:\n%s", body)
	}

	entries, err := layers.Dist.List(docIncludesRoot)
	if err != nil {
		t.Fatalf("includes/ listen: %v", err)
	}
	for _, entry := range entries {
		if entry.Name != "README.md" {
			t.Errorf("includes/ trägt %s, laut README aber keinen Baustein", entry.Name)
		}
	}
}

// inventoryBody ist der Rumpf eines Dokuments ohne Frontmatter.
func inventoryBody(t *testing.T, content string) []byte {
	t.Helper()
	return inventory.Body([]byte(content))
}

// /k-upgrade ist der erste Command auf dem Briefing: sein Text beginnt mit
// `include: _shared/erster-schritt.md`, aufgelöst steht an der Stelle der
// Rumpf des Moduls ohne Frontmatter, roh die Include-Zeile; die Felder kommen
// aus dem Briefing, nicht mehr aus `k-playbook context`. Seit Task 096 gibt
// der Text die Meldung von `k-playbook upgrade` wieder; seit 2026-10-25 führt
// die Hülle den Aufruf aus, der Text ruft ihn nur noch als Rückfall.
// Die Hülle von /k-upgrade führt `k-playbook upgrade` per Shell-Einbettung schon
// beim Aufruf aus, vor dem Briefing des Textes: sonst zöge das Briefing die
// Verlinkung noch mit dem alten Programm nach. Die Einbettung steht am
// Zeilenanfang, so wie Claude Code und OpenCode sie erkennen (gemessen
// 2026-10-25), und der Text nimmt auf ihre Ausgabe Bezug.
func TestShareKUpgradeHuelleFuehrtUpgradeAus(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "share", "commands", "k-upgrade.md"))
	if err != nil {
		t.Fatal(err)
	}
	hull := string(data)
	if !strings.Contains(hull, "\n!`k-playbook upgrade 2>&1`\n") {
		t.Errorf("Hülle ohne Einbettung von k-playbook upgrade:\n%s", hull)
	}
	if !strings.Contains(hull, "allowed-tools: [Bash,") {
		t.Errorf("Claude Code führt die Einbettung nur mit Bash in allowed-tools aus:\n%s", hull)
	}
	if text := readRepoDefault(t, "commands/k-upgrade.md"); !strings.Contains(text, "Ausgabe von\n`k-playbook upgrade`") {
		t.Errorf("der Text nimmt keinen Bezug auf die Ausgabe der Hülle:\n%s", text)
	}
}

func TestDefaultsKUpgradeBindetContextEin(t *testing.T) {
	layers := repoDefaultsLayers(t)
	const ref = "k-playbook:/commands/k-upgrade.md"
	const includeLine = "include: _shared/erster-schritt.md\n"

	file := readRepoDefault(t, "commands/k-upgrade.md")
	if !strings.Contains(file, "\n"+includeLine) {
		t.Fatalf("k-upgrade.md ohne Include-Zeile:\n%s", file)
	}
	if !strings.Contains(file, "\n    k-playbook upgrade\n") || strings.Contains(file, "k-playbook context") {
		t.Errorf("k-upgrade.md ruft nicht `k-playbook upgrade`:\n%s", file)
	}
	module := strings.TrimSpace(string(inventoryBody(t, readRepoDefault(t, "commands/_shared/erster-schritt.md"))))

	got := mustResolveDoc(t, layers, ref)
	want := strings.Replace(file, includeLine, module+"\n", 1)
	if got.Content != want {
		t.Errorf("eingesetzter Text:\n%q\nerwartet\n%q", got.Content, want)
	}
	// Die Include-Zeile selbst ist weg; das Modul nennt sie nur als Prosa in
	// Backticks.
	if strings.Contains(got.Content, "\n"+includeLine) || strings.Contains(got.Content, "title: Erster Schritt") {
		t.Errorf("Include-Zeile oder Frontmatter des Moduls im eingesetzten Text:\n%s", got.Content)
	}
	if !strings.Contains(got.Content, "`documents.always.content`") || strings.Contains(got.Content, "k-playbook context") {
		t.Errorf("k-upgrade liest nicht aus dem Briefing:\n%s", got.Content)
	}
	if len(got.Includes) != 1 || got.Includes[0] != (DocInclude{Ref: "k-playbook:/commands/_shared/erster-schritt.md", Origin: DocOriginDist, Depth: 1}) {
		t.Errorf("includes: %+v", got.Includes)
	}
	if len(got.Problems) != 0 || got.Disabled || got.Directory {
		t.Errorf("Antwort: %+v", got)
	}

	raw, err := ResolveDocRefWith(context.Background(), layers, ref, DocReadOptions{Raw: true})
	if err != nil {
		t.Fatalf("roh: %v", err)
	}
	if raw.Content != file || len(raw.Includes) != 0 {
		t.Errorf("roh: %q, includes %+v", raw.Content, raw.Includes)
	}
}

// /k-doc-inventory ist seit Task 100 Hülle plus Text: Die Hülle in
// share/commands/ nennt nur den Verweis, der Text in defaults/commands/
// beginnt mit den Includes _shared/erster-schritt.md und _shared/docs.md,
// stößt `k-playbook inventory` an und nennt den Ort des Inventars nur als
// keph:-Verweis. Das Modul _docs/inventory.md gibt es nicht mehr.
func TestDefaultsKDocInventoryIstHuelleUndText(t *testing.T) {
	layers := repoDefaultsLayers(t)
	const ref = "k-playbook:/commands/k-doc-inventory.md"
	const inventoryRef = "keph:/docs/generated/versions/inventory.md"

	hull, err := os.ReadFile(filepath.Join("..", "..", "..", "share", "commands", "k-doc-inventory.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(hull), "`"+ref+"`") || strings.Contains(string(hull), "_docs/inventory") {
		t.Errorf("Hülle ohne Verweis auf den Text oder noch mit dem Modul:\n%s", hull)
	}
	if !strings.Contains(string(hull), "mcp__k-playbook__k_playbook_doc_read") {
		t.Errorf("Hülle erlaubt k_playbook_doc_read nicht:\n%s", hull)
	}
	if _, err := os.Stat(filepath.Join("..", "..", "..", "share", "commands", "_docs", "inventory.md")); !os.IsNotExist(err) {
		t.Error("share/commands/_docs/inventory.md ist noch da; der Ablauf steht im Text des Commands")
	}

	file := readRepoDefault(t, "commands/k-doc-inventory.md")
	includes := []string{"erster-schritt.md", "docs.md"}
	want := file
	for _, name := range includes {
		line := "include: _shared/" + name + "\n"
		if !strings.Contains(file, "\n"+line) {
			t.Fatalf("k-doc-inventory.md ohne %q:\n%s", line, file)
		}
		module := strings.TrimSpace(string(inventoryBody(t, readRepoDefault(t, "commands/_shared/"+name))))
		want = strings.Replace(want, line, module+"\n", 1)
	}
	for _, needle := range []string{"k-playbook inventory", inventoryRef, "kephalaion_unavailable", "stale_revision"} {
		if !strings.Contains(file, needle) {
			t.Errorf("k-doc-inventory.md nennt %q nicht", needle)
		}
	}
	for _, stale := range []string{"k-playbook context", "/k-docs-index", "_docs/inventory", "INVENTORY_FILE", ".claude/commands/"} {
		if strings.Contains(file, stale) {
			t.Errorf("k-doc-inventory.md trägt noch %q", stale)
		}
	}

	got := mustResolveDoc(t, layers, ref)
	if got.Content != want {
		t.Errorf("eingesetzter Text:\n%q\nerwartet\n%q", got.Content, want)
	}
	if strings.Contains(got.Content, "\ninclude: ") || strings.Contains(got.Content, "title: Erster Schritt") ||
		strings.Contains(got.Content, "title: Die Docs-Ablage") {
		t.Errorf("Include-Zeile oder Frontmatter eines Moduls im eingesetzten Text:\n%s", got.Content)
	}
	for _, needle := range []string{"`documents.always.content`", "keph:/docs/", "generated/<erzeuger>/", "README.md` und `index.md"} {
		if !strings.Contains(got.Content, needle) {
			t.Errorf("der eingesetzte Text nennt %q nicht", needle)
		}
	}
	wantIncludes := []DocInclude{
		{Ref: "k-playbook:/commands/_shared/erster-schritt.md", Origin: DocOriginDist, Depth: 1},
		{Ref: "k-playbook:/commands/_shared/docs.md", Origin: DocOriginDist, Depth: 1},
	}
	if len(got.Includes) != 2 || got.Includes[0] != wantIncludes[0] || got.Includes[1] != wantIncludes[1] {
		t.Errorf("includes: %+v", got.Includes)
	}
	if len(got.Problems) != 0 || got.Disabled || got.Directory {
		t.Errorf("Antwort: %+v", got)
	}

	raw, err := ResolveDocRefWith(context.Background(), layers, ref, DocReadOptions{Raw: true})
	if err != nil {
		t.Fatalf("roh: %v", err)
	}
	if raw.Content != file || len(raw.Includes) != 0 {
		t.Errorf("roh: %q, includes %+v", raw.Content, raw.Includes)
	}
}

// /k-docs-code ist seit Task 109 Hülle plus Text: Die Hülle nennt nur den
// Verweis und erlaubt die Werkzeuge von checkout und publish; der Text beginnt
// mit den Includes _shared/erster-schritt.md und _shared/docs.md, schreibt nur
// über checkout/publish, scannt ab project.repoRoot ohne die Playbook-
// Verzeichnisse und lässt Dokumente anderer Schreiber unverändert. Das Modul
// _docs/code.md gibt es nicht mehr.
func TestDefaultsKDocsCodeIstHuelleUndText(t *testing.T) {
	layers := repoDefaultsLayers(t)
	const ref = "k-playbook:/commands/k-docs-code.md"

	hullData, err := os.ReadFile(filepath.Join("..", "..", "..", "share", "commands", "k-docs-code.md"))
	if err != nil {
		t.Fatal(err)
	}
	hull := string(hullData)
	if !strings.Contains(hull, "`"+ref+"`") || strings.Contains(hull, "Vorübergehend nicht verfügbar") || strings.Contains(hull, "_docs/code") {
		t.Errorf("Hülle ohne Verweis auf den Text oder noch mit dem alten Ablauf:\n%s", hull)
	}
	for _, tool := range []string{"mcp__k-playbook__k_playbook_doc_read", "mcp__k-playbook__k_playbook_docs_checkout",
		"mcp__k-playbook__k_playbook_docs_publish", "mcp__k-playbook__k_playbook_docs_exclude_add",
		"mcp__kephalaion__list", "mcp__kephalaion__read"} {
		if !strings.Contains(hull, tool) {
			t.Errorf("Hülle erlaubt %s nicht", tool)
		}
	}
	if _, err := os.Stat(filepath.Join("..", "..", "..", "share", "commands", "_docs", "code.md")); !os.IsNotExist(err) {
		t.Error("share/commands/_docs/code.md ist noch da; der Ablauf steht im Text des Commands")
	}

	file := readRepoDefault(t, "commands/k-docs-code.md")
	want := file
	for _, name := range []string{"erster-schritt.md", "docs.md"} {
		line := "include: _shared/" + name + "\n"
		if !strings.Contains(file, "\n"+line) {
			t.Fatalf("k-docs-code.md ohne %q", line)
		}
		module := strings.TrimSpace(string(inventoryBody(t, readRepoDefault(t, "commands/_shared/"+name))))
		want = strings.Replace(want, line, module+"\n", 1)
	}
	if strings.Index(file, "include: _shared/erster-schritt.md") > strings.Index(file, "## Schritt 2") {
		t.Error("die Includes stehen nicht im ersten Schritt")
	}
	for _, needle := range []string{
		"k_playbook_docs_checkout", "k_playbook_docs_publish", "keph:/docs/generated/code/", ".k-playbook/",
		"discard: true", "allow_hints",
		"generated: { by: k-docs-code", "questions:", "hints", "kephalaion_unavailable", "outcome_unknown",
		"generated.by: ks-overlay-repo-analyse", "unverändert liegen",
		"k_playbook_docs_exclude_add", "NEW_EXCLUDES", "Abschnitt „Scope“", "Abschnitt „Der Kopf“",
		"<die Scope-Zeilen aus dem Abschnitt „Scope“>",
	} {
		if !strings.Contains(file, needle) {
			t.Errorf("k-docs-code.md nennt %q nicht", needle)
		}
	}
	// Kopf und Scope stehen seit Task 120 allein in _shared/docs.md; der Text
	// des Commands verweist darauf und wiederholt sie nicht.
	for _, repeated := range []string{
		"SCAN_ROOT = context.project.repoRoot", "**Versteckte Pfade**", "`tests/fixtures/`", "Aus K-PLAYBOOK.yaml",
		"context.docs.exclude", "context.docs.problems", "„Neu laden“", "(etwa `exclude: [docs/]`)", "k-playbook docs exclude add",
		"`verified:` trägt nur ein Mensch", "`title` und `description` sind Pflicht", "eine Frage, eine\n  Datei",
	} {
		if strings.Contains(file, repeated) {
			t.Errorf("k-docs-code.md wiederholt %q aus _shared/docs.md", repeated)
		}
	}
	// Ein Dokument des Overlay-Skills wird nie als „entfällt“ vorgeschlagen.
	if !strings.Contains(file, "**entfällt** schlägst du nur für Dokumente vor, für die dieser Lauf zuständig ist") ||
		!strings.Contains(file, "bleibt       20-overlay-delta       generated.by: ks-overlay-repo-analyse") {
		t.Error("k-docs-code.md regelt nicht, dass Dokumente anderer Schreiber bleiben")
	}
	for _, stale := range []string{"k-playbook context", "/k-docs-index", "_docs/code", ".claude/commands/", "CODE_DIR", "Vorübergehend"} {
		if strings.Contains(file, stale) {
			t.Errorf("k-docs-code.md trägt noch %q", stale)
		}
	}

	got := mustResolveDoc(t, layers, ref)
	if got.Content != want || len(got.Problems) != 0 || len(got.Includes) != 2 {
		t.Errorf("eingesetzter Text weicht ab: problems %v, includes %+v", got.Problems, got.Includes)
	}

	shared := readRepoDefault(t, "commands/_shared/docs.md")
	for _, stale := range []string{"gehört genau einem Erzeuger", "als Ganzes", "erzeugt Kephalaion"} {
		if strings.Contains(shared, stale) {
			t.Errorf("_shared/docs.md trägt noch %q", stale)
		}
	}
	for _, needle := range []string{"mehrere\n  Schreiber", "k_playbook_docs_checkout", "`hints`", "`questions`", "`publish` aus der Erzeugertabelle", "Indizieren",
		"`docs.exclude`", "`context.docs.exclude`", "`k_playbook_docs_exclude_add`"} {
		if !strings.Contains(shared, needle) {
			t.Errorf("_shared/docs.md nennt %q nicht", needle)
		}
	}
	assertSharedDocsKopfUndScope(t, shared)
}

// assertSharedDocsKopfUndScope prüft die beiden Abschnitte, die Task 120 in
// _shared/docs.md gesammelt hat: den einen Kopf aller Dokumente unter docs/
// mit seinen Feldern und den Scope der Erzeuger, die den Code durchsuchen,
// samt seiner Geltung nur für /k-docs-code und /k-docs-tools.
func assertSharedDocsKopfUndScope(t *testing.T, shared string) {
	t.Helper()
	kopf, scope := strings.Index(shared, "\n### Der Kopf\n"), strings.Index(shared, "\n### Scope\n")
	if kopf < 0 || scope < kopf {
		t.Fatalf("_shared/docs.md ohne die Abschnitte „Der Kopf“ und „Scope“ in dieser Reihenfolge")
	}
	head, scopeText := shared[kopf:scope], squashSpace(shared[scope:])
	order := []string{"\ntitle: ", "\ndescription: ", "\ngenerated: { by: <erzeuger>, at: <GENERATED_AT>, stats: { … } }",
		"\nhints: ", "\nquestions: ", "\nsources:\n  - resource: ", "\n    item: ", "\n    version: ", "\nverified: "}
	last := -1
	for _, field := range order {
		at := strings.Index(head, field)
		if at < 0 || at < last {
			t.Errorf("Kopf: Feld %q fehlt oder steht nicht in der Reihenfolge des Schemas", strings.TrimSpace(field))
			continue
		}
		last = at
	}
	for _, needle := range []string{"`title` — Pflicht", "`description` — Pflicht, ein Satz", "`stats` ist optional",
		"kein Suchinhalt", "`verified` — trägt nur ein Mensch ein", "Darüber hinaus schreibt kein Erzeuger ein Kopffeld",
		"`type`", "`tags`", "`status`", "`inventory:`"} {
		if !strings.Contains(squashSpace(head), needle) {
			t.Errorf("Kopf nennt %q nicht", needle)
		}
	}
	for _, needle := range []string{
		"gilt nur für die Erzeuger, die den Code durchsuchen: `/k-docs-code` und `/k-docs-tools`",
		"`/k-doc-inventory` und `ks-overlay-repo-analyse` binden dieses Modul auch ein, wenden den Scope aber nicht an",
		"`SCAN_ROOT = context.project.repoRoot`", "**Stand**", "`EXTRA_PATH`", "**Immer ausgeschlossen**",
		"`context.playbook.dir`", "`context.local.dir`", "`<context.project.dir>/.k-playbook/`", "**Versteckte Pfade**",
		"**Weitere Ausschlüsse**", "`.gitignore`", "`context.docs.exclude`", "`context.docs.problems`", "Aus K-PLAYBOOK.yaml",
		"`NEW_EXCLUDES`", "**Ausschlüsse speichern.**", "`k_playbook_docs_exclude_add`", "k-playbook docs exclude add",
		"„Neu laden“", "`rejected`", "`unsupported`",
	} {
		if !strings.Contains(scopeText, needle) {
			t.Errorf("Scope nennt %q nicht", needle)
		}
	}
}

// squashSpace fasst Zeilenumbrüche und Leerraum zu je einem Leerzeichen
// zusammen, damit ein Satz unabhängig vom Umbruch gefunden wird.
func squashSpace(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

// /k-docs-tools ist seit Task 120 Hülle plus Text wie /k-docs-code: Die Hülle
// nennt nur den Verweis, ist nicht mehr abgeschaltet und erlaubt WebFetch
// und die Werkzeuge von checkout und publish; der Text nimmt die Kandidaten
// allein aus dem Inventar, schreibt über checkout/publish mit dem Erzeuger
// libs, belegt nur den gemeinsamen Kopf und verweist für Kopf und Scope auf
// _shared/docs.md. Das Modul _docs/tools.md gibt es nicht mehr.
func TestDefaultsKDocsToolsIstHuelleUndText(t *testing.T) {
	layers := repoDefaultsLayers(t)
	const ref = "k-playbook:/commands/k-docs-tools.md"

	hullData, err := os.ReadFile(filepath.Join("..", "..", "..", "share", "commands", "k-docs-tools.md"))
	if err != nil {
		t.Fatal(err)
	}
	hull := string(hullData)
	if !strings.Contains(hull, "`"+ref+"`") || strings.Contains(hull, "Vorübergehend nicht verfügbar") || strings.Contains(hull, "_docs/tools") {
		t.Errorf("Hülle ohne Verweis auf den Text, noch abgeschaltet oder mit dem alten Ablauf:\n%s", hull)
	}
	description := ""
	for _, line := range strings.Split(hull, "\n") {
		if value, ok := strings.CutPrefix(line, "description: "); ok {
			description = value
		}
	}
	if !strings.Contains(description, "keph:/docs/generated/libs/") || strings.Contains(description, "k-playbook-local/docs/libs/") {
		t.Errorf("description nennt nicht keph:/docs/generated/libs/ oder noch die Platte: %q", description)
	}
	for _, tool := range []string{"WebFetch", "mcp__k-playbook__k_playbook_doc_read", "mcp__k-playbook__k_playbook_docs_checkout",
		"mcp__k-playbook__k_playbook_docs_publish", "mcp__k-playbook__k_playbook_docs_exclude_add",
		"mcp__kephalaion__list", "mcp__kephalaion__read"} {
		if !strings.Contains(hull, tool) {
			t.Errorf("Hülle erlaubt %s nicht", tool)
		}
	}
	if _, err := os.Stat(filepath.Join("..", "..", "..", "share", "commands", "_docs", "tools.md")); !os.IsNotExist(err) {
		t.Error("share/commands/_docs/tools.md ist noch da; der Ablauf steht im Text des Commands")
	}

	file := readRepoDefault(t, "commands/k-docs-tools.md")
	want := file
	for _, name := range []string{"erster-schritt.md", "docs.md"} {
		line := "include: _shared/" + name + "\n"
		if !strings.Contains(file, "\n"+line) {
			t.Fatalf("k-docs-tools.md ohne %q", line)
		}
		module := strings.TrimSpace(string(inventoryBody(t, readRepoDefault(t, "commands/_shared/"+name))))
		want = strings.Replace(want, line, module+"\n", 1)
	}
	if strings.Index(file, "include: _shared/erster-schritt.md") > strings.Index(file, "## Schritt 2") {
		t.Error("die Includes stehen nicht im ersten Schritt")
	}
	for _, needle := range []string{
		"k_playbook_docs_checkout", "k_playbook_docs_publish", "`erzeuger: libs`", "keph:/docs/generated/libs/",
		"keph:/docs/generated/versions/inventory.md", "`generated.stats.entries`", "`k-playbook inventory`",
		"keinen Lauf und keinen eigenen Scan", "Kandidaten sind alle Gegenstände des Inventars",
		"begrenzt nur dieses Zählen, nicht die Liste", "generated: { by: k-docs-tools", "`k-tools-scan`",
		"Abschnitt „Scope“", "Abschnitt „Der Kopf“", "<die Scope-Zeilen aus dem Abschnitt „Scope“>",
		"resource: keph:/docs/generated/versions/inventory.md", "item: <gegenstand>", "version: <version>",
		"go-github.com-yuin-goldmark", "`docs/version-inventory.md`", "Known-Pitfall-Liste", "**höchstens drei**",
		"vorhanden", "nicht\nvorausgewählt", "discard: true", "allow_hints", "kephalaion_unavailable", "outcome_unknown",
		"unverändert liegen",
	} {
		if !strings.Contains(file, needle) {
			t.Errorf("k-docs-tools.md nennt %q nicht", needle)
		}
	}
	for _, stale := range []string{
		"k-playbook context", "/k-docs-index", "_docs/tools", ".claude/commands/", "LIBS_DIR", "Vorübergehend",
		"type: Tool Reference", "last-reviewed:", "severity:", "version-pin:", "lib: ", "## Offene Fragen",
		"SCAN_ROOT = context.project.repoRoot", "**Versteckte Pfade**", "„Neu laden“", "Transitive-only",
	} {
		if strings.Contains(file, stale) {
			t.Errorf("k-docs-tools.md trägt noch %q", stale)
		}
	}

	// Die Vorlage in Schritt 6 belegt nur Felder des gemeinsamen Kopfs.
	start := strings.Index(file, "```markdown\n---\n")
	if start < 0 {
		t.Fatal("k-docs-tools.md ohne Vorlage mit Frontmatter")
	}
	block, ok := inventory.FrontmatterBlock([]byte(file[start+len("```markdown\n"):]))
	if !ok {
		t.Fatal("Vorlage ohne geschlossenes Frontmatter")
	}
	for _, line := range strings.Split(block, "\n") {
		key, _, _ := strings.Cut(line, ":")
		if line != "" && !strings.HasPrefix(line, " ") && !slices.Contains([]string{"title", "description", "generated", "hints", "questions", "sources", "verified"}, key) {
			t.Errorf("Vorlage trägt das fremde Kopffeld %q", key)
		}
	}

	got := mustResolveDoc(t, layers, ref)
	if got.Content != want || len(got.Problems) != 0 || len(got.Includes) != 2 {
		t.Errorf("eingesetzter Text weicht ab: problems %v, includes %+v", got.Problems, got.Includes)
	}
}

// /k-docs ist Hülle plus Text: Er berichtet über keph:/docs/ und gibt an die
// drei Erzeuger weiter, in der Reihenfolge Inventar, Code, Tools; vom alten
// Verteiler über die Platte bleibt nichts.
func TestDefaultsKDocsIstHuelleUndText(t *testing.T) {
	layers := repoDefaultsLayers(t)
	const ref = "k-playbook:/commands/k-docs.md"

	hullData, err := os.ReadFile(filepath.Join("..", "..", "..", "share", "commands", "k-docs.md"))
	if err != nil {
		t.Fatal(err)
	}
	hull := string(hullData)
	if !strings.Contains(hull, "`"+ref+"`") || strings.Contains(hull, "Vorübergehend nicht verfügbar") || strings.Contains(hull, "_docs/") {
		t.Errorf("Hülle ohne Verweis auf den Text, noch abgeschaltet oder mit dem alten Ablauf:\n%s", hull)
	}
	for _, tool := range []string{"WebFetch", "mcp__k-playbook__k_playbook_doc_read", "mcp__k-playbook__k_playbook_docs_checkout",
		"mcp__k-playbook__k_playbook_docs_publish", "mcp__k-playbook__k_playbook_docs_exclude_add",
		"mcp__kephalaion__list", "mcp__kephalaion__read"} {
		if !strings.Contains(hull, tool) {
			t.Errorf("Hülle erlaubt %s nicht", tool)
		}
	}
	if strings.Contains(hull, "knowledge") {
		t.Error("Hülle erlaubt noch Werkzeuge der Wissensablage")
	}

	file := readRepoDefault(t, "commands/k-docs.md")
	want := file
	for _, name := range []string{"erster-schritt.md", "docs.md"} {
		line := "include: _shared/" + name + "\n"
		if !strings.Contains(file, "\n"+line) {
			t.Fatalf("k-docs.md ohne %q", line)
		}
		module := strings.TrimSpace(string(inventoryBody(t, readRepoDefault(t, "commands/_shared/"+name))))
		want = strings.Replace(want, line, module+"\n", 1)
	}
	if strings.Index(file, "include: _shared/erster-schritt.md") > strings.Index(file, "## Schritt 2") {
		t.Error("die Includes stehen nicht im ersten Schritt")
	}
	for _, needle := range []string{
		"k-playbook:/commands/k-doc-inventory.md", "k-playbook:/commands/k-docs-code.md", "k-playbook:/commands/k-docs-tools.md",
		"`recursive: true`, `frontmatter: true`", "`generated.stats`", "alter Kopf", "`alle`",
		"nacheinander `inventory`, `code`, `tools`, in genau dieser Reihenfolge", "Es wurde nichts geschrieben",
		"kephalaion_unavailable", "/k-upgrade",
	} {
		if !strings.Contains(file, needle) {
			t.Errorf("k-docs.md nennt %q nicht", needle)
		}
	}
	for _, stale := range []string{
		"k-playbook context", "/k-docs-index", "/k-docs-extract", "_docs/", ".claude/commands/", "Vorübergehend",
		"RESOLVED_DOCS_DIR", "knowledge", "opencode.json", "inbox/", "material/",
	} {
		if strings.Contains(file, stale) {
			t.Errorf("k-docs.md trägt noch %q", stale)
		}
	}

	got := mustResolveDoc(t, layers, ref)
	if got.Content != want || len(got.Problems) != 0 || len(got.Includes) != 2 {
		t.Errorf("eingesetzter Text weicht ab: problems %v, includes %+v", got.Problems, got.Includes)
	}
}

// Der Skill ks-overlay-repo-analyse ist seit Task 109 Hülle plus Text: Die
// Hülle unter share/skills/ bleibt ein Skill-Verzeichnis mit SKILL.md (so wird
// sie weiter verlinkt) und nennt nur den Verweis; Text und Vorlagen liegen
// unter defaults/commands/_skills/ und kommen über k-playbook:/commands/. Phase 5
// schreibt über checkout/publish und lässt Dokumente von /k-docs-code
// unverändert.
func TestDefaultsOverlaySkillIstHuelleUndText(t *testing.T) {
	layers := repoDefaultsLayers(t)
	const ref = "k-playbook:/commands/_skills/overlay-repo-analyse.md"
	skillDir := filepath.Join("..", "..", "..", "share", "skills", "overlay-repo-analyse")

	if _, ok := skillSources(filepath.Join("..", "..", "..", "share", "skills"))["overlay-repo-analyse"]; !ok {
		t.Fatal("share/skills/overlay-repo-analyse wird nicht mehr als Skill erkannt")
	}
	entries, err := os.ReadDir(skillDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != skillFileName {
		t.Errorf("Skill-Verzeichnis trägt mehr als die Hülle: %v", entries)
	}
	hull, err := os.ReadFile(filepath.Join(skillDir, skillFileName))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(hull), "name: ks-overlay-repo-analyse\n") || !strings.Contains(string(hull), "`"+ref+"`") ||
		strings.Contains(string(hull), "_shared/context.md") || strings.Contains(string(hull), "/k-docs-index") {
		t.Errorf("Hülle:\n%s", hull)
	}

	got := mustResolveDoc(t, layers, ref)
	if got.Parts[0].Origin != DocOriginDist || len(got.Problems) != 0 || len(got.Includes) != 2 ||
		got.Includes[0].Ref != "k-playbook:/commands/_shared/erster-schritt.md" || got.Includes[1].Ref != "k-playbook:/commands/_shared/docs.md" {
		t.Errorf("Auflösung: parts %+v, problems %v, includes %+v", got.Parts, got.Problems, got.Includes)
	}
	file := readRepoDefault(t, "commands/_skills/overlay-repo-analyse.md")
	for _, needle := range []string{
		"k_playbook_docs_checkout", "k_playbook_docs_publish", "keph:/docs/generated/code/", "discard: true",
		"generated: { by: ks-overlay-repo-analyse", "Abschnitt „Der Kopf“", "freien Nummern im Arbeitsverzeichnis",
		"Ein Dokument mit `generated.by: k-docs-code`", "**unverändert liegen**",
	} {
		if !strings.Contains(file, needle) {
			t.Errorf("Skill-Text nennt %q nicht", needle)
		}
	}
	for _, stale := range []string{"k-playbook-local/docs", "/k-docs-index", "_shared/context.md", "k-playbook context", ".claude/skills/", "PLAYBOOK.md"} {
		if strings.Contains(file, stale) {
			t.Errorf("Skill-Text trägt noch %q", stale)
		}
	}

	for _, name := range []string{"overlay-doc.md.template", "subagent-prompt-overlay.md.template"} {
		template := "k-playbook:/commands/_skills/overlay-repo-analyse/" + name
		if !strings.Contains(file, "`"+template+"`") {
			t.Errorf("Skill-Text nennt die Vorlage %s nicht", template)
		}
		result := mustResolveDoc(t, layers, template)
		if result.Content != readRepoDefault(t, "commands/_skills/overlay-repo-analyse/"+name) {
			t.Errorf("%s nicht aus defaults/ aufgelöst", template)
		}
	}
	if !strings.Contains(squashSpace(file), "Den Scope der Docs-Ablage wendet dieser Skill nicht an") {
		t.Error("Skill-Text sagt nicht, dass er den Scope der Docs-Ablage nicht anwendet")
	}
	doc := readRepoDefault(t, "commands/_skills/overlay-repo-analyse/overlay-doc.md.template")
	if !strings.Contains(doc, "generated: { by: ks-overlay-repo-analyse") || !strings.Contains(doc, "questions: [") || strings.Contains(doc, "](./") {
		t.Errorf("Vorlage overlay-doc.md.template:\n%s", doc)
	}
	// Die Vorlage belegt nur Felder des gemeinsamen Kopfs.
	if block, ok := inventory.FrontmatterBlock([]byte(doc)); ok {
		for _, line := range strings.Split(block, "\n") {
			key, _, _ := strings.Cut(line, ":")
			if line != "" && !strings.HasPrefix(line, " ") && !slices.Contains([]string{"title", "description", "generated", "hints", "questions", "sources", "verified"}, key) {
				t.Errorf("Vorlage overlay-doc.md.template trägt das fremde Kopffeld %q", key)
			}
		}
	} else {
		t.Error("Vorlage overlay-doc.md.template ohne Frontmatter")
	}
}

// /k-upgrade 3.2 räumt seit Task 109 die alte Code-Doku ab (löschen, nicht
// verschieben) und sucht in AGENTS.md und opencode.json[c] nach dem ganzen
// Präfix k-playbook-local/docs/; vorgeschlagen wird immer entfernen.
func TestDefaultsKUpgradeDokuUmstellungCode(t *testing.T) {
	file := readRepoDefault(t, "commands/k-upgrade.md")
	start, end := strings.Index(file, "### 3.2 — "), strings.Index(file, "### 3.3 — ")
	if start < 0 || end < start {
		t.Fatal("Abschnitt 3.2 nicht gefunden")
	}
	section := file[start:end]
	for _, needle := range []string{
		"| `code/` | `keph:/docs/generated/code/` | Erzeuger `/k-docs-code` schreibt nach Kephalaion: Plattendateien löschen",
		"| `libs/` | `keph:/docs/generated/libs/` | Erzeuger `/k-docs-tools` schreibt nach Kephalaion: Plattendateien löschen; neu erzeugen mit `/k-docs-tools`",
		"nennen das Präfix\n`k-playbook-local/docs/`",
		"Der Vorschlag ist immer\n**entfernen**, nie umschreiben",
		"`pfad:von-bis`",
		"code/00-overview.md          [löschen]",
		"AGENTS.md:254-264            [entfernen]",
		"opencode.json:8              [entfernen]",
		"Die Umstellung ist wiederholbar",
	} {
		if !strings.Contains(section, needle) {
			t.Errorf("3.2 nennt %q nicht", needle)
		}
	}
	for _, stale := range []string{"`/k-docs-code` schreibt noch auf die Platte", "code/, libs/", "Skill `ai-session-memory` schreiben"} {
		if strings.Contains(section, stale) {
			t.Errorf("3.2 trägt noch %q", stale)
		}
	}
}

// /k-upgrade 3.2 räumt seit Task 120 auch libs/ ab wie code/: löschen, neu
// erzeugen mit /k-docs-tools. Kein Bereich bleibt mehr liegen, weil sein
// Erzeuger noch auf die Platte schreibt; alter Index, Aufräumen leerer
// Verzeichnisse und das Beispiel ziehen libs/ mit.
func TestDefaultsKUpgradeDokuUmstellungLibs(t *testing.T) {
	file := readRepoDefault(t, "commands/k-upgrade.md")
	start, end := strings.Index(file, "### 3.2 — "), strings.Index(file, "### 3.3 — ")
	if start < 0 || end < start {
		t.Fatal("Abschnitt 3.2 nicht gefunden")
	}
	section := squashSpace(file[start:end])
	for _, needle := range []string{
		"| `libs/` | `keph:/docs/generated/libs/` | Erzeuger `/k-docs-tools` schreibt nach Kephalaion: Plattendateien löschen; neu erzeugen mit `/k-docs-tools`. Kein Verschieben",
		"Für die Erzeuger (`versions/`, `code/` und `libs/`)",
		"Jede Datei unter `code/` und `libs/` wird einzeln zum Löschen vorgeschlagen",
		"`keph:/docs/generated/libs/` noch leer ist",
		"`/k-docs-tools` nach dem Inventar",
		"Einträge zu `manual/`, `versions/`, `code/`, `libs/` und losen Dateien abgeräumt sind",
		"alle Zeilen zu `manual/`, `versions/`, `code/`, `libs/` und losen Dateien ausgeführt sind",
		"(`manual/`, `versions/`, `code/`, `libs/`, `extracted/`), löschst du mit",
		"libs/celery.md [löschen] Erzeuger /k-docs-tools schreibt nach keph:/docs/generated/libs/",
		"auch für solche auf `libs/`.",
	} {
		if !strings.Contains(section, needle) {
			t.Errorf("3.2 nennt %q nicht", needle)
		}
	}
	for _, stale := range []string{
		"schreibt noch auf die Platte", "heute nur `libs/`", "auch wenn `libs/` noch liegt", "libs/ [bleibt]",
		"bis `/k-docs-tools` umgestellt ist", "`libs/` bleibt liegen", "umgestellten Erzeuger", "ändert dessen Task seine Zeile",
	} {
		if strings.Contains(section, stale) {
			t.Errorf("3.2 trägt noch %q", stale)
		}
	}
}

// /k-upgrade 3.4 räumt seit Task 129 auch commands/ und skills/ unter
// k-playbook-local/ ab: leer wird gelöscht, sonst je Eintrag mit Ziel unter
// .k-playbook/ genannt; umgezogen wird nur auf ausdrücklichen Zuruf. Die
// Hülle nennt beide Verzeichnisse in ihrer description.
func TestDefaultsKUpgradeAufraeumenCommandsSkills(t *testing.T) {
	file := readRepoDefault(t, "commands/k-upgrade.md")
	start, end := strings.Index(file, "### 3.4 — "), strings.Index(file, "## Schritt 4 — ")
	if start < 0 || end < start {
		t.Fatal("Abschnitt 3.4 nicht gefunden")
	}
	section := squashSpace(file[start:end])
	for _, needle := range []string{
		"legt sieben Verzeichnisse unter `<context.local.dir>/` nicht mehr an",
		"`<context.project.dir>/.k-playbook/commands/` und `<context.project.dir>/.k-playbook/skills/`",
		"- `material/` - `commands/` - `skills/`",
		"Erst das neue Programm nimmt Einträge aus `.k-playbook/<sorte>/`",
		"diese Dateien mit diesem Programm nicht mehr wirken",
		"`.k-playbook/<sorte>/<gleicher pfad>`",
		"Liegt am Ziel schon etwas, steht `Ziel belegt` hinter der Zeile.",
		"Dateien mit dem Namen `README.md` und die verwaltete `.gitignore` ziehen nicht um",
		"k-gui.md → .k-playbook/commands/k-gui.md Ziel belegt",
		"forelle/ → .k-playbook/skills/forelle/",
		"Umgebaut wird von Hand oder auf ausdrücklichen Zuruf des Nutzers in derselben Sitzung",
		"gehören in den nächsten Commit des Projekts",
		"`git mv`",
		"Ein Eintrag mit `Ziel belegt` bleibt liegen und wird genannt; zusammengeführt wird nicht.",
		"Briefing neu (`k-playbook briefing`; der Katalog hat sich geändert), das die Verlinkung nachzieht",
		"führst c) und d) für `commands/` und `skills/` noch einmal aus",
	} {
		if !strings.Contains(section, needle) {
			t.Errorf("3.4 nennt %q nicht", needle)
		}
	}
	if strings.Contains(section, "fünf Verzeichnisse") {
		t.Error("3.4 zählt noch fünf Verzeichnisse")
	}

	hull, err := os.ReadFile(filepath.Join("..", "..", "..", "share", "commands", "k-upgrade.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, needle := range []string{"(docs, tasks, data, material, commands, skills)", "mit seinem Ziel unter .k-playbook/"} {
		if !strings.Contains(string(hull), needle) {
			t.Errorf("Hülle von /k-upgrade nennt %q nicht", needle)
		}
	}
}

// Die Doku-Aufträge (Task 124): Die Anweisung doku-auftraege.md beschreibt
// Kopf und Lebenslauf, docs-sync nennt das Einreihen als Normalfall, /k-docs
// zählt die offenen Aufträge, und die beiden Erzeuger listen sie im Preflight
// und schließen sie nach einem erfolgreichen publish ab — alles über die
// doc-Werkzeuge, die ihre Hüllen erlauben.
func TestDefaultsDocsAuftraege(t *testing.T) {
	layers := repoDefaultsLayers(t)

	instruction := mustResolveDoc(t, layers, "k-playbook:/instructions/on-demand/doku-auftraege.md").Content
	if !strings.HasPrefix(instruction, "---\ntitle: Doku-Aufträge\ndescription: ") {
		t.Errorf("doku-auftraege.md ohne title und description:\n%.200s", instruction)
	}
	for _, needle := range []string{
		"keph:/queue/docs/", "Sorte `docs`", "erzeuger: code | tools", "pfade:", "anlass:", "sitzung:", "entstanden:",
		"Ein Auftrag je Sitzung und Erzeuger", "Kein Doppel", "`^sitzung: <session>$`", "Wann nicht eingereiht wird",
		"Wer ihn schreibt", "Wer ihn abschließt", "Was nicht hineingehört", "`/k-enforcement`", "`open` leer", "`cleaned` ist wahr",
	} {
		if !strings.Contains(squashSpace(instruction), squashSpace(needle)) {
			t.Errorf("doku-auftraege.md nennt %q nicht", needle)
		}
	}

	sync := readRepoDefault(t, "rules/docs-sync.md")
	project, err := os.ReadFile(filepath.Join("..", "..", "..", "rules", "docs-sync.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(project) != sync {
		t.Error("rules/docs-sync.md und defaults/rules/docs-sync.md weichen voneinander ab")
	}
	for _, needle := range []string{"k-playbook:/instructions/on-demand/doku-auftraege.md", "„Nachzug eingereiht“",
		"`erzeuger: tools`", "**einzureihen**"} {
		if !strings.Contains(sync, needle) {
			t.Errorf("docs-sync.md nennt %q nicht", needle)
		}
	}

	shared := readRepoDefault(t, "commands/_shared/docs.md")
	section := shared[strings.Index(shared, "\n### Doku-Aufträge\n"):]
	for _, needle := range []string{"`k_playbook_doc_grep` mit `kind: docs`", "`^erzeuger: code$`", "`^erzeuger: tools$`",
		"`ORDERS`", "`k_playbook_doc_done`", "`open` leer", "`cleaned` ist wahr", "bleiben offen", "engen den Scope nicht ein"} {
		if !strings.Contains(squashSpace(section), needle) {
			t.Errorf("_shared/docs.md, Doku-Aufträge, nennt %q nicht", needle)
		}
	}
	for name, erzeuger := range map[string]string{"k-docs-code": "code", "k-docs-tools": "tools"} {
		file := readRepoDefault(t, "commands/"+name+".md")
		for _, needle := range []string{"`erzeuger: " + erzeuger + "`", "`ORDERS`", "<die Zeilen „Aufträge“ aus dem Abschnitt „Doku-Aufträge“>",
			"keph:/queue/docs/done/"} {
			if !strings.Contains(file, needle) {
				t.Errorf("%s.md nennt %q nicht", name, needle)
			}
		}
	}
	docs := readRepoDefault(t, "commands/k-docs.md")
	for _, needle := range []string{"`kind: docs`", "Aufträge    <N> (<N> code, <N> tools), ältester <entstanden> | keine",
		"<N> Aufträge offen"} {
		if !strings.Contains(docs, needle) {
			t.Errorf("k-docs.md nennt %q nicht", needle)
		}
	}
	for _, name := range []string{"k-docs", "k-docs-code", "k-docs-tools"} {
		hull, err := os.ReadFile(filepath.Join("..", "..", "..", "share", "commands", name+".md"))
		if err != nil {
			t.Fatal(err)
		}
		for _, tool := range []string{"mcp__k-playbook__k_playbook_doc_list", "mcp__k-playbook__k_playbook_doc_grep",
			"mcp__k-playbook__k_playbook_doc_done"} {
			if !strings.Contains(string(hull), tool) {
				t.Errorf("Hülle %s erlaubt %s nicht", name, tool)
			}
		}
	}
}

// /k-enforcement ist seit Task 124 Hülle plus Text: Die Hülle nennt nur den
// Verweis und erlaubt genau die Werkzeuge des Prüfberichts samt Einreihen;
// der Text ist deutsch, beginnt mit dem Include _shared/erster-schritt.md,
// nimmt die Regeln aus documents.rules, prüft dieselbe Dateimenge wie
// k-check --mode changed, führt k-check selbst aus und reiht erst nach
// Freigabe einen Doku-Auftrag ein. _shared/context.md, catalogs.rules,
// extracted/ und der Skill ks-enforcement kommen nicht mehr vor.
func TestDefaultsKEnforcementIstHuelleUndText(t *testing.T) {
	layers := repoDefaultsLayers(t)
	const ref = "k-playbook:/commands/k-enforcement.md"

	hullData, err := os.ReadFile(filepath.Join("..", "..", "..", "share", "commands", "k-enforcement.md"))
	if err != nil {
		t.Fatal(err)
	}
	hull := string(hullData)
	if !strings.Contains(hull, "`"+ref+"`") || strings.Contains(hull, "## Schritt") || strings.Contains(hull, "_shared/context.md") {
		t.Errorf("Hülle ohne Verweis auf den Text oder noch mit dem alten Ablauf:\n%s", hull)
	}
	allowed := ""
	for _, line := range strings.Split(hull, "\n") {
		if value, ok := strings.CutPrefix(line, "allowed-tools: "); ok {
			allowed = value
		}
	}
	want := "[Read, Bash, mcp__k-playbook__k_playbook_briefing, mcp__k-playbook__k_playbook_doc_read, " +
		"mcp__k-playbook__k_playbook_doc_create, mcp__k-playbook__k_playbook_doc_list, mcp__k-playbook__k_playbook_doc_grep, " +
		"mcp__kephalaion__list, mcp__kephalaion__read, mcp__kephalaion__write]"
	if allowed != want {
		t.Errorf("allowed-tools:\n%s\nerwartet\n%s", allowed, want)
	}

	file := readRepoDefault(t, "commands/k-enforcement.md")
	line := "include: _shared/erster-schritt.md\n"
	if !strings.Contains(file, "\n"+line) {
		t.Fatalf("k-enforcement.md ohne %q", line)
	}
	if strings.Index(file, line) > strings.Index(file, "## Schritt 2") {
		t.Error("das Include steht nicht im ersten Schritt")
	}
	module := strings.TrimSpace(string(inventoryBody(t, readRepoDefault(t, "commands/_shared/erster-schritt.md"))))
	wantText := strings.Replace(file, line, module+"\n", 1)
	for _, needle := range []string{
		"`documents.rules`", "`k_playbook_doc_read`", "gegen `HEAD`, samt\nunverfolgten",
		"git -C \"$ROOT\" diff --name-only HEAD", "git -C \"$ROOT\" ls-files --others --exclude-standard",
		"bin/k-check\" --mode changed", "--target-root \"<context.project.repoRoot>\"", "--files-from", "k-playbook:/rules/docs-sync.md",
		"k-playbook:/instructions/on-demand/doku-auftraege.md", "nach Freigabe", "`k_playbook_doc_create` mit `kind: docs`",
		"Kephalaion `write`", "`^sitzung: <session>$`", "eingereiht", "nachzuziehen", "Folge-Command: keiner",
	} {
		if !strings.Contains(file, needle) {
			t.Errorf("k-enforcement.md nennt %q nicht", needle)
		}
	}
	for _, stale := range []string{"_shared/context.md", "catalogs.rules", "catalogs.checks", "extracted/", "ks-enforcement",
		"k-playbook context", "Check k-playbook enforcement rules", "nicht ausgeführt"} {
		if strings.Contains(file, stale) {
			t.Errorf("k-enforcement.md trägt noch %q", stale)
		}
	}

	// Der Skill ks-enforcement ist entfallen; die Regeln wendet jede Sitzung
	// über die always-Anweisung an.
	if _, ok := skillSources(filepath.Join("..", "..", "..", "share", "skills"))["enforcement"]; ok {
		t.Error("share/skills/enforcement ist noch da")
	}

	got := mustResolveDoc(t, layers, ref)
	if got.Content != wantText || len(got.Problems) != 0 || len(got.Includes) != 1 {
		t.Errorf("eingesetzter Text weicht ab: problems %v, includes %+v", got.Problems, got.Includes)
	}
}

// Die always-Anweisung „Regeln anwenden“ (Task 124) steht im zusammengebauten
// Text zwischen Workbench und Tasks und verweist auf die Doku-Aufträge und
// /k-enforcement; /k-task-run reiht im Hauptkontext ein, /k-danke zählt
// „eingereiht“ als erledigt.
func TestDefaultsRegelnAnwenden(t *testing.T) {
	layers := repoDefaultsLayers(t)
	part := readRepoDefault(t, "instructions/always/220-regeln-anwenden.md")
	if !strings.HasPrefix(part, "---\ntitle: Regeln anwenden\n---\n") {
		t.Errorf("220-regeln-anwenden.md ohne title:\n%.120s", part)
	}
	for _, needle := range []string{"`documents.rules`", "`k_playbook_doc_read`", "k-playbook:/rules/docs-sync.md",
		"k-playbook:/instructions/on-demand/doku-auftraege.md", "`/k-enforcement`"} {
		if !strings.Contains(part, needle) {
			t.Errorf("220-regeln-anwenden.md nennt %q nicht", needle)
		}
	}
	always := mustResolveDoc(t, layers, "k-playbook:/instructions/always/").Content
	workbench, rules, tasks := strings.Index(always, "## Die Workbench"), strings.Index(always, "## Regeln anwenden"),
		strings.Index(always, "## Arbeiten in Tasks")
	if workbench < 0 || rules < workbench || tasks < rules {
		t.Errorf("Reihenfolge im zusammengebauten Text: Workbench %d, Regeln %d, Tasks %d", workbench, rules, tasks)
	}

	run := readRepoDefault(t, "commands/k-task-run.md")
	for _, needle := range []string{"`CHANGED_PATHS`", "git diff --cached --name-only \"<SNAPSHOT>\"",
		"k-playbook:/instructions/on-demand/doku-auftraege.md", "**Doku-Nachzug.**", "die Sub-Agenten laden das Briefing nicht"} {
		if !strings.Contains(run, needle) {
			t.Errorf("k-task-run.md nennt %q nicht", needle)
		}
	}
	danke, err := os.ReadFile(filepath.Join("..", "..", "..", "share", "commands", "k-danke.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(danke), "Der Ausgang **„eingereiht“** zählt als erledigt") ||
		!strings.Contains(string(danke), "Auffangnetz") {
		t.Error("k-danke.md zählt „eingereiht“ nicht als erledigt oder ist kein Auffangnetz mehr")
	}
}
