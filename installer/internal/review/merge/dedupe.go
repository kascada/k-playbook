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

	byID := findingsByID(findings)
	for index := range groups {
		groups[index].ID = fmt.Sprintf("G%03d", index+1)
		groupFindings := findingsForGroup(byID, groups[index])
		groups[index].DedupeRules = dedupeRules(groupFindings)
		applyRepresentative(&groups[index], groupFindings)
	}
	markPossibleDuplicates(groups)
	assignStableIDs(groups, findings)
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
	keys = append(keys, dependencyKeys(finding)...)
	if key := sameLocationToolKey(finding); key != "" {
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

// dependencyKeys liefert einen Schlüssel **je Kennung** statt eines Schlüssels
// aus allen Kennungen zusammen. Über Union-Find in GroupFindings finden damit
// zwei Werkzeuge zusammen, sobald sie sich *eine* Kennung teilen — vorher
// mussten sie exakt dieselbe Aliasmenge nennen, und mehr Aliase machten die
// Gruppierung schlechter statt besser.
//
// Was im Schlüssel steht und was nicht:
//
//   - Kennung: die eingeengte Menge KeyIDs, mit Rückfall auf IDs. Der Freitext
//     bleibt draußen, weil Union-Find transitiv gruppiert und eine beiläufig
//     genannte Fremd-Kennung sonst zwei verschiedene Befunde verkettete.
//   - Paket: bleibt drin. Dieselbe CVE in zwei verschiedenen Paketen ist ein
//     anderer Befund (vendored libs).
//   - Version: bleibt drin. Eine abweichende oder fehlende Version fängt der
//     weiche Zweig in markPossibleDuplicates auf.
//   - Manifest: **nicht** drin. Die Messung aus Task 027 zeigt dieselbe Datei in
//     drei Schreibweisen (`requirements.txt`, `/requirements.txt`,
//     `file:///abs/pfad/requirements.txt`), und auch nach der zielfreien
//     Normierung in normalizePath fällt der absolute Pfad noch auseinander. Der
//     Preis ist bekannt: im Monorepo bleiben gleiches Paket und gleiche CVE in
//     services/a und services/b nicht mehr getrennt.
//
// Der Guard auf leeres Package bleibt: ein Werkzeug ohne diese Property bekommt
// bewusst keinen harten Schlüssel. Auch das fängt der weiche Zweig auf.
func dependencyKeys(finding Finding) []string {
	dependency := finding.Dependency
	if dependency.Package == "" {
		return nil
	}
	ids := dependency.KeyIDs
	if len(ids) == 0 {
		ids = dependency.IDs
	}
	if len(ids) == 0 {
		return nil
	}
	suffix := ":" + strings.ToLower(dependency.Package) + ":" + strings.ToLower(dependency.Version)
	keys := make([]string, 0, len(ids))
	for _, id := range ids {
		keys = append(keys, "dependency:"+strings.ToUpper(id)+suffix)
	}
	sort.Strings(keys)
	return keys
}

func sameLocationToolKey(finding Finding) string {
	if finding.Evidence.Tool == "" || finding.Evidence.Job == "" || finding.Location.URI == "" || finding.Location.StartLine == 0 {
		return ""
	}
	return fmt.Sprintf("same-location-tool:%s:%s:%s:%d",
		strings.ToLower(finding.Evidence.Tool), strings.ToLower(finding.Evidence.Job),
		normalizePath(finding.Location.URI), finding.Location.StartLine)
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
		if len(dependencyKeys(finding)) > 0 {
			rules["dependency"] = true
		}
		if sameLocationToolKey(finding) != "" {
			rules["same-location-tool"] = true
		}
	}
	list := make([]string, 0, len(rules))
	for rule := range rules {
		list = append(list, rule)
	}
	sort.Strings(list)
	return list
}

