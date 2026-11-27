package mcpserver

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// Die drei Feedback-Werkzeuge (Task 198) sind dünne Hüllen über denselben
// Funktionen in project/, die auch `k-playbook feedback` benutzt. Über MCP
// meldet immer eine KI: von ist fest ki, client kommt aus der clientInfo der
// Initialisierung, wenn der Aufruf keinen nennt.
const (
	feedbackToolCreate = "k_playbook_feedback_create"
	feedbackToolList   = "k_playbook_feedback_list"
	feedbackToolUpdate = "k_playbook_feedback_update"
)

type feedbackCreateInput struct {
	ProjectDir    string `json:"projectDir,omitempty" jsonschema:"Pflicht. Verzeichnis, ab dem aufwärts nach K-PLAYBOOK.yaml gesucht wird. Relative Pfade werden relativ zum Arbeitsverzeichnis des MCP-Servers aufgelöst."`
	Title         string `json:"title" jsonschema:"Pflicht. Der Titel der Meldung in einer Zeile; das Werkzeug setzt ihn in doppelte Anführungszeichen ins Frontmatter."`
	Art           string `json:"art" jsonschema:"Pflicht. fehler oder wunsch."`
	Dringlichkeit string `json:"dringlichkeit" jsonschema:"Pflicht. hoch, mittel oder niedrig; hoch beim Fehler heißt: er blockiert die Arbeit."`
	Betrifft      string `json:"betrifft" jsonschema:"Pflicht. k-playbook, kephalaion oder projekt."`
	Body          string `json:"body" jsonschema:"Pflicht. Der Text der Meldung in Markdown, ohne Frontmatter und ohne Überschrift: was geschah, was erwartet war, wie es sich nachstellen lässt."`
	Sitzung       string `json:"sitzung" jsonschema:"Pflicht. Die Sitzungs-ID aus der Zeile Sitzung: des Briefings."`
	Modell        string `json:"modell" jsonschema:"Pflicht. Die Modell-ID der Sitzung, etwa claude-opus-5-5."`
	Client        string `json:"client,omitempty" jsonschema:"Der Client, etwa claude-code oder opencode. Ohne Angabe der Name aus der clientInfo der Initialisierung."`
}

type feedbackListInput struct {
	ProjectDir  string `json:"projectDir,omitempty" jsonschema:"Pflicht. Verzeichnis, ab dem aufwärts nach K-PLAYBOOK.yaml gesucht wird. Relative Pfade werden relativ zum Arbeitsverzeichnis des MCP-Servers aufgelöst."`
	IncludeDone bool   `json:"includeDone,omitempty" jsonschema:"Nimmt die erledigten und abgelehnten Meldungen unter feedback/done/ mit auf. Ohne Angabe nur die übrigen."`
	Art         string `json:"art,omitempty" jsonschema:"Nur Meldungen dieser Art: fehler oder wunsch."`
	Betrifft    string `json:"betrifft,omitempty" jsonschema:"Nur Meldungen mit diesem Betreff: k-playbook, kephalaion oder projekt."`
	Status      string `json:"status,omitempty" jsonschema:"Nur Meldungen mit diesem Status: offen, angenommen, erledigt oder abgelehnt; erledigt und abgelehnt suchen von selbst in feedback/done/."`
	// AllCollections fragt alle Hubs nach den lesbaren Collections und liest
	// in jeder feedback/; das dauert länger als die eigene Liste.
	AllCollections bool `json:"allCollections,omitempty" jsonschema:"Zählt zusätzlich in jeder anderen Collection aller Hubs, die du lesen darfst (ohne die eigene und die öffentliche k-playbook), die offenen Meldungen: collections mit address, open, byBetrifft und bei einem Lesefehler error. Die Filter gelten dafür nicht."`
}

type feedbackUpdateInput struct {
	ProjectDir    string `json:"projectDir,omitempty" jsonschema:"Pflicht. Verzeichnis, ab dem aufwärts nach K-PLAYBOOK.yaml gesucht wird. Relative Pfade werden relativ zum Arbeitsverzeichnis des MCP-Servers aufgelöst."`
	Ref           string `json:"ref,omitempty" jsonschema:"Der keph:-Verweis der Meldung, etwa keph:/feedback/003-sync-haengt.md. Entweder ref oder number."`
	Number        int    `json:"number,omitempty" jsonschema:"Die Nummer der Meldung. Entweder ref oder number; ist sie mehrdeutig, kommt invalid_input mit den Verweisen."`
	Status        string `json:"status" jsonschema:"Pflicht. offen, angenommen, erledigt oder abgelehnt. erledigt und abgelehnt verschieben nach feedback/done/, offen und angenommen holen von dort zurück."`
	SinceRevision int64  `json:"since_revision,omitempty" jsonschema:"Die Revision, die du zuletzt gesehen hast (revision aus list oder create). Weicht die Meldung davon ab, ist das foreign_change und nichts wird geschrieben. Ohne since_revision keine Vorbedingung."`
}

