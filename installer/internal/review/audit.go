package review

import (
	"fmt"
	"strings"
)

// AuditContract ist der audit-Block eines Review-Rezepts, so wie er im
// Frontmatter steht — die Form, über die entschieden wird, ob das Rezept im
// Lauf als Perspektive oder als Evidence-Quelle taugt.
//
// Die Felder stehen hier nebeneinander, weil erst ihre Kombination gültig oder
// ungültig ist: resultRequired allein sagt nichts, resultRequired neben
// mode: evidence sagt, dass jemand zwei Ergebnisformen zugleich erwartet.
type AuditContract struct {
	// Mode ist audit.mode. Leer heißt perspective.
	Mode Mode
	// Scope ist audit.scope mit tools und paths.
	Scope *Scope
	// RuleIDs ist audit.ruleIds: die abschließende Liste der Rule-IDs, die ein
	// Evidence-Rezept vergeben darf. Sie hält die Funde über Läufe hinweg
	// vergleichbar — frei erfundene Rule-IDs je Lauf zerstörten das.
	RuleIDs []string
	// ResultRequiredSet meldet, ob audit.resultRequired im Rezept steht. Nicht
	// der Wert zählt, sondern dass jemand ihn gesetzt hat: für mode: evidence
	// gibt es keine Ergebnisdatei, über die er entscheiden könnte.
	ResultRequiredSet bool
	// DefaultResult ist audit.defaultResult.
	DefaultResult string
}

// ValidateAuditContract prüft die zulässigen Kombinationen im audit-Block eines
// Rezepts.
//
// mode: perspective ist die Vorgabe und bleibt unverändert wie bisher: Der
// Eintrag läuft nach dem Merge, filtert die Gruppen aus review-input.json über
// scope.tools und schreibt ein Markdown-Ergebnis. Die Evidence-Felder
// scope.paths und ruleIds haben dort keine Wirkung und gelten deshalb als
// Fehler statt still zu verschwinden.
//
// mode: evidence liest Code im Pfad-Scope und schreibt SARIF nach
// raw/<entry>.sarif. Das SARIF ist das Pflichtartefakt und ersetzt das
// Markdown-Ergebnis; ein zweites Ergebnisdokument entsteht nicht. Deshalb darf
// ein Evidence-Rezept weder defaultResult noch resultRequired setzen — beide
// beschreiben die Ergebnisdatei, die es nicht mehr gibt.
//
// Verlangt werden umgekehrt scope.paths und ruleIds: ohne Pfad-Scope läse der
// Eintrag das ganze Repo, ohne Rule-ID-Liste wären seine Funde von Lauf zu Lauf
// nicht vergleichbar.
func ValidateAuditContract(contract AuditContract) error {
	if !ValidMode(contract.Mode) {
		return fmt.Errorf("audit.mode: unbekannte Betriebsart %q — zulässig sind %q und %q", contract.Mode, ModePerspective, ModeEvidence)
	}
	tools, paths := scopeLists(contract.Scope)
	ruleIDs := nonEmpty(contract.RuleIDs)

	if NormalizeMode(contract.Mode) == ModePerspective {
		if len(paths) > 0 {
			return fmt.Errorf("audit.scope.paths gilt nur für mode: %s", ModeEvidence)
		}
		if len(ruleIDs) > 0 {
			return fmt.Errorf("audit.ruleIds gilt nur für mode: %s", ModeEvidence)
		}
		return nil
	}

	if len(paths) == 0 {
		return fmt.Errorf("mode: %s verlangt audit.scope.paths", ModeEvidence)
	}
	if len(ruleIDs) == 0 {
		return fmt.Errorf("mode: %s verlangt audit.ruleIds", ModeEvidence)
	}
	if len(tools) > 0 {
		return fmt.Errorf("mode: %s verträgt kein audit.scope.tools — der Eintrag liest Code, nicht review-input.json", ModeEvidence)
	}
	if contract.ResultRequiredSet {
		return fmt.Errorf("mode: %s verträgt kein audit.resultRequired — Pflichtartefakt ist raw/<entry>.sarif", ModeEvidence)
	}
	if strings.TrimSpace(contract.DefaultResult) != "" {
		return fmt.Errorf("mode: %s verträgt kein audit.defaultResult — Pflichtartefakt ist raw/<entry>.sarif", ModeEvidence)
	}
	return nil
}

func scopeLists(scope *Scope) ([]string, []string) {
	if scope == nil {
		return nil, nil
	}
	return nonEmpty(scope.Tools), nonEmpty(scope.Paths)
}

func nonEmpty(values []string) []string {
	kept := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			kept = append(kept, value)
		}
	}
	return kept
}
