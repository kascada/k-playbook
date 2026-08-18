package review

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// sarifMit erzeugt ein SARIF-Dokument mit der angegebenen Zahl an Befunden.
func sarifMit(count int) string {
	results := make([]string, 0, count)
	for index := 0; index < count; index++ {
		results = append(results, fmt.Sprintf(`{"ruleId":"R%d"}`, index))
	}
	return fmt.Sprintf(`{"version":"2.1.0","runs":[{"results":[%s]}]}`, strings.Join(results, ","))
}

// fakeTool legt ein Programm an, das sich wie ein Scanner verhält. Ein echtes
// Werkzeug im Test hieße Netz, Laufzeit und ein fremdes Ergebnis.
func fakeTool(t *testing.T, name string, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatalf("Attrappe %s: %v", name, err)
	}
	return path
}

// schreibtSARIF ist eine Attrappe, die nach ihrem ersten Argument schreibt und
// mit dem angegebenen Code endet.
func schreibtSARIF(t *testing.T, name string, findings int, exitCode int) string {
	t.Helper()
	return fakeTool(t, name, fmt.Sprintf("cat > \"$1\" <<'SARIF'\n%s\nSARIF\nexit %d", sarifMit(findings), exitCode))
}

func nativeScanner(job string, tool string, args ...string) Scanner {
	return Scanner{
		Job:       job,
		Tool:      tool,
		Languages: "*",
		SARIF:     SARIFNative,
		Output:    OutputFile,
		Workdir:   WorkdirTarget,
		Timeout:   30 * time.Second,
		Args:      args,
	}
}

// moduleScanner ist ein Job, der ein Modulverzeichnis braucht: er läuft je
// gefundenem Modul einmal.
func moduleScanner(job string, tool string, args ...string) Scanner {
	scanner := nativeScanner(job, tool, args...)
	scanner.Workdir = WorkdirModule
	return scanner
}

func neuerLauf(t *testing.T, entries ...Entry) string {
	t.Helper()
	runDir, err := CreateRun(newLocalDir(t), day(t, "2026-08-16"), []string{"python", "go"}, entries)
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	return runDir
}

func führeAus(t *testing.T, options Options, entries ...Entry) []EntryStatus {
	t.Helper()
	if options.Target == "" {
		options.Target = t.TempDir()
	}
	if options.Languages == nil {
		options.Languages = []string{"python", "go"}
	}
	statuses, err := Execute(context.Background(), entries, options)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	return statuses
}

// Der leichteste Weg, es falsch zu machen: fast alle Scanner enden mit einem
// Code ungleich 0, wenn sie etwas gefunden haben. Das ist ihr Ergebnis.
func TestExecuteBefundeSindKeinFehlschlag(t *testing.T) {
	runDir := neuerLauf(t, Entry{Name: "gitleaks", Kind: KindTool})
	tool := schreibtSARIF(t, "gitleaks", 3, 1)

	statuses := führeAus(t, Options{
		RunDir:   runDir,
		Scanners: []Scanner{nativeScanner("gitleaks", "gitleaks", "{out}")},
		Tools:    map[string]Tool{"gitleaks": {Path: tool}},
	}, Entry{Name: "gitleaks", Kind: KindTool})

	status := statuses[0]
	if status.State != StateDone {
		t.Fatalf("Eintrag = %q, erwartet %q: %+v", status.State, StateDone, status.Jobs)
	}
	job := status.Jobs[0]
	if job.State != StateDone {
		t.Errorf("Job = %q, erwartet %q (Grund: %s)", job.State, StateDone, job.Reason)
	}
	if job.ExitCode == nil || *job.ExitCode != 1 {
		t.Errorf("Exit-Code = %v, erwartet 1", job.ExitCode)
	}
	if job.Findings == nil || *job.Findings != 3 {
		t.Errorf("Befunde = %v, erwartet 3", job.Findings)
	}
	if job.SARIF != "raw/gitleaks.sarif" {
		t.Errorf("SARIF = %q, erwartet raw/gitleaks.sarif", job.SARIF)
	}
	if _, err := os.Stat(filepath.Join(runDir, RawDirName, "gitleaks.sarif")); err != nil {
		t.Errorf("Datei unter raw/ fehlt: %v", err)
	}
	if status.Started == "" || status.Finished == "" {
		t.Errorf("Start- oder Endzeit fehlt: %+v", status)
	}
}

// Ohne SARIF gibt es kein Ergebnis — auch bei Exit-Code 0.
func TestExecuteOhneSARIFIstFehlschlag(t *testing.T) {
	runDir := neuerLauf(t, Entry{Name: "gitleaks", Kind: KindTool})
	tool := fakeTool(t, "gitleaks", "echo 'unknown flag: --report-format' >&2\nexit 2")

	statuses := führeAus(t, Options{
		RunDir:   runDir,
		Scanners: []Scanner{nativeScanner("gitleaks", "gitleaks", "{out}")},
		Tools:    map[string]Tool{"gitleaks": {Path: tool}},
	}, Entry{Name: "gitleaks", Kind: KindTool})

	if statuses[0].State != StateFailed {
		t.Fatalf("Eintrag = %q, erwartet %q", statuses[0].State, StateFailed)
	}
	job := statuses[0].Jobs[0]
	if job.State != StateFailed || !strings.Contains(job.Reason, "unknown flag") {
		t.Errorf("Grund nennt die Meldung des Werkzeugs nicht: %+v", job)
	}
}

