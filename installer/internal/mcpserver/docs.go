package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
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
	addUserDocTools(server)
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

// Die Werkzeuge für nummerierte Dokumente in der Collection des Projekts —
// Hüllen über project.UserDocs, dem Weg, den auch `k-playbook doc` geht. Die
// erlaubten Werte von kind kommen aus der Sorten-Tabelle project.DocKinds.
const (
	docToolCreate = "k_playbook_doc_create"
	docToolList   = "k_playbook_doc_list"
	docToolDone   = "k_playbook_doc_done"
	docToolReopen = "k_playbook_doc_reopen"
)

// openUserDocs ist austauschbar, damit Tests den Node nicht fragen.
var openUserDocs = project.OpenUserDocs

type docCreateInput struct {
	ProjectDir string `json:"projectDir,omitempty" jsonschema:"Pflicht. Verzeichnis, ab dem aufwärts nach K-PLAYBOOK.yaml gesucht wird. Relative Pfade werden relativ zum Arbeitsverzeichnis des MCP-Servers aufgelöst."`
	Kind       string `json:"kind" jsonschema:"Pflicht. Die Sorte des Dokuments."`
	Name       string `json:"name" jsonschema:"Pflicht. Der Kurzname; k-playbook macht ihn passend (Kleinbuchstaben, Bindestriche) und setzt die Nummer davor."`
}

type docListInput struct {
	ProjectDir string `json:"projectDir,omitempty" jsonschema:"Pflicht. Verzeichnis, ab dem aufwärts nach K-PLAYBOOK.yaml gesucht wird. Relative Pfade werden relativ zum Arbeitsverzeichnis des MCP-Servers aufgelöst."`
	Kind       string `json:"kind" jsonschema:"Pflicht. Die Sorte."`
	User       string `json:"user,omitempty" jsonschema:"Wessen Dokumente: leer der eigene User, * alle, sonst ein Username. Greift nur bei Sorten mit Userbereich."`
	Done       bool   `json:"done,omitempty" jsonschema:"Auch die Erledigten unter done/ zeigen."`
}

type docMoveInput struct {
	ProjectDir string `json:"projectDir,omitempty" jsonschema:"Pflicht. Verzeichnis, ab dem aufwärts nach K-PLAYBOOK.yaml gesucht wird. Relative Pfade werden relativ zum Arbeitsverzeichnis des MCP-Servers aufgelöst."`
	Ref        string `json:"ref" jsonschema:"Pflicht. Der keph:-Verweis auf ein eigenes Dokument, etwa keph:/user/~/tasks/100-name.md; ~ steht für den eigenen User."`
}

// docCreateEnvelope, docListEnvelope und docMoveEnvelope sind die
// Antwortformen; die Felder des Ergebnisses fehlen bei einem Fehler.
type docCreateEnvelope struct {
	OK         bool   `json:"ok"`
	Tool       string `json:"tool"`
	ProjectDir string `json:"projectDir"`
	*project.DocCreated
	Error *knowledgeErrorEnvelope `json:"error,omitempty"`
}

type docListEnvelope struct {
	OK         bool   `json:"ok"`
	Tool       string `json:"tool"`
	ProjectDir string `json:"projectDir"`
	*project.DocList
	Error *knowledgeErrorEnvelope `json:"error,omitempty"`
}

type docMoveEnvelope struct {
	OK         bool   `json:"ok"`
	Tool       string `json:"tool"`
	ProjectDir string `json:"projectDir"`
	*project.DocMoved
	Error *knowledgeErrorEnvelope `json:"error,omitempty"`
}

// docKindSchema bildet das Eingabeschema aus dem Typ und setzt die erlaubten
// Werte von kind aus der Sorten-Tabelle.
func docKindSchema[T any]() *jsonschema.Schema {
	schema, err := jsonschema.For[T](nil)
	if err != nil {
		panic(fmt.Sprintf("Schema für %T: %v", *new(T), err))
	}
	if prop, ok := schema.Properties["kind"]; ok {
		for _, name := range project.DocKindNames(project.DocKinds) {
			prop.Enum = append(prop.Enum, name)
		}
	}
	return schema
}

