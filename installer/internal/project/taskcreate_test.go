package project

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
)

const taskBodyOK = `Ein Satz, was der Task erreicht.

## Intent

Rahmen.
- Bedingung

## Referenzen

- ` + "`project:/a.txt`" + ` — warum

## Ziel

Tun.

## Zu bauen

### Etappe 1 — A

x

### Etappe 2 — B

x

### Etappe 3 — C

x

### Etappe 4 — D

x
`

func taskTestDocs(t *testing.T, names ...string) (*UserDocs, *fakeDocStore, string) {
	t.Helper()
	store := newFakeDocStore("kamran", names...)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	return newTestUserDocs(t, store, DocKinds), store, dir
}

func violationsOf(t *testing.T, body string, noIntent bool) []string {
	t.Helper()
	docs, _, dir := taskTestDocs(t)
	v, _ := checkTask(context.Background(), "Titel", body, noIntent, taskRefChecker{projectDir: dir, docs: docs})
	return v
}

func TestTaskCreateGueltigerTaskHatKeineVerstoesse(t *testing.T) {
	if v := violationsOf(t, strings.TrimSpace(taskBodyOK), false); len(v) != 0 {
		t.Fatalf("Verstöße: %v", v)
	}
}

func TestTaskCreatePruefungen(t *testing.T) {
	ok := strings.TrimSpace(taskBodyOK)
	cases := map[string]struct {
		body     string
		noIntent bool
		want     string
	}{
		"kein Einleitungssatz":   {body: strings.TrimPrefix(ok, "Ein Satz, was der Task erreicht.\n\n"), want: "Einleitungssatz"},
		"Intent fehlt":           {body: strings.Replace(ok, "## Intent\n\nRahmen.\n- Bedingung\n\n", "", 1), want: "## Intent"},
		"Reihenfolge":            {body: strings.Replace(ok, "## Ziel\n\nTun.\n\n", "", 1) + "\n\n## Ziel\n\nTun.\n", want: "falschen Stelle"},
		"leerer Abschnitt":       {body: strings.Replace(ok, "Tun.", "", 1), want: "„Ziel“ ist leer"},
		"nackter Pfad":           {body: strings.Replace(ok, "`project:/a.txt`", "`a.txt`", 1), want: "kein Verweis mit Präfix"},
		"relativer Link":         {body: ok + "\n\nSiehe [x](../x.md).", want: "relativer Link"},
		"queue-Verweis":          {body: ok + "\n\nBefund keph:/queue/findings/001-x.md lesen.", want: "keph:/queue/"},
		"Standardwerkzeug":       {body: strings.Replace(ok, "## Ziel", "## Tools\n\n- `Read` — lesen\n\n## Ziel", 1), want: "Standardwerkzeug"},
		"Etappen nicht fortlauf": {body: strings.Replace(ok, "Etappe 3", "Etappe 5", 1), want: "nicht fortlaufend"},
		"H1 im Rumpf":            {body: "# Doppelt\n\n" + ok, want: "ersten Ebene"},
		"Frontmatter im Rumpf":   {body: "---\ntitle: x\n---\n\n" + ok, want: "Frontmatter"},
	}
	for name, c := range cases {
		got := strings.Join(violationsOf(t, c.body, c.noIntent), "\n")
		if !strings.Contains(got, c.want) {
			t.Errorf("%s: erwartet %q in %q", name, c.want, got)
		}
	}
}

func TestTaskCreateNoIntentHebtPflichtAuf(t *testing.T) {
	body := strings.Replace(strings.TrimSpace(taskBodyOK), "## Intent\n\nRahmen.\n- Bedingung\n\n", "", 1)
	if v := violationsOf(t, body, true); len(v) != 0 {
		t.Fatalf("Verstöße trotz noIntent: %v", v)
	}
}

func TestTaskCreateQueueOrtsangabeIstErlaubt(t *testing.T) {
	body := strings.TrimSpace(taskBodyOK) + "\n\nBefunde kommen nach keph:/queue/findings/ und keph:/queue/…"
	if v := violationsOf(t, body, false); len(v) != 0 {
		t.Fatalf("Ortsangabe gemeldet: %v", v)
	}
}

func TestTaskCreateCodeblockZaehltNicht(t *testing.T) {
	body := strings.TrimSpace(taskBodyOK) + "\n\n```markdown\n# Task 1 — x\n[l](../x.md)\n## Intent\nkeph:/queue/findings/001-x.md\n```\n"
	if v := violationsOf(t, body, false); len(v) != 0 {
		t.Fatalf("Codeblock gemeldet: %v", v)
	}
}