// feedbackEnvelope ist die Antwortform der drei Werkzeuge. create trägt
// ref, number, name, revision und feedback; list items (feedbackListEnvelope); update
// feedback, from und changed. Bei foreign_change stehen revision,
// sinceRevision und updatedAt der Meldung daneben.
type feedbackEnvelope struct {
	OK            bool              `json:"ok"`
	Tool          string            `json:"tool"`
	ProjectDir    string            `json:"projectDir"`
	Ref           string            `json:"ref,omitempty"`
	Number        int               `json:"number,omitempty"`
	Name          string            `json:"name,omitempty"`
	Revision      int64             `json:"revision,omitempty"`
	Feedback      *project.Feedback `json:"feedback,omitempty"`
	From          string            `json:"from,omitempty"`
	Changed       *bool             `json:"changed,omitempty"`
	SinceRevision int64             `json:"sinceRevision,omitempty"`
	UpdatedAt     string            `json:"updatedAt,omitempty"`
	Error         *errorEnvelope    `json:"error,omitempty"`
}

// feedbackHelp ist der gemeinsame Teil der Beschreibungen: Ablage und Codes.
const feedbackHelp = " Meldungen liegen als Dokumente in der Kephalaion-Collection des Projekts (feedback/, erledigt und " +
	"abgelehnt feedback/done/) und werden nur über diese Werkzeuge gelesen und geschrieben. kephalaion_unavailable " +
	"heißt: Kephalaion ist nicht erreichbar, es gibt dann keine Meldungen und keinen Rückfall auf die Platte."

// feedbackWriteHelp ergänzt die Beschreibungen der schreibenden Werkzeuge.
const feedbackWriteHelp = " outcome_unknown: Der Vorgang kann angekommen sein — nicht wiederholen, sondern mit " +
	"k_playbook_feedback_list (includeDone) nachsehen."

func addFeedbackTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name: feedbackToolCreate,
		Description: "Legt eine Meldung an — einen Fehler oder Wunsch, den jemand anderes erledigen soll (an k-playbook, " +
			"Kephalaion oder das Projekt) —, mit der nächsten Nummer und fertigem Inhalt in einem Zug. Du lieferst title, " +
			"art, dringlichkeit, betrifft, body, sitzung und modell; den Kopf baut das Werkzeug: von ki, client aus der " +
			"clientInfo (oder dem Parameter client), user und account vom Kephalaion-Node, die k-playbook-Version, status " +
			"offen. Ungültige Werte kommen gesammelt als invalid_input. Antwort: ref, number, name, revision, feedback." +
			feedbackHelp + feedbackWriteHelp + " Hülle über `k-playbook feedback create`.",
	}, feedbackCreateTool)
	mcp.AddTool(server, &mcp.Tool{
		Name: feedbackToolList,
		Description: "Listet die Meldungen der eigenen Collection nach Nummer, mit Kopf, Text und revision (items). Ohne " +
			"includeDone nur die unter feedback/; art, betrifft und status filtern. Mit allCollections dazu je anderer " +
			"lesbarer Collection aller Hubs die Zahl offener Meldungen, aufgeteilt nach betrifft (collections: address, " +
			"open, byBetrifft, error); eine Collection ohne feedback/ zählt 0, eine, die beim Lesen scheitert, steht mit " +
			"ihrem Fehler da. Wer liest, entscheiden allein die Leserechte in Kephalaion." + feedbackHelp +
			" Hülle über `k-playbook feedback list`.",
	}, feedbackListTool)
	mcp.AddTool(server, &mcp.Tool{
		Name: feedbackToolUpdate,
		Description: "Setzt den Status einer Meldung der eigenen Collection, gewählt über ref oder number: erledigt " +
			"und abgelehnt verschieben nach feedback/done/, offen und angenommen holen sie zurück; derselbe Status " +
			"schreibt nichts (changed false). Sonst bleibt das Dokument, wie es ist. since_revision: weicht die Meldung " +
			"ab, ist das foreign_change mit revision und updatedAt, nichts geschrieben — nicht still nachziehen, sondern " +
			"den Nutzer fragen. Antwort: feedback (mit neuem ref und revision), from, changed." + feedbackHelp +
			feedbackWriteHelp + " Hülle über `k-playbook feedback update`.",
	}, feedbackUpdateTool)
}

