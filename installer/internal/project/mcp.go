package project

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/tailscale/hujson"
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
	opencodeSchemaKey       = "$schema"
	opencodeSchemaURL       = "https://opencode.ai/config.json"
	opencodeInstructionsKey = "instructions"
	opencodeReferencesKey   = "references"
	opencodeDocsKey         = "docs"
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

// opencodeConfigJSON und opencodeConfigJSONC sind die beiden Endungen, unter
// denen OpenCode seine Projektkonfiguration liest. Beide gelten gleichzeitig:
// liegen sie nebeneinander, führt OpenCode sie tief zusammen.
const (
	opencodeConfigJSON  = "opencode.json"
	opencodeConfigJSONC = "opencode.jsonc"
)

// MCPTargets sind die drei Registrierungen, die ein Zielprojekt braucht.
//
// Anders als bei der Verlinkung gehören diese Dateien vollständig dem Projekt:
// sie können fremde MCP-Server tragen, und die OpenCode-Konfiguration trägt
// daneben noch ganz andere Einstellungen. Angefasst wird deshalb nur der eigene
// Eintrag.
//
// Das OpenCode-Ziel hängt davon ab, was im Projekt liegt — deshalb der
// projectRoot: es gibt keine feste Liste, sondern eine Regel.
func MCPTargets(projectRoot string) []MCPTarget {
	return []MCPTarget{
		{Path: ".mcp.json", Assistant: "Claude Code", Schema: MCPSchemaServers},
		{Path: filepath.Join(".cursor", "mcp.json"), Assistant: "Cursor", Schema: MCPSchemaServers},
		{Path: opencodeTarget(projectRoot), Assistant: "OpenCode", Schema: MCPSchemaOpenCode},
	}
}

// opencodeTarget wählt die Datei, in die der OpenCode-Eintrag geht:
// opencode.jsonc nur, wenn sie da ist und opencode.json fehlt — sonst
// opencode.json.
//
// Die Regel entscheidet immer eindeutig und legt nie eine zweite Datei an. Wer
// seine Konfiguration mit Kommentaren pflegt, behält sie; wer keine hat,
// bekommt die gebräuchlichere Endung.
func opencodeTarget(projectRoot string) string {
	if opencodeConfigExists(projectRoot, opencodeConfigJSONC) &&
		!opencodeConfigExists(projectRoot, opencodeConfigJSON) {
		return opencodeConfigJSONC
	}
	return opencodeConfigJSON
}

// opencodeAmbiguous meldet, ob beide Endungen nebeneinander liegen.
//
// Dann ist der wirksame Zustand aus keiner der beiden Dateien ablesbar: was
// hier eingetragen wird, kann der Eintrag in der anderen Datei beim Merge
// überstimmen. Geschrieben wird trotzdem nur in opencode.json — gemeldet wird
// die Doppelung.
func opencodeAmbiguous(projectRoot string) bool {
	return opencodeConfigExists(projectRoot, opencodeConfigJSON) &&
		opencodeConfigExists(projectRoot, opencodeConfigJSONC)
}

