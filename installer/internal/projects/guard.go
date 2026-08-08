package projects

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/install"
)

// ErrProjectLocalLayout is returned when a legacy write path is asked to touch a
// project that has already moved to the project-local layout.
//
// The GUI and the older projects.* helpers still implement the central model:
// they write schema_version 1, expect K-PLAYBOOK.yaml in the project root and
// create directories under k-playbook/ that v2 does not use. Running them on a
// migrated project would write a second, contradictory config and silently undo
// the migration. Until the GUI is rebuilt, refusing is the safe behaviour.
var ErrProjectLocalLayout = fmt.Errorf("Projekt nutzt das projektlokale Layout (schema_version %d)", install.SchemaVersion)

// guardProjectLocal blocks legacy writes against an already-migrated project.
// Reads are unaffected.
func guardProjectLocal(projectPath string) error {
	normalized, err := NormalizePath(projectPath)
	if err != nil {
		return err
	}

	for _, candidate := range []string{
		filepath.Join(normalized, install.ConfigFileName),
		filepath.Join(normalized, install.PlaybookDirName, install.ConfigFileName),
	} {
		data, err := os.ReadFile(candidate)
		if err != nil {
			continue
		}
		if !isProjectLocalConfig(string(data)) {
			continue
		}
		return fmt.Errorf(
			"%w: %s. Diese Aktion gehoert zum alten zentralen Modell und wuerde die Migration rueckgaengig machen. "+
				"Nutze stattdessen `k-playbook-installer init|update|restore` auf der Kommandozeile",
			ErrProjectLocalLayout, candidate)
	}

	return nil
}

// isProjectLocalConfig detects a v2 config without a full parse, so this stays
// usable from the legacy code path.
func isProjectLocalConfig(text string) bool {
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "layout: "+install.LayoutName:
			return true
		case strings.HasPrefix(trimmed, "schema_version:"):
			value := strings.TrimSpace(strings.TrimPrefix(trimmed, "schema_version:"))
			if value != "" && value != "1" {
				return true
			}
		}
	}
	return false
}
