// Package project findet die k-playbook-Installation eines Projekts und richtet
// dessen Assistenten-Verlinkung ein.
package project

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ConfigFileName ist der Anker. Er liegt im Hauptverzeichnis des Projekts, nicht
// in der Installation — dadurch bleibt PlaybookDirName vollständig ersetzbar.
const ConfigFileName = "K-PLAYBOOK.yaml"

// PlaybookDirName ist der Name der Installation innerhalb des Projekts. Er ist
// fest; wie das Projektverzeichnis selbst heißt, spielt keine Rolle.
const PlaybookDirName = "k-playbook"

// VersionFileName koppelt einen Clone-Stand an ein Binary. Die Datei liegt im
// Wurzelverzeichnis der Installation und nennt den Release-Tag, dessen Assets
// zu diesem Stand gehören. Content-Commits ändern sie nicht.
const VersionFileName = "VERSION"

// SumsFileName trägt die Prüfsummen der Release-Assets. Sie liegt versioniert
// im Repo, damit die erwartete Summe über den Git-Remote kommt und nicht über
// dieselbe HTTPS-Quelle wie das Binary.
const SumsFileName = "SHA256SUMS"

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

// writeJSONFileAtomic schreibt payload als eingerücktes JSON nach path: erst
// eine temporäre Datei im selben Verzeichnis, dann ein Rename. Ein
// abgebrochener Lauf hinterlässt damit keine halbe Datei, sondern gar keine —
// und ein Leser sieht entweder den alten oder den neuen Stand, nie einen
// dazwischen. Das Verzeichnis entsteht, wenn es fehlt.
//
// label steht am Anfang jeder Fehlermeldung („Todos schreiben"), damit ein
// Fehler die Datei benennt, um die es ging.
func writeJSONFileAtomic(path string, payload any, label string) error {
	content, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("%s: %w", label, err)
	}
	content = append(content, '\n')

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("%s anlegen: %w", dir, err)
	}

	// Das Muster der temporären Datei leitet sich aus dem Ziel ab: bleibt
	// nach einem Absturz eine liegen, ist ihr anzusehen, wozu sie gehörte.
	temp, err := os.CreateTemp(dir, "."+strings.TrimSuffix(filepath.Base(path), ".json")+"-*.json")
	if err != nil {
		return fmt.Errorf("%s: %w", label, err)
	}
	tempPath := temp.Name()
	fail := func(err error) error {
		os.Remove(tempPath)
		return fmt.Errorf("%s: %w", label, err)
	}
	if _, err := temp.Write(content); err != nil {
		temp.Close()
		return fail(err)
	}
	if err := temp.Close(); err != nil {
		return fail(err)
	}
	// CreateTemp legt mit 0600 an; die Datei soll lesbar sein wie jede andere
	// im Projekt.
	if err := os.Chmod(tempPath, 0o644); err != nil {
		return fail(err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fail(err)
	}
	return nil
}
