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

// checkProjectLocalStructure validates a schema_version 2 project against its own
// paths.* instead of the hardcoded v1 layout. Reports ok=false when the project is
// not project-local, so the caller falls back to the legacy check.
func checkProjectLocalStructure(projectRoot string) (StructureStatus, bool, error) {
	playbookDir := filepath.Join(projectRoot, install.PlaybookDirName)
	if _, err := os.Stat(filepath.Join(playbookDir, install.ConfigFileName)); err != nil {
		return StructureStatus{}, false, nil
	}

	config, err := install.ReadConfig(playbookDir)
	if err != nil || config.IsLegacy() {
		return StructureStatus{}, false, nil
	}

	missing := []string{}
	if _, err := os.Stat(config.DistDir(playbookDir)); os.IsNotExist(err) {
		missing = append(missing, config.Dist+"/ (fehlt — `k-playbook-installer restore`)")
	}

	for _, entry := range install.PathKeys {
		value, ok := config.Paths[entry.Key]
		if !ok || value == "" {
			missing = append(missing, "paths."+entry.Key+" (nicht konfiguriert)")
			continue
		}
		if err := config.ValidatePath(playbookDir, entry.Key, value); err != nil {
			missing = append(missing, err.Error())
			continue
		}

		resolved, _ := config.ResolvePath(playbookDir, entry.Key)
		info, err := os.Stat(resolved)
		if os.IsNotExist(err) {
			missing = append(missing, value+" (fehlt)")
			continue
		}
		if err != nil {
			return StructureStatus{}, false, fmt.Errorf("%s pruefen: %w", value, err)
		}
		// todo is the only file-valued key; everything else must be a directory.
		if entry.Key == "todo" && info.IsDir() {
			missing = append(missing, value+" (ist ein Verzeichnis, erwartet: Datei)")
		} else if entry.Key != "todo" && !info.IsDir() {
			missing = append(missing, value+" (ist kein Verzeichnis)")
		}
	}

	status := StructureStatus{OK: len(missing) == 0, Missing: missing}
	if status.OK {
		status.Message = "Projektstruktur vollstaendig (project-local)."
	} else {
		status.Message = fmt.Sprintf("Projektstruktur unvollstaendig: %d Pfade fehlen oder sind falsch.", len(missing))
	}
	return status, true, nil
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
