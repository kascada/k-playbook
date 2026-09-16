package webui

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
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
	return serveChatState(t, &serverState{}, method, path, body)
}

// serveChatState bedient eine Anfrage gegen einen Server mit gegebenem
// Zustand. serveChat baut für jede Anfrage einen frischen; die Ausgänge der
// abgekoppelten Command-Aufrufe leben aber im Zustand und müssen zwischen zwei
// Anfragen stehen bleiben.
func serveChatState(t *testing.T, state *serverState, method string, path string, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	recorder := httptest.NewRecorder()
	routes(state).ServeHTTP(recorder, httptest.NewRequest(method, path, reader))
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
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"ok":true`) {
		t.Fatalf("Antwort = %d %s, erwartet 200 mit \"ok\":true", recorder.Code, recorder.Body.String())
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

// Rückfragen: die Kennung landet im Pfad, die Antwortliste im Rumpf, das
// Verzeichnis in der Abfrage — und das nackte true, mit dem die alte API
// antwortet, kommt unverändert bei der Seite an.
func TestChatRueckfragenReichenWeiter(t *testing.T) {
	directory := chatProject(t)
	var gotPath, gotDirectory, gotMethod string
	var gotBody struct {
		Answers [][]string `json:"answers"`
	}
	fakeOpenCode(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		gotDirectory = r.URL.Query().Get("directory")
		gotBody.Answers = nil
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/question" {
			fmt.Fprint(w, `[{"id":"que_abc","sessionID":"ses_kind","questions":[]}]`)
			return
		}
		// Reply und Reject antworten mit einem nackten true, nicht mit einem
		// Objekt.
		fmt.Fprint(w, `true`)
	})

	t.Run("Liste", func(t *testing.T) {
		recorder := serveChat(t, http.MethodGet, "/api/chat/questions", "")
		if recorder.Code != http.StatusOK || gotPath != "/question" || gotDirectory != directory {
			t.Fatalf("Status = %d, weitergeleitet an %q mit directory=%q", recorder.Code, gotPath, gotDirectory)
		}
		if !strings.Contains(recorder.Body.String(), `"que_abc"`) {
			t.Errorf("Antwort nicht durchgereicht: %s", recorder.Body.String())
		}
	})

	t.Run("Antworten", func(t *testing.T) {
		body := `{"answers":[["Ja","Vielleicht"],[],["eigener Text"]]}`
		recorder := serveChat(t, http.MethodPost, "/api/chat/questions/que_abc123/reply", body)
		if recorder.Code != http.StatusOK {
			t.Fatalf("Status = %d, erwartet 200: %s", recorder.Code, recorder.Body.String())
		}
		if strings.TrimSpace(recorder.Body.String()) != "true" {
			t.Errorf("Antwort = %q, erwartet das nackte true von OpenCode", recorder.Body.String())
		}
		if gotPath != "/question/que_abc123/reply" || gotMethod != http.MethodPost || gotDirectory != directory {
			t.Errorf("weitergeleitet als %s %q mit directory=%q", gotMethod, gotPath, gotDirectory)
		}
		want := [][]string{{"Ja", "Vielleicht"}, {}, {"eigener Text"}}
		if fmt.Sprint(gotBody.Answers) != fmt.Sprint(want) {
			t.Errorf("answers = %v, erwartet %v", gotBody.Answers, want)
		}
	})

	t.Run("null wird zur leeren Liste", func(t *testing.T) {
		recorder := serveChat(t, http.MethodPost, "/api/chat/questions/que_abc123/reply", `{"answers":[null,["Ja"]]}`)
		if recorder.Code != http.StatusOK {
			t.Fatalf("Status = %d, erwartet 200: %s", recorder.Code, recorder.Body.String())
		}
		want := [][]string{{}, {"Ja"}}
		if fmt.Sprint(gotBody.Answers) != fmt.Sprint(want) {
			t.Errorf("answers = %v, erwartet %v", gotBody.Answers, want)
		}
	})

	t.Run("Ablehnen", func(t *testing.T) {
		recorder := serveChat(t, http.MethodPost, "/api/chat/questions/que_abc123/reject", "")
		if recorder.Code != http.StatusOK || strings.TrimSpace(recorder.Body.String()) != "true" {
			t.Fatalf("Antwort = %d %q, erwartet 200 true", recorder.Code, recorder.Body.String())
		}
		if gotPath != "/question/que_abc123/reject" || gotMethod != http.MethodPost || gotDirectory != directory {
			t.Errorf("weitergeleitet als %s %q mit directory=%q", gotMethod, gotPath, gotDirectory)
		}
	})

	t.Run("Kind-Sitzungen", func(t *testing.T) {
		recorder := serveChat(t, http.MethodGet, "/api/chat/sessions/ses_abc123/children", "")
		if recorder.Code != http.StatusOK || gotPath != "/session/ses_abc123/children" || gotDirectory != directory {
			t.Errorf("Status = %d, weitergeleitet an %q mit directory=%q", recorder.Code, gotPath, gotDirectory)
		}
	})
}

// Eine ungültige Kennung und eine Antwort, die keine Liste von Listen von
// Zeichenketten ist, erreichen OpenCode nie.
func TestChatWeistUngueltigeRueckfragenAb(t *testing.T) {
	chatProject(t)
	called := false
	fakeOpenCode(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		fmt.Fprint(w, `true`)
	})

	tests := []struct {
		name, path, body string
	}{
		{"Kennung ohne Präfix", "/api/chat/questions/abc/reply", `{"answers":[["Ja"]]}`},
		{"Kennung mit Schrägstrich beim Ablehnen", "/api/chat/questions/que_a%2Fb/reject", ""},
		{"answers fehlt", "/api/chat/questions/que_abc/reply", `{}`},
		{"answers ist null", "/api/chat/questions/que_abc/reply", `{"answers":null}`},
		{"answers ist eine flache Liste", "/api/chat/questions/que_abc/reply", `{"answers":["Ja"]}`},
		{"answers trägt eine Zahl", "/api/chat/questions/que_abc/reply", `{"answers":[[1]]}`},
		{"answers ist ein Objekt", "/api/chat/questions/que_abc/reply", `{"answers":{"0":["Ja"]}}`},
		{"kein JSON", "/api/chat/questions/que_abc/reply", `Ja`},
		{"Kind-Sitzungen ungültiger Kennung", "/api/chat/sessions/abc/children", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			called = false
			method := http.MethodPost
			if strings.HasSuffix(test.path, "/children") {
				method = http.MethodGet
			}
			recorder := serveChat(t, method, test.path, test.body)
			if recorder.Code != http.StatusBadRequest {
				t.Errorf("Status = %d, erwartet 400: %s", recorder.Code, recorder.Body.String())
			}
			if called {
				t.Error("die Anfrage hat OpenCode erreicht")
			}
		})
	}
}

// --- Commands ---------------------------------------------------------------

// chatCommandList ist die Antwort, mit der der Stellvertreter GET /command
// bedient: ein gewöhnlicher Command, ein Subtask und ein interner Baustein.
const chatCommandList = `[
	{"name":"k-todo","description":"Todos","source":"command","template":"","hints":["eintrag"]},
	{"name":"review","description":"Review","source":"command","template":"","hints":[],"subtask":true},
	{"name":"_docs/code","description":"Baustein","source":"command","template":"","hints":[]}
]`

// awaitCommandState holt den Ausgang, bis er nicht mehr läuft — wie es die
// Seite tut.
func awaitCommandState(t *testing.T, state *serverState, path string) chatCommandStateResponse {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		recorder := serveChatState(t, state, http.MethodGet, path, "")
		if recorder.Code != http.StatusOK {
			t.Fatalf("command-state: Status = %d, erwartet 200: %s", recorder.Code, recorder.Body.String())
		}
		var response chatCommandStateResponse
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("Antwort nicht lesbar: %v", err)
		}
		if response.State != "running" {
			return response
		}
		if time.Now().After(deadline) {
			t.Fatal("der abgekoppelte Aufruf bleibt auf running stehen")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// Ein bekannter Command geht mit seinen Argumenten an OpenCode, und die
// Antwort an die Seite wartet nicht auf den Lauf: der Stellvertreter hält den
// Aufruf fest, die 202 ist trotzdem sofort da.
func TestChatCommandLaeuftAbgekoppelt(t *testing.T) {
	directory := chatProject(t)
	release := make(chan struct{})
	reached := make(chan struct{})
	var gotPath, gotDirectory string
	var gotBody struct {
		Command   string `json:"command"`
		Arguments string `json:"arguments"`
	}
	fakeOpenCode(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/command" {
			fmt.Fprint(w, chatCommandList)
			return
		}
		gotPath = r.URL.Path
		gotDirectory = r.URL.Query().Get("directory")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		close(reached)
		<-release
		fmt.Fprint(w, `{"info":{"id":"msg_abc"},"parts":[]}`)
	})
	defer close(release)

	state := &serverState{streams: make(chan struct{})}
	recorder := serveChatState(t, state, http.MethodPost, "/api/chat/sessions/ses_abc123/command", `{"command":"k-todo","arguments":"neuer Eintrag"}`)
	if recorder.Code != http.StatusAccepted || !strings.Contains(recorder.Body.String(), `"ok":true`) {
		t.Fatalf("Antwort = %d %s, erwartet 202 mit \"ok\":true", recorder.Code, recorder.Body.String())
	}

	select {
	case <-reached:
	case <-time.After(5 * time.Second):
		t.Fatal("der Command hat OpenCode nicht erreicht")
	}
	if gotPath != "/session/ses_abc123/command" || gotDirectory != directory {
		t.Errorf("weitergeleitet an %q mit directory=%q, erwartet /session/ses_abc123/command mit %q", gotPath, gotDirectory, directory)
	}
	if gotBody.Command != "k-todo" || gotBody.Arguments != "neuer Eintrag" {
		t.Errorf("Rumpf = %+v, erwartet k-todo mit „neuer Eintrag“", gotBody)
	}
	// Solange der Stellvertreter festhält, läuft der Aufruf noch.
	recorder = serveChatState(t, state, http.MethodGet, "/api/chat/sessions/ses_abc123/command-state", "")
	if !strings.Contains(recorder.Body.String(), `"running"`) {
		t.Errorf("command-state = %s, erwartet running", recorder.Body.String())
	}
}

// Ein Name, den die gefilterte Liste nicht kennt, wird abgewiesen, ohne dass
// der Command-Endpunkt von OpenCode ihn zu sehen bekommt. Das gilt auch für
// einen internen Baustein mit führendem Unterstrich.
func TestChatCommandWeistUnbekanntenNamenAb(t *testing.T) {
	chatProject(t)
	sent := false
	fakeOpenCode(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/command" {
			fmt.Fprint(w, chatCommandList)
			return
		}
		sent = true
		fmt.Fprint(w, `{}`)
	})

	tests := []struct{ name, body string }{
		{"unbekannter Name", `{"command":"gibt-es-nicht","arguments":""}`},
		{"interner Baustein", `{"command":"_docs/code","arguments":""}`},
		{"kein Name", `{"command":"  ","arguments":""}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sent = false
			recorder := serveChatState(t, &serverState{}, http.MethodPost, "/api/chat/sessions/ses_abc123/command", test.body)
			if recorder.Code != http.StatusBadRequest {
				t.Errorf("Status = %d, erwartet 400: %s", recorder.Code, recorder.Body.String())
			}
			if sent {
				t.Error("der Command hat den Endpunkt von OpenCode erreicht")
			}
		})
	}
}

