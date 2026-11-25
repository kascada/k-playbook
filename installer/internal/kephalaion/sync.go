package kephalaion

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// MinSyncNodeVersion ist die älteste Version des Nodes mit dem Werkzeug sync
// (Kephalaion v0.5.0, docs/installation.md, „Einen Abgleich anfordern“). Ein
// älterer Node kennt das Werkzeug nicht; sein SDK antwortet darauf mit dem
// JSON-RPC-Fehler -32602 wie auf ungültige Argumente, deshalb entscheidet die
// Version aus whoami.
const MinSyncNodeVersion = "v0.5.0"

// Die Codes des Werkzeugs sync über die der schreibenden Werkzeuge hinaus
// (CodeUnreachable, CodeUnsupported, CodeInternal in docs.go). Welcher Code
// für welchen Fall gilt, steht bei Kephalaion in der Tabelle „Codes je Fall“.
const (
	// CodeSyncTimeout: Die Wartefrist des Nodes (40 s) ist abgelaufen, bevor
	// der abgewartete Abgleich geendet hat; er läuft im Hintergrund weiter.
	CodeSyncTimeout = "sync_timeout"
	// CodeNeverSynced: Der Hub-Eintrag am Node hat noch keine Replica.
	CodeNeverSynced = "never_synced"
	// CodeUnauthenticated: Der Hub nimmt den Node nicht an (gesperrt, Token
	// des Nodes ungültig); die Abhilfe liegt am Rechner des Nodes.
	CodeUnauthenticated = "unauthenticated"
	// CodeUnsupportedVersion: Der Hub bedient die Fassung des Vertrags dieses
	// Nodes nicht.
	CodeUnsupportedVersion = "unsupported_version"
	// CodeHubRefused: Der Hub hat mit einem Fehler des Vertrags abgelehnt, oder
	// der Abgleich scheiterte mit der Art protocol.
	CodeHubRefused = "hub_refused"
	// CodeUnknownHub: Der Node hat keinen Hub-Eintrag mit dem Alias (nur, wenn
	// die Antwort nicht verdeckt ist).
	CodeUnknownHub = "unknown_hub"
	// CodeNotReadable: Der Aufrufer ist am Hub nicht gültig angemeldet — und,
	// verdeckt, jeder Alias, gleich ob es ihn gibt.
	CodeNotReadable = "not_readable"
	// CodeInvalid: hub ist kein gültiger Alias.
	CodeInvalid = "invalid"
)

// NodeSyncWait ist die Wartefrist des Nodes für das Werkzeug sync, fest im
// Node (Kephalaion v0.5.0: 40 s). Nur für Meldungen; die Grenze setzt der Node.
const NodeSyncWait = 40 * time.Second

// SyncTimeout begrenzt den ganzen Abgleich aus Sicht von k-playbook: Sitzung
// öffnen, whoami und sync. Sie liegt über der Wartefrist des Nodes
// (NodeSyncWait, 40 s), damit der Node seinen Frist-Fehler selbst melden kann;
// ToolTimeout gilt für diesen Aufruf nicht. Austauschbar für Tests.
var SyncTimeout = time.Minute

// SyncError ist der Fehler von SyncHub: ein Satz ohne Token, der den Grund
// nennt, und die Abhilfe. Code ist der Code des Werkzeugs sync, wenn der Node
// einen genannt hat, sonst leer.
type SyncError struct {
	Code    string
	Message string
}

func (e *SyncError) Error() string { return e.Message }

// SyncState ist der Stand nach einem bestätigten Abgleich, wie der Node ihn
// meldet: LastSuccess ist der Zeitpunkt des abgewarteten Abgleichs (RFC 3339),
// Revision der Stand der Replica beim Antworten — 0 für einen Hub-Eintrag ohne
// gewünschte Collection, nicht als monoton zugesichert.
type SyncState struct {
	LastSuccess string
	Revision    int64
}

// syncOutput ist die Struktur der Antwort von sync: bei Erfolg Hub und Stand
// (in der Form von sync aus whoami), bei einem Fehler Hub und error.
type syncOutput struct {
	Hub  string `json:"hub"`
	Sync *struct {
		LastSuccess string `json:"last_success"`
		Revision    *int64 `json:"revision"`
		LastError   string `json:"last_error"`
	} `json:"sync"`
}

