package project

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
)

// Der Schritt docs-backup (Task 161) spiegelt keph:/docs/ über den Export von
// Kephalaion (`kephalaion node dir pull … --delete`) in einen Ordner des
// Repositorys und committet ihn auf Wunsch. Er ist kein Collection-Backup und
// kein Ersatz für Datenbank- oder Hub-Backups.

// maintenanceBackupMarkerContent steht in der Markierung des Ziels.
const maintenanceBackupMarkerContent = `# Dieser Ordner gehört der Doku-Sicherung von k-playbook
# (k-playbook maintain, Schritt docs-backup). Jeder Lauf gleicht ihn mit
# keph:/docs/ ab und löscht, was dort fehlt; nichts von Hand hineinlegen.
`

// maintenanceGitTimeout begrenzt einen git-Aufruf der Sicherung. add und
// commit über einen großen Doku-Baum brauchen länger als die lokalen
// Abfragen in gitrun.go.
const maintenanceGitTimeout = 2 * time.Minute

// exportOutcome ist der Ausgang des Exports.
type exportOutcome struct {
	// code ist der Exit-Code; -1, wenn der Prozess nicht startete oder durch
	// ein Signal endete.
	code int
	// interrupted sagt, dass ein SIGINT oder SIGTERM den Lauf erreicht hat.
	interrupted bool
	// err ist der Fehler beim Starten oder Warten, nil bei einem Exit-Code.
	err error
	// stderr ist die Fehlerausgabe von Kephalaion, für die Meldung bei Exit 1.
	stderr string
}

// maintenanceExport ruft den Export auf: binary mit args als
// Prozessargumenten, ohne Shell. Tests ersetzen ihn, damit kein echter Node
// gefragt wird.
var maintenanceExport = runKephalaionExport

// maintenanceKephalaionBinary findet das Binary; Tests ersetzen die Suche
// über kephalaion.LookupBinary.
var maintenanceKephalaionBinary = kephalaion.FindBinary

// runKephalaionExport startet den Export in einer eigenen Prozessgruppe, ohne
// Eingabe. Ein SIGINT oder SIGTERM an den Wartungslauf geht genau einmal an
// Kephalaion weiter: Es beendet den laufenden Einzelvorgang, meldet, wie weit
// es kam, und endet mit Exit 3. Der Wartungslauf selbst läuft danach weiter,
// meldet den Abbruch und committet nicht.
func runKephalaionExport(ctx context.Context, binary string, args []string, stdout, stderr io.Writer) exportOutcome {
	var captured bytes.Buffer
	cmd := exec.Command(binary, args...)
	cmd.Stdin = nil
	cmd.Stdout = stdout
	cmd.Stderr = io.MultiWriter(stderr, &captured)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	signals := make(chan os.Signal, 2)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)

	if err := cmd.Start(); err != nil {
		return exportOutcome{code: -1, err: err}
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	outcome := exportOutcome{}
	for {
		select {
		case sig := <-signals:
			if !outcome.interrupted {
				outcome.interrupted = true
				_ = cmd.Process.Signal(sig)
			}
		case <-ctx.Done():
			if !outcome.interrupted {
				outcome.interrupted = true
				_ = cmd.Process.Signal(syscall.SIGTERM)
			}
		case err := <-done:
			outcome.stderr = captured.String()
			if err == nil {
				return outcome
			}
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				outcome.code = exitErr.ExitCode()
				if outcome.code < 0 {
					outcome.err = err
				}
				return outcome
			}
			outcome.code, outcome.err = -1, err
			return outcome
		}
	}
}

