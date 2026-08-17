package review

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// DefaultParallel ist die Obergrenze gleichzeitiger Jobs, gezählt über den
// ganzen Lauf.
//
// Vier, nicht die Kernzahl: die Scanner sind selbst nebenläufig und teils
// speicherhungrig (semgrep, trivy), und über vier hinaus gewinnt ein Lauf nicht
// mehr an Zeit, wohl aber an Spitzenlast. Auf einem kleinen Rechner gilt die
// Kernzahl, damit vier Scanner dort nicht um zwei Kerne streiten.
const DefaultParallel = 4

// stderrKeep ist die Menge an Fehlerausgabe, die für die Begründung eines
// Fehlschlags aufgehoben wird. Ein Scanner kann sehr viel schreiben; für die
// Meldung zählt das Ende.
const stderrKeep = 8 << 10

// killGrace ist die Frist zwischen dem Abbruch eines Jobs und dem Schließen
// seiner Ausgabeleitungen.
const killGrace = 5 * time.Second

// Tool ist ein aufgelöstes Werkzeug.
//
// Der Pfad kommt aus dem Preflight und wird nicht selbst im PATH gesucht: in
// einem Python-Projekt mit aktivem venv griffe eine eigene Auflösung dessen
// ruff, und der Lauf startete ein anderes Werkzeug, als der Preflight geprüft
// hat (rules/tool-install-scope.md).
type Tool struct {
	// Path ist das Programm, absolut. Leer heißt: nicht vorhanden.
	Path string
	// Reason nennt bei leerem Path den Grund im Klartext.
	Reason string
}

// Options sind die Vorgaben eines Laufs für den Ausführer.
//
// Der Ausführer sucht sich nichts davon selbst zusammen — Installation,
// Preflight und Konfiguration liest das Kommando und reicht das Ergebnis
// herein. Dadurch bleibt das Ausführen ohne echte Installation prüfbar.
type Options struct {
	// RunDir ist das Laufverzeichnis mit run.json und entries/.
	RunDir string
	// Target ist project.repo_root, absolut: der Gegenstand des Scans und das
	// Arbeitsverzeichnis jedes Jobs.
	Target string
	// ScriptsDir ist das scripts/-Verzeichnis der Installation, für {scripts}.
	ScriptsDir string
	// Languages ist die Sprachauswahl des Laufs, aus run.json.
	Languages []string
	// Scanners ist der gelesene Katalog.
	Scanners []Scanner
	// Tools sind die aufgelösten Werkzeuge, nach Tool-Namen.
	Tools map[string]Tool
	// Parallel ist die Obergrenze gleichzeitiger Jobs. 0 heißt DefaultParallel.
	Parallel int
	// Progress meldet jede Zustandsänderung eines Jobs, auch den Start. Darf
	// fehlen; wird aus mehreren Goroutinen aufgerufen.
	Progress func(entry string, job JobStatus)
}

// Execute führt die angegebenen Einträge aus und liefert ihren Endstand.
//
// Ausgeführt werden nur Einträge der Art tool. Die Jobs eines Eintrags
// schreiben nichts selbst: sie melden ihr Ergebnis an den Eintrag, und der
// schreibt seine entries/<name>.json. Damit bleibt es bei einem Schreiber je
// Datei — deshalb geht parallel überhaupt. run.json wird nicht angefasst.
func Execute(ctx context.Context, entries []Entry, options Options) ([]EntryStatus, error) {
	if options.RunDir == "" {
		return nil, errors.New("kein Laufverzeichnis angegeben")
	}
	if options.Target == "" {
		return nil, errors.New("kein Ziel angegeben; erwartet wird project.repo_root")
	}

	parallel := options.Parallel
	if parallel <= 0 {
		parallel = DefaultParallel
		if cores := runtime.NumCPU(); cores < parallel {
			parallel = cores
		}
	}
	if parallel < 1 {
		parallel = 1
	}

	// Nur kind tool. Ein ai-Eintrag wird von einem Assistenten über seinen
	// Command ausgeführt und bleibt unangetastet auf start.
	for _, entry := range entries {
		if entry.Kind != KindTool {
			return nil, fmt.Errorf("%s ist ein Eintrag der Art %s; ausgeführt wird nur %s", entry.Name, entry.Kind, KindTool)
		}
	}

	// Über den ganzen Lauf gezählt, nicht je Eintrag: sonst liefen bei acht
	// Einträgen acht mal so viele Jobs wie erlaubt.
	tokens := make(chan struct{}, parallel)

	results := make([]EntryStatus, len(entries))
	failures := make([]error, len(entries))

	var group sync.WaitGroup
	for index, entry := range entries {
		group.Add(1)
		go func(index int, entry Entry) {
			defer group.Done()
			status, err := executeEntry(ctx, entry, options, tokens)
			results[index] = status
			failures[index] = err
		}(index, entry)
	}
	group.Wait()

	return results, errors.Join(failures...)
}

