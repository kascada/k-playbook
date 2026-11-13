package release

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestMain schaltet die Quelle ab: Ein Test, der sie nicht ausdrücklich
// ersetzt, fragt nichts — nie das echte GitHub. (releasetest.Isolate tut
// dasselbe für andere Pakete; hier wäre der Import ein Zyklus.)
func TestMain(m *testing.M) {
	os.Setenv(EnvSource, SourceOff)
	os.Exit(m.Run())
}

// fakeGit ersetzt ls-remote: die Ausgabe steht fest, und gezählt wird, wie oft
// gefragt wurde. Kein Test dieses Pakets startet git oder berührt ein Netz.
func fakeGit(t *testing.T, output string, err error) *atomic.Int32 {
	t.Helper()
	var calls atomic.Int32
	before := RunGit
	RunGit = func(ctx context.Context, args ...string) ([]byte, error) {
		calls.Add(1)
		want := []string{"ls-remote", "--tags", "--refs", CurrentSource().Repo, "v*"}
		if strings.Join(args, " ") != strings.Join(want, " ") {
			t.Errorf("git-Aufruf = %q, erwartet %q", args, want)
		}
		return []byte(output), err
	}
	t.Cleanup(func() { RunGit = before })
	return &calls
}

// source stellt eine Ersatzquelle für Prüfsummen und Assets hin und setzt
// EnvSource darauf.
func source(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	t.Setenv(EnvSource, server.URL)
	return server
}

func refs(tags ...string) string {
	var b strings.Builder
	for i, tag := range tags {
		fmt.Fprintf(&b, "%040x\trefs/tags/%s\n", i+1, tag)
	}
	return b.String()
}

// Der neueste Tag ist der höchste der Form vX.Y.Z — semantisch, nicht
// lexikalisch: v0.10.0 steht über v0.9.4.
func TestLatestTagNimmtDenHoechsten(t *testing.T) {
	t.Setenv(EnvSource, "")
	fakeGit(t, refs("v0.9.4", "v0.10.0", "v0.9.12", "v0.2.0"), nil)

	tag, err := LatestTag(context.Background())
	if err != nil {
		t.Fatalf("Fehler: %v", err)
	}
	if tag != "v0.10.0" {
		t.Errorf("neuester Tag = %q, erwartet v0.10.0", tag)
	}
}

// Tags mit Zusatz sind keine Releases: ein Vorabkennzeichen, Build-Metadaten,
// ein Name ohne v oder mit weniger als drei Zahlen zählen nicht — auch dann
// nicht, wenn sie höher wären.
func TestLatestTagUebergehtTagsMitZusatz(t *testing.T) {
	t.Setenv(EnvSource, "")
	fakeGit(t, refs("v0.9.4", "v0.10.0-rc.1", "v1.0.0-beta", "v0.9.5+build", "0.9.9", "v2.0", "vorlage", "v0.9.4.1"), nil)

	tag, err := LatestTag(context.Background())
	if err != nil {
		t.Fatalf("Fehler: %v", err)
	}
	if tag != "v0.9.4" {
		t.Errorf("neuester Tag = %q, erwartet v0.9.4", tag)
	}

	// Nur Tags mit Zusatz: es gibt kein Release.
	fakeGit(t, refs("v0.10.0-rc.1", "vorlage"), nil)
	if _, err := LatestTag(context.Background()); !errors.Is(err, ErrNoTag) {
		t.Errorf("Fehler = %v, erwartet ErrNoTag", err)
	}
}

