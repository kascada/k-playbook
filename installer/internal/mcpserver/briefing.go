package mcpserver

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// k_playbook_briefing ist die Hülle über project.BuildBriefing, derselben
// Funktion, die `k-playbook briefing` ausgibt. Das Briefing ist der eine
// Aufruf zu Beginn einer Sitzung: Datum, Projekt und Anbindung in eigenen
// Abschnitten, der Text der immer geltenden Anweisungen, dann welche
// Dokumente gelten und was ansteht, als Verweise — gelesen wird dann über
// k_playbook_doc_read —, dazu unter health, was an der Umgebung nicht stimmt
// (health.Check, wie k_playbook_health). Das Werkzeug heilt nebenbei die
// Assistenten-Links (project.HealLinks, Ergebnis in links) — es schreibt also
// als Nebenwirkung. Mit part "environment" liefert es statt des Briefings den
// Teilabruf (project.BuildBriefingEnvironment), der nichts schreibt.
const briefingTool = "k_playbook_briefing"

// briefingInput trägt projectDir wie die übrigen Familien: Pflicht im
// Vertrag, optional im Schema, geprüft in checkProjectDir.
type briefingInput struct {
	ProjectDir string `json:"projectDir,omitempty" jsonschema:"Pflicht. Verzeichnis, ab dem aufwärts nach K-PLAYBOOK.yaml gesucht wird. Relative Pfade werden relativ zum Arbeitsverzeichnis des MCP-Servers aufgelöst."`
	Part       string `json:"part,omitempty" jsonschema:"Leer für das Briefing. Mit environment kommt statt des Briefings nur der Teilabruf environment: die Umgebungsangaben gh, github, baseTools, versionSources, docs und remediation — ohne Sitzungskennung, Anweisungen und Arbeitsliste, kein Neuladen des Briefings."`
}

// briefingEnvelope ist die Antwortform. Das Briefing steht eingebettet, mit
// denselben Feldnamen wie auf der Kommandozeile; bei einem Fehler fehlt es.
// Beim Teilabruf steht statt des Briefings nur environment.
type briefingEnvelope struct {
	OK         bool   `json:"ok"`
	Tool       string `json:"tool"`
	ProjectDir string `json:"projectDir"`
	*project.Briefing
	Environment *project.BriefingEnvironment `json:"environment,omitempty"`
	Error       *errorEnvelope               `json:"error,omitempty"`
}