// runDocsBackup ist der Schritt docs-backup.
func runDocsBackup(run *maintenanceRun) MaintenanceStepResult {
	const step = MaintenanceStepDocsBackup
	m := run.config.Maintenance
	if !m.Present {
		return stepSkipped(step, "Sicherung deaktiviert: der Block maintenance fehlt in "+ConfigFileName+
			" (k-playbook upgrade ergänzt ihn, deaktiviert)")
	}
	if !m.BackupEnabled {
		return stepSkipped(step, "Sicherung deaktiviert (maintenance.backup.enabled ist nicht true)")
	}
	commitRequested := run.commit || m.GitCommit

	status := KephalaionStatus(run.projectDir)
	if !status.OK {
		return stepFailed(step, "Kephalaion ist nicht bereit ("+string(status.State)+"): "+status.Message+" — nichts exportiert")
	}
	address := status.Address
	if address == "" {
		address = m.backupAddress(run.config)
	}

	target, err := ResolveMaintenanceTarget(run.projectDir, run.config)
	if err != nil {
		return stepFailed(step, err.Error()+" — nichts exportiert")
	}

	if !run.dryRun {
		unlock, err := lockMaintenanceTarget(target.Path)
		if err != nil {
			return stepFailed(step, err.Error()+" — nichts exportiert")
		}
		defer unlock()
	}

	var notes []string
	first, err := prepareBackupTarget(target.Path, run.dryRun)
	if err != nil {
		return stepFailed(step, err.Error())
	}
	if first != "" {
		notes = append(notes, first)
	}

	binary, ok := maintenanceKephalaionBinary()
	if !ok {
		return stepFailed(step, "kephalaion nicht gefunden — nichts exportiert; Abhilfe: /k-upgrade", notes...)
	}
	args := []string{"node", "dir", "pull", address, m.BackupSource, target.Path,
		"--delete", "--exclude", MaintenanceBackupMarker}
	preview := ""
	if run.dryRun {
		args = append(args, "--dry-run")
		preview = " (Vorschau)"
	}
	run.progress("%s: Export %s %s/ → %s%s", step, address, m.BackupSource, target.Rel, preview)
	outcome := maintenanceExport(run.ctx, binary, args, run.out, run.errOut)

	partial := "Der lokale Stand unter " + target.Rel + " kann teilweise aktualisiert sein."
	switch {
	case outcome.interrupted:
		return stepFailed(step, "Export abgebrochen — kein Commit; erneut ausführen", append(notes, partial)...)
	case outcome.code == 0 && outcome.err == nil:
	case outcome.code == 1:
		message := lastLine(outcome.stderr)
		if message == "" {
			message = "ohne Meldung"
		}
		return stepFailed(step, "Fehler bei Kephalaion: "+message+" — kein Commit", append(notes, partial)...)
	case outcome.code == 2:
		return stepFailed(step, "Kephalaion zu alt oder falscher Aufruf → /k-upgrade — kein Commit", append(notes, partial)...)
	case outcome.code == 3:
		return stepFailed(step, "partiell, erneut ausführen, kein Commit", append(notes, partial)...)
	case outcome.err != nil:
		return stepFailed(step, "Ausgang des Exports offen ("+outcome.err.Error()+") — kein Commit", append(notes, partial)...)
	default:
		return stepFailed(step, fmt.Sprintf("Export endete mit unbekanntem Exit-Code %d — kein Commit", outcome.code), append(notes, partial)...)
	}

	source := "--commit"
	if !run.commit {
		source = "maintenance.backup.git.commit"
	}
	if run.dryRun {
		if commitRequested {
			notes = append(notes, "Ein Commit wäre angefordert ("+source+"); die Vorschau committet nicht.")
		} else {
			notes = append(notes, "Kein Commit angefordert.")
		}
		return stepOK(step, "Vorschau: nichts geschrieben", notes...)
	}
	if !commitRequested {
		return stepOK(step, "exportiert nach "+target.Rel+"; kein Commit angefordert", notes...)
	}

	message := "Doku-Sicherung: keph:/docs/ aus " + address + "\n\nErstellt von k-playbook maintain (Schritt " + step + ")."
	hash, changed, err := commitBackupTarget(target, message)
	switch {
	case err != nil:
		return stepFailed(step, "exportiert nach "+target.Rel+", aber Git: "+err.Error(), notes...)
	case !changed:
		return stepOK(step, "exportiert nach "+target.Rel+"; keine Änderungen, kein Commit", notes...)
	}
	return stepOK(step, "exportiert nach "+target.Rel+" und committet ("+hash+", "+source+"); nicht gepusht", notes...)
}

