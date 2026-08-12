package review

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// newLocalDir legt eine projekteigene Struktur mit results/ an, wie sie die
// Oberfläche anlegt.
func newLocalDir(t *testing.T) string {
	t.Helper()
	localDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(localDir, ResultsDirName), 0o755); err != nil {
		t.Fatalf("results anlegen: %v", err)
	}
	return localDir
}

func day(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(DateLayout, value)
	if err != nil {
		t.Fatalf("Datum: %v", err)
	}
	return parsed
}

func TestCreateRunLegtVerzeichnisUndDateiAn(t *testing.T) {
	localDir := newLocalDir(t)

	runDir, err := CreateRun(localDir, day(t, "2026-08-12"), []string{"python", "go"}, []Entry{
		{Name: "semgrep", Kind: KindTool},
		{Name: "review-secret-scanning", Kind: KindAI},
	})
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if filepath.Base(runDir) != "2026-08-12" {
		t.Errorf("Verzeichnis = %q, erwartet 2026-08-12", filepath.Base(runDir))
	}
	if !isDir(filepath.Join(runDir, EntriesDirName)) {
		t.Error("entries/ fehlt")
	}

	run, err := ReadRun(runDir)
	if err != nil {
		t.Fatalf("ReadRun: %v", err)
	}
	if run.SchemaVersion != SchemaVersion {
		t.Errorf("SchemaVersion = %d, erwartet %d", run.SchemaVersion, SchemaVersion)
	}
	if run.State != StateCreated {
		t.Errorf("State = %q, erwartet %q", run.State, StateCreated)
	}
	if len(run.Entries) != 2 {
		t.Fatalf("%d Einträge, erwartet 2", len(run.Entries))
	}
	// Der Zustand der Auswahl wird gesetzt, nicht vom Aufrufer übernommen.
	for _, entry := range run.Entries {
		if entry.State != StateStart {
			t.Errorf("%s hat State %q, erwartet %q", entry.Name, entry.State, StateStart)
		}
	}
	if run.Entries[0].Kind != KindTool || run.Entries[1].Kind != KindAI {
		t.Errorf("Arten falsch übernommen: %+v", run.Entries)
	}
}

// Ein Tag, ein Lauf. Der zweite Versuch muss auffallen und darf nichts anfassen.
func TestCreateRunBrichtBeiVorhandenemVerzeichnisAb(t *testing.T) {
	localDir := newLocalDir(t)
	entries := []Entry{{Name: "semgrep", Kind: KindTool}}

	runDir, err := CreateRun(localDir, day(t, "2026-08-12"), nil, entries)
	if err != nil {
		t.Fatalf("erster CreateRun: %v", err)
	}
	before, err := os.ReadFile(filepath.Join(runDir, RunFileName))
	if err != nil {
		t.Fatalf("run.json lesen: %v", err)
	}

	if _, err := CreateRun(localDir, day(t, "2026-08-12"), nil, entries); err == nil {
		t.Fatal("zweiter Lauf am selben Tag wurde nicht abgewiesen")
	}

	after, err := os.ReadFile(filepath.Join(runDir, RunFileName))
	if err != nil {
		t.Fatalf("run.json lesen: %v", err)
	}
	if string(before) != string(after) {
		t.Error("der abgewiesene Versuch hat die vorhandene run.json verändert")
	}
}

// Der Name wird zu einem Dateinamen unter entries/ und darf nicht aus dem
// Verzeichnis herausführen.
func TestCreateRunWeistUnzulaessigeNamenAb(t *testing.T) {
	for _, name := range []string{"../böse", "mit/schrägstrich", "", ".versteckt"} {
		localDir := newLocalDir(t)
		_, err := CreateRun(localDir, day(t, "2026-08-12"), nil, []Entry{{Name: name, Kind: KindTool}})
		if err == nil {
			t.Errorf("Name %q wurde angenommen", name)
		}
	}
}

func TestCreateRunWeistUnbekannteArtAb(t *testing.T) {
	localDir := newLocalDir(t)

	if _, err := CreateRun(localDir, day(t, "2026-08-12"), nil, []Entry{{Name: "semgrep", Kind: "magie"}}); err == nil {
		t.Error("unbekannte Art wurde angenommen")
	}
}

func TestCreateRunWeistDoppelteEintraegeAb(t *testing.T) {
	localDir := newLocalDir(t)

	_, err := CreateRun(localDir, day(t, "2026-08-12"), nil, []Entry{
		{Name: "semgrep", Kind: KindTool},
		{Name: "semgrep", Kind: KindTool},
	})
	if err == nil {
		t.Error("doppelter Eintrag wurde angenommen")
	}
}

func TestCreateRunOhneAuswahl(t *testing.T) {
	localDir := newLocalDir(t)

	if _, err := CreateRun(localDir, day(t, "2026-08-12"), nil, nil); err == nil {
		t.Error("leere Auswahl wurde angenommen")
	}
}

