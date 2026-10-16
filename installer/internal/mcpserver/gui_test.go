package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kascada/k-playbook/installer/internal/guiproc"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// newGUIStatusProject legt ein Projekt an und gibt dem Test ein eigenes
// Laufzeitverzeichnis. Ein Verzeichnis **mit** Installation ist nötig:
// resolveProjectDir lehnt jedes andere ab, auch wenn guiproc für ein solches
// trotzdem einen Schlüssel bildet. Zurück kommt dazu das Verzeichnis, in dem
// die Laufzeitdateien liegen würden — es existiert noch nicht.
func newGUIStatusProject(t *testing.T) (string, string) {
	t.Helper()

	runtimeRoot := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", runtimeRoot)

	root := t.TempDir()
	if err := project.CreateConfig(root, "."); err != nil {
		t.Fatalf("Konfiguration anlegen: %v", err)
	}
	return root, filepath.Join(runtimeRoot, "k-playbook")
}

// registerGUIStatusFile schreibt die Laufzeitdatei für das Projekt unter
// projectDir und gibt ihren Pfad zurück.
func registerGUIStatusFile(t *testing.T, projectDir string, record guiproc.Record) string {
	t.Helper()

	record.Key = guiproc.KeyFrom(projectDir)
	registration, err := guiproc.Register(record)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	return registration.Path()
}

// serveGUIStatusHealth stellt einen Dienst hin, der auf /api/health mit health
// antwortet, und gibt dessen Adresse zurück.
func serveGUIStatusHealth(t *testing.T, health guiproc.Health) string {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/health" {
			http.NotFound(writer, request)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(health)
	}))
	t.Cleanup(server.Close)
	return strings.TrimPrefix(server.URL, "http://")
}

// ownGUIStatusRecord ist eine Laufzeitdatei, die auf diesen Testprozess zeigt:
// die PID lebt, und die Startzeit passt. Was danach unterschieden wird, ist
// allein die Antwort von /api/health.
func ownGUIStatusRecord(addr string) guiproc.Record {
	return guiproc.Record{
		Addr:      addr,
		PID:       os.Getpid(),
		Version:   guiproc.OwnVersion(),
		Build:     guiproc.OwnBuild(),
		StartTime: guiproc.OwnStartTime().Unix(),
	}
}

// callGUIStatus ruft das Werkzeug auf und gibt die Felder der Antwort zurück.
// Als map, nicht als Struct: so ist prüfbar, dass ein nicht gemessenes Feld
// wirklich fehlt und nicht als Nullwert dasteht.
func callGUIStatus(t *testing.T, projectDir string) (map[string]any, bool) {
	t.Helper()

	result, _, err := guiStatusTool(context.Background(), nil, guiStatusInput{ProjectDir: projectDir})
	if err != nil {
		t.Fatalf("gui_status: %v", err)
	}
	if len(result.Content) != 1 {
		t.Fatalf("erwartet genau einen Inhaltsblock, bekommen %d", len(result.Content))
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("Inhalt ist kein Text: %#v", result.Content[0])
	}
	fields := map[string]any{}
	if err := json.Unmarshal([]byte(text.Text), &fields); err != nil {
		t.Fatalf("Antwort ist kein JSON-Objekt (%q): %v", text.Text, err)
	}
	return fields, result.IsError
}

// wantGUIStatus prüft den Umschlag und gibt den Zustand zurück.
func wantGUIStatus(t *testing.T, fields map[string]any, isError bool, projectDir string) string {
	t.Helper()

	if isError {
		t.Fatalf("Werkzeugfehler: %v", fields)
	}
	if fields["ok"] != true || fields["tool"] != guiToolStatus || fields["projectDir"] != projectDir {
		t.Fatalf("Umschlag: %v", fields)
	}
	state, ok := fields["state"].(string)
	if !ok {
		t.Fatalf("state fehlt: %v", fields)
	}
	return state
}

// Ohne Laufzeitdatei — und ohne Laufzeitverzeichnis — ist die Antwort
// state none, und angelegt wird dabei nichts.
func TestGUIStatusToolOhneDienst(t *testing.T) {
	root, runtimeDir := newGUIStatusProject(t)

	fields, isError := callGUIStatus(t, root)
	if state := wantGUIStatus(t, fields, isError, root); state != string(guiproc.StateNone) {
		t.Errorf("state = %q, erwartet none", state)
	}
	for _, name := range []string{"url", "pid", "version", "build"} {
		if _, ok := fields[name]; ok {
			t.Errorf("%s steht da, obwohl nichts läuft: %v", name, fields)
		}
	}
	if _, err := os.Stat(runtimeDir); !os.IsNotExist(err) {
		t.Errorf("%s wurde angelegt: %v", runtimeDir, err)
	}
}

// Der eigene Dienst läuft: Adresse, PID, Version und Build-Kennung stehen in
// der Antwort, Version und Build aus /api/health.
func TestGUIStatusToolLaufenderDienst(t *testing.T) {
	root, _ := newGUIStatusProject(t)

	addr := serveGUIStatusHealth(t, guiproc.Health{
		Status: "ok", Key: guiproc.KeyFrom(root), Version: guiproc.OwnVersion(),
		Build: guiproc.OwnBuild(), PID: os.Getpid(),
	})
	registerGUIStatusFile(t, root, ownGUIStatusRecord(addr))

	fields, isError := callGUIStatus(t, root)
	if state := wantGUIStatus(t, fields, isError, root); state != string(guiproc.StateRunning) {
		t.Fatalf("state = %q, erwartet running: %v", state, fields)
	}
	if fields["url"] != "http://"+addr+"/" {
		t.Errorf("url = %v", fields["url"])
	}
	if pid, ok := fields["pid"].(float64); !ok || int(pid) != os.Getpid() {
		t.Errorf("pid = %v, erwartet %d", fields["pid"], os.Getpid())
	}
}

