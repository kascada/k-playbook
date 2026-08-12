package webui

import (
	"net/http"

	"github.com/kascada/k-playbook/installer/internal/hostinstall"
)

// hostPathResponse ist der Zustand der host-weiten Aufrufbarkeit.
//
// Es gibt dazu keinen POST: das Shell-Profil gehört dem Nutzer, und ein
// Programm, das ungefragt darin schreibt, wäre schwerer zu durchschauen als
// eine Zeile zum Kopieren. Die Oberfläche zeigt den Zustand, gehandelt wird im
// Terminal.
type hostPathResponse struct {
	hostinstall.PathStatus
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

// hostPathHandler prüft, ohne etwas zu verändern.
func hostPathHandler(w http.ResponseWriter, r *http.Request) {
	status := hostinstall.CheckPath()
	response := hostPathResponse{PathStatus: status, OK: status.OK()}

	switch {
	case status.Dir == "":
		response.Message = "Das Home-Verzeichnis ließ sich nicht bestimmen."
	case !status.Linked:
		response.Message = "Der Symlink fehlt noch. Er entsteht beim nächsten Start der Oberfläche."
	case status.InPath:
		response.Message = "k-playbook ist aus jedem Verzeichnis aufrufbar."
	default:
		response.Message = "Der Symlink steht, aber das Verzeichnis liegt nicht im PATH. " +
			"Diese Zeile ins Shell-Profil (~/.bashrc oder ~/.zshrc), danach eine neue Shell öffnen:"
	}

	writeJSON(w, http.StatusOK, response)
}
