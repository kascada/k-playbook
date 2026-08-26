package merge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// commandModule liest ein Command-Modul aus dem Repo-Wurzelverzeichnis. Von
// installer/internal/review/merge/ sind das vier Ebenen aufwärts.
func commandModule(t *testing.T, parts ...string) string {
	t.Helper()
	elems := append([]string{"..", "..", "..", "..", "commands"}, parts...)
	path := filepath.Clean(filepath.Join(elems...))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Command-Modul %s lesen: %v", path, err)
	}
	return string(data)
}

// TestReviewScanTriageNutztCoverageAusJSON hält fest, dass die Triage die Deckung
// nicht selbst ermittelt. Die Feldnamen prüft dieser Test nicht mehr: sie stehen
// seit Task 032 im Belegvertrag und nicht im Triage-Modul — siehe
// TestReviewInputContractNenntCoverageFelder.
func TestReviewScanTriageNutztCoverageAusJSON(t *testing.T) {
	content := commandModule(t, "_audit", "review-scan-triage.md")
	if !strings.Contains(content, "Suche keine `known-decisions.md` und führe kein eigenes Matching aus") {
		t.Fatalf("Command erlaubt noch eigenes Matching")
	}
}

// TestReviewInputContractNenntCoverageFelder sichert die Gegenseite: Der
// Belegvertrag ist die einzige Stelle, die das Schema beschreibt, und muss die
// Coverage-Felder mit vollem Pfad benennen. Ohne sie wüsste die Triage nicht, woran
// sie eine gedeckte Gruppe erkennt.
func TestReviewInputContractNenntCoverageFelder(t *testing.T) {
	content := commandModule(t, "_review-run", "review-input-contract.md")
	for _, key := range []string{
		"groups[].coveredByKnownDecision",
		"groups[].partialCoverage",
		"groups[].knownDecisionCoverage",
		"knownDecisions",
	} {
		if !strings.Contains(content, key) {
			t.Fatalf("Belegvertrag erwähnt JSON-Coverage-Feld %s nicht", key)
		}
	}
}
