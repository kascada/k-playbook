package project

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/kascada/k-playbook/installer/internal/githubaccess/accesstest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/kascada/k-playbook/installer/internal/health"
	"github.com/kascada/k-playbook/installer/internal/kephalaion"
)

// newBriefingProject baut ein Projekt mit beiden Ebenen: ersetzt,
// abgeschaltet, nur lokal und nur mitgeliefert kommen in jeder Sorte vor. Die
// projekteigene Ebene von instructions/ und reviews/ liegt in der Collection
// des Projekts (test-collection/ baut sie, isolateDefaults).
func newBriefingProject(t *testing.T) string {
	t.Helper()

	root := newContextProject(t)
	files := map[string]string{
		"test-defaults/instructions/always/010-verweise.md":             "## Verweise\n\nText.\n",
		"test-defaults/instructions/always/100-wissen.md":               "---\ntitle: Projektwissen zuerst\ndescription: Doku vor Code.\n---\n\n## Projektwissen\n",
		"test-defaults/instructions/always/200-befunde.md":              "## Befunde festhalten\n",
		"test-defaults/instructions/always/README.md":                   "# Liesmich\n",
		"test-defaults/instructions/on-demand/muster.md":                "---\ntitle: Muster\ndescription: Zeigt den Aufbau.\n---\n\n## Muster\n",
		"test-defaults/instructions/on-demand/release.md":               "## Release fahren\n",
		"test-defaults/reviews/review-code.md":                          "---\nname: review-code\ntitle: Code-Review\n---\n# Code\n",
		"test-defaults/reviews/review-tech.md":                          "# Tech-Review\n",
		"test-defaults/reviews/notiz.md":                                "# kein Rezept\n",
		"test-defaults/guidelines/betrieb.md":                           "# Betrieb: Ablageorte\n\nText.\n",
		"test-defaults/guidelines/README.md":                            "# Guidelines\n",
		"test-collection/k-playbook/instructions/always/100-wissen.md":  "## Projektwissen lokal\n\nText.\n",
		"test-collection/k-playbook/instructions/always/200-befunde.md": "# Abgeschaltet: keine Befunde.\n",
		"test-collection/k-playbook/instructions/always/1000-eigen.md":  "## Eigene Anweisung\n\nText.\n",
		"test-collection/k-playbook/instructions/always/notiz.md":       "## Ohne Ziffer\n",
		"test-collection/k-playbook/instructions/on-demand/release.md":  "",
		"test-collection/k-playbook/instructions/on-demand/eigen.md":    "## Eigen auf Abruf\n\nText.\n",
		"test-collection/k-playbook/reviews/review-tech.md":             "# Abgeschaltet.\n",
	}
	for rel, content := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		mkdirAll(t, filepath.Dir(path))
		write(t, path, content)
	}
	return root
}

func mkdirAll(t *testing.T, dir string) {
	t.Helper()
	writeDocTree(t, dir, nil)
}

// stubBriefingDocs ersetzt den Zugang zur Collection für die Dauer des Tests.
func stubBriefingDocs(t *testing.T, open func(string) (*UserDocs, error)) {
	t.Helper()
	original := openBriefingDocs
	openBriefingDocs = open
	t.Cleanup(func() { openBriefingDocs = original })
}

func unavailableDocs(string) (*UserDocs, error) {
	return nil, docRefErrorf(DocCodeKephalaionUnavailable, "Kephalaion ist für dieses Projekt nicht bereit (not-configured): kein Block")
}

