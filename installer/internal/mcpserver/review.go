package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kascada/k-playbook/installer/internal/project"
	"github.com/kascada/k-playbook/installer/internal/review"
	"github.com/kascada/k-playbook/installer/internal/review/merge"
)

const timeLayout = time.RFC3339

type reviewBaseInput struct {
	ProjectDir string `json:"projectDir" jsonschema:"Pflicht. Verzeichnis, ab dem aufwärts nach K-PLAYBOOK.yaml gesucht wird. Relative Pfade werden relativ zum Arbeitsverzeichnis des MCP-Servers aufgelöst."`
}

type reviewStatusInput struct {
	reviewBaseInput
	Run  string `json:"run,omitempty"`
	Mode string `json:"mode,omitempty"`
}

type reviewSelectionInput struct {
	Name string      `json:"name"`
	Kind review.Kind `json:"kind"`
}

type reviewCreateInput struct {
	reviewBaseInput
	Entries []reviewSelectionInput `json:"entries,omitempty"`
	Day     string                 `json:"day,omitempty"`
	DryRun  bool                   `json:"dryRun,omitempty"`
}

type reviewScanInput struct {
	reviewBaseInput
	Run     string   `json:"run"`
	Entries []string `json:"entries,omitempty"`
}

type reviewMergeInput struct {
	reviewBaseInput
	Run string `json:"run"`
}

type reviewWriteAIEntryInput struct {
	reviewBaseInput
	Run        string       `json:"run"`
	Entry      string       `json:"entry"`
	State      review.State `json:"state"`
	Result     string       `json:"result,omitempty"`
	Reason     string       `json:"reason,omitempty"`
	StartedAt  string       `json:"startedAt,omitempty"`
	FinishedAt string       `json:"finishedAt,omitempty"`
	// Job ist die Fertigmeldung eines Evidence-Eintrags: der Ort des SARIF und
	// die Zeiten seines Laufs. Er gehört zu mode: evidence und zu state: done —
	// für eine Perspektive gibt es keinen Job, und ein noch laufender Eintrag
	// hat sein Artefakt noch nicht.
	Job *reviewAIJobInput `json:"job,omitempty"`
}

// reviewAIJobInput ist der Job-Teil der Ergebnismeldung. Er trägt dieselben
// Angaben wie review.JobStatus, soweit ein Assistent sie kennt: Zustand,
// Fundzahl und Job-Name entstehen beim Melden und werden nicht entgegengenommen.
type reviewAIJobInput struct {
	SARIF    string `json:"sarif" jsonschema:"Pflicht. Ort des SARIF, relativ zum Laufverzeichnis und unterhalb von raw/. Für einen Evidence-Eintrag ist das raw/<entry>.sarif."`
	Started  string `json:"started,omitempty" jsonschema:"Optionale Startzeit des Rezeptlaufs im RFC3339-Format."`
	Finished string `json:"finished,omitempty" jsonschema:"Optionale Endzeit des Rezeptlaufs im RFC3339-Format."`
}

type reviewCandidate struct {
	Name              string      `json:"name"`
	Kind              review.Kind `json:"kind"`
	Title             string      `json:"title"`
	Selectable        bool        `json:"selectable"`
	DefaultSelected   bool        `json:"defaultSelected"`
	UnavailableReason string      `json:"unavailableReason"`
	Detail            string      `json:"detail,omitempty"`
	Languages         string      `json:"languages,omitempty"`
	Status            string      `json:"status,omitempty"`
	Path              string      `json:"path,omitempty"`
	RecipeKey         string      `json:"recipeKey,omitempty"`
	RecipePath        string      `json:"recipePath,omitempty"`
	RecipeOrigin      string      `json:"recipeOrigin,omitempty"`
	// Mode ist die Betriebsart eines AI-Rezepts: perspective oder evidence.
	// Leer bei Tool-Kandidaten.
	Mode           review.Mode   `json:"mode,omitempty"`
	AuditEnabled   *bool         `json:"auditEnabled,omitempty"`
	ReviewEnabled  *bool         `json:"reviewEnabled,omitempty"`
	ResultRequired *bool         `json:"resultRequired,omitempty"`
	DefaultResult  string        `json:"defaultResult,omitempty"`
	RuleIDs        []string      `json:"ruleIds,omitempty"`
	Scope          *review.Scope `json:"scope,omitempty"`
}

type reviewSelectionBase struct {
	Languages             []string          `json:"languages"`
	Candidates            []reviewCandidate `json:"candidates"`
	UnavailableCandidates []reviewCandidate `json:"unavailableCandidates"`
	DefaultEntries        []review.Entry    `json:"defaultEntries"`
	// EvidenceCandidates und PerspectiveCandidates trennen die auswählbaren
	// AI-Rezepte nach Betriebsart. Beide Listen stehen neben Candidates und
	// nicht an deren Stelle: Evidence läuft vor dem Merge, Perspektiven danach,
	// und diese Reihenfolge soll aus der Ausgabe ablesbar sein, ohne dass ein
	// Command sie aus den Rezepten neu ableitet.
	EvidenceCandidates    []string `json:"evidenceCandidates"`
	PerspectiveCandidates []string `json:"perspectiveCandidates"`
	Preflight             any      `json:"preflight,omitempty"`
}

type aiRecipeMetadata struct {
	Enabled       bool
	ReviewEnabled bool
	Title         string
	// Mode ist audit.mode aus dem Rezept, normalisiert auf perspective oder
	// evidence.
	Mode           review.Mode
	ResultRequired bool
	// ResultRequiredSet meldet, ob audit.resultRequired im Rezept steht.
	// ResultRequired allein sagt das nicht: sein Vorgabewert ist true.
	ResultRequiredSet bool
	DefaultResult     string
	// RuleIDs ist die abschließende Rule-ID-Liste eines Evidence-Rezepts.
	RuleIDs []string
	Scope   *review.Scope
}

// auditContract macht aus den gelesenen Metadaten die Form, über die
// review.ValidateAuditContract entscheidet.
func (m aiRecipeMetadata) auditContract() review.AuditContract {
	return review.AuditContract{
		Mode:              m.Mode,
		Scope:             m.Scope,
		RuleIDs:           m.RuleIDs,
		ResultRequiredSet: m.ResultRequiredSet,
		DefaultResult:     m.DefaultResult,
	}
}

type reviewProjectEnvelope struct {
	InputDir      string   `json:"inputDir"`
	Root          string   `json:"root,omitempty"`
	PlaybookDir   string   `json:"playbookDir,omitempty"`
	LocalDir      string   `json:"localDir,omitempty"`
	ReviewRunsDir string   `json:"reviewRunsDir,omitempty"`
	Languages     []string `json:"languages,omitempty"`
}

type reviewErrorEnvelope struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details"`
}

type reviewEnvelope struct {
	OK       bool                  `json:"ok"`
	Tool     string                `json:"tool"`
	Project  reviewProjectEnvelope `json:"project"`
	Data     any                   `json:"data,omitempty"`
	Error    *reviewErrorEnvelope  `json:"error,omitempty"`
	Warnings []string              `json:"warnings"`
}

type reviewToolError struct {
	Code    string
	Message string
	Details map[string]any
}

func (e reviewToolError) Error() string { return e.Message }

type reviewEnvironment struct {
	InputDir    string
	Root        string
	PlaybookDir string
	LocalDir    string
	TargetDir   string
	Languages   []string
}

const (
	reviewToolStatus       = "k_playbook_review_status"
	reviewToolCreate       = "k_playbook_review_create"
	reviewToolScan         = "k_playbook_review_scan"
	reviewToolMerge        = "k_playbook_review_merge"
	reviewToolWriteAIEntry = "k_playbook_review_write_ai_entry"
	scanTriageEntry        = "scan-triage"
	scanTriageModule       = "_audit/review-scan-triage.md"
	scanTriageResult       = "review-triage.md"
	reviewInputJSON        = "review-input.json"
	reviewInputMarkdown    = "review-input.md"
)

func addReviewTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        reviewToolStatus,
		Description: "Liest den Status eines Review-Laufs oder berechnet die Auswahlbasis für einen neuen Lauf.",
	}, reviewStatusTool)
	mcp.AddTool(server, &mcp.Tool{
		Name:        reviewToolCreate,
		Description: "Legt einen Review-Lauf an oder gibt im Dry-Run die validierte run.json-Struktur zurück.",
	}, reviewCreateTool)
	mcp.AddTool(server, &mcp.Tool{
		Name:        reviewToolScan,
		Description: "Führt Werkzeug-Einträge eines Review-Laufs über die bestehende Scan-Fachlogik aus.",
	}, reviewScanTool)
	mcp.AddTool(server, &mcp.Tool{
		Name:        reviewToolMerge,
		Description: "Fasst einen Review-Lauf über die bestehende Merge-Fachlogik zu review-input.* zusammen.",
	}, reviewMergeTool)
	mcp.AddTool(server, &mcp.Tool{
		Name:        reviewToolWriteAIEntry,
		Description: "Schreibt den Status eines AI-Review-Eintrags in dessen eigene Entry-Datei.",
	}, reviewWriteAIEntryTool)
}

func reviewStatusTool(ctx context.Context, req *mcp.CallToolRequest, input reviewStatusInput) (*mcp.CallToolResult, any, error) {
	return wrapReviewTool(reviewToolStatus, input.ProjectDir, func(env reviewEnvironment) *mcp.CallToolResult {
		mode := strings.TrimSpace(input.Mode)
		if mode == "" {
			if strings.TrimSpace(input.Run) == "" {
				mode = "available"
			} else {
				mode = "existing"
			}
		}
		switch mode {
		case "available":
			data, toolErr := reviewAvailableStatus(env)
			if toolErr.Code != "" {
				return reviewErrorResult(reviewToolStatus, env.projectEnvelope(), toolErr)
			}
			return reviewSuccessResult(reviewToolStatus, env.projectEnvelope(), data, nil)
		case "existing":
			if strings.TrimSpace(input.Run) == "" {
				return reviewErrorResult(reviewToolStatus, env.projectEnvelope(), reviewToolError{
					Code:    "invalid_mode",
					Message: "Modus existing braucht einen Lauf.",
					Details: map[string]any{"mode": mode},
				})
			}
			data, toolErr := reviewExistingStatus(env, input.Run)
			if toolErr.Code != "" {
				return reviewErrorResult(reviewToolStatus, env.projectEnvelope(), toolErr)
			}
			return reviewSuccessResult(reviewToolStatus, env.projectEnvelope(), data, nil)
		default:
			return reviewErrorResult(reviewToolStatus, env.projectEnvelope(), reviewToolError{
				Code:    "invalid_mode",
				Message: "Unbekannter Statusmodus.",
				Details: map[string]any{"mode": mode, "allowed": []string{"existing", "available"}},
			})
		}
	}), nil, nil
}

