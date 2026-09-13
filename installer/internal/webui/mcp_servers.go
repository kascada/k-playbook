package webui

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// mcpServersResponse ist die Übersicht aller MCP-Server, wie die Seite
// /mcp-servers sie braucht: die gelesenen Dateien, die Server, die
// Pflichtliste und die Lücken darin.
//
// Quelle sind ausschließlich die Projektdateien der drei Assistenten. Die
// globalen Konfigurationen (~/.claude.json, ~/.config/opencode/) und die
// Freigabelisten in .claude/settings*.json werden bewusst nicht gelesen; die
// Seite sagt das.
type mcpServersResponse struct {
	Environment project.Environment `json:"environment"`
	project.MCPServerInventory
	// RequiredError ist gesetzt, wenn tools.mcp.required nicht lesbar ist —
	// etwa wegen eines unzulässigen Namens. Dann ist die Pflichtliste leer,
	// ok false, und die Seite zeigt den Fehler an der Pflichtkarte; die Server
	// stehen trotzdem da.
	RequiredError string `json:"requiredError,omitempty"`
	// OK: alle Dateien lesbar, Pflichtliste lesbar und kein Pflichtserver fehlt.
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

// mcpServersHandler liest die Übersicht. Er startet nichts: gelesen werden
// Dateien, sonst nichts.
func mcpServersHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, mcpServersState())
}

// mcpServersState stellt die Übersicht zusammen. Ohne Konfiguration gibt es
// kein Projekt und damit keine Dateien, die zu lesen wären — wie bei
// mcpState.
func mcpServersState() mcpServersResponse {
	environment := project.Detect()
	response := mcpServersResponse{Environment: environment}
	response.MCPServerInventory = project.MCPServerInventory{
		Files:    []project.MCPFileInfo{},
		Servers:  []project.MCPServerEntry{},
		Required: []string{},
		Missing:  []project.MCPRequirementGap{},
	}

	if !environment.Installed {
		response.Message = "Keine " + project.ConfigFileName + " gefunden (gesucht ab " +
			project.DisplayPath(environment.SearchedFrom) + " aufwärts)."
		return response
	}

	inventory, err := project.MCPServerInventoryFor(environment.ProjectDir)
	response.MCPServerInventory = inventory
	if err != nil {
		// `k-playbook context` bricht an derselben Datei ab. Die Oberfläche
		// bleibt bedienbar, sagt aber ebenso deutlich, dass etwas nicht stimmt.
		response.RequiredError = err.Error()
		response.Message = "Pflichtliste nicht lesbar: " + err.Error()
		return response
	}

	response.OK = len(inventory.Missing) == 0
	for _, file := range inventory.Files {
		if file.Error != "" {
			response.OK = false
			break
		}
	}
	return response
}

// mcpServerDetailResponse ist die Konfiguration eines einzelnen Servers, wie
// die Detailseite sie beim Laden zeigt. Sie startet nichts: gemessen wird
// erst auf Knopfdruck über den POST.
type mcpServerDetailResponse struct {
	Environment project.Environment  `json:"environment"`
	Entry       mcpServerDetailEntry `json:"entry"`
	// RequiredError ist gesetzt, wenn tools.mcp.required nicht lesbar ist —
	// dasselbe Feld wie in der Übersicht. entry.required ist dann null: ob
	// der Server Pflicht ist, lässt sich nicht sagen.
	RequiredError string `json:"requiredError,omitempty"`
	// ResolvedCommand ist der Pfad, den ein bloßer Kommandoname über die PATH
	// dieses Prozesses ergibt. Leer, wenn er sich nicht auflösen lässt — dann
	// sagt Note, warum.
	ResolvedCommand string `json:"resolvedCommand,omitempty"`
	// Probeable: der Eintrag ist lokal und ließe sich starten. Remote und
	// unknown werden nur gezeigt.
	Probeable bool   `json:"probeable"`
	Note      string `json:"note,omitempty"`
}

