package projects

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/kascada/k-playbook/installer/internal/store"
)

type SmokeSeverity string

const (
	SmokeSeverityOK    SmokeSeverity = "ok"
	SmokeSeverityWarn  SmokeSeverity = "warn"
	SmokeSeverityError SmokeSeverity = "error"
)

type SmokeCheck struct {
	Name     string        `json:"name"`
	OK       bool          `json:"ok"`
	Severity SmokeSeverity `json:"severity"`
	Message  string        `json:"message"`
	Output   string        `json:"output,omitempty"`
}

type SmokeResult struct {
	Path    string       `json:"path"`
	Name    string       `json:"name"`
	OK      bool         `json:"ok"`
	Checks  []SmokeCheck `json:"checks"`
	Message string       `json:"message"`
}

type SmokeAllResult struct {
	OK       bool          `json:"ok"`
	Projects []SmokeResult `json:"projects"`
	Message  string        `json:"message"`
}

func Smoke(projectPath string) (SmokeResult, error) {
	project, err := ProjectFromPath(projectPath)
	if err != nil {
		return SmokeResult{}, err
	}
	return SmokeFromProject(project), nil
}

func SmokeFromProject(project store.Project) SmokeResult {
	result := SmokeResult{Path: project.Path, Name: project.Name}
	status := StatusFromProject(project)
	result.Checks = append(result.Checks, smokeProjectConfigCheck(status))
	result.Checks = append(result.Checks, smokeGHChecks(status)...)
	result.OK = smokeChecksOK(result.Checks)
	if result.OK {
		result.Message = "Smoke-Test erfolgreich."
	} else {
		result.Message = "Smoke-Test mit Hinweisen oder Fehlern."
	}
	return result
}

func SmokeAll(file store.ProjectsFile) SmokeAllResult {
	result := SmokeAllResult{Projects: make([]SmokeResult, 0, len(file.Projects))}
	for _, project := range file.Projects {
		result.Projects = append(result.Projects, SmokeFromProject(project))
	}
	result.OK = true
	for _, project := range result.Projects {
		if !project.OK {
			result.OK = false
			break
		}
	}
	if result.OK {
		result.Message = "Smoke-Test fuer alle Projekte erfolgreich."
	} else {
		result.Message = "Smoke-Test fuer mindestens ein Projekt mit Hinweisen oder Fehlern."
	}
	return result
}

func smokeProjectConfigCheck(status ProjectStatus) SmokeCheck {
	if status.Playbook.OK && status.ProjectRoot.OK {
		return SmokeCheck{Name: "config", OK: true, Severity: SmokeSeverityOK, Message: "K-PLAYBOOK.yaml und Git-Konfiguration sind plausibel."}
	}
	return SmokeCheck{Name: "config", Severity: SmokeSeverityError, Message: "K-PLAYBOOK.yaml oder Git-Konfiguration ist unvollstaendig. Erst in /k-gui korrigieren."}
}

func smokeGHChecks(status ProjectStatus) []SmokeCheck {
	if status.ProjectRoot.VCS == string(ProjectVCSNone) {
		return []SmokeCheck{{Name: "gh", OK: true, Severity: SmokeSeverityOK, Message: "Projekt ist ohne Git konfiguriert; gh wird uebersprungen."}}
	}
	if !status.ProjectRoot.OK {
		return []SmokeCheck{{Name: "gh", Severity: SmokeSeverityError, Message: "gh nicht geprueft, weil die Git-Konfiguration unvollstaendig ist."}}
	}

	checks := []SmokeCheck{}
	versionOutput, err := runSmokeCommand(status.ProjectRoot.Path, "gh", "--version")
	if err != nil {
		return []SmokeCheck{{Name: "gh", Severity: SmokeSeverityError, Message: "GitHub CLI `gh` ist nicht verfuegbar oder nicht ausfuehrbar.", Output: strings.TrimSpace(versionOutput)}}
	}
	checks = append(checks, SmokeCheck{Name: "gh-version", OK: true, Severity: SmokeSeverityOK, Message: "GitHub CLI ist verfuegbar.", Output: firstLine(versionOutput)})

	authOutput, err := runSmokeCommand(status.ProjectRoot.Path, "gh", "auth", "status")
	if err != nil {
		checks = append(checks, SmokeCheck{Name: "gh-auth", Severity: SmokeSeverityWarn, Message: "GitHub CLI ist nicht authentifiziert oder GitHub-Auth ist nicht nutzbar.", Output: strings.TrimSpace(authOutput)})
		return checks
	}
	checks = append(checks, SmokeCheck{Name: "gh-auth", OK: true, Severity: SmokeSeverityOK, Message: "GitHub CLI Auth funktioniert.", Output: firstLine(authOutput)})
	return checks
}

func smokeChecksOK(checks []SmokeCheck) bool {
	for _, check := range checks {
		if !check.OK {
			return false
		}
	}
	return true
}

func runSmokeCommand(dir string, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return string(output), fmt.Errorf("%s timeout", name)
	}
	return string(output), err
}

func firstLine(value string) string {
	for _, line := range strings.Split(strings.TrimSpace(value), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}