// Ohne Netz, ohne git und bei Zeitüberschreitung ist der Tag unbekannt: ein
// TagError mit Grund, nie ein leerer Tag ohne Fehler.
func TestLatestTagOhneNetzIstUnbekannt(t *testing.T) {
	t.Setenv(EnvSource, "")

	fakeGit(t, "", errors.New("exit status 128: fatal: unable to access 'https://github.com/kascada/k-playbook.git/': Could not resolve host: github.com"))
	tag, err := LatestTag(context.Background())
	var tagErr *TagError
	if tag != "" || !errors.As(err, &tagErr) {
		t.Fatalf("ohne Netz: Tag %q, Fehler %v", tag, err)
	}
	if !strings.Contains(tagErr.Reason, "Could not resolve host") {
		t.Errorf("der Grund nennt die Meldung von git nicht: %q", tagErr.Reason)
	}

	fakeGit(t, "", &exec.Error{Name: "git", Err: exec.ErrNotFound})
	_, err = LatestTag(context.Background())
	if !errors.As(err, &tagErr) || !strings.Contains(tagErr.Reason, "git fehlt im PATH") {
		t.Errorf("ohne git: %v", err)
	}

	before := TagTimeout
	TagTimeout = 20 * time.Millisecond
	t.Cleanup(func() { TagTimeout = before })
	beforeGit := RunGit
	RunGit = func(ctx context.Context, args ...string) ([]byte, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	t.Cleanup(func() { RunGit = beforeGit })
	_, err = LatestTag(context.Background())
	if !errors.As(err, &tagErr) || !strings.Contains(tagErr.Reason, "nicht geantwortet") {
		t.Errorf("Zeitüberschreitung: %v", err)
	}
}

// Läuft die Frist ab, endet git samt seinem Kindprozess: Für HTTPS startet
// git einen Helfer, der die Ausgabe offen hält. Hier ein Ersatz-git, das einen
// Kindprozess mit geerbter Ausgabe startet und selbst hängt. Träfe das Signal
// nur git, kehrte LatestTag erst nach WaitDelay (1 s) zurück.
func TestLatestTagFristBeendetAuchDenKindprozess(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh fehlt")
	}
	dir := t.TempDir()
	// Der Kindprozess erbt stdout und stderr und hält sie 30 Sekunden offen.
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte("#!/bin/sh\nsleep 30 &\nsleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv(EnvSource, "http://127.0.0.1:9")
	before := TagTimeout
	TagTimeout = 150 * time.Millisecond
	t.Cleanup(func() { TagTimeout = before })

	start := time.Now()
	_, err := LatestTag(context.Background())
	elapsed := time.Since(start)
	var tagErr *TagError
	if !errors.As(err, &tagErr) || !strings.Contains(tagErr.Reason, "nicht geantwortet") {
		t.Fatalf("Fehler = %v", err)
	}
	if elapsed > 700*time.Millisecond {
		t.Errorf("LatestTag kehrte erst nach %s zurück — der Kindprozess hielt die Ausgabe offen", elapsed)
	}
}

// Abgeschaltet heißt: kein Aufruf von git, keine Anfrage — nur ErrDisabled.
func TestAbgeschalteteQuelleFragtNichts(t *testing.T) {
	t.Setenv(EnvSource, SourceOff)
	calls := fakeGit(t, refs("v0.9.4"), nil)

	if _, err := LatestTag(context.Background()); !errors.Is(err, ErrDisabled) {
		t.Errorf("LatestTag: %v", err)
	}
	if _, err := Checksums(context.Background(), "v0.9.4"); !errors.Is(err, ErrDisabled) {
		t.Errorf("Checksums: %v", err)
	}
	target := filepath.Join(t.TempDir(), "bin", "k-playbook")
	if _, err := Install(context.Background(), "v0.9.4", target); !errors.Is(err, ErrDisabled) {
		t.Errorf("Install: %v", err)
	}
	if calls.Load() != 0 {
		t.Errorf("git wurde %d-mal aufgerufen", calls.Load())
	}
	if _, err := os.Stat(filepath.Dir(target)); err == nil {
		t.Error("das Zielverzeichnis wurde angelegt")
	}
	if note := CurrentSource().Note(); !strings.Contains(note, EnvSource) {
		t.Errorf("Note nennt die Variable nicht: %q", note)
	}
}

// Die Quelle aus der Umgebung: leer ist GitHub, eine Basis-URL ersetzt alle
// drei Orte, und Note sagt es.
func TestCurrentSource(t *testing.T) {
	t.Setenv(EnvSource, "")
	github := CurrentSource()
	if github.Repo != RepoURL || github.Replaced || github.Off || github.Note() != "" {
		t.Errorf("ohne Variable: %+v, Note %q", github, github.Note())
	}
	if got := github.SumsURL("v0.9.4"); got != "https://raw.githubusercontent.com/kascada/k-playbook/v0.9.4/SHA256SUMS" {
		t.Errorf("SumsURL = %s", got)
	}
	if got := github.AssetURL("v0.9.4", "k-playbook-linux-amd64"); got != "https://github.com/kascada/k-playbook/releases/download/v0.9.4/k-playbook-linux-amd64" {
		t.Errorf("AssetURL = %s", got)
	}

	t.Setenv(EnvSource, "http://127.0.0.1:9/")
	replaced := CurrentSource()
	if replaced.Repo != "http://127.0.0.1:9/k-playbook.git" || replaced.Raw != "http://127.0.0.1:9/raw" ||
		replaced.Download != "http://127.0.0.1:9/download" || !replaced.Replaced {
		t.Errorf("ersetzt: %+v", replaced)
	}
	if note := replaced.Note(); !strings.Contains(note, "http://127.0.0.1:9") {
		t.Errorf("Note = %q", note)
	}
	// Dieselbe Naht ersetzt die eine Adresse von Kephalaion.
	if github.KephalaionInstall != KephalaionInstallURL || replaced.KephalaionInstall != "http://127.0.0.1:9/kephalaion/install.sh" {
		t.Errorf("install.sh von Kephalaion: %q / %q", github.KephalaionInstall, replaced.KephalaionInstall)
	}
	if FirstInstallCommand != "curl -fsSL https://raw.githubusercontent.com/kascada/k-playbook/main/bin/install | bash" {
		t.Errorf("FirstInstallCommand = %q", FirstInstallCommand)
	}
}

// Fetch lädt ganz oder gar nicht: bei 404 und bei abgeschalteter Quelle
// entsteht keine Datei.
func TestFetch(t *testing.T) {
	server := source(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/kephalaion/install.sh" {
			fmt.Fprint(w, "echo hallo\n")
			return
		}
		http.NotFound(w, r)
	})
	dir := t.TempDir()

	path := filepath.Join(dir, "install.sh")
	if err := Fetch(context.Background(), CurrentSource().KephalaionInstall, path); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if content, _ := os.ReadFile(path); string(content) != "echo hallo\n" {
		t.Errorf("Inhalt = %q", content)
	}

	missing := filepath.Join(dir, "fehlt.sh")
	if err := Fetch(context.Background(), server.URL+"/gibt-es-nicht", missing); err == nil || !strings.Contains(err.Error(), "404") {
		t.Errorf("404: %v", err)
	}
	if _, err := os.Stat(missing); err == nil {
		t.Error("bei 404 entstand eine Datei")
	}

	t.Setenv(EnvSource, SourceOff)
	off := filepath.Join(dir, "aus.sh")
	if err := Fetch(context.Background(), server.URL+"/kephalaion/install.sh", off); !errors.Is(err, ErrDisabled) {
		t.Errorf("abgeschaltet: %v", err)
	}
	if _, err := os.Stat(off); err == nil {
		t.Error("bei abgeschalteter Quelle entstand eine Datei")
	}
}

