package mcpserver

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kascada/k-playbook/installer/internal/project"
	"github.com/kascada/k-playbook/installer/internal/review"
)

func TestReviewStatusAvailable(t *testing.T) {
	root := newReviewProject(t)
	result, _, err := reviewStatusTool(context.Background(), nil, reviewStatusInput{reviewBaseInput: reviewBaseInput{ProjectDir: root}, Mode: "available"})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	envelope := decodeReviewEnvelope(t, result)
	if !envelope.OK {
		t.Fatalf("status fehlgeschlagen: %#v", envelope.Error)
	}
	data := envelope.Data.(map[string]any)
	if data["mode"] != "available" {
		t.Fatalf("mode = %v", data["mode"])
	}
	selection := data["selection"].(map[string]any)
	candidates := selection["candidates"].([]any)
	if len(candidates) != 3 {
		t.Fatalf("candidates = %v", candidates)
	}
	if defaults := selection["defaultEntries"].([]any); len(defaults) != 2 {
		t.Fatalf("defaultEntries = %v", defaults)
	}
}

func TestReviewStatusExisting(t *testing.T) {
	root := newReviewProject(t)
	runDir := mustCreateRun(t, root, []review.Entry{{Name: "mockscan", Kind: review.KindTool}, aiEntry("tech")})
	mustWriteJSON(t, review.EntryFile(runDir, "mockscan"), review.EntryStatus{Name: "mockscan", Kind: review.KindTool, State: review.StateDone})
	mustWriteFile(t, filepath.Join(runDir, review.RawDirName, "mockscan.sarif"), minimalSARIF())
	mustWriteFile(t, filepath.Join(runDir, "review-input.md"), "# Review\n")

	result, _, err := reviewStatusTool(context.Background(), nil, reviewStatusInput{reviewBaseInput: reviewBaseInput{ProjectDir: root}, Run: "2026-08-19"})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	envelope := decodeReviewEnvelope(t, result)
	if !envelope.OK {
		t.Fatalf("status fehlgeschlagen: %#v", envelope.Error)
	}
	data := envelope.Data.(map[string]any)
	if data["state"] != string(review.StateRunning) {
		t.Fatalf("state = %v", data["state"])
	}
	if len(data["rawSarif"].([]any)) != 1 {
		t.Fatalf("rawSarif = %v", data["rawSarif"])
	}
}

func TestReviewStatusProjectFehltUndModeInvalid(t *testing.T) {
	result, _, err := reviewStatusTool(context.Background(), nil, reviewStatusInput{reviewBaseInput: reviewBaseInput{ProjectDir: t.TempDir()}, Mode: "available"})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	assertToolErrorCode(t, result, "project_not_found")

	root := newReviewProject(t)
	result, _, err = reviewStatusTool(context.Background(), nil, reviewStatusInput{reviewBaseInput: reviewBaseInput{ProjectDir: root}, Mode: "existing"})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	assertToolErrorCode(t, result, "invalid_mode")
}

