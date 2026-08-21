package merge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kascada/k-playbook/installer/internal/review"
)

func intPtr(value int) *int { return &value }

func writeJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("JSON bauen: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("Verzeichnis anlegen: %v", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatalf("%s schreiben: %v", path, err)
	}
}

func writeText(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("Verzeichnis anlegen: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("%s schreiben: %v", path, err)
	}
}

func TestBuildLiestSARIFUndNormalisiertFindings(t *testing.T) {
	projectDir := t.TempDir()
	runDir := filepath.Join(projectDir, review.ResultsDirName, "2026-08-19")
	writeJSON(t, filepath.Join(runDir, review.RunFileName), review.Run{
		SchemaVersion: review.SchemaVersion,
		Created:       "2026-08-19T12:00:00Z",
		State:         review.StateCreated,
		Languages:     []string{"go"},
		Entries:       []review.Entry{{Name: "semgrep", Kind: review.KindTool, State: review.StateStart}},
	})
	writeJSON(t, review.EntryFile(runDir, "semgrep"), review.EntryStatus{
		SchemaVersion: review.EntrySchemaVersion,
		Name:          "semgrep",
		Kind:          review.KindTool,
		State:         review.StateDone,
		Jobs: []review.JobStatus{{
			Job: "semgrep", State: review.StateDone, SARIF: "raw/semgrep.sarif",
			Findings: intPtr(1), Candidates: intPtr(2),
		}},
	})
	writeText(t, filepath.Join(runDir, "raw", "semgrep.sarif"), `{
  "version": "2.1.0",
  "runs": [{
    "tool": {"driver": {"name": "semgrep", "rules": [{"id": "go.security", "name": "Security", "fullDescription": {"text": "Beschreibung"}}]}},
    "results": [{
      "ruleId": "go.security",
      "level": "warning",
      "message": {"text": "CVE-2026-1234 in lib"},
      "locations": [{"physicalLocation": {"artifactLocation": {"uri": "main.go"}, "region": {"startLine": 7, "startColumn": 2}}}],
      "fingerprints": {"match": "fp1"},
      "properties": {"package": "lib", "version": "1.2.3", "manifest": "go.mod"}
    }]
  }]
}`)

	result, err := Build(Options{ProjectDir: projectDir, RunName: "2026-08-19", RunDir: runDir, Now: fixedNow})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if result.SchemaVersion != 1 || result.Generated != "2026-08-19T10:00:00Z" {
		t.Fatalf("Provenienz fehlt: %+v", result)
	}
	if result.Run.Dir != "results/2026-08-19" || result.Run.DerivedState != review.StateDone {
		t.Errorf("Run-Kontext falsch: %+v", result.Run)
	}
	if len(result.Findings) != 1 {
		t.Fatalf("%d Findings, erwartet 1", len(result.Findings))
	}
	finding := result.Findings[0]
	if finding.Evidence.Tool != "semgrep" || finding.Evidence.Job != "semgrep" || finding.Evidence.SARIF != "raw/semgrep.sarif" {
		t.Errorf("Evidence falsch: %+v", finding.Evidence)
	}
	if finding.RuleID != "go.security" || finding.RuleName != "Security" || finding.RuleDescription != "Beschreibung" {
		t.Errorf("Rule-Kontext falsch: %+v", finding)
	}
	if finding.Location.URI != "main.go" || finding.Location.StartLine != 7 || finding.Location.StartColumn != 2 {
		t.Errorf("Location falsch: %+v", finding.Location)
	}
	if finding.Dependency.Package != "lib" || finding.Dependency.Version != "1.2.3" || len(finding.Dependency.IDs) != 1 || finding.Dependency.IDs[0] != "CVE-2026-1234" {
		t.Errorf("Dependency falsch: %+v", finding.Dependency)
	}
	if finding.DerivedSeverity != "warning" || finding.SeveritySource != "native" {
		t.Errorf("abgeleitete Schwere falsch: %+v", finding)
	}
}