func reviewCreateTool(ctx context.Context, req *mcp.CallToolRequest, input reviewCreateInput) (*mcp.CallToolResult, any, error) {
	return wrapReviewTool(reviewToolCreate, input.ProjectDir, func(env reviewEnvironment) *mcp.CallToolResult {
		data, toolErr := createReviewRun(env, input)
		if toolErr.Code != "" {
			return reviewErrorResult(reviewToolCreate, env.projectEnvelope(), toolErr)
		}
		return reviewSuccessResult(reviewToolCreate, env.projectEnvelope(), data, nil)
	}), nil, nil
}

func reviewScanTool(ctx context.Context, req *mcp.CallToolRequest, input reviewScanInput) (*mcp.CallToolResult, any, error) {
	return wrapReviewTool(reviewToolScan, input.ProjectDir, func(env reviewEnvironment) *mcp.CallToolResult {
		data, toolErr := scanReviewRun(ctx, req, env, input)
		if toolErr.Code != "" {
			return reviewErrorResult(reviewToolScan, env.projectEnvelope(), toolErr)
		}
		return reviewSuccessResult(reviewToolScan, env.projectEnvelope(), data, nil)
	}), nil, nil
}

func reviewMergeTool(ctx context.Context, req *mcp.CallToolRequest, input reviewMergeInput) (*mcp.CallToolResult, any, error) {
	return wrapReviewTool(reviewToolMerge, input.ProjectDir, func(env reviewEnvironment) *mcp.CallToolResult {
		data, warnings, toolErr := mergeReviewRun(env, input)
		if toolErr.Code != "" {
			return reviewErrorResult(reviewToolMerge, env.projectEnvelope(), toolErr)
		}
		return reviewSuccessResult(reviewToolMerge, env.projectEnvelope(), data, warnings)
	}), nil, nil
}

func reviewWriteAIEntryTool(ctx context.Context, req *mcp.CallToolRequest, input reviewWriteAIEntryInput) (*mcp.CallToolResult, any, error) {
	return wrapReviewTool(reviewToolWriteAIEntry, input.ProjectDir, func(env reviewEnvironment) *mcp.CallToolResult {
		data, toolErr := writeAIEntry(env, input)
		if toolErr.Code != "" {
			return reviewErrorResult(reviewToolWriteAIEntry, env.projectEnvelope(), toolErr)
		}
		return reviewSuccessResult(reviewToolWriteAIEntry, env.projectEnvelope(), data, nil)
	}), nil, nil
}

func reviewNotImplemented(tool string, inputDir string) *mcp.CallToolResult {
	return reviewErrorResult(tool, reviewProjectEnvelope{InputDir: inputDir}, reviewToolError{
		Code:    "execution_failed",
		Message: "Werkzeug noch nicht implementiert.",
		Details: map[string]any{"tool": tool},
	})
}

func reviewAvailableStatus(env reviewEnvironment) (map[string]any, reviewToolError) {
	base, toolErr := buildSelectionBase(env)
	if toolErr.Code != "" {
		return nil, toolErr
	}
	runs, err := review.ListRuns(env.LocalDir)
	if err != nil {
		return nil, wrapError(err, "read_failed", "Review-Läufe sind nicht lesbar.", map[string]any{"path": review.ResultsDir(env.LocalDir)})
	}
	today := time.Now().Format(review.DateLayout)
	todayExists := false
	for _, run := range runs {
		if run.Name == today {
			todayExists = true
			break
		}
	}
	return map[string]any{
		"mode":        "available",
		"today":       today,
		"todayExists": todayExists,
		"runs":        runs,
		"selection":   base,
	}, reviewToolError{}
}

func createReviewRun(env reviewEnvironment, input reviewCreateInput) (map[string]any, reviewToolError) {
	base, toolErr := buildSelectionBase(env)
	if toolErr.Code != "" {
		return nil, toolErr
	}
	day := time.Now()
	if strings.TrimSpace(input.Day) != "" {
		parsed, err := time.ParseInLocation(review.DateLayout, input.Day, time.Local)
		if err != nil {
			return nil, reviewToolError{
				Code:    "invalid_selection",
				Message: "Datum ist ungültig.",
				Details: map[string]any{"day": input.Day, "layout": review.DateLayout},
			}
		}
		day = parsed
	}
	runName := day.Format(review.DateLayout)
	selected, toolErr := validateCreateSelection(base, input.Entries)
	if toolErr.Code != "" {
		return nil, toolErr
	}
	preparedRun := review.Run{
		SchemaVersion: review.SchemaVersion,
		Created:       day.Format(timeLayout),
		State:         review.StateCreated,
		Languages:     append([]string{}, env.Languages...),
		Entries:       selected,
	}
	data := map[string]any{
		"run":                     runName,
		"runDir":                  runDirFor(env, runName),
		"dryRun":                  input.DryRun,
		"selectedEntries":         selected,
		"runJSON":                 preparedRun,
		"languages":               append([]string{}, env.Languages...),
		"validatedCandidates":     candidatesForEntries(base.Candidates, selected),
		"unavailableCandidates":   base.UnavailableCandidates,
		"effectiveSelectionBasis": base,
	}
	if input.DryRun {
		return data, reviewToolError{}
	}
	if runExists(runDirFor(env, runName)) {
		return nil, reviewToolError{
			Code:    "run_exists",
			Message: "Für dieses Datum gibt es bereits einen Lauf.",
			Details: map[string]any{"run": runName, "runDir": runDirFor(env, runName)},
		}
	}
	runDir, err := review.CreateRun(env.LocalDir, day, env.Languages, selected)
	if err != nil {
		code := "write_failed"
		message := "Review-Lauf konnte nicht angelegt werden."
		if os.IsExist(err) || strings.Contains(err.Error(), "bereits") {
			code = "run_exists"
			message = "Für dieses Datum gibt es bereits einen Lauf."
		}
		return nil, wrapError(err, code, message, map[string]any{"run": runName, "runDir": runDirFor(env, runName)})
	}
	if err := writeInitialScanTriageEntry(runDir, selected); err != nil {
		return nil, wrapError(err, "write_failed", "Scan-Triage-Eintrag konnte nicht angelegt werden.", map[string]any{"run": runName, "path": review.EntryFile(runDir, scanTriageEntry)})
	}
	written, err := review.ReadRun(runDir)
	if err != nil {
		return nil, wrapError(err, "read_failed", "Geschriebener Lauf ist nicht lesbar.", map[string]any{"run": runName, "path": filepath.Join(runDir, review.RunFileName)})
	}
	data["runDir"] = runDir
	data["runJSON"] = written
	return data, reviewToolError{}
}

func scanReviewRun(ctx context.Context, req *mcp.CallToolRequest, env reviewEnvironment, input reviewScanInput) (map[string]any, reviewToolError) {
	runDir, run, toolErr := requireRun(env, input.Run)
	if toolErr.Code != "" {
		return nil, toolErr
	}
	entries, toolErr := validateScanSelection(runDir, run, input.Entries)
	if toolErr.Code != "" {
		return nil, toolErr
	}
	if len(entries) == 0 {
		return map[string]any{
			"run":     input.Run,
			"runDir":  runDir,
			"state":   review.DeriveRunState(runDir, run),
			"entries": []review.EntryStatus{},
			"message": "Keine Werkzeug-Einträge auszuführen.",
		}, reviewToolError{}
	}
	scanners, err := review.LoadScanners(review.ScannerCatalog(env.PlaybookDir))
	if err != nil {
		return nil, wrapError(err, "read_failed", "Scanner-Katalog ist nicht lesbar.", map[string]any{"path": review.ScannerCatalog(env.PlaybookDir)})
	}
	preflight, err := project.CheckTools(env.Root, run.Languages)
	if err != nil {
		return nil, wrapError(err, "preflight_failed", "Tool-Preflight fehlgeschlagen.", map[string]any{"projectDir": env.Root, "languages": run.Languages})
	}
	emitter := newReviewProgressEmitter(ctx, req, runDir, entries)
	finishMessage := "scan complete"
	var progress func(string, review.JobStatus)
	if emitter != nil {
		progress = emitter.Report
		defer func() { emitter.Stop(finishMessage) }()
	}
	statuses, err := review.Execute(ctx, entries, review.Options{
		RunDir:     runDir,
		Target:     env.TargetDir,
		ScriptsDir: filepath.Join(env.PlaybookDir, "scripts"),
		Languages:  run.Languages,
		Scanners:   scanners,
		Tools:      resolveReviewTools(preflight),
		Progress:   progress,
	})
	if err != nil {
		finishMessage = "scan failed"
		return nil, wrapError(err, "execution_failed", "Scan-Ausführung ist abgebrochen.", map[string]any{"run": input.Run})
	}
	return map[string]any{
		"run":     input.Run,
		"runDir":  runDir,
		"state":   review.DeriveRunState(runDir, run),
		"entries": statuses,
	}, reviewToolError{}
}

// mergeReviewRun gibt neben den Daten die Warnungen zum Laden der
// known-decisions.md zurück. Sie gehören in den Warnings-Slot des Envelopes:
// /k-audit mergt ausschließlich über MCP, hier ist der einzige Weg, auf dem sie
// den Agenten erreichen.
func mergeReviewRun(env reviewEnvironment, input reviewMergeInput) (map[string]any, []string, reviewToolError) {
	runDir, run, toolErr := requireRun(env, input.Run)
	if toolErr.Code != "" {
		return nil, nil, toolErr
	}
	result, output, err := merge.Run(merge.Options{
		ProjectDir:          env.Root,
		RunName:             input.Run,
		RunDir:              runDir,
		KPlaybookVersion:    mcpKPlaybookVersion(),
		SeverityMappingPath: merge.SeverityCatalog(env.PlaybookDir),
		LocalDir:            env.LocalDir,
	})
	if err != nil {
		code := "merge_failed"
		message := "Review-Lauf konnte nicht zusammengeführt werden."
		if isNotExist(err) {
			code = "read_failed"
			message = "Merge-Eingabe ist nicht lesbar."
		}
		return nil, nil, wrapError(err, code, message, map[string]any{"run": input.Run, "runDir": runDir})
	}
	return map[string]any{
		"run":    input.Run,
		"runDir": runDir,
		"outputs": map[string]string{
			"reviewInputJSON":     output.JSON,
			"reviewInputMarkdown": output.Markdown,
		},
		"summary": map[string]any{
			"findings":    len(result.Findings),
			"groups":      len(result.Groups),
			"entryStates": countStatusEntries(result.Entries),
			"state":       review.DeriveRunState(runDir, run),
		},
	}, result.KnownDecisions.Warnings, reviewToolError{}
}

