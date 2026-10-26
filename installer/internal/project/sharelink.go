package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/sharedir"
)

// ProjectDirName ist das Verzeichnis im Projekt, das künftig alles trägt, was
// k-playbook im Projekt braucht (Punkt 21 der Arbeitsliste in
// docs/knowledge-aktuell.md). Heute steht darin nur der Link share.
const ProjectDirName = ".k-playbook"

// shareLinkName ist der Link auf ~/.local/share/k-playbook/ in ProjectDirName.
const shareLinkName = "share"

// shareIgnoreLine nimmt den Link aus der Versionskontrolle. Sein Ziel ist
// absolut und hängt am Home des Rechners; alles, was durch ihn hindurch
// verlinkt ist, bleibt relativ und darf eingecheckt werden.
const shareIgnoreLine = "/" + shareLinkName

// ShareLink ist der Pfad des Links im Projekt.
func ShareLink(projectDir string) string {
	return filepath.Join(projectDir, ProjectDirName, shareLinkName)
}

// shareLinkRel ist ShareLink relativ zur Projektwurzel, für Anzeige und Links().
func shareLinkRel() string {
	return filepath.Join(ProjectDirName, shareLinkName)
}

// SharedDir ist das Verzeichnis der mitgelieferten Dateien dieses Rechners,
// ~/.local/share/k-playbook — Ziel des Links .k-playbook/share und Ort von
// scripts/. Leer, wenn es sich nicht bestimmen lässt.
func SharedDir() string { return sharedDataDir() }

// sharedDataDir ist das Ziel des Links: die mitgelieferten Dateien dieses
// Rechners. Leer, wenn es sich nicht bestimmen lässt.
func sharedDataDir() string {
	dir, err := sharedir.Dir()
	if err != nil {
		return ""
	}
	return dir
}

// checkShareLink prüft den Umweg-Link.
//
// Ein Link, der woandershin zeigt, ist StateStale und wird umgesetzt — der
// typische Fall ist ein Projektverzeichnis, das Host und DevContainer mit
// verschiedenem Home teilen: Jede Umgebung setzt ihn auf ihr eigenes. Wer das
// Hin und Her vermeiden will, bindet ~/.local/share/k-playbook im Container
// unter demselben Pfad ein wie auf dem Host.
func checkShareLink(projectRoot string, link Link) LinkStatus {
	status := LinkStatus{Link: link}

	want := sharedDataDir()
	if want == "" || !isDir(want) {
		status.State = StateNoSource
		status.Detail = "~/.local/share/k-playbook fehlt; jeder Aufruf von k-playbook entpackt es"
		return status
	}

	target := filepath.Join(projectRoot, link.Path)
	info, err := os.Lstat(target)
	switch {
	case err != nil && os.IsNotExist(err):
		status.State = StateMissing
		status.Detail = "nicht vorhanden, zeigt künftig auf " + want

	case err != nil:
		status.State = StateBlocked
		status.Detail = err.Error()

	case info.Mode()&os.ModeSymlink == 0:
		status.State = StateBlocked
		status.Detail = "echte Datei oder echtes Verzeichnis steht im Weg"

	default:
		destination, err := os.Readlink(target)
		switch {
		case err != nil || destination != want:
			status.State = StateStale
			status.Detail = "zeigt auf " + destination + ", wird auf " + want + " umgesetzt"
		case !shareIgnored(projectRoot):
			status.State = StateStale
			status.Detail = "Eintrag " + shareIgnoreLine + " in " + filepath.Join(ProjectDirName, ".gitignore") + " fehlt, wird ergänzt"
		default:
			status.State = StateOK
			status.Detail = "zeigt auf " + want
		}
	}
	return status
}

// applyShareLink setzt den Umweg-Link und den Eintrag in .k-playbook/.gitignore.
// Eine echte Datei an seiner Stelle bleibt liegen; die Prüfung meldet sie.
func applyShareLink(projectRoot string, link Link) error {
	want := sharedDataDir()
	if want == "" || !isDir(want) {
		return nil
	}

	target := filepath.Join(projectRoot, link.Path)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("%s anlegen: %w", ProjectDirName, err)
	}
	if err := ensureShareIgnored(projectRoot); err != nil {
		return err
	}

	info, err := os.Lstat(target)
	switch {
	case err != nil && os.IsNotExist(err):

	case err != nil:
		return fmt.Errorf("%s prüfen: %w", link.Path, err)

	case info.Mode()&os.ModeSymlink == 0:
		return nil

	default:
		if destination, err := os.Readlink(target); err == nil && destination == want {
			return nil
		}
		if err := os.Remove(target); err != nil {
			return fmt.Errorf("%s ersetzen: %w", link.Path, err)
		}
	}

	if err := os.Symlink(want, target); err != nil {
		return fmt.Errorf("%s verlinken: %w", link.Path, err)
	}
	return nil
}

func shareIgnorePath(projectRoot string) string {
	return filepath.Join(projectRoot, ProjectDirName, ".gitignore")
}

// shareIgnored meldet, ob .k-playbook/.gitignore den Link ausnimmt.
func shareIgnored(projectRoot string) bool {
	data, err := os.ReadFile(shareIgnorePath(projectRoot))
	if err != nil {
		return false
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if strings.TrimSpace(line) == shareIgnoreLine {
			return true
		}
	}
	return false
}

// ensureShareIgnored ergänzt die Zeile, ohne Vorhandenes anzufassen.
func ensureShareIgnored(projectRoot string) error {
	if shareIgnored(projectRoot) {
		return nil
	}
	path := shareIgnorePath(projectRoot)
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("%s lesen: %w", path, err)
	}

	content := string(data)
	if content == "" {
		content = "# Rechnerabhängig: zeigt auf ~/.local/share/k-playbook dieses Rechners.\n"
	} else if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += shareIgnoreLine + "\n"

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("%s schreiben: %w", path, err)
	}
	return nil
}
