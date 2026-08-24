package webui

import (
	"net/http"

	"github.com/kascada/k-playbook/installer/internal/project"
	"github.com/kascada/k-playbook/installer/internal/review"
)

// reviewsResponse ist die Liste der bisherigen Läufe. Gestartet wird ein Lauf
// nicht mehr über die Oberfläche — dafür sind /k-audit und /k-review im
// Assistenten zuständig, beide über den MCP-Server. Diese Seite zeigt nur,
// was bereits vorliegt.
type reviewsResponse struct {
	Available bool             `json:"available"`
	Runs      []review.Summary `json:"runs"`
	Message   string           `json:"message"`
}

func reviewsHandler(w http.ResponseWriter, r *http.Request) {
	environment := project.Detect()
	if !environment.Installed {
		writeJSON(w, http.StatusOK, reviewsResponse{})
		return
	}

	response := reviewsResponse{Available: true}

	runs, err := review.ListRuns(project.LocalDir(environment.ProjectDir))
	if err != nil {
		response.Message = err.Error()
	}
	response.Runs = runs

	writeJSON(w, http.StatusOK, response)
}
