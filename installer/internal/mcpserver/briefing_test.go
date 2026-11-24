package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
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
	if !strings.HasPrefix(always.Content, "## A\n\n## B lokal\n\nText.\n") || strings.Contains(always.Content, "## C") {
		t.Errorf("always.content: %q", always.Content)
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
		"reviews": envelope.Documents.Reviews, "guidelines": envelope.Documents.Guidelines,
	} {
		if list == nil || len(list) != 0 {
			t.Errorf("%s: %v, erwartet leere Liste", name, list)
		}
	}
	if envelope.Work.Available || envelope.Work.Message == "" || envelope.Work.Tasks == nil || envelope.Work.Workbench == nil {
		t.Errorf("work: %+v", envelope.Work)
	}
	if envelope.ProjectDocs != "keph:/docs/" {
		t.Errorf("projectDocs %q", envelope.ProjectDocs)
	}
	// Die Abschnitte, die jede Sitzung braucht, stehen im Briefing selbst.
	if !envelope.Project.Available || envelope.Project.Dir != root || envelope.Now.Date == "" || envelope.Kephalaion.Message == "" ||
		envelope.GitHub.ProjectServices == nil {
		t.Errorf("project %+v, now %+v, kephalaion %+v, github %+v", envelope.Project, envelope.Now, envelope.Kephalaion, envelope.GitHub)
	}
	// Den Übergangsabschnitt context gibt es nicht mehr (Task 187), und ohne
	// part enthält die Antwort den Teilabruf nicht.
	text := result.Content[0].(*mcp.TextContent).Text
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(text), &raw); err != nil {
		t.Fatal(err)
	}
	for _, absent := range []string{"context", "environment", "catalogs", "remediation", "baseTools"} {
		if _, ok := raw[absent]; ok {
			t.Errorf("Antwort trägt %s: %s", absent, raw[absent])
		}
	}
}

// Der Teilabruf (Task 187): Mit part environment trägt die Antwort nur den
// Umschlag und environment — weder session noch documents noch work — und
// vergibt keine Sitzungskennung. Ein unbekannter Teil ist invalid_input.
func TestBriefingWerkzeugTeilabrufEnvironment(t *testing.T) {
	root := newDocProject(t)

	result, _, err := briefingToolHandler(context.Background(), nil, briefingInput{ProjectDir: root, Part: project.BriefingPartEnvironment})
	if err != nil {
		t.Fatalf("briefing: %v", err)
	}
	text := result.Content[0].(*mcp.TextContent).Text
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(text), &raw); err != nil {
		t.Fatalf("Antwort ist kein JSON: %v — %s", err, text)
	}
	if result.IsError || string(raw["ok"]) != "true" || raw["environment"] == nil {
		t.Fatalf("Antwort: %s", text)
	}
	for _, absent := range []string{"session", "documents", "work", "schemaVersion", "health", "now", "project", "kephalaion", "links", "projectDocs"} {
		if _, ok := raw[absent]; ok {
			t.Errorf("Teilabruf trägt %s: %s", absent, raw[absent])
		}
	}
	var envelope briefingEnvelope
	if err := json.Unmarshal([]byte(text), &envelope); err != nil {
		t.Fatal(err)
	}
	environment := envelope.Environment
	if environment == nil || !environment.Available || environment.GH == nil || environment.GitHub == nil || environment.BaseTools == nil ||
		environment.VersionSources == nil || environment.Docs == nil || environment.Remediation == nil {
		t.Errorf("environment: %+v", environment)
	}

	result, _, err = briefingToolHandler(context.Background(), nil, briefingInput{ProjectDir: root, Part: "work"})
	if err != nil {
		t.Fatalf("briefing: %v", err)
	}
	envelope = briefingEnvelope{}
	if err := json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &envelope); err != nil {
		t.Fatal(err)
	}
	if !result.IsError || envelope.OK || envelope.Error == nil || envelope.Error.Code != "invalid_input" || envelope.Briefing != nil || envelope.Environment != nil {
		t.Errorf("unbekannter Teil: %#v", envelope)
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

// Ohne mitgelieferte Ebene ist das Briefing ein Werkzeugfehler mit dem Code
// kephalaion_unavailable, nicht read_failed, und ohne halbes Briefing.
func TestBriefingWerkzeugOhneMitgelieferteEbene(t *testing.T) {
	root := newDocProject(t)
	original := project.OpenDefaultsStore
	project.OpenDefaultsStore = func(string) (project.DefaultsStore, string, error) {
		return nil, "vm:k-playbook", errors.New("Kephalaion ist für dieses Projekt nicht bereit")
	}
	t.Cleanup(func() { project.OpenDefaultsStore = original })

	envelope, result := callBriefing(t, root)
	if !result.IsError || envelope.OK || envelope.Error == nil || envelope.Error.Code != project.DocCodeKephalaionUnavailable {
		t.Fatalf("Umschlag: %#v", envelope)
	}
	if envelope.Briefing != nil {
		t.Errorf("halbes Briefing in der Fehlerantwort: %#v", envelope.Briefing)
	}
	if !strings.Contains(envelope.Error.Message, "/k-gui") {
		t.Errorf("Meldung ohne Abhilfe: %q", envelope.Error.Message)
	}
}
