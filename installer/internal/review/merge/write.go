package merge

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/knowndecisions"
)

const markdownGroupLimit = 80

func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	name := temp.Name()
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		os.Remove(name)
		return err
	}
	if err := temp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Chmod(name, 0o644); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}

func markdown(result Result) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "# Review-Input %s\n\n", result.Run.Name)
	fmt.Fprintf(&builder, "Erzeugt: %s\n\n", result.Generated)
	fmt.Fprintf(&builder, "Lauf: `%s` (%s), Zustand: `%s`, abgeleitet: `%s`\n\n",
		result.Run.Dir, strings.Join(result.Run.Languages, ", "), result.Run.State, result.Run.DerivedState)

	statusCounts := countEntries(result.Entries)
	fmt.Fprintf(&builder, "## Überblick\n\n")
	fmt.Fprintf(&builder, "- Einträge: %d (%s)\n", len(result.Entries), formatCounts(statusCounts))
	fmt.Fprintf(&builder, "- Findings: %d\n", len(result.Findings))
	fmt.Fprintf(&builder, "- Gruppen: %d\n", len(result.Groups))
	fullCoverage, partialCoverage := countGroupCoverage(result.Groups)
	fmt.Fprintf(&builder, "- Gruppen mit vollständiger Deckung: %d von %d\n", fullCoverage, len(result.Groups))
	if partialCoverage > 0 {
		fmt.Fprintf(&builder, "- Gruppen mit Teildeckung: %d\n", partialCoverage)
	}
	fmt.Fprintf(&builder, "- Vollständige Belege: `review-input.json`\n\n")
	if len(result.KnownDecisions.Decisions) == 0 {
		fmt.Fprintf(&builder, "Known-Decisions: keine known-decisions geladen.\n\n")
	} else {
		fmt.Fprintf(&builder, "Known-Decisions: %d geladen.\n\n", len(result.KnownDecisions.Decisions))
	}
	if expired := expiredDecisionLines(result.KnownDecisions.Decisions); len(expired) > 0 {
		fmt.Fprintf(&builder, "Abgelaufene Known-Decisions:\n")
		for _, line := range expired {
			fmt.Fprintf(&builder, "- %s\n", line)
		}
		fmt.Fprintf(&builder, "\n")
	}
	if warnings := KnownDecisionWarnings(result); len(warnings) > 0 {
		fmt.Fprintf(&builder, "Hinweise zu Known-Decisions:\n")
		for _, line := range warnings {
			fmt.Fprintf(&builder, "- %s\n", line)
		}
		fmt.Fprintf(&builder, "\n")
	}

	fmt.Fprintf(&builder, "## Tool- und Job-Status\n\n")
	fmt.Fprintf(&builder, "| Eintrag | Zustand | Jobs | Hinweis |\n")
	fmt.Fprintf(&builder, "|---|---|---:|---|\n")
	for _, entry := range result.Entries {
		fmt.Fprintf(&builder, "| %s | %s | %d | %s |\n", entry.Name, entry.State, len(entry.Jobs), tableText(entryReason(entry)))
	}
	fmt.Fprintf(&builder, "\n")

	fmt.Fprintf(&builder, "## Zahlen\n\n")
	fmt.Fprintf(&builder, "### Schwere\n\n")
	for _, line := range countLines(countSeverity(result.Findings)) {
		fmt.Fprintf(&builder, "- %s\n", line)
	}
	fmt.Fprintf(&builder, "\n### Tools\n\n")
	for _, line := range countLines(countTools(result)) {
		fmt.Fprintf(&builder, "- %s\n", line)
	}
	fmt.Fprintf(&builder, "\n")

	fmt.Fprintf(&builder, "## Findings nach Gruppe\n\n")
	fmt.Fprintf(&builder, "| Gruppe | Stable-ID | Schwere | Ort | Befunde | Belege | Known-Decision | Hinweis |\n")
	fmt.Fprintf(&builder, "|---|---|---|---|---:|---|---|---|\n")
	limit := len(result.Groups)
	if limit > markdownGroupLimit {
		limit = markdownGroupLimit
	}
	for _, group := range result.Groups[:limit] {
		fmt.Fprintf(&builder, "| %s | `%s` | %s | %s | %d | %s | %s | %s |\n",
			group.ID, group.StableID, tableText(group.DerivedSeverity), tableText(locationText(group.Location)), len(group.FindingIDs),
			tableText(evidenceText(group.Evidence)), tableText(knownDecisionCoverageText(group)), tableText(groupHint(group)))
	}
	if len(result.Groups) > limit {
		fmt.Fprintf(&builder, "\n%d weitere Gruppen sind nur im vollständigen JSON aufgeführt.\n", len(result.Groups)-limit)
	}
	fmt.Fprintf(&builder, "\n")

	fmt.Fprintf(&builder, "## Dedupe-Hinweise\n\n")
	fmt.Fprintf(&builder, "- Harte Dedupe-Gruppen entstehen aus Fingerprints, exakter Datei/Zeile/Rule/Message oder Dependency-IDs.\n")
	fmt.Fprintf(&builder, "- Bei Dependency-Funden genügt **eine** gemeinsame Kennung, wenn Package und Version übereinstimmen.\n")
	fmt.Fprintf(&builder, "- Gleiche Datei/Zeile mit ähnlicher Rule-Familie wird nur als `possible-duplicate` markiert.\n")
	fmt.Fprintf(&builder, "- Ebenso Dependency-Funde mit gemeinsamer Kennung, deren Package oder Version fehlt oder abweicht.\n")
	fmt.Fprintf(&builder, "- SARIF-Rohdaten, `run.json` und `entries/*.json` bleiben unverändert.\n")
	fmt.Fprintf(&builder, "- Sehr große Detailblöcke stehen ausschließlich in `review-input.json`.\n")
	return builder.String()
}

