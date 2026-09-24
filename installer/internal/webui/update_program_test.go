package webui

import (
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

	"github.com/kascada/k-playbook/installer/internal/project"
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

// gitProject baut ein Projekt, dessen Clone ein Git-Repo mit Upstream ist und
// VERSION version trägt. Zurück kommen Hauptverzeichnis und eine zweite
// Arbeitskopie des Remotes, über die Tests neue Stände einspielen.
func gitProject(t *testing.T, version string) (string, string) {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git nicht verfügbar")
	}
	base := t.TempDir()
	remote := filepath.Join(base, "remote.git")
	gitCmd(t, base, "init", "--bare", "-b", "main", remote)

	upstream := filepath.Join(base, "upstream")
	gitCmd(t, base, "clone", remote, upstream)
	writeFile(t, filepath.Join(upstream, project.VersionFileName), version+"\n")
	writeFile(t, filepath.Join(upstream, "commands", "k-test.md"), "test\n")
	gitCmd(t, upstream, "add", "-A")
	gitCmd(t, upstream, "commit", "-m", "erster Stand")
	gitCmd(t, upstream, "push", "-u", "origin", "main")

	projectDir := filepath.Join(base, "projekt")
	writeFile(t, filepath.Join(projectDir, project.ConfigFileName), "schema_version: 3\n")
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
// obwohl Clone und Remote gleich sind: der Vergleich ist lokal.
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
		if test.outdated && (!response.Program.Installable || response.Program.Clone != "v0.9.4" || response.Program.Running != "v0.9.3") {
			t.Errorf("laufend %q: nicht angeboten: %+v", test.running, response.Program)
		}
	}
}

// Ohne Remote — kein Netz — meldet die Prüfung das ältere Programm trotzdem.
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

// Regression: ein Update ohne Versionswechsel zieht den Stand, verlinkt und
// lässt den Dienst laufen.
func TestUpdateOhneVersionswechselLaeuftWeiter(t *testing.T) {
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

// Versionswechsel bei neuerem laufendem Programm — das Entwicklungsrepo nach
// `make dev-install`: keine Installation, der Dienst läuft weiter.
func TestUpdateMitVersionswechselBeiNeueremProgrammInstalliertNicht(t *testing.T) {
	target := isolateHome(t)
	writeExecutable(t, target, versionScript("v0.9.3"))
	projectDir, upstream := gitProject(t, "v0.9.4")
	marker := filepath.Join(t.TempDir(), "bootstrap-gelaufen")
	pushChange(t, upstream, "bin/install", "#!/bin/sh\n: > "+marker+"\n")
	pushChange(t, upstream, project.VersionFileName, "v0.9.5\n")
	chdir(t, projectDir)

	shutdown, wasShutdown := shutdownProbe()
	code, response := postUpdate(t, &serverState{version: "v0.10.0", shutdown: shutdown}, pullHandler, "/api/update")
	if code != http.StatusOK || response.Restarted || response.InstallFailed {
		t.Fatalf("Status %d, Antwort %+v", code, response)
	}
	if response.Program.Outdated {
		t.Errorf("neueres Programm als veraltet gemeldet: %+v", response.Program)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Error("der Bootstrap ist gelaufen")
	}
	if wasShutdown() {
		t.Error("der Dienst hat sich beendet")
	}
}

// Versionswechsel bei älterem Programm, aber der PATH zeigt woandershin: Pull
// ja, der Dienst läuft weiter, Hinweis statt Neustart.
func TestUpdateMitVersionswechselBeiFalschemPath(t *testing.T) {
	target := isolateHome(t)
	writeExecutable(t, target, versionScript("v0.9.4"))
	other := filepath.Join(t.TempDir(), project.InstalledCommandName)
	writeExecutable(t, other, versionScript("v0.9.4"))
	t.Setenv("PATH", filepath.Dir(other)+string(os.PathListSeparator)+os.Getenv("PATH"))
	projectDir, upstream := gitProject(t, "v0.9.4")
	pushChange(t, upstream, project.VersionFileName, "v0.9.5\n")
	chdir(t, projectDir)

	shutdown, wasShutdown := shutdownProbe()
	code, response := postUpdate(t, &serverState{version: "v0.9.4", shutdown: shutdown}, pullHandler, "/api/update")
	if code != http.StatusOK || response.Restarted {
		t.Fatalf("Status %d, Antwort %+v", code, response)
	}
	if project.InstalledVersion(filepath.Join(projectDir, project.PlaybookDirName)) != "v0.9.5" {
		t.Error("der Pull kam nicht an")
	}
	if !strings.Contains(response.Message, other) || !strings.Contains(response.Message, target) {
		t.Errorf("der Hinweis nennt nicht beide Pfade: %s", response.Message)
	}
	if !response.Program.Outdated || response.Program.Installable {
		t.Errorf("Program = %+v", response.Program)
	}
	if wasShutdown() {
		t.Error("der Dienst hat sich beendet")
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
	marker := filepath.Join(t.TempDir(), "bootstrap-gelaufen")
	writeExecutable(t, filepath.Join(projectDir, project.PlaybookDirName, "bin", "install"), "#!/bin/sh\n: > "+marker+"\n")
	chdir(t, projectDir)

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
	if _, err := os.Stat(marker); err == nil {
		t.Error("der Bootstrap ist ohne Bestätigung gelaufen")
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
