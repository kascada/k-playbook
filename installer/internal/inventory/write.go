package inventory

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// Outcome sagt, was ein Lauf mit der Inventardatei gemacht hat.
type Outcome struct {
	Path string `json:"path"`
	// Written ist false, wenn der Bestand inhaltlich derselbe war und die Datei
	// deshalb gar nicht angefasst wurde.
	Written bool `json:"written"`
	// At ist der Zeitstempel, der jetzt in der Datei steht: bei einem
	// geschriebenen Lauf der dieses Laufs, sonst der des Bestands.
	At string `json:"at"`
	// Problem übernimmt einen sichtbaren Befund zum Bestand, etwa ein defektes
	// Frontmatter. Dann wird neu geschrieben, weil ein Vergleich nicht möglich
	// ist.
	Problem string `json:"problem,omitempty"`
}

// Write setzt die Byte-Stabilitäts- und Zeitstempelregel des Vertrags um:
//
//  1. Das vollständige Ergebnis wird im Speicher gerendert.
//  2. Existiert die Datei, wird verglichen — ausgenommen `generated.at`, und
//     ausgenommen nichts anderes.
//  3. Sind sie gleich, wird gar nicht geschrieben.
//  4. Unterscheiden sie sich, wird vollständig neu geschrieben, mit
//     `generated.at` = Zeitpunkt dieses Laufs.
//  5. Existiert sie nicht, wird sie geschrieben.
//
// Der Vergleich läuft über den Bestand selbst, nicht über eine Prüfsumme
// daneben: derselbe Bestand wird ein zweites Mal gerendert, mit dem Zeitstempel,
// der schon dort steht. Sind die Bytes dann gleich, hat sich nichts geändert.
func Write(options Options, result Result) (Outcome, error) {
	path := options.InventoryFile
	if path == "" {
		return Outcome{}, fmt.Errorf("kein Zielpfad für die Inventardatei angegeben")
	}
	outcome := Outcome{Path: path}

	existing, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		// Nichts da: schreiben.
	case err != nil:
		outcome.Problem = "Bestand nicht lesbar: " + err.Error()
	default:
		status := Status{Present: true, Path: path}
		fillStatus(&status, existing)
		if status.Problem != "" {
			outcome.Problem = status.Problem
		} else if Render(result, status.GeneratedAt) == string(existing) {
			outcome.At = status.GeneratedAt
			return outcome, nil
		}
	}

	at := options.now().Format(time.RFC3339)
	if err := writeAtomic(path, []byte(Render(result, at))); err != nil {
		return outcome, err
	}
	outcome.Written = true
	outcome.At = at
	return outcome, nil
}

// Run erhebt und schreibt in einem Zug. Das ist der Weg, den jeder Einstieg
// nimmt; die Vertrauensgrenze liegt dahinter und nicht beim Aufrufer.
func Run(options Options) (Result, Outcome, error) {
	result, err := Collect(options)
	if err != nil {
		return Result{}, Outcome{Path: options.InventoryFile}, err
	}
	outcome, err := Write(options, result)
	return result, outcome, err
}

// writeAtomic schreibt über eine Nachbardatei und benennt um. Ein abgebrochener
// Lauf lässt damit den alten Stand stehen statt eine halbe Datei.
func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("%s anlegen: %w", dir, err)
	}
	temporary, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	name := temporary.Name()
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		os.Remove(name)
		return err
	}
	if err := temporary.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Chmod(name, 0o644); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}
