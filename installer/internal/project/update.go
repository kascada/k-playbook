package project

import (
	"context"
	"fmt"
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
)

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
	Ahead   int    `json:"ahead"`
	Message string `json:"message"`
}

// Blocking sagt, ob ein Update in diesem Zustand scheitern oder still das
// Falsche tun würde. Untracked Dateien zählen nicht dazu: sie stehen einem
// Fast-Forward nicht im Weg.
func (c Cleanliness) Blocking() bool {
	return len(c.Modified) > 0 || c.Ahead > 0
}

// maxReportedPaths begrenzt die gemeldete Dateiliste. Wer dort umfangreich
// gearbeitet hat, braucht keine vollständige Aufzählung, sondern die
// Erkenntnis, dass er es getan hat.
const maxReportedPaths = 20

// CheckCleanliness liest den lokalen Zustand des Clones. Rein lesend, ohne Netz.
func CheckCleanliness(projectDir string) Cleanliness {
	dir := PlaybookDir(projectDir)

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
	// BinaryChanged meldet, ob der Stand ein anderes Binary verlangt — das
	// heißt: ob VERSION gewechselt hat. Nur dann bringt ein Neustart eine
	// andere Programmversion.
	BinaryChanged bool   `json:"binaryChanged"`
	Message       string `json:"message"`
	// Cleanliness trägt den Grund, wenn das Update gar nicht erst lief.
	Cleanliness Cleanliness `json:"cleanliness"`
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
	// Seit die Binaries Release-Assets sind, hängt alles an VERSION: der Clone
	// trägt kein Binary mehr, das sich vergleichen ließe.
	versionChanged := versionBefore != versionAfter
	result.BinaryChanged = versionChanged

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

// GitOutput führt ein Git-Kommando in dir aus und liefert die getrimmte
// Ausgabe.
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
