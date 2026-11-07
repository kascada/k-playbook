package github

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Hält ein Kindprozess die Pipes offen, kehrt ExecRunner trotzdem kurz nach der
// Frist zurück. Der Abbruch tötet nur den Prozess selbst; ohne WaitDelay
// wartete Run, bis auch das Kind endet — hier 5 s.
func TestExecRunnerKehrtTrotzOffenerPipesZurueck(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("braucht sh")
	}
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh fehlt")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	started := time.Now()
	_, err := ExecRunner{}.Run(ctx, t.TempDir(), "sh", "-c", "sleep 5 & wait")
	elapsed := time.Since(started)

	if err == nil {
		t.Fatal("kein Fehler, obwohl die Frist abgelaufen ist")
	}
	if elapsed > 100*time.Millisecond+waitDelay+2*time.Second {
		t.Errorf("Rückkehr nach %s: Run wartete auf den Kindprozess", elapsed)
	}
	var commandErr *CommandError
	if !errors.As(err, &commandErr) {
		t.Errorf("Fehler %T, erwartet *CommandError", err)
	}
}

// envRecorder hält Argumente und Umgebung jedes Aufrufs fest.
type envRecorder struct {
	args [][]string
	envs [][]string
}

func (e *envRecorder) Run(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	return e.RunEnv(ctx, dir, nil, name, args...)
}

func (e *envRecorder) RunEnv(_ context.Context, _ string, env []string, name string, args ...string) ([]byte, error) {
	e.args = append(e.args, append([]string{name}, args...))
	e.envs = append(e.envs, env)
	return []byte(`{"login":"kamran"}`), nil
}

// Mit Env gehen die gh-Aufrufe des Clients mit dieser Umgebung los; der
// Token steht dort und in keinem Argument. Die git-Aufrufe lesen nur den
// Arbeitsstand und laufen ohne sie.
func TestClientGibtGHDieUmgebungMit(t *testing.T) {
	const token = "ghp_fake_client_token"
	env := []string{"PATH=/usr/bin", "GH_TOKEN=" + token, "GITHUB_TOKEN=" + token}
	recorder := &envRecorder{}
	client := &Client{Runner: recorder, Dir: t.TempDir(), Timeout: time.Second, Env: env}

	if _, err := client.gh(context.Background(), "api", "user"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.git(context.Background(), "remote", "get-url", "origin"); err != nil {
		t.Fatal(err)
	}
	if len(recorder.envs) != 2 || len(recorder.envs[0]) != len(env) || recorder.envs[1] != nil {
		t.Fatalf("Umgebungen: %q", recorder.envs)
	}
	for _, args := range recorder.args {
		for _, arg := range args {
			if strings.Contains(arg, token) {
				t.Errorf("Token im Argument: %q", args)
			}
		}
	}
}

// Ein Runner ohne RunEnv bekommt keinen Aufruf mit Umgebung: Er liefe sonst
// still mit der des Servers, also über das aktive gh-Konto.
func TestRunWithEnvVerlangtEinenEnvRunner(t *testing.T) {
	runner := &fakeRunner{answers: []fakeAnswer{{prefix: "gh", out: "{}"}}}
	_, err := RunWithEnv(context.Background(), runner, "", []string{"GH_TOKEN=x"}, "gh", "api", "user")
	if err == nil || !strings.Contains(err.Error(), "aktive gh-Konto") {
		t.Errorf("Fehler %v", err)
	}
	if len(runner.calls) != 0 {
		t.Errorf("Aufrufe: %q", runner.calls)
	}
	if _, err := RunWithEnv(context.Background(), runner, "", nil, "gh", "api", "user"); err != nil {
		t.Errorf("ohne Umgebung: %v", err)
	}
}

// ExecRunner setzt die Umgebung wirklich: Das Kind sieht genau sie.
func TestExecRunnerSetztDieUmgebung(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("braucht sh")
	}
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh fehlt")
	}
	out, err := ExecRunner{}.RunEnv(context.Background(), t.TempDir(), []string{"K_PLAYBOOK_ENV_PROBE=126"}, sh, "-c", "echo \"$K_PLAYBOOK_ENV_PROBE:${HOME:-leer}\"")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(out)); got != "126:leer" {
		t.Errorf("Ausgabe %q, erwartet 126:leer", got)
	}
}
