package project

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

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
	// traceDir ist das Verzeichnis des Assistenten im Hauptverzeichnis, an dem
	// der Startlauf seine Spur sucht, bevor er eine fehlende Datei anlegt
	// (assistantTrace). Nur intern: für die Anzeige spielt es keine Rolle.
	traceDir string
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
		{Path: ".mcp.json", Assistant: "Claude Code", Schema: MCPSchemaServers, traceDir: ".claude"},
		{Path: filepath.Join(".cursor", "mcp.json"), Assistant: "Cursor", Schema: MCPSchemaServers, traceDir: ".cursor"},
		{Path: opencodeTarget(projectRoot), Assistant: "OpenCode", Schema: MCPSchemaOpenCode, traceDir: ".opencode"},
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
	// MCPStateNoCommand: auf diesem Rechner ließ sich kein installiertes
	// k-playbook auflösen. Dann wird nichts geschrieben — sonst stünde eine
	// Registrierung da, die auf nichts zeigt.
	MCPStateNoCommand MCPState = "no-command"
	// MCPStateMissingFile: die Datei gibt es noch nicht. Der Start legt sie
	// von sich aus an, wenn sie nicht von git erfasst ist und der Assistent
	// eine eigene Spur im Projekt hat (MCPWriteOutdatedAndUnversioned).
	MCPStateMissingFile MCPState = "missing-file"
	// MCPStateMissingEntry: die Datei steht, unser Eintrag fehlt darin. Der
	// Start ergänzt ihn von sich aus, wenn die Datei nicht von git erfasst ist.
	MCPStateMissingEntry MCPState = "missing-entry"
	// MCPStateOutdated: der Eintrag trägt das Kommando des abgelösten
	// Wrapper-Modells. Das ist der eine Fall, den die Auto-Korrektur beim
	// Clone-Update und beim Start unabhängig von der Versionierung
	// richtigstellt — eine Reparatur an Inhalt, den k-playbook selbst
	// geschrieben hat.
	MCPStateOutdated MCPState = "outdated"
	// MCPStateStale: der Eintrag steht, gehört aber zu keiner akzeptierten Form
	// und ist auch nicht der alte Wrapper. Er wird gemeldet und erst beim
	// ausdrücklichen Einrichten überschrieben.
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

// MCPSubcommand ist das Argument, mit dem das installierte k-playbook seinen
// MCP-Server über stdin und stdout spricht.
const MCPSubcommand = "mcp"

// MCPCommand ist das Kommando, das registriert wird: der beim Schreiben
// aufgelöste absolute Pfad des installierten k-playbook, gefolgt vom
// Subkommando.
//
// Absolut und nicht als bloßer Name: aus Dock oder Finder gestartete Clients
// erben die Shell-PATH nicht, dort fehlt ~/.local/bin typischerweise. Ein
// Kommandoname wäre in genau diesen Umgebungen tot.
//
// Der Schrägstrich ist hier kein Dateisystempfad, sondern Teil eines Wertes in
// einer Konfigurationsdatei — deshalb ToSlash und nicht der Trenner der
// Plattform.
//
// Ohne installiertes k-playbook gibt es kein Kommando; der Fehler ist das
// Signal, nichts zu schreiben.
func MCPCommand() (string, []string, error) {
	installed, err := InstalledCommandPath()
	if err != nil {
		return "", nil, err
	}
	return filepath.ToSlash(installed), []string{MCPSubcommand}, nil
}

// legacyWrapperCommand ist das Kommando des abgelösten Wrapper-Modells: der
// projekteigene Wrapper, relativ zum Hauptverzeichnis eingetragen.
//
// Bewusst als eigene Konstante und nicht aus einem Wrapper-Bezeichner
// abgeleitet: die Erkennung muss den Wrapper überleben. Die Datei
// `bin/k-playbook` gibt es im Quell-Repo nicht mehr; genau deshalb ist diese
// Konstante das Einzige, was Bestandsprojekte noch von ihr weg migriert. Sie
// wird nicht mit dem Wrapper zusammen entfernt.
const legacyWrapperCommand = "k-playbook/bin/k-playbook"

// legacyWrapperTail ist der Rest desselben Pfades ohne das Installationsver-
// zeichnis. Ein Zielprojekt konnte ihn auch so eintragen.
const legacyWrapperTail = "bin/k-playbook"