func writeAIEntry(env reviewEnvironment, input reviewWriteAIEntryInput) (map[string]any, reviewToolError) {
	runDir, run, toolErr := requireRun(env, input.Run)
	if toolErr.Code != "" {
		return nil, toolErr
	}
	entry, found := findRunEntry(run, input.Entry)
	if !found {
		var ok bool
		entry, ok = repairableScanTriageEntry(env, input.Entry)
		if !ok {
			return nil, reviewToolError{
				Code:    "entry_not_found",
				Message: "Eintrag ist nicht im Lauf enthalten.",
				Details: map[string]any{"entry": input.Entry, "run": input.Run, "known": runEntryNames(run)},
			}
		}
	}
	if entry.Kind != review.KindAI {
		return nil, reviewToolError{
			Code:    "entry_kind_invalid",
			Message: "Nur AI-Einträge können so geschrieben werden.",
			Details: map[string]any{"entry": input.Entry, "kind": entry.Kind, "expectedKind": review.KindAI},
		}
	}
	if toolErr := validateAIEntryInput(runDir, entry, input); toolErr.Code != "" {
		return nil, toolErr
	}
	aiStatus := aiEntryStatus{
		Name:       input.Entry,
		Kind:       review.KindAI,
		State:      input.State,
		Result:     input.Result,
		Reason:     input.Reason,
		StartedAt:  input.StartedAt,
		FinishedAt: input.FinishedAt,
	}
	outcome, toolErr := evaluateEvidenceJob(runDir, entry, input)
	if toolErr.Code != "" {
		return nil, toolErr
	}
	if outcome.Present {
		aiStatus.Jobs = []review.JobStatus{outcome.Job}
		aiStatus.State = outcome.State
		aiStatus.Reason = joinReason(input.Reason, outcome.Note)
	}
	if err := writeAIEntryStatusFile(runDir, aiStatus); err != nil {
		return nil, wrapError(err, "write_failed", "AI-Entry-Status konnte nicht geschrieben werden.", map[string]any{"entry": input.Entry, "path": review.EntryFile(runDir, input.Entry)})
	}
	data := map[string]any{
		"run":      input.Run,
		"runDir":   runDir,
		"entry":    input.Entry,
		"path":     review.EntryFile(runDir, input.Entry),
		"status":   aiStatus,
		"runState": review.DeriveRunState(runDir, effectiveRunForStatus(env, run)),
	}
	if outcome.Present {
		data["evidence"] = map[string]any{
			"sarif":           outcome.Job.SARIF,
			"findings":        outcome.Report.Kept,
			"droppedFindings": outcome.Report.Dropped,
			"droppedPaths":    outcome.Report.DroppedPaths,
			"sarifRewritten":  outcome.Rewritten,
		}
		// Der abweichende Zustand wird ausdrücklich gemeldet: das Werkzeug hat
		// etwas anderes geschrieben als verlangt, und wer done gemeldet hat,
		// soll das nicht erst im nächsten Status erfahren.
		if outcome.State != input.State {
			data["stateOverridden"] = true
			data["requestedState"] = input.State
		}
	}
	return data, reviewToolError{}
}

// evidenceOutcome ist das Ergebnis der SARIF-Prüfung beim Melden.
type evidenceOutcome struct {
	// Present meldet, ob überhaupt ein Job zu prüfen war.
	Present bool
	// State ist der Zustand, der in die Entry-Datei geht. Er weicht von der
	// Meldung ab, wenn das SARIF ungültig ist.
	State review.State
	Job   review.JobStatus
	// Note ist der Zusatz für den Grund: die Zahl der verworfenen Funde oder
	// der Grund der Ungültigkeit.
	Note      string
	Report    review.EvidenceReport
	Rewritten bool
}

// evaluateEvidenceJob prüft das gemeldete SARIF eines Evidence-Eintrags und
// bereinigt es um Funde außerhalb des Pfad-Scopes.
//
// Die Fehler teilen sich in zwei Gruppen, und die Grenze verläuft dort, wo auch
// die Zuständigkeit wechselt:
//
//   - Fehler des Aufrufs — ein Pfad außerhalb von raw/, eine fehlende oder leere
//     Datei, ein Rezept ohne gültigen Evidence-Vertrag — werden abgewiesen, und
//     es wird nichts geschrieben. Der Melder kann sie im nächsten Aufruf
//     beheben, ohne das Rezept erneut laufen zu lassen.
//   - Fehler des Artefakts — unlesbares SARIF, fremder Werkzeugname, eine
//     Rule-ID außerhalb der Liste des Rezepts — machen den Eintrag failed und
//     nennen den Grund. Sie bedeuten, dass das Rezept etwas Falsches erzeugt
//     hat; repariert wird das über einen erneuten Lauf und nicht über einen
//     zweiten Statusaufruf. Ein stilles done gibt es dafür nicht.
//
// Der Scope wirkt dazwischen als Teilannahme: verworfene Funde verschwinden aus
// dem SARIF, ihre Zahl und die ersten Pfade stehen im Grund, der Eintrag bleibt
// gültig.
func evaluateEvidenceJob(runDir string, entry review.Entry, input reviewWriteAIEntryInput) (evidenceOutcome, reviewToolError) {
	if input.Job == nil {
		return evidenceOutcome{}, reviewToolError{}
	}
	relative := filepath.ToSlash(filepath.Clean(filepath.FromSlash(strings.TrimSpace(input.Job.SARIF))))
	outcome := evidenceOutcome{
		Present: true,
		State:   input.State,
		Job: review.JobStatus{
			Job:      entry.Name,
			State:    review.StateDone,
			SARIF:    relative,
			Started:  input.Job.Started,
			Finished: input.Job.Finished,
		},
	}

	sarifPath := filepath.Join(runDir, filepath.FromSlash(relative))
	info, err := os.Stat(sarifPath)
	if err != nil {
		return evidenceOutcome{}, wrapError(err, "sarif_path_invalid", "SARIF-Artefakt existiert nicht.", map[string]any{"entry": input.Entry, "sarif": relative, "path": sarifPath})
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return evidenceOutcome{}, reviewToolError{
			Code:    "sarif_path_invalid",
			Message: "SARIF-Artefakt ist keine nutzbare Datei.",
			Details: map[string]any{"entry": input.Entry, "sarif": relative, "path": sarifPath},
		}
	}
	ruleIDs, toolErr := evidenceRuleIDs(entry)
	if toolErr.Code != "" {
		return evidenceOutcome{}, toolErr
	}
	data, err := os.ReadFile(sarifPath)
	if err != nil {
		return evidenceOutcome{}, wrapError(err, "read_failed", "SARIF-Artefakt ist nicht lesbar.", map[string]any{"entry": input.Entry, "sarif": relative, "path": sarifPath})
	}

	report, err := review.CheckEvidenceSARIF(data, entry.Name, ruleIDs, entryScopePaths(entry))
	if err != nil {
		outcome.State = review.StateFailed
		outcome.Note = err.Error()
		outcome.Job.State = review.StateFailed
		outcome.Job.Reason = err.Error()
		return outcome, reviewToolError{}
	}
	if report.Cleaned != nil {
		if err := writeFileAtomic(sarifPath, report.Cleaned); err != nil {
			return evidenceOutcome{}, wrapError(err, "write_failed", "Bereinigtes SARIF konnte nicht geschrieben werden.", map[string]any{"entry": input.Entry, "sarif": relative, "path": sarifPath})
		}
		outcome.Rewritten = true
	}
	findings := report.Kept
	outcome.Job.Findings = &findings
	outcome.Job.Reason = report.ScopeNote()
	outcome.Note = report.ScopeNote()
	outcome.Report = report
	return outcome, reviewToolError{}
}

// evidenceRuleIDs holt die Rule-ID-Liste aus dem Rezept des Eintrags.
//
// Nachgeladen statt im Lauf eingefroren: die Liste ist der Vertrag des Rezepts
// und keine Festlegung des Laufs. Wer sie ändert, ändert sie für alle Läufe —
// und ein Rezept, das seinen Evidence-Vertrag inzwischen nicht mehr erfüllt,
// soll beim Melden auffallen und nicht mit einer halben Prüfung durchgehen.
func evidenceRuleIDs(entry review.Entry) ([]string, reviewToolError) {
	if strings.TrimSpace(entry.RecipePath) == "" {
		return nil, reviewToolError{
			Code:    "recipe_contract_invalid",
			Message: "Evidence-Eintrag ohne Rezeptpfad — die Rule-ID-Liste ist nicht prüfbar.",
			Details: map[string]any{"entry": entry.Name},
		}
	}
	metadata, err := readAIRecipeMetadata(entry.RecipeKey, entry.RecipePath)
	if err != nil {
		return nil, wrapError(err, "read_failed", "Review-Rezept ist nicht lesbar.", map[string]any{"entry": entry.Name, "path": entry.RecipePath})
	}
	contract := metadata.auditContract()
	contract.Mode = review.ModeEvidence
	if err := review.ValidateAuditContract(contract); err != nil {
		return nil, reviewToolError{
			Code:    "recipe_contract_invalid",
			Message: "Rezept erfüllt den Evidence-Vertrag nicht mehr.",
			Details: map[string]any{"entry": entry.Name, "path": entry.RecipePath, "reason": err.Error()},
		}
	}
	return metadata.RuleIDs, reviewToolError{}
}

func entryScopePaths(entry review.Entry) []string {
	if entry.Scope == nil {
		return nil
	}
	return entry.Scope.Paths
}

// joinReason hängt den Zusatz an den gemeldeten Grund an, ohne ihn zu ersetzen.
func joinReason(reason string, note string) string {
	reason = strings.TrimSpace(reason)
	note = strings.TrimSpace(note)
	switch {
	case note == "":
		return reason
	case reason == "":
		return note
	default:
		return reason + " — " + note
	}
}

func writeInitialScanTriageEntry(runDir string, entries []review.Entry) error {
	for _, entry := range entries {
		if entry.Name != scanTriageEntry || entry.Kind != review.KindAI {
			continue
		}
		return writeAIEntryStatusFile(runDir, aiEntryStatus{
			Name:  scanTriageEntry,
			Kind:  review.KindAI,
			State: review.StateStart,
		})
	}
	return nil
}

func repairableScanTriageEntry(env reviewEnvironment, name string) (review.Entry, bool) {
	if name != scanTriageEntry {
		return review.Entry{}, false
	}
	candidate, ok := scanTriageCandidate(env)
	if !ok {
		return review.Entry{}, false
	}
	return entryFromCandidate(candidate), true
}

func findRunEntry(run review.Run, name string) (review.Entry, bool) {
	for _, entry := range run.Entries {
		if entry.Name == name {
			return entry, true
		}
	}
	return review.Entry{}, false
}

