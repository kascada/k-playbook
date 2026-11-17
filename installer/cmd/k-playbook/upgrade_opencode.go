package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// Neuladen des OpenCode-2-Dienstes nach einem Binary-Tausch (Task 165).
//
// OpenCode 2 hat je Benutzer einen gemeinsamen Hintergrunddienst
// (`opencode serve --service`), der die stdio-MCP-Server startet, auch
// `k-playbook mcp`. Nach dem Tausch des Binarys läuft dort sonst weiter das
// alte Programm. `opencode reload` startet alle MCP-Server des Dienstes neu;
// der Dienst selbst und seine Sitzungen laufen unter derselben PID weiter.
//
// Nie `reload` ohne vorheriges `service status`: `reload` startet einen
// Dienst, wenn keiner läuft — auf dem Host ohne die Umgebung und die Grenzen
// der systemd-Unit. `service status` startet nichts und schreibt die URL des
// Dienstes oder `stopped` (gemessen an OpenCode 2.3.0: der Handler schreibt
// `url ?? "stopped"` und endet in beiden Fällen mit 0). Nur nach einer URL
// wird neu geladen, und ohne `--server`: Gemessen ist nur die einfache Form,
// und die Status-URL `http://0.0.0.0:4096` taugt als Ziel wenig.
//
// Keine Prüfung auf OpenCode 1: Es wird überall auf Version 2 umgestellt. Ein
// OpenCode 1 kennt `service` nicht und landet bei „Status unklar“; hängt es,
// fängt die Frist es ab.
//
// Nebenwirkung, bewusst in Kauf genommen: `reload` trifft alle MCP-Server des
// Dienstes, auch die anderer Projekte und Sitzungen desselben Benutzers und
// fremde wie Kephalaion. Einen Neustart von Kephalaion lädt dieser Schritt
// nicht nach; das erledigt `kephalaion upgrade` selbst.

// Die Fristen für `opencode service status` und `opencode reload`, und wie
// lange nach SIGTERM gewartet wird, bevor SIGKILL kommt. Variablen, damit ein
// Test den Zeitablauf ohne Wartezeit prüfen kann.
var (
	openCodeStatusTimeout = 10 * time.Second
	openCodeReloadTimeout = 30 * time.Second
	openCodeStopGrace     = 2 * time.Second
)

// Austauschbar, damit die Abfolge ohne echtes OpenCode prüfbar ist: die Suche
// nach opencode im PATH und der Aufruf mit Frist.
var (
	upgradeFindOpenCode = func() (string, bool) {
		path, err := exec.LookPath("opencode")
		return path, err == nil
	}
	upgradeRunOpenCode = runOpenCode
)

// Die Zustände von upgradeOpenCodeReport.Status.
const (
	openCodeStatusURL     = "url"
	openCodeStatusStopped = "stopped"
	openCodeStatusUnknown = "unknown"
)

// upgradeOpenCodeReport ist das Neuladen des OpenCode-Dienstes. Es fehlt im
// Bericht ohne Binary-Tausch, bei --check und --project-only und wenn opencode
// nicht im PATH liegt; sonst ist es da, auch mit Status stopped.
type upgradeOpenCodeReport struct {
	// Status ist url, stopped oder unknown — wie `opencode service status`
	// geantwortet hat.
	Status string `json:"status"`
	// URL ist die gemeldete Adresse des Dienstes, nur bei Status url.
	URL string `json:"url,omitempty"`
	// Reloaded: `opencode reload` ist erfolgreich gelaufen.
	Reloaded bool `json:"reloaded"`
	// Error nennt, warum der Status unklar ist oder der Reload scheiterte.
	Error string `json:"error,omitempty"`
}

// openCodeRun ist das Ergebnis eines Aufrufs von opencode.
type openCodeRun struct {
	stdout   string
	stderr   string
	exitCode int
	timedOut bool
}