func sumOf(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// release liefert einen Handler mit SHA256SUMS und Asset eines Tags.
func releaseHandler(tag string, sums string, assets map[string][]byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/raw/"+tag+"/"+SumsFileName && sums != "":
			fmt.Fprint(w, sums)
		case strings.HasPrefix(r.URL.Path, "/download/"+tag+"/"):
			content, ok := assets[strings.TrimPrefix(r.URL.Path, "/download/"+tag+"/")]
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.Write(content)
		default:
			http.NotFound(w, r)
		}
	}
}

// installTarget legt ein Zielverzeichnis an, das im PATH steht.
func installTarget(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return filepath.Join(dir, "k-playbook")
}

// Install lädt das Asset dieser Plattform, prüft es gegen die SHA256SUMS des
// Tags und legt es ausführbar am Ziel ab; daneben bleibt nichts liegen.
func TestInstallLaedtPrueftUndErsetzt(t *testing.T) {
	content := []byte("#!/bin/sh\necho v0.9.4\n")
	// Beide Schreibweisen der Datei: zwei Leerzeichen und der Stern davor.
	sums := sumOf([]byte("fremd")) + "  k-playbook-fremd-arch\n" + sumOf(content) + " *" + AssetName() + "\n"
	source(t, releaseHandler("v0.9.4", sums, map[string][]byte{AssetName(): content}))
	target := installTarget(t)
	if err := os.WriteFile(target, []byte("alt"), 0o755); err != nil {
		t.Fatal(err)
	}

	result, err := Install(context.Background(), "v0.9.4", target)
	if err != nil {
		t.Fatalf("Fehler: %v", err)
	}
	if result.Sum != sumOf(content) || result.Asset != AssetName() || result.Tag != "v0.9.4" || result.Foreign != "" {
		t.Errorf("Ergebnis = %+v", result)
	}
	got, err := os.ReadFile(target)
	if err != nil || string(got) != string(content) {
		t.Fatalf("Ziel = %q, %v", got, err)
	}
	info, _ := os.Stat(target)
	if info.Mode().Perm() != 0o755 {
		t.Errorf("Rechte = %v, erwartet 0755", info.Mode().Perm())
	}
	entries, _ := os.ReadDir(filepath.Dir(target))
	if len(entries) != 1 {
		t.Errorf("neben dem Ziel liegt noch etwas: %v", entries)
	}
	if err := Verify(context.Background(), "v0.9.4", target); err != nil {
		t.Errorf("Verify: %v", err)
	}
}

