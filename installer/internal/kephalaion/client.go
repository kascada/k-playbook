package kephalaion

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// DefaultTimeout ist die Zeitgrenze für eine ganze Prüfung: initialize,
// notifications/initialized und tools/call zusammen. Eine abgewiesene
// Loopback-Verbindung scheitert sofort; die Grenze greift nur bei einem Node,
// der annimmt und nicht antwortet.
const DefaultTimeout = 3 * time.Second

// protocolVersion ist die MCP-Fassung, die der Client anbietet. Der Node
// antwortet mit seiner; die schickt der Client danach mit.
const protocolVersion = "2025-06-18"

// maxResponseBytes begrenzt jede Antwort des Nodes. whoami ist klein; mehr
// wird nicht gelesen.
const maxResponseBytes = 1 << 20

// probeError ist ein Fehler der Verbindung zum Node oder seiner Antwort —
// nicht eine Auskunft von whoami. Text ist bereits ohne Token.
type probeError struct {
	text string
}

func (e *probeError) Error() string { return e.text }

// whoamiOutput ist die Struktur von whoami, soweit sie hier gebraucht wird
// (WhoamiOutput in Kephalaions internal/node/mcpnode/whoami.go).
type whoamiOutput struct {
	Version     string        `json:"version"`
	Hidden      bool          `json:"hidden"`
	Hubs        []whoamiHub   `json:"hubs"`
	UnknownHubs []string      `json:"unknown_hubs"`
	Update      *whoamiUpdate `json:"update"`
}

type whoamiUpdate struct {
	UpdateAvailable bool   `json:"update_available"`
	Latest          string `json:"latest"`
}

type whoamiHub struct {
	Hub         string             `json:"hub"`
	Login       string             `json:"login"`
	Node        string             `json:"node"`
	Sync        whoamiSync         `json:"sync"`
	Account     string             `json:"account"`
	User        string             `json:"user"`
	Collections []whoamiCollection `json:"collections"`
}

type whoamiSync struct {
	NeverSynced bool   `json:"never_synced"`
	LastSuccess string `json:"last_success"`
	Revision    *int64 `json:"revision"`
	LastError   string `json:"last_error"`
	LastErrorAt string `json:"last_error_at"`
}

type whoamiCollection struct {
	Collection string   `json:"collection"`
	Address    string   `json:"address"`
	Rights     []string `json:"rights"`
	Dirs       []string `json:"dirs"`
}

// newHTTPClient baut einen Client, der nur das Nötigste tut: kein Proxy aus
// der Umgebung — die Anfrage geht an Loopback und nirgends sonst hin —, keine
// Weiterleitung — der Node sendet keine, und ein Folgen trüge die Header an ein
// anderes Ziel —, keine offenen Verbindungen danach.
func newHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Proxy:                  nil,
			DialContext:            (&net.Dialer{Timeout: DefaultTimeout}).DialContext,
			DisableKeepAlives:      true,
			MaxResponseHeaderBytes: 64 << 10,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// rpcResponse ist eine JSON-RPC-Antwort.
type rpcResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// callWhoami fragt den Node unter endpoint nach whoami, mit dem Header-Paar des
// Kandidaten. initialize geht ohne Anmeldung; das Paar steht nur an
// tools/call. Jeder Fehlertext läuft durch redact, bevor er das Paket
// verlässt.
func callWhoami(ctx context.Context, endpoint string, candidate Candidate, token string) (*whoamiOutput, error) {
	client := newHTTPClient()
	redactErr := func(format string, args ...any) error {
		return &probeError{text: redact(fmt.Sprintf(format, args...), token)}
	}

	initParams := map[string]any{
		"protocolVersion": protocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "k-playbook", "version": "1"},
	}
	initResult, session, err := post(ctx, client, endpoint, nil, "", "", 1, "initialize", initParams)
	if err != nil {
		return nil, redactErr("%v", err)
	}
	var init struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if err := json.Unmarshal(initResult, &init); err != nil || init.ProtocolVersion == "" {
		return nil, redactErr("Antwort auf initialize nicht lesbar")
	}

	// Die Bestätigung von initialize: Ein zustandsloser Node braucht sie
	// nicht, ein Server mit Sitzung erwartet sie. Ihr Ausgang zählt nicht.
	_ = notify(ctx, client, endpoint, init.ProtocolVersion, session, "notifications/initialized")

	headers := map[string]string{
		"X-Keph-Account-" + candidate.Hub: candidate.Account,
		"X-Keph-Token-" + candidate.Hub:   token,
	}
	callParams := map[string]any{"name": "whoami", "arguments": map[string]any{}}
	callResult, _, err := post(ctx, client, endpoint, headers, init.ProtocolVersion, session, 2, "tools/call", callParams)
	if err != nil {
		return nil, redactErr("%v", err)
	}
	var result struct {
		IsError           bool            `json:"isError"`
		StructuredContent json.RawMessage `json:"structuredContent"`
		Content           []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(callResult, &result); err != nil {
		return nil, redactErr("Antwort auf whoami nicht lesbar")
	}
	if result.IsError {
		var parts []string
		for _, c := range result.Content {
			parts = append(parts, c.Text)
		}
		return nil, redactErr("whoami meldet einen Fehler: %s", strings.Join(parts, " "))
	}
	if len(result.StructuredContent) == 0 {
		return nil, redactErr("Antwort auf whoami ohne strukturierten Inhalt")
	}
	var out whoamiOutput
	if err := json.Unmarshal(result.StructuredContent, &out); err != nil {
		return nil, redactErr("Antwort auf whoami nicht lesbar")
	}
	return &out, nil
}

