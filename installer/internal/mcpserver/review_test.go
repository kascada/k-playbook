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
	if _, err := os.Stat(review.EntryFile(filepath.Join(project.LocalDir(root), review.ResultsDirName, "2026-08-19"), scanTriageEntry)); err != nil {
		t.Fatalf("scan-triage Entry fehlt: %v", err)
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
	mustWriteFile(t, filepath.Join(root, "k-playbook", "reviews", "review-tech.md"), "---\ntitle: Technischer Review\naudit:\n  enabled: true\n  resultRequired: true\n  defaultResult: review-tech.md\nreview:\n  enabled: true\n---\n# Fallback\n")
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
