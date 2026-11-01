package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// cliDocStore ist ein Kephalaion im Speicher für die Tests des Subkommandos.
type cliDocStore struct {
	docs map[string]string
	// writeFail lässt create, write und rename mit diesem Fehler scheitern.
	writeFail error
	// content und revs tragen Inhalt und Revision je Dokument, rev die
	// letzte vergebene Revision.
	content map[string]string
	revs    map[string]int64
	rev     int64
}

func (s *cliDocStore) Read(_ context.Context, name string) (kephalaion.ReadResult, error) {
	by, ok := s.docs[name]
	if !ok {
		return kephalaion.ReadResult{Kind: kephalaion.ReadKindNone, Name: name}, nil
	}
	content := s.content[name]
	return kephalaion.ReadResult{Kind: kephalaion.ReadKindDocument, Name: name, Revision: s.revision(name), Content: &content,
		Updated: &kephalaion.Stamp{At: "2026-10-14T11:00:00.000Z", By: by}}, nil
}

func (s *cliDocStore) Write(_ context.Context, name, content string, base int64) (kephalaion.Document, error) {
	if s.writeFail != nil {
		return kephalaion.Document{}, s.writeFail
	}
	if _, ok := s.docs[name]; !ok {
		return kephalaion.Document{}, &kephalaion.ToolError{Tool: "write", Code: kephalaion.CodeNotFound, Message: "fehlt"}
	}
	if base > 0 && base != s.revision(name) {
		return kephalaion.Document{}, &kephalaion.ToolError{Tool: "write", Code: kephalaion.CodeStaleRevision, Message: "veraltet"}
	}
	s.content[name] = content
	return kephalaion.Document{Name: name, Revision: s.bump(name)}, nil
}

// revision ist die Revision eines Dokuments; eines ohne eigene bekommt
// beim ersten Blick eine.
func (s *cliDocStore) revision(name string) int64 {
	if s.revs[name] == 0 {
		s.bump(name)
	}
	return s.revs[name]
}

func (s *cliDocStore) bump(name string) int64 {
	s.rev++
	s.revs[name] = s.rev
	return s.rev
}

