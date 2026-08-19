package project

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// privateGitTimeout begrenzt jeden einzelnen git-Aufruf der Messung. Alle
// Aufrufe sind rein lokal; wer länger braucht, antwortet nicht mehr sinnvoll.
const privateGitTimeout = 10 * time.Second

// PrivateIgnoreFile ist die eine Datei, die k-playbook für diese Entscheidung
// schreibt. Gemessen wird über alle Ignore-Ebenen, geschrieben nur hier.
const PrivateIgnoreFile = ".gitignore"

// privateProbeName ist der Pfad, an dem gemessen wird. Er liegt *innerhalb* des
// Verzeichnisses, weil eine .gitignore mit `*` den Inhalt ignoriert und nicht
// den Verzeichniseintrag — auf das Verzeichnis selbst gefragt meldete
// check-ignore dann „nicht ignoriert". Die Datei muss nicht existieren:
// check-ignore --no-index beantwortet die Frage rein an den Regeln.
const privateProbeName = "k-playbook-privat-probe"

// privateIgnoreLines ist der verwaltete Inhalt: alles draußen, das Verzeichnis
// selbst und seine README bleiben sichtbar. Nur eine Datei mit genau diesen
// Zeilen gilt als verwaltet und darf umgeschaltet werden.
var privateIgnoreLines = []string{"*", "!" + PrivateIgnoreFile, "!README.md"}

// PrivacyState ist der gemessene Zustand eines Verzeichnisses. Vier davon
// beschreiben ein Repository, drei sagen, dass es nichts zu messen gab.
type PrivacyState string

const (
	// PrivacyPrivate: eine Regel greift, und weder Index noch HEAD tragen
	// erfasste Dateien.
	PrivacyPrivate PrivacyState = "private"
	// PrivacyPublic: keine Regel, der Inhalt wird ganz normal versioniert.
	PrivacyPublic PrivacyState = "public"
	// PrivacyPartial: eine Regel greift, es stehen aber Dateien im Index. Sieht
	// privat aus und ist es nicht — die Regel wirkt nur für neue Dateien.
	PrivacyPartial PrivacyState = "partial"
	// PrivacyPendingCommit: eine Regel greift, der Index ist leer, HEAD trägt
	// die Dateien noch. Nach einem git rm --cached ohne Commit: jeder Clone
	// bekommt sie weiterhin.
	PrivacyPendingCommit PrivacyState = "pending-commit"
	// PrivacyNoVCS: das Projekt sagt selbst, dass es kein git benutzt.
	PrivacyNoVCS PrivacyState = "no-vcs"
	// PrivacyMissing: das Verzeichnis ist noch nicht angelegt.
	PrivacyMissing PrivacyState = "missing"
	// PrivacyUnknown: git konnte die Frage nicht beantworten. Reason sagt warum.
	PrivacyUnknown PrivacyState = "unknown"
)

// PrivacyRule ist die Regel, die den Inhalt ignoriert, so wie
// `git check-ignore -v` sie meldet.
type PrivacyRule struct {
	// Source ist die Datei mit der Regel, wie git sie nennt: relativ zum
	// Repo-Root, bei der globalen Konfiguration absolut.
	Source string `json:"source"`
	// Line ist die Zeilennummer darin, 0 wenn git keine nennt.
	Line int `json:"line"`
	// Pattern ist das Muster selbst.
	Pattern string `json:"pattern"`
}

