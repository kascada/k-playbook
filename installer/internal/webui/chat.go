package webui

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
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
	// chatMessageLimit kürzt einen Fehlerrumpf von OpenCode, bevor er in einer
	// Meldung landet. Er kann eine ganze Antwortnachricht sein.
	chatMessageLimit = 500
	// chatCommandStateLimit begrenzt die Ablage der Command-Ausgänge. Sie wächst
	// je Sitzung um einen Eintrag und würde sonst nie kleiner.
	chatCommandStateLimit = 32
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
	forwardOpenCodeAdding(w, r, method, path, query, payload, nil)
}

// forwardOpenCodeAdding arbeitet wie forwardOpenCode, legt einer leeren
// Antwort aber noch Felder bei. prompt_async antwortet ohne Rumpf; die Seite
// braucht von dort trotzdem das erzeugte messageID zurück, sonst kann sie ihre
// vorläufige Blase der echten Nachricht nicht zuordnen.
//
// Antwortet OpenCode wider Erwarten mit einem Rumpf, geht der unverändert
// durch — die Seite bekommt dann kein messageID und nimmt den Rückfallweg.
func forwardOpenCodeAdding(w http.ResponseWriter, r *http.Request, method string, path string, query url.Values, payload any, extra map[string]any) {
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
		response := map[string]any{"ok": true}
		for key, value := range extra {
			response[key] = value
		}
		writeJSON(w, http.StatusOK, response)
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

// openCodeFailure trägt den Status, mit dem die Oberfläche einen gescheiterten
// Lesezugriff meldet, und die Meldung dazu.
type openCodeFailure struct {
	status  int
	message string
}

func (failure *openCodeFailure) Error() string { return failure.message }

// fetchOpenCode ruft OpenCode auf und gibt den Rumpf **zurück**, statt ihn
// durchzureichen. forwardOpenCode schreibt direkt in w und taugt deshalb
// überall dort nicht, wo der Server die Antwort selbst braucht: beim Filtern
// der Commands, bei der Namensprüfung vor dem Senden und beim abgekoppelten
// Aufruf, der niemandem mehr antwortet.
//
// Die Frist steckt im übergebenen Kontext — der abgekoppelte Aufruf braucht
// eine andere als eine gewöhnliche Weiterleitung.
func fetchOpenCode(ctx context.Context, target openCodeTarget, method string, path string, directory string, payload any) ([]byte, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, &openCodeFailure{status: http.StatusInternalServerError, message: fmt.Sprintf("Anfrage an OpenCode bauen: %v", err)}
		}
		body = bytes.NewReader(data)
	}
	request, err := target.newRequest(ctx, method, path, url.Values{"directory": {directory}}, body)
	if err != nil {
		return nil, &openCodeFailure{status: http.StatusInternalServerError, message: fmt.Sprintf("Anfrage an OpenCode bauen: %v", err)}
	}
	upstream, err := openCodeClient.Do(request)
	if err != nil {
		return nil, &openCodeFailure{status: http.StatusBadGateway, message: fmt.Sprintf("OpenCode-Dienst unter %s nicht erreichbar: %v", target.baseURL, err)}
	}
	defer upstream.Body.Close()
	data, err := io.ReadAll(io.LimitReader(upstream.Body, chatBodyLimit))
	if err != nil {
		return nil, &openCodeFailure{status: http.StatusBadGateway, message: fmt.Sprintf("Antwort von OpenCode lesen: %v", err)}
	}
	switch {
	case upstream.StatusCode == http.StatusUnauthorized:
		return nil, &openCodeFailure{status: http.StatusBadGateway, message: target.authMessage()}
	case upstream.StatusCode < 200 || upstream.StatusCode > 299:
		return nil, &openCodeFailure{
			status:  http.StatusBadGateway,
			message: strings.TrimSpace(fmt.Sprintf("OpenCode-Dienst antwortet mit Status %d. %s", upstream.StatusCode, shortenChatMessage(string(data)))),
		}
	}
	return data, nil
}

// writeOpenCodeFailure meldet der Seite, woran ein Lesezugriff gescheitert ist.
func writeOpenCodeFailure(w http.ResponseWriter, err error) {
	var failure *openCodeFailure
	if errors.As(err, &failure) {
		writeJSON(w, failure.status, chatError{Message: failure.message})
		return
	}
	writeJSON(w, http.StatusBadGateway, chatError{Message: err.Error()})
}

