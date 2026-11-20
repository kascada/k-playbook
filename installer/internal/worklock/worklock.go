// Package worklock ist die Sperre des Arbeitsbaums eines Projekts (Task 171).
//
// Zu einer Zeit ändert höchstens ein Lauf von /k-task-run den Arbeitsbaum
// eines Projekts. Die Sperre vergibt und hält dieses Paket, nicht der
// Assistent: Was hier steht, gilt — auch gegen das, was eine Sitzung von sich
// glaubt. Gesperrt wird je Projekt (das Verzeichnis mit .k-playbook/); ein
// Lauf in einem anderen Projekt wartet nicht.
//
// Wer die Sperre nimmt, bekommt eine Marke. Sie ist für die Sperre, was
// since_revision für ein Task-Dokument ist: Ihre Zählnummer steigt bei jedem
// Nehmen, auch wenn dieselbe Sitzung nach einem Verlust neu nimmt, und ein
// Zufallsteil sorgt dafür, dass eine alte Marke auch dann nicht zu einer
// neueren Sperre passt, wenn die Datei zwischendurch fehlte und der Zähler
// von vorn begann. Erneutes Nehmen (reentrant), Prüfen und Freigeben hängen
// allein an der Marke, nie an einer Sitzungskennung oder einem Prozess.
//
// Der Halter-Prozess entscheidet nur, ob eine Sperre als verwaist gilt — und
// sicher verwaist ist sie nur, wenn sein Ende das Ende der Sitzung belegt
// (holder.go).
//
// Format und Ort sind auf die Belegungen späterer Arbeit ausgelegt: Gegenstand
// (subject), Art (kind) und Wozu (purpose) stehen schon in der Datei, auch
// wenn heute nur der Gegenstand „arbeitsbaum“ der Art „k-task-run“ vorkommt.
package worklock

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// projectDirName ist das Verzeichnis von k-playbook im Projekt,
// project.ProjectDirName. Hier als eigene Konstante, damit das Paket ohne den
// Projektaufbau auskommt; ein Test hält beide gleich.
const projectDirName = ".k-playbook"

// lockDirRel ist der Ort der Sperrdateien, relativ zur Projektwurzel.
//
// Er liegt im Arbeitsbaum, damit jede Umgebung, die denselben Arbeitsbaum
// sieht — Host und Devcontainer über ein Mount, jeder Worktree für sich —,
// dieselbe Sperre sieht, auch ohne Git. Ein Verzeichnis unter dem Home sähe
// nur dieser Benutzer auf diesem Rechner, das Git-Verzeichnis gäbe es ohne
// Git nicht und teilen sich mehrere Worktrees. Aus der Versionskontrolle hält
// sich das Verzeichnis selbst heraus (ignoreFile), damit kein `git add -A`
// — Snapshot, Auto-Commit — die Sperre je erfasst; .k-playbook/.gitignore
// des Projekts bleibt unberührt.
var lockDirRel = filepath.Join(projectDirName, "work", "lock")

// ignoreFile nimmt das Verzeichnis samt sich selbst aus Git heraus.
const (
	ignoreFileName    = ".gitignore"
	ignoreFileContent = "# Von k-playbook angelegt: Sperren des Arbeitsbaums, nie einchecken.\n*\n"
)

// Gegenstand und Art der einzigen Sperre, die es heute gibt.
const (
	SubjectWorktree = "arbeitsbaum"
	KindTaskRun     = "k-task-run"
)

// formatVersion ist die Fassung des Dateiformats.
const formatVersion = 1

// historyLimit begrenzt die gemerkten Enden früherer Marken. Sie reichen, um
// einer Sitzung, die ihre Sperre verloren hat, den Grund zu nennen.
const historyLimit = 20

// Die Zustände einer Antwort.
const (
	// StateAcquired: genommen, die Antwort trägt die Marke.
	StateAcquired = "acquired"
	// StateBusy: belegt, die Antwort trägt den Halter.
	StateBusy = "busy"
	// StateHeld: die Marke gilt.
	StateHeld = "held"
	// StateLost: die Marke gilt nicht mehr; Halter (oder frei) und Grund.
	StateLost = "lost"
	// StateReleased: freigegeben.
	StateReleased = "released"
	// StateFree: niemand hält die Sperre.
	StateFree = "free"
)

