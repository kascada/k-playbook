package webui

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/kascada/k-playbook/installer/internal/guiproc"
	"github.com/kascada/k-playbook/installer/internal/program"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// Programmaktualisierung und Neustart.
//
// Ist das laufende Programm älter als die VERSION des Clones, installiert der
// Dienst auf Knopfdruck das passende Release-Programm über den Bootstrap des
// Clones und startet sich daraus neu. Das Herunterladen liegt ganz bei
// k-playbook/bin/install; hier wird es nur angestoßen und danach an der Datei
// geprüft (program.Install).
//
// Die Übergabe der Laufzeitdatei hält eine Invariante: **nie zwei registrierte
// Dienste für ein Projekt.** Die Datei entsteht mit O_CREAT|O_EXCL, ein neuer
// Dienst weicht einer vorhandenen aus — „erst starten, dann übergeben" geht
// deshalb nicht. Der Ablauf:
//
//  1. Der alte Dienst gibt seine Registrierung frei (Registration.Release).
//  2. Er startet den neuen abgekoppelt aus dem Installationsziel, im selben
//     Arbeitsverzeichnis, mit der Marke für die Startpflege.
//  3. Er wartet, bis die Laufzeitdatei die PID des neuen trägt und
//     /api/health mit demselben Schlüssel, derselben PID und der erwarteten
//     Version antwortet.
//  4. Gelingt das, antwortet er der Seite mit der neuen Adresse und beendet
//     sich. Gelingt es nicht, holt er seine Registrierung zurück
//     (Registration.Reclaim), läuft weiter und nennt den Fehler.
//
// Ein gleichzeitiger `k-playbook`-Start in der Lücke darf gewinnen. Dann weicht
// der neue Dienst aus, Reclaim scheitert an der vorhandenen Datei, und der
// alte Dienst tritt zurück: er antwortet der Seite mit der Adresse des
// Gewinners und beendet sich. Am Ende ist genau ein Dienst registriert.

// restartStartupTimeout ist die Wartegrenze auf den neuen Dienst. Sie liegt
// über guiproc.StartupTimeout, weil der neue Dienst vor dem Binden noch die
// Startpflege erledigt, die sonst der argumentlose Aufruf übernimmt. Eine
// Variable, damit Tests den Weg über die Frist gehen können.
var restartStartupTimeout = 2 * guiproc.StartupTimeout

// afterRelease läuft in Tests zwischen Freigabe und Start des neuen Dienstes:
// dort wird der gleichzeitige Start in der Lücke nachgestellt.
var afterRelease func()

// restartOutcome ist der Ausgang von Installation und Neustart.
type restartOutcome struct {
	Restarted bool
	URL       string
	Version   string
	Failed    bool
	Output    string
	Message   string
}

func (o restartOutcome) applyTo(response *updateResponse) {
	response.Restarted = o.Restarted
	response.URL = o.URL
	response.Version = o.Version
	response.InstallFailed = o.Failed
	response.InstallOutput = o.Output
	if o.Message != "" {
		response.Message = strings.TrimSpace(response.Message + " " + o.Message)
	}
}

// applyProgramHandler installiert das zum Clone passende Programm und startet
// den Dienst daraus neu. Er läuft nur auf ausdrücklichen Klick.
//
// Abgelehnt wird mit 409, wenn schon ein Ablauf läuft, wenn das laufende
// Programm nicht älter ist als der Clone oder wenn der PATH nicht auf das
// Installationsziel zeigt. Laufen Befehle oder Chats, antwortet er ohne
// Bestätigung (?confirm=1) mit 428 und der Rückfrage; die Seite stellt sie und
// schickt den Aufruf bestätigt erneut. Scheitert die Installation oder der
// Neustart, läuft der Dienst weiter, und die Antwort trägt Fehler, Ausgabe und
// den Befehl zum Nachholen.
func (state *serverState) applyProgramHandler(w http.ResponseWriter, r *http.Request) {
	environment := project.Detect()
	if !environment.Installed {
		writeJSON(w, http.StatusConflict, updateResponse{Message: "Keine " + project.ConfigFileName + " gefunden."})
		return
	}
	if !state.updateMu.TryLock() {
		writeJSON(w, http.StatusConflict, updateResponse{Message: updateBusyMessage})
		return
	}
	defer state.updateMu.Unlock()

	status := checkProgram(state.version, environment.PlaybookDir)
	response := updateResponse{Program: status}
	switch {
	case !status.Outdated:
		response.Message = fmt.Sprintf("Das laufende Programm (%s) ist nicht älter als die Installation (%s); es gibt nichts zu aktualisieren.",
			displayVersion(status.Running), displayVersion(status.Clone))
		writeJSON(w, http.StatusConflict, response)
		return
	case !status.Installable:
		response.Message = status.Hint
		writeJSON(w, http.StatusConflict, response)
		return
	}
	if activity := state.activeWork(); activity != "" && !confirmed(r) {
		response.Busy = true
		response.Message = activity + " Der Neustart trennt sie vom Dienst; den Ausgang eines laufenden Commands meldet danach niemand mehr. Trotzdem jetzt aktualisieren und neu starten?"
		writeJSON(w, http.StatusPreconditionRequired, response)
		return
	}

	outcome := state.installAndRestart(environment.ProjectDir)
	outcome.applyTo(&response)
	if !outcome.Restarted {
		writeJSON(w, http.StatusInternalServerError, response)
		return
	}
	writeJSON(w, http.StatusOK, response)
	state.shutdownAfterResponse()
}

