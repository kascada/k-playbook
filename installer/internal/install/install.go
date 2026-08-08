package install

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kascada/k-playbook/installer/payload"
)

// ErrNotFound means no K-PLAYBOOK.yaml was reachable from the start directory.
var ErrNotFound = errors.New("kein k-playbook-Projekt gefunden")

// Discover locates the k-playbook directory by walking upward, mirroring
// commands/_shared/path-resolution.md. It never consults $HOME/dev/k-playbook or
// any other fixed host path — that was the old model.
func Discover(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", fmt.Errorf("Startverzeichnis aufloesen: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}

	home, _ := os.UserHomeDir()
	for {
		if isFile(filepath.Join(dir, ConfigFileName)) {
			return dir, nil
		}
		if isFile(filepath.Join(dir, PlaybookDirName, ConfigFileName)) {
			return filepath.Join(dir, PlaybookDirName), nil
		}

		// Stop at the worktree root, at $HOME and at /. Checking .git after the
		// lookups above means a playbook directly at the worktree root is still
		// found.
		if isDir(filepath.Join(dir, ".git")) || isFile(filepath.Join(dir, ".git")) {
			return "", fmt.Errorf("%w unter %s", ErrNotFound, startDir)
		}
		parent := filepath.Dir(dir)
		if parent == dir || dir == home {
			return "", fmt.Errorf("%w unter %s", ErrNotFound, startDir)
		}
		dir = parent
	}
}

// Options configures a fresh installation.
type Options struct {
	RepoRoot    string // relative to the playbook dir, normally ".."
	VCS         string // "git" or "none"
	Remediation string
	Now         time.Time
}

// Result reports what an operation changed, so callers can print an honest
// summary instead of claiming more than happened.
type Result struct {
	PlaybookDir   string   `json:"playbookDir"`
	DistDir       string   `json:"distDir"`
	Version       string   `json:"version"`
	ConfigWritten bool     `json:"configWritten"`
	Created       []string `json:"created,omitempty"`
	Linked        []string `json:"linked,omitempty"`
	Notes         []string `json:"notes,omitempty"`
}

// Init installs k-playbook into projectRoot.
//
// It is safe to re-run: an existing K-PLAYBOOK.yaml is never overwritten, and
// existing artifact directories are left as they are. Only _dist is replaced.
func Init(projectRoot string, options Options) (Result, error) {
	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return Result{}, fmt.Errorf("Projektpfad aufloesen: %w", err)
	}
	if !isDir(root) {
		return Result{}, fmt.Errorf("Projektverzeichnis nicht gefunden: %s", root)
	}
	if options.Now.IsZero() {
		options.Now = time.Now()
	}
	if options.VCS == "" {
		if isDir(filepath.Join(root, ".git")) || isFile(filepath.Join(root, ".git")) {
			options.VCS = "git"
		} else {
			options.VCS = "none"
		}
	}

	playbookDir := filepath.Join(root, PlaybookDirName)
	result := Result{PlaybookDir: playbookDir, Version: payload.Version()}

	if err := os.MkdirAll(playbookDir, 0o755); err != nil {
		return result, fmt.Errorf("k-playbook-Verzeichnis anlegen: %w", err)
	}

	distDir := filepath.Join(playbookDir, DistDirName)
	result.DistDir = distDir
	if err := payload.Extract(distDir); err != nil {
		return result, err
	}

	configPath := filepath.Join(playbookDir, ConfigFileName)
	if !isFile(configPath) {
		content := RenderConfig(payload.Version(), options.RepoRoot, options.VCS, options.Remediation, options.Now)
		if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
			return result, fmt.Errorf("%s schreiben: %w", ConfigFileName, err)
		}
		result.ConfigWritten = true
	} else {
		result.Notes = append(result.Notes, "K-PLAYBOOK.yaml war vorhanden und wurde nicht ueberschrieben")
		if err := setVersion(configPath, payload.Version(), options.Now); err != nil {
			return result, err
		}
	}

	created, err := ensureStructure(playbookDir)
	if err != nil {
		return result, err
	}
	result.Created = created

	linked, notes, err := LinkAssistant(root, playbookDir)
	if err != nil {
		return result, err
	}
	result.Linked = linked
	result.Notes = append(result.Notes, notes...)

	if note, err := ensureGitignore(root); err != nil {
		return result, err
	} else if note != "" {
		result.Notes = append(result.Notes, note)
	}

	return result, nil
}

// Update replaces _dist with the payload carried by this binary. Nothing outside
// _dist is touched, which is what makes updates safe to run on a dirty project.
func Update(playbookDir string) (Result, error) {
	config, err := ReadConfig(playbookDir)
	if err != nil {
		return Result{}, err
	}
	if config.IsLegacy() {
		return Result{}, fmt.Errorf("Projekt nutzt schema_version %d; zuerst `k-playbook-installer migrate` ausfuehren", config.SchemaVersion)
	}

	distDir := config.DistDir(playbookDir)
	result := Result{PlaybookDir: playbookDir, DistDir: distDir, Version: payload.Version()}
	if err := payload.Extract(distDir); err != nil {
		return result, err
	}
	if err := setVersion(filepath.Join(playbookDir, ConfigFileName), payload.Version(), time.Now()); err != nil {
		return result, err
	}

	if config.Version != "" && config.Version != payload.Version() {
		result.Notes = append(result.Notes, fmt.Sprintf("Version %s -> %s", config.Version, payload.Version()))
	}
	return result, nil
}

