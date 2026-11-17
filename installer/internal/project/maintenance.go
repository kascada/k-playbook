package project

import (
	"context"
	"fmt"
	"io"
	"strings"
)

// Die Schritte des Wartungslaufs `k-playbook maintain` (Task 161) stehen hier
// genau einmal, mit Name und Reihenfolge: --only, maintenance.daily, die
// Vorgabe des Tagesprogramms und die Hilfe beziehen die Namen von dort. Ein
// späterer Schritt — etwa das Abarbeiten der Queues mit KI — kommt als weiterer
// Eintrag in maintenanceSteps dazu, ohne das Gerüst umzubauen.

// MaintenanceStepDocsBackup ist der Schritt, der keph:/docs/ in den Ordner
// maintenance.backup.target exportiert und ihn auf Wunsch committet.
const MaintenanceStepDocsBackup = "docs-backup"

// DefaultMaintenanceDaily ist das Tagesprogramm, wenn maintenance.daily fehlt.
var DefaultMaintenanceDaily = []string{MaintenanceStepDocsBackup}

// MaintenanceStatus ist der Ausgang eines Schritts.
type MaintenanceStatus string

const (
	// MaintenanceOK: der Schritt hat getan, was er sollte (in der Vorschau:
	// gezeigt, was er täte).
	MaintenanceOK MaintenanceStatus = "erfolgreich"
	// MaintenanceSkipped: der Schritt ist abgeschaltet; kein Fehler.
	MaintenanceSkipped MaintenanceStatus = "übersprungen"
	// MaintenanceFailed: der Schritt ist gescheitert; das Gesamtergebnis ist
	// dann „Fehler“.
	MaintenanceFailed MaintenanceStatus = "Fehler"
)

// MaintenanceStepResult ist das Ergebnis eines Schritts mit Grund.
type MaintenanceStepResult struct {
	Step   string            `json:"step"`
	Status MaintenanceStatus `json:"status"`
	Reason string            `json:"reason"`
	// Notes sind weitere Sätze zum Schritt, etwa der erste Lauf oder ein
	// möglicherweise teilweise aktualisierter lokaler Stand.
	Notes []string `json:"notes,omitempty"`
}

func stepOK(step, reason string, notes ...string) MaintenanceStepResult {
	return MaintenanceStepResult{Step: step, Status: MaintenanceOK, Reason: reason, Notes: notes}
}

func stepSkipped(step, reason string) MaintenanceStepResult {
	return MaintenanceStepResult{Step: step, Status: MaintenanceSkipped, Reason: reason}
}

func stepFailed(step, reason string, notes ...string) MaintenanceStepResult {
	return MaintenanceStepResult{Step: step, Status: MaintenanceFailed, Reason: reason, Notes: notes}
}

// maintenanceRun ist der Zustand eines Laufs, den jeder Schritt bekommt.
type maintenanceRun struct {
	ctx        context.Context
	projectDir string
	config     Config
	dryRun     bool
	// commit ist --commit: eine Git-Sicherung ausdrücklich angefordert.
	commit bool
	out    io.Writer
	errOut io.Writer
}

// progress schreibt eine Zeile des Laufs nach out.
func (r *maintenanceRun) progress(format string, args ...any) {
	fmt.Fprintf(r.out, format+"\n", args...)
}

// maintenanceStep ist ein Schritt des Wartungslaufs.
type maintenanceStep struct {
	name    string
	summary string
	run     func(*maintenanceRun) MaintenanceStepResult
}

// maintenanceSteps ist die Liste der Schritte in der Reihenfolge, in der ein
// Lauf sie ausführt — gleich, in welcher Reihenfolge --only oder
// maintenance.daily sie nennen.
func maintenanceSteps() []maintenanceStep {
	return []maintenanceStep{
		{
			name:    MaintenanceStepDocsBackup,
			summary: "sichert keph:/docs/ nach maintenance.backup.target und committet auf Wunsch nur dieses Ziel",
			run:     runDocsBackup,
		},
	}
}

// MaintenanceStepInfo nennt einen Schritt für Hilfe und Meldungen.
type MaintenanceStepInfo struct {
	Name    string `json:"name"`
	Summary string `json:"summary"`
}

// MaintenanceSteps sind die Schritte in ihrer festen Reihenfolge.
func MaintenanceSteps() []MaintenanceStepInfo {
	var infos []MaintenanceStepInfo
	for _, step := range maintenanceSteps() {
		infos = append(infos, MaintenanceStepInfo{Name: step.name, Summary: step.summary})
	}
	return infos
}

// MaintenanceStepNames sind die gültigen Namen in ihrer Reihenfolge.
func MaintenanceStepNames() []string {
	var names []string
	for _, step := range maintenanceSteps() {
		names = append(names, step.name)
	}
	return names
}

