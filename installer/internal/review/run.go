// Package review verwaltet Review-Läufe: die Auswahl, ihr Verzeichnis und den
// Zustand der Einträge darin.
//
// Ein Lauf ist die Klammer um alles, was an einem Tag geprüft wird — Werkzeuge
// wie Assistenten-Reviews. Beide sind Einträge desselben Laufs; nur der Weg zur
// Ausführung unterscheidet sich.
package review

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	// ResultsDirName ist das Verzeichnis unter local.dir, in dem die Läufe liegen.
	ResultsDirName = "results"
	// RunFileName trägt Festlegung und Gesamtzustand eines Laufs.
	RunFileName = "run.json"
	// EntriesDirName nimmt je Eintrag eine eigene Datei auf.
	EntriesDirName = "entries"
	// SchemaVersion ist die Fassung von run.json.
	SchemaVersion = 1
	// DateLayout ist der Name eines Laufverzeichnisses.
	DateLayout = "2006-01-02"
)

// Kind ist die Art eines Eintrags: wer ihn ausführt.
type Kind string

const (
	// KindTool ist ein Security-Tool aus der Matrix, ausgeführt über eine CLI.
	KindTool Kind = "tool"
	// KindAI ist ein Review-Rezept aus dem Katalog, ausgeführt von einem Assistenten.
	KindAI Kind = "ai"
)

// ValidKind meldet, ob eine Art bekannt ist.
func ValidKind(kind Kind) bool {
	return kind == KindTool || kind == KindAI
}

// State ist der Zustand eines Laufs oder eines Eintrags.
type State string

const (
	// StateCreated: der Lauf ist angelegt, gestartet ist noch nichts.
	StateCreated State = "created"
	// StateStart: der Eintrag ist ausgewählt, aber noch nicht gestartet.
	StateStart State = "start"
	// StateRunning: läuft gerade.
	StateRunning State = "running"
	// StateDone: fertig. Auch dann, wenn Befunde entstanden sind — die sind das
	// Ergebnis und kein Fehlschlag.
	StateDone State = "done"
	// StateFailed: technisch fehlgeschlagen.
	StateFailed State = "failed"
	// StateSkipped: übersprungen, etwa weil das Werkzeug fehlt.
	StateSkipped State = "skipped"
)

// Mode ist die Betriebsart eines AI-Eintrags im Lauf.
//
// Sie steht im Rezept und nicht im Command: derselbe Lauf führt Perspektiven
// und Evidence-Quellen nebeneinander, und welche Art ein Rezept ist, gehört
// zum Rezept.
type Mode string

const (
	// ModePerspective ist die Vorgabe: der Eintrag läuft nach dem Merge und
	// bewertet die Gruppen aus review-input.json, gefiltert über scope.tools.
	// Sein Ergebnis ist ein Markdown-Dokument im Laufordner.
	ModePerspective Mode = "perspective"
	// ModeEvidence: der Eintrag läuft vor dem Merge, liest Code im
	// eingefrorenen Pfad-Scope und schreibt SARIF nach raw/<entry>.sarif. Der
	// Merge sammelt es wie Scanner-Rohdaten ein.
	ModeEvidence Mode = "evidence"
)

// ValidMode meldet, ob eine Betriebsart bekannt ist. Das leere Feld gilt als
// bekannt: es ist die Vorgabe perspective.
func ValidMode(mode Mode) bool {
	return mode == "" || mode == ModePerspective || mode == ModeEvidence
}

// NormalizeMode macht aus dem leeren Feld die Vorgabe perspective.
//
// Läufe aus der Zeit vor dieser Betriebsart tragen kein mode-Feld. Sie sind
// Perspektiven-Läufe und sollen es bleiben — deshalb wird das leere Feld
// gelesen und nicht abgewiesen, und deshalb bleibt schemaVersion unverändert:
// das Feld kommt hinzu, keines ändert seine Bedeutung.
func NormalizeMode(mode Mode) Mode {
	if mode == "" {
		return ModePerspective
	}
	return mode
}

// Entry ist ein einzelner Punkt eines Laufs.
type Entry struct {
	Name         string `json:"name"`
	Kind         Kind   `json:"kind"`
	State        State  `json:"state"`
	RecipeKey    string `json:"recipeKey,omitempty"`
	RecipePath   string `json:"recipePath,omitempty"`
	RecipeOrigin string `json:"recipeOrigin,omitempty"`
	Title        string `json:"title,omitempty"`
	// Mode ist die beim Anlegen des Laufs eingefrorene Betriebsart eines
	// AI-Eintrags. Leer bei Tool-Einträgen und in Läufen von vor der
	// Evidence-Betriebsart; EntryMode macht daraus perspective.
	Mode           Mode   `json:"mode,omitempty"`
	ResultRequired *bool  `json:"resultRequired,omitempty"`
	DefaultResult  string `json:"defaultResult,omitempty"`
	Scope          *Scope `json:"scope,omitempty"`
}

