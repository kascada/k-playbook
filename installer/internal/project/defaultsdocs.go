package project

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"sync"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
)

// Die mitgelieferte Ebene hinter k-playbook:-Verweisen liegt in der
// öffentlichen Collection <hub>:k-playbook unter defaults/ (Task 131). Der
// Release packt defaults/ und VERSION nach k-playbook-defaults.tar.gz, der Hub
// holt das Archiv dorthin (fetch_release), der Node gleicht es auf den
// Rechner ab. Gelesen wird aus der Replica des Nodes; geschrieben wird dort
// nie. Ohne Kephalaion gibt es keine mitgelieferten Texte: Jeder Zugriff ist
// dann kephalaion_unavailable, kein Rückfall auf die Platte.
//
// Die Command-Hüllen kommen aus dem Binary, die Texte von hier. Der Hub holt
// einen neuen Release nur beim Anlegen, von Hand oder auf Anfrage; ein neueres
// Binary kann also auf ältere Texte treffen. Das ist bewusst hingenommen,
// ohne Versionsprüfung (installer/docs/architecture.md).

// DefaultsCollectionName ist die öffentliche Collection mit dem Lieferumfang,
// auf demselben Hub wie die Collection des Projekts.
const DefaultsCollectionName = "k-playbook"

// defaultsDirName ist das Verzeichnis in DefaultsCollectionName, das den
// Lieferumfang trägt: derselbe Aufbau wie defaults/ im Quell-Repo.
const defaultsDirName = "defaults"

// defaultsVersionName ist die Datei unter defaults/, an der sich entscheidet,
// ob der Lieferumfang angekommen ist: Der Release legt sie immer mit.
const defaultsVersionName = "VERSION"

// DefaultsStore sind die Aufrufe an Kephalaion, die die mitgelieferte Ebene
// braucht — nur lesende. *kephalaion.Client erfüllt sie; Tests setzen
// kephtest.Store ein.
type DefaultsStore interface {
	List(ctx context.Context, req kephalaion.ListRequest) ([]kephalaion.Entry, error)
	Read(ctx context.Context, name string) (kephalaion.ReadResult, error)
}

// OpenDefaultsStore öffnet den Zugang zu <hub>:k-playbook: derselbe Weg wie
// openKephClient (Status, Anbindung, Umgebung), mit dem Hub und dem Account
// der Projekt-Anbindung und der Collection DefaultsCollectionName. Zurück
// kommt neben dem Store die Adresse der Collection für Meldungen. Ein Fehler
// trägt DocCodeKephalaionUnavailable. Der Store ist ein
// kephalaion.ReaderPool mit defaultsReaders Sitzungen am Node: Das Briefing
// liest einige Dutzend Dokumente, je eines dauert am Node einige
// Millisekunden, und erst nebeneinander gelesen bleibt die Laufzeit des
// Briefings im Rahmen (gemessen in Task 131). Austauschbar, damit Tests den
// Node nicht fragen.
var OpenDefaultsStore = func(projectDir string) (DefaultsStore, string, error) {
	client, _, err := openKephClientFor(projectDir, DefaultsCollectionName)
	if err != nil {
		return nil, defaultsAddress(projectDir), err
	}
	return client.ReaderPool(defaultsReaders), client.Address(), nil
}

// defaultsReaders ist die Zahl der Sitzungen, über die die mitgelieferte
// Ebene gleichzeitig liest — beim Vorauslesen eines Verzeichnisses.
const defaultsReaders = 8

// defaultsAddress nennt die Adresse von DefaultsCollectionName für eine
// Meldung, auch wenn der Zugang nicht zustande kommt: mit dem Hub aus der
// Anbindung, sonst mit dem Platzhalter <hub>.
func defaultsAddress(projectDir string) string {
	hub := "<hub>"
	if binding, err := ProjectBinding(projectDir); err == nil && kephalaion.CheckName("Hub", binding.Hub) == nil {
		hub = binding.Hub
	}
	return hub + ":" + DefaultsCollectionName
}

// NewProjectDefaultsSource ist die mitgelieferte Ebene eines Projekts: die
// Collection DefaultsCollectionName unter defaults/, geöffnet über
// OpenDefaultsStore erst beim ersten Zugriff.
func NewProjectDefaultsSource(projectDir string) DocSource {
	return NewKephDefaultsSource(func() (DefaultsStore, string, error) { return OpenDefaultsStore(projectDir) })
}

