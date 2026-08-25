package merge

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/review"
)

var stableDigest = func(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

func assignStableIDs(groups []Group, findings []Finding) {
	byID := findingsByID(findings)
	for index := range groups {
		prefix, key := stablePrefixAndKey(findingsForGroup(byID, groups[index]))
		groups[index].StableKey = key
		groups[index].StableID = prefix + stableDigest(key)
	}
	shortenStableIDs(groups)
	rewritePossibleDuplicatesToStableIDs(groups)
}

// stablePrefixAndKey bildet Präfix und stabilen Schlüssel einer Gruppe.
//
// Die Klasse entscheidet, woraus der Schlüssel besteht: location und dependency
// nehmen alles, was einen Fund beschreibt, class=ai nur das, was der Vertrag des
// Rezepts festhält — siehe stableClass und aiKeyLines.
func stablePrefixAndKey(findings []Finding) (string, string) {
	class := stableClass(findings)
	lines := append([]string{"class=" + class}, stableKeyLines(class, findings)...)
	sort.Strings(lines[1:])
	return stablePrefix(class, findings), strings.Join(lines, "\n")
}

// stableClass ordnet eine Gruppe einer Schlüsselklasse zu.
//
// ai steht vor dependency und nicht dahinter: die Klasse folgt der Herkunft der
// Funde, und die ist eindeutig. Die Dependency-Klasse entsteht dagegen aus
// Kennungen, die aus dem Freitext gelesen werden — ein KI-Fund, der eine CVE
// nur erwähnt, bekäme sonst den Dependency-Schlüssel mit Ort und Meldung darin
// und verlöre genau die Stabilität, für die class=ai gebaut ist.
//
// ai gilt nur, wenn *jeder* Fund der Gruppe aus einem Evidence-Eintrag stammt.
// Findet ein Scanner dieselbe Stelle, zieht exactKey beide zusammen und die
// Gruppe bleibt location: sonst verlöre eine bestehende Scanner-Gruppe ihre ID,
// sobald ein KI-Fund hinzukommt. Anders als bei dependency, wo eine einzelne
// Kennung die Klasse setzt, wiegt hier die Rückwärtskompatibilität der
// Scanner-IDs schwerer.
func stableClass(findings []Finding) string {
	if aiClassApplies(findings) {
		return "ai"
	}
	if dependencyPrimaryID(findings) != "" {
		return "dependency"
	}
	return "location"
}

// aiClassApplies meldet, ob die Gruppe vollständig aus KI-Evidence besteht und
// die Bestandteile des KI-Schlüssels mitbringt.
//
// Ohne Dateipfad und Rule-ID bliebe von class=ai nur der Rezeptname übrig, und
// zwei verschiedene Funde bekämen denselben Schlüssel — eine Kollision, die
// shortenStableIDs nicht auflösen kann, weil sie schon im Digest steckt. Solche
// Funde fallen auf location zurück, wo Ort und Meldung sie wieder
// unterscheiden. Auf dem regulären Weg tritt der Fall nicht auf:
// review.CheckEvidenceSARIF verwirft Funde ohne Ort und weist Funde ohne
// Rule-ID ab.
func aiClassApplies(findings []Finding) bool {
	if len(findings) == 0 {
		return false
	}
	for _, finding := range findings {
		if finding.Mode != review.ModeEvidence {
			return false
		}
		if normalizePath(finding.Location.URI) == "" || strings.TrimSpace(finding.RuleID) == "" {
			return false
		}
	}
	return true
}

func stableKeyLines(class string, findings []Finding) []string {
	if class == "ai" {
		return aiKeyLines(findings)
	}
	return locationKeyLines(findings)
}

// aiKeyLines ist der Schlüssel für KI-Evidence: Rezeptname, Rule-ID und
// normalisierter Dateipfad.
//
// Ohne startLine/startColumn, ohne Meldung, ohne Fingerprints und ohne
// Dependency-Teile. KI-Funde kommen im nächsten Lauf mit verschobener Zeile und
// umformuliertem Text zurück; stünde beides im Schlüssel, hätte die Gruppe jedes
// Mal eine neue ID und keine stableId-Decision griffe je ein zweites Mal.
//
// Auch der Job bleibt draußen: bei einem Evidence-Eintrag heißt er wie der
// Eintrag selbst und trüge nichts bei, was tools nicht schon sagt.
func aiKeyLines(findings []Finding) []string {
	lines := stableList("tools", stableValues(findings, func(f Finding) string { return f.Evidence.Tool }))
	lines = append(lines, stableList("rules", stableValues(findings, func(f Finding) string { return strings.ToLower(f.RuleID) }))...)
	lines = append(lines, stableList("paths", stableValues(findings, func(f Finding) string {
		return normalizePath(f.Location.URI)
	}))...)
	return lines
}

func locationKeyLines(findings []Finding) []string {
	lines := stableList("tools", stableValues(findings, func(f Finding) string { return f.Evidence.Tool }))
	lines = append(lines, stableList("jobs", stableValues(findings, func(f Finding) string { return f.Evidence.Job }))...)
	lines = append(lines, stableList("locations", stableValues(findings, func(f Finding) string {
		if f.Location.URI == "" && f.Location.StartLine == 0 {
			return ""
		}
		return fmt.Sprintf("%s:%d:%d", normalizePath(f.Location.URI), f.Location.StartLine, f.Location.StartColumn)
	}))...)
	lines = append(lines, stableList("rules", stableValues(findings, func(f Finding) string { return strings.ToLower(f.RuleID) }))...)
	lines = append(lines, stableList("messages", stableValues(findings, func(f Finding) string { return normalizeMessage(f.Message) }))...)
	lines = append(lines, stableList("fingerprints", stableValues(findings, func(f Finding) string {
		return stableMapText(f.Fingerprints)
	}))...)
	lines = append(lines, stableList("partialFingerprints", stableValues(findings, func(f Finding) string {
		return stableMapText(f.PartialFingerprints)
	}))...)
	lines = append(lines, stableList("dependencies", stableValues(findings, func(f Finding) string {
		dependency := f.Dependency
		if len(dependency.IDs) == 0 && dependency.Package == "" && dependency.Version == "" && dependency.Manifest == "" {
			return ""
		}
		ids := append([]string{}, dependency.IDs...)
		sort.Strings(ids)
		return strings.Join([]string{
			strings.ToLower(strings.Join(ids, ",")),
			strings.ToLower(dependency.Package),
			strings.ToLower(dependency.Version),
			normalizePath(dependency.Manifest),
		}, "|")
	}))...)
	return lines
}

// stablePrefix ist der lesbare Teil der Anzeige-ID.
//
// Für class=ai heißt er ai-<eintrag>- statt scan-<werkzeug>-: KI-Evidence ist
// damit schon an der ID als solche erkennbar. stablePrefixFromID trennt am
// letzten Bindestrich und trägt ihn mit, weil der Digest hexadezimal ist —
// Kürzung und Eindeutigkeit rechnen weiter je Präfix.
func stablePrefix(class string, findings []Finding) string {
	if class == "dependency" {
		id := strings.ToLower(dependencyPrimaryID(findings))
		if id == "" {
			id = "dependency"
		}
		return "scan-cve-" + stableSegment(id) + "-"
	}
	tools := stableValues(findings, func(f Finding) string { return f.Evidence.Tool })
	tool := "multi"
	if len(tools) == 1 {
		tool = tools[0]
	}
	if class == "ai" {
		return "ai-" + stableSegment(tool) + "-"
	}
	return "scan-" + stableSegment(tool) + "-"
}

func dependencyPrimaryID(findings []Finding) string {
	ids := []string{}
	for _, finding := range findings {
		ids = append(ids, finding.Dependency.IDs...)
	}
	if len(ids) == 0 {
		return ""
	}
	sort.Strings(ids)
	return ids[0]
}

func stableValues(findings []Finding, value func(Finding) string) []string {
	seen := map[string]bool{}
	values := []string{}
	for _, finding := range findings {
		item := strings.TrimSpace(strings.ToLower(value(finding)))
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		values = append(values, item)
	}
	sort.Strings(values)
	return values
}

func stableList(key string, values []string) []string {
	if len(values) == 0 {
		return nil
	}
	lines := make([]string, 0, len(values))
	for _, value := range values {
		lines = append(lines, key+"="+value)
	}
	return lines
}

func stableMapText(values map[string]string) string {
	if len(values) == 0 {
		return ""
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, strings.ToLower(key)+"="+values[key])
	}
	return strings.Join(parts, ";")
}