// isOutdatedMCPCommand ist die enge Definition von „veraltet": der alte
// Wrapper-Pfad und sonst nichts.
//
// Die Enge ist der Punkt. Nur dieser Fall wird von selbst überschrieben; alles
// andere bleibt liegen, bis jemand ausdrücklich einrichtet. Deshalb sind die
// beiden Fälle getrennt:
//
//   - Ein **relativer** Eintrag konnte immer nur den projektlokalen Wrapper
//     meinen; es genügt, dass er auf bin/k-playbook endet.
//   - Ein **absoluter** Eintrag muss auf k-playbook/bin/k-playbook enden, also
//     auf den Wrapper unterhalb einer Installation. Sonst fiele das neue
//     ~/.local/bin/k-playbook selbst darunter — es endet ebenfalls auf
//     bin/k-playbook — und die Auto-Korrektur schriebe endlos.
func isOutdatedMCPCommand(command string) bool {
	cleaned := path.Clean(filepath.ToSlash(strings.TrimSpace(command)))
	if cleaned == "" || cleaned == "." || cleaned == "/" {
		return false
	}

	if path.IsAbs(cleaned) {
		return strings.HasSuffix(cleaned, "/"+legacyWrapperCommand)
	}
	return cleaned == legacyWrapperTail || strings.HasSuffix(cleaned, "/"+legacyWrapperTail)
}

// mcpCommandForm ist eine Schreibweise des registrierten Kommandos, die als
// aktuell gilt.
type mcpCommandForm struct {
	// name benennt die Form für Menschen — Doku und Fehlersuche, nicht Logik.
	name string
	// matches prüft ein vorgefundenes Kommando.
	matches func(command string) bool
}

// acceptedMCPCommandForms ist die Menge der Eintragsformen, die als aktuell
// gelten.
//
// Geschrieben wird immer nur **eine** davon — der aufgelöste absolute Pfad aus
// MCPCommand(). Geprüft wird gegen die ganze Menge, und das ist keine
// Bequemlichkeit: ein Vergleich auf Gleichheit mit einem einzigen Sollwert
// erklärte jede andere gültige Schreibweise für falsch und schriebe sie bei
// jedem Lauf um. Zwei Fälle machen das konkret:
//
//   - Host und DevContainer haben verschiedene HOMEs. Ein absoluter Pfad aus
//     dem jeweils anderen HOME gilt deshalb ausdrücklich als aktuell; sonst
//     erklärten sich beide Umgebungen in derselben Datei wechselseitig für
//     veraltet und spielten bei jedem Wechsel Ping-Pong. Der Preis steht in
//     docs/mcp.md: bei geteiltem Repo und getrennten HOMEs bleibt MCP in der
//     anderen Umgebung tot, ohne dass die Auto-Korrektur greift — nur der
//     Selbsttest schlägt an.
//   - Eine eingecheckte Registrierung kann keinen absoluten Home-Pfad tragen.
//     Ihre portable Form ist der bloße Kommandoname; sie gehört deshalb
//     ebenfalls in diese Menge.
//
// **Erweiterungspunkt:** eine weitere Form ist ein weiterer Eintrag in dieser
// Liste. Der Vergleich selbst — isAcceptedMCPCommand, checkMCPTarget,
// mcpTargetNeedsWrite — bleibt dabei unverändert.
func acceptedMCPCommandForms() []mcpCommandForm {
	return []mcpCommandForm{
		{name: "aufgelöster absoluter Pfad", matches: isInstalledCommandForm},
		{name: "portabler Kommandoname", matches: isPortableCommandForm},
	}
}