// post schickt eine JSON-RPC-Anfrage und liefert ihr Ergebnis. Fehler nennen
// nie den Rumpf einer Antwort, nur Status und Art.
func post(ctx context.Context, client *http.Client, endpoint string, headers map[string]string,
	version, session string, id int, method string, params any) (json.RawMessage, string, error) {
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
	if err != nil {
		return nil, "", err
	}
	resp, err := send(ctx, client, endpoint, headers, version, session, body)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("%s: HTTP %d", method, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, "", fmt.Errorf("%s: Antwort abgebrochen", method)
	}
	msg, err := parseRPC(data, resp.Header.Get("Content-Type"))
	if err != nil {
		return nil, "", fmt.Errorf("%s: Antwort nicht lesbar", method)
	}
	if msg.Error != nil {
		return nil, "", fmt.Errorf("%s: Fehler %d vom Node", method, msg.Error.Code)
	}
	if len(msg.Result) == 0 {
		return nil, "", fmt.Errorf("%s: Antwort ohne Ergebnis", method)
	}
	return msg.Result, resp.Header.Get("Mcp-Session-Id"), nil
}

// notify schickt eine Benachrichtigung ohne Antworterwartung.
func notify(ctx context.Context, client *http.Client, endpoint, version, session, method string) error {
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": method})
	if err != nil {
		return err
	}
	resp, err := send(ctx, client, endpoint, nil, version, session, body)
	if err != nil {
		return err
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseBytes))
	return resp.Body.Close()
}

// send setzt die Anfrage ab. Ein Fehler der Verbindung wird hier in einen
// eigenen Satz übersetzt; der Text von net/http nennt Methode und URL, aber
// keine Header.
func send(ctx context.Context, client *http.Client, endpoint string, headers map[string]string,
	version, session string, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, errors.New("Anfrage an den Node nicht baubar")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if version != "" {
		req.Header.Set("MCP-Protocol-Version", version)
	}
	if session != "" {
		req.Header.Set("Mcp-Session-Id", session)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("keine Antwort innerhalb von %s", DefaultTimeout)
		}
		var opErr *net.OpError
		if errors.As(err, &opErr) && opErr.Err != nil {
			return nil, fmt.Errorf("keine Verbindung (%s)", opErr.Err.Error())
		}
		return nil, errors.New("keine Verbindung")
	}
	return resp, nil
}

// parseRPC liest eine Antwort als JSON oder als SSE mit data:-Zeilen; bei SSE
// zählt die letzte.
func parseRPC(data []byte, contentType string) (rpcResponse, error) {
	var msg rpcResponse
	if strings.Contains(contentType, "text/event-stream") {
		var last string
		for _, line := range strings.Split(string(data), "\n") {
			if rest, ok := strings.CutPrefix(line, "data:"); ok {
				last = strings.TrimSpace(rest)
			}
		}
		if last == "" {
			return msg, errors.New("leerer Ereignisstrom")
		}
		data = []byte(last)
	}
	err := json.Unmarshal(data, &msg)
	return msg, err
}

// redact entfernt ein Token aus einem Text — die letzte Sicherung, falls ein
// Fehlertext es doch enthält.
func redact(text, token string) string {
	if token == "" {
		return text
	}
	return strings.ReplaceAll(text, token, "***")
}
