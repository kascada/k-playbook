package branches

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/github"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// Ergebnisse eines Prüfpunkts.
const (
	ResultOK          = "ok"
	ResultHint        = "hinweis"
	ResultBlocked     = "blockiert"
	ResultUncheckable = "nicht-pruefbar"
)

// Zustand einer Vorprüfung, die gar nicht erst prüfen konnte.
const StateInvalidTarget = "invalid-target"

// Check ist ein Prüfpunkt mit Ergebnis und Begründung.
type Check struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	// Result ist ok, hinweis, blockiert oder nicht-pruefbar.
	Result string `json:"result"`
	Reason string `json:"reason"`
	// Blocking sagt, ob der Punkt das Angebot verhindern kann. Bei einem
	// blockierenden Punkt verhindert auch nicht-pruefbar das Angebot; ein
	// Hinweis-Punkt darf nicht-pruefbar sein, ohne es zu verhindern.
	Blocking  bool             `json:"blocking"`
	Details   []string         `json:"details,omitempty"`
	Processes []SessionProcess `json:"processes,omitempty"`
}

// CheckOptions ist alles, was eine Vorprüfung braucht.
type CheckOptions struct {
	ProjectDir string
	RepoDir    string
	Settings   project.GitSettings
	// SettingsError ist ein Fehler im Abschnitt git:. Dann ist die Freigabe
	// nicht prüfbar.
	SettingsError error
	Target        string
	// Remote wählt bei einem nur remote vorhandenen Ziel den Remote, falls der
	// Name unter mehreren vorkommt.
	Remote    string
	Runner    github.Runner
	Processes ProcessSource
	// OwnPID ist der eigene Prozess, der von der Sitzungsprüfung ausgenommen ist.
	OwnPID int
}

// SwitchCheck ist die Antwort von GET /api/branches/switch-check.
type SwitchCheck struct {
	State   string `json:"state"`
	Message string `json:"message"`

	Target     string `json:"target"`
	Remote     string `json:"remote,omitempty"`
	RemoteOnly bool   `json:"remoteOnly"`
	TargetSHA  string `json:"targetSha"`
	Source     Head   `json:"source"`
	// Command ist der genaue git-Befehl, der beim Umschalten läuft.
	Command string   `json:"command"`
	Args    []string `json:"-"`

	Checks []Check `json:"checks"`
	// Offered: nichts Blockierendes gefunden, umschalten wird angeboten.
	Offered bool `json:"offered"`
	// Stamp ist der Prüfstempel aus HEAD-SHA, Ziel-SHA und Status-Hash. Der
	// Umschalt-POST schickt ihn mit; stimmt er dann nicht mehr, wird nichts
	// ausgeführt.
	Stamp string `json:"stamp"`
}

// RunCheck führt die Vorprüfung aus. Sie liest nur.
func RunCheck(ctx context.Context, options CheckOptions) SwitchCheck {
	result := SwitchCheck{Target: options.Target, Remote: options.Remote, Checks: []Check{}}
	if !project.ValidBranchName(options.Target) {
		result.State = StateInvalidTarget
		result.Message = "Das Ziel ist kein zulässiger Branch-Name."
		return result
	}
	if options.Remote != "" && !project.ValidBranchName(options.Remote) {
		result.State = StateInvalidTarget
		result.Message = "Der Remote ist kein zulässiger Name."
		return result
	}

	st, ok := load(ctx, Options{ProjectDir: options.ProjectDir, RepoDir: options.RepoDir, Settings: options.Settings, Runner: options.Runner})
	if !ok {
		result.State, result.Message = st.listing.State, st.listing.Message
		return result
	}
	result.State = StateOK
	result.Source = st.head

	c := &checker{st: st, options: options, result: &result}
	c.resolveTarget()
	c.checkPolicy()
	c.checkTarget()
	status := c.checkWorkingTree(ctx)
	ignored := c.checkIgnored(ctx)
	c.checkOperation(ctx)
	c.checkWorktree()
	c.checkDetached(ctx)
	c.checkSessions(ctx)
	c.addInvisible(ctx)
	c.checkUnpushed(ctx)
	c.checkTargetBehind()
	c.checkRemoteOnly()
	c.checkConfigInTarget(ctx)
	c.checkStash(ctx)

	result.Offered = true
	for _, check := range result.Checks {
		if check.Blocking && check.Result != ResultOK {
			result.Offered = false
		}
	}
	sum := sha256.New()
	sum.Write(status)
	sum.Write([]byte{0})
	sum.Write(ignored)
	result.Stamp = strings.Join([]string{st.head.SHA, result.TargetSHA, hex.EncodeToString(sum.Sum(nil))[:16]}, ":")
	return result
}

