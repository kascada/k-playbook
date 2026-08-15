package webui

import (
	"encoding/json"
	"net/http"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// privateResponse ist der gemessene Ist-Zustand der Verzeichnisse, für die die
// Wahl „privat oder versioniert" ansteht. Die Struktur-Liste in /api/local
// bleibt davon unberührt: sie sagt, welche Verzeichnisse es gibt, diese Antwort
// sagt, wie sie im Repository dastehen.
type privateResponse struct {
	Available bool                    `json:"available"`
	Dir       string                  `json:"dir"`
	Entries   []project.PrivacyStatus `json:"entries"`
	// Untracked steht nur in der Antwort auf ein POST: die Dateien, die dabei
	// aus dem Index genommen wurden.
	Untracked []string `json:"untracked,omitempty"`
	Message   string   `json:"message"`
}

// privateRequest ist ein Eintrag plus Zielzustand, kein freier Pfad — der
// Handler führt schreibende git-Operationen aus.
type privateRequest struct {
	Path    string `json:"path"`
	Private bool   `json:"private"`
}

func localPrivateHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, privateState(""))
}

// setLocalPrivateHandler schaltet einen Eintrag um.
//
// Zulässig ist nur, was in LocalStructure() steht und dort Private trägt.
// Idempotent: steht der Eintrag bereits im Zielzustand, passiert nichts und die
// Antwort ist dieselbe.
func setLocalPrivateHandler(w http.ResponseWriter, r *http.Request) {
	environment := project.Detect()
	if !environment.Installed {
		writeJSON(w, http.StatusConflict, privateResponse{
			Message: "Keine " + project.ConfigFileName + " gefunden.",
		})
		return
	}

	var request privateRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, privateState("Anfrage nicht lesbar: "+err.Error()))
		return
	}

	entry, ok := project.PrivateEntry(request.Path)
	if !ok {
		writeJSON(w, http.StatusBadRequest, privateState(
			"Für "+request.Path+" steht diese Wahl nicht an; umgeschaltet werden nur die Verzeichnisse der lokalen Struktur."))
		return
	}

	change, err := project.SetPrivate(environment.ProjectDir, entry, request.Private)
	if err != nil {
		writeJSON(w, http.StatusConflict, privateState("Nicht umgeschaltet: "+err.Error()))
		return
	}

	response := privateState(change.Message)
	response.Untracked = change.Untracked
	writeJSON(w, http.StatusOK, response)
}

// privateState misst den Zustand aller betroffenen Einträge. Ohne
// Konfiguration ist noch nicht bekannt, wo sie liegen — dieselbe Behandlung wie
// in createLocalHandler.
func privateState(message string) privateResponse {
	environment := project.Detect()
	response := privateResponse{Available: environment.Installed, Message: message}
	if !environment.Installed {
		return response
	}

	response.Dir = project.LocalDir(environment.ProjectDir)
	response.Entries = project.PrivacyStatuses(environment.ProjectDir)
	return response
}
