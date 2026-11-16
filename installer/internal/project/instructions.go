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

// legacyInstructionsBlock ist eine abgelöste Fassung des Anstoßblocks: der
// Aufruf, an dem sie erkannt wird, und ihre Zeilen, so wie sie ausgeliefert
// wurde.
//
// Der Aufruf ist die enge Definition von „veraltet" für den Block — dieselbe
// Enge wie bei der MCP-Registrierung. Nur ein Block, der einen abgelösten
// Aufruf noch trägt, wird von selbst ersetzt; jede andere Fassung gehört dem
// Projekt und bleibt stehen.
//
// Die Zeilen sind bewusst eine eigene Kopie und nicht aus instructionsBlock()
// abgeleitet: eine abgelöste Fassung ist Geschichte und ändert sich nicht
// mehr, die aktuelle darf sich jederzeit ändern. Nur so bleibt sie als Grenze
// verlässlich. Genau darin liegt ihr Nutzen: passt sie, ist das Ende des
// Blocks positiv bestimmt — auch dann, wenn dahinter Projekt-Prosa ohne eigene
// Überschrift steht, die sonst nichts vom Block trennt.
type legacyInstructionsBlock struct {
	command string
	lines   []string
}

// legacyInstructionsCommandLine markiert in den Zeilen einer abgelösten
// Fassung die eine Zeile, die variieren darf: den Aufruf. Erkannt wird sie
// über den Aufruf der Fassung und nicht wörtlich, damit auch eine Fassung
// trifft, die eine weitere Verzeichnisebene davorgeschrieben hat. Der Wert
// kann in einer Markdown-Datei nicht vorkommen.
const legacyInstructionsCommandLine = "\x00aufruf\x00"

// legacyInstructionsBlocks sind die abgelösten Fassungen des Anstoßblocks, die
// älteste zuerst.
//
// Die erste nannte den projekteigenen Wrapper unterhalb des früheren Clones
// (`k-playbook/bin/k-playbook context`); erkannt wird der Teilpfad und nicht
// der ganze Wrapper-Pfad, damit auch eine Fassung trifft, die eine andere
// Verzeichnisebene davorgeschrieben hat. Die Datei `bin/k-playbook` gibt es im
// Quell-Repo nicht mehr; die Fassung bleibt trotzdem — sie ist es, die
// Bestandsprojekte von ihr weg migriert.
//
// Die zweite nannte das installierte Programm mit `k-playbook context` und
// verlangte, die Dateien aus `instructions` zu lesen. Beides kommt seit dem
// Briefing mit einem Aufruf; die aktuelle Fassung (instructionsBlock) nennt
// deshalb `k-playbook briefing`.
//
// Beide nennen noch „Kataloge für Regeln, Reviews und Checks“. Das ist der
// Wortlaut von damals, an dem der Block erkannt wird, nicht der heutige
// Stand: Regeln und Checks gibt es nicht mehr (Task 135, Task 155). Wer ihn
// ändert, erkennt die ausgelieferten Fassungen nicht mehr.
var legacyInstructionsBlocks = []legacyInstructionsBlock{
	{
		command: "/bin/" + InstalledCommandName + " context",
		lines: []string{
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
		},
	},
	{
		command: InstalledCommandName + " context",
		lines: []string{
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
		},
	},
}

// hasLegacyInstructionsCommand meldet, ob text den Aufruf einer abgelösten
// Fassung trägt.
func hasLegacyInstructionsCommand(text string) bool {
	for _, block := range legacyInstructionsBlocks {
		if strings.Contains(text, block.command) {
			return true
		}
	}
	return false
}

// errAnstossEndeUnbestimmt meldet den einen Fall, in dem ein abgelöster Aufruf
// dasteht, das Ende des Blocks sich aber nicht bestimmen lässt.
//
// Ersetzt wird dann nichts. Eine Grenze zu raten hieße, bis zum Dateiende zu
// verwerfen — und damit Projekttext, den niemand zurückbekommt. Ein Hinweis ist
// die einzige ehrliche Antwort.
var errAnstossEndeUnbestimmt = errors.New("das Ende des veralteten Anstoßblocks in " +
	RootInstructionsFile + " ließ sich nicht bestimmen; die Datei bleibt unangetastet." +
	" Den Block bitte von Hand auf `" + InstalledCommandName + " briefing` umstellen.")

