package kephalaion

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Dokumente in der Collection des Projekts: list, create und rename über den
// Node dieses Rechners, mit dem Token aus der Anbindung. Der Vertrag ist der
// des Pakets: Hub und Account aus der Anbindung, das Token aus dem Pfad des
// Kandidaten, gesendet nur an die Loopback-Adresse des Nodes, kein Token in
// einem Ergebnis oder Fehler.
//
// list liest die Replica des Nodes; create und rename gehen über den Hub. Ein
// Fehler eines Werkzeugs kommt als *ToolError mit dem Code von Kephalaion
// (name_taken, not_found, …) zurück.

// ToolTimeout ist die Zeitgrenze für einen Aufruf von List, Create oder
// Rename samt Anmeldung. create und rename warten auf den Hub; ein Listing
// blättert womöglich mehrere Seiten.
const ToolTimeout = 15 * time.Second

// maxToolResponseBytes begrenzt jede Antwort eines Werkzeugs. Eine Seite von
// list mit Frontmatter kann groß werden; mehr wird nicht gelesen.
const maxToolResponseBytes = 16 << 20

// listPageLimit ist die Seitengröße von list (der Höchstwert in Kephalaion),
// maxListPages die Zahl der Seiten, nach der List aufgibt.
const (
	listPageLimit = 1000
	maxListPages  = 100
)

// Die Codes von Kephalaion, auf die k-playbook sich verlässt (error.code der
// Werkzeuge, die schreiben).
const (
	// CodeNameTaken: Unter dem Namen lebt schon ein Dokument (create) oder
	// das Ziel ist belegt (rename); nichts wurde geschrieben.
	CodeNameTaken = "name_taken"
	// CodeNotFound: Das Dokument gibt es nicht.
	CodeNotFound = "not_found"
)

// CodeNodeUnreachable ist kein Code von Kephalaion, sondern der dieses
// Pakets: Der Node war nicht erreichbar, die Antwort nicht lesbar, oder auf
// diesem Rechner fehlt, was ein Aufruf braucht (Adresse, Token). Nichts ist
// beim Hub angekommen — außer bei einer Zeitüberschreitung, deren Ausgang
// offen ist.
const CodeNodeUnreachable = "node_unreachable"

// ToolError ist der Fehler eines Werkzeugs von Kephalaion oder des Wegs
// dorthin. Code ist der Code von Kephalaion; leer, wenn das Werkzeug keinen
// nennt (list meldet Fehler nur als Text). Message enthält kein Token.
type ToolError struct {
	Tool    string
	Code    string
	Message string
}

// Error nennt Werkzeug und Meldung; den Code trägt das Feld Code.
func (e *ToolError) Error() string {
	return fmt.Sprintf("Kephalaion %s: %s", e.Tool, e.Message)
}

// ErrorCode liefert den Code eines *ToolError, leer bei jedem anderen Fehler.
func ErrorCode(err error) string {
	var toolErr *ToolError
	if errors.As(err, &toolErr) {
		return toolErr.Code
	}
	return ""
}

// Stamp ist ein Zeitpunkt mit dem User, der ihn gesetzt hat (Stamp in
// Kephalaion): angelegt oder geändert.
type Stamp struct {
	At string `json:"at"`
	By string `json:"by"`
}

// Entry ist ein Dokument aus list. Name ist der volle Name in der
// Collection. Frontmatter steht nur mit ListRequest.Frontmatter da, und nur
// bei .md-Dokumenten, deren Frontmatter sich lesen ließ.
type Entry struct {
	Kind             string         `json:"kind"`
	Name             string         `json:"name"`
	ID               string         `json:"id,omitempty"`
	Revision         int64          `json:"revision,omitempty"`
	Created          *Stamp         `json:"created,omitempty"`
	Updated          *Stamp         `json:"updated,omitempty"`
	Size             *int64         `json:"size,omitempty"`
	Frontmatter      map[string]any `json:"frontmatter,omitempty"`
	FrontmatterError string         `json:"frontmatter_error,omitempty"`
}

// ListRequest sind die Argumente von List. Path ist ein Verzeichnis der
// Collection ohne Schrägstrich am Ende, leer für die Wurzel. Ein Verzeichnis,
// das es nicht gibt, liefert eine leere Liste.
type ListRequest struct {
	Path        string
	Recursive   bool
	Frontmatter bool
}

// Document ist die Antwort von create und rename: das Dokument, wie der Hub
// es nach dem Vorgang meldet, bei rename unter dem neuen Namen. Note ist ein
// Hinweis des Nodes zu einem Erfolg (Replica nicht geschrieben).
type Document struct {
	Kind     string `json:"kind,omitempty"`
	Address  string `json:"address"`
	Name     string `json:"name"`
	ID       string `json:"id,omitempty"`
	Revision int64  `json:"revision,omitempty"`
	Created  *Stamp `json:"created,omitempty"`
	Updated  *Stamp `json:"updated,omitempty"`
	Size     *int64 `json:"size,omitempty"`
	Note     string `json:"note,omitempty"`
}

// Client ruft Werkzeuge des Nodes für eine Collection auf. Er hält das Token
// unexportiert; es gelangt in keine Antwort.
type Client struct {
	endpoint  string
	candidate Candidate
	token     string
	address   string
}

