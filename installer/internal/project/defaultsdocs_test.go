package project

import (
	"errors"
	"io/fs"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
	"github.com/kascada/k-playbook/installer/internal/kephalaion/kephtest"
)

// defaultsStore baut die öffentliche Collection im Speicher: VERSION und die
// übergebenen Dateien unter defaults/.
func defaultsStore(files map[string]string) *kephtest.Store {
	store := kephtest.New()
	store.Put("defaults/VERSION", "v0.0.0\n")
	for rel, content := range files {
		store.Put("defaults/"+rel, content)
	}
	return store
}

// openStore ist ein open für NewKephDefaultsSource, das mitzählt.
func openStore(store DefaultsStore, calls *int) func() (DefaultsStore, string, error) {
	return func() (DefaultsStore, string, error) {
		*calls++
		return store, "vm:k-playbook", nil
	}
}

func TestDefaultsSourceListetUndLiest(t *testing.T) {
	store := defaultsStore(map[string]string{
		"guidelines/a.md":                  "# A\n",
		"guidelines/README.md":             "# Guidelines\n",
		"instructions/always/010-x.md":     "## X\n",
		"commands/_skills/s/vorlage.md":    "V\n",
		"commands/k-task-create.md":        "# k-task-create\n",
		"instructions/on-demand/thema.md":  "## Thema\n",
		"testdata/includes/teile/stück.md": "T\n",
	})
	calls := 0
	source := NewKephDefaultsSource(openStore(store, &calls))

	root, err := source.List("")
	if err != nil {
		t.Fatalf("List(\"\"): %v", err)
	}
	if got := entryNames(root); got != "VERSION commands/ guidelines/ instructions/ testdata/" {
		t.Errorf("Wurzel: %s", got)
	}
	commands, err := source.List("commands")
	if err != nil {
		t.Fatalf("List(commands): %v", err)
	}
	if got := entryNames(commands); got != "_skills/ k-task-create.md" {
		t.Errorf("commands: %s", got)
	}
	data, err := source.Read("commands/k-task-create.md")
	if err != nil || string(data) != "# k-task-create\n" {
		t.Errorf("Read: %q, %v", data, err)
	}
	// Ein zweites Lesen fragt den Node nicht noch einmal.
	if _, err := source.Read("commands/k-task-create.md"); err != nil {
		t.Fatalf("zweites Read: %v", err)
	}
	reads := 0
	lists := 0
	for _, call := range store.Calls {
		switch {
		case strings.HasPrefix(call, "read "):
			reads++
		case strings.HasPrefix(call, "list "):
			lists++
		}
	}
	if reads != 1 || lists != 1 || calls != 1 {
		t.Errorf("Aufrufe: open %d, list %d, read %d; erwartet je einen (%v)", calls, lists, reads, store.Calls)
	}
}

func TestDefaultsSourceFehlendesIstNichtVorhanden(t *testing.T) {
	store := defaultsStore(map[string]string{"guidelines/a.md": "# A\n"})
	calls := 0
	source := NewKephDefaultsSource(openStore(store, &calls))

	for _, dir := range []string{"reviews", "guidelines/a.md", "guidelines/leer"} {
		if _, err := source.List(dir); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("List(%s): %v, erwartet fs.ErrNotExist", dir, err)
		}
	}
	for _, file := range []string{"guidelines/b.md", "guidelines/a.md/x.md", "reviews/x.md"} {
		if _, err := source.Read(file); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("Read(%s): %v, erwartet fs.ErrNotExist", file, err)
		}
	}
	if _, err := source.Read("guidelines"); !errors.Is(err, ErrDocIsDir) {
		t.Errorf("Read(guidelines): %v, erwartet ErrDocIsDir", err)
	}
	for _, call := range store.Calls {
		if strings.HasPrefix(call, "read ") {
			t.Errorf("für Fehlendes den Node gefragt: %s", call)
		}
	}
}

func TestDefaultsSourceOhneKephalaion(t *testing.T) {
	calls := 0
	source := NewKephDefaultsSource(func() (DefaultsStore, string, error) {
		calls++
		return nil, "vm:k-playbook", docRefErrorf(DocCodeKephalaionUnavailable, "Kephalaion ist für dieses Projekt nicht bereit")
	})
	_, listErr := source.List("guidelines")
	_, readErr := source.Read("guidelines/a.md")
	for _, err := range []error{listErr, readErr} {
		if DocRefErrorCode(err) != DocCodeKephalaionUnavailable {
			t.Fatalf("Code %q, erwartet %s: %v", DocRefErrorCode(err), DocCodeKephalaionUnavailable, err)
		}
		if errors.Is(err, fs.ErrNotExist) {
			t.Errorf("darf nicht wie nicht vorhanden aussehen: %v", err)
		}
		for _, want := range []string{"vm:k-playbook/defaults/", "/k-gui"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("Meldung ohne %q: %v", want, err)
			}
		}
	}
	if calls != 1 {
		t.Errorf("open %d-mal gerufen, erwartet einmal — auch „nicht bereit“ bleibt stehen", calls)
	}
}

