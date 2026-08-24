package merge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
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

// Der realistische Fall: dieselbe Schwachstelle, aber jedes Werkzeug nennt eine
// andere Aliasmenge und schreibt den Manifest-Pfad anders. Gemessen an einem
// echten Lauf (Task 027, Etappe 1): pip-audit nennt CVE, GHSA und PYSEC,
// osv-scanner CVE und GHSA, grype ausschließlich die GHSA.
func TestGroupFindingsGleicheSchwachstelleUnterschiedlicheAliasmengen(t *testing.T) {
	groups := GroupFindings([]Finding{
		finding("a", "pip-audit", "CVE-2026-1234", "tmp/requirements.txt", 0, "requests 2.19.0", Dependency{
			Package: "requests", Version: "2.19.0", Manifest: "tmp/requirements.txt",
			IDs:    []string{"CVE-2026-1234", "GHSA-AAAA-BBBB-CCCC", "PYSEC-2026-1"},
			KeyIDs: []string{"CVE-2026-1234", "GHSA-AAAA-BBBB-CCCC", "PYSEC-2026-1"},
		}),
		finding("b", "grype", "GHSA-aaaa-bbbb-cccc-requests", "/tmp/requirements.txt", 0, "requests", Dependency{
			Package: "requests", Version: "2.19.0", Manifest: "/tmp/requirements.txt",
			IDs:    []string{"GHSA-AAAA-BBBB-CCCC"},
			KeyIDs: []string{"GHSA-AAAA-BBBB-CCCC"},
		}),
		finding("c", "osv-scanner", "CVE-2026-1234", "file:///abs/tmp/requirements.txt", 0, "requests", Dependency{
			Package: "requests", Version: "2.19.0", Manifest: "file:///abs/tmp/requirements.txt",
			IDs:    []string{"CVE-2026-1234", "GHSA-AAAA-BBBB-CCCC", "PYSEC-2026-1"},
			KeyIDs: []string{"CVE-2026-1234"},
		}),
	})
	if len(groups) != 1 || len(groups[0].FindingIDs) != 3 || len(groups[0].Evidence) != 3 {
		t.Fatalf("werkzeugübergreifend nicht gruppiert: %+v", groups)
	}
	want := []string{"CVE-2026-1234", "GHSA-AAAA-BBBB-CCCC", "PYSEC-2026-1"}
	if !reflect.DeepEqual(groups[0].Dependency.IDs, want) {
		t.Errorf("Alias-Union falsch: %+v, erwartet %v", groups[0].Dependency.IDs, want)
	}
	if groups[0].Dependency.Manifest != "tmp/requirements.txt" || groups[0].Dependency.Package != "requests" {
		t.Errorf("Package und Manifest nicht vom ersten Finding: %+v", groups[0].Dependency)
	}
	if !contains(groups[0].DedupeRules, "dependency") {
		t.Errorf("Dedupe-Regel dependency fehlt: %+v", groups[0].DedupeRules)
	}
}

// Gegentest 1: dieselbe CVE in zwei verschiedenen Paketen — etwa eine vendored
// Kopie. Sie darf nicht zu einer Gruppe verschmelzen.
func TestGroupFindingsGleicheCVEVerschiedenePaketeBleibenGetrennt(t *testing.T) {
	groups := GroupFindings([]Finding{
		finding("a", "pip-audit", "CVE-2026-1234", "requirements.txt", 0, "requests", Dependency{
			Package: "requests", Version: "2.19.0", Manifest: "requirements.txt",
			IDs: []string{"CVE-2026-1234"}, KeyIDs: []string{"CVE-2026-1234"},
		}),
		finding("b", "pip-audit", "CVE-2026-1234", "requirements.txt", 0, "urllib3", Dependency{
			Package: "urllib3", Version: "1.24.1", Manifest: "requirements.txt",
			IDs: []string{"CVE-2026-1234"}, KeyIDs: []string{"CVE-2026-1234"},
		}),
	})
	if len(groups) != 2 {
		t.Fatalf("gleiche CVE in zwei Paketen verschmolzen: %+v", groups)
	}
	// Bewusst in Kauf genommen: aus zwei verschiedenen Paketnamen ist nicht zu
	// lesen, ob es zwei Pakete sind oder zwei Schreibweisen desselben. Der
	// weiche Zweig stellt sie deshalb nebeneinander — verschmolzen wird nichts.
	if len(groups[0].PossibleDuplicates) != 1 || len(groups[1].PossibleDuplicates) != 1 {
		t.Errorf("abweichendes Paket nicht als possible-duplicate markiert: %+v", groups)
	}
}