// EntryMode liefert die Betriebsart eines Eintrags mit der Vorgabe für
// Altläufe.
func EntryMode(entry Entry) Mode {
	return NormalizeMode(entry.Mode)
}

// Scope ist der beim Erzeugen eines Laufs eingefrorene Bewertungs-Scope eines
// AI-Eintrags.
//
// Tools gehört zu mode: perspective und filtert die Gruppen aus
// review-input.json über evidence.tool. Paths gehört zu mode: evidence und
// begrenzt, welchen Code der Eintrag lesen darf. Beides zusammen ergibt keinen
// Sinn und wird von ValidateAuditContract abgewiesen.
type Scope struct {
	Tools []string `json:"tools,omitempty"`
	// Paths sind Globs relativ zur Projektwurzel. Sie sind der verbindliche
	// Scope eines Evidence-Laufs: scope-hint bleibt Freitext für /k-review und
	// darf sie weder erweitern noch überstimmen. Innerhalb der Globs gelten
	// zusätzlich die zentralen Ausschlüsse der Modulsuche — siehe
	// PathExcludedFromScope.
	Paths []string `json:"paths,omitempty"`
}

// Run ist der Inhalt von run.json.
type Run struct {
	SchemaVersion int      `json:"schemaVersion"`
	Created       string   `json:"created"`
	State         State    `json:"state"`
	Languages     []string `json:"languages"`
	Entries       []Entry  `json:"entries"`
}

// Summary beschreibt einen vorhandenen Lauf, ohne ihn vollständig zu laden.
type Summary struct {
	// Name ist der Verzeichnisname, üblich das Datum.
	Name string `json:"name"`
	Dir  string `json:"dir"`
	// EntryCount stammt aus run.json, State wird aus entries/ abgeleitet.
	// Fehlt run.json, bleiben beide leer: unter results/ können auch
	// Verzeichnisse aus der Zeit davor liegen.
	State      State `json:"state"`
	EntryCount int   `json:"entryCount"`
	HasRunFile bool  `json:"hasRunFile"`
}

// entryNamePattern begrenzt, was als Eintragsname durchgeht. Der Name wird zu
// einem Dateinamen unter entries/, darf also weder Pfadtrenner noch etwas
// enthalten, das aus dem Verzeichnis herausführt.
var entryNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

// ValidEntryName meldet, ob ein Name als Dateiname taugt.
func ValidEntryName(name string) bool {
	return entryNamePattern.MatchString(name)
}

// ResultsDir ist das Verzeichnis, in dem die Läufe liegen.
func ResultsDir(localDir string) string {
	return filepath.Join(localDir, ResultsDirName)
}

// RunDir ist das Verzeichnis eines Laufs.
func RunDir(localDir string, name string) string {
	return filepath.Join(ResultsDir(localDir), name)
}

// EntryFile ist die Datei, in die ein Eintrag seinen Fortschritt schreibt.
func EntryFile(runDir string, name string) string {
	return filepath.Join(runDir, EntriesDirName, name+".json")
}

// ListRuns liefert die vorhandenen Läufe, den jüngsten zuerst.
//
// Gelistet werden alle Verzeichnisse unter results/, nicht nur die mit einer
// run.json: ältere Ergebnisse aus der Zeit vor diesem Modell sollen sichtbar
// bleiben, statt stillschweigend zu fehlen.
func ListRuns(localDir string) ([]Summary, error) {
	resultsDir := ResultsDir(localDir)
	entries, err := os.ReadDir(resultsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Summary{}, nil
		}
		return nil, err
	}

	runs := []Summary{}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		summary := Summary{
			Name: entry.Name(),
			Dir:  filepath.Join(resultsDir, entry.Name()),
		}
		if run, err := ReadRun(summary.Dir); err == nil {
			summary.HasRunFile = true
			// Der Zustand kommt aus entries/, nicht aus run.json: die Datei
			// hält fest, was ausgewählt wurde, den Fortschritt führen die
			// Einträge. Weichen beide ab, gilt entries/.
			summary.State = DeriveRunState(summary.Dir, run)
			summary.EntryCount = len(run.Entries)
		}
		runs = append(runs, summary)
	}

	// Erst die Läufe, jüngster zuerst; danach alles andere alphabetisch. Ein
	// reines Absteigend-nach-Namen schöbe die Ergebnisfamilien aus der Zeit vor
	// diesem Modell nach oben, weil Buchstaben über Ziffern liegen.
	sort.Slice(runs, func(a, b int) bool {
		left, right := isRunName(runs[a].Name), isRunName(runs[b].Name)
		if left != right {
			return left
		}
		if left {
			return runs[a].Name > runs[b].Name
		}
		return runs[a].Name < runs[b].Name
	})
	return runs, nil
}

