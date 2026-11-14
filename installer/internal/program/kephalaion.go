package program

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
	"github.com/kascada/k-playbook/installer/internal/release"
)

// KephalaionResult ist der Ausgang von InstallKephalaion.
type KephalaionResult struct {
	// Path ist der Ort von kephalaion: vorhanden oder gerade installiert.
	Path string `json:"path,omitempty"`
	// Installed: das install.sh von Kephalaion ist gelaufen.
	Installed bool `json:"installed,omitempty"`
	// Output ist die Ausgabe des install.sh, stdout und stderr zusammen.
	Output string `json:"output,omitempty"`
}

// InstallKephalaion sorgt dafür, dass kephalaion auf diesem Rechner liegt. Ist
// es da (kephalaion.FindBinary), geschieht nichts — kein Abruf und auch keine
// Aktualisierung, die macht kephalaion upgrade.
//
// Sonst lädt es das install.sh aus dem neuesten Kephalaion-Release
// (release.KephalaionInstallURL) und führt es aus, mit demselben Zeitlimit wie
// die Installation des Programms. Einen Clone braucht es dafür nicht: Das
// Skript bin/install-kephalaion des Clones ruft dieses Programm nicht mehr auf.
// Asset-Namen, Plattformen und Prüfsumme bleiben Sache von Kephalaion. Erst
// ganz laden, dann ausführen: Bricht der Download ab, läuft kein halbes
// Skript, und ein Fehler beim Laden ist von einem Fehler der Installation zu
// unterscheiden.
//
// node init, service install und Token ruft diese Funktion nicht auf: Im
// DevContainer kommt die config read-only vom Host, und der Node ist der des
// Hosts. Das install.sh von Kephalaion tut aber mehr als das Binary abzulegen:
// Es ruft danach `kephalaion node mcp add --auto` auf und, wenn ein Editor-CLI
// im PATH liegt, `kephalaion vscode install --code` — es meldet den Node also
// bei den Assistenten an und installiert die Erweiterung.
//
// Erfolg heißt nicht Exit 0, sondern: danach findet FindBinary kephalaion. Das
// install.sh von Kephalaion installiert nach ~/.local/bin.
//
// Die Adresse ist wie die Release-Quelle austauschbar (release.EnvSource), auch
// für gebaute Unterprozesse; kein Test lädt oder startet das echte install.sh.
func InstallKephalaion(ctx context.Context) (KephalaionResult, error) {
	result := KephalaionResult{}
	if path, ok := kephalaion.FindBinary(); ok {
		result.Path = path
		return result, nil
	}

	ctx, cancel := context.WithTimeout(ctx, InstallTimeout)
	defer cancel()

	work, err := os.MkdirTemp("", "k-playbook-kephalaion-")
	if err != nil {
		return result, fmt.Errorf("Arbeitsverzeichnis für das install.sh von Kephalaion nicht anlegbar: %w", err)
	}
	defer os.RemoveAll(work)

	script := filepath.Join(work, "install.sh")
	if err := release.Fetch(ctx, release.CurrentSource().KephalaionInstall, script); err != nil {
		return result, fmt.Errorf("das install.sh von Kephalaion ließ sich nicht laden: %w", err)
	}

	result.Installed = true
	output, err := runKephalaionInstall(ctx, script)
	result.Output = output
	if err != nil {
		return result, err
	}
	path, ok := kephalaion.FindBinary()
	if !ok {
		return result, errors.New("kephalaion liegt nach der Installation weder im PATH noch unter ~/.local/bin oder /usr/local/bin")
	}
	result.Path = path
	return result, nil
}

// kephalaionInstallLabel ist das install.sh, wie die Meldungen es nennen.
const kephalaionInstallLabel = "das install.sh von Kephalaion"

// runKephalaionInstall führt das geladene install.sh mit sh aus — so, wie
// kephalaion.InstallCommand es einem Nutzer sagt (curl … | sh). Ohne stdin:
// Das Skript fragt nichts.
func runKephalaionInstall(ctx context.Context, script string) (string, error) {
	cmd := exec.CommandContext(ctx, "sh", script)
	cmd.Dir = filepath.Dir(script)
	cmd.Env = withoutServeMarker(os.Environ())
	// Ohne WaitDelay hielte ein Enkelprozess — curl — die Pipe nach dem
	// Abbruch offen, und CombinedOutput kehrte erst mit ihm zurück.
	cmd.WaitDelay = 5 * time.Second
	output, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(output))

	if ctx.Err() == context.DeadlineExceeded {
		return text, fmt.Errorf("%s hat nach %s nicht geendet", kephalaionInstallLabel, InstallTimeout)
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return text, fmt.Errorf("%s endete mit Exit-Code %d", kephalaionInstallLabel, exitErr.ExitCode())
		}
		return text, fmt.Errorf("%s nicht ausführbar: %v", kephalaionInstallLabel, err)
	}
	return text, nil
}

