package project

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
	"github.com/kascada/k-playbook/installer/internal/kephalaion/kephtest"
)

// findingsFakeDoc ist ein Eintrag im gefälschten Kephalaion der Vorprüfung.
type findingsFakeDoc struct {
	content          string
	by               string
	created          string
	updated          string
	ziel             string
	frontmatterError string
	// readErr lässt read scheitern; gone lässt read „none“ melden.
	readErr error
	gone    bool
}

// findingsFakeStore listet und liest Dokumente wie die Replica.
type findingsFakeStore struct {
	docs    map[string]*findingsFakeDoc
	listErr error
	lists   []kephalaion.ListRequest
	reads   []string
}

func (s *findingsFakeStore) List(_ context.Context, req kephalaion.ListRequest) ([]kephalaion.Entry, error) {
	s.lists = append(s.lists, req)
	if s.listErr != nil {
		return nil, s.listErr
	}
	var out []kephalaion.Entry
	revision := int64(0)
	for name, doc := range s.docs {
		revision++
		if !strings.HasPrefix(name, req.Path+"/") {
			continue
		}
		entry := kephalaion.Entry{Kind: "document", Name: name, Revision: revision,
			Created: &kephalaion.Stamp{At: doc.created, By: doc.by}, Updated: &kephalaion.Stamp{At: doc.updated, By: doc.by},
			FrontmatterError: doc.frontmatterError}
		if doc.ziel != "" {
			entry.Frontmatter = map[string]any{"title": "x", "ziel": doc.ziel}
		}
		out = append(out, entry)
	}
	return out, nil
}

func (s *findingsFakeStore) Read(_ context.Context, name string) (kephalaion.ReadResult, error) {
	s.reads = append(s.reads, name)
	doc, ok := s.docs[name]
	if !ok || doc.gone {
		return kephalaion.ReadResult{Kind: kephalaion.ReadKindNone, Name: name}, nil
	}
	if doc.readErr != nil {
		return kephalaion.ReadResult{}, doc.readErr
	}
	content := doc.content
	return kephalaion.ReadResult{Kind: kephalaion.ReadKindDocument, Name: name, Revision: 4711, Content: &content,
		Updated: &kephalaion.Stamp{At: doc.updated, By: doc.by}}, nil
}

// findingsNow ist der Zeitpunkt der Prüfung in den Tests.
var findingsNow = time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)

const findingsOld = "2026-11-01T08:00:00.000Z"

func befund(by, created string) *findingsFakeDoc {
	return &findingsFakeDoc{content: "---\ntitle: \"x\"\nziel:\n---\n\n**Befund:** y\n", by: by, created: created, updated: created}
}

// findingsTestEnv baut die Zugänge: Hub vm, Abgleich gelingt, Anbindung ok
// mit den Rechten rights, der Store store.
func findingsTestEnv(store FindingsStore, rights ...string) (findingsEnv, *[]string) {
	var synced []string
	revision := int64(7664)
	return findingsEnv{
		binding: func() (kephalaion.Binding, error) {
			return kephalaion.Binding{Hub: "vm", Collection: "k-playbook-dev"}, nil
		},
		sync: func(_ context.Context, hub string) error { synced = append(synced, hub); return nil },
		status: func() kephalaion.Status {
			return kephalaion.Status{OK: true, State: kephalaion.StateOK, Address: "vm:k-playbook-dev", User: "kamran",
				Rights: rights, Sync: &kephalaion.Sync{LastSuccess: "2026-11-17T11:59:00Z", Revision: &revision}}
		},
		open: func(kephalaion.Status) (FindingsStore, error) { return store, nil },
		now:  func() time.Time { return findingsNow },
	}, &synced
}

var allFindingsRights = []string{"read", "write", "supersede"}

func problemKinds(check FindingsCheck) []string {
	var kinds []string
	for _, p := range check.Problems {
		kinds = append(kinds, p.Kind+":"+p.Name)
	}
	return kinds
}

