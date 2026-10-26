// Package sharedir entpackt die mitgereisten Dateien aus share/ nach
// ~/.local/share/k-playbook/.
//
// Das Verzeichnis gilt für alle Projekte eines Rechners und wird einmal je
// Rechner gepflegt, nicht je Projekt. Es ersetzt auf Dauer, was heute im Clone
// <projekt>/k-playbook/ als Datei auf der Platte liegen muss: die Commands und
// die Skills der Assistenten. In Stufe 1 (docs/knowledge-aktuell.md, Punkt 20
// der Arbeitsliste) wird nur entpackt; verlinkt wird noch nichts.
package sharedir

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kascada/k-playbook/share"
)

const (
	dirName = "k-playbook"

	// SumName ist die Datei mit der Prüfsumme des entpackten Inhalts. Sie
	// entscheidet, ob entpackt wird — nicht VERSION: jeder lokale Build
	// derselben VERSION (`make dev-install`) trägt dieselbe Version, aber
	// womöglich anderen Inhalt.
	SumName = ".inhalt-sha256"

	// VersionName nennt dem Leser, welches Programm zuletzt entpackt hat.
	VersionName = "VERSION"

	lockName = ".lock"

	// Schreibgeschützt sind nur die Dateien. Gesperrte Verzeichnisse hielten
	// auch ein `rm -rf` des Nutzers auf, und jeder Tausch müsste die Sperre
	// erst lösen; gegen das versehentliche Ändern einer Datei genügt ihr
	// eigener Modus.
	fileMode = 0o444
	dirMode  = 0o755
)

// Dir ist das Zielverzeichnis: $XDG_DATA_HOME/k-playbook, sonst
// ~/.local/share/k-playbook — auf macOS derselbe Pfad, weil das Binary dort
// ebenfalls in ~/.local/bin liegt und ~/Library/Application Support ein
// Leerzeichen trägt.
func Dir() (string, error) {
	return dirFrom(os.Getenv, os.UserHomeDir)
}

// dirFrom ist die Auflösung ohne Seiteneffekt, damit sie prüfbar ist.
func dirFrom(getenv func(string) string, homeDir func() (string, error)) (string, error) {
	if dir := strings.TrimSpace(getenv("XDG_DATA_HOME")); dir != "" && filepath.IsAbs(dir) {
		return filepath.Join(dir, dirName), nil
	}
	home, err := homeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return "", errors.New("weder XDG_DATA_HOME noch ein Home-Verzeichnis bekannt")
	}
	return filepath.Join(home, ".local", "share", dirName), nil
}

// Ensure bringt das Zielverzeichnis auf den eingebetteten Stand und meldet,
// ob es dazu entpacken musste. version landet nur in VERSION und wird auch bei
// gleichem Inhalt nachgeführt.
func Ensure(version string) (bool, error) {
	dir, err := Dir()
	if err != nil {
		return false, err
	}
	return ensure(dir, share.Files, share.Roots, version)
}

func ensure(dir string, fsys fs.FS, roots []string, version string) (bool, error) {
	sum, err := Sum(fsys, roots)
	if err != nil {
		return false, err
	}
	// Der häufige Fall ohne Sperre: eine kleine Datei lesen und vergleichen.
	if current(dir) == sum {
		return false, syncVersion(dir, version)
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, fmt.Errorf("%s anlegen: %w", dir, err)
	}
	release, err := lock(filepath.Join(dir, lockName))
	if err != nil {
		return false, err
	}
	defer release()
	// Wer auf die Sperre gewartet hat, findet in der Regel nichts mehr zu tun.
	if current(dir) == sum {
		return false, syncVersion(dir, version)
	}

	staging, err := os.MkdirTemp(dir, ".neu-")
	if err != nil {
		return false, fmt.Errorf("Zwischenverzeichnis anlegen: %w", err)
	}
	defer os.RemoveAll(staging)

	for _, root := range roots {
		if err := unpack(fsys, root, filepath.Join(staging, root)); err != nil {
			return false, err
		}
	}
	for _, root := range roots {
		if err := swap(filepath.Join(staging, root), filepath.Join(dir, root), filepath.Join(staging, ".alt-"+root)); err != nil {
			return false, err
		}
	}

	if err := syncVersion(dir, version); err != nil {
		return true, err
	}
	// Zuletzt: bricht der Lauf vorher ab, entpackt der nächste noch einmal.
	if err := writeFile(filepath.Join(dir, SumName), sum+"\n"); err != nil {
		return true, err
	}
	return true, nil
}

