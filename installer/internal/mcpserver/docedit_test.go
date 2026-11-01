package mcpserver

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kascada/k-playbook/installer/internal/project"
)

func slicesContain(list []string, value string) bool { return slices.Contains(list, value) }

// docEditAnswer ist die Antwort eines Änderungswerkzeugs, zerlegt.
type docEditAnswer struct {
	OK            bool                    `json:"ok"`
	Tool          string                  `json:"tool"`
	Ref           string                  `json:"ref"`
	Revision      int64                   `json:"revision"`
	Size          int64                   `json:"size"`
	Changed       *bool                   `json:"changed"`
	SinceRevision int64                   `json:"sinceRevision"`
	UpdatedBy     string                  `json:"updatedBy"`
	UpdatedAt     string                  `json:"updatedAt"`
	Tail          *string                 `json:"tail"`
	Error         *knowledgeErrorEnvelope `json:"error"`
}

func TestDocAppendReplaceSetUeberHuellen(t *testing.T) {
	root := newDocProject(t)
	store := useMemDocs(t, "user/kamran/tasks/100-a.md", "user/anna/tasks/101-b.md")
	store.content["user/kamran/tasks/100-a.md"] = "# A\n"
	ctx := context.Background()
	ref := "keph:/user/~/tasks/100-a.md"

	result, _, _ := docAppendTool(ctx, nil, docAppendInput{ProjectDir: root, Ref: ref, Content: "Eintrag mit `x` und $(y)"})
	var answer docEditAnswer
	decodeEnvelope(t, result, &answer)
	if result.IsError || !answer.OK || answer.Tool != docToolAppend || answer.Ref != "keph:/user/kamran/tasks/100-a.md" ||
		answer.Changed == nil || !*answer.Changed || answer.Revision == 0 {
		t.Fatalf("append: %+v", answer)
	}
	if got := store.content["user/kamran/tasks/100-a.md"]; got != "# A\n\nEintrag mit `x` und $(y)\n" {
		t.Fatalf("Inhalt: %q", got)
	}

	// Hintereinander mit since_revision aus der Antwort.
	result, _, _ = docReplaceTool(ctx, nil, docReplaceInput{ProjectDir: root, Ref: ref, Old: "# A", New: "# B", SinceRevision: answer.Revision})
	decodeEnvelope(t, result, &answer)
	if result.IsError || answer.Tool != docToolReplace || !*answer.Changed {
		t.Fatalf("replace: %+v", answer)
	}
	value := "offen"
	result, _, _ = docSetFieldTool(ctx, nil, docSetFieldInput{ProjectDir: root, Ref: ref, Key: "status", Value: &value, SinceRevision: answer.Revision})
	decodeEnvelope(t, result, &answer)
	if result.IsError || answer.Tool != docToolSetField || !strings.HasPrefix(store.content["user/kamran/tasks/100-a.md"], "---\nstatus: offen\n---\n\n# B\n") {
		t.Fatalf("set: %+v %q", answer, store.content["user/kamran/tasks/100-a.md"])
	}
	result, _, _ = docSetFieldTool(ctx, nil, docSetFieldInput{ProjectDir: root, Ref: ref, Key: "status", Unset: true})
	decodeEnvelope(t, result, &answer)
	if result.IsError || !*answer.Changed || strings.Contains(store.content["user/kamran/tasks/100-a.md"], "status") {
		t.Fatalf("unset: %+v", answer)
	}

	// Veraltete Revision: foreign_change mit Stand.
	result, _, _ = docAppendTool(ctx, nil, docAppendInput{ProjectDir: root, Ref: ref, Content: "x", SinceRevision: 1})
	answer = docEditAnswer{}
	decodeEnvelope(t, result, &answer)
	if !result.IsError || answer.OK || answer.Error == nil || answer.Error.Code != project.DocCodeForeignChange ||
		answer.Revision == 0 || answer.SinceRevision != 1 || answer.UpdatedBy == "" || answer.UpdatedAt == "" ||
		answer.Tail == nil || !strings.Contains(*answer.Tail, "# B") {
		t.Fatalf("foreign_change: %+v", answer)
	}
}