func TestBuildFuehrtFehlendeEntryDateiAlsStart(t *testing.T) {
	projectDir := t.TempDir()
	runDir := filepath.Join(projectDir, review.ResultsDirName, "2026-08-19")
	writeJSON(t, filepath.Join(runDir, review.RunFileName), review.Run{
		SchemaVersion: review.SchemaVersion,
		Created:       "2026-08-19T12:00:00Z",
		State:         review.StateCreated,
		Languages:     []string{"python"},
		Entries:       []review.Entry{{Name: "ruff", Kind: review.KindTool, State: review.StateStart}},
	})

	result, err := Build(Options{ProjectDir: projectDir, RunName: "2026-08-19", RunDir: runDir})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(result.Entries) != 1 || result.Entries[0].Present || result.Entries[0].State != review.StateStart {
		t.Fatalf("fehlende Entry-Datei nicht als start sichtbar: %+v", result.Entries)
	}
	if len(result.Findings) != 0 {
		t.Fatalf("Findings aus fehlendem Entry: %+v", result.Findings)
	}
}

func TestGroupFindingsExaktGleich(t *testing.T) {
	groups := GroupFindings([]Finding{
		finding("a", "tool-a", "rule", "app.go", 12, "Gefahr hier", Dependency{}),
		finding("b", "tool-b", "rule", "app.go", 12, "Gefahr   hier", Dependency{}),
	})
	if len(groups) != 1 || len(groups[0].FindingIDs) != 2 || len(groups[0].Evidence) != 2 {
		t.Fatalf("exakte Dublette nicht gruppiert: %+v", groups)
	}
}

func TestGroupFindingsGleicherCVEAusZweiTools(t *testing.T) {
	dependency := Dependency{Package: "lib", Version: "1.0", Manifest: "go.mod", IDs: []string{"CVE-2026-1234"}}
	groups := GroupFindings([]Finding{
		finding("a", "trivy", "CVE-2026-1234", "go.mod", 1, "lib", dependency),
		finding("b", "grype", "GHSA-x", "go.mod", 2, "lib", dependency),
	})
	if len(groups) != 1 || len(groups[0].FindingIDs) != 2 {
		t.Fatalf("Dependency-Dublette nicht gruppiert: %+v", groups)
	}
}

func TestGroupFindingsLocationNurPossibleDuplicate(t *testing.T) {
	groups := GroupFindings([]Finding{
		finding("a", "semgrep", "path-traversal-a", "app.go", 12, "A", Dependency{}),
		finding("b", "gosec", "path-traversal-b", "app.go", 12, "B", Dependency{}),
	})
	if len(groups) != 2 {
		t.Fatalf("Location-Dublette hart zusammengelegt: %+v", groups)
	}
	if len(groups[0].PossibleDuplicates) != 1 || len(groups[1].PossibleDuplicates) != 1 {
		t.Fatalf("possible-duplicate fehlt: %+v", groups)
	}
}

func TestGroupFindingsSameLocationInnerhalbEinesJobs(t *testing.T) {
	groups := GroupFindings([]Finding{
		findingWithJob("a", "trivy", "trivy-fs", "CVE-2026-1111", "requirements.txt", 29, "A", Dependency{}),
		findingWithJob("b", "trivy", "trivy-fs", "CVE-2026-2222", "requirements.txt", 29, "B", Dependency{}),
		findingWithJob("c", "trivy", "trivy-config", "CVE-2026-3333", "requirements.txt", 29, "C", Dependency{}),
		findingWithJob("d", "grype", "grype", "CVE-2026-4444", "requirements.txt", 29, "D", Dependency{}),
	})
	if len(groups) != 3 {
		t.Fatalf("unerwartete Gruppen: %+v", groups)
	}
	if len(groups[0].FindingIDs) != 2 || len(groups[0].Evidence) != 2 {
		t.Fatalf("Same-Location-Bundle fehlt: %+v", groups[0])
	}
	if !contains(groups[0].DedupeRules, "same-location-tool") {
		t.Fatalf("Dedupe-Regel fehlt: %+v", groups[0].DedupeRules)
	}
	if len(groups[1].PossibleDuplicates) == 0 || len(groups[2].PossibleDuplicates) == 0 {
		t.Fatalf("Cross-Job/Cross-Tool nicht als possible markiert: %+v", groups)
	}
}

