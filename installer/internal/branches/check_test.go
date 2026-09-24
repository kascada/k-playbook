package branches

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// fakeProcesses ist die austauschbare Prozessquelle der Tests.
type fakeProcesses struct {
	processes []Process
	err       error
}

func (f fakeProcesses) Processes() ([]Process, error) {
	return f.processes, f.err
}

var offerAll = project.GitSettings{Switch: project.GitSwitchOffer, Allow: []string{}, Configured: true}

// checkOptions sind Vorgaben, unter denen im sauberen Fixture alles durchgeht:
// Umschalten freigegeben, keine Prozesse.
func (f fixture) checkOptions(target string) CheckOptions {
	return CheckOptions{
		ProjectDir: f.work,
		RepoDir:    f.work,
		Settings:   offerAll,
		Target:     target,
		Processes:  fakeProcesses{},
		OwnPID:     os.Getpid(),
	}
}

func checkByID(t *testing.T, result SwitchCheck, id string) Check {
	t.Helper()
	for _, check := range result.Checks {
		if check.ID == id {
			return check
		}
	}
	t.Fatalf("Prüfpunkt %s fehlt: %+v", id, result.Checks)
	return Check{}
}

func expectResult(t *testing.T, result SwitchCheck, id string, want string) Check {
	t.Helper()
	check := checkByID(t, result, id)
	if check.Result != want {
		t.Errorf("%s = %s (%s), erwartet %s", id, check.Result, check.Reason, want)
	}
	if check.Reason == "" {
		t.Errorf("%s ohne Begründung", id)
	}
	return check
}

// Im sauberen Repo wird umgeschaltet; jeder Prüfpunkt steht mit Ergebnis und
// Grund da, auch die bestandenen.
func TestVorpruefungSauberesRepo(t *testing.T) {
	f := newFixture(t)
	result := RunCheck(testContext(t), f.checkOptions("feature/b"))
	if result.State != StateOK {
		t.Fatalf("State = %q: %s", result.State, result.Message)
	}
	if !result.Offered {
		t.Errorf("nicht angeboten: %+v", result.Checks)
	}
	if result.Command != "git switch --no-overwrite-ignore feature/b" || result.TargetSHA == "" || result.Source.Branch != "main" {
		t.Errorf("Command = %q, TargetSHA = %q, Source = %+v", result.Command, result.TargetSHA, result.Source)
	}
	parts := strings.Split(result.Stamp, ":")
	if len(parts) != 3 || parts[0] != result.Source.SHA || parts[1] != result.TargetSHA || len(parts[2]) != 16 {
		t.Errorf("Stamp = %q", result.Stamp)
	}
	for _, id := range []string{"freigabe", "ziel", "arbeitsbaum", "ignorierte-dateien", "git-operation", "worktree", "detached-head", "sitzungen"} {
		check := expectResult(t, result, id, ResultOK)
		if !check.Blocking {
			t.Errorf("%s ist nicht als blockierend markiert", id)
		}
	}
	invisible := expectResult(t, result, "unsichtbare-sitzungen", ResultHint)
	if invisible.Blocking || !strings.Contains(invisible.Reason, "opencode") {
		t.Errorf("unsichtbare-sitzungen = %+v", invisible)
	}
	for _, id := range []string{"ungepushte-commits", "ziel-hinter-upstream", "nur-remote", "editorfenster", "konfiguration-im-ziel", "stash"} {
		if check := checkByID(t, result, id); check.Blocking {
			t.Errorf("%s ist blockierend, soll Hinweis sein", id)
		}
	}
}