type checker struct {
	st      *state
	options CheckOptions
	result  *SwitchCheck
	// target ist der Eintrag des Ziels, leer wenn es nicht gefunden wurde.
	target    refEntry
	found     bool
	targetRef string
	sessions  []SessionProcess
	sessErr   error
	scanned   bool
	// top ist die Wurzel des Arbeitsbaums, einmal gelesen.
	top     string
	topErr  error
	topRead bool
}

// worktreeTop liest die Wurzel des Arbeitsbaums einmal für alle Punkte, die sie
// brauchen. Sie ist nicht immer das Code-Repo: liegt das Projekt in einem
// größeren Repo, ist sie dessen Wurzel.
func (c *checker) worktreeTop(ctx context.Context) (string, error) {
	if !c.topRead {
		c.topRead = true
		c.top, c.topErr = c.st.repo.readString(ctx, "rev-parse", "--show-toplevel")
	}
	return c.top, c.topErr
}

func (c *checker) add(check Check) {
	c.result.Checks = append(c.result.Checks, check)
}

// resolveTarget findet das Ziel: zuerst lokal, sonst als Remote-Tracking-Branch.
//
// --no-overwrite-ignore ist die zweite Sicherung neben checkIgnored: Ohne die
// Option ersetzt git switch eine ignorierte Datei oder löscht ein ignoriertes
// Verzeichnis samt Inhalt still, wenn das Ziel dort eine Datei versioniert. Mit
// ihr bricht git ab und nennt den Pfad, falls die Prüfung etwas übersehen hat
// oder sich zwischen Prüfung und Ausführung etwas geändert hat.
func (c *checker) resolveTarget() {
	name := c.options.Target
	if entry, ok := c.st.locals[name]; ok {
		c.target, c.found, c.targetRef = entry, true, entry.ref
		c.result.TargetSHA = entry.sha
		c.result.Args = []string{"switch", "--no-overwrite-ignore", name}
		c.result.Command = "git " + strings.Join(c.result.Args, " ")
		return
	}
	byRemote := c.st.remoteRef[name]
	remote := c.options.Remote
	if remote == "" {
		switch {
		case len(byRemote) == 1:
			for key := range byRemote {
				remote = key
			}
		case byRemote[c.st.remote].ref != "":
			remote = c.st.remote
		}
	}
	if entry, ok := byRemote[remote]; ok {
		c.target, c.found, c.targetRef = entry, true, entry.ref
		c.result.RemoteOnly = true
		c.result.Remote = remote
		c.result.TargetSHA = entry.sha
		c.result.Args = []string{"switch", "--no-overwrite-ignore", "--track", remote + "/" + name}
		c.result.Command = "git " + strings.Join(c.result.Args, " ")
	}
}

func (c *checker) checkPolicy() {
	check := Check{ID: "freigabe", Title: "Umschalten freigegeben", Blocking: true}
	settings := c.options.Settings
	switch {
	case c.options.SettingsError != nil:
		check.Result = ResultUncheckable
		check.Reason = "Der Abschnitt git: in " + project.ConfigFileName + " ist fehlerhaft: " + c.options.SettingsError.Error()
	case settings.Switch != project.GitSwitchOffer:
		check.Result = ResultBlocked
		if settings.Switch == project.GitSwitchOff {
			check.Reason = "git.switch steht auf off: Umschalten aus der Oberfläche ist für dieses Projekt abgeschaltet."
		} else {
			check.Reason = "git.switch ist nicht entschieden (unknown). Freigeben lässt es sich in " + project.ConfigFileName + " mit git.switch: offer."
		}
	default:
		allowed, pattern := project.BranchAllowed(settings.Allow, c.options.Target)
		switch {
		case !allowed:
			check.Result = ResultBlocked
			check.Reason = c.options.Target + " passt zu keinem Muster in git.allow (" + strings.Join(settings.Allow, ", ") + ")."
		case pattern == "":
			check.Result = ResultOK
			check.Reason = "git.switch: offer, und git.allow ist leer: jeder Branch ist zulässig."
		default:
			check.Result = ResultOK
			check.Reason = "git.switch: offer, und " + c.options.Target + " passt zum Muster " + pattern + " in git.allow."
		}
	}
	c.add(check)
}

