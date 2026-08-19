package merge

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const SeverityCatalogName = "scripts/severity.tsv"

type SeverityMapping []SeverityRule

type SeverityRule struct {
	Tool       string
	RulePrefix string
	Severity   string
	Notes      string
}

func SeverityCatalog(playbookDir string) string {
	return filepath.Join(playbookDir, filepath.FromSlash(SeverityCatalogName))
}

func LoadSeverityMapping(path string) (SeverityMapping, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseSeverityMapping(string(data), path)
}

func ParseSeverityMapping(content string, source string) (SeverityMapping, error) {
	mapping := SeverityMapping{}
	for number, line := range strings.Split(content, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if fields[0] == "tool" {
			continue
		}
		where := fmt.Sprintf("%s, Zeile %d", source, number+1)
		if len(fields) != 4 {
			return nil, fmt.Errorf("%s: %d Spalten, erwartet 4", where, len(fields))
		}
		rule := SeverityRule{
			Tool:       strings.ToLower(strings.TrimSpace(fields[0])),
			RulePrefix: strings.ToLower(strings.TrimSpace(fields[1])),
			Severity:   strings.ToLower(strings.TrimSpace(fields[2])),
			Notes:      strings.TrimSpace(fields[3]),
		}
		if rule.Tool == "" || rule.RulePrefix == "" || rule.Severity == "" {
			return nil, fmt.Errorf("%s: tool, rule_prefix und severity müssen gesetzt sein", where)
		}
		if !validSeverity(rule.Severity) {
			return nil, fmt.Errorf("%s: unbekannte Schwere %q", where, rule.Severity)
		}
		mapping = append(mapping, rule)
	}
	sort.SliceStable(mapping, func(left, right int) bool {
		if mapping[left].Tool != mapping[right].Tool {
			return mapping[left].Tool < mapping[right].Tool
		}
		return len(mapping[left].RulePrefix) > len(mapping[right].RulePrefix)
	})
	return mapping, nil
}

func deriveSeverity(finding Finding, result sarifResult, rule sarifRule, mapping SeverityMapping) (string, string) {
	level := strings.ToLower(strings.TrimSpace(finding.Level))
	if level != "" && level != "none" && level != "warning" {
		return level, "native"
	}
	if severity, ok := severityFromCVSS(result.Properties, rule.Properties); ok {
		return severity, "cvss"
	}
	if severity, ok := severityFromToolMetadata(result.Properties, rule.Properties); ok {
		return severity, "tool-metadata"
	}
	if severity, ok := mappingSeverity(mapping, finding.Evidence.Tool, finding.RuleID); ok {
		return severity, "mapping"
	}
	if level == "warning" || level == "none" {
		return level, "native"
	}
	return "unmapped", "unmapped"
}

func severityFromCVSS(objects ...sarifObject) (string, bool) {
	for _, properties := range objects {
		if score, ok := numericProperty(properties, "security-severity", "cvss", "cvssScore", "cvss_score"); ok {
			return severityForCVSS(score), true
		}
		if severity, ok := cvssVectorSeverity(properties); ok {
			return severity, true
		}
	}
	return "", false
}

func cvssVectorSeverity(properties sarifObject) (string, bool) {
	vector := stringProperty(properties, "cvssVector")
	if vector == "" {
		vector = stringProperty(properties, "cvss_vector")
	}
	if vector == "" {
		return "", false
	}
	if strings.Contains(vector, "/C:H") || strings.Contains(vector, "/I:H") || strings.Contains(vector, "/A:H") {
		return "error", true
	}
	if strings.Contains(vector, "/C:L") || strings.Contains(vector, "/I:L") || strings.Contains(vector, "/A:L") {
		return "warning", true
	}
	return "note", true
}

func severityFromToolMetadata(objects ...sarifObject) (string, bool) {
	for _, properties := range objects {
		for _, key := range []string{"severity", "Severity", "impact", "priority"} {
			severity := normalizeToolSeverity(stringProperty(properties, key))
			if severity != "" {
				return severity, true
			}
		}
	}
	return "", false
}

func mappingSeverity(mapping SeverityMapping, tool string, ruleID string) (string, bool) {
	tool = strings.ToLower(strings.TrimSpace(tool))
	ruleID = strings.ToLower(strings.TrimSpace(ruleID))
	if tool == "" || ruleID == "" {
		return "", false
	}
	for _, rule := range mapping {
		if rule.Tool == tool && strings.HasPrefix(ruleID, rule.RulePrefix) {
			return rule.Severity, true
		}
	}
	return "", false
}

func numericProperty(properties sarifObject, keys ...string) (float64, bool) {
	for _, key := range keys {
		value, ok := properties[key]
		if !ok {
			continue
		}
		score, ok := numberValue(value)
		if ok {
			return score, true
		}
	}
	return 0, false
}

func numberValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, !math.IsNaN(typed)
	case int:
		return float64(typed), true
	case json.Number:
		score, err := typed.Float64()
		return score, err == nil
	case string:
		score, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return score, err == nil
	default:
		return 0, false
	}
}

func severityForCVSS(score float64) string {
	switch {
	case score >= 7.0:
		return "error"
	case score >= 4.0:
		return "warning"
	case score > 0:
		return "note"
	default:
		return "none"
	}
}

func normalizeToolSeverity(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "critical", "high", "error":
		return "error"
	case "medium", "moderate", "warning", "warn":
		return "warning"
	case "low", "note", "info", "informational":
		return "note"
	case "none", "negligible":
		return "none"
	default:
		return ""
	}
}

func validSeverity(value string) bool {
	switch value {
	case "error", "warning", "note", "none":
		return true
	default:
		return false
	}
}
