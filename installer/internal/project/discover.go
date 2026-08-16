// Package project findet die k-playbook-Installation eines Projekts und richtet
// dessen Assistenten-Verlinkung ein.
package project

import (
	"errors"
	"os"
	"path/filepath"
)

// ConfigFileName ist der Anker. Er liegt im Hauptverzeichnis des Projekts, nicht
// in der Installation — dadurch bleibt PlaybookDirName vollständig ersetzbar.
const ConfigFileName = "K-PLAYBOOK.yaml"

// PlaybookDirName ist der Name der Installation innerhalb des Projekts. Er ist
// fest; wie das Projektverzeichnis selbst heißt, spielt keine Rolle.
const PlaybookDirName = "k-playbook"

const (
	// WrapperName ist der Name des Wrappers in bin/ und des Symlinks darauf.
	WrapperName = "k-playbook"
	// BinDirName ist das Verzeichnis, in dem der Wrapper liegt.
	BinDirName = "bin"
)

// ErrNotFound meldet, dass oberhalb des Startverzeichnisses keine Installation liegt.
var ErrNotFound = errors.New("kein k-playbook-Projekt gefunden")

// Discover liefert das Hauptverzeichnis des Projekts, erkannt an der
// K-PLAYBOOK.yaml darin.
//
// Gesucht wird ab startDir aufwärts, ein Kandidat je Ebene. Die Suche bricht
// bewusst nicht am Git-Worktree-Root ab: die Installation ist selbst ein Clone
// und damit ein eigener Worktree, die Config liegt eine Ebene darüber. Ein
// Abbruch dort würde sie unerreichbar machen.
func Discover(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}

	home := homeDir()

	for {
		if fileExists(filepath.Join(dir, ConfigFileName)) {
			return dir, nil
		}

		// $HOME und / werden noch geprüft, aber nicht überschritten.
		if dir == home {
			return "", ErrNotFound
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", ErrNotFound
		}
		dir = parent
	}
}

// PlaybookDir ist die Installation innerhalb eines Projekts.
func PlaybookDir(projectDir string) string {
	return filepath.Join(projectDir, PlaybookDirName)
}

// WrapperPath ist der Wrapper des Projekts — bewusst **relativ** zum
// Hauptverzeichnis.
//
// Genau dieser Wert wird als Kommando bei den Assistenten registriert. Ein
// absoluter Pfad wäre auf jedem Rechner ein anderer und damit nicht teilbar,
// und im DevContainer zeigte er ins Leere. Der Wrapper wählt die Binary selbst
// über `uname`, deshalb genügt derselbe Eintrag für Host und Container.
func WrapperPath() string {
	return filepath.Join(PlaybookDirName, BinDirName, WrapperName)
}

// ConfigPath ist der Ort der K-PLAYBOOK.yaml eines Projekts.
func ConfigPath(projectDir string) string {
	return filepath.Join(projectDir, ConfigFileName)
}

// homeDir liefert das Home-Verzeichnis aufgelöst, damit der Vergleich in
// Discover auch bei verlinktem $HOME greift. Leer, wenn nicht ermittelbar.
func homeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(home); err == nil {
		return resolved
	}
	return home
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
