package project

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ClaudeInstructionsFile ist der Symlink auf RootInstructionsFile. Claude Code
// liest ausschließlich diese Datei, OpenCode bevorzugt AGENTS.md. Die Richtung
// CLAUDE.md -> AGENTS.md ist deshalb fest und keine projektabhängige Variable:
// ein umgedrehter Link müsste in Prüfung, Oberfläche und Doku dauerhaft
// mitgedacht werden.
const ClaudeInstructionsFile = "CLAUDE.md"

// checkIgnoreTimeout begrenzt den einen git-Aufruf des Ignoriert-Vorbehalts. Er
// ist rein lokal; wer länger braucht, antwortet nicht mehr sinnvoll.
const checkIgnoreTimeout = 10 * time.Second

// conflictHint steht in jedem Konflikttext. Ein gemeldeter Konflikt ist kein
// Schönheitsfehler: solange er steht, sieht Claude Code vom Einrichten nichts,
// weil er ausschließlich CLAUDE.md liest.
const conflictHint = "Bis das von Hand aufgelöst ist, sieht Claude Code den Anstoß nicht."

// pathKind ist der Zustand einer der beiden Instruktionsdateien, abgelesen mit
// Lstat und Readlink.
//
// os.Stat taugt dafür nicht: es folgt Symlinks. Die verdrehte Richtung —
// AGENTS.md als Symlink auf CLAUDE.md — bliebe damit unsichtbar, weil AGENTS.md
// durch den Link hindurch als vorhanden gilt.
type pathKind int

const (
	// kindMissing: nichts vorhanden.
	kindMissing pathKind = iota
	// kindRegular: eine reguläre Datei.
	kindRegular
	// kindDir: ein Verzeichnis.
	kindDir
	// kindLinkToOther: ein Symlink auf die jeweils andere der beiden Dateien.
	// Der Zustand hat Vorrang vor kindLinkDangling: ein Link auf eine fehlende
	// Gegendatei bleibt die verdrehte Richtung und ist kein Rest.
	kindLinkToOther
	// kindLinkForeign: ein Symlink auf ein fremdes, vorhandenes Ziel.
	kindLinkForeign
	// kindLinkDangling: ein Rest-Link — fremdes Ziel, per Lstat nicht erreichbar.
	kindLinkDangling
	// kindUnreadable: Lstat oder Readlink sind fehlgeschlagen.
	kindUnreadable
	// kindOther: etwas, das weder reguläre Datei noch Verzeichnis noch Symlink
	// ist — Socket, FIFO, Gerät.
	kindOther
)

// pathState ist der Zustand einer der beiden Dateien samt der Angaben, die die
// Detailtexte brauchen.
type pathState struct {
	kind pathKind
	// link ist der Rohwert aus Readlink — nur für die Anzeige. Verglichen wird
	// nie damit, sondern mit dem aufgelösten Ziel.
	link string
	// mode steht nur bei kindOther und benennt, was dort liegt.
	mode os.FileMode
	// err steht nur bei kindUnreadable.
	err error
}

// classifyPath ordnet eine der beiden Dateien ein. other ist die jeweils
// andere; ein Link darauf zählt als verdrehte Richtung, auch wenn das Ziel
// fehlt.
func classifyPath(projectDir string, name string, other string) pathState {
	path := filepath.Join(projectDir, name)

	info, err := os.Lstat(path)
	switch {
	case err != nil && os.IsNotExist(err):
		return pathState{kind: kindMissing}

	case err != nil:
		return pathState{kind: kindUnreadable, err: err}

	case info.Mode()&os.ModeSymlink != 0:
		return classifyLink(projectDir, path, other)

	case info.IsDir():
		return pathState{kind: kindDir}

	case info.Mode().IsRegular():
		return pathState{kind: kindRegular}

	default:
		return pathState{kind: kindOther, mode: info.Mode()}
	}
}

