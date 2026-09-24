package branches

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kascada/k-playbook/installer/internal/github"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// Die Ordnung: aktueller Branch, langlebige mit Umgebung, dann Arbeitsbranches
// nach Präfix — alphabetisch, ohne Präfix zuletzt.
func TestListOrdnetBranches(t *testing.T) {
	f := newFixture(t)
	listing := List(testContext(t), f.options())
	if listing.State != StateOK {
		t.Fatalf("State = %q: %s", listing.State, listing.Message)
	}

	var titles []string
	for _, group := range listing.Groups {
		var names []string
		for _, branch := range group.Branches {
			names = append(names, branch.Name)
		}
		titles = append(titles, group.Title+"="+strings.Join(names, ","))
	}
	// Innerhalb einer Präfixgruppe gilt der jüngste Commit zuerst; feature/a
	// steht auf dem ältesten Commit und gehört deshalb nach hinten.
	want := []string{
		"Aktueller Branch=main",
		"Langlebige Branches=development,stage",
		"feature/=feature/b,feature/a",
		"old/=old/gone",
		"remediation/=remediation/x",
		"Ohne Präfix=hotfix-y",
	}
	if strings.Join(titles, " | ") != strings.Join(want, " | ") {
		t.Errorf("Gruppen:\n  %s\nerwartet:\n  %s", strings.Join(titles, "\n  "), strings.Join(want, "\n  "))
	}

	if listing.Default.State != FieldKnown || listing.Default.Name != "main" || listing.Default.Source != "remote-head" {
		t.Errorf("Default = %+v", listing.Default)
	}

	development, _ := findBranch(t, listing, "development")
	if !development.LongLived || len(development.Environments) != 1 || development.Environments[0].Name != "dev" ||
		!development.Environments[0].Suggested || development.Environments[0].Source != SourceName {
		t.Errorf("development = %+v, erwartet Umgebung dev als Namensvorschlag", development)
	}
	stage, _ := findBranch(t, listing, "stage")
	if !stage.RemoteOnly || stage.Remote != "origin" {
		t.Errorf("stage = %+v, erwartet nur remote", stage)
	}
	main, _ := findBranch(t, listing, "main")
	if !main.Current || !main.Default || main.Merged.State != FieldSelf {
		t.Errorf("main = %+v", main)
	}
	// main bedient als Default-Branch namentlich prod.
	if len(main.Environments) != 1 || main.Environments[0].Name != "prod" {
		t.Errorf("main.Environments = %+v", main.Environments)
	}
}

// gemergt, Abstand zum Default-Branch und Upstream mit [gone].
func TestListGemergtGoneUndAbstand(t *testing.T) {
	f := newFixture(t)
	listing := List(testContext(t), f.options())
	if !listing.Git.AheadBehind {
		t.Skipf("git %s kennt %%(ahead-behind) nicht", listing.Git.Version)
	}

	featureA, _ := findBranch(t, listing, "feature/a")
	if featureA.Merged.State != FieldKnown || !featureA.Merged.Merged {
		t.Errorf("feature/a.Merged = %+v, erwartet gemergt", featureA.Merged)
	}
	if featureA.ToDefault.State != FieldKnown || featureA.ToDefault.Ahead != 0 || featureA.ToDefault.Behind != 1 {
		t.Errorf("feature/a.ToDefault = %+v", featureA.ToDefault)
	}
	featureB, _ := findBranch(t, listing, "feature/b")
	if featureB.Merged.Merged || featureB.ToDefault.Ahead != 1 || featureB.ToDefault.Behind != 0 {
		t.Errorf("feature/b = %+v / %+v", featureB.Merged, featureB.ToDefault)
	}
	if featureB.Upstream == nil || featureB.Upstream.Name != "origin/feature/b" || featureB.Upstream.Gone {
		t.Errorf("feature/b.Upstream = %+v", featureB.Upstream)
	}
	gone, _ := findBranch(t, listing, "old/gone")
	if gone.Upstream == nil || !gone.Upstream.Gone {
		t.Errorf("old/gone.Upstream = %+v, erwartet gone", gone.Upstream)
	}
	if featureB.Commit.Subject != "Arbeit an feature/b" || featureB.Commit.Author != "Test" || featureB.Commit.Date == "" {
		t.Errorf("feature/b.Commit = %+v", featureB.Commit)
	}
}