// Die Gründe, aus denen eine Marke nicht mehr gilt.
const (
	// ReasonReleased: vom Halter selbst freigegeben.
	ReasonReleased = "released"
	// ReasonForced: vom Nutzer ausdrücklich freigegeben (force).
	ReasonForced = "forced"
	// ReasonTakenOver: als sicher verwaist von einem anderen übernommen.
	ReasonTakenOver = "taken_over"
	// ReasonFileMissing: die Sperrdatei fehlte.
	ReasonFileMissing = "file_missing"
	// ReasonUnknown: über diese Marke ist nichts bekannt.
	ReasonUnknown = "unknown"
)

// Die Werte von Orphaned.
const (
	OrphanedNo        = "no"
	OrphanedYes       = "yes"
	OrphanedUncertain = "uncertain"
)

// ErrInvalidInput ist ein Aufruf mit fehlender oder unpassender Angabe.
var ErrInvalidInput = errors.New("ungültige Eingabe")

// Holder beschreibt, wer die Sperre hält. Session, Client, ClientSession und
// Purpose sind reine Anzeige; Process entscheidet über „verwaist“.
type Holder struct {
	// Session ist die Sitzungskennung aus dem Briefing. Sie wechselt mit
	// jedem neuen Briefing und taugt deshalb nicht zum Wiedererkennen.
	Session string `json:"session,omitempty"`
	// Client ist der Assistent (claude-code, opencode) oder cli.
	Client string `json:"client,omitempty"`
	// ClientSession ist die Sitzung, die der Client bei einem Aufruf mitgibt
	// (OpenCode), nur zur Anzeige.
	ClientSession string `json:"clientSession,omitempty"`
	// Purpose ist das Wozu im Klartext.
	Purpose string `json:"purpose,omitempty"`
	// Process ist der Halter-Prozess; fehlt er, ist die Sperre nie sicher
	// verwaist.
	Process *Process `json:"process,omitempty"`
}

// event ist das Ende einer Marke.
type event struct {
	Token  string    `json:"token"`
	Number int64     `json:"number"`
	At     time.Time `json:"at"`
	Reason string    `json:"reason"`
	// By ist der Halter, der danach kam (bei taken_over), sonst leer.
	By *Holder `json:"by,omitempty"`
}

// fileState ist der Inhalt der Sperrdatei. Sie bleibt nach dem Freigeben
// stehen: der Zähler muss weiterlaufen, und die Enden früherer Marken nennen
// einer Sitzung den Grund ihres Verlusts.
type fileState struct {
	Version int    `json:"version"`
	Subject string `json:"subject"`
	Kind    string `json:"kind"`
	// Counter ist die Zählnummer der letzten vergebenen Marke.
	Counter int64 `json:"counter"`
	// Token ist die gültige Marke; leer heißt frei.
	Token   string    `json:"token,omitempty"`
	Since   time.Time `json:"since,omitzero"`
	Holder  *Holder   `json:"holder,omitempty"`
	History []event   `json:"history,omitempty"`
}

// HolderView ist der Halter in einer Antwort: wer, seit wann, die Nummer
// seiner Marke (nicht die Marke selbst — mit ihr könnte jeder freigeben) und
// ob er als verwaist gilt.
type HolderView struct {
	Holder
	Number   int64     `json:"number"`
	Since    time.Time `json:"since"`
	Orphaned string    `json:"orphaned"`
	// OrphanedWhy sagt, worauf Orphaned beruht.
	OrphanedWhy string `json:"orphanedWhy,omitempty"`
}

// Result ist die Antwort aller Aktionen.
type Result struct {
	State string `json:"state"`
	// Token ist die eigene Marke bei acquired und held.
	Token  string `json:"token,omitempty"`
	Number int64  `json:"number,omitempty"`
	// Reentrant: acquire mit der eigenen, noch gültigen Marke.
	Reentrant bool `json:"reentrant,omitempty"`
	// Holder ist der Halter bei busy, lost und bei status; bei lost ohne
	// Halter ist die Sperre frei.
	Holder *HolderView `json:"holder,omitempty"`
	// Reason ist bei lost der Grund, soweit bekannt.
	Reason string `json:"reason,omitempty"`
	// TakenOver ist der als sicher verwaist übernommene Halter.
	TakenOver *HolderView `json:"takenOver,omitempty"`
	// WaitedSeconds ist bei wait die gewartete Zeit.
	WaitedSeconds int `json:"waitedSeconds,omitempty"`
	// Path ist die Sperrdatei.
	Path string `json:"path"`
}

