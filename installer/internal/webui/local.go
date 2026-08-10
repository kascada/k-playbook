package webui

import (
	"net/http"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// localResponse ist der Zustand der projekteigenen Struktur.
type localResponse struct {
	Available bool                       `json:"available"`
	Dir       string                     `json:"dir"`
	Entries   []project.LocalEntryStatus `json:"entries"`
	OK        bool                       `json:"ok"`
	Message   string                     `json:"message"`
}

func localHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, localState(""))
}

// createLocalHandler legt die fehlenden Teile der Struktur an.
func createLocalHandler(w http.ResponseWriter, r *http.Request) {
	environment := project.Detect()
	if !environment.Installed {
		writeJSON(w, http.StatusConflict, localResponse{
			Message: "Keine " + project.ConfigFileName + " gefunden. Es gibt kein Projekt zum Einrichten.",
		})
		return
	}

	message := "Struktur angelegt."
	if _, err := project.CreateLocal(environment.ProjectDir); err != nil {
		message = "Nicht vollstaendig angelegt: " + err.Error()
	}
	writeJSON(w, http.StatusOK, localState(message))
}

// localState liest den aktuellen Zustand. Ohne Konfiguration ist noch nicht
// bekannt, wo die Struktur hingehoert.
func localState(message string) localResponse {
	environment := project.Detect()
	response := localResponse{Available: environment.Installed, Message: message}
	if !environment.Installed {
		return response
	}

	response.Dir = project.LocalDir(environment.ProjectDir)
	response.Entries = project.CheckLocal(environment.ProjectDir)
	response.OK = project.LocalOK(response.Entries)
	return response
}