func addBriefingTool(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name: briefingTool,
		Description: "Das Briefing zu Beginn einer Sitzung: was für dieses Projekt gilt und was ansteht. " +
			"documents.always.content ist der Text der immer geltenden Anweisungen k-playbook:/instructions/always/, " +
			"zusammengebaut wie bei k_playbook_doc_read — lies ihn zuerst, er ist der einzige Inhalt im Briefing; " +
			"alles Weitere sind Verweise im Präfixschema, keine Pfade. session: die Kennung dieser Sitzung, je Aufruf " +
			"neu vergeben — sie steht in allem, was die Sitzung später wiederfinden muss, etwa im Kopf eines Befunds. " +
			"now: date und timestamp des Aufrufs. project: dir (die Wurzel des Projekts), repoRoot, vcs und languages; " +
			"lässt sich die Konfiguration nicht lesen, steht available false mit message, dir bleibt. kephalaion: ok " +
			"(ob keph:-Verweise erreichbar sind), message, address (<hub>:<collection>, der Wert für collection in den " +
			"Kephalaion-Werkzeugen) und user (wofür ~ in einem keph:-Verweis steht). documents: always (ref, content, parts), " +
			"onDemand, reviews, guidelines — lokal und " +
			"mitgeliefert schon gegeneinander aufgelöst, guidelines nur mitgeliefert (origin immer dist), je Eintrag ref, title, description und origin (dist, local, " +
			"override, disabled), bei einem lokalen Eintrag aus der Collection des Projekts dazu location, sein " +
			"keph:-Ort unter keph:/k-playbook/. work: die eigenen offenen Tasks (nach Nummer) und aktiven Arbeitsstände (zuletzt geänderte zuerst) aus der Projekt-Collection " +
			"mit Titel — und problem, wenn das Frontmatter nicht lesbar oder der Titel vermutlich an ‚ #‘ abgeschnitten " +
			"ist —, je Liste höchstens 10 Einträge; total nennt je Liste die Zahl aller, den Rest liefert " +
			"k_playbook_doc_list (kind tasks bzw. workbench) — und openFindings, die Zahl der offenen Befunde in queue/findings/ — offen heißt nicht in done/; " +
			"ist Kephalaion nicht nutzbar, steht available false mit message. projectDocs: der Einstieg der " +
			"Projektdoku, der Verzeichnis-Verweis keph:/docs/ in der Collection des Projekts — gelesen mit " +
			"Kephalaion list auf docs/ (recursive, frontmatter), nicht mit k_playbook_doc_read; zuerst docs/index.md, " +
			"sobald es sie gibt. Einen Eintrag unter documents liest du mit k_playbook_doc_read (ref). " +
			"github.projectServices: die Dienste der persönlichen GitHub-Zugangsdatei für das Repo des " +
			"Arbeitsverzeichnisses, der Wert für k-playbook github run <dienst>. health: die Umgebungsprüfung — ok " +
			"und problems, je Eintrag level (error, warning), area (kephalaion, baseTools, versionSources, …), message " +
			"und hint; nenne die Einträge in deiner ersten Antwort, Fehler vor der Arbeit, behebe nichts still — " +
			"dieselbe Antwort wie k_playbook_health. links: nur wenn es etwas meldet — healed, open und note zur " +
			"Assistenten-Verlinkung, die der Aufruf als Nebenwirkung nachzieht. Ein Aufruf genügt für den Einstieg. " +
			"Dieselbe Antwort wie `k-playbook briefing --json`. " +
			"Mit part \"environment\" kommt statt des Briefings nur environment (wie `k-playbook briefing --part " +
			"environment --json`): available, message und die Umgebungsangaben, die einzelne Commands an der Stelle " +
			"brauchen, an der sie sie lesen — gh (ready, account), github (file, present, problems, remote, repo, " +
			"projectServices, services), baseTools (present, error, missing, ok, installCommand), versionSources, docs " +
			"(exclude, problems) und remediation. Der Teilabruf vergibt keine Sitzungskennung, liefert weder " +
			"Anweisungen noch Arbeitsliste, schreibt nichts und gilt nicht als Neuladen des Briefings; er wird bei " +
			"Bedarf frisch geholt, auch mehrmals je Sitzung. health bewertet und nennt Abhilfen, environment liefert " +
			"die Rohwerte.",
	}, briefingToolHandler)
}

func briefingToolHandler(ctx context.Context, _ *mcp.CallToolRequest, input briefingInput) (*mcp.CallToolResult, any, error) {
	fail := func(dir string, code string, err error) *mcp.CallToolResult {
		return jsonToolResult(briefingEnvelope{OK: false, Tool: briefingTool, ProjectDir: dir,
			Error: &errorEnvelope{Code: code, Message: err.Error()}}, true)
	}
	projectDir, err := resolveProjectDir(input.ProjectDir)
	if err != nil {
		return fail(projectDir, projectDirErrorCode(err), err), nil, nil
	}
	// github ab dem übergebenen Verzeichnis: Liegt es in einem eigenen Repo
	// unterhalb des Projekts, zählt dessen Remote.
	startDir := projectDir
	if input.ProjectDir != "" {
		if absolute, err := filepath.Abs(input.ProjectDir); err == nil {
			startDir = absolute
		}
	}
	switch input.Part {
	case "":
	case project.BriefingPartEnvironment:
		environment := project.BuildBriefingEnvironment(projectDir, startDir)
		return jsonToolResult(briefingEnvelope{OK: true, Tool: briefingTool, ProjectDir: projectDir, Environment: &environment}, false), nil, nil
	default:
		return fail(projectDir, "invalid_input", fmt.Errorf("unbekannter Teil %q — erlaubt ist part %q oder kein part", input.Part, project.BriefingPartEnvironment)), nil, nil
	}
	briefing, err := project.BuildBriefingFrom(ctx, projectDir, startDir)
	if err != nil {
		// Ohne mitgelieferte Ebene (Kephalaion nicht bereit, <hub>:k-playbook
		// fehlt oder ist leer) scheitert das Briefing als Ganzes, mit dem
		// Code der Ursache; jede andere unlesbare Ebene ist read_failed.
		code := "read_failed"
		if project.DocRefErrorCode(err) == project.DocCodeKephalaionUnavailable {
			code = project.DocCodeKephalaionUnavailable
		}
		return fail(projectDir, code, err), nil, nil
	}
	return jsonToolResult(briefingEnvelope{OK: true, Tool: briefingTool, ProjectDir: projectDir, Briefing: &briefing}, false), nil, nil
}
