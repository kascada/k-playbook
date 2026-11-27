package merge

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/kascada/k-playbook/installer/internal/review"
)

// Der Merge auf ein Arbeitsverzeichnis ist der Weg eines eigenständigen
// Reviews (Task 197): /k-review erhebt mit einem Rezept eine Fundliste als
// SARIF in ein Verzeichnis, und dieser Merge fasst sie zusammen — ohne Lauf,
// ohne run.json und entries/, ohne known-decisions. Gruppen, stabile IDs und
// Schwere rechnet derselbe Code wie im Audit-Lauf; die Ergebnisform ist
// dieselbe review-input.json.
//
// Jede Datei <eintrag>.sarif unmittelbar im Verzeichnis ist eine Quelle. Der
// Eintragsname ist der Rezeptschlüssel (code, tech). Geprüft wird jede Quelle
// über DirOptions.Check, bevor sie gelesen wird — dieselbe Prüfung, die im
// Audit-Lauf beim Melden eines Evidence-Eintrags läuft. Eine ungültige Quelle
// hält den ganzen Merge an: Es wird nichts geschrieben, und der Fehler nennt
// jede Quelle mit ihrem Grund.

// DirSARIFSuffix ist die Endung einer Quelle im Arbeitsverzeichnis.
const DirSARIFSuffix = ".sarif"

// dirFilePerm sind die Rechte jeder Datei, die der Merge in ein
// Arbeitsverzeichnis schreibt: Rohdaten eines Reviews können Secrets
// enthalten, deshalb liest sie nur der eigene Benutzer.
const dirFilePerm os.FileMode = 0o600

// knownDecisionsSkippedWarning ist der Hinweis im Beleg, dass dieser Weg
// keine bekannten Entscheidungen abgleicht.
const knownDecisionsSkippedWarning = "Bekannte Entscheidungen wurden nicht berücksichtigt: Der Merge auf ein Arbeitsverzeichnis gleicht keine known-decisions ab."

// DirOptions beschreibt den Merge auf ein Arbeitsverzeichnis.
type DirOptions struct {
	// ProjectDir ist die Projektwurzel; run.dir im Beleg steht relativ dazu,
	// wenn das Verzeichnis im Projekt liegt, sonst absolut.
	ProjectDir string
	// Dir ist das Arbeitsverzeichnis mit den SARIF-Dateien.
	Dir string
	// KPlaybookVersion ist die Version des Werkzeugs, wenn sie bekannt ist.
	KPlaybookVersion string
	// SeverityMappingPath ist der optionale Pfad zur Fallback-Tabelle für
	// abgeleitete Schwere.
	SeverityMappingPath string
	// Check prüft eine Quelle gegen den Vertrag ihres Rezepts und bereinigt
	// sie um Funde außerhalb des Pfad-Scopes (review.CheckEvidenceSARIF mit
	// den Angaben des Rezepts). Pflicht: Ohne Prüfung käme eine erfundene
	// Rule-ID still in den Beleg.
	Check func(entry string, data []byte) (review.EvidenceReport, error)
	// Now liefert die Erzeugungszeit. Fehlt es, wird time.Now verwendet.
	Now func() time.Time
}

// DirSource ist eine geprüfte Quelle des Arbeitsverzeichnisses.
type DirSource struct {
	// Entry ist der Eintragsname, der Dateiname ohne .sarif.
	Entry string `json:"entry"`
	// SARIF ist der Dateiname im Arbeitsverzeichnis.
	SARIF string `json:"sarif"`
	// Findings ist die Zahl der Funde, die stehen bleiben.
	Findings int `json:"findings"`
	// Dropped ist die Zahl der Funde außerhalb des Pfad-Scopes.
	Dropped int `json:"droppedFindings"`
	// DroppedPaths sind die Orte der verworfenen Funde.
	DroppedPaths []string `json:"droppedPaths,omitempty"`
	// Note ist der Satz zu den verworfenen Funden; leer ohne Verwerfen.
	Note string `json:"note,omitempty"`

	cleaned []byte
}

// DirProblem ist eine Quelle, die die Prüfung nicht bestanden hat.
type DirProblem struct {
	SARIF  string `json:"sarif"`
	Reason string `json:"reason"`
}