// mcpServerDetailEntry ist der Eintrag, wie die Detailseite ihn bekommt.
//
// Required überdeckt das gleichnamige Feld des Eintrags: encoding/json nimmt
// das weniger tief eingebettete. Als Zeiger kann es null sein — bei nicht
// lesbarer Pflichtliste stünde dort sonst false, und die Seite behauptete
// „Pflicht: nein".
type mcpServerDetailEntry struct {
	project.MCPServerEntry
	Required *bool `json:"required"`
}

// mcpServerProbeResponse ist das Ergebnis einer Messung auf der Detailseite.
// Started sagt, ob überhaupt ein Prozess lief: bei remote und unknown nicht,
// und auch dann nicht, wenn das Kommando fehlt oder sich nicht starten ließ.
// Den Wert liefert probeMCPCommand selbst; der Handler setzt nichts dazu.
type mcpServerProbeResponse struct {
	mcpToolsResponse
	Started bool `json:"started"`
}

// findMCPServer sucht den Eintrag zu Assistent und Name in der gelesenen
// Liste. Nur was dort steht, lässt sich anzeigen oder messen — ein Pfad, der
// keinen Eintrag trifft, ist 404 und kein Start von irgendetwas.
//
// Gelesen werden nur die MCP-Dateien, nicht die Pflichtliste: Seite, GET und
// POST rufen den Lookup je einmal auf, und nur das GET braucht Required — es
// liest die Liste selbst. Required ist im Ergebnis deshalb immer false.
//
// file unterscheidet gleichnamige Einträge, wenn opencode.json und
// opencode.jsonc nebeneinander liegen. Es ist ein reiner Vergleichswert gegen
// MCPServerEntry.File aus der gelesenen Liste und wird nie als Pfad geöffnet;
// gesetzt, aber ohne Treffer, ist es 404 wie ein unbekannter Name. Leer gilt
// der erste Treffer.
func findMCPServer(projectRoot string, assistantID string, name string, file string) (project.MCPServerEntry, bool) {
	servers, _ := project.ListMCPServers(projectRoot)
	for _, entry := range servers {
		if entry.AssistantID != assistantID || entry.Name != name {
			continue
		}
		if file != "" && entry.File != file {
			continue
		}
		return entry, true
	}
	return project.MCPServerEntry{}, false
}

// mcpServerFileParam ist der optionale Query-Parameter, der unter
// gleichnamigen Einträgen den aus einer bestimmten Datei wählt.
func mcpServerFileParam(r *http.Request) string {
	return r.URL.Query().Get("file")
}

// resolveMCPCommand löst das Kommando eines lokalen Eintrags auf: ein bloßer
// Name über die PATH dieses Prozesses, ein Pfad relativ zum Hauptverzeichnis.
func resolveMCPCommand(projectRoot string, command string) (string, error) {
	if strings.ContainsRune(command, '/') || strings.ContainsRune(command, os.PathSeparator) {
		if filepath.IsAbs(command) {
			return command, nil
		}
		return filepath.Join(projectRoot, command), nil
	}
	return exec.LookPath(command)
}

