package merge

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/pathnorm"
	"github.com/kascada/k-playbook/installer/internal/review"
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
	if key := aiPathRuleKey(finding); key != "" {
		keys = append(keys, key)
	}
	return keys
}

// aiPathRuleKey ist der harte Schlüssel für KI-Evidence: eine Rule-ID in einer
// Datei ist eine Gruppe.
//
// Er hält die Gruppierung mit der Schlüsselklasse zusammen. class=ai kennt weder
// Zeile noch Meldung (siehe aiKeyLines in stable.go); ohne diesen Schlüssel
// blieben zwei Funde derselben Rule-ID in derselben Datei zwei Gruppen mit
// identischem stabilem Schlüssel — also kollidierenden stableIds.
//
// Die Grobkörnigkeit ist gewollt: alle Instanzen bleiben als findingIds und
// evidence erhalten, der Repräsentant nennt eine Stelle, die Triage liest die
// Anzahl mit. Eine Decision auf diese Gruppe deckt die Datei für diese Rule-ID
// ganz; eine Abweisung je Instanz gibt es nicht.
func aiPathRuleKey(finding Finding) string {
	if finding.Mode != review.ModeEvidence {
		return ""
	}
	if finding.Evidence.Tool == "" || finding.Location.URI == "" || finding.RuleID == "" {
		return ""
	}
	return fmt.Sprintf("ai:%s:%s:%s",
		strings.ToLower(finding.Evidence.Tool), pathnorm.Normalize(finding.Location.URI),
		strings.ToLower(finding.RuleID))
}

// namingFingerprints ist die Zulassungsliste für Dependency-Funde: nur diese
// Fingerprints benennen den Fund und dürfen ihn deshalb gruppieren. Jeder nicht
// geführte Fingerprint bildet für einen Fund mit erkannter Dependency keinen
// Schlüssel mehr.
//
// Geführt werden **Paare aus Werkzeug und Name**, nicht bloße Namen. Der Name
// allein trägt nicht: grype und osv-scanner nennen ihren Eintrag beide
// primaryLocationLineHash und meinen etwas anderes damit. Bei grype ist er ein
// Hash über die Manifest-Zeile und je Paket gleich — im Lauf 2026-08-28 decken
// acht seiner neun Werte je 3 bis 6 verschiedene Schwachstellen ab. Bei
// osv-scanner unterscheidet er Schwachstellen innerhalb derselben Zeile: 32
// Werte für 32 Rule-IDs, eine Bijektion, und dieselben Pakete tragen mehrere
// Schwachstellen mit verschiedenen Werten (urllib3@1.23.0 unter drei Werten).
// Eine Liste über den Namen allein ließe grypes Ortsgruppierung bestehen.
//
// Die Liste ist eine Zulassungs-, keine Sperrliste, und irrt damit im Zweifel
// konservativ: ein Name, den erst ein Werkzeug-Update mitbringt, steht nicht
// darauf und kann die Ortsgruppierung nicht still wiederherstellen. Der Preis
// ist die Pflege je Werkzeug — ein neuer benennender Fingerprint wirkt erst,
// wenn er am Roh-SARIF belegt und hier eingetragen ist.
//
// Werkzeug- und Fingerprint-Name stehen kleingeschrieben; verglichen wird
// kleingeschrieben.
var namingFingerprints = map[string]map[string]bool{
	"osv-scanner": {"primarylocationlinehash": true},
}

// namesFinding meldet, ob ein Fingerprint dieses Werkzeugs den Fund benennt und
// damit auch für Dependency-Funde einen Schlüssel bilden darf.
func namesFinding(tool string, name string) bool {
	names, ok := namingFingerprints[strings.ToLower(strings.TrimSpace(tool))]
	if !ok {
		return false
	}
	return names[strings.ToLower(strings.TrimSpace(name))]
}