func TestVorpruefungFreigabe(t *testing.T) {
	f := newFixture(t)
	ctx := testContext(t)

	options := f.checkOptions("feature/b")
	options.Settings = project.GitSettings{Switch: project.GitSwitchUnknown}
	result := RunCheck(ctx, options)
	check := expectResult(t, result, "freigabe", ResultBlocked)
	if result.Offered || !strings.Contains(check.Reason, "git.switch") {
		t.Errorf("unknown: Offered = %v, %s", result.Offered, check.Reason)
	}

	options.Settings = project.GitSettings{Switch: project.GitSwitchOffer, Allow: []string{"development", "remediation/*"}}
	result = RunCheck(ctx, options)
	expectResult(t, result, "freigabe", ResultBlocked)

	options.Target = "development"
	result = RunCheck(ctx, options)
	expectResult(t, result, "freigabe", ResultOK)

	// Ein fehlerhafter Abschnitt macht die Freigabe nicht prüfbar — und das
	// verhindert das Angebot wie blockiert.
	options.SettingsError = errors.New("git.switch hat den unbekannten Wert \"ja\"")
	result = RunCheck(ctx, options)
	expectResult(t, result, "freigabe", ResultUncheckable)
	if result.Offered {
		t.Error("nicht-pruefbar bei einem blockierenden Punkt gibt das Angebot frei")
	}
}

func TestVorpruefungZiel(t *testing.T) {
	f := newFixture(t)
	ctx := testContext(t)

	result := RunCheck(ctx, f.checkOptions("gibt-es-nicht"))
	expectResult(t, result, "ziel", ResultBlocked)
	if result.Offered {
		t.Error("fehlendes Ziel angeboten")
	}
	expectResult(t, RunCheck(ctx, f.checkOptions("main")), "ziel", ResultBlocked)

	if result := RunCheck(ctx, f.checkOptions("--force")); result.State != StateInvalidTarget {
		t.Errorf("State für --force = %q", result.State)
	}
}

// Nur remote: angeboten mit --track und als Hinweis gekennzeichnet.
func TestVorpruefungNurRemote(t *testing.T) {
	f := newFixture(t)
	result := RunCheck(testContext(t), f.checkOptions("remediation/x"))
	if !result.Offered || !result.RemoteOnly || result.Command != "git switch --no-overwrite-ignore --track origin/remediation/x" {
		t.Errorf("Offered = %v, RemoteOnly = %v, Command = %q", result.Offered, result.RemoteOnly, result.Command)
	}
	expectResult(t, result, "nur-remote", ResultHint)
}

func TestVorpruefungArbeitsbaum(t *testing.T) {
	f := newFixture(t)
	ctx := testContext(t)

	writeFile(t, filepath.Join(f.work, "README.md"), "geändert\n")
	result := RunCheck(ctx, f.checkOptions("feature/b"))
	check := expectResult(t, result, "arbeitsbaum", ResultBlocked)
	if result.Offered || len(check.Details) != 1 || !strings.Contains(check.Details[0], "README.md") {
		t.Errorf("geändert: Offered = %v, Details = %v", result.Offered, check.Details)
	}
	run(t, f.work, "checkout", "--", "README.md")

	writeFile(t, filepath.Join(f.work, "neu.txt"), "neu\n")
	expectResult(t, RunCheck(ctx, f.checkOptions("feature/b")), "arbeitsbaum", ResultBlocked)
	os.Remove(filepath.Join(f.work, "neu.txt"))

	run(t, f.work, "switch", "-q", "development")
	writeFile(t, filepath.Join(f.work, "development.txt"), "gestagt\n")
	run(t, f.work, "add", "development.txt")
	expectResult(t, RunCheck(ctx, f.checkOptions("feature/b")), "arbeitsbaum", ResultBlocked)
}

// Eine ignorierte Datei, die das Ziel versioniert, blockiert: git switch
// überschriebe sie ohne Warnung.
func TestVorpruefungIgnorierteDateiImZiel(t *testing.T) {
	f := newFixture(t)
	ctx := testContext(t)

	// mit-env versioniert .env; main ignoriert sie.
	run(t, f.work, "switch", "-q", "-c", "mit-env")
	writeFile(t, filepath.Join(f.work, ".env"), "GEHEIM=aus-dem-branch\n")
	run(t, f.work, "add", "-f", ".env")
	run(t, f.work, "commit", "-q", "-m", ".env versioniert")
	run(t, f.work, "switch", "-q", "main")
	commit(t, f.work, ".gitignore", ".env\ncache/\n", "ignoriert .env")
	writeFile(t, filepath.Join(f.work, ".env"), "GEHEIM=lokal\n")

	result := RunCheck(ctx, f.checkOptions("mit-env"))
	expectResult(t, result, "arbeitsbaum", ResultOK)
	check := expectResult(t, result, "ignorierte-dateien", ResultBlocked)
	if result.Offered || len(check.Details) != 1 || check.Details[0] != ".env" {
		t.Errorf("Offered = %v, Details = %v", result.Offered, check.Details)
	}

	// Ein ignoriertes Verzeichnis ohne die Datei des Ziels ist kein Konflikt.
	os.Remove(filepath.Join(f.work, ".env"))
	writeFile(t, filepath.Join(f.work, "cache", "lokal.bin"), "x")
	expectResult(t, RunCheck(ctx, f.checkOptions("mit-env")), "ignorierte-dateien", ResultOK)
}