// NewClient baut einen Client für die Collection einer Anbindung. Geprüft
// werden die Namen, die Adresse des Nodes und das Token des Kandidaten; der
// Node wird hier nicht gefragt. Ob die Anbindung trägt, sagt Check.
func NewClient(binding Binding, env Env) (*Client, error) {
	fail := func(format string, args ...any) error {
		return &ToolError{Tool: "Anbindung", Code: CodeNodeUnreachable, Message: fmt.Sprintf(format, args...)}
	}
	if err := ValidateBinding(binding); err != nil {
		return nil, fail("Eintrag ungültig: %v", err)
	}
	if env.Endpoint == "" {
		return nil, fail("keine zulässige Adresse des Nodes")
	}
	candidate, ok := env.FindCandidate(binding.Hub, binding.Account)
	if !ok {
		return nil, fail("keine Token-Datei für Account %s am Hub %s", binding.Account, binding.Hub)
	}
	token, err := candidate.readToken()
	if err != nil {
		return nil, fail("Token-Datei für Account %s am Hub %s unbrauchbar", binding.Account, binding.Hub)
	}
	return &Client{endpoint: env.Endpoint, candidate: candidate, token: token, address: binding.Address()}, nil
}

// Address ist die Adresse der Collection, <hub>:<collection>.
func (c *Client) Address() string { return c.address }

// open meldet sich am Node an.
func (c *Client) open(ctx context.Context, tool string) (*toolSession, error) {
	session, err := openSession(ctx, newRPC(c.endpoint, maxToolResponseBytes, ToolTimeout), c.candidate, c.token)
	if err != nil {
		return nil, c.transportError(tool, err)
	}
	return session, nil
}

func (c *Client) transportError(tool string, err error) error {
	return &ToolError{Tool: tool, Code: CodeNodeUnreachable,
		Message: redact(fmt.Sprintf("Node unter %s: %v", c.endpoint, err), c.token)}
}

// call ruft ein Werkzeug auf und liest den strukturierten Inhalt nach out.
// Ein Ergebnis mit isError wird ein *ToolError: mit error.code, wenn die
// Struktur ihn trägt (create, rename), sonst ohne Code mit dem Text.
func (c *Client) call(ctx context.Context, session *toolSession, tool string, args map[string]any, out any) error {
	result, err := session.call(ctx, tool, args)
	if err != nil {
		return c.transportError(tool, err)
	}
	if result.IsError {
		toolErr := &ToolError{Tool: tool, Message: result.Text}
		var failure struct {
			Error *struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if len(result.Structured) > 0 && json.Unmarshal(result.Structured, &failure) == nil && failure.Error != nil {
			toolErr.Code = failure.Error.Code
			if failure.Error.Message != "" {
				toolErr.Message = failure.Error.Message
			}
		}
		if toolErr.Message == "" {
			toolErr.Message = "Fehler ohne Meldung"
		}
		toolErr.Message = redact(toolErr.Message, c.token)
		return toolErr
	}
	if len(result.Structured) == 0 {
		return &ToolError{Tool: tool, Code: CodeNodeUnreachable, Message: "Antwort ohne strukturierten Inhalt"}
	}
	if err := json.Unmarshal(result.Structured, out); err != nil {
		return &ToolError{Tool: tool, Code: CodeNodeUnreachable, Message: "Antwort nicht lesbar"}
	}
	return nil
}

// List listet die Dokumente unter req.Path aus der Replica des Nodes und
// blättert dabei alle Seiten durch. Verzeichnisse als Einträge (ohne
// recursive) werden übergangen.
func (c *Client) List(ctx context.Context, req ListRequest) ([]Entry, error) {
	ctx, cancel := context.WithTimeout(ctx, ToolTimeout)
	defer cancel()
	session, err := c.open(ctx, "list")
	if err != nil {
		return nil, err
	}
	entries := []Entry{}
	cursor := ""
	for page := 0; page < maxListPages; page++ {
		args := map[string]any{"collection": c.address, "path": req.Path, "limit": listPageLimit}
		if req.Recursive {
			args["recursive"] = true
		}
		if req.Frontmatter {
			args["frontmatter"] = true
		}
		if cursor != "" {
			args["cursor"] = cursor
		}
		var out struct {
			Entries []Entry `json:"entries"`
			More    bool    `json:"more"`
			Cursor  string  `json:"cursor"`
		}
		if err := c.call(ctx, session, "list", args, &out); err != nil {
			return nil, err
		}
		for _, e := range out.Entries {
			if e.Kind == "document" {
				entries = append(entries, e)
			}
		}
		if !out.More {
			return entries, nil
		}
		if out.Cursor == "" {
			return nil, &ToolError{Tool: "list", Code: CodeNodeUnreachable, Message: "weitere Einträge, aber kein cursor"}
		}
		cursor = out.Cursor
	}
	return nil, &ToolError{Tool: "list", Code: CodeNodeUnreachable,
		Message: fmt.Sprintf("mehr als %d Seiten; abgebrochen", maxListPages)}
}

// Create legt ein Dokument an, über den Hub. Lebt unter dem Namen schon ein
// Dokument, ist der Fehler ein *ToolError mit CodeNameTaken.
func (c *Client) Create(ctx context.Context, name, content string) (Document, error) {
	return c.write(ctx, "create", map[string]any{"collection": c.address, "name": name, "content": content})
}

// Rename benennt ein Dokument um, über den Hub; die id bleibt. Ist das Ziel
// belegt, ist der Fehler ein *ToolError mit CodeNameTaken, und nichts wird
// überschrieben.
func (c *Client) Rename(ctx context.Context, name, newName string) (Document, error) {
	return c.write(ctx, "rename", map[string]any{"collection": c.address, "name": name, "new_name": newName})
}

func (c *Client) write(ctx context.Context, tool string, args map[string]any) (Document, error) {
	ctx, cancel := context.WithTimeout(ctx, ToolTimeout)
	defer cancel()
	session, err := c.open(ctx, tool)
	if err != nil {
		return Document{}, err
	}
	var doc Document
	if err := c.call(ctx, session, tool, args, &doc); err != nil {
		return Document{}, err
	}
	return doc, nil
}
