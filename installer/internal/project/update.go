package project

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	// updateCheckTimeout begrenzt die Netzwerkabfrage. Sie läuft im
	// Hintergrund; ein hängender Remote darf die Oberfläche nicht blockieren.
	updateCheckTimeout = 15 * time.Second
	// pullTimeout: ein Clone mit Binaries kann etwas dauern.
	pullTimeout = 2 * time.Minute
	// cleanlinessTimeout: rein lokale Git-Aufrufe, die nur bei einem
	// blockierten Index überhaupt hängen können.
	cleanlinessTimeout = 10 * time.Second
	// prefetchTimeout begrenzt den Nachladeversuch nach einem
	// VERSION-Wechsel. Bewusst ein eigener Timeout und nicht der Kontext des
	// Pulls: der Pull ist zu diesem Zeitpunkt schon durch, und ein hängender
	// Download darf ihn nicht nachträglich zum Fehler machen.
	prefetchTimeout = 3 * time.Minute
)

// DevSyncMarker liegt in der Installation, wenn dort ein Arbeitsstand
// eingespielt wurde statt eines Clones.
//
// Nötig, weil Git die eingespielten Dateien zwangsläufig als Änderungen sieht
// und sich das nicht verbergen lässt: .git/info/exclude wirkt nur auf
// Unverfolgtes, --assume-unchanged ist unverbindlich, und --skip-worktree
// bricht den Checkout. Statt die Änderungen zu verstecken, wird der Zustand
// benannt — sonst stünde dauerhaft ein Alarm da, der eigentlich echte Handarbeit
// im Clone melden soll.
const DevSyncMarker = ".k-playbook-devsync"

// UpdateStatus beschreibt, ob die Installation hinter dem Remote liegt.
type UpdateStatus struct {
	Available bool   `json:"available"`
	Branch    string `json:"branch"`
	Local     string `json:"local"`
	Remote    string `json:"remote"`
	Message   string `json:"message"`
	// Cleanliness ist der lokale Zustand des Clones. Er hat mit dem
	// Remote-Stand nichts zu tun, gehört aber hierher: beides zusammen
	// beantwortet erst, ob ein Update überhaupt durchlaufen kann.
	Cleanliness Cleanliness `json:"cleanliness"`
}

// Cleanliness meldet, ob in der Installation lokal gearbeitet wurde.
//
// Das Modell verlangt, dass dort nie geschrieben wird: das Verzeichnis ist ein
// Clone und wird bei jedem Update ersetzt. Eine Abweichung fällt trotzdem
// nicht von selbst auf — im Gegenteil. Ändert sich eine lokal veränderte
// Datei upstream nicht mit, läuft `git pull` sauber durch und lässt sie
// stehen; die Änderung überlebt dann jedes Update, ohne je gemeldet zu
// werden. Genau deshalb wird hier ungefragt geprüft.
type Cleanliness struct {
	Clean bool `json:"clean"`
	// Modified sind verfolgte Dateien, die geändert oder gelöscht wurden.
	Modified []string `json:"modified"`
	// Untracked sind zusätzliche Dateien. Weniger scharf, aber auch nicht
	// vorgesehen: was das Projekt hervorbringt, gehört nach k-playbook-local/.
	Untracked []string `json:"untracked"`
	// Ahead sind lokale Commits. Sie blockieren `--ff-only` und lassen sich
	// nicht durch Verwerfen von Dateien auflösen.
	Ahead int `json:"ahead"`
	// DevSync: in der Installation liegt ein eingespielter Arbeitsstand. Kein
	// Versehen, sondern ein gewollter Zustand — er blockiert das Update trotzdem.
	DevSync bool   `json:"devSync"`
	Message string `json:"message"`
}

