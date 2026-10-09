package vscodeext

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Kind ist die Art der gewählten CLI. Sie steht in jeder Ausgabe, weil die
// drei Wege sich im Verhalten unterscheiden und ein Fehlschlag sonst nicht
// einzuordnen ist.
type Kind string

const (
	// KindRemote ist die Remote-CLI des laufenden Servers. Sie trägt den
	// Befehl über VSCODE_IPC_HOOK_CLI an das offene Fenster und ist nur in
	// einem Terminal von VS Code erreichbar.
	KindRemote Kind = "remote-cli"
	// KindServer ist das Server-Binary selbst. Es schreibt direkt in das
	// Erweiterungsverzeichnis und braucht weder Fenster noch IPC-Socket —
	// der Weg des Dienstes.
	KindServer Kind = "code-server"
	// KindDesktop ist ein `code` aus dem PATH.
	KindDesktop Kind = "desktop"
)

// CLI ist die gewählte Befehlszeile samt Art.
type CLI struct {
	Path string `json:"path"`
	Kind Kind   `json:"kind"`
}

// ExtensionsDir ist das Erweiterungsverzeichnis, das ein Aufruf dieser CLI
// beschreibt. Es hängt allein an der Art:
//
//   - KindRemote und KindServer bedienen den Server einer Remote-Sitzung und
//     schreiben nach ~/.vscode-server/extensions. Die Remote-CLI übergeht
//     dabei ein --extensions-dir und schreibt in die laufende Umgebung.
//   - KindDesktop ist ein `code` aus dem PATH, also die lokale Installation:
//     ~/.vscode/extensions.
//
// Gebraucht wird die Zuordnung, weil die Frage „ist nachzuziehen?“ dasselbe
// Verzeichnis meinen muss wie der Nachzug selbst (siehe NeedsInstallIn).
func (c CLI) ExtensionsDir(home string) string {
	if c.Kind == KindDesktop {
		return filepath.Join(home, filepath.FromSlash(localExtensions))
	}
	return filepath.Join(home, filepath.FromSlash(serverExtensions))
}

// ErrNoCLI heißt: in dieser Umgebung ist kein VS Code zu finden. Das ist kein
// Fehler, sondern eine Auskunft — der selbsttätige Weg bleibt dann still.
var ErrNoCLI = errors.New("kein VS Code in dieser Umgebung")

// ManualHint ist der Weg für den Fall, dass keine CLI zu finden ist.
const ManualHint = "VSIX schreiben mit `k-playbook vscode vsix -o k-playbook-workspace-tools.vsix` " +
	"und in VS Code „Extensions: Install from VSIX…“ aufrufen"

// Lookup ist exec.LookPath; als Parameter, damit Tests einen eigenen PATH
// vorgeben können, ohne das echte VS Code zu treffen.
type Lookup func(string) (string, error)

// ChooseCLI wählt die CLI in dieser Reihenfolge:
//
//  1. die Remote-CLI, wenn VSCODE_IPC_HOOK_CLI auf einen vorhandenen Socket
//     zeigt — nur dann ist sie lebendig;
//  2. das neueste ~/.vscode-server/bin/<commit>/bin/code-server;
//  3. ein `code` aus dem PATH, nie unterhalb von /mnt/.
//
// Zu 3.: Das Verbot ist eine Vorsichtsregel, keine Behauptung über das Ziel.
// Gemessen am 2026-10-08 leitet der Windows-Shim aus WSL über wslCode.sh an
// genau die Remote-CLI der WSL-Seite weiter, installiert dort also richtig.
// Er tut das aber nur, solange ms-vscode-remote.remote-wsl windows-seitig
// installiert ist — fehlt sie, fällt er auf die echte Windows-CLI durch —,
// und er lädt bei Bedarf über wslDownload.sh einen Server nach. Diesen
// Nebeneffekt darf ein selbsttätiger Dienstschritt nicht blind auslösen.
func ChooseCLI(home string, getenv func(string) string, lookPath Lookup) (CLI, error) {
	if socket := getenv("VSCODE_IPC_HOOK_CLI"); socket != "" {
		if _, err := os.Stat(socket); err == nil {
			if path, ok := newestServerBinary(home, filepath.Join("bin", "remote-cli", "code")); ok {
				return CLI{Path: path, Kind: KindRemote}, nil
			}
		}
	}

	if path, ok := newestServerBinary(home, filepath.Join("bin", "code-server")); ok {
		return CLI{Path: path, Kind: KindServer}, nil
	}

	if path, err := lookPath("code"); err == nil {
		if underMount(path) {
			return CLI{}, ErrNoCLI
		}
		return CLI{Path: path, Kind: KindDesktop}, nil
	}

	return CLI{}, ErrNoCLI
}

// underMount sagt, ob der Pfad in einem eingebundenen Windows-Laufwerk liegt.
func underMount(path string) bool {
	clean := filepath.ToSlash(filepath.Clean(path))
	return clean == "/mnt" || strings.HasPrefix(clean, "/mnt/")
}

// newestServerBinary sucht ~/.vscode-server/bin/<commit>/<rel> und nimmt das
// Verzeichnis mit der jüngsten Änderungszeit. Die Commit-Kennungen sind
// Hashes und nicht sortierbar; mehrere Server-Fassungen liegen nach einem
// Update von VS Code nebeneinander.
func newestServerBinary(home string, rel string) (string, bool) {
	base := filepath.Join(home, ".vscode-server", "bin")
	entries, err := os.ReadDir(base)
	if err != nil {
		return "", false
	}

	type candidate struct {
		path    string
		modTime int64
		name    string
	}
	var candidates []candidate
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(base, entry.Name(), rel)
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			continue
		}
		dirInfo, err := entry.Info()
		if err != nil {
			continue
		}
		candidates = append(candidates, candidate{path: path, modTime: dirInfo.ModTime().UnixNano(), name: entry.Name()})
	}
	if len(candidates) == 0 {
		return "", false
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].modTime != candidates[j].modTime {
			return candidates[i].modTime > candidates[j].modTime
		}
		return candidates[i].name < candidates[j].name
	})
	return candidates[0].path, true
}
