package webui

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kascada/k-playbook/installer/internal/program"
	"github.com/kascada/k-playbook/installer/internal/project"
	"github.com/kascada/k-playbook/installer/internal/release"
	"github.com/kascada/k-playbook/installer/internal/release/releasetest"
)

// isolateHome gibt dem Test ein eigenes HOME, einen PATH, der mit dessen
// ~/.local/bin beginnt, und ein eigenes Laufzeitverzeichnis. Das echte
// ~/.local/bin/k-playbook und die Laufzeitdateien paralleler Sitzungen bleiben
// so unerreichbar. Gibt das Installationsziel zurück.
func isolateHome(t *testing.T) string {
	t.Helper()

	home := t.TempDir()
	bin := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatalf("%s anlegen: %v", bin, err)
	}
	t.Setenv("HOME", home)
	t.Setenv("PATH", strings.Join([]string{bin, "/usr/local/bin", "/usr/bin", "/bin"}, string(os.PathListSeparator)))
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	return filepath.Join(bin, project.InstalledCommandName)
}

// versionScript ist ein Programm, das auf `version` mit version antwortet und
// sonst mit Exit 1 endet — als Server taugt es nicht.
func versionScript(version string) string {
	return "#!/bin/sh\nif [ \"$1\" = version ]; then echo " + version + "; exit 0; fi\nexit 1\n"
}

// gitCmd führt git in dir aus, mit fester Identität.
func gitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com",
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}

// gitProject baut ein Projekt, dessen Clone ein Git-Repo mit Upstream ist, und
// stellt eine Ersatzquelle hin, die latest als neuesten Release-Tag nennt —
// ohne Assets; wer eines braucht, legt es mit releasetest.Serve selbst an. Die
// VERSION im Clone weicht bewusst ab: Sie wird nicht mehr gelesen. Zurück
// kommen Hauptverzeichnis und eine zweite Arbeitskopie des Remotes, über die
// Tests neue Stände einspielen.
func gitProject(t *testing.T, latest string) (string, string) {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git nicht verfügbar")
	}
	releasetest.Serve(t).AddTag(latest)
	base := t.TempDir()
	remote := filepath.Join(base, "remote.git")
	gitCmd(t, base, "init", "--bare", "-b", "main", remote)

	upstream := filepath.Join(base, "upstream")
	gitCmd(t, base, "clone", remote, upstream)
	writeFile(t, filepath.Join(upstream, "VERSION"), "v0.0.1\n")
	writeFile(t, filepath.Join(upstream, "commands", "k-test.md"), "test\n")
	gitCmd(t, upstream, "add", "-A")
	gitCmd(t, upstream, "commit", "-m", "erster Stand")
	gitCmd(t, upstream, "push", "-u", "origin", "main")

	projectDir := filepath.Join(base, "projekt")
	writeFile(t, project.ConfigPath(projectDir), "schema_version: 3\n")
	gitCmd(t, projectDir, "clone", remote, project.PlaybookDirName)
	t.Cleanup(func() { makeWritable(projectDir) })
	return projectDir, upstream
}

// pushChange spielt über die zweite Arbeitskopie einen neuen Stand ein.
func pushChange(t *testing.T, upstream string, file string, content string) {
	t.Helper()
	writeFile(t, filepath.Join(upstream, file), content)
	gitCmd(t, upstream, "add", "-A")
	gitCmd(t, upstream, "commit", "-m", "neuer Stand")
	gitCmd(t, upstream, "push")
}

// makeWritable hebt den Schreibschutz auf, den Update und Startpflege auf den
// Clone setzen — sonst scheitert das Aufräumen des Testverzeichnisses.
func makeWritable(dir string) {
	_ = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && entry.IsDir() {
			_ = os.Chmod(path, 0o755)
		}
		return nil
	})
}

func getUpdate(t *testing.T, state *serverState) updateResponse {
	t.Helper()

	recorder := httptest.NewRecorder()
	state.updateCheckHandler(recorder, httptest.NewRequest(http.MethodGet, "/api/update", nil))
	var response updateResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Antwort lesen: %v\n%s", err, recorder.Body.String())
	}
	if response.Program == nil {
		t.Fatalf("die Antwort trägt keinen Programmvergleich: %s", recorder.Body.String())
	}
	return response
}

