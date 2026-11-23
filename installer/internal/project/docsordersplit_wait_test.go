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