// feedbackFailure ist die Fehlerantwort; bei foreign_change mit dem Stand
// der Meldung.
func feedbackFailure(tool, projectDir, code string, err error) *mcp.CallToolResult {
	envelope := feedbackEnvelope{OK: false, Tool: tool, ProjectDir: projectDir,
		Error: &errorEnvelope{Code: code, Message: err.Error()}}
	if change, ok := project.DocForeignChangeOf(err); ok {
		envelope.Ref, envelope.Revision = change.Ref, change.Revision
		envelope.SinceRevision, envelope.UpdatedAt = change.SinceRevision, change.UpdatedAt
	}
	return jsonToolResult(envelope, true)
}

// feedbackClient ist der Client der Meldung: der Parameter, sonst der Name
// aus der clientInfo der Initialisierung.
func feedbackClient(req *mcp.CallToolRequest, input string) string {
	if client := strings.TrimSpace(input); client != "" {
		return client
	}
	if req != nil && req.Session != nil {
		if params := req.Session.InitializeParams(); params != nil && params.ClientInfo != nil {
			return strings.TrimSpace(params.ClientInfo.Name)
		}
	}
	return ""
}

func feedbackCreateTool(ctx context.Context, req *mcp.CallToolRequest, input feedbackCreateInput) (*mcp.CallToolResult, any, error) {
	projectDir, err := resolveProjectDir(input.ProjectDir)
	if err != nil {
		return feedbackFailure(feedbackToolCreate, projectDir, projectDirErrorCode(err), err), nil, nil
	}
	created, err := project.CreateFeedback(ctx, projectDir, project.FeedbackCreateOptions{
		Title: input.Title, Art: input.Art, Dringlichkeit: input.Dringlichkeit, Betrifft: input.Betrifft,
		Body: input.Body, Von: project.FeedbackVonKI, Client: feedbackClient(req, input.Client),
		Modell: input.Modell, Sitzung: input.Sitzung,
	})
	if err != nil {
		return feedbackFailure(feedbackToolCreate, projectDir, docErrorCode(err), err), nil, nil
	}
	return jsonToolResult(feedbackEnvelope{OK: true, Tool: feedbackToolCreate, ProjectDir: projectDir,
		Ref: created.Ref, Number: created.Number, Name: created.Name, Revision: created.Revision,
		Feedback: &created.Feedback}, false), nil, nil
}

func feedbackListTool(ctx context.Context, _ *mcp.CallToolRequest, input feedbackListInput) (*mcp.CallToolResult, any, error) {
	projectDir, err := resolveProjectDir(input.ProjectDir)
	if err != nil {
		return feedbackFailure(feedbackToolList, projectDir, projectDirErrorCode(err), err), nil, nil
	}
	listing, err := project.ListFeedback(ctx, projectDir, project.FeedbackListOptions{
		IncludeDone: input.IncludeDone, Art: input.Art, Betrifft: input.Betrifft, Status: input.Status,
		AllCollections: input.AllCollections,
	})
	if err != nil {
		return feedbackFailure(feedbackToolList, projectDir, docErrorCode(err), err), nil, nil
	}
	envelope := feedbackEnvelope{OK: true, Tool: feedbackToolList, ProjectDir: projectDir}
	return jsonToolResult(feedbackListEnvelope{feedbackEnvelope: envelope, FeedbackListing: listing}, false), nil, nil
}

// feedbackListEnvelope ist die Antwort von list: items auch leer als [],
// mit allCollections dazu collections bzw. collectionsError.
type feedbackListEnvelope struct {
	feedbackEnvelope
	project.FeedbackListing
}

func feedbackUpdateTool(ctx context.Context, _ *mcp.CallToolRequest, input feedbackUpdateInput) (*mcp.CallToolResult, any, error) {
	projectDir, err := resolveProjectDir(input.ProjectDir)
	if err != nil {
		return feedbackFailure(feedbackToolUpdate, projectDir, projectDirErrorCode(err), err), nil, nil
	}
	ref := strings.TrimSpace(input.Ref)
	if (ref == "") == (input.Number == 0) {
		return feedbackFailure(feedbackToolUpdate, projectDir, project.DocCodeInvalidInput,
			errors.New("entweder ref oder number angeben, genau eines")), nil, nil
	}
	target := ref
	if target == "" {
		target = strconv.Itoa(input.Number)
	}
	updated, err := project.UpdateFeedback(ctx, projectDir, target, input.Status, input.SinceRevision)
	if err != nil {
		return feedbackFailure(feedbackToolUpdate, projectDir, docErrorCode(err), err), nil, nil
	}
	return jsonToolResult(feedbackEnvelope{OK: true, Tool: feedbackToolUpdate, ProjectDir: projectDir,
		Feedback: &updated.Feedback, From: updated.From, Changed: &updated.Changed}, false), nil, nil
}
