package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// newMCPProject legt ein Projekt mit Konfiguration und den MCP-Dateien an und
// macht es zum Arbeitsverzeichnis: die Handler leiten ihr Projekt daraus ab.
func newMCPProject(t *testing.T, config string, files map[string]string) string {
	t.Helper()

	root := t.TempDir()
	if err := project.CreateConfig(root, "."); err != nil {
		t.Fatalf("Konfiguration anlegen: %v", err)
	}
	if config != "" {
		if err := os.WriteFile(project.ConfigPath(root), []byte(config), 0o644); err != nil {
			t.Fatalf("Konfiguration schreiben: %v", err)
		}
	}
	for path, content := range files {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("%s anlegen: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("%s schreiben: %v", path, err)
		}
	}
	chdir(t, root)
	return root
}

// Die Übersicht antwortet mit der Matrix aus den drei Projektdateien und
// meldet Lücken in der Pflichtliste: ok ist erst wahr, wenn jeder
// Pflichtname bei jedem Assistenten steht.
func TestMCPServersAPIMeldetPflichtluecke(t *testing.T) {
	newMCPProject(t, "schema_version: 3\n\nproject:\n  repo_root: .\n\ntools:\n  mcp:\n    required:\n      - k-playbook\n      - atlassian\n", map[string]string{
		".mcp.json":                          `{"mcpServers": {"k-playbook": {"command": "k-playbook", "args": ["mcp"]}}}`,
		"opencode.json":                      `{"mcp": {"k-playbook": {"type": "local", "command": ["k-playbook", "mcp"], "enabled": true}}}`,
		filepath.Join(".cursor", "mcp.json"): `{"mcpServers": {"k-playbook": {"command": "k-playbook", "args": ["mcp"]}}}`,
	})

	var response mcpServersResponse
	if status := getJSON(t, "/api/mcp-servers", &response); status != http.StatusOK {
		t.Fatalf("Status = %d, erwartet 200", status)
	}
	if !response.Environment.Installed {
		t.Fatal("das Projekt gilt nicht als installiert")
	}
	if response.OK {
		t.Error("ok ist true, obwohl atlassian fehlt")
	}
	if !response.RequiredConfigured || strings.Join(response.Required, ",") != "k-playbook,atlassian" {
		t.Errorf("Pflichtliste = %v (configured %t)", response.Required, response.RequiredConfigured)
	}
	if len(response.Missing) != 3 {
		t.Errorf("Missing = %+v, erwartet atlassian bei allen drei Assistenten", response.Missing)
	}
	for _, gap := range response.Missing {
		if gap.Name != "atlassian" {
			t.Errorf("Lücke %+v ist nicht atlassian", gap)
		}
	}
	if len(response.Servers) != 3 || len(response.Files) != 3 {
		t.Errorf("Servers = %d, Files = %d, erwartet je 3", len(response.Servers), len(response.Files))
	}
	for _, server := range response.Servers {
		if !server.Required || !server.Own {
			t.Errorf("%+v ist nicht als Pflicht und eigener Server markiert", server)
		}
	}
}

// Ohne Pflichtliste und ohne Lücke ist die Übersicht in Ordnung; die Listen
// sind leer, nicht null.
func TestMCPServersAPIOhnePflichtliste(t *testing.T) {
	newMCPProject(t, "", map[string]string{
		".mcp.json": `{"mcpServers": {"k-playbook": {"command": "k-playbook", "args": ["mcp"]}}}`,
	})

	recorder := httptest.NewRecorder()
	routes(&serverState{}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/mcp-servers", nil))
	body := recorder.Body.String()
	for _, want := range []string{`"required":[]`, `"missing":[]`, `"requiredConfigured":false`, `"ok":true`} {
		if !strings.Contains(body, want) {
			t.Errorf("%s fehlt in %s", want, body)
		}
	}
}

// Ohne Konfiguration gibt es kein Projekt: die Antwort sagt das und bleibt
// dekodierbar.
func TestMCPServersAPIOhneInstallation(t *testing.T) {
	chdir(t, t.TempDir())

	var response mcpServersResponse
	if status := getJSON(t, "/api/mcp-servers", &response); status != http.StatusOK {
		t.Fatalf("Status = %d, erwartet 200", status)
	}
	if response.Environment.Installed || response.OK || response.Message == "" {
		t.Errorf("Antwort = %+v, erwartet nicht installiert mit Meldung", response)
	}
}

// Die Seite trägt die drei Karten, und die linke Spalte führt den Unterpunkt
// mit aria-current="page".
func TestMCPServersSeite(t *testing.T) {
	newMCPProject(t, "", nil)

	status, body := getPage(t, "/mcp-servers")
	if status != http.StatusOK {
		t.Fatalf("Status = %d, erwartet 200", status)
	}
	for _, want := range []string{
		`id="servers-card"`,
		`id="required-card"`,
		`id="files-card"`,
		`<a class="area-nav-subitem active" href="/mcp-servers" aria-current="page">`,
		`<a class="area-nav-item active" href="/" aria-current="true">`,
		`/static/mcp-servers.js`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("die Seite enthält %q nicht", want)
		}
	}
}

