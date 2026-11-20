package worklock

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Der Halter-Prozess und die Frage „verwaist?“.
//
// Gemessen (Claude Code 2.1.328, OpenCode 2.3.0, Linux/WSL):
//
//   - Claude Code startet je Sitzung einen eigenen Prozess `claude`, und der
//     startet `k-playbook mcp` als Kind; die Sub-Agenten der Sitzung laufen im
//     selben Prozess und rufen denselben MCP-Server. Stirbt der MCP-Server,
//     startet Claude Code beim nächsten Werkzeugaufruf einen neuen — mit neuer
//     PID, unter demselben Prozess `claude`. Der MCP-Server wird also innerhalb
//     einer lebenden Sitzung ersetzt und taugt nicht als Halter; der Prozess
//     `claude` wird es nicht.
//   - OpenCode 2 hat je Benutzer einen Dienst `opencode serve --service`; auch
//     `opencode run` hängt sich an ihn. Er startet die MCP-Server, je Projekt
//     einen, den sich alle Sitzungen teilen, und `opencode reload` (nach
//     /k-upgrade) ersetzt sie, während Dienst und Sitzungen weiterlaufen. Der
//     Dienst lebt länger als jede Sitzung: Ob eine einzelne Sitzung zu Ende
//     ist, lässt sich an keinem Prozess ablesen.
//
// Deshalb ist der Halter der Prozess des Clients über dem MCP-Server, nicht
// der MCP-Server selbst, und sicher verwaist ist eine Sperre nur, wenn dieser
// Prozess ein Sitzungsprozess ist (Claude Code), auf demselben Rechner seit
// demselben Systemstart und im selben PID-Namensraum lief und PID samt
// Startzeit nicht mehr stimmen. Mit ihm endet alles, was die Sitzung gerade
// tat, Sub-Agenten eingeschlossen; setzt der Nutzer die Unterhaltung später
// fort (claude --resume), ist das ein neuer Prozess, der seine Marke erst
// wieder prüft. Alles andere — der geteilte Dienst von OpenCode, ein Halter
// ohne Prozess (die CLI), ein anderer Rechner oder Namensraum, ein System
// ohne /proc — ist unsicher und wird nie übernommen.

// Die Rollen eines Halter-Prozesses.
const (
	// RoleSession: der Prozess lebt genau so lange wie eine Sitzung.
	RoleSession = "session"
	// RoleShared: der Prozess dient mehreren Sitzungen und lebt länger als sie.
	RoleShared = "shared"
)

// Process ist der Halter-Prozess.
type Process struct {
	PID int `json:"pid"`
	// Start ist Feld 22 aus /proc/<pid>/stat: die Startzeit in Ticks seit dem
	// Systemstart. Zusammen mit BootID macht sie die PID eindeutig.
	Start        string `json:"start"`
	Name         string `json:"name"`
	Role         string `json:"role"`
	Host         string `json:"host"`
	BootID       string `json:"bootId"`
	PIDNamespace string `json:"pidNamespace"`
}

// ProcessProbe ist der Blick auf die Prozesse dieses Rechners.
type ProcessProbe interface {
	// Self nennt Rechner, Systemstart und PID-Namensraum des eigenen
	// Prozesses; ok ist false, wenn sie sich nicht bestimmen lassen.
	Self() (host, bootID, pidNamespace string, ok bool)
	// Start nennt die Startzeit von pid; ok ist false, wenn es den Prozess
	// nicht gibt.
	Start(pid int) (start string, ok bool, err error)
}

// orphaned beurteilt, ob die Sperre von holder verwaist ist.
func orphaned(probe ProcessProbe, holder *Holder) (string, string) {
	if holder == nil || holder.Process == nil {
		return OrphanedUncertain, "kein Halter-Prozess bekannt (von der CLI genommen)"
	}
	process := holder.Process
	host, bootID, namespace, ok := probe.Self()
	if !ok {
		return OrphanedUncertain, "Prozesse dieses Rechners nicht prüfbar"
	}
	if process.Host != host || process.BootID != bootID || process.PIDNamespace != namespace {
		return OrphanedUncertain, fmt.Sprintf("Halter auf einem anderen Rechner, seit einem anderen Systemstart oder in einem anderen PID-Namensraum (%s)", process.Host)
	}
	start, alive, err := probe.Start(process.PID)
	if err != nil {
		return OrphanedUncertain, fmt.Sprintf("Halter-Prozess %d nicht prüfbar: %v", process.PID, err)
	}
	alive = alive && start == process.Start
	switch {
	case alive && process.Role == RoleSession:
		return OrphanedNo, fmt.Sprintf("Sitzungsprozess %s (%d) läuft", process.Name, process.PID)
	case alive:
		return OrphanedUncertain, fmt.Sprintf("geteilter Prozess %s (%d) läuft; ob die Sitzung noch lebt, ist daran nicht zu sehen", process.Name, process.PID)
	case process.Role == RoleSession:
		return OrphanedYes, fmt.Sprintf("Sitzungsprozess %s (%d) beendet", process.Name, process.PID)
	default:
		return OrphanedUncertain, fmt.Sprintf("geteilter Prozess %s (%d) beendet; ein Ende der Sitzung ist damit nicht belegt", process.Name, process.PID)
	}
}

