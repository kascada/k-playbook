package merge

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var (
	spacePattern       = regexp.MustCompile(`\s+`)
	messageKeepPattern = regexp.MustCompile(`[^a-z0-9]+`)
	ruleTokenPattern   = regexp.MustCompile(`[a-z0-9]+`)
)

// GroupFindings bildet harte Dedupe-Gruppen und markiert unsichere
// Location-Dubletten nur als possible-duplicate. Kein Finding fällt dabei weg.
func GroupFindings(findings []Finding) []Group {
	if len(findings) == 0 {
		return []Group{}
	}

	uf := newUnionFind(len(findings))
	keyOwners := map[string]int{}
	for index, finding := range findings {
		for _, key := range hardKeys(finding) {
			if owner, ok := keyOwners[key]; ok {
				uf.union(owner, index)
			} else {
				keyOwners[key] = index
			}
		}
	}

	groupIndex := map[int]int{}
	groups := []Group{}
	for index, finding := range findings {
		root := uf.find(index)
		position, ok := groupIndex[root]
		if !ok {
			position = len(groups)
			groupIndex[root] = position
			groups = append(groups, Group{})
		}
		groups[position].FindingIDs = append(groups[position].FindingIDs, finding.ID)
		groups[position].Evidence = append(groups[position].Evidence, finding.Evidence)
	}

	for index := range groups {
		groups[index].ID = fmt.Sprintf("G%03d", index+1)
		groups[index].DedupeRules = dedupeRules(findingsForGroup(findings, groups[index]))
		applyRepresentative(&groups[index], findingsByID(findings)[groups[index].FindingIDs[0]])
	}
	markPossibleDuplicates(groups)
	return groups
}

func hardKeys(finding Finding) []string {
	keys := []string{}
	for _, key := range fingerprintKeys(finding) {
		keys = append(keys, key)
	}
	if key := exactKey(finding); key != "" {
		keys = append(keys, key)
	}
	if key := dependencyKey(finding); key != "" {
		keys = append(keys, key)
	}
	return keys
}

func fingerprintKeys(finding Finding) []string {
	keys := []string{}
	for _, fingerprints := range []map[string]string{finding.Fingerprints, finding.PartialFingerprints} {
		for name, value := range fingerprints {
			if value == "" {
				continue
			}
			// Fingerprints sind nicht werkzeugübergreifend normiert. Vergleichbar
			// sind sie deshalb nur innerhalb derselben Quelle.
			keys = append(keys, "fingerprint:"+finding.Evidence.Tool+":"+name+":"+value)
		}
	}
	sort.Strings(keys)
	return keys
}

func exactKey(finding Finding) string {
	if finding.Location.URI == "" || finding.Location.StartLine == 0 || finding.RuleID == "" || finding.Message == "" {
		return ""
	}
	return fmt.Sprintf("exact:%s:%d:%s:%s",
		normalizePath(finding.Location.URI), finding.Location.StartLine,
		strings.ToLower(finding.RuleID), normalizeMessage(finding.Message))
}

func dependencyKey(finding Finding) string {
	dependency := finding.Dependency
	if len(dependency.IDs) == 0 || dependency.Package == "" {
		return ""
	}
	parts := append([]string{}, dependency.IDs...)
	sort.Strings(parts)
	return "dependency:" + strings.Join(parts, ",") + ":" + strings.ToLower(dependency.Package) + ":" +
		strings.ToLower(dependency.Version) + ":" + normalizePath(dependency.Manifest)
}

func dedupeRules(findings []Finding) []string {
	if len(findings) < 2 {
		return nil
	}
	rules := map[string]bool{}
	for _, finding := range findings {
		if len(fingerprintKeys(finding)) > 0 {
			rules["fingerprint"] = true
		}
		if exactKey(finding) != "" {
			rules["exact-location-message"] = true
		}
		if dependencyKey(finding) != "" {
			rules["dependency"] = true
		}
	}
	list := make([]string, 0, len(rules))
	for rule := range rules {
		list = append(list, rule)
	}
	sort.Strings(list)
	return list
}

