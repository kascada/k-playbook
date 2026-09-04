package webui

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// mcpResponse ist der Zustand der Registrierung, wie ihn die Oberfläche
// braucht: Kontext, Einzelzustände und das Kommando, um das es geht.
type mcpResponse struct {
	Environment project.Environment `json:"environment"`
	Entries     []project.MCPStatus `json:"entries"`
	// Command ist der Eintrag, der geschrieben wird: der beim Schreiben
	// aufgelöste absolute Pfad des installierten k-playbook. Leer, wenn sich
	// keines auflösen ließ — dann wird auch nichts geschrieben.
	Command string `json:"command"`
	// WorkdirMismatch: die Oberfläche selbst wurde nicht im Hauptverzeichnis
	// gestartet. Der eingetragene Befehl ist zwar absolut, der MCP-Server löst
	// das Projekt aber über sein Arbeitsverzeichnis auf; dann ist der Verdacht
	// begründet, dass auch der Assistent woanders geöffnet wird.
	WorkdirMismatch bool   `json:"workdirMismatch"`
	OK              bool   `json:"ok"`
	Message         string `json:"message"`
}

func mcpHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, mcpState(""))
}

// applyMCPHandler richtet die Registrierung ein. Ohne gefundene Config gibt es
// kein Projekt, auf das sich die Aktion beziehen könnte.
func applyMCPHandler(w http.ResponseWriter, r *http.Request) {
	environment := project.Detect()
	if !environment.Installed {
		writeJSON(w, http.StatusConflict, mcpResponse{
			Environment: environment,
			Message:     "Keine " + project.ConfigFileName + " gefunden. Es gibt kein Projekt zum Einrichten.",
		})
		return
	}

	_, err := project.ApplyMCP(environment.ProjectDir)

	message := "Registrierung eingerichtet. Der Assistent liest sie beim nächsten Start; " +
		"Claude Code fragt dabei einmal nach der Freigabe."
	if err != nil {
		message = "Nicht vollständig eingerichtet: " + err.Error()
	}
	writeJSON(w, http.StatusOK, mcpState(message))
}

// mcpState liest den aktuellen Zustand. Wie beim Assistenten-Block wird ein
// fehlendes k-playbook/ vorab abgefangen, statt in die Messung zu laufen.
func mcpState(message string) mcpResponse {
	environment := project.Detect()

	response := mcpResponse{
		Environment: environment,
		Message:     message,
	}
	command, args, err := project.MCPCommand()
	if err == nil {
		response.Command = command + " " + args[0]
	}

	switch {
	case !environment.Installed:
		if response.Message == "" {
			response.Message = "Keine " + project.ConfigFileName + " gefunden (gesucht ab " +
				project.DisplayPath(environment.SearchedFrom) + " aufwärts)."
		}

	case !environment.PlaybookPresent:
		if response.Message == "" {
			response.Message = "Installationsverzeichnis " + project.PlaybookDirName +
				"/ fehlt. Ohne die Inhalte darin hätte der Server nichts auszuliefern."
		}

	case err != nil:
		if response.Message == "" {
			response.Message = err.Error() + ". Ohne installiertes Binary gibt es keinen Pfad, " +
				"der sich eintragen ließe — erst den Bootstrap ausführen."
		}
		response.Entries = project.CheckMCP(environment.ProjectDir)

	default:
		response.Entries = project.CheckMCP(environment.ProjectDir)
		response.OK = project.MCPOK(response.Entries)
		response.WorkdirMismatch = !samePath(environment.SearchedFrom, environment.ProjectDir)
	}

	return response
}

// samePath vergleicht zwei Verzeichnisse und löst dabei Symlinks auf, soweit sie
// existieren. Ohne das meldete ein verlinktes Elternverzeichnis eine Abweichung,
// die keine ist: ProjectDir kommt aufgelöst aus der Suche, SearchedFrom nicht.
func samePath(left string, right string) bool {
	return resolvePath(left) == resolvePath(right)
}

func resolvePath(path string) string {
	cleaned := filepath.Clean(path)
	if resolved, err := filepath.EvalSymlinks(cleaned); err == nil {
		return resolved
	}
	return cleaned
}

