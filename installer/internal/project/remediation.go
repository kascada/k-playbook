package project

import (
	"fmt"
	"os"
	"strings"
)

// RemediationMode bestimmt, wie Befunde aus Reviews abgearbeitet werden.
type RemediationMode string

const (
	// ModeTaskBranchPR: keine direkten Fixes. Jedes bestätigte Bündel wird
	// eine Task mit Branch- und PR-Hinweis.
	ModeTaskBranchPR RemediationMode = "task-branch-pr"
	// ModeTaskFirst: Tasks sind der Standard, direkte Fixes nur nach
	// ausdrücklicher Freigabe einzelner kleiner Bündel.
	ModeTaskFirst RemediationMode = "task-first"
	// ModeDirectAllowed: kleine, sichere Befunde dürfen nach Code-Sichtung
	// direkt behoben werden.
	ModeDirectAllowed RemediationMode = "direct-allowed"
)

// DefaultRemediationMode gilt, solange nichts anderes eingestellt ist. Tasks
// als Standard sind die sichere Vorgabe: nichts wird ohne Zutun am Code
// geändert, direkte Fixes bleiben nach Freigabe trotzdem möglich.
const DefaultRemediationMode = ModeTaskFirst

// RemediationChoice beschreibt einen Modus für die Auswahl in der Oberfläche.
type RemediationChoice struct {
	Mode        RemediationMode `json:"mode"`
	Label       string          `json:"label"`
	Description string          `json:"description"`
}

// RemediationModes sind die wählbaren Modi, vom striktesten zum offensten.
func RemediationModes() []RemediationChoice {
	return []RemediationChoice{
		{
			Mode:        ModeTaskBranchPR,
			Label:       "Task, Branch und PR",
			Description: "Keine direkten Fixes. Jedes bestätigte Bündel wird eine Task mit Branch- und PR-Hinweis; umgesetzt wird später über /k-run.",
		},
		{
			Mode:        ModeTaskFirst,
			Label:       "Task zuerst",
			Description: "Tasks sind der Standard. Direkte Fixes nur, wenn du sie für einzelne kleine Bündel ausdrücklich freigibst.",
		},
		{
			Mode:        ModeDirectAllowed,
			Label:       "Direkte Fixes erlaubt",
			Description: "Kleine, sichere Befunde dürfen nach Code-Sichtung sofort behoben werden, wenn du die Kategorien freigibst.",
		},
	}
}

// RemediationPolicy leitet die Flags aus dem Modus ab. Sie stehen zusätzlich
// in der Datei, damit Commands sie lesen können, ohne den Modus zu deuten.
func RemediationPolicy(mode RemediationMode) (prRequired bool, directFixes bool) {
	if mode == ModeTaskBranchPR {
		return true, false
	}
	return false, true
}

// ValidRemediationMode meldet, ob der Modus bekannt ist.
func ValidRemediationMode(mode RemediationMode) bool {
	for _, choice := range RemediationModes() {
		if choice.Mode == mode {
			return true
		}
	}
	return false
}

// Remediation ist der gelesene Zustand aus der Konfiguration.
type Remediation struct {
	Mode         RemediationMode `json:"mode"`
	Target       string          `json:"target"`
	Grouping     bool            `json:"grouping"`
	QuickWins    bool            `json:"quickWins"`
	BranchPrefix string          `json:"branchPrefix"`
	PRRequired   bool            `json:"prRequired"`
	DirectFixes  bool            `json:"directFixes"`
	// Configured meldet, ob ein remediation-Block vorhanden war.
	Configured bool `json:"configured"`
}

