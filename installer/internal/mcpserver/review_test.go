package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	if len(candidates) != 4 {
		t.Fatalf("candidates = %v", candidates)
	}
	if !hasCandidate(candidates, scanTriageEntry) {
		t.Fatalf("scan-triage fehlt in candidates: %v", candidates)
	}
	if defaults := selection["defaultEntries"].([]any); len(defaults) != 3 {
		t.Fatalf("defaultEntries = %v", defaults)
	}
}

func TestReviewStatusAvailableTrenntAuditUndReviewAktivierung(t *testing.T) {
	root := newReviewProject(t)
	mustWriteFile(t, filepath.Join(root, "k-playbook", "reviews", "review-review-only.md"), "---\ntitle: Review Only\nreview:\n  enabled: true\n---\n# Review Only\n")
	mustWriteFile(t, filepath.Join(root, "k-playbook", "reviews", "review-disabled.md"), "---\naudit:\n  enabled: false\nreview:\n  enabled: false\n---\n# Disabled\n")

	result, _, err := reviewStatusTool(context.Background(), nil, reviewStatusInput{reviewBaseInput: reviewBaseInput{ProjectDir: root}, Mode: "available"})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	envelope := decodeReviewEnvelope(t, result)
	if !envelope.OK {
		t.Fatalf("status fehlgeschlagen: %#v", envelope.Error)
	}
	selection := envelope.Data.(map[string]any)["selection"].(map[string]any)
	candidates := selection["candidates"].([]any)
	if !hasCandidate(candidates, "tech") {
		t.Fatalf("audit-aktiviertes Rezept fehlt: %v", candidates)
	}
	if hasCandidate(candidates, "review-only") || hasCandidate(candidates, "disabled") {
		t.Fatalf("nicht audit-aktivierte Rezepte in Audit-Auswahl: %v", candidates)
	}

	context, err := project.BuildContext(root)
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}
	entry := reviewCatalogEntry(t, context.Catalogs["reviews"], "review-review-only.md")
	if entry.Audit == nil || entry.Audit.Enabled || entry.Review == nil || !entry.Review.Enabled {
		t.Fatalf("review-only Modi = audit:%#v review:%#v", entry.Audit, entry.Review)
	}
}

func TestReviewStatusAvailableOhneScanTriageBeiLeeremOverlay(t *testing.T) {
	root := newReviewProject(t)
	mustWriteFile(t, filepath.Join(root, project.LocalDirName, "commands", filepath.FromSlash(scanTriageModule)), "# abgeschaltet\n")

	result, _, err := reviewStatusTool(context.Background(), nil, reviewStatusInput{reviewBaseInput: reviewBaseInput{ProjectDir: root}, Mode: "available"})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	envelope := decodeReviewEnvelope(t, result)
	if !envelope.OK {
		t.Fatalf("status fehlgeschlagen: %#v", envelope.Error)
	}
	selection := envelope.Data.(map[string]any)["selection"].(map[string]any)
	candidates := selection["candidates"].([]any)
	if hasCandidate(candidates, scanTriageEntry) {
		t.Fatalf("scan-triage trotz leerem Overlay enthalten: %v", candidates)
	}
}

