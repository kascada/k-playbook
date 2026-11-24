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

// RootInstructionsState ist der Zustand der Wurzeldatei.
type RootInstructionsState struct {
	Path string `json:"path"`
	// Present: die Datei existiert.
	Present bool `json:"present"`
	// HasMarker: der Anstoß steht bereits darin.
	HasMarker bool `json:"hasMarker"`
}

// OK meldet, ob nichts mehr zu tun ist. CLAUDE.md gehört nicht dazu: die
// Include-Datei, die diese Datei per @AGENTS.md einbindet, läuft über den
// Link-Mechanismus in links.go und hat dort ihren eigenen Zustand.
func (s RootInstructionsState) OK() bool {
	return s.Present && s.HasMarker
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
	return state
}

// ApplyRootInstructions legt die Wurzeldatei an oder zieht sie nach.
//
// Eine vorhandene Datei wird nie überschrieben — sie gehört dem Projekt. Der
// Anstoß wird angehängt und per Marker gegen Dopplung geschützt; ein
// vorhandener Anstoß bleibt, wie er ist.
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
		return CheckRootInstructions(projectDir), nil

	case err != nil:
		return CheckRootInstructions(projectDir), fmt.Errorf("%s lesen: %w", RootInstructionsFile, err)
	}

	content, changed := refreshRootInstructions(strings.TrimRight(string(data), "\n"))
	if changed {
		if err := os.WriteFile(path, []byte(content+"\n"), 0o644); err != nil {
			return CheckRootInstructions(projectDir), fmt.Errorf("%s nachziehen: %w", RootInstructionsFile, err)
		}
	}
	return CheckRootInstructions(projectDir), nil
}

// refreshRootInstructions zieht den Inhalt einer vorhandenen AGENTS.md nach:
// ein fehlender Anstoß wird angehängt, ein vorhandener bleibt. Das zweite
// Ergebnis meldet, ob sich etwas geändert hat.
func refreshRootInstructions(content string) (string, bool) {
	if strings.Contains(content, instructionsMarker) {
		return content, false
	}

	block := strings.TrimRight(instructionsBlock(), "\n")
	if strings.TrimSpace(content) == "" {
		// Nichts davor — die Datei ist leer: dann beginnt sie mit dem
		// Anstoß, nicht mit einem leeren Absatz.
		return block, true
	}
	return content + "\n\n" + block, true
}

// RootInstructionsRepair nennt, was der Auffangweg beim Start an AGENTS.md
// getan hat.
type RootInstructionsRepair struct {
	// Created: die Datei fehlte ganz und wurde aus der Vorlage angelegt.
	Created bool
}

// Changed meldet, ob der Auffangweg geschrieben hat.
func (r RootInstructionsRepair) Changed() bool {
	return r.Created
}

// RepairRootInstructions ist der Auffangweg beim Start, das Gegenstück zu
// RepairMCP: Er zieht AGENTS.md auch dann nach, wenn niemand über die
// Oberfläche oder `k-playbook upgrade` aktualisiert hat.
//
// Genau ein Fall, idempotent: Die Datei fehlt ganz und die Einordnung des
// Paars CLAUDE.md/AGENTS.md kennt weder Konflikt noch etwas zu verschieben:
// sie entsteht aus der Vorlage (createRootInstructions). Es gibt dann nichts
// zu überschreiben und keine Entscheidung zu übergehen.
//
// Alles Übrige bleibt dem Einrichten über ApplyRootInstructions und
// `k-playbook upgrade`: eine vorhandene Datei wird nicht angefasst, eine echte CLAUDE.md nicht umbenannt, kein opencode.json
// angefasst.
func RepairRootInstructions(projectDir string) (RootInstructionsRepair, error) {
	path := filepath.Join(projectDir, RootInstructionsFile)

	if _, err := os.ReadFile(path); err != nil && os.IsNotExist(err) {
		return createRootInstructions(projectDir)
	}
	// Vorhanden oder nicht lesbar heißt: nichts anzulegen.
	return RootInstructionsRepair{}, nil
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
// k-playbook unter seinem Namen. Ein projektrelativer Pfad wäre falsch: er
// hinge daran, aus welchem Verzeichnis der Assistent gestartet wurde.
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
