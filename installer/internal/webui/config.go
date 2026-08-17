package webui

import (
	"encoding/json"
	"net/http"
	"path/filepath"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// configResponse beschreibt den Konfigurationszustand für die Oberfläche.
// Ist noch nichts angelegt, trägt Suggestion die Vorbelegung des Formulars.
type configResponse struct {
	// Installed meldet, ob mit diesem Projekt gearbeitet werden kann. Eine
	// Konfiguration aus einem abgelösten Modell zählt nicht dazu: sie ist da,
	// aber unbrauchbar, und alles Weitere bliebe Anzeige ohne Grundlage.
	Installed   bool                `json:"installed"`
	ProjectDir  string              `json:"projectDir"`
	ConfigPath  string              `json:"configPath"`
	PlaybookDir string              `json:"playbookDir"`
	RepoRoot    string              `json:"repoRoot"`
	VCS         string              `json:"vcs"`
	Suggestion  *project.Suggestion `json:"suggestion,omitempty"`
	// Schema ist die Einordnung der gefundenen schema_version. Die Oberfläche
	// braucht den Fall, nicht nur den Fehlertext: nur bei einer zu alten Datei
	// steht das Zurücksetzen zur Wahl.
	Schema        project.SchemaStatus `json:"schema"`
	SchemaVersion string               `json:"schemaVersion"`
	// LegacyModel beschreibt das abgelöste Modell, zu dem die Datei gehört.
	LegacyModel string `json:"legacyModel,omitempty"`
	// LegacyContent sind Projektinhalte im Installationsverzeichnis. Solange
	// welche daliegen, wird nicht zurückgesetzt.
	LegacyContent []string `json:"legacyContent,omitempty"`
	// BackupPath ist die weggesicherte alte Datei, nach dem Zurücksetzen.
	BackupPath string `json:"backupPath,omitempty"`
	Message    string `json:"message"`
}

// createConfigRequest ist die Auswahl des Nutzers aus dem Formular. Dieselben
// Felder gelten beim Zurücksetzen: dort wird die Datei neu angelegt, und der
// Nutzer soll denselben Blick darauf haben wie beim ersten Mal.
type createConfigRequest struct {
	ProjectDir string `json:"projectDir"`
	RepoRoot   string `json:"repoRoot"`
}

func configHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, configState(""))
}

// createConfigHandler legt die K-PLAYBOOK.yaml an dem vom Nutzer bestätigten
// Ort an.
func createConfigHandler(w http.ResponseWriter, r *http.Request) {
	var request createConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, configResponse{Message: "Anfrage nicht lesbar: " + err.Error()})
		return
	}

	if err := project.CreateConfig(request.ProjectDir, request.RepoRoot); err != nil {
		writeJSON(w, http.StatusConflict, configState("Nicht angelegt: "+err.Error()))
		return
	}

	writeJSON(w, http.StatusOK, configState(project.ConfigFileName+" angelegt."))
}

// resetConfigHandler sichert eine Konfiguration aus einem abgelösten Modell
// weg und legt eine frische an.
//
// Getrennt von createConfigHandler, obwohl beide dieselbe Datei schreiben: das
// Anlegen darf nie über etwas Vorhandenes gehen, und diese Grenze soll nicht
// von einem Flag im Anfragekörper abhängen.
func resetConfigHandler(w http.ResponseWriter, r *http.Request) {
	var request createConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, configResponse{Message: "Anfrage nicht lesbar: " + err.Error()})
		return
	}

	environment := project.Detect()
	if !environment.Installed {
		writeJSON(w, http.StatusConflict, configState("Keine "+project.ConfigFileName+" gefunden."))
		return
	}

	result, err := project.ResetConfig(environment.ProjectDir, request.RepoRoot)
	if err != nil {
		writeJSON(w, http.StatusConflict, configState("Nicht zurückgesetzt: "+err.Error()))
		return
	}

	state := configState(project.ConfigFileName + " neu angelegt, die alte Datei liegt als " +
		filepath.Base(result.BackupPath) + " daneben.")
	state.BackupPath = result.BackupPath
	writeJSON(w, http.StatusOK, state)
}

// configState ermittelt den aktuellen Zustand. Gesucht wird ab dem
// Arbeitsverzeichnis aufwärts; erst wenn nichts gefunden wird, greift der
// aus dem Ort des Binaries abgeleitete Vorschlag.
func configState(message string) configResponse {
	environment := project.Detect()
	response := configResponse{
		Installed: environment.Installed,
		Message:   message,
	}

	if !environment.Installed {
		suggestion := project.Suggest()
		response.Suggestion = &suggestion
		if response.Message == "" {
			response.Message = "Noch keine " + project.ConfigFileName +
				". Ort bestätigen, dann wird sie angelegt."
		}
		return response
	}

	response.ProjectDir = environment.ProjectDir
	response.ConfigPath = project.ConfigPath(environment.ProjectDir)
	response.PlaybookDir = environment.PlaybookDir

	config, err := project.ReadConfig(environment.ProjectDir)
	if err != nil {
		response.Message = project.ConfigFileName + " nicht lesbar: " + err.Error()
		return response
	}
	// Anders als bei `context` wird hier nicht abgebrochen: die Oberfläche
	// soll den Zustand zeigen können, gerade wenn etwas nicht stimmt.
	response.SchemaVersion = config.SchemaVersion
	response.Schema = project.SchemaState(config)
	if err := project.CheckSchema(config); err != nil && response.Message == "" {
		response.Message = err.Error()
	}
	if response.Schema.Resettable() {
		// Solange die Datei nicht passt, ist das Projekt nicht benutzbar — der
		// Block bleibt stehen und führt zum Zurücksetzen, statt sich wie nach
		// einer gelungenen Einrichtung auszublenden.
		response.Installed = false
		response.LegacyModel = project.LegacyModels[config.SchemaVersion]
		response.LegacyContent = project.LegacyContent(environment.ProjectDir)
		// Das Formular soll nicht bei "." anfangen, wenn der alte Wert woanders
		// hinzeigt: die Lage des Repositorys ändert sich durch den Modellwechsel
		// nicht.
		suggestion := project.Suggestion{ProjectDir: environment.ProjectDir, RepoRoot: config.RepoRoot}
		if suggestion.RepoRoot == "" {
			suggestion.RepoRoot = "."
		}
		response.Suggestion = &suggestion
	}
	response.RepoRoot = project.RepoRootDir(environment.ProjectDir, config)
	response.VCS = config.VCS
	return response
}
