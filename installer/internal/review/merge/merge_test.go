package merge

import (
	"encoding/json"
	"os"
	"path/filepath"
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

func fixedNow() time.Time {
	return time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC)
}

func finding(id string, tool string, rule string, uri string, line int, message string, dependency Dependency) Finding {
	return Finding{
		ID: id,
		Evidence: Evidence{
			Tool:        tool,
			Job:         tool,
			SARIF:       "raw/" + tool + ".sarif",
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
