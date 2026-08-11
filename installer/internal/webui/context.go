package webui

import (
	"net/http"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// contextResponse ist der aufgeloeste Arbeitsstand, wie ihn auch das
// Unterkommando `context` ausgibt.
//
// Rein lesend und nur auf Anforderung: die Oberflaeche ruft ihn erst ab, wenn
// der Block aufgeklappt wird.
type contextResponse struct {
	Available bool `json:"available"`
	// Context ist der unveraenderte Arbeitsstand, damit die Antwort dasselbe
	// bedeutet wie die des Unterkommandos.
	Context *project.Context `json:"context,omitempty"`
	// Display traegt dieselben Pfade in Anzeigeform. Die Kuerzung auf ~ braucht
	// das Home-Verzeichnis und kann deshalb nur hier passieren.
	Display *contextDisplay `json:"display,omitempty"`
	Message string          `json:"message"`
}

type contextDisplay struct {
	ProjectDir string `json:"projectDir"`
	RepoRoot   string `json:"repoRoot"`
	Config     string `json:"config"`
	Playbook   string `json:"playbook"`
	Local      string `json:"local"`
}

func contextHandler(w http.ResponseWriter, r *http.Request) {
	environment := project.Detect()
	if !environment.Installed {
		writeJSON(w, http.StatusOK, contextResponse{})
		return
	}

	built, err := project.BuildContext(environment.ProjectDir)
	if err != nil {
		// BuildContext bricht bei unbekannter Schema-Fassung ab. Das ist keine
		// Stoerung der Oberflaeche, sondern der Befund selbst.
		writeJSON(w, http.StatusOK, contextResponse{Available: true, Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, contextResponse{
		Available: true,
		Context:   &built,
		Display: &contextDisplay{
			ProjectDir: project.DisplayPath(built.Project.Dir),
			RepoRoot:   project.DisplayPath(built.Project.RepoRoot),
			Config:     project.DisplayPath(built.Project.Config),
			Playbook:   project.DisplayPath(built.Playbook.Dir),
			Local:      project.DisplayPath(built.Local.Dir),
		},
	})
}