// Etappe-4-Fall 1: gleiche Kennung, gleiches Paket, aber abweichende
// Versionsschreibweise. Hart trennt das die Funde; weich stehen sie nebeneinander.
func TestGroupFindingsAbweichendeVersionAlsPossibleDuplicate(t *testing.T) {
	groups := GroupFindings([]Finding{
		finding("a", "pip-audit", "CVE-2026-1234", "requirements.txt", 0, "requests", Dependency{
			Package: "requests", Version: "2.19.0", Manifest: "requirements.txt",
			IDs: []string{"CVE-2026-1234"}, KeyIDs: []string{"CVE-2026-1234"},
		}),
		finding("b", "trivy", "CVE-2026-1234", "requirements.txt", 0, "requests", Dependency{
			Package: "requests", Version: "2.19.0-r1", Manifest: "requirements.txt",
			IDs: []string{"CVE-2026-1234"}, KeyIDs: []string{"CVE-2026-1234"},
		}),
	})
	if len(groups) != 2 {
		t.Fatalf("abweichende Version hart zusammengelegt: %+v", groups)
	}
	if len(groups[0].PossibleDuplicates) != 1 || len(groups[1].PossibleDuplicates) != 1 {
		t.Fatalf("abweichende Version nicht als possible-duplicate markiert: %+v", groups)
	}
}

// Etappe-4-Fall 2: ein Werkzeug ohne package-Property hat gar keinen harten
// Schlüssel. Es darf trotzdem nicht beziehungslos danebenstehen, wenn es sich
// in den Kennungen mit einer Gruppe überschneidet. Das Manifest wird nicht
// verlangt — der weiche Zweig ist nie strenger als der harte.
func TestGroupFindingsOhneHartenSchluesselAlsPossibleDuplicate(t *testing.T) {
	groups := GroupFindings([]Finding{
		finding("a", "pip-audit", "CVE-2026-1234", "requirements.txt", 0, "requests", Dependency{
			Package: "requests", Version: "2.19.0", Manifest: "requirements.txt",
			IDs:    []string{"CVE-2026-1234", "GHSA-AAAA-BBBB-CCCC"},
			KeyIDs: []string{"CVE-2026-1234", "GHSA-AAAA-BBBB-CCCC"},
		}),
		// grype-Fall: kein package, kein version, Pfad anders geschrieben.
		finding("b", "grype", "GHSA-aaaa-bbbb-cccc-requests", "/abs/requirements.txt", 0, "requests", Dependency{
			Manifest: "/abs/requirements.txt",
			IDs:      []string{"GHSA-AAAA-BBBB-CCCC"},
			KeyIDs:   []string{"GHSA-AAAA-BBBB-CCCC"},
		}),
	})
	if len(groups) != 2 {
		t.Fatalf("Fund ohne Paketangabe hart zusammengelegt: %+v", groups)
	}
	if len(groups[0].PossibleDuplicates) != 1 || len(groups[1].PossibleDuplicates) != 1 {
		t.Fatalf("Fund ohne harten Schlüssel bleibt beziehungslos: %+v", groups)
	}
}