// Fristen für die Aufrufe von kephalaion upgrade. Variablen, damit Tests den
// Weg über die Frist gehen können.
var (
	// KephalaionAutoTimeout begrenzt `kephalaion upgrade --auto`: Frage an
	// GitHub, Download, Probe (30 s), Frage an die Hubs (je 5 s), Neustart
	// mit Prüfung (10 s) — dieselben 3 Minuten wie InstallTimeout.
	KephalaionAutoTimeout = 3 * time.Minute
	// KephalaionCheckTimeout begrenzt `kephalaion upgrade --check`: eine
	// Frage an GitHub.
	KephalaionCheckTimeout = 30 * time.Second
	// KephalaionStopGrace ist die Wartezeit nach SIGTERM bei Fristablauf.
	// Erst danach kommt SIGKILL: ein SIGKILL während Ersetzen, Neustart oder
	// Rückweg hebelte die Zusicherungen von Kephalaion aus. 30 s deckt die
	// Prüfung nach dem Neustart (10 s) samt Rückweg.
	KephalaionStopGrace = 30 * time.Second
)

// Werte von outcome in der Antwort von `kephalaion upgrade --auto --json`.
const (
	KephalaionUpgraded = "upgraded"
	KephalaionCurrent  = "current"
	KephalaionSkipped  = "skipped"
	KephalaionFailed   = "failed"
	// KephalaionReasonRestart ist der reason von failed, wenn der Dienst
	// nach dem Neustart nicht mit der Zielversion anlief; der Node wurde
	// dann neu gestartet, auch wenn restarted fehlt.
	KephalaionReasonRestart = "restart"
)

// KephalaionCheck ist die Antwort von `kephalaion upgrade --check --json` —
// und das Feld check der Antwort von --auto. Vertrag: docs/installation.md
// von Kephalaion, „Upgrade: was Kephalaion meldet".
type KephalaionCheck struct {
	State           string `json:"state"`
	Error           string `json:"error,omitempty"`
	CheckedAt       string `json:"checked_at,omitempty"`
	Version         string `json:"version,omitempty"`
	DevBuild        bool   `json:"dev_build,omitempty"`
	Latest          string `json:"latest,omitempty"`
	UpdateAvailable bool   `json:"update_available,omitempty"`
	SelfUpgrade     bool   `json:"self_upgrade,omitempty"`
	Method          string `json:"method,omitempty"`
	Command         string `json:"command,omitempty"`
	Hint            string `json:"hint,omitempty"`
}

// KephalaionAuto ist die Antwort von `kephalaion upgrade --auto --json`.
// Ausgewertet wird nur, was Kephalaion meldet; k-playbook entscheidet nichts
// über Hub, config oder Installationsart.
type KephalaionAuto struct {
	Outcome string `json:"outcome"`
	Reason  string `json:"reason,omitempty"`
	From    string `json:"from,omitempty"`
	To      string `json:"to,omitempty"`
	// Restarted steht nur, wenn ein Dienst pro User lief.
	Restarted bool `json:"restarted,omitempty"`
	// RolledBack steht nur bei failed, restart.
	RolledBack *bool            `json:"rolled_back,omitempty"`
	Warnings   []string         `json:"warnings,omitempty"`
	Error      string           `json:"error,omitempty"`
	Hint       string           `json:"hint,omitempty"`
	Check      *KephalaionCheck `json:"check,omitempty"`
}

// NodeRestarted sagt, ob der Lauf den Node neu gestartet hat: restarted, und
// bei failed, restart immer — dort hat Kephalaion neu gestartet, auch wenn
// das Feld fehlt.
func (a KephalaionAuto) NodeRestarted() bool {
	return a.Restarted || (a.Outcome == KephalaionFailed && a.Reason == KephalaionReasonRestart)
}

// ErrKephalaionAutoUnsupported: das Binary endete bei --auto mit Exit 2 —
// nach dem Vertrag ein Binary, das --auto noch nicht kennt. Der Aufrufer fällt
// auf `upgrade --check --json` zurück.
var ErrKephalaionAutoUnsupported = errors.New("dieses kephalaion kennt upgrade --auto noch nicht")

// KephalaionNoJSONError: der Aufruf lieferte auf stdout kein lesbares JSON.
type KephalaionNoJSONError struct {
	Command  string
	ExitCode int
	Stdout   string
	Stderr   string
}

func (e *KephalaionNoJSONError) Error() string {
	message := fmt.Sprintf("%s lieferte kein lesbares JSON (Exit-Code %d)", e.Command, e.ExitCode)
	if output := strings.TrimSpace(strings.Join(nonEmpty(e.Stderr, e.Stdout), "\n")); output != "" {
		message += ": " + output
	}
	return message
}