// confirmed meldet, ob der Aufruf die Rückfrage schon bestätigt hat.
func confirmed(r *http.Request) bool {
	return r.URL.Query().Get("confirm") == "1"
}

// activeWork beschreibt, was ein Neustart unterbräche. Leer, wenn nichts
// läuft.
func (state *serverState) activeWork() string {
	commands := state.runningCommands()
	streams := int(state.openStreams.Load())
	parts := []string{}
	switch {
	case commands == 1:
		parts = append(parts, "ein Command läuft")
	case commands > 1:
		parts = append(parts, fmt.Sprintf("%d Commands laufen", commands))
	}
	switch {
	case streams == 1:
		parts = append(parts, "eine Chat-Ansicht ist verbunden")
	case streams > 1:
		parts = append(parts, fmt.Sprintf("%d Chat-Ansichten sind verbunden", streams))
	}
	if len(parts) == 0 {
		return ""
	}
	text := strings.Join(parts, ", und ")
	return "Noch in Arbeit: " + text + "."
}

// installAndRestart ist der gemeinsame Ablauf beider POSTs: Programm
// installieren, falls nötig, dann neu starten. Der Leerlaufwächter ruht
// solange.
func (state *serverState) installAndRestart(projectDir string) restartOutcome {
	state.restarting.Store(true)
	defer func() {
		state.noteRequest(time.Now())
		state.restarting.Store(false)
	}()

	result, err := program.Install(projectDir)
	if err != nil {
		return restartOutcome{
			Failed:  true,
			Output:  result.Output,
			Message: installFailureMessage(err),
		}
	}
	outcome := state.restartFrom(result.Target, result.Expected)
	if outcome.Output == "" {
		outcome.Output = result.Output
	}
	return outcome
}

// installFailureMessage nennt den Fehler und den Weg im Terminal. Der Dienst
// läuft in diesem Fall weiter; danach meldet die Prüfung wieder „Programm
// älter".
func installFailureMessage(err error) string {
	return "Programm nicht aktualisiert: " + strings.TrimSuffix(err.Error(), ".") + ". " +
		"Der Dienst läuft mit dem bisherigen Programm weiter. Nachholen im Terminal: " +
		project.BootstrapHint + ", danach k-playbook aufrufen."
}