// Die Liste, die die Seite bekommt, lässt die internen Bausteine weg und trägt
// Beschreibung, Quelle, Hinweise und subtask mit.
func TestChatCommandsLassenBausteineWeg(t *testing.T) {
	directory := chatProject(t)
	var gotPath, gotDirectory string
	fakeOpenCode(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotDirectory = r.URL.Path, r.URL.Query().Get("directory")
		fmt.Fprint(w, chatCommandList)
	})

	recorder := serveChat(t, http.MethodGet, "/api/chat/commands", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("Status = %d, erwartet 200: %s", recorder.Code, recorder.Body.String())
	}
	if gotPath != "/command" || gotDirectory != directory {
		t.Errorf("weitergeleitet an %q mit directory=%q", gotPath, gotDirectory)
	}
	var commands []chatCommand
	if err := json.Unmarshal(recorder.Body.Bytes(), &commands); err != nil {
		t.Fatalf("Antwort nicht lesbar: %v", err)
	}
	if len(commands) != 2 {
		t.Fatalf("Liste = %+v, erwartet zwei Einträge ohne den Baustein", commands)
	}
	if commands[0].Name != "k-todo" || commands[0].Description != "Todos" || commands[0].Source != "command" {
		t.Errorf("erster Eintrag = %+v", commands[0])
	}
	if len(commands[0].Hints) != 1 || commands[0].Hints[0] != "eintrag" {
		t.Errorf("hints = %v, erwartet [eintrag]", commands[0].Hints)
	}
	if commands[1].Name != "review" || !commands[1].Subtask {
		t.Errorf("zweiter Eintrag = %+v, erwartet review mit subtask", commands[1])
	}
	for _, command := range commands {
		if strings.HasPrefix(command.Name, "_") {
			t.Errorf("interner Baustein in der Liste: %q", command.Name)
		}
	}
}

