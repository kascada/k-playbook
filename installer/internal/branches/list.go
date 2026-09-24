package branches

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kascada/k-playbook/installer/internal/github"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// Zustände einer Antwort. Wie bei der GitHub-Ansicht feste Werte, zu denen
// immer ein Satz gehört.
const (
	StateOK        = "ok"
	StateNoProject = "no-project"
	StateNoVCS     = "no-vcs"
	StateNoRepo    = "no-repo"
	StateNoGit     = "no-git"
	StateTimeout   = "timeout"
	StateError     = "error"
)

// Zustände eines einzelnen Feldes, das auch unbekannt sein kann. Ein Feld ist
// nie still leer: unknown trägt immer einen Grund.
const (
	FieldKnown   = "ok"
	FieldUnknown = "unknown"
	// FieldSelf steht beim Default-Branch selbst: Abstand und „gemergt" zu sich
	// selbst sind keine Aussage.
	FieldSelf = "self"
)

// Options ist alles, was eine Abfrage braucht. Gesucht wird nichts selbst.
type Options struct {
	ProjectDir string
	RepoDir    string
	Settings   project.GitSettings
	GitHub     GitHubData
	Runner     github.Runner
	// Now ist ausgelagert, damit Tests das Alter des letzten Fetch prüfen können.
	Now func() time.Time
}

// GitHubData sind die GitHub-Daten, die der Aufrufer beschafft hat. Enabled ist
// nur wahr bei tools.gh.status = enabled und bereitem gh; sonst bleiben alle
// Felder leer, auch wenn gh installiert ist.
type GitHubData struct {
	Enabled bool `json:"enabled"`
	// Result ist der Zustand der Vorprüfung oder des ersten Aufrufs.
	github.Result
	Repo          string `json:"repo"`
	DefaultBranch string `json:"defaultBranch"`
	// Notes sagt je Teilabfrage, ob sie gelesen wurde.
	Notes        GitHubNotes          `json:"notes"`
	Pulls        []github.PullRequest `json:"-"`
	Environments []string             `json:"environments"`
	Deployments  []github.Deployment  `json:"deployments"`
}

// GitHubNotes sind die Zustände der Teilabfragen.
type GitHubNotes struct {
	Pulls        github.Result `json:"pulls"`
	Environments github.Result `json:"environments"`
	Deployments  github.Result `json:"deployments"`
}

// Listing ist die Antwort von GET /api/branches.
type Listing struct {
	State   string `json:"state"`
	Message string `json:"message"`

	ProjectDir string `json:"projectDir"`
	RepoDir    string `json:"repoDir"`
	// ConfigInRepo meldet, dass K-PLAYBOOK.yaml im umgeschalteten Repo liegt
	// (repo_root: .) und sich damit zwischen Branches unterscheiden kann.
	ConfigInRepo bool `json:"configInRepo"`

	Git     GitInfo      `json:"git"`
	Switch  SwitchPolicy `json:"switch"`
	Head    Head         `json:"head"`
	Remote  string       `json:"remote"`
	Default Default      `json:"defaultBranch"`
	Fetch   FetchInfo    `json:"fetch"`
	GitHub  GitHubData   `json:"github"`

	Groups       []Group       `json:"groups"`
	Worktrees    []Worktree    `json:"worktrees"`
	Environments []Environment `json:"environments"`
}

// GitInfo ist die Fassung von git und was sie kann.
type GitInfo struct {
	Version string `json:"version"`
	// AheadBehind meldet, ob %(ahead-behind:…) genutzt wurde. Ohne das Feld
	// steht der Abstand zum Default-Branch als unbekannt mit Grund da.
	AheadBehind bool   `json:"aheadBehind"`
	Reason      string `json:"reason,omitempty"`
}

// SwitchPolicy ist die Projektentscheidung zum Umschalten, wie die Seite sie
// braucht.
type SwitchPolicy struct {
	Status     project.GitSwitch `json:"status"`
	Configured bool              `json:"configured"`
	Allow      []string          `json:"allow"`
	// Offered meldet, ob überhaupt ein Wechsel angeboten wird.
	Offered bool   `json:"offered"`
	Message string `json:"message"`
}

// Head ist der ausgecheckte Stand.
type Head struct {
	Branch   string `json:"branch"`
	Detached bool   `json:"detached"`
	SHA      string `json:"sha"`
	// Unborn: der Branch hat noch keinen Commit.
	Unborn bool `json:"unborn"`
}