func TestDocEditHuellenFehlercodes(t *testing.T) {
	root := newDocProject(t)
	store := useMemDocs(t, "user/kamran/tasks/100-a.md", "user/anna/tasks/101-b.md")
	store.content["user/kamran/tasks/100-a.md"] = "a a\n"
	ctx := context.Background()
	value := "x"
	cases := []struct {
		name   string
		result func() *mcp.CallToolResult
		code   string
	}{
		{"fremder User", func() *mcp.CallToolResult {
			r, _, _ := docAppendTool(ctx, nil, docAppendInput{ProjectDir: root, Ref: "keph:/user/anna/tasks/101-b.md", Content: "x"})
			return r
		}, project.DocCodeForeignUser},
		{"außerhalb der Sorten", func() *mcp.CallToolResult {
			r, _, _ := docAppendTool(ctx, nil, docAppendInput{ProjectDir: root, Ref: "keph:/todos/001-a.md", Content: "x"})
			return r
		}, project.DocCodeInvalidInput},
		{"leerer Text", func() *mcp.CallToolResult {
			r, _, _ := docAppendTool(ctx, nil, docAppendInput{ProjectDir: root, Ref: "keph:/user/~/tasks/100-a.md"})
			return r
		}, project.DocCodeInvalidInput},
		{"nicht eindeutig", func() *mcp.CallToolResult {
			r, _, _ := docReplaceTool(ctx, nil, docReplaceInput{ProjectDir: root, Ref: "keph:/user/~/tasks/100-a.md", Old: "a", New: "b"})
			return r
		}, project.DocCodeTextNotUnique},
		{"nicht gefunden", func() *mcp.CallToolResult {
			r, _, _ := docReplaceTool(ctx, nil, docReplaceInput{ProjectDir: root, Ref: "keph:/user/~/tasks/100-a.md", Old: "z", New: "b"})
			return r
		}, project.DocCodeTextNotFound},
		{"value und unset", func() *mcp.CallToolResult {
			r, _, _ := docSetFieldTool(ctx, nil, docSetFieldInput{ProjectDir: root, Ref: "keph:/user/~/tasks/100-a.md", Key: "a", Value: &value, Unset: true})
			return r
		}, project.DocCodeInvalidInput},
		{"weder value noch unset", func() *mcp.CallToolResult {
			r, _, _ := docSetFieldTool(ctx, nil, docSetFieldInput{ProjectDir: root, Ref: "keph:/user/~/tasks/100-a.md", Key: "a"})
			return r
		}, project.DocCodeInvalidInput},
		{"fehlt", func() *mcp.CallToolResult {
			r, _, _ := docAppendTool(ctx, nil, docAppendInput{ProjectDir: root, Ref: "keph:/user/~/tasks/999-x.md", Content: "x"})
			return r
		}, project.DocCodeNotFound},
		{"ohne projectDir", func() *mcp.CallToolResult {
			r, _, _ := docAppendTool(ctx, nil, docAppendInput{Ref: "keph:/user/~/tasks/100-a.md", Content: "x"})
			return r
		}, "invalid_input"},
	}
	for _, c := range cases {
		result := c.result()
		var answer docEditAnswer
		decodeEnvelope(t, result, &answer)
		if !result.IsError || answer.OK || answer.Error == nil || answer.Error.Code != c.code {
			t.Errorf("%s: erwartet %s, bekommen %+v", c.name, c.code, answer.Error)
		}
	}
	if store.content["user/kamran/tasks/100-a.md"] != "a a\n" {
		t.Fatalf("abgelehnt und doch geschrieben: %q", store.content["user/kamran/tasks/100-a.md"])
	}
}

func TestDocGrepUeberHuelle(t *testing.T) {
	root := newDocProject(t)
	store := useMemDocs(t, "user/kamran/tasks/100-a.md", "user/kamran/tasks/done/101-b.md")
	store.content["user/kamran/tasks/100-a.md"] = "## Intent\nstatus: offen\n"
	store.content["user/kamran/tasks/done/101-b.md"] = "status: erledigt\n"
	useMemLayers(t, store)
	ctx := context.Background()

	type grepAnswer struct {
		OK        bool                    `json:"ok"`
		Tool      string                  `json:"tool"`
		Ref       string                  `json:"ref"`
		Revision  int64                   `json:"revision"`
		Kind      string                  `json:"kind"`
		Matches   []project.DocGrepMatch  `json:"matches"`
		Counts    []project.DocGrepCount  `json:"counts"`
		Count     int                     `json:"count"`
		Truncated bool                    `json:"truncated"`
		Error     *knowledgeErrorEnvelope `json:"error"`
	}
	result, _, _ := docGrepTool(ctx, nil, docGrepInput{ProjectDir: root, Kind: "tasks", Done: true, Pattern: "^status:"})
	var answer grepAnswer
	decodeEnvelope(t, result, &answer)
	if result.IsError || answer.Tool != docToolGrep || answer.Kind != "tasks" || answer.Count != 2 || len(answer.Matches) != 2 {
		t.Fatalf("Sorte: %+v", answer)
	}

	result, _, _ = docGrepTool(ctx, nil, docGrepInput{ProjectDir: root, Ref: "keph:/user/~/tasks/100-a.md", Pattern: "x^", CountOnly: true})
	answer = grepAnswer{}
	decodeEnvelope(t, result, &answer)
	if result.IsError || answer.Revision == 0 || len(answer.Counts) != 1 || answer.Counts[0].Revision != answer.Revision || answer.Matches != nil {
		t.Fatalf("count_only: %+v", answer)
	}

	result, _, _ = docGrepTool(ctx, nil, docGrepInput{ProjectDir: root, Ref: "k-playbook:/instructions/on-demand/thema.md", Pattern: "Thema"})
	answer = grepAnswer{}
	decodeEnvelope(t, result, &answer)
	if result.IsError || answer.Count != 2 || answer.Revision != 0 {
		t.Fatalf("k-playbook: %+v", answer)
	}

	for name, input := range map[string]docGrepInput{
		"weder ref noch kind": {ProjectDir: root, Pattern: "a"},
		"beides":              {ProjectDir: root, Ref: "keph:/user/~/tasks/100-a.md", Kind: "tasks", Pattern: "a"},
		"user ohne kind":      {ProjectDir: root, Ref: "keph:/user/~/tasks/100-a.md", User: "*", Pattern: "a"},
		"kaputtes Muster":     {ProjectDir: root, Kind: "tasks", Pattern: "("},
	} {
		result, _, _ := docGrepTool(ctx, nil, input)
		answer = grepAnswer{}
		decodeEnvelope(t, result, &answer)
		if !result.IsError || answer.Error == nil || answer.Error.Code != project.DocCodeInvalidInput {
			t.Errorf("%s: %+v", name, answer.Error)
		}
	}
}