// Scheitert der abgekoppelte Aufruf, entsteht kein Ereignis — der Ausgang
// kommt deshalb über command-state, samt Meldung. Er geht genau einmal hinaus.
func TestChatCommandMeldetSpaetenFehler(t *testing.T) {
	chatProject(t)
	fakeOpenCode(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/command" {
			fmt.Fprint(w, chatCommandList)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"message":"Modell nicht erreichbar"}`)
	})

	state := &serverState{streams: make(chan struct{})}
	recorder := serveChatState(t, state, http.MethodPost, "/api/chat/sessions/ses_abc123/command", `{"command":"k-todo","arguments":""}`)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("Status = %d, erwartet 202: %s", recorder.Code, recorder.Body.String())
	}

	response := awaitCommandState(t, state, "/api/chat/sessions/ses_abc123/command-state")
	if response.State != "failed" {
		t.Fatalf("Zustand = %q, erwartet failed", response.State)
	}
	if !strings.Contains(response.Message, "Modell nicht erreichbar") {
		t.Errorf("Meldung = %q, erwartet den Fehlerrumpf von OpenCode", response.Message)
	}

	recorder = serveChatState(t, state, http.MethodGet, "/api/chat/sessions/ses_abc123/command-state", "")
	var second chatCommandStateResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &second); err != nil {
		t.Fatalf("Antwort nicht lesbar: %v", err)
	}
	if second.State == "failed" || second.Message != "" {
		t.Errorf("zweite Antwort = %+v, ein gelesener Fehler darf kein zweites Mal kommen", second)
	}
}

// Eine Sitzung ohne Eintrag liefert unknown — die Seite lässt den Laufzustand
// dann unberührt. done wäre dort irreführend.
func TestChatCommandStateOhneEintrag(t *testing.T) {
	chatProject(t)
	fakeOpenCode(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("command-state hat OpenCode erreicht: %s", r.URL.Path)
	})

	recorder := serveChat(t, http.MethodGet, "/api/chat/sessions/ses_abc123/command-state", "")
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"unknown"`) {
		t.Fatalf("Antwort = %d %s, erwartet 200 mit unknown", recorder.Code, recorder.Body.String())
	}
}

