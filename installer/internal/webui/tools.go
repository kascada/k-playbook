package webui

import (
	"net/http"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// toolsResponse ist der Zustand der Security-Tools.
//
// Rein lesend: installiert wird im Terminal, weil das den Host veraendert und
// nicht das Projekt. Die Oberflaeche zeigt dafuer den fertigen Befehl.
type toolsResponse struct {
	Available bool           `json:"available"`
	Tools     []project.Tool `json:"tools"`
	BinDir    string         `json:"binDir"`
	Command   string         `json:"command"`
	Missing   int            `json:"missing"`
	OK        bool           `json:"ok"`
	Message   string         `json:"message"`
}

func toolsHandler(w http.ResponseWriter, r *http.Request) {
	environment := project.Detect()
	if !environment.Installed {
		writeJSON(w, http.StatusOK, toolsResponse{})
		return
	}

	preflight, err := project.CheckTools(environment.ProjectDir)
	if err != nil {
		writeJSON(w, http.StatusOK, toolsResponse{
			Available: true,
			Message:   err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, toolsResponse{
		Available: true,
		Tools:     preflight.Tools,
		BinDir:    preflight.BinDir,
		Command:   preflight.InstallCommand,
		Missing:   preflight.MissingRequired,
		OK:        preflight.MissingRequired == 0,
	})
}
