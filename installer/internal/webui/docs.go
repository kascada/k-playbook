package webui

import (
	"bytes"
	"net/http"

	"github.com/kascada/k-playbook/installer/internal/inventory"
	mdconfig "github.com/kascada/k-playbook/installer/internal/markdown"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// markdown rendert die Doku. Die Konfiguration — GFM, automatische
// Überschriften-Ids, kein rohes HTML — steht in internal/markdown und ist
// dieselbe, mit der das Wissenstor in project/ die Anker seiner Chunks bildet:
// nur so treffen Sprünge aus einem Suchtreffer die gerenderte Überschrift.
var markdown = mdconfig.New()

// docsResponse ist die Liste der verfügbaren Dateien.
type docsResponse struct {
	Available bool          `json:"available"`
	Docs      []project.Doc `json:"docs"`
	Message   string        `json:"message"`
}

// docResponse ist eine einzelne Datei, fertig gerendert.
type docResponse struct {
	Available bool   `json:"available"`
	Path      string `json:"path"`
	Title     string `json:"title"`
	HTML      string `json:"html"`
	Message   string `json:"message"`
}

func docsHandler(w http.ResponseWriter, r *http.Request) {
	environment := project.Detect()
	if !environment.Installed {
		writeJSON(w, http.StatusOK, docsResponse{})
		return
	}

	docs, err := project.ListDocs(environment.ProjectDir)
	if err != nil {
		writeJSON(w, http.StatusOK, docsResponse{Available: true, Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, docsResponse{Available: true, Docs: docs})
}

func docFileHandler(w http.ResponseWriter, r *http.Request) {
	environment := project.Detect()
	if !environment.Installed {
		writeJSON(w, http.StatusOK, docResponse{})
		return
	}

	doc, content, err := project.ReadDoc(environment.ProjectDir, r.URL.Query().Get("path"))
	if err != nil {
		writeJSON(w, http.StatusOK, docResponse{Available: true, Message: err.Error()})
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
