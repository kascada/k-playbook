package review

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// EvidenceSARIFSuffix ist die Endung des Pflichtartefakts eines
// Evidence-Eintrags. Der Pfad selbst ist raw/<entry>.sarif — siehe
// EvidenceSARIFPath.
const EvidenceSARIFSuffix = ".sarif"

// evidenceReasonPaths ist die Zahl der Pfade, die in den Grund geschrieben
// werden. Der Grund soll sagen, wohin ein Rezept gegriffen hat, und nicht die
// verworfene Liste noch einmal abdrucken.
const evidenceReasonPaths = 3

// EvidenceSARIFPath ist der Ort des Pflichtartefakts eines Evidence-Eintrags,
// relativ zum Laufverzeichnis.
func EvidenceSARIFPath(entry string) string {
	return RawDirName + "/" + entry + EvidenceSARIFSuffix
}

// EvidenceReport ist das Ergebnis der Prüfung eines Evidence-SARIF.
//
// Verworfen wird ein Fund nur wegen seines Ortes: Alles andere — unlesbares
// JSON, fremder Werkzeugname, eine Rule-ID außerhalb der Liste — ist ein Fehler
// des ganzen Artefakts und kommt als error zurück. Die Trennung ist die
// Teilannahme: ein einzelner Ausreißer im Scope vernichtet nicht das ganze
// SARIF, eine erfundene Rule-ID dagegen macht die Funde unvergleichbar und darf
// nicht still durchgehen.
type EvidenceReport struct {
	// Kept ist die Zahl der Funde, die im Scope liegen und stehen bleiben.
	Kept int
	// Dropped ist die Zahl der Funde außerhalb des Scopes.
	Dropped int
	// DroppedPaths sind die Fundorte der verworfenen Funde, ohne Wiederholung
	// und in der Reihenfolge des SARIF.
	DroppedPaths []string
	// Cleaned ist das bereinigte SARIF. Es ist nil, wenn nichts verworfen
	// wurde — dann bleibt die Datei unangetastet.
	Cleaned []byte
}

// ScopeNote ist der Satz für den Grund des Eintrags: wie viele Funde außerhalb
// des Scopes lagen und wo die ersten davon standen. Leer, wenn nichts verworfen
// wurde.
func (report EvidenceReport) ScopeNote() string {
	if report.Dropped == 0 {
		return ""
	}
	note := fmt.Sprintf("%d Fund(e) außerhalb von scope.paths verworfen", report.Dropped)
	if len(report.DroppedPaths) == 0 {
		return note + "."
	}
	shown := report.DroppedPaths
	suffix := ""
	if len(shown) > evidenceReasonPaths {
		shown = shown[:evidenceReasonPaths]
		suffix = ", …"
	}
	return note + ": " + strings.Join(shown, ", ") + suffix + "."
}

// CheckEvidenceSARIF prüft das SARIF eines Evidence-Eintrags und entfernt die
// Funde außerhalb des Pfad-Scopes.
//
// Geprüft wird, was das Melden erzwingen kann: der Werkzeugname jedes SARIF-Runs
// muss der Eintragsname sein, und jede Rule-ID muss in der Liste des Rezepts
// stehen. Beides ist ein Fehler des Artefakts, kein Fund-Problem — ein Rezept,
// das seine eigenen Rule-IDs erfindet, macht seine Funde über Läufe hinweg
// unvergleichbar.
//
// Der Scope wirkt dagegen je Fund: Wer außerhalb von scopePaths oder in einem
// der zentralen Ausschlüsse (PathExcludedFromScope) liegt, fällt heraus, und
// das Artefakt bleibt gültig. Maßgeblich ist der erste Fundort eines Results —
// derselbe Ort, über den auch der Merge gruppiert. Ein Fund ohne Ort lässt sich
// keinem Scope zuordnen und fällt deshalb ebenfalls heraus.
//
// Ein SARIF ohne Ergebnisse ist kein Fehler: ein leerer Scope-Befund ist ein
// Ergebnis.
func CheckEvidenceSARIF(data []byte, entry string, ruleIDs []string, scopePaths []string) (EvidenceReport, error) {
	root, err := decodeSARIF(data)
	if err != nil {
		return EvidenceReport{}, err
	}
	rawRuns, found := root["runs"]
	if !found {
		return EvidenceReport{}, errors.New("SARIF ohne runs")
	}
	runs, ok := rawRuns.([]any)
	if !ok {
		return EvidenceReport{}, errors.New("SARIF-Feld runs ist keine Liste")
	}

	allowed := map[string]bool{}
	for _, id := range nonEmpty(ruleIDs) {
		allowed[strings.TrimSpace(id)] = true
	}

	report := EvidenceReport{}
	seenDropped := map[string]bool{}
	changed := false
	for _, rawRun := range runs {
		run, ok := rawRun.(map[string]any)
		if !ok {
			return EvidenceReport{}, errors.New("SARIF-Eintrag in runs ist kein Objekt")
		}
		if err := checkEvidenceToolName(run, entry); err != nil {
			return EvidenceReport{}, err
		}
		results, ok, err := sarifResults(run)
		if err != nil {
			return EvidenceReport{}, err
		}
		if !ok {
			continue
		}
		kept := make([]any, 0, len(results))
		for _, rawResult := range results {
			result, ok := rawResult.(map[string]any)
			if !ok {
				return EvidenceReport{}, errors.New("SARIF-Eintrag in results ist kein Objekt")
			}
			ruleID, err := evidenceRuleID(run, result)
			if err != nil {
				return EvidenceReport{}, err
			}
			if !allowed[ruleID] {
				return EvidenceReport{}, fmt.Errorf("Rule-ID %q steht nicht in audit.ruleIds (%s)", ruleID, strings.Join(nonEmpty(ruleIDs), ", "))
			}
			location := evidenceLocation(result)
			if evidenceInScope(location, scopePaths) {
				kept = append(kept, rawResult)
				report.Kept++
				continue
			}
			report.Dropped++
			changed = true
			marker := location
			if marker == "" {
				marker = "(ohne Fundort)"
			}
			if !seenDropped[marker] {
				seenDropped[marker] = true
				report.DroppedPaths = append(report.DroppedPaths, marker)
			}
		}
		run["results"] = kept
	}

	if !changed {
		return report, nil
	}
	cleaned, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return EvidenceReport{}, err
	}
	report.Cleaned = append(cleaned, '\n')
	return report, nil
}

