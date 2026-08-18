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
		Jobs:          []JobStatus{},
	}
	if len(scanners) == 0 {
		// syft ist der Fall: es erzeugt eine SBOM und damit keine Befunde. Ohne
		// Job gibt es keinen Ort, an dem der Grund sonst stünde — und die
		// Oberfläche bietet das Werkzeug weiterhin zur Auswahl an.
		status.Reason = fmt.Sprintf("kein Scan-Job für %s: das Werkzeug erzeugt keine Befunde", entry.Name)
	}

	// Vor allem anderen: weg mit dem, was dieser Eintrag beim vorigen Aufruf
	// geschrieben hat. Der Auslöser hängt am Eintrag und nicht am ersten Job —
	// startet diesmal keiner, bliebe sonst eine alte Datei liegen, während
	// entries/<tool>.json skipped meldet.
	if err := clearEntryOutputs(options.RunDir, entry.Name); err != nil {
		status.State = StateFailed
		status.Reason = err.Error()
		status.Finished = timestamp()
		if writeErr := writeProgress(options, status); writeErr != nil {
			return status, writeErr
		}
		return status, fmt.Errorf("%s: %w", entry.Name, err)
	}

	// Erst alles einsortieren, was ohne Aufruf feststeht. Was übrig bleibt,
	// läuft wirklich.
	plans := planJobs(scanners, entry, options, tool, known)
	status.Jobs = make([]JobStatus, len(plans))
	runnable := []int{}
	for index, plan := range plans {
		status.Jobs[index] = JobStatus{
			Job:    plan.name,
			State:  plan.state,
			Module: plan.module,
			Reason: plan.reason,
		}
		if plan.state == StateStart {
			runnable = append(runnable, index)
		}
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
			go func(plan jobPlan, job JobStatus) {
				defer group.Done()
				tokens <- struct{}{}
				defer func() { <-tokens }()

				job.State = StateRunning
				job.Started = timestamp()
				updates <- job
				updates <- runJob(ctx, plan, job, options)
			}(plans[index], status.Jobs[index])
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

// jobPlan ist ein geplanter Aufruf: eine Katalogzeile, bei workdir module an
// genau ein Modul gebunden.
type jobPlan struct {
	scanner Scanner
	// name ist der Job-Name und damit der Dateiname unter raw/. Er weicht vom
	// Katalog nur ab, wenn aufgefächert wurde.
	name string
	// module ist das geprüfte Modul, relativ zum Ziel; leer bei workdir target.
	module string
	// state ist start, wenn der Job wirklich läuft; sonst steht der Ausgang
	// schon vor dem Aufruf fest und reason nennt ihn.
	state  State
	reason string
}

// planJobs macht aus den Katalogzeilen eines Werkzeugs die Aufrufe dieses Laufs.
//
// Was ohne Aufruf feststeht, wird zuerst geprüft: eine Suche im Dateisystem
// lohnt nicht für einen Job, dessen Sprache gar nicht gewählt ist — und
// „Sprache nicht gewählt" ist auch die genauere Auskunft als „kein Modul
// gefunden".
func planJobs(scanners []Scanner, entry Entry, options Options, tool Tool, known bool) []jobPlan {
	plans := []jobPlan{}

	// Höchstens eine Suche je Eintrag, auch wenn mehrere Jobs Module brauchen:
	// sie liefe über denselben Baum und ergäbe dasselbe.
	var modules []string
	var searchErr error
	searched := false

	// Die Namen aus dem Katalog sind schon vergeben — ein abgeleiteter Name
	// darf keinen davon treffen.
	taken := map[string]bool{}
	for _, scanner := range scanners {
		taken[scanner.Job] = true
	}

	for _, scanner := range scanners {
		plan := jobPlan{scanner: scanner, name: scanner.Job, state: StateStart}
		switch {
		case scanner.SARIF == SARIFConvert:
			plan.state, plan.reason = StateSkipped, "SARIF-Konverter nicht gebaut"
		case scanner.SARIF == SARIFNone:
			plan.state, plan.reason = StateSkipped, "erzeugt kein SARIF"
		case !scanner.AppliesTo(options.Languages):
			plan.state, plan.reason = StateSkipped, "Sprache nicht gewählt"
		case !known || tool.Path == "":
			plan.state, plan.reason = StateSkipped, missingToolReason(entry.Name, known, tool)
		}
		if plan.state != StateStart || scanner.Workdir != WorkdirModule {
			plans = append(plans, plan)
			continue
		}

		if !searched {
			// go.mod fest an dieser Stelle: die Suche selbst ist sprachneutral,
			// angewandt wird sie bisher nur auf Go.
			modules, searchErr = FindModules(options.Target, ManifestGoModule)
			searched = true
		}

		switch {
		case searchErr != nil:
			// Nicht skipped: nach einer abgebrochenen Suche ist gerade
			// unbekannt, ob es ein Modul gibt, und skipped behauptete, es gebe
			// nichts zu tun.
			plan.state = StateFailed
			plan.reason = fmt.Sprintf("Suche nach %s nicht durchführbar: %v", ManifestGoModule, searchErr)
			plans = append(plans, plan)
		case len(modules) == 0:
			// Es fehlt der Gegenstand, nicht das Werkzeug.
			plan.state = StateSkipped
			plan.reason = fmt.Sprintf("kein %s unter dem Ziel gefunden", ManifestGoModule)
			plans = append(plans, plan)
		case len(modules) == 1:
			// Ein Modul heißt: alles bleibt, wie es war. Der Name kommt aus dem
			// Katalog, die Datei unter raw/ heißt wie bisher — sichtbar ist das
			// geprüfte Modul trotzdem, nämlich am Job.
			plan.module = modules[0]
			plans = append(plans, plan)
		default:
			for _, module := range modules {
				fanned := plan
				fanned.module = module
				fanned.name = jobNameForModule(taken, scanner.Job, module)
				plans = append(plans, fanned)
			}
		}
	}
	return plans
}

// clearEntryOutputs räumt weg, was dieser Eintrag beim vorigen Aufruf unter
// raw/ geschrieben hat.
//
// runJob entfernt nur die eine Datei, die es selbst gleich anlegt. Das genügte,
// solange die Job-Namen fest im Katalog standen; jetzt hängen sie am
// Modulbestand: kommt ein zweites Modul hinzu, heißt der Job nicht mehr
// govulncheck, sondern govulncheck-installer — und raw/govulncheck.sarif des
// vorigen Aufrufs bliebe daneben liegen und gälte weiter als Ergebnis.
//
// Weggeräumt wird ausschließlich Eigenes: die Dateien anderer Einträge bleiben
// stehen, auch bei einem selektiven Aufruf.
func clearEntryOutputs(runDir string, entry string) error {
	rawDir := filepath.Join(runDir, RawDirName)
	// raw/ entsteht erst beim ersten Job. Fehlt es, gibt es nichts wegzuräumen.
	if _, err := os.Stat(rawDir); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	files, err := previousJobFiles(runDir, rawDir, entry)
	if err != nil {
		return err
	}
	for _, file := range files {
		if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("%s ließ sich nicht entfernen: %v", file, err)
		}
	}
	return nil
}

