package program

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/kascada/k-playbook/installer/internal/guiproc"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// VersionCommand ist das Subkommando, mit dem eine installierte Datei ihre
// Version nennt. Programme vor seiner Einführung kennen es nicht und enden mit
// einem Fehler; das ergibt „unbekannt".
const VersionCommand = "version"

// SumsFileName ist die Prüfsummendatei im Clone, dieselbe, gegen die
// bin/install prüft.
const SumsFileName = "SHA256SUMS"

var (
	// versionTimeout begrenzt `k-playbook version`. Das Subkommando rechnet
	// nichts; wer länger braucht, ist kein Programm, dessen Antwort zählt.
	versionTimeout = 10 * time.Second
	// BootstrapTimeout begrenzt den Lauf von bin/install: ein Download von
	// rund 12 MB, dazu curl mit 15 Sekunden Verbindungsfrist und zwei
	// Wiederholungen. Eine Variable, damit Tests den Weg über die Frist gehen
	// können.
	BootstrapTimeout = 3 * time.Minute
)

// ReadVersion liest die Version der Datei unter path über
// `<path> version`. Leer, wenn die Datei fehlt, das Subkommando nicht kennt,
// mit Fehler endet oder etwas anderes als genau eine Zeile ausgibt.
//
// Ausgeführt wird die Datei, nicht gelesen: die Version ist in das Programm
// gestempelt und steht nirgends daneben. Die Servermarke wird dabei aus der
// Umgebung genommen, sonst startete die Datei statt einer Auskunft einen
// zweiten Dienst.
func ReadVersion(path string) string {
	if info, err := os.Stat(path); err != nil || info.IsDir() {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), versionTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, path, VersionCommand)
	cmd.Env = withoutServeMarker(os.Environ())
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	text := strings.TrimSpace(string(output))
	if text == "" || strings.ContainsAny(text, "\n\r") {
		return ""
	}
	if _, ok := parse(text); !ok {
		return ""
	}
	return text
}

func withoutServeMarker(environ []string) []string {
	result := make([]string, 0, len(environ))
	for _, entry := range environ {
		if strings.HasPrefix(entry, guiproc.ServeEnv+"=") {
			continue
		}
		result = append(result, entry)
	}
	return result
}

// Result ist der Ausgang von Install.
type Result struct {
	// Target ist das Installationsziel, aus dem neu gestartet wird.
	Target string `json:"target"`
	// Expected ist die Version, die der neue Dienst melden muss: nach einem
	// Bootstrap die VERSION des Clones, sonst die an der Datei gelesene.
	Expected string `json:"expected"`
	// Bootstrapped: bin/install ist gelaufen.
	Bootstrapped bool `json:"bootstrapped"`
	// Output ist die Ausgabe von bin/install, stdout und stderr zusammen.
	Output string `json:"output,omitempty"`
	// ExitCode von bin/install; -1, wenn es nicht bis zu einem Ende kam.
	ExitCode int `json:"exitCode"`
}

// Install sorgt dafür, dass am Installationsziel ein Programm liegt, das zur
// VERSION des Clones passt oder neuer ist.
//
// Zuerst wird der PATH geprüft (CheckPath): zeigt `k-playbook` nicht auf das
// Ziel, geschieht nichts. Dann wird die Version der Datei am Ziel gelesen.
// Gleich oder neuer als VERSION: kein Download — die Datei hat ein anderes
// Projekt oder `make dev-install` dort abgelegt, und herabgestuft wird nie.
// Älter oder unbekannt: bin/install des Clones läuft, aus dem
// Hauptverzeichnis, mit Zeitlimit.
//
// Erfolg nach einem Bootstrap heißt nicht Exit 0, sondern: die Datei am Ziel
// trägt die sha256 aus SHA256SUMS des Clones für diese Plattform. Das ist
// dieselbe Quelle, gegen die bin/install den Download prüft, und die Prüfung
// kommt ohne Ausführen der Datei aus.
//
// Ein Fehler lässt den Aufrufer weiterlaufen. bin/install ersetzt die Datei
// atomar per mv; halb ersetzt bleibt sie nicht.
func Install(projectDir string) (Result, error) {
	result := Result{ExitCode: -1}

	path := CheckPath()
	result.Target = path.Target
	if !path.OK {
		return result, errors.New(path.Hint())
	}

	playbookDir := project.PlaybookDir(projectDir)
	clone := project.InstalledVersion(playbookDir)
	if _, ok := parse(clone); !ok {
		return result, fmt.Errorf("die Installation nennt keine lesbare VERSION (%q)", clone)
	}

	if have := ReadVersion(path.Target); Compare(have, clone) == Equal || Compare(have, clone) == Newer {
		result.Expected = have
		return result, nil
	}

	result.Bootstrapped = true
	output, exitCode, err := runBootstrap(projectDir, playbookDir)
	result.Output, result.ExitCode = output, exitCode
	if err != nil {
		return result, err
	}
	if err := Verify(playbookDir, path.Target); err != nil {
		return result, err
	}
	result.Expected = clone
	return result, nil
}