// RootInstructionsState ist der Zustand der Wurzeldatei.
type RootInstructionsState struct {
	Path string `json:"path"`
	// Present: die Datei existiert.
	Present bool `json:"present"`
	// HasMarker: der Anstoß steht bereits darin.
	HasMarker bool `json:"hasMarker"`
	// HasOutdatedAnstoss: der Anstoß steht darin, trägt aber noch einen
	// abgelösten Aufruf (`k-playbook context`, davor der Wrapper) — auch
	// dann, wenn sich sein Ende nicht bestimmen lässt und ein Lauf ihn
	// deshalb stehen lässt (Hint). Ob ein Lauf ihn ersetzt, sagt
	// AnstossRefreshable.
	HasOutdatedAnstoss bool `json:"hasOutdatedAnstoss,omitempty"`
	// AnstossRefreshable: das nächste Einrichten, der nächste Start oder
	// `k-playbook upgrade` ersetzt den veralteten Anstoß — sein Ende ist
	// bestimmt. Nie ohne HasOutdatedAnstoss.
	AnstossRefreshable bool `json:"anstossRefreshable,omitempty"`
	// Hint nennt, was sich nicht von selbst nachziehen lässt: einen
	// veralteten Anstoß, dessen Ende sich nicht bestimmen ließ. Leer, wenn
	// alles von selbst geht.
	Hint string `json:"hint,omitempty"`
}

// OK meldet, ob nichts mehr zu tun ist. CLAUDE.md gehört nicht dazu: die
// Include-Datei, die diese Datei per @AGENTS.md einbindet, läuft über den
// Link-Mechanismus in links.go und hat dort ihren eigenen Zustand.
func (s RootInstructionsState) OK() bool {
	return s.Present && s.HasMarker
}

// NeedsRefresh meldet, ob ein Lauf von Einrichten, Start oder Upgrade an der
// Datei etwas nachzöge — oder es versuchte und am Hinweis scheiterte.
func (s RootInstructionsState) NeedsRefresh() bool {
	return s.HasOutdatedAnstoss
}

// CheckRootInstructions prüft, ohne etwas zu verändern.
func CheckRootInstructions(projectDir string) RootInstructionsState {
	path := filepath.Join(projectDir, RootInstructionsFile)
	state := RootInstructionsState{Path: path}

	data, err := os.ReadFile(path)
	if err != nil {
		return state
	}
	state.Present = true
	content := strings.TrimRight(string(data), "\n")
	state.HasMarker = strings.Contains(content, instructionsMarker)

	if state.HasMarker {
		_, replaced, err := replaceOutdatedInstructionsBlock(content)
		// Ein unbestimmbares Blockende ändert nichts daran, dass der
		// abgelöste Aufruf dasteht — nur daran, dass er sich nicht von
		// selbst ersetzen lässt. Beides wird getrennt gemeldet, damit
		// `upgrade --check` sagt, was ein Lauf täte, und nicht mehr.
		state.HasOutdatedAnstoss = replaced || err != nil
		state.AnstossRefreshable = replaced
		if err != nil {
			state.Hint = err.Error()
		}
	}
	return state
}

// ApplyRootInstructions legt die Wurzeldatei an oder zieht sie nach.
//
// Eine vorhandene Datei wird nie überschrieben — sie gehört dem Projekt. Der
// Anstoß wird angehängt und per Marker gegen Dopplung geschützt; ein
// veralteter Anstoß wird ersetzt.
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
//
// Der zurückgegebene Fehler ist ein Hinweis, kein Abbruch: ein veralteter
// Anstoß steht da, lässt sich aber nicht abgrenzen. Er hält das Einrichten
// nicht auf — alles Übrige gehört trotzdem getan —, darf aber auch nicht
// untergehen.
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
		return CheckRootInstructions(projectDir), nil

	case err != nil:
		return CheckRootInstructions(projectDir), fmt.Errorf("%s lesen: %w", RootInstructionsFile, err)
	}

	content, changed, hint := refreshRootInstructions(strings.TrimRight(string(data), "\n"))
	if changed {
		if err := os.WriteFile(path, []byte(content+"\n"), 0o644); err != nil {
			return CheckRootInstructions(projectDir), fmt.Errorf("%s nachziehen: %w", RootInstructionsFile, err)
		}
	}
	return CheckRootInstructions(projectDir), hint
}

