package project

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
)

// MCPServerKey ist der Schlüssel, unter dem k-playbook seinen Server registriert
// — derselbe Name, den die Handarbeit in .mcp.json schon benutzt.
//
// Der Schlüssel gehört k-playbook. Steht dort etwas anderes, ist das kein
// Konflikt, sondern ein falscher Stand: er wird gemeldet und beim Einrichten
// überschrieben. Alle anderen Schlüssel derselben Datei gehören dem Projekt und
// bleiben unberührt.
const MCPServerKey = "k-playbook"

// opencodeSchemaKey und opencodeSchemaURL sind der Verweis auf das Config-Schema,
// den OpenCode in jeder Projektkonfiguration erwartet.
//
// Fehlt er, trägt OpenCode ihn beim nächsten Start selbst nach und schreibt die
// Datei dabei zurück. Eine Datei, die hier neu entsteht, bekommt ihn deshalb
// gleich mit: dann bleibt sie so liegen, wie k-playbook sie hinterlassen hat.
//
// Nachgetragen wird er ausschließlich bei Neuanlage. Eine Datei, die schon dem
// Projekt gehört, wird nicht um Schlüssel ergänzt, die niemand angefordert hat —
// auch nicht um diesen.
const (
	opencodeSchemaKey = "$schema"
	opencodeSchemaURL = "https://opencode.ai/config.json"
)

// MCPSchema unterscheidet die beiden Schreibweisen, in denen ein Assistent
// seine MCP-Server notiert.
type MCPSchema string

const (
	// MCPSchemaServers ist das Schema von Claude Code und Cursor: unter
	// "mcpServers" ein Objekt mit "command" und "args".
	MCPSchemaServers MCPSchema = "mcpServers"
	// MCPSchemaOpenCode ist das Schema von OpenCode: unter "mcp" ein Objekt mit
	// "type", "enabled" und einem "command"-**Array** aus Kommando und
	// Argumenten — nicht zwei Feldern.
	MCPSchemaOpenCode MCPSchema = "mcp"
)

// MCPTarget ist eine Datei, in der ein Assistent seine MCP-Server liest.
type MCPTarget struct {
	// Path ist relativ zur Projektwurzel.
	Path string `json:"path"`
	// Assistant nennt, wer hier liest — nur für die Anzeige.
	Assistant string `json:"assistant"`
	// Schema ist zugleich der Schlüssel, unter dem die Server in der Datei
	// stehen.
	Schema MCPSchema `json:"schema"`
}

// MCPTargets sind die drei Registrierungen, die ein Zielprojekt braucht.
//
// Anders als bei der Verlinkung gehören diese Dateien vollständig dem Projekt:
// sie können fremde MCP-Server tragen, und opencode.json trägt daneben noch
// ganz andere Einstellungen. Angefasst wird deshalb nur der eigene Eintrag.
func MCPTargets() []MCPTarget {
	return []MCPTarget{
		{Path: ".mcp.json", Assistant: "Claude Code", Schema: MCPSchemaServers},
		{Path: filepath.Join(".cursor", "mcp.json"), Assistant: "Cursor", Schema: MCPSchemaServers},
		{Path: "opencode.json", Assistant: "OpenCode", Schema: MCPSchemaOpenCode},
	}
}

// MCPState ist der Zustand einer einzelnen Registrierung.
type MCPState string

const (
	// MCPStateOK: der Eintrag steht so, wie er stehen soll.
	MCPStateOK MCPState = "ok"
	// MCPStateNoWrapper: der registrierte Wrapper existiert nicht. Dann wird
	// nichts geschrieben — sonst meldete ein frischer Clone ohne k-playbook/
	// eine Registrierung als "steht richtig", die auf nichts zeigt.
	MCPStateNoWrapper MCPState = "no-wrapper"
	// MCPStateMissingFile: die Datei gibt es noch nicht.
	MCPStateMissingFile MCPState = "missing-file"
	// MCPStateMissingEntry: die Datei steht, unser Eintrag fehlt darin.
	MCPStateMissingEntry MCPState = "missing-entry"
	// MCPStateStale: der Eintrag steht, zeigt aber woandershin.
	MCPStateStale MCPState = "stale"
	// MCPStateUnreadable: die Datei lässt sich nicht als JSON-Objekt lesen. Sie
	// wird nicht angefasst — sonst ginge die Handarbeit eines Projekts verloren.
	MCPStateUnreadable MCPState = "unreadable"
)

