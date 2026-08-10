package webui

import (
	"net/http"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// updateResponse ist der Zustand der Aktualisierung.
type updateResponse struct {
	Available bool   `json:"available"`
	Branch    string `json:"branch"`
	Local     string `json:"local"`
	Remote    string `json:"remote"`
	Output    string `json:"output"`
	// RestartRequired: die Binaries wurden ersetzt, der laufende Prozess
	// arbeitet aber weiter mit dem alten Code.
	RestartRequired bool   `json:"restartRequired"`
	Message         string `json:"message"`
}

// updateCheckHandler prueft den Remote-Stand. Rein lesend.
func updateCheckHandler(w http.ResponseWriter, r *http.Request) {
	environment := project.Detect()
	if !environment.Installed {
		writeJSON(w, http.StatusOK, updateResponse{})
		return
	}

	status, err := project.CheckUpdate(environment.ProjectDir)
	if err != nil {
		writeJSON(w, http.StatusOK, updateResponse{Message: "Pruefung fehlgeschlagen: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, updateResponse{
		Available: status.Available,
		Branch:    status.Branch,
		Local:     shortCommit(status.Local),
		Remote:    shortCommit(status.Remote),
		Message:   status.Message,
	})
}

// applyUpdateHandler holt den neuen Stand.
func applyUpdateHandler(w http.ResponseWriter, r *http.Request) {
	environment := project.Detect()
	if !environment.Installed {
		writeJSON(w, http.StatusConflict, updateResponse{
			Message: "Keine " + project.ConfigFileName + " gefunden.",
		})
		return
	}

	result, err := project.Update(environment.ProjectDir)
	if err != nil {
		writeJSON(w, http.StatusConflict, updateResponse{
			Output:  result.Output,
			Message: err.Error(),
		})
		return
	}

	response := updateResponse{
		Output:          result.Output,
		RestartRequired: result.BinaryChanged,
		Message:         "Aktualisiert.",
	}
	if result.BinaryChanged {
		response.Message = "Aktualisiert. Das Programm wurde ersetzt und laeuft bis zum Neustart mit dem bisherigen Stand."
	}

	// Nach dem Pull erneut pruefen, damit der Button den neuen Zustand zeigt.
	if status, err := project.CheckUpdate(environment.ProjectDir); err == nil {
		response.Available = status.Available
		response.Branch = status.Branch
		response.Local = shortCommit(status.Local)
		response.Remote = shortCommit(status.Remote)
	}
	writeJSON(w, http.StatusOK, response)
}

// shortCommit kuerzt einen Commit-Hash auf die uebliche Anzeigelaenge.
func shortCommit(hash string) string {
	if len(hash) > 7 {
		return hash[:7]
	}
	return hash
}