// Nur „älter" meldet. Gleich, neuer und unbekannt melden nichts — und das,
// obwohl Clone und Remote gleich sind: verglichen wird mit dem neuesten
// Release-Tag, nicht mit dem Clone.
func TestUpdatePruefungMeldetAelteresProgramm(t *testing.T) {
	target := isolateHome(t)
	writeExecutable(t, target, versionScript("v0.9.3"))
	projectDir, _ := gitProject(t, "v0.9.4")
	chdir(t, projectDir)

	tests := []struct {
		running  string
		outdated bool
		order    string
	}{
		{"v0.9.3", true, "älter"},
		{"v0.9.4", false, "gleich"},
		{"v0.9.5", false, "neuer"},
		{"", false, "unbekannt"},
		{"dev", false, "unbekannt"},
	}
	for _, test := range tests {
		response := getUpdate(t, &serverState{version: test.running})
		if response.Program.Outdated != test.outdated || response.Program.Order != test.order {
			t.Errorf("laufend %q: Program = %+v", test.running, response.Program)
		}
		if response.Available {
			t.Errorf("laufend %q: Clone gleich Remote, aber available", test.running)
		}
		if test.outdated && (!response.Program.Installable || response.Program.Latest != "v0.9.4" || response.Program.Running != "v0.9.3") {
			t.Errorf("laufend %q: nicht angeboten: %+v", test.running, response.Program)
		}
	}
}

// Ohne eigene Version wird der neueste Tag nicht abgerufen; ist er unbekannt,
// meldet die Prüfung nichts und nennt den Grund.
func TestUpdatePruefungOhneVersionUndOhneTag(t *testing.T) {
	target := isolateHome(t)
	writeExecutable(t, target, versionScript("v0.9.3"))
	projectDir, _ := gitProject(t, "v0.9.4")
	chdir(t, projectDir)
	source := releasetest.Serve(t)
	source.AddTag("v0.9.4")

	for _, running := range []string{"", "dev"} {
		response := getUpdate(t, &serverState{version: running})
		if response.Program.Outdated || response.Program.Latest != "" || response.Program.LatestError != "" {
			t.Errorf("laufend %q: Program = %+v", running, response.Program)
		}
	}
	if requests := source.Requests(); len(requests) != 0 {
		t.Errorf("ohne eigene Version wurde die Release-Quelle gefragt: %v", requests)
	}

	releasetest.Off(t)
	response := getUpdate(t, &serverState{version: "v0.9.3"})
	if response.Program.Outdated || response.Program.Latest != "" || !strings.Contains(response.Program.LatestError, "abgeschaltet") {
		t.Errorf("Tag unbekannt: Program = %+v", response.Program)
	}
	code, post := postUpdate(t, &serverState{version: "v0.9.3"}, programHandler, "/api/update/program")
	if code != http.StatusConflict || !strings.Contains(post.Message, "Der neueste Release-Tag ist unbekannt") {
		t.Errorf("Knopf bei unbekanntem Tag: Status %d, Meldung %q", code, post.Message)
	}
}

// Ohne Clone meldet die Prüfung das ältere Programm trotzdem: Der Vergleich
// hängt nicht am Clone.
func TestUpdatePruefungMeldetAelteresProgrammOhneClone(t *testing.T) {
	target := isolateHome(t)
	writeExecutable(t, target, versionScript("v0.9.3"))
	projectDir := t.TempDir()
	writeFile(t, project.ConfigPath(projectDir), "schema_version: 3\n")
	chdir(t, projectDir)
	releasetest.Serve(t).AddTag("v0.9.4")

	response := getUpdate(t, &serverState{version: "v0.9.3"})
	if !response.Program.Outdated || !response.Program.Installable || response.Program.Latest != "v0.9.4" {
		t.Errorf("Program = %+v", response.Program)
	}
	if response.Available {
		t.Error("ohne Clone ein Update des Clones gemeldet")
	}
}

