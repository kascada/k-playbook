package merge

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/kascada/k-playbook/installer/internal/knowndecisions"
)

func applyKnownDecisions(findings []Finding, groups []Group, decisions []knowndecisions.Decision, meta *KnownDecisions, now time.Time) {
	applied := map[string]bool{}
	if len(decisions) == 0 {
		markKnownDecisionReasons(meta, applied, now)
		return
	}
	stableIDByFinding := map[string]string{}
	for _, group := range groups {
		for _, findingID := range group.FindingIDs {
			stableIDByFinding[findingID] = group.StableID
		}
	}
	for index := range findings {
		match := knowndecisions.Match(knownDecisionFinding(findings[index], stableIDByFinding[findings[index].ID]), decisions, now)
		if match == nil {
			continue
		}
		findings[index].CoveredByKnownDecision = &KnownDecisionCoverage{
			ID:        match.Decision.ID,
			Category:  match.Decision.Category,
			MatchedBy: match.MatchedBy,
		}
		applied[match.Decision.ID] = true
	}
	coverageByGroup(groups, findings)
	markKnownDecisionReasons(meta, applied, now)
}

func knownDecisionFinding(finding Finding, stableID string) knowndecisions.Finding {
	locations := make([]knowndecisions.Location, 0, len(finding.Locations))
	if len(finding.Locations) == 0 && finding.Location.URI != "" {
		locations = append(locations, knownDecisionLocation(finding.Location))
	}
	for _, location := range finding.Locations {
		locations = append(locations, knownDecisionLocation(location))
	}
	return knowndecisions.Finding{
		StableID:  stableID,
		RuleID:    finding.RuleID,
		Locations: locations,
		Dependency: knowndecisions.Dependency{
			Package:  finding.Dependency.Package,
			Version:  finding.Dependency.Version,
			Manifest: finding.Dependency.Manifest,
			IDs:      append([]string{}, finding.Dependency.IDs...),
		},
	}
}

func knownDecisionLocation(location Location) knowndecisions.Location {
	return knowndecisions.Location{Path: location.URI, Line: location.StartLine, Column: location.StartColumn}
}

func coverageByGroup(groups []Group, findings []Finding) {
	byID := map[string]Finding{}
	for _, finding := range findings {
		byID[finding.ID] = finding
	}
	for groupIndex := range groups {
		counts := map[string]KnownDecisionCoverageCount{}
		matchedBy := map[string]string{}
		covered := 0
		for _, findingID := range groups[groupIndex].FindingIDs {
			coverage := byID[findingID].CoveredByKnownDecision
			if coverage == nil {
				continue
			}
			covered++
			count := counts[coverage.ID]
			count.ID = coverage.ID
			count.Category = coverage.Category
			count.Findings++
			counts[coverage.ID] = count
			matchedBy[coverage.ID] = coverage.MatchedBy
		}
		if covered == 0 {
			continue
		}
		groups[groupIndex].KnownDecisionCoverage = sortedCoverageCounts(counts)
		if covered != len(groups[groupIndex].FindingIDs) || len(groups[groupIndex].KnownDecisionCoverage) != 1 {
			groups[groupIndex].PartialCoverage = true
			continue
		}
		coverage := groups[groupIndex].KnownDecisionCoverage[0]
		groups[groupIndex].CoveredByKnownDecision = &KnownDecisionCoverage{
			ID:        coverage.ID,
			Category:  coverage.Category,
			MatchedBy: matchedBy[coverage.ID],
		}
	}
}

func sortedCoverageCounts(counts map[string]KnownDecisionCoverageCount) []KnownDecisionCoverageCount {
	ids := make([]string, 0, len(counts))
	for id := range counts {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	output := make([]KnownDecisionCoverageCount, 0, len(ids))
	for _, id := range ids {
		output = append(output, counts[id])
	}
	return output
}

func markKnownDecisionReasons(meta *KnownDecisions, applied map[string]bool, now time.Time) {
	for index := range meta.Decisions {
		decision := knowndecisions.Decision{ID: meta.Decisions[index].ID, Expires: meta.Decisions[index].Expires}
		if knowndecisions.Expired(decision, now) {
			meta.Decisions[index].Expired = true
			meta.Decisions[index].Applied = false
			meta.Decisions[index].NotAppliedReason = "abgelaufen"
			continue
		}
		if applied[meta.Decisions[index].ID] {
			meta.Decisions[index].Applied = true
			continue
		}
		meta.Decisions[index].Applied = false
		if meta.Decisions[index].NotAppliedReason == "" {
			meta.Decisions[index].NotAppliedReason = "kein Finding getroffen"
		}
	}
	if len(meta.Decisions) == 0 {
		meta.Warnings = append(meta.Warnings, knownDecisionsEmptyWarning)
	}
}

// knownDecisionsEmptyWarning ist der Sentinel, den markKnownDecisionReasons bei
// leerem Ergebnis anhängt.
const knownDecisionsEmptyWarning = "keine known-decisions geladen"

// KnownDecisionWarnings sind die Warnungen zum Laden der known-decisions.md ohne
// diesen Sentinel. Dass keine Decisions geladen wurden, steht in jedem Bericht
// bereits an eigener Stelle; im Warnungsblock stünde der Satz ein zweites Mal.
func KnownDecisionWarnings(result Result) []string {
	warnings := make([]string, 0, len(result.KnownDecisions.Warnings))
	for _, warning := range result.KnownDecisions.Warnings {
		if warning == knownDecisionsEmptyWarning {
			continue
		}
		warnings = append(warnings, warning)
	}
	return warnings
}

func knownDecisionLabel(coverage *KnownDecisionCoverage) string {
	if coverage == nil {
		return ""
	}
	return fmt.Sprintf("%s (%s)", coverage.ID, coverage.Category)
}

func knownDecisionCoverageText(group Group) string {
	if group.CoveredByKnownDecision != nil {
		return knownDecisionLabel(group.CoveredByKnownDecision)
	}
	if group.PartialCoverage {
		parts := make([]string, 0, len(group.KnownDecisionCoverage))
		for _, coverage := range group.KnownDecisionCoverage {
			parts = append(parts, fmt.Sprintf("%s (%s): %d", coverage.ID, coverage.Category, coverage.Findings))
		}
		return "Teildeckung: " + strings.Join(parts, ", ")
	}
	return ""
}