// Default ist der Default-Branch mit seiner Quelle.
type Default struct {
	State string `json:"state"`
	Name  string `json:"name"`
	// Ref ist der Ref, gegen den gerechnet wird: der Remote-Tracking-Branch,
	// wenn es ihn gibt, sonst der lokale.
	Ref string `json:"ref"`
	// Source ist github oder remote-head.
	Source string `json:"source"`
	Reason string `json:"reason,omitempty"`
}

// FetchInfo ist das Alter des letzten Fetch.
type FetchInfo struct {
	// State ist ok, never oder unknown.
	State string `json:"state"`
	At    string `json:"at,omitempty"`
	// AgeSeconds ist das Alter beim Lesen.
	AgeSeconds int64  `json:"ageSeconds,omitempty"`
	Message    string `json:"message,omitempty"`
}

// Group ist ein Abschnitt der geordneten Liste.
type Group struct {
	// Key ist current, longlived oder work.
	Key   string `json:"key"`
	Title string `json:"title"`
	// Prefix ist bei Arbeitsbranches der Teil bis zum ersten /.
	Prefix   string   `json:"prefix,omitempty"`
	Branches []Branch `json:"branches"`
}

// Branch ist ein Eintrag der Liste.
type Branch struct {
	Name string `json:"name"`
	Ref  string `json:"ref"`
	// RemoteOnly: es gibt nur den Remote-Tracking-Branch; ein Wechsel legt den
	// lokalen Branch als Tracking-Branch an.
	RemoteOnly bool   `json:"remoteOnly"`
	Remote     string `json:"remote,omitempty"`
	Current    bool   `json:"current"`
	LongLived  bool   `json:"longLived"`
	Default    bool   `json:"default"`

	Upstream *Upstream `json:"upstream,omitempty"`
	Commit   Commit    `json:"commit"`

	// ToDefault ist der Abstand zum Default-Branch.
	ToDefault Distance `json:"toDefault"`
	// Merged sagt, ob der Branch im Default-Branch enthalten ist.
	Merged Merged `json:"merged"`

	Environments []EnvironmentLabel `json:"environments"`
	// Worktree ist gesetzt, wenn der Branch in einem anderen Worktree
	// ausgecheckt ist.
	Worktree *Worktree `json:"worktree,omitempty"`
	Pull     *Pull     `json:"pull,omitempty"`
	// Allowed meldet, ob git.allow den Branch als Ziel zulässt.
	Allowed bool `json:"allowed"`
}

// Upstream ist der Upstream eines lokalen Branches.
type Upstream struct {
	Name   string `json:"name"`
	Gone   bool   `json:"gone"`
	Ahead  int    `json:"ahead"`
	Behind int    `json:"behind"`
}

// Commit ist der letzte Commit eines Branches.
type Commit struct {
	SHA     string `json:"sha"`
	Date    string `json:"date"`
	Subject string `json:"subject"`
	Author  string `json:"author"`
}

// Distance ist ein Abstand, der auch unbekannt sein kann.
type Distance struct {
	State  string `json:"state"`
	Ahead  int    `json:"ahead"`
	Behind int    `json:"behind"`
	Reason string `json:"reason,omitempty"`
}

// Merged ist die Antwort auf „bereits gemergt".
type Merged struct {
	State  string `json:"state"`
	Merged bool   `json:"merged"`
	Reason string `json:"reason,omitempty"`
}

// EnvironmentLabel ist die Umgebung an einem Branch der Liste.
type EnvironmentLabel struct {
	Name      string `json:"name"`
	Source    string `json:"source"`
	Suggested bool   `json:"suggested"`
}

// Pull ist der offene PR eines Branches.
type Pull struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	URL    string `json:"url"`
	State  string `json:"state"`
	Base   string `json:"base"`
}

// Worktree ist ein Eintrag aus `git worktree list --porcelain`.
type Worktree struct {
	Path     string `json:"path"`
	HEAD     string `json:"head"`
	Branch   string `json:"branch,omitempty"`
	Detached bool   `json:"detached"`
	Bare     bool   `json:"bare"`
	Locked   bool   `json:"locked"`
	// LockedReason und PrunableReason sind die Begründungen, die git nennt.
	LockedReason   string `json:"lockedReason,omitempty"`
	Prunable       bool   `json:"prunable"`
	PrunableReason string `json:"prunableReason,omitempty"`
	// Current ist der Worktree, in dem die Oberfläche arbeitet.
	Current bool `json:"current"`
}

