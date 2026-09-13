package project

import (
	"fmt"
	"path/filepath"
	"sort"
)

// MCPTransport sagt, wie ein Assistent einen MCP-Server erreicht.
type MCPTransport string

const (
	// MCPTransportLocal: ein Kommando, das der Assistent selbst startet und
	// über stdin/stdout spricht.
	MCPTransportLocal MCPTransport = "local"
	// MCPTransportRemote: eine URL, hinter der der Server schon läuft — HTTP
	// oder SSE.
	MCPTransportRemote MCPTransport = "remote"
	// MCPTransportUnknown: der Eintrag passt in keine der beiden Formen. Er
	// wird gezeigt, wie er dasteht, aber nie gestartet.
	MCPTransportUnknown MCPTransport = "unknown"
)

// MCPAssistant ist einer der drei Assistenten, wie ihn die Übersicht führt:
// ID für Pfade und Vergleiche, Name für die Anzeige.
type MCPAssistant struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// MCPAssistants sind die drei Assistenten in der Reihenfolge der Matrix.
//
// Die Namen sind dieselben, die MCPTargets in seinen Zielen trägt; die IDs
// sind die Pfadsegmente der Detailseite.
func MCPAssistants() []MCPAssistant {
	return []MCPAssistant{
		{ID: "claude-code", Name: "Claude Code"},
		{ID: "opencode", Name: "OpenCode"},
		{ID: "cursor", Name: "Cursor"},
	}
}

// MCPAssistantID übersetzt den Anzeigenamen eines Ziels in die ID. Für einen
// unbekannten Namen ist das zweite Ergebnis false.
func MCPAssistantID(name string) (string, bool) {
	for _, assistant := range MCPAssistants() {
		if assistant.Name == name {
			return assistant.ID, true
		}
	}
	return "", false
}

// MCPServerEntry ist ein Server, wie er in einer der Projektdateien steht.
//
// Die Werte aus `env` und `environment` gehören nicht in eine Antwort: dort
// stehen Tokens. Sie bleiben im unexportierten Feld und erreichen nur den
// Start der Messung; nach außen gehen ausschließlich die Schlüsselnamen.
type MCPServerEntry struct {
	Name string `json:"name"`
	// Assistant ist der Anzeigename, AssistantID das Pfadsegment.
	Assistant   string `json:"assistant"`
	AssistantID string `json:"assistantId"`
	// File ist die gelesene Datei, relativ zur Projektwurzel.
	File      string       `json:"file"`
	Schema    MCPSchema    `json:"schema"`
	Transport MCPTransport `json:"transport"`
	// Command und Args sind nur bei local gesetzt, URL nur bei remote.
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args"`
	URL     string   `json:"url,omitempty"`
	// Enabled ist das OpenCode-Flag; fehlt es, gilt true. Die beiden anderen
	// Schemata kennen kein solches Flag, dort ist ein Eintrag immer aktiv.
	Enabled bool `json:"enabled"`
	// Own: der Eintrag ist die Registrierung von k-playbook selbst.
	Own bool `json:"own"`
	// Required: der Name steht in tools.mcp.required.
	Required bool `json:"required"`
	// EnvKeys sind die Schlüsselnamen aus env/environment, sortiert — ohne
	// Werte.
	EnvKeys []string `json:"envKeys"`

	env map[string]string
}

// Environ liefert env/environment als Zeilen der Form NAME=WERT, so wie ein
// Prozessstart sie braucht. Sortiert, damit zwei Starts dieselbe Umgebung
// bekommen.
func (e MCPServerEntry) Environ() []string {
	environ := make([]string, 0, len(e.env))
	for _, key := range e.EnvKeys {
		environ = append(environ, key+"="+e.env[key])
	}
	return environ
}

