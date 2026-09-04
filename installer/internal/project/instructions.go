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

// legacyInstructionsCommand ist der Aufruf, den der abgelöste Anstoßblock
// nannte: der projekteigene Wrapper unterhalb der Installation.
//
// Er ist die enge Definition von „veraltet" für den Block — dieselbe Enge wie
// bei der MCP-Registrierung. Nur ein Block, der diesen Aufruf noch trägt, wird
// von selbst ersetzt; jede andere Fassung gehört dem Projekt und bleibt stehen.
// Der Teilpfad und nicht der ganze Wrapper-Pfad, damit auch eine Fassung
// trifft, die eine andere Verzeichnisebene davorgeschrieben hat.
//
// Die Datei `bin/k-playbook` gibt es im Quell-Repo nicht mehr; diese Konstante
// bleibt trotzdem — sie ist es, die Bestandsprojekte von ihr weg migriert.
const legacyInstructionsCommand = "/bin/k-playbook context"

// sessionMemoryMarker hält den nachträglich ergänzten Docs-first-Block vom
// allgemeinen k-playbook-Anstoß getrennt. So erhalten Bestandsprojekte die
// neue Regel, ohne ihren vorhandenen Anstoß doppelt zu bekommen.
const sessionMemoryMarker = "<!-- k-playbook:session-memory -->"

// RootInstructionsState ist der Zustand der Wurzeldatei.
type RootInstructionsState struct {
	Path string `json:"path"`
	// Present: die Datei existiert.
	Present bool `json:"present"`
	// HasMarker: der Anstoß steht bereits darin.
	HasMarker bool `json:"hasMarker"`
	// HasOutdatedAnstoss: der Anstoß steht darin, ruft aber noch den
	// abgelösten Wrapper auf. Er wird beim nächsten Einrichten oder Start
	// ersetzt.
	HasOutdatedAnstoss bool `json:"hasOutdatedAnstoss,omitempty"`
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
		if state.HasMarker {
			_, replaced := replaceOutdatedInstructionsBlock(strings.TrimRight(string(data), "\n"))
			state.HasOutdatedAnstoss = replaced
		}
	}
	return state
}

// ApplyRootInstructions legt die Wurzeldatei an oder ergänzt sie.
//
// Eine vorhandene Datei wird nie überschrieben — sie gehört dem Projekt. Der
// Anstoß wird angehängt und per Marker gegen Dopplung geschützt.
func ApplyRootInstructions(projectDir string) (RootInstructionsState, error) {
	return applyRootInstructions(projectDir, true)
}

// applyRootInstructions kennt zusätzlich den Konfliktfall: dort darf nichts
// angelegt werden.
//
// Zu verhindern ist ausschließlich das Anlegen. Steht bereits eine echte Datei,
// ist das Anhängen richtig und schadet nichts; zeigt AGENTS.md bewusst auf ein
// fremdes Ziel, lebt der Fall geradezu davon, dass der Anstoß durch den Link
// dort ankommt. Das Anlegen dagegen erzeugte neben dem echten Inhalt eine
// zweite, fast leere Instruktionsquelle — und folgte bei einem Rest-Link sogar
// dessen totem Ziel, weil os.WriteFile den Link öffnet.
func applyRootInstructions(projectDir string, mayCreate bool) (RootInstructionsState, error) {
	path := filepath.Join(projectDir, RootInstructionsFile)

	data, err := os.ReadFile(path)
	switch {
	case err != nil && os.IsNotExist(err):
		if !mayCreate {
			return CheckRootInstructions(projectDir), nil
		}
		if err := os.WriteFile(path, []byte(rootInstructionsTemplate()), 0o644); err != nil {
			return CheckRootInstructions(projectDir), fmt.Errorf("%s anlegen: %w", RootInstructionsFile, err)
		}

	case err != nil:
		return CheckRootInstructions(projectDir), fmt.Errorf("%s lesen: %w", RootInstructionsFile, err)

	default:
		content := strings.TrimRight(string(data), "\n")
		changed := false
		switch {
		case !strings.Contains(content, instructionsMarker):
			content += "\n\n" + instructionsBlock()
			changed = true
		default:
			// Der Marker steht — bisher war das gleichbedeutend mit „nichts zu
			// tun". Ein Bestandsprojekt behielt damit dauerhaft den Aufruf des
			// abgelösten Wrappers, und der Git-Update-Weg erreicht die Datei
			// nicht: sie liegt im Hauptverzeichnis, nicht im Clone.
			if replaced, ok := replaceOutdatedInstructionsBlock(content); ok {
				content = replaced
				changed = true
			}
		}
		if !strings.Contains(content, sessionMemoryMarker) {
			content += "\n\n" + sessionMemoryBlock()
			changed = true
		}
		if changed {
			content += "\n"
		}
		if changed {
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				return CheckRootInstructions(projectDir), fmt.Errorf("%s ergänzen: %w", RootInstructionsFile, err)
			}
		}
	}

	return CheckRootInstructions(projectDir), nil
}

