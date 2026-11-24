package project

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
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
		"`now.date`",
		"`k-playbook:/instructions/on-demand/briefing-felder.md`",
		"`k-playbook:/guidelines/betrieb.md`",
		"`k-playbook:/instructions/on-demand/k-playbook.md`",
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

// Die Einstiegsdatei on-demand/k-playbook.md (Task 158) steht in der Liste
// „Auf Abruf“, wie das Briefing sie aus dem Arbeitsstand baut: mitgeliefert,
// mit Titel und Beschreibung aus dem Frontmatter. Geprüft wird über den Leser
// des Briefings (briefingEntries, darin docTitleAndDescription), nicht am
// Rohtext: Ein unquotierter Wert mit „: “ macht das Frontmatter unlesbar,
// „ #“ schneidet ihn ab — die Beschreibung fiele still weg oder bräche ab,
// der Titel käme aus der Überschrift. Die Nadeln der Beschreibung stehen an
// ihrem Anfang, in der Mitte und am Ende. Dazu: Die Datei trägt ihre fünf
// Abschnitte, und jeder Verweis, auf den sie baut, löst im Arbeitsstand auf.
func TestDefaultsEinstiegsdateiKPlaybook(t *testing.T) {
	layers := repoDefaultsLayers(t)
	const name = "k-playbook.md"
	const ref = "k-playbook:/instructions/on-demand/" + name

	entries, err := briefingEntries(layers, catalogKind{name: "on-demand", dirName: briefingOnDemandDir, suffix: ".md"})
	if err != nil {
		t.Fatalf("Auf Abruf listen: %v", err)
	}
	index := slices.IndexFunc(entries, func(entry BriefingEntry) bool { return entry.Ref == ref })
	if index < 0 {
		t.Fatalf("%s fehlt unter „Auf Abruf“: %+v", ref, entries)
	}
	entry := entries[index]
	if entry.Origin != DocOriginDist {
		t.Errorf("%s: Herkunft %q, erwartet %q", ref, entry.Origin, DocOriginDist)
	}
	if entry.Title != "Wie k-playbook aufgebaut ist" {
		t.Errorf("%s: Titel %q", ref, entry.Title)
	}
	if entry.Description == "" {
		t.Fatalf("%s: Beschreibung leer — das Frontmatter ist für den Leser des Briefings nicht lesbar", ref)
	}
	if !strings.HasPrefix(entry.Description, "Was wo liegt") || !strings.Contains(entry.Description, "AGENTS.md") ||
		!strings.HasSuffix(entry.Description, "ins Handbuch.") {
		t.Errorf("%s: Beschreibung unvollständig gelesen: %q", ref, entry.Description)
	}

	doc := mustResolveDoc(t, layers, ref)
	if doc.Disabled || len(doc.Problems) != 0 || doc.Content != readRepoDefault(t, "instructions/on-demand/"+name) {
		t.Errorf("%s: %+v", ref, doc)
	}
	// Das Frontmatter selbst trägt den Titel: Mit einem unlesbaren Kopf käme
	// derselbe Text ersatzweise aus der Überschrift.
	head, ok := frontmatterOf(doc.Content)
	if !ok {
		t.Fatalf("%s ohne Frontmatter:\n%.120s", ref, doc.Content)
	}
	if title, description := docTitleAndDescription(name, []byte("---\n"+head+"\n---\n")); title != entry.Title || description != entry.Description {
		t.Errorf("%s: Frontmatter allein liefert %q/%q", ref, title, description)
	}
	for _, heading := range []string{"### Aufbau", "### Wie Anweisungen funktionieren", "### Eine `AGENTS.md` aufteilen",
		"### Was in die Queue kommt", "### Weiterlesen"} {
		if !strings.Contains(doc.Content, "\n"+heading+"\n") {
			t.Errorf("%s: Abschnitt %q fehlt", ref, heading)
		}
	}
	for _, target := range []string{
		"k-playbook:/guidelines/betrieb.md",
		"k-playbook:/instructions/on-demand/befunde.md",
		"k-playbook:/instructions/on-demand/doku-auftraege.md",
		"k-playbook:/docs/handbook/overview.md",
		"k-playbook:/docs/handbook/manual.md",
		"k-playbook:/docs/handbook/k-playbook-format.md",
		"k-playbook:/docs/handbook/verweisschema.md",
		"k-playbook:/docs/handbook/commands.md",
		"k-playbook:/docs/handbook/task-flow.md",
		"k-playbook:/docs/handbook/installation.md",
		"k-playbook:/docs/handbook/faq.md",
	} {
		if !strings.Contains(doc.Content, "`"+target+"`") {
			t.Errorf("%s nennt nicht %s", ref, target)
		}
		mustResolveDoc(t, layers, target)
	}
}

// Das Blatt on-demand/briefing-felder.md nennt jedes Feld des Briefings:
// jeden Schlüssel der obersten Ebene, jedes Feld der Abschnitte now, project,
// kephalaion, github und links und jeden Schlüssel des Teilabrufs environment
// — als `name`, `abschnitt.name` bzw. `environment.name`. Die Schlüssel
// kommen auf demselben Weg zustande wie die Ausgabe selbst (Marshal eines
// gebauten Briefings und eines gebauten Teilabrufs), nicht aus einer von Hand
// gepflegten Liste. Optionale Felder werden gesetzt, damit sie in der Liste
// sind.
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
	briefing.Links = &ContextLinks{Healed: LinkChanges{Added: []string{"x"}}, Open: []LinkIssue{{Path: "x"}}, Note: "x"}
	briefing.Project.Message = "x"
	briefing.Kephalaion.User = "x"
	top := topLevelKeys(t, briefing)
	if len(top) < 10 {
		t.Fatalf("Briefing mit %d Feldern oben: %v", len(top), top)
	}
	for key := range top {
		if !mentions(key) {
			t.Errorf("Blatt nennt das Feld `%s` nicht", key)
		}
	}
	for section, value := range map[string]any{
		"now": briefing.Now, "project": briefing.Project, "kephalaion": briefing.Kephalaion, "github": briefing.GitHub,
		"links": briefing.Links, "work": briefing.Work,
	} {
		keys := topLevelKeys(t, value)
		if len(keys) < 1 {
			t.Fatalf("%s ohne Felder", section)
		}
		for key := range keys {
			if !mentions(section + "." + key) {
				t.Errorf("Blatt nennt `%s.%s` nicht", section, key)
			}
		}
	}

	environment := BuildBriefingEnvironment(root, root)
	environment.Message = "x"
	keys := topLevelKeys(t, environment)
	if len(keys) < 8 {
		t.Fatalf("environment mit %d Schlüsseln: %v", len(keys), keys)
	}
	for key := range keys {
		if !mentions("environment." + key) {
			t.Errorf("Blatt nennt `environment.%s` nicht", key)
		}
	}
	// Den Abschnitt context trägt das Briefing nicht mehr (Task 187): Das
	// Blatt nennt kein Feld mehr unter diesem Präfix.
	if strings.Contains(sheet, "`context.") || strings.Contains(sheet, "`context`") {
		t.Errorf("Blatt nennt noch den Abschnitt context")
	}
	// Das Blatt nennt die Fassung, die das Programm ausgibt.
	if want := "heute `\"" + BriefingSchemaVersion + "\"`"; !strings.Contains(sheet, want) {
		t.Errorf("Blatt nennt die Fassung %s nicht", want)
	}
}

// Kein mitgelieferter Text nennt das Feld playbook der Kontextausgabe mehr
// als Wurzel, Leseort oder Ausschluss (Task 157): weder context.playbook.dir
// noch playbook.dir, weder unter defaults/ noch unter share/. Die Wurzel
// ist project.dir; ein liegengebliebener Clone steht als
// fester Pfad <project.dir>/k-playbook/ da, nicht als Feld. Nur die
// Liste der Fassungen im Handbuch (mcp.md) nennt context.playbook noch, als
// das, was Fassung 6 trug.
func TestMitgelieferteTexteNennenKeinPlaybookDir(t *testing.T) {
	assertShippedTextsLack(t, []string{"playbook.dir", "playbookDir"})
}

// Kein mitgelieferter Text nennt das frühere projekteigene Verzeichnis oder
// das Kontextfeld mit seinem Ort mehr (Task 159), weder unter defaults/ noch unter
// share/: Jeder Ablageort ist ein keph:-Verweis oder liegt unter .k-playbook/.
// Der Verzeichnisname ist hier aus Teilen zusammengesetzt, damit die Suche
// danach im Quell-Repo selbst keinen Treffer hat.
func TestMitgelieferteTexteNennenKeinLokalesVerzeichnis(t *testing.T) {
	assertShippedTextsLack(t, []string{"k-playbook-" + "local", "local" + ".dir"})
}

