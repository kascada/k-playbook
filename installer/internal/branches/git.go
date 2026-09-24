// Package branches liest die Branches des Code-Repos eines Projekts, ordnet
// ihnen Umgebungen zu, prüft vor einem Wechsel, ob er gefahrlos ist, und
// schaltet erst danach um.
//
// Das Paket kennt kein HTTP. Projekt, Code-Repo, der Abschnitt git: und die
// GitHub-Daten kommen von außen; gesucht wird nichts selbst. Alle git-Aufrufe
// gehen über einen github.Runner: dasselbe Interface, mit dem die GitHub-Ansicht
// ihre Subprozesse ausführt, und dieselbe Naht für Tests.
//
// Lesende Aufrufe laufen mit `git --no-optional-locks`: `git status` frischt sonst
// nebenbei den Index auf und schreibt damit in ein Repo, das hier nur gelesen
// werden soll. Ungefragt läuft kein `git fetch`; der ist eine eigene Aktion.
package branches

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/kascada/k-playbook/installer/internal/github"
)

// DefaultTimeout begrenzt einen einzelnen lesenden git-Aufruf. Die Frist einer
// ganzen Anfrage setzt der Aufrufer über den Kontext.
const DefaultTimeout = 15 * time.Second

// Repo bündelt Runner und Verzeichnis eines Repos.
type Repo struct {
	Runner github.Runner
	Dir    string
	// Timeout gilt je Aufruf; 0 heißt DefaultTimeout.
	Timeout time.Duration
}

func (r Repo) runner() github.Runner {
	if r.Runner == nil {
		return github.ExecRunner{}
	}
	return r.Runner
}

func (r Repo) timeout() time.Duration {
	if r.Timeout > 0 {
		return r.Timeout
	}
	return DefaultTimeout
}

// read führt einen lesenden git-Aufruf aus.
func (r Repo) read(ctx context.Context, args ...string) ([]byte, error) {
	return r.run(ctx, r.timeout(), append([]string{"--no-optional-locks"}, args...)...)
}

// readString liefert die Ausgabe ohne Leerraum am Ende.
func (r Repo) readString(ctx context.Context, args ...string) (string, error) {
	out, err := r.read(ctx, args...)
	return strings.TrimRight(string(out), "\r\n"), err
}

func (r Repo) run(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, &github.CommandError{Name: "git", Args: args, ExitCode: -1, Err: err}
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	out, err := r.runner().Run(callCtx, r.Dir, "git", args...)
	if err != nil {
		if cause := callCtx.Err(); cause != nil {
			return out, fmt.Errorf("%w: %w", cause, err)
		}
	}
	return out, err
}

// exitCode liefert den Exit-Code eines gescheiterten Aufrufs, -1 wenn keiner
// bekannt ist.
func exitCode(err error) int {
	var commandErr *github.CommandError
	if errors.As(err, &commandErr) {
		return commandErr.ExitCode
	}
	return -1
}

// stderrOf liefert, was git auf stderr geschrieben hat, sonst den Fehlertext.
func stderrOf(err error) string {
	var commandErr *github.CommandError
	if errors.As(err, &commandErr) {
		if text := strings.TrimSpace(commandErr.Stderr); text != "" {
			return text
		}
	}
	if err == nil {
		return ""
	}
	return err.Error()
}

// timedOut meldet, ob ein Aufruf an Frist oder Abbruch gescheitert ist.
func timedOut(err error) bool {
	return errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)
}

// firstLine kürzt eine git-Meldung auf ihre erste inhaltliche Zeile.
func firstLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return ""
}

// describeError macht aus einem gescheiterten Aufruf einen Satz für die Seite.
func describeError(subject string, err error) string {
	if timedOut(err) {
		return subject + ": git hat nicht rechtzeitig geantwortet."
	}
	if line := firstLine(stderrOf(err)); line != "" {
		return subject + ": " + line
	}
	return subject + " ist gescheitert."
}

// Version ist die Fassung des git-Binarys.
type Version struct {
	Major, Minor, Patch int
	Raw                 string
}

// AtLeast meldet, ob die Fassung mindestens major.minor ist.
func (v Version) AtLeast(major, minor int) bool {
	if v.Major != major {
		return v.Major > major
	}
	return v.Minor >= minor
}

func (v Version) String() string {
	if v.Raw != "" {
		return v.Raw
	}
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

var versionPattern = regexp.MustCompile(`(\d+)\.(\d+)(?:\.(\d+))?`)

// ParseVersion liest die Ausgabe von `git version`, etwa „git version 2.53.0"
// oder „git version 2.39.3 (Apple Git-146)".
func ParseVersion(output string) (Version, bool) {
	match := versionPattern.FindStringSubmatch(output)
	if match == nil {
		return Version{}, false
	}
	version := Version{Raw: strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(output), "git version "))}
	version.Major, _ = strconv.Atoi(match[1])
	version.Minor, _ = strconv.Atoi(match[2])
	if match[3] != "" {
		version.Patch, _ = strconv.Atoi(match[3])
	}
	return version, true
}

// GitVersion fragt die Fassung ab. Sie geht über den Runner und ist damit in
// Tests austauschbar: der Weg für git vor 2.41 lässt sich prüfen, ohne ein altes
// git zu installieren.
func (r Repo) GitVersion(ctx context.Context) (Version, error) {
	out, err := r.run(ctx, r.timeout(), "version")
	if err != nil {
		return Version{}, err
	}
	version, ok := ParseVersion(string(out))
	if !ok {
		return Version{}, fmt.Errorf("git version: unbekannte Ausgabe %q", strings.TrimSpace(string(out)))
	}
	return version, nil
}

// aheadBehindMinimum ist die erste git-Fassung mit %(ahead-behind:<ref>).
const aheadBehindMajor, aheadBehindMinor = 2, 41
