package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kascada/k-playbook/installer/internal/guiproc"
)

// k_playbook_gui_status ist die Hülle über guiproc.StatusForDir, derselben
// Mitte, die `k-playbook gui status` rendert. Das Werkzeug serialisiert deren
// Wertesatz; zusammengesetzt wird er in guiproc, denn zwei Fassungen derselben
// Antwort driften.
const guiToolStatus = "k_playbook_gui_status"

// guiStatusInput trägt projectDir wie die übrigen Familien: Pflicht im
// Vertrag, optional im Schema, geprüft in checkProjectDir.
//
// Ein Rückfall auf das Arbeitsverzeichnis des Serverprozesses gibt es hier so
// wenig wie dort — und er wäre hier besonders still: der Server antwortete
// dann für das Projekt, in dem der Client ihn gestartet hat, mit einem
// Zustand, der nach einer Auskunft über das gefragte aussieht.
type guiStatusInput struct {
	ProjectDir string `json:"projectDir,omitempty" jsonschema:"Pflicht. Verzeichnis, ab dem aufwärts nach K-PLAYBOOK.yaml gesucht wird. Relative Pfade werden relativ zum Arbeitsverzeichnis des MCP-Servers aufgelöst."`
}

// guiStatusEnvelope ist die Antwortform. Der Wertesatz steht eingebettet, mit
// denselben Feldnamen wie auf der Kommandozeile; bei einem Fehler fehlt er
// ganz, wie in den übrigen Familien.
type guiStatusEnvelope struct {
	OK         bool   `json:"ok"`
	Tool       string `json:"tool"`
	ProjectDir string `json:"projectDir"`
	*guiproc.StatusReport
	Error *knowledgeErrorEnvelope `json:"error,omitempty"`
}

func addGUITools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name: guiToolStatus,
		Description: "Nennt Zustand und Adresse der Oberfläche eines Projekts, ohne irgendetwas zu verändern: " +
			"state ist none (kein Dienst — dasselbe gilt für ein fehlendes Laufzeitverzeichnis), running " +
			"(der Dienst dieses Programms läuft unter url), otherVersion (ein Dienst aus einem anderen Stand " +
			"läuft), orphaned (die Laufzeitdatei nennt einen Prozess, den es nicht mehr gibt) oder unresponsive " +
			"(der Prozess lebt, /api/health antwortet aber nicht). Dazu, soweit gemessen, url und pid des " +
			"lebenden Prozesses sowie version und build aus /api/health; nicht gemessene Felder fehlen in der " +
			"Antwort. Es startet nichts, beendet nichts und räumt keine verwaiste Laufzeitdatei weg — das tut " +
			"`k-playbook stop`. Dieselbe Antwort wie das Subkommando `k-playbook gui status`.",
	}, guiStatusTool)
}

func guiStatusTool(ctx context.Context, _ *mcp.CallToolRequest, input guiStatusInput) (*mcp.CallToolResult, any, error) {
	fail := func(dir string, code string, err error) *mcp.CallToolResult {
		return jsonToolResult(guiStatusEnvelope{OK: false, Tool: guiToolStatus, ProjectDir: dir,
			Error: &knowledgeErrorEnvelope{Code: code, Message: err.Error()}}, true)
	}
	projectDir, err := resolveProjectDir(input.ProjectDir)
	if err != nil {
		return fail(projectDir, projectDirErrorCode(err), err), nil, nil
	}
	report, err := guiproc.StatusForDir(projectDir)
	if err != nil {
		return fail(projectDir, "failed", err), nil, nil
	}
	return jsonToolResult(guiStatusEnvelope{OK: true, Tool: guiToolStatus, ProjectDir: projectDir,
		StatusReport: &report}, false), nil, nil
}
