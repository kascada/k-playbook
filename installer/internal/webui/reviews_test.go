package webui

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/project"
	"github.com/kascada/k-playbook/installer/internal/review"
)

// GET /api/reviews liefert keine Läufe, sondern die Meldung der Sperre
// (review.PausedMessage): paused ist wahr, ein Feld runs gibt es nicht, und im
// Projekt entsteht nichts. Ohne Projekt meldet der Endpunkt nur, dass er nicht
// anwendbar ist.
func TestReviewsEndpunktMeldetDieSperre(t *testing.T) {
	root := t.TempDir()
	if err := project.CreateConfig(root, "."); err != nil {
		t.Fatalf("Konfiguration anlegen: %v", err)
	}
	chdir(t, root)

	var raw map[string]json.RawMessage
	if code := getJSON(t, "/api/reviews", &raw); code != http.StatusOK {
		t.Fatalf("Status = %d", code)
	}
	if _, found := raw["runs"]; found {
		t.Errorf("die Antwort trägt Läufe: %s", raw["runs"])
	}
	var response reviewsResponse
	getJSON(t, "/api/reviews", &response)
	if !response.Available || !response.Paused || response.Message != review.PausedMessage {
		t.Errorf("Antwort = %+v", response)
	}

	chdir(t, t.TempDir())
	response = reviewsResponse{}
	getJSON(t, "/api/reviews", &response)
	if response.Available || response.Paused || response.Message != "" {
		t.Errorf("ohne Projekt: %+v", response)
	}
}

// Die Seite Reviews und die Karte der Übersicht nennen keinen Ort der
// Ergebnisse und zeigen keine Liste von Läufen: Die Seite trägt nur noch das
// Meldungsziel, die Karte das Element für den Satz der Sperre.
func TestReviewSeiteZeigtDieSperreStattLaeufen(t *testing.T) {
	root := t.TempDir()
	if err := project.CreateConfig(root, "."); err != nil {
		t.Fatalf("Konfiguration anlegen: %v", err)
	}
	chdir(t, root)

	status, body := getPage(t, "/workflows/reviews")
	if status != http.StatusOK {
		t.Fatalf("Status = %d", status)
	}
	if !strings.Contains(body, `id="runs-message"`) || strings.Contains(body, `id="runs-list"`) {
		t.Errorf("die Seite trägt das Meldungsziel nicht oder noch eine Liste der Läufe")
	}
	if strings.Contains(body, "results/") {
		t.Errorf("die Seite nennt einen Ort der Ergebnisse")
	}

	status, body = getPage(t, "/workflows")
	if status != http.StatusOK {
		t.Fatalf("Status = %d", status)
	}
	if !strings.Contains(body, `id="reviews-paused"`) || strings.Contains(body, "results/") {
		t.Errorf("die Karte Reviews trägt das Element der Sperre nicht oder nennt einen Ort")
	}
}