// docErrorCode liefert den Code eines Fehlers der doc-Werkzeuge; ohne Code
// ist es ein Fehler der Umgebung.
func docErrorCode(err error) string {
	if code := project.DocRefErrorCode(err); code != "" {
		return code
	}
	return "failed"
}

func addUserDocTools(server *mcp.Server) {
	kinds := strings.Join(project.DocKindNames(project.DocKinds), ", ")
	mcp.AddTool(server, &mcp.Tool{
		Name:        docToolCreate,
		InputSchema: docKindSchema[docCreateInput](),
		Description: "Legt ein leeres, nummeriertes Dokument einer Sorte (" + kinds + ") in der Kephalaion-Collection " +
			"des Projekts an: Nummer = höchste im Zählbereich plus eins, mindestens die Startnummer der Sorte, " +
			"dreistellig; bei name_taken die nächste. Antwort: ref (keph:-Verweis mit echtem Usernamen, bei Sorten " +
			"ohne Userbereich ohne User), kind, number, name, user. Den Inhalt schreibst du danach direkt über " +
			"Kephalaion (write mit collection = kephalaion.address). kephalaion_unavailable heißt: beim Hub ist nichts " +
			"angekommen (Anbindung nicht bereit, Node nicht erreichbar oder älter als v0.3.1 — dann `kephalaion " +
			"upgrade`), kein Rückfall auf die Platte. Jeder Aufruf trägt einen eigenen request_id und wird nach offenem " +
			"Ausgang einmal mit demselben Schlüssel wiederholt; bleibt er offen: outcome_unknown — dann nicht noch " +
			"einmal aufrufen, sondern mit k_playbook_doc_list nachsehen. Hülle über `k-playbook doc create`.",
	}, docCreateTool)
	mcp.AddTool(server, &mcp.Tool{
		Name:        docToolList,
		InputSchema: docKindSchema[docListInput](),
		Description: "Listet die nummerierten Dokumente einer Sorte (" + kinds + ") aus der Replica: standardmäßig " +
			"nur die eigenen, mit user * alle, mit done auch die Erledigten. Bei Sorten ohne Userbereich greift user " +
			"nicht; der User kommt dann aus den Autor-Metadaten. Jeder Eintrag trägt ref, title (Frontmatter title, " +
			"sonst Dateiname), user, created, updated, done. Was ein anderer Rechner gerade angelegt hat, kann fehlen, " +
			"bis die Replica abgeglichen ist; Eigenes steht sofort da. Hülle über " +
			"`k-playbook doc list`.",
	}, docListTool)
	mcp.AddTool(server, &mcp.Tool{
		Name:        docToolDone,
		InputSchema: docKindSchema[docMoveInput](),
		Description: "Verschiebt ein eigenes Dokument aus seiner Ablage nach done/ (Kephalaion rename über den Hub, " +
			"die id bleibt). ref ist ein keph:-Verweis, ~ steht für den eigenen User. Abgelehnt: fremder User " +
			"(foreign_user), Sorte ohne done/ (done_unsupported), Dokument schon in done/ oder in keiner Ablage " +
			"(wrong_dir); belegtes Ziel: name_taken, nichts überschrieben. kephalaion_unavailable und outcome_unknown " +
			"wie bei k_playbook_doc_create: bei outcome_unknown nicht wiederholen, sondern mit k_playbook_doc_list " +
			"(done: true) nachsehen. Antwort: ref (neu, mit echtem Namen), from, kind. Hülle über `k-playbook doc done`.",
	}, docDoneTool)
	mcp.AddTool(server, &mcp.Tool{
		Name:        docToolReopen,
		InputSchema: docKindSchema[docMoveInput](),
		Description: "Holt ein eigenes Dokument aus done/ zurück in seine Ablage — der umgekehrte Weg von " +
			"k_playbook_doc_done, mit denselben Ablehnungen (wrong_dir, wenn es nicht in done/ liegt). Hülle über " +
			"`k-playbook doc reopen`.",
	}, docReopenTool)
}