// Fehlt results/, wird es nicht stillschweigend angelegt: dann stimmt an der
// projekteigenen Struktur etwas nicht, und das gehört gemeldet.
func TestCreateRunOhneResultsVerzeichnis(t *testing.T) {
	if _, err := CreateRun(t.TempDir(), day(t, "2026-08-12"), nil, []Entry{{Name: "semgrep", Kind: KindTool}}); err == nil {
		t.Error("fehlendes results/ wurde nicht gemeldet")
	}
}

func TestListRunsJuengsterZuerst(t *testing.T) {
	localDir := newLocalDir(t)
	for _, name := range []string{"2026-08-10", "2026-08-12", "2026-08-11"} {
		if _, err := CreateRun(localDir, day(t, name), nil, []Entry{{Name: "semgrep", Kind: KindTool}}); err != nil {
			t.Fatalf("CreateRun %s: %v", name, err)
		}
	}

	runs, err := ListRuns(localDir)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	want := []string{"2026-08-12", "2026-08-11", "2026-08-10"}
	if len(runs) != len(want) {
		t.Fatalf("%d Läufe, erwartet %d", len(runs), len(want))
	}
	for index, name := range want {
		if runs[index].Name != name {
			t.Errorf("Platz %d ist %q, erwartet %q", index, runs[index].Name, name)
		}
		if !runs[index].HasRunFile || runs[index].EntryCount != 1 {
			t.Errorf("%s: HasRunFile=%v EntryCount=%d", name, runs[index].HasRunFile, runs[index].EntryCount)
		}
	}
}

// Unter results/ liegen auch Verzeichnisse aus der Zeit vor diesem Modell. Sie
// sollen sichtbar bleiben, statt stillschweigend zu fehlen.
func TestListRunsZeigtVerzeichnisseOhneRunDatei(t *testing.T) {
	localDir := newLocalDir(t)
	if err := os.Mkdir(filepath.Join(ResultsDir(localDir), "dependency-cve"), 0o755); err != nil {
		t.Fatalf("Altverzeichnis anlegen: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ResultsDir(localDir), "README.md"), []byte("# results\n"), 0o644); err != nil {
		t.Fatalf("README anlegen: %v", err)
	}

	runs, err := ListRuns(localDir)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("%d Einträge, erwartet 1 — Dateien gehören nicht dazu: %+v", len(runs), runs)
	}
	if runs[0].Name != "dependency-cve" || runs[0].HasRunFile {
		t.Errorf("unerwarteter Eintrag: %+v", runs[0])
	}
}

// Buchstaben liegen beim Sortieren über Ziffern. Ein reines Absteigend-nach-
// Namen schöbe die Ergebnisfamilien von früher vor die eigentlichen Läufe.
func TestListRunsSortiertLaeufeVorAltverzeichnisse(t *testing.T) {
	localDir := newLocalDir(t)
	for _, name := range []string{"dependency-cve", "secret-scanning"} {
		if err := os.Mkdir(filepath.Join(ResultsDir(localDir), name), 0o755); err != nil {
			t.Fatalf("Altverzeichnis %s: %v", name, err)
		}
	}
	for _, name := range []string{"2026-08-10", "2026-08-12"} {
		if _, err := CreateRun(localDir, day(t, name), nil, []Entry{{Name: "ruff", Kind: KindTool}}); err != nil {
			t.Fatalf("CreateRun %s: %v", name, err)
		}
	}

	runs, err := ListRuns(localDir)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	var got []string
	for _, run := range runs {
		got = append(got, run.Name)
	}
	want := []string{"2026-08-12", "2026-08-10", "dependency-cve", "secret-scanning"}
	for index, name := range want {
		if index >= len(got) || got[index] != name {
			t.Fatalf("Reihenfolge = %v, erwartet %v", got, want)
		}
	}
}

func TestListRunsOhneResultsVerzeichnis(t *testing.T) {
	runs, err := ListRuns(t.TempDir())
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 0 {
		t.Errorf("%d Läufe, erwartet 0", len(runs))
	}
}

// Die Schlüssel sind englisch und stabil: sie stehen in der Doku und werden von
// den Einträgen gelesen.
func TestRunJSONSchluessel(t *testing.T) {
	localDir := newLocalDir(t)
	runDir, err := CreateRun(localDir, day(t, "2026-08-12"), []string{"python"}, []Entry{{Name: "ruff", Kind: KindTool}})
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(runDir, RunFileName))
	if err != nil {
		t.Fatalf("run.json lesen: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("run.json ist kein JSON: %v", err)
	}
	for _, key := range []string{"schemaVersion", "created", "state", "languages", "entries"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("Schlüssel %q fehlt in run.json:\n%s", key, data)
		}
	}
}
