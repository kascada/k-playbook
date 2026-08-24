package sarifconvert

import (
	"encoding/json"
	"fmt"
	"strings"
)

// pipAuditMessageLimit ist die Länge, auf die die Beschreibung einer
// Schwachstelle für message.text gekürzt wird.
//
// pip-audit liefert dort mehrere hundert Zeichen Markdown-Freitext — bei
// GHSA-Einträgen ganze Abschnitte mit Impact, Patches und Links. Ungekürzt
// trüge das jede Ergebnisansicht mit. Dieselbe Größenordnung wie
// firstMatchLine() (review/scanners.go), das die Werkzeugausgabe auf 200
// Zeichen kürzt; die vollständige Beschreibung steht weiter im Fund selbst,
// unter der Kennung, mit der er nachzuschlagen ist.
const pipAuditMessageLimit = 300

// pipAuditUnknownRule tritt an die Stelle einer Schwachstelle ohne jede
// Kennung — weder id noch aliases.
//
// Verworfen wird sie nicht: ein Fund ohne Kennung ist immer noch ein Fund, und
// stillschweigend fehlen dürfte er nicht. Beobachtet wurde der Fall nicht;
// pip-audits Einträge kommen aus einer Schwachstellendatenbank und tragen dort
// immer eine ID.
const pipAuditUnknownRule = "PIP-AUDIT-UNKNOWN"

// pipAuditReport ist pip-audits JSON-Ausgabe (Aufruf `--format json`),
// beschränkt auf die Felder, die ins SARIF einfließen.
//
// Schema, verifiziert gegen pip-audit 2.10.0:
//
//	{"dependencies":[{"name","version","vulns":[{"id","fix_versions","aliases","description"}]}],"fixes":[…]}
//
// fixes bleibt außen vor: es fasst nur zusammen, was an den Vulns schon steht.
// Ein Paket ohne Schwachstellen trägt eine leere vulns-Liste — es ist geprüft
// und sauber und ergibt deshalb kein Result. Zu den Paketen zählen auch die
// transitiven: pip-audit löst die Abhängigkeiten des Manifests auf und prüft
// sie mit.
type pipAuditReport struct {
	Dependencies []pipAuditDependency `json:"dependencies"`
}

type pipAuditDependency struct {
	Name    string         `json:"name"`
	Version string         `json:"version"`
	Vulns   []pipAuditVuln `json:"vulns"`
}

type pipAuditVuln struct {
	ID          string   `json:"id"`
	FixVersions []string `json:"fix_versions"`
	Aliases     []string `json:"aliases"`
	Description string   `json:"description"`
}

// PipAudit wandelt pip-audits JSON-Ausgabe (Katalog-Aufruf `--format json
// --progress-spinner off -r {module}`, siehe scripts/scanners.tsv) in valides
// SARIF 2.1.0 um: ein Result je Schwachstelle, nicht je Paket — ein Paket mit
// sechs bekannten Lücken ist sechsmal zu beheben, nicht einmal.
//
// manifest ist die geprüfte Datei, relativ zum Ziel. pip-audit nennt sie in
// seiner Ausgabe nicht, der Aufrufer kennt sie aber, weil er sie als -r
// übergeben hat (execute.go bindet sie an den Job). Ohne die Angabe stünde am
// zusammengeführten Befund nicht, welches Manifest betroffen ist
// (merge.extractDependency); leer heißt „unbekannt", dann bleiben Property und
// Fundstelle weg statt falsch zu sein.
//
// Leere oder unlesbare Eingabe ist hier — anders als bei TruffleHog — ein
// Fehler: pip-audit schreibt auch ohne einen einzigen Fund ein vollständiges
// JSON-Dokument. Nichts zu bekommen heißt deshalb, dass der Lauf nicht
// zustande kam, und ein leeres SARIF sähe an dieser Stelle wie ein sauberer
// Scan aus.
func PipAudit(raw []byte, manifest string) ([]byte, error) {
	var report pipAuditReport
	if err := json.Unmarshal(raw, &report); err != nil {
		return nil, fmt.Errorf("pip-audit-JSON ließ sich nicht lesen: %w", err)
	}

	driver := sarifDriver{Name: "pip-audit", Rules: []sarifRule{}}
	ruleIndex := map[string]bool{}
	results := []sarifResult{}

	for _, dependency := range report.Dependencies {
		for _, vuln := range dependency.Vulns {
			ruleID := pipAuditRuleID(vuln)
			if !ruleIndex[ruleID] {
				rule := sarifRule{ID: ruleID, Name: ruleID}
				if summary := shortenPipAuditText(vuln.Description); summary != "" {
					rule.ShortDescription = &sarifText{Text: summary}
				}
				driver.Rules = append(driver.Rules, rule)
				ruleIndex[ruleID] = true
			}

			results = append(results, sarifResult{
				RuleID:     ruleID,
				Message:    sarifText{Text: pipAuditMessage(dependency, vuln, ruleID)},
				Locations:  pipAuditLocations(manifest),
				Properties: pipAuditProperties(dependency, vuln, manifest),
			})
		}
	}

	document := sarifLog{
		Schema:  sarifSchema,
		Version: sarifVersion,
		Runs: []sarifRun{{
			Tool:    sarifTool{Driver: driver},
			Results: results,
		}},
	}

	data, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("SARIF ließ sich nicht erzeugen: %w", err)
	}
	return append(data, '\n'), nil
}