// refreshRootInstructions zieht den Inhalt einer vorhandenen AGENTS.md nach:
// ein veralteter Anstoß wird ersetzt, ein fehlender angehängt. Das zweite
// Ergebnis meldet, ob sich etwas geändert hat, das dritte den Hinweis auf
// einen veralteten Anstoß, dessen Ende sich nicht bestimmen ließ.
func refreshRootInstructions(content string) (string, bool, error) {
	if strings.Contains(content, instructionsMarker) {
		// Der Marker steht — bisher war das gleichbedeutend mit „nichts zu
		// tun". Ein Bestandsprojekt behielt damit dauerhaft den abgelösten
		// Aufruf, und der frühere Git-Update-Weg erreichte die Datei nicht:
		// sie liegt im Hauptverzeichnis, nicht im damaligen Clone.
		return replaceOutdatedInstructionsBlock(content)
	}

	block := strings.TrimRight(instructionsBlock(), "\n")
	if strings.TrimSpace(content) == "" {
		// Nichts davor — die Datei ist leer: dann beginnt sie mit dem
		// Anstoß, nicht mit einem leeren Absatz.
		return block, true, nil
	}
	return content + "\n\n" + block, true, nil
}

// RootInstructionsRepair nennt, was der Auffangweg beim Start an AGENTS.md
// getan hat. Beides zugleich ist unmöglich: ersetzt wird in einer vorhandenen
// Datei, angelegt wird eine fehlende.
type RootInstructionsRepair struct {
	// Refreshed: ein veralteter Anstoßblock wurde ersetzt.
	Refreshed bool
	// Created: die Datei fehlte ganz und wurde aus der Vorlage angelegt.
	Created bool
}

// Changed meldet, ob der Auffangweg geschrieben hat.
func (r RootInstructionsRepair) Changed() bool {
	return r.Refreshed || r.Created
}

// RepairRootInstructions ist der Auffangweg beim Start, das Gegenstück zu
// RepairMCP: Er zieht AGENTS.md auch dann nach, wenn niemand über die
// Oberfläche oder `k-playbook upgrade` aktualisiert hat.
//
// Genau zwei Fälle, beide idempotent:
//
//   - Die Datei steht und trägt den Anstoßmarker: ein Anstoßblock mit
//     abgelöstem Aufruf wird ersetzt — eine Reparatur an Inhalt, den
//     k-playbook selbst geschrieben hat, unabhängig davon, ob die Datei
//     versioniert ist. Das ist die Regel der Startwege
//     (keph:/docs/manual/assistenten-verlinkung.md).
//   - Die Datei fehlt ganz und die Einordnung des Paars CLAUDE.md/AGENTS.md
//     kennt weder Konflikt noch etwas zu verschieben: sie entsteht aus der
//     Vorlage (createRootInstructions). Es gibt dann nichts zu überschreiben
//     und keine Entscheidung zu übergehen.
//
// Alles Übrige bleibt dem Einrichten über ApplyRootInstructions und
// `k-playbook upgrade`: eine vorhandene Datei ohne Anstoßmarker wird nicht
// angefasst, eine echte CLAUDE.md nicht umbenannt, kein opencode.json
// angefasst.
func RepairRootInstructions(projectDir string) (RootInstructionsRepair, error) {
	path := filepath.Join(projectDir, RootInstructionsFile)

	data, err := os.ReadFile(path)
	switch {
	case err != nil && os.IsNotExist(err):
		return createRootInstructions(projectDir)

	case err != nil:
		// Nicht lesbar heißt: nichts zu reparieren und nichts anzulegen.
		return RootInstructionsRepair{}, nil
	}

	replaced, ok, hint := replaceOutdatedInstructionsBlock(strings.TrimRight(string(data), "\n"))
	if !ok {
		return RootInstructionsRepair{}, hint
	}
	if err := os.WriteFile(path, []byte(replaced+"\n"), 0o644); err != nil {
		return RootInstructionsRepair{}, fmt.Errorf("%s auffrischen: %w", RootInstructionsFile, err)
	}
	return RootInstructionsRepair{Refreshed: true}, nil
}

