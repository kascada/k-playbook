package kephalaion

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func needGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git fehlt")
	}
}

// gitRepo legt ein Repo mit k-playbook-local/ an und liefert dessen Pfad.
func gitRepo(t *testing.T) (root, local string) {
	t.Helper()
	needGit(t)
	root = t.TempDir()
	local = filepath.Join(root, "k-playbook-local")
	if err := os.MkdirAll(local, 0o755); err != nil {
		t.Fatal(err)
	}
	run(t, root, "init", "-q")
	return root, local
}

func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func ignored(t *testing.T, local string) bool {
	t.Helper()
	ok, err := isIgnored(local)
	if err != nil {
		t.Fatal(err)
	}
	return ok
}

func TestWriteBindingAddsIgnoreEntryWhenMissing(t *testing.T) {
	_, local := gitRepo(t)
	writeFile(t, filepath.Join(local, ".gitignore"), ".env")
	if ignored(t, local) {
		t.Fatal("Vorbedingung: noch nicht ignoriert")
	}
	if err := WriteBinding(local, goodBinding); err != nil {
		t.Fatal(err)
	}
	if !ignored(t, local) {
		t.Fatal("nach dem Schreiben nicht ignoriert")
	}
	data, _ := os.ReadFile(filepath.Join(local, ".gitignore"))
	if !strings.HasPrefix(string(data), ".env\n") || !strings.Contains(string(data), "\n"+IgnoreLine+"\n") {
		t.Fatalf(".gitignore: %q", data)
	}
	got, configured, err := ReadBinding(local)
	if err != nil || !configured || got != goodBinding {
		t.Fatalf("gelesen: %+v %v %v", got, configured, err)
	}
}

func TestWriteBindingCreatesGitignore(t *testing.T) {
	_, local := gitRepo(t)
	if err := WriteBinding(local, goodBinding); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(local, ".gitignore"))
	if err != nil || !strings.Contains(string(data), IgnoreLine) {
		t.Fatalf(".gitignore: %q %v", data, err)
	}
	if !ignored(t, local) {
		t.Fatal("nicht ignoriert")
	}
}

func TestWriteBindingKeepsParentIgnore(t *testing.T) {
	root, local := gitRepo(t)
	writeFile(t, filepath.Join(root, ".gitignore"), "/k-playbook-local/\n")
	if err := WriteBinding(local, goodBinding); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(local, ".gitignore")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("überflüssige .gitignore angelegt: %v", err)
	}
}

func TestWriteBindingRefusesTrackedFile(t *testing.T) {
	root, local := gitRepo(t)
	original := "# versioniert\nkephalaion:\n  hub: alt\n"
	writeFile(t, LocalFilePath(local), original)
	run(t, root, "add", "k-playbook-local/"+LocalFileName)
	err := WriteBinding(local, goodBinding)
	if !errors.Is(err, ErrTracked) || !strings.Contains(err.Error(), "git rm --cached") {
		t.Fatalf("Fehler: %v", err)
	}
	data, _ := os.ReadFile(LocalFilePath(local))
	if string(data) != original {
		t.Fatalf("Datei verändert: %q", data)
	}
	if _, err := os.Stat(filepath.Join(local, ".gitignore")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(".gitignore trotz Abbruch angelegt")
	}
}

func TestWriteBindingWithoutGitRepo(t *testing.T) {
	needGit(t)
	t.Setenv("GIT_CEILING_DIRECTORIES", os.TempDir())
	local := t.TempDir()
	if err := WriteBinding(local, goodBinding); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(local, ".gitignore")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(".gitignore ohne Repo angelegt")
	}
	data, _ := os.ReadFile(LocalFilePath(local))
	if !strings.HasPrefix(string(data), "# k-playbook — Einstellungen nur für diesen Rechner") {
		t.Fatalf("neue Datei ohne Kopf: %q", data)
	}
}

func TestWriteBindingKeepsRestOfFile(t *testing.T) {
	needGit(t)
	t.Setenv("GIT_CEILING_DIRECTORIES", os.TempDir())
	local := t.TempDir()
	before := "# Kopf\n# zweite Zeile\n\nvorher:\n  a: 1\n\nkephalaion:\n  # alter Kommentar\n  hub: alt\n  collection: alt\n  account: alt\n\n# Kommentar zum nächsten Block\nnachher:\n  b: 2\n"
	writeFile(t, LocalFilePath(local), before)
	if err := WriteBinding(local, goodBinding); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(LocalFilePath(local))
	want := "# Kopf\n# zweite Zeile\n\nvorher:\n  a: 1\n\n" + bindingBlock(goodBinding) + "\n# Kommentar zum nächsten Block\nnachher:\n  b: 2\n"
	if string(data) != want {
		t.Fatalf("Rest verändert:\n%s\n--- erwartet ---\n%s", data, want)
	}
	// Ein zweites Schreiben ändert nichts mehr.
	if err := WriteBinding(local, goodBinding); err != nil {
		t.Fatal(err)
	}
	again, _ := os.ReadFile(LocalFilePath(local))
	if string(again) != want {
		t.Fatalf("zweites Schreiben verändert:\n%s", again)
	}

	// Ohne Block wird er angehängt; der Rest bleibt.
	writeFile(t, LocalFilePath(local), "# nur Kopf\nanderes: 1\n")
	if err := WriteBinding(local, goodBinding); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(LocalFilePath(local))
	if string(data) != "# nur Kopf\nanderes: 1\n\n"+bindingBlock(goodBinding) {
		t.Fatalf("angehängt: %q", data)
	}
}

func TestWriteBindingRejectsInvalidNames(t *testing.T) {
	local := t.TempDir()
	if err := WriteBinding(local, Binding{Hub: "../x", Collection: "c", Account: "a"}); err == nil {
		t.Fatal("Fehler erwartet")
	}
	if _, err := os.Stat(LocalFilePath(local)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("trotz ungültigem Namen geschrieben")
	}
}

// Lehnt Git das Repo ab — hier wegen safe.directory („dubious ownership“) —,
// steht nicht fest, dass kein Repo da ist. Dann wird nichts geschrieben,
// weder die Datei noch .gitignore.
func TestWriteBindingRefusesWhenGitRejectsRepo(t *testing.T) {
	root, local := gitRepo(t)
	t.Setenv("GIT_TEST_ASSUME_DIFFERENT_OWNER", "1")
	if code, _ := git(root, "rev-parse", "--is-inside-work-tree"); code == 0 {
		t.Skip("git lehnt das Repo trotz GIT_TEST_ASSUME_DIFFERENT_OWNER nicht ab (safe.directory gesetzt?)")
	}
	err := WriteBinding(local, goodBinding)
	if err == nil || !strings.Contains(err.Error(), "safe.directory") || !strings.Contains(err.Error(), "nichts geschrieben") {
		t.Fatalf("Fehler: %v", err)
	}
	if _, err := os.Stat(LocalFilePath(local)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(LocalFileName + " trotz abgelehntem Repo geschrieben")
	}
	if _, err := os.Stat(filepath.Join(local, ".gitignore")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(".gitignore trotz abgelehntem Repo angelegt")
	}
}