// refEntry ist eine Zeile aus for-each-ref.
type refEntry struct {
	ref         string
	sha         string
	upstream    string
	track       string
	date        string
	author      string
	subject     string
	head        bool
	worktree    string
	symref      string
	aheadBehind string
}

// state ist der Zwischenstand einer Abfrage, den Liste, Umgebungen und
// Vorprüfung teilen.
type state struct {
	repo      Repo
	options   Options
	version   Version
	head      Head
	remotes   []string
	remote    string
	def       Default
	locals    map[string]refEntry
	remoteRef map[string]map[string]refEntry // Name → Remote → Eintrag
	merged    map[string]bool
	worktrees []Worktree
	listing   Listing
}

// List liest die Branches des Code-Repos, geordnet und mit ihren Umgebungen.
func List(ctx context.Context, options Options) Listing {
	st, ok := load(ctx, options)
	if !ok {
		return st.listing
	}
	return st.build(ctx)
}

// load liest alles Lokale. Scheitert ein tragender Aufruf, ist die Antwort ein
// Zustand mit Satz und sonst nichts.
func load(ctx context.Context, options Options) (*state, bool) {
	st := &state{
		repo:    Repo{Runner: options.Runner, Dir: options.RepoDir},
		options: options,
		listing: Listing{
			ProjectDir:   options.ProjectDir,
			RepoDir:      options.RepoDir,
			ConfigInRepo: configInRepo(options.ProjectDir, options.RepoDir),
			Switch:       switchPolicy(options.Settings),
			GitHub:       options.GitHub,
			Groups:       []Group{},
			Worktrees:    []Worktree{},
			Environments: []Environment{},
		},
	}
	if st.listing.GitHub.Environments == nil {
		st.listing.GitHub.Environments = []string{}
	}
	if st.listing.GitHub.Deployments == nil {
		st.listing.GitHub.Deployments = []github.Deployment{}
	}

	version, err := st.repo.GitVersion(ctx)
	if err != nil {
		st.fail(StateNoGit, "git ist nicht ausführbar: "+firstLine(stderrOf(err)))
		if timedOut(err) {
			st.fail(StateTimeout, "git hat nicht rechtzeitig geantwortet.")
		}
		return st, false
	}
	st.version = version
	st.listing.Git.Version = version.String()

	if _, err := st.repo.readString(ctx, "rev-parse", "--git-dir"); err != nil {
		if timedOut(err) {
			st.fail(StateTimeout, "git hat nicht rechtzeitig geantwortet.")
		} else {
			st.fail(StateNoRepo, "Das Code-Repo "+options.RepoDir+" ist kein git-Repository: "+firstLine(stderrOf(err)))
		}
		return st, false
	}

	if err := st.readHead(ctx); err != nil {
		st.failErr("Der ausgecheckte Stand", err)
		return st, false
	}
	if err := st.readRemotes(ctx); err != nil {
		st.failErr("Die Remotes", err)
		return st, false
	}
	st.readDefault(ctx)
	if err := st.readRefs(ctx); err != nil {
		st.failErr("Die Branches", err)
		return st, false
	}
	if err := st.readWorktrees(ctx); err != nil {
		st.failErr("Die Worktrees", err)
		return st, false
	}
	st.readFetch(ctx)
	st.listing.State = StateOK
	st.listing.Head = st.head
	st.listing.Remote = st.remote
	st.listing.Default = st.def
	st.listing.Worktrees = st.worktrees
	return st, true
}

func (st *state) fail(state string, message string) {
	st.listing.State = state
	st.listing.Message = message
}

func (st *state) failErr(subject string, err error) {
	if timedOut(err) {
		st.fail(StateTimeout, subject+": git hat nicht rechtzeitig geantwortet.")
		return
	}
	st.fail(StateError, describeError(subject, err))
}

// configInRepo meldet, ob die Konfiguration im Code-Repo liegt.
func configInRepo(projectDir string, repoDir string) bool {
	if projectDir == "" || repoDir == "" {
		return false
	}
	relative, err := filepath.Rel(repoDir, projectDir)
	if err != nil {
		return false
	}
	return relative == "." || (!strings.HasPrefix(relative, "..") && !filepath.IsAbs(relative))
}

