// Package install implements the project-local installation model: k-playbook
// lives in a subdirectory of the target project, the shipped payload under
// _dist/, and everything beside it belongs to the project.
package install

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	// ConfigFileName is the only config name commands have to look for.
	ConfigFileName = "K-PLAYBOOK.yaml"
	// PlaybookDirName is the conventional k-playbook directory inside a project.
	PlaybookDirName = "k-playbook"
	// DistDirName holds the shipped payload. Replaced wholesale on update.
	DistDirName = "_dist"

	SchemaVersion = 2
	LayoutName    = "project-local"

	// LegacySchemaVersion and LegacyLayoutName identify pre-migration projects.
	LegacySchemaVersion = 1
	LegacyLayoutName    = "fixed-project-k-playbook"
)

// PathKeys are the project-local artifact paths, with their conventional values
// relative to the k-playbook directory. Order is the order written to the file.
var PathKeys = []struct{ Key, Default string }{
	{"tasks", "tasks"},
	{"completed_tasks", "tasks/done"},
	{"todo", "TODO.md"},
	{"checks", "checks"},
	{"reviews", "reviews"},
	{"guidelines", "guidelines"},
	{"enforcement", "enforcement"},
	{"docs", "docs"},
	{"commands", "commands"},
}

// Config is the subset of K-PLAYBOOK.yaml the installer reasons about.
type Config struct {
	SchemaVersion int
	Layout        string
	Dist          string
	Version       string
	Paths         map[string]string
	RepoRoot      string
	VCS           string
	// Raw keeps the file verbatim so migration can rewrite it line by line
	// instead of round-tripping through a parser, which would drop comments,
	// key order and any field the installer does not know about.
	Raw string
}

// IsLegacy reports a project that still uses the central base installation.
func (c Config) IsLegacy() bool {
	return c.SchemaVersion == LegacySchemaVersion || c.Layout == LegacyLayoutName
}

var (
	yamlEntry = regexp.MustCompile(`^(\s*)([A-Za-z0-9_-]+):\s*(.*?)\s*$`)
	yamlItem  = regexp.MustCompile(`^(\s*)-\s+(.*?)\s*$`)
)

// parseFlat reads the config into dot-separated keys. It is deliberately not a
// full YAML parser: the file is installer-generated and uses plain nested
// mappings plus simple lists, and avoiding a YAML dependency keeps the binary
// free of one more thing that can disagree between versions.
func parseFlat(text string) (map[string]string, map[string][]string) {
	scalars := map[string]string{}
	lists := map[string][]string{}
	var stack []struct {
		indent int
		key    string
	}
	lastKey := ""

	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if match := yamlItem.FindStringSubmatch(line); match != nil && lastKey != "" {
			lists[lastKey] = append(lists[lastKey], strings.Trim(match[2], `"'`))
			continue
		}

		match := yamlEntry.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		indent, key, value := len(match[1]), match[2], strings.Trim(match[3], `"'`)

		for len(stack) > 0 && stack[len(stack)-1].indent >= indent {
			stack = stack[:len(stack)-1]
		}
		parts := make([]string, 0, len(stack)+1)
		for _, frame := range stack {
			parts = append(parts, frame.key)
		}
		full := strings.Join(append(parts, key), ".")

		if value != "" && !strings.HasPrefix(value, "#") {
			scalars[full] = value
			lastKey = ""
		} else {
			stack = append(stack, struct {
				indent int
				key    string
			}{indent, key})
			lastKey = full
		}
	}
	return scalars, lists
}

// ReadConfig loads K-PLAYBOOK.yaml from a k-playbook directory.
func ReadConfig(playbookDir string) (Config, error) {
	path := filepath.Join(playbookDir, ConfigFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("%s lesen: %w", path, err)
	}

	text := string(data)
	scalars, _ := parseFlat(text)

	config := Config{
		Layout:   scalars["layout"],
		Dist:     scalars["k_playbook.dist"],
		Version:  scalars["k_playbook.version"],
		RepoRoot: scalars["project.repo_root"],
		VCS:      scalars["project.vcs"],
		Paths:    map[string]string{},
		Raw:      text,
	}
	fmt.Sscanf(scalars["schema_version"], "%d", &config.SchemaVersion)
	for _, entry := range PathKeys {
		if value := scalars["paths."+entry.Key]; value != "" {
			config.Paths[entry.Key] = value
		}
	}
	return config, nil
}