// restartFrom übergibt die Laufzeitdatei an einen neuen Dienst aus target und
// wartet auf ihn. expected ist die Version, die er melden muss.
func (state *serverState) restartFrom(target string, expected string) restartOutcome {
	if state.registration == nil {
		return restartOutcome{Failed: true, Message: installFailureMessage(errors.New("der Dienst hat keine Laufzeitdatei, die er übergeben könnte"))}
	}
	key, err := guiproc.Key()
	if err != nil {
		return restartOutcome{Failed: true, Message: installFailureMessage(err)}
	}
	location, err := guiproc.Locate(key)
	if err != nil {
		return restartOutcome{Failed: true, Message: installFailureMessage(err)}
	}

	if err := state.registration.Release(); err != nil {
		return restartOutcome{Failed: true, Message: installFailureMessage(fmt.Errorf("Laufzeitdatei nicht freigegeben: %w", err))}
	}
	if afterRelease != nil {
		afterRelease()
	}

	child, err := guiproc.Spawn(target, location.Log, guiproc.HostCareEnv+"=1")
	if err != nil {
		return state.takeBack(key, err, "")
	}

	var mismatch string
	probe := func(addr string) (guiproc.Health, error) {
		health, err := guiproc.ProbeHealth(addr)
		if err == nil && expected != "" && health.Version != expected {
			mismatch = health.Version
			return health, fmt.Errorf("Version %q statt %q", health.Version, expected)
		}
		return health, err
	}
	record, err := child.Await(key, location.File, restartStartupTimeout, probe)
	if err == nil {
		return restartOutcome{
			Restarted: true,
			URL:       record.URL(),
			Version:   expected,
			Message:   fmt.Sprintf("Programm aktualisiert; der Dienst läuft jetzt mit %s unter neuer Adresse.", displayVersion(expected)),
		}
	}
	if !errors.Is(err, guiproc.ErrChildExited) {
		// Er lebt, antwortet aber nicht oder mit fremder Version. Er darf
		// nicht stehen bleiben: seine Laufzeitdatei, falls er sie schon hat,
		// verschwindet mit ihm, und erst danach kann der alte sich wieder
		// anmelden.
		child.Terminate()
		waitForChild(child, guiproc.ExitTimeout)
		if mismatch != "" {
			err = fmt.Errorf("%w: er meldet Version %q statt %q", err, mismatch, expected)
		}
	}
	return state.takeBack(key, err, child.Log())
}

// waitForChild wartet auf das Ende eines beendeten Kindes, höchstens timeout
// lang, und tötet es danach.
func waitForChild(child *guiproc.Child, timeout time.Duration) {
	select {
	case <-child.Exited:
	case <-time.After(timeout):
		_ = child.Process.Kill()
		<-child.Exited
	}
}

// takeBack ist der Rückweg, wenn der neue Dienst nicht übernommen hat: der
// alte meldet sich wieder an und läuft weiter. Hat inzwischen ein
// gleichzeitiger Start die Laufzeitdatei geschrieben, tritt er zurück und
// nennt der Seite dessen Adresse — so ist am Ende genau einer registriert.
func (state *serverState) takeBack(key string, cause error, log string) restartOutcome {
	failed := func(reason string) restartOutcome {
		return restartOutcome{Failed: true, Output: log, Message: installFailureMessage(errors.New(reason))}
	}

	// Mehr als ein Versuch, weil zwischen dem gescheiterten Reclaim und dem
	// Blick auf die Datei der andere Start schon wieder weg sein kann.
	for attempt := 0; attempt < 3; attempt++ {
		err := state.registration.Reclaim()
		if err == nil {
			return failed(fmt.Sprintf("der neue Dienst ist nicht gestartet (%v)", cause))
		}
		if !errors.Is(err, fs.ErrExist) {
			return failed(fmt.Sprintf("der neue Dienst ist nicht gestartet (%v), und die Laufzeitdatei ließ sich nicht zurückschreiben (%v); "+
				"dieser Dienst ist für k-playbook bis zum Neustart nicht auffindbar", cause, err))
		}

		finding, err := guiproc.Inspect(key, guiproc.Identity{}, guiproc.DefaultInspector())
		if err != nil {
			return failed(fmt.Sprintf("der neue Dienst ist nicht gestartet (%v), und die Laufzeitdatei eines anderen Starts ist nicht lesbar (%v)", cause, err))
		}
		switch finding.Status {
		case guiproc.StatusRunning, guiproc.StatusOtherVersion:
			return restartOutcome{
				Restarted: true,
				URL:       finding.Record.URL(),
				Version:   finding.Health.Version,
				Message:   "Ein gleichzeitiger Aufruf von k-playbook hat den Dienst gestartet; dieser tritt zurück.",
			}
		case guiproc.StatusUnresponsive:
			// Registriert ist er; ein zweiter daneben bräche die Invariante.
			// Die Seite kommt dort vermutlich nicht hin und sagt das.
			return restartOutcome{
				Restarted: true,
				URL:       finding.Record.URL(),
				Message:   fmt.Sprintf("Ein gleichzeitiger Aufruf von k-playbook hat einen Dienst gestartet (PID %d), der nicht antwortet; dieser tritt trotzdem zurück. Beenden mit: k-playbook stop", finding.Record.PID),
			}
		case guiproc.StatusOrphaned:
			_ = guiproc.Remove(finding.Path)
		}
	}
	return failed(fmt.Sprintf("der neue Dienst ist nicht gestartet (%v), und die Laufzeitdatei ließ sich nicht zurückschreiben", cause))
}
