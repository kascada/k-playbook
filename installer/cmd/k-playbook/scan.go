package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/kascada/k-playbook/installer/internal/project"
	"github.com/kascada/k-playbook/installer/internal/review"
)

// runScan führt die Werkzeug-Einträge eines Laufs aus.
//
// Das Kommando blockiert, bis alle ausgewählten Einträge durch sind. Einen
// Anstoß-und-Zurück-Modus gibt es nicht: der Fortschritt wird währenddessen
// allein aus entries/ gelesen.
func runScan(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("kein Lauf angegeben: k-playbook scan <lauf> [eintrag …]")
	}
	runName := args[0]
	wanted := args[1:]

	environment := project.Detect()
	if !environment.Installed {
		return fmt.Errorf("keine %s gefunden — gesucht ab %s aufwärts",
			project.ConfigFileName, project.DisplayPath(environment.SearchedFrom))
	}
	if !environment.PlaybookPresent {
		return fmt.Errorf("die Installation unter %s fehlt", project.DisplayPath(environment.PlaybookDir))
	}

	config, err := project.ReadConfig(environment.ProjectDir)
	if err != nil {
		return err
	}
	if err := project.CheckSchema(config); err != nil {
		return err
	}
	target := project.RepoRootDir(environment.ProjectDir, config)

	localDir := project.LocalDir(environment.ProjectDir)
	runDir := review.RunDir(localDir, runName)
	run, err := review.ReadRun(runDir)
	if err != nil {
		return fmt.Errorf("Lauf %s ist nicht lesbar: %w", runName, err)
	}

	entries, err := selectEntries(runDir, run, wanted)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		if len(wanted) > 0 {
			fmt.Println("Nichts auszuführen: unter den genannten Einträgen ist keiner, den das Werkzeug ausführt.")
		} else {
			fmt.Printf("Nichts auszuführen: in %s steht kein Werkzeug-Eintrag auf %s.\n",
				project.DisplayPath(runDir), review.StateStart)
		}
		return nil
	}

	scanners, err := review.LoadScanners(review.ScannerCatalog(environment.PlaybookDir))
	if err != nil {
		return fmt.Errorf("Katalog der Scan-Jobs: %w", err)
	}

	// Die Sprachen kommen aus run.json, nicht aus der heutigen Konfiguration:
	// der Lauf soll das prüfen, wofür er angelegt wurde.
	preflight, err := project.CheckTools(environment.ProjectDir, run.Languages)
	if err != nil {
		return err
	}

	options := review.Options{
		RunDir:     runDir,
		Target:     target,
		ScriptsDir: filepath.Join(environment.PlaybookDir, "scripts"),
		Languages:  run.Languages,
		Scanners:   scanners,
		Tools:      resolveTools(preflight),
		Progress:   progressPrinter(),
	}

	fmt.Printf("Lauf %s in %s\n", runName, project.DisplayPath(runDir))
	fmt.Printf("Ziel: %s\n\n", project.DisplayPath(target))

	statuses, err := review.Execute(context.Background(), entries, options)
	if err != nil {
		return err
	}

	return printSummary(runDir, run, statuses)
}