// Ein Dienst aus einem anderen Stand ist ein eigener Zustand, und Version und
// Build der Antwort sind die des fremden Standes.
func TestGUIStatusToolFremderStand(t *testing.T) {
	root, _ := newGUIStatusProject(t)

	addr := serveGUIStatusHealth(t, guiproc.Health{
		Status: "ok", Key: guiproc.KeyFrom(root), Version: "0.0.1-anders",
		Build: "anderer-build", PID: os.Getpid(),
	})
	registerGUIStatusFile(t, root, ownGUIStatusRecord(addr))

	fields, isError := callGUIStatus(t, root)
	if state := wantGUIStatus(t, fields, isError, root); state != string(guiproc.StateOtherVersion) {
		t.Fatalf("state = %q, erwartet otherVersion: %v", state, fields)
	}
	if fields["version"] != "0.0.1-anders" || fields["build"] != "anderer-build" {
		t.Errorf("Version und Build kommen nicht aus /api/health: %v", fields)
	}
}

// Der Prozess lebt und ist der aus der Datei, antwortet aber nicht: Adresse
// und PID stehen da, Version und Build fehlen.
func TestGUIStatusToolHaengenderDienst(t *testing.T) {
	root, _ := newGUIStatusProject(t)

	// Port 1 ist privilegiert; dort hört nichts.
	registerGUIStatusFile(t, root, ownGUIStatusRecord("127.0.0.1:1"))

	fields, isError := callGUIStatus(t, root)
	if state := wantGUIStatus(t, fields, isError, root); state != string(guiproc.StateUnresponsive) {
		t.Fatalf("state = %q, erwartet unresponsive: %v", state, fields)
	}
	if fields["url"] != "http://127.0.0.1:1/" {
		t.Errorf("url = %v", fields["url"])
	}
	if _, ok := fields["version"]; ok {
		t.Errorf("version steht da, obwohl niemand geantwortet hat: %v", fields)
	}
}

// Ein verwaister Stand wird gemeldet und nicht aufgeräumt.
func TestGUIStatusToolVerwaisterStandBleibtLiegen(t *testing.T) {
	root, _ := newGUIStatusProject(t)

	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Skipf("Kindprozess: %v", err)
	}
	path := registerGUIStatusFile(t, root, guiproc.Record{
		Addr: "127.0.0.1:1", PID: cmd.Process.Pid, StartTime: guiproc.OwnStartTime().Unix(),
	})

	fields, isError := callGUIStatus(t, root)
	if state := wantGUIStatus(t, fields, isError, root); state != string(guiproc.StateOrphaned) {
		t.Fatalf("state = %q, erwartet orphaned: %v", state, fields)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("die verwaiste Datei ist weg: %v", err)
	}
}

// Zwei Verzeichnisse, zwei Antworten: das Werkzeug fragt nach dem Projekt in
// projectDir und nicht nach dem Arbeitsverzeichnis des Serverprozesses.
func TestGUIStatusToolZweiVerzeichnisseZweiAntworten(t *testing.T) {
	mitDienst, _ := newGUIStatusProject(t)
	registerGUIStatusFile(t, mitDienst, ownGUIStatusRecord("127.0.0.1:1"))

	ohneDienst := t.TempDir()
	if err := project.CreateConfig(ohneDienst, "."); err != nil {
		t.Fatalf("Konfiguration anlegen: %v", err)
	}

	erste, isError := callGUIStatus(t, mitDienst)
	if state := wantGUIStatus(t, erste, isError, mitDienst); state != string(guiproc.StateUnresponsive) {
		t.Fatalf("state = %q, erwartet unresponsive: %v", state, erste)
	}
	zweite, isError := callGUIStatus(t, ohneDienst)
	if state := wantGUIStatus(t, zweite, isError, ohneDienst); state != string(guiproc.StateNone) {
		t.Fatalf("state = %q, erwartet none: %v", state, zweite)
	}
}

// Ohne projectDir wird abgelehnt — invalid_input wie in den übrigen Familien —
// und nichts verändert; mit einem Verzeichnis ohne Installation
// project_not_found.
func TestGUIStatusToolOhneProjectDir(t *testing.T) {
	_, runtimeDir := newGUIStatusProject(t)

	fields, isError := callGUIStatus(t, "")
	if !isError || fields["ok"] != false {
		t.Fatalf("ein Aufruf ohne projectDir wurde nicht abgelehnt: %v", fields)
	}
	problem, ok := fields["error"].(map[string]any)
	if !ok || problem["code"] != "invalid_input" {
		t.Errorf("Fehlercode: %v", fields["error"])
	}
	if _, ok := fields["state"]; ok {
		t.Errorf("state steht in einer Fehlerantwort: %v", fields)
	}
	if _, err := os.Stat(runtimeDir); !os.IsNotExist(err) {
		t.Errorf("%s wurde angelegt: %v", runtimeDir, err)
	}

	fields, isError = callGUIStatus(t, t.TempDir())
	if !isError {
		t.Fatalf("ein Verzeichnis ohne Installation wurde nicht abgelehnt: %v", fields)
	}
	problem, ok = fields["error"].(map[string]any)
	if !ok || problem["code"] != "project_not_found" {
		t.Errorf("Fehlercode: %v", fields["error"])
	}
}
