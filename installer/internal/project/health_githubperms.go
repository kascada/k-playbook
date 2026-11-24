package project

import (
	"context"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/health"
)

// Die Quelle der Umgebungsprüfung für die Berechtigungen der Clients (Task
// 190): eine Warnung, wenn ein Projekt mit GitHub-Dienst die Berechtigungen
// nicht trägt oder ihnen widerspricht, und wenn ohne Dienst noch Einträge von
// k-playbook stehen. Gemessen wird gegen die Entscheidung in Task 190: Eine
// von git erfasste Zieldatei und eine Datei hinter einem Link bleiben bewusst
// ungeschrieben und sind keine Warnung; eine unbeantwortete Frage nach dem
// Dienst (Zugangsdatei mit Fehlern) meldet die Quelle github schon.
func init() {
	health.Register(healthGitHubPermissions)
}

// gitHubPermissionsHint ist die Abhilfe für fehlende und veraltete Einträge.
const gitHubPermissionsHint = "k-playbook upgrade ausführen oder in der Oberfläche bei der MCP-Registrierung „Einrichten“ drücken; Hintergrund: k-playbook:/instructions/on-demand/github.md."

func healthGitHubPermissions(_ context.Context, projectDir string) []health.Problem {
	report := CheckGitHubPermissions(projectDir)
	if !report.Known {
		return nil
	}
	var problems []health.Problem
	for _, status := range report.Statuses {
		switch status.State {
		case GitHubPermissionsMissing:
			problems = append(problems, health.Problem{
				Level:   health.LevelWarning,
				Area:    health.AreaGitHub,
				Message: "Die Berechtigungen für GitHub über run fehlen für " + status.Assistant + " in " + status.Path + " (Dienst " + strings.Join(report.Services, ", ") + "); nacktes gh und die Netzbefehle von git sind dort nicht gesperrt.",
				Hint:    gitHubPermissionsHint,
			})
		case GitHubPermissionsStale:
			problems = append(problems, health.Problem{
				Level:   health.LevelWarning,
				Area:    health.AreaGitHub,
				Message: status.Path + " trägt die Berechtigungen für GitHub über run, aber dem Projekt ist kein Dienst zugeordnet; nacktes gh, hier der erlaubte Weg, ist gesperrt.",
				Hint:    gitHubPermissionsHint,
			})
		case GitHubPermissionsUnreadable:
			if len(report.Services) > 0 {
				problems = append(problems, health.Problem{
					Level:   health.LevelWarning,
					Area:    health.AreaGitHub,
					Message: "Die Berechtigungen für GitHub über run lassen sich für " + status.Assistant + " nicht eintragen: " + status.Path + " — " + status.Detail + ".",
					Hint:    "Die Datei so richten, dass sie ein JSON-Objekt ist und die Berechtigungen als Objekt bzw. Liste trägt; danach k-playbook upgrade.",
				})
			}
		}
	}

	byFile := map[string][]GitHubPermissionConflict{}
	var files []string
	for _, conflict := range report.Conflicts {
		if _, ok := byFile[conflict.Path]; !ok {
			files = append(files, conflict.Path)
		}
		byFile[conflict.Path] = append(byFile[conflict.Path], conflict)
	}
	for _, file := range files {
		conflicts := byFile[file]
		rules := make([]string, 0, len(conflicts))
		effective := false
		for _, conflict := range conflicts {
			rules = append(rules, "„"+conflict.Rule+"“ ("+conflict.Effect+")")
			effective = effective || conflict.Effective
		}
		message := file + " widerspricht der Regel „GitHub über run“: " + strings.Join(rules, ", ") + "."
		if !effective {
			message += " Die Einträge von k-playbook überstimmen das derzeit."
		}
		problems = append(problems, health.Problem{
			Level:   health.LevelWarning,
			Area:    health.AreaGitHub,
			Message: message,
			Hint:    "Die genannten Einträge entfernen; k-playbook upgrade --check nennt je Eintrag eine Beispielzeile und den Vorschlag.",
		})
	}
	return problems
}
