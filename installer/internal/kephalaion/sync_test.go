package kephalaion

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

// Die Tests von SyncHub laufen gegen einen Test-Node (toolNode aus
// docs_test.go), ohne Binary und ohne node.db: so, wie ein reiner Client den
// Node seines Hosts erreicht.

// newSyncNode startet einen Test-Node mit der Version version, der sync mit
// reply beantwortet.
func newSyncNode(t *testing.T, version string, reply func(call toolCall) (bool, string, string)) (*toolNode, *Client) {
	t.Helper()
	node := &toolNode{version: version, reply: reply}
	server := startToolNode(t, node)
	return node, newTestClient(t, server.URL+"/mcp")
}

// syncFailure beantwortet sync mit isError und error.code.
func syncFailure(code, message string) func(toolCall) (bool, string, string) {
	return func(toolCall) (bool, string, string) {
		messageJSON, _ := json.Marshal(message)
		return true, fmt.Sprintf(`{"hub":"vm","error":{"code":%q,"message":%s}}`, code, messageJSON), message
	}
}

func syncErrorOf(t *testing.T, err error) *SyncError {
	t.Helper()
	var syncErr *SyncError
	if !errors.As(err, &syncErr) {
		t.Fatalf("erwartet *SyncError, bekommen %T: %v", err, err)
	}
	return syncErr
}

func TestSyncHubErfolg(t *testing.T) {
	node, client := newSyncNode(t, MinSyncNodeVersion, func(toolCall) (bool, string, string) {
		return false, `{"hub":"vm","sync":{"last_success":"2026-11-24T16:05:12Z","revision":5411}}`, "Hub vm: abgeglichen"
	})
	state, err := client.SyncHub(context.Background())
	if err != nil {
		t.Fatalf("Abgleich: %v", err)
	}
	if state.LastSuccess != "2026-11-24T16:05:12Z" || state.Revision != 5411 {
		t.Fatalf("Stand: %+v", state)
	}
	calls := node.recorded()
	if len(calls) != 1 || calls[0].Name != "sync" || calls[0].Args["hub"] != "vm" || len(calls[0].Args) != 1 {
		t.Fatalf("Aufruf: %+v", calls)
	}
	if calls[0].Header.Get("X-Keph-Account-vm") != "kamran-wsl" || calls[0].Header.Get("X-Keph-Token-vm") != testToken {
		t.Fatalf("Header-Paar fehlt: %v", calls[0].Header)
	}
}

func TestSyncHubRevisionNullIstErfolg(t *testing.T) {
	_, client := newSyncNode(t, "v0.5.1", func(toolCall) (bool, string, string) {
		return false, `{"hub":"vm","sync":{"last_success":"2026-11-24T16:05:12Z","revision":0}}`, ""
	})
	state, err := client.SyncHub(context.Background())
	if err != nil || state.Revision != 0 {
		t.Fatalf("revision 0: %+v %v", state, err)
	}
}

func TestSyncHubErfolgOhneRevision(t *testing.T) {
	_, client := newSyncNode(t, MinSyncNodeVersion, func(toolCall) (bool, string, string) {
		return false, `{"hub":"vm","sync":{"last_success":"2026-11-24T16:05:12Z"}}`, ""
	})
	_, err := client.SyncHub(context.Background())
	if syncErr := syncErrorOf(t, err); !strings.Contains(syncErr.Message, "ohne Revision") {
		t.Fatalf("ohne revision: %v", err)
	}
}

func TestSyncHubErfolgMitLastError(t *testing.T) {
	_, client := newSyncNode(t, MinSyncNodeVersion, func(toolCall) (bool, string, string) {
		return false, `{"hub":"vm","sync":{"revision":7,"last_error":"Hub vm: nicht erreichbar."}}`, ""
	})
	_, err := client.SyncHub(context.Background())
	if syncErr := syncErrorOf(t, err); !strings.Contains(syncErr.Message, "mit einem Fehler: Hub vm: nicht erreichbar. Abhilfe") {
		t.Fatalf("mit last_error: %v", err)
	}
}