// shortenChatMessage kürzt einen Fehlerrumpf auf eine Länge, die in eine
// Meldung passt.
func shortenChatMessage(text string) string {
	// Gezählt wird in Zeichen, nicht in Bytes: mitten in einem Umlaut zu
	// schneiden ergäbe eine kaputte Meldung.
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= chatMessageLimit {
		return string(runes)
	}
	return string(runes[:chatMessageLimit]) + "…"
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

// --- Kennung der eigenen Nachricht -------------------------------------------
//
// prompt_async und command nehmen beide ein optionales messageID (Muster ^msg).
// Die Weiterleitung erzeugt es und gibt es zurück: nur damit kann die Seite die
// sofort gezeigte, vorläufige Blase der später eintreffenden Nachricht
// zuordnen — sonst stünde sie doppelt da oder bliebe als Geisterblase stehen.
//
// Das Format ist nicht frei wählbar. renderLog() sortiert die Nachrichten per
// reinem Zeichenvergleich ihrer Kennungen, und OpenCode vergibt sie mit
// Identifier.ascending: msg_ + 12 Hexziffern + 14 Zeichen aus [A-Za-z0-9], die
// Hexziffern (Zeitstempel_ms << 12 | Zähler), auf 48 Bit abgeschnitten und
// links mit Nullen gefüllt. Der Zähler beginnt bei jeder neuen Millisekunde
// wieder bei 1. Eine abweichende Kodierung — anderes Alphabet, andere Länge,
// keine Auffüllung — sortierte alle eigenen Nutzernachrichten als Block vor
// oder hinter alle fremden, dauerhaft und auch nach dem Laden.
//
// Belegt im Befund k-playbook-local/material/befunde/
// opencode-im-browser-einbinden.md, Abschnitt „Kodierung der msg_…-Kennung".

const (
	messageIDPrefix = "msg_"
	// messageIDTimeDigits ist die Länge des Zeitanteils in Hexziffern; 12
	// Ziffern sind die 48 Bit, auf die OpenCode abschneidet.
	messageIDTimeDigits = 12
	// messageIDCounterBits ist die Breite des Zählers im unteren Teil.
	messageIDCounterBits = 12
	// messageIDRandomLength ist der zufällige Rest, der zwei Kennungen
	// derselben Millisekunde aus verschiedenen Prozessen auseinanderhält.
	messageIDRandomLength = 14
)

// messageIDAlphabet ist das Alphabet des zufälligen Teils.
const messageIDAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// messageIDClock trägt die zuletzt benutzte Millisekunde und den Zähler darin.
// Ohne ihn bekämen zwei Nachrichten derselben Millisekunde denselben
// Zeitanteil und sortierten zufällig.
var messageIDClock struct {
	mu      sync.Mutex
	ms      int64
	counter uint64
}

// newMessageID erzeugt die Kennung für eine eigene Nachricht.
func newMessageID() string {
	return messageIDAt(time.Now().UnixMilli())
}

// messageIDAt erzeugt die Kennung zu einer gegebenen Millisekunde. Getrennt von
// newMessageID, damit ein Test die Ordnung gegen eine tatsächlich beobachtete
// Kennung von OpenCode prüfen kann — die liegt in der Vergangenheit.
//
// Eine rückwärts laufende Uhr wird nicht abgefangen: die Kennungen von OpenCode
// sind es ebenso wenig, und gegen sie muss die Ordnung stimmen.
func messageIDAt(ms int64) string {
	messageIDClock.mu.Lock()
	if ms == messageIDClock.ms {
		messageIDClock.counter++
	} else {
		messageIDClock.ms, messageIDClock.counter = ms, 1
	}
	counter := messageIDClock.counter
	messageIDClock.mu.Unlock()

	bits := uint64(messageIDTimeDigits * 4)
	value := (uint64(ms)<<messageIDCounterBits | counter) & (1<<bits - 1)

	var id strings.Builder
	id.Grow(len(messageIDPrefix) + messageIDTimeDigits + messageIDRandomLength)
	id.WriteString(messageIDPrefix)
	fmt.Fprintf(&id, "%0*x", messageIDTimeDigits, value)
	for i := 0; i < messageIDRandomLength; i++ {
		id.WriteByte(messageIDAlphabet[rand.IntN(len(messageIDAlphabet))])
	}
	return id.String()
}

type chatPromptRequest struct {
	Text string `json:"text"`
	// Agent ist der gewählte primäre Agent; leer heißt: OpenCode entscheidet.
	Agent string `json:"agent"`
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
	messageID := newMessageID()
	payload := map[string]any{
		"messageID": messageID,
		"parts":     []map[string]string{{"type": "text", "text": input.Text}},
	}
	if agent := strings.TrimSpace(input.Agent); agent != "" {
		payload["agent"] = agent
	}
	forwardOpenCodeAdding(w, r, http.MethodPost, "/session/"+id+"/prompt_async", nil, payload, map[string]any{"messageID": messageID})
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
	// Ab hier ist eine Chat-Ansicht live verbunden. Der Zähler sagt dem
	// Neustart nach einer Programmaktualisierung, dass er sie trennen würde.
	state.openStreams.Add(1)
	defer state.openStreams.Add(-1)
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

// --- Rückfragen -------------------------------------------------------------
//
// Eine Rückfrage des Agenten hält den Lauf an, bis sie beantwortet oder
// abgelehnt ist. Die alte API führt sie über alle Sitzungen hinweg unter
// /question; weder reply noch reject tragen eine sessionID — die gehört zur
// V2-Route /api/session/{sessionID}/question/…. Beide antworten mit einem
// nackten true, das forwardOpenCode unverändert durchreicht.

// chatQuestionsHandler liefert die offenen Rückfragen über alle Sitzungen des
// Projekts: eine Seite, die erst nach der Frage geöffnet wird, bekommt sie
// nicht mehr als Ereignis. Welche davon sie zeigt, entscheidet die Seite.
func chatQuestionsHandler(w http.ResponseWriter, r *http.Request) {
	forwardOpenCode(w, r, http.MethodGet, "/question", nil, nil)
}

type chatQuestionReply struct {
	// Answers trägt je Frage einen Eintrag in der Reihenfolge der Fragen; ein
	// Eintrag enthält die label-Werte der gewählten Optionen, eine
	// unbeantwortete Frage eine leere Liste. Alles andere lehnt das Decodieren
	// von [][]string bereits ab.
	Answers [][]string `json:"answers"`
}

// chatQuestionReplyHandler beantwortet eine Rückfrage.
func chatQuestionReplyHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := chatPathID(w, r)
	if !ok {
		return
	}
	var input chatQuestionReply
	if !decodeChatBody(w, r, &input, false) {
		return
	}
	if input.Answers == nil {
		writeJSON(w, http.StatusBadRequest, chatError{Message: "answers muss eine Liste von Listen von Zeichenketten sein."})
		return
	}
	// null als Eintrag wird zur leeren Liste: die Positionen der Fragen müssen
	// stimmen, und OpenCode erwartet dort eine Liste.
	answers := make([][]string, len(input.Answers))
	for i, answer := range input.Answers {
		if answer == nil {
			answer = []string{}
		}
		answers[i] = answer
	}
	forwardOpenCode(w, r, http.MethodPost, "/question/"+id+"/reply", nil, map[string]any{"answers": answers})
}

// chatQuestionRejectHandler lehnt eine Rückfrage ab; der Lauf entscheidet
// selbst, wie er damit weitermacht.
func chatQuestionRejectHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := chatPathID(w, r)
	if !ok {
		return
	}
	forwardOpenCode(w, r, http.MethodPost, "/question/"+id+"/reject", nil, nil)
}

