// Package hostinstall hält eine host-weite Kopie der Installation aktuell.
//
// Die Installation selbst liegt pro Projekt (`<projekt>/k-playbook/`). Damit die
// Oberfläche nicht über diesen tiefen Pfad gestartet werden muss, spiegelt
// jeder Start seine eigenen Dateien nach ~/.local/share/k-playbook/installation
// und verlinkt sie nach ~/.local/bin. Wer aus einem aktuelleren Clone startet,
// hebt die host-weite Kopie damit von selbst an.
//
// Gespiegelt wird der Wrapper zusammen mit dem Binary, nicht das Binary allein:
// Host und Container teilen sich unter Umständen dasselbe Home, brauchen aber
// verschiedene Plattformen. Erst der Wrapper wählt zur Laufzeit aus.
package hostinstall

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/kascada/k-playbook/installer/internal/project"
)

const (
	// WrapperName ist der Name des Wrappers in bin/ und des Symlinks.
	WrapperName = "k-playbook"
	binDirName  = "bin"
	distDirName = "dist"
	// installDirName trennt die Spiegelung von den Tool-venvs, die unter
	// ~/.local/share/k-playbook/ ebenfalls zuhause sind. Ein venv bringt ein
	// eigenes bin/ mit; ohne diese Ebene kollidierten beide.
	installDirName = "installation"
	// stampSuffix hält den Commit-Stand neben dem gespiegelten Binary fest.
	stampSuffix = ".stamp"
	// stampTimeout begrenzt die Git-Abfrage. Sie läuft lokal und ist schnell;
	// ein hängendes Git darf den Start trotzdem nicht aufhalten.
	stampTimeout = 5 * time.Second
)

// Result beschreibt, was die Spiegelung getan hat. Alle Felder sind leer, wenn
// nichts zu tun war — der Normalfall bei jedem weiteren Start.
type Result struct {
	// Copied nennt die gespiegelten Dateien relativ zum Ziel.
	Copied []string
	// Link ist der angelegte oder korrigierte Symlink. Leer, wenn er bereits
	// stimmte oder nicht angefasst werden durfte.
	Link string
	// PathHint ist gesetzt, wenn das Linkverzeichnis nicht im PATH liegt.
	PathHint string
}

// Empty meldet, ob nichts passiert ist.
func (r Result) Empty() bool {
	return len(r.Copied) == 0 && r.Link == "" && r.PathHint == ""
}

// request bündelt die aufgelösten Pfade eines Spiegellaufs.
type request struct {
	// source ist die Installation, aus der der laufende Prozess stammt.
	source string
	// target ist die host-weite Installation.
	target string
	// linkDir nimmt den Symlink auf; hier greift der PATH.
	linkDir string
	// platform ist der Dateiname des Binaries in dist/.
	platform string
	// stamp ist der Commit-Stand der Quelle. Leer, wenn nicht ermittelbar.
	stamp string
	// pathValue ist der PATH, gegen den linkDir geprüft wird.
	pathValue string
}

// Mirror spiegelt die laufende Installation host-weit.
//
// Läuft der Prozess bereits aus der host-weiten Kopie, passiert nichts. Fehlt
// das Home-Verzeichnis oder lässt sich die Installation nicht bestimmen, wird
// still aufgegeben: die Spiegelung ist Komfort und darf nichts erzwingen.
func Mirror() (Result, error) {
	source, ok := project.InstallDir()
	if !ok {
		return Result{}, nil
	}

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return Result{}, nil
	}

	target := filepath.Join(home, ".local", "share", WrapperName, installDirName)
	if samePath(source, target) {
		return Result{}, nil
	}

	return mirrorInto(request{
		source:    source,
		target:    target,
		linkDir:   filepath.Join(home, ".local", binDirName),
		platform:  PlatformBinary(runtime.GOOS, runtime.GOARCH),
		stamp:     sourceStamp(source),
		pathValue: os.Getenv("PATH"),
	})
}

// PlatformBinary bildet den Dateinamen in dist/. GOOS und GOARCH tragen bereits
// die Schreibweise, die beim Bauen verwendet wird — anders als `uname`, das der
// Wrapper erst übersetzen muss.
func PlatformBinary(goos string, goarch string) string {
	return fmt.Sprintf("%s-%s-%s", WrapperName, goos, goarch)
}