// Ohne Remote des Clones — kein Netz dorthin — meldet die Prüfung das ältere
// Programm trotzdem.
func TestUpdatePruefungMeldetAelteresProgrammOhneRemote(t *testing.T) {
	target := isolateHome(t)
	writeExecutable(t, target, versionScript("v0.9.3"))
	projectDir, _ := gitProject(t, "v0.9.4")
	gitCmd(t, filepath.Join(projectDir, project.PlaybookDirName), "remote", "set-url", "origin", filepath.Join(t.TempDir(), "gibt-es-nicht.git"))
	chdir(t, projectDir)

	response := getUpdate(t, &serverState{version: "v0.9.3"})
	if !strings.Contains(response.Message, "Remote nicht erreichbar") {
		t.Errorf("Meldung = %q, erwartet den gescheiterten Remote", response.Message)
	}
	if !response.Program.Outdated || !response.Program.Installable {
		t.Errorf("Program = %+v", response.Program)
	}
}

// Zeigt `k-playbook` im PATH auf ein anderes Programm oder auf keins, gibt es
// keinen Knopf, sondern einen Hinweis mit beiden Pfaden.
func TestUpdatePruefungOhneKnopfBeiFalschemPath(t *testing.T) {
	t.Run("anderes Programm", func(t *testing.T) {
		target := isolateHome(t)
		writeExecutable(t, target, versionScript("v0.9.3"))
		other := filepath.Join(t.TempDir(), project.InstalledCommandName)
		writeExecutable(t, other, versionScript("v0.9.3"))
		t.Setenv("PATH", filepath.Dir(other)+string(os.PathListSeparator)+os.Getenv("PATH"))
		projectDir, _ := gitProject(t, "v0.9.4")
		chdir(t, projectDir)

		response := getUpdate(t, &serverState{version: "v0.9.3"})
		if !response.Program.Outdated || response.Program.Installable {
			t.Fatalf("Program = %+v", response.Program)
		}
		for _, want := range []string{other, target} {
			if !strings.Contains(response.Program.Hint, want) {
				t.Errorf("Hinweis nennt %q nicht: %s", want, response.Program.Hint)
			}
		}
	})

	t.Run("keins", func(t *testing.T) {
		target := isolateHome(t)
		projectDir, _ := gitProject(t, "v0.9.4")
		chdir(t, projectDir)

		response := getUpdate(t, &serverState{version: "v0.9.3"})
		if !response.Program.Outdated || response.Program.Installable {
			t.Fatalf("Program = %+v", response.Program)
		}
		if !strings.Contains(response.Program.Hint, target) || !strings.Contains(response.Program.Hint, "nicht zu finden") {
			t.Errorf("Hinweis: %s", response.Program.Hint)
		}
	})
}

func postUpdate(t *testing.T, state *serverState, handler func(*serverState) http.HandlerFunc, path string) (int, updateResponse) {
	t.Helper()

	recorder := httptest.NewRecorder()
	handler(state)(recorder, httptest.NewRequest(http.MethodPost, path, nil))
	var response updateResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Antwort lesen: %v\n%s", err, recorder.Body.String())
	}
	return recorder.Code, response
}

func pullHandler(state *serverState) http.HandlerFunc    { return state.applyUpdateHandler }
func programHandler(state *serverState) http.HandlerFunc { return state.applyProgramHandler }

// shutdownProbe ist ein shutdown, der meldet, ob er gerufen wurde.
func shutdownProbe() (func(), func() bool) {
	called := make(chan struct{}, 1)
	return func() {
			select {
			case called <- struct{}{}:
			default:
			}
		}, func() bool {
			select {
			case <-called:
				return true
			case <-time.After(3 * shutdownResponseDelay):
				return false
			}
		}
}

