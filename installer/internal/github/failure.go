package github

import (
	"context"
	"regexp"
	"strconv"
	"strings"
)

// Ein roter Lauf soll seine Ursache zeigen, nicht 37 Einzelzeilen. Die
// Meldungen werden deshalb nach gleicher Ursache gruppiert: „32 Tests: stat
// …/K-PLAYBOOK.yaml: no such file".
//
// **Gezählt wird ein Test mit eigener Meldungszeile.** Ein übergeordneter Test
// mit Subtests hat keine eigene Meldung — er zählt nicht, sonst zählte dieselbe
// Ursache doppelt. Im Log von Lauf 35002675449 stehen 37 `--- FAIL`-Zeilen, die
// zu genau zwei Gruppen mit 32 und 1 Test führen; die vier übrigen sind Eltern
// von Subtests.

// FailureGroup ist eine Ursache: eine Meldung und die Tests, die an ihr
// gescheitert sind.
type FailureGroup struct {
	Message string `json:"message"`
	Count   int    `json:"count"`
	// Tests nennt die Namen. Sie stehen hinter dem Aufklappen der Gruppe; die
	// Karte zeigt zuerst die Ursache.
	Tests []string `json:"tests"`
}

// Failure ist die Ursache eines roten Laufs.
type Failure struct {
	Result
	RunID int64 `json:"runId"`
	// Groups sind die Ursachen, die häufigste zuerst.
	Groups []FailureGroup `json:"groups"`
	// FailLines ist die Zahl der `--- FAIL`-Zeilen, Counted die Zahl der
	// gezählten Tests. Dass beide auseinanderfallen, ist die Zählregel und
	// keine verlorene Zeile — deshalb stehen beide in der Antwort.
	FailLines int `json:"failLines"`
	Counted   int `json:"counted"`
	// Lines ist der Rückfall: bei einem Logformat ohne `--- FAIL` stehen hier
	// die letzten Fehlerzeilen des Jobs statt einer Gruppierung.
	Lines []string `json:"lines"`
	// Note erklärt, was zu sehen ist — auch dann, wenn nichts zu sehen ist.
	Note string `json:"note"`
}

var (
	// ansiEscape entfernt die Farbcodes, die GitHub im Log stehen lässt.
	ansiEscape = regexp.MustCompile("\x1b\\[[0-9;]*[a-zA-Z]")
	// logTimestamp ist der Zeitstempel, den GitHub jeder Zeile voranstellt.
	// Das Byte-Order-Mark davor steht nur in der ersten Zeile.
	logTimestamp = regexp.MustCompile(`^\x{FEFF}?\d{4}-\d{2}-\d{2}T[0-9:.]+Z ?`)
	// failLine ist `--- FAIL: TestName (0.00s)`, samt Einrückung.
	failLine = regexp.MustCompile(`^(\s*)--- FAIL: (\S+) \(`)
	// messageLine ist die erste Zeile, die ein Test selbst geschrieben hat:
	// `datei.go:42: Text`. Das Präfix wird abgeschnitten — es unterscheidet
	// sich je Test und ließe gleiche Ursachen auseinanderfallen.
	messageLine = regexp.MustCompile(`^(\s*)([\w./+-]+\.go:\d+): (.*)$`)
	// runnerWorkDir ist das Arbeitsverzeichnis des Runners. Es steckt in fast
	// jeder Pfadangabe und gehört vereinheitlicht, sonst gruppierte dieselbe
	// Ursache aus zwei Repos getrennt.
	runnerWorkDir = regexp.MustCompile(`/home/runner/work/[^/\s:]+/[^/\s:]+`)
	runnerHome    = regexp.MustCompile(`/home/runner\b`)
	// goTempDir ist das Verzeichnis aus t.TempDir(): der Name trägt Testnamen
	// und eine laufende Nummer und wäre in jedem Lauf ein anderer.
	goTempDir  = regexp.MustCompile(`/tmp/Test[\w]+/\d+`)
	errorMark  = regexp.MustCompile(`(?i)##\[error\]`)
	whitespace = regexp.MustCompile(`\s+`)
)

// FetchFailure holt das Log der fehlgeschlagenen Jobs und zieht die Ursache
// heraus.
//
// Das ist die teuerste Abfrage der Seite — gh lädt das Archiv des Laufs — und
// deshalb die einzige, die nicht beim Laden läuft: die Seite holt sie erst
// beim Aufklappen eines roten Laufs. Ihre Frist ist eigens länger.
func (c *Client) FetchFailure(ctx context.Context, runID int64) Failure {
	raw, err := c.ghWithTimeout(ctx, LogTimeout, "run", "view", strconv.FormatInt(runID, 10), "--log-failed")
	if err != nil {
		return Failure{Result: Classify(err), RunID: runID, Groups: []FailureGroup{}, Lines: []string{}}
	}
	failure := ParseFailureLog(string(raw))
	failure.RunID = runID
	return failure
}

// ParseFailureLog wertet das Log aus. Eigene Funktion, damit die Gruppierung an
// der gesicherten Fixture prüfbar ist, ohne Netz und ohne den Lauf: GitHub
// bewahrt Logs nur begrenzt auf.
func ParseFailureLog(raw string) Failure {
	failure := Failure{Result: Result{State: StateOK}, Groups: []FailureGroup{}, Lines: []string{}}

	lines := contentLines(raw)
	tests, failLines := collectFailedTests(lines)
	failure.FailLines = failLines

	if failLines == 0 {
		// Unbekanntes Logformat: gruppiert wird nichts, gezeigt werden die
		// letzten Fehlerzeilen des Jobs. Eine leere Karte wäre die schlechtere
		// Auskunft.
		failure.Lines = lastErrorLines(lines)
		if len(failure.Lines) == 0 {
			failure.Note = "Im Log stehen keine erkennbaren Testfehler. Die Ursache steht auf github.com im vollständigen Log."
			return failure
		}
		failure.Note = "Das Log folgt keinem erkannten Testformat. Statt einer Gruppierung stehen hier die letzten Fehlerzeilen des Jobs."
		return failure
	}

	failure.Groups = groupFailures(tests)
	failure.Counted = len(tests)
	failure.Note = failureNote(failure)
	return failure
}