// createRootInstructions legt eine ganz fehlende AGENTS.md aus der Vorlage an
// — nur die Datei, nichts weiter: kein Eintrag in opencode.json, keine
// Umbenennung, kein entfernter Link. Das bleibt dem Einrichten.
//
// Die Einordnung kommt aus instructions_layout.go und wird nicht nachgebaut.
// Angelegt wird nur, wenn die Fallmatrix nichts anderes vorsieht: ein Konflikt
// (etwa CLAUDE.md als Link auf ein fremdes Ziel) und eine Blockade bleiben
// liegen und werden nur gemeldet; steht eine echte CLAUDE.md, gehört die
// Umbenennung dem Knopf — eine Vorlage daneben ergäbe zwei echte Dateien und
// damit den Konflikt aus Zeile 11; ein Symlink an AGENTS.md ist erst zu
// entfernen und heißt ebenfalls Knopf.
//
// O_EXCL statt WriteFile: ein Rest-Link an AGENTS.md ist per ReadFile
// „nicht vorhanden", WriteFile folgte ihm aber an sein totes Ziel. So entsteht
// die Datei nur dort, wo wirklich nichts liegt.
func createRootInstructions(projectDir string) (RootInstructionsRepair, error) {
	plan := classifyInstructions(projectDir)
	if plan.conflict || plan.blocked || plan.rename || plan.removeAgentsLink || !plan.mayCreate {
		return RootInstructionsRepair{}, nil
	}

	path := filepath.Join(projectDir, RootInstructionsFile)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return RootInstructionsRepair{}, nil
		}
		return RootInstructionsRepair{}, fmt.Errorf("%s anlegen: %w", RootInstructionsFile, err)
	}
	_, writeErr := file.WriteString(rootInstructionsTemplate())
	if err := errors.Join(writeErr, file.Close()); err != nil {
		return RootInstructionsRepair{}, fmt.Errorf("%s anlegen: %w", RootInstructionsFile, err)
	}
	return RootInstructionsRepair{Created: true}, nil
}