// pipAuditRuleID wählt die Kennung, unter der ein Fund geführt wird: bevorzugt
// die CVE-Alias, sonst pip-audits eigene ID (PYSEC-…), sonst die erste andere
// Alias.
//
// Die CVE-Nummer zuerst, weil dieselbe Schwachstelle bei den anderen
// Werkzeugen desselben Laufs (trivy, grype, osv-scanner) unter ihr steht —
// nur so lässt sich ein Befund über die Werkzeuge hinweg wiedererkennen. Die
// pip-audit-eigene ID geht darüber nicht verloren: sie steht in den
// Properties.
func pipAuditRuleID(vuln pipAuditVuln) string {
	for _, alias := range vuln.Aliases {
		if strings.HasPrefix(strings.ToUpper(alias), "CVE-") {
			return strings.ToUpper(alias)
		}
	}
	if vuln.ID != "" {
		return vuln.ID
	}
	for _, alias := range vuln.Aliases {
		if alias != "" {
			return alias
		}
	}
	return pipAuditUnknownRule
}

// pipAuditMessage schreibt den Text eines Results so, dass er allein steht:
// betroffenes Paket samt Version, die Kennung, die gekürzte Beschreibung und
// die Version, in der die Lücke behoben ist.
//
// Paket und Version stehen zwar auch in den Properties, aber die liest nur die
// Zusammenführung; in einer Ergebnisansicht ist der Text alles, was ankommt.
func pipAuditMessage(dependency pipAuditDependency, vuln pipAuditVuln, ruleID string) string {
	parts := []string{fmt.Sprintf("%s %s: %s", dependency.Name, dependency.Version, ruleID)}
	if description := shortenPipAuditText(vuln.Description); description != "" {
		parts = append(parts, description)
	}
	if len(vuln.FixVersions) > 0 {
		parts = append(parts, "behoben in "+strings.Join(vuln.FixVersions, ", "))
	}
	return strings.Join(parts, " — ")
}

// shortenPipAuditText macht aus pip-audits Freitext eine Zeile: Zeilenumbrüche
// und Mehrfach-Leerzeichen fallen weg, danach wird auf pipAuditMessageLimit
// gekürzt.
//
// Gekürzt wird über die Runen und nicht über die Bytes: ein Schnitt mitten in
// einem Mehrbyte-Zeichen ergäbe ungültiges UTF-8 im SARIF.
func shortenPipAuditText(text string) string {
	runes := []rune(strings.Join(strings.Fields(text), " "))
	if len(runes) <= pipAuditMessageLimit {
		return string(runes)
	}
	return strings.TrimRight(string(runes[:pipAuditMessageLimit]), " ") + "…"
}

// pipAuditProperties setzt die Konvention, an der merge.extractDependency eine
// Abhängigkeit erkennt: package, version, manifest.
//
// id, aliases und fixVersions kommen dazu, weil sie sonst nirgends stünden:
// die CVE-Alias hat unter Umständen die pip-audit-eigene ID aus ruleId
// verdrängt (siehe pipAuditRuleID), und die Fix-Version ist die eine Angabe,
// mit der aus einem Befund eine Handlung wird.
//
// severity, impact und priority stehen mit Absicht nicht darin: pip-audit
// liefert keine Schwere. Ein gesetzter Wert gälte in deriveSeverity
// (merge/severity.go) als tool-metadata und wäre damit eine Angabe des
// Werkzeugs, die es nie gemacht hat. Ohne sie bleibt die Schwere unmapped —
// das ist die richtige Auskunft.
func pipAuditProperties(dependency pipAuditDependency, vuln pipAuditVuln, manifest string) map[string]string {
	properties := map[string]string{
		"package": dependency.Name,
		"version": dependency.Version,
	}
	if manifest != "" {
		properties["manifest"] = manifest
	}
	if vuln.ID != "" {
		properties["id"] = vuln.ID
	}
	if len(vuln.Aliases) > 0 {
		properties["aliases"] = strings.Join(vuln.Aliases, ", ")
	}
	if len(vuln.FixVersions) > 0 {
		properties["fixVersions"] = strings.Join(vuln.FixVersions, ", ")
	}
	return properties
}

// pipAuditLocations zeigt auf das geprüfte Manifest, wenn es bekannt ist.
//
// Eine Zeilenangabe gibt es nicht: pip-audit sagt nicht, in welcher Zeile das
// Paket steht, und bei einer transitiven Abhängigkeit steht es überhaupt in
// keiner. Eine erfundene Zeile wäre schlechter als keine.
func pipAuditLocations(manifest string) []sarifLocation {
	if manifest == "" {
		return nil
	}
	location := sarifLocation{}
	location.PhysicalLocation.ArtifactLocation.URI = manifest
	return []sarifLocation{location}
}
