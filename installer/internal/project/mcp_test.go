package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// newMCPProject legt ein Projekt mit dem Wrapper an, den die Registrierung
// einträgt. Ohne ihn meldet jede Prüfung MCPStateNoWrapper.
func newMCPProject(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	writeFile(t, filepath.Join(root, WrapperPath()), "#!/usr/bin/env bash\n")
	return root
}

func mcpStatusFor(t *testing.T, statuses []MCPStatus, path string) MCPStatus {
	t.Helper()

	for _, status := range statuses {
		if status.Path == path {
			return status
		}
	}
	t.Fatalf("kein Status für %s", path)
	return MCPStatus{}
}

// readJSON liest eine geschriebene Datei zurück. Geprüft wird der Inhalt, nicht
// die Formatierung — die Normalisierung durch den Round-Trip ist gewollt.
func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s lesen: %v", path, err)
	}
	var content map[string]any
	if err := json.Unmarshal(raw, &content); err != nil {
		t.Fatalf("%s ist kein JSON: %v", path, err)
	}
	return content
}

func TestCheckMCPMeldetFehlendeDateien(t *testing.T) {
	root := newMCPProject(t)

	statuses := CheckMCP(root)
	if MCPOK(statuses) {
		t.Fatal("frisches Projekt darf nicht als registriert gelten")
	}
	if len(statuses) != 3 {
		t.Fatalf("%d Ziele geprüft, erwartet 3", len(statuses))
	}
	for _, status := range statuses {
		if status.State != MCPStateMissingFile {
			t.Errorf("%s: State = %q, erwartet %q", status.Path, status.State, MCPStateMissingFile)
		}
	}
}

func TestApplyMCPSchreibtBeideSchemata(t *testing.T) {
	root := newMCPProject(t)

	statuses, err := ApplyMCP(root)
	if err != nil {
		t.Fatalf("ApplyMCP: %v", err)
	}
	if !MCPOK(statuses) {
		t.Fatalf("nach ApplyMCP nicht registriert: %+v", statuses)
	}

	// Claude Code und Cursor: command und args getrennt.
	claude := readJSON(t, filepath.Join(root, ".mcp.json"))
	servers, ok := claude["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf(".mcp.json hat keinen mcpServers-Block: %v", claude)
	}
	entry, ok := servers[MCPServerKey].(map[string]any)
	if !ok {
		t.Fatalf("kein Eintrag %s: %v", MCPServerKey, servers)
	}
	if got := entry["command"]; got != "k-playbook/bin/k-playbook" {
		t.Errorf("command = %v, erwartet den projekteigenen Wrapper", got)
	}
	if args, ok := entry["args"].([]any); !ok || len(args) != 1 || args[0] != "mcp" {
		t.Errorf("args = %v, erwartet [mcp]", entry["args"])
	}

	// OpenCode: command ist ein Array aus Kommando und Argument.
	opencode := readJSON(t, filepath.Join(root, "opencode.json"))
	block, ok := opencode["mcp"].(map[string]any)
	if !ok {
		t.Fatalf("opencode.json hat keinen mcp-Block: %v", opencode)
	}
	entry, ok = block[MCPServerKey].(map[string]any)
	if !ok {
		t.Fatalf("kein Eintrag %s: %v", MCPServerKey, block)
	}
	command, ok := entry["command"].([]any)
	if !ok || len(command) != 2 || command[0] != "k-playbook/bin/k-playbook" || command[1] != "mcp" {
		t.Errorf("command = %v, erwartet ein Array aus Wrapper und mcp", entry["command"])
	}
	if entry["enabled"] != true || entry["type"] != "local" {
		t.Errorf("type/enabled = %v/%v, erwartet local/true", entry["type"], entry["enabled"])
	}

	// Cursor bekommt dasselbe Schema in seinem eigenen Verzeichnis.
	if _, err := os.Stat(filepath.Join(root, ".cursor", "mcp.json")); err != nil {
		t.Errorf(".cursor/mcp.json fehlt: %v", err)
	}
}

