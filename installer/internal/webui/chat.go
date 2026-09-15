package webui

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// Der Chat der Oberfläche spricht kein Modell selbst an, sondern den
// OpenCode-Dienst: Sitzungen, Agenten, Tools und MCP liegen dort, die
// Oberfläche zeigt an und reicht weiter. Der Browser kennt nur /api/chat/*;
// welche OpenCode-Endpunkte dahinter stehen, weiß allein diese Datei.
//
// Genutzt wird die alte, unpräfixierte API (/session, /event). Die neue unter
// /api/ gilt bei OpenCode 1.18 als experimentell, ihre Daten als verwerfbar,
// und sie sieht die Sitzungen der alten nicht. Ein späterer Umstieg betrifft
// nur diese Datei; die Kriterien stehen in installer/docs/architecture.md
// unter „Chat in der Oberfläche".
//
// Adresse und Anmeldung kommen aus denselben Variablen, die auch
// `opencode attach` liest. Das Passwort bleibt damit im Server und erreicht
// den Browser nie. Im Container gilt zusätzlich containerGuard.

const (
	openCodeDefaultURL      = "http://127.0.0.1:4096"
	openCodeDefaultUsername = "opencode"
	// openCodeRequestTimeout begrenzt jede gewöhnliche Weiterleitung. Der
	// Ereignisstrom hat keine Frist: er lebt, solange die Seite offen ist.
	openCodeRequestTimeout = 30 * time.Second
	openCodeHealthTimeout  = 3 * time.Second
	// chatBodyLimit begrenzt, was der Browser an einen Chat-Endpunkt schickt.
	chatBodyLimit = 1 << 20
)

// openCodeID lässt nur Kennungen durch, wie OpenCode sie vergibt (ses_…,
// per_…). Sie werden in den Pfad der Weiterleitung eingesetzt und dürfen ihn
// deshalb nicht verlassen.
var openCodeID = regexp.MustCompile(`^[a-z]{3}_[A-Za-z0-9]{1,64}$`)

// openCodeClient hat keine Gesamtfrist, weil er auch den Ereignisstrom trägt.
// Fristen setzt jede Anfrage über ihren Kontext.
var openCodeClient = &http.Client{}

// chatError ist die Antwort, wenn die Weiterleitung selbst scheitert oder
// verweigert wird. Fehler, die OpenCode meldet, reicht der Server unverändert
// durch.
type chatError struct {
	Message string `json:"message"`
}

type openCodeTarget struct {
	baseURL  string
	username string
	password string
}

func openCodeFromEnv() openCodeTarget {
	target := openCodeTarget{
		baseURL:  strings.TrimRight(os.Getenv("OPENCODE_SERVER_URL"), "/"),
		username: os.Getenv("OPENCODE_SERVER_USERNAME"),
		password: os.Getenv("OPENCODE_SERVER_PASSWORD"),
	}
	if target.baseURL == "" {
		target.baseURL = openCodeDefaultURL
	}
	if target.username == "" {
		target.username = openCodeDefaultUsername
	}
	return target
}

