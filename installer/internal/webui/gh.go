package webui

import (
	"encoding/json"
	"net/http"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// ghResponse ist die Entscheidung zur GitHub CLI samt Host-Befund.
//
// Geschrieben wird hier nur die Entscheidung. Installation und Anmeldung bleiben
// im Terminal: beides veraendert den Host und nicht das Projekt, und `gh auth
// login` will ohnehin einen Browser. Die Oberflaeche zeigt dafuer den Befehl.
type ghResponse struct {
	Available bool               `json:"available"`
	Current   project.GH         `json:"current"`
	Choices   []project.GHChoice `json:"choices"`
	Commands  ghCommands         `json:"commands"`
	Message   string             `json:"message"`
}

// ghCommands sind die Befehle zum Kopieren, je nach Zustand.
type ghCommands struct {
	Install string `json:"install"`
	Login   string `json:"login"`
	Switch  string `json:"switch"`
	Status  string `json:"status"`
}

type ghRequest struct {
	Status string `json:"status"`
}

func ghHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, ghState(""))
}

// setGHHandler schreibt die Projektentscheidung nach tools.gh.status.
func setGHHandler(w http.ResponseWriter, r *http.Request) {
	environment := project.Detect()
	if !environment.Installed {
		writeJSON(w, http.StatusConflict, ghResponse{
			Message: "Keine " + project.ConfigFileName + " gefunden.",
		})
		return
	}

	var request ghRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, ghState("Anfrage nicht lesbar: "+err.Error()))
		return
	}

	if err := project.SetGHStatus(environment.ProjectDir, project.GHStatus(request.Status)); err != nil {
		writeJSON(w, http.StatusConflict, ghState("Nicht gespeichert: "+err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, ghState("Gespeichert."))
}

func ghState(message string) ghResponse {
	environment := project.Detect()
	response := ghResponse{
		Available: environment.Installed,
		Choices:   project.GHChoices(),
		Commands:  ghCommandSet(),
		Message:   message,
	}
	if !environment.Installed {
		return response
	}

	state, err := project.GHState(environment.ProjectDir)
	if err != nil {
		response.Current = state
		response.Message = project.ConfigFileName + ": " + err.Error()
		return response
	}
	response.Current = state
	return response
}

// ghCommandSet nennt fuer jeden Zustand den passenden Befehl. Der
// Installationsbefehl haengt am Paketmanager des Rechners; statt zu raten,
// verweist er auf die Anleitung.
func ghCommandSet() ghCommands {
	return ghCommands{
		Install: "https://github.com/cli/cli#installation",
		Login:   "gh auth login --hostname " + project.GHHost,
		Switch:  "gh auth switch --hostname " + project.GHHost + " --user <account>",
		Status:  "gh auth status",
	}
}
