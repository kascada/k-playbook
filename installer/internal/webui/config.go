package webui

import (
	"encoding/json"
	"net/http"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// configResponse beschreibt den Konfigurationszustand fuer die Oberflaeche.
// Ist noch nichts angelegt, traegt Suggestion die Vorbelegung des Formulars.
type configResponse struct {
	Installed   bool                `json:"installed"`
	ProjectDir  string              `json:"projectDir"`
	ConfigPath  string              `json:"configPath"`
	PlaybookDir string              `json:"playbookDir"`
	RepoRoot    string              `json:"repoRoot"`
	VCS         string              `json:"vcs"`
	Suggestion  *project.Suggestion `json:"suggestion,omitempty"`
	Message     string              `json:"message"`
}

// createConfigRequest ist die Auswahl des Nutzers aus dem Formular.
type createConfigRequest struct {
	ProjectDir string `json:"projectDir"`
	RepoRoot   string `json:"repoRoot"`
}

func configHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, configState(""))
}

// createConfigHandler legt die K-PLAYBOOK.yaml an dem vom Nutzer bestaetigten
// Ort an.
func createConfigHandler(w http.ResponseWriter, r *http.Request) {
	var request createConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, configResponse{Message: "Anfrage nicht lesbar: " + err.Error()})
		return
	}

	if err := project.CreateConfig(request.ProjectDir, request.RepoRoot); err != nil {
		writeJSON(w, http.StatusConflict, configState("Nicht angelegt: "+err.Error()))
		return
	}

	writeJSON(w, http.StatusOK, configState(project.ConfigFileName+" angelegt."))
}

// configState ermittelt den aktuellen Zustand. Gesucht wird ab dem
// Arbeitsverzeichnis aufwaerts; erst wenn nichts gefunden wird, greift der
// aus dem Ort des Binaries abgeleitete Vorschlag.
func configState(message string) configResponse {
	environment := project.Detect()
	response := configResponse{
		Installed: environment.Installed,
		Message:   message,
	}

	if !environment.Installed {
		suggestion := project.Suggest()
		response.Suggestion = &suggestion
		if response.Message == "" {
			response.Message = "Noch keine " + project.ConfigFileName +
				". Ort bestaetigen, dann wird sie angelegt."
		}
		return response
	}

	response.ProjectDir = environment.ProjectDir
	response.ConfigPath = project.ConfigPath(environment.ProjectDir)
	response.PlaybookDir = environment.PlaybookDir

	config, err := project.ReadConfig(environment.ProjectDir)
	if err != nil {
		response.Message = project.ConfigFileName + " nicht lesbar: " + err.Error()
		return response
	}
	// Anders als bei `context` wird hier nicht abgebrochen: die Oberflaeche
	// soll den Zustand zeigen koennen, gerade wenn etwas nicht stimmt.
	if err := project.CheckSchema(config); err != nil && response.Message == "" {
		response.Message = err.Error()
	}
	response.RepoRoot = project.RepoRootDir(environment.ProjectDir, config)
	response.VCS = config.VCS
	return response
}
