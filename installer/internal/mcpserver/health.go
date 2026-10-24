package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kascada/k-playbook/installer/internal/health"
)

// k_playbook_health ist die Hülle über health.Check, derselben Funktion, die
// `k-playbook health` ausgibt und die das Briefing als Abschnitt health
// trägt: Fehler und Warnungen über die Umgebung des Projekts, alle auf
// einmal. Das Werkzeug ist für den Aufruf zwischendurch — etwa nachdem der
// Nutzer eine Abhilfe ausgeführt hat —, das Briefing trägt denselben Report
// zu Beginn.
const healthTool = "k_playbook_health"

// healthInput trägt projectDir wie die übrigen Familien: Pflicht im Vertrag,
// optional im Schema, geprüft in checkProjectDir.
type healthInput struct {
	ProjectDir string `json:"projectDir,omitempty" jsonschema:"Pflicht. Verzeichnis, ab dem aufwärts nach K-PLAYBOOK.yaml gesucht wird. Relative Pfade werden relativ zum Arbeitsverzeichnis des MCP-Servers aufgelöst."`
}

// healthEnvelope ist die Antwortform. Der Report steht unter health — nicht
// eingebettet, denn sein ok (keine Fehler) ist ein anderes als das des
// Umschlags (Werkzeug gelaufen); bei einem Fehler fehlt er.
type healthEnvelope struct {
	OK         bool                    `json:"ok"`
	Tool       string                  `json:"tool"`
	ProjectDir string                  `json:"projectDir"`
	Health     *health.Report          `json:"health,omitempty"`
	Error      *knowledgeErrorEnvelope `json:"error,omitempty"`
}

func addHealthTool(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name: healthTool,
		Description: "Prüft die Umgebung des Projekts und nennt alle Fehler und Warnungen auf einmal — repariert nichts. " +
			"health.ok ist false, sobald ein Eintrag die Stufe error trägt; Warnungen lassen es true. health.problems: " +
			"je Eintrag level (error, warning), area (kephalaion, gh, baseTools, versionSources), message (ein Satz, nie ein " +
			"Token oder Wert) und hint (die Abhilfe), sortiert nach area, dann level. Ohne Problem ist problems leer. " +
			"Das Briefing (k_playbook_briefing) trägt denselben Report unter health; dieses Werkzeug ist für den Aufruf " +
			"zwischendurch, etwa nach einer Abhilfe. Dieselbe Antwort wie `k-playbook health --json`.",
	}, healthToolHandler)
}

func healthToolHandler(ctx context.Context, _ *mcp.CallToolRequest, input healthInput) (*mcp.CallToolResult, any, error) {
	projectDir, err := resolveProjectDir(input.ProjectDir)
	if err != nil {
		return jsonToolResult(healthEnvelope{OK: false, Tool: healthTool, ProjectDir: projectDir,
			Error: &knowledgeErrorEnvelope{Code: projectDirErrorCode(err), Message: err.Error()}}, true), nil, nil
	}
	report := health.Check(ctx, projectDir)
	return jsonToolResult(healthEnvelope{OK: true, Tool: healthTool, ProjectDir: projectDir, Health: &report}, false), nil, nil
}