func TestBriefingListetDokumenteMitHerkunft(t *testing.T) {
	root := newBriefingProject(t)
	stubBriefingDocs(t, unavailableDocs)

	briefing, err := BuildBriefing(context.Background(), root)
	if err != nil {
		t.Fatalf("BuildBriefing: %v", err)
	}
	if briefing.SchemaVersion != BriefingSchemaVersion {
		t.Errorf("schemaVersion %q", briefing.SchemaVersion)
	}
	if briefing.ProjectDocs != "keph:/docs/" {
		t.Errorf("projectDocs %q", briefing.ProjectDocs)
	}

	always := briefing.Documents.Always
	if always.Ref != "k-playbook:/instructions/always/" {
		t.Errorf("always.ref %q", always.Ref)
	}
	wantAlways := []BriefingEntry{
		{Ref: "k-playbook:/instructions/always/010-verweise.md", Title: "Verweise", Origin: DocOriginDist},
		{Ref: "k-playbook:/instructions/always/100-wissen.md", Title: "Projektwissen lokal", Origin: DocOriginOverride,
			Location: "keph:/k-playbook/instructions/always/100-wissen.md"},
		{Ref: "k-playbook:/instructions/always/200-befunde.md", Title: "200-befunde.md", Origin: BriefingOriginDisabled,
			Location: "keph:/k-playbook/instructions/always/200-befunde.md"},
		{Ref: "k-playbook:/instructions/always/1000-eigen.md", Title: "Eigene Anweisung", Origin: DocOriginLocal,
			Location: "keph:/k-playbook/instructions/always/1000-eigen.md"},
	}
	if !reflect.DeepEqual(always.Parts, wantAlways) {
		t.Errorf("always.parts:\n%+v\nerwartet\n%+v", always.Parts, wantAlways)
	}
	if len(always.Problems) != 1 || !strings.Contains(always.Problems[0], "notiz.md") {
		t.Errorf("always.problems %v", always.Problems)
	}
	// Der Text der immer geltenden Anweisungen steht im Briefing selbst: ohne
	// Frontmatter, der abgeschaltete Teil übergangen, derselbe Text wie bei
	// `k-playbook read` für den Verweis.
	wantContent := "## Verweise\n\nText.\n\n## Projektwissen lokal\n\nText.\n\n## Eigene Anweisung\n\nText.\n"
	if always.Content != wantContent {
		t.Errorf("always.content:\n%q\nerwartet\n%q", always.Content, wantContent)
	}
	read, err := ResolveDocRef(ProjectDocLayers(root), always.Ref)
	if err != nil {
		t.Fatalf("ResolveDocRef: %v", err)
	}
	if read.Content != always.Content {
		t.Errorf("Briefing und read liefern verschiedene Texte:\n%q\n%q", always.Content, read.Content)
	}

	wantOnDemand := []BriefingEntry{
		{Ref: "k-playbook:/instructions/on-demand/eigen.md", Title: "Eigen auf Abruf", Origin: DocOriginLocal,
			Location: "keph:/k-playbook/instructions/on-demand/eigen.md"},
		{Ref: "k-playbook:/instructions/on-demand/muster.md", Title: "Muster", Description: "Zeigt den Aufbau.", Origin: DocOriginDist},
		{Ref: "k-playbook:/instructions/on-demand/release.md", Title: "release.md", Origin: BriefingOriginDisabled,
			Location: "keph:/k-playbook/instructions/on-demand/release.md"},
	}
	if !reflect.DeepEqual(briefing.Documents.OnDemand, wantOnDemand) {
		t.Errorf("onDemand:\n%+v\nerwartet\n%+v", briefing.Documents.OnDemand, wantOnDemand)
	}

	// Die Sorte rules gibt es nicht mehr: Sie erscheint nirgends im Briefing.
	if data, err := json.Marshal(briefing.Documents); err != nil || strings.Contains(string(data), "rules") {
		t.Errorf("documents nennt die entfallene Sorte: %s %v", data, err)
	}

	// Reviews folgen dem Muster des Katalogs: notiz.md ist kein Rezept.
	wantReviews := []BriefingEntry{
		{Ref: "k-playbook:/reviews/review-code.md", Title: "Code-Review", Origin: DocOriginDist},
		{Ref: "k-playbook:/reviews/review-tech.md", Title: "review-tech.md", Origin: BriefingOriginDisabled,
			Location: "keph:/k-playbook/reviews/review-tech.md"},
	}
	if !reflect.DeepEqual(briefing.Documents.Reviews, wantReviews) {
		t.Errorf("reviews:\n%+v\nerwartet\n%+v", briefing.Documents.Reviews, wantReviews)
	}

	// Guidelines kommen nur mitgeliefert (Task 148), README nie.
	wantGuidelines := []BriefingEntry{
		{Ref: "k-playbook:/guidelines/betrieb.md", Title: "Betrieb: Ablageorte", Origin: DocOriginDist},
	}
	if !reflect.DeepEqual(briefing.Documents.Guidelines, wantGuidelines) {
		t.Errorf("guidelines:\n%+v\nerwartet\n%+v", briefing.Documents.Guidelines, wantGuidelines)
	}

	// Jeder Verweis des Briefings lässt sich über ResolveDocRef lesen, mit
	// derselben Herkunft und demselben Ort.
	layers := ProjectDocLayers(root)
	for _, entries := range [][]BriefingEntry{always.Parts, briefing.Documents.OnDemand, briefing.Documents.Reviews, briefing.Documents.Guidelines} {
		for _, entry := range entries {
			result := mustResolveDoc(t, layers, entry.Ref)
			origin := result.Parts[0].Origin
			if result.Disabled {
				origin = BriefingOriginDisabled
			}
			if origin != entry.Origin {
				t.Errorf("%s: Briefing %s, Verweis %s", entry.Ref, entry.Origin, origin)
			}
			if result.Parts[0].Location != entry.Location {
				t.Errorf("%s: Ort im Briefing %q, beim Verweis %q", entry.Ref, entry.Location, result.Parts[0].Location)
			}
		}
	}
}

