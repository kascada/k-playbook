package mcpserver

import (
	"context"
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kascada/k-playbook/installer/internal/notify"
)

// k_playbook_notify ist die Hülle über notify.Send, derselben Mitte, die
// `k-playbook notify` ruft. Die Meldung erscheint auf dem Rechner, auf dem
// der MCP-Server läuft — das ist der Rechner, auf dem der Client ihn
// gestartet hat, und in einem Devcontainer ist das der Container, nicht der
// Desktop davor. Dann gibt es keinen Kanal, und das Werkzeug sagt es.
const notifyToolName = "k_playbook_notify"

// notifyInput trägt projectDir wie die übrigen Familien: Pflicht im Vertrag,
// optional im Schema. Die Meldung selbst braucht kein Projekt; der Parameter
// hält das Werkzeug in der Reihe, und ein späterer Schalter — etwa den Ton
// rechnerlokal abzustellen — hätte im Projekt seinen Platz.
type notifyInput struct {
	ProjectDir string `json:"projectDir,omitempty" jsonschema:"Pflicht. Verzeichnis, ab dem aufwärts nach K-PLAYBOOK.yaml gesucht wird. Relative Pfade werden relativ zum Arbeitsverzeichnis des MCP-Servers aufgelöst."`
	Title      string `json:"title,omitempty" jsonschema:"Pflicht. Überschrift der Meldung, kurz."`
	Body       string `json:"body,omitempty" jsonschema:"Text der Meldung; wird nach etwa 200 Zeichen gekürzt."`
	Sound      bool   `json:"sound,omitempty" jsonschema:"true spielt zusätzlich einen Ton. Ohne Angabe bleibt die Meldung stumm."`
}

// notifyEnvelope ist die Antwortform: der Wertesatz von notify.Send
// eingebettet, bei einem Fehler fehlt er.
type notifyEnvelope struct {
	OK         bool   `json:"ok"`
	Tool       string `json:"tool"`
	ProjectDir string `json:"projectDir"`
	*notify.Result
	Error *knowledgeErrorEnvelope `json:"error,omitempty"`
}

// notifySend ist notify.Send, als Variable: die Tests der Hülle sollen keinen
// Toast auf dem Rechner des Entwicklers auslösen.
var notifySend = notify.Send

func addNotifyTool(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name: notifyToolName,
		Description: "Zeigt dem Nutzer eine Meldung auf dem Rechner, auf dem dieser MCP-Server läuft — ein " +
			"Popup mit title und body, mit sound: true dazu ein Ton. Gedacht für das Ende einer langen " +
			"Arbeit, wenn niemand mehr auf den Bildschirm schaut; nicht für Zwischenstände. Die Antwort " +
			"nennt channel (windows-toast über WSL, macos, linux-desktop) und soundPlayed; war ein Ton " +
			"verlangt und ging nicht, steht der Grund in reason. Hat der Rechner keinen Weg zum Nutzer — " +
			"etwa ein Devcontainer ohne Display und ohne Windows-Seite —, ist die Antwort ok: false mit " +
			"error.code no_channel und dem Grund in error.message; das ist eine Auskunft, kein Grund zum " +
			"Wiederholen. Dieselbe Antwort wie das Subkommando `k-playbook notify`.",
	}, notifyTool)
}

func notifyTool(ctx context.Context, _ *mcp.CallToolRequest, input notifyInput) (*mcp.CallToolResult, any, error) {
	fail := func(dir string, code string, err error) *mcp.CallToolResult {
		return jsonToolResult(notifyEnvelope{OK: false, Tool: notifyToolName, ProjectDir: dir,
			Error: &knowledgeErrorEnvelope{Code: code, Message: err.Error()}}, true)
	}
	projectDir, err := resolveProjectDir(input.ProjectDir)
	if err != nil {
		return fail(projectDir, projectDirErrorCode(err), err), nil, nil
	}
	result, err := notifySend(ctx, notify.Request{Title: input.Title, Body: input.Body, Sound: input.Sound})
	if err != nil {
		return fail(projectDir, notifyErrorCode(err), err), nil, nil
	}
	return jsonToolResult(notifyEnvelope{OK: true, Tool: notifyToolName, ProjectDir: projectDir,
		Result: &result}, false), nil, nil
}

// notifyErrorCode ordnet einen Fehler aus notify.Send seinem Code zu: ein
// fehlender Titel ist ein Eingabefehler, ein fehlender Kanal eine Auskunft
// über den Rechner, alles andere ist das Programm, das scheiterte.
func notifyErrorCode(err error) string {
	switch {
	case errors.Is(err, notify.ErrEmptyTitle):
		return "invalid_input"
	case errors.Is(err, notify.ErrNoChannel):
		return "no_channel"
	default:
		return "failed"
	}
}
