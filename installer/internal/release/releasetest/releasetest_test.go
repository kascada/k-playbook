package releasetest_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/release"
	"github.com/kascada/k-playbook/installer/internal/release/releasetest"
)

// Die Ersatzquelle wird mit dem echten git und dem echten HTTP-Client
// gefragt: Tag, Prüfsummen und Asset kommen von ihr, nie von GitHub.
func TestErsatzquelleMitEchtemGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git fehlt")
	}
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

	// Das Fenster zwischen Tag und Assets: der Tag steht, das Asset fehlt.
	source.AddTag("v0.10.1")
	source.SetSums("v0.10.1", "00  "+release.AssetName()+"\n")
	tag, err = release.LatestTag(context.Background())
	if err != nil || tag != "v0.10.1" {
		t.Fatalf("LatestTag = %q, %v", tag, err)
	}
	var missing *release.AssetMissingError
	if _, err := release.Install(context.Background(), tag, target); !errors.As(err, &missing) {
		t.Errorf("Install im Fenster: %v", err)
	}
	if len(source.Requests()) == 0 {
		t.Error("die Ersatzquelle wurde nicht gefragt")
	}
}

// Isolate schaltet die Quelle ab und stellt den alten Wert wieder her.
func TestIsolate(t *testing.T) {
	t.Setenv(release.EnvSource, "http://127.0.0.1:9")
	cleanup := releasetest.Isolate()
	if _, err := release.LatestTag(context.Background()); !errors.Is(err, release.ErrDisabled) {
		t.Errorf("abgeschaltet: %v", err)
	}
	cleanup()
	if got := os.Getenv(release.EnvSource); got != "http://127.0.0.1:9" {
		t.Errorf("nach dem Aufräumen: %q", got)
	}
}
