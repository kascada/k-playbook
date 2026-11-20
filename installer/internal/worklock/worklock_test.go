package worklock

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// fakeProbe ist ein Rechner mit einstellbaren Prozessen.
type fakeProbe struct {
	mu        sync.Mutex
	host      string
	boot      string
	namespace string
	starts    map[int]string
}

func newFakeProbe() *fakeProbe {
	return &fakeProbe{host: "rechner", boot: "boot-1", namespace: "pid:[1]", starts: map[int]string{}}
}

func (probe *fakeProbe) Self() (string, string, string, bool) {
	return probe.host, probe.boot, probe.namespace, true
}

func (probe *fakeProbe) Start(pid int) (string, bool, error) {
	probe.mu.Lock()
	defer probe.mu.Unlock()
	start, ok := probe.starts[pid]
	return start, ok, nil
}

func (probe *fakeProbe) run(pid int, start string) {
	probe.mu.Lock()
	defer probe.mu.Unlock()
	probe.starts[pid] = start
}

func (probe *fakeProbe) kill(pid int) {
	probe.mu.Lock()
	defer probe.mu.Unlock()
	delete(probe.starts, pid)
}

// sessionHolder ist eine Sitzung von Claude Code mit Prozess pid.
func sessionHolder(probe *fakeProbe, pid int, role string) Holder {
	start := "1000" + strconv.Itoa(pid)
	probe.run(pid, start)
	return Holder{Session: "s-" + strconv.Itoa(pid), Client: "claude-code", Purpose: "Task 001", Process: &Process{
		PID: pid, Start: start, Name: "claude", Role: role, Host: probe.host, BootID: probe.boot, PIDNamespace: probe.namespace,
	}}
}

func newLock(t *testing.T) (*Lock, *fakeProbe) {
	t.Helper()
	probe := newFakeProbe()
	return New(t.TempDir()).WithProbe(probe), probe
}

func mustAcquire(t *testing.T, lock *Lock, token string, holder Holder) Result {
	t.Helper()
	result, err := lock.Acquire(token, holder)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	return result
}

func TestProjectDirNameMatchesProject(t *testing.T) {
	if projectDirName != project.ProjectDirName {
		t.Fatalf("projectDirName = %q, project.ProjectDirName = %q", projectDirName, project.ProjectDirName)
	}
}

func TestAcquireBusyReentrant(t *testing.T) {
	lock, probe := newLock(t)
	first := sessionHolder(probe, 100, RoleSession)
	second := sessionHolder(probe, 200, RoleSession)

	taken := mustAcquire(t, lock, "", first)
	if taken.State != StateAcquired || taken.Token == "" || taken.Number != 1 {
		t.Fatalf("erstes Nehmen: %+v", taken)
	}

	busy := mustAcquire(t, lock, "", second)
	if busy.State != StateBusy || busy.Holder == nil || busy.Holder.Session != "s-100" || busy.Holder.Orphaned != OrphanedNo {
		t.Fatalf("belegt: %+v", busy)
	}
	if busy.Token != "" {
		t.Fatalf("belegt verrät eine Marke: %+v", busy)
	}

	// Dieselbe Sitzung ohne Marke nimmt nicht noch einmal: erkannt wird an der
	// Marke, nicht am Prozess.
	if again := mustAcquire(t, lock, "", first); again.State != StateBusy {
		t.Fatalf("ohne Marke erneut genommen: %+v", again)
	}

	reentrant := mustAcquire(t, lock, taken.Token, first)
	if reentrant.State != StateAcquired || !reentrant.Reentrant || reentrant.Token != taken.Token {
		t.Fatalf("reentrant: %+v", reentrant)
	}
}

