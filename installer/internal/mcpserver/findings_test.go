package mcpserver

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// Das Werkzeug ist die Hülle über project.CheckFindings: das Ergebnis steht
// unter check, im üblichen Umschlag; ein „nicht bereit“ ist kein
// Werkzeugfehler.
func TestFindingsCheckWerkzeug(t *testing.T) {
	root := newDocProject(t)
	var gotDir string
	original := checkFindings
	checkFindings = func(_ context.Context, projectDir string) project.FindingsCheck {
		gotDir = projectDir
		return project.FindingsCheck{Ready: false, Total: 3, ByUser: map[string]int{"kamran": 2, "anna": 1},
			Problems: []project.FindingsProblem{{Kind: project.FindingsProblemRight, Name: "supersede", Message: "fehlt"}},
			Rights:   []string{"read", "write"}, Skipped: []project.FindingsSkipped{}, Findings: []project.FindingItem{}}
	}
	t.Cleanup(func() { checkFindings = original })

	result, _, err := findingsCheckHandler(context.Background(), nil, findingsCheckInput{ProjectDir: root})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError || gotDir != root {
		t.Fatalf("Fehler oder falsches Projekt: %v %s", result.IsError, gotDir)
	}
	var envelope findingsCheckEnvelope
	if err := json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &envelope); err != nil {
		t.Fatal(err)
	}
	if !envelope.OK || envelope.Tool != findingsCheckTool || envelope.Check == nil || envelope.Check.Ready ||
		envelope.Check.ByUser["anna"] != 1 || envelope.Check.Problems[0].Name != "supersede" {
		t.Fatalf("Umschlag: %+v", envelope)
	}
}

func TestFindingsCheckWerkzeugOhneProjekt(t *testing.T) {
	result, _, err := findingsCheckHandler(context.Background(), nil, findingsCheckInput{ProjectDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	var envelope findingsCheckEnvelope
	if err := json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &envelope); err != nil {
		t.Fatal(err)
	}
	if !result.IsError || envelope.OK || envelope.Error == nil || envelope.Error.Code != "project_not_found" {
		t.Fatalf("ohne Projekt: %+v", envelope)
	}
}