// decodeSARIF liest das Dokument mit UseNumber: Zahlen bleiben so stehen, wie
// sie im SARIF standen. Ohne das würde jede Zeilennummer beim Zurückschreiben
// durch float64 laufen — aus 1 würde je nach Größe 1e+06.
func decodeSARIF(data []byte) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var document any
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("kein lesbares SARIF-JSON: %w", err)
	}
	root, ok := document.(map[string]any)
	if !ok {
		return nil, errors.New("SARIF-Wurzel ist kein Objekt")
	}
	return root, nil
}

// checkEvidenceToolName vergleicht den Werkzeugnamen eines SARIF-Runs mit dem
// Eintragsnamen. Der Merge trägt den Eintragsnamen als evidence.tool ein; steht
// im SARIF ein anderer, behauptet das Artefakt eine Herkunft, die es nicht hat.
func checkEvidenceToolName(run map[string]any, entry string) error {
	tool, ok := run["tool"].(map[string]any)
	if !ok {
		return errors.New("SARIF-Run ohne tool")
	}
	driver, ok := tool["driver"].(map[string]any)
	if !ok {
		return errors.New("SARIF-Run ohne tool.driver")
	}
	name, _ := driver["name"].(string)
	if strings.TrimSpace(name) != entry {
		return fmt.Errorf("SARIF-Werkzeugname %q entspricht nicht dem Eintrag %q", name, entry)
	}
	return nil
}

// sarifResults liefert die Results eines Runs. Fehlt das Feld, hat der Run
// keine Funde — das ist erlaubt und wird nicht zum Fehler.
func sarifResults(run map[string]any) ([]any, bool, error) {
	raw, found := run["results"]
	if !found || raw == nil {
		return nil, false, nil
	}
	results, ok := raw.([]any)
	if !ok {
		return nil, false, errors.New("SARIF-Feld results ist keine Liste")
	}
	return results, true, nil
}

// evidenceRuleID löst die Rule-ID eines Results so auf wie der Merge: erst
// ruleId, dann ruleIndex über tool.driver.rules. Ohne Rule-ID ist der Fund
// nicht gegen die Liste des Rezepts prüfbar.
func evidenceRuleID(run map[string]any, result map[string]any) (string, error) {
	if id, ok := result["ruleId"].(string); ok && strings.TrimSpace(id) != "" {
		return strings.TrimSpace(id), nil
	}
	index, ok := sarifNumber(result["ruleIndex"])
	if !ok {
		return "", errors.New("SARIF-Fund ohne ruleId")
	}
	rules := sarifRules(run)
	if index < 0 || index >= len(rules) {
		return "", fmt.Errorf("SARIF-Fund mit ruleIndex %d außerhalb von tool.driver.rules", index)
	}
	rule, ok := rules[index].(map[string]any)
	if !ok {
		return "", errors.New("SARIF-Regel ist kein Objekt")
	}
	id, _ := rule["id"].(string)
	if strings.TrimSpace(id) == "" {
		return "", errors.New("SARIF-Fund ohne ruleId")
	}
	return strings.TrimSpace(id), nil
}

