package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RootInstructionsFile ist die Datei, die Assistenten von sich aus lesen.
// Claude Code liest ausschliesslich CLAUDE.md, OpenCode bevorzugt AGENTS.md.
const RootInstructionsFile = "AGENTS.md"

// instructionsMarker erkennt den bereits eingefuegten Anstoss wieder. Ohne ihn
// wuerde ein zweiter Lauf den Block ein zweites Mal anhaengen.
const instructionsMarker = "<!-- k-playbook:anstoss -->"

// RootInstructionsState ist der Zustand der Wurzeldatei.
type RootInstructionsState struct {
	Path string `json:"path"`
	// Present: die Datei existiert.
	Present bool `json:"present"`
	// HasMarker: der Anstoss steht bereits darin.
	HasMarker bool `json:"hasMarker"`
}

// OK meldet, ob nichts mehr zu tun ist. CLAUDE.md gehoert nicht dazu: der
// Symlink darauf laeuft ueber den Link-Mechanismus.
func (s RootInstructionsState) OK() bool {
	return s.Present && s.HasMarker
}

// CheckRootInstructions prueft, ohne etwas zu veraendern.
func CheckRootInstructions(projectDir string) RootInstructionsState {
	path := filepath.Join(projectDir, RootInstructionsFile)
	state := RootInstructionsState{Path: path}

	if data, err := os.ReadFile(path); err == nil {
		state.Present = true
		state.HasMarker = strings.Contains(string(data), instructionsMarker)
	}
	return state
}

// ApplyRootInstructions legt die Wurzeldatei an oder ergaenzt sie.
//
// Eine vorhandene Datei wird nie ueberschrieben — sie gehoert dem Projekt. Der
// Anstoss wird angehaengt und per Marker gegen Dopplung geschuetzt.
func ApplyRootInstructions(projectDir string) (RootInstructionsState, error) {
	path := filepath.Join(projectDir, RootInstructionsFile)

	data, err := os.ReadFile(path)
	switch {
	case err != nil && os.IsNotExist(err):
		if err := os.WriteFile(path, []byte(rootInstructionsTemplate()), 0o644); err != nil {
			return CheckRootInstructions(projectDir), fmt.Errorf("%s anlegen: %w", RootInstructionsFile, err)
		}

	case err != nil:
		return CheckRootInstructions(projectDir), fmt.Errorf("%s lesen: %w", RootInstructionsFile, err)

	case !strings.Contains(string(data), instructionsMarker):
		content := strings.TrimRight(string(data), "\n") + "\n\n" + instructionsBlock()
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return CheckRootInstructions(projectDir), fmt.Errorf("%s ergaenzen: %w", RootInstructionsFile, err)
		}
	}

	return CheckRootInstructions(projectDir), nil
}

func rootInstructionsTemplate() string {
	return "# AGENTS.md\n\n" + instructionsBlock()
}

// instructionsBlock ist der Anstoss. Er nennt bewusst keine Verzeichnisebenen:
// dieselbe Datei liegt im Projekt, in der Installation und im Entwicklungsrepo,
// und ein Verweis auf eine Ebene waere an zwei dieser Orte falsch. Wo die
// Instruktionen liegen, beantwortet der Aufruf.
func instructionsBlock() string {
	return instructionsMarker + `
## k-playbook

Fuer dieses Projekt gilt k-playbook. Rufe zu Beginn

    ` + PlaybookDirName + `/bin/k-playbook context

auf und lies die Dateien aus ` + "`instructions`" + ` in der angegebenen Reihenfolge,
bevor du arbeitest. Die Ausgabe nennt ausserdem die aufgeloesten Verzeichnisse und
die effektiven Kataloge fuer Regeln, Reviews und Checks.
`
}
