package project

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func serverFor(t *testing.T, servers []MCPServerEntry, assistantID string, name string) MCPServerEntry {
	t.Helper()
	for _, server := range servers {
		if server.AssistantID == assistantID && server.Name == name {
			return server
		}
	}
	t.Fatalf("kein Server %s bei %s in %+v", name, assistantID, servers)
	return MCPServerEntry{}
}

func fileFor(t *testing.T, files []MCPFileInfo, path string) MCPFileInfo {
	t.Helper()
	for _, file := range files {
		if file.Path == path {
			return file
		}
	}
	t.Fatalf("keine Datei %s in %+v", path, files)
	return MCPFileInfo{}
}

// Alle drei Schemata werden gelesen — Claude Code und Cursor mit command+args,
// OpenCode mit command-Array —, Kommentare und Trailing Commas eingeschlossen.
func TestListMCPServersLiestDreiSchemata(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".mcp.json"), `{
  // Kommentar ist erlaubt
  "mcpServers": {
    "k-playbook": {"command": "k-playbook", "args": ["mcp"],},
    "zeta": {"command": "npx", "args": ["-y", "zeta-mcp"]}
  },
}`)
	writeFile(t, filepath.Join(root, ".cursor", "mcp.json"), `{"mcpServers": {"k-playbook": {"command": "/home/wer/.local/bin/k-playbook", "args": ["mcp"]}}}`)
	writeFile(t, filepath.Join(root, "opencode.json"), `{"mcp": {"k-playbook": {"type": "local", "command": ["k-playbook", "mcp"], "enabled": true}}}`)

	servers, files := ListMCPServers(root)
	if len(servers) != 4 {
		t.Fatalf("%d Server, erwartet 4: %+v", len(servers), servers)
	}

	claude := serverFor(t, servers, "claude-code", "k-playbook")
	if claude.Transport != MCPTransportLocal || claude.Command != "k-playbook" || strings.Join(claude.Args, " ") != "mcp" {
		t.Errorf("Claude-Eintrag = %+v", claude)
	}
	if !claude.Own || !claude.Enabled || claude.File != ".mcp.json" || claude.Schema != MCPSchemaServers {
		t.Errorf("Claude-Eintrag = %+v", claude)
	}
	zeta := serverFor(t, servers, "claude-code", "zeta")
	if zeta.Own || zeta.Command != "npx" || strings.Join(zeta.Args, " ") != "-y zeta-mcp" {
		t.Errorf("zeta = %+v", zeta)
	}
	cursor := serverFor(t, servers, "cursor", "k-playbook")
	if cursor.Assistant != "Cursor" || cursor.File != filepath.Join(".cursor", "mcp.json") || cursor.Command != "/home/wer/.local/bin/k-playbook" {
		t.Errorf("Cursor-Eintrag = %+v", cursor)
	}
	opencode := serverFor(t, servers, "opencode", "k-playbook")
	if opencode.Transport != MCPTransportLocal || opencode.Command != "k-playbook" || strings.Join(opencode.Args, " ") != "mcp" || !opencode.Enabled {
		t.Errorf("OpenCode-Eintrag = %+v", opencode)
	}
	if opencode.Schema != MCPSchemaOpenCode || opencode.File != "opencode.json" {
		t.Errorf("OpenCode-Eintrag = %+v", opencode)
	}

	if len(files) != 3 {
		t.Fatalf("%d Dateien, erwartet 3: %+v", len(files), files)
	}
	if info := fileFor(t, files, ".mcp.json"); !info.Exists || info.Count != 2 || info.Error != "" || info.AssistantID != "claude-code" {
		t.Errorf(".mcp.json = %+v", info)
	}
}