// chatSessionChildrenHandler liefert die Kind-Sitzungen einer Sitzung. Die
// Seite braucht sie, um eine Rückfrage aus einer Kind-Sitzung als zu ihr
// gehörig zu erkennen; geliefert wird nur die erste Ebene.
func chatSessionChildrenHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := chatPathID(w, r)
	if !ok {
		return
	}
	forwardOpenCode(w, r, http.MethodGet, "/session/"+id+"/children", nil, nil)
}

// --- Commands ---------------------------------------------------------------
//
// GET /command liefert die Commands des Projekts; Einträge mit führendem _
// sind interne Bausteine und werden weder angeboten noch beim Senden
// angenommen. POST /session/{sessionID}/command antwortet erst nach dem
// ganzen Lauf — dafür taugt keine gewöhnliche Weiterleitung. Der Aufruf wird
// deshalb abgekoppelt, und sein Ausgang steht danach unter command-state
// bereit.

// chatCommand ist ein Command, wie die Seite ihn braucht: Name für die
// Weiche, der Rest für die Vorschlagsliste.
type chatCommand struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Source      string   `json:"source,omitempty"`
	Hints       []string `json:"hints"`
	Subtask     bool     `json:"subtask"`
}

// openCodeCommands holt die Commands und lässt die internen Bausteine weg.
// Dieselbe Liste trägt die Vorschläge der Seite und die Namensprüfung beim
// Senden: ein von Hand getipptes /_docs:code ist damit auch dort unbekannt.
func openCodeCommands(ctx context.Context, target openCodeTarget, directory string) ([]chatCommand, error) {
	data, err := fetchOpenCode(ctx, target, http.MethodGet, "/command", directory, nil)
	if err != nil {
		return nil, err
	}
	var all []chatCommand
	if err := json.Unmarshal(data, &all); err != nil {
		return nil, &openCodeFailure{status: http.StatusBadGateway, message: fmt.Sprintf("Liste der Commands von OpenCode nicht lesbar: %v", err)}
	}
	commands := make([]chatCommand, 0, len(all))
	for _, command := range all {
		if command.Name == "" || strings.HasPrefix(command.Name, "_") {
			continue
		}
		if command.Hints == nil {
			command.Hints = []string{}
		}
		commands = append(commands, command)
	}
	return commands, nil
}

