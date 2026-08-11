package project

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	// updateCheckTimeout begrenzt die Netzwerkabfrage. Sie laeuft im
	// Hintergrund; ein haengender Remote darf die Oberflaeche nicht blockieren.
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

// CheckUpdate fragt den Remote-Stand ab, ohne etwas zu veraendern.
//
// Bewusst `git ls-remote` statt `git fetch`: die Pruefung laeuft ungefragt nach
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
		return UpdateStatus{Branch: branch, Message: "Kein Upstream fuer diesen Branch konfiguriert."}, nil
	}
	mergeRef, err := GitOutput(ctx, dir, "config", "--get", "branch."+branch+".merge")
	if err != nil || mergeRef == "" {
		return UpdateStatus{Branch: branch, Message: "Kein Upstream fuer diesen Branch konfiguriert."}, nil
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
	// BinaryChanged meldet, ob sich die ausgelieferten Binaries geaendert
	// haben. Nur dann bringt ein Neustart eine andere Programmversion.
	BinaryChanged bool   `json:"binaryChanged"`
	Message       string `json:"message"`
}

// Update holt den neuen Stand per Fast-Forward.
//
// Nur `--ff-only`: ein Merge im Clone wuerde eine lokale Historie erzeugen, die
// niemand pflegt. Wer dort committet hat, soll das selbst aufloesen.
func Update(projectDir string) (UpdateResult, error) {
	dir := PlaybookDir(projectDir)
	before := binaryHashes(dir)

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

	result := UpdateResult{Output: text}
	result.BinaryChanged = !sameHashes(before, binaryHashes(dir))
	return result, nil
}

// binaryHashes bildet die ausgelieferten Binaries ab. Aendern sie sich, laeuft
// der aktuelle Prozess weiterhin mit dem alten Code: unter Linux behaelt er
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

// GitOutput fuehrt ein Git-Kommando in dir aus und liefert die getrimmte
// Ausgabe. Exportiert, weil auch hostinstall den Commit-Stand einer
// Installation braucht.
func GitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}