func findingsForGroup(findings []Finding, group Group) []Finding {
	byID := findingsByID(findings)
	list := make([]Finding, 0, len(group.FindingIDs))
	for _, id := range group.FindingIDs {
		list = append(list, byID[id])
	}
	return list
}

func findingsByID(findings []Finding) map[string]Finding {
	byID := make(map[string]Finding, len(findings))
	for _, finding := range findings {
		byID[finding.ID] = finding
	}
	return byID
}

func applyRepresentative(group *Group, finding Finding) {
	group.Title = firstNonEmpty(finding.RuleName, finding.RuleID, trimLength(finding.Message, 120))
	group.RuleID = finding.RuleID
	group.Level = finding.Level
	group.Location = finding.Location
	group.Dependency = finding.Dependency
}

func markPossibleDuplicates(groups []Group) {
	for left := range groups {
		for right := left + 1; right < len(groups); right++ {
			if !sameLine(groups[left].Location, groups[right].Location) {
				continue
			}
			if !similarRuleFamily(groups[left], groups[right]) {
				continue
			}
			groups[left].PossibleDuplicates = appendUnique(groups[left].PossibleDuplicates, groups[right].ID)
			groups[right].PossibleDuplicates = appendUnique(groups[right].PossibleDuplicates, groups[left].ID)
		}
	}
}

func sameLine(left Location, right Location) bool {
	return left.URI != "" && right.URI != "" && left.StartLine != 0 && right.StartLine != 0 &&
		normalizePath(left.URI) == normalizePath(right.URI) && left.StartLine == right.StartLine
}

func similarRuleFamily(left Group, right Group) bool {
	leftTokens := ruleTokens(left.RuleID + " " + left.Title)
	for token := range ruleTokens(right.RuleID + " " + right.Title) {
		if leftTokens[token] {
			return true
		}
	}
	return false
}

func ruleTokens(text string) map[string]bool {
	text = strings.ToLower(text)
	tokens := map[string]bool{}
	for _, token := range ruleTokenPattern.FindAllString(text, -1) {
		if len(token) >= 4 {
			tokens[token] = true
		}
	}
	return tokens
}

func appendUnique(list []string, value string) []string {
	for _, existing := range list {
		if existing == value {
			return list
		}
	}
	return append(list, value)
}

func normalizeMessage(message string) string {
	message = strings.ToLower(strings.TrimSpace(message))
	message = messageKeepPattern.ReplaceAllString(message, " ")
	return spacePattern.ReplaceAllString(strings.TrimSpace(message), " ")
}

func normalizePath(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	path = strings.TrimPrefix(path, "file://")
	return strings.ToLower(strings.TrimPrefix(path, "./"))
}

func trimLength(text string, limit int) string {
	text = strings.TrimSpace(spacePattern.ReplaceAllString(text, " "))
	if len(text) <= limit {
		return text
	}
	return text[:limit] + "…"
}

type unionFind struct {
	parent []int
	rank   []int
}

func newUnionFind(size int) *unionFind {
	parent := make([]int, size)
	for index := range parent {
		parent[index] = index
	}
	return &unionFind{parent: parent, rank: make([]int, size)}
}

func (uf *unionFind) find(index int) int {
	if uf.parent[index] != index {
		uf.parent[index] = uf.find(uf.parent[index])
	}
	return uf.parent[index]
}

func (uf *unionFind) union(left int, right int) {
	leftRoot, rightRoot := uf.find(left), uf.find(right)
	if leftRoot == rightRoot {
		return
	}
	if uf.rank[leftRoot] < uf.rank[rightRoot] {
		uf.parent[leftRoot] = rightRoot
		return
	}
	if uf.rank[leftRoot] > uf.rank[rightRoot] {
		uf.parent[rightRoot] = leftRoot
		return
	}
	uf.parent[rightRoot] = leftRoot
	uf.rank[leftRoot]++
}
