package review

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func jobs(states ...State) []JobStatus {
	list := make([]JobStatus, 0, len(states))
	for index, state := range states {
		list = append(list, JobStatus{Job: string(rune('a'+index)) + "-job", State: state})
	}
	return list
}

// Die Regel aus dem Laufmodell, Fall für Fall. Regel 0 steht voran: ein
// laufender Eintrag darf nicht über die Auffangregel als skipped gelten.
func TestDeriveEntryState(t *testing.T) {
	fälle := []struct {
		name string
		jobs []JobStatus
		want State
	}{
		{"ein Job läuft, einer ist fertig", jobs(StateRunning, StateDone), StateRunning},
		{"ein Job steht noch aus", jobs(StateStart, StateDone), StateRunning},
		{"fertig und fehlgeschlagen", jobs(StateDone, StateFailed), StateFailed},
		{"fertig und übersprungen", jobs(StateDone, StateSkipped), StateDone},
		{"alles übersprungen", jobs(StateSkipped, StateSkipped), StateSkipped},
		{"gar kein Job", nil, StateSkipped},
		{"läuft noch, obwohl schon einer fehlschlug", jobs(StateFailed, StateRunning), StateRunning},
	}

	for _, fall := range fälle {
		if got := DeriveEntryState(fall.jobs); got != fall.want {
			t.Errorf("%s: %q, erwartet %q", fall.name, got, fall.want)
		}
	}
}

func TestWriteEntryStatusSchreibtSchluesselUndBleibtLesbar(t *testing.T) {
	localDir := newLocalDir(t)
	runDir, err := CreateRun(localDir, day(t, "2026-08-16"), []string{"python"}, []Entry{{Name: "trivy", Kind: KindTool}})
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	exitCode, findings := 1, 12
	status := EntryStatus{
		Name:     "trivy",
		Kind:     KindTool,
		State:    StateDone,
		Started:  "2026-08-16T09:12:04+02:00",
		Finished: "2026-08-16T09:13:41+02:00",
		Jobs: []JobStatus{
			{Job: "trivy-fs", State: StateDone, ExitCode: &exitCode, SARIF: "raw/trivy-fs.sarif", Findings: &findings},
			{Job: "trivy-config", State: StateSkipped, Reason: "Sprache nicht gewählt"},
		},
	}
	if err := WriteEntryStatus(runDir, status); err != nil {
		t.Fatalf("WriteEntryStatus: %v", err)
	}

	data, err := os.ReadFile(EntryFile(runDir, "trivy"))
	if err != nil {
		t.Fatalf("Eintragsdatei lesen: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Eintragsdatei ist kein JSON: %v", err)
	}
	for _, key := range []string{"schemaVersion", "name", "kind", "state", "started", "finished", "jobs"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("Schlüssel %q fehlt:\n%s", key, data)
		}
	}

	// Ein übersprungener Job hat keinen Exit-Code — 0 hieße hier „gemessen".
	list := raw["jobs"].([]any)
	skipped := list[1].(map[string]any)
	if _, ok := skipped["exitCode"]; ok {
		t.Errorf("der übersprungene Job trägt einen Exit-Code:\n%s", data)
	}
	if skipped["reason"] != "Sprache nicht gewählt" {
		t.Errorf("Grund fehlt:\n%s", data)
	}

	gelesen, err := ReadEntryStatus(runDir, "trivy")
	if err != nil {
		t.Fatalf("ReadEntryStatus: %v", err)
	}
	if gelesen.State != StateDone || len(gelesen.Jobs) != 2 || *gelesen.Jobs[0].Findings != 12 {
		t.Errorf("nicht rundlaufend gelesen: %+v", gelesen)
	}

	// Nach dem atomaren Schreiben darf keine Temp-Datei liegen bleiben.
	dir, err := os.ReadDir(filepath.Join(runDir, EntriesDirName))
	if err != nil {
		t.Fatalf("entries/ lesen: %v", err)
	}
	if len(dir) != 1 {
		t.Errorf("%d Dateien unter entries/, erwartet 1: %+v", len(dir), dir)
	}
}

// Fehlt die Datei, ist der Eintrag noch nicht gestartet.
func TestEntryStateOhneDatei(t *testing.T) {
	if got := EntryState(t.TempDir(), "ruff"); got != StateStart {
		t.Errorf("State = %q, erwartet %q", got, StateStart)
	}
}

func TestDeriveRunState(t *testing.T) {
	localDir := newLocalDir(t)
	runDir, err := CreateRun(localDir, day(t, "2026-08-16"), []string{"python"}, []Entry{
		{Name: "ruff", Kind: KindTool},
		{Name: "review-secret-scanning", Kind: KindAI},
	})
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	run, err := ReadRun(runDir)
	if err != nil {
		t.Fatalf("ReadRun: %v", err)
	}

	if got := DeriveRunState(runDir, run); got != StateCreated {
		t.Errorf("frisch angelegt = %q, erwartet %q", got, StateCreated)
	}

	if err := WriteEntryStatus(runDir, EntryStatus{Name: "ruff", Kind: KindTool, State: StateRunning}); err != nil {
		t.Fatalf("WriteEntryStatus: %v", err)
	}
	if got := DeriveRunState(runDir, run); got != StateRunning {
		t.Errorf("ein laufender Eintrag = %q, erwartet %q", got, StateRunning)
	}

	// Der ai-Eintrag steht weiterhin auf start: seine Datei entsteht erst,
	// wenn der Assistent seinen Command ausführt.
	if err := WriteEntryStatus(runDir, EntryStatus{Name: "ruff", Kind: KindTool, State: StateDone}); err != nil {
		t.Fatalf("WriteEntryStatus: %v", err)
	}
	if got := DeriveRunState(runDir, run); got != StateRunning {
		t.Errorf("mit ausstehendem ai-Eintrag = %q, erwartet %q", got, StateRunning)
	}

	if err := WriteEntryStatus(runDir, EntryStatus{Name: "review-secret-scanning", Kind: KindAI, State: StateDone}); err != nil {
		t.Fatalf("WriteEntryStatus: %v", err)
	}
	if got := DeriveRunState(runDir, run); got != StateDone {
		t.Errorf("alle durch = %q, erwartet %q", got, StateDone)
	}
}

// Weichen run.json und entries/ voneinander ab, gilt entries/. Das Anlegen
// schreibt created; danach fasst niemand die Datei mehr an.
func TestListRunsNimmtZustandAusEntries(t *testing.T) {
	localDir := newLocalDir(t)
	runDir, err := CreateRun(localDir, day(t, "2026-08-16"), nil, []Entry{{Name: "ruff", Kind: KindTool}})
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if err := WriteEntryStatus(runDir, EntryStatus{Name: "ruff", Kind: KindTool, State: StateFailed}); err != nil {
		t.Fatalf("WriteEntryStatus: %v", err)
	}

	runs, err := ListRuns(localDir)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if runs[0].State != StateDone {
		t.Errorf("Laufzustand = %q, erwartet %q — ein Fehlschlag steht am Eintrag, nicht am Lauf", runs[0].State, StateDone)
	}

	run, err := ReadRun(runDir)
	if err != nil {
		t.Fatalf("ReadRun: %v", err)
	}
	if run.State != StateCreated {
		t.Errorf("run.json wurde verändert: State = %q", run.State)
	}
}
