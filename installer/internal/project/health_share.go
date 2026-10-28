package project

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/health"
	"github.com/kascada/k-playbook/installer/internal/sharedir"
)

// Die Quelle share der Umgebungsprüfung: Sie warnt, wenn der Link
// .k-playbook/share zwischen Host und DevContainer hin- und herspringen wird.
//
// Teilen Host und Container den Projektordner bei verschiedenem Home, setzt
// jede Umgebung den Link bei jedem Aufruf auf ihr eigenes Verzeichnis, und in
// der anderen fehlen die mitgelieferten Commands — ohne Fehlermeldung, weil
// Claude Code Links ohne Ziel still weglässt. Abhilfe ist sharedir.EnvDir im
// Container (sharelink.go, docs/installation.md).
//
// Geprüft wird ohne Zustand, aus dem, was der Container selbst sieht: läuft
// k-playbook in einem Container, fehlt die Variable, und ist der Projektordner
// vom Host eingehängt. Ein beobachteter Wechsel des Links taugt dafür nicht:
// Briefing, GUI und MCP heilen den Link, und wer zuerst heilt, sieht den
// Wechsel; die anderen sehen einen Link, der stimmt. Gewarnt wird deshalb nur
// im Container, dort, wo die Abhilfe hingehört; auf dem Host bleibt es still.
//
// Ein Docker- oder Podman-Volume (Repository im Container-Volume geklont)
// teilt nichts mit dem Host und zählt nicht. Wer das Projekt eingehängt hat,
// aber auf dem Host nie k-playbook darin aufruft, setzt die Variable auf das
// eigene Verzeichnis; dann schweigt die Prüfung.
func init() {
	health.Register(healthShare)
}

// Prüfpunkte als Variablen, damit Tests einen Container nachstellen können.
var (
	containerMarkers = []string{"/.dockerenv", "/run/.containerenv"}
	mountInfoPath    = "/proc/self/mountinfo"
	shareGetenv      = os.Getenv
)

func healthShare(_ context.Context, projectDir string) []health.Problem {
	if _, err := sharedir.Dir(); err != nil {
		return []health.Problem{{
			Level:   health.LevelWarning,
			Area:    health.AreaShare,
			Message: "Das Verzeichnis der mitgelieferten Dateien ist unbekannt: " + err.Error() + ".",
			Hint: "Die Variable " + sharedir.EnvDir + " auf einen absoluten Pfad setzen oder entfernen; " +
				"bis dahin fehlen die mitgelieferten Commands und Skills.",
		}}
	}
	if strings.TrimSpace(shareGetenv(sharedir.EnvDir)) != "" || !inContainer() {
		return nil
	}
	mount, ok := hostMountOf(projectDir)
	if !ok {
		return nil
	}
	own := SharedDir()
	return []health.Problem{{
		Level: health.LevelWarning,
		Area:  health.AreaShare,
		Message: "Dieser Container teilt den Projektordner mit dem Host (" + mount + "), aber " + sharedir.EnvDir +
			" ist nicht gesetzt: Jeder Aufruf von k-playbook hier setzt .k-playbook/share auf " + own +
			", und auf dem Host fehlen danach die mitgelieferten Commands und Skills.",
		Hint: "In .devcontainer/devcontainer.json unter containerEnv " + sharedir.EnvDir +
			" auf \"${localEnv:HOME}/.local/share/k-playbook\" setzen, mit eigenem Volume an diesem Pfad und " +
			"einem chown in postCreateCommand (k-playbook-Doku docs/installation.md, „Host and DevContainer sharing the project“). " +
			"Ruft auf dem Host niemand k-playbook in diesem Projekt auf, die Variable auf " + own + " setzen.",
	}}
}

// inContainer erkennt Docker und Podman an ihren Markierungsdateien.
func inContainer() bool {
	for _, marker := range containerMarkers {
		if _, err := os.Stat(marker); err == nil {
			return true
		}
	}
	return false
}

// hostMountOf meldet, ob projectDir auf einem Bind-Mount vom Host liegt, und
// nennt dessen Einhängepunkt. Maßgeblich ist der Mount mit dem längsten
// Einhängepunkt, der projectDir enthält. Ein Bind-Mount trägt in mountinfo
// den Pfad auf dem Host als Wurzel; das Dateisystem des Containers selbst
// (overlay) und ein Volume (Wurzel unter …/volumes/…) zählen nicht.
// Gemessen 2026-10-28 mit Docker Desktop 29.7.2 unter WSL.
func hostMountOf(projectDir string) (string, bool) {
	file, err := os.Open(mountInfoPath)
	if err != nil {
		return "", false
	}
	defer file.Close()

	dir := filepath.Clean(projectDir)
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	var best mountEntry
	found := false
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		entry, ok := parseMountInfo(scanner.Text())
		if !ok || !pathWithin(dir, entry.point) {
			continue
		}
		if !found || len(entry.point) > len(best.point) {
			best, found = entry, true
		}
	}
	if !found || best.point == "/" || best.fsType == "overlay" || best.root == "/" ||
		strings.Contains(best.root, "/volumes/") {
		return "", false
	}
	return best.point, true
}

type mountEntry struct {
	root, point, fsType string
}

// parseMountInfo liest eine Zeile aus /proc/self/mountinfo: Feld 4 ist die
// Wurzel, Feld 5 der Einhängepunkt, nach dem Trenner "-" folgt der Typ.
func parseMountInfo(line string) (mountEntry, bool) {
	fields := strings.Fields(line)
	if len(fields) < 7 {
		return mountEntry{}, false
	}
	sep := -1
	for i := 6; i < len(fields); i++ {
		if fields[i] == "-" {
			sep = i
			break
		}
	}
	if sep < 0 || sep+1 >= len(fields) {
		return mountEntry{}, false
	}
	return mountEntry{
		root:   unescapeMount(fields[3]),
		point:  unescapeMount(fields[4]),
		fsType: fields[sep+1],
	}, true
}

// unescapeMount löst die Oktal-Escapes auf, mit denen der Kernel Leerzeichen,
// Tabs, Zeilenumbrüche und Backslashes in mountinfo schreibt (\040 usw.).
func unescapeMount(value string) string {
	if !strings.Contains(value, `\`) {
		return value
	}
	var out strings.Builder
	for i := 0; i < len(value); i++ {
		if value[i] == '\\' && i+4 <= len(value) {
			if n, err := strconv.ParseUint(value[i+1:i+4], 8, 8); err == nil {
				out.WriteByte(byte(n))
				i += 3
				continue
			}
		}
		out.WriteByte(value[i])
	}
	return out.String()
}

// pathWithin meldet, ob path gleich dir ist oder darunter liegt.
func pathWithin(path, dir string) bool {
	if dir == "/" {
		return true
	}
	return path == dir || strings.HasPrefix(path, dir+string(filepath.Separator))
}
