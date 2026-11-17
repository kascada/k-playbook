package project

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
	"github.com/kascada/k-playbook/installer/internal/kephalaion/kephtest"
)

// fakePull spielt `kephalaion node dir pull` gegen einen Store im Speicher:
// schreibt, was fehlt oder abweicht, löscht mit --delete, was im Store fehlt,
// und lässt .git und Treffer von --exclude unberührt. Kein Node, kein Netz.
type fakePull struct {
	store     map[string]string
	code      int
	stderr    string
	interrupt bool
	// limit > 0 schreibt nur so viele Dateien und endet dann (partieller
	// Stand).
	limit int
	calls [][]string
}

func (f *fakePull) install(t *testing.T) {
	t.Helper()
	original := maintenanceExport
	maintenanceExport = f.run
	t.Cleanup(func() { maintenanceExport = original })
}

func (f *fakePull) run(_ context.Context, _ string, args []string, stdout, _ io.Writer) exportOutcome {
	f.calls = append(f.calls, append([]string{}, args...))
	target := args[5]
	var excludes []string
	dryRun, del := false, false
	for index, arg := range args {
		switch arg {
		case "--exclude":
			excludes = append(excludes, args[index+1])
		case "--dry-run":
			dryRun = true
		case "--delete":
			del = true
		}
	}
	if dryRun {
		fmt.Fprintln(stdout, "(Vorschau)")
		return exportOutcome{code: f.code, stderr: f.stderr, interrupted: f.interrupt}
	}
	names := make([]string, 0, len(f.store))
	for name := range f.store {
		names = append(names, name)
	}
	sort.Strings(names)
	for index, name := range names {
		if f.limit > 0 && index >= f.limit {
			return exportOutcome{code: f.code, stderr: f.stderr, interrupted: f.interrupt}
		}
		path := filepath.Join(target, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return exportOutcome{code: 1, stderr: err.Error()}
		}
		if err := os.WriteFile(path, []byte(f.store[name]), 0o644); err != nil {
			return exportOutcome{code: 1, stderr: err.Error()}
		}
	}
	if del {
		_ = filepath.WalkDir(target, func(path string, entry os.DirEntry, err error) error {
			if err != nil || path == target {
				return err
			}
			base := entry.Name()
			if base == ".git" {
				return filepath.SkipDir
			}
			for _, pattern := range excludes {
				if ok, _ := filepath.Match(pattern, base); ok {
					if entry.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}
			if entry.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(target, path)
			if _, ok := f.store[filepath.ToSlash(rel)]; !ok {
				return os.Remove(path)
			}
			return nil
		})
	}
	return exportOutcome{code: f.code, stderr: f.stderr, interrupted: f.interrupt}
}

// isolateGit hält git von der Konfiguration des Rechners fern und gibt ihm
// eine feste Identität.
func isolateGit(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.invalid")
	t.Setenv("GIT_COMMITTER_NAME", "Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.invalid")
}

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// stubMaintenanceKephalaion lässt den Status bereit oder nicht bereit melden,
// ohne einen Node zu fragen, und setzt die Sperren in ein Temp-Verzeichnis.
func stubMaintenanceKephalaion(t *testing.T, ok bool) {
	t.Helper()
	originalCheck, originalBinary, originalLock := kephalaionCheck, maintenanceKephalaionBinary, maintenanceLockDir
	status := kephalaion.Status{Configured: true, Hub: "vm", Collection: "proj", Address: "vm:proj", State: kephalaion.StateOK, OK: true}
	if !ok {
		status = kephalaion.Status{Configured: true, State: kephalaion.State("unreachable"), Message: "Node nicht erreichbar."}
	}
	kephalaionCheck = func(string) kephalaion.Status { return status }
	maintenanceKephalaionBinary = func() (string, bool) { return "kephalaion", true }
	locks := t.TempDir()
	maintenanceLockDir = func() string { return locks }
	t.Cleanup(func() {
		kephalaionCheck, maintenanceKephalaionBinary, maintenanceLockDir = originalCheck, originalBinary, originalLock
	})
}

// backupProject legt ein Git-Repository mit Konfiguration an; block ersetzt
// den Block maintenance: (leer: Sicherung an, kein Commit).
func backupProject(t *testing.T, block string) string {
	t.Helper()
	isolateGit(t)
	stubMaintenanceKephalaion(t, true)
	root := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(root))
	if block == "" {
		block = "maintenance:\n  backup:\n    enabled: true\n"
	}
	writeFile(t, ConfigPath(root), "schema_version: 3\n\nproject:\n  repo_root: .\n  vcs: git\n\nkephalaion:\n  hub: vm\n  collection: proj\n\n"+block)
	gitIn(t, root, "init", "-q")
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "-q", "-m", "Start")
	return root
}