func switchPolicy(settings project.GitSettings) SwitchPolicy {
	policy := SwitchPolicy{
		Status:     settings.Switch,
		Configured: settings.Configured,
		Allow:      append([]string{}, settings.Allow...),
		Offered:    settings.Switch == project.GitSwitchOffer,
	}
	switch settings.Switch {
	case project.GitSwitchOffer:
		if len(settings.Allow) == 0 {
			policy.Message = "Umschalten wird angeboten, auf jeden Branch (git.allow ist leer)."
		} else {
			policy.Message = "Umschalten wird angeboten, auf Branches aus git.allow: " + strings.Join(settings.Allow, ", ") + "."
		}
	case project.GitSwitchOff:
		policy.Message = "Umschalten aus der Oberfläche ist für dieses Projekt abgeschaltet (git.switch: off in " + project.ConfigFileName + ")."
	default:
		policy.Message = "Ob die Oberfläche umschalten darf, ist nicht entschieden. Die Entscheidung steht in " + project.ConfigFileName + " unter git.switch (offer oder off); bis dahin zeigt die Seite nur die Liste."
	}
	return policy
}

func (st *state) readHead(ctx context.Context) error {
	branch, err := st.repo.readString(ctx, "symbolic-ref", "-q", "--short", "HEAD")
	switch {
	case err == nil:
		st.head.Branch = branch
	case exitCode(err) == 1:
		st.head.Detached = true
	default:
		return err
	}
	sha, err := st.repo.readString(ctx, "rev-parse", "-q", "--verify", "HEAD^{commit}")
	switch {
	case err == nil:
		st.head.SHA = sha
	case exitCode(err) == 1:
		st.head.Unborn = true
	default:
		return err
	}
	return nil
}

func (st *state) readRemotes(ctx context.Context) error {
	out, err := st.repo.readString(ctx, "remote")
	if err != nil {
		return err
	}
	for _, line := range strings.Split(out, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			st.remotes = append(st.remotes, line)
		}
	}
	// Maßgeblich ist der Remote des aktuellen Branches, sonst origin, sonst der
	// erste. Er bestimmt refs/remotes/<remote>/HEAD und die Refs, gegen die
	// gerechnet wird.
	if st.head.Branch != "" {
		if name, err := st.repo.readString(ctx, "config", "--get", "branch."+st.head.Branch+".remote"); err == nil && contains(st.remotes, name) {
			st.remote = name
			return nil
		}
	}
	if contains(st.remotes, "origin") {
		st.remote = "origin"
	} else if len(st.remotes) > 0 {
		st.remote = st.remotes[0]
	}
	return nil
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

// readDefault bestimmt den Default-Branch: mit gh den von GitHub, sonst
// refs/remotes/<remote>/HEAD. Fehlt er, ist er unbekannt mit Grund.
func (st *state) readDefault(ctx context.Context) {
	gh := st.options.GitHub
	if gh.Enabled && gh.DefaultBranch != "" {
		st.def = Default{Name: gh.DefaultBranch, Source: "github"}
		st.resolveDefaultRef(ctx)
		return
	}

	if st.remote == "" {
		st.def = Default{State: FieldUnknown, Reason: "Das Repo hat kein Remote; ohne refs/remotes/<remote>/HEAD ist kein Default-Branch bekannt."}
		st.addGitHubReason()
		return
	}
	target, err := st.repo.readString(ctx, "symbolic-ref", "-q", "refs/remotes/"+st.remote+"/HEAD")
	if err != nil || target == "" {
		reason := "refs/remotes/" + st.remote + "/HEAD fehlt. Gesetzt wird er beim Klonen oder mit „git remote set-head " + st.remote + " --auto“."
		if err != nil && exitCode(err) != 1 {
			reason = describeError("refs/remotes/"+st.remote+"/HEAD", err)
		}
		st.def = Default{State: FieldUnknown, Reason: reason}
		st.addGitHubReason()
		return
	}
	st.def = Default{
		State:  FieldKnown,
		Name:   strings.TrimPrefix(target, "refs/remotes/"+st.remote+"/"),
		Ref:    target,
		Source: "remote-head",
	}
	st.addGitHubReason()
}

// addGitHubReason nennt, warum GitHub nicht die Quelle ist, obwohl gh genutzt
// wird — sonst sähe der Rückfall aus wie die Absicht.
func (st *state) addGitHubReason() {
	gh := st.options.GitHub
	if !gh.Enabled || gh.State == github.StateOK {
		return
	}
	note := "GitHub lieferte keinen Default-Branch: " + gh.Message
	if st.def.Reason == "" {
		st.def.Reason = note
	} else {
		st.def.Reason += " " + note
	}
}

