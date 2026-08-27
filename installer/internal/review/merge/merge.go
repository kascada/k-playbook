// Package merge fasst die Rohdaten eines Review-Laufs zu Review-Input-Artefakten
// zusammen.
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

	"github.com/kascada/k-playbook/installer/internal/knowndecisions"
	"github.com/kascada/k-playbook/installer/internal/review"
)

// Options beschreibt, welcher Lauf zusammengeführt wird.
type Options struct {
	// ProjectDir ist das Hauptverzeichnis mit K-PLAYBOOK.yaml.
	ProjectDir string
	// RunName ist der Name unter k-playbook-local/results/.
	RunName string
	// RunDir ist das Laufverzeichnis mit run.json, entries/ und raw/.
	RunDir string
	// KPlaybookVersion ist die Version des Werkzeugs, wenn sie bekannt ist.
	KPlaybookVersion string
	// SeverityMappingPath ist der optionale Pfad zur Fallback-Tabelle für
	// abgeleitete Schwere. Fehlt er, bleibt nur SARIF-/Metadaten-Ableitung.
	SeverityMappingPath string
	// LocalDir ist k-playbook-local; dort liegt die projektweite
	// known-decisions.md.
	LocalDir string
	// Now liefert die Erzeugungszeit. Fehlt es, wird time.Now verwendet.
	Now func() time.Time
}

// Output nennt die geschriebenen Artefakte.
type Output struct {
	JSON     string
	Markdown string
}

// RunContext hält die Festlegung und den abgeleiteten Zustand des Laufs.
type RunContext struct {
	Name            string         `json:"name"`
	Dir             string         `json:"dir"`
	Created         string         `json:"created,omitempty"`
	State           review.State   `json:"state,omitempty"`
	DerivedState    review.State   `json:"derivedState,omitempty"`
	Languages       []string       `json:"languages"`
	SelectedEntries []review.Entry `json:"selectedEntries"`
}

// EntrySummary hält den tatsächlichen Status eines ausgewählten Eintrags. Fehlt
// entries/<name>.json, bleibt Present false und State steht auf start.
type EntrySummary struct {
	Name          string       `json:"name"`
	Kind          review.Kind  `json:"kind"`
	SelectedState review.State `json:"selectedState,omitempty"`
	State         review.State `json:"state"`
	Present       bool         `json:"present"`
	Started       string       `json:"started,omitempty"`
	Finished      string       `json:"finished,omitempty"`
	Reason        string       `json:"reason,omitempty"`
	Jobs          []JobSummary `json:"jobs"`
	// Mode ist die eingefrorene Betriebsart eines AI-Eintrags aus run.json.
	// Sie bleibt intern: die Gruppierung braucht sie, das Schema von
	// review-input.json ändert sich dafür nicht. Bei Tool-Einträgen bleibt sie
	// leer — eine Betriebsart haben nur AI-Einträge.
	Mode review.Mode `json:"-"`
}

// JobSummary übernimmt den Status eines einzelnen Scan-Jobs.
type JobSummary struct {
	Job        string       `json:"job"`
	State      review.State `json:"state"`
	Module     string       `json:"module,omitempty"`
	ExitCode   *int         `json:"exitCode,omitempty"`
	SARIF      string       `json:"sarif,omitempty"`
	Findings   *int         `json:"findings,omitempty"`
	Candidates *int         `json:"candidates,omitempty"`
	Started    string       `json:"started,omitempty"`
	Finished   string       `json:"finished,omitempty"`
	Reason     string       `json:"reason,omitempty"`
}

// Result ist das zusammengeführte Modell eines Laufs.
type Result struct {
	SchemaVersion    int            `json:"schemaVersion"`
	Generated        string         `json:"generated"`
	KPlaybookVersion string         `json:"kPlaybookVersion"`
	Run              RunContext     `json:"run"`
	Entries          []EntrySummary `json:"entries"`
	Findings         []Finding      `json:"findings"`
	Groups           []Group        `json:"groups"`
	KnownDecisions   KnownDecisions `json:"knownDecisions"`
}

