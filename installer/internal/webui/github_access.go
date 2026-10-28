package webui

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/githubaccess"
)

// Die Seite „GitHub-Zugänge“ (/github-access) zeigt die persönliche
// GitHub-Zugangsdatei nach dem Vertrag docs/github-access.md, Abschnitt
// Interface page: den Pfad, den Stand von check (Probleme oder eine Zeile),
// je Dienst einen Block mit Repository und den Zugängen in Vorrangreihenfolge
// samt Verfügbarkeit, und auf Knopfdruck den Test gegen GitHub. Sie steht
// neben den Projektseiten, handelt aber von der Person: die Datei gilt für
// jedes Projekt dieses Rechners. Nichts auf der Seite schreibt die Datei;
// bearbeitet wird im Editor, die Seite sagt, wo sie liegt.
//
// Zwei Endpunkte. GET /api/github-access ist das Objekt von `check --json`
// (file, created, services mit state und reason, problems), dazu available
// und message wie bei jedem GET der Oberfläche, wenn die Datei gar nicht
// lesbar ist. Er ist ohne Netz: Lesen, Vorlage anlegen, Verfügbarkeit
// beurteilen (gh-Konfiguration, Dateien, ssh -G). POST /api/github-access/test
// mit optionalem service ist das Objekt von `test --json` — der einzige
// Endpunkt dieser Seite mit Netz, und er läuft nur auf den Knopf hin: weder
// der Seitenaufbau noch die Statusseite noch das Menü rufen ihn. Die Seite
// ruft „Alle prüfen“ als einen POST je Dienst, damit sich die Blöcke einzeln
// füllen und kein Aufruf die Summe aller Fristen trägt; der Endpunkt ohne
// service bleibt nach Vertrag bestehen. Eine eigene Frist setzt der Handler
// nicht: Jede Anfrage und jeder Kindprozess des Tests hat zehn Sekunden, und
// ein Dienst mit mehreren Zugängen braucht ihre Summe — eine Server-Frist
// darüber schnitte genau den Lauf ab, den der Knopf anstößt.
//
// Kein Token verlässt den Prozess: Das Paket githubaccess hält es im Header
// seines HTTP-Clients und redigiert jede Meldung; die Antworten hier reichen
// Report und Probleme unverändert durch. Der POST nimmt einen Dienstnamen aus
// der Anfrage und vergleicht ihn nur mit den Namen aus der Datei; in keinen
// Pfad und keine Kommandozeile fließt ein Wert aus der Anfrage.

// newGitHubAccessTester baut den Tester: die echte API, zehn Sekunden je
// Aufruf. Die Naht für die Handler-Tests, die einen httptest.Server einsetzen.
var newGitHubAccessTester = githubaccess.NewTester

// githubAccessResponse ist die Antwort von GET /api/github-access: das Objekt
// von check --json, dazu available und message. available ist falsch, wenn
// der Ort der Datei unbekannt ist oder die Datei als Ganzes nicht lesbar
// (ein Problem mit Pfad file); problems trägt dann trotzdem, was der Leser
// sagt.
type githubAccessResponse struct {
	Available bool                   `json:"available"`
	File      string                 `json:"file,omitempty"`
	Created   bool                   `json:"created"`
	Services  githubaccess.Services  `json:"services"`
	Problems  []githubaccess.Problem `json:"problems"`
	Message   string                 `json:"message,omitempty"`
}

// githubAccessHandler liest die Datei und beurteilt die Verfügbarkeit. GET
// ohne Werte aus der Anfrage, ohne Netz.
func githubAccessHandler(w http.ResponseWriter, r *http.Request) {
	path, err := githubaccess.Path()
	if err != nil {
		writeJSON(w, http.StatusOK, githubAccessResponse{
			Services: githubaccess.Services{}, Problems: []githubaccess.Problem{},
			Message: "Der Ort der GitHub-Zugangsdatei ist unbekannt: " + err.Error() + ". HOME oder XDG_CONFIG_HOME setzen oder den Pfad in " + githubaccess.EnvConfig + " nennen.",
		})
		return
	}
	result := githubaccess.Load(path)
	response := githubAccessResponse{Available: true, File: result.File, Created: result.Created, Services: result.Services, Problems: result.Problems}
	if response.Services == nil {
		response.Services = githubaccess.Services{}
	}
	for _, problem := range result.Problems {
		if problem.Level == githubaccess.LevelError && problem.Path == githubaccess.PathFile {
			response.Available = false
			response.Message = problem.Message + " " + problem.Hint
			break
		}
	}
	if !result.HasErrors() {
		githubaccess.Judge(response.Services)
	}
	writeJSON(w, http.StatusOK, response)
}

// maxGitHubAccessRequest begrenzt den Rumpf des POST: ein Dienstname.
const maxGitHubAccessRequest = 4 << 10

type githubAccessTestRequest struct {
	Service string `json:"service"`
}

// githubAccessTestError ist die Fehlerantwort des POST: ein Satz, bei einem
// unbekannten Dienst die bekannten, bei einer defekten Datei ihre Probleme.
// Testergebnisse trägt sie nie.
type githubAccessTestError struct {
	Message  string                 `json:"message"`
	Known    []string               `json:"known,omitempty"`
	Problems []githubaccess.Problem `json:"problems,omitempty"`
}

// githubAccessTestHandler testet alle Dienste oder den genannten gegen
// GitHub und antwortet mit dem Report von test --json. POST hinter
// sameOrigin; der Dienstname aus der Anfrage wird nur mit den Namen der
// Datei verglichen. Bricht der Browser ab, antwortet der Handler nicht mehr.
func githubAccessTestHandler(w http.ResponseWriter, r *http.Request) {
	var request githubAccessTestRequest
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxGitHubAccessRequest))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, githubAccessTestError{Message: "Anfrage nicht lesbar."})
		return
	}
	if len(strings.TrimSpace(string(body))) > 0 {
		if err := json.Unmarshal(body, &request); err != nil {
			writeJSON(w, http.StatusBadRequest, githubAccessTestError{Message: "Anfrage nicht lesbar: erwartet wird ein JSON-Objekt mit optionalem service."})
			return
		}
	}

	path, err := githubaccess.Path()
	if err != nil {
		writeJSON(w, http.StatusConflict, githubAccessTestError{Message: "Der Ort der GitHub-Zugangsdatei ist unbekannt: " + err.Error() + "."})
		return
	}
	result := githubaccess.Load(path)
	if result.HasErrors() {
		writeJSON(w, http.StatusConflict, githubAccessTestError{
			Message:  "Die Zugangsdatei " + path + " hat Probleme; nichts getestet.",
			Problems: result.Problems,
		})
		return
	}
	services := result.Services
	if name := strings.TrimSpace(request.Service); name != "" {
		service, ok := services.Lookup(name)
		if !ok {
			writeJSON(w, http.StatusBadRequest, githubAccessTestError{
				Message: "Unbekannter Dienst " + name + " in " + path + ".",
				Known:   services.Names(),
			})
			return
		}
		services = githubaccess.Services{service}
	}

	report := newGitHubAccessTester().Test(r.Context(), path, services)
	if errors.Is(r.Context().Err(), context.Canceled) {
		// Der Browser hat die Anfrage abgebrochen; niemand hört mehr zu.
		return
	}
	writeJSON(w, http.StatusOK, report)
}
