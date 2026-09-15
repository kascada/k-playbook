package webui

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// chatProject legt ein Projekt an, wechselt hinein und liefert das
// Verzeichnis, das die Weiterleitung mitschicken muss.
func chatProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := project.CreateConfig(root, "."); err != nil {
		t.Fatalf("Konfiguration anlegen: %v", err)
	}
	chdir(t, root)
	return project.Detect().ProjectDir
}

// fakeOpenCode startet einen Stellvertreter des OpenCode-Dienstes und richtet
// die Oberfläche auf ihn aus — ohne Passwort, solange ein Test keines setzt.
func fakeOpenCode(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	t.Setenv("OPENCODE_SERVER_URL", server.URL)
	t.Setenv("OPENCODE_SERVER_USERNAME", "")
	t.Setenv("OPENCODE_SERVER_PASSWORD", "")
	// Ob der Test selbst in einem Container läuft, darf das Ergebnis nicht
	// ändern; die Container-Fälle prüft chat_guard_test.go ausdrücklich.
	setContainer(t, false)
	return server
}

func serveChat(t *testing.T, method string, path string, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	recorder := httptest.NewRecorder()
	routes(&serverState{}).ServeHTTP(recorder, httptest.NewRequest(method, path, reader))
	return recorder
}

// Jede Weiterleitung trägt das Projektverzeichnis des Servers, und die
// Anmeldung geht nur mit, wenn ein Passwort gesetzt ist.
func TestChatReichtVerzeichnisUndAnmeldungWeiter(t *testing.T) {
	directory := chatProject(t)
	var gotPath, gotDirectory, gotRoots, gotUser, gotPassword string
	var gotAuth bool
	fakeOpenCode(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotDirectory = r.URL.Query().Get("directory")
		gotRoots = r.URL.Query().Get("roots")
		gotUser, gotPassword, gotAuth = r.BasicAuth()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[{"id":"ses_abc","title":"Test"}]`)
	})

	recorder := serveChat(t, http.MethodGet, "/api/chat/sessions", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("Status = %d, erwartet 200: %s", recorder.Code, recorder.Body.String())
	}
	if gotPath != "/session" || gotDirectory != directory || gotRoots != "true" {
		t.Errorf("weitergeleitet an %s mit directory=%q roots=%q, erwartet /session mit %q und true", gotPath, gotDirectory, gotRoots, directory)
	}
	if gotAuth {
		t.Error("ohne Passwort darf keine Anmeldung mitgehen")
	}
	if !strings.Contains(recorder.Body.String(), `"ses_abc"`) {
		t.Errorf("Antwort nicht durchgereicht: %s", recorder.Body.String())
	}

	t.Setenv("OPENCODE_SERVER_PASSWORD", "geheim")
	serveChat(t, http.MethodGet, "/api/chat/sessions", "")
	if !gotAuth || gotUser != "opencode" || gotPassword != "geheim" {
		t.Errorf("Anmeldung = %v %q/%q, erwartet opencode/geheim", gotAuth, gotUser, gotPassword)
	}
}

// Der Text wird zu einem Textteil für prompt_async, und das leere 204 von
// OpenCode kommt als JSON bei der Seite an.
func TestChatPromptWirdZuTextteil(t *testing.T) {
	chatProject(t)
	var gotPath string
	var gotBody struct {
		Parts []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"parts"`
	}
	fakeOpenCode(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("Rumpf nicht lesbar: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	recorder := serveChat(t, http.MethodPost, "/api/chat/sessions/ses_abc123/prompt", `{"text":"Hallo"}`)
	if recorder.Code != http.StatusOK || strings.TrimSpace(recorder.Body.String()) != `{"ok":true}` {
		t.Fatalf("Antwort = %d %s, erwartet 200 {\"ok\":true}", recorder.Code, recorder.Body.String())
	}
	if gotPath != "/session/ses_abc123/prompt_async" {
		t.Errorf("weitergeleitet an %s", gotPath)
	}
	if len(gotBody.Parts) != 1 || gotBody.Parts[0].Type != "text" || gotBody.Parts[0].Text != "Hallo" {
		t.Errorf("Teile = %+v, erwartet einen Textteil „Hallo“", gotBody.Parts)
	}
}

// Kennungen landen im Pfad der Weiterleitung. Was nicht wie eine
// OpenCode-Kennung aussieht, kommt dort nie an; leere Nachrichten und
// unbekannte Freigabe-Antworten ebenso wenig.
func TestChatWeistUngueltigeAnfragenAb(t *testing.T) {
	chatProject(t)
	called := false
	fakeOpenCode(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})

	tests := []struct {
		name, path, body string
	}{
		{"Kennung ohne Präfix", "/api/chat/sessions/abc/prompt", `{"text":"Hallo"}`},
		{"Kennung mit Punkt", "/api/chat/sessions/ses_a.b/abort", ""},
		{"leere Nachricht", "/api/chat/sessions/ses_abc/prompt", `{"text":"  "}`},
		{"unbekannte Antwort", "/api/chat/permissions/per_abc/reply", `{"reply":"maybe"}`},
		{"kein JSON", "/api/chat/sessions/ses_abc/prompt", `Hallo`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			called = false
			recorder := serveChat(t, http.MethodPost, test.path, test.body)
			if recorder.Code != http.StatusBadRequest {
				t.Errorf("Status = %d, erwartet 400", recorder.Code)
			}
			if called {
				t.Error("die Anfrage hat OpenCode erreicht")
			}
		})
	}
}