// isRunName meldet, ob ein Verzeichnisname nach einem Lauf aussieht. Geprüft
// wird nur die Form: ein Name wie 2026-13-45 wäre zwar kein Datum, aber auch
// kein Grund, das Verzeichnis anders einzuordnen.
func isRunName(name string) bool {
	_, err := time.Parse(DateLayout, name)
	return err == nil
}

// ReadRun liest die run.json eines Laufs.
func ReadRun(runDir string) (Run, error) {
	data, err := os.ReadFile(filepath.Join(runDir, RunFileName))
	if err != nil {
		return Run{}, err
	}
	var run Run
	if err := json.Unmarshal(data, &run); err != nil {
		return Run{}, fmt.Errorf("%s ist nicht lesbar: %w", RunFileName, err)
	}
	return run, nil
}

// CreateRun legt einen Lauf für den angegebenen Tag an.
//
// Gibt es das Verzeichnis schon, bricht der Aufruf ab. Ein Tag, ein Lauf: wäre
// die zweite Fassung erlaubt, müsste an jeder auswertenden Stelle entschieden
// werden, welche gilt.
//
// Angelegt wird nur; gestartet wird nichts.
func CreateRun(localDir string, day time.Time, languages []string, entries []Entry) (string, error) {
	if len(entries) == 0 {
		return "", fmt.Errorf("kein Eintrag ausgewählt")
	}

	seen := map[string]bool{}
	for _, entry := range entries {
		if !ValidEntryName(entry.Name) {
			return "", fmt.Errorf("unzulässiger Eintragsname: %q", entry.Name)
		}
		if !ValidKind(entry.Kind) {
			return "", fmt.Errorf("unbekannte Art für %s: %q", entry.Name, entry.Kind)
		}
		if !ValidMode(entry.Mode) {
			return "", fmt.Errorf("unbekannte Betriebsart für %s: %q", entry.Name, entry.Mode)
		}
		if seen[entry.Name] {
			return "", fmt.Errorf("Eintrag doppelt ausgewählt: %s", entry.Name)
		}
		seen[entry.Name] = true
	}

	resultsDir := ResultsDir(localDir)
	if !isDir(resultsDir) {
		return "", fmt.Errorf("%s fehlt — die projekteigene Struktur ist nicht vollständig", resultsDir)
	}

	name := day.Format(DateLayout)
	runDir := RunDir(localDir, name)
	// MkdirAll wäre hier falsch: ein vorhandenes Verzeichnis soll auffallen.
	if err := os.Mkdir(runDir, 0o755); err != nil {
		if os.IsExist(err) {
			return "", fmt.Errorf("für %s gibt es bereits einen Lauf: %s", name, runDir)
		}
		return "", err
	}

	if err := os.Mkdir(filepath.Join(runDir, EntriesDirName), 0o755); err != nil {
		return "", err
	}

	prepared := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		entry.State = StateStart
		entry.Scope = cloneScope(entry.Scope)
		prepared = append(prepared, entry)
	}

	run := Run{
		SchemaVersion: SchemaVersion,
		Created:       day.Format(time.RFC3339),
		State:         StateCreated,
		Languages:     languages,
		Entries:       prepared,
	}
	if run.Languages == nil {
		run.Languages = []string{}
	}

	if err := writeRun(runDir, run); err != nil {
		return "", err
	}
	return runDir, nil
}

func cloneScope(scope *Scope) *Scope {
	if scope == nil {
		return nil
	}
	cloned := &Scope{}
	if scope.Tools != nil {
		cloned.Tools = append([]string{}, scope.Tools...)
	}
	if scope.Paths != nil {
		cloned.Paths = append([]string{}, scope.Paths...)
	}
	return cloned
}

func writeRun(runDir string, run Run) error {
	data, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(runDir, RunFileName), append(data, '\n'), 0o644)
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