// isPortableCommandForm meldet, ob das Kommando der bloße Name des
// installierten k-playbook ist.
//
// Das ist die **eincheckbare** Form. Sie steht in den drei MCP-Dateien dieses
// Quell-Repos und in jedem Projekt, das seine Registrierung teilen will: ein
// absoluter Pfad trägt ein `$HOME` und ist damit an eine Umgebung gebunden,
// der Name ist es nicht. Host und DevContainer lösen ihn über ihre je eigene
// PATH auf ihr je eigenes Binary auf — genau der DevContainer-Vertrag aus
// docs/mcp.md, ohne dass eine der beiden Umgebungen die Datei umschreibt.
//
// Geschrieben wird sie nie: MCPCommand() liefert weiter den aufgelösten
// absoluten Pfad. Ihre Grenze steht in docs/mcp.md — ein aus Dock oder Finder
// gestarteter Client erbt die Shell-PATH nicht und findet unter diesem Namen
// nichts. Wer so arbeitet, richtet in seiner Umgebung einmal ausdrücklich ein
// und bekommt dabei den absoluten Pfad.
//
// Bewusst ohne path.Clean: akzeptiert ist genau der Name, nicht `./k-playbook`
// und nicht irgendein Pfad, der darauf endet. Alles andere bleibt „falscher
// Stand" — gemeldet, nie von selbst überschrieben.
func isPortableCommandForm(command string) bool {
	return strings.TrimSpace(command) == InstalledCommandName
}

// isInstalledCommandForm meldet, ob das Kommando ein absoluter Pfad auf ein
// installiertes k-playbook ist — gleich in welchem HOME.
//
// Geprüft wird der Name, nicht die Existenz: die Datei eines fremden HOME liegt
// hier nicht, und ein Eintrag deswegen für falsch zu erklären wäre genau das
// Ping-Pong, das vermieden werden soll.
func isInstalledCommandForm(command string) bool {
	cleaned := path.Clean(filepath.ToSlash(strings.TrimSpace(command)))
	if !path.IsAbs(cleaned) {
		return false
	}
	return path.Base(cleaned) == InstalledCommandName
}

