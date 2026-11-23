package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// k_playbook_task_create ist eine dünne Hülle über project.CreateTask, dem
// Weg, den auch `k-playbook task create` geht (Task 178).
const taskToolCreate = "k_playbook_task_create"

type taskCreateInput struct {
	ProjectDir string `json:"projectDir,omitempty" jsonschema:"Pflicht. Verzeichnis, ab dem aufwärts nach K-PLAYBOOK.yaml gesucht wird. Relative Pfade werden relativ zum Arbeitsverzeichnis des MCP-Servers aufgelöst."`
	Title      string `json:"title" jsonschema:"Pflicht. Der Titel des Tasks, ohne \"Task <n> —\"; das Werkzeug setzt ihn in doppelte Anführungszeichen ins Frontmatter."`
	Name       string `json:"name" jsonschema:"Pflicht. Der Kurzname in 2–4 Worten; k-playbook macht ihn passend (Kleinbuchstaben, Bindestriche) und setzt die Nummer davor."`
	Body       string `json:"body" jsonschema:"Pflicht. Der Rumpf in Markdown, ohne Frontmatter und ohne die Überschrift \"# Task <NNN> — <Titel>\": ein Einleitungssatz, dann die Abschnitte Intent, Referenzen, Tools, Ziel, Kontext, Zu bauen in dieser Reihenfolge (Aufbau: k-playbook:/instructions/on-demand/tasks.md)."`
	DryRun     bool   `json:"dryRun,omitempty" jsonschema:"Nur prüfen und rendern, nichts anlegen. Die Antwort trägt den Text, der später geschrieben wird, mit <NNN> anstelle der Nummer."`
	NoIntent   bool   `json:"noIntent,omitempty" jsonschema:"Hebt die Pflicht zu einem Abschnitt Intent auf, etwa bei rein mechanischer Arbeit oder bei einem Task, der in einer Reihe nicht der letzte ist."`
}

// taskCreateEnvelope ist die Antwortform; die Felder des Ergebnisses fehlen
// bei einem Fehler.
type taskCreateEnvelope struct {
	OK         bool   `json:"ok"`
	Tool       string `json:"tool"`
	ProjectDir string `json:"projectDir"`
	*project.TaskCreated
	Error *errorEnvelope `json:"error,omitempty"`
}

func addTaskCreateTool(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name: taskToolCreate,
		Description: "Legt einen Task in einem Schritt an: normalisiert den Kurznamen, vergibt die nächste Nummer wie " +
			"k_playbook_doc_create, baut das Dokument (Frontmatter mit title in doppelten Anführungszeichen, " +
			"\"# Task <NNN> — <Titel>\", Rumpf), prüft es und legt es mit dem fertigen Inhalt in einem Zug an — es gibt " +
			"keinen leeren Zwischenzustand und keinen zweiten Schreibaufruf. Geprüft wird vor dem Schreiben, die " +
			"Verstöße kommen gesammelt als invalid_input: Einleitungssatz, Reihenfolge der Abschnitte (Intent, " +
			"Referenzen, Tools, Ziel, Kontext, Zu bauen) ohne leere, Intent (außer mit noIntent), Verweise mit Präfix " +
			"unter Referenzen, relative Markdown-Links, Verweise auf keph:/queue/…, Standardwerkzeuge unter Tools, " +
			"fortlaufende Etappen, ein sauberer Titel. Warnungen (warnings) verhindern nichts: fehlende project:- und " +
			"keph:-Verweise unter Referenzen, eine Etappenzahl außerhalb 4–8, ein gleichnamiger eigener offener Task. " +
			"Antwort: ref, number, name, revision, warnings; mit dryRun stattdessen der gerenderte Text (rendered) und " +
			"nichts angelegt. kephalaion_unavailable heißt: beim Hub ist nichts angekommen, kein Rückfall auf die " +
			"Platte (ein Trockenlauf läuft auch dann, ohne Nummer und ohne Prüfung der keph:-Verweise). " +
			"outcome_unknown: ob der Task entstand, ist offen — nicht wiederholen, sondern mit k_playbook_doc_list " +
			"(kind tasks) nachsehen. Hülle über `k-playbook task create`.",
	}, taskCreateTool)
}

func taskCreateTool(ctx context.Context, _ *mcp.CallToolRequest, input taskCreateInput) (*mcp.CallToolResult, any, error) {
	fail := func(dir string, code string, err error) *mcp.CallToolResult {
		return jsonToolResult(taskCreateEnvelope{OK: false, Tool: taskToolCreate, ProjectDir: dir,
			Error: &errorEnvelope{Code: code, Message: err.Error()}}, true)
	}
	projectDir, err := resolveProjectDir(input.ProjectDir)
	if err != nil {
		return fail(projectDir, projectDirErrorCode(err), err), nil, nil
	}
	created, err := project.CreateTaskVia(ctx, projectDir, project.TaskCreateOptions{
		Title: input.Title, Name: input.Name, Body: input.Body, DryRun: input.DryRun, NoIntent: input.NoIntent,
	}, openUserDocs)
	if err != nil {
		return fail(projectDir, docErrorCode(err), err), nil, nil
	}
	if created.Warnings == nil {
		created.Warnings = []string{}
	}
	return jsonToolResult(taskCreateEnvelope{OK: true, Tool: taskToolCreate, ProjectDir: projectDir, TaskCreated: &created}, false), nil, nil
}
