package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RootInstructionsFile ist die Datei, die Assistenten von sich aus lesen.
// Claude Code liest ausschließlich CLAUDE.md, OpenCode bevorzugt AGENTS.md.
const RootInstructionsFile = "AGENTS.md"

// instructionsMarker erkennt den bereits eingefügten Anstoß wieder. Ohne ihn
// würde ein zweiter Lauf den Block ein zweites Mal anhängen.
const instructionsMarker = "<!-- k-playbook:anstoss -->"

// RootInstructionsState ist der Zustand der Wurzeldatei.
type RootInstructionsState struct {
	Path string `json:"path"`
	// Present: die Datei existiert.
	Present bool `json:"present"`
	// HasMarker: der Anstoß steht bereits darin.
	HasMarker bool `json:"hasMarker"`
}

// OK meldet, ob nichts mehr zu tun ist. CLAUDE.md gehört nicht dazu: der
// Symlink darauf läuft über den Link-Mechanismus.
func (s RootInstructionsState) OK() bool {
	return s.Present && s.HasMarker
}

// CheckRootInstructions prüft, ohne etwas zu verändern.
func CheckRootInstructions(projectDir string) RootInstructionsState {
	path := filepath.Join(projectDir, RootInstructionsFile)
	state := RootInstructionsState{Path: path}

	if data, err := os.ReadFile(path); err == nil {
		state.Present = true
		state.HasMarker = strings.Contains(string(data), instructionsMarker)
	}
	return state
}

// ApplyRootInstructions legt die Wurzeldatei an oder ergänzt sie.
//
// Eine vorhandene Datei wird nie überschrieben — sie gehört dem Projekt. Der
// Anstoß wird angehängt und per Marker gegen Dopplung geschützt.
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
			return CheckRootInstructions(projectDir), fmt.Errorf("%s ergänzen: %w", RootInstructionsFile, err)
		}
	}

	return CheckRootInstructions(projectDir), nil
}

func rootInstructionsTemplate() string {
	return "# AGENTS.md\n\n" + instructionsBlock()
}

// instructionsBlock ist der Anstoß. Er nennt bewusst keine Verzeichnisebenen:
// dieselbe Datei liegt im Projekt, in der Installation und im Entwicklungsrepo,
// und ein Verweis auf eine Ebene wäre an zwei dieser Orte falsch. Wo die
// Instruktionen liegen, beantwortet der Aufruf.
func instructionsBlock() string {
	return instructionsMarker + `
## k-playbook

Für dieses Projekt gilt k-playbook. Rufe zu Beginn

    ` + PlaybookDirName + `/bin/k-playbook context

auf und lies die Dateien aus ` + "`instructions`" + ` in der angegebenen Reihenfolge,
bevor du arbeitest. Die Ausgabe nennt außerdem die aufgelösten Verzeichnisse und
die effektiven Kataloge für Regeln, Reviews und Checks.
`
}