// previousJobFiles sind die Dateien unter raw/, die dem Eintrag gehören.
//
// Maßgeblich ist die vorige entries/<tool>.json: sie nennt die Job-Namen samt
// sarif-Pfad. Fehlt oder bricht sie, greift die Namensregel aus checkScanner()
// als Rückfall — der Job-Name beginnt mit dem Tool-Namen. Der Rückfall bleibt
// auf *.sarif beschränkt, damit spätere Konverter-Zwischendateien nicht
// mitgehen.
func previousJobFiles(runDir string, rawDir string, entry string) ([]string, error) {
	if status, err := ReadEntryStatus(runDir, entry); err == nil {
		files := []string{}
		for _, job := range status.Jobs {
			if name, ok := rawFileName(job.SARIF); ok {
				files = append(files, filepath.Join(rawDir, name))
			}
		}
		return files, nil
	}

	files := []string{}
	for _, pattern := range []string{entry + ".sarif", entry + "-*.sarif"} {
		matches, err := filepath.Glob(filepath.Join(rawDir, pattern))
		if err != nil {
			return nil, err
		}
		files = append(files, matches...)
	}
	return files, nil
}

// rawFileName prüft den Ort, den die vorige Datei nennt, und liefert den
// Dateinamen daraus.
//
// Geschrieben hat den Pfad zwar der Ausführer selbst, gelesen wird er aber von
// der Platte: ein Löschen soll nicht davon abhängen, was in der Datei steht.
// Angenommen wird deshalb nur, was auch als Job-Datei entstehen konnte.
func rawFileName(sarif string) (string, bool) {
	rest, found := strings.CutPrefix(sarif, RawDirName+"/")
	if !found || !strings.HasSuffix(rest, ".sarif") {
		return "", false
	}
	if !ValidEntryName(strings.TrimSuffix(rest, ".sarif")) {
		return "", false
	}
	return rest, true
}

// runJob startet ein Werkzeug und wertet den Ausgang aus.
func runJob(ctx context.Context, plan jobPlan, job JobStatus, options Options) JobStatus {
	scanner := plan.scanner
	tool := options.Tools[scanner.Tool]

	rawDir := filepath.Join(options.RunDir, RawDirName)
	if err := os.MkdirAll(rawDir, 0o755); err != nil {
		return failJob(job, fmt.Sprintf("%s ließ sich nicht anlegen: %v", RawDirName, err))
	}
	outPath := filepath.Join(rawDir, plan.name+".sarif")
	// Ein zweiter Aufruf überschreibt. Die alte Datei muss vorher weg, sonst
	// gälte sie als Ergebnis, wenn der Scanner diesmal nichts schreibt.
	if err := os.Remove(outPath); err != nil && !os.IsNotExist(err) {
		return failJob(job, fmt.Sprintf("%s ließ sich nicht ersetzen: %v", outPath, err))
	}

	jobCtx, cancel := context.WithTimeout(ctx, scanner.Timeout)
	defer cancel()

	// {target} bleibt die Projektwurzel, auch wenn der Job in einem Modul
	// darunter arbeitet: sonst hätte ein Aufruf keine Möglichkeit mehr, auf das
	// Projekt als Ganzes zu zeigen.
	moduleDir := ""
	if plan.module != "" {
		moduleDir = filepath.Join(options.Target, plan.module)
	}
	workDir := options.Target
	if scanner.Workdir == WorkdirModule {
		workDir = moduleDir
	}

	args := scanner.Command(outPath, options.Target, moduleDir, options.ScriptsDir)
	command := exec.CommandContext(jobCtx, tool.Path, args...)
	command.Dir = workDir
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
	job.SARIF = RawDirName + "/" + plan.name + ".sarif"
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
