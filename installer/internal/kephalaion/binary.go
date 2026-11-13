package kephalaion

import (
	"os"
	"os/exec"
	"path/filepath"
)

// BinaryName ist der Programmname von Kephalaion.
const BinaryName = "kephalaion"

// GlobalBinary ist der Ort einer globalen Installation von kephalaion.
const GlobalBinary = "/usr/local/bin/" + BinaryName

// InstallCommand ist die Installation aus dem README von Kephalaion: das
// install.sh des neuesten Releases. Die Hinweise von k-playbook nennen ihn als
// Weg von Hand; `k-playbook upgrade` lädt dasselbe Skript selbst
// (program.InstallKephalaion, Adresse release.KephalaionInstallURL) — ein
// Clone mit bin/install-kephalaion wird dafür nicht gebraucht.
const InstallCommand = "curl -fsSL https://github.com/kephalaion/kephalaion/releases/latest/download/install.sh | sh"

// FindBinary sucht kephalaion an den Orten, die Kephalaion für Clients
// festlegt, in dieser Reihenfolge: PATH, ~/.local/bin/kephalaion (install.sh),
// /usr/local/bin/kephalaion (globale Installation). Ein MCP-Server, den ein
// Assistent mit grafischer Oberfläche startet, erbt PATH nicht immer; darum
// die festen Orte danach. Ohne Binary gibt es keine Auskunft (QueryClient)
// und damit keine Anbindung.
func FindBinary() (string, bool) {
	return LookupBinary()
}

// LookupBinary ist die Suche hinter FindBinary. Tests anderer Pakete ersetzen
// sie (kephtest.Binary, kephtest.NoBinary), damit sie weder vom PATH noch vom
// Heimatverzeichnis des Rechners abhängen — und nie das echte kephalaion
// fragen.
var LookupBinary = func() (string, bool) {
	home, _ := os.UserHomeDir()
	return locateBinary(exec.LookPath, binaryFallbacks(home))
}

// binaryFallbacks sind die festen Orte nach PATH. Ohne bekanntes
// Heimatverzeichnis bleibt nur der globale.
func binaryFallbacks(home string) []string {
	var places []string
	if home != "" {
		places = append(places, filepath.Join(home, ".local", "bin", BinaryName))
	}
	return append(places, GlobalBinary)
}

// locateBinary nimmt zuerst, was lookPath im PATH findet, dann den ersten
// festen Ort mit einer ausführbaren Datei.
func locateBinary(lookPath func(string) (string, error), fallbacks []string) (string, bool) {
	if path, err := lookPath(BinaryName); err == nil {
		return path, true
	}
	for _, path := range fallbacks {
		if isExecutableFile(path) {
			return path, true
		}
	}
	return "", false
}

// isExecutableFile sagt, ob path eine reguläre Datei mit einem Recht zum
// Ausführen ist (einem Link wird gefolgt, wie beim Start).
func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0
}