func (target openCodeTarget) newRequest(ctx context.Context, method string, path string, query url.Values, body io.Reader) (*http.Request, error) {
	address := target.baseURL + path
	if len(query) > 0 {
		address += "?" + query.Encode()
	}
	request, err := http.NewRequestWithContext(ctx, method, address, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	// Ohne Passwort keine Anmeldung: der Dienst verlangt dann auch keine.
	if target.password != "" {
		request.SetBasicAuth(target.username, target.password)
	}
	return request, nil
}

// authMessage erklärt eine abgewiesene Anmeldung. Der Browser bekommt dafür
// 502 statt 401 — sonst fragte er selbst nach einem Passwort, das er gar nicht
// weitergeben soll.
func (target openCodeTarget) authMessage() string {
	if target.password == "" {
		return "Der OpenCode-Dienst verlangt eine Anmeldung, OPENCODE_SERVER_PASSWORD ist für die Oberfläche aber nicht gesetzt."
	}
	return "Der OpenCode-Dienst weist die Anmeldung ab: OPENCODE_SERVER_PASSWORD oder OPENCODE_SERVER_USERNAME stimmt nicht."
}

type chatStatusResponse struct {
	// Installed meldet, ob es ein Projekt gibt; ohne Verzeichnis kein Chat.
	Installed bool `json:"installed"`
	// Available meldet, ob der Dienst antwortet, bereit ist und benutzt werden
	// darf.
	Available bool `json:"available"`
	// Blocked meldet, dass der Dienst hier nicht benutzt werden darf — im
	// Container einer, der nicht zum Container gehört.
	Blocked bool `json:"blocked"`
	// Container meldet, dass der Server in einem Container läuft.
	Container bool   `json:"container"`
	URL       string `json:"url"`
	// Auth sagt, ob die Oberfläche ein Passwort mitschickt — nicht, ob es stimmt.
	Auth      bool   `json:"auth"`
	Version   string `json:"version,omitempty"`
	Directory string `json:"directory,omitempty"`
	Message   string `json:"message,omitempty"`
	// Hint sagt, wie ein fehlender Dienst entsteht. Nur, wenn keiner benutzbar
	// ist.
	Hint *chatHint `json:"hint,omitempty"`
}

// chatStatusHandler sagt der Seite, ob sie überhaupt einen Chat anbieten kann,
// und wenn nicht, was zu tun ist.
func chatStatusHandler(w http.ResponseWriter, r *http.Request) {
	target := openCodeFromEnv()
	response := chatStatusResponse{URL: target.baseURL, Auth: target.password != "", Container: chatInContainer()}

	environment := project.Detect()
	if !environment.Installed {
		response.Message = "Keine Projektkonfiguration gefunden; der Chat braucht ein Projektverzeichnis."
		writeJSON(w, http.StatusOK, response)
		return
	}
	response.Installed = true
	response.Directory = environment.ProjectDir

	if message := target.containerGuard(); message != "" {
		response.Blocked = true
		response.Message = message
		response.Hint = openCodeInstallHint(target)
		writeJSON(w, http.StatusOK, response)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), openCodeHealthTimeout)
	defer cancel()
	request, err := target.newRequest(ctx, http.MethodGet, "/global/health", nil, nil)
	if err != nil {
		response.Message = fmt.Sprintf("Adresse des OpenCode-Dienstes ist ungültig: %v", err)
		writeJSON(w, http.StatusOK, response)
		return
	}
	upstream, err := openCodeClient.Do(request)
	if err != nil {
		response.Message = fmt.Sprintf("OpenCode-Dienst unter %s nicht erreichbar: %v", target.baseURL, err)
		response.Hint = openCodeInstallHint(target)
		writeJSON(w, http.StatusOK, response)
		return
	}
	defer upstream.Body.Close()

	switch {
	case upstream.StatusCode == http.StatusUnauthorized:
		response.Message = target.authMessage()
	case upstream.StatusCode != http.StatusOK:
		response.Message = fmt.Sprintf("OpenCode-Dienst unter %s antwortet mit Status %d.", target.baseURL, upstream.StatusCode)
	default:
		var health struct {
			Healthy bool   `json:"healthy"`
			Version string `json:"version"`
		}
		if err := json.NewDecoder(io.LimitReader(upstream.Body, chatBodyLimit)).Decode(&health); err != nil || !health.Healthy {
			response.Message = fmt.Sprintf("OpenCode-Dienst unter %s meldet sich nicht als bereit.", target.baseURL)
			break
		}
		response.Available = true
		response.Version = health.Version
	}
	writeJSON(w, http.StatusOK, response)
}

// chatTarget liefert Projektverzeichnis und Ziel jeder Weiterleitung. Welches
// Projekt gemeint ist, entscheidet der Server, nicht der Browser; ob der Dienst
// benutzt werden darf, ebenso.
func chatTarget(w http.ResponseWriter) (openCodeTarget, string, bool) {
	environment := project.Detect()
	if !environment.Installed {
		writeJSON(w, http.StatusConflict, chatError{Message: "Keine Projektkonfiguration gefunden; der Chat braucht ein Projektverzeichnis."})
		return openCodeTarget{}, "", false
	}
	target := openCodeFromEnv()
	if message := target.containerGuard(); message != "" {
		writeJSON(w, http.StatusForbidden, chatError{Message: message})
		return openCodeTarget{}, "", false
	}
	return target, environment.ProjectDir, true
}

