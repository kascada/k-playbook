package webui

import (
	"bytes"
	"net/http"
	"os"
	"path/filepath"

	"github.com/kascada/k-playbook/installer/internal/inventory"
	"github.com/kascada/k-playbook/installer/internal/project"
	"github.com/kascada/k-playbook/installer/internal/versionsources"
)

// Der Inventarbereich der Oberfläche: eigene Seite, eigene API, getrennt vom
// Docs-Bereich. Der zeigt die mitgelieferte Doku aus k-playbook/docs und
// kennt k-playbook-local/docs nicht — so hat es Task 037 festgelegt, und das
// bleibt so. Das Inventar ist eine Datei des Projekts, keine der
// Installation, und bekommt deshalb seinen eigenen Weg.
//
// Die Handler hier rufen ausschließlich die Fachlogik aus internal/inventory
// auf. Kein zweiter Parser, keine Sonderbehandlung von Pfaden: der Pfad der
// Inventardatei ist fest (inventory.FilePath), der Anstoß ist inventory.Run,
// und die Vertrauensgrenze liegt dahinter in trust.go — abgelehnte Pfade
// stehen in der Antwort so, wie die Fachlogik sie meldet.
//
// Die Quellenkonfiguration wird nur angezeigt, nicht gepflegt. Die
// Schreibregel des Vertrags verlangt ausdrückliche Bestätigung und rein
// ergänzendes Schreiben unter Erhalt von Kommentaren und Reihenfolge; eine
// Eingabemaske bräuchte dafür eigene Validierung und einen zweiten Schreibpfad
// neben dem Command. Es gibt deshalb keinen Endpunkt, der in
// version-sources.yaml schreibt.

// inventorySourcesState ist die Quellenkonfiguration, wie die Oberfläche sie
// zeigt: wo sie liegt, ob sie da ist, und was sie enthält — als Zahlen, nicht
// als Formular. Gelesen wird sie mit demselben Leser wie im Sammler und in
// `k-playbook context`.
type inventorySourcesState struct {
	Path        string `json:"path"`
	DisplayPath string `json:"displayPath"`
	Present     bool   `json:"present"`
	Roots       int    `json:"roots"`
	Sources     int    `json:"sources"`
	Exclude     int    `json:"exclude"`
	// Error ist gesetzt, wenn die Datei da, aber nicht lesbar oder von
	// unbekannter Fassung ist. Der Erhebungslauf bricht dann ab; die Anzeige
	// nennt den Grund.
	Error string `json:"error,omitempty"`
}

// inventoryResponse ist der Stand des Inventars: Status aus dem Frontmatter
// der Inventardatei — der einen Statusquelle — und der Zustand der
// Quellenkonfiguration.
type inventoryResponse struct {
	Available   bool                  `json:"available"`
	Status      inventory.Status      `json:"status"`
	DisplayPath string                `json:"displayPath"`
	Sources     inventorySourcesState `json:"sources"`
	Message     string                `json:"message,omitempty"`
}

// inventoryRunSummary sind die Zahlen eines Laufs, dieselben, die das
// Subkommando ausgibt.
type inventoryRunSummary struct {
	Sources           int `json:"sources"`
	ConfiguredSources int `json:"configuredSources"`
	Entries           int `json:"entries"`
	Deviations        int `json:"deviations"`
	Conflicting       int `json:"conflicting"`
	Rejected          int `json:"rejected"`
	Excluded          int `json:"excluded"`
	Notes             int `json:"notes"`
}

