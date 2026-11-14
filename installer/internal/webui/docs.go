package webui

import (
	"bytes"
	"errors"
	"net/http"

	"github.com/kascada/k-playbook/installer/internal/inventory"
	mdconfig "github.com/kascada/k-playbook/installer/internal/markdown"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// markdown rendert die Doku. Die Konfiguration — GFM, automatische
// Überschriften-Ids, kein rohes HTML — steht in internal/markdown, an genau
// einer Stelle.
var markdown = mdconfig.New()

// Die Oberfläche zeigt zwei Handbücher über dieselbe Anzeige (Task 153):
// „Docs“ das Handbuch von k-playbook aus der mitgelieferten Ebene
// (/api/docs), „Handbuch“ das des Projekts aus keph:/docs/manual/handbook/
// (/api/handbook). Beide liest project.Handbook; welches, wählt der Handler.

// docsResponse ist die Liste der verfügbaren Seiten. Kann das Handbuch nicht
// gezeigt werden, steht statt der Liste eine Meldung da, und Code sagt,
// warum: kephalaion_unavailable oder handbook_missing.
type docsResponse struct {
	Available bool          `json:"available"`
	Docs      []project.Doc `json:"docs"`
	Message   string        `json:"message"`
	Code      string        `json:"code,omitempty"`
}

// docResponse ist eine einzelne Seite, fertig gerendert.
type docResponse struct {
	Available bool   `json:"available"`
	Path      string `json:"path"`
	Title     string `json:"title"`
	HTML      string `json:"html"`
	Message   string `json:"message"`
	Code      string `json:"code,omitempty"`
}

// handbookOpener wählt das Handbuch eines Projekts.
type handbookOpener func(projectDir string) project.Handbook

// openKPlaybookHandbook und openProjectHandbook sind austauschbar, damit
// Tests den Node nicht fragen.
var (
	openKPlaybookHandbook handbookOpener = project.KPlaybookHandbook
	openProjectHandbook   handbookOpener = project.ProjectHandbook
)

func docsHandler(w http.ResponseWriter, r *http.Request) {
	handbookListHandler(w, openKPlaybookHandbook)
}

func docFileHandler(w http.ResponseWriter, r *http.Request) {
	handbookFileHandler(w, r, openKPlaybookHandbook)
}

func projectHandbookHandler(w http.ResponseWriter, r *http.Request) {
	handbookListHandler(w, openProjectHandbook)
}

func projectHandbookFileHandler(w http.ResponseWriter, r *http.Request) {
	handbookFileHandler(w, r, openProjectHandbook)
}

// handbookCode ist der Code eines Fehlers, wenn er ein HandbookError ist.
func handbookCode(err error) string {
	var handbookErr *project.HandbookError
	if errors.As(err, &handbookErr) {
		return handbookErr.Code
	}
	return ""
}

func handbookListHandler(w http.ResponseWriter, open handbookOpener) {
	environment := project.Detect()
	if !environment.Installed {
		writeJSON(w, http.StatusOK, docsResponse{})
		return
	}

	docs, err := open(environment.ProjectDir).List()
	if err != nil {
		writeJSON(w, http.StatusOK, docsResponse{Available: true, Message: err.Error(), Code: handbookCode(err)})
		return
	}

	writeJSON(w, http.StatusOK, docsResponse{Available: true, Docs: docs})
}

func handbookFileHandler(w http.ResponseWriter, r *http.Request, open handbookOpener) {
	environment := project.Detect()
	if !environment.Installed {
		writeJSON(w, http.StatusOK, docResponse{})
		return
	}

	doc, content, err := open(environment.ProjectDir).Read(r.URL.Query().Get("path"))
	if err != nil {
		writeJSON(w, http.StatusOK, docResponse{Available: true, Message: err.Error(), Code: handbookCode(err)})
		return
	}

	// Der Frontmatter-Block gehört nicht in die Ansicht: er trägt Metadaten für
	// den Doku-Index, keinen Text. Ungetrennt läse Goldmark ihn als Trennlinie
	// samt Überschrift — die Datei begänne mit ihren eigenen Kopfdaten.
	// Abgetrennt wird er von derselben Stelle wie beim Inventar; ohne
	// Frontmatter ist der Rumpf die ganze Datei.
	var rendered bytes.Buffer
	if err := markdown.Convert(inventory.Body(content), &rendered); err != nil {
		writeJSON(w, http.StatusOK, docResponse{
			Available: true,
			Path:      doc.Path,
			Title:     doc.Title,
			Message:   "Markdown konnte nicht gerendert werden: " + err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, docResponse{
		Available: true,
		Path:      doc.Path,
		Title:     doc.Title,
		HTML:      rendered.String(),
	})
}