func (c *checker) checkTarget() {
	check := Check{ID: "ziel", Title: "Ziel vorhanden", Blocking: true}
	name := c.options.Target
	switch {
	case !c.found && c.options.Remote != "":
		check.Result = ResultBlocked
		check.Reason = "Den Branch " + name + " gibt es weder lokal noch als " + c.options.Remote + "/" + name + "."
	case !c.found && len(c.st.remoteRef[name]) > 1:
		check.Result = ResultBlocked
		check.Reason = "Den Branch " + name + " gibt es nur remote, und das unter mehreren Remotes; welcher gemeint ist, ist nicht eindeutig."
	case !c.found:
		check.Result = ResultBlocked
		check.Reason = "Den Branch " + name + " gibt es weder lokal noch als Remote-Tracking-Branch. Ist er neu auf dem Remote, holt ihn ein Fetch."
	case !c.st.head.Detached && !c.result.RemoteOnly && c.st.head.Branch == name:
		check.Result = ResultBlocked
		check.Reason = name + " ist bereits ausgecheckt."
	case c.result.RemoteOnly:
		check.Result = ResultOK
		check.Reason = name + " gibt es als " + c.result.Remote + "/" + name + "; der lokale Branch entsteht beim Umschalten."
	default:
		check.Result = ResultOK
		check.Reason = name + " ist ein lokaler Branch auf " + shortSHA(c.target.sha) + "."
	}
	c.add(check)
}

// checkWorkingTree prüft auf geänderte, gestagte und nicht ignorierte
// unversionierte Dateien. Zurück kommt die rohe Ausgabe für den Prüfstempel.
func (c *checker) checkWorkingTree(ctx context.Context) []byte {
	check := Check{ID: "arbeitsbaum", Title: "Arbeitsbaum sauber", Blocking: true}
	out, err := c.st.repo.read(ctx, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		check.Result = ResultUncheckable
		check.Reason = describeError("git status", err)
		c.add(check)
		return []byte("unlesbar")
	}
	entries := splitNul(out)
	// Bei Umbenennungen folgt der alte Name als eigener Eintrag ohne Status.
	lines := []string{}
	for index := 0; index < len(entries); index++ {
		entry := entries[index]
		if len(entry) < 4 {
			continue
		}
		lines = append(lines, entry)
		if entry[0] == 'R' || entry[0] == 'C' {
			index++
		}
	}
	if len(lines) == 0 {
		check.Result = ResultOK
		check.Reason = "Keine geänderten, gestagten oder unversionierten Dateien."
	} else {
		check.Result = ResultBlocked
		check.Reason = countText(len(lines), "Datei ist", "Dateien sind") + " geändert, gestagt oder unversioniert. Sie gehen beim Umschalten nicht verloren, würden aber mitgenommen oder den Wechsel verhindern; erst committen, sichern oder aufräumen."
		check.Details = limitDetails(lines, 20)
	}
	c.add(check)
	return out
}

// checkIgnored prüft vorhandene ignorierte Pfade gegen den Baum des Ziels. git
// switch überschreibt ohne --no-overwrite-ignore eine ignorierte Datei ohne
// Warnung, wenn das Ziel sie versioniert — etwa eine .env —, und löscht ein
// ignoriertes Verzeichnis samt Inhalt, wenn das Ziel dort eine Datei hat.
func (c *checker) checkIgnored(ctx context.Context) []byte {
	check := Check{ID: "ignorierte-dateien", Title: "Keine ignorierte Datei, die das Ziel versioniert", Blocking: true}
	top, err := c.worktreeTop(ctx)
	if err != nil {
		check.Result = ResultUncheckable
		check.Reason = describeError("Die Wurzel des Arbeitsbaums", err)
		c.add(check)
		return nil
	}
	topRepo := Repo{Runner: c.st.repo.Runner, Dir: top}
	ignoredRaw, err := topRepo.read(ctx, "ls-files", "-z", "--others", "--ignored", "--exclude-standard", "--directory")
	if err != nil {
		check.Result = ResultUncheckable
		check.Reason = describeError("Die ignorierten Dateien", err)
		c.add(check)
		return []byte("unlesbar")
	}
	if !c.found {
		check.Result = ResultUncheckable
		check.Reason = "Ohne Ziel gibt es keinen Baum, gegen den sich prüfen ließe."
		c.add(check)
		return ignoredRaw
	}
	treeRaw, err := topRepo.read(ctx, "ls-tree", "-r", "-z", "--name-only", "--full-tree", c.targetRef)
	if err != nil {
		check.Result = ResultUncheckable
		check.Reason = describeError("Der Baum des Ziels", err)
		c.add(check)
		return ignoredRaw
	}
	conflicts := IgnoredConflicts(top, splitNul(ignoredRaw), splitNul(treeRaw))
	if len(conflicts) == 0 {
		check.Result = ResultOK
		check.Reason = "Das Ziel versioniert keine der vorhandenen ignorierten Dateien."
	} else {
		check.Result = ResultBlocked
		check.Reason = countText(len(conflicts), "ignorierter Pfad liegt", "ignorierte Pfade liegen") + " dort, wo das Ziel eine Datei versioniert. Ein einfaches git switch überschriebe sie ohne Warnung, der Befehl hier bräche wegen --no-overwrite-ignore ab; erst sichern oder verschieben."
		check.Details = limitDetails(conflicts, 20)
	}
	c.add(check)
	return ignoredRaw
}

