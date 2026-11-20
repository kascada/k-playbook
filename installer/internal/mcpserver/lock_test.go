package mcpserver

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kascada/k-playbook/installer/internal/project"
	"github.com/kascada/k-playbook/installer/internal/worklock"
)

// lockSession verbindet einen Client mit einem Server, der nur das
// Sperr-Werkzeug trägt — derselbe Weg wie über stdio, mit nebenläufigen
// Aufrufen.
func lockSession(t *testing.T, clientName string) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	server := mcp.NewServer(&mcp.Implementation{Name: "server", Version: "v0.0.1"}, nil)
	addLockTool(server)
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("Server verbinden: %v", err)
	}
	t.Cleanup(func() { serverSession.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: clientName, Version: "v0.0.1"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("Client verbinden: %v", err)
	}
	t.Cleanup(func() { clientSession.Close() })
	return clientSession
}

// noHolder schaltet die Suche nach dem Client-Prozess ab: Im Test läuft der
// Server unter `go test`, nicht unter einem Client.
func noHolder(t *testing.T) {
	t.Helper()
	before := lockFindHolder
	lockFindHolder = func() *worklock.Process { return nil }
	t.Cleanup(func() { lockFindHolder = before })
}

type lockAnswer struct {
	OK     bool   `json:"ok"`
	Action string `json:"action"`
	worklock.Result
	Error *errorEnvelope `json:"error"`
}

func callLock(t *testing.T, session *mcp.ClientSession, args map[string]any) lockAnswer {
	t.Helper()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: lockToolName, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("Inhalt ist kein Text: %#v", result.Content[0])
	}
	var answer lockAnswer
	if err := json.Unmarshal([]byte(text.Text), &answer); err != nil {
		t.Fatalf("Antwort ist kein JSON (%q): %v", text.Text, err)
	}
	return answer
}

func newLockProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := project.CreateConfig(root, "."); err != nil {
		t.Fatalf("Konfiguration anlegen: %v", err)
	}
	return root
}

func TestLockToolAcquireCheckRelease(t *testing.T) {
	noHolder(t)
	root := newLockProject(t)
	session := lockSession(t, "claude-code")

	taken := callLock(t, session, map[string]any{"projectDir": root, "action": "acquire", "purpose": "Task 171", "session": "s-1"})
	if !taken.OK || taken.State != worklock.StateAcquired || taken.Token == "" {
		t.Fatalf("acquire: %+v", taken)
	}
	status := callLock(t, session, map[string]any{"projectDir": root, "action": "status"})
	if status.State != worklock.StateBusy || status.Holder == nil || status.Holder.Client != "claude-code" ||
		status.Holder.Purpose != "Task 171" || status.Holder.Session != "s-1" {
		t.Fatalf("status: %+v", status)
	}
	// Ohne Halter-Prozess (wie von der CLI) nie sicher verwaist.
	if status.Holder.Orphaned != worklock.OrphanedUncertain {
		t.Fatalf("ohne Halter-Prozess: %+v", status.Holder)
	}
	if status.Token != "" {
		t.Fatalf("status verrät die Marke: %+v", status)
	}
	check := callLock(t, session, map[string]any{"projectDir": root, "action": "check", "token": taken.Token})
	if check.State != worklock.StateHeld {
		t.Fatalf("check: %+v", check)
	}
	released := callLock(t, session, map[string]any{"projectDir": root, "action": "release", "token": taken.Token})
	if released.State != worklock.StateReleased {
		t.Fatalf("release: %+v", released)
	}
	lost := callLock(t, session, map[string]any{"projectDir": root, "action": "check", "token": taken.Token})
	if lost.State != worklock.StateLost || lost.Reason != worklock.ReasonReleased {
		t.Fatalf("check nach release: %+v", lost)
	}
}

func TestLockToolInvalidInput(t *testing.T) {
	noHolder(t)
	root := newLockProject(t)
	session := lockSession(t, "claude-code")
	for _, args := range []map[string]any{
		{"projectDir": root, "action": "sperren"},
		{"projectDir": root, "action": "check"},
		{"projectDir": root, "action": "release"},
		{"projectDir": root, "action": "wait", "seconds": 101},
	} {
		answer := callLock(t, session, args)
		if answer.OK || answer.Error == nil || answer.Error.Code != "invalid_input" {
			t.Errorf("%v: %+v", args, answer)
		}
	}
}

// Ein laufendes wait hält den Server nicht auf: Über denselben Server —
// wie die Sub-Agenten einer Sitzung in Claude Code oder zwei Sitzungen in
// OpenCode — kommen status und release durch, und wait kehrt danach mit einer
// Marke zurück.
func TestLockToolWaitDoesNotBlockServer(t *testing.T) {
	noHolder(t)
	root := newLockProject(t)
	session := lockSession(t, "opencode")

	taken := callLock(t, session, map[string]any{"projectDir": root, "action": "acquire"})
	if taken.State != worklock.StateAcquired {
		t.Fatalf("acquire: %+v", taken)
	}

	done := make(chan lockAnswer, 1)
	go func() {
		done <- callLock(t, session, map[string]any{"projectDir": root, "action": "wait", "seconds": 30})
	}()
	time.Sleep(300 * time.Millisecond)

	statusDone := make(chan lockAnswer, 1)
	go func() { statusDone <- callLock(t, session, map[string]any{"projectDir": root, "action": "status"}) }()
	select {
	case status := <-statusDone:
		if status.State != worklock.StateBusy {
			t.Fatalf("status während wait: %+v", status)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("status hängt hinter wait")
	}

	released := callLock(t, session, map[string]any{"projectDir": root, "action": "release", "token": taken.Token})
	if released.State != worklock.StateReleased {
		t.Fatalf("release: %+v", released)
	}
	select {
	case waited := <-done:
		if !waited.OK || waited.State != worklock.StateAcquired || waited.Token == "" || waited.Token == taken.Token {
			t.Fatalf("wait: %+v", waited)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("wait kehrt nach der Freigabe nicht zurück")
	}
}

func TestLockToolWaitTimeout(t *testing.T) {
	noHolder(t)
	root := newLockProject(t)
	session := lockSession(t, "claude-code")
	callLock(t, session, map[string]any{"projectDir": root, "action": "acquire", "purpose": "erster"})
	waited := callLock(t, session, map[string]any{"projectDir": root, "action": "wait", "seconds": 1})
	if !waited.OK || waited.State != worklock.StateBusy || waited.Holder == nil || waited.Holder.Purpose != "erster" {
		t.Fatalf("wait nach Frist: %+v", waited)
	}
}

func TestLockToolHolderIsClientProcess(t *testing.T) {
	root := newLockProject(t)
	before := lockFindHolder
	lockFindHolder = func() *worklock.Process {
		return &worklock.Process{PID: os.Getpid(), Name: "claude", Role: worklock.RoleSession}
	}
	t.Cleanup(func() { lockFindHolder = before })
	session := lockSession(t, "claude-code")
	callLock(t, session, map[string]any{"projectDir": root, "action": "acquire"})
	data, err := os.ReadFile(worklock.New(root).Path())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"role": "session"`) || !strings.Contains(string(data), `"name": "claude"`) {
		t.Fatalf("Halter-Prozess fehlt in der Sperrdatei:\n%s", data)
	}
}