// Regression: ein Update bei aktuellem Programm zieht den Stand des Clones,
// verlinkt und lässt den Dienst laufen.
func TestUpdateBeiAktuellemProgrammLaeuftWeiter(t *testing.T) {
	target := isolateHome(t)
	writeExecutable(t, target, versionScript("v0.9.4"))
	projectDir, upstream := gitProject(t, "v0.9.4")
	pushChange(t, upstream, filepath.Join("commands", "k-neu.md"), "neu\n")
	chdir(t, projectDir)

	shutdown, wasShutdown := shutdownProbe()
	code, response := postUpdate(t, &serverState{version: "v0.9.4", shutdown: shutdown}, pullHandler, "/api/update")
	if code != http.StatusOK || response.Restarted || response.InstallFailed {
		t.Fatalf("Status %d, Antwort %+v", code, response)
	}
	if _, err := os.Stat(filepath.Join(projectDir, project.PlaybookDirName, "commands", "k-neu.md")); err != nil {
		t.Errorf("der Pull kam nicht an: %v", err)
	}
	// Die Einrichtung lief mit: sie legt in einem Projekt ohne AGENTS.md die
	// Datei aus der Vorlage an und sagt das.
	if !strings.HasPrefix(response.Message, "Aktualisiert.") || !strings.Contains(response.Message, project.RootInstructionsFile) {
		t.Errorf("Meldung = %q", response.Message)
	}
	if _, err := os.Stat(filepath.Join(projectDir, project.RootInstructionsFile)); err != nil {
		t.Errorf("die Einrichtung lief nicht mit: %v", err)
	}
	if wasShutdown() {
		t.Error("der Dienst hat sich beendet")
	}
}

// Ein laufendes Programm über dem neuesten Tag — ein Build mit
// `make dev-install VERSION=…` — gilt als aktuell: keine Installation, der
// Dienst läuft weiter, auch wenn der Pull die VERSION im Clone bewegt.
func TestUpdateBeiNeueremProgrammInstalliertNicht(t *testing.T) {
	target := isolateHome(t)
	writeExecutable(t, target, versionScript("v0.9.3"))
	projectDir, upstream := gitProject(t, "v0.9.4")
	pushChange(t, upstream, "VERSION", "v0.9.5\n")
	chdir(t, projectDir)
	source := releasetest.Serve(t)
	source.AddRelease("v0.9.4", []byte(versionScript("v0.9.4")))

	shutdown, wasShutdown := shutdownProbe()
	code, response := postUpdate(t, &serverState{version: "v0.10.0", shutdown: shutdown}, pullHandler, "/api/update")
	if code != http.StatusOK || response.Restarted || response.InstallFailed {
		t.Fatalf("Status %d, Antwort %+v", code, response)
	}
	if response.Program.Outdated || response.Program.Latest != "v0.9.4" {
		t.Errorf("neueres Programm als veraltet gemeldet: %+v", response.Program)
	}
	for _, request := range source.Requests() {
		if strings.HasPrefix(request, "/download/") || strings.HasPrefix(request, "/raw/") {
			t.Errorf("es wurde geladen: %s", request)
		}
	}
	if got := program.ReadVersion(target); got != "v0.9.3" {
		t.Errorf("am Ziel liegt %q, die Datei wurde ersetzt", got)
	}
	if wasShutdown() {
		t.Error("der Dienst hat sich beendet")
	}
}

// Älteres Programm, aber der PATH zeigt woandershin: Pull ja, der Dienst läuft
// weiter, Hinweis statt Neustart.
func TestUpdateBeiAelteremProgrammUndFalschemPath(t *testing.T) {
	target := isolateHome(t)
	writeExecutable(t, target, versionScript("v0.9.4"))
	other := filepath.Join(t.TempDir(), project.InstalledCommandName)
	writeExecutable(t, other, versionScript("v0.9.4"))
	t.Setenv("PATH", filepath.Dir(other)+string(os.PathListSeparator)+os.Getenv("PATH"))
	projectDir, upstream := gitProject(t, "v0.9.5")
	pushChange(t, upstream, filepath.Join("commands", "k-neu.md"), "neu\n")
	chdir(t, projectDir)

	shutdown, wasShutdown := shutdownProbe()
	code, response := postUpdate(t, &serverState{version: "v0.9.4", shutdown: shutdown}, pullHandler, "/api/update")
	if code != http.StatusOK || response.Restarted {
		t.Fatalf("Status %d, Antwort %+v", code, response)
	}
	if _, err := os.Stat(filepath.Join(projectDir, project.PlaybookDirName, "commands", "k-neu.md")); err != nil {
		t.Errorf("der Pull kam nicht an: %v", err)
	}
	for _, want := range []string{other, target, "älter als das neueste Release (v0.9.5)", project.FirstInstallCommand} {
		if !strings.Contains(response.Message, want) {
			t.Errorf("der Hinweis nennt %q nicht: %s", want, response.Message)
		}
	}
	if !response.Program.Outdated || response.Program.Installable {
		t.Errorf("Program = %+v", response.Program)
	}
	if wasShutdown() {
		t.Error("der Dienst hat sich beendet")
	}
}

