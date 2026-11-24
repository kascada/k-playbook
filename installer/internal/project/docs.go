package project

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/inventory"
)

// Die Oberfläche zeigt zwei Handbücher (Task 153), jedes unter einem eigenen
// Menüpunkt und beide nur über Kephalaion — nichts ist ins Binary
// eingebettet, und es gibt keinen Rückfall auf die Platte:
//
//   - das Handbuch von k-playbook unter docs/handbook/ der mitgelieferten
//     Ebene, im Quell-Repo defaults/docs/handbook/, mit dem Release auf dem
//     Hub unter <hub>:k-playbook/defaults/docs/handbook/ (Menüpunkt „Docs“);
//   - das Handbuch des Projekts in seiner Collection unter
//     keph:/docs/manual/handbook/, einem Unterverzeichnis der Handdoku
//     (Menüpunkt „Handbuch“).
//
// Gelesen wird beides über eine DocSource, dieselbe Schnittstelle wie hinter
// k-playbook:-Verweisen. Ist ein Handbuch nicht lesbar, kommt ein
// HandbookError mit einem von zwei Codes zurück: Kephalaion ist nicht
// erreichbar, oder das Handbuch fehlt — beim Mitgelieferten ein Lieferumfang
// aus einem älteren Release, beim Projekt ein Verzeichnis, das es nicht gibt.

// KPlaybookHandbookDir ist das Handbuch von k-playbook, relativ zur
// mitgelieferten Ebene.
const KPlaybookHandbookDir = docDocsRoot + "/handbook"

// KPlaybookHandbookStart ist die Startseite des Handbuchs von k-playbook. Eine
// README.md gibt es dort nicht: Sie wäre eine Vorlage, und ListDocsTemplates
// schriebe aus ihr eine keph:/docs/handbook/README.md in jedes Projekt.
const KPlaybookHandbookStart = "overview.md"

// ProjectHandbookDir ist das Handbuch des Projekts in seiner Collection.
const ProjectHandbookDir = docDocsRoot + "/manual/handbook"

// ProjectHandbookStart ist die Startseite des Handbuchs eines Projekts: die
// README aus der Vorlage k-playbook:/docs/manual/handbook/README.md. Sie wird
// immer gezeigt, auch wenn sie noch der unveränderten Vorlage entspricht.
const ProjectHandbookStart = docsReadmeTemplateName

// HandbookCodeMissing meldet ein Handbuch, das es am erwarteten Ort nicht
// gibt, obwohl Kephalaion erreichbar ist.
const HandbookCodeMissing = "handbook_missing"

// HandbookError ist der Grund, aus dem ein Handbuch nicht gezeigt werden
// kann: Code ist DocCodeKephalaionUnavailable oder HandbookCodeMissing, die
// Meldung nennt den Ort und die Abhilfe.
type HandbookError struct {
	Code    string
	Message string
}

func (e *HandbookError) Error() string {
	return e.Message
}

// Doc ist eine Markdown-Seite eines Handbuchs.
type Doc struct {
	// Path ist der Ort relativ zum Handbuch, immer mit Schrägstrich.
	Path string `json:"path"`
	// Title ist die erste Überschrift der Seite, ersatzweise der Dateiname.
	Title string `json:"title"`
}

// Handbook ist ein Handbuch hinter einer DocSource: alle Markdown-Dateien
// unter Dir, auch in Unterverzeichnissen.
type Handbook struct {
	// Source ist die Ebene, aus der gelesen wird.
	Source DocSource
	// Dir ist das Verzeichnis des Handbuchs in Source; "" ist ihre Wurzel.
	Dir string
	// Start ist die Startseite: Sie steht in der Liste vorn.
	Start string
	// unavailable und missing bauen die Meldungen der beiden Fehlerfälle.
	unavailable func(err error) string
	missing     func() string
}

// KPlaybookHandbook ist das Handbuch von k-playbook eines Projekts, aus der
// mitgelieferten Ebene (NewProjectDefaultsSource).
func KPlaybookHandbook(projectDir string) Handbook {
	return KPlaybookHandbookFrom(NewProjectDefaultsSource(projectDir), defaultsAddress(projectDir))
}