// syncVersion bringt VERSION auf version, ohne etwas zu schreiben, wenn es
// schon stimmt. Es läuft auch, wenn der Inhalt gleich geblieben ist: Ein
// Release ohne Änderung an share/ entpackt nichts, VERSION soll trotzdem das
// Programm nennen, das zuletzt lief — sonst stünde dort nach `make dev-install`
// und dem Release derselben Dateien die Version davor.
func syncVersion(dir, version string) error {
	path := filepath.Join(dir, VersionName)
	if version == "" {
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%s entfernen: %w", VersionName, err)
		}
		return nil
	}
	if data, err := os.ReadFile(path); err == nil && strings.TrimSpace(string(data)) == version {
		return nil
	}
	return writeFile(path, version+"\n")
}

// Sum ist die SHA-256 über Pfade und Inhalte aller Dateien unter roots, in
// fester Reihenfolge.
func Sum(fsys fs.FS, roots []string) (string, error) {
	var files []string
	for _, root := range roots {
		err := fs.WalkDir(fsys, root, func(name string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !entry.IsDir() {
				files = append(files, name)
			}
			return nil
		})
		if err != nil {
			return "", fmt.Errorf("eingebetteten Inhalt lesen: %w", err)
		}
	}
	sort.Strings(files)

	hash := sha256.New()
	for _, name := range files {
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			return "", fmt.Errorf("eingebetteten Inhalt lesen: %w", err)
		}
		// Länge vor Pfad und Inhalt, damit keine zwei Bestände dieselbe
		// Bytefolge ergeben.
		fmt.Fprintf(hash, "%d:%s\n%d:", len(name), name, len(data))
		hash.Write(data)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// current ist die Prüfsumme des entpackten Standes, leer, wenn sie fehlt.
func current(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, SumName))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// unpack schreibt root aus fsys nach target.
func unpack(fsys fs.FS, root, target string) error {
	return fs.WalkDir(fsys, root, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(strings.TrimPrefix(name, root), "/")
		out := filepath.Join(target, filepath.FromSlash(rel))
		if entry.IsDir() {
			return os.MkdirAll(out, dirMode)
		}
		in, err := fsys.Open(name)
		if err != nil {
			return err
		}
		defer in.Close()
		file, err := os.OpenFile(out, os.O_CREATE|os.O_EXCL|os.O_WRONLY, fileMode)
		if err != nil {
			return fmt.Errorf("%s schreiben: %w", path.Join(root, rel), err)
		}
		if _, err := io.Copy(file, in); err != nil {
			_ = file.Close()
			return fmt.Errorf("%s schreiben: %w", path.Join(root, rel), err)
		}
		return file.Close()
	})
}

// swap stellt fresh an die Stelle von target; ein vorhandenes target wandert
// nach aside und wird mit dem Zwischenverzeichnis entfernt.
func swap(fresh, target, aside string) error {
	if _, err := os.Lstat(target); err == nil {
		if err := os.Rename(target, aside); err != nil {
			return fmt.Errorf("%s beiseitelegen: %w", target, err)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%s prüfen: %w", target, err)
	}
	if err := os.Rename(fresh, target); err != nil {
		return fmt.Errorf("%s einsetzen: %w", target, err)
	}
	return nil
}

// writeFile ersetzt name über eine Zwischendatei, damit ein Leser nie eine
// halbe Datei sieht.
func writeFile(name, content string) error {
	tmp, err := os.CreateTemp(filepath.Dir(name), "."+filepath.Base(name)+"-")
	if err != nil {
		return fmt.Errorf("%s schreiben: %w", name, err)
	}
	if _, err := tmp.WriteString(content); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("%s schreiben: %w", name, err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("%s schreiben: %w", name, err)
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("%s schreiben: %w", name, err)
	}
	if err := os.Rename(tmp.Name(), name); err != nil {
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("%s schreiben: %w", name, err)
	}
	return nil
}
