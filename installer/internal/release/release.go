// Package release kennt GitHub für k-playbook: den neuesten Release-Tag, die
// Prüfsummen eines Tags und das Release-Binary dieser Plattform — dazu die
// eine Adresse von Kephalaion, die k-playbook lädt: das install.sh des
// neuesten Kephalaion-Releases.
//
// Es gibt genau zwei Versionsquellen — die eigene Version im Binary
// (buildinfo.Version) und den neuesten Tag auf GitHub —, und dieses Paket ist
// die zweite. Gelesen wird über HTTPS und anonym: das Repo ist öffentlich,
// kein Nutzer braucht einen Schlüssel. Das ist eine bewusste Ausnahme von der
// Regel, Netzbefehle von git über `k-playbook github run` zu führen.
//
//   - Der Tag kommt aus `git ls-remote --tags --refs <repo> 'v*'`: der höchste
//     der Form vX.Y.Z, ohne Zusatz. Er steht, sobald er gepusht ist — also
//     schon, bevor die CI die Assets gebaut hat.
//   - Die Prüfsummen kommen aus der committeten SHA256SUMS des Tags, über
//     raw.githubusercontent.com — nicht als Release-Asset: Die erwartete Summe
//     kommt so über den Git-Stand, das Binary über das Release, und ein
//     ausgetauschtes Asset fällt auf.
//   - Das Binary kommt aus den Assets des Releases.
//
// Ein Blattpaket: es kennt weder project noch program, damit beide es nutzen
// (die Umgebungsprüfung in project, Vergleich und Installation in program).
//
// Für Tests ist die Quelle austauschbar, auch für gebaute Unterprozesse: über
// die Umgebungsvariable EnvSource (Testhilfe: Paket releasetest).
package release

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/kascada/k-playbook/installer/internal/version"
)

// Die Orte auf GitHub.
const (
	// RepoURL ist das öffentliche Repo, gegen das ls-remote fragt.
	RepoURL = "https://github.com/kascada/k-playbook.git"
	// RawBaseURL trägt die committeten Dateien je Tag: <basis>/<tag>/SHA256SUMS.
	RawBaseURL = "https://raw.githubusercontent.com/kascada/k-playbook"
	// DownloadBaseURL trägt die Assets je Release: <basis>/<tag>/<asset>.
	DownloadBaseURL = "https://github.com/kascada/k-playbook/releases/download"
	// InstallScriptURL ist bin/install auf main: die Erstinstallation.
	InstallScriptURL = "https://raw.githubusercontent.com/kascada/k-playbook/main/bin/install"
	// KephalaionInstallURL ist das install.sh des neuesten Kephalaion-Releases.
	// Asset-Namen, Plattformen und Prüfsumme von Kephalaion bleiben damit
	// Sache von Kephalaion.
	KephalaionInstallURL = "https://github.com/kephalaion/kephalaion/releases/latest/download/install.sh"
)

// FirstInstallCommand ist die Erstinstallation von k-playbook: bin/install aus
// dem Repo, eigenständig über curl | bash aufrufbar. Es bestimmt Tag und
// Prüfsumme wie dieses Paket und installiert danach Kephalaion, wenn es fehlt.
const FirstInstallCommand = "curl -fsSL " + InstallScriptURL + " | bash"

// SumsFileName ist die Prüfsummendatei im Repo, dieselbe, gegen die die CI
// das Release baut.
const SumsFileName = "SHA256SUMS"

// assetPrefix ist der Name des Programms in den Asset-Namen.
const assetPrefix = "k-playbook"

// EnvSource ersetzt die Release-Quelle oder schaltet sie ab — die Testnaht,
// die auch gebaute Unterprozesse erreicht, weil sie die Umgebung erben:
//
//   - leer: GitHub.
//   - SourceOff: kein Abruf. LatestTag, Checksums und Install enden mit
//     ErrDisabled, ohne das Netz zu berühren.
//   - sonst eine Basis-URL: das Repo liegt unter <basis>/k-playbook.git, die
//     Prüfsummen unter <basis>/raw/<tag>/SHA256SUMS, die Assets unter
//     <basis>/download/<tag>/<asset>, das install.sh von Kephalaion unter
//     <basis>/kephalaion/install.sh.
//
// bin/install liest dieselbe Variable mit derselben Bedeutung.
//
// Die Variable wirkt auch im ausgelieferten Programm. Damit das niemand
// übersieht, nennen `k-playbook version --all` und `k-playbook upgrade` eine
// ersetzte oder abgeschaltete Quelle (Source.Note).
const EnvSource = "K_PLAYBOOK_RELEASE_SOURCE"

