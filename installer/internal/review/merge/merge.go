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
	Name          string             `json:"name"`
	Kind          review.Kind        `json:"kind"`
	SelectedState review.State       `json:"selectedState,omitempty"`
	State         review.State       `json:"state"`
	Present       bool               `json:"present"`
	Started       string             `json:"started,omitempty"`
	Finished      string             `json:"finished,omitempty"`
	Reason        string             `json:"reason,omitempty"`
	Jobs          []JobSummary       `json:"jobs"`
	Source        review.EntryStatus `json:"source,omitempty"`
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
}

// Location ist die primäre Stelle eines SARIF-Results.
type Location struct {
	URI         string `json:"uri,omitempty"`
	StartLine   int    `json:"startLine,omitempty"`
	StartColumn int    `json:"startColumn,omitempty"`
}

// Dependency beschreibt einen Dependency-Befund, soweit er aus SARIF erkennbar
// ist.
type Dependency struct {
	Package  string   `json:"package,omitempty"`
	Version  string   `json:"version,omitempty"`
	Manifest string   `json:"manifest,omitempty"`
	IDs      []string `json:"ids,omitempty"`
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

// Finding ist die normalisierte Form eines SARIF-Results.
type Finding struct {
	ID                  string            `json:"id"`
	Evidence            Evidence          `json:"evidence"`
	RuleID              string            `json:"ruleId,omitempty"`
	RuleName            string            `json:"ruleName,omitempty"`
	RuleDescription     string            `json:"ruleDescription,omitempty"`
	Level               string            `json:"level,omitempty"`
	Message             string            `json:"message,omitempty"`
	Location            Location          `json:"location,omitempty"`
	Fingerprints        map[string]string `json:"fingerprints,omitempty"`
	PartialFingerprints map[string]string `json:"partialFingerprints,omitempty"`
	Dependency          Dependency        `json:"dependency,omitempty"`
}

// Group ist eine Dedupe-Gruppe. Sie löscht keine Findings: alle Belege und die
// Finding-IDs bleiben erhalten.
type Group struct {
	ID                 string     `json:"id"`
	DedupeRules        []string   `json:"dedupeRules,omitempty"`
	PossibleDuplicates []string   `json:"possibleDuplicates,omitempty"`
	Title              string     `json:"title,omitempty"`
	RuleID             string     `json:"ruleId,omitempty"`
	Level              string     `json:"level,omitempty"`
	Location           Location   `json:"location,omitempty"`
	Dependency         Dependency `json:"dependency,omitempty"`
	FindingIDs         []string   `json:"findingIds"`
	Evidence           []Evidence `json:"evidence"`
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
		context.Source = status
		entries = append(entries, context)
	}

	findings, err := loadFindings(options.RunDir, entries)
	if err != nil {
		return Result{}, err
	}
	groups := GroupFindings(findings)

	now := time.Now
	if options.Now != nil {
		now = options.Now
	}
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

func loadFindings(runDir string, entries []EntrySummary) ([]Finding, error) {
	findings := []Finding{}
	for _, entry := range entries {
		if entry.State != review.StateDone {
			continue
		}
		for _, job := range entry.Source.Jobs {
			if job.State != review.StateDone || job.SARIF == "" {
				continue
			}
			jobFindings, err := readSARIF(filepath.Join(runDir, job.SARIF), entry.Name, job)
			if err != nil {
				return nil, err
			}
			findings = append(findings, jobFindings...)
		}
	}
	return findings, nil
}

func readSARIF(path string, tool string, job review.JobStatus) ([]Finding, error) {
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
				ID: fmt.Sprintf("%s:%d:%d", job.SARIF, runIndex, resultIndex),
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
				Fingerprints:        cloneMap(result.Fingerprints),
				PartialFingerprints: cloneMap(result.PartialFingerprints),
			}
			finding.Dependency = extractDependency(finding, result, rule)
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