// MCPStatus ist der geprüfte Zustand einer Registrierung.
type MCPStatus struct {
	MCPTarget
	State  MCPState `json:"state"`
	Detail string   `json:"detail"`
}

// OK meldet, ob die Registrierung steht.
func (s MCPStatus) OK() bool { return s.State == MCPStateOK }

// MCPCommand ist das Kommando, das registriert wird: der projekteigene Wrapper
// relativ zum Hauptverzeichnis, gefolgt vom Subkommando.
//
// Der Schrägstrich ist hier kein Dateisystempfad, sondern Teil eines Wertes in
// einer Konfigurationsdatei — deshalb ToSlash und nicht der Trenner der
// Plattform.
func MCPCommand() (string, []string) {
	return filepath.ToSlash(WrapperPath()), []string{"mcp"}
}

// CheckMCP prüft den Zustand, ohne etwas zu verändern.
func CheckMCP(projectRoot string) []MCPStatus {
	targets := MCPTargets()
	statuses := make([]MCPStatus, 0, len(targets))
	for _, target := range targets {
		statuses = append(statuses, checkMCPTarget(projectRoot, target))
	}
	return statuses
}

// MCPOK meldet, ob nichts mehr einzurichten ist.
func MCPOK(statuses []MCPStatus) bool {
	for _, status := range statuses {
		if !status.OK() {
			return false
		}
	}
	return len(statuses) > 0
}

// ApplyMCP setzt den Eintrag in allen drei Dateien und meldet den Zustand
// danach.
//
// Fehlt der Wrapper, wird nichts geschrieben: eine Registrierung, die auf nichts
// zeigt, ist schlechter als keine. Eine Datei, die kein JSON-Objekt ist, bleibt
// ebenfalls unberührt; die Prüfung meldet sie.
//
// Die drei Ziele sind voneinander unabhängig, deshalb hält ein gescheitertes die
// anderen nicht auf: ein nicht schreibbares .cursor/ ist kein Grund, OpenCode
// unregistriert zu lassen. Gesammelt wird, was schiefging; welches Ziel steht
// und welches nicht, sagt ohnehin der zurückgegebene Zustand.
func ApplyMCP(projectRoot string) ([]MCPStatus, error) {
	if !wrapperPresent(projectRoot) {
		return CheckMCP(projectRoot), nil
	}

	var failures []error
	for _, target := range MCPTargets() {
		if err := applyMCPTarget(projectRoot, target); err != nil {
			failures = append(failures, err)
		}
	}
	return CheckMCP(projectRoot), errors.Join(failures...)
}

// wrapperPresent meldet, ob der registrierte Wrapper tatsächlich im Projekt
// liegt.
func wrapperPresent(projectRoot string) bool {
	return fileExists(filepath.Join(projectRoot, WrapperPath()))
}

// mcpEntry ist der Eintrag, der unter MCPServerKey steht — je Schema anders
// geformt, aber immer derselbe Wrapper mit demselben Subkommando.
//
// Die Werte sind so getypt, wie encoding/json sie beim Lesen zurückgibt.
// Nur dadurch lässt sich der vorgefundene Eintrag ohne Umweg mit dem
// gewünschten vergleichen.
func mcpEntry(schema MCPSchema) map[string]any {
	command, args := MCPCommand()

	if schema == MCPSchemaOpenCode {
		line := make([]any, 0, len(args)+1)
		line = append(line, command)
		for _, arg := range args {
			line = append(line, arg)
		}
		return map[string]any{"type": "local", "command": line, "enabled": true}
	}

	line := make([]any, 0, len(args))
	for _, arg := range args {
		line = append(line, arg)
	}
	return map[string]any{"command": command, "args": line}
}

