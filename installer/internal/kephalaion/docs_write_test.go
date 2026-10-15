package kephalaion

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

// Antworten des gefälschten Nodes auf create und rename.
const (
	createdJSON  = `{"kind":"document","address":"vm:k-playbook","name":"a/001-x.md","id":"d1","revision":4}`
	replayedJSON = `{"address":"vm:k-playbook","name":"a/001-x.md","revision":4,"replayed":true,"ids":["d1"]}`
)

// requestIDPattern ist die Form eines request_id in Kephalaion.
var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

func nodeError(code, message string) (bool, string, string) {
	return true, fmt.Sprintf(`{"address":"vm:k-playbook","name":"a/001-x.md","error":{"code":%q,"message":%q}}`, code, message), message
}

// answerOpenly beantwortet einen Aufruf so, dass sein Ausgang offen bleibt.
type answerOpenly func(w http.ResponseWriter, r *http.Request)

var openAnswers = map[string]answerOpenly{
	"HTTP 500": func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	},
	"unlesbare Antwort": func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":`))
	},
	"abgebrochene Verbindung": func(w http.ResponseWriter, _ *http.Request) {
		conn, _, err := w.(http.Hijacker).Hijack()
		if err == nil {
			conn.Close()
		}
	},
	"ohne strukturierten Inhalt": func(w http.ResponseWriter, _ *http.Request) {
		writeToolResult(w, 3, false, "", "gespeichert")
	},
	"strukturierter Inhalt unlesbar": func(w http.ResponseWriter, _ *http.Request) {
		writeToolResult(w, 3, false, `{"name":17}`, "")
	},
}

// firstOpen beantwortet den ersten Aufruf mit answer, jeden weiteren über reply.
func firstOpen(answer answerOpenly) func(int, toolCall, http.ResponseWriter, *http.Request) bool {
	return func(n int, _ toolCall, w http.ResponseWriter, r *http.Request) bool {
		if n == 1 {
			answer(w, r)
			return true
		}
		return false
	}
}

// sameRequest prüft, dass zwei Aufrufe derselbe Vorgang mit demselben
// Schlüssel sind.
func sameRequest(t *testing.T, calls []toolCall) {
	t.Helper()
	if len(calls) != 2 {
		t.Fatalf("erwartet zwei Aufrufe, bekommen %d", len(calls))
	}
	key, ok := calls[0].Args["request_id"].(string)
	if !ok || key == "" {
		t.Fatalf("kein request_id: %+v", calls[0].Args)
	}
	if !reflect.DeepEqual(calls[0].Args, calls[1].Args) || calls[0].Name != calls[1].Name {
		t.Fatalf("Wiederholung weicht ab: %+v / %+v", calls[0], calls[1])
	}
}