// SourceOff ist der Wert von EnvSource, der jeden Abruf abschaltet.
const SourceOff = "off"

// Fristen. Variablen, damit Tests den Weg über die Frist gehen können.
var (
	// TagTimeout begrenzt die Frage nach dem neuesten Tag. Sie läuft in der
	// Umgebungsprüfung und damit in jedem Briefing; ohne Antwort ist der Tag
	// unbekannt, und nichts wartet länger.
	TagTimeout = 3 * time.Second
	// ChecksumTimeout begrenzt das Laden von SHA256SUMS — eine kleine Datei.
	ChecksumTimeout = 30 * time.Second
)

// ErrDisabled: die Release-Quelle ist über EnvSource abgeschaltet.
var ErrDisabled = errors.New("die Release-Abfrage ist abgeschaltet (" + EnvSource + "=" + SourceOff + ")")

// ErrNoTag: das Repo nennt keinen Tag der Form vX.Y.Z.
var ErrNoTag = errors.New("das Repo nennt keinen Release-Tag der Form vX.Y.Z")

// Source sind die Orte, von denen gelesen wird.
type Source struct {
	// Repo ist die Adresse für ls-remote.
	Repo string
	// Raw und Download sind die Basis-URLs für Prüfsummen und Assets.
	Raw      string
	Download string
	// KephalaionInstall ist die Adresse des install.sh von Kephalaion.
	KephalaionInstall string
	// Off: kein Abruf.
	Off bool
	// Replaced: die Quelle kommt aus EnvSource, nicht von GitHub.
	Replaced bool
}

// CurrentSource liest die Quelle aus der Umgebung dieses Prozesses.
func CurrentSource() Source {
	value := strings.TrimSpace(os.Getenv(EnvSource))
	switch value {
	case "":
		return Source{Repo: RepoURL, Raw: RawBaseURL, Download: DownloadBaseURL, KephalaionInstall: KephalaionInstallURL}
	case SourceOff:
		return Source{Off: true, Replaced: true}
	}
	base := strings.TrimRight(value, "/")
	return Source{
		Repo:              base + "/k-playbook.git",
		Raw:               base + "/raw",
		Download:          base + "/download",
		KephalaionInstall: base + "/kephalaion/install.sh",
		Replaced:          true,
	}
}

// Note sagt in einem Halbsatz, dass die Quelle nicht GitHub ist. Leer im
// Normalfall.
func (s Source) Note() string {
	switch {
	case s.Off:
		return "Release-Quelle abgeschaltet über " + EnvSource + "=" + SourceOff
	case s.Replaced:
		return "Release-Quelle ersetzt über " + EnvSource + ": " + strings.TrimSuffix(s.Repo, "/k-playbook.git")
	}
	return ""
}

// SumsURL ist die Adresse der Prüfsummendatei eines Tags.
func (s Source) SumsURL(tag string) string {
	return s.Raw + "/" + tag + "/" + SumsFileName
}

// AssetURL ist die Adresse eines Assets eines Releases.
func (s Source) AssetURL(tag string, asset string) string {
	return s.Download + "/" + tag + "/" + asset
}

// AssetName ist der Name des Release-Assets dieser Plattform, wie er in
// SHA256SUMS steht.
func AssetName() string {
	return assetPrefix + "-" + runtime.GOOS + "-" + runtime.GOARCH
}

// releaseTag ist die Form eines Release-Tags: vX.Y.Z ohne Zusatz. Ein Tag mit
// Vorabkennzeichen oder Build-Metadaten ist kein Release.
var releaseTag = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)

// IsReleaseTag sagt, ob tag die Form vX.Y.Z hat.
func IsReleaseTag(tag string) bool {
	return releaseTag.MatchString(tag) && version.Valid(tag)
}