// DistDir resolves the shipped installation directory for a k-playbook directory.
func (c Config) DistDir(playbookDir string) string {
	dist := c.Dist
	if dist == "" {
		dist = DistDirName
	}
	return filepath.Join(playbookDir, dist)
}

// ProjectRoot resolves project.repo_root against the k-playbook directory. This is
// the outer boundary for every configured path.
func (c Config) ProjectRoot(playbookDir string) string {
	if c.RepoRoot == "" {
		return filepath.Dir(playbookDir)
	}
	return filepath.Clean(filepath.Join(playbookDir, c.RepoRoot))
}

// ResolvePath resolves a paths.* key against the k-playbook directory.
func (c Config) ResolvePath(playbookDir string, key string) (string, bool) {
	value, ok := c.Paths[key]
	if !ok || value == "" {
		return "", false
	}
	return filepath.Clean(filepath.Join(playbookDir, value)), true
}

// ValidatePath checks a configured path against the two boundaries from
// docs/k-playbook-format.md.
//
// A path may leave the k-playbook directory with ../ — a project that already keeps
// its docs elsewhere should not have to move them. It may not leave the project, and
// it may never point into the installation, because an update replaces that directory
// wholesale and would take the project's files with it.
func (c Config) ValidatePath(playbookDir string, key string, value string) error {
	if value == "" {
		return fmt.Errorf("paths.%s ist leer", key)
	}
	if filepath.IsAbs(value) {
		return fmt.Errorf("paths.%s muss relativ sein: %s", key, value)
	}

	resolved := filepath.Clean(filepath.Join(playbookDir, value))
	projectRoot := c.ProjectRoot(playbookDir)
	if !within(projectRoot, resolved) {
		return fmt.Errorf("paths.%s fuehrt aus dem Projekt heraus: %s -> %s", key, value, resolved)
	}

	distDir := filepath.Clean(c.DistDir(playbookDir))
	if resolved == distDir || within(distDir, resolved) {
		return fmt.Errorf("paths.%s zeigt in die Installation (%s): %s", key, c.Dist, value)
	}
	return nil
}

// within reports whether path is root itself or below it.
func within(root string, path string) bool {
	if path == root {
		return true
	}
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

// RenderConfig produces a fresh schema_version 2 config.
func RenderConfig(version string, repoRoot string, vcs string, remediation string, now time.Time) string {
	if repoRoot == "" {
		repoRoot = ".."
	}
	if vcs == "" {
		vcs = "git"
	}
	if remediation == "" {
		remediation = "direct-allowed"
	}
	prRequired, directFixes := remediationPolicy(remediation)

	var builder strings.Builder
	fmt.Fprintf(&builder, "schema_version: %d\n", SchemaVersion)
	fmt.Fprintf(&builder, "layout: %s\n\n", LayoutName)
	fmt.Fprintf(&builder, "k_playbook:\n  dist: %s\n  version: %s\n  installed_at: %s\n\n",
		DistDirName, version, now.Format("2006-01-02"))

	builder.WriteString("paths:\n")
	for _, entry := range PathKeys {
		fmt.Fprintf(&builder, "  %s: %s\n", entry.Key, entry.Default)
	}

	fmt.Fprintf(&builder, "\nproject:\n  repo_root: %s\n  vcs: %s\n", repoRoot, vcs)
	builder.WriteString("\noverlay:\n  rules:\n    disabled: []\n  reviews:\n    disabled: []\n  checks:\n    disabled: []\n")
	fmt.Fprintf(&builder, "\nsetup:\n  updated_at: %s\n", now.Format("2006-01-02"))
	fmt.Fprintf(&builder, "\nremediation:\n  mode: %s\n  target: .\n  grouping: true\n  quick_wins: true\n  branch_prefix: remediation/\n  pr_required: %t\n  direct_fixes: %t\n",
		remediation, prRequired, directFixes)

	return builder.String()
}

func remediationPolicy(mode string) (prRequired bool, directFixes bool) {
	switch mode {
	case "task-branch-pr":
		return true, false
	case "task-first":
		return false, false
	default:
		return false, true
	}
}
