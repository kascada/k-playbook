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
// das den Host veraendert und nicht das Projekt. Die Oberflaeche zeigt dafuer den
// fertigen Befehl. Die Sprachauswahl dagegen gehoert dem Projekt und wird hier
// gesetzt.
type toolsResponse struct {
	Available bool           `json:"available"`
	Tools     []project.Tool `json:"tools"`
	BinDir    string         `json:"binDir"`
	Command   string         `json:"command"`
	Missing   int            `json:"missing"`
	OK        bool           `json:"ok"`
	Message   string         `json:"message"`
	// Languages ist die aktuelle Auswahl, Available... die Liste, aus der
	// gewaehlt werden kann. Letztere kommt aus der Tool-Matrix selbst, damit sie
	// nicht ein zweites Mal gepflegt werden muss.
	Languages          []string `json:"languages"`
	AvailableLanguages []string `json:"availableLanguages"`
	// Configured meldet, ob project.languages in der Konfiguration steht. Ist es
	// das nicht, zeigt die Oberflaeche die Vorauswahl als noch nicht getroffen.
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
		Command:            installCommand(preflight.InstallCommand, languages),
		Missing:            preflight.MissingRequired,
		OK:                 preflight.MissingRequired == 0,
		Languages:          languages,
		AvailableLanguages: languagesFromMatrix(preflight.Tools),
		Configured:         configured,
	}
}

// installCommand ergaenzt den Befehl aus dem Skript um die Sprachauswahl, damit
// der kopierte Befehl genau das installiert, was die Karte als fehlend zeigt.
func installCommand(command string, languages []string) string {
	joined := strings.Join(languages, ",")
	if command == "" || joined == "" {
		return command
	}
	// Vor --install, damit die Sprachen auch dann gelten, wenn jemand das Ziel
	// dahinter noch von Hand aendert.
	if index := strings.Index(command, " --install "); index >= 0 {
		return command[:index] + " --languages " + joined + command[index:]
	}
	return command + " --languages " + joined
}

// languagesFromMatrix sammelt die waehlbaren Sprachen aus der Tool-Matrix: alles,
// was ein Tool als Zustaendigkeit nennt, ausser dem sprachunabhaengigen *. Damit
// bringt ein kuenftiges Tool seine Sprache von allein mit.
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
