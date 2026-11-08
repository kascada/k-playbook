package project

import (
	"errors"
	"io/fs"
	"os"
	"strings"
)

// LegacyGHBlockName ist der Name, unter dem das Einrichten das Entfernen des
// alten Blocks meldet (AssistantSetup.ConfigBlocksRemoved).
const LegacyGHBlockName = "tools.gh"

// Bis Task 128 stand in K-PLAYBOOK.yaml ein Block tools.gh mit dem Feld
// status (unknown, enabled, disabled): die Projektentscheidung, ob gh genutzt
// wird. Die gibt es nicht mehr — ob gh im Spiel ist, ergibt sich aus der
// GitHub-Zugangsdatei, ohne Dienst daraus, ob gh bereit ist. Gelesen wird der
// Block nirgends mehr; Kontext, Briefing und MCP lassen ihn unbeachtet stehen.
// Entfernt wird er nur im Schreibweg des Einrichtens (ApplyAssistantSetup),
// ohne Rückfrage, aber mit Meldung in der Antwort.

// DescribeConfigBlocksRemoved ist der Satz, mit dem `k-playbook upgrade` und
// die Oberfläche das Entfernen melden — eine Fassung für beide.
func DescribeConfigBlocksRemoved(names []string) string {
	return strings.Join(names, " und ") + " aus " + ConfigFileName +
		" entfernt; ob gh genutzt wird, ergibt sich aus der GitHub-Zugangsdatei, ohne Dienst daraus, ob gh bereit ist."
}

// LegacyGHBlockPresent meldet, ob K-PLAYBOOK.yaml noch einen Block tools.gh
// trägt. Eine fehlende oder unlesbare Datei zählt als „nein": Melden soll das
// der Weg, der sie liest, nicht diese Prüfung.
func LegacyGHBlockPresent(projectDir string) bool {
	data, err := os.ReadFile(ConfigPath(projectDir))
	if err != nil {
		return false
	}
	_, removed := removeLegacyGHBlock(string(data))
	return removed
}

// RemoveLegacyGHBlock entfernt den Block tools.gh samt seiner Kommentare aus
// K-PLAYBOOK.yaml. Andere Blöcke unter tools: (etwa tools.mcp) und ihre
// Kommentare bleiben; bleibt unter tools: nichts, fällt auch tools:. Ohne
// Block bleibt die Datei unberührt, und die Antwort ist false. Eine fehlende
// Datei ist kein Fehler: Dann gibt es nichts zu entfernen.
func RemoveLegacyGHBlock(projectDir string) (bool, error) {
	path := ConfigPath(projectDir)
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	updated, removed := removeLegacyGHBlock(string(data))
	if !removed {
		return false, nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	if err := os.WriteFile(path, []byte(updated), info.Mode().Perm()); err != nil {
		return false, err
	}
	return true, nil
}

// removeLegacyGHBlock ist der zeilenweise Kern von RemoveLegacyGHBlock, auf
// denselben Helfern wie replaceNestedBlock (isTopLevelYAMLLine, lineIndent)
// und ohne YAML-Parser: Kommentare, Reihenfolge und unbekannte Nachbarn
// bleiben erhalten.
//
// Zum Block gehören die Zeile gh: unter tools:, alles tiefer Eingerückte
// darunter und Kommentarzeilen direkt darüber auf seiner Ebene. Kommentare und
// Leerzeilen am Ende des Bereichs, die nicht tiefer eingerückt sind, gehören
// schon zum nächsten Block und bleiben. Dasselbe gilt für tools: selbst: Ein
// Kommentar auf oberster Ebene direkt über tools: bleibt stehen — in der alten
// Vorlage ist das der auskommentierte Block docs:.
func removeLegacyGHBlock(content string) (string, bool) {
	lines := strings.Split(content, "\n")

	parentStart := -1
	for index, line := range lines {
		if isTopLevelYAMLLine(line) && strings.TrimSpace(line) == "tools:" {
			parentStart = index
			break
		}
	}
	if parentStart == -1 {
		return content, false
	}

	parentEnd := len(lines)
	for index := parentStart + 1; index < len(lines); index++ {
		if isTopLevelYAMLLine(lines[index]) {
			parentEnd = index
			break
		}
	}
	parentEnd = trimTrailingForeign(lines, parentStart, parentEnd, 0)

	// Die Ebene der Tool-Namen ist die Einrückung der ersten inhaltlichen
	// Zeile; ein gh: tiefer darunter gehört zu einem anderen Tool.
	childLevel := -1
	childStart := -1
	for index := parentStart + 1; index < parentEnd; index++ {
		trimmed := strings.TrimSpace(lines[index])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := lineIndent(lines[index])
		if childLevel < 0 {
			childLevel = indent
		}
		if indent == childLevel && (trimmed == "gh:" || strings.HasPrefix(trimmed, "gh: ")) {
			childStart = index
			break
		}
	}
	if childStart == -1 {
		return content, false
	}

	childEnd := parentEnd
	for index := childStart + 1; index < parentEnd; index++ {
		trimmed := strings.TrimSpace(lines[index])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if lineIndent(lines[index]) <= childLevel {
			childEnd = index
			break
		}
	}
	childEnd = trimTrailingForeign(lines, childStart, childEnd, childLevel)

	// Kommentarzeilen direkt über gh: auf seiner Ebene oder tiefer gehören
	// zu ihm.
	for childStart-1 > parentStart {
		previous := lines[childStart-1]
		if !strings.HasPrefix(strings.TrimSpace(previous), "#") || lineIndent(previous) < childLevel {
			break
		}
		childStart--
	}

	lines = append(lines[:childStart:childStart], lines[childEnd:]...)
	parentEnd -= childEnd - childStart

	remains := false
	for index := parentStart + 1; index < parentEnd; index++ {
		trimmed := strings.TrimSpace(lines[index])
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			remains = true
			break
		}
	}
	if !remains {
		lines = append(lines[:parentStart:parentStart], lines[parentEnd:]...)
		// Zwei Leerzeilen an der Nahtstelle werden eine.
		if parentStart > 0 && parentStart < len(lines) &&
			strings.TrimSpace(lines[parentStart-1]) == "" && strings.TrimSpace(lines[parentStart]) == "" {
			lines = append(lines[:parentStart:parentStart], lines[parentStart+1:]...)
		}
	}

	return strings.TrimRight(strings.Join(lines, "\n"), "\n") + "\n", true
}

// trimTrailingForeign schiebt das Ende eines Bereichs (start, end) über
// Leerzeilen und Kommentarzeilen zurück, die nicht tiefer als level
// eingerückt sind: Sie stehen vor dem nächsten Block und gehören zu ihm.
func trimTrailingForeign(lines []string, start int, end int, level int) int {
	for end-1 > start {
		line := lines[end-1]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || (strings.HasPrefix(trimmed, "#") && lineIndent(line) <= level) {
			end--
			continue
		}
		break
	}
	return end
}
