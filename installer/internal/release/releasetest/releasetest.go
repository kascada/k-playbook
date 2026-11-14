// Package releasetest hält Tests vom echten GitHub fern: Kein Test fragt den
// neuesten Release-Tag von k-playbook ab oder lädt ein Release-Binary.
//
// Sobald ein Test eine eigene Version setzt (buildinfo.Version) oder ein
// Programm mit gestempelter Version baut und startet, erreichen
// Umgebungsprüfung, Programm-Stand und Installation die Release-Quelle. Jedes
// Testpaket, das dorthin kommt, schaltet sie in TestMain mit Isolate ab, wie
// sharetest.Isolate und accesstest.Isolate es für ihre Orte tun. Wer die
// Quelle braucht, stellt mit Serve eine Ersatzquelle hin.
//
// Beides wirkt über die Umgebungsvariable release.EnvSource und erreicht damit
// auch gebaute Unterprozesse — einen gestarteten Dienst, ein neu installiertes
// Programm —, für die eine Paketvariable nicht gälte. Ebenso setzen beide
// XDG_CACHE_HOME auf ein temporäres Verzeichnis: Kein Test liest oder
// schreibt den Cache des neuesten Release (release.CacheDir) im echten
// ~/.cache, auch nicht über eine Ersatzquelle.
package releasetest

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/release"
	"github.com/kascada/k-playbook/installer/internal/version"
)

// Isolate schaltet die Release-Quelle für diesen Prozess und seine
// Unterprozesse ab und gibt die Aufräumfunktion zurück. Gedacht für TestMain:
// Danach endet jede Frage nach dem neuesten Tag mit release.ErrDisabled, ohne
// Netz.
//
// Dazu zeigt XDG_CACHE_HOME auf ein eigenes temporäres Verzeichnis, das die
// Aufräumfunktion wieder löscht.
func Isolate() (cleanup func()) {
	restoreSource := setenv(release.EnvSource, release.SourceOff)
	cache, err := os.MkdirTemp("", "k-playbook-cache-*")
	if err != nil {
		panic("releasetest: temporäres Cache-Verzeichnis nicht anlegbar: " + err.Error())
	}
	restoreCache := setenv("XDG_CACHE_HOME", cache)
	return func() {
		restoreCache()
		os.RemoveAll(cache)
		restoreSource()
	}
}

// setenv setzt eine Variable und gibt zurück, was den alten Zustand
// wiederherstellt.
func setenv(name string, value string) (restore func()) {
	before, had := os.LookupEnv(name)
	os.Setenv(name, value)
	return func() {
		if had {
			os.Setenv(name, before)
			return
		}
		os.Unsetenv(name)
	}
}

// Cache setzt XDG_CACHE_HOME für einen Test auf ein temporäres Verzeichnis und
// nennt das Verzeichnis des Caches darunter (release.CacheDir). Serve ruft es
// selbst; ein Test ruft es, wenn er den Cache ohne Ersatzquelle braucht.
func Cache(t *testing.T) string {
	t.Helper()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	return release.CacheDir()
}

// Off schaltet die Quelle für einen Test ab — für Tests, die prüfen, dass
// ohne Quelle nichts geschieht, in einem Paket, das sonst eine Ersatzquelle
// trägt.
func Off(t *testing.T) {
	t.Helper()
	t.Setenv(release.EnvSource, release.SourceOff)
}

// Source ist eine Ersatzquelle: ein lokaler HTTP-Server, der das neueste
// Release so nennt wie GitHub — <basis>/releases/latest leitet auf
// <basis>/releases/tag/<tag> weiter —, dazu SHA256SUMS je Tag, die Assets und
// — wenn gesetzt — ein Ersatz für das install.sh von Kephalaion.
// Gefragt wird sie mit dem echten HTTP-Client und von bin/install mit curl
// oder wget — ersetzt sind nur die Adressen.
//
// Jeder genannte Tag gilt als veröffentlichtes Release. Das neueste ist der
// höchste der Form vX.Y.Z, wie GitHub Vorabversionen übergeht; SetLatest
// nennt ein anderes. Ohne einen solchen Tag antwortet /releases/latest mit
// 404.
type Source struct {
	server *httptest.Server

	mu   sync.Mutex
	tags []string
	// latest ersetzt das neueste Release, wenn gesetzt.
	latest string
	sums   map[string]string            // Tag -> Inhalt von SHA256SUMS
	assets map[string]map[string][]byte // Tag -> Asset -> Inhalt
	// kephalaion ist der Ersatz für das install.sh von Kephalaion; nil heißt:
	// die Quelle hat keines (404).
	kephalaion *string
	// stalled: Anfragen nach Assets werden angenommen und nie beantwortet.
	stalled  bool
	requests []string
}