// RepairRootInstructions ersetzt einen veralteten Anstoßblock in einer
// **vorhandenen** Wurzeldatei — und sonst nichts.
//
// Das ist der Auffangweg beim Start, das Gegenstück zu RepairMCP: ein
// `git pull` von Hand oder `make -C k-playbook installer-update` erreicht
// AGENTS.md nicht, weil sie im Hauptverzeichnis liegt und nicht im Clone.
//
// Eng und idempotent wie die MCP-Korrektur: eine fehlende Datei wird nicht
// angelegt, ein fehlender Anstoß nicht ergänzt, der Session-Memory-Block nicht
// nachgetragen. Geschrieben wird nur, wenn der vorhandene Block noch den
// abgelösten Wrapper aufruft. Der ausdrückliche Weg für alles Weitere bleibt
// das Einrichten über ApplyRootInstructions.
//
// Zurück kommt, ob geschrieben wurde.
func RepairRootInstructions(projectDir string) (bool, error) {
	path := filepath.Join(projectDir, RootInstructionsFile)

	data, err := os.ReadFile(path)
	if err != nil {
		// Keine lesbare Datei heißt: nichts zu reparieren. Das Anlegen ist
		// Sache des Einrichtens, nicht dieses Weges.
		return false, nil
	}

	content := strings.TrimRight(string(data), "\n")
	replaced, ok := replaceOutdatedInstructionsBlock(content)
	if !ok {
		return false, nil
	}
	if err := os.WriteFile(path, []byte(replaced+"\n"), 0o644); err != nil {
		return false, fmt.Errorf("%s auffrischen: %w", RootInstructionsFile, err)
	}
	return true, nil
}

// replaceOutdatedInstructionsBlock ersetzt den Anstoßblock durch die aktuelle
// Fassung, wenn er noch den abgelösten Wrapper aufruft. Das zweite Ergebnis
// meldet, ob etwas ersetzt wurde.
func replaceOutdatedInstructionsBlock(content string) (string, bool) {
	lines := strings.Split(content, "\n")
	start, end, ok := instructionsBlockBounds(lines)
	if !ok {
		return content, false
	}
	if !strings.Contains(strings.Join(lines[start:end], "\n"), legacyInstructionsCommand) {
		return content, false
	}

	replacement := strings.Split(strings.TrimRight(instructionsBlock(), "\n"), "\n")
	updated := make([]string, 0, len(lines)-(end-start)+len(replacement))
	updated = append(updated, lines[:start]...)
	updated = append(updated, replacement...)
	updated = append(updated, lines[end:]...)
	return strings.Join(updated, "\n"), true
}

// instructionsBlockBounds grenzt den Anstoßblock in den Zeilen ab: die
// Markerzeile bis ausschließlich der ersten Zeile, die nicht mehr dazugehört.
//
// Das Ende ist bewusst eng gefasst. Der Block trägt genau einen Marker und
// genau eine Überschrift; die nächste Überschrift oder der nächste
// HTML-Kommentar — etwa der Session-Memory-Block — beginnt bereits etwas
// anderes. Leerzeilen davor bleiben außerhalb, damit die Trennung zum
// Folgeabschnitt beim Ersetzen erhalten bleibt. Alles, was ein Projekt hinter
// den Block geschrieben hat, wird so nicht mitgenommen.
func instructionsBlockBounds(lines []string) (int, int, bool) {
	start := -1
	for i, line := range lines {
		if strings.Contains(line, instructionsMarker) {
			start = i
			break
		}
	}
	if start < 0 {
		return 0, 0, false
	}

	end := len(lines)
	headings := 0
	for i := start + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, "<!--") {
			end = i
			break
		}
		if strings.HasPrefix(trimmed, "#") {
			headings++
			if headings > 1 {
				end = i
				break
			}
		}
	}
	for end > start+1 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return start, end, true
}

func rootInstructionsTemplate() string {
	return "# AGENTS.md\n\n" + instructionsBlock() + "\n" + sessionMemoryBlock()
}

// instructionsBlock ist der Anstoß. Er nennt bewusst keine Verzeichnisebenen:
// dieselbe Datei liegt im Projekt, in der Installation und im Entwicklungsrepo,
// und ein Verweis auf eine Ebene wäre an zwei dieser Orte falsch. Wo die
// Instruktionen liegen, beantwortet der Aufruf.
//
// Aufgerufen wird das einmal je Host oder DevContainer installierte
// k-playbook unter seinem Namen. Ein projektrelativer Pfad wäre doppelt falsch:
// er zeigte auf den abgelösten Wrapper, und er hinge daran, aus welchem
// Verzeichnis der Assistent gestartet wurde.
func instructionsBlock() string {
	return instructionsMarker + `
## k-playbook

Für dieses Projekt gilt k-playbook. Rufe zu Beginn

    ` + InstalledCommandName + ` context

auf und lies die Dateien aus ` + "`instructions`" + ` in der angegebenen Reihenfolge,
bevor du arbeitest. Die Ausgabe nennt außerdem die aufgelösten Verzeichnisse und
die effektiven Kataloge für Regeln, Reviews und Checks.
`
}

func sessionMemoryBlock() string {
	return sessionMemoryMarker + `
## Projektwissen zuerst

Die autoritative Projektdokumentation beginnt bei
` + "`" + LocalDirName + `/docs/README.md` + "`" + `. Lies diesen Index zuerst, bevor du den
Code analysierst. Erst wenn die Dokumentation fehlt, nicht passt oder ein
konkreter Fix den aktuellen Code verlangt, ist eine Code-Recherche nötig.
`
}
