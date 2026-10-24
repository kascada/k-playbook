package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// k_playbook_briefing ist die Hülle über project.BuildBriefing, derselben
// Funktion, die `k-playbook briefing` ausgibt. Das Briefing ist der eine
// Aufruf zu Beginn einer Sitzung: der Text der immer geltenden Anweisungen,
// dann welche Dokumente gelten und was ansteht, als Verweise — gelesen wird
// dann über k_playbook_doc_read —, dazu unter health, was an der Umgebung
// nicht stimmt (health.Check, wie k_playbook_health). Umgebungsangaben
// stehen nur im Übergangsabschnitt context, dem Rest von k_playbook_context;
// nichts steht doppelt. Weil der Rest über ContextForDir kommt, heilt auch
// dieses Werkzeug nebenbei die Assistenten-Links (Ergebnis in context.links)
// — neben k_playbook_context das zweite, das als Nebenwirkung schreibt.
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
		Description: "Das Briefing zu Beginn einer Sitzung: was für dieses Projekt gilt und was ansteht. " +
			"documents.always.content ist der Text der immer geltenden Anweisungen k-playbook:/instructions/always/, " +
			"zusammengebaut wie bei k_playbook_doc_read — lies ihn zuerst, er ist der einzige Inhalt im Briefing; " +
			"alles Weitere sind Verweise im Präfixschema, keine Pfade. documents: always (ref, content, parts), " +
			"onDemand, rules, reviews, guidelines — lokal und " +
			"mitgeliefert schon gegeneinander aufgelöst, je Eintrag ref, title, description und origin (dist, local, " +
			"override, disabled). work: die eigenen offenen Tasks und aktiven Arbeitsstände aus der Projekt-Collection " +
			"mit Titel und openFindings, die Zahl der offenen Befunde in queue/findings/ — offen heißt nicht in done/; " +
			"ist Kephalaion nicht nutzbar, steht available false mit message. projectDocs: der Einstieg der " +
			"Projektdoku. Einen Eintrag liest du mit k_playbook_doc_read (ref). health: die Umgebungsprüfung — ok " +
			"und problems, je Eintrag level (error, warning), area (kephalaion, gh, baseTools, versionSources), message " +
			"und hint; nenne die Einträge in deiner ersten Antwort, Fehler vor der Arbeit, behebe nichts still — " +
			"dieselbe Antwort wie k_playbook_health. context: der Übergangsabschnitt " +
			"mit dem Rest von k_playbook_context — die Umgebungsangaben (Pfade, Werkzeuge, gh, git, mcp, " +
			"remediation, Kephalaion-Anbindung …) unter ihren Namen aus der Kontextausgabe, ohne catalogs.rules, " +
			"catalogs.reviews und guidelines, die documents schon liefert; lässt sich die Kontextausgabe nicht " +
			"bauen, steht available false mit message. Ein Aufruf genügt damit für den Einstieg; k_playbook_context " +
			"ist nicht zusätzlich nötig. Nebenwirkung wie dort: zieht die Assistenten-Verlinkung nach und meldet " +
			"unter context.links, was sich geändert hat. Dieselbe Antwort wie `k-playbook briefing --json`.",
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