func runBackup(t *testing.T, root string, dryRun, commit bool) (MaintenanceStepResult, string) {
	t.Helper()
	config, err := ReadConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	// Getrennte Puffer: exec schreibt stdout und stderr nebenläufig.
	var out, errOut bytes.Buffer
	run := &maintenanceRun{ctx: context.Background(), projectDir: root, config: config, dryRun: dryRun, commit: commit, out: &out, errOut: &errOut}
	result := runDocsBackup(run)
	return result, out.String() + errOut.String()
}

func headOf(t *testing.T, root string) string {
	t.Helper()
	return strings.TrimSpace(gitIn(t, root, "rev-parse", "HEAD"))
}

func assertNoCommit(t *testing.T, root, before string) {
	t.Helper()
	if after := headOf(t, root); after != before {
		t.Errorf("unerwarteter Commit: %s → %s", before, after)
	}
}

func targetDir(root string) string { return filepath.Join(root, "keph-backup", "docs") }

// Erfolg ohne angeforderten Commit: exportiert, Markierung angelegt, Aufruf
// mit Adresse, Quelle, geprüftem Ziel, --delete und --exclude; kein Commit.
func TestDocsBackupErfolgOhneCommit(t *testing.T) {
	root := backupProject(t, "")
	pull := &fakePull{store: map[string]string{"index.md": "# Index\n", "manual/a.md": "A\n"}}
	pull.install(t)
	before := headOf(t, root)

	result, out := runBackup(t, root, false, false)
	if result.Status != MaintenanceOK || !strings.Contains(result.Reason, "kein Commit angefordert") {
		t.Fatalf("Ergebnis: %+v", result)
	}
	if len(result.Notes) != 1 || !strings.Contains(result.Notes[0], "Erster Lauf") {
		t.Errorf("Hinweise: %v", result.Notes)
	}
	want := []string{"node", "dir", "pull", "vm:proj", "docs", targetDir(root), "--delete", "--exclude", MaintenanceBackupMarker}
	if len(pull.calls) != 1 || !reflect.DeepEqual(pull.calls[0], want) {
		t.Errorf("Aufruf: %v, erwartet %v", pull.calls, want)
	}
	if !strings.Contains(out, "docs-backup: Export vm:proj docs/ → keph-backup/docs") {
		t.Errorf("Ausgabe: %s", out)
	}
	for _, name := range []string{MaintenanceBackupMarker, "index.md", "manual/a.md"} {
		if _, err := os.Stat(filepath.Join(targetDir(root), name)); err != nil {
			t.Errorf("%s fehlt: %v", name, err)
		}
	}
	assertNoCommit(t, root, before)
}

