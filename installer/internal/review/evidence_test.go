package review

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

// evidenceSARIF baut ein SARIF mit einem Result je Fundort.
func evidenceSARIF(tool string, results ...string) string {
	return `{"version":"2.1.0","runs":[{"tool":{"driver":{"name":"` + tool + `","rules":[{"id":"tech-veraltet"},{"id":"tech-kopplung"}]}},"results":[` + strings.Join(results, ",") + `]}]}`
}

func evidenceResult(ruleID string, uri string, line int) string {
	location := ""
	if uri != "" {
		location = `,"locations":[{"physicalLocation":{"artifactLocation":{"uri":"` + uri + `"},"region":{"startLine":` + strconv.Itoa(line) + `}}}]`
	}
	return `{"ruleId":"` + ruleID + `","level":"error","message":{"text":"Fund"}` + location + `}`
}

func TestPathInScopeTrifftVerzeichnisUndGlob(t *testing.T) {
	scope := []string{"installer/**", "commands/**"}
	tests := []struct {
		path    string
		matched bool
	}{
		{"installer/internal/review/run.go", true},
		{"commands/k-audit.md", true},
		{"./installer/main.go", true},
		{"docs/README.md", false},
		{"installers/main.go", false},
		{"", false},
	}
	for _, test := range tests {
		if got := PathInScope(test.path, scope); got != test.matched {
			t.Errorf("PathInScope(%q) = %v, erwartet %v", test.path, got, test.matched)
		}
	}

	// Ein Muster ohne ** benennt trotzdem einen Bereich: sonst verlöre ein
	// Rezept mit scope.paths: [installer] jeden Fund.
	if !PathInScope("installer/internal/run.go", []string{"installer"}) {
		t.Error("Verzeichnismuster ohne ** trifft nicht")
	}
	if PathInScope("installer/main.go", nil) {
		t.Error("ohne scope.paths liegt nichts im Scope")
	}
}

