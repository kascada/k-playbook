package main

import (
	"fmt"
	"path/filepath"

	"github.com/kascada/k-playbook/installer/internal/project"
	"github.com/kascada/k-playbook/installer/internal/review"
	"github.com/kascada/k-playbook/installer/internal/review/merge"
)

// runMerge fasst einen bestehenden Review-Lauf zu Review-Input-Artefakten
// zusammen. Die fachliche Arbeit liegt in review/merge; das Kommando löst nur
// Argumente und Projektumgebung auf.
func runMerge(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("erwartet genau ein Lauf: k-playbook merge <lauf>")
	}
	runName := args[0]

	environment := project.Detect()
	if !environment.Installed {
		return fmt.Errorf("keine %s gefunden — gesucht ab %s aufwärts",
			project.ConfigFileName, project.DisplayPath(environment.SearchedFrom))
	}

	runDir := review.RunDir(project.LocalDir(environment.ProjectDir), runName)
	_, output, err := merge.Run(merge.Options{
		ProjectDir:       environment.ProjectDir,
		RunName:          runName,
		RunDir:           runDir,
		KPlaybookVersion: "unknown",
	})
	if err != nil {
		return err
	}

	fmt.Printf("Lauf %s in %s zusammengeführt.\n", runName, project.DisplayPath(filepath.Clean(runDir)))
	fmt.Printf("Geschrieben: %s\n", project.DisplayPath(output.JSON))
	fmt.Printf("Geschrieben: %s\n", project.DisplayPath(output.Markdown))
	return nil
}