// Lock ist die Sperre eines Projekts.
type Lock struct {
	projectDir string
	// now und alive sind für Tests austauschbar.
	now   func() time.Time
	probe ProcessProbe
}

// New gibt die Sperre des Projekts projectDir zurück. Es legt nichts an.
func New(projectDir string) *Lock {
	return &Lock{projectDir: projectDir, now: time.Now, probe: systemProbe{}}
}

// WithProbe ersetzt die Prozessprüfung, für Tests.
func (lock *Lock) WithProbe(probe ProcessProbe) *Lock {
	lock.probe = probe
	return lock
}

// WithClock ersetzt die Uhr, für Tests.
func (lock *Lock) WithClock(now func() time.Time) *Lock {
	lock.now = now
	return lock
}

// Dir ist das Verzeichnis der Sperrdateien.
func (lock *Lock) Dir() string { return filepath.Join(lock.projectDir, lockDirRel) }

// Path ist die Sperrdatei des Arbeitsbaums.
func (lock *Lock) Path() string { return filepath.Join(lock.Dir(), SubjectWorktree+".json") }

func (lock *Lock) mutexPath() string { return filepath.Join(lock.Dir(), SubjectWorktree+".mutex") }

// Acquire nimmt die Sperre für holder. Mit token, der noch gültigen eigenen
// Marke, gibt es dieselbe Marke zurück und setzt den Halter neu (reentrant,
// etwa nach einem Neustart des Clients). Hält ein anderer, und ist er sicher
// verwaist, wird übernommen; sonst busy mit dem Halter.
func (lock *Lock) Acquire(token string, holder Holder) (Result, error) {
	var result Result
	err := lock.update(true, func(state *fileState, exists bool) (bool, error) {
		if state.Token != "" {
			if token != "" && token == state.Token {
				state.Holder = mergeHolder(state.Holder, holder)
				result = Result{State: StateAcquired, Token: state.Token, Number: state.Counter, Reentrant: true}
				return true, nil
			}
			view := lock.view(state)
			if view.Orphaned != OrphanedYes {
				result = Result{State: StateBusy, Holder: &view}
				return false, nil
			}
			result.TakenOver = &view
			lock.end(state, ReasonTakenOver, &holder)
		}
		newToken, err := lock.issue(state, holder)
		if err != nil {
			return false, err
		}
		result.State = StateAcquired
		result.Token = newToken
		result.Number = state.Counter
		return true, nil
	})
	result.Path = lock.Path()
	return result, err
}

// Wait nimmt die Sperre wie Acquire und wartet, solange sie belegt ist, bis
// höchstens timeout. Während des Wartens hält es nichts fest: Andere Aufrufe
// — check und release anderer Sitzungen, die Aufrufe der eigenen
// Sub-Agenten — laufen ungehindert. Nach der Frist kommt busy mit dem Halter.
func (lock *Lock) Wait(ctx context.Context, token string, holder Holder, timeout, interval time.Duration) (Result, error) {
	start := lock.now()
	deadline := time.Now().Add(timeout)
	for {
		result, err := lock.Acquire(token, holder)
		result.WaitedSeconds = int(lock.now().Sub(start).Round(time.Second) / time.Second)
		if err != nil || result.State != StateBusy {
			return result, err
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return result, nil
		}
		pause := min(interval, remaining)
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case <-time.After(pause):
		}
	}
}

// Check prüft die eigene Marke: held, oder lost mit dem jetzigen Halter (oder
// frei) und dem Grund. Gilt die Marke, wird der Halter auf den Aufrufer
// gesetzt, wenn der einen Halter-Prozess kennt — so gilt die Sperre einer
// Sitzung, deren Client neu gestartet wurde und die weiterarbeitet, nicht
// länger als verwaist.
func (lock *Lock) Check(token string, holder Holder) (Result, error) {
	if strings.TrimSpace(token) == "" {
		return Result{Path: lock.Path()}, fmt.Errorf("%w: check braucht die Marke (token)", ErrInvalidInput)
	}
	var result Result
	err := lock.update(false, func(state *fileState, exists bool) (bool, error) {
		if !exists {
			result = Result{State: StateLost, Reason: ReasonFileMissing}
			return false, nil
		}
		if state.Token == token {
			changed := holder.Process != nil && !sameProcess(state.Holder, holder.Process)
			if changed {
				state.Holder = mergeHolder(state.Holder, holder)
			}
			result = Result{State: StateHeld, Token: token, Number: state.Counter}
			return changed, nil
		}
		result = lock.lost(state, token)
		return false, nil
	})
	result.Path = lock.Path()
	return result, err
}

