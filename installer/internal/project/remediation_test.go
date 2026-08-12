package project

import (
	"os"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()

	root := t.TempDir()
	if err := os.WriteFile(ConfigPath(root), []byte(content), 0o644); err != nil {
		t.Fatalf("Config anlegen: %v", err)
	}
	return root
}

func TestReadRemediationOhneBlock(t *testing.T) {
	root := writeConfig(t, "schema_version: 2\n\nproject:\n  repo_root: .\n")

	remediation, err := ReadRemediation(root)
	if err != nil {
		t.Fatalf("ReadRemediation: %v", err)
	}
	if remediation.Configured {
		t.Error("Configured = true, obwohl kein Block vorhanden ist")
	}
}

func TestReadRemediationLiestAlleFelder(t *testing.T) {
	root := writeConfig(t, `schema_version: 2

remediation:
  mode: task-branch-pr
  target: .
  grouping: true
  quick_wins: true
  branch_prefix: remediation/
  pr_required: true
  direct_fixes: false
`)

	remediation, err := ReadRemediation(root)
	if err != nil {
		t.Fatalf("ReadRemediation: %v", err)
	}
	if !remediation.Configured {
		t.Error("Configured = false")
	}
	if remediation.Mode != ModeTaskBranchPR {
		t.Errorf("Mode = %q, erwartet %q", remediation.Mode, ModeTaskBranchPR)
	}
	if !remediation.PRRequired || remediation.DirectFixes {
		t.Errorf("PRRequired = %v, DirectFixes = %v, erwartet true/false", remediation.PRRequired, remediation.DirectFixes)
	}
	if remediation.BranchPrefix != "remediation/" {
		t.Errorf("BranchPrefix = %q", remediation.BranchPrefix)
	}
}

func TestSetRemediationModeLegtBlockAn(t *testing.T) {
	root := writeConfig(t, "schema_version: 2\n\nproject:\n  repo_root: .\n  vcs: git\n")

	if err := SetRemediationMode(root, ModeTaskBranchPR); err != nil {
		t.Fatalf("SetRemediationMode: %v", err)
	}

	remediation, err := ReadRemediation(root)
	if err != nil {
		t.Fatalf("ReadRemediation: %v", err)
	}
	if remediation.Mode != ModeTaskBranchPR || !remediation.PRRequired || remediation.DirectFixes {
		t.Errorf("Zustand nach dem Schreiben: %+v", remediation)
	}

	// Der bestehende Inhalt muss erhalten bleiben.
	content, err := os.ReadFile(ConfigPath(root))
	if err != nil {
		t.Fatalf("Config lesen: %v", err)
	}
	for _, want := range []string{"schema_version: 2", "repo_root: .", "vcs: git"} {
		if !strings.Contains(string(content), want) {
			t.Errorf("%q fehlt nach dem Schreiben:\n%s", want, content)
		}
	}
}

// Ein Wechsel ersetzt den Block samt abgeleiteter Flags, statt nur mode zu
// ändern — sonst blieben widersprüchliche Werte stehen.
func TestSetRemediationModeErsetztFlagsMit(t *testing.T) {
	root := writeConfig(t, `schema_version: 2

remediation:
  mode: task-branch-pr
  target: .
  grouping: true
  quick_wins: true
  branch_prefix: remediation/
  pr_required: true
  direct_fixes: false

project:
  repo_root: .
`)

	if err := SetRemediationMode(root, ModeDirectAllowed); err != nil {
		t.Fatalf("SetRemediationMode: %v", err)
	}

	remediation, err := ReadRemediation(root)
	if err != nil {
		t.Fatalf("ReadRemediation: %v", err)
	}
	if remediation.Mode != ModeDirectAllowed {
		t.Errorf("Mode = %q, erwartet %q", remediation.Mode, ModeDirectAllowed)
	}
	if remediation.PRRequired || !remediation.DirectFixes {
		t.Errorf("Flags nicht mitgezogen: PRRequired = %v, DirectFixes = %v", remediation.PRRequired, remediation.DirectFixes)
	}

	// Der Block danach darf nicht verschluckt werden.
	content, err := os.ReadFile(ConfigPath(root))
	if err != nil {
		t.Fatalf("Config lesen: %v", err)
	}
	if !strings.Contains(string(content), "repo_root: .") {
		t.Errorf("nachfolgender Block verloren:\n%s", content)
	}
	if strings.Count(string(content), "remediation:") != 1 {
		t.Errorf("remediation-Block nicht ersetzt, sondern gedoppelt:\n%s", content)
	}
}

func TestSetRemediationModeLehntUnbekanntenModusAb(t *testing.T) {
	root := writeConfig(t, "schema_version: 2\n")

	if err := SetRemediationMode(root, RemediationMode("irgendwas")); err == nil {
		t.Error("unbekannter Modus wurde akzeptiert")
	}
}

func TestRemediationPolicy(t *testing.T) {
	cases := map[RemediationMode][2]bool{
		ModeTaskBranchPR:  {true, false},
		ModeTaskFirst:     {false, true},
		ModeDirectAllowed: {false, true},
	}
	for mode, want := range cases {
		pr, direct := RemediationPolicy(mode)
		if pr != want[0] || direct != want[1] {
			t.Errorf("%s: PRRequired = %v, DirectFixes = %v, erwartet %v/%v", mode, pr, direct, want[0], want[1])
		}
	}
}

// Ohne Block gilt der Standard, damit Commands nicht raten müssen.
func TestReadRemediationDefaultOhneBlock(t *testing.T) {
	root := writeConfig(t, "schema_version: 3\n\nproject:\n  repo_root: .\n")

	remediation, err := ReadRemediation(root)
	if err != nil {
		t.Fatalf("ReadRemediation: %v", err)
	}
	if remediation.Mode != DefaultRemediationMode {
		t.Errorf("Mode = %q, erwartet %q", remediation.Mode, DefaultRemediationMode)
	}
	if remediation.Configured {
		t.Error("Configured = true, obwohl der Wert nur der Standard ist")
	}

	// Die Flags müssen zum Standardmodus passen.
	wantPR, wantDirect := RemediationPolicy(DefaultRemediationMode)
	if remediation.PRRequired != wantPR || remediation.DirectFixes != wantDirect {
		t.Errorf("Flags = %v/%v, erwartet %v/%v",
			remediation.PRRequired, remediation.DirectFixes, wantPR, wantDirect)
	}
}