func TestCheckEvidenceSARIFVerwirftFundeAusserhalbDesScopes(t *testing.T) {
	raw := evidenceSARIF("hardspots",
		evidenceResult("tech-veraltet", "installer/internal/review/run.go", 12),
		evidenceResult("tech-kopplung", "docs/README.md", 3),
		evidenceResult("tech-veraltet", "installer/testdata/alt.go", 7),
		evidenceResult("tech-kopplung", "", 0),
	)
	report, err := CheckEvidenceSARIF([]byte(raw), "hardspots", []string{"tech-veraltet", "tech-kopplung"}, []string{"installer/**"})
	if err != nil {
		t.Fatalf("CheckEvidenceSARIF: %v", err)
	}
	if report.Kept != 1 || report.Dropped != 3 {
		t.Fatalf("Kept = %d, Dropped = %d", report.Kept, report.Dropped)
	}
	if len(report.DroppedPaths) != 3 || report.DroppedPaths[0] != "docs/README.md" {
		t.Fatalf("DroppedPaths = %#v", report.DroppedPaths)
	}
	if report.DroppedPaths[1] != "installer/testdata/alt.go" {
		t.Errorf("zentrale Ausschlüsse greifen nicht: %#v", report.DroppedPaths)
	}
	note := report.ScopeNote()
	if !strings.Contains(note, "3 Fund") || !strings.Contains(note, "docs/README.md") {
		t.Fatalf("ScopeNote = %q", note)
	}
	if report.Cleaned == nil {
		t.Fatal("bereinigtes SARIF fehlt")
	}

	// Das bereinigte SARIF enthält genau den Fund im Scope, und die
	// Zeilennummer steht unverändert darin.
	var cleaned struct {
		Runs []struct {
			Results []struct {
				RuleID    string `json:"ruleId"`
				Locations []struct {
					PhysicalLocation struct {
						ArtifactLocation struct {
							URI string `json:"uri"`
						} `json:"artifactLocation"`
						Region struct {
							StartLine int `json:"startLine"`
						} `json:"region"`
					} `json:"physicalLocation"`
				} `json:"locations"`
			} `json:"results"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(report.Cleaned, &cleaned); err != nil {
		t.Fatalf("bereinigtes SARIF nicht lesbar: %v", err)
	}
	if len(cleaned.Runs) != 1 || len(cleaned.Runs[0].Results) != 1 {
		t.Fatalf("bereinigtes SARIF = %s", report.Cleaned)
	}
	result := cleaned.Runs[0].Results[0]
	if result.RuleID != "tech-veraltet" || result.Locations[0].PhysicalLocation.ArtifactLocation.URI != "installer/internal/review/run.go" {
		t.Fatalf("verbliebener Fund = %#v", result)
	}
	if result.Locations[0].PhysicalLocation.Region.StartLine != 12 {
		t.Errorf("startLine = %d, erwartet 12", result.Locations[0].PhysicalLocation.Region.StartLine)
	}
}

func TestCheckEvidenceSARIFLaesstVollstaendigesArtefaktUnangetastet(t *testing.T) {
	raw := evidenceSARIF("hardspots", evidenceResult("tech-veraltet", "installer/main.go", 1))
	report, err := CheckEvidenceSARIF([]byte(raw), "hardspots", []string{"tech-veraltet"}, []string{"installer/**"})
	if err != nil {
		t.Fatalf("CheckEvidenceSARIF: %v", err)
	}
	if report.Kept != 1 || report.Dropped != 0 {
		t.Fatalf("Kept = %d, Dropped = %d", report.Kept, report.Dropped)
	}
	if report.Cleaned != nil {
		t.Error("ohne verworfenen Fund wird nichts zurückgeschrieben")
	}
	if report.ScopeNote() != "" {
		t.Errorf("ScopeNote = %q, erwartet leer", report.ScopeNote())
	}
}

// Ein leerer Scope-Befund ist ein Ergebnis und kein Fehler.
func TestCheckEvidenceSARIFOhneErgebnisseIstGueltig(t *testing.T) {
	report, err := CheckEvidenceSARIF([]byte(evidenceSARIF("hardspots")), "hardspots", []string{"tech-veraltet"}, []string{"installer/**"})
	if err != nil {
		t.Fatalf("CheckEvidenceSARIF: %v", err)
	}
	if report.Kept != 0 || report.Dropped != 0 || report.Cleaned != nil {
		t.Fatalf("report = %#v", report)
	}
}

func TestCheckEvidenceSARIFMeldetUngueltigesArtefakt(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		ruleIDs  []string
		fragment string
	}{
		{
			name:     "kein JSON",
			raw:      "{kein json",
			ruleIDs:  []string{"tech-veraltet"},
			fragment: "lesbares SARIF-JSON",
		},
		{
			name:     "fremder Werkzeugname",
			raw:      evidenceSARIF("semgrep", evidenceResult("tech-veraltet", "installer/main.go", 1)),
			ruleIDs:  []string{"tech-veraltet"},
			fragment: "entspricht nicht dem Eintrag",
		},
		{
			name:     "Rule-ID außerhalb der Liste",
			raw:      evidenceSARIF("hardspots", evidenceResult("tech-erfunden", "installer/main.go", 1)),
			ruleIDs:  []string{"tech-veraltet"},
			fragment: "steht nicht in audit.ruleIds",
		},
		{
			name:     "Fund ohne ruleId",
			raw:      evidenceSARIF("hardspots", evidenceResult("", "installer/main.go", 1)),
			ruleIDs:  []string{"tech-veraltet"},
			fragment: "ohne ruleId",
		},
		{
			name:     "ohne runs",
			raw:      `{"version":"2.1.0"}`,
			ruleIDs:  []string{"tech-veraltet"},
			fragment: "ohne runs",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			report, err := CheckEvidenceSARIF([]byte(test.raw), "hardspots", test.ruleIDs, []string{"installer/**"})
			if err == nil {
				t.Fatalf("kein Fehler, report = %#v", report)
			}
			if !strings.Contains(err.Error(), test.fragment) {
				t.Fatalf("Fehler = %q, erwartet %q", err.Error(), test.fragment)
			}
			if report.Cleaned != nil {
				t.Error("ein ungültiges Artefakt wird nicht bereinigt zurückgeschrieben")
			}
		})
	}
}

// Die Rule-ID darf auch über ruleIndex kommen — der Merge löst sie genauso auf.
func TestCheckEvidenceSARIFLiestRuleIndex(t *testing.T) {
	raw := `{"runs":[{"tool":{"driver":{"name":"hardspots","rules":[{"id":"tech-veraltet"}]}},"results":[{"ruleIndex":0,"message":{"text":"Fund"},"locations":[{"physicalLocation":{"artifactLocation":{"uri":"installer/main.go"}}}]}]}]}`
	report, err := CheckEvidenceSARIF([]byte(raw), "hardspots", []string{"tech-veraltet"}, []string{"installer/**"})
	if err != nil {
		t.Fatalf("CheckEvidenceSARIF: %v", err)
	}
	if report.Kept != 1 {
		t.Fatalf("Kept = %d", report.Kept)
	}
}

func TestEvidenceSARIFPath(t *testing.T) {
	if got := EvidenceSARIFPath("hardspots"); got != "raw/hardspots.sarif" {
		t.Fatalf("EvidenceSARIFPath = %q", got)
	}
}

func TestNormalizeScopePath(t *testing.T) {
	tests := map[string]string{
		"file:///home/x/installer/main.go": "home/x/installer/main.go",
		"./installer/main.go":              "installer/main.go",
		"/installer/main.go":               "installer/main.go",
		`installer\main.go`:                "installer/main.go",
	}
	for input, want := range tests {
		if got := NormalizeScopePath(input); got != want {
			t.Errorf("NormalizeScopePath(%q) = %q, erwartet %q", input, got, want)
		}
	}
}
