// Package health ist die Sammelstelle für Fehler und Warnungen über die
// Umgebung eines Projekts (Task 098). Eine Sitzung bekommt den Report mit dem
// Briefing und sagt zu Beginn, was fehlt; Mensch und Oberfläche bekommen
// dieselbe Liste über `k-playbook health`, k_playbook_health und die GUI.
//
// Das Paket prüft selbst nichts. Jede Quelle hängt sich mit Register ein und
// liefert ihre Probleme; Check ruft alle und sortiert. So stellt sich eine
// weitere Quelle daneben, ohne dass Briefing, CLI oder GUI sich ändern. Die
// Quellen stehen bewusst nicht hier: Die ersten vier lesen aus dem Paket
// project (gh, Basis-Werkzeuge, Quellenkonfiguration, Kephalaion-Anbindung),
// die fünfte — die persönliche GitHub-Zugangsdatei, Paket githubaccess —
// wird dort daneben eingehängt, und project trägt den Report im Briefing —
// dieses Paket darf project darum nicht importieren. project registriert
// seine Quellen beim Laden (project/health.go).
//
// Repariert wird nichts, mit einer benannten Ausnahme: Die Quelle github
// legt die fehlende Zugangsdatei einmal als Vorlage an, weil ihr Vertrag das
// so will; sie meldet es als warning. Kein Eintrag nennt einen Token oder
// einen Wert aus einer Datei — Dienst- und Schlüsselnamen dürfen vorkommen,
// Werte nie: nur Stufe, Bereich, einen Satz und die Abhilfe.
package health

import (
	"context"
	"sort"
	"sync"
)

// Level ist die Stufe eines Problems.
type Level string

const (
	// LevelError: Etwas, das die Arbeit behindert; macht Report.OK falsch.
	LevelError Level = "error"
	// LevelWarning: Etwas fehlt, die Arbeit geht trotzdem; lässt OK wahr.
	LevelWarning Level = "warning"
)

// Die Bereiche der heutigen Quellen. Eine Quelle nennt ihren Bereich selbst;
// die Konstanten halten die Namen an einer Stelle fest, damit Doku und Tests
// dieselben benutzen.
const (
	AreaKephalaion     = "kephalaion"
	AreaGH             = "gh"
	AreaBaseTools      = "baseTools"
	AreaVersionSources = "versionSources"
	AreaGitHub         = "github"
)

// Problem ist ein Eintrag des Reports.
type Problem struct {
	Level Level `json:"level"`
	// Area ist der Bereich, aus dem der Eintrag stammt.
	Area string `json:"area"`
	// Message ist ein Satz, der sagt, was nicht stimmt — nie ein Token oder
	// ein Wert aus einer Datei.
	Message string `json:"message"`
	// Hint ist die Abhilfe: was der Nutzer tun kann.
	Hint string `json:"hint"`
}

// Report ist die Antwort von Check.
type Report struct {
	// OK ist wahr, wenn kein Eintrag die Stufe error trägt; Warnungen lassen
	// es wahr.
	OK bool `json:"ok"`
	// Problems sind alle Einträge, sortiert nach Bereich, dann Stufe (error
	// vor warning), dann Satz. Nie nil: ohne Probleme eine leere Liste.
	Problems []Problem `json:"problems"`
}

// Source ist eine Quelle: Sie prüft ihren Bereich und liefert dessen
// Probleme, leer ohne. Eine Quelle bricht nie ab — was sie nicht lesen kann,
// ist selbst ein Problem.
type Source func(ctx context.Context, projectDir string) []Problem

var (
	sourcesMu sync.RWMutex
	sources   []Source
)

// Register hängt eine Quelle ein. Gedacht für die Initialisierung eines
// Pakets; die Reihenfolge der Registrierung spielt keine Rolle, Check
// sortiert.
func Register(source Source) {
	if source == nil {
		return
	}
	sourcesMu.Lock()
	defer sourcesMu.Unlock()
	sources = append(sources, source)
}

// Check fragt alle Quellen und stellt den Report zusammen. Es bricht nicht
// beim ersten Problem ab, sondern liefert alle.
func Check(ctx context.Context, projectDir string) Report {
	sourcesMu.RLock()
	registered := append([]Source(nil), sources...)
	sourcesMu.RUnlock()

	report := Report{OK: true, Problems: []Problem{}}
	for _, source := range registered {
		report.Problems = append(report.Problems, source(ctx, projectDir)...)
	}
	sortProblems(report.Problems)
	for _, problem := range report.Problems {
		if problem.Level == LevelError {
			report.OK = false
			break
		}
	}
	return report
}

// sortProblems ordnet stabil: Bereich, dann Stufe (error vor warning), dann
// Satz — damit zwei Aufrufe dieselbe Liste liefern.
func sortProblems(problems []Problem) {
	sort.SliceStable(problems, func(i, j int) bool {
		a, b := problems[i], problems[j]
		if a.Area != b.Area {
			return a.Area < b.Area
		}
		if a.Level != b.Level {
			return levelRank(a.Level) < levelRank(b.Level)
		}
		return a.Message < b.Message
	})
}

func levelRank(level Level) int {
	switch level {
	case LevelError:
		return 0
	case LevelWarning:
		return 1
	}
	return 2
}