// Fehlt eine Ebene oder eine Sorte ganz, bleibt die Liste leer — kein Fehler.
func TestBriefingOhneVerzeichnisse(t *testing.T) {
	root := newContextProject(t)
	stubBriefingDocs(t, unavailableDocs)

	briefing, err := BuildBriefing(context.Background(), root)
	if err != nil {
		t.Fatalf("BuildBriefing: %v", err)
	}
	if briefing.Documents.Always.Ref != "k-playbook:/instructions/always/" || briefing.Documents.Always.Content != "" || len(briefing.Documents.Always.Parts) != 0 || briefing.Documents.Always.Problems == nil {
		t.Errorf("always: %+v", briefing.Documents.Always)
	}
	for name, list := range map[string][]BriefingEntry{
		"onDemand": briefing.Documents.OnDemand, "reviews": briefing.Documents.Reviews,
		"guidelines": briefing.Documents.Guidelines,
	} {
		if list == nil || len(list) != 0 {
			t.Errorf("%s: %v, erwartet leere Liste", name, list)
		}
	}
}

// Die Sitzungskennung vergibt das Briefing selbst, je Aufruf neu: Zeitpunkt
// des Briefings und sechs Zeichen Zufall. Sie hängt nicht an Kephalaion —
// auch ohne nutzbare Collection hat die Sitzung eine Kennung.
func TestBriefingVergibtSitzungskennung(t *testing.T) {
	root := newContextProject(t)
	stubBriefingDocs(t, unavailableDocs)
	fixed := time.Date(2026, 10, 24, 18, 30, 12, 0, time.Local)
	previous := now
	now = func() time.Time { return fixed }
	t.Cleanup(func() { now = previous })

	first, err := BuildBriefing(context.Background(), root)
	if err != nil {
		t.Fatalf("BuildBriefing: %v", err)
	}
	second, err := BuildBriefing(context.Background(), root)
	if err != nil {
		t.Fatalf("BuildBriefing: %v", err)
	}
	pattern := regexp.MustCompile(`^20261024-183012-[0-9a-f]{6}$`)
	for _, session := range []string{first.Session, second.Session} {
		if !pattern.MatchString(session) {
			t.Errorf("session %q passt nicht zu %s", session, pattern)
		}
	}
	// Dieselbe Sekunde, zwei Briefings: der Zufall hält sie auseinander.
	if first.Session == second.Session {
		t.Errorf("zwei Briefings mit derselben Kennung %q", first.Session)
	}
	if first.Work.Available {
		t.Errorf("Collection wider Erwarten nutzbar: %+v", first.Work)
	}
}

func TestBriefingKephalaionNichtNutzbar(t *testing.T) {
	root := newBriefingProject(t)
	stubBriefingDocs(t, unavailableDocs)

	briefing, err := BuildBriefing(context.Background(), root)
	if err != nil {
		t.Fatalf("BuildBriefing: %v", err)
	}
	work := briefing.Work
	if work.Available || !strings.Contains(work.Message, "nicht bereit") || work.User != "" {
		t.Errorf("work: %+v", work)
	}
	if work.Tasks == nil || len(work.Tasks) != 0 || work.Workbench == nil || len(work.Workbench) != 0 || work.OpenFindings != 0 {
		t.Errorf("Listen bei nicht nutzbarem Kephalaion: %+v", work)
	}

	// Ohne Anbindung in der Konfiguration meldet auch der echte Zugang das,
	// ohne den Node zu fragen.
	stubBriefingDocs(t, OpenUserDocs)
	original := kephalaionCheck
	kephalaionCheck = func(string) kephalaion.Status {
		return kephalaion.Status{State: kephalaion.StateNotConfigured, Message: "Keine Collection eingetragen."}
	}
	t.Cleanup(func() { kephalaionCheck = original })
	briefing, err = BuildBriefing(context.Background(), root)
	if err != nil {
		t.Fatalf("BuildBriefing: %v", err)
	}
	if briefing.Work.Available || !strings.Contains(briefing.Work.Message, "Keine Collection eingetragen") {
		t.Errorf("work ohne Anbindung: %+v", briefing.Work)
	}
}

