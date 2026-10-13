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

// testEnv legt eine config mit dem Endpunkt des Nodes und eine Token-Datei
// für vm/kamran-wsl an.
func testEnv(t *testing.T, endpoint string) Env {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "config.yaml"), "node:\n  db: sqlite:///x\n")
	writeFile(t, filepath.Join(dir, "tokens", "vm", "kamran-wsl.token"), testToken+"\n")
	env := LoadEnv(dir)
	env.Endpoint = endpoint
	return env
}

var goodBinding = Binding{Hub: "vm", Collection: "k-playbook", Account: "kamran-wsl"}

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
		{"collection", Binding{Hub: "vm", Collection: "fremd", Account: "kamran-wsl"},
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
			status := CheckBinding(context.Background(), c.binding, testEnv(t, node.server.URL+"/mcp"))
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
	env := testEnv(t, node.server.URL+"/mcp")

	status := CheckBinding(context.Background(), goodBinding, env)
	if status.Address != "vm:k-playbook" || status.User != "kamran" {
		t.Fatalf("%+v", status)
	}
	if strings.Join(status.Rights, ",") != "read,write" || len(status.Scopes) != 0 {
		t.Fatalf("Rechte %v Scopes %v", status.Rights, status.Scopes)
	}
	if status.Sync == nil || status.Sync.Revision == nil || *status.Sync.Revision != 41 || status.Sync.LastSuccess == "" {
		t.Fatalf("Abgleich: %+v", status.Sync)
	}

	withScopes := CheckBinding(context.Background(), Binding{Hub: "vm", Collection: "test", Account: "kamran-wsl"}, env)
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
	local := t.TempDir()
	env := Env{ConfigDir: t.TempDir()}
	if s := Check(context.Background(), local, env); s.State != StateNotConfigured || s.Configured {
		t.Fatalf("nicht eingetragen: %+v", s)
	}

	writeFile(t, LocalFilePath(local), "kephalaion:\n  hub: ../vm\n  collection: k-playbook\n  account: kamran-wsl\n")
	if s := Check(context.Background(), local, env); s.State != StateInvalidEntry || s.Address != "" {
		t.Fatalf("ungültig: %+v", s)
	}

	writeFile(t, LocalFilePath(local), "kephalaion:\n  hub: vm\n  collection: k-playbook\n  account: kamran-wsl\n")
	if s := Check(context.Background(), local, env); s.State != StateNoKephalaion {
		t.Fatalf("keine config: %+v", s)
	}

	writeFile(t, filepath.Join(env.ConfigDir, "config.yaml"), "node:\n  listen: 10.1.2.3:7433\n")
	if s := Check(context.Background(), local, LoadEnv(env.ConfigDir)); s.State != StateNodeAddress {
		t.Fatalf("fremde Adresse: %+v", s)
	}
	writeFile(t, filepath.Join(env.ConfigDir, "config.yaml"), "hub:\n  db: x\n")
	if s := Check(context.Background(), local, LoadEnv(env.ConfigDir)); s.State != StateNodeAddress {
		t.Fatalf("kein Node: %+v", s)
	}

	writeFile(t, filepath.Join(env.ConfigDir, "config.yaml"), "node:\n  listen: 127.0.0.1:7433\n")
	if s := Check(context.Background(), local, LoadEnv(env.ConfigDir)); s.State != StateTokenMissing {
		t.Fatalf("ohne Token: %+v", s)
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
	status := CheckBinding(context.Background(), goodBinding, testEnv(t, "http://"+addr+"/mcp"))
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
	status := CheckBinding(ctx, goodBinding, testEnv(t, server.URL+"/mcp"))
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
	status := CheckBinding(context.Background(), goodBinding, testEnv(t, node.URL+"/mcp"))
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
	for i, reply := range replies {
		node := newFakeNode(t, reply)
		env := testEnv(t, node.server.URL+"/mcp")
		status := CheckBinding(context.Background(), goodBinding, env)
		pairs := ProbePairs(context.Background(), env)
		data, _ := json.Marshal(struct {
			S Status
			P []Pair
		}{status, pairs})
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
	if s := CheckBinding(context.Background(), goodBinding, testEnv(t, node.server.URL+"/mcp")); s.State != StateOK {
		t.Fatalf("%+v", s)
	}
}

func TestProbePairs(t *testing.T) {
	node := newFakeNode(t, func(h http.Header) (int, string, string) {
		if h.Get("X-Keph-Account-vm") == "kamran-wsl" {
			return 200, "application/json", whoamiBody(`{"version":"v0.2.4","hubs":[` + okHub + `],"unknown_hubs":[]}`)
		}
		return 200, "application/json", whoamiBody(`{"version":"v0.2.4","hubs":[{"hub":"vm","login":"missing","node":"n","sync":{}}],"unknown_hubs":["privat"]}`)
	})
	env := testEnv(t, node.server.URL+"/mcp")
	writeFile(t, filepath.Join(env.ConfigDir, "tokens", "privat", "alice.token"), testToken)

	pairs := ProbePairs(context.Background(), env)
	if len(pairs) != 2 {
		t.Fatalf("%+v", pairs)
	}
	if pairs[0].Hub != "privat" || pairs[0].State != StateHubUnknown {
		t.Fatalf("privat: %+v", pairs[0])
	}
	if pairs[1].State != StateOK || len(pairs[1].Collections) != 2 || pairs[1].Collections[1].Address != "vm:test" {
		t.Fatalf("vm: %+v", pairs[1])
	}
}

func TestProbePairsStopsWaitingWhenNodeIsDown(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	listener.Close()
	env := testEnv(t, "http://"+addr+"/mcp")
	writeFile(t, filepath.Join(env.ConfigDir, "tokens", "privat", "alice.token"), testToken)
	pairs := ProbePairs(context.Background(), env)
	if len(pairs) != 2 || pairs[0].State != StateNodeUnreachable || pairs[1].State != StateNodeUnreachable {
		t.Fatalf("%+v", pairs)
	}
}
