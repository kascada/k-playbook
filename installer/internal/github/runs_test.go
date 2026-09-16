package github

import (
	"context"
	"testing"
)

const runsJSON = `[
 {"conclusion":"failure","createdAt":"2026-09-15T17:39:55Z","databaseId":35002675449,
  "displayTitle":"Release v0.9.0","event":"push","headBranch":"main","startedAt":"2026-09-15T17:39:55Z",
  "status":"completed","updatedAt":"2026-09-15T17:40:46Z",
  "url":"https://github.com/kascada/k-playbook/actions/runs/35002675449","workflowName":"Tests"},
 {"conclusion":"failure","createdAt":"2026-09-15T17:30:00Z","databaseId":35002537701,
  "displayTitle":"v0.9.0","event":"push","headBranch":"v0.9.0","startedAt":"2026-09-15T17:30:00Z",
  "status":"completed","updatedAt":"2026-09-15T17:31:10Z",
  "url":"https://github.com/kascada/k-playbook/actions/runs/35002537701","workflowName":"Tests"},
 {"conclusion":"","createdAt":"2026-09-16T06:00:00Z","databaseId":35100000000,
  "displayTitle":"Zwischenstand","event":"push","headBranch":"main","startedAt":"2026-09-16T06:00:00Z",
  "status":"in_progress","updatedAt":"2026-09-16T06:00:30Z",
  "url":"https://github.com/kascada/k-playbook/actions/runs/35100000000","workflowName":"VERSION-Wächter"}
]`

func TestRunsLiestBranchTagDauerUndErgebnis(t *testing.T) {
	runs := ParseRuns([]byte(runsJSON))

	if runs.State != StateOK {
		t.Fatalf("Zustand %q, erwartet ok", runs.State)
	}
	if len(runs.Runs) != 3 {
		t.Fatalf("%d Läufe, erwartet 3", len(runs.Runs))
	}

	auf := runs.Runs[0]
	if auf.Ref != "main" || auf.IsTag {
		t.Errorf("erster Lauf: Ref %q, Tag %v — erwartet main, kein Tag", auf.Ref, auf.IsTag)
	}
	if auf.Seconds != 51 {
		t.Errorf("Dauer %ds, erwartet 51s", auf.Seconds)
	}
	// Nur ein roter Lauf bietet seine Ursache an; das Log wird erst beim
	// Aufklappen geholt.
	if !auf.Failed {
		t.Error("der rote Lauf ist nicht als fehlgeschlagen markiert")
	}

	// `gh run list` schreibt Tag und Branch in dasselbe Feld. Ohne die
	// Unterscheidung sähe der Lauf zum Tag aus wie ein Branch.
	if tag := runs.Runs[1]; !tag.IsTag || tag.Ref != "v0.9.0" {
		t.Errorf("zweiter Lauf: Ref %q, Tag %v — erwartet v0.9.0 als Tag", tag.Ref, tag.IsTag)
	}

	if laufend := runs.Runs[2]; laufend.Failed {
		t.Error("ein noch laufender Lauf ist als fehlgeschlagen markiert")
	}
}

// Ein Lauf ohne Ende hat keine Dauer. Bei einem laufenden wäre updatedAt nur
// der Stand von eben, und ein noch nicht gestarteter trägt die Nullzeit — die
// parst fehlerfrei und ergäbe rund zweitausend Jahre Laufzeit.
func TestRunsZeigtKeineDauerOhneEnde(t *testing.T) {
	const ohneEndeJSON = `[
	 {"conclusion":"","createdAt":"2026-09-16T06:00:00Z","databaseId":1,
	  "displayTitle":"läuft","event":"push","headBranch":"main","startedAt":"2026-09-16T06:00:00Z",
	  "status":"in_progress","updatedAt":"2026-09-16T06:00:30Z","workflowName":"Tests"},
	 {"conclusion":"","createdAt":"2026-09-16T06:00:00Z","databaseId":2,
	  "displayTitle":"wartet","event":"push","headBranch":"main","startedAt":"0001-01-01T00:00:00Z",
	  "status":"queued","updatedAt":"0001-01-01T00:00:00Z","workflowName":"Tests"}
	]`

	runs := ParseRuns([]byte(ohneEndeJSON))
	if len(runs.Runs) != 2 {
		t.Fatalf("%d Läufe, erwartet 2", len(runs.Runs))
	}
	for _, lauf := range runs.Runs {
		if lauf.Seconds != 0 {
			t.Errorf("Lauf %d (%s): Dauer %ds, erwartet 0", lauf.ID, lauf.Status, lauf.Seconds)
		}
	}
}

func TestRunsFragtOhneLogAb(t *testing.T) {
	runner := &fakeRunner{answers: []fakeAnswer{{prefix: "gh run list", out: runsJSON}}}
	runner.client().FetchRuns(context.Background())

	if len(runner.calls) != 1 {
		t.Fatalf("%d Aufrufe, erwartet 1: %v", len(runner.calls), runner.calls)
	}
	// Das Log ist teuer und wird nur auf Anforderung geholt, nie beim Laden
	// der Karte.
	if runner.calls[0] == "gh run view --log-failed" {
		t.Errorf("die Lauf-Karte holt Logs: %v", runner.calls)
	}
}

func TestRunsOhneLaeufeIstEinGueltigerZustand(t *testing.T) {
	runs := ParseRuns([]byte(`[]`))

	if runs.State != StateOK {
		t.Fatalf("Zustand %q, erwartet ok", runs.State)
	}
	if runs.Runs == nil {
		t.Fatal("leere Liste ist null statt []")
	}
}
