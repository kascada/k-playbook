package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// runFindingsCaptured führt das Subkommando aus und fängt stdout ab.
func runFindingsCaptured(t *testing.T, args ...string) (string, error) {
	t.Helper()
	file, err := os.Create(filepath.Join(t.TempDir(), "stdout"))
	if err != nil {
		t.Fatalf("Ausgabedatei: %v", err)
	}
	before := os.Stdout
	os.Stdout = file
	runErr := runFindings(args)
	os.Stdout = before
	file.Close()
	stdout, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatalf("Ausgabe lesen: %v", err)
	}
	return string(stdout), runErr
}

// fakeFindingsCheck setzt das Ergebnis der Vorprüfung und merkt sich das
// Projekt, für das gefragt wurde.
func fakeFindingsCheck(t *testing.T, check project.FindingsCheck) *string {
	t.Helper()
	var asked string
	original := checkFindings
	checkFindings = func(_ context.Context, projectDir string) project.FindingsCheck {
		asked = projectDir
		return check
	}
	t.Cleanup(func() { checkFindings = original })
	return &asked
}

func notReadyCheck() project.FindingsCheck {
	revision := int64(7664)
	return project.FindingsCheck{
		Ready:    false,
		Problems: []project.FindingsProblem{{Kind: project.FindingsProblemRight, Name: "supersede", Message: "Das Recht supersede fehlt."}},
		Sync:     &project.FindingsSync{At: "2026-11-17T16:17:10Z", Revision: &revision},
		Rights:   []string{"read", "write"},
		Total:    3,
		ByUser:   map[string]int{"anna": 1, "kamran": 2},
		Skipped: []project.FindingsSkipped{{FindingItem: project.FindingItem{Name: "queue/findings/003-c.md"},
			Reason: project.FindingsSkipEmpty}},
		Findings: []project.FindingItem{{Name: "queue/findings/001-a.md"}, {Name: "queue/findings/002-b.md"}},
	}
}

func TestFindingsCheckKlartextNichtBereit(t *testing.T) {
	root := todoProjectRoot(t)
	asked := fakeFindingsCheck(t, notReadyCheck())

	stdout, err := runFindingsCaptured(t, "check")
	if err == nil || err.Error() != "" || exitCodeOf(err) != findingsExitNotReady {
		t.Fatalf("Fehler: %v (Code %d)", err, exitCodeOf(err))
	}
	if *asked != root {
		t.Fatalf("Projekt: %s, erwartet %s", *asked, root)
	}
	for _, want := range []string{
		"Befund-Queue: nicht bereit\n",
		"Abgleich: 2026-11-17T16:17:10Z, Revision 7664\n",
		"Rechte: read, write\n",
		"Befunde: 3 (kamran 2, anna 1)\n",
		"Übersprungen: 1\n  queue/findings/003-c.md (leer)\n",
		"Probleme:\n  [right] Das Recht supersede fehlt.\n",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("fehlt %q in:\n%s", want, stdout)
		}
	}
}

func TestFindingsCheckJSONBereit(t *testing.T) {
	todoProjectRoot(t)
	fakeFindingsCheck(t, project.FindingsCheck{Ready: true, Problems: []project.FindingsProblem{}, Rights: []string{"read", "write", "supersede"},
		ByUser: map[string]int{}, Skipped: []project.FindingsSkipped{}, Findings: []project.FindingItem{}})

	stdout, err := runFindingsCaptured(t, "check", "--json")
	if err != nil {
		t.Fatalf("bereit endet mit Fehler: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("kein JSON: %v\n%s", err, stdout)
	}
	for _, key := range []string{"ready", "problems", "rights", "total", "byUser", "skipped", "findings"} {
		if _, ok := got[key]; !ok {
			t.Errorf("Feld %s fehlt: %v", key, got)
		}
	}
	if got["ready"] != true || got["total"] != float64(0) {
		t.Fatalf("Ergebnis: %v", got)
	}
}

func TestFindingsAufruffehler(t *testing.T) {
	todoProjectRoot(t)
	fakeFindingsCheck(t, notReadyCheck())
	for _, args := range [][]string{{"pruefen"}, {"check", "--bunt"}} {
		_, err := runFindingsCaptured(t, args...)
		if exitCodeOf(err) != findingsExitUsage {
			t.Errorf("%v: %v (Code %d)", args, err, exitCodeOf(err))
		}
	}
	if stdout, err := runFindingsCaptured(t, "--help"); err != nil || !strings.Contains(stdout, "findings check [--json]") {
		t.Errorf("Hilfe: %v\n%s", err, stdout)
	}
}