// executeEntry führt die Jobs eines Eintrags aus und führt dessen Datei.
func executeEntry(ctx context.Context, entry Entry, options Options, tokens chan struct{}) (EntryStatus, error) {
	scanners := ScannersFor(options.Scanners, entry.Name)
	tool, known := options.Tools[entry.Name]

	status := EntryStatus{
		SchemaVersion: EntrySchemaVersion,
		Name:          entry.Name,
		Kind:          entry.Kind,
		Started:       timestamp(),
		Jobs:          make([]JobStatus, len(scanners)),
	}
	if len(scanners) == 0 {
		// syft ist der Fall: es erzeugt eine SBOM und damit keine Befunde. Ohne
		// Job gibt es keinen Ort, an dem der Grund sonst stünde — und die
		// Oberfläche bietet das Werkzeug weiterhin zur Auswahl an.
		status.Reason = fmt.Sprintf("kein Scan-Job für %s: das Werkzeug erzeugt keine Befunde", entry.Name)
	}

	// Erst alles einsortieren, was ohne Aufruf feststeht. Was übrig bleibt,
	// läuft wirklich.
	runnable := []int{}
	for index, scanner := range scanners {
		job := JobStatus{Job: scanner.Job, State: StateSkipped}
		switch {
		case scanner.SARIF == SARIFConvert:
			job.Reason = "SARIF-Konverter nicht gebaut"
		case scanner.SARIF == SARIFNone:
			job.Reason = "erzeugt kein SARIF"
		case !scanner.AppliesTo(options.Languages):
			job.Reason = "Sprache nicht gewählt"
		case !known || tool.Path == "":
			job.Reason = missingToolReason(entry.Name, known, tool)
		default:
			job.State = StateStart
			runnable = append(runnable, index)
		}
		status.Jobs[index] = job
	}

	// Geschrieben wird schon beim Start, mit dem Zustand nach Regel 0: läge die
	// Datei erst am Ende, wäre der Fortschritt während des Laufs nicht lesbar.
	status.State = DeriveEntryState(status.Jobs)
	if err := writeProgress(options, status); err != nil {
		return status, err
	}
	// Gemeldet wird nur, was schon feststeht. Die übrigen melden sich gleich
	// selbst mit running.
	for _, job := range status.Jobs {
		if terminal(job.State) {
			report(options, entry.Name, job)
		}
	}

	if len(runnable) > 0 {
		updates := make(chan JobStatus)
		var group sync.WaitGroup
		for _, index := range runnable {
			group.Add(1)
			go func(scanner Scanner, job JobStatus) {
				defer group.Done()
				tokens <- struct{}{}
				defer func() { <-tokens }()

				job.State = StateRunning
				job.Started = timestamp()
				updates <- job
				updates <- runJob(ctx, scanner, job, options)
			}(scanners[index], status.Jobs[index])
		}
		go func() {
			group.Wait()
			close(updates)
		}()

		// Ein einziger Leser hält die Jobliste — deshalb braucht sie keine
		// Sperre, und die Datei bekommt genau einen Schreiber.
		byJob := map[string]int{}
		for index, job := range status.Jobs {
			byJob[job.Job] = index
		}
		// Ein Schreibfehler beendet die Schleife nicht: die laufenden Jobs
		// melden weiter, und niemand nähme ihre Meldungen mehr ab — sie hingen
		// bis zum Ende des Prozesses. Gemerkt wird der erste Fehler.
		var writeErr error
		for update := range updates {
			status.Jobs[byJob[update.Job]] = update
			status.State = DeriveEntryState(status.Jobs)
			if err := writeProgress(options, status); err != nil && writeErr == nil {
				writeErr = err
			}
			report(options, entry.Name, update)
		}
		if writeErr != nil {
			return status, writeErr
		}
	}

	status.State = DeriveEntryState(status.Jobs)
	status.Finished = timestamp()
	if err := writeProgress(options, status); err != nil {
		return status, err
	}
	return status, nil
}

