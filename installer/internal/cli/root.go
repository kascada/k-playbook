package cli

import (
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"

	"github.com/kascada/k-playbook/installer/internal/pathcontract"
	"github.com/kascada/k-playbook/installer/internal/projects"
	"github.com/kascada/k-playbook/installer/internal/store"
	"github.com/kascada/k-playbook/installer/internal/ui"
	"github.com/kascada/k-playbook/installer/internal/webui"
)

func Execute() {
	rootCmd := newRootCommand()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "k-playbook-installer",
		Short: "Deterministischer Installer fuer k-playbook",
		RunE: func(cmd *cobra.Command, args []string) error {
			return webui.Run()
		},
	}

	rootCmd.AddCommand(newStatusCommand())
	rootCmd.AddCommand(newGUICommand())
	rootCmd.AddCommand(newProjectsCommand())

	return rootCmd
}

func newStatusCommand() *cobra.Command {
	fix := false
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Prueft den ~/dev/k-playbook Pfadvertrag",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := pathcontract.Check()
			if err != nil {
				return err
			}

			if fix && !result.OK {
				if err := pathcontract.Repair(result); err != nil {
					return err
				}
				result, err = pathcontract.Check()
				if err != nil {
					return err
				}
			}

			fmt.Print(ui.RenderPathStatus(result, false))
			if !result.OK {
				return fmt.Errorf("Pfadvertrag nicht erfuellt: %s", result.Code)
			}

			return nil
		},
	}
	cmd.Flags().BoolVar(&fix, "fix", false, "legt den Symlink an, wenn ~/dev/k-playbook fehlt und das aktuelle Repo sicher erkannt wurde")

	return cmd
}

func newGUICommand() *cobra.Command {
	return &cobra.Command{
		Use:   "gui",
		Short: "Startet die lokale Browser-GUI",
		RunE: func(cmd *cobra.Command, args []string) error {
			return webui.Run()
		},
	}
}

func newProjectsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "projects",
		Short: "Zeigt oder sammelt die lokale Projekt-Auswahl",
		RunE: func(cmd *cobra.Command, args []string) error {
			return listProjects()
		},
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "Zeigt gespeicherte Projekte",
		RunE: func(cmd *cobra.Command, args []string) error {
			return listProjects()
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "scan",
		Short: "Sucht Projektkandidaten unter ~/dev und zeigt sie an",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requirePathContract(); err != nil {
				return err
			}
			candidates, err := projects.ScanDefaultDev()
			if err != nil {
				return err
			}
			sort.Slice(candidates, func(i int, j int) bool {
				return candidates[i].Path < candidates[j].Path
			})
			for _, candidate := range candidates {
				fmt.Printf("%s\t%s\n", candidate.Environment, candidate.Path)
			}
			return nil
		},
	})

	addEnvironment := string(store.EnvironmentUnknown)
	addCmd := &cobra.Command{
		Use:   "add <path>",
		Short: "Fuegt ein Projekt zur lokalen Auswahl hinzu",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			project, err := projects.ProjectFromPath(args[0])
			if err != nil {
				return err
			}
			if !isValidEnvironment(addEnvironment) {
				return fmt.Errorf("ungueltige Umgebung: %s", addEnvironment)
			}
			if addEnvironment != string(store.EnvironmentUnknown) {
				project.Environment = store.ProjectEnvironment(addEnvironment)
			}
			createdConfig, err := projects.EnsureConfig(project.Path, projects.RemediationModeDirectAllowed)
			if err != nil {
				return err
			}

			file, err := store.LoadProjects()
			if err != nil {
				return err
			}
			file = store.UpsertProject(file, project)
			if err := store.SaveProjects(file); err != nil {
				return err
			}
			fmt.Printf("Projekt gespeichert: %s (%s)\n", project.Path, project.Environment)
			if createdConfig {
				fmt.Printf("K-PLAYBOOK.yaml angelegt: %s (remediation: %s)\n", project.Path, projects.RemediationModeDirectAllowed)
			}
			return nil
		},
	}
	addCmd.Flags().StringVar(&addEnvironment, "env", string(store.EnvironmentUnknown), "Umgebung: unknown, plain, venv oder devcontainer")
	cmd.AddCommand(addCmd)

	return cmd
}

func listProjects() error {
	if err := requirePathContract(); err != nil {
		return err
	}

	file, err := store.LoadProjects()
	if err != nil {
		return err
	}

	fmt.Print(ui.RenderProjects(file, false))
	return nil
}

func requirePathContract() error {
	result, err := pathcontract.Check()
	if err != nil {
		return err
	}
	if !result.OK {
		return fmt.Errorf("Pfadvertrag nicht erfuellt: %s", result.Code)
	}

	return nil
}

func isValidEnvironment(value string) bool {
	switch store.ProjectEnvironment(value) {
	case store.EnvironmentUnknown, store.EnvironmentPlain, store.EnvironmentVenv, store.EnvironmentDevContainer:
		return true
	default:
		return false
	}
}
