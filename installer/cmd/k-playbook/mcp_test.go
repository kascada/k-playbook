package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// childEnv schaltet das Test-Binary in den Serverbetrieb. Der Server muss als
// eigener Prozess laufen: run() schreibt direkt nach os.Stdout, und genau diese
// Ausgabe ist der Prüfgegenstand — in-process ließe sie sich nicht sauber
// abgreifen.
const childEnv = "K_PLAYBOOK_TEST_MCP_CHILD"

func TestMain(m *testing.M) {
	if os.Getenv(childEnv) == "1" {
		if err := run([]string{"mcp"}); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// newProject legt ein Projekt an, wie es ContextForDir erwartet.
func newProject(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	for _, dir := range []string{
		filepath.Join(root, "k-playbook", "rules"),
		filepath.Join(root, "k-playbook", "reviews"),
		filepath.Join(root, "k-playbook", "checks"),
		filepath.Join(root, "k-playbook-local", "rules"),
		filepath.Join(root, "k-playbook-local", "reviews"),
		filepath.Join(root, "k-playbook-local", "checks"),
		filepath.Join(root, "k-playbook-local", "guidelines"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("%s anlegen: %v", dir, err)
		}
	}
	config := filepath.Join(root, "K-PLAYBOOK.yaml")
	if err := os.WriteFile(config, []byte("schema_version: 3\n\nproject:\n  repo_root: .\n  vcs: git\n"), 0o644); err != nil {
		t.Fatalf("Config anlegen: %v", err)
	}
	return root
}

// speak startet den Server, schickt ihm die Zeilen und gibt alles zurück, was
// auf stdout ankam.
//
// stdin bleibt offen, bis die erwarteten Antworten da sind — genau wie bei einem
// echten Client. Ein Reader, der sofort EOF liefert, beendet die Verbindung,
// während die Anfragen noch in Arbeit sind.
//
// wantResponses zählt die Anfragen mit id; Benachrichtigungen bleiben unbeantwortet.
func speak(t *testing.T, workdir string, wantResponses int, lines ...string) string {
	t.Helper()

	cmd := exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(), childEnv+"=1")
	cmd.Dir = workdir

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin anlegen: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout anlegen: %v", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("Server starten: %v", err)
	}

	if _, err := io.WriteString(stdin, strings.Join(lines, "\n")+"\n"); err != nil {
		t.Fatalf("Anfragen schreiben: %v", err)
	}

	// Erst die erwarteten Antworten einsammeln, dann stdin schließen und den
	// Rest lesen. Was danach noch kommt, gehört nicht auf stdout und fällt beim
	// Zeilenvergleich auf.
	collected := &bytes.Buffer{}
	reader := bufio.NewReader(stdout)
	for range wantResponses {
		line, err := reader.ReadString('\n')
		collected.WriteString(line)
		if err != nil {
			t.Fatalf("Antwort lesen: %v\nbisher:\n%s\nstderr:\n%s", err, collected, stderr.String())
		}
	}

	if err := stdin.Close(); err != nil {
		t.Fatalf("stdin schließen: %v", err)
	}
	rest, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("stdout leerlesen: %v", err)
	}
	collected.Write(rest)

	if err := cmd.Wait(); err != nil {
		t.Fatalf("Server beendete sich mit Fehler: %v\nstderr:\n%s", err, stderr.String())
	}
	return collected.String()
}