func TestWriteRequestIDJeAufruf(t *testing.T) {
	node := &toolNode{reply: func(toolCall) (bool, string, string) { return false, createdJSON, "" }}
	client := newTestClient(t, startToolNode(t, node).URL+"/mcp")
	ctx := context.Background()
	if _, err := client.Create(ctx, "a/001-x.md", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Create(ctx, "a/002-x.md", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Rename(ctx, "a/001-x.md", "a/done/001-x.md"); err != nil {
		t.Fatal(err)
	}
	calls := node.recorded()
	seen := map[string]bool{}
	for _, call := range calls {
		key, _ := call.Args["request_id"].(string)
		if !requestIDPattern.MatchString(key) || seen[key] {
			t.Fatalf("request_id %q ungültig oder doppelt", key)
		}
		seen[key] = true
	}
	if len(seen) != 3 {
		t.Fatalf("erwartet drei Schlüssel, bekommen %d", len(seen))
	}
}

func TestWriteOffenerAusgangWiederholtMitGleichemSchluessel(t *testing.T) {
	for name, answer := range openAnswers {
		t.Run(name, func(t *testing.T) {
			node := &toolNode{raw: firstOpen(answer),
				reply: func(toolCall) (bool, string, string) { return false, createdJSON, "" }}
			client := newTestClient(t, startToolNode(t, node).URL+"/mcp")
			doc, err := client.Create(context.Background(), "a/001-x.md", "")
			if err != nil || doc.ID != "d1" || doc.Replayed {
				t.Fatalf("Create: %+v %v", doc, err)
			}
			sameRequest(t, node.recorded())
			if node.initCount() != 2 {
				t.Fatalf("Wiederholung nicht in neuer Sitzung: %d initialize", node.initCount())
			}
		})
	}
}

func TestWriteWiederholungReplayedIstErfolg(t *testing.T) {
	node := &toolNode{raw: firstOpen(openAnswers["HTTP 500"]),
		reply: func(call toolCall) (bool, string, string) {
			if call.Name == "rename" {
				// Ohne Namen in der Antwort gilt der gesendete neue Name.
				return false, `{"address":"vm:k-playbook","revision":9,"replayed":true,"ids":["d7"]}`, ""
			}
			return false, replayedJSON, ""
		}}
	client := newTestClient(t, startToolNode(t, node).URL+"/mcp")
	doc, err := client.Create(context.Background(), "a/001-x.md", "")
	if err != nil || !doc.Replayed || doc.ID != "d1" || doc.Revision != 4 || doc.Name != "a/001-x.md" {
		t.Fatalf("Create: %+v %v", doc, err)
	}
	sameRequest(t, node.recorded())

	node.raw = func(n int, call toolCall, w http.ResponseWriter, r *http.Request) bool {
		return firstOpen(openAnswers["unlesbare Antwort"])(n-2, call, w, r)
	}
	doc, err = client.Rename(context.Background(), "a/001-x.md", "a/done/001-x.md")
	if err != nil || !doc.Replayed || doc.ID != "d7" || doc.Name != "a/done/001-x.md" {
		t.Fatalf("Rename: %+v %v", doc, err)
	}
	sameRequest(t, node.recorded()[2:])
}

func TestWriteWiederholungWiederOffenIstOutcomeUnknown(t *testing.T) {
	node := &toolNode{raw: func(_ int, _ toolCall, w http.ResponseWriter, r *http.Request) bool {
		openAnswers["HTTP 500"](w, r)
		return true
	}}
	client := newTestClient(t, startToolNode(t, node).URL+"/mcp")
	_, err := client.Create(context.Background(), "a/001-x.md", "")
	if ErrorCode(err) != CodeOutcomeUnknown || !strings.Contains(err.Error(), "nachsehen") ||
		strings.Contains(err.Error(), testToken) {
		t.Fatalf("erwartet %s: %v", CodeOutcomeUnknown, err)
	}
	sameRequest(t, node.recorded())
}

func TestWriteWiederholungAbgelehntIstAblehnung(t *testing.T) {
	node := &toolNode{raw: firstOpen(openAnswers["HTTP 500"]),
		reply: func(toolCall) (bool, string, string) { return nodeError(CodeNameTaken, "Name belegt") }}
	client := newTestClient(t, startToolNode(t, node).URL+"/mcp")
	_, err := client.Create(context.Background(), "a/001-x.md", "")
	if ErrorCode(err) != CodeNameTaken {
		t.Fatalf("erwartet %s: %v", CodeNameTaken, err)
	}
	sameRequest(t, node.recorded())
}

func TestWriteNodeOutcomeUnknownWiederholt(t *testing.T) {
	node := &toolNode{reply: func() func(toolCall) (bool, string, string) {
		n := 0
		return func(toolCall) (bool, string, string) {
			n++
			if n == 1 {
				return nodeError(CodeOutcomeUnknown, "Ausgang unklar")
			}
			return false, createdJSON, ""
		}
	}()}
	client := newTestClient(t, startToolNode(t, node).URL+"/mcp")
	doc, err := client.Create(context.Background(), "a/001-x.md", "")
	if err != nil || doc.ID != "d1" {
		t.Fatalf("Create: %+v %v", doc, err)
	}
	sameRequest(t, node.recorded())
}

func TestWriteNodeCodesOhneWiederholung(t *testing.T) {
	for _, code := range []string{CodeUnreachable, CodeUnsupported, CodeInternal, CodeNotFound, "forbidden"} {
		node := &toolNode{reply: func(toolCall) (bool, string, string) { return nodeError(code, "nichts gespeichert") }}
		client := newTestClient(t, startToolNode(t, node).URL+"/mcp")
		_, err := client.Rename(context.Background(), "a/001-x.md", "a/done/001-x.md")
		if ErrorCode(err) != code {
			t.Errorf("%s: bekommen %v (%q)", code, err, ErrorCode(err))
		}
		if n := len(node.recorded()); n != 1 {
			t.Errorf("%s: %d Aufrufe, erwartet einer", code, n)
		}
	}
}

func TestWriteOeffnenScheitertIstNodeUnreachable(t *testing.T) {
	node := &toolNode{failInit: func(int) bool { return true },
		reply: func(toolCall) (bool, string, string) { return false, createdJSON, "" }}
	client := newTestClient(t, startToolNode(t, node).URL+"/mcp")
	_, err := client.Create(context.Background(), "a/001-x.md", "")
	if ErrorCode(err) != CodeNodeUnreachable || len(node.recorded()) != 0 || node.initCount() != 1 {
		t.Fatalf("erwartet %s ohne Aufruf: %v, %d Aufrufe, %d initialize", CodeNodeUnreachable, err,
			len(node.recorded()), node.initCount())
	}
}

func TestWriteWiederholungssitzungScheitertIstOutcomeUnknown(t *testing.T) {
	node := &toolNode{failInit: func(n int) bool { return n == 2 }, raw: firstOpen(openAnswers["HTTP 500"]),
		reply: func(toolCall) (bool, string, string) { return false, createdJSON, "" }}
	client := newTestClient(t, startToolNode(t, node).URL+"/mcp")
	_, err := client.Create(context.Background(), "a/001-x.md", "")
	if ErrorCode(err) != CodeOutcomeUnknown || len(node.recorded()) != 1 {
		t.Fatalf("erwartet %s nach einem Aufruf: %v, %d Aufrufe", CodeOutcomeUnknown, err, len(node.recorded()))
	}
}

func shortToolTimeout(t *testing.T, d time.Duration) {
	t.Helper()
	original := toolTimeout
	toolTimeout = d
	t.Cleanup(func() { toolTimeout = original })
}

func TestWriteZeitgrenzeDannWiederholungGelingt(t *testing.T) {
	shortToolTimeout(t, 300*time.Millisecond)
	node := &toolNode{raw: firstOpen(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	}), reply: func(toolCall) (bool, string, string) { return false, createdJSON, "" }}
	client := newTestClient(t, startToolNode(t, node).URL+"/mcp")
	doc, err := client.Create(context.Background(), "a/001-x.md", "")
	if err != nil || doc.ID != "d1" {
		t.Fatalf("Create: %+v %v", doc, err)
	}
	sameRequest(t, node.recorded())
}

func TestWriteAufruferBrichtNachAbschickenAb(t *testing.T) {
	received := make(chan struct{})
	var once sync.Once
	node := &toolNode{raw: func(_ int, _ toolCall, _ http.ResponseWriter, r *http.Request) bool {
		once.Do(func() { close(received) })
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
		return true
	}}
	client := newTestClient(t, startToolNode(t, node).URL+"/mcp")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-received
		cancel()
	}()
	_, err := client.Create(ctx, "a/001-x.md", "")
	if ErrorCode(err) != CodeOutcomeUnknown {
		t.Fatalf("erwartet %s: %v", CodeOutcomeUnknown, err)
	}
	if len(node.recorded()) != 1 || node.initCount() != 1 {
		t.Fatalf("Wiederholung trotz Abbruch: %d Aufrufe, %d initialize", len(node.recorded()), node.initCount())
	}
}

