package mcpserver

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kascada/k-playbook/installer/internal/project"
)

func callBriefing(t *testing.T, projectDir string) (briefingEnvelope, *mcp.CallToolResult) {
	t.Helper()

	result, _, err := briefingToolHandler(context.Background(), nil, briefingInput{ProjectDir: projectDir})
	if err != nil {
		t.Fatalf("briefing: %v", err)
	}
	if len(result.Content) != 1 {
		t.Fatalf("erwartet genau einen Inhaltsblock, bekommen %d", len(result.Content))
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("Inhalt ist kein Text: %#v", result.Content[0])
	}
	envelope := briefingEnvelope{}
	if err := json.Unmarshal([]byte(text.Text), &envelope); err != nil {
		t.Fatalf("Antwort ist kein JSON: %v — %s", err, text.Text)
	}
	return envelope, result
}

// Das Werkzeug ist die Hülle über project.BuildBriefing: dieselben Felder wie
// auf der Kommandozeile, im üblichen Umschlag. Ohne Anbindung in der
// Konfiguration ist die Collection nicht nutzbar — ein Zustand in work, kein
// Fehler.
func TestBriefingWerkzeugListetDokumente(t *testing.T) {
	root := newDocProject(t)

	envelope, result := callBriefing(t, root)
	if result.IsError || !envelope.OK || envelope.Tool != briefingTool || envelope.ProjectDir != root || envelope.Briefing == nil {
		t.Fatalf("Umschlag: %#v", envelope)
	}
	if envelope.SchemaVersion != project.BriefingSchemaVersion {
		t.Errorf("schemaVersion %q", envelope.SchemaVersion)
	}
	always := envelope.Documents.Always
	if always.Ref != "k-playbook:/instructions/always/" || len(always.Parts) != 4 {
		t.Fatalf("always: %+v", always)
	}
	if always.Parts[1].Ref != "k-playbook:/instructions/always/100-b.md" || always.Parts[1].Origin != project.DocOriginOverride || always.Parts[1].Title != "B lokal" {
		t.Errorf("ersetzter Teil: %+v", always.Parts[1])
	}
	if always.Parts[2].Origin != project.BriefingOriginDisabled {
		t.Errorf("abgeschalteter Teil: %+v", always.Parts[2])
	}
	if len(envelope.Documents.OnDemand) != 2 || envelope.Documents.OnDemand[0].Origin != project.BriefingOriginDisabled ||
		envelope.Documents.OnDemand[1].Title != "Thema" || envelope.Documents.OnDemand[1].Description != "D" {
		t.Errorf("onDemand: %+v", envelope.Documents.OnDemand)
	}
	for name, list := range map[string][]project.BriefingEntry{
		"rules": envelope.Documents.Rules, "reviews": envelope.Documents.Reviews, "guidelines": envelope.Documents.Guidelines,
	} {
		if list == nil || len(list) != 0 {
			t.Errorf("%s: %v, erwartet leere Liste", name, list)
		}
	}
	if envelope.Work.Available || envelope.Work.Message == "" || envelope.Work.Tasks == nil || envelope.Work.Workbench == nil {
		t.Errorf("work: %+v", envelope.Work)
	}
	if envelope.ProjectDocs != "project:/k-playbook-local/docs/README.md" {
		t.Errorf("projectDocs %q", envelope.ProjectDocs)
	}
	// Der Rest von k_playbook_context steht im Abschnitt context, ohne die
	// Felder, die documents schon liefert.
	section := envelope.Context
	if !section.Available || section.Message != "" {
		t.Fatalf("context: %+v", section)
	}
	if raw, ok := section.Field("project"); !ok || !strings.Contains(string(raw), root) {
		t.Errorf("context.project: %s", raw)
	}
	if _, ok := section.Field("guidelines"); ok {
		t.Errorf("context trägt guidelines")
	}
	if raw, ok := section.Field("catalogs"); !ok || strings.Contains(string(raw), `"rules"`) || !strings.Contains(string(raw), `"checks"`) {
		t.Errorf("context.catalogs: %s", raw)
	}
}

func TestBriefingWerkzeugFehlercodes(t *testing.T) {
	envelope, result := callBriefing(t, "")
	if !result.IsError || envelope.OK || envelope.Error == nil || envelope.Error.Code != "invalid_input" || envelope.Briefing != nil {
		t.Errorf("ohne projectDir: %#v", envelope)
	}
	envelope, result = callBriefing(t, t.TempDir())
	if !result.IsError || envelope.OK || envelope.Error == nil || envelope.Error.Code != "project_not_found" {
		t.Errorf("ohne Projekt: %#v", envelope)
	}
}
