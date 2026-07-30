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

func EnsureConfig(projectPath string) (bool, error) {
	normalized, err := NormalizePath(projectPath)
	if err != nil {
		return false, err
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

	if _, err := file.WriteString(MinimalConfig(time.Now())); err != nil {
		return false, fmt.Errorf("%s schreiben: %w", ConfigFileName, err)
	}

	return true, nil
}

func MinimalConfig(updatedAt time.Time) string {
	return fmt.Sprintf(`schema_version: 1
layout: fixed-project-k-playbook

k_playbook:
  repo: ~/dev/k-playbook

setup:
  updated_at: %s
`, updatedAt.Format("2006-01-02"))
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
