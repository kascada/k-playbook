package project

import (
	"context"
	"os"
	"path/filepath"
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

// Die drei Command-Module in defaults/commands/_shared/ sind über die Sorte
// commands erreichbar, tragen Frontmatter title und description und kommen als
// Einzeldatei unverändert; context.md bleibt wenige Zeilen. includes/ trägt
// weiterhin keinen Baustein — wie seine README sagt.
func TestDefaultsCommandsSharedIncludes(t *testing.T) {
	layers := repoDefaultsLayers(t)

	for _, name := range []string{"context.md", "gh.md", "base-tools.md"} {
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

	shared := mustResolveDoc(t, layers, "k-playbook:/commands/_shared/context.md")
	body := strings.TrimSpace(string(inventoryBody(t, shared.Content)))
	if lines := strings.Count(body, "\n") + 1; lines > 8 {
		t.Errorf("context.md hat %d Zeilen Rumpf, soll wenige bleiben:\n%s", lines, body)
	}
	for _, want := range []string{"`k_playbook_briefing`", "`k-playbook briefing --json`", "`documents.always.content`"} {
		if !strings.Contains(body, want) {
			t.Errorf("context.md nennt nicht %s", want)
		}
	}
	if strings.Contains(body, "k-playbook context") {
		t.Errorf("context.md ruft noch `k-playbook context`:\n%s", body)
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
// `include: _shared/context.md`, aufgelöst steht an der Stelle der Rumpf des
// Moduls ohne Frontmatter, roh die Include-Zeile; die Felder kommen aus dem
// Briefing, nicht mehr aus `k-playbook context`.
func TestDefaultsKUpgradeBindetContextEin(t *testing.T) {
	layers := repoDefaultsLayers(t)
	const ref = "k-playbook:/commands/k-upgrade.md"
	const includeLine = "include: _shared/context.md\n"

	file := readRepoDefault(t, "commands/k-upgrade.md")
	if !strings.Contains(file, "\n"+includeLine) {
		t.Fatalf("k-upgrade.md ohne Include-Zeile:\n%s", file)
	}
	module := strings.TrimSpace(string(inventoryBody(t, readRepoDefault(t, "commands/_shared/context.md"))))

	got := mustResolveDoc(t, layers, ref)
	want := strings.Replace(file, includeLine, module+"\n", 1)
	if got.Content != want {
		t.Errorf("eingesetzter Text:\n%q\nerwartet\n%q", got.Content, want)
	}
	if strings.Contains(got.Content, "include:") || strings.Contains(got.Content, "title: Briefing laden") {
		t.Errorf("Include-Zeile oder Frontmatter des Moduls im eingesetzten Text:\n%s", got.Content)
	}
	if !strings.Contains(got.Content, "`context.playbook.dir`") || strings.Contains(got.Content, "k-playbook context") {
		t.Errorf("k-upgrade liest nicht aus dem Briefing:\n%s", got.Content)
	}
	if len(got.Includes) != 1 || got.Includes[0] != (DocInclude{Ref: "k-playbook:/commands/_shared/context.md", Origin: DocOriginDist, Depth: 1}) {
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