func TestSyncHubNodeZuAlt(t *testing.T) {
	node, client := newSyncNode(t, "v0.4.3", func(toolCall) (bool, string, string) {
		t.Error("sync an einem zu alten Node aufgerufen")
		return true, "", ""
	})
	_, err := client.SyncHub(context.Background())
	syncErr := syncErrorOf(t, err)
	for _, want := range []string{"v0.4.3", MinSyncNodeVersion, "kephalaion upgrade am Rechner des Nodes"} {
		if !strings.Contains(syncErr.Message, want) {
			t.Fatalf("zu alt, %q fehlt: %v", want, err)
		}
	}
	if len(node.recorded()) != 0 {
		t.Fatal("Aufruf trotz zu altem Node")
	}
}

// Ein dev-Build gilt als nicht zu alt; kennt er sync nicht, antwortet sein
// SDK mit -32602 — das ist ein gescheiterter Abgleich, nicht „zu alt“.
func TestSyncHubDevVersionOhneWerkzeug(t *testing.T) {
	node := &toolNode{version: "dev", raw: func(_ int, _ toolCall, w http.ResponseWriter, r *http.Request) bool {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"jsonrpc":"2.0","id":3,"error":{"code":-32602,"message":"unknown tool \"sync\""}}`)
		return true
	}}
	server := startToolNode(t, node)
	client := newTestClient(t, server.URL+"/mcp")
	_, err := client.SyncHub(context.Background())
	syncErr := syncErrorOf(t, err)
	if !strings.Contains(syncErr.Message, "nicht bestätigt") || !strings.Contains(syncErr.Message, "-32602") ||
		strings.Contains(syncErr.Message, "kephalaion upgrade") {
		t.Fatalf("dev mit -32602: %v", err)
	}
}

func TestSyncHubFehlercodes(t *testing.T) {
	general := "kephalaion node sync vm von Hand"
	cases := []struct {
		code   string
		remedy string
	}{
		{CodeUnreachable, "Hub und Netz prüfen"},
		{CodeUnsupported, "Hub-Eintrag am Rechner des Nodes"},
		{CodeSyncTimeout, "Wartefrist des Nodes (40 s)"},
		{CodeUnknownHub, "Hub der Projektbindung, Account und Token prüfen"},
		{CodeNotReadable, "Hub der Projektbindung, Account und Token prüfen"},
		{CodeUnauthenticated, "Abhilfe am Rechner des Nodes, nicht hier: kephalaion node hub check vm"},
		{CodeNeverSynced, "Abhilfe am Rechner des Nodes, nicht hier: kephalaion node sync vm"},
		{CodeInternal, general},
		{CodeInvalid, general},
		{CodeHubRefused, general},
		{CodeUnsupportedVersion, general},
		{"etwas_neues", general},
	}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			_, client := newSyncNode(t, MinSyncNodeVersion, syncFailure(tc.code, "Hub vm: Meldung des Nodes zu "+tc.code+"."))
			_, err := client.SyncHub(context.Background())
			syncErr := syncErrorOf(t, err)
			if syncErr.Code != tc.code {
				t.Fatalf("Code %q, erwartet %q", syncErr.Code, tc.code)
			}
			if !strings.Contains(syncErr.Message, "Hub vm: Meldung des Nodes zu "+tc.code+". ") ||
				strings.Contains(syncErr.Message, "..") {
				t.Fatalf("Meldung des Nodes nicht wörtlich: %v", err)
			}
			if !strings.Contains(syncErr.Message, tc.remedy) {
				t.Fatalf("Abhilfe %q fehlt: %v", tc.remedy, err)
			}
			if tc.remedy != general && strings.Contains(syncErr.Message, general) {
				t.Fatalf("allgemeine Abhilfe statt der eigenen: %v", err)
			}
		})
	}
}

// unknown_hub und not_readable sind derselbe Fall mit derselben Abhilfe.
func TestSyncHubUnbekannterHubWieNichtLesbar(t *testing.T) {
	remedy := func(code string) string {
		_, client := newSyncNode(t, MinSyncNodeVersion, syncFailure(code, "M"))
		_, err := client.SyncHub(context.Background())
		_, after, _ := strings.Cut(syncErrorOf(t, err).Message, "Abhilfe: ")
		return after
	}
	if a, b := remedy(CodeUnknownHub), remedy(CodeNotReadable); a != b || a == "" {
		t.Fatalf("verschiedene Abhilfen: %q / %q", a, b)
	}
}

func TestSyncHubFehlerOhneCode(t *testing.T) {
	_, client := newSyncNode(t, MinSyncNodeVersion, func(toolCall) (bool, string, string) {
		return true, "", "Argumente abgelehnt"
	})
	_, err := client.SyncHub(context.Background())
	syncErr := syncErrorOf(t, err)
	if syncErr.Code != "" || !strings.Contains(syncErr.Message, "gescheitert: Argumente abgelehnt.") ||
		!strings.Contains(syncErr.Message, "kephalaion node sync vm von Hand") {
		t.Fatalf("ohne Code: %v", err)
	}
}

func TestSyncHubUnlesbareAntwort(t *testing.T) {
	for name, structured := range map[string]string{
		"ohne sync":   `{"hub":"vm"}`,
		"falsche Art": `{"hub":"vm","sync":"gestern"}`,
		"leer":        "",
	} {
		t.Run(name, func(t *testing.T) {
			_, client := newSyncNode(t, MinSyncNodeVersion, func(toolCall) (bool, string, string) {
				return false, structured, "Hub vm: abgeglichen"
			})
			_, err := client.SyncHub(context.Background())
			if syncErr := syncErrorOf(t, err); !strings.Contains(syncErr.Message, "nicht lesbar") {
				t.Fatalf("%s: %v", name, err)
			}
		})
	}
	node := &toolNode{version: MinSyncNodeVersion, raw: func(_ int, _ toolCall, w http.ResponseWriter, _ *http.Request) bool {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, "kein JSON")
		return true
	}}
	client := newTestClient(t, startToolNode(t, node).URL+"/mcp")
	_, err := client.SyncHub(context.Background())
	if syncErr := syncErrorOf(t, err); !strings.Contains(syncErr.Message, "nicht bestätigt") {
		t.Fatalf("kein JSON: %v", err)
	}
}

func TestSyncHubEigeneZeitgrenze(t *testing.T) {
	original := SyncTimeout
	SyncTimeout = 200 * time.Millisecond
	t.Cleanup(func() { SyncTimeout = original })
	node := &toolNode{version: MinSyncNodeVersion, raw: func(_ int, _ toolCall, _ http.ResponseWriter, r *http.Request) bool {
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
		return true
	}}
	client := newTestClient(t, startToolNode(t, node).URL+"/mcp")
	_, err := client.SyncHub(context.Background())
	if syncErr := syncErrorOf(t, err); !strings.Contains(syncErr.Message, "nicht innerhalb von 200ms geendet") {
		t.Fatalf("Zeitgrenze: %v", err)
	}
}

func TestSyncHubAbbruch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	node := &toolNode{version: MinSyncNodeVersion, raw: func(_ int, _ toolCall, _ http.ResponseWriter, r *http.Request) bool {
		cancel()
		<-r.Context().Done()
		return true
	}}
	client := newTestClient(t, startToolNode(t, node).URL+"/mcp")
	_, err := client.SyncHub(ctx)
	if syncErr := syncErrorOf(t, err); !strings.Contains(syncErr.Message, "abgebrochen") {
		t.Fatalf("Abbruch: %v", err)
	}
}

func TestSyncHubNodeNichtErreichbar(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	endpoint := "http://" + listener.Addr().String() + "/mcp"
	listener.Close()
	client := newTestClient(t, endpoint)
	_, err = client.SyncHub(context.Background())
	syncErr := syncErrorOf(t, err)
	if !strings.Contains(syncErr.Message, "nicht erreichbar") || !strings.Contains(syncErr.Message, "kephalaion serve") {
		t.Fatalf("Node nicht erreichbar: %v", err)
	}
}

func TestSyncHubUngueltigerHub(t *testing.T) {
	node := &toolNode{version: MinSyncNodeVersion}
	server := startToolNode(t, node)
	client := &Client{endpoint: server.URL + "/mcp", hub: "--config", account: "kamran-wsl", token: testToken}
	_, err := client.SyncHub(context.Background())
	syncErrorOf(t, err)
	if node.initCount() != 0 {
		t.Fatal("Node trotz ungültigem Hub gefragt")
	}
}

func TestSyncHubKeinTokenImFehler(t *testing.T) {
	_, client := newSyncNode(t, MinSyncNodeVersion, syncFailure(CodeInternal, "Fehler mit "+testToken+" darin"))
	_, err := client.SyncHub(context.Background())
	if syncErr := syncErrorOf(t, err); strings.Contains(syncErr.Message, testToken) || !strings.Contains(syncErr.Message, "***") {
		t.Fatalf("Token im Fehler: %v", err)
	}
}
