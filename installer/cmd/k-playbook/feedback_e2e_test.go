package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/kephalaion/kephtest"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// feedbackProject legt ein Projekt an, dessen Meldungen in einer Collection
// im Speicher liegen, und macht es zum Arbeitsverzeichnis.
func feedbackProject(t *testing.T) *kephtest.Store {
	t.Helper()
	store := kephtest.New()
	original, version := project.OpenFeedbackStore, project.FeedbackVersion
	project.OpenFeedbackStore = func(string) (project.FeedbackStore, project.FeedbackIdentity, error) {
		return store, project.FeedbackIdentity{User: "kamran", Account: "kamran-wsl", Address: "vm:test"}, nil
	}
	project.FeedbackVersion = func() string { return "v9.9.9" }
	t.Cleanup(func() { project.OpenFeedbackStore, project.FeedbackVersion = original, version })
	todoProjectRoot(t)
	return store
}

// runFeedbackCaptured führt `feedback …` mit stdin aus und fängt stdout ab.
func runFeedbackCaptured(t *testing.T, stdin string, args ...string) (string, error) {
	t.Helper()
	dir := t.TempDir()
	in, err := os.Create(filepath.Join(dir, "stdin"))
	if err != nil {
		t.Fatal(err)
	}
	in.WriteString(stdin)
	in.Seek(0, 0)
	out, err := os.Create(filepath.Join(dir, "stdout"))
	if err != nil {
		t.Fatal(err)
	}
	beforeIn, beforeOut := os.Stdin, os.Stdout
	os.Stdin, os.Stdout = in, out
	runErr := runFeedback(args)
	os.Stdin, os.Stdout = beforeIn, beforeOut
	out.Close()
	in.Close()
	data, _ := os.ReadFile(out.Name())
	return string(data), runErr
}

// Anlegen, Listen und Status ändern über die Kommandozeile; create verlangt
// --von, bei ki dazu client, modell und sitzung.
func TestFeedbackSubkommando(t *testing.T) {
	store := feedbackProject(t)
	base := []string{"create", "--title", "Wunsch: mehr Farbe", "--art", "wunsch", "--dringlichkeit", "niedrig",
		"--betrifft", "k-playbook"}

	if _, err := runFeedbackCaptured(t, "Text\n", base...); err == nil || !strings.Contains(err.Error(), "--von") {
		t.Fatalf("ohne --von: %v", err)
	}
	_, err := runFeedbackCaptured(t, "Text\n", append(base, "--von", "ki", "--client", "opencode")...)
	if err == nil || !strings.Contains(err.Error(), "invalid_input") || !strings.Contains(err.Error(), "modell, sitzung") {
		t.Fatalf("ki ohne modell und sitzung: %v", err)
	}
	if len(store.Docs) != 0 {
		t.Fatalf("angelegt trotz Fehler: %v", store.Names())
	}

	stdout, err := runFeedbackCaptured(t, "Bitte bunter.\n", append(base, "--von", "mensch")...)
	var created project.FeedbackCreated
	if err != nil || json.Unmarshal([]byte(stdout), &created) != nil || created.Ref != "keph:/feedback/001-wunsch-mehr-farbe.md" {
		t.Fatalf("create: %q %v", stdout, err)
	}
	content, _ := store.Content("feedback/001-wunsch-mehr-farbe.md")
	if !strings.Contains(content, "von: mensch\nuser: \"kamran\"\naccount: \"kamran-wsl\"\nversion: \"v9.9.9\"\n---") {
		t.Errorf("Kopf: %s", content)
	}

	stdout, err = runFeedbackCaptured(t, "", "list", "--betrifft", "k-playbook")
	var list project.FeedbackListing
	if err != nil || json.Unmarshal([]byte(stdout), &list) != nil || len(list.Items) != 1 || list.Items[0].Body != "Bitte bunter." {
		t.Fatalf("list: %q %v", stdout, err)
	}

	_, err = runFeedbackCaptured(t, "", "update", "1", "--status", "erledigt", "--since", "999")
	if err == nil || !strings.Contains(err.Error(), "foreign_change") {
		t.Fatalf("fremde Revision: %v", err)
	}
	stdout, err = runFeedbackCaptured(t, "", "update", "1", "--status", "erledigt", "--since",
		strconv.FormatInt(list.Items[0].Revision, 10))
	var updated project.FeedbackUpdated
	if err != nil || json.Unmarshal([]byte(stdout), &updated) != nil || updated.Feedback.Ref != "keph:/feedback/done/001-wunsch-mehr-farbe.md" {
		t.Fatalf("update: %q %v", stdout, err)
	}
	stdout, _ = runFeedbackCaptured(t, "", "list")
	if err := json.Unmarshal([]byte(stdout), &list); err != nil || len(list.Items) != 0 {
		t.Errorf("erledigte in der Liste ohne --all: %q", stdout)
	}
	stdout, _ = runFeedbackCaptured(t, "", "list", "--all")
	if err := json.Unmarshal([]byte(stdout), &list); err != nil || len(list.Items) != 1 || !list.Items[0].Done {
		t.Errorf("--all: %q", stdout)
	}
}

// Ohne Unterbefehl und mit --help nur die Kurzhilfe; die Collection wird
// nicht gefragt.
func TestFeedbackHilfeIstFolgenlos(t *testing.T) {
	store := feedbackProject(t)
	for _, args := range [][]string{{}, {"--help"}, {"help"}} {
		output, err := runFeedbackCaptured(t, "", args...)
		if err != nil || !strings.Contains(output, "k-playbook feedback — Meldungen des Projekts") {
			t.Errorf("%v: %q %v", args, output, err)
		}
	}
	if len(store.Calls) != 0 {
		t.Errorf("Collection gefragt: %v", store.Calls)
	}
}