// PathStatus meldet, ob `k-playbook` ohne Pfadangabe aufrufbar ist.
type PathStatus struct {
	// Dir ist das Verzeichnis, in das verlinkt wird.
	Dir string `json:"dir"`
	// Linked: der Symlink liegt tatsächlich dort.
	Linked bool `json:"linked"`
	// InPath: das Verzeichnis steht in der PATH-Variablen dieses Prozesses.
	InPath bool `json:"inPath"`
	// Export ist die Zeile fürs Shell-Profil. Leer, wenn nichts zu tun ist.
	Export string `json:"export"`
}

// OK meldet, ob nichts mehr zu tun ist.
func (s PathStatus) OK() bool { return s.Linked && s.InPath }

// CheckPath prüft, ohne etwas zu verändern.
//
// Geprüft wird der PATH **dieses** Prozesses. Wer die Zeile gerade erst ins
// Profil geschrieben hat, sieht die Änderung deshalb erst in einer neuen Shell.
func CheckPath() PathStatus {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return PathStatus{}
	}

	dir := filepath.Join(home, ".local", binDirName)
	status := PathStatus{
		Dir:    dir,
		Linked: linkExists(filepath.Join(dir, WrapperName)),
		InPath: inPath(os.Getenv("PATH"), dir),
	}
	if !status.InPath {
		status.Export = ExportLine(dir, home)
	}
	return status
}

// ExportLine baut die Zeile fürs Shell-Profil. Wenn das Verzeichnis unter dem
// Home liegt, wird `$HOME` eingesetzt: die Zeile bleibt dann auch dann richtig,
// wenn dasselbe Profil auf einem anderen Rechner oder im Container gelesen wird.
func ExportLine(dir string, home string) string {
	value := dir
	if relative, err := filepath.Rel(home, dir); err == nil && !strings.HasPrefix(relative, "..") {
		value = filepath.Join("$HOME", relative)
	}
	return fmt.Sprintf("export PATH=\"%s:$PATH\"", value)
}

// linkExists meldet, ob dort ein Eintrag liegt — auch ein Symlink, dessen Ziel
// gerade fehlt. Genau der wäre sonst unsichtbar und der Zustand irreführend.
func linkExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// mirrorInto arbeitet auf ausdrücklich übergebenen Pfaden und ist dadurch
// prüfbar.
func mirrorInto(req request) (Result, error) {
	result := Result{}

	sourceWrapper := filepath.Join(req.source, binDirName, WrapperName)
	sourceBinary := filepath.Join(req.source, distDirName, req.platform)
	targetWrapper := filepath.Join(req.target, binDirName, WrapperName)
	targetBinary := filepath.Join(req.target, distDirName, req.platform)
	stampPath := targetBinary + stampSuffix

	if needsCopy(req.stamp, stampPath, targetWrapper, targetBinary) {
		if !fileExists(sourceBinary) {
			return result, fmt.Errorf("in der Quelle fehlt %s", sourceBinary)
		}
		if !fileExists(sourceWrapper) {
			return result, fmt.Errorf("in der Quelle fehlt %s", sourceWrapper)
		}

		if err := copyExecutable(sourceWrapper, targetWrapper); err != nil {
			return result, err
		}
		result.Copied = append(result.Copied, filepath.Join(binDirName, WrapperName))

		if err := copyExecutable(sourceBinary, targetBinary); err != nil {
			return result, err
		}
		result.Copied = append(result.Copied, filepath.Join(distDirName, req.platform))

		// Ohne Stempel bleibt die Datei weg: ein leerer Wert würde beim
		// nächsten Start wie "unbekannt" gelesen und nichts ändern.
		if req.stamp != "" {
			if err := writeStamp(stampPath, req.stamp); err != nil {
				return result, err
			}
		}
	}

	link, err := ensureLink(req.linkDir, targetWrapper)
	if err != nil {
		return result, err
	}
	result.Link = link

	if !inPath(req.pathValue, req.linkDir) {
		result.PathHint = req.linkDir
	}
	return result, nil
}

