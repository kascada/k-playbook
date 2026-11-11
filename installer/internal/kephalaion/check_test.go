package kephalaion

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeNode ist ein gefälschter Node: Er beantwortet initialize wie der echte
// (zustandslos, JSON), nimmt Benachrichtigungen mit 202 an und gibt für
// tools/call whoami zurück, was reply aus den empfangenen Headern baut.
type fakeNode struct {
	server *httptest.Server
	// seen sind die Header jeder Anfrage an tools/call.
	seen []http.Header
	// initSeen sind die Header jeder Anfrage an initialize.
	initSeen []http.Header
}

func newFakeNode(t *testing.T, reply func(h http.Header) (status int, contentType string, body string)) *fakeNode {
	t.Helper()
	node := &fakeNode{}
	node.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		var msg struct {
			ID     *int   `json:"id"`
			Method string `json:"method"`
		}
		_ = json.Unmarshal(data, &msg)
		switch msg.Method {
		case "initialize":
			node.initSeen = append(node.initSeen, r.Header.Clone())
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":{"protocolVersion":"2025-06-18","capabilities":{"tools":{}},"serverInfo":{"name":"kephalaion","version":"v0.2.4"}}}`, *msg.ID)
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
		case "tools/call":
			node.seen = append(node.seen, r.Header.Clone())
			status, contentType, body := reply(r.Header)
			w.Header().Set("Content-Type", contentType)
			w.WriteHeader(status)
			io.WriteString(w, body)
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	t.Cleanup(node.server.Close)
	return node
}

// whoamiBody baut die Antwort auf tools/call mit structuredContent.
func whoamiBody(structured string) string {
	return `{"jsonrpc":"2.0","id":2,"result":{"content":[{"type":"text","text":"kephalaion v0.2.4"}],"structuredContent":` + structured + `}}`
}

// sse verpackt eine Antwort als Ereignisstrom; die data:-Zeile ist einzeilig.
func sse(body string) string {
	var out bytes.Buffer
	if err := json.Compact(&out, []byte(body)); err != nil {
		panic(err)
	}
	return "event: message\ndata: " + out.String() + "\n\n"
}

var goodBinding = Binding{Hub: "vm", Collection: "k-playbook"}

// testAccess baut einen Zugang, wie die Auskunft ihn ergibt: Node unter
// endpoint, die einzige Token-Datei für vm/kamran-wsl.
func testAccess(t *testing.T, endpoint string) Access {
	t.Helper()
	tokens := tokensWith(t)
	info := infoFor(tokens, hubWith(tokens, "vm", "kamran-wsl", ChoiceOnly))
	info.Node.URL = endpoint
	access, err := ResolveInfo(info, "vm")
	if err != nil {
		t.Fatalf("Zugang: %v", err)
	}
	return access
}

const okHub = `{"hub":"vm","login":"ok","node":"wsl-kleist","account":"kamran-wsl","user":"kamran",
 "sync":{"last_success":"2026-10-09T18:30:31Z","revision":41},
 "collections":[{"address":"vm:k-playbook","collection":"k-playbook","dirs":[],"rights":["read","write"]},
                {"address":"vm:test","collection":"test","dirs":[],"rights":["read","write","vendor/k-playbook","dir docs/"]}]}`

func TestCheckBindingStates(t *testing.T) {
	cases := []struct {
		name       string
		binding    Binding
		structured string
		want       State
	}{
		{"ok", goodBinding, `{"version":"v0.2.4","hubs":[` + okHub + `],"unknown_hubs":[]}`, StateOK},
		{"collection", Binding{Hub: "vm", Collection: "fremd"},
			`{"version":"v0.2.4","hubs":[` + okHub + `],"unknown_hubs":[]}`, StateCollectionUnreadable},
		{"unknown hub", goodBinding, `{"version":"v0.2.4","hubs":[],"unknown_hubs":["vm"]}`, StateHubUnknown},
		{"hub fehlt in hubs", goodBinding, `{"version":"v0.2.4","hubs":[{"hub":"anders","login":"missing","node":"n","sync":{}}],"unknown_hubs":[]}`, StateHubUnknown},
		{"never synced", goodBinding, `{"version":"v0.2.4","hubs":[{"hub":"vm","login":"invalid","node":"n","sync":{"never_synced":true}}],"unknown_hubs":[]}`, StateHubNeverSynced},
		{"invalid", goodBinding, `{"version":"v0.2.4","hubs":[{"hub":"vm","login":"invalid","node":"n","sync":{"last_success":"2026-10-09T18:30:31Z","revision":3}}],"unknown_hubs":[]}`, StateLoginInvalid},
		{"missing", goodBinding, `{"version":"v0.2.4","hubs":[{"hub":"vm","login":"missing","node":"n","sync":{"revision":3}}],"unknown_hubs":[]}`, StateLoginMissing},
		{"replica", goodBinding, `{"version":"v0.2.4","hubs":[{"hub":"vm","login":"missing","node":"n","sync":{"last_error":"Replica nicht lesbar"}}],"unknown_hubs":[]}`, StateReplicaUnreadable},
		{"hidden", goodBinding, `{"hidden":true,"hubs":[],"unknown_hubs":[]}`, StateLoginInvalid},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			node := newFakeNode(t, func(http.Header) (int, string, string) {
				return 200, "application/json", whoamiBody(c.structured)
			})
			status := checkAccess(context.Background(), c.binding, testAccess(t, node.server.URL+"/mcp"))
			if status.State != c.want {
				t.Fatalf("Zustand %s, erwartet %s (%s)", status.State, c.want, status.Message)
			}
			if status.Message == "" {
				t.Fatal("Zustand ohne Satz")
			}
			if status.OK != (c.want == StateOK) {
				t.Fatalf("OK=%v bei %s", status.OK, status.State)
			}
		})
	}
}