// MaintenanceOptions steuert einen Lauf.
type MaintenanceOptions struct {
	// Only ist die Auswahl aus --only; nil heißt Tagesprogramm.
	Only []string
	// DryRun zeigt, was geschähe, ohne zu schreiben.
	DryRun bool
	// Commit fordert die Git-Sicherung an; eine abgeschaltete Sicherung
	// schaltet es nicht ein.
	Commit bool
	Out    io.Writer
	ErrOut io.Writer
}

// MaintenanceResult ist das Ergebnis eines Laufs.
type MaintenanceResult struct {
	// Selection sagt, woher die Auswahl kam: --only, maintenance.daily oder
	// die Vorgabe des Tagesprogramms.
	Selection string                  `json:"selection"`
	Steps     []MaintenanceStepResult `json:"steps"`
	// Failed ist das Gesamtergebnis „Fehler“: mindestens ein Schritt ist
	// gescheitert.
	Failed bool `json:"failed"`
}

// MaintenanceSelectionError ist eine ungültige Auswahl: ein unbekannter
// Schritt in --only oder maintenance.daily. Abgelehnt wird vor jedem
// Schreiben; die Meldung nennt die gültigen Namen.
type MaintenanceSelectionError struct {
	message string
}

func (e *MaintenanceSelectionError) Error() string { return e.message }

// checkMaintenanceNames prüft Namen gegen die Liste der Schritte.
func checkMaintenanceNames(names []string, origin string) error {
	valid := MaintenanceStepNames()
	var unknown []string
	for _, name := range names {
		if !containsString(valid, name) {
			unknown = append(unknown, name)
		}
	}
	if len(unknown) == 0 {
		return nil
	}
	return &MaintenanceSelectionError{message: fmt.Sprintf("unbekannter Schritt in %s: %s — gültig: %s; nichts ausgeführt",
		origin, strings.Join(unknown, ", "), strings.Join(valid, ", "))}
}

// RunMaintenance führt den Wartungslauf aus: ohne Auswahl das Tagesprogramm
// (maintenance.daily, ohne Feld die Vorgabe), sonst die Schritte aus Only, in
// der festen Reihenfolge von maintenanceSteps. Kein Schritt fragt nach, keiner
// ruft eine KI auf; ohne Terminal verhält sich der Lauf gleich.
//
// Vor jedem Schreiben abgelehnt werden: eine unlesbare oder fremde
// Konfiguration, ein Block maintenance:, der sich nicht lesen lässt, und ein
// unbekannter Name in --only oder maintenance.daily (MaintenanceSelectionError).
// Danach bekommt jeder Schritt sein eigenes Ergebnis; das Gesamtergebnis ist
// Fehler, sobald einer scheitert.
func RunMaintenance(ctx context.Context, projectDir string, options MaintenanceOptions) (MaintenanceResult, error) {
	result := MaintenanceResult{Steps: []MaintenanceStepResult{}}
	if options.Out == nil {
		options.Out = io.Discard
	}
	if options.ErrOut == nil {
		options.ErrOut = io.Discard
	}

	config, err := ReadConfig(projectDir)
	if err != nil {
		return result, err
	}
	if err := CheckSchema(config); err != nil {
		return result, err
	}
	if config.Maintenance.DailySet {
		if err := checkMaintenanceNames(config.Maintenance.Daily, "maintenance.daily"); err != nil {
			return result, err
		}
	}
	if problems := config.Maintenance.Problems; len(problems) > 0 {
		return result, fmt.Errorf("der Block %s in %s ist nicht lesbar — %s; nichts ausgeführt",
			MaintenanceBlockName, ConfigDisplayName, strings.Join(problems, "; "))
	}

	var selected []string
	switch {
	case options.Only != nil:
		result.Selection = "--only"
		for _, name := range options.Only {
			if name = strings.TrimSpace(name); name != "" {
				selected = append(selected, name)
			}
		}
		if len(selected) == 0 {
			return result, &MaintenanceSelectionError{message: "--only ohne Schritt — gültig: " +
				strings.Join(MaintenanceStepNames(), ", ") + "; nichts ausgeführt"}
		}
		if err := checkMaintenanceNames(selected, "--only"); err != nil {
			return result, err
		}
	case config.Maintenance.DailySet:
		result.Selection = "maintenance.daily"
		selected = config.Maintenance.Daily
	default:
		result.Selection = "Vorgabe des Tagesprogramms"
		selected = DefaultMaintenanceDaily
	}

	run := &maintenanceRun{ctx: ctx, projectDir: projectDir, config: config, dryRun: options.DryRun,
		commit: options.Commit, out: options.Out, errOut: options.ErrOut}
	for _, step := range maintenanceSteps() {
		if !containsString(selected, step.name) {
			continue
		}
		stepResult := step.run(run)
		result.Steps = append(result.Steps, stepResult)
		if stepResult.Status == MaintenanceFailed {
			result.Failed = true
		}
	}
	return result, nil
}
