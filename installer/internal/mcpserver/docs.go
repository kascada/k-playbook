package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// k_playbook_doc_read ist eine dünne Hülle über project.ResolveDocRef, dem
// Zugriffsweg, den auch das Subkommando `k-playbook read` benutzt. Der
// Verweis im Präfixschema (docs/verweisschema.md) ist der dauerhafte Vertrag
// des Werkzeugs: welche Quelle dahinter liegt — heute die Platte —, sieht der
// Aufrufer nicht.
const docToolRead = "k_playbook_doc_read"

// docReadInput trägt projectDir wie die übrigen Familien: Pflicht im Vertrag,
// optional im Schema, geprüft in checkProjectDir.
type docReadInput struct {
	ProjectDir string `json:"projectDir,omitempty" jsonschema:"Pflicht. Verzeichnis, ab dem aufwärts nach K-PLAYBOOK.yaml gesucht wird. Relative Pfade werden relativ zum Arbeitsverzeichnis des MCP-Servers aufgelöst."`
	Ref        string `json:"ref" jsonschema:"Pflicht. Der Verweis im Präfixschema, so wie er im Text steht, etwa k-playbook:/instructions/on-demand/thema.md für eine Datei oder k-playbook:/instructions/always/ (mit / am Ende) für das zusammengebaute Verzeichnis — das einzige; jeder andere Verweis mit / am Ende, etwa k-playbook:/instructions/on-demand/, ist invalid_input, weil der Dateiname fehlt. Aufgelöst werden heute nur k-playbook:-Verweise unter instructions/; project: und keph: werden als unsupported gemeldet."`
}

// docEnvelope ist die Antwortform von k_playbook_doc_read. Die Felder der
// Antwort sind Zeiger, damit eine Fehlerantwort sie gar nicht erst trägt,
// eine Erfolgsantwort aber immer — auch content "" und parts [].
type docEnvelope struct {
	OK         bool                    `json:"ok"`
	Tool       string                  `json:"tool"`
	ProjectDir string                  `json:"projectDir"`
	Ref        string                  `json:"ref,omitempty"`
	Directory  *bool                   `json:"directory,omitempty"`
	Content    *string                 `json:"content,omitempty"`
	Disabled   *bool                   `json:"disabled,omitempty"`
	Hint       string                  `json:"hint,omitempty"`
	Parts      *[]project.DocPart      `json:"parts,omitempty"`
	Problems   *[]string               `json:"problems,omitempty"`
	Error      *knowledgeErrorEnvelope `json:"error,omitempty"`
}

func addDocTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name: docToolRead,
		Description: "Liest, was hinter einem k-playbook:-Verweis steht — lokal und mitgeliefert schon gegeneinander " +
			"aufgelöst: eine lokale Datei ersetzt die gleichnamige mitgelieferte, eine leere schaltet sie ab. " +
			"Eine Datei kommt unverändert mit Frontmatter; k-playbook:/instructions/always/ (mit / am Ende) baut das " +
			"Verzeichnis zu einem Text zusammen (Teile mit Ziffer vorn, numerisch sortiert, ohne Frontmatter) — andere " +
			"Verzeichnisse liest du einzeln mit Dateinamen, mit / am Ende sind sie invalid_input. Die Antwort trägt content, " +
			"disabled (abgeschaltete Datei: leer, kein Fehler, mit hint), parts (name, origin, disabled) und problems " +
			"(übergangene Dateien). Ein unbekannter Verweis ist ein Fehler (not_found) — dann nicht selbst auf der " +
			"Platte suchen. Hülle über `k-playbook read`.",
	}, docReadTool)
}

func docReadTool(ctx context.Context, req *mcp.CallToolRequest, input docReadInput) (*mcp.CallToolResult, any, error) {
	projectDir, err := resolveProjectDir(input.ProjectDir)
	if err != nil {
		return docErrorResult(projectDir, input.Ref, projectDirErrorCode(err), err), nil, nil
	}
	result, err := project.ResolveDocRef(project.ProjectDocLayers(projectDir), input.Ref)
	if err != nil {
		code := project.DocRefErrorCode(err)
		if code == "" {
			code = "read_failed"
		}
		return docErrorResult(projectDir, input.Ref, code, err), nil, nil
	}
	return jsonToolResult(docEnvelope{
		OK: true, Tool: docToolRead, ProjectDir: projectDir,
		Ref: result.Ref, Directory: &result.Directory, Content: &result.Content,
		Disabled: &result.Disabled, Hint: result.Hint, Parts: &result.Parts, Problems: &result.Problems,
	}, false), nil, nil
}

func docErrorResult(projectDir string, ref string, code string, err error) *mcp.CallToolResult {
	return jsonToolResult(docEnvelope{
		OK: false, Tool: docToolRead, ProjectDir: projectDir, Ref: ref,
		Error: &knowledgeErrorEnvelope{Code: code, Message: err.Error()},
	}, true)
}