func TestCheckBindingOKCarriesRightsScopesAndSync(t *testing.T) {
	node := newFakeNode(t, func(http.Header) (int, string, string) {
		return 200, "application/json", whoamiBody(`{"version":"v0.2.4","hubs":[` + okHub + `],"unknown_hubs":[]}`)
	})
	access := testAccess(t, node.server.URL+"/mcp")

	status := checkAccess(context.Background(), goodBinding, access)
	if status.Address != "vm:k-playbook" || status.User != "kamran" {
		t.Fatalf("%+v", status)
	}
	if strings.Join(status.Rights, ",") != "read,write" || len(status.Scopes) != 0 {
		t.Fatalf("Rechte %v Scopes %v", status.Rights, status.Scopes)
	}
	if status.Sync == nil || status.Sync.Revision == nil || *status.Sync.Revision != 41 || status.Sync.LastSuccess == "" {
		t.Fatalf("Abgleich: %+v", status.Sync)
	}

	if status.Account != "kamran-wsl" || status.Choice != ChoiceOnly {
		t.Fatalf("Account und Herkunft: %+v", status)
	}
	withScopes := checkAccess(context.Background(), Binding{Hub: "vm", Collection: "test"}, access)
	if strings.Join(withScopes.Scopes, ",") != "vendor/k-playbook,dir docs/" {
		t.Fatalf("%+v", withScopes)
	}

	// Gesendet wird das Header-Paar nur an tools/call, nicht an initialize.
	if len(node.seen) == 0 || node.seen[0].Get("X-Keph-Account-vm") != "kamran-wsl" || node.seen[0].Get("X-Keph-Token-vm") != testToken {
		t.Fatalf("Header-Paar fehlt: %v", node.seen)
	}
	for _, h := range node.initSeen {
		if h.Get("X-Keph-Token-vm") != "" {
			t.Fatal("Token an initialize geschickt")
		}
	}
}