// Gegentest 2: zwei verschiedene CVEs im selben Paket und derselben Version,
// wobei der eine Fund die fremde Kennung im Freitext und in den Properties
// nennt. Sie dürfen weder verschmelzen noch als possible-duplicate gelten:
// Paket und Version stimmen überein, also greift auch der weiche Zweig nicht.
func TestGroupFindingsZweiCVEsImSelbenPaketBleibenGetrennt(t *testing.T) {
	groups := GroupFindings([]Finding{
		finding("a", "pip-audit", "CVE-2026-1111", "requirements.txt", 0,
			"lib 1.0: CVE-2026-1111 — siehe auch CVE-2026-2222", Dependency{
				Package: "lib", Version: "1.0", Manifest: "requirements.txt",
				IDs:    []string{"CVE-2026-1111", "CVE-2026-2222"},
				KeyIDs: []string{"CVE-2026-1111"},
			}),
		finding("b", "pip-audit", "CVE-2026-2222", "requirements.txt", 0, "lib 1.0: CVE-2026-2222", Dependency{
			Package: "lib", Version: "1.0", Manifest: "requirements.txt",
			IDs:    []string{"CVE-2026-2222"},
			KeyIDs: []string{"CVE-2026-2222"},
		}),
	})
	if len(groups) != 2 {
		t.Fatalf("zwei verschiedene CVEs verkettet: %+v", groups)
	}
	if len(groups[0].PossibleDuplicates) != 0 || len(groups[1].PossibleDuplicates) != 0 {
		t.Fatalf("verschiedene CVEs als possible-duplicate markiert: %+v", groups)
	}
}

// Steht die einzige Kennung eines Werkzeugs im Freitext, füllt extractDependency
// KeyIDs aus der breiten Menge. Ohne diesen Rückfall verlöre der Fund seinen
// harten Schlüssel ganz — schlechter als vor der Einengung.
func TestGroupFindingsRueckfallAufBreiteIDMenge(t *testing.T) {
	groups := GroupFindings([]Finding{
		finding("a", "pip-audit", "CVE-2026-1234", "requirements.txt", 0, "lib", Dependency{
			Package: "lib", Version: "1.0", Manifest: "requirements.txt",
			IDs: []string{"CVE-2026-1234"}, KeyIDs: []string{"CVE-2026-1234"},
		}),
		// Kein KeyIDs: das Werkzeug nennt die Kennung nur in der Message.
		finding("b", "freitext-tool", "generic-rule", "requirements.txt", 0, "CVE-2026-1234 in lib", Dependency{
			Package: "lib", Version: "1.0", Manifest: "requirements.txt",
			IDs: []string{"CVE-2026-1234"},
		}),
	})
	if len(groups) != 1 || len(groups[0].FindingIDs) != 2 {
		t.Fatalf("Rückfall auf die breite ID-Menge greift nicht: %+v", groups)
	}
}

// Die Einengung selbst: IDs bleibt breit, KeyIDs nimmt nur RuleID und die
// benannten Kennungsfelder. Die im Advisory-Text und in der Referenzliste
// beiläufig genannten Fremd-Kennungen dürfen nicht in den Schlüssel gelangen.
func TestExtractDependencyEngtSchluesselKennungenEin(t *testing.T) {
	result := sarifResult{
		RuleID:  "CVE-2026-1111",
		Message: sarifText{Text: "lib 1.0: CVE-2026-1111 — behoben in 1.1, siehe auch CVE-2026-2222"},
		Properties: sarifObject{
			"package":     "lib",
			"version":     "1.0",
			"manifest":    "requirements.txt",
			"id":          "PYSEC-2026-9",
			"aliases":     "CVE-2026-1111, GHSA-aaaa-bbbb-cccc",
			"fixVersions": "1.1",
			"references":  "CVE-2026-3333",
		},
	}
	rule := sarifRule{ID: "CVE-2026-1111", FullDescription: sarifText{Text: "Verwandt: CVE-2026-4444"}}
	finding := Finding{
		RuleID:          "CVE-2026-1111",
		RuleDescription: rule.FullDescription.Text,
		Message:         result.Message.Text,
	}

	dependency := extractDependency(finding, result, rule)

	wantIDs := []string{"CVE-2026-1111", "CVE-2026-2222", "CVE-2026-3333", "CVE-2026-4444", "GHSA-AAAA-BBBB-CCCC", "PYSEC-2026-9"}
	if !reflect.DeepEqual(dependency.IDs, wantIDs) {
		t.Errorf("breite ID-Menge falsch: %v, erwartet %v", dependency.IDs, wantIDs)
	}
	wantKeyIDs := []string{"CVE-2026-1111", "GHSA-AAAA-BBBB-CCCC", "PYSEC-2026-9"}
	if !reflect.DeepEqual(dependency.KeyIDs, wantKeyIDs) {
		t.Errorf("eingeengte ID-Menge falsch: %v, erwartet %v", dependency.KeyIDs, wantKeyIDs)
	}
}