func validateAIEntryInput(runDir string, entry review.Entry, input reviewWriteAIEntryInput) reviewToolError {
	switch input.State {
	case review.StateRunning, review.StateDone, review.StateFailed, review.StateSkipped:
	default:
		return reviewToolError{
			Code:    "entry_state_invalid",
			Message: "Eintragszustand ist ungültig.",
			Details: map[string]any{"entry": input.Entry, "state": input.State, "allowed": []string{"running", "done", "failed", "skipped"}},
		}
	}
	if toolErr := validateAIEntryJob(runDir, entry, input); toolErr.Code != "" {
		return toolErr
	}
	if input.Result != "" {
		if toolErr := validateResultPath(runDir, input.Result); toolErr.Code != "" {
			return toolErr
		}
	}
	if input.State == review.StateDone && entryResultRequired(entry) && input.Result == "" {
		return reviewToolError{
			Code:    "result_required",
			Message: "Ein Ergebnis ist für diesen AI-Eintrag Pflicht.",
			Details: map[string]any{"entry": input.Entry},
		}
	}
	if input.State == review.StateDone && input.Result != "" {
		path := filepath.Join(runDir, filepath.FromSlash(input.Result))
		info, err := os.Stat(path)
		if err != nil {
			return wrapError(err, "result_path_invalid", "Ergebnisartefakt existiert nicht.", map[string]any{"entry": input.Entry, "result": input.Result, "path": path})
		}
		if !info.Mode().IsRegular() {
			return reviewToolError{
				Code:    "result_path_invalid",
				Message: "Ergebnisartefakt ist keine reguläre Datei.",
				Details: map[string]any{"entry": input.Entry, "result": input.Result, "path": path},
			}
		}
		if info.Size() == 0 {
			return reviewToolError{
				Code:    "result_path_invalid",
				Message: "Ergebnisartefakt ist leer.",
				Details: map[string]any{"entry": input.Entry, "result": input.Result, "path": path},
			}
		}
	}
	if input.State == review.StateRunning && input.FinishedAt != "" {
		return reviewToolError{
			Code:    "entry_state_invalid",
			Message: "Ein laufender Eintrag darf keine finishedAt-Zeit tragen.",
			Details: map[string]any{"entry": input.Entry, "finishedAt": input.FinishedAt},
		}
	}
	if (input.State == review.StateFailed || input.State == review.StateSkipped) && strings.TrimSpace(input.Reason) == "" {
		return reviewToolError{
			Code:    "entry_state_invalid",
			Message: "Failed und skipped brauchen einen Grund.",
			Details: map[string]any{"entry": input.Entry, "state": input.State},
		}
	}
	for field, value := range map[string]string{"startedAt": input.StartedAt, "finishedAt": input.FinishedAt} {
		if value == "" {
			continue
		}
		if _, err := time.Parse(time.RFC3339, value); err != nil {
			return reviewToolError{
				Code:    "entry_state_invalid",
				Message: "Zeitstempel ist ungültig.",
				Details: map[string]any{"entry": input.Entry, "field": field, "value": value, "layout": time.RFC3339},
			}
		}
	}
	return reviewToolError{}
}

// validateAIEntryJob prüft die Form der Ergebnismeldung an den beiden
// Betriebsarten.
//
// mode: perspective bleibt unverändert: keine Jobs, das Markdown-Ergebnis ist
// das Artefakt. mode: evidence kehrt das um — das SARIF ist Pflicht, eine
// zweite Ergebnisdatei gibt es nicht. Der Job gehört dabei zur Fertigmeldung:
// vorher ist das Artefakt noch nicht da, bei failed und skipped erklärt der
// Grund den Ausgang.
func validateAIEntryJob(runDir string, entry review.Entry, input reviewWriteAIEntryInput) reviewToolError {
	evidence := review.EntryMode(entry) == review.ModeEvidence
	if input.Job != nil {
		if !evidence {
			return reviewToolError{
				Code:    "entry_job_invalid",
				Message: "Ein Job gehört zu mode: evidence.",
				Details: map[string]any{"entry": input.Entry, "mode": review.EntryMode(entry)},
			}
		}
		if input.State != review.StateDone {
			return reviewToolError{
				Code:    "entry_job_invalid",
				Message: "Ein Job gehört zur Fertigmeldung eines Evidence-Eintrags.",
				Details: map[string]any{"entry": input.Entry, "state": input.State, "expectedState": review.StateDone},
			}
		}
		if toolErr := validateSARIFPath(runDir, input.Entry, input.Job.SARIF); toolErr.Code != "" {
			return toolErr
		}
		for field, value := range map[string]string{"job.started": input.Job.Started, "job.finished": input.Job.Finished} {
			if value == "" {
				continue
			}
			if _, err := time.Parse(time.RFC3339, value); err != nil {
				return reviewToolError{
					Code:    "entry_state_invalid",
					Message: "Zeitstempel ist ungültig.",
					Details: map[string]any{"entry": input.Entry, "field": field, "value": value, "layout": time.RFC3339},
				}
			}
		}
	}
	if !evidence {
		return reviewToolError{}
	}
	if input.Result != "" {
		return reviewToolError{
			Code:    "entry_result_invalid",
			Message: "Ein Evidence-Eintrag hat keine Ergebnisdatei — Pflichtartefakt ist raw/<entry>.sarif.",
			Details: map[string]any{"entry": input.Entry, "result": input.Result, "sarif": review.EvidenceSARIFPath(input.Entry)},
		}
	}
	if input.State == review.StateDone && input.Job == nil {
		return reviewToolError{
			Code:    "sarif_required",
			Message: "Ein fertiger Evidence-Eintrag braucht einen Job mit SARIF-Pfad.",
			Details: map[string]any{"entry": input.Entry, "sarif": review.EvidenceSARIFPath(input.Entry)},
		}
	}
	return reviewToolError{}
}

// validateSARIFPath prüft den gemeldeten SARIF-Pfad auf seine Form: relativ zum
// Laufverzeichnis, innerhalb davon und unterhalb von raw/. Das Verzeichnis ist
// der Ort der Rohdaten, und der Merge sammelt von dort ein — ein Artefakt
// daneben würde nie gelesen.
func validateSARIFPath(runDir string, entry string, value string) reviewToolError {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return reviewToolError{
			Code:    "sarif_path_invalid",
			Message: "Der Job braucht einen SARIF-Pfad.",
			Details: map[string]any{"entry": entry, "sarif": review.EvidenceSARIFPath(entry)},
		}
	}
	if toolErr := validateResultPath(runDir, trimmed); toolErr.Code != "" {
		return reviewToolError{
			Code:    "sarif_path_invalid",
			Message: "SARIF-Pfad muss relativ zum Laufverzeichnis sein und darf es nicht verlassen.",
			Details: map[string]any{"entry": entry, "sarif": value},
		}
	}
	cleaned := filepath.ToSlash(filepath.Clean(filepath.FromSlash(trimmed)))
	if !strings.HasPrefix(cleaned, review.RawDirName+"/") {
		return reviewToolError{
			Code:    "sarif_path_invalid",
			Message: "SARIF-Pfad muss unterhalb von " + review.RawDirName + "/ liegen.",
			Details: map[string]any{"entry": entry, "sarif": value, "expected": review.EvidenceSARIFPath(entry)},
		}
	}
	return reviewToolError{}
}

func validateResultPath(runDir string, value string) reviewToolError {
	if filepath.IsAbs(value) || filepath.Clean(value) == "." {
		return reviewToolError{
			Code:    "result_path_invalid",
			Message: "Ergebnispfad muss relativ zum Laufverzeichnis sein.",
			Details: map[string]any{"result": value},
		}
	}
	cleaned := filepath.Clean(filepath.FromSlash(value))
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return reviewToolError{
			Code:    "result_path_invalid",
			Message: "Ergebnispfad darf das Laufverzeichnis nicht verlassen.",
			Details: map[string]any{"result": value},
		}
	}
	joined := filepath.Join(runDir, cleaned)
	rel, err := filepath.Rel(runDir, joined)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return reviewToolError{
			Code:    "result_path_invalid",
			Message: "Ergebnispfad darf das Laufverzeichnis nicht verlassen.",
			Details: map[string]any{"result": value},
		}
	}
	return reviewToolError{}
}

func writeAIEntryStatusFile(runDir string, status aiEntryStatus) error {
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(review.EntryFile(runDir, status.Name)), 0o755); err != nil {
		return err
	}
	return writeFileAtomic(review.EntryFile(runDir, status.Name), append(data, '\n'))
}

// writeFileAtomic ersetzt eine Datei über eine Temp-Datei im selben
// Verzeichnis. Ein Leser, der während des Schreibens nachschaut, sieht die alte
// Fassung oder die neue, nie eine halbe — das gilt für die Entry-Datei wie für
// das bereinigte SARIF, das an die Stelle des gemeldeten tritt.
func writeFileAtomic(target string, data []byte) error {
	dir := filepath.Dir(target)
	temp, err := os.CreateTemp(dir, "."+filepath.Base(target)+".*")
	if err != nil {
		return err
	}
	name := temp.Name()
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		os.Remove(name)
		return err
	}
	if err := temp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Chmod(name, 0o644); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, target); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}

func validateScanSelection(runDir string, run review.Run, wanted []string) ([]review.Entry, reviewToolError) {
	byName := map[string]review.Entry{}
	for _, entry := range run.Entries {
		byName[entry.Name] = entry
	}
	if len(wanted) == 0 {
		entries := []review.Entry{}
		for _, entry := range run.Entries {
			if entry.Kind == review.KindTool && review.EntryState(runDir, entry.Name) == review.StateStart {
				entries = append(entries, entry)
			}
		}
		return entries, reviewToolError{}
	}
	entries := []review.Entry{}
	seen := map[string]bool{}
	for _, name := range wanted {
		if err := ensureEntryName(name); err.Code != "" {
			return nil, err
		}
		entry, known := byName[name]
		if !known {
			return nil, reviewToolError{
				Code:    "selection_unknown",
				Message: "Eintrag ist nicht im Lauf enthalten.",
				Details: map[string]any{"entry": name, "known": runEntryNames(run)},
			}
		}
		if entry.Kind != review.KindTool {
			return nil, reviewToolError{
				Code:    "entry_kind_invalid",
				Message: "Nur Tool-Einträge können gescannt werden.",
				Details: map[string]any{"entry": name, "kind": entry.Kind, "expectedKind": review.KindTool},
			}
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		entries = append(entries, entry)
	}
	return entries, reviewToolError{}
}

func resolveReviewTools(preflight project.ToolPreflight) map[string]review.Tool {
	tools := map[string]review.Tool{}
	for _, tool := range preflight.Tools {
		resolved := review.Tool{}
		if tool.Status == "ok" && tool.Path != "" {
			resolved.Path = tool.Path
		} else {
			resolved.Reason = "Werkzeug " + tool.Name + " ist nicht installiert"
		}
		tools[tool.Name] = resolved
	}
	return tools
}

func countStatusEntries(entries []merge.EntrySummary) map[string]int {
	counts := map[string]int{}
	for _, entry := range entries {
		counts[string(entry.State)]++
	}
	return counts
}

func mcpKPlaybookVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info == nil {
		return "unknown"
	}
	settings := map[string]string{}
	for _, setting := range info.Settings {
		settings[setting.Key] = setting.Value
	}
	version := strings.TrimSpace(info.Main.Version)
	revision := strings.TrimSpace(settings["vcs.revision"])
	dirty := settings["vcs.modified"] == "true"
	if version != "" && version != "(devel)" {
		if dirty {
			return version + "-dirty"
		}
		return version
	}
	if revision == "" {
		return "unknown"
	}
	short := revision
	if len(short) > 7 {
		short = short[:7]
	}
	if version == "(devel)" {
		short = "(devel)+" + short
	}
	if dirty {
		short += "-dirty"
	}
	return short
}