func checkMCPTarget(projectRoot string, target MCPTarget) MCPStatus {
	status := MCPStatus{MCPTarget: target}

	if !wrapperPresent(projectRoot) {
		status.State = MCPStateNoWrapper
		status.Detail = WrapperPath() + " fehlt im Projekt"
		return status
	}

	content, exists, err := readJSONObject(filepath.Join(projectRoot, target.Path))
	switch {
	case err != nil:
		status.State = MCPStateUnreadable
		status.Detail = err.Error()
		return status

	case !exists:
		status.State = MCPStateMissingFile
		status.Detail = "nicht vorhanden"
		return status
	}

	section, ok := mcpSection(content, target.Schema)
	if !ok {
		status.State = MCPStateUnreadable
		status.Detail = string(target.Schema) + " ist kein Objekt"
		return status
	}

	found, present := section[MCPServerKey]
	switch {
	case !present:
		status.State = MCPStateMissingEntry
		status.Detail = "Eintrag " + MCPServerKey + " fehlt"

	case reflect.DeepEqual(found, mcpEntry(target.Schema)):
		command, _ := MCPCommand()
		status.State = MCPStateOK
		status.Detail = "-> " + command

	default:
		status.State = MCPStateStale
		status.Detail = "zeigt auf " + describeMCPEntry(found)
	}
	return status
}

// mcpSection holt den Abschnitt, unter dem die Server stehen. Fehlt er, ist das
// kein Fehler — nur ein leerer Abschnitt. Steht dort etwas anderes als ein
// Objekt, meldet das zweite Ergebnis false.
func mcpSection(content map[string]any, schema MCPSchema) (map[string]any, bool) {
	value, present := content[string(schema)]
	if !present {
		return map[string]any{}, true
	}

	section, ok := value.(map[string]any)
	if !ok {
		return nil, false
	}
	return section, true
}

// describeMCPEntry nennt den vorgefundenen Wert so kurz wie möglich: das
// Kommando, wenn eines erkennbar ist, sonst die rohe Form.
func describeMCPEntry(value any) string {
	entry, ok := value.(map[string]any)
	if !ok {
		return fmt.Sprintf("%v", value)
	}

	switch command := entry["command"].(type) {
	case string:
		return command
	case []any:
		if len(command) > 0 {
			return fmt.Sprintf("%v", command[0])
		}
	}
	return "einen anderen Eintrag"
}

// applyMCPTarget setzt den eigenen Eintrag und lässt alles andere in der Datei
// stehen. Eine fehlende Datei entsteht dabei neu — bei OpenCode samt $schema,
// damit der Assistent sie nicht seinerseits umschreiben muss.
//
// Nicht erhalten bleibt die Formatierung: gelesen und geschrieben wird über
// map[string]any, und ein solcher Round-Trip sortiert die Schlüssel
// alphabetisch und setzt die Einrückung auf zwei Leerzeichen. Ordnungserhaltend
// zu schreiben wäre ein eigener Parser.
func applyMCPTarget(projectRoot string, target MCPTarget) error {
	path := filepath.Join(projectRoot, target.Path)

	content, exists, err := readJSONObject(path)
	if err != nil {
		// Nicht lesbar heißt: nicht anfassen. Die Prüfung meldet es.
		return nil
	}

	if !exists && target.Schema == MCPSchemaOpenCode {
		content[opencodeSchemaKey] = opencodeSchemaURL
	}

	section, ok := mcpSection(content, target.Schema)
	if !ok {
		return nil
	}

	section[MCPServerKey] = mcpEntry(target.Schema)
	content[string(target.Schema)] = section

	encoded, err := json.MarshalIndent(content, "", "  ")
	if err != nil {
		return fmt.Errorf("%s kodieren: %w", target.Path, err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("%s anlegen: %w", filepath.Dir(target.Path), err)
	}
	if err := os.WriteFile(path, append(encoded, '\n'), 0o644); err != nil {
		return fmt.Errorf("%s schreiben: %w", target.Path, err)
	}
	return nil
}

// readJSONObject liest eine Datei als JSON-Objekt. Eine fehlende Datei ist kein
// Fehler: der Aufrufer bekommt ein leeres Objekt und exists false.
func readJSONObject(path string) (content map[string]any, exists bool, err error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, false, nil
		}
		return nil, false, fmt.Errorf("nicht lesbar: %w", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, true, fmt.Errorf("kein gültiges JSON: %w", err)
	}
	if decoded == nil {
		// `null` ist gültiges JSON, aber kein Objekt, in das sich etwas
		// eintragen ließe.
		return nil, true, fmt.Errorf("kein JSON-Objekt")
	}
	return decoded, true, nil
}