// Mit Commit: erst ein Commit nur des Ziels, ein unveränderter
// Wiederholungslauf committet nichts; Löschungen kommen in den nächsten
// Commit, die Markierung bleibt trotz --delete.
func TestDocsBackupCommitWiederholungUndLoeschung(t *testing.T) {
	root := backupProject(t, "maintenance:\n  backup:\n    enabled: true\n    git:\n      commit: true\n")
	pull := &fakePull{store: map[string]string{"a.md": "A\n", "b.md": "B\n"}}
	pull.install(t)
	start := headOf(t, root)

	result, _ := runBackup(t, root, false, false)
	if result.Status != MaintenanceOK || !strings.Contains(result.Reason, "committet") || !strings.Contains(result.Reason, "maintenance.backup.git.commit") {
		t.Fatalf("erster Lauf: %+v", result)
	}
	first := headOf(t, root)
	if first == start {
		t.Fatal("kein Commit")
	}
	files := strings.Fields(gitIn(t, root, "show", "--name-only", "--format=", "HEAD"))
	if want := []string{"keph-backup/docs/.k-playbook-backup", "keph-backup/docs/a.md", "keph-backup/docs/b.md"}; !reflect.DeepEqual(files, want) {
		t.Errorf("Commit enthält %v, erwartet %v", files, want)
	}
	if subject := gitIn(t, root, "log", "-1", "--format=%s"); !strings.Contains(subject, "Doku-Sicherung: keph:/docs/ aus vm:proj") {
		t.Errorf("Betreff: %s", subject)
	}

	result, _ = runBackup(t, root, false, false)
	if result.Status != MaintenanceOK || !strings.Contains(result.Reason, "keine Änderungen, kein Commit") || len(result.Notes) != 0 {
		t.Fatalf("Wiederholung: %+v", result)
	}
	assertNoCommit(t, root, first)

	delete(pull.store, "b.md")
	result, _ = runBackup(t, root, false, true)
	if result.Status != MaintenanceOK || !strings.Contains(result.Reason, "--commit") {
		t.Fatalf("Löschung: %+v", result)
	}
	if _, err := os.Stat(filepath.Join(targetDir(root), "b.md")); !os.IsNotExist(err) {
		t.Errorf("b.md nicht gelöscht: %v", err)
	}
	if _, err := os.Stat(filepath.Join(targetDir(root), MaintenanceBackupMarker)); err != nil {
		t.Errorf("Markierung weg: %v", err)
	}
	status := gitIn(t, root, "show", "--name-status", "--format=", "HEAD")
	if strings.TrimSpace(status) != "D\tkeph-backup/docs/b.md" {
		t.Errorf("Commit der Löschung: %q", status)
	}
}

// Die Vorschau ruft den Export mit --dry-run auf, legt weder Ziel noch
// Markierung an, ändert weder Git noch Konfiguration und nennt den Commit.
func TestDocsBackupVorschau(t *testing.T) {
	root := backupProject(t, "")
	pull := &fakePull{store: map[string]string{"a.md": "A\n"}}
	pull.install(t)
	before := headOf(t, root)
	config := readConfigFile(t, root)

	result, _ := runBackup(t, root, true, true)
	if result.Status != MaintenanceOK || result.Reason != "Vorschau: nichts geschrieben" {
		t.Fatalf("Vorschau: %+v", result)
	}
	if len(result.Notes) != 2 || !strings.Contains(result.Notes[0], "würden angelegt") || !strings.Contains(result.Notes[1], "Commit wäre angefordert (--commit)") {
		t.Errorf("Hinweise: %v", result.Notes)
	}
	if len(pull.calls) != 1 || pull.calls[0][len(pull.calls[0])-1] != "--dry-run" {
		t.Errorf("Aufruf: %v", pull.calls)
	}
	if _, err := os.Stat(filepath.Join(root, "keph-backup")); !os.IsNotExist(err) {
		t.Errorf("Ziel angelegt: %v", err)
	}
	assertNoCommit(t, root, before)
	if readConfigFile(t, root) != config {
		t.Error("Konfiguration geändert")
	}
	if status := gitIn(t, root, "status", "--porcelain"); status != "" {
		t.Errorf("Arbeitsbaum geändert: %s", status)
	}

	result, _ = runBackup(t, root, true, false)
	if len(result.Notes) != 2 || result.Notes[1] != "Kein Commit angefordert." {
		t.Errorf("ohne Commit: %v", result.Notes)
	}
}

