package main

import (
	"fmt"
	"path/filepath"
	"runtime/debug"
	"strings"

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
		ProjectDir:          environment.ProjectDir,
		RunName:             runName,
		RunDir:              runDir,
		KPlaybookVersion:    kPlaybookVersion(),
		SeverityMappingPath: merge.SeverityCatalog(environment.PlaybookDir),
		LocalResultsDir:     review.ResultsDir(project.LocalDir(environment.ProjectDir)),
	})
	if err != nil {
		return err
	}

	fmt.Printf("Lauf %s in %s zusammengeführt.\n", runName, project.DisplayPath(filepath.Clean(runDir)))
	fmt.Printf("Geschrieben: %s\n", project.DisplayPath(output.JSON))
	fmt.Printf("Geschrieben: %s\n", project.DisplayPath(output.Markdown))
	return nil
}

func kPlaybookVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info == nil {
		return "unknown"
	}
	settings := map[string]string{}
	for _, setting := range info.Settings {
		settings[setting.Key] = setting.Value
	}
	return formatBuildVersion(info.Main.Version, settings)
}

func formatBuildVersion(version string, settings map[string]string) string {
	version = strings.TrimSpace(version)
	revision := strings.TrimSpace(settings["vcs.revision"])
	dirty := settings["vcs.modified"] == "true"
	if version != "" && version != "(devel)" {
		if dirty {
			return version + "-dirty"
		}
		return version
	}
	if revision == "" {
		return "unknown"
	}
	short := revision
	if len(short) > 7 {
		short = short[:7]
	}
	if version == "(devel)" {
		short = "(devel)+" + short
	}
	if dirty {
		short += "-dirty"
	}
	return short
}