// PrivacyStatus ist der gemessene Ist-Zustand eines Verzeichnisses. Er ist die
// einzige Quelle dieser Aussage — Anzeige und Umschaltung lesen denselben Wert.
type PrivacyStatus struct {
	// Path ist der Eintragspfad unterhalb von k-playbook-local, z. B. "priv".
	Path string `json:"path"`
	// Dir ist das gemessene Verzeichnis.
	Dir string `json:"dir"`
	// State ist einer der sieben Zustände.
	State PrivacyState `json:"state"`
	// RepoRoot ist das Repository, auf das sich die Aussage bezieht. Ohne ihn
	// wäre sie mehrdeutig: git -C nimmt das nächstgelegene Repo, und
	// k-playbook-local kann ein eigenes sein.
	RepoRoot string `json:"repoRoot,omitempty"`
	// Rule ist die greifende Regel, falls es eine gibt.
	Rule *PrivacyRule `json:"rule,omitempty"`
	// Managed: die Regel steht in der verwalteten Datei und diese trägt genau
	// den verwalteten Inhalt.
	Managed bool `json:"managed"`
	// CanToggle: k-playbook darf den Zustand umschalten.
	CanToggle bool `json:"canToggle"`
	// Blocked nennt die fremde Quelle, wenn nicht umgeschaltet werden darf.
	Blocked string `json:"blocked,omitempty"`
	// Tracked sind die von der Regel erfassten Dateien, die im Index stehen.
	Tracked []string `json:"tracked,omitempty"`
	// InHead sind die von der Regel erfassten Dateien, die in HEAD stehen.
	InHead []string `json:"inHead,omitempty"`
	// Reason erklärt die drei Zustände ohne Messung.
	Reason string `json:"reason,omitempty"`
}

// PrivateEntries sind die Einträge, für die diese Wahl überhaupt ansteht.
// Andere Verzeichnisse werden nicht umgeschaltet, auch nicht auf Zuruf.
func PrivateEntries() []LocalEntry {
	entries := []LocalEntry{}
	for _, entry := range LocalStructure() {
		if entry.Private {
			entries = append(entries, entry)
		}
	}
	return entries
}

// PrivateEntry sucht einen Eintrag anhand seines Pfades. Gefunden wird nur, was
// in LocalStructure() steht und dort Private trägt.
func PrivateEntry(path string) (LocalEntry, bool) {
	for _, entry := range PrivateEntries() {
		if entry.Path == path {
			return entry, true
		}
	}
	return LocalEntry{}, false
}

// PrivacyStatuses misst alle Einträge, für die die Wahl ansteht.
func PrivacyStatuses(projectDir string) []PrivacyStatus {
	entries := PrivateEntries()
	statuses := make([]PrivacyStatus, 0, len(entries))
	for _, entry := range entries {
		statuses = append(statuses, privacyStatus(projectDir, entry))
	}
	return statuses
}

// PrivacyStatusFor misst einen einzelnen Eintrag.
func PrivacyStatusFor(projectDir string, entry LocalEntry) PrivacyStatus {
	return privacyStatus(projectDir, entry)
}

