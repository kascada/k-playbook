package webui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
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

// updateProject baut ein Projekt — nur die Konfiguration — und stellt eine Ersatzquelle hin, die latest als neuesten
// Release-Tag nennt, ohne Assets; wer eines braucht, legt es mit
// releasetest.Serve selbst an.
func updateProject(t *testing.T, latest string) string {
	t.Helper()

	releasetest.Serve(t).AddTag(latest)
	projectDir := t.TempDir()
	writeFile(t, project.ConfigPath(projectDir), "schema_version: 3\n")
	return projectDir
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

// Nur „älter" meldet. Gleich, neuer und unbekannt melden nichts: verglichen
// wird mit dem neuesten Release-Tag.
func TestUpdatePruefungMeldetAelteresProgramm(t *testing.T) {
	target := isolateHome(t)
	writeExecutable(t, target, versionScript("v0.9.3"))
	projectDir := updateProject(t, "v0.9.4")
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
	projectDir := updateProject(t, "v0.9.4")
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

// Zeigt `k-playbook` im PATH auf ein anderes Programm oder auf keins, gibt es
// keinen Knopf, sondern einen Hinweis mit beiden Pfaden.
func TestUpdatePruefungOhneKnopfBeiFalschemPath(t *testing.T) {
	t.Run("anderes Programm", func(t *testing.T) {
		target := isolateHome(t)
		writeExecutable(t, target, versionScript("v0.9.3"))
		other := filepath.Join(t.TempDir(), project.InstalledCommandName)
		writeExecutable(t, other, versionScript("v0.9.3"))
		t.Setenv("PATH", filepath.Dir(other)+string(os.PathListSeparator)+os.Getenv("PATH"))
		projectDir := updateProject(t, "v0.9.4")
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
		projectDir := updateProject(t, "v0.9.4")
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

// Ein laufendes Programm über dem neuesten Tag — ein Build mit
// `make dev-install VERSION=…` — gilt als aktuell: keine Installation, der
// Dienst läuft weiter, und die Programmaktualisierung lehnt mit 409 ab.
func TestProgrammaktualisierungBeiNeueremProgrammInstalliertNicht(t *testing.T) {
	target := isolateHome(t)
	writeExecutable(t, target, versionScript("v0.9.3"))
	projectDir := updateProject(t, "v0.9.4")
	chdir(t, projectDir)
	source := releasetest.Serve(t)
	source.AddRelease("v0.9.4", []byte(versionScript("v0.9.4")))

	shutdown, wasShutdown := shutdownProbe()
	code, response := postUpdate(t, &serverState{version: "v0.10.0", shutdown: shutdown}, programHandler, "/api/update/program")
	if code != http.StatusConflict || response.Restarted || response.InstallFailed || !strings.Contains(response.Message, "nicht älter") {
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

// Älteres Programm, aber der PATH zeigt woandershin: Die
// Programmaktualisierung lehnt mit 409 ab, der Dienst läuft weiter, Hinweis
// statt Neustart.
func TestProgrammaktualisierungBeiFalschemPath(t *testing.T) {
	target := isolateHome(t)
	writeExecutable(t, target, versionScript("v0.9.4"))
	other := filepath.Join(t.TempDir(), project.InstalledCommandName)
	writeExecutable(t, other, versionScript("v0.9.4"))
	t.Setenv("PATH", filepath.Dir(other)+string(os.PathListSeparator)+os.Getenv("PATH"))
	projectDir := updateProject(t, "v0.9.5")
	chdir(t, projectDir)

	shutdown, wasShutdown := shutdownProbe()
	code, response := postUpdate(t, &serverState{version: "v0.9.4", shutdown: shutdown}, programHandler, "/api/update/program")
	if code != http.StatusConflict || response.Restarted {
		t.Fatalf("Status %d, Antwort %+v", code, response)
	}
	for _, want := range []string{other, target, project.FirstInstallCommand} {
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

// Während eines Ablaufs bekommt ein zweiter Aufruf 409.
func TestZweiterAufrufWaehrendDesAblaufsIst409(t *testing.T) {
	isolateHome(t)
	projectDir := updateProject(t, "v0.9.4")
	chdir(t, projectDir)

	state := &serverState{version: "v0.9.3"}
	state.updateMu.Lock()
	defer state.updateMu.Unlock()

	code, response := postUpdate(t, state, programHandler, "/api/update/program")
	if code != http.StatusConflict || response.Message != updateBusyMessage {
		t.Errorf("Status %d, Meldung %q", code, response.Message)
	}
}

// Laufen Befehle oder Chats, fragt der Knopf vor dem Neustart nach: ohne
// Bestätigung 428 mit der Rückfrage, und installiert wird nichts.
func TestProgrammaktualisierungFragtBeiLaufenderArbeit(t *testing.T) {
	target := isolateHome(t)
	writeExecutable(t, target, versionScript("v0.9.3"))
	projectDir := updateProject(t, "v0.9.4")
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
	projectDir := updateProject(t, "v0.9.4")
	chdir(t, projectDir)

	code, response := postUpdate(t, &serverState{version: "v0.9.4"}, programHandler, "/api/update/program")
	if code != http.StatusConflict || !strings.Contains(response.Message, "nicht älter") {
		t.Errorf("Status %d, Meldung %q", code, response.Message)
	}
}

// GET /api/update ist ein Hinweis beim Laden der Seite und nimmt den neuesten
// Tag aus dem Cache; mit fresh=1 — dem Knopf „Update prüfen“ — fragt der
// Dienst frisch und schreibt den Cache. Sichtbar an der Ersatzquelle. Kein
// Programm im PATH: installiert wird nichts.
func TestUpdatePruefungLiestDenCacheUndFragtAufWunschFrisch(t *testing.T) {
	isolateHome(t)
	projectDir := updateProject(t, "v0.9.4")
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
