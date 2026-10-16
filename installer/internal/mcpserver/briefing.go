package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// k_playbook_briefing ist die Hülle über project.BuildBriefing, derselben
// Funktion, die `k-playbook briefing` ausgibt. Das Briefing ist der eine
// Aufruf zu Beginn einer Sitzung: welche Dokumente gelten und was ansteht, als
// Verweise — gelesen wird dann über k_playbook_doc_read. Umgebungsangaben
// bleiben in k_playbook_context; nichts steht doppelt.
const briefingTool = "k_playbook_briefing"

// briefingInput trägt projectDir wie die übrigen Familien: Pflicht im
// Vertrag, optional im Schema, geprüft in checkProjectDir.
type briefingInput struct {
	ProjectDir string `json:"projectDir,omitempty" jsonschema:"Pflicht. Verzeichnis, ab dem aufwärts nach K-PLAYBOOK.yaml gesucht wird. Relative Pfade werden relativ zum Arbeitsverzeichnis des MCP-Servers aufgelöst."`
}

// briefingEnvelope ist die Antwortform. Das Briefing steht eingebettet, mit
// denselben Feldnamen wie auf der Kommandozeile; bei einem Fehler fehlt es.
type briefingEnvelope struct {
	OK         bool   `json:"ok"`
	Tool       string `json:"tool"`
	ProjectDir string `json:"projectDir"`
	*project.Briefing
	Error *knowledgeErrorEnvelope `json:"error,omitempty"`
}

func addBriefingTool(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name: briefingTool,
		Description: "Das Briefing zu Beginn einer Sitzung: listet als Daten, was für dieses Projekt gilt und was " +
			"ansteht — Verweise im Präfixschema, keine Pfade, keine Inhalte. documents: always (das zusammengebaute " +
			"k-playbook:/instructions/always/ mit seinen Teilen), onDemand, rules, reviews, guidelines — lokal und " +
			"mitgeliefert schon gegeneinander aufgelöst, je Eintrag ref, title, description und origin (dist, local, " +
			"override, disabled). work: die eigenen offenen Tasks und aktiven Arbeitsstände aus der Projekt-Collection " +
			"mit Titel und openFindings, die Zahl der offenen Befunde in queue/findings/ — offen heißt nicht in done/; " +
			"ist Kephalaion nicht nutzbar, steht available false mit message. projectDocs: der Einstieg der " +
			"Projektdoku. Einen Eintrag liest du mit k_playbook_doc_read (ref). Umgebungsangaben — Pfade, Werkzeuge, " +
			"gh, Kephalaion-Anbindung — stehen nicht hier, sondern in k_playbook_context. Dieselbe Antwort wie " +
			"`k-playbook briefing --json`.",
	}, briefingToolHandler)
}

func briefingToolHandler(ctx context.Context, _ *mcp.CallToolRequest, input briefingInput) (*mcp.CallToolResult, any, error) {
	fail := func(dir string, code string, err error) *mcp.CallToolResult {
		return jsonToolResult(briefingEnvelope{OK: false, Tool: briefingTool, ProjectDir: dir,
			Error: &knowledgeErrorEnvelope{Code: code, Message: err.Error()}}, true)
	}
	projectDir, err := resolveProjectDir(input.ProjectDir)
	if err != nil {
		return fail(projectDir, projectDirErrorCode(err), err), nil, nil
	}
	briefing, err := project.BuildBriefing(ctx, projectDir)
	if err != nil {
		return fail(projectDir, "read_failed", err), nil, nil
	}
	return jsonToolResult(briefingEnvelope{OK: true, Tool: briefingTool, ProjectDir: projectDir, Briefing: &briefing}, false), nil, nil
}
