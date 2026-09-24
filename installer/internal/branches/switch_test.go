package branches

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestUmschaltenErfolg(t *testing.T) {
	f := newFixture(t)
	ctx := testContext(t)
	options := f.checkOptions("feature/b")
	check := RunCheck(ctx, options)
	result := Switch(ctx, options, check.Stamp, nil)
	if !result.Executed || !result.OK {
		t.Fatalf("result = %+v", result)
	}
	if result.Before.Branch != "main" || result.After.Branch != "feature/b" || result.After.SHA != check.TargetSHA {
		t.Errorf("Before = %+v, After = %+v", result.Before, result.After)
	}
	if !strings.Contains(result.Output, "feature/b") {
		t.Errorf("Ausgabe von git fehlt: %q", result.Output)
	}
	if got := run(t, f.work, "branch", "--show-current"); got != "feature/b" {
		t.Errorf("ausgecheckt = %q", got)
	}
}

// Nur remote: der lokale Branch entsteht als Tracking-Branch.
func TestUmschaltenNurRemote(t *testing.T) {
	f := newFixture(t)
	ctx := testContext(t)
	options := f.checkOptions("remediation/x")
	result := Switch(ctx, options, RunCheck(ctx, options).Stamp, nil)
	if !result.OK || result.Command != "git switch --no-overwrite-ignore --track origin/remediation/x" {
		t.Fatalf("result = %+v", result)
	}
	if upstream := run(t, f.work, "rev-parse", "--abbrev-ref", "remediation/x@{upstream}"); upstream != "origin/remediation/x" {
		t.Errorf("Upstream = %q", upstream)
	}
}

// Ein veralteter Stempel führt nichts aus und gibt die neue Prüfung zurück.
func TestUmschaltenStempelVeraltet(t *testing.T) {
	f := newFixture(t)
	ctx := testContext(t)
	options := f.checkOptions("feature/b")
	stamp := RunCheck(ctx, options).Stamp

	// Das Ziel bewegt sich zwischen Prüfung und Ausführung.
	run(t, f.work, "branch", "-f", "feature/b", "main")
	result := Switch(ctx, options, stamp, nil)
	if result.Executed || result.Reason != SwitchReasonStamp || result.Check.Stamp == stamp || len(result.Check.Checks) == 0 {
		t.Errorf("result = %+v", result)
	}
	if got := run(t, f.work, "branch", "--show-current"); got != "main" {
		t.Errorf("trotzdem umgeschaltet: %q", got)
	}
	if result := Switch(ctx, options, "", nil); result.Executed {
		t.Error("ohne Stempel umgeschaltet")
	}
}

// Eine Datei ändert sich zwischen Prüfung und Ausführung: Stempel passt nicht,
// und die neue Prüfung blockiert am Arbeitsbaum.
func TestUmschaltenDateiZwischendurchGeaendert(t *testing.T) {
	f := newFixture(t)
	ctx := testContext(t)
	options := f.checkOptions("feature/b")
	stamp := RunCheck(ctx, options).Stamp

	writeFile(t, filepath.Join(f.work, "README.md"), "zwischendurch\n")
	result := Switch(ctx, options, stamp, nil)
	if result.Executed || result.Reason != SwitchReasonStamp {
		t.Fatalf("result = %+v", result)
	}
	if check := checkByID(t, result.Check, "arbeitsbaum"); check.Result != ResultBlocked {
		t.Errorf("arbeitsbaum = %+v", check)
	}
	// Auch mit dem neuen Stempel wird nicht umgeschaltet: es blockiert.
	blocked := Switch(ctx, options, result.Check.Stamp, nil)
	if blocked.Executed || blocked.Reason != SwitchReasonBlocked {
		t.Errorf("blocked = %+v", blocked)
	}
}