func (st *state) resolveDefaultRef(ctx context.Context) {
	candidates := []string{}
	if st.remote != "" {
		candidates = append(candidates, "refs/remotes/"+st.remote+"/"+st.def.Name)
	}
	candidates = append(candidates, "refs/heads/"+st.def.Name)
	for _, ref := range candidates {
		if _, err := st.repo.readString(ctx, "rev-parse", "-q", "--verify", ref+"^{commit}"); err == nil {
			st.def.State = FieldKnown
			st.def.Ref = ref
			return
		}
	}
	st.def.State = FieldUnknown
	st.def.Reason = "Der Default-Branch " + st.def.Name + " von GitHub ist lokal weder als Branch noch als Remote-Tracking-Branch vorhanden; ein Fetch holt ihn."
}

const refFormat = "%(refname)%00%(objectname)%00%(upstream)%00%(upstream:track,nobracket)%00%(committerdate:iso-strict)%00%(authorname)%00%(subject)%00%(HEAD)%00%(worktreepath)%00%(symref)"

// readRefs liest lokale und Remote-Branches in einem for-each-ref-Aufruf. Kann
// git %(ahead-behind:…), kommt der Abstand zum Default-Branch im selben Aufruf
// mit; sonst hängt „gemergt" an einem zweiten Aufruf mit --merged, und der
// Abstand bleibt unbekannt mit Grund.
func (st *state) readRefs(ctx context.Context) error {
	format := refFormat
	useAheadBehind := st.def.State == FieldKnown && st.version.AtLeast(aheadBehindMajor, aheadBehindMinor)
	if useAheadBehind {
		format += "%00%(ahead-behind:" + st.def.Ref + ")"
	}
	switch {
	case useAheadBehind:
		st.listing.Git.AheadBehind = true
	case st.def.State != FieldKnown:
		st.listing.Git.Reason = "Ohne bekannten Default-Branch gibt es keinen Abstand dazu."
	default:
		st.listing.Git.Reason = "git " + st.version.String() + " kennt %(ahead-behind:…) noch nicht (ab 2.41); der Abstand zum Default-Branch bleibt unbekannt, „gemergt“ wird über --merged bestimmt."
	}

	out, err := st.repo.read(ctx, "for-each-ref", "--format="+format, "refs/heads", "refs/remotes")
	if err != nil {
		return err
	}
	st.locals = map[string]refEntry{}
	st.remoteRef = map[string]map[string]refEntry{}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, "\x00")
		if len(fields) < 10 {
			continue
		}
		entry := refEntry{
			ref: fields[0], sha: fields[1], upstream: fields[2], track: fields[3], date: fields[4],
			author: fields[5], subject: fields[6], head: strings.TrimSpace(fields[7]) == "*",
			worktree: fields[8], symref: fields[9],
		}
		if len(fields) > 10 {
			entry.aheadBehind = fields[10]
		}
		if entry.symref != "" {
			continue
		}
		switch {
		case strings.HasPrefix(entry.ref, "refs/heads/"):
			st.locals[strings.TrimPrefix(entry.ref, "refs/heads/")] = entry
		case strings.HasPrefix(entry.ref, "refs/remotes/"):
			rest := strings.TrimPrefix(entry.ref, "refs/remotes/")
			remote, name, found := st.splitRemoteRef(rest)
			if !found {
				continue
			}
			if st.remoteRef[name] == nil {
				st.remoteRef[name] = map[string]refEntry{}
			}
			st.remoteRef[name][remote] = entry
		}
	}

	if st.def.State == FieldKnown && !useAheadBehind {
		out, err := st.repo.read(ctx, "for-each-ref", "--merged="+st.def.Ref, "--format=%(refname)", "refs/heads", "refs/remotes")
		if err != nil {
			st.listing.Git.Reason += " " + describeError("Die Frage „gemergt“", err)
		} else {
			st.merged = map[string]bool{}
			for _, line := range strings.Split(string(out), "\n") {
				if line = strings.TrimSpace(line); line != "" {
					st.merged[line] = true
				}
			}
		}
	}
	return nil
}

