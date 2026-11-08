// Package github fragt den Stand eines GitHub-Repos über das gh-Binary ab.
//
// Bewusst ein eigenes Paket neben `internal/project`: dort steht mit `gh.go`
// der **netzfreie** Host-Befund — liegt gh im PATH, welche Konten sind
// hinterlegt —, und der soll billig genug bleiben, um in jeder Kontextausgabe
// zu stehen. Hier dagegen laufen Subprozesse gegen die API, mit Zeitlimit,
// Fehlereinordnung und JSON-Auswertung. Zwei Zuständigkeiten, zwei Pakete.
//
// Gelesen wird ausschließlich. Kein Aufruf dieses Pakets verändert etwas an
// GitHub.
package github

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// DefaultTimeout ist die Frist je Abfrage. Jede Karte der Seite hat einen
// eigenen Endpunkt, damit eine langsame Antwort die anderen nicht aufhält;
// die Frist begrenzt, wie lange eine einzelne warten lässt. Sie gilt je
// Aufruf; die Frist einer ganzen Anfrage setzt der Aufrufer über den Kontext
// (in der Oberfläche `githubBudgets`, mit demselben Wert).
const DefaultTimeout = 20 * time.Second

// LogTimeout gilt für das Log eines Laufs. Es ist die teuerste Abfrage —
// gh lädt das Archiv der fehlgeschlagenen Jobs — und wird nur beim Aufklappen
// eines roten Laufs geholt, nie beim Laden der Seite.
const LogTimeout = 60 * time.Second

// waitDelay begrenzt, wie lange ExecRunner nach dem Abbruch noch auf die Pipes
// wartet. Ohne ihn endet ein Aufruf erst, wenn auch jeder Kindprozess von gh
// seine Pipes schließt: der Abbruch tötet nur gh selbst. Am Testserver lief so
// ein Endpunkt mit 20 s Budget 90 s — bis der Kindprozess von selbst endete.
const waitDelay = time.Second

// Runner führt ein Kommando aus und gibt dessen Standardausgabe zurück.
//
// Das Interface ist der Grund, warum die Tests dieses Pakets ohne Netz und
// ohne installiertes gh laufen: sie setzen einen Runner ein, der Fixtures
// zurückgibt.
type Runner interface {
	Run(ctx context.Context, dir string, name string, args ...string) ([]byte, error)
}

// EnvRunner ist ein Runner, der einem Kommando eine eigene Umgebung mitgibt —
// die Kindumgebung eines Dienstes der Zugangsdatei (githubaccess.ChildEnv),
// mit der die Oberfläche GitHub so erreicht wie `k-playbook github run`. env
// ersetzt die Umgebung ganz, wie exec.Cmd.Env; nil heißt die des Servers. Ein
// Token steht nur in env, nie in den Argumenten.
type EnvRunner interface {
	RunEnv(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error)
}

// RunWithEnv führt ein Kommando mit env aus. Ohne env genügt jeder Runner.
// Mit env muss der Runner sie annehmen können; einer, der es nicht kann,
// bekommt das Kommando nicht — sonst liefe es still mit der Umgebung des
// Servers und damit über das aktive gh-Konto.
func RunWithEnv(ctx context.Context, runner Runner, dir string, env []string, name string, args ...string) ([]byte, error) {
	if env == nil {
		return runner.Run(ctx, dir, name, args...)
	}
	envRunner, ok := runner.(EnvRunner)
	if !ok {
		return nil, &CommandError{Name: name, Args: args, ExitCode: -1, Err: errors.New("der Runner nimmt keine Umgebung an; ohne sie liefe der Aufruf über das aktive gh-Konto")}
	}
	return envRunner.RunEnv(ctx, dir, env, name, args...)
}

// CommandError trägt, was ein gescheitertes Kommando gesagt hat. Die Stderr
// bleibt erhalten, weil erst sie die Einordnung erlaubt (401, 403, Rate-Limit).
type CommandError struct {
	Name     string
	Args     []string
	ExitCode int
	Stderr   string
	Err      error
}

func (e *CommandError) Error() string {
	text := strings.TrimSpace(e.Stderr)
	if text == "" {
		text = e.Err.Error()
	}
	return fmt.Sprintf("%s %s: %s", e.Name, strings.Join(e.Args, " "), text)
}

func (e *CommandError) Unwrap() error { return e.Err }

// ExecRunner startet die Kommandos wirklich. Der Standardfall außerhalb der
// Tests.
type ExecRunner struct{}

func (r ExecRunner) Run(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	return r.RunEnv(ctx, dir, nil, name, args...)
}