// needsCopy entscheidet, ob gespiegelt wird.
//
// Der Stempel allein reicht nicht: die Kopie trägt nur die Plattformen, von
// denen aus sie schon einmal aufgerufen wurde. Startet ein Container aus
// demselben Clone, den zuvor der Host gespiegelt hat, sind die Stände gleich —
// sein Binary fehlt trotzdem.
func needsCopy(sourceStamp string, stampPath string, targetWrapper string, targetBinary string) bool {
	if !fileExists(targetBinary) || !fileExists(targetWrapper) {
		return true
	}
	return newer(sourceStamp, readStamp(stampPath))
}

// newer vergleicht zwei Commit-Zeitpunkte als Sekunden seit Epoch.
//
// Bewusst nicht die mtime der Dateien: Git setzt sie beim Auschecken auf den
// Zeitpunkt des Clones, nicht des Commits. Ein frisch geklonter alter Stand
// sähe damit neuer aus als eine korrekte Installation.
func newer(source string, target string) bool {
	sourceValue, err := strconv.ParseInt(source, 10, 64)
	if err != nil {
		return false
	}
	targetValue, err := strconv.ParseInt(target, 10, 64)
	if err != nil {
		return true
	}
	return sourceValue > targetValue
}

// sourceStamp liest den Zeitpunkt des letzten Commits, der dist/ angefasst hat.
// Leer, wenn die Quelle kein Git-Repository ist — dann wird nur gespiegelt,
// falls im Ziel etwas fehlt.
func sourceStamp(source string) string {
	ctx, cancel := context.WithTimeout(context.Background(), stampTimeout)
	defer cancel()

	stamp, err := project.GitOutput(ctx, source, "log", "-1", "--format=%ct", "--", distDirName)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(stamp)
}

func readStamp(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(content))
}

func writeStamp(path string, stamp string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(stamp+"\n"), 0o644)
}

// copyExecutable schreibt erst daneben und benennt dann um.
//
// Das Umbenennen ist atomar und umgeht ETXTBSY: eine parallel laufende Instanz
// hält die alte Datei offen, während der Name schon auf die neue zeigt.
func copyExecutable(source string, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}

	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()

	temporary := target + ".tmp"
	out, err := os.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(temporary)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(temporary)
		return err
	}
	// Explizit, weil eine vorhandene Datei ihre alten Rechte behält.
	if err := os.Chmod(temporary, 0o755); err != nil {
		os.Remove(temporary)
		return err
	}

	if err := os.Rename(temporary, target); err != nil {
		os.Remove(temporary)
		return err
	}
	return nil
}

// ensureLink legt den Symlink an oder richtet ihn neu aus. Liegt dort eine
// echte Datei, gewinnt sie und bleibt unberührt — dieselbe Regel wie bei den
// Assistenten-Verlinkungen. Rückgabe ist der Pfad, wenn etwas geschrieben
// wurde, sonst leer.
func ensureLink(linkDir string, targetWrapper string) (string, error) {
	if err := os.MkdirAll(linkDir, 0o755); err != nil {
		return "", err
	}

	linkPath := filepath.Join(linkDir, WrapperName)
	// Relativ, damit der Link ein verschobenes oder gemountetes Home überlebt.
	want, err := filepath.Rel(linkDir, targetWrapper)
	if err != nil {
		want = targetWrapper
	}

	info, err := os.Lstat(linkPath)
	switch {
	case os.IsNotExist(err):
		// fällt durch zum Anlegen
	case err != nil:
		return "", err
	case info.Mode()&os.ModeSymlink == 0:
		return "", nil
	default:
		current, err := os.Readlink(linkPath)
		if err == nil && current == want {
			return "", nil
		}
		if err := os.Remove(linkPath); err != nil {
			return "", err
		}
	}

	if err := os.Symlink(want, linkPath); err != nil {
		return "", err
	}
	return linkPath, nil
}

// inPath meldet, ob dir in der PATH-Liste steht.
func inPath(pathValue string, dir string) bool {
	for _, entry := range filepath.SplitList(pathValue) {
		if entry == "" {
			continue
		}
		if samePath(entry, dir) {
			return true
		}
	}
	return false
}

// samePath vergleicht zwei Pfade und löst dabei Symlinks auf, soweit sie
// existieren. Ein verlinktes Home soll nicht als anderer Ort gelten.
func samePath(left string, right string) bool {
	return resolve(left) == resolve(right)
}

func resolve(path string) string {
	cleaned := filepath.Clean(path)
	if resolved, err := filepath.EvalSymlinks(cleaned); err == nil {
		return resolved
	}
	return cleaned
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