func runEntryNames(run review.Run) []string {
	names := make([]string, 0, len(run.Entries))
	for _, entry := range run.Entries {
		names = append(names, entry.Name)
	}
	sort.Strings(names)
	return names
}

func reviewExistingStatus(env reviewEnvironment, runName string) (map[string]any, reviewToolError) {
	runDir, run, toolErr := requireRun(env, runName)
	if toolErr.Code != "" {
		return nil, toolErr
	}
	effectiveRun := effectiveRunForStatus(env, run)
	entries, toolErr := statusEntries(runDir, effectiveRun)
	if toolErr.Code != "" {
		return nil, toolErr
	}
	raw, toolErr := listRunFiles(filepath.Join(runDir, review.RawDirName), ".sarif")
	if toolErr.Code != "" {
		return nil, toolErr
	}
	artifacts, modified, toolErr := listExistingArtifacts(runDir, []string{reviewInputJSON, reviewInputMarkdown, scanTriageResult})
	if toolErr.Code != "" {
		return nil, toolErr
	}
	evidence, perspectives := entriesByMode(effectiveRun)
	return map[string]any{
		"mode":    "existing",
		"run":     runName,
		"runDir":  runDir,
		"runJSON": run,
		"state":   review.DeriveRunState(runDir, effectiveRun),
		"entries": entries,
		// Die beiden Listen halten die Reihenfolge des Laufs fest:
		// Evidence-Einträge liefern Rohdaten und laufen vor dem Merge,
		// Perspektiven bewerten review-input.json und laufen danach.
		"evidenceEntries":    evidence,
		"perspectiveEntries": perspectives,
		"rawSarif":           raw,
		"reviewInput":        artifacts,
		"triage":             triageFreshness(artifacts, modified, entryFinishedAt(entries, scanTriageEntry)),
	}, reviewToolError{}
}

// Zustände der Bewertung eines Laufs. Sie stehen neben dem Eintragszustand von
// scan-triage und ersetzen ihn nicht: der Eintrag sagt, ob die Bewertung
// geschrieben wurde, diese drei sagen, ob sie noch gilt.
const (
	triageStateMissing = "missing"
	triageStateCurrent = "current"
	triageStateStale   = "stale"
)

// triageFreshness vergleicht die Änderungszeit von review-input.json mit der
// Endzeit des Eintrags scan-triage.
//
// Der Grund für den Vergleich: markAIRepairStatus misst an review-triage.md nur
// Existenz und Größe. Ein erneuter Merge — der reguläre Weg, wenn Evidence
// nachträglich eintrifft — lässt den Eintrag deshalb done und konsistent, obwohl
// die Bewertung einen Stand beschreibt, den es nicht mehr gibt.
//
// Nicht belegbar heißt hier stale und nicht current: fehlt review-input.json
// oder nennt der Eintrag keine brauchbare Endzeit, lässt sich die Aktualität
// nicht zeigen, und eine unbelegte Bewertung darf einen Lauf nicht vollständig
// aussehen lassen. Der Grund sagt jeweils, welcher Fall vorlag.
//
// Gleiche Zeiten gelten als aktuell: die Bewertung entsteht nach dem Merge, und
// ein Merge danach hinterlässt eine echt spätere Änderungszeit.
func triageFreshness(artifacts map[string]string, modified map[string]time.Time, finishedAt string) map[string]any {
	status := map[string]any{"result": scanTriageResult, "state": triageStateMissing}
	if _, found := artifacts[scanTriageResult]; !found {
		return status
	}
	status["finishedAt"] = finishedAt
	mergedAt, found := modified[reviewInputJSON]
	if !found {
		status["state"] = triageStateStale
		status["reason"] = reviewInputJSON + " fehlt — die Aktualität der Bewertung ist nicht belegbar."
		return status
	}
	status["reviewInputModified"] = mergedAt.Format(timeLayout)
	finished, err := time.Parse(timeLayout, strings.TrimSpace(finishedAt))
	if err != nil {
		status["state"] = triageStateStale
		if strings.TrimSpace(finishedAt) == "" {
			status["reason"] = "Der Eintrag " + scanTriageEntry + " nennt keine Endzeit — die Aktualität der Bewertung ist nicht belegbar."
			return status
		}
		status["reason"] = "Die Endzeit des Eintrags " + scanTriageEntry + " ist kein " + timeLayout + "-Zeitstempel — die Aktualität der Bewertung ist nicht belegbar."
		return status
	}
	if mergedAt.After(finished) {
		status["state"] = triageStateStale
		status["reason"] = reviewInputJSON + " ist jünger als die Bewertung — der Merge lief danach."
		return status
	}
	status["state"] = triageStateCurrent
	return status
}

// entryFinishedAt liest die Endzeit eines AI-Eintrags aus der Statusliste, die
// gerade gebaut wurde — nicht aus einem zweiten Dateizugriff. Der Vergleich soll
// genau die Zeit benutzen, die der Status auch meldet.
func entryFinishedAt(entries []map[string]any, name string) string {
	for _, item := range entries {
		if item["name"] != name {
			continue
		}
		finished, _ := item["finishedAt"].(string)
		return finished
	}
	return ""
}

// entriesByMode trennt die AI-Einträge eines Laufs nach Betriebsart.
// Tool-Einträge stehen in keiner der beiden Listen: sie haben keine.
func entriesByMode(run review.Run) ([]string, []string) {
	evidence := []string{}
	perspectives := []string{}
	for _, entry := range run.Entries {
		if entry.Kind != review.KindAI {
			continue
		}
		if review.EntryMode(entry) == review.ModeEvidence {
			evidence = append(evidence, entry.Name)
			continue
		}
		perspectives = append(perspectives, entry.Name)
	}
	return evidence, perspectives
}

func effectiveRunForStatus(env reviewEnvironment, run review.Run) review.Run {
	if _, found := findRunEntry(run, scanTriageEntry); found {
		return run
	}
	candidate, ok := scanTriageCandidate(env)
	if !ok {
		return run
	}
	effective := run
	effective.Entries = append(append([]review.Entry{}, run.Entries...), entryFromCandidate(candidate))
	return effective
}

func buildSelectionBase(env reviewEnvironment) (reviewSelectionBase, reviewToolError) {
	preflight, err := project.CheckTools(env.Root, env.Languages)
	if err != nil {
		return reviewSelectionBase{}, wrapError(err, "preflight_failed", "Tool-Preflight fehlgeschlagen.", map[string]any{"projectDir": env.Root})
	}
	candidates := []reviewCandidate{}
	selectedLanguages := stringsSet(env.Languages)
	for _, tool := range preflight.Tools {
		applies := toolAppliesToLanguages(tool.Languages, selectedLanguages)
		candidate := reviewCandidate{
			Name:      tool.Name,
			Kind:      review.KindTool,
			Title:     tool.Name,
			Detail:    tool.Role,
			Languages: tool.Languages,
			Status:    tool.Status,
			Path:      tool.Path,
		}
		switch {
		case !applies:
			candidate.UnavailableReason = "Sprache nicht gewählt"
		case tool.Status != "ok":
			candidate.UnavailableReason = "nicht installiert"
		default:
			candidate.Selectable = true
			candidate.DefaultSelected = true
		}
		candidates = append(candidates, candidate)
	}

	built, err := project.BuildContext(env.Root)
	if err != nil {
		return reviewSelectionBase{}, wrapError(err, "read_failed", "Review-Katalog ist nicht lesbar.", map[string]any{"projectDir": env.Root})
	}
	for _, entry := range built.Catalogs["reviews"] {
		if entry.Disabled {
			continue
		}
		metadata, err := readAIRecipeMetadata(entry.Key, entry.Path)
		if err != nil {
			return reviewSelectionBase{}, wrapError(err, "read_failed", "Review-Rezept ist nicht lesbar.", map[string]any{"path": entry.Path, "key": entry.Key})
		}
		if !metadata.Enabled {
			continue
		}
		resultRequired := metadata.ResultRequired
		auditEnabled := metadata.Enabled
		reviewEnabled := metadata.ReviewEnabled
		candidate := reviewCandidate{
			Name:            entry.Key,
			Kind:            review.KindAI,
			Title:           metadata.Title,
			Selectable:      true,
			DefaultSelected: true,
			Detail:          originLabel(entry.Origin),
			Mode:            metadata.Mode,
			RecipeKey:       entry.Key,
			RecipePath:      entry.Path,
			RecipeOrigin:    entry.Origin,
			AuditEnabled:    &auditEnabled,
			ReviewEnabled:   &reviewEnabled,
			ResultRequired:  &resultRequired,
			DefaultResult:   metadata.DefaultResult,
			RuleIDs:         append([]string{}, metadata.RuleIDs...),
			Scope:           cloneReviewScope(metadata.Scope),
		}
		if len(candidate.RuleIDs) == 0 {
			candidate.RuleIDs = nil
		}
		// Ein Rezept mit widersprüchlichem audit-Block wird nicht
		// stillschweigend zurechtgebogen, sondern fällt aus der Auswahl: es
		// bliebe sonst offen, welche Hälfte des Vertrags gilt. Der Grund steht
		// am Kandidaten und ist damit im Status sichtbar.
		if err := review.ValidateAuditContract(metadata.auditContract()); err != nil {
			candidate.Selectable = false
			candidate.DefaultSelected = false
			candidate.UnavailableReason = "Audit-Vertrag ungültig: " + err.Error()
		}
		candidates = append(candidates, candidate)
	}
	if candidate, ok := scanTriageCandidate(env); ok {
		candidates = append(candidates, candidate)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Kind != candidates[j].Kind {
			return candidates[i].Kind == review.KindTool
		}
		return candidates[i].Name < candidates[j].Name
	})
	unavailable := []reviewCandidate{}
	defaults := []review.Entry{}
	evidence := []string{}
	perspectives := []string{}
	for _, candidate := range candidates {
		if !candidate.Selectable {
			unavailable = append(unavailable, candidate)
			continue
		}
		if candidate.Kind == review.KindAI {
			if review.NormalizeMode(candidate.Mode) == review.ModeEvidence {
				evidence = append(evidence, candidate.Name)
			} else {
				perspectives = append(perspectives, candidate.Name)
			}
		}
		if candidate.DefaultSelected {
			defaults = append(defaults, entryFromCandidate(candidate))
		}
	}
	return reviewSelectionBase{
		Languages:             append([]string{}, env.Languages...),
		Candidates:            candidates,
		UnavailableCandidates: unavailable,
		DefaultEntries:        defaults,
		EvidenceCandidates:    evidence,
		PerspectiveCandidates: perspectives,
		Preflight:             preflight,
	}, reviewToolError{}
}

