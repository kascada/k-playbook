package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

// badRequiredConfig ist eine Pflichtliste mit unzulässigem Namen. `k-playbook
// context` bricht daran ab; die Oberfläche zeigt den Fehler und bleibt
// bedienbar.
const badRequiredConfig = "schema_version: 3\n\nproject:\n  repo_root: .\n\ntools:\n  mcp:\n    required: [k-playbook, \"bad name\"]\n"

// Eine nicht lesbare Pflichtliste ist ein Fehler, kein Erfolg: ok ist false,
// requiredError trägt den Grund, die Server stehen trotzdem da, und die Seite
// lädt.
func TestMCPServersAPIPflichtlisteNichtLesbar(t *testing.T) {
	newMCPProject(t, badRequiredConfig, map[string]string{
		".mcp.json": `{"mcpServers": {"k-playbook": {"command": "k-playbook", "args": ["mcp"]}}}`,
	})

	var response mcpServersResponse
	if status := getJSON(t, "/api/mcp-servers", &response); status != http.StatusOK {
		t.Fatalf("Status = %d, erwartet 200", status)
	}
	if response.OK {
		t.Error("ok = true bei unzulässigem Pflichtnamen")
	}
	if !strings.Contains(response.RequiredError, `"bad name"`) {
		t.Errorf("requiredError = %q, erwartet den unzulässigen Namen", response.RequiredError)
	}
	if !response.RequiredConfigured {
		t.Error("requiredConfigured = false, obwohl der Block dasteht")
	}
	if response.Message == "" {
		t.Error("keine Meldung")
	}
	if len(response.Servers) != 1 {
		t.Errorf("Servers = %+v, die Serverliste ging verloren", response.Servers)
	}

	if status, _ := getPage(t, "/mcp-servers"); status != http.StatusOK {
		t.Errorf("Seite: Status = %d, erwartet 200", status)
	}
}