// backupAddress ist <hub>:<collection> aus dem Block kephalaion:, falls der
// Status keine Adresse nennt.
func (m ConfigMaintenance) backupAddress(config Config) string {
	return config.Kephalaion.Hub + ":" + config.Kephalaion.Collection
}

// lastLine ist die letzte nicht leere Zeile.
func lastLine(text string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	for index := len(lines) - 1; index >= 0; index-- {
		if line := strings.TrimSpace(lines[index]); line != "" {
			return line
		}
	}
	return ""
}

// prepareBackupTarget prüft die Markierung. Trägt das Ziel sie, geht es
// weiter. Fehlt das Ziel oder ist es leer, ist das der erste Lauf: Ordner und
// Markierung werden angelegt, in der Vorschau nur angekündigt. Ein nicht
// leeres Ziel ohne Markierung gehört nicht dem Export; dann bricht der Schritt
// vor dem Export ab. Zurück kommt der Satz zum ersten Lauf, sonst leer.
func prepareBackupTarget(path string, dryRun bool) (string, error) {
	info, err := os.Lstat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return "", fmt.Errorf("Ziel nicht lesbar: %v — nichts exportiert", err)
	case !info.IsDir():
		return "", fmt.Errorf("Ziel %s ist kein Verzeichnis — nichts exportiert", path)
	default:
		marker, err := os.Lstat(filepath.Join(path, MaintenanceBackupMarker))
		if err == nil && marker.Mode().IsRegular() {
			return "", nil
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			return "", fmt.Errorf("Ziel nicht lesbar: %v — nichts exportiert", err)
		}
		if len(entries) > 0 {
			return "", fmt.Errorf("Ziel %s ist nicht leer und trägt keine Markierung %s: es gehört nicht dem Export — "+
				"nichts exportiert; ein leeres oder fehlendes Ziel wählen", path, MaintenanceBackupMarker)
		}
	}
	if dryRun {
		return "Erster Lauf: Ziel und Markierung " + MaintenanceBackupMarker + " würden angelegt.", nil
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return "", fmt.Errorf("Ziel anlegen: %v — nichts exportiert", err)
	}
	if err := os.WriteFile(filepath.Join(path, MaintenanceBackupMarker), []byte(maintenanceBackupMarkerContent), 0o644); err != nil {
		return "", fmt.Errorf("Markierung anlegen: %v — nichts exportiert", err)
	}
	return "Erster Lauf: Ziel und Markierung " + MaintenanceBackupMarker + " angelegt.", nil
}

// maintenanceLockDir ist der Ort der Sperren: im Cache des Nutzers, nicht im
// Repository, damit nichts davon im Arbeitsbaum auftaucht. Tests ersetzen ihn.
var maintenanceLockDir = func() string {
	dir, err := os.UserCacheDir()
	if err != nil || dir == "" {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "k-playbook", "locks")
}

// lockMaintenanceTarget sperrt das Ziel für die Dauer des Schritts. Ein
// zweiter Lauf auf dasselbe Ziel — auch aus einem anderen Projekt-Checkout,
// der darauf zeigt — wird abgelehnt, statt zu warten: ein Cron-Lauf soll nicht
// hängen. Die Sperrdatei bleibt liegen; sie zu löschen öffnete ein Fenster, in
// dem zwei Läufe verschiedene Dateien sperren.
func lockMaintenanceTarget(path string) (func(), error) {
	dir := maintenanceLockDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("Sperre anlegen: %v", err)
	}
	sum := sha256.Sum256([]byte(path))
	file, err := os.OpenFile(filepath.Join(dir, "maintain-"+hex.EncodeToString(sum[:8])+".lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("Sperre anlegen: %v", err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, fmt.Errorf("ein anderer Wartungslauf sichert gerade dasselbe Ziel (%s); abgelehnt", path)
		}
		return nil, fmt.Errorf("Sperre setzen: %v", err)
	}
	return func() {
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		file.Close()
	}, nil
}

