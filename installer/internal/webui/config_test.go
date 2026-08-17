package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// legacyProject legt ein Projekt mit einer Konfiguration aus Modell 1 an und
// macht es zum Arbeitsverzeichnis: die Handler leiten ihr Projekt über
// project.Detect() daraus ab.
func legacyProject(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	writeFile(t, project.ConfigPath(root), `schema_version: 1
layout: fixed-project-k-playbook

paths:
  playbook: k-playbook
  tasks: k-playbook/tasks

project:
  repo_root: code
  vcs: git
`)

	before, err := os.Getwd()
	if err != nil {
		t.Fatalf("Arbeitsverzeichnis: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("nach %s wechseln: %v", root, err)
	}
	t.Cleanup(func() { os.Chdir(before) })
	return root
}

func getConfig(t *testing.T) configResponse {
	t.Helper()

	recorder := httptest.NewRecorder()
	configHandler(recorder, httptest.NewRequest(http.MethodGet, "/api/config", nil))

	var response configResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Antwort nicht lesbar: %v — %s", err, recorder.Body.String())
	}
	return response
}

func postReset(t *testing.T, body string) (int, configResponse) {
	t.Helper()

	recorder := httptest.NewRecorder()
	resetConfigHandler(recorder, httptest.NewRequest(http.MethodPost, "/api/config/reset", strings.NewReader(body)))

	var response configResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Antwort nicht lesbar: %v — %s", err, recorder.Body.String())
	}
	return recorder.Code, response
}

// Eine unbrauchbare Konfiguration darf nicht als eingerichtet durchgehen: der
// Block bliebe sonst verborgen, und alles Weitere wäre Anzeige ohne Grundlage.
func TestConfigHandlerMeldetAbgeloesteFassung(t *testing.T) {
	legacyProject(t)

	response := getConfig(t)
	if response.Installed {
		t.Error("Installed = true, obwohl die Fassung abgelöst ist")
	}
	if response.Schema != project.SchemaOutdated {
		t.Errorf("Schema = %q, erwartet %q", response.Schema, project.SchemaOutdated)
	}
	if response.SchemaVersion != "1" {
		t.Errorf("SchemaVersion = %q, erwartet %q", response.SchemaVersion, "1")
	}
	if response.LegacyModel == "" {
		t.Error("das abgelöste Modell wird nicht benannt")
	}
	// Die Lage des Repositorys ändert sich durch den Modellwechsel nicht.
	if response.Suggestion == nil || response.Suggestion.RepoRoot != "code" {
		t.Errorf("Suggestion = %+v, erwartet repo_root aus der alten Datei", response.Suggestion)
	}
}

func TestResetConfigHandlerLegtNeuAn(t *testing.T) {
	root := legacyProject(t)

	status, response := postReset(t, `{"repoRoot":"code"}`)
	if status != http.StatusOK {
		t.Fatalf("Status = %d, erwartet %d — %s", status, http.StatusOK, response.Message)
	}
	if !response.Installed {
		t.Error("Installed = false nach dem Zurücksetzen")
	}
	if response.BackupPath == "" {
		t.Error("die Sicherung wird nicht benannt")
	}
	if !strings.Contains(response.Message, filepath.Base(response.BackupPath)) {
		t.Errorf("die Meldung nennt die Sicherung nicht: %q", response.Message)
	}

	config, err := project.ReadConfig(root)
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}
	if err := project.CheckSchema(config); err != nil {
		t.Errorf("die neue Datei besteht die Prüfung nicht: %v", err)
	}
}

// Liegen Projektinhalte in der Installation, wird nichts geschrieben — und die
// Antwort nennt sie, damit der Umzug möglich ist.
func TestResetConfigHandlerBlocktBeiProjektinhalten(t *testing.T) {
	root := legacyProject(t)
	writeFile(t, filepath.Join(root, project.PlaybookDirName, "tasks", "001-etwas.md"), "# Aufgabe\n")

	status, response := postReset(t, `{"repoRoot":"code"}`)
	if status != http.StatusConflict {
		t.Fatalf("Status = %d, erwartet %d", status, http.StatusConflict)
	}
	if response.Installed {
		t.Error("Installed = true, obwohl nichts geschrieben wurde")
	}
	if len(response.LegacyContent) == 0 {
		t.Error("die Antwort nennt die Projektinhalte nicht")
	}

	config, err := project.ReadConfig(root)
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}
	if config.SchemaVersion != "1" {
		t.Errorf("SchemaVersion = %q, die alte Datei wurde angefasst", config.SchemaVersion)
	}
}
