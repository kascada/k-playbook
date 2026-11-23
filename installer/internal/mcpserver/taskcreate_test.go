package mcpserver

import (
	"context"
	"strings"
	"testing"
)

const mcpTaskBody = "Ein Satz.\n\n## Intent\n\nRahmen.\n- Bedingung\n\n## Ziel\n\nTun.\n"

type taskCreateAnswer struct {
	OK       bool     `json:"ok"`
	Tool     string   `json:"tool"`
	Ref      string   `json:"ref"`
	Number   int      `json:"number"`
	Name     string   `json:"name"`
	Revision int64    `json:"revision"`
	Warnings []string `json:"warnings"`
	DryRun   bool     `json:"dryRun"`
	Rendered string   `json:"rendered"`
	Error    *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func callTaskCreate(t *testing.T, input taskCreateInput) (taskCreateAnswer, bool) {
	t.Helper()
	result, _, err := taskCreateTool(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("task_create: %v", err)
	}
	var answer taskCreateAnswer
	decodeEnvelope(t, result, &answer)
	return answer, result.IsError
}

func TestTaskCreateUeberHuelle(t *testing.T) {
	root := newDocProject(t)
	store := useMemDocs(t, "user/anna/tasks/007-fremd.md")

	dry, isErr := callTaskCreate(t, taskCreateInput{ProjectDir: root, Title: "Neu: Ding", Name: "Neues Ding", Body: mcpTaskBody, DryRun: true})
	if isErr || !dry.OK || !dry.DryRun || dry.Ref != "" || !strings.Contains(dry.Rendered, "# Task <NNN> — Neu: Ding") || dry.Warnings == nil {
		t.Fatalf("Trockenlauf: %+v", dry)
	}
	if _, ok := store.docs["user/kamran/tasks/008-neues-ding.md"]; ok {
		t.Fatal("Trockenlauf hat angelegt")
	}

	created, isErr := callTaskCreate(t, taskCreateInput{ProjectDir: root, Title: "Neu: Ding", Name: "Neues Ding", Body: mcpTaskBody})
	if isErr || !created.OK || created.Tool != taskToolCreate || created.Number != 8 ||
		created.Ref != "keph:/user/kamran/tasks/008-neues-ding.md" || created.Revision == 0 {
		t.Fatalf("Antwort: %+v", created)
	}
	want := strings.Replace(dry.Rendered, "<NNN>", "008", 1)
	if got := store.content["user/kamran/tasks/008-neues-ding.md"]; got != want {
		t.Fatalf("Inhalt %q, erwartet %q", got, want)
	}
}

func TestTaskCreateVerstoesseUndFehler(t *testing.T) {
	root := newDocProject(t)
	store := useMemDocs(t)

	answer, isErr := callTaskCreate(t, taskCreateInput{ProjectDir: root, Title: "T", Name: "x", Body: "## Ziel\n\nTun.\n"})
	if !isErr || answer.OK || answer.Error == nil || answer.Error.Code != "invalid_input" ||
		!strings.Contains(answer.Error.Message, "Einleitungssatz") || !strings.Contains(answer.Error.Message, "## Intent") {
		t.Fatalf("Verstöße: %+v", answer)
	}
	if len(store.docs) != 0 {
		t.Fatalf("trotz Verstößen angelegt: %v", store.docs)
	}

	noIntent, isErr := callTaskCreate(t, taskCreateInput{ProjectDir: root, Title: "T", Name: "x", NoIntent: true,
		Body: "Ein Satz.\n\n## Ziel\n\nTun.\n"})
	if isErr || !noIntent.OK {
		t.Fatalf("noIntent: %+v", noIntent)
	}

	answer, isErr = callTaskCreate(t, taskCreateInput{Title: "T", Name: "x", Body: mcpTaskBody})
	if !isErr || answer.Error == nil || answer.Error.Code == "" {
		t.Fatalf("ohne projectDir: %+v", answer)
	}
}