func TestTaskCreateTitel(t *testing.T) {
	docs, _, dir := taskTestDocs(t)
	checker := taskRefChecker{projectDir: dir, docs: docs}
	for _, title := range []string{"", "  ", "Task 12 — Foo", "task 3 - Foo"} {
		v, _ := checkTask(context.Background(), strings.TrimSpace(title), strings.TrimSpace(taskBodyOK), false, checker)
		if len(v) == 0 {
			t.Errorf("Titel %q angenommen", title)
		}
	}
}

func TestTaskCreateWarnungen(t *testing.T) {
	docs, _, dir := taskTestDocs(t)
	checker := taskRefChecker{projectDir: dir, docs: docs}
	body := strings.Replace(strings.TrimSpace(taskBodyOK), "`project:/a.txt`", "`project:/fehlt.txt`", 1)
	body = strings.Replace(body, "### Etappe 4 — D\n\nx", "", 1)
	body += "\n\n- `keph:/user/~/tasks/fehlt.md`"
	v, w := checkTask(context.Background(), "T", body, false, checker)
	if len(v) != 0 {
		t.Fatalf("Verstöße: %v", v)
	}
	joined := strings.Join(w, "\n")
	for _, want := range []string{"project:/fehlt.txt gibt es im Projekt nicht", "3 Etappen"} {
		if !strings.Contains(joined, want) {
			t.Errorf("Warnung %q fehlt in %q", want, joined)
		}
	}
}

func TestTaskCreateKephVerweisMitTilde(t *testing.T) {
	docs, store, dir := taskTestDocs(t, "user/kamran/tasks/001-da.md")
	checker := taskRefChecker{projectDir: dir, docs: docs}
	if w := checker.check(context.Background(), "keph:/user/~/tasks/001-da.md"); len(w) != 0 {
		t.Fatalf("vorhandenes Ziel gewarnt: %v", w)
	}
	if w := checker.check(context.Background(), "keph:/user/~/tasks/002-weg.md"); len(w) != 1 {
		t.Fatalf("fehlendes Ziel: %v", w)
	}
	if got := store.reads[0]; got != "user/kamran/tasks/001-da.md" {
		t.Fatalf("~ nicht ersetzt: %s", got)
	}
	if w := (taskRefChecker{projectDir: dir}).check(context.Background(), "keph:/x/y.md"); len(w) != 0 {
		t.Fatalf("ohne Anbindung gewarnt: %v", w)
	}
}

func TestTaskCreateTitelQuoting(t *testing.T) {
	cases := map[string]string{
		`Plain`:       `title: "Plain"`,
		`A: b`:        `title: "A: b"`,
		`A #b`:        `title: "A #b"`,
		`Sag "hallo"`: `title: "Sag \"hallo\""`,
		`Pfad a\b`:    `title: "Pfad a\\b"`,
	}
	for title, want := range cases {
		doc := composeTask("007", title, "Satz.")
		if !strings.Contains(doc, want+"\n---\n\n# Task 007 — "+title+"\n\nSatz.\n") {
			t.Errorf("%q: %q", title, doc)
		}
	}
}

func TestCreateTaskLegtInEinemSchreibvorgangAn(t *testing.T) {
	docs, store, dir := taskTestDocs(t, "user/kamran/tasks/004-alt.md")
	got, err := createTaskWith(context.Background(), docs, dir, TaskCreateOptions{Title: "Neu: mit #", Name: "Mein Neuer", Body: taskBodyOK})
	if err != nil {
		t.Fatal(err)
	}
	if got.Number != 5 || got.Name != "005-mein-neuer.md" || got.Ref != "keph:/user/kamran/tasks/005-mein-neuer.md" || got.Revision == 0 {
		t.Fatalf("Antwort: %+v", got)
	}
	if len(store.creates) != 1 || len(store.writes) != 0 {
		t.Fatalf("Aufrufe: create %v write %v", store.creates, store.writes)
	}
	doc := store.docs["user/kamran/tasks/005-mein-neuer.md"]
	if doc == nil || !strings.HasPrefix(doc.content, "---\ntitle: \"Neu: mit #\"\n---\n\n# Task 005 — Neu: mit #\n\nEin Satz") {
		t.Fatalf("Inhalt: %+v", doc)
	}
}