// SyncHub fordert beim Node den Abgleich der Replica für den Hub dieses
// Clients an — das MCP-Werkzeug sync, kein Prozess und keine Datenbank des
// Nodes auf diesem Rechner. Damit geht es auch in einem reinen Client
// (DevContainer mit der config des Hosts, Node über einen Weiterleiter).
// Gebraucht, wenn ein Leser sich auf eine frische Replica verlassen muss —
// etwa die Vorprüfung der Befund-Queue (project.CheckFindings), die sonst
// übersähe, was ein anderer Rechner gerade angelegt hat.
//
// Ablauf: initialize, whoami (Version mindestens MinSyncNodeVersion; eine
// Version ohne Semver gilt als nicht zu alt), dann sync mit hub. Der Node
// antwortet erst, wenn ein Abgleich geendet hat, der nach der Anfrage begonnen
// hat. Erfolg ist nur eine Antwort ohne isError mit dem Feld revision und
// ohne last_error; alles andere — isError, eine unlesbare Antwort, ein
// Abbruch, die eigene Zeitgrenze SyncTimeout — ist ein *SyncError mit einem
// Satz und einer Abhilfe je Fall. Die Meldung des Nodes wird wörtlich
// weitergereicht, jeder Text läuft durch redact.
func (c *Client) SyncHub(ctx context.Context) (SyncState, error) {
	hub := c.hub
	// Der Hub wird Teil eines Header-Namens; die Namensregel hält ihn sauber.
	if err := CheckName("Hub", hub); err != nil {
		return SyncState{}, &SyncError{Message: "Abgleich nicht möglich: " + err.Error() + "."}
	}
	general := fmt.Sprintf("Abhilfe: erneut versuchen; bleibt es dabei, kephalaion node sync %s von Hand aufrufen und den Grund beheben.", hub)
	fail := func(code, format string, args ...any) (SyncState, error) {
		return SyncState{}, &SyncError{Code: code, Message: redact(fmt.Sprintf(format, args...), c.token)}
	}

	runCtx, cancel := context.WithTimeout(ctx, SyncTimeout)
	defer cancel()
	// stopped meldet einen Abbruch oder die eigene Zeitgrenze, sonst nil;
	// beide gehen jedem anderen Grund vor, weil eine Verbindung dann nur
	// deshalb abreißt.
	stopped := func() error {
		switch {
		case ctx.Err() != nil:
			_, err := fail("", "Der Abgleich für den Hub %s wurde abgebrochen.", hub)
			return err
		case runCtx.Err() != nil:
			_, err := fail("", "Der Abgleich für den Hub %s hat nicht innerhalb von %s geendet. %s", hub, SyncTimeout, general)
			return err
		}
		return nil
	}

	session, err := openSession(runCtx, newRPC(c.endpoint, maxResponseBytes, SyncTimeout), hub, c.account, c.token)
	if err != nil {
		if stop := stopped(); stop != nil {
			return SyncState{}, stop
		}
		return fail("", "Der Node unter %s ist für den Abgleich nicht erreichbar (%v). "+
			"Abhilfe: am Rechner des Nodes prüfen, ob kephalaion serve läuft (kephalaion status).", c.endpoint, err)
	}

	// Die Version entscheidet, ob der Node das Werkzeug kennt: Ein älterer
	// beantwortet sync mit -32602 wie ungültige Argumente.
	who, err := session.call(runCtx, "whoami", map[string]any{})
	if err != nil {
		if stop := stopped(); stop != nil {
			return SyncState{}, stop
		}
		return fail("", "Der Node unter %s beantwortet whoami nicht (%v). %s", c.endpoint, err, general)
	}
	var out whoamiOutput
	if who.IsError || len(who.Structured) == 0 || json.Unmarshal(who.Structured, &out) != nil {
		return fail("", "Die Antwort des Nodes auf whoami ist nicht lesbar; der Abgleich wurde nicht angefordert. %s", general)
	}
	if versionBefore(out.Version, MinSyncNodeVersion) {
		return fail("", "Der Node (Version %s) kennt das Werkzeug sync nicht; nötig ist mindestens %s. "+
			"Abhilfe: kephalaion upgrade am Rechner des Nodes.", out.Version, MinSyncNodeVersion)
	}

	result, err := session.call(runCtx, "sync", map[string]any{"hub": hub})
	if err != nil {
		if stop := stopped(); stop != nil {
			return SyncState{}, stop
		}
		return fail("", "Der Node hat den Abgleich für den Hub %s nicht bestätigt (%v). %s", hub, err, general)
	}
	if result.IsError {
		toolErr := c.toolFailure("sync", result)
		return fail(toolErr.Code, "%s", syncFailureMessage(hub, toolErr.Code, clause(toolErr.Message), general))
	}
	var reply syncOutput
	if len(result.Structured) == 0 || json.Unmarshal(result.Structured, &reply) != nil || reply.Sync == nil {
		return fail("", "Die Antwort des Nodes auf sync für den Hub %s ist nicht lesbar; der Abgleich ist nicht bestätigt. %s", hub, general)
	}
	if reply.Sync.LastError != "" {
		return fail("", "Der Node meldet den Abgleich für den Hub %s mit einem Fehler: %s. %s", hub, clause(sanitize(reply.Sync.LastError)), general)
	}
	if reply.Sync.Revision == nil {
		return fail("", "Der Node meldet den Abgleich für den Hub %s ohne Revision; der Abgleich ist nicht bestätigt. %s", hub, general)
	}
	return SyncState{LastSuccess: reply.Sync.LastSuccess, Revision: *reply.Sync.Revision}, nil
}

