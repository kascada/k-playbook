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
	AuditEnabled      *bool       `json:"auditEnabled,omitempty"`
	ReviewEnabled     *bool       `json:"reviewEnabled,omitempty"`
	ResultRequired    *bool       `json:"resultRequired,omitempty"`
	DefaultResult     string      `json:"defaultResult,omitempty"`
}

type reviewSelectionBase struct {
	Languages             []string          `json:"languages"`
	Candidates            []reviewCandidate `json:"candidates"`
	UnavailableCandidates []reviewCandidate `json:"unavailableCandidates"`
	DefaultEntries        []review.Entry    `json:"defaultEntries"`
	Preflight             any               `json:"preflight,omitempty"`
}

type aiRecipeMetadata struct {
	Enabled        bool
	ReviewEnabled  bool
	Title          string
	ResultRequired bool
	DefaultResult  string
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
		data, toolErr := scanReviewRun(ctx, env, input)
		if toolErr.Code != "" {
			return reviewErrorResult(reviewToolScan, env.projectEnvelope(), toolErr)
		}
		return reviewSuccessResult(reviewToolScan, env.projectEnvelope(), data, nil)
	}), nil, nil
}

func reviewMergeTool(ctx context.Context, req *mcp.CallToolRequest, input reviewMergeInput) (*mcp.CallToolResult, any, error) {
	return wrapReviewTool(reviewToolMerge, input.ProjectDir, func(env reviewEnvironment) *mcp.CallToolResult {
		data, toolErr := mergeReviewRun(env, input)
		if toolErr.Code != "" {
			return reviewErrorResult(reviewToolMerge, env.projectEnvelope(), toolErr)
		}
		return reviewSuccessResult(reviewToolMerge, env.projectEnvelope(), data, nil)
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

func scanReviewRun(ctx context.Context, env reviewEnvironment, input reviewScanInput) (map[string]any, reviewToolError) {
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
	statuses, err := review.Execute(ctx, entries, review.Options{
		RunDir:     runDir,
		Target:     env.TargetDir,
		ScriptsDir: filepath.Join(env.PlaybookDir, "scripts"),
		Languages:  run.Languages,
		Scanners:   scanners,
		Tools:      resolveReviewTools(preflight),
	})
	if err != nil {
		return nil, wrapError(err, "execution_failed", "Scan-Ausführung ist abgebrochen.", map[string]any{"run": input.Run})
	}
	return map[string]any{
		"run":     input.Run,
		"runDir":  runDir,
		"state":   review.DeriveRunState(runDir, run),
		"entries": statuses,
	}, reviewToolError{}
}

func mergeReviewRun(env reviewEnvironment, input reviewMergeInput) (map[string]any, reviewToolError) {
	runDir, run, toolErr := requireRun(env, input.Run)
	if toolErr.Code != "" {
		return nil, toolErr
	}
	result, output, err := merge.Run(merge.Options{
		ProjectDir:          env.Root,
		RunName:             input.Run,
		RunDir:              runDir,
		KPlaybookVersion:    mcpKPlaybookVersion(),
		SeverityMappingPath: merge.SeverityCatalog(env.PlaybookDir),
		LocalResultsDir:     review.ResultsDir(env.LocalDir),
	})
	if err != nil {
		code := "merge_failed"
		message := "Review-Lauf konnte nicht zusammengeführt werden."
		if isNotExist(err) {
			code = "read_failed"
			message = "Merge-Eingabe ist nicht lesbar."
		}
		return nil, wrapError(err, code, message, map[string]any{"run": input.Run, "runDir": runDir})
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
	}, reviewToolError{}
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
	if err := writeAIEntryStatusFile(runDir, aiStatus); err != nil {
		return nil, wrapError(err, "write_failed", "AI-Entry-Status konnte nicht geschrieben werden.", map[string]any{"entry": input.Entry, "path": review.EntryFile(runDir, input.Entry)})
	}
	return map[string]any{
		"run":      input.Run,
		"runDir":   runDir,
		"entry":    input.Entry,
		"path":     review.EntryFile(runDir, input.Entry),
		"status":   aiStatus,
		"runState": review.DeriveRunState(runDir, effectiveRunForStatus(env, run)),
	}, reviewToolError{}
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
	data = append(data, '\n')
	target := review.EntryFile(runDir, status.Name)
	dir := filepath.Dir(target)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(dir, "."+status.Name+".*.json")
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
	artifacts, toolErr := listExistingArtifacts(runDir, []string{"review-input.json", "review-input.md", scanTriageResult})
	if toolErr.Code != "" {
		return nil, toolErr
	}
	return map[string]any{
		"mode":        "existing",
		"run":         runName,
		"runDir":      runDir,
		"runJSON":     run,
		"state":       review.DeriveRunState(runDir, effectiveRun),
		"entries":     entries,
		"rawSarif":    raw,
		"reviewInput": artifacts,
	}, reviewToolError{}
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
		candidates = append(candidates, reviewCandidate{
			Name:            entry.Key,
			Kind:            review.KindAI,
			Title:           metadata.Title,
			Selectable:      true,
			DefaultSelected: true,
			Detail:          originLabel(entry.Origin),
			RecipeKey:       entry.Key,
			RecipePath:      entry.Path,
			RecipeOrigin:    entry.Origin,
			AuditEnabled:    &auditEnabled,
			ReviewEnabled:   &reviewEnabled,
			ResultRequired:  &resultRequired,
			DefaultResult:   metadata.DefaultResult,
		})
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
	for _, candidate := range candidates {
		if !candidate.Selectable {
			unavailable = append(unavailable, candidate)
			continue
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
		entry.ResultRequired = candidate.ResultRequired
		entry.DefaultResult = candidate.DefaultResult
	}
	return entry
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
	blockIndent := -1
	for _, line := range strings.Split(frontmatter, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || trimmed == "---" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		if strings.HasSuffix(trimmed, ":") {
			key := strings.TrimSuffix(trimmed, ":")
			if indent <= blockIndent {
				inAudit = false
				inReview = false
			}
			if key == "audit" || key == "review" {
				inAudit = key == "audit"
				inReview = key == "review"
				blockIndent = indent
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
		if !inAudit {
			continue
		}
		switch field {
		case "enabled":
			metadata.Enabled = value != "false"
		case "title":
			metadata.Title = value
		case "resultRequired":
			metadata.ResultRequired = value != "false"
		case "defaultResult":
			metadata.DefaultResult = value
		}
	}
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
			item["resultRequired"] = entryResultRequired(entry)
			if entry.DefaultResult != "" {
				item["defaultResult"] = entry.DefaultResult
			}
			status := aiEntryStatus{}
			err := readAIEntryStatus(runDir, entry.Name, &status)
			if err != nil {
				if os.IsNotExist(err) {
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
			if status.State == review.StateDone && entryResultRequired(entry) && status.Result == "" {
				item["resultMissing"] = true
			}
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

type aiEntryStatus struct {
	Name       string       `json:"name"`
	Kind       review.Kind  `json:"kind"`
	State      review.State `json:"state"`
	Result     string       `json:"result,omitempty"`
	Reason     string       `json:"reason,omitempty"`
	StartedAt  string       `json:"startedAt,omitempty"`
	FinishedAt string       `json:"finishedAt,omitempty"`
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

func entryResultRequired(entry review.Entry) bool {
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

func listExistingArtifacts(runDir string, names []string) (map[string]string, reviewToolError) {
	artifacts := map[string]string{}
	for _, name := range names {
		path := filepath.Join(runDir, name)
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, wrapError(err, "read_failed", "Artefakt ist nicht lesbar.", map[string]any{"path": path})
		}
		if info.Mode().IsRegular() {
			artifacts[name] = path
		}
	}
	return artifacts, reviewToolError{}
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