// Blocking sagt, ob ein Update in diesem Zustand scheitern oder still das
// Falsche tun würde. Untracked Dateien zählen nicht dazu: sie stehen einem
// Fast-Forward nicht im Weg.
func (c Cleanliness) Blocking() bool {
	return len(c.Modified) > 0 || c.Ahead > 0 || c.DevSync
}

// maxReportedPaths begrenzt die gemeldete Dateiliste. Wer dort umfangreich
// gearbeitet hat, braucht keine vollständige Aufzählung, sondern die
// Erkenntnis, dass er es getan hat.
const maxReportedPaths = 20

// CheckCleanliness liest den lokalen Zustand des Clones. Rein lesend, ohne Netz.
func CheckCleanliness(projectDir string) Cleanliness {
	dir := PlaybookDir(projectDir)

	// Vor dem git status: was hier steht, erklärt jede Abweichung, die der
	// Vergleich danach fände. Sie einzeln aufzuzählen wäre nur Lärm.
	if fileExists(filepath.Join(dir, DevSyncMarker)) {
		return Cleanliness{
			DevSync: true,
			Message: "Hier liegt ein eingespielter Arbeitsstand, kein Clone. Solange das so ist, " +
				"wird nicht aktualisiert — \"Arbeitsstand verwerfen\" stellt den Clone wieder her.",
		}
	}

	if !isDir(filepath.Join(dir, ".git")) {
		return Cleanliness{Clean: true, Message: "Die Installation ist kein Git-Repository."}
	}

	ctx, cancel := context.WithTimeout(context.Background(), cleanlinessTimeout)
	defer cancel()

	output, err := GitOutput(ctx, dir, "status", "--porcelain")
	if err != nil {
		return Cleanliness{Clean: true, Message: "Zustand nicht lesbar: " + err.Error()}
	}

	state := Cleanliness{Clean: true}
	for _, line := range strings.Split(output, "\n") {
		// Porcelain v1 ist `XY<Leerzeichen>PFAD`. Nicht nach fester Spalte
		// schneiden: `GitOutput` trimmt die Gesamtausgabe, wodurch der ersten
		// Zeile ein führendes Leerzeichen fehlen kann.
		code, path, found := strings.Cut(strings.TrimSpace(line), " ")
		if !found {
			continue
		}
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		if code == "??" {
			state.Untracked = append(state.Untracked, path)
			continue
		}
		state.Modified = append(state.Modified, path)
	}

	// `@{u}` schlägt ohne Upstream fehl; das ist kein Fehlerfall, sondern
	// heißt nur, dass sich die Frage nach lokalen Commits nicht stellt.
	if count, err := GitOutput(ctx, dir, "rev-list", "--count", "@{u}..HEAD"); err == nil {
		if n, convErr := strconv.Atoi(count); convErr == nil {
			state.Ahead = n
		}
	}

	state.Modified = truncatePaths(state.Modified)
	state.Untracked = truncatePaths(state.Untracked)
	state.Clean = len(state.Modified) == 0 && len(state.Untracked) == 0 && state.Ahead == 0
	state.Message = describeCleanliness(state)
	return state
}

func truncatePaths(paths []string) []string {
	if len(paths) <= maxReportedPaths {
		return paths
	}
	return append(paths[:maxReportedPaths:maxReportedPaths],
		fmt.Sprintf("... und %d weitere", len(paths)-maxReportedPaths))
}

func describeCleanliness(state Cleanliness) string {
	if state.Clean {
		return ""
	}

	parts := []string{}
	if n := len(state.Modified); n > 0 {
		parts = append(parts, fmt.Sprintf("%d veränderte Datei(en)", n))
	}
	if n := len(state.Untracked); n > 0 {
		parts = append(parts, fmt.Sprintf("%d zusätzliche Datei(en)", n))
	}
	if state.Ahead > 0 {
		parts = append(parts, fmt.Sprintf("%d lokale(r) Commit(s)", state.Ahead))
	}

	message := "In der Installation wurde lokal gearbeitet: " + strings.Join(parts, ", ") + "."
	if state.Ahead > 0 {
		return message + " Lokale Commits müssen von Hand aufgelöst werden."
	}
	if len(state.Modified) > 0 {
		return message + " Änderungen dort gehen beim Update verloren oder verhindern es."
	}
	return message + " Projekteigene Dateien gehören nach " + LocalDirName + "/."
}

