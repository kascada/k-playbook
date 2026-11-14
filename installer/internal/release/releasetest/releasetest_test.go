package releasetest_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/release"
	"github.com/kascada/k-playbook/installer/internal/release/releasetest"
)

// Die Ersatzquelle wird mit dem echten HTTP-Client gefragt: Tag, Prüfsummen
// und Asset kommen von ihr, nie von GitHub. Das neueste Release ist der
// höchste Tag der Form vX.Y.Z.
func TestErsatzquelle(t *testing.T) {
	source := releasetest.Serve(t)
	source.AddRelease("v0.9.4", []byte("alt"))
	source.AddRelease("v0.10.0", []byte("neu"))
	source.AddTag("v0.11.0-rc.1")

	tag, err := release.LatestTag(context.Background())
	if err != nil {
		t.Fatalf("LatestTag: %v", err)
	}
	if tag != "v0.10.0" {
		t.Errorf("neuester Tag = %q, erwartet v0.10.0", tag)
	}

	dir := filepath.Join(t.TempDir(), "bin")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	target := filepath.Join(dir, "k-playbook")
	if _, err := release.Install(context.Background(), tag, target); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if content, _ := os.ReadFile(target); string(content) != "neu" {
		t.Errorf("Ziel = %q", content)
	}

	// Ein Release ohne Asset: Install meldet es als nicht fertig.
	source.AddTag("v0.10.1")
	source.SetSums("v0.10.1", "00  "+release.AssetName()+"\n")
	tag, err = release.LatestTag(context.Background())
	if err != nil || tag != "v0.10.1" {
		t.Fatalf("LatestTag = %q, %v", tag, err)
	}
	var missing *release.AssetMissingError
	if _, err := release.Install(context.Background(), tag, target); !errors.As(err, &missing) {
		t.Errorf("Install ohne Asset: %v", err)
	}

	// Als neuestes markiert ist ein Tag mit Zusatz: der Tag ist unbekannt.
	source.SetLatest("v0.11.0-rc.1")
	if tag, err := release.LatestTag(context.Background()); err == nil {
		t.Errorf("LatestTag = %q, erwartet einen Fehler", tag)
	}
	if len(source.Requests()) == 0 {
		t.Error("die Ersatzquelle wurde nicht gefragt")
	}
}

// Isolate schaltet die Quelle ab, lenkt den Cache in ein temporäres
// Verzeichnis und stellt beides danach wieder her.
func TestIsolate(t *testing.T) {
	t.Setenv(release.EnvSource, "http://127.0.0.1:9")
	t.Setenv("XDG_CACHE_HOME", "/vorher")
	cleanup := releasetest.Isolate()
	if _, err := release.LatestTag(context.Background()); !errors.Is(err, release.ErrDisabled) {
		t.Errorf("abgeschaltet: %v", err)
	}
	cache := os.Getenv("XDG_CACHE_HOME")
	if cache == "/vorher" || !filepath.IsAbs(cache) {
		t.Errorf("XDG_CACHE_HOME = %q", cache)
	}
	if _, err := os.Stat(cache); err != nil {
		t.Errorf("das Cache-Verzeichnis fehlt: %v", err)
	}
	cleanup()
	if got := os.Getenv(release.EnvSource); got != "http://127.0.0.1:9" {
		t.Errorf("nach dem Aufräumen: %q", got)
	}
	if got := os.Getenv("XDG_CACHE_HOME"); got != "/vorher" {
		t.Errorf("XDG_CACHE_HOME nach dem Aufräumen: %q", got)
	}
	if _, err := os.Stat(cache); !os.IsNotExist(err) {
		t.Errorf("das temporäre Cache-Verzeichnis blieb liegen: %v", err)
	}
}

// Serve lenkt den Cache in ein temporäres Verzeichnis; das Ergebnis der
// Ersatzquelle landet dort, nie im echten ~/.cache.
func TestServeSchreibtNurInDenTemporaerenCache(t *testing.T) {
	home := os.Getenv("HOME")
	releasetest.Serve(t).AddTag("v0.9.4")
	dir := release.CacheDir()
	if home != "" && strings.HasPrefix(dir, filepath.Join(home, ".cache")) {
		t.Fatalf("Cache unter dem echten HOME: %s", dir)
	}
	if _, err := release.LatestTag(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, release.CacheFileName)); err != nil {
		t.Errorf("kein Eintrag im temporären Cache: %v", err)
	}
}

// Ohne Release der Form vX.Y.Z antwortet /releases/latest mit 404.
func TestErsatzquelleOhneRelease(t *testing.T) {
	source := releasetest.Serve(t)
	source.AddTag("v0.11.0-rc.1")
	var tagErr *release.TagError
	if _, err := release.LatestTag(context.Background()); !errors.As(err, &tagErr) {
		t.Errorf("Fehler = %v", err)
	}
	if source.Latest() != "" {
		t.Errorf("Latest = %q", source.Latest())
	}
}