func (s *cliDocStore) List(_ context.Context, req kephalaion.ListRequest) ([]kephalaion.Entry, error) {
	var out []kephalaion.Entry
	for name, by := range s.docs {
		if strings.HasPrefix(name, req.Path+"/") {
			out = append(out, kephalaion.Entry{Kind: "document", Name: name,
				Created: &kephalaion.Stamp{At: "2026-10-14T10:00:00.000Z", By: by},
				Updated: &kephalaion.Stamp{At: "2026-10-14T11:00:00.000Z", By: by}})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (s *cliDocStore) Create(_ context.Context, name, _ string) (kephalaion.Document, error) {
	if s.writeFail != nil {
		return kephalaion.Document{}, s.writeFail
	}
	if _, ok := s.docs[name]; ok {
		return kephalaion.Document{}, &kephalaion.ToolError{Tool: "create", Code: kephalaion.CodeNameTaken, Message: "belegt"}
	}
	s.docs[name] = "kamran"
	return kephalaion.Document{Name: name, Revision: s.bump(name)}, nil
}

func (s *cliDocStore) Rename(_ context.Context, name, newName string) (kephalaion.Document, error) {
	if s.writeFail != nil {
		return kephalaion.Document{}, s.writeFail
	}
	by, ok := s.docs[name]
	if !ok {
		return kephalaion.Document{}, &kephalaion.ToolError{Tool: "rename", Code: kephalaion.CodeNotFound, Message: "fehlt"}
	}
	if _, ok := s.docs[newName]; ok {
		return kephalaion.Document{}, &kephalaion.ToolError{Tool: "rename", Code: kephalaion.CodeNameTaken, Message: "belegt"}
	}
	delete(s.docs, name)
	s.docs[newName] = by
	s.content[newName] = s.content[name]
	delete(s.content, name)
	delete(s.revs, name)
	return kephalaion.Document{Name: newName, Revision: s.bump(newName)}, nil
}

func useCLIDocs(t *testing.T, names ...string) *cliDocStore {
	t.Helper()
	store := &cliDocStore{docs: map[string]string{}, content: map[string]string{}, revs: map[string]int64{}}
	for _, name := range names {
		store.docs[name] = "anna"
	}
	original := openDocs
	openDocs = func(string) (*project.UserDocs, error) {
		return project.NewUserDocs(project.DocKinds, store, "kamran")
	}
	t.Cleanup(func() { openDocs = original })
	return store
}

// runDocCaptured führt das Subkommando aus und fängt stdout und stderr ab.
func runDocCaptured(t *testing.T, args ...string) (string, string, error) {
	t.Helper()

	dir := t.TempDir()
	stdoutFile, err := os.Create(filepath.Join(dir, "stdout"))
	if err != nil {
		t.Fatalf("Ausgabedatei: %v", err)
	}
	stderrFile, err := os.Create(filepath.Join(dir, "stderr"))
	if err != nil {
		t.Fatalf("Fehlerdatei: %v", err)
	}
	beforeOut, beforeErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = stdoutFile, stderrFile
	runErr := runDoc(args)
	os.Stdout, os.Stderr = beforeOut, beforeErr
	stdoutFile.Close()
	stderrFile.Close()

	stdout, _ := os.ReadFile(stdoutFile.Name())
	stderr, _ := os.ReadFile(stderrFile.Name())
	return string(stdout), string(stderr), runErr
}

func TestDocSubkommandoAblauf(t *testing.T) {
	todoProject(t)
	useCLIDocs(t, "user/anna/tasks/100-fremd.md", "queue/findings/003-alt.md")

	stdout, _, err := runDocCaptured(t, "create", "tasks", "Meine Aufgabe")
	if err != nil || stdout != "keph:/user/kamran/tasks/101-meine-aufgabe.md\n" {
		t.Fatalf("create: %q %v", stdout, err)
	}
	stdout, _, err = runDocCaptured(t, "create", "findings", "Befund", "--json")
	var created project.DocCreated
	if err != nil || json.Unmarshal([]byte(stdout), &created) != nil || created.Ref != "keph:/queue/findings/004-befund.md" || created.User != "" {
		t.Fatalf("create --json: %q %v", stdout, err)
	}

	stdout, _, err = runDocCaptured(t, "list", "tasks", "--user", "*")
	if err != nil || strings.Count(stdout, "\n") != 2 || !strings.Contains(stdout, "keph:/user/anna/tasks/100-fremd.md\t100-fremd.md\tanna\t") {
		t.Fatalf("list: %q %v", stdout, err)
	}
	stdout, stderr, err := runDocCaptured(t, "list", "findings", "--user=kamran", "--json")
	var list project.DocList
	if err != nil || json.Unmarshal([]byte(stdout), &list) != nil || len(list.Items) != 2 || !strings.Contains(stderr, "Userbereich") {
		t.Fatalf("list findings: %q %q %v", stdout, stderr, err)
	}

	stdout, _, err = runDocCaptured(t, "done", "keph:/user/~/tasks/101-meine-aufgabe.md")
	if err != nil || stdout != "keph:/user/kamran/tasks/done/101-meine-aufgabe.md\n" {
		t.Fatalf("done: %q %v", stdout, err)
	}
	stdout, _, err = runDocCaptured(t, "list", "tasks", "--done")
	if err != nil || !strings.HasSuffix(strings.TrimSpace(stdout), "\tdone") {
		t.Fatalf("list --done: %q %v", stdout, err)
	}
	stdout, _, err = runDocCaptured(t, "reopen", "keph:/user/kamran/tasks/done/101-meine-aufgabe.md", "--json")
	var moved project.DocMoved
	if err != nil || json.Unmarshal([]byte(stdout), &moved) != nil || moved.Ref != "keph:/user/kamran/tasks/101-meine-aufgabe.md" {
		t.Fatalf("reopen: %q %v", stdout, err)
	}

	_, _, err = runDocCaptured(t, "done", "keph:/user/anna/tasks/100-fremd.md")
	if err == nil || !strings.Contains(err.Error(), "(foreign_user)") {
		t.Fatalf("fremder User: %v", err)
	}
}

func TestDocSubkommandoAufrufFehler(t *testing.T) {
	todoProject(t)
	useCLIDocs(t)

	for _, args := range [][]string{
		{"create", "tasks"},
		{"list"},
		{"done"},
		{"list", "tasks", "--user"},
		{"done", "keph:/x.md", "--done"},
		{"loeschen", "x"},
	} {
		if _, _, err := runDocCaptured(t, args...); err == nil {
			t.Errorf("%v: kein Fehler", args)
		}
	}
	for _, args := range [][]string{{}, {"--help"}, {"create", "--help"}} {
		stdout, _, err := runDocCaptured(t, args...)
		if err != nil || !strings.Contains(stdout, "k-playbook doc create <sorte>") || !strings.Contains(stdout, "tasks, workbench, findings") {
			t.Errorf("%v: Hilfe fehlt: %q %v", args, stdout, err)
		}
	}
}

func TestDocSubkommandoOhneKephalaion(t *testing.T) {
	// Ohne Anbindung: der echte Zugang, kein Node wird gefragt.
	todoProject(t)
	_, _, err := runDocCaptured(t, "list", "tasks")
	if err == nil || !strings.Contains(err.Error(), "(kephalaion_unavailable)") {
		t.Fatalf("erwartet kephalaion_unavailable: %v", err)
	}
}

func TestDocSubkommandoOffenerAusgang(t *testing.T) {
	todoProject(t)
	store := useCLIDocs(t, "user/kamran/tasks/100-a.md")
	store.writeFail = &kephalaion.ToolError{Tool: "create", Code: kephalaion.CodeOutcomeUnknown, Message: "Ausgang offen"}
	for _, args := range [][]string{{"create", "tasks", "b"}, {"done", "keph:/user/~/tasks/100-a.md"}} {
		_, _, err := runDocCaptured(t, args...)
		if err == nil || !strings.HasSuffix(err.Error(), "(outcome_unknown)") || !strings.Contains(err.Error(), "doc list") {
			t.Errorf("%v: %v", args, err)
		}
	}
	store.writeFail = &kephalaion.ToolError{Tool: "create", Code: kephalaion.CodeUnreachable, Message: "Hub nicht erreicht"}
	_, _, err := runDocCaptured(t, "create", "tasks", "b")
	if err == nil || !strings.HasSuffix(err.Error(), "(kephalaion_unavailable)") {
		t.Fatalf("unreachable: %v", err)
	}
}

// withDocStdin setzt die Eingabe von doc append und doc replace.
func withDocStdin(t *testing.T, text string) {
	t.Helper()
	original := docStdin
	docStdin = strings.NewReader(text)
	t.Cleanup(func() { docStdin = original })
}

// useCLILayers lässt keph:-Verweise von doc grep über den Store im Speicher
// laufen.
func useCLILayers(t *testing.T, store *cliDocStore) {
	t.Helper()
	original := docLayers
	docLayers = func(dir string) project.DocLayers {
		layers := project.ProjectDocLayers(dir)
		layers.Keph = func() (project.KephDocs, error) { return project.KephDocs{Store: store, User: "kamran"}, nil }
		return layers
	}
	t.Cleanup(func() { docLayers = original })
}

func TestDocSubkommandoAendern(t *testing.T) {
	todoProject(t)
	store := useCLIDocs(t, "user/kamran/tasks/100-a.md", "user/anna/tasks/101-b.md")
	store.content["user/kamran/tasks/100-a.md"] = "# A"
	ref := "keph:/user/~/tasks/100-a.md"
	name := "user/kamran/tasks/100-a.md"

	withDocStdin(t, "Text mit `Backticks` und $(nicht ausgeführt)\n")
	stdout, _, err := runDocCaptured(t, "append", ref)
	fields := strings.Split(strings.TrimSpace(stdout), "\t")
	if err != nil || len(fields) != 2 || fields[0] != "keph:/"+name {
		t.Fatalf("append: %q %v", stdout, err)
	}
	if got := store.content[name]; got != "# A\n\nText mit `Backticks` und $(nicht ausgeführt)\n" {
		t.Fatalf("Inhalt: %q", got)
	}

	// Zwei append --since hintereinander, das zweite mit der Revision aus dem ersten.
	withDocStdin(t, "zwei")
	stdout, _, err = runDocCaptured(t, "append", ref, "--since", fields[1], "--json")
	var edited project.DocEdited
	if err != nil || json.Unmarshal([]byte(stdout), &edited) != nil || !edited.Changed {
		t.Fatalf("append --since: %q %v", stdout, err)
	}
	withDocStdin(t, "drei")
	stdout, _, err = runDocCaptured(t, "append", ref, "--since="+strconv.FormatInt(edited.Revision, 10))
	if err != nil {
		t.Fatalf("zweites append --since: %q %v", stdout, err)
	}

	// Veraltete Revision: foreign_change, die letzten Zeilen auf stderr.
	withDocStdin(t, "vier")
	_, stderr, err := runDocCaptured(t, "append", ref, "--since", "1")
	if err == nil || !strings.HasSuffix(err.Error(), "(foreign_change)") || !strings.Contains(stderr, "drei") {
		t.Fatalf("foreign_change: %v %q", err, stderr)
	}
	withDocStdin(t, "vier")
	stdout, _, err = runDocCaptured(t, "append", ref, "--since", "1", "--json")
	var change project.DocForeignChange
	if err == nil || json.Unmarshal([]byte(stdout), &change) != nil || change.Revision == 0 || !strings.Contains(change.Tail, "drei") {
		t.Fatalf("foreign_change --json: %q %v", stdout, err)
	}

	withDocStdin(t, `{"old": "zwei", "new": "2"}`)
	stdout, _, err = runDocCaptured(t, "replace", ref)
	if err != nil || !strings.HasPrefix(stdout, "keph:/"+name+"\t") || !strings.Contains(store.content[name], "\n2\n") {
		t.Fatalf("replace: %q %v %q", stdout, err, store.content[name])
	}
	withDocStdin(t, `{"old": "fehlt"}`)
	_, _, err = runDocCaptured(t, "replace", ref)
	if err == nil || !strings.HasSuffix(err.Error(), "(text_not_found)") {
		t.Fatalf("replace fehlt: %v", err)
	}
	withDocStdin(t, `kein json`)
	if _, _, err = runDocCaptured(t, "replace", ref); err == nil {
		t.Fatal("replace ohne JSON angenommen")
	}

	stdout, _, err = runDocCaptured(t, "set", ref, "status", "-- offen")
	if err == nil {
		t.Fatalf("Wert mit - ohne -- angenommen: %q", stdout)
	}
	stdout, _, err = runDocCaptured(t, "set", ref, "status", "offen: ja")
	if err != nil || !strings.HasPrefix(store.content[name], "---\nstatus: \"offen: ja\"\n---\n\n# A\n") {
		t.Fatalf("set: %q %v %q", stdout, err, store.content[name])
	}
	stdout, _, err = runDocCaptured(t, "set", ref, "status", "offen: ja")
	if err != nil || !strings.HasSuffix(strings.TrimSpace(stdout), "\tunverändert") {
		t.Fatalf("set unverändert: %q %v", stdout, err)
	}
	stdout, _, err = runDocCaptured(t, "set", ref, "--", "titel", "-x")
	if err != nil || !strings.Contains(store.content[name], "titel: \"-x\"\n") {
		t.Fatalf("set nach --: %q %v %q", stdout, err, store.content[name])
	}
	_, _, err = runDocCaptured(t, "set", ref, "--unset", "status")
	if err != nil || strings.Contains(store.content[name], "status") {
		t.Fatalf("set --unset: %v %q", err, store.content[name])
	}

	_, _, err = runDocCaptured(t, "set", "keph:/user/anna/tasks/101-b.md", "a", "b")
	if err == nil || !strings.HasSuffix(err.Error(), "(foreign_user)") {
		t.Fatalf("fremder User: %v", err)
	}
	withDocStdin(t, "x")
	_, _, err = runDocCaptured(t, "append", "project:/README.md")
	if err == nil || !strings.HasSuffix(err.Error(), "(invalid_input)") {
		t.Fatalf("project:: %v", err)
	}
}

func TestDocSubkommandoGrep(t *testing.T) {
	root := todoProject(t)
	store := useCLIDocs(t, "user/kamran/tasks/100-a.md", "user/kamran/tasks/done/101-b.md")
	store.content["user/kamran/tasks/100-a.md"] = "## Intent\nstatus: offen\n"
	store.content["user/kamran/tasks/done/101-b.md"] = "status: erledigt\n"
	useCLILayers(t, store)
	if err := os.WriteFile(filepath.Join(root, "notiz.md"), []byte("eins\n-zwei\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, _, err := runDocCaptured(t, "grep", "--kind", "tasks", "--done", "^status:")
	if err != nil || stdout != "keph:/user/kamran/tasks/100-a.md:2:status: offen\nkeph:/user/kamran/tasks/done/101-b.md:1:status: erledigt\n" {
		t.Fatalf("Sorte: %q %v", stdout, err)
	}
	stdout, _, err = runDocCaptured(t, "grep", "keph:/user/~/tasks/100-a.md", "nichts", "--count")
	if err != nil || !strings.HasPrefix(stdout, "keph:/user/kamran/tasks/100-a.md:0:") {
		t.Fatalf("--count: %q %v", stdout, err)
	}
	stdout, _, err = runDocCaptured(t, "grep", "keph:/user/~/tasks/100-a.md", "Intent", "--json")
	var result project.DocGrepResult
	if err != nil || json.Unmarshal([]byte(stdout), &result) != nil || result.Revision == 0 || result.Count != 1 {
		t.Fatalf("--json: %q %v", stdout, err)
	}
	stdout, _, err = runDocCaptured(t, "grep", "project:/notiz.md", "--", "-zwei")
	if err != nil || stdout != "project:/notiz.md:2:-zwei\n" {
		t.Fatalf("project: %q %v", stdout, err)
	}
	for _, args := range [][]string{
		{"grep", "keph:/user/~/tasks/100-a.md"},
		{"grep", "keph:/user/~/tasks/100-a.md", "a", "--user", "*"},
		{"grep", "--kind", "tasks"},
	} {
		if _, _, err := runDocCaptured(t, args...); err == nil {
			t.Errorf("%v: kein Fehler", args)
		}
	}
	_, _, err = runDocCaptured(t, "grep", "--kind", "tasks", "(")
	if err == nil || !strings.HasSuffix(err.Error(), "(invalid_input)") {
		t.Fatalf("kaputtes Muster: %v", err)
	}
}

func TestDocSubkommandoDoneMitRevision(t *testing.T) {
	todoProject(t)
	useCLIDocs(t, "user/kamran/tasks/100-a.md")
	stdout, _, err := runDocCaptured(t, "done", "keph:/user/~/tasks/100-a.md", "--json")
	var moved project.DocMoved
	if err != nil || json.Unmarshal([]byte(stdout), &moved) != nil || moved.Revision == 0 {
		t.Fatalf("done --json: %q %v", stdout, err)
	}
	stdout, _, err = runDocCaptured(t, "reopen", moved.Ref)
	if err != nil || stdout != "keph:/user/kamran/tasks/100-a.md\n" {
		t.Fatalf("reopen ohne --json bleibt der Verweis: %q %v", stdout, err)
	}
}