// privacyStatus misst den Zustand eines Verzeichnisses.
//
// Vorbedingungen und Fehlerfälle folgen agentsIgnored: Gate auf
// project.vcs == git, Timeout-Kontext, kein Aufruf ohne diese Vorbedingung.
// Anders als dort wird Exit 1 von check-ignore aber nicht wie jeder andere
// Nicht-Null-Ausgang behandelt — es ist die reguläre Antwort „keine Regel",
// während 128 heißt, dass die Frage gar nicht beantwortet wurde. Ein kaputter
// git-Aufruf stünde sonst als „nicht privat" da.
func privacyStatus(projectDir string, entry LocalEntry) PrivacyStatus {
	status := PrivacyStatus{
		Path: entry.Path,
		Dir:  filepath.Join(LocalDir(projectDir), entry.Path),
	}

	config, err := ReadConfig(projectDir)
	if err != nil || config.VCS != "git" {
		status.State = PrivacyNoVCS
		status.Reason = "Das Projekt ist nicht als git-Projekt konfiguriert; es gibt nichts, wovor der Inhalt zu schützen wäre."
		return status
	}
	repoRoot := RepoRootDir(projectDir, config)
	if !isDir(status.Dir) {
		status.State = PrivacyMissing
		status.Reason = "Das Verzeichnis ist noch nicht angelegt."
		return status
	}
	if !pathWithin(status.Dir, repoRoot) {
		return unmeasured(status, LocalDirName+" liegt außerhalb des konfigurierten Projekt-Repositorys "+repoRoot+
			". Der Inhalt kann so nicht durch dieses Repository versioniert oder ignoriert werden.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), privateGitTimeout)
	defer cancel()

	root, code, detail := runGit(ctx, status.Dir, "rev-parse", "--show-toplevel")
	if code != 0 {
		return unmeasured(status, detail)
	}
	status.RepoRoot = strings.TrimSpace(root)

	rule, found, detail := checkIgnoreRule(ctx, status.Dir)
	if detail != "" {
		return unmeasured(status, detail)
	}
	if found {
		status.Rule = &rule
		status.Managed = managedRule(status.Dir, status.RepoRoot, rule)
	}

	tracked, detail := ignoredFiles(ctx, status.Dir, "ls-files", "-z")
	if detail != "" {
		return unmeasured(status, detail)
	}
	head, detail := headFiles(ctx, status.Dir)
	if detail != "" {
		return unmeasured(status, detail)
	}

	status.Tracked = tracked
	status.InHead = head
	status.State = privacyState(found, tracked, head)
	status.CanToggle, status.Blocked = privacyToggle(status)
	return status
}

// unmeasured setzt den Zustand „nicht ermittelbar" samt Grund. Kein Fehler an
// die Oberfläche: ein Verzeichnis ohne Repository ist eine Auskunft, keine
// Störung.
func unmeasured(status PrivacyStatus, detail string) PrivacyStatus {
	status.State = PrivacyUnknown
	status.Reason = detail
	if status.Reason == "" {
		status.Reason = "git konnte den Zustand nicht ermitteln."
	}
	return status
}

// privacyState ordnet die Messung einem der vier Repository-Zustände zu.
//
// Die Reihenfolge zählt: eine Regel plus Dateien im Index ist der gefährlichste
// Fall und wird zuerst geprüft. Erst danach kommt der gestagete, aber noch
// nicht committete Stand.
func privacyState(rule bool, tracked []string, head []string) PrivacyState {
	switch {
	case !rule:
		return PrivacyPublic
	case len(tracked) > 0:
		return PrivacyPartial
	case len(head) > 0:
		return PrivacyPendingCommit
	default:
		return PrivacyPrivate
	}
}

// privacyToggle entscheidet, ob k-playbook umschalten darf.
//
// Erkennung und Umschaltung dürfen nicht auseinanderfallen: gemessen wird über
// alle Ignore-Ebenen, geschrieben wird nur die eine verwaltete Datei. Stammt
// die Regel von woanders, wäre ein Ausschalten wirkungslos; trägt die Datei im
// Verzeichnis eigenen Inhalt, wäre ein Einschalten zerstörend.
func privacyToggle(status PrivacyStatus) (bool, string) {
	managedPath := filepath.Join(status.Dir, PrivateIgnoreFile)

	switch status.State {
	case PrivacyPublic:
		if !pathExists(managedPath) || hasManagedContent(managedPath) {
			return true, ""
		}
		return false, PrivateIgnoreFile + " im Verzeichnis trägt eigenen Inhalt. " +
			"k-playbook schreibt nur die von ihm verwaltete Fassung und fasst diese Datei nicht an."

	case PrivacyPrivate, PrivacyPartial, PrivacyPendingCommit:
		if status.Managed {
			return true, ""
		}
		return false, "Die Regel stammt aus " + status.Rule.Source +
			" und nicht aus der von k-playbook verwalteten " + PrivateIgnoreFile +
			" im Verzeichnis. Umgeschaltet wird sie deshalb nicht."

	default:
		return false, ""
	}
}

// PrivacyChange ist das Ergebnis einer Umschaltung: der neue Zustand und das,
// was dabei geschehen ist.
type PrivacyChange struct {
	// Status ist der frisch gemessene Zustand — dieselbe Quelle wie beim Lesen.
	Status PrivacyStatus `json:"status"`
	// Untracked sind die Dateien, die dabei aus dem Index genommen wurden.
	Untracked []string `json:"untracked,omitempty"`
	// Changed: es wurde tatsächlich etwas geschrieben.
	Changed bool `json:"changed"`
	// Message sagt im Klartext, was geschehen ist, einschließlich des noch
	// ausstehenden Commits.
	Message string `json:"message"`
}

// SetPrivate schaltet ein Verzeichnis um.
//
// Geschrieben wird ausschließlich die verwaltete .gitignore im Verzeichnis
// selbst. Sind Dateien getrackt, gehört ein git rm --cached dazu — als Teil
// derselben Operation, sonst bliebe ein Zustand stehen, der privat aussieht und
// keiner ist. Der Aufruf ist idempotent: steht der Eintrag bereits im
// Zielzustand, passiert nichts.
func SetPrivate(projectDir string, entry LocalEntry, private bool) (PrivacyChange, error) {
	if !entry.Private {
		return PrivacyChange{}, fmt.Errorf("%s ist kein Verzeichnis, für das diese Wahl ansteht", entry.Path)
	}

	status := privacyStatus(projectDir, entry)
	change := PrivacyChange{Status: status}

	switch status.State {
	case PrivacyNoVCS, PrivacyMissing, PrivacyUnknown:
		return change, errors.New(status.Reason)
	}

	if private {
		return makePrivate(projectDir, entry, change)
	}
	return makePublic(projectDir, entry, change)
}

// makePrivate legt die verwaltete Datei an und nimmt aus dem Index, was die
// Regel danach erfasst.
//
// Gemessen wird nach dem Schreiben ein zweites Mal: welche Dateien die neue
// Regel erfasst, beantwortet git, nicht der Inhalt der Datei. Eine eigene
// Ableitung würde von der Erkennung abweichen, sobald jemand die Regeln
// ergänzt.
func makePrivate(projectDir string, entry LocalEntry, change PrivacyChange) (PrivacyChange, error) {
	status := change.Status
	switch status.State {
	case PrivacyPrivate:
		change.Message = "Der Inhalt ist bereits privat."
		return change, nil

	case PrivacyPendingCommit:
		change.Message = pendingCommitMessage(status.InHead)
		return change, nil
	}

	if !status.CanToggle {
		return change, errors.New(status.Blocked)
	}

	managed := filepath.Join(status.Dir, PrivateIgnoreFile)
	if !hasManagedContent(managed) {
		if err := os.WriteFile(managed, []byte(managedIgnoreContent()), 0o644); err != nil {
			return change, fmt.Errorf("%s schreiben: %w", managed, err)
		}
	}
	change.Changed = true

	change.Status = privacyStatus(projectDir, entry)
	if len(change.Status.Tracked) > 0 {
		if err := removeFromIndex(change.Status.Dir, change.Status.Tracked); err != nil {
			return change, err
		}
		change.Untracked = change.Status.Tracked
		change.Status = privacyStatus(projectDir, entry)
	}

	change.Message = "Der Inhalt ist ab jetzt privat."
	if len(change.Untracked) > 0 {
		change.Message += " " + untrackedMessage(change.Untracked)
	}
	if change.Status.State == PrivacyPendingCommit {
		change.Message += " " + pendingCommitMessage(change.Status.InHead)
	}
	return change, nil
}

// makePublic entfernt die verwaltete Datei. Aus dem Index genommene Dateien
// kommen dabei nicht von selbst zurück — was wieder versioniert werden soll,
// fügt das Projekt selbst hinzu.
func makePublic(projectDir string, entry LocalEntry, change PrivacyChange) (PrivacyChange, error) {
	status := change.Status
	if status.State == PrivacyPublic {
		change.Message = "Der Inhalt wird bereits versioniert."
		return change, nil
	}
	if !status.CanToggle {
		return change, errors.New(status.Blocked)
	}

	managed := filepath.Join(status.Dir, PrivateIgnoreFile)
	if err := os.Remove(managed); err != nil && !os.IsNotExist(err) {
		return change, fmt.Errorf("%s entfernen: %w", managed, err)
	}
	change.Changed = true

	change.Status = privacyStatus(projectDir, entry)
	change.Message = PrivateIgnoreFile + " im Verzeichnis ist entfernt. Der Inhalt wird ab jetzt wieder von git gesehen; " +
		"was wieder versioniert werden soll, muss von Hand hinzugefügt werden."
	return change, nil
}

// removeFromIndex nimmt die Dateien aus dem Index, ohne sie im
// Arbeitsverzeichnis anzufassen.
//
// -f ist dabei nötig und ungefährlich: --cached lässt die Datei auf der Platte
// stehen, ohne den Schalter würde git aber jeden Pfad ablehnen, dessen
// gestageter Stand von HEAD abweicht. :(literal) hält Sonderzeichen in
// Dateinamen aus der Pathspec-Auswertung heraus.
func removeFromIndex(dir string, paths []string) error {
	args := []string{"rm", "--cached", "--quiet", "-f", "--"}
	for _, path := range paths {
		args = append(args, ":(literal)"+path)
	}

	ctx, cancel := context.WithTimeout(context.Background(), privateGitTimeout)
	defer cancel()

	if _, code, detail := runGit(ctx, dir, args...); code != 0 {
		return fmt.Errorf("git rm --cached: %s", detail)
	}
	return nil
}

func untrackedMessage(paths []string) string {
	return fmt.Sprintf("%s aus dem Index genommen: %s.", countFiles(len(paths)), strings.Join(paths, ", "))
}

// pendingCommitMessage benennt den Zwischenzustand: die Löschung ist gestaget,
// ohne Commit trägt jeder Clone die Dateien weiter.
func pendingCommitMessage(paths []string) string {
	verb := "stehen"
	if len(paths) == 1 {
		verb = "steht"
	}
	return fmt.Sprintf("%s %s noch in HEAD; privat wird der Inhalt erst mit dem nächsten Commit.",
		countFiles(len(paths)), verb)
}

func countFiles(count int) string {
	if count == 1 {
		return "1 Datei"
	}
	return strconv.Itoa(count) + " Dateien"
}

// checkIgnoreRule fragt, ob eine Regel den Inhalt des Verzeichnisses erfasst,
// und liefert sie samt Quelle und Zeile für die Anzeige.
//
// --no-index ist Pflicht: per Default zieht git den Index heran und meldet eine
// getrackte Datei als nicht ignoriert. Für genau die Zustände, die privat
// aussehen und keiner sind, gäbe es dann gar keine Aussage.
//
// Exit 1 ist die reguläre Antwort „keine Regel", Exit 128 dagegen ein Fehler.
func checkIgnoreRule(ctx context.Context, dir string) (PrivacyRule, bool, string) {
	out, code, detail := runGit(ctx, dir, "check-ignore", "-v", "--no-index", privateProbeName)
	switch code {
	case 0:
	case 1:
		return PrivacyRule{}, false, ""
	default:
		return PrivacyRule{}, false, detail
	}

	// Format: <quelle>:<zeile>:<muster>\t<pfad>
	line, _, _ := strings.Cut(strings.TrimRight(out, "\n"), "\t")
	source, rest, found := strings.Cut(line, ":")
	if !found {
		return PrivacyRule{}, false, "Antwort von git check-ignore nicht lesbar: " + line
	}
	number, pattern, _ := strings.Cut(rest, ":")

	rule := PrivacyRule{Source: source, Pattern: pattern}
	if parsed, err := strconv.Atoi(number); err == nil {
		rule.Line = parsed
	}
	return rule, true, ""
}

// ignoredFiles listet Dateien mit dem angegebenen git-Aufruf und behält nur
// die, die eine Ignore-Regel tatsächlich erfasst.
//
// Der Filter ist nötig, weil der verwaltete Inhalt README.md und die .gitignore
// ausdrücklich drin lässt: ungefiltert stünde jedes verwaltete Verzeichnis
// dauerhaft als „teilweise privat" da. Gefragt wird auch hier git, nicht die
// verwaltete Datei — sonst fielen Erkennung und Anzeige auseinander.
func ignoredFiles(ctx context.Context, dir string, args ...string) ([]string, string) {
	out, code, detail := runGit(ctx, dir, args...)
	if code != 0 {
		return nil, detail
	}
	return ignoredAmong(ctx, dir, splitNUL(out))
}

// headFiles listet die Dateien, die in HEAD stehen.
//
// Ein Repository ohne Commits ist kein Fehler: ls-tree scheitert dort mit Exit
// 128 („Not a valid object name HEAD"), und ohne diesen Vorabtest stünde ein
// frisch initialisiertes Repository als „nicht ermittelbar" statt als „privat"
// da. rev-parse beantwortet die Frage, ohne den Fehlertext zu lesen.
func headFiles(ctx context.Context, dir string) ([]string, string) {
	if _, code, _ := runGit(ctx, dir, "rev-parse", "--verify", "--quiet", "HEAD"); code != 0 {
		return nil, ""
	}
	return ignoredFiles(ctx, dir, "ls-tree", "-r", "--name-only", "-z", "HEAD", "--", ".")
}

// ignoredAmong behält von einer Pfadliste die, die eine Ignore-Regel erfasst.
// Exit 1 heißt: keiner davon.
func ignoredAmong(ctx context.Context, dir string, paths []string) ([]string, string) {
	if len(paths) == 0 {
		return nil, ""
	}

	input := strings.Join(paths, "\x00") + "\x00"
	out, code, detail := runGitStdin(ctx, dir, input, "check-ignore", "--no-index", "-z", "--stdin")
	switch code {
	case 0:
		return splitNUL(out), ""
	case 1:
		return nil, ""
	default:
		return nil, detail
	}
}

// managedRule meldet, ob die greifende Regel aus der verwalteten Datei stammt
// und diese genau den verwalteten Inhalt trägt.
//
// git nennt die Quelle relativ zum Repo-Root; die globale Konfiguration steht
// mit absolutem Pfad da.
func managedRule(dir string, repoRoot string, rule PrivacyRule) bool {
	source := rule.Source
	if !filepath.IsAbs(source) {
		source = filepath.Join(repoRoot, source)
	}

	managed := filepath.Join(dir, PrivateIgnoreFile)
	if !samePath(source, managed) {
		return false
	}
	return hasManagedContent(managed)
}

// samePath vergleicht zwei Pfade und löst dafür Symlinks auf: der Repo-Root aus
// rev-parse ist aufgelöst, der selbst gebaute Pfad nicht.
func samePath(left string, right string) bool {
	if filepath.Clean(left) == filepath.Clean(right) {
		return true
	}

	resolvedLeft, err := filepath.EvalSymlinks(left)
	if err != nil {
		return false
	}
	resolvedRight, err := filepath.EvalSymlinks(right)
	if err != nil {
		return false
	}
	return resolvedLeft == resolvedRight
}

// pathWithin meldet, ob path innerhalb von root liegt. Symlinks werden soweit
// möglich aufgelöst, weil Discover und git ebenfalls mit aufgelösten Pfaden
// arbeiten.
func pathWithin(path string, root string) bool {
	path = resolvedOrClean(path)
	root = resolvedOrClean(root)
	if path == root {
		return true
	}
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func resolvedOrClean(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return filepath.Clean(resolved)
	}
	return filepath.Clean(path)
}

// hasManagedContent meldet, ob die Datei genau den verwalteten Inhalt trägt.
// Leerzeilen und Zeilenenden zählen nicht mit; jede andere Zeile schon — die
// Datei gehört dann dem Projekt.
func hasManagedContent(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}

	lines := []string{}
	for _, line := range strings.Split(string(data), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	if len(lines) != len(privateIgnoreLines) {
		return false
	}
	for index, want := range privateIgnoreLines {
		if lines[index] != want {
			return false
		}
	}
	return true
}

// managedIgnoreContent ist der Inhalt, den SetPrivate schreibt.
func managedIgnoreContent() string {
	return strings.Join(privateIgnoreLines, "\n") + "\n"
}

// splitNUL zerlegt eine NUL-terminierte git-Ausgabe.
func splitNUL(out string) []string {
	parts := []string{}
	for _, part := range strings.Split(out, "\x00") {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

// runGit führt einen git-Aufruf im Verzeichnis aus.
//
// Zurück kommen stdout, der Exit-Code und ein Grund im Klartext. Der Code ist
// -1, wenn git gar nicht gelaufen ist — nicht installiert oder Timeout; die
// Aufrufer unterscheiden 0, 1 und alles darüber.
func runGit(ctx context.Context, dir string, args ...string) (string, int, string) {
	return runGitStdin(ctx, dir, "", args...)
}

// runGitStdin ist runGit mit einer Eingabe auf stdin.
func runGitStdin(ctx context.Context, dir string, input string, args ...string) (string, int, string) {
	var stdout, stderr bytes.Buffer

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if input != "" {
		cmd.Stdin = strings.NewReader(input)
	}
	err := cmd.Run()

	switch {
	case ctx.Err() != nil:
		return "", -1, fmt.Sprintf("git %s hat nicht innerhalb von %s geantwortet", args[0], privateGitTimeout)
	case err == nil:
		return stdout.String(), 0, ""
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() > 0 {
		return stdout.String(), exitErr.ExitCode(), firstLine(stderr.String())
	}
	return "", -1, err.Error()
}

// firstLine nimmt die erste nicht leere Zeile — git schreibt den Grund dorthin,
// alles Weitere ist Hinweistext.
func firstLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
