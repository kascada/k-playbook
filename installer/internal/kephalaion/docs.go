package kephalaion

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
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
//
// create und rename tragen je Aufruf einen eigenen request_id (Kephalaion ab
// v0.3.1, „Wiederholung mit Schlüssel“ in Kephalaions docs/vertrag.md). Bleibt
// der Ausgang nach dem Abschicken offen, schickt der Client denselben Aufruf
// mit demselben Schlüssel genau einmal in einer neuen Sitzung noch einmal; der
// Hub wiederholt einen schon ausgeführten Vorgang nicht, sondern antwortet
// replayed. Bleibt auch die Wiederholung offen, ist der Fehler
// CodeOutcomeUnknown. CodeNodeUnreachable heißt dagegen immer: Beim Hub ist
// nichts angekommen.

// ToolTimeout ist die Zeitgrenze für einen Aufruf von List oder für einen
// Versuch von Create oder Rename, jeweils samt Anmeldung. create und rename
// warten auf den Hub; ein Listing blättert womöglich mehrere Seiten. Eine
// Wiederholung bekommt ihre eigene Zeitgrenze.
const ToolTimeout = 15 * time.Second

// toolTimeout ist die Zeitgrenze, mit der gerechnet wird — ToolTimeout, in
// Tests kürzer.
var toolTimeout = ToolTimeout

// MinWriteNodeVersion ist die älteste Version des Nodes, die request_id an
// create und rename annimmt. Ein älterer lehnt den Aufruf laut ab und
// schreibt nichts.
const MinWriteNodeVersion = "v0.3.1"

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

// Die Codes der Werkzeuge des Nodes, die schreiben, über die des Vertrags
// hinaus (write error codes in Kephalaions docs/begriffe.md).
const (
	// CodeUnreachable: Der Node hat den Hub nicht erreicht; nichts gespeichert.
	CodeUnreachable = "unreachable"
	// CodeUnsupported: Der Hub kennt den Vorgang nicht, oder der Node kann den
	// Transport nicht; nichts gespeichert.
	CodeUnsupported = "unsupported"
	// CodeInternal: ein Fehler des Nodes selbst; nichts abgeschickt.
	CodeInternal = "internal"
	// CodeOutcomeUnknown: Der Ausgang ist offen — der Vorgang kann beim Hub
	// gespeichert sein. Der Node meldet ihn, wenn er abgeschickt, aber keine
	// brauchbare Antwort bekommen hat; dieses Paket meldet ihn, wenn auch die
	// Wiederholung mit demselben request_id nichts geklärt hat. Nicht noch
	// einmal schicken, sondern nachsehen (list).
	CodeOutcomeUnknown = "outcome_unknown"
)

// CodeNodeUnreachable ist kein Code von Kephalaion, sondern der dieses
// Pakets: Beim Hub ist nichts angekommen. Der Node war nicht erreichbar, die
// Sitzung ließ sich nicht öffnen, auf diesem Rechner fehlt, was ein Aufruf
// braucht (Adresse, Token), oder der Node ist zu alt für create und rename
// (MinWriteNodeVersion). Nach dem Abschicken von create oder rename entsteht
// dieser Code nie; dann ist der Ausgang offen (CodeOutcomeUnknown). Bei list
// steht er auch für eine unlesbare Antwort — list ändert nichts.
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
//
// Replayed heißt: Der Vorgang war schon ausgeführt (der erste Versuch hatte
// den Hub erreicht), diesmal ist nichts geschrieben. Dann stehen nur Name,
// Revision und IDs da — keine Art, keine Stempel; ID ist gesetzt, wenn IDs
// genau ein Dokument nennt.
type Document struct {
	Kind     string   `json:"kind,omitempty"`
	Address  string   `json:"address"`
	Name     string   `json:"name"`
	ID       string   `json:"id,omitempty"`
	Revision int64    `json:"revision,omitempty"`
	Created  *Stamp   `json:"created,omitempty"`
	Updated  *Stamp   `json:"updated,omitempty"`
	Size     *int64   `json:"size,omitempty"`
	Note     string   `json:"note,omitempty"`
	Replayed bool     `json:"replayed,omitempty"`
	IDs      []string `json:"ids,omitempty"`
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
	session, err := openSession(ctx, newRPC(c.endpoint, maxToolResponseBytes, toolTimeout), c.candidate, c.token)
	if err != nil {
		return nil, c.transportError(tool, err)
	}
	return session, nil
}

func (c *Client) transportError(tool string, err error) error {
	return &ToolError{Tool: tool, Code: CodeNodeUnreachable,
		Message: redact(fmt.Sprintf("Node unter %s: %v", c.endpoint, err), c.token)}
}