func TestCounterRisesAndOldTokenNeverMatches(t *testing.T) {
	lock, probe := newLock(t)
	holder := sessionHolder(probe, 100, RoleSession)
	first := mustAcquire(t, lock, "", holder)
	if _, err := lock.Release(first.Token, false, 0); err != nil {
		t.Fatal(err)
	}
	second := mustAcquire(t, lock, first.Token, holder)
	if second.Reentrant || second.Number != 2 || second.Token == first.Token {
		t.Fatalf("neues Nehmen nach Freigabe: %+v", second)
	}

	// Fehlt die Datei, beginnt der Zähler neu — die alte Marke passt trotzdem nicht.
	if err := os.Remove(lock.Path()); err != nil {
		t.Fatal(err)
	}
	third := mustAcquire(t, lock, "", holder)
	if third.Number != 1 || third.Token == first.Token {
		t.Fatalf("nach fehlender Datei: %+v (erste Marke %s)", third, first.Token)
	}
	if check, _ := lock.Check(first.Token, Holder{}); check.State != StateLost {
		t.Fatalf("alte Marke gilt: %+v", check)
	}
}

func TestCLIHolderAlwaysUncertain(t *testing.T) {
	lock, probe := newLock(t)
	mustAcquire(t, lock, "", Holder{Client: "cli", Purpose: "von Hand"})
	status, err := lock.Status()
	if err != nil {
		t.Fatal(err)
	}
	if status.State != StateBusy || status.Holder.Orphaned != OrphanedUncertain {
		t.Fatalf("CLI-Halter: %+v", status)
	}
	busy := mustAcquire(t, lock, "", sessionHolder(probe, 300, RoleSession))
	if busy.State != StateBusy {
		t.Fatalf("CLI-Sperre übernommen: %+v", busy)
	}
}

func TestCheckHeldAndLost(t *testing.T) {
	t.Run("gehalten", func(t *testing.T) {
		lock, probe := newLock(t)
		taken := mustAcquire(t, lock, "", sessionHolder(probe, 100, RoleSession))
		check, err := lock.Check(taken.Token, Holder{})
		if err != nil || check.State != StateHeld || check.Token != taken.Token {
			t.Fatalf("check: %+v, %v", check, err)
		}
	})
	t.Run("force", func(t *testing.T) {
		lock, probe := newLock(t)
		taken := mustAcquire(t, lock, "", sessionHolder(probe, 100, RoleSession))
		released, err := lock.Release("", true, 0)
		if err != nil || released.State != StateReleased || released.Holder == nil || released.Holder.Session != "s-100" {
			t.Fatalf("force: %+v, %v", released, err)
		}
		check, _ := lock.Check(taken.Token, Holder{})
		if check.State != StateLost || check.Reason != ReasonForced || check.Holder != nil {
			t.Fatalf("nach force: %+v", check)
		}
	})
	t.Run("übernommen", func(t *testing.T) {
		lock, probe := newLock(t)
		taken := mustAcquire(t, lock, "", sessionHolder(probe, 100, RoleSession))
		probe.kill(100)
		next := mustAcquire(t, lock, "", sessionHolder(probe, 200, RoleSession))
		if next.State != StateAcquired || next.TakenOver == nil || next.TakenOver.Session != "s-100" || next.TakenOver.Orphaned != OrphanedYes {
			t.Fatalf("Übernahme: %+v", next)
		}
		check, _ := lock.Check(taken.Token, Holder{})
		if check.State != StateLost || check.Reason != ReasonTakenOver || check.Holder == nil || check.Holder.Session != "s-200" {
			t.Fatalf("nach Übernahme: %+v", check)
		}
	})
	t.Run("Datei fehlte", func(t *testing.T) {
		lock, probe := newLock(t)
		taken := mustAcquire(t, lock, "", sessionHolder(probe, 100, RoleSession))
		if err := os.Remove(lock.Path()); err != nil {
			t.Fatal(err)
		}
		check, _ := lock.Check(taken.Token, Holder{})
		if check.State != StateLost || check.Reason != ReasonFileMissing {
			t.Fatalf("ohne Datei: %+v", check)
		}
		// Auch ohne das ganze Verzeichnis, und ohne es anzulegen.
		if err := os.RemoveAll(lock.Dir()); err != nil {
			t.Fatal(err)
		}
		check, _ = lock.Check(taken.Token, Holder{})
		if check.State != StateLost || check.Reason != ReasonFileMissing {
			t.Fatalf("ohne Verzeichnis: %+v", check)
		}
		if _, err := os.Stat(lock.Dir()); !os.IsNotExist(err) {
			t.Fatalf("check hat das Verzeichnis angelegt: %v", err)
		}
	})
	t.Run("ohne Marke", func(t *testing.T) {
		lock, _ := newLock(t)
		if _, err := lock.Check("", Holder{}); err == nil {
			t.Fatal("check ohne Marke ohne Fehler")
		}
	})
}