// clientRoles ordnet den Namen eines Client-Prozesses seiner Rolle zu.
var clientRoles = map[string]string{
	"claude":   RoleSession,
	"opencode": RoleShared,
}

// FindHolder sucht ab dem Elternprozess des eigenen Prozesses aufwärts den
// Prozess eines bekannten Clients und gibt ihn als Halter zurück. Ohne
// Treffer (anderer Client, kein /proc) ist das Ergebnis nil: dann gilt die
// Sperre nie als sicher verwaist.
func FindHolder() *Process {
	return findHolder(procRoot, os.Getppid())
}

// procRoot ist für Tests austauschbar.
var procRoot = "/proc"

func findHolder(root string, pid int) *Process {
	host, bootID, namespace, ok := systemProbe{root: root}.Self()
	if !ok {
		return nil
	}
	for depth := 0; pid > 1 && depth < 16; depth++ {
		stat, err := os.ReadFile(filepath.Join(root, strconv.Itoa(pid), "stat"))
		if err != nil {
			return nil
		}
		name, parent, start, err := parseStat(string(stat))
		if err != nil {
			return nil
		}
		if exe, err := os.Readlink(filepath.Join(root, strconv.Itoa(pid), "exe")); err == nil {
			if base := filepath.Base(strings.TrimSuffix(exe, " (deleted)")); clientRoles[base] != "" {
				name = base
			}
		}
		if role := clientRoles[name]; role != "" {
			return &Process{PID: pid, Start: start, Name: name, Role: role,
				Host: host, BootID: bootID, PIDNamespace: namespace}
		}
		pid = parent
	}
	return nil
}

// parseStat holt comm (Feld 2), ppid (Feld 4) und starttime (Feld 22) aus
// einer Zeile von /proc/<pid>/stat. comm steht in Klammern und darf selbst
// Leerzeichen und Klammern enthalten; gezählt wird hinter der letzten.
func parseStat(stat string) (name string, parent int, start string, err error) {
	open := strings.Index(stat, "(")
	end := strings.LastIndex(stat, ")")
	if open < 0 || end < open {
		return "", 0, "", fmt.Errorf("kein comm-Feld")
	}
	name = stat[open+1 : end]
	fields := strings.Fields(stat[end+1:])
	// fields[0] ist Feld 3.
	if len(fields) < 20 {
		return "", 0, "", fmt.Errorf("nur %d Felder", len(fields)+2)
	}
	parent, err = strconv.Atoi(fields[1])
	if err != nil {
		return "", 0, "", err
	}
	return name, parent, fields[19], nil
}

// systemProbe liest /proc.
type systemProbe struct{ root string }

func (probe systemProbe) dir() string {
	if probe.root != "" {
		return probe.root
	}
	return procRoot
}

func (probe systemProbe) Self() (string, string, string, bool) {
	root := probe.dir()
	host, err := os.Hostname()
	if err != nil {
		return "", "", "", false
	}
	boot, err := os.ReadFile(filepath.Join(root, "sys", "kernel", "random", "boot_id"))
	if err != nil {
		return "", "", "", false
	}
	namespace, err := os.Readlink(filepath.Join(root, "self", "ns", "pid"))
	if err != nil {
		return "", "", "", false
	}
	return host, strings.TrimSpace(string(boot)), namespace, true
}

func (probe systemProbe) Start(pid int) (string, bool, error) {
	stat, err := os.ReadFile(filepath.Join(probe.dir(), strconv.Itoa(pid), "stat"))
	if os.IsNotExist(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	_, _, start, err := parseStat(string(stat))
	if err != nil {
		return "", false, err
	}
	return start, true, nil
}