// Remote-Formen: Claude Code und Cursor mit type http|sse und url, OpenCode
// mit type remote und url.
func TestListMCPServersErkenntRemote(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".mcp.json"), `{"mcpServers": {
  "atlassian": {"type": "http", "url": "https://mcp.atlassian.com/v1/mcp"},
  "events": {"type": "sse", "url": "https://example.test/sse"}
}}`)
	writeFile(t, filepath.Join(root, "opencode.json"), `{"mcp": {"atlassian": {"type": "remote", "url": "https://mcp.atlassian.com/v1/mcp"}}}`)

	servers, _ := ListMCPServers(root)
	for _, fall := range []struct{ assistant, name, url string }{
		{"claude-code", "atlassian", "https://mcp.atlassian.com/v1/mcp"},
		{"claude-code", "events", "https://example.test/sse"},
		{"opencode", "atlassian", "https://mcp.atlassian.com/v1/mcp"},
	} {
		server := serverFor(t, servers, fall.assistant, fall.name)
		if server.Transport != MCPTransportRemote || server.URL != fall.url || server.Command != "" {
			t.Errorf("%s/%s = %+v, erwartet remote auf %s", fall.assistant, fall.name, server, fall.url)
		}
	}
}

// Was weder lokal noch remote ist, bleibt unknown: gezeigt, nie gestartet.
func TestListMCPServersUnbekannteFormIstUnknown(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".mcp.json"), `{"mcpServers": {
  "leer": {},
  "text": "kein Objekt",
  "nur-args": {"args": ["x"]},
  "http-ohne-url": {"type": "http"}
}}`)
	writeFile(t, filepath.Join(root, "opencode.json"), `{"mcp": {
  "leeres-kommando": {"type": "local", "command": []},
  "remote-ohne-url": {"type": "remote"},
  "fremder-typ": {"type": "websocket", "url": "wss://x"}
}}`)

	servers, _ := ListMCPServers(root)
	if len(servers) != 7 {
		t.Fatalf("%d Server, erwartet 7: %+v", len(servers), servers)
	}
	for _, server := range servers {
		if server.Transport != MCPTransportUnknown {
			t.Errorf("%s/%s = %q, erwartet unknown", server.AssistantID, server.Name, server.Transport)
		}
		if server.Command != "" || server.URL != "" {
			t.Errorf("%s/%s trägt Kommando oder URL: %+v", server.AssistantID, server.Name, server)
		}
	}
}

// OpenCode: enabled fehlt → true, enabled false → false. Die anderen Schemata
// kennen kein solches Flag und sind immer aktiv.
func TestListMCPServersLiestEnabled(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "opencode.json"), `{"mcp": {
  "an": {"type": "local", "command": ["a"]},
  "aus": {"type": "local", "command": ["b"], "enabled": false}
}}`)
	writeFile(t, filepath.Join(root, ".mcp.json"), `{"mcpServers": {"immer": {"command": "c"}}}`)

	servers, _ := ListMCPServers(root)
	if !serverFor(t, servers, "opencode", "an").Enabled {
		t.Error("ohne enabled gilt der Eintrag nicht als aktiv")
	}
	if aus := serverFor(t, servers, "opencode", "aus"); aus.Enabled || aus.Transport != MCPTransportLocal {
		t.Errorf("enabled: false wurde nicht gelesen oder der Eintrag verlor seinen Transport: %+v", aus)
	}
	if !serverFor(t, servers, "claude-code", "immer").Enabled {
		t.Error("ein Claude-Code-Eintrag muss aktiv sein")
	}
}