// replaceOutdatedInstructionsBlock ersetzt den Anstoßblock durch die aktuelle
// Fassung, wenn er noch einen abgelösten Aufruf trägt. Das zweite Ergebnis
// meldet, ob etwas ersetzt wurde, das dritte den Fall aus
// errAnstossEndeUnbestimmt: ein abgelöster Aufruf steht da, das Ende des
// Blocks ließ sich aber nicht bestimmen.
func replaceOutdatedInstructionsBlock(content string) (string, bool, error) {
	lines := strings.Split(content, "\n")
	start := instructionsBlockStart(lines)
	if start < 0 {
		return content, false, nil
	}

	end, bounded := instructionsBlockEnd(lines, start)
	if !bounded {
		// Ohne bestimmtes Ende wird nichts ersetzt. Ob überhaupt etwas zu tun
		// wäre, entscheidet der Rest der Datei ab dem Marker: steht ein
		// abgelöster Aufruf dort, bekommt der Nutzer einen Hinweis.
		if hasLegacyInstructionsCommand(strings.Join(lines[start:], "\n")) {
			return content, false, errAnstossEndeUnbestimmt
		}
		return content, false, nil
	}
	if !hasLegacyInstructionsCommand(strings.Join(lines[start:end], "\n")) {
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
// Erstens die bekannte Gestalt. Eine abgelöste Fassung ist ein endlicher,
// längst ausgelieferter Textbaustein; steht sie ab dem Marker Zeile für Zeile
// da, ist ihr Ende exakt bekannt, gleichgültig was dahinter folgt.
//
// Zweitens die Abgrenzung am Folgeabschnitt. Der Block trägt genau einen Marker
// und genau eine Überschrift; die nächste Überschrift oder der nächste
// HTML-Kommentar beginnt bereits etwas anderes. Leerzeilen davor bleiben
// außerhalb, damit die Trennung zum Folgeabschnitt beim Ersetzen erhalten
// bleibt.
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

// knownInstructionsBlockEnd meldet das Ende, wenn ab start wörtlich eine
// abgelöste Fassung des Anstoßblocks steht.
func knownInstructionsBlockEnd(lines []string, start int) (int, bool) {
	for _, block := range legacyInstructionsBlocks {
		if end, ok := knownBlockEnd(lines, start, block.lines, block.command); ok {
			return end, true
		}
	}
	return 0, false
}

// knownBlockEnd meldet das Ende, wenn ab start wörtlich die Zeilen want
// stehen.
//
// Verglichen wird zeilenweise ohne nachlaufende Leerzeichen — die tilgt mancher
// Editor beim Speichern. Eine Zeile legacyInstructionsCommandLine zählt, sobald
// sie command enthält; alle übrigen Zeilen müssen wörtlich stimmen.
func knownBlockEnd(lines []string, start int, want []string, command string) (int, bool) {
	end := start + len(want)
	if end > len(lines) {
		return 0, false
	}
	for offset, wantLine := range want {
		got := strings.TrimRight(lines[start+offset], " \t")
		if wantLine == legacyInstructionsCommandLine {
			if !strings.Contains(got, command) {
				return 0, false
			}
			continue
		}
		if got != wantLine {
			return 0, false
		}
	}
	return end, true
}

// rootInstructionsTemplate ist die Vorlage einer neuen AGENTS.md: die
// Überschrift und der Anstoßblock, sonst nichts — genau die AGENTS.md des
// Quell-Repos. Alles Weitere, was eine Sitzung wissen muss, kommt mit dem
// Briefing.
func rootInstructionsTemplate() string {
	return "# AGENTS.md\n\n" + instructionsBlock()
}

// instructionsBlock ist der Anstoß: der eine Aufruf, mit dem eine Sitzung
// alles bekommt, was für das Projekt gilt — das Briefing. Zuerst der
// Shell-Befehl, eingerückt als Codeblock, dann das gleichwertige MCP-Werkzeug:
// in dieser Reihenfolge wird der Block befolgt (gemessen 2026-10-18),
// umgekehrt nicht.
//
// Er nennt bewusst keine Verzeichnisebenen: dieselbe Datei liegt im Projekt
// und im Entwicklungsrepo, und ein Verweis auf eine Ebene wäre an einem
// dieser Orte falsch. Was es zu lesen gibt, sagt das Briefing.
//
// Aufgerufen wird das einmal je Host oder DevContainer installierte
// k-playbook unter seinem Namen. Ein projektrelativer Pfad wäre doppelt falsch:
// er zeigte auf den abgelösten Wrapper, und er hinge daran, aus welchem
// Verzeichnis der Assistent gestartet wurde.
//
// Die Zielgestalt ist die AGENTS.md des Quell-Repos; ein Test hält beide
// gleich.
func instructionsBlock() string {
	return instructionsMarker + `
## k-playbook

Für dieses Projekt gilt k-playbook. Führe zu Beginn, bevor du arbeitest, diesen
Befehl aus und lies seine Ausgabe:

    ` + InstalledCommandName + ` briefing

Gleichwertig ist das MCP-Werkzeug ` + "`k_playbook_briefing`" + ` mit ` + "`projectDir`" + ` gleich
diesem Verzeichnis. Die Ausgabe beginnt mit den Anweisungen, die in jeder Sitzung
gelten; darauf folgen die Regeln, Reviews und Leitlinien dieses Projekts als
Verweise, deine offene Arbeit und die Umgebung. Mehr ist zu Beginn nicht zu lesen.
Was du sonst brauchst, erreichst du über die Verweise darin.
`
}
