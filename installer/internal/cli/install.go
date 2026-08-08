package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/kascada/k-playbook/installer/internal/install"
	"github.com/kascada/k-playbook/installer/payload"
)

// resolvePlaybookDir turns an optional path argument into a k-playbook directory.
// Without an argument it discovers one from the working directory, the same way
// the commands do.
func resolvePlaybookDir(args []string) (string, error) {
	start := "."
	if len(args) == 1 {
		start = args[0]
	}
	return install.Discover(start)
}

func newInitCommand() *cobra.Command {
	var repoRoot, vcs, remediation string
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "init [projektpfad]",
		Short: "Installiert k-playbook in ein Unterverzeichnis des Projekts",
		Long: "Legt <projekt>/k-playbook/ an, entpackt die mitgelieferte Installation nach _dist/,\n" +
			"schreibt K-PLAYBOOK.yaml, verlinkt die Assistant-Verzeichnisse und ergaenzt die .gitignore.\n\n" +
			"Mehrfach ausfuehrbar: eine vorhandene K-PLAYBOOK.yaml wird nie ueberschrieben.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectRoot := "."
			if len(args) == 1 {
				projectRoot = args[0]
			}
			result, err := install.Init(projectRoot, install.Options{
				RepoRoot:    repoRoot,
				VCS:         vcs,
				Remediation: remediation,
			})
			if err != nil {
				return err
			}
			return reportResult("Installiert", result, asJSON)
		},
	}

	cmd.Flags().StringVar(&repoRoot, "repo-root", "", "Code-Root relativ zum k-playbook-Verzeichnis (Default: ..)")
	cmd.Flags().StringVar(&vcs, "vcs", "", "git oder none (Default: automatisch erkannt)")
	cmd.Flags().StringVar(&remediation, "remediation", "", "direct-allowed, task-first oder task-branch-pr")
	cmd.Flags().BoolVar(&asJSON, "json", false, "Ergebnis als JSON ausgeben")
	return cmd
}

func newUpdateCommand() *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "update [pfad]",
		Short: "Ersetzt die mitgelieferte Installation unter _dist",
		Long: "Entpackt die Payload dieses Binaries neu nach _dist/. Alles ausserhalb von _dist\n" +
			"bleibt unangetastet, also Config, Tasks, Reviews, Ergebnisse und eigene Regeln.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			playbookDir, err := resolvePlaybookDir(args)
			if err != nil {
				return err
			}
			result, err := install.Update(playbookDir)
			if err != nil {
				return err
			}
			return reportResult("Aktualisiert", result, asJSON)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Ergebnis als JSON ausgeben")
	return cmd
}

func newRestoreCommand() *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "restore [pfad]",
		Short: "Stellt _dist nach einem git clone wieder her",
		Long: "_dist steht in der .gitignore und fehlt daher nach einem frischen Clone.\n" +
			"restore entpackt es erneut und verlinkt die Assistant-Verzeichnisse.\n" +
			"Weicht die Payload dieses Binaries von k_playbook.version ab, wird das gemeldet.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			playbookDir, err := resolvePlaybookDir(args)
			if err != nil {
				return err
			}
			result, err := install.Restore(playbookDir)
			if err != nil {
				return err
			}
			return reportResult("Wiederhergestellt", result, asJSON)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Ergebnis als JSON ausgeben")
	return cmd
}

func newMigrateCommand() *cobra.Command {
	var dryRun, asJSON bool

	cmd := &cobra.Command{
		Use:   "migrate [projektpfad]",
		Short: "Stellt ein Projekt von schema_version 1 auf 2 um",
		Long: "Verschiebt K-PLAYBOOK.yaml nach <projekt>/k-playbook/, kuerzt die paths.*-Werte um das\n" +
			"k-playbook/-Praefix, hebt project.repo_root eine Ebene an, ersetzt k_playbook.repo durch\n" +
			"dist/version/installed_at und ergaenzt den overlay-Block.\n\n" +
			"Tasks, Reviews, Ergebnisse und Docs liegen bereits richtig und werden nicht bewegt.\n" +
			"Unbekannte Felder bleiben unveraendert erhalten.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectRoot := "."
			if len(args) == 1 {
				projectRoot = args[0]
			}
			result, err := install.Migrate(projectRoot, dryRun)
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(result)
			}

			if dryRun {
				fmt.Printf("Probelauf fuer %s\n", result.ProjectRoot)
			} else {
				fmt.Printf("Migriert: %s\n", result.PlaybookDir)
			}
			for _, change := range result.Changes {
				fmt.Printf("  - %s\n", change)
			}
			for _, note := range result.Notes {
				fmt.Printf("  Hinweis: %s\n", note)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "zeigt die Aenderungen, ohne etwas zu schreiben")
	cmd.Flags().BoolVar(&asJSON, "json", false, "Ergebnis als JSON ausgeben")
	return cmd
}

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Zeigt die Payload-Version dieses Binaries",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(payload.Version())
			return nil
		},
	}
}

func reportResult(action string, result install.Result, asJSON bool) error {
	if asJSON {
		return writeJSON(result)
	}

	fmt.Printf("%s: %s (Version %s)\n", action, result.PlaybookDir, result.Version)
	if result.ConfigWritten {
		fmt.Printf("  - %s angelegt\n", install.ConfigFileName)
	}
	for _, entry := range result.Created {
		fmt.Printf("  - %s angelegt\n", entry)
	}
	for _, entry := range result.Linked {
		fmt.Printf("  - %s\n", entry)
	}
	for _, note := range result.Notes {
		fmt.Printf("  Hinweis: %s\n", note)
	}
	return nil
}

func writeJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