// RunGit führt git aus und liefert stdout. Tests ersetzen die Variable, damit
// sie weder git noch ein Netz brauchen. Der Prozess fragt nie nach Zugangs-
// daten (GIT_TERMINAL_PROMPT=0) und läuft außerhalb jedes Repos, damit die
// Einstellungen des Arbeitsverzeichnisses nicht mitreden.
//
// Läuft die Frist ab, endet die ganze Prozessgruppe: git startet für HTTPS
// einen Kindprozess (git-remote-https), der die Ausgabe offen hält. Träfe das
// Signal nur git selbst, kehrte der Aufruf erst nach WaitDelay zurück — das
// Briefing wartete dann eine Sekunde länger als die Frist (gemessen in
// Task 149: 4,5 s statt 3,5 s bei hängender Verbindung).
var RunGit = func(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = os.TempDir()
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	// Das Netz für den Fall, dass ein Enkelprozess die Gruppe verlassen hat
	// und die Pipe weiter offen hält.
	cmd.WaitDelay = time.Second
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		if text := firstLine(stderr.String()); text != "" {
			return output, fmt.Errorf("%w: %s", err, text)
		}
		return output, err
	}
	return output, nil
}

// TagError: der neueste Tag ließ sich nicht bestimmen — ohne Netz, ohne git
// oder bei Zeitüberschreitung. Der Tag ist dann unbekannt, nicht „keiner“.
type TagError struct {
	// Reason ist der Grund als Halbsatz, ohne Punkt.
	Reason string
	cause  error
}

func (e *TagError) Error() string { return e.Reason }
func (e *TagError) Unwrap() error { return e.cause }

// LatestTag fragt das Repo nach seinen Tags und nennt den höchsten der Form
// vX.Y.Z, verglichen nach version.Compare — nicht lexikalisch, sonst stünde
// v0.9.4 über v0.10.0.
//
// Ein Fehler heißt: der Tag ist unbekannt. Aufrufer, die nur warnen
// (Umgebungsprüfung), schweigen dann; `upgrade` meldet den Grund.
func LatestTag(ctx context.Context) (string, error) {
	source := CurrentSource()
	if source.Off {
		return "", ErrDisabled
	}
	ctx, cancel := context.WithTimeout(ctx, TagTimeout)
	defer cancel()

	output, err := RunGit(ctx, "ls-remote", "--tags", "--refs", source.Repo, "v*")
	switch {
	case ctx.Err() == context.DeadlineExceeded:
		return "", &TagError{Reason: fmt.Sprintf("GitHub hat nach %s nicht geantwortet", TagTimeout), cause: ctx.Err()}
	case errors.Is(err, exec.ErrNotFound):
		return "", &TagError{Reason: "git fehlt im PATH; der neueste Release-Tag wird über git ls-remote gelesen", cause: err}
	case err != nil:
		return "", &TagError{Reason: "die Tags ließen sich nicht lesen (git ls-remote " + source.Repo + "): " + err.Error(), cause: err}
	}

	latest := ""
	for _, tag := range parseTags(output) {
		if latest == "" || version.Compare(tag, latest) == version.Newer {
			latest = tag
		}
	}
	if latest == "" {
		return "", ErrNoTag
	}
	return latest, nil
}

// parseTags liest die Ausgabe von ls-remote — je Zeile „<hash>\trefs/tags/<name>“
// — und behält die Tags der Form vX.Y.Z.
func parseTags(output []byte) []string {
	var tags []string
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		_, ref, ok := strings.Cut(strings.TrimSpace(scanner.Text()), "\t")
		if !ok {
			continue
		}
		name, ok := strings.CutPrefix(strings.TrimSpace(ref), "refs/tags/")
		if !ok || !IsReleaseTag(name) {
			continue
		}
		tags = append(tags, name)
	}
	return tags
}

// Sums sind die Einträge einer SHA256SUMS: Asset-Name auf sha256 in
// Kleinbuchstaben.
type Sums map[string]string

// NoChecksumError: SHA256SUMS des Tags nennt das Asset nicht.
type NoChecksumError struct {
	Tag   string
	Asset string
}

func (e *NoChecksumError) Error() string {
	return fmt.Sprintf("keine Prüfsumme für %s in %s von %s", e.Asset, SumsFileName, e.Tag)
}

