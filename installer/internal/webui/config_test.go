package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/guiproc"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// outdatedProject legt ein Projekt mit einer Konfiguration alter
// schema_version an und macht es zum Arbeitsverzeichnis: die Handler leiten
// ihr Projekt über project.Detect() daraus ab.
func outdatedProject(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	writeFile(t, project.ConfigPath(root), `schema_version: 1

project:
  repo_root: code
  vcs: git
`)

	before, err := os.Getwd()
	if err != nil {
		t.Fatalf("Arbeitsverzeichnis: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("nach %s wechseln: %v", root, err)
	}
	t.Cleanup(func() { os.Chdir(before) })
	return root
}

func getConfig(t *testing.T) configResponse {
	t.Helper()

	recorder := httptest.NewRecorder()
	configHandler(recorder, httptest.NewRequest(http.MethodGet, "/api/config", nil))

	var response configResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Antwort nicht lesbar: %v — %s", err, recorder.Body.String())
	}
	return response
}

func postReset(t *testing.T, body string) (int, configResponse) {
	t.Helper()

	recorder := httptest.NewRecorder()
	resetConfigHandler(recorder, httptest.NewRequest(http.MethodPost, "/api/config/reset", strings.NewReader(body)))

	var response configResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Antwort nicht lesbar: %v — %s", err, recorder.Body.String())
	}
	return recorder.Code, response
}

// Eine unbrauchbare Konfiguration darf nicht als eingerichtet durchgehen: der
// Block bliebe sonst verborgen, und alles Weitere wäre Anzeige ohne Grundlage.
func TestConfigHandlerMeldetAlteFassung(t *testing.T) {
	outdatedProject(t)

	response := getConfig(t)
	if response.Installed {
		t.Error("Installed = true, obwohl die Fassung alt ist")
	}
	if response.Schema != project.SchemaOutdated {
		t.Errorf("Schema = %q, erwartet %q", response.Schema, project.SchemaOutdated)
	}
	if response.SchemaVersion != "1" {
		t.Errorf("SchemaVersion = %q, erwartet %q", response.SchemaVersion, "1")
	}
	if !strings.Contains(response.Message, "bitte neu einrichten") {
		t.Errorf("Message = %q, erwartet „bitte neu einrichten“", response.Message)
	}
	// Die Lage des Repositorys ändert sich durch das Neueinrichten nicht.
	if response.Suggestion == nil || response.Suggestion.RepoRoot != "code" {
		t.Errorf("Suggestion = %+v, erwartet repo_root aus der alten Datei", response.Suggestion)
	}
}

func TestResetConfigHandlerLegtNeuAn(t *testing.T) {
	root := outdatedProject(t)

	status, response := postReset(t, `{"repoRoot":"code"}`)
	if status != http.StatusOK {
		t.Fatalf("Status = %d, erwartet %d — %s", status, http.StatusOK, response.Message)
	}
	if !response.Installed {
		t.Error("Installed = false nach dem Zurücksetzen")
	}
	if response.BackupPath == "" {
		t.Error("die Sicherung wird nicht benannt")
	}
	if !strings.Contains(response.Message, filepath.Base(response.BackupPath)) {
		t.Errorf("die Meldung nennt die Sicherung nicht: %q", response.Message)
	}

	config, err := project.ReadConfig(root)
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}
	if err := project.CheckSchema(config); err != nil {
		t.Errorf("die neue Datei besteht die Prüfung nicht: %v", err)
	}
}

// rekeyFixture stellt einen Dienst hin, der in einem Unterverzeichnis ohne
// Konfiguration gestartet wurde und dort registriert ist: Legt
// POST /api/config die Konfiguration im Verzeichnis darüber an, ändert sich
// das aufgelöste Hauptverzeichnis und damit der Schlüssel.
func rekeyFixture(t *testing.T) (state *serverState, root string, startKey string) {
	t.Helper()

	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	workdir := filepath.Join(root, "sub")
	if err := os.Mkdir(workdir, 0o755); err != nil {
		t.Fatal(err)
	}
	chdir(t, workdir)

	startKey, err = guiproc.Key()
	if err != nil {
		t.Fatal(err)
	}
	if startKey != workdir {
		t.Fatalf("Schlüssel vor dem Anlegen = %q, erwartet %q", startKey, workdir)
	}
	registration, err := guiproc.Register(guiproc.Record{Key: startKey, Addr: "127.0.0.1:1", PID: os.Getpid(), Version: "v1"})
	if err != nil {
		t.Fatalf("Registrierung: %v", err)
	}
	t.Cleanup(func() { _ = registration.Remove() })
	return &serverState{version: "v1", registration: registration}, root, startKey
}

