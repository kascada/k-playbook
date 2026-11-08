package project

import (
	"os/exec"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/ghconfig"
)

// GHHost ist der Host, um den es geht — ghconfig.Host, hier unter dem Namen,
// den die Oberfläche und die Kontextausgabe schon tragen. Bewusst nur einer:
// Enterprise-Instanzen hätten eigene Accounts je Host; das wäre ein anderes
// Feature als das hier.
const GHHost = ghconfig.Host

// GH ist der Befund zur GitHub CLI auf diesem Rechner. Eine Projektentscheidung
// darüber gibt es nicht mehr (Task 128): Ob gh genutzt wird, ergibt sich aus
// der GitHub-Zugangsdatei, ohne Dienst daraus, ob gh bereit ist (Ready). Ein
// alter Block tools.gh in K-PLAYBOOK.yaml wird nicht gelesen; das Einrichten
// entfernt ihn (RemoveLegacyGHBlock).
type GH struct {
	Host string `json:"host"`
	// Installed und Path sind der Host-Befund.
	Installed bool   `json:"installed"`
	Path      string `json:"path"`
	// LoggedIn ist aus der gh-Konfiguration gelesen, nicht geprüft: ein
	// hinterlegter Token kann abgelaufen oder zurückgezogen sein. Wer Gewissheit
	// braucht, ruft `gh auth status` auf — das kostet einen Netzzugriff.
	LoggedIn bool `json:"loggedIn"`
	// Account ist der aktive Account für Host. Leer bei Anmeldung über die
	// Umgebung: dort steht nur ein Token, kein Name.
	Account string `json:"account"`
	// Accounts sind alle hinterlegten Accounts, der aktive zuerst.
	Accounts []string `json:"accounts"`
	// TokenFromEnv meldet eine Anmeldung über GH_TOKEN oder GITHUB_TOKEN. Die
	// sticht die Konfigurationsdatei, deshalb steht sie hier eigens.
	TokenFromEnv bool `json:"tokenFromEnv"`
	// Ready fasst zusammen, was ein Command wissen muss: gh ist da und angemeldet.
	Ready bool `json:"ready"`
}

// DetectGH liest den Host-Befund: liegt gh im PATH, und ist ein Account
// hinterlegt.
//
// Bewusst ohne Aufruf von `gh auth status`: der prüft den Token beim Server und
// kostet einen Netzzugriff. Dieser Befund soll billig genug sein, um in der
// Kontextausgabe zu stehen. Die gh-Konfiguration liest das Paket ghconfig,
// das auch githubaccess für die Verfügbarkeit eines gh-Zugangs nutzt — eine
// Fassung der Logik für beide.
func DetectGH() GH {
	state := GH{Host: GHHost, Accounts: []string{}}

	if path, err := exec.LookPath("gh"); err == nil {
		state.Installed = true
		state.Path = path
	}

	active, accounts := ghconfig.ReadHosts(ghconfig.ConfigDir())
	state.Account = active
	state.Accounts = accounts
	state.LoggedIn = len(accounts) > 0

	// Ein Token in der Umgebung sticht die Konfigurationsdatei. Ohne diesen Fall
	// meldete die Oberfläche „nicht angemeldet", während gh läuft.
	if ghconfig.TokenFromEnv() {
		state.TokenFromEnv = true
		state.LoggedIn = true
	}

	state.Ready = state.Installed && state.LoggedIn
	return state
}

func lineIndent(line string) int {
	return len(line) - len(strings.TrimLeft(line, " \t"))
}

// replaceNestedBlock tauscht einen Unterblock innerhalb eines Blocks auf oberster
// Ebene aus. Fehlt der Unterblock, wird er angehängt; fehlt der äußere Block,
// entsteht er neu.
//
// Zeilenweise statt über einen YAML-Parser, aus demselben Grund wie
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
		// Auch ein Schlüssel mit Wert in derselben Zeile zählt, etwa
		// "languages: []". Sonst fände der zweite Aufruf seinen eigenen ersten
		// nicht wieder und hängte einen zweiten Block an.
		if trimmed == child+":" || strings.HasPrefix(trimmed, child+": ") {
			childStart = index
			childIndent = lineIndent(lines[index])
			break
		}
	}

	if childStart == -1 {
		// Angehängt wird hinter der letzten inhaltlichen Zeile des Blocks, nicht
		// hinter etwaigen Leerzeilen davor: die trennen den nächsten Block ab.
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
	// Leerzeilen am Ende gehören nicht mehr zum Unterblock, sondern trennen ihn
	// vom nächsten ab. Würden sie mitersetzt, klebte der Rest der Datei nach
	// jedem Schreiben eine Zeile näher.
	for childEnd > childStart+1 && strings.TrimSpace(lines[childEnd-1]) == "" {
		childEnd--
	}

	result := append([]string{}, lines[:childStart]...)
	result = append(result, blockLines...)
	result = append(result, lines[childEnd:]...)
	return strings.TrimRight(strings.Join(result, "\n"), "\n") + "\n"
}