// MCPFileInfo ist der Zustand einer gelesenen Datei.
type MCPFileInfo struct {
	Path        string    `json:"path"`
	Assistant   string    `json:"assistant"`
	AssistantID string    `json:"assistantId"`
	Schema      MCPSchema `json:"schema"`
	Exists      bool      `json:"exists"`
	// Error ist gesetzt, wenn die Datei da, aber nicht lesbar ist. Die anderen
	// Dateien werden trotzdem gelesen.
	Error string `json:"error,omitempty"`
	// Count ist die Zahl der Server in dieser Datei.
	Count int `json:"count"`
	// Ambiguous: opencode.json und opencode.jsonc liegen nebeneinander. Beide
	// werden gelesen; welcher Eintrag am Ende wirkt, ist von außen nicht zu
	// sehen.
	Ambiguous bool `json:"ambiguous"`
}

// MCPRequirementGap ist ein Pflichtserver, der bei einem Assistenten fehlt.
type MCPRequirementGap struct {
	Name        string `json:"name"`
	Assistant   string `json:"assistant"`
	AssistantID string `json:"assistantId"`
}

// MCPServerInventory ist alles, was die Übersicht braucht: die gelesenen
// Dateien, die Server, die Pflichtliste und die Lücken darin.
type MCPServerInventory struct {
	Files   []MCPFileInfo    `json:"files"`
	Servers []MCPServerEntry `json:"servers"`
	// Required ist die Pflichtliste aus tools.mcp.required; RequiredConfigured
	// meldet, ob der Block überhaupt dastand.
	Required           []string            `json:"required"`
	RequiredConfigured bool                `json:"requiredConfigured"`
	Missing            []MCPRequirementGap `json:"missing"`
}

// mcpListTargets sind die Dateien, die für die Übersicht gelesen werden: die
// drei Registrierungsziele und, wenn beide OpenCode-Endungen nebeneinander
// liegen, auch die zweite. Geschrieben wird nur in die erste — gelesen werden
// beide, weil OpenCode beide zusammenführt.
func mcpListTargets(projectRoot string) []MCPTarget {
	targets := MCPTargets(projectRoot)
	if opencodeAmbiguous(projectRoot) {
		targets = append(targets, MCPTarget{Path: opencodeConfigJSONC, Assistant: "OpenCode", Schema: MCPSchemaOpenCode})
	}
	return targets
}

// ListMCPServers liest alle Server aus den Projektdateien der drei
// Assistenten. Eine unlesbare Datei hält die anderen nicht auf: ihr Zustand
// steht in der Dateiliste, ihre Server fehlen.
func ListMCPServers(projectRoot string) ([]MCPServerEntry, []MCPFileInfo) {
	targets := mcpListTargets(projectRoot)
	ambiguous := opencodeAmbiguous(projectRoot)

	servers := []MCPServerEntry{}
	files := make([]MCPFileInfo, 0, len(targets))
	for _, target := range targets {
		assistantID, _ := MCPAssistantID(target.Assistant)
		info := MCPFileInfo{
			Path:        target.Path,
			Assistant:   target.Assistant,
			AssistantID: assistantID,
			Schema:      target.Schema,
			Ambiguous:   target.Schema == MCPSchemaOpenCode && ambiguous,
		}

		doc, exists, err := readJSONObject(filepath.Join(projectRoot, target.Path))
		info.Exists = exists
		if err != nil {
			info.Error = err.Error()
			files = append(files, info)
			continue
		}
		section, ok := mcpSection(doc.content, target.Schema)
		if !ok {
			info.Error = string(target.Schema) + " ist kein Objekt"
			files = append(files, info)
			continue
		}

		names := make([]string, 0, len(section))
		for name := range section {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			servers = append(servers, describeMCPServer(target, assistantID, name, section[name]))
		}
		info.Count = len(names)
		files = append(files, info)
	}
	return servers, files
}

