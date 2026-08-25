// Package knowndecisions lädt und matcht bewusst getroffene Review-Entscheidungen.
package knowndecisions

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const fileName = "known-decisions.md"

// legacyResultsDirName ist der alte Ort: k-playbook-local/results/known-decisions.md.
// Die Datei ist eine handgepflegte Eingabe und kein Review-Ergebnis, deshalb liegt
// sie jetzt eine Ebene höher, direkt in k-playbook-local/.
//
// Der alte Ort wird nur übergangsweise weiter gelesen. Diese Konstante, resolvePath
// und die beiden Warnungen darin werden zum 2027-02-28 ersatzlos entfernt; der
// Ausbau steht als Eintrag in k-playbook-local/TODO.md.
const legacyResultsDirName = "results"

var validCategories = map[string]bool{
	"false-positive": true,
	"accepted-risk":  true,
	"deferred":       true,
	"wontfix":        true,
}

// Decision ist ein maschinenlesbarer Eintrag aus known-decisions.md.
type Decision struct {
	ID       string
	Category string
	Expires  string
	Owner    string
	Match    []Criterion
	Reason   string
	Source   string
}

// Criterion beschreibt eine OR-Bedingung eines Decision-Eintrags.
type Criterion struct {
	StableID     string
	RuleID       string
	Location     string
	CVEID        string
	GHSAID       string
	OSVID        string
	Package      string
	Version      string
	ManifestGlob string
	PathGlob     string
}

// SourceReport beschreibt, ob eine bekannte Datei geladen wurde.
type SourceReport struct {
	Path    string `json:"path"`
	Scope   string `json:"scope"`
	Loaded  bool   `json:"loaded"`
	Missing bool   `json:"missing"`
}

// DecisionReport ist die Herkunfts- und Anwendungs-Metainformation zu einer Decision.
type DecisionReport struct {
	ID               string `json:"id"`
	Category         string `json:"category"`
	Source           string `json:"source"`
	Path             string `json:"path"`
	Expires          string `json:"expires,omitempty"`
	Expired          bool   `json:"expired"`
	Applied          bool   `json:"applied"`
	NotAppliedReason string `json:"notAppliedReason,omitempty"`
}

// LoadReport ist das sichtbare Ergebnis des Ladens.
type LoadReport struct {
	Sources   []SourceReport
	Decisions []DecisionReport
	Warnings  []string
}

// Finding ist das minimale Matchmodell für Review-Findings.
type Finding struct {
	StableID   string
	RuleID     string
	Locations  []Location
	Dependency Dependency
}

// Location ist ein projektrelativer Finding-Ort.
type Location struct {
	Path        string
	Line        int
	Column      int
	Description string
}

// Dependency beschreibt Dependency-Metadaten für CVE-/GHSA-/OSV-Matches.
type Dependency struct {
	Package  string
	Version  string
	Manifest string
	IDs      []string
}

// MatchResult nennt primäre und sekundäre Treffer einer Finding-Prüfung.
type MatchResult struct {
	Decision  Decision
	MatchedBy string
	Location  string
	Secondary []Decision
}

type sourceDecision struct {
	decision Decision
	path     string
}

// Load liest die projektweite known-decisions.md aus k-playbook-local/.
//
// Sources bleibt eine Liste, obwohl heute nur eine Quelle darin steht: der
// Report ist auf mehrere Quellen ausgelegt und soll das bleiben.
func Load(localDir string) ([]Decision, LoadReport, error) {
	projectPath, transitionWarnings := resolvePath(localDir)
	report := LoadReport{
		Sources:  []SourceReport{{Path: projectPath, Scope: "project"}},
		Warnings: transitionWarnings,
	}

	byID := map[string]sourceDecision{}
	for index, source := range report.Sources {
		decisions, loaded, warnings, err := loadFile(source.Path, source.Scope)
		if err != nil {
			return nil, report, err
		}
		report.Warnings = append(report.Warnings, warnings...)
		report.Sources[index].Loaded = loaded
		report.Sources[index].Missing = !loaded
		if !loaded {
			continue
		}
		for _, decision := range decisions {
			if existing, ok := byID[decision.ID]; ok {
				report.Warnings = append(report.Warnings, fmt.Sprintf("%s: doppelte Decision-ID %q verdrängt nicht %s", source.Path, decision.ID, existing.path))
				continue
			}
			byID[decision.ID] = sourceDecision{decision: decision, path: source.Path}
		}
	}

	decisions := make([]Decision, 0, len(byID))
	for _, item := range byID {
		decisions = append(decisions, item.decision)
		report.Decisions = append(report.Decisions, DecisionReport{
			ID:       item.decision.ID,
			Category: item.decision.Category,
			Source:   item.decision.Source,
			Path:     item.path,
			Expires:  item.decision.Expires,
		})
	}
	sort.Slice(decisions, func(left, right int) bool { return decisions[left].ID < decisions[right].ID })
	sort.Slice(report.Decisions, func(left, right int) bool {
		if report.Decisions[left].ID == report.Decisions[right].ID {
			return report.Decisions[left].Source < report.Decisions[right].Source
		}
		return report.Decisions[left].ID < report.Decisions[right].ID
	})
	return decisions, report, nil
}

