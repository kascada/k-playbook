package webui

import (
	"net/http"

	"github.com/kascada/k-playbook/installer/internal/health"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// Die Umgebungsprüfung in der Oberfläche (Task 098): GET /api/environment
// liefert den Report von health.Check — dieselbe Liste, die `k-playbook health
// --json` ausgibt, k_playbook_health anbietet und das Briefing als Abschnitt
// health trägt. Die Statusseite zeigt sie in der Karte „Umgebung“: ohne Problem
// eine Zeile, sonst je Eintrag Stufe, Bereich, Satz und Abhilfe.
//
// Der Pfad heißt bewusst nicht /api/health: Der ist das Lebenszeichen des
// Fensters (healthHandler in server.go) und Teil des guiproc-Vertrags, den
// `k-playbook gui status`, der Neustart und jedes offene Fenster alle paar
// Sekunden rufen. Die Umgebungsprüfung fragt den Kephalaion-Node und gehört
// nicht in einen Heartbeat.
//
// Rein lesend: Es wird nichts repariert, und kein Eintrag nennt einen Token
// oder einen Wert aus einer Datei — das stellt das Paket health sicher, die
// Oberfläche reicht die Einträge nur durch. Werte aus der Anfrage gehen nicht
// in die Prüfung ein.

// environmentResponse ist die Antwort von GET /api/environment. Der Report
// steht unter health, wie beim MCP-Werkzeug, und nicht eingebettet: sein ok
// (keine Fehler) ist ein anderes als available (es gibt ein Projekt, die
// Prüfung lief). Ohne Projektkonfiguration fehlt er.
type environmentResponse struct {
	Available bool           `json:"available"`
	Health    *health.Report `json:"health,omitempty"`
	Message   string         `json:"message,omitempty"`
}

// environmentHandler antwortet mit dem Report der Umgebungsprüfung. Kein
// Zwischenspeicher: Die Statusseite fragt einmal beim Öffnen, und was die
// Kephalaion-Quelle am Node fragt, ist derselbe whoami-Aufruf wie bei
// /api/kephalaion.
func environmentHandler(w http.ResponseWriter, r *http.Request) {
	environment := project.Detect()
	if !environment.Installed {
		writeJSON(w, http.StatusOK, environmentResponse{Message: "Keine " + project.ConfigFileName + " gefunden."})
		return
	}
	report := health.Check(r.Context(), environment.ProjectDir)
	writeJSON(w, http.StatusOK, environmentResponse{Available: true, Health: &report})
}