// CheckUpdate fragt den Remote-Stand ab, ohne etwas zu verändern.
//
// Bewusst `git ls-remote` statt `git fetch`: die Prüfung läuft ungefragt nach
// dem Start und darf den Zustand des Repositorys nicht anfassen.
func CheckUpdate(projectDir string) (UpdateStatus, error) {
	dir := PlaybookDir(projectDir)
	cleanliness := CheckCleanliness(projectDir)
	if !isDir(filepath.Join(dir, ".git")) {
		return UpdateStatus{Message: "Die Installation ist kein Git-Repository.", Cleanliness: cleanliness}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), updateCheckTimeout)
	defer cancel()

	branch, err := GitOutput(ctx, dir, "branch", "--show-current")
	if err != nil {
		return UpdateStatus{}, err
	}
	if branch == "" {
		return UpdateStatus{Message: "Kein aktiver Branch, vermutlich ein Detached HEAD.", Cleanliness: cleanliness}, nil
	}

	remoteName, err := GitOutput(ctx, dir, "config", "--get", "branch."+branch+".remote")
	if err != nil || remoteName == "" {
		return UpdateStatus{Branch: branch, Message: "Kein Upstream für diesen Branch konfiguriert.", Cleanliness: cleanliness}, nil
	}
	mergeRef, err := GitOutput(ctx, dir, "config", "--get", "branch."+branch+".merge")
	if err != nil || mergeRef == "" {
		return UpdateStatus{Branch: branch, Message: "Kein Upstream für diesen Branch konfiguriert.", Cleanliness: cleanliness}, nil
	}

	local, err := GitOutput(ctx, dir, "rev-parse", "HEAD")
	if err != nil {
		return UpdateStatus{}, err
	}

	remoteBranch := strings.TrimPrefix(mergeRef, "refs/heads/")
	line, err := GitOutput(ctx, dir, "ls-remote", "--heads", remoteName, remoteBranch)
	if ctx.Err() == context.DeadlineExceeded {
		return UpdateStatus{Branch: branch, Message: "Der Remote hat nicht rechtzeitig geantwortet.", Cleanliness: cleanliness}, nil
	}
	if err != nil {
		return UpdateStatus{Branch: branch, Message: "Remote nicht erreichbar: " + err.Error(), Cleanliness: cleanliness}, nil
	}

	remote, _, _ := strings.Cut(strings.TrimSpace(line), "\t")
	if remote == "" {
		return UpdateStatus{Branch: branch, Local: local, Message: "Branch existiert nicht auf dem Remote.", Cleanliness: cleanliness}, nil
	}

	status := UpdateStatus{Branch: branch, Local: local, Remote: remote, Cleanliness: cleanliness}
	if local != remote {
		status.Available = true
	}
	return status, nil
}

// UpdateResult ist das Ergebnis eines Pull-Laufs.
type UpdateResult struct {
	Output string `json:"output"`
	// BinaryChanged meldet, ob sich die ausgelieferten Binaries geändert
	// haben. Nur dann bringt ein Neustart eine andere Programmversion.
	BinaryChanged bool   `json:"binaryChanged"`
	Message       string `json:"message"`
	// Cleanliness trägt den Grund, wenn das Update gar nicht erst lief.
	Cleanliness Cleanliness `json:"cleanliness"`
}

