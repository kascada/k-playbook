package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/kascada/k-playbook/installer/internal/buildinfo"
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
		// unterpunkt ist der markierte Eintrag unter dem Bereich. Nur die drei
		// Seiten von Workflows haben einen; leer heißt: keiner ist markiert.
		unterpunkt string
		// fileIndex sagt, ob das Blockmenü den Dateiindex trägt und deshalb
		// auch schmal stehen bleibt.
		fileIndex bool
	}{
		{path: "/", markiert: `<a class="area-nav-item active" href="/" aria-current="page">`},
		{path: "/setup", markiert: `<a class="area-nav-item active" href="/setup" aria-current="page">`},
		{path: "/workflows", markiert: `<a class="area-nav-item active" href="/workflows" aria-current="page">`},
		// Die drei Seiten des Bereichs: Workflows ist aktiv, seine Übersicht
		// ist aber nicht offen — offen ist der Unterpunkt.
		{
			path:       "/workflows/tasks",
			markiert:   `<a class="area-nav-item active" href="/workflows" aria-current="true">`,
			unterpunkt: `<a class="area-nav-subitem active" href="/workflows/tasks" aria-current="page">`,
		},
		{
			path:       "/workflows/reviews",
			markiert:   `<a class="area-nav-item active" href="/workflows" aria-current="true">`,
			unterpunkt: `<a class="area-nav-subitem active" href="/workflows/reviews" aria-current="page">`,
		},
		{
			path:       "/workflows/todos",
			markiert:   `<a class="area-nav-item active" href="/workflows" aria-current="true">`,
			unterpunkt: `<a class="area-nav-subitem active" href="/workflows/todos" aria-current="page">`,
		},
		// /chat ist ein eigener Bereich unter Workflows, mit kartenbasiertem
		// Blockmenü.
		{path: "/chat", markiert: `<a class="area-nav-item active" href="/chat" aria-current="page">`},
		// Die Seite einer Sitzung trägt den Bereich Chat, ist aber nicht
		// dessen Übersicht.
		{path: "/chat/ses_abc123", markiert: `<a class="area-nav-item active" href="/chat" aria-current="true">`},
		// /github ist ein eigener Bereich: seine Karten fragen als einzige
		// der Oberfläche über das Netz, und das darf weder die Statusseite
		// noch das Menü auslösen.
		{path: "/github", markiert: `<a class="area-nav-item active" href="/github" aria-current="page">`},
		// /knowledge ist ein eigener Bereich über Docs: er zeigt das Wissen
		// des Projekts, nicht das Nachschlagewerk der Installation.
		{path: "/knowledge", markiert: `<a class="area-nav-item active" href="/knowledge" aria-current="page">`},
		{path: "/docs", markiert: `<a class="area-nav-item active" href="/docs" aria-current="page">`, fileIndex: true},
		// /inventory ist ein eigener Bereich neben Docs, mit kartenbasiertem
		// Blockmenü wie die Setup-Seite.
		{path: "/inventory", markiert: `<a class="area-nav-item active" href="/inventory" aria-current="page">`},
		// /mcp ist die Detailseite des Setup-Blocks: der Bereich ist aktiv,
		// die Setup-Seite darunter ist aber nicht offen.
		{path: "/mcp", markiert: `<a class="area-nav-item active" href="/setup" aria-current="true">`},
		// /mcp-servers ist der Unterpunkt von Setup: der Bereich ist aktiv, der
		// Unterpunkt ist die offene Seite.
		{
			path:       "/mcp-servers",
			markiert:   `<a class="area-nav-item active" href="/setup" aria-current="true">`,
			unterpunkt: `<a class="area-nav-subitem active" href="/mcp-servers" aria-current="page">`,
		},
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
			// Die Übersicht von Workflows hat kein Blockmenü: ihre Karten
			// sind die drei Seiten, die als Unterpunkte schon im Umschalter
			// stehen.
			wantBlockNav := test.path != "/workflows"
			if got := strings.Contains(body, `id="block-nav"`); got != wantBlockNav {
				t.Errorf("Blockmenü vorhanden = %v, erwartet %v", got, wantBlockNav)
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
			erwarteteUnterpunkte := 0
			if test.unterpunkt != "" {
				erwarteteUnterpunkte = 1
				if !strings.Contains(body, test.unterpunkt) {
					t.Errorf("der markierte Unterpunkt ist nicht %s", test.unterpunkt)
				}
			}
			if count := strings.Count(body, "area-nav-subitem active"); count != erwarteteUnterpunkte {
				t.Errorf("markierte Unterpunkte = %d, erwartet %d", count, erwarteteUnterpunkte)
			}
			// Die drei Seiten des Workflows-Bereichs und der Unterpunkt von Setup
			// stehen im Umschalter jeder Seite: von Setup aus soll der Weg zu den
			// Tasks nicht erst über die Übersicht führen, und umgekehrt.
			for _, sub := range []string{"/mcp-servers", "/workflows/tasks", "/workflows/reviews", "/workflows/todos"} {
				if !strings.Contains(body, `href="`+sub+`"`) {
					t.Errorf("der Unterpunkt %s fehlt im Umschalter", sub)
				}
			}
			if got := strings.Contains(body, "block-nav file-index"); got != test.fileIndex {
				t.Errorf("Modifier file-index = %v, erwartet %v", got, test.fileIndex)
			}
		})
	}
}