const (
	// mcpProbeTimeout begrenzt den Selbsttest. Der Server antwortet lokal und
	// sofort; hängt er trotzdem, darf er die Seite nicht mitnehmen.
	mcpProbeTimeout = 10 * time.Second
	// mcpWaitDelay begrenzt, wie lange danach noch auf die Rohre gewartet wird.
	mcpWaitDelay = time.Second
)

// mcpProbePath ist die PATH, mit der der Selbsttest läuft.
//
// Bewusst **nicht** die geerbte Shell-PATH: der Fall, den der Selbsttest
// abbilden soll, ist der aus Dock oder Finder gestartete Client. Der erbt keine
// Login-Shell, und ~/.local/bin fehlt dort typischerweise. Liefe der Test mit
// der PATH der Shell, in der die Oberfläche gestartet wurde, meldete er grün,
// während der Client scheitert — genau die Lücke, die der eingetragene absolute
// Pfad schließt.
//
// Leer wäre falsch: der Server ruft seinerseits git auf. Was hier steht, ist
// die Umgebung, die launchd einem GUI-Programm mitgibt.
const mcpProbePath = "/usr/bin:/bin:/usr/sbin:/sbin"

// mcpProbeEnv baut die Umgebung des Selbsttests: alles Geerbte außer PATH,
// dazu die minimale System-PATH.
func mcpProbeEnv(environ []string) []string {
	filtered := make([]string, 0, len(environ)+1)
	for _, entry := range environ {
		if strings.HasPrefix(entry, "PATH=") {
			continue
		}
		filtered = append(filtered, entry)
	}
	return append(filtered, "PATH="+mcpProbePath)
}

// mcpToolsResponse ist das Ergebnis des Selbsttests: was der registrierte
// Befehl tatsächlich anbietet.
type mcpToolsResponse struct {
	// Command ist, was gestartet wurde — absolut, damit erkennbar ist, welche
	// Datei geantwortet hat.
	Command string `json:"command"`
	// Available: der Server hat geantwortet.
	Available       bool      `json:"available"`
	ServerName      string    `json:"serverName,omitempty"`
	ServerVersion   string    `json:"serverVersion,omitempty"`
	ProtocolVersion string    `json:"protocolVersion,omitempty"`
	Tools           []mcpTool `json:"tools"`
	Message         string    `json:"message"`
}

// mcpTool ist ein angebotenes Werkzeug samt seinen Parametern.
type mcpTool struct {
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	Parameters  []mcpToolParameter `json:"parameters,omitempty"`
}

type mcpToolParameter struct {
	Name        string `json:"name"`
	Type        string `json:"type,omitempty"`
	Required    bool   `json:"required"`
	Description string `json:"description,omitempty"`
}

// mcpToolsHandler misst, was der Server anbietet. Er ist ein eigener Endpunkt
// und nicht Teil von GET /api/mcp: er startet einen Subprozess und würde die
// Startseite ausbremsen.
func mcpToolsHandler(w http.ResponseWriter, r *http.Request) {
	environment := project.Detect()
	if !environment.Installed {
		writeJSON(w, http.StatusOK, mcpToolsResponse{
			Message: "Keine " + project.ConfigFileName + " gefunden.",
		})
		return
	}
	writeJSON(w, http.StatusOK, probeMCPServer(environment.ProjectDir))
}