func TestCheckStatesBeforeTheNode(t *testing.T) {
	fake := newFakeClient(t)
	const configFile = ".k-playbook/K-PLAYBOOK.yaml"
	if s := Check(context.Background(), Binding{}, configFile); s.State != StateNotConfigured || s.Configured ||
		!strings.Contains(s.Message, configFile) || !strings.Contains(s.Message, "kephalaion:") || !strings.Contains(s.Message, "Abhilfe") {
		t.Fatalf("nicht eingetragen: %+v", s)
	}
	if s := Check(context.Background(), Binding{Hub: "../vm", Collection: "k-playbook"}, configFile); s.State != StateInvalidEntry || s.Address != "" {
		t.Fatalf("ungültig: %+v", s)
	}
	if s := Check(context.Background(), Binding{Hub: "vm"}, configFile); s.State != StateInvalidEntry {
		t.Fatalf("ohne Collection: %+v", s)
	}
	if fake.argsOrNil(t) != nil {
		t.Fatal("Auskunft vor der Prüfung der Namen gefragt")
	}

	tokens := tokensWith(t)
	info := infoFor(tokens, hubWith(tokens, "vm", "kamran-wsl", ChoiceOnly))
	info.Node.URL = "http://10.1.2.3:7433/mcp"
	fake.setInfo(t, info)
	if s := Check(context.Background(), goodBinding, configFile); s.State != StateNodeAddress || s.Address != "vm:k-playbook" {
		t.Fatalf("fremde Adresse: %+v", s)
	}

	info = infoFor(tokens, ClientHub{Alias: "vm", Choice: ChoiceMissing, Candidates: []string{}, Pending: []string{}})
	fake.setInfo(t, info)
	if s := Check(context.Background(), goodBinding, configFile); s.State != StateTokenMissing {
		t.Fatalf("ohne Token: %+v", s)
	}
	info = infoFor(tokens, ClientHub{Alias: "vm", Choice: ChoiceMissing, Candidates: []string{"a", "b"}, Pending: []string{}})
	fake.setInfo(t, info)
	if s := Check(context.Background(), goodBinding, configFile); s.State != StateAccountChoiceMissing {
		t.Fatalf("Wahl fehlt: %+v", s)
	}
	fake.set(t, "", "", 2)
	if s := Check(context.Background(), goodBinding, configFile); s.State != StateNoKephalaion {
		t.Fatalf("zu altes Binary: %+v", s)
	}
}

// Der ganze Weg: Auskunft über das Ersatz-Binary, whoami am Ersatz-Node.
// Account und Herkunft kommen aus der Auskunft, nicht aus der Anbindung; ein
// offenes rotate und ein Problem der config stehen hinter der Meldung.
func TestCheckBindingTakesAccountFromTheClientInfo(t *testing.T) {
	node := newFakeNode(t, func(http.Header) (int, string, string) {
		return 200, "application/json", whoamiBody(`{"version":"v0.2.4","hubs":[` + okHub + `],"unknown_hubs":[]}`)
	})
	fake := newFakeClient(t)
	tokens := tokensWith(t)
	hub := hubWith(tokens, "vm", "kamran-wsl", ChoiceDefault, "kamran-wsl", "zweit")
	hub.Pending = []string{"kamran-wsl"}
	info := infoFor(tokens, hub)
	info.Node.URL = node.server.URL + "/mcp"
	info.Config.Problem = ptr("kein lesbares YAML")
	fake.setInfo(t, info)

	status := Check(context.Background(), goodBinding, "K-PLAYBOOK.yaml")
	if status.State != StateOK || status.Account != "kamran-wsl" || status.Choice != ChoiceDefault {
		t.Fatalf("%+v", status)
	}
	if node.seen[0].Get("X-Keph-Account-vm") != "kamran-wsl" {
		t.Fatalf("Account am Node: %q", node.seen[0].Get("X-Keph-Account-vm"))
	}
	for _, want := range []string{"Angemeldet als kamran-wsl", "kephalaion node account check vm kamran-wsl --token-file", "kein lesbares YAML"} {
		if !strings.Contains(status.Message, want) {
			t.Errorf("Meldung ohne %q: %s", want, status.Message)
		}
	}
	if got := fake.args(t); got != "node client --json --format 1 --no-probe --hub vm" {
		t.Fatalf("Aufruf: %q", got)
	}
}

// SplitRemedy schneidet nur im Satz des Zustands: Die Abhilfe eines
// Hinweises dahinter wird nie die des Zustands und bleibt beim Hinweis.
func TestStatusSplitRemedy(t *testing.T) {
	const rotate = "Rotate offen. Abhilfe: kephalaion node account check vm a --token-file f räumt auf."
	const config = "config nicht lesbar. Abhilfe: die config prüfen (kephalaion status)."
	cases := []struct {
		status           Status
		sentence, remedy string
	}{
		{Status{Message: "Satz. Abhilfe: tun."}, "Satz.", "tun."},
		{Status{Message: "Satz ohne Abhilfe."}, "Satz ohne Abhilfe.", ""},
		{Status{Message: "Node weg. " + rotate + " " + config, Notes: []string{rotate, config}},
			"Node weg. " + rotate + " " + config, ""},
		{Status{Message: "Wahl fehlt. Abhilfe: Standard setzen. " + config, Notes: []string{config}},
			"Wahl fehlt. " + config, "Standard setzen."},
	}
	for _, c := range cases {
		sentence, remedy := c.status.SplitRemedy()
		if sentence != c.sentence || remedy != c.remedy {
			t.Errorf("%q:\nSatz    %q\nAbhilfe %q", c.status.Message, sentence, remedy)
		}
	}
}