// Eine abgewiesene Anmeldung wird nicht als 401 durchgereicht — der Browser
// fragte sonst selbst nach einem Passwort —, sondern erklärt.
func TestChatErklaertAbgewieseneAnmeldung(t *testing.T) {
	chatProject(t)
	fakeOpenCode(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	recorder := serveChat(t, http.MethodGet, "/api/chat/sessions", "")
	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("Status = %d, erwartet 502", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "OPENCODE_SERVER_PASSWORD") {
		t.Errorf("die Meldung nennt die Variable nicht: %s", recorder.Body.String())
	}
}

func TestChatStatus(t *testing.T) {
	chatProject(t)

	t.Run("Dienst bereit", func(t *testing.T) {
		fakeOpenCode(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/global/health" {
				t.Errorf("Lebenszeichen an %s", r.URL.Path)
			}
			fmt.Fprint(w, `{"healthy":true,"version":"1.18.30"}`)
		})
		var status chatStatusResponse
		recorder := serveChat(t, http.MethodGet, "/api/chat/status", "")
		if err := json.Unmarshal(recorder.Body.Bytes(), &status); err != nil {
			t.Fatalf("Antwort nicht lesbar: %v", err)
		}
		if !status.Installed || !status.Available || status.Version != "1.18.30" || status.Message != "" {
			t.Errorf("Status = %+v, erwartet bereit mit Version 1.18.30", status)
		}
	})

	t.Run("Dienst weg", func(t *testing.T) {
		server := fakeOpenCode(t, func(w http.ResponseWriter, r *http.Request) {})
		server.Close()
		var status chatStatusResponse
		recorder := serveChat(t, http.MethodGet, "/api/chat/status", "")
		if err := json.Unmarshal(recorder.Body.Bytes(), &status); err != nil {
			t.Fatalf("Antwort nicht lesbar: %v", err)
		}
		if status.Available || status.Message == "" {
			t.Errorf("Status = %+v, erwartet nicht erreichbar mit Meldung", status)
		}
	})
}

// Ohne Projekt gibt es kein Verzeichnis und damit keine Weiterleitung.
func TestChatOhneProjekt(t *testing.T) {
	chdir(t, t.TempDir())
	called := false
	fakeOpenCode(t, func(w http.ResponseWriter, r *http.Request) { called = true })

	recorder := serveChat(t, http.MethodGet, "/api/chat/sessions", "")
	if recorder.Code != http.StatusConflict {
		t.Errorf("Status = %d, erwartet 409", recorder.Code)
	}
	if called {
		t.Error("ohne Projekt hat eine Anfrage OpenCode erreicht")
	}
}

// Der Ereignisstrom geht als text/event-stream durch, mit dem Verzeichnis.
func TestChatEreignisstromReichtDurch(t *testing.T) {
	directory := chatProject(t)
	var gotDirectory string
	fakeOpenCode(t, func(w http.ResponseWriter, r *http.Request) {
		gotDirectory = r.URL.Query().Get("directory")
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"type\":\"server.connected\",\"properties\":{}}\n\n")
	})

	recorder := serveChat(t, http.MethodGet, "/api/chat/events", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("Status = %d, erwartet 200: %s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Errorf("Content-Type = %q", got)
	}
	if !strings.Contains(recorder.Body.String(), `"server.connected"`) {
		t.Errorf("Ereignis nicht durchgereicht: %q", recorder.Body.String())
	}
	if gotDirectory != directory {
		t.Errorf("directory = %q, erwartet %q", gotDirectory, directory)
	}
}

// Ein offener Strom endet, sobald der Server sich beendet — sonst hielte er
// Shutdown bis zur Frist auf.
func TestChatEreignisstromEndetBeimBeenden(t *testing.T) {
	chatProject(t)
	opened := make(chan struct{})
	fakeOpenCode(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {}\n\n")
		w.(http.Flusher).Flush()
		close(opened)
		<-r.Context().Done()
	})

	state := &serverState{streams: make(chan struct{})}
	finished := make(chan struct{})
	go func() {
		recorder := httptest.NewRecorder()
		routes(state).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/chat/events", nil))
		close(finished)
	}()

	select {
	case <-opened:
	case <-time.After(5 * time.Second):
		t.Fatal("der Strom wurde nicht geöffnet")
	}
	state.closeStreams()
	state.closeStreams()
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("der Strom endet nicht, nachdem der Server sich beendet")
	}
}

// Eine einzelne Sitzung wird mit ihrer Kennung weitergeleitet; die Seite dazu
// gibt es nur für Kennungen, die wie eine von OpenCode aussehen.
func TestChatEinzelneSitzung(t *testing.T) {
	chatProject(t)
	var gotPath string
	fakeOpenCode(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		fmt.Fprint(w, `{"id":"ses_abc123","title":"Test"}`)
	})

	if recorder := serveChat(t, http.MethodGet, "/api/chat/sessions/ses_abc123", ""); recorder.Code != http.StatusOK || gotPath != "/session/ses_abc123" {
		t.Errorf("Status = %d, weitergeleitet an %q", recorder.Code, gotPath)
	}
	if status, _ := getPage(t, "/chat/ses_abc123"); status != http.StatusOK {
		t.Errorf("Seite der Sitzung: Status = %d, erwartet 200", status)
	}
	if status, _ := getPage(t, "/chat/..%2Fdocs"); status != http.StatusNotFound {
		t.Errorf("Seite mit ungültiger Kennung: Status = %d, erwartet 404", status)
	}
}
