package webui

import (
	"net/http"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// ghResponse ist der Befund zur GitHub CLI auf diesem Rechner.
//
// Geschrieben wird hier nichts. Eine Projektentscheidung über gh gibt es nicht
// mehr (Task 128): Ob gh genutzt wird, ergibt sich aus der GitHub-Zugangsdatei,
// ohne Dienst daraus, ob gh bereit ist. Installation und Anmeldung bleiben im
// Terminal: beides verändert den Host und nicht das Projekt, und `gh auth
// login` will ohnehin einen Browser. Die Oberfläche zeigt dafür den Befehl.
type ghResponse struct {
	Available bool       `json:"available"`
	Current   project.GH `json:"current"`
	Commands  ghCommands `json:"commands"`
	Message   string     `json:"message"`
}

// ghCommands sind die Befehle zum Kopieren, je nach Zustand.
type ghCommands struct {
	Install string `json:"install"`
	Login   string `json:"login"`
	Status  string `json:"status"`
}

// ghHandler ist GET /api/gh: der Befund für die Setup-Karte „GitHub CLI".
func ghHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, ghState())
}

func ghState() ghResponse {
	environment := project.Detect()
	response := ghResponse{
		Available: environment.Installed,
		Commands:  ghCommandSet(),
	}
	if !environment.Installed {
		return response
	}
	response.Current = project.DetectGH()
	return response
}

// ghCommandSet nennt für jeden Zustand den passenden Befehl. Der
// Installationsbefehl hängt am Paketmanager des Rechners; statt zu raten,
// verweist er auf die Anleitung.
func ghCommandSet() ghCommands {
	return ghCommands{
		Install: "https://github.com/cli/cli#installation",
		Login:   "gh auth login --hostname " + project.GHHost,
		Status:  "gh auth status",
	}
}
