package project

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	// updateCheckTimeout begrenzt die Netzwerkabfrage. Sie läuft im
	// Hintergrund; ein hängender Remote darf die Oberfläche nicht blockieren.
	updateCheckTimeout = 15 * time.Second
	// pullTimeout: ein Clone mit Binaries kann etwas dauern.
	pullTimeout = 2 * time.Minute
)

// UpdateStatus beschreibt, ob die Installation hinter dem Remote liegt.
type UpdateStatus struct {
	Available bool   `json:"available"`
	Branch    string `json:"branch"`
	Local     string `json:"local"`
	Remote    string `json:"remote"`
	Message   string `json:"message"`
}

// CheckUpdate fragt den Remote-Stand ab, ohne etwas zu verändern.
//
// Bewusst `git ls-remote` statt `git fetch`: die Prüfung läuft ungefragt nach
// dem Start und darf den Zustand des Repositorys nicht anfassen.
func CheckUpdate(projectDir string) (UpdateStatus, error) {
	dir := PlaybookDir(projectDir)
	if !isDir(filepath.Join(dir, ".git")) {
		return UpdateStatus{Message: "Die Installation ist kein Git-Repository."}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), updateCheckTimeout)
	defer cancel()

	branch, err := GitOutput(ctx, dir, "branch", "--show-current")
	if err != nil {
		return UpdateStatus{}, err
	}
	if branch == "" {
		return UpdateStatus{Message: "Kein aktiver Branch, vermutlich ein Detached HEAD."}, nil
	}

	remoteName, err := GitOutput(ctx, dir, "config", "--get", "branch."+branch+".remote")
	if err != nil || remoteName == "" {
		return UpdateStatus{Branch: branch, Message: "Kein Upstream für diesen Branch konfiguriert."}, nil
	}
	mergeRef, err := GitOutput(ctx, dir, "config", "--get", "branch."+branch+".merge")
	if err != nil || mergeRef == "" {
		return UpdateStatus{Branch: branch, Message: "Kein Upstream für diesen Branch konfiguriert."}, nil
	}

	local, err := GitOutput(ctx, dir, "rev-parse", "HEAD")
	if err != nil {
		return UpdateStatus{}, err
	}

	remoteBranch := strings.TrimPrefix(mergeRef, "refs/heads/")
	line, err := GitOutput(ctx, dir, "ls-remote", "--heads", remoteName, remoteBranch)
	if ctx.Err() == context.DeadlineExceeded {
		return UpdateStatus{Branch: branch, Message: "Der Remote hat nicht rechtzeitig geantwortet."}, nil
	}
	if err != nil {
		return UpdateStatus{Branch: branch, Message: "Remote nicht erreichbar: " + err.Error()}, nil
	}

	remote, _, _ := strings.Cut(strings.TrimSpace(line), "\t")
	if remote == "" {
		return UpdateStatus{Branch: branch, Local: local, Message: "Branch existiert nicht auf dem Remote."}, nil
	}

	status := UpdateStatus{Branch: branch, Local: local, Remote: remote}
	if local != remote {
		status.Available = true
	}
	return status, nil
}

// UpdateResult ist das Ergebnis eines Pull-Laufs.
type UpdateResult struct {
	Output string `json:"output"`
	// VersionChanged meldet, ob der Pull die VERSION der Installation bewegt
	// hat. Das ist eine Tatsache über den Clone, noch keine Aufforderung: ob
	// dazu auch ein anderes Binary gehört, entscheidet der Aufrufer anhand von
	// Version — er allein weiß, aus welcher Version er selbst läuft.
	VersionChanged bool `json:"versionChanged"`
	// Version ist die VERSION der Installation nach dem Pull.
	Version string `json:"version"`
	Message string `json:"message"`
	// MCPRepaired nennt die MCP-Dateien, deren veralteter Eintrag beim Update
	// selbsttätig korrigiert wurde — relativ zum Hauptverzeichnis.
	MCPRepaired []string `json:"mcpRepaired,omitempty"`
}

// Update holt den neuen Stand per Fast-Forward.
//
// Nur `--ff-only`: ein Merge im Clone würde eine lokale Historie erzeugen, die
// niemand pflegt. Wer dort committet hat, soll das selbst auflösen.
func Update(projectDir string) (result UpdateResult, err error) {
	dir := PlaybookDir(projectDir)

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
	result.VersionChanged = versionBefore != versionAfter
	result.Version = versionAfter

	// Die MCP-Registrierung liegt im Hauptverzeichnis, nicht im Clone — der
	// Pull erreicht sie nicht. Ein Bestandsprojekt käme sonst mit einem Eintrag
	// aus dem Update, der auf den abgelösten Wrapper zeigt. Korrigiert wird nur
	// dieser eine Fall — das Update ergänzt keinen fehlenden Eintrag, das ist
	// Sache des Starts —, und ein Fehlschlag entwertet das Update nicht: der
	// Pull ist durch, und der nächste Start versucht es erneut.
	repaired, repairErr := RepairMCP(projectDir, MCPWriteOutdatedOnly)
	result.MCPRepaired = repaired
	if repairErr != nil {
		result.Message = "MCP-Registrierung nicht vollständig korrigiert: " + repairErr.Error()
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