// Die Ablage wird begrenzt, aber ein ungelesener Fehler wird dabei nie
// verdrängt: sonst antwortete command-state unknown und die Seite bliebe auf
// „Arbeitet" stehen — genau der Zustand, gegen den der Rückweg gebaut ist.
func TestChatCommandStateHaeltUngeleseneFehler(t *testing.T) {
	chatProject(t)
	fakeOpenCode(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("command-state hat OpenCode erreicht: %s", r.URL.Path)
	})

	state := &serverState{}
	state.noteCommandFinished("ses_fehler", "Modell nicht erreichbar")
	for i := 0; i < chatCommandStateLimit*2; i++ {
		state.noteCommandFinished(fmt.Sprintf("ses_fertig%d", i), "")
	}
	if len(state.commandRuns) > chatCommandStateLimit {
		t.Errorf("Ablage = %d Einträge, erwartet höchstens %d", len(state.commandRuns), chatCommandStateLimit)
	}

	recorder := serveChatState(t, state, http.MethodGet, "/api/chat/sessions/ses_fehler/command-state", "")
	var response chatCommandStateResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Antwort nicht lesbar: %v", err)
	}
	if response.State != "failed" || !strings.Contains(response.Message, "Modell nicht erreichbar") {
		t.Errorf("Antwort = %+v, erwartet den ungelesenen Fehler", response)
	}
}