func TestDefaultsSourceCollectionNichtLesbar(t *testing.T) {
	store := defaultsStore(nil)
	store.Fail["list"] = &kephalaion.ToolError{Tool: "list", Message: "vm:k-playbook nicht lesbar"}
	calls := 0
	_, err := NewKephDefaultsSource(openStore(store, &calls)).List("guidelines")
	if DocRefErrorCode(err) != DocCodeKephalaionUnavailable {
		t.Fatalf("Code %q: %v", DocRefErrorCode(err), err)
	}
	for _, want := range []string{"vm:k-playbook", "Hub-Betreiber", "fetch_release"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Meldung ohne %q: %v", want, err)
		}
	}
}

func TestDefaultsSourceOhneVersionIstLeer(t *testing.T) {
	store := kephtest.New()
	store.Put("defaults/guidelines/a.md", "# A\n")
	calls := 0
	_, err := NewKephDefaultsSource(openStore(store, &calls)).Read("guidelines/a.md")
	if DocRefErrorCode(err) != DocCodeKephalaionUnavailable {
		t.Fatalf("Code %q: %v", DocRefErrorCode(err), err)
	}
	for _, want := range []string{"defaults/VERSION fehlt", "fetch_release für vm:k-playbook"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Meldung ohne %q: %v", want, err)
		}
	}
}

func TestDefaultsSourceLesefehlerIstNichtErreichbar(t *testing.T) {
	store := defaultsStore(map[string]string{"guidelines/a.md": "# A\n"})
	store.Fail["read"] = &kephalaion.ToolError{Tool: "read", Code: kephalaion.CodeNodeUnreachable, Message: "weg"}
	calls := 0
	_, err := NewKephDefaultsSource(openStore(store, &calls)).Read("guidelines/a.md")
	if DocRefErrorCode(err) != DocCodeKephalaionUnavailable {
		t.Fatalf("Code %q: %v", DocRefErrorCode(err), err)
	}
}

// Über die Auflösung: lokal vor mitgeliefert wie bei der Platte, Fehlendes
// ist not_found, eine nicht bereite Collection kephalaion_unavailable — auch
// wenn die lokale Ebene den Eintrag hat.
func TestDefaultsSourceHinterVerweisen(t *testing.T) {
	root := t.TempDir()
	writeDocTree(t, filepath.Join(root, "k-playbook-local"), map[string]string{"guidelines/b.md": "# B lokal\n\nText.\n"})
	store := defaultsStore(map[string]string{"guidelines/a.md": "# A\n", "guidelines/b.md": "# B\n"})
	calls := 0
	layers := DocLayers{Dist: NewKephDefaultsSource(openStore(store, &calls)), Local: NewProjectLocalDocSource(root)}

	a, err := ResolveDocRef(layers, "k-playbook:/guidelines/a.md")
	if err != nil || a.Content != "# A\n" || a.Parts[0].Origin != DocOriginDist {
		t.Errorf("a: %+v, %v", a, err)
	}
	b, err := ResolveDocRef(layers, "k-playbook:/guidelines/b.md")
	if err != nil || b.Content != "# B lokal\n\nText.\n" || b.Parts[0].Origin != DocOriginOverride {
		t.Errorf("b: %+v, %v", b, err)
	}
	if _, err := ResolveDocRef(layers, "k-playbook:/guidelines/c.md"); DocRefErrorCode(err) != DocCodeNotFound {
		t.Errorf("c: %v, erwartet not_found", err)
	}

	down := DocLayers{
		Dist: NewKephDefaultsSource(func() (DefaultsStore, string, error) {
			return nil, "vm:k-playbook", docRefErrorf(DocCodeKephalaionUnavailable, "nicht bereit")
		}),
		Local: NewProjectLocalDocSource(root),
	}
	if _, err := ResolveDocRef(down, "k-playbook:/guidelines/b.md"); DocRefErrorCode(err) != DocCodeKephalaionUnavailable {
		t.Errorf("lokaler Eintrag ohne mitgelieferte Ebene: %v, erwartet kephalaion_unavailable", err)
	}
}

// OpenDefaultsStore nimmt Hub und Account der Anbindung, die Collection
// heißt fest k-playbook. Ohne bereite Anbindung nennt der Fehler die
// Adresse mit dem Hub aus der Anbindung.
func TestOpenDefaultsStoreOhneBereiteAnbindung(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Pfade der Anbindung")
	}
	root := t.TempDir()
	writeDocTree(t, filepath.Join(root, "k-playbook-local"), map[string]string{
		kephalaion.LocalFileName: "kephalaion:\n  hub: vm\n  collection: projekt\n  account: kamran-wsl\n",
	})
	stubKephalaionCheck(t, kephalaion.Status{Configured: true, State: kephalaion.StateNodeUnreachable, Message: "Node nicht erreichbar."})
	_, address, err := realOpenDefaultsStore(root)
	if DocRefErrorCode(err) != DocCodeKephalaionUnavailable {
		t.Fatalf("Code %q: %v", DocRefErrorCode(err), err)
	}
	if address != "vm:k-playbook" {
		t.Errorf("Adresse %q, erwartet vm:k-playbook", address)
	}
}