// chatCommandsHandler liefert der Seite die gefilterte Liste.
func chatCommandsHandler(w http.ResponseWriter, r *http.Request) {
	target, directory, ok := chatTarget(w)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), openCodeRequestTimeout)
	defer cancel()
	commands, err := openCodeCommands(ctx, target, directory)
	if err != nil {
		writeOpenCodeFailure(w, err)
		return
	}
	writeJSON(w, http.StatusOK, commands)
}

type chatCommandRequest struct {
	Command   string `json:"command"`
	Arguments string `json:"arguments"`
	Agent     string `json:"agent"`
}

// chatCommandHandler nimmt einen Command an und antwortet sofort: der Aufruf
// bei OpenCode kehrt erst nach dem ganzen Lauf zurück, und die Seite soll
// nicht so lange warten.
//
// Alles, was die Anfrage braucht — Ziel, Verzeichnis, Rumpf und die Prüfung
// des Namens —, läuft vor dem Start der Goroutine; die fasst w und r nicht
// mehr an.
func (state *serverState) chatCommandHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := chatPathID(w, r)
	if !ok {
		return
	}
	var input chatCommandRequest
	if !decodeChatBody(w, r, &input, false) {
		return
	}
	name := strings.TrimSpace(input.Command)
	if name == "" {
		writeJSON(w, http.StatusBadRequest, chatError{Message: "Kein Command genannt."})
		return
	}
	target, directory, ok := chatTarget(w)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), openCodeRequestTimeout)
	defer cancel()
	commands, err := openCodeCommands(ctx, target, directory)
	if err != nil {
		writeOpenCodeFailure(w, err)
		return
	}
	known := false
	for _, command := range commands {
		if command.Name == name {
			known = true
			break
		}
	}
	if !known {
		writeJSON(w, http.StatusBadRequest, chatError{Message: fmt.Sprintf("Unbekannter Command: %s", name)})
		return
	}

	messageID := newMessageID()
	payload := map[string]any{"command": name, "arguments": input.Arguments, "messageID": messageID}
	if agent := strings.TrimSpace(input.Agent); agent != "" {
		payload["agent"] = agent
	}
	state.noteCommandRunning(id, messageID)
	go state.runCommand(id, messageID, target, directory, payload)
	// Die Kennung geht mit der 202 zurück: die Seite hängt ihre vorläufige
	// Blase daran und verwirft sie, sobald die echte Nachricht eintrifft.
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true, "messageID": messageID})
}

// runCommand schickt den Command ab und wartet auf das Ende des Laufs — das
// können Minuten sein.
//
// Der Kontext kommt aus context.Background() und nicht aus der Anfrage: die
// ist mit der 202 beendet, ein daran hängender Aufruf stürbe vor der
// Bearbeitung. Abgebrochen wird er allein über state.streams, und das beendet
// ausdrücklich nur die wartende HTTP-Anfrage — der Lauf in OpenCode läuft
// weiter und endet nur über POST …/abort.
//
// messageID ist der Schlüssel dieses Aufrufs in der Ablage: sein Ende räumt nur
// ihn, nicht einen zweiten Command derselben Sitzung.
func (state *serverState) runCommand(sessionID string, messageID string, target openCodeTarget, directory string, payload map[string]any) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-state.streams:
			cancel()
		case <-ctx.Done():
		}
	}()

	message := ""
	if _, err := fetchOpenCode(ctx, target, http.MethodPost, "/session/"+sessionID+"/command", directory, payload); err != nil {
		message = err.Error()
	}
	state.noteCommandFinished(sessionID, messageID, message)
}