// resolvePath wählt den zu lesenden Ort und meldet den Umzug.
//
// Übergangscode bis 2027-02-28: existiert nur der alte Ort, wird er gelesen und
// der Umzug gemeldet. Existieren beide, gewinnt der neue Ort und der alte wird
// als ignoriert gemeldet. Der Warntext nennt bewusst kein Datum — durchsetzbar
// ist die Frist nicht, und ein sichtbares Datum würde zur Falschaussage, sobald
// der Ausbau verrutscht.
func resolvePath(localDir string) (string, []string) {
	if localDir == "" {
		return "", nil
	}
	current := filepath.Join(localDir, fileName)
	legacy := filepath.Join(localDir, legacyResultsDirName, fileName)

	currentExists := fileExists(current)
	legacyExists := fileExists(legacy)
	switch {
	case currentExists && legacyExists:
		return current, []string{fmt.Sprintf(
			"%s wird gelesen; %s liegt am alten Ort und wird ignoriert.", current, legacy)}
	case legacyExists:
		return legacy, []string{fmt.Sprintf(
			"%s liegt am alten Ort und wird künftig nicht mehr gelesen. Verschiebe die Datei nach %s.", legacy, current)}
	default:
		return current, nil
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func loadFile(path string, source string) ([]Decision, bool, []string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, false, nil, nil
		}
		return nil, false, nil, err
	}
	decisions, warnings := ParseMarkdown(string(data), path, source)
	return decisions, true, warnings, nil
}

// ParseMarkdown liest das Markdown-Format mit einem ##-Header je Eintrag.
func ParseMarkdown(content string, path string, source string) ([]Decision, []string) {
	sections := splitSections(content)
	decisions := []Decision{}
	warnings := []string{}
	for _, section := range sections {
		decision, sectionWarnings := parseSection(section, path, source)
		warnings = append(warnings, sectionWarnings...)
		if decision.ID != "" && len(sectionWarnings) == 0 {
			decisions = append(decisions, decision)
		}
	}
	sort.Slice(decisions, func(left, right int) bool { return decisions[left].ID < decisions[right].ID })
	return decisions, warnings
}

type section struct {
	header string
	body   string
}

func splitSections(content string) []section {
	sections := []section{}
	var current *section
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "## ") && !strings.HasPrefix(line, "### ") {
			if current != nil {
				sections = append(sections, *current)
			}
			current = &section{header: strings.TrimSpace(strings.TrimPrefix(line, "## "))}
			continue
		}
		if current != nil {
			current.body += line + "\n"
		}
	}
	if current != nil {
		sections = append(sections, *current)
	}
	return sections
}

func parseSection(sec section, path string, source string) (Decision, []string) {
	warnings := []string{}
	yamlBlock, reason, ok := extractYAML(sec.body)
	if !ok {
		return Decision{}, []string{fmt.Sprintf("%s: Decision %q hat keinen eindeutigen yaml-Block", path, sec.header)}
	}
	decision, parseWarnings := parseDecisionYAML(yamlBlock)
	warnings = append(warnings, parseWarnings...)
	decision.Reason = strings.TrimSpace(reason)
	decision.Source = source
	if decision.ID == "" {
		warnings = append(warnings, fmt.Sprintf("%s: Decision %q ohne id", path, sec.header))
	}
	if sec.header != decision.ID {
		warnings = append(warnings, fmt.Sprintf("%s: Header %q entspricht nicht id %q", path, sec.header, decision.ID))
	}
	if !validCategories[decision.Category] {
		warnings = append(warnings, fmt.Sprintf("%s: Decision %q hat ungültige category %q", path, sec.header, decision.Category))
	}
	if len(decision.Match) == 0 {
		warnings = append(warnings, fmt.Sprintf("%s: Decision %q hat keine match-Kriterien", path, sec.header))
	}
	for index, criterion := range decision.Match {
		if warning := validateCriterion(criterion); warning != "" {
			warnings = append(warnings, fmt.Sprintf("%s: Decision %q match[%d]: %s", path, sec.header, index, warning))
		}
	}
	if decision.Expires != "" {
		if _, err := time.Parse("2006-01-02", decision.Expires); err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: Decision %q hat ungültiges expires %q", path, sec.header, decision.Expires))
		}
	}
	return decision, warnings
}