// KPlaybookHandbookFrom ist das Handbuch von k-playbook aus einer gegebenen
// mitgelieferten Ebene; address ist die Adresse der öffentlichen Collection
// für die Meldungen.
func KPlaybookHandbookFrom(dist DocSource, address string) Handbook {
	return Handbook{
		Source: dist,
		Dir:    KPlaybookHandbookDir,
		Start:  KPlaybookHandbookStart,
		unavailable: func(err error) string {
			return fmt.Sprintf("Kephalaion ist nicht erreichbar: Das Handbuch von k-playbook liegt in %s unter %s/%s/ "+
				"und lässt sich ohne Kephalaion nicht zeigen. %v", address, defaultsDirName, KPlaybookHandbookDir, err)
		},
		missing: func() string {
			return fmt.Sprintf("Der Lieferumfang in %s enthält kein Handbuch (%s/%s/ fehlt); er stammt aus einem älteren "+
				"Release von k-playbook. Abhilfe: Der Hub holt das Release von k-playbook nach — Kephalaion "+
				"fetch_release für %s.", address, defaultsDirName, KPlaybookHandbookDir, address)
		},
	}
}

// ProjectHandbook ist das Handbuch eines Projekts in seiner Collection
// unter keph:/docs/manual/handbook/, geöffnet über OpenProjectLayerStore —
// derselbe Zugang wie für die projekteigene Ebene — erst beim ersten Zugriff.
func ProjectHandbook(projectDir string) Handbook {
	source := NewKephProjectHandbookSource(func() (DefaultsStore, string, error) { return OpenProjectLayerStore(projectDir) })
	return ProjectHandbookFrom(source, projectCollectionAddress(projectDir))
}

// NewKephProjectHandbookSource ist eine DocSource über docs/manual/handbook/
// einer Collection. open liefert Store und Adresse und wird höchstens einmal
// aufgerufen. Sie verhält sich wie die projekteigene Ebene
// (NewKephProjectLayerSource): Ein fehlendes Verzeichnis ist eine leere
// Quelle, kein Fehler; jeder Fehler beim Zugriff ist kephalaion_unavailable.
func NewKephProjectHandbookSource(open func() (DefaultsStore, string, error)) DocSource {
	return &kephCollectionSource{open: open, ctx: context.Background(), root: ProjectHandbookDir, project: true}
}

// ProjectHandbookFrom ist das Handbuch eines Projekts aus einer gegebenen
// Quelle, deren Wurzel keph:/docs/manual/handbook/ ist; address ist die
// Adresse der Collection für die Meldungen.
func ProjectHandbookFrom(source DocSource, address string) Handbook {
	ref := DocSchemeKeph + ":/" + ProjectHandbookDir + "/"
	return Handbook{
		Source: source,
		Start:  ProjectHandbookStart,
		unavailable: func(err error) string {
			return fmt.Sprintf("Kephalaion ist nicht erreichbar: Das Handbuch des Projekts liegt in %s unter %s "+
				"und lässt sich ohne Kephalaion nicht zeigen. %v — Abhilfe: Kephalaion einrichten auf der Statusseite "+
				"(Karte „Kephalaion-Collection einrichten“).", address, ref, rootCause(err))
		},
		missing: func() string {
			return fmt.Sprintf("Das Projekt hat kein Handbuch: %s gibt es in %s nicht. Die Startseite %s schreibt "+
				"k-playbook aus ihrer Vorlage %s (k-playbook docs readme manual/handbook).",
				ref, address, ProjectHandbookStart, DocsTemplateRef("manual/handbook"))
		},
	}
}

// List sammelt alle Markdown-Dateien des Handbuchs, auch aus
// Unterverzeichnissen. Die Startseite steht vorn, der Rest folgt
// alphabetisch. Ein Handbuch ohne eine einzige Seite gilt als fehlend.
func (h Handbook) List() ([]Doc, error) {
	files := []string{}
	var walk func(rel string) error
	walk = func(rel string) error {
		entries, err := h.Source.List(h.join(rel))
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name, ".") {
				continue
			}
			child := entry.Name
			if rel != "" {
				child = rel + "/" + entry.Name
			}
			switch {
			case entry.Dir:
				// Ein unlesbarer Teilbaum darf die übrige Liste nicht
				// verhindern — außer Kephalaion selbst ist weg.
				if err := walk(child); err != nil && errDefaultsUnavailable(err) {
					return err
				}
			case strings.EqualFold(path.Ext(entry.Name), ".md"):
				files = append(files, child)
			}
		}
		return nil
	}
	if err := walk(""); err != nil {
		return nil, h.mapError(err)
	}
	if len(files) == 0 {
		return nil, &HandbookError{Code: HandbookCodeMissing, Message: h.missing()}
	}

	docs := make([]Doc, 0, len(files))
	for _, file := range files {
		content, err := h.Source.Read(h.join(file))
		if err != nil && errDefaultsUnavailable(err) {
			return nil, h.mapError(err)
		}
		docs = append(docs, Doc{Path: file, Title: docTitle(file, content)})
	}
	sort.Slice(docs, func(i int, j int) bool {
		if start := docs[i].Path == h.Start; start != (docs[j].Path == h.Start) {
			return start
		}
		return docs[i].Path < docs[j].Path
	})
	return docs, nil
}