// Der abgekoppelte Aufruf endet mit dem Server. Gemeint ist allein die
// wartende Anfrage an OpenCode; der Lauf dort endet nur über abort. Der
// Stellvertreter hält deshalb weiter fest — enden muss das Warten hier.
func TestChatCommandBrichtBeimBeendenAb(t *testing.T) {
	chatProject(t)
	opened := make(chan struct{})
	release := make(chan struct{})
	fakeOpenCode(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/command" {
			fmt.Fprint(w, chatCommandList)
			return
		}
		close(opened)
		<-release
	})
	defer close(release)

	state := &serverState{streams: make(chan struct{})}
	recorder := serveChatState(t, state, http.MethodPost, "/api/chat/sessions/ses_abc123/command", `{"command":"k-todo","arguments":""}`)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("Status = %d, erwartet 202: %s", recorder.Code, recorder.Body.String())
	}
	select {
	case <-opened:
	case <-time.After(5 * time.Second):
		t.Fatal("der Command hat OpenCode nicht erreicht")
	}

	state.closeStreams()
	response := awaitCommandState(t, state, "/api/chat/sessions/ses_abc123/command-state")
	if response.State != "failed" {
		t.Fatalf("Zustand = %q, erwartet failed — das Warten endet mit dem Server", response.State)
	}
}

// --- Bedienung: Agenten und die Kennung der eigenen Nachricht ----------------

// chatAgentList ist die Antwort, mit der der Stellvertreter GET /agent bedient:
// ein primärer Agent, einer mit mode „all", ein Subagent und ein versteckter.
const chatAgentList = `[
	{"name":"build","description":"Baut","mode":"primary"},
	{"name":"general","description":"Beides","mode":"all"},
	{"name":"researcher","description":"Sucht","mode":"subagent"},
	{"name":"intern","description":"Versteckt","mode":"primary","hidden":true}
]`

// Angeboten wird alles außer mode: "subagent" und hidden: true. Eine Prüfung
// auf mode == "primary" würfe den Agenten mit mode „all" fälschlich weg und
// böte den versteckten an.
func TestChatAgentenOhneSubagentenUndVersteckte(t *testing.T) {
	directory := chatProject(t)
	var gotPath, gotDirectory, gotUser, gotPassword string
	var gotAuth bool
	fakeOpenCode(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotDirectory = r.URL.Path, r.URL.Query().Get("directory")
		gotUser, gotPassword, gotAuth = r.BasicAuth()
		fmt.Fprint(w, chatAgentList)
	})
	t.Setenv("OPENCODE_SERVER_PASSWORD", "geheim")

	recorder := serveChat(t, http.MethodGet, "/api/chat/agents", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("Status = %d, erwartet 200: %s", recorder.Code, recorder.Body.String())
	}
	if gotPath != "/agent" || gotDirectory != directory {
		t.Errorf("weitergeleitet an %q mit directory=%q, erwartet /agent mit %q", gotPath, gotDirectory, directory)
	}
	if !gotAuth || gotUser != "opencode" || gotPassword != "geheim" {
		t.Errorf("Anmeldung = %v %q/%q, erwartet opencode/geheim", gotAuth, gotUser, gotPassword)
	}

	var agents []chatAgent
	if err := json.Unmarshal(recorder.Body.Bytes(), &agents); err != nil {
		t.Fatalf("Antwort nicht lesbar: %v", err)
	}
	if len(agents) != 2 {
		t.Fatalf("Liste = %+v, erwartet genau build und general", agents)
	}
	if agents[0].Name != "build" || agents[0].Mode != "primary" || agents[0].Description != "Baut" {
		t.Errorf("erster Eintrag = %+v", agents[0])
	}
	if agents[1].Name != "general" || agents[1].Mode != "all" {
		t.Errorf("zweiter Eintrag = %+v, erwartet general mit mode all", agents[1])
	}
}

