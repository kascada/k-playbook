package project

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// gitCallTimeout begrenzt einen einzelnen git-Aufruf. Alle Aufrufe sind rein
// lokal; wer länger braucht, antwortet nicht mehr sinnvoll.
const gitCallTimeout = 10 * time.Second

// runGit führt einen git-Aufruf im Verzeichnis aus.
//
// Zurück kommen stdout, der Exit-Code und ein Grund im Klartext. Der Code ist
// -1, wenn git gar nicht gelaufen ist — nicht installiert oder Timeout; die
// Aufrufer unterscheiden 0, 1 und alles darüber.
func runGit(ctx context.Context, dir string, args ...string) (string, int, string) {
	return runGitCommand(ctx, dir, nil, args...)
}

// gitUntranslatedEnv hält git bei der englischen Originalmeldung. LC_ALL=C
// schlägt LANG und LC_MESSAGES, das leere LANGUAGE hebt die GNU-Liste auf,
// die sonst sogar LC_ALL überstimmt.
var gitUntranslatedEnv = []string{"LC_ALL=C", "LANGUAGE="}

// runGitUntranslated ist runGit mit gitUntranslatedEnv — für den einen Fall,
// in dem der Aufrufer den Grund am Wortlaut auswertet, statt ihn anzuzeigen.
// Alle anderen Aufrufer behalten die Sprache des Nutzers, weil ihr Grund in
// der Oberfläche erscheint.
func runGitUntranslated(ctx context.Context, dir string, args ...string) (string, int, string) {
	return runGitCommand(ctx, dir, gitUntranslatedEnv, args...)
}

// runGitCommand ist der gemeinsame Aufruf. env ergänzt die geerbte Umgebung;
// bei doppelten Schlüsseln gilt der letzte Wert (os/exec).
func runGitCommand(ctx context.Context, dir string, env []string, args ...string) (string, int, string) {
	var stdout, stderr bytes.Buffer

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	err := cmd.Run()

	switch {
	case ctx.Err() != nil:
		return "", -1, fmt.Sprintf("git %s hat nicht innerhalb von %s geantwortet", args[0], gitCallTimeout)
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