func entryNames(entries []DocSourceEntry) string {
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name
		if entry.Dir {
			name += "/"
		}
		names = append(names, name)
	}
	return strings.Join(names, " ")
}

// List liest die Dateien des Verzeichnisses voraus; das spätere Read fragt
// den Node nicht mehr. README.md und Unterverzeichnisse liest es nicht.
func TestDefaultsSourceListLiestVoraus(t *testing.T) {
	store := defaultsStore(map[string]string{
		"reviews/review-a.md":       "# A\n",
		"reviews/review-b.md":       "# B\n",
		"reviews/README.md":         "# Liesmich\n",
		"reviews/unter/x.md":        "# X\n",
		"guidelines/nicht-gelistet": "# R\n",
	})
	calls := 0
	source := NewKephDefaultsSource(openStore(store, &calls))
	if _, err := source.List("reviews"); err != nil {
		t.Fatal(err)
	}
	reads := map[string]int{}
	for _, call := range store.Calls {
		if name, ok := strings.CutPrefix(call, "read "); ok {
			reads[name]++
		}
	}
	if reads["defaults/reviews/review-a.md"] != 1 || reads["defaults/reviews/review-b.md"] != 1 || len(reads) != 2 {
		t.Fatalf("vorausgelesen: %v", reads)
	}
	before := len(store.Calls)
	if data, err := source.Read("reviews/review-b.md"); err != nil || string(data) != "# B\n" {
		t.Fatalf("Read: %q, %v", data, err)
	}
	if len(store.Calls) != before {
		t.Errorf("Read nach dem Vorauslesen fragt den Node: %v", store.Calls[before:])
	}
}

// Ist die mitgelieferte Ebene nicht lesbar, scheitert das Briefing als
// Ganzes mit kephalaion_unavailable — auch wenn die lokale Ebene Anweisungen
// trägt: Sie wird nie allein ausgegeben. Die Meldung nennt Ursache und
// Abhilfe: ohne bereite Anbindung /k-gui, mit leerem defaults/ fetch_release.
func TestBriefingOhneMitgelieferteEbeneScheitert(t *testing.T) {
	root := newBriefingProject(t)
	stubBriefingDocs(t, unavailableDocs)

	stubDefaultsStore(t, realOpenDefaultsStore)
	stubKephalaionCheck(t, kephalaion.Status{State: kephalaion.StateNotConfigured, Message: "Keine Collection eingetragen."})
	_, err := BuildBriefing(t.Context(), root)
	if DocRefErrorCode(err) != DocCodeKephalaionUnavailable {
		t.Fatalf("ohne Anbindung: %v (Code %q), erwartet %s", err, DocRefErrorCode(err), DocCodeKephalaionUnavailable)
	}
	for _, want := range []string{"Keine Collection eingetragen", "/k-gui"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Meldung ohne %q: %v", want, err)
		}
	}

	empty := kephtest.New()
	stubDefaultsStore(t, func(string) (DefaultsStore, string, error) { return empty, "vm:k-playbook", nil })
	_, err = BuildBriefing(t.Context(), root)
	if DocRefErrorCode(err) != DocCodeKephalaionUnavailable || !strings.Contains(err.Error(), "fetch_release für vm:k-playbook") {
		t.Errorf("leeres defaults/: %v", err)
	}
}

// Die Kontextausgabe scheitert ohne mitgelieferte Ebene nicht — sie trägt
// unter kephalaion, warum —, führt aber reviews leer, statt die
// lokale Ebene allein zu zeigen, und nennt den Grund in catalogsProblem.
// checks liegt weiter in der Installation und bleibt.
func TestBuildContextOhneMitgelieferteEbene(t *testing.T) {
	root := newContextProject(t)
	write(t, filepath.Join(LocalDir(root), "reviews", "review-eigen.md"), "# Eigene\n\nText.\n")
	write(t, filepath.Join(PlaybookDir(root), "checks", "check_a.sh"), "#!/bin/sh\n")
	stubDefaultsStore(t, func(string) (DefaultsStore, string, error) {
		return nil, "vm:k-playbook", docRefErrorf(DocCodeKephalaionUnavailable, "nicht bereit")
	})

	built, err := BuildContext(root)
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}
	if len(built.Catalogs["reviews"]) != 0 {
		t.Errorf("lokale Ebene allein im Katalog: %+v", built.Catalogs)
	}
	if !strings.Contains(built.CatalogsProblem, "nicht bereit") {
		t.Errorf("catalogsProblem: %q", built.CatalogsProblem)
	}
	if len(built.Catalogs["checks"]) != 1 {
		t.Errorf("checks: %+v", built.Catalogs["checks"])
	}
}
