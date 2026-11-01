package project

import (
	"context"
	"strings"
	"testing"
)

const sliceDoc = `---
title: T
# kein Abschnitt
---
# Titel

Einleitung

## Intent

Absicht

### Unterpunkt

Detail

` + "```" + `
## Im Code-Block
` + "```" + `

## Folge

Rest
`

// sliceLayers sind Ebenen mit sliceDoc im Projekt und in der Collection.
func sliceLayers(t *testing.T) (DocLayers, *fakeDocStore) {
	t.Helper()
	layers, _, _ := docTree(t, map[string]string{"rules/a.md": sliceDoc}, nil)
	root := t.TempDir()
	writeDocTree(t, root, map[string]string{"docs/a.md": sliceDoc})
	layers.Project = NewDiskDocSource(root)
	store := newFakeDocStore("kamran")
	store.put("user/kamran/tasks/100-a.md", sliceDoc)
	layers.Keph = func() (KephDocs, error) { return KephDocs{Store: store, User: "kamran"}, nil }
	return layers, store
}

func readWith(t *testing.T, layers DocLayers, ref string, opts DocReadOptions) (DocResult, error) {
	t.Helper()
	return ResolveDocRefWith(context.Background(), layers, ref, opts)
}

func TestDocReadZeilenbereich(t *testing.T) {
	layers, _ := sliceLayers(t)
	total := strings.Count(sliceDoc, "\n")

	result, err := readWith(t, layers, "project:/docs/a.md", DocReadOptions{Lines: "5-7"})
	if err != nil || result.Content != "# Titel\n\nEinleitung\n" || result.TotalLines != total {
		t.Fatalf("5-7: %q %d %v", result.Content, result.TotalLines, err)
	}
	result, err = readWith(t, layers, "k-playbook:/rules/a.md", DocReadOptions{Lines: "21-"})
	if err != nil || result.Content != "## Folge\n\nRest\n" {
		t.Fatalf("21-: %q %v", result.Content, err)
	}
	result, err = readWith(t, layers, "project:/docs/a.md", DocReadOptions{Lines: "23-100"})
	if err != nil || result.Content != "Rest\n" {
		t.Fatalf("über das Ende: %q %v", result.Content, err)
	}
	result, err = readWith(t, layers, "project:/docs/a.md", DocReadOptions{Lines: "100-"})
	if err != nil || result.Content != "" || result.TotalLines != total {
		t.Fatalf("hinter dem Ende: %q %v", result.Content, err)
	}
	// Ohne Option: der ganze Inhalt, mit totalLines.
	result, err = readWith(t, layers, "project:/docs/a.md", DocReadOptions{})
	if err != nil || result.Content != sliceDoc || result.TotalLines != total {
		t.Fatalf("ganz: %d %v", result.TotalLines, err)
	}

	for _, lines := range []string{"5", "0-3", "a-b", "7-5", "-3", "3-x"} {
		_, err := readWith(t, layers, "project:/docs/a.md", DocReadOptions{Lines: lines})
		wantCode(t, err, DocCodeInvalidInput)
	}
	_, err = readWith(t, layers, "project:/docs/a.md", DocReadOptions{Lines: "1-2", Section: "## Intent"})
	wantCode(t, err, DocCodeInvalidInput)
	_, err = readWith(t, layers, "k-playbook:/instructions/always/", DocReadOptions{Lines: "1-2"})
	wantCode(t, err, DocCodeInvalidInput)
}

func TestDocReadAbschnitt(t *testing.T) {
	layers, _ := sliceLayers(t)

	// Mit Unterüberschrift, bis vor die Folgeüberschrift gleicher Ebene; die
	// Überschrift im Code-Block beendet nichts.
	result, err := readWith(t, layers, "project:/docs/a.md", DocReadOptions{Section: "## Intent"})
	want := "## Intent\n\nAbsicht\n\n### Unterpunkt\n\nDetail\n\n```\n## Im Code-Block\n```\n\n"
	if err != nil || result.Content != want {
		t.Fatalf("Intent: %q %v", result.Content, err)
	}
	result, err = readWith(t, layers, "project:/docs/a.md", DocReadOptions{Section: "### Unterpunkt"})
	if err != nil || !strings.HasPrefix(result.Content, "### Unterpunkt\n") || !strings.HasSuffix(result.Content, "```\n\n") {
		t.Fatalf("Unterpunkt: %q %v", result.Content, err)
	}
	// Höhere Ebene: bis zum Ende.
	result, err = readWith(t, layers, "project:/docs/a.md", DocReadOptions{Section: "# Titel"})
	if err != nil || !strings.HasSuffix(result.Content, "Rest\n") || strings.Contains(result.Content, "title: T") {
		t.Fatalf("Titel: %q %v", result.Content, err)
	}
	// Im Code-Block und im Frontmatter: kein Abschnitt.
	_, err = readWith(t, layers, "project:/docs/a.md", DocReadOptions{Section: "## Im Code-Block"})
	wantCode(t, err, DocCodeSectionNotFound)
	_, err = readWith(t, layers, "project:/docs/a.md", DocReadOptions{Section: "# kein Abschnitt"})
	wantCode(t, err, DocCodeSectionNotFound)
	_, err = readWith(t, layers, "project:/docs/a.md", DocReadOptions{Section: "## Fehlt"})
	wantCode(t, err, DocCodeSectionNotFound)
	_, err = readWith(t, layers, "project:/docs/a.md", DocReadOptions{Section: "Intent"})
	wantCode(t, err, DocCodeInvalidInput)
}

