package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kascada/k-playbook/installer/internal/notify"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// fakeNotify ersetzt notify.Send und zeichnet die Anfrage auf. Kein Test
// dieser Hülle darf auf dem Rechner des Entwicklers etwas anzeigen.
func fakeNotify(t *testing.T, result notify.Result, err error) *notify.Request {
	t.Helper()
	got := &notify.Request{}
	before := notifySend
	notifySend = func(_ context.Context, req notify.Request) (notify.Result, error) {
		*got = req
		return result, err
	}
	t.Cleanup(func() { notifySend = before })
	return got
}

func newNotifyProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := project.CreateConfig(root, "."); err != nil {
		t.Fatalf("Konfiguration anlegen: %v", err)
	}
	return root
}

func callNotify(t *testing.T, input notifyInput) (map[string]any, bool) {
	t.Helper()
	result, _, err := notifyTool(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("notify: %v", err)
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("Inhalt ist kein Text: %#v", result.Content[0])
	}
	fields := map[string]any{}
	if err := json.Unmarshal([]byte(text.Text), &fields); err != nil {
		t.Fatalf("Antwort ist kein JSON-Objekt (%q): %v", text.Text, err)
	}
	return fields, result.IsError
}

func errorCode(t *testing.T, fields map[string]any) string {
	t.Helper()
	problem, ok := fields["error"].(map[string]any)
	if !ok {
		t.Fatalf("kein error in der Antwort: %v", fields)
	}
	code, _ := problem["code"].(string)
	return code
}

// Der Erfolgsfall: die Anfrage geht unverändert an notify.Send, der
// Wertesatz steht eingebettet im Umschlag.
func TestNotifyToolErfolg(t *testing.T) {
	root := newNotifyProject(t)
	got := fakeNotify(t, notify.Result{Channel: notify.ChannelWindowsToast, SoundPlayed: true}, nil)

	fields, isError := callNotify(t, notifyInput{ProjectDir: root, Title: "Fertig", Body: "Lauf 3", Sound: true})
	if isError || fields["ok"] != true || fields["tool"] != notifyToolName || fields["projectDir"] != root {
		t.Fatalf("Umschlag: %v", fields)
	}
	if fields["channel"] != notify.ChannelWindowsToast || fields["soundPlayed"] != true {
		t.Errorf("Wertesatz: %v", fields)
	}
	if _, ok := fields["reason"]; ok {
		t.Errorf("reason steht da, obwohl nichts zu erklären ist: %v", fields)
	}
	if got.Title != "Fertig" || got.Body != "Lauf 3" || !got.Sound {
		t.Errorf("Anfrage an notify.Send: %+v", *got)
	}
}

// Ohne Kanal ist die Antwort ok: false mit no_channel und dem Grund; der
// Wertesatz fehlt.
func TestNotifyToolOhneKanal(t *testing.T) {
	root := newNotifyProject(t)
	fakeNotify(t, notify.Result{}, &notify.NoChannelError{Reason: "Container erkannt (DEVCONTAINER)"})

	fields, isError := callNotify(t, notifyInput{ProjectDir: root, Title: "Fertig"})
	if !isError || fields["ok"] != false {
		t.Fatalf("ein fehlender Kanal wurde nicht gemeldet: %v", fields)
	}
	if code := errorCode(t, fields); code != "no_channel" {
		t.Errorf("Fehlercode = %q, erwartet no_channel", code)
	}
	problem := fields["error"].(map[string]any)
	if message, _ := problem["message"].(string); message != "keine Meldung möglich: Container erkannt (DEVCONTAINER)" {
		t.Errorf("Meldung: %q", message)
	}
	if _, ok := fields["channel"]; ok {
		t.Errorf("channel steht in einer Fehlerantwort: %v", fields)
	}
}

// Ein leerer Titel ist invalid_input; ein scheiterndes Programm ist failed.
func TestNotifyToolFehlercodes(t *testing.T) {
	root := newNotifyProject(t)

	fakeNotify(t, notify.Result{}, notify.ErrEmptyTitle)
	fields, isError := callNotify(t, notifyInput{ProjectDir: root})
	if !isError || errorCode(t, fields) != "invalid_input" {
		t.Errorf("leerer Titel: %v", fields)
	}

	fakeNotify(t, notify.Result{}, errors.New("powershell.exe: Toast kaputt"))
	fields, isError = callNotify(t, notifyInput{ProjectDir: root, Title: "Fertig"})
	if !isError || errorCode(t, fields) != "failed" {
		t.Errorf("scheiterndes Programm: %v", fields)
	}
}

// Ohne projectDir wird abgelehnt, bevor notify.Send läuft; ein Verzeichnis
// ohne Installation ist project_not_found.
func TestNotifyToolOhneProjectDir(t *testing.T) {
	got := fakeNotify(t, notify.Result{Channel: notify.ChannelMacOS}, nil)

	fields, isError := callNotify(t, notifyInput{Title: "Fertig"})
	if !isError || errorCode(t, fields) != "invalid_input" {
		t.Errorf("ohne projectDir: %v", fields)
	}
	fields, isError = callNotify(t, notifyInput{ProjectDir: t.TempDir(), Title: "Fertig"})
	if !isError || errorCode(t, fields) != "project_not_found" {
		t.Errorf("ohne Installation: %v", fields)
	}
	if got.Title != "" {
		t.Errorf("notify.Send lief trotz abgelehnter Eingabe: %+v", *got)
	}
}