// call ruft ein Werkzeug auf und liest den strukturierten Inhalt nach out —
// für list, das nichts ändert. Ein Ergebnis mit isError wird ein *ToolError:
// mit error.code, wenn die Struktur ihn trägt, sonst ohne Code mit dem Text.
func (c *Client) call(ctx context.Context, session *toolSession, tool string, args map[string]any, out any) error {
	result, err := session.call(ctx, tool, args)
	if err != nil {
		return c.transportError(tool, err)
	}
	if result.IsError {
		return c.toolFailure(tool, result)
	}
	if len(result.Structured) == 0 {
		return &ToolError{Tool: tool, Code: CodeNodeUnreachable, Message: "Antwort ohne strukturierten Inhalt"}
	}
	if err := json.Unmarshal(result.Structured, out); err != nil {
		return &ToolError{Tool: tool, Code: CodeNodeUnreachable, Message: "Antwort nicht lesbar"}
	}
	return nil
}

// toolFailure macht aus einem Ergebnis mit isError einen *ToolError: mit
// error.code, wenn die Struktur ihn trägt (create, rename), sonst ohne Code
// mit dem Text.
func (c *Client) toolFailure(tool string, result toolResult) *ToolError {
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

// List listet die Dokumente unter req.Path aus der Replica des Nodes und
// blättert dabei alle Seiten durch. Verzeichnisse als Einträge (ohne
// recursive) werden übergangen.
func (c *Client) List(ctx context.Context, req ListRequest) ([]Entry, error) {
	ctx, cancel := context.WithTimeout(ctx, toolTimeout)
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
// Dokument, ist der Fehler ein *ToolError mit CodeNameTaken. Jeder Aufruf
// trägt einen neuen request_id: Ein zweiter Aufruf ist ein neuer Vorgang,
// auch mit demselben Namen. Ablauf und Codes wie im Paketkommentar.
func (c *Client) Create(ctx context.Context, name, content string) (Document, error) {
	return c.write(ctx, "create", name, map[string]any{"collection": c.address, "name": name, "content": content})
}

// Rename benennt ein Dokument um, über den Hub; die id bleibt. Ist das Ziel
// belegt, ist der Fehler ein *ToolError mit CodeNameTaken, und nichts wird
// überschrieben. request_id und Wiederholung wie bei Create.
func (c *Client) Rename(ctx context.Context, name, newName string) (Document, error) {
	return c.write(ctx, "rename", newName, map[string]any{"collection": c.address, "name": name, "new_name": newName})
}

// attemptOutcome ist der Ausgang eines Versuchs von create oder rename.
type attemptOutcome int

const (
	// attemptDone: Erfolg, auch replayed.
	attemptDone attemptOutcome = iota
	// attemptRejected: abgelehnt mit einem Code des Vertrags oder ohne Code;
	// der Hub hat nichts geschrieben.
	attemptRejected
	// attemptNotSent: Beim Hub ist nichts angekommen — die Sitzung ließ sich
	// nicht öffnen, die Versionsprüfung scheiterte, oder der Node meldet
	// unreachable, unsupported oder internal.
	attemptNotSent
	// attemptOpen: abgeschickt, Ausgang offen.
	attemptOpen
)

// write führt create oder rename aus: ein Versuch, nach offenem Ausgang genau
// eine Wiederholung mit demselben request_id und denselben Feldern in neuer
// Sitzung, jeder Versuch mit eigener Zeitgrenze. target ist der Name, den das
// Dokument danach trägt.
func (c *Client) write(ctx context.Context, tool, target string, args map[string]any) (Document, error) {
	args["request_id"] = rand.Text()
	doc, outcome, err := c.attempt(ctx, tool, target, args, true)
	if outcome != attemptOpen {
		return doc, err
	}
	first := err
	if ctx.Err() != nil {
		// Der Aufrufer hat aufgegeben, nachdem der Aufruf abgeschickt war:
		// keine Wiederholung.
		return Document{}, c.outcomeUnknown(tool, target, first, "keine Wiederholung, der Aufruf wurde abgebrochen")
	}
	doc, outcome, err = c.attempt(ctx, tool, target, args, false)
	switch outcome {
	case attemptDone, attemptRejected:
		return doc, err
	}
	return Document{}, c.outcomeUnknown(tool, target, first, "Wiederholung mit demselben request_id: "+errorMessage(err))
}

// attempt ist ein Versuch von create oder rename in eigener Sitzung mit
// eigener Zeitgrenze. checkVersion prüft vorher die Version des Nodes; das
// braucht nur der erste Versuch.
func (c *Client) attempt(ctx context.Context, tool, target string, args map[string]any, checkVersion bool) (Document, attemptOutcome, error) {
	ctx, cancel := context.WithTimeout(ctx, toolTimeout)
	defer cancel()
	session, err := c.open(ctx, tool)
	if err != nil {
		return Document{}, attemptNotSent, err
	}
	if checkVersion {
		if err := c.checkNodeVersion(ctx, session, tool); err != nil {
			return Document{}, attemptNotSent, err
		}
	}
	if err := ctx.Err(); err != nil {
		return Document{}, attemptNotSent, &ToolError{Tool: tool, Code: CodeNodeUnreachable,
			Message: "abgebrochen, bevor der Aufruf abgeschickt war"}
	}

	// Ab hier ist der Aufruf abgeschickt: Jeder Fehler, der keine Auskunft
	// des Nodes ist, lässt den Ausgang offen.
	open := func(message string) (Document, attemptOutcome, error) {
		return Document{}, attemptOpen, &ToolError{Tool: tool, Code: CodeOutcomeUnknown, Message: redact(message, c.token)}
	}
	result, err := session.call(ctx, tool, args)
	if err != nil {
		return open(fmt.Sprintf("Node unter %s: %v", c.endpoint, err))
	}
	if result.IsError {
		toolErr := c.toolFailure(tool, result)
		switch toolErr.Code {
		case CodeOutcomeUnknown:
			return Document{}, attemptOpen, toolErr
		case CodeUnreachable, CodeUnsupported, CodeInternal:
			return Document{}, attemptNotSent, toolErr
		}
		return Document{}, attemptRejected, toolErr
	}
	if len(result.Structured) == 0 {
		return open("Antwort ohne strukturierten Inhalt")
	}
	var doc Document
	if err := json.Unmarshal(result.Structured, &doc); err != nil {
		return open("Antwort nicht lesbar")
	}
	if doc.Name == "" {
		doc.Name = target
	}
	if doc.Replayed && doc.ID == "" && len(doc.IDs) == 1 {
		doc.ID = doc.IDs[0]
	}
	return doc, attemptDone, nil
}

// checkNodeVersion fragt den Node in der offenen Sitzung nach whoami und
// lehnt einen Node vor MinWriteNodeVersion ab. Eine Version, die sich nicht
// vergleichen lässt (dev, mit Suffix, verdeckt ohne Version), geht durch: Ein
// alter Node lehnt request_id dann selbst ab und schreibt nichts.
func (c *Client) checkNodeVersion(ctx context.Context, session *toolSession, tool string) error {
	result, err := session.call(ctx, "whoami", map[string]any{})
	if err != nil {
		return c.transportError(tool, fmt.Errorf("whoami: %w", err))
	}
	fail := func(message string) error {
		return &ToolError{Tool: tool, Code: CodeNodeUnreachable, Message: redact(message, c.token)}
	}
	if result.IsError {
		return fail("whoami meldet einen Fehler: " + result.Text)
	}
	var out whoamiOutput
	if len(result.Structured) == 0 || json.Unmarshal(result.Structured, &out) != nil {
		return fail("Antwort auf whoami nicht lesbar")
	}
	if versionBefore(out.Version, MinWriteNodeVersion) {
		return fail(fmt.Sprintf("Node %s zu alt für Schreibvorgänge, mindestens %s — `kephalaion upgrade`",
			out.Version, MinWriteNodeVersion))
	}
	return nil
}

// outcomeUnknown ist der Fehler eines offenen Ausgangs.
func (c *Client) outcomeUnknown(tool, target string, first error, detail string) error {
	return &ToolError{Tool: tool, Code: CodeOutcomeUnknown, Message: redact(fmt.Sprintf(
		"Ausgang offen: %s von %s kann beim Hub angekommen sein (erster Versuch: %s; %s). "+
			"Nicht noch einmal schicken, sondern nachsehen.", tool, target, errorMessage(first), detail), c.token)}
}

// errorMessage ist die Meldung eines Fehlers, bei einem *ToolError ohne
// Werkzeug davor.
func errorMessage(err error) string {
	var toolErr *ToolError
	if errors.As(err, &toolErr) {
		return toolErr.Message
	}
	return err.Error()
}

// semverPattern ist eine Version vMAJOR.MINOR.PATCH ohne Zusatz.
var semverPattern = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

// versionBefore sagt, ob version älter ist als min. Lässt sich eine der
// beiden nicht lesen, ist die Antwort nein.
func versionBefore(version, min string) bool {
	a, b := semverPattern.FindStringSubmatch(version), semverPattern.FindStringSubmatch(min)
	if a == nil || b == nil {
		return false
	}
	for i := 1; i <= 3; i++ {
		x, errX := strconv.Atoi(a[i])
		y, errY := strconv.Atoi(b[i])
		if errX != nil || errY != nil {
			return false
		}
		if x != y {
			return x < y
		}
	}
	return false
}