// staleMCPEntry zeigt auf den abgelösten Wrapper — MCPStateOutdated.
const staleMCPEntry = `{"mcpServers":{"` + project.MCPServerKey + `":{"command":"k-playbook/bin/k-playbook","args":["mcp"]}}}` + "\n"

// Nach einem gescheiterten Pull kehrt „Aktualisieren“ nicht zurück, bevor die
// MCP-Registrierung repariert und das Programm geprüft ist. Der gescheiterte
// Pull wird weiter gemeldet, mit 409.
func TestUpdateNachGescheitertemPullRepariertMCPUndPrueftProgramm(t *testing.T) {
	target := isolateHome(t)
	writeExecutable(t, target, versionScript("v0.9.4"))
	projectDir, _ := gitProject(t, "v0.9.4")
	gitCmd(t, filepath.Join(projectDir, project.PlaybookDirName), "remote", "set-url", "origin", filepath.Join(t.TempDir(), "gibt-es-nicht.git"))
	writeFile(t, filepath.Join(projectDir, ".mcp.json"), staleMCPEntry)
	chdir(t, projectDir)

	shutdown, wasShutdown := shutdownProbe()
	code, response := postUpdate(t, &serverState{version: "v0.9.4", shutdown: shutdown}, pullHandler, "/api/update")
	if code != http.StatusConflict || response.Restarted {
		t.Fatalf("Status %d, Antwort %+v", code, response)
	}
	for _, want := range []string{"git pull --ff-only fehlgeschlagen", "MCP-Registrierung auf das installierte k-playbook korrigiert: .mcp.json"} {
		if !strings.Contains(response.Message, want) {
			t.Errorf("Meldung ohne %q: %s", want, response.Message)
		}
	}
	if strings.Contains(response.Message, "Aktualisiert.") {
		t.Errorf("ein gescheiterter Pull meldet „Aktualisiert.“: %s", response.Message)
	}
	if response.Program == nil || response.Program.Latest != "v0.9.4" || response.Program.Outdated {
		t.Errorf("das Programm wurde nach dem gescheiterten Pull nicht geprüft: %+v", response.Program)
	}
	if content := readFile(t, filepath.Join(projectDir, ".mcp.json")); strings.Contains(content, "k-playbook/bin/k-playbook") {
		t.Errorf("der veraltete Eintrag steht noch:\n%s", content)
	}
	// Eingerichtet wird nach einem gescheiterten Pull nicht — wie bisher.
	if _, err := os.Stat(filepath.Join(projectDir, project.RootInstructionsFile)); err == nil {
		t.Error("nach dem gescheiterten Pull lief die Einrichtung")
	}
	if wasShutdown() {
		t.Error("der Dienst hat sich beendet")
	}
}

// Ohne Clone meldet „Aktualisieren“ den fehlenden Clone, repariert die
// MCP-Registrierung und prüft das Programm: Ist es älter, aber nicht
// installierbar, steht der Hinweis da.
func TestUpdateOhneCloneMeldetUndPrueftProgramm(t *testing.T) {
	target := isolateHome(t)
	writeExecutable(t, target, versionScript("v0.9.3"))
	other := filepath.Join(t.TempDir(), project.InstalledCommandName)
	writeExecutable(t, other, versionScript("v0.9.3"))
	t.Setenv("PATH", filepath.Dir(other)+string(os.PathListSeparator)+os.Getenv("PATH"))
	projectDir := t.TempDir()
	writeFile(t, project.ConfigPath(projectDir), "schema_version: 3\n")
	writeFile(t, filepath.Join(projectDir, ".mcp.json"), staleMCPEntry)
	chdir(t, projectDir)
	releasetest.Serve(t).AddTag("v0.9.4")

	code, response := postUpdate(t, &serverState{version: "v0.9.3"}, pullHandler, "/api/update")
	if code != http.StatusConflict || response.Restarted {
		t.Fatalf("Status %d, Antwort %+v", code, response)
	}
	for _, want := range []string{
		"Die Installation " + project.PlaybookDirName + "/ fehlt; der Pull entfällt.",
		"MCP-Registrierung auf das installierte k-playbook korrigiert: .mcp.json",
		"älter als das neueste Release (v0.9.4)",
		other,
	} {
		if !strings.Contains(response.Message, want) {
			t.Errorf("Meldung ohne %q: %s", want, response.Message)
		}
	}
	if !response.Program.Outdated || response.Program.Installable {
		t.Errorf("Program = %+v", response.Program)
	}
}