// Exit 1, 2 und 3 melden nach der Tabelle, nennen den möglicherweise
// partiellen Stand und committen nie, auch wenn ein Commit angefordert ist.
func TestDocsBackupExitCodes(t *testing.T) {
	for _, tc := range []struct {
		code   int
		stderr string
		want   string
	}{
		{1, "Fehler: forbidden: kein Recht auf docs\n", "Fehler bei Kephalaion: Fehler: forbidden: kein Recht auf docs — kein Commit"},
		{2, "", "Kephalaion zu alt oder falscher Aufruf → /k-upgrade — kein Commit"},
		{3, "", "partiell, erneut ausführen, kein Commit"},
		{7, "", "unbekanntem Exit-Code 7"},
	} {
		t.Run(fmt.Sprint(tc.code), func(t *testing.T) {
			root := backupProject(t, "")
			pull := &fakePull{store: map[string]string{"a.md": "A\n"}, code: tc.code, stderr: tc.stderr}
			pull.install(t)
			before := headOf(t, root)

			result, _ := runBackup(t, root, false, true)
			if result.Status != MaintenanceFailed || !strings.Contains(result.Reason, tc.want) {
				t.Fatalf("Ergebnis: %+v, erwartet %q", result, tc.want)
			}
			if !strings.Contains(strings.Join(result.Notes, " "), "teilweise aktualisiert") {
				t.Errorf("partieller Stand nicht genannt: %v", result.Notes)
			}
			assertNoCommit(t, root, before)
			if staged := gitIn(t, root, "diff", "--cached", "--name-only"); staged != "" {
				t.Errorf("vorgemerkt: %s", staged)
			}
		})
	}
}

// Ein Abbruch mitten im Export lässt einen partiellen Stand zurück: gemeldet,
// nicht committet.
func TestDocsBackupAbbruchPartiell(t *testing.T) {
	root := backupProject(t, "")
	pull := &fakePull{store: map[string]string{"a.md": "A\n", "b.md": "B\n"}, code: 3, interrupt: true, limit: 1}
	pull.install(t)
	before := headOf(t, root)

	result, _ := runBackup(t, root, false, true)
	if result.Status != MaintenanceFailed || !strings.Contains(result.Reason, "abgebrochen") || !strings.Contains(result.Reason, "kein Commit") {
		t.Fatalf("Ergebnis: %+v", result)
	}
	if _, err := os.Stat(filepath.Join(targetDir(root), "a.md")); err != nil {
		t.Errorf("partielle Datei fehlt: %v", err)
	}
	if _, err := os.Stat(filepath.Join(targetDir(root), "b.md")); !os.IsNotExist(err) {
		t.Errorf("b.md dürfte nicht da sein: %v", err)
	}
	assertNoCommit(t, root, before)
}

// Ist Kephalaion nicht bereit, wird nichts exportiert.
func TestDocsBackupKephalaionNichtBereit(t *testing.T) {
	root := backupProject(t, "")
	stubMaintenanceKephalaion(t, false)
	pull := &fakePull{store: map[string]string{"a.md": "A\n"}}
	pull.install(t)

	result, _ := runBackup(t, root, false, true)
	if result.Status != MaintenanceFailed || !strings.Contains(result.Reason, "Kephalaion ist nicht bereit") || !strings.Contains(result.Reason, "Node nicht erreichbar") {
		t.Fatalf("Ergebnis: %+v", result)
	}
	if len(pull.calls) != 0 {
		t.Errorf("Export aufgerufen: %v", pull.calls)
	}
	if _, err := os.Stat(filepath.Join(root, "keph-backup")); !os.IsNotExist(err) {
		t.Errorf("Ziel angelegt: %v", err)
	}
}