// Ein fehlendes Werkzeug ist kein Fehlschlag, und es bricht den Lauf nicht ab.
func TestExecuteUeberspringtOhneWerkzeug(t *testing.T) {
	runDir := neuerLauf(t,
		Entry{Name: "gosec", Kind: KindTool},
		Entry{Name: "gitleaks", Kind: KindTool},
	)
	tool := schreibtSARIF(t, "gitleaks", 0, 0)

	statuses := führeAus(t, Options{
		RunDir: runDir,
		Scanners: []Scanner{
			nativeScanner("gosec", "gosec", "{out}"),
			nativeScanner("gitleaks", "gitleaks", "{out}"),
		},
		Tools: map[string]Tool{
			"gosec":    {Reason: "Werkzeug gosec ist nicht installiert"},
			"gitleaks": {Path: tool},
		},
	},
		Entry{Name: "gosec", Kind: KindTool},
		Entry{Name: "gitleaks", Kind: KindTool},
	)

	if statuses[0].State != StateSkipped {
		t.Errorf("gosec = %q, erwartet %q", statuses[0].State, StateSkipped)
	}
	if reason := statuses[0].Jobs[0].Reason; !strings.Contains(reason, "nicht installiert") {
		t.Errorf("Grund = %q", reason)
	}
	if statuses[1].State != StateDone {
		t.Errorf("gitleaks = %q, erwartet %q — ein fehlendes Werkzeug bricht nichts ab", statuses[1].State, StateDone)
	}
	if _, err := os.Stat(filepath.Join(runDir, RawDirName, "gosec.sarif")); !os.IsNotExist(err) {
		t.Errorf("der übersprungene Job hat eine Datei unter raw/ hinterlassen")
	}
}

// syft: ein Werkzeug ohne Job. Es erzeugt eine SBOM, keine Befunde — und die
// Oberfläche bietet es weiterhin zur Auswahl an.
func TestExecuteWerkzeugOhneJob(t *testing.T) {
	runDir := neuerLauf(t, Entry{Name: "syft", Kind: KindTool})

	statuses := führeAus(t, Options{
		RunDir:   runDir,
		Scanners: []Scanner{nativeScanner("gitleaks", "gitleaks", "{out}")},
		Tools:    map[string]Tool{"syft": {Path: "/bin/true"}},
	}, Entry{Name: "syft", Kind: KindTool})

	if statuses[0].State != StateSkipped {
		t.Errorf("syft = %q, erwartet %q", statuses[0].State, StateSkipped)
	}
	if len(statuses[0].Jobs) != 0 {
		t.Errorf("syft hat Jobs bekommen: %+v", statuses[0].Jobs)
	}
	// Ohne Job gibt es keinen Ort, an dem der Grund sonst stünde.
	if !strings.Contains(statuses[0].Reason, "kein Scan-Job") {
		t.Errorf("Grund = %q", statuses[0].Reason)
	}
	if _, err := os.Stat(filepath.Join(runDir, RawDirName)); !os.IsNotExist(err) {
		t.Errorf("raw/ wurde angelegt, obwohl kein Job lief")
	}
}

// convert und none laufen nicht, solange die Konverter fehlen: sonst läge
// unter raw/ rohes JSON mit der Endung .sarif.
func TestExecuteUeberspringtNichtNativeUndFremdeSprachen(t *testing.T) {
	runDir := neuerLauf(t, Entry{Name: "trufflehog", Kind: KindTool}, Entry{Name: "gosec", Kind: KindTool})
	tool := schreibtSARIF(t, "egal", 0, 0)

	convert := nativeScanner("trufflehog-git", "trufflehog", "{out}")
	convert.SARIF = SARIFConvert
	fremd := nativeScanner("gosec", "gosec", "{out}")
	fremd.Languages = "go"

	statuses := führeAus(t, Options{
		RunDir:    runDir,
		Languages: []string{"python"},
		Scanners:  []Scanner{convert, fremd},
		Tools:     map[string]Tool{"trufflehog": {Path: tool}, "gosec": {Path: tool}},
	}, Entry{Name: "trufflehog", Kind: KindTool}, Entry{Name: "gosec", Kind: KindTool})

	if reason := statuses[0].Jobs[0].Reason; !strings.Contains(reason, "Konverter") {
		t.Errorf("trufflehog-Grund = %q", reason)
	}
	if reason := statuses[1].Jobs[0].Reason; !strings.Contains(reason, "Sprache") {
		t.Errorf("gosec-Grund = %q", reason)
	}
	for _, status := range statuses {
		if status.State != StateSkipped {
			t.Errorf("%s = %q, erwartet %q", status.Name, status.State, StateSkipped)
		}
	}
}

