package kephalaion

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// toolCall ist ein empfangener tools/call: Werkzeug, Argumente, Header.
type toolCall struct {
	Name   string
	Args   map[string]any
	Header http.Header
}

// newToolNode ist ein gefälschter Node wie newFakeNode, der aber den Rumpf
// von tools/call liest: reply bekommt Werkzeug und Argumente und liefert
// isError und structuredContent.
func newToolNode(t *testing.T, reply func(call toolCall) (isError bool, structured string, text string)) (*httptest.Server, *[]toolCall) {
	t.Helper()
	var calls []toolCall
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
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":{"protocolVersion":"2025-06-18","capabilities":{"tools":{}},"serverInfo":{"name":"kephalaion","version":"v0.3.0"}}}`, *msg.ID)
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
		case "tools/call":
			call := toolCall{Name: msg.Params.Name, Args: msg.Params.Arguments, Header: r.Header.Clone()}
			calls = append(calls, call)
			isError, structured, text := reply(call)
			textJSON, _ := json.Marshal(text)
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":{"isError":%v,"content":[{"type":"text","text":%s}]`, *msg.ID, isError, textJSON)
			if structured != "" {
				fmt.Fprintf(w, `,"structuredContent":%s`, structured)
			}
			io.WriteString(w, "}}")
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	t.Cleanup(server.Close)
	return server, &calls
}

func newTestClient(t *testing.T, endpoint string) *Client {
	t.Helper()
	client, err := NewClient(goodBinding, testEnv(t, endpoint))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client
}

func TestClientListBlaettertUndUebergehtVerzeichnisse(t *testing.T) {
	server, calls := newToolNode(t, func(call toolCall) (bool, string, string) {
		if call.Args["cursor"] == nil {
			return false, `{"entries":[{"kind":"directory","name":"user/kamran/tasks/done"},
				{"kind":"document","name":"user/kamran/tasks/100-a.md","id":"d1","revision":3,
				 "created":{"at":"2026-10-14T10:00:00.000Z","by":"kamran"},"updated":{"at":"2026-10-14T10:00:00.000Z","by":"kamran"},
				 "frontmatter":{"title":"A"}}],"more":true,"cursor":"c1"}`, ""
		}
		return false, `{"entries":[{"kind":"document","name":"user/kamran/tasks/done/099-b.md"}],"more":false}`, ""
	})
	client := newTestClient(t, server.URL+"/mcp")

	entries, err := client.List(context.Background(), ListRequest{Path: "user", Recursive: true, Frontmatter: true})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 2 || entries[0].Name != "user/kamran/tasks/100-a.md" || entries[1].Name != "user/kamran/tasks/done/099-b.md" {
		t.Fatalf("Einträge: %+v", entries)
	}
	if entries[0].Created == nil || entries[0].Created.By != "kamran" || entries[0].Frontmatter["title"] != "A" {
		t.Fatalf("Angaben: %+v", entries[0])
	}
	if len(*calls) != 2 {
		t.Fatalf("erwartet zwei Seiten, bekommen %d Aufrufe", len(*calls))
	}
	first := (*calls)[0]
	if first.Name != "list" || first.Args["collection"] != "vm:k-playbook" || first.Args["path"] != "user" ||
		first.Args["recursive"] != true || first.Args["frontmatter"] != true {
		t.Fatalf("Argumente: %+v", first.Args)
	}
	if (*calls)[1].Args["cursor"] != "c1" {
		t.Fatalf("cursor nicht weitergegeben: %+v", (*calls)[1].Args)
	}
	if first.Header.Get("X-Keph-Account-vm") != "kamran-wsl" || first.Header.Get("X-Keph-Token-vm") != testToken {
		t.Fatalf("Header-Paar fehlt: %v", first.Header)
	}
}