// Location ist die primäre Stelle eines SARIF-Results.
type Location struct {
	URI         string `json:"uri,omitempty"`
	StartLine   int    `json:"startLine,omitempty"`
	StartColumn int    `json:"startColumn,omitempty"`
}

// Dependency beschreibt einen Dependency-Befund, soweit er aus SARIF erkennbar
// ist.
//
// IDs und KeyIDs trennen zwei verschiedene Fragen. IDs ist alles, was im Text
// des Befunds an Kennungen vorkommt — die Menge für Anzeige, Bericht und
// known-decisions, und sie darf großzügig sein. KeyIDs ist die Teilmenge, die
// den Befund *identifiziert*: sie geht in den harten Dedupe-Schlüssel, und dort
// wäre eine im Advisory beiläufig genannte Fremd-Kennung schädlich, weil
// Union-Find transitiv gruppiert (siehe dependencyKeys in dedupe.go).
//
// Package/Version und TextPackage/TextVersion trennen dieselbe Frage für das
// Paket, nur andersherum benannt: Package und Version tragen ausschließlich
// **strukturiert** gelesene Werte — eine benannte Property oder ein purl — und
// nur sie gehen in den harten Schlüssel. TextPackage und TextVersion sind aus
// dem Freitext geparst (Message, Rule-Beschreibung) und ausdrücklich **nur**
// für Anzeige und Bericht da. Sie füllen sich nur, wo der strukturierte Wert
// fehlt.
//
// Anders als bei KeyIDs gibt es hier **keinen** Rückfall von der engen auf die
// breite Seite. KeyIDs darf bei leerer enger Menge auf IDs zurückfallen, weil
// eine Kennung den Befund auch dann benennt, wenn sie im Text steht. Ein aus
// Fließtext geratener Paketname tut das nicht: liegt er daneben, verschmilzt er
// zwei verschiedene Befunde, und Paket und Version sind im Schlüssel das
// Einzige, was dieselbe Kennung in zwei Paketen auseinanderhält (vendored
// libs). Ohne strukturierten Wert bleibt der harte Schlüssel deshalb aus.
type Dependency struct {
	Package     string   `json:"package,omitempty"`
	Version     string   `json:"version,omitempty"`
	Manifest    string   `json:"manifest,omitempty"`
	IDs         []string `json:"ids,omitempty"`
	KeyIDs      []string `json:"keyIds,omitempty"`
	TextPackage string   `json:"textPackage,omitempty"`
	TextVersion string   `json:"textVersion,omitempty"`
}

// Evidence nennt die Quelle eines Findings. Sie bleibt auch nach Dedupe
// erhalten, damit keine Scanner-Meldung still verschwindet.
type Evidence struct {
	Tool        string `json:"tool"`
	Job         string `json:"job"`
	SARIF       string `json:"sarif"`
	RunIndex    int    `json:"runIndex"`
	ResultIndex int    `json:"resultIndex"`
}

// KnownDecisionCoverage ist der sichtbare Marker für gedeckte Findings und Gruppen.
type KnownDecisionCoverage struct {
	ID        string `json:"id"`
	Category  string `json:"category"`
	MatchedBy string `json:"matchedBy,omitempty"`
}

// KnownDecisionCoverageCount zählt Deckung in einer Gruppe.
type KnownDecisionCoverageCount struct {
	ID       string `json:"id"`
	Category string `json:"category"`
	Findings int    `json:"findings"`
}

// KnownDecisions beschreibt geladene und angewendete Decisions im Merge-Ergebnis.
type KnownDecisions struct {
	Sources   []knowndecisions.SourceReport   `json:"sources"`
	Decisions []knowndecisions.DecisionReport `json:"decisions"`
	Warnings  []string                        `json:"warnings,omitempty"`
}

