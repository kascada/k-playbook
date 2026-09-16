// Package github fragt den Stand eines GitHub-Repos über das gh-Binary ab.
//
// Bewusst ein eigenes Paket neben `internal/project`: dort steht mit `gh.go`
// der **netzfreie** Host-Befund — liegt gh im PATH, welche Konten sind
// hinterlegt —, und der soll billig genug bleiben, um in jeder Kontextausgabe
// zu stehen. Hier dagegen laufen Subprozesse gegen die API, mit Zeitlimit,
// Fehlereinordnung und JSON-Auswertung. Zwei Zuständigkeiten, zwei Pakete.
//
// Gelesen wird ausschließlich. Kein Aufruf dieses Pakets verändert etwas an
// GitHub; Approve, Merge und Kommentare bleiben bei /k-pr-review.
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

func (ExecRunner) Run(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, name, args...)
	command.Dir = dir
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
}

// NewClient ist der Standardfall: echte Kommandos, Standardfrist.
func NewClient(dir string) *Client {
	return &Client{Runner: ExecRunner{}, Dir: dir, Timeout: DefaultTimeout}
}

func (c *Client) timeout() time.Duration {
	if c.Timeout > 0 {
		return c.Timeout
	}
	return DefaultTimeout
}

// gh ruft das gh-Binary mit der Frist des Clients auf.
func (c *Client) gh(ctx context.Context, args ...string) ([]byte, error) {
	return c.run(ctx, c.timeout(), "gh", args...)
}

// ghWithTimeout ruft gh mit einer eigenen Frist auf. Nur das Log eines Laufs
// braucht das: es ist die einzige Abfrage, die spürbar länger dauern darf.
func (c *Client) ghWithTimeout(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
	return c.run(ctx, timeout, "gh", args...)
}

// git ruft git auf. Tag und Commits seit dem Tag kommen aus dem Arbeitsstand
// und nicht aus der API: sie stehen lokal, und ein Netzaufruf dafür wäre
// verschenkt.
func (c *Client) git(ctx context.Context, args ...string) ([]byte, error) {
	return c.run(ctx, c.timeout(), "git", args...)
}

func (c *Client) run(ctx context.Context, timeout time.Duration, name string, args ...string) ([]byte, error) {
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
	out, err := runner.Run(callCtx, c.Dir, name, args...)
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
