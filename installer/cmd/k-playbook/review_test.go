package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/project/defaultstest"
	"github.com/kascada/k-playbook/installer/internal/review"
	"github.com/kascada/k-playbook/installer/internal/review/workdir"
	"github.com/kascada/k-playbook/installer/internal/sharedir/sharetest"
)

const cliCodeRecipe = "---\ntitle: Code-Review\nhandoff: /k-remediation\naudit:\n  enabled: true\n  mode: evidence\n  ruleIds:\n    - code-logic-error\n  scope:\n    paths:\n      - \"**/*.go\"\nreview:\n  enabled: true\n---\n# Code\n"

// `k-playbook review workdir` legt das Standardverzeichnis mit 0700 an,
// `review merge` fasst eine Fundliste darin zusammen und schreibt mit 0600;
// scan und merge bleiben dabei gesperrt.
func TestReviewWorkdirUndMergeUeberCLI(t *testing.T) {
	root := newProject(t)
	mustWriteMCPFile(t, filepath.Join(root, defaultstest.DefaultsDir, "reviews", "review-code.md"), cliCodeRecipe)
	mustWriteMCPFile(t, filepath.Join(sharetest.Dir(t), "scripts", "severity.tsv"), "tool\trule_prefix\tseverity\tnotes\n")
	temp := t.TempDir()
	before := workdir.TempRoot
	workdir.TempRoot = temp
	t.Cleanup(func() { workdir.TempRoot = before })
	t.Chdir(root)

	var out bytes.Buffer
	if err := runReviewWorkdir(&out, []string{"code"}); err != nil {
		t.Fatalf("review workdir: %v", err)
	}
	var prepared workdir.Prepared
	if err := json.Unmarshal(out.Bytes(), &prepared); err != nil {
		t.Fatalf("Ausgabe kein JSON: %v\n%s", err, out.String())
	}
	if filepath.Dir(prepared.Dir) != temp || !prepared.Created {
		t.Fatalf("workdir = %+v", prepared)
	}
	if info, err := os.Stat(prepared.Dir); err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("Verzeichnis: %v %v", info, err)
	}

	mustWriteMCPFile(t, filepath.Join(prepared.Dir, "code.sarif"), `{"version":"2.1.0","runs":[{"tool":{"driver":{"name":"code"}},"results":[`+
		`{"ruleId":"code-logic-error","level":"error","message":{"text":"Falsch."},`+
		`"locations":[{"physicalLocation":{"artifactLocation":{"uri":"main.go"},"region":{"startLine":2}}}]}]}]}`)
	out.Reset()
	if err := runReviewMerge(&out, []string{prepared.Dir}); err != nil {
		t.Fatalf("review merge: %v", err)
	}
	var merged workdir.MergeOutput
	if err := json.Unmarshal(out.Bytes(), &merged); err != nil {
		t.Fatalf("Ausgabe kein JSON: %v\n%s", err, out.String())
	}
	if merged.Findings != 1 || merged.Groups == nil || *merged.Groups != 1 || merged.Outputs["reviewInputJSON"] == "" {
		t.Errorf("merge = %+v", merged)
	}
	if info, err := os.Stat(merged.Outputs["reviewInputJSON"]); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("review-input.json: %v %v", info, err)
	}

	for _, args := range [][]string{{"merge", "2026-08-19"}, {"scan", "2026-08-19"}} {
		if err := run(args); !errors.Is(err, review.ErrPaused) {
			t.Errorf("%v: %v, erwartet die Sperre", args, err)
		}
	}
}

// Falsche Aufrufe enden mit einem Satz, ohne etwas anzulegen.
func TestReviewCLIFalscheAufrufe(t *testing.T) {
	root := newProject(t)
	t.Chdir(root)
	var out bytes.Buffer
	for _, call := range []struct {
		fn   func() error
		want string
	}{
		{func() error { return runReviewWorkdir(&out, nil) }, "Rezept"},
		{func() error { return runReviewWorkdir(&out, []string{"code", "tech"}) }, "genau ein Rezept"},
		{func() error { return runReviewWorkdir(&out, []string{"code", "--dir"}) }, "--dir"},
		{func() error { return runReviewMerge(&out, nil) }, "Arbeitsverzeichnis"},
		{func() error { return runReviewMerge(&out, []string{"a", "--x"}) }, "unbekannte Option"},
		{func() error { return runReview([]string{"gibt-es-nicht"}) }, "unbekannter Unterbefehl"},
	} {
		if err := call.fn(); err == nil || !strings.Contains(err.Error(), call.want) {
			t.Errorf("Fehler = %v, erwartet %q", err, call.want)
		}
	}
	if err := runReview(nil); err != nil {
		t.Errorf("Kurzhilfe: %v", err)
	}
}
