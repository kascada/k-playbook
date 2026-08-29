package merge

import (
	"encoding/json"
	"fmt"
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

// grype legt Paket und Version strukturiert ab: rule.properties.purls, ein
// JSON-Array. Gekürzt aus raw/grype.sarif des Messlaufs 2026-08-27.
func TestExtractDependencyPaketAusPurl(t *testing.T) {
	result := sarifResult{
		RuleID:  "GHSA-x84v-xcm2-53pg-requests",
		Message: sarifText{Text: "A high vulnerability in python package: requests, version 2.19.0 was found at: /tmp-cve-probe/requirements.txt"},
	}
	rule := sarifRule{
		ID:               "GHSA-x84v-xcm2-53pg-requests",
		Name:             "PythonMatcherExactDirectMatch",
		ShortDescription: sarifText{Text: "GHSA-x84v-xcm2-53pg high vulnerability for requests package"},
		FullDescription:  sarifText{Text: "Insufficiently Protected Credentials in Requests"},
		Properties: sarifObject{
			"purls":             []any{"pkg:pypi/requests@2.19.0"},
			"security-severity": "7.5",
		},
	}
	finding := Finding{
		RuleID:          result.RuleID,
		RuleName:        rule.Name,
		RuleDescription: rule.FullDescription.Text,
		Message:         result.Message.Text,
		Location:        Location{URI: "/tmp-cve-probe/requirements.txt", StartLine: 1},
	}

	dependency := extractDependency(finding, result, rule)

	if dependency.Package != "requests" || dependency.Version != "2.19.0" {
		t.Fatalf("purl nicht zerlegt: %q / %q", dependency.Package, dependency.Version)
	}
	if dependency.TextPackage != "" || dependency.TextVersion != "" {
		t.Errorf("Freitextfelder trotz strukturiertem Wert gefüllt: %q / %q",
			dependency.TextPackage, dependency.TextVersion)
	}
	keys := dependencyKeys(Finding{Dependency: dependency})
	if !contains(keys, "dependency:GHSA-X84V-XCM2-53PG:requests:2.19.0") {
		t.Errorf("harter Schlüssel fehlt: %v", keys)
	}
}

// grype führt seine Go-Funde ausschließlich unter der Kennung der Go
// Vulnerability Database. Gekürzt aus raw/grype.sarif des Messlaufs 2026-08-28.
//
// Der Fund trägt damit erstmals eine Kennung — und weil das Paket schon aus dem
// purl kommt, auch einen harten Dependency-Schlüssel.
func TestExtractDependencyGoKennungAusGrype(t *testing.T) {
	result := sarifResult{
		RuleID:  "GO-2026-5024-golang.org/x/sys",
		Message: sarifText{Text: "A low vulnerability in go-module package: golang.org/x/sys, version v0.41.0 was found at: /installer/go.mod"},
	}
	rule := sarifRule{
		ID:               "GO-2026-5024-golang.org/x/sys",
		Name:             "GoModuleMatcherExactDirectMatch",
		ShortDescription: sarifText{Text: "GO-2026-5024 low vulnerability for golang.org/x/sys package"},
		FullDescription:  sarifText{Text: "NewNTUnicodeString does not check for string length overflow."},
		Properties: sarifObject{
			"purls":             []any{"pkg:golang/golang.org/x/sys@v0.41.0"},
			"security-severity": "3.3",
		},
	}
	finding := Finding{
		RuleID:          result.RuleID,
		RuleName:        rule.Name,
		RuleDescription: rule.FullDescription.Text,
		Message:         result.Message.Text,
		Location:        Location{URI: "/installer/go.mod", StartLine: 1},
	}

	dependency := extractDependency(finding, result, rule)

	if !reflect.DeepEqual(dependency.KeyIDs, []string{"GO-2026-5024"}) {
		t.Fatalf("Go-Kennung nicht in der engen Menge: %v", dependency.KeyIDs)
	}
	if dependency.Package != "golang.org/x/sys" || dependency.Version != "v0.41.0" {
		t.Fatalf("purl nicht zerlegt: %q / %q", dependency.Package, dependency.Version)
	}
	keys := dependencyKeys(Finding{Dependency: dependency})
	if !contains(keys, "dependency:GO-2026-5024:golang.org/x/sys:v0.41.0") {
		t.Errorf("harter Schlüssel fehlt: %v", keys)
	}
}

// Die vier Binaries unter dist/ melden dieselben stdlib-Schwachstellen. Über die
// Go-Kennung und das purl-Paket finden sie zu einer Gruppe je Schwachstelle
// zusammen statt zu einer je Fundort — genau die Umkehrung, um die es diesem
// Task geht. Das Manifest steht seit Task 027 nicht mehr im Schlüssel.
func TestGroupFindingsGoFundeGruppierenNachSchwachstelle(t *testing.T) {
	stdlib := func(id string, rule string, uri string) Finding {
		return finding(id, "grype", rule, uri, 1, "stdlib", Dependency{
			Package: "stdlib", Version: "1.26.4", Manifest: uri,
			IDs: []string{"GO-2026-5972"}, KeyIDs: []string{"GO-2026-5972"},
		})
	}
	groups := GroupFindings([]Finding{
		stdlib("a", "GO-2026-5972-stdlib", "/dist/k-playbook-linux-amd64"),
		stdlib("b", "GO-2026-5972-stdlib", "/dist/k-playbook-darwin-arm64"),
	})
	if len(groups) != 1 || len(groups[0].FindingIDs) != 2 {
		t.Fatalf("dieselbe Go-Schwachstelle in zwei Binaries nicht zusammengeführt: %+v", groups)
	}
	if !strings.HasPrefix(groups[0].StableID, "scan-cve-go-2026-5972-") {
		t.Errorf("Präfix folgt der Go-Kennung nicht: %q", groups[0].StableID)
	}
}

// Gegentest zur Go-Kennung: eine Zeichenfolge aus gewöhnlichem Text ist keine
// Kennung. Die Go-Datenbank vergibt vierstellige, nullgefüllte Nummern; ohne
// diese Untergrenze läse das case-insensitive Muster jede Datumsangabe der Form
// go-2026-08 als Kennung und verkettete darüber fremde Funde.
func TestExtractIDsLiestGewoehnlichenTextNichtAlsKennung(t *testing.T) {
	text := "Siehe go-2026-08 im Änderungsprotokoll, dazu GO-26-1234 und den Branch go-2026-1 von gestern."
	if ids := extractIDs(text); len(ids) != 0 {
		t.Errorf("gewöhnlicher Text als Kennung gelesen: %v", ids)
	}
	if ids := extractIDs("Behoben laut GO-2026-5024."); !reflect.DeepEqual(ids, []string{"GO-2026-5024"}) {
		t.Errorf("echte Go-Kennung nicht gelesen: %v", ids)
	}
}

// osv-scanner nennt Paket und Version ausschließlich in der Meldung. Gekürzt
// aus raw/osv-scanner.sarif des Messlaufs 2026-08-27.
func TestExtractDependencyPaketNurImFreitextOsvScanner(t *testing.T) {
	result := sarifResult{
		RuleID:  "CVE-2018-18074",
		Message: sarifText{Text: "Package 'requests@2.19.0' is vulnerable to 'CVE-2018-18074' (also known as 'PYSEC-2018-28', 'GHSA-x84v-xcm2-53pg')."},
	}
	rule := sarifRule{
		ID:         "CVE-2018-18074",
		Properties: sarifObject{"security-severity": "7.5"},
	}
	finding := Finding{
		RuleID:   result.RuleID,
		Message:  result.Message.Text,
		Location: Location{URI: "file:///home/kleist/dev/k-playbook/tmp-cve-probe/requirements.txt"},
	}

	dependency := extractDependency(finding, result, rule)

	if dependency.Package != "" || dependency.Version != "" {
		t.Errorf("Freitextwert im engen Feld: %q / %q", dependency.Package, dependency.Version)
	}
	if dependency.TextPackage != "requests" || dependency.TextVersion != "2.19.0" {
		t.Errorf("Anzeige-Rückfall falsch: %q / %q", dependency.TextPackage, dependency.TextVersion)
	}
	if keys := dependencyKeys(Finding{Dependency: dependency}); len(keys) != 0 {
		t.Errorf("harter Schlüssel ohne strukturierten Wert: %v", keys)
	}
}

// trivy nennt neben der installierten auch die behobene Version. Der Rückfall
// muss die installierte lesen. Gekürzt aus raw/trivy-fs.sarif des Messlaufs
// 2026-08-27.
func TestExtractDependencyPaketNurImFreitextTrivy(t *testing.T) {
	result := sarifResult{
		RuleID:  "CVE-2018-18074",
		Message: sarifText{Text: "Package: requests\nInstalled Version: 2.19.0\nVulnerability CVE-2018-18074\nSeverity: HIGH\nFixed Version: 2.20.0\nLink: [CVE-2018-18074](https://avd.aquasec.com/nvd/cve-2018-18074)"},
	}
	rule := sarifRule{
		ID:   "CVE-2018-18074",
		Name: "LanguageSpecificPackageVulnerability",
		Properties: sarifObject{
			"precision":         "very-high",
			"security-severity": "7.5",
			"tags":              []any{"vulnerability", "security", "HIGH"},
		},
	}
	finding := Finding{
		RuleID:   result.RuleID,
		RuleName: rule.Name,
		Message:  result.Message.Text,
		Location: Location{URI: "tmp-cve-probe/requirements.txt", StartLine: 1},
	}

	dependency := extractDependency(finding, result, rule)

	if dependency.Package != "" || dependency.Version != "" {
		t.Errorf("Freitextwert im engen Feld: %q / %q", dependency.Package, dependency.Version)
	}
	if dependency.TextPackage != "requests" {
		t.Errorf("Paket aus der Meldung falsch: %q", dependency.TextPackage)
	}
	if dependency.TextVersion != "2.19.0" {
		t.Errorf("installierte Version erwartet, gelesen wurde %q", dependency.TextVersion)
	}
	if keys := dependencyKeys(Finding{Dependency: dependency}); len(keys) != 0 {
		t.Errorf("harter Schlüssel ohne strukturierten Wert: %v", keys)
	}
}

// pip-audit trägt die benannten Properties und braucht keinen Rückfall; die
// Freitextfelder bleiben leer.
func TestExtractDependencyBenanntePropertyBleibtVorrangig(t *testing.T) {
	result := sarifResult{
		RuleID:  "CVE-2018-18074",
		Message: sarifText{Text: "requests 2.19.0: CVE-2018-18074 — behoben in 2.20.0"},
		Properties: sarifObject{
			"package": "requests", "version": "2.19.0",
			"manifest": "tmp-cve-probe/requirements.txt",
			"id":       "PYSEC-2018-28", "aliases": "CVE-2018-18074, GHSA-x84v-xcm2-53pg",
		},
	}
	finding := Finding{RuleID: result.RuleID, Message: result.Message.Text}

	dependency := extractDependency(finding, result, sarifRule{})

	if dependency.Package != "requests" || dependency.Version != "2.19.0" {
		t.Fatalf("benannte Property nicht gelesen: %q / %q", dependency.Package, dependency.Version)
	}
	if dependency.TextPackage != "" || dependency.TextVersion != "" {
		t.Errorf("Freitextfelder trotz benannter Property gefüllt: %q / %q",
			dependency.TextPackage, dependency.TextVersion)
	}
}

// Pflicht-Gegentest: ein aus Freitext gelesenes Paket darf nicht in den harten
// Schlüssel. Beide Funde nennen dieselbe Kennung und dasselbe Paket in
// derselben Version — der eine strukturiert, der andere nur in der Meldung.
// Zusammengeführt werden dürfen sie trotzdem nicht; der weiche Zweig nimmt sie
// auf.
func TestGroupFindingsFreitextPaketKommtNichtInDenHartenSchluessel(t *testing.T) {
	strukturiert := sarifResult{
		RuleID:     "CVE-2026-1111",
		Message:    sarifText{Text: "lib 1.0: CVE-2026-1111"},
		Properties: sarifObject{"package": "lib", "version": "1.0"},
	}
	freitext := sarifResult{
		RuleID:  "CVE-2026-1111",
		Message: sarifText{Text: "Package: lib\nInstalled Version: 1.0\nVulnerability CVE-2026-1111"},
	}
	links := Finding{
		ID: "a", Evidence: Evidence{Tool: "pip-audit", Job: "pip-audit"},
		RuleID: strukturiert.RuleID, Message: strukturiert.Message.Text,
	}
	links.Dependency = extractDependency(links, strukturiert, sarifRule{})
	rechts := Finding{
		ID: "b", Evidence: Evidence{Tool: "trivy", Job: "trivy-fs"},
		RuleID: freitext.RuleID, Message: freitext.Message.Text,
	}
	rechts.Dependency = extractDependency(rechts, freitext, sarifRule{})

	if rechts.Dependency.TextPackage != "lib" {
		t.Fatalf("Vorbedingung: Freitextpaket nicht gelesen, %q", rechts.Dependency.TextPackage)
	}
	if keys := dependencyKeys(rechts); len(keys) != 0 {
		t.Fatalf("Freitextpaket im harten Schlüssel: %v", keys)
	}

	groups := GroupFindings([]Finding{links, rechts})
	if len(groups) != 2 {
		t.Fatalf("erwartet zwei Gruppen, bekommen %d", len(groups))
	}
	if len(groups[0].PossibleDuplicates) == 0 || len(groups[1].PossibleDuplicates) == 0 {
		t.Errorf("weicher Zweig greift nicht: %v / %v",
			groups[0].PossibleDuplicates, groups[1].PossibleDuplicates)
	}
}

// Der Fall, für den der purl-Rückfall gebaut ist: grype nennt nur die GHSA und
// legt das Paket im purl ab, pip-audit nennt alle drei Kennungen und die
// benannten Properties. Sie gehören in eine Gruppe.
func TestGroupFindingsPurlUndPropertyFindenZusammen(t *testing.T) {
	grypeResult := sarifResult{
		RuleID:  "GHSA-x84v-xcm2-53pg-requests",
		Message: sarifText{Text: "A high vulnerability in python package: requests, version 2.19.0 was found at: /tmp-cve-probe/requirements.txt"},
	}
	grypeRule := sarifRule{
		ID:         "GHSA-x84v-xcm2-53pg-requests",
		Properties: sarifObject{"purls": []any{"pkg:pypi/requests@2.19.0"}},
	}
	grype := Finding{
		ID: "grype", Evidence: Evidence{Tool: "grype", Job: "grype"},
		RuleID:   grypeResult.RuleID,
		Message:  grypeResult.Message.Text,
		Location: Location{URI: "/tmp-cve-probe/requirements.txt", StartLine: 1},
	}
	grype.Dependency = extractDependency(grype, grypeResult, grypeRule)

	pipResult := sarifResult{
		RuleID:  "CVE-2018-18074",
		Message: sarifText{Text: "requests 2.19.0: CVE-2018-18074"},
		Properties: sarifObject{
			"package": "requests", "version": "2.19.0",
			"id": "PYSEC-2018-28", "aliases": "CVE-2018-18074, GHSA-x84v-xcm2-53pg",
		},
	}
	pipAudit := Finding{
		ID: "pip-audit", Evidence: Evidence{Tool: "pip-audit", Job: "pip-audit"},
		RuleID:   pipResult.RuleID,
		Message:  pipResult.Message.Text,
		Location: Location{URI: "tmp-cve-probe/requirements.txt"},
	}
	pipAudit.Dependency = extractDependency(pipAudit, pipResult, sarifRule{})

	groups := GroupFindings([]Finding{grype, pipAudit})
	if len(groups) != 1 {
		t.Fatalf("erwartet eine Gruppe, bekommen %d", len(groups))
	}
	if len(groups[0].FindingIDs) != 2 {
		t.Errorf("Gruppe unvollständig: %v", groups[0].FindingIDs)
	}
}

func TestParsePurlZerlegtNameUndVersion(t *testing.T) {
	cases := []struct {
		purl    string
		name    string
		version string
	}{
		{"pkg:pypi/requests@2.19.0", "requests", "2.19.0"},
		{"pkg:pypi/Requests@2.19.0", "requests", "2.19.0"},
		{"pkg:golang/golang.org/x/sys@v0.41.0", "golang.org/x/sys", "v0.41.0"},
		{"pkg:golang/stdlib@1.26.4", "stdlib", "1.26.4"},
		{"pkg:deb/debian/curl@7.50.3-1?arch=i386", "debian/curl", "7.50.3-1"},
		{"pkg:pypi/requests", "requests", ""},
		{"requests@2.19.0", "", ""},
		{"", "", ""},
	}
	for _, testCase := range cases {
		name, version := parsePurl(testCase.purl)
		if name != testCase.name || version != testCase.version {
			t.Errorf("%q → %q / %q, erwartet %q / %q",
				testCase.purl, name, version, testCase.name, testCase.version)
		}
	}
}

// stringProperty fiele für ein Array auf fmt.Sprint zurück und lieferte den
// Wert samt Klammern; stringListProperty muss die Einträge einzeln herausgeben.
func TestStringListPropertyLiestArrayUndString(t *testing.T) {
	properties := sarifObject{
		"purls": []any{"pkg:pypi/requests@2.19.0", "pkg:pypi/urllib3@1.24.1"},
		"purl":  "pkg:pypi/flask@0.12.2",
	}
	got := stringListProperty(properties, "purl", "purls")
	want := []string{"pkg:pypi/flask@0.12.2", "pkg:pypi/requests@2.19.0", "pkg:pypi/urllib3@1.24.1"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Liste falsch gelesen: %v, erwartet %v", got, want)
	}
	if value := stringProperty(properties, "purls"); !strings.HasPrefix(value, "[") {
		t.Errorf("Vorbedingung entfallen: stringProperty liefert %q", value)
	}
}

// Die Pfadnormierung selbst wird in internal/pathnorm getestet; sie ist seit
// Task 029 dort und wird mit knowndecisions geteilt. Hier stehen ihre
// **Wirkungen** im Merge: exactKey, sameLine und sameLocationToolKey rufen sie,
// und die drei Wirkungen werden festgeschrieben, damit die erweiterte Normierung
// nicht unbemerkt zurückfällt.
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

// Zwei verschiedene CVEs desselben Werkzeugs auf derselben Manifest-Zeile:
// same-location-tool darf sie nicht mehr bündeln, sobald eine Dependency
// erkannt ist. Der weiche Zweig darf sie verbinden, die harte Gruppe nicht.
func TestGroupFindingsSameLocationToolNichtFuerDependencyFunde(t *testing.T) {
	links := findingWithJob("a", "grype", "grype", "GHSA-aaaa-bbbb-cccc", "requirements.txt", 1, "A",
		Dependency{Package: "requests", Version: "2.19.0", IDs: []string{"GHSA-AAAA-BBBB-CCCC"}, KeyIDs: []string{"GHSA-AAAA-BBBB-CCCC"}})
	rechts := findingWithJob("b", "grype", "grype", "GHSA-dddd-eeee-ffff", "requirements.txt", 1, "B",
		Dependency{Package: "jinja2", Version: "2.10", IDs: []string{"GHSA-DDDD-EEEE-FFFF"}, KeyIDs: []string{"GHSA-DDDD-EEEE-FFFF"}})

	if key := sameLocationToolKey(links); key != "" {
		t.Errorf("same-location-tool greift trotz erkannter Dependency: %q", key)
	}
	groups := GroupFindings([]Finding{links, rechts})
	if len(groups) != 2 {
		t.Fatalf("verschiedene Befunde auf derselben Zeile zusammengelegt: %+v", groups)
	}
	for _, group := range groups {
		if contains(group.DedupeRules, "same-location-tool") {
			t.Errorf("Regel bei Dependency-Funden noch gemeldet: %v", group.DedupeRules)
		}
	}
}

// Auch der reine Anzeige-Rückfall zählt als erkannte Dependency: trivy und
// osv-scanner haben kein strukturiertes Paket, melden aber ebenfalls jeden Fund
// auf derselben Manifest-Zeile.
func TestSameLocationToolKeyEntfaelltAuchBeiFreitextDependency(t *testing.T) {
	finding := findingWithJob("a", "trivy", "trivy-fs", "CVE-2026-1111", "requirements.txt", 1, "A",
		Dependency{IDs: []string{"CVE-2026-1111"}, TextPackage: "requests", TextVersion: "2.19.0"})
	if key := sameLocationToolKey(finding); key != "" {
		t.Errorf("same-location-tool greift trotz Freitext-Dependency: %q", key)
	}
}

// Der Ausschluss aus Task 034: grypes partialFingerprints.primaryLocationLineHash
// ist je Paket und Datei gleich, nicht je Schwachstelle. Für Dependency-Funde
// bildet er deshalb keinen Schlüssel mehr — grype steht nicht auf der
// Zulassungsliste namingFingerprints.
//
// Die Werte stammen aus dem Lauf 2026-08-28: `1249092561d58ae3…` trug dort alle
// sechs jinja2-Funde von grype.
func TestGroupFindingsFingerprintNichtFuerDependencyFundeOhneZulassung(t *testing.T) {
	links := finding("a", "grype", "GHSA-462w-v97r-4m45-jinja2", "/tmp-cve-probe/requirements.txt", 1, "A",
		Dependency{Package: "jinja2", Version: "2.10", IDs: []string{"GHSA-462W-V97R-4M45"}, KeyIDs: []string{"GHSA-462W-V97R-4M45"}})
	links.PartialFingerprints = map[string]string{"primaryLocationLineHash": "1249092561d58ae3"}
	rechts := finding("b", "grype", "GHSA-cpwx-vrp4-4pq7-jinja2", "/tmp-cve-probe/requirements.txt", 1, "B",
		Dependency{Package: "jinja2", Version: "2.10", IDs: []string{"GHSA-CPWX-VRP4-4PQ7"}, KeyIDs: []string{"GHSA-CPWX-VRP4-4PQ7"}})
	rechts.PartialFingerprints = map[string]string{"primaryLocationLineHash": "1249092561d58ae3"}

	if keys := fingerprintKeys(links); len(keys) != 0 {
		t.Errorf("Fingerprint bildet trotz erkannter Dependency einen Schlüssel: %v", keys)
	}
	groups := GroupFindings([]Finding{links, rechts})
	if len(groups) != 2 {
		t.Fatalf("zwei Schwachstellen desselben Pakets über den Ortshash zusammengelegt: %+v", groups)
	}
	for _, group := range groups {
		if contains(group.DedupeRules, "fingerprint") {
			t.Errorf("Regel bei Dependency-Funden noch gemeldet: %v", group.DedupeRules)
		}
	}
}

// Die Gegenprobe: osv-scanner vergibt denselben Namen, aber je Schwachstelle
// einen eigenen Wert. Es steht auf der Zulassungsliste, und seine zwei
// deckungsgleichen Meldungen desselben CVE finden weiter zusammen — genau die
// Gruppierung, die eine pauschale Regel verloren hätte.
func TestGroupFindingsFingerprintZulassungslisteGruppiertWeiter(t *testing.T) {
	links := finding("a", "osv-scanner", "CVE-2018-18074", "requirements.txt", 0, "A",
		Dependency{IDs: []string{"CVE-2018-18074"}, KeyIDs: []string{"CVE-2018-18074"}, TextPackage: "requests"})
	links.PartialFingerprints = map[string]string{"primaryLocationLineHash": "3949d8b838308400"}
	rechts := finding("b", "osv-scanner", "CVE-2018-18074", "requirements.txt", 0, "B",
		Dependency{IDs: []string{"CVE-2018-18074"}, KeyIDs: []string{"CVE-2018-18074"}, TextPackage: "requests"})
	rechts.PartialFingerprints = map[string]string{"primaryLocationLineHash": "3949d8b838308400"}

	if keys := fingerprintKeys(links); len(keys) != 1 {
		t.Fatalf("zugelassener Fingerprint bildet keinen Schlüssel: %v", keys)
	}
	groups := GroupFindings([]Finding{links, rechts})
	if len(groups) != 1 || len(groups[0].FindingIDs) != 2 {
		t.Fatalf("zugelassener Fingerprint gruppiert nicht mehr: %+v", groups)
	}
}

// Für Funde ohne erkannte Dependency bleibt fingerprintKeys unverändert; das
// ist hier nicht Gegenstand.
func TestFingerprintKeysOhneDependencyUnveraendert(t *testing.T) {
	plain := finding("a", "semgrep", "go.security", "main.go", 7, "A", Dependency{})
	plain.Fingerprints = map[string]string{"match": "fp1"}
	keys := fingerprintKeys(plain)
	if len(keys) != 1 || keys[0] != "fingerprint:semgrep:match:fp1" {
		t.Errorf("Fingerprint für Nicht-Dependency-Fund verändert: %v", keys)
	}
}

// Ein Name, den erst ein Werkzeug-Update mitbringt, steht nicht auf der
// Zulassungsliste und stellt die Ortsgruppierung nicht wieder her. Das ist der
// Grund, aus dem die Liste eine Zulassungs- und keine Sperrliste ist.
func TestNamesFindingKenntNurBelegtePaare(t *testing.T) {
	if !namesFinding("osv-scanner", "primaryLocationLineHash") {
		t.Error("belegtes Paar nicht zugelassen")
	}
	if !namesFinding("OSV-Scanner", "PRIMARYLOCATIONLINEHASH") {
		t.Error("Vergleich ist nicht schreibungsunabhängig")
	}
	if namesFinding("grype", "primaryLocationLineHash") {
		t.Error("derselbe Name eines anderen Werkzeugs zugelassen")
	}
	if namesFinding("osv-scanner", "neuerHashAusEinemUpdate") {
		t.Error("unbekannter Name zugelassen")
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
	writeText(t, filepath.Join(projectDir, "known-decisions.md"), "## kd-old-tree\n\n```yaml\nid: kd-old-tree\ncategory: wontfix\nmatch:\n  - pathGlob: _old/**\n```\n\nBegründung.\n")
	before, err := os.ReadFile(rawPath)
	if err != nil {
		t.Fatalf("Raw lesen: %v", err)
	}

	result, output, err := Run(Options{ProjectDir: projectDir, RunName: "2026-08-19", RunDir: runDir, LocalDir: projectDir, Now: fixedNow})
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
// Eine Gruppe kann verschiedene Dependencies enthalten, sobald sie nicht über
// den Dependency-Schlüssel entstanden ist. Seit Task 028 bündelt
// same-location-tool Dependency-Funde nicht mehr; den Fall stellt hier ein
// gemeinsamer Fingerprint desselben Werkzeugs her.
//
// Seit Task 034 muss es dafür ein Paar aus der Zulassungsliste
// namingFingerprints sein — nur die bilden für Dependency-Funde überhaupt noch
// einen Schlüssel.
func TestApplyRepresentativeVereinigtNurDieselbeDependency(t *testing.T) {
	links := finding("a", "osv-scanner", "GHSA-AAAA-BBBB-CCCC", "requirements.txt", 1, "requests", Dependency{
		Package: "requests", Version: "2.19.0", Manifest: "requirements.txt",
		IDs: []string{"GHSA-AAAA-BBBB-CCCC"}, KeyIDs: []string{"GHSA-AAAA-BBBB-CCCC"},
	})
	links.Fingerprints = map[string]string{"primaryLocationLineHash": "gemeinsam"}
	rechts := finding("b", "osv-scanner", "GHSA-DDDD-EEEE-FFFF", "requirements.txt", 1, "jinja2", Dependency{
		Package: "jinja2", Version: "2.10", Manifest: "requirements.txt",
		IDs: []string{"GHSA-DDDD-EEEE-FFFF"}, KeyIDs: []string{"GHSA-DDDD-EEEE-FFFF"},
	})
	rechts.Fingerprints = map[string]string{"primaryLocationLineHash": "gemeinsam"}

	groups := GroupFindings([]Finding{links, rechts})
	if len(groups) != 1 || len(groups[0].FindingIDs) != 2 {
		t.Fatalf("Fingerprint gruppiert nicht wie erwartet: %+v", groups)
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
	links := finding("a", "osv-scanner", "CVE-2026-1111", "requirements.txt", 1, "CVE-2026-1111", Dependency{
		Manifest: "requirements.txt",
		IDs:      []string{"CVE-2026-1111"}, KeyIDs: []string{"CVE-2026-1111"},
	})
	links.Fingerprints = map[string]string{"primaryLocationLineHash": "gemeinsam"}
	rechts := finding("b", "osv-scanner", "CVE-2026-2222", "requirements.txt", 1, "CVE-2026-2222", Dependency{
		Manifest: "requirements.txt",
		IDs:      []string{"CVE-2026-2222"}, KeyIDs: []string{"CVE-2026-2222"},
	})
	rechts.Fingerprints = map[string]string{"primaryLocationLineHash": "gemeinsam"}

	groups := GroupFindings([]Finding{links, rechts})
	if len(groups) != 1 || len(groups[0].FindingIDs) != 2 {
		t.Fatalf("Fingerprint gruppiert nicht wie erwartet: %+v", groups)
	}
	want := []string{"CVE-2026-1111"}
	if !reflect.DeepEqual(groups[0].Dependency.IDs, want) {
		t.Errorf("Union trotz fehlendem Paket: %+v, erwartet %v", groups[0].Dependency.IDs, want)
	}
}

// TestStableIDHaeltBeiBreitererAliasmenge ist die Zusage aus dem Intent von
// Task 029: dieselbe Gruppe mit breiterer Aliasmenge behält Schlüssel, Präfix
// und class.
//
// Der Zuschnitt ist eng und mit Absicht: die zusätzliche Kennung steht **nur**
// im Advisory-Freitext und damit in IDs, nicht in KeyIDs — sie käme aus keinem
// der dependencyKeyIDProperties-Felder. Nur dieser Fall ist über die Einengung
// auf KeyIDs überhaupt lösbar. Nennt ein Werkzeug die zusätzliche Kennung in
// einem benannten Alias-Feld, zählt sie zur engen Menge und die ID verschiebt
// sich weiterhin; das ist eine bewusst offene Restinstabilität, siehe
// docs/review-runs.md.
func TestStableIDHaeltBeiBreitererAliasmenge(t *testing.T) {
	schmal := Dependency{
		Package:  "requests",
		Version:  "2.19.0",
		Manifest: "requirements.txt",
		IDs:      []string{"CVE-2018-18074"},
		KeyIDs:   []string{"CVE-2018-18074"},
	}
	// Dasselbe Werkzeug, derselbe Befund — der Advisory-Text nennt zusätzlich
	// eine Fremd-Kennung. KeyIDs bleibt unberührt.
	breit := schmal
	breit.IDs = []string{"CVE-2018-18074", "CVE-2099-99999"}

	pruefeGleicheStabileGruppe(t, schmal, breit)
}

// TestStableIDHaeltBeiAlphabetischFruehererFremdkennung deckt den Präfix-Pfad
// ab: dependencyPrimaryID sortiert und nimmt ids[0], also entscheidet die
// alphabetisch erste Kennung über scan-cve-<kennung>-. Läuft die Funktion über
// die breite Menge, kippt das Präfix, sobald eine beiläufig genannte Kennung
// vorne einsortiert. Nur dieser Fall prüft das wirklich.
func TestStableIDHaeltBeiAlphabetischFruehererFremdkennung(t *testing.T) {
	schmal := Dependency{
		Package:  "requests",
		Version:  "2.19.0",
		Manifest: "requirements.txt",
		IDs:      []string{"GHSA-x84v-xcm2-53pg"},
		KeyIDs:   []string{"GHSA-x84v-xcm2-53pg"},
	}
	breit := schmal
	// CVE-… sortiert vor GHSA-… und wäre über die breite Menge die Primärkennung.
	breit.IDs = []string{"CVE-2000-0001", "GHSA-x84v-xcm2-53pg"}

	prefix, _ := stablePrefixAndKey([]Finding{dependencyFinding("a", schmal)})
	if prefix != "scan-cve-ghsa-x84v-xcm2-53pg-" {
		t.Fatalf("Vorbedingung entfallen: Präfix ist %q", prefix)
	}
	pruefeGleicheStabileGruppe(t, schmal, breit)
}

// TestStableIDNutztBreiteMengeOhneEngeMenge hält den Rückfall fest: ohne
// KeyIDs bleibt es bei IDs, sonst hätte eine Gruppe, deren Werkzeug seine
// Kennung nur im Freitext nennt, gar keinen Dependency-Schlüssel mehr — und
// verlöre Präfix und Klasse.
func TestStableIDNutztBreiteMengeOhneEngeMenge(t *testing.T) {
	ohne := Dependency{
		Package:  "requests",
		Version:  "2.19.0",
		Manifest: "requirements.txt",
		IDs:      []string{"CVE-2018-18074"},
	}
	findings := []Finding{dependencyFinding("a", ohne)}
	if class := stableClass(findings); class != "dependency" {
		t.Fatalf("class = %q, erwartet dependency", class)
	}
	prefix, _ := stablePrefixAndKey(findings)
	if prefix != "scan-cve-cve-2018-18074-" {
		t.Fatalf("Präfix = %q, erwartet scan-cve-cve-2018-18074-", prefix)
	}
}

func pruefeGleicheStabileGruppe(t *testing.T, schmal Dependency, breit Dependency) {
	t.Helper()
	schmalFindings := []Finding{dependencyFinding("a", schmal)}
	breitFindings := []Finding{dependencyFinding("a", breit)}

	if got, want := stableClass(breitFindings), stableClass(schmalFindings); got != want {
		t.Errorf("class verschoben: %q statt %q", got, want)
	}
	schmalPrefix, schmalKey := stablePrefixAndKey(schmalFindings)
	breitPrefix, breitKey := stablePrefixAndKey(breitFindings)
	if breitPrefix != schmalPrefix {
		t.Errorf("Präfix verschoben: %q statt %q", breitPrefix, schmalPrefix)
	}
	if breitKey != schmalKey {
		t.Errorf("stableKey verschoben:\n breit:  %s\n schmal: %s", breitKey, schmalKey)
	}
}

func dependencyFinding(id string, dependency Dependency) Finding {
	return Finding{
		ID:         id,
		Evidence:   Evidence{Tool: "pip-audit", Job: "pip-audit"},
		RuleID:     "PYSEC-2018-28",
		Level:      "warning",
		Message:    "requests 2.19.0 ist verwundbar",
		Location:   Location{URI: "requirements.txt", StartLine: 1},
		Dependency: dependency,
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

// --- KI-Evidence: Gruppierung und stabile IDs ---------------------------------

// aiFinding ist ein Fund aus einem Evidence-Eintrag. Werkzeug und Job tragen
// beide den Eintragsnamen — so schreibt der Melde-Schritt den Job.
func aiFinding(id string, entry string, rule string, uri string, line int, message string) Finding {
	item := findingWithJob(id, entry, entry, rule, uri, line, message, Dependency{})
	item.Mode = review.ModeEvidence
	return item
}

// sarifFixture baut ein SARIF-Dokument. Bei einem Evidence-Eintrag ist der
// Werkzeugname der Eintragsname — so verlangt es review.CheckEvidenceSARIF, und
// so trägt der Merge ihn als evidence.tool ein.
func sarifFixture(tool string, results ...string) string {
	return `{"version": "2.1.0", "runs": [{"tool": {"driver": {"name": "` + tool + `"}}, "results": [` +
		strings.Join(results, ", ") + `]}]}`
}

func sarifResultFixture(ruleID string, level string, uri string, line int, message string) string {
	return fmt.Sprintf(`{"ruleId": %q, "level": %q, "message": {"text": %q}, `+
		`"locations": [{"physicalLocation": {"artifactLocation": {"uri": %q}, "region": {"startLine": %d}}}]}`,
		ruleID, level, message, uri, line)
}

// fixtureEntry ist ein Eintrag eines gebauten Laufordners: Name, Art,
// Betriebsart und das SARIF, das sein Job hinterlässt.
type fixtureEntry struct {
	name  string
	kind  review.Kind
	mode  review.Mode
	sarif string
}

func toolEntry(name string, sarif string) fixtureEntry {
	return fixtureEntry{name: name, kind: review.KindTool, sarif: sarif}
}

func evidenceEntry(name string, sarif string) fixtureEntry {
	return fixtureEntry{name: name, kind: review.KindAI, mode: review.ModeEvidence, sarif: sarif}
}

// runDirWithEntries legt einen fertigen Laufordner an: run.json mit der
// Auswahl, je Eintrag die Datei unter entries/ mit einem abgeschlossenen Job und
// das zugehörige SARIF unter raw/.
func runDirWithEntries(t *testing.T, projectDir string, name string, entries ...fixtureEntry) string {
	t.Helper()
	runDir := filepath.Join(projectDir, review.ResultsDirName, name)
	selected := make([]review.Entry, 0, len(entries))
	for _, item := range entries {
		entry := review.Entry{Name: item.name, Kind: item.kind, State: review.StateStart, Mode: item.mode}
		if item.mode == review.ModeEvidence {
			entry.Scope = &review.Scope{Paths: []string{"installer/**"}}
		}
		selected = append(selected, entry)
	}
	writeJSON(t, filepath.Join(runDir, review.RunFileName), review.Run{
		SchemaVersion: review.SchemaVersion,
		Created:       "2026-08-25T12:00:00Z",
		State:         review.StateCreated,
		Languages:     []string{"go"},
		Entries:       selected,
	})
	for _, item := range entries {
		sarifPath := review.EvidenceSARIFPath(item.name)
		writeJSON(t, review.EntryFile(runDir, item.name), review.EntryStatus{
			SchemaVersion: review.EntrySchemaVersion,
			Name:          item.name,
			Kind:          item.kind,
			State:         review.StateDone,
			Jobs:          []review.JobStatus{{Job: item.name, State: review.StateDone, SARIF: sarifPath}},
		})
		writeText(t, filepath.Join(runDir, filepath.FromSlash(sarifPath)), item.sarif)
	}
	return runDir
}

func evidenceRunDir(t *testing.T, projectDir string, name string, entry string, sarif string) string {
	t.Helper()
	return runDirWithEntries(t, projectDir, name, evidenceEntry(entry, sarif))
}

func TestAIPathRuleKeyNurFuerEvidence(t *testing.T) {
	evidence := aiFinding("a", "review-tech", "tech.Altlast", "installer/App.go", 12, "Text")
	if key := aiPathRuleKey(evidence); key != "ai:review-tech:installer/app.go:tech.altlast" {
		t.Fatalf("KI-Schlüssel falsch: %q", key)
	}

	perspective := evidence
	perspective.Mode = review.ModePerspective
	if key := aiPathRuleKey(perspective); key != "" {
		t.Errorf("Perspektive bekommt einen KI-Schlüssel: %q", key)
	}
	scanner := finding("b", "semgrep", "rule", "installer/app.go", 12, "Text", Dependency{})
	if key := aiPathRuleKey(scanner); key != "" {
		t.Errorf("Scanner-Fund bekommt einen KI-Schlüssel: %q", key)
	}
}

// Der Schlüssel ai:<tool>:<pfad>:<ruleId> hält die Gruppierung mit der
// Schlüsselklasse zusammen: class=ai kennt keine Zeile, zwei Funde derselben
// Rule-ID in derselben Datei hätten sonst denselben stabilen Schlüssel.
func TestGroupFindingsKIEvidenceBuendeltRegelJeDatei(t *testing.T) {
	groups := GroupFindings([]Finding{
		aiFinding("a", "review-tech", "tech.altlast", "installer/app.go", 12, "Erste Stelle"),
		aiFinding("b", "review-tech", "tech.altlast", "installer/app.go", 240, "Zweite Stelle"),
		aiFinding("c", "review-tech", "tech.altlast", "installer/other.go", 3, "Andere Datei"),
		aiFinding("d", "review-tech", "tech.kopplung", "installer/app.go", 12, "Andere Regel"),
	})
	if len(groups) != 3 {
		t.Fatalf("%d Gruppen, erwartet 3: %+v", len(groups), groups)
	}
	if len(groups[0].FindingIDs) != 2 || !contains(groups[0].FindingIDs, "a") || !contains(groups[0].FindingIDs, "b") {
		t.Fatalf("zwei Instanzen derselben Regel in derselben Datei nicht gebündelt: %+v", groups[0])
	}
	if !contains(groups[0].DedupeRules, "ai-path-rule") {
		t.Errorf("Dedupe-Regel fehlt: %+v", groups[0].DedupeRules)
	}
	// „a" und „d" stehen in derselben Zeile. Für einen Scanner wäre das eine
	// Gruppe; für Evidence darf es keine sein, sonst hinge die Gruppen-ID doch
	// wieder an der Zeile.
	if contains(groups[0].DedupeRules, "same-location-tool") {
		t.Errorf("Zeilen-Schlüssel greift für Evidence: %+v", groups[0].DedupeRules)
	}
	if len(groups[2].FindingIDs) != 1 || groups[2].FindingIDs[0] != "d" {
		t.Fatalf("zweite Rule-ID in derselben Zeile nicht getrennt: %+v", groups[2])
	}
	seen := map[string]bool{}
	for _, group := range groups {
		if seen[group.StableID] {
			t.Fatalf("kollidierende stableId: %s", group.StableID)
		}
		seen[group.StableID] = true
		if !strings.HasPrefix(group.StableID, "ai-review-tech-") {
			t.Errorf("KI-Präfix fehlt: %s", group.StableID)
		}
		if !strings.HasPrefix(group.StableKey, "class=ai\n") {
			t.Errorf("Klasse falsch: %q", group.StableKey)
		}
	}
}

// Gemischte Gruppen bleiben location. Setzte ein einzelner KI-Fund die Klasse,
// wechselte eine bestehende Scanner-Gruppe beim Hinzukommen dieses Funds von
// scan- auf ai- und verlöre Ort und Meldung aus ihrem Schlüssel.
func TestStableIDGemischteGruppeBleibtLocation(t *testing.T) {
	scanner := finding("s", "semgrep", "shared.rule", "installer/app.go", 12, "Derselbe Wortlaut", Dependency{})
	evidence := aiFinding("a", "review-tech", "shared.rule", "installer/app.go", 12, "Derselbe Wortlaut")

	mixed := GroupFindings([]Finding{scanner, evidence})
	if len(mixed) != 1 || len(mixed[0].FindingIDs) != 2 {
		t.Fatalf("Scanner und KI an derselben Stelle nicht gebündelt: %+v", mixed)
	}
	if !strings.HasPrefix(mixed[0].StableKey, "class=location\n") {
		t.Fatalf("gemischte Gruppe nicht in class=location: %q", mixed[0].StableKey)
	}
	if !strings.Contains(mixed[0].StableKey, "locations=") || !strings.Contains(mixed[0].StableKey, "messages=") {
		t.Errorf("Ort und Meldung fehlen im Schlüssel: %q", mixed[0].StableKey)
	}
	if !strings.HasPrefix(mixed[0].StableID, "scan-") {
		t.Errorf("gemischte Gruppe trägt kein Scanner-Präfix: %s", mixed[0].StableID)
	}

	// Der Kontrast: derselbe Fund allein ist class=ai.
	alone := GroupFindings([]Finding{evidence})
	if !strings.HasPrefix(alone[0].StableKey, "class=ai\n") {
		t.Fatalf("reine KI-Gruppe nicht in class=ai: %q", alone[0].StableKey)
	}
}

// Der Test aus Etappe 4 des Tasks: zwei Läufe über dieselbe Datei, im zweiten
// die Zeile verschoben und der Meldungstext umformuliert. Die Gruppen-ID bleibt
// gleich, und eine stableId-Decision aus Lauf 1 greift in Lauf 2.
func TestStableIDFuerKIEvidenceUeberlebtZeileUndMeldung(t *testing.T) {
	projectDir := t.TempDir()
	firstDir := evidenceRunDir(t, projectDir, "2026-08-25", "review-tech",
		sarifFixture("review-tech", sarifResultFixture("tech.altlast", "warning",
			"installer/internal/app.go", 12, "Veraltete Abhängigkeit an dieser Stelle")))
	first, err := Build(Options{ProjectDir: projectDir, RunName: "2026-08-25", RunDir: firstDir, LocalDir: projectDir, Now: fixedNow})
	if err != nil {
		t.Fatalf("Build Lauf 1: %v", err)
	}
	if len(first.Groups) != 1 {
		t.Fatalf("Lauf 1: %d Gruppen, erwartet 1", len(first.Groups))
	}

	secondDir := evidenceRunDir(t, projectDir, "2026-08-26", "review-tech",
		sarifFixture("review-tech", sarifResultFixture("tech.altlast", "warning",
			"installer/internal/app.go", 87, "Diese Stelle hängt an einer Bibliothek, die niemand mehr pflegt")))
	second, err := Build(Options{ProjectDir: projectDir, RunName: "2026-08-26", RunDir: secondDir, LocalDir: projectDir, Now: fixedNow})
	if err != nil {
		t.Fatalf("Build Lauf 2: %v", err)
	}
	if len(second.Groups) != 1 {
		t.Fatalf("Lauf 2: %d Gruppen, erwartet 1", len(second.Groups))
	}
	if first.Groups[0].StableKey != second.Groups[0].StableKey {
		t.Fatalf("Schlüssel verschoben:\n%q\n%q", first.Groups[0].StableKey, second.Groups[0].StableKey)
	}
	if first.Groups[0].StableID != second.Groups[0].StableID {
		t.Fatalf("stableId nicht stabil: %s / %s", first.Groups[0].StableID, second.Groups[0].StableID)
	}
	if !strings.HasPrefix(first.Groups[0].StableID, "ai-review-tech-") {
		t.Fatalf("KI-Präfix fehlt: %s", first.Groups[0].StableID)
	}

	writeText(t, filepath.Join(projectDir, "known-decisions.md"),
		"## kd-ai-altlast\n\n```yaml\nid: kd-ai-altlast\ncategory: wontfix\nmatch:\n  - stableId: "+
			first.Groups[0].StableID+"\n```\n\nBewusst so gelassen.\n")
	covered, err := Build(Options{ProjectDir: projectDir, RunName: "2026-08-26", RunDir: secondDir, LocalDir: projectDir, Now: fixedNow})
	if err != nil {
		t.Fatalf("Build Lauf 2 mit Decision: %v", err)
	}
	if len(covered.Groups) != 1 || covered.Groups[0].CoveredByKnownDecision == nil {
		t.Fatalf("Decision aus Lauf 1 greift in Lauf 2 nicht: %+v", covered.Groups)
	}
	if covered.Groups[0].CoveredByKnownDecision.ID != "kd-ai-altlast" || covered.Groups[0].CoveredByKnownDecision.MatchedBy != "stableId" {
		t.Fatalf("Deckung falsch: %+v", covered.Groups[0].CoveredByKnownDecision)
	}
}

// --- KI-Evidence: Merge-Integration ------------------------------------------

// Der Grundfall aus Etappe 5: ein Evidence-Eintrag neben einem Scanner. Beide
// Quellen landen in denselben Artefakten, ihre Gruppen bekommen eigene IDs, und
// review-input.json bekommt kein neues Feld dafür.
func TestBuildSammeltEvidenceEintragNebenScannerEin(t *testing.T) {
	projectDir := t.TempDir()
	runDir := runDirWithEntries(t, projectDir, "2026-08-25",
		toolEntry("gosec", sarifFixture("gosec",
			sarifResultFixture("G304", "warning", "installer/internal/app.go", 12, "Potential file inclusion"))),
		evidenceEntry("review-tech", sarifFixture("review-tech",
			sarifResultFixture("tech.altlast", "error", "installer/internal/app.go", 88, "Nicht mehr gepflegte Bibliothek"))),
	)

	result, output, err := Run(Options{ProjectDir: projectDir, RunName: "2026-08-25", RunDir: runDir, LocalDir: projectDir, Now: fixedNow})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(result.Findings) != 2 {
		t.Fatalf("%d Findings, erwartet 2: %+v", len(result.Findings), result.Findings)
	}
	tools := map[string]string{}
	for _, item := range result.Findings {
		tools[item.Evidence.Tool] = item.Evidence.SARIF
	}
	if tools["review-tech"] != "raw/review-tech.sarif" {
		t.Fatalf("evidence.tool trägt den Rezeptnamen nicht: %+v", tools)
	}
	if _, ok := tools["gosec"]; !ok {
		t.Fatalf("Scanner-Fund fehlt: %+v", tools)
	}

	if len(result.Groups) != 2 {
		t.Fatalf("%d Gruppen, erwartet 2: %+v", len(result.Groups), result.Groups)
	}
	if result.Groups[0].StableID == result.Groups[1].StableID {
		t.Fatalf("Tool- und KI-Gruppe teilen sich eine stableId: %+v", result.Groups)
	}
	prefixes := map[string]bool{}
	for _, group := range result.Groups {
		prefixes[stablePrefixFromID(group.StableID)] = true
	}
	if !prefixes["scan-gosec-"] || !prefixes["ai-review-tech-"] {
		t.Fatalf("Präfixe trennen Tool- und KI-Evidence nicht: %+v", prefixes)
	}

	// entries[] und findings[] bekommen kein mode-Feld: der Modus bleibt intern.
	// Unter run.selectedEntries steht er sehr wohl — das ist die unveränderte
	// Spiegelung von run.json aus Etappe 1 und kein neues Merge-Feld.
	data, err := os.ReadFile(output.JSON)
	if err != nil {
		t.Fatalf("JSON lesen: %v", err)
	}
	var document struct {
		Entries  []map[string]any `json:"entries"`
		Findings []map[string]any `json:"findings"`
		Run      struct {
			SelectedEntries []map[string]any `json:"selectedEntries"`
		} `json:"run"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("JSON lesen: %v", err)
	}
	for _, item := range append(append([]map[string]any{}, document.Entries...), document.Findings...) {
		if _, found := item["mode"]; found {
			t.Fatalf("mode ist im Schema von review-input.json gelandet: %+v", item)
		}
	}
	selected := document.Run.SelectedEntries
	if len(selected) != 2 || selected[1]["mode"] != string(review.ModeEvidence) {
		t.Fatalf("run.selectedEntries spiegelt run.json nicht mehr: %+v", selected)
	}

	// Unterscheidbar bleibt die Herkunft über evidence.tool — im JSON wie in der
	// Belege-Spalte des Markdowns. Ein eigenes Schemafeld gibt es dafür nicht.
	markdownData, err := os.ReadFile(output.Markdown)
	if err != nil {
		t.Fatalf("Markdown lesen: %v", err)
	}
	if !strings.Contains(string(markdownData), "review-tech/review-tech") {
		t.Fatalf("KI-Evidence im Markdown nicht als solche erkennbar: %s", markdownData)
	}
}

// Severity: für KI-Evidence ist das level aus dem SARIF maßgeblich. Das Mapping
// greift nur, wo level warning oder none sagt — und legt damit fest, dass die
// Rule-ID-Liste im Rezept das level je Rule-ID mitbringen muss.
func TestEvidenceSeverityFolgtDemLevel(t *testing.T) {
	projectDir := t.TempDir()
	runDir := evidenceRunDir(t, projectDir, "2026-08-25", "review-tech",
		sarifFixture("review-tech",
			sarifResultFixture("tech.altlast", "error", "installer/internal/a.go", 3, "Schwer"),
			sarifResultFixture("tech.kopplung", "warning", "installer/internal/b.go", 4, "Mittel"),
			sarifResultFixture("tech.stilblüte", "note", "installer/internal/c.go", 5, "Leicht"),
		))
	mappingPath := filepath.Join(projectDir, "severity.tsv")
	writeText(t, mappingPath, "tool\trule_prefix\tseverity\tnotes\nreview-tech\ttech.\tnote\tRückfall\n")

	result, err := Build(Options{
		ProjectDir: projectDir, RunName: "2026-08-25", RunDir: runDir, LocalDir: projectDir,
		SeverityMappingPath: mappingPath, Now: fixedNow,
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	want := map[string][2]string{
		"tech.altlast":   {"error", "native"},
		"tech.kopplung":  {"note", "mapping"},
		"tech.stilblüte": {"note", "native"},
	}
	if len(result.Findings) != len(want) {
		t.Fatalf("%d Findings, erwartet %d", len(result.Findings), len(want))
	}
	for _, item := range result.Findings {
		expected, ok := want[item.RuleID]
		if !ok {
			t.Fatalf("unerwartete Rule-ID: %s", item.RuleID)
		}
		if item.DerivedSeverity != expected[0] || item.SeveritySource != expected[1] {
			t.Errorf("%s: Schwere %s/%s, erwartet %s/%s",
				item.RuleID, item.DerivedSeverity, item.SeveritySource, expected[0], expected[1])
		}
	}
}

// Re-Run: findet der Folgelauf einen entschiedenen Fund nicht wieder, bleibt die
// Decision bestehen und erscheint als nicht angewendet. Das ist kein
// Erledigt-Signal — die Triage muss es so benennen.
func TestEvidenceDecisionOhneWiederfundBleibtNichtAngewendet(t *testing.T) {
	projectDir := t.TempDir()
	firstDir := evidenceRunDir(t, projectDir, "2026-08-25", "review-tech",
		sarifFixture("review-tech", sarifResultFixture("tech.altlast", "warning",
			"installer/internal/app.go", 12, "Nicht mehr gepflegte Bibliothek")))
	first, err := Build(Options{ProjectDir: projectDir, RunName: "2026-08-25", RunDir: firstDir, LocalDir: projectDir, Now: fixedNow})
	if err != nil {
		t.Fatalf("Build Lauf 1: %v", err)
	}
	if len(first.Groups) != 1 {
		t.Fatalf("Lauf 1: %d Gruppen, erwartet 1", len(first.Groups))
	}
	writeText(t, filepath.Join(projectDir, "known-decisions.md"),
		"## kd-ai-altlast\n\n```yaml\nid: kd-ai-altlast\ncategory: wontfix\nmatch:\n  - stableId: "+
			first.Groups[0].StableID+"\n```\n\nBewusst so gelassen.\n")

	// Derselbe Auftrag, andere Fundmenge: ein leeres SARIF ist ein gültiges
	// Ergebnis, kein Fehler.
	secondDir := evidenceRunDir(t, projectDir, "2026-08-26", "review-tech", sarifFixture("review-tech"))
	second, err := Build(Options{ProjectDir: projectDir, RunName: "2026-08-26", RunDir: secondDir, LocalDir: projectDir, Now: fixedNow})
	if err != nil {
		t.Fatalf("Build Lauf 2: %v", err)
	}
	if len(second.Findings) != 0 || len(second.Groups) != 0 {
		t.Fatalf("leeres SARIF liefert Funde: %+v", second.Findings)
	}
	if len(second.KnownDecisions.Decisions) != 1 {
		t.Fatalf("Decision verschwunden: %+v", second.KnownDecisions.Decisions)
	}
	report := second.KnownDecisions.Decisions[0]
	if report.ID != "kd-ai-altlast" || report.Applied || report.Expired {
		t.Fatalf("Decision falsch gemeldet: %+v", report)
	}
	if report.NotAppliedReason != "kein Finding getroffen" {
		t.Fatalf("Grund fehlt oder ist falsch: %q", report.NotAppliedReason)
	}
}

// Die Deckungswege für KI-Evidence, nebeneinander gemessen:
//
//   - pathGlob ist die grobe Ausnahme. Er trifft jeden Fund an diesem Pfad, auch
//     den Scanner-Fund eines fremden Werkzeugs mit fremder Regel.
//   - ruleId + location ist enger, aber location ist ein **Pfad**-Glob und keine
//     Zeile: derselbe Fund an anderer Zeile bleibt gedeckt.
func TestEvidenceDeckungPathGlobUndRuleIDLocation(t *testing.T) {
	build := func(t *testing.T, decision string, line int) Result {
		t.Helper()
		projectDir := t.TempDir()
		runDir := runDirWithEntries(t, projectDir, "2026-08-25",
			toolEntry("gosec", sarifFixture("gosec",
				sarifResultFixture("G304", "warning", "installer/internal/app.go", 12, "Potential file inclusion"))),
			evidenceEntry("review-tech", sarifFixture("review-tech",
				sarifResultFixture("tech.altlast", "warning", "installer/internal/app.go", line, "Nicht mehr gepflegte Bibliothek"))),
		)
		writeText(t, filepath.Join(projectDir, "known-decisions.md"), decision)
		result, err := Build(Options{ProjectDir: projectDir, RunName: "2026-08-25", RunDir: runDir, LocalDir: projectDir, Now: fixedNow})
		if err != nil {
			t.Fatalf("Build: %v", err)
		}
		return result
	}
	coveredTools := func(result Result) map[string]string {
		covered := map[string]string{}
		for _, item := range result.Findings {
			if item.CoveredByKnownDecision != nil {
				covered[item.Evidence.Tool] = item.CoveredByKnownDecision.MatchedBy
			}
		}
		return covered
	}

	broad := build(t, "## kd-pfad\n\n```yaml\nid: kd-pfad\ncategory: accepted-risk\nmatch:\n"+
		"  - pathGlob: installer/internal/app.go\n```\n\nGrobe Ausnahme.\n", 88)
	if got := coveredTools(broad); len(got) != 2 || got["review-tech"] != "pathGlob" || got["gosec"] != "pathGlob" {
		t.Fatalf("pathGlob deckt nicht auch den Scanner-Fund: %+v", got)
	}

	narrow := build(t, "## kd-regel\n\n```yaml\nid: kd-regel\ncategory: accepted-risk\nmatch:\n"+
		"  - ruleId: tech.altlast\n    location: installer/internal/app.go\n```\n\nEnger Weg.\n", 88)
	if got := coveredTools(narrow); len(got) != 1 || got["review-tech"] != "ruleId+location" {
		t.Fatalf("ruleId+location deckt die falsche Menge: %+v", got)
	}

	// location ist ein Pfad-Glob: die verschobene Zeile ändert die Deckung nicht.
	moved := build(t, "## kd-regel\n\n```yaml\nid: kd-regel\ncategory: accepted-risk\nmatch:\n"+
		"  - ruleId: tech.altlast\n    location: installer/internal/app.go\n```\n\nEnger Weg.\n", 401)
	if got := coveredTools(moved); len(got) != 1 || got["review-tech"] != "ruleId+location" {
		t.Fatalf("ruleId+location hängt an der Zeile: %+v", got)
	}
}