// describeMCPServer liest einen Eintrag tolerant: was als lokal oder remote
// erkennbar ist, wird so beschrieben; alles andere ist unknown und wird nur
// gezeigt.
//
// Claude Code und Cursor: `command` + `args` ist lokal, `type: http|sse` mit
// `url` ist remote. OpenCode: `type: local` mit `command`-Array, `type: remote`
// mit `url`; fehlt `type`, entscheidet die Form. `enabled` fehlt → true.
func describeMCPServer(target MCPTarget, assistantID string, name string, value any) MCPServerEntry {
	entry := MCPServerEntry{
		Name:        name,
		Assistant:   target.Assistant,
		AssistantID: assistantID,
		File:        target.Path,
		Schema:      target.Schema,
		Transport:   MCPTransportUnknown,
		Args:        []string{},
		Enabled:     true,
		Own:         name == MCPServerKey,
		EnvKeys:     []string{},
	}

	fields, ok := value.(map[string]any)
	if !ok {
		return entry
	}

	if enabled, ok := fields["enabled"].(bool); ok {
		entry.Enabled = enabled
	}

	kind, _ := fields["type"].(string)
	url, _ := fields["url"].(string)

	if target.Schema == MCPSchemaOpenCode {
		line, _ := fields["command"].([]any)
		switch {
		case kind == "local" || (kind == "" && len(line) > 0):
			if command, ok := firstString(line); ok {
				entry.Transport = MCPTransportLocal
				entry.Command = command
				entry.Args = stringItems(line[1:])
			}
		case kind == "remote" || (kind == "" && url != ""):
			if url != "" {
				entry.Transport = MCPTransportRemote
				entry.URL = url
			}
		}
		entry.env, entry.EnvKeys = stringMap(fields["environment"])
		return entry
	}

	command, _ := fields["command"].(string)
	switch {
	case kind == "http" || kind == "sse" || (kind == "" && command == "" && url != ""):
		if url != "" {
			entry.Transport = MCPTransportRemote
			entry.URL = url
		}
	case command != "" && (kind == "" || kind == "stdio"):
		entry.Transport = MCPTransportLocal
		entry.Command = command
		if raw, ok := fields["args"].([]any); ok {
			entry.Args = stringItems(raw)
		}
	}
	entry.env, entry.EnvKeys = stringMap(fields["env"])
	return entry
}

func firstString(values []any) (string, bool) {
	if len(values) == 0 {
		return "", false
	}
	first, ok := values[0].(string)
	return first, ok && first != ""
}

// stringItems nimmt die Zeichenketten einer Liste und übergeht den Rest: ein
// Zahlenwert in args macht den Eintrag nicht unlesbar, er fällt nur weg.
func stringItems(values []any) []string {
	items := make([]string, 0, len(values))
	for _, value := range values {
		if text, ok := value.(string); ok {
			items = append(items, text)
		}
	}
	return items
}

// stringMap liest ein Objekt aus Umgebungsvariablen: Werte als Text, Schlüssel
// sortiert. Ein anderer Typ ergibt eine leere Map.
func stringMap(value any) (map[string]string, []string) {
	fields, ok := value.(map[string]any)
	if !ok {
		return map[string]string{}, []string{}
	}
	result := make(map[string]string, len(fields))
	keys := make([]string, 0, len(fields))
	for key, raw := range fields {
		result[key] = fmt.Sprint(raw)
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return result, keys
}

// newMCPServerInventory setzt Liste und Pflichtliste zusammen: markiert die
// Pflichtserver und rechnet aus, welcher Pflichtname bei welchem Assistenten
// fehlt. Ein Eintrag in einer der beiden OpenCode-Dateien genügt.
func newMCPServerInventory(servers []MCPServerEntry, files []MCPFileInfo, required []string, configured bool) MCPServerInventory {
	inventory := MCPServerInventory{
		Files:              files,
		Servers:            servers,
		Required:           append([]string{}, required...),
		RequiredConfigured: configured,
		Missing:            []MCPRequirementGap{},
	}
	if inventory.Required == nil {
		inventory.Required = []string{}
	}

	requiredNames := map[string]bool{}
	for _, name := range required {
		requiredNames[name] = true
	}
	present := map[string]map[string]bool{}
	for index := range inventory.Servers {
		entry := &inventory.Servers[index]
		entry.Required = requiredNames[entry.Name]
		if present[entry.Name] == nil {
			present[entry.Name] = map[string]bool{}
		}
		present[entry.Name][entry.AssistantID] = true
	}

	for _, name := range required {
		for _, assistant := range MCPAssistants() {
			if !present[name][assistant.ID] {
				inventory.Missing = append(inventory.Missing, MCPRequirementGap{
					Name:        name,
					Assistant:   assistant.Name,
					AssistantID: assistant.ID,
				})
			}
		}
	}
	return inventory
}
