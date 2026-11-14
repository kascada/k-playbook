package release

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Die Tests hier stellen mit latest (release_test.go) eine Ersatzquelle hin,
// die auf v0.10.0 weiterleitet und ihre Anfragen zählt; latest setzt dabei
// XDG_CACHE_HOME auf ein eigenes temporäres Verzeichnis.

func cacheFile(t *testing.T) string {
	t.Helper()
	dir := CacheDir()
	if dir == "" || !strings.HasPrefix(dir, os.Getenv("XDG_CACHE_HOME")) {
		t.Fatalf("Cache-Verzeichnis %q liegt nicht unter dem temporären XDG_CACHE_HOME", dir)
	}
	return filepath.Join(dir, CacheFileName)
}

func putEntry(t *testing.T, entry cacheEntry) {
	t.Helper()
	path := cacheFile(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	content, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

func getEntry(t *testing.T) cacheEntry {
	t.Helper()
	content, err := os.ReadFile(cacheFile(t))
	if err != nil {
		t.Fatalf("kein Eintrag: %v", err)
	}
	var entry cacheEntry
	if err := json.Unmarshal(content, &entry); err != nil {
		t.Fatalf("Eintrag unlesbar: %v (%s)", err, content)
	}
	return entry
}

func replacedSource() string { return CurrentSource().Base }

// Ein frischer Eintrag der aktuellen Quelle: kein Abruf.
func TestCachedLatestTagFrischerEintrag(t *testing.T) {
	calls := latest(t, http.StatusFound, "/releases/tag/v0.10.0")
	putEntry(t, cacheEntry{Source: replacedSource(), Checked: time.Now().Add(-time.Hour), Tag: "v0.9.4"})

	tag, err := CachedLatestTag(context.Background())
	if err != nil || tag != "v0.9.4" {
		t.Fatalf("CachedLatestTag = %q, %v", tag, err)
	}
	if calls.Load() != 0 {
		t.Errorf("trotz frischem Eintrag %d-mal gefragt", calls.Load())
	}
}

// Ein abgelaufener Eintrag: Abruf, danach ein neuer Eintrag mit dem Tag und
// dem Zeitpunkt dieser Prüfung.
func TestCachedLatestTagAbgelaufenerEintrag(t *testing.T) {
	calls := latest(t, http.StatusFound, "/releases/tag/v0.10.0")
	putEntry(t, cacheEntry{Source: replacedSource(), Checked: time.Now().Add(-CacheSuccessTTL - time.Minute), Tag: "v0.9.4"})

	before := time.Now()
	tag, err := CachedLatestTag(context.Background())
	if err != nil || tag != "v0.10.0" {
		t.Fatalf("CachedLatestTag = %q, %v", tag, err)
	}
	if calls.Load() != 1 {
		t.Errorf("%d Anfragen, erwartet eine", calls.Load())
	}
	entry := getEntry(t)
	if entry.Tag != "v0.10.0" || entry.Error != "" || entry.Source != replacedSource() || entry.Checked.Before(before.Add(-time.Second)) {
		t.Errorf("neuer Eintrag: %+v", entry)
	}

	// Der neue Eintrag ist frisch: der nächste Aufruf fragt nicht.
	if tag, err := CachedLatestTag(context.Background()); err != nil || tag != "v0.10.0" || calls.Load() != 1 {
		t.Errorf("zweiter Aufruf: %q, %v, %d Anfragen", tag, err, calls.Load())
	}
}

// Ein frischer Fehlschlag: kein Abruf, die Quelle schweigt, der Grund kommt
// als TagError. Ist er älter als CacheFailureTTL, wird wieder gefragt.
func TestCachedLatestTagFrischerFehlschlag(t *testing.T) {
	calls := latest(t, http.StatusFound, "/releases/tag/v0.10.0")
	putEntry(t, cacheEntry{Source: replacedSource(), Checked: time.Now().Add(-5 * time.Minute), Error: "GitHub hat nach 3s nicht geantwortet"})

	tag, err := CachedLatestTag(context.Background())
	var tagErr *TagError
	if tag != "" || !errors.As(err, &tagErr) || tagErr.Reason != "GitHub hat nach 3s nicht geantwortet" {
		t.Fatalf("CachedLatestTag = %q, %v", tag, err)
	}
	if calls.Load() != 0 {
		t.Errorf("trotz frischem Fehlschlag %d-mal gefragt", calls.Load())
	}

	putEntry(t, cacheEntry{Source: replacedSource(), Checked: time.Now().Add(-CacheFailureTTL - time.Minute), Error: "alt"})
	if tag, err := CachedLatestTag(context.Background()); err != nil || tag != "v0.10.0" || calls.Load() != 1 {
		t.Errorf("abgelaufener Fehlschlag: %q, %v, %d Anfragen", tag, err, calls.Load())
	}
}

// Jedes Ergebnis landet im Cache, auch ein Fehlschlag; LatestTag fragt dabei
// immer, auch bei frischem Eintrag.
func TestLatestTagFragtImmerUndSchreibt(t *testing.T) {
	calls := latest(t, http.StatusFound, "/releases/tag/v0.10.0")
	putEntry(t, cacheEntry{Source: replacedSource(), Checked: time.Now(), Tag: "v0.9.4"})

	tag, err := LatestTag(context.Background())
	if err != nil || tag != "v0.10.0" || calls.Load() != 1 {
		t.Fatalf("LatestTag = %q, %v, %d Anfragen", tag, err, calls.Load())
	}
	if entry := getEntry(t); entry.Tag != "v0.10.0" {
		t.Errorf("Eintrag: %+v", entry)
	}

	latest(t, http.StatusNotFound, "")
	if _, err := LatestTag(context.Background()); err == nil {
		t.Fatal("kein Fehler bei 404")
	}
	if entry := getEntry(t); entry.Tag != "" || !strings.Contains(entry.Error, "kein veröffentlichtes Release") || entry.Source != replacedSource() {
		t.Errorf("Fehlschlag nicht im Cache: %+v", entry)
	}
}

// Bricht der Aufrufer selbst ab, bleibt der Cache unberührt: das sagt nichts
// über die Quelle.
func TestAbbruchDesAufrufersSchreibtNichts(t *testing.T) {
	latest(t, http.StatusFound, "/releases/tag/v0.10.0")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := LatestTag(ctx); err == nil {
		t.Fatal("kein Fehler trotz Abbruch")
	}
	if _, err := os.Stat(cacheFile(t)); !os.IsNotExist(err) {
		t.Errorf("nach dem Abbruch liegt ein Eintrag: %v", err)
	}
}

// Eine kaputte Datei gilt als kein Eintrag: Abruf, danach ein gültiger.
func TestCachedLatestTagKaputteDatei(t *testing.T) {
	for _, content := range []string{"{nicht json", "", `{"source":"` + "%s" + `","checked":"2026-11-14T00:00:00Z","tag":"v1.0.0-rc.1"}`, `{"source":"%s"}`} {
		calls := latest(t, http.StatusFound, "/releases/tag/v0.10.0")
		path := cacheFile(t)
		os.MkdirAll(filepath.Dir(path), 0o755)
		if err := os.WriteFile(path, []byte(strings.ReplaceAll(content, "%s", replacedSource())), 0o644); err != nil {
			t.Fatal(err)
		}
		tag, err := CachedLatestTag(context.Background())
		if err != nil || tag != "v0.10.0" || calls.Load() != 1 {
			t.Errorf("%q: %q, %v, %d Anfragen", content, tag, err, calls.Load())
		}
		if entry := getEntry(t); entry.Tag != "v0.10.0" {
			t.Errorf("%q: kein gültiger Eintrag danach: %+v", content, entry)
		}
	}
}

// Ist das Verzeichnis nicht beschreibbar, läuft alles wie ohne Cache: Tag ohne
// Fehler, jeder Aufruf fragt. Hier liegt an der Stelle des Verzeichnisses eine
// Datei — das sperrt auch unter root.
func TestCachedLatestTagNichtBeschreibbar(t *testing.T) {
	calls := latest(t, http.StatusFound, "/releases/tag/v0.10.0")
	if err := os.WriteFile(CacheDir(), []byte("eine Datei"), 0o644); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 2; i++ {
		tag, err := CachedLatestTag(context.Background())
		if err != nil || tag != "v0.10.0" || calls.Load() != int32(i) {
			t.Errorf("Aufruf %d: %q, %v, %d Anfragen", i, tag, err, calls.Load())
		}
	}
	if content, _ := os.ReadFile(CacheDir()); string(content) != "eine Datei" {
		t.Errorf("die Datei wurde verändert: %q", content)
	}
}

// Ein Eintrag einer anderen Quelle gilt als kein Eintrag: Abruf. So bedient
// eine Ersatzquelle den Cache der echten nie, und umgekehrt.
func TestCachedLatestTagFremdeQuelle(t *testing.T) {
	calls := latest(t, http.StatusFound, "/releases/tag/v0.10.0")
	putEntry(t, cacheEntry{Source: sourceGitHub, Checked: time.Now(), Tag: "v0.9.4"})

	tag, err := CachedLatestTag(context.Background())
	if err != nil || tag != "v0.10.0" || calls.Load() != 1 {
		t.Fatalf("CachedLatestTag = %q, %v, %d Anfragen", tag, err, calls.Load())
	}
	if entry := getEntry(t); entry.Source != replacedSource() {
		t.Errorf("Eintrag: %+v", entry)
	}
}

// Bei abgeschalteter Quelle wird der Cache weder gelesen noch geschrieben.
func TestAbgeschalteteQuelleLaesstDenCacheUnberuehrt(t *testing.T) {
	latest(t, http.StatusFound, "/releases/tag/v0.10.0")
	putEntry(t, cacheEntry{Source: replacedSource(), Checked: time.Now(), Tag: "v0.9.4"})
	before, err := os.ReadFile(cacheFile(t))
	if err != nil {
		t.Fatal(err)
	}
	path := cacheFile(t)
	t.Setenv(EnvSource, SourceOff)

	if _, err := CachedLatestTag(context.Background()); !errors.Is(err, ErrDisabled) {
		t.Errorf("CachedLatestTag: %v", err)
	}
	if _, err := LatestTag(context.Background()); !errors.Is(err, ErrDisabled) {
		t.Errorf("LatestTag: %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Errorf("der Cache hat sich verändert: %s → %s (%v)", before, after, err)
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Errorf("im Cache-Verzeichnis liegt mehr: %v", entries)
	}
}

// Ein Eintrag mit Zeitpunkt in der Zukunft (verstellte Uhr) gilt nicht.
func TestCachedLatestTagZukunft(t *testing.T) {
	calls := latest(t, http.StatusFound, "/releases/tag/v0.10.0")
	putEntry(t, cacheEntry{Source: replacedSource(), Checked: time.Now().Add(time.Hour), Tag: "v0.9.4"})
	if tag, _ := CachedLatestTag(context.Background()); tag != "v0.10.0" || calls.Load() != 1 {
		t.Errorf("Tag %q, %d Anfragen", tag, calls.Load())
	}
}

// Der Ort: $XDG_CACHE_HOME/k-playbook, ein relatives XDG_CACHE_HOME zählt
// nicht, sonst $HOME/.cache/k-playbook — auf Linux und macOS gleich.
func TestCacheDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", "/tmp/xdg")
	if got := CacheDir(); got != "/tmp/xdg/k-playbook" {
		t.Errorf("mit XDG_CACHE_HOME: %q", got)
	}
	t.Setenv("XDG_CACHE_HOME", "relativ")
	if got := CacheDir(); got != filepath.Join(home, ".cache", "k-playbook") {
		t.Errorf("relatives XDG_CACHE_HOME: %q", got)
	}
	t.Setenv("XDG_CACHE_HOME", "")
	if got := CacheDir(); got != filepath.Join(home, ".cache", "k-playbook") {
		t.Errorf("ohne XDG_CACHE_HOME: %q", got)
	}
	t.Setenv("HOME", "")
	if got := CacheDir(); got != "" {
		t.Errorf("ohne HOME: %q", got)
	}
}