func scanTriageCandidate(env reviewEnvironment) (reviewCandidate, bool) {
	for _, entry := range project.ActiveRegistry(env.Root, project.KindCommands) {
		if entry.Name != scanTriageModule {
			continue
		}
		resultRequired := true
		return reviewCandidate{
			Name:            scanTriageEntry,
			Kind:            review.KindAI,
			Title:           "Review-Triage",
			Selectable:      true,
			DefaultSelected: true,
			Detail:          "Command-Modul " + scanTriageModule,
			Path:            entry.Path,
			RecipeKey:       scanTriageEntry,
			RecipePath:      entry.Path,
			RecipeOrigin:    entry.Origin,
			ResultRequired:  &resultRequired,
			DefaultResult:   scanTriageResult,
		}, true
	}
	return reviewCandidate{}, false
}

func validateCreateSelection(base reviewSelectionBase, requested []reviewSelectionInput) ([]review.Entry, reviewToolError) {
	if len(requested) == 0 {
		return append([]review.Entry{}, base.DefaultEntries...), reviewToolError{}
	}
	byName := map[string]reviewCandidate{}
	for _, candidate := range base.Candidates {
		byName[candidate.Name] = candidate
	}
	selected := make([]review.Entry, 0, len(requested))
	seen := map[string]bool{}
	for _, item := range requested {
		if err := ensureEntryName(item.Name); err.Code != "" {
			return nil, err
		}
		if !review.ValidKind(item.Kind) {
			return nil, reviewToolError{
				Code:    "invalid_selection",
				Message: "Eintragsart ist ungültig.",
				Details: map[string]any{"entry": item.Name, "kind": item.Kind, "allowed": []string{string(review.KindTool), string(review.KindAI)}},
			}
		}
		if seen[item.Name] {
			return nil, reviewToolError{
				Code:    "invalid_selection",
				Message: "Eintrag ist doppelt ausgewählt.",
				Details: map[string]any{"entry": item.Name},
			}
		}
		seen[item.Name] = true
		candidate, known := byName[item.Name]
		if !known {
			return nil, reviewToolError{
				Code:    "selection_unknown",
				Message: "Auswahl ist unbekannt.",
				Details: map[string]any{"entry": item.Name, "known": candidateNames(base.Candidates)},
			}
		}
		if candidate.Kind != item.Kind {
			return nil, reviewToolError{
				Code:    "invalid_selection",
				Message: "Eintragsart passt nicht zum Kandidaten.",
				Details: map[string]any{"entry": item.Name, "kind": item.Kind, "expectedKind": candidate.Kind},
			}
		}
		if !candidate.Selectable {
			return nil, reviewToolError{
				Code:    "selection_unavailable",
				Message: "Auswahl ist nicht verfügbar.",
				Details: map[string]any{"entry": item.Name, "kind": item.Kind, "reason": candidate.UnavailableReason},
			}
		}
		selected = append(selected, entryFromCandidate(candidate))
	}
	if len(selected) == 0 {
		return nil, reviewToolError{
			Code:    "invalid_selection",
			Message: "Keine Einträge ausgewählt.",
			Details: map[string]any{},
		}
	}
	return selected, reviewToolError{}
}

func candidateNames(candidates []reviewCandidate) []string {
	names := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		names = append(names, candidate.Name)
	}
	sort.Strings(names)
	return names
}

func candidatesForEntries(candidates []reviewCandidate, entries []review.Entry) []reviewCandidate {
	selected := map[string]bool{}
	for _, entry := range entries {
		selected[entry.Name] = true
	}
	items := []reviewCandidate{}
	for _, candidate := range candidates {
		if selected[candidate.Name] {
			items = append(items, candidate)
		}
	}
	return items
}

func entryFromCandidate(candidate reviewCandidate) review.Entry {
	entry := review.Entry{Name: candidate.Name, Kind: candidate.Kind, State: review.StateStart}
	if candidate.Kind == review.KindAI {
		entry.RecipeKey = candidate.RecipeKey
		entry.RecipePath = candidate.RecipePath
		entry.RecipeOrigin = candidate.RecipeOrigin
		entry.Title = candidate.Title
		// Die Betriebsart wird ausgeschrieben und nicht bei perspective
		// weggelassen: der Lauf soll aus sich heraus sagen, wie ein Eintrag
		// gemeint war. Das leere Feld bleibt trotzdem lesbar — es ist die Form
		// der Läufe von vor der Evidence-Betriebsart.
		entry.Mode = review.NormalizeMode(candidate.Mode)
		entry.ResultRequired = candidate.ResultRequired
		entry.DefaultResult = candidate.DefaultResult
		entry.Scope = cloneReviewScope(candidate.Scope)
	}
	return entry
}

func cloneReviewScope(scope *review.Scope) *review.Scope {
	if scope == nil {
		return nil
	}
	cloned := &review.Scope{}
	if scope.Tools != nil {
		cloned.Tools = append([]string{}, scope.Tools...)
	}
	if scope.Paths != nil {
		cloned.Paths = append([]string{}, scope.Paths...)
	}
	return cloned
}

func readAIRecipeMetadata(key string, path string) (aiRecipeMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return aiRecipeMetadata{}, err
	}
	metadata := aiRecipeMetadata{Enabled: false, ReviewEnabled: true, ResultRequired: true}
	content := string(data)
	body := content
	if strings.HasPrefix(content, "---\n") || strings.HasPrefix(content, "---\r\n") {
		end := frontmatterEnd(content)
		if end >= 0 {
			parseAIRecipeFrontmatter(content[:end], &metadata)
			body = content[end:]
		}
	}
	metadata.Mode = review.NormalizeMode(metadata.Mode)
	// Für mode: evidence ist raw/<entry>.sarif das Pflichtartefakt; eine
	// Ergebnisdatei gibt es nicht. ResultRequired wird deshalb hart auf false
	// gesetzt und nicht bloß nicht gelesen: seine Vorgabe ist true, und mit ihr
	// meldete review_status jeden erfolgreichen Evidence-Eintrag als
	// resultMissing und inconsistent.
	//
	// ResultRequiredSet und DefaultResult bleiben dagegen so stehen, wie sie im
	// Rezept stehen. Sie sind die Eingabe für ValidateAuditContract, das genau
	// diese beiden Felder neben mode: evidence als Fehler meldet — überschrieben
	// wären sie dort nicht mehr zu sehen.
	if metadata.Mode == review.ModeEvidence {
		metadata.ResultRequired = false
	}
	if metadata.Title == "" {
		metadata.Title = firstHeading(body)
	}
	if metadata.Title == "" {
		metadata.Title = key
	}
	return metadata, nil
}

func frontmatterEnd(content string) int {
	startEnd := strings.Index(content, "\n")
	if startEnd < 0 {
		return -1
	}
	offset := startEnd + 1
	for offset <= len(content) {
		next := strings.Index(content[offset:], "\n")
		lineEnd := len(content)
		if next >= 0 {
			lineEnd = offset + next
		}
		line := strings.TrimSpace(strings.TrimSuffix(content[offset:lineEnd], "\r"))
		if line == "---" {
			if next >= 0 {
				return lineEnd + 1
			}
			return lineEnd
		}
		if next < 0 {
			break
		}
		offset = lineEnd + 1
	}
	return -1
}

func parseAIRecipeFrontmatter(frontmatter string, metadata *aiRecipeMetadata) {
	inAudit := false
	inReview := false
	inScope := false
	// listKey ist der Name der zuletzt geöffneten Blockliste — tools, paths
	// oder ruleIds. Ein einzelnes Flag reichte, solange nur scope.tools eine
	// Liste war; mit scope.paths und audit.ruleIds muss beim Einsammeln
	// bekannt sein, wohin die Einträge gehören.
	listKey := ""
	blockIndent := -1
	scopeIndent := -1
	listIndent := -1
	for _, line := range strings.Split(frontmatter, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || trimmed == "---" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		if listKey != "" && indent <= listIndent {
			listKey = ""
		}
		if inScope && indent <= scopeIndent {
			inScope = false
			listKey = ""
		}
		if (inAudit || inReview) && indent <= blockIndent {
			inAudit = false
			inReview = false
			inScope = false
			listKey = ""
		}
		if listKey != "" && strings.HasPrefix(trimmed, "- ") {
			item := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
			switch listKey {
			case "tools":
				metadata.Scope = appendScopeTools(metadata.Scope, item)
			case "paths":
				metadata.Scope = appendScopePaths(metadata.Scope, item)
			case "ruleIds":
				metadata.RuleIDs = appendUniqueValues(metadata.RuleIDs, parseFrontmatterStringList(item))
			}
			continue
		}
		if strings.HasSuffix(trimmed, ":") {
			key := strings.TrimSuffix(trimmed, ":")
			if key == "audit" || key == "review" {
				inAudit = key == "audit"
				inReview = key == "review"
				blockIndent = indent
				continue
			}
			if inAudit && indent > blockIndent && key == "scope" {
				inScope = true
				scopeIndent = indent
				if metadata.Scope == nil {
					metadata.Scope = &review.Scope{}
				}
				continue
			}
		}
		key, value, found := strings.Cut(trimmed, ":")
		if !found {
			continue
		}
		value = strings.TrimSpace(strings.Trim(strings.TrimSpace(value), `"'`))
		field := strings.TrimSpace(key)
		if !inAudit && !inReview && indent == 0 && field == "title" && metadata.Title == "" {
			metadata.Title = value
			continue
		}
		if (!inAudit && !inReview) || indent <= blockIndent {
			continue
		}
		if inReview && field == "enabled" {
			metadata.ReviewEnabled = value != "false"
			continue
		}
		if inAudit && inScope && indent > scopeIndent && (field == "tools" || field == "paths") {
			if value == "" {
				listKey = field
				listIndent = indent
				continue
			}
			if field == "tools" {
				metadata.Scope = appendScopeTools(metadata.Scope, value)
			} else {
				metadata.Scope = appendScopePaths(metadata.Scope, value)
			}
			continue
		}
		if !inAudit {
			continue
		}
		switch field {
		case "enabled":
			metadata.Enabled = value != "false"
		case "title":
			metadata.Title = value
		case "mode":
			metadata.Mode = review.Mode(value)
		case "resultRequired":
			metadata.ResultRequired = value != "false"
			metadata.ResultRequiredSet = true
		case "defaultResult":
			metadata.DefaultResult = value
		case "ruleIds":
			if value == "" {
				listKey = "ruleIds"
				listIndent = indent
				continue
			}
			metadata.RuleIDs = appendUniqueValues(metadata.RuleIDs, parseFrontmatterStringList(value))
		}
	}
}