func stableSegment(value string) string {
	value = strings.ToLower(value)
	var builder strings.Builder
	previousDash := false
	for _, char := range value {
		valid := (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9')
		if valid {
			builder.WriteRune(char)
			previousDash = false
			continue
		}
		if !previousDash {
			builder.WriteByte('-')
			previousDash = true
		}
	}
	segment := strings.Trim(builder.String(), "-")
	if segment == "" {
		return "unknown"
	}
	return segment
}

func shortenStableIDs(groups []Group) {
	full := make([]string, len(groups))
	prefixes := make([]string, len(groups))
	for index := range groups {
		prefixes[index] = stablePrefixFromID(groups[index].StableID)
		full[index] = strings.TrimPrefix(groups[index].StableID, prefixes[index])
	}
	for index := range groups {
		length := 6
		if length > len(full[index]) {
			length = len(full[index])
		}
		for !stablePrefixUnique(index, prefixes, full, length) && length < len(full[index]) {
			length++
		}
		groups[index].StableID = prefixes[index] + full[index][:length]
	}
}

func stablePrefixUnique(index int, prefixes []string, full []string, length int) bool {
	candidate := full[index][:length]
	for other := range full {
		if other == index || prefixes[other] != prefixes[index] {
			continue
		}
		otherLength := length
		if otherLength > len(full[other]) {
			otherLength = len(full[other])
		}
		if full[other][:otherLength] == candidate {
			return false
		}
	}
	return true
}

func stablePrefixFromID(id string) string {
	last := strings.LastIndex(id, "-")
	if last < 0 {
		return ""
	}
	return id[:last+1]
}

func rewritePossibleDuplicatesToStableIDs(groups []Group) {
	byDisplayID := map[string]string{}
	for _, group := range groups {
		byDisplayID[group.ID] = group.StableID
	}
	for index := range groups {
		for duplicate := range groups[index].PossibleDuplicates {
			if stableID, ok := byDisplayID[groups[index].PossibleDuplicates[duplicate]]; ok {
				groups[index].PossibleDuplicates[duplicate] = stableID
			}
		}
		sort.Strings(groups[index].PossibleDuplicates)
	}
}