func TestCheckRebindsHolderProcess(t *testing.T) {
	lock, probe := newLock(t)
	taken := mustAcquire(t, lock, "", sessionHolder(probe, 100, RoleSession))
	probe.kill(100)
	// Die Sitzung lebt in einem neuen Prozess weiter (Client neu gestartet)
	// und prüft ihre Marke, bevor ein anderer übernommen hat.
	check, err := lock.Check(taken.Token, sessionHolder(probe, 101, RoleSession))
	if err != nil || check.State != StateHeld {
		t.Fatalf("check: %+v, %v", check, err)
	}
	status, _ := lock.Status()
	if status.Holder.Orphaned != OrphanedNo || status.Holder.Process.PID != 101 || status.Holder.Purpose != "Task 001" {
		t.Fatalf("Halter nicht umgesetzt: %+v", status.Holder)
	}
}

func TestReleaseOnlyByHolder(t *testing.T) {
	lock, probe := newLock(t)
	taken := mustAcquire(t, lock, "", sessionHolder(probe, 100, RoleSession))

	if _, err := lock.Release("", false, 0); err == nil {
		t.Fatal("release ohne Marke und ohne force ohne Fehler")
	}
	foreign, err := lock.Release("99-000000000000", false, 0)
	if err != nil || foreign.State != StateLost || foreign.Reason != ReasonUnknown || foreign.Holder == nil {
		t.Fatalf("fremde Marke: %+v, %v", foreign, err)
	}
	if status, _ := lock.Status(); status.State != StateBusy {
		t.Fatalf("fremde Marke hat freigegeben: %+v", status)
	}

	released, err := lock.Release(taken.Token, false, 0)
	if err != nil || released.State != StateReleased {
		t.Fatalf("release: %+v, %v", released, err)
	}
	if status, _ := lock.Status(); status.State != StateFree {
		t.Fatalf("nach release: %+v", status)
	}
}

func TestReleaseWithStaleToken(t *testing.T) {
	lock, probe := newLock(t)
	first := mustAcquire(t, lock, "", sessionHolder(probe, 100, RoleSession))
	if _, err := lock.Release("", true, 0); err != nil {
		t.Fatal(err)
	}
	second := mustAcquire(t, lock, "", sessionHolder(probe, 200, RoleSession))

	stale, err := lock.Release(first.Token, false, 0)
	if err != nil || stale.State != StateLost || stale.Reason != ReasonForced || stale.Holder == nil || stale.Holder.Number != second.Number {
		t.Fatalf("veraltete Marke: %+v, %v", stale, err)
	}
	if check, _ := lock.Check(second.Token, Holder{}); check.State != StateHeld {
		t.Fatalf("veraltete Marke hat den neuen Halter freigegeben: %+v", check)
	}

	// Gegen einen inzwischen neuen Halter wirkt force mit erwarteter Nummer nicht.
	guarded, err := lock.Release("", true, first.Number)
	if err != nil || guarded.State != StateBusy {
		t.Fatalf("force mit alter Nummer: %+v, %v", guarded, err)
	}
	if check, _ := lock.Check(second.Token, Holder{}); check.State != StateHeld {
		t.Fatalf("force mit alter Nummer hat freigegeben: %+v", check)
	}
}