// IgnoredConflicts vergleicht vorhandene ignorierte Pfade mit den Dateien des
// Zielbaums. ignored kommt aus ls-files --directory: ein ganz ignoriertes
// Verzeichnis steht als „dir/". Ein Konflikt ist eine Zieldatei,
//
//   - die genau eine ignorierte Datei trifft,
//   - an deren Stelle ein ignoriertes Verzeichnis mit Inhalt liegt — git switch
//     löscht es samt Inhalt, um die Datei anzulegen,
//   - deren Verzeichnis eine ignorierte Datei im Weg steht,
//   - oder die in einem ignorierten Verzeichnis liegt und dort tatsächlich
//     vorhanden ist oder eine vorhandene Datei als Verzeichnis bräuchte.
//
// Ein leeres ignoriertes Verzeichnis ist kein Konflikt: git räumt es ohne
// Verlust weg.
func IgnoredConflicts(top string, ignored []string, tree []string) []string {
	files := map[string]bool{}
	dirs := map[string]bool{}
	for _, path := range ignored {
		if strings.HasSuffix(path, "/") {
			dirs[strings.TrimSuffix(path, "/")] = true
		} else if path != "" {
			files[path] = true
		}
	}
	if len(files) == 0 && len(dirs) == 0 {
		return nil
	}
	onDisk := func(path string) string {
		return filepath.Join(top, filepath.FromSlash(path))
	}
	conflicts := []string{}
	for _, path := range tree {
		if path == "" {
			continue
		}
		if files[path] {
			conflicts = append(conflicts, path)
			continue
		}
		if dirs[path] {
			// Ist es nicht lesbar, zählt es als belegt; verschwunden ist es kein Konflikt.
			if entries, err := os.ReadDir(onDisk(path)); (err != nil && !os.IsNotExist(err)) || len(entries) > 0 {
				conflicts = append(conflicts, path+" (ignoriertes Verzeichnis "+path+"/ mit Inhalt steht im Weg)")
			}
			continue
		}
		parts := strings.Split(path, "/")
		for depth := 1; depth < len(parts); depth++ {
			prefix := strings.Join(parts[:depth], "/")
			if files[prefix] {
				conflicts = append(conflicts, path+" (ignorierte Datei "+prefix+" steht im Weg)")
				break
			}
			if dirs[prefix] {
				if conflict := conflictBelow(onDisk, parts, depth); conflict != "" {
					conflicts = append(conflicts, conflict)
				}
				break
			}
		}
	}
	return conflicts
}

// conflictBelow prüft eine Zieldatei unterhalb eines ignorierten Verzeichnisses
// auf der Platte: Liegt eine Datei dort, wo das Ziel ein Verzeichnis braucht,
// steht sie im Weg; liegt am Ziel selbst schon etwas, würde es ersetzt.
func conflictBelow(onDisk func(string) string, parts []string, depth int) string {
	path := strings.Join(parts, "/")
	for below := depth + 1; below <= len(parts); below++ {
		current := strings.Join(parts[:below], "/")
		info, err := os.Lstat(onDisk(current))
		if err != nil {
			return ""
		}
		if below == len(parts) {
			return path
		}
		if !info.IsDir() {
			return path + " (ignorierte Datei " + current + " steht im Weg)"
		}
	}
	return ""
}

