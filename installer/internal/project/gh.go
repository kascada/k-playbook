package project

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GHHost ist der Host, um den es geht. Bewusst nur einer: Enterprise-Instanzen
// haetten eigene Accounts je Host und eine eigene Entscheidung je Projekt; das
// waere ein anderes Feature als das hier.
const GHHost = "github.com"

// GHStatus ist die Projektentscheidung zur GitHub CLI.
type GHStatus string

const (
	// GHUnknown: noch nicht entschieden. Ausdruecklicher Zustand, kein
	// stillschweigendes Nein — die Oberflaeche zeigt ihn als offenen Punkt.
	GHUnknown GHStatus = "unknown"
	// GHEnabled: das Projekt setzt gh voraus.
	GHEnabled GHStatus = "enabled"
	// GHDisabled: das Projekt kommt ohne gh aus.
	GHDisabled GHStatus = "disabled"
)

// DefaultGHStatus gilt, solange nichts in der Datei steht.
const DefaultGHStatus = GHUnknown

// GHChoice beschreibt eine Entscheidung fuer die Auswahl in der Oberflaeche.
type GHChoice struct {
	Status      GHStatus `json:"status"`
	Label       string   `json:"label"`
	Description string   `json:"description"`
}

// GHChoices sind die waehlbaren Entscheidungen. `unknown` fehlt bewusst: es ist
// der Ausgangszustand, keine Wahl.
func GHChoices() []GHChoice {
	return []GHChoice{
		{
			Status:      GHEnabled,
			Label:       "gh wird genutzt",
			Description: "Commands wie /k-pr-review und das Dependabot-Review duerfen gh voraussetzen und brechen ab, wenn es fehlt oder keine Anmeldung besteht.",
		},
		{
			Status:      GHDisabled,
			Label:       "gh wird nicht genutzt",
			Description: "Commands, die gh brauchen, melden das als nicht vorgesehen, statt zur Installation aufzufordern.",
		},
	}
}

// ValidGHStatus meldet, ob der Wert bekannt ist.
func ValidGHStatus(status GHStatus) bool {
	switch status {
	case GHUnknown, GHEnabled, GHDisabled:
		return true
	}
	return false
}

// GH ist der zusammengefuehrte Zustand: die Projektentscheidung aus der
// Konfiguration und der Host-Befund.
//
// Beides gehoert zusammen in eine Antwort, aber nicht in dieselbe Datei: die
// Entscheidung ist versioniert, der Befund gilt nur fuer diesen Rechner.
type GH struct {
	Host string `json:"host"`
	// Status ist die Projektentscheidung aus tools.gh.status.
	Status GHStatus `json:"status"`
	// Configured meldet, ob der Wert in der Datei stand. Ohne Eintrag gilt
	// unknown, und die Oberflaeche kann das als offenen Punkt zeigen.
	Configured bool `json:"configured"`
	// Installed und Path sind der Host-Befund.
	Installed bool   `json:"installed"`
	Path      string `json:"path"`
	// LoggedIn ist aus der gh-Konfiguration gelesen, nicht geprueft: ein
	// hinterlegter Token kann abgelaufen oder zurueckgezogen sein. Wer Gewissheit
	// braucht, ruft `gh auth status` auf — das kostet einen Netzzugriff.
	LoggedIn bool `json:"loggedIn"`
	// Account ist der aktive Account fuer Host. Leer bei Anmeldung ueber die
	// Umgebung: dort steht nur ein Token, kein Name.
	Account string `json:"account"`
	// Accounts sind alle hinterlegten Accounts, der aktive zuerst.
	Accounts []string `json:"accounts"`
	// TokenFromEnv meldet eine Anmeldung ueber GH_TOKEN oder GITHUB_TOKEN. Die
	// sticht die Konfigurationsdatei, deshalb steht sie hier eigens.
	TokenFromEnv bool `json:"tokenFromEnv"`
	// Ready fasst zusammen, was ein Command wissen muss: gh ist da und angemeldet.
	Ready bool `json:"ready"`
}