// Die Startseite verlinkt die Übersicht aus der MCP-Karte.
func TestStartseiteVerlinktMCPServer(t *testing.T) {
	newMCPProject(t, "", nil)

	_, body := getPage(t, "/")
	if !strings.Contains(body, `href="/mcp-servers"`) {
		t.Error("die Startseite verlinkt /mcp-servers nicht")
	}
}

// mcpFixtureServer ist ein Server, der den Handshake beantwortet und dabei
// festhält, dass er gestartet wurde: PATH, Arbeitsverzeichnis und den Wert
// einer Umgebungsvariablen aus env.
func mcpFixtureServer(t *testing.T, dir string, beleg string) string {
	t.Helper()
	binary := filepath.Join(dir, "beispiel-mcp")
	writeExecutable(t, binary, `#!/bin/sh
printf '%s\n%s\n%s\n' "$PATH" "$PWD" "$BEISPIEL_TOKEN" > `+beleg+`
printf '%s\n' '{"jsonrpc":"2.0","method":"notifications/message","params":{"level":"info","data":"hallo"}}'
printf '%s\n' '{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-11-25","capabilities":{"tools":{},"prompts":{},"resources":{}},"serverInfo":{"name":"beispiel","version":"9"}}}'
printf '%s\n' '{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"gruss","description":"Grüßt","inputSchema":{"properties":{"name":{"type":"string"}},"required":["name"]}}]}}'
printf '%s\n' '{"jsonrpc":"2.0","id":4,"result":{"resources":[{"uri":"file:///x","name":"x","mimeType":"text/plain"}]}}'
printf '%s\n' '{"jsonrpc":"2.0","id":3,"result":{"prompts":[{"name":"vorlage","arguments":[{"name":"thema","required":true}]}]}}'
`)
	return binary
}

func postProbe(t *testing.T, path string) (int, mcpServerProbeResponse) {
	t.Helper()

	request := httptest.NewRequest(http.MethodPost, path, nil)
	request.Header.Set("Origin", "http://"+request.Host)
	recorder := httptest.NewRecorder()
	routes(&serverState{}).ServeHTTP(recorder, request)

	var response mcpServerProbeResponse
	if recorder.Code == http.StatusOK {
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("Antwort nicht lesbar: %v — %s", err, recorder.Body.String())
		}
	}
	return recorder.Code, response
}

// Die Detailseite gibt es nur für Einträge aus den Projektdateien; alles
// andere ist 404. Der Unterpunkt ist aktiv, aria-current="page" trägt er nicht.
func TestMCPServerDetailseite(t *testing.T) {
	newMCPProject(t, "", map[string]string{
		".mcp.json": `{"mcpServers": {"beispiel": {"command": "beispiel-mcp"}}}`,
	})

	status, body := getPage(t, "/mcp-servers/claude-code/beispiel")
	if status != http.StatusOK {
		t.Fatalf("Status = %d, erwartet 200", status)
	}
	for _, want := range []string{
		`id="config-card"`, `id="server-card"`, `id="tools-card"`, `id="prompts-card"`, `id="resources-card"`,
		`<a class="area-nav-subitem active" href="/mcp-servers">`,
		`<a class="area-nav-item active" href="/" aria-current="true">`,
		`<h1>beispiel</h1>`,
		`/static/mcp-server.js`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("die Seite enthält %q nicht", want)
		}
	}
	if strings.Contains(body, `href="/mcp-servers" aria-current="page"`) {
		t.Error("die Detailseite markiert die Übersicht als offene Seite")
	}

	for _, path := range []string{"/mcp-servers/claude-code/unbekannt", "/mcp-servers/opencode/beispiel", "/mcp-servers/fremd/beispiel"} {
		if status, _ := getPage(t, path); status != http.StatusNotFound {
			t.Errorf("%s: Status = %d, erwartet 404", path, status)
		}
	}
}

