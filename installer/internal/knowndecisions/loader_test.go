package knowndecisions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseMarkdownPflichtfelderUndKategorie(t *testing.T) {
	_, warnings := ParseMarkdown("## kd-test\n\n```yaml\nid: kd-test\ncategory: wrong\nmatch:\n  - pathGlob: app/**\n```\n\nBegründung.\n", "known-decisions.md", "project")
	if len(warnings) == 0 {
		t.Fatalf("ungültige Kategorie ohne Warnung")
	}

	_, warnings = ParseMarkdown("## kd-test\n\n```yaml\ncategory: wontfix\nmatch:\n  - pathGlob: app/**\n```\n", "known-decisions.md", "project")
	if len(warnings) == 0 {
		t.Fatalf("fehlende id ohne Warnung")
	}
}

func TestParseMarkdownMatchLeerUndRuleIDAllein(t *testing.T) {
	_, warnings := ParseMarkdown("## kd-empty\n\n```yaml\nid: kd-empty\ncategory: wontfix\nmatch:\n```\n", "known-decisions.md", "project")
	if len(warnings) == 0 {
		t.Fatalf("leeres match ohne Warnung")
	}

	_, warnings = ParseMarkdown("## kd-rule\n\n```yaml\nid: kd-rule\ncategory: false-positive\nmatch:\n  - ruleId: G304\n```\n", "known-decisions.md", "project")
	if len(warnings) == 0 {
		t.Fatalf("ruleId ohne location ohne Warnung")
	}
}

func TestExpired(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	if !Expired(Decision{Expires: "2026-08-21"}, now) {
		t.Fatalf("Decision am Ablaufdatum muss abgelaufen sein")
	}
	if Expired(Decision{Expires: "2026-08-22"}, now) {
		t.Fatalf("zukünftige Decision abgelaufen")
	}
}

func TestLoadLiestProjektweiteDatei(t *testing.T) {
	root := t.TempDir()
	writeKnownDecision(t, filepath.Join(root, fileName), "kd-shared", "wontfix", "project/**")

	decisions, report, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(decisions) != 1 || decisions[0].ID != "kd-shared" || decisions[0].Source != "project" {
		t.Fatalf("projektweite Decision nicht geladen: %+v", decisions)
	}
	if len(report.Sources) != 1 || report.Sources[0].Scope != "project" || !report.Sources[0].Loaded {
		t.Fatalf("Quellenbericht falsch: %+v", report.Sources)
	}
	if report.Sources[0].Path != filepath.Join(root, fileName) {
		t.Fatalf("falscher Ort gelesen: %s", report.Sources[0].Path)
	}
	if len(report.Warnings) != 0 {
		t.Fatalf("unerwartete Warnung: %+v", report.Warnings)
	}
}

// Eine known-decisions.md im Laufverzeichnis gibt es nicht mehr: sie wird weder
// gelesen noch als Quelle gemeldet.
func TestLoadIgnoriertLaufverzeichnis(t *testing.T) {
	root := t.TempDir()
	runDir := filepath.Join(root, "results", "2026-08-21")
	writeKnownDecision(t, filepath.Join(root, fileName), "kd-shared", "wontfix", "project/**")
	writeKnownDecision(t, filepath.Join(runDir, fileName), "kd-run", "accepted-risk", "run/**")

	decisions, report, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, decision := range decisions {
		if decision.ID == "kd-run" {
			t.Fatalf("Decision aus dem Laufverzeichnis geladen: %+v", decisions)
		}
	}
	for _, source := range report.Sources {
		if source.Path == filepath.Join(runDir, fileName) {
			t.Fatalf("Laufverzeichnis steht noch im Quellenbericht: %+v", report.Sources)
		}
	}
}

// Übergang: liegt die Datei nur noch am alten Ort, wird sie gelesen und der
// Umzug gemeldet. Entfällt mit dem Ausbau zum 2027-02-28.
func TestLoadLiestAltenOrtUndMeldetUmzug(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, "results", fileName)
	writeKnownDecision(t, legacy, "kd-alt", "wontfix", "project/**")

	decisions, report, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(decisions) != 1 || decisions[0].ID != "kd-alt" {
		t.Fatalf("Decision vom alten Ort nicht geladen: %+v", decisions)
	}
	if report.Sources[0].Path != legacy || !report.Sources[0].Loaded {
		t.Fatalf("alter Ort nicht als Quelle gemeldet: %+v", report.Sources)
	}
	if len(report.Warnings) != 1 || !strings.Contains(report.Warnings[0], filepath.Join(root, fileName)) {
		t.Fatalf("Umzugswarnung nennt den neuen Ort nicht: %+v", report.Warnings)
	}
}

