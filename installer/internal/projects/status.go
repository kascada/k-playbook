package projects

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/kascada/k-playbook/installer/internal/store"
)

type ProjectStatus struct {
	store.Project
	Playbook        PlaybookStatus      `json:"playbook"`
	Setup           CommandStatus       `json:"setup"`
	Structure       StructureStatus     `json:"structure"`
	Docs            CommandStatus       `json:"docs"`
	Remediation     RemediationStatus   `json:"remediation"`
	Tasks           TasksStatus         `json:"tasks"`
	Todo            TodoStatus          `json:"todo"`
	Reviews         ReviewsStatus       `json:"reviews"`
	Enforcement     EnforcementStatus   `json:"enforcement"`
	Git             GitStatus           `json:"git"`
	Recommendations []string            `json:"recommendations"`
	Devcontainer    *DevcontainerStatus `json:"devcontainer,omitempty"`
}

type PlaybookStatus struct {
	OK            bool   `json:"ok"`
	Status        string `json:"status"`
	Path          string `json:"path"`
	Found         bool   `json:"found"`
	SchemaVersion string `json:"schemaVersion"`
	Layout        string `json:"layout"`
	Repo          string `json:"repo"`
	UpdatedAt     string `json:"updatedAt"`
	Message       string `json:"message"`
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

type TasksStatus struct {
	OK      bool   `json:"ok"`
	Path    string `json:"path"`
	Open    int    `json:"open"`
	Done    int    `json:"done"`
	Next    string `json:"next"`
	Message string `json:"message"`
}

type TodoStatus struct {
	OK      bool   `json:"ok"`
	Path    string `json:"path"`
	Open    int    `json:"open"`
	Message string `json:"message"`
}

type ReviewsStatus struct {
	OK                bool     `json:"ok"`
	Path              string   `json:"path"`
	Reviews           int      `json:"reviews"`
	ReviewFiles       []string `json:"reviewFiles"`
	HasLog            bool     `json:"hasLog"`
	HasKnownDecisions bool     `json:"hasKnownDecisions"`
	Message           string   `json:"message"`
}

type EnforcementStatus struct {
	OK      bool     `json:"ok"`
	Path    string   `json:"path"`
	Rules   int      `json:"rules"`
	Files   []string `json:"files"`
	Message string   `json:"message"`
}

type GitStatus struct {
	OK        bool   `json:"ok"`
	Path      string `json:"path"`
	Worktree  bool   `json:"worktree"`
	Branch    string `json:"branch"`
	Changed   int    `json:"changed"`
	Untracked int    `json:"untracked"`
	Message   string `json:"message"`
}

func Status(projectPath string) (ProjectStatus, error) {
	projectPath, err := ResolveStatusPath(projectPath)
	if err != nil {
		return ProjectStatus{}, err
	}
	project, err := ProjectFromPath(projectPath)
	if err != nil {
		return ProjectStatus{}, err
	}

	return StatusFromProject(project), nil
}

func StatusFromProject(project store.Project) ProjectStatus {
	status := ProjectStatus{
		Project:     project,
		Playbook:    CheckPlaybook(project.Path),
		Setup:       CheckSetup(project.Path),
		Structure:   CheckStructure(project.Path),
		Docs:        CheckDocs(project.Path),
		Remediation: CheckRemediation(project.Path),
		Tasks:       CheckTasks(project.Path),
		Todo:        CheckTodo(project.Path),
		Reviews:     CheckReviews(project.Path),
		Enforcement: CheckEnforcement(project.Path),
		Git:         CheckGit(project.Path),
	}
	if project.Environment == store.EnvironmentDevContainer {
		devcontainer := CheckDevcontainer(project.Path)
		status.Devcontainer = &devcontainer
	}
	status.Recommendations = Recommendations(status)

	return status
}

func ResolveStatusPath(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		value = "."
	}
	normalized, err := NormalizePath(value)
	if err != nil {
		return "", err
	}

	if filepath.Base(normalized) == "k-playbook" && !exists(filepath.Join(normalized, ConfigFileName)) {
		parent := filepath.Dir(normalized)
		if exists(filepath.Join(parent, ConfigFileName)) {
			return parent, nil
		}
	}

	return normalized, nil
}

