package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/health"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// environmentCall ruft GET /api/environment über routes() — denselben Mux wie
// im Betrieb — und liest die Antwort.
func environmentCall(t *testing.T, path string) (int, environmentResponse, string) {
	t.Helper()
	recorder := httptest.NewRecorder()
	routes(&serverState{}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	var response environmentResponse
	if recorder.Code == http.StatusOK {
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("Antwort nicht lesbar: %v — %s", err, recorder.Body.String())
		}
	}
	return recorder.Code, response, recorder.Body.String()
}

// TestEnvironmentLiefertDenReport: der Endpunkt antwortet mit dem Report von
// health.Check unter health, in derselben Struktur wie `k-playbook health
// --json`. Das Projekt hier hat keine Kephalaion-Anbindung eingetragen — die
// Quelle meldet das als Warnung, ohne den Node zu fragen —, setzt gh nicht
// voraus, führt keine Matrix der Basis-Werkzeuge und keine
// Quellenkonfiguration: genau ein Eintrag, ok bleibt wahr.
func TestEnvironmentLiefertDenReport(t *testing.T) {
	root := t.TempDir()
	if err := project.CreateConfig(root, "."); err != nil {
		t.Fatalf("Konfiguration anlegen: %v", err)
	}
	chdir(t, root)

	code, response, body := environmentCall(t, "/api/environment")
	if code != http.StatusOK {
		t.Fatalf("Status = %d, erwartet 200: %s", code, body)
	}
	if !response.Available || response.Health == nil {
		t.Fatalf("Antwort ohne Report: %s", body)
	}
	report := *response.Health
	if !report.OK {
		t.Errorf("ok = false, erwartet true (eine Warnung lässt ok wahr): %s", body)
	}
	if len(report.Problems) != 1 {
		t.Fatalf("Problems = %+v, erwartet genau den Eintrag zur fehlenden Anbindung", report.Problems)
	}
	problem := report.Problems[0]
	if problem.Level != health.LevelWarning || problem.Area != health.AreaKephalaion {
		t.Errorf("Eintrag = %+v, erwartet warning im Bereich kephalaion", problem)
	}
	if problem.Message == "" || problem.Hint == "" {
		t.Errorf("Eintrag ohne Satz oder Abhilfe: %+v", problem)
	}
	// Dieselbe Struktur wie der Report der CLI: die vier Felder je Eintrag,
	// problems nie null.
	for _, want := range []string{`"ok":true`, `"problems":[{`, `"level":"warning"`, `"area":"kephalaion"`, `"message":"`, `"hint":"`} {
		if !strings.Contains(body, want) {
			t.Errorf("Antwort enthält %s nicht: %s", want, body)
		}
	}
}

// TestEnvironmentOhneProjekt: ohne Projektkonfiguration gibt es keinen Report,
// nur die Meldung — wie bei jedem GET der Oberfläche.
func TestEnvironmentOhneProjekt(t *testing.T) {
	chdir(t, t.TempDir())

	code, response, body := environmentCall(t, "/api/environment")
	if code != http.StatusOK {
		t.Fatalf("Status = %d, erwartet 200: %s", code, body)
	}
	if response.Available || response.Health != nil {
		t.Errorf("ohne Projekt wird ein Report geliefert: %s", body)
	}
	if response.Message == "" {
		t.Error("der Zustand wird verschwiegen statt gemeldet")
	}
}

// TestEnvironmentLaesstLebenszeichenUnberuehrt: GET /api/health bleibt das
// Lebenszeichen des Fensters (guiproc-Vertrag) und trägt den Report nicht.
func TestEnvironmentLaesstLebenszeichenUnberuehrt(t *testing.T) {
	recorder := httptest.NewRecorder()
	routes(&serverState{version: "v1", build: "b1"}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("Status = %d, erwartet 200", recorder.Code)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"status":"ok"`) {
		t.Errorf("Lebenszeichen ohne status ok: %s", body)
	}
	if strings.Contains(body, `"problems"`) {
		t.Errorf("das Lebenszeichen trägt den Report der Umgebungsprüfung: %s", body)
	}
}

// TestEnvironmentKarteWirdAusgeliefert: die Statusseite trägt die Karte
// „Umgebung“ mit Marke, Liste und Meldungsfeld, aus denen status.js die
// Anzeige baut; das Blockmenü entsteht aus id und h2 der Karte.
func TestEnvironmentKarteWirdAusgeliefert(t *testing.T) {
	root := t.TempDir()
	if err := project.CreateConfig(root, "."); err != nil {
		t.Fatalf("Konfiguration anlegen: %v", err)
	}
	chdir(t, root)

	status, page := getPage(t, "/")
	if status != http.StatusOK {
		t.Fatalf("Status = %d, erwartet 200", status)
	}
	for _, want := range []string{
		`id="environment-card"`,
		`<h2>Umgebung</h2>`,
		`id="environment-pill"`,
		`id="environment-problems"`,
		`id="environment-message"`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("Statusseite enthält %q nicht", want)
		}
	}
}