func TestGroupFindingsKeineZusammenlegungBeiUnsichererLage(t *testing.T) {
	groups := GroupFindings([]Finding{
		finding("a", "semgrep", "sql-injection", "app.go", 12, "A", Dependency{}),
		finding("b", "gosec", "path-traversal", "app.go", 13, "B", Dependency{}),
	})
	if len(groups) != 2 {
		t.Fatalf("unsichere Lage wurde zusammengelegt: %+v", groups)
	}
	if len(groups[0].PossibleDuplicates) != 0 || len(groups[1].PossibleDuplicates) != 0 {
		t.Fatalf("unsichere Lage als possible markiert: %+v", groups)
	}
}

func TestWriteSchreibtArtefakte(t *testing.T) {
	runDir := t.TempDir()
	result := Result{
		SchemaVersion: 1,
		Generated:     "2026-08-19T10:00:00Z",
		Run:           RunContext{Name: "2026-08-19", Dir: "k-playbook-local/results/2026-08-19", Languages: []string{"go"}},
		Entries:       []EntrySummary{{Name: "ruff", Kind: review.KindTool, State: review.StateStart, Jobs: []JobSummary{}}},
	}
	output, err := Write(Options{RunDir: runDir}, result)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	for _, path := range []string{output.JSON, output.Markdown} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("Artefakt fehlt: %s: %v", path, err)
		}
	}
}

func TestBuildSchreibtKnownDecisionCoverageUndLaesstRawUnveraendert(t *testing.T) {
	projectDir := t.TempDir()
	localResultsDir := filepath.Join(projectDir, review.ResultsDirName)
	runDir := filepath.Join(localResultsDir, "2026-08-19")
	writeJSON(t, filepath.Join(runDir, review.RunFileName), review.Run{
		SchemaVersion: review.SchemaVersion,
		Created:       "2026-08-19T12:00:00Z",
		State:         review.StateCreated,
		Languages:     []string{"go"},
		Entries:       []review.Entry{{Name: "gosec", Kind: review.KindTool, State: review.StateStart}},
	})
	writeJSON(t, review.EntryFile(runDir, "gosec"), review.EntryStatus{
		SchemaVersion: review.EntrySchemaVersion,
		Name:          "gosec",
		Kind:          review.KindTool,
		State:         review.StateDone,
		Jobs:          []review.JobStatus{{Job: "gosec", State: review.StateDone, SARIF: "raw/gosec.sarif", Findings: intPtr(1)}},
	})
	rawPath := filepath.Join(runDir, "raw", "gosec.sarif")
	rawContent := `{
  "version": "2.1.0",
  "runs": [{
    "tool": {"driver": {"name": "gosec", "rules": [{"id": "G304", "name": "File path"}]}},
    "results": [{
      "ruleId": "G304",
      "level": "warning",
      "message": {"text": "Potential file inclusion"},
      "locations": [{"physicalLocation": {"artifactLocation": {"uri": "_old/internal/app.go"}, "region": {"startLine": 7}}}]
    }]
  }]
}`
	writeText(t, rawPath, rawContent)
	writeText(t, filepath.Join(localResultsDir, "known-decisions.md"), "## kd-old-tree\n\n```yaml\nid: kd-old-tree\ncategory: wontfix\nmatch:\n  - pathGlob: _old/**\n```\n\nBegründung.\n")
	before, err := os.ReadFile(rawPath)
	if err != nil {
		t.Fatalf("Raw lesen: %v", err)
	}

	result, output, err := Run(Options{ProjectDir: projectDir, RunName: "2026-08-19", RunDir: runDir, LocalResultsDir: localResultsDir, Now: fixedNow})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	after, err := os.ReadFile(rawPath)
	if err != nil {
		t.Fatalf("Raw nach Run lesen: %v", err)
	}
	if string(before) != string(after) {
		t.Fatalf("Raw wurde verändert")
	}
	if len(result.Findings) != 1 || result.Findings[0].CoveredByKnownDecision == nil || result.Findings[0].CoveredByKnownDecision.ID != "kd-old-tree" {
		t.Fatalf("Finding-Deckung fehlt: %+v", result.Findings)
	}
	if len(result.Groups) != 1 || result.Groups[0].CoveredByKnownDecision == nil || result.Groups[0].PartialCoverage {
		t.Fatalf("Gruppen-Deckung falsch: %+v", result.Groups)
	}
	markdownData, err := os.ReadFile(output.Markdown)
	if err != nil {
		t.Fatalf("Markdown lesen: %v", err)
	}
	if !strings.Contains(string(markdownData), "Gruppen mit vollständiger Deckung: 1 von 1") || !strings.Contains(string(markdownData), "kd-old-tree (wontfix)") {
		t.Fatalf("Markdown-Deckung fehlt: %s", markdownData)
	}
}

