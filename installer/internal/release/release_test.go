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
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestMain schaltet die Quelle ab: Ein Test, der sie nicht ausdrücklich
// ersetzt, fragt nichts — nie das echte GitHub. (releasetest.Isolate tut
// dasselbe für andere Pakete; hier wäre der Import ein Zyklus.)
//
// Dazu zeigt XDG_CACHE_HOME auf ein temporäres Verzeichnis: Kein Test liest
// oder schreibt den Cache im echten ~/.cache.
func TestMain(m *testing.M) {
	os.Setenv(EnvSource, SourceOff)
	cache, err := os.MkdirTemp("", "k-playbook-cache-*")
	if err != nil {
		panic(err)
	}
	os.Setenv("XDG_CACHE_HOME", cache)
	code := m.Run()
	os.RemoveAll(cache)
	os.Exit(code)
}

// source stellt eine Ersatzquelle hin und setzt EnvSource darauf.
func source(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	t.Setenv(EnvSource, server.URL)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	return server
}

// latest ist eine Ersatzquelle, die /releases/latest mit status beantwortet und
// dabei auf location weiterleitet (leer: kein Location-Kopf). Gezählt wird,
// wie oft gefragt wurde, und es gilt: nur HEAD auf /releases/latest.
func latest(t *testing.T, status int, location string) *atomic.Int32 {
	t.Helper()
	var calls atomic.Int32
	source(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/releases/latest" || r.Method != http.MethodHead {
			t.Errorf("Anfrage %s %s, erwartet HEAD /releases/latest", r.Method, r.URL.Path)
		}
		if location != "" {
			w.Header().Set("Location", location)
		}
		w.WriteHeader(status)
	})
	return &calls
}

// Der Tag ist das Ziel der Weiterleitung von /releases/latest — gefolgt wird
// ihr nicht: Die Seite des Release fragt niemand.
func TestLatestTagLiestDieWeiterleitung(t *testing.T) {
	var calls atomic.Int32
	server := source(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path == "/releases/latest" {
			http.Redirect(w, r, "/releases/tag/v0.10.0", http.StatusFound)
			return
		}
		t.Errorf("der Weiterleitung wurde gefolgt: %s", r.URL.Path)
	})

	tag, err := LatestTag(context.Background())
	if err != nil || tag != "v0.10.0" {
		t.Fatalf("LatestTag = %q, %v", tag, err)
	}
	if calls.Load() != 1 {
		t.Errorf("%d Anfragen, erwartet eine", calls.Load())
	}

	// Ein absolutes Ziel, wie GitHub es schickt.
	latest(t, http.StatusFound, server.URL+"/releases/tag/v0.16.38")
	// latest ersetzt EnvSource durch einen neuen Server; dessen Adresse ist
	// eine andere — das Ziel unter der alten gilt nicht als Release.
	if _, err := LatestTag(context.Background()); err == nil {
		t.Error("ein Ziel unter einer fremden Adresse galt als Release")
	}
	var calls2 atomic.Int32
	server2 := source(t, func(w http.ResponseWriter, r *http.Request) {
		calls2.Add(1)
		w.Header().Set("Location", "http://"+r.Host+"/releases/tag/v0.16.38")
		w.WriteHeader(http.StatusFound)
	})
	tag, err = LatestTag(context.Background())
	if err != nil || tag != "v0.16.38" {
		t.Fatalf("absolutes Ziel unter %s: %q, %v", server2.URL, tag, err)
	}
}

// Ein Tag mit Zusatz ist kein Release, auch wenn GitHub ihn als neuestes
// nennt: der Tag ist dann unbekannt, ein TagError — kein Rückgriff auf ein
// anderes Release.
func TestLatestTagMitZusatzIstUnbekannt(t *testing.T) {
	for _, tag := range []string{"v0.10.0-rc.1", "v1.0.0-beta", "v0.9.5+build", "0.9.9", "v2.0", "vorlage", "v0.9.4.1"} {
		latest(t, http.StatusFound, "/releases/tag/"+tag)
		got, err := LatestTag(context.Background())
		var tagErr *TagError
		if got != "" || !errors.As(err, &tagErr) || !strings.Contains(tagErr.Reason, "keinen der Form vX.Y.Z") {
			t.Errorf("%s: Tag %q, Fehler %v", tag, got, err)
		}
	}
}

// Ohne Weiterleitung, mit 404 (kein veröffentlichtes Release), mit einem Ziel,
// das kein Release ist, und ohne Netz ist der Tag unbekannt: ein TagError mit
// Grund, nie ein leerer Tag ohne Fehler.
func TestLatestTagOhneWeiterleitungIstUnbekannt(t *testing.T) {
	for _, fall := range []struct {
		name     string
		status   int
		location string
		want     string
	}{
		{"keine Weiterleitung", http.StatusOK, "", "statt mit einer Weiterleitung"},
		{"404", http.StatusNotFound, "", "kein veröffentlichtes Release"},
		{"Serverfehler", http.StatusBadGateway, "", "502"},
		{"Weiterleitung ohne Ziel", http.StatusFound, "", "nennt aber kein Ziel"},
		{"Ziel ist die Liste", http.StatusFound, "/releases", "leitet nicht auf ein Release weiter"},
		{"Ziel mit Unterpfad", http.StatusFound, "/releases/tag/v0.10.0/mehr", "leitet nicht auf ein Release weiter"},
	} {
		t.Run(fall.name, func(t *testing.T) {
			latest(t, fall.status, fall.location)
			tag, err := LatestTag(context.Background())
			var tagErr *TagError
			if tag != "" || !errors.As(err, &tagErr) || !strings.Contains(tagErr.Reason, fall.want) {
				t.Errorf("Tag %q, Fehler %v; erwartet %q im Grund", tag, err, fall.want)
			}
		})
	}

	t.Run("ohne Netz", func(t *testing.T) {
		// Ein Port, auf dem niemand lauscht.
		listener := httptest.NewServer(http.NotFoundHandler())
		address := listener.URL
		listener.Close()
		t.Setenv(EnvSource, address)
		tag, err := LatestTag(context.Background())
		var tagErr *TagError
		if tag != "" || !errors.As(err, &tagErr) || !strings.Contains(tagErr.Reason, "ließ sich nicht erfragen") {
			t.Errorf("Tag %q, Fehler %v", tag, err)
		}
	})
}

// Läuft die Frist ab, kehrt LatestTag mit ihr zurück, nicht erst, wenn die
// Quelle antwortet.
func TestLatestTagFristAbgelaufen(t *testing.T) {
	release := make(chan struct{})
	source(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	})
	t.Cleanup(func() { close(release) })
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
		t.Errorf("LatestTag kehrte erst nach %s zurück", elapsed)
	}
}

// Abgeschaltet heißt: keine Anfrage — nur ErrDisabled.
func TestAbgeschalteteQuelleFragtNichts(t *testing.T) {
	calls := latest(t, http.StatusFound, "/releases/tag/v0.9.4")
	t.Setenv(EnvSource, SourceOff)

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
		t.Errorf("die Quelle wurde %d-mal gefragt", calls.Load())
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
	if github.Latest != "https://github.com/kascada/k-playbook/releases/latest" ||
		github.TagPrefix != "https://github.com/kascada/k-playbook/releases/tag/" ||
		github.Replaced || github.Off || github.Note() != "" {
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
	if replaced.Latest != "http://127.0.0.1:9/releases/latest" || replaced.TagPrefix != "http://127.0.0.1:9/releases/tag/" || replaced.Raw != "http://127.0.0.1:9/raw" ||
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