// Ein Fehler beim Lesen der Collection ist ein Zustand, kein Abbruch.
func TestBriefingCollectionNichtLesbar(t *testing.T) {
	root := newBriefingProject(t)
	store := newFakeDocStore("kamran")
	store.fail = &kephalaion.ToolError{Tool: "list", Code: kephalaion.CodeNodeUnreachable, Message: "Node weg"}
	stubBriefingDocs(t, func(string) (*UserDocs, error) { return newTestUserDocs(t, store, DocKinds), nil })

	briefing, err := BuildBriefing(context.Background(), root)
	if err != nil {
		t.Fatalf("BuildBriefing: %v", err)
	}
	if briefing.Work.Available || !strings.Contains(briefing.Work.Message, "Node weg") || len(briefing.Work.Tasks) != 0 {
		t.Errorf("work: %+v", briefing.Work)
	}
}

// Die eigene Arbeit: offene Tasks und aktive Arbeitsstände sind, was
// k_playbook_doc_list ohne done liefert — eigene, nicht in done/; dazu die
// Zahl der offenen Befunde.
func TestBriefingEigeneArbeit(t *testing.T) {
	root := newBriefingProject(t)
	store := newFakeDocStore("kamran",
		"user/kamran/tasks/100-briefing.md",
		"user/kamran/tasks/101-ohne-titel.md",
		"user/kamran/tasks/102-kaputt.md",
		"user/kamran/tasks/done/099-alt.md",
		"user/hp/tasks/102-fremd.md",
		"user/kamran/workbench/001-umbau.md",
		"user/kamran/workbench/003-todo.md",
		"user/kamran/workbench/done/002-fertig.md",
		"queue/findings/001-a.md",
		"queue/findings/002-b.md",
		"queue/findings/003-c.md",
	)
	store.docs["user/kamran/tasks/100-briefing.md"].title = "Briefing bauen"
	// Ein Frontmatter, das der Node nicht lesen konnte (Task 136), kommt mit
	// seinem Problem ins Briefing, nicht still mit dem Dateinamen als Titel.
	store.docs["user/kamran/tasks/102-kaputt.md"].frontmatterError = "YAML ungültig: mapping values are not allowed in this context"
	store.docs["user/kamran/workbench/001-umbau.md"].title = "Umbau auf Kephalaion"
	store.docs["user/kamran/workbench/003-todo.md"].title = "Offene Punkte (Todo"
	stubBriefingDocs(t, func(string) (*UserDocs, error) { return newTestUserDocs(t, store, DocKinds), nil })

	briefing, err := BuildBriefing(context.Background(), root)
	if err != nil {
		t.Fatalf("BuildBriefing: %v", err)
	}
	work := briefing.Work
	if !work.Available || work.Message != "" || work.User != "kamran" {
		t.Fatalf("work: %+v", work)
	}
	wantTasks := []BriefingDoc{
		{Ref: "keph:/user/kamran/tasks/100-briefing.md", Title: "Briefing bauen", Updated: "2026-10-14T11:00:00.000Z"},
		{Ref: "keph:/user/kamran/tasks/101-ohne-titel.md", Title: "101-ohne-titel.md", Updated: "2026-10-14T11:00:00.000Z"},
		{Ref: "keph:/user/kamran/tasks/102-kaputt.md", Title: "102-kaputt.md", Updated: "2026-10-14T11:00:00.000Z",
			Problem: "Frontmatter nicht lesbar: YAML ungültig: mapping values are not allowed in this context"},
	}
	if !reflect.DeepEqual(work.Tasks, wantTasks) {
		t.Errorf("tasks: %+v", work.Tasks)
	}
	wantWorkbench := []BriefingDoc{
		{Ref: "keph:/user/kamran/workbench/001-umbau.md", Title: "Umbau auf Kephalaion", Updated: "2026-10-14T11:00:00.000Z"},
		{Ref: "keph:/user/kamran/workbench/003-todo.md", Title: "Offene Punkte (Todo", Updated: "2026-10-14T11:00:00.000Z",
			Problem: "Titel vermutlich an ‚ #‘ abgeschnitten (YAML-Kommentar); Titel in doppelte Anführungszeichen setzen"},
	}
	if !reflect.DeepEqual(work.Workbench, wantWorkbench) {
		t.Errorf("workbench: %+v", work.Workbench)
	}
	if work.OpenFindings != 3 {
		t.Errorf("openFindings %d, erwartet 3", work.OpenFindings)
	}
}