func TestIgnoredConflicts(t *testing.T) {
	top := t.TempDir()
	writeFile(t, filepath.Join(top, "cache", "a.txt"), "a")
	writeFile(t, filepath.Join(top, "daten", "x.bin"), "x")
	writeFile(t, filepath.Join(top, "src", "tmp", "y.bin"), "y")
	writeFile(t, filepath.Join(top, "ablage", "sub"), "datei statt verzeichnis")
	if err := os.MkdirAll(filepath.Join(top, "leer"), 0o755); err != nil {
		t.Fatal(err)
	}
	conflicts := IgnoredConflicts(top,
		[]string{".env", "cache/", "build", "daten/", "src/tmp/", "ablage/", "leer/"},
		[]string{
			".env",
			"cache/a.txt",
			"cache/b.txt",
			"build/out.js",
			"src/main.go",
			// Das Ziel versioniert eine Datei, wo ein ignoriertes Verzeichnis
			// mit Inhalt liegt — auf oberster Ebene und darunter.
			"daten",
			"src/tmp",
			// Das Ziel versioniert eine Datei unter einem Verzeichnis, an dessen
			// Stelle im ignorierten Verzeichnis eine Datei liegt.
			"ablage/sub/datei",
			// Ein leeres ignoriertes Verzeichnis räumt git ohne Verlust weg.
			"leer",
		},
	)
	want := strings.Join([]string{
		".env",
		"cache/a.txt",
		"build/out.js (ignorierte Datei build steht im Weg)",
		"daten (ignoriertes Verzeichnis daten/ mit Inhalt steht im Weg)",
		"src/tmp (ignoriertes Verzeichnis src/tmp/ mit Inhalt steht im Weg)",
		"ablage/sub/datei (ignorierte Datei ablage/sub steht im Weg)",
	}, " | ")
	if strings.Join(conflicts, " | ") != want {
		t.Errorf("IgnoredConflicts = %q\nerwartet            %q", strings.Join(conflicts, " | "), want)
	}
}