// probeMCPServer startet den registrierten Befehl als Subprozess, spricht das
// Protokoll und gibt zurück, was ankommt.
//
// Gestartet wird genau das, was auch der Assistent startet — derselbe absolute
// Pfad —, mit dem Hauptverzeichnis als Arbeitsverzeichnis und **ohne die
// geerbte Shell-PATH**. Der laufende Prozess wäre der bequemere, aber falsche
// Messgegenstand, und eine geerbte PATH machte den Test zur Schönwettermessung:
// er liefe grün, während der aus Dock oder Finder gestartete Client scheitert.
//
// Jeder Fehlfall ist ein Ergebnis, keine Störung: die Seite sagt „antwortet
// nicht" samt Grund und bleibt bedienbar.
func probeMCPServer(projectRoot string) mcpToolsResponse {
	response := mcpToolsResponse{Tools: []mcpTool{}}

	binary, args, err := project.MCPCommand()
	if err != nil {
		response.Message = err.Error() + " — es gibt nichts zu starten."
		return response
	}
	response.Command = binary + " " + args[0]

	if info, err := os.Stat(binary); err != nil || info.IsDir() {
		response.Message = binary + " ist nicht ausführbar — es gibt nichts zu starten."
		return response
	}

	ctx, cancel := context.WithTimeout(context.Background(), mcpProbeTimeout)
	defer cancel()

	command := exec.CommandContext(ctx, binary, args...)
	command.Dir = projectRoot
	command.Env = mcpProbeEnv(os.Environ())

	stdin, err := command.StdinPipe()
	if err != nil {
		response.Message = "Server nicht ansprechbar: " + err.Error()
		return response
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		response.Message = "Server nicht ansprechbar: " + err.Error()
		return response
	}
	var stderr lockedBuffer
	command.Stderr = &stderr

	// WaitDelay begrenzt, wie lange Wait() auf die Rohre wartet, nachdem der
	// Prozess weg ist. Ohne das hinge es an einem Kindeskind, das sie geerbt hat
	// und weiterlebt — der Wrapper könnte eines hinterlassen.
	command.WaitDelay = mcpWaitDelay

	if err := command.Start(); err != nil {
		response.Message = "Server ließ sich nicht starten: " + err.Error()
		return response
	}
	// Der Prozess darf den Handler unter keinen Umständen überleben: cancel()
	// beendet ihn, das Schließen der Rohre löst den Leser aus seiner Blockade,
	// Wait() räumt ab. Das läuft auf jedem Rückweg, auch dem frühen.
	defer func() {
		stdin.Close()
		cancel()
		stdout.Close()
		command.Wait()
	}()

	// Gesprochen wird nebenläufig, damit das Zeitlimit auch dann greift, wenn
	// niemand antwortet: ein Lesen auf einem Rohr, das offen bleibt, ließe sich
	// sonst durch nichts unterbrechen und nähme die Seite mit.
	answered := make(chan mcpProbeResult, 1)
	go func() {
		answered <- speakMCP(stdin, stdout)
	}()

	select {
	case <-ctx.Done():
		response.Message = fmt.Sprintf("Server antwortet nicht: nach %s abgebrochen.", mcpProbeTimeout)
		return response

	case result := <-answered:
		if result.err != nil {
			response.Message = mcpFailureMessage(ctx, result.err, stderr.String())
			return response
		}

		response.Available = true
		response.ServerName = result.initialized.ServerInfo.Name
		response.ServerVersion = result.initialized.ServerInfo.Version
		response.ProtocolVersion = result.initialized.ProtocolVersion
		response.Tools = describeTools(result.listed)
		return response
	}
}

// mcpProbeResult ist das Ergebnis des Dialogs mit dem Server.
type mcpProbeResult struct {
	initialized mcpInitializeResult
	listed      mcpToolsResult
	err         error
}

// speakMCP schickt den Handshake und sammelt die beiden Antworten ein.
//
// stdin bleibt offen, bis sie da sind — genau wie bei einem echten Client. Ein
// sofortiges EOF beendet die Verbindung, während die Anfragen noch in Arbeit
// sind.
func speakMCP(stdin io.Writer, stdout io.Reader) mcpProbeResult {
	if _, err := io.WriteString(stdin, mcpHandshake()); err != nil {
		return mcpProbeResult{err: fmt.Errorf("Anfragen nicht schreibbar: %w", err)}
	}

	reader := bufio.NewReader(stdout)
	result := mcpProbeResult{}

	if err := readMCPResult(reader, &result.initialized); err != nil {
		result.err = err
		return result
	}
	if err := readMCPResult(reader, &result.listed); err != nil {
		result.err = err
	}
	return result
}

// lockedBuffer sammelt die Ausgabe auf stderr.
//
// Geschrieben wird sie von der Kopiergoroutine des Subprozesses, gelesen im
// Fehlerfall — also möglicherweise gleichzeitig. Ein blanker bytes.Buffer wäre
// dabei ein Datenrennen.
type lockedBuffer struct {
	mu      sync.Mutex
	content bytes.Buffer
}

func (b *lockedBuffer) Write(chunk []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.content.Write(chunk)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.content.String()
}

// mcpHandshake sind die drei Zeilen, die ein Client zu Beginn schickt:
// initialize, die Bestätigung und die Frage nach den Werkzeugen.
//
// Protokollversion 2025-11-25 mit initialize-Handshake — das ist, was die
// Clients heute sprechen.
func mcpHandshake() string {
	return `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"k-playbook-gui","version":"0"}}}` + "\n" +
		`{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}` + "\n"
}