// Kein mitgelieferter Text liest noch aus dem Abschnitt context oder nennt den
// context-Befehl als Weg (Task 187): Anweisungen, Leitlinien, Reviews,
// Commands und Skills unter defaults/ und share/ nennen weder `context.` als
// Feldpräfix noch `k-playbook context` bzw. `k_playbook_context` als Aufruf
// noch `k-playbook briefing --json` als Ersatzweg über die Shell — der ist der
// Klartext. Ein Zeilenumbruch zwischen den Wörtern zählt als Leerzeichen. Das
// Handbuch unter defaults/docs/handbook/ prüft dieser Test nicht: Es
// beschreibt Befehl und Werkzeug selbst und darf `--json` weiter nennen. Die
// Stellen, die `catalogs.reviews` aus der Kontextausgabe holen (die gesperrten
// /k-review und /k-audit mit _audit/review-scan-triage.md), nennen keinen der
// Aufrufe und bleiben.
func TestMitgelieferteTexteLesenNichtMehrAusContext(t *testing.T) {
	patterns := map[string]*regexp.Regexp{
		"context. als Feldpräfix":       regexp.MustCompile(`(^|[^A-Za-z0-9_./-])context\.[A-Za-z]`),
		"k-playbook context als Aufruf": regexp.MustCompile(`k-playbook context\b`),
		"k_playbook_context als Aufruf": regexp.MustCompile(`k_playbook_context`),
		"k-playbook briefing --json":    regexp.MustCompile(`k-playbook briefing --json`),
		"den Abschnitt context":         regexp.MustCompile("Abschnitt `context`|unter `context`"),
		"das Modul _shared/context.md":  regexp.MustCompile(`_shared/context\.md`),
	}
	space := regexp.MustCompile(`\s+`)
	for _, dir := range []string{"defaults", "share"} {
		root := filepath.Join("..", "..", "..", dir)
		files := 0
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel := filepath.ToSlash(strings.TrimPrefix(path, root+string(filepath.Separator)))
			if entry.IsDir() {
				// Das Handbuch zieht Etappe 6 nach; testdata sind feste Fälle
				// für die Includes.
				if dir == "defaults" && (rel == "docs/handbook" || rel == "testdata") {
					return filepath.SkipDir
				}
				return nil
			}
			if !entry.Type().IsRegular() || filepath.Ext(path) == ".go" {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			files++
			// Zeilenumbrüche zählen als Leerzeichen: `k-playbook briefing\n--json`
			// ist derselbe Aufruf.
			text := space.ReplaceAllString(string(data), " ")
			for name, pattern := range patterns {
				if loc := pattern.FindStringIndex(text); loc != nil {
					from, to := max(0, loc[0]-60), min(len(text), loc[1]+60)
					t.Errorf("%s/%s nennt %s: …%s…", dir, rel, name, text[from:to])
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("%s durchsuchen: %v", dir, err)
		}
		if files < 20 {
			t.Fatalf("%s: nur %d Dateien gelesen", dir, files)
		}
	}
	// Das Modul, das `k-playbook context` rief, gibt es nicht mehr.
	if _, err := os.Stat(filepath.Join("..", "..", "..", "share", "commands", "_shared", "context.md")); !os.IsNotExist(err) {
		t.Errorf("share/commands/_shared/context.md liegt noch (%v)", err)
	}
	// Der Ersatzweg über die Shell ist in jedem Text derselbe Satz wie im
	// Modul erster-schritt.md.
	const fallback = "ersatzweise in der Shell `k-playbook briefing`. Ist k-playbook"
	for _, rel := range []string{
		"defaults/commands/_shared/erster-schritt.md",
		"share/commands/k-audit.md", "share/commands/k-danke.md", "share/commands/k-gui.md", "share/commands/k-review.md",
		"share/commands/k-test-check.md", "share/commands/k-todo.md", "share/commands/k-verlauf.md",
		"share/commands/k-vscode-project-color.md", "share/skills/befunde/SKILL.md",
	} {
		data, err := os.ReadFile(filepath.Join("..", "..", "..", filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("%s lesen: %v", rel, err)
		}
		if !strings.Contains(space.ReplaceAllString(string(data), " "), fallback) {
			t.Errorf("%s nennt den Ersatzweg über den Klartext nicht", rel)
		}
	}
	// Die Ausnahme: /k-review holt catalogs.reviews weiter aus der
	// Kontextausgabe, bis es neu gebaut ist.
	if review := readRepoShare(t, "commands/k-review.md"); !strings.Contains(review, "Take `catalogs.reviews` from the context output.") {
		t.Errorf("k-review.md holt catalogs.reviews nicht mehr aus der Kontextausgabe")
	}
	// Die Ausnahme gilt nur für catalogs.reviews: /k-audit und das Modul
	// review-scan-triage.md holen den Katalog an keiner Stelle selbst aus der
	// Kontextausgabe und nennen sie deshalb in keiner Schreibweise; die Felder
	// von Schritt 1 in /k-audit (project.dir, now.date) kommen aus dem Briefing.
	contextOutput := regexp.MustCompile(`(?i)context-ausgabe|kontextausgabe|context output|context-aufruf`)
	for _, rel := range []string{"commands/k-audit.md", "commands/_audit/review-scan-triage.md"} {
		if loc := contextOutput.FindString(readRepoShare(t, rel)); loc != "" {
			t.Errorf("share/%s nennt die Kontextausgabe (%q), holt aber catalogs.reviews nicht daraus", rel, loc)
		}
	}
	if audit := space.ReplaceAllString(readRepoShare(t, "commands/k-audit.md"), " "); !strings.Contains(audit, "Löse aus dem Briefing: - `RESOLVED_PROJECT_DIR` = `project.dir` - `TODAY` = `now.date`") {
		t.Errorf("k-audit.md löst project.dir und now.date in Schritt 1 nicht aus dem Briefing auf")
	}
}

// readRepoShare liest eine Datei aus share/ des Repos.
func readRepoShare(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "share", filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("share/%s lesen: %v", rel, err)
	}
	return string(data)
}

// assertShippedTextsLack liest jede Datei unter defaults/ und share/ und
// meldet jede Zeile, die eine der Zeichenfolgen aus gone trägt.
func assertShippedTextsLack(t *testing.T, gone []string) {
	t.Helper()
	for _, dir := range []string{"defaults", "share"} {
		root := filepath.Join("..", "..", "..", dir)
		files := 0
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !entry.Type().IsRegular() {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			files++
			for number, line := range strings.Split(string(data), "\n") {
				for _, needle := range gone {
					if strings.Contains(line, needle) {
						t.Errorf("%s:%d nennt %s: %s", path, number+1, needle, strings.TrimSpace(line))
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("%s durchsuchen: %v", dir, err)
		}
		if files < 20 {
			t.Fatalf("%s: nur %d Dateien gelesen", dir, files)
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
	// Der Ersatzweg über die Shell ist der Klartext (Task 187): Nur für ihn
	// ist gesichert, dass er in einem Stück ankommt.
	for _, want := range []string{"`k_playbook_briefing`", "ersatzweise in der Shell `k-playbook briefing`.", "`documents.always.content`"} {
		if !strings.Contains(body, want) {
			t.Errorf("erster-schritt.md nennt nicht %s", want)
		}
	}
	if strings.Contains(body, "--json") {
		t.Errorf("erster-schritt.md nennt noch --json:\n%s", body)
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
// 2026-10-25), und der Text nimmt auf ihre Ausgabe Bezug. Sie setzt
// K_PLAYBOOK_VIA_COMMAND=1 vor den Aufruf, damit der Hinweis auf /k-upgrade
// nicht mitten in /k-upgrade steht; die Form ohne env ist in beiden Clients
// gemessen (2026-11-11, Task 142), ebenso der Ersatzaufruf im Text.
func TestShareKUpgradeHuelleFuehrtUpgradeAus(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "share", "commands", "k-upgrade.md"))
	if err != nil {
		t.Fatal(err)
	}
	hull := string(data)
	if !strings.Contains(hull, "\n!`K_PLAYBOOK_VIA_COMMAND=1 k-playbook upgrade 2>&1`\n") {
		t.Errorf("Hülle ohne Einbettung von k-playbook upgrade:\n%s", hull)
	}
	if !strings.Contains(hull, "allowed-tools: [Bash,") {
		t.Errorf("Claude Code führt die Einbettung nur mit Bash in allowed-tools aus:\n%s", hull)
	}
	if text := readRepoDefault(t, "commands/k-upgrade.md"); !strings.Contains(text, "Ausgabe von\n`k-playbook upgrade`") {
		t.Errorf("der Text nimmt keinen Bezug auf die Ausgabe der Hülle:\n%s", text)
	}
	if text := readRepoDefault(t, "commands/k-upgrade.md"); !strings.Contains(text, "\n    K_PLAYBOOK_VIA_COMMAND=1 k-playbook upgrade\n") {
		t.Errorf("der Ersatzaufruf setzt K_PLAYBOOK_VIA_COMMAND=1 nicht:\n%s", text)
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
	if !strings.Contains(file, "\n    K_PLAYBOOK_VIA_COMMAND=1 k-playbook upgrade\n") || strings.Contains(file, "k-playbook context") {
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
	includes := []string{"erster-schritt.md", "docs.md", "inventory-config.md"}
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
		{Ref: "k-playbook:/commands/_shared/inventory-config.md", Origin: DocOriginDist, Depth: 1},
	}
	if len(got.Includes) != 3 || got.Includes[0] != wantIncludes[0] || got.Includes[1] != wantIncludes[1] || got.Includes[2] != wantIncludes[2] {
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
		"mcp__kephalaion__list", "mcp__kephalaion__read",
		// Task 167: Der Verarbeiter legt zerlegte Aufträge an und ergänzt sie.
		"mcp__k-playbook__k_playbook_doc_create", "mcp__kephalaion__write"} {
		if !strings.Contains(hull, tool) {
			t.Errorf("Hülle erlaubt %s nicht", tool)
		}
	}
	// Seit Task 167 nimmt /k-docs-code kein Argument mehr: Die Hülle trägt
	// keinen argument-hint, und die description nennt die Aufträge statt des
	// Zusatzpfads.
	description := ""
	for _, line := range strings.Split(hull, "\n") {
		if value, ok := strings.CutPrefix(line, "description: "); ok {
			description = value
		}
	}
	if strings.Contains(hull, "argument-hint") || strings.Contains(description, "ein Argument ergänzt") ||
		!strings.Contains(description, "Doku-Aufträge") || !strings.Contains(description, "Nimmt kein Argument") ||
		!strings.Contains(description, "keph:/docs/generated/code/") || !strings.Contains(description, "/k-docs code") {
		t.Errorf("Hülle noch mit argument-hint oder description ohne die Aufträge: %q", description)
	}
	// Ein „: “ in der unquotierten description machte das Frontmatter unlesbar.
	if strings.Contains(description, ": ") || strings.Contains(description, " #") {
		t.Errorf("description mit „: “ oder „ #“: %q", description)
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
	// Task 180: Kern und Module (_docs/, _docs-code/) zusammen tragen den Text;
	// was in ein Modul oder ein Programm-Kommando gewandert ist, wird dort gesucht.
	bundle := docsCommandBundle(t, "k-docs-code.md", "_docs", "_docs-code")
	for _, needle := range []string{
		"k_playbook_docs_checkout", "k_playbook_docs_publish", "keph:/docs/generated/code/", ".k-playbook/",
		"discard: true", "allow_hints",
		"generated: { by: k-docs-code", "questions:", "hints", "kephalaion_unavailable", "outcome_unknown",
		"generated.by: ks-overlay-repo-analyse", "unverändert liegen",
		"k_playbook_docs_exclude_add", "NEW_EXCLUDES", "Modul zum Scope", "Kopf in der Ablage oben",
		"<die Scope-Zeilen aus dem Modul zum Scope>",
	} {
		if !strings.Contains(bundle, needle) {
			t.Errorf("k-docs-code.md samt Modulen nennt %q nicht", needle)
		}
	}
	// Kopf und Scope stehen seit Task 120 allein in _shared/docs.md; der Text
	// des Commands verweist darauf und wiederholt sie nicht.
	for _, repeated := range []string{
		"SCAN_ROOT = project.repoRoot", "**Versteckte Pfade**", "`tests/fixtures/`", "Aus K-PLAYBOOK.yaml",
		"environment.docs.exclude", "environment.docs.problems", "„Neu laden“", "(etwa `exclude: [docs/]`)", "k-playbook docs exclude add",
		"`verified:` trägt nur ein Mensch", "`title` und `description` sind Pflicht", "eine Frage, eine\n  Datei",
	} {
		if strings.Contains(file, repeated) {
			t.Errorf("k-docs-code.md wiederholt %q aus _shared/docs.md", repeated)
		}
	}
	// Task 167: Die Aufträge steuern den Lauf. Der Text weist ein Argument ab,
	// liest die Aufträge (roh, zerlegt, wartend), legt zusammen, zerlegt in die
	// Queue, arbeitet je Auftrag und schließt nach „Abschluss eines Auftrags“.
	squashed := squashSpace(bundle)
	for _, needle := range []string{
		"**Kein Argument.** `/k-docs-code` nimmt kein Argument", "bevor etwas gesucht, geholt oder geschrieben wird",
		"/k-docs-code nimmt kein Argument: „<$ARGUMENTS>“", "**Kein `EXTRA_PATH`:**", "**Die Dokumente kommen aus den Aufträgen.**",
		"**Gelesen wird innerhalb von `SCAN_ROOT`:**", "**Die Ausschlüsse gelten** auch für die Pfade eines Auftrags",
		"k-playbook:/instructions/on-demand/doku-auftraege.md",
		"`RAW_ORDERS`", "`SPLIT_ORDERS`", "`WAITING`", "`WORK_ORDERS`", "`WORKED`",
		"**Nichts zu tun.**", "nenne `/k-docs code` als Weg zum vollen Lauf", "`/k-docs code` als Weg, sofort zu laufen",
		"wartet <datei> — noch <restzeit>", "Lies **alle** Dokumente in `WORK_DIR`, bevor du den Code analysierst — auch im Teil-Lauf",
		"**Zuordnung Pfad → Dokument.**", "`## Key places in the code`", "Der Auftrag tritt an die Stelle von „im Scope“",
		"**Zerlegen.**", "**`neu` und `entfällt` bestätigt der Nutzer, `umschreiben` nicht.**",
		"**In die Queue schreiben.**", "## Schritt 5 — Schreiben, je Auftrag", "## Schritt 6 — publish und Aufträge schließen",
		"**Abschluss eines Auftrags.**", "mit `created`, `written`, `deleted` oder `unchanged` meldet",
		"Ein Auftrag, der nicht in `WORKED` steht, bleibt deshalb offen", "**Neu auschecken.**",
		"`$ARGUMENTS` ist nicht leer → kein Lauf", "**Ohne Auftrag laufen.**", "**Einen wartenden Auftrag anfassen**",
		"**einen Auftrag zu früh schließen.**", "keph:/queue/docs/done/", "`erzeuger: code`",
		// Aufträge und alte Fassung sind Hinweise, keine Quelle: geschrieben wird
		// nur, was der Stand des Preflights bestätigt, auch beim Text der Sub-Agenten.
		"## Nur was der Code jetzt zeigt", "**Vor dem Einfügen prüfen.**",
		"Zugeordnet wird nach dem heutigen Stand der Pfade, nicht nach dem `anlass`",
		"**aus dem Auftrag oder der alten Fassung schreiben**",
		// Task 180: Scope und Aufträge laufen über die Programm-Kommandos.
		"`k_playbook_docs_scope`", "`k_playbook_docs_orders`", "`k_playbook_docs_orders_split`", "`rawOrders`", "`nothingToDo`",
		"`k-playbook docs orders <code|tools> --json`", "`k-playbook docs orders split --json`", "`/k-upgrade`",
	} {
		if !strings.Contains(squashed, needle) {
			t.Errorf("k-docs-code.md samt Modulen nennt %q nicht", needle)
		}
	}
	// Die Regeln selbst — Wartezeit, Zusammenlegen, die vier Teilschritte des
	// Schreibens in die Queue — stehen nur im Programm und in der Instruktion zum
	// Auftragskopf, nicht in den Texten.
	for _, rule := range []string{"2 Stunden", "**Zusammenlegen.**", "`k_playbook_doc_create` mit `kind: docs`", "`code-<dokument ohne .md>`",
		"Kephalaion `write` mit `base_revision`", "wenn ein Pfad des einen gleich einem Pfad des anderen ist oder darin liegt",
		"`now.timestamp`", "Abschnitt „Wartezeit“"} {
		if strings.Contains(squashed, rule) {
			t.Errorf("k-docs-code.md samt Modulen trägt die Programmregel %q", rule)
		}
	}
	// Vom alten Lauf — Argument als Zusatzpfad, ORDERS aus dem Preflight,
	// Abschluss nach einem aufgeräumten publish — bleibt im Text nichts.
	for _, stale := range []string{"`EXTRA_PATH` aus `$ARGUMENTS`", "`ORDERS`", "<die Zeilen „Aufträge“ aus dem Abschnitt „Doku-Aufträge“>",
		"`SCAN_ROOT`, `EXTRA_PATH`", "liegt außerhalb von `project.dir`", "argument-hint"} {
		if strings.Contains(squashSpace(file), stale) {
			t.Errorf("k-docs-code.md trägt noch %q", stale)
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
	for _, needle := range []string{"mehrere\n  Schreiber", "k_playbook_docs_checkout", "`hints`", "`questions`", "aus ihrer Vorlage `k-playbook:/docs/<pfad>/README.md`", "`k-playbook docs readme`", "Indizieren",
		"`docs.exclude`", "`environment.docs.exclude`", "`k_playbook_docs_exclude_add`"} {
		if !strings.Contains(shared, needle) {
			t.Errorf("_shared/docs.md nennt %q nicht", needle)
		}
	}
	assertSharedDocsKopf(t, shared)
	assertDocsScopeModul(t)
}

// assertSharedDocsKopf prüft den Abschnitt, den Task 120 in _shared/docs.md
// gesammelt hat: den einen Kopf aller Dokumente unter docs/ mit seinen
// Feldern. Seit Task 180 trägt die Datei weder „Scope“ noch „Doku-Aufträge“:
// Beides steht als Aufruf der Programm-Kommandos in _docs/scope.md und
// _docs/auftraege.md.
func assertSharedDocsKopf(t *testing.T, shared string) {
	t.Helper()
	kopf := strings.Index(shared, "\n### Der Kopf\n")
	if kopf < 0 {
		t.Fatalf("_shared/docs.md ohne den Abschnitt „Der Kopf“")
	}
	for _, gone := range []string{"\n### Scope\n", "\n### Doku-Aufträge\n"} {
		if strings.Contains(shared, gone) {
			t.Errorf("_shared/docs.md trägt noch den Abschnitt %q", strings.TrimSpace(gone))
		}
	}
	head := shared[kopf:]
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
	// Die Regeln eines schreibenden Laufs stehen im Modul, die Ablage verweist darauf.
	if !strings.Contains(shared, "k-playbook:/commands/_docs/schreiblauf.md") {
		t.Error("_shared/docs.md verweist nicht auf _docs/schreiblauf.md")
	}
}

// assertDocsScopeModul prüft _docs/scope.md: Aufruf von docs scope, die Zeilen
// des Preflights, NEW_EXCLUDES und das Speichern; die Ausschlussregeln selbst
// stehen nur im Programm.
func assertDocsScopeModul(t *testing.T) {
	t.Helper()
	scopeText := squashSpace(readRepoDefault(t, "commands/_docs/scope.md"))
	for _, needle := range []string{
		"Der Scope gilt nur für die Erzeuger, die den Code durchsuchen: `/k-docs-code` und `/k-docs-tools`",
		"`/k-doc-inventory` liest seine Quellen nach seinem eigenen Vertrag", "`k_playbook_docs_scope`",
		"`k-playbook docs scope [<pfad>] [--check <pfad>…] --json`", "`EXTRA_PATH`", "`SCAN_ROOT`", "**Stand.**", "**Preflight.**",
		"Immer außen   k-playbook/, .k-playbook/, keph-backup/, .git/", "`NEW_EXCLUDES`", "**Ausschlüsse speichern.**",
		"`k_playbook_docs_exclude_add`", "„Neu laden“", "`rejected`", "`unsupported`", "`/k-upgrade`",
	} {
		if !strings.Contains(scopeText, squashSpace(needle)) {
			t.Errorf("_docs/scope.md nennt %q nicht", needle)
		}
	}
	for _, rule := range []string{"`tests/fixtures/`", "node_modules", "**Weitere Ausschlüsse**", "**Immer ausgeschlossen**", "context.playbook.dir"} {
		if strings.Contains(scopeText, rule) {
			t.Errorf("_docs/scope.md trägt die Programmregel %q", rule)
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
	if !strings.Contains(description, "keph:/docs/generated/libs/") {
		t.Errorf("description nennt nicht keph:/docs/generated/libs/: %q", description)
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
	// Task 180: Kern und Module (_docs/, _docs-tools/) zusammen tragen den Text.
	bundle := docsCommandBundle(t, "k-docs-tools.md", "_docs", "_docs-tools")
	for _, needle := range []string{
		"k_playbook_docs_checkout", "k_playbook_docs_publish", "`erzeuger: libs`", "keph:/docs/generated/libs/",
		"keph:/docs/generated/versions/inventory.md", "`generated.stats.entries`", "`k-playbook inventory`",
		"keinen Lauf und keinen eigenen Scan", "Kandidaten sind alle Gegenstände des Inventars",
		"begrenzt nur dieses Zählen, nicht die Liste", "generated: { by: k-docs-tools", "`k-tools-scan`",
		"Modul zum Scope", "Kopf in der Ablage oben", "<die Scope-Zeilen aus dem Modul zum Scope>",
		"resource: keph:/docs/generated/versions/inventory.md", "item: <gegenstand>", "version: <version>",
		"go-github.com-yuin-goldmark", "`k-playbook:/docs/handbook/version-inventory.md`", "Known-Pitfall-Liste", "**höchstens drei**",
		"vorhanden", "nicht\nvorausgewählt", "discard: true", "allow_hints", "kephalaion_unavailable", "outcome_unknown",
		"unverändert liegen",
		// Task 180: Scope und Aufträge über die Programm-Kommandos, Preflight-Zeile der Aufträge.
		"`k_playbook_docs_scope`", "`k_playbook_docs_orders`", "`erzeuger: tools`", "`ORDERS`", "Aufträge      <N> offen | keine",
		"**Nichts schreiben, bevor die Auswahl bestätigt ist.**", "`/k-upgrade`",
	} {
		if !strings.Contains(bundle, squashSpace(needle)) {
			t.Errorf("k-docs-tools.md samt Modulen nennt %q nicht", needle)
		}
	}
	for _, stale := range []string{
		"k-playbook context", "/k-docs-index", "_docs/tools", ".claude/commands/", "LIBS_DIR", "Vorübergehend",
		"type: Tool Reference", "last-reviewed:", "severity:", "version-pin:", "lib: ", "## Offene Fragen",
		"SCAN_ROOT = project.repoRoot", "**Versteckte Pfade**", "Transitive-only",
		"Abschnitt „Scope“", "Abschnitt „Doku-Aufträge“",
	} {
		if strings.Contains(bundle, stale) {
			t.Errorf("k-docs-tools.md samt Modulen trägt noch %q", stale)
		}
	}
	// Das Speichern der Ausschlüsse steht im Modul zum Scope, nicht im Kern.
	if strings.Contains(file, "„Neu laden“") {
		t.Error("k-docs-tools.md wiederholt das Speichern der Ausschlüsse aus _docs/scope.md")
	}

	// Die Vorlage des Steckbriefs (Modul _docs-tools/schreiben.md) belegt nur
	// Felder des gemeinsamen Kopfs.
	module := readRepoDefault(t, "commands/_docs-tools/schreiben.md")
	start := strings.Index(module, "```markdown\n---\n")
	if start < 0 {
		t.Fatal("_docs-tools/schreiben.md ohne Vorlage mit Frontmatter")
	}
	block, ok := inventory.FrontmatterBlock([]byte(module[start+len("```markdown\n"):]))
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
// Erzeuger Inventar und Tools weiter, in dieser Reihenfolge; vom alten
// Verteiler über die Platte bleibt nichts. Seit Task 167 startet er
// /k-docs-code nicht mehr: Für code und alle reiht er den vollen Lauf als
// Doku-Auftrag mit pfade: . ein, ein Rest nach code wird abgewiesen.
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
		"mcp__k-playbook__k_playbook_docs_publish", "mcp__k-playbook__k_playbook_docs_readme", "mcp__k-playbook__k_playbook_docs_exclude_add",
		"mcp__kephalaion__list", "mcp__kephalaion__read",
		// Task 167: /k-docs legt den Auftrag für den vollen Lauf an und schreibt ihn.
		"mcp__k-playbook__k_playbook_doc_create", "mcp__kephalaion__write"} {
		if !strings.Contains(hull, tool) {
			t.Errorf("Hülle erlaubt %s nicht", tool)
		}
	}
	if strings.Contains(hull, "knowledge") {
		t.Error("Hülle erlaubt noch Werkzeuge der Wissensablage")
	}
	// Der Zusatzpfad gilt nur noch für tools und alle; die description nennt
	// das Einreihen statt des Laufs über Inventar, Code und Tools.
	description := ""
	for _, line := range strings.Split(hull, "\n") {
		if value, ok := strings.CutPrefix(line, "description: "); ok {
			description = value
		}
	}
	if !strings.Contains(hull, "\nargument-hint: \"[status|inventory|code|tools|alle] [zusatz-pfad nur bei tools und alle]\"\n") {
		t.Errorf("argument-hint nennt den Zusatzpfad nicht nur für tools und alle:\n%s", hull)
	}
	if strings.Contains(description, "Inventar, Code und Tools nacheinander") ||
		!strings.Contains(description, "reiht den vollen Lauf der Code-Doku als Doku-Auftrag ein") ||
		!strings.Contains(description, "mit alle laufen Inventar und Tools nacheinander, die Code-Doku wird eingereiht") ||
		strings.Contains(description, ": ") || strings.Contains(description, " #") {
		t.Errorf("description nennt das Einreihen nicht oder ist nicht als YAML-Wert lesbar: %q", description)
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
		"nacheinander `inventory` und `tools`, in genau dieser Reihenfolge", "Es wurde nichts geschrieben",
		"kephalaion_unavailable", "/k-upgrade",
		// Task 137: README gegen die Vorlagen — read-only im Statuslauf, geschrieben mit alle.
		"`k_playbook_docs_readme`", "`check: true`", "`k-playbook docs readme --check --json`", "`missing`", "`differs`",
		"ohne `check` (ersatzweise `k-playbook docs readme --json`)", "Verzeichnisse ohne Vorlage",
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
	// Task 167: code und alle reihen ein, statt /k-docs-code zu starten; der
	// Bericht trennt rohe, wartende und zerlegte Aufträge.
	squashed := squashSpace(file)
	for _, needle := range []string{
		"Doku aus dem Code: vollen Lauf einreihen", "<N> Aufträge offen → /k-docs-code",
		"code: <N> roh (<N> warten noch), <N> zerlegt — ein Lauf ist unterbrochen",
		"`^(erzeuger|entstanden|dokument): |^\\s*- \\.$`", "`updated`", "`k_playbook_docs_orders`", "`remainingMinutes`",
		"**Vollen Lauf einreihen.**",
		"höchstens einen rohen Auftrag für `code` mit `pfade: .`", "wird nicht verdoppelt",
		"nimmt sie alle auf, auch die wartenden", "wenn es unter `keph:/docs/generated/code/` noch kein Dokument gibt",
		"`k_playbook_docs_order_add`", "`k-playbook docs order add code --pfad . --anlass", "`covered`", "`skipped`",
		"`/k-docs-code` startest du nicht", "/k-docs code nimmt kein Argument: „<rest>“", "reihst nichts ein",
		"`code` wird nur eingereiht, die Code-Doku läuft hier nicht", "Der Rest der Argumente geht nur an `tools`",
		"Die Code-Doku ist noch nicht gelaufen", "Folge-Command: `/k-docs-code`", "**`/k-docs-code` starten.**",
	} {
		if !strings.Contains(squashed, needle) {
			t.Errorf("k-docs.md nennt %q nicht", needle)
		}
	}
	for _, stale := range []string{
		"befolge ihn mit dem Rest der Argumente. - `tools`", "Der Rest der Argumente geht an `code`",
		"1, 2 und 3 nacheinander", "→ /k-docs-code <N> Aufträge offen", "2 Stunden",
		// Task 180: Die Anlage von Hand ist ein Aufruf des Programms.
		"`code-voller-lauf`", "Kephalaion `write` mit `collection`", "`k_playbook_doc_create` mit `kind: docs`",
	} {
		if strings.Contains(squashed, stale) {
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
	if got.Parts[0].Origin != DocOriginDist || len(got.Problems) != 0 || len(got.Includes) != 3 ||
		got.Includes[0].Ref != "k-playbook:/commands/_shared/erster-schritt.md" || got.Includes[1].Ref != "k-playbook:/commands/_shared/docs.md" ||
		got.Includes[2].Ref != "k-playbook:/commands/_docs/schreiblauf.md" {
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
	for _, stale := range []string{"/k-docs-index", "_shared/context.md", "k-playbook context", ".claude/skills/", "PLAYBOOK.md"} {
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
	if !strings.Contains(squashSpace(file), "Den Scope der Docs-Erzeuger (`k-playbook docs scope`) wendet dieser Skill nicht an") {
		t.Error("Skill-Text sagt nicht, dass er den Scope der Docs-Erzeuger nicht anwendet")
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

// /k-upgrade 3.8 löscht seit Task 156 einen alten Clone k-playbook/ ganz und
// ohne Rückfrage — nur ein echtes Verzeichnis mit eigenem .git-Verzeichnis,
// nach dem Nennen dessen, was nicht vom Repo kam — und streicht danach die
// Zeilen, die k-playbook/ ausnehmen, aus jeder .gitignore vom
// Projektverzeichnis bis zur Repo-Wurzel, sobald das Verzeichnis weg ist. Die
// Vorbedingung steht für sich: Die Zeile zum Programm-Stand muss „neuestes
// Release“ nennen. 3.8 ist die einzige Umstellung und behält ihre Nummer;
// Schritt 3, Schritt 4, Einleitung und Hülle nennen sie.
func TestDefaultsKUpgradeCloneLoeschen(t *testing.T) {
	file := readRepoDefault(t, "commands/k-upgrade.md")
	start, end := strings.Index(file, "### 3.8 — "), strings.Index(file, "## Schritt 4 — ")
	if start < 0 || end < start {
		t.Fatal("Abschnitt 3.8 nicht vor Schritt 4 gefunden")
	}
	if count := strings.Count(file, "\n### 3."); count != 1 {
		t.Fatalf("erwartet genau eine Umstellung unter Schritt 3, gefunden %d", count)
	}
	section := squashSpace(file[start:end])
	for _, needle := range []string{
		"**Anlass:** `<project.dir>/k-playbook` liegt, als Verzeichnis oder als Symlink, oder eine `.gitignore` von `project.dir` bis `project.repoRoot` trägt eine der Zeilen",
		"**Vorbedingung:** Schritt 2 hat die Zeile `Programm-Stand: nichts zu tun — …` ausgegeben, und sie nennt `neuestes Release`",
		"So schreibt sie erst ein Programm ab v0.16.39",
		"Fehlt die Zeile oder nennt sie stattdessen `Installation`, löschst und änderst du nichts",
		"`k-playbook/ löschen wartet auf den Binärwechsel`",
		"braucht den Clone für sein eigenes Upgrade",
		`test -L "<project.dir>/k-playbook" && echo SYMLINK`,
		`test -d "<project.dir>/k-playbook/.git" && ! test -L "<project.dir>/k-playbook/.git" && echo CLONE`,
		"nur ein echtes Verzeichnis, kein Symlink, in dem direkt ein Verzeichnis `.git` liegt",
		"`.git` ist eine Datei wie in einem Submodul oder Worktree",
		"`k-playbook/ ist ein Symlink, nichts gelöscht`",
		"`k-playbook/ ist kein Clone, nichts gelöscht: <grund>`",
		"auch mit unverfolgten, geänderten oder ignorierten Dateien und mit eigenen Commits darin",
		`git -C "<project.dir>/k-playbook" status --porcelain --ignored`,
		`git -C "<project.dir>/k-playbook" rev-list --count HEAD --branches --not --remotes`,
		"`k-playbook/ ließ sich nicht prüfen, nichts gelöscht: <fehler>`",
		"jede Zeile des ersten Befehls, auch wenn es viele sind, und die Zahl der Commits",
		"Das ist eine Meldung, keine Vorlage: Du wartest auf keine Antwort, und eine Rückfrage gibt es auch dann nicht.",
		`chmod -R u+w -- "<project.dir>/k-playbook"`,
		`rm -rf -- "<project.dir>/k-playbook"`,
		"von `project.dir` aufwärts bis `project.repoRoot`, beide eingeschlossen",
		"`k-playbook`, `/k-playbook`, `k-playbook/`, `/k-playbook/`, `**/k-playbook` und `**/k-playbook/`",
		"`<projektpfad>/k-playbook/`, mit und ohne `/` vorn, mit und ohne `/` hinten",
		"Kommentare und alle anderen Muster bleiben stehen",
		"Existiert es nicht oder nicht mehr, gleich wer es gelöscht hat, fällt die Zeile.",
		"bleibt auch die Zeile, und die Meldung sagt warum",
		"noch ein anderes vorhandenes Verzeichnis trifft, etwa den Clone eines zweiten Projekts",
		"ohne Vorlage und ohne Rückfrage, auch in einer versionierten `.gitignore`",
		"Committet wird nicht",
		"`k-playbook/ gelöscht`",
		"andere Checkouts lassen `/k-upgrade` laufen, bevor sie mit `git add -A` committen",
		"findet `k-playbook/` sonst als unverfolgtes Verzeichnis im Arbeitsbaum",
	} {
		if !strings.Contains(section, needle) {
			t.Errorf("3.8 nennt %q nicht", needle)
		}
	}
	// Kein Befehl der Vorlage darf den Pfad ungequotet einsetzen, und keiner
	// nimmt context.playbook.dir: Das Feld fällt mit Task 157.
	for _, bare := range []string{"test -L <project.dir>", "rm -rf -- <project.dir>", "chmod -R u+w -- <project.dir>", "git -C <project.dir>", "context.playbook.dir"} {
		if strings.Contains(section, bare) {
			t.Errorf("3.8 setzt den Pfad ungequotet ein oder nennt das alte Feld: %q", bare)
		}
	}
	general := squashSpace(file[:start])
	for _, needle := range []string{
		"Ausgenommen ist ein Fall, und nur er: der Clone `k-playbook/`, den 3.8 löscht, samt der Zeilen, die 3.8 streicht, auch in einer versionierten `.gitignore`",
		"das Löschen eines alten Clones `k-playbook/` samt der Zeilen, die `k-playbook/` in einer `.gitignore` ausnehmen",
		"wenn das laufende älter ist als das neueste Release",
		"sie behält ihre Nummer",
	} {
		if !strings.Contains(general, needle) {
			t.Errorf("Einleitung oder Schritt 3 nennt %q nicht", needle)
		}
	}
	four := squashSpace(file[end:])
	for _, needle := range []string{
		"Für 3.8 die Zeilen aus Schritt 3",
		"`k-playbook/ löschen wartet auf den Binärwechsel`",
	} {
		if !strings.Contains(four, needle) {
			t.Errorf("Schritt 4 nennt %q nicht", needle)
		}
	}
	hull, err := os.ReadFile(filepath.Join("..", "..", "..", "share", "commands", "k-upgrade.md"))
	if err != nil {
		t.Fatal(err)
	}
	description := string(hull)
	for _, needle := range []string{
		"Zieht Kephalaion, Programm und Projekt nach",
		"wenn es älter ist als das neueste Release",
		"(3.8) — er löscht ohne Rückfrage einen alten Clone k-playbook/ ganz",
		"streicht aus jeder .gitignore vom Projektverzeichnis bis zur Repo-Wurzel die Zeilen, die k-playbook/ ausnehmen, sobald das Verzeichnis weg ist",
	} {
		if !strings.Contains(description, needle) {
			t.Errorf("die Hülle nennt %q nicht", needle)
		}
	}
	if strings.Contains(description, "älter ist als die Installation") {
		t.Error("die Hülle vergleicht das Programm noch mit der Installation")
	}
}

// Die Doku-Aufträge (Task 124): Die Anweisung doku-auftraege.md beschreibt
// Kopf und Lebenslauf, /k-docs zählt die offenen Aufträge, und /k-docs-tools
// listet sie im Preflight und schließt sie nach einem erfolgreichen publish
// ab — alles über die doc-Werkzeuge, die ihre Hüllen erlauben. Seit Task 167
// steuern die Aufträge für code den Lauf von /k-docs-code: zwei Reifegrade
// (roh, zerlegt), Ergänzen statt Doppel, Wartezeit für rohe Aufträge, deren
// Wert nur in doku-auftraege.md steht.
func TestDefaultsDocsAuftraege(t *testing.T) {
	layers := repoDefaultsLayers(t)

	instruction := mustResolveDoc(t, layers, "k-playbook:/instructions/on-demand/doku-auftraege.md").Content
	if !strings.HasPrefix(instruction, "---\ntitle: Doku-Aufträge\ndescription: ") {
		t.Errorf("doku-auftraege.md ohne title und description:\n%.200s", instruction)
	}
	for _, needle := range []string{
		"keph:/queue/docs/", "`k_playbook_docs_order_add`", "`k_playbook_docs_orders_split`", "erzeuger: code | tools", "pfade:", "anlass:", "sitzung:", "entstanden:",
		"Ein Auftrag je Sitzung und Erzeuger", "Kein Doppel", "Wann nicht eingereiht wird",
		"Wer ihn schreibt", "Wer ihn abschließt", "Was nicht hineingehört", "`/k-enforcement`", "`open` leer", "`cleaned` ist wahr",
		// Task 167: zwei Reifegrade für code, Ergänzen statt Doppel, Wartezeit.
		"Ein Auftrag ist **zerlegt**, wenn sein Kopf `dokument` trägt, sonst **roh**",
		"dokument: <NN>-<slug>.md", "aktion: neu | umschreiben | entfaellt", "`^dokument: `", "`pfade: .`", "genau als `  - .`",
		"so fein wie möglich", "Die Ausschlüsse von `k-playbook docs scope`",
		"**`/k-docs`** bei der Wahl `code` oder `alle`", "**Der Verarbeiter `/k-docs-code`**",
		"zählt nur rohe Aufträge", "`covered`", "`extended`", "`no_generated_docs`", "`kephalaion_unavailable`",
		"### Wartezeit", "`k-playbook docs orders code`", "`waitMinutes`",
		"**Der volle Lauf wartet nicht.**", "Für `tools` gibt es keine Wartezeit",
		"mit `created`, `written`, `deleted` oder `unchanged` meldet", "ein noch nicht abgearbeiteter Auftrag bleibt deshalb offen",
		"Den Auftrag mit `pfade: .` reiht `/k-docs` trotzdem ein", "**Ein Teil-Scope für `tools`.**",
	} {
		if !strings.Contains(squashSpace(instruction), squashSpace(needle)) {
			t.Errorf("doku-auftraege.md nennt %q nicht", needle)
		}
	}
	// Task 180: Die Regeln selbst — der Wert der Wartezeit, die Suche nach dem
	// Auftrag der Sitzung, das Schreiben mit base_revision — stehen nur im Programm.
	for _, rule := range []string{"2 Stunden", "`^sitzung: <session>$`", "Kephalaion `write` mit `base_revision`", "`stale_revision`",
		"`now.timestamp`", "`k_playbook_doc_create`"} {
		if strings.Contains(squashSpace(instruction), rule) {
			t.Errorf("doku-auftraege.md trägt die Programmregel %q", rule)
		}
	}
	// Der alte Lebenslauf für code — Lauf immer ganz, Pfade ohne Wirkung — steht
	// nicht mehr da; für tools bleibt er benannt.
	for _, stale := range []string{"Der Erzeuger läuft immer über das ganze Projekt", "Ein Auftrag engt den Lauf nicht auf seine Pfade ein"} {
		if strings.Contains(squashSpace(instruction), stale) {
			t.Errorf("doku-auftraege.md trägt noch %q", stale)
		}
	}
	// „Kein Doppel“ zählt nur rohe Aufträge und ergänzt für code den vorhandenen:
	// dieselbe Regel in /k-enforcement; /k-task-run erzählt sie seit Task 176
	// nicht mehr nach, sondern verweist beim Einreihen auf doku-auftraege.md.
	for rel, needles := range map[string][]string{
		"commands/k-enforcement.md": {"`k_playbook_docs_order_add`", "„Kein Doppel“", "„Wann nicht eingereiht wird“", "`covered`", "`skipped`",
			"`/k-upgrade`", "`k-playbook:/instructions/on-demand/doku-auftraege.md`"},
		"commands/_task-run/doku-nachzug.md": {"`k-playbook:/instructions/on-demand/doku-auftraege.md`", "„Kein Doppel“", "„Wann nicht eingereiht wird“",
			"`k_playbook_docs_order_add`", "`covered`", "`skipped`", "`/k-upgrade`"},
		"../share/commands/k-danke.md": {"`k_playbook_docs_order_add`", "„Kein Doppel“", "`covered`", "`/k-upgrade`"},
	} {
		text := squashSpace(readRepoDefault(t, rel))
		for _, needle := range needles {
			if !strings.Contains(text, needle) {
				t.Errorf("%s nennt %q nicht", rel, needle)
			}
		}
		// Das Verfahren — Suche der Sitzung, Zusammenführen der Pfade, Schreiben mit
		// base_revision — steht im Programm, nicht in diesen Texten.
		for _, stale := range []string{"`^dokument: `", "statt einen zweiten anzulegen", "Kephalaion `write` mit `base_revision`", "`k_playbook_doc_create` mit `kind: docs`"} {
			if strings.Contains(text, stale) {
				t.Errorf("%s trägt das Verfahren %q", rel, stale)
			}
		}
	}
	for _, stale := range []string{"nach einem offenen rohen Auftrag", "`^dokument: `", "Kephalaion `write` mit `base_revision`"} {
		if strings.Contains(squashSpace(taskCommandBuendel(t, "k-task-run.md", "_task-run")), stale) {
			t.Errorf("k-task-run.md erzählt „Kein Doppel“ aus doku-auftraege.md nach: %q", stale)
		}
	}

	shared := readRepoDefault(t, "commands/_shared/docs.md")
	// Der Wert der Wartezeit steht nur im Programm.
	if strings.Contains(shared, "2 Stunden") || strings.Contains(shared, "### Doku-Aufträge") {
		t.Error("_shared/docs.md trägt den Wert der Wartezeit oder den Abschnitt „Doku-Aufträge“")
	}
	// Task 180: Die Aufträge laufen über _docs/auftraege.md und `docs orders`.
	section := squashSpace(readRepoDefault(t, "commands/_docs/auftraege.md"))
	if strings.Contains(section, "2 Stunden") {
		t.Error("_docs/auftraege.md wiederholt den Wert der Wartezeit")
	}
	for _, needle := range []string{"`k_playbook_docs_orders`", "`erzeuger: code`", "`erzeuger: tools`", "`k_playbook_doc_done`", "`nothingToDo`",
		"`run`", "`waitMinutes`", "`rawOrders`", "`splitOrders`", "Bei `code` steuern die Aufträge den Lauf", "bei `tools` begründen sie ihn",
		"`wrong_dir` oder `not_found`", "`outcome_unknown`", "`/k-upgrade`", "`pfade: .`", "eine Wartezeit gibt es nicht",
		"`k-playbook docs orders <code|tools> --json`"} {
		if !strings.Contains(section, needle) {
			t.Errorf("_docs/auftraege.md nennt %q nicht", needle)
		}
	}
	// Das Handbuch beschreibt den neuen Lebenslauf: kein Argument für
	// /k-docs-code, /k-docs code reiht ein, die Sorte docs in zwei Reifegraden.
	for rel, stales := range map[string][]string{
		"docs/handbook/commands.md": {"an argument adds one path inside the project", "`alle` runs them in that order",
			"Dispatches `inventory`, `code` and `tools`"},
		"docs/handbook/mcp.md":               {"one per session and producer, written after a code change, closed by"},
		"docs/handbook/k-playbook-format.md": {"the argument of `/k-docs-code` or `/k-docs-tools`"},
	} {
		page := squashSpace(readRepoDefault(t, rel))
		for _, stale := range stales {
			if strings.Contains(page, stale) {
				t.Errorf("%s trägt noch %q", rel, stale)
			}
		}
	}
	for rel, needles := range map[string][]string{
		"docs/handbook/commands.md": {"The command takes no argument (task 167)", "it queues the full run as one raw documentation order for `code` with `pfade: .`"},
		"docs/handbook/manual.md":   {"For `code` the orders drive the run (task 167)", "`/k-docs code` queues it instead of starting the producer"},
		"docs/handbook/mcp.md":      {"raw — from a session, or from `/k-docs` for the full run with `pfade: .` — and split"},
	} {
		page := squashSpace(readRepoDefault(t, rel))
		for _, needle := range needles {
			if !strings.Contains(page, needle) {
				t.Errorf("%s nennt %q nicht", rel, needle)
			}
		}
	}
	// /k-docs-tools listet und schließt wie bisher über ORDERS; /k-docs-code
	// nimmt den Umfang aus den Aufträgen (TestDefaultsKDocsCodeIstHuelleUndText).
	tools := readRepoDefault(t, "commands/k-docs-tools.md")
	for _, needle := range []string{"`erzeuger: tools`", "`ORDERS`", "Aufträge      <N> offen | keine",
		"keph:/queue/docs/done/", "`k_playbook_docs_orders`"} {
		if !strings.Contains(tools, needle) {
			t.Errorf("k-docs-tools.md nennt %q nicht", needle)
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
// prüft den Doku-Nachzug nach 220-regeln-anwenden.md über die geänderten
// Dateien des Arbeitsbaums und reiht erst nach Freigabe einen Doku-Auftrag
// ein. _shared/context.md, die entfallene Regelprüfung, extracted/ und der
// Skill ks-enforcement kommen nicht mehr vor.
//
// Seit Task 155 ist /k-enforcement pausiert: k-check gibt es nicht mehr. Hülle
// und Text tragen oben den Hinweis „Vorübergehend nicht verfügbar“ wie
// /k-audit, die Beschreibung der Hülle beginnt damit, und keiner nennt
// k-check noch. Der Text darunter bleibt Vorlage für den Neuaufbau (Todo #52).
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
	const pause = "## Vorübergehend nicht verfügbar\n"
	if !strings.Contains(hull, "\ndescription: Vorübergehend nicht verfügbar (") ||
		!strings.Contains(hull, "\n"+pause) || strings.Index(hull, pause) > strings.Index(hull, "`"+ref+"`") ||
		!strings.Contains(squashSpace(hull), "Führe nichts von dem aus, was darunter steht") ||
		strings.Contains(hull, "k-check") {
		t.Errorf("Hülle ohne Pause wie /k-audit oder noch mit k-check:\n%s", hull)
	}
	allowed := ""
	for _, line := range strings.Split(hull, "\n") {
		if value, ok := strings.CutPrefix(line, "allowed-tools: "); ok {
			allowed = value
		}
	}
	want := "[Read, Bash, mcp__k-playbook__k_playbook_briefing, mcp__k-playbook__k_playbook_doc_read, " +
		"mcp__k-playbook__k_playbook_doc_create, mcp__k-playbook__k_playbook_docs_order_add, mcp__k-playbook__k_playbook_doc_list, " +
		"mcp__k-playbook__k_playbook_doc_grep, mcp__kephalaion__list, mcp__kephalaion__read, mcp__kephalaion__write]"
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
	if !strings.HasPrefix(file, "# k-enforcement\n\n"+pause) || !strings.Contains(squashSpace(file), "Führe nichts von dem aus, was darunter steht") {
		t.Errorf("k-enforcement.md ohne Pausenhinweis oben:\n%.400s", file)
	}
	module := strings.TrimSpace(string(inventoryBody(t, readRepoDefault(t, "commands/_shared/erster-schritt.md"))))
	wantText := strings.Replace(file, line, module+"\n", 1)
	for _, needle := range []string{
		"`k_playbook_doc_read`", "gegen `HEAD`, samt\nunverfolgten",
		"git -C \"$ROOT\" diff --name-only HEAD", "git -C \"$ROOT\" ls-files --others --exclude-standard",
		"k-playbook:/instructions/always/220-regeln-anwenden.md", "interne Umbauten ohne geänderte Semantik",
		"k-playbook:/instructions/on-demand/doku-auftraege.md", "nach Freigabe", "`k_playbook_docs_order_add`",
		"`covered`", "`skipped`", "eingereiht", "nachzuziehen", "Folge-Command: keiner",
	} {
		if !strings.Contains(file, needle) {
			t.Errorf("k-enforcement.md nennt %q nicht", needle)
		}
	}
	for _, stale := range []string{"_shared/context.md", "documents.rules", "k-playbook:/rules/", "catalogs.checks", "extracted/", "ks-enforcement",
		"k-playbook context", "Check k-playbook enforcement rules", "nicht ausgeführt",
		"k-check", "--files-from", "--target-root", "context.playbook.dir", "## Schritt 4"} {
		if strings.Contains(file, stale) {
			t.Errorf("k-enforcement.md trägt noch %q", stale)
		}
	}

	// Der Skill ks-enforcement ist entfallen; den Doku-Nachzug leistet jede
	// Sitzung über die always-Anweisung.
	if _, ok := skillSources(filepath.Join("..", "..", "..", "share", "skills"))["enforcement"]; ok {
		t.Error("share/skills/enforcement ist noch da")
	}

	got := mustResolveDoc(t, layers, ref)
	if got.Content != wantText || len(got.Problems) != 0 || len(got.Includes) != 1 {
		t.Errorf("eingesetzter Text weicht ab: problems %v, includes %+v", got.Problems, got.Includes)
	}
}

// Die always-Anweisung „Doku nachziehen“ (Task 124, Titel seit Task 135) steht im zusammengebauten
// Text zwischen Workbench und Tasks und verweist auf die Doku-Aufträge und
// /k-enforcement; /k-task-run reiht im Hauptkontext ein, /k-danke zählt
// „eingereiht“ als erledigt.
func TestDefaultsRegelnAnwenden(t *testing.T) {
	layers := repoDefaultsLayers(t)
	part := readRepoDefault(t, "instructions/always/220-regeln-anwenden.md")
	if !strings.HasPrefix(part, "---\ntitle: Doku nachziehen\n---\n") {
		t.Errorf("220-regeln-anwenden.md ohne title:\n%.120s", part)
	}
	for _, needle := range []string{"im selben Arbeitsgang", "fragst im Zweifel", "sagt er kurz, warum",
		"k-playbook:/instructions/on-demand/doku-auftraege.md", "`/k-enforcement`"} {
		if !strings.Contains(squashSpace(part), needle) {
			t.Errorf("220-regeln-anwenden.md nennt %q nicht", needle)
		}
	}
	always := mustResolveDoc(t, layers, "k-playbook:/instructions/always/").Content
	workbench, rules, tasks := strings.Index(always, "## Die Workbench"), strings.Index(always, "## Doku nachziehen"),
		strings.Index(always, "## Arbeiten in Tasks")
	if workbench < 0 || rules < workbench || tasks < rules {
		t.Errorf("Reihenfolge im zusammengebauten Text: Workbench %d, Regeln %d, Tasks %d", workbench, rules, tasks)
	}

	run := taskCommandBuendel(t, "k-task-run.md", "_task-run")
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

// Frontmatter-Titel in den Vorlagen (Task 136): Jede mitgelieferte Vorlage,
// die ein Frontmatter mit title zeigt, zeigt den Wert in doppelten
// Anführungszeichen — ein Titel mit „: “ macht das Frontmatter sonst
// unlesbar, „ #“ schneidet ihn ab —, und die Anweisung dazu nennt die Regel
// einmal. Die Needles halten die Vorlagen davon ab, still auf `title: <…>`
// zurückzufallen.
func TestDefaultsFrontmatterTitelQuotiert(t *testing.T) {
	templates := map[string][]string{
		"instructions/on-demand/tasks.md":                               {`title: "<Titel>"`},
		"instructions/on-demand/befunde.md":                             {`title: "<der Befund in einem Satz>"`, `thema: "<`},
		"instructions/on-demand/doku-auftraege.md":                      {`title: "<was nachzuziehen ist, in einem Satz>"`, `anlass: "<`},
		"commands/_shared/docs.md":                                      {`title: "<Titel>"`, `description: "<Ein Satz: was dieses Dokument erklärt.>"`},
		"commands/_docs-code/schreiben.md":                              {`title: "<Titel>"`, `description: "<Ein Satz: was dieses Dokument erklärt.>"`},
		"commands/_docs-tools/schreiben.md":                             {`title: "<Name, wie das Projekt ihn nennt>"`, `description: "Projektrelevante Pitfalls und Idiome für <Name>."`},
		"commands/_skills/overlay-repo-analyse/overlay-doc.md.template": {`title: "<Overlay-Repo-Name>"`, `description: "<`},
	}
	for rel, needles := range templates {
		file := readRepoDefault(t, rel)
		for _, needle := range needles {
			if !strings.Contains(file, needle) {
				t.Errorf("%s zeigt %q nicht", rel, needle)
			}
		}
		for _, stale := range []string{"title: <", "description: <"} {
			if strings.Contains(file, stale) {
				t.Errorf("%s zeigt noch einen unquotierten Platzhalter %q", rel, stale)
			}
		}
	}
	// Die beiden Vorlagen in tasks.md — „Kopf“ und „Aufbau“ — sind beide quotiert.
	if tasks := readRepoDefault(t, "instructions/on-demand/tasks.md"); strings.Count(tasks, "---\ntitle: \"<Titel>\"\n---\n") != 2 {
		t.Errorf("tasks.md zeigt die quotierte Vorlage nicht zweimal")
	}
	// Der Satz zur Regel steht einmal je Anweisung, die den Kopf beschreibt.
	for rel, needle := range map[string]string{
		"instructions/on-demand/tasks.md":          "stehen in doppelten Anführungszeichen, ein `\"` im Wert wird `\\\"`",
		"instructions/on-demand/befunde.md":        "in doppelten Anführungszeichen, ein `\"` im Wert wird `\\\"`",
		"instructions/on-demand/doku-auftraege.md": "in doppelten Anführungszeichen, ein `\"` im Wert wird `\\\"`",
		"instructions/always/210-workbench.md":     "`title` ist Pflicht, in doppelten Anführungszeichen",
		"commands/k-task-create.md":                "Frontmatter mit `title` in doppelten\nAnführungszeichen",
	} {
		if !strings.Contains(readRepoDefault(t, rel), needle) {
			t.Errorf("%s nennt die Quotierregel nicht: %q", rel, needle)
		}
	}
}

// /k-befunde-einordnen (Task 166) ist Hülle plus Text: Die Hülle nennt nur den
// Verweis, erlaubt die Werkzeuge der Vorprüfung, der Ziele und des Löschens
// und nimmt die Obergrenze als Argument. Der Text beginnt mit dem ersten
// Schritt, liest die Projekterweiterung befunde-einordnen.md (not_found ist
// dort kein Fehler), hält die Reihenfolge schreiben → gegenlesen → löschen →
// protokollieren und bindet _shared/docs.md nicht ein, sondern liest nur dessen
// Abschnitt „Der Kopf“. k-playbook liefert unter dem Namen der Erweiterung
// selbst nichts.
func TestDefaultsKBefundeEinordnenIstHuelleUndText(t *testing.T) {
	layers := repoDefaultsLayers(t)
	const ref = "k-playbook:/commands/k-befunde-einordnen.md"

	hullData, err := os.ReadFile(filepath.Join("..", "..", "..", "share", "commands", "k-befunde-einordnen.md"))
	if err != nil {
		t.Fatal(err)
	}
	hull := string(hullData)
	if !strings.Contains(hull, "`"+ref+"`") || !strings.Contains(hull, "argument-hint: \"[anzahl]\"") {
		t.Errorf("Hülle ohne Verweis auf den Text oder ohne Argument:\n%s", hull)
	}
	for _, tool := range []string{"mcp__k-playbook__k_playbook_findings_check", "mcp__k-playbook__k_playbook_doc_create",
		"mcp__k-playbook__k_playbook_doc_append", "mcp__k-playbook__k_playbook_todo_add", "mcp__kephalaion__delete",
		"mcp__kephalaion__rename", "mcp__kephalaion__write", "mcp__kephalaion__create", "mcp__kephalaion__search"} {
		if !strings.Contains(hull, tool) {
			t.Errorf("Hülle erlaubt %s nicht", tool)
		}
	}

	file := readRepoDefault(t, "commands/k-befunde-einordnen.md")
	if !strings.Contains(file, "\ninclude: _shared/erster-schritt.md\n") ||
		strings.Index(file, "include: _shared/erster-schritt.md") > strings.Index(file, "## Schritt 2") {
		t.Error("k-befunde-einordnen.md beginnt nicht mit dem ersten Schritt")
	}
	if strings.Contains(file, "include: _shared/docs.md") {
		t.Error("k-befunde-einordnen.md bindet die Docs-Ablage ein; gelesen wird nur „Der Kopf“")
	}
	for _, needle := range []string{
		"`k-playbook:/instructions/on-demand/befunde-einordnen.md`", "**Das ist kein Fehler.**",
		"`k-playbook:/instructions/always/010-verweise.md`", "`k_playbook_findings_check`",
		"`k-playbook findings check --json`", "`check.byUser`", "Standard 25", "`kind: einordnung`",
		"`- <voller Name> | übersprungen (<reason>)`", "`base_revision`", "`request_id`", "`replayed`",
		"**(a) Ziel schreiben.**", "**(b) Gegenlesen:**", "**(c) Löschen bzw. verschieben,**",
		"**(d) Erst dann die Protokollzeile**", "`stale_revision`", "`übersprungen (anderer Lauf)`",
		"`übersprungen (Ausgang unklar)`", "`übersprungen (Rückfrage)`", "`queue/unclear/<dateiname>`", "`frage`",
		"nie ein Muster, nie ein Verzeichnis", "`section: \"### Der Kopf\"`", "**Ziele nennen den Befund nicht.**",
		"`git symbolic-ref --short refs/remotes/origin/HEAD`", "git fetch origin <hauptbranch>", "`FETCH_HEAD`",
		"wie viele Befunde noch in\n  `keph:/queue/findings/` offen sind",
	} {
		if !strings.Contains(file, needle) {
			t.Errorf("k-befunde-einordnen.md nennt %q nicht", needle)
		}
	}
	// Nichts Projektspezifisches aus dem Entwurf.
	for _, stale := range []string{"origin/development", "stage", "master", "prod", "readgate", "k-playbook context"} {
		if strings.Contains(file, stale) {
			t.Errorf("k-befunde-einordnen.md trägt %q", stale)
		}
	}
	got := mustResolveDoc(t, layers, ref)
	if len(got.Problems) != 0 || len(got.Includes) != 1 {
		t.Errorf("Text: problems %v, includes %+v", got.Problems, got.Includes)
	}
	// Die Erweiterung liefert k-playbook selbst nicht.
	if _, err := os.Stat(filepath.Join("..", "..", "..", "defaults", "instructions", "on-demand", "befunde-einordnen.md")); err == nil {
		t.Error("defaults/instructions/on-demand/befunde-einordnen.md liegt im Lieferumfang")
	}
}

// Task 169: /k-task-refine nimmt ohne Argument nur Ungeprüfte, prüft sie in
// Gruppen je in einem Sub-Agenten und fragt bei schon Geprüftem nach, außer
// nochmal steht hinter dem Ziel. Die Hülle sagt das in description und
// argument-hint; der Text trägt die Verträge mit dem Sub-Agenten einer Gruppe
// und mit dem Critic der Überschneidungen.
func TestShareKTaskRefineHuelleNurUngepruefte(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "share", "commands", "k-task-refine.md"))
	if err != nil {
		t.Fatal(err)
	}
	hull := string(data)
	if !strings.Contains(hull, "\nargument-hint: \"[ref-or-path] [nochmal]\"\n") {
		t.Errorf("argument-hint nennt nochmal nicht:\n%s", hull)
	}
	description := ""
	for _, line := range strings.Split(hull, "\n") {
		if value, ok := strings.CutPrefix(line, "description: "); ok {
			description = value
		}
	}
	for _, want := range []string{"nur die eigenen offenen Tasks ohne Review-Log", "in Gruppen zusammenhängender Tasks",
		"Sub-Agenten parallel", "Überschneidungen", "erst nach Rückfrage, außer nochmal", "k-playbook:/commands/k-task-refine.md"} {
		if !strings.Contains(description, want) {
			t.Errorf("description ohne %q: %q", want, description)
		}
	}
	// Ein „: “ in der unquotierten description machte das Frontmatter unlesbar.
	if strings.Contains(description, ": ") || strings.Contains(description, " #") {
		t.Errorf("description mit „: “ oder „ #“: %q", description)
	}

	// Seit Task 176 liegen die Textstellen im Kern oder in einem Modul.
	text := taskCommandBuendel(t, "k-task-refine.md", "_task", "_task-refine")
	for _, want := range []string{
		"Muster `^## Review-Log`",
		"`nochmal`, `erneut` oder\n`again`",
		"Alle <n> offenen Tasks sind schon geprüft.",
		"`Rückfrage: schon geprüft`",
		"Ergebnis: geprüft | rückfrage | abgebrochen | keine-subagenten",
		"`Foreign-Change: <revision> <updatedBy> <updatedAt>`",
		"`Zwischenstand:`",
		"Werkzeug `subagent`",
		"Überschneidung: <nr> + <nr> — <was> — Empfehlung:",
		"Überschneidungen: keine",
		"Folge-Command: **`/k-task-run`**",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("k-task-refine.md ohne %q", want)
		}
	}
}

// Task 170: Die Rückgabe des Review-Moduls nennt die Befunde „wichtig“ in einem
// eigenen Block, die Hülle von /k-task-run darf daraus Todos anlegen, und 5a,
// Option (a), holt die Befunde eines schon ausgeführten Tasks über den
// Befund-Leser, dessen Rückgabe ebenfalls `Wichtig:` trägt.
func TestKTaskRunReviewBefundeWichtigAlsTodo(t *testing.T) {
	module := readRepoDefault(t, "commands/_task-run/diff-review.md")
	at := strings.Index(module, "### Rückgabe")
	if at < 0 {
		t.Fatal("diff-review.md ohne Abschnitt „Rückgabe“")
	}
	rueckgabe, _, ok := strings.Cut(module[at:], "## Abbruch")
	if !ok {
		t.Fatal("diff-review.md ohne Abschnitte „Rückgabe“ und „Abbruch“")
	}
	for _, want := range []string{"\nKritisch:\n  - <datei:zeile oder Prüfbefehl> — <ein Satz>\nWichtig:\n  - <datei:zeile oder Prüfbefehl> — <ein Satz>\nHinweise:\n  - <datei:zeile oder Prüfbefehl> — <ein Satz>\nRevision: <n>\n",
		"`Wichtig: keine`", "`Hinweise: keine`"} {
		if !strings.Contains(rueckgabe, want) {
			t.Errorf("Rückgabe in diff-review.md ohne %q", want)
		}
	}

	data, err := os.ReadFile(filepath.Join("..", "..", "..", "share", "commands", "k-task-run.md"))
	if err != nil {
		t.Fatal(err)
	}
	allowed := ""
	for _, line := range strings.Split(string(data), "\n") {
		if value, ok := strings.CutPrefix(line, "allowed-tools: "); ok {
			allowed = value
		}
	}
	for _, want := range []string{"mcp__k-playbook__k_playbook_todo_add", "mcp__k-playbook__k_playbook_todo_list"} {
		if !strings.Contains(allowed, want) {
			t.Errorf("Hülle k-task-run.md erlaubt %s nicht: %q", want, allowed)
		}
	}

	// Seit Task 176 liegen die Stellen aus 5a, 5f und Schritt 7 in Modulen.
	nichtAbgeschlossen := readRepoDefault(t, "commands/_task-run/nicht-abgeschlossen.md")
	if !strings.Contains(nichtAbgeschlossen, "- (a): Zuerst die Befunde des letzten Laufs holen") {
		t.Errorf("nicht-abgeschlossen.md ohne Option (a) mit dem Befund-Leser")
	}
	leser := readRepoDefault(t, "commands/_task-run/befund-leser.md")
	for _, want := range []string{"# Befund-Leser",
		"Kritisch:\n  - <Ort aus der Tabelle> — <Befund in einem Satz>\nWichtig:\n  - <Ort aus der Tabelle> — <Befund in einem Satz>\nHinweise:\n  - <Ort aus der Tabelle> — <Befund in einem Satz>",
		"`Hinweise: keine`", "`Review: kein Abschnitt`"} {
		if !strings.Contains(leser, want) {
			t.Errorf("befund-leser.md ohne %q", want)
		}
	}
	if !strings.Contains(readRepoDefault(t, "commands/_task-run/abschluss.md"), "## Befund-Leser") {
		t.Errorf("abschluss.md beschreibt den Start des Befund-Lesers nicht")
	}

	if todo := readRepoDefault(t, "commands/_task-run/befunde-als-todo.md"); !strings.Contains(todo, "Hinweise (`Hinweise:`) gehören nicht dazu") {
		t.Errorf("befunde-als-todo.md bietet Hinweise nicht ausdrücklich ausgenommen an")
	}
	if strings.Contains(module, "Hinweise bleiben allein eine Zahl") || strings.Contains(leser, "Hinweise nur als Zahl") {
		t.Errorf("Rückgabe zählt Hinweise noch als bloße Zahl")
	}

	text := readRepoDefault(t, "commands/k-task-run.md")
	bundle := taskCommandBuendel(t, "k-task-run.md", "_task-run")
	for _, want := range []string{"Eine Rückgabe ohne `Wichtig:`-Block ist wie jede\n  formwidrige kein gültiges Review.",
		"Befunde sammeln", "`Rückfrage: Befunde als Todo`",
		"Als Todo übernehmen? (alle / keine / alle außer <nummern> / nur <nummern>)",
		"`origin` ist `Task <NNN> — <TASK_REF_DONE>`", "Todos:         angelegt <kennung>",
		"`Review: kein Abschnitt im letzten Lauf`", "„— Hinweise nicht übermittelt“", "Vorsatz `Hinweis:`"} {
		if !strings.Contains(bundle, want) {
			t.Errorf("k-task-run.md samt Modulen ohne %q", want)
		}
	}
	// Die Abfrage steht in Schritt 7 nach dem Doku-Nachzug und vor der Fertig-Meldung.
	nachzug := strings.Index(text, "**Doku-Nachzug.**")
	abfrage := strings.Index(text, "**Befunde als Todo.**")
	fertig := strings.Index(text, "Nach allen Tasks zuerst die Fertig-Meldung")
	if nachzug < 0 || abfrage < nachzug || fertig < abfrage {
		t.Errorf("Abfrage nicht zwischen Doku-Nachzug (%d) und Fertig-Meldung (%d): %d", nachzug, fertig, abfrage)
	}
}

// Task 171: Die Hülle von /k-task-run nennt das Wort refine im argument-hint und
// erlaubt das Sperr-Werkzeug; der Text nimmt die Sperre nach Schritt 3, prüft sie
// an den festen Stellen und gibt sie beim Verlassen von Schritt 5 frei.
func TestKTaskRunSperreUndRefine(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "share", "commands", "k-task-run.md"))
	if err != nil {
		t.Fatal(err)
	}
	hint, allowed := "", ""
	for _, line := range strings.Split(string(data), "\n") {
		if value, ok := strings.CutPrefix(line, "argument-hint: "); ok {
			hint = value
		}
		if value, ok := strings.CutPrefix(line, "allowed-tools: "); ok {
			allowed = value
		}
	}
	if hint != `"[ref-or-number] [refine]"` {
		t.Errorf("argument-hint der Hülle: %q", hint)
	}
	for _, want := range []string{"mcp__k-playbook__k_playbook_lock", "mcp__k-playbook__k_playbook_notify",
		"mcp__k-playbook__k_playbook_doc_append", "Agent"} {
		if !strings.Contains(allowed, want) {
			t.Errorf("Hülle k-task-run.md erlaubt %s nicht: %q", want, allowed)
		}
	}

	text := readRepoDefault(t, "commands/k-task-run.md")
	bundle := taskCommandBuendel(t, "k-task-run.md", "_task-run")
	for _, want := range []string{"## Sperre des Arbeitsbaums", "`LOCK_TOKEN`", "**Wort `refine`.**",
		"`SCHON_AUSGEFUEHRT`", "**Modus `refine`.**", "### Sperre nehmen", "**Liste neu prüfen.**",
		"`Rückfrage: Sperre unsicher`", "`Rückfrage: Sperre verloren`", "`Rückfrage: Marke unbekannt`",
		"k-playbook lock release --force", "Ohne `refine` bleibt Schritt 3", "Sperre:        sofort genommen",
		"Refine:        <je Ziel"} {
		if !strings.Contains(bundle, want) {
			t.Errorf("k-task-run.md samt Modulen ohne %q", want)
		}
	}
	// Die Sperre wird nach Schritt 3 genommen und vor Schritt 6 freigegeben.
	schritt3 := strings.Index(text, "## Schritt 3 — ")
	nehmen := strings.Index(text, "### Sperre nehmen")
	fuenfA := strings.Index(text, "### 5a — Lesen und verstehen")
	freigeben := strings.Index(text, "Kein weiterer Task — nach dem letzten oder nach einem Anhalten —:\nSperre freigeben")
	schritt6 := strings.Index(text, "## Schritt 6 — ")
	if schritt3 < 0 || nehmen < schritt3 || fuenfA < nehmen || freigeben < fuenfA || schritt6 < freigeben {
		t.Errorf("Reihenfolge Schritt 3 (%d) < Sperre nehmen (%d) < 5a (%d) < Freigabe (%d) < Schritt 6 (%d) verletzt",
			schritt3, nehmen, fuenfA, freigeben, schritt6)
	}
	// check vor jeder Stelle, die den Arbeitsbaum ändert.
	for _, want := range []string{"(vor einem Wechsel `check` der Sperre)", "Vor dem Anlegen `check` der Sperre.",
		"Vorher `check` der Sperre, dann die Fortschrittszeilen zählen", "Nach `check` der Sperre startet",
		"`check` der Sperre, dann pushen."} {
		if !strings.Contains(bundle, want) {
			t.Errorf("k-task-run.md ohne Prüfstelle %q", want)
		}
	}
}