func extractYAML(body string) (string, string, bool) {
	lines := strings.Split(body, "\n")
	start := -1
	end := -1
	for index, line := range lines {
		if strings.TrimSpace(line) == "```yaml" {
			if start >= 0 {
				return "", "", false
			}
			start = index
			continue
		}
		if start >= 0 && strings.TrimSpace(line) == "```" {
			end = index
			break
		}
	}
	if start < 0 || end < 0 {
		return "", "", false
	}
	for _, line := range lines[end+1:] {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			return "", "", false
		}
	}
	return strings.Join(lines[start+1:end], "\n"), strings.Join(lines[end+1:], "\n"), true
}

func parseDecisionYAML(content string) (Decision, []string) {
	decision := Decision{}
	warnings := []string{}
	lines := strings.Split(content, "\n")
	inMatch := false
	var current *Criterion
	flush := func() {
		if current != nil {
			decision.Match = append(decision.Match, *current)
			current = nil
		}
	}
	for _, raw := range lines {
		line := strings.TrimRight(raw, " \t")
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		if !strings.HasPrefix(line, " ") && strings.HasSuffix(line, ":") {
			key := strings.TrimSuffix(strings.TrimSpace(line), ":")
			inMatch = key == "match"
			if !inMatch {
				flush()
			}
			continue
		}
		if !inMatch && !strings.HasPrefix(line, " ") {
			key, value, ok := parseKeyValue(line)
			if !ok {
				warnings = append(warnings, fmt.Sprintf("nicht lesbare YAML-Zeile %q", line))
				continue
			}
			setDecisionField(&decision, key, value)
			continue
		}
		if inMatch {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "- ") {
				flush()
				current = &Criterion{}
				key, value, ok := parseKeyValue(strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")))
				if !ok {
					warnings = append(warnings, fmt.Sprintf("nicht lesbares match-Kriterium %q", line))
					continue
				}
				setCriterionField(current, key, value)
				continue
			}
			if current == nil {
				warnings = append(warnings, fmt.Sprintf("match-Feld ohne Listeneintrag %q", line))
				continue
			}
			key, value, ok := parseKeyValue(trimmed)
			if !ok {
				warnings = append(warnings, fmt.Sprintf("nicht lesbares match-Feld %q", line))
				continue
			}
			setCriterionField(current, key, value)
		}
	}
	flush()
	return decision, warnings
}

func parseKeyValue(line string) (string, string, bool) {
	before, after, ok := strings.Cut(line, ":")
	if !ok {
		return "", "", false
	}
	key := strings.TrimSpace(before)
	value := strings.TrimSpace(after)
	value = strings.Trim(value, `"'`)
	return key, value, key != ""
}

func setDecisionField(decision *Decision, key string, value string) {
	switch key {
	case "id":
		decision.ID = value
	case "category":
		decision.Category = value
	case "expires":
		decision.Expires = value
	case "owner":
		decision.Owner = value
	}
}

func setCriterionField(criterion *Criterion, key string, value string) {
	switch key {
	case "stableId":
		criterion.StableID = normalizePath(value)
	case "ruleId":
		criterion.RuleID = value
	case "location":
		criterion.Location = normalizePath(value)
	case "cveId":
		criterion.CVEID = normalizeID(value)
	case "ghsaId":
		criterion.GHSAID = normalizeID(value)
	case "osvId":
		criterion.OSVID = normalizeID(value)
	case "package":
		criterion.Package = value
	case "version":
		criterion.Version = value
	case "manifestGlob":
		criterion.ManifestGlob = normalizePath(value)
	case "pathGlob":
		criterion.PathGlob = normalizePath(value)
	}
}

func validateCriterion(criterion Criterion) string {
	switch {
	case criterion.StableID != "":
		return ""
	case criterion.PathGlob != "":
		return ""
	case criterion.RuleID != "":
		if criterion.Location == "" {
			return "ruleId ohne location ist verboten"
		}
		return ""
	case criterion.CVEID != "" || criterion.GHSAID != "" || criterion.OSVID != "":
		if criterion.Package == "" && criterion.Version == "" && criterion.ManifestGlob == "" && criterion.StableID == "" {
			return "Dependency-ID ohne Scope ist verboten"
		}
		return ""
	default:
		return "leeres oder unbekanntes Kriterium"
	}
}

