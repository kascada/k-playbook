package merge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// commandModule liest ein Command-Modul aus defaults/commands/ im Repo — dort
// liegen Triage-Modul und Belegvertrag seit Task 197. Von
// installer/internal/review/merge/ sind das vier Ebenen aufwärts.
func commandModule(t *testing.T, parts ...string) string {
	t.Helper()
	elems := append([]string{"..", "..", "..", "..", "defaults", "commands"}, parts...)
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

// TestReviewScanTriageIstAbhakliste hält das Format der Ergebnisdatei fest
// (Task 197): eine Tabelle je Bündel mit der Spalte „erledigt“, nach
// Priorität sortiert, darunter jeder Fund im Detail, und ein Restabschnitt,
// damit kein Fund still herausfällt.
func TestReviewScanTriageIstAbhakliste(t *testing.T) {
	content := commandModule(t, "_audit", "review-scan-triage.md")
	for _, needle := range []string{
		"| erledigt | ID | Priorität | Kategorie | Titel | Funde |",
		"**Eine Zeile je Bündel**, sortiert nach Priorität `P1` → `P2` → `P3`",
		"### R — Nicht gebündelt",
		"**Kein Fund fällt still heraus.**",
		"| Ort | Rule-ID | Level | Meldung |",
		"Bekannte Entscheidungen:",
	} {
		if !strings.Contains(content, needle) {
			t.Errorf("Triage-Modul nennt %q nicht", needle)
		}
	}
	for _, stale := range []string{"## Bündel-Details", "Family-Date", "Report-Modus", "## Deckung aus known-decisions"} {
		if strings.Contains(content, stale) {
			t.Errorf("Triage-Modul trägt noch %q", stale)
		}
	}
}

// TestReviewInputContractOhneReportWeg: Den von Hand geschriebenen Beleg gibt
// es nicht mehr (Task 197); der Vertrag kennt nur den Go-Merge.
func TestReviewInputContractOhneReportWeg(t *testing.T) {
	content := commandModule(t, "_review-run", "review-input-contract.md")
	for _, stale := range []string{"Report-Weg", "merge-only", "## Kern", "ai-<eintrag>-<schlüssel>", "result-family"} {
		if strings.Contains(content, stale) {
			t.Errorf("Belegvertrag trägt noch %q", stale)
		}
	}
	if !strings.Contains(content, "k_playbook_review_merge_dir") {
		t.Error("Belegvertrag nennt den Merge auf ein Arbeitsverzeichnis nicht")
	}
}