func TestOrphanedCertainAndUncertain(t *testing.T) {
	cases := []struct {
		name   string
		prep   func(probe *fakeProbe, holder *Holder)
		expect string
	}{
		{"Sitzungsprozess läuft", func(*fakeProbe, *Holder) {}, OrphanedNo},
		{"Sitzungsprozess beendet", func(probe *fakeProbe, holder *Holder) { probe.kill(holder.Process.PID) }, OrphanedYes},
		{"PID neu vergeben", func(probe *fakeProbe, holder *Holder) { probe.run(holder.Process.PID, "999") }, OrphanedYes},
		{"geteilter Prozess läuft", func(_ *fakeProbe, holder *Holder) { holder.Process.Role = RoleShared }, OrphanedUncertain},
		{"geteilter Prozess beendet", func(probe *fakeProbe, holder *Holder) {
			holder.Process.Role = RoleShared
			probe.kill(holder.Process.PID)
		}, OrphanedUncertain},
		{"anderer Rechner", func(probe *fakeProbe, holder *Holder) {
			holder.Process.Host = "anderer"
			probe.kill(holder.Process.PID)
		}, OrphanedUncertain},
		{"anderer Systemstart", func(probe *fakeProbe, holder *Holder) {
			holder.Process.BootID = "boot-0"
			probe.kill(holder.Process.PID)
		}, OrphanedUncertain},
		{"anderer PID-Namensraum", func(probe *fakeProbe, holder *Holder) {
			holder.Process.PIDNamespace = "pid:[2]"
			probe.kill(holder.Process.PID)
		}, OrphanedUncertain},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			lock, probe := newLock(t)
			holder := sessionHolder(probe, 100, RoleSession)
			testCase.prep(probe, &holder)
			mustAcquire(t, lock, "", holder)
			status, _ := lock.Status()
			if status.Holder.Orphaned != testCase.expect {
				t.Fatalf("orphaned = %s (%s), erwartet %s", status.Holder.Orphaned, status.Holder.OrphanedWhy, testCase.expect)
			}
			next := mustAcquire(t, lock, "", sessionHolder(probe, 200, RoleSession))
			if taken := next.State == StateAcquired; taken != (testCase.expect == OrphanedYes) {
				t.Fatalf("Übernahme %v bei orphaned %s", taken, testCase.expect)
			}
		})
	}
}

func TestWaitTimeoutAndRelease(t *testing.T) {
	lock, probe := newLock(t)
	taken := mustAcquire(t, lock, "", sessionHolder(probe, 100, RoleSession))
	waiter := sessionHolder(probe, 200, RoleSession)

	start := time.Now()
	timedOut, err := lock.Wait(context.Background(), "", waiter, 150*time.Millisecond, 20*time.Millisecond)
	if err != nil || timedOut.State != StateBusy || timedOut.Holder == nil || timedOut.Holder.Session != "s-100" {
		t.Fatalf("Frist: %+v, %v", timedOut, err)
	}
	if elapsed := time.Since(start); elapsed < 150*time.Millisecond {
		t.Fatalf("Frist nicht abgewartet: %v", elapsed)
	}

	done := make(chan Result, 1)
	go func() {
		result, err := lock.Wait(context.Background(), "", waiter, 5*time.Second, 20*time.Millisecond)
		if err != nil {
			t.Error(err)
		}
		done <- result
	}()
	time.Sleep(100 * time.Millisecond)
	// Während des Wartens kommen check und release durch.
	if check, err := lock.Check(taken.Token, Holder{}); err != nil || check.State != StateHeld {
		t.Fatalf("check während wait: %+v, %v", check, err)
	}
	if released, err := lock.Release(taken.Token, false, 0); err != nil || released.State != StateReleased {
		t.Fatalf("release während wait: %+v, %v", released, err)
	}
	select {
	case result := <-done:
		if result.State != StateAcquired || result.Token == "" || result.Token == taken.Token {
			t.Fatalf("wait nach Freigabe: %+v", result)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("wait kehrt nach der Freigabe nicht zurück")
	}
}

func TestWaitCancelled(t *testing.T) {
	lock, probe := newLock(t)
	mustAcquire(t, lock, "", sessionHolder(probe, 100, RoleSession))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := lock.Wait(ctx, "", sessionHolder(probe, 200, RoleSession), time.Minute, 10*time.Millisecond); err == nil {
		t.Fatal("abgebrochenes wait ohne Fehler")
	}
}

func TestConcurrentAcquireOneWinner(t *testing.T) {
	lock, probe := newLock(t)
	var wg sync.WaitGroup
	results := make(chan Result, 20)
	for index := range 20 {
		holder := sessionHolder(probe, 1000+index, RoleSession)
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Je Aufruf eine eigene Lock-Instanz wie zwei Prozesse.
			result, err := New(lock.projectDir).WithProbe(probe).Acquire("", holder)
			if err != nil {
				t.Error(err)
			}
			results <- result
		}()
	}
	wg.Wait()
	close(results)
	winners := 0
	for result := range results {
		if result.State == StateAcquired {
			winners++
		}
	}
	if winners != 1 {
		t.Fatalf("%d Sitzungen haben genommen, erwartet genau eine", winners)
	}
}

