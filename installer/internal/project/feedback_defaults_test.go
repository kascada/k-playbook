package project

import (
	"strings"
	"testing"
)

// /k-feedback ist eine dünne Hülle über k-playbook:/commands/k-feedback.md;
// die Hülle erlaubt die drei Feedback-Werkzeuge, der Text beginnt mit dem
// ersten Schritt, wählt den Zugriffsweg MCP vor dem Subkommando und verbietet
// den direkten Zugriff auf feedback/. Die Guideline „Betrieb“ trägt den
// Ablageort.
func TestDefaultsKFeedbackIstHuelleUndText(t *testing.T) {
	hull := readRepoShare(t, "commands/k-feedback.md")
	for _, needle := range []string{"k-playbook:/commands/k-feedback.md", "$ARGUMENTS", "mcp__k-playbook__k_playbook_doc_read",
		"mcp__k-playbook__k_playbook_feedback_list", "mcp__k-playbook__k_playbook_feedback_create",
		"mcp__k-playbook__k_playbook_feedback_update", "argument-hint: \""} {
		if !strings.Contains(hull, needle) {
			t.Errorf("Hülle enthält %q nicht", needle)
		}
	}
	text := readRepoDefault(t, "commands/k-feedback.md")
	if !strings.Contains(text, "## Schritt 1 — Briefing und Zugriffsweg\n\ninclude: _shared/erster-schritt.md\n") {
		t.Error("Text beginnt nicht mit dem ersten Schritt")
	}
	for _, needle := range []string{"`k_playbook_feedback_list`", "`allCollections: true`", "`k-playbook feedback",
		"`k_playbook_feedback_create`", "`k_playbook_feedback_update`", "## Anti-Muster", "Dokumente unter `feedback/` selbst lesen oder schreiben",
		"`done <nr>`", "`ablehnen <nr>`", "`annehmen <nr>`"} {
		if !strings.Contains(text, needle) {
			t.Errorf("k-feedback.md enthält %q nicht", needle)
		}
	}
	if mcp, sub := strings.Index(text, "1. **MCP-Werkzeuge.**"), strings.Index(text, "2. **Subkommando.**"); mcp < 0 || sub < mcp {
		t.Error("Zugriffsweg nicht MCP vor Subkommando")
	}
	betrieb := readRepoDefault(t, "guidelines/betrieb.md")
	for _, needle := range []string{"`keph:/feedback/<NNN>-<slug>.md`", "`keph:/feedback/done/`",
		"**Meldungen gehen ausschließlich über die Feedback-Werkzeuge, nie von Hand.**"} {
		if !strings.Contains(betrieb, needle) {
			t.Errorf("betrieb.md enthält %q nicht", needle)
		}
	}
}