// Nichts doppelt, nichts vergessen (Task 086, umgewidmet in Task 091): Die
// obersten Felder von Briefing und Kontextausgabe bleiben disjunkt —
// Umgebungsangaben stehen im Briefing nur im Übergangsabschnitt context, nicht
// zusätzlich in den eigenen Abschnitten. schemaVersion ist oben die einzige
// erlaubte Überschneidung, denn sie beschreibt die jeweilige Ausgabe selbst.
// Im Abschnitt context fehlen genau die Felder, die das Briefing selbst
// liefert; jedes übrige Feld der Kontextausgabe steht da — verglichen gegen
// BuildContext, nicht gegen eine von Hand gepflegte Liste. `k-playbook
// context` selbst bleibt unverändert; das sichern die Tests der Kontextausgabe.
func TestBriefingTraegtKeinFeldDerKontextausgabe(t *testing.T) {
	root := newBriefingProject(t)
	stubBriefingDocs(t, unavailableDocs)
	stubKephalaionCheck(t, kephalaion.Status{State: kephalaion.StateNotConfigured, Message: "Keine Collection eingetragen."})

	context, err := BuildContext(root)
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}
	briefing, err := BuildBriefing(t.Context(), root)
	if err != nil {
		t.Fatalf("BuildBriefing: %v", err)
	}

	// Oben disjunkt — auch gegen die Felder, die erst ContextForDir füllt.
	full := context
	full.Kephalaion = &kephalaion.Status{}
	full.Links = &ContextLinks{}
	full.GitHub = &ContextGitHub{}
	contextKeys := topLevelKeys(t, full)
	for key := range topLevelKeys(t, briefing) {
		if key == "schemaVersion" {
			continue
		}
		if contextKeys[key] {
			t.Errorf("Feld %q steht im Briefing und in der Kontextausgabe", key)
		}
	}

	// Der Rest: vollständig gegen BuildContext, ohne die abgedeckten Felder.
	rest := briefing.Context
	if !rest.Available || rest.Message != "" {
		t.Fatalf("context: available %v, message %q", rest.Available, rest.Message)
	}
	covered := map[string]bool{}
	for _, path := range briefingCoveredContextFields {
		covered[path] = true
	}
	// catalogs fällt ganz weg, wenn jeder Katalog darin abgedeckt ist: Seit
	// Task 155 gibt es nur noch reviews, und das steht unter documents.
	catalogsLeft := false
	for name := range context.Catalogs {
		if !covered["catalogs."+name] {
			catalogsLeft = true
		}
	}
	for key := range topLevelKeys(t, context) {
		_, present := rest.Field(key)
		expected := !covered[key]
		if key == "catalogs" {
			expected = catalogsLeft
		}
		if expected != present {
			t.Errorf("Kontextfeld %q: im Rest %v, erwartet %v", key, present, expected)
		}
	}
	for name, entries := range context.Catalogs {
		var catalogs map[string]json.RawMessage
		if raw, ok := rest.Field("catalogs"); ok {
			if err := json.Unmarshal(raw, &catalogs); err != nil {
				t.Fatalf("context.catalogs: %v", err)
			}
		}
		_, present := catalogs[name]
		if covered["catalogs."+name] == present {
			t.Errorf("catalogs.%s: im Rest %v, abgedeckt %v", name, present, covered["catalogs."+name])
		}
		if present {
			want, _ := json.Marshal(entries)
			if string(catalogs[name]) != string(want) {
				t.Errorf("catalogs.%s abweichend:\n%s\n%s", name, catalogs[name], want)
			}
		}
	}
	// Was ContextForDir über BuildContext hinaus füllt, ist auch da.
	if raw, ok := rest.Field("kephalaion"); !ok || !strings.Contains(string(raw), "Keine Collection eingetragen") {
		t.Errorf("context.kephalaion: %s", raw)
	}
	if raw, ok := rest.Field("github"); !ok || !strings.Contains(string(raw), `"projectServices":[]`) {
		t.Errorf("context.github: %s", raw)
	}
	// Ein übriges Feld steht unverändert da, wie `k-playbook context` es ausgibt.
	want, _ := json.Marshal(context.Project)
	if got, _ := rest.Field("project"); string(got) != string(want) {
		t.Errorf("context.project:\n%s\nerwartet\n%s", got, want)
	}

	// Das abgedeckte Feld catalogs.reviews: ausgelassen hier, geliefert dort.
	// guidelines gibt es in der Kontextausgabe nicht mehr (Task 148), die
	// Leitlinien stehen nur unter documents.guidelines.
	if _, ok := rest.Field("guidelines"); ok || len(briefing.Documents.Guidelines) == 0 {
		t.Errorf("guidelines: im Rest %v, documents.guidelines %d", ok, len(briefing.Documents.Guidelines))
	}
	raw, _ := rest.Field("catalogs")
	if strings.Contains(string(raw), "review-code.md") || len(briefing.Documents.Reviews) == 0 {
		t.Errorf("catalogs.reviews im Rest: %s", raw)
	}

	// Die Dokumentlisten nennen Verweise, keine Pfade der Platte.
	data, err := json.Marshal(briefing.Documents)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), root) {
		t.Errorf("Briefing nennt den Projektpfad: %s", data)
	}

	// JSON: available und die Felder stehen unmittelbar unter context, in der
	// Reihenfolge der Kontextausgabe, und lesen sich zurück.
	encoded, err := json.Marshal(briefing)
	if err != nil {
		t.Fatal(err)
	}
	var raws map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &raws); err != nil {
		t.Fatal(err)
	}
	section := string(raws["context"])
	if !strings.HasPrefix(section, `{"available":true,"schemaVersion":`) || !strings.Contains(section, `"project":{"dir":`) {
		t.Errorf("context im JSON: %s", section)
	}
	var decoded Briefing
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded.Context, briefing.Context) {
		t.Errorf("Rückweg:\n%+v\nerwartet\n%+v", decoded.Context, briefing.Context)
	}
}