func findingsForGroup(byID map[string]Finding, group Group) []Finding {
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

// applyRepresentative übernimmt Titel, Rule, Schwere, Lage und Dependency vom
// ersten Finding der Gruppe.
//
// Ausnahme sind die Kennungen: sie sind die Vereinigung über die Findings, die
// dieselbe Dependency beschreiben wie der Repräsentant. Sonst ginge gerade das
// verloren, wofür der Schlüssel je Kennung gebaut ist — nennt ein Werkzeug nur
// die GHSA und ein anderes nur die CVE, trüge die Gruppe je nach Reihenfolge
// nur die eine oder nur die andere.
//
// Die Einschränkung auf dieselbe Dependency ist nötig, weil eine Gruppe nicht
// zwangsläufig über den Dependency-Schlüssel entstanden ist. Bildet
// sameLocationToolKey sie, stehen darin verschiedene Schwachstellen
// nebeneinander: bei Manifest-Funden zeigen alle Funde eines Werkzeugs auf
// dieselbe Zeile. Eine Vereinigung über *alle* Findings schriebe deren
// Kennungen in einen dependency-Block, dessen Package und Version weiter vom
// ersten Finding stammen — in sich widersprüchlich, und als Vorfilter des
// weichen Zweigs ein Magnet, der die Gruppe mit fast jeder anderen verbindet.
//
// Package, Version und Manifest bleiben unverändert vom ersten Finding: sie
// sind Angaben über *einen* Fund und lassen sich nicht vereinigen.
func applyRepresentative(group *Group, findings []Finding) {
	finding := findings[0]
	group.Title = firstNonEmpty(finding.RuleName, finding.RuleID, trimLength(finding.Message, 120))
	group.RuleID = finding.RuleID
	group.Level = finding.Level
	group.DerivedSeverity = finding.DerivedSeverity
	group.SeveritySource = finding.SeveritySource
	group.Location = finding.Location
	group.Dependency = finding.Dependency
	sameDependency := findingsWithSameDependency(findings, finding.Dependency)
	group.Dependency.IDs = unionDependencyIDs(sameDependency, func(dependency Dependency) []string { return dependency.IDs })
	group.Dependency.KeyIDs = unionDependencyIDs(sameDependency, func(dependency Dependency) []string { return dependency.KeyIDs })
}

// findingsWithSameDependency wählt die Findings, deren Paket und Version zu
// denen des Repräsentanten passen — die also überhaupt eine gemeinsame
// Dependency beschreiben können.
//
// Ohne Paket gibt es keinen harten Dependency-Schlüssel und damit nichts, was
// eine Vereinigung rechtfertigte. Dann bleibt es beim Repräsentanten allein:
// ein Vergleich auf „beide ohne Paket" träfe sonst jedes Finding der Gruppe und
// führte genau die Vermischung herbei, die hier vermieden werden soll.
func findingsWithSameDependency(findings []Finding, representative Dependency) []Finding {
	if representative.Package == "" {
		return findings[:1]
	}
	list := make([]Finding, 0, len(findings))
	for _, finding := range findings {
		if !strings.EqualFold(finding.Dependency.Package, representative.Package) {
			continue
		}
		if !strings.EqualFold(finding.Dependency.Version, representative.Version) {
			continue
		}
		list = append(list, finding)
	}
	return list
}

func unionDependencyIDs(findings []Finding, pick func(Dependency) []string) []string {
	seen := map[string]bool{}
	ids := []string{}
	for _, finding := range findings {
		for _, id := range pick(finding.Dependency) {
			if seen[id] {
				continue
			}
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	sort.Strings(ids)
	return ids
}

func markPossibleDuplicates(groups []Group) {
	markPossibleLocationDuplicates(groups)
	markPossibleDependencyDuplicates(groups)
}

func markPossibleLocationDuplicates(groups []Group) {
	for left := range groups {
		for right := left + 1; right < len(groups); right++ {
			if !sameLine(groups[left].Location, groups[right].Location) {
				continue
			}
			if !similarRuleFamily(groups[left], groups[right]) {
				continue
			}
			markPossiblePair(groups, left, right)
		}
	}
}

// markPossibleDependencyDuplicates ergänzt den Location-Zweig um einen für
// Dependency-Funde. Nötig ist er, weil der Location-Zweig über sameLine läuft
// und Dependency-Funde regelmäßig keine Zeilenangabe haben — der Mechanismus
// erreichte diese Fundklasse bisher gar nicht.
//
// Er greift in zwei Fällen, beide nur bei Überschneidung in der **breiten**
// ID-Menge (die Einengung aus dependencyKeys gilt ausschließlich für den harten
// Schlüssel):
//
//  1. Beide Seiten haben einen harten Schlüssel, aber Paket oder Version weicht
//     ab.
//
// Nicht markiert wird der Fall, dass beide einen harten Schlüssel haben und
// Paket und Version übereinstimmen, die Überschneidung aber allein in der
// breiten Menge liegt. Hart gruppiert sind die beiden dann nicht — dazu müsste
// sich die *enge* Menge überschneiden —, und beziehungslos bleiben sie
// absichtlich: dieselbe Signatur tragen zwei verschiedene CVEs desselben
// Pakets, von denen die eine die andere im Text nennt (siehe
// TestGroupFindingsZweiCVEsImSelbenPaketBleibenGetrennt). Sie ist von zwei
// Werkzeugen, die dieselbe Schwachstelle melden und die gemeinsame Kennung nur
// im Freitext führen, nicht zu unterscheiden. Der Intent wiegt die falsche
// Verschmelzung schwerer als die ausgebliebene Markierung.
//  2. Mindestens eine Seite hat gar keinen harten Schlüssel — kein Paket, oder
//     keine Kennung. Im gemessenen Lauf ist das der Regelfall: grype,
//     osv-scanner und trivy schreiben keine package-Property.
//
// Das Manifest wird nicht verlangt. Der weiche Zweig darf nie strenger sein als
// der harte, und dort steht es seit Task 027 nicht mehr drin.
//
// Der Index über die Kennungen ist Vorfilter: ohne ihn liefe der Zweig über
// alle Paare, obwohl nur Paare mit gemeinsamer Kennung überhaupt in Frage
// kommen.
func markPossibleDependencyDuplicates(groups []Group) {
	groupsByID := map[string][]int{}
	for index := range groups {
		for _, id := range groups[index].Dependency.IDs {
			groupsByID[id] = append(groupsByID[id], index)
		}
	}

	hardKeyed := make([]bool, len(groups))
	for index := range groups {
		hardKeyed[index] = len(dependencyKeys(Finding{Dependency: groups[index].Dependency})) > 0
	}

	checked := map[[2]int]bool{}
	for _, indices := range groupsByID {
		for position, left := range indices {
			for _, right := range indices[position+1:] {
				pair := [2]int{left, right}
				if checked[pair] {
					continue
				}
				checked[pair] = true
				if !possibleDependencyDuplicate(groups[left], groups[right], hardKeyed[left], hardKeyed[right]) {
					continue
				}
				markPossiblePair(groups, left, right)
			}
		}
	}
}

func possibleDependencyDuplicate(left Group, right Group, leftHardKeyed bool, rightHardKeyed bool) bool {
	if !leftHardKeyed || !rightHardKeyed {
		return true
	}
	if !strings.EqualFold(left.Dependency.Package, right.Dependency.Package) {
		return true
	}
	return !strings.EqualFold(left.Dependency.Version, right.Dependency.Version)
}

func markPossiblePair(groups []Group, left int, right int) {
	groups[left].PossibleDuplicates = appendUnique(groups[left].PossibleDuplicates, groups[right].ID)
	groups[right].PossibleDuplicates = appendUnique(groups[right].PossibleDuplicates, groups[left].ID)
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

// normalizePath bringt Pfadangaben verschiedener Werkzeuge auf eine
// Schreibweise. Sie bleibt rein und einparametrig: das Scan-Ziel wird bewusst
// nicht durchgereicht — GroupFindings als einzige Einstiegsstelle kennt es
// nicht, und alle vier Aufrufer (exactKey, sameLine, sameLocationToolKey,
// stablePrefixAndKey) hängen an dieser Signatur.
//
// Normiert wird deshalb nur, was ohne Zielkenntnis geht:
//
//   - Backslashes zu `/`,
//   - `file://` weg, mit und ohne Host-Teil (`file:///pfad`, `file://host/pfad`),
//   - `.`- und `..`-Segmente sowie doppelte Slashes auflösen,
//   - führendes `/` entfernen (`/requirements.txt` → `requirements.txt`),
//   - Groß-/Kleinschreibung angleichen.
//
// Was damit ausdrücklich **nicht** zusammenfindet: ein absoluter Pfad
// unterhalb des Scan-Ziels und derselbe Pfad relativ dazu. `/abs/projekt/a.txt`
// und `a.txt` bleiben verschieden, weil ohne das Ziel nicht zu entscheiden ist,
// wo der gemeinsame Teil aufhört. Genau daran scheitert im gemessenen Lauf die
// Zusammenführung von osv-scanner mit den übrigen Werkzeugen — der Grund, aus
// dem das Manifest nicht mehr im harten Dependency-Schlüssel steht.
func normalizePath(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	path = trimFileScheme(path)
	return strings.ToLower(collapsePath(path))
}

// trimFileScheme schneidet ein `file://`-Präfix ab. Der Teil zwischen den
// Doppelslashes und dem nächsten `/` ist die Authority und gehört nicht zum
// Pfad; bei `file:///pfad` ist sie leer, bei `file://host/pfad` steht dort ein
// Rechnername. Fehlt ein weiterer `/`, ist die ganze Restzeichenkette der Pfad
// — eine formal falsche, aber vorkommende Schreibweise, bei der Wegwerfen mehr
// schadete als Behalten.
func trimFileScheme(path string) string {
	if len(path) < len("file://") || !strings.EqualFold(path[:len("file://")], "file://") {
		return path
	}
	rest := path[len("file://"):]
	if slash := strings.Index(rest, "/"); slash >= 0 {
		return rest[slash:]
	}
	return rest
}

// collapsePath löst `.`, `..` und doppelte Slashes auf und liefert den Pfad
// ohne führenden `/`.
//
// Kein filepath.Clean: das arbeitet mit dem Trennzeichen des laufenden Systems
// und ließe eine unter Windows erzeugte SARIF-Datei anders normieren als
// dieselbe unter Linux. Hier ist `/` gesetzt, gleich auf welchem System.
func collapsePath(path string) string {
	rooted := strings.HasPrefix(path, "/")
	segments := []string{}
	for _, segment := range strings.Split(path, "/") {
		switch segment {
		case "", ".":
			continue
		case "..":
			if len(segments) > 0 && segments[len(segments)-1] != ".." {
				segments = segments[:len(segments)-1]
				continue
			}
			// Über die Wurzel hinaus führt kein Weg; ein relativer Pfad behält
			// sein `..`, weil es dort etwas bezeichnet.
			if rooted {
				continue
			}
			segments = append(segments, "..")
		default:
			segments = append(segments, segment)
		}
	}
	return strings.Join(segments, "/")
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