// forwardOpenCode reicht eine Anfrage an OpenCode weiter und dessen Antwort
// zurück. 204 wird zu {"ok": true}, damit die Seite jede Antwort als JSON
// lesen kann — prompt_async etwa antwortet ohne Rumpf.
func forwardOpenCode(w http.ResponseWriter, r *http.Request, method string, path string, query url.Values, payload any) {
	target, directory, ok := chatTarget(w)
	if !ok {
		return
	}
	if query == nil {
		query = url.Values{}
	}
	query.Set("directory", directory)

	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, chatError{Message: fmt.Sprintf("Anfrage an OpenCode bauen: %v", err)})
			return
		}
		body = bytes.NewReader(data)
	}

	ctx, cancel := context.WithTimeout(r.Context(), openCodeRequestTimeout)
	defer cancel()
	request, err := target.newRequest(ctx, method, path, query, body)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, chatError{Message: fmt.Sprintf("Anfrage an OpenCode bauen: %v", err)})
		return
	}
	upstream, err := openCodeClient.Do(request)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, chatError{Message: fmt.Sprintf("OpenCode-Dienst unter %s nicht erreichbar: %v", target.baseURL, err)})
		return
	}
	defer upstream.Body.Close()

	switch upstream.StatusCode {
	case http.StatusNoContent:
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	case http.StatusUnauthorized:
		writeJSON(w, http.StatusBadGateway, chatError{Message: target.authMessage()})
		return
	}

	contentType := upstream.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/json; charset=utf-8"
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(upstream.StatusCode)
	if _, err := io.Copy(w, upstream.Body); err != nil {
		// Die Antwort läuft bereits; mehr als protokollieren geht nicht.
		fmt.Fprintf(os.Stderr, "Antwort von OpenCode weiterreichen: %v\n", err)
	}
}

// chatPathID liest die Kennung aus dem Pfad und weist alles ab, was nicht wie
// eine OpenCode-Kennung aussieht.
func chatPathID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	if !openCodeID.MatchString(id) {
		writeJSON(w, http.StatusBadRequest, chatError{Message: "Ungültige Kennung."})
		return "", false
	}
	return id, true
}

// decodeChatBody liest den JSON-Rumpf des Browsers. emptyOK lässt einen
// fehlenden Rumpf zu.
func decodeChatBody(w http.ResponseWriter, r *http.Request, target any, emptyOK bool) bool {
	err := json.NewDecoder(http.MaxBytesReader(w, r.Body, chatBodyLimit)).Decode(target)
	if err == nil || (emptyOK && errors.Is(err, io.EOF)) {
		return true
	}
	writeJSON(w, http.StatusBadRequest, chatError{Message: "Anfrage nicht lesbar."})
	return false
}

// chatSessionsHandler listet die Sitzungen des Projekts. roots=true lässt die
// Kind-Sitzungen der Subagenten weg: sie gehören zu ihrer Elternsitzung.
func chatSessionsHandler(w http.ResponseWriter, r *http.Request) {
	forwardOpenCode(w, r, http.MethodGet, "/session", url.Values{"roots": {"true"}, "limit": {"50"}}, nil)
}

// chatSessionHandler liefert eine einzelne Sitzung: die Seite einer Sitzung
// braucht ihren Titel, ohne die ganze Liste zu laden.
func chatSessionHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := chatPathID(w, r)
	if !ok {
		return
	}
	forwardOpenCode(w, r, http.MethodGet, "/session/"+id, nil, nil)
}

type chatCreateRequest struct {
	Title string `json:"title"`
}

// chatCreateSessionHandler legt eine Sitzung an. Ohne Titel vergibt OpenCode
// selbst einen, sobald die erste Nachricht da ist.
func chatCreateSessionHandler(w http.ResponseWriter, r *http.Request) {
	var input chatCreateRequest
	if !decodeChatBody(w, r, &input, true) {
		return
	}
	payload := map[string]string{}
	if title := strings.TrimSpace(input.Title); title != "" {
		payload["title"] = title
	}
	forwardOpenCode(w, r, http.MethodPost, "/session", nil, payload)
}

// chatMessagesHandler liefert den Verlauf einer Sitzung: {info, parts} je
// Nachricht. Danach hält der Ereignisstrom ihn aktuell.
func chatMessagesHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := chatPathID(w, r)
	if !ok {
		return
	}
	forwardOpenCode(w, r, http.MethodGet, "/session/"+id+"/message", nil, nil)
}

type chatPromptRequest struct {
	Text string `json:"text"`
}