func initialize(id int) string {
	// Protokollversion 2025-11-25 mit initialize-Handshake: das ist, was die
	// Clients heute sprechen. Das SDK bedient daneben auch das zustandslose
	// Modell aus 2026-07-28.
	return fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"k-playbook-test","version":"0"}}}`, id)
}

const initialized = `{"jsonrpc":"2.0","method":"notifications/initialized"}`

// decodeResponses zerlegt stdout in JSON-RPC-Antworten und schlägt fehl, sobald
// eine Zeile etwas anderes ist.
//
// Das ist der Kern des Tests: auf stdout läuft der Protokollstrom, und dort darf
// nichts sonst erscheinen. Ein `fmt.Print` in einem transitiv genutzten Paket
// macht die Verbindung unbrauchbar, ohne dass irgendwo ein Fehler auftaucht —
// eine Handprüfung würde das nur zufällig bemerken.
//
// Verglichen wird der Rahmen, nicht der vollständige Antwortinhalt: die Felder,
// die das SDK in eine initialize-Antwort schreibt, gehören ihm und dürfen sich
// mit einer neuen SDK-Fassung ändern, ohne dass hier etwas kaputt ist.
func decodeResponses(t *testing.T, stdout string, wantIDs ...int) []map[string]any {
	t.Helper()

	if stdout == "" {
		t.Fatal("stdout ist leer — es kam keine einzige Antwort")
	}
	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	if len(lines) != len(wantIDs) {
		t.Fatalf("stdout hat %d Zeilen, erwartet %d — Fremdausgabe auf stdout?\n%s",
			len(lines), len(wantIDs), stdout)
	}

	responses := make([]map[string]any, 0, len(lines))
	for i, line := range lines {
		var response map[string]any
		if err := json.Unmarshal([]byte(line), &response); err != nil {
			t.Fatalf("Zeile %d ist kein JSON: %v\n%q", i+1, err, line)
		}
		if response["jsonrpc"] != "2.0" {
			t.Errorf("Zeile %d ist kein JSON-RPC 2.0: %q", i+1, line)
		}
		if got, want := response["id"], float64(wantIDs[i]); got != want {
			t.Errorf("Zeile %d hat id %v, erwartet %v", i+1, got, want)
		}
		if errField, isError := response["error"]; isError {
			t.Errorf("Zeile %d ist eine Fehlerantwort: %v", i+1, errField)
		}
		responses = append(responses, response)
	}
	return responses
}

// result holt das result-Objekt aus einer Antwort.
func result(t *testing.T, response map[string]any) map[string]any {
	t.Helper()

	value, ok := response["result"].(map[string]any)
	if !ok {
		t.Fatalf("Antwort hat kein result-Objekt: %v", response)
	}
	return value
}

// callContext ruft das Werkzeug auf. Ohne dir gilt das Arbeitsverzeichnis des
// Serverprozesses.
func callContext(id int, dir string) string {
	arguments := "{}"
	if dir != "" {
		encoded, _ := json.Marshal(map[string]string{"dir": dir})
		arguments = string(encoded)
	}
	return fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"tools/call","params":{"name":"k_playbook_context","arguments":%s}}`, id, arguments)
}

// toolText holt den Text aus einem Werkzeugergebnis.
func toolText(t *testing.T, response map[string]any) string {
	t.Helper()

	content, ok := result(t, response)["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("Werkzeugergebnis ohne content: %v", response)
	}
	first, ok := content[0].(map[string]any)
	if !ok {
		t.Fatalf("content[0] ist kein Objekt: %v", content[0])
	}
	text, ok := first["text"].(string)
	if !ok {
		t.Fatalf("content[0] hat kein Textfeld: %v", first)
	}
	return text
}

// TestStdoutTraegtNurProtokoll ist die Kernprüfung: initialize, tools/list und
// tools/call — und auf stdout nichts als die drei Antworten. tools/call ist
// dabei der wichtigste Aufruf, weil dort eigener Code läuft.
func TestStdoutTraegtNurProtokoll(t *testing.T) {
	root := newProject(t)

	stdout := speak(t, root, 3,
		initialize(1),
		initialized,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
		callContext(3, root),
	)

	responses := decodeResponses(t, stdout, 1, 2, 3)

	tools, ok := result(t, responses[1])["tools"].([]any)
	if !ok || len(tools) != 1 {
		t.Fatalf("tools/list meldet nicht genau ein Werkzeug: %v", responses[1])
	}
	tool, _ := tools[0].(map[string]any)
	if got := tool["name"]; got != "k_playbook_context" {
		t.Errorf("Werkzeug heißt %v, erwartet k_playbook_context", got)
	}

	var built map[string]any
	if err := json.Unmarshal([]byte(toolText(t, responses[2])), &built); err != nil {
		t.Fatalf("Werkzeug lieferte kein JSON: %v", err)
	}
	if got := built["schemaVersion"]; got != "3" {
		t.Errorf("schemaVersion ist %v, erwartet 3", got)
	}
	if project, ok := built["project"].(map[string]any); !ok || project["dir"] != root {
		t.Errorf("Arbeitsstand beschreibt nicht %s: %v", root, built["project"])
	}
}

