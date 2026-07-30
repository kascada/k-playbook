package projects

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kascada/k-playbook/installer/internal/pathcontract"
	"github.com/kascada/k-playbook/installer/internal/store"
)

const ConfigFileName = "K-PLAYBOOK.yaml"

const knownDecisionsTemplate = `# Known Decisions

Eintraege in dieser Datei dokumentieren bewusste Design-Entscheidungen und bekannte Trade-offs.
Bei Reviews (/k-review, /k-remediation) werden passende Befunde automatisch als "Akzeptiert (A)" eingestuft - kein manuelles Durchgehen noetig.

Format je Eintrag: ID (KD-NNN), Kurztitel, Bereich (Datei/Modul/Konzept), Begruendung, Datum.

---

<!-- Eintraege folgen hier -->
`

const todoTemplate = `# TODO

`

type RemediationMode string

type StructureStatus struct {
	OK      bool     `json:"ok"`
	Missing []string `json:"missing"`
	Message string   `json:"message"`
}

const (
	RemediationModeTaskBranchPR  RemediationMode = "task-branch-pr"
	RemediationModeTaskFirst     RemediationMode = "task-first"
	RemediationModeDirectAllowed RemediationMode = "direct-allowed"
)

func NormalizePath(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("Projektpfad ist leer")
	}

	if value == "~" || strings.HasPrefix(value, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("user home ermitteln: %w", err)
		}
		if value == "~" {
			value = home
		} else {
			value = filepath.Join(home, strings.TrimPrefix(value, "~/"))
		}
	}

	abs, err := filepath.Abs(value)
	if err != nil {
		return "", fmt.Errorf("Projektpfad normalisieren: %w", err)
	}

	return filepath.Clean(abs), nil
}

func ProjectFromPath(path string) (store.Project, error) {
	normalized, err := NormalizePath(path)
	if err != nil {
		return store.Project{}, err
	}

	info, err := os.Stat(normalized)
	if err != nil {
		return store.Project{}, fmt.Errorf("Projektpfad pruefen: %w", err)
	}
	if !info.IsDir() {
		return store.Project{}, fmt.Errorf("Projektpfad ist kein Verzeichnis: %s", normalized)
	}

	environment, detected := DetectEnvironment(normalized)
	return store.Project{
		Path:        normalized,
		Name:        filepath.Base(normalized),
		Environment: environment,
		Selected:    true,
		Detected:    detected,
	}, nil
}

func DetectEnvironment(path string) (store.ProjectEnvironment, []string) {
	detected := []string{}

	if exists(filepath.Join(path, ".devcontainer", "devcontainer.json")) {
		detected = append(detected, ".devcontainer/devcontainer.json")
		return store.EnvironmentDevContainer, detected
	}

	for _, name := range []string{".venv", "venv"} {
		candidate := filepath.Join(path, name)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			detected = append(detected, name+"/")
		}
	}

	plainMarkers := existingPlainProjectMarkers(path)
	if len(plainMarkers) > 0 {
		detected = append(detected, plainMarkers...)
		return store.EnvironmentPlain, detected
	}
	if len(detected) > 0 {
		return store.EnvironmentPlain, detected
	}

	return store.EnvironmentUnknown, detected
}

func EnsureConfig(projectPath string, remediationMode RemediationMode) (bool, error) {
	normalized, err := NormalizePath(projectPath)
	if err != nil {
		return false, err
	}
	if remediationMode == "" {
		remediationMode = RemediationModeDirectAllowed
	}
	if !IsValidRemediationMode(remediationMode) {
		return false, fmt.Errorf("ungueltiger Remediation-Modus: %s", remediationMode)
	}

	info, err := os.Stat(normalized)
	if err != nil {
		return false, fmt.Errorf("Projektpfad pruefen: %w", err)
	}
	if !info.IsDir() {
		return false, fmt.Errorf("Projektpfad ist kein Verzeichnis: %s", normalized)
	}

	path := filepath.Join(normalized, ConfigFileName)
	if info, err := os.Stat(path); err == nil {
		if info.IsDir() {
			return false, fmt.Errorf("%s ist ein Verzeichnis", path)
		}
		return false, nil
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("%s pruefen: %w", ConfigFileName, err)
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return false, fmt.Errorf("%s anlegen: %w", ConfigFileName, err)
	}
	defer file.Close()

	if _, err := file.WriteString(MinimalConfig(time.Now(), remediationMode)); err != nil {
		return false, fmt.Errorf("%s schreiben: %w", ConfigFileName, err)
	}

	return true, nil
}