// Release gibt die Sperre mit der eigenen Marke frei. Eine veraltete Marke
// gibt nichts frei und meldet lost. Ohne Marke nur mit force, und das nur auf
// ausdrücklichen Wunsch des Nutzers; expectNumber > 0 gibt dann nur frei,
// solange noch die Marke mit dieser Nummer gilt, damit ein inzwischen neu
// genommener Halter nicht mit freigegeben wird.
func (lock *Lock) Release(token string, force bool, expectNumber int64) (Result, error) {
	if strings.TrimSpace(token) == "" && !force {
		return Result{Path: lock.Path()}, fmt.Errorf("%w: release braucht die Marke (token) oder force", ErrInvalidInput)
	}
	var result Result
	err := lock.update(false, func(state *fileState, exists bool) (bool, error) {
		switch {
		case !exists && token != "" && !force:
			result = Result{State: StateLost, Reason: ReasonFileMissing}
			return false, nil
		case state.Token == "":
			if token != "" && !force {
				result = lock.lost(state, token)
				return false, nil
			}
			result = Result{State: StateFree}
			return false, nil
		case token != "" && token == state.Token:
			number := state.Counter
			lock.end(state, ReasonReleased, nil)
			result = Result{State: StateReleased, Number: number}
			return true, nil
		case force:
			if expectNumber > 0 && expectNumber != state.Counter {
				view := lock.view(state)
				result = Result{State: StateBusy, Holder: &view}
				return false, nil
			}
			view := lock.view(state)
			lock.end(state, ReasonForced, nil)
			result = Result{State: StateReleased, Number: view.Number, Holder: &view}
			return true, nil
		default:
			result = lock.lost(state, token)
			return false, nil
		}
	})
	result.Path = lock.Path()
	return result, err
}

// Status nennt den Halter oder frei — für jede Sitzung, auch eine, die ihre
// Marke nicht mehr kennt.
func (lock *Lock) Status() (Result, error) {
	var result Result
	err := lock.update(false, func(state *fileState, exists bool) (bool, error) {
		if state.Token == "" {
			result = Result{State: StateFree}
			return false, nil
		}
		view := lock.view(state)
		result = Result{State: StateBusy, Holder: &view}
		return false, nil
	})
	result.Path = lock.Path()
	return result, err
}

// lost baut die Antwort für eine Marke, die nicht (mehr) gilt.
func (lock *Lock) lost(state *fileState, token string) Result {
	result := Result{State: StateLost, Reason: ReasonUnknown}
	for index := len(state.History) - 1; index >= 0; index-- {
		if state.History[index].Token == token {
			result.Reason = state.History[index].Reason
			break
		}
	}
	if state.Token != "" {
		view := lock.view(state)
		result.Holder = &view
	}
	return result
}

// issue vergibt eine neue Marke an holder.
func (lock *Lock) issue(state *fileState, holder Holder) (string, error) {
	nonce := make([]byte, 6)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("Marke erzeugen: %w", err)
	}
	state.Counter++
	state.Token = strconv.FormatInt(state.Counter, 10) + "-" + hex.EncodeToString(nonce)
	state.Since = lock.now().UTC().Truncate(time.Second)
	held := holder
	state.Holder = &held
	return state.Token, nil
}

// end beendet die gültige Marke mit reason und merkt sich das Ende.
func (lock *Lock) end(state *fileState, reason string, by *Holder) {
	state.History = append(state.History, event{
		Token: state.Token, Number: state.Counter, At: lock.now().UTC().Truncate(time.Second),
		Reason: reason, By: by,
	})
	if len(state.History) > historyLimit {
		state.History = state.History[len(state.History)-historyLimit:]
	}
	state.Token = ""
	state.Since = time.Time{}
	state.Holder = nil
}

// view macht aus dem Halter die Anzeige samt der Verwaist-Prüfung.
func (lock *Lock) view(state *fileState) HolderView {
	view := HolderView{Number: state.Counter, Since: state.Since}
	if state.Holder != nil {
		view.Holder = *state.Holder
	}
	view.Orphaned, view.OrphanedWhy = orphaned(lock.probe, state.Holder)
	return view
}

// mergeHolder setzt den neuen Halter ein und behält Angaben zur Anzeige, die
// der neue nicht mitbringt.
func mergeHolder(old *Holder, next Holder) *Holder {
	merged := next
	if old != nil {
		if merged.Session == "" {
			merged.Session = old.Session
		}
		if merged.Purpose == "" {
			merged.Purpose = old.Purpose
		}
		if merged.Process == nil {
			merged.Process = old.Process
		}
	}
	return &merged
}

