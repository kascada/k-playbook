package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/kephalaion/kephtest"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// useCLIDocsWork setzt eine Collection im Speicher hinter docs checkout und
// publish ein.
func useCLIDocsWork(t *testing.T) *kephtest.Store {
	t.Helper()
	store := kephtest.New()
	original := openDocsWork
	openDocsWork = func(projectDir string) (*project.DocsWork, error) {
		return project.NewDocsWork(projectDir, store, "vm:projekt"), nil
	}
	t.Cleanup(func() { openDocsWork = original })
	return store
}

// runDocsCaptured führt das Subkommando aus und fängt stdout und stderr ab.
func runDocsCaptured(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	dir := t.TempDir()
	stdoutFile, err := os.Create(filepath.Join(dir, "stdout"))
	if err != nil {
		t.Fatal(err)
	}
	stderrFile, err := os.Create(filepath.Join(dir, "stderr"))
	if err != nil {
		t.Fatal(err)
	}
	beforeOut, beforeErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = stdoutFile, stderrFile
	runErr := run(append([]string{"docs"}, args...))
	os.Stdout, os.Stderr = beforeOut, beforeErr
	stdoutFile.Close()
	stderrFile.Close()
	stdout, _ := os.ReadFile(stdoutFile.Name())
	stderr, _ := os.ReadFile(stderrFile.Name())
	return string(stdout), string(stderr), runErr
}

