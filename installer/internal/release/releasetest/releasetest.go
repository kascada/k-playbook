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
// Programm —, für die eine Paketvariable nicht gälte.
package releasetest

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/release"
)

// Isolate schaltet die Release-Quelle für diesen Prozess und seine
// Unterprozesse ab und gibt die Aufräumfunktion zurück. Gedacht für TestMain:
// Danach endet jede Frage nach dem neuesten Tag mit release.ErrDisabled, ohne
// git und ohne Netz.
func Isolate() (cleanup func()) {
	before, had := os.LookupEnv(release.EnvSource)
	os.Setenv(release.EnvSource, release.SourceOff)
	return func() {
		if had {
			os.Setenv(release.EnvSource, before)
			return
		}
		os.Unsetenv(release.EnvSource)
	}
}

// Off schaltet die Quelle für einen Test ab — für Tests, die prüfen, dass
// ohne Quelle nichts geschieht, in einem Paket, das sonst eine Ersatzquelle
// trägt.
func Off(t *testing.T) {
	t.Helper()
	t.Setenv(release.EnvSource, release.SourceOff)
}

// Source ist eine Ersatzquelle: ein lokaler HTTP-Server, der die Tags so
// ausliefert, wie git sie über das einfache HTTP-Protokoll liest
// (<basis>/k-playbook.git/info/refs), dazu SHA256SUMS je Tag, die Assets und —
// wenn gesetzt — ein Ersatz für das install.sh von Kephalaion.
// Gefragt wird sie mit dem echten git und dem echten HTTP-Client — ersetzt
// sind nur die Adressen.
type Source struct {
	server *httptest.Server

	mu     sync.Mutex
	tags   []string
	sums   map[string]string            // Tag -> Inhalt von SHA256SUMS
	assets map[string]map[string][]byte // Tag -> Asset -> Inhalt
	// kephalaion ist der Ersatz für das install.sh von Kephalaion; nil heißt:
	// die Quelle hat keines (404).
	kephalaion *string
	requests   []string
}

// Serve startet eine Ersatzquelle und setzt release.EnvSource für diesen Test
// darauf. Sie trägt zunächst keinen Tag.
func Serve(t *testing.T) *Source {
	t.Helper()
	source := &Source{sums: map[string]string{}, assets: map[string]map[string][]byte{}}
	source.server = httptest.NewServer(http.HandlerFunc(source.handle))
	t.Cleanup(source.server.Close)
	t.Setenv(release.EnvSource, source.server.URL)
	// git liest die Proxy-Variablen der Umgebung auch für 127.0.0.1; der
	// HTTP-Client von Go tut das nicht. Ohne die Ausnahme fragte git auf einem
	// Rechner mit Proxy diesen statt der Ersatzquelle.
	t.Setenv("NO_PROXY", "127.0.0.1,localhost")
	t.Setenv("no_proxy", "127.0.0.1,localhost")
	return source
}

// URL ist die Basis-URL der Ersatzquelle, der Wert von release.EnvSource.
func (s *Source) URL() string { return s.server.URL }

// AddTag nennt einen Tag, ohne Prüfsummen und ohne Assets — das Fenster, in
// dem der Tag steht und die CI noch baut.
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
	switch {
	case path == "k-playbook.git/info/refs":
		// Das einfache HTTP-Protokoll: je Zeile „<hash>\t<ref>“. Ohne den
		// Content-Type des smarten Protokolls liest git die Antwort so.
		tags := append([]string(nil), s.tags...)
		sort.Strings(tags)
		w.Header().Set("Content-Type", "text/plain")
		for i, tag := range tags {
			fmt.Fprintf(w, "%040x\trefs/tags/%s\n", i+1, tag)
		}
		return
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
