package branches

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// fetchEnvRunner hält Argumente und Umgebung des Fetch fest und meldet ihn
// als gelungen.
type fetchEnvRunner struct {
	args [][]string
	envs [][]string
}

func (f *fetchEnvRunner) Run(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	return f.RunEnv(ctx, dir, nil, name, args...)
}

func (f *fetchEnvRunner) RunEnv(_ context.Context, _ string, env []string, name string, args ...string) ([]byte, error) {
	f.args = append(f.args, append([]string{name}, args...))
	f.envs = append(f.envs, env)
	return []byte("From github.com:kascada/k-playbook\n"), nil
}

// Der Fetch über einen Dienst bekommt dessen Kindumgebung; der Token steht
// nur dort — weder in den Argumenten noch in der Antwort.
func TestFetchMitUmgebung(t *testing.T) {
	const token = "ghp_fake_fetch_token"
	env := []string{"PATH=/usr/bin", "GH_TOKEN=" + token, "GIT_TERMINAL_PROMPT=0"}
	runner := &fetchEnvRunner{}

	result := Fetch(context.Background(), runner, t.TempDir(), env)
	if !result.OK {
		t.Fatalf("Fetch: %+v", result)
	}
	if len(runner.envs) != 1 || len(runner.envs[0]) != len(env) || runner.envs[0][1] != "GH_TOKEN="+token {
		t.Fatalf("Umgebung des Fetch: %q", runner.envs)
	}
	if strings.Join(runner.args[0], " ") != "git fetch --all --prune" {
		t.Errorf("Aufruf: %q", runner.args[0])
	}
	data, _ := json.Marshal(result)
	if strings.Contains(string(data), token) {
		t.Errorf("Token in der Antwort: %s", data)
	}
}

// Ohne Umgebung bleibt es beim Bisherigen: kein Env, also die des Servers.
func TestFetchOhneUmgebung(t *testing.T) {
	runner := &fetchEnvRunner{}
	Fetch(context.Background(), runner, t.TempDir(), nil)
	if len(runner.envs) != 1 || runner.envs[0] != nil {
		t.Errorf("Umgebung: %q", runner.envs)
	}
}