func countGroupCoverage(groups []Group) (int, int) {
	full, partial := 0, 0
	for _, group := range groups {
		if group.CoveredByKnownDecision != nil {
			full++
			continue
		}
		if group.PartialCoverage {
			partial++
		}
	}
	return full, partial
}

func expiredDecisionLines(decisions []knowndecisions.DecisionReport) []string {
	lines := []string{}
	for _, decision := range decisions {
		if decision.Expired {
			lines = append(lines, fmt.Sprintf("%s (%s), Ablaufdatum %s", decision.ID, decision.Category, decision.Expires))
		}
	}
	sort.Strings(lines)
	return lines
}

func countEntries(entries []EntrySummary) map[string]int {
	counts := map[string]int{}
	for _, entry := range entries {
		counts[string(entry.State)]++
	}
	return counts
}

func countSeverity(findings []Finding) map[string]int {
	counts := map[string]int{}
	for _, finding := range findings {
		level := finding.DerivedSeverity
		if level == "" {
			level = "unmapped"
		}
		counts[level]++
	}
	return counts
}

func countTools(result Result) map[string]int {
	counts := map[string]int{}
	for _, entry := range result.Entries {
		if entry.State == "done" {
			counts[entry.Name] = 0
		}
	}
	for _, finding := range result.Findings {
		counts[finding.Evidence.Tool]++
	}
	return counts
}

func formatCounts(counts map[string]int) string {
	return strings.Join(countLines(counts), ", ")
}

func countLines(counts map[string]int) []string {
	if len(counts) == 0 {
		return []string{"keine"}
	}
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		lines = append(lines, fmt.Sprintf("%s: %d", key, counts[key]))
	}
	return lines
}

func entryReason(entry EntrySummary) string {
	if entry.Reason != "" {
		return entry.Reason
	}
	if !entry.Present {
		return "Entry-Datei fehlt, Zustand start"
	}
	failed, skipped := 0, 0
	for _, job := range entry.Jobs {
		if job.State == "failed" {
			failed++
		}
		if job.State == "skipped" {
			skipped++
		}
	}
	parts := []string{}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("failed: %d", failed))
	}
	if skipped > 0 {
		parts = append(parts, fmt.Sprintf("skipped: %d", skipped))
	}
	return strings.Join(parts, ", ")
}

func locationText(location Location) string {
	if location.URI == "" {
		return ""
	}
	if location.StartLine > 0 {
		return fmt.Sprintf("%s:%d", location.URI, location.StartLine)
	}
	return location.URI
}

func evidenceText(evidence []Evidence) string {
	parts := make([]string, 0, len(evidence))
	for index, item := range evidence {
		if index >= 3 {
			parts = append(parts, fmt.Sprintf("+%d", len(evidence)-index))
			break
		}
		parts = append(parts, item.Tool+"/"+item.Job)
	}
	return strings.Join(parts, ", ")
}

func groupHint(group Group) string {
	parts := []string{}
	if len(group.DedupeRules) > 0 {
		parts = append(parts, strings.Join(group.DedupeRules, "+"))
	}
	if len(group.PossibleDuplicates) > 0 {
		parts = append(parts, "possible: "+strings.Join(group.PossibleDuplicates, ","))
	}
	return strings.Join(parts, "; ")
}

func tableText(text string) string {
	text = strings.ReplaceAll(text, "|", "\\|")
	text = strings.ReplaceAll(text, "\n", " ")
	return strings.TrimSpace(text)
}
