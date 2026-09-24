package branches

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Process ist ein laufender Prozess, soweit die Sitzungsprüfung ihn braucht.
type Process struct {
	PID     int
	PPID    int
	Cwd     string
	Cmdline []string
}

// ProcessSource liefert die laufenden Prozesse. Austauschbar, damit die
// Sitzungserkennung ohne echte Prozesse prüfbar ist.
type ProcessSource interface {
	Processes() ([]Process, error)
}

// ErrNoProcessSource heißt: auf diesem System gibt es keine Prozessquelle. Die
// Sitzungsprüfung ist dann nicht prüfbar, und ohne sie wird nicht umgeschaltet.
var ErrNoProcessSource = errors.New("keine Prozessquelle")

// ProcSource liest /proc. Unter Linux und WSL sieht sie jeden Prozess des
// eigenen Nutzers samt Arbeitsverzeichnis; fremde Prozesse lassen ihr cwd nicht
// lesen und fallen heraus. Unter macOS gibt es kein /proc.
type ProcSource struct {
	// Root ist das Verzeichnis, sonst /proc.
	Root string
}

func (source ProcSource) root() string {
	if source.Root != "" {
		return source.Root
	}
	return "/proc"
}

// Processes liest alle lesbaren Prozesse.
func (source ProcSource) Processes() ([]Process, error) {
	root := source.root()
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNoProcessSource
		}
		return nil, err
	}
	if _, err := os.Stat(filepath.Join(root, "self")); err != nil && source.Root == "" {
		return nil, ErrNoProcessSource
	}
	processes := []Process{}
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		cwd, err := os.Readlink(filepath.Join(dir, "cwd"))
		if err != nil {
			continue
		}
		raw, _ := os.ReadFile(filepath.Join(dir, "cmdline"))
		process := Process{PID: pid, Cwd: strings.TrimSuffix(cwd, " (deleted)")}
		for _, arg := range strings.Split(strings.TrimRight(string(raw), "\x00"), "\x00") {
			if arg != "" {
				process.Cmdline = append(process.Cmdline, arg)
			}
		}
		if stat, err := os.ReadFile(filepath.Join(dir, "stat")); err == nil {
			process.PPID = parentFromStat(string(stat))
		}
		processes = append(processes, process)
	}
	return processes, nil
}

// parentFromStat liest die PPID aus /proc/<pid>/stat. Der Name steht in
// Klammern und darf selbst Leerzeichen und Klammern enthalten; gezählt wird ab
// der letzten schließenden Klammer.
func parentFromStat(stat string) int {
	end := strings.LastIndex(stat, ")")
	if end < 0 {
		return 0
	}
	fields := strings.Fields(stat[end+1:])
	if len(fields) < 2 {
		return 0
	}
	ppid, _ := strconv.Atoi(fields[1])
	return ppid
}

// Arten erkannter Prozesse.
const (
	KindClaude       = "Claude Code"
	KindClaudeVSCode = "Claude Code (Erweiterung im VS-Code-Server)"
	KindOpenCode     = "opencode"
	KindCursor       = "Cursor"
	KindCodex        = "Codex"
	KindMCP          = "MCP-Server"
	KindVSCode       = "VS-Code-Server"
	KindOther        = "anderer Prozess"
)

// SessionProcess ist ein gefundener Prozess mit Art, PID und Verzeichnis.
type SessionProcess struct {
	Kind    string `json:"kind"`
	PID     int    `json:"pid"`
	Cwd     string `json:"cwd"`
	Command string `json:"command"`
	// Blocking: eine KI-Sitzung, ein MCP-Server oder ein Prozess, den eine
	// solche Sitzung gestartet hat.
	Blocking bool `json:"blocking"`
	// Session nennt bei einem Kindprozess die Sitzung, zu der er gehört.
	Session int `json:"session,omitempty"`
}

var mcpToken = regexp.MustCompile(`(^|[/@_.-])mcp([/@_.:-]|$)`)

// classifyProcess ordnet einen Prozess einer Art zu. Die Reihenfolge zählt:
// die Claude-Code-Erweiterung läuft unter dem VS-Code-Server und muss vor ihm
// erkannt werden, ein MCP-Server von Claude oder opencode als das, was er ist.
func classifyProcess(process Process) string {
	lower := strings.ToLower(strings.Join(process.Cmdline, " "))
	base := ""
	if len(process.Cmdline) > 0 {
		base = strings.ToLower(filepath.Base(process.Cmdline[0]))
	}
	for _, arg := range process.Cmdline {
		if mcpToken.MatchString(strings.ToLower(arg)) {
			return KindMCP
		}
	}
	switch {
	case strings.Contains(lower, "anthropic.claude-code"):
		return KindClaudeVSCode
	case base == "claude" || strings.Contains(lower, "@anthropic-ai/claude-code") || strings.Contains(lower, "/claude "):
		return KindClaude
	case base == "opencode" || strings.Contains(lower, "opencode-ai") || strings.Contains(lower, "/opencode "):
		return KindOpenCode
	case base == "codex" || strings.Contains(lower, "@openai/codex"):
		return KindCodex
	case base == "cursor" || strings.Contains(lower, ".cursor-server") || strings.Contains(lower, "cursor-agent"):
		return KindCursor
	case strings.Contains(lower, ".vscode-server") || strings.Contains(lower, "vscode-server"):
		return KindVSCode
	}
	return KindOther
}