// DirCheckError sagt, dass mindestens eine Quelle ungültig ist; nichts wurde
// geschrieben.
type DirCheckError struct {
	Problems []DirProblem
}

func (e *DirCheckError) Error() string {
	parts := make([]string, 0, len(e.Problems))
	for _, problem := range e.Problems {
		parts = append(parts, problem.SARIF+": "+problem.Reason)
	}
	return "Fundliste ungültig — " + strings.Join(parts, "; ")
}

// ErrNoDirSARIF sagt, dass das Arbeitsverzeichnis keine Quelle enthält.
var ErrNoDirSARIF = errors.New("im Arbeitsverzeichnis liegt keine SARIF-Datei (<eintrag>.sarif)")

// dirEntryPattern ist die zulässige Form eines Eintragsnamens: der
// Rezeptschlüssel, klein, mit Ziffern und Bindestrichen.
var dirEntryPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// CheckDir prüft die Quellen eines Arbeitsverzeichnisses und schreibt nichts.
// Das ist der Abschluss des Bausteins Erheben: Er zeigt, ob die Fundliste den
// Vertrag des Rezepts erfüllt, bevor zusammengefasst wird.
func CheckDir(options DirOptions) ([]DirSource, error) {
	if err := checkDirOptions(options); err != nil {
		return nil, err
	}
	names, err := listDirSARIF(options.Dir)
	if err != nil {
		return nil, err
	}
	sources := make([]DirSource, 0, len(names))
	problems := []DirProblem{}
	for _, name := range names {
		source, problem := checkDirSource(options, name)
		if problem != nil {
			problems = append(problems, *problem)
			continue
		}
		sources = append(sources, source)
	}
	if len(problems) > 0 {
		return nil, &DirCheckError{Problems: problems}
	}
	return sources, nil
}

// BuildDir prüft die Quellen und rechnet den Beleg, ohne zu schreiben.
func BuildDir(options DirOptions) (Result, []DirSource, error) {
	sources, err := CheckDir(options)
	if err != nil {
		return Result{}, nil, err
	}
	severityMapping, err := LoadSeverityMapping(options.SeverityMappingPath)
	if err != nil {
		return Result{}, nil, err
	}
	now := time.Now
	if options.Now != nil {
		now = options.Now
	}

	entries := make([]EntrySummary, 0, len(sources))
	selected := make([]review.Entry, 0, len(sources))
	findings := []Finding{}
	for _, source := range sources {
		data := source.cleaned
		if data == nil {
			data, err = os.ReadFile(filepath.Join(options.Dir, source.SARIF))
			if err != nil {
				return Result{}, nil, err
			}
		}
		count := source.Findings
		job := JobSummary{Job: source.Entry, State: review.StateDone, SARIF: source.SARIF, Findings: &count, Reason: source.Note}
		sourceFindings, err := parseSARIF(data, filepath.Join(options.Dir, source.SARIF), source.Entry, review.ModeEvidence, job, severityMapping)
		if err != nil {
			return Result{}, nil, err
		}
		findings = append(findings, sourceFindings...)
		entries = append(entries, EntrySummary{
			Name:          source.Entry,
			Kind:          review.KindAI,
			SelectedState: review.StateDone,
			State:         review.StateDone,
			Present:       true,
			Reason:        source.Note,
			Jobs:          []JobSummary{job},
			Mode:          review.ModeEvidence,
		})
		selected = append(selected, review.Entry{Name: source.Entry, Kind: review.KindAI, State: review.StateDone, RecipeKey: source.Entry, Mode: review.ModeEvidence})
	}
	groups := GroupFindings(findings)

	version := options.KPlaybookVersion
	if version == "" {
		version = "unknown"
	}
	return Result{
		SchemaVersion:    1,
		Generated:        now().Format(time.RFC3339),
		KPlaybookVersion: version,
		Run: RunContext{
			Name:            filepath.Base(filepath.Clean(options.Dir)),
			Dir:             dirDisplay(options.ProjectDir, options.Dir),
			State:           review.StateDone,
			DerivedState:    review.StateDone,
			Languages:       []string{},
			SelectedEntries: selected,
		},
		Entries:        entries,
		Findings:       findings,
		Groups:         groups,
		KnownDecisions: KnownDecisions{Sources: nil, Decisions: nil, Warnings: []string{knownDecisionsSkippedWarning}},
	}, sources, nil
}

