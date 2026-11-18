package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultsKSessionContactIstHuelleUndText(t *testing.T) {
	hull, err := os.ReadFile(filepath.Join("..", "..", "..", "share", "commands", "k-session-contact.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, needle := range []string{"k-playbook:/commands/k-session-contact.md", "$ARGUMENTS", "mcp__k-playbook__k_playbook_doc_read"} {
		if !strings.Contains(string(hull), needle) {
			t.Errorf("Hülle enthält %q nicht", needle)
		}
	}
	if strings.Contains(string(hull), "/api/session/") {
		t.Error("API-Ablauf gehört in den Hub-Text, nicht in die Hülle")
	}
	for file, needles := range map[string][]string{
		"commands/k-session-contact.md": {"Session: <bestätigte eigene ses_… oder unbekannt>", "keine** OpenCode-Session-ID", "session-kommunikation.md", "Playwright", "keine Nachricht", "Claude"},
		"instructions/always/410-session-kommunikation.md": {"### OpenCode", "### Claude", "opencode api", "session-kommunikation.md", "/k-session-contact"},
		"instructions/on-demand/session-kommunikation.md": {"\"resume\":true", "/api/experimental/session/SESSION_ID/wait", "Auftrag-ID", "Berechtigungen", "experimentell", "keinen zweiten Auftrag blind senden"},
	} {
		text := readRepoDefault(t, file)
		for _, needle := range needles {
			if !strings.Contains(text, needle) {
				t.Errorf("%s enthält %q nicht", file, needle)
			}
		}
	}
}
