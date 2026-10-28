// Package ghconfig liest die lokale Konfiguration der GitHub CLI, ohne gh
// aufzurufen: welches Konfigurationsverzeichnis gilt und welche Accounts für
// github.com in hosts.yml hinterlegt sind. Gelesen werden nur `user` und die
// Namen unter `users`; die Token-Zeilen daneben werden übergangen und tauchen
// in keiner Antwort auf.
//
// Das Paket steht bewusst unter project und githubaccess: project braucht den
// Befund für die Kontextausgabe (DetectGH), githubaccess für die Verfügbarkeit
// eines gh-Zugangs (docs/github-access.md, Abschnitt Availability). project
// importiert githubaccess für die Umgebungsprüfung; läge das Lesen in project,
// könnte githubaccess es nur über einen Importzyklus erreichen. Eine zweite
// Fassung der Logik wollte niemand — deshalb ein gemeinsames Paket, das nur
// die Standardbibliothek kennt.
//
// Kein Netz, kein Subprozess: `gh auth status` prüft den Token beim Server
// und kostet einen Netzzugriff. Der Preis ist, dass ein abgelaufener Token
// hier als Anmeldung gilt; wer Gewissheit will, testet (k-playbook github
// test).
package ghconfig

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Host ist der Host, um den es geht. Bewusst nur einer: Enterprise-Instanzen
// hätten eigene Accounts je Host und eine eigene Entscheidung je Projekt; das
// wäre ein anderes Feature als das hier. GitHub Enterprise Cloud liegt auf
// github.com und ist damit abgedeckt.
const Host = "github.com"

// ConfigDir folgt derselben Reihenfolge wie gh selbst: GH_CONFIG_DIR, dann
// $XDG_CONFIG_HOME/gh, sonst ~/.config/gh. Leer, wenn kein Heimatverzeichnis
// bekannt ist.
func ConfigDir() string {
	if dir := strings.TrimSpace(os.Getenv("GH_CONFIG_DIR")); dir != "" {
		return dir
	}
	if dir := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); dir != "" {
		return filepath.Join(dir, "gh")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(home); err == nil {
		home = resolved
	}
	return filepath.Join(home, ".config", "gh")
}

// ReadHosts liest den aktiven Account und alle Accounts für Host aus
// hosts.yml im Konfigurationsverzeichnis, der aktive zuerst. Fehlt die Datei
// oder das Verzeichnis, ist nichts hinterlegt: leer, kein Fehler.
func ReadHosts(configDir string) (active string, accounts []string) {
	if configDir == "" {
		return "", []string{}
	}
	data, err := os.ReadFile(filepath.Join(configDir, "hosts.yml"))
	if err != nil {
		return "", []string{}
	}
	return ParseHosts(string(data))
}

// Accounts nennt die hinterlegten Accounts für Host aus dem
// Konfigurationsverzeichnis, das ConfigDir bestimmt — der aktive zuerst.
func Accounts() []string {
	_, accounts := ReadHosts(ConfigDir())
	return accounts
}

// TokenFromEnv meldet eine Anmeldung über GH_TOKEN oder GITHUB_TOKEN. Die
// sticht die Konfigurationsdatei; gh nimmt dann dieses Token, egal welcher
// Account hinterlegt ist.
func TokenFromEnv() bool {
	for _, name := range []string{"GH_TOKEN", "GITHUB_TOKEN"} {
		if strings.TrimSpace(os.Getenv(name)) != "" {
			return true
		}
	}
	return false
}

// ParseHosts liest hosts.yml zeilenweise. Bewusst ohne YAML-Parser: gelesen
// werden zwei Schlüssel, und die Token-Zeilen daneben sollen nie in einem
// Baum stehen, aus dem sie versehentlich ausgegeben werden könnten.
func ParseHosts(content string) (active string, accounts []string) {
	accounts = []string{}

	inHost := false
	usersIndent := -1
	accountIndent := -1

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := lineIndent(line)
		key, value, found := strings.Cut(trimmed, ":")
		if !found {
			continue
		}
		key = strings.Trim(strings.TrimSpace(key), `"'`)
		value = strings.TrimSpace(value)

		if indent == 0 {
			inHost = key == Host
			usersIndent = -1
			accountIndent = -1
			continue
		}
		if !inHost {
			continue
		}

		// Innerhalb von `users` stehen die Accountnamen; alles tiefer darunter
		// gehört zum jeweiligen Account und interessiert hier nicht.
		if usersIndent >= 0 {
			if indent > usersIndent {
				if accountIndent < 0 {
					accountIndent = indent
				}
				if indent == accountIndent && value == "" {
					accounts = addUnique(accounts, key)
				}
				continue
			}
			usersIndent = -1
			accountIndent = -1
		}

		switch {
		case key == "user" && value != "":
			active = strings.Trim(value, `"'`)
		case key == "users" && value == "":
			usersIndent = indent
		}
	}

	// Ältere gh-Fassungen kennen keinen users-Block; dort ist `user` der einzige
	// Account. Und der aktive gehört nach vorn, damit die Oberfläche ihn nicht
	// eigens heraussuchen muss.
	if active != "" {
		accounts = addUnique(accounts, active)
		for index, name := range accounts {
			if name == active && index > 0 {
				accounts = append([]string{active}, append(accounts[:index:index], accounts[index+1:]...)...)
				break
			}
		}
	}
	return active, accounts
}

func lineIndent(line string) int {
	return len(line) - len(strings.TrimLeft(line, " \t"))
}

func addUnique(list []string, value string) []string {
	if value == "" || slices.Contains(list, value) {
		return list
	}
	return append(list, value)
}