// Worktrees samt prunable, und ein Branch, der in einem anderen Worktree
// ausgecheckt ist, trägt ihn.
func TestListWorktrees(t *testing.T) {
	f := newFixture(t)
	other := filepath.Join(f.root, "zweiter")
	run(t, f.work, "worktree", "add", "-q", other, "feature/a")
	if err := os.RemoveAll(other); err != nil {
		t.Fatal(err)
	}

	listing := List(testContext(t), f.options())
	if len(listing.Worktrees) != 2 {
		t.Fatalf("Worktrees = %+v", listing.Worktrees)
	}
	if !listing.Worktrees[0].Current || listing.Worktrees[0].Branch != "main" {
		t.Errorf("erster Worktree = %+v", listing.Worktrees[0])
	}
	second := listing.Worktrees[1]
	if !second.Prunable || second.Branch != "feature/a" || second.PrunableReason == "" {
		t.Errorf("zweiter Worktree = %+v, erwartet prunable", second)
	}
	featureA, _ := findBranch(t, listing, "feature/a")
	if featureA.Worktree == nil || !featureA.Worktree.Prunable {
		t.Errorf("feature/a.Worktree = %+v", featureA.Worktree)
	}
}

// Ohne Remote gibt es kein refs/remotes/<remote>/HEAD: Abstand und „gemergt"
// sind unbekannt mit Grund, nie leer.
func TestListDefaultBranchUnbekannt(t *testing.T) {
	isolateGit(t)
	dir := t.TempDir()
	run(t, dir, "init", "-q", "-b", "main", dir)
	commit(t, dir, "a.txt", "a\n", "a")
	run(t, dir, "switch", "-q", "-c", "feature/x")
	commit(t, dir, "b.txt", "b\n", "b")

	listing := List(testContext(t), Options{ProjectDir: dir, RepoDir: dir})
	if listing.State != StateOK {
		t.Fatalf("State = %q: %s", listing.State, listing.Message)
	}
	if listing.Default.State != FieldUnknown || listing.Default.Reason == "" {
		t.Errorf("Default = %+v, erwartet unbekannt mit Grund", listing.Default)
	}
	main, _ := findBranch(t, listing, "main")
	if main.ToDefault.State != FieldUnknown || main.ToDefault.Reason == "" || main.Merged.State != FieldUnknown || main.Merged.Reason == "" {
		t.Errorf("main = %+v / %+v", main.ToDefault, main.Merged)
	}

	// Mit Remote, aber ohne origin/HEAD, nennt der Grund den Weg dorthin.
	origin := filepath.Join(t.TempDir(), "origin.git")
	run(t, dir, "init", "-q", "--bare", origin)
	run(t, dir, "remote", "add", "origin", origin)
	run(t, dir, "push", "-q", "origin", "main")
	run(t, dir, "fetch", "-q", "origin")
	// Seit git 2.48 legt fetch refs/remotes/origin/HEAD selbst an.
	run(t, dir, "remote", "set-head", "origin", "-d")
	listing = List(testContext(t), Options{ProjectDir: dir, RepoDir: dir})
	if listing.Default.State != FieldUnknown || !strings.Contains(listing.Default.Reason, "refs/remotes/origin/HEAD") {
		t.Errorf("Default = %+v", listing.Default)
	}
}

