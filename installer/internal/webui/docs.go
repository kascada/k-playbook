package webui

import (
	"bytes"
	"net/http"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// markdown rendert die Doku. GFM deckt Tabellen und Aufgabenlisten ab, die in
// den mitgelieferten Dateien vorkommen; Überschriften bekommen eine ID, damit
// Verweise innerhalb einer Datei funktionieren.
//
// Rohes HTML aus der Quelle bleibt bewusst abgeschaltet — das ist die
// Voreinstellung von goldmark und genau richtig für Text, der einfach im
// Browser landet.
var markdown = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
)

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

	var rendered bytes.Buffer
	if err := markdown.Convert(content, &rendered); err != nil {
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
