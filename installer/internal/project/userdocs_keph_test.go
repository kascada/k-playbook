package project

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
)

// kephCall ist ein create oder rename, wie der gefälschte Node es bekommt.
type kephCall struct {
	tool      string
	name      string
	requestID string
	content   string
}

// kephNode ist ein gefälschter Node für den echten kephalaion.Client:
// initialize, whoami (v0.3.1), list (leer) und create/rename über answer.
type kephNode struct {
	mu     sync.Mutex
	calls  []kephCall
	answer func(n int, call kephCall, w http.ResponseWriter, id int)
}

func (n *kephNode) recorded() []kephCall {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]kephCall{}, n.calls...)
}

func writeKephResult(w http.ResponseWriter, id int, isError bool, structured string) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":{"isError":%v,"content":[{"type":"text","text":"x"}],"structuredContent":%s}}`,
		id, isError, structured)
}

// newKephUserDocs baut UserDocs über einen echten kephalaion.Client gegen
// den gefälschten Node.
func newKephUserDocs(t *testing.T, node *kephNode) *UserDocs {
	t.Helper()
	docs, err := NewUserDocs(DocKinds, newKephClient(t, node), "kamran")
	if err != nil {
		t.Fatal(err)
	}
	return docs
}

// newKephClient baut einen echten kephalaion.Client gegen den gefälschten
// Node.
func newKephClient(t *testing.T, node *kephNode) *kephalaion.Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		var msg struct {
			ID     *int   `json:"id"`
			Method string `json:"method"`
			Params struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			} `json:"params"`
		}
		_ = json.Unmarshal(data, &msg)
		switch msg.Method {
		case "initialize":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":{"protocolVersion":"2025-06-18","capabilities":{}}}`, *msg.ID)
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
		case "tools/call":
			switch msg.Params.Name {
			case "whoami":
				writeKephResult(w, *msg.ID, false, `{"version":"v0.3.1","hubs":[],"unknown_hubs":[]}`)
			case "list":
				writeKephResult(w, *msg.ID, false, `{"entries":[],"more":false}`)
			default:
				call := kephCall{tool: msg.Params.Name}
				call.name, _ = msg.Params.Arguments["name"].(string)
				if newName, ok := msg.Params.Arguments["new_name"].(string); ok {
					call.name = newName
				}
				call.requestID, _ = msg.Params.Arguments["request_id"].(string)
				call.content, _ = msg.Params.Arguments["content"].(string)
				node.mu.Lock()
				node.calls = append(node.calls, call)
				n := len(node.calls)
				node.mu.Unlock()
				node.answer(n, call, w, *msg.ID)
			}
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	t.Cleanup(server.Close)

	dir := t.TempDir()
	for name, content := range map[string]string{
		"config.yaml":                "node:\n  db: sqlite:///x\n",
		"tokens/vm/kamran-wsl.token": "keph_USERDOCSLEAKCANARYabcdefghijklmnopqrstu1234\n",
	} {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	env := kephalaion.LoadEnv(dir)
	env.Endpoint = server.URL + "/mcp"
	client, err := kephalaion.NewClient(kephalaion.Binding{Hub: "vm", Collection: "k-playbook", Account: "kamran-wsl"}, env)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client
}

func kephOK(w http.ResponseWriter, id int, name string) {
	writeKephResult(w, id, false, fmt.Sprintf(`{"kind":"document","address":"vm:k-playbook","name":%q,"id":"d1","revision":3}`, name))
}

func kephNameTaken(w http.ResponseWriter, id int, name string) {
	writeKephResult(w, id, true, fmt.Sprintf(`{"address":"vm:k-playbook","name":%q,"error":{"code":"name_taken","message":"belegt"}}`, name))
}

func TestDocCreateNeuerSchluesselJeNummer(t *testing.T) {
	// 100 ist belegt, 101 wird angelegt: zwei Aufrufe, zwei Schlüssel.
	node := &kephNode{answer: func(n int, call kephCall, w http.ResponseWriter, id int) {
		if n == 1 {
			kephNameTaken(w, id, call.name)
			return
		}
		kephOK(w, id, call.name)
	}}
	created, err := newKephUserDocs(t, node).Create(context.Background(), "tasks", "a")
	if err != nil || created.Number != 101 {
		t.Fatalf("Create: %+v %v", created, err)
	}
	calls := node.recorded()
	if len(calls) != 2 || calls[0].requestID == "" || calls[0].requestID == calls[1].requestID {
		t.Fatalf("erwartet je Nummer einen eigenen Schlüssel: %+v", calls)
	}
}

func TestDocCreateOffenDannNameTakenZiehtNaechsteNummer(t *testing.T) {
	// Der erste Aufruf für 100 bleibt offen, die Wiederholung mit demselben
	// Schlüssel klärt ihn als name_taken (ein anderer hatte die Nummer). Erst
	// dann kommt 101, mit neuem Schlüssel.
	node := &kephNode{answer: func(n int, call kephCall, w http.ResponseWriter, id int) {
		switch n {
		case 1:
			w.WriteHeader(http.StatusBadGateway)
		case 2:
			kephNameTaken(w, id, call.name)
		default:
			kephOK(w, id, call.name)
		}
	}}
	created, err := newKephUserDocs(t, node).Create(context.Background(), "tasks", "a")
	if err != nil || created.Number != 101 {
		t.Fatalf("Create: %+v %v", created, err)
	}
	calls := node.recorded()
	if len(calls) != 3 || calls[0] != calls[1] || calls[2].requestID == calls[0].requestID ||
		calls[2].name != "user/kamran/tasks/101-a.md" {
		t.Fatalf("Aufrufe: %+v", calls)
	}
}

func TestDocCreateBleibendOffenIstOutcomeUnknown(t *testing.T) {
	node := &kephNode{answer: func(_ int, _ kephCall, w http.ResponseWriter, _ int) {
		w.WriteHeader(http.StatusInternalServerError)
	}}
	_, err := newKephUserDocs(t, node).Create(context.Background(), "tasks", "a")
	wantCode(t, err, DocCodeOutcomeUnknown)
	calls := node.recorded()
	if len(calls) != 2 || calls[0] != calls[1] {
		t.Fatalf("erwartet genau eine Wiederholung mit demselben Schlüssel, ohne nächste Nummer: %+v", calls)
	}
}

func TestDocDoneWiederholungReplayed(t *testing.T) {
	node := &kephNode{answer: func(n int, call kephCall, w http.ResponseWriter, id int) {
		if n == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		writeKephResult(w, id, false, fmt.Sprintf(`{"address":"vm:k-playbook","name":%q,"revision":4,"replayed":true,"ids":["d1"]}`, call.name))
	}}
	moved, err := newKephUserDocs(t, node).Done(context.Background(), "keph:/user/~/tasks/100-a.md")
	if err != nil || moved.Ref != "keph:/user/kamran/tasks/done/100-a.md" {
		t.Fatalf("Done: %+v %v", moved, err)
	}
	calls := node.recorded()
	if len(calls) != 2 || calls[0] != calls[1] || calls[0].requestID == "" {
		t.Fatalf("Aufrufe: %+v", calls)
	}
}