// classifyLink ordnet einen Symlink ein.
//
// Verglichen wird das über filepath.Clean relativ zum Verzeichnis des Links
// aufgelöste Ziel, nie der Rohstring: AGENTS.md, ./AGENTS.md und ein absoluter
// Pfad auf dieselbe Datei sind derselbe Zustand. Am Rohstring gemessen fiele
// ./CLAUDE.md in den Fremdlink-Zweig und bliebe dauerhaft blockiert.
func classifyLink(projectDir string, path string, other string) pathState {
	destination, err := os.Readlink(path)
	if err != nil {
		return pathState{kind: kindUnreadable, err: err}
	}

	resolved := destination
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(filepath.Dir(path), resolved)
	}
	resolved = filepath.Clean(resolved)

	state := pathState{link: destination}
	switch {
	case resolved == filepath.Clean(filepath.Join(projectDir, other)):
		state.kind = kindLinkToOther

	// Ein Symlink zählt nur dann als Fremdlink, wenn sein Ziel existiert. Ein
	// Link ins Leere ist Rest, kein Inhalt.
	case linkTargetExists(resolved):
		state.kind = kindLinkForeign

	default:
		state.kind = kindLinkDangling
	}
	return state
}

func linkTargetExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// instructionsPlan ist die Zeile der Fallmatrix, die auf die Ausgangslage
// passt, samt den Folgerungen daraus.
type instructionsPlan struct {
	// matched ist die erste passende Zeile der Fallmatrix.
	matched int
	// row ist die Zeile, die die Aktion bestimmt hat. Die Zeilen 6 und 7 ordnen
	// nach dem Entfernen ein zweites Mal ein; dort steht die Zeile des zweiten
	// Durchgangs, sonst dieselbe wie in matched.
	row int
	// removeAgentsLink: der Symlink an AGENTS.md wird entfernt (Zeilen 5, 6, 7).
	removeAgentsLink bool
	// removeReason benennt den entfernten Link im Ergebnistext.
	removeReason string
	// rename: CLAUDE.md wird nach AGENTS.md umbenannt (Zeilen 5, 10).
	rename bool
	// conflict: nicht auflösbar — Zeilen 4, 8, 11, 17 und der
	// Ignoriert-Vorbehalt. Es wird nichts verschoben, nichts gelöscht, nichts
	// gesichert und kein AGENTS.md angelegt.
	conflict bool
	// blocked: etwas steht im Weg (Zeilen 1, 2, 3).
	blocked bool
	// mayCreate: ein fehlendes AGENTS.md darf aus der Vorlage angelegt werden.
	mayCreate bool
	// runInstructions: ApplyRootInstructions darf überhaupt laufen. Zeile 2
	// nicht — os.ReadFile liefe an einem Verzeichnis in den Fehlerzweig.
	runInstructions bool
	// detail beschreibt Konflikt oder Blockade im Klartext.
	detail string
}

// classifyInstructions ordnet das Paar (CLAUDE.md, AGENTS.md) ein, ohne etwas zu
// verändern. Prüfung und Einrichten benutzen dieselbe Klassifikation; zwei
// Implementierungen liefen auseinander.
func classifyInstructions(projectDir string) instructionsPlan {
	claude := classifyPath(projectDir, ClaudeInstructionsFile, RootInstructionsFile)
	agents := classifyPath(projectDir, RootInstructionsFile, ClaudeInstructionsFile)
	return planInstructions(projectDir, claude, agents)
}

// planInstructions ordnet ein Paar ein und hält fest, welche Zeile zuerst
// gepasst hat.
func planInstructions(projectDir string, claude pathState, agents pathState) instructionsPlan {
	plan := matchInstructions(projectDir, claude, agents)
	if plan.matched == 0 {
		plan.matched = plan.row
	}
	return plan
}