// chatPromptHandler schickt Text in eine Sitzung. prompt_async kehrt sofort
// zurück; was OpenCode daraus macht, meldet ausschließlich der Ereignisstrom.
func chatPromptHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := chatPathID(w, r)
	if !ok {
		return
	}
	var input chatPromptRequest
	if !decodeChatBody(w, r, &input, false) {
		return
	}
	if strings.TrimSpace(input.Text) == "" {
		writeJSON(w, http.StatusBadRequest, chatError{Message: "Leere Nachricht."})
		return
	}
	payload := map[string]any{
		"parts": []map[string]string{{"type": "text", "text": input.Text}},
	}
	forwardOpenCode(w, r, http.MethodPost, "/session/"+id+"/prompt_async", nil, payload)
}

// chatAbortHandler bricht den laufenden Lauf einer Sitzung ab.
func chatAbortHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := chatPathID(w, r)
	if !ok {
		return
	}
	forwardOpenCode(w, r, http.MethodPost, "/session/"+id+"/abort", nil, nil)
}

// chatPermissionsHandler liefert die offenen Freigaben: eine Seite, die erst
// nach der Anfrage geöffnet wird, bekommt sie nicht mehr als Ereignis.
func chatPermissionsHandler(w http.ResponseWriter, r *http.Request) {
	forwardOpenCode(w, r, http.MethodGet, "/permission", nil, nil)
}

type chatPermissionReply struct {
	Reply string `json:"reply"`
}

// chatPermissionReplyHandler beantwortet eine Freigabe: einmal, immer oder
// abgelehnt.
func chatPermissionReplyHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := chatPathID(w, r)
	if !ok {
		return
	}
	var input chatPermissionReply
	if !decodeChatBody(w, r, &input, false) {
		return
	}
	switch input.Reply {
	case "once", "always", "reject":
	default:
		writeJSON(w, http.StatusBadRequest, chatError{Message: "Antwort muss once, always oder reject sein."})
		return
	}
	forwardOpenCode(w, r, http.MethodPost, "/permission/"+id+"/reply", nil, map[string]string{"reply": input.Reply})
}

// chatEventsHandler reicht den Ereignisstrom von OpenCode an den Browser
// durch, gefiltert auf das Projektverzeichnis. Jeder Block geht sofort weiter;
// gepuffert stockte die Live-Ausgabe.
//
// Der Strom endet nie von selbst. Er endet, wenn der Browser geht, wenn
// OpenCode die Verbindung schließt oder wenn der Server sich beendet — für
// Letzteres schließt Serve() state.streams, sonst wartete Shutdown auf ihn bis
// zur Frist.
func (state *serverState) chatEventsHandler(w http.ResponseWriter, r *http.Request) {
	target, directory, ok := chatTarget(w)
	if !ok {
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	go func() {
		select {
		case <-state.streams:
			cancel()
		case <-ctx.Done():
		}
	}()

	request, err := target.newRequest(ctx, http.MethodGet, "/event", url.Values{"directory": {directory}}, nil)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, chatError{Message: fmt.Sprintf("Anfrage an OpenCode bauen: %v", err)})
		return
	}
	request.Header.Set("Accept", "text/event-stream")
	upstream, err := openCodeClient.Do(request)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, chatError{Message: fmt.Sprintf("OpenCode-Dienst unter %s nicht erreichbar: %v", target.baseURL, err)})
		return
	}
	defer upstream.Body.Close()
	if upstream.StatusCode == http.StatusUnauthorized {
		writeJSON(w, http.StatusBadGateway, chatError{Message: target.authMessage()})
		return
	}
	if upstream.StatusCode != http.StatusOK {
		writeJSON(w, http.StatusBadGateway, chatError{Message: fmt.Sprintf("Ereignisstrom von OpenCode antwortet mit Status %d.", upstream.StatusCode)})
		return
	}

	controller := http.NewResponseController(w)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	if err := controller.Flush(); err != nil {
		return
	}

	buffer := make([]byte, 32*1024)
	for {
		n, readErr := upstream.Body.Read(buffer)
		if n > 0 {
			if _, err := w.Write(buffer[:n]); err != nil {
				return
			}
			if err := controller.Flush(); err != nil {
				return
			}
		}
		if readErr != nil {
			return
		}
	}
}

// closeStreams beendet alle offenen Ereignisströme. Mehrfach aufrufbar; ohne
// Kanal — in Tests, die nur Routen prüfen — tut es nichts.
func (state *serverState) closeStreams() {
	state.streamsOnce.Do(func() {
		if state.streams != nil {
			close(state.streams)
		}
	})
}