func appendScopeTools(scope *review.Scope, value string) *review.Scope {
	tools := parseFrontmatterStringList(value)
	if len(tools) == 0 {
		return scope
	}
	if scope == nil {
		scope = &review.Scope{}
	}
	scope.Tools = appendUniqueValues(scope.Tools, tools)
	return scope
}

func appendScopePaths(scope *review.Scope, value string) *review.Scope {
	paths := parseFrontmatterStringList(value)
	if len(paths) == 0 {
		return scope
	}
	if scope == nil {
		scope = &review.Scope{}
	}
	scope.Paths = appendUniqueValues(scope.Paths, paths)
	return scope
}

// appendUniqueValues hängt an, was noch nicht dasteht. Ein Rezept, das eine
// Liste zweimal schreibt, soll den Eintrag nicht zweimal in den Lauf tragen.
func appendUniqueValues(existing []string, values []string) []string {
	seen := stringsSet(existing)
	for _, value := range values {
		if seen[value] {
			continue
		}
		existing = append(existing, value)
		seen[value] = true
	}
	return existing
}

func parseFrontmatterStringList(value string) []string {
	value = strings.TrimSpace(strings.Trim(value, `"'`))
	if value == "" {
		return nil
	}
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		value = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(value, "["), "]"))
		if value == "" {
			return []string{}
		}
		parts := strings.Split(value, ",")
		values := make([]string, 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(strings.Trim(strings.TrimSpace(part), `"'`))
			if part != "" {
				values = append(values, part)
			}
		}
		return values
	}
	if strings.HasPrefix(value, "- ") {
		value = strings.TrimSpace(strings.TrimPrefix(value, "- "))
	}
	if value == "" {
		return nil
	}
	return []string{value}
}

func firstHeading(content string) string {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
		}
	}
	return ""
}

func statusEntries(runDir string, run review.Run) ([]map[string]any, reviewToolError) {
	entries := []map[string]any{}
	for _, entry := range run.Entries {
		item := map[string]any{
			"name":          entry.Name,
			"kind":          entry.Kind,
			"selectedState": entry.State,
			"state":         review.StateStart,
			"present":       false,
		}
		if entry.Kind == review.KindAI {
			item["recipeKey"] = entry.RecipeKey
			item["recipePath"] = entry.RecipePath
			item["recipeOrigin"] = entry.RecipeOrigin
			item["title"] = entry.Title
			item["mode"] = review.EntryMode(entry)
			item["resultRequired"] = entryResultRequired(entry)
			if entry.Scope != nil {
				item["scope"] = cloneReviewScope(entry.Scope)
			}
			if entry.DefaultResult != "" {
				item["defaultResult"] = entry.DefaultResult
			}
			defaultResultState := aiArtifactStateFor(runDir, entry.DefaultResult)
			status := aiEntryStatus{}
			err := readAIEntryStatus(runDir, entry.Name, &status)
			if err != nil {
				if os.IsNotExist(err) {
					sarif, sarifJob := evidenceArtifact(runDir, entry, nil)
					markAIRepairStatus(item, entry, aiStatusView{State: review.StateStart, ResultFile: defaultResultState, SARIF: sarif, SARIFJob: sarifJob})
					entries = append(entries, item)
					continue
				}
				return nil, wrapError(err, "read_failed", "Entry-Datei ist nicht lesbar.", map[string]any{"entry": entry.Name, "path": review.EntryFile(runDir, entry.Name)})
			}
			item["present"] = true
			item["state"] = status.State
			item["result"] = status.Result
			item["reason"] = status.Reason
			item["startedAt"] = status.StartedAt
			item["finishedAt"] = status.FinishedAt
			if len(status.Jobs) > 0 {
				item["jobs"] = status.Jobs
			}
			resultState := defaultResultState
			if status.Result != "" {
				resultState = aiArtifactStateFor(runDir, status.Result)
			}
			sarif, sarifJob := evidenceArtifact(runDir, entry, status.Jobs)
			markAIRepairStatus(item, entry, aiStatusView{State: status.State, Result: status.Result, ResultFile: resultState, SARIF: sarif, SARIFJob: sarifJob})
			entries = append(entries, item)
			continue
		}
		status, err := review.ReadEntryStatus(runDir, entry.Name)
		if err != nil {
			if os.IsNotExist(err) {
				entries = append(entries, item)
				continue
			}
			return nil, wrapError(err, "read_failed", "Entry-Datei ist nicht lesbar.", map[string]any{"entry": entry.Name, "path": review.EntryFile(runDir, entry.Name)})
		}
		item["present"] = true
		item["state"] = status.State
		item["started"] = status.Started
		item["finished"] = status.Finished
		item["reason"] = status.Reason
		item["jobs"] = status.Jobs
		entries = append(entries, item)
	}
	return entries, reviewToolError{}
}

// aiArtifactState ist der Zustand eines Pflichtartefakts im Laufordner — der
// Ergebnisdatei einer Perspektive oder des SARIF einer Evidence-Quelle. Beide
// werden gleich befragt: liegt die Datei da, hat sie Inhalt, ist ihr Pfad
// überhaupt zulässig.
type aiArtifactState struct {
	Path     string
	Exists   bool
	NonEmpty bool
	Invalid  bool
}

func aiArtifactStateFor(runDir string, relative string) aiArtifactState {
	state := aiArtifactState{Path: relative}
	if strings.TrimSpace(relative) == "" {
		return state
	}
	if err := validateResultPath(runDir, relative); err.Code != "" {
		state.Invalid = true
		return state
	}
	info, err := os.Stat(filepath.Join(runDir, filepath.FromSlash(relative)))
	if err != nil || !info.Mode().IsRegular() {
		return state
	}
	state.Exists = true
	state.NonEmpty = info.Size() > 0
	return state
}

// aiStatusView ist, was über einen AI-Eintrag auf der Platte bekannt ist: sein
// gemeldeter Zustand und der Zustand seiner Artefakte.
type aiStatusView struct {
	State  review.State
	Result string
	// ResultFile ist die Ergebnisdatei einer Perspektive.
	ResultFile aiArtifactState
	// SARIF ist das Pflichtartefakt einer Evidence-Quelle.
	SARIF aiArtifactState
	// SARIFJob meldet, ob der Eintrag einen fertigen Job mit SARIF-Pfad führt.
	// Ohne ihn liest der Merge die Datei nicht ein, auch wenn sie dasteht.
	SARIFJob bool
}

// markAIRepairStatus setzt die Marker, an denen /k-audit einen Eintrag als
// inkonsistent oder reparierbar erkennt.
//
// Für mode: evidence entscheidet dabei das SARIF und nicht die Ergebnisdatei:
// konsistent ist done mit Job und vorhandenem raw/<entry>.sarif. Ohne diese
// Unterscheidung wäre jeder erfolgreiche Evidence-Eintrag resultMissing — er
// schreibt ja gerade keine Ergebnisdatei.
func markAIRepairStatus(item map[string]any, entry review.Entry, view aiStatusView) {
	if review.EntryMode(entry) == review.ModeEvidence {
		markEvidenceRepairStatus(item, view)
		return
	}
	if view.ResultFile.Path != "" && view.ResultFile.Exists {
		item["resultExists"] = true
	}
	if view.ResultFile.Invalid {
		item["resultInvalid"] = true
		item["inconsistent"] = true
	}
	if view.State == review.StateDone && entryResultRequired(entry) {
		if strings.TrimSpace(view.Result) == "" || view.ResultFile.Path == "" || !view.ResultFile.Exists || !view.ResultFile.NonEmpty {
			item["resultMissing"] = true
			item["inconsistent"] = true
		}
	}
	if view.State == review.StateStart || view.State == review.StateRunning {
		if view.ResultFile.Exists && view.ResultFile.NonEmpty {
			item["repairable"] = true
			item["repairReason"] = "Ergebnisdatei vorhanden, Entry-Status offen oder fehlt."
		}
	}
}

func markEvidenceRepairStatus(item map[string]any, view aiStatusView) {
	item["sarif"] = view.SARIF.Path
	if view.SARIF.Exists {
		item["sarifExists"] = true
	}
	if view.SARIF.Invalid {
		item["sarifInvalid"] = true
		item["inconsistent"] = true
	}
	if view.State == review.StateDone && (!view.SARIFJob || !view.SARIF.Exists || !view.SARIF.NonEmpty) {
		item["sarifMissing"] = true
		item["inconsistent"] = true
	}
	// Gültiges SARIF bei offenem oder fehlendem Entry-Status ist über
	// k_playbook_review_write_ai_entry reparierbar — der Lauf des Rezepts muss
	// dafür nicht wiederholt werden.
	if view.State == review.StateStart || view.State == review.StateRunning {
		if view.SARIF.Exists && view.SARIF.NonEmpty {
			item["repairable"] = true
			item["repairReason"] = "SARIF vorhanden, Entry-Status offen oder fehlt."
		}
	}
}

// evidenceArtifact ist der Ort des SARIF eines Evidence-Eintrags: der gemeldete
// Job-Pfad, sonst der Ort aus dem Vertrag. Der Rückfall trägt den Reparaturfall
// — ohne Entry-Datei gibt es keinen Job, und trotzdem soll ein dort liegendes
// SARIF sichtbar sein.
func evidenceArtifact(runDir string, entry review.Entry, jobs []review.JobStatus) (aiArtifactState, bool) {
	for _, job := range jobs {
		if strings.TrimSpace(job.SARIF) == "" {
			continue
		}
		return aiArtifactStateFor(runDir, job.SARIF), job.State == review.StateDone
	}
	return aiArtifactStateFor(runDir, review.EvidenceSARIFPath(entry.Name)), false
}

type aiEntryStatus struct {
	Name       string       `json:"name"`
	Kind       review.Kind  `json:"kind"`
	State      review.State `json:"state"`
	Result     string       `json:"result,omitempty"`
	Reason     string       `json:"reason,omitempty"`
	StartedAt  string       `json:"startedAt,omitempty"`
	FinishedAt string       `json:"finishedAt,omitempty"`
	// Jobs ist dieselbe Darstellung wie in review.EntryStatus und keine zweite
	// daneben: der Merge liest die Entry-Dateien über review.ReadEntryStatus,
	// und ein eigenes Job-Format wäre dort unsichtbar. Weggelassen wird das
	// Feld, wo es keinen Job gibt — die Dateien der Perspektiven behalten damit
	// genau die Form, die sie bisher hatten, und Dateien aus der Zeit davor
	// bleiben lesbar.
	Jobs []review.JobStatus `json:"jobs,omitempty"`
}

