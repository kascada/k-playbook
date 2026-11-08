package mcpserver

import (
	"context"
	"errors"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// Die Änderungs- und Suchwerkzeuge für Dokumente der Collection (Task 104) —
// Hüllen über project.UserDocs.Append, .Replace, .SetField und .Grep sowie
// project.GrepDocRef, dem Weg, den auch `k-playbook doc append|replace|set|grep`
// geht. Der Inhalt bleibt im Programm: Die Antwort nennt Verweis, Revision,
// Größe und ob geschrieben wurde, nicht den Text. Geltungsbereich: die Sorten
// ohne eigene Werkzeuge und, für append und replace, die Bereiche aus
// project.DocEditAreas (Handdoku unter docs/manual/).
const (
	docToolAppend   = "k_playbook_doc_append"
	docToolReplace  = "k_playbook_doc_replace"
	docToolSetField = "k_playbook_doc_set_field"
	docToolGrep     = "k_playbook_doc_grep"
)

const docSinceSchema = "Die Revision, die du zuletzt gesehen hast — aus deinem letzten Schreiben (revision der Antwort), " +
	"aus k_playbook_doc_read oder aus k_playbook_doc_grep mit count_only. Weicht das Dokument davon ab, hat ein anderer " +
	"geschrieben: foreign_change, nichts geschrieben. Ohne since_revision keine Vorbedingung."

// docEditKinds sind die Sorten, die die Änderungswerkzeuge erreichen, aus der
// Sorten-Tabelle project.DocKinds; docEditAreas die Bereiche außerhalb der
// Sorten, die append und replace erreichen (project.DocEditAreas).
var (
	docEditKinds = strings.Join(project.DocKindNames(project.DocKinds), ", ")
	docEditAreas = "keph:/" + strings.Join(project.DocEditAreaDirs(project.DocEditAreas), ", keph:/")
)

var docEditRefSchema = "Pflicht. Der keph:-Verweis auf ein Dokument der Sorten " + docEditKinds + ", offen oder " +
	"unter done/, bei Sorten mit Userbereich ein eigenes; ~ steht für den eigenen User, etwa keph:/user/~/tasks/100-name.md."

// docEditRefAreaSchema ergänzt die Beschreibung von ref um die Bereiche, für
// append und replace; docEditRefNoAreaSchema sagt set_field, dass sie dort
// nicht gelten.
var (
	docEditRefAreaSchema = docEditRefSchema + " Oder ein Dokument der Handdoku unter " + docEditAreas +
		" (auch in Unterverzeichnissen), etwa keph:/docs/manual/einrichtung.md — sie gehört allen, kein Userbereich."
	docEditRefNoAreaSchema = docEditRefSchema + " Die Handdoku unter " + docEditAreas + " erreicht set_field nicht (invalid_input)."
)

type docAppendInput struct {
	ProjectDir    string `json:"projectDir,omitempty" jsonschema:"Pflicht. Verzeichnis, ab dem aufwärts nach K-PLAYBOOK.yaml gesucht wird. Relative Pfade werden relativ zum Arbeitsverzeichnis des MCP-Servers aufgelöst."`
	Ref           string `json:"ref"`
	Content       string `json:"content" jsonschema:"Pflicht. Der Text, der angehängt wird; nicht leer."`
	SinceRevision int64  `json:"since_revision,omitempty"`
}

type docReplaceInput struct {
	ProjectDir    string `json:"projectDir,omitempty" jsonschema:"Pflicht. Verzeichnis, ab dem aufwärts nach K-PLAYBOOK.yaml gesucht wird. Relative Pfade werden relativ zum Arbeitsverzeichnis des MCP-Servers aufgelöst."`
	Ref           string `json:"ref"`
	Old           string `json:"old" jsonschema:"Pflicht. Der Text, der ersetzt wird; er muss genau einmal vorkommen."`
	New           string `json:"new" jsonschema:"Der neue Text; leer löscht old."`
	SinceRevision int64  `json:"since_revision,omitempty"`
}

type docSetFieldInput struct {
	ProjectDir    string  `json:"projectDir,omitempty" jsonschema:"Pflicht. Verzeichnis, ab dem aufwärts nach K-PLAYBOOK.yaml gesucht wird. Relative Pfade werden relativ zum Arbeitsverzeichnis des MCP-Servers aufgelöst."`
	Ref           string  `json:"ref"`
	Key           string  `json:"key" jsonschema:"Pflicht. Das Feld im Frontmatter, Form [a-z][a-z0-9_-]*."`
	Value         *string `json:"value,omitempty" jsonschema:"Der Wert, ein YAML-Skalar; k-playbook quotiert ihn, wenn nötig. Entweder value oder unset."`
	Unset         bool    `json:"unset,omitempty" jsonschema:"Das Feld entfernen. Entweder value oder unset."`
	SinceRevision int64   `json:"since_revision,omitempty"`
}

type docGrepInput struct {
	ProjectDir string `json:"projectDir,omitempty" jsonschema:"Pflicht. Verzeichnis, ab dem aufwärts nach K-PLAYBOOK.yaml gesucht wird. Relative Pfade werden relativ zum Arbeitsverzeichnis des MCP-Servers aufgelöst."`
	Pattern    string `json:"pattern" jsonschema:"Pflicht. Regulärer Ausdruck nach Go (RE2), zeilenweise."`
	Ref        string `json:"ref,omitempty" jsonschema:"Eine Einzeldatei wie bei k_playbook_doc_read (project:, k-playbook:, keph:). Entweder ref oder kind."`
	Kind       string `json:"kind,omitempty" jsonschema:"Alle Dokumente dieser Sorte durchsuchen, ausgewählt wie bei k_playbook_doc_list. Entweder ref oder kind."`
	User       string `json:"user,omitempty" jsonschema:"Nur mit kind: wessen Dokumente — leer der eigene User, * alle, sonst ein Username."`
	Done       bool   `json:"done,omitempty" jsonschema:"Nur mit kind: auch die Erledigten unter done/."`
	CountOnly  bool   `json:"count_only,omitempty" jsonschema:"Nur die Zahl der Treffer je Dokument (counts), mit revision bei keph: — der Weg zur Revision ohne Inhalt."`
}

// docEditEnvelope ist die Antwortform der Änderungswerkzeuge. Bei
// foreign_change trägt sie zum Fehler, wie das Dokument jetzt steht:
// revision, updatedBy, updatedAt und tail.
type docEditEnvelope struct {
	OK            bool           `json:"ok"`
	Tool          string         `json:"tool"`
	ProjectDir    string         `json:"projectDir"`
	Ref           string         `json:"ref,omitempty"`
	Revision      *int64         `json:"revision,omitempty"`
	Size          *int64         `json:"size,omitempty"`
	Changed       *bool          `json:"changed,omitempty"`
	SinceRevision int64          `json:"sinceRevision,omitempty"`
	UpdatedBy     string         `json:"updatedBy,omitempty"`
	UpdatedAt     string         `json:"updatedAt,omitempty"`
	Tail          *string        `json:"tail,omitempty"`
	Error         *errorEnvelope `json:"error,omitempty"`
}

type docGrepEnvelope struct {
	OK         bool   `json:"ok"`
	Tool       string `json:"tool"`
	ProjectDir string `json:"projectDir"`
	*project.DocGrepResult
	Error *errorEnvelope `json:"error,omitempty"`
}

var docEditCommon = " Abgelehnt: anderer Verweis als ein Dokument der Sorten " + docEditKinds + " (offen oder unter " +
	"done/) oder — nur bei append und replace — der Handdoku unter " + docEditAreas + " — invalid_input; fremder User — " +
	"foreign_user; Dokument fehlt — not_found. since_revision: weicht das " +
	"Dokument ab, ist das foreign_change — nichts geschrieben; die Antwort trägt revision, updatedBy, updatedAt und " +
	"tail (die letzten 20 Zeilen); dann nicht still nachziehen, sondern den Nutzer fragen, wer weitermacht. Ohne " +
	"since_revision wird nach stale_revision neu gelesen und bis zu dreimal wiederholt. Antwort: ref, revision, size, " +
	"changed (unveränderter Inhalt schreibt nichts). kephalaion_unavailable: beim Hub ist nichts angekommen. " +
	"outcome_unknown: die Änderung kann angekommen sein — dann nicht noch einmal ändern oder anhängen, sondern " +
	"nachlesen (k_playbook_doc_read oder k_playbook_doc_grep mit count_only)."

func addDocEditTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        docToolAppend,
		InputSchema: docEditSchema[docAppendInput](docEditRefAreaSchema),
		Description: "Hängt content an ein Dokument der Collection an — ein Dokument der Sorten " + docEditKinds +
			" oder der Handdoku unter " + docEditAreas + " —, ohne dass der Inhalt durch deinen Kontext läuft: " +
			"Endet das Dokument nicht mit einem Zeilenumbruch, kommt einer dazu, dann eine Leerzeile, dann der Text mit " +
			"genau einem Zeilenumbruch am Ende — jeder Eintrag beginnt am Zeilenanfang." + docEditCommon +
			" Hülle über `k-playbook doc append`.",
	}, docAppendTool)
	mcp.AddTool(server, &mcp.Tool{
		Name:        docToolReplace,
		InputSchema: docEditSchema[docReplaceInput](docEditRefAreaSchema),
		Description: "Ersetzt in einem Dokument der Collection — ein Dokument der Sorten " + docEditKinds +
			" oder der Handdoku unter " + docEditAreas + " — den Text old, der genau einmal vorkommen muss, durch new " +
			"(leer löscht), ohne dass der Inhalt durch deinen Kontext läuft; der Weg, eine Stelle einer Handbuchseite " +
			"nachzuziehen, statt sie ganz mit write neu zu schreiben. Kein Treffer: text_not_found; mehrere: " +
			"text_not_unique mit der Anzahl — dann mehr Umgebung in old mitgeben." + docEditCommon +
			" Hülle über `k-playbook doc replace`.",
	}, docReplaceTool)
	mcp.AddTool(server, &mcp.Tool{
		Name:        docToolSetField,
		InputSchema: docEditSchema[docSetFieldInput](docEditRefNoAreaSchema),
		Description: "Setzt ein Feld im Frontmatter eines Dokuments der Sorten " + docEditKinds + " (value, ein " +
			"YAML-Skalar, quotiert wenn nötig) oder entfernt es (unset); nicht für die Handdoku. Fehlt das Frontmatter, " +
			"wird beim Setzen eines angelegt. Andere Felder, ihre Reihenfolge und der Rumpf bleiben Byte für Byte." + docEditCommon +
			" Hülle über `k-playbook doc set`.",
	}, docSetFieldTool)
	mcp.AddTool(server, &mcp.Tool{
		Name:        docToolGrep,
		InputSchema: docKindSchema[docGrepInput](),
		Description: "Findet Zeilen nach einem Muster (RE2), ohne den Inhalt zu lesen: in einer Einzeldatei (ref, wie " +
			"bei k_playbook_doc_read — jeder keph:-Verweis, auch die Handdoku unter " + docEditAreas + ") oder in allen " +
			"Dokumenten einer Sorte (kind mit user und done wie bei " +
			"k_playbook_doc_list). Gesucht wird im gespeicherten Text, ohne eingesetzte Includes. Antwort: matches " +
			"(ref, line, text; fehlt ohne Treffer), count und bei einem keph:-ref revision; höchstens 200 Treffer, " +
			"darüber truncated: true. Mit count_only statt matches counts (ref, count, revision je Dokument, auch 0) — " +
			"so bekommst du die Revision für since_revision, ohne den Inhalt zu lesen. Hülle über `k-playbook doc grep`.",
	}, docGrepTool)
}