// Finding ist die normalisierte Form eines SARIF-Results.
type Finding struct {
	ID                     string                 `json:"id"`
	Evidence               Evidence               `json:"evidence"`
	RuleID                 string                 `json:"ruleId,omitempty"`
	RuleName               string                 `json:"ruleName,omitempty"`
	RuleDescription        string                 `json:"ruleDescription,omitempty"`
	Level                  string                 `json:"level,omitempty"`
	DerivedSeverity        string                 `json:"derivedSeverity"`
	SeveritySource         string                 `json:"severitySource"`
	Message                string                 `json:"message,omitempty"`
	Location               Location               `json:"location,omitempty"`
	Locations              []Location             `json:"locations,omitempty"`
	Fingerprints           map[string]string      `json:"fingerprints,omitempty"`
	PartialFingerprints    map[string]string      `json:"partialFingerprints,omitempty"`
	Dependency             Dependency             `json:"dependency,omitempty"`
	CoveredByKnownDecision *KnownDecisionCoverage `json:"coveredByKnownDecision,omitempty"`
	// Mode ist die Betriebsart des Eintrags, aus dem der Fund stammt. Sie
	// entscheidet über Gruppierung und Schlüsselklasse — siehe aiPathRuleKey in
	// dedupe.go und stableClass in stable.go. Wie bei EntrySummary bleibt sie
	// intern: das Schema von review-input.json ändert sich nicht.
	Mode review.Mode `json:"-"`
}

// Group ist eine Dedupe-Gruppe. Sie löscht keine Findings: alle Belege und die
// Finding-IDs bleiben erhalten.
type Group struct {
	ID                     string                       `json:"displayId,omitempty"`
	StableID               string                       `json:"stableId"`
	StableKey              string                       `json:"stableKey"`
	DedupeRules            []string                     `json:"dedupeRules,omitempty"`
	PossibleDuplicates     []string                     `json:"possibleDuplicates,omitempty"`
	Title                  string                       `json:"title,omitempty"`
	RuleID                 string                       `json:"ruleId,omitempty"`
	Level                  string                       `json:"level,omitempty"`
	DerivedSeverity        string                       `json:"derivedSeverity,omitempty"`
	SeveritySource         string                       `json:"severitySource,omitempty"`
	Location               Location                     `json:"location,omitempty"`
	Dependency             Dependency                   `json:"dependency,omitempty"`
	FindingIDs             []string                     `json:"findingIds"`
	Evidence               []Evidence                   `json:"evidence"`
	CoveredByKnownDecision *KnownDecisionCoverage       `json:"coveredByKnownDecision,omitempty"`
	PartialCoverage        bool                         `json:"partialCoverage,omitempty"`
	KnownDecisionCoverage  []KnownDecisionCoverageCount `json:"knownDecisionCoverage,omitempty"`
}

