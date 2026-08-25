package review

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeModeMachtAusLeerPerspective(t *testing.T) {
	if got := NormalizeMode(""); got != ModePerspective {
		t.Errorf("NormalizeMode(\"\") = %q, erwartet %q", got, ModePerspective)
	}
	if got := NormalizeMode(ModeEvidence); got != ModeEvidence {
		t.Errorf("NormalizeMode(evidence) = %q", got)
	}
}

func TestValidModeKenntNurDieBeidenBetriebsartenUndDasLeereFeld(t *testing.T) {
	for _, mode := range []Mode{"", ModePerspective, ModeEvidence} {
		if !ValidMode(mode) {
			t.Errorf("ValidMode(%q) = false", mode)
		}
	}
	if ValidMode("scanner") {
		t.Error("ValidMode(\"scanner\") = true")
	}
}

func TestValidateAuditContract(t *testing.T) {
	cases := []struct {
		name     string
		contract AuditContract
		wantErr  bool
	}{
		{
			name:     "leeres Feld ist eine gültige Perspektive",
			contract: AuditContract{Scope: &Scope{Tools: []string{"gitleaks"}}, ResultRequiredSet: true, DefaultResult: "review-x.md"},
		},
		{
			name:     "perspective mit tools, resultRequired und defaultResult",
			contract: AuditContract{Mode: ModePerspective, Scope: &Scope{Tools: []string{"gitleaks"}}, ResultRequiredSet: true, DefaultResult: "review-x.md"},
		},
		{
			name:     "perspective ohne jeden Scope bleibt gültig",
			contract: AuditContract{Mode: ModePerspective},
		},
		{
			name:     "perspective mit scope.paths",
			contract: AuditContract{Mode: ModePerspective, Scope: &Scope{Paths: []string{"installer/**"}}},
			wantErr:  true,
		},
		{
			name:     "perspective mit ruleIds",
			contract: AuditContract{Mode: ModePerspective, RuleIDs: []string{"tech-x"}},
			wantErr:  true,
		},
		{
			name:     "evidence mit paths und ruleIds",
			contract: AuditContract{Mode: ModeEvidence, Scope: &Scope{Paths: []string{"installer/**"}}, RuleIDs: []string{"tech-x"}},
		},
		{
			name:     "evidence ohne paths",
			contract: AuditContract{Mode: ModeEvidence, RuleIDs: []string{"tech-x"}},
			wantErr:  true,
		},
		{
			name:     "evidence ohne ruleIds",
			contract: AuditContract{Mode: ModeEvidence, Scope: &Scope{Paths: []string{"installer/**"}}},
			wantErr:  true,
		},
		{
			name:     "evidence mit leeren ruleIds zählt als ohne",
			contract: AuditContract{Mode: ModeEvidence, Scope: &Scope{Paths: []string{"installer/**"}}, RuleIDs: []string{"  "}},
			wantErr:  true,
		},
		{
			name:     "evidence mit scope.tools",
			contract: AuditContract{Mode: ModeEvidence, Scope: &Scope{Tools: []string{"gitleaks"}, Paths: []string{"installer/**"}}, RuleIDs: []string{"tech-x"}},
			wantErr:  true,
		},
		{
			name:     "evidence mit resultRequired",
			contract: AuditContract{Mode: ModeEvidence, Scope: &Scope{Paths: []string{"installer/**"}}, RuleIDs: []string{"tech-x"}, ResultRequiredSet: true},
			wantErr:  true,
		},
		{
			name:     "evidence mit defaultResult",
			contract: AuditContract{Mode: ModeEvidence, Scope: &Scope{Paths: []string{"installer/**"}}, RuleIDs: []string{"tech-x"}, DefaultResult: "review-tech.md"},
			wantErr:  true,
		},
		{
			name:     "unbekannte Betriebsart",
			contract: AuditContract{Mode: "scanner"},
			wantErr:  true,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			err := ValidateAuditContract(testCase.contract)
			if testCase.wantErr && err == nil {
				t.Fatal("Fehler erwartet, keiner gemeldet")
			}
			if !testCase.wantErr && err != nil {
				t.Fatalf("kein Fehler erwartet: %v", err)
			}
		})
	}
}