// Der gewählte Agent geht bei Text und bei Command mit.
func TestChatSendetGewaehltenAgentenMit(t *testing.T) {
	chatProject(t)
	reached := make(chan struct{})
	var gotAgent string
	fakeOpenCode(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/command" {
			fmt.Fprint(w, chatCommandList)
			return
		}
		var body struct {
			Agent string `json:"agent"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotAgent = body.Agent
		if strings.HasSuffix(r.URL.Path, "/command") {
			close(reached)
			fmt.Fprint(w, `{}`)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	serveChat(t, http.MethodPost, "/api/chat/sessions/ses_abc123/prompt", `{"text":"Hallo","agent":"plan"}`)
	if gotAgent != "plan" {
		t.Errorf("agent bei prompt = %q, erwartet plan", gotAgent)
	}

	gotAgent = ""
	state := &serverState{streams: make(chan struct{})}
	serveChatState(t, state, http.MethodPost, "/api/chat/sessions/ses_abc123/command", `{"command":"k-todo","arguments":"","agent":"build"}`)
	select {
	case <-reached:
	case <-time.After(5 * time.Second):
		t.Fatal("der Command hat OpenCode nicht erreicht")
	}
	if gotAgent != "build" {
		t.Errorf("agent bei command = %q, erwartet build", gotAgent)
	}
}

// promptMessageID schickt eine Nachricht und liefert die Kennung aus der
// Antwort und die, die bei OpenCode ankam.
func promptMessageID(t *testing.T, sent *string) string {
	t.Helper()
	recorder := serveChat(t, http.MethodPost, "/api/chat/sessions/ses_abc123/prompt", `{"text":"Hallo"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("Status = %d, erwartet 200: %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		MessageID string `json:"messageID"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Antwort nicht lesbar: %v", err)
	}
	if response.MessageID == "" {
		t.Fatalf("keine Kennung in der Antwort: %s", recorder.Body.String())
	}
	if sent != nil && *sent != response.MessageID {
		t.Errorf("mitgeschickt wurde %q, zurückgegeben %q", *sent, response.MessageID)
	}
	return response.MessageID
}

// prompt und command erzeugen die Kennung, schicken sie mit und geben sie
// zurück; zwei Aufrufe nacheinander liefern aufsteigende Kennungen.
func TestChatSendetUndMeldetMessageID(t *testing.T) {
	chatProject(t)
	reached := make(chan struct{})
	var sent string
	fakeOpenCode(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/command" {
			fmt.Fprint(w, chatCommandList)
			return
		}
		var body struct {
			MessageID string `json:"messageID"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		sent = body.MessageID
		if strings.HasSuffix(r.URL.Path, "/command") {
			close(reached)
			fmt.Fprint(w, `{}`)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	first := promptMessageID(t, &sent)
	second := promptMessageID(t, &sent)
	if !(first < second) {
		t.Errorf("Kennungen = %q und %q, erwartet aufsteigend", first, second)
	}

	sent = ""
	state := &serverState{streams: make(chan struct{})}
	recorder := serveChatState(t, state, http.MethodPost, "/api/chat/sessions/ses_abc123/command", `{"command":"k-todo","arguments":""}`)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("Status = %d, erwartet 202: %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		MessageID string `json:"messageID"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Antwort nicht lesbar: %v", err)
	}
	if response.MessageID == "" {
		t.Fatalf("keine Kennung in der Antwort: %s", recorder.Body.String())
	}
	select {
	case <-reached:
	case <-time.After(5 * time.Second):
		t.Fatal("der Command hat OpenCode nicht erreicht")
	}
	if sent != response.MessageID {
		t.Errorf("mitgeschickt wurde %q, zurückgegeben %q", sent, response.MessageID)
	}
	if !(second < response.MessageID) {
		t.Errorf("Kennungen = %q und %q, erwartet aufsteigend", second, response.MessageID)
	}
}

// chatObservedMessageID ist eine tatsächlich beobachtete Kennung von OpenCode,
// abgelesen aus ~/.local/share/opencode/log/opencode.log zur Logzeile
// 2026-07-06T09:43:51.392Z. Zwei eigene Kennungen allein belegen die Ordnung
// nicht: entscheidend ist die Ordnung gegen die Kennungen von OpenCode, weil
// renderLog() im Browser per reinem Zeichenvergleich sortiert. Der Beleg steht
// in k-playbook-local/material/befunde/opencode-im-browser-einbinden.md.
const chatObservedMessageID = "msg_f36d0094f001nAtF2QwVYJfSW7"

// Die erzeugte Kennung hat dieselbe Gestalt wie eine echte und ordnet sich beim
// Zeichenvergleich richtig zwischen fremde ein — davor erzeugte davor, danach
// erzeugte danach.
func TestMessageIDSortiertGegenEchteKennung(t *testing.T) {
	shape := regexp.MustCompile(`^msg_[0-9a-f]{12}[A-Za-z0-9]{14}$`)
	if !shape.MatchString(chatObservedMessageID) {
		t.Fatalf("die beobachtete Kennung %q passt nicht auf das Muster", chatObservedMessageID)
	}

	// Die 12 Hexziffern sind (Zeitstempel_ms << 12 | Zähler), auf 48 Bit
	// abgeschnitten; daraus ergibt sich die Millisekunde zurück.
	digits := chatObservedMessageID[len(messageIDPrefix) : len(messageIDPrefix)+messageIDTimeDigits]
	value, err := strconv.ParseUint(digits, 16, 64)
	if err != nil {
		t.Fatalf("Zeitanteil %q nicht lesbar: %v", digits, err)
	}
	ms := int64(value >> messageIDCounterBits)

	before := messageIDAt(ms - 1)
	after := messageIDAt(ms + 1)
	for _, id := range []string{before, after, newMessageID()} {
		if len(id) != len(chatObservedMessageID) {
			t.Errorf("Kennung %q ist %d Zeichen lang, erwartet %d", id, len(id), len(chatObservedMessageID))
		}
		if !shape.MatchString(id) {
			t.Errorf("Kennung %q passt nicht auf das Muster einer echten", id)
		}
	}
	if !(before < chatObservedMessageID) {
		t.Errorf("%q sortiert nicht vor der beobachteten Kennung %q", before, chatObservedMessageID)
	}
	if !(chatObservedMessageID < after) {
		t.Errorf("%q sortiert nicht hinter der beobachteten Kennung %q", after, chatObservedMessageID)
	}
	if !(before < after) {
		t.Errorf("%q sortiert nicht vor %q", before, after)
	}
	// Innerhalb derselben Millisekunde zählt der Zähler weiter; ohne ihn
	// bekämen zwei Nachrichten denselben Zeitanteil und sortierten zufällig.
	sameFirst, sameSecond := messageIDAt(ms), messageIDAt(ms)
	if !(sameFirst < sameSecond) {
		t.Errorf("in derselben Millisekunde erzeugt: %q und %q, erwartet aufsteigend", sameFirst, sameSecond)
	}
	if !(chatObservedMessageID < sameSecond) {
		t.Errorf("%q sortiert nicht hinter der beobachteten Kennung %q — der Zähler darf den Zeitanteil nicht verschieben", sameSecond, chatObservedMessageID)
	}
}