// missingToolReason erklärt, warum ein Werkzeug nicht startet. Der Preflight
// kennt den Grund; fehlt er, bleibt die allgemeine Auskunft.
func missingToolReason(name string, known bool, tool Tool) string {
	if known && tool.Reason != "" {
		return tool.Reason
	}
	return fmt.Sprintf("Werkzeug %s ist nicht installiert", name)
}

// runJob startet ein Werkzeug und wertet den Ausgang aus.
func runJob(ctx context.Context, scanner Scanner, job JobStatus, options Options) JobStatus {
	tool := options.Tools[scanner.Tool]

	rawDir := filepath.Join(options.RunDir, RawDirName)
	if err := os.MkdirAll(rawDir, 0o755); err != nil {
		return failJob(job, fmt.Sprintf("%s ließ sich nicht anlegen: %v", RawDirName, err))
	}
	outPath := filepath.Join(rawDir, scanner.Job+".sarif")
	// Ein zweiter Aufruf überschreibt. Die alte Datei muss vorher weg, sonst
	// gälte sie als Ergebnis, wenn der Scanner diesmal nichts schreibt.
	if err := os.Remove(outPath); err != nil && !os.IsNotExist(err) {
		return failJob(job, fmt.Sprintf("%s ließ sich nicht ersetzen: %v", outPath, err))
	}

	jobCtx, cancel := context.WithTimeout(ctx, scanner.Timeout)
	defer cancel()

	args := scanner.Command(outPath, options.Target, options.ScriptsDir)
	command := exec.CommandContext(jobCtx, tool.Path, args...)
	command.Dir = options.Target
	// Ohne WaitDelay liefe das Timeout ins Leere, sobald ein Scanner selbst
	// Prozesse startet: der Abbruch trifft nur ihn, seine Kinder halten das
	// Ende der Ausgabepipe offen, und Run wartete weiter auf sie. Die Frist
	// gibt dem Werkzeug Zeit, sauber zu enden, und nimmt ihm danach die Pipe.
	command.WaitDelay = killGrace

	var errOutput tailBuffer
	errOutput.limit = stderrKeep
	command.Stderr = &errOutput

	var outOutput tailBuffer
	outOutput.limit = stderrKeep
	if scanner.Output == OutputStdout {
		file, err := os.Create(outPath)
		if err != nil {
			return failJob(job, fmt.Sprintf("%s ließ sich nicht anlegen: %v", outPath, err))
		}
		command.Stdout = file
		defer file.Close()
	} else {
		command.Stdout = &outOutput
	}

	runErr := command.Run()
	job.Finished = timestamp()
	// ProcessState fehlt, wenn der Start selbst misslungen ist — dann gibt es
	// keinen Exit-Code, und das soll man der Datei ansehen.
	exitCode := -1
	if command.ProcessState != nil {
		exitCode = command.ProcessState.ExitCode()
		job.ExitCode = &exitCode
	}

	if jobCtx.Err() == context.DeadlineExceeded {
		return failJob(job, fmt.Sprintf("Zeitüberschreitung nach %s", scanner.Timeout))
	}
	if ctx.Err() != nil {
		return failJob(job, "abgebrochen")
	}

	// Der Ausgang hängt am Ergebnis, nicht am Exit-Code: fast alle Scanner
	// enden mit einem Code ungleich 0, wenn sie etwas gefunden haben. Das ist
	// ihr Ergebnis und kein Fehler. Liegt lesbares SARIF vor, ist der Job done.
	findings, err := countFindings(outPath)
	if err != nil {
		return failJob(job, describeRunFailure(scanner, exitCode, runErr, err, errOutput.String(), outOutput.String()))
	}

	job.State = StateDone
	job.Findings = &findings
	job.SARIF = RawDirName + "/" + scanner.Job + ".sarif"
	job.Reason = ""
	return job
}