func TestCreateRunFriertModusUndPfadScopeEin(t *testing.T) {
	localDir := newLocalDir(t)

	runDir, err := CreateRun(localDir, day(t, "2026-08-25"), []string{"go"}, []Entry{
		{Name: "review-tech", Kind: KindAI, Mode: ModeEvidence, Scope: &Scope{Paths: []string{"installer/**", "commands/**"}}},
		{Name: "review-secret-scanning", Kind: KindAI, Scope: &Scope{Tools: []string{"gitleaks"}}},
	})
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	run, err := ReadRun(runDir)
	if err != nil {
		t.Fatalf("ReadRun: %v", err)
	}
	evidence := run.Entries[0]
	if evidence.Mode != ModeEvidence {
		t.Errorf("Mode = %q, erwartet %q", evidence.Mode, ModeEvidence)
	}
	if evidence.Scope == nil || len(evidence.Scope.Paths) != 2 || evidence.Scope.Paths[0] != "installer/**" {
		t.Fatalf("Pfad-Scope = %#v", evidence.Scope)
	}
	// Der zweite Eintrag trägt keinen Modus: die Vorgabe steht nicht in der
	// Datei, sondern entsteht beim Lesen.
	perspective := run.Entries[1]
	if perspective.Mode != "" {
		t.Errorf("Mode = %q, erwartet leer", perspective.Mode)
	}
	if EntryMode(perspective) != ModePerspective {
		t.Errorf("EntryMode = %q, erwartet %q", EntryMode(perspective), ModePerspective)
	}
}

func TestCreateRunWeistUnbekannteBetriebsartAb(t *testing.T) {
	localDir := newLocalDir(t)

	_, err := CreateRun(localDir, day(t, "2026-08-25"), nil, []Entry{
		{Name: "review-tech", Kind: KindAI, Mode: "scanner"},
	})
	if err == nil {
		t.Fatal("Fehler erwartet")
	}
}

// Ein Lauf aus der Zeit vor der Evidence-Betriebsart hat kein mode-Feld und
// dieselbe schemaVersion. Er muss unverändert lesbar bleiben und als
// Perspektiven-Lauf gelten.
func TestReadRunLiestAltlaufOhneModeAlsPerspektive(t *testing.T) {
	runDir := t.TempDir()
	altlauf := `{
  "schemaVersion": 1,
  "created": "2026-01-05T00:00:00+01:00",
  "state": "created",
  "languages": ["go"],
  "entries": [
    {"name": "gitleaks", "kind": "tool", "state": "start"},
    {"name": "review-tech", "kind": "ai", "state": "start", "resultRequired": true, "defaultResult": "review-tech.md", "scope": {"tools": ["gitleaks"]}}
  ]
}
`
	if err := os.WriteFile(filepath.Join(runDir, RunFileName), []byte(altlauf), 0o644); err != nil {
		t.Fatalf("run.json schreiben: %v", err)
	}

	run, err := ReadRun(runDir)
	if err != nil {
		t.Fatalf("ReadRun: %v", err)
	}
	if run.SchemaVersion != SchemaVersion {
		t.Errorf("SchemaVersion = %d, erwartet %d", run.SchemaVersion, SchemaVersion)
	}
	for _, entry := range run.Entries {
		if entry.Mode != "" {
			t.Errorf("%s trägt Mode %q, erwartet leer", entry.Name, entry.Mode)
		}
		if EntryMode(entry) != ModePerspective {
			t.Errorf("%s: EntryMode = %q, erwartet %q", entry.Name, EntryMode(entry), ModePerspective)
		}
	}
	if run.Entries[1].Scope == nil || len(run.Entries[1].Scope.Tools) != 1 {
		t.Fatalf("Scope = %#v", run.Entries[1].Scope)
	}
	if len(run.Entries[1].Scope.Paths) != 0 {
		t.Errorf("Paths = %#v, erwartet leer", run.Entries[1].Scope.Paths)
	}
}

// Ein Perspektiven-Lauf schreibt kein mode-Feld: run.json bleibt Zeichen für
// Zeichen so, wie sie vor der Evidence-Betriebsart aussah.
func TestCreateRunSchreibtKeinModeFeldFuerPerspektiven(t *testing.T) {
	localDir := newLocalDir(t)

	runDir, err := CreateRun(localDir, day(t, "2026-08-25"), []string{"go"}, []Entry{
		{Name: "gitleaks", Kind: KindTool},
	})
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(runDir, RunFileName))
	if err != nil {
		t.Fatalf("run.json lesen: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("run.json: %v", err)
	}
	entry := raw["entries"].([]any)[0].(map[string]any)
	if _, found := entry["mode"]; found {
		t.Errorf("mode steht in run.json: %v", entry)
	}
}

func TestPathExcludedFromScope(t *testing.T) {
	cases := map[string]bool{
		"installer/internal/review/run.go":  false,
		"commands/k-audit.md":               false,
		"installer/.golangci.yml":           false,
		".golangci.yml":                     false,
		"k-playbook/reviews/review-tech.md": true,
		"k-playbook-local/tasks/031.md":     true,
		"installer/testdata/go.mod":         true,
		"web/node_modules/x/index.js":       true,
		"third/vendor/lib/a.go":             true,
		".github/workflows/ci.yml":          true,
		"./installer/internal/review/x.go":  false,
	}
	for path, want := range cases {
		if got := PathExcludedFromScope(path); got != want {
			t.Errorf("PathExcludedFromScope(%q) = %v, erwartet %v", path, got, want)
		}
	}
}
