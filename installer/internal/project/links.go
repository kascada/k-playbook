package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Link beschreibt ein Verzeichnis, aus dem ein Assistent Commands oder Skills
// liest. Beide Pfade sind relativ zur Projektwurzel.
type Link struct {
	Path   string `json:"path"`
	Source string `json:"source"`
}

// Links sind die Verzeichnisse, die ein Zielprojekt braucht.
func Links() []Link {
	return []Link{
		{Path: filepath.Join(".claude", "commands"), Source: filepath.Join(PlaybookDirName, "commands")},
		{Path: filepath.Join(".claude", "skills"), Source: filepath.Join(PlaybookDirName, "skills")},
	}
}

// LinkState ist der Zustand einer einzelnen Verlinkung.
type LinkState string

const (
	// StateOK: Symlink vorhanden und zeigt auf die Installation.
	StateOK LinkState = "ok"
	// StateMissing: nichts vorhanden.
	StateMissing LinkState = "missing"
	// StateStale: Symlink vorhanden, zeigt aber woandershin.
	StateStale LinkState = "stale"
	// StateOwnDirectory: echtes Verzeichnis, das dem Projekt gehoert. Wird per
	// Einzeldatei-Symlinks bestueckt, damit projekteigene Dateien bleiben.
	StateOwnDirectory LinkState = "own-directory"
	// StateBlocked: eine Datei steht im Weg. Wird nicht angefasst.
	StateBlocked LinkState = "blocked"
	// StateNoSource: die Installation hat das Quellverzeichnis nicht.
	StateNoSource LinkState = "no-source"
)

// LinkStatus ist der gepruefte Zustand einer Verlinkung.
type LinkStatus struct {
	Link
	State  LinkState `json:"state"`
	Detail string    `json:"detail"`
}

// OK meldet, ob dieser Eintrag nichts mehr braucht.
func (s LinkStatus) OK() bool { return s.State == StateOK }

// CheckLinks prueft den Zustand, ohne etwas zu veraendern.
func CheckLinks(projectRoot string) []LinkStatus {
	statuses := make([]LinkStatus, 0, len(Links()))
	for _, link := range Links() {
		statuses = append(statuses, checkLink(projectRoot, link))
	}
	return statuses
}

// LinksOK meldet, ob alle Eintraege eingerichtet sind.
func LinksOK(statuses []LinkStatus) bool {
	for _, status := range statuses {
		if !status.OK() {
			return false
		}
	}
	return len(statuses) > 0
}

func checkLink(projectRoot string, link Link) LinkStatus {
	status := LinkStatus{Link: link}

	source := filepath.Join(projectRoot, link.Source)
	if !isDir(source) {
		status.State = StateNoSource
		status.Detail = link.Source + " fehlt in der Installation"
		return status
	}

	target := filepath.Join(projectRoot, link.Path)
	wanted := relativeSource(target, source)

	info, err := os.Lstat(target)
	switch {
	case err != nil && os.IsNotExist(err):
		status.State = StateMissing
		status.Detail = "nicht vorhanden"

	case err != nil:
		status.State = StateBlocked
		status.Detail = err.Error()

	case info.Mode()&os.ModeSymlink != 0:
		destination, err := os.Readlink(target)
		if err != nil {
			status.State = StateStale
			status.Detail = "Ziel nicht lesbar"
		} else if destination == wanted {
			status.State = StateOK
			status.Detail = "-> " + destination
		} else {
			status.State = StateStale
			status.Detail = "zeigt auf " + destination
		}

	case info.IsDir():
		missing := missingFileLinks(source, target)
		if missing == 0 {
			status.State = StateOK
			status.Detail = "eigenes Verzeichnis, Einzeldateien verlinkt"
		} else {
			status.State = StateOwnDirectory
			status.Detail = fmt.Sprintf("eigenes Verzeichnis, %d Eintraege fehlen", missing)
		}

	default:
		status.State = StateBlocked
		status.Detail = "Datei steht im Weg"
	}

	return status
}

