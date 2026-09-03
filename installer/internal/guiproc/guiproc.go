// Package guiproc verwaltet die Laufzeitdatei des GUI-Servers eines Projekts.
//
// Ein Server je Projekt: Der Schlüssel ist das aufgelöste ProjectDir aus
// project.Detect(), ohne Installation das Arbeitsverzeichnis. Client und
// Server berechnen ihn über dieselbe Funktion, und die Laufzeitdatei trägt
// neben dem Schlüssel Adresse, PID, Version und Startzeit des Prozesses.
//
// Wiedergefunden wird nicht der Port, sondern der Server: erst muss der
// Prozess aus der Datei noch derselbe sein — PID lebt und Startzeit passt —,
// dann muss /api/health denselben Schlüssel melden. Ein Port aus einer alten
// Datei kann längst einem fremden Prozess gehören, und eine PID kann nach
// unsauberem Ende neu vergeben sein.
//
// Die Datei liegt im Laufzeitverzeichnis des Nutzers, nicht im Projekt: die
// Installation ist read-only, und k-playbook-local/ gehört dem Projekt.
package guiproc

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// dirName ist das Unterverzeichnis im Laufzeitverzeichnis.
const dirName = "k-playbook"

// Key liefert den Schlüssel des Projekts, auf dem dieser Prozess arbeitet:
// das aufgelöste ProjectDir aus project.Detect(), ohne Installation das
// Arbeitsverzeichnis. Beides ist absolut und symlink-aufgelöst, damit zwei
// Aufrufe aus demselben Projekt denselben Schlüssel bilden — auch wenn einer
// aus einem Unterverzeichnis kommt.
func Key() (string, error) {
	workdir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("Arbeitsverzeichnis: %w", err)
	}
	return keyFor(project.Detect(), workdir), nil
}

// keyFor ist die Regel hinter Key, ohne Blick auf die Platte.
//
// ProjectDir zählt, sobald es aufgelöst ist — auch beim ersten Start aus einer
// projektlokalen Installation, die noch keine Konfiguration trägt: legt
// POST /api/config sie dort an, bleibt der Schlüssel derselbe.
func keyFor(environment project.Environment, workdir string) string {
	if environment.ProjectDir != "" {
		return environment.ProjectDir
	}
	return canonical(workdir)
}

// canonical macht einen Pfad absolut und löst Symlinks auf — dieselbe
// Normalisierung, die project.Discover für das ProjectDir vornimmt.
func canonical(dir string) string {
	if absolute, err := filepath.Abs(dir); err == nil {
		dir = absolute
	}
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	return dir
}

// OwnVersion ist die VERSION der Installation, die dieses Binary gewählt hat:
// die aus K_PLAYBOOK_INSTALL_DIR, nicht die Projektinstallation — ein bloßes
// `k-playbook` aus dem PATH läuft über die host-weite Spiegelung. Leer, wenn
// InstallDir() nichts liefert.
//
// Einmal beim Start festhalten und nicht je Anfrage lesen: sonst wäre ein
// Wechsel auf der Platte nie zu erkennen.
func OwnVersion() string {
	dir, ok := project.InstallDir()
	if !ok {
		return ""
	}
	return project.InstalledVersion(dir)
}

// Location sind die Pfade eines Schlüssels: das Verzeichnis, die
// Laufzeitdatei und die Logdatei daneben.
type Location struct {
	Dir  string
	File string
	Log  string
}

// Locate löst die Pfade für key auf und legt das Verzeichnis an.
func Locate(key string) (Location, error) {
	dir, err := RuntimeDir()
	if err != nil {
		return Location{}, err
	}
	name := fileName(key)
	return Location{
		Dir:  dir,
		File: filepath.Join(dir, name+".json"),
		Log:  filepath.Join(dir, name+".log"),
	}, nil
}

// RuntimeDir ist das Verzeichnis der Laufzeitdateien: $XDG_RUNTIME_DIR, sonst
// $XDG_STATE_HOME, sonst ~/.local/state — jeweils mit k-playbook darunter.
// Angelegt wird es mit 0700; die Dateien darin nennen Ports und Pfade.
//
// $XDG_RUNTIME_DIR trennt Host und DevContainer, weil es je Sitzung vergeben
// wird. Der Rückfall — auf macOS immer — nimmt ein geteiltes Home als bekannte
// Grenze in Kauf.
func RuntimeDir() (string, error) {
	dir, err := runtimeDirFrom(os.Getenv, os.UserHomeDir)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return "", fmt.Errorf("Laufzeitverzeichnis anlegen: %w", err)
	}
	if err := os.Mkdir(dir, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
		return "", fmt.Errorf("Laufzeitverzeichnis anlegen: %w", err)
	}
	return dir, nil
}