// Ein hängender Scanner darf den Lauf nicht anhalten.
func TestExecuteZeitueberschreitung(t *testing.T) {
	runDir := neuerLauf(t, Entry{Name: "semgrep", Kind: KindTool})
	tool := fakeTool(t, "semgrep", "sleep 30")

	scanner := nativeScanner("semgrep", "semgrep", "{out}")
	scanner.Timeout = 150 * time.Millisecond

	statuses := führeAus(t, Options{
		RunDir:   runDir,
		Scanners: []Scanner{scanner},
		Tools:    map[string]Tool{"semgrep": {Path: tool}},
	}, Entry{Name: "semgrep", Kind: KindTool})

	job := statuses[0].Jobs[0]
	if job.State != StateFailed || !strings.Contains(job.Reason, "Zeitüberschreitung") {
		t.Errorf("Job = %+v, erwartet einen Fehlschlag wegen Zeitüberschreitung", job)
	}
}

// Der Ausgang des Eintrags ist die Kurzfassung; welcher Job was hatte, steht
// weiterhin je Job in der Datei.
func TestExecuteEintragszustandAusJobs(t *testing.T) {
	runDir := neuerLauf(t, Entry{Name: "gitleaks", Kind: KindTool})
	gut := schreibtSARIF(t, "gitleaks", 2, 1)

	erster := nativeScanner("gitleaks-git", "gitleaks", "{out}")
	zweiter := nativeScanner("gitleaks-dir", "gitleaks", "{out}")
	zweiter.Languages = "rust"

	statuses := führeAus(t, Options{
		RunDir:    runDir,
		Languages: []string{"python"},
		Scanners:  []Scanner{erster, zweiter},
		Tools:     map[string]Tool{"gitleaks": {Path: gut}},
	}, Entry{Name: "gitleaks", Kind: KindTool})

	// done über skipped: sonst versteckte der übersprungene zweite Job die
	// Datei, die der erste geschrieben hat.
	if statuses[0].State != StateDone {
		t.Errorf("Eintrag = %q, erwartet %q", statuses[0].State, StateDone)
	}
	if statuses[0].Jobs[1].State != StateSkipped {
		t.Errorf("zweiter Job = %q, erwartet %q", statuses[0].Jobs[1].State, StateSkipped)
	}
}

// Zwei Einträge gleichzeitig: jede Datei hat genau einen Schreiber, keine
// überschreibt die andere.
func TestExecuteParallelSchreibtVollstaendig(t *testing.T) {
	entries := []Entry{}
	scanners := []Scanner{}
	tools := map[string]Tool{}
	for index := 0; index < 6; index++ {
		name := fmt.Sprintf("werkzeug%d", index)
		entries = append(entries, Entry{Name: name, Kind: KindTool})
		scanners = append(scanners, nativeScanner(name, name, "{out}"))
		tools[name] = Tool{Path: schreibtSARIF(t, name, index, 0)}
	}
	runDir := neuerLauf(t, entries...)

	statuses := führeAus(t, Options{
		RunDir:   runDir,
		Scanners: scanners,
		Tools:    tools,
		Parallel: 3,
	}, entries...)

	for index, status := range statuses {
		if status.State != StateDone {
			t.Errorf("%s = %q, erwartet %q", status.Name, status.State, StateDone)
		}
		gelesen, err := ReadEntryStatus(runDir, status.Name)
		if err != nil {
			t.Fatalf("%s lesen: %v", status.Name, err)
		}
		if gelesen.State != StateDone || len(gelesen.Jobs) != 1 {
			t.Errorf("%s unvollständig geschrieben: %+v", status.Name, gelesen)
		}
		if gelesen.Jobs[0].Findings == nil || *gelesen.Jobs[0].Findings != index {
			t.Errorf("%s: Befunde = %v, erwartet %d — hier hat ein Schreiber den anderen überholt",
				status.Name, gelesen.Jobs[0].Findings, index)
		}
	}
}

// Der Fortschritt ist von außen lesbar, während der Lauf noch läuft.
func TestExecuteFortschrittWaehrendDesLaufsLesbar(t *testing.T) {
	runDir := neuerLauf(t, Entry{Name: "gitleaks", Kind: KindTool})
	tool := fakeTool(t, "gitleaks", fmt.Sprintf("sleep 0.3\ncat > \"$1\" <<'SARIF'\n%s\nSARIF", sarifMit(1)))
	run, err := ReadRun(runDir)
	if err != nil {
		t.Fatalf("ReadRun: %v", err)
	}

	var mutex sync.Mutex
	gesehen := map[State]bool{}
	var laufzustand State

	führeAus(t, Options{
		RunDir:   runDir,
		Scanners: []Scanner{nativeScanner("gitleaks", "gitleaks", "{out}")},
		Tools:    map[string]Tool{"gitleaks": {Path: tool}},
		Progress: func(entry string, job JobStatus) {
			if job.State != StateRunning {
				return
			}
			// Ein zweiter Leser sieht nur die Platte, nicht den Speicher.
			mutex.Lock()
			defer mutex.Unlock()
			gesehen[EntryState(runDir, entry)] = true
			laufzustand = DeriveRunState(runDir, run)
		},
	}, Entry{Name: "gitleaks", Kind: KindTool})

	if !gesehen[StateRunning] {
		t.Errorf("während des Laufs stand unter entries/ nicht %q, sondern %v", StateRunning, gesehen)
	}
	if laufzustand != StateRunning {
		t.Errorf("Laufzustand während des Laufs = %q, erwartet %q", laufzustand, StateRunning)
	}
}

