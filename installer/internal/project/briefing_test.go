package project

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
)

// newBriefingProject baut ein Projekt mit beiden Ebenen: ersetzt,
// abgeschaltet, nur lokal und nur mitgeliefert kommen in jeder Sorte vor.
func newBriefingProject(t *testing.T) string {
	t.Helper()

	root := newContextProject(t)
	files := map[string]string{
		"k-playbook/instructions/always/010-verweise.md":      "## Verweise\n\nText.\n",
		"k-playbook/instructions/always/100-wissen.md":        "---\ntitle: Projektwissen zuerst\ndescription: Doku vor Code.\n---\n\n## Projektwissen\n",
		"k-playbook/instructions/always/200-befunde.md":       "## Befunde festhalten\n",
		"k-playbook/instructions/always/README.md":            "# Liesmich\n",
		"k-playbook/instructions/on-demand/muster.md":         "---\ntitle: Muster\ndescription: Zeigt den Aufbau.\n---\n\n## Muster\n",
		"k-playbook/instructions/on-demand/release.md":        "## Release fahren\n",
		"k-playbook/rules/befunde.md":                         "# Regel: Befunde\n\nText.\n",
		"k-playbook/rules/docs-sync.md":                       "# Regel: Docs\n\nmitgeliefert\n",
		"k-playbook/rules/README.md":                          "# Regeln\n",
		"k-playbook/reviews/review-code.md":                   "---\nname: review-code\ntitle: Code-Review\n---\n# Code\n",
		"k-playbook/reviews/review-tech.md":                   "# Tech-Review\n",
		"k-playbook/reviews/notiz.md":                         "# kein Rezept\n",
		"k-playbook/guidelines/betrieb.md":                    "# Betrieb: Ablageorte\n\nText.\n",
		"k-playbook/guidelines/README.md":                     "# Guidelines\n",
		"k-playbook-local/instructions/always/100-wissen.md":  "## Projektwissen lokal\n\nText.\n",
		"k-playbook-local/instructions/always/200-befunde.md": "# Abgeschaltet: keine Befunde.\n",
		"k-playbook-local/instructions/always/1000-eigen.md":  "## Eigene Anweisung\n\nText.\n",
		"k-playbook-local/instructions/always/notiz.md":       "## Ohne Ziffer\n",
		"k-playbook-local/instructions/on-demand/release.md":  "",
		"k-playbook-local/instructions/on-demand/eigen.md":    "## Eigen auf Abruf\n\nText.\n",
		"k-playbook-local/rules/docs-sync.md":                 "# Regel: Docs\n\nlokal\n",
		"k-playbook-local/rules/eigene.md":                    "Text ohne Überschrift.\n",
		"k-playbook-local/reviews/review-tech.md":             "# Abgeschaltet.\n",
		"k-playbook-local/guidelines/stil.md":                 "# Stil\n\nText.\n",
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
	if briefing.ProjectDocs != "project:/k-playbook-local/docs/README.md" {
		t.Errorf("projectDocs %q", briefing.ProjectDocs)
	}

	always := briefing.Documents.Always
	if always.Ref != "k-playbook:/instructions/always/" {
		t.Errorf("always.ref %q", always.Ref)
	}
	wantAlways := []BriefingEntry{
		{Ref: "k-playbook:/instructions/always/010-verweise.md", Title: "Verweise", Origin: DocOriginDist},
		{Ref: "k-playbook:/instructions/always/100-wissen.md", Title: "Projektwissen lokal", Origin: DocOriginOverride},
		{Ref: "k-playbook:/instructions/always/200-befunde.md", Title: "200-befunde.md", Origin: BriefingOriginDisabled},
		{Ref: "k-playbook:/instructions/always/1000-eigen.md", Title: "Eigene Anweisung", Origin: DocOriginLocal},
	}
	if !reflect.DeepEqual(always.Parts, wantAlways) {
		t.Errorf("always.parts:\n%+v\nerwartet\n%+v", always.Parts, wantAlways)
	}
	if len(always.Problems) != 1 || !strings.Contains(always.Problems[0], "notiz.md") {
		t.Errorf("always.problems %v", always.Problems)
	}

	wantOnDemand := []BriefingEntry{
		{Ref: "k-playbook:/instructions/on-demand/eigen.md", Title: "Eigen auf Abruf", Origin: DocOriginLocal},
		{Ref: "k-playbook:/instructions/on-demand/muster.md", Title: "Muster", Description: "Zeigt den Aufbau.", Origin: DocOriginDist},
		{Ref: "k-playbook:/instructions/on-demand/release.md", Title: "release.md", Origin: BriefingOriginDisabled},
	}
	if !reflect.DeepEqual(briefing.Documents.OnDemand, wantOnDemand) {
		t.Errorf("onDemand:\n%+v\nerwartet\n%+v", briefing.Documents.OnDemand, wantOnDemand)
	}

	wantRules := []BriefingEntry{
		{Ref: "k-playbook:/rules/befunde.md", Title: "Regel: Befunde", Origin: DocOriginDist},
		{Ref: "k-playbook:/rules/docs-sync.md", Title: "Regel: Docs", Origin: DocOriginOverride},
		{Ref: "k-playbook:/rules/eigene.md", Title: "eigene.md", Origin: DocOriginLocal},
	}
	if !reflect.DeepEqual(briefing.Documents.Rules, wantRules) {
		t.Errorf("rules:\n%+v\nerwartet\n%+v", briefing.Documents.Rules, wantRules)
	}

	// Reviews folgen dem Muster des Katalogs: notiz.md ist kein Rezept.
	wantReviews := []BriefingEntry{
		{Ref: "k-playbook:/reviews/review-code.md", Title: "Code-Review", Origin: DocOriginDist},
		{Ref: "k-playbook:/reviews/review-tech.md", Title: "review-tech.md", Origin: BriefingOriginDisabled},
	}
	if !reflect.DeepEqual(briefing.Documents.Reviews, wantReviews) {
		t.Errorf("reviews:\n%+v\nerwartet\n%+v", briefing.Documents.Reviews, wantReviews)
	}

	// Guidelines sind überlagert: mitgeliefert und lokal, README nie.
	wantGuidelines := []BriefingEntry{
		{Ref: "k-playbook:/guidelines/betrieb.md", Title: "Betrieb: Ablageorte", Origin: DocOriginDist},
		{Ref: "k-playbook:/guidelines/stil.md", Title: "Stil", Origin: DocOriginLocal},
	}
	if !reflect.DeepEqual(briefing.Documents.Guidelines, wantGuidelines) {
		t.Errorf("guidelines:\n%+v\nerwartet\n%+v", briefing.Documents.Guidelines, wantGuidelines)
	}

	// Jeder Verweis des Briefings lässt sich über ResolveDocRef lesen, mit
	// derselben Herkunft.
	layers := ProjectDocLayers(root)
	for _, entries := range [][]BriefingEntry{always.Parts, briefing.Documents.OnDemand, briefing.Documents.Rules, briefing.Documents.Reviews, briefing.Documents.Guidelines} {
		for _, entry := range entries {
			result := mustResolveDoc(t, layers, entry.Ref)
			origin := result.Parts[0].Origin
			if result.Disabled {
				origin = BriefingOriginDisabled
			}
			if origin != entry.Origin {
				t.Errorf("%s: Briefing %s, Verweis %s", entry.Ref, entry.Origin, origin)
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
	if briefing.Documents.Always.Ref != "k-playbook:/instructions/always/" || len(briefing.Documents.Always.Parts) != 0 || briefing.Documents.Always.Problems == nil {
		t.Errorf("always: %+v", briefing.Documents.Always)
	}
	for name, list := range map[string][]BriefingEntry{
		"onDemand": briefing.Documents.OnDemand, "rules": briefing.Documents.Rules,
		"reviews": briefing.Documents.Reviews, "guidelines": briefing.Documents.Guidelines,
	} {
		if list == nil || len(list) != 0 {
			t.Errorf("%s: %v, erwartet leere Liste", name, list)
		}
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
		"user/kamran/tasks/done/099-alt.md",
		"user/hp/tasks/102-fremd.md",
		"user/kamran/workbench/001-umbau.md",
		"user/kamran/workbench/done/002-fertig.md",
		"queue/findings/001-a.md",
		"queue/findings/002-b.md",
		"queue/findings/003-c.md",
	)
	store.docs["user/kamran/tasks/100-briefing.md"].title = "Briefing bauen"
	store.docs["user/kamran/workbench/001-umbau.md"].title = "Umbau auf Kephalaion"
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
	}
	if !reflect.DeepEqual(work.Tasks, wantTasks) {
		t.Errorf("tasks: %+v", work.Tasks)
	}
	wantWorkbench := []BriefingDoc{
		{Ref: "keph:/user/kamran/workbench/001-umbau.md", Title: "Umbau auf Kephalaion", Updated: "2026-10-14T11:00:00.000Z"},
	}
	if !reflect.DeepEqual(work.Workbench, wantWorkbench) {
		t.Errorf("workbench: %+v", work.Workbench)
	}
	if work.OpenFindings != 3 {
		t.Errorf("openFindings %d, erwartet 3", work.OpenFindings)
	}
}

// Nichts doppelt: das Briefing trägt kein Feld der Kontextausgabe —
// Umgebungsangaben bleiben allein in `k-playbook context`. Verglichen werden
// die obersten Feldnamen beider Ausgaben; schemaVersion ist die einzige
// erlaubte Überschneidung, denn sie beschreibt die jeweilige Ausgabe selbst.
func TestBriefingTraegtKeinFeldDerKontextausgabe(t *testing.T) {
	root := newBriefingProject(t)
	stubBriefingDocs(t, unavailableDocs)

	context, err := BuildContext(root)
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}
	context.Kephalaion = &kephalaion.Status{}
	context.Links = &ContextLinks{}
	briefing, err := BuildBriefing(t.Context(), root)
	if err != nil {
		t.Fatalf("BuildBriefing: %v", err)
	}

	contextKeys := topLevelKeys(t, context)
	for key := range topLevelKeys(t, briefing) {
		if key == "schemaVersion" {
			continue
		}
		if contextKeys[key] {
			t.Errorf("Feld %q steht im Briefing und in der Kontextausgabe", key)
		}
	}
	// Die Dokumentlisten nennen Verweise, keine Pfade der Platte.
	data, err := json.Marshal(briefing.Documents)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), root) {
		t.Errorf("Briefing nennt den Projektpfad: %s", data)
	}
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
	assembled, err := briefingAssembled(DocLayers{Dist: failingDocSource{}, Local: failingDocSource{}}, briefingAlwaysDir)
	if err == nil || !errors.Is(err, err) || len(assembled.Parts) != 0 {
		t.Fatalf("erwartet Fehler, bekommen %+v", assembled)
	}
}