// NewKephDefaultsSource ist eine DocSource über defaults/ einer Collection.
// open liefert den Store und die Adresse der Collection; aufgerufen wird es
// höchstens einmal je Quelle, auch das Ergebnis „nicht bereit“ bleibt stehen.
//
// Beim ersten Zugriff listet die Quelle defaults/ einmal rekursiv und kennt
// danach jeden Namen: List antwortet ohne weiteren Aufruf, Read fragt den
// Node nur für Dateien, die es gibt, und jede Datei höchstens einmal. List
// liest dabei die Dateien des Verzeichnisses nebeneinander voraus (bis zu
// defaultsReaders zugleich): Wer ein Verzeichnis listet — Briefing, Kataloge,
// zusammengebaute Verzeichnisse —, liest danach dessen Dateien. Eine
// Collection kennt nur implizite Verzeichnisse — ein Verzeichnis gibt es,
// wenn ein Dokument darunter liegt; eine leere Liste heißt fs.ErrNotExist.
// Fehlt defaults/VERSION, ist der Lieferumfang nicht angekommen: dann ist
// jeder Zugriff kephalaion_unavailable mit der Abhilfe fetch_release.
func NewKephDefaultsSource(open func() (DefaultsStore, string, error)) DocSource {
	return &kephCollectionSource{open: open, ctx: context.Background(), root: defaultsDirName}
}

// newProjectDefaultsSourceContext ist NewProjectDefaultsSource, dessen
// Aufrufe am Node ctx begrenzt — für einen Schritt mit Zeitlimit wie das
// Schreiben der READMEs beim Einrichten. DocSource selbst kennt keinen
// Kontext.
func newProjectDefaultsSourceContext(ctx context.Context, projectDir string) DocSource {
	return &kephCollectionSource{open: func() (DefaultsStore, string, error) { return OpenDefaultsStore(projectDir) },
		ctx: ctx, root: defaultsDirName}
}

// kephCollectionSource ist eine DocSource über ein Verzeichnis root einer
// Collection: defaults/ in <hub>:k-playbook für das Mitgelieferte
// (NewKephDefaultsSource), k-playbook/ in der Collection des Projekts für die
// projekteigene Ebene (NewKephProjectLayerSource, projectlayer.go). Beide
// verhalten sich gleich — einmal rekursiv listen, beim Listen vorauslesen,
// nur Vorhandenes lesen —, bis auf die Frage, was ein leeres root heißt: Beim
// Mitgelieferten ist der Lieferumfang nicht angekommen (VERSION fehlt), bei
// der projekteigenen Ebene ist sie einfach leer.
type kephCollectionSource struct {
	open func() (DefaultsStore, string, error)
	// ctx begrenzt List und Read am Node; Hintergrund, wenn niemand eine
	// Frist setzt.
	ctx context.Context
	// root ist das Verzeichnis in der Collection, das die Ebene trägt.
	root string
	// project meldet die projekteigene Ebene: keine Prüfung auf VERSION,
	// keine Abhilfe fetch_release, ein leeres root ist eine leere Ebene.
	project bool

	once    sync.Once
	err     error
	store   DefaultsStore
	address string
	// files sind die Dateien unter root, relativ dazu.
	files map[string]bool
	// dirs sind die Verzeichnisse unter root, relativ dazu; "" ist root
	// selbst.
	dirs map[string]bool

	mu       sync.Mutex
	contents map[string][]byte
}

// load öffnet den Zugang und listet root einmal.
func (s *kephCollectionSource) load() error {
	s.once.Do(func() { s.err = s.index() })
	return s.err
}

func (s *kephCollectionSource) index() error {
	if s.project {
		return s.indexProject()
	}
	store, address, err := s.open()
	if address == "" {
		address = "<hub>:" + DefaultsCollectionName
	}
	s.address = address
	if err != nil {
		return docRefErrorf(DocCodeKephalaionUnavailable,
			"die mitgelieferten Texte hinter k-playbook: liegen in %s/%s/ und sind ohne Kephalaion nicht erreichbar: %v — "+
				"Abhilfe: Kephalaion einrichten über /k-gui (k-playbook gui, Abschnitt Kephalaion)",
			address, defaultsDirName, err)
	}
	entries, err := store.List(s.ctx, kephalaion.ListRequest{Path: defaultsDirName, Recursive: true})
	if err != nil {
		return docRefErrorf(DocCodeKephalaionUnavailable,
			"die mitgelieferten Texte sind nicht lesbar: %s lässt sich nicht auflisten (%v) — gibt es die Collection "+
				"%s auf dem Hub nicht oder darf der Account sie nicht lesen, beim Hub-Betreiber anlegen lassen und "+
				"fetch_release für %s anstoßen",
			address, err, DefaultsCollectionName, address)
	}
	s.remember(store, entries)
	if !s.files[defaultsVersionName] {
		return docRefErrorf(DocCodeKephalaionUnavailable,
			"die mitgelieferten Texte fehlen: %s/%s/ ist leer (%s/%s fehlt) — Abhilfe: beim Hub-Betreiber "+
				"fetch_release für %s anstoßen",
			address, defaultsDirName, defaultsDirName, defaultsVersionName, address)
	}
	return nil
}

