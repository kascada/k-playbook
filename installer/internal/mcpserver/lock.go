package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kascada/k-playbook/installer/internal/worklock"
)

// k_playbook_lock ist die Hülle über worklock, dieselbe Mitte, die
// `k-playbook lock` ruft. Anders als die CLI kennt das Werkzeug einen
// langlebigen Halter: den Prozess des Clients über diesem Server
// (worklock.FindHolder). Nur über das Werkzeug genommen kann eine Sperre
// deshalb je als sicher verwaist gelten.
const lockToolName = "k_playbook_lock"

// Die Frist von wait. Sie liegt unter dem kleinsten gemessenen Limit beider
// Clients: Ein MCP-Aufruf durfte in Claude Code 2.1.328 und OpenCode 2.3.0
// mindestens 540 Sekunden blockieren, ein Bash-Aufruf nur 120 Sekunden —
// danach bricht OpenCode ab, Claude Code schiebt ihn in den Hintergrund. Die
// CLI hat dieselbe Frist, damit `k-playbook lock wait` über Bash im
// Vordergrund zurückkehrt.
const (
	lockWaitDefault = 90 * time.Second
	lockWaitMax     = 100 * time.Second
	// lockWaitInterval ist der Abstand, in dem wait nachsieht.
	lockWaitInterval = time.Second
)

// lockInput trägt projectDir wie die übrigen Familien.
type lockInput struct {
	ProjectDir string `json:"projectDir,omitempty" jsonschema:"Pflicht. Verzeichnis, ab dem aufwärts nach K-PLAYBOOK.yaml gesucht wird. Relative Pfade werden relativ zum Arbeitsverzeichnis des MCP-Servers aufgelöst."`
	Action     string `json:"action,omitempty" jsonschema:"Pflicht. acquire, wait, check, release oder status."`
	Token      string `json:"token,omitempty" jsonschema:"Die eigene Marke aus acquire oder wait. Pflicht für check; für release ohne force; bei acquire und wait nimmt die noch gültige eigene Marke erneut (reentrant)."`
	Seconds    int    `json:"seconds,omitempty" jsonschema:"Nur wait: Frist in Sekunden, höchstens 100, ohne Angabe 90."`
	Force      bool   `json:"force,omitempty" jsonschema:"Nur release: ohne Marke freigeben. Nur auf ausdrücklichen Wunsch des Nutzers."`
	Expect     int64  `json:"expect,omitempty" jsonschema:"Nur release mit force: gibt nur frei, solange der Halter noch die Marke mit dieser Nummer (holder.number) hat."`
	Purpose    string `json:"purpose,omitempty" jsonschema:"acquire und wait: Wozu, im Klartext, etwa der Task; nur zur Anzeige für andere."`
	Session    string `json:"session,omitempty" jsonschema:"acquire und wait: die Sitzungskennung aus dem Briefing (Zeile Sitzung:); nur zur Anzeige."`
}

// lockEnvelope ist die Antwortform: das Ergebnis von worklock eingebettet.
type lockEnvelope struct {
	OK         bool   `json:"ok"`
	Tool       string `json:"tool"`
	ProjectDir string `json:"projectDir"`
	Action     string `json:"action,omitempty"`
	*worklock.Result
	Error *errorEnvelope `json:"error,omitempty"`
}

// lockFindHolder ist worklock.FindHolder, für Tests austauschbar.
var lockFindHolder = worklock.FindHolder

const lockToolDescription = "Die Sperre des Arbeitsbaums eines Projekts: Zu einer Zeit ändert höchstens ein Lauf " +
	"von /k-task-run den Arbeitsbaum. Es gilt, was dieses Werkzeug sagt. action: acquire nimmt die Sperre und " +
	"gibt state acquired mit token (der Marke, merken) und number zurück, oder state busy mit holder (session, " +
	"client, purpose, since, number, orphaned yes|no|uncertain, orphanedWhy); ist der Halter sicher verwaist " +
	"(orphaned yes), wird übernommen, die Antwort nennt ihn unter takenOver. Mit der eigenen, noch gültigen Marke " +
	"als token gibt acquire sie zurück (reentrant). wait wie acquire, wartet aber, solange belegt, bis zur Frist " +
	"seconds (Standard 90, höchstens 100) und gibt danach busy mit dem Halter zurück; ein laufendes wait hält " +
	"keine anderen Aufrufe auf. check mit token: state held, oder lost mit reason (released, forced, taken_over, " +
	"file_missing, unknown) und dem jetzigen holder (fehlt er, ist die Sperre frei). release mit token gibt frei; " +
	"eine veraltete Marke gibt nichts frei und meldet lost. release mit force ohne Marke nur auf ausdrücklichen " +
	"Wunsch des Nutzers, mit expect gleich holder.number nur, solange dieser Halter noch hält. status: busy mit " +
	"holder oder free. Halter ist der Prozess des Clients über diesem Server. Dieselbe Mitte wie " +
	"`k-playbook lock`; über die CLI genommen gilt eine Sperre nie als sicher verwaist."