// KephalaionUnknownOutcomeError: der Aufruf hat die Frist überschritten und
// wurde beendet, oder er ließ sich nicht abwarten. Was er bis dahin getan hat,
// ist offen — nie „nichts geändert".
type KephalaionUnknownOutcomeError struct {
	Command string
	Timeout time.Duration
	Stderr  string
}

func (e *KephalaionUnknownOutcomeError) Error() string {
	message := fmt.Sprintf("%s hat nach %s nicht geendet und wurde beendet — Ausgang unbekannt", e.Command, e.Timeout)
	if stderr := strings.TrimSpace(e.Stderr); stderr != "" {
		message += ": " + stderr
	}
	return message
}

// KephalaionAutoUpgrade ruft `<binary> upgrade --auto --json` auf. Exit 0 und
// 1 mit lesbarem JSON auf stdout sind ein Ergebnis (auch failed); Exit 2 ist
// ErrKephalaionAutoUnsupported; sonst KephalaionNoJSONError, bei Fristablauf
// KephalaionUnknownOutcomeError.
func KephalaionAutoUpgrade(binary string) (KephalaionAuto, error) {
	var result KephalaionAuto
	command := binary + " upgrade --auto --json"
	run, err := runKephalaion(binary, KephalaionAutoTimeout, "upgrade", "--auto", "--json")
	if err != nil {
		return result, err
	}
	if run.timedOut {
		return result, &KephalaionUnknownOutcomeError{Command: command, Timeout: KephalaionAutoTimeout, Stderr: run.stderr}
	}
	if run.exitCode == 2 {
		return result, ErrKephalaionAutoUnsupported
	}
	if (run.exitCode != 0 && run.exitCode != 1) || json.Unmarshal([]byte(run.stdout), &result) != nil || result.Outcome == "" {
		return KephalaionAuto{}, &KephalaionNoJSONError{Command: command, ExitCode: run.exitCode, Stdout: run.stdout, Stderr: run.stderr}
	}
	return result, nil
}

// KephalaionCheckUpgrade ruft `<binary> upgrade --check --json` auf. Das
// schreibt nichts. Exit 0 und 1 mit lesbarem JSON sind ein Ergebnis (auch
// state failed); sonst KephalaionNoJSONError, bei Fristablauf
// KephalaionUnknownOutcomeError.
func KephalaionCheckUpgrade(binary string) (KephalaionCheck, error) {
	var result KephalaionCheck
	command := binary + " upgrade --check --json"
	run, err := runKephalaion(binary, KephalaionCheckTimeout, "upgrade", "--check", "--json")
	if err != nil {
		return result, err
	}
	if run.timedOut {
		return result, &KephalaionUnknownOutcomeError{Command: command, Timeout: KephalaionCheckTimeout, Stderr: run.stderr}
	}
	if (run.exitCode != 0 && run.exitCode != 1) || json.Unmarshal([]byte(run.stdout), &result) != nil || result.State == "" {
		return KephalaionCheck{}, &KephalaionNoJSONError{Command: command, ExitCode: run.exitCode, Stdout: run.stdout, Stderr: run.stderr}
	}
	return result, nil
}

// kephalaionRun ist ein beendeter Aufruf von kephalaion.
type kephalaionRun struct {
	stdout   string
	stderr   string
	exitCode int
	timedOut bool
}

// runKephalaion ruft binary mit args auf, stdout und stderr getrennt. Bei
// Fristablauf geht SIGTERM an den Prozess, nicht SIGKILL wie beim Standard
// von exec.CommandContext; SIGKILL kommt erst, wenn er nach
// KephalaionStopGrace noch läuft. Ein Fehler kommt nur, wenn der Prozess gar
// nicht startet.
func runKephalaion(binary string, timeout time.Duration, args ...string) (kephalaionRun, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Env = withoutServeMarker(os.Environ())
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.WaitDelay = KephalaionStopGrace
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	err := cmd.Run()
	run := kephalaionRun{stdout: stdout.String(), stderr: stderr.String()}
	if ctx.Err() == context.DeadlineExceeded {
		run.timedOut = true
		return run, nil
	}
	if err != nil && !errors.Is(err, exec.ErrWaitDelay) {
		// ErrWaitDelay: der Prozess endete mit 0, ein Kindprozess hielt die
		// Ausgabe offen; was bis dahin kam, gilt.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			run.exitCode = exitErr.ExitCode()
			return run, nil
		}
		return run, fmt.Errorf("%s nicht ausführbar: %w", binary, err)
	}
	return run, nil
}

func nonEmpty(values ...string) []string {
	result := []string{}
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			result = append(result, strings.TrimSpace(value))
		}
	}
	return result
}