// fingerprintKeys bildet je Fingerprint einen harten Schlüssel — für
// Dependency-Funde nur noch aus der Zulassungsliste namingFingerprints.
//
// Der Grund ist derselbe wie bei sameLocationToolKey seit Task 028: ein
// Fingerprint, der nur den Fundort hasht, gruppiert Dependency-Funde nach Ort
// statt nach Identität. Bei grype ist genau das der Fall, und im Lauf
// 2026-08-28 zog es alle sechs jinja2-Schwachstellen in eine Gruppe und alle
// sechs stdlib-Funde je Binary in eine weitere. Anders als der Ort *kann* ein
// Fingerprint den Fund aber benennen — SARIF sieht ihn dafür vor, und
// osv-scanner vergibt ihn so. Pauschal abzuschalten verlöre diese Gruppierung;
// deshalb die namentliche Zulassung statt einer Regel.
//
// Für Funde ohne erkannte Dependency bleibt alles, wie es war: dort ist kein
// Manifest im Spiel, das alle Funde eines Werkzeugs auf dieselbe Zeile legt.
func fingerprintKeys(finding Finding) []string {
	dependency := hasDependency(finding.Dependency)
	keys := []string{}
	for _, fingerprints := range []map[string]string{finding.Fingerprints, finding.PartialFingerprints} {
		for name, value := range fingerprints {
			if value == "" {
				continue
			}
			if dependency && !namesFinding(finding.Evidence.Tool, name) {
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
		pathnorm.Normalize(finding.Location.URI), finding.Location.StartLine,
		strings.ToLower(finding.RuleID), normalizeMessage(finding.Message))
}

// dependencyKeyIDs ist die enge Kennungsmenge eines Fundes: KeyIDs, mit
// Rückfall auf IDs, wenn ein Werkzeug seine einzige Kennung nur im Freitext
// nennt und die enge Menge deshalb leer bliebe.
//
// Sie steht hier als eine Funktion, weil zwei Stellen sie brauchen und
// auseinanderlaufen dürfen sie nicht: dependencyKeys bildet daraus den harten
// Dedupe-Schlüssel, stable.go den stabilen Gruppenschlüssel samt Präfix und
// Klasse. Nennt ein Werkzeug eine Kennung mehr, die nur im Advisory-Text
// vorkommt, ändert das seit Task 029 weder die Gruppe noch ihre Stable-ID.
func dependencyKeyIDs(dependency Dependency) []string {
	if len(dependency.KeyIDs) > 0 {
		return dependency.KeyIDs
	}
	return dependency.IDs
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
//     Normierung in pathnorm.Normalize fällt der absolute Pfad noch auseinander. Der
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
	ids := dependencyKeyIDs(dependency)
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

// sameLocationToolKey fasst zusammen, was ein Werkzeug in einem Lauf an
// derselben Zeile meldet.
//
// Für KI-Evidence gilt er nicht. class=ai kennt keine Zeile (siehe aiKeyLines in
// stable.go); ein Schlüssel über die Zeile zöge Funde verschiedener Rule-IDs
// zusammen, sobald sie zufällig in derselben stehen, und im nächsten Lauf
// stünden sie woanders — die Gruppe zerfiele, und mit ihr die stableId. An
// seine Stelle tritt aiPathRuleKey; „dieselbe Regel in derselben Zeile" ist
// darin als Teilfall enthalten.
//
// Für Dependency-Funde gilt er seit Task 028 ebenfalls nicht. Bei
// Manifest-Funden zeigt jeder Fund eines Werkzeugs auf dieselbe Zeile der
// Manifest-Datei; die Regel gruppierte dort nach Fundort statt nach Identität.
// Gemessen am Lauf 2026-08-27: neun Gruppen entstanden über sie, und in jeder
// steckten so viele verschiedene Rule-IDs wie Findings — bis zu 18 verschiedene
// GHSA in einer Gruppe, weil grype für jeden Fund auf requirements.txt:1 zeigt.
// Zusammen mit dem Paket-Rückfall aus derselben Task wurde daraus eine Gruppe
// aus 39 Findings mit 36 verschiedenen Rule-IDs: die Regel verkettete über den
// Fundort alles, was der Dependency-Schlüssel korrekt zusammengeführt hatte, mit
// allem übrigen desselben Werkzeugs. Für Nicht-Dependency-Funde bleibt sie
// unverändert — dort ist dieselbe Zeile eine Aussage über den Fund.
func sameLocationToolKey(finding Finding) string {
	if finding.Mode == review.ModeEvidence {
		return ""
	}
	if hasDependency(finding.Dependency) {
		return ""
	}
	if finding.Evidence.Tool == "" || finding.Evidence.Job == "" || finding.Location.URI == "" || finding.Location.StartLine == 0 {
		return ""
	}
	return fmt.Sprintf("same-location-tool:%s:%s:%s:%d",
		strings.ToLower(finding.Evidence.Tool), strings.ToLower(finding.Evidence.Job),
		pathnorm.Normalize(finding.Location.URI), finding.Location.StartLine)
}

// hasDependency meldet, ob aus dem Fund überhaupt eine Dependency erkannt
// wurde — gleich ob strukturiert oder nur aus dem Freitext für die Anzeige.
//
// Die Bedingung ist dieselbe, an der extractDependency entscheidet, ob es eine
// Dependency herausgibt oder die leere Struktur, um die beiden Stellen nicht
// auseinanderlaufen zu lassen.
func hasDependency(dependency Dependency) bool {
	return len(dependency.IDs) > 0 || dependency.Package != "" || dependency.Version != "" ||
		dependency.TextPackage != "" || dependency.TextVersion != ""
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
		if aiPathRuleKey(finding) != "" {
			rules["ai-path-rule"] = true
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
		pathnorm.Normalize(left.URI) == pathnorm.Normalize(right.URI) && left.StartLine == right.StartLine
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

// parsePurl zerlegt einen Package-URL in Namen und Version.
//
//	pkg:pypi/requests@2.19.0            → requests, 2.19.0
//	pkg:golang/golang.org/x/sys@v0.41.0 → golang.org/x/sys, v0.41.0
//
// Der Typ (`pypi`, `golang`) fällt weg, der Namensteil bleibt vollständig: er
// darf selbst Slashes enthalten, und am ersten Slash zu trennen verstümmelte
// jeden Go-Modulpfad. Getrennt wird deshalb am ersten Slash **nach** dem Typ
// und an der letzten `@`.
//
// Die Version bleibt bis auf Kleinschreibung, wie sie steht — ein `v`-Präfix
// wird nicht abgeschnitten. Es abzuschneiden würde `v0.41.0` und `0.41.0`
// verschmelzen, und die Messung aus Task 028 zeigt keinen Fall, in dem zwei
// Werkzeuge dieselbe Version verschieden schreiben.
//
// Die Funktion ist rein und einparametrig wie pathnorm.Normalize; sie liest nur
// den übergebenen String.
func parsePurl(purl string) (string, string) {
	value := strings.TrimSpace(purl)
	if len(value) < len("pkg:") || !strings.EqualFold(value[:len("pkg:")], "pkg:") {
		return "", ""
	}
	value = value[len("pkg:"):]
	// Qualifier (`?arch=…`) und Subpath (`#pfad`) gehören nicht zum Namen.
	if cut := strings.IndexAny(value, "?#"); cut >= 0 {
		value = value[:cut]
	}
	slash := strings.Index(value, "/")
	if slash < 0 {
		// Ohne Typ/Name-Trennung ist kein Name zu benennen.
		return "", ""
	}
	value = value[slash+1:]
	version := ""
	if at := strings.LastIndex(value, "@"); at >= 0 {
		version = value[at+1:]
		value = value[:at]
	}
	return normalizePackageName(value), strings.ToLower(strings.TrimSpace(version))
}

// normalizePackageName bringt Paketnamen aus verschiedenen Quellen auf eine
// Schreibweise: Rand-Leerzeichen und Rand-Slashes weg, Kleinschreibung.
//
// Absichtlich **kein** Abschneiden am Slash: `golang.org/x/sys` ist ein Name,
// kein Präfix samt Name. Das Ecosystem-Präfix entfernt parsePurl, das als
// einziges weiß, wo es aufhört.
func normalizePackageName(name string) string {
	return strings.ToLower(strings.Trim(strings.TrimSpace(name), "/"))
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
