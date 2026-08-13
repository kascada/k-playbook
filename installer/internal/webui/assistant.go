package webui

import (
	"net/http"
	"strings"

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

	// Ein Ablauf für alles: Einordnen und Auflösen der Instruktionsdateien, der
	// Anstoß, dann die Verlinkung. Die Reihenfolge steckt in der Funktion.
	setup, err := project.ApplyAssistantSetup(environment.ProjectDir)

	message := "Verlinkung eingerichtet."
	if note := describeSetup(setup); note != "" {
		message += " " + note
	}
	if err != nil {
		message += " Nicht vollständig eingerichtet: " + err.Error()
	}
	writeJSON(w, http.StatusOK, assistantState(message))
}

// describeSetup nennt im Klartext, was das Einrichten an den
// Instruktionsdateien getan hat — oder warum es nichts tun konnte.
//
// Der Konfliktfall gehört ausdrücklich in den Text: er ist kein
// Schönheitsfehler, sondern bedeutet, dass Claude Code das Playbook bis zur
// Handarbeit nicht kennt. Genau das steht im Detailtext.
func describeSetup(setup project.AssistantSetup) string {
	parts := []string{}
	if detail := setup.Instructions.Detail; detail != "" {
		parts = append(parts, detail)
	}

	switch {
	case setup.RootCreated:
		parts = append(parts, project.RootInstructionsFile+" aus der Vorlage angelegt.")
	case setup.RootExtended:
		parts = append(parts, "Der Anstoß steht jetzt in "+project.RootInstructionsFile+".")
	}

	return strings.Join(parts, " ")
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