// Jeder Hilfe-Block der Workflows-Seiten verweist in die mitgelieferte Doku,
// und zwar immer auf demselben Weg: /docs?file=<datei>. Der Bereich Docs macht
// daraus die gelesene Datei — eine zweite Ansicht gibt es dafür nicht. Ohne
// diesen Test fiele ein vertippter oder verlorener Verweis erst im Browser auf.
func TestHilfeVerweiseZeigenInDieDoku(t *testing.T) {
	root := t.TempDir()
	if err := project.CreateConfig(root, "."); err != nil {
		t.Fatalf("Konfiguration anlegen: %v", err)
	}
	chdir(t, root)

	for path, dateien := range map[string][]string{
		"/workflows":         {"commands.md"},
		"/workflows/tasks":   {"task-flow.md"},
		"/workflows/reviews": {"code-review.md", "review-runs.md"},
		"/workflows/todos":   {"commands.md"},
	} {
		t.Run(path, func(t *testing.T) {
			status, body := getPage(t, path)
			if status != http.StatusOK {
				t.Fatalf("Status = %d, erwartet %d", status, http.StatusOK)
			}
			for _, datei := range dateien {
				want := `<a class="doc-link" href="/docs?file=` + datei + `"`
				if !strings.Contains(body, want) {
					t.Errorf("der Verweis %s fehlt", want)
				}
			}
		})
	}
}

// Jede Seite trägt rechts oben die Version des Binarys, das sie ausliefert.
// Die Installation daneben kann einen anderen Stand haben; ohne die Marke
// verriete ein Fenster nach einem Update nicht, welcher Server antwortet.
// Ein Build ohne Flags zeigt das sichtbar an, statt eine leere Marke zu
// tragen.
func TestSeitenTragenDieVersion(t *testing.T) {
	root := t.TempDir()
	if err := project.CreateConfig(root, "."); err != nil {
		t.Fatalf("Konfiguration anlegen: %v", err)
	}
	chdir(t, root)

	before := buildinfo.Version
	t.Cleanup(func() { buildinfo.Version = before })

	for _, path := range []string{"/", "/setup", "/workflows", "/workflows/tasks", "/workflows/reviews", "/workflows/todos", "/chat", "/chat/ses_abc123", "/knowledge", "/docs", "/inventory", "/mcp", "/mcp-servers"} {
		t.Run(path, func(t *testing.T) {
			buildinfo.Version = "v1.2.3"
			status, body := getPage(t, path)
			if status != http.StatusOK {
				t.Fatalf("Status = %d, erwartet %d", status, http.StatusOK)
			}
			if !strings.Contains(body, `class="version-badge"`) {
				t.Error("die Versionsmarke fehlt")
			}
			if !strings.Contains(body, ">v1.2.3</span>") {
				t.Error("die Versionsmarke nennt nicht v1.2.3")
			}

			buildinfo.Version = ""
			_, body = getPage(t, path)
			if !strings.Contains(body, ">ohne Version</span>") {
				t.Error("ein Build ohne Version zeigt keine Marke „ohne Version“")
			}
		})
	}
}