// matchInstructions geht die Fallmatrix von oben nach unten durch; die erste
// passende Zeile gewinnt. Damit ist sie disjunkt, und Zeile 17 fängt alles
// Übrige auf — kein Zustand fällt durch.
func matchInstructions(projectDir string, claude pathState, agents pathState) instructionsPlan {
	switch {
	// 1 — eine der beiden unlesbar: nichts anfassen, nichts anlegen.
	case claude.kind == kindUnreadable || agents.kind == kindUnreadable:
		return instructionsPlan{
			row: 1, blocked: true, runInstructions: true,
			detail: unreadableDetail(claude, agents),
		}

	// 2 — AGENTS.md ist ein Verzeichnis. ApplyRootInstructions liefe dort in
	// den Lesefehler; die Katalog-Links werden trotzdem gesetzt.
	case agents.kind == kindDir:
		return instructionsPlan{
			row: 2, blocked: true,
			detail: RootInstructionsFile + " ist ein Verzeichnis",
		}

	// 3 — CLAUDE.md ist ein Verzeichnis; AGENTS.md wird normal behandelt.
	case claude.kind == kindDir:
		return instructionsPlan{
			row: 3, blocked: true, mayCreate: true, runInstructions: true,
			detail: "Verzeichnis steht im Weg",
		}

	// 4 — CLAUDE.md zeigt auf ein fremdes, vorhandenes Ziel. Würde der Link
	// umgebogen, wären die bisher gelesenen Instruktionen ab dann unwirksam.
	case claude.kind == kindLinkForeign:
		return conflictPlan(4, foreignClaudeDetail(claude))

	// 5 — verdrehte Richtung, der Inhalt steht in CLAUDE.md.
	case agents.kind == kindLinkToOther && claude.kind == kindRegular:
		return renamePlan(projectDir, 5, verdrehtReason())

	// 6 — verdrehte Richtung, am Ziel steht kein echter Inhalt.
	case agents.kind == kindLinkToOther &&
		(claude.kind == kindMissing || claude.kind == kindLinkDangling || claude.kind == kindLinkToOther):
		return afterRemovingAgentsLink(projectDir, 6, claude, verdrehtReason())

	// 7 — AGENTS.md ist ein Rest-Link. Bliebe er stehen, legte das Einrichten
	// die Datei an seinem toten Ziel an.
	case agents.kind == kindLinkDangling:
		return afterRemovingAgentsLink(projectDir, 7, claude, restLinkReason())

	// 8 — AGENTS.md ist bewusst verlinkt, CLAUDE.md hat daneben eigenen Inhalt.
	case agents.kind == kindLinkForeign && claude.kind == kindRegular:
		return conflictPlan(8, foreignAgentsDetail(agents))

	// 9 — AGENTS.md ist bewusst verlinkt, an CLAUDE.md steht kein eigener
	// Inhalt. Das ist eine Entscheidung des Projekts, kein Fehler.
	case agents.kind == kindLinkForeign &&
		(claude.kind == kindMissing || claude.kind == kindLinkToOther || claude.kind == kindLinkDangling):
		return plainPlan(9)

	// 10 — nur CLAUDE.md: umbenennen, damit der Inhalt erhalten bleibt.
	case claude.kind == kindRegular && agents.kind == kindMissing:
		return renamePlan(projectDir, 10, "")

	// 11 — beide echte Dateien, zwei Ursachen mit verschiedenen Auflösungen.
	case claude.kind == kindRegular && agents.kind == kindRegular:
		return conflictPlan(11, bothRealDetail())

	// 12 — beides fehlt.
	case claude.kind == kindMissing && agents.kind == kindMissing:
		return plainPlan(12)

	// 13 — CLAUDE.md fehlt, AGENTS.md ist eine echte Datei.
	case claude.kind == kindMissing && agents.kind == kindRegular:
		return plainPlan(13)

	// 14 — der Sollzustand.
	case claude.kind == kindLinkToOther && agents.kind == kindRegular:
		return plainPlan(14)

	// 15 — CLAUDE.md zeigt auf ein noch fehlendes AGENTS.md.
	case claude.kind == kindLinkToOther && agents.kind == kindMissing:
		return plainPlan(15)

	// 16 — CLAUDE.md ist ein Rest-Link und wird neu gesetzt.
	case claude.kind == kindLinkDangling && (agents.kind == kindMissing || agents.kind == kindRegular):
		return plainPlan(16)

	// 17 — Auffangzweig: an einer der beiden steht etwas, das sich nicht
	// automatisch auflösen lässt.
	default:
		return conflictPlan(17, pairDetail(claude, agents))
	}
}