func TestWriteAufruferBrichtVorAbschickenAb(t *testing.T) {
	node := &toolNode{reply: func(toolCall) (bool, string, string) { return false, createdJSON, "" }}
	client := newTestClient(t, startToolNode(t, node).URL+"/mcp")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.Create(ctx, "a/001-x.md", "")
	if ErrorCode(err) != CodeNodeUnreachable || len(node.recorded()) != 0 {
		t.Fatalf("erwartet %s ohne Aufruf: %v", CodeNodeUnreachable, err)
	}
}

func TestWriteAlterNode(t *testing.T) {
	node := &toolNode{version: "v0.3.0", reply: func(toolCall) (bool, string, string) { return false, createdJSON, "" }}
	client := newTestClient(t, startToolNode(t, node).URL+"/mcp")
	_, err := client.Create(context.Background(), "a/001-x.md", "")
	if ErrorCode(err) != CodeNodeUnreachable || !strings.Contains(err.Error(), "Node v0.3.0 zu alt für Schreibvorgänge, mindestens v0.3.1") ||
		!strings.Contains(err.Error(), "kephalaion upgrade") {
		t.Fatalf("Meldung für alten Node: %v", err)
	}
	if len(node.recorded()) != 0 {
		t.Fatalf("alter Node hat create bekommen: %+v", node.recorded())
	}

	// Nicht vergleichbare Versionen gehen durch; die codelose Ablehnung eines
	// alten Nodes bleibt eine Ablehnung, ohne Wiederholung.
	for _, version := range []string{"dev", "v0.3.0-rc1", "v0.10.0"} {
		node := &toolNode{version: version,
			reply: func(toolCall) (bool, string, string) {
				return true, "", "unexpected additional properties [\"request_id\"]"
			}}
		client := newTestClient(t, startToolNode(t, node).URL+"/mcp")
		_, err := client.Create(context.Background(), "a/001-x.md", "")
		if err == nil || ErrorCode(err) != "" || len(node.recorded()) != 1 {
			t.Errorf("%s: %v (%q), %d Aufrufe", version, err, ErrorCode(err), len(node.recorded()))
		}
	}
}