// ApplyLinks richtet die Verlinkung ein und meldet den Zustand danach.
//
// Bevorzugt wird ein einzelner Verzeichnis-Symlink. Hat das Projekt an der
// Stelle ein echtes Verzeichnis, bekommt es stattdessen Einzeldatei-Symlinks,
// damit projekteigene Commands nicht verdraengt werden. Eine Datei im Weg
// bleibt unangetastet.
func ApplyLinks(projectRoot string) ([]LinkStatus, error) {
	for _, link := range Links() {
		source := filepath.Join(projectRoot, link.Source)
		if !isDir(source) {
			continue
		}

		target := filepath.Join(projectRoot, link.Path)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return CheckLinks(projectRoot), fmt.Errorf("%s anlegen: %w", filepath.Dir(link.Path), err)
		}

		info, err := os.Lstat(target)
		switch {
		case err != nil && os.IsNotExist(err):
			if err := os.Symlink(relativeSource(target, source), target); err != nil {
				return CheckLinks(projectRoot), fmt.Errorf("%s verlinken: %w", link.Path, err)
			}

		case err != nil:
			return CheckLinks(projectRoot), fmt.Errorf("%s pruefen: %w", link.Path, err)

		case info.Mode()&os.ModeSymlink != 0:
			// Neu setzen: ein bestehender Link kann nach einem Umzug ins Leere zeigen.
			if err := os.Remove(target); err != nil {
				return CheckLinks(projectRoot), fmt.Errorf("%s ersetzen: %w", link.Path, err)
			}
			if err := os.Symlink(relativeSource(target, source), target); err != nil {
				return CheckLinks(projectRoot), fmt.Errorf("%s verlinken: %w", link.Path, err)
			}

		case info.IsDir():
			if err := linkFiles(source, target); err != nil {
				return CheckLinks(projectRoot), err
			}

		default:
			// Datei im Weg: gehoert dem Projekt, bleibt liegen.
		}
	}

	return CheckLinks(projectRoot), nil
}

// linkFiles verlinkt jeden Eintrag der Installation einzeln in ein bestehendes
// echtes Verzeichnis. Projekteigene Dateien gewinnen und bleiben unberuehrt.
func linkFiles(source string, target string) error {
	entries, err := os.ReadDir(source)
	if err != nil {
		return fmt.Errorf("%s lesen: %w", source, err)
	}

	wanted := map[string]bool{}
	for _, entry := range entries {
		name := entry.Name()
		wanted[name] = true

		linkPath := filepath.Join(target, name)
		if info, err := os.Lstat(linkPath); err == nil {
			if info.Mode()&os.ModeSymlink == 0 {
				continue
			}
			if err := os.Remove(linkPath); err != nil {
				return fmt.Errorf("%s ersetzen: %w", linkPath, err)
			}
		}
		if err := os.Symlink(relativeSource(target, filepath.Join(source, name)), linkPath); err != nil {
			return fmt.Errorf("%s verlinken: %w", linkPath, err)
		}
	}

	removeStaleLinks(target, wanted)
	return nil
}

// missingFileLinks zaehlt die Eintraege der Installation, die im eigenen
// Verzeichnis des Projekts weder als Datei noch als Symlink vorliegen.
func missingFileLinks(source string, target string) int {
	entries, err := os.ReadDir(source)
	if err != nil {
		return 0
	}

	missing := 0
	for _, entry := range entries {
		if _, err := os.Lstat(filepath.Join(target, entry.Name())); err != nil {
			missing++
		}
	}
	return missing
}

// removeStaleLinks entfernt Symlinks, die in die Installation zeigen, dort aber
// nicht mehr existieren. Alles andere bleibt liegen.
func removeStaleLinks(target string, wanted map[string]bool) {
	existing, err := os.ReadDir(target)
	if err != nil {
		return
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
		if strings.Contains(destination, PlaybookDirName) {
			os.Remove(linkPath)
		}
	}
}

// relativeSource bildet den Symlink-Wert relativ zum Verzeichnis des Links,
// damit das Projekt als Ganzes verschiebbar bleibt.
func relativeSource(target string, source string) string {
	relative, err := filepath.Rel(filepath.Dir(target), source)
	if err != nil {
		return source
	}
	return relative
}