// useMemLayers lässt keph:-Verweise von doc_read und doc_grep über den Store
// im Speicher laufen.
func useMemLayers(t *testing.T, store *memDocStore) {
	t.Helper()
	original := projectDocLayers
	projectDocLayers = func(dir string) project.DocLayers {
		layers := project.ProjectDocLayers(dir)
		layers.Keph = func() (project.KephDocs, error) { return project.KephDocs{Store: store, User: "kamran"}, nil }
		return layers
	}
	t.Cleanup(func() { projectDocLayers = original })
}

func TestDocReadTeileUndRevisionUeberHuelle(t *testing.T) {
	root := newDocProject(t)
	store := useMemDocs(t, "user/kamran/tasks/100-a.md")
	store.content["user/kamran/tasks/100-a.md"] = "# A\n\n## Intent\n\nText\n\n## Folge\n"
	useMemLayers(t, store)
	ctx := context.Background()

	result, _, _ := docReadTool(ctx, nil, docReadInput{ProjectDir: root, Ref: "keph:/user/~/tasks/100-a.md", Section: "## Intent"})
	var envelope docEnvelope
	decodeEnvelope(t, result, &envelope)
	if result.IsError || deref(envelope.Content) != "## Intent\n\nText\n\n" || envelope.Revision == 0 ||
		envelope.TotalLines == nil || *envelope.TotalLines != 7 {
		t.Fatalf("section: %+v %q", envelope, deref(envelope.Content))
	}
	result, _, _ = docReadTool(ctx, nil, docReadInput{ProjectDir: root, Ref: "k-playbook:/instructions/on-demand/thema.md", Lines: "5-"})
	envelope = docEnvelope{}
	decodeEnvelope(t, result, &envelope)
	if result.IsError || deref(envelope.Content) != "\n## Thema\n" || envelope.Revision != 0 {
		t.Fatalf("lines: %+v %q", envelope, deref(envelope.Content))
	}
	result, _, _ = docReadTool(ctx, nil, docReadInput{ProjectDir: root, Ref: "keph:/user/~/tasks/100-a.md", Section: "## Fehlt"})
	envelope = docEnvelope{}
	decodeEnvelope(t, result, &envelope)
	if !result.IsError || envelope.Error == nil || envelope.Error.Code != project.DocCodeSectionNotFound {
		t.Fatalf("section_not_found: %+v", envelope.Error)
	}
}

func TestDocDoneUndReopenMitRevision(t *testing.T) {
	root := newDocProject(t)
	useMemDocs(t, "user/kamran/tasks/100-a.md")
	ctx := context.Background()
	var moved struct {
		Ref      string `json:"ref"`
		Revision int64  `json:"revision"`
	}
	result, _, _ := docDoneTool(ctx, nil, docMoveInput{ProjectDir: root, Ref: "keph:/user/~/tasks/100-a.md"})
	decodeEnvelope(t, result, &moved)
	first := moved.Revision
	if result.IsError || first == 0 {
		t.Fatalf("done: %+v", moved)
	}
	result, _, _ = docReopenTool(ctx, nil, docMoveInput{ProjectDir: root, Ref: moved.Ref})
	decodeEnvelope(t, result, &moved)
	if result.IsError || moved.Revision <= first {
		t.Fatalf("reopen: %+v", moved)
	}
}
