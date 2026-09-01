// Package mcpserver bietet den aufgelösten Arbeitsstand als MCP-Werkzeug an.
//
// Es ist die dritte Fassade auf project.BuildContext, neben dem Subkommando
// `context` und dem Web-Endpunkt. Alle drei geben dieselbe Antwort; wer eine
// vierte Quelle aufmacht, bekommt zwangsläufig eine abweichende.
//
// Der Paketname ist bewusst nicht `mcp`: so heißt das Paket des SDK.
package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// serverName und serverVersion melden sich beim Client an. k-playbook führt
// keine Versionsnummer — der Stand ergibt sich aus dem Git-Clone der
// Installation. Der Wert hier ist reine Anzeige: die Spec verlangt das Feld,
// und sie hält ausdrücklich fest, dass es nicht für Entscheidungen taugt.
const (
	serverName    = "k-playbook"
	serverVersion = "0.1.0"
)

// contextInput ist die Eingabe des Werkzeugs.
//
// Der Parameter ist der einzige Weg, das Projekt zu bestimmen. Ein
// stdio-Server wird einmal vom Client gestartet und behält dessen
// Arbeitsverzeichnis über die ganze Sitzung; die Spec verlangt darum, dass
// solcher Zustand über einen expliziten Bezeichner läuft und nicht über die
// Verbindung.
type contextInput struct {
	Dir string `json:"dir,omitempty" jsonschema:"Verzeichnis, ab dem aufwärts nach K-PLAYBOOK.yaml gesucht wird. Ohne Angabe gilt das Arbeitsverzeichnis des Serverprozesses — das ist nicht zwingend das Projekt, an dem gerade gearbeitet wird. Im Zweifel angeben."`
}

const toolDescription = `Gibt den aufgelösten Arbeitsstand eines k-playbook-Projekts zurück: ` +
	`Pfade, Instruktionsdateien in Lesereihenfolge, Remediation-Policy, Guidelines ` +
	`und die effektiven Kataloge für rules, reviews und checks — mitgeliefert und ` +
	`projekteigen bereits zusammengeführt. Zieht dabei die Assistenten-Verlinkung ` +
	`auf den Katalog nach und meldet unter "links", was sich geändert hat. ` +
	`Dieselbe Antwort wie das Subkommando "k-playbook context".`

// Run startet den Server und blockiert, bis der Client die Verbindung schließt
// oder ctx abgebrochen wird.
//
// Auf stdout darf ausschließlich das Protokoll erscheinen: dort läuft der
// JSON-RPC-Strom, und er bleibt über die ganze Sitzung offen. Diagnose gehört
// nach stderr — die Spec 2026-07-28 hat das Logging-Primitiv abgekündigt und
// empfiehlt für stdio genau das.
func Run(ctx context.Context) error {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    serverName,
		Version: serverVersion,
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "k_playbook_context",
		Description: toolDescription,
	}, contextTool)
	addReviewTools(server)

	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil && !isSessionEnd(err) {
		return fmt.Errorf("MCP-Server: %w", err)
	}
	return nil
}

// sessionEndMarker ist der Text, mit dem das SDK das Ende einer Sitzung meldet.
const sessionEndMarker = "server is closing"

// isSessionEnd erkennt das erwartete Ende: der Client hat stdin geschlossen.
//
// Der Weg über den Text ist unschön, aber der einzige. Das SDK meldet den
// Vorgang als JSON-RPC-Fehler -32004, dessen Typ in seinem internal-Paket liegt
// und deshalb nicht prüfbar ist; io.EOF steht nur in der Meldung und ist nicht
// umhüllt, und weder mcp.ErrConnectionClosed noch io.EOF greifen über
// errors.Is. Alles selbst nachgemessen an v1.7.0.
//
// Ohne diese Unterscheidung endete der Prozess bei jedem normalen
// Sitzungsende mit Exit 1 — für den Client sieht das aus wie ein Absturz.
// Erkennt eine neue SDK-Fassung den Fall nicht mehr, fällt das sofort auf:
// die Tests in cmd/k-playbook verlangen einen sauberen Abgang.
func isSessionEnd(err error) bool {
	return strings.Contains(err.Error(), sessionEndMarker)
}

// contextTool löst den Arbeitsstand bei jedem Aufruf neu auf. Kein
// Zwischenspeicher über die Sitzung: der Client kann nacheinander nach
// verschiedenen Projekten fragen, und eine Konfiguration kann sich zwischen
// zwei Aufrufen ändern.
//
// Der Rückgabewert ist any statt project.Context. Damit entsteht kein
// Ausgabeschema, das sonst bei jedem tools/list mitginge, und der Inhalt ist
// Zeichen für Zeichen derselbe wie beim Subkommando — nur so lässt sich die
// Gleichheit beider Fassaden überhaupt prüfen.
//
// Ein zurückgegebener Fehler wird vom SDK zum Werkzeugfehler, nicht zum
// Protokollfehler: der Server bleibt für den nächsten Aufruf ansprechbar.
func contextTool(ctx context.Context, req *mcp.CallToolRequest, in contextInput) (*mcp.CallToolResult, any, error) {
	dir := in.Dir
	if dir == "" {
		workdir, err := os.Getwd()
		if err != nil {
			return nil, nil, fmt.Errorf("Arbeitsverzeichnis ermitteln: %w", err)
		}
		dir = workdir
	}

	built, err := project.ContextForDir(dir)
	if err != nil {
		return nil, nil, err
	}

	encoded, err := json.MarshalIndent(built, "", "  ")
	if err != nil {
		return nil, nil, fmt.Errorf("Arbeitsstand kodieren: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(encoded)}},
	}, nil, nil
}
