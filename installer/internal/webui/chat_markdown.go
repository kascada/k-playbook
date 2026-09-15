package webui

import (
	"bytes"
	"fmt"
	"net/http"
)

// Antworten von OpenCode sind Markdown. Gerendert wird hier mit derselben
// Goldmark-Instanz wie die Doku (markdown in docs.go): ohne WithUnsafe lässt
// sie rohes HTML aus dem Text weg und entschärft gefährliche Verweise wie
// javascript:. Die Seite setzt das Ergebnis deshalb ohne eigenen Parser in den
// Verlauf ein.

// chatMarkdownLimit begrenzt die Zahl der Texte je Anfrage; die Gesamtgröße
// begrenzt chatBodyLimit.
const chatMarkdownLimit = 200

type chatMarkdownRequest struct {
	Texts []string `json:"texts"`
}

type chatMarkdownResponse struct {
	HTML []string `json:"html"`
}

// chatMarkdownHandler rendert mehrere Texte in einem Aufruf — ein geladener
// Verlauf bringt viele auf einmal —, in der Reihenfolge der Anfrage. Er fragt
// OpenCode nicht und braucht deshalb weder Projekt noch Dienst.
func chatMarkdownHandler(w http.ResponseWriter, r *http.Request) {
	var input chatMarkdownRequest
	if !decodeChatBody(w, r, &input, false) {
		return
	}
	if len(input.Texts) > chatMarkdownLimit {
		writeJSON(w, http.StatusBadRequest, chatError{Message: fmt.Sprintf("Höchstens %d Texte je Anfrage.", chatMarkdownLimit)})
		return
	}
	response := chatMarkdownResponse{HTML: make([]string, len(input.Texts))}
	for i, text := range input.Texts {
		var rendered bytes.Buffer
		if err := markdown.Convert([]byte(text), &rendered); err != nil {
			writeJSON(w, http.StatusInternalServerError, chatError{Message: fmt.Sprintf("Markdown rendern: %v", err)})
			return
		}
		response.HTML[i] = rendered.String()
	}
	writeJSON(w, http.StatusOK, response)
}