func TestReviewCreateDryRunUndEcht(t *testing.T) {
	root := newReviewProject(t)
	result, _, err := reviewCreateTool(context.Background(), nil, reviewCreateInput{reviewBaseInput: reviewBaseInput{ProjectDir: root}, Day: "2026-08-19", DryRun: true})
	if err != nil {
		t.Fatalf("create dry-run: %v", err)
	}
	envelope := decodeReviewEnvelope(t, result)
	if !envelope.OK {
		t.Fatalf("dry-run fehlgeschlagen: %#v", envelope.Error)
	}
	if _, err := os.Stat(filepath.Join(project.LocalDir(root), review.ResultsDirName, "2026-08-19")); !os.IsNotExist(err) {
		t.Fatalf("dry-run hat Lauf angelegt")
	}

	result, _, err = reviewCreateTool(context.Background(), nil, reviewCreateInput{reviewBaseInput: reviewBaseInput{ProjectDir: root}, Day: "2026-08-19"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if envelope = decodeReviewEnvelope(t, result); !envelope.OK {
		t.Fatalf("create fehlgeschlagen: %#v", envelope.Error)
	}
	written, err := review.ReadRun(filepath.Join(project.LocalDir(root), review.ResultsDirName, "2026-08-19"))
	if err != nil {
		t.Fatalf("run.json lesen: %v", err)
	}
	if len(written.Entries) != 2 || written.Entries[1].RecipeKey != "tech" {
		t.Fatalf("entries = %#v", written.Entries)
	}
}

func TestReviewCreateAuswahlfehler(t *testing.T) {
	root := newReviewProject(t)
	tests := []struct {
		name    string
		entries []reviewSelectionInput
		code    string
	}{
		{name: "unbekannt", entries: []reviewSelectionInput{{Name: "missing", Kind: review.KindTool}}, code: "selection_unknown"},
		{name: "falsche Art", entries: []reviewSelectionInput{{Name: "tech", Kind: review.KindTool}}, code: "invalid_selection"},
		{name: "nicht installiert", entries: []reviewSelectionInput{{Name: "missingtool", Kind: review.KindTool}}, code: "selection_unavailable"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, _, err := reviewCreateTool(context.Background(), nil, reviewCreateInput{reviewBaseInput: reviewBaseInput{ProjectDir: root}, Day: "2026-08-19", Entries: test.entries})
			if err != nil {
				t.Fatalf("create: %v", err)
			}
			assertToolErrorCode(t, result, test.code)
		})
	}
}

func TestReviewCreateRunExists(t *testing.T) {
	root := newReviewProject(t)
	mustCreateRun(t, root, []review.Entry{{Name: "mockscan", Kind: review.KindTool}})
	result, _, err := reviewCreateTool(context.Background(), nil, reviewCreateInput{reviewBaseInput: reviewBaseInput{ProjectDir: root}, Day: "2026-08-19"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	assertToolErrorCode(t, result, "run_exists")
}

func TestReviewScan(t *testing.T) {
	root := newReviewProject(t)
	mustCreateRun(t, root, []review.Entry{{Name: "mockscan", Kind: review.KindTool}, aiEntry("tech")})
	result, _, err := reviewScanTool(context.Background(), nil, reviewScanInput{reviewBaseInput: reviewBaseInput{ProjectDir: root}, Run: "2026-08-19"})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	envelope := decodeReviewEnvelope(t, result)
	if !envelope.OK {
		t.Fatalf("scan fehlgeschlagen: %#v", envelope.Error)
	}
	data := envelope.Data.(map[string]any)
	entries := data["entries"].([]any)
	if len(entries) != 1 {
		t.Fatalf("entries = %v", entries)
	}
	if _, err := os.Stat(filepath.Join(project.LocalDir(root), review.ResultsDirName, "2026-08-19", review.RawDirName, "mockscan.sarif")); err != nil {
		t.Fatalf("SARIF fehlt: %v", err)
	}
}

func TestReviewScanFehler(t *testing.T) {
	root := newReviewProject(t)
	mustCreateRun(t, root, []review.Entry{{Name: "mockscan", Kind: review.KindTool}, aiEntry("tech")})
	result, _, err := reviewScanTool(context.Background(), nil, reviewScanInput{reviewBaseInput: reviewBaseInput{ProjectDir: root}, Run: "2026-08-19", Entries: []string{"tech"}})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	assertToolErrorCode(t, result, "entry_kind_invalid")

	result, _, err = reviewScanTool(context.Background(), nil, reviewScanInput{reviewBaseInput: reviewBaseInput{ProjectDir: root}, Run: "2026-08-19", Entries: []string{"missing"}})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	assertToolErrorCode(t, result, "selection_unknown")
}

func TestReviewMerge(t *testing.T) {
	root := newReviewProject(t)
	runDir := mustCreateRun(t, root, []review.Entry{{Name: "mockscan", Kind: review.KindTool}})
	mustWriteJSON(t, review.EntryFile(runDir, "mockscan"), review.EntryStatus{
		Name:  "mockscan",
		Kind:  review.KindTool,
		State: review.StateDone,
		Jobs:  []review.JobStatus{{Job: "mockscan", State: review.StateDone, SARIF: "raw/mockscan.sarif", Findings: intPtr(1)}},
	})
	mustWriteFile(t, filepath.Join(runDir, review.RawDirName, "mockscan.sarif"), minimalSARIF())
	result, _, err := reviewMergeTool(context.Background(), nil, reviewMergeInput{reviewBaseInput: reviewBaseInput{ProjectDir: root}, Run: "2026-08-19"})
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	envelope := decodeReviewEnvelope(t, result)
	if !envelope.OK {
		t.Fatalf("merge fehlgeschlagen: %#v", envelope.Error)
	}
	for _, name := range []string{"review-input.json", "review-input.md"} {
		if _, err := os.Stat(filepath.Join(runDir, name)); err != nil {
			t.Fatalf("%s fehlt: %v", name, err)
		}
	}
}

func TestReviewWriteAIEntry(t *testing.T) {
	root := newReviewProject(t)
	runDir := mustCreateRun(t, root, []review.Entry{{Name: "mockscan", Kind: review.KindTool}, aiEntry("tech")})
	mustWriteFile(t, filepath.Join(runDir, "review-tech.md"), "# Ergebnis\n")
	result, _, err := reviewWriteAIEntryTool(context.Background(), nil, reviewWriteAIEntryInput{
		reviewBaseInput: reviewBaseInput{ProjectDir: root},
		Run:             "2026-08-19",
		Entry:           "tech",
		State:           review.StateDone,
		Result:          "review-tech.md",
		StartedAt:       "2026-08-19T10:00:00Z",
		FinishedAt:      "2026-08-19T10:05:00Z",
	})
	if err != nil {
		t.Fatalf("write_ai_entry: %v", err)
	}
	envelope := decodeReviewEnvelope(t, result)
	if !envelope.OK {
		t.Fatalf("write_ai_entry fehlgeschlagen: %#v", envelope.Error)
	}
	var status aiEntryStatus
	if err := readAIEntryStatus(runDir, "tech", &status); err != nil {
		t.Fatalf("AI-Status lesen: %v", err)
	}
	if status.Result != "review-tech.md" || status.State != review.StateDone {
		t.Fatalf("status = %#v", status)
	}
}

func TestReviewWriteAIEntryFehler(t *testing.T) {
	root := newReviewProject(t)
	mustCreateRun(t, root, []review.Entry{{Name: "mockscan", Kind: review.KindTool}, aiEntry("tech")})
	tests := []struct {
		name  string
		input reviewWriteAIEntryInput
		code  string
	}{
		{name: "unbekannt", input: reviewWriteAIEntryInput{Run: "2026-08-19", Entry: "missing", State: review.StateDone, Result: "x.md"}, code: "entry_not_found"},
		{name: "tool", input: reviewWriteAIEntryInput{Run: "2026-08-19", Entry: "mockscan", State: review.StateDone, Result: "x.md"}, code: "entry_kind_invalid"},
		{name: "done ohne Result", input: reviewWriteAIEntryInput{Run: "2026-08-19", Entry: "tech", State: review.StateDone}, code: "result_required"},
		{name: "Pfad", input: reviewWriteAIEntryInput{Run: "2026-08-19", Entry: "tech", State: review.StateDone, Result: "../x.md"}, code: "result_path_invalid"},
		{name: "failed ohne Grund", input: reviewWriteAIEntryInput{Run: "2026-08-19", Entry: "tech", State: review.StateFailed}, code: "entry_state_invalid"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.input.reviewBaseInput = reviewBaseInput{ProjectDir: root}
			result, _, err := reviewWriteAIEntryTool(context.Background(), nil, test.input)
			if err != nil {
				t.Fatalf("write_ai_entry: %v", err)
			}
			assertToolErrorCode(t, result, test.code)
		})
	}
}

func newReviewProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dirs := []string{
		filepath.Join(root, "k-playbook", "scripts"),
		filepath.Join(root, "k-playbook", "reviews"),
		filepath.Join(root, "k-playbook", "rules"),
		filepath.Join(root, "k-playbook", "checks"),
		filepath.Join(root, "k-playbook-local", "reviews"),
		filepath.Join(root, "k-playbook-local", review.ResultsDirName),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("%s anlegen: %v", dir, err)
		}
	}
	mustWriteFile(t, filepath.Join(root, "K-PLAYBOOK.yaml"), "schema_version: 3\n\nproject:\n  repo_root: .\n  vcs: none\n  languages:\n    - go\n")
	mustWriteFile(t, filepath.Join(root, "k-playbook", "reviews", "review-tech.md"), "---\nreviewRun:\n  title: Technischer Review\n  resultRequired: true\n  defaultResult: review-tech.md\n---\n# Fallback\n")
	mustWriteFile(t, filepath.Join(root, "k-playbook", "scripts", "severity.tsv"), "tool\trule_prefix\tseverity\tnotes\n")
	mustWriteFile(t, filepath.Join(root, "k-playbook", "scripts", "scanners.tsv"), "job\ttool\tlanguages\tcandidates\tsarif\toutput\ttimeout\tsoft_skip\tworkdir\targs\nmockscan\tmockscan\tgo\tsource\tnative\tstdout\t5s\t\ttarget\t--sarif\n")
	mustWriteFile(t, filepath.Join(root, "k-playbook", "scripts", "install-security-tools.sh"), preflightScript(t))
	if err := os.Chmod(filepath.Join(root, "k-playbook", "scripts", "install-security-tools.sh"), 0o755); err != nil {
		t.Fatalf("Preflight chmod: %v", err)
	}
	mustWriteFile(t, filepath.Join(root, "mockscan"), "#!/usr/bin/env bash\nprintf '%s' '{\"version\":\"2.1.0\",\"runs\":[{\"tool\":{\"driver\":{\"name\":\"mockscan\",\"rules\":[{\"id\":\"R1\",\"name\":\"Regel\"}]}},\"results\":[{\"ruleId\":\"R1\",\"level\":\"warning\",\"message\":{\"text\":\"Fund\"},\"locations\":[{\"physicalLocation\":{\"artifactLocation\":{\"uri\":\"main.go\"},\"region\":{\"startLine\":1}}}]}]}]}'\n")
	if err := os.Chmod(filepath.Join(root, "mockscan"), 0o755); err != nil {
		t.Fatalf("mockscan chmod: %v", err)
	}
	mustWriteFile(t, filepath.Join(root, "main.go"), "package main\n")
	return root
}