// Build liest Lauf- und Entry-Kontext. Fehlende Entry-Dateien sind der Zustand
// start; vorhandene, aber unlesbare Dateien sind ein Fehler.
func Build(options Options) (Result, error) {
	if options.RunDir == "" {
		return Result{}, errors.New("kein Laufverzeichnis angegeben")
	}
	if info, err := os.Stat(options.RunDir); err != nil {
		if os.IsNotExist(err) {
			return Result{}, fmt.Errorf("Lauf %s fehlt: %w", options.RunName, err)
		}
		return Result{}, fmt.Errorf("Lauf %s ist nicht lesbar: %w", options.RunName, err)
	} else if !info.IsDir() {
		return Result{}, fmt.Errorf("Lauf %s ist kein Verzeichnis: %s", options.RunName, options.RunDir)
	}

	run, err := review.ReadRun(options.RunDir)
	if err != nil {
		return Result{}, fmt.Errorf("%s: %w", filepath.Join(options.RunDir, review.RunFileName), err)
	}

	entries := make([]EntrySummary, 0, len(run.Entries))
	for _, entry := range run.Entries {
		context := EntrySummary{
			Name:          entry.Name,
			Kind:          entry.Kind,
			SelectedState: entry.State,
			State:         review.StateStart,
			Jobs:          []JobSummary{},
			Mode:          entryMode(entry),
		}
		status, err := review.ReadEntryStatus(options.RunDir, entry.Name)
		if err != nil {
			if os.IsNotExist(err) {
				entries = append(entries, context)
				continue
			}
			return Result{}, fmt.Errorf("%s: %w", review.EntryFile(options.RunDir, entry.Name), err)
		}
		if status.State == "" {
			status.State = review.StateStart
		}
		context.State = status.State
		context.Present = true
		context.Started = status.Started
		context.Finished = status.Finished
		context.Reason = status.Reason
		context.Jobs = summarizeJobs(status.Jobs)
		entries = append(entries, context)
	}

	severityMapping, err := LoadSeverityMapping(options.SeverityMappingPath)
	if err != nil {
		return Result{}, err
	}

	now := time.Now
	if options.Now != nil {
		now = options.Now
	}

	decisions, loadReport, err := knowndecisions.Load(options.LocalDir)
	if err != nil {
		return Result{}, err
	}
	knownMeta := KnownDecisions{Sources: loadReport.Sources, Decisions: loadReport.Decisions, Warnings: loadReport.Warnings}

	findings, err := loadFindings(options.RunDir, entries, severityMapping)
	if err != nil {
		return Result{}, err
	}
	groups := GroupFindings(findings)
	applyKnownDecisions(findings, groups, decisions, &knownMeta, now())

	version := options.KPlaybookVersion
	if version == "" {
		version = "unknown"
	}
	return Result{
		SchemaVersion:    1,
		Generated:        now().Format(time.RFC3339),
		KPlaybookVersion: version,
		Run:              runContext(options, run),
		Entries:          entries,
		Findings:         findings,
		Groups:           groups,
		KnownDecisions:   knownMeta,
	}, nil
}

// Write erzeugt review-input.json und review-input.md. Beide Dateien werden
// atomar ersetzt; Rohdaten und Entry-Dateien bleiben unangetastet.
func Write(options Options, result Result) (Output, error) {
	jsonPath := filepath.Join(options.RunDir, "review-input.json")
	markdownPath := filepath.Join(options.RunDir, "review-input.md")

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return Output{}, err
	}
	if err := writeAtomic(jsonPath, append(data, '\n')); err != nil {
		return Output{}, err
	}
	if err := writeAtomic(markdownPath, []byte(markdown(result))); err != nil {
		return Output{}, err
	}
	return Output{JSON: jsonPath, Markdown: markdownPath}, nil
}

// Run baut und schreibt die Review-Input-Artefakte eines Laufs.
func Run(options Options) (Result, Output, error) {
	result, err := Build(options)
	if err != nil {
		return Result{}, Output{}, err
	}
	output, err := Write(options, result)
	if err != nil {
		return Result{}, Output{}, err
	}
	return result, output, nil
}

func runContext(options Options, run review.Run) RunContext {
	dir := options.RunDir
	if options.ProjectDir != "" {
		if relative, err := filepath.Rel(options.ProjectDir, options.RunDir); err == nil {
			dir = filepath.ToSlash(relative)
		}
	}
	return RunContext{
		Name:            options.RunName,
		Dir:             dir,
		Created:         run.Created,
		State:           run.State,
		DerivedState:    review.DeriveRunState(options.RunDir, run),
		Languages:       append([]string{}, run.Languages...),
		SelectedEntries: append([]review.Entry{}, run.Entries...),
	}
}

// entryMode liefert die Betriebsart eines Eintrags für den Merge.
//
// Nur AI-Einträge haben eine: review.EntryMode macht aus dem leeren Feld die
// Vorgabe perspective, und die einem Tool-Eintrag zuzuschreiben behauptete eine
// Betriebsart, die es dort nicht gibt.
func entryMode(entry review.Entry) review.Mode {
	if entry.Kind != review.KindAI {
		return ""
	}
	return review.EntryMode(entry)
}

