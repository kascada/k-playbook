package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
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
	templates := repoDocsTemplates(t)
	original := openDocsWork
	openDocsWork = func(projectDir string) (*project.DocsWork, error) {
		return project.NewDocsWork(projectDir, store, "vm:projekt").WithTemplates(templates), nil
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

// Der Erzeuger libs (Task 120) geht denselben Weg wie code: checkout holt
// docs/generated/libs/, publish legt neue Steckbriefe an, schreibt die README
// des Verzeichnisses aus der Erzeugertabelle und räumt auf; code bleibt
// unberührt.
func TestDocsSubkommandoLibs(t *testing.T) {
	root := todoProjectRoot(t)
	store := useCLIDocsWork(t)
	store.Put("docs/generated/code/01-a.md", "---\ntitle: A\ndescription: D\n---\n")
	store.Put("docs/generated/libs/python-celery.md", "---\ntitle: celery\ndescription: D\ngenerated: { by: k-docs-tools, at: 2026-11-01T10:00:00+01:00 }\n---\n")

	stdout, _, err := runDocsCaptured(t, "checkout", "libs")
	if err != nil || !strings.Contains(stdout, "Geholt: 1 Dokumente aus keph:/docs/generated/libs/") || !strings.Contains(stdout, "python-celery.md\t") {
		t.Fatalf("checkout libs: %q %v", stdout, err)
	}
	work := project.DocsWorkDir(root, "libs")
	if _, err := os.Stat(filepath.Join(work, "python-celery.md")); err != nil {
		t.Fatalf("nicht geholt: %v", err)
	}
	if _, err := os.Stat(filepath.Join(work, "01-a.md")); !os.IsNotExist(err) {
		t.Errorf("Dokument aus code/ im Arbeitsverzeichnis von libs: %v", err)
	}
	neu := "---\ntitle: goldmark\ndescription: D\ngenerated: { by: k-docs-tools, at: 2026-11-06T10:00:00+01:00 }\n" +
		"sources:\n  - resource: keph:/docs/generated/versions/inventory.md\n    item: go/github.com/yuin/goldmark\n    version: v1.7.8\n---\n"
	if err := os.WriteFile(filepath.Join(work, "go-github.com-yuin-goldmark.md"), []byte(neu), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, _, err = runDocsCaptured(t, "publish", "libs", "--json")
	var publish project.DocsPublishResult
	if err != nil || json.Unmarshal([]byte(stdout), &publish) != nil || !publish.Cleaned || publish.Readme.Status != project.DocsCreated {
		t.Fatalf("publish libs: %q %v", stdout, err)
	}
	if content, ok := store.Content("docs/generated/libs/go-github.com-yuin-goldmark.md"); !ok || content != neu {
		t.Errorf("nicht angelegt: %q", content)
	}
	if readme, ok := store.Content("docs/generated/libs/README.md"); !ok || !strings.Contains(readme, "title: \"Profiles of libraries and tools\"") {
		t.Errorf("README von libs/: %q", readme)
	}
	if content, _ := store.Content("docs/generated/code/01-a.md"); content != "---\ntitle: A\ndescription: D\n---\n" {
		t.Errorf("code/ verändert: %q", content)
	}
	if _, err := os.Stat(work); !os.IsNotExist(err) {
		t.Errorf("Arbeitsverzeichnis nicht entfernt: %v", err)
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
	if _, _, err := runDocsCaptured(t, "checkout", "tools"); err == nil || !strings.Contains(err.Error(), "(invalid_input)") {
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

	stdout, _, err := runDocsCaptured(t, "exclude", "add", "docs/", "vendor/")
	if err != nil || !strings.Contains(stdout, "Eingetragen in ") || !strings.Contains(stdout, "docs.exclude: docs/, vendor/") {
		t.Fatalf("exclude add: %q %v", stdout, err)
	}
	config, err := project.ReadConfig(root)
	if err != nil || strings.Join(config.DocsExclude, ",") != "docs/,vendor/" {
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
	if err != nil || strings.Join(config.DocsExclude, ",") != "docs/,vendor/,-gen/" {
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

// repoDefaultsDir ist defaults/ des Repos — über den Ort dieser Datei, nicht
// das Arbeitsverzeichnis, das die Tests ins Projekt wechseln.
func repoDefaultsDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("Ort der Testdatei unbekannt")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "defaults")
}

// repoDocsTemplates ist das defaults/ des Repos als mitgelieferte Ebene — die
// Vorlagen unter docs/, wie der Hub sie nach dem Release trägt.
func repoDocsTemplates(t *testing.T) project.DocSource {
	t.Helper()
	store, err := kephtest.Defaults(repoDefaultsDir(t))
	if err != nil {
		t.Fatalf("defaults/ des Repos laden: %v", err)
	}
	return project.NewKephDefaultsSource(func() (project.DefaultsStore, string, error) { return store, "test:k-playbook", nil })
}

// useCLIDocsReadmes setzt eine Collection im Speicher und die Vorlagen des
// Repos hinter docs readme ein.
func useCLIDocsReadmes(t *testing.T) *kephtest.Store {
	t.Helper()
	store := kephtest.New()
	templates := repoDocsTemplates(t)
	original := openDocsReadmes
	openDocsReadmes = func(string) (*project.DocsReadmes, error) { return project.NewDocsReadmes(store, templates), nil }
	t.Cleanup(func() { openDocsReadmes = original })
	return store
}

// docs readme über die CLI: --check schreibt nichts, ohne Pfad alle, mit Pfad
// die eine; eine fehlende Vorlage endet mit Exit 1 und template_missing; ein
// ungültiger Pfad ist invalid_input.
func TestDocsReadmeSubkommando(t *testing.T) {
	todoProjectRoot(t)
	store := useCLIDocsReadmes(t)
	store.Put("docs/manual/README.md", "---\ntitle: alt\n---\n")

	stdout, _, err := runDocsCaptured(t, "readme", "--check")
	if err != nil || !strings.Contains(stdout, "missing\tkeph:/docs/generated/code/README.md\t0") ||
		!strings.Contains(stdout, "differs\tkeph:/docs/manual/README.md") || !strings.Contains(stdout, "nichts geschrieben") {
		t.Fatalf("--check: %q %v", stdout, err)
	}
	if len(store.Names()) != 1 {
		t.Fatalf("--check hat geschrieben: %v", store.Names())
	}

	stdout, _, err = runDocsCaptured(t, "readme", "keph:/docs/manual/", "--json")
	var one project.DocsReadmeResult
	if err != nil || json.Unmarshal([]byte(stdout), &one) != nil || len(one.Readmes) != 1 || one.Readmes[0].Status != project.DocsWritten ||
		one.Readmes[0].Dir != "manual" {
		t.Fatalf("readme manual: %q %v", stdout, err)
	}

	stdout, _, err = runDocsCaptured(t, "readme")
	if err != nil || strings.Count(stdout, "created\t") != 6 || !strings.Contains(stdout, "unchanged\tkeph:/docs/manual/README.md") {
		t.Fatalf("readme alle: %q %v", stdout, err)
	}

	stdout, _, err = runDocsCaptured(t, "readme", "generated/fehlt")
	if err == nil || !strings.Contains(stdout, "failed\tkeph:/docs/generated/fehlt/README.md\t0\ttemplate_missing") {
		t.Fatalf("fehlende Vorlage: %q %v", stdout, err)
	}
	if _, _, err := runDocsCaptured(t, "readme", "keph:/docs/README.md"); err == nil || !strings.Contains(err.Error(), "(invalid_input)") {
		t.Fatalf("docs/README.md: %v", err)
	}
	if _, _, err := runDocsCaptured(t, "readme", "a", "b"); err == nil {
		t.Fatal("zwei Pfade angenommen")
	}
}

// Fehlt die README-Vorlage (Versatz Binary/Hub, Task 133), veröffentlicht
// publish über die CLI die Dokumente und räumt auf, endet aber mit Exit 1 und
// nennt die README mit template_missing; eine README entsteht nicht.
func TestDocsSubkommandoPublishOhneVorlage(t *testing.T) {
	root := todoProjectRoot(t)
	store := kephtest.New()
	empty := project.NewDiskDocSource(t.TempDir())
	original := openDocsWork
	openDocsWork = func(projectDir string) (*project.DocsWork, error) {
		return project.NewDocsWork(projectDir, store, "vm:projekt").WithTemplates(empty), nil
	}
	t.Cleanup(func() { openDocsWork = original })
	store.Put("docs/generated/code/01-a.md", "---\ntitle: A\ndescription: D\n---\n")

	if _, _, err := runDocsCaptured(t, "checkout", "code"); err != nil {
		t.Fatal(err)
	}
	work := project.DocsWorkDir(root, "code")
	if err := os.WriteFile(filepath.Join(work, "01-a.md"), []byte("---\ntitle: A neu\ndescription: D\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err := runDocsCaptured(t, "publish", "code")
	if err == nil || !strings.Contains(err.Error(), "Dokumente veröffentlicht, aber keph:/docs/generated/code/README.md nicht geschrieben") ||
		!strings.Contains(err.Error(), "("+project.DocCodeTemplateMissing+")") {
		t.Fatalf("publish ohne Vorlage: %v", err)
	}
	if content, _ := store.Content("docs/generated/code/01-a.md"); !strings.Contains(content, "A neu") {
		t.Errorf("Dokument nicht geschrieben: %q", content)
	}
	if _, ok := store.Content("docs/generated/code/README.md"); ok {
		t.Error("README ohne Vorlage angelegt")
	}
	if _, err := os.Stat(work); !os.IsNotExist(err) {
		t.Errorf("Arbeitsverzeichnis nicht entfernt: %v", err)
	}
}