// splitRemoteRef trennt „origin/feature/x" in Remote und Branch. Der Remote ist
// der längste bekannte Remote-Name, der passt — Remote-Namen dürfen selbst /
// enthalten.
func (st *state) splitRemoteRef(rest string) (string, string, bool) {
	best := ""
	for _, remote := range st.remotes {
		if strings.HasPrefix(rest, remote+"/") && len(remote) > len(best) {
			best = remote
		}
	}
	if best == "" {
		remote, name, found := strings.Cut(rest, "/")
		return remote, name, found && name != ""
	}
	return best, strings.TrimPrefix(rest, best+"/"), true
}

func (st *state) readWorktrees(ctx context.Context) error {
	out, err := st.repo.readString(ctx, "worktree", "list", "--porcelain")
	if err != nil {
		return err
	}
	st.worktrees = ParseWorktrees(out)
	own := resolvedPath(st.options.RepoDir)
	top, _ := st.repo.readString(ctx, "rev-parse", "--show-toplevel")
	if top != "" {
		own = resolvedPath(top)
	}
	for index := range st.worktrees {
		if resolvedPath(st.worktrees[index].Path) == own {
			st.worktrees[index].Current = true
		}
	}
	return nil
}

// ParseWorktrees liest die Ausgabe von `git worktree list --porcelain`.
func ParseWorktrees(output string) []Worktree {
	worktrees := []Worktree{}
	var current *Worktree
	flush := func() {
		if current != nil {
			worktrees = append(worktrees, *current)
			current = nil
		}
	}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			flush()
			continue
		}
		key, value, _ := strings.Cut(line, " ")
		if key == "worktree" {
			flush()
			current = &Worktree{Path: value}
			continue
		}
		if current == nil {
			continue
		}
		switch key {
		case "HEAD":
			current.HEAD = value
		case "branch":
			current.Branch = strings.TrimPrefix(value, "refs/heads/")
		case "detached":
			current.Detached = true
		case "bare":
			current.Bare = true
		case "locked":
			current.Locked = true
			current.LockedReason = value
		case "prunable":
			current.Prunable = true
			current.PrunableReason = value
		}
	}
	flush()
	return worktrees
}

func resolvedPath(path string) string {
	if path == "" {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return filepath.Clean(resolved)
	}
	return filepath.Clean(path)
}

// readFetch liest das Alter des letzten Fetch am Zeitstempel von FETCH_HEAD.
func (st *state) readFetch(ctx context.Context) {
	path, err := st.repo.readString(ctx, "rev-parse", "--git-path", "FETCH_HEAD")
	if err != nil {
		st.listing.Fetch = FetchInfo{State: FieldUnknown, Message: describeError("Der Ort von FETCH_HEAD", err)}
		return
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(st.options.RepoDir, path)
	}
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		st.listing.Fetch = FetchInfo{State: "never", Message: "In diesem Repo wurde noch kein Fetch ausgeführt; die Remote-Tracking-Branches stammen vom Klonen."}
		return
	}
	if err != nil {
		st.listing.Fetch = FetchInfo{State: FieldUnknown, Message: "FETCH_HEAD ist nicht lesbar: " + err.Error()}
		return
	}
	now := time.Now
	if st.options.Now != nil {
		now = st.options.Now
	}
	at := info.ModTime()
	st.listing.Fetch = FetchInfo{State: FieldKnown, At: at.Format(time.RFC3339), AgeSeconds: int64(now().Sub(at).Seconds())}
}

