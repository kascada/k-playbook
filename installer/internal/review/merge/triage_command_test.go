package merge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReviewScanTriageNutztCoverageAusJSON(t *testing.T) {
	path := filepath.Clean(filepath.Join("..", "..", "..", "..", "commands", "_audit", "review-scan-triage.md"))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Command-Modul lesen: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "Suche keine `known-decisions.md` und führe kein eigenes Matching aus") {
		t.Fatalf("Command erlaubt noch eigenes Matching")
	}
	for _, key := range []string{"groups[].coveredByKnownDecision", "groups[].partialCoverage", "groups[].knownDecisionCoverage", "knownDecisions"} {
		if !strings.Contains(content, key) {
			t.Fatalf("Command erwähnt JSON-Coverage-Feld %s nicht", key)
		}
	}
}