// TestOhneDirGiltDasArbeitsverzeichnis prüft den zweiten Auflösungsweg: kein
// Parameter, dafür der Serverprozess im Projekt.
func TestOhneDirGiltDasArbeitsverzeichnis(t *testing.T) {
	root := newProject(t)

	stdout := speak(t, root, 2,
		initialize(1),
		initialized,
		callContext(2, ""),
	)

	responses := decodeResponses(t, stdout, 1, 2)

	var built map[string]any
	if err := json.Unmarshal([]byte(toolText(t, responses[1])), &built); err != nil {
		t.Fatalf("Werkzeug lieferte kein JSON: %v", err)
	}
	if project, ok := built["project"].(map[string]any); !ok || project["dir"] != root {
		t.Errorf("Arbeitsstand beschreibt nicht %s: %v", root, built["project"])
	}
}

// TestFehlerBleibenWerkzeugfehler prüft alle drei Fehlerfälle. Keiner darf den
// Server beenden: er muss für den nächsten Aufruf ansprechbar bleiben.
func TestFehlerBleibenWerkzeugfehler(t *testing.T) {
	tests := []struct {
		name string
		dir  func(t *testing.T) string
	}{
		{
			name: "Verzeichnis existiert nicht",
			dir: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "gibt-es-nicht")
			},
		},
		{
			name: "keine K-PLAYBOOK.yaml",
			dir: func(t *testing.T) string {
				// t.TempDir() liegt unter /tmp, dort findet die Aufwärtssuche
				// keine Konfiguration.
				return t.TempDir()
			},
		},
		{
			name: "fremde schema_version",
			dir: func(t *testing.T) string {
				root := newProject(t)
				config := filepath.Join(root, "K-PLAYBOOK.yaml")
				if err := os.WriteFile(config, []byte("schema_version: 2\n\nproject:\n  repo_root: .\n"), 0o644); err != nil {
					t.Fatalf("Config überschreiben: %v", err)
				}
				return root
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			healthy := newProject(t)

			// Der fehlerhafte Aufruf steht zwischen zwei gesunden: der zweite
			// zeigt, dass der Server weiterlebt.
			stdout := speak(t, healthy, 3,
				initialize(1),
				initialized,
				callContext(2, test.dir(t)),
				callContext(3, healthy),
			)

			lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
			if len(lines) != 3 {
				t.Fatalf("stdout hat %d Zeilen, erwartet 3\n%s", len(lines), stdout)
			}

			var failed map[string]any
			if err := json.Unmarshal([]byte(lines[1]), &failed); err != nil {
				t.Fatalf("Fehlerantwort ist kein JSON: %v", err)
			}
			// Werkzeugfehler, nicht Protokollfehler: das Ergebnis trägt isError,
			// die Antwort selbst kein error-Feld.
			if _, isProtocolError := failed["error"]; isProtocolError {
				t.Errorf("Fehler kam als Protokollfehler statt als Werkzeugfehler: %v", failed)
			}
			if isError := result(t, failed)["isError"]; isError != true {
				t.Errorf("Werkzeugergebnis ist nicht als Fehler markiert: %v", failed)
			}

			var recovered map[string]any
			if err := json.Unmarshal([]byte(lines[2]), &recovered); err != nil {
				t.Fatalf("Antwort nach dem Fehler ist kein JSON: %v", err)
			}
			if isError := result(t, recovered)["isError"]; isError == true {
				t.Errorf("Der Aufruf nach dem Fehler schlug ebenfalls fehl: %v", recovered)
			}
		})
	}
}
