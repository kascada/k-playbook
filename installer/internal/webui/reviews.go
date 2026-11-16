package webui

import (
	"net/http"

	"github.com/kascada/k-playbook/installer/internal/project"
	"github.com/kascada/k-playbook/installer/internal/review"
)

// reviewsResponse ist die Antwort der Seite Reviews. Solange die Review-Wege
// pausieren (review.Paused), trägt sie keine Läufe, sondern die Meldung der
// Sperre: Paused ist wahr und Message der eine Satz mit dem Grund. Gestartet
// wird ein Lauf ohnehin nicht über die Oberfläche — dafür sind /k-audit und
// /k-review im Assistenten zuständig, beide über den MCP-Server.
type reviewsResponse struct {
	Available bool   `json:"available"`
	Paused    bool   `json:"paused"`
	Message   string `json:"message"`
}

// reviewsHandler liest nichts: Die Sperre gilt auch für die Oberfläche, und
// einen Ort der Ergebnisse kennt das Programm bis zur Entscheidung nicht.
func reviewsHandler(w http.ResponseWriter, r *http.Request) {
	environment := project.Detect()
	if !environment.Installed {
		writeJSON(w, http.StatusOK, reviewsResponse{})
		return
	}
	writeJSON(w, http.StatusOK, reviewsResponse{Available: true, Paused: true, Message: review.PausedMessage})
}