// Der Weg über die CLI: checkout holt, ein zweites checkout meldet das
// Arbeitsverzeichnis, publish veröffentlicht und räumt auf; --json trägt die
// Antwort von project.
func TestDocsSubkommandoAblauf(t *testing.T) {
	root := todoProjectRoot(t)
	store := useCLIDocsWork(t)
	store.Put("docs/generated/code/01-a.md", "---\ntitle: A\ndescription: D\n---\n")
	store.Put("docs/generated/code/02-b.md", "---\ntitle: B\ndescription: D\n---\n")

	stdout, _, err := runDocsCaptured(t, "checkout", "code")
	if err != nil || !strings.Contains(stdout, "Geholt: 2 Dokumente aus keph:/docs/generated/code/") || !strings.Contains(stdout, "01-a.md\t") {
		t.Fatalf("checkout: %q %v", stdout, err)
	}
	work := project.DocsWorkDir(root, "code")
	if _, err := os.Stat(filepath.Join(work, "02-b.md")); err != nil {
		t.Fatalf("nicht geholt: %v", err)
	}

	stdout, _, err = runDocsCaptured(t, "checkout", "code", "--json")
	var checkout project.DocsCheckoutResult
	if err != nil || json.Unmarshal([]byte(stdout), &checkout) != nil || checkout.State != project.DocsStateExisting || checkout.Existing.Files != 2 {
		t.Fatalf("checkout vorhanden: %q %v", stdout, err)
	}
	stdout, _, err = runDocsCaptured(t, "checkout", "code")
	if err != nil || !strings.Contains(stdout, "liegt schon da") || !strings.Contains(stdout, "--discard") {
		t.Fatalf("checkout vorhanden, Text: %q %v", stdout, err)
	}

	if err := os.WriteFile(filepath.Join(work, "01-a.md"), []byte("---\ntitle: A neu\ndescription: D\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(work, "02-b.md")); err != nil {
		t.Fatal(err)
	}
	stdout, _, err = runDocsCaptured(t, "publish", "code", "--json")
	var publish project.DocsPublishResult
	if err != nil || json.Unmarshal([]byte(stdout), &publish) != nil || !publish.Cleaned || publish.Readme.Status != project.DocsCreated {
		t.Fatalf("publish: %q %v", stdout, err)
	}
	if content, _ := store.Content("docs/generated/code/01-a.md"); !strings.Contains(content, "A neu") {
		t.Errorf("nicht geschrieben: %q", content)
	}
	if _, ok := store.Content("docs/generated/code/02-b.md"); ok {
		t.Error("02-b.md nicht gelöscht")
	}
	if _, err := os.Stat(work); !os.IsNotExist(err) {
		t.Errorf("Arbeitsverzeichnis nicht entfernt: %v", err)
	}

	// Ohne checkout: not_checked_out.
	_, _, err = runDocsCaptured(t, "publish", "code")
	if err == nil || !strings.Contains(err.Error(), "(not_checked_out)") {
		t.Fatalf("publish ohne checkout: %v", err)
	}

	// Konflikt: Text je Dokument, Exit mit Fehler, Arbeitsverzeichnis bleibt.
	if _, _, err := runDocsCaptured(t, "checkout", "code", "--discard"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "01-a.md"), []byte("---\ntitle: A lokal\ndescription: D\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	store.Put("docs/generated/code/01-a.md", "---\ntitle: A fremd\ndescription: D\n---\n")
	stdout, _, err = runDocsCaptured(t, "publish", "code")
	if err == nil || !strings.Contains(err.Error(), "01-a.md") || !strings.Contains(stdout, "conflict\tkeph:/docs/generated/code/01-a.md") ||
		!strings.Contains(stdout, "Offen: 01-a.md") {
		t.Fatalf("publish mit Konflikt: %q %v", stdout, err)
	}
	if _, err := os.Stat(work); err != nil {
		t.Errorf("Arbeitsverzeichnis nach Konflikt weg: %v", err)
	}

	// hints ohne --allow-hints: hints_changed, mit: geschrieben.
	if err := os.WriteFile(filepath.Join(work, "01-a.md"), []byte("---\ntitle: A lokal\ndescription: D\nhints: [ \"Neu\" ]\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runDocsCaptured(t, "checkout", "code", "--discard"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "03-neu.md"), []byte("---\ntitle: N\ndescription: D\nhints: [ \"Neu\" ]\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runDocsCaptured(t, "publish", "code"); err == nil || !strings.Contains(err.Error(), "(hints_changed)") {
		t.Fatalf("ohne --allow-hints: %v", err)
	}
	stdout, _, err = runDocsCaptured(t, "publish", "code", "--allow-hints")
	if err != nil || !strings.Contains(stdout, "created\tkeph:/docs/generated/code/03-neu.md") || !strings.Contains(stdout, "hints geändert (allow_hints): 03-neu.md") {
		t.Fatalf("mit --allow-hints: %q %v", stdout, err)
	}
}

// Aufruffehler, unbekannter Erzeuger und fehlendes Kephalaion enden mit
// einer Meldung samt Code, ohne etwas zu schreiben.
func TestDocsSubkommandoFehler(t *testing.T) {
	root := todoProjectRoot(t)
	useCLIDocsWork(t)

	for _, args := range [][]string{
		{"checkout"},
		{"checkout", "code", "libs"},
		{"checkout", "code", "--allow-hints"},
		{"publish", "code", "--discard"},
		{"pull", "code"},
	} {
		if _, _, err := runDocsCaptured(t, args...); err == nil {
			t.Errorf("%v: kein Fehler", args)
		}
	}
	if _, _, err := runDocsCaptured(t, "checkout", "libs"); err == nil || !strings.Contains(err.Error(), "(invalid_input)") {
		t.Errorf("unbekannter Erzeuger: %v", err)
	}
	stdout, _, err := runDocsCaptured(t, "--help")
	if err != nil || !strings.Contains(stdout, "k-playbook docs checkout <erzeuger>") {
		t.Errorf("Hilfe: %q %v", stdout, err)
	}

	// Ohne Anbindung: der echte Zugang, kein Node wird gefragt.
	openDocsWork = project.OpenDocsWork
	_, _, err = runDocsCaptured(t, "checkout", "code")
	if err == nil || !strings.Contains(err.Error(), "(kephalaion_unavailable)") {
		t.Errorf("ohne Kephalaion: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, project.ProjectDirName, "work")); !os.IsNotExist(err) {
		t.Errorf("ohne Kephalaion geschrieben: %v", err)
	}
}

// docs exclude add über die CLI: trägt ein, --json trägt die Antwort von
// project, ein Duplikat lässt die Datei unberührt, Verworfenes endet nach der
// Ausgabe mit Exit 1, Aufruffehler ohne Schreiben.
func TestDocsExcludeSubkommandoAblauf(t *testing.T) {
	root := todoProjectRoot(t)

	stdout, _, err := runDocsCaptured(t, "exclude", "add", "docs/", "rules/")
	if err != nil || !strings.Contains(stdout, "Eingetragen in ") || !strings.Contains(stdout, "docs.exclude: docs/, rules/") {
		t.Fatalf("exclude add: %q %v", stdout, err)
	}
	config, err := project.ReadConfig(root)
	if err != nil || strings.Join(config.DocsExclude, ",") != "docs/,rules/" {
		t.Fatalf("ReadConfig: %v %v", config.DocsExclude, err)
	}
	before, err := os.ReadFile(project.ConfigPath(root))
	if err != nil {
		t.Fatal(err)
	}

	stdout, _, err = runDocsCaptured(t, "exclude", "add", "docs/", "--json")
	var result project.DocsExcludeResult
	if err != nil || json.Unmarshal([]byte(stdout), &result) != nil || result.Changed || len(result.Added) != 0 || len(result.Exclude) != 2 {
		t.Fatalf("exclude add Duplikat: %q %v", stdout, err)
	}
	if after, _ := os.ReadFile(project.ConfigPath(root)); string(after) != string(before) {
		t.Errorf("Datei bei nichts Neuem geändert:\n%s", after)
	}

	stdout, _, err = runDocsCaptured(t, "exclude", "add", "--json", "--", "../weg", "-gen/")
	result = project.DocsExcludeResult{}
	if err == nil || !strings.Contains(err.Error(), "../weg") || json.Unmarshal([]byte(stdout), &result) != nil ||
		len(result.Rejected) != 1 || len(result.Added) != 1 || result.Added[0] != "-gen/" {
		t.Fatalf("exclude add verworfen: %q %v", stdout, err)
	}

	for _, args := range [][]string{
		{"exclude"},
		{"exclude", "add"},
		{"exclude", "add", "--allow-hints", "x/"},
		{"exclude", "remove", "x/"},
	} {
		stdout, _, err := runDocsCaptured(t, args...)
		if len(args) == 1 {
			if err != nil || !strings.Contains(stdout, "k-playbook docs exclude add <muster>") {
				t.Errorf("%v: Hilfe erwartet: %q %v", args, stdout, err)
			}
			continue
		}
		if err == nil {
			t.Errorf("%v: kein Fehler", args)
		}
	}
	config, err = project.ReadConfig(root)
	if err != nil || strings.Join(config.DocsExclude, ",") != "docs/,rules/,-gen/" {
		t.Errorf("Liste nach Aufruffehlern: %v %v", config.DocsExclude, err)
	}
}

// docs exclude add mit Zeilenumbruch im Muster über die CLI: verworfen mit
// Grund (Exit 1), in der Textausgabe in Anführungszeichen statt umgebrochen;
// das gültige Muster daneben kommt dazu, außerhalb des Blocks bleibt die Datei
// byte-gleich, und die gemeldete Liste ist die, die ReadConfig danach liest —
// ohne Block (er wird angelegt) und mit Block.
func TestDocsExcludeSubkommandoVerwirftZeilenumbruch(t *testing.T) {
	root := todoProjectRoot(t)
	before, err := os.ReadFile(project.ConfigPath(root))
	if err != nil {
		t.Fatal(err)
	}

	stdout, _, err := runDocsCaptured(t, "exclude", "add", "--", "a\nd", "neu/")
	if err == nil || !strings.Contains(err.Error(), `"a\nd"`) || !strings.Contains(stdout, `Verworfen: "a\nd" — `) ||
		!strings.Contains(stdout, "docs.exclude: neu/\n") {
		t.Fatalf("exclude add ohne Block: %q %v", stdout, err)
	}
	first, err := os.ReadFile(project.ConfigPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(first), strings.TrimRight(string(before), "\n")+"\n\n") ||
		!strings.HasSuffix(string(first), "  exclude:\n    - neu/\n") || strings.Contains(string(first), "\nd\n") {
		t.Fatalf("Datei nach dem Anlegen:\n%s", first)
	}

	stdout, _, err = runDocsCaptured(t, "exclude", "add", "--json", "--", "b\r\ne", "zwei/")
	var result project.DocsExcludeResult
	if err == nil || json.Unmarshal([]byte(stdout), &result) != nil || len(result.Rejected) != 1 ||
		result.Rejected[0].Pattern != "b\r\ne" || !strings.Contains(result.Rejected[0].Reason, "Steuerzeichen") ||
		strings.Join(result.Added, ",") != "zwei/" {
		t.Fatalf("exclude add mit Block: %q %v", stdout, err)
	}
	second, err := os.ReadFile(project.ConfigPath(root))
	if err != nil || string(second) != string(first)+"    - zwei/\n" {
		t.Fatalf("Datei nach dem Ergänzen: %v\n%s", err, second)
	}
	config, err := project.ReadConfig(root)
	if err != nil || strings.Join(config.DocsExclude, ",") != strings.Join(result.Exclude, ",") ||
		strings.Join(result.Exclude, ",") != "neu/,zwei/" || len(config.DocsProblems) != 0 {
		t.Errorf("ReadConfig: %q %+v %v; gemeldet %q", config.DocsExclude, config.DocsProblems, err, result.Exclude)
	}
}
