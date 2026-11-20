package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/worklock"
)

func lockCLI(t *testing.T, args ...string) (worklock.Result, error) {
	t.Helper()
	var out bytes.Buffer
	err := runLock(&out, append(args, "--json"))
	var result worklock.Result
	if out.Len() > 0 {
		if jerr := json.Unmarshal(out.Bytes(), &result); jerr != nil {
			t.Fatalf("lock %v: kein JSON (%v): %s", args, jerr, out.String())
		}
	}
	return result, err
}

// Die CLI nimmt ohne Halter-Prozess: Ihre Sperre gilt nie als sicher
// verwaist, belegt endet mit Exit-Code 3, und release --force gibt frei.
func TestLockCLI(t *testing.T) {
	root := newProject(t)

	taken, err := lockCLI(t, "acquire", "--dir", root, "--purpose", "von Hand")
	if err != nil || taken.State != worklock.StateAcquired || taken.Token == "" {
		t.Fatalf("acquire: %+v, %v", taken, err)
	}
	busy, err := lockCLI(t, "acquire", "--dir", root)
	if exitCodeOf(err) != lockExitNotYours || busy.State != worklock.StateBusy || busy.Holder == nil ||
		busy.Holder.Client != "cli" || busy.Holder.Orphaned != worklock.OrphanedUncertain || busy.Holder.Process != nil {
		t.Fatalf("belegt: %+v, %v", busy, err)
	}
	held, err := lockCLI(t, "check", "--dir", root, "--token", taken.Token)
	if err != nil || held.State != worklock.StateHeld {
		t.Fatalf("check: %+v, %v", held, err)
	}
	forced, err := lockCLI(t, "release", "--dir", root, "--force")
	if err != nil || forced.State != worklock.StateReleased {
		t.Fatalf("release --force: %+v, %v", forced, err)
	}
	lost, err := lockCLI(t, "check", "--dir", root, "--token", taken.Token)
	if exitCodeOf(err) != lockExitNotYours || lost.State != worklock.StateLost || lost.Reason != worklock.ReasonForced {
		t.Fatalf("check nach force: %+v, %v", lost, err)
	}
	if status, err := lockCLI(t, "status", "--dir", root); err != nil || status.State != worklock.StateFree {
		t.Fatalf("status: %+v, %v", status, err)
	}

	var out bytes.Buffer
	if err := runLock(&out, []string{"status", "--dir", root}); err != nil || !strings.Contains(out.String(), "Frei") {
		t.Fatalf("Klartext: %q, %v", out.String(), err)
	}
}

// Über den echten Server: Ein laufendes wait hält den Server nicht auf. Die
// Freigabe kommt über denselben Server, während wait noch läuft, und wird
// zuerst beantwortet; wait kehrt danach mit einer neuen Marke zurück.
func TestLockWaitUndReleaseUeberDenselbenServer(t *testing.T) {
	root := newProject(t)
	taken, err := lockCLI(t, "acquire", "--dir", root)
	if err != nil || taken.State != worklock.StateAcquired {
		t.Fatalf("acquire: %+v, %v", taken, err)
	}

	stdout := speak(t, root, 3, initialize(1), initialized,
		callTool(2, "k_playbook_lock", map[string]any{"projectDir": root, "action": "wait", "seconds": 30}),
		callTool(3, "k_playbook_lock", map[string]any{"projectDir": root, "action": "release", "token": taken.Token}),
	)
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 3 || !strings.Contains(lines[1], `"id":3`) || !strings.Contains(lines[2], `"id":2`) {
		t.Fatalf("release nicht vor wait beantwortet:\n%s", stdout)
	}
	responses := decodeResponses(t, stdout, 1, 2, 3)

	var waited, released struct {
		OK bool `json:"ok"`
		worklock.Result
	}
	if err := json.Unmarshal([]byte(toolText(t, responses[1])), &waited); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(toolText(t, responses[2])), &released); err != nil {
		t.Fatal(err)
	}
	if !released.OK || released.State != worklock.StateReleased {
		t.Fatalf("release: %+v", released)
	}
	if !waited.OK || waited.State != worklock.StateAcquired || waited.Token == "" || waited.Token == taken.Token {
		t.Fatalf("wait: %+v", waited)
	}
}