// Match gibt die erste passende, nicht abgelaufene Decision zurück.
func Match(finding Finding, decisions []Decision, now time.Time) *MatchResult {
	var result *MatchResult
	for _, decision := range decisions {
		if Expired(decision, now) {
			continue
		}
		matchedBy, location := matchDecision(finding, decision)
		if matchedBy == "" {
			continue
		}
		if result == nil {
			result = &MatchResult{Decision: decision, MatchedBy: matchedBy, Location: location}
			continue
		}
		result.Secondary = append(result.Secondary, decision)
	}
	return result
}

// Expired sagt, ob expires vor dem Prüftag liegt oder auf ihn fällt.
func Expired(decision Decision, now time.Time) bool {
	if decision.Expires == "" {
		return false
	}
	expires, err := time.Parse("2006-01-02", decision.Expires)
	if err != nil {
		return false
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return !expires.After(today)
}

func matchDecision(finding Finding, decision Decision) (string, string) {
	for _, criterion := range decision.Match {
		if criterion.StableID != "" && !hasDependencyID(criterion) && criterion.StableID == finding.StableID {
			return "stableId", ""
		}
		if criterion.PathGlob != "" {
			for _, location := range finding.Locations {
				path := normalizePath(location.Path)
				if globMatch(criterion.PathGlob, path) {
					return "pathGlob", path
				}
			}
		}
		if criterion.RuleID != "" && criterion.RuleID == finding.RuleID {
			for _, location := range finding.Locations {
				path := normalizePath(location.Path)
				if globMatch(criterion.Location, path) {
					return "ruleId+location", path
				}
			}
		}
		if dependencyIDMatches(criterion, finding.Dependency.IDs) && dependencyScopeMatches(criterion, finding) {
			return dependencyMatchedBy(criterion), normalizePath(finding.Dependency.Manifest)
		}
	}
	return "", ""
}

func hasDependencyID(criterion Criterion) bool {
	return criterion.CVEID != "" || criterion.GHSAID != "" || criterion.OSVID != ""
}

func dependencyIDMatches(criterion Criterion, ids []string) bool {
	for _, id := range ids {
		normalized := normalizeID(id)
		if criterion.CVEID != "" && normalized == normalizeID(criterion.CVEID) {
			return true
		}
		if criterion.GHSAID != "" && normalized == normalizeID(criterion.GHSAID) {
			return true
		}
		if criterion.OSVID != "" && normalized == normalizeID(criterion.OSVID) {
			return true
		}
	}
	return false
}

func normalizeID(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func dependencyScopeMatches(criterion Criterion, finding Finding) bool {
	if criterion.StableID != "" && criterion.StableID == finding.StableID {
		return true
	}
	dependency := finding.Dependency
	if criterion.Package != "" && !strings.EqualFold(criterion.Package, dependency.Package) {
		return false
	}
	if criterion.Version != "" && criterion.Version != dependency.Version {
		return false
	}
	if criterion.ManifestGlob != "" && !globMatch(criterion.ManifestGlob, normalizePath(dependency.Manifest)) {
		return false
	}
	return criterion.Package != "" || criterion.Version != "" || criterion.ManifestGlob != "" || criterion.StableID != ""
}

func dependencyMatchedBy(criterion Criterion) string {
	if criterion.CVEID != "" {
		return "cveId"
	}
	if criterion.GHSAID != "" {
		return "ghsaId"
	}
	return "osvId"
}

func globMatch(pattern string, value string) bool {
	pattern = normalizePath(pattern)
	value = normalizePath(value)
	if pattern == "" || value == "" {
		return false
	}
	return matchSegments(strings.Split(pattern, "/"), strings.Split(value, "/"))
}

func matchSegments(pattern []string, value []string) bool {
	if len(pattern) == 0 {
		return len(value) == 0
	}
	if pattern[0] == "**" {
		if matchSegments(pattern[1:], value) {
			return true
		}
		return len(value) > 0 && matchSegments(pattern, value[1:])
	}
	if len(value) == 0 {
		return false
	}
	matched, err := filepath.Match(pattern[0], value[0])
	if err != nil || !matched {
		return false
	}
	return matchSegments(pattern[1:], value[1:])
}

func normalizePath(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	path = strings.TrimPrefix(path, "file://")
	path = strings.TrimPrefix(path, "./")
	path = strings.TrimPrefix(path, "/")
	return strings.ToLower(path)
}