// mcpRPCResponse ist der Rahmen einer Antwort. Verglichen wird nur er; was das
// SDK in result schreibt, gehört ihm.
type mcpRPCResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Message string `json:"message"`
	} `json:"error"`
}

type mcpInitializeResult struct {
	ProtocolVersion string `json:"protocolVersion"`
	ServerInfo      struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"serverInfo"`
}

type mcpToolsResult struct {
	Tools []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		InputSchema struct {
			Properties map[string]struct {
				Type        schemaType `json:"type"`
				Description string     `json:"description"`
			} `json:"properties"`
			Required []string `json:"required"`
		} `json:"inputSchema"`
	} `json:"tools"`
}

// schemaType nimmt das `type`-Feld eines JSON-Schemas auf.
//
// JSON Schema erlaubt dort zwei Formen: einen Namen (`"string"`) und eine Liste
// von Namen (`["null","array"]`). Die zweite ist keine Ausnahme — die eigenen
// Werkzeuge dieses Servers nutzen sie für jeden optionalen Parameter, den das
// Go-SDK aus einem Zeiger- oder Slice-Feld ableitet.
//
// Ein blankes `string` an dieser Stelle wäre deshalb kein kleiner Anzeigefehler:
// json.Unmarshal bricht die **ganze** tools/list-Antwort ab, und der Selbsttest
// meldete „Server antwortet nicht" für einen Server, der einwandfrei geantwortet
// hat. Ein unlesbares type-Feld darf den Test nie kippen: unbekannte Formen
// werden still leer, mehr nicht.
type schemaType string

func (t *schemaType) UnmarshalJSON(raw []byte) error {
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		*t = schemaType(single)
		return nil
	}

	var many []string
	if err := json.Unmarshal(raw, &many); err == nil {
		*t = schemaType(strings.Join(many, " | "))
		return nil
	}

	*t = ""
	return nil
}

// readMCPResult liest eine Antwortzeile und packt ihr result-Feld aus.
func readMCPResult(reader *bufio.Reader, target any) error {
	line, err := reader.ReadString('\n')
	if err != nil {
		return err
	}

	var response mcpRPCResponse
	if err := json.Unmarshal([]byte(line), &response); err != nil {
		return fmt.Errorf("Antwort ist kein JSON: %w", err)
	}
	if response.Error != nil {
		return fmt.Errorf("Fehlerantwort: %s", response.Error.Message)
	}
	return json.Unmarshal(response.Result, target)
}

// mcpFailureMessage sagt, woran es lag. Die Ausgabe auf stderr kommt mit, wenn
// es eine gibt: dort steht bei einem gescheiterten Wrapper der eigentliche
// Grund.
func mcpFailureMessage(ctx context.Context, err error, stderr string) string {
	if ctx.Err() != nil {
		return fmt.Sprintf("Server antwortet nicht: nach %s abgebrochen.", mcpProbeTimeout)
	}

	message := "Server antwortet nicht: " + err.Error()
	if stderr != "" {
		message += "\n" + stderr
	}
	return message
}

// describeTools bringt die Werkzeuge in eine Form, die sich anzeigen lässt.
// Parameter kommen als Map und damit in zufälliger Reihenfolge — sortiert,
// damit zwei Aufrufe dasselbe zeigen.
func describeTools(listed mcpToolsResult) []mcpTool {
	tools := make([]mcpTool, 0, len(listed.Tools))

	for _, entry := range listed.Tools {
		required := map[string]bool{}
		for _, name := range entry.InputSchema.Required {
			required[name] = true
		}

		names := make([]string, 0, len(entry.InputSchema.Properties))
		for name := range entry.InputSchema.Properties {
			names = append(names, name)
		}
		sort.Strings(names)

		tool := mcpTool{Name: entry.Name, Description: entry.Description}
		for _, name := range names {
			property := entry.InputSchema.Properties[name]
			tool.Parameters = append(tool.Parameters, mcpToolParameter{
				Name:        name,
				Type:        string(property.Type),
				Required:    required[name],
				Description: property.Description,
			})
		}
		tools = append(tools, tool)
	}
	return tools
}
