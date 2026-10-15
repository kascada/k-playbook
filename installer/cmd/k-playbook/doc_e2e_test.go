package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// cliDocStore ist ein Kephalaion im Speicher für die Tests des Subkommandos.
type cliDocStore struct {
	docs map[string]string
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
	if _, ok := s.docs[name]; ok {
		return kephalaion.Document{}, &kephalaion.ToolError{Tool: "create", Code: kephalaion.CodeNameTaken, Message: "belegt"}
	}
	s.docs[name] = "kamran"
	return kephalaion.Document{Name: name}, nil
}

func (s *cliDocStore) Rename(_ context.Context, name, newName string) (kephalaion.Document, error) {
	by, ok := s.docs[name]
	if !ok {
		return kephalaion.Document{}, &kephalaion.ToolError{Tool: "rename", Code: kephalaion.CodeNotFound, Message: "fehlt"}
	}
	if _, ok := s.docs[newName]; ok {
		return kephalaion.Document{}, &kephalaion.ToolError{Tool: "rename", Code: kephalaion.CodeNameTaken, Message: "belegt"}
	}
	delete(s.docs, name)
	s.docs[newName] = by
	return kephalaion.Document{Name: newName}, nil
}

func useCLIDocs(t *testing.T, names ...string) {
	t.Helper()
	store := &cliDocStore{docs: map[string]string{}}
	for _, name := range names {
		store.docs[name] = "anna"
	}
	original := openDocs
	openDocs = func(string) (*project.UserDocs, error) {
		return project.NewUserDocs(project.DocKinds, store, "kamran")
	}
	t.Cleanup(func() { openDocs = original })
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