// describeRunFailure setzt die Begründung eines technischen Fehlschlags
// zusammen: was das Werkzeug gemeldet hat, kommt mit hinein — sonst steht in
// der Datei nur, dass es nicht geklappt hat.
func describeRunFailure(scanner Scanner, exitCode int, runErr error, sarifErr error, stderr string, stdout string) string {
	parts := []string{fmt.Sprintf("kein lesbares SARIF: %v", sarifErr)}
	if runErr != nil {
		parts = append(parts, fmt.Sprintf("%s endete mit Exit-Code %d", scanner.Tool, exitCode))
	}
	if message := lastLine(stderr); message != "" {
		parts = append(parts, message)
	} else if message := lastLine(stdout); message != "" {
		parts = append(parts, message)
	}
	return strings.Join(parts, "; ")
}

func failJob(job JobStatus, reason string) JobStatus {
	job.State = StateFailed
	job.Reason = reason
	if job.Finished == "" {
		job.Finished = timestamp()
	}
	return job
}

// sarifDocument ist so viel von SARIF, wie zum Zählen nötig ist. Mehr zu lesen
// hieße, ein Format nachzubauen, das hier niemand auswertet.
type sarifDocument struct {
	Version string `json:"version"`
	Runs    []struct {
		Results []json.RawMessage `json:"results"`
	} `json:"runs"`
}

// countFindings liest die geschriebene Datei und zählt die Befunde. Sie ist
// zugleich die Prüfung, ob der Job überhaupt ein Ergebnis hinterlassen hat.
func countFindings(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, errors.New("die Datei wurde nicht geschrieben")
		}
		return 0, err
	}

	var document sarifDocument
	if err := json.Unmarshal(data, &document); err != nil {
		return 0, fmt.Errorf("die Datei ist kein JSON: %w", err)
	}
	if document.Version == "" && len(document.Runs) == 0 {
		return 0, errors.New("die Datei ist kein SARIF")
	}

	count := 0
	for _, run := range document.Runs {
		count += len(run.Results)
	}
	return count, nil
}

func writeProgress(options Options, status EntryStatus) error {
	if err := WriteEntryStatus(options.RunDir, status); err != nil {
		return fmt.Errorf("%s: %w", EntryFile(options.RunDir, status.Name), err)
	}
	return nil
}

func report(options Options, entry string, job JobStatus) {
	if options.Progress != nil {
		options.Progress(entry, job)
	}
}

func timestamp() string {
	return time.Now().Format(time.RFC3339)
}

// tailBuffer hält das Ende eines Stroms. Ein Scanner kann sehr viel schreiben,
// und für die Begründung eines Fehlschlags zählt die letzte Meldung.
type tailBuffer struct {
	limit int
	data  bytes.Buffer
}

func (b *tailBuffer) Write(chunk []byte) (int, error) {
	written := len(chunk)
	b.data.Write(chunk)
	if b.limit > 0 && b.data.Len() > b.limit {
		kept := bytes.Clone(b.data.Bytes()[b.data.Len()-b.limit:])
		b.data.Reset()
		b.data.Write(kept)
	}
	return written, nil
}

func (b *tailBuffer) String() string {
	return b.data.String()
}

// lastLine ist die letzte Zeile mit Inhalt, gekürzt: sie steht in einer
// Meldung, nicht in einem Protokoll.
func lastLine(text string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	for index := len(lines) - 1; index >= 0; index-- {
		line := strings.TrimSpace(lines[index])
		if line == "" {
			continue
		}
		if len(line) > 200 {
			line = line[:200] + "…"
		}
		return line
	}
	return ""
}