// DiscardDevSync verwirft einen eingespielten Arbeitsstand und stellt den
// unberuehrten Clone wieder her.
//
// Es gibt dafuer bewusst kein Make-Target mehr: `make installer-sync` spielt
// beim naechsten Lauf ohnehin wieder ein, ein Zurueck im Terminal waere also nur
// ein zweiter Weg zu demselben Ergebnis.
//
// Anders als bei Handarbeit im Clone darf die Oberflaeche das hier selbst: die
// Markierung sagt, woher der Inhalt kommt — aus `make installer-sync`, also aus
// dem Arbeitsstand. Verworfen wird eine Kopie, keine Arbeit. Ohne Markierung
// bleibt es bei der Verweigerung, denn dann laesst sich das nicht wissen.
func DiscardDevSync(projectDir string) (err error) {
	dir := PlaybookDir(projectDir)
	if !fileExists(filepath.Join(dir, DevSyncMarker)) {
		return fmt.Errorf("in %s liegt kein eingespielter Arbeitsstand", DisplayPath(dir))
	}
	if !isDir(filepath.Join(dir, ".git")) {
		return fmt.Errorf("%s ist kein Git-Repository", DisplayPath(dir))
	}
	if err := setInstallationWritable(projectDir); err != nil {
		return fmt.Errorf("Installation beschreibbar machen: %w", err)
	}
	defer keepInstallationReadOnly(projectDir, &err)

	// Zuerst die Markierung: bricht ein Git-Aufruf danach ab, steht der
	// Zustand wenigstens nicht mehr als Entwicklungsstand da, waehrend er
	// halb zurueckgesetzt ist.
	if err := os.Remove(filepath.Join(dir, DevSyncMarker)); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), cleanlinessTimeout)
	defer cancel()

	if _, err := GitOutput(ctx, dir, "checkout", "--", "."); err != nil {
		return fmt.Errorf("Zuruecksetzen fehlgeschlagen: %w", err)
	}
	// Nimmt auch mit, was jemand von Hand danebengelegt hat. Bei einem
	// eingespielten Arbeitsstand ist das folgerichtig — dort gehoert nichts hin,
	// was nicht aus dem Arbeitsstand kommt.
	if _, err := GitOutput(ctx, dir, "clean", "-qfd"); err != nil {
		return fmt.Errorf("Aufraeumen fehlgeschlagen: %w", err)
	}
	return nil
}

// Update holt den neuen Stand per Fast-Forward.
//
// Nur `--ff-only`: ein Merge im Clone würde eine lokale Historie erzeugen, die
// niemand pflegt. Wer dort committet hat, soll das selbst auflösen.
func Update(projectDir string) (result UpdateResult, err error) {
	dir := PlaybookDir(projectDir)

	// Vorher prüfen statt hinterher stolpern. `git pull` scheitert an einer
	// kollidierenden Datei mit einer Meldung, die niemand liest, und läuft an
	// einer nicht kollidierenden still vorbei. Beides ist schlechter als die
	// Ansage, welche Datei im Weg ist.
	if state := CheckCleanliness(projectDir); state.Blocking() {
		return UpdateResult{Cleanliness: state}, fmt.Errorf("%s", state.Message)
	}

	versionBefore := InstalledVersion(dir)
	before := binaryHashes(dir)
	if err := setInstallationWritable(projectDir); err != nil {
		return UpdateResult{}, fmt.Errorf("Installation beschreibbar machen: %w", err)
	}
	defer keepInstallationReadOnly(projectDir, &err)

	ctx, cancel := context.WithTimeout(context.Background(), pullTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "pull", "--ff-only")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(output))

	if ctx.Err() == context.DeadlineExceeded {
		return UpdateResult{Output: text}, fmt.Errorf("git pull hat nach %s nicht geantwortet", pullTimeout)
	}
	if err != nil {
		return UpdateResult{Output: text}, fmt.Errorf("git pull --ff-only fehlgeschlagen")
	}

	result = UpdateResult{Output: text}

	versionAfter := InstalledVersion(dir)
	versionChanged := versionBefore != versionAfter
	// Übergangsregel, solange dist/ versioniert ist: auch ein ersetztes Binary
	// meldet den Neustart. Ohne diese Hälfte fiele der Hinweis zwischen dem
	// Go-Umbau und dem ersten Release still aus, weil es noch keine VERSION
	// gibt. Mit dist/ fällt sie weg.
	result.BinaryChanged = versionChanged || !sameHashes(before, binaryHashes(dir))

	if versionChanged {
		output, err := prefetchBinary(dir)
		if output != "" {
			result.Output = strings.TrimSpace(result.Output + "\n" + output)
		}
		if err != nil {
			hint := fmt.Sprintf("Neue Version %s: das Binary konnte nicht vorab geladen werden (%v). Der nächste Start lädt es nach oder nennt den Ausweg.", versionAfter, err)
			result.Message = hint
			result.Output = strings.TrimSpace(result.Output + "\n" + hint)
		}
	}
	return result, nil
}

