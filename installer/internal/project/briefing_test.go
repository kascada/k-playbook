package project

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/kascada/k-playbook/installer/internal/githubaccess/accesstest"
	"os"
	"path"
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
	original := OpenBriefingDocs
	OpenBriefingDocs = open
	t.Cleanup(func() { OpenBriefingDocs = original })
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
	// Unter dem Deckel nennt total dieselbe Zahl wie die Liste.
	if work.Total != (BriefingWorkTotal{Tasks: 3, Workbench: 2}) {
		t.Errorf("total %+v", work.Total)
	}
}

// Die Arbeitsliste ist gedeckelt (Task 187): Von 40 offenen Tasks und 40
// aktiven Arbeitsständen stehen je BriefingWorkLimit im Briefing — die Tasks
// nach Nummer, die Arbeitsstände nach letzter Änderung, die neueste zuerst —,
// total nennt je Liste alle 40. Erledigte und fremde zählen nicht mit.
func TestBriefingArbeitslisteMitDeckel(t *testing.T) {
	root := newBriefingProject(t)
	names := []string{"user/kamran/tasks/done/001-alt.md", "user/hp/tasks/002-fremd.md", "user/kamran/workbench/done/001-fertig.md"}
	for i := 1; i <= 40; i++ {
		// Vierstellige Nummern neben dreistelligen: geordnet wird nach der
		// Zahl, nicht nach dem Namen (1000 stünde sonst vor 101).
		number := 100 + i
		if i > 35 {
			number = 1000 + i
		}
		names = append(names, fmt.Sprintf("user/kamran/tasks/%03d-task.md", number), fmt.Sprintf("user/kamran/workbench/%03d-thema.md", i))
	}
	store := newFakeDocStore("kamran", names...)
	for i := 1; i <= 40; i++ {
		// Je höher die Nummer, desto älter die Änderung — außer Thema 007,
		// das zuletzt geändert wurde.
		store.docs[fmt.Sprintf("user/kamran/workbench/%03d-thema.md", i)].updated = fmt.Sprintf("2026-11-%02dT10:00:00.000Z", 1+(40-i)/2)
	}
	store.docs["user/kamran/workbench/007-thema.md"].updated = "2026-11-24T09:00:00.000Z"
	stubBriefingDocs(t, func(string) (*UserDocs, error) { return newTestUserDocs(t, store, DocKinds), nil })

	briefing, err := BuildBriefing(context.Background(), root)
	if err != nil {
		t.Fatalf("BuildBriefing: %v", err)
	}
	work := briefing.Work
	if !work.Available || work.Total != (BriefingWorkTotal{Tasks: 40, Workbench: 40}) {
		t.Fatalf("work: available %v, total %+v", work.Available, work.Total)
	}
	if len(work.Tasks) != BriefingWorkLimit || len(work.Workbench) != BriefingWorkLimit {
		t.Fatalf("Listen: %d Tasks, %d Arbeitsstände, erwartet je %d", len(work.Tasks), len(work.Workbench), BriefingWorkLimit)
	}
	for i, doc := range work.Tasks {
		if want := fmt.Sprintf("keph:/user/kamran/tasks/%03d-task.md", 101+i); doc.Ref != want {
			t.Errorf("Task %d: %s, erwartet %s", i, doc.Ref, want)
		}
	}
	// Zuerst das zuletzt geänderte Thema, dann die mit dem jüngsten Datum;
	// bei gleichem Zeitpunkt bleibt die Reihenfolge nach Namen.
	wantWorkbench := []string{"007", "001", "002", "003", "004", "005", "006", "008", "009", "010"}
	for i, doc := range work.Workbench {
		if want := "keph:/user/kamran/workbench/" + wantWorkbench[i] + "-thema.md"; doc.Ref != want {
			t.Errorf("Arbeitsstand %d: %s, erwartet %s", i, doc.Ref, want)
		}
	}

	// Im JSON steht total neben den Listen.
	encoded, err := json.Marshal(work)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"total":{"tasks":40,"workbench":40}`) {
		t.Errorf("total im JSON: %s", encoded)
	}
}

// Die Reihenfolge der Tasks folgt der Nummer: vierstellig hinter dreistellig,
// gleiche Nummer nach Namen, ohne Nummer am Ende.
func TestSortBriefingDocsByNumber(t *testing.T) {
	docs := []BriefingDoc{
		{Ref: "keph:/user/k/tasks/1000-z.md"}, {Ref: "keph:/user/k/tasks/notiz.md"}, {Ref: "keph:/user/k/tasks/101-b.md"},
		{Ref: "keph:/user/k/tasks/099-c.md"}, {Ref: "keph:/user/k/tasks/101-a.md"},
	}
	sortBriefingDocsByNumber(docs)
	want := []string{"099-c.md", "101-a.md", "101-b.md", "1000-z.md", "notiz.md"}
	for i, doc := range docs {
		if got := path.Base(doc.Ref); got != want[i] {
			t.Errorf("Platz %d: %s, erwartet %s", i, got, want[i])
		}
	}
}

// Die eigenen Abschnitte (Task 187, Fassung 9): Was jede Sitzung braucht,
// steht im Briefing unter now, project, kephalaion und github — mit denselben
// Werten wie in der Kontextausgabe, aber nur mit den Feldern, die ein Text
// liest. Den Übergangsabschnitt context gibt es nicht mehr, und was der
// Teilabruf environment liefert, steht nicht im Briefing. `k-playbook context`
// selbst bleibt unverändert; das sichern die Tests der Kontextausgabe.
func TestBriefingTraegtEigeneAbschnitte(t *testing.T) {
	root := newBriefingProject(t)
	stubBriefingDocs(t, unavailableDocs)
	accesstest.Empty(t)
	stubKephalaionCheck(t, kephalaion.Status{Configured: true, Hub: "vm", Collection: "projekt", Account: "kamran-wsl",
		Address: "vm:projekt", State: kephalaion.StateOK, OK: true, Message: "Angemeldet als kamran-wsl (User kamran).",
		Rights: []string{"read", "write"}, User: "kamran", Node: "wsl", NodeVersion: "v0.4.2"})
	fixed := time.Date(2026, 11, 24, 10, 55, 30, 0, time.Local)
	previous := now
	now = func() time.Time { return fixed }
	t.Cleanup(func() { now = previous })

	built, err := BuildContext(root)
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}
	briefing, err := BuildBriefing(t.Context(), root)
	if err != nil {
		t.Fatalf("BuildBriefing: %v", err)
	}

	if briefing.Now != (ContextNow{Date: "2026-11-24", Timestamp: fixed.Format(time.RFC3339)}) {
		t.Errorf("now: %+v", briefing.Now)
	}
	wantProject := BriefingProject{Available: true, Dir: root, RepoRoot: built.Project.RepoRoot, VCS: built.Project.VCS,
		Languages: built.Project.Languages}
	if !reflect.DeepEqual(briefing.Project, wantProject) || len(briefing.Project.Languages) == 0 {
		t.Errorf("project:\n%+v\nerwartet\n%+v", briefing.Project, wantProject)
	}
	wantKephalaion := BriefingKephalaion{OK: true, Message: "Angemeldet als kamran-wsl (User kamran).", Address: "vm:projekt", User: "kamran"}
	if briefing.Kephalaion != wantKephalaion {
		t.Errorf("kephalaion: %+v", briefing.Kephalaion)
	}
	if briefing.GitHub.ProjectServices == nil || len(briefing.GitHub.ProjectServices) != 0 || briefing.GitHub.Access == nil {
		t.Errorf("github: %+v", briefing.GitHub)
	}

	// Im JSON genau diese obersten Felder — links fehlt, solange die
	// Selbstheilung nichts meldet, und kommt sonst dazu.
	encoded, err := json.Marshal(briefing)
	if err != nil {
		t.Fatal(err)
	}
	keys := topLevelKeys(t, briefing)
	delete(keys, "links")
	want := map[string]bool{"schemaVersion": true, "session": true, "now": true, "project": true, "kephalaion": true,
		"documents": true, "work": true, "projectDocs": true, "github": true, "health": true}
	if !reflect.DeepEqual(keys, want) {
		t.Errorf("oberste Felder %v, erwartet %v", keys, want)
	}
	var raws map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &raws); err != nil {
		t.Fatal(err)
	}
	for name, wantJSON := range map[string]string{
		"now":        `{"date":"2026-11-24","timestamp":` + string(mustJSON(t, fixed.Format(time.RFC3339))) + `}`,
		"kephalaion": `{"ok":true,"message":"Angemeldet als kamran-wsl (User kamran).","address":"vm:projekt","user":"kamran"}`,
		"github":     `{"projectServices":[]}`,
	} {
		if string(raws[name]) != wantJSON {
			t.Errorf("%s im JSON:\n%s\nerwartet\n%s", name, raws[name], wantJSON)
		}
	}
	// project trägt den Ort der Konfiguration nicht mehr.
	var projectFields map[string]json.RawMessage
	if err := json.Unmarshal(raws["project"], &projectFields); err != nil {
		t.Fatal(err)
	}
	if got := topLevelKeys(t, projectFields); !reflect.DeepEqual(got, map[string]bool{"available": true, "dir": true, "repoRoot": true, "vcs": true, "languages": true}) {
		t.Errorf("Felder von project: %v", got)
	}

	// Die Dokumentlisten nennen Verweise, keine Pfade der Platte.
	data, err := json.Marshal(briefing.Documents)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), root) {
		t.Errorf("Briefing nennt den Projektpfad: %s", data)
	}

	// Der Rückweg: Das JSON liest sich in dieselben Abschnitte zurück.
	var decoded Briefing
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded.Project, briefing.Project) || decoded.Kephalaion != briefing.Kephalaion || decoded.Now != briefing.Now {
		t.Errorf("Rückweg: %+v %+v %+v", decoded.Project, decoded.Kephalaion, decoded.Now)
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// Das Briefing nennt keinen Clone k-playbook/ mehr (Task 157, Fassung 7), auch
// wenn ein liegengebliebener im Projekt liegt, und seit Fassung 8 kein Feld
// local (Task 159): die einzige Wurzel ist project.dir. Seit Fassung 9
// (Task 187) fehlt der Abschnitt context ganz.
func TestBriefingNenntKeinPlaybook(t *testing.T) {
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
	if briefing.SchemaVersion != "9" {
		t.Errorf("schemaVersion %q, erwartet 9", briefing.SchemaVersion)
	}
	if !briefing.Project.Available || briefing.Project.Dir != root {
		t.Fatalf("project: %+v", briefing.Project)
	}

	encoded, err := json.Marshal(briefing)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &raw); err != nil {
		t.Fatal(err)
	}
	for _, absent := range []string{"context", "playbook", "local"} {
		if _, ok := raw[absent]; ok {
			t.Errorf("das Briefing trägt %s: %s", absent, raw[absent])
		}
	}
	if quoted, _ := json.Marshal(PlaybookDir(root)); strings.Contains(string(encoded), string(quoted)) {
		t.Errorf("das Briefing nennt %s:\n%s", PlaybookDir(root), encoded)
	}
}

// Lässt sich die Konfiguration nicht lesen, ist das ein Zustand an der Stelle
// des Abschnitts project — wie bei work —, kein Fehler: available false mit
// der Meldung, dir bleibt, das übrige Briefing bleibt. Eingerichtet wird dann
// nichts: Ein Werkzeug, das die Fassung nicht versteht, schreibt die
// Verlinkung nicht um. Der Teilabruf environment meldet denselben Zustand.
func TestBriefingProjektNichtLesbar(t *testing.T) {
	root := newBriefingProject(t)
	stubBriefingDocs(t, unavailableDocs)
	write(t, ConfigPath(root), "schema_version: 99\n")

	briefing, err := BuildBriefing(t.Context(), root)
	if err != nil {
		t.Fatalf("BuildBriefing: %v", err)
	}
	section := briefing.Project
	if section.Available || !strings.Contains(section.Message, "schema_version 99") || section.Dir != root ||
		section.RepoRoot != "" || section.VCS != "" || len(section.Languages) != 0 {
		t.Errorf("project: %+v", section)
	}
	if briefing.Links != nil {
		t.Errorf("links trotz unlesbarer Konfiguration: %+v", briefing.Links)
	}
	for _, name := range []string{".claude", ".opencode", ClaudeInstructionsFile} {
		if _, err := os.Lstat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Errorf("%s eingerichtet trotz unlesbarer Konfiguration (%v)", name, err)
		}
	}
	if briefing.Now.Date == "" || briefing.Kephalaion.Message == "" {
		t.Errorf("now %+v, kephalaion %+v", briefing.Now, briefing.Kephalaion)
	}
	if len(briefing.Documents.Reviews) == 0 || len(briefing.Documents.Always.Parts) == 0 {
		t.Errorf("Dokumente fehlen: %+v", briefing.Documents)
	}
	encoded, err := json.Marshal(section)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(encoded), `{"available":false,"message":"`) || !strings.HasSuffix(string(encoded), `,"dir":`+string(mustJSON(t, root))+`}`) {
		t.Errorf("project im JSON: %s", encoded)
	}

	environment := BuildBriefingEnvironment(root, root)
	if environment.Available || !strings.Contains(environment.Message, "schema_version 99") || environment.GH != nil ||
		environment.Remediation != nil {
		t.Errorf("environment: %+v", environment)
	}
	if encoded, _ := json.Marshal(environment); !strings.HasPrefix(string(encoded), `{"available":false,"message":"`) || strings.Count(string(encoded), `":`) != 2 {
		t.Errorf("environment im JSON: %s", encoded)
	}
}