// remember hält den Store und die Namen aus dem Listing von root fest.
func (s *kephCollectionSource) remember(store DefaultsStore, entries []kephalaion.Entry) {
	s.store = store
	s.files = map[string]bool{}
	s.dirs = map[string]bool{"": true}
	s.contents = map[string][]byte{}
	for _, entry := range entries {
		rel, ok := strings.CutPrefix(entry.Name, s.root+"/")
		if !ok || rel == "" {
			continue
		}
		s.files[rel] = true
		for dir := path.Dir(rel); dir != "."; dir = path.Dir(dir) {
			s.dirs[dir] = true
		}
	}
}

func (s *kephCollectionSource) List(dir string) ([]DocSourceEntry, error) {
	if err := s.load(); err != nil {
		return nil, err
	}
	if !s.dirs[dir] {
		return nil, &fs.PathError{Op: "readdir", Path: dir, Err: fs.ErrNotExist}
	}
	prefix := ""
	if dir != "" {
		prefix = dir + "/"
	}
	seen := map[string]bool{}
	listed := []DocSourceEntry{}
	add := func(name string, isDir bool) {
		rest, ok := strings.CutPrefix(name, prefix)
		if !ok || rest == "" || strings.Contains(rest, "/") || seen[rest] {
			return
		}
		seen[rest] = true
		listed = append(listed, DocSourceEntry{Name: rest, Dir: isDir})
	}
	for name := range s.dirs {
		add(name, true)
	}
	for name := range s.files {
		add(name, false)
	}
	sort.Slice(listed, func(i, j int) bool { return listed[i].Name < listed[j].Name })
	s.prefetch(prefix, listed)
	return listed, nil
}

// prefetch liest die Dateien eines gelisteten Verzeichnisses nebeneinander
// in den Zwischenspeicher — außer README.md und Dotfiles, die keiner der
// Leser eines ganzen Verzeichnisses nimmt (die Vorlagen README.md unter docs/
// liest docsreadme.go einzeln). Ein Fehler bleibt hier stumm: Das spätere
// Read fragt noch einmal und meldet ihn.
func (s *kephCollectionSource) prefetch(prefix string, entries []DocSourceEntry) {
	files := []string{}
	s.mu.Lock()
	for _, entry := range entries {
		if entry.Dir || entry.Name == "README.md" || strings.HasPrefix(entry.Name, ".") {
			continue
		}
		if _, ok := s.contents[prefix+entry.Name]; !ok {
			files = append(files, prefix+entry.Name)
		}
	}
	s.mu.Unlock()
	if len(files) < 2 {
		return
	}
	sem := make(chan struct{}, defaultsReaders)
	var wg sync.WaitGroup
	for _, file := range files {
		wg.Add(1)
		sem <- struct{}{}
		go func(file string) {
			defer wg.Done()
			defer func() { <-sem }()
			_, _ = s.Read(file)
		}(file)
	}
	wg.Wait()
}

func (s *kephCollectionSource) Read(file string) ([]byte, error) {
	if err := s.load(); err != nil {
		return nil, err
	}
	if s.dirs[file] {
		return nil, ErrDocIsDir
	}
	if !s.files[file] {
		return nil, &fs.PathError{Op: "open", Path: file, Err: fs.ErrNotExist}
	}
	s.mu.Lock()
	data, ok := s.contents[file]
	s.mu.Unlock()
	if ok {
		return data, nil
	}
	read, err := s.store.Read(s.ctx, s.root+"/"+file)
	if err != nil {
		mapped := kephError(err)
		if DocRefErrorCode(mapped) != DocCodeKephalaionUnavailable {
			mapped = &DocRefError{Code: DocCodeKephalaionUnavailable, err: mapped}
		}
		return nil, fmt.Errorf("%s/%s/%s lesen: %w", s.address, s.root, file, mapped)
	}
	switch read.Kind {
	case kephalaion.ReadKindNone:
		// Seit dem Listing verschwunden: wie bei der Platte nicht vorhanden.
		return nil, &fs.PathError{Op: "open", Path: file, Err: fs.ErrNotExist}
	case kephalaion.ReadKindDirectory:
		return nil, ErrDocIsDir
	}
	data = []byte{}
	if read.Content != nil {
		data = []byte(*read.Content)
	}
	s.mu.Lock()
	s.contents[file] = data
	s.mu.Unlock()
	return data, nil
}

// errDefaultsUnavailable erkennt einen Fehler der mitgelieferten Ebene, der
// heißt: Kephalaion ist nicht nutzbar.
func errDefaultsUnavailable(err error) bool {
	return err != nil && !errors.Is(err, fs.ErrNotExist) && DocRefErrorCode(err) == DocCodeKephalaionUnavailable
}