// Deaktiviert oder ohne Block: übersprungen, kein Aufruf — auch nicht mit
// --commit.
func TestDocsBackupDeaktiviertUebersprungen(t *testing.T) {
	for name, block := range map[string]string{
		"deaktiviert": "maintenance:\n  backup:\n    enabled: false\n    git:\n      commit: true\n",
		"ohne Block":  "# kein Block\n",
	} {
		t.Run(name, func(t *testing.T) {
			root := backupProject(t, block)
			pull := &fakePull{store: map[string]string{"a.md": "A\n"}}
			pull.install(t)
			result, _ := runBackup(t, root, false, true)
			if result.Status != MaintenanceSkipped || !strings.Contains(result.Reason, "deaktiviert") || len(pull.calls) != 0 {
				t.Errorf("Ergebnis: %+v, Aufrufe %v", result, pull.calls)
			}
		})
	}
}

// Ein nicht leeres Ziel ohne Markierung gehört nicht dem Export: Abbruch vor
// dem Export, die fremde Datei bleibt.
func TestDocsBackupFremdesZielOhneMarkierung(t *testing.T) {
	root := backupProject(t, "")
	writeFile(t, filepath.Join(targetDir(root), "eigen.md"), "eigen\n")
	pull := &fakePull{store: map[string]string{"a.md": "A\n"}}
	pull.install(t)

	result, _ := runBackup(t, root, false, false)
	if result.Status != MaintenanceFailed || !strings.Contains(result.Reason, "keine Markierung") || len(pull.calls) != 0 {
		t.Fatalf("Ergebnis: %+v, Aufrufe %v", result, pull.calls)
	}
	if data, err := os.ReadFile(filepath.Join(targetDir(root), "eigen.md")); err != nil || string(data) != "eigen\n" {
		t.Errorf("fremde Datei verändert: %q %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(targetDir(root), MaintenanceBackupMarker)); !os.IsNotExist(err) {
		t.Errorf("Markierung angelegt: %v", err)
	}

	// Ein leeres vorhandenes Ziel ist ein erster Lauf.
	root = backupProject(t, "")
	if err := os.MkdirAll(targetDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	pull.install(t)
	if result, _ := runBackup(t, root, false, false); result.Status != MaintenanceOK {
		t.Errorf("leeres Ziel: %+v", result)
	}
}

// Ein gefährliches Ziel wird vor dem Export abgelehnt.
func TestDocsBackupGefaehrlichesZiel(t *testing.T) {
	root := backupProject(t, "maintenance:\n  backup:\n    enabled: true\n    target: .k-playbook\n")
	pull := &fakePull{store: map[string]string{"a.md": "A\n"}}
	pull.install(t)
	result, _ := runBackup(t, root, false, false)
	if result.Status != MaintenanceFailed || !strings.Contains(result.Reason, "ist .k-playbook/") || len(pull.calls) != 0 {
		t.Errorf("Ergebnis: %+v, Aufrufe %v", result, pull.calls)
	}
}

// Ein paralleler Lauf auf dasselbe Ziel wird sicher abgelehnt.
func TestDocsBackupParallelAbgelehnt(t *testing.T) {
	root := backupProject(t, "")
	pull := &fakePull{store: map[string]string{"a.md": "A\n"}}
	pull.install(t)

	unlock, err := lockMaintenanceTarget(targetDir(root))
	if err != nil {
		t.Fatalf("Sperre: %v", err)
	}
	result, _ := runBackup(t, root, false, false)
	if result.Status != MaintenanceFailed || !strings.Contains(result.Reason, "anderer Wartungslauf") || len(pull.calls) != 0 {
		t.Fatalf("parallel: %+v, Aufrufe %v", result, pull.calls)
	}
	unlock()
	if result, _ := runBackup(t, root, false, false); result.Status != MaintenanceOK {
		t.Errorf("nach Freigabe: %+v", result)
	}
}

// Fremde vorgemerkte und unversionierte Änderungen bleiben, wie sie sind, und
// gelangen nicht in den Commit.
func TestDocsBackupCommitNurZiel(t *testing.T) {
	root := backupProject(t, "")
	pull := &fakePull{store: map[string]string{"a.md": "A\n"}}
	pull.install(t)
	writeFile(t, filepath.Join(root, "fremd.txt"), "vorgemerkt\n")
	gitIn(t, root, "add", "fremd.txt")
	writeFile(t, filepath.Join(root, "lose.txt"), "unversioniert\n")
	writeFile(t, ConfigPath(root), readConfigFile(t, root)+"# geändert, nicht vorgemerkt\n")

	result, _ := runBackup(t, root, false, true)
	if result.Status != MaintenanceOK || !strings.Contains(result.Reason, "committet") {
		t.Fatalf("Ergebnis: %+v", result)
	}
	files := strings.Fields(gitIn(t, root, "show", "--name-only", "--format=", "HEAD"))
	for _, file := range files {
		if !strings.HasPrefix(file, "keph-backup/docs/") {
			t.Errorf("fremde Datei im Commit: %s", file)
		}
	}
	status := gitIn(t, root, "status", "--porcelain")
	for _, want := range []string{"A  fremd.txt", "?? lose.txt", " M .k-playbook/K-PLAYBOOK.yaml"} {
		if !strings.Contains(status, want) {
			t.Errorf("Status ohne %q:\n%s", want, status)
		}
	}
	if strings.Contains(status, "keph-backup") {
		t.Errorf("Ziel nicht vollständig committet:\n%s", status)
	}
}

// Ein scheiternder Commit (Hook) ist ein Git-Fehler; der Export ist gelaufen,
// die Änderungen im Ziel bleiben vorgemerkt.
func TestDocsBackupCommitFehler(t *testing.T) {
	root := backupProject(t, "")
	pull := &fakePull{store: map[string]string{"a.md": "A\n"}}
	pull.install(t)
	hook := filepath.Join(root, ".git", "hooks", "pre-commit")
	writeFile(t, hook, "#!/bin/sh\necho 'Hook lehnt ab' >&2\nexit 1\n")
	if err := os.Chmod(hook, 0o755); err != nil {
		t.Fatal(err)
	}
	before := headOf(t, root)

	result, _ := runBackup(t, root, false, true)
	if result.Status != MaintenanceFailed || !strings.Contains(result.Reason, "Commit fehlgeschlagen: Hook lehnt ab") {
		t.Fatalf("Ergebnis: %+v", result)
	}
	assertNoCommit(t, root, before)
	if _, err := os.Stat(filepath.Join(targetDir(root), "a.md")); err != nil {
		t.Errorf("Export nicht gelaufen: %v", err)
	}
	if staged := gitIn(t, root, "diff", "--cached", "--name-only"); !strings.Contains(staged, "keph-backup/docs/a.md") {
		t.Errorf("nicht vorgemerkt: %q", staged)
	}
}

// Ohne Git oder bei ignoriertem Ziel ist ein angeforderter Commit ein
// gemeldeter Fehler; der Export läuft trotzdem.
func TestDocsBackupCommitOhneGitOderIgnoriert(t *testing.T) {
	root := backupProject(t, "")
	pull := &fakePull{store: map[string]string{"a.md": "A\n"}}
	pull.install(t)
	writeFile(t, filepath.Join(root, ".gitignore"), "keph-backup/\n")
	result, _ := runBackup(t, root, false, true)
	if result.Status != MaintenanceFailed || !strings.Contains(result.Reason, "Git ignoriert das Ziel keph-backup/docs") {
		t.Errorf("ignoriert: %+v", result)
	}
	if _, err := os.Stat(filepath.Join(targetDir(root), "a.md")); err != nil {
		t.Errorf("Export nicht gelaufen: %v", err)
	}

	root = backupProject(t, "")
	if err := os.RemoveAll(filepath.Join(root, ".git")); err != nil {
		t.Fatal(err)
	}
	result, _ = runBackup(t, root, false, true)
	if result.Status != MaintenanceFailed || !strings.Contains(result.Reason, "kein Git-Repository") {
		t.Errorf("ohne Git: %+v", result)
	}
	if _, err := os.Stat(filepath.Join(targetDir(root), "a.md")); err != nil {
		t.Errorf("Export nicht gelaufen: %v", err)
	}
}

// Der echte Aufruf geht ohne Shell an das Binary: die Argumente kommen genau
// so an, auch mit Leerzeichen und $(…) im Ziel, und Exit 3 wird ausgewertet.
func TestDocsBackupEchterAufrufOhneShell(t *testing.T) {
	root := backupProject(t, "maintenance:\n  backup:\n    enabled: true\n    target: \"sich rung/$(touch boom)\"\n")
	fake := kephtest.NewFakeClient(t)
	maintenanceKephalaionBinary = kephalaion.FindBinary
	fake.Set(t, "+ a.md\n", "unvollständig: 1 gemeldet\n", 3)

	result, out := runBackup(t, root, false, true)
	if result.Status != MaintenanceFailed || !strings.Contains(result.Reason, "partiell") {
		t.Fatalf("Ergebnis: %+v", result)
	}
	want := []string{"node", "dir", "pull", "vm:proj", "docs", filepath.Join(root, "sich rung", "$(touch boom)"),
		"--delete", "--exclude", MaintenanceBackupMarker}
	if got := fake.Args(t); !reflect.DeepEqual(got, want) {
		t.Errorf("Argumente: %q, erwartet %q", got, want)
	}
	if !strings.Contains(out, "+ a.md") || !strings.Contains(out, "unvollständig") {
		t.Errorf("Ausgabe nicht durchgereicht: %s", out)
	}
	if _, err := os.Stat(filepath.Join(root, "boom")); !os.IsNotExist(err) {
		t.Error("Shell hat das Argument ausgewertet")
	}

	fake.Set(t, "", "Fehler: Node nicht erreichbar\n", 1)
	result, _ = runBackup(t, root, false, true)
	if !strings.Contains(result.Reason, "Fehler bei Kephalaion: Fehler: Node nicht erreichbar") {
		t.Errorf("Exit 1: %+v", result)
	}
}

// Ein SIGTERM an den Wartungslauf geht an Kephalaion weiter; der Lauf selbst
// endet nicht, sondern meldet den Abbruch.
func TestRunKephalaionExportReichtSignalWeiter(t *testing.T) {
	dir := t.TempDir()
	started := filepath.Join(dir, "gestartet")
	script := filepath.Join(dir, "kephalaion")
	writeFile(t, script, "#!/bin/sh\ntrap 'echo abgebrochen >&2; exit 3' TERM\ntouch '"+started+"'\nwhile :; do sleep 0.05; done\n")
	if err := os.Chmod(script, 0o755); err != nil {
		t.Fatal(err)
	}

	done := make(chan exportOutcome, 1)
	go func() {
		done <- runKephalaionExport(context.Background(), script, nil, io.Discard, io.Discard)
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(started); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("Export nicht gestartet")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case outcome := <-done:
		if !outcome.interrupted || outcome.code != 3 || !strings.Contains(outcome.stderr, "abgebrochen") {
			t.Errorf("Ausgang: %+v", outcome)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Export endete nicht")
	}
}

// Ein Binary, das nicht startet, ist ein offener Ausgang.
func TestRunKephalaionExportOhneBinary(t *testing.T) {
	outcome := runKephalaionExport(context.Background(), filepath.Join(t.TempDir(), "fehlt"), nil, io.Discard, io.Discard)
	if outcome.code != -1 || outcome.err == nil {
		t.Errorf("Ausgang: %+v", outcome)
	}
}

// RunMaintenance: unbekannte Namen in --only und maintenance.daily werden vor
// jedem Schreiben abgelehnt; die Meldung nennt die gültigen Namen.
func TestRunMaintenanceUngueltigeSchritte(t *testing.T) {
	root := backupProject(t, "")
	pull := &fakePull{store: map[string]string{"a.md": "A\n"}}
	pull.install(t)

	_, err := RunMaintenance(context.Background(), root, MaintenanceOptions{Only: []string{"docs-backup", "queues"}})
	var selection *MaintenanceSelectionError
	if !errors.As(err, &selection) || !strings.Contains(err.Error(), "queues") || !strings.Contains(err.Error(), "gültig: docs-backup") {
		t.Errorf("--only: %v", err)
	}
	_, err = RunMaintenance(context.Background(), root, MaintenanceOptions{Only: []string{" ", ""}})
	if !errors.As(err, &selection) {
		t.Errorf("--only leer: %v", err)
	}

	writeFile(t, ConfigPath(root), readConfigFile(t, root)+"  daily:\n    - docs-backup\n    - aufraeumen\n")
	_, err = RunMaintenance(context.Background(), root, MaintenanceOptions{})
	if !errors.As(err, &selection) || !strings.Contains(err.Error(), "maintenance.daily: aufraeumen") {
		t.Errorf("daily: %v", err)
	}
	if len(pull.calls) != 0 {
		t.Errorf("Export trotz Ablehnung: %v", pull.calls)
	}
	if _, err := os.Stat(filepath.Join(root, "keph-backup")); !os.IsNotExist(err) {
		t.Errorf("geschrieben trotz Ablehnung: %v", err)
	}
}

// Die einzeilige Form von daily bricht vor jedem Schreiben ab.
func TestRunMaintenanceDailyEinzeilig(t *testing.T) {
	root := backupProject(t, "maintenance:\n  daily: [docs-backup]\n  backup:\n    enabled: true\n")
	pull := &fakePull{store: map[string]string{"a.md": "A\n"}}
	pull.install(t)
	_, err := RunMaintenance(context.Background(), root, MaintenanceOptions{})
	if err == nil || !strings.Contains(err.Error(), "Wert in einer Zeile") || len(pull.calls) != 0 {
		t.Errorf("einzeilig: %v, Aufrufe %v", err, pull.calls)
	}
}

// Tagesprogramm, Vorgabe und Einzelauswahl; ein leeres Tagesprogramm führt
// nichts aus.
func TestRunMaintenanceAuswahl(t *testing.T) {
	root := backupProject(t, "")
	pull := &fakePull{store: map[string]string{"a.md": "A\n"}}
	pull.install(t)

	result, err := RunMaintenance(context.Background(), root, MaintenanceOptions{})
	if err != nil || result.Selection != "Vorgabe des Tagesprogramms" || len(result.Steps) != 1 || result.Steps[0].Step != MaintenanceStepDocsBackup || result.Failed {
		t.Errorf("Vorgabe: %+v %v", result, err)
	}
	result, err = RunMaintenance(context.Background(), root, MaintenanceOptions{Only: []string{"docs-backup", "docs-backup"}})
	if err != nil || result.Selection != "--only" || len(result.Steps) != 1 {
		t.Errorf("--only: %+v %v", result, err)
	}

	writeFile(t, ConfigPath(root), readConfigFile(t, root)+"  daily: []\n")
	result, err = RunMaintenance(context.Background(), root, MaintenanceOptions{})
	if err != nil || result.Selection != "maintenance.daily" || len(result.Steps) != 0 || result.Failed {
		t.Errorf("leeres Tagesprogramm: %+v %v", result, err)
	}

	stubMaintenanceKephalaion(t, false)
	result, err = RunMaintenance(context.Background(), root, MaintenanceOptions{Only: []string{"docs-backup"}})
	if err != nil || !result.Failed || result.Steps[0].Status != MaintenanceFailed {
		t.Errorf("Fehler: %+v %v", result, err)
	}
}

// Die Schrittnamen stehen genau einmal: Liste, Vorgabe und Hilfe stimmen
// überein.
func TestMaintenanceSchrittnamen(t *testing.T) {
	if names := MaintenanceStepNames(); !reflect.DeepEqual(names, []string{MaintenanceStepDocsBackup}) {
		t.Errorf("Namen: %v", names)
	}
	for _, name := range DefaultMaintenanceDaily {
		if !containsString(MaintenanceStepNames(), name) {
			t.Errorf("Vorgabe nennt unbekannten Schritt %s", name)
		}
	}
	if infos := MaintenanceSteps(); len(infos) != 1 || infos[0].Summary == "" {
		t.Errorf("Infos: %+v", infos)
	}
}