// Ein zweiter Aufruf über denselben Eintrag überschreibt, statt daneben zu
// schreiben oder abzubrechen.
func TestExecuteIstWiederholbar(t *testing.T) {
	runDir := neuerLauf(t, Entry{Name: "gitleaks", Kind: KindTool})
	scanner := nativeScanner("gitleaks", "gitleaks", "{out}")
	entry := Entry{Name: "gitleaks", Kind: KindTool}

	führeAus(t, Options{
		RunDir:   runDir,
		Scanners: []Scanner{scanner},
		Tools:    map[string]Tool{"gitleaks": {Path: schreibtSARIF(t, "gitleaks", 5, 1)}},
	}, entry)

	statuses := führeAus(t, Options{
		RunDir:   runDir,
		Scanners: []Scanner{scanner},
		Tools:    map[string]Tool{"gitleaks": {Path: schreibtSARIF(t, "gitleaks", 2, 1)}},
	}, entry)

	if *statuses[0].Jobs[0].Findings != 2 {
		t.Errorf("Befunde = %d, erwartet 2 — der zweite Lauf hat nicht überschrieben", *statuses[0].Jobs[0].Findings)
	}
	raw, err := os.ReadDir(filepath.Join(runDir, RawDirName))
	if err != nil {
		t.Fatalf("raw/ lesen: %v", err)
	}
	if len(raw) != 1 {
		t.Errorf("%d Dateien unter raw/, erwartet 1", len(raw))
	}
	entries, err := os.ReadDir(filepath.Join(runDir, EntriesDirName))
	if err != nil {
		t.Fatalf("entries/ lesen: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("%d Dateien unter entries/, erwartet 1", len(entries))
	}

	// Eine alte Datei darf nicht als Ergebnis eines neuen Laufs durchgehen.
	statuses = führeAus(t, Options{
		RunDir:   runDir,
		Scanners: []Scanner{scanner},
		Tools:    map[string]Tool{"gitleaks": {Path: fakeTool(t, "gitleaks", "exit 3")}},
	}, entry)
	if statuses[0].Jobs[0].State != StateFailed {
		t.Errorf("Job = %q, erwartet %q — die Datei des vorigen Laufs galt als Ergebnis",
			statuses[0].Jobs[0].State, StateFailed)
	}
}

// run.json gehört dem Anlegen. Das Ausführen fasst sie nicht an — deshalb
// braucht es auch keine Sperre je Lauf.
func TestExecuteLaesstRunJSONUnberuehrt(t *testing.T) {
	runDir := neuerLauf(t, Entry{Name: "gitleaks", Kind: KindTool})
	runFile := filepath.Join(runDir, RunFileName)
	vorher, err := os.ReadFile(runFile)
	if err != nil {
		t.Fatalf("run.json lesen: %v", err)
	}
	info, err := os.Stat(runFile)
	if err != nil {
		t.Fatalf("run.json prüfen: %v", err)
	}

	führeAus(t, Options{
		RunDir:   runDir,
		Scanners: []Scanner{nativeScanner("gitleaks", "gitleaks", "{out}")},
		Tools:    map[string]Tool{"gitleaks": {Path: schreibtSARIF(t, "gitleaks", 1, 1)}},
	}, Entry{Name: "gitleaks", Kind: KindTool})

	nachher, err := os.ReadFile(runFile)
	if err != nil {
		t.Fatalf("run.json lesen: %v", err)
	}
	if string(vorher) != string(nachher) {
		t.Error("das Ausführen hat run.json verändert")
	}
	danach, err := os.Stat(runFile)
	if err != nil {
		t.Fatalf("run.json prüfen: %v", err)
	}
	if !danach.ModTime().Equal(info.ModTime()) {
		t.Error("das Ausführen hat run.json angefasst")
	}
}

// Der Job läuft im Zielverzeichnis, nicht dort, wo jemand gerade steht.
func TestExecuteArbeitetImZielverzeichnis(t *testing.T) {
	runDir := neuerLauf(t, Entry{Name: "gitleaks", Kind: KindTool})
	target := t.TempDir()
	tool := fakeTool(t, "gitleaks", fmt.Sprintf("printf '{\"version\":\"2.1.0\",\"runs\":[{\"results\":[]}]}' > \"$1\"\ntest \"$(pwd -P)\" = \"$(cd %q && pwd -P)\"", target))

	statuses := führeAus(t, Options{
		RunDir:   runDir,
		Target:   target,
		Scanners: []Scanner{nativeScanner("gitleaks", "gitleaks", "{out}")},
		Tools:    map[string]Tool{"gitleaks": {Path: tool}},
	}, Entry{Name: "gitleaks", Kind: KindTool})

	if code := statuses[0].Jobs[0].ExitCode; code == nil || *code != 0 {
		t.Errorf("die Attrappe lief nicht im Zielverzeichnis: Exit-Code %v", code)
	}
}

// Ein ai-Eintrag wird von einem Assistenten ausgeführt, nicht hier.
func TestExecuteWeistAIEintragAb(t *testing.T) {
	runDir := neuerLauf(t, Entry{Name: "review-secret-scanning", Kind: KindAI})

	_, err := Execute(context.Background(), []Entry{{Name: "review-secret-scanning", Kind: KindAI}}, Options{
		RunDir: runDir,
		Target: t.TempDir(),
	})
	if err == nil {
		t.Fatal("ein ai-Eintrag wurde zur Ausführung angenommen")
	}
	if _, statErr := os.Stat(EntryFile(runDir, "review-secret-scanning")); !os.IsNotExist(statErr) {
		t.Error("für den ai-Eintrag wurde eine Datei unter entries/ angelegt")
	}
}

// Die Ausgabe eines Werkzeugs, das nur nach stdout schreibt, landet unter raw/.
func TestExecuteLeitetStdoutUm(t *testing.T) {
	runDir := neuerLauf(t, Entry{Name: "govulncheck", Kind: KindTool})
	tool := fakeTool(t, "govulncheck", fmt.Sprintf("printf '%%s' '%s'", sarifMit(4)))

	scanner := nativeScanner("govulncheck", "govulncheck", "-format", "sarif")
	scanner.Output = OutputStdout

	statuses := führeAus(t, Options{
		RunDir:   runDir,
		Scanners: []Scanner{scanner},
		Tools:    map[string]Tool{"govulncheck": {Path: tool}},
	}, Entry{Name: "govulncheck", Kind: KindTool})

	job := statuses[0].Jobs[0]
	if job.State != StateDone || job.Findings == nil || *job.Findings != 4 {
		t.Fatalf("Job = %+v, erwartet done mit 4 Befunden", job)
	}
}

// prueftArbeitsverzeichnis ist eine Attrappe, die nach ihrem ersten Argument
// schreibt und nur dann mit 0 endet, wenn sie im zweiten Argument steht. So
// hängt das Ergebnis am Arbeitsverzeichnis und nicht an einer Zusicherung.
func prueftArbeitsverzeichnis(t *testing.T, name string) string {
	t.Helper()
	return fakeTool(t, name, fmt.Sprintf("cat > \"$1\" <<'SARIF'\n%s\nSARIF\ntest \"$(pwd -P)\" = \"$(cd \"$2\" && pwd -P)\"", sarifMit(1)))
}

// rawDateien sind die Namen der Dateien unter raw/, sortiert.
func rawDateien(t *testing.T, runDir string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(runDir, RawDirName))
	if err != nil {
		t.Fatalf("raw/ lesen: %v", err)
	}
	names := []string{}
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names
}