func TestSeverityMappingLeitetSchwereAb(t *testing.T) {
	mapping, err := ParseSeverityMapping("tool\trule_prefix\tseverity\tnotes\nsemgrep\tpython.django.security.audit\twarning\tTest\n", "test.tsv")
	if err != nil {
		t.Fatalf("Mapping lesen: %v", err)
	}
	finding := Finding{Evidence: Evidence{Tool: "semgrep"}, RuleID: "python.django.security.audit.csrf-exempt.no-csrf-exempt"}
	severity, source := deriveSeverity(finding, sarifResult{}, sarifRule{}, mapping)
	if severity != "warning" || source != "mapping" {
		t.Fatalf("Mapping nicht genutzt: %s/%s", severity, source)
	}
	unknown, unknownSource := deriveSeverity(Finding{Evidence: Evidence{Tool: "semgrep"}, RuleID: "unknown.rule"}, sarifResult{}, sarifRule{}, mapping)
	if unknown != "unmapped" || unknownSource != "unmapped" {
		t.Fatalf("Restkategorie falsch: %s/%s", unknown, unknownSource)
	}
	native, nativeSource := deriveSeverity(Finding{Level: "error", Evidence: Evidence{Tool: "semgrep"}, RuleID: "python.django.security.audit.x"}, sarifResult{}, sarifRule{}, mapping)
	if native != "error" || nativeSource != "native" {
		t.Fatalf("native Schwere nicht vorrangig: %s/%s", native, nativeSource)
	}
	weak, weakSource := deriveSeverity(Finding{Level: "warning", Evidence: Evidence{Tool: "semgrep"}, RuleID: "python.django.security.audit.x"}, sarifResult{}, sarifRule{}, mapping)
	if weak != "warning" || weakSource != "mapping" {
		t.Fatalf("Mapping überschreibt schwaches warning nicht: %s/%s", weak, weakSource)
	}
}

func TestJSONEnthaeltKeineEntrySource(t *testing.T) {
	runDir := t.TempDir()
	result := Result{SchemaVersion: 1, Entries: []EntrySummary{{Name: "semgrep", Kind: review.KindTool, State: review.StateDone, Jobs: []JobSummary{{Job: "semgrep", State: review.StateDone, SARIF: "raw/semgrep.sarif"}}}}}
	output, err := Write(Options{RunDir: runDir}, result)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	data, err := os.ReadFile(output.JSON)
	if err != nil {
		t.Fatalf("JSON lesen: %v", err)
	}
	if strings.Contains(string(data), "\"source\"") {
		t.Fatalf("entries[].source ist noch im JSON: %s", data)
	}
	if !strings.Contains(string(data), "\"job\": \"semgrep\"") || !strings.Contains(string(data), "\"sarif\": \"raw/semgrep.sarif\"") {
		t.Fatalf("Rückführdaten fehlen: %s", data)
	}
}

func TestMarkdownToolsListetZeroDoneUndKeineSkips(t *testing.T) {
	result := Result{
		Entries: []EntrySummary{
			{Name: "ruff", State: review.StateDone, Jobs: []JobSummary{{Job: "ruff", State: review.StateDone, SARIF: "raw/ruff.sarif"}}},
			{Name: "syft", State: review.StateSkipped, Jobs: []JobSummary{}},
		},
		Findings: []Finding{},
	}
	content := markdown(result)
	if !strings.Contains(content, "- ruff: 0") {
		t.Fatalf("Zero-Findings-Tool fehlt: %s", content)
	}
	if strings.Contains(content, "- syft: 0") {
		t.Fatalf("skipped Tool im Zahlenblock: %s", content)
	}
}

