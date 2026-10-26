package mcpserver

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kascada/k-playbook/installer/internal/githubaccess/accesstest"
	"github.com/kascada/k-playbook/installer/internal/health"
	"github.com/kascada/k-playbook/installer/internal/project"
)

func callHealth(t *testing.T, projectDir string) (healthEnvelope, map[string]any, *mcp.CallToolResult) {
	t.Helper()

	result, _, err := healthToolHandler(context.Background(), nil, healthInput{ProjectDir: projectDir})
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	if len(result.Content) != 1 {
		t.Fatalf("erwartet genau einen Inhaltsblock, bekommen %d", len(result.Content))
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("Inhalt ist kein Text: %#v", result.Content[0])
	}
	envelope := healthEnvelope{}
	if err := json.Unmarshal([]byte(text.Text), &envelope); err != nil {
		t.Fatalf("Antwort ist kein JSON: %v — %s", err, text.Text)
	}
	fields := map[string]any{}
	if err := json.Unmarshal([]byte(text.Text), &fields); err != nil {
		t.Fatal(err)
	}
	return envelope, fields, result
}

// Das Werkzeug ist die Hülle über health.Check: der Report steht unter
// health, im üblichen Umschlag. Ohne Anbindung ist genau eine Warnung da; sie
// lässt health.ok wahr.
func TestHealthWerkzeugLiefertDenReport(t *testing.T) {
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	accesstest.Empty(t)
	root := newDocProject(t)

	envelope, fields, result := callHealth(t, root)
	if result.IsError || !envelope.OK || envelope.Tool != healthTool || envelope.ProjectDir != root || envelope.Health == nil {
		t.Fatalf("Umschlag: %#v", envelope)
	}
	report := *envelope.Health
	if !report.OK || len(report.Problems) != 1 || report.Problems[0].Area != health.AreaKephalaion || report.Problems[0].Level != health.LevelWarning {
		t.Errorf("Report: %+v", report)
	}
	if _, embedded := fields["problems"]; embedded {
		t.Errorf("Report liegt eingebettet statt unter health: %v", fields)
	}
	if _, present := fields["error"]; present {
		t.Errorf("error steht in einer guten Antwort: %v", fields)
	}

	// Ein Fehler macht health.ok falsch; der Umschlag bleibt ok.
	path := filepath.Join(project.LocalDir(root), project.VersionSourcesFileName)
	if err := os.WriteFile(path, []byte(": ungültig\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	envelope, _, result = callHealth(t, root)
	if result.IsError || !envelope.OK || envelope.Health == nil || envelope.Health.OK || len(envelope.Health.Problems) != 2 {
		t.Errorf("mit Fehler: %#v", envelope)
	}
}

func TestHealthWerkzeugOhneProjektIstEingabefehler(t *testing.T) {
	envelope, _, result := callHealth(t, t.TempDir())
	if !result.IsError || envelope.OK || envelope.Health != nil || envelope.Error == nil || envelope.Error.Code != "project_not_found" {
		t.Errorf("Umschlag: %#v", envelope)
	}
	envelope, _, result = callHealth(t, "")
	if !result.IsError || envelope.Error == nil || envelope.Error.Code != "invalid_input" {
		t.Errorf("ohne projectDir: %#v", envelope)
	}
}