// git verweigert — hier wegen einer liegengebliebenen index.lock, die die
// Prüfung nicht sieht. Die Antwort trägt die Ausgabe, und es wird nichts
// weiter versucht.
func TestUmschaltenGitVerweigert(t *testing.T) {
	f := newFixture(t)
	ctx := testContext(t)
	options := f.checkOptions("feature/b")
	check := RunCheck(ctx, options)
	if !check.Offered {
		t.Fatalf("nicht angeboten: %+v", check.Checks)
	}
	gitDir := run(t, f.work, "rev-parse", "--absolute-git-dir")
	lock := filepath.Join(gitDir, "index.lock")
	if err := os.WriteFile(lock, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(lock)

	result := Switch(ctx, options, check.Stamp, nil)
	if !result.Executed || result.OK || result.Reason != SwitchReasonGitFailed {
		t.Fatalf("result = %+v", result)
	}
	if !strings.Contains(result.Output, "index.lock") || !strings.Contains(result.Message, "nichts weiter versucht") {
		t.Errorf("Output = %q, Message = %q", result.Output, result.Message)
	}
	if result.After.Branch != "main" {
		t.Errorf("After = %+v", result.After)
	}
}

// Zweite Sicherung neben der Vorprüfung: Der ausgeführte Befehl trägt
// --no-overwrite-ignore. Übersieht die Prüfung eine ignorierte Datei, bricht git
// ab, statt sie zu überschreiben. Geprüft wird der Befehl selbst, an der Prüfung
// vorbei, für beide Formen: lokal und --track.
func TestUmschaltbefehlUeberschreibtKeineIgnorierteDatei(t *testing.T) {
	f := newFixture(t)
	ctx := testContext(t)

	// mit-env gibt es lokal, nur-remote-env nur auf dem Remote; beide
	// versionieren .env, main ignoriert sie.
	run(t, f.seed, "switch", "-q", "-c", "nur-remote-env", "main")
	writeFile(t, filepath.Join(f.seed, ".env"), "GEHEIM=remote\n")
	run(t, f.seed, "add", "-f", ".env")
	run(t, f.seed, "commit", "-q", "-m", ".env versioniert")
	run(t, f.seed, "push", "-q", "origin", "nur-remote-env")
	run(t, f.seed, "switch", "-q", "main")
	run(t, f.work, "fetch", "-q", "origin")
	run(t, f.work, "switch", "-q", "-c", "mit-env")
	writeFile(t, filepath.Join(f.work, ".env"), "GEHEIM=aus-dem-branch\n")
	run(t, f.work, "add", "-f", ".env")
	run(t, f.work, "commit", "-q", "-m", ".env versioniert")
	run(t, f.work, "switch", "-q", "main")
	commit(t, f.work, ".gitignore", ".env\n", "ignoriert .env")
	envPath := filepath.Join(f.work, ".env")
	writeFile(t, envPath, "GEHEIM=lokal\n")

	for _, target := range []struct {
		name    string
		command string
	}{
		{"mit-env", "git switch --no-overwrite-ignore mit-env"},
		{"nur-remote-env", "git switch --no-overwrite-ignore --track origin/nur-remote-env"},
	} {
		check := RunCheck(ctx, f.checkOptions(target.name))
		expectResult(t, check, "ignorierte-dateien", ResultBlocked)
		if check.Command != target.command || "git "+strings.Join(check.Args, " ") != target.command {
			t.Fatalf("Command = %q, Args = %v, erwartet %q", check.Command, check.Args, target.command)
		}
		command := exec.Command("git", check.Args...)
		command.Dir = f.work
		out, err := command.CombinedOutput()
		if err == nil {
			t.Errorf("%s: git hat umgeschaltet: %s", target.command, out)
		}
		if !strings.Contains(string(out), ".env") {
			t.Errorf("%s: Ausgabe nennt .env nicht: %s", target.command, out)
		}
		if content, _ := os.ReadFile(envPath); string(content) != "GEHEIM=lokal\n" {
			t.Fatalf("%s: .env überschrieben: %q", target.command, content)
		}
		if got := run(t, f.work, "branch", "--show-current"); got != "main" {
			t.Fatalf("%s: ausgecheckt = %q", target.command, got)
		}
	}
}
