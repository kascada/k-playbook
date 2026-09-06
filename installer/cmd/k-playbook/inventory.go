package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/kascada/k-playbook/installer/internal/inventory"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// runInventory erhebt das Versionsinventar und schreibt es nach
// k-playbook-local/docs/versions/inventory.md.
//
// Wie scan und merge sammelt das Kommando nur zusammen, was der Lauf braucht —
// Projektwurzel, Quellenkonfiguration und Zielpfad — und reicht es an
// internal/inventory weiter. Die Fachlogik steht dort, allen voran die
// Vertrauensgrenze: sie ist einmal definiert und gilt für jeden Aufrufweg
// gleich, das Kommando öffnet keine Quelle an ihr vorbei.
//
// Die Installation unter playbook.dir wird nicht gebraucht: das Inventar liest
// die deklarativen Quellen des Projekts, keinen Katalog. Geprüft wird deshalb
// nur der Anker, wie bei merge.
func runInventory(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("erwartet keine Argumente: k-playbook inventory")
	}

	environment := project.Detect()
	if !environment.Installed {
		return fmt.Errorf("keine %s gefunden — gesucht ab %s aufwärts",
			project.ConfigFileName, project.DisplayPath(environment.SearchedFrom))
	}

	localDir := project.LocalDir(environment.ProjectDir)
	options := inventory.Options{
		ProjectDir:    environment.ProjectDir,
		SourcesFile:   filepath.Join(localDir, project.VersionSourcesFileName),
		InventoryFile: inventory.FilePath(localDir),
	}

	result, outcome, err := inventory.Run(options)
	if err != nil {
		return err
	}

	printInventory(os.Stdout, options, result, outcome)
	return nil
}

// printInventory schreibt den Bericht eines Laufs.
//
// Ablehnungen und Hinweise stehen vollständig darin, nicht nur als Zahl: eine
// Quelle, die konfiguriert ist und nicht gelesen werden konnte, ist eine Lücke
// im Inventar, und eine Lücke, die niemand sieht, ist schlimmer als ein Fehler.
func printInventory(out io.Writer, options inventory.Options, result inventory.Result, outcome inventory.Outcome) {
	fmt.Fprintf(out, "Versionsinventar für %s\n\n", project.DisplayPath(options.ProjectDir))

	fmt.Fprintf(out, "  Ausgewertete Quellen:         %d\n", len(result.Sources))
	fmt.Fprintf(out, "  Konfigurierte Zusatzquellen:  %d\n", result.ConfiguredSources)
	fmt.Fprintf(out, "  Einträge:                     %d\n", len(result.Entries))
	fmt.Fprintf(out, "  Abweichungen:                 %s\n", describeDeviations(result.Deviations))
	fmt.Fprintf(out, "  Abgelehnte Quellen:           %d\n", len(result.Rejections))
	fmt.Fprintf(out, "  Nicht durchsuchte Quellen:    %d\n", excludedSources(result))
	fmt.Fprintf(out, "  Hinweise:                     %d\n", len(result.Notes))

	if len(result.Rejections) > 0 {
		fmt.Fprintf(out, "\nAbgelehnte Quellen:\n")
		for _, rejection := range result.Rejections {
			fmt.Fprintf(out, "  %s\n", describeRejection(rejection))
		}
	}
	if excludedSources(result) > 0 {
		// Ein Ausschluss ist keine Ablehnung — der Bereich dürfte gelesen
		// werden, er wird nur nicht von selbst gesucht. Genannt wird er
		// trotzdem: ein stiller Filter wäre eine Lücke, die niemand sieht.
		fmt.Fprintf(out, "\nNicht durchsuchte Bereiche:\n")
		for _, exclusion := range result.Exclusions {
			if exclusion.Skipped == 0 {
				continue
			}
			fmt.Fprintf(out, "  %s (%s): %d Quellen übergangen — %s\n",
				exclusion.Pattern, exclusion.Origin, exclusion.Skipped, exclusion.Reason)
		}
	}
	if len(result.Notes) > 0 {
		fmt.Fprintf(out, "\nHinweise:\n")
		for _, note := range result.Notes {
			if note.Source == "" {
				fmt.Fprintf(out, "  %s\n", note.Text)
				continue
			}
			fmt.Fprintf(out, "  %s: %s\n", note.Source, note.Text)
		}
	}

	fmt.Fprintln(out)
	if outcome.Problem != "" {
		fmt.Fprintf(out, "Hinweis zum Bestand: %s\n", outcome.Problem)
	}
	// Ein Lauf ohne inhaltliche Änderung fasst die Datei nicht an. Das steht
	// hier ausdrücklich, sonst liest sich ein unveränderter Zeitstempel wie ein
	// fehlgeschlagener Lauf.
	if outcome.Written {
		fmt.Fprintf(out, "Geschrieben: %s (erhoben %s)\n",
			project.DisplayPath(outcome.Path), outcome.At)
		return
	}
	fmt.Fprintf(out, "Unverändert: %s — die Erhebung ist inhaltlich dieselbe (erhoben %s)\n",
		project.DisplayPath(outcome.Path), outcome.At)
}

// excludedSources ist die Zahl der Quellen, die eine Ausschlussregel übergangen
// hat. Sie steht auch dann in der Übersicht, wenn sie 0 ist: „hier wurde nichts
// gefunden" und „hier wurde nicht gesucht" sind zwei verschiedene Aussagen.
func excludedSources(result inventory.Result) int {
	total := 0
	for _, exclusion := range result.Exclusions {
		total += exclusion.Skipped
	}
	return total
}

// describeDeviations nennt die Zahl der Gruppen mit Abweichung und darin die
// widersprüchlichen. Gezählt werden Gruppen, nicht Zeilen.
func describeDeviations(deviations []inventory.Deviation) string {
	conflicting := 0
	for _, deviation := range deviations {
		if deviation.Art == inventory.DeviationConflicting {
			conflicting++
		}
	}
	if conflicting == 0 {
		return fmt.Sprintf("%d", len(deviations))
	}
	return fmt.Sprintf("%d (davon %d widersprüchlich)", len(deviations), conflicting)
}

// describeRejection nennt den angefragten und den aufgelösten Pfad, damit
// erkennbar ist, was tatsächlich gelesen worden wäre.
func describeRejection(rejection inventory.Rejection) string {
	if rejection.Resolved == "" || rejection.Resolved == rejection.Requested {
		return fmt.Sprintf("%s: %s", rejection.Requested, rejection.Reason)
	}
	return fmt.Sprintf("%s → %s: %s", rejection.Requested, rejection.Resolved, rejection.Reason)
}