// runtimeDirFrom ist die Auflösung ohne Seiteneffekt, damit sie prüfbar ist.
func runtimeDirFrom(getenv func(string) string, homeDir func() (string, error)) (string, error) {
	if dir := strings.TrimSpace(getenv("XDG_RUNTIME_DIR")); dir != "" {
		return filepath.Join(dir, dirName), nil
	}
	if dir := strings.TrimSpace(getenv("XDG_STATE_HOME")); dir != "" {
		return filepath.Join(dir, dirName), nil
	}
	home, err := homeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return "", errors.New("weder XDG_RUNTIME_DIR noch XDG_STATE_HOME noch ein Home-Verzeichnis bekannt")
	}
	return filepath.Join(home, ".local", "state", dirName), nil
}

// fileName ist der Name der Laufzeitdatei ohne Endung: die ersten 16
// Hex-Zeichen des SHA-256 über den Schlüssel. Ein Pfad taugt nicht als
// Dateiname, und die Kürzung reicht — verglichen wird ohnehin der Schlüssel
// in der Datei.
func fileName(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])[:16]
}

// Record ist der Inhalt der Laufzeitdatei.
type Record struct {
	Key     string `json:"key"`
	Addr    string `json:"addr"`
	PID     int    `json:"pid"`
	Version string `json:"version"`
	// StartTime ist die Startzeit des Prozesses in Unix-Sekunden. Sie
	// unterscheidet den Prozess von einem späteren mit derselben PID.
	StartTime int64 `json:"startTime"`
}

// URL ist die Adresse der Oberfläche.
func (r Record) URL() string {
	return "http://" + r.Addr + "/"
}

// Read liest die Laufzeitdatei unter path. Fehlt sie, ist ok false und err
// nil; eine unlesbare Datei ist ein Fehler.
func Read(path string) (record Record, ok bool, err error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Record{}, false, nil
	}
	if err != nil {
		return Record{}, false, err
	}
	if err := json.Unmarshal(data, &record); err != nil {
		return Record{}, false, fmt.Errorf("%s: %w", path, err)
	}
	return record, true, nil
}

// Remove löscht eine Laufzeitdatei. Eine fehlende ist kein Fehler.
func Remove(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// Registration ist die Laufzeitdatei des laufenden Servers.
type Registration struct {
	mu     sync.Mutex
	record Record
	path   string
}

// Register schreibt die Laufzeitdatei für record.Key. Sie entsteht mit
// O_CREAT|O_EXCL: liegt schon eine, endet Register mit fs.ErrExist, und der
// andere Start hat gewonnen — zwei gleichzeitige Aufrufe ziehen so nicht
// beide einen Server hoch.
func Register(record Record) (*Registration, error) {
	location, err := Locate(record.Key)
	if err != nil {
		return nil, err
	}
	if err := writeExclusive(location.File, record); err != nil {
		return nil, err
	}
	return &Registration{record: record, path: location.File}, nil
}

func writeExclusive(path string, record Record) error {
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(append(data, '\n'))
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		// Eine halbe Datei darf nicht liegen bleiben: sie blockierte jeden
		// weiteren Start, ohne einen Server zu benennen.
		_ = os.Remove(path)
	}
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

// Path ist der Ort der Laufzeitdatei.
func (r *Registration) Path() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.path
}

// Record ist der geschriebene Inhalt.
func (r *Registration) Record() Record {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.record
}

// Rekey schlüsselt um: neue Datei schreiben, alte löschen. Nötig, wenn
// POST /api/config die Konfiguration anlegt und sich das aufgelöste ProjectDir
// dadurch ändert. Bei gleichem Schlüssel passiert nichts.
func (r *Registration) Rekey(key string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if key == r.record.Key {
		return nil
	}
	location, err := Locate(key)
	if err != nil {
		return err
	}
	next := r.record
	next.Key = key
	if err := writeExclusive(location.File, next); err != nil {
		return err
	}
	previous := r.path
	r.record, r.path = next, location.File
	return Remove(previous)
}

// Remove löscht die Laufzeitdatei. Sie gehört beim Beenden weg, sonst fände
// der nächste Aufruf eine Datei ohne Prozess.
func (r *Registration) Remove() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return Remove(r.path)
}