// docEditSchema ist das Eingabeschema der Änderungswerkzeuge mit den
// Beschreibungen von ref (refSchema, je nachdem, ob das Werkzeug die
// Bereiche erreicht) und since_revision.
func docEditSchema[T any](refSchema string) *jsonschema.Schema {
	schema := docKindSchema[T]()
	if prop, ok := schema.Properties["ref"]; ok {
		prop.Description = refSchema
	}
	if prop, ok := schema.Properties["since_revision"]; ok {
		prop.Description = docSinceSchema
	}
	return schema
}

func docAppendTool(ctx context.Context, _ *mcp.CallToolRequest, input docAppendInput) (*mcp.CallToolResult, any, error) {
	return docEdit(docToolAppend, input.ProjectDir, func(docs *project.UserDocs) (project.DocEdited, error) {
		return docs.Append(ctx, input.Ref, input.Content, project.DocEditOptions{SinceRevision: input.SinceRevision})
	}, input.Ref), nil, nil
}

func docReplaceTool(ctx context.Context, _ *mcp.CallToolRequest, input docReplaceInput) (*mcp.CallToolResult, any, error) {
	return docEdit(docToolReplace, input.ProjectDir, func(docs *project.UserDocs) (project.DocEdited, error) {
		return docs.Replace(ctx, input.Ref, input.Old, input.New, project.DocEditOptions{SinceRevision: input.SinceRevision})
	}, input.Ref), nil, nil
}