// For liefert die Summe eines Assets.
func (s Sums) For(tag string, asset string) (string, error) {
	sum, ok := s[asset]
	if !ok {
		return "", &NoChecksumError{Tag: tag, Asset: asset}
	}
	return sum, nil
}

// HTTPClient lädt Prüfsummen und Assets. Ohne eigene Frist: die setzt der
// Kontext des Aufrufers. Umleitungen folgt er — die Assets eines Releases
// liegen hinter einer.
var HTTPClient = &http.Client{}

// Checksums lädt die SHA256SUMS des Tags aus dem Repo und liest ihre
// Einträge, in der Form „<sha256>  <name>“ oder „<sha256> *<name>“.
func Checksums(ctx context.Context, tag string) (Sums, error) {
	source := CurrentSource()
	if source.Off {
		return nil, ErrDisabled
	}
	if !IsReleaseTag(tag) {
		return nil, fmt.Errorf("kein Release-Tag: %q", tag)
	}
	ctx, cancel := context.WithTimeout(ctx, ChecksumTimeout)
	defer cancel()

	url := source.SumsURL(tag)
	response, err := get(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("%s von %s nicht ladbar: %w", SumsFileName, tag, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s von %s nicht ladbar: %s antwortet mit %s", SumsFileName, tag, url, response.Status)
	}

	sums := Sums{}
	// Die Datei trägt eine Zeile je Plattform; ein Megabyte ist weit mehr,
	// als sie je braucht, und begrenzt eine Antwort, die keine ist.
	scanner := bufio.NewScanner(io.LimitReader(response.Body, 1<<20))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 {
			continue
		}
		sums[strings.TrimPrefix(fields[1], "*")] = strings.ToLower(fields[0])
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("%s von %s nicht lesbar: %w", SumsFileName, tag, err)
	}
	return sums, nil
}

func get(ctx context.Context, url string) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", assetPrefix)
	return HTTPClient.Do(request)
}

// AssetMissingError: das Release trägt das Asset (noch) nicht. Der Tag steht,
// bevor die CI die Assets gebaut hat; in diesem Fenster ist das Release noch
// nicht fertig.
type AssetMissingError struct {
	Tag   string
	Asset string
	URL   string
}

func (e *AssetMissingError) Error() string {
	return fmt.Sprintf("das Release %s ist noch nicht fertig: das Asset %s liegt noch nicht bereit (%s antwortet mit 404); nichts wurde ersetzt",
		e.Tag, e.Asset, e.URL)
}

// ChecksumError: der Download trägt nicht die Summe aus SHA256SUMS.
type ChecksumError struct {
	Asset    string
	Expected string
	Actual   string
}

func (e *ChecksumError) Error() string {
	return fmt.Sprintf("die Prüfsumme weicht ab für %s (erwartet %s, erhalten %s); nichts wurde ersetzt",
		e.Asset, e.Expected, e.Actual)
}

// PathError: das Verzeichnis des Installationsziels steht nicht im PATH. Ein
// Programm dort wäre installiert, aber unter seinem Namen nicht auffindbar.
type PathError struct {
	Dir string
	// Created: das Verzeichnis gab es bis eben nicht. Debian und Ubuntu nehmen
	// ~/.local/bin in ~/.profile nur auf, wenn es beim Anmelden existiert —
	// dann reicht eine neue Anmeldung.
	Created bool
}

func (e *PathError) Error() string {
	if e.Created {
		return fmt.Sprintf("%s liegt nicht im PATH; das Verzeichnis ist jetzt angelegt — neu anmelden oder das Profil neu laden, dann erneut aufrufen", e.Dir)
	}
	return fmt.Sprintf("%s liegt nicht im PATH; ins Shell-Profil aufnehmen (export PATH=\"%s:$PATH\"), dann erneut aufrufen", e.Dir, e.Dir)
}

// Installed ist der Ausgang von Install.
type Installed struct {
	Tag    string `json:"tag"`
	Asset  string `json:"asset"`
	URL    string `json:"url"`
	Target string `json:"target"`
	// Sum ist die geprüfte sha256 der installierten Datei.
	Sum string `json:"sum"`
	// Foreign nennt die fremde Plattform („macOS“ oder „Linux“), wenn am Ziel
	// vorher ein Binary der anderen Plattform lag — Host und DevContainer
	// teilen sich dann dasselbe HOME und überschreiben einander.
	Foreign string `json:"foreign,omitempty"`
}

