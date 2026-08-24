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

	"github.com/kascada/k-playbook/installer/internal/review/sarifconvert"
)

// sarifConverters ordnet einem Werkzeug mit sarif: convert im Katalog seine
// Konverterfunktion zu. Fehlt ein Werkzeug hier, bleibt sein Job skipped mit
// dem Grund „SARIF-Konverter nicht gebaut" (planJobs) — so lange, bis eine
// Folge-Task (z. B. für pip-audit) den nächsten Eintrag ergänzt.
var sarifConverters = map[string]func([]byte) ([]byte, error){
	"trufflehog": sarifconvert.TruffleHog,
}

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
	// candidates zählt je Bezugspunkt und Sorte höchstens einmal über den
	// ganzen Lauf. Execute legt ihn an und reicht ihn an alle Einträge weiter;
	// ein von außen gesetzter bleibt stehen — das nutzt nur der Test, um die
	// Zählung zu ersetzen und mitzuzählen, wie oft sie läuft.
	candidates *candidateCache
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

	// Ebenfalls über den ganzen Lauf: derselbe Baumlauf über dasselbe Ziel
	// ergibt für jeden Job derselben Sorte dasselbe. Angelegt wird er vor den
	// Goroutinen — Options geht als Kopie hinein, der Zeiger darin bleibt
	// derselbe.
	if options.candidates == nil {
		options.candidates = newCandidateCache(countCandidates)
	}

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
			Job:        plan.name,
			State:      plan.state,
			Module:     plan.module,
			Candidates: plan.candidates,
			Reason:     plan.reason,
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
	if len(status.Jobs) == 0 {
		report(options, entry.Name, JobStatus{Job: entry.Name, State: status.State, Finished: status.Finished, Reason: status.Reason})
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
	// candidates ist die Zahl der Dateien unter dem Bezugspunkt, die als
	// Gegenstand in Frage kamen; nil heißt „nicht gezählt".
	candidates *int
	// converter ist gesetzt, wenn scanner.SARIF == SARIFConvert und für
	// scanner.Tool ein Konverter existiert (sarifConverters). runJob schreibt
	// dann den nativen Output zunächst in eine Zwischendatei und schickt sie
	// durch converter, statt sie unverändert als raw/<job>.sarif zu führen.
	converter func([]byte) ([]byte, error)
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
		converter, hasConverter := sarifConverters[scanner.Tool]
		switch {
		case scanner.SARIF == SARIFConvert && !hasConverter:
			plan.state, plan.reason = StateSkipped, "SARIF-Konverter nicht gebaut"
		case scanner.SARIF == SARIFNone:
			plan.state, plan.reason = StateSkipped, "erzeugt kein SARIF"
		case !scanner.AppliesTo(options.Languages):
			plan.state, plan.reason = StateSkipped, "Sprache nicht gewählt"
		case !known || tool.Path == "":
			plan.state, plan.reason = StateSkipped, missingToolReason(entry.Name, known, tool)
		}
		// Ein convert-Job mit vorhandenem Konverter durchläuft dieselben
		// Sprach- und Werkzeug-Prüfungen wie ein nativer — nur der Grund „kein
		// Konverter" entfällt für ihn.
		if plan.state == StateStart && scanner.SARIF == SARIFConvert {
			plan.converter = converter
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

	// Zuletzt, wenn die Bezugspunkte feststehen: was hätte dieser Job prüfen
	// können? Nur für Jobs, die wirklich laufen — ein übersprungener Job hat
	// keinen Gegenstand, und 0 hieße dort fälschlich „gemessen und null".
	//
	// Ein Fehler beim Baumlauf macht keinen Job zum Fehlschlag: die Zählung ist
	// eine Zusatzauskunft, kein Ergebnis. Sie bleibt dann ungesetzt, heißt also
	// „nicht gemessen" — of() gibt dafür nil zurück.
	for index := range plans {
		if plans[index].state != StateStart {
			continue
		}
		plans[index].candidates = options.candidates.of(
			candidateRoot(options.Target, plans[index].module),
			plans[index].scanner.Candidates,
			plans[index].scanner.Languages,
		)
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
// Maßgeblich ist die vorige entries/<tool>.json, und zwar über den Job-Namen:
// raw/<job>.sarif ist die Namensregel, nach der runJob() seinen outPath bildet,
// und der Name steht bei jedem Ausgang in der Datei. Das sarif-Feld täte es
// nicht — runJob() setzt es nur im Erfolgsfall, failJob() gar nicht, und ein
// beim vorigen Aufruf gescheiterter Job kann trotzdem eine Datei hinterlassen
// haben.
//
// Fehlt oder bricht die Datei, wird raw/ selbst gelesen; angenommen wird
// dieselbe Namensregel, beschränkt auf *.sarif, damit spätere
// Konverter-Zwischendateien nicht mitgehen.
func previousJobFiles(runDir string, rawDir string, entry string) ([]string, error) {
	if status, err := ReadEntryStatus(runDir, entry); err == nil {
		files := []string{}
		for _, job := range status.Jobs {
			// Ein skipped-Job hat nie eine Datei geschrieben; sein Name geht
			// trotzdem mit, weil os.Remove eine fehlende Datei ohnehin nicht
			// als Fehler wertet.
			if belongsToEntry(entry, job.Job) {
				files = append(files, filepath.Join(rawDir, job.Job+".sarif"))
			}
		}
		return files, nil
	}

	listing, err := os.ReadDir(rawDir)
	if err != nil {
		return nil, err
	}
	files := []string{}
	for _, item := range listing {
		job, found := strings.CutSuffix(item.Name(), ".sarif")
		if !found || !belongsToEntry(entry, job) {
			continue
		}
		files = append(files, filepath.Join(rawDir, item.Name()))
	}
	return files, nil
}

// belongsToEntry meldet, ob ein Job-Name zu diesem Eintrag gehört und als
// Dateiname unter raw/ taugt.
//
// Es ist die Namensregel aus checkScanner(): der Job-Name ist der Tool-Name
// selbst oder beginnt mit <tool>-. Sie gilt auch für die abgeleiteten Namen der
// Auffächerung, weil die das Präfix erben. Geprüft wird beides, weil der Name
// von der Platte gelesen wird und ein os.Remove steuert: ein Löschen soll weder
// aus dem Verzeichnis herausführen noch eine fremde Datei treffen, was auch
// immer in der gelesenen Datei steht.
func belongsToEntry(entry string, job string) bool {
	if !ValidEntryName(job) {
		return false
	}
	return job == entry || strings.HasPrefix(job, entry+"-")
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
	// rawPath ist nur bei einem Konverter-Job gesetzt (plan.converter != nil):
	// dort schreibt der Prozess zunächst hierher, und der native Text wird
	// erst danach nach outPath konvertiert. Ohne Konverter schreibt der
	// Prozess wie bisher direkt nach outPath — eine Datei existiert dann auch,
	// wenn er nichts liefert (siehe TestExecuteRaeumtDateiEinesGescheitertenJobsWeg).
	rawPath := ""
	if scanner.Output == OutputStdout {
		target := outPath
		if plan.converter != nil {
			rawPath = filepath.Join(rawDir, plan.name+".raw.jsonl")
			target = rawPath
		}
		file, err := os.Create(target)
		if err != nil {
			return failJob(job, fmt.Sprintf("%s ließ sich nicht anlegen: %v", target, err))
		}
		command.Stdout = file
		defer file.Close()
		if rawPath != "" {
			// Die Zwischendatei kann den nativen Text ungekürzt enthalten —
			// bei trufflehog potenziell mit Rohsecrets (Raw/RawV2). Sie darf
			// über das Ende dieses Jobs hinaus nicht liegen bleiben.
			defer os.Remove(rawPath)
		}
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

	// Konvertiert wird unabhängig vom Exit-Code — aus demselben Grund wie bei
	// countFindings gleich: das Ergebnis zählt, nicht der Exit-Code.
	//
	// Vorausgesetzt ist aber, dass der Prozess überhaupt gestartet ist:
	// command.ProcessState == nil heißt, der Start selbst ist misslungen (z. B.
	// falscher Pfad, keine Ausführungsrechte). Ohne diese Prüfung würde eine
	// leere Zwischendatei (von os.Create vor command.Run angelegt) klaglos zu
	// validem, leerem SARIF konvertiert — ein echter Fehlschlag sähe dann wie
	// ein sauberer Scan ohne Funde aus. Bei fehlendem ProcessState bleibt die
	// Konvertierung deshalb aus; der Job fällt in den bestehenden
	// Soft-Skip/Fehlschlag-Pfad unten, der genau diesen Fall schon abdeckt.
	if rawPath != "" && command.ProcessState != nil {
		nativeOutput, err := os.ReadFile(rawPath)
		if err != nil {
			return failJob(job, fmt.Sprintf("%s ließ sich nicht lesen: %v", rawPath, err))
		}
		converted, err := plan.converter(nativeOutput)
		if err != nil {
			return failJob(job, fmt.Sprintf("Konvertierung nach SARIF fehlgeschlagen: %v", err))
		}
		if err := os.WriteFile(outPath, converted, 0o644); err != nil {
			return failJob(job, fmt.Sprintf("%s ließ sich nicht schreiben: %v", outPath, err))
		}
	}

	// Der Ausgang hängt am Ergebnis, nicht am Exit-Code: fast alle Scanner
	// enden mit einem Code ungleich 0, wenn sie etwas gefunden haben. Das ist
	// ihr Ergebnis und kein Fehler. Liegt lesbares SARIF vor, ist der Job done.
	findings, err := countFindings(outPath)
	if err == nil {
		job.State = StateDone
		job.Findings = &findings
		job.SARIF = RawDirName + "/" + plan.name + ".sarif"
		job.Reason = ""
		return job
	}

	// SARIF ist nicht lesbar. Zwei Fälle unterscheiden sich für den Ausgang:
	//   1. Die Datei fehlt oder ist leer — der Scanner hat kein Ergebnis
	//      hinterlassen. Hier greift ein Soft-Skip aus dem Katalog, wenn Exit-
	//      Code und Ausgabe zueinander passen; dann hat das Werkzeug selbst
	//      erklärt, dass es unter dem Bezugspunkt nichts zu prüfen gab, und
	//      der Job wird skipped mit Grund statt failed.
	//   2. Die Datei liegt vor, ist aber nicht lesbar (kaputtes SARIF, JSON-
	//      Fehler bei nicht leerer Datei) — das bleibt ein technischer
	//      Fehlschlag. Kaputtes, nicht leeres SARIF gewinnt vor jedem Marker.
	//
	// Der Soft-Skip greift nur, wenn der Prozess überhaupt regulär mit einem
	// Exit-Code beendet ist. Fehlt ProcessState, ist der Start selbst
	// misslungen; dort gibt es keinen Exit-Code, und die Marker meinen einen
	// Fall, den der Katalog gar nicht abbilden kann.
	if command.ProcessState != nil && sarifMissingOrEmpty(outPath) {
		if rule, match := scanner.MatchSoftSkip(exitCode, errOutput.String(), outOutput.String()); rule != nil {
			return skipJob(job, describeSoftSkip(scanner.Tool, rule, match))
		}
	}

	return failJob(job, describeRunFailure(scanner, exitCode, runErr, err, errOutput.String(), outOutput.String()))
}

// sarifMissingOrEmpty meldet, ob unter path keine oder eine leere Datei liegt.
// Beides zählt als „nichts geschrieben"; nur dann darf ein Soft-Skip überhaupt
// greifen — sobald der Scanner Bytes hinterlassen hat, gewinnt ihre Auswertung.
func sarifMissingOrEmpty(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return os.IsNotExist(err)
	}
	return info.Size() == 0
}

// describeSoftSkip fasst Grund und Herkunft eines Soft-Skips zusammen. Der
// Grund kommt aus der getroffenen Zeile der Werkzeugausgabe — nicht die ganze
// Ausgabe, sondern nur der Match, damit entries/<tool>.json lesbar bleibt.
func describeSoftSkip(tool string, rule *SoftSkipRule, match string) string {
	if match == "" {
		return fmt.Sprintf("%s: soft_skip (Exit %d, /%s/)", tool, rule.ExitCode, rule.Raw)
	}
	return fmt.Sprintf("%s: %s (Exit %d, /%s/)", tool, match, rule.ExitCode, rule.Raw)
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

// skipJob ist das Gegenstück zu failJob für den Soft-Skip: derselbe Weg, sicher
// zu setzen, dass Zeit und Grund am Job stehen — nur der Zustand ist skipped.
func skipJob(job JobStatus, reason string) JobStatus {
	job.State = StateSkipped
	job.Reason = reason
	if job.Finished == "" {
		job.Finished = timestamp()
	}
	// Ohne Findings-Feld: ein übersprungener Job hat kein Ergebnis, und 0
	// hieße dort fälschlich „gemessen und null".
	job.Findings = nil
	job.SARIF = ""
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