// RunEnv ist Run mit eigener Umgebung; nil heißt die des Servers.
func (ExecRunner) RunEnv(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, name, args...)
	command.Dir = dir
	command.Env = env
	command.WaitDelay = waitDelay
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	err := command.Run()
	if err != nil {
		exitCode := -1
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		}
		return stdout.Bytes(), &CommandError{
			Name:     name,
			Args:     args,
			ExitCode: exitCode,
			Stderr:   stderr.String(),
			Err:      err,
		}
	}
	return stdout.Bytes(), nil
}

// Client bündelt Runner, Arbeitsverzeichnis und Frist. Er hält keinen Zustand
// zwischen zwei Abfragen: wie überall in der Oberfläche liest jede Anfrage neu.
type Client struct {
	Runner Runner
	// Dir ist das Verzeichnis, in dem gh und git laufen. Aus ihm löst gh das
	// Repo auf — auch dann, wenn das Remote über einen SSH-Alias läuft, den
	// ein eigenes Parsen der URL nicht auflösen könnte.
	Dir     string
	Timeout time.Duration
	// Env ist die Umgebung der gh-Aufrufe: mit einem Dienst der Zugangsdatei
	// dessen Kindumgebung (Token in GH_TOKEN/GITHUB_TOKEN, nie in einem
	// Argument), ohne Dienst nil und damit die des Servers. Die git-Aufrufe
	// des Clients lesen nur den Arbeitsstand und laufen ohne sie: Unter den
	// Umlenkungen des Dienstes nennte `git remote get-url` die Zielform von
	// run statt des eingetragenen Remotes. Der Runner muss Env annehmen
	// können (EnvRunner), sonst startet kein gh.
	Env []string
}

// NewClient ist der Standardfall: echte Kommandos, Standardfrist. env ist
// die Umgebung der gh-Aufrufe, nil die des Servers.
func NewClient(dir string, env []string) *Client {
	return &Client{Runner: ExecRunner{}, Dir: dir, Timeout: DefaultTimeout, Env: env}
}

func (c *Client) timeout() time.Duration {
	if c.Timeout > 0 {
		return c.Timeout
	}
	return DefaultTimeout
}

// gh ruft das gh-Binary mit der Frist des Clients auf.
func (c *Client) gh(ctx context.Context, args ...string) ([]byte, error) {
	return c.run(ctx, c.timeout(), c.Env, "gh", args...)
}

// ghWithTimeout ruft gh mit einer eigenen Frist auf. Nur das Log eines Laufs
// braucht das: es ist die einzige Abfrage, die spürbar länger dauern darf.
func (c *Client) ghWithTimeout(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
	return c.run(ctx, timeout, c.Env, "gh", args...)
}

// git ruft git auf. Tag und Commits seit dem Tag kommen aus dem Arbeitsstand
// und nicht aus der API: sie stehen lokal, und ein Netzaufruf dafür wäre
// verschenkt.
func (c *Client) git(ctx context.Context, args ...string) ([]byte, error) {
	return c.run(ctx, c.timeout(), nil, "git", args...)
}

func (c *Client) run(ctx context.Context, timeout time.Duration, env []string, name string, args ...string) ([]byte, error) {
	runner := c.Runner
	if runner == nil {
		runner = ExecRunner{}
	}
	// Ist das Budget der Anfrage schon verbraucht, startet kein Prozess mehr:
	// er würde sofort abgebrochen und kostete nur den Start.
	if err := ctx.Err(); err != nil {
		return nil, &CommandError{Name: name, Args: args, ExitCode: -1, Err: err}
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	out, err := RunWithEnv(callCtx, runner, c.Dir, env, name, args...)
	if err != nil {
		// Ein Prozess, den der Kontext beendet hat, meldet sich als
		// „signal: killed": os/exec zieht den Exit-Status dem Kontextfehler
		// vor. Die Ursache steht deshalb nur noch am Kontext — hier wird sie an
		// den Fehler gehängt, damit Classify sie mit errors.Is findet, egal was
		// gh vorher auf stderr geschrieben hat. Das gilt für die Frist dieses
		// Aufrufs wie für das Budget der Anfrage darüber, und ebenso für den
		// Abbruch durch den Browser.
		if cause := callCtx.Err(); cause != nil {
			return out, &contextError{err: err, cause: cause}
		}
	}
	return out, err
}

// contextError ist ein Fehler, dessen Aufruf an seinem Kontext gescheitert ist.
// Der Text bleibt der des Kommandos; errors.Is findet den Kontextfehler, und
// errors.As findet weiterhin den CommandError mit seiner Stderr.
type contextError struct {
	err   error
	cause error
}

func (e *contextError) Error() string { return e.err.Error() }

func (e *contextError) Unwrap() []error { return []error{e.cause, e.err} }
