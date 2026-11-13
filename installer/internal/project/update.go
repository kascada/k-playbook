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

	// Trägt der Clone noch die SSH-Adresse des offiziellen Repos, fragt die
	// Prüfung über HTTPS: sie soll keinen Schlüssel verlangen und den Clone
	// nicht anfassen. Umgestellt wird erst beim Update.
	query := remoteName
	if public, ok := remotePublicURL(ctx, dir, remoteName); ok {
		query = public
	}

	remoteBranch := strings.TrimPrefix(mergeRef, "refs/heads/")
	line, err := GitOutput(ctx, dir, "ls-remote", "--heads", query, remoteBranch)
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
//
// Über Version und Installation des Programms sagt es nichts: Die entscheiden
// sich am neuesten Release-Tag (Paket release), nicht am Clone. Der Pull dient
// nur noch den Inhalten, die am Clone hängen.
type UpdateResult struct {
	Output string `json:"output"`
	// RemoteSwitched ist die HTTPS-Adresse, auf die der Upstream vor dem Pull
	// umgestellt wurde, weil er die SSH-Form des offiziellen Repos trug.
	RemoteSwitched string `json:"remoteSwitched,omitempty"`
}

// Update holt den neuen Stand per Fast-Forward.
//
// Nur `--ff-only`: ein Merge im Clone würde eine lokale Historie erzeugen, die
// niemand pflegt. Wer dort committet hat, soll das selbst auflösen.
//
// Die MCP-Registrierung repariert der Pull nicht mehr selbst: Sie liegt im
// Hauptverzeichnis, nicht im Clone, und hängt nicht daran, ob der Pull
// gelingt. Das tun die Aufrufer — `k-playbook upgrade` im Projektschritt, die
// Oberfläche nach dem Pull (RepairMCPReport).
func Update(projectDir string) (result UpdateResult, err error) {
	dir := PlaybookDir(projectDir)

	if err := setInstallationWritable(projectDir); err != nil {
		return UpdateResult{}, fmt.Errorf("Installation beschreibbar machen: %w", err)
	}
	defer keepInstallationReadOnly(projectDir, &err)

	ctx, cancel := context.WithTimeout(context.Background(), pullTimeout)
	defer cancel()

	switched, err := usePublicRemote(ctx, dir)
	if err != nil {
		return UpdateResult{}, fmt.Errorf("Remote auf %s umstellen: %w", PublicRepoURL, err)
	}

	cmd := exec.CommandContext(ctx, "git", "pull", "--ff-only")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(output))

	if ctx.Err() == context.DeadlineExceeded {
		return UpdateResult{Output: text, RemoteSwitched: switched}, fmt.Errorf("git pull hat nach %s nicht geantwortet", pullTimeout)
	}
	if err != nil {
		return UpdateResult{Output: text, RemoteSwitched: switched}, fmt.Errorf("git pull --ff-only fehlgeschlagen")
	}
	return UpdateResult{Output: text, RemoteSwitched: switched}, nil
}

// MCPRepairReport ist der Ausgang einer selbsttätigen Reparatur der
// MCP-Registrierung, so wie die Meldungen ihn brauchen.
type MCPRepairReport struct {
	// Repaired nennt die geschriebenen MCP-Dateien, relativ zum
	// Hauptverzeichnis.
	Repaired []string `json:"mcpRepaired,omitempty"`
	// Portable nennt die Dateien aus Repaired, in denen danach der bloße
	// Kommandoname steht — die Datei ist erfasst, oder die Tracking-Frage
	// blieb unbeantwortet. Der Hinweis auf den absoluten Pfad hängt an dieser
	// Form.
	Portable []string `json:"mcpRepairedPortable,omitempty"`
	// Message nennt, was nicht vollständig gelang. Ein Fehlschlag entwertet
	// den Lauf nicht: Der nächste Start versucht es erneut.
	Message string `json:"message,omitempty"`
}

// RepairMCPReport repariert die MCP-Registrierung im Umfang scope
// (RepairMCP) und fasst das Ergebnis für die Meldungen zusammen.
//
// Die Registrierung liegt im Hauptverzeichnis, nicht im Clone — ein Pull
// erreicht sie nicht. Ein Bestandsprojekt bliebe sonst bei einem Eintrag, der
// auf den abgelösten Wrapper zeigt. Wer wie weit geht: die Oberfläche nach
// „Aktualisieren“ mit MCPWriteOutdatedOnly, `k-playbook upgrade` im
// Projektschritt mit MCPWriteOutdatedAndUnversioned.
func RepairMCPReport(projectDir string, scope MCPWriteScope) MCPRepairReport {
	var report MCPRepairReport
	repaired, err := RepairMCP(projectDir, scope)
	report.Repaired = MCPWritePaths(repaired)
	for _, write := range repaired {
		if write.Portable() {
			report.Portable = append(report.Portable, write.Path)
		}
	}
	if err != nil {
		report.Message = "MCP-Registrierung nicht vollständig korrigiert: " + err.Error()
	}
	return report
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
