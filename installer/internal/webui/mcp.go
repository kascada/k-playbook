package webui

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
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

// mcpProbeTimeout begrenzt den Selbsttest. Der Server antwortet lokal und
// sofort; hängt er trotzdem, darf er die Seite nicht mitnehmen.
//
// Eine Variable, keine Konstante: nur so kann ein Test den Weg über die
// abgelaufene Frist gehen, ohne jedes Mal zehn Sekunden zu warten.
var mcpProbeTimeout = 10 * time.Second

// mcpWaitDelay begrenzt, wie lange danach noch auf die Rohre gewartet wird.
const mcpWaitDelay = time.Second

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

// mcpToolsResponse ist das Ergebnis einer Messung: was ein gestarteter
// Server tatsächlich anbietet.
type mcpToolsResponse struct {
	// Command ist, was gestartet wurde — absolut, damit erkennbar ist, welche
	// Datei geantwortet hat.
	Command string `json:"command"`
	// Available: der Server hat geantwortet.
	Available       bool   `json:"available"`
	ServerName      string `json:"serverName,omitempty"`
	ServerVersion   string `json:"serverVersion,omitempty"`
	ProtocolVersion string `json:"protocolVersion,omitempty"`
	// Capabilities sind die Namen, die der Server in initialize meldet —
	// tools, prompts, resources, logging und was er sonst kann. Sortiert.
	Capabilities []string  `json:"capabilities"`
	Tools        []mcpTool `json:"tools"`
	// Prompts und Resources sind nur gefüllt, wenn der Server die jeweilige
	// Fähigkeit gemeldet hat; erst dann wird danach gefragt.
	Prompts   []mcpPrompt   `json:"prompts,omitempty"`
	Resources []mcpResource `json:"resources,omitempty"`
	// Message ist ohne Antwort der Grund. Mit Antwort ist sie leer oder ein
	// Hinweis auf eine gescheiterte Folgeanfrage (prompts/list,
	// resources/list); die übrigen Felder gelten dann trotzdem.
	Message string `json:"message"`
}

// mcpPrompt ist eine angebotene Vorlage samt ihren Argumenten.
type mcpPrompt struct {
	Name        string              `json:"name"`
	Description string              `json:"description,omitempty"`
	Arguments   []mcpPromptArgument `json:"arguments,omitempty"`
}

type mcpPromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required"`
}