// build ordnet die Branches und hängt Umgebungen, Worktrees und PRs an.
func (st *state) build(ctx context.Context) Listing {
	environments := st.environments(ctx)
	st.listing.Environments = environments

	labels := map[string][]EnvironmentLabel{}
	longLived := map[string]bool{}
	if st.def.Name != "" {
		longLived[st.def.Name] = true
	}
	for _, env := range environments {
		for _, candidate := range []string{env.Configured, env.NameSuggestion} {
			if candidate != "" && st.exists(candidate) {
				longLived[candidate] = true
			}
		}
		if env.Branch != "" {
			longLived[env.Branch] = true
			labels[env.Branch] = append(labels[env.Branch], EnvironmentLabel{Name: env.Name, Source: env.Source, Suggested: env.Suggested})
		}
	}

	worktreeOf := map[string]Worktree{}
	for _, worktree := range st.worktrees {
		if worktree.Branch != "" && !worktree.Current {
			worktreeOf[worktree.Branch] = worktree
		}
	}
	pullOf := map[string]github.PullRequest{}
	if st.options.GitHub.Enabled {
		for _, pull := range st.options.GitHub.Pulls {
			if pull.Fork {
				continue
			}
			if _, seen := pullOf[pull.Head]; !seen {
				pullOf[pull.Head] = pull
			}
		}
	}

	var current []Branch
	var long []Branch
	work := map[string][]Branch{}

	add := func(branch Branch) {
		branch.Default = st.def.Name != "" && branch.Name == st.def.Name
		branch.LongLived = longLived[branch.Name]
		branch.Environments = labels[branch.Name]
		if branch.Environments == nil {
			branch.Environments = []EnvironmentLabel{}
		}
		if worktree, ok := worktreeOf[branch.Name]; ok && !branch.RemoteOnly {
			other := worktree
			branch.Worktree = &other
		}
		if pull, ok := pullOf[branch.Name]; ok {
			branch.Pull = &Pull{Number: pull.Number, Title: pull.Title, URL: pull.URL, State: pull.State, Base: pull.Base}
		}
		branch.Allowed, _ = project.BranchAllowed(st.options.Settings.Allow, branch.Name)
		switch {
		case branch.Current:
			current = append(current, branch)
		case branch.LongLived:
			long = append(long, branch)
		default:
			prefix := ""
			if index := strings.Index(branch.Name, "/"); index > 0 {
				prefix = branch.Name[:index]
			}
			work[prefix] = append(work[prefix], branch)
		}
	}

	for name, entry := range st.locals {
		add(st.localBranch(name, entry))
	}
	for name, byRemote := range st.remoteRef {
		if _, local := st.locals[name]; local {
			continue
		}
		for remote, entry := range byRemote {
			add(st.remoteBranch(name, remote, entry))
		}
	}

	groups := []Group{}
	if len(current) > 0 {
		groups = append(groups, Group{Key: "current", Title: "Aktueller Branch", Branches: current})
	}
	if len(long) > 0 {
		order := environmentOrder(environments, st.def.Name)
		sort.SliceStable(long, func(i, j int) bool {
			oi, oj := order(long[i].Name), order(long[j].Name)
			if oi != oj {
				return oi < oj
			}
			if long[i].Name != long[j].Name {
				return long[i].Name < long[j].Name
			}
			return long[i].Remote < long[j].Remote
		})
		groups = append(groups, Group{Key: "longlived", Title: "Langlebige Branches", Branches: long})
	}
	prefixes := make([]string, 0, len(work))
	for prefix := range work {
		prefixes = append(prefixes, prefix)
	}
	// Gruppen alphabetisch, Branches ohne Präfix zuletzt; innerhalb einer Gruppe
	// der jüngste Commit zuerst.
	sort.Slice(prefixes, func(i, j int) bool {
		if (prefixes[i] == "") != (prefixes[j] == "") {
			return prefixes[j] == ""
		}
		return strings.ToLower(prefixes[i]) < strings.ToLower(prefixes[j])
	})
	for _, prefix := range prefixes {
		list := work[prefix]
		sort.SliceStable(list, func(i, j int) bool {
			if list[i].Commit.Date != list[j].Commit.Date {
				return list[i].Commit.Date > list[j].Commit.Date
			}
			return list[i].Name < list[j].Name
		})
		title := "Ohne Präfix"
		if prefix != "" {
			title = prefix + "/"
		}
		groups = append(groups, Group{Key: "work", Title: title, Prefix: prefix, Branches: list})
	}
	st.listing.Groups = groups
	return st.listing
}

// environmentOrder sortiert langlebige Branches nach der Reihenfolge ihrer
// Umgebungen; der Default-Branch ohne Umgebung folgt danach.
func environmentOrder(environments []Environment, defaultName string) func(string) int {
	index := map[string]int{}
	for position, env := range environments {
		for _, name := range []string{env.Branch, env.Configured, env.NameSuggestion} {
			if name == "" {
				continue
			}
			if _, seen := index[name]; !seen {
				index[name] = position
			}
		}
	}
	return func(name string) int {
		if position, ok := index[name]; ok {
			return position
		}
		if name == defaultName {
			return len(environments)
		}
		return len(environments) + 1
	}
}

func (st *state) exists(name string) bool {
	if _, ok := st.locals[name]; ok {
		return true
	}
	return len(st.remoteRef[name]) > 0
}

