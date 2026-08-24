// Package sarifconvert wandelt die native Ausgabe von Werkzeugen ohne eigenes
// SARIF in valides SARIF 2.1.0 um. Jede Funktion ist rein: sie liest nur ihr
// []byte-Argument und liefert nur ihr Ergebnis — der Aufrufer (execute.go)
// entscheidet, woher der Rohtext kommt und wohin das SARIF geschrieben wird.
package sarifconvert

import (
	"bytes"
	"encoding/json"
	"fmt"
)

const (
	sarifSchema  = "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/master/Schemata/sarif-schema-2.1.0.json"
	sarifVersion = "2.1.0"
)

// sarifLog ist so viel von SARIF 2.1.0, wie ein Konverter zum Schreiben
// braucht. Das Gegenstück beim Lesen ist merge.readSARIF (review/merge/merge.go).
type sarifLog struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name  string      `json:"name"`
	Rules []sarifRule `json:"rules"`
}

type sarifRule struct {
	ID               string     `json:"id"`
	Name             string     `json:"name,omitempty"`
	ShortDescription *sarifText `json:"shortDescription,omitempty"`
}

type sarifText struct {
	Text string `json:"text"`
}

type sarifResult struct {
	RuleID    string          `json:"ruleId"`
	Level     string          `json:"level"`
	Message   sarifText       `json:"message"`
	Locations []sarifLocation `json:"locations,omitempty"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysicalLocation `json:"physicalLocation"`
}

type sarifPhysicalLocation struct {
	ArtifactLocation sarifArtifactLocation `json:"artifactLocation"`
	Region           *sarifRegion          `json:"region,omitempty"`
}

type sarifArtifactLocation struct {
	URI string `json:"uri"`
}

type sarifRegion struct {
	StartLine int `json:"startLine"`
}

// truffleHogFinding ist eine Fund-Zeile aus trufflehogs NDJSON-Ausgabe
// (Aufruf `--json`), beschränkt auf die Felder, die ins SARIF-Ergebnis
// einfließen dürfen.
//
// Absichtlich nicht gelesen: Raw, RawV2, SecretParts und ExtraData — sie
// können den Wert eines Secrets im Klartext tragen. Einzige Textquelle fürs
// Ergebnis ist Redacted, trufflehogs eigene gekürzte Fassung.
type truffleHogFinding struct {
	SourceMetadata struct {
		Data struct {
			Git struct {
				File string `json:"file"`
				Line int    `json:"line"`
			} `json:"Git"`
		} `json:"Data"`
	} `json:"SourceMetadata"`
	DetectorName        string `json:"DetectorName"`
	DetectorDescription string `json:"DetectorDescription"`
	Verified            bool   `json:"Verified"`
	Redacted            string `json:"Redacted"`
}

// TruffleHog wandelt trufflehogs NDJSON-Ausgabe (Aufruf `git file://{target}
// --json --no-update`, siehe scripts/scanners.tsv) in valides SARIF 2.1.0 um.
//
// Jede Zeile ohne erkennbaren DetectorName wird verworfen — im tatsächlichen
// Aufrufpfad (execute.go, runJob) laufen Log-Meldungen ohnehin ausschließlich
// über stderr, getrennt von diesem stdout-Text; das Verwerfen ist eine
// defensive Absicherung, keine beobachtete Notwendigkeit (siehe Task
// 024-trufflehog-sarif-konverter.md, Abschnitt „Kontext").
//
// Leere Eingabe — der Normalfall bei 0 Funden im echten Aufrufpfad — liefert
// ein valides, leeres SARIF-Log statt eines Fehlers.
func TruffleHog(raw []byte) ([]byte, error) {
	findings := parseTruffleHogLines(raw)

	driver := sarifDriver{Name: "trufflehog", Rules: []sarifRule{}}
	ruleIndex := map[string]bool{}
	results := make([]sarifResult, 0, len(findings))

	for _, finding := range findings {
		if !ruleIndex[finding.DetectorName] {
			rule := sarifRule{ID: finding.DetectorName, Name: finding.DetectorName}
			if finding.DetectorDescription != "" {
				rule.ShortDescription = &sarifText{Text: finding.DetectorDescription}
			}
			driver.Rules = append(driver.Rules, rule)
			ruleIndex[finding.DetectorName] = true
		}

		results = append(results, sarifResult{
			RuleID:    finding.DetectorName,
			Level:     truffleHogLevel(finding.Verified),
			Message:   sarifText{Text: truffleHogMessage(finding)},
			Locations: truffleHogLocations(finding),
		})
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

// parseTruffleHogLines liest raw zeilenweise NDJSON und liefert nur die
// Zeilen, die als Fund erkennbar sind. Eine Zeile, die sich nicht als JSON
// lesen lässt oder kein DetectorName trägt, wird verworfen statt den ganzen
// Lauf scheitern zu lassen — beides ist für diese Funktion gleichwertig
// „keine Fund-Zeile".
func parseTruffleHogLines(raw []byte) []truffleHogFinding {
	findings := []truffleHogFinding{}
	for _, line := range bytes.Split(raw, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var finding truffleHogFinding
		if err := json.Unmarshal(line, &finding); err != nil {
			continue
		}
		if finding.DetectorName == "" {
			continue
		}
		findings = append(findings, finding)
	}
	return findings
}

// truffleHogLevel bildet Verified auf SARIF-level ab: ein verifiziertes
// Credential ist ein reales, aktives Secret (error); unverifiziert bleibt
// warning. deriveSeverity (merge/severity.go) bevorzugt dieses native level
// ohnehin vor jeder Mapping-Tabelle.
func truffleHogLevel(verified bool) string {
	if verified {
		return "error"
	}
	return "warning"
}

// truffleHogMessage liefert den Text eines Results — ausschließlich aus
// Redacted, nie aus Raw/RawV2. Manche Detektoren (z. B. SlackWebhook) lassen
// Redacted leer; dann tritt ein eigener, secretfreier Platzhalter an ihre
// Stelle statt eines leeren message.text.
func truffleHogMessage(finding truffleHogFinding) string {
	if finding.Redacted != "" {
		return finding.Redacted
	}
	return fmt.Sprintf("%s-Fund (trufflehog hat keinen gekürzten Wert geliefert)", finding.DetectorName)
}

// truffleHogLocations liefert die Fundstelle aus SourceMetadata.Data.Git,
// wenn eine Datei bekannt ist; sonst keine Location.
func truffleHogLocations(finding truffleHogFinding) []sarifLocation {
	git := finding.SourceMetadata.Data.Git
	if git.File == "" {
		return nil
	}
	location := sarifLocation{}
	location.PhysicalLocation.ArtifactLocation.URI = git.File
	if git.Line > 0 {
		location.PhysicalLocation.Region = &sarifRegion{StartLine: git.Line}
	}
	return []sarifLocation{location}
}