// mcpResource ist eine angebotene Ressource.
type mcpResource struct {
	URI         string `json:"uri"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
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
	binary, args, err := project.MCPCommand()
	if err != nil {
		return mcpToolsResponse{
			Capabilities: []string{},
			Tools:        []mcpTool{},
			Message:      err.Error() + " — es gibt nichts zu starten.",
		}
	}
	// Der Selbsttest hat keinen Knopf „Erneut messen", und sein Befehl ist nie
	// npx oder uvx: er bekommt den Hinweis dazu nicht. Ob der Prozess lief,
	// wertet /mcp nicht aus.
	response, _ := probeMCPCommand(projectRoot, binary, args, mcpProbeEnv(os.Environ()), "")
	return response
}

// probeMCPCommand ist der Kern jeder Messung: startet binary mit args im
// Hauptverzeichnis und der übergebenen Umgebung, spricht das Protokoll und
// gibt zurück, was ankommt.
//
// Der Selbsttest des eigenen Servers gibt die gesäuberte PATH mit, die
// Detailseite eines fremden Servers die geerbte Umgebung samt env aus dem
// Eintrag. binary ist ein Pfad, kein bloßer Name: aufgelöst wird vorher, damit
// die Antwort nennt, welche Datei geantwortet hat.
//
// timeoutHint wird an die Meldung nach abgelaufener Frist angehängt; leer
// bleibt es beim bloßen Satz. Das zweite Ergebnis sagt, ob command.Start()
// gelungen ist — ob also überhaupt ein Prozess lief. Jeder Rückweg davor
// liefert false.
func probeMCPCommand(projectRoot string, binary string, args []string, env []string, timeoutHint string) (mcpToolsResponse, bool) {
	response := mcpToolsResponse{Capabilities: []string{}, Tools: []mcpTool{}}
	response.Command = strings.Join(append([]string{binary}, args...), " ")

	if info, err := os.Stat(binary); err != nil || info.IsDir() {
		response.Message = binary + " ist nicht ausführbar — es gibt nichts zu starten."
		return response, false
	}

	ctx, cancel := context.WithTimeout(context.Background(), mcpProbeTimeout)
	defer cancel()

	command := exec.CommandContext(ctx, binary, args...)
	command.Dir = projectRoot
	command.Env = env

	stdin, err := command.StdinPipe()
	if err != nil {
		response.Message = "Server nicht ansprechbar: " + err.Error()
		return response, false
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		response.Message = "Server nicht ansprechbar: " + err.Error()
		return response, false
	}
	var stderr lockedBuffer
	command.Stderr = &stderr

	// WaitDelay begrenzt, wie lange Wait() auf die Rohre wartet, nachdem der
	// Prozess weg ist. Ohne das hinge es an einem Kindeskind, das sie geerbt hat
	// und weiterlebt — der Wrapper könnte eines hinterlassen.
	command.WaitDelay = mcpWaitDelay

	if err := command.Start(); err != nil {
		response.Message = "Server ließ sich nicht starten: " + err.Error()
		return response, false
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
	progress := &mcpProgress{}
	go func() {
		answered <- speakMCP(stdin, stdout, progress)
	}()

	select {
	case <-ctx.Done():
		// Hing erst eine Folgeanfrage, sind initialize und tools/list schon
		// da: sie bleiben stehen, und die Frist wird zum Hinweis — wie eine
		// Fehlerantwort auf dieselbe Frage.
		if partial, waiting, ok := progress.load(); ok {
			fillMCPResponse(&response, partial)
			response.Message = strings.Join(append(partial.notes, mcpFollowUpTimeoutNote(waiting)), "\n")
			return response, true
		}
		response.Message = mcpTimeoutMessage(timeoutHint)
		return response, true

	case result := <-answered:
		if result.err != nil {
			response.Message = mcpFailureMessage(ctx, result.err, stderr.String(), timeoutHint)
			return response, true
		}

		fillMCPResponse(&response, result)
		response.Message = strings.Join(result.notes, "\n")
		return response, true
	}
}

// fillMCPResponse überträgt, was der Server geantwortet hat. Message bleibt
// Sache des Aufrufers.
func fillMCPResponse(response *mcpToolsResponse, result mcpProbeResult) {
	response.Available = true
	response.ServerName = result.initialized.ServerInfo.Name
	response.ServerVersion = result.initialized.ServerInfo.Version
	response.ProtocolVersion = result.initialized.ProtocolVersion
	response.Capabilities = capabilityNames(result.initialized.Capabilities)
	response.Tools = describeTools(result.listed)
	response.Prompts = describePrompts(result.prompts)
	response.Resources = describeResources(result.resources)
}

// mcpProbeResult ist das Ergebnis des Dialogs mit dem Server.
//
// err ist fatal und gilt nur für initialize und tools/list: ohne sie gibt es
// nichts anzuzeigen. Scheitern die Folgeanfragen, bleibt, was bis dahin
// angekommen ist; notes sagt, welche Frage keine Antwort bekam.
type mcpProbeResult struct {
	initialized mcpInitializeResult
	listed      mcpToolsResult
	prompts     mcpPromptsResult
	resources   mcpResourcesResult
	notes       []string
	err         error
}

// Die IDs der Anfragen. Antworten werden über sie zugeordnet, nicht über die
// Reihenfolge: ein Server darf sie in anderer Folge schicken und dazwischen
// Benachrichtigungen ausgeben.
const (
	mcpInitializeID = 1
	mcpToolsListID  = 2
	mcpPromptsID    = 3
	mcpResourcesID  = 4
)

// speakMCP schickt den Handshake und sammelt die Antworten ein.
//
// Was der Server in initialize als Fähigkeiten meldet, entscheidet, was noch
// gefragt wird: prompts/list und resources/list nur, wenn er prompts bzw.
// resources kann. Einen Server nach etwas zu fragen, das er nicht kann,
// brächte eine Fehlerantwort, und die sähe aus wie ein Ausfall.
//
// stdin bleibt offen, bis alles da ist — genau wie bei einem echten Client.
// Ein sofortiges EOF beendet die Verbindung, während die Anfragen noch in
// Arbeit sind.
//
// progress bekommt vor jeder Folgeanfrage den Stand bis dahin: läuft die Frist
// ab, während sie aussteht, gehen die Werkzeuge nicht verloren.
func speakMCP(stdin io.Writer, stdout io.Reader, progress *mcpProgress) mcpProbeResult {
	if _, err := io.WriteString(stdin, mcpHandshake()); err != nil {
		return mcpProbeResult{err: fmt.Errorf("Anfragen nicht schreibbar: %w", err)}
	}

	reader := newMCPReader(stdout)
	result := mcpProbeResult{}

	if err := reader.result(mcpInitializeID, &result.initialized); err != nil {
		result.err = err
		return result
	}
	if err := reader.result(mcpToolsListID, &result.listed); err != nil {
		result.err = err
		return result
	}

	// Ab hier ist ein Fehler kein Ausfall mehr: der Server hat geantwortet und
	// seine Werkzeuge genannt. Eine Folgeanfrage, die scheitert, wird Hinweis;
	// ihre Karte bleibt leer, die Werkzeuge bleiben stehen.
	if _, ok := result.initialized.Capabilities["prompts"]; ok {
		progress.waitFor(result, "prompts/list")
		if err := askMCP(stdin, reader, mcpPromptsID, "prompts/list", &result.prompts); err != nil {
			result.prompts = mcpPromptsResult{}
			result.notes = append(result.notes, mcpFollowUpNote("prompts", "prompts/list", err))
		}
	}
	if _, ok := result.initialized.Capabilities["resources"]; ok {
		progress.waitFor(result, "resources/list")
		if err := askMCP(stdin, reader, mcpResourcesID, "resources/list", &result.resources); err != nil {
			result.resources = mcpResourcesResult{}
			result.notes = append(result.notes, mcpFollowUpNote("resources", "resources/list", err))
		}
	}
	return result
}

// mcpFollowUpNote ist der Hinweis zu einer gescheiterten Folgeanfrage.
func mcpFollowUpNote(capability string, method string, err error) string {
	return fmt.Sprintf("Der Server meldet %s, aber %s scheiterte: %s. Werkzeuge und Serverdaten oben sind vollständig.",
		capability, method, err.Error())
}

// mcpFollowUpTimeoutNote ist der Hinweis, wenn eine Folgeanfrage bis zum Ende
// der Frist ohne Antwort blieb.
func mcpFollowUpTimeoutNote(method string) string {
	return fmt.Sprintf("Der Server antwortete auf %s nicht innerhalb von %s. Werkzeuge und Serverdaten oben sind vollständig.",
		method, mcpProbeTimeout)
}

// mcpProgress ist der Stand des Dialogs, während eine Folgeanfrage aussteht.
// Geschrieben wird er von der Goroutine, die mit dem Server spricht, gelesen
// nach abgelaufener Frist im Handler — daher der Mutex. Gespeichert wird eine
// Kopie: der Dialog arbeitet danach an seinem eigenen Ergebnis weiter.
type mcpProgress struct {
	mu       sync.Mutex
	snapshot mcpProbeResult
	waiting  string
	ok       bool
}

// waitFor hält fest, was bis jetzt da ist und auf welche Anfrage gewartet wird.
func (p *mcpProgress) waitFor(result mcpProbeResult, method string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	result.notes = slices.Clone(result.notes)
	p.snapshot, p.waiting, p.ok = result, method, true
}

// load liefert den festgehaltenen Stand; ok ist false, solange initialize und
// tools/list noch ausstehen.
func (p *mcpProgress) load() (mcpProbeResult, string, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	snapshot := p.snapshot
	snapshot.notes = slices.Clone(snapshot.notes)
	return snapshot, p.waiting, p.ok
}

// askMCP schickt eine Folgeanfrage und liest ihre Antwort.
//
// Ein Schreibfehler ist hier noch kein Ergebnis: ein Server, der sich nach
// dem Handshake beendet hat, kann die Antwort schon geschickt haben, und die
// steht dann im Puffer. Erst wenn auch das Lesen scheitert, gilt der
// Schreibfehler.
func askMCP(stdin io.Writer, reader *mcpReader, id int, method string, target any) error {
	_, writeErr := io.WriteString(stdin, mcpRequest(id, method))
	if err := reader.result(id, target); err != nil {
		// Eine Fehlerantwort des Servers ist die eigentliche Auskunft und
		// geht vor: er hat sie geschickt, bevor er sich beendete, und der
		// Schreibfehler danach sagt nur, dass er weg ist.
		var answered mcpResponseError
		if writeErr != nil && !errors.As(err, &answered) {
			return fmt.Errorf("Anfragen nicht schreibbar: %w", writeErr)
		}
		return err
	}
	return nil
}

// mcpResponseError ist eine Fehlerantwort des Servers — im Unterschied zu
// einem Lese- oder Schreibfehler auf den Rohren.
type mcpResponseError string

func (e mcpResponseError) Error() string {
	return "Fehlerantwort: " + string(e)
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
		mcpRequest(mcpToolsListID, "tools/list")
}

// mcpRequest ist eine Anfrage ohne Parameter, als eine Zeile.
func mcpRequest(id int, method string) string {
	return fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"%s","params":{}}`, id, method) + "\n"
}

