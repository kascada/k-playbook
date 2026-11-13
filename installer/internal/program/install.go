package program

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/kascada/k-playbook/installer/internal/guiproc"
	"github.com/kascada/k-playbook/installer/internal/release"
	"github.com/kascada/k-playbook/installer/internal/version"
)

// VersionCommand ist das Subkommando, mit dem eine installierte Datei ihre
// Version nennt. Programme vor seiner Einführung kennen es nicht und enden mit
// einem Fehler; das ergibt „unbekannt".
const VersionCommand = "version"

var (
	// versionTimeout begrenzt `k-playbook version`. Das Subkommando rechnet
	// nichts; wer länger braucht, ist kein Programm, dessen Antwort zählt.
	versionTimeout = 10 * time.Second
	// InstallTimeout begrenzt Download und Prüfung des Release-Binaries: rund
	// 20 MB und eine kleine Prüfsummendatei. Eine Variable, damit Tests den
	// Weg über die Frist gehen können.
	InstallTimeout = 3 * time.Minute
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
	if !version.Valid(text) {
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
	// Expected ist die Version, die der neue Dienst melden muss: nach einer
	// Installation der Tag, sonst die an der Datei gelesene.
	Expected string `json:"expected"`
	// Installed: das Release-Binary des Tags wurde geladen und hat die Datei
	// am Ziel ersetzt.
	Installed bool `json:"installed"`
	// Output nennt, was geladen und geprüft wurde, und den Hinweis zu einem
	// ersetzten plattformfremden Binary.
	Output string `json:"output,omitempty"`
}

// Install sorgt dafür, dass am Installationsziel ein Programm liegt, das dem
// Release-Tag tag entspricht oder neuer ist. tag ist der neueste Tag, so wie
// CheckStatus ihn gelesen hat (Status.Latest).
//
// Zuerst wird der PATH geprüft (CheckPath): zeigt `k-playbook` nicht auf das
// Ziel, geschieht nichts. Dann wird die Version der Datei am Ziel gelesen.
// Gleich oder neuer als der Tag: kein Download — die Datei hat ein anderer
// Lauf oder `make dev-install` dort abgelegt, und herabgestuft wird nie.
// Älter oder unbekannt: release.Install lädt das Asset dieser Plattform,
// prüft es gegen die SHA256SUMS des Tags und ersetzt die Datei atomar; halb
// ersetzt bleibt sie nie.
//
// Erfolg heißt: die Datei am Ziel trägt die sha256, die release.Install gegen
// die SHA256SUMS des Tags geprüft hat. Nachgerechnet wird an der Datei selbst,
// ohne sie auszuführen und ohne zweiten Abruf.
//
// Ein Fehler lässt den Aufrufer weiterlaufen. Der Clone des Projekts wird
// weder gelesen noch gebraucht.
func Install(ctx context.Context, tag string) (Result, error) {
	result := Result{}

	path := CheckPath()
	result.Target = path.Target
	if !path.OK {
		return result, errors.New(path.Hint())
	}
	if !release.IsReleaseTag(tag) {
		return result, fmt.Errorf("kein Release-Tag, gegen den installiert werden könnte (%q)", tag)
	}

	if have := ReadVersion(path.Target); version.Compare(have, tag) == version.Equal || version.Compare(have, tag) == version.Newer {
		result.Expected = have
		return result, nil
	}

	ctx, cancel := context.WithTimeout(ctx, InstallTimeout)
	defer cancel()
	installed, err := release.Install(ctx, tag, path.Target)
	if ctx.Err() == context.DeadlineExceeded {
		return result, fmt.Errorf("der Download von %s hat nach %s nicht geendet", installed.URL, InstallTimeout)
	}
	if err != nil {
		return result, err
	}
	result.Installed = true
	result.Output = describeInstalled(installed)

	actual, err := release.FileSum(path.Target)
	if err != nil {
		return result, fmt.Errorf("installierte Datei nicht lesbar: %w", err)
	}
	if actual != installed.Sum {
		return result, fmt.Errorf("die installierte Datei %s trägt nicht die geprüfte Summe (erwartet %s, gefunden %s)",
			path.Target, installed.Sum, actual)
	}
	result.Expected = tag
	return result, nil
}

// describeInstalled nennt Quelle und geprüfte Summe, dazu den Hinweis auf ein
// ersetztes plattformfremdes Binary.
func describeInstalled(installed release.Installed) string {
	lines := []string{
		"Geladen: " + installed.URL,
		"Geprüft gegen " + release.SumsFileName + " von " + installed.Tag + ": sha256 " + installed.Sum,
	}
	if note := installed.ForeignNote(); note != "" {
		lines = append(lines, note)
	}
	return strings.Join(lines, "\n")
}

// AssetName ist der Name des Release-Assets dieser Plattform, wie er in
// SHA256SUMS steht.
func AssetName() string {
	return release.AssetName()
}

// Verify prüft die Datei unter target gegen den Eintrag des Plattform-Assets
// in der SHA256SUMS des Release-Tags tag — geladen aus dem Repo, nicht aus
// dem Clone. Install braucht sie nicht: release.Install prüft vor dem
// Ersetzen. Sie ist die Frage „ist das am Ziel das Release-Binary von tag?“
// für eine Datei, die schon liegt.
func Verify(ctx context.Context, tag string, target string) error {
	return release.Verify(ctx, tag, target)
}