// Ein Modul in der Wurzel: der Job heißt wie im Katalog, die Datei ebenso —
// sichtbar ist das geprüfte Modul trotzdem, nämlich am Job.
func TestExecuteEinModulLaesstDenNamen(t *testing.T) {
	runDir := neuerLauf(t, Entry{Name: "govulncheck", Kind: KindTool})
	target := modulBaum(t, "go.mod")

	statuses := führeAus(t, Options{
		RunDir:   runDir,
		Target:   target,
		Scanners: []Scanner{moduleScanner("govulncheck", "govulncheck", "{out}", "{module}")},
		Tools:    map[string]Tool{"govulncheck": {Path: prueftArbeitsverzeichnis(t, "govulncheck")}},
	}, Entry{Name: "govulncheck", Kind: KindTool})

	if len(statuses[0].Jobs) != 1 {
		t.Fatalf("%d Jobs, erwartet 1: %+v", len(statuses[0].Jobs), statuses[0].Jobs)
	}
	job := statuses[0].Jobs[0]
	if job.Job != "govulncheck" || job.SARIF != "raw/govulncheck.sarif" {
		t.Errorf("Name oder Datei haben sich geändert: %+v", job)
	}
	if job.Module != "." {
		t.Errorf("Modul = %q, erwartet \".\"", job.Module)
	}
	if job.State != StateDone || job.ExitCode == nil || *job.ExitCode != 0 {
		t.Errorf("der Job lief nicht im Modulverzeichnis: %+v", job)
	}
}