// git vor 2.41 kennt %(ahead-behind) nicht. Dann ist der Abstand unbekannt mit
// Grund, und „gemergt" kommt über --merged.
func TestListOhneAheadBehind(t *testing.T) {
	f := newFixture(t)
	options := f.options()
	options.Runner = versionRunner{version: "2.40.1"}
	listing := List(testContext(t), options)
	if listing.State != StateOK {
		t.Fatalf("State = %q: %s", listing.State, listing.Message)
	}
	if listing.Git.AheadBehind || !strings.Contains(listing.Git.Reason, "2.41") {
		t.Errorf("Git = %+v", listing.Git)
	}
	featureA, _ := findBranch(t, listing, "feature/a")
	if featureA.ToDefault.State != FieldUnknown || !strings.Contains(featureA.ToDefault.Reason, "2.41") {
		t.Errorf("feature/a.ToDefault = %+v", featureA.ToDefault)
	}
	if featureA.Merged.State != FieldKnown || !featureA.Merged.Merged {
		t.Errorf("feature/a.Merged = %+v, erwartet gemergt über --merged", featureA.Merged)
	}
	featureB, _ := findBranch(t, listing, "feature/b")
	if featureB.Merged.State != FieldKnown || featureB.Merged.Merged {
		t.Errorf("feature/b.Merged = %+v", featureB.Merged)
	}
}

// Das Alter des letzten Fetch kommt aus FETCH_HEAD; ohne Fetch heißt es never.
func TestListFetchAlter(t *testing.T) {
	f := newFixture(t)
	options := f.options()
	options.Now = func() time.Time { return time.Now().Add(2 * time.Hour) }
	listing := List(testContext(t), options)
	if listing.Fetch.State != FieldKnown || listing.Fetch.AgeSeconds < 7000 {
		t.Errorf("Fetch = %+v", listing.Fetch)
	}

	isolateGit(t)
	dir := t.TempDir()
	run(t, dir, "init", "-q", dir)
	commit(t, dir, "a.txt", "a\n", "a")
	listing = List(testContext(t), Options{ProjectDir: dir, RepoDir: dir})
	if listing.Fetch.State != "never" || listing.Fetch.Message == "" {
		t.Errorf("Fetch ohne FETCH_HEAD = %+v", listing.Fetch)
	}
}

// Die Liste liest nur: kein fetch, kein switch, und jeder Aufruf mit
// --no-optional-locks, damit git status nicht nebenbei den Index schreibt.
func TestListLiestNur(t *testing.T) {
	f := newFixture(t)
	recorder := &recordingRunner{}
	options := f.options()
	options.Runner = recorder
	List(testContext(t), options)
	for _, call := range recorder.calls {
		if call == "git version" {
			continue
		}
		if !strings.HasPrefix(call, "git --no-optional-locks ") {
			t.Errorf("Aufruf ohne --no-optional-locks: %s", call)
		}
		for _, forbidden := range []string{" fetch", " switch", " checkout", " pull", " worktree prune"} {
			if strings.Contains(call, forbidden) {
				t.Errorf("verändernder Aufruf in der Liste: %s", call)
			}
		}
	}
}

// Kein Repo ist ein Zustand mit Satz.
func TestListOhneRepo(t *testing.T) {
	isolateGit(t)
	dir := t.TempDir()
	listing := List(testContext(t), Options{ProjectDir: dir, RepoDir: dir})
	if listing.State != StateNoRepo || listing.Message == "" {
		t.Errorf("State = %q, Message = %q", listing.State, listing.Message)
	}
}

