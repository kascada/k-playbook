package webui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// Die geerbte PATH wird ersetzt, nicht ergänzt: der Selbsttest soll den aus
// Dock oder Finder gestarteten Client abbilden, und der hat ~/.local/bin nicht.
func TestMCPProbeEnvErsetztDiePfadVariable(t *testing.T) {
	environ := []string{"HOME=/home/wer", "PATH=/home/wer/.local/bin:/usr/bin", "LANG=de_DE.UTF-8"}

	got := mcpProbeEnv(environ)

	paths := 0
	for _, entry := range got {
		if strings.HasPrefix(entry, "PATH=") {
			paths++
			if entry != "PATH="+mcpProbePath {
				t.Errorf("PATH = %q, erwartet %q", entry, "PATH="+mcpProbePath)
			}
		}
	}
	if paths != 1 {
		t.Errorf("%d PATH-Einträge, erwartet genau einen: %v", paths, got)
	}
	for _, wanted := range []string{"HOME=/home/wer", "LANG=de_DE.UTF-8"} {
		if !containsString(got, wanted) {
			t.Errorf("%q ging verloren: %v", wanted, got)
		}
	}
	if containsString(got, "PATH=/home/wer/.local/bin:/usr/bin") {
		t.Error("die geerbte Shell-PATH steht noch in der Umgebung")
	}
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

// Der Selbsttest startet den registrierten Befehl — den absoluten Pfad, mit dem
// Hauptverzeichnis als Arbeitsverzeichnis und ohne die geerbte Shell-PATH.
//
// Geprüft wird das an einem Platzhalter, der beides festhält, bevor er das
// Protokoll spricht: nur so lässt sich belegen, dass der Test misst, was der
// Client später bekommt.
func TestProbeMCPServerLaeuftOhneGeerbteShellPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", filepath.Join(home, ".local", "bin")+":"+os.Getenv("PATH"))

	projectRoot := t.TempDir()
	beleg := filepath.Join(projectRoot, "beleg.txt")

	binary := filepath.Join(home, ".local", "bin", project.InstalledCommandName)
	writeExecutable(t, binary, `#!/bin/sh
printf '%s\n%s\n' "$PATH" "$PWD" > `+beleg+`
printf '%s\n' '{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-11-25","serverInfo":{"name":"k-playbook","version":"test"}}}'
printf '%s\n' '{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"k_playbook_context","description":"Arbeitsstand"}]}}'
`)

	response := probeMCPServer(projectRoot)
	if !response.Available {
		t.Fatalf("Server nicht erreichbar: %s", response.Message)
	}
	if response.Command != binary+" "+project.MCPSubcommand {
		t.Errorf("Command = %q, erwartet %q", response.Command, binary+" "+project.MCPSubcommand)
	}
	if len(response.Tools) != 1 || response.Tools[0].Name != "k_playbook_context" {
		t.Errorf("Werkzeuge = %+v, erwartet k_playbook_context", response.Tools)
	}

	raw, err := os.ReadFile(beleg)
	if err != nil {
		t.Fatalf("Beleg lesen: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 2 {
		t.Fatalf("Beleg = %q, erwartet PATH und Arbeitsverzeichnis", raw)
	}
	if lines[0] != mcpProbePath {
		t.Errorf("PATH des Subprozesses = %q, erwartet %q", lines[0], mcpProbePath)
	}
	if resolvePath(lines[1]) != resolvePath(projectRoot) {
		t.Errorf("Arbeitsverzeichnis = %q, erwartet %q", lines[1], projectRoot)
	}
}

// Der eigene MCP-Server beschreibt jeden optionalen Parameter mit einem
// type-Feld in Listenform (`["null","array"]`) — so leitet das Go-SDK Zeiger-
// und Slice-Felder ab. Der Selbsttest muss das lesen können.
//
// Der Fall ist keine Feinheit: an einem starren string-Feld scheiterte
// json.Unmarshal auf der ganzen tools/list-Antwort, und die Oberfläche meldete
// „Server antwortet nicht" für einen Server, der einwandfrei antwortete. Der
// Nachweis läuft deshalb über probeMCPServer und nicht über describeTools
// allein: erst der ganze Weg zeigt, dass die Antwort ankommt.
func TestProbeMCPServerLiestTypeInListenform(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	binary := filepath.Join(home, ".local", "bin", project.InstalledCommandName)
	writeExecutable(t, binary, `#!/bin/sh
printf '%s\n' '{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-11-25","serverInfo":{"name":"k-playbook","version":"test"}}}'
printf '%s\n' '{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"k_playbook_review_scan","inputSchema":{"properties":{"entries":{"type":["null","array"],"description":"Auswahl"},"run":{"type":"string"}},"required":["run"]}}]}}'
`)

	response := probeMCPServer(t.TempDir())
	if !response.Available {
		t.Fatalf("Server nicht erreichbar: %s", response.Message)
	}
	if len(response.Tools) != 1 {
		t.Fatalf("Werkzeuge = %+v, erwartet genau eines", response.Tools)
	}

	types := map[string]string{}
	required := map[string]bool{}
	for _, parameter := range response.Tools[0].Parameters {
		types[parameter.Name] = parameter.Type
		required[parameter.Name] = parameter.Required
	}
	if types["entries"] != "null | array" {
		t.Errorf("Typ von entries = %q, erwartet %q", types["entries"], "null | array")
	}
	if types["run"] != "string" {
		t.Errorf("Typ von run = %q, erwartet %q", types["run"], "string")
	}
	if required["entries"] || !required["run"] {
		t.Errorf("Pflichtfelder falsch: %+v", required)
	}
}

// Eine type-Form, die weder Name noch Liste ist, bleibt leer und kippt den
// Selbsttest nicht.
func TestSchemaTypeVerkraftetUnbekannteForm(t *testing.T) {
	for _, fall := range []struct {
		raw    string
		wanted schemaType
	}{
		{`"string"`, "string"},
		{`["null","array"]`, "null | array"},
		{`{"unerwartet":true}`, ""},
		{`null`, ""},
		{`7`, ""},
	} {
		var got schemaType
		if err := json.Unmarshal([]byte(fall.raw), &got); err != nil {
			t.Errorf("%s: Fehler %v — ein type-Feld darf nie scheitern", fall.raw, err)
			continue
		}
		if got != fall.wanted {
			t.Errorf("%s ergab %q, erwartet %q", fall.raw, got, fall.wanted)
		}
	}
}

// Ohne installiertes k-playbook gibt es nichts zu starten; das ist ein
// Ergebnis der Seite, keine Störung.
func TestProbeMCPServerOhneInstalliertesBinary(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	response := probeMCPServer(t.TempDir())
	if response.Available {
		t.Fatal("ohne installiertes Binary wurde ein Server gemeldet")
	}
	if response.Message == "" {
		t.Error("kein Grund genannt")
	}
}

func writeExecutable(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("%s anlegen: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("%s schreiben: %v", path, err)
	}
}
