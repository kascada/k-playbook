package main

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/guiproc"
)

// Beide Wege, eine Antwort: `k-playbook gui status` auf der Kommandozeile und
// k_playbook_gui_status über MCP liefern denselben Wertesatz. Der Test trägt,
// weil beide aus guiproc.StatusForDir kommen — verglichen werden nicht zwei
// Nachbauten.
//
// Er läuft auf einem Verzeichnis **mit** Installation: nur so eines nimmt
// resolveProjectDir an, während guiproc auch für ein Verzeichnis ohne eine
// einen Schlüssel bildet. Für Verzeichnisse ohne Installation gilt die
// Gleichheit deshalb nicht — dort gibt es den MCP-Weg nicht.
func TestGUIStatusCLIUndMCPGebenDasselbe(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	root := newProject(t)

	key := guiproc.KeyFrom(root)
	addr := serveGUIStatusHealth(t, guiproc.Health{
		Status: "ok", Key: key, Version: guiproc.OwnVersion(), Build: guiproc.OwnBuild(), PID: os.Getpid(),
	})
	record := ownGUIStatusRecord(addr)
	record.Key = key
	if _, err := guiproc.Register(record); err != nil {
		t.Fatalf("Register: %v", err)
	}

	t.Chdir(root)
	cli := guiStatusFields(t)
	if cli["state"] != string(guiproc.StateRunning) {
		t.Fatalf("die CLI meldet nicht running: %v", cli)
	}

	stdout := speak(t, root, 2, initialize(1), initialized,
		callTool(2, "k_playbook_gui_status", map[string]any{"projectDir": root}))
	responses := decodeResponses(t, stdout, 1, 2)

	envelope := map[string]any{}
	if err := json.Unmarshal([]byte(toolText(t, responses[1])), &envelope); err != nil {
		t.Fatalf("Werkzeug lieferte kein JSON: %v", err)
	}
	if envelope["ok"] != true || envelope["tool"] != "k_playbook_gui_status" || envelope["projectDir"] != root {
		t.Fatalf("Umschlag: %v", envelope)
	}
	// Den Umschlag abziehen; was bleibt, ist der Wertesatz.
	for _, name := range []string{"ok", "tool", "projectDir"} {
		delete(envelope, name)
	}
	if !reflect.DeepEqual(cli, envelope) {
		t.Errorf("CLI %v gegen MCP %v", cli, envelope)
	}
}
