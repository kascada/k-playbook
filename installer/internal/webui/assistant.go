package webui

import (
	"net/http"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// assistantResponse ist der Zustand der Verlinkung, wie ihn die Oberfläche
// braucht: Kontext, Einzelzustände und ein Flag für die Button-Darstellung.
type assistantResponse struct {
	Environment project.Environment           `json:"environment"`
	Entries     []project.LinkStatus          `json:"entries"`
	Root        project.RootInstructionsState `json:"root"`
	OK          bool                          `json:"ok"`
	Message     string                        `json:"message"`
}

func assistantHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, assistantState(""))
}

// applyAssistantHandler richtet die Verlinkung ein. Ohne gefundene Config gibt
// es kein Projekt, auf das sich die Aktion beziehen könnte.
func applyAssistantHandler(w http.ResponseWriter, r *http.Request) {
	environment := project.Detect()
	if !environment.Installed {
		writeJSON(w, http.StatusConflict, assistantResponse{
			Environment: environment,
			Message:     "Keine " + project.ConfigFileName + " gefunden. Es gibt kein Projekt zum Einrichten.",
		})
		return
	}

	// Erst die Wurzeldatei: der Symlink CLAUDE.md braucht sie als Ziel.
	message := "Verlinkung eingerichtet."
	if _, err := project.ApplyRootInstructions(environment.ProjectDir); err != nil {
		message = "Nicht vollständig eingerichtet: " + err.Error()
	} else if _, err := project.ApplyLinks(environment.ProjectDir); err != nil {
		message = "Nicht vollständig eingerichtet: " + err.Error()
	}
	writeJSON(w, http.StatusOK, assistantState(message))
}

// assistantState liest den aktuellen Zustand.
func assistantState(message string) assistantResponse {
	environment := project.Detect()
	response := assistantResponse{Environment: environment, Message: message}

	switch {
	case !environment.Installed:
		if response.Message == "" {
			response.Message = "Keine " + project.ConfigFileName + " gefunden (gesucht ab " +
				project.DisplayPath(environment.SearchedFrom) + " aufwärts)."
		}

	case !environment.PlaybookPresent:
		if response.Message == "" {
			response.Message = "Installationsverzeichnis " + project.PlaybookDirName +
				"/ fehlt. Typisch nach einem frischen Clone des Projekts."
		}

	default:
		response.Entries = project.CheckLinks(environment.ProjectDir)
		response.Root = project.CheckRootInstructions(environment.ProjectDir)
		response.OK = project.LinksOK(response.Entries) && response.Root.OK()
	}

	return response
}
