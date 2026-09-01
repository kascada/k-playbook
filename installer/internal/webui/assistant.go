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

// assistantHandler liest den Zustand — und zieht dabei nach, was sich von
// selbst nachziehen lässt.
//
// Ein reiner Lesepfad wäre hier zu wenig. Die Registrierung hängt am Katalog
// des Projekts, und der ändert sich hinter dem Rücken der Oberfläche: ein
// aktualisierter Stand der Installation, ein neuer projekteigener Command, ein
// umbenannter mitgelieferter. Ohne die Heilung stünde die Abweichung nur in der
// Karte und bliebe dort, bis jemand sie wegklickt. Genau so überlebt ein toter
// Link auf einen umbenannten Command wochenlang, ohne dass es jemandem auffällt.
//
// Was das Einrichten nicht auflösen kann, bleibt liegen und steht weiter in der
// Karte. Der Knopf bleibt deshalb: er tut mehr als die Heilung — er ordnet auch
// das Paar CLAUDE.md/AGENTS.md ein und legt den Anstoß an.
func assistantHandler(w http.ResponseWriter, r *http.Request) {
	environment := project.Detect()
	if !environment.Installed {
		writeJSON(w, http.StatusOK, assistantState(""))
		return
	}

	repair := project.HealLinks(environment.ProjectDir)
	writeJSON(w, http.StatusOK, assistantState(describeRepair(repair)))
}

// describeRepair formuliert, was die Selbstheilung getan hat. Stimmte alles,
// bleibt der Text leer — die Karte meldet dann ohnehin „eingerichtet".
func describeRepair(repair project.LinkRepair) string {
	parts := []string{}
	switch {
	case !repair.Changed.Empty():
		parts = append(parts, describeLinkChanges(repair.Changed))
	case repair.Applied:
		// Ein Ziel, das es vorher gar nicht gab: da hat sich keine
		// Registrierung verändert, da ist eine entstanden.
		parts = append(parts, "Verlinkung eingerichtet.")
	}
	if repair.Error != "" {
		parts = append(parts, "Nicht vollständig eingerichtet: "+repair.Error)
	}
	return strings.Join(parts, " ")
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