func summarizeJobs(jobs []review.JobStatus) []JobSummary {
	if jobs == nil {
		return []JobSummary{}
	}
	summaries := make([]JobSummary, 0, len(jobs))
	for _, job := range jobs {
		summaries = append(summaries, JobSummary{
			Job:        job.Job,
			State:      job.State,
			Module:     job.Module,
			ExitCode:   cloneInt(job.ExitCode),
			SARIF:      job.SARIF,
			Findings:   cloneInt(job.Findings),
			Candidates: cloneInt(job.Candidates),
			Started:    job.Started,
			Finished:   job.Finished,
			Reason:     job.Reason,
		})
	}
	return summaries
}

func cloneInt(value *int) *int {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

type sarifDocument struct {
	Runs []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool struct {
		Driver sarifToolComponent `json:"driver"`
	} `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifToolComponent struct {
	Name  string      `json:"name"`
	Rules []sarifRule `json:"rules"`
}

type sarifRule struct {
	ID               string      `json:"id"`
	Name             string      `json:"name"`
	ShortDescription sarifText   `json:"shortDescription"`
	FullDescription  sarifText   `json:"fullDescription"`
	Properties       sarifObject `json:"properties"`
}

type sarifText struct {
	Text     string `json:"text"`
	Markdown string `json:"markdown"`
}

type sarifResult struct {
	RuleID              string            `json:"ruleId"`
	RuleIndex           *int              `json:"ruleIndex"`
	Level               string            `json:"level"`
	Message             sarifText         `json:"message"`
	Locations           []sarifLocation   `json:"locations"`
	Fingerprints        map[string]string `json:"fingerprints"`
	PartialFingerprints map[string]string `json:"partialFingerprints"`
	Properties          sarifObject       `json:"properties"`
}

type sarifLocation struct {
	PhysicalLocation struct {
		ArtifactLocation struct {
			URI string `json:"uri"`
		} `json:"artifactLocation"`
		Region struct {
			StartLine   int `json:"startLine"`
			StartColumn int `json:"startColumn"`
		} `json:"region"`
	} `json:"physicalLocation"`
}

type sarifObject map[string]any

var dependencyIDPattern = regexp.MustCompile(`(?i)\b(CVE-\d{4}-\d{4,}|GHSA-[0-9a-z]{4}-[0-9a-z]{4}-[0-9a-z]{4}|OSV-[0-9A-Z-]+|PYSEC-\d{4}-\d+)\b`)

func loadFindings(runDir string, entries []EntrySummary, severityMapping SeverityMapping) ([]Finding, error) {
	findings := []Finding{}
	for _, entry := range entries {
		if entry.State != review.StateDone {
			continue
		}
		for _, job := range entry.Jobs {
			if job.State != review.StateDone || job.SARIF == "" {
				continue
			}
			jobFindings, err := readSARIF(filepath.Join(runDir, job.SARIF), entry.Name, entry.Mode, job, severityMapping)
			if err != nil {
				return nil, err
			}
			findings = append(findings, jobFindings...)
		}
	}
	return findings, nil
}

func readSARIF(path string, tool string, mode review.Mode, job JobSummary, severityMapping SeverityMapping) ([]Finding, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	var document sarifDocument
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("%s ist kein lesbares SARIF-JSON: %w", path, err)
	}

	findings := []Finding{}
	for runIndex, run := range document.Runs {
		rules := rulesByID(run.Tool.Driver.Rules)
		for resultIndex, result := range run.Results {
			rule := ruleForResult(run.Tool.Driver.Rules, rules, result)
			finding := Finding{
				ID:   fmt.Sprintf("%s:%d:%d", job.SARIF, runIndex, resultIndex),
				Mode: mode,
				Evidence: Evidence{
					Tool:        tool,
					Job:         job.Job,
					SARIF:       job.SARIF,
					RunIndex:    runIndex,
					ResultIndex: resultIndex,
				},
				RuleID:              firstNonEmpty(result.RuleID, rule.ID),
				RuleName:            firstNonEmpty(rule.Name, rule.ShortDescription.text()),
				RuleDescription:     firstNonEmpty(rule.FullDescription.text(), rule.ShortDescription.text()),
				Level:               result.Level,
				Message:             result.Message.text(),
				Location:            primaryLocation(result.Locations),
				Locations:           allLocations(result.Locations),
				Fingerprints:        cloneMap(result.Fingerprints),
				PartialFingerprints: cloneMap(result.PartialFingerprints),
			}
			finding.Dependency = extractDependency(finding, result, rule)
			finding.DerivedSeverity, finding.SeveritySource = deriveSeverity(finding, result, rule, severityMapping)
			findings = append(findings, finding)
		}
	}
	return findings, nil
}

func rulesByID(rules []sarifRule) map[string]sarifRule {
	byID := map[string]sarifRule{}
	for _, rule := range rules {
		if rule.ID != "" {
			byID[rule.ID] = rule
		}
	}
	return byID
}

func ruleForResult(rules []sarifRule, byID map[string]sarifRule, result sarifResult) sarifRule {
	if result.RuleID != "" {
		if rule, ok := byID[result.RuleID]; ok {
			return rule
		}
	}
	if result.RuleIndex != nil && *result.RuleIndex >= 0 && *result.RuleIndex < len(rules) {
		return rules[*result.RuleIndex]
	}
	return sarifRule{}
}

func (text sarifText) text() string {
	if text.Text != "" {
		return text.Text
	}
	return text.Markdown
}

func primaryLocation(locations []sarifLocation) Location {
	if len(locations) == 0 {
		return Location{}
	}
	physical := locations[0].PhysicalLocation
	return Location{
		URI:         physical.ArtifactLocation.URI,
		StartLine:   physical.Region.StartLine,
		StartColumn: physical.Region.StartColumn,
	}
}

func allLocations(locations []sarifLocation) []Location {
	if len(locations) == 0 {
		return nil
	}
	output := make([]Location, 0, len(locations))
	for _, location := range locations {
		physical := location.PhysicalLocation
		item := Location{
			URI:         physical.ArtifactLocation.URI,
			StartLine:   physical.Region.StartLine,
			StartColumn: physical.Region.StartColumn,
		}
		output = append(output, item)
	}
	return output
}

func cloneMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	output := make(map[string]string, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func extractDependency(finding Finding, result sarifResult, rule sarifRule) Dependency {
	dependency := Dependency{IDs: extractIDs(strings.Join([]string{
		finding.RuleID,
		finding.RuleName,
		finding.RuleDescription,
		finding.Message,
		propertiesText(result.Properties),
		propertiesText(rule.Properties),
	}, "\n"))}

	for _, properties := range []sarifObject{result.Properties, rule.Properties} {
		dependency.Package = firstNonEmpty(dependency.Package,
			stringProperty(properties, "package"), stringProperty(properties, "packageName"),
			stringProperty(properties, "dependency"), stringProperty(properties, "artifactName"),
			stringProperty(properties, "name"))
		dependency.Version = firstNonEmpty(dependency.Version,
			stringProperty(properties, "version"), stringProperty(properties, "installedVersion"),
			stringProperty(properties, "packageVersion"))
		dependency.Manifest = firstNonEmpty(dependency.Manifest,
			stringProperty(properties, "manifest"), stringProperty(properties, "manifestPath"),
			stringProperty(properties, "target"), stringProperty(properties, "file"),
			stringProperty(properties, "path"))
	}

	if dependency.Manifest == "" {
		dependency.Manifest = finding.Location.URI
	}

	// Rückfall 1, strukturiert: der purl. grype ist das einzige Werkzeug des
	// Messlaufs aus Task 028, das Paket und Version überhaupt in einem Feld
	// nennt, und es tut es unter rule.properties.purls — als Array. Der Wert
	// bleibt damit ein Feld mit einem Wert je Eintrag und darf in den harten
	// Schlüssel.
	if dependency.Package == "" {
		for _, properties := range []sarifObject{result.Properties, rule.Properties} {
			for _, purl := range stringListProperty(properties, "purl", "purls") {
				name, version := parsePurl(purl)
				if name == "" {
					continue
				}
				dependency.Package = name
				dependency.Version = firstNonEmpty(dependency.Version, version)
				break
			}
			if dependency.Package != "" {
				break
			}
		}
	}

	// Rückfall 2, Freitext: nur für die Anzeige. osv-scanner und trivy nennen
	// Paket und Version ausschließlich in der Meldung ("Package 'requests@2.19.0'
	// is vulnerable …", "Package: requests\nInstalled Version: 2.19.0"). Beide
	// Quellen liegen bereits in der Finding-Struktur; rule.help wird bewusst
	// nicht gelesen, es trüge nichts bei, was die Meldung nicht schon nennt, und
	// verlangte eine Erweiterung der SARIF-Structs.
	if dependency.Package == "" || dependency.Version == "" {
		name, version := packageFromText(finding.Message, finding.RuleDescription)
		if dependency.Package == "" {
			dependency.TextPackage = name
		}
		if dependency.Version == "" {
			dependency.TextVersion = version
		}
	}

	// Die enge Menge für den harten Schlüssel: RuleID plus die benannten
	// Kennungsfelder der Properties. Der Freitext bleibt draußen — Message,
	// Beschreibung und das Properties-JSON als Ganzes tragen regelmäßig
	// Fremd-Kennungen aus Referenz- und Fixed-in-Listen mit sich.
	dependency.KeyIDs = extractIDs(strings.Join([]string{
		finding.RuleID,
		propertyIDText(result.Properties),
		propertyIDText(rule.Properties),
	}, "\n"))
	if len(dependency.KeyIDs) == 0 {
		// Rückfall auf die breite Menge. Ohne ihn gruppierte ein Werkzeug,
		// dessen einzige Kennung in Message oder Beschreibung steht, nach der
		// Einengung schlechter als vorher — es hätte gar keinen Schlüssel mehr.
		dependency.KeyIDs = dependency.IDs
	}

	if dependency.Package == "" && len(dependency.IDs) == 0 && dependency.Version == "" {
		return Dependency{}
	}
	return dependency
}

func extractIDs(text string) []string {
	seen := map[string]bool{}
	ids := []string{}
	for _, match := range dependencyIDPattern.FindAllString(text, -1) {
		id := strings.ToUpper(match)
		if seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// dependencyKeyIDProperties sind die Property-Namen, unter denen ein Werkzeug
// die Kennungen des Befunds selbst ablegt — im Unterschied zu Kennungen, die im
// Advisory-Text nebenbei vorkommen.
//
// Erhoben an einem echten Lauf (Task 027, Etappe 1): pip-audit schreibt seine
// eigene Kennung nach id und die Aliase nach aliases, während grype,
// osv-scanner und trivy ihre Hauptkennung ausschließlich in ruleId tragen. Die
// übrigen Namen sind gebräuchliche Schreibweisen derselben Sache; sie kosten
// nichts, solange sie benannte Felder bleiben und nicht der Freitext.
var dependencyKeyIDProperties = []string{
	"id", "ids", "alias", "aliases", "identifiers",
	"cve", "cveId", "ghsa", "osv",
	"vulnerabilityId", "vulnId", "advisoryId",
}

// propertyIDText sammelt den Text der benannten Kennungsfelder eines
// Properties-Objekts. Anders als propertiesText gibt es nicht das ganze JSON
// heraus, sondern nur die Felder, deren Inhalt den Befund benennt.
func propertyIDText(properties sarifObject) string {
	if len(properties) == 0 {
		return ""
	}
	parts := make([]string, 0, len(dependencyKeyIDProperties))
	for _, key := range dependencyKeyIDProperties {
		if value := stringProperty(properties, key); value != "" {
			parts = append(parts, value)
		}
	}
	return strings.Join(parts, "\n")
}

func propertiesText(properties sarifObject) string {
	if len(properties) == 0 {
		return ""
	}
	data, err := json.Marshal(properties)
	if err != nil {
		return ""
	}
	return string(data)
}

func stringProperty(properties sarifObject, key string) string {
	value, ok := properties[key]
	if !ok {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	default:
		return fmt.Sprint(typed)
	}
}

// stringListProperty liest ein Property, das eine Liste sein darf, als Liste
// von Strings.
//
// Nötig, weil stringProperty für alles, was kein String und keine Zahl ist, auf
// fmt.Sprint zurückfällt: aus ["pkg:pypi/requests@2.19.0"] würde dort
// `[pkg:pypi/requests@2.19.0]` samt Klammern, und der purl-Zerleger bekäme
// einen Wert, den kein Werkzeug je geschrieben hat.
func stringListProperty(properties sarifObject, keys ...string) []string {
	values := []string{}
	for _, key := range keys {
		value, ok := properties[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case string:
			if typed != "" {
				values = append(values, typed)
			}
		case []any:
			for _, entry := range typed {
				if text, ok := entry.(string); ok && text != "" {
					values = append(values, text)
				}
			}
		case []string:
			for _, text := range typed {
				if text != "" {
					values = append(values, text)
				}
			}
		}
	}
	return values
}

// textPackageVersionPattern trifft die Form, in der osv-scanner Paket und
// Version zusammen nennt: `Package 'requests@2.19.0' is vulnerable to …`.
var textPackageVersionPattern = regexp.MustCompile(`(?i)\bpackages?\b[\s:='"]*([A-Za-z0-9][A-Za-z0-9._/-]*)@v?([0-9][0-9A-Za-z._+-]*)`)

// textPackagePattern trifft die getrennte Form, in der trivy das Paket nennt:
// `Package: requests`.
var textPackagePattern = regexp.MustCompile(`(?i)\bpackages?\b[\s:='"]*([A-Za-z0-9][A-Za-z0-9._/-]*)`)

// textVersionPattern trifft `Installed Version: 2.19.0` und `version 2.19.0`.
//
// Die erste Gruppe fängt ein vorangestelltes „fixed", „fix", „patched" oder
// „affected" mit ab. Sie wird nicht verworfen, sondern ausgewertet: RE2 kennt
// keinen Lookbehind, und ohne diese Unterscheidung läse der Rückfall aus trivys
// Meldung die *behobene* Version statt der installierten.
var textVersionPattern = regexp.MustCompile(`(?i)\b(fixed|fix|patched|affected)?\s*(?:installed\s+)?versions?\b[\s:='"]*v?([0-9][0-9A-Za-z._+-]*)`)

// packageFromText liest Paketnamen und Version aus Fließtext. Das Ergebnis ist
// ausdrücklich **nicht** identitätstragend: es füllt TextPackage/TextVersion
// und darf nie in den harten Dedupe-Schlüssel (siehe Dependency).
//
// Gelesen wird der erste Text, der überhaupt etwas hergibt — in der Praxis die
// Meldung; die Rule-Beschreibung ist der Rückfall für Werkzeuge, die ihre
// Meldung knapp halten.
func packageFromText(texts ...string) (string, string) {
	for _, text := range texts {
		if text == "" {
			continue
		}
		if match := textPackageVersionPattern.FindStringSubmatch(text); match != nil {
			return normalizePackageName(match[1]), strings.ToLower(match[2])
		}
		name := ""
		if match := textPackagePattern.FindStringSubmatch(text); match != nil {
			name = normalizePackageName(match[1])
		}
		version := ""
		for _, match := range textVersionPattern.FindAllStringSubmatch(text, -1) {
			if match[1] != "" {
				continue
			}
			version = strings.ToLower(match[2])
			break
		}
		if name != "" || version != "" {
			return name, version
		}
	}
	return "", ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