// --- Agenten ------------------------------------------------------------------
//
// GET /agent liefert die Agenten des Projekts. Angeboten wird alles außer
// mode: "subagent" und hidden: true — eine Prüfung auf mode == "primary" würfe
// die Agenten mit mode "all" fälschlich weg und böte versteckte an.

// chatAgent ist ein Agent, wie die Auswahl der Seite ihn braucht. Hidden dient
// nur dem Filtern; in der Antwort steht es nie, weil jeder übrige Agent es auf
// false hat und omitempty es dann weglässt.
type chatAgent struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Mode        string `json:"mode,omitempty"`
	Hidden      bool   `json:"hidden,omitempty"`
}

// chatAgentsHandler liefert die Agenten, unter denen die Seite wählen lässt.
func chatAgentsHandler(w http.ResponseWriter, r *http.Request) {
	target, directory, ok := chatTarget(w)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), openCodeRequestTimeout)
	defer cancel()
	data, err := fetchOpenCode(ctx, target, http.MethodGet, "/agent", directory, nil)
	if err != nil {
		writeOpenCodeFailure(w, err)
		return
	}
	var all []chatAgent
	if err := json.Unmarshal(data, &all); err != nil {
		writeJSON(w, http.StatusBadGateway, chatError{Message: fmt.Sprintf("Liste der Agenten von OpenCode nicht lesbar: %v", err)})
		return
	}
	agents := make([]chatAgent, 0, len(all))
	for _, agent := range all {
		if agent.Name == "" || agent.Mode == "subagent" || agent.Hidden {
			continue
		}
		agents = append(agents, agent)
	}
	writeJSON(w, http.StatusOK, agents)
}

// --- Ausgang des abgekoppelten Aufrufs --------------------------------------
//
// Scheitert der Aufruf nach der 202 — 404, 400, Modell- oder Providerfehler,
// Verbindungsabbruch —, entsteht kein Ereignis. Nur zu protokollieren ließe
// die Seite dauerhaft auf „Arbeitet" stehen. Der Server merkt sich deshalb je
// Sitzung, was läuft und was gescheitert ist, und die Seite holt es ab.
//
// Eine Sitzung kann mehrere Commands zugleich laufen haben. Ein einziger
// Eintrag, den jedes Ende überschreibt, verlöre dabei Auskunft: endet /cmd1
// erfolgreich, nachdem /cmd2 schon gescheitert ist, würde aus failed done, und
// ein noch laufender /cmd2 meldete sich als done. Laufende Aufrufe und Fehler
// stehen deshalb getrennt.

// chatCommandRuns ist der Stand einer Sitzung. Dass es ihn gibt, ist zugleich
// die Marke, dass diese Sitzung überhaupt einen Command von hier gesendet hat —
// sie trennt done von unknown.
type chatCommandRuns struct {
	// running hält die laufenden Aufrufe, geschlüsselt über das messageID, das
	// der Handler je Aufruf erzeugt. Ein Ende entfernt nur seinen Schlüssel.
	running map[string]struct{}
	// failed und message sind der ungelesene Fehler. Tritt ein zweiter auf,
	// bevor der erste gelesen ist, bleibt der erste stehen.
	failed  bool
	message string
	// at ist die letzte Änderung; nach ihr wird verdrängt.
	at time.Time
}

type chatCommandStateResponse struct {
	State   string `json:"state"`
	Message string `json:"message,omitempty"`
}

// runningCommands zählt die laufenden abgekoppelten Command-Aufrufe über alle
// Sitzungen. Ein Neustart des Dienstes beendet ihre Goroutinen; ihren Ausgang
// meldete danach niemand mehr.
func (state *serverState) runningCommands() int {
	state.commandMu.Lock()
	defer state.commandMu.Unlock()
	count := 0
	for _, runs := range state.commandRuns {
		count += len(runs.running)
	}
	return count
}