// InstalledVersion liest die VERSION der Installation. Leer, wenn es keine
// gibt — dann gehört zu diesem Stand kein Release.
func InstalledVersion(playbookDir string) string {
	content, err := os.ReadFile(filepath.Join(playbookDir, VersionFileName))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(content))
}

// prefetchBinary legt das Binary der eigenen Plattform gleich nach dem Pull in
// den Cache. Ohne das wartet der Nutzer beim Neustart auf den Download.
//
// Aufgerufen wird ausdrücklich der Wrapper dieser Installation, nicht der aus
// InstallDir(): das laufende Binary kann eine fremde Installation
// aktualisieren, und nur der frisch gezogene Clone trägt die neue VERSION und
// die dazu passenden Prüfsummen.
//
// Best effort. Der Rückgabefehler von Update() trägt über
// keepInstallationReadOnly das Ergebnis des ganzen Laufs; ein gescheiterter
// Download ließe ein erfolgreiches git pull als gescheitertes Update
// erscheinen — offline, hinter einem Proxy und genau im Release-Fenster.
func prefetchBinary(playbookDir string) (string, error) {
	wrapper := filepath.Join(playbookDir, BinDirName, WrapperName)
	if !fileExists(wrapper) {
		return "", fmt.Errorf("%s fehlt", DisplayPath(wrapper))
	}

	ctx, cancel := context.WithTimeout(context.Background(), prefetchTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, wrapper, "--prefetch")
	cmd.Dir = playbookDir
	output, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(output))

	if ctx.Err() == context.DeadlineExceeded {
		return text, fmt.Errorf("--prefetch hat nach %s nicht geantwortet", prefetchTimeout)
	}
	if err != nil {
		return text, fmt.Errorf("--prefetch fehlgeschlagen")
	}
	return text, nil
}

// binaryHashes bildet die ausgelieferten Binaries ab. Ändern sie sich, läuft
// der aktuelle Prozess weiterhin mit dem alten Code: unter Linux behält er
// seinen Inode, auch wenn die Datei ersetzt wurde.
func binaryHashes(playbookDir string) map[string]string {
	hashes := map[string]string{}

	entries, err := os.ReadDir(filepath.Join(playbookDir, "dist"))
	if err != nil {
		return hashes
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(playbookDir, "dist", entry.Name())
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		digest := sha256.New()
		_, err = io.Copy(digest, file)
		file.Close()
		if err != nil {
			continue
		}
		hashes[entry.Name()] = fmt.Sprintf("%x", digest.Sum(nil))
	}
	return hashes
}

func sameHashes(before map[string]string, after map[string]string) bool {
	if len(before) != len(after) {
		return false
	}
	for name, hash := range before {
		if after[name] != hash {
			return false
		}
	}
	return true
}

// GitOutput führt ein Git-Kommando in dir aus und liefert die getrimmte
// Ausgabe. Exportiert, weil auch hostinstall den Commit-Stand einer
// Installation braucht.
func GitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}