// Übergang: liegen beide Orte vor, gewinnt der neue; der alte wird als ignoriert
// gemeldet. Entfällt mit dem Ausbau zum 2027-02-28.
func TestLoadBevorzugtNeuenOrtUndMeldetAlten(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, "results", fileName)
	writeKnownDecision(t, filepath.Join(root, fileName), "kd-neu", "wontfix", "project/**")
	writeKnownDecision(t, legacy, "kd-alt", "accepted-risk", "alt/**")

	decisions, report, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(decisions) != 1 || decisions[0].ID != "kd-neu" {
		t.Fatalf("neuer Ort gewinnt nicht: %+v", decisions)
	}
	if report.Sources[0].Path != filepath.Join(root, fileName) {
		t.Fatalf("falsche Quelle: %+v", report.Sources)
	}
	if len(report.Warnings) != 1 || !strings.Contains(report.Warnings[0], legacy) {
		t.Fatalf("alter Ort nicht als ignoriert gemeldet: %+v", report.Warnings)
	}
}

func TestMatchJeKriterium(t *testing.T) {
	now := time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)
	finding := Finding{
		StableID:   "scan-gosec-abc123",
		RuleID:     "G304",
		Locations:  []Location{{Path: "internal/app.go"}, {Path: "_old/internal/app.go"}},
		Dependency: Dependency{Package: "lib", Version: "1.2.3", Manifest: "go.mod", IDs: []string{"CVE-2026-1234", "GHSA-abcd-efgh-ijkl", "OSV-2026-1"}},
	}
	cases := []struct {
		name      string
		criteria  []Criterion
		matchedBy string
	}{
		{"stable", []Criterion{{StableID: "scan-gosec-abc123"}}, "stableId"},
		{"rule", []Criterion{{RuleID: "G304", Location: "internal/**"}}, "ruleId+location"},
		{"cve", []Criterion{{CVEID: "CVE-2026-1234", Package: "lib"}}, "cveId"},
		{"ghsa", []Criterion{{GHSAID: "GHSA-abcd-efgh-ijkl", Version: "1.2.3"}}, "ghsaId"},
		{"osv", []Criterion{{OSVID: "OSV-2026-1", ManifestGlob: "go.mod"}}, "osvId"},
		{"path", []Criterion{{PathGlob: "_old/**"}}, "pathGlob"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			match := Match(finding, []Decision{{ID: "kd-" + tc.name, Category: "wontfix", Match: tc.criteria}}, now)
			if match == nil || match.MatchedBy != tc.matchedBy {
				t.Fatalf("kein Treffer über %s: %+v", tc.matchedBy, match)
			}
		})
	}
}

func TestMatchSortiertePrimaerUndSekundaer(t *testing.T) {
	finding := Finding{Locations: []Location{{Path: "_old/a.go"}}}
	decisions := []Decision{
		{ID: "kd-b", Category: "wontfix", Match: []Criterion{{PathGlob: "_old/**"}}},
		{ID: "kd-a", Category: "false-positive", Match: []Criterion{{PathGlob: "_old/**"}}},
	}
	// Load sortiert; Match selbst nutzt die gegebene Reihenfolge.
	decisions[0], decisions[1] = decisions[1], decisions[0]
	match := Match(finding, decisions, time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC))
	if match == nil || match.Decision.ID != "kd-a" || len(match.Secondary) != 1 || match.Secondary[0].ID != "kd-b" {
		t.Fatalf("primäre/sekundäre Treffer falsch: %+v", match)
	}
}

func TestMatchIgnoriertAbgelaufeneDecision(t *testing.T) {
	match := Match(Finding{Locations: []Location{{Path: "app.go"}}}, []Decision{{ID: "kd-old", Category: "wontfix", Expires: "2026-08-20", Match: []Criterion{{PathGlob: "**/*.go"}}}}, time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC))
	if match != nil {
		t.Fatalf("abgelaufene Decision matcht: %+v", match)
	}
}

func writeKnownDecision(t *testing.T, path string, id string, category string, pathGlob string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("Verzeichnis anlegen: %v", err)
	}
	content := "## " + id + "\n\n```yaml\nid: " + id + "\ncategory: " + category + "\nmatch:\n  - pathGlob: " + pathGlob + "\n```\n\nBegründung.\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("%s schreiben: %v", path, err)
	}
}
