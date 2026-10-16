package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/guiproc"
)

// guiStatusEnvironment gibt dem Test ein eigenes Laufzeitverzeichnis und macht
// ein leeres Verzeichnis zum Bezug: ohne K-PLAYBOOK.yaml darüber ist der
// Schlüssel das Arbeitsverzeichnis selbst. Zurück kommt das Verzeichnis, in
// dem die Laufzeitdateien liegen würden — es existiert noch nicht.
func guiStatusEnvironment(t *testing.T) string {
	t.Helper()

	runtimeRoot := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", runtimeRoot)
	t.Chdir(t.TempDir())
	return filepath.Join(runtimeRoot, "k-playbook")
}

// registerGUIStatusRecord schreibt die Laufzeitdatei für dieses Projekt und
// gibt ihren Pfad zurück.
func registerGUIStatusRecord(t *testing.T, record guiproc.Record) string {
	t.Helper()

	key, err := guiproc.Key()
	if err != nil {
		t.Fatalf("Schlüssel: %v", err)
	}
	record.Key = key
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

// guiStatusFields ruft `gui status` auf und gibt die Felder der Antwort
// zurück. Als map, nicht als Struct: so ist prüfbar, dass ein nicht gemessenes
// Feld wirklich fehlt und nicht als Nullwert dasteht.
func guiStatusFields(t *testing.T) map[string]any {
	t.Helper()

	var out bytes.Buffer
	if err := runGUIStatus(&out, nil); err != nil {
		t.Fatalf("gui status: %v", err)
	}
	fields := map[string]any{}
	if err := json.Unmarshal(out.Bytes(), &fields); err != nil {
		t.Fatalf("Antwort ist kein JSON-Objekt (%q): %v", out.String(), err)
	}
	return fields
}

// ownGUIStatusRecord ist eine Laufzeitdatei, die auf diesen Testprozess zeigt:
// die PID lebt, und die Startzeit passt. Damit ist alles, was danach
// unterschieden wird, allein die Antwort von /api/health.
func ownGUIStatusRecord(addr string) guiproc.Record {
	return guiproc.Record{
		Addr:      addr,
		PID:       os.Getpid(),
		Version:   guiproc.OwnVersion(),
		Build:     guiproc.OwnBuild(),
		StartTime: guiproc.OwnStartTime().Unix(),
	}
}

// Ohne Laufzeitdatei — und ohne Laufzeitverzeichnis — ist die Antwort genau
// {"state": "none"} und kein Fehler.
func TestGUIStatusOhneDienst(t *testing.T) {
	guiStatusEnvironment(t)

	fields := guiStatusFields(t)
	if fields["state"] != string(guiproc.StateNone) {
		t.Errorf("state = %v, erwartet none", fields["state"])
	}
	if len(fields) != 1 {
		t.Errorf("neben state stehen weitere Felder: %v", fields)
	}
}

// Der eigene Dienst läuft: Adresse, PID, Version und Build-Kennung stehen in
// der Antwort, und Version und Build kommen aus /api/health.
func TestGUIStatusLaufenderDienst(t *testing.T) {
	guiStatusEnvironment(t)

	key, err := guiproc.Key()
	if err != nil {
		t.Fatalf("Schlüssel: %v", err)
	}
	addr := serveGUIStatusHealth(t, guiproc.Health{
		Status: "ok", Key: key, Version: guiproc.OwnVersion(), Build: guiproc.OwnBuild(), PID: os.Getpid(),
	})
	registerGUIStatusRecord(t, ownGUIStatusRecord(addr))

	fields := guiStatusFields(t)
	if fields["state"] != string(guiproc.StateRunning) {
		t.Fatalf("state = %v, erwartet running: %v", fields["state"], fields)
	}
	if fields["url"] != "http://"+addr+"/" {
		t.Errorf("url = %v, erwartet die Adresse des Dienstes", fields["url"])
	}
	if pid, ok := fields["pid"].(float64); !ok || int(pid) != os.Getpid() {
		t.Errorf("pid = %v, erwartet %d", fields["pid"], os.Getpid())
	}
	if guiproc.OwnBuild() != "" && fields["build"] != guiproc.OwnBuild() {
		t.Errorf("build = %v, erwartet %q", fields["build"], guiproc.OwnBuild())
	}
}

// Ein Dienst aus einem anderen Stand ist ein eigener Zustand — heute nur
// erkennbar, wenn man den Dienst beendet. Version und Build der Antwort sind
// die des fremden Standes, nicht die eigenen.
func TestGUIStatusFremderStand(t *testing.T) {
	guiStatusEnvironment(t)

	key, err := guiproc.Key()
	if err != nil {
		t.Fatalf("Schlüssel: %v", err)
	}
	addr := serveGUIStatusHealth(t, guiproc.Health{
		Status: "ok", Key: key, Version: "0.0.1-anders", Build: "anderer-build", PID: os.Getpid(),
	})
	registerGUIStatusRecord(t, ownGUIStatusRecord(addr))

	fields := guiStatusFields(t)
	if fields["state"] != string(guiproc.StateOtherVersion) {
		t.Fatalf("state = %v, erwartet otherVersion: %v", fields["state"], fields)
	}
	if fields["version"] != "0.0.1-anders" || fields["build"] != "anderer-build" {
		t.Errorf("Version und Build kommen nicht aus /api/health: %v", fields)
	}
}

// Der Prozess lebt und ist der aus der Datei, antwortet aber nicht: Adresse
// und PID stehen da — an ihnen hängt der Weg über `k-playbook stop` —, Version
// und Build fehlen, weil sie nur aus /api/health kommen.
func TestGUIStatusHaengenderDienst(t *testing.T) {
	guiStatusEnvironment(t)

	// Port 1 ist privilegiert; dort hört nichts, und der Versuch endet
	// unmittelbar mit „connection refused".
	registerGUIStatusRecord(t, ownGUIStatusRecord("127.0.0.1:1"))

	fields := guiStatusFields(t)
	if fields["state"] != string(guiproc.StateUnresponsive) {
		t.Fatalf("state = %v, erwartet unresponsive: %v", fields["state"], fields)
	}
	if fields["url"] != "http://127.0.0.1:1/" {
		t.Errorf("url = %v", fields["url"])
	}
	if _, ok := fields["version"]; ok {
		t.Errorf("version steht da, obwohl niemand geantwortet hat: %v", fields)
	}
	if _, ok := fields["build"]; ok {
		t.Errorf("build steht da, obwohl niemand geantwortet hat: %v", fields)
	}
}

// Ein verwaister Stand wird gemeldet und nicht aufgeräumt: die Datei liegt
// nach dem Aufruf unverändert da. Adresse und PID fehlen — sie nennen einen
// Prozess, den es nicht mehr gibt.
func TestGUIStatusVerwaisterStandBleibtLiegen(t *testing.T) {
	guiStatusEnvironment(t)

	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Skipf("Kindprozess: %v", err)
	}
	path := registerGUIStatusRecord(t, guiproc.Record{
		Addr: "127.0.0.1:1", PID: cmd.Process.Pid, StartTime: guiproc.OwnStartTime().Unix(),
	})
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Laufzeitdatei lesen: %v", err)
	}

	fields := guiStatusFields(t)
	if fields["state"] != string(guiproc.StateOrphaned) {
		t.Fatalf("state = %v, erwartet orphaned: %v", fields["state"], fields)
	}
	if len(fields) != 1 {
		t.Errorf("neben state stehen weitere Felder: %v", fields)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("die verwaiste Datei ist weg: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Error("die verwaiste Datei wurde verändert")
	}
}

