// Package project findet die k-playbook-Installation eines Projekts und richtet
// dessen Assistenten-Verlinkung ein.
package project

import (
	"errors"
	"os"
	"path/filepath"
)

// ConfigFileName ist der Anker. Er liegt im Hauptverzeichnis des Projekts, nicht
// in der Installation — dadurch bleibt PlaybookDirName vollstaendig ersetzbar.
const ConfigFileName = "K-PLAYBOOK.yaml"

// PlaybookDirName ist der Name der Installation innerhalb des Projekts. Er ist
// fest; wie das Projektverzeichnis selbst heisst, spielt keine Rolle.
const PlaybookDirName = "k-playbook"

// ErrNotFound meldet, dass oberhalb des Startverzeichnisses keine Installation liegt.
var ErrNotFound = errors.New("kein k-playbook-Projekt gefunden")

// Discover liefert das Hauptverzeichnis des Projekts, erkannt an der
// K-PLAYBOOK.yaml darin.
//
// Gesucht wird ab startDir aufwaerts, ein Kandidat je Ebene. Die Suche bricht
// bewusst nicht am Git-Worktree-Root ab: die Installation ist selbst ein Clone
// und damit ein eigener Worktree, die Config liegt eine Ebene darueber. Ein
// Abbruch dort wuerde sie unerreichbar machen.
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

		// $HOME und / werden noch geprueft, aber nicht ueberschritten.
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

// ConfigPath ist der Ort der K-PLAYBOOK.yaml eines Projekts.
func ConfigPath(projectDir string) string {
	return filepath.Join(projectDir, ConfigFileName)
}

// homeDir liefert das Home-Verzeichnis aufgeloest, damit der Vergleich in
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