// Liegen opencode.json und opencode.jsonc nebeneinander, werden beide gelesen
// und als doppelt markiert.
func TestListMCPServersLiestBeideOpenCodeDateien(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "opencode.json"), `{"mcp": {"eins": {"type": "local", "command": ["a"]}}}`)
	writeFile(t, filepath.Join(root, "opencode.jsonc"), `{"mcp": {"zwei": {"type": "local", "command": ["b"]}}}`)

	servers, files := ListMCPServers(root)
	if len(files) != 4 {
		t.Fatalf("%d Dateien, erwartet 4: %+v", len(files), files)
	}
	for _, path := range []string{"opencode.json", "opencode.jsonc"} {
		info := fileFor(t, files, path)
		if !info.Ambiguous || info.Count != 1 || info.AssistantID != "opencode" {
			t.Errorf("%s = %+v, erwartet ambiguous mit einem Server", path, info)
		}
	}
	if serverFor(t, servers, "opencode", "eins").File != "opencode.json" {
		t.Error("eins kommt nicht aus opencode.json")
	}
	if serverFor(t, servers, "opencode", "zwei").File != "opencode.jsonc" {
		t.Error("zwei kommt nicht aus opencode.jsonc")
	}
	if fileFor(t, files, ".mcp.json").Ambiguous {
		t.Error(".mcp.json ist als doppelt markiert")
	}
}

// Eine fehlende Datei ist kein Fehler, nur leer.
func TestListMCPServersOhneDateien(t *testing.T) {
	servers, files := ListMCPServers(t.TempDir())
	if len(servers) != 0 {
		t.Errorf("Server ohne Dateien: %+v", servers)
	}
	if len(files) != 3 {
		t.Fatalf("%d Dateien, erwartet 3", len(files))
	}
	for _, info := range files {
		if info.Exists || info.Error != "" || info.Count != 0 {
			t.Errorf("%s = %+v, erwartet nicht vorhanden ohne Fehler", info.Path, info)
		}
	}
}

// Kaputtes JSON in einer Datei hält die anderen nicht auf; der Fehler steht
// an der Datei. Ein Abschnitt, der kein Objekt ist, ebenso.
func TestListMCPServersKaputteDateiBleibtIsoliert(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".mcp.json"), `{"mcpServers": {`)
	writeFile(t, filepath.Join(root, ".cursor", "mcp.json"), `{"mcpServers": ["liste"]}`)
	writeFile(t, filepath.Join(root, "opencode.json"), `{"mcp": {"ok": {"type": "local", "command": ["a"]}}}`)

	servers, files := ListMCPServers(root)
	if len(servers) != 1 || servers[0].Name != "ok" {
		t.Errorf("Server = %+v, erwartet nur ok aus opencode.json", servers)
	}
	if info := fileFor(t, files, ".mcp.json"); !info.Exists || info.Error == "" {
		t.Errorf(".mcp.json = %+v, erwartet einen Fehler", info)
	}
	if info := fileFor(t, files, filepath.Join(".cursor", "mcp.json")); info.Error == "" {
		t.Errorf("Cursor-Datei = %+v, erwartet einen Fehler", info)
	}
	if info := fileFor(t, files, "opencode.json"); info.Error != "" || info.Count != 1 {
		t.Errorf("opencode.json = %+v", info)
	}
}

// Werte aus env und environment erreichen nie das JSON — nur die Schlüsselnamen.
// Der Prozessstart bekommt sie über Environ.
func TestListMCPServersVerbirgtEnvWerte(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".mcp.json"), `{"mcpServers": {"geheim": {"command": "x", "env": {"TOKEN": "sehr-geheim", "ANDERES": 7}}}}`)
	writeFile(t, filepath.Join(root, "opencode.json"), `{"mcp": {"geheim": {"type": "local", "command": ["x"], "environment": {"API_KEY": "streng-geheim"}}}}`)

	servers, _ := ListMCPServers(root)
	encoded, err := json.Marshal(servers)
	if err != nil {
		t.Fatalf("kodieren: %v", err)
	}
	for _, secret := range []string{"sehr-geheim", "streng-geheim"} {
		if strings.Contains(string(encoded), secret) {
			t.Errorf("der Wert %q steht im JSON: %s", secret, encoded)
		}
	}

	claude := serverFor(t, servers, "claude-code", "geheim")
	if strings.Join(claude.EnvKeys, ",") != "ANDERES,TOKEN" {
		t.Errorf("EnvKeys = %v, erwartet ANDERES,TOKEN", claude.EnvKeys)
	}
	if strings.Join(claude.Environ(), ";") != "ANDERES=7;TOKEN=sehr-geheim" {
		t.Errorf("Environ = %v", claude.Environ())
	}
	opencode := serverFor(t, servers, "opencode", "geheim")
	if strings.Join(opencode.Environ(), ";") != "API_KEY=streng-geheim" {
		t.Errorf("Environ = %v", opencode.Environ())
	}
}

