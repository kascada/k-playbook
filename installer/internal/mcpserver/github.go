package mcpserver

import (
	"context"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kascada/k-playbook/installer/internal/githubaccess"
)

// k_playbook_github_test ist die Hülle über githubaccess.Tester.Test, denselben
// Test, den `k-playbook github test` ausgibt und den die Oberfläche auf
// Knopfdruck ruft (docs/github-access.md, Abschnitte Testing an access und
// MCP tool). Es ist das einzige Werkzeug dieses Servers, das GitHub fragt:
// mehrere Anfragen je Zugang, zehn Sekunden Frist je Anfrage. Es läuft nur
// auf diesen Aufruf hin — eine Sitzung, der gerade ein Push misslungen ist,
// fragt damit, welcher Zugang trägt, statt zu raten. Weder Briefing noch
// k_playbook_health rufen es.
//
// Kein Token verlässt den Prozess: Das Paket githubaccess hält es im Header
// seines HTTP-Clients und redigiert jede Meldung; die Antwort hier reicht den
// Report unverändert durch.
const githubTestTool = "k_playbook_github_test"

// githubTestInput trägt projectDir wie die übrigen Familien — die Datei ist
// persönlich und projectDir bestimmt kein Projekt, aber jedes Werkzeug trägt
// es nach dem Vertrag von mcp.md — und einen optionalen Dienst.
type githubTestInput struct {
	ProjectDir string `json:"projectDir,omitempty" jsonschema:"Pflicht. Verzeichnis, ab dem aufwärts nach K-PLAYBOOK.yaml gesucht wird. Relative Pfade werden relativ zum Arbeitsverzeichnis des MCP-Servers aufgelöst."`
	Service    string `json:"service,omitempty" jsonschema:"Optional. Der Dienst aus der Zugangsdatei, der getestet wird; ohne Angabe alle Dienste. Ein unbekannter Name ist ein Eingabefehler, dessen Meldung die bekannten nennt."`
}

// githubTestEnvelope ist die Antwortform. Der Report steht unter test — nicht
// eingebettet, denn sein ok (kein failed) ist ein anderes als das des
// Umschlags (Werkzeug gelaufen); bei einem Fehler fehlt er.
type githubTestEnvelope struct {
	OK         bool                    `json:"ok"`
	Tool       string                  `json:"tool"`
	ProjectDir string                  `json:"projectDir"`
	Test       *githubaccess.Report    `json:"test,omitempty"`
	Error      *knowledgeErrorEnvelope `json:"error,omitempty"`
}

// Die Fehlercodes des Werkzeugs neben invalid_input und project_not_found.
const (
	// githubErrConfig: Die Zugangsdatei hat mindestens ein Problem der Stufe
	// error; es gibt keine Konfiguration zu testen. Die Meldung nennt die
	// Probleme wie `k-playbook github check`.
	githubErrConfig = "github_config_invalid"
	// githubErrUnknownService: Der genannte Dienst steht nicht in der Datei.
	githubErrUnknownService = "unknown_service"
)

// newGitHubTester baut den Tester: die echte API, zehn Sekunden je Aufruf.
// Die Naht für die Tests, die einen httptest.Server einsetzen.
var newGitHubTester = githubaccess.NewTester

func addGitHubTestTool(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name: githubTestTool,
		Description: "Fragt GitHub, ob die Zugänge der persönlichen GitHub-Zugangsdatei ~/.config/k-playbook/github.yaml " +
			"funktionieren — für einen Dienst (service) oder alle. Das einzige Werkzeug mit Netz: mehrere Anfragen je " +
			"Zugang, zehn Sekunden Frist je Anfrage; nur auf diesen Aufruf hin, nie aus Briefing oder health. Liest nur, " +
			"pusht nie, ändert die Datei nicht. test.services: je Dienst repo und access, je Zugang result (ok, failed, " +
			"skipped — deaktiviert, nicht verfügbar), identity (Login, <owner>/<repo> für einen Deploy Key, anonymous), " +
			"read und write (yes, no, unknown), expires (Datum oder none), warning (Ablauf in weniger als 14 Tagen), " +
			"message (GitHubs Antwort bei failed, der Grund bei skipped); test.ok ist false, sobald ein Zugang failed ist. " +
			"Kein Token steht in der Antwort. Dieselbe Antwort wie `k-playbook github test [<dienst>] --json` unter test.",
	}, githubTestToolHandler)
}

func githubTestToolHandler(ctx context.Context, _ *mcp.CallToolRequest, input githubTestInput) (*mcp.CallToolResult, any, error) {
	projectDir, err := resolveProjectDir(input.ProjectDir)
	if err != nil {
		return jsonToolResult(githubTestEnvelope{OK: false, Tool: githubTestTool, ProjectDir: projectDir,
			Error: &knowledgeErrorEnvelope{Code: projectDirErrorCode(err), Message: err.Error()}}, true), nil, nil
	}
	fail := func(code string, message string) (*mcp.CallToolResult, any, error) {
		return jsonToolResult(githubTestEnvelope{OK: false, Tool: githubTestTool, ProjectDir: projectDir,
			Error: &knowledgeErrorEnvelope{Code: code, Message: message}}, true), nil, nil
	}

	path, err := githubaccess.Path()
	if err != nil {
		return fail(githubErrConfig, "Der Ort der GitHub-Zugangsdatei ist unbekannt: "+err.Error()+".")
	}
	result := githubaccess.Load(path)
	if result.HasErrors() {
		lines := make([]string, 0, len(result.Problems))
		for _, problem := range result.Problems {
			lines = append(lines, problem.Path+": "+problem.Message+" — "+problem.Hint)
		}
		return fail(githubErrConfig, "Die Zugangsdatei "+path+" hat Probleme, nichts getestet: "+strings.Join(lines, " | "))
	}
	services := result.Services
	if name := strings.TrimSpace(input.Service); name != "" {
		service, ok := services.Lookup(name)
		if !ok {
			known := "keiner"
			if names := services.Names(); len(names) > 0 {
				known = strings.Join(names, ", ")
			}
			return fail(githubErrUnknownService, "Unbekannter Dienst "+name+" in "+path+"; bekannt: "+known+".")
		}
		services = githubaccess.Services{service}
	}
	report := newGitHubTester().Test(ctx, path, services)
	return jsonToolResult(githubTestEnvelope{OK: true, Tool: githubTestTool, ProjectDir: projectDir, Test: &report}, false), nil, nil
}