// Das Briefing steht am Anfang jeder Sitzung und zieht deshalb selbst die
// Assistenten-Verlinkung nach (Task 187) — bis dahin kam die Heilung nur
// nebenbei über die Kontextausgabe mit. Was sich geändert hat, steht unter links;
// beim zweiten Briefing gibt es nichts mehr zu melden, und das Feld fehlt.
func TestBriefingZiehtVerlinkungNach(t *testing.T) {
	root := newBriefingProject(t)
	stubBriefingDocs(t, unavailableDocs)
	writeFile(t, filepath.Join(shipped(t), "commands", "k-test.md"), "test\n")
	writeFile(t, filepath.Join(shipped(t), "skills", "beispiel", skillFileName), "# Beispiel\n")

	briefing, err := BuildBriefing(t.Context(), root)
	if err != nil {
		t.Fatalf("BuildBriefing: %v", err)
	}
	if briefing.Links == nil {
		t.Fatal("die Selbstheilung muss im Briefing stehen")
	}
	if briefing.Links.Note == "" {
		t.Error("ohne Klartext weiß ein Assistent nicht, was das für seine Sitzung heißt")
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "commands", "k-test.md")); err != nil {
		t.Errorf("der Command ist nicht registriert: %v", err)
	}
	if !LinksOK(CheckLinks(root)) {
		t.Errorf("nach dem Briefing steht die Verlinkung nicht: %+v", CheckLinks(root))
	}
	encoded, err := json.Marshal(briefing)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"links":{`) || !strings.Contains(string(encoded), `"note":"`) {
		t.Errorf("links im JSON: %s", encoded)
	}

	again, err := BuildBriefing(t.Context(), root)
	if err != nil {
		t.Fatalf("BuildBriefing: %v", err)
	}
	if again.Links != nil {
		t.Errorf("links = %+v, erwartet nichts zu melden", again.Links)
	}
	if encoded, _ := json.Marshal(again); strings.Contains(string(encoded), `"links"`) {
		t.Errorf("das zweite Briefing trägt links: %s", encoded)
	}

	// Der Teilabruf environment heilt nicht: Er schreibt nichts.
	commands := filepath.Join(shipped(t), "commands")
	if err := os.Rename(filepath.Join(commands, "k-test.md"), filepath.Join(commands, "k-neu.md")); err != nil {
		t.Fatal(err)
	}
	if environment := BuildBriefingEnvironment(root, root); !environment.Available {
		t.Fatalf("environment: %+v", environment)
	}
	if _, err := os.Lstat(filepath.Join(root, ".claude", "commands", "k-neu.md")); !os.IsNotExist(err) {
		t.Errorf("der Teilabruf hat die Verlinkung nachgezogen (%v)", err)
	}
}

