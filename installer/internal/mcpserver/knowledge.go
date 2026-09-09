package mcpserver

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// Die fünf Wissens-Werkzeuge sind dünne Hüllen über project.Knowledge, dem
// Zugriffsweg, den auch das Subkommando `k-playbook knowledge` benutzt.
// Zweite Fachlogik gibt es hier nicht: gesucht, gechunkt und indiziert wird
// ausschließlich in project/.
//
// Tragende Schicht ist das Subkommando: der MCP-Server wird pro Projekt
// registriert und kann fehlen, das Binary ist mit der Installation da.
const (
	knowledgeToolSearch = "k_playbook_knowledge_search"
	knowledgeToolList   = "k_playbook_knowledge_list"
	knowledgeToolRead   = "k_playbook_knowledge_read"
	knowledgeToolWrite  = "k_playbook_knowledge_write"
	knowledgeToolStatus = "k_playbook_knowledge_status"
)

type knowledgeBaseInput struct {
	ProjectDir string `json:"projectDir" jsonschema:"Pflicht. Verzeichnis, ab dem aufwärts nach K-PLAYBOOK.yaml gesucht wird. Relative Pfade werden relativ zum Arbeitsverzeichnis des MCP-Servers aufgelöst."`
}

type knowledgeSearchInput struct {
	knowledgeBaseInput
	Query  string `json:"query" jsonschema:"Pflicht. Die Suchanfrage, Volltext über alle Abschnitte des Wissensverzeichnisses."`
	Source string `json:"source,omitempty" jsonschema:"Grenzt auf eine Herkunft ein: code, libs, extracted, versions, manual, learned oder root. Ohne Angabe alle."`
	Limit  int    `json:"limit,omitempty" jsonschema:"Höchstzahl der Treffer. Ohne Angabe 10."`
}

type knowledgeListInput struct {
	knowledgeBaseInput
	Source string `json:"source,omitempty" jsonschema:"Grenzt auf eine Herkunft ein: code, libs, extracted, versions, manual, learned oder root. Ohne Angabe alle."`
}

type knowledgeReadInput struct {
	knowledgeBaseInput
	Path string `json:"path" jsonschema:"Pflicht. Pfad relativ zu k-playbook-local/docs/, so wie search und list ihn nennen, etwa manual/ablauf.md oder README.md."`
}

type knowledgeWriteInput struct {
	knowledgeBaseInput
	Path    string `json:"path" jsonschema:"Pflicht. Pfad relativ zu k-playbook-local/docs/learned/ — geschrieben wird ausschließlich dorthin, jeder Pfad, der herausführt, wird abgewiesen. Eine vorhandene Datei wird ersetzt."`
	Content string `json:"content" jsonschema:"Pflicht. Der Inhalt als Markdown."`
	Source  string `json:"source" jsonschema:"Pflicht. Einzeiliger Herkunftsvermerk, etwa \"Sitzung 2026-09-09\" oder \"Task 055\"; landet als source im Frontmatter."`
}

type knowledgeStatusInput struct {
	knowledgeBaseInput
}

// knowledgeEnvelope ist die Antwortform aller fünf Werkzeuge. Die Felder
// tragen dieselben Namen wie die Antworten von `k-playbook knowledge --json`.
//
// Hits und Entries sind Zeiger auf Scheiben, wie Content und Status: nur das
// Werkzeug, das sie beantwortet, setzt sie. Eine trefferlose Suche antwortet
// deshalb mit "hits": [] und nicht mit einer Antwort, in der der Schlüssel
// fehlt — das Subkommando tut dasselbe, und ein Aufrufer, der hits.length
// liest, soll nicht am leeren Ergebnis auflaufen. Umgekehrt trägt eine Antwort
// von read, write oder status den Schlüssel gar nicht erst, statt ihn auf null
// zu setzen und einen leeren Treffer vorzutäuschen.
//
// Hint meldet, was der Zugriff übergehen musste — ein nicht beschreibbares
// cache/, eine unlesbare Datei. Es ist kein Fehler: die Antwort steht.
type knowledgeEnvelope struct {
	OK         bool                      `json:"ok"`
	Tool       string                    `json:"tool"`
	ProjectDir string                    `json:"projectDir"`
	Query      string                    `json:"query,omitempty"`
	Source     string                    `json:"source,omitempty"`
	Hits       *[]project.Hit            `json:"hits,omitempty"`
	Entries    *[]project.KnowledgeEntry `json:"entries,omitempty"`
	Path       string                    `json:"path,omitempty"`
	Content    *string                   `json:"content,omitempty"`
	Written    bool                      `json:"written,omitempty"`
	Status     *project.KnowledgeStatus  `json:"status,omitempty"`
	Hint       string                    `json:"hint,omitempty"`
	Error      *knowledgeErrorEnvelope   `json:"error,omitempty"`
}