// blockingKind meldet, ob eine Art das Umschalten verhindert: KI-Sitzungen und
// MCP-Server, die das Verzeichnis ihres Clients erben. Ein Editorfenster und
// andere Prozesse sind Hinweise.
func blockingKind(kind string) bool {
	switch kind {
	case KindClaude, KindClaudeVSCode, KindOpenCode, KindCursor, KindCodex, KindMCP:
		return true
	}
	return false
}

// maxAncestors begrenzt den Weg über die Elternprozesse. Ein Zyklus kommt in
// /proc nicht vor, ein kaputter Eintrag soll die Prüfung trotzdem nicht festhalten.
const maxAncestors = 32

// findSessions sucht Prozesse, deren Arbeitsverzeichnis in einem der
// Verzeichnisse liegt, gleich oder darunter.
//
// Ausgenommen sind der eigene Prozess und was er gestartet hat, soweit es selbst
// keine KI-Sitzung und kein MCP-Server ist und nicht von einer solchen stammt —
// die git-Aufrufe der Prüfung. Eine Sitzung unter dem Server blockiert wie jede
// andere: heute startet der Chat keine, er spricht per HTTP mit einem
// opencode-Dienst; ein späterer Chat oder ein MCP-Server, den die Oberfläche zur
// Probe startet, sollen aber nicht unsichtbar sein, nur weil sie Kinder des
// Servers sind.
//
// Ein Prozess, den eine KI-Sitzung gestartet hat, zählt zu ihr: ein MCP-Server
// wie confluence-companion trägt keinen erkennbaren Namen, wohl aber einen
// Elternprozess claude, und die Shell eines Werkzeugaufrufs ebenso.
func findSessions(source ProcessSource, ownPID int, dirs []string) ([]SessionProcess, error) {
	if source == nil {
		source = ProcSource{}
	}
	processes, err := source.Processes()
	if err != nil {
		return nil, err
	}
	roots := []string{}
	for _, dir := range dirs {
		if dir = resolvedPath(dir); dir != "" && !contains(roots, dir) {
			roots = append(roots, dir)
		}
	}
	byPID := map[int]Process{}
	for _, process := range processes {
		byPID[process.PID] = process
	}
	// ancestor liefert den nächsten Vorfahren, für den match gilt.
	ancestor := func(process Process, match func(Process) bool) (Process, bool) {
		current := process
		for step := 0; step < maxAncestors && current.PPID > 0; step++ {
			parent, ok := byPID[current.PPID]
			if !ok {
				return Process{}, false
			}
			if match(parent) {
				return parent, true
			}
			current = parent
		}
		return Process{}, false
	}

	// ownHelper meldet einen Nachkommen des eigenen Prozesses, der keine
	// Sitzung ist und auf dem Weg hinauf zum eigenen Prozess keine Sitzung
	// als Vorfahren hat.
	ownHelper := func(process Process) bool {
		if blockingKind(classifyProcess(process)) {
			return false
		}
		sessionBetween := false
		_, own := ancestor(process, func(p Process) bool {
			if p.PID == ownPID {
				return true
			}
			if blockingKind(classifyProcess(p)) {
				sessionBetween = true
			}
			return false
		})
		return own && !sessionBetween
	}

	found := []SessionProcess{}
	for _, process := range processes {
		if ownPID > 0 && (process.PID == ownPID || ownHelper(process)) {
			continue
		}
		cwd := filepath.Clean(process.Cwd)
		inside := false
		for _, root := range roots {
			if cwd == root || strings.HasPrefix(cwd, root+string(filepath.Separator)) {
				inside = true
				break
			}
		}
		if !inside {
			continue
		}
		command := strings.Join(strings.Fields(strings.Join(process.Cmdline, " ")), " ")
		if len(command) > 160 {
			command = command[:157] + "..."
		}
		entry := SessionProcess{Kind: classifyProcess(process), PID: process.PID, Cwd: cwd, Command: command}
		entry.Blocking = blockingKind(entry.Kind)
		if !entry.Blocking {
			if session, ok := ancestor(process, func(p Process) bool { return blockingKind(classifyProcess(p)) }); ok {
				entry.Kind = "gestartet von " + classifyProcess(session)
				entry.Blocking = true
				entry.Session = session.PID
			}
		}
		found = append(found, entry)
	}
	sort.Slice(found, func(i, j int) bool { return found[i].PID < found[j].PID })
	return found, nil
}