// inventoryRunResponse ist die Antwort auf einen Anstoß. Ablehnungen,
// Ausschlüsse und Hinweise stehen vollständig darin, nicht nur als Zahl: eine
// Quelle, die konfiguriert ist und nicht gelesen werden konnte, ist eine Lücke
// im Inventar, und eine Lücke, die niemand sieht, ist schlimmer als ein Fehler.
type inventoryRunResponse struct {
	Available bool `json:"available"`
	// OK ist false, wenn der Lauf abgebrochen hat — unlesbare oder fremd
	// versionierte Quellenkonfiguration. Dann steht der Grund in Message und
	// es wurde nichts geschrieben.
	OK         bool                  `json:"ok"`
	Message    string                `json:"message,omitempty"`
	Outcome    inventory.Outcome     `json:"outcome"`
	Summary    inventoryRunSummary   `json:"summary"`
	Rejections []inventory.Rejection `json:"rejections"`
	Exclusions []inventory.Exclusion `json:"exclusions"`
	Notes      []inventory.Note      `json:"notes"`
	// Status ist der Stand nach dem Lauf, aus dem Frontmatter gelesen — so,
	// wie GET /api/inventory ihn liefert.
	Status      inventory.Status `json:"status"`
	DisplayPath string           `json:"displayPath"`
}

// inventoryFileResponse ist die Inventardatei, fertig gerendert.
type inventoryFileResponse struct {
	Available bool   `json:"available"`
	Present   bool   `json:"present"`
	Path      string `json:"path"`
	HTML      string `json:"html,omitempty"`
	Message   string `json:"message,omitempty"`
}

// inventoryOptions sind die Optionen eines Laufs — dieselben drei Pfade, die
// auch das Subkommando zusammensammelt. Der Zielpfad kommt aus der Fachlogik,
// nicht aus einem zweiten Zusammensetzen hier.
func inventoryOptions(projectDir string) inventory.Options {
	localDir := project.LocalDir(projectDir)
	return inventory.Options{
		ProjectDir:    projectDir,
		SourcesFile:   filepath.Join(localDir, project.VersionSourcesFileName),
		InventoryFile: inventory.FilePath(localDir),
	}
}

// inventoryDisplayPath ist der Pfad der Inventardatei, wie die Oberfläche ihn
// nennt: relativ zum Hauptverzeichnis.
func inventoryDisplayPath(projectDir string, path string) string {
	if relative, err := filepath.Rel(projectDir, path); err == nil {
		return filepath.ToSlash(relative)
	}
	return project.DisplayPath(path)
}

// inventorySources liest den Zustand der Quellenkonfiguration für die
// Anzeige. Eine defekte Datei ist hier ein sichtbarer Zustand, kein Abbruch —
// dieselbe Trennung wie in `k-playbook context`: Auskunft geben ist nicht
// Erheben.
func inventorySources(projectDir string, path string) inventorySourcesState {
	config, err := versionsources.Read(path)
	state := inventorySourcesState{
		Path:        path,
		DisplayPath: inventoryDisplayPath(projectDir, path),
		Present:     config.Present,
	}
	if err != nil {
		state.Error = err.Error()
		return state
	}
	state.Roots = len(config.Roots)
	state.Sources = len(config.Sources)
	state.Exclude = len(config.Exclude)
	return state
}

func inventoryState(projectDir string) inventoryResponse {
	options := inventoryOptions(projectDir)
	return inventoryResponse{
		Available:   true,
		Status:      inventory.ReadStatus(options.InventoryFile),
		DisplayPath: inventoryDisplayPath(projectDir, options.InventoryFile),
		Sources:     inventorySources(projectDir, options.SourcesFile),
	}
}

func inventoryHandler(w http.ResponseWriter, r *http.Request) {
	environment := project.Detect()
	if !environment.Installed {
		writeJSON(w, http.StatusOK, inventoryResponse{})
		return
	}
	writeJSON(w, http.StatusOK, inventoryState(environment.ProjectDir))
}