// afterRemovingAgentsLink ordnet die Zeilen 6 und 7 nach dem Entfernen des
// Symlinks ein zweites Mal ein.
//
// Das terminiert: nach dem Entfernen fehlt AGENTS.md sicher, und keine Zeile,
// die dann greifen kann, entfernt noch etwas. Endet der zweite Durchgang in
// einem Konflikt, wird auch nicht entfernt — im Konfliktfall wird nichts
// gelöscht.
func afterRemovingAgentsLink(projectDir string, matched int, claude pathState, reason string) instructionsPlan {
	plan := planInstructions(projectDir, claude, pathState{kind: kindMissing})
	plan.matched = matched
	if plan.conflict || plan.blocked {
		return plan
	}

	plan.removeAgentsLink = true
	plan.removeReason = reason
	return plan
}

// renamePlan plant eine Umbenennung — sofern der Ignoriert-Vorbehalt nicht
// greift.
func renamePlan(projectDir string, row int, removeReason string) instructionsPlan {
	if agentsIgnored(projectDir) {
		return conflictPlan(row, ignoredDetail())
	}

	return instructionsPlan{
		row:              row,
		removeAgentsLink: removeReason != "",
		removeReason:     removeReason,
		rename:           true,
		mayCreate:        true,
		runInstructions:  true,
	}
}

func conflictPlan(row int, detail string) instructionsPlan {
	return instructionsPlan{row: row, conflict: true, runInstructions: true, detail: detail}
}

func plainPlan(row int) instructionsPlan {
	return instructionsPlan{row: row, mayCreate: true, runInstructions: true}
}

func verdrehtReason() string {
	return RootInstructionsFile + " zeigte auf " + ClaudeInstructionsFile + "; der Symlink ist entfernt."
}

func restLinkReason() string {
	return RootInstructionsFile + " war ein toter Symlink; er ist entfernt."
}

// InstructionsOutcome nennt, was das Einrichten an den Instruktionsdateien
// getan hat.
type InstructionsOutcome string

const (
	// InstructionsUnchanged: nichts zu tun.
	InstructionsUnchanged InstructionsOutcome = "unchanged"
	// InstructionsRenamed: CLAUDE.md wurde nach AGENTS.md umbenannt.
	InstructionsRenamed InstructionsOutcome = "renamed"
	// InstructionsCleared: ein irreführender Symlink an AGENTS.md wurde entfernt.
	InstructionsCleared InstructionsOutcome = "cleared"
	// InstructionsConflict: nicht auflösbar, nur gemeldet.
	InstructionsConflict InstructionsOutcome = "conflict"
	// InstructionsBlocked: etwas steht im Weg.
	InstructionsBlocked InstructionsOutcome = "blocked"
)