// isAcceptedMCPCommand meldet, ob ein vorgefundenes Kommando zur Menge der
// akzeptierten Formen gehört. Der veraltete Wrapper gehört nie dazu, auch wenn
// er als absoluter Pfad geschrieben ist.
func isAcceptedMCPCommand(command string) bool {
	if isOutdatedMCPCommand(command) {
		return false
	}
	for _, form := range acceptedMCPCommandForms() {
		if form.matches(command) {
			return true
		}
	}
	return false
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
// danach. Das ist der **ausdrückliche** Schreibweg — der Klick auf
// „Einrichten".
//
// Geschrieben wird jedes Ziel, dessen Eintrag nicht zur Menge der akzeptierten
// Formen gehört. Ohne installiertes k-playbook wird nichts geschrieben: eine
// Registrierung, die auf nichts zeigt, ist schlechter als keine. Eine Datei,
// die kein JSON-Objekt ist, bleibt ebenfalls unberührt; die Prüfung meldet sie.
//
// Die drei Ziele sind voneinander unabhängig, deshalb hält ein gescheitertes die
// anderen nicht auf: ein nicht schreibbares .cursor/ ist kein Grund, OpenCode
// unregistriert zu lassen. Gesammelt wird, was schiefging; welches Ziel steht
// und welches nicht, sagt ohnehin der zurückgegebene Zustand.
func ApplyMCP(projectRoot string) ([]MCPStatus, error) {
	_, err := writeMCPEntries(projectRoot, MCPWriteAll)
	return CheckMCP(projectRoot), err
}

// MCPWriteScope benennt, wie weit ein Schreibweg greifen darf. Drei Werte,
// weil es drei Einstiege gibt, die verschieden weit gehen dürfen — und weil
// die Entscheidung trotzdem an einer Stelle bleiben soll: mcpTargetNeedsWrite.
type MCPWriteScope int

const (
	// MCPWriteAll: alles, was nicht zur Menge der akzeptierten Formen gehört
	// — der Klick auf „Einrichten" (ApplyMCP).
	MCPWriteAll MCPWriteScope = iota
	// MCPWriteOutdatedOnly: ausschließlich MCPStateOutdated — das
	// Clone-Update. Es soll den Eintrag auf den abgelösten Wrapper
	// nachziehen und sonst nichts.
	MCPWriteOutdatedOnly
	// MCPWriteOutdatedAndUnversioned: MCPStateOutdated, dazu eine fehlende
	// Datei und ein fehlender Eintrag, wenn die Zieldatei nicht von git
	// erfasst ist — der Start. Die fehlende Datei zusätzlich nur, wenn der
	// Assistent eine eigene Spur im Projekt hat.
	MCPWriteOutdatedAndUnversioned
)

// RepairMCP ist der **selbsttätige** Weg: er läuft beim Clone-Update und bei
// jedem Start, ohne dass jemand einen Knopf drückt. Wie weit er gehen darf,
// sagt der Modus — das Update übergibt MCPWriteOutdatedOnly, der Start
// MCPWriteOutdatedAndUnversioned.
//
// Zurück kommen die Pfade der geschriebenen Dateien, relativ zum
// Hauptverzeichnis — leer, wenn nichts zu tun war.
//
// Die Enge ist hier die eigentliche Zusage, und sie hat zwei Stufen. Ein
// vorhandener Eintrag wird ausschließlich bei MCPStateOutdated ersetzt — eine
// Reparatur an Inhalt, den k-playbook selbst geschrieben hat, unabhängig von
// der Versionierung. Eine akzeptierte Form und ein fremder Stand
// (MCPStateStale) bleiben liegen: der Eintrag kann aus einem fremden $HOME
// stammen und dort gültig sein. Angelegt oder ergänzt wird nur, wenn die
// Zieldatei **nicht von git erfasst** ist, und das wird gemessen, nicht
// geraten (mcpTargetTracked). Sonst machte jeder Start die getrackten
// MCP-Dateien eines Projekts dreckig, und ein Repo, das seine Registrierung in
// portabler Form eincheckt, käme nie mehr an einem sauberen Arbeitsbaum
// vorbei. Gelöscht wird nie.
func RepairMCP(projectRoot string, scope MCPWriteScope) ([]string, error) {
	return writeMCPEntries(projectRoot, scope)
}

// writeMCPEntries ist der gemeinsame Schreibweg aller Einstiege; der Modus
// unterscheidet sie.
func writeMCPEntries(projectRoot string, scope MCPWriteScope) ([]string, error) {
	command, args, err := MCPCommand()
	if err != nil {
		// Kein installiertes k-playbook: es gibt kein Kommando, das sich
		// eintragen ließe. CheckMCP meldet den Zustand.
		return nil, nil
	}

	var written []string
	var failures []error
	for _, target := range MCPTargets(projectRoot) {
		changed, err := applyMCPTarget(projectRoot, target, command, args, scope)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		if changed {
			written = append(written, target.Path)
		}
	}
	return written, errors.Join(failures...)
}

// mcpEntry ist der Eintrag, der unter MCPServerKey steht — je Schema anders
// geformt, aber immer dasselbe Kommando mit demselben Subkommando.
//
// Die Werte sind so getypt, wie encoding/json sie beim Lesen zurückgibt.
func mcpEntry(schema MCPSchema, command string, args []string) map[string]any {
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

// mcpEntryCommand liest Kommando und Argumente aus einem vorgefundenen Eintrag,
// streng nach dem Schema. Passt die Form nicht, ist das dritte Ergebnis false.
//
// Geprüft wird nur, was den Eintrag ausmacht. Zusätzliche Schlüssel — etwa ein
// von Hand gesetztes timeout bei OpenCode — bleiben davon unberührt und machen
// den Eintrag nicht falsch. Ein Vergleich auf strukturelle Gleichheit hätte sie
// bei jedem Lauf weggeschrieben.
func mcpEntryCommand(schema MCPSchema, value any) (string, []string, bool) {
	entry, ok := value.(map[string]any)
	if !ok {
		return "", nil, false
	}

	if schema == MCPSchemaOpenCode {
		if entry["type"] != "local" || entry["enabled"] != true {
			return "", nil, false
		}
		line, ok := entry["command"].([]any)
		if !ok || len(line) == 0 {
			return "", nil, false
		}
		command, ok := line[0].(string)
		if !ok {
			return "", nil, false
		}
		args, ok := stringsFromAny(line[1:])
		if !ok {
			return "", nil, false
		}
		return command, args, true
	}

	command, ok := entry["command"].(string)
	if !ok {
		return "", nil, false
	}
	raw, ok := entry["args"].([]any)
	if !ok {
		return "", nil, false
	}
	args, ok := stringsFromAny(raw)
	if !ok {
		return "", nil, false
	}
	return command, args, true
}

// looseMCPEntryCommand liest nur das Kommando heraus, ohne die übrige Form zu
// bewerten.
//
// Für die Migration ist das richtig: ein Eintrag, der den alten Wrapper nennt,
// ist veraltet — auch wenn daneben etwas steht, das nicht ins Schema passt.
func looseMCPEntryCommand(value any) (string, bool) {
	entry, ok := value.(map[string]any)
	if !ok {
		return "", false
	}

	switch command := entry["command"].(type) {
	case string:
		return command, true
	case []any:
		if len(command) == 0 {
			return "", false
		}
		first, ok := command[0].(string)
		return first, ok
	}
	return "", false
}

// mcpEntryUpToDate meldet, ob ein vorgefundener Eintrag als aktuell gilt:
// richtige Form, richtiges Subkommando, Kommando aus der Menge der akzeptierten
// Formen.
func mcpEntryUpToDate(schema MCPSchema, value any) bool {
	command, args, ok := mcpEntryCommand(schema, value)
	if !ok || len(args) != 1 || args[0] != MCPSubcommand {
		return false
	}
	return isAcceptedMCPCommand(command)
}

// mcpEntryOutdated meldet, ob ein vorgefundener Eintrag im engen Sinn veraltet
// ist — also auf den abgelösten Wrapper zeigt.
func mcpEntryOutdated(value any) bool {
	command, ok := looseMCPEntryCommand(value)
	return ok && isOutdatedMCPCommand(command)
}

func stringsFromAny(values []any) ([]string, bool) {
	result := make([]string, 0, len(values))
	for _, value := range values {
		text, ok := value.(string)
		if !ok {
			return nil, false
		}
		result = append(result, text)
	}
	return result, true
}

func checkMCPTarget(projectRoot string, target MCPTarget) MCPStatus {
	status := MCPStatus{MCPTarget: target}

	if _, err := InstalledCommandPath(); err != nil {
		status.State = MCPStateNoCommand
		status.Detail = ErrNoInstalledCommand.Error()
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

	case mcpEntryUpToDate(target.Schema, found):
		// Der Eintrag steht — aber bei zwei OpenCode-Configs sagt das nichts
		// darüber, was am Ende wirkt.
		if target.Schema == MCPSchemaOpenCode && opencodeAmbiguous(projectRoot) {
			status.State = MCPStateAmbiguousTarget
			status.Detail = opencodeConfigJSON + " und " + opencodeConfigJSONC + " liegen nebeneinander"
			return status
		}
		// Gezeigt wird, was dasteht, nicht was geschrieben würde: als aktuell
		// gilt eine Menge von Formen, nicht ein einziger Wert.
		status.State = MCPStateOK
		status.Detail = "-> " + describeMCPEntry(found)

	case mcpEntryOutdated(found):
		status.State = MCPStateOutdated
		status.Detail = "zeigt auf den abgelösten Wrapper " + describeMCPEntry(found)

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
// Steht der Eintrag bereits in einer akzeptierten Form, wird gar nicht
// geschrieben. Sonst würde jeder Lauf eine fremde Datei erneut umformatieren.
//
// Zurück kommt, ob tatsächlich geschrieben wurde.
func applyMCPTarget(projectRoot string, target MCPTarget, command string, args []string, scope MCPWriteScope) (bool, error) {
	file := filepath.Join(projectRoot, target.Path)

	doc, exists, err := readJSONObject(file)
	if err != nil {
		// Nicht lesbar heißt: nicht anfassen. Die Prüfung meldet es.
		return false, nil
	}

	section, ok := mcpSection(doc.content, target.Schema)
	if !ok {
		return false, nil
	}
	found, present := section[MCPServerKey]
	if !mcpTargetNeedsWrite(projectRoot, target, doc.content, exists, found, present, scope) {
		return false, nil
	}

	var encoded []byte
	if exists {
		_, sectionPresent := doc.content[string(target.Schema)]
		encoded, err = patchMCPFile(doc.raw, target, sectionPresent, doc.content, command, args)
	} else {
		encoded, err = newMCPFile(target, command, args)
	}
	if err != nil {
		return false, fmt.Errorf("%s kodieren: %w", target.Path, err)
	}

	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return false, fmt.Errorf("%s anlegen: %w", filepath.Dir(target.Path), err)
	}
	if err := os.WriteFile(file, encoded, 0o644); err != nil {
		return false, fmt.Errorf("%s schreiben: %w", target.Path, err)
	}
	return true, nil
}

// mcpTargetNeedsWrite ist die eine Stelle, an der alle Schreibwege ihre
// Entscheidung treffen.
//
// Clone-Update (MCPWriteOutdatedOnly): ausschließlich ein vorhandener, im
// engen Sinn veralteter Eintrag. Eine fehlende Datei, ein fehlender Eintrag
// und jede akzeptierte Form bleiben unangetastet.
//
// Start (MCPWriteOutdatedAndUnversioned): dasselbe, dazu der fehlende Eintrag
// und die fehlende Datei — aber nur, wenn die Zieldatei nicht von git erfasst
// ist, und die fehlende Datei nur, wenn der Assistent eine eigene Spur im
// Projekt hat. Ein vorhandener Eintrag, der weder veraltet noch akzeptiert
// ist (MCPStateStale), bleibt auch hier liegen. Gemessen wird erst, wenn es
// darauf ankommt: die beiden git-Aufrufe laufen nicht für Dateien, die ohnehin
// nicht geschrieben würden.
//
// Einrichten (MCPWriteAll): alles, was nicht zur Menge der akzeptierten Formen
// gehört, dazu bei OpenCode der Memory-Block, der zum Einrichten dazugehört.
func mcpTargetNeedsWrite(projectRoot string, target MCPTarget, content map[string]any, exists bool, found any, present bool, scope MCPWriteScope) bool {
	switch scope {
	case MCPWriteOutdatedOnly:
		return present && mcpEntryOutdated(found)

	case MCPWriteOutdatedAndUnversioned:
		if present {
			return mcpEntryOutdated(found)
		}
		if !exists && !assistantTrace(projectRoot, target) {
			return false
		}
		return !mcpTargetTracked(projectRoot, target.Path)
	}

	if !present || !mcpEntryUpToDate(target.Schema, found) {
		return true
	}
	return target.Schema == MCPSchemaOpenCode && !opencodeMemoryConfigured(content)
}

// mcpTargetTracked meldet, ob git die Zieldatei erfasst. Gemessen wird im
// Hauptverzeichnis, denn dort liegen die Dateien — nicht in project.repo_root,
// das daneben liegen kann.
//
// Die Kette hat drei Stufen, und jede unbeantwortete Frage fällt in dieselbe
// Richtung wie bei agentsIgnored: als erfasst, also nicht schreiben.
//
//   - K-PLAYBOOK.yaml nicht lesbar: unbeantwortet, gilt als erfasst. „Keine
//     Config = kein git = schreiben" wäre die unsichere Richtung.
//   - project.vcs nicht git: nichts ist erfasst.
//   - rev-parse --show-toplevel sagt nein: an dieser Stelle gibt es kein
//     Repository, nichts ist erfasst. Nur ein git, das gelaufen ist und
//     geantwortet hat, zählt dafür; kam es gar nicht zu Wort — nicht
//     installiert, Timeout —, ist die Frage unbeantwortet.
//   - ls-files --error-unmatch: Exit 0 heißt erfasst, Exit 1 nicht erfasst,
//     alles andere unbeantwortet. ls-files deckt auch die gelöschte, aber noch
//     im Index stehende Datei ab; ein reiner Existenztest täte das nicht.
func mcpTargetTracked(projectRoot string, path string) bool {
	config, err := ReadConfig(projectRoot)
	if err != nil {
		return true
	}
	if config.VCS != "git" {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), privateGitTimeout)
	defer cancel()

	if _, code, _ := runGit(ctx, projectRoot, "rev-parse", "--show-toplevel"); code != 0 {
		return code < 0
	}
	_, code, _ := runGit(ctx, projectRoot, "ls-files", "--error-unmatch", "--", path)
	switch code {
	case 0:
		return true
	case 1:
		return false
	default:
		return true
	}
}

// assistantTrace meldet, ob der Assistent eines Ziels eine Spur im Projekt
// hat, die nicht von k-playbook stammt: das Assistenten-Verzeichnis trägt
// mindestens einen Eintrag, der keiner der von Links() verwalteten Pfade ist
// — etwa .claude/settings.json, .cursor/rules/ oder eine eigene Datei unter
// .opencode/.
//
// Die Verzeichnisse selbst taugen nicht als Spur: ApplyLinks legt sie in jedem
// Projekt an, schon auf dem Lesepfad. Ohne diese Bedingung entstünden bei
// jedem Start drei Dateien, die niemand braucht. Ein fehlendes oder
// unlesbares Verzeichnis ist keine Spur.
func assistantTrace(projectRoot string, target MCPTarget) bool {
	if target.traceDir == "" {
		return false
	}
	entries, err := os.ReadDir(filepath.Join(projectRoot, target.traceDir))
	if err != nil {
		return false
	}

	managed := map[string]bool{}
	for _, link := range Links() {
		managed[link.Path] = true
	}
	for _, entry := range entries {
		if !managed[filepath.Join(target.traceDir, entry.Name())] {
			return true
		}
	}
	return false
}

// newMCPFile baut eine Datei, die es noch nicht gab: nur der eigene Eintrag,
// bei OpenCode zusätzlich der Schema-Verweis. Hier gibt es nichts zu erhalten,
// deshalb genügt gewöhnliches JSON mit zwei Leerzeichen Einrückung.
func newMCPFile(target MCPTarget, command string, args []string) ([]byte, error) {
	content := map[string]any{}
	if target.Schema == MCPSchemaOpenCode {
		content[opencodeSchemaKey] = opencodeSchemaURL
		content[opencodeInstructionsKey] = []any{RootInstructionsFile}
		content[opencodeReferencesKey] = map[string]any{opencodeDocsKey: opencodeDocsReference()}
	}
	content[string(target.Schema)] = map[string]any{MCPServerKey: mcpEntry(target.Schema, command, args)}

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
func patchMCPFile(raw []byte, target MCPTarget, sectionPresent bool, content map[string]any, command string, args []string) ([]byte, error) {
	value, err := hujson.Parse(raw)
	if err != nil {
		return nil, err
	}

	entry, err := json.Marshal(mcpEntry(target.Schema, command, args))
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

// opencodeMemoryConfigured meldet, ob am Memory-Block nichts mehr zu tun ist.
//
// Gefragt wird die Stelle, die auch patcht: was opencodeMemoryPatchOps stehen
// lässt, gilt hier als eingerichtet. Beides auseinanderlaufen zu lassen kostete
// genau das, was die Entscheidung verhindern soll — ein fremd getypter Wert
// (instructions als Zeichenkette, references als Liste) galt als nicht
// eingerichtet, die Patch-Folge hatte aber nichts zu ändern. Jedes Einrichten
// schrieb und formatierte opencode.json dann neu, ohne dass sich inhaltlich
// etwas bewegte.
func opencodeMemoryConfigured(content map[string]any) bool {
	return len(opencodeMemoryPatchOps(content)) == 0
}

func opencodeMemoryPatchOps(content map[string]any) []jsonPatchOp {
	ops := []jsonPatchOp{}
	switch values := content[opencodeInstructionsKey].(type) {
	case nil:
		ops = append(ops, jsonPatchOp{Op: "add", Path: "/" + opencodeInstructionsKey, Value: json.RawMessage(`["AGENTS.md"]`)})
	case []any:
		if !containsAnyString(values, RootInstructionsFile) {
			ops = append(ops, jsonPatchOp{Op: "add", Path: "/" + opencodeInstructionsKey + "/-", Value: json.RawMessage(`"AGENTS.md"`)})
		}
	default:
		// Ein anderer Typ gehört dem Projekt. Hineingeschrieben wird dort
		// nicht — und deshalb gilt der Block als eingerichtet.
	}

	encoded, _ := json.Marshal(opencodeDocsReference())
	switch values := content[opencodeReferencesKey].(type) {
	case nil:
		ops = append(ops, jsonPatchOp{Op: "add", Path: "/" + opencodeReferencesKey, Value: json.RawMessage(`{"docs":` + string(encoded) + `}`)})
	case map[string]any:
		if _, ok := values[opencodeDocsKey]; !ok {
			ops = append(ops, jsonPatchOp{Op: "add", Path: "/" + opencodeReferencesKey + "/" + opencodeDocsKey, Value: encoded})
		}
	default:
		// Wie oben: fremder Typ, nichts zu tun.
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