// Während eines Ablaufs bekommt ein zweiter Aufruf 409 — beide Endpunkte
// teilen dieselbe Sperre.
func TestZweiterAufrufWaehrendDesAblaufsIst409(t *testing.T) {
	isolateHome(t)
	projectDir, _ := gitProject(t, "v0.9.4")
	chdir(t, projectDir)

	state := &serverState{version: "v0.9.3"}
	state.updateMu.Lock()
	defer state.updateMu.Unlock()

	for _, test := range []struct {
		handler func(*serverState) http.HandlerFunc
		path    string
	}{
		{pullHandler, "/api/update"},
		{programHandler, "/api/update/program"},
	} {
		code, response := postUpdate(t, state, test.handler, test.path)
		if code != http.StatusConflict || response.Message != updateBusyMessage {
			t.Errorf("%s: Status %d, Meldung %q", test.path, code, response.Message)
		}
	}
}

// Laufen Befehle oder Chats, fragt der Knopf vor dem Neustart nach: ohne
// Bestätigung 428 mit der Rückfrage, und installiert wird nichts.
func TestProgrammaktualisierungFragtBeiLaufenderArbeit(t *testing.T) {
	target := isolateHome(t)
	writeExecutable(t, target, versionScript("v0.9.3"))
	projectDir, _ := gitProject(t, "v0.9.4")
	chdir(t, projectDir)
	source := releasetest.Serve(t)
	source.AddRelease("v0.9.4", []byte(versionScript("v0.9.4")))

	state := &serverState{version: "v0.9.3"}
	state.openStreams.Store(1)
	state.noteCommandRunning("ses_1", "msg_1")

	code, response := postUpdate(t, state, programHandler, "/api/update/program")
	if code != http.StatusPreconditionRequired || !response.Busy {
		t.Fatalf("Status %d, Antwort %+v", code, response)
	}
	for _, want := range []string{"ein Command läuft", "eine Chat-Ansicht ist verbunden", "Trotzdem"} {
		if !strings.Contains(response.Message, want) {
			t.Errorf("Rückfrage ohne %q: %s", want, response.Message)
		}
	}
	for _, request := range source.Requests() {
		if strings.HasPrefix(request, "/download/") {
			t.Errorf("ohne Bestätigung wurde geladen: %s", request)
		}
	}
	if got := program.ReadVersion(target); got != "v0.9.3" {
		t.Errorf("am Ziel liegt %q, ohne Bestätigung wurde installiert", got)
	}
}

// Ein Programm, das nicht älter ist, wird nicht aktualisiert.
func TestProgrammaktualisierungNurBeiAelteremProgramm(t *testing.T) {
	target := isolateHome(t)
	writeExecutable(t, target, versionScript("v0.9.4"))
	projectDir, _ := gitProject(t, "v0.9.4")
	chdir(t, projectDir)

	code, response := postUpdate(t, &serverState{version: "v0.9.4"}, programHandler, "/api/update/program")
	if code != http.StatusConflict || !strings.Contains(response.Message, "nicht älter") {
		t.Errorf("Status %d, Meldung %q", code, response.Message)
	}
}