func TestReviewStatusAvailableScanTriageOverlayOverride(t *testing.T) {
	root := newReviewProject(t)
	mustWriteFile(t, filepath.Join(root, project.LocalDirName, "commands", filepath.FromSlash(scanTriageModule)), "# Lokales Modul\n\nAktiv.\n")

	result, _, err := reviewStatusTool(context.Background(), nil, reviewStatusInput{reviewBaseInput: reviewBaseInput{ProjectDir: root}, Mode: "available"})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	envelope := decodeReviewEnvelope(t, result)
	if !envelope.OK {
		t.Fatalf("status fehlgeschlagen: %#v", envelope.Error)
	}
	selection := envelope.Data.(map[string]any)["selection"].(map[string]any)
	candidate := candidateByName(t, selection["candidates"].([]any), scanTriageEntry)
	if candidate["recipeOrigin"] != "override" {
		t.Fatalf("scan-triage Origin = %v, erwartet override", candidate["recipeOrigin"])
	}
	if path := candidate["recipePath"].(string); filepath.Base(filepath.Dir(path)) != "_audit" {
		t.Fatalf("scan-triage Pfad = %s, erwartet _audit", path)
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
	entries := data["entries"].([]any)
	if !hasStatusEntry(entries, scanTriageEntry) {
		t.Fatalf("synthetischer scan-triage fehlt: %v", entries)
	}
	if len(data["rawSarif"].([]any)) != 1 {
		t.Fatalf("rawSarif = %v", data["rawSarif"])
	}
}

func TestReviewStatusExistingMarkiertAIDoneOhneResultAlsInkonsistent(t *testing.T) {
	root := newReviewProject(t)
	runDir := mustCreateRun(t, root, []review.Entry{{Name: "mockscan", Kind: review.KindTool}, aiEntry("tech")})
	mustWriteJSON(t, review.EntryFile(runDir, "tech"), aiEntryStatus{Name: "tech", Kind: review.KindAI, State: review.StateDone, Result: "review-tech.md"})

	result, _, err := reviewStatusTool(context.Background(), nil, reviewStatusInput{reviewBaseInput: reviewBaseInput{ProjectDir: root}, Run: "2026-08-19"})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	envelope := decodeReviewEnvelope(t, result)
	if !envelope.OK {
		t.Fatalf("status fehlgeschlagen: %#v", envelope.Error)
	}
	entry := statusEntry(t, envelope.Data.(map[string]any)["entries"].([]any), "tech")
	if entry["resultMissing"] != true || entry["inconsistent"] != true {
		t.Fatalf("AI-Status = %#v", entry)
	}
}

func TestReviewStatusExistingMarkiertVorhandenesAIResultAlsReparabel(t *testing.T) {
	root := newReviewProject(t)
	runDir := mustCreateRun(t, root, []review.Entry{{Name: "mockscan", Kind: review.KindTool}, aiEntry("tech")})
	mustWriteFile(t, filepath.Join(runDir, "review-tech.md"), "# Ergebnis\n")

	result, _, err := reviewStatusTool(context.Background(), nil, reviewStatusInput{reviewBaseInput: reviewBaseInput{ProjectDir: root}, Run: "2026-08-19"})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	envelope := decodeReviewEnvelope(t, result)
	if !envelope.OK {
		t.Fatalf("status fehlgeschlagen: %#v", envelope.Error)
	}
	entry := statusEntry(t, envelope.Data.(map[string]any)["entries"].([]any), "tech")
	if entry["repairable"] != true || entry["resultExists"] != true {
		t.Fatalf("AI-Status = %#v", entry)
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
	if len(written.Entries) != 3 || written.Entries[1].RecipeKey != scanTriageEntry || written.Entries[2].RecipeKey != "tech" {
		t.Fatalf("entries = %#v", written.Entries)
	}
	if written.Entries[2].Scope == nil || len(written.Entries[2].Scope.Tools) != 1 || written.Entries[2].Scope.Tools[0] != "mockscan" {
		t.Fatalf("Scope-Snapshot = %#v", written.Entries[2].Scope)
	}
	if _, err := os.Stat(review.EntryFile(filepath.Join(project.LocalDir(root), review.ResultsDirName, "2026-08-19"), scanTriageEntry)); err != nil {
		t.Fatalf("scan-triage Entry fehlt: %v", err)
	}
}

func TestReviewCreateDryRunZeigtScopeSnapshot(t *testing.T) {
	root := newReviewProject(t)
	result, _, err := reviewCreateTool(context.Background(), nil, reviewCreateInput{reviewBaseInput: reviewBaseInput{ProjectDir: root}, Day: "2026-08-19", DryRun: true})
	if err != nil {
		t.Fatalf("create dry-run: %v", err)
	}
	envelope := decodeReviewEnvelope(t, result)
	if !envelope.OK {
		t.Fatalf("dry-run fehlgeschlagen: %#v", envelope.Error)
	}
	data := envelope.Data.(map[string]any)
	runJSON := data["runJSON"].(map[string]any)
	entries := runJSON["entries"].([]any)
	tech := statusEntry(t, entries, "tech")
	scope := tech["scope"].(map[string]any)
	tools := scope["tools"].([]any)
	if len(tools) != 1 || tools[0] != "mockscan" {
		t.Fatalf("runJSON Scope = %#v", scope)
	}
	candidate := candidateByName(t, data["validatedCandidates"].([]any), "tech")
	candidateScope := candidate["scope"].(map[string]any)
	candidateTools := candidateScope["tools"].([]any)
	if len(candidateTools) != 1 || candidateTools[0] != "mockscan" {
		t.Fatalf("Candidate Scope = %#v", candidateScope)
	}
}

func TestReviewStatusExistingZeigtGespeichertenScopeSnapshot(t *testing.T) {
	root := newReviewProject(t)
	entry := aiEntry("tech")
	entry.Scope = &review.Scope{Tools: []string{"old-tool"}}
	mustCreateRun(t, root, []review.Entry{{Name: "mockscan", Kind: review.KindTool}, entry})
	mustWriteFile(t, filepath.Join(root, "k-playbook", "reviews", "review-tech.md"), "---\ntitle: Technischer Review\naudit:\n  enabled: true\n  resultRequired: true\n  defaultResult: review-tech.md\n  scope:\n    tools: [new-tool]\nreview:\n  enabled: true\n---\n# Fallback\n")

	result, _, err := reviewStatusTool(context.Background(), nil, reviewStatusInput{reviewBaseInput: reviewBaseInput{ProjectDir: root}, Run: "2026-08-19"})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	envelope := decodeReviewEnvelope(t, result)
	if !envelope.OK {
		t.Fatalf("status fehlgeschlagen: %#v", envelope.Error)
	}
	entryStatus := statusEntry(t, envelope.Data.(map[string]any)["entries"].([]any), "tech")
	scope := entryStatus["scope"].(map[string]any)
	tools := scope["tools"].([]any)
	if len(tools) != 1 || tools[0] != "old-tool" {
		t.Fatalf("Status Scope = %#v", scope)
	}
}

func TestReviewCreateAkzeptiertScanTriageAuswahl(t *testing.T) {
	root := newReviewProject(t)
	result, _, err := reviewCreateTool(context.Background(), nil, reviewCreateInput{
		reviewBaseInput: reviewBaseInput{ProjectDir: root},
		Day:             "2026-08-19",
		Entries:         []reviewSelectionInput{{Name: scanTriageEntry, Kind: review.KindAI}},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	envelope := decodeReviewEnvelope(t, result)
	if !envelope.OK {
		t.Fatalf("create fehlgeschlagen: %#v", envelope.Error)
	}
	runDir := filepath.Join(project.LocalDir(root), review.ResultsDirName, "2026-08-19")
	written, err := review.ReadRun(runDir)
	if err != nil {
		t.Fatalf("run.json lesen: %v", err)
	}
	if len(written.Entries) != 1 || written.Entries[0].Name != scanTriageEntry || written.Entries[0].RecipePath == "" {
		t.Fatalf("entries = %#v", written.Entries)
	}
	var status aiEntryStatus
	if err := readAIEntryStatus(runDir, scanTriageEntry, &status); err != nil {
		t.Fatalf("scan-triage Status lesen: %v", err)
	}
	if status.State != review.StateStart {
		t.Fatalf("scan-triage Status = %#v", status)
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

func TestReviewScanSendetProgressMitToken(t *testing.T) {
	root := newReviewProject(t)
	mustWriteFile(t, filepath.Join(root, "k-playbook", "scripts", "scanners.tsv"), "job\ttool\tlanguages\tcandidates\tsarif\toutput\ttimeout\tsoft_skip\tworkdir\targs\nalpha\talpha\tgo\tsource\tnative\tstdout\t5s\t\ttarget\t--sarif\nbeta\tbeta\tgo\tsource\tnative\tstdout\t5s\t\ttarget\t--sarif\ngamma\tgamma\tgo\tsource\tnative\tstdout\t5s\t\ttarget\t--sarif\n")
	mustWriteFile(t, filepath.Join(root, "k-playbook", "scripts", "install-security-tools.sh"), multiPreflightScript(root, "alpha", "beta", "gamma"))
	if err := os.Chmod(filepath.Join(root, "k-playbook", "scripts", "install-security-tools.sh"), 0o755); err != nil {
		t.Fatalf("Preflight chmod: %v", err)
	}
	mustWriteScanner(t, root, "alpha", 1200*time.Millisecond)
	mustWriteScanner(t, root, "beta", 2400*time.Millisecond)
	mustWriteScanner(t, root, "gamma", 3600*time.Millisecond)
	mustCreateRun(t, root, []review.Entry{{Name: "alpha", Kind: review.KindTool}, {Name: "beta", Kind: review.KindTool}, {Name: "gamma", Kind: review.KindTool}})

	ctx := context.Background()
	server := mcp.NewServer(&mcp.Implementation{Name: "server", Version: "v0.0.1"}, nil)
	addReviewTools(server)
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("Server verbinden: %v", err)
	}
	defer serverSession.Close()

	var mutex sync.Mutex
	progressEvents := []*mcp.ProgressNotificationParams{}
	client := mcp.NewClient(&mcp.Implementation{Name: "client", Version: "v0.0.1"}, &mcp.ClientOptions{
		ProgressNotificationHandler: func(_ context.Context, req *mcp.ProgressNotificationClientRequest) {
			mutex.Lock()
			defer mutex.Unlock()
			progressEvents = append(progressEvents, req.Params)
		},
	})
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("Client verbinden: %v", err)
	}
	defer clientSession.Close()

	params := &mcp.CallToolParams{
		Name: reviewToolScan,
		Arguments: reviewScanInput{
			reviewBaseInput: reviewBaseInput{ProjectDir: root},
			Run:             "2026-08-19",
		},
	}
	params.SetProgressToken("scan-token")
	result, err := clientSession.CallTool(ctx, params)
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if envelope := decodeReviewEnvelope(t, result); !envelope.OK {
		t.Fatalf("scan fehlgeschlagen: %#v", envelope.Error)
	}

	mutex.Lock()
	defer mutex.Unlock()
	if len(progressEvents) < 4 {
		t.Fatalf("Progress-Events = %d, erwartet mindestens 4: %+v", len(progressEvents), progressEvents)
	}
	seen := map[string]bool{}
	for _, event := range progressEvents {
		if event.ProgressToken != "scan-token" {
			t.Fatalf("ProgressToken = %v", event.ProgressToken)
		}
		if event.Total != 3 {
			t.Fatalf("Total = %.0f, erwartet 3", event.Total)
		}
		for _, name := range []string{"alpha", "beta", "gamma"} {
			if strings.Contains(event.Message, name) {
				seen[name] = true
			}
		}
	}
	for _, name := range []string{"alpha", "beta", "gamma"} {
		if !seen[name] {
			t.Fatalf("kein Progress-Event für %s in %+v", name, progressEvents)
		}
	}
}

func TestReviewScanOhneProgressTokenSendetKeineNotifications(t *testing.T) {
	root := newReviewProject(t)
	mustCreateRun(t, root, []review.Entry{{Name: "mockscan", Kind: review.KindTool}})
	ctx := context.Background()
	server := mcp.NewServer(&mcp.Implementation{Name: "server", Version: "v0.0.1"}, nil)
	addReviewTools(server)
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("Server verbinden: %v", err)
	}
	defer serverSession.Close()

	var count int
	client := mcp.NewClient(&mcp.Implementation{Name: "client", Version: "v0.0.1"}, &mcp.ClientOptions{
		ProgressNotificationHandler: func(context.Context, *mcp.ProgressNotificationClientRequest) {
			count++
		},
	})
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("Client verbinden: %v", err)
	}
	defer clientSession.Close()

	result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: reviewToolScan,
		Arguments: reviewScanInput{
			reviewBaseInput: reviewBaseInput{ProjectDir: root},
			Run:             "2026-08-19",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if envelope := decodeReviewEnvelope(t, result); !envelope.OK {
		t.Fatalf("scan fehlgeschlagen: %#v", envelope.Error)
	}
	if count != 0 {
		t.Fatalf("Progress-Notifications ohne Token = %d, erwartet 0", count)
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

func TestReviewWriteAIEntryScanTriageDoneUndStatus(t *testing.T) {
	root := newReviewProject(t)
	runDir := mustCreateRun(t, root, []review.Entry{{Name: scanTriageEntry, Kind: review.KindAI, RecipeKey: scanTriageEntry, RecipePath: filepath.Join(root, project.PlaybookDirName, "commands", filepath.FromSlash(scanTriageModule)), RecipeOrigin: "dist", Title: "Review-Triage", ResultRequired: boolPtr(true), DefaultResult: scanTriageResult}})
	mustWriteFile(t, filepath.Join(runDir, scanTriageResult), minimalTriage())
	result, _, err := reviewWriteAIEntryTool(context.Background(), nil, reviewWriteAIEntryInput{
		reviewBaseInput: reviewBaseInput{ProjectDir: root},
		Run:             "2026-08-19",
		Entry:           scanTriageEntry,
		State:           review.StateDone,
		Result:          scanTriageResult,
	})
	if err != nil {
		t.Fatalf("write_ai_entry: %v", err)
	}
	if envelope := decodeReviewEnvelope(t, result); !envelope.OK {
		t.Fatalf("write_ai_entry fehlgeschlagen: %#v", envelope.Error)
	}

	result, _, err = reviewStatusTool(context.Background(), nil, reviewStatusInput{reviewBaseInput: reviewBaseInput{ProjectDir: root}, Run: "2026-08-19"})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	envelope := decodeReviewEnvelope(t, result)
	if !envelope.OK {
		t.Fatalf("status fehlgeschlagen: %#v", envelope.Error)
	}
	entry := statusEntry(t, envelope.Data.(map[string]any)["entries"].([]any), scanTriageEntry)
	if entry["state"] != string(review.StateDone) || entry["result"] != scanTriageResult {
		t.Fatalf("scan-triage Status = %#v", entry)
	}
}

func TestReviewWriteAIEntryScanTriageDoneOhneDateiAbgelehnt(t *testing.T) {
	root := newReviewProject(t)
	mustCreateRun(t, root, []review.Entry{{Name: scanTriageEntry, Kind: review.KindAI, RecipeKey: scanTriageEntry, ResultRequired: boolPtr(true), DefaultResult: scanTriageResult}})
	result, _, err := reviewWriteAIEntryTool(context.Background(), nil, reviewWriteAIEntryInput{
		reviewBaseInput: reviewBaseInput{ProjectDir: root},
		Run:             "2026-08-19",
		Entry:           scanTriageEntry,
		State:           review.StateDone,
		Result:          scanTriageResult,
	})
	if err != nil {
		t.Fatalf("write_ai_entry: %v", err)
	}
	assertToolErrorCode(t, result, "result_path_invalid")
}

func TestReviewWriteAIEntryRepariertFehlendenScanTriageEintrag(t *testing.T) {
	root := newReviewProject(t)
	runDir := mustCreateRun(t, root, []review.Entry{{Name: "mockscan", Kind: review.KindTool}})
	mustWriteFile(t, filepath.Join(runDir, scanTriageResult), minimalTriage())
	result, _, err := reviewWriteAIEntryTool(context.Background(), nil, reviewWriteAIEntryInput{
		reviewBaseInput: reviewBaseInput{ProjectDir: root},
		Run:             "2026-08-19",
		Entry:           scanTriageEntry,
		State:           review.StateDone,
		Result:          scanTriageResult,
	})
	if err != nil {
		t.Fatalf("write_ai_entry: %v", err)
	}
	if envelope := decodeReviewEnvelope(t, result); !envelope.OK {
		t.Fatalf("write_ai_entry fehlgeschlagen: %#v", envelope.Error)
	}
	var status aiEntryStatus
	if err := readAIEntryStatus(runDir, scanTriageEntry, &status); err != nil {
		t.Fatalf("scan-triage Status lesen: %v", err)
	}
	if status.State != review.StateDone || status.Result != scanTriageResult {
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
		{name: "leeres Result", input: reviewWriteAIEntryInput{Run: "2026-08-19", Entry: "tech", State: review.StateDone, Result: "empty.md"}, code: "result_path_invalid"},
		{name: "failed ohne Grund", input: reviewWriteAIEntryInput{Run: "2026-08-19", Entry: "tech", State: review.StateFailed}, code: "entry_state_invalid"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.input.Result == "empty.md" {
				mustWriteFile(t, filepath.Join(project.LocalDir(root), review.ResultsDirName, "2026-08-19", "empty.md"), "")
			}
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
		filepath.Join(root, "k-playbook", "commands", "_audit"),
		filepath.Join(root, "k-playbook", "reviews"),
		filepath.Join(root, "k-playbook", "rules"),
		filepath.Join(root, "k-playbook", "checks"),
		filepath.Join(root, "k-playbook-local", "commands", "_audit"),
		filepath.Join(root, "k-playbook-local", "reviews"),
		filepath.Join(root, "k-playbook-local", review.ResultsDirName),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("%s anlegen: %v", dir, err)
		}
	}
	mustWriteFile(t, filepath.Join(root, "K-PLAYBOOK.yaml"), "schema_version: 3\n\nproject:\n  repo_root: .\n  vcs: none\n  languages:\n    - go\n")
	mustWriteFile(t, filepath.Join(root, "k-playbook", "commands", filepath.FromSlash(scanTriageModule)), "# Review-Triage\n")
	mustWriteFile(t, filepath.Join(root, "k-playbook", "reviews", "review-tech.md"), "---\ntitle: Technischer Review\naudit:\n  enabled: true\n  resultRequired: true\n  defaultResult: review-tech.md\n  scope:\n    tools: [mockscan]\nreview:\n  enabled: true\n---\n# Fallback\n")
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

func multiPreflightScript(root string, names ...string) string {
	tools := []string{}
	for _, name := range names {
		tools = append(tools, `    {"name":"`+name+`","languages":"go","status":"ok","path":"`+filepath.ToSlash(filepath.Join(root, name))+`","role":"Mock-Scanner"}`)
	}
	return "#!/usr/bin/env bash\ncat <<JSON\n{\n  \"playbookDir\": \"" + filepath.ToSlash(filepath.Join(root, "k-playbook")) + "\",\n  \"languages\": \"go\",\n  \"tools\": [\n" + strings.Join(tools, ",\n") + "\n  ]\n}\nJSON\n"
}

func mustWriteScanner(t *testing.T, root string, name string, delay time.Duration) {
	t.Helper()
	body := ""
	if delay > 0 {
		body += "sleep " + fmtDurationSeconds(delay) + "\n"
	}
	body += "printf '%s' '" + minimalSARIF() + "'\n"
	mustWriteFile(t, filepath.Join(root, name), "#!/usr/bin/env bash\n"+body)
	if err := os.Chmod(filepath.Join(root, name), 0o755); err != nil {
		t.Fatalf("%s chmod: %v", name, err)
	}
}

func fmtDurationSeconds(duration time.Duration) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.3f", duration.Seconds()), "0"), ".")
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
	return review.Entry{Name: name, Kind: review.KindAI, RecipeKey: name, RecipePath: "/review-" + name + ".md", RecipeOrigin: "dist", Title: "Technischer Review", ResultRequired: &required, DefaultResult: "review-" + name + ".md", Scope: &review.Scope{Tools: []string{"mockscan"}}}
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

func hasCandidate(candidates []any, name string) bool {
	for _, raw := range candidates {
		candidate := raw.(map[string]any)
		if candidate["name"] == name {
			return true
		}
	}
	return false
}

func candidateByName(t *testing.T, candidates []any, name string) map[string]any {
	t.Helper()
	for _, raw := range candidates {
		candidate := raw.(map[string]any)
		if candidate["name"] == name {
			return candidate
		}
	}
	t.Fatalf("kein Candidate %s in %v", name, candidates)
	return nil
}

func reviewCatalogEntry(t *testing.T, entries []project.CatalogEntry, name string) project.CatalogEntry {
	t.Helper()
	for _, entry := range entries {
		if entry.Name == name {
			return entry
		}
	}
	t.Fatalf("kein Review-Katalogeintrag %s in %+v", name, entries)
	return project.CatalogEntry{}
}

func hasStatusEntry(entries []any, name string) bool {
	for _, raw := range entries {
		entry := raw.(map[string]any)
		if entry["name"] == name {
			return true
		}
	}
	return false
}

func statusEntry(t *testing.T, entries []any, name string) map[string]any {
	t.Helper()
	for _, raw := range entries {
		entry := raw.(map[string]any)
		if entry["name"] == name {
			return entry
		}
	}
	t.Fatalf("kein Entry %s in %v", name, entries)
	return nil
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

func minimalTriage() string {
	return "# Review-Triage\n\n## Bündel\n\n## Bündel-Details\n\n## Nicht gebündelt\n\n## Deckung aus known-decisions\n"
}

func intPtr(value int) *int { return &value }

func boolPtr(value bool) *bool { return &value }

// evidenceRecipe ist ein Rezept, das den Evidence-Vertrag erfüllt: Betriebsart,
// Pfad-Scope und Rule-ID-Liste, kein defaultResult und kein resultRequired.
func evidenceRecipe() string {
	return "---\ntitle: Technischer Review\naudit:\n  enabled: true\n  mode: evidence\n  ruleIds:\n    - tech-veraltet\n    - tech-kopplung\n  scope:\n    paths:\n      - installer/**\n      - commands/**\nreview:\n  enabled: true\n---\n# Technischer Review\n"
}

func TestReadAIRecipeMetadataLiestEvidenceVertrag(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "review-tech.md")
	mustWriteFile(t, path, evidenceRecipe())

	metadata, err := readAIRecipeMetadata("review-tech", path)
	if err != nil {
		t.Fatalf("readAIRecipeMetadata: %v", err)
	}
	if metadata.Mode != review.ModeEvidence {
		t.Errorf("Mode = %q, erwartet %q", metadata.Mode, review.ModeEvidence)
	}
	if metadata.Scope == nil || len(metadata.Scope.Paths) != 2 || metadata.Scope.Paths[0] != "installer/**" {
		t.Fatalf("Pfad-Scope = %#v", metadata.Scope)
	}
	if len(metadata.Scope.Tools) != 0 {
		t.Errorf("Tools = %#v, erwartet leer", metadata.Scope.Tools)
	}
	if len(metadata.RuleIDs) != 2 || metadata.RuleIDs[1] != "tech-kopplung" {
		t.Fatalf("RuleIDs = %#v", metadata.RuleIDs)
	}
	// Der Ergebnisvertrag: Pflichtartefakt ist raw/<entry>.sarif, deshalb darf
	// resultRequired nicht als true in den Lauf geraten.
	if metadata.ResultRequired {
		t.Error("ResultRequired = true, erwartet false")
	}
	if metadata.DefaultResult != "" {
		t.Errorf("DefaultResult = %q, erwartet leer", metadata.DefaultResult)
	}
	if err := review.ValidateAuditContract(metadata.auditContract()); err != nil {
		t.Fatalf("Vertrag ungültig: %v", err)
	}
}

func TestReadAIRecipeMetadataHaeltPerspektiveUnveraendert(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "review-tech.md")
	mustWriteFile(t, path, "---\ntitle: Technischer Review\naudit:\n  enabled: true\n  resultRequired: true\n  defaultResult: review-tech.md\n  scope:\n    tools: [mockscan]\nreview:\n  enabled: true\n---\n# Fallback\n")

	metadata, err := readAIRecipeMetadata("review-tech", path)
	if err != nil {
		t.Fatalf("readAIRecipeMetadata: %v", err)
	}
	if metadata.Mode != review.ModePerspective {
		t.Errorf("Mode = %q, erwartet %q", metadata.Mode, review.ModePerspective)
	}
	if !metadata.ResultRequired || !metadata.ResultRequiredSet {
		t.Errorf("ResultRequired = %v/%v, erwartet true/true", metadata.ResultRequired, metadata.ResultRequiredSet)
	}
	if metadata.DefaultResult != "review-tech.md" {
		t.Errorf("DefaultResult = %q", metadata.DefaultResult)
	}
	if metadata.Scope == nil || len(metadata.Scope.Tools) != 1 || len(metadata.Scope.Paths) != 0 {
		t.Fatalf("Scope = %#v", metadata.Scope)
	}
	if err := review.ValidateAuditContract(metadata.auditContract()); err != nil {
		t.Fatalf("Vertrag ungültig: %v", err)
	}
}

func TestReadAIRecipeMetadataMeldetUnzulaessigeEvidenceKombination(t *testing.T) {
	cases := map[string]string{
		"resultRequired neben evidence": "---\naudit:\n  enabled: true\n  mode: evidence\n  resultRequired: true\n  ruleIds: [tech-x]\n  scope:\n    paths: [installer/**]\n---\n# X\n",
		"defaultResult neben evidence":  "---\naudit:\n  enabled: true\n  mode: evidence\n  defaultResult: review-tech.md\n  ruleIds: [tech-x]\n  scope:\n    paths: [installer/**]\n---\n# X\n",
		"tools neben evidence":          "---\naudit:\n  enabled: true\n  mode: evidence\n  ruleIds: [tech-x]\n  scope:\n    tools: [mockscan]\n    paths: [installer/**]\n---\n# X\n",
		"evidence ohne paths":           "---\naudit:\n  enabled: true\n  mode: evidence\n  ruleIds: [tech-x]\n---\n# X\n",
		"evidence ohne ruleIds":         "---\naudit:\n  enabled: true\n  mode: evidence\n  scope:\n    paths: [installer/**]\n---\n# X\n",
		"paths neben perspective":       "---\naudit:\n  enabled: true\n  scope:\n    paths: [installer/**]\n---\n# X\n",
		"unbekannte Betriebsart":        "---\naudit:\n  enabled: true\n  mode: scanner\n---\n# X\n",
	}
	for name, recipe := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "review-x.md")
			mustWriteFile(t, path, recipe)
			metadata, err := readAIRecipeMetadata("review-x", path)
			if err != nil {
				t.Fatalf("readAIRecipeMetadata: %v", err)
			}
			if err := review.ValidateAuditContract(metadata.auditContract()); err == nil {
				t.Fatalf("Vertrag gilt als gültig: %#v", metadata)
			}
		})
	}
}

func TestReviewStatusAvailableTrenntEvidenceVonPerspektiven(t *testing.T) {
	root := newReviewProject(t)
	mustWriteFile(t, filepath.Join(root, "k-playbook", "reviews", "review-hardspots.md"), evidenceRecipe())

	result, _, err := reviewStatusTool(context.Background(), nil, reviewStatusInput{reviewBaseInput: reviewBaseInput{ProjectDir: root}, Mode: "available"})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	envelope := decodeReviewEnvelope(t, result)
	if !envelope.OK {
		t.Fatalf("status fehlgeschlagen: %#v", envelope.Error)
	}
	selection := envelope.Data.(map[string]any)["selection"].(map[string]any)

	candidate := candidateByName(t, selection["candidates"].([]any), "hardspots")
	if candidate["mode"] != string(review.ModeEvidence) {
		t.Fatalf("mode = %v", candidate["mode"])
	}
	if candidate["selectable"] != true {
		t.Fatalf("selectable = %v", candidate["selectable"])
	}
	scope := candidate["scope"].(map[string]any)
	if paths := scope["paths"].([]any); len(paths) != 2 || paths[0] != "installer/**" {
		t.Fatalf("paths = %#v", scope)
	}
	if ruleIDs := candidate["ruleIds"].([]any); len(ruleIDs) != 2 {
		t.Fatalf("ruleIds = %#v", candidate["ruleIds"])
	}
	// Ein Evidence-Rezept schreibt kein Ergebnisdokument: resultRequired darf
	// nicht als true in die Auswahlbasis geraten.
	if candidate["resultRequired"] != false {
		t.Fatalf("resultRequired = %v", candidate["resultRequired"])
	}
	if _, found := candidate["defaultResult"]; found {
		t.Fatalf("defaultResult = %v", candidate["defaultResult"])
	}

	evidence := selection["evidenceCandidates"].([]any)
	if len(evidence) != 1 || evidence[0] != "hardspots" {
		t.Fatalf("evidenceCandidates = %#v", evidence)
	}
	perspectives := selection["perspectiveCandidates"].([]any)
	if len(perspectives) != 2 || !containsValue(perspectives, "tech") || !containsValue(perspectives, scanTriageEntry) {
		t.Fatalf("perspectiveCandidates = %#v", perspectives)
	}

	tech := candidateByName(t, selection["candidates"].([]any), "tech")
	if tech["mode"] != string(review.ModePerspective) {
		t.Fatalf("tech mode = %v", tech["mode"])
	}
}

func TestReviewStatusAvailableHaeltUngueltigenAuditVertragAusDerAuswahl(t *testing.T) {
	root := newReviewProject(t)
	mustWriteFile(t, filepath.Join(root, "k-playbook", "reviews", "review-broken.md"),
		"---\ntitle: Kaputt\naudit:\n  enabled: true\n  mode: evidence\n  resultRequired: true\n  ruleIds: [x]\n  scope:\n    paths: [installer/**]\n---\n# Kaputt\n")

	result, _, err := reviewStatusTool(context.Background(), nil, reviewStatusInput{reviewBaseInput: reviewBaseInput{ProjectDir: root}, Mode: "available"})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	envelope := decodeReviewEnvelope(t, result)
	if !envelope.OK {
		t.Fatalf("status fehlgeschlagen: %#v", envelope.Error)
	}
	selection := envelope.Data.(map[string]any)["selection"].(map[string]any)
	candidate := candidateByName(t, selection["candidates"].([]any), "broken")
	if candidate["selectable"] != false {
		t.Fatalf("selectable = %v", candidate["selectable"])
	}
	if reason, _ := candidate["unavailableReason"].(string); !strings.Contains(reason, "Audit-Vertrag") {
		t.Fatalf("unavailableReason = %q", reason)
	}
	if !hasCandidate(selection["unavailableCandidates"].([]any), "broken") {
		t.Fatalf("broken fehlt in unavailableCandidates")
	}
	if containsValue(selection["evidenceCandidates"].([]any), "broken") {
		t.Fatalf("broken steht in evidenceCandidates")
	}

	// Ausdrücklich ausgewählt wird das Rezept abgewiesen statt still übergangen.
	created, _, err := reviewCreateTool(context.Background(), nil, reviewCreateInput{
		reviewBaseInput: reviewBaseInput{ProjectDir: root},
		Day:             "2026-08-19",
		Entries:         []reviewSelectionInput{{Name: "broken", Kind: review.KindAI}},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	assertToolErrorCode(t, created, "selection_unavailable")
}

func TestReviewCreateFriertModusUndPfadScopeEin(t *testing.T) {
	root := newReviewProject(t)
	mustWriteFile(t, filepath.Join(root, "k-playbook", "reviews", "review-hardspots.md"), evidenceRecipe())

	result, _, err := reviewCreateTool(context.Background(), nil, reviewCreateInput{reviewBaseInput: reviewBaseInput{ProjectDir: root}, Day: "2026-08-19"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if envelope := decodeReviewEnvelope(t, result); !envelope.OK {
		t.Fatalf("create fehlgeschlagen: %#v", envelope.Error)
	}

	written, err := review.ReadRun(filepath.Join(project.LocalDir(root), review.ResultsDirName, "2026-08-19"))
	if err != nil {
		t.Fatalf("run.json lesen: %v", err)
	}
	evidence, ok := runEntry(written, "hardspots")
	if !ok {
		t.Fatalf("hardspots fehlt: %#v", written.Entries)
	}
	if evidence.Mode != review.ModeEvidence {
		t.Errorf("Mode = %q", evidence.Mode)
	}
	if evidence.Scope == nil || len(evidence.Scope.Paths) != 2 || evidence.Scope.Paths[0] != "installer/**" {
		t.Fatalf("Pfad-Scope = %#v", evidence.Scope)
	}
	if len(evidence.Scope.Tools) != 0 {
		t.Errorf("Tools = %#v, erwartet leer", evidence.Scope.Tools)
	}
	if evidence.ResultRequired == nil || *evidence.ResultRequired {
		t.Errorf("ResultRequired = %#v, erwartet false", evidence.ResultRequired)
	}
	if evidence.DefaultResult != "" {
		t.Errorf("DefaultResult = %q, erwartet leer", evidence.DefaultResult)
	}

	perspective, ok := runEntry(written, "tech")
	if !ok {
		t.Fatalf("tech fehlt: %#v", written.Entries)
	}
	if perspective.Mode != review.ModePerspective {
		t.Errorf("tech Mode = %q", perspective.Mode)
	}
}

func TestReviewStatusExistingTrenntEvidenceVonPerspektiven(t *testing.T) {
	root := newReviewProject(t)
	evidence := aiEntry("hardspots")
	evidence.Mode = review.ModeEvidence
	evidence.ResultRequired = boolPtr(false)
	evidence.DefaultResult = ""
	evidence.Scope = &review.Scope{Paths: []string{"installer/**"}}
	mustCreateRun(t, root, []review.Entry{{Name: "mockscan", Kind: review.KindTool}, aiEntry("tech"), evidence})

	result, _, err := reviewStatusTool(context.Background(), nil, reviewStatusInput{reviewBaseInput: reviewBaseInput{ProjectDir: root}, Run: "2026-08-19"})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	envelope := decodeReviewEnvelope(t, result)
	if !envelope.OK {
		t.Fatalf("status fehlgeschlagen: %#v", envelope.Error)
	}
	data := envelope.Data.(map[string]any)

	evidenceNames := data["evidenceEntries"].([]any)
	if len(evidenceNames) != 1 || evidenceNames[0] != "hardspots" {
		t.Fatalf("evidenceEntries = %#v", evidenceNames)
	}
	perspectiveNames := data["perspectiveEntries"].([]any)
	if len(perspectiveNames) != 2 || !containsValue(perspectiveNames, "tech") || !containsValue(perspectiveNames, scanTriageEntry) {
		t.Fatalf("perspectiveEntries = %#v", perspectiveNames)
	}

	entries := data["entries"].([]any)
	if mode := statusEntry(t, entries, "hardspots")["mode"]; mode != string(review.ModeEvidence) {
		t.Fatalf("hardspots mode = %v", mode)
	}
	if mode := statusEntry(t, entries, "tech")["mode"]; mode != string(review.ModePerspective) {
		t.Fatalf("tech mode = %v", mode)
	}
	if _, found := statusEntry(t, entries, "mockscan")["mode"]; found {
		t.Fatal("Tool-Eintrag trägt eine Betriebsart")
	}
}

// Ein Lauf aus der Zeit vor der Evidence-Betriebsart hat kein mode-Feld. Der
// Status muss ihn unverändert als Perspektiven-Lauf zeigen.
func TestReviewStatusExistingZeigtAltlaufAlsPerspektive(t *testing.T) {
	root := newReviewProject(t)
	entry := aiEntry("tech")
	entry.Mode = ""
	mustCreateRun(t, root, []review.Entry{{Name: "mockscan", Kind: review.KindTool}, entry})

	result, _, err := reviewStatusTool(context.Background(), nil, reviewStatusInput{reviewBaseInput: reviewBaseInput{ProjectDir: root}, Run: "2026-08-19"})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	envelope := decodeReviewEnvelope(t, result)
	if !envelope.OK {
		t.Fatalf("status fehlgeschlagen: %#v", envelope.Error)
	}
	data := envelope.Data.(map[string]any)
	if evidenceNames := data["evidenceEntries"].([]any); len(evidenceNames) != 0 {
		t.Fatalf("evidenceEntries = %#v", evidenceNames)
	}
	entries := data["entries"].([]any)
	if mode := statusEntry(t, entries, "tech")["mode"]; mode != string(review.ModePerspective) {
		t.Fatalf("tech mode = %v", mode)
	}
	if statusEntry(t, entries, "tech")["resultRequired"] != true {
		t.Fatal("resultRequired eines Altlaufs verändert")
	}
}

func runEntry(run review.Run, name string) (review.Entry, bool) {
	for _, entry := range run.Entries {
		if entry.Name == name {
			return entry, true
		}
	}
	return review.Entry{}, false
}

func containsValue(values []any, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

// evidenceRun legt einen Lauf mit einem Evidence-Eintrag an: Rezept im
// Katalog, Betriebsart und Pfad-Scope im Eintrag eingefroren.
func evidenceRun(t *testing.T, root string, name string) (string, review.Entry) {
	t.Helper()
	recipePath := filepath.Join(root, "k-playbook", "reviews", "review-"+name+".md")
	mustWriteFile(t, recipePath, evidenceRecipe())
	entry := review.Entry{
		Name:           name,
		Kind:           review.KindAI,
		RecipeKey:      name,
		RecipePath:     recipePath,
		RecipeOrigin:   "dist",
		Title:          "Technischer Review",
		Mode:           review.ModeEvidence,
		ResultRequired: boolPtr(false),
		Scope:          &review.Scope{Paths: []string{"installer/**", "commands/**"}},
	}
	runDir := mustCreateRun(t, root, []review.Entry{{Name: "mockscan", Kind: review.KindTool}, entry})
	return runDir, entry
}

func evidenceSARIF(tool string, results ...string) string {
	return `{"version":"2.1.0","runs":[{"tool":{"driver":{"name":"` + tool + `","rules":[{"id":"tech-veraltet"},{"id":"tech-kopplung"}]}},"results":[` + strings.Join(results, ",") + `]}]}`
}

func evidenceResult(ruleID string, uri string) string {
	return `{"ruleId":"` + ruleID + `","level":"error","message":{"text":"Fund"},"locations":[{"physicalLocation":{"artifactLocation":{"uri":"` + uri + `"},"region":{"startLine":7}}}]}`
}

func writeEvidenceEntry(t *testing.T, root string, entry string, sarif string) reviewEnvelope {
	t.Helper()
	result, _, err := reviewWriteAIEntryTool(context.Background(), nil, reviewWriteAIEntryInput{
		reviewBaseInput: reviewBaseInput{ProjectDir: root},
		Run:             "2026-08-19",
		Entry:           entry,
		State:           review.StateDone,
		Job:             &reviewAIJobInput{SARIF: sarif, Started: "2026-08-19T10:00:00Z", Finished: "2026-08-19T10:05:00Z"},
	})
	if err != nil {
		t.Fatalf("write_ai_entry: %v", err)
	}
	return decodeReviewEnvelope(t, result)
}

// Ein Fund außerhalb von scope.paths kostet den Fund, nicht den Eintrag: das
// SARIF wird bereinigt zurückgeschrieben, die Zahl steht im Grund.
func TestReviewWriteAIEntryEvidenceTeilannahmeBeiScopeVerstoss(t *testing.T) {
	root := newReviewProject(t)
	runDir, _ := evidenceRun(t, root, "hardspots")
	sarifPath := filepath.Join(runDir, review.RawDirName, "hardspots.sarif")
	mustWriteFile(t, sarifPath, evidenceSARIF("hardspots",
		evidenceResult("tech-veraltet", "installer/internal/review/run.go"),
		evidenceResult("tech-kopplung", "docs/handbuch.md"),
		evidenceResult("tech-kopplung", "k-playbook/reviews/review-tech.md"),
	))

	envelope := writeEvidenceEntry(t, root, "hardspots", "raw/hardspots.sarif")
	if !envelope.OK {
		t.Fatalf("write_ai_entry fehlgeschlagen: %#v", envelope.Error)
	}
	data := envelope.Data.(map[string]any)
	evidence := data["evidence"].(map[string]any)
	if evidence["findings"] != float64(1) || evidence["droppedFindings"] != float64(2) {
		t.Fatalf("evidence = %#v", evidence)
	}
	if evidence["sarifRewritten"] != true {
		t.Error("bereinigtes SARIF wurde nicht zurückgeschrieben")
	}
	if _, overridden := data["stateOverridden"]; overridden {
		t.Error("Teilannahme darf den Zustand nicht ändern")
	}

	var status aiEntryStatus
	if err := readAIEntryStatus(runDir, "hardspots", &status); err != nil {
		t.Fatalf("Status lesen: %v", err)
	}
	if status.State != review.StateDone {
		t.Fatalf("state = %q", status.State)
	}
	if len(status.Jobs) != 1 || status.Jobs[0].SARIF != "raw/hardspots.sarif" || status.Jobs[0].State != review.StateDone {
		t.Fatalf("jobs = %#v", status.Jobs)
	}
	if status.Jobs[0].Findings == nil || *status.Jobs[0].Findings != 1 {
		t.Fatalf("findings = %#v", status.Jobs[0].Findings)
	}
	if status.Jobs[0].Started != "2026-08-19T10:00:00Z" || status.Jobs[0].Finished != "2026-08-19T10:05:00Z" {
		t.Fatalf("Job-Zeiten = %#v", status.Jobs[0])
	}
	if !strings.Contains(status.Reason, "2 Fund") || !strings.Contains(status.Reason, "docs/handbuch.md") {
		t.Fatalf("reason = %q", status.Reason)
	}

	raw, err := os.ReadFile(sarifPath)
	if err != nil {
		t.Fatalf("SARIF lesen: %v", err)
	}
	if strings.Contains(string(raw), "docs/handbuch.md") || strings.Contains(string(raw), "k-playbook/reviews") {
		t.Fatalf("SARIF wurde nicht bereinigt: %s", raw)
	}
	if !strings.Contains(string(raw), "installer/internal/review/run.go") {
		t.Fatalf("Fund im Scope fehlt: %s", raw)
	}
}

// Eine Rule-ID außerhalb der Liste macht den Eintrag failed — kein stilles done.
func TestReviewWriteAIEntryEvidenceRuleIDAusserhalbDerListe(t *testing.T) {
	root := newReviewProject(t)
	runDir, _ := evidenceRun(t, root, "hardspots")
	mustWriteFile(t, filepath.Join(runDir, review.RawDirName, "hardspots.sarif"), evidenceSARIF("hardspots", evidenceResult("tech-erfunden", "installer/main.go")))

	envelope := writeEvidenceEntry(t, root, "hardspots", "raw/hardspots.sarif")
	if !envelope.OK {
		t.Fatalf("write_ai_entry fehlgeschlagen: %#v", envelope.Error)
	}
	data := envelope.Data.(map[string]any)
	if data["stateOverridden"] != true || data["requestedState"] != string(review.StateDone) {
		t.Fatalf("data = %#v", data)
	}

	var status aiEntryStatus
	if err := readAIEntryStatus(runDir, "hardspots", &status); err != nil {
		t.Fatalf("Status lesen: %v", err)
	}
	if status.State != review.StateFailed {
		t.Fatalf("state = %q, erwartet failed", status.State)
	}
	if !strings.Contains(status.Reason, "tech-erfunden") {
		t.Fatalf("reason = %q", status.Reason)
	}
	if len(status.Jobs) != 1 || status.Jobs[0].State != review.StateFailed || status.Jobs[0].Reason == "" {
		t.Fatalf("jobs = %#v", status.Jobs)
	}
}

// Ein fremder Werkzeugname im SARIF wird genauso behandelt.
func TestReviewWriteAIEntryEvidenceFremderWerkzeugname(t *testing.T) {
	root := newReviewProject(t)
	runDir, _ := evidenceRun(t, root, "hardspots")
	mustWriteFile(t, filepath.Join(runDir, review.RawDirName, "hardspots.sarif"), evidenceSARIF("semgrep", evidenceResult("tech-veraltet", "installer/main.go")))

	envelope := writeEvidenceEntry(t, root, "hardspots", "raw/hardspots.sarif")
	if !envelope.OK {
		t.Fatalf("write_ai_entry fehlgeschlagen: %#v", envelope.Error)
	}
	var status aiEntryStatus
	if err := readAIEntryStatus(runDir, "hardspots", &status); err != nil {
		t.Fatalf("Status lesen: %v", err)
	}
	if status.State != review.StateFailed || !strings.Contains(status.Reason, "semgrep") {
		t.Fatalf("status = %#v", status)
	}
}

// Ein leerer Scope-Befund ist ein Ergebnis: done ohne Funde, ohne Inkonsistenz.
func TestReviewWriteAIEntryEvidenceLeeresSARIFIstDone(t *testing.T) {
	root := newReviewProject(t)
	runDir, _ := evidenceRun(t, root, "hardspots")
	mustWriteFile(t, filepath.Join(runDir, review.RawDirName, "hardspots.sarif"), evidenceSARIF("hardspots"))

	envelope := writeEvidenceEntry(t, root, "hardspots", "raw/hardspots.sarif")
	if !envelope.OK {
		t.Fatalf("write_ai_entry fehlgeschlagen: %#v", envelope.Error)
	}
	evidence := envelope.Data.(map[string]any)["evidence"].(map[string]any)
	if evidence["findings"] != float64(0) || evidence["droppedFindings"] != float64(0) {
		t.Fatalf("evidence = %#v", evidence)
	}
	var status aiEntryStatus
	if err := readAIEntryStatus(runDir, "hardspots", &status); err != nil {
		t.Fatalf("Status lesen: %v", err)
	}
	if status.State != review.StateDone || status.Reason != "" {
		t.Fatalf("status = %#v", status)
	}

	result, _, err := reviewStatusTool(context.Background(), nil, reviewStatusInput{reviewBaseInput: reviewBaseInput{ProjectDir: root}, Run: "2026-08-19"})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	entry := statusEntry(t, decodeReviewEnvelope(t, result).Data.(map[string]any)["entries"].([]any), "hardspots")
	if _, found := entry["inconsistent"]; found {
		t.Fatalf("leeres SARIF gilt als inkonsistent: %#v", entry)
	}
	if entry["resultRequired"] != false {
		t.Errorf("resultRequired = %#v, erwartet false", entry["resultRequired"])
	}
}

func TestReviewWriteAIEntryEvidenceFormfehler(t *testing.T) {
	root := newReviewProject(t)
	runDir, _ := evidenceRun(t, root, "hardspots")
	mustWriteFile(t, filepath.Join(runDir, review.RawDirName, "hardspots.sarif"), evidenceSARIF("hardspots"))
	mustWriteFile(t, filepath.Join(runDir, "daneben.sarif"), evidenceSARIF("hardspots"))

	tests := []struct {
		name  string
		input reviewWriteAIEntryInput
		code  string
	}{
		{
			name:  "done ohne Job",
			input: reviewWriteAIEntryInput{Run: "2026-08-19", Entry: "hardspots", State: review.StateDone},
			code:  "sarif_required",
		},
		{
			name:  "Ergebnisdatei statt SARIF",
			input: reviewWriteAIEntryInput{Run: "2026-08-19", Entry: "hardspots", State: review.StateDone, Result: "review-hardspots.md"},
			code:  "entry_result_invalid",
		},
		{
			name:  "SARIF neben raw",
			input: reviewWriteAIEntryInput{Run: "2026-08-19", Entry: "hardspots", State: review.StateDone, Job: &reviewAIJobInput{SARIF: "daneben.sarif"}},
			code:  "sarif_path_invalid",
		},
		{
			name:  "SARIF außerhalb des Laufs",
			input: reviewWriteAIEntryInput{Run: "2026-08-19", Entry: "hardspots", State: review.StateDone, Job: &reviewAIJobInput{SARIF: "../raw/hardspots.sarif"}},
			code:  "sarif_path_invalid",
		},
		{
			name:  "SARIF fehlt",
			input: reviewWriteAIEntryInput{Run: "2026-08-19", Entry: "hardspots", State: review.StateDone, Job: &reviewAIJobInput{SARIF: "raw/fehlt.sarif"}},
			code:  "sarif_path_invalid",
		},
		{
			name:  "Job bei running",
			input: reviewWriteAIEntryInput{Run: "2026-08-19", Entry: "hardspots", State: review.StateRunning, Job: &reviewAIJobInput{SARIF: "raw/hardspots.sarif"}},
			code:  "entry_job_invalid",
		},
		{
			name:  "Job an einer Perspektive",
			input: reviewWriteAIEntryInput{Run: "2026-08-19", Entry: scanTriageEntry, State: review.StateDone, Job: &reviewAIJobInput{SARIF: "raw/hardspots.sarif"}},
			code:  "entry_job_invalid",
		},
		{
			name:  "ungültige Job-Zeit",
			input: reviewWriteAIEntryInput{Run: "2026-08-19", Entry: "hardspots", State: review.StateDone, Job: &reviewAIJobInput{SARIF: "raw/hardspots.sarif", Started: "gestern"}},
			code:  "entry_state_invalid",
		},
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

// Der Reparaturvertrag eines Evidence-Eintrags hängt am SARIF und nicht an
// einer Ergebnisdatei.
func TestReviewStatusExistingEvidenceReparaturvertrag(t *testing.T) {
	root := newReviewProject(t)
	runDir, _ := evidenceRun(t, root, "hardspots")

	// done ohne SARIF: inkonsistent.
	mustWriteJSON(t, review.EntryFile(runDir, "hardspots"), aiEntryStatus{Name: "hardspots", Kind: review.KindAI, State: review.StateDone})
	entry := evidenceStatusEntry(t, root, "hardspots")
	if entry["sarifMissing"] != true || entry["inconsistent"] != true {
		t.Fatalf("done ohne SARIF = %#v", entry)
	}
	if _, found := entry["resultMissing"]; found {
		t.Error("Evidence-Eintrag wird an der Ergebnisdatei gemessen")
	}

	// SARIF vorhanden, Entry-Status offen: reparierbar.
	mustWriteFile(t, filepath.Join(runDir, review.RawDirName, "hardspots.sarif"), evidenceSARIF("hardspots"))
	if err := os.Remove(review.EntryFile(runDir, "hardspots")); err != nil {
		t.Fatalf("Entry-Datei entfernen: %v", err)
	}
	entry = evidenceStatusEntry(t, root, "hardspots")
	if entry["repairable"] != true || entry["sarifExists"] != true {
		t.Fatalf("offener Eintrag mit SARIF = %#v", entry)
	}

	// Gemeldeter Eintrag mit Job: konsistent.
	writeEvidenceEntry(t, root, "hardspots", "raw/hardspots.sarif")
	entry = evidenceStatusEntry(t, root, "hardspots")
	if _, found := entry["inconsistent"]; found {
		t.Fatalf("gemeldeter Eintrag gilt als inkonsistent: %#v", entry)
	}
	jobs, ok := entry["jobs"].([]any)
	if !ok || len(jobs) != 1 {
		t.Fatalf("jobs = %#v", entry["jobs"])
	}
}

func evidenceStatusEntry(t *testing.T, root string, name string) map[string]any {
	t.Helper()
	result, _, err := reviewStatusTool(context.Background(), nil, reviewStatusInput{reviewBaseInput: reviewBaseInput{ProjectDir: root}, Run: "2026-08-19"})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	envelope := decodeReviewEnvelope(t, result)
	if !envelope.OK {
		t.Fatalf("status fehlgeschlagen: %#v", envelope.Error)
	}
	return statusEntry(t, envelope.Data.(map[string]any)["entries"].([]any), name)
}

// Eine Entry-Datei aus der Zeit vor dem Job-Teil hat kein jobs-Feld. Sie bleibt
// lesbar, und die Perspektive verhält sich unverändert.
func TestReviewWriteAIEntryPerspektiveBleibtUnveraendert(t *testing.T) {
	root := newReviewProject(t)
	runDir := mustCreateRun(t, root, []review.Entry{{Name: "mockscan", Kind: review.KindTool}, aiEntry("tech")})
	mustWriteFile(t, review.EntryFile(runDir, "tech"), `{"schemaVersion":1,"name":"tech","kind":"ai","state":"running","startedAt":"2026-08-19T10:00:00Z"}`+"\n")

	entry := evidenceStatusEntry(t, root, "tech")
	if entry["state"] != string(review.StateRunning) || entry["present"] != true {
		t.Fatalf("Altdatei = %#v", entry)
	}
	if _, found := entry["jobs"]; found {
		t.Error("Altdatei bekommt Jobs untergeschoben")
	}

	mustWriteFile(t, filepath.Join(runDir, "review-tech.md"), "# Ergebnis\n")
	result, _, err := reviewWriteAIEntryTool(context.Background(), nil, reviewWriteAIEntryInput{
		reviewBaseInput: reviewBaseInput{ProjectDir: root},
		Run:             "2026-08-19",
		Entry:           "tech",
		State:           review.StateDone,
		Result:          "review-tech.md",
	})
	if err != nil {
		t.Fatalf("write_ai_entry: %v", err)
	}
	if envelope := decodeReviewEnvelope(t, result); !envelope.OK {
		t.Fatalf("write_ai_entry fehlgeschlagen: %#v", envelope.Error)
	}
	written, err := os.ReadFile(review.EntryFile(runDir, "tech"))
	if err != nil {
		t.Fatalf("Entry-Datei lesen: %v", err)
	}
	if strings.Contains(string(written), "\"jobs\"") {
		t.Fatalf("Perspektive schreibt ein jobs-Feld: %s", written)
	}
}

// TestReviewStatusTriageZustand prüft den Zeitvergleich, mit dem der Status eine
// Bewertung als veraltet meldet.
//
// Der Eintragszustand allein trägt das nicht: markAIRepairStatus misst an
// review-triage.md nur Existenz und Größe und bleibt nach einem erneuten Merge
// unverändert done und konsistent.
func TestReviewStatusTriageZustand(t *testing.T) {
	merged := time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		name       string
		setup      func(t *testing.T, root string, runDir string)
		state      string
		wantReason bool
	}{
		{
			name: "aktuell: Bewertung nach dem Merge geschrieben",
			setup: func(t *testing.T, root string, runDir string) {
				mustWriteMergeArtifact(t, runDir, merged)
				mustWriteTriageEntry(t, root, runDir, merged.Add(5*time.Minute).Format(timeLayout))
			},
			state: triageStateCurrent,
		},
		{
			name: "aktuell: gleiche Zeiten",
			setup: func(t *testing.T, root string, runDir string) {
				mustWriteMergeArtifact(t, runDir, merged)
				mustWriteTriageEntry(t, root, runDir, merged.Format(timeLayout))
			},
			state: triageStateCurrent,
		},
		{
			name: "veraltet: Merge lief nach der Bewertung",
			setup: func(t *testing.T, root string, runDir string) {
				mustWriteTriageEntry(t, root, runDir, merged.Format(timeLayout))
				mustWriteMergeArtifact(t, runDir, merged.Add(10*time.Minute))
			},
			state:      triageStateStale,
			wantReason: true,
		},
		{
			name: "veraltet: scan-triage nennt keine Endzeit",
			setup: func(t *testing.T, root string, runDir string) {
				mustWriteMergeArtifact(t, runDir, merged)
				mustWriteTriageEntry(t, root, runDir, "")
			},
			state:      triageStateStale,
			wantReason: true,
		},
		{
			name: "veraltet: Endzeit ist kein Zeitstempel",
			setup: func(t *testing.T, root string, runDir string) {
				mustWriteMergeArtifact(t, runDir, merged)
				mustWriteFile(t, filepath.Join(runDir, scanTriageResult), minimalTriage())
				mustWriteJSON(t, review.EntryFile(runDir, scanTriageEntry), aiEntryStatus{
					Name:       scanTriageEntry,
					Kind:       review.KindAI,
					State:      review.StateDone,
					Result:     scanTriageResult,
					FinishedAt: "gestern",
				})
			},
			state:      triageStateStale,
			wantReason: true,
		},
		{
			name: "veraltet: Bewertung ohne Eintrag ist nicht belegbar",
			setup: func(t *testing.T, root string, runDir string) {
				mustWriteMergeArtifact(t, runDir, merged)
				mustWriteFile(t, filepath.Join(runDir, scanTriageResult), minimalTriage())
			},
			state:      triageStateStale,
			wantReason: true,
		},
		{
			name: "veraltet: review-input.json fehlt",
			setup: func(t *testing.T, root string, runDir string) {
				mustWriteTriageEntry(t, root, runDir, merged.Format(timeLayout))
			},
			state:      triageStateStale,
			wantReason: true,
		},
		{
			name:  "fehlt: keine Bewertung im Laufordner",
			setup: func(t *testing.T, root string, runDir string) { mustWriteMergeArtifact(t, runDir, merged) },
			state: triageStateMissing,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := newReviewProject(t)
			runDir := mustCreateRun(t, root, []review.Entry{scanTriageRunEntry(root)})
			test.setup(t, root, runDir)
			triage := triageStatus(t, root, "2026-08-19")
			if triage["state"] != test.state {
				t.Fatalf("state = %v, erwartet %s (%#v)", triage["state"], test.state, triage)
			}
			reason, _ := triage["reason"].(string)
			if test.wantReason != (strings.TrimSpace(reason) != "") {
				t.Fatalf("reason = %q, erwartet vorhanden = %v", reason, test.wantReason)
			}
		})
	}
}

// TestReviewStatusTriageVeraltetNachErneutemMerge ist der Fall, für den der
// Vergleich da ist: der Eintrag bleibt done und konsistent, die Bewertung ist
// trotzdem nicht mehr die zu diesem review-input.json.
func TestReviewStatusTriageVeraltetNachErneutemMerge(t *testing.T) {
	root := newReviewProject(t)
	runDir := mustCreateRun(t, root, []review.Entry{scanTriageRunEntry(root)})
	merged := time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC)
	mustWriteMergeArtifact(t, runDir, merged)
	mustWriteTriageEntry(t, root, runDir, merged.Add(5*time.Minute).Format(timeLayout))
	if state := triageStatus(t, root, "2026-08-19")["state"]; state != triageStateCurrent {
		t.Fatalf("vor dem erneuten Merge: state = %v", state)
	}

	mustWriteMergeArtifact(t, runDir, merged.Add(30*time.Minute))

	result, _, err := reviewStatusTool(context.Background(), nil, reviewStatusInput{reviewBaseInput: reviewBaseInput{ProjectDir: root}, Run: "2026-08-19"})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	data := decodeReviewEnvelope(t, result).Data.(map[string]any)
	entry := statusEntry(t, data["entries"].([]any), scanTriageEntry)
	if entry["state"] != string(review.StateDone) || entry["inconsistent"] != nil {
		t.Fatalf("Eintrag sollte done und konsistent bleiben: %#v", entry)
	}
	triage := data["triage"].(map[string]any)
	if triage["state"] != triageStateStale {
		t.Fatalf("Bewertung sollte veraltet sein: %#v", triage)
	}
}

// scanTriageRunEntry ist der Laufeintrag des Bewertungsmoduls.
func scanTriageRunEntry(root string) review.Entry {
	return review.Entry{
		Name:           scanTriageEntry,
		Kind:           review.KindAI,
		RecipeKey:      scanTriageEntry,
		RecipePath:     filepath.Join(root, project.PlaybookDirName, "commands", filepath.FromSlash(scanTriageModule)),
		RecipeOrigin:   "dist",
		Title:          "Review-Triage",
		ResultRequired: boolPtr(true),
		DefaultResult:  scanTriageResult,
	}
}

// mustWriteMergeArtifact legt review-input.json mit einer festen Änderungszeit
// an. Die Zeit wird gesetzt und nicht abgewartet: der Vergleich hängt an ihr.
func mustWriteMergeArtifact(t *testing.T, runDir string, modified time.Time) {
	t.Helper()
	path := filepath.Join(runDir, reviewInputJSON)
	mustWriteFile(t, path, "{}\n")
	if err := os.Chtimes(path, modified, modified); err != nil {
		t.Fatalf("Änderungszeit setzen: %v", err)
	}
}

// mustWriteTriageEntry schreibt review-triage.md und meldet den Eintrag über das
// Werkzeug, damit die Endzeit denselben Weg nimmt wie im Lauf.
func mustWriteTriageEntry(t *testing.T, root string, runDir string, finishedAt string) {
	t.Helper()
	mustWriteFile(t, filepath.Join(runDir, scanTriageResult), minimalTriage())
	result, _, err := reviewWriteAIEntryTool(context.Background(), nil, reviewWriteAIEntryInput{
		reviewBaseInput: reviewBaseInput{ProjectDir: root},
		Run:             "2026-08-19",
		Entry:           scanTriageEntry,
		State:           review.StateDone,
		Result:          scanTriageResult,
		FinishedAt:      finishedAt,
	})
	if err != nil {
		t.Fatalf("write_ai_entry: %v", err)
	}
	if envelope := decodeReviewEnvelope(t, result); !envelope.OK {
		t.Fatalf("write_ai_entry fehlgeschlagen: %#v", envelope.Error)
	}
}

// triageStatus liest den Bewertungszustand aus dem Laufstatus.
func triageStatus(t *testing.T, root string, run string) map[string]any {
	t.Helper()
	result, _, err := reviewStatusTool(context.Background(), nil, reviewStatusInput{reviewBaseInput: reviewBaseInput{ProjectDir: root}, Run: run})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	envelope := decodeReviewEnvelope(t, result)
	if !envelope.OK {
		t.Fatalf("status fehlgeschlagen: %#v", envelope.Error)
	}
	triage, ok := envelope.Data.(map[string]any)["triage"].(map[string]any)
	if !ok {
		t.Fatalf("kein triage-Block im Status: %#v", envelope.Data)
	}
	return triage
}