func TestDocReadAbschnittNachIncludes(t *testing.T) {
	layers, _, _ := docTree(t, map[string]string{
		"rules/a.md":         "# A\n\ninclude: teil.md\n\n## Danach\n",
		"rules/teil.md":      "## Teil\n\nEingesetzt\n",
		"rules/kein-teil.md": "x\n",
	}, nil)
	result, err := readWith(t, layers, "k-playbook:/rules/a.md", DocReadOptions{Section: "## Teil"})
	if err != nil || result.Content != "## Teil\n\nEingesetzt\n\n" {
		t.Fatalf("%q %v", result.Content, err)
	}
}

func TestDocReadKephRevision(t *testing.T) {
	layers, store := sliceLayers(t)
	result, err := readWith(t, layers, "keph:/user/~/tasks/100-a.md", DocReadOptions{Section: "## Folge"})
	want := store.docs["user/kamran/tasks/100-a.md"].revision
	if err != nil || result.Revision != want || want == 0 || result.Content != "## Folge\n\nRest\n" {
		t.Fatalf("%+v %v", result, err)
	}
	// Ohne keph: keine Revision.
	result, _ = readWith(t, layers, "project:/docs/a.md", DocReadOptions{})
	if result.Revision != 0 {
		t.Fatalf("project: mit Revision %d", result.Revision)
	}
}

func TestDocGrepEinDokument(t *testing.T) {
	layers, store := sliceLayers(t)
	ctx := context.Background()

	result, err := GrepDocRef(ctx, layers, "keph:/user/~/tasks/100-a.md", DocGrepOptions{Pattern: `^##+ `})
	if err != nil || result.Count != 4 || len(result.Matches) != 4 || result.Ref != "keph:/user/kamran/tasks/100-a.md" ||
		result.Revision != store.docs["user/kamran/tasks/100-a.md"].revision {
		t.Fatalf("%+v %v", result, err)
	}
	if m := result.Matches[0]; m.Line != 9 || m.Text != "## Intent" || m.Ref != result.Ref {
		t.Fatalf("Treffer: %+v", m)
	}

	result, err = GrepDocRef(ctx, layers, "keph:/user/~/tasks/100-a.md", DocGrepOptions{Pattern: `nirgends`, CountOnly: true})
	if err != nil || result.Count != 0 || len(result.Counts) != 1 || result.Counts[0].Revision == 0 || result.Matches != nil {
		t.Fatalf("count_only: %+v %v", result, err)
	}

	result, err = GrepDocRef(ctx, layers, "project:/docs/a.md", DocGrepOptions{Pattern: "Rest"})
	if err != nil || result.Count != 1 || result.Revision != 0 || result.Matches[0].Line != 23 {
		t.Fatalf("project: %+v %v", result, err)
	}

	_, err = GrepDocRef(ctx, layers, "project:/docs/a.md", DocGrepOptions{Pattern: "("})
	wantCode(t, err, DocCodeInvalidInput)
	_, err = GrepDocRef(ctx, layers, "project:/docs/a.md", DocGrepOptions{})
	wantCode(t, err, DocCodeInvalidInput)
	_, err = GrepDocRef(ctx, layers, "k-playbook:/instructions/always/", DocGrepOptions{Pattern: "a"})
	wantCode(t, err, DocCodeInvalidInput)
}

func TestDocGrepSorte(t *testing.T) {
	store := newFakeDocStore("kamran")
	store.put("user/kamran/tasks/100-a.md", "status: offen\nx\n")
	store.put("user/kamran/tasks/done/101-b.md", "status: erledigt\n")
	store.put("user/anna/tasks/102-c.md", "status: offen\n")
	docs := newTestUserDocs(t, store, DocKinds)
	ctx := context.Background()

	result, err := docs.Grep(ctx, DocListOptions{Kind: "tasks"}, DocGrepOptions{Pattern: "^status:"})
	if err != nil || result.Count != 1 || result.Kind != "tasks" || result.User != "kamran" || result.Matches[0].Ref != "keph:/user/kamran/tasks/100-a.md" {
		t.Fatalf("eigene: %+v %v", result, err)
	}
	result, err = docs.Grep(ctx, DocListOptions{Kind: "tasks", Done: true}, DocGrepOptions{Pattern: "^status:"})
	if err != nil || result.Count != 2 || result.Matches[1].Ref != "keph:/user/kamran/tasks/done/101-b.md" || !result.Done {
		t.Fatalf("mit done: %+v %v", result, err)
	}
	result, err = docs.Grep(ctx, DocListOptions{Kind: "tasks", User: "*", Done: true}, DocGrepOptions{Pattern: "offen", CountOnly: true})
	if err != nil || result.Count != 2 || len(result.Counts) != 3 || result.Matches != nil {
		t.Fatalf("alle, count_only: %+v %v", result, err)
	}
	for _, c := range result.Counts {
		if c.Revision == 0 {
			t.Errorf("ohne Revision: %+v", c)
		}
	}
	_, err = docs.Grep(ctx, DocListOptions{Kind: "todos"}, DocGrepOptions{Pattern: "a"})
	wantCode(t, err, DocCodeInvalidInput)
}

func TestDocGrepHoechstens200(t *testing.T) {
	store := newFakeDocStore("kamran")
	store.put("queue/findings/001-a.md", strings.Repeat("treffer\n", 150))
	store.put("queue/findings/002-b.md", strings.Repeat("treffer\n", 150))
	docs := newTestUserDocs(t, store, DocKinds)
	result, err := docs.Grep(context.Background(), DocListOptions{Kind: "findings"}, DocGrepOptions{Pattern: "treffer"})
	if err != nil || result.Count != 300 || len(result.Matches) != 200 || !result.Truncated {
		t.Fatalf("Count %d, Matches %d, truncated %v, %v", result.Count, len(result.Matches), result.Truncated, err)
	}
}
