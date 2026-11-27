package project

import (
	"context"
	"errors"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
)

// fakeFeedbackLister ist eine Collection, deren list das Frontmatter
// mitliefert, wie der Node es mit frontmatter tut.
type fakeFeedbackLister struct {
	entries []kephalaion.Entry
	err     error
	paths   []string
}

func (f *fakeFeedbackLister) List(_ context.Context, req kephalaion.ListRequest) ([]kephalaion.Entry, error) {
	f.paths = append(f.paths, req.Path)
	if !req.Frontmatter || !req.Recursive {
		return nil, errors.New("erwartet recursive und frontmatter")
	}
	return f.entries, f.err
}

func feedbackEntryWith(name, status, betrifft string) kephalaion.Entry {
	return kephalaion.Entry{Kind: "document", Name: name, Frontmatter: map[string]any{"status": status, "betrifft": betrifft}}
}

// Eine gestellte Collection-Liste: eine lesbare mit Meldungen, eine ohne
// feedback/, eine, die beim Lesen scheitert, und eine, die schon beim Öffnen
// scheitert. Gezählt werden nur offene direkt unter feedback/; die Fehler
// stehen an ihrer Collection, der Rest läuft weiter.
func TestCountFeedbackUeberCollections(t *testing.T) {
	readable := &fakeFeedbackLister{entries: []kephalaion.Entry{
		feedbackEntryWith("feedback/001-a.md", "offen", "kephalaion"),
		feedbackEntryWith("feedback/002-b.md", "offen", "kephalaion"),
		feedbackEntryWith("feedback/003-c.md", "offen", "k-playbook"),
		feedbackEntryWith("feedback/004-d.md", "angenommen", "k-playbook"),
		feedbackEntryWith("feedback/005-e.md", "offen", ""),
		feedbackEntryWith("feedback/done/006-f.md", "offen", "projekt"),
		feedbackEntryWith("feedback/tief/007-g.md", "offen", "projekt"),
		feedbackEntryWith("feedback/notiz.md", "offen", "projekt"),
		{Kind: "document", Name: "feedback/008-kaputt.md", FrontmatterError: "YAML ungültig"},
	}}
	empty := &fakeFeedbackLister{entries: []kephalaion.Entry{}}
	forbidden := &fakeFeedbackLister{err: &kephalaion.ToolError{Tool: "list", Code: "forbidden", Message: "kein Leserecht"}}
	sources := []FeedbackSource{
		{Address: "vm:omni", Store: readable},
		{Address: "vm:bid", Store: empty},
		{Address: "vm:gesperrt", Store: forbidden},
		{Address: "hub2:", Err: errors.New("nicht angemeldet")},
	}
	original := FeedbackSources
	var askedOwn string
	FeedbackSources = func(_ context.Context, own string) ([]FeedbackSource, error) {
		askedOwn = own
		return sources, nil
	}
	t.Cleanup(func() { FeedbackSources = original })

	counts, err := CountFeedbackElsewhere(context.Background(), "vm:k-playbook-dev")
	if err != nil || askedOwn != "vm:k-playbook-dev" || len(counts) != 4 {
		t.Fatalf("%+v %v", counts, err)
	}
	byAddress := map[string]FeedbackCollectionCount{}
	for _, count := range counts {
		byAddress[count.Address] = count
	}
	omni := byAddress["vm:omni"]
	if omni.Open != 4 || omni.ByBetrifft["kephalaion"] != 2 || omni.ByBetrifft["k-playbook"] != 1 ||
		omni.ByBetrifft["unbekannt"] != 1 || omni.Unreadable != 1 || omni.Error != "" {
		t.Errorf("omni: %+v", omni)
	}
	if readable.paths[0] != "feedback" {
		t.Errorf("gelesen: %v", readable.paths)
	}
	if bid := byAddress["vm:bid"]; bid.Open != 0 || bid.Error != "" {
		t.Errorf("ohne feedback/: %+v", bid)
	}
	if locked := byAddress["vm:gesperrt"]; locked.Error == "" || locked.Open != 0 {
		t.Errorf("Lesefehler: %+v", locked)
	}
	if hub := byAddress["hub2:"]; hub.Error != "nicht angemeldet" {
		t.Errorf("Hub: %+v", hub)
	}
	if counts[0].Address != "hub2:" || counts[3].Address != "vm:omni" {
		t.Errorf("nicht nach Adresse sortiert: %+v", counts)
	}
}