// mcpRPCResponse ist der Rahmen einer Antwort. Verglichen wird nur er; was das
// SDK in result schreibt, gehört ihm. ID fehlt bei Benachrichtigungen — die
// überspringt der Leser — und ist null bei einer Fehlerantwort, die der Server
// keiner Anfrage zuordnen konnte; die meldet er.
type mcpRPCResponse struct {
	ID     *json.RawMessage `json:"id"`
	Result json.RawMessage  `json:"result"`
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
	// Capabilities ist, was der Server kann — die Schlüssel zählen, ihr
	// Inhalt (etwa listChanged) nicht.
	Capabilities map[string]json.RawMessage `json:"capabilities"`
}

type mcpPromptsResult struct {
	Prompts []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Arguments   []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Required    bool   `json:"required"`
		} `json:"arguments"`
	} `json:"prompts"`
}

type mcpResourcesResult struct {
	Resources []struct {
		URI         string `json:"uri"`
		Name        string `json:"name"`
		Description string `json:"description"`
		MimeType    string `json:"mimeType"`
	} `json:"resources"`
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

// mcpReader liest Antwortzeilen und ordnet sie über die ID zu.
//
// Benachrichtigungen ohne ID — Logausgaben, die manche Server auf stdout
// schicken — werden übersprungen; eine Antwort auf eine andere als die
// erwartete Anfrage wird aufgehoben, bis sie an der Reihe ist.
type mcpReader struct {
	reader  *bufio.Reader
	pending map[string]mcpRPCResponse
}

func newMCPReader(stdout io.Reader) *mcpReader {
	return &mcpReader{reader: bufio.NewReader(stdout), pending: map[string]mcpRPCResponse{}}
}

// result wartet auf die Antwort mit der ID und packt ihr result-Feld aus.
func (r *mcpReader) result(id int, target any) error {
	wanted := fmt.Sprint(id)
	for {
		response, ok := r.pending[wanted]
		if ok {
			delete(r.pending, wanted)
			if response.Error != nil {
				return mcpResponseError(response.Error.Message)
			}
			return json.Unmarshal(response.Result, target)
		}

		line, err := r.reader.ReadString('\n')
		if err != nil {
			return err
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		var parsed mcpRPCResponse
		if err := json.Unmarshal([]byte(line), &parsed); err != nil {
			return fmt.Errorf("Antwort ist kein JSON: %w", err)
		}
		if parsed.ID == nil {
			// Ohne ID, aber mit error ist es keine Benachrichtigung, sondern
			// eine Fehlerantwort, deren Anfrage der Server nicht zuordnen
			// konnte — JSON-RPC schreibt dann id: null. Sie gilt der gerade
			// ausstehenden Anfrage; wartete der Leser weiter, liefe die
			// Messung in die Frist, statt den Grund zu nennen.
			if parsed.Error != nil {
				return mcpResponseError(parsed.Error.Message)
			}
			continue
		}
		r.pending[strings.Trim(string(*parsed.ID), `"`)] = parsed
	}
}

// mcpTimeoutMessage ist die Meldung nach abgelaufener Frist. hint wird mit
// einem Leerzeichen angehängt, wenn es einen gibt; ohne ist es der Satz, den
// /mcp immer gezeigt hat.
func mcpTimeoutMessage(hint string) string {
	message := fmt.Sprintf("Server antwortet nicht: nach %s abgebrochen.", mcpProbeTimeout)
	if hint != "" {
		message += " " + hint
	}
	return message
}

// mcpInstallTimeoutHint nennt den häufigsten Grund bei fremden Servern: npx
// und uvx laden beim ersten Start Pakete nach, und das dauert länger als die
// Frist. Ein zweiter Versuch findet sie im Cache. Nur die Detailseite gibt
// ihn mit — sie hat den Knopf, den er nennt.
const mcpInstallTimeoutHint = "Wird der Server über npx oder uvx gestartet, kann das eine Erstinstallation gewesen sein — " +
	"dann „Erneut messen“."

// mcpFailureMessage sagt, woran es lag. Die Ausgabe auf stderr kommt mit, wenn
// es eine gibt: dort steht bei einem gescheiterten Wrapper der eigentliche
// Grund. timeoutHint gilt nur, wenn die Frist der Grund war.
func mcpFailureMessage(ctx context.Context, err error, stderr string, timeoutHint string) string {
	if ctx.Err() != nil {
		return mcpTimeoutMessage(timeoutHint)
	}

	message := "Server antwortet nicht: " + err.Error()
	if stderr != "" {
		message += "\n" + stderr
	}
	return message
}

// capabilityNames sind die gemeldeten Fähigkeiten als sortierte Liste.
func capabilityNames(capabilities map[string]json.RawMessage) []string {
	names := make([]string, 0, len(capabilities))
	for name := range capabilities {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// describePrompts bringt die Vorlagen in eine Form, die sich anzeigen lässt.
func describePrompts(listed mcpPromptsResult) []mcpPrompt {
	if listed.Prompts == nil {
		return nil
	}
	prompts := make([]mcpPrompt, 0, len(listed.Prompts))
	for _, entry := range listed.Prompts {
		prompt := mcpPrompt{Name: entry.Name, Description: entry.Description}
		for _, argument := range entry.Arguments {
			prompt.Arguments = append(prompt.Arguments, mcpPromptArgument{
				Name:        argument.Name,
				Description: argument.Description,
				Required:    argument.Required,
			})
		}
		prompts = append(prompts, prompt)
	}
	return prompts
}

// describeResources bringt die Ressourcen in eine Form, die sich anzeigen
// lässt.
func describeResources(listed mcpResourcesResult) []mcpResource {
	if listed.Resources == nil {
		return nil
	}
	resources := make([]mcpResource, 0, len(listed.Resources))
	for _, entry := range listed.Resources {
		resources = append(resources, mcpResource{
			URI:         entry.URI,
			Name:        entry.Name,
			Description: entry.Description,
			MimeType:    entry.MimeType,
		})
	}
	return resources
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
