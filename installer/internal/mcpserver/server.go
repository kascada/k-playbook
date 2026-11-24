// Package mcpserver bietet die Auskünfte und Operationen von k-playbook als
// MCP-Werkzeuge an: Briefing, Umgebungsprüfung, Dokumente der Collection,
// Todos, Doku-Aufträge, Sperre, Oberfläche und die gesperrten
// Review-Werkzeuge. Jedes Werkzeug ist eine Hülle über derselben Funktion
// wie das gleichnamige Unterkommando; eine eigene Fassung im Server gibt es
// nicht.
//
// Der Paketname ist bewusst nicht `mcp`: so heißt das Paket des SDK.
package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kascada/k-playbook/installer/internal/guiproc"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// serverName und serverVersion melden sich beim Client an. Der Wert hier ist
// reine Anzeige und nicht die Version des Binarys (die nennt `k-playbook
// version`): die Spec verlangt das Feld, und sie hält ausdrücklich fest, dass
// es nicht für Entscheidungen taugt.
const (
	serverName    = "k-playbook"
	serverVersion = "0.1.0"
)

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

	addReviewTools(server)
	addTodoTools(server)
	addDocTools(server)
	addBriefingTool(server)
	addHealthTool(server)
	addGitHubTestTool(server)
	addGUITools(server)
	addNotifyTool(server)
	addFindingsTool(server)
	addLockTool(server)

	// Die Kennung des Binaries wird vor dem ersten Aufruf festgehalten. Danach
	// meldet dieselbe Funktion, was unter dem Pfad **jetzt** liegt: os.Executable()
	// steht für einen laufenden Prozess fest, und Go schneidet das " (deleted)"
	// weg, das Linux an den Link hängt, sobald die Datei ersetzt wurde. Ein
	// späterer Aufruf sieht deshalb die neue Datei.
	server.AddReceivingMiddleware(staleBinaryNotice(guiproc.OwnBuild(), guiproc.OwnBuild))

	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil && !isSessionEnd(err) {
		return fmt.Errorf("MCP-Server: %w", err)
	}
	return nil
}

// projectDirMissingMessage ist die Meldung für ein fehlendes, leeres oder nur
// aus Leerraum bestehendes projectDir — für alle Werkzeugfamilien dieselbe.
//
// Sie richtet sich an das Modell, das den Aufruf abgesetzt hat: es muss daraus
// lesen können, dass nichts geschehen ist und was es tun soll. Ohne das gilt
// ein gescheiterter Schreibaufruf leicht als erledigt.
const projectDirMissingMessage = "projectDir fehlt — nichts ausgeführt. Wiederhole den Aufruf mit " +
	"projectDir, dem Projektverzeichnis mit " + project.ConfigFileName + " (absoluter Pfad)."

// errProjectDirMissing ist der Fehler zu projectDirMissingMessage. Die Hüllen
// erkennen ihn über errors.Is und melden ihn als invalid_input.
var errProjectDirMissing = errors.New(projectDirMissingMessage)

// checkProjectDir ist die gemeinsame Prüfung aller Werkzeuge mit projectDir,
// bevor irgendetwas aufgelöst wird. Im Schema ist das Feld optional, damit ein
// Aufruf ohne es die Hülle erreicht; Pflicht ist es trotzdem.
//
// Einen Rückfall auf das Arbeitsverzeichnis des Servers gibt es bewusst nicht:
// ein stdio-Server behält das Verzeichnis, in dem der Client ihn gestartet hat,
// und das ist nicht zwingend das Projekt, an dem gerade gearbeitet wird.
func checkProjectDir(inputDir string) error {
	if strings.TrimSpace(inputDir) == "" {
		return errProjectDirMissing
	}
	return nil
}

// projectDirErrorCode ordnet einen Fehler aus resolveProjectDir seinem Code
// zu: ein fehlendes projectDir ist ein Eingabefehler, alles andere heißt, dass
// der angegebene Pfad zu keinem Projekt führt.
func projectDirErrorCode(err error) string {
	if errors.Is(err, errProjectDirMissing) {
		return "invalid_input"
	}
	return "project_not_found"
}

// resolveProjectDir löst den projectDir-Parameter eines Werkzeugs auf: fehlt
// er, ist das errProjectDirMissing, ein relativer Pfad gilt zum
// Arbeitsverzeichnis des Serverprozesses, und ab dem Ergebnis wird aufwärts
// nach dem Anker gesucht.
//
// Im Fehlerfall trägt das erste Ergebnis das Verzeichnis, das die Meldung
// nennen soll — vor der Auflösung gibt es kein Projektverzeichnis, aber der
// Aufrufer soll sagen, wonach gesucht wurde.
//
// Gemeinsam für alle Werkzeugfamilien, die nur den Anker brauchen: Todos und
// Dokumente liegen in der Collection des Projekts, nicht im Katalog der
// Installation. Zweimal aufgelöst hieße zwei Chancen, es verschieden zu tun.
func resolveProjectDir(inputDir string) (string, error) {
	if err := checkProjectDir(inputDir); err != nil {
		return "", err
	}
	dir := filepath.Clean(inputDir)
	if !filepath.IsAbs(dir) {
		workdir, err := os.Getwd()
		if err != nil {
			return inputDir, err
		}
		dir = filepath.Join(workdir, dir)
	}

	environment := project.DetectFrom(dir)
	if !environment.Installed {
		return dir, fmt.Errorf("kein k-playbook-Projekt gefunden — gesucht ab %s aufwärts nach %s", dir, project.ConfigFileName)
	}
	return environment.ProjectDir, nil
}

// jsonToolResult kodiert einen Antwort-Umschlag als eingerückten JSON-Text.
// Der Umschlag ist je Werkzeugfamilie ein eigener Typ; das Kodieren ist es
// nicht.
func jsonToolResult(envelope any, toolError bool) *mcp.CallToolResult {
	encoded, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Antwort kodieren: %v", err)}},
		}
	}
	return &mcp.CallToolResult{
		IsError: toolError,
		Content: []mcp.Content{&mcp.TextContent{Text: string(encoded)}},
	}
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