func TestFindingsCheckBereit(t *testing.T) {
	store := &findingsFakeStore{docs: map[string]*findingsFakeDoc{
		"queue/findings/002-b.md": befund("anna", "2026-11-02T08:00:00.000Z"),
		"queue/findings/001-a.md": befund("kamran", "2026-11-03T08:00:00.000Z"),
		"queue/findings/003-c.md": befund("kamran", findingsOld),
		// Eine andere Ablage zählt nicht mit.
		"queue/docs/001-auftrag.md": befund("kamran", findingsOld),
	}}
	store.docs["queue/findings/003-c.md"].ziel = "wissen"
	env, synced := findingsTestEnv(store, allFindingsRights...)
	check := env.check(context.Background())

	if !check.Ready || len(check.Problems) != 0 {
		t.Fatalf("nicht bereit: %+v", check.Problems)
	}
	if len(*synced) != 1 || (*synced)[0] != "vm" {
		t.Fatalf("Abgleich: %v", *synced)
	}
	if check.Total != 3 || check.ByUser["kamran"] != 2 || check.ByUser["anna"] != 1 || len(check.ByUser) != 2 {
		t.Fatalf("Bestand: total %d byUser %v", check.Total, check.ByUser)
	}
	var names []string
	for _, f := range check.Findings {
		names = append(names, f.Name)
	}
	// Ältester zuerst, nach created — nicht nach Nummer.
	if want := "queue/findings/003-c.md queue/findings/002-b.md queue/findings/001-a.md"; strings.Join(names, " ") != want {
		t.Fatalf("Reihenfolge: %v", names)
	}
	first := check.Findings[0]
	if first.Ref != "keph:/queue/findings/003-c.md" || first.Revision != 4711 || first.User != "kamran" || first.Ziel != "wissen" ||
		first.Created != findingsOld {
		t.Fatalf("Eintrag: %+v", first)
	}
	if check.Sync == nil || check.Sync.At != "2026-11-17T11:59:00Z" || check.Sync.Revision == nil || *check.Sync.Revision != 7664 {
		t.Fatalf("sync: %+v", check.Sync)
	}
	if len(store.lists) != 1 || store.lists[0].Path != "queue/findings" || !store.lists[0].Recursive || !store.lists[0].Frontmatter {
		t.Fatalf("list: %+v", store.lists)
	}
}

func TestFindingsCheckLeereQueue(t *testing.T) {
	env, _ := findingsTestEnv(&findingsFakeStore{docs: map[string]*findingsFakeDoc{}}, allFindingsRights...)
	check := env.check(context.Background())
	if !check.Ready || check.Total != 0 || check.Findings == nil || check.Skipped == nil || check.ByUser == nil {
		t.Fatalf("leere Queue: %+v", check)
	}
}

// Ohne supersede: nicht bereit, aber der Bestand wird trotzdem gezählt — der
// Command meldet die Zahl je Nutzer.
func TestFindingsCheckOhneSupersede(t *testing.T) {
	store := &findingsFakeStore{docs: map[string]*findingsFakeDoc{
		"queue/findings/001-a.md": befund("kamran", findingsOld),
		"queue/findings/002-b.md": befund("anna", findingsOld),
	}}
	env, _ := findingsTestEnv(store, "read", "write")
	check := env.check(context.Background())
	if check.Ready {
		t.Fatal("bereit ohne supersede")
	}
	if got := problemKinds(check); len(got) != 1 || got[0] != "right:supersede" {
		t.Fatalf("Probleme: %v", got)
	}
	if !strings.Contains(check.Problems[0].Message, "supersede") || !strings.Contains(check.Problems[0].Message, "Abhilfe") {
		t.Fatalf("Meldung: %s", check.Problems[0].Message)
	}
	if check.Total != 2 || check.ByUser["kamran"] != 1 || check.ByUser["anna"] != 1 {
		t.Fatalf("Bestand: %d %v", check.Total, check.ByUser)
	}
	if strings.Join(check.Rights, ",") != "read,write" {
		t.Fatalf("rights: %v", check.Rights)
	}
}