// Ohne Konfiguration führt der Umschalter nur nach Setup: Workflows und Docs
// hätten dort nichts zu zeigen. Er steht dort, wo man ohne Konfiguration
// landet — auf /setup, denn / leitet dorthin um.
func TestUmschalterOhneInstallation(t *testing.T) {
	chdir(t, t.TempDir())

	status, body := getPage(t, "/setup")
	if status != http.StatusOK {
		t.Fatalf("Status = %d, erwartet %d", status, http.StatusOK)
	}
	if count := strings.Count(body, `class="area-nav-item`); count != 1 {
		t.Errorf("Einträge im Umschalter = %d, erwartet genau 1", count)
	}
	for _, ziel := range []string{"/workflows", "/chat", "/knowledge", "/docs", "/inventory"} {
		if strings.Contains(body, `href="`+ziel+`"`) {
			t.Errorf("der Umschalter führt nach %s, obwohl nichts eingerichtet ist", ziel)
		}
	}
	// Status führte auf eine sofortige Umleitung und fehlt deshalb.
	if strings.Contains(body, `href="/"`) {
		t.Error("der Umschalter führt nach /, obwohl nichts eingerichtet ist")
	}
	// Die Unterpunkte hängen an demselben Zweig und dürfen ihn nicht überleben.
	if strings.Contains(body, `class="area-nav-subitem`) {
		t.Error("der Umschalter zeigt Unterpunkte, obwohl nichts eingerichtet ist")
	}
}