// Read liefert eine einzelne Seite des Handbuchs als Rohtext. Der Pfad kommt
// aus dem Browser: Er muss relativ bleiben, im Handbuch liegen und eine
// Markdown-Datei meinen.
func (h Handbook) Read(rel string) (Doc, []byte, error) {
	cleaned, err := handbookPath(rel)
	if err != nil {
		return Doc{}, nil, err
	}
	content, err := h.Source.Read(h.join(cleaned))
	switch {
	case err == nil:
	case errors.Is(err, fs.ErrNotExist):
		// Fehlt das ganze Handbuch, ist das die bessere Auskunft als eine
		// fehlende Seite.
		entries, listErr := h.Source.List(h.Dir)
		if listErr != nil {
			return Doc{}, nil, h.mapError(listErr)
		}
		if len(entries) == 0 {
			return Doc{}, nil, &HandbookError{Code: HandbookCodeMissing, Message: h.missing()}
		}
		return Doc{}, nil, fmt.Errorf("%s gibt es im Handbuch nicht", cleaned)
	case errors.Is(err, ErrDocIsDir):
		return Doc{}, nil, InputErrorf("%s ist ein Verzeichnis, keine Seite", cleaned)
	default:
		return Doc{}, nil, h.mapError(err)
	}
	return Doc{Path: cleaned, Title: docTitle(cleaned, content)}, content, nil
}

// join setzt einen Pfad relativ zum Handbuch vor Dir.
func (h Handbook) join(rel string) string {
	switch {
	case h.Dir == "":
		return rel
	case rel == "":
		return h.Dir
	}
	return h.Dir + "/" + rel
}

// mapError macht aus dem Fehler der Quelle die Auskunft der Oberfläche:
// Kephalaion nicht erreichbar, Handbuch fehlt, sonst der Fehler selbst.
func (h Handbook) mapError(err error) error {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return &HandbookError{Code: HandbookCodeMissing, Message: h.missing()}
	case DocRefErrorCode(err) == DocCodeKephalaionUnavailable:
		return &HandbookError{Code: DocCodeKephalaionUnavailable, Message: h.unavailable(err)}
	}
	return err
}

// rootCause ist der innerste Fehler einer Kette: Die Hüllen der Quelle
// sprechen von der projekteigenen Ebene, gemeint ist hier das Handbuch.
func rootCause(err error) error {
	for {
		inner := errors.Unwrap(err)
		if inner == nil {
			return err
		}
		err = inner
	}
}

// handbookPath prüft einen angefragten Pfad. Alles, was aus dem Handbuch
// herausführt oder keine Markdown-Datei meint, wäre ein Weg, anderes aus der
// Collection zu lesen.
func handbookPath(rel string) (string, error) {
	if strings.TrimSpace(rel) == "" {
		return "", InputErrorf("kein Pfad angegeben")
	}
	if strings.HasPrefix(rel, "/") || strings.Contains(rel, "\\") || filepath.IsAbs(rel) {
		return "", InputErrorf("nur Pfade innerhalb des Handbuchs")
	}
	cleaned := path.Clean(rel)
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") || cleaned == "." {
		return "", InputErrorf("nur Pfade innerhalb des Handbuchs")
	}
	for _, part := range strings.Split(rel, "/") {
		if part == ".." {
			return "", InputErrorf("nur Pfade innerhalb des Handbuchs")
		}
	}
	if !strings.EqualFold(path.Ext(cleaned), ".md") {
		return "", InputErrorf("nur Markdown-Dateien")
	}
	return cleaned, nil
}

// docTitle nimmt die erste Überschrift der Seite, nach dem Frontmatter. Sie
// beschreibt den Inhalt besser als der Dateiname, der nur der Rückfall ist.
func docTitle(file string, content []byte) string {
	fallback := strings.TrimSuffix(path.Base(file), path.Ext(file))
	for _, line := range strings.Split(string(inventory.Body(content)), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return fallback
}