func docCreateTool(ctx context.Context, _ *mcp.CallToolRequest, input docCreateInput) (*mcp.CallToolResult, any, error) {
	fail := func(dir string, code string, err error) *mcp.CallToolResult {
		return jsonToolResult(docCreateEnvelope{OK: false, Tool: docToolCreate, ProjectDir: dir,
			Error: &knowledgeErrorEnvelope{Code: code, Message: err.Error()}}, true)
	}
	projectDir, err := resolveProjectDir(input.ProjectDir)
	if err != nil {
		return fail(projectDir, projectDirErrorCode(err), err), nil, nil
	}
	docs, err := openUserDocs(projectDir)
	if err != nil {
		return fail(projectDir, docErrorCode(err), err), nil, nil
	}
	created, err := docs.Create(ctx, input.Kind, input.Name)
	if err != nil {
		return fail(projectDir, docErrorCode(err), err), nil, nil
	}
	return jsonToolResult(docCreateEnvelope{OK: true, Tool: docToolCreate, ProjectDir: projectDir, DocCreated: &created}, false), nil, nil
}

func docListTool(ctx context.Context, _ *mcp.CallToolRequest, input docListInput) (*mcp.CallToolResult, any, error) {
	fail := func(dir string, code string, err error) *mcp.CallToolResult {
		return jsonToolResult(docListEnvelope{OK: false, Tool: docToolList, ProjectDir: dir,
			Error: &knowledgeErrorEnvelope{Code: code, Message: err.Error()}}, true)
	}
	projectDir, err := resolveProjectDir(input.ProjectDir)
	if err != nil {
		return fail(projectDir, projectDirErrorCode(err), err), nil, nil
	}
	docs, err := openUserDocs(projectDir)
	if err != nil {
		return fail(projectDir, docErrorCode(err), err), nil, nil
	}
	list, err := docs.List(ctx, project.DocListOptions{Kind: input.Kind, User: input.User, Done: input.Done})
	if err != nil {
		return fail(projectDir, docErrorCode(err), err), nil, nil
	}
	return jsonToolResult(docListEnvelope{OK: true, Tool: docToolList, ProjectDir: projectDir, DocList: &list}, false), nil, nil
}

func docDoneTool(ctx context.Context, _ *mcp.CallToolRequest, input docMoveInput) (*mcp.CallToolResult, any, error) {
	return docMove(ctx, docToolDone, input, (*project.UserDocs).Done), nil, nil
}

func docReopenTool(ctx context.Context, _ *mcp.CallToolRequest, input docMoveInput) (*mcp.CallToolResult, any, error) {
	return docMove(ctx, docToolReopen, input, (*project.UserDocs).Reopen), nil, nil
}

func docMove(ctx context.Context, tool string, input docMoveInput,
	move func(*project.UserDocs, context.Context, string) (project.DocMoved, error)) *mcp.CallToolResult {
	fail := func(dir string, code string, err error) *mcp.CallToolResult {
		return jsonToolResult(docMoveEnvelope{OK: false, Tool: tool, ProjectDir: dir,
			Error: &knowledgeErrorEnvelope{Code: code, Message: err.Error()}}, true)
	}
	projectDir, err := resolveProjectDir(input.ProjectDir)
	if err != nil {
		return fail(projectDir, projectDirErrorCode(err), err)
	}
	docs, err := openUserDocs(projectDir)
	if err != nil {
		return fail(projectDir, docErrorCode(err), err)
	}
	moved, err := move(docs, ctx, input.Ref)
	if err != nil {
		return fail(projectDir, docErrorCode(err), err)
	}
	return jsonToolResult(docMoveEnvelope{OK: true, Tool: tool, ProjectDir: projectDir, DocMoved: &moved}, false)
}
