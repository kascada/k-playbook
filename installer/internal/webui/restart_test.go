package webui

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kascada/k-playbook/installer/internal/guiproc"
	"github.com/kascada/k-playbook/installer/internal/program"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// Die Tests hier bauen echte Programme mit gestempelter Version und starten
// sie als abgekoppelte Dienste — in eigenem HOME und eigenem
// Laufzeitverzeichnis (isolateHome). Das echte ~/.local/bin/k-playbook und die
// Laufzeitdateien paralleler Sitzungen berührt keiner davon.

// Umgebung und Werkzeug für den Bau werden beim Laden des Pakets
// festgehalten, bevor ein Test HOME und PATH umstellt: mit dem Test-HOME fände
// go weder Build-Cache noch Modul-Cache.
var (
	buildEnviron  = os.Environ()
	goBinary, _   = exec.LookPath("go")
	buildMu       sync.Mutex
	buildDir      string
	builtPrograms = map[string]string{}
)

func TestMain(m *testing.M) {
	code := m.Run()
	if buildDir != "" {
		_ = os.RemoveAll(buildDir)
	}
	os.Exit(code)
}

// buildProgram baut k-playbook mit der Version version über -ldflags, wie
// make und der Release-Workflow es tun, einmal je Version und Testlauf.
func buildProgram(t *testing.T, version string) string {
	t.Helper()

	buildMu.Lock()
	defer buildMu.Unlock()
	if path, ok := builtPrograms[version]; ok {
		return path
	}
	if goBinary == "" {
		t.Skip("go nicht verfügbar, das Programm lässt sich nicht bauen")
	}
	if buildDir == "" {
		dir, err := os.MkdirTemp("", "k-playbook-076-")
		if err != nil {
			t.Fatal(err)
		}
		buildDir = dir
	}
	output := filepath.Join(buildDir, "k-playbook-"+version)
	cmd := exec.Command(goBinary, "build", "-trimpath", "-buildvcs=false",
		"-ldflags", "-X github.com/kascada/k-playbook/installer/internal/buildinfo.Version="+version,
		"-o", output, "github.com/kascada/k-playbook/installer/cmd/k-playbook")
	cmd.Env = buildEnviron
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Programm bauen: %v\n%s", err, out)
	}
	builtPrograms[version] = output
	return output
}

// restartFixture ist ein laufender „alter" Dienst — dieser Testprozess mit
// eigener Registrierung — in einem Projekt, dessen Clone einen Stub-Bootstrap
// trägt.
type restartFixture struct {
	projectDir string
	target     string
	state      *serverState
	file       string
	wasStopped func() bool
}

// newRestartFixture baut das Projekt: VERSION clone, SHA256SUMS mit der Summe
// von asset für diese Plattform und bin/install als Stub, der asset ans Ziel
// kopiert und mit exitCode endet. Am Ziel liegt vorher targetScript.
func newRestartFixture(t *testing.T, running string, clone string, asset string, exitCode int, targetScript string) *restartFixture {
	t.Helper()

	target := isolateHome(t)
	writeExecutable(t, target, targetScript)

	projectDir := t.TempDir()
	playbook := filepath.Join(projectDir, project.PlaybookDirName)
	writeFile(t, filepath.Join(projectDir, project.ConfigFileName), "schema_version: 3\n")
	writeFile(t, filepath.Join(playbook, project.VersionFileName), clone+"\n")
	writeFile(t, filepath.Join(playbook, program.SumsFileName), fileSum(t, asset)+"  "+program.AssetName()+"\n")
	// Der Stub ersetzt wie bin/install atomar über mktemp und mv: ein `cp`
	// über die laufende Programmdatei scheitert mit „Text file busy".
	writeExecutable(t, filepath.Join(playbook, "bin", "install"), fmt.Sprintf(
		"#!/bin/sh\n"+
			"echo \"Stub-Bootstrap für %s\"\n"+
			"tmp=\"$(mktemp \"$HOME/.local/bin/.k-playbook.XXXXXX\")\"\n"+
			"cp %q \"$tmp\" || exit 9\n"+
			"chmod 0755 \"$tmp\"\n"+
			"mv -f \"$tmp\" \"$HOME/.local/bin/k-playbook\" || exit 9\n"+
			"exit %d\n", clone, asset, exitCode))
	t.Cleanup(func() { makeWritable(projectDir) })
	chdir(t, projectDir)

	key, err := guiproc.Key()
	if err != nil {
		t.Fatal(err)
	}
	registration, err := guiproc.Register(guiproc.Record{
		Key: key, Addr: "127.0.0.1:1", PID: os.Getpid(), Version: running,
		StartTime: guiproc.OwnStartTime().Unix(),
	})
	if err != nil {
		t.Fatalf("Registrierung: %v", err)
	}
	t.Cleanup(func() { _ = registration.Remove() })
	// Wer immer am Ende registriert ist, wird beendet — vor dem Aufräumen der
	// Verzeichnisse, damit kein abgekoppelter Prozess übrig bleibt.
	t.Cleanup(func() { stopRegistered(t, registration.Path()) })

	shutdown, wasStopped := shutdownProbe()
	return &restartFixture{
		projectDir: projectDir,
		target:     target,
		state:      &serverState{version: running, registration: registration, shutdown: shutdown},
		file:       registration.Path(),
		wasStopped: wasStopped,
	}
}