// Lässt sich die Kontextausgabe nicht bauen, ist das ein Zustand im Abschnitt
// context — wie bei work —, kein Fehler: das übrige Briefing bleibt.
// Der Abschnitt context trägt kein Feld playbook mehr (Task 157, Fassung 7),
// auch wenn ein liegengebliebener Clone k-playbook/ im Projekt liegt — weder
// als Feld noch im JSON des Briefings. Seit Fassung 8 fehlt auch das Feld
// local (Task 159): die einzige Wurzel ist project.
func TestBriefingKontextNenntKeinPlaybook(t *testing.T) {
	root := newBriefingProject(t)
	stubBriefingDocs(t, unavailableDocs)
	stubKephalaionCheck(t, kephalaion.Status{State: kephalaion.StateNotConfigured, Message: "Keine Collection eingetragen."})
	if err := os.MkdirAll(filepath.Join(PlaybookDir(root), ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	briefing, err := BuildBriefing(t.Context(), root)
	if err != nil {
		t.Fatalf("BuildBriefing: %v", err)
	}
	if briefing.SchemaVersion != "8" {
		t.Errorf("schemaVersion %q, erwartet 8", briefing.SchemaVersion)
	}
	if !briefing.Context.Available {
		t.Fatalf("context: %+v", briefing.Context)
	}
	if value, ok := briefing.Context.Field("playbook"); ok {
		t.Errorf("context trägt playbook: %s", value)
	}
	if _, ok := briefing.Context.Field("project"); !ok {
		t.Errorf("context trägt project nicht")
	}
	if value, ok := briefing.Context.Field("local"); ok {
		t.Errorf("context trägt local: %s", value)
	}

	encoded, err := json.Marshal(briefing)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &raw); err != nil {
		t.Fatal(err)
	}
	var section map[string]json.RawMessage
	if err := json.Unmarshal(raw["context"], &section); err != nil {
		t.Fatalf("context im JSON: %v\n%s", err, raw["context"])
	}
	for _, absent := range []string{"playbook", "local"} {
		if _, ok := section[absent]; ok {
			t.Errorf("context im JSON trägt %s: %s", absent, raw["context"])
		}
	}
	if quoted, _ := json.Marshal(PlaybookDir(root)); strings.Contains(string(encoded), string(quoted)) {
		t.Errorf("das Briefing nennt %s:\n%s", PlaybookDir(root), encoded)
	}
}

func TestBriefingKontextNichtBaubar(t *testing.T) {
	root := newBriefingProject(t)
	stubBriefingDocs(t, unavailableDocs)
	write(t, ConfigPath(root), "schema_version: 99\n")

	briefing, err := BuildBriefing(t.Context(), root)
	if err != nil {
		t.Fatalf("BuildBriefing: %v", err)
	}
	section := briefing.Context
	if section.Available || !strings.Contains(section.Message, "schema_version 99") || len(section.Fields) != 0 {
		t.Errorf("context: %+v", section)
	}
	if len(briefing.Documents.Reviews) == 0 || len(briefing.Documents.Always.Parts) == 0 {
		t.Errorf("Dokumente fehlen: %+v", briefing.Documents)
	}
	encoded, err := json.Marshal(section)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(encoded), `{"available":false,"message":"`) || strings.Count(string(encoded), ":") != 2 {
		t.Errorf("context im JSON: %s", encoded)
	}
}

