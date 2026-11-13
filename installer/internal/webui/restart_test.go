package webui

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kascada/k-playbook/installer/internal/githubaccess/accesstest"
	"github.com/kascada/k-playbook/installer/internal/guiproc"
	"github.com/kascada/k-playbook/installer/internal/kephalaion/kephtest"
	"github.com/kascada/k-playbook/installer/internal/program"
	"github.com/kascada/k-playbook/installer/internal/project"
	"github.com/kascada/k-playbook/installer/internal/project/defaultstest"
	"github.com/kascada/k-playbook/installer/internal/release/releasetest"
	"github.com/kascada/k-playbook/installer/internal/sharedir/sharetest"
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

// packageDir ist das Verzeichnis dieses Pakets, festgehalten in TestMain,
// bevor die Tests das Arbeitsverzeichnis verlassen. Alles, was relativ zum
// Modul liegt — die Fixtures des Inventars, der Bau des Programms —, geht
// von hier aus, nicht vom Arbeitsverzeichnis.
var packageDir string

func TestMain(m *testing.M) {
	// Kein Test liest das ~/.local/share/k-playbook dieses Rechners; wer
	// mitgelieferte Commands braucht, legt sie mit sharetest.Dir an. Und
	// keiner die persönliche GitHub-Zugangsdatei: /api/environment erreicht
	// deren Leser, der sonst am echten Ort die Vorlage anlegte; ein
	// gestarteter Dienst erbt den Override.
	cleanupShare, err := sharetest.Isolate()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	cleanupGitHub, err := accesstest.Isolate()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	// Kein Test läuft im Verzeichnis dieses Pakets. Die Handler suchen das
	// Projekt ab dem Arbeitsverzeichnis aufwärts (project.Detect), und das
	// Paketverzeichnis liegt in einem echten k-playbook-Projekt — dem
	// Quell-Repo. Ein Test ohne eigenes chdir fände dessen Anker und zöge
	// ihn um (Discover migriert vor jeder Prüfung). Deshalb läuft das Paket
	// in einem leeren Temp-Verzeichnis, in dem die Suche nichts findet.
	packageDir, err = os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	workDir, err := os.MkdirTemp("", "k-playbook-webui-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.Chdir(workDir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	cleanupDefaults := defaultstest.Isolate()
	// Kein Test fragt das echte kephalaion dieses Rechners (Task 138).
	cleanupBinary := kephtest.IsolateBinary()
	// Und keiner das echte GitHub nach dem neuesten Release-Tag oder einem
	// Release-Binary (Task 149). Die Tests hier setzen eigene Versionen und
	// starten gebaute Programme als Dienst; wer die Quelle braucht, stellt mit
	// releasetest.Serve eine Ersatzquelle hin. Ein gestarteter Dienst erbt die
	// Umgebung — abgeschaltet oder die Ersatzquelle.
	cleanupRelease := releasetest.Isolate()
	code := m.Run()
	cleanupRelease()
	cleanupBinary()
	cleanupDefaults()
	_ = os.Chdir(packageDir)
	_ = os.RemoveAll(workDir)
	cleanupGitHub()
	cleanupShare()
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
	// Im Paketverzeichnis, nicht im Arbeitsverzeichnis der Tests (TestMain):
	// außerhalb des Moduls kennt go build das Paket nicht.
	cmd.Dir = packageDir
	cmd.Env = buildEnviron
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Programm bauen: %v\n%s", err, out)
	}
	builtPrograms[version] = output
	return output
}

// restartFixture ist ein laufender „alter" Dienst — dieser Testprozess mit
// eigener Registrierung — in einem Projekt ohne Clone, vor einer Ersatzquelle
// für das Release.
type restartFixture struct {
	projectDir string
	target     string
	state      *serverState
	file       string
	wasStopped func() bool
	source     *releasetest.Source
}