func fileSum(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// stopRegistered beendet den Dienst aus der Laufzeitdatei, sofern es nicht
// dieser Testprozess ist, und wartet auf sein Ende.
func stopRegistered(t *testing.T, file string) {
	record, ok, err := guiproc.Read(file)
	if err != nil || !ok || record.PID == os.Getpid() {
		return
	}
	_ = guiproc.Terminate(record.PID)
	if !guiproc.WaitForExit(file, record.PID, 5*time.Second) {
		t.Errorf("Dienst PID %d endet nicht", record.PID)
	}
}

func (f *restartFixture) registered(t *testing.T) guiproc.Record {
	t.Helper()
	record, ok, err := guiproc.Read(f.file)
	if err != nil || !ok {
		t.Fatalf("keine Laufzeitdatei: %v", err)
	}
	return record
}

// Der ganze Ablauf: älteres Programm, Stub-Bootstrap legt ein getrennt
// gebautes Programm mit neuer Version ab, der neue Dienst übernimmt die
// Laufzeitdatei, meldet die neue Version und hat die Startpflege erledigt; der
// alte ist nicht mehr registriert und beendet sich.
func TestProgrammaktualisierungStartetNeu(t *testing.T) {
	asset := buildProgram(t, "v0.9.91")
	f := newRestartFixture(t, "v0.9.90", "v0.9.91", asset, 0, versionScript("v0.9.90"))

	code, response := postUpdate(t, f.state, programHandler, "/api/update/program")
	if code != http.StatusOK || !response.Restarted {
		t.Fatalf("Status %d, Antwort %+v", code, response)
	}
	if response.Version != "v0.9.91" || !strings.Contains(response.InstallOutput, "Stub-Bootstrap") {
		t.Errorf("Antwort = %+v", response)
	}

	record := f.registered(t)
	if record.PID == os.Getpid() || record.URL() != response.URL {
		t.Fatalf("registriert ist %+v, Antwort nennt %s", record, response.URL)
	}
	health, err := guiproc.ProbeHealth(record.Addr)
	if err != nil || health.Version != "v0.9.91" || health.PID != record.PID {
		t.Fatalf("Health = %+v, %v", health, err)
	}
	if !f.state.registration.Released() {
		t.Error("der alte Dienst gilt noch als registriert")
	}
	// Das Ende des alten Dienstes darf die Datei des neuen nicht löschen.
	if err := f.state.registration.Remove(); err != nil {
		t.Fatal(err)
	}
	if again := f.registered(t); again.PID != record.PID {
		t.Errorf("nach dem Ende des alten steht %+v", again)
	}
	if !f.wasStopped() {
		t.Error("der alte Dienst beendet sich nicht")
	}
	// Die Startpflege lief im neuen Dienst: sie legt die fehlende
	// projekteigene Struktur an.
	if _, err := os.Stat(filepath.Join(f.projectDir, project.LocalDirName)); err != nil {
		t.Errorf("keine Startpflege im neuen Dienst: %v", err)
	}
	if got := program.ReadVersion(f.target); got != "v0.9.91" {
		t.Errorf("am Ziel liegt %q", got)
	}
}

// Startet der neue Dienst nicht, meldet sich der alte wieder an, läuft weiter
// und nennt den Fehler — ohne Sperrfläche, mit dem Befehl zum Nachholen.
func TestNeuerDienstStartetNichtAlterLaeuftWeiter(t *testing.T) {
	asset := filepath.Join(t.TempDir(), "asset")
	writeExecutable(t, asset, versionScript("v0.9.91"))
	// Am Ziel liegt schon ein Programm mit der VERSION des Clones: kein
	// Download, nur der Neustart — und der scheitert, denn als Server taugt
	// das Skript nicht.
	f := newRestartFixture(t, "v0.9.90", "v0.9.91", asset, 0, versionScript("v0.9.91"))

	code, response := postUpdate(t, f.state, programHandler, "/api/update/program")
	if code != http.StatusInternalServerError || response.Restarted || !response.InstallFailed {
		t.Fatalf("Status %d, Antwort %+v", code, response)
	}
	for _, want := range []string{"nicht gestartet", "läuft mit dem bisherigen Programm weiter", project.BootstrapCommand} {
		if !strings.Contains(response.Message, want) {
			t.Errorf("Meldung ohne %q: %s", want, response.Message)
		}
	}
	if record := f.registered(t); record.PID != os.Getpid() {
		t.Errorf("registriert ist %+v statt des alten Dienstes", record)
	}
	if f.state.registration.Released() {
		t.Error("der alte Dienst hat sich nicht wieder angemeldet")
	}
	if f.wasStopped() {
		t.Error("der alte Dienst hat sich beendet")
	}
}

// Scheitert die Installation, läuft der Dienst weiter; die Antwort trägt
// Fehler, Ausgabe und Befehl. Freigegeben wurde nichts.
func TestInstallationScheitertDienstLaeuftWeiter(t *testing.T) {
	asset := filepath.Join(t.TempDir(), "asset")
	writeExecutable(t, asset, versionScript("v0.9.91"))
	f := newRestartFixture(t, "v0.9.90", "v0.9.91", asset, 1, versionScript("v0.9.90"))

	code, response := postUpdate(t, f.state, programHandler, "/api/update/program")
	if code != http.StatusInternalServerError || !response.InstallFailed || response.Restarted {
		t.Fatalf("Status %d, Antwort %+v", code, response)
	}
	for _, want := range []string{"Exit-Code 1", project.BootstrapCommand, project.BootstrapCommandNoMake} {
		if !strings.Contains(response.Message, want) {
			t.Errorf("Meldung ohne %q: %s", want, response.Message)
		}
	}
	if !strings.Contains(response.InstallOutput, "Stub-Bootstrap") {
		t.Errorf("Ausgabe fehlt: %q", response.InstallOutput)
	}
	if record := f.registered(t); record.PID != os.Getpid() || f.state.registration.Released() {
		t.Errorf("Registrierung angetastet: %+v", record)
	}
	if f.wasStopped() {
		t.Error("der Dienst hat sich beendet")
	}
	// Danach meldet die Prüfung wieder „Programm älter".
	if check := getUpdate(t, f.state); !check.Program.Outdated || !check.Program.Installable {
		t.Errorf("Prüfung danach: %+v", check.Program)
	}
}

// Ein gleichzeitiger Start in der Lücke gewinnt: der neue Dienst weicht aus,
// der alte tritt zurück und nennt der Seite die Adresse des Gewinners. Am Ende
// ist genau einer registriert.
func TestGleichzeitigerStartInDerLueckeGewinnt(t *testing.T) {
	asset := buildProgram(t, "v0.9.91")
	f := newRestartFixture(t, "v0.9.90", "v0.9.91", asset, 0, versionScript("v0.9.90"))

	var competitor *guiproc.Child
	afterRelease = func() {
		child, err := guiproc.Spawn(asset, filepath.Join(t.TempDir(), "gleichzeitig.log"))
		if err != nil {
			t.Errorf("gleichzeitiger Start: %v", err)
			return
		}
		competitor = child
		key, _ := guiproc.Key()
		if _, err := child.Await(key, f.file, 10*time.Second, guiproc.ProbeHealth); err != nil {
			t.Errorf("gleichzeitiger Start antwortet nicht: %v\n%s", err, child.Log())
		}
	}
	t.Cleanup(func() { afterRelease = nil })

	code, response := postUpdate(t, f.state, programHandler, "/api/update/program")
	if competitor == nil {
		t.Fatal("kein gleichzeitiger Start")
	}
	if code != http.StatusOK || !response.Restarted || !strings.Contains(response.Message, "gleichzeitiger Aufruf") {
		t.Fatalf("Status %d, Antwort %+v", code, response)
	}
	record := f.registered(t)
	if record.PID != competitor.Process.Pid || response.URL != record.URL() {
		t.Errorf("registriert ist %+v, Gewinner PID %d, Antwort %s", record, competitor.Process.Pid, response.URL)
	}
	if !f.state.registration.Released() {
		t.Error("der alte Dienst gilt noch als registriert")
	}
}

// Der Weg über „Update verfügbar": der Pull bewegt die VERSION, das laufende
// Programm ist älter — dann installiert und startet derselbe Ablauf neu.
func TestUpdateMitVersionswechselInstalliertUndStartetNeu(t *testing.T) {
	asset := buildProgram(t, "v0.9.91")
	target := isolateHome(t)
	writeExecutable(t, target, versionScript("v0.9.90"))

	projectDir, upstream := gitProject(t, "v0.9.90")
	// Das Ausführungsbit kommt von der Datei: git übernimmt es beim add.
	writeExecutable(t, filepath.Join(upstream, "bin", "install"), "#!/bin/sh\n"+
		"echo \"Stub-Bootstrap\"\n"+
		"tmp=\"$(mktemp \"$HOME/.local/bin/.k-playbook.XXXXXX\")\"\n"+
		"cp "+asset+" \"$tmp\" || exit 9\n"+
		"chmod 0755 \"$tmp\"\n"+
		"mv -f \"$tmp\" \"$HOME/.local/bin/k-playbook\" || exit 9\n")
	gitCmd(t, upstream, "add", "-A")
	gitCmd(t, upstream, "commit", "-m", "Stub-Bootstrap")
	gitCmd(t, upstream, "push")
	pushChange(t, upstream, program.SumsFileName, fileSum(t, asset)+"  "+program.AssetName()+"\n")
	pushChange(t, upstream, project.VersionFileName, "v0.9.91\n")
	chdir(t, projectDir)

	key, err := guiproc.Key()
	if err != nil {
		t.Fatal(err)
	}
	registration, err := guiproc.Register(guiproc.Record{
		Key: key, Addr: "127.0.0.1:1", PID: os.Getpid(), Version: "v0.9.90",
		StartTime: guiproc.OwnStartTime().Unix(),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = registration.Remove() })
	t.Cleanup(func() { stopRegistered(t, registration.Path()) })

	shutdown, wasStopped := shutdownProbe()
	state := &serverState{version: "v0.9.90", registration: registration, shutdown: shutdown}
	code, response := postUpdate(t, state, pullHandler, "/api/update")
	if code != http.StatusOK || !response.Restarted || response.Version != "v0.9.91" {
		t.Fatalf("Status %d, Antwort %+v", code, response)
	}
	record, ok, err := guiproc.Read(registration.Path())
	if err != nil || !ok || record.Version != "v0.9.91" || record.URL() != response.URL {
		t.Fatalf("Laufzeitdatei = %+v (%v, %v)", record, ok, err)
	}
	if !wasStopped() {
		t.Error("der alte Dienst beendet sich nicht")
	}
}