func TestStatusWithoutDirCreatesNothing(t *testing.T) {
	lock, _ := newLock(t)
	status, err := lock.Status()
	if err != nil || status.State != StateFree {
		t.Fatalf("status: %+v, %v", status, err)
	}
	if _, err := os.Stat(filepath.Join(lock.projectDir, projectDirName)); !os.IsNotExist(err) {
		t.Fatalf("status hat etwas angelegt: %v", err)
	}
}

// Die Sperrdatei darf nie von `git add -A` erfasst werden, auch in einem
// Projekt, dessen .k-playbook/.gitignore work/ nicht ausnimmt.
func TestLockDirIgnoredByGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git fehlt")
	}
	dir := t.TempDir()
	gitCmd := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return string(out)
	}
	gitCmd("init", "-q")
	probe := newFakeProbe()
	lock := New(dir).WithProbe(probe)
	mustAcquire(t, lock, "", sessionHolder(probe, 100, RoleSession))
	gitCmd("add", "-A")
	if staged := gitCmd("diff", "--cached", "--name-only"); strings.TrimSpace(staged) != "" {
		t.Fatalf("git add -A erfasst: %s", staged)
	}
}

func TestFindHolderWalksToClient(t *testing.T) {
	root := t.TempDir()
	write := func(path, content string) {
		t.Helper()
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	stat := func(pid int, comm string, parent int, start string) string {
		fields := []string{strconv.Itoa(pid), "(" + comm + ")", "S", strconv.Itoa(parent)}
		for field := 5; field <= 21; field++ {
			fields = append(fields, "0")
		}
		return strings.Join(append(fields, start, "0"), " ")
	}
	write("sys/kernel/random/boot_id", "boot-x\n")
	if err := os.MkdirAll(filepath.Join(root, "self", "ns"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("pid:[7]", filepath.Join(root, "self", "ns", "pid")); err != nil {
		t.Fatal(err)
	}
	write("50/stat", stat(50, "sh (wrapper)", 40, "500"))
	write("40/stat", stat(40, "claude", 1, "400"))

	holder := findHolder(root, 50)
	if holder == nil || holder.PID != 40 || holder.Role != RoleSession || holder.Start != "400" || holder.BootID != "boot-x" || holder.PIDNamespace != "pid:[7]" {
		t.Fatalf("Halter: %+v", holder)
	}

	write("60/stat", stat(60, "opencode", 1, "600"))
	if shared := findHolder(root, 60); shared == nil || shared.Role != RoleShared {
		t.Fatalf("OpenCode: %+v", shared)
	}

	write("70/stat", stat(70, "bash", 1, "700"))
	if none := findHolder(root, 70); none != nil {
		t.Fatalf("ohne Client: %+v", none)
	}
}
