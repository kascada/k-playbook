package project

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// docsScopeProject legt ein Projekt mit git-Repository an: ein Commit, eine
// .gitignore, ein Eintrag in docs.exclude und Verzeichnisse für jede Art von
// Ausschluss.
func docsScopeProject(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git nicht installiert")
	}
	root := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	if err := CreateConfig(root, "."); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		".gitignore":                 "out/\n*.log\n!keep.log\n# Kommentar\n",
		"src/a.go":                   "package a\n",
		"src/.gitignore":             "generated/\n",
		"src/generated/g.go":         "x",
		"node_modules/x/index.js":    "x",
		".hidden/y.md":               "x",
		"keph-backup/z.md":           "x",
		"sub/keph-backup/q.md":       "x",
		"docs/a.md":                  "x",
		"build.log":                  "x",
		"keep.log":                   "x",
		"out/o.js":                   "x",
		"k-playbook/old.md":          "x",
		"assets/app.min.js":          "x",
		"tests/fixtures/data.json":   "{}",
		"src/tests/fixtures/more.js": "x",
	}
	for name, content := range files {
		full := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"add", "-A"},
		{"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "x"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	if _, err := AddDocsExclude(root, []string{"docs/", "vendor/lib"}); err != nil {
		t.Fatal(err)
	}
	return root
}