func sarifRules(run map[string]any) []any {
	tool, ok := run["tool"].(map[string]any)
	if !ok {
		return nil
	}
	driver, ok := tool["driver"].(map[string]any)
	if !ok {
		return nil
	}
	rules, _ := driver["rules"].([]any)
	return rules
}

// sarifNumber liest eine Ganzzahl aus einem mit UseNumber gelesenen Dokument.
func sarifNumber(value any) (int, bool) {
	number, ok := value.(json.Number)
	if !ok {
		return 0, false
	}
	parsed, err := number.Int64()
	if err != nil {
		return 0, false
	}
	return int(parsed), true
}

// evidenceLocation ist der erste Fundort eines Results, normalisiert auf einen
// Pfad relativ zur Projektwurzel.
func evidenceLocation(result map[string]any) string {
	locations, ok := result["locations"].([]any)
	if !ok || len(locations) == 0 {
		return ""
	}
	location, ok := locations[0].(map[string]any)
	if !ok {
		return ""
	}
	physical, ok := location["physicalLocation"].(map[string]any)
	if !ok {
		return ""
	}
	artifact, ok := physical["artifactLocation"].(map[string]any)
	if !ok {
		return ""
	}
	uri, _ := artifact["uri"].(string)
	return NormalizeScopePath(uri)
}

// evidenceInScope meldet, ob ein Fundort im Scope liegt: innerhalb der Globs
// und außerhalb der zentralen Ausschlüsse. Ein leerer Ort liegt nie im Scope.
func evidenceInScope(path string, scopePaths []string) bool {
	if path == "" {
		return false
	}
	if PathExcludedFromScope(path) {
		return false
	}
	return PathInScope(path, scopePaths)
}

// NormalizeScopePath macht aus einer SARIF-URI einen Pfad, wie ihn ein Glob aus
// scope.paths trifft: Schrägstriche als Trenner, ohne file://-Schema und ohne
// führendes ./ oder /.
//
// Die Groß-/Kleinschreibung bleibt, anders als bei der Normalisierung im Merge:
// dort geht es darum, denselben Fund zweier Werkzeuge zusammenzuführen, hier um
// die Frage, ob eine Datei in einem erlaubten Bereich liegt. Diese Frage
// beantwortet das Dateisystem, und unter Linux beantwortet es sie nach
// Groß-/Kleinschreibung.
func NormalizeScopePath(uri string) string {
	path := strings.ReplaceAll(strings.TrimSpace(uri), "\\", "/")
	if lower := strings.ToLower(path); strings.HasPrefix(lower, "file://") {
		path = path[len("file://"):]
		if slash := strings.Index(path, "/"); slash >= 0 {
			path = path[slash:]
		}
	}
	path = strings.TrimPrefix(path, "/")
	for strings.HasPrefix(path, "./") {
		path = strings.TrimPrefix(path, "./")
	}
	return path
}

// PathInScope meldet, ob ein Pfad von einem der Globs getroffen wird.
//
// Verglichen wird Segment für Segment; ** überspringt beliebig viele Segmente,
// * und ? gelten innerhalb eines Segments (filepath.Match). Zusätzlich trifft
// ein Muster eine Datei, wenn es ihr Verzeichnis trifft: installer und
// installer/** decken beide installer/internal/review/run.go. scope.paths
// benennt Bereiche und keine Dateilisten — ein Muster ohne ** wäre sonst
// wirkungslos, und das fiele erst auf, wenn ein Rezept jeden Fund verlöre.
//
// Ohne Globs liegt nichts im Scope. Das ist die Vorsichtsrichtung: ein
// Evidence-Eintrag ohne Pfad-Scope kommt gar nicht erst in den Lauf
// (ValidateAuditContract), und wo er es doch täte, soll er nicht das ganze Repo
// melden dürfen.
func PathInScope(path string, scopePaths []string) bool {
	path = NormalizeScopePath(path)
	if path == "" {
		return false
	}
	segments := strings.Split(path, "/")
	for _, pattern := range nonEmpty(scopePaths) {
		if matchScopeSegments(strings.Split(NormalizeScopePath(pattern), "/"), segments) {
			return true
		}
	}
	return false
}

// matchScopeSegments vergleicht Muster- und Pfadsegmente. Ein aufgebrauchtes
// Muster trifft auch dann, wenn noch Pfadsegmente übrig sind: das ist die
// Verzeichnisregel aus PathInScope.
func matchScopeSegments(pattern []string, value []string) bool {
	if len(pattern) == 0 {
		return true
	}
	if pattern[0] == "**" {
		if matchScopeSegments(pattern[1:], value) {
			return true
		}
		return len(value) > 0 && matchScopeSegments(pattern, value[1:])
	}
	if len(value) == 0 {
		return false
	}
	matched, err := filepath.Match(pattern[0], value[0])
	if err != nil || !matched {
		return false
	}
	return matchScopeSegments(pattern[1:], value[1:])
}