func TestApplyMCPSetztSchemaNurBeiNeuanlage(t *testing.T) {
	root := newMCPProject(t)

	if _, err := ApplyMCP(root); err != nil {
		t.Fatalf("ApplyMCP: %v", err)
	}

	// Neu angelegt: OpenCode findet den Schema-Verweis vor und muss die Datei
	// nicht selbst umschreiben.
	opencode := readJSON(t, filepath.Join(root, "opencode.json"))
	if opencode[opencodeSchemaKey] != opencodeSchemaURL {
		t.Errorf("$schema = %v, erwartet %q", opencode[opencodeSchemaKey], opencodeSchemaURL)
	}

	// Die Dateien der anderen Assistenten kennen den Schlüssel nicht.
	claude := readJSON(t, filepath.Join(root, ".mcp.json"))
	if _, ok := claude[opencodeSchemaKey]; ok {
		t.Errorf(".mcp.json hat einen $schema-Eintrag bekommen: %v", claude)
	}

	// Vorgefunden ohne $schema: der Schlüssel bleibt aus. Was dem Projekt
	// gehört, wird nicht um ungefragte Einträge ergänzt.
	bestand := newMCPProject(t)
	writeFile(t, filepath.Join(bestand, "opencode.json"), `{"instructions":["AGENTS.md"]}`+"\n")

	if _, err := ApplyMCP(bestand); err != nil {
		t.Fatalf("ApplyMCP: %v", err)
	}

	vorhanden := readJSON(t, filepath.Join(bestand, "opencode.json"))
	if _, ok := vorhanden[opencodeSchemaKey]; ok {
		t.Errorf("$schema in eine bestehende Datei geschrieben: %v", vorhanden)
	}
	if _, ok := vorhanden["mcp"].(map[string]any)[MCPServerKey]; !ok {
		t.Errorf("eigener Eintrag fehlt: %v", vorhanden)
	}
}

func TestApplyMCPLaesstFremdeEintraegeStehen(t *testing.T) {
	root := newMCPProject(t)
	writeFile(t, filepath.Join(root, ".mcp.json"),
		`{"mcpServers":{"fremd":{"command":"anderes","args":["dienen"]}}}`+"\n")
	writeFile(t, filepath.Join(root, "opencode.json"),
		`{"$schema":"https://opencode.ai/config.json","instructions":["AGENTS.md"]}`+"\n")

	if _, err := ApplyMCP(root); err != nil {
		t.Fatalf("ApplyMCP: %v", err)
	}

	servers := readJSON(t, filepath.Join(root, ".mcp.json"))["mcpServers"].(map[string]any)
	fremd, ok := servers["fremd"].(map[string]any)
	if !ok || fremd["command"] != "anderes" {
		t.Errorf("fremder Server verändert oder verloren: %v", servers["fremd"])
	}
	if _, ok := servers[MCPServerKey]; !ok {
		t.Error("eigener Eintrag fehlt")
	}

	opencode := readJSON(t, filepath.Join(root, "opencode.json"))
	if opencode["$schema"] != "https://opencode.ai/config.json" {
		t.Errorf("$schema verloren: %v", opencode["$schema"])
	}
	instructions, ok := opencode["instructions"].([]any)
	if !ok || len(instructions) != 1 || instructions[0] != "AGENTS.md" {
		t.Errorf("instructions verändert: %v", opencode["instructions"])
	}
}

func TestApplyMCPKorrigiertFremdenWert(t *testing.T) {
	root := newMCPProject(t)
	// Ein absoluter Pfad auf die host-weite Kopie: derselbe Schlüssel, falscher
	// Stand.
	writeFile(t, filepath.Join(root, ".mcp.json"),
		`{"mcpServers":{"`+MCPServerKey+`":{"command":"/home/wer/.local/bin/k-playbook","args":["mcp"]}}}`+"\n")

	status := mcpStatusFor(t, CheckMCP(root), ".mcp.json")
	if status.State != MCPStateStale {
		t.Fatalf("State = %q, erwartet %q", status.State, MCPStateStale)
	}

	statuses, err := ApplyMCP(root)
	if err != nil {
		t.Fatalf("ApplyMCP: %v", err)
	}
	if status := mcpStatusFor(t, statuses, ".mcp.json"); !status.OK() {
		t.Errorf("nach ApplyMCP nicht korrigiert: %+v", status)
	}
}