func TestFindingsCheckAbgleichGescheitert(t *testing.T) {
	store := &findingsFakeStore{docs: map[string]*findingsFakeDoc{"queue/findings/001-a.md": befund("kamran", findingsOld)}}
	env, _ := findingsTestEnv(store, allFindingsRights...)
	env.sync = func(context.Context, string) error { return errors.New("Der Abgleich ist gescheitert (Exit 1).") }
	check := env.check(context.Background())
	if check.Ready {
		t.Fatal("bereit nach gescheitertem Abgleich")
	}
	if got := problemKinds(check); len(got) != 1 || got[0] != "sync:" {
		t.Fatalf("Probleme: %v", got)
	}
	if check.Total != 1 {
		t.Fatalf("Bestand trotzdem gezählt: %d", check.Total)
	}
}

// Ohne Binary kephalaion: der echte Weg über kephalaion.SyncHub.
func TestFindingsCheckOhneKephalaionBinary(t *testing.T) {
	kephtest.NoBinary(t)
	env, _ := findingsTestEnv(&findingsFakeStore{docs: map[string]*findingsFakeDoc{}}, allFindingsRights...)
	env.sync = kephalaion.SyncHub
	check := env.check(context.Background())
	if check.Ready {
		t.Fatal("bereit ohne Binary")
	}
	if got := problemKinds(check); len(got) != 1 || got[0] != "sync:" || !strings.Contains(check.Problems[0].Message, "nicht installiert") {
		t.Fatalf("Probleme: %v %+v", got, check.Problems)
	}
}

func TestFindingsCheckAnbindungNichtBereit(t *testing.T) {
	store := &findingsFakeStore{docs: map[string]*findingsFakeDoc{}}
	env, _ := findingsTestEnv(store, allFindingsRights...)
	env.status = func() kephalaion.Status {
		return kephalaion.Status{State: kephalaion.StateNodeUnreachable, Message: "Der Node antwortet nicht."}
	}
	check := env.check(context.Background())
	if check.Ready || len(check.Problems) != 1 || check.Problems[0].Kind != FindingsProblemKephalaion {
		t.Fatalf("Anbindung: %+v", check)
	}
	if len(store.lists) != 0 {
		t.Fatal("gelistet ohne Anbindung")
	}
}

func TestFindingsCheckFalscheEintraege(t *testing.T) {
	store := &findingsFakeStore{docs: map[string]*findingsFakeDoc{
		"queue/findings/001-a.md":           befund("kamran", findingsOld),
		"queue/findings/Notiz.md":           befund("kamran", findingsOld),
		"queue/findings/12-zu-kurz.md":      befund("kamran", findingsOld),
		"queue/findings/003-text.txt":       befund("kamran", findingsOld),
		"queue/findings/unclear/004-x.md":   befund("kamran", findingsOld),
		"queue/findings/done/005-alt.md":    befund("kamran", findingsOld),
		"queue/findings/006-unlesbar.md":    {readErr: &kephalaion.ToolError{Tool: "read", Code: "internal", Message: "kaputt"}, by: "anna", created: findingsOld, updated: findingsOld},
		"queue/findings/007-weg.md":         {gone: true, by: "anna", created: findingsOld, updated: findingsOld},
		"queue/findings/008-frontmatter.md": befund("anna", findingsOld),
	}}
	store.docs["queue/findings/008-frontmatter.md"].frontmatterError = "YAML ungültig"
	env, _ := findingsTestEnv(store, allFindingsRights...)
	check := env.check(context.Background())
	if check.Ready {
		t.Fatal("bereit trotz falscher Einträge")
	}
	want := map[string]bool{
		"name:queue/findings/Notiz.md":                  true,
		"name:queue/findings/12-zu-kurz.md":             true,
		"name:queue/findings/003-text.txt":              true,
		"subdir:queue/findings/unclear/004-x.md":        true,
		"subdir:queue/findings/done/005-alt.md":         true,
		"unreadable:queue/findings/006-unlesbar.md":     true,
		"unreadable:queue/findings/007-weg.md":          true,
		"frontmatter:queue/findings/008-frontmatter.md": true,
	}
	got := problemKinds(check)
	if len(got) != len(want) {
		t.Fatalf("Probleme: %v", got)
	}
	for _, kind := range got {
		if !want[kind] {
			t.Errorf("unerwartet: %s", kind)
		}
	}
	if len(check.Findings) != 1 || check.Findings[0].Name != "queue/findings/001-a.md" {
		t.Fatalf("findings: %+v", check.Findings)
	}
	// Der unlesbare Frontmatter-Befund ist gelesen und zählt zum Bestand,
	// die unlesbaren nicht.
	if check.Total != 2 {
		t.Fatalf("total: %d", check.Total)
	}
}