func opencodeConfigExists(projectRoot, name string) bool {
	return fileExists(filepath.Join(projectRoot, name))
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
	// Kommentare und Trailing Commas sind kein Grund dafür; die werden gelesen.
	MCPStateUnreadable MCPState = "unreadable"
	// MCPStateAmbiguousTarget: opencode.json und opencode.jsonc liegen beide
	// vor. Der Eintrag steht in opencode.json, aber OpenCode führt beide Dateien
	// zusammen — was dabei gewinnt, ist von außen nicht zu sehen. Solange das so
	// ist, gilt die Registrierung nicht als in Ordnung: hier muss jemand eine
	// der beiden Dateien auflösen.
	MCPStateAmbiguousTarget MCPState = "ambiguous-target"
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
	targets := MCPTargets(projectRoot)
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
	for _, target := range MCPTargets(projectRoot) {
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

	doc, exists, err := readJSONObject(filepath.Join(projectRoot, target.Path))
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

	section, ok := mcpSection(doc.content, target.Schema)
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
		// Der Eintrag steht — aber bei zwei OpenCode-Configs sagt das nichts
		// darüber, was am Ende wirkt.
		if target.Schema == MCPSchemaOpenCode && opencodeAmbiguous(projectRoot) {
			status.State = MCPStateAmbiguousTarget
			status.Detail = opencodeConfigJSON + " und " + opencodeConfigJSONC + " liegen nebeneinander"
			return status
		}
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
// In eine vorhandene Datei wird nicht der ganze Inhalt zurückgeschrieben,
// sondern nur der eine Schlüssel gepatcht. Kommentare, Trailing Commas,
// Schlüsselreihenfolge und fremde Einträge bleiben damit erhalten. Sichtbar
// bleibt eine Nebenwirkung: der abschließende Format() rückt die Datei
// einheitlich mit Tabs ein. Das geschieht einmal, beim ersten Schreiben.
//
// Steht der Eintrag bereits richtig, wird gar nicht geschrieben. Sonst würde
// jeder Lauf eine fremde Datei erneut umformatieren.
func applyMCPTarget(projectRoot string, target MCPTarget) error {
	path := filepath.Join(projectRoot, target.Path)

	doc, exists, err := readJSONObject(path)
	if err != nil {
		// Nicht lesbar heißt: nicht anfassen. Die Prüfung meldet es.
		return nil
	}

	section, ok := mcpSection(doc.content, target.Schema)
	if !ok {
		return nil
	}
	if found, present := section[MCPServerKey]; present && reflect.DeepEqual(found, mcpEntry(target.Schema)) &&
		(target.Schema != MCPSchemaOpenCode || opencodeMemoryConfigured(doc.content)) {
		return nil
	}

	var encoded []byte
	if exists {
		_, sectionPresent := doc.content[string(target.Schema)]
		encoded, err = patchMCPFile(doc.raw, target, sectionPresent, doc.content)
	} else {
		encoded, err = newMCPFile(target)
	}
	if err != nil {
		return fmt.Errorf("%s kodieren: %w", target.Path, err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("%s anlegen: %w", filepath.Dir(target.Path), err)
	}
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		return fmt.Errorf("%s schreiben: %w", target.Path, err)
	}
	return nil
}

// newMCPFile baut eine Datei, die es noch nicht gab: nur der eigene Eintrag,
// bei OpenCode zusätzlich der Schema-Verweis. Hier gibt es nichts zu erhalten,
// deshalb genügt gewöhnliches JSON mit zwei Leerzeichen Einrückung.
func newMCPFile(target MCPTarget) ([]byte, error) {
	content := map[string]any{}
	if target.Schema == MCPSchemaOpenCode {
		content[opencodeSchemaKey] = opencodeSchemaURL
		content[opencodeInstructionsKey] = []any{RootInstructionsFile}
		content[opencodeReferencesKey] = map[string]any{opencodeDocsKey: opencodeDocsReference()}
	}
	content[string(target.Schema)] = map[string]any{MCPServerKey: mcpEntry(target.Schema)}

	encoded, err := json.MarshalIndent(content, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}

// jsonPatchOp ist eine einzelne Operation eines JSON Patch nach RFC 6902.
type jsonPatchOp struct {
	Op    string          `json:"op"`
	Path  string          `json:"path"`
	Value json.RawMessage `json:"value"`
}

// patchMCPFile setzt den eigenen Eintrag im Rohtext einer vorhandenen Datei.
//
// Gepatcht wird auf /<schema>/k-playbook. Der Abschnitt selbst muss dafür
// existieren; fehlt er, wird er in derselben Patch-Folge zuerst als leeres
// Objekt angelegt. Beide Pfadbestandteile sind Konstanten ohne "/" und "~",
// deshalb braucht der JSON-Pointer keine Maskierung.
func patchMCPFile(raw []byte, target MCPTarget, sectionPresent bool, content map[string]any) ([]byte, error) {
	value, err := hujson.Parse(raw)
	if err != nil {
		return nil, err
	}

	entry, err := json.Marshal(mcpEntry(target.Schema))
	if err != nil {
		return nil, err
	}

	ops := make([]jsonPatchOp, 0, 2)
	if !sectionPresent {
		ops = append(ops, jsonPatchOp{
			Op:    "add",
			Path:  "/" + string(target.Schema),
			Value: json.RawMessage("{}"),
		})
	}
	ops = append(ops, jsonPatchOp{
		Op:    "add",
		Path:  "/" + string(target.Schema) + "/" + MCPServerKey,
		Value: entry,
	})
	if target.Schema == MCPSchemaOpenCode {
		ops = append(ops, opencodeMemoryPatchOps(content)...)
	}

	patch, err := json.Marshal(ops)
	if err != nil {
		return nil, err
	}
	if err := value.Patch(patch); err != nil {
		return nil, err
	}

	value.Format()
	packed := value.Pack()
	if len(packed) == 0 || packed[len(packed)-1] != '\n' {
		packed = append(packed, '\n')
	}
	return packed, nil
}

func opencodeDocsReference() map[string]any {
	return map[string]any{
		"path":        "./" + LocalDirName + "/docs",
		"description": "Autoritative Projektdokumentation. Zuerst " + LocalDirName + "/docs/README.md als Index lesen, bevor Code analysiert wird.",
	}
}

func opencodeMemoryConfigured(content map[string]any) bool {
	instructions, ok := content[opencodeInstructionsKey].([]any)
	if !ok || !containsAnyString(instructions, RootInstructionsFile) {
		return false
	}
	references, ok := content[opencodeReferencesKey].(map[string]any)
	if !ok {
		return false
	}
	_, ok = references[opencodeDocsKey]
	return ok
}

func opencodeMemoryPatchOps(content map[string]any) []jsonPatchOp {
	ops := []jsonPatchOp{}
	instructions, present := content[opencodeInstructionsKey]
	switch values := instructions.(type) {
	case nil:
		ops = append(ops, jsonPatchOp{Op: "add", Path: "/" + opencodeInstructionsKey, Value: json.RawMessage(`["AGENTS.md"]`)})
	case []any:
		if !containsAnyString(values, RootInstructionsFile) {
			ops = append(ops, jsonPatchOp{Op: "add", Path: "/" + opencodeInstructionsKey + "/-", Value: json.RawMessage(`"AGENTS.md"`)})
		}
	default:
		_ = present
	}

	references, present := content[opencodeReferencesKey]
	encoded, _ := json.Marshal(opencodeDocsReference())
	switch values := references.(type) {
	case nil:
		ops = append(ops, jsonPatchOp{Op: "add", Path: "/" + opencodeReferencesKey, Value: json.RawMessage(`{"docs":` + string(encoded) + `}`)})
	case map[string]any:
		if _, ok := values[opencodeDocsKey]; !ok {
			ops = append(ops, jsonPatchOp{Op: "add", Path: "/" + opencodeReferencesKey + "/" + opencodeDocsKey, Value: encoded})
		}
	default:
		_ = present
	}
	return ops
}

func containsAnyString(values []any, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// jsonDocument ist eine gelesene Konfigurationsdatei: der ausgewertete Inhalt
// zum Prüfen und Vergleichen, dazu der Rohtext, auf dem gepatcht wird.
type jsonDocument struct {
	content map[string]any
	raw     []byte
}

// readJSONObject liest eine Datei als JSON-Objekt und führt den Rohtext mit.
//
// Gelesen wird im JWCC-Format: Kommentare und Trailing Commas sind erlaubt.
// OpenCode wertet jede seiner Config-Dateien so aus, unabhängig von der Endung
// — eine kommentierte opencode.json ist also gültig und kein Grund, die Datei
// für unlesbar zu erklären.
//
// Eine fehlende Datei ist kein Fehler: der Aufrufer bekommt ein leeres Objekt
// und exists false.
func readJSONObject(path string) (doc jsonDocument, exists bool, err error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return jsonDocument{content: map[string]any{}}, false, nil
		}
		return jsonDocument{}, false, fmt.Errorf("nicht lesbar: %w", err)
	}

	value, err := hujson.Parse(raw)
	if err != nil {
		return jsonDocument{}, true, fmt.Errorf("kein gültiges JSON: %w", err)
	}

	// Standardisiert wird auf einer Kopie: der Rohtext bleibt so, wie er auf
	// der Platte steht, und nur er wird später gepatcht.
	standard := value.Clone()
	standard.Standardize()

	var decoded map[string]any
	if err := json.Unmarshal(standard.Pack(), &decoded); err != nil {
		return jsonDocument{}, true, fmt.Errorf("kein gültiges JSON: %w", err)
	}
	if decoded == nil {
		// `null` ist gültiges JSON, aber kein Objekt, in das sich etwas
		// eintragen ließe.
		return jsonDocument{}, true, fmt.Errorf("kein JSON-Objekt")
	}
	return jsonDocument{content: decoded, raw: raw}, true, nil
}
