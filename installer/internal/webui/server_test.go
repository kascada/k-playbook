package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/kascada/k-playbook/installer/internal/guiproc"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// chdir macht ein Verzeichnis zum Arbeitsverzeichnis des Tests: die Handler
// leiten ihr Projekt über project.Detect() daraus ab.
func chdir(t *testing.T, dir string) {
	t.Helper()

	before, err := os.Getwd()
	if err != nil {
		t.Fatalf("Arbeitsverzeichnis: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("nach %s wechseln: %v", dir, err)
	}
	t.Cleanup(func() { os.Chdir(before) })
}

// getPage holt eine Seite über routes() — also über denselben Mux wie im
// Betrieb, samt Zuordnung von Pfad zu Handler und Vorlage.
func getPage(t *testing.T, path string) (int, string) {
	t.Helper()

	recorder := httptest.NewRecorder()
	routes(&serverState{}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	return recorder.Code, recorder.Body.String()
}

// Jede Seite trägt dasselbe Fragment der linken Spalte, und darin ist genau
// ein Bereich markiert. Ohne diesen Test fällt eine vergessene area-Übergabe,
// ein umbenanntes {{define "sidebar"}} oder eine Seite ohne
// {{template "sidebar" .}} erst im Browser auf.
//
// Geprüft wird Vorhandensein und Markierung, nicht die Vollständigkeit der
// Spalte: was später zusätzlich hineingesetzt wird, darf den Test nicht
// brechen.
func TestSeitenTragenDieLinkeSpalte(t *testing.T) {
	root := t.TempDir()
	if err := project.CreateConfig(root, "."); err != nil {
		t.Fatalf("Konfiguration anlegen: %v", err)
	}
	chdir(t, root)

	tests := []struct {
		path string
		// markiert ist der Eintrag des aktiven Bereichs, samt aria-current:
		// "page" nur auf der offenen Seite selbst.
		markiert string
		// fileIndex sagt, ob das Blockmenü den Dateiindex trägt und deshalb
		// auch schmal stehen bleibt.
		fileIndex bool
	}{
		{path: "/", markiert: `<a class="area-nav-item active" href="/" aria-current="page">`},
		{path: "/workflows", markiert: `<a class="area-nav-item active" href="/workflows" aria-current="page">`},
		{path: "/docs", markiert: `<a class="area-nav-item active" href="/docs" aria-current="page">`, fileIndex: true},
		// /inventory ist ein eigener Bereich neben Docs, mit kartenbasiertem
		// Blockmenü wie die Startseite.
		{path: "/inventory", markiert: `<a class="area-nav-item active" href="/inventory" aria-current="page">`},
		// /mcp ist die Detailseite des Setup-Blocks: der Bereich ist aktiv,
		// die Startseite darunter ist aber nicht offen.
		{path: "/mcp", markiert: `<a class="area-nav-item active" href="/" aria-current="true">`},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			status, body := getPage(t, test.path)
			if status != http.StatusOK {
				t.Fatalf("Status = %d, erwartet %d", status, http.StatusOK)
			}
			if !strings.Contains(body, `class="area-nav-item`) {
				t.Error("der Umschalter fehlt")
			}
			if !strings.Contains(body, `id="block-nav"`) {
				t.Error("das Blockmenü fehlt")
			}
			if !strings.Contains(body, `id="reload-page"`) {
				t.Error("der Knopf „Neu einlesen“ fehlt")
			}
			if count := strings.Count(body, "area-nav-item active"); count != 1 {
				t.Errorf("markierte Bereiche = %d, erwartet genau 1", count)
			}
			if !strings.Contains(body, test.markiert) {
				t.Errorf("der markierte Eintrag ist nicht %s", test.markiert)
			}
			if got := strings.Contains(body, "block-nav file-index"); got != test.fileIndex {
				t.Errorf("Modifier file-index = %v, erwartet %v", got, test.fileIndex)
			}
		})
	}
}

// Ohne Konfiguration führt der Umschalter nur nach Setup: Workflows und Docs
// hätten dort nichts zu zeigen.
func TestUmschalterOhneInstallation(t *testing.T) {
	chdir(t, t.TempDir())

	status, body := getPage(t, "/")
	if status != http.StatusOK {
		t.Fatalf("Status = %d, erwartet %d", status, http.StatusOK)
	}
	if count := strings.Count(body, `class="area-nav-item`); count != 1 {
		t.Errorf("Einträge im Umschalter = %d, erwartet genau 1", count)
	}
	if strings.Contains(body, `href="/workflows"`) || strings.Contains(body, `href="/docs"`) || strings.Contains(body, `href="/inventory"`) {
		t.Error("der Umschalter führt nach Workflows, Docs oder Inventar, obwohl nichts eingerichtet ist")
	}
}

// /api/health nennt Schlüssel, Version und PID: daran erkennt ein CLI-Aufruf
// den Server als seinen eigenen. Der Schlüssel ist das aufgelöste ProjectDir,
// die Version die vom Start.
func TestHealthNenntSchluesselVersionUndPID(t *testing.T) {
	root := t.TempDir()
	if err := project.CreateConfig(root, "."); err != nil {
		t.Fatalf("Konfiguration anlegen: %v", err)
	}
	chdir(t, root)

	recorder := httptest.NewRecorder()
	routes(&serverState{version: "v1.2.3"}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("Status = %d, erwartet %d", recorder.Code, http.StatusOK)
	}

	var health guiproc.Health
	if err := json.Unmarshal(recorder.Body.Bytes(), &health); err != nil {
		t.Fatalf("Antwort lesen: %v", err)
	}
	if health.Status != "ok" {
		t.Errorf("status = %q", health.Status)
	}
	want, err := guiproc.Key()
	if err != nil {
		t.Fatalf("Schlüssel: %v", err)
	}
	if health.Key != want {
		t.Errorf("key = %q, erwartet %q", health.Key, want)
	}
	if health.Version != "v1.2.3" {
		t.Errorf("version = %q", health.Version)
	}
	if health.PID != os.Getpid() {
		t.Errorf("pid = %d, erwartet %d", health.PID, os.Getpid())
	}
}

