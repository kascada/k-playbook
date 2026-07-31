package projects

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/store"
)

type ProjectStatus struct {
	store.Project
	Setup        CommandStatus       `json:"setup"`
	Structure    StructureStatus     `json:"structure"`
	Docs         CommandStatus       `json:"docs"`
	Remediation  RemediationStatus   `json:"remediation"`
	Devcontainer *DevcontainerStatus `json:"devcontainer,omitempty"`
}

type CommandStatus struct {
	OK       bool   `json:"ok"`
	Path     string `json:"path"`
	Command  string `json:"command"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
}

type RemediationStatus struct {
	OK      bool   `json:"ok"`
	Mode    string `json:"mode"`
	Message string `json:"message"`
}

type DevcontainerStatus struct {
	OK      bool     `json:"ok"`
	Path    string   `json:"path"`
	Missing []string `json:"missing"`
	Message string   `json:"message"`
}

func Status(projectPath string) (ProjectStatus, error) {
	project, err := ProjectFromPath(projectPath)
	if err != nil {
		return ProjectStatus{}, err
	}

	return StatusFromProject(project), nil
}

func StatusFromProject(project store.Project) ProjectStatus {
	status := ProjectStatus{
		Project:     project,
		Setup:       CheckSetup(project.Path),
		Structure:   CheckStructure(project.Path),
		Docs:        CheckDocs(project.Path),
		Remediation: CheckRemediation(project.Path),
	}
	if project.Environment == store.EnvironmentDevContainer {
		devcontainer := CheckDevcontainer(project.Path)
		status.Devcontainer = &devcontainer
	}

	return status
}

func CheckSetup(projectPath string) CommandStatus {
	status := CommandStatus{Command: "/k-setup"}
	path := filepath.Join(projectPath, ConfigFileName)
	info, err := os.Stat(path)
	if err == nil && !info.IsDir() {
		status.OK = true
		status.Path = path
		status.Severity = "ok"
		status.Message = "K-PLAYBOOK.yaml vorhanden."
		return status
	}

	status.Path = path
	status.Severity = "error"
	status.Message = "K-PLAYBOOK.yaml fehlt. Empfehlung: Projekt aus der Installer-Liste entfernen und neu einbinden."
	return status
}

func CheckStructure(projectPath string) StructureStatus {
	status, err := CheckProjectStructure(projectPath)
	if err != nil {
		return StructureStatus{Message: "Projektstruktur nicht lesbar."}
	}
	return status
}

func CheckDocs(projectPath string) CommandStatus {
	status := CommandStatus{Command: "/k-code2docs", Path: filepath.Join(projectPath, "k-playbook", "docs")}
	hasDocs, err := hasMarkdownFiles(status.Path)
	if err != nil {
		status.Message = "docs-Verzeichnis fehlt oder ist nicht lesbar."
		return status
	}
	if !hasDocs {
		status.Message = "docs-Verzeichnis enthaelt noch keine Markdown-Dateien."
		return status
	}

	status.OK = true
	status.Message = "docs-Verzeichnis enthaelt Markdown-Dateien."
	return status
}

func CheckRemediation(projectPath string) RemediationStatus {
	mode, found, err := ReadRemediationMode(projectPath)
	if err != nil {
		return RemediationStatus{Message: "Remediation-Policy nicht lesbar."}
	}
	if !found {
		return RemediationStatus{Message: "Remediation-Policy fehlt."}
	}
	if !IsValidRemediationMode(mode) {
		return RemediationStatus{Mode: string(mode), Message: "Remediation-Policy ist ungueltig."}
	}
	return RemediationStatus{OK: true, Mode: string(mode), Message: "Remediation-Policy vorhanden."}
}

func CheckDevcontainer(projectPath string) DevcontainerStatus {
	missing := missingDevcontainerIntegration(projectPath)
	status := DevcontainerStatus{OK: len(missing) == 0, Path: projectPath, Missing: missing}
	if status.OK {
		status.Message = "k-playbook ist im Container erreichbar."
	} else {
		status.Message = "DevContainer-Projekt braucht die k-playbook-Integration."
	}
	return status
}

func hasMarkdownFiles(root string) (bool, error) {
	info, err := os.Stat(root)
	if err != nil {
		return false, err
	}
	if !info.IsDir() {
		return false, fmt.Errorf("kein Verzeichnis: %s", root)
	}

	found := false
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || entry.Name() == "node_modules" || entry.Name() == ".venv" || entry.Name() == "venv" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.ToLower(filepath.Ext(path)) == ".md" {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil && !errors.Is(err, filepath.SkipAll) {
		return false, err
	}
	return found, nil
}

func missingDevcontainerIntegration(projectPath string) []string {
	devcontainerJSON := filepath.Join(projectPath, ".devcontainer", "devcontainer.json")
	setupScript := filepath.Join(projectPath, ".devcontainer", "setup-k-playbook.sh")
	missing := []string{}

	data, err := os.ReadFile(devcontainerJSON)
	if err != nil {
		return []string{".devcontainer/devcontainer.json fehlt oder ist nicht lesbar"}
	}
	content := string(data)
	if !strings.Contains(content, devcontainerMountEntry()) {
		missing = append(missing, "Bind-Mount ~/dev/k-playbook -> /workspaces/k-playbook")
	}
	if !hasDevcontainerCommand(content, "postCreateCommand", "sudo bash .devcontainer/setup-k-playbook.sh --install-security-tools") {
		missing = append(missing, "postCreateCommand fuer setup-k-playbook.sh")
	}
	if !hasDevcontainerCommand(content, "postStartCommand", "sudo bash .devcontainer/setup-k-playbook.sh") {
		missing = append(missing, "postStartCommand fuer setup-k-playbook.sh")
	}
	if info, err := os.Stat(setupScript); err != nil || info.IsDir() || info.Mode()&0o111 == 0 {
		missing = append(missing, ".devcontainer/setup-k-playbook.sh fehlt oder ist nicht ausfuehrbar")
	}

	return missing
}

func devcontainerMountEntry() string {
	return "source=${localEnv:HOME}/dev/k-playbook,target=/workspaces/k-playbook,type=bind"
}

func hasDevcontainerCommand(content string, name string, desired string) bool {
	pattern := regexp.MustCompile(`"` + regexp.QuoteMeta(name) + `"\s*:\s*"((?:\\.|[^"\\])*)"`)
	match := pattern.FindStringSubmatch(content)
	if len(match) != 2 {
		return false
	}

	var value string
	if err := json.Unmarshal([]byte(`"`+match[1]+`"`), &value); err != nil {
		return false
	}
	for _, part := range strings.Split(value, "&&") {
		if strings.TrimSpace(part) == desired {
			return true
		}
	}
	return false
}
