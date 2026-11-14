package release

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Der Cache des neuesten Release (Task 151): Die reinen Hinweise —
// Umgebungsprüfung (Briefing, `k-playbook health`, k_playbook_health), der
// Hinweis beim Start der Oberfläche, GET /api/update — fragen GitHub nur, wenn
// der letzte Eintrag nicht mehr frisch ist (CachedLatestTag). Was installiert
// oder ausdrücklich prüft — `k-playbook upgrade` samt `--check`,
// `version --all`, POST /api/update und /api/update/program — fragt immer
// sofort (LatestTag); jedes Ergebnis landet dabei im Cache.
//
// Der Eintrag liegt unter $XDG_CACHE_HOME/k-playbook/, sonst unter
// $HOME/.cache/k-playbook/ — auf Linux und macOS gleich, deshalb nicht
// os.UserCacheDir (das nennt auf macOS ~/Library/Caches). Der neueste Tag
// hängt am Rechner, nicht am Projekt; .k-playbook/ im Projekt wird
// eingecheckt.
//
// Der Cache ist nur eine Abkürzung: eine fehlende, kaputte oder fremde Datei
// gilt als „kein Eintrag“, und ein Fehler beim Schreiben wird nicht gemeldet
// und hält nichts auf.

// Gültigkeit eines Eintrags.
const (
	// CacheSuccessTTL: Ein gelesener Tag gilt sechs Stunden. Ein Release
	// erscheint selten öfter als ein paarmal am Tag; die Umgebungsprüfung darf
	// so lange hinterherhinken, `upgrade` fragt ohnehin frisch.
	CacheSuccessTTL = 6 * time.Hour
	// CacheFailureTTL: Ein Fehlschlag (kein Netz, Frist abgelaufen) gilt eine
	// halbe Stunde. Sonst kostete ein hängendes Netz jedes Briefing die volle
	// Frist (TagTimeout); kürzer als ein Erfolg, damit ein zurückgekehrtes
	// Netz bald wieder zählt.
	CacheFailureTTL = 30 * time.Minute
)

// CacheFileName ist der Eintrag im Cache-Verzeichnis.
const CacheFileName = "latest-release.json"

// sourceGitHub ist die Quelle eines Eintrags, der von GitHub stammt.
const sourceGitHub = "GitHub"

// cacheEntry ist der Inhalt von CacheFileName: Tag oder Fehlergrund, die
// Quelle und der Zeitpunkt der Prüfung.
type cacheEntry struct {
	// Source ist "GitHub" oder die Basis-URL der Ersatzquelle (EnvSource).
	// Ein Eintrag einer anderen Quelle als der aktuellen gilt als „kein
	// Eintrag“: So bedient eine Ersatzquelle den Cache der echten nie.
	Source  string    `json:"source"`
	Checked time.Time `json:"checked"`
	Tag     string    `json:"tag,omitempty"`
	Error   string    `json:"error,omitempty"`
}

// CacheDir ist das Verzeichnis des Caches: $XDG_CACHE_HOME/k-playbook, sonst
// $HOME/.cache/k-playbook. Ein relatives XDG_CACHE_HOME zählt nach der
// XDG-Spezifikation nicht. Leer, wenn keines bestimmbar ist — dann gibt es
// keinen Cache.
func CacheDir() string {
	if base := os.Getenv("XDG_CACHE_HOME"); base != "" && filepath.IsAbs(base) {
		return filepath.Join(base, "k-playbook")
	}
	if home := os.Getenv("HOME"); home != "" {
		return filepath.Join(home, ".cache", "k-playbook")
	}
	return ""
}

// CachedLatestTag nennt den neuesten Tag aus dem Cache, wenn der Eintrag
// frisch ist und von der aktuellen Quelle stammt — ohne Netzaufruf; ein
// frischer Fehlschlag gibt seinen Grund als TagError zurück. Sonst fragt sie
// wie LatestTag und schreibt das Ergebnis in den Cache. Bei abgeschalteter
// Quelle ErrDisabled, ohne den Cache zu lesen.
//
// Für die reinen Hinweise; was installiert oder ausdrücklich prüft, nimmt
// LatestTag.
func CachedLatestTag(ctx context.Context) (string, error) {
	source := CurrentSource()
	if source.Off {
		return "", ErrDisabled
	}
	if entry, ok := readCache(source, time.Now()); ok {
		if entry.Tag != "" {
			return entry.Tag, nil
		}
		return "", &TagError{Reason: entry.Error}
	}
	return askAndStore(ctx, source)
}

// askAndStore fragt sofort und schreibt das Ergebnis in den Cache. Bricht der
// Aufrufer selbst ab (sein Kontext endet), sagt das nichts über die Quelle:
// dann bleibt der Cache, wie er war.
func askAndStore(ctx context.Context, source Source) (string, error) {
	tag, err := askLatest(ctx, source)
	if ctx.Err() == nil {
		entry := cacheEntry{Source: sourceID(source), Checked: time.Now().UTC(), Tag: tag}
		if err != nil {
			entry.Tag, entry.Error = "", err.Error()
		}
		writeCache(entry)
	}
	return tag, err
}

// sourceID ist die Quelle, wie der Eintrag sie trägt.
func sourceID(source Source) string {
	if source.Replaced {
		return source.Base
	}
	return sourceGitHub
}

// readCache liest den Eintrag und sagt, ob er gilt: lesbar, von dieser
// Quelle, mit Tag der Form vX.Y.Z oder einem Grund, und frisch. Ein Zeitpunkt
// in der Zukunft (verstellte Uhr) gilt nicht.
func readCache(source Source, now time.Time) (cacheEntry, bool) {
	dir := CacheDir()
	if dir == "" {
		return cacheEntry{}, false
	}
	file, err := os.Open(filepath.Join(dir, CacheFileName))
	if err != nil {
		return cacheEntry{}, false
	}
	defer file.Close()
	var entry cacheEntry
	// Ein Eintrag ist ein paar hundert Bytes; mehr ist keiner.
	if err := json.NewDecoder(io.LimitReader(file, 64<<10)).Decode(&entry); err != nil {
		return cacheEntry{}, false
	}
	if entry.Source != sourceID(source) || entry.Checked.IsZero() {
		return cacheEntry{}, false
	}
	ttl := CacheSuccessTTL
	switch {
	case entry.Tag != "" && entry.Error == "" && IsReleaseTag(entry.Tag):
	case entry.Tag == "" && entry.Error != "":
		ttl = CacheFailureTTL
	default:
		return cacheEntry{}, false
	}
	age := now.Sub(entry.Checked)
	if age < 0 || age >= ttl {
		return cacheEntry{}, false
	}
	return entry, true
}

// writeCache schreibt den Eintrag atomar: in eine Datei daneben, dann
// umbenannt — ein Leser sieht nie einen halben. Ein Fehler wird nicht
// gemeldet: der Cache ist nur eine Abkürzung.
func writeCache(entry cacheEntry) {
	dir := CacheDir()
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	temporary, err := os.CreateTemp(dir, "."+CacheFileName+".*")
	if err != nil {
		return
	}
	defer os.Remove(temporary.Name())
	if err := json.NewEncoder(temporary).Encode(entry); err != nil {
		temporary.Close()
		return
	}
	if err := temporary.Close(); err != nil {
		return
	}
	os.Chmod(temporary.Name(), 0o644)
	os.Rename(temporary.Name(), filepath.Join(dir, CacheFileName))
}
