package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/store"
)

func RenderProjects(file store.ProjectsFile, styled bool) string {
	_ = styled

	if len(file.Projects) == 0 {
		return "No projects selected.\n"
	}

	projects := append([]store.Project{}, file.Projects...)
	sort.Slice(projects, func(i int, j int) bool {
		return projects[i].Path < projects[j].Path
	})

	var builder strings.Builder
	for _, project := range projects {
		selected := " "
		if project.Selected {
			selected = "x"
		}
		builder.WriteString(fmt.Sprintf("[%s] %s\n", selected, projectLabel(project)))
	}

	return builder.String()
}

func projectLabel(project store.Project) string {
	details := string(project.Environment)
	if len(project.Detected) > 0 {
		details = details + ", " + strings.Join(project.Detected, ", ")
	}

	return fmt.Sprintf("%s (%s)", project.Path, details)
}