// preferredRef ist der Ref, gegen den für einen Branch-Namen gerechnet wird: der
// Remote-Tracking-Branch des maßgeblichen Remotes, sonst der lokale, sonst
// irgendein Remote.
func (st *state) preferredRef(name string) string {
	if entry, ok := st.remoteRef[name][st.remote]; ok {
		return entry.ref
	}
	if entry, ok := st.locals[name]; ok {
		return entry.ref
	}
	for _, entry := range st.remoteRef[name] {
		return entry.ref
	}
	return ""
}

func (st *state) localBranch(name string, entry refEntry) Branch {
	branch := Branch{
		Name:    name,
		Ref:     entry.ref,
		Current: !st.head.Detached && st.head.Branch == name,
		Commit:  commitOf(entry),
	}
	if entry.upstream != "" {
		upstream := &Upstream{Name: strings.TrimPrefix(entry.upstream, "refs/remotes/")}
		upstream.Gone, upstream.Ahead, upstream.Behind = parseTrack(entry.track)
		branch.Upstream = upstream
	}
	st.distance(&branch, entry)
	return branch
}

func (st *state) remoteBranch(name string, remote string, entry refEntry) Branch {
	branch := Branch{
		Name:       name,
		Ref:        entry.ref,
		RemoteOnly: true,
		Remote:     remote,
		Commit:     commitOf(entry),
	}
	st.distance(&branch, entry)
	return branch
}

func commitOf(entry refEntry) Commit {
	return Commit{SHA: entry.sha, Date: entry.date, Subject: entry.subject, Author: entry.author}
}

// distance füllt Abstand und „gemergt" gegenüber dem Default-Branch.
func (st *state) distance(branch *Branch, entry refEntry) {
	if st.def.State != FieldKnown {
		reason := st.def.Reason
		if reason == "" {
			reason = "Der Default-Branch ist unbekannt."
		}
		branch.ToDefault = Distance{State: FieldUnknown, Reason: reason}
		branch.Merged = Merged{State: FieldUnknown, Reason: reason}
		return
	}
	// Der Default-Branch selbst: „gemergt" ist keine Aussage. Der Abstand bleibt
	// eine, solange der lokale Branch gegen den Remote-Tracking-Branch steht.
	self := branch.Name == st.def.Name
	if self && entry.ref == st.def.Ref {
		branch.ToDefault = Distance{State: FieldSelf}
		branch.Merged = Merged{State: FieldSelf}
		return
	}
	defer func() {
		if self {
			branch.Merged = Merged{State: FieldSelf}
		}
	}()
	if st.listing.Git.AheadBehind {
		ahead, behind, ok := parseAheadBehind(entry.aheadBehind)
		if !ok {
			branch.ToDefault = Distance{State: FieldUnknown, Reason: "git lieferte für %(ahead-behind) keinen lesbaren Wert."}
			branch.Merged = Merged{State: FieldUnknown, Reason: branch.ToDefault.Reason}
			return
		}
		branch.ToDefault = Distance{State: FieldKnown, Ahead: ahead, Behind: behind}
		branch.Merged = Merged{State: FieldKnown, Merged: ahead == 0}
		return
	}
	branch.ToDefault = Distance{State: FieldUnknown, Reason: st.listing.Git.Reason}
	if st.merged == nil {
		branch.Merged = Merged{State: FieldUnknown, Reason: st.listing.Git.Reason}
		return
	}
	branch.Merged = Merged{State: FieldKnown, Merged: st.merged[entry.ref]}
}

// parseTrack liest %(upstream:track,nobracket): „ahead 1, behind 2", „gone" oder leer.
func parseTrack(track string) (gone bool, ahead int, behind int) {
	track = strings.TrimSpace(track)
	if track == "gone" {
		return true, 0, 0
	}
	for _, part := range strings.Split(track, ",") {
		fields := strings.Fields(part)
		if len(fields) != 2 {
			continue
		}
		value, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		switch fields[0] {
		case "ahead":
			ahead = value
		case "behind":
			behind = value
		}
	}
	return false, ahead, behind
}

func parseAheadBehind(value string) (int, int, bool) {
	fields := strings.Fields(value)
	if len(fields) != 2 {
		return 0, 0, false
	}
	ahead, errAhead := strconv.Atoi(fields[0])
	behind, errBehind := strconv.Atoi(fields[1])
	return ahead, behind, errAhead == nil && errBehind == nil
}
