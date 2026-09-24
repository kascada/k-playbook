package branches

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"

	"github.com/kascada/k-playbook/installer/internal/github"
)

// CombinedRunner führt ein Kommando aus und liefert stdout und stderr zusammen,
// in der Reihenfolge, in der sie geschrieben wurden. Fetch und Switch brauchen
// das: git schreibt „Switched to branch …" und den Fortschritt eines Fetch auf
// stderr, und github.ExecRunner behält stderr nur im Fehlerfall. Es erfüllt
// dasselbe Interface github.Runner und bleibt damit dieselbe Naht.
type CombinedRunner struct{}

func (CombinedRunner) Run(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, name, args...)
	command.Dir = dir
	command.WaitDelay = time.Second
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	err := command.Run()
	if err != nil {
		code := -1
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code = exitErr.ExitCode()
		}
		return output.Bytes(), &github.CommandError{Name: name, Args: args, ExitCode: code, Stderr: output.String(), Err: err}
	}
	return output.Bytes(), nil
}

// FetchTimeout begrenzt den ausdrücklichen Fetch. Er spricht mit jedem Remote
// und darf dauern, aber nicht ohne Ende.
const FetchTimeout = 2 * time.Minute

// FetchResult ist die Antwort von POST /api/branches/fetch.
type FetchResult struct {
	OK      bool      `json:"ok"`
	Command string    `json:"command"`
	Output  string    `json:"output"`
	Message string    `json:"message"`
	Fetch   FetchInfo `json:"fetch"`
}

// fetchArgs ist der eine Fetch, den die Oberfläche kennt. --prune entfernt
// Remote-Tracking-Branches, die es auf dem Remote nicht mehr gibt — erst dadurch
// erscheint ein Upstream als [gone]. Lokale Branches und der Arbeitsbaum bleiben
// unberührt.
var fetchArgs = []string{"fetch", "--all", "--prune"}

// Fetch holt den Stand aller Remotes. Er läuft nur auf ausdrückliche Anforderung:
// die Liste arbeitet sonst mit den vorhandenen Remote-Tracking-Refs. runner ist
// in der Regel ein CombinedRunner; nil heißt ebenfalls CombinedRunner.
func Fetch(ctx context.Context, runner github.Runner, repoDir string) FetchResult {
	if runner == nil {
		runner = CombinedRunner{}
	}
	repo := Repo{Runner: runner, Dir: repoDir}
	result := FetchResult{Command: "git " + strings.Join(fetchArgs, " ")}
	out, err := repo.run(ctx, FetchTimeout, fetchArgs...)
	result.Output = strings.TrimSpace(string(out))
	switch {
	case err != nil && timedOut(err):
		result.Message = "git fetch hat nicht rechtzeitig geantwortet und wurde abgebrochen."
	case err != nil:
		if result.Output == "" {
			result.Output = strings.TrimSpace(stderrOf(err))
		}
		result.Message = "git fetch ist gescheitert; die Ausgabe steht darunter."
	default:
		result.OK = true
		result.Message = "Die Remote-Tracking-Branches sind aktualisiert."
	}

	st := &state{repo: Repo{Dir: repoDir}, options: Options{RepoDir: repoDir}}
	st.readFetch(ctx)
	result.Fetch = st.listing.Fetch
	return result
}

// SwitchTimeout begrenzt git switch. Ein großer Arbeitsbaum braucht Zeit, ein
// hängender Hook soll die Anfrage trotzdem nicht ewig offen halten.
const SwitchTimeout = time.Minute

// Gründe, aus denen nicht umgeschaltet wurde oder werden konnte.
const (
	SwitchReasonStamp     = "stamp"
	SwitchReasonBlocked   = "blocked"
	SwitchReasonState     = "state"
	SwitchReasonGitFailed = "git-failed"
)

// SwitchResult ist die Antwort von POST /api/branches/switch.
type SwitchResult struct {
	// Executed meldet, ob git switch überhaupt gestartet wurde.
	Executed bool `json:"executed"`
	// OK meldet, ob git switch erfolgreich war.
	OK      bool   `json:"ok"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message"`
	Command string `json:"command,omitempty"`
	Output  string `json:"output,omitempty"`
	Before  Head   `json:"before"`
	After   Head   `json:"after"`
	// Check ist die Prüfung unmittelbar vor der Ausführung. Wurde nicht
	// umgeschaltet, sagt sie warum.
	Check SwitchCheck `json:"check"`
}

// Switch prüft erneut und schaltet nur um, wenn der Prüfstempel noch stimmt und
// nichts blockiert. Kein --force, kein --discard-changes, kein Stash, kein Pull;
// scheitert git switch, wird nichts weiter versucht.
func Switch(ctx context.Context, options CheckOptions, stamp string, action github.Runner) SwitchResult {
	if action == nil {
		action = CombinedRunner{}
	}
	check := RunCheck(ctx, options)
	result := SwitchResult{Check: check, Before: check.Source, After: check.Source, Command: check.Command}
	switch {
	case check.State != StateOK:
		result.Reason = SwitchReasonState
		result.Message = "Nicht umgeschaltet: " + check.Message
		return result
	case stamp == "" || check.Stamp != stamp:
		result.Reason = SwitchReasonStamp
		result.Message = "Nicht umgeschaltet: Seit der Prüfung hat sich der Stand geändert (HEAD, Ziel oder Arbeitsbaum). Die Prüfpunkte zeigen jetzt den neuen Stand; umgeschaltet wird erst nach erneuter Bestätigung."
		return result
	case !check.Offered || len(check.Args) == 0:
		result.Reason = SwitchReasonBlocked
		result.Message = "Nicht umgeschaltet: Die erneute Prüfung hat etwas Blockierendes gefunden."
		return result
	}

	repo := Repo{Runner: action, Dir: options.RepoDir}
	result.Executed = true
	out, err := repo.run(ctx, SwitchTimeout, check.Args...)
	result.Output = strings.TrimSpace(string(out))
	if err != nil && result.Output == "" {
		result.Output = strings.TrimSpace(stderrOf(err))
	}

	after := &state{repo: Repo{Runner: options.Runner, Dir: options.RepoDir}}
	if readErr := after.readHead(ctx); readErr == nil {
		result.After = after.head
	}

	switch {
	case err != nil && timedOut(err):
		result.Reason = SwitchReasonGitFailed
		result.Message = "git switch hat nicht rechtzeitig geantwortet und wurde abgebrochen. Es wurde nichts weiter versucht; den Stand danach zeigt die Liste."
	case err != nil:
		result.Reason = SwitchReasonGitFailed
		result.Message = "git switch hat den Wechsel verweigert. Es wurde nichts weiter versucht — kein --force, kein Stash; die Ausgabe von git steht darunter."
	default:
		result.OK = true
		result.Message = "Umgeschaltet auf " + options.Target + "."
	}
	return result
}