type knowledgeErrorEnvelope struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func addKnowledgeTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name: knowledgeToolSearch,
		Description: "Durchsucht das Wissensverzeichnis k-playbook-local/docs/ des Projekts abschnittsweise. " +
			"Jeder Treffer trägt path, heading, excerpt, source, rank (ab 1, die Reihenfolge ist die Aussage) " +
			"und anchor für den Sprung in der Oberfläche. Hülle über `k-playbook knowledge search`.",
	}, knowledgeSearchTool)
	mcp.AddTool(server, &mcp.Tool{
		Name: knowledgeToolList,
		Description: "Listet die Dateien des Wissensverzeichnisses k-playbook-local/docs/ mit path, title und source; " +
			"die README steht vorn. Hülle über `k-playbook knowledge list`.",
	}, knowledgeListTool)
	mcp.AddTool(server, &mcp.Tool{
		Name: knowledgeToolRead,
		Description: "Liest eine Datei des Wissensverzeichnisses k-playbook-local/docs/ als Markdown. " +
			"Hülle über `k-playbook knowledge read`.",
	}, knowledgeReadTool)
	mcp.AddTool(server, &mcp.Tool{
		Name: knowledgeToolWrite,
		Description: "Schreibt ein Markdown-Dokument nach k-playbook-local/docs/learned/ — nur dorthin, " +
			"die übrigen Herkunftsordner gehören den Generatoren. source geht als Herkunftsvermerk ins Frontmatter, " +
			"der Suchindex wird im selben Zug nachgezogen. Hülle über `k-playbook knowledge write`.",
	}, knowledgeWriteTool)
	mcp.AddTool(server, &mcp.Tool{
		Name: knowledgeToolStatus,
		Description: "Meldet Art und Größe des Suchindex über k-playbook-local/docs/: indexKind, fileCount, chunkCount, " +
			"bySource, builtAt sowie stale/staleFiles, wenn der letzte Zugriff am Tor vorbei geänderte Dateien " +
			"neu eingelesen hat. Hülle über `k-playbook knowledge status`.",
	}, knowledgeStatusTool)
}

func knowledgeSearchTool(ctx context.Context, req *mcp.CallToolRequest, input knowledgeSearchInput) (*mcp.CallToolResult, any, error) {
	return wrapKnowledgeTool(knowledgeToolSearch, input.ProjectDir, func(projectDir string) *mcp.CallToolResult {
		query := strings.TrimSpace(input.Query)
		source := strings.TrimSpace(input.Source)
		knowledge := project.NewKnowledge(projectDir)
		hits, err := knowledge.Search(query, project.KnowledgeFilter{Source: source}, input.Limit)
		if err != nil {
			return knowledgeErrorResult(knowledgeToolSearch, projectDir, "invalid_input", err)
		}
		return knowledgeResult(knowledgeEnvelope{
			OK: true, Tool: knowledgeToolSearch, ProjectDir: projectDir,
			Query: query, Source: source, Hits: &hits, Hint: knowledgeHint(knowledge),
		}, false)
	}), nil, nil
}

func knowledgeListTool(ctx context.Context, req *mcp.CallToolRequest, input knowledgeListInput) (*mcp.CallToolResult, any, error) {
	return wrapKnowledgeTool(knowledgeToolList, input.ProjectDir, func(projectDir string) *mcp.CallToolResult {
		source := strings.TrimSpace(input.Source)
		knowledge := project.NewKnowledge(projectDir)
		entries, err := knowledge.List(project.KnowledgeFilter{Source: source})
		if err != nil {
			return knowledgeErrorResult(knowledgeToolList, projectDir, "read_failed", err)
		}
		return knowledgeResult(knowledgeEnvelope{
			OK: true, Tool: knowledgeToolList, ProjectDir: projectDir,
			Source: source, Entries: &entries, Hint: knowledgeHint(knowledge),
		}, false)
	}), nil, nil
}