// checkOperation sucht eine laufende git-Operation. Die Pfade kommen aus
// git rev-parse --git-path, damit sie auch in einem weiteren Worktree stimmen.
func (c *checker) checkOperation(ctx context.Context) {
	check := Check{ID: "git-operation", Title: "Keine laufende git-Operation", Blocking: true}
	markers := []struct {
		path string
		name string
	}{
		{"MERGE_HEAD", "merge"},
		{"rebase-merge", "rebase"},
		{"rebase-apply", "rebase oder am"},
		{"CHERRY_PICK_HEAD", "cherry-pick"},
		{"REVERT_HEAD", "revert"},
		{"BISECT_LOG", "bisect"},
	}
	args := []string{"rev-parse"}
	for _, marker := range markers {
		args = append(args, "--git-path", marker.path)
	}
	out, err := c.st.repo.readString(ctx, args...)
	if err != nil {
		check.Result = ResultUncheckable
		check.Reason = describeError("Die Pfade der git-Operationen", err)
		c.add(check)
		return
	}
	paths := strings.Split(out, "\n")
	if len(paths) != len(markers) {
		check.Result = ResultUncheckable
		check.Reason = "git rev-parse --git-path lieferte eine unerwartete Antwort."
		c.add(check)
		return
	}
	running := []string{}
	for index, marker := range markers {
		path := strings.TrimSpace(paths[index])
		if !filepath.IsAbs(path) {
			path = filepath.Join(c.options.RepoDir, path)
		}
		if _, err := os.Lstat(path); err == nil {
			running = append(running, marker.name)
		}
	}
	if len(running) == 0 {
		check.Result = ResultOK
		check.Reason = "Kein merge, rebase, cherry-pick, revert oder bisect in Arbeit."
	} else {
		check.Result = ResultBlocked
		check.Reason = "In Arbeit: " + strings.Join(running, ", ") + ". Erst abschließen oder abbrechen."
	}
	c.add(check)
}

func (c *checker) checkWorktree() {
	check := Check{ID: "worktree", Title: "Ziel in keinem anderen Worktree ausgecheckt", Blocking: true}
	for _, worktree := range c.st.worktrees {
		if worktree.Current || worktree.Branch != c.options.Target || c.result.RemoteOnly {
			continue
		}
		check.Result = ResultBlocked
		if worktree.Prunable {
			check.Reason = c.options.Target + " ist im Worktree " + worktree.Path + " eingetragen, den es nicht mehr gibt (prunable: " + worktree.PrunableReason + "). git hält den Branch trotzdem fest, bis „git worktree prune“ den Eintrag entfernt."
		} else {
			check.Reason = c.options.Target + " ist im Worktree " + worktree.Path + " ausgecheckt. Ein Branch kann nur in einem Worktree zugleich ausgecheckt sein."
		}
		c.add(check)
		return
	}
	check.Result = ResultOK
	check.Reason = "Das Ziel ist in keinem anderen Worktree ausgecheckt."
	c.add(check)
}

func (c *checker) checkDetached(ctx context.Context) {
	check := Check{ID: "detached-head", Title: "Kein unerreichbarer Detached HEAD", Blocking: true}
	switch {
	case !c.st.head.Detached:
		check.Result = ResultOK
		check.Reason = "HEAD steht auf dem Branch " + c.st.head.Branch + "."
	case c.st.head.SHA == "":
		check.Result = ResultOK
		check.Reason = "HEAD hat noch keinen Commit."
	default:
		out, err := c.st.repo.readString(ctx, "for-each-ref", "--contains", c.st.head.SHA, "--count=1", "--format=%(refname)", "refs/heads", "refs/remotes", "refs/tags")
		switch {
		case err != nil:
			check.Result = ResultUncheckable
			check.Reason = describeError("Die Erreichbarkeit von HEAD", err)
		case strings.TrimSpace(out) == "":
			check.Result = ResultBlocked
			check.Reason = "HEAD steht losgelöst auf " + shortSHA(c.st.head.SHA) + ", und kein Branch und kein Tag erreicht diesen Commit. Nach dem Umschalten wäre er nur noch über das Reflog auffindbar; erst einen Branch darauf anlegen."
		default:
			check.Result = ResultOK
			check.Reason = "HEAD steht losgelöst auf " + shortSHA(c.st.head.SHA) + ", der Commit ist über " + strings.TrimSpace(out) + " erreichbar."
		}
	}
	c.add(check)
}

// scan liest die Prozesse einmal für alle Punkte, die sie brauchen.
func (c *checker) scan(ctx context.Context) {
	if c.scanned {
		return
	}
	c.scanned = true
	c.sessions, c.sessErr = findSessions(c.options.Processes, c.options.OwnPID, c.sessionDirs(ctx))
}