// runBootstrap ruft k-playbook/bin/install aus dem Hauptverzeichnis auf — so,
// wie die Doku es einem Nutzer sagt.
func runBootstrap(projectDir string, playbookDir string) (string, int, error) {
	script := filepath.Join(playbookDir, "bin", "install")
	if _, err := os.Stat(script); err != nil {
		return "", -1, fmt.Errorf("Bootstrap nicht gefunden: %s", script)
	}

	ctx, cancel := context.WithTimeout(context.Background(), BootstrapTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, script)
	cmd.Dir = projectDir
	cmd.Env = withoutServeMarker(os.Environ())
	// Ohne WaitDelay hielte ein Enkelprozess — curl — die Pipe nach dem
	// Abbruch offen, und CombinedOutput kehrte erst mit ihm zurück.
	cmd.WaitDelay = 5 * time.Second
	output, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(output))

	if ctx.Err() == context.DeadlineExceeded {
		return text, -1, fmt.Errorf("%s hat nach %s nicht geendet", project.BootstrapCommandNoMake, BootstrapTimeout)
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return text, exitErr.ExitCode(), fmt.Errorf("%s endete mit Exit-Code %d", project.BootstrapCommandNoMake, exitErr.ExitCode())
		}
		return text, -1, fmt.Errorf("%s nicht ausführbar: %v", project.BootstrapCommandNoMake, err)
	}
	return text, 0, nil
}

// AssetName ist der Name des Release-Assets dieser Plattform, wie er in
// SHA256SUMS steht.
func AssetName() string {
	return project.InstalledCommandName + "-" + runtime.GOOS + "-" + runtime.GOARCH
}

// Verify prüft die Datei unter target gegen den Eintrag des Plattform-Assets
// in SHA256SUMS des Clones.
func Verify(playbookDir string, target string) error {
	expected, err := expectedSum(filepath.Join(playbookDir, SumsFileName), AssetName())
	if err != nil {
		return err
	}
	actual, err := fileSum(target)
	if err != nil {
		return fmt.Errorf("installierte Datei nicht lesbar: %w", err)
	}
	if actual != expected {
		return fmt.Errorf("die installierte Datei %s passt nicht zu %s (erwartet %s, gefunden %s)",
			target, SumsFileName, expected, actual)
	}
	return nil
}

// expectedSum liest den Eintrag für asset, in der Form „<sha256>  <name>"
// oder „<sha256> *<name>" — dieselben beiden, die bin/install annimmt.
func expectedSum(sumsFile string, asset string) (string, error) {
	file, err := os.Open(sumsFile)
	if err != nil {
		return "", fmt.Errorf("%s nicht lesbar: %w", SumsFileName, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 {
			continue
		}
		if fields[1] == asset || fields[1] == "*"+asset {
			return strings.ToLower(fields[0]), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("%s nicht lesbar: %w", SumsFileName, err)
	}
	return "", fmt.Errorf("keine Prüfsumme für %s in %s", asset, sumsFile)
}

func fileSum(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