// Leere und kürzlich geänderte Befunde werden übersprungen, nicht als
// Problem gezählt; sie stehen in skipped, nicht in findings.
func TestFindingsCheckUebersprungen(t *testing.T) {
	recent := findingsNow.Add(-2 * time.Minute).Format("2006-01-02T15:04:05.000Z")
	store := &findingsFakeStore{docs: map[string]*findingsFakeDoc{
		"queue/findings/001-a.md":    befund("kamran", findingsOld),
		"queue/findings/002-leer.md": {content: "", by: "anna", created: findingsOld, updated: findingsOld},
		"queue/findings/003-jung.md": befund("kamran", recent),
		"queue/findings/004-raum.md": {content: "  \n\n", by: "kamran", created: "2026-10-01T08:00:00.000Z", updated: findingsOld},
	}}
	env, _ := findingsTestEnv(store, allFindingsRights...)
	check := env.check(context.Background())
	if !check.Ready {
		t.Fatalf("nicht bereit: %+v", check.Problems)
	}
	if len(check.Findings) != 1 || check.Findings[0].Name != "queue/findings/001-a.md" {
		t.Fatalf("findings: %+v", check.Findings)
	}
	var got []string
	for _, s := range check.Skipped {
		got = append(got, s.Name+"="+s.Reason)
	}
	want := "queue/findings/004-raum.md=leer queue/findings/002-leer.md=leer queue/findings/003-jung.md=kürzlich geändert"
	if strings.Join(got, " ") != want {
		t.Fatalf("skipped: %v", got)
	}
	if check.Total != 4 || check.ByUser["anna"] != 1 || check.ByUser["kamran"] != 3 {
		t.Fatalf("Bestand: %d %v", check.Total, check.ByUser)
	}
}

func TestFindingsCheckListeScheitert(t *testing.T) {
	store := &findingsFakeStore{listErr: &kephalaion.ToolError{Tool: "list", Code: kephalaion.CodeNodeUnreachable, Message: "weg"}}
	env, _ := findingsTestEnv(store, allFindingsRights...)
	check := env.check(context.Background())
	if check.Ready || len(check.Problems) != 1 || check.Problems[0].Kind != FindingsProblemKephalaion {
		t.Fatalf("Liste: %+v", check.Problems)
	}
}

func TestFindingNamePattern(t *testing.T) {
	for _, name := range []string{"001-a.md", "226-k-danke-ohne-befund.md", "1000-x2.md"} {
		if !findingNamePattern.MatchString(name) {
			t.Errorf("%s abgelehnt", name)
		}
	}
	for _, name := range []string{"01-a.md", "001-.md", "001-A.md", "001-a--b.md", "001-a.md.bak", "001_a.md", "-001-a.md"} {
		if findingNamePattern.MatchString(name) {
			t.Errorf("%s angenommen", name)
		}
	}
}