func CheckPlaybook(projectPath string) PlaybookStatus {
	path := filepath.Join(projectPath, ConfigFileName)
	status := PlaybookStatus{Path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		status.Status = "FAIL"
		if os.IsNotExist(err) {
			status.Message = "K-PLAYBOOK.yaml fehlt."
		} else {
			status.Message = "K-PLAYBOOK.yaml ist nicht lesbar."
		}
		return status
	}

	status.Found = true
	values := simpleYAMLValues(string(data))
	status.SchemaVersion = values["schema_version"]
	status.Layout = values["layout"]
	status.Repo = values["k_playbook.repo"]
	status.UpdatedAt = values["setup.updated_at"]
	if status.SchemaVersion == "1" && status.Layout == "fixed-project-k-playbook" {
		status.OK = true
		status.Status = "OK"
		status.Message = "K-PLAYBOOK.yaml plausibel."
		return status
	}

	status.Status = "FAIL"
	status.Message = "K-PLAYBOOK.yaml hat kein unterstuetztes Schema/Layout."
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

func CheckTasks(projectPath string) TasksStatus {
	path := filepath.Join(projectPath, "k-playbook", "tasks")
	status := TasksStatus{Path: path}
	open, err := numberedMarkdownFiles(path)
	if err != nil {
		status.Message = "Tasks-Verzeichnis fehlt oder ist nicht lesbar."
		return status
	}
	done, _ := numberedMarkdownFiles(filepath.Join(path, "done"))
	status.Open = len(open)
	status.Done = len(done)
	if len(open) > 0 {
		status.Next = open[0]
		status.Message = fmt.Sprintf("%d offene Tasks.", len(open))
		return status
	}

	status.OK = true
	status.Message = "Keine offenen Tasks."
	return status
}

func CheckTodo(projectPath string) TodoStatus {
	path := filepath.Join(projectPath, "k-playbook", "TODO.md")
	status := TodoStatus{Path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		status.Message = "TODO.md fehlt oder ist nicht lesbar."
		return status
	}

	for _, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, "- [ ]") {
			status.Open++
		}
	}
	if status.Open > 0 {
		status.Message = fmt.Sprintf("%d offene TODO-Checkboxen.", status.Open)
		return status
	}

	status.OK = true
	status.Message = "Keine offenen TODO-Checkboxen."
	return status
}

func CheckReviews(projectPath string) ReviewsStatus {
	path := filepath.Join(projectPath, "k-playbook", "reviews")
	status := ReviewsStatus{Path: path}
	entries, err := os.ReadDir(path)
	if err != nil {
		status.Message = "Reviews-Verzeichnis fehlt oder ist nicht lesbar."
		return status
	}

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			continue
		}
		switch name {
		case "log.md":
			status.HasLog = true
		case "known-decisions.md":
			status.HasKnownDecisions = true
		}
		if strings.HasPrefix(name, "review-") && strings.HasSuffix(name, ".md") {
			status.ReviewFiles = append(status.ReviewFiles, name)
		}
	}
	sort.Strings(status.ReviewFiles)
	status.Reviews = len(status.ReviewFiles)
	if status.Reviews > 0 && status.HasLog && status.HasKnownDecisions {
		status.OK = true
		status.Message = "Review-Struktur plausibel."
		return status
	}

	missing := []string{}
	if !status.HasLog {
		missing = append(missing, "log.md")
	}
	if !status.HasKnownDecisions {
		missing = append(missing, "known-decisions.md")
	}
	if status.Reviews == 0 {
		missing = append(missing, "review-*.md")
	}
	status.Message = "Review-Struktur unvollstaendig: " + strings.Join(missing, ", ")
	return status
}