func docSetFieldTool(ctx context.Context, _ *mcp.CallToolRequest, input docSetFieldInput) (*mcp.CallToolResult, any, error) {
	if input.Unset == (input.Value != nil) {
		projectDir, _ := resolveProjectDir(input.ProjectDir)
		return docEditFailure(docToolSetField, projectDir, input.Ref, project.DocCodeInvalidInput,
			errors.New("entweder value oder unset angeben, genau eines")), nil, nil
	}
	return docEdit(docToolSetField, input.ProjectDir, func(docs *project.UserDocs) (project.DocEdited, error) {
		return docs.SetField(ctx, input.Ref, input.Key, input.Value, project.DocEditOptions{SinceRevision: input.SinceRevision})
	}, input.Ref), nil, nil
}

// docEditFailure ist die Fehlerantwort eines Änderungswerkzeugs; bei
// foreign_change mit dem Stand des Dokuments.
func docEditFailure(tool, projectDir, ref, code string, err error) *mcp.CallToolResult {
	envelope := docEditEnvelope{OK: false, Tool: tool, ProjectDir: projectDir, Ref: strings.TrimSpace(ref),
		Error: &errorEnvelope{Code: code, Message: err.Error()}}
	if change, ok := project.DocForeignChangeOf(err); ok {
		envelope.Ref = change.Ref
		envelope.Revision = &change.Revision
		envelope.SinceRevision = change.SinceRevision
		envelope.UpdatedBy, envelope.UpdatedAt = change.UpdatedBy, change.UpdatedAt
		envelope.Tail = &change.Tail
	}
	return jsonToolResult(envelope, true)
}