// GET /api/update ist ein Hinweis beim Laden der Seite und nimmt den neuesten
// Tag aus dem Cache; mit fresh=1 — dem Knopf „Update prüfen“ — und auf
// POST /api/update fragt der Dienst frisch und schreibt den Cache. Sichtbar an
// der Ersatzquelle. Kein Programm im PATH: installiert wird nichts.
func TestUpdatePruefungLiestDenCacheUndFragtAufWunschFrisch(t *testing.T) {
	isolateHome(t)
	projectDir, _ := gitProject(t, "v0.9.4")
	chdir(t, projectDir)
	source := releasetest.Serve(t)
	source.AddTag("v0.9.4")
	state := &serverState{version: "v0.9.4"}

	// Der erste GET findet keinen Eintrag und fragt.
	if response := getUpdate(t, state); response.Program.Latest != "v0.9.4" || len(source.Requests()) != 1 {
		t.Fatalf("erster GET: %+v, Anfragen %v", response.Program, source.Requests())
	}

	source.AddTag("v0.9.5")
	if response := getUpdate(t, state); response.Program.Latest != "v0.9.4" || response.Program.Outdated || len(source.Requests()) != 1 {
		t.Errorf("GET mit frischem Eintrag: %+v, Anfragen %v", response.Program, source.Requests())
	}

	recorder := httptest.NewRecorder()
	state.updateCheckHandler(recorder, httptest.NewRequest(http.MethodGet, "/api/update?fresh=1", nil))
	var fresh updateResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &fresh); err != nil || fresh.Program == nil {
		t.Fatalf("Antwort: %v\n%s", err, recorder.Body.String())
	}
	if fresh.Program.Latest != "v0.9.5" || !fresh.Program.Outdated || len(source.Requests()) != 2 {
		t.Errorf("GET mit fresh=1: %+v, Anfragen %v", fresh.Program, source.Requests())
	}
	// Das frische Ergebnis steht im Cache: der nächste GET sieht es ohne Anfrage.
	if response := getUpdate(t, state); response.Program.Latest != "v0.9.5" || len(source.Requests()) != 2 {
		t.Errorf("GET nach fresh=1: %+v, Anfragen %v", response.Program, source.Requests())
	}

	source.AddTag("v0.9.6")
	_, post := postUpdate(t, state, pullHandler, "/api/update")
	if post.Program == nil || post.Program.Latest != "v0.9.6" || post.Restarted {
		t.Errorf("POST /api/update: %+v", post)
	}
	if requests := source.Requests(); len(requests) != 3 {
		t.Errorf("POST /api/update hat nicht frisch gefragt: %v", requests)
	}
}

// POST /api/update/program fragt frisch, auch bei frischem Eintrag im Cache,
// und schreibt das Ergebnis in den Cache. Aus dem Cache wäre das laufende
// Programm aktuell (409); frisch gefragt ist es älter, und weil ein Command
// läuft, endet der Aufruf ohne Bestätigung mit der Rückfrage (428) — geladen
// und installiert wird nichts.
func TestProgrammaktualisierungFragtTrotzFrischemCache(t *testing.T) {
	target := isolateHome(t)
	writeExecutable(t, target, versionScript("v0.9.4"))
	projectDir := t.TempDir()
	writeFile(t, project.ConfigPath(projectDir), "schema_version: 3\n")
	chdir(t, projectDir)
	source := releasetest.Serve(t)
	source.AddTag("v0.9.4")

	// Ein frischer Eintrag dieser Quelle: v0.9.4.
	if tag, err := release.CachedLatestTag(context.Background()); err != nil || tag != "v0.9.4" {
		t.Fatalf("CachedLatestTag = %q, %v", tag, err)
	}
	asked := len(source.Requests())
	source.AddRelease("v0.9.5", []byte(versionScript("v0.9.5")))

	state := &serverState{version: "v0.9.4"}
	state.noteCommandRunning("ses_1", "msg_1")
	code, response := postUpdate(t, state, programHandler, "/api/update/program")
	if code != http.StatusPreconditionRequired || !response.Busy {
		t.Fatalf("Status %d, Antwort %+v", code, response)
	}
	if response.Program == nil || response.Program.Latest != "v0.9.5" || !response.Program.Outdated {
		t.Errorf("Program = %+v", response.Program)
	}
	if requests := source.Requests(); len(requests) != asked+1 {
		t.Errorf("POST /api/update/program hat nicht frisch gefragt: %v", requests)
	}
	if got := program.ReadVersion(target); got != "v0.9.4" {
		t.Errorf("am Ziel liegt %q, ohne Bestätigung wurde installiert", got)
	}
	if tag, err := release.CachedLatestTag(context.Background()); err != nil || tag != "v0.9.5" || len(source.Requests()) != asked+1 {
		t.Errorf("der Cache trägt nicht das frische Ergebnis: %q, %v, Anfragen %v", tag, err, source.Requests())
	}
}