// Ein ignoriertes Verzeichnis mit Inhalt, an dessen Stelle das Ziel eine Datei
// versioniert, blockiert: git switch löscht das Verzeichnis samt Inhalt still.
func TestVorpruefungIgnoriertesVerzeichnisImZiel(t *testing.T) {
	f := newFixture(t)
	ctx := testContext(t)

	run(t, f.work, "switch", "-q", "-c", "cache-als-datei")
	commit(t, f.work, "cache", "versionierte Datei\n", "cache als Datei")
	run(t, f.work, "switch", "-q", "main")
	commit(t, f.work, ".gitignore", "cache/\n", "ignoriert cache/")
	if err := os.MkdirAll(filepath.Join(f.work, "cache"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Leer ist es kein Konflikt: git räumt es ohne Verlust weg.
	expectResult(t, RunCheck(ctx, f.checkOptions("cache-als-datei")), "ignorierte-dateien", ResultOK)

	writeFile(t, filepath.Join(f.work, "cache", "lokal.bin"), "lokal")
	result := RunCheck(ctx, f.checkOptions("cache-als-datei"))
	check := expectResult(t, result, "ignorierte-dateien", ResultBlocked)
	if result.Offered || len(check.Details) != 1 || !strings.HasPrefix(check.Details[0], "cache (") {
		t.Errorf("Offered = %v, Details = %v", result.Offered, check.Details)
	}
}

func TestVorpruefungLaufendeOperation(t *testing.T) {
	f := newFixture(t)
	head := run(t, f.work, "rev-parse", "HEAD")
	gitDir := run(t, f.work, "rev-parse", "--absolute-git-dir")
	writeFile(t, filepath.Join(gitDir, "MERGE_HEAD"), head+"\n")
	result := RunCheck(testContext(t), f.checkOptions("feature/b"))
	check := expectResult(t, result, "git-operation", ResultBlocked)
	if result.Offered || !strings.Contains(check.Reason, "merge") {
		t.Errorf("Offered = %v, Reason = %s", result.Offered, check.Reason)
	}
}

// Die Pfade kommen aus --git-path: in einem weiteren Worktree liegt MERGE_HEAD
// nicht unter .git des Hauptverzeichnisses.
func TestVorpruefungOperationImWorktree(t *testing.T) {
	f := newFixture(t)
	other := filepath.Join(f.root, "wt")
	run(t, f.work, "worktree", "add", "-q", other, "feature/b")
	gitDir := run(t, other, "rev-parse", "--absolute-git-dir")
	writeFile(t, filepath.Join(gitDir, "REVERT_HEAD"), run(t, other, "rev-parse", "HEAD")+"\n")

	options := f.checkOptions("development")
	options.ProjectDir, options.RepoDir = other, other
	expectResult(t, RunCheck(testContext(t), options), "git-operation", ResultBlocked)
	// Das Hauptverzeichnis ist davon nicht betroffen.
	expectResult(t, RunCheck(testContext(t), f.checkOptions("development")), "git-operation", ResultOK)
}

func TestVorpruefungZielInAnderemWorktree(t *testing.T) {
	f := newFixture(t)
	ctx := testContext(t)
	other := filepath.Join(f.root, "wt")
	run(t, f.work, "worktree", "add", "-q", other, "feature/b")

	check := expectResult(t, RunCheck(ctx, f.checkOptions("feature/b")), "worktree", ResultBlocked)
	if strings.Contains(check.Reason, "prune") {
		t.Errorf("vorhandener Worktree nennt prune: %s", check.Reason)
	}
	if err := os.RemoveAll(other); err != nil {
		t.Fatal(err)
	}
	check = expectResult(t, RunCheck(ctx, f.checkOptions("feature/b")), "worktree", ResultBlocked)
	if !strings.Contains(check.Reason, "git worktree prune") {
		t.Errorf("prunable ohne Hinweis auf prune: %s", check.Reason)
	}
}

func TestVorpruefungDetachedHead(t *testing.T) {
	f := newFixture(t)
	ctx := testContext(t)

	run(t, f.work, "switch", "-q", "--detach", "HEAD")
	expectResult(t, RunCheck(ctx, f.checkOptions("feature/b")), "detached-head", ResultOK)

	commit(t, f.work, "lose.txt", "lose\n", "nur losgelöst")
	result := RunCheck(ctx, f.checkOptions("feature/b"))
	expectResult(t, result, "detached-head", ResultBlocked)
	if result.Offered {
		t.Error("unerreichbarer Detached HEAD angeboten")
	}
}

// Sitzungen im Projektverzeichnis und im Code-Repo blockieren, mit Art, PID
// und Verzeichnis. Ein VS-Code-Server ohne KI-Erweiterung ist ein Hinweis; mit
// Erweiterung blockiert deren Prozess.
func TestVorpruefungSitzungen(t *testing.T) {
	f := newFixture(t)
	ctx := testContext(t)
	projectDir := f.root // enthält das Code-Repo work
	own := os.Getpid()

	vscodeNode := Process{PID: 100, PPID: 1, Cwd: projectDir, Cmdline: []string{"/home/u/.vscode-server/bin/abc/node", "/home/u/.vscode-server/bin/abc/out/server-main.js"}}
	extension := Process{PID: 101, PPID: 100, Cwd: projectDir, Cmdline: []string{"/home/u/.vscode-server/extensions/anthropic.claude-code-2.1.273-linux-x64/resources/native-binary/claude", "--output-format", "stream-json"}}
	companion := Process{PID: 102, PPID: 101, Cwd: projectDir, Cmdline: []string{"node", "/home/u/.local/bin/confluence-companion"}}
	mcp := Process{PID: 103, PPID: 101, Cwd: projectDir, Cmdline: []string{"k-playbook", "mcp"}}
	elsewhere := Process{PID: 104, PPID: 1, Cwd: "/tmp/anderswo", Cmdline: []string{"claude"}}
	ownChild := Process{PID: 105, PPID: own, Cwd: f.work, Cmdline: []string{"git", "status"}}
	self := Process{PID: own, PPID: 1, Cwd: f.work, Cmdline: []string{"k-playbook"}}
	shell := Process{PID: 106, PPID: 1, Cwd: filepath.Join(f.work, "src"), Cmdline: []string{"-bash"}}

	options := f.checkOptions("feature/b")
	options.ProjectDir = projectDir

	// Ohne KI-Erweiterung: nur Hinweise.
	options.Processes = fakeProcesses{processes: []Process{vscodeNode, elsewhere, ownChild, self, shell}}
	result := RunCheck(ctx, options)
	expectResult(t, result, "sitzungen", ResultOK)
	editor := expectResult(t, result, "editorfenster", ResultHint)
	if len(editor.Processes) != 1 || editor.Processes[0].PID != 100 {
		t.Errorf("editorfenster.Processes = %+v", editor.Processes)
	}
	others := expectResult(t, result, "andere-prozesse", ResultHint)
	if len(others.Processes) != 1 || others.Processes[0].PID != 106 {
		t.Errorf("andere-prozesse.Processes = %+v", others.Processes)
	}
	if !result.Offered {
		t.Error("Hinweise verhindern das Angebot")
	}

	// Mit Erweiterung, MCP-Server und einem Kindprozess ohne sprechenden Namen.
	options.Processes = fakeProcesses{processes: []Process{vscodeNode, extension, companion, mcp, elsewhere, ownChild, self}}
	result = RunCheck(ctx, options)
	sessions := expectResult(t, result, "sitzungen", ResultBlocked)
	if result.Offered {
		t.Error("laufende Sitzung angeboten")
	}
	kinds := map[int]string{}
	for _, process := range sessions.Processes {
		kinds[process.PID] = process.Kind
		if process.Cwd != projectDir {
			t.Errorf("PID %d: Cwd = %q", process.PID, process.Cwd)
		}
	}
	if kinds[101] != KindClaudeVSCode || kinds[103] != KindMCP || !strings.Contains(kinds[102], KindClaudeVSCode) || len(kinds) != 3 {
		t.Errorf("Arten = %v", kinds)
	}
	expectResult(t, result, "editorfenster", ResultHint)
}

// Liegt das Code-Repo in einem größeren Repo — ein Monorepo, das Projekt unter
// services/x —, schaltet der Wechsel das ganze Repo um. Eine Sitzung in der
// Wurzel oder in einem Nachbarordner blockiert deshalb ebenso.
func TestVorpruefungSitzungInDerWurzelDesArbeitsbaums(t *testing.T) {
	f := newFixture(t)
	ctx := testContext(t)
	commit(t, f.work, "services/x/main.go", "package x\n", "Dienst x")
	commit(t, f.work, "services/y/main.go", "package y\n", "Dienst y")
	projectDir := filepath.Join(f.work, "services", "x")

	options := f.checkOptions("feature/b")
	options.ProjectDir, options.RepoDir = projectDir, projectDir
	for _, cwd := range []string{f.work, filepath.Join(f.work, "services", "y")} {
		options.Processes = fakeProcesses{processes: []Process{{PID: 200, PPID: 1, Cwd: cwd, Cmdline: []string{"claude"}}}}
		result := RunCheck(ctx, options)
		check := expectResult(t, result, "sitzungen", ResultBlocked)
		if result.Offered || len(check.Processes) != 1 || check.Processes[0].PID != 200 {
			t.Errorf("%s: Offered = %v, Processes = %+v", cwd, result.Offered, check.Processes)
		}
		if !strings.Contains(check.Reason, "Wurzel des Arbeitsbaums "+resolvedPath(f.work)) {
			t.Errorf("%s: Begründung nennt die Wurzel nicht: %s", cwd, check.Reason)
		}
	}

	// Außerhalb des Arbeitsbaums bleibt es ok.
	options.Processes = fakeProcesses{processes: []Process{{PID: 201, PPID: 1, Cwd: f.root, Cmdline: []string{"claude"}}}}
	expectResult(t, RunCheck(ctx, options), "sitzungen", ResultOK)
}

// Ausgenommen sind nur der eigene Prozess und seine Kinder, die selbst keine
// Sitzung sind — die git-Aufrufe der Prüfung. Eine KI-Sitzung, die der Server
// gestartet hat, blockiert wie jede andere, samt dem, was sie gestartet hat.
func TestVorpruefungSitzungUnterDemServer(t *testing.T) {
	f := newFixture(t)
	own := os.Getpid()
	self := Process{PID: own, PPID: 1, Cwd: f.work, Cmdline: []string{"k-playbook"}}
	ownGit := Process{PID: 400, PPID: own, Cwd: f.work, Cmdline: []string{"git", "status"}}
	claude := Process{PID: 401, PPID: own, Cwd: f.work, Cmdline: []string{"claude", "-p"}}
	shell := Process{PID: 402, PPID: 401, Cwd: f.work, Cmdline: []string{"bash", "-c", "go test"}}
	opencodeMCP := Process{PID: 403, PPID: own, Cwd: f.work, Cmdline: []string{"k-playbook", "mcp"}}

	options := f.checkOptions("feature/b")
	options.Processes = fakeProcesses{processes: []Process{self, ownGit, claude, shell, opencodeMCP}}
	result := RunCheck(testContext(t), options)
	check := expectResult(t, result, "sitzungen", ResultBlocked)
	kinds := map[int]string{}
	for _, process := range check.Processes {
		kinds[process.PID] = process.Kind
	}
	if kinds[401] != KindClaude || kinds[403] != KindMCP || !strings.Contains(kinds[402], KindClaude) || len(kinds) != 3 {
		t.Errorf("Arten = %v", kinds)
	}
	if others := checkByID(t, result, "andere-prozesse"); len(others.Processes) != 0 {
		t.Errorf("eigene git-Aufrufe erscheinen: %+v", others.Processes)
	}

	// Nur die eigenen git-Aufrufe: nichts gefunden.
	options.Processes = fakeProcesses{processes: []Process{self, ownGit}}
	result = RunCheck(testContext(t), options)
	expectResult(t, result, "sitzungen", ResultOK)
	expectResult(t, result, "andere-prozesse", ResultOK)
}

// Ohne /proc ist die Sitzungsprüfung nicht prüfbar, und ohne sie wird nicht
// umgeschaltet.
func TestVorpruefungOhneProzessquelle(t *testing.T) {
	f := newFixture(t)
	options := f.checkOptions("feature/b")
	options.Processes = fakeProcesses{err: ErrNoProcessSource}
	result := RunCheck(testContext(t), options)
	check := expectResult(t, result, "sitzungen", ResultUncheckable)
	if result.Offered || !strings.Contains(check.Reason, "/proc") {
		t.Errorf("Offered = %v, Reason = %s", result.Offered, check.Reason)
	}
	// Die Hinweis-Punkte dürfen nicht prüfbar sein, ohne selbst zu blockieren.
	if check := checkByID(t, result, "editorfenster"); check.Blocking || check.Result != ResultUncheckable {
		t.Errorf("editorfenster = %+v", check)
	}
}

func TestProcSourceOhneVerzeichnis(t *testing.T) {
	if _, err := (ProcSource{Root: filepath.Join(t.TempDir(), "fehlt")}).Processes(); !errors.Is(err, ErrNoProcessSource) {
		t.Errorf("err = %v", err)
	}
}

// Unter Linux sieht die echte Quelle mindestens den eigenen Prozess.
func TestProcSourceSiehtEigenenProzess(t *testing.T) {
	if _, err := os.Stat("/proc/self"); err != nil {
		t.Skip("kein /proc")
	}
	processes, err := ProcSource{}.Processes()
	if err != nil {
		t.Fatal(err)
	}
	for _, process := range processes {
		if process.PID == os.Getpid() {
			if process.Cwd == "" || len(process.Cmdline) == 0 || process.PPID == 0 {
				t.Errorf("eigener Prozess = %+v", process)
			}
			return
		}
	}
	t.Error("eigener Prozess nicht gefunden")
}

func TestVorpruefungHinweise(t *testing.T) {
	f := newFixture(t)
	ctx := testContext(t)

	// Ungepushter Commit und ein Stash-Eintrag.
	commit(t, f.work, "lokal.txt", "lokal\n", "nicht gepusht")
	writeFile(t, filepath.Join(f.work, "README.md"), "gestasht\n")
	run(t, f.work, "stash", "push", "-q")
	result := RunCheck(ctx, f.checkOptions("feature/b"))
	expectResult(t, result, "ungepushte-commits", ResultHint)
	expectResult(t, result, "stash", ResultHint)
	if !result.Offered {
		t.Errorf("Hinweise verhindern das Angebot: %+v", result.Checks)
	}

	// Ziel hinter seinem Upstream.
	run(t, f.seed, "switch", "-q", "development")
	commit(t, f.seed, "weiter.txt", "weiter\n", "weiter auf development")
	run(t, f.seed, "push", "-q", "origin", "development")
	run(t, f.work, "fetch", "-q", "origin")
	check := expectResult(t, RunCheck(ctx, f.checkOptions("development")), "ziel-hinter-upstream", ResultHint)
	if !strings.Contains(check.Reason, "1 Commit hinter") {
		t.Errorf("Reason = %s", check.Reason)
	}
}

// Liegt die Konfiguration im umgeschalteten Repo und unterscheidet sie sich im
// Ziel, ist das ein Hinweis. Liegt sie außerhalb, ist der Punkt ok.
func TestVorpruefungKonfigurationImZiel(t *testing.T) {
	f := newFixture(t)
	ctx := testContext(t)

	expectResult(t, RunCheck(ctx, f.checkOptions("feature/b")), "konfiguration-im-ziel", ResultOK)

	run(t, f.work, "switch", "-q", "-c", "andere-konfiguration")
	commit(t, f.work, "K-PLAYBOOK.yaml", "schema_version: 3\ngit:\n  switch: off\n", "Konfiguration im Ziel")
	commit(t, f.work, "k-playbook-local/k-playbook.md", "# anders\n", "Regeln im Ziel")
	run(t, f.work, "switch", "-q", "main")
	check := expectResult(t, RunCheck(ctx, f.checkOptions("andere-konfiguration")), "konfiguration-im-ziel", ResultHint)
	if len(check.Details) != 2 {
		t.Errorf("Details = %v", check.Details)
	}

	options := f.checkOptions("andere-konfiguration")
	options.ProjectDir = f.root
	check = expectResult(t, RunCheck(ctx, options), "konfiguration-im-ziel", ResultOK)
	if !strings.Contains(check.Reason, "außerhalb") {
		t.Errorf("Reason = %s", check.Reason)
	}
}

// Der Prüfstempel ändert sich, sobald sich der Arbeitsbaum ändert.
func TestVorpruefungStempel(t *testing.T) {
	f := newFixture(t)
	ctx := testContext(t)
	first := RunCheck(ctx, f.checkOptions("feature/b")).Stamp
	if again := RunCheck(ctx, f.checkOptions("feature/b")).Stamp; again != first {
		t.Errorf("Stempel ohne Änderung verschieden: %s / %s", first, again)
	}
	writeFile(t, filepath.Join(f.work, "README.md"), "anders\n")
	if changed := RunCheck(ctx, f.checkOptions("feature/b")).Stamp; changed == first {
		t.Error("Stempel bleibt trotz geänderter Datei gleich")
	}
}

// Ein angebundener DevContainer ist auf dem Host nur als docker exec sichtbar.
// Der Hinweis nennt ihn, blockiert aber nicht.
func TestVorpruefungNenntAngebundenenContainer(t *testing.T) {
	f := newFixture(t)
	options := f.checkOptions("feature/b")
	options.Processes = fakeProcesses{processes: []Process{
		{PID: 300, PPID: 1, Cwd: f.work, Cmdline: []string{"docker", "exec", "-i", "-u", "vscode", "-e", "VSCODE_REMOTE_CONTAINERS_SESSION=x", "abc", "sh"}},
	}}
	result := RunCheck(testContext(t), options)
	check := expectResult(t, result, "unsichtbare-sitzungen", ResultHint)
	if !strings.Contains(check.Reason, "Container angebunden") || !result.Offered {
		t.Errorf("Reason = %s, Offered = %v", check.Reason, result.Offered)
	}
}