// InstructionsResult ist das Ergebnis des Einordnens und Auflösens. Der
// Aufrufer braucht beides: den Klartext für den Ergebnistext und mayCreate für
// den weiteren Ablauf.
type InstructionsResult struct {
	Outcome InstructionsOutcome `json:"outcome"`
	// Row ist die erste passende Zeile der Fallmatrix.
	Row int `json:"row"`
	// ResolvedRow ist die Zeile, die die Aktion bestimmt hat. Die Zeilen 6 und
	// 7 ordnen nach dem Entfernen des Symlinks ein zweites Mal ein; sonst
	// stimmen beide überein.
	ResolvedRow int `json:"resolvedRow"`
	// Detail beschreibt im Klartext, was geschehen ist oder warum nichts
	// geschehen konnte.
	Detail string `json:"detail"`
	// MayCreate: ein fehlendes AGENTS.md darf aus der Vorlage angelegt werden.
	MayCreate bool `json:"mayCreate"`
	// RemovedLink: der Symlink an AGENTS.md wurde entfernt.
	RemovedLink bool `json:"removedLink,omitempty"`
	// Renamed: CLAUDE.md wurde nach AGENTS.md umbenannt.
	Renamed bool `json:"renamed,omitempty"`
	// runInstructions: ApplyRootInstructions darf laufen.
	runInstructions bool
}

// resolveInstructions ordnet die Ausgangslage ein und löst auf, was sich
// auflösen lässt.
//
// Die Reihenfolge zählt: erst den irreführenden Symlink weg, dann umbenennen.
// Den neuen Symlink an CLAUDE.md setzt danach ApplyLinks.
func resolveInstructions(projectDir string) (InstructionsResult, error) {
	plan := classifyInstructions(projectDir)
	result := InstructionsResult{
		Row:             plan.matched,
		ResolvedRow:     plan.row,
		Detail:          plan.detail,
		MayCreate:       plan.mayCreate,
		runInstructions: plan.runInstructions,
	}

	if plan.removeAgentsLink {
		if err := os.Remove(filepath.Join(projectDir, RootInstructionsFile)); err != nil {
			result.MayCreate = false
			result.Outcome = outcomeOf(plan, result)
			return result, fmt.Errorf("Symlink %s entfernen: %w", RootInstructionsFile, err)
		}
		result.RemovedLink = true
	}

	if plan.rename {
		from := filepath.Join(projectDir, ClaudeInstructionsFile)
		to := filepath.Join(projectDir, RootInstructionsFile)
		if err := os.Rename(from, to); err != nil {
			// Ohne die Umbenennung stünde neben dem erhaltenen Inhalt sonst
			// gleich die zweite, fast leere Quelle aus der Vorlage.
			result.MayCreate = false
			result.Outcome = outcomeOf(plan, result)
			return result, fmt.Errorf("%s nach %s umbenennen: %w",
				ClaudeInstructionsFile, RootInstructionsFile, err)
		}
		result.Renamed = true
	}

	result.Outcome = outcomeOf(plan, result)
	result.Detail = resolvedDetail(plan, result)
	return result, nil
}

func outcomeOf(plan instructionsPlan, result InstructionsResult) InstructionsOutcome {
	switch {
	case plan.blocked:
		return InstructionsBlocked
	case plan.conflict:
		return InstructionsConflict
	case result.Renamed:
		return InstructionsRenamed
	case result.RemovedLink:
		return InstructionsCleared
	default:
		return InstructionsUnchanged
	}
}

// resolvedDetail formuliert, was tatsächlich geschehen ist. Der Text aus dem
// Plan bleibt stehen, wo nichts geschehen ist — dort beschreibt er den Grund.
func resolvedDetail(plan instructionsPlan, result InstructionsResult) string {
	parts := []string{}
	if result.RemovedLink {
		parts = append(parts, plan.removeReason)
	}
	if result.Renamed {
		parts = append(parts, ClaudeInstructionsFile+" ist nach "+RootInstructionsFile+
			" umbenannt und selbst wieder ein Symlink darauf.")
	}
	if len(parts) == 0 {
		return plan.detail
	}
	return strings.Join(parts, " ")
}