func TestClientCreateUndRename(t *testing.T) {
	server, calls := newToolNode(t, func(call toolCall) (bool, string, string) {
		switch call.Name {
		case "create":
			return false, `{"kind":"document","address":"vm:k-playbook","name":"` + call.Args["name"].(string) + `","id":"d9","revision":7,
				"created":{"at":"2026-10-14T10:00:00.000Z","by":"kamran"}}`, ""
		case "rename":
			return false, `{"kind":"document","address":"vm:k-playbook","name":"` + call.Args["new_name"].(string) + `","id":"d9","revision":8}`, ""
		}
		return true, "", "unbekannt"
	})
	client := newTestClient(t, server.URL+"/mcp")

	doc, err := client.Create(context.Background(), "queue/findings/001-x.md", "")
	if err != nil || doc.Name != "queue/findings/001-x.md" || doc.ID != "d9" || doc.Created.By != "kamran" {
		t.Fatalf("Create: %+v %v", doc, err)
	}
	if args := (*calls)[0].Args; args["content"] != "" || args["collection"] != "vm:k-playbook" {
		t.Fatalf("Argumente create: %+v", args)
	}
	doc, err = client.Rename(context.Background(), "a/001-x.md", "a/done/001-x.md")
	if err != nil || doc.Name != "a/done/001-x.md" || doc.Revision != 8 {
		t.Fatalf("Rename: %+v %v", doc, err)
	}
	if args := (*calls)[1].Args; args["name"] != "a/001-x.md" || args["new_name"] != "a/done/001-x.md" {
		t.Fatalf("Argumente rename: %+v", args)
	}
}

func TestClientFehlercodesSindUnterscheidbar(t *testing.T) {
	server, _ := newToolNode(t, func(call toolCall) (bool, string, string) {
		switch call.Name {
		case "create":
			return true, `{"address":"vm:k-playbook","name":"x","error":{"code":"name_taken","message":"Name belegt"}}`, "Name belegt"
		case "rename":
			return true, `{"address":"vm:k-playbook","name":"x","error":{"code":"not_found","message":"gibt es nicht"}}`, "gibt es nicht"
		case "list":
			// list meldet Fehler nur als Text, ohne Code.
			return true, "", "path: ungültig"
		}
		return true, "", "?"
	})
	client := newTestClient(t, server.URL+"/mcp")

	_, err := client.Create(context.Background(), "x", "")
	if ErrorCode(err) != CodeNameTaken || !strings.Contains(err.Error(), "Name belegt") {
		t.Fatalf("create: %v (%q)", err, ErrorCode(err))
	}
	_, err = client.Rename(context.Background(), "x", "y")
	if ErrorCode(err) != CodeNotFound {
		t.Fatalf("rename: %v", err)
	}
	_, err = client.List(context.Background(), ListRequest{Path: "x"})
	var toolErr *ToolError
	if err == nil || ErrorCode(err) != "" || !strings.Contains(err.Error(), "path: ungültig") {
		t.Fatalf("list: %v", err)
	}
	if !errors.As(err, &toolErr) || toolErr.Tool != "list" {
		t.Fatalf("kein ToolError: %#v", err)
	}
}

func TestClientNodeNichtErreichbar(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	listener.Close()
	client := newTestClient(t, "http://"+addr+"/mcp")

	_, err = client.Create(context.Background(), "x", "")
	if ErrorCode(err) != CodeNodeUnreachable {
		t.Fatalf("erwartet %s, bekommen %v", CodeNodeUnreachable, err)
	}
}

func TestNewClientOhneTokenOderAdresse(t *testing.T) {
	env := testEnv(t, "http://127.0.0.1:1/mcp")
	if _, err := NewClient(Binding{Hub: "vm", Collection: "k-playbook", Account: "fremd"}, env); ErrorCode(err) != CodeNodeUnreachable {
		t.Fatalf("ohne Token: %v", err)
	}
	env.Endpoint = ""
	if _, err := NewClient(goodBinding, env); ErrorCode(err) != CodeNodeUnreachable {
		t.Fatalf("ohne Adresse: %v", err)
	}
	if _, err := NewClient(Binding{Hub: "../vm", Collection: "k", Account: "a"}, env); err == nil {
		t.Fatal("ungültige Anbindung angenommen")
	}
}

func TestClientFehlerOhneToken(t *testing.T) {
	server, _ := newToolNode(t, func(call toolCall) (bool, string, string) {
		token := call.Header.Get("X-Keph-Token-vm")
		return true, `{"error":{"code":"forbidden","message":"Token ` + token + ` abgelehnt"}}`, "Token " + token
	})
	client := newTestClient(t, server.URL+"/mcp")
	_, err := client.Create(context.Background(), "x", "")
	if err == nil || strings.Contains(err.Error(), testToken) {
		t.Fatalf("Token im Fehler: %v", err)
	}
}