// sessionDirs sind die Verzeichnisse, in denen eine Sitzung den Wechsel spürt:
// Projektverzeichnis, Code-Repo und die Wurzel des Arbeitsbaums. Die Wurzel
// zählt, weil git switch den ganzen Arbeitsbaum umschaltet — liegt das
// Code-Repo in einem Monorepo, trifft es auch eine Sitzung in dessen Wurzel
// oder in einem Nachbarordner. Ist die Wurzel nicht lesbar, blockiert schon der
// Punkt zu ignorierten Dateien.
func (c *checker) sessionDirs(ctx context.Context) []string {
	dirs := []string{c.options.ProjectDir, c.options.RepoDir}
	if top, err := c.worktreeTop(ctx); err == nil && top != "" {
		dirs = append(dirs, top)
	}
	return dirs
}

// sessionPlaces nennt die geprüften Verzeichnisse für die Begründung, ohne
// dasselbe Verzeichnis zweimal.
func (c *checker) sessionPlaces(ctx context.Context) string {
	places := []string{}
	seen := map[string]bool{}
	for index, dir := range c.sessionDirs(ctx) {
		resolved := resolvedPath(dir)
		if resolved == "" || seen[resolved] {
			continue
		}
		seen[resolved] = true
		if index == 2 {
			places = append(places, "der Wurzel des Arbeitsbaums "+resolved)
		} else {
			places = append(places, dir)
		}
	}
	if len(places) <= 1 {
		return strings.Join(places, "")
	}
	return strings.Join(places[:len(places)-1], ", ") + " und " + places[len(places)-1]
}

func (c *checker) checkSessions(ctx context.Context) {
	c.scan(ctx)
	check := Check{ID: "sitzungen", Title: "Keine laufende Sitzung im Projekt", Blocking: true}
	if c.sessErr != nil {
		check.Result = ResultUncheckable
		if errors.Is(c.sessErr, ErrNoProcessSource) {
			check.Reason = "Auf diesem System gibt es kein /proc; laufende Sitzungen lassen sich nicht erkennen. Ohne diese Prüfung wird nicht umgeschaltet."
		} else {
			check.Reason = "Die Prozesse ließen sich nicht lesen: " + c.sessErr.Error() + ". Ohne diese Prüfung wird nicht umgeschaltet."
		}
		c.add(check)
		return
	}
	blocking := []SessionProcess{}
	for _, process := range c.sessions {
		if process.Blocking {
			blocking = append(blocking, process)
		}
	}
	where := c.sessionPlaces(ctx)
	if len(blocking) == 0 {
		check.Result = ResultOK
		check.Reason = "In " + where + " hat die Prozessquelle keine KI-Sitzung und keinen MCP-Server gefunden. Das heißt nur: dort nichts gesehen — siehe den Punkt zu nicht sichtbaren Sitzungen."
	} else {
		check.Result = ResultBlocked
		check.Reason = countText(len(blocking), "Prozess einer Sitzung läuft", "Prozesse von Sitzungen laufen") + " in " + where + ". Ein Wechsel änderte ihnen die Dateien unter der Hand; wer umschalten will, beendet sie."
		check.Processes = blocking
	}
	c.add(check)
}