// commandRunsFor liefert den Stand einer Sitzung und legt ihn bei Bedarf an.
//
// Wird nur mit gehaltenem commandMu gerufen.
func (state *serverState) commandRunsFor(sessionID string) *chatCommandRuns {
	if state.commandRuns == nil {
		state.commandRuns = map[string]*chatCommandRuns{}
	}
	runs := state.commandRuns[sessionID]
	if runs == nil {
		runs = &chatCommandRuns{running: map[string]struct{}{}}
		state.commandRuns[sessionID] = runs
	}
	return runs
}

// noteCommandRunning hält fest, dass für diese Sitzung der abgekoppelte Aufruf
// mit diesem messageID läuft.
func (state *serverState) noteCommandRunning(sessionID string, messageID string) {
	state.commandMu.Lock()
	defer state.commandMu.Unlock()
	runs := state.commandRunsFor(sessionID)
	runs.running[messageID] = struct{}{}
	runs.at = time.Now()
}

// noteCommandFinished hält das Ende eines Aufrufs fest: er verlässt die Menge
// der laufenden, und eine Meldung bedeutet gescheitert. Ein schon stehender,
// ungelesener Fehler bleibt dabei stehen. Danach wird die Ablage begrenzt.
func (state *serverState) noteCommandFinished(sessionID string, messageID string, message string) {
	state.commandMu.Lock()
	defer state.commandMu.Unlock()
	runs := state.commandRunsFor(sessionID)
	delete(runs.running, messageID)
	if message != "" && !runs.failed {
		runs.failed = true
		runs.message = shortenChatMessage(message)
	}
	runs.at = time.Now()
	state.trimCommandRuns()
}

// trimCommandRuns hält die Ablage klein. Verdrängt werden nur Sitzungen ohne
// laufenden Aufruf und ohne ungelesenen Fehler: fiele ein ungelesener Fehler
// vor dem Abholen heraus, antwortete command-state unknown, die Seite täte
// nichts und bliebe auf „Arbeitet" stehen — genau der Zustand, gegen den der
// Rückweg gebaut ist. Ein laufender Aufruf wird aus demselben Grund nicht
// verdrängt.
//
// Wird nur mit gehaltenem commandMu gerufen.
func (state *serverState) trimCommandRuns() {
	for len(state.commandRuns) > chatCommandStateLimit {
		oldestID := ""
		var oldest *chatCommandRuns
		for id, runs := range state.commandRuns {
			if len(runs.running) > 0 || runs.failed {
				continue
			}
			if oldest == nil || runs.at.Before(oldest.at) {
				oldestID, oldest = id, runs
			}
		}
		if oldest == nil {
			return
		}
		delete(state.commandRuns, oldestID)
	}
}

// commandState liest den Stand einer Sitzung. Ein ungelesener Fehler geht
// genau einmal hinaus und ist danach geräumt; läuft dann noch ein Aufruf,
// meldet die nächste Antwort running. Dass bei zwei offenen Tabs derselben
// Sitzung der erste Leser den Fehler wegräumt, wird bewusst hingenommen.
func (state *serverState) commandState(sessionID string) chatCommandStateResponse {
	state.commandMu.Lock()
	defer state.commandMu.Unlock()
	runs := state.commandRuns[sessionID]
	switch {
	case runs == nil:
		// Keine Auskunft: GUI neu gestartet, Command aus dem Terminal. done
		// wäre hier irreführend, die Seite lässt den Laufzustand unberührt.
		return chatCommandStateResponse{State: "unknown"}
	case runs.failed:
		message := runs.message
		runs.failed, runs.message = false, ""
		state.trimCommandRuns()
		return chatCommandStateResponse{State: "failed", Message: message}
	case len(runs.running) > 0:
		return chatCommandStateResponse{State: "running"}
	default:
		return chatCommandStateResponse{State: "done"}
	}
}

// chatCommandStateHandler liefert den Ausgang an die Seite. Er durchläuft
// chatTarget — Projektbezug und containerGuard gelten auch hier —, leitet aber
// nichts weiter und trägt deshalb kein directory: er liest nur lokalen Zustand.
func (state *serverState) chatCommandStateHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := chatPathID(w, r)
	if !ok {
		return
	}
	if _, _, ok := chatTarget(w); !ok {
		return
	}
	writeJSON(w, http.StatusOK, state.commandState(id))
}