// ForeignNote ist der Hinweis zu einem ersetzten plattformfremden Binary.
// Leer, wenn keines ersetzt wurde.
func (i Installed) ForeignNote() string {
	if i.Foreign == "" {
		return ""
	}
	// Nicht `k-playbook upgrade`: In der anderen Umgebung liegt jetzt ein
	// Binary, das sich dort nicht ausführen lässt.
	return fmt.Sprintf("Ersetzt wurde ein %s-Binary — offenbar teilen sich Host und Container dasselbe HOME. "+
		"In der anderen Umgebung ist k-playbook damit bis auf Weiteres tot; dort erneut installieren: %s", i.Foreign, FirstInstallCommand)
}

// Install lädt das Asset dieser Plattform aus dem Release tag und legt es als
// target ab.
//
// Die Reihenfolge ist die von bin/install: Zielverzeichnis anlegen,
// PATH prüfen, Prüfsumme des Tags lesen — alles vor dem Download, damit ein
// Abbruch nichts geladen und nichts ersetzt hat. Dann der Download in eine
// temporäre Datei neben dem Ziel, die Prüfsumme, chmod 0755 und das Umbenennen
// auf das Ziel: atomar, halb ersetzt bleibt die Datei nie. Ein Symlink am Ziel
// wird selbst ersetzt, nicht sein bisheriges Ziel.
//
// Die PATH-Prüfung hier ist die des Verzeichnisses. Ob `k-playbook` im PATH
// genau auf das Ziel zeigt, prüft program.CheckPath davor: das braucht ein
// vorhandenes Programm und gilt nur für die Aktualisierung.
func Install(ctx context.Context, tag string, target string) (Installed, error) {
	result := Installed{Tag: tag, Asset: AssetName(), Target: target}
	source := CurrentSource()
	if source.Off {
		return result, ErrDisabled
	}
	if !IsReleaseTag(tag) {
		return result, fmt.Errorf("kein Release-Tag: %q", tag)
	}
	result.URL = source.AssetURL(tag, result.Asset)

	dir := filepath.Dir(target)
	created := false
	if _, err := os.Stat(dir); err != nil {
		created = true
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return result, fmt.Errorf("Zielverzeichnis nicht anlegbar: %w", err)
	}
	if !DirInPath(dir) {
		return result, &PathError{Dir: dir, Created: created}
	}
	result.Foreign = foreignPlatformKind(runtime.GOOS, target)

	sums, err := Checksums(ctx, tag)
	if err != nil {
		return result, err
	}
	expected, err := sums.For(tag, result.Asset)
	if err != nil {
		return result, err
	}

	temporary, err := os.CreateTemp(dir, ".k-playbook.*")
	if err != nil {
		return result, fmt.Errorf("temporäre Datei neben dem Ziel nicht anlegbar: %w", err)
	}
	keep := false
	defer func() {
		temporary.Close()
		if !keep {
			os.Remove(temporary.Name())
		}
	}()

	actual, err := download(ctx, result, temporary)
	if err != nil {
		return result, err
	}
	if actual != expected {
		return result, &ChecksumError{Asset: result.Asset, Expected: expected, Actual: actual}
	}
	if err := temporary.Close(); err != nil {
		return result, fmt.Errorf("Download nicht vollständig geschrieben: %w", err)
	}
	if err := os.Chmod(temporary.Name(), 0o755); err != nil {
		return result, err
	}
	// Ein alter Symlink am Ziel wird selbst ersetzt, nicht sein bisheriges
	// Ziel: rename(2) folgt dem Link nicht. bin/install muss ihn dafür vorher
	// entfernen, weil `mv` in einen Link auf ein Verzeichnis hineinverschöbe.
	if err := os.Rename(temporary.Name(), target); err != nil {
		return result, fmt.Errorf("Ziel nicht ersetzbar: %w", err)
	}
	keep = true
	result.Sum = actual
	return result, nil
}