func (c *checker) addInvisible(ctx context.Context) {
	c.scan(ctx)
	invisible := Check{
		ID:     "unsichtbare-sitzungen",
		Title:  "Nicht sichtbare Sitzungen",
		Result: ResultHint,
		Reason: "Nicht gesehen werden können: Sitzungen des zentralen opencode-Dienstes (per --dir, ohne Arbeitsverzeichnis im Projekt), der Chat dieser Oberfläche und Sitzungen in einem DevContainer oder anderen Container. Läuft dort etwas in diesem Projekt, gilt es ebenso.",
	}
	// Ein angebundener DevContainer zeigt sich auf dem Host nur als docker exec
	// mit dem Projekt als Arbeitsverzeichnis. Was darin läuft, bleibt unsichtbar;
	// dass er offen ist, lässt sich aber sagen.
	for _, process := range c.sessions {
		if strings.Contains(process.Command, "docker exec") || strings.Contains(process.Command, "VSCODE_REMOTE_CONTAINERS_SESSION") {
			invisible.Reason += " Für dieses Projekt ist ein Container angebunden (docker exec, PID " + strconv.Itoa(process.PID) + "); Sitzungen darin sind nicht geprüft."
			break
		}
	}
	c.add(invisible)
	if c.sessErr != nil {
		c.add(Check{ID: "editorfenster", Title: "Offene Editorfenster", Result: ResultUncheckable, Reason: "Ohne Prozessquelle nicht prüfbar."})
		return
	}
	editors, others := []SessionProcess{}, []SessionProcess{}
	for _, process := range c.sessions {
		switch {
		case process.Blocking:
		case process.Kind == KindVSCode:
			editors = append(editors, process)
		default:
			others = append(others, process)
		}
	}
	editor := Check{ID: "editorfenster", Title: "Offene Editorfenster", Result: ResultOK, Reason: "Kein VS-Code-Server-Prozess ohne KI-Erweiterung im Projekt gefunden."}
	if len(editors) > 0 {
		editor.Result = ResultHint
		editor.Reason = countText(len(editors), "Prozess des VS-Code-Servers arbeitet", "Prozesse des VS-Code-Servers arbeiten") + " im Projekt. Ungespeicherte Änderungen im Editor gehören nicht zum Arbeitsbaum und werden nicht geprüft."
		editor.Processes = editors
	}
	c.add(editor)
	other := Check{ID: "andere-prozesse", Title: "Weitere Prozesse im Projekt", Result: ResultOK, Reason: "Keine weiteren Prozesse im Projekt gefunden."}
	if len(others) > 0 {
		other.Result = ResultHint
		other.Reason = countText(len(others), "weiterer Prozess arbeitet", "weitere Prozesse arbeiten") + " im Projekt, etwa eine Shell oder ein Dev-Server. Sie blockieren nicht; ein laufender Dev-Server sieht nach dem Wechsel andere Dateien."
		other.Processes = others
	}
	c.add(other)
}

func (c *checker) checkUnpushed(ctx context.Context) {
	check := Check{ID: "ungepushte-commits", Title: "Ungepushte Commits"}
	if c.st.head.SHA == "" {
		check.Result = ResultOK
		check.Reason = "HEAD hat noch keinen Commit."
		c.add(check)
		return
	}
	if len(c.st.remotes) == 0 {
		check.Result = ResultHint
		check.Reason = "Das Repo hat kein Remote; kein Commit ist irgendwo gesichert."
		c.add(check)
		return
	}
	out, err := c.st.repo.readString(ctx, "rev-list", "--count", "HEAD", "--not", "--remotes")
	count, convErr := strconv.Atoi(strings.TrimSpace(out))
	switch {
	case err != nil:
		check.Result = ResultUncheckable
		check.Reason = describeError("Die Zahl ungepushter Commits", err)
	case convErr != nil:
		check.Result = ResultUncheckable
		check.Reason = "git lieferte keine Zahl ungepushter Commits."
	case count == 0:
		check.Result = ResultOK
		check.Reason = "Jeder Commit des aktuellen Stands liegt auch auf einem Remote-Tracking-Branch."
	default:
		check.Result = ResultHint
		check.Reason = countText(count, "Commit des aktuellen Stands liegt", "Commits des aktuellen Stands liegen") + " auf keinem Remote-Tracking-Branch. Sie bleiben im Branch erhalten, sind aber nicht gepusht."
	}
	c.add(check)
}

func (c *checker) checkTargetBehind() {
	check := Check{ID: "ziel-hinter-upstream", Title: "Ziel auf dem Stand seines Upstreams"}
	switch {
	case !c.found || c.result.RemoteOnly:
		check.Result = ResultOK
		check.Reason = "Kein lokaler Ziel-Branch, dessen Upstream zurückliegen könnte."
	case c.target.upstream == "":
		check.Result = ResultHint
		check.Reason = c.options.Target + " hat keinen Upstream."
	default:
		gone, ahead, behind := parseTrack(c.target.track)
		upstream := strings.TrimPrefix(c.target.upstream, "refs/remotes/")
		switch {
		case gone:
			check.Result = ResultHint
			check.Reason = "Der Upstream " + upstream + " ist auf dem Remote gelöscht ([gone])."
		case behind > 0:
			check.Result = ResultHint
			check.Reason = c.options.Target + " liegt " + commits(behind) + " hinter " + upstream + ". Umgeschaltet wird ohne Pull; aktualisieren ist ein eigener Schritt."
			if ahead > 0 {
				check.Reason += " Zugleich trägt er " + commits(ahead) + ", die dort fehlen."
			}
		default:
			check.Result = ResultOK
			check.Reason = c.options.Target + " liegt nicht hinter " + upstream + " (Stand des letzten Fetch)."
		}
	}
	c.add(check)
}