func CheckEnforcement(projectPath string) EnforcementStatus {
	path := filepath.Join(projectPath, "k-playbook", "enforcement")
	status := EnforcementStatus{Path: path}
	entries, err := os.ReadDir(path)
	if err != nil {
		status.Message = "Enforcement-Verzeichnis fehlt oder ist nicht lesbar."
		return status
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			status.Files = append(status.Files, entry.Name())
		}
	}
	sort.Strings(status.Files)
	status.Rules = len(status.Files)
	if status.Rules > 0 {
		status.OK = true
		status.Message = fmt.Sprintf("%d Enforcement-Regeln vorhanden.", status.Rules)
		return status
	}

	status.Message = "Keine projektlokalen Enforcement-Regeln vorhanden."
	return status
}

func CheckGit(projectPath string) GitStatus {
	status := GitStatus{Path: projectPath}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	inside, err := gitOutput(ctx, projectPath, "rev-parse", "--is-inside-work-tree")
	if err != nil || strings.TrimSpace(inside) != "true" {
		status.Message = "Kein Git-Worktree."
		return status
	}
	status.Worktree = true
	branch, _ := gitOutput(ctx, projectPath, "branch", "--show-current")
	status.Branch = strings.TrimSpace(branch)
	porcelain, err := gitOutput(ctx, projectPath, "status", "--porcelain")
	if err != nil {
		status.Message = "Git-Status nicht lesbar."
		return status
	}
	for _, line := range strings.Split(strings.TrimSpace(porcelain), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.HasPrefix(line, "??") {
			status.Untracked++
		} else {
			status.Changed++
		}
	}
	if status.Changed == 0 && status.Untracked == 0 {
		status.OK = true
		status.Message = "Git-Worktree sauber."
		return status
	}

	status.Message = fmt.Sprintf("Git-Worktree dirty (%d geaendert, %d untracked).", status.Changed, status.Untracked)
	return status
}

func Recommendations(status ProjectStatus) []string {
	recommendations := []string{}
	add := func(command string) {
		if len(recommendations) >= 3 {
			return
		}
		for _, existing := range recommendations {
			if existing == command {
				return
			}
		}
		recommendations = append(recommendations, command)
	}

	if !status.Playbook.OK || !status.Structure.OK {
		add("/k-gui")
	}
	if status.Devcontainer != nil && !status.Devcontainer.OK {
		add("Devcontainer-Integration reparieren")
	}
	if status.Tasks.Open > 0 {
		add("/k-run")
	}
	if !status.Reviews.OK {
		add("/k-review")
	}
	if !status.Docs.OK {
		add("/k-code2docs")
	}

	return recommendations
}

func hasMarkdownFiles(root string) (bool, error) {
	info, err := os.Stat(root)
	if err != nil {
		return false, err
	}
	if !info.IsDir() {
		return false, fmt.Errorf("kein Verzeichnis: %s", root)
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.ToLower(filepath.Ext(entry.Name())) == ".md" {
			return true, nil
		}
	}
	return false, nil
}

func numberedMarkdownFiles(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	files := []string{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".md") || !startsWithDigit(name) {
			continue
		}
		files = append(files, name)
	}
	sort.Strings(files)
	return files, nil
}

func startsWithDigit(value string) bool {
	if value == "" {
		return false
	}
	return value[0] >= '0' && value[0] <= '9'
}

func simpleYAMLValues(content string) map[string]string {
	values := map[string]string{}
	section := ""
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || !strings.Contains(trimmed, ":") {
			continue
		}
		if isTopLevelYAMLLine(line) {
			parts := strings.SplitN(trimmed, ":", 2)
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			if value == "" {
				section = key
				continue
			}
			section = ""
			values[key] = unquoteYAMLValue(value)
			continue
		}
		if section == "" {
			continue
		}
		parts := strings.SplitN(trimmed, ":", 2)
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if value != "" {
			values[section+"."+key] = unquoteYAMLValue(value)
		}
	}
	return values
}

func unquoteYAMLValue(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"'`)
	return value
}

func gitOutput(ctx context.Context, root string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return string(output), nil
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
