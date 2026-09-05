package project

import (
	"errors"
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

// legacyInstructionsCommandLine markiert in legacyInstructionsBlockLines die
// eine Zeile, die variieren darf: den Aufruf. Erkannt wird sie über
// legacyInstructionsCommand und nicht wörtlich, damit auch eine Fassung trifft,
// die eine weitere Verzeichnisebene davorgeschrieben hat. Der Wert kann in
// einer Markdown-Datei nicht vorkommen.
const legacyInstructionsCommandLine = "\x00aufruf\x00"

// legacyInstructionsBlockLines ist der abgelöste Anstoßblock, Zeile für Zeile,
// so wie er ausgeliefert wurde.
//
// Bewusst eine eigene Kopie und nicht aus instructionsBlock() abgeleitet: der
// abgelöste Block ist Geschichte und ändert sich nicht mehr, der aktuelle darf
// sich jederzeit ändern. Nur so bleibt er als Grenze verlässlich.
//
// Genau darin liegt sein Nutzen: passt er, ist das Ende des Blocks positiv
// bestimmt — auch dann, wenn dahinter Projekt-Prosa ohne eigene Überschrift
// steht, die sonst nichts vom Block trennt.
var legacyInstructionsBlockLines = []string{
	instructionsMarker,
	"## k-playbook",
	"",
	"Für dieses Projekt gilt k-playbook. Rufe zu Beginn",
	"",
	legacyInstructionsCommandLine,
	"",
	"auf und lies die Dateien aus `instructions` in der angegebenen Reihenfolge,",
	"bevor du arbeitest. Die Ausgabe nennt außerdem die aufgelösten Verzeichnisse und",
	"die effektiven Kataloge für Regeln, Reviews und Checks.",
}

// errAnstossEndeUnbestimmt meldet den einen Fall, in dem der abgelöste Aufruf
// dasteht, das Ende des Blocks sich aber nicht bestimmen lässt.
//
// Ersetzt wird dann nichts. Eine Grenze zu raten hieße, bis zum Dateiende zu
// verwerfen — und damit Projekttext, den niemand zurückbekommt. Ein Hinweis ist
// die einzige ehrliche Antwort.
var errAnstossEndeUnbestimmt = errors.New("das Ende des veralteten Anstoßblocks in " +
	RootInstructionsFile + " ließ sich nicht bestimmen; die Datei bleibt unangetastet." +
	" Den Block bitte von Hand auf `" + InstalledCommandName + " context` umstellen.")

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
			_, replaced, err := replaceOutdatedInstructionsBlock(strings.TrimRight(string(data), "\n"))
			// Ein unbestimmbares Blockende ändert nichts daran, dass der
			// abgelöste Aufruf dasteht — nur daran, dass er sich nicht von
			// selbst ersetzen lässt.
			state.HasOutdatedAnstoss = replaced || err != nil
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

	// hint trägt den Fall, in dem der abgelöste Block dasteht, aber nicht
	// abgegrenzt werden kann. Er hält das Einrichten nicht auf — alles Übrige
	// gehört trotzdem getan —, darf aber auch nicht untergehen.
	var hint error

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
			replaced, ok, err := replaceOutdatedInstructionsBlock(content)
			if err != nil {
				hint = err
			}
			if ok {
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

	return CheckRootInstructions(projectDir), hint
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
	replaced, ok, err := replaceOutdatedInstructionsBlock(content)
	if err != nil {
		return false, err
	}
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
// meldet, ob etwas ersetzt wurde, das dritte den Fall aus
// errAnstossEndeUnbestimmt: der abgelöste Aufruf steht da, das Ende des Blocks
// ließ sich aber nicht bestimmen.
func replaceOutdatedInstructionsBlock(content string) (string, bool, error) {
	lines := strings.Split(content, "\n")
	start := instructionsBlockStart(lines)
	if start < 0 {
		return content, false, nil
	}

	end, bounded := instructionsBlockEnd(lines, start)
	if !bounded {
		// Ohne bestimmtes Ende wird nichts ersetzt. Ob überhaupt etwas zu tun
		// wäre, entscheidet der Rest der Datei ab dem Marker: steht der
		// abgelöste Aufruf dort, bekommt der Nutzer einen Hinweis.
		if strings.Contains(strings.Join(lines[start:], "\n"), legacyInstructionsCommand) {
			return content, false, errAnstossEndeUnbestimmt
		}
		return content, false, nil
	}
	if !strings.Contains(strings.Join(lines[start:end], "\n"), legacyInstructionsCommand) {
		return content, false, nil
	}

	replacement := strings.Split(strings.TrimRight(instructionsBlock(), "\n"), "\n")
	updated := make([]string, 0, len(lines)-(end-start)+len(replacement))
	updated = append(updated, lines[:start]...)
	updated = append(updated, replacement...)
	updated = append(updated, lines[end:]...)
	return strings.Join(updated, "\n"), true, nil
}

// instructionsBlockStart meldet die Markerzeile des Anstoßblocks, sonst -1.
func instructionsBlockStart(lines []string) int {
	for i, line := range lines {
		if strings.Contains(line, instructionsMarker) {
			return i
		}
	}
	return -1
}

// instructionsBlockEnd grenzt den Anstoßblock nach hinten ab: die erste Zeile,
// die nicht mehr dazugehört. Das zweite Ergebnis meldet, ob sich diese Grenze
// überhaupt bestimmen ließ.
//
// Zwei Wege, in dieser Reihenfolge:
//
// Erstens die bekannte Gestalt. Der abgelöste Block ist ein endlicher, längst
// ausgelieferter Textbaustein; steht er ab dem Marker Zeile für Zeile da, ist
// sein Ende exakt bekannt, gleichgültig was dahinter folgt.
//
// Zweitens die Abgrenzung am Folgeabschnitt. Der Block trägt genau einen Marker
// und genau eine Überschrift; die nächste Überschrift oder der nächste
// HTML-Kommentar — etwa der Session-Memory-Block — beginnt bereits etwas
// anderes. Leerzeilen davor bleiben außerhalb, damit die Trennung zum
// Folgeabschnitt beim Ersetzen erhalten bleibt.
//
// Findet keiner der beiden Wege eine Grenze, ist das Ergebnis **nicht** das
// Dateiende. Genau diese Annahme verwarf früher Projekt-Prosa, die ohne eigene
// Überschrift und ohne HTML-Kommentar hinter dem Block steht: sie lag innerhalb
// der vermeintlichen Grenzen und verschwand beim Ersetzen — still, denn
// gemeldet wurde nur „korrigiert". Alles, was ein Projekt hinter den Block
// geschrieben hat, bleibt so unangetastet.
func instructionsBlockEnd(lines []string, start int) (int, bool) {
	if end, ok := knownInstructionsBlockEnd(lines, start); ok {
		return end, true
	}

	headings := 0
	for i := start + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		boundary := strings.HasPrefix(trimmed, "<!--")
		if strings.HasPrefix(trimmed, "#") {
			headings++
			boundary = boundary || headings > 1
		}
		if !boundary {
			continue
		}
		end := i
		for end > start+1 && strings.TrimSpace(lines[end-1]) == "" {
			end--
		}
		return end, true
	}
	return 0, false
}

// knownInstructionsBlockEnd meldet das Ende, wenn ab start wörtlich der
// abgelöste Block steht.
//
// Verglichen wird zeilenweise ohne nachlaufende Leerzeichen — die tilgt mancher
// Editor beim Speichern. Die Aufrufzeile zählt, sobald sie den abgelösten
// Aufruf enthält; alle übrigen Zeilen müssen wörtlich stimmen.
func knownInstructionsBlockEnd(lines []string, start int) (int, bool) {
	end := start + len(legacyInstructionsBlockLines)
	if end > len(lines) {
		return 0, false
	}
	for offset, want := range legacyInstructionsBlockLines {
		got := strings.TrimRight(lines[start+offset], " \t")
		if want == legacyInstructionsCommandLine {
			if !strings.Contains(got, legacyInstructionsCommand) {
				return 0, false
			}
			continue
		}
		if got != want {
			return 0, false
		}
	}
	return end, true
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