func (c *checker) checkRemoteOnly() {
	check := Check{ID: "nur-remote", Title: "Ziel lokal vorhanden", Result: ResultOK, Reason: "Das Ziel ist ein lokaler Branch."}
	if c.result.RemoteOnly {
		check.Result = ResultHint
		check.Reason = c.options.Target + " gibt es nur als " + c.result.Remote + "/" + c.options.Target + ". Beim Umschalten entsteht der lokale Branch als Tracking-Branch (" + c.result.Command + ")."
	}
	if !c.found {
		check.Result = ResultUncheckable
		check.Reason = "Ohne Ziel nicht bestimmbar."
	}
	c.add(check)
}

// checkConfigInTarget vergleicht K-PLAYBOOK.yaml und k-playbook-local/ zwischen
// HEAD und Ziel, wenn sie im umgeschalteten Repo liegen.
func (c *checker) checkConfigInTarget(ctx context.Context) {
	check := Check{ID: "konfiguration-im-ziel", Title: "Konfiguration im Ziel gleich"}
	if !configInRepo(c.options.ProjectDir, c.options.RepoDir) {
		check.Result = ResultOK
		check.Reason = project.ConfigFileName + " und " + project.LocalDirName + "/ liegen außerhalb des Code-Repos und wechseln nicht mit."
		c.add(check)
		return
	}
	if !c.found || c.st.head.SHA == "" {
		check.Result = ResultUncheckable
		check.Reason = "Ohne Ziel oder ohne Commit in HEAD gibt es nichts zu vergleichen."
		c.add(check)
		return
	}
	top, err := c.worktreeTop(ctx)
	if err != nil {
		check.Result = ResultUncheckable
		check.Reason = describeError("Die Wurzel des Arbeitsbaums", err)
		c.add(check)
		return
	}
	paths := []string{}
	for _, path := range []string{project.ConfigPath(c.options.ProjectDir), project.LocalDir(c.options.ProjectDir)} {
		relative, err := filepath.Rel(resolvedPath(top), resolvedPath(filepath.Dir(path)))
		if err != nil {
			continue
		}
		paths = append(paths, filepath.ToSlash(filepath.Join(relative, filepath.Base(path))))
	}
	topRepo := Repo{Runner: c.st.repo.Runner, Dir: top}
	out, err := topRepo.readString(ctx, append([]string{"diff", "--name-only", "HEAD", c.targetRef, "--"}, paths...)...)
	switch {
	case err != nil:
		check.Result = ResultUncheckable
		check.Reason = describeError("Der Vergleich der Konfiguration", err)
	case strings.TrimSpace(out) == "":
		check.Result = ResultOK
		check.Reason = project.ConfigFileName + " und " + project.LocalDirName + "/ sind im Ziel gleich."
	default:
		changed := strings.Split(strings.TrimSpace(out), "\n")
		check.Result = ResultHint
		check.Reason = "Im Ziel " + countText(len(changed), "Datei unterscheidet", "Dateien unterscheiden") + " sich in " + project.ConfigFileName + " oder " + project.LocalDirName + "/. Nach dem Wechsel gelten Konfiguration, Regeln und Tasks des Ziels."
		check.Details = limitDetails(changed, 20)
	}
	c.add(check)
}

func (c *checker) checkStash(ctx context.Context) {
	check := Check{ID: "stash", Title: "Stash-Einträge"}
	out, err := c.st.repo.read(ctx, "stash", "list")
	switch {
	case err != nil:
		check.Result = ResultUncheckable
		check.Reason = describeError("git stash list", err)
	case len(bytes.TrimSpace(out)) == 0:
		check.Result = ResultOK
		check.Reason = "Keine Stash-Einträge."
	default:
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		check.Result = ResultHint
		check.Reason = countText(len(lines), "Stash-Eintrag liegt", "Stash-Einträge liegen") + " vor. Sie bleiben beim Umschalten erhalten; ein Wechsel legt keinen neuen an."
		check.Details = limitDetails(lines, 10)
	}
	c.add(check)
}

func splitNul(raw []byte) []string {
	parts := []string{}
	for _, part := range strings.Split(string(raw), "\x00") {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

// countText setzt eine Zahl vor Einzahl oder Mehrzahl.
func countText(count int, singular string, plural string) string {
	if count == 1 {
		return "1 " + singular
	}
	return strconv.Itoa(count) + " " + plural
}

func limitDetails(lines []string, limit int) []string {
	if len(lines) <= limit {
		return lines
	}
	rest := len(lines) - limit
	return append(append([]string{}, lines[:limit]...), "… und "+strconv.Itoa(rest)+" weitere")
}