func preflightScript(t *testing.T) string {
	t.Helper()
	return `#!/usr/bin/env bash
root="$(cd "$(dirname "$0")/../.." && pwd)"
cat <<JSON
{
  "playbookDir": "$root/k-playbook",
  "languages": "go",
  "tools": [
    {"name":"mockscan","languages":"go","status":"ok","path":"$root/mockscan","role":"Mock-Scanner"},
    {"name":"missingtool","languages":"go","status":"missing","role":"Fehlt"}
  ]
}
JSON
`
}

func mustCreateRun(t *testing.T, root string, entries []review.Entry) string {
	t.Helper()
	runDir, err := review.CreateRun(project.LocalDir(root), time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC), []string{"go"}, entries)
	if err != nil {
		t.Fatalf("Lauf anlegen: %v", err)
	}
	return runDir
}

func aiEntry(name string) review.Entry {
	required := true
	return review.Entry{Name: name, Kind: review.KindAI, RecipeKey: name, RecipePath: "/review-" + name + ".md", RecipeOrigin: "dist", Title: "Technischer Review", ResultRequired: &required, DefaultResult: "review-" + name + ".md"}
}

func decodeReviewEnvelope(t *testing.T, result *mcp.CallToolResult) reviewEnvelope {
	t.Helper()
	if result == nil || len(result.Content) == 0 {
		t.Fatalf("kein Werkzeugergebnis")
	}
	text := result.Content[0].(*mcp.TextContent).Text
	var envelope reviewEnvelope
	if err := json.Unmarshal([]byte(text), &envelope); err != nil {
		t.Fatalf("Envelope nicht lesbar: %v\n%s", err, text)
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(text), &raw); err != nil {
		t.Fatalf("Envelope map nicht lesbar: %v", err)
	}
	envelope.Data = raw["data"]
	return envelope
}

func assertToolErrorCode(t *testing.T, result *mcp.CallToolResult, code string) {
	t.Helper()
	envelope := decodeReviewEnvelope(t, result)
	if envelope.OK {
		t.Fatalf("Werkzeug war erfolgreich, erwartet %s", code)
	}
	if envelope.Error == nil || envelope.Error.Code != code {
		t.Fatalf("Fehlercode = %#v, erwartet %s", envelope.Error, code)
	}
}

func mustWriteFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("%s Verzeichnis anlegen: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("%s schreiben: %v", path, err)
	}
}

func mustWriteJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("JSON kodieren: %v", err)
	}
	mustWriteFile(t, path, string(append(data, '\n')))
}

func minimalSARIF() string {
	return `{"version":"2.1.0","runs":[{"tool":{"driver":{"name":"mockscan","rules":[{"id":"R1","name":"Regel"}]}},"results":[{"ruleId":"R1","level":"warning","message":{"text":"Fund"},"locations":[{"physicalLocation":{"artifactLocation":{"uri":"main.go"},"region":{"startLine":1}}}]}]}]}`
}

func intPtr(value int) *int { return &value }