func TestVersionBefore(t *testing.T) {
	cases := map[string]bool{
		"v0.3.0": true, "v0.2.9": true, "v0.3.1": false, "v0.3.2": false, "v0.10.0": false, "v1.0.0": false,
		"dev": false, "v0.3.0-rc1": false, "0.3.0": false, "": false, "v00.3.0": false,
	}
	for version, want := range cases {
		if got := versionBefore(version, MinWriteNodeVersion); got != want {
			t.Errorf("versionBefore(%q) = %v, erwartet %v", version, got, want)
		}
	}
}

func TestClientListMoreOhneCursor(t *testing.T) {
	server, _ := newToolNode(t, func(toolCall) (bool, string, string) {
		return false, `{"entries":[],"more":true}`, ""
	})
	_, err := newTestClient(t, server.URL+"/mcp").List(context.Background(), ListRequest{Path: "x"})
	if ErrorCode(err) != CodeNodeUnreachable || !strings.Contains(err.Error(), "kein cursor") {
		t.Fatalf("more ohne cursor: %v", err)
	}
}

func TestClientListZuVieleSeiten(t *testing.T) {
	server, calls := newToolNode(t, func(toolCall) (bool, string, string) {
		return false, `{"entries":[],"more":true,"cursor":"weiter"}`, ""
	})
	_, err := newTestClient(t, server.URL+"/mcp").List(context.Background(), ListRequest{Path: "x"})
	if ErrorCode(err) != CodeNodeUnreachable || !strings.Contains(err.Error(), fmt.Sprintf("mehr als %d Seiten", maxListPages)) {
		t.Fatalf("zu viele Seiten: %v", err)
	}
	if len(*calls) != maxListPages {
		t.Fatalf("erwartet %d Seiten, bekommen %d", maxListPages, len(*calls))
	}
	for _, call := range *calls {
		if _, ok := call.Args["request_id"]; ok {
			t.Fatal("list trägt einen request_id")
		}
	}
}