func TestStableIDBleibtBeiGleicherGruppeStabil(t *testing.T) {
	findings := []Finding{
		findingWithJob("a", "trivy", "trivy-fs", "CVE-2026-1111", "requirements.txt", 29, "A", Dependency{}),
		findingWithJob("b", "trivy", "trivy-fs", "CVE-2026-2222", "requirements.txt", 29, "B", Dependency{}),
	}
	first := GroupFindings(findings)
	second := GroupFindings([]Finding{findings[1], findings[0]})
	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("Testdaten nicht gebündelt: %+v / %+v", first, second)
	}
	if first[0].StableID != second[0].StableID || first[0].StableKey != second[0].StableKey {
		t.Fatalf("Stable-ID nicht stabil: %+v / %+v", first[0], second[0])
	}
	if first[0].ID == "" || !strings.HasPrefix(first[0].StableID, "scan-trivy-") {
		t.Fatalf("Anzeige-ID oder Präfix falsch: %+v", first[0])
	}
}

func TestStableIDUnterscheidetUnterschiedlicheGruppen(t *testing.T) {
	groups := GroupFindings([]Finding{
		finding("a", "semgrep", "sql-injection", "app.go", 12, "A", Dependency{}),
		finding("b", "semgrep", "xss", "app.go", 13, "B", Dependency{}),
	})
	if len(groups) != 2 {
		t.Fatalf("unerwartete Gruppen: %+v", groups)
	}
	if groups[0].StableID == groups[1].StableID || groups[0].StableKey == groups[1].StableKey {
		t.Fatalf("Stable-IDs nicht eindeutig: %+v", groups)
	}
}

func TestStableIDPrefixFuerDependency(t *testing.T) {
	dependency := Dependency{Package: "lib", Version: "1.0", Manifest: "requirements.txt", IDs: []string{"CVE-2026-1234"}}
	groups := GroupFindings([]Finding{finding("a", "trivy", "CVE-2026-1234", "requirements.txt", 1, "lib", dependency)})
	if len(groups) != 1 || !strings.HasPrefix(groups[0].StableID, "scan-cve-cve-2026-1234-") {
		t.Fatalf("Dependency-Präfix falsch: %+v", groups)
	}
}

func TestStableIDKollisionVerlaengertHashDeterministisch(t *testing.T) {
	original := stableDigest
	defer func() { stableDigest = original }()
	stableDigest = func(key string) string {
		if strings.Contains(key, "app.go:12") {
			return "abcdef1111111111111111111111111111111111111111111111111111111111"
		}
		return "abcdef2222222222222222222222222222222222222222222222222222222222"
	}
	groups := GroupFindings([]Finding{
		finding("a", "semgrep", "rule-a", "app.go", 12, "A", Dependency{}),
		finding("b", "semgrep", "rule-b", "app.go", 13, "B", Dependency{}),
	})
	if len(groups) != 2 {
		t.Fatalf("unerwartete Gruppen: %+v", groups)
	}
	if groups[0].StableID == groups[1].StableID {
		t.Fatalf("Kollision nicht aufgelöst: %+v", groups)
	}
	if len(strings.TrimPrefix(groups[0].StableID, "scan-semgrep-")) != 7 || len(strings.TrimPrefix(groups[1].StableID, "scan-semgrep-")) != 7 {
		t.Fatalf("Hash nicht deterministisch verlängert: %+v", groups)
	}
}

func fixedNow() time.Time {
	return time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC)
}

func finding(id string, tool string, rule string, uri string, line int, message string, dependency Dependency) Finding {
	return findingWithJob(id, tool, tool, rule, uri, line, message, dependency)
}

func findingWithJob(id string, tool string, job string, rule string, uri string, line int, message string, dependency Dependency) Finding {
	return Finding{
		ID: id,
		Evidence: Evidence{
			Tool:        tool,
			Job:         job,
			SARIF:       "raw/" + job + ".sarif",
			ResultIndex: 1,
		},
		RuleID:     rule,
		RuleName:   rule,
		Level:      "warning",
		Message:    message,
		Location:   Location{URI: uri, StartLine: line},
		Dependency: dependency,
	}
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
