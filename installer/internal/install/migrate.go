package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kascada/k-playbook/installer/payload"
)

// MigrateResult reports what the v1 -> v2 migration did.
type MigrateResult struct {
	ProjectRoot string   `json:"projectRoot"`
	PlaybookDir string   `json:"playbookDir"`
	ConfigMoved bool     `json:"configMoved"`
	Changes     []string `json:"changes,omitempty"`
	Notes       []string `json:"notes,omitempty"`
}

// Migrate converts a project from the central-installation model to the
// project-local one, following the rule in docs/k-playbook-format.md.
//
// It is purely mechanical: the config moves into the k-playbook directory, path
// values lose their k-playbook/ prefix because they are now relative to that
// directory, and repo_root gains a level. Tasks, reviews, results and docs are
// already under k-playbook/ and are not moved.
//
// Fields the installer does not know about are preserved verbatim, which is why
// this rewrites line by line instead of re-serialising the file.
func Migrate(projectRoot string, dryRun bool) (MigrateResult, error) {
	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return MigrateResult{}, fmt.Errorf("Projektpfad aufloesen: %w", err)
	}

	playbookDir := filepath.Join(root, PlaybookDirName)
	result := MigrateResult{ProjectRoot: root, PlaybookDir: playbookDir}

	legacyConfig := filepath.Join(root, ConfigFileName)
	newConfig := filepath.Join(playbookDir, ConfigFileName)

	source := ""
	switch {
	case isFile(newConfig):
		source = newConfig
	case isFile(legacyConfig):
		source = legacyConfig
		result.ConfigMoved = true
	default:
		return result, fmt.Errorf("keine %s gefunden unter %s", ConfigFileName, root)
	}

	data, err := os.ReadFile(source)
	if err != nil {
		return result, fmt.Errorf("%s lesen: %w", source, err)
	}

	current, err := ReadConfig(filepath.Dir(source))
	if err != nil {
		return result, err
	}
	if !current.IsLegacy() {
		result.Notes = append(result.Notes, "Projekt ist bereits schema_version 2; keine Aenderung noetig")
		return result, nil
	}

	migrated, changes := migrateConfigText(string(data), payload.Version(), time.Now())
	result.Changes = changes

	if dryRun {
		result.Notes = append(result.Notes, "Probelauf: nichts geschrieben")
		return result, nil
	}

	if err := os.MkdirAll(playbookDir, 0o755); err != nil {
		return result, fmt.Errorf("k-playbook-Verzeichnis anlegen: %w", err)
	}
	if err := os.WriteFile(newConfig, []byte(migrated), 0o644); err != nil {
		return result, fmt.Errorf("%s schreiben: %w", newConfig, err)
	}
	if result.ConfigMoved {
		if err := os.Remove(legacyConfig); err != nil {
			return result, fmt.Errorf("alte %s entfernen: %w", ConfigFileName, err)
		}
	}

	if err := payload.Extract(filepath.Join(playbookDir, DistDirName)); err != nil {
		return result, err
	}
	result.Changes = append(result.Changes, "_dist installiert (Version "+payload.Version()+")")

	linked, notes, err := LinkAssistant(root, playbookDir)
	if err != nil {
		return result, err
	}
	result.Changes = append(result.Changes, linked...)
	result.Notes = append(result.Notes, notes...)

	if note, err := ensureGitignore(root); err != nil {
		return result, err
	} else if note != "" {
		result.Changes = append(result.Changes, note)
	}

	return result, nil
}