// MergeDir prüft, rechnet und schreibt: bereinigte Quellen zurück an ihren
// Ort, dann review-input.json und review-input.md ins Arbeitsverzeichnis,
// jede Datei mit den Rechten 0600.
func MergeDir(options DirOptions) (Result, []DirSource, Output, error) {
	result, sources, err := BuildDir(options)
	if err != nil {
		return Result{}, nil, Output{}, err
	}
	for _, source := range sources {
		if source.cleaned == nil {
			continue
		}
		if err := writeAtomicPerm(filepath.Join(options.Dir, source.SARIF), source.cleaned, dirFilePerm, false); err != nil {
			return Result{}, nil, Output{}, err
		}
	}
	jsonPath := filepath.Join(options.Dir, "review-input.json")
	markdownPath := filepath.Join(options.Dir, "review-input.md")
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return Result{}, nil, Output{}, err
	}
	if err := writeAtomicPerm(jsonPath, append(data, '\n'), dirFilePerm, false); err != nil {
		return Result{}, nil, Output{}, err
	}
	if err := writeAtomicPerm(markdownPath, []byte(markdown(result, dirRawNote)), dirFilePerm, false); err != nil {
		return Result{}, nil, Output{}, err
	}
	return result, sources, Output{JSON: jsonPath, Markdown: markdownPath}, nil
}

func checkDirOptions(options DirOptions) error {
	if strings.TrimSpace(options.Dir) == "" {
		return errors.New("kein Arbeitsverzeichnis angegeben")
	}
	if options.Check == nil {
		return errors.New("keine Prüfung der Quellen angegeben")
	}
	info, err := os.Stat(options.Dir)
	if err != nil {
		return fmt.Errorf("Arbeitsverzeichnis %s: %w", options.Dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("Arbeitsverzeichnis %s ist kein Verzeichnis", options.Dir)
	}
	return nil
}

// listDirSARIF nennt die Quellen unmittelbar im Verzeichnis, sortiert. Andere
// Dateien — review-input.*, review-triage.md — und Unterverzeichnisse zählen
// nicht.
func listDirSARIF(dir string) ([]string, error) {
	items, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	names := []string{}
	for _, item := range items {
		if !item.Type().IsRegular() || !strings.HasSuffix(item.Name(), DirSARIFSuffix) || strings.HasPrefix(item.Name(), ".") {
			continue
		}
		names = append(names, item.Name())
	}
	if len(names) == 0 {
		return nil, ErrNoDirSARIF
	}
	sort.Strings(names)
	return names, nil
}

func checkDirSource(options DirOptions, name string) (DirSource, *DirProblem) {
	entry := strings.TrimSuffix(name, DirSARIFSuffix)
	if !dirEntryPattern.MatchString(entry) {
		return DirSource{}, &DirProblem{SARIF: name, Reason: "der Dateiname ist kein Eintragsname (klein, Ziffern und Bindestriche, etwa code.sarif)"}
	}
	data, err := os.ReadFile(filepath.Join(options.Dir, name))
	if err != nil {
		return DirSource{}, &DirProblem{SARIF: name, Reason: err.Error()}
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return DirSource{}, &DirProblem{SARIF: name, Reason: "die Datei ist leer"}
	}
	report, err := options.Check(entry, data)
	if err != nil {
		return DirSource{}, &DirProblem{SARIF: name, Reason: err.Error()}
	}
	return DirSource{
		Entry:        entry,
		SARIF:        name,
		Findings:     report.Kept,
		Dropped:      report.Dropped,
		DroppedPaths: report.DroppedPaths,
		Note:         report.ScopeNote(),
		cleaned:      report.Cleaned,
	}, nil
}

// dirDisplay ist run.dir: projektrelativ, wenn das Verzeichnis im Projekt
// liegt, sonst der absolute Pfad.
func dirDisplay(projectDir string, dir string) string {
	clean := filepath.Clean(dir)
	if projectDir != "" {
		if relative, err := filepath.Rel(projectDir, clean); err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return filepath.ToSlash(relative)
		}
	}
	return filepath.ToSlash(clean)
}
