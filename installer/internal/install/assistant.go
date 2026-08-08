package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// assistantLink describes one directory an assistant reads commands or skills from.
type assistantLink struct {
	// LinkPath is relative to the project root.
	LinkPath string
	// Source is relative to the k-playbook directory.
	Source string
}

func assistantLinks() []assistantLink {
	return []assistantLink{
		{LinkPath: filepath.Join(".claude", "commands"), Source: filepath.Join(DistDirName, "commands")},
		{LinkPath: filepath.Join(".claude", "skills"), Source: filepath.Join(DistDirName, "skills")},
	}
}

// LinkAssistant points the project's assistant directories at the installation.
//
// This replaces the old global registration under ~/.claude and ~/.config/opencode.
// Registering per project is what allows two projects to run different payload
// versions side by side without interfering.
//
// Preferred form is a single directory symlink. If a real directory already
// exists — the project has its own commands — individual file symlinks are used
// instead so nothing of the project's own is displaced.
func LinkAssistant(projectRoot string, playbookDir string) ([]string, []string, error) {
	var linked, notes []string

	for _, link := range assistantLinks() {
		source := filepath.Join(playbookDir, link.Source)
		if !isDir(source) {
			continue
		}
		target := filepath.Join(projectRoot, link.LinkPath)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return linked, notes, fmt.Errorf("%s anlegen: %w", filepath.Dir(link.LinkPath), err)
		}

		relative, err := filepath.Rel(filepath.Dir(target), source)
		if err != nil {
			relative = source
		}

		info, err := os.Lstat(target)
		switch {
		case err != nil && os.IsNotExist(err):
			if err := os.Symlink(relative, target); err != nil {
				return linked, notes, fmt.Errorf("%s verlinken: %w", link.LinkPath, err)
			}
			linked = append(linked, link.LinkPath+" -> "+relative)

		case err != nil:
			return linked, notes, fmt.Errorf("%s pruefen: %w", link.LinkPath, err)

		case info.Mode()&os.ModeSymlink != 0:
			// Repoint an existing symlink; it may be stale after a move.
			if err := os.Remove(target); err != nil {
				return linked, notes, fmt.Errorf("%s ersetzen: %w", link.LinkPath, err)
			}
			if err := os.Symlink(relative, target); err != nil {
				return linked, notes, fmt.Errorf("%s verlinken: %w", link.LinkPath, err)
			}
			linked = append(linked, link.LinkPath+" -> "+relative)

		case info.IsDir():
			count, err := linkFiles(source, target)
			if err != nil {
				return linked, notes, err
			}
			linked = append(linked, fmt.Sprintf("%s (%d Einzeldateien, eigenes Verzeichnis bleibt erhalten)", link.LinkPath, count))

		default:
			notes = append(notes, link.LinkPath+" existiert als Datei und wurde nicht angefasst")
		}
	}

	return linked, notes, nil
}

// linkFiles symlinks each payload entry into an existing real directory and
// removes only stale links it created earlier. Project-owned files stay.
func linkFiles(source string, target string) (int, error) {
	entries, err := os.ReadDir(source)
	if err != nil {
		return 0, fmt.Errorf("%s lesen: %w", source, err)
	}

	wanted := map[string]bool{}
	count := 0
	for _, entry := range entries {
		name := entry.Name()
		wanted[name] = true
		linkPath := filepath.Join(target, name)
		relative, err := filepath.Rel(target, filepath.Join(source, name))
		if err != nil {
			relative = filepath.Join(source, name)
		}

		if info, err := os.Lstat(linkPath); err == nil {
			if info.Mode()&os.ModeSymlink == 0 {
				// A real file with the same name is project-owned and wins.
				continue
			}
			if err := os.Remove(linkPath); err != nil {
				return count, fmt.Errorf("%s ersetzen: %w", linkPath, err)
			}
		}
		if err := os.Symlink(relative, linkPath); err != nil {
			return count, fmt.Errorf("%s verlinken: %w", linkPath, err)
		}
		count++
	}

	// Drop symlinks that point into the installation but no longer exist there.
	existing, err := os.ReadDir(target)
	if err != nil {
		return count, nil
	}
	for _, entry := range existing {
		if wanted[entry.Name()] {
			continue
		}
		linkPath := filepath.Join(target, entry.Name())
		info, err := os.Lstat(linkPath)
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			continue
		}
		destination, err := os.Readlink(linkPath)
		if err != nil {
			continue
		}
		if strings.Contains(destination, DistDirName) {
			os.Remove(linkPath)
		}
	}

	return count, nil
}
