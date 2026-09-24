package branches

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kascada/k-playbook/installer/internal/github"
)

// Die Tests dieses Pakets laufen an Repos, die sie selbst anlegen. Nie an einem
// echten Arbeitsrepo: umgeschaltet wird ausschließlich hier im Temp-Verzeichnis.

// isolateGit trennt git von der Konfiguration des Rechners: keine globalen
// Hooks, keine Signatur, kein fremder Default-Branch.
func isolateGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git fehlt")
	}
	config := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(config, []byte("[init]\n\tdefaultBranch = main\n[commit]\n\tgpgsign = false\n[advice]\n\tdetachedHead = false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", config)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.invalid")
	t.Setenv("GIT_COMMITTER_NAME", "Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.invalid")
}

// run führt git in dir aus und bricht den Test bei einem Fehler ab.
func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s in %s: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// commitClock gibt jedem Commit eine eigene Minute. Commits innerhalb derselben
// Sekunde hätten sonst dasselbe Datum, und die Ordnung „jüngster zuerst" hinge
// am Zufall.
var commitClock = time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)

// commit legt eine Datei an und committet sie.
func commit(t *testing.T, dir string, file string, content string, message string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, file), content)
	run(t, dir, "add", "--", file)
	commitClock = commitClock.Add(time.Minute)
	stamp := commitClock.Format(time.RFC3339)
	t.Setenv("GIT_AUTHOR_DATE", stamp)
	t.Setenv("GIT_COMMITTER_DATE", stamp)
	run(t, dir, "commit", "-q", "-m", message)
}

// fixture ist ein Repo mit Remote, wie es die Liste vorfindet.
type fixture struct {
	root   string
	origin string
	seed   string
	work   string
}

// newFixture baut:
//
//	main           2 Commits, ausgecheckt in work
//	development    main + 1, lokal mit Upstream
//	stage          = main, nur remote
//	feature/a      = erster Commit von main (gemergt), lokal, in einem zweiten Worktree
//	feature/b      main + 1, lokal mit Upstream
//	remediation/x  main + 1, nur remote
//	hotfix-y       main + 1, nur remote
//	old/gone       lokal, sein Upstream ist auf dem Remote gelöscht
func newFixture(t *testing.T) fixture {
	t.Helper()
	isolateGit(t)
	root := t.TempDir()
	f := fixture{
		root:   root,
		origin: filepath.Join(root, "origin.git"),
		seed:   filepath.Join(root, "seed"),
		work:   filepath.Join(root, "work"),
	}
	run(t, root, "init", "-q", "--bare", "-b", "main", f.origin)
	run(t, root, "init", "-q", "-b", "main", f.seed)
	commit(t, f.seed, "README.md", "eins\n", "erster Commit")
	first := run(t, f.seed, "rev-parse", "HEAD")
	commit(t, f.seed, "README.md", "zwei\n", "zweiter Commit")

	run(t, f.seed, "branch", "feature/a", first)
	run(t, f.seed, "branch", "stage")
	for _, branch := range []string{"development", "feature/b", "remediation/x", "hotfix-y", "old/gone"} {
		run(t, f.seed, "switch", "-q", "-c", branch, "main")
		commit(t, f.seed, strings.ReplaceAll(branch, "/", "-")+".txt", branch+"\n", "Arbeit an "+branch)
	}
	run(t, f.seed, "switch", "-q", "main")
	run(t, f.seed, "remote", "add", "origin", f.origin)
	run(t, f.seed, "push", "-q", "origin", "--all")

	run(t, root, "clone", "-q", f.origin, f.work)
	for _, branch := range []string{"development", "feature/b", "old/gone", "feature/a"} {
		run(t, f.work, "branch", "--track", branch, "origin/"+branch)
	}
	run(t, f.seed, "push", "-q", "origin", "--delete", "old/gone")
	run(t, f.work, "fetch", "-q", "--prune")
	return f
}

// options sind die Vorgaben für eine Abfrage im Arbeitsrepo der Fixture.
func (f fixture) options() Options {
	return Options{ProjectDir: f.work, RepoDir: f.work}
}

// versionRunner meldet eine andere git-Fassung und reicht alles andere an das
// echte git weiter. So lässt sich der Weg für git vor 2.41 prüfen, ohne ein
// altes git zu installieren.
type versionRunner struct {
	version string
}

func (v versionRunner) Run(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	if name == "git" && len(args) == 1 && args[0] == "version" {
		return []byte("git version " + v.version + "\n"), nil
	}
	return github.ExecRunner{}.Run(ctx, dir, name, args...)
}

// recordingRunner hält jeden Aufruf fest und reicht ihn an das echte git weiter.
type recordingRunner struct {
	calls []string
}

func (r *recordingRunner) Run(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, name+" "+strings.Join(args, " "))
	return github.ExecRunner{}.Run(ctx, dir, name, args...)
}

func findBranch(t *testing.T, listing Listing, name string) (Branch, Group) {
	t.Helper()
	for _, group := range listing.Groups {
		for _, branch := range group.Branches {
			if branch.Name == name {
				return branch, group
			}
		}
	}
	t.Fatalf("Branch %s fehlt in der Liste", name)
	return Branch{}, Group{}
}

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	return ctx
}