// runInventoryHandler stößt die Erhebung an. Er ist der einzige schreibende
// Endpunkt des Bereichs und steht wie jeder POST hinter der Herkunftsprüfung.
//
// Geschrieben wird ausschließlich die Inventardatei, und auch die nur, wenn
// die Erhebung sich inhaltlich vom Bestand unterscheidet — das ist die
// Byte-Stabilitätsregel des Vertrags, umgesetzt in inventory.Write, nicht hier.
func runInventoryHandler(w http.ResponseWriter, r *http.Request) {
	environment := project.Detect()
	if !environment.Installed {
		writeJSON(w, http.StatusConflict, inventoryRunResponse{
			Message: "Keine " + project.ConfigFileName + " gefunden. Es gibt kein Projekt, dessen Inventar sich erheben ließe.",
		})
		return
	}

	options := inventoryOptions(environment.ProjectDir)
	response := inventoryRunResponse{
		Available:   true,
		DisplayPath: inventoryDisplayPath(environment.ProjectDir, options.InventoryFile),
		Rejections:  []inventory.Rejection{},
		Exclusions:  []inventory.Exclusion{},
		Notes:       []inventory.Note{},
	}

	result, outcome, err := inventory.Run(options)
	if err != nil {
		// Der abbrechende Fall und nur er: die Quellenkonfiguration ist nicht
		// lesbar oder von fremder Fassung. Es wurde nichts geschrieben; der
		// Stand bleibt der von vorher.
		response.Message = "Erhebung abgebrochen: " + err.Error()
		response.Outcome = outcome
		response.Status = inventory.ReadStatus(options.InventoryFile)
		writeJSON(w, http.StatusOK, response)
		return
	}

	response.OK = true
	response.Outcome = outcome
	response.Summary = summarizeInventoryRun(result)
	if result.Rejections != nil {
		response.Rejections = result.Rejections
	}
	if result.Exclusions != nil {
		response.Exclusions = result.Exclusions
	}
	if result.Notes != nil {
		response.Notes = result.Notes
	}
	response.Status = inventory.ReadStatus(options.InventoryFile)
	writeJSON(w, http.StatusOK, response)
}

// summarizeInventoryRun zählt, was das Subkommando in seiner Übersicht
// ausgibt. Gruppen, nicht Zeilen; die widersprüchlichen daneben, weil sie die
// Frage aufwerfen.
func summarizeInventoryRun(result inventory.Result) inventoryRunSummary {
	summary := inventoryRunSummary{
		Sources:           len(result.Sources),
		ConfiguredSources: result.ConfiguredSources,
		Entries:           len(result.Entries),
		Deviations:        len(result.Deviations),
		Rejected:          len(result.Rejections),
		Notes:             len(result.Notes),
	}
	for _, deviation := range result.Deviations {
		if deviation.Art == inventory.DeviationConflicting {
			summary.Conflicting++
		}
	}
	for _, exclusion := range result.Exclusions {
		summary.Excluded += exclusion.Skipped
	}
	return summary
}

// inventoryFileHandler liefert die Inventardatei gerendert. Der Pfad ist fest
// und kommt nicht aus der Anfrage: dieser Endpunkt liest genau eine Datei, und
// es gibt nichts zu prüfen, was ein Browser mitschicken könnte.
//
// Gerendert wird mit demselben Goldmark wie die mitgelieferte Doku — rohes
// HTML bleibt abgeschaltet. Geteilt wird nur der Renderer; der Docs-Bereich
// selbst bleibt unberührt.
func inventoryFileHandler(w http.ResponseWriter, r *http.Request) {
	environment := project.Detect()
	if !environment.Installed {
		writeJSON(w, http.StatusOK, inventoryFileResponse{})
		return
	}

	options := inventoryOptions(environment.ProjectDir)
	response := inventoryFileResponse{
		Available: true,
		Path:      inventoryDisplayPath(environment.ProjectDir, options.InventoryFile),
	}

	content, err := os.ReadFile(options.InventoryFile)
	if err != nil {
		if os.IsNotExist(err) {
			response.Message = "Das Inventar wurde noch nicht erhoben. „Aktualisieren“ erhebt es und legt die Datei an."
		} else {
			response.Message = "Inventardatei nicht lesbar: " + err.Error()
		}
		writeJSON(w, http.StatusOK, response)
		return
	}
	response.Present = true

	// Der Frontmatter-Block gehört nicht in die Ansicht: seine Werte stehen
	// bereits als Status daneben. Abgetrennt wird er von der Fachlogik, die
	// das Dateiformat kennt.
	var rendered bytes.Buffer
	if err := markdown.Convert(inventory.Body(content), &rendered); err != nil {
		response.Message = "Markdown konnte nicht gerendert werden: " + err.Error()
		writeJSON(w, http.StatusOK, response)
		return
	}
	response.HTML = rendered.String()
	writeJSON(w, http.StatusOK, response)
}