// stubKephalaionCheck ersetzt die Prüfung der Anbindung, damit der Test den
// Node nicht fragt.
func stubKephalaionCheck(t *testing.T, status kephalaion.Status) {
	t.Helper()
	original := kephalaionCheck
	kephalaionCheck = func(string) kephalaion.Status { return status }
	t.Cleanup(func() { kephalaionCheck = original })
}

// stubKephalaionBinary legt fest, ob kephalaion im PATH liegt.
func stubKephalaionBinary(t *testing.T, present bool) {
	t.Helper()
	original := kephalaionBinary
	kephalaionBinary = func() (string, bool) {
		if present {
			return "/opt/bin/kephalaion", true
		}
		return "", false
	}
	t.Cleanup(func() { kephalaionBinary = original })
}

func topLevelKeys(t *testing.T, value any) map[string]bool {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	fields := map[string]json.RawMessage{}
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	keys := map[string]bool{}
	for key := range fields {
		keys[key] = true
	}
	return keys
}

func TestDocTitleAndDescription(t *testing.T) {
	for _, tc := range []struct {
		content     string
		title, desc string
	}{
		{"---\ntitle: Aus dem Kopf\ndescription: Beschreibung.\n---\n\n# Überschrift\n", "Aus dem Kopf", "Beschreibung."},
		{"---\ndescription: Nur Beschreibung.\n---\n\n## Zweite Ebene\n\nText.\n", "Zweite Ebene", "Nur Beschreibung."},
		{"# Regel: Befunde\n\nText.\n", "Regel: Befunde", ""},
		{"Text.\n\n```\n# kein Titel\n```\n\n### Doch\n", "Doch", ""},
		{"#ohne Leerzeichen\nText.\n", "name.md", ""},
		{"", "name.md", ""},
	} {
		title, desc := docTitleAndDescription("name.md", []byte(tc.content))
		if title != tc.title || desc != tc.desc {
			t.Errorf("%q: %q/%q, erwartet %q/%q", tc.content, title, desc, tc.title, tc.desc)
		}
	}
}

// Ein Umgebungsfehler beim Lesen einer Ebene bricht ab — ein halbes Briefing
// sähe aus wie ein ganzes.
func TestBriefingUmgebungsfehler(t *testing.T) {
	assembled, err := briefingAssembled(context.Background(), DocLayers{Dist: failingDocSource{}, Local: failingDocSource{}}, briefingAlwaysDir)
	if err == nil || !errors.Is(err, err) || len(assembled.Parts) != 0 {
		t.Fatalf("erwartet Fehler, bekommen %+v", assembled)
	}
}

// Das Briefing setzt in den immer geltenden Anweisungen dieselben Includes
// ein wie `k-playbook read` für den Verweis — auch wenn ein Teil scheitert:
// der landet mit Kette unter problems, die übrigen bleiben, das Briefing
// kommt zustande.
func TestBriefingSetztIncludesEinWieRead(t *testing.T) {
	root := newContextProject(t)
	for rel, content := range map[string]string{
		"test-defaults/instructions/always/010-a.md":                    "## A\n\ninclude: k-playbook:/instructions/on-demand/baustein.md\n",
		"test-defaults/instructions/always/020-b.md":                    "## B\n",
		"test-defaults/instructions/on-demand/baustein.md":              "---\ntitle: Baustein\n---\n\nBaustein.\n",
		"test-collection/k-playbook/instructions/always/1000-kaputt.md": "## Kaputt\n\ninclude: fehlt.md\n",
		"test-collection/k-playbook/instructions/always/1100-eigen.md":  "## Eigen\n\ninclude: 020-b.md\n",
	} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		mkdirAll(t, filepath.Dir(path))
		write(t, path, content)
	}
	stubBriefingDocs(t, unavailableDocs)

	briefing, err := BuildBriefing(context.Background(), root)
	if err != nil {
		t.Fatalf("BuildBriefing: %v", err)
	}
	always := briefing.Documents.Always
	want := "## A\n\nBaustein.\n\n## B\n\n## Eigen\n\n## B\n"
	if always.Content != want {
		t.Errorf("always.content:\n%q\nerwartet\n%q", always.Content, want)
	}
	if !reflect.DeepEqual(briefingRefs(always.Parts), []string{
		"k-playbook:/instructions/always/010-a.md",
		"k-playbook:/instructions/always/020-b.md",
		"k-playbook:/instructions/always/1100-eigen.md",
	}) {
		t.Errorf("always.parts: %+v", always.Parts)
	}
	if len(always.Problems) != 1 || !strings.Contains(always.Problems[0], "1000-kaputt.md") ||
		!strings.Contains(always.Problems[0], "Include-Kette") {
		t.Errorf("always.problems: %v", always.Problems)
	}

	read, err := ResolveDocRef(ProjectDocLayers(root), always.Ref)
	if err != nil {
		t.Fatalf("ResolveDocRef: %v", err)
	}
	if read.Content != always.Content {
		t.Errorf("Briefing und read liefern verschiedene Texte:\n%q\n%q", always.Content, read.Content)
	}
	if !reflect.DeepEqual(read.Problems, always.Problems) {
		t.Errorf("Briefing und read melden verschiedene Probleme:\n%v\n%v", always.Problems, read.Problems)
	}
	if !reflect.DeepEqual(partNames(read.Parts), []string{"010-a.md", "020-b.md", "1100-eigen.md"}) {
		t.Errorf("read.parts: %+v", read.Parts)
	}
}