// Die Detailseite trägt denselben Fehler im selben Feld, und entry.required
// ist null statt false: ob der Server Pflicht ist, weiß sie nicht.
func TestMCPServerDetailPflichtlisteNichtLesbar(t *testing.T) {
	newMCPProject(t, badRequiredConfig, map[string]string{
		".mcp.json": `{"mcpServers": {"k-playbook": {"command": "k-playbook", "args": ["mcp"]}}}`,
	})

	recorder := httptest.NewRecorder()
	routes(&serverState{}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/mcp-servers/claude-code/k-playbook", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("Status = %d, erwartet 200", recorder.Code)
	}
	var raw struct {
		RequiredError string                     `json:"requiredError"`
		Entry         map[string]json.RawMessage `json:"entry"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &raw); err != nil {
		t.Fatalf("Antwort nicht lesbar: %v", err)
	}
	if !strings.Contains(raw.RequiredError, `"bad name"`) {
		t.Errorf("requiredError = %q, erwartet den unzulässigen Namen", raw.RequiredError)
	}
	if got := string(raw.Entry["required"]); got != "null" {
		t.Errorf("entry.required = %s, erwartet null", got)
	}

	// Mit lesbarer Liste ist required wieder ein Wahrheitswert.
	if err := os.WriteFile(project.ConfigPath("."), []byte("schema_version: 3\n\ntools:\n  mcp:\n    required: [k-playbook]\n"), 0o644); err != nil {
		t.Fatalf("Konfiguration schreiben: %v", err)
	}
	var detail mcpServerDetailResponse
	if status := getJSON(t, "/api/mcp-servers/claude-code/k-playbook", &detail); status != http.StatusOK {
		t.Fatalf("Status = %d, erwartet 200", status)
	}
	if detail.RequiredError != "" || detail.Entry.Required == nil || !*detail.Entry.Required {
		t.Errorf("Detail = %+v, erwartet required true ohne Fehler", detail)
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
		`<a class="area-nav-item active" href="/setup" aria-current="true">`,
		`/static/mcp-servers.js`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("die Seite enthält %q nicht", want)
		}
	}
}

// Die Setup-Seite verlinkt die Übersicht aus der MCP-Karte.
func TestStartseiteVerlinktMCPServer(t *testing.T) {
	newMCPProject(t, "", nil)

	_, body := getPage(t, "/setup")
	if !strings.Contains(body, `href="/mcp-servers"`) {
		t.Error("die Setup-Seite verlinkt /mcp-servers nicht")
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
		`<a class="area-nav-item active" href="/setup" aria-current="true">`,
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

// Ein relativer Pfad, hinter dem keine Datei liegt, lässt sich auflösen, aber
// nicht starten. Dann lief kein Prozess: started ist false, die Seite zeigt
// „Nicht gemessen" statt „Antwortet nicht".
func TestMCPServerProbeBinaryFehlt(t *testing.T) {
	newMCPProject(t, "", map[string]string{
		filepath.Join(".cursor", "mcp.json"): `{"mcpServers": {"kaputt": {"command": "./bin/gibt-es-nicht"}}}`,
	})

	status, response := postProbe(t, "/api/mcp-servers/cursor/kaputt/probe")
	if status != http.StatusOK {
		t.Fatalf("Status = %d, erwartet 200", status)
	}
	if response.Started {
		t.Errorf("started = true, obwohl kein Prozess lief: %+v", response)
	}
	if response.Available || !strings.Contains(response.Message, "nicht ausführbar") {
		t.Errorf("Messung = %+v", response)
	}
}

// Meldet ein Server prompts, beantwortet prompts/list aber mit einem Fehler,
// bleiben initialize und tools/list stehen: die Werkzeuge erscheinen, die
// Ressourcen auch, und der Fehler kommt als Hinweis mit.
func TestMCPServerProbeFolgeanfrageScheitertNichtFatal(t *testing.T) {
	bin := t.TempDir()
	writeExecutable(t, filepath.Join(bin, "halb-mcp"), `#!/bin/sh
printf '%s\n' '{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-11-25","capabilities":{"tools":{},"prompts":{},"resources":{}},"serverInfo":{"name":"halb","version":"1"}}}'
printf '%s\n' '{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"gruss"}]}}'
printf '%s\n' '{"jsonrpc":"2.0","id":3,"error":{"code":-32601,"message":"prompts/list gibt es nicht"}}'
printf '%s\n' '{"jsonrpc":"2.0","id":4,"result":{"resources":[{"uri":"file:///x"}]}}'
`)
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	newMCPProject(t, "", map[string]string{
		".mcp.json": `{"mcpServers": {"halb": {"command": "halb-mcp"}}}`,
	})

	status, response := postProbe(t, "/api/mcp-servers/claude-code/halb/probe")
	if status != http.StatusOK {
		t.Fatalf("Status = %d, erwartet 200", status)
	}
	if !response.Started || !response.Available {
		t.Fatalf("Messung = %+v, erwartet gestartet und verfügbar", response)
	}
	if response.ServerName != "halb" || strings.Join(response.Capabilities, ",") != "prompts,resources,tools" {
		t.Errorf("Serverdaten gingen verloren: %+v", response)
	}
	if len(response.Tools) != 1 || response.Tools[0].Name != "gruss" {
		t.Errorf("Tools = %+v, erwartet gruss", response.Tools)
	}
	if response.Prompts != nil {
		t.Errorf("Prompts = %+v, erwartet keine", response.Prompts)
	}
	if len(response.Resources) != 1 {
		t.Errorf("Resources = %+v, erwartet eine", response.Resources)
	}
	if !strings.Contains(response.Message, "prompts/list") || !strings.Contains(response.Message, "prompts/list gibt es nicht") {
		t.Errorf("Hinweis = %q, erwartet den Fehler von prompts/list", response.Message)
	}
}

// Schweigt ein Server auf prompts/list, läuft die Frist ab. initialize und
// tools/list sind dann aber schon da: die Werkzeuge bleiben stehen, und die
// Frist wird Hinweis statt Ausfall — ohne den npx/uvx-Hinweis, denn der
// Server lief und antwortete.
func TestMCPServerProbeFolgeanfrageHaengtNichtFatal(t *testing.T) {
	bin := t.TempDir()
	writeExecutable(t, filepath.Join(bin, "zaeh-mcp"), `#!/bin/sh
printf '%s\n' '{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-11-25","capabilities":{"tools":{},"prompts":{}},"serverInfo":{"name":"zaeh","version":"1"}}}'
printf '%s\n' '{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"gruss"}]}}'
while read -r line; do :; done
`)
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	shortProbeTimeout(t, 300*time.Millisecond)
	newMCPProject(t, "", map[string]string{
		".mcp.json": `{"mcpServers": {"zaeh": {"command": "zaeh-mcp"}}}`,
	})

	status, response := postProbe(t, "/api/mcp-servers/claude-code/zaeh/probe")
	if status != http.StatusOK {
		t.Fatalf("Status = %d, erwartet 200", status)
	}
	if !response.Started || !response.Available {
		t.Fatalf("Messung = %+v, erwartet gestartet und verfügbar", response)
	}
	if response.ServerName != "zaeh" || len(response.Tools) != 1 || response.Tools[0].Name != "gruss" {
		t.Errorf("Serverdaten oder Werkzeuge gingen verloren: %+v", response)
	}
	if !strings.Contains(response.Message, "prompts/list") || !strings.Contains(response.Message, "300ms") {
		t.Errorf("Hinweis = %q, erwartet prompts/list und die Frist", response.Message)
	}
	if strings.Contains(response.Message, "npx") {
		t.Errorf("Hinweis = %q, der Installationshinweis gehört nicht dazu", response.Message)
	}
}

// Die Detailseite bekommt nach abgelaufener Frist den npx/uvx-Hinweis — genau
// einmal: probeMCPCommand hängt ihn an, der Handler nichts mehr dazu.
func TestMCPServerProbeTimeoutMitHinweisGenauEinmal(t *testing.T) {
	bin := t.TempDir()
	mcpHangingServer(t, filepath.Join(bin, "stumm-mcp"))
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	shortProbeTimeout(t, 300*time.Millisecond)
	newMCPProject(t, "", map[string]string{
		".mcp.json": `{"mcpServers": {"stumm": {"command": "stumm-mcp"}}}`,
	})

	status, response := postProbe(t, "/api/mcp-servers/claude-code/stumm/probe")
	if status != http.StatusOK {
		t.Fatalf("Status = %d, erwartet 200", status)
	}
	if !response.Started || response.Available {
		t.Errorf("Messung = %+v, erwartet gestartet, nicht verfügbar", response)
	}
	want := "Server antwortet nicht: nach 300ms abgebrochen. " + mcpInstallTimeoutHint
	if response.Message != want {
		t.Errorf("Meldung = %q, erwartet %q", response.Message, want)
	}
	for _, part := range []string{"npx", "Erneut messen"} {
		if count := strings.Count(response.Message, part); count != 1 {
			t.Errorf("%q steht %d-mal in der Meldung, erwartet genau einmal", part, count)
		}
	}
}

// Steht ein Name in opencode.json und opencode.jsonc, wählt ?file= den
// Eintrag: auf der Seite, im GET und im POST. Ohne Parameter gilt der erste,
// ein Dateiname ohne Treffer ist 404 — er wird nur verglichen, nie geöffnet.
func TestMCPServerDoppelterOpenCodeEintragPerFile(t *testing.T) {
	root := newMCPProject(t, "", map[string]string{
		"opencode.json":  `{"mcp": {"doppelt": {"type": "local", "command": ["./bin/lokal-mcp"]}}}`,
		"opencode.jsonc": `{"mcp": {"doppelt": {"type": "remote", "url": "https://example.invalid/mcp"}}}`,
	})
	beleg := filepath.Join(root, "beleg.txt")
	writeExecutable(t, filepath.Join(root, "bin", "lokal-mcp"), "#!/bin/sh\ntouch "+beleg+"\n")

	var remote mcpServerDetailResponse
	if status := getJSON(t, "/api/mcp-servers/opencode/doppelt?file=opencode.jsonc", &remote); status != http.StatusOK {
		t.Fatalf("GET mit file: Status = %d, erwartet 200", status)
	}
	if remote.Entry.File != "opencode.jsonc" || remote.Entry.Transport != project.MCPTransportRemote {
		t.Errorf("GET mit file = %+v, erwartet den Remote-Eintrag aus opencode.jsonc", remote.Entry)
	}

	var first mcpServerDetailResponse
	if status := getJSON(t, "/api/mcp-servers/opencode/doppelt", &first); status != http.StatusOK {
		t.Fatalf("GET ohne file: Status = %d, erwartet 200", status)
	}
	if first.Entry.File != "opencode.json" || first.Entry.Transport != project.MCPTransportLocal {
		t.Errorf("GET ohne file = %+v, erwartet den ersten Eintrag aus opencode.json", first.Entry)
	}

	status, response := postProbe(t, "/api/mcp-servers/opencode/doppelt/probe?file=opencode.jsonc")
	if status != http.StatusOK {
		t.Fatalf("POST mit file: Status = %d, erwartet 200", status)
	}
	if response.Started || response.Command != "https://example.invalid/mcp" || !strings.HasPrefix(response.Message, "Nicht gemessen") {
		t.Errorf("POST mit file = %+v, erwartet den Remote-Eintrag ohne Start", response)
	}
	if _, err := os.Stat(beleg); err == nil {
		t.Error("POST auf den Remote-Eintrag hat das lokale Kommando gestartet")
	}

	if status, _ := getPage(t, "/mcp-servers/opencode/doppelt?file=opencode.jsonc"); status != http.StatusOK {
		t.Errorf("Seite mit file: Status = %d, erwartet 200", status)
	}

	const unknown = "?file=gibt-es-nicht.json"
	if status, _ := getPage(t, "/mcp-servers/opencode/doppelt"+unknown); status != http.StatusNotFound {
		t.Errorf("Seite mit unbekannter Datei: Status = %d, erwartet 404", status)
	}
	recorder := httptest.NewRecorder()
	routes(&serverState{}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/mcp-servers/opencode/doppelt"+unknown, nil))
	if recorder.Code != http.StatusNotFound {
		t.Errorf("GET mit unbekannter Datei: Status = %d, erwartet 404", recorder.Code)
	}
	if status, _ := postProbe(t, "/api/mcp-servers/opencode/doppelt/probe"+unknown); status != http.StatusNotFound {
		t.Errorf("POST mit unbekannter Datei: Status = %d, erwartet 404", status)
	}
	if _, err := os.Stat(beleg); err == nil {
		t.Error("ein POST hat das lokale Kommando gestartet")
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