// Der Leerlaufwächter misst ab der letzten Anfrage, egal welcher. Geprüft mit
// injizierter Zeit, ohne zu warten.
func TestLeerlaufwaechterMitInjizierterZeit(t *testing.T) {
	start := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	state := &serverState{lastRequestAt: start}

	if state.idleExceeded(start.Add(idleTimeout - time.Second)) {
		t.Error("kurz vor der Grenze gilt schon als Leerlauf")
	}
	if !state.idleExceeded(start.Add(idleTimeout)) {
		t.Error("an der Grenze gilt nicht als Leerlauf")
	}

	// Eine Anfrage setzt die Uhr zurück.
	state.noteRequest(start.Add(idleTimeout))
	if state.idleExceeded(start.Add(idleTimeout + time.Minute)) {
		t.Error("eine Anfrage setzt den Leerlauf nicht zurück")
	}

	// Ohne Startzeit — nur in Tests, die die Routen prüfen — läuft nichts ab.
	if (&serverState{}).idleExceeded(start.Add(24 * time.Hour)) {
		t.Error("ohne Bezugszeit gilt als Leerlauf")
	}
}

// Jede Anfrage über die Routen zählt als Lebenszeichen, nicht nur /api/health.
func TestJedeAnfrageSetztDieLeerlaufuhr(t *testing.T) {
	chdir(t, t.TempDir())
	state := &serverState{}

	recorder := httptest.NewRecorder()
	routes(state).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/static/styles.css", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("Status = %d", recorder.Code)
	}
	if state.lastRequestAt.IsZero() {
		t.Error("die Anfrage wurde nicht festgehalten")
	}
}

// Der Heartbeat-Suizid ist weg: POST /api/client-gone gibt es nicht mehr. Der
// Mux antwortet mit 405, weil das Muster „GET /" den Pfad noch deckt — eine
// alte Seite, die den Endpunkt bis zum Neuladen weiter ruft, trifft ins Leere.
func TestClientGoneEntfallen(t *testing.T) {
	chdir(t, t.TempDir())

	recorder := httptest.NewRecorder()
	routes(&serverState{}).ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/client-gone", nil))
	if recorder.Code < http.StatusBadRequest {
		t.Errorf("Status = %d, erwartet einen Fehlerstatus", recorder.Code)
	}
}

// Schreibende Anfragen müssen von derselben Herkunft kommen: der Host-Anteil
// von Origin gegen den Host-Header, ohne Schema und ohne feste Adressliste.
func TestHerkunftspruefung(t *testing.T) {
	chdir(t, t.TempDir())

	tests := []struct {
		name   string
		method string
		url    string
		origin string
		want   int
	}{
		{name: "gleiche Herkunft erlaubt", method: http.MethodPost, url: "http://127.0.0.1:4711/api/shutdown", origin: "http://127.0.0.1:4711", want: http.StatusOK},
		{name: "fremde Herkunft 403", method: http.MethodPost, url: "http://127.0.0.1:4711/api/shutdown", origin: "http://boese.example", want: http.StatusForbidden},
		{name: "gleicher Host, anderer Port 403", method: http.MethodPost, url: "http://127.0.0.1:4711/api/shutdown", origin: "http://127.0.0.1:4712", want: http.StatusForbidden},
		{name: "opakes Origin 403", method: http.MethodPost, url: "http://127.0.0.1:4711/api/shutdown", origin: "null", want: http.StatusForbidden},
		// Weitergeleiteter Port: der Browser sieht einen anderen Host als den,
		// auf dem der Server lauscht, und schickt beides passend zueinander.
		{name: "weitergeleiteter Port erlaubt", method: http.MethodPost, url: "http://localhost:9999/api/shutdown", origin: "http://localhost:9999", want: http.StatusOK},
		// Codespaces terminiert TLS: https im Origin, http am Server.
		{name: "Fremddomain hinter TLS erlaubt", method: http.MethodPost, url: "http://x-8080.app.github.dev/api/shutdown", origin: "https://x-8080.app.github.dev", want: http.StatusOK},
		{name: "ohne Origin erlaubt", method: http.MethodPost, url: "http://127.0.0.1:4711/api/shutdown", want: http.StatusOK},
		{name: "GET bleibt ungeprüft", method: http.MethodGet, url: "http://127.0.0.1:4711/api/health", origin: "http://boese.example", want: http.StatusOK},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.url, nil)
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			recorder := httptest.NewRecorder()
			routes(&serverState{}).ServeHTTP(recorder, request)
			if recorder.Code != test.want {
				t.Errorf("Status = %d, erwartet %d (Body: %s)", recorder.Code, test.want, strings.TrimSpace(recorder.Body.String()))
			}
		})
	}
}

func TestOriginHost(t *testing.T) {
	for origin, want := range map[string]string{
		"http://127.0.0.1:4711":      "127.0.0.1:4711",
		"https://a.b:1/pfad?x=1":     "a.b:1",
		"http://localhost":           "localhost",
		"null":                       "null",
		"x-8080.app.github.dev/ohne": "x-8080.app.github.dev",
	} {
		if got := originHost(origin); got != want {
			t.Errorf("originHost(%q) = %q, erwartet %q", origin, got, want)
		}
	}
}
