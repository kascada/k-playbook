package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// k_playbook_findings_check ist die Hülle über project.CheckFindings, der
// Funktion, die auch `k-playbook findings check` ausgibt: die Vorprüfung der
// Befund-Queue vor /k-befunde-einordnen (Task 166).
const findingsCheckTool = "k_playbook_findings_check"

// checkFindings ist austauschbar, damit Tests den Node nicht fragen.
var checkFindings = project.CheckFindings

type findingsCheckInput struct {
	ProjectDir string `json:"projectDir,omitempty" jsonschema:"Pflicht. Verzeichnis, ab dem aufwärts nach K-PLAYBOOK.yaml gesucht wird. Relative Pfade werden relativ zum Arbeitsverzeichnis des MCP-Servers aufgelöst."`
}

// findingsCheckEnvelope ist die Antwortform. Das Ergebnis steht unter check —
// nicht eingebettet, denn sein ready (Einordnung darf anfangen) ist ein
// anderes als das ok des Umschlags (Werkzeug gelaufen).
type findingsCheckEnvelope struct {
	OK         bool                   `json:"ok"`
	Tool       string                 `json:"tool"`
	ProjectDir string                 `json:"projectDir"`
	Check      *project.FindingsCheck `json:"check,omitempty"`
	Error      *errorEnvelope         `json:"error,omitempty"`
}

func addFindingsTool(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name: findingsCheckTool,
		Description: "Vorprüfung der Befund-Queue keph:/queue/findings/ vor /k-befunde-einordnen. Fordert zuerst beim " +
			"Node den Abgleich der Replica an (MCP-Werkzeug sync, Node ab Kephalaion v0.5.0; geht auch in einem reinen " +
			"Client ohne Datenbank des Nodes), prüft die Rechte read, write und supersede, dann jeden Eintrag unter " +
			"queue/findings/ (Namensmuster <nummer>-<kurzname>.md, direkt in der Ablage) und liest jeden Befund. " +
			"Meldet alles auf einmal und zählt den Bestand auch, wenn sie nicht bereit ist. Antwort unter check: ready " +
			"(true genau dann, wenn problems leer ist), problems (je kind — kephalaion, sync, right, name, subdir, " +
			"unreadable, frontmatter —, name, message), sync (at, revision), rights, total, byUser (Zahl je User, der " +
			"den Befund angelegt hat), skipped (leere oder in den letzten sechs Stunden geänderte Befunde mit reason " +
			"„leer“ bzw. „jünger als 6 h“; sie machen nicht „nicht bereit“) und findings (name — der volle Name in " +
			"der Collection —, ref, revision, user, created, updated, ziel, thema). findings und skipped sind getrennte Mengen, " +
			"nach Alter sortiert, ältester zuerst. Eine leere Queue ist bereit mit total 0. Schreibt nichts außer der " +
			"Replica des Nodes. Dieselbe Antwort wie `k-playbook findings check --json`.",
	}, findingsCheckHandler)
}

func findingsCheckHandler(ctx context.Context, _ *mcp.CallToolRequest, input findingsCheckInput) (*mcp.CallToolResult, any, error) {
	projectDir, err := resolveProjectDir(input.ProjectDir)
	if err != nil {
		return jsonToolResult(findingsCheckEnvelope{OK: false, Tool: findingsCheckTool, ProjectDir: projectDir,
			Error: &errorEnvelope{Code: projectDirErrorCode(err), Message: err.Error()}}, true), nil, nil
	}
	check := checkFindings(ctx, projectDir)
	return jsonToolResult(findingsCheckEnvelope{OK: true, Tool: findingsCheckTool, ProjectDir: projectDir, Check: &check}, false), nil, nil
}