// GET liefert die Konfiguration und startet nichts. Erst der POST misst —
// mit der geerbten PATH, dem Hauptverzeichnis als Arbeitsverzeichnis und env
// aus dem Eintrag; Benachrichtigungen und vertauschte Antworten stören nicht.
func TestMCPServerProbeNurPerPost(t *testing.T) {
	bin := t.TempDir()
	beleg := filepath.Join(bin, "beleg.txt")
	mcpFixtureServer(t, bin, beleg)
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))

	root := newMCPProject(t, "", map[string]string{
		".mcp.json": `{"mcpServers": {"beispiel": {"command": "beispiel-mcp", "args": ["--x"], "env": {"BEISPIEL_TOKEN": "sehr-geheim"}}}}`,
	})

	recorder := httptest.NewRecorder()
	routes(&serverState{}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/mcp-servers/claude-code/beispiel", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET: Status = %d, erwartet 200", recorder.Code)
	}
	var detail mcpServerDetailResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &detail); err != nil {
		t.Fatalf("Antwort nicht lesbar: %v", err)
	}
	if !detail.Probeable || detail.ResolvedCommand != filepath.Join(bin, "beispiel-mcp") {
		t.Errorf("Detail = %+v, erwartet aufgelösten Pfad", detail)
	}
	if strings.Contains(recorder.Body.String(), "sehr-geheim") {
		t.Error("der env-Wert steht in der Antwort")
	}
	if strings.Join(detail.Entry.EnvKeys, ",") != "BEISPIEL_TOKEN" {
		t.Errorf("EnvKeys = %v", detail.Entry.EnvKeys)
	}
	if _, err := os.Stat(beleg); err == nil {
		t.Fatal("GET hat den Server gestartet")
	}

	status, response := postProbe(t, "/api/mcp-servers/claude-code/beispiel/probe")
	if status != http.StatusOK {
		t.Fatalf("POST: Status = %d, erwartet 200", status)
	}
	if !response.Started || !response.Available {
		t.Fatalf("Messung = %+v", response)
	}
	if response.ServerName != "beispiel" || response.ServerVersion != "9" {
		t.Errorf("Server = %s %s", response.ServerName, response.ServerVersion)
	}
	if strings.Join(response.Capabilities, ",") != "prompts,resources,tools" {
		t.Errorf("Capabilities = %v", response.Capabilities)
	}
	if len(response.Tools) != 1 || response.Tools[0].Name != "gruss" || len(response.Tools[0].Parameters) != 1 {
		t.Errorf("Tools = %+v", response.Tools)
	}
	if len(response.Prompts) != 1 || response.Prompts[0].Name != "vorlage" || len(response.Prompts[0].Arguments) != 1 {
		t.Errorf("Prompts = %+v", response.Prompts)
	}
	if len(response.Resources) != 1 || response.Resources[0].URI != "file:///x" {
		t.Errorf("Resources = %+v", response.Resources)
	}
	if !strings.HasSuffix(response.Command, "beispiel-mcp --x") {
		t.Errorf("Command = %q", response.Command)
	}

	raw, err := os.ReadFile(beleg)
	if err != nil {
		t.Fatalf("der Server wurde nicht gestartet: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 3 {
		t.Fatalf("Beleg = %q", raw)
	}
	if !strings.HasPrefix(lines[0], bin+":") {
		t.Errorf("PATH des Subprozesses = %q, erwartet die geerbte PATH", lines[0])
	}
	if resolvePath(lines[1]) != resolvePath(root) {
		t.Errorf("Arbeitsverzeichnis = %q, erwartet %q", lines[1], root)
	}
	if lines[2] != "sehr-geheim" {
		t.Errorf("env aus dem Eintrag kam nicht an: %q", lines[2])
	}
}

// Ein Kommando, das es nicht gibt, ist ein Ergebnis der Messung: „ließ sich
// nicht starten", kein Fehlerstatus.
func TestMCPServerProbeOhneKommando(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	newMCPProject(t, "", map[string]string{
		filepath.Join(".cursor", "mcp.json"): `{"mcpServers": {"kaputt": {"command": "gibt-es-nicht-4711"}}}`,
	})

	status, response := postProbe(t, "/api/mcp-servers/cursor/kaputt/probe")
	if status != http.StatusOK {
		t.Fatalf("Status = %d, erwartet 200", status)
	}
	if response.Started || response.Available || !strings.Contains(response.Message, "ließ sich nicht starten") {
		t.Errorf("Messung = %+v", response)
	}
}

// Remote und unknown werden nicht gestartet; der POST sagt das und liefert
// keinen Fehler.
func TestMCPServerProbeRemoteUndUnknown(t *testing.T) {
	newMCPProject(t, "", map[string]string{
		"opencode.json": `{"mcp": {"atlassian": {"type": "remote", "url": "https://mcp.atlassian.com/v1/mcp"}, "seltsam": {"type": "websocket"}}}`,
	})

	for _, name := range []string{"atlassian", "seltsam"} {
		status, response := postProbe(t, "/api/mcp-servers/opencode/"+name+"/probe")
		if status != http.StatusOK {
			t.Errorf("%s: Status = %d, erwartet 200", name, status)
			continue
		}
		if response.Started || response.Available || !strings.HasPrefix(response.Message, "Nicht gemessen") {
			t.Errorf("%s: Messung = %+v", name, response)
		}
	}

	if status, _ := postProbe(t, "/api/mcp-servers/opencode/unbekannt/probe"); status != http.StatusNotFound {
		t.Errorf("unbekannter Server: Status = %d, erwartet 404", status)
	}
}