// syncFailureMessage ist der Satz zu einem Fehler des Werkzeugs sync: der
// Grund mit der Meldung des Nodes und die Abhilfe, die zum Code passt. Die
// Codes steuern nur die Abhilfe; gescheitert ist der Abgleich bei jedem.
func syncFailureMessage(hub, code, message, general string) string {
	switch code {
	case CodeUnreachable:
		return fmt.Sprintf("Der Node hat den Hub %s nicht erreicht: %s. Abhilfe: Hub und Netz prüfen, dann erneut versuchen.", hub, message)
	case CodeUnsupported:
		return fmt.Sprintf("Der Node kann den Abgleich für den Hub %s nicht anstoßen: %s. "+
			"Abhilfe: den Hub-Eintrag am Rechner des Nodes prüfen (Transport ssh wird nicht abgeglichen).", hub, message)
	case CodeSyncTimeout:
		return fmt.Sprintf("Der Abgleich für den Hub %s hat nicht innerhalb der Wartefrist des Nodes (%d s) geendet: %s. "+
			"Er läuft im Hintergrund weiter. Abhilfe: in einer Minute erneut versuchen.", hub, int(NodeSyncWait.Seconds()), message)
	case CodeUnknownHub, CodeNotReadable:
		return fmt.Sprintf("Der Node kennt den Hub %s nicht, oder die Anmeldung dort gilt nicht: %s. "+
			"Abhilfe: kephalaion status zeigt die Hubs des Nodes und den Zustand der Anmeldung; "+
			"Hub der Projektbindung, Account und Token prüfen.", hub, message)
	case CodeUnauthenticated:
		return fmt.Sprintf("Der Hub %s nimmt den Node nicht an: %s. "+
			"Abhilfe am Rechner des Nodes, nicht hier: kephalaion node hub check %s.", hub, message, hub)
	case CodeNeverSynced:
		return fmt.Sprintf("Der Node hat für den Hub %s noch keine Replica: %s. "+
			"Abhilfe am Rechner des Nodes, nicht hier: kephalaion node sync %s.", hub, message, hub)
	case "":
		return fmt.Sprintf("Der Abgleich für den Hub %s ist gescheitert: %s. %s", hub, message, general)
	}
	return fmt.Sprintf("Der Abgleich für den Hub %s ist gescheitert (%s): %s. %s", hub, code, message, general)
}

// clause macht aus einer Meldung des Nodes einen Satzteil vor „. “: ohne
// Punkt und Leerraum am Ende, damit kein doppelter Punkt entsteht.
func clause(message string) string {
	return strings.TrimRight(strings.TrimSpace(message), ".")
}