func TestCheckNodeUnreachable(t *testing.T) {
	// Ein Port, auf dem niemand lauscht.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	listener.Close()

	start := time.Now()
	status := checkAccess(context.Background(), goodBinding, testAccess(t, "http://"+addr+"/mcp"))
	if status.State != StateNodeUnreachable {
		t.Fatalf("%+v", status)
	}
	if time.Since(start) > time.Second {
		t.Fatalf("abgewiesene Verbindung dauerte %s", time.Since(start))
	}
}

func TestCheckHangingNodeHitsTimeout(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(func() { close(release); server.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	status := checkAccess(ctx, goodBinding, testAccess(t, server.URL+"/mcp"))
	if status.State != StateNodeUnreachable {
		t.Fatalf("%+v", status)
	}
	if time.Since(start) > DefaultTimeout+time.Second {
		t.Fatalf("Zeitgrenze nicht eingehalten: %s", time.Since(start))
	}
}

func TestCheckDoesNotFollowRedirects(t *testing.T) {
	var redirected bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirected = true }))
	t.Cleanup(target.Close)
	node := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	t.Cleanup(node.Close)
	status := checkAccess(context.Background(), goodBinding, testAccess(t, node.URL+"/mcp"))
	if status.State != StateNodeUnreachable || redirected {
		t.Fatalf("%+v redirected=%v", status, redirected)
	}
}

// TestTokenNeverLeaks schickt ein erkennbares Token und lässt den Node es in
// jeder denkbaren Form zurückspiegeln: im Rumpf einer Fehlerantwort, im Text
// eines Werkzeugfehlers, in einer unlesbaren Antwort. Es darf in keinem
// Ergebnis, keinem Satz und keinem Log auftauchen.
func TestTokenNeverLeaks(t *testing.T) {
	var logs bytes.Buffer
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	replies := []func(h http.Header) (int, string, string){
		func(h http.Header) (int, string, string) {
			return 500, "text/plain", "kaputt: " + h.Get("X-Keph-Token-vm")
		},
		func(h http.Header) (int, string, string) {
			return 200, "application/json", `{"jsonrpc":"2.0","id":2,"result":{"isError":true,"content":[{"type":"text","text":"Token ` + h.Get("X-Keph-Token-vm") + ` abgelehnt"}]}}`
		},
		func(h http.Header) (int, string, string) {
			return 200, "application/json", "kein json " + h.Get("X-Keph-Token-vm")
		},
		func(h http.Header) (int, string, string) {
			return 200, "application/json", `{"jsonrpc":"2.0","id":2,"error":{"code":-32000,"message":"` + h.Get("X-Keph-Token-vm") + `"}}`
		},
		func(h http.Header) (int, string, string) {
			return 200, "application/json", whoamiBody(`{"version":"v0.2.4","hubs":[` + okHub + `],"unknown_hubs":["` + h.Get("X-Keph-Token-vm") + `"]}`)
		},
		func(h http.Header) (int, string, string) {
			return 200, "text/event-stream", sse(whoamiBody(`{"version":"v0.2.4","hubs":[` + okHub + `],"unknown_hubs":[]}`))
		},
	}
	fake := newFakeClient(t)
	for i, reply := range replies {
		node := newFakeNode(t, reply)
		access := testAccess(t, node.server.URL+"/mcp")
		status := checkAccess(context.Background(), goodBinding, access)
		info := infoFor(access.TokensDir, hubWith(access.TokensDir, "vm", "kamran-wsl", ChoiceOnly))
		info.Node.URL = node.server.URL + "/mcp"
		fake.setInfo(t, info)
		_, probes, err := ProbeHubs(context.Background())
		if err != nil {
			t.Fatalf("Antwort %d: %v", i, err)
		}
		data, _ := json.Marshal(struct {
			S Status
			P []HubProbe
		}{status, probes})
		if strings.Contains(string(data), testToken) || strings.Contains(string(data), "TOKENLEAKCANARY") {
			t.Fatalf("Antwort %d: Token im Ergebnis: %s", i, data)
		}
		if len(node.seen) == 0 {
			t.Fatalf("Antwort %d: Node nicht gefragt", i)
		}
	}
	if strings.Contains(logs.String(), "TOKENLEAKCANARY") {
		t.Fatalf("Token im Log: %s", logs.String())
	}
}