// Kein benanntes Kennungsfeld und eine RuleID ohne Kennung: KeyIDs fällt auf die
// breite Menge zurück, damit das Werkzeug überhaupt einen Schlüssel behält.
func TestExtractDependencyRueckfallWennKennungNurImFreitextSteht(t *testing.T) {
	result := sarifResult{
		RuleID:     "generic-dependency-rule",
		Message:    sarifText{Text: "CVE-2026-1111 in lib 1.0"},
		Properties: sarifObject{"package": "lib", "version": "1.0"},
	}
	finding := Finding{RuleID: result.RuleID, Message: result.Message.Text}

	dependency := extractDependency(finding, result, sarifRule{})

	want := []string{"CVE-2026-1111"}
	if !reflect.DeepEqual(dependency.KeyIDs, want) {
		t.Errorf("Rückfall auf die breite Menge fehlt: %v", dependency.KeyIDs)
	}
}

func TestNormalizePathNormiertZielfrei(t *testing.T) {
	cases := map[string]string{
		"requirements.txt":                      "requirements.txt",
		"/requirements.txt":                     "requirements.txt",
		"./requirements.txt":                    "requirements.txt",
		"file:///home/x/requirements.txt":       "home/x/requirements.txt",
		"file://host/pfad/requirements.txt":     "pfad/requirements.txt",
		"FILE:///Home/X/Requirements.txt":       "home/x/requirements.txt",
		"file://requirements.txt":               "requirements.txt",
		`src\pkg\App.go`:                        "src/pkg/app.go",
		"src//pkg/./app.go":                     "src/pkg/app.go",
		"src/pkg/../app.go":                     "src/app.go",
		"../a/b.txt":                            "../a/b.txt",
		"/../a.txt":                             "a.txt",
		"":                                      "",
		"/abs/projekt/tmp/requirements.txt":     "abs/projekt/tmp/requirements.txt",
		"file:///abs/projekt/requirements.txt/": "abs/projekt/requirements.txt",
	}
	for input, want := range cases {
		if got := normalizePath(input); got != want {
			t.Errorf("normalizePath(%q) = %q, erwartet %q", input, got, want)
		}
	}
}

// normalizePath bedient nicht nur Dependencies: exactKey, sameLine und
// sameLocationToolKey nutzen dieselbe Funktion. Die drei Wirkungen werden hier
// festgeschrieben, damit die erweiterte Normierung nicht unbemerkt zurückfällt.
func TestGroupFindingsExactKeyUeberPfadschreibweisen(t *testing.T) {
	groups := GroupFindings([]Finding{
		finding("a", "semgrep", "sql-injection", "app.go", 12, "Gefahr hier", Dependency{}),
		finding("b", "gosec", "sql-injection", "/app.go", 12, "Gefahr hier", Dependency{}),
	})
	if len(groups) != 1 || len(groups[0].FindingIDs) != 2 {
		t.Fatalf("exakte Dublette über Pfadschreibweisen nicht gruppiert: %+v", groups)
	}
	if !contains(groups[0].DedupeRules, "exact-location-message") {
		t.Errorf("Dedupe-Regel fehlt: %+v", groups[0].DedupeRules)
	}
}

func TestGroupFindingsSameLocationToolUeberPfadschreibweisen(t *testing.T) {
	groups := GroupFindings([]Finding{
		findingWithJob("a", "trivy", "trivy-fs", "CVE-2026-1111", "requirements.txt", 1, "A", Dependency{}),
		findingWithJob("b", "trivy", "trivy-fs", "CVE-2026-2222", "/requirements.txt", 1, "B", Dependency{}),
	})
	if len(groups) != 1 || len(groups[0].FindingIDs) != 2 {
		t.Fatalf("Same-Location-Bundle über Pfadschreibweisen fehlt: %+v", groups)
	}
	if !contains(groups[0].DedupeRules, "same-location-tool") {
		t.Errorf("Dedupe-Regel fehlt: %+v", groups[0].DedupeRules)
	}
}

