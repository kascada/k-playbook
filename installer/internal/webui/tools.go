package webui

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// toolsResponse ist der Zustand der Security-Tools.
//
// Rein lesend, was die Installation angeht: installiert wird im Terminal, weil
// das den Host verändert und nicht das Projekt. Die Oberfläche zeigt dafür den
// fertigen Befehl. Die Sprachauswahl dagegen gehört dem Projekt und wird hier
// gesetzt.
type toolsResponse struct {
	Available bool           `json:"available"`
	Tools     []project.Tool `json:"tools"`
	BinDir    string         `json:"binDir"`
	Command   string         `json:"command"`
	// CommandOptional nimmt die optionalen mit. Beide kommen fertig aus dem
	// Preflight-Skript.
	CommandOptional string `json:"commandOptional"`
	Missing         int    `json:"missing"`
	// MissingOptional blockiert nichts, gehört aber gesagt.
	MissingOptional int    `json:"missingOptional"`
	OK              bool   `json:"ok"`
	Message         string `json:"message"`
	// Languages ist die aktuelle Auswahl, Available... die Liste, aus der
	// gewählt werden kann. Letztere kommt aus der Tool-Matrix selbst, damit sie
	// nicht ein zweites Mal gepflegt werden muss.
	Languages          []string `json:"languages"`
	AvailableLanguages []string `json:"availableLanguages"`
	// Configured meldet, ob project.languages in der Konfiguration steht. Ist es
	// das nicht, zeigt die Oberfläche die Vorauswahl als noch nicht getroffen.
	Configured bool `json:"configured"`
}

type languagesRequest struct {
	Languages []string `json:"languages"`
}

func toolsHandler(w http.ResponseWriter, r *http.Request) {
	environment := project.Detect()
	if !environment.Installed {
		writeJSON(w, http.StatusOK, toolsResponse{})
		return
	}

	languages, configured, err := project.ReadLanguages(environment.ProjectDir)
	if err != nil {
		writeJSON(w, http.StatusOK, toolsResponse{Available: true, Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, buildToolsResponse(environment.ProjectDir, languages, configured))
}

// setLanguagesHandler schreibt project.languages und antwortet mit dem neuen
// Tool-Zustand. Eine Antwort statt zwei Aufrufen: die Karte soll die Liste
// unmittelbar nach dem Umschalten zeigen.
func setLanguagesHandler(w http.ResponseWriter, r *http.Request) {
	environment := project.Detect()
	if !environment.Installed {
		writeJSON(w, http.StatusOK, toolsResponse{})
		return
	}

	var request languagesRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, toolsResponse{Available: true, Message: "Anfrage nicht lesbar."})
		return
	}

	if err := project.SetLanguages(environment.ProjectDir, request.Languages); err != nil {
		writeJSON(w, http.StatusOK, toolsResponse{Available: true, Message: err.Error()})
		return
	}

	languages, configured, err := project.ReadLanguages(environment.ProjectDir)
	if err != nil {
		writeJSON(w, http.StatusOK, toolsResponse{Available: true, Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, buildToolsResponse(environment.ProjectDir, languages, configured))
}

func buildToolsResponse(projectDir string, languages []string, configured bool) toolsResponse {
	preflight, err := project.CheckTools(projectDir, languages)
	if err != nil {
		return toolsResponse{
			Available:  true,
			Message:    err.Error(),
			Languages:  languages,
			Configured: configured,
		}
	}

	return toolsResponse{
		Available:          true,
		Tools:              preflight.Tools,
		BinDir:             preflight.BinDir,
		Command:            preflight.InstallCommand,
		CommandOptional:    preflight.InstallCommandOptional,
		Missing:            preflight.MissingRequired,
		MissingOptional:    preflight.MissingOptional,
		OK:                 preflight.MissingRequired == 0,
		Languages:          languages,
		AvailableLanguages: languagesFromMatrix(preflight.Tools),
		Configured:         configured,
	}
}

// languagesFromMatrix sammelt die wählbaren Sprachen aus der Tool-Matrix: alles,
// was ein Tool als Zuständigkeit nennt, außer dem sprachunabhängigen *. Damit
// bringt ein künftiges Tool seine Sprache von allein mit.
func languagesFromMatrix(tools []project.Tool) []string {
	seen := map[string]bool{}
	for _, tool := range tools {
		for _, language := range strings.Split(tool.Languages, ",") {
			language = strings.TrimSpace(language)
			if language == "" || language == "*" {
				continue
			}
			seen[language] = true
		}
	}

	languages := make([]string, 0, len(seen))
	for language := range seen {
		languages = append(languages, language)
	}
	sort.Strings(languages)
	return languages
}