// ReadGH liest die Projektentscheidung aus tools.gh.status.
func ReadGH(projectDir string) (GHStatus, bool, error) {
	data, err := os.ReadFile(ConfigPath(projectDir))
	if err != nil {
		return DefaultGHStatus, false, err
	}
	return parseGHStatus(string(data))
}

// parseGHStatus liest tools.gh.status zeilenweise. Wie beim Rest der
// Konfiguration bewusst ohne YAML-Parser: gelesen wird ein Skalar, und
// Kommentare, Reihenfolge und unbekannte Bloecke bleiben unangetastet, wenn
// spaeter zurueckgeschrieben wird.
func parseGHStatus(content string) (GHStatus, bool, error) {
	inTools := false
	toolsIndent := -1
	ghIndent := -1

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
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		if indent == 0 {
			inTools = key == "tools"
			toolsIndent = 0
			ghIndent = -1
			continue
		}
		if !inTools {
			continue
		}
		// Zurueck auf die Ebene der Tool-Namen: der gh-Block ist zu Ende.
		if ghIndent >= 0 && indent <= ghIndent {
			ghIndent = -1
		}
		if ghIndent < 0 {
			if key == "gh" && value == "" && indent > toolsIndent {
				ghIndent = indent
			}
			continue
		}
		if key == "status" {
			status := GHStatus(value)
			if !ValidGHStatus(status) {
				return DefaultGHStatus, true, fmt.Errorf("tools.gh.status hat den unbekannten Wert %q; erlaubt sind unknown, enabled und disabled", value)
			}
			return status, true, nil
		}
	}
	return DefaultGHStatus, false, nil
}

// SetGHStatus schreibt die Projektentscheidung nach tools.gh.status.
//
// Ersetzt wird nur der gh-Block. Ein danebenliegender Block eines anderen Tools
// bleibt unberuehrt.
func SetGHStatus(projectDir string, status GHStatus) error {
	if !ValidGHStatus(status) {
		return fmt.Errorf("unbekannter gh-Status: %s", status)
	}

	path := ConfigPath(projectDir)
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	updated := replaceNestedBlock(string(data), "tools", "gh", ghBlock(status))
	return os.WriteFile(path, []byte(updated), 0o644)
}

func ghBlock(status GHStatus) string {
	return fmt.Sprintf(`  gh:
    # Soll die GitHub CLI genutzt werden? unknown, enabled oder disabled.
    # Ob gh auf diesem Rechner liegt, steht bewusst nicht hier: das ist ein
    # Host-Befund und gehoert in die Kontextausgabe.
    status: %s
`, status)
}

// DetectGH liest den Host-Befund: liegt gh im PATH, und ist ein Account
// hinterlegt.
//
// Bewusst ohne Aufruf von `gh auth status`: der prueft den Token beim Server und
// kostet einen Netzzugriff. Dieser Befund soll billig genug sein, um in der
// Kontextausgabe zu stehen.
func DetectGH() GH {
	state := GH{Host: GHHost, Accounts: []string{}}

	if path, err := exec.LookPath("gh"); err == nil {
		state.Installed = true
		state.Path = path
	}

	active, accounts := readGHHosts(ghConfigDir())
	state.Account = active
	state.Accounts = accounts
	state.LoggedIn = len(accounts) > 0

	// Ein Token in der Umgebung sticht die Konfigurationsdatei. Ohne diesen Fall
	// meldete die Oberflaeche „nicht angemeldet", waehrend gh laeuft.
	if ghTokenFromEnv() {
		state.TokenFromEnv = true
		state.LoggedIn = true
	}

	state.Ready = state.Installed && state.LoggedIn
	return state
}

// GHState fuehrt Entscheidung und Befund zusammen.
func GHState(projectDir string) (GH, error) {
	state := DetectGH()
	status, configured, err := ReadGH(projectDir)
	state.Status = status
	state.Configured = configured
	return state, err
}

func ghTokenFromEnv() bool {
	for _, name := range []string{"GH_TOKEN", "GITHUB_TOKEN"} {
		if strings.TrimSpace(os.Getenv(name)) != "" {
			return true
		}
	}
	return false
}

