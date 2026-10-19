// Package hostenv beantwortet, wo das Programm gerade läuft: in einem
// Container, unter WSL. Die Antworten brauchen mehrere Stellen — der
// Browser-Start der Oberfläche, der Chat-Wächter, die Fertig-Meldung —, und
// jede eigene Fassung derselben Erkennung driftet.
package hostenv

import (
	"os"
	"strings"
)

// procVersion ist die Datei, an der WSL sich zu erkennen gibt. Eine Variable,
// damit Tests eine nachgebaute unterschieben können.
var procVersion = "/proc/version"

// ContainerMarker meldet, ob der Prozess in einem Container läuft, und woran
// das erkannt wurde. Dort taugen geratene Programme des Desktops nicht: sie
// liefen im Container und nicht auf dem Rechner vor dem Nutzer.
func ContainerMarker() (string, bool) {
	for _, name := range []string{"REMOTE_CONTAINERS", "DEVCONTAINER", "CODESPACES"} {
		if strings.TrimSpace(os.Getenv(name)) != "" {
			return name, true
		}
	}
	if workdir, err := os.Getwd(); err == nil && strings.HasPrefix(workdir, "/workspaces/") {
		return "/workspaces/", true
	}
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return "/.dockerenv", true
	}
	return "", false
}

// IsWSL meldet, ob der Prozess in einer WSL-Distribution läuft. WSL setzt
// WSL_DISTRO_NAME und WSL_INTEROP in jede Sitzung; fehlen beide — etwa in
// einem Dienst, der ohne die Sitzungsumgebung startet —, bleibt der Kernel,
// der sich in /proc/version als Microsoft-Kernel nennt.
func IsWSL() bool {
	for _, name := range []string{"WSL_DISTRO_NAME", "WSL_INTEROP"} {
		if strings.TrimSpace(os.Getenv(name)) != "" {
			return true
		}
	}
	content, err := os.ReadFile(procVersion)
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(content)), "microsoft")
}