// migrateConfigText applies the v1 -> v2 transformation to the raw config.
func migrateConfigText(text string, version string, now time.Time) (string, []string) {
	var changes []string
	prefix := PlaybookDirName + "/"

	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines)+8)

	topLevel := ""
	skipBlock := ""
	pathsSeen := map[string]bool{}
	haveOverlay := strings.Contains(text, "\noverlay:")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		isTop := trimmed != "" && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") && strings.Contains(trimmed, ":")

		if isTop {
			topLevel = strings.TrimSuffix(strings.Fields(trimmed)[0], ":")
			skipBlock = ""

			switch topLevel {
			case "schema_version":
				out = append(out, fmt.Sprintf("schema_version: %d", SchemaVersion))
				changes = append(changes, fmt.Sprintf("schema_version %d -> %d", LegacySchemaVersion, SchemaVersion))
				continue
			case "layout":
				out = append(out, "layout: "+LayoutName)
				changes = append(changes, "layout "+LegacyLayoutName+" -> "+LayoutName)
				continue
			case "k_playbook":
				// Rewritten wholesale: repo is gone, dist/version/installed_at are new.
				out = append(out, "k_playbook:",
					"  dist: "+DistDirName,
					"  version: "+version,
					"  installed_at: "+now.Format("2006-01-02"),
					"")
				changes = append(changes, "k_playbook.repo entfernt, dist/version/installed_at gesetzt")
				skipBlock = "k_playbook"
				continue
			}
		}

		if skipBlock != "" {
			if !isTop {
				continue // swallow the old block body
			}
			skipBlock = ""
		}

		switch topLevel {
		case "paths":
			match := yamlEntry.FindStringSubmatch(line)
			if match == nil || match[2] == "paths" {
				out = append(out, line)
				continue
			}
			key, value := match[2], match[3]
			if key == "playbook" {
				changes = append(changes, "paths.playbook entfernt (das Verzeichnis der Config ist die Basis)")
				continue
			}
			stripped := strings.TrimPrefix(value, prefix)
			if stripped == "" {
				stripped = "."
			}
			if stripped != value {
				changes = append(changes, fmt.Sprintf("paths.%s: %s -> %s", key, value, stripped))
			}
			pathsSeen[key] = true
			out = append(out, "  "+key+": "+stripped)
			continue

		case "setup":
			match := yamlEntry.FindStringSubmatch(line)
			if match != nil && match[2] == "updated_at" {
				out = append(out, "  updated_at: "+now.Format("2006-01-02"))
				continue
			}

		case "project":
			match := yamlEntry.FindStringSubmatch(line)
			if match != nil && match[2] == "repo_root" {
				old := match[3]
				updated := "../" + strings.TrimPrefix(old, "./")
				if old == "." || old == "" {
					updated = ".."
				}
				changes = append(changes, fmt.Sprintf("project.repo_root: %s -> %s", old, updated))
				out = append(out, "  repo_root: "+updated)
				continue
			}
		}

		out = append(out, line)
	}

	// paths.commands is new in v2; append it so the key exists without asking.
	if !pathsSeen["commands"] {
		out = insertIntoBlock(out, "paths:", "  commands: commands")
		changes = append(changes, "paths.commands ergaenzt")
	}

	result := strings.Join(out, "\n")
	if !haveOverlay {
		if !strings.HasSuffix(result, "\n") {
			result += "\n"
		}
		result += "\noverlay:\n  rules:\n    disabled: []\n  reviews:\n    disabled: []\n  checks:\n    disabled: []\n"
		changes = append(changes, "overlay-Block ergaenzt")
	}

	return result, changes
}

// insertIntoBlock appends a line at the end of a top-level block.
func insertIntoBlock(lines []string, header string, addition string) []string {
	start := -1
	for index, line := range lines {
		if strings.TrimSpace(line) == header && !strings.HasPrefix(line, " ") {
			start = index
			break
		}
	}
	if start < 0 {
		return lines
	}

	end := start + 1
	for index := start + 1; index < len(lines); index++ {
		trimmed := strings.TrimSpace(lines[index])
		if trimmed == "" {
			continue
		}
		if !strings.HasPrefix(lines[index], " ") && !strings.HasPrefix(lines[index], "\t") {
			break
		}
		end = index + 1
	}

	result := make([]string, 0, len(lines)+1)
	result = append(result, lines[:end]...)
	result = append(result, addition)
	return append(result, lines[end:]...)
}