func TestCreateTaskNummernwechselBeiNameTaken(t *testing.T) {
	docs, store, dir := taskTestDocs(t)
	store.docs["user/kamran/tasks/001-x.md"] = &fakeDoc{by: "kamran", hidden: true}
	got, err := createTaskWith(context.Background(), docs, dir, TaskCreateOptions{Title: "T", Name: "x", Body: taskBodyOK})
	if err != nil {
		t.Fatal(err)
	}
	if got.Number != 2 || len(store.creates) != 2 {
		t.Fatalf("Antwort %+v, Aufrufe %v", got, store.creates)
	}
	content := store.docs["user/kamran/tasks/002-x.md"].content
	if !strings.Contains(content, "# Task 002 — T") || strings.Contains(content, "Task 001") {
		t.Fatalf("Überschrift nicht neu gebaut: %q", content)
	}
}

func TestCreateTaskTrockenlaufLegtNichtsAn(t *testing.T) {
	docs, store, dir := taskTestDocs(t)
	got, err := createTaskWith(context.Background(), docs, dir, TaskCreateOptions{Title: "T", Name: "x", Body: taskBodyOK, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(store.creates) != 0 || !got.DryRun || got.Name != "<NNN>-x.md" || got.Ref != "" ||
		!strings.Contains(got.Rendered, "# Task <NNN> — T") {
		t.Fatalf("Trockenlauf: %+v, create %v", got, store.creates)
	}
	// Derselbe Text wie beim Anlegen, bis auf die Nummer.
	real, err := createTaskWith(context.Background(), docs, dir, TaskCreateOptions{Title: "T", Name: "x", Body: taskBodyOK})
	if err != nil {
		t.Fatal(err)
	}
	if want := strings.Replace(got.Rendered, "<NNN>", "001", 1); store.docs["user/kamran/tasks/"+real.Name].content != want {
		t.Fatalf("Trockenlauf weicht ab")
	}
}

func TestCreateTaskVerstoesseSchreibenNichts(t *testing.T) {
	docs, store, dir := taskTestDocs(t)
	_, err := createTaskWith(context.Background(), docs, dir, TaskCreateOptions{Title: "", Name: "x", Body: "# H\n"})
	wantCode(t, err, DocCodeInvalidInput)
	if len(store.creates) != 0 {
		t.Fatalf("trotz Verstößen angelegt: %v", store.creates)
	}
}

func TestCreateTaskOutcomeUnknownZiehtKeineNeueNummer(t *testing.T) {
	docs, store, dir := taskTestDocs(t)
	store.createFail = &kephalaion.ToolError{Tool: "create", Code: kephalaion.CodeOutcomeUnknown, Message: "offen"}
	_, err := createTaskWith(context.Background(), docs, dir, TaskCreateOptions{Title: "T", Name: "x", Body: taskBodyOK})
	wantCode(t, err, DocCodeOutcomeUnknown)
	if len(store.creates) != 1 {
		t.Fatalf("Aufrufe: %v", store.creates)
	}
}

func TestCreateTaskWarntVorDoppeltemKurznamen(t *testing.T) {
	docs, _, dir := taskTestDocs(t, "user/kamran/tasks/001-x.md", "user/kamran/tasks/done/002-y.md")
	got, err := createTaskWith(context.Background(), docs, dir, TaskCreateOptions{Title: "T", Name: "x", Body: taskBodyOK, DryRun: true})
	if err != nil || !strings.Contains(strings.Join(got.Warnings, "\n"), "001-x.md") {
		t.Fatalf("Warnungen: %v %v", got.Warnings, err)
	}
	got, _ = createTaskWith(context.Background(), docs, dir, TaskCreateOptions{Title: "T", Name: "y", Body: taskBodyOK, DryRun: true})
	if strings.Contains(strings.Join(got.Warnings, "\n"), "002-y") {
		t.Fatalf("erledigter Task gewarnt: %v", got.Warnings)
	}
}

func TestCreateTaskTrockenlaufOhneKephalaion(t *testing.T) {
	dir := t.TempDir()
	got, err := createTaskWith(context.Background(), nil, dir, TaskCreateOptions{Title: "T", Name: "x", Body: taskBodyOK, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(got.Warnings, "\n"), "Kephalaion ist nicht bereit") {
		t.Fatalf("Hinweis fehlt: %v", got.Warnings)
	}
}