// Serve startet eine Ersatzquelle und setzt release.EnvSource für diesen Test
// darauf. Sie trägt zunächst keinen Tag.
func Serve(t *testing.T) *Source {
	t.Helper()
	source := &Source{sums: map[string]string{}, assets: map[string]map[string][]byte{}}
	source.server = httptest.NewServer(http.HandlerFunc(source.handle))
	t.Cleanup(source.server.Close)
	t.Setenv(release.EnvSource, source.server.URL)
	Cache(t)
	// curl und wget (bin/install) lesen die Proxy-Variablen der Umgebung auch
	// für 127.0.0.1; der HTTP-Client von Go tut das nicht. Ohne die Ausnahme
	// fragte bin/install auf einem Rechner mit Proxy diesen statt der
	// Ersatzquelle.
	t.Setenv("NO_PROXY", "127.0.0.1,localhost")
	t.Setenv("no_proxy", "127.0.0.1,localhost")
	return source
}

// URL ist die Basis-URL der Ersatzquelle, der Wert von release.EnvSource.
func (s *Source) URL() string { return s.server.URL }

// AddTag nennt ein veröffentlichtes Release, ohne Prüfsummen und ohne Assets —
// ein Release, das (noch) nicht fertig bestückt ist.
func (s *Source) AddTag(tag string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, known := range s.tags {
		if known == tag {
			return
		}
	}
	s.tags = append(s.tags, tag)
}

// SetLatest nennt tag als neuestes Release, wie GitHub es über „Latest“
// markiert — auch einen Tag mit Zusatz. Leer gilt wieder der höchste Tag der
// Form vX.Y.Z.
func (s *Source) SetLatest(tag string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.latest = tag
}

// Latest ist das Release, auf das /releases/latest gerade weiterleitet, leer
// ohne eines.
func (s *Source) Latest() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.latestLocked()
}

func (s *Source) latestLocked() string {
	if s.latest != "" {
		return s.latest
	}
	latest := ""
	for _, tag := range s.tags {
		if release.IsReleaseTag(tag) && (latest == "" || version.Compare(tag, latest) == version.Newer) {
			latest = tag
		}
	}
	return latest
}

// AddRelease nennt einen Tag samt dem Asset dieser Plattform und der passenden
// SHA256SUMS.
func (s *Source) AddRelease(tag string, content []byte) {
	s.AddAsset(tag, release.AssetName(), content)
	sum := sha256.Sum256(content)
	s.SetSums(tag, hex.EncodeToString(sum[:])+"  "+release.AssetName()+"\n")
}

// AddAsset legt ein Asset unter einem Tag ab und nennt den Tag.
func (s *Source) AddAsset(tag string, asset string, content []byte) {
	s.AddTag(tag)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.assets[tag] == nil {
		s.assets[tag] = map[string][]byte{}
	}
	s.assets[tag][asset] = content
}

// RemoveAsset entfernt ein Asset; der Tag und seine Prüfsummen bleiben.
func (s *Source) RemoveAsset(tag string, asset string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.assets[tag], asset)
}

// SetSums setzt den Inhalt der SHA256SUMS eines Tags und nennt den Tag.
func (s *Source) SetSums(tag string, content string) {
	s.AddTag(tag)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sums[tag] = content
}

// SetKephalaionInstall legt ein Ersatz-install.sh für Kephalaion ab
// (<basis>/kephalaion/install.sh). Ohne den Aufruf antwortet die Quelle dort
// mit 404 — der gescheiterte Download. Kein Test lädt oder startet das echte
// install.sh von Kephalaion.
func (s *Source) SetKephalaionInstall(script string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.kephalaion = &script
}

// StallDownloads lässt jede folgende Anfrage nach einem Asset hängen: Die
// Quelle nimmt sie an und antwortet nie — bis der Aufrufer aufgibt oder der
// Test endet. Tag, Prüfsummen und install.sh antworten weiter. So lässt sich
// der Fristablauf eines Downloads prüfen.
func (s *Source) StallDownloads() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stalled = true
}

// Requests sind die Pfade aller Anfragen bisher, in ihrer Reihenfolge.
func (s *Source) Requests() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.requests...)
}

func (s *Source) handle(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, r.URL.Path)

	path := strings.TrimPrefix(r.URL.Path, "/")
	if s.stalled && strings.HasPrefix(path, "download/") {
		// Ohne die Sperre warten: Andere Anfragen laufen weiter.
		s.mu.Unlock()
		<-r.Context().Done()
		s.mu.Lock()
		return
	}
	switch {
	case path == "releases/latest":
		// Wie GitHub: eine Weiterleitung mit absolutem Ziel.
		if latest := s.latestLocked(); latest != "" {
			w.Header().Set("Location", s.server.URL+"/releases/tag/"+latest)
			w.WriteHeader(http.StatusFound)
			return
		}
	case path == "kephalaion/install.sh":
		if s.kephalaion != nil {
			fmt.Fprint(w, *s.kephalaion)
			return
		}
	case strings.HasPrefix(path, "raw/"):
		tag, file, ok := strings.Cut(strings.TrimPrefix(path, "raw/"), "/")
		if content, known := s.sums[tag]; ok && known && file == release.SumsFileName {
			fmt.Fprint(w, content)
			return
		}
	case strings.HasPrefix(path, "download/"):
		tag, asset, ok := strings.Cut(strings.TrimPrefix(path, "download/"), "/")
		if content, known := s.assets[tag][asset]; ok && known {
			w.Write(content)
			return
		}
	}
	http.NotFound(w, r)
}