// ghConfigDir folgt derselben Reihenfolge wie gh selbst.
func ghConfigDir() string {
	if dir := strings.TrimSpace(os.Getenv("GH_CONFIG_DIR")); dir != "" {
		return dir
	}
	if dir := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); dir != "" {
		return filepath.Join(dir, "gh")
	}
	home := homeDir()
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".config", "gh")
}

// readGHHosts liest die Accounts fuer GHHost aus hosts.yml.
//
// Gelesen werden nur `user` und die Namen unter `users`. Die Token-Zeilen daneben
// werden uebergangen und tauchen nirgends in einer Antwort auf.
func readGHHosts(configDir string) (string, []string) {
	if configDir == "" {
		return "", []string{}
	}
	data, err := os.ReadFile(filepath.Join(configDir, "hosts.yml"))
	if err != nil {
		return "", []string{}
	}
	return parseGHHosts(string(data))
}

func parseGHHosts(content string) (string, []string) {
	active := ""
	accounts := []string{}

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
			inHost = key == GHHost
			usersIndent = -1
			accountIndent = -1
			continue
		}
		if !inHost {
			continue
		}

		// Innerhalb von `users` stehen die Accountnamen; alles tiefer darunter
		// gehoert zum jeweiligen Account und interessiert hier nicht.
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

	// Aeltere gh-Fassungen kennen keinen users-Block; dort ist `user` der einzige
	// Account. Und der aktive gehoert nach vorn, damit die Oberflaeche ihn nicht
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

// replaceNestedBlock tauscht einen Unterblock innerhalb eines Blocks auf oberster
// Ebene aus. Fehlt der Unterblock, wird er angehaengt; fehlt der aeussere Block,
// entsteht er neu.
//
// Zeilenweise statt ueber einen YAML-Parser, aus demselben Grund wie
// replaceTopLevelBlock: Kommentare, Reihenfolge und unbekannte Nachbarn bleiben
// erhalten.
func replaceNestedBlock(content string, parent string, child string, block string) string {
	lines := strings.Split(content, "\n")
	blockLines := strings.Split(strings.TrimRight(block, "\n"), "\n")

	parentStart := -1
	for index, line := range lines {
		if isTopLevelYAMLLine(line) && strings.TrimSpace(line) == parent+":" {
			parentStart = index
			break
		}
	}
	if parentStart == -1 {
		trimmed := strings.TrimRight(content, "\n")
		newBlock := parent + ":\n" + strings.TrimRight(block, "\n")
		if trimmed == "" {
			return newBlock + "\n"
		}
		return trimmed + "\n\n" + newBlock + "\n"
	}

	parentEnd := len(lines)
	for index := parentStart + 1; index < len(lines); index++ {
		trimmed := strings.TrimSpace(lines[index])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if isTopLevelYAMLLine(lines[index]) {
			parentEnd = index
			break
		}
	}

	childStart := -1
	childIndent := 0
	for index := parentStart + 1; index < parentEnd; index++ {
		trimmed := strings.TrimSpace(lines[index])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if trimmed == child+":" {
			childStart = index
			childIndent = lineIndent(lines[index])
			break
		}
	}

	if childStart == -1 {
		// Angehaengt wird hinter der letzten inhaltlichen Zeile des Blocks, nicht
		// hinter etwaigen Leerzeilen davor: die trennen den naechsten Block ab.
		insert := parentStart + 1
		for index := parentStart + 1; index < parentEnd; index++ {
			if strings.TrimSpace(lines[index]) != "" {
				insert = index + 1
			}
		}
		result := append([]string{}, lines[:insert]...)
		result = append(result, blockLines...)
		result = append(result, lines[insert:]...)
		return strings.TrimRight(strings.Join(result, "\n"), "\n") + "\n"
	}

	childEnd := parentEnd
	for index := childStart + 1; index < parentEnd; index++ {
		trimmed := strings.TrimSpace(lines[index])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if lineIndent(lines[index]) <= childIndent {
			childEnd = index
			break
		}
	}

	result := append([]string{}, lines[:childStart]...)
	result = append(result, blockLines...)
	result = append(result, lines[childEnd:]...)
	return strings.TrimRight(strings.Join(result, "\n"), "\n") + "\n"
}
