package mcpserver

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kascada/k-playbook/installer/internal/kephalaion/kephtest"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// newFeedbackProject legt ein Projekt an, dessen Meldungen in einer
// Collection im Speicher liegen.
func newFeedbackProject(t *testing.T) (string, *kephtest.Store) {
	t.Helper()
	root := t.TempDir()
	if err := project.CreateConfig(root, "."); err != nil {
		t.Fatalf("Konfiguration anlegen: %v", err)
	}
	store := kephtest.New()
	original, version := project.OpenFeedbackStore, project.FeedbackVersion
	project.OpenFeedbackStore = func(string) (project.FeedbackStore, project.FeedbackIdentity, error) {
		return store, project.FeedbackIdentity{User: "kamran", Account: "kamran-wsl", Address: "vm:test"}, nil
	}
	project.FeedbackVersion = func() string { return "v9.9.9" }
	t.Cleanup(func() { project.OpenFeedbackStore, project.FeedbackVersion = original, version })
	return root, store
}

// feedbackReply ist die Antwort der Feedback-Werkzeuge, wie ein Client sie
// liest.
type feedbackReply struct {
	OK            bool               `json:"ok"`
	Ref           string             `json:"ref"`
	Number        int                `json:"number"`
	Name          string             `json:"name"`
	Revision      int64              `json:"revision"`
	Items         []project.Feedback `json:"items"`
	Feedback      *project.Feedback  `json:"feedback"`
	From          string             `json:"from"`
	Changed       *bool              `json:"changed"`
	SinceRevision int64              `json:"sinceRevision"`
	Error         *errorEnvelope     `json:"error"`
}

func decodeFeedbackReply(t *testing.T, result *mcp.CallToolResult) (feedbackReply, string) {
	t.Helper()
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("Inhalt ist kein Text: %#v", result.Content[0])
	}
	var reply feedbackReply
	if err := json.Unmarshal([]byte(text.Text), &reply); err != nil {
		t.Fatalf("Antwort ist kein JSON: %v — %s", err, text.Text)
	}
	return reply, text.Text
}

// Über MCP meldet immer eine KI: von ki, client aus dem Parameter; ohne ihn
// und ohne clientInfo ist das ein Eingabefehler.
func TestFeedbackWerkzeugeAnlegenListenAendern(t *testing.T) {
	root, store := newFeedbackProject(t)
	ctx := context.Background()
	input := feedbackCreateInput{ProjectDir: root, Title: "Node hängt", Art: "fehler", Dringlichkeit: "hoch",
		Betrifft: "kephalaion", Body: "Text.", Sitzung: "s-1", Modell: "claude-opus-5-5"}

	result, _, _ := feedbackCreateTool(ctx, nil, input)
	if reply, text := decodeFeedbackReply(t, result); reply.OK || reply.Error.Code != "invalid_input" || !strings.Contains(text, "client") {
		t.Fatalf("ohne client: %s", text)
	}
	input.Client = "claude-code"
	result, _, _ = feedbackCreateTool(ctx, nil, input)
	created, text := decodeFeedbackReply(t, result)
	if !created.OK || created.Ref != "keph:/feedback/001-node-haengt.md" || created.Number != 1 || created.Name != "001-node-haengt.md" ||
		created.Revision == 0 || created.Feedback == nil || created.Feedback.Von != "ki" || created.Feedback.Client != "claude-code" {
		t.Fatalf("create: %s", text)
	}
	if content, _ := store.Content("feedback/001-node-haengt.md"); !strings.Contains(content, "von: ki\n") {
		t.Errorf("Inhalt: %s", content)
	}

	result, _, _ = feedbackListTool(ctx, nil, feedbackListInput{ProjectDir: root, Status: "offen"})
	list, text := decodeFeedbackReply(t, result)
	if !list.OK || len(list.Items) != 1 || list.Items[0].Revision != created.Revision {
		t.Fatalf("list: %s", text)
	}
	result, _, _ = feedbackListTool(ctx, nil, feedbackListInput{ProjectDir: root, Art: "wunsch"})
	if _, text := decodeFeedbackReply(t, result); !strings.Contains(text, `"items": []`) {
		t.Errorf("leere Liste fehlt als []: %s", text)
	}

	result, _, _ = feedbackUpdateTool(ctx, nil, feedbackUpdateInput{ProjectDir: root, Status: "erledigt"})
	if reply, _ := decodeFeedbackReply(t, result); reply.OK || reply.Error.Code != "invalid_input" {
		t.Errorf("weder ref noch number: %+v", reply)
	}
	result, _, _ = feedbackUpdateTool(ctx, nil, feedbackUpdateInput{ProjectDir: root, Number: 1, Status: "erledigt", SinceRevision: created.Revision + 5})
	if reply, text := decodeFeedbackReply(t, result); reply.OK || reply.Error.Code != "foreign_change" || reply.Revision != created.Revision ||
		reply.SinceRevision != created.Revision+5 {
		t.Errorf("foreign_change: %s", text)
	}
	result, _, _ = feedbackUpdateTool(ctx, nil, feedbackUpdateInput{ProjectDir: root, Ref: created.Ref, Status: "erledigt", SinceRevision: created.Revision})
	updated, text := decodeFeedbackReply(t, result)
	if !updated.OK || updated.Feedback.Ref != "keph:/feedback/done/001-node-haengt.md" || updated.From != created.Ref ||
		updated.Changed == nil || !*updated.Changed {
		t.Fatalf("update: %s", text)
	}
}