// sameProcess meldet, ob holder schon auf process steht.
func sameProcess(holder *Holder, process *Process) bool {
	if holder == nil || holder.Process == nil || process == nil {
		return false
	}
	return *holder.Process == *process
}

// update liest die Sperrdatei unter dem Mutex, lässt change sie ändern und
// schreibt sie zurück, wenn change das meldet. create legt Verzeichnis und
// Mutex an, wenn sie fehlen; sonst gilt ein fehlendes Verzeichnis als frei,
// ohne etwas anzulegen.
//
// Atomar über flock auf eine eigene Mutex-Datei: Die Sperrdatei selbst wird
// über eine temporäre Datei und rename ersetzt, eine Sperre auf ihr ginge mit
// jedem Schreiben verloren. Zwei Sitzungen — auch zwei Aufrufe in demselben
// Prozess, denn flock gilt je geöffneter Datei — kommen so nie gleichzeitig
// an den Inhalt.
func (lock *Lock) update(create bool, change func(state *fileState, exists bool) (bool, error)) error {
	dir := lock.Dir()
	if _, err := os.Stat(dir); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("%s: %w", dir, err)
		}
		if !create {
			state := emptyState()
			_, err := change(&state, false)
			return err
		}
	}
	if create {
		if err := lock.ensureDir(); err != nil {
			return err
		}
	}

	mutex, err := os.OpenFile(lock.mutexPath(), os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return fmt.Errorf("%s öffnen: %w", lock.mutexPath(), err)
	}
	defer mutex.Close()
	if err := syscall.Flock(int(mutex.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("%s sperren: %w", lock.mutexPath(), err)
	}
	defer syscall.Flock(int(mutex.Fd()), syscall.LOCK_UN) //nolint:errcheck // Close gibt ohnehin frei

	state, exists, err := lock.read()
	if err != nil {
		return err
	}
	write, err := change(&state, exists)
	if err != nil || !write {
		return err
	}
	return lock.write(state)
}

// ensureDir legt das Verzeichnis samt der Datei an, die es aus Git heraushält.
// Die Datei kommt zuerst, damit nie eine Sperrdatei ohne sie dasteht.
func (lock *Lock) ensureDir() error {
	dir := lock.Dir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("%s anlegen: %w", dir, err)
	}
	ignore := filepath.Join(dir, ignoreFileName)
	if _, err := os.Stat(ignore); os.IsNotExist(err) {
		if err := os.WriteFile(ignore, []byte(ignoreFileContent), 0o644); err != nil {
			return fmt.Errorf("%s schreiben: %w", ignore, err)
		}
	}
	return nil
}

func emptyState() fileState {
	return fileState{Version: formatVersion, Subject: SubjectWorktree, Kind: KindTaskRun}
}

// read liest die Sperrdatei; fehlt sie, ist das ein leerer Zustand.
func (lock *Lock) read() (fileState, bool, error) {
	data, err := os.ReadFile(lock.Path())
	if os.IsNotExist(err) {
		return emptyState(), false, nil
	}
	if err != nil {
		return fileState{}, false, fmt.Errorf("%s lesen: %w", lock.Path(), err)
	}
	state := emptyState()
	if err := json.Unmarshal(data, &state); err != nil {
		return fileState{}, false, fmt.Errorf("%s ist unlesbar: %w", lock.Path(), err)
	}
	return state, true, nil
}

// write ersetzt die Sperrdatei atomar.
func (lock *Lock) write(state fileState) error {
	state.Version = formatVersion
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(lock.Dir(), SubjectWorktree+".*.tmp")
	if err != nil {
		return fmt.Errorf("Sperrdatei schreiben: %w", err)
	}
	if _, err := temp.Write(append(data, '\n')); err != nil {
		temp.Close()
		os.Remove(temp.Name())
		return fmt.Errorf("Sperrdatei schreiben: %w", err)
	}
	if err := temp.Close(); err != nil {
		os.Remove(temp.Name())
		return fmt.Errorf("Sperrdatei schreiben: %w", err)
	}
	if err := os.Rename(temp.Name(), lock.Path()); err != nil {
		os.Remove(temp.Name())
		return fmt.Errorf("Sperrdatei ersetzen: %w", err)
	}
	return nil
}
