package webui

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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
	// OK: alle Dateien lesbar und kein Pflichtserver fehlt.
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
	Environment project.Environment    `json:"environment"`
	Entry       project.MCPServerEntry `json:"entry"`
	// ResolvedCommand ist der Pfad, den ein bloßer Kommandoname über die PATH
	// dieses Prozesses ergibt. Leer, wenn er sich nicht auflösen lässt — dann
	// sagt Note, warum.
	ResolvedCommand string `json:"resolvedCommand,omitempty"`
	// Probeable: der Eintrag ist lokal und ließe sich starten. Remote und
	// unknown werden nur gezeigt.
	Probeable bool   `json:"probeable"`
	Note      string `json:"note,omitempty"`
}

// mcpServerProbeResponse ist das Ergebnis einer Messung auf der Detailseite.
// Started sagt, ob überhaupt ein Prozess lief: bei remote und unknown nicht.
type mcpServerProbeResponse struct {
	mcpToolsResponse
	Started bool `json:"started"`
}

// findMCPServer sucht den Eintrag zu Assistent und Name in der gelesenen
// Liste. Nur was dort steht, lässt sich anzeigen oder messen — ein Pfad, der
// keinen Eintrag trifft, ist 404 und kein Start von irgendetwas.
func findMCPServer(projectRoot string, assistantID string, name string) (project.MCPServerEntry, bool) {
	inventory, _ := project.MCPServerInventoryFor(projectRoot)
	for _, entry := range inventory.Servers {
		if entry.AssistantID == assistantID && entry.Name == name {
			return entry, true
		}
	}
	return project.MCPServerEntry{}, false
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
	entry, ok := findMCPServer(environment.ProjectDir, r.PathValue("assistant"), r.PathValue("name"))
	if !ok {
		http.NotFound(w, r)
		return
	}

	response := mcpServerDetailResponse{Environment: environment, Entry: entry}
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
	entry, ok := findMCPServer(environment.ProjectDir, r.PathValue("assistant"), r.PathValue("name"))
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
	response.mcpToolsResponse = probeMCPCommand(environment.ProjectDir, binary, entry.Args, env)
	response.Started = true
	writeJSON(w, http.StatusOK, response)
}
