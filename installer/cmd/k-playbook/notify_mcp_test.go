package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// Beide Wege, eine Antwort: `k-playbook notify` und k_playbook_notify kommen
// aus notify.Send. Geprüft wird der eine Fall, der auf jedem Rechner gleich
// ausgeht, ohne dort etwas anzuzeigen: kein Kanal. Dafür bekommt der
// Testprozess einen leeren PATH, kein Display und die Container-Marke — der
// MCP-Server erbt die Umgebung, die CLI läuft im selben Prozess.
func TestNotifyCLIUndMCPGebenDasselbe(t *testing.T) {
	root := newProject(t)
	t.Setenv("PATH", t.TempDir())
	t.Setenv("DISPLAY", "")
	t.Setenv("WAYLAND_DISPLAY", "")
	t.Setenv("WSL_DISTRO_NAME", "")
	t.Setenv("WSL_INTEROP", "")
	t.Setenv("DEVCONTAINER", "true")

	var out bytes.Buffer
	cliErr := runNotify(&out, []string{"--title", "Fertig", "--body", "Lauf 3", "--json"})
	if cliErr == nil {
		t.Fatalf("die CLI fand einen Kanal, obwohl keiner da sein darf: %s", out.String())
	}
	if out.Len() != 0 {
		t.Errorf("ohne Kanal steht etwas auf stdout: %q", out.String())
	}
	if !strings.Contains(cliErr.Error(), "Container erkannt (DEVCONTAINER)") {
		t.Errorf("die CLI nennt den Container nicht: %v", cliErr)
	}

	stdout := speak(t, root, 2, initialize(1), initialized,
		callTool(2, "k_playbook_notify", map[string]any{"projectDir": root, "title": "Fertig", "body": "Lauf 3"}))
	responses := decodeResponses(t, stdout, 1, 2)

	envelope := map[string]any{}
	if err := json.Unmarshal([]byte(toolText(t, responses[1])), &envelope); err != nil {
		t.Fatalf("Werkzeug lieferte kein JSON: %v", err)
	}
	if envelope["ok"] != false || envelope["tool"] != "k_playbook_notify" || envelope["projectDir"] != root {
		t.Fatalf("Umschlag: %v", envelope)
	}
	problem, _ := envelope["error"].(map[string]any)
	if problem["code"] != "no_channel" {
		t.Fatalf("Fehlercode: %v", envelope)
	}
	if problem["message"] != cliErr.Error() {
		t.Errorf("CLI %q gegen MCP %q", cliErr.Error(), problem["message"])
	}
}

// Ohne --title läuft nichts: Fehler, Hilfe auf stderr, nichts auf stdout.
func TestNotifyCLIOhneTitel(t *testing.T) {
	var out bytes.Buffer
	if err := runNotify(&out, []string{"--body", "x"}); err == nil || !strings.Contains(err.Error(), "--title") {
		t.Fatalf("Fehler = %v, erwartet den Hinweis auf --title", err)
	}
	if out.Len() != 0 {
		t.Errorf("stdout ist nicht leer: %q", out.String())
	}
}

// --help druckt die Hilfe auf stdout und endet ohne Fehler.
func TestNotifyCLIHilfe(t *testing.T) {
	var out bytes.Buffer
	if err := runNotify(&out, []string{"--help"}); err != nil {
		t.Fatalf("--help: %v", err)
	}
	if !strings.HasPrefix(out.String(), "k-playbook notify") {
		t.Errorf("Hilfe: %q", out.String())
	}
}

// Das Unterkommando steht im Dispatch von main: `notify --help` kommt bei
// runNotify an.
func TestNotifyImDispatch(t *testing.T) {
	stdout := captureStdout(t, func() {
		if err := run([]string{"notify", "--help"}); err != nil {
			t.Errorf("run: %v", err)
		}
	})
	if !strings.HasPrefix(stdout, "k-playbook notify") {
		t.Errorf("Hilfe über run(): %q", stdout)
	}
}
