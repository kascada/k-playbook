package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/project"
)

const cliTaskBody = "Ein Satz.\n\n## Intent\n\nRahmen.\n- Bedingung\n\n## Ziel\n\nTun.\n"

// runTaskCaptured führt `task create` mit dem Rumpf auf stdin aus und fängt
// stdout ab.
func runTaskCaptured(t *testing.T, body string, args ...string) (string, error) {
	t.Helper()
	dir := t.TempDir()
	in, err := os.Create(filepath.Join(dir, "stdin"))
	if err != nil {
		t.Fatal(err)
	}
	in.WriteString(body)
	in.Seek(0, 0)
	out, err := os.Create(filepath.Join(dir, "stdout"))
	if err != nil {
		t.Fatal(err)
	}
	before := os.Stdout
	os.Stdout = out
	runErr := runTaskCreate(args, in)
	os.Stdout = before
	out.Close()
	in.Close()
	data, _ := os.ReadFile(out.Name())
	return string(data), runErr
}

func TestTaskCreateSubkommando(t *testing.T) {
	todoProject(t)
	store := useCLIDocs(t, "user/anna/tasks/041-fremd.md")

	stdout, err := runTaskCaptured(t, cliTaskBody, "--title", "Mein Task", "--name", "Mein Task", "--dry-run")
	var dry project.TaskCreated
	if err != nil || json.Unmarshal([]byte(stdout), &dry) != nil || !dry.DryRun || !strings.Contains(dry.Rendered, "# Task <NNN> — Mein Task") {
		t.Fatalf("Trockenlauf: %q %v", stdout, err)
	}
	if len(store.content) != 0 {
		t.Fatalf("Trockenlauf hat angelegt: %v", store.content)
	}

	stdout, err = runTaskCaptured(t, cliTaskBody, "--title", "Mein Task", "--name", "Mein Task")
	var created project.TaskCreated
	if err != nil || json.Unmarshal([]byte(stdout), &created) != nil || created.Ref != "keph:/user/kamran/tasks/042-mein-task.md" {
		t.Fatalf("create: %q %v", stdout, err)
	}
	if got := store.content["user/kamran/tasks/042-mein-task.md"]; !strings.HasPrefix(got, "---\ntitle: \"Mein Task\"\n---\n\n# Task 042 — Mein Task\n") {
		t.Fatalf("Inhalt: %q", got)
	}

	_, err = runTaskCaptured(t, "# H1\n", "--title", "T", "--name", "x")
	if err == nil || !strings.Contains(err.Error(), "invalid_input") {
		t.Fatalf("Verstoß: %v", err)
	}
	if _, err = runTaskCaptured(t, cliTaskBody, "--name", "x"); err == nil {
		t.Fatal("ohne --title angenommen")
	}
	if _, err = runTaskCaptured(t, cliTaskBody, "--title", "T", "--name", "x", "--bogus"); err == nil {
		t.Fatal("unbekanntes Argument angenommen")
	}
}