// Fehlt das Asset (404), ist das Release noch nicht fertig: die Meldung sagt
// das, und am Ziel bleibt, was dort lag.
func TestInstallAssetFehlt(t *testing.T) {
	sums := sumOf([]byte("x")) + "  " + AssetName() + "\n"
	source(t, releaseHandler("v0.9.4", sums, nil))
	target := installTarget(t)
	if err := os.WriteFile(target, []byte("alt"), 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := Install(context.Background(), "v0.9.4", target)
	var missing *AssetMissingError
	if !errors.As(err, &missing) {
		t.Fatalf("Fehler = %v, erwartet AssetMissingError", err)
	}
	if !strings.Contains(err.Error(), "noch nicht fertig") || missing.Tag != "v0.9.4" || missing.Asset != AssetName() {
		t.Errorf("Meldung = %q", err)
	}
	assertUntouched(t, target, "alt")
}

// Weicht die Prüfsumme ab, wird nichts ersetzt, und der Download bleibt nicht
// liegen.
func TestInstallPruefsummeWeichtAb(t *testing.T) {
	sums := sumOf([]byte("etwas anderes")) + "  " + AssetName() + "\n"
	source(t, releaseHandler("v0.9.4", sums, map[string][]byte{AssetName(): []byte("ausgetauscht")}))
	target := installTarget(t)
	if err := os.WriteFile(target, []byte("alt"), 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := Install(context.Background(), "v0.9.4", target)
	var mismatch *ChecksumError
	if !errors.As(err, &mismatch) {
		t.Fatalf("Fehler = %v, erwartet ChecksumError", err)
	}
	if mismatch.Expected != sumOf([]byte("etwas anderes")) || mismatch.Actual != sumOf([]byte("ausgetauscht")) {
		t.Errorf("Summen = %+v", mismatch)
	}
	assertUntouched(t, target, "alt")
}

// Nennt SHA256SUMS des Tags das Asset nicht, wird gar nicht erst geladen.
func TestInstallEintragFehltInSHA256SUMS(t *testing.T) {
	var downloads atomic.Int32
	inner := releaseHandler("v0.9.4", sumOf([]byte("x"))+"  k-playbook-fremd-arch\n", map[string][]byte{AssetName(): []byte("neu")})
	source(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/download/") {
			downloads.Add(1)
		}
		inner(w, r)
	})
	target := installTarget(t)
	if err := os.WriteFile(target, []byte("alt"), 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := Install(context.Background(), "v0.9.4", target)
	var missing *NoChecksumError
	if !errors.As(err, &missing) || missing.Asset != AssetName() {
		t.Fatalf("Fehler = %v, erwartet NoChecksumError", err)
	}
	if downloads.Load() != 0 {
		t.Errorf("trotz fehlendem Eintrag wurde geladen (%d-mal)", downloads.Load())
	}
	assertUntouched(t, target, "alt")

	// Fehlt die Datei selbst, ist das ein eigener Fehler mit dem Tag.
	source(t, releaseHandler("v0.9.4", "", nil))
	if _, err := Checksums(context.Background(), "v0.9.4"); err == nil || !strings.Contains(err.Error(), "v0.9.4") {
		t.Errorf("ohne SHA256SUMS: %v", err)
	}
	if err := Verify(context.Background(), "v0.9.4", target); err == nil {
		t.Error("Verify ohne SHA256SUMS ohne Fehler")
	}
}

// Steht das Zielverzeichnis nicht im PATH, bricht Install vor dem Download
// ab: ein Programm dort fände kein Aufruf. Das Verzeichnis bleibt angelegt.
func TestInstallZielverzeichnisNichtImPATH(t *testing.T) {
	var requests atomic.Int32
	inner := releaseHandler("v0.9.4", sumOf([]byte("neu"))+"  "+AssetName()+"\n", map[string][]byte{AssetName(): []byte("neu")})
	source(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		inner(w, r)
	})
	dir := filepath.Join(t.TempDir(), "bin")
	target := filepath.Join(dir, "k-playbook")

	_, err := Install(context.Background(), "v0.9.4", target)
	var pathErr *PathError
	if !errors.As(err, &pathErr) || !pathErr.Created || pathErr.Dir != dir {
		t.Fatalf("Fehler = %v", err)
	}
	if requests.Load() != 0 {
		t.Errorf("vor der PATH-Prüfung wurde gefragt (%d-mal)", requests.Load())
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Errorf("das Zielverzeichnis fehlt: %v", err)
	}
	if _, err := os.Stat(target); err == nil {
		t.Error("am Ziel liegt eine Datei")
	}

	// Beim zweiten Mal gibt es das Verzeichnis schon; der Hinweis ist ein
	// anderer.
	_, err = Install(context.Background(), "v0.9.4", target)
	if !errors.As(err, &pathErr) || pathErr.Created {
		t.Errorf("zweiter Lauf: %v", err)
	}
}