// download lädt das Asset nach file und liefert dessen sha256.
func download(ctx context.Context, result Installed, file *os.File) (string, error) {
	response, err := get(ctx, result.URL)
	if err != nil {
		return "", fmt.Errorf("%s nicht ladbar: %w", result.URL, err)
	}
	defer response.Body.Close()
	switch {
	case response.StatusCode == http.StatusNotFound:
		return "", &AssetMissingError{Tag: result.Tag, Asset: result.Asset, URL: result.URL}
	case response.StatusCode != http.StatusOK:
		return "", fmt.Errorf("%s nicht ladbar: die Antwort ist %s", result.URL, response.Status)
	}
	hash := sha256.New()
	if _, err := io.Copy(io.MultiWriter(file, hash), response.Body); err != nil {
		return "", fmt.Errorf("Download von %s abgebrochen: %w", result.URL, err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// Fetch lädt url vollständig nach path. Gedacht für ein Skript, das danach
// ausgeführt wird: erst ganz laden, dann ausführen — bricht der Download ab,
// läuft kein halbes Skript, und ein Fehler beim Laden ist von einem Fehler
// der Ausführung zu unterscheiden. Bei abgeschalteter Quelle ErrDisabled.
func Fetch(ctx context.Context, url string, path string) error {
	if CurrentSource().Off {
		return ErrDisabled
	}
	response, err := get(ctx, url)
	if err != nil {
		return fmt.Errorf("%s nicht ladbar: %w", url, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("%s nicht ladbar: die Antwort ist %s", url, response.Status)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(file, response.Body); err != nil {
		file.Close()
		os.Remove(path)
		return fmt.Errorf("Download von %s abgebrochen: %w", url, err)
	}
	return file.Close()
}

// Verify prüft die Datei unter target gegen den Eintrag des Plattform-Assets
// in der SHA256SUMS des Tags.
func Verify(ctx context.Context, tag string, target string) error {
	sums, err := Checksums(ctx, tag)
	if err != nil {
		return err
	}
	expected, err := sums.For(tag, AssetName())
	if err != nil {
		return err
	}
	actual, err := FileSum(target)
	if err != nil {
		return fmt.Errorf("installierte Datei nicht lesbar: %w", err)
	}
	if actual != expected {
		return fmt.Errorf("die installierte Datei %s passt nicht zu %s von %s (erwartet %s, gefunden %s)",
			target, SumsFileName, tag, expected, actual)
	}
	return nil
}

// FileSum ist die sha256 einer Datei, hexadezimal in Kleinbuchstaben.
func FileSum(path string) (string, error) {
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

// DirInPath meldet, ob dir im PATH dieses Prozesses steht — verglichen wird
// die Schreibweise selbst und, wo auflösbar, der physische Pfad. Auf macOS ist
// $HOME über /System/Volumes/Data verlinkt; ohne die Auflösung hielte der
// Vergleich zwei Schreibweisen desselben Verzeichnisses auseinander.
func DirInPath(dir string) bool {
	wanted := physicalPath(dir)
	for _, entry := range filepath.SplitList(os.Getenv("PATH")) {
		if entry == "" {
			continue
		}
		if entry == dir || entry == wanted || physicalPath(entry) == wanted {
			return true
		}
	}
	return false
}

func physicalPath(dir string) string {
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		return resolved
	}
	return filepath.Clean(dir)
}

// foreignPlatformKind meldet, welcher fremden Plattform ein vorhandenes
// Binary am Ziel angehört: „macOS“, „Linux“ oder leer.
//
// Erkannt wird an den Magic-Bytes. Gemeldet wird nur, was sich positiv als
// Format der jeweils anderen Plattform lesen lässt; alles andere (Skript,
// Symlink, Fragment) bleibt stumm.
func foreignPlatformKind(goos string, path string) string {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return ""
	}
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()
	magic := make([]byte, 4)
	if _, err := io.ReadFull(file, magic); err != nil {
		return ""
	}
	switch goos + ":" + hex.EncodeToString(magic) {
	case "darwin:7f454c46":
		// ELF auf einem Darwin-Host.
		return "Linux"
	case "linux:cffaedfe", "linux:cefaedfe", "linux:feedface", "linux:feedfacf", "linux:cafebabe", "linux:bebafeca":
		// Mach-O (32/64 Bit, beide Endianness) und Universal Binaries auf Linux.
		return "macOS"
	}
	return ""
}

func firstLine(text string) string {
	text = strings.TrimSpace(text)
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		text = text[:i]
	}
	return strings.TrimSpace(text)
}