// docEdit ist der gemeinsame Ablauf der Änderungswerkzeuge: Projekt und
// Zugang auflösen, ändern, antworten.
func docEdit(tool, dir string, edit func(*project.UserDocs) (project.DocEdited, error), ref string) *mcp.CallToolResult {
	projectDir, err := resolveProjectDir(dir)
	if err != nil {
		return docEditFailure(tool, projectDir, ref, projectDirErrorCode(err), err)
	}
	docs, err := openUserDocs(projectDir)
	if err != nil {
		return docEditFailure(tool, projectDir, ref, docErrorCode(err), err)
	}
	edited, err := edit(docs)
	if err != nil {
		return docEditFailure(tool, projectDir, ref, docErrorCode(err), err)
	}
	return jsonToolResult(docEditEnvelope{OK: true, Tool: tool, ProjectDir: projectDir, Ref: edited.Ref,
		Revision: &edited.Revision, Size: &edited.Size, Changed: &edited.Changed}, false)
}

func docGrepTool(ctx context.Context, _ *mcp.CallToolRequest, input docGrepInput) (*mcp.CallToolResult, any, error) {
	fail := func(dir string, code string, err error) *mcp.CallToolResult {
		return jsonToolResult(docGrepEnvelope{OK: false, Tool: docToolGrep, ProjectDir: dir,
			Error: &errorEnvelope{Code: code, Message: err.Error()}}, true)
	}
	projectDir, err := resolveProjectDir(input.ProjectDir)
	if err != nil {
		return fail(projectDir, projectDirErrorCode(err), err), nil, nil
	}
	ref, kind := strings.TrimSpace(input.Ref), strings.TrimSpace(input.Kind)
	opts := project.DocGrepOptions{Pattern: input.Pattern, CountOnly: input.CountOnly}
	var result project.DocGrepResult
	switch {
	case (ref == "") == (kind == ""):
		return fail(projectDir, project.DocCodeInvalidInput, errors.New("entweder ref oder kind angeben, genau eines")), nil, nil
	case ref != "":
		if input.User != "" || input.Done {
			return fail(projectDir, project.DocCodeInvalidInput, errors.New("user und done gelten nur mit kind")), nil, nil
		}
		result, err = project.GrepDocRef(ctx, projectDocLayers(projectDir), ref, opts)
	default:
		var docs *project.UserDocs
		if docs, err = openUserDocs(projectDir); err == nil {
			result, err = docs.Grep(ctx, project.DocListOptions{Kind: kind, User: input.User, Done: input.Done}, opts)
		}
	}
	if err != nil {
		code := project.DocRefErrorCode(err)
		if code == "" {
			code = "read_failed"
		}
		return fail(projectDir, code, err), nil, nil
	}
	return jsonToolResult(docGrepEnvelope{OK: true, Tool: docToolGrep, ProjectDir: projectDir, DocGrepResult: &result}, false), nil, nil
}