// newRestartFixture baut das Projekt — bewusst ohne Clone: Programmprüfung und
// Installation hängen nicht an ihm — und die Ersatzquelle: latest ist der
// neueste Release-Tag, asset das Release-Binary dieser Plattform samt passender
// SHA256SUMS. Mit assetMissing fehlt das Asset (der Tag steht, die CI baut
// noch): die Installation scheitert. Am Ziel liegt vorher targetScript.
func newRestartFixture(t *testing.T, running string, latest string, asset string, assetMissing bool, targetScript string) *restartFixture {
	t.Helper()

	target := isolateHome(t)
	writeExecutable(t, target, targetScript)

	projectDir := t.TempDir()
	writeFile(t, project.ConfigPath(projectDir), "schema_version: 3\n")
	source := serveRelease(t, latest, asset)
	if assetMissing {
		source.RemoveAsset(latest, program.AssetName())
	}
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
		source:     source,
	}
}

// serveRelease stellt eine Ersatzquelle hin, deren neuester Tag tag ist und
// die die Datei asset als Release-Binary dieser Plattform ausliefert.
func serveRelease(t *testing.T, tag string, asset string) *releasetest.Source {
	t.Helper()
	content, err := os.ReadFile(asset)
	if err != nil {
		t.Fatal(err)
	}
	source := releasetest.Serve(t)
	source.AddRelease(tag, content)
	return source
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

// Der ganze Ablauf, ohne Clone: älteres Programm, die Installation lädt ein
// getrennt gebautes Programm mit neuer Version aus dem Release des neuesten
// Tags und prüft es gegen dessen SHA256SUMS, der neue Dienst übernimmt die
// Laufzeitdatei, meldet die neue Version und hat die Startpflege erledigt; der
// alte ist nicht mehr registriert und beendet sich.
func TestProgrammaktualisierungStartetNeu(t *testing.T) {
	asset := buildProgram(t, "v0.9.91")
	f := newRestartFixture(t, "v0.9.90", "v0.9.91", asset, false, versionScript("v0.9.90"))

	code, response := postUpdate(t, f.state, programHandler, "/api/update/program")
	if code != http.StatusOK || !response.Restarted {
		t.Fatalf("Status %d, Antwort %+v", code, response)
	}
	if response.Version != "v0.9.91" || !strings.Contains(response.InstallOutput, "Geprüft gegen SHA256SUMS von v0.9.91") ||
		!strings.Contains(response.InstallOutput, f.source.URL()+"/download/v0.9.91/"+program.AssetName()) {
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
	// Die Startpflege lief im neuen Dienst: sie legt die fehlende AGENTS.md
	// aus der Vorlage an. k-playbook-local/ legt sie nicht an (Task 147).
	if _, err := os.Stat(filepath.Join(f.projectDir, project.RootInstructionsFile)); err != nil {
		t.Errorf("keine Startpflege im neuen Dienst: %v", err)
	}
	if _, err := os.Stat(filepath.Join(f.projectDir, project.LocalDirName)); !os.IsNotExist(err) {
		t.Errorf("die Startpflege hat %s/ angelegt (Stat: %v)", project.LocalDirName, err)
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
	// Am Ziel liegt schon ein Programm mit der Version des neuesten Tags: kein
	// Download, nur der Neustart — und der scheitert, denn als Server taugt
	// das Skript nicht.
	f := newRestartFixture(t, "v0.9.90", "v0.9.91", asset, false, versionScript("v0.9.91"))

	code, response := postUpdate(t, f.state, programHandler, "/api/update/program")
	if code != http.StatusInternalServerError || response.Restarted || !response.InstallFailed {
		t.Fatalf("Status %d, Antwort %+v", code, response)
	}
	for _, want := range []string{"nicht gestartet", "läuft mit dem bisherigen Programm weiter", project.UpgradeCommand} {
		if !strings.Contains(response.Message, want) {
			t.Errorf("Meldung ohne %q: %s", want, response.Message)
		}
	}
	for _, request := range f.source.Requests() {
		if strings.HasPrefix(request, "/download/") {
			t.Errorf("trotz passender Datei am Ziel wurde geladen: %s", request)
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

// Scheitert die Installation — hier fehlt das Asset noch, der Tag steht schon —,
// läuft der Dienst weiter; die Antwort trägt Fehler und Befehl. Freigegeben
// und ersetzt wurde nichts.
func TestInstallationScheitertDienstLaeuftWeiter(t *testing.T) {
	asset := filepath.Join(t.TempDir(), "asset")
	writeExecutable(t, asset, versionScript("v0.9.91"))
	f := newRestartFixture(t, "v0.9.90", "v0.9.91", asset, true, versionScript("v0.9.90"))

	code, response := postUpdate(t, f.state, programHandler, "/api/update/program")
	if code != http.StatusInternalServerError || !response.InstallFailed || response.Restarted {
		t.Fatalf("Status %d, Antwort %+v", code, response)
	}
	for _, want := range []string{"das Release v0.9.91 ist noch nicht fertig", "Nachholen im Terminal: " + project.UpgradeCommand} {
		if !strings.Contains(response.Message, want) {
			t.Errorf("Meldung ohne %q: %s", want, response.Message)
		}
	}
	if strings.Contains(response.Message, project.BootstrapCommand) {
		t.Errorf("die Meldung nennt noch den Bootstrap des Clones: %s", response.Message)
	}
	if got := program.ReadVersion(f.target); got != "v0.9.90" {
		t.Errorf("am Ziel liegt %q, die Datei wurde ersetzt", got)
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
	f := newRestartFixture(t, "v0.9.90", "v0.9.91", asset, false, versionScript("v0.9.90"))

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

// Der Weg über „Update verfügbar": Ist das laufende Programm älter als der
// neueste Release-Tag, installiert und startet derselbe Ablauf neu. Knopf und
// Neustart hängen am Tag, nicht am Pull: Es geht nach einem Pull, der die
// VERSION im Clone gar nicht bewegt, und ebenso nach einem gescheiterten Pull.
func TestUpdateInstalliertUndStartetNeuBeiAelteremProgramm(t *testing.T) {
	asset := buildProgram(t, "v0.9.91")

	for _, fall := range []struct {
		name       string
		breakPull  bool
		pullFailed string
	}{
		{name: "nach gelungenem Pull"},
		{name: "nach gescheitertem Pull", breakPull: true, pullFailed: "git pull --ff-only fehlgeschlagen"},
	} {
		t.Run(fall.name, func(t *testing.T) {
			target := isolateHome(t)
			writeExecutable(t, target, versionScript("v0.9.90"))

			projectDir, upstream := gitProject(t, "v0.9.91")
			pushChange(t, upstream, filepath.Join("commands", "k-neu.md"), "neu\n")
			if fall.breakPull {
				gitCmd(t, filepath.Join(projectDir, project.PlaybookDirName), "remote", "set-url", "origin", filepath.Join(t.TempDir(), "gibt-es-nicht.git"))
			}
			serveRelease(t, "v0.9.91", asset)
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
			// Auch nach einem gescheiterten Pull 200: die Seite muss der neuen
			// Adresse folgen.
			if code != http.StatusOK || !response.Restarted || response.Version != "v0.9.91" {
				t.Fatalf("Status %d, Antwort %+v", code, response)
			}
			if fall.pullFailed != "" && !strings.Contains(response.Message, fall.pullFailed) {
				t.Errorf("der gescheiterte Pull wird nicht gemeldet: %s", response.Message)
			}
			_, pullErr := os.Stat(filepath.Join(projectDir, project.PlaybookDirName, "commands", "k-neu.md"))
			if (pullErr == nil) == fall.breakPull {
				t.Errorf("Pull angekommen = %v, erwartet %v", pullErr == nil, !fall.breakPull)
			}
			record, ok, err := guiproc.Read(registration.Path())
			if err != nil || !ok || record.Version != "v0.9.91" || record.URL() != response.URL {
				t.Fatalf("Laufzeitdatei = %+v (%v, %v)", record, ok, err)
			}
			if got := program.ReadVersion(target); got != "v0.9.91" {
				t.Errorf("am Ziel liegt %q", got)
			}
			if !wasStopped() {
				t.Error("der alte Dienst beendet sich nicht")
			}
		})
	}
}