func postConfig(t *testing.T, state *serverState, projectDir string) (int, configResponse) {
	t.Helper()

	body, err := json.Marshal(createConfigRequest{ProjectDir: projectDir, RepoRoot: "."})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	state.createConfigHandler(recorder, httptest.NewRequest(http.MethodPost, "/api/config", strings.NewReader(string(body))))
	var response configResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Antwort nicht lesbar: %v — %s", err, recorder.Body.String())
	}
	return recorder.Code, response
}

func healthKey(t *testing.T, state *serverState) string {
	t.Helper()

	recorder := httptest.NewRecorder()
	state.healthHandler(recorder, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	var health guiproc.Health
	if err := json.Unmarshal(recorder.Body.Bytes(), &health); err != nil {
		t.Fatalf("Antwort lesen: %v", err)
	}
	return health.Key
}

// Nach POST /api/config liegt die Laufzeitdatei unter dem neuen Schlüssel,
// und /api/health meldet ihn.
func TestConfigAnlegenSchluesseltUm(t *testing.T) {
	state, root, startKey := rekeyFixture(t)
	if got := healthKey(t, state); got != startKey {
		t.Fatalf("key vor dem Anlegen = %q, erwartet %q", got, startKey)
	}

	status, response := postConfig(t, state, root)
	if status != http.StatusOK || !response.Installed {
		t.Fatalf("Anlegen: %d %+v", status, response)
	}
	if strings.Contains(response.Message, "Hinweis") {
		t.Errorf("Hinweis ohne Anlass: %q", response.Message)
	}
	if got := healthKey(t, state); got != root {
		t.Errorf("key nach dem Umschlüsseln = %q, erwartet %q", got, root)
	}
}

// Scheitert das Umschlüsseln — hier, weil unter dem neuen Schlüssel schon eine
// Laufzeitdatei liegt —, ist die Konfiguration trotzdem angelegt, die Antwort
// trägt den Hinweis, und /api/health meldet weiter den Schlüssel, unter dem
// der Dienst wirklich registriert ist (Task 191). Vorher stand der Fehler nur
// im Log, und das Lebenszeichen nannte den neuen Schlüssel.
func TestConfigAnlegenMeldetGescheitertesUmschluesseln(t *testing.T) {
	state, root, startKey := rekeyFixture(t)
	other, err := guiproc.Register(guiproc.Record{Key: root, Addr: "127.0.0.1:2", PID: os.Getpid(), Version: "v1"})
	if err != nil {
		t.Fatalf("zweite Registrierung: %v", err)
	}
	t.Cleanup(func() { _ = other.Remove() })

	status, response := postConfig(t, state, root)
	if status != http.StatusOK || !response.Installed {
		t.Fatalf("Anlegen: %d %+v", status, response)
	}
	if !strings.HasPrefix(response.Message, project.ConfigFileName+" angelegt.") ||
		!strings.Contains(response.Message, "Hinweis") || !strings.Contains(response.Message, "nicht auf das neue Hauptverzeichnis umschlüsseln") {
		t.Errorf("Message trägt den Hinweis nicht: %q", response.Message)
	}
	if now, err := guiproc.Key(); err != nil || now != root {
		t.Fatalf("berechneter Schlüssel = %q (%v), erwartet %q", now, err, root)
	}
	if got := healthKey(t, state); got != startKey {
		t.Errorf("key nach gescheitertem Umschlüsseln = %q, erwartet den registrierten %q", got, startKey)
	}
	if _, err := os.Stat(state.registration.Path()); err != nil {
		t.Errorf("die Laufzeitdatei unter dem alten Schlüssel fehlt: %v", err)
	}
}
