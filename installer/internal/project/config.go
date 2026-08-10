package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config sind die Werte aus der K-PLAYBOOK.yaml, soweit sie derzeit gebraucht
// werden.
type Config struct {
	SchemaVersion string `json:"schemaVersion"`
	RepoRoot      string `json:"repoRoot"`
	VCS           string `json:"vcs"`
}

// ReadConfig liest die Konfiguration eines Projekts.
//
// Bewusst kein YAML-Parser: gelesen werden nur wenige Skalare, und der
// zeilenweise Zugriff laesst Kommentare, Reihenfolge und unbekannte Bloecke
// unangetastet, wenn spaeter zurueckgeschrieben wird.
func ReadConfig(projectDir string) (Config, error) {
	data, err := os.ReadFile(ConfigPath(projectDir))
	if err != nil {
		return Config{}, err
	}

	config := Config{}
	section := ""
	for _, line := range strings.Split(string(data), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		key, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		indented := strings.HasPrefix(key, " ") || strings.HasPrefix(key, "\t")
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		if !indented {
			section = key
			if value == "" {
				continue
			}
		}

		switch {
		case !indented && key == "schema_version":
			config.SchemaVersion = value
		case indented && section == "project" && key == "repo_root":
			config.RepoRoot = value
		case indented && section == "project" && key == "vcs":
			config.VCS = value
		}
	}
	return config, nil
}

// RepoRootDir loest project.repo_root gegen das Hauptverzeichnis auf.
func RepoRootDir(projectDir string, config Config) string {
	repoRoot := config.RepoRoot
	if repoRoot == "" {
		repoRoot = "."
	}
	return filepath.Clean(filepath.Join(projectDir, repoRoot))
}

// Suggestion ist der Vorschlag fuer eine noch nicht angelegte Konfiguration.
type Suggestion struct {
	// ProjectDir ist das vorgeschlagene Hauptverzeichnis.
	ProjectDir string `json:"projectDir"`
	// Derived meldet, ob ProjectDir aus dem Ort des Binaries abgeleitet werden
	// konnte. Ist es false, wurde auf das Arbeitsverzeichnis zurueckgegriffen.
	Derived bool `json:"derived"`
	// RepoRoot ist der Vorschlag fuer project.repo_root, relativ zu ProjectDir.
	// Leer, wenn kein Repository gefunden wurde oder mehrere in Frage kommen.
	RepoRoot string `json:"repoRoot"`
	// RepoCandidates sind alle gefundenen Repositories unterhalb von ProjectDir.
	RepoCandidates []string `json:"repoCandidates"`
}

// Suggest leitet einen Vorschlag ab, ohne etwas zu schreiben.
//
// Das Hauptverzeichnis muss nicht geraten werden: der Aufruf erfolgt aus
// <A>/k-playbook/bin/, und das Binary liegt in <A>/k-playbook/dist/. Zwei Ebenen
// darueber liegt A.
func Suggest() Suggestion {
	suggestion := Suggestion{}

	if dir, ok := projectDirFromExecutable(); ok {
		suggestion.ProjectDir = dir
		suggestion.Derived = true
	} else if wd, err := os.Getwd(); err == nil {
		suggestion.ProjectDir = wd
	}

	if suggestion.ProjectDir != "" {
		suggestion.RepoRoot, suggestion.RepoCandidates = suggestRepoRoot(suggestion.ProjectDir)
	}
	return suggestion
}

// projectDirFromExecutable leitet A aus <A>/k-playbook/dist/<binary> ab. Der
// Name des Zwischenverzeichnisses wird geprueft, damit ein anderswohin
// kopiertes Binary nicht auf ein beliebiges Verzeichnis zeigt.
func projectDirFromExecutable() (string, bool) {
	exe, err := os.Executable()
	if err != nil {
		return "", false
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}

	playbookDir := filepath.Dir(filepath.Dir(exe))
	if filepath.Base(playbookDir) != PlaybookDirName {
		return "", false
	}
	return filepath.Dir(playbookDir), true
}

// suggestRepoRoot sucht das Projekt-Repository. Liegt es im Hauptverzeichnis
// selbst, ist der Wert "."; sonst kommen die Unterverzeichnisse in Frage, etwa
// wenn das Repo parallel zur Installation ausgecheckt ist.
func suggestRepoRoot(projectDir string) (string, []string) {
	if pathExists(filepath.Join(projectDir, ".git")) {
		return ".", []string{"."}
	}

	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return "", nil
	}

	var candidates []string
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() || name == PlaybookDirName || strings.HasPrefix(name, ".") {
			continue
		}
		if pathExists(filepath.Join(projectDir, name, ".git")) {
			candidates = append(candidates, name)
		}
	}

	// Nur bei genau einem Fund ist die Zuordnung eindeutig.
	if len(candidates) == 1 {
		return candidates[0], candidates
	}
	return "", candidates
}

// CreateConfig legt die K-PLAYBOOK.yaml im Hauptverzeichnis an.
//
// Eine vorhandene Datei wird nie ueberschrieben: sie gehoert dem Projekt und
// kann Werte enthalten, die hier nicht bekannt sind.
func CreateConfig(projectDir string, repoRoot string) error {
	if strings.TrimSpace(projectDir) == "" {
		return fmt.Errorf("kein Hauptverzeichnis angegeben")
	}
	if !isDir(projectDir) {
		return fmt.Errorf("%s ist kein Verzeichnis", projectDir)
	}

	path := ConfigPath(projectDir)
	if pathExists(path) {
		return fmt.Errorf("%s existiert bereits", ConfigFileName)
	}

	repoRoot = strings.TrimSpace(repoRoot)
	if repoRoot == "" {
		repoRoot = "."
	}
	vcs := "none"
	if pathExists(filepath.Join(projectDir, repoRoot, ".git")) {
		vcs = "git"
	}

	return os.WriteFile(path, []byte(renderConfig(repoRoot, vcs)), 0o644)
}

func renderConfig(repoRoot string, vcs string) string {
	var out strings.Builder
	out.WriteString("# k-playbook\n")
	out.WriteString("#\n")
	out.WriteString("# Der Ort dieser Datei bestimmt das Hauptverzeichnis des Projekts.\n")
	out.WriteString("# Die Installation liegt daneben unter " + PlaybookDirName + "/ und ist\n")
	out.WriteString("# vollstaendig ersetzbar; projekteigene Dateien gehoeren nicht hinein.\n")
	out.WriteString("\n")
	out.WriteString("schema_version: 2\n")
	out.WriteString("\n")
	out.WriteString("project:\n")
	out.WriteString("  # Ort des Projekt-Repositorys, relativ zu dieser Datei.\n")
	out.WriteString("  repo_root: " + repoRoot + "\n")
	out.WriteString("  vcs: " + vcs + "\n")
	return out.String()
}

func pathExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}
