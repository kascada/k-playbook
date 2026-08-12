package webui

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/kascada/k-playbook/installer/internal/project"
	"github.com/kascada/k-playbook/installer/internal/review"
)

// choice ist ein wählbarer Eintrag für einen neuen Lauf.
//
// Werkzeuge und Reviews sehen in der Oberfläche gleich aus; sie unterscheiden
// sich nur in Kind und darin, ob sie verfügbar sind. Deshalb ein Typ.
type choice struct {
	Name string      `json:"name"`
	Kind review.Kind `json:"kind"`
	// Detail ist die Rolle des Werkzeugs oder die Herkunft des Rezepts.
	Detail string `json:"detail"`
	// Available: wählbar. Ein fehlendes Werkzeug wird gezeigt, aber nicht
	// angeboten — sonst startete ein Lauf mit etwas, das nicht da ist.
	Available bool `json:"available"`
	// Reason nennt den Grund, wenn Available false ist.
	Reason string `json:"reason"`
}

type reviewsResponse struct {
	Available bool             `json:"available"`
	Runs      []review.Summary `json:"runs"`
	Tools     []choice         `json:"tools"`
	Reviews   []choice         `json:"reviews"`
	Languages []string         `json:"languages"`
	// Today ist der Name, den ein jetzt angelegter Lauf bekäme.
	Today string `json:"today"`
	// Exists: für heute gibt es bereits einen Lauf.
	Exists  bool   `json:"exists"`
	Created string `json:"created"`
	Message string `json:"message"`
}

type createRunRequest struct {
	Entries []review.Entry `json:"entries"`
}

func reviewsHandler(w http.ResponseWriter, r *http.Request) {
	environment := project.Detect()
	if !environment.Installed {
		writeJSON(w, http.StatusOK, reviewsResponse{})
		return
	}
	writeJSON(w, http.StatusOK, reviewsState(environment, "", ""))
}

// createRunHandler legt den Lauf an. Mehr nicht: gestartet wird nichts, das ist
// ein eigener Schritt.
func createRunHandler(w http.ResponseWriter, r *http.Request) {
	environment := project.Detect()
	if !environment.Installed {
		writeJSON(w, http.StatusOK, reviewsResponse{})
		return
	}

	var request createRunRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, reviewsState(environment, "", "Anfrage nicht lesbar."))
		return
	}

	languages, _, err := project.ReadLanguages(environment.ProjectDir)
	if err != nil {
		writeJSON(w, http.StatusOK, reviewsState(environment, "", err.Error()))
		return
	}

	localDir := project.LocalDir(environment.ProjectDir)
	runDir, err := review.CreateRun(localDir, time.Now(), languages, request.Entries)
	if err != nil {
		// Kein Fehlerstatus für den vorhandenen Lauf: das ist ein normales
		// Ergebnis der Bedienung und keine kaputte Anfrage.
		writeJSON(w, http.StatusOK, reviewsState(environment, "", err.Error()))
		return
	}

	writeJSON(w, http.StatusOK, reviewsState(environment, project.DisplayPath(runDir), ""))
}

func reviewsState(environment project.Environment, created string, message string) reviewsResponse {
	response := reviewsResponse{
		Available: true,
		Created:   created,
		Message:   message,
		Today:     time.Now().Format(review.DateLayout),
	}

	localDir := project.LocalDir(environment.ProjectDir)
	runs, err := review.ListRuns(localDir)
	if err != nil && response.Message == "" {
		response.Message = err.Error()
	}
	response.Runs = runs
	for _, run := range runs {
		if run.Name == response.Today {
			response.Exists = true
			break
		}
	}

	languages, _, err := project.ReadLanguages(environment.ProjectDir)
	if err != nil && response.Message == "" {
		response.Message = err.Error()
	}
	response.Languages = languages
	response.Tools = toolChoices(environment.ProjectDir, languages)
	response.Reviews = reviewChoices(environment.ProjectDir)
	return response
}

// toolChoices sind die Werkzeuge, die für die gewählten Sprachen zuständig
// sind. Was nicht zuständig ist, fällt ganz weg; was fehlt, bleibt sichtbar,
// aber nicht wählbar.
func toolChoices(projectDir string, languages []string) []choice {
	preflight, err := project.CheckTools(projectDir, languages)
	if err != nil {
		return []choice{}
	}

	selected := map[string]bool{}
	for _, language := range languages {
		selected[language] = true
	}

	choices := []choice{}
	for _, tool := range preflight.Tools {
		if !toolApplies(tool.Languages, selected) {
			continue
		}
		item := choice{
			Name:      tool.Name,
			Kind:      review.KindTool,
			Detail:    tool.Role,
			Available: tool.Status == "ok",
		}
		if !item.Available {
			item.Reason = "nicht installiert"
		}
		choices = append(choices, item)
	}
	return choices
}

func toolApplies(languages string, selected map[string]bool) bool {
	for _, language := range strings.Split(languages, ",") {
		language = strings.TrimSpace(language)
		if language == "*" {
			return true
		}
		if selected[language] {
			return true
		}
	}
	return false
}

// reviewChoices sind die Rezepte aus dem aufgelösten Katalog. Abgeschaltete
// fehlen: sie wurden mit Absicht ausgeschaltet, die Datei sagt warum.
func reviewChoices(projectDir string) []choice {
	context, err := project.BuildContext(projectDir)
	if err != nil {
		return []choice{}
	}

	choices := []choice{}
	for _, entry := range context.Catalogs["reviews"] {
		if entry.Disabled {
			continue
		}
		choices = append(choices, choice{
			Name:      entry.Key,
			Kind:      review.KindAI,
			Detail:    originLabel(entry.Origin),
			Available: true,
		})
	}
	return choices
}

func originLabel(origin string) string {
	switch origin {
	case "dist":
		return "mitgeliefert"
	case "local":
		return "projekteigen"
	case "override":
		return "projekteigen, ersetzt mitgeliefert"
	default:
		return origin
	}
}
