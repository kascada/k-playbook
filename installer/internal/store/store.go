package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/kascada/k-playbook/installer/internal/pathcontract"
)

const LocalDirName = ".k-playbook-local"

type ProjectEnvironment string

const (
	EnvironmentUnknown      ProjectEnvironment = "unknown"
	EnvironmentPlain        ProjectEnvironment = "plain"
	EnvironmentVenv         ProjectEnvironment = "venv"
	EnvironmentDevContainer ProjectEnvironment = "devcontainer"
)

type Project struct {
	Path        string             `json:"path"`
	Name        string             `json:"name"`
	Environment ProjectEnvironment `json:"environment"`
	Selected    bool               `json:"selected"`
	Detected    []string           `json:"detected,omitempty"`
	AddedAt     string             `json:"addedAt"`
	UpdatedAt   string             `json:"updatedAt"`
}

type ProjectsFile struct {
	Version  int       `json:"version"`
	Projects []Project `json:"projects"`
}

func LocalDir() (string, error) {
	result, err := pathcontract.Check()
	if err != nil {
		return "", err
	}
	if !result.OK {
		return "", fmt.Errorf("Pfadvertrag nicht erfuellt: %s", result.Code)
	}

	return filepath.Join(result.Expected, LocalDirName), nil
}

func ProjectsPath() (string, error) {
	dir, err := LocalDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "projects.json"), nil
}

func LoadProjects() (ProjectsFile, error) {
	path, err := ProjectsPath()
	if err != nil {
		return ProjectsFile{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ProjectsFile{Version: 1, Projects: []Project{}}, nil
		}
		return ProjectsFile{}, fmt.Errorf("Projekt-Auswahl lesen: %w", err)
	}

	var file ProjectsFile
	if err := json.Unmarshal(data, &file); err != nil {
		return ProjectsFile{}, fmt.Errorf("Projekt-Auswahl parsen: %w", err)
	}
	if file.Version == 0 {
		file.Version = 1
	}
	if file.Projects == nil {
		file.Projects = []Project{}
	}

	return file, nil
}

func SaveProjects(file ProjectsFile) error {
	path, err := ProjectsPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("lokalen Store anlegen: %w", err)
	}

	file.Version = 1
	sort.Slice(file.Projects, func(i int, j int) bool {
		return file.Projects[i].Path < file.Projects[j].Path
	})

	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("Projekt-Auswahl serialisieren: %w", err)
	}

	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("Projekt-Auswahl schreiben: %w", err)
	}

	return nil
}

func UpsertProject(file ProjectsFile, project Project) ProjectsFile {
	now := time.Now().UTC().Format(time.RFC3339)
	project.UpdatedAt = now
	if project.AddedAt == "" {
		project.AddedAt = now
	}

	for index, existing := range file.Projects {
		if existing.Path == project.Path {
			if project.AddedAt == now && existing.AddedAt != "" {
				project.AddedAt = existing.AddedAt
			}
			file.Projects[index] = project
			return file
		}
	}

	file.Projects = append(file.Projects, project)
	return file
}

func RemoveProject(file ProjectsFile, path string) (ProjectsFile, bool) {
	projects := file.Projects[:0]
	removed := false
	for _, project := range file.Projects {
		if project.Path == path {
			removed = true
			continue
		}
		projects = append(projects, project)
	}
	file.Projects = projects
	return file, removed
}