func TestApplyMCPLaesstKaputtesJSONLiegen(t *testing.T) {
	root := newMCPProject(t)
	broken := "{ das ist kein JSON\n"
	writeFile(t, filepath.Join(root, ".mcp.json"), broken)

	status := mcpStatusFor(t, CheckMCP(root), ".mcp.json")
	if status.State != MCPStateUnreadable {
		t.Fatalf("State = %q, erwartet %q", status.State, MCPStateUnreadable)
	}

	statuses, err := ApplyMCP(root)
	if err != nil {
		t.Fatalf("ApplyMCP: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(root, ".mcp.json"))
	if err != nil || string(content) != broken {
		t.Errorf("kaputte Datei wurde angefasst: %q, %v", content, err)
	}
	// Die beiden anderen Ziele stehen trotzdem.
	if status := mcpStatusFor(t, statuses, "opencode.json"); !status.OK() {
		t.Errorf("opencode.json nicht registriert: %+v", status)
	}
}

// Ein Ziel, das sich nicht schreiben lässt, darf die anderen nicht aufhalten:
// die drei Dateien gehören verschiedenen Assistenten und hängen nicht
// voneinander ab.
func TestApplyMCPSchreibtTrotzGescheitertemZiel(t *testing.T) {
	root := newMCPProject(t)

	// .cursor/ existiert, ist aber nicht beschreibbar. Damit scheitert genau das
	// mittlere Ziel, und zwar erst beim Schreiben — nicht schon beim Lesen.
	cursor := filepath.Join(root, ".cursor")
	if err := os.MkdirAll(cursor, 0o555); err != nil {
		t.Fatalf(".cursor anlegen: %v", err)
	}
	t.Cleanup(func() { os.Chmod(cursor, 0o755) })
	if err := os.WriteFile(filepath.Join(cursor, "probe"), []byte("x"), 0o644); err == nil {
		t.Skip("Schreibrechte lassen sich hier nicht entziehen (root?)")
	}

	statuses, err := ApplyMCP(root)
	if err == nil {
		t.Error("gescheitertes Ziel wurde nicht gemeldet")
	}

	// Das mittlere Ziel fehlt, die beiden anderen stehen trotzdem.
	if status := mcpStatusFor(t, statuses, filepath.Join(".cursor", "mcp.json")); status.OK() {
		t.Errorf(".cursor/mcp.json gilt als registriert: %+v", status)
	}
	for _, path := range []string{".mcp.json", "opencode.json"} {
		if status := mcpStatusFor(t, statuses, path); !status.OK() {
			t.Errorf("%s nicht registriert, obwohl unabhängig: %+v", path, status)
		}
	}
}

func TestMCPOhneWrapperWirdNichtsGeschrieben(t *testing.T) {
	root := t.TempDir()

	statuses := CheckMCP(root)
	for _, status := range statuses {
		if status.State != MCPStateNoWrapper {
			t.Errorf("%s: State = %q, erwartet %q", status.Path, status.State, MCPStateNoWrapper)
		}
	}

	if _, err := ApplyMCP(root); err != nil {
		t.Fatalf("ApplyMCP: %v", err)
	}
	for _, target := range MCPTargets() {
		if _, err := os.Stat(filepath.Join(root, target.Path)); !os.IsNotExist(err) {
			t.Errorf("%s wurde ohne Wrapper angelegt", target.Path)
		}
	}
}

func TestApplyMCPIstIdempotent(t *testing.T) {
	root := newMCPProject(t)

	if _, err := ApplyMCP(root); err != nil {
		t.Fatalf("erster Lauf: %v", err)
	}
	first, err := os.ReadFile(filepath.Join(root, "opencode.json"))
	if err != nil {
		t.Fatalf("opencode.json lesen: %v", err)
	}

	statuses, err := ApplyMCP(root)
	if err != nil {
		t.Fatalf("zweiter Lauf: %v", err)
	}
	if !MCPOK(statuses) {
		t.Errorf("zweiter Lauf nicht registriert: %+v", statuses)
	}

	second, err := os.ReadFile(filepath.Join(root, "opencode.json"))
	if err != nil {
		t.Fatalf("opencode.json lesen: %v", err)
	}
	if string(first) != string(second) {
		t.Errorf("zweiter Lauf hat die Datei verändert:\n%s\n%s", first, second)
	}
}