// maintenanceGit führt git in dir aus, ohne Eingabe und ohne Rückfrage nach
// Zugangsdaten. Zurück kommen stdout, der Exit-Code (-1, wenn git nicht lief)
// und die erste Zeile der Fehlerausgabe.
func maintenanceGit(dir string, args ...string) (string, int, string) {
	ctx, cancel := context.WithTimeout(context.Background(), maintenanceGitTimeout)
	defer cancel()
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Stdin = nil
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	err := cmd.Run()
	switch {
	case ctx.Err() != nil:
		return "", -1, fmt.Sprintf("git %s hat nicht innerhalb von %s geantwortet", args[0], maintenanceGitTimeout)
	case err == nil:
		return stdout.String(), 0, ""
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() > 0 {
		reason := firstLine(stderr.String())
		if reason == "" {
			reason = firstLine(stdout.String())
		}
		return stdout.String(), exitErr.ExitCode(), reason
	}
	return "", -1, err.Error()
}

// commitBackupTarget committet ausschließlich die Änderungen und Löschungen
// im Ziel: git add -A und git commit --only, je auf den Pfad des Ziels
// beschränkt (wörtlich, ohne Pfadmuster; check-ignore nimmt ohnehin Pfade). Vorgemerkte fremde Änderungen
// bleiben vorgemerkt und gelangen nicht in den Commit, der übrige Arbeitsbaum
// bleibt unberührt. Ohne Änderung kein Commit. Kein Reset, kein Stash, kein
// Push. Ohne Git oder wenn Git das Ziel ignoriert, ist das ein Fehler.
func commitBackupTarget(target MaintenanceTarget, message string) (string, bool, error) {
	top, code, reason := maintenanceGit(target.RepoRoot, "rev-parse", "--show-toplevel")
	switch {
	case code == -1:
		return "", false, fmt.Errorf("git nicht ausführbar (%s) — kein Commit", reason)
	case code != 0:
		return "", false, fmt.Errorf("project.repoRoot ist kein Git-Repository (%s) — kein Commit", reason)
	}
	root := strings.TrimSpace(top)
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	rel, err := filepath.Rel(root, target.Path)
	if err != nil || !pathStrictlyBelow(root, target.Path) {
		return "", false, fmt.Errorf("Ziel %s liegt nicht im Git-Repository %s — kein Commit", target.Path, root)
	}
	rel = filepath.ToSlash(rel)

	_, code, reason = maintenanceGit(root, "check-ignore", "-q", "--", rel+"/"+MaintenanceBackupMarker)
	switch code {
	case 0:
		return "", false, fmt.Errorf("Git ignoriert das Ziel %s (.gitignore) — kein Commit", rel)
	case 1:
	default:
		return "", false, fmt.Errorf("git check-ignore: %s — kein Commit", reason)
	}

	if _, code, reason = maintenanceGit(root, "--literal-pathspecs", "add", "-A", "--", rel); code != 0 {
		return "", false, fmt.Errorf("git add: %s — kein Commit", reason)
	}
	_, code, reason = maintenanceGit(root, "--literal-pathspecs", "diff", "--cached", "--quiet", "--", rel)
	switch code {
	case 0:
		return "", false, nil
	case 1:
	default:
		return "", false, fmt.Errorf("git diff: %s — kein Commit; die Änderungen im Ziel sind vorgemerkt", reason)
	}
	if _, code, reason = maintenanceGit(root, "--literal-pathspecs", "commit", "--quiet", "--only", "-m", message, "--", rel); code != 0 {
		return "", false, fmt.Errorf("Commit fehlgeschlagen: %s; die Änderungen im Ziel bleiben vorgemerkt", reason)
	}
	hash, _, _ := maintenanceGit(root, "rev-parse", "--short", "HEAD")
	return strings.TrimSpace(hash), true, nil
}