func CheckProjectStructure(projectPath string) (StructureStatus, error) {
	normalized, err := NormalizePath(projectPath)
	if err != nil {
		return StructureStatus{}, err
	}

	missing := []string{}
	for _, rel := range fixedProjectDirs() {
		path := filepath.Join(normalized, filepath.FromSlash(rel))
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				missing = append(missing, rel+"/")
				continue
			}
			return StructureStatus{}, fmt.Errorf("%s pruefen: %w", rel, err)
		}
		if !info.IsDir() {
			missing = append(missing, rel+"/ (kein Verzeichnis)")
		}
	}
	for _, file := range fixedProjectFiles() {
		path := filepath.Join(normalized, filepath.FromSlash(file.Path))
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				missing = append(missing, file.Path)
				continue
			}
			return StructureStatus{}, fmt.Errorf("%s pruefen: %w", file.Path, err)
		}
		if info.IsDir() {
			missing = append(missing, file.Path+" (kein Datei)")
		}
	}

	status := StructureStatus{OK: len(missing) == 0, Missing: missing}
	if status.OK {
		status.Message = "Projektstruktur vollstaendig."
	} else {
		status.Message = fmt.Sprintf("Projektstruktur unvollstaendig: %d Pfade fehlen oder sind falsch.", len(missing))
	}
	return status, nil
}

func CompleteProjectStructure(projectPath string) (StructureStatus, error) {
	normalized, err := NormalizePath(projectPath)
	if err != nil {
		return StructureStatus{}, err
	}

	for _, rel := range fixedProjectDirs() {
		path := filepath.Join(normalized, filepath.FromSlash(rel))
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return StructureStatus{}, fmt.Errorf("%s ist kein Verzeichnis", rel)
		} else if err != nil && !os.IsNotExist(err) {
			return StructureStatus{}, fmt.Errorf("%s pruefen: %w", rel, err)
		}
		if err := os.MkdirAll(path, 0o755); err != nil {
			return StructureStatus{}, fmt.Errorf("%s anlegen: %w", rel, err)
		}
	}

	for _, file := range fixedProjectFiles() {
		path := filepath.Join(normalized, filepath.FromSlash(file.Path))
		if info, err := os.Stat(path); err == nil {
			if info.IsDir() {
				return StructureStatus{}, fmt.Errorf("%s ist ein Verzeichnis", file.Path)
			}
			continue
		} else if !os.IsNotExist(err) {
			return StructureStatus{}, fmt.Errorf("%s pruefen: %w", file.Path, err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return StructureStatus{}, fmt.Errorf("%s Elternverzeichnis anlegen: %w", file.Path, err)
		}
		if err := os.WriteFile(path, []byte(file.Content), 0o644); err != nil {
			return StructureStatus{}, fmt.Errorf("%s anlegen: %w", file.Path, err)
		}
	}

	return CheckProjectStructure(normalized)
}

func ReadRemediationMode(projectPath string) (RemediationMode, bool, error) {
	path, err := configPath(projectPath)
	if err != nil {
		return "", false, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false, fmt.Errorf("%s lesen: %w", ConfigFileName, err)
	}

	lines := strings.Split(string(data), "\n")
	inRemediation := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if isTopLevelYAMLLine(line) {
			inRemediation = strings.TrimSuffix(trimmed, ":") == "remediation"
			continue
		}
		if inRemediation && strings.HasPrefix(trimmed, "mode:") {
			mode := RemediationMode(strings.TrimSpace(strings.TrimPrefix(trimmed, "mode:")))
			return mode, true, nil
		}
	}

	return "", false, nil
}

func UpdateRemediationMode(projectPath string, remediationMode RemediationMode) error {
	if remediationMode == "" {
		remediationMode = RemediationModeDirectAllowed
	}
	if !IsValidRemediationMode(remediationMode) {
		return fmt.Errorf("ungueltiger Remediation-Modus: %s", remediationMode)
	}

	path, err := configPath(projectPath)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("%s lesen: %w", ConfigFileName, err)
	}

	content := replaceRemediationBlock(string(data), remediationBlock(remediationMode))
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("%s schreiben: %w", ConfigFileName, err)
	}
	return nil
}

func MinimalConfig(updatedAt time.Time, remediationMode RemediationMode) string {
	if remediationMode == "" {
		remediationMode = RemediationModeDirectAllowed
	}
	return fmt.Sprintf(`schema_version: 1
layout: fixed-project-k-playbook

k_playbook:
  repo: ~/dev/k-playbook

setup:
  updated_at: %s

%s`, updatedAt.Format("2006-01-02"), remediationBlock(remediationMode))
}

func remediationBlock(remediationMode RemediationMode) string {
	policy := RemediationPolicy(remediationMode)
	return fmt.Sprintf(`remediation:
  mode: %s
  target: .
  grouping: true
  quick_wins: true
  branch_prefix: remediation/
  pr_required: %t
  direct_fixes: %t
`, remediationMode, policy.PRRequired, policy.DirectFixes)
}

type RemediationPolicyConfig struct {
	PRRequired  bool
	DirectFixes bool
}

func RemediationPolicy(mode RemediationMode) RemediationPolicyConfig {
	switch mode {
	case RemediationModeTaskBranchPR:
		return RemediationPolicyConfig{PRRequired: true, DirectFixes: false}
	case RemediationModeTaskFirst:
		return RemediationPolicyConfig{PRRequired: false, DirectFixes: true}
	default:
		return RemediationPolicyConfig{PRRequired: false, DirectFixes: true}
	}
}

