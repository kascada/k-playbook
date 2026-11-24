package project

import (
	"context"
	"strings"
	"testing"
	"time"
)

// Split prüft die Wartezeit der rohen Aufträge der Eingabe (Task 184): Ein
// wartender allein wird abgelehnt und nichts geschrieben; mit einem vollen
// Lauf unter den rohen der Eingabe wird er aufgenommen; ein wartender
// außerhalb von raw bleibt offen; ein reifer bleibt wie bisher.
func TestDocsOrdersSplitWartezeit(t *testing.T) {
	at := docsOrdersClock()
	docsOrdersNow(t, at)
	ctx := context.Background()
	docs := []DocsSplitDocument{{Dokument: "11-a.md", Aktion: "neu", Pfade: []string{"a"}, Anlass: "x"}}

	t.Run("wartender allein", func(t *testing.T) {
		orders, store := newDocsOrders(t)
		putOrder(store, "queue/docs/001-wartet.md", rawOrder("s1", "2026-11-23", "a"), at.Add(-10*time.Minute))
		_, err := orders.Split(ctx, DocsSplitInput{Sitzung: "V1", Documents: docs, Raw: []string{"keph:/queue/docs/001-wartet.md"}})
		if DocRefErrorCode(err) != DocCodeInvalidInput || !strings.Contains(err.Error(), "wartet") {
			t.Fatalf("erwartet invalid_input: %v", err)
		}
		if _, ok := store.Docs["queue/docs/001-wartet.md"]; !ok {
			t.Error("wartender Auftrag verschwunden")
		}
		for name := range store.Docs {
			if strings.Contains(name, "code-11-a") || strings.Contains(name, "done/") {
				t.Errorf("geschrieben: %s", name)
			}
		}
	})

	t.Run("mit vollem Lauf", func(t *testing.T) {
		orders, store := newDocsOrders(t)
		putOrder(store, "queue/docs/001-wartet.md", rawOrder("s1", "2026-11-23", "a"), at.Add(-10*time.Minute))
		putOrder(store, "queue/docs/002-voll.md", rawOrder("s2", "2026-11-23", "."), at.Add(-5*time.Minute))
		result, err := orders.Split(ctx, DocsSplitInput{Sitzung: "V1", Documents: docs,
			Raw: []string{"keph:/queue/docs/001-wartet.md", "keph:/queue/docs/002-voll.md"}})
		if err != nil || len(result.ClosedRaw) != 2 {
			t.Fatalf("voller Lauf: %+v %v", result, err)
		}
	})

	t.Run("wartender außerhalb von raw", func(t *testing.T) {
		orders, store := newDocsOrders(t)
		putOrder(store, "queue/docs/001-wartet.md", rawOrder("s1", "2026-11-23", "b"), at.Add(-10*time.Minute))
		store.Put("queue/docs/002-reif.md", rawOrder("s2", "2026-11-20", "a"))
		result, err := orders.Split(ctx, DocsSplitInput{Sitzung: "V1", Documents: docs, Raw: []string{"keph:/queue/docs/002-reif.md"}})
		if err != nil || len(result.ClosedRaw) != 1 {
			t.Fatalf("reifer Auftrag: %+v %v", result, err)
		}
		if _, ok := store.Docs["queue/docs/001-wartet.md"]; !ok {
			t.Error("wartender Auftrag außerhalb von raw ist nicht offen geblieben")
		}
	})
}

// Orders und Split rechnen die Wartezeit auf dieselbe Weise (Task 191): Ohne
// lesbares updated zählt created. Ein solcher Auftrag mit frischem created
// wird als wartend gelistet und von Split abgelehnt — vorher listete Orders
// ihn als wartend, und Split zerlegte ihn trotzdem. Ohne beide Zeitpunkte ist
// er hier wie dort reif; unter problems meldet das allein Orders.
func TestDocsOrdersSplitWartezeitRueckfallAufCreated(t *testing.T) {
	at := docsOrdersClock()
	docsOrdersNow(t, at)
	ctx := context.Background()
	docs := []DocsSplitDocument{{Dokument: "11-a.md", Aktion: "neu", Pfade: []string{"a"}, Anlass: "x"}}
	stamp := func(d time.Duration) string { return at.Add(-d).UTC().Format(time.RFC3339Nano) }
	const name = "queue/docs/001-auftrag.md"
	const ref = "keph:/" + name

	for _, test := range []struct {
		name             string
		created, updated string
		waiting          bool
	}{
		{"frisches created ohne updated", stamp(30 * time.Minute), "", true},
		{"frisches created, updated unlesbar", stamp(30 * time.Minute), "gestern", true},
		{"altes created ohne updated", stamp(DocsOrderWait), "", false},
		{"ohne beide Zeitpunkte", "", "", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			orders, store := newDocsOrders(t)
			store.Put(name, rawOrder("s1", "2026-11-23", "a"))
			store.Docs[name].Created = test.created
			store.Docs[name].Updated = test.updated

			listed, err := orders.Orders(ctx, DocsOrderCode)
			if err != nil {
				t.Fatal(err)
			}
			if len(listed.Orders) != 1 || listed.Orders[0].Waiting != test.waiting {
				t.Fatalf("Orders: wartet %+v, erwartet %v", listed.Orders, test.waiting)
			}

			result, err := orders.Split(ctx, DocsSplitInput{Sitzung: "V1", Documents: docs, Raw: []string{ref}})
			if !test.waiting {
				if err != nil || len(result.ClosedRaw) != 1 {
					t.Fatalf("reifer Auftrag: %+v %v", result, err)
				}
				return
			}
			if DocRefErrorCode(err) != DocCodeInvalidInput {
				t.Fatalf("erwartet invalid_input: %+v %v", result, err)
			}
			// Die Ablehnung nennt die Wartezeit, nicht die „letzte Änderung“:
			// Beim Rückfall auf created gibt es keine.
			if !strings.Contains(err.Error(), "wartet noch (Wartezeit von 120 Minuten nicht um)") || strings.Contains(err.Error(), "letzte Änderung") {
				t.Errorf("Wortlaut der Ablehnung: %v", err)
			}
			if _, ok := store.Docs[name]; !ok {
				t.Error("wartender Auftrag verschwunden")
			}
			for written := range store.Docs {
				if strings.Contains(written, "code-11-a") || strings.Contains(written, "done/") {
					t.Errorf("geschrieben: %s", written)
				}
			}
		})
	}

	// Ohne beide Zeitpunkte: Orders meldet den Rückfall unter problems, das
	// Ergebnis von Split trägt dafür kein Feld.
	orders, store := newDocsOrders(t)
	store.Put(name, rawOrder("s1", "2026-11-23", "a"))
	store.Docs[name].Created, store.Docs[name].Updated = "", ""
	listed, err := orders.Orders(ctx, DocsOrderCode)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, problem := range listed.Problems {
		found = found || strings.Contains(problem, ref) && strings.Contains(problem, "gilt als reif")
	}
	if !found {
		t.Errorf("Orders meldet den Auftrag ohne Zeitpunkt nicht unter problems: %v", listed.Problems)
	}
}