// docs scope: Stand, die Ausschlüsse nach Herkunft und --check je Pfad.
func TestDocsScope(t *testing.T) {
	root := docsScopeProject(t)
	result, err := DocsScope(context.Background(), root, DocsScopeOptions{Check: []string{
		"src/a.go", "node_modules", "node_modules/x/index.js", ".hidden/y.md", "keph-backup/z.md", "sub/keph-backup/q.md",
		".k-playbook/K-PLAYBOOK.yaml", "k-playbook/old.md", "out/o.js", "build.log", "keep.log", "src/generated/g.go",
		"docs/a.md", "assets/app.min.js", "tests/fixtures/data.json", "src/tests/fixtures/more.js", "vendor/lib/x.go",
		filepath.Join(root, "src", "a.go"), "../fremd.go",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if result.ScanRoot != root || result.ProjectDir != root {
		t.Errorf("Wurzeln: %s, %s", result.ScanRoot, result.ProjectDir)
	}
	if !result.Stand.Git || result.Stand.Branch != "main" || result.Stand.Commit == "" {
		t.Errorf("Stand: %+v", result.Stand)
	}
	if result.Stand.Clean {
		// Die Konfiguration und docs.exclude sind nach dem Commit geschrieben worden.
		if status, _ := exec.Command("git", "-C", root, "status", "--porcelain").Output(); strings.TrimSpace(string(status)) != "" {
			t.Errorf("Stand sauber, git sagt: %s", status)
		}
	}
	if got := strings.Join(result.Exclude.Always, " "); got != "k-playbook/ .k-playbook/ keph-backup/ .git/" {
		t.Errorf("immer ausgeschlossen: %s", got)
	}
	if !result.Exclude.Hidden.Excluded || len(result.Exclude.Hidden.Excepted) != 0 {
		t.Errorf("versteckt: %+v", result.Exclude.Hidden)
	}
	foundRoot, foundNested := false, false
	for _, file := range result.Exclude.Gitignore {
		switch file.File {
		case ".gitignore":
			foundRoot = len(file.Patterns) == 3 // der Kommentar fällt weg
		case "src/.gitignore":
			foundNested = len(file.Patterns) == 1
		}
	}
	if !foundRoot || !foundNested {
		t.Errorf(".gitignore: %+v", result.Exclude.Gitignore)
	}
	if got := strings.Join(result.Exclude.Config, " "); !strings.Contains(got, "docs/") || !strings.Contains(got, "vendor/lib") {
		t.Errorf("docs.exclude: %s", got)
	}

	want := map[string][2]string{ // Pfad → Grund, Muster
		"src/a.go":                    {"", ""},
		"node_modules":                {"further", "node_modules/"},
		"node_modules/x/index.js":     {"further", "node_modules/"},
		".hidden/y.md":                {"hidden", "."},
		"keph-backup/z.md":            {"always", "keph-backup/"},
		"sub/keph-backup/q.md":        {"always", "keph-backup/"},
		".k-playbook/K-PLAYBOOK.yaml": {"always", ".k-playbook/"},
		"k-playbook/old.md":           {"always", "k-playbook/"},
		"out/o.js":                    {"gitignore", ".gitignore: out/"},
		"build.log":                   {"gitignore", ".gitignore: *.log"},
		"keep.log":                    {"", ""},
		"src/generated/g.go":          {"gitignore", "src/.gitignore: generated/"},
		"docs/a.md":                   {"config", "docs/"},
		"assets/app.min.js":           {"further", "**/*.min.js"},
		"tests/fixtures/data.json":    {"further", "tests/fixtures/"},
		"src/tests/fixtures/more.js":  {"further", "tests/fixtures/"},
		"vendor/lib/x.go":             {"config", "vendor/lib"},
		"../fremd.go":                 {"outside", ""},
	}
	byPath := map[string]DocsScopeCheck{}
	for _, check := range result.Checks {
		byPath[check.Path] = check
	}
	for path, expected := range want {
		got, ok := byPath[path]
		if !ok {
			t.Errorf("%s fehlt in checks", path)
			continue
		}
		if got.Excluded != (expected[0] != "") || got.Reason != expected[0] || (expected[1] != "" && got.By != expected[1]) {
			t.Errorf("%s: %+v, erwartet %v", path, got, expected)
		}
	}
	if abs := byPath[filepath.Join(root, "src", "a.go")]; abs.Excluded {
		t.Errorf("absoluter Pfad im Scope: %+v", abs)
	}
}

// Der zusätzliche Pfad: Ein verstecktes Verzeichnis und ein Verzeichnis, das
// docs.exclude ausnimmt, kommen durch das Argument in den Scan; ein Pfad, den
// es nicht gibt, außerhalb des Projekts oder in einem immer ausgeschlossenen
// Verzeichnis, ist invalid_input.
func TestDocsScopeZusatzpfad(t *testing.T) {
	root := docsScopeProject(t)
	ctx := context.Background()

	result, err := DocsScope(ctx, root, DocsScopeOptions{ExtraPath: ".hidden", Check: []string{".hidden/y.md", ".git/config", "docs/a.md"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Extra == nil || result.Extra.Path != ".hidden" || !result.Extra.Exists || !result.Extra.InProject {
		t.Errorf("extra: %+v", result.Extra)
	}
	if got := result.Exclude.Hidden.Excepted; len(got) != 1 || got[0] != ".hidden" {
		t.Errorf("Ausnahme: %v", got)
	}
	if result.Checks[0].Excluded {
		t.Errorf("versteckter Pfad aus dem Argument ausgeschlossen: %+v", result.Checks[0])
	}
	if !result.Checks[1].Excluded || result.Checks[1].Reason != "always" {
		t.Errorf(".git/ ist nie im Scan: %+v", result.Checks[1])
	}
	if !result.Checks[2].Excluded || result.Checks[2].Reason != "config" {
		t.Errorf("docs/ bleibt ohne Argument ausgeschlossen: %+v", result.Checks[2])
	}

	result, err = DocsScope(ctx, root, DocsScopeOptions{ExtraPath: "docs", Check: []string{"docs/a.md"}})
	if err != nil || result.Checks[0].Excluded {
		t.Errorf("docs.exclude gegen das Argument: %+v %v", result.Checks, err)
	}

	abs, err := DocsScope(ctx, root, DocsScopeOptions{ExtraPath: filepath.Join(root, "src")})
	if err != nil || abs.Extra == nil || abs.Extra.Path != "src" {
		t.Errorf("absoluter Pfad im Projekt: %+v %v", abs.Extra, err)
	}

	for _, bad := range []string{"gibt-es-nicht", "../draussen", filepath.Dir(root), ".k-playbook", "k-playbook/old.md", "keph-backup", "sub/keph-backup/q.md", ".git"} {
		_, err := DocsScope(ctx, root, DocsScopeOptions{ExtraPath: bad})
		if DocRefErrorCode(err) != DocCodeInvalidInput {
			t.Errorf("Zusatzpfad %q: %v (%s)", bad, err, DocRefErrorCode(err))
		}
	}
}

// Ohne git-Arbeitsbaum meldet der Stand das, statt zu scheitern; ein Eintrag
// in docs.exclude, den das Lesen verworfen hat, steht unter rejected.
func TestDocsScopeOhneGit(t *testing.T) {
	root := t.TempDir()
	if err := CreateConfig(root, "."); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(root))
	config := ConfigPath(root)
	data, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, append(data, []byte("docs:\n  exclude:\n    - /abs/lute\n    - ../raus\n    - ok/\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := DocsScope(context.Background(), root, DocsScopeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Stand.Git || result.Stand.Message == "" {
		t.Errorf("Stand ohne git: %+v", result.Stand)
	}
	if !containsString(result.Exclude.Config, "ok/") || len(result.Exclude.Rejected) != 2 {
		t.Errorf("docs.exclude: %+v", result.Exclude)
	}
}