// ReadRemediation liest die Remediation-Einstellungen eines Projekts.
func ReadRemediation(projectDir string) (Remediation, error) {
	data, err := os.ReadFile(ConfigPath(projectDir))
	if err != nil {
		return Remediation{}, err
	}

	// Ohne Block gilt der Standard. Configured bleibt false, damit die
	// Oberfläche zeigen kann, dass der Wert noch nicht in der Datei steht.
	remediation := Remediation{Mode: DefaultRemediationMode}
	remediation.PRRequired, remediation.DirectFixes = RemediationPolicy(DefaultRemediationMode)
	section := ""
	for _, line := range strings.Split(string(data), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		key, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		indented := strings.HasPrefix(key, " ") || strings.HasPrefix(key, "\t")
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		if !indented {
			section = key
			if section == "remediation" {
				remediation.Configured = true
			}
			continue
		}
		if section != "remediation" {
			continue
		}

		switch key {
		case "mode":
			remediation.Mode = RemediationMode(value)
		case "target":
			remediation.Target = value
		case "branch_prefix":
			remediation.BranchPrefix = value
		case "grouping":
			remediation.Grouping = value == "true"
		case "quick_wins":
			remediation.QuickWins = value == "true"
		case "pr_required":
			remediation.PRRequired = value == "true"
		case "direct_fixes":
			remediation.DirectFixes = value == "true"
		}
	}
	return remediation, nil
}

// SetRemediationMode schreibt den Modus samt abgeleiteter Flags.
//
// Ersetzt wird der gesamte remediation-Block: die Flags hängen am Modus, ein
// Teilupdate könnte sie widersprüchlich zurücklassen. Alles außerhalb des
// Blocks bleibt unangetastet, samt Kommentaren und unbekannten Feldern.
func SetRemediationMode(projectDir string, mode RemediationMode) error {
	if !ValidRemediationMode(mode) {
		return fmt.Errorf("unbekannter Remediation-Modus: %s", mode)
	}

	path := ConfigPath(projectDir)
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	updated := replaceTopLevelBlock(string(data), "remediation", remediationBlock(mode))
	return os.WriteFile(path, []byte(updated), 0o644)
}

func remediationBlock(mode RemediationMode) string {
	prRequired, directFixes := RemediationPolicy(mode)
	return fmt.Sprintf(`remediation:
  # Wie Befunde aus Reviews abgearbeitet werden.
  mode: %s
  target: .
  grouping: true
  quick_wins: true
  branch_prefix: remediation/
  # Aus dem Modus abgeleitet; Commands lesen sie direkt.
  pr_required: %t
  direct_fixes: %t
`, mode, prRequired, directFixes)
}

// replaceTopLevelBlock tauscht einen Block auf oberster Ebene aus. Fehlt er,
// wird er angehängt. Zeilenweise statt über einen YAML-Parser, damit
// Kommentare, Reihenfolge und unbekannte Felder erhalten bleiben.
func replaceTopLevelBlock(content string, name string, block string) string {
	lines := strings.Split(content, "\n")

	start := -1
	for index, line := range lines {
		if isTopLevelYAMLLine(line) && strings.TrimSpace(line) == name+":" {
			start = index
			break
		}
	}
	if start == -1 {
		trimmed := strings.TrimRight(content, "\n")
		if trimmed == "" {
			return block
		}
		return trimmed + "\n\n" + block
	}

	// Bis zur nächsten Zeile auf oberster Ebene; Leerzeilen und Kommentare
	// dazwischen gehören noch zum Block.
	end := len(lines)
	for index := start + 1; index < len(lines); index++ {
		trimmed := strings.TrimSpace(lines[index])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if isTopLevelYAMLLine(lines[index]) {
			end = index
			break
		}
	}

	result := append([]string{}, lines[:start]...)
	result = append(result, strings.Split(strings.TrimRight(block, "\n"), "\n")...)
	if end < len(lines) {
		result = append(result, "")
		result = append(result, lines[end:]...)
	}
	return strings.TrimRight(strings.Join(result, "\n"), "\n") + "\n"
}

// isTopLevelYAMLLine erkennt eine Zeile ohne Einrückung, die kein Kommentar
// und keine Listenzeile ist.
func isTopLevelYAMLLine(line string) bool {
	if line == "" || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
		return false
	}
	trimmed := strings.TrimSpace(line)
	return trimmed != "" && !strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "-")
}