// selectEntries bestimmt, was läuft.
//
// Ohne Angabe alle Werkzeug-Einträge, deren Zustand unter entries/ start ist —
// eine fehlende Datei zählt als start. Ein ausdrücklich genannter ai-Eintrag
// wird nicht ausgeführt, sondern erklärt.
func selectEntries(runDir string, run review.Run, wanted []string) ([]review.Entry, error) {
	byName := map[string]review.Entry{}
	for _, entry := range run.Entries {
		byName[entry.Name] = entry
	}

	if len(wanted) == 0 {
		entries := []review.Entry{}
		for _, entry := range run.Entries {
			if entry.Kind == review.KindTool && review.EntryState(runDir, entry.Name) == review.StateStart {
				entries = append(entries, entry)
			}
		}
		return entries, nil
	}

	entries := []review.Entry{}
	seen := map[string]bool{}
	for _, name := range wanted {
		entry, known := byName[name]
		if !known {
			return nil, fmt.Errorf("%s ist kein Eintrag des Laufs", name)
		}
		if entry.Kind != review.KindTool {
			fmt.Fprintf(os.Stderr,
				"%s ist ein Review-Rezept: das führt ein Assistent über seinen Command aus. Der Eintrag bleibt auf %s.\n",
				name, review.StateStart)
			continue
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		entries = append(entries, entry)
	}
	return entries, nil
}

// resolveTools übernimmt die Auflösung des Preflights.
//
// Gesucht wird nicht selbst im PATH: der Preflight bricht vorher ab, wenn ein
// Projekt-venv aktiv ist, und liefert genau das Werkzeug, dessen Vorhandensein
// er geprüft hat (rules/tool-install-scope.md).
func resolveTools(preflight project.ToolPreflight) map[string]review.Tool {
	tools := map[string]review.Tool{}
	for _, tool := range preflight.Tools {
		resolved := review.Tool{}
		if tool.Status == "ok" && tool.Path != "" {
			resolved.Path = tool.Path
		} else {
			resolved.Reason = fmt.Sprintf("Werkzeug %s ist nicht installiert", tool.Name)
		}
		tools[tool.Name] = resolved
	}
	return tools
}

// progressPrinter meldet jede Zustandsänderung eines Jobs, während der Lauf
// läuft. Die Sperre hält die Zeilen ganz: die Jobs mehrerer Einträge melden
// sich gleichzeitig.
func progressPrinter() func(string, review.JobStatus) {
	var mutex sync.Mutex
	return func(entry string, job review.JobStatus) {
		mutex.Lock()
		defer mutex.Unlock()
		fmt.Printf("  %-16s %-16s %s\n", entry, job.Job, describeJob(job))
	}
}

// describeEntry nennt den Zustand und, wo es einen gibt, den Grund. Ein
// Werkzeug ohne Scan-Job hat keinen Job, an dem der Grund stehen könnte.
func describeEntry(status review.EntryStatus) string {
	if status.Reason != "" {
		return fmt.Sprintf("%s: %s", status.State, status.Reason)
	}
	return string(status.State)
}

func describeJob(job review.JobStatus) string {
	switch job.State {
	case review.StateRunning:
		return "läuft"
	case review.StateDone:
		findings := 0
		if job.Findings != nil {
			findings = *job.Findings
		}
		// Die Kandidatenzahl steht dort, wo sie etwas bedeutet: bei 0 Befunden
		// trennt sie „nichts zu prüfen" von „nichts geprüft". Bei Befunden ist
		// sie Rauschen — dass geprüft wurde, steht schon in der Zahl davor.
		if findings == 0 && job.Candidates != nil {
			return fmt.Sprintf("fertig, 0 Befunde bei %d Kandidaten → %s", *job.Candidates, job.SARIF)
		}
		return fmt.Sprintf("fertig, %d Befunde → %s", findings, job.SARIF)
	default:
		if job.Reason != "" {
			return fmt.Sprintf("%s: %s", job.State, job.Reason)
		}
		return string(job.State)
	}
}

// printSummary fasst den Lauf zusammen und meldet einen technischen
// Fehlschlag als Fehler des Kommandos — ein Befund ist keiner.
func printSummary(runDir string, run review.Run, statuses []review.EntryStatus) error {
	fmt.Println()
	failed := []string{}
	for _, status := range statuses {
		fmt.Printf("  %-16s %s\n", status.Name, describeEntry(status))
		if status.State == review.StateFailed {
			failed = append(failed, status.Name)
		}
	}

	fmt.Printf("\nLaufzustand: %s\n", review.DeriveRunState(runDir, run))

	if len(failed) > 0 {
		sort.Strings(failed)
		return fmt.Errorf("technisch fehlgeschlagen: %s — Einzelheiten in %s/",
			strings.Join(failed, ", "), review.EntriesDirName)
	}
	return nil
}