// agentsIgnored meldet, ob git AGENTS.md in diesem Projekt ignoriert.
//
// Nur dieser eine Fall blockiert die Umbenennung: sie nähme versionierten
// Inhalt still aus der Versionskontrolle, hinterher stünde er nur noch im
// Arbeitsverzeichnis. Ein Gate auf einen sauberen git-Zustand gibt es bewusst
// nicht — ein Arbeitsverzeichnis mit Änderungen ist der Normalfall.
//
// Der Aufruf läuft bewusst ohne --no-index: per Default zieht git den Index
// heran und meldet eine bereits getrackte Datei als nicht ignoriert. Genau das
// ist gewollt — ist AGENTS.md versioniert, geht durch die Umbenennung nichts
// verloren.
//
// Exit 0 heißt „ignoriert". Jeder andere Ausgang — 1, 128, kein git, kein
// Repository, Timeout — heißt „nicht ignoriert" und blockiert nicht.
func agentsIgnored(projectDir string) bool {
	config, err := ReadConfig(projectDir)
	if err != nil || config.VCS != "git" {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), checkIgnoreTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "check-ignore", "--quiet", RootInstructionsFile)
	cmd.Dir = projectDir
	return cmd.Run() == nil
}

func unreadableDetail(claude pathState, agents pathState) string {
	if claude.kind == kindUnreadable {
		return ClaudeInstructionsFile + " ist nicht lesbar: " + claude.err.Error()
	}
	return RootInstructionsFile + " ist nicht lesbar: " + agents.err.Error()
}

func foreignClaudeDetail(claude pathState) string {
	return ClaudeInstructionsFile + " zeigt auf " + claude.link +
		"; der Link bleibt stehen, sonst wären die dort gelesenen Instruktionen ab sofort unwirksam. " +
		RootInstructionsFile + " wird deshalb auch nicht angelegt. " + conflictHint
}

func foreignAgentsDetail(agents pathState) string {
	return RootInstructionsFile + " zeigt auf " + agents.link + ", daneben trägt " +
		ClaudeInstructionsFile + " eigenen Inhalt. Der Link gehört dem Projekt und bleibt stehen; " +
		"beide Quellen sind von Hand zusammenzuführen. " + conflictHint
}

func bothRealDetail() string {
	return ClaudeInstructionsFile + " und " + RootInstructionsFile + " sind zwei echte Dateien. " +
		"Entweder bringt das Projekt eine eigene " + ClaudeInstructionsFile + " mit — dann beide von Hand " +
		"zusammenführen und " + ClaudeInstructionsFile + " danach löschen. Oder ein Editor hat den Symlink " +
		"beim Speichern durch eine echte Datei ersetzt — dann " + ClaudeInstructionsFile + " löschen und neu " +
		"einrichten, der Inhalt steht bereits in " + RootInstructionsFile + ". " + conflictHint
}

func ignoredDetail() string {
	return RootInstructionsFile + " ist in diesem Projekt von git ignoriert; " + ClaudeInstructionsFile +
		" wird deshalb nicht umbenannt, sonst fiele der Inhalt still aus der Versionskontrolle. " +
		"Ausweg: die Ignore-Regel für " + RootInstructionsFile + " entfernen und neu einrichten, " +
		"oder den Inhalt von Hand nach " + RootInstructionsFile + " übernehmen. " + conflictHint
}

func pairDetail(claude pathState, agents pathState) string {
	return ClaudeInstructionsFile + ": " + describeKind(claude) + "; " +
		RootInstructionsFile + ": " + describeKind(agents) +
		". Das lässt sich nicht automatisch auflösen. " + conflictHint
}

func describeKind(state pathState) string {
	switch state.kind {
	case kindMissing:
		return "nicht vorhanden"
	case kindRegular:
		return "eine echte Datei"
	case kindDir:
		return "ein Verzeichnis"
	case kindLinkToOther, kindLinkForeign:
		return "ein Symlink auf " + state.link
	case kindLinkDangling:
		return "ein toter Symlink auf " + state.link
	case kindUnreadable:
		return "nicht lesbar"
	default:
		return "weder Datei noch Verzeichnis noch Symlink (" + state.mode.String() + ")"
	}
}