// Ist Umschalten freigegeben, trägt jeder Branch, ob git.allow ihn zulässt.
func TestListFreigabeJeBranch(t *testing.T) {
	f := newFixture(t)
	options := f.options()
	options.Settings = project.GitSettings{Switch: project.GitSwitchOffer, Allow: []string{"feature/*"}, Configured: true}
	listing := List(testContext(t), options)
	if !listing.Switch.Offered {
		t.Errorf("Switch = %+v", listing.Switch)
	}
	featureB, _ := findBranch(t, listing, "feature/b")
	stage, _ := findBranch(t, listing, "stage")
	if !featureB.Allowed || stage.Allowed {
		t.Errorf("Allowed: feature/b=%v stage=%v", featureB.Allowed, stage.Allowed)
	}

	options.Settings = project.GitSettings{Switch: project.GitSwitchUnknown}
	listing = List(testContext(t), options)
	if listing.Switch.Offered || !strings.Contains(listing.Switch.Message, "git.switch") {
		t.Errorf("Switch ohne Entscheidung = %+v", listing.Switch)
	}
}

// Mit gh: Default-Branch von GitHub und der offene PR je Branch. Ohne
// Enabled bleiben die Daten draußen, auch wenn sie mitgegeben wurden.
func TestListMitGitHubDaten(t *testing.T) {
	f := newFixture(t)
	data := GitHubData{
		Enabled:       true,
		Result:        github.Result{State: github.StateOK},
		DefaultBranch: "development",
		Pulls: []github.PullRequest{
			{Number: 7, Title: "Feature B", Head: "feature/b", Base: "main", State: "open"},
			{Number: 8, Title: "Fork", Head: "feature/a", Base: "main", State: "open", Fork: true},
		},
	}
	options := f.options()
	options.GitHub = data
	listing := List(testContext(t), options)
	if listing.Default.Name != "development" || listing.Default.Source != "github" || listing.Default.Ref != "refs/remotes/origin/development" {
		t.Errorf("Default = %+v", listing.Default)
	}
	featureB, _ := findBranch(t, listing, "feature/b")
	if featureB.Pull == nil || featureB.Pull.Number != 7 {
		t.Errorf("feature/b.Pull = %+v", featureB.Pull)
	}
	featureA, _ := findBranch(t, listing, "feature/a")
	if featureA.Pull != nil {
		t.Errorf("ein PR aus einem Fork hängt an feature/a: %+v", featureA.Pull)
	}

	data.Enabled = false
	options.GitHub = data
	listing = List(testContext(t), options)
	if listing.Default.Source != "remote-head" {
		t.Errorf("ohne gh kommt der Default-Branch von GitHub: %+v", listing.Default)
	}
	featureB, _ = findBranch(t, listing, "feature/b")
	if featureB.Pull != nil {
		t.Errorf("ohne gh hängt ein PR an feature/b: %+v", featureB.Pull)
	}
}

func TestParseWorktrees(t *testing.T) {
	output := "worktree /a\nHEAD 1111\nbranch refs/heads/main\n\nworktree /b\nHEAD 2222\ndetached\nlocked grund\n\nworktree /c\nHEAD 3333\nbranch refs/heads/x\nprunable gitdir file points to non-existent location\n"
	worktrees := ParseWorktrees(output)
	if len(worktrees) != 3 || worktrees[0].Branch != "main" || !worktrees[1].Detached || worktrees[1].LockedReason != "grund" ||
		!worktrees[2].Prunable || worktrees[2].PrunableReason != "gitdir file points to non-existent location" {
		t.Errorf("ParseWorktrees = %+v", worktrees)
	}
}

func TestParseVersion(t *testing.T) {
	for input, want := range map[string][2]int{
		"git version 2.53.0":                 {2, 53},
		"git version 2.39.3 (Apple Git-146)": {2, 39},
		"git version 2.41.0.windows.1":       {2, 41},
	} {
		version, ok := ParseVersion(input)
		if !ok || version.Major != want[0] || version.Minor != want[1] {
			t.Errorf("ParseVersion(%q) = %+v", input, version)
		}
	}
	if (Version{Major: 2, Minor: 40}).AtLeast(2, 41) || !(Version{Major: 3, Minor: 0}).AtLeast(2, 41) {
		t.Error("AtLeast rechnet falsch")
	}
}