// Liegt das Modul unter der Wurzel, bleibt der Name ebenfalls unverändert —
// und das Feld am Job nennt es. Das ist der Fall dieses Repos.
func TestExecuteModulUnterDerWurzelStehtAmJob(t *testing.T) {
	runDir := neuerLauf(t, Entry{Name: "govulncheck", Kind: KindTool})
	target := modulBaum(t, "installer/go.mod")

	statuses := führeAus(t, Options{
		RunDir:   runDir,
		Target:   target,
		Scanners: []Scanner{moduleScanner("govulncheck", "govulncheck", "{out}", "{module}")},
		Tools:    map[string]Tool{"govulncheck": {Path: prueftArbeitsverzeichnis(t, "govulncheck")}},
	}, Entry{Name: "govulncheck", Kind: KindTool})

	job := statuses[0].Jobs[0]
	if job.Job != "govulncheck" {
		t.Errorf("Job = %q, erwartet den Namen aus dem Katalog", job.Job)
	}
	if job.Module != "installer" {
		t.Errorf("Modul = %q, erwartet installer", job.Module)
	}
	if job.State != StateDone || job.ExitCode == nil || *job.ExitCode != 0 {
		t.Errorf("der Job lief nicht im Modulverzeichnis: %+v", job)
	}
	// Die gelesene Datei ist die Auskunft, die bleibt.
	gelesen, err := ReadEntryStatus(runDir, "govulncheck")
	if err != nil {
		t.Fatalf("entries/govulncheck.json: %v", err)
	}
	if gelesen.Jobs[0].Module != "installer" {
		t.Errorf("in entries/govulncheck.json steht Modul %q", gelesen.Jobs[0].Module)
	}
}

// Mehrere Module: ein Job je Modul, mit abgeleitetem Namen und eigener Datei.
func TestExecuteFaechertJeModulAuf(t *testing.T) {
	runDir := neuerLauf(t, Entry{Name: "govulncheck", Kind: KindTool})
	target := modulBaum(t, "installer/go.mod", "werkzeuge/prüfer/go.mod")

	statuses := führeAus(t, Options{
		RunDir:   runDir,
		Target:   target,
		Scanners: []Scanner{moduleScanner("govulncheck", "govulncheck", "{out}", "{module}")},
		Tools:    map[string]Tool{"govulncheck": {Path: prueftArbeitsverzeichnis(t, "govulncheck")}},
	}, Entry{Name: "govulncheck", Kind: KindTool})

	if len(statuses[0].Jobs) != 2 {
		t.Fatalf("%d Jobs, erwartet 2: %+v", len(statuses[0].Jobs), statuses[0].Jobs)
	}
	namen := map[string]string{}
	for _, job := range statuses[0].Jobs {
		if job.State != StateDone || job.ExitCode == nil || *job.ExitCode != 0 {
			t.Errorf("Job %s lief nicht in seinem Modul: %+v", job.Job, job)
		}
		namen[job.Job] = job.Module
	}
	if namen["govulncheck-installer"] != "installer" {
		t.Errorf("Jobs = %+v, erwartet govulncheck-installer für das Modul installer", namen)
	}
	// Der Name muss als Dateiname taugen: das ü fällt weg.
	if namen["govulncheck-werkzeuge-prfer"] != "werkzeuge/prüfer" {
		t.Errorf("Jobs = %+v, erwartet einen abgeleiteten Namen für werkzeuge/prüfer", namen)
	}

	dateien := rawDateien(t, runDir)
	if len(dateien) != 2 {
		t.Errorf("Dateien unter raw/ = %v, erwartet zwei", dateien)
	}
}

// Zwei Module können auf denselben abgeleiteten Namen führen. Ohne
// Unterscheidung überschriebe der zweite Job die Datei des ersten.
func TestExecuteKollidierendeModulnamen(t *testing.T) {
	runDir := neuerLauf(t, Entry{Name: "govulncheck", Kind: KindTool})
	target := modulBaum(t, "dienst/api/go.mod", "dienst-api/go.mod")

	statuses := führeAus(t, Options{
		RunDir:   runDir,
		Target:   target,
		Scanners: []Scanner{moduleScanner("govulncheck", "govulncheck", "{out}", "{module}")},
		Tools:    map[string]Tool{"govulncheck": {Path: prueftArbeitsverzeichnis(t, "govulncheck")}},
	}, Entry{Name: "govulncheck", Kind: KindTool})

	namen := map[string]bool{}
	module := map[string]bool{}
	for _, job := range statuses[0].Jobs {
		namen[job.Job] = true
		module[job.Module] = true
		if job.State != StateDone {
			t.Errorf("Job %s = %q (%s)", job.Job, job.State, job.Reason)
		}
	}
	if len(namen) != 2 || len(module) != 2 {
		t.Errorf("Jobs = %+v, erwartet zwei Namen für zwei Module", statuses[0].Jobs)
	}
	if dateien := rawDateien(t, runDir); len(dateien) != 2 {
		t.Errorf("Dateien unter raw/ = %v, erwartet zwei", dateien)
	}
}