func TestGroupFindingsSameLineUeberPfadschreibweisen(t *testing.T) {
	groups := GroupFindings([]Finding{
		finding("a", "semgrep", "path-traversal-a", "file://host/src/app.go", 12, "A", Dependency{}),
		finding("b", "gosec", "path-traversal-b", "/src/app.go", 12, "B", Dependency{}),
	})
	if len(groups) != 2 {
		t.Fatalf("unsichere Lage hart zusammengelegt: %+v", groups)
	}
	if len(groups[0].PossibleDuplicates) != 1 || len(groups[1].PossibleDuplicates) != 1 {
		t.Fatalf("possible-duplicate über Pfadschreibweisen fehlt: %+v", groups)
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

// Eine Gruppe entsteht nicht zwangsläufig über den Dependency-Schlüssel. Bildet
// sie sameLocationToolKey — bei Manifest-Funden der Regelfall, weil alle Funde
// eines Werkzeugs auf dieselbe Zeile zeigen —, stehen darin verschiedene
// Schwachstellen nebeneinander. Deren Kennungen dürfen nicht in einen
// dependency-Block wandern, dessen Package und Version vom ersten Finding
// stammen.
func TestApplyRepresentativeVereinigtNurDieselbeDependency(t *testing.T) {
	groups := GroupFindings([]Finding{
		finding("a", "grype", "GHSA-AAAA-BBBB-CCCC", "requirements.txt", 1, "requests", Dependency{
			Package: "requests", Version: "2.19.0", Manifest: "requirements.txt",
			IDs: []string{"GHSA-AAAA-BBBB-CCCC"}, KeyIDs: []string{"GHSA-AAAA-BBBB-CCCC"},
		}),
		finding("b", "grype", "GHSA-DDDD-EEEE-FFFF", "requirements.txt", 1, "jinja2", Dependency{
			Package: "jinja2", Version: "2.10", Manifest: "requirements.txt",
			IDs: []string{"GHSA-DDDD-EEEE-FFFF"}, KeyIDs: []string{"GHSA-DDDD-EEEE-FFFF"},
		}),
	})
	if len(groups) != 1 || len(groups[0].FindingIDs) != 2 {
		t.Fatalf("same-location-tool gruppiert nicht wie erwartet: %+v", groups)
	}
	want := []string{"GHSA-AAAA-BBBB-CCCC"}
	if !reflect.DeepEqual(groups[0].Dependency.IDs, want) {
		t.Errorf("fremde Kennung in die Union geraten: %+v, erwartet %v", groups[0].Dependency.IDs, want)
	}
}

// Ohne Paket gibt es keinen harten Dependency-Schlüssel und damit nichts, was
// eine Vereinigung rechtfertigte. Genau so treten grype, osv-scanner und trivy
// im gemessenen Lauf auf.
func TestApplyRepresentativeOhnePaketKeineUnion(t *testing.T) {
	groups := GroupFindings([]Finding{
		finding("a", "osv-scanner", "CVE-2026-1111", "requirements.txt", 1, "CVE-2026-1111", Dependency{
			Manifest: "requirements.txt",
			IDs:      []string{"CVE-2026-1111"}, KeyIDs: []string{"CVE-2026-1111"},
		}),
		finding("b", "osv-scanner", "CVE-2026-2222", "requirements.txt", 1, "CVE-2026-2222", Dependency{
			Manifest: "requirements.txt",
			IDs:      []string{"CVE-2026-2222"}, KeyIDs: []string{"CVE-2026-2222"},
		}),
	})
	if len(groups) != 1 || len(groups[0].FindingIDs) != 2 {
		t.Fatalf("same-location-tool gruppiert nicht wie erwartet: %+v", groups)
	}
	want := []string{"CVE-2026-1111"}
	if !reflect.DeepEqual(groups[0].Dependency.IDs, want) {
		t.Errorf("Union trotz fehlendem Paket: %+v, erwartet %v", groups[0].Dependency.IDs, want)
	}
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
