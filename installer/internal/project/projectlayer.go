package project

import (
	"context"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
)

// Die projekteigene Ebene hinter k-playbook:-Verweisen liegt für die Sorten
// instructions/, includes/ und reviews/ in der Collection des Projekts unter
// k-playbook/ (Task 143), mit demselben Baum wie unter k-playbook:/ — neben
// docs/, queue/, todos/ und user/. Gleicher Name ersetzt das Mitgelieferte,
// eine leere Datei schaltet es ab, ein neuer Name kommt hinzu; die Regel ist
// dieselbe wie für jede Ebene (overlayDocDir). k-playbook-local/instructions/,
// includes/ und reviews/ liest seitdem nichts mehr (harter Wechsel); /k-upgrade
// räumt sie ab.
//
// Fehlt k-playbook/ in der Collection, ist die Ebene leer, ohne Fehler. Ein
// Fehler beim Zugriff auf die Collection ist dagegen kephalaion_unavailable,
// auch wenn <hub>:k-playbook lesbar ist: Es gibt keinen stillen Rückfall auf
// das Mitgelieferte allein.

// ProjectLayerDirName ist das Verzeichnis in der Collection des Projekts, das
// die projekteigene Ebene trägt.
const ProjectLayerDirName = "k-playbook"

// projectLayerKinds sind die Sorten, deren projekteigene Ebene in der
// Collection liegt. commands/ bleibt in .k-playbook/commands/ (die Assistenten
// brauchen die Hüllen auf der Platte), guidelines/ und testdata/ bleiben bis
// zu ihrem Umbau in k-playbook-local/.
var projectLayerKinds = []string{docInstructionsRoot, docIncludesRoot, "reviews"}

// OpenProjectLayerStore öffnet den Zugang zur Collection des Projekts für die
// projekteigene Ebene: derselbe Weg wie OpenKephDocs (Status, Anbindung,
// Umgebung), als kephalaion.ReaderPool wie OpenDefaultsStore, weil das
// Briefing die Dateien eines Verzeichnisses nebeneinander vorausliest. Zurück
// kommt neben dem Store die Adresse der Collection für Meldungen. Ein Fehler
// trägt DocCodeKephalaionUnavailable. Austauschbar, damit Tests den Node nicht
// fragen.
var OpenProjectLayerStore = func(projectDir string) (DefaultsStore, string, error) {
	client, _, err := openKephClient(projectDir)
	if err != nil {
		return nil, projectCollectionAddress(projectDir), err
	}
	return client.ReaderPool(defaultsReaders), client.Address(), nil
}

// projectCollectionAddress nennt die Adresse der Collection des Projekts für
// eine Meldung, auch wenn der Zugang nicht zustande kommt.
func projectCollectionAddress(projectDir string) string {
	if binding, err := ProjectBinding(projectDir); err == nil && binding.Hub != "" && binding.Collection != "" {
		return binding.Hub + ":" + binding.Collection
	}
	return "<Collection des Projekts>"
}

// NewKephProjectLayerSource ist eine DocSource über k-playbook/ einer
// Collection — die projekteigene Ebene. open liefert Store und Adresse und
// wird höchstens einmal aufgerufen; auch „nicht bereit“ bleibt stehen. Sie
// verhält sich wie NewKephDefaultsSource (einmal rekursiv listen,
// vorauslesen, nur Vorhandenes lesen), nur ist eine leere Liste unter
// k-playbook/ eine leere Ebene und kein Fehler.
func NewKephProjectLayerSource(open func() (DefaultsStore, string, error)) DocSource {
	return &kephCollectionSource{open: open, ctx: context.Background(), root: ProjectLayerDirName, project: true}
}

// NewProjectLayerSource ist die projekteigene Ebene eines Projekts in seiner
// Collection, geöffnet über OpenProjectLayerStore erst beim ersten Zugriff.
func NewProjectLayerSource(projectDir string) DocSource {
	return NewKephProjectLayerSource(func() (DefaultsStore, string, error) { return OpenProjectLayerStore(projectDir) })
}

// indexProject öffnet die Collection des Projekts und listet k-playbook/
// einmal. Jeder Fehler dabei ist kephalaion_unavailable; eine leere Liste ist
// eine leere Ebene.
func (s *kephCollectionSource) indexProject() error {
	store, address, err := s.open()
	if address == "" {
		address = "<Collection des Projekts>"
	}
	s.address = address
	if err != nil {
		return docRefErrorf(DocCodeKephalaionUnavailable,
			"die projekteigene Ebene hinter k-playbook: liegt in %s unter %s:/%s/ und ist ohne Kephalaion nicht erreichbar: %w — "+
				"Abhilfe: Kephalaion einrichten über /k-gui (k-playbook gui, Abschnitt Kephalaion)",
			address, DocSchemeKeph, s.root, err)
	}
	entries, err := store.List(s.ctx, kephalaion.ListRequest{Path: s.root, Recursive: true})
	if err != nil {
		return docRefErrorf(DocCodeKephalaionUnavailable,
			"die projekteigene Ebene ist nicht lesbar: %s:/%s/ in %s lässt sich nicht auflisten (%v) — gibt es die "+
				"Collection nicht oder darf der Account sie nicht lesen, die Anbindung über /k-gui prüfen",
			DocSchemeKeph, s.root, address, err)
	}
	s.remember(store, entries)
	return nil
}

// isProjectLayerKind meldet, ob ein Pfad unter k-playbook: in einer Sorte
// liegt, deren projekteigene Ebene in der Collection liegt.
func isProjectLayerKind(refPath string) bool {
	return inDocKinds(refPath, projectLayerKinds)
}

// projectLayerLocation ist der Ort eines projekteigenen Teils in der
// Collection als keph:-Verweis, etwa keph:/k-playbook/reviews/review-x.md für
// k-playbook:/reviews/review-x.md.
func projectLayerLocation(file string) string {
	return DocSchemeKeph + ":/" + ProjectLayerDirName + "/" + strings.TrimPrefix(file, "/")
}