func failureNote(failure Failure) string {
	if failure.Counted == 0 {
		return strconv.Itoa(failure.FailLines) + " fehlgeschlagene Tests, aber keiner mit eigener Meldungszeile. Die Ursache steht im vollständigen Log."
	}
	note := strconv.Itoa(failure.Counted) + " von " + strconv.Itoa(failure.FailLines) + " FAIL-Zeilen tragen eine eigene Meldung"
	if skipped := failure.FailLines - failure.Counted; skipped > 0 {
		note += "; die übrigen " + strconv.Itoa(skipped) + " sind übergeordnete Tests, deren Subtests die Meldung tragen"
	}
	return note + "."
}

// contentLines schneidet Job, Schritt, Zeitstempel und Farbcodes ab und lässt
// die Einrückung stehen: an ihr hängt, welcher Test welche Meldung trägt.
func contentLines(raw string) []string {
	var out []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimRight(line, "\r")
		// `gh run view --log-failed` stellt jeder Zeile Job und Schritt voran,
		// getrennt durch Tabulatoren.
		if parts := strings.Split(line, "\t"); len(parts) >= 3 {
			line = strings.Join(parts[2:], "\t")
		}
		line = logTimestamp.ReplaceAllString(line, "")
		line = ansiEscape.ReplaceAllString(line, "")
		out = append(out, line)
	}
	return out
}

type failedTest struct {
	name    string
	message string
}

// collectFailedTests sammelt jeden Test mit eigener Meldungszeile. Der zweite
// Rückgabewert ist die Zahl aller `--- FAIL`-Zeilen — sie ist größer, und
// genau das ist die Zählregel.
func collectFailedTests(lines []string) ([]failedTest, int) {
	var tests []failedTest
	failLines := 0

	pending := ""
	pendingIndent := 0
	waiting := false

	for _, line := range lines {
		if match := failLine.FindStringSubmatch(line); match != nil {
			failLines++
			// Der vorige Test hat keine eigene Meldung bekommen: er ist ein
			// übergeordneter Test, seine Subtests tragen sie. Er zählt nicht.
			pending = match[2]
			pendingIndent = len(match[1])
			waiting = true
			continue
		}
		if !waiting {
			continue
		}
		match := messageLine.FindStringSubmatch(line)
		if match == nil || len(match[1]) <= pendingIndent {
			continue
		}
		tests = append(tests, failedTest{name: pending, message: strings.TrimSpace(match[3])})
		waiting = false
	}
	return tests, failLines
}

// groupFailures fasst nach vereinheitlichter Meldung zusammen, die größte
// Gruppe zuerst. Gleich große Gruppen behalten die Reihenfolge des Logs.
func groupFailures(tests []failedTest) []FailureGroup {
	groups := []FailureGroup{}
	index := map[string]int{}

	for _, test := range tests {
		key := NormalizeFailureMessage(test.message)
		if at, ok := index[key]; ok {
			groups[at].Count++
			groups[at].Tests = append(groups[at].Tests, test.name)
			continue
		}
		index[key] = len(groups)
		groups = append(groups, FailureGroup{Message: key, Count: 1, Tests: []string{test.name}})
	}

	// Stabil sortiert: eine Einfügesortierung hält gleich große Gruppen in der
	// Reihenfolge, in der sie im Log stehen.
	for i := 1; i < len(groups); i++ {
		for j := i; j > 0 && groups[j].Count > groups[j-1].Count; j-- {
			groups[j], groups[j-1] = groups[j-1], groups[j]
		}
	}
	return groups
}

// NormalizeFailureMessage vereinheitlicht eine Meldung, damit gleiche Ursachen
// zusammenfallen: die Pfade des Runners und die Temp-Verzeichnisse der Tests
// tragen in jedem Lauf andere Namen. Das Präfix `datei:zeile:` ist bereits
// beim Lesen abgeschnitten — es unterscheidet sich je Test.
func NormalizeFailureMessage(message string) string {
	message = runnerWorkDir.ReplaceAllString(message, "<runner>")
	message = goTempDir.ReplaceAllString(message, "<tmp>")
	message = runnerHome.ReplaceAllString(message, "<runner>")
	return strings.TrimSpace(whitespace.ReplaceAllString(message, " "))
}

// lastErrorLines ist der Rückfall bei unbekanntem Logformat: die Zeilen, die
// GitHub selbst als Fehler markiert hat, sonst das Ende des Logs.
func lastErrorLines(lines []string) []string {
	const maximum = 12

	var marked []string
	for _, line := range lines {
		if errorMark.MatchString(line) {
			marked = append(marked, strings.TrimSpace(errorMark.ReplaceAllString(line, "")))
		}
	}
	if len(marked) > 0 {
		return tail(marked, maximum)
	}

	var content []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			content = append(content, strings.TrimSpace(line))
		}
	}
	return tail(content, maximum)
}

func tail(lines []string, maximum int) []string {
	if len(lines) <= maximum {
		return lines
	}
	return lines[len(lines)-maximum:]
}