func IsValidRemediationMode(mode RemediationMode) bool {
	switch mode {
	case RemediationModeTaskBranchPR, RemediationModeTaskFirst, RemediationModeDirectAllowed:
		return true
	default:
		return false
	}
}

func configPath(projectPath string) (string, error) {
	normalized, err := NormalizePath(projectPath)
	if err != nil {
		return "", err
	}
	return filepath.Join(normalized, ConfigFileName), nil
}

func replaceRemediationBlock(content string, block string) string {
	lines := strings.Split(content, "\n")
	start := -1
	end := len(lines)
	for index, line := range lines {
		if isTopLevelYAMLLine(line) && strings.TrimSpace(line) == "remediation:" {
			start = index
			break
		}
	}
	if start == -1 {
		content = strings.TrimRight(content, "\n")
		if content == "" {
			return block
		}
		return content + "\n\n" + block
	}

	for index := start + 1; index < len(lines); index++ {
		line := lines[index]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if isTopLevelYAMLLine(line) {
			end = index
			break
		}
	}

	newLines := append([]string{}, lines[:start]...)
	newLines = append(newLines, strings.Split(strings.TrimRight(block, "\n"), "\n")...)
	newLines = append(newLines, lines[end:]...)
	return strings.TrimRight(strings.Join(newLines, "\n"), "\n") + "\n"
}

func isTopLevelYAMLLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	return trimmed != "" && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") && strings.Contains(trimmed, ":")
}

type fixedFile struct {
	Path    string
	Content string
}

func fixedProjectDirs() []string {
	return []string{
		"k-playbook",
		"k-playbook/tasks",
		"k-playbook/tasks/done",
		"k-playbook/checks",
		"k-playbook/reviews",
		"k-playbook/guidelines",
		"k-playbook/enforcement",
		"k-playbook/docs",
	}
}

func fixedProjectFiles() []fixedFile {
	return []fixedFile{
		{Path: "k-playbook/TODO.md", Content: todoTemplate},
		{Path: "k-playbook/reviews/known-decisions.md", Content: knownDecisionsTemplate},
	}
}

func ScanDefaultDev() ([]store.Project, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("user home ermitteln: %w", err)
	}

	return Scan(filepath.Join(home, "dev"))
}

func Scan(root string) ([]store.Project, error) {
	root, err := NormalizePath(root)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("Scan-Root pruefen: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("Scan-Root ist kein Verzeichnis: %s", root)
	}

	expected, _ := pathcontract.ExpectedPath()
	expectedReal := realPath(expected)
	found := map[string]store.Project{}

	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if !entry.IsDir() {
			return nil
		}

		if path != root {
			name := entry.Name()
			if shouldSkipDir(name) {
				return filepath.SkipDir
			}
		}

		if depth(root, path) > 3 {
			return filepath.SkipDir
		}

		if samePath(realPath(path), expectedReal) {
			return filepath.SkipDir
		}

		if !looksLikeProject(path) {
			return nil
		}

		project, err := ProjectFromPath(path)
		if err == nil {
			found[project.Path] = project
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("Projekte suchen: %w", err)
	}

	projects := make([]store.Project, 0, len(found))
	for _, project := range found {
		projects = append(projects, project)
	}

	return projects, nil
}

func looksLikeProject(path string) bool {
	markers := []string{
		".git",
		ConfigFileName,
		"pyproject.toml",
		"package.json",
		"go.mod",
		filepath.Join(".devcontainer", "devcontainer.json"),
	}

	for _, marker := range markers {
		if exists(filepath.Join(path, marker)) {
			return true
		}
	}

	return false
}

func existingPlainProjectMarkers(path string) []string {
	markers := []string{
		".git",
		ConfigFileName,
		"pyproject.toml",
		"package.json",
		"go.mod",
	}
	detected := []string{}
	for _, marker := range markers {
		if exists(filepath.Join(path, marker)) {
			detected = append(detected, marker)
		}
	}
	return detected
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func shouldSkipDir(name string) bool {
	skip := map[string]bool{
		".cache":            true,
		".git":              true,
		".hg":               true,
		".k-playbook-local": true,
		".svn":              true,
		".venv":             true,
		"dist":              true,
		"node_modules":      true,
		"results":           true,
		"target":            true,
		"vendor":            true,
		"venv":              true,
	}

	return skip[name]
}

func depth(root string, path string) int {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." {
		return 0
	}

	return len(strings.Split(rel, string(os.PathSeparator)))
}

func realPath(path string) string {
	real, err := filepath.EvalSymlinks(path)
	if err == nil {
		return filepath.Clean(real)
	}

	abs, err := filepath.Abs(path)
	if err == nil {
		return filepath.Clean(abs)
	}

	return filepath.Clean(path)
}

func samePath(left string, right string) bool {
	return filepath.Clean(left) == filepath.Clean(right)
}