// mcpServerDetailHandler liefert die Konfiguration eines Servers. GET startet
// nichts — auch nicht den eigenen Server.
func mcpServerDetailHandler(w http.ResponseWriter, r *http.Request) {
	environment := project.Detect()
	if !environment.Installed {
		http.NotFound(w, r)
		return
	}
	entry, ok := findMCPServer(environment.ProjectDir, r.PathValue("assistant"), r.PathValue("name"), mcpServerFileParam(r))
	if !ok {
		http.NotFound(w, r)
		return
	}

	response := mcpServerDetailResponse{Environment: environment, Entry: mcpServerDetailEntry{MCPServerEntry: entry}}
	// Die Pflichtliste wird hier einmal gelesen, nicht im Lookup: Seite und
	// Messung brauchen sie nicht.
	if required, _, err := project.ReadRequiredMCPServers(environment.ProjectDir); err != nil {
		response.RequiredError = err.Error()
	} else {
		isRequired := slices.Contains(required, entry.Name)
		response.Entry.Required = &isRequired
		response.Entry.MCPServerEntry.Required = isRequired
	}

	switch entry.Transport {
	case project.MCPTransportLocal:
		response.Probeable = true
		resolved, err := resolveMCPCommand(environment.ProjectDir, entry.Command)
		if err != nil {
			response.Note = "Das Kommando " + entry.Command + " ist über die PATH dieses Prozesses nicht auffindbar. " +
				"Messen wird daran scheitern; der Assistent findet es vielleicht über seine eigene PATH."
		} else {
			response.ResolvedCommand = resolved
		}
	case project.MCPTransportRemote:
		response.Note = "Ein Remote-Server wird nicht angesprochen: er läuft hinter einer URL, " +
			"und seine Anmeldung — bei OAuth-Servern die Token des Assistenten — liegt hier nicht vor."
	default:
		response.Note = "Der Eintrag ist weder als lokales Kommando noch als Remote-URL erkennbar. " +
			"Er wird gezeigt, wie er dasteht, und nicht gestartet."
	}
	writeJSON(w, http.StatusOK, response)
}

// mcpProbeLocks serialisiert die Messungen je Servername: zwei Klicks kurz
// nacheinander starten denselben Server nicht zweimal, der zweite wartet.
var mcpProbeLocks sync.Map

func mcpProbeLock(name string) *sync.Mutex {
	lock, _ := mcpProbeLocks.LoadOrStore(name, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

// mcpServerProbeHandler misst einen Server: startet das Kommando aus der
// Projektdatei mit dem Hauptverzeichnis als Arbeitsverzeichnis, der geerbten
// Umgebung und env aus dem Eintrag.
//
// Ausschließlich POST, ausschließlich auf Knopfdruck. Was hier läuft,
// bestimmt, wer die Projektdateien schreibt — dieselbe Vertrauensgrenze wie
// beim Assistenten, der dieselben Kommandos startet. Ein GET oder ein
// Seitenaufruf löst nie eine Messung aus, damit eine fremde Seite das nicht
// über einen bloßen Verweis kann.
func mcpServerProbeHandler(w http.ResponseWriter, r *http.Request) {
	environment := project.Detect()
	if !environment.Installed {
		http.NotFound(w, r)
		return
	}
	// Beide Einträge desselben Namens teilen den Mutex unten: er hängt am
	// Namen, nicht an der Datei.
	entry, ok := findMCPServer(environment.ProjectDir, r.PathValue("assistant"), r.PathValue("name"), mcpServerFileParam(r))
	if !ok {
		http.NotFound(w, r)
		return
	}

	response := mcpServerProbeResponse{mcpToolsResponse: mcpToolsResponse{Capabilities: []string{}, Tools: []mcpTool{}}}
	switch entry.Transport {
	case project.MCPTransportRemote:
		response.Command = entry.URL
		response.Message = "Nicht gemessen: ein Remote-Server wird nicht angesprochen. " +
			"Seine Anmeldung liegt beim Assistenten, nicht hier."
		writeJSON(w, http.StatusOK, response)
		return
	case project.MCPTransportUnknown:
		response.Message = "Nicht gemessen: der Eintrag ist weder lokal noch remote erkennbar und wird nicht gestartet."
		writeJSON(w, http.StatusOK, response)
		return
	}

	binary, err := resolveMCPCommand(environment.ProjectDir, entry.Command)
	if err != nil {
		response.Command = strings.Join(append([]string{entry.Command}, entry.Args...), " ")
		response.Message = "Server ließ sich nicht starten: " + err.Error()
		writeJSON(w, http.StatusOK, response)
		return
	}

	lock := mcpProbeLock(entry.Name)
	lock.Lock()
	defer lock.Unlock()

	env := append(os.Environ(), entry.Environ()...)
	response.mcpToolsResponse, response.Started = probeMCPCommand(environment.ProjectDir, binary, entry.Args, env, mcpInstallTimeoutHint)
	writeJSON(w, http.StatusOK, response)
}