// Versteht das Werkzeug die Konfiguration nicht, schreibt auch das Briefing
// nicht: Es meldet den Zustand unter project und lässt die Verlinkung stehen.
func TestBriefingHeiltNichtBeiUnbekannterFassung(t *testing.T) {
	root := newBriefingProject(t)
	stubBriefingDocs(t, unavailableDocs)
	writeFile(t, filepath.Join(shipped(t), "commands", "k-test.md"), "test\n")
	write(t, ConfigPath(root), "schema_version: 99\n\nproject:\n  repo_root: .\n  vcs: git\n")

	briefing, err := BuildBriefing(t.Context(), root)
	if err != nil {
		t.Fatalf("BuildBriefing: %v", err)
	}
	if briefing.Project.Available || briefing.Links != nil {
		t.Errorf("project %+v, links %+v", briefing.Project, briefing.Links)
	}
	if _, err := os.Lstat(filepath.Join(root, ".claude")); !os.IsNotExist(err) {
		t.Error("bei unlesbarer Konfiguration darf nichts verlinkt worden sein")
	}
}

// Der Teilabruf environment (Task 187) liefert die Umgebungsangaben, die das
// Briefing nicht mehr trägt, unter denselben Namen und mit denselben Werten
// wie ihre Leser — BuildContext für gh, baseTools, docs und remediation,
// contextGitHub und versionSourcesWithStatus für github und versionSources —,
// ohne die Felder, die kein Text liest: von gh bleiben ready und account,
// baseTools verliert matrix; schemaVersion, mcp, git, catalogs, now, project,
// kephalaion und links gehören nicht dazu. Er trägt weder session noch
// documents noch work.
func TestBriefingEnvironment(t *testing.T) {
	root := newBriefingProject(t)
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	accesstest.Empty(t)
	stubKephalaionCheck(t, kephalaion.Status{State: kephalaion.StateNotConfigured, Message: "Keine Collection eingetragen."})

	full, err := BuildContext(root)
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}
	environment := BuildBriefingEnvironment(root, root)
	if !environment.Available || environment.Message != "" {
		t.Fatalf("environment: %+v", environment)
	}
	encoded, err := json.Marshal(environment)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"available": true, "gh": true, "github": true, "baseTools": true, "versionSources": true, "docs": true, "remediation": true}
	if keys := topLevelKeys(t, environment); !reflect.DeepEqual(keys, want) {
		t.Errorf("Felder %v, erwartet %v", keys, want)
	}

	if *environment.GH != (BriefingGH{Ready: full.GH.Ready, Account: full.GH.Account}) {
		t.Errorf("gh: %+v, Kontextausgabe %+v", environment.GH, full.GH)
	}
	if got := topLevelKeys(t, environment.GH); !reflect.DeepEqual(got, map[string]bool{"ready": true, "account": true}) {
		t.Errorf("Felder von gh: %v", got)
	}
	tools := environment.BaseTools
	if tools.Present != full.BaseTools.Present || tools.OK != full.BaseTools.OK || tools.Error != full.BaseTools.Error ||
		tools.InstallCommand != full.BaseTools.InstallCommand || !reflect.DeepEqual(tools.Missing, full.BaseTools.Missing) {
		t.Errorf("baseTools: %+v, Kontextausgabe %+v", tools, full.BaseTools)
	}
	if strings.Contains(string(encoded), `"matrix"`) {
		t.Errorf("baseTools trägt matrix: %s", encoded)
	}
	// Die übrigen Unterfelder unverändert, wie ihre Leser sie liefern.
	for name, pair := range map[string][2]any{
		"github":         {environment.GitHub, contextGitHub(root)},
		"versionSources": {environment.VersionSources, versionSourcesWithStatus(root, KephalaionStatus(root))},
		"docs":           {environment.Docs, full.Docs},
		"remediation":    {environment.Remediation, full.Remediation},
	} {
		got, want := mustJSON(t, pair[0]), mustJSON(t, pair[1])
		if string(got) != string(want) {
			t.Errorf("%s:\n%s\nerwartet\n%s", name, got, want)
		}
	}
	if environment.VersionSources.State != VersionSourcesUnavailable || environment.VersionSources.Ref != "keph:/k-playbook/inventory.md" {
		t.Errorf("versionSources: %+v", environment.VersionSources)
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
// Einträge sortiert; ein Fehler macht ok falsch. health bewertet und nennt
// Abhilfen; die Rohwerte dazu liefert der Teilabruf environment.
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
	if briefing.SchemaVersion != "9" {
		t.Errorf("schemaVersion %q, erwartet 9", briefing.SchemaVersion)
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
	// Die Felder, die health bewertet, liefert der Teilabruf environment als
	// Rohwerte.
	sources := BuildBriefingEnvironment(root, root).VersionSources
	if sources == nil || sources.State != VersionSourcesUnavailable || sources.Ref != "keph:/k-playbook/inventory.md" || sources.Error == "" {
		t.Errorf("environment.versionSources: %+v", sources)
	}
}
