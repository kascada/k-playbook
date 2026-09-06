package webui

import (
	"net/http"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// baseToolsResponse ist der Zustand der Basis-Werkzeuge für die Karte.
//
// Gespeist aus dem Kontextbefund (project.DetectBaseTools), nicht aus einem
// Skriptaufruf: der Befund ist ein PATH-Lookup je Werkzeug und kostet nichts,
// während der Security-Preflight einen Unterprozess startet.
//
// Was hier steht, ist bereits die Antwort — die Karte rechnet nichts nach. Sie
// leitet insbesondere nicht ab, welcher Weg root braucht: das entscheidet das
// Skript beim Laufen, je Eintrag und erst nach `command -v apt-get`. Läge die
// Rangfolge auch hier, stünde sie zweimal im Repo und liefe nach der ersten
// einseitigen Änderung auseinander. Dasselbe Muster wie bei
// fallbackToolInstallCommand(): nur der Aufruf, keine je Werkzeug ausgerechnete
// Entscheidung.
type baseToolsResponse struct {
	Available bool `json:"available"`
	// Present meldet, ob die Matrix gelesen werden konnte. Fehlt sie, zeigt die
	// Karte den Zustand statt zu raten.
	Present bool   `json:"present"`
	Matrix  string `json:"matrix"`
	// Missing sind die fehlenden Werkzeuge mit Rolle und Methodenspalte, so wie
	// die Matrix sie führt.
	Missing []project.BaseTool `json:"missing"`
	// Command ist der eine Skriptaufruf zum Kopieren. Ausgeführt wird er nie.
	Command string `json:"command"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

// baseToolsHandler antwortet mit dem Host-Befund zu den Basis-Werkzeugen.
// Rein lesend: es wird nichts installiert und kein Skript gestartet.
func baseToolsHandler(w http.ResponseWriter, r *http.Request) {
	environment := project.Detect()
	if !environment.Installed {
		writeJSON(w, http.StatusOK, baseToolsResponse{})
		return
	}
	writeJSON(w, http.StatusOK, buildBaseToolsResponse(environment.ProjectDir))
}

func buildBaseToolsResponse(projectDir string) baseToolsResponse {
	state := project.DetectBaseTools(projectDir)
	return baseToolsResponse{
		Available: true,
		Present:   state.Present,
		Matrix:    state.Matrix,
		Missing:   state.Missing,
		Command:   state.InstallCommand,
		OK:        state.OK,
		Message:   state.Error,
	}
}