// Ohne Projektkonfiguration leitet / in die Einrichtung um; mit ihr zeigt /
// die Statusseite. Ein unbekannter Pfad bleibt in beiden Fällen 404: GET / ist
// das Auffangmuster, und die Umleitung darf keinen Tippfehler nach /setup
// schicken.
func TestStartseiteLeitetOhneKonfigurationUm(t *testing.T) {
	get := func(path string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		routes(&serverState{}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		return recorder
	}

	t.Run("ohne Konfiguration", func(t *testing.T) {
		chdir(t, t.TempDir())

		recorder := get("/")
		if recorder.Code != http.StatusFound {
			t.Fatalf("Status = %d, erwartet %d", recorder.Code, http.StatusFound)
		}
		if location := recorder.Header().Get("Location"); location != "/setup" {
			t.Errorf("Location = %q, erwartet /setup", location)
		}
		if code := get("/gibtesnicht").Code; code != http.StatusNotFound {
			t.Errorf("unbekannter Pfad: Status = %d, erwartet %d", code, http.StatusNotFound)
		}
	})

	t.Run("mit Konfiguration", func(t *testing.T) {
		root := t.TempDir()
		if err := project.CreateConfig(root, "."); err != nil {
			t.Fatalf("Konfiguration anlegen: %v", err)
		}
		chdir(t, root)

		recorder := get("/")
		if recorder.Code != http.StatusOK {
			t.Fatalf("Status = %d, erwartet %d", recorder.Code, http.StatusOK)
		}
		body := recorder.Body.String()
		for _, want := range []string{
			`id="status-card"`,
			"Statusfeld (hier werden in Kürze die wichtigsten Werte angezeigt)",
			`<p id="status-message" class="message hidden"></p>`,
			`id="closed"`,
			`/static/service.js`,
			// Das Bild der Wissensablage: zuunterst, aufgeklappt, mit Verweis
			// in die Doku.
			`id="knowledge-picture-card"`,
			`<details id="knowledge-picture-fold" open>`,
			`href="/docs?file=knowledge-storage.md"`,
			`/static/docview.js`,
		} {
			if !strings.Contains(body, want) {
				t.Errorf("die Statusseite enthält %q nicht", want)
			}
		}
		if code := get("/gibtesnicht").Code; code != http.StatusNotFound {
			t.Errorf("unbekannter Pfad: Status = %d, erwartet %d", code, http.StatusNotFound)
		}
	})
}

// Die drei Workflow-Karten stehen als ein Fragment auf /workflows und auf der
// Statusseite, jede Karte genau einmal. Die Zahlen holt auf beiden Seiten
// workflows.js; die Einführung „Was Workflows sind" bleibt auf /workflows.
func TestWorkflowKartenAufBeidenSeiten(t *testing.T) {
	root := t.TempDir()
	if err := project.CreateConfig(root, "."); err != nil {
		t.Fatalf("Konfiguration anlegen: %v", err)
	}
	chdir(t, root)

	for path, einfuehrung := range map[string]bool{"/": false, "/workflows": true} {
		t.Run(path, func(t *testing.T) {
			status, body := getPage(t, path)
			if status != http.StatusOK {
				t.Fatalf("Status = %d, erwartet %d", status, http.StatusOK)
			}
			for _, id := range []string{`id="tasks-card"`, `id="reviews-card"`, `id="todos-card"`} {
				if count := strings.Count(body, id); count != 1 {
					t.Errorf("%s kommt %d-mal vor, erwartet genau 1", id, count)
				}
			}
			if count := strings.Count(body, `/static/workflows.js"`); count != 1 {
				t.Errorf("workflows.js wird %d-mal geladen, erwartet genau 1", count)
			}
			if got := strings.Contains(body, `id="workflows-intro-card"`); got != einfuehrung {
				t.Errorf("Einführungskarte vorhanden = %v, erwartet %v", got, einfuehrung)
			}
		})
	}
}

// Die Knöpfe für Update und Dienst stehen im gemeinsamen Kopf, aber nur dort,
// wo service.js sie bedient: auf der Statusseite und auf /setup, solange keine
// Projektkonfiguration besteht. Auf jeder anderen Seite wären sie tot. Die
// Sperrfläche tragen Statusseite und /setup immer — sie ist der Weg beider
// Seiten beim Serververlust.
func TestKnoepfeImKopf(t *testing.T) {
	type fall struct {
		path    string
		knoepfe bool
		sperre  bool
	}
	pruefe := func(t *testing.T, faelle []fall) {
		for _, test := range faelle {
			status, body := getPage(t, test.path)
			if status != http.StatusOK {
				t.Fatalf("%s: Status = %d, erwartet %d", test.path, status, http.StatusOK)
			}
			for _, id := range []string{`id="shutdown"`, `id="update"`} {
				if got := strings.Contains(body, id); got != test.knoepfe {
					t.Errorf("%s: %s vorhanden = %v, erwartet %v", test.path, id, got, test.knoepfe)
				}
			}
			if got := strings.Contains(body, `id="closed"`); got != test.sperre {
				t.Errorf("%s: Sperrfläche vorhanden = %v, erwartet %v", test.path, got, test.sperre)
			}
		}
	}

	t.Run("ohne Konfiguration", func(t *testing.T) {
		chdir(t, t.TempDir())
		pruefe(t, []fall{
			{path: "/setup", knoepfe: true, sperre: true},
			{path: "/workflows"},
			{path: "/docs"},
		})
	})

	t.Run("mit Konfiguration", func(t *testing.T) {
		root := t.TempDir()
		if err := project.CreateConfig(root, "."); err != nil {
			t.Fatalf("Konfiguration anlegen: %v", err)
		}
		chdir(t, root)
		pruefe(t, []fall{
			{path: "/", knoepfe: true, sperre: true},
			{path: "/setup", sperre: true},
			{path: "/workflows"},
			{path: "/mcp"},
		})
	})
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
