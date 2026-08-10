package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Link beschreibt einen Symlink, den ein Assistent zum Lesen braucht. Beide
// Pfade sind relativ zur Projektwurzel.
type Link struct {
	Path   string `json:"path"`
	Source string `json:"source"`
	// Assistant nennt, wer hier liest — nur fuer die Anzeige.
	Assistant string `json:"assistant"`
	// IsFile unterscheidet Datei- von Verzeichnis-Links. Bei Verzeichnissen
	// darf ein projekteigenes Verzeichnis bestehen bleiben und wird per
	// Einzeldatei-Links bestueckt; bei Dateien gibt es diesen Fall nicht.
	IsFile bool `json:"isFile"`
	// Optional markiert Links, deren Quelle dem Projekt gehoert. Fehlt sie,
	// ist nichts zu tun — im Gegensatz zu einer fehlenden Quelle in der
	// Installation, die auf eine beschaedigte Installation hindeutet.
	Optional bool `json:"optional"`
}

// Links sind die Symlinks, die ein Zielprojekt braucht.
//
// Skills stehen nur einmal: OpenCode durchsucht neben .opencode/skills/ auch
// .claude/skills/, ein zweiter Link waere Dopplung. Cursor kennt kein
// Skill-Konzept.
//
// CLAUDE.md zeigt auf AGENTS.md, weil Claude Code ausschliesslich CLAUDE.md
// liest und OpenCode AGENTS.md bevorzugt. Ein Symlink statt eines Imports,
// damit eine Aenderung immer in beiden ankommt — wer in CLAUDE.md schreibt,
// schreibt durch den Link hindurch in AGENTS.md.
func Links() []Link {
	commands := filepath.Join(PlaybookDirName, "commands")
	skills := filepath.Join(PlaybookDirName, "skills")

	return []Link{
		{Path: filepath.Join(".claude", "commands"), Source: commands, Assistant: "Claude Code"},
		{Path: filepath.Join(".claude", "skills"), Source: skills, Assistant: "Claude Code, OpenCode"},
		{Path: filepath.Join(".opencode", "commands"), Source: commands, Assistant: "OpenCode"},
		{Path: filepath.Join(".cursor", "commands"), Source: commands, Assistant: "Cursor"},
		{Path: "CLAUDE.md", Source: "AGENTS.md", Assistant: "Claude Code", IsFile: true, Optional: true},
	}
}

// sourcePresent meldet, ob die Quelle in der erwarteten Form vorliegt.
func (l Link) sourcePresent(projectRoot string) bool {
	source := filepath.Join(projectRoot, l.Source)
	if l.IsFile {
		return fileExists(source)
	}
	return isDir(source)
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

// OK meldet, ob der Symlink steht.
func (s LinkStatus) OK() bool { return s.State == StateOK }

// NeedsAction meldet, ob noch etwas einzurichten ist. Ein optionaler Link ohne
// Quelle zaehlt nicht dazu: dort gibt es nichts zu tun, solange das Projekt die
// Datei nicht selbst anlegt.
func (s LinkStatus) NeedsAction() bool {
	if s.State == StateOK {
		return false
	}
	return !(s.Optional && s.State == StateNoSource)
}

// CheckLinks prueft den Zustand, ohne etwas zu veraendern.
func CheckLinks(projectRoot string) []LinkStatus {
	statuses := make([]LinkStatus, 0, len(Links()))
	for _, link := range Links() {
		statuses = append(statuses, checkLink(projectRoot, link))
	}
	return statuses
}

// LinksOK meldet, ob nichts mehr einzurichten ist.
func LinksOK(statuses []LinkStatus) bool {
	for _, status := range statuses {
		if status.NeedsAction() {
			return false
		}
	}
	return len(statuses) > 0
}

func checkLink(projectRoot string, link Link) LinkStatus {
	status := LinkStatus{Link: link}

	source := filepath.Join(projectRoot, link.Source)
	if !link.sourcePresent(projectRoot) {
		status.State = StateNoSource
		if link.IsFile {
			// Instruktionsdateien gehoeren dem Projekt; wir legen keine an.
			status.Detail = link.Source + " fehlt im Projekt"
		} else {
			status.Detail = link.Source + " fehlt in der Installation"
		}
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

	case info.IsDir() && link.IsFile:
		status.State = StateBlocked
		status.Detail = "Verzeichnis steht im Weg"

	case info.IsDir():
		missing := missingFileLinks(source, target)
		if missing == 0 {
			status.State = StateOK
			status.Detail = "eigenes Verzeichnis, Einzeldateien verlinkt"
		} else {
			status.State = StateOwnDirectory
			status.Detail = fmt.Sprintf("eigenes Verzeichnis, %d Eintraege fehlen", missing)
		}

	case link.IsFile:
		// Typisch nach einem Editor, der "atomar" speichert: er ersetzt den
		// Symlink durch eine echte Datei. Ab dann laufen beide auseinander.
		status.State = StateBlocked
		status.Detail = "echte Datei statt Symlink, Aenderungen erreichen " + link.Source + " nicht"

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
		if !link.sourcePresent(projectRoot) {
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

		case info.IsDir() && !link.IsFile:
			if err := linkFiles(source, target); err != nil {
				return CheckLinks(projectRoot), err
			}

		default:
			// Etwas Echtes steht im Weg. Es gehoert dem Projekt und bleibt
			// liegen; die Pruefung meldet den Zustand.
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