func addLockTool(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{Name: lockToolName, Description: lockToolDescription}, lockTool)
}

func lockTool(ctx context.Context, req *mcp.CallToolRequest, input lockInput) (*mcp.CallToolResult, any, error) {
	action := strings.TrimSpace(input.Action)
	fail := func(dir, code string, err error) *mcp.CallToolResult {
		return jsonToolResult(lockEnvelope{OK: false, Tool: lockToolName, ProjectDir: dir, Action: action,
			Error: &errorEnvelope{Code: code, Message: err.Error()}}, true)
	}
	projectDir, err := resolveProjectDir(input.ProjectDir)
	if err != nil {
		return fail(projectDir, projectDirErrorCode(err), err), nil, nil
	}

	lock := worklock.New(projectDir)
	var result worklock.Result
	switch action {
	case "acquire":
		result, err = lock.Acquire(input.Token, lockHolder(req, input))
	case "wait":
		timeout, terr := lockWaitTimeout(input.Seconds)
		if terr != nil {
			return fail(projectDir, "invalid_input", terr), nil, nil
		}
		result, err = lock.Wait(ctx, input.Token, lockHolder(req, input), timeout, lockWaitInterval)
	case "check":
		result, err = lock.Check(input.Token, lockHolder(req, input))
	case "release":
		result, err = lock.Release(input.Token, input.Force, input.Expect)
	case "status":
		result, err = lock.Status()
	default:
		return fail(projectDir, "invalid_input", fmt.Errorf("action muss acquire, wait, check, release oder status sein, nicht %q", action)), nil, nil
	}
	if err != nil {
		code := "failed"
		switch {
		case errors.Is(err, worklock.ErrInvalidInput):
			code = "invalid_input"
		case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
			code = "cancelled"
		}
		return fail(projectDir, code, err), nil, nil
	}
	return jsonToolResult(lockEnvelope{OK: true, Tool: lockToolName, ProjectDir: projectDir, Action: action,
		Result: &result}, false), nil, nil
}

// lockWaitTimeout prüft die Frist von wait.
func lockWaitTimeout(seconds int) (time.Duration, error) {
	switch {
	case seconds == 0:
		return lockWaitDefault, nil
	case seconds < 0 || time.Duration(seconds)*time.Second > lockWaitMax:
		return 0, fmt.Errorf("seconds muss zwischen 1 und %d liegen, nicht %d", int(lockWaitMax/time.Second), seconds)
	default:
		return time.Duration(seconds) * time.Second, nil
	}
}

// lockHolder baut den Halter aus dem Aufruf: Anzeige aus Eingabe und Client,
// der Prozess des Clients als Halter.
func lockHolder(req *mcp.CallToolRequest, input lockInput) worklock.Holder {
	holder := worklock.Holder{
		Session: strings.TrimSpace(input.Session),
		Purpose: strings.TrimSpace(input.Purpose),
		Process: lockFindHolder(),
	}
	if req == nil {
		return holder
	}
	if req.Session != nil {
		if params := req.Session.InitializeParams(); params != nil && params.ClientInfo != nil {
			holder.Client = params.ClientInfo.Name
		}
	}
	if req.Params != nil {
		// OpenCode gibt bei jedem Aufruf seine Sitzung mit; sie hilft dem
		// Nutzer, den Halter zu erkennen, mehr nicht.
		if session, ok := req.Params.Meta["ai.opencode/sessionID"].(string); ok {
			holder.ClientSession = session
		}
	}
	return holder
}