func briefingRefs(entries []BriefingEntry) []string {
	refs := make([]string, 0, len(entries))
	for _, entry := range entries {
		refs = append(refs, entry.Ref)
	}
	return refs
}

// Das Briefing trägt die Umgebungsprüfung als Abschnitt health (Task 098):
// derselbe Report wie health.Check — ohne Problem leer mit ok, sonst die
// Einträge sortiert; ein Fehler macht ok falsch. health ist kein Feld der
// Kontextausgabe und bewertet nur, was dort steht.
func TestBriefingTraegtHealth(t *testing.T) {
	root := newBriefingProject(t)
	stubBriefingDocs(t, unavailableDocs)
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	accesstest.Empty(t)
	stubKephalaionCheck(t, kephalaion.Status{Configured: true, State: kephalaion.StateOK, OK: true, Message: "Angemeldet."})

	briefing, err := BuildBriefing(t.Context(), root)
	if err != nil {
		t.Fatalf("BuildBriefing: %v", err)
	}
	if briefing.SchemaVersion != "8" {
		t.Errorf("schemaVersion %q, erwartet 8", briefing.SchemaVersion)
	}
	if !briefing.Health.OK || len(briefing.Health.Problems) != 0 {
		t.Errorf("health ohne Probleme: %+v", briefing.Health)
	}
	encoded, err := json.Marshal(briefing)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"health":{"ok":true,"problems":[]}`) {
		t.Errorf("health im JSON: %s", encoded)
	}

	// Ein Fehler und eine Warnung — die nicht eingetragene Anbindung ist seit
	// Task 131 ein Fehler, ein fehlendes Basis-Werkzeug eine Warnung: beide
	// da, ok falsch, Reihenfolge nach Bereich.
	stubKephalaionCheck(t, kephalaion.Status{State: kephalaion.StateNotConfigured, Message: "Keine Collection eingetragen."})
	write(t, filepath.Join(shipped(t), "scripts", "base-tools.tsv"), baseMatrixHeader+"k-weg\t-\tFehlt\tnein\tapt\tpaket\t-\t-\t-\n")
	briefing, err = BuildBriefing(t.Context(), root)
	if err != nil {
		t.Fatalf("BuildBriefing: %v", err)
	}
	report := briefing.Health
	if report.OK || len(report.Problems) != 2 {
		t.Fatalf("health: %+v", report)
	}
	if report.Problems[0].Area != health.AreaBaseTools || report.Problems[0].Level != health.LevelWarning ||
		report.Problems[1].Area != health.AreaKephalaion || report.Problems[1].Level != health.LevelError {
		t.Errorf("Einträge: %+v", report.Problems)
	}
	// Dieselbe Antwort wie health.Check selbst.
	if direct := health.Check(t.Context(), root); !reflect.DeepEqual(direct, report) {
		t.Errorf("health.Check:\n%+v\nBriefing:\n%+v", direct, report)
	}
	// Die Felder, die health bewertet, stehen weiter unverändert im Rest.
	if raw, ok := briefing.Context.Field("versionSources"); !ok || !strings.Contains(string(raw), `"state":"unavailable"`) ||
		!strings.Contains(string(raw), `"ref":"keph:/k-playbook/inventory.md"`) || !strings.Contains(string(raw), `"error":`) {
		t.Errorf("context.versionSources: %s", raw)
	}
}