// runOpenCode ruft binary mit args auf, mit Frist. Bei Fristablauf geht
// SIGTERM an den Prozess, SIGKILL erst nach openCodeStopGrace. Ein Fehler kommt
// nur, wenn der Prozess gar nicht startet.
func runOpenCode(ctx context.Context, binary string, timeout time.Duration, args ...string) (openCodeRun, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.WaitDelay = openCodeStopGrace
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	err := cmd.Run()
	run := openCodeRun{stdout: stdout.String(), stderr: stderr.String()}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		run.timedOut = true
		return run, nil
	}
	if err != nil && !errors.Is(err, exec.ErrWaitDelay) {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			run.exitCode = exitErr.ExitCode()
			return run, nil
		}
		return run, fmt.Errorf("%s nicht ausführbar: %w", binary, err)
	}
	return run, nil
}

// reloadOpenCode lädt die MCP-Server eines laufenden OpenCode-Dienstes neu.
// Ohne opencode im PATH kommt nil und keine Zeile. Sonst der Bericht und
// höchstens eine Zeile; bei stopped keine. Nichts davon hält den Lauf auf.
func reloadOpenCode(ctx context.Context) (*upgradeOpenCodeReport, []string) {
	binary, found := upgradeFindOpenCode()
	if !found {
		return nil, nil
	}
	report := &upgradeOpenCodeReport{}
	manual := "von Hand: opencode service status, dann opencode reload."

	run, err := upgradeRunOpenCode(ctx, binary, openCodeStatusTimeout, "service", "status")
	status, url, reason := classifyOpenCodeStatus(run, err)
	report.Status, report.URL = status, url
	switch status {
	case openCodeStatusStopped:
		return report, nil
	case openCodeStatusUnknown:
		report.Error = reason
		return report, []string{"OpenCode: Dienststatus unklar (" + reason + ") — nicht neu geladen; " + manual}
	}

	run, err = upgradeRunOpenCode(ctx, binary, openCodeReloadTimeout, "reload")
	if reason := openCodeRunFailure(run, err, openCodeReloadTimeout); reason != "" {
		report.Error = reason
		return report, []string{"OpenCode: Neuladen der MCP-Server gescheitert (" + reason + "); " + manual}
	}
	report.Reloaded = true
	return report, []string{"OpenCode: alle MCP-Server des Dienstes neu geladen (opencode reload)."}
}

// classifyOpenCodeStatus ordnet die Antwort von `opencode service status` ein:
// url mit der Adresse, stopped, oder unknown mit dem Kurzgrund.
func classifyOpenCodeStatus(run openCodeRun, err error) (status, url, reason string) {
	if reason := openCodeRunFailure(run, err, openCodeStatusTimeout); reason != "" {
		return openCodeStatusUnknown, "", reason
	}
	output := strings.TrimSpace(run.stdout)
	switch {
	case output == "stopped":
		return openCodeStatusStopped, "", ""
	case (strings.HasPrefix(output, "http://") || strings.HasPrefix(output, "https://")) &&
		!strings.ContainsAny(output, " \t\r\n"):
		return openCodeStatusURL, output, ""
	case output == "":
		return openCodeStatusUnknown, "", "keine Ausgabe"
	default:
		return openCodeStatusUnknown, "", fmt.Sprintf("unerwartete Ausgabe %q", firstLine(output, 80))
	}
}

// openCodeRunFailure sagt in einem Kurzgrund, warum ein Aufruf nicht gelang,
// oder "" bei Erfolg.
func openCodeRunFailure(run openCodeRun, err error, timeout time.Duration) string {
	switch {
	case err != nil:
		return err.Error()
	case run.timedOut:
		return fmt.Sprintf("keine Antwort nach %s", timeout)
	case run.exitCode != 0:
		reason := fmt.Sprintf("Exit-Code %d", run.exitCode)
		if detail := firstLine(strings.TrimSpace(run.stderr), 120); detail != "" {
			reason += ": " + detail
		}
		return reason
	}
	return ""
}

// firstLine ist die erste Zeile von text, auf höchstens limit Zeichen gekürzt.
func firstLine(text string, limit int) string {
	line, _, _ := strings.Cut(text, "\n")
	line = strings.TrimSpace(line)
	if runes := []rune(line); len(runes) > limit {
		line = string(runes[:limit]) + "…"
	}
	return line
}