// Restore rebuilds _dist after a git clone, where it is absent because it is
// gitignored. It reports a version mismatch instead of silently installing a
// different payload than the project recorded.
func Restore(playbookDir string) (Result, error) {
	config, err := ReadConfig(playbookDir)
	if err != nil {
		return Result{}, err
	}
	if config.IsLegacy() {
		return Result{}, fmt.Errorf("Projekt nutzt schema_version %d; zuerst `k-playbook-installer migrate` ausfuehren", config.SchemaVersion)
	}

	distDir := config.DistDir(playbookDir)
	result := Result{PlaybookDir: playbookDir, DistDir: distDir, Version: payload.Version()}

	if config.Version != "" && config.Version != payload.Version() {
		result.Notes = append(result.Notes, fmt.Sprintf(
			"Projekt erwartet Version %s, dieses Binary liefert %s; es wird %s installiert",
			config.Version, payload.Version(), payload.Version()))
	}
	if err := payload.Extract(distDir); err != nil {
		return result, err
	}
	if err := setVersion(filepath.Join(playbookDir, ConfigFileName), payload.Version(), time.Now()); err != nil {
		return result, err
	}

	projectRoot := filepath.Dir(playbookDir)
	if config.RepoRoot != "" {
		projectRoot = filepath.Clean(filepath.Join(playbookDir, config.RepoRoot))
	}
	linked, notes, err := LinkAssistant(projectRoot, playbookDir)
	if err != nil {
		return result, err
	}
	result.Linked = linked
	result.Notes = append(result.Notes, notes...)

	return result, nil
}

// ensureStructure creates the configured artifact directories and seed files.
// Paths come from the config; nothing is guessed.
func ensureStructure(playbookDir string) ([]string, error) {
	config, err := ReadConfig(playbookDir)
	if err != nil {
		return nil, err
	}

	var created []string
	for _, entry := range PathKeys {
		value, ok := config.Paths[entry.Key]
		if !ok || value == "" {
			continue
		}
		target := filepath.Join(playbookDir, value)
		if entry.Key == "todo" {
			if !isFile(target) {
				if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
					return created, err
				}
				if err := os.WriteFile(target, []byte(todoSeed), 0o644); err != nil {
					return created, fmt.Errorf("%s anlegen: %w", value, err)
				}
				created = append(created, value)
			}
			continue
		}
		if !isDir(target) {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return created, fmt.Errorf("%s anlegen: %w", value, err)
			}
			created = append(created, value+"/")
		}
	}

	if reviews, ok := config.ResolvePath(playbookDir, "reviews"); ok {
		decisions := filepath.Join(reviews, "known-decisions.md")
		if !isFile(decisions) {
			if err := os.WriteFile(decisions, []byte(knownDecisionsSeed), 0o644); err != nil {
				return created, fmt.Errorf("known-decisions.md anlegen: %w", err)
			}
			created = append(created, filepath.Join(config.Paths["reviews"], "known-decisions.md"))
		}
	}

	return created, nil
}

// setVersion updates k_playbook.version and installed_at in place, leaving every
// other line untouched.
func setVersion(configPath string, version string, now time.Time) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("%s lesen: %w", configPath, err)
	}

	lines := strings.Split(string(data), "\n")
	inBlock := false
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") && trimmed != "" {
			inBlock = trimmed == "k_playbook:"
			continue
		}
		if !inBlock {
			continue
		}
		switch {
		case strings.HasPrefix(trimmed, "version:"):
			lines[index] = "  version: " + version
		case strings.HasPrefix(trimmed, "installed_at:"):
			lines[index] = "  installed_at: " + now.Format("2006-01-02")
		}
	}

	return os.WriteFile(configPath, []byte(strings.Join(lines, "\n")), 0o644)
}

// ensureGitignore adds the _dist ignore rule to the project's .gitignore. The
// payload is reproducible from the binary, so checking it in would only add
// noise to every update.
func ensureGitignore(projectRoot string) (string, error) {
	rule := PlaybookDirName + "/" + DistDirName + "/"
	path := filepath.Join(projectRoot, ".gitignore")

	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf(".gitignore lesen: %w", err)
	}
	existing := string(data)
	for _, line := range strings.Split(existing, "\n") {
		if strings.TrimSpace(line) == rule {
			return "", nil
		}
	}

	addition := rule + "\n"
	if existing != "" && !strings.HasSuffix(existing, "\n") {
		addition = "\n" + addition
	}
	if existing != "" {
		addition = "\n# k-playbook: mitgelieferte Installation, per Installer wiederherstellbar\n" + addition
	}
	handle, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return "", fmt.Errorf(".gitignore schreiben: %w", err)
	}
	defer handle.Close()
	if _, err := handle.WriteString(addition); err != nil {
		return "", fmt.Errorf(".gitignore schreiben: %w", err)
	}
	return ".gitignore um " + rule + " ergaenzt", nil
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

const todoSeed = `# TODO

Projektlokale Todos. Wird von ` + "`/k-todo`" + ` gepflegt.
`

const knownDecisionsSeed = `# Known Decisions

Eintraege in dieser Datei dokumentieren bewusste Design-Entscheidungen und bekannte Trade-offs.
Reviews melden Findings nicht erneut, wenn sie hier gedeckt sind.
`