// Ein Symlink am Ziel wird selbst ersetzt, nicht sein bisheriges Ziel.
func TestInstallErsetztSymlinkSelbst(t *testing.T) {
	content := []byte("neu")
	source(t, releaseHandler("v0.9.4", sumOf(content)+"  "+AssetName()+"\n", map[string][]byte{AssetName(): content}))
	target := installTarget(t)
	elsewhere := filepath.Join(t.TempDir(), "wrapper")
	if err := os.WriteFile(elsewhere, []byte("wrapper"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(elsewhere, target); err != nil {
		t.Fatal(err)
	}

	if _, err := Install(context.Background(), "v0.9.4", target); err != nil {
		t.Fatalf("Fehler: %v", err)
	}
	info, err := os.Lstat(target)
	if err != nil || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("am Ziel liegt noch ein Link: %v, %v", info, err)
	}
	assertUntouched(t, target, "neu")
	assertUntouched(t, elsewhere, "wrapper")
}

// Ein plattformfremdes Binary am Ziel — Host und DevContainer mit demselben
// HOME — wird ersetzt und genannt. Erkannt wird an den Magic-Bytes; alles
// andere bleibt stumm.
func TestForeignPlatformKind(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, content []byte) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, content, 0o755); err != nil {
			t.Fatal(err)
		}
		return path
	}
	elf := write("elf", []byte{0x7f, 'E', 'L', 'F', 2, 1, 1, 0})
	macho := write("macho", []byte{0xcf, 0xfa, 0xed, 0xfe, 7, 0, 0, 1})
	universal := write("universal", []byte{0xca, 0xfe, 0xba, 0xbe, 0, 0, 0, 2})
	script := write("script", []byte("#!/bin/sh\n"))
	short := write("short", []byte{0x7f})
	link := filepath.Join(dir, "link")
	if err := os.Symlink(macho, link); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		goos string
		path string
		want string
	}{
		{"linux", macho, "macOS"},
		{"linux", universal, "macOS"},
		{"linux", elf, ""},
		{"darwin", elf, "Linux"},
		{"darwin", macho, ""},
		{"linux", script, ""},
		{"linux", short, ""},
		{"linux", link, ""},
		{"linux", filepath.Join(dir, "fehlt"), ""},
	}
	for _, test := range tests {
		if got := foreignPlatformKind(test.goos, test.path); got != test.want {
			t.Errorf("foreignPlatformKind(%s, %s) = %q, erwartet %q", test.goos, filepath.Base(test.path), got, test.want)
		}
	}

	if note := (Installed{Foreign: "macOS"}).ForeignNote(); !strings.Contains(note, "macOS") || !strings.Contains(note, FirstInstallCommand) {
		t.Errorf("Hinweis = %q", note)
	}
	if note := (Installed{}).ForeignNote(); note != "" {
		t.Errorf("ohne fremdes Binary ein Hinweis: %q", note)
	}
}