func knowledgeReadTool(ctx context.Context, req *mcp.CallToolRequest, input knowledgeReadInput) (*mcp.CallToolResult, any, error) {
	return wrapKnowledgeTool(knowledgeToolRead, input.ProjectDir, func(projectDir string) *mcp.CallToolResult {
		content, err := project.NewKnowledge(projectDir).Read(input.Path)
		if err != nil {
			return knowledgeErrorResult(knowledgeToolRead, projectDir, "invalid_input", err)
		}
		return knowledgeResult(knowledgeEnvelope{
			OK: true, Tool: knowledgeToolRead, ProjectDir: projectDir,
			Path: filepath.ToSlash(input.Path), Content: &content,
		}, false)
	}), nil, nil
}

func knowledgeWriteTool(ctx context.Context, req *mcp.CallToolRequest, input knowledgeWriteInput) (*mcp.CallToolResult, any, error) {
	return wrapKnowledgeTool(knowledgeToolWrite, input.ProjectDir, func(projectDir string) *mcp.CallToolResult {
		if strings.TrimSpace(input.Content) == "" {
			return knowledgeErrorResult(knowledgeToolWrite, projectDir, "invalid_input",
				fmt.Errorf("leerer Inhalt: content muss Markdown enthalten"))
		}
		knowledge := project.NewKnowledge(projectDir)
		// Der gemeldete Pfad kommt aus Write: er ist der Ort relativ zum
		// Wissensverzeichnis, also mit learned/ davor — so, wie read und
		// search ihn nennen. Nachgebaut wird er hier nicht.
		rel, err := knowledge.Write(input.Path, input.Content, input.Source)
		if err != nil {
			return knowledgeErrorResult(knowledgeToolWrite, projectDir, "invalid_input", err)
		}
		return knowledgeResult(knowledgeEnvelope{
			OK: true, Tool: knowledgeToolWrite, ProjectDir: projectDir,
			Path: rel, Source: strings.TrimSpace(input.Source), Written: true,
			Hint: knowledgeHint(knowledge),
		}, false)
	}), nil, nil
}

func knowledgeStatusTool(ctx context.Context, req *mcp.CallToolRequest, input knowledgeStatusInput) (*mcp.CallToolResult, any, error) {
	return wrapKnowledgeTool(knowledgeToolStatus, input.ProjectDir, func(projectDir string) *mcp.CallToolResult {
		knowledge := project.NewKnowledge(projectDir)
		status, err := knowledge.Status()
		if err != nil {
			return knowledgeErrorResult(knowledgeToolStatus, projectDir, "read_failed", err)
		}
		return knowledgeResult(knowledgeEnvelope{
			OK: true, Tool: knowledgeToolStatus, ProjectDir: projectDir,
			Status: &status, Hint: knowledgeHint(knowledge),
		}, false)
	}), nil, nil
}

// wrapKnowledgeTool löst das Projekt auf und übergibt dessen
// Hauptverzeichnis. Aufgelöst wird über resolveProjectDir, gemeinsam mit den
// übrigen Werkzeugfamilien.
func wrapKnowledgeTool(tool string, inputDir string, fn func(projectDir string) *mcp.CallToolResult) *mcp.CallToolResult {
	projectDir, err := resolveProjectDir(inputDir)
	if err != nil {
		return knowledgeErrorResult(tool, projectDir, "project_not_found", err)
	}
	return fn(projectDir)
}

// knowledgeHint fasst zusammen, was der Zugriff übergehen musste. Leer, wenn
// nichts anlag.
func knowledgeHint(knowledge *project.Knowledge) string {
	return strings.Join(knowledge.Notes(), "; ")
}

func knowledgeErrorResult(tool string, projectDir string, code string, err error) *mcp.CallToolResult {
	return knowledgeResult(knowledgeEnvelope{
		OK:         false,
		Tool:       tool,
		ProjectDir: projectDir,
		Error:      &knowledgeErrorEnvelope{Code: code, Message: err.Error()},
	}, true)
}

func knowledgeResult(envelope knowledgeEnvelope, toolError bool) *mcp.CallToolResult {
	return jsonToolResult(envelope, toolError)
}
