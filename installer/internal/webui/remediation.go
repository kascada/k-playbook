package webui

import (
	"encoding/json"
	"net/http"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// remediationResponse ist der Zustand der Remediation-Policy.
type remediationResponse struct {
	Available bool                        `json:"available"`
	Current   project.Remediation         `json:"current"`
	Choices   []project.RemediationChoice `json:"choices"`
	Message   string                      `json:"message"`
}

type remediationRequest struct {
	Mode string `json:"mode"`
}

func remediationHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, remediationState(""))
}

// setRemediationHandler schreibt den gewaehlten Modus samt abgeleiteter Flags.
func setRemediationHandler(w http.ResponseWriter, r *http.Request) {
	environment := project.Detect()
	if !environment.Installed {
		writeJSON(w, http.StatusConflict, remediationResponse{
			Message: "Keine " + project.ConfigFileName + " gefunden.",
		})
		return
	}

	var request remediationRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, remediationState("Anfrage nicht lesbar: "+err.Error()))
		return
	}

	if err := project.SetRemediationMode(environment.ProjectDir, project.RemediationMode(request.Mode)); err != nil {
		writeJSON(w, http.StatusConflict, remediationState("Nicht gespeichert: "+err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, remediationState("Gespeichert."))
}

func remediationState(message string) remediationResponse {
	environment := project.Detect()
	response := remediationResponse{
		Available: environment.Installed,
		Choices:   project.RemediationModes(),
		Message:   message,
	}
	if !environment.Installed {
		return response
	}

	remediation, err := project.ReadRemediation(environment.ProjectDir)
	if err != nil {
		response.Message = project.ConfigFileName + " nicht lesbar: " + err.Error()
		return response
	}
	response.Current = remediation
	return response
}