// Kein Modul: es fehlt der Gegenstand, nicht das Werkzeug.
func TestExecuteOhneModulIstUebersprungen(t *testing.T) {
	runDir := neuerLauf(t, Entry{Name: "govulncheck", Kind: KindTool})

	statuses := führeAus(t, Options{
		RunDir:   runDir,
		Target:   modulBaum(t, "quelle/beispiel.py"),
		Scanners: []Scanner{moduleScanner("govulncheck", "govulncheck", "{out}")},
		Tools:    map[string]Tool{"govulncheck": {Path: schreibtSARIF(t, "govulncheck", 0, 0)}},
	}, Entry{Name: "govulncheck", Kind: KindTool})

	if statuses[0].State != StateSkipped {
		t.Fatalf("Eintrag = %q, erwartet %q", statuses[0].State, StateSkipped)
	}
	job := statuses[0].Jobs[0]
	if job.State != StateSkipped || !strings.Contains(job.Reason, ManifestGoModule) {
		t.Errorf("Job = %+v, erwartet übersprungen mit Grund", job)
	}
	if job.Module != "" {
		t.Errorf("Modul = %q, erwartet leer — es wurde keins geprüft", job.Module)
	}
}

// Eine nicht durchführbare Suche ist etwas anderes als kein Modul: hier ist
// gerade unbekannt, ob es eins gibt.
func TestExecuteNichtDurchfuehrbareSucheIstFehlschlag(t *testing.T) {
	runDir := neuerLauf(t, Entry{Name: "govulncheck", Kind: KindTool})
	target := modulBaum(t, "gesperrt/go.mod")
	gesperrt := filepath.Join(target, "gesperrt")
	if err := os.Chmod(gesperrt, 0o000); err != nil {
		t.Fatalf("Rechte setzen: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(gesperrt, 0o755) })

	statuses := führeAus(t, Options{
		RunDir:   runDir,
		Target:   target,
		Scanners: []Scanner{moduleScanner("govulncheck", "govulncheck", "{out}")},
		Tools:    map[string]Tool{"govulncheck": {Path: schreibtSARIF(t, "govulncheck", 0, 0)}},
	}, Entry{Name: "govulncheck", Kind: KindTool})

	if statuses[0].State != StateFailed {
		t.Fatalf("Eintrag = %q, erwartet %q", statuses[0].State, StateFailed)
	}
	if job := statuses[0].Jobs[0]; job.State != StateFailed || !strings.Contains(job.Reason, "nicht durchführbar") {
		t.Errorf("Job = %+v, erwartet einen Fehlschlag mit Grund", job)
	}
}

// workdir target bleibt, wie es war: ein Job, Arbeitsverzeichnis {target},
// kein Modul am Job. Das ist der Fall gosec.
func TestExecuteWorkdirTargetOhneModul(t *testing.T) {
	runDir := neuerLauf(t, Entry{Name: "gosec", Kind: KindTool})
	target := modulBaum(t, "installer/go.mod", "werkzeuge/go.mod")

	statuses := führeAus(t, Options{
		RunDir:   runDir,
		Target:   target,
		Scanners: []Scanner{nativeScanner("gosec", "gosec", "{out}", target)},
		Tools:    map[string]Tool{"gosec": {Path: prueftArbeitsverzeichnis(t, "gosec")}},
	}, Entry{Name: "gosec", Kind: KindTool})

	if len(statuses[0].Jobs) != 1 {
		t.Fatalf("%d Jobs, erwartet 1 — gosec bleibt projektweit: %+v", len(statuses[0].Jobs), statuses[0].Jobs)
	}
	job := statuses[0].Jobs[0]
	if job.Job != "gosec" || job.Module != "" {
		t.Errorf("Job = %+v, erwartet den Katalognamen ohne Modul", job)
	}
	if job.State != StateDone || job.ExitCode == nil || *job.ExitCode != 0 {
		t.Errorf("der Job lief nicht im Zielverzeichnis: %+v", job)
	}
}

// Wechselt der Modulbestand, wechseln die Job-Namen — und die Datei des alten
// Namens bliebe sonst als vermeintliches Ergebnis liegen.
func TestExecuteRaeumtDateienDesVorigenLaufsWeg(t *testing.T) {
	runDir := neuerLauf(t,
		Entry{Name: "govulncheck", Kind: KindTool},
		Entry{Name: "gitleaks", Kind: KindTool},
	)
	target := modulBaum(t, "installer/go.mod")
	scanner := moduleScanner("govulncheck", "govulncheck", "{out}")
	tools := map[string]Tool{
		"govulncheck": {Path: schreibtSARIF(t, "govulncheck", 1, 0)},
		"gitleaks":    {Path: schreibtSARIF(t, "gitleaks", 1, 0)},
	}
	entry := Entry{Name: "govulncheck", Kind: KindTool}

	options := Options{
		RunDir:   runDir,
		Target:   target,
		Scanners: []Scanner{scanner, nativeScanner("gitleaks", "gitleaks", "{out}")},
		Tools:    tools,
	}
	führeAus(t, options, entry, Entry{Name: "gitleaks", Kind: KindTool})

	if dateien := rawDateien(t, runDir); len(dateien) != 2 {
		t.Fatalf("nach dem ersten Lauf: %v, erwartet govulncheck.sarif und gitleaks.sarif", dateien)
	}

	// Ein zweites Modul kommt hinzu: aus govulncheck werden zwei Jobs mit
	// anderen Namen.
	if err := os.MkdirAll(filepath.Join(target, "werkzeuge"), 0o755); err != nil {
		t.Fatalf("zweites Modul anlegen: %v", err)
	}
	if err := os.WriteFile(filepath.Join(target, "werkzeuge", "go.mod"), []byte("module w\n"), 0o644); err != nil {
		t.Fatalf("zweites Modul anlegen: %v", err)
	}

	führeAus(t, options, entry)

	dateien := rawDateien(t, runDir)
	want := []string{"gitleaks.sarif", "govulncheck-installer.sarif", "govulncheck-werkzeuge.sarif"}
	if strings.Join(dateien, ",") != strings.Join(want, ",") {
		t.Errorf("Dateien unter raw/ = %v, erwartet %v", dateien, want)
	}
	// Gegenprobe: der selektive Aufruf hat die Datei des anderen Eintrags
	// stehen lassen.
	if _, err := os.Stat(filepath.Join(runDir, RawDirName, "gitleaks.sarif")); err != nil {
		t.Errorf("die Datei eines fremden Eintrags wurde mit weggeräumt: %v", err)
	}
}

// Der Auslöser hängt am Eintrag, nicht am ersten Job: startet diesmal keiner,
// bliebe die alte Datei liegen, während entries/<tool>.json skipped meldet.
func TestExecuteRaeumtAuchOhneLaufendenJobAuf(t *testing.T) {
	runDir := neuerLauf(t, Entry{Name: "govulncheck", Kind: KindTool})
	target := modulBaum(t, "installer/go.mod")
	scanner := moduleScanner("govulncheck", "govulncheck", "{out}")

	führeAus(t, Options{
		RunDir:   runDir,
		Target:   target,
		Scanners: []Scanner{scanner},
		Tools:    map[string]Tool{"govulncheck": {Path: schreibtSARIF(t, "govulncheck", 1, 0)}},
	}, Entry{Name: "govulncheck", Kind: KindTool})

	if _, err := os.Stat(filepath.Join(runDir, RawDirName, "govulncheck.sarif")); err != nil {
		t.Fatalf("der erste Lauf hat nichts geschrieben: %v", err)
	}

	// Diesmal fehlt das Werkzeug: kein Job läuft.
	statuses := führeAus(t, Options{
		RunDir:   runDir,
		Target:   target,
		Scanners: []Scanner{scanner},
		Tools:    map[string]Tool{"govulncheck": {Reason: "Werkzeug govulncheck ist nicht installiert"}},
	}, Entry{Name: "govulncheck", Kind: KindTool})

	if statuses[0].State != StateSkipped {
		t.Fatalf("Eintrag = %q, erwartet %q", statuses[0].State, StateSkipped)
	}
	if dateien := rawDateien(t, runDir); len(dateien) != 0 {
		t.Errorf("Dateien unter raw/ = %v, erwartet keine — der Eintrag meldet skipped", dateien)
	}
}

// Fehlt die vorige entries/<tool>.json, greift die Namensregel als Rückfall:
// der Job-Name beginnt mit dem Tool-Namen. Der Rückfall bleibt auf *.sarif
// beschränkt und trifft keinen fremden Eintrag.
func TestExecuteRaeumtOhneVorigeEintragsdateiAuf(t *testing.T) {
	runDir := neuerLauf(t, Entry{Name: "govulncheck", Kind: KindTool})
	target := modulBaum(t, "installer/go.mod")

	rawDir := filepath.Join(runDir, RawDirName)
	if err := os.MkdirAll(rawDir, 0o755); err != nil {
		t.Fatalf("raw/ anlegen: %v", err)
	}
	weg := []string{"govulncheck-werkzeuge.sarif"}
	bleibt := []string{"govulncheck-alt.txt", "govulnchecker.sarif", "gitleaks.sarif"}
	for _, name := range append(append([]string{}, weg...), bleibt...) {
		if err := os.WriteFile(filepath.Join(rawDir, name), []byte("{}"), 0o644); err != nil {
			t.Fatalf("%s anlegen: %v", name, err)
		}
	}

	führeAus(t, Options{
		RunDir:   runDir,
		Target:   target,
		Scanners: []Scanner{moduleScanner("govulncheck", "govulncheck", "{out}")},
		Tools:    map[string]Tool{"govulncheck": {Path: schreibtSARIF(t, "govulncheck", 1, 0)}},
	}, Entry{Name: "govulncheck", Kind: KindTool})

	for _, name := range weg {
		if _, err := os.Stat(filepath.Join(rawDir, name)); !os.IsNotExist(err) {
			t.Errorf("%s liegt noch da", name)
		}
	}
	for _, name := range bleibt {
		if _, err := os.Stat(filepath.Join(rawDir, name)); err != nil {
			t.Errorf("%s wurde weggeräumt, gehört aber nicht zu den Job-Dateien dieses Eintrags: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(rawDir, "govulncheck.sarif")); err != nil {
		t.Errorf("der Lauf hat seine eigene Datei nicht geschrieben: %v", err)
	}
}