// Innerhalb einer Datei sind die Server nach Namen sortiert; die Dateien
// kommen in der Reihenfolge der Ziele.
func TestListMCPServersSortiert(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".mcp.json"), `{"mcpServers": {"zeta": {"command": "z"}, "alpha": {"command": "a"}, "mitte": {"command": "m"}}}`)
	writeFile(t, filepath.Join(root, ".cursor", "mcp.json"), `{"mcpServers": {"beta": {"command": "b"}}}`)

	servers, _ := ListMCPServers(root)
	got := make([]string, 0, len(servers))
	for _, server := range servers {
		got = append(got, server.AssistantID+"/"+server.Name)
	}
	want := "claude-code/alpha claude-code/mitte claude-code/zeta cursor/beta"
	if strings.Join(got, " ") != want {
		t.Errorf("Reihenfolge = %q, erwartet %q", strings.Join(got, " "), want)
	}
}

// Die Pflichtliste markiert Einträge und rechnet je Name × Assistent die
// Lücken aus. Ein Eintrag in einer der beiden OpenCode-Dateien genügt.
func TestMCPServerInventoryRechnetLuecken(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".mcp.json"), `{"mcpServers": {"k-playbook": {"command": "k-playbook", "args": ["mcp"]}, "extra": {"command": "e"}}}`)
	writeFile(t, filepath.Join(root, "opencode.json"), `{"mcp": {}}`)
	writeFile(t, filepath.Join(root, "opencode.jsonc"), `{"mcp": {"k-playbook": {"type": "local", "command": ["k-playbook", "mcp"]}}}`)

	servers, files := ListMCPServers(root)
	inventory := newMCPServerInventory(servers, files, []string{"k-playbook", "atlassian"}, true)

	if !inventory.RequiredConfigured || strings.Join(inventory.Required, ",") != "k-playbook,atlassian" {
		t.Errorf("Pflichtliste = %+v", inventory)
	}
	if !serverFor(t, inventory.Servers, "claude-code", "k-playbook").Required {
		t.Error("k-playbook ist nicht als Pflicht markiert")
	}
	if serverFor(t, inventory.Servers, "claude-code", "extra").Required {
		t.Error("extra ist als Pflicht markiert")
	}

	got := make([]string, 0, len(inventory.Missing))
	for _, gap := range inventory.Missing {
		got = append(got, gap.AssistantID+"/"+gap.Name)
	}
	want := "cursor/k-playbook claude-code/atlassian opencode/atlassian cursor/atlassian"
	if strings.Join(got, " ") != want {
		t.Errorf("Lücken = %q, erwartet %q", strings.Join(got, " "), want)
	}
}

// Ohne Pflichtliste gibt es keine Lücken und keine Markierung; die Felder
// bleiben leere Listen, keine null-Werte.
func TestMCPServerInventoryOhnePflichtliste(t *testing.T) {
	inventory := newMCPServerInventory(nil, nil, nil, false)
	if inventory.RequiredConfigured || len(inventory.Missing) != 0 {
		t.Errorf("Inventar = %+v", inventory)
	}
	encoded, err := json.Marshal(inventory)
	if err != nil {
		t.Fatalf("kodieren: %v", err)
	}
	for _, want := range []string{`"required":[]`, `"missing":[]`} {
		if !strings.Contains(string(encoded), want) {
			t.Errorf("%s fehlt in %s", want, encoded)
		}
	}
}