func readAIEntryStatus(runDir string, name string, status *aiEntryStatus) error {
	data, err := os.ReadFile(review.EntryFile(runDir, name))
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, status); err != nil {
		return err
	}
	if status.State == "" {
		status.State = review.StateStart
	}
	return nil
}

// entryResultRequired meldet, ob ein Eintrag eine Ergebnisdatei braucht.
//
// Für mode: evidence ist die Antwort nein, und zwar unabhängig davon, was im
// Lauf steht: das Pflichtartefakt ist raw/<entry>.sarif. Der Wert in run.json
// wird beim Anlegen zwar schon auf false gesetzt (readAIRecipeMetadata), aber
// die Vorgabe dieser Funktion ist true — ein Altlauf oder eine von Hand
// geschriebene run.json meldete sonst jeden erfolgreichen Evidence-Eintrag als
// resultMissing und inconsistent.
func entryResultRequired(entry review.Entry) bool {
	if review.EntryMode(entry) == review.ModeEvidence {
		return false
	}
	if entry.ResultRequired == nil {
		return true
	}
	return *entry.ResultRequired
}

func listRunFiles(dir string, suffix string) ([]string, reviewToolError) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, reviewToolError{}
		}
		return nil, wrapError(err, "read_failed", "Verzeichnis ist nicht lesbar.", map[string]any{"path": dir})
	}
	files := []string{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), suffix) {
			continue
		}
		files = append(files, resultPath(filepath.Join(filepath.Base(dir), entry.Name())))
	}
	sort.Strings(files)
	return files, reviewToolError{}
}

// listExistingArtifacts sammelt die Artefakte, die im Laufordner wirklich
// liegen, samt ihren Änderungszeiten.
//
// Die Zeiten stehen neben der Pfadzuordnung und nicht in ihr: reviewInput bleibt
// damit die Zuordnung Name → Pfad, die es bisher war, und der Zeitvergleich der
// Bewertung bekommt trotzdem seine Grundlage, ohne dieselben Dateien ein zweites
// Mal zu statten.
func listExistingArtifacts(runDir string, names []string) (map[string]string, map[string]time.Time, reviewToolError) {
	artifacts := map[string]string{}
	modified := map[string]time.Time{}
	for _, name := range names {
		path := filepath.Join(runDir, name)
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, nil, wrapError(err, "read_failed", "Artefakt ist nicht lesbar.", map[string]any{"path": path})
		}
		if info.Mode().IsRegular() {
			artifacts[name] = path
			modified[name] = info.ModTime()
		}
	}
	return artifacts, modified, reviewToolError{}
}

func isNotExist(err error) bool {
	return errors.Is(err, os.ErrNotExist) || os.IsNotExist(err)
}

func resolveReviewEnvironment(inputDir string) (reviewEnvironment, reviewToolError) {
	if strings.TrimSpace(inputDir) == "" {
		return reviewEnvironment{}, reviewToolError{
			Code:    "project_not_found",
			Message: "Kein projectDir angegeben.",
			Details: map[string]any{},
		}
	}
	inputDir = filepath.Clean(inputDir)
	if !filepath.IsAbs(inputDir) {
		workdir, err := os.Getwd()
		if err != nil {
			return reviewEnvironment{}, reviewToolError{
				Code:    "project_not_found",
				Message: "Arbeitsverzeichnis nicht lesbar.",
				Details: map[string]any{"error": err.Error()},
			}
		}
		inputDir = filepath.Join(workdir, inputDir)
	}

	environment := project.DetectFrom(inputDir)
	if !environment.Installed {
		return reviewEnvironment{}, reviewToolError{
			Code:    "project_not_found",
			Message: "Kein k-playbook-Projekt gefunden.",
			Details: map[string]any{"searchedFrom": inputDir, "config": project.ConfigFileName},
		}
	}
	if !environment.PlaybookPresent {
		return reviewEnvironment{}, reviewToolError{
			Code:    "project_not_found",
			Message: "k-playbook-Installation fehlt.",
			Details: map[string]any{"playbookDir": environment.PlaybookDir},
		}
	}

	config, err := project.ReadConfig(environment.ProjectDir)
	if err != nil {
		return reviewEnvironment{}, reviewToolError{
			Code:    "read_failed",
			Message: "Projektkonfiguration nicht lesbar.",
			Details: map[string]any{"path": project.ConfigPath(environment.ProjectDir), "error": err.Error()},
		}
	}
	if err := project.CheckSchema(config); err != nil {
		return reviewEnvironment{}, reviewToolError{
			Code:    "read_failed",
			Message: "Projektkonfiguration hat eine unpassende Fassung.",
			Details: map[string]any{"path": project.ConfigPath(environment.ProjectDir), "error": err.Error()},
		}
	}
	languages, _, err := project.ReadLanguages(environment.ProjectDir)
	if err != nil {
		return reviewEnvironment{}, reviewToolError{
			Code:    "read_failed",
			Message: "Sprachauswahl nicht lesbar.",
			Details: map[string]any{"path": project.ConfigPath(environment.ProjectDir), "error": err.Error()},
		}
	}

	root := environment.ProjectDir
	return reviewEnvironment{
		InputDir:    inputDir,
		Root:        root,
		PlaybookDir: environment.PlaybookDir,
		LocalDir:    project.LocalDir(root),
		TargetDir:   project.RepoRootDir(root, config),
		Languages:   languages,
	}, reviewToolError{}
}

func (env reviewEnvironment) projectEnvelope() reviewProjectEnvelope {
	return reviewProjectEnvelope{
		InputDir:      env.InputDir,
		Root:          env.Root,
		PlaybookDir:   env.PlaybookDir,
		LocalDir:      env.LocalDir,
		ReviewRunsDir: review.ResultsDir(env.LocalDir),
		Languages:     append([]string{}, env.Languages...),
	}
}

func reviewSuccessResult(tool string, project reviewProjectEnvelope, data any, warnings []string) *mcp.CallToolResult {
	if warnings == nil {
		warnings = []string{}
	}
	envelope := reviewEnvelope{OK: true, Tool: tool, Project: project, Data: data, Warnings: warnings}
	return reviewResult(envelope, false)
}

func reviewErrorResult(tool string, project reviewProjectEnvelope, toolErr reviewToolError) *mcp.CallToolResult {
	if toolErr.Details == nil {
		toolErr.Details = map[string]any{}
	}
	envelope := reviewEnvelope{
		OK:      false,
		Tool:    tool,
		Project: project,
		Error: &reviewErrorEnvelope{
			Code:    toolErr.Code,
			Message: toolErr.Message,
			Details: toolErr.Details,
		},
		Warnings: []string{},
	}
	return reviewResult(envelope, true)
}

func reviewResult(envelope reviewEnvelope, toolError bool) *mcp.CallToolResult {
	encoded, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		fallback := reviewEnvelope{
			OK:      false,
			Tool:    envelope.Tool,
			Project: envelope.Project,
			Error: &reviewErrorEnvelope{
				Code:    "execution_failed",
				Message: "Antwort konnte nicht kodiert werden.",
				Details: map[string]any{"error": err.Error()},
			},
			Warnings: []string{},
		}
		encoded, _ = json.MarshalIndent(fallback, "", "  ")
		envelope = fallback
	}
	return &mcp.CallToolResult{
		Content:           []mcp.Content{&mcp.TextContent{Text: string(encoded)}},
		StructuredContent: envelope,
		IsError:           toolError,
	}
}

func runDirFor(env reviewEnvironment, runName string) string {
	return review.RunDir(env.LocalDir, runName)
}

func runExists(runDir string) bool {
	info, err := os.Stat(runDir)
	return err == nil && info.IsDir()
}

func requireRun(env reviewEnvironment, runName string) (string, review.Run, reviewToolError) {
	if strings.TrimSpace(runName) == "" {
		return "", review.Run{}, reviewToolError{
			Code:    "run_not_found",
			Message: "Kein Lauf angegeben.",
			Details: map[string]any{},
		}
	}
	runDir := runDirFor(env, runName)
	run, err := review.ReadRun(runDir)
	if err != nil {
		code := "read_failed"
		message := "Lauf ist nicht lesbar."
		if os.IsNotExist(err) {
			code = "run_not_found"
			message = "Lauf nicht gefunden."
		}
		return "", review.Run{}, reviewToolError{
			Code:    code,
			Message: message,
			Details: map[string]any{"run": runName, "path": filepath.Join(runDir, review.RunFileName), "error": err.Error()},
		}
	}
	return runDir, run, reviewToolError{}
}

func wrapReviewTool(tool string, inputDir string, fn func(reviewEnvironment) *mcp.CallToolResult) *mcp.CallToolResult {
	env, toolErr := resolveReviewEnvironment(inputDir)
	if toolErr.Code != "" {
		return reviewErrorResult(tool, reviewProjectEnvelope{InputDir: inputDir}, toolErr)
	}
	return fn(env)
}

func wrapError(err error, code string, message string, details map[string]any) reviewToolError {
	if details == nil {
		details = map[string]any{}
	}
	if err != nil {
		details["error"] = err.Error()
	}
	return reviewToolError{Code: code, Message: message, Details: details}
}

func stringsSet(values []string) map[string]bool {
	set := map[string]bool{}
	for _, value := range values {
		set[value] = true
	}
	return set
}

func toolAppliesToLanguages(languages string, selected map[string]bool) bool {
	for _, language := range strings.Split(languages, ",") {
		language = strings.TrimSpace(language)
		if language == "*" || selected[language] {
			return true
		}
	}
	return false
}

func originLabel(origin string) string {
	switch origin {
	case "dist":
		return "mitgeliefert"
	case "local":
		return "projekteigen"
	case "override":
		return "projekteigen, ersetzt mitgeliefert"
	default:
		return origin
	}
}

func ensureEntryName(name string) reviewToolError {
	if review.ValidEntryName(name) {
		return reviewToolError{}
	}
	return reviewToolError{
		Code:    "invalid_selection",
		Message: "Eintragsname ist ungültig.",
		Details: map[string]any{"entry": name},
	}
}

func readJSONFile(path string, target any) reviewToolError {
	data, err := os.ReadFile(path)
	if err != nil {
		return wrapError(err, "read_failed", "Datei ist nicht lesbar.", map[string]any{"path": path})
	}
	if err := json.Unmarshal(data, target); err != nil {
		return wrapError(err, "read_failed", "JSON-Datei ist nicht lesbar.", map[string]any{"path": path})
	}
	return reviewToolError{}
}

func resultPath(path string) string {
	return filepath.ToSlash(path)
}