func TestEventStreamReplyIsRead(t *testing.T) {
	node := newFakeNode(t, func(http.Header) (int, string, string) {
		return 200, "text/event-stream", sse(whoamiBody(`{"version":"v0.2.4","hubs":[` + okHub + `],"unknown_hubs":[]}`))
	})
	if s := checkAccess(context.Background(), goodBinding, testAccess(t, node.server.URL+"/mcp")); s.State != StateOK {
		t.Fatalf("%+v", s)
	}
}

// ProbeHubs nimmt die Hubs und Accounts allein aus der Auskunft: je Hub mit
// Account die lesbaren Collections, ein Hub ohne gewählten Account mit dem
// Zustand der Auskunft.
func TestProbeHubs(t *testing.T) {
	node := newFakeNode(t, func(h http.Header) (int, string, string) {
		if h.Get("X-Keph-Account-vm") == "kamran-wsl" {
			return 200, "application/json", whoamiBody(`{"version":"v0.2.4","hubs":[` + okHub + `],"unknown_hubs":[]}`)
		}
		return 200, "application/json", whoamiBody(`{"version":"v0.2.4","hubs":[{"hub":"vm","login":"missing","node":"n","sync":{}}],"unknown_hubs":["privat"]}`)
	})
	fake := newFakeClient(t)
	tokens := tokensWith(t)
	writeFile(t, filepath.Join(tokens, "privat", "alice.token"), testToken)
	writeFile(t, filepath.Join(tokens, "zwei", "a.token"), testToken)
	writeFile(t, filepath.Join(tokens, "zwei", "b.token"), testToken)
	info := infoFor(tokens,
		hubWith(tokens, "privat", "alice", ChoiceOnly),
		hubWith(tokens, "vm", "kamran-wsl", ChoiceDefault),
		ClientHub{Alias: "zwei", Choice: ChoiceMissing, Candidates: []string{"a", "b"}, Pending: []string{}},
		ClientHub{Alias: "../x", Choice: ChoiceMissing, Candidates: []string{}, Pending: []string{}})
	info.Node.URL = node.server.URL + "/mcp"
	fake.setInfo(t, info)

	got, probes, err := ProbeHubs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.TokensDir != tokens {
		t.Fatalf("Auskunft: %+v", got)
	}
	if fake.args(t) != "node client --json --format 1 --no-probe" {
		t.Fatalf("Aufruf: %q", fake.args(t))
	}
	if len(probes) != 3 {
		t.Fatalf("%+v", probes)
	}
	if probes[0].Hub != "privat" || probes[0].State != StateHubUnknown {
		t.Fatalf("privat: %+v", probes[0])
	}
	if probes[1].State != StateOK || probes[1].Choice != ChoiceDefault || len(probes[1].Collections) != 2 || probes[1].Collections[1].Address != "vm:test" {
		t.Fatalf("vm: %+v", probes[1])
	}
	if probes[2].State != StateAccountChoiceMissing || !strings.Contains(probes[2].Message, "kephalaion node account default zwei a") {
		t.Fatalf("zwei: %+v", probes[2])
	}
}

func TestProbeHubsStopsWaitingWhenNodeIsDown(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	listener.Close()
	fake := newFakeClient(t)
	tokens := tokensWith(t)
	writeFile(t, filepath.Join(tokens, "privat", "alice.token"), testToken)
	info := infoFor(tokens, hubWith(tokens, "privat", "alice", ChoiceOnly), hubWith(tokens, "vm", "kamran-wsl", ChoiceOnly))
	info.Node.URL = "http://" + addr + "/mcp"
	fake.setInfo(t, info)
	_, probes, err := ProbeHubs(context.Background())
	if err != nil || len(probes) != 2 || probes[0].State != StateNodeUnreachable || probes[1].State != StateNodeUnreachable {
		t.Fatalf("%+v %v", probes, err)
	}

	// Ohne Auskunft gibt es keine Hubs, sondern den Zustand der Auskunft.
	fake.set(t, "", "", 2)
	if _, _, err := ProbeHubs(context.Background()); err == nil || !strings.Contains(err.Error(), "kephalaion upgrade") {
		t.Fatalf("zu altes Binary: %v", err)
	}
}