// DirInPath vergleicht auch den physischen Pfad: ein Eintrag über einen
// Symlink meint dasselbe Verzeichnis.
func TestDirInPath(t *testing.T) {
	base := t.TempDir()
	real := filepath.Join(base, "echt")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", "/usr/bin"+string(os.PathListSeparator)+link)
	if !DirInPath(real) {
		t.Error("über den Symlink im PATH nicht erkannt")
	}
	if !DirInPath(link) {
		t.Error("die Schreibweise selbst nicht erkannt")
	}
	t.Setenv("PATH", "/usr/bin"+string(os.PathListSeparator))
	if DirInPath(real) {
		t.Error("ohne Eintrag erkannt")
	}
}

func TestIsReleaseTag(t *testing.T) {
	for tag, want := range map[string]bool{
		"v0.16.1": true, "v1.0.0": true, "v10.20.30": true,
		"0.16.1": false, "v0.16": false, "v0.16.1-rc.1": false, "v0.16.1+x": false,
		"v0.16.1.2": false, "": false, "vX.Y.Z": false, "v0.16.1/..": false,
	} {
		if got := IsReleaseTag(tag); got != want {
			t.Errorf("IsReleaseTag(%q) = %v", tag, got)
		}
	}
}

func assertUntouched(t *testing.T, path string, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil || string(got) != want {
		t.Errorf("%s = %q, %v — erwartet %q", filepath.Base(path), got, err, want)
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".k-playbook.") {
			t.Errorf("der Download blieb liegen: %s", entry.Name())
		}
	}
}