// Aus der Auskunft über die Hubs werden die lesbaren Collections außer der
// eigenen und der öffentlichen k-playbook gewählt, je Hub; ein nicht
// angemeldeter Hub ist ein Fehler.
func TestFeedbackTargetsWaehltLesbare(t *testing.T) {
	probes := []kephalaion.HubProbe{
		{Hub: "vm", State: kephalaion.StateOK, Collections: []kephalaion.Collection{
			{Collection: "k-playbook", Address: "vm:k-playbook", Rights: []string{"read"}},
			{Collection: "k-playbook-dev", Address: "vm:k-playbook-dev", Rights: []string{"read", "write"}},
			{Collection: "omni", Address: "vm:omni", Rights: []string{"read", "write"}},
			{Collection: "nur-dir", Address: "vm:nur-dir", Rights: []string{"dir docs/"}},
		}},
		{Hub: "hub2", State: kephalaion.StateOK, Collections: []kephalaion.Collection{
			{Collection: "kunde", Address: "hub2:kunde", Rights: []string{"read"}},
		}},
		{Hub: "hub3", State: kephalaion.StateLoginInvalid, Message: "Token ungültig"},
	}
	targets, failed := feedbackTargets(probes, "vm:k-playbook-dev")
	if len(targets) != 2 || targets[0].address != "vm:omni" || targets[1].address != "hub2:kunde" || targets[1].hub != "hub2" {
		t.Errorf("Ziele: %+v", targets)
	}
	if len(failed) != 1 || failed[0].Address != "hub3:" || failed[0].Error != "Token ungültig" {
		t.Errorf("Fehler: %+v", failed)
	}
}

// Mit AllCollections trägt die Liste die Zählung; scheitert die Auskunft
// über die Hubs, steht der Fehler daneben und die eigene Liste trotzdem da.
func TestFeedbackListMitAllenCollections(t *testing.T) {
	store := useFeedbackStore(t)
	putFeedback(store, "feedback/001-a.md", "fehler", "kephalaion", "offen")
	original := FeedbackSources
	t.Cleanup(func() { FeedbackSources = original })
	FeedbackSources = func(context.Context, string) ([]FeedbackSource, error) {
		return []FeedbackSource{{Address: "vm:omni", Store: &fakeFeedbackLister{entries: []kephalaion.Entry{
			feedbackEntryWith("feedback/001-x.md", "offen", "k-playbook")}}}}, nil
	}
	listing, err := ListFeedback(context.Background(), "", FeedbackListOptions{AllCollections: true})
	if err != nil || len(listing.Items) != 1 || len(listing.Collections) != 1 || listing.Collections[0].Open != 1 {
		t.Fatalf("%+v %v", listing, err)
	}
	FeedbackSources = func(context.Context, string) ([]FeedbackSource, error) { return nil, errors.New("Auskunft fehlt") }
	listing, err = ListFeedback(context.Background(), "", FeedbackListOptions{AllCollections: true})
	if err != nil || len(listing.Items) != 1 || listing.CollectionsError != "Auskunft fehlt" {
		t.Fatalf("Auskunft scheitert: %+v %v", listing, err)
	}
	listing, _ = ListFeedback(context.Background(), "", FeedbackListOptions{})
	if listing.Collections != nil || listing.CollectionsError != "" {
		t.Errorf("ohne AllCollections gezählt: %+v", listing)
	}
}