// `gui status` verändert nichts — auch nicht das Laufzeitverzeichnis: fehlte
// es vorher, fehlt es nachher. Der gewiesene Griff über guiproc.Inspect legte
// es an und scheiterte, wenn das misslingt.
func TestGUIStatusLegtNichtsAn(t *testing.T) {
	runtimeDir := guiStatusEnvironment(t)

	if _, err := os.Stat(runtimeDir); !os.IsNotExist(err) {
		t.Fatalf("das Laufzeitverzeichnis besteht schon: %v", err)
	}
	guiStatusFields(t)
	if _, err := os.Stat(runtimeDir); !os.IsNotExist(err) {
		t.Errorf("%s wurde angelegt", runtimeDir)
	}
}

// `gui` ohne Unterkommando startet nichts: es druckt die Hilfe und endet mit
// Fehler, wie ein unbekanntes Kommando. Dasselbe gilt für ein unbekanntes
// Unterkommando und ein überzähliges Argument.
func TestGUIOhneUndMitUnbekanntemUnterkommando(t *testing.T) {
	guiStatusEnvironment(t)

	if err := runGUICommand(nil); err == nil {
		t.Error("gui ohne Unterkommando ohne Fehler")
	}
	if err := runGUICommand([]string{"start"}); err == nil {
		t.Error("unbekanntes Unterkommando ohne Fehler")
	}
	if err := runGUIStatus(&bytes.Buffer{}, []string{"zuviel"}); err == nil {
		t.Error("überzähliges Argument ohne Fehler")
	}
}
