package webui

import (
	"bytes"
	"context"
	"net/http"
	"path/filepath"

	"github.com/kascada/k-playbook/installer/internal/inventory"
	"github.com/kascada/k-playbook/installer/internal/kephalaion"
	"github.com/kascada/k-playbook/installer/internal/project"
	"github.com/kascada/k-playbook/installer/internal/versionsources"
)

// Der Inventarbereich der Oberfläche: eigene Seite, eigene API, getrennt vom
// Docs-Bereich. Der zeigt die mitgelieferte Doku aus k-playbook/docs und
// kennt das Inventar nicht — so hat es Task 037 festgelegt, und das bleibt
// so. Das Inventar ist ein erzeugtes Dokument des Projekts in seiner
// Collection (inventory.DocRef), keine Datei der Installation, und bekommt
// deshalb seinen eigenen Weg.
//
// Die Handler hier rufen ausschließlich die Fachlogik aus internal/inventory
// auf. Kein zweiter Parser, keine Sonderbehandlung von Pfaden: der Ort des
// Inventars ist fest (inventory.DocName), der Anstoß ist inventory.Run, und
// die Vertrauensgrenze liegt dahinter in trust.go — abgelehnte Pfade stehen
// in der Antwort so, wie die Fachlogik sie meldet. Den Zugang zur Collection
// öffnet project.OpenInventoryStore, derselbe Weg wie beim Subkommando; ohne
// bereite Anbindung zeigt die Oberfläche einen sichtbaren Zustand
// (kephalaion_unavailable) und fällt nicht auf die Platte zurück.
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
	// HelmValues ist die Zahl der Einträge unter helm_values, abgelehnte
	// eingeschlossen — wie Sources.
	HelmValues int `json:"helmValues"`
	// Error ist gesetzt, wenn die Datei da, aber nicht lesbar oder von
	// unbekannter Fassung ist. Der Erhebungslauf bricht dann ab; die Anzeige
	// nennt den Grund.
	Error string `json:"error,omitempty"`
}

// inventoryResponse ist der Stand des Inventars: Status aus dem Frontmatter
// des Dokuments in der Collection — der einen Statusquelle — und der Zustand
// der Quellenkonfiguration. Ist die Collection nicht erreichbar, sagt das
// Status.Unavailable mit dem Grund in Status.Message.
type inventoryResponse struct {
	Available bool             `json:"available"`
	Status    inventory.Status `json:"status"`
	// DisplayPath ist der Ort des Inventars, wie die Oberfläche ihn nennt:
	// der keph:-Verweis.
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
	// Unevaluable ist die Zahl der Quellen im Zustand „nicht auswertbar" —
	// dieselbe Zahl wie in der Übersicht und im Frontmatter des Dokuments.
	Unevaluable int `json:"unevaluable"`
	Notes       int `json:"notes"`
}

// inventoryRunResponse ist die Antwort auf einen Anstoß. Ablehnungen,
// Ausschlüsse und Hinweise stehen vollständig darin, nicht nur als Zahl: eine
// Quelle, die konfiguriert ist und nicht gelesen werden konnte, ist eine Lücke
// im Inventar, und eine Lücke, die niemand sieht, ist schlimmer als ein Fehler.
type inventoryRunResponse struct {
	Available bool `json:"available"`
	// OK ist false, wenn der Lauf abgebrochen hat: Kephalaion ist nicht
	// bereit, die Quellenkonfiguration ist nicht lesbar oder von fremder
	// Fassung, oder das Schreiben in die Collection ist fehlgeschlagen
	// (etwa stale_revision). Der Grund steht in Message; ob das Dokument
	// dabei geschrieben wurde, sagt allein Outcome.Written — der Hub
	// schreibt ganz oder gar nicht, ein Fehler lässt den alten Stand stehen.
	OK         bool                  `json:"ok"`
	Message    string                `json:"message,omitempty"`
	Outcome    inventory.Outcome     `json:"outcome"`
	Summary    inventoryRunSummary   `json:"summary"`
	Rejections []inventory.Rejection `json:"rejections"`
	Exclusions []inventory.Exclusion `json:"exclusions"`
	// Unevaluable sind die Quellen im Zustand „nicht auswertbar", mit Datei
	// und Quellart; der Grund steht unter Notes.
	Unevaluable []inventory.SourceRead `json:"unevaluable"`
	Notes       []inventory.Note       `json:"notes"`
	// Status ist der Stand nach dem Lauf, aus dem Frontmatter gelesen — so,
	// wie GET /api/inventory ihn liefert.
	Status      inventory.Status `json:"status"`
	DisplayPath string           `json:"displayPath"`
}

// inventoryFileResponse ist das Inventar, fertig gerendert.
type inventoryFileResponse struct {
	Available bool `json:"available"`
	Present   bool `json:"present"`
	// Path ist der keph:-Verweis des Dokuments.
	Path    string `json:"path"`
	HTML    string `json:"html,omitempty"`
	Message string `json:"message,omitempty"`
}

// inventoryOptions sind die Optionen eines Laufs — dieselben Pfade, die auch
// das Subkommando zusammensammelt. Der Ort des Inventars ist keiner davon
// mehr: er steht in der Fachlogik (inventory.DocName).
func inventoryOptions(projectDir string) inventory.Options {
	localDir := project.LocalDir(projectDir)
	return inventory.Options{
		ProjectDir:  projectDir,
		SourcesFile: filepath.Join(localDir, project.VersionSourcesFileName),
	}
}

// inventoryDisplayPath ist der Pfad der Quellenkonfiguration, wie die
// Oberfläche ihn nennt: relativ zum Hauptverzeichnis.
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
	state.HelmValues = len(config.HelmValues)
	return state
}

// inventoryStatus ist der Stand aus der Collection. Ohne bereiten Zugang
// ist er „nicht erreichbar“ mit dem Grund aus der Anbindung — ein eigener
// Zustand, kein „fehlt“.
func inventoryStatus(ctx context.Context, projectDir string) inventory.Status {
	store, err := project.OpenInventoryStore(projectDir)
	if err != nil {
		return inventory.UnavailableStatus(err.Error())
	}
	return inventory.ReadStatus(ctx, store)
}

func inventoryState(ctx context.Context, projectDir string) inventoryResponse {
	options := inventoryOptions(projectDir)
	return inventoryResponse{
		Available:   true,
		Status:      inventoryStatus(ctx, projectDir),
		DisplayPath: inventory.DocRef,
		Sources:     inventorySources(projectDir, options.SourcesFile),
	}
}

func inventoryHandler(w http.ResponseWriter, r *http.Request) {
	environment := project.Detect()
	if !environment.Installed {
		writeJSON(w, http.StatusOK, inventoryResponse{})
		return
	}
	writeJSON(w, http.StatusOK, inventoryState(r.Context(), environment.ProjectDir))
}

// runInventoryHandler stößt die Erhebung an. Er ist der einzige schreibende
// Endpunkt des Bereichs und steht wie jeder POST hinter der Herkunftsprüfung.
//
// Geschrieben wird ausschließlich das Inventar in der Collection, und auch
// das nur, wenn die Erhebung sich inhaltlich vom Bestand unterscheidet — das
// ist die Byte-Stabilitätsregel des Vertrags, umgesetzt in inventory.Write,
// nicht hier.
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
		DisplayPath: inventory.DocRef,
		Outcome:     inventory.Outcome{Path: inventory.DocRef},
		Rejections:  []inventory.Rejection{},
		Exclusions:  []inventory.Exclusion{},
		Unevaluable: []inventory.SourceRead{},
		Notes:       []inventory.Note{},
	}

	store, err := project.OpenInventoryStore(environment.ProjectDir)
	if err != nil {
		// Kephalaion ist nicht bereit: Es wird nichts erhoben und nichts
		// geschrieben, und die Oberfläche sagt, warum.
		response.Message = "Erhebung abgebrochen: " + describeInventoryError(err)
		response.Status = inventory.UnavailableStatus(err.Error())
		writeJSON(w, http.StatusOK, response)
		return
	}

	result, outcome, err := inventory.Run(r.Context(), store, options)
	if err != nil {
		// Abgebrochen: Quellenkonfiguration nicht lesbar oder von fremder
		// Fassung, oder das Schreiben ist fehlgeschlagen. Was davon, sagt err;
		// ob geschrieben wurde, sagt outcome.Written. Der Stand kommt in jedem
		// Fall neu aus der Collection.
		response.Message = "Erhebung abgebrochen: " + describeInventoryError(err)
		response.Outcome = outcome
		response.Status = inventory.ReadStatus(r.Context(), store)
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
	if unevaluable := result.UnevaluableSources(); unevaluable != nil {
		response.Unevaluable = unevaluable
	}
	if result.Notes != nil {
		response.Notes = result.Notes
	}
	response.Status = inventory.ReadStatus(r.Context(), store)
	writeJSON(w, http.StatusOK, response)
}

// describeInventoryError nennt einen Fehler mit seinem Code, wenn er einen
// trägt — kephalaion_unavailable, stale_revision —, wie das Subkommando. Der
// Zugang übersetzt die Codes von Kephalaion in die der doc-Werkzeuge; ein
// Store ohne diese Übersetzung nennt den Code von Kephalaion selbst.
func describeInventoryError(err error) string {
	code := project.DocRefErrorCode(err)
	if code == "" {
		code = kephalaion.ErrorCode(err)
	}
	if code != "" {
		return err.Error() + " (" + code + ")"
	}
	return err.Error()
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
		Unevaluable:       len(result.UnevaluableSources()),
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

// inventoryFileHandler liefert das Inventar gerendert. Der Ort ist fest und
// kommt nicht aus der Anfrage: dieser Endpunkt liest genau das eine Dokument
// aus der Collection, und es gibt nichts zu prüfen, was ein Browser
// mitschicken könnte.
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

	response := inventoryFileResponse{Available: true, Path: inventory.DocRef}

	store, err := project.OpenInventoryStore(environment.ProjectDir)
	if err != nil {
		response.Message = "Kephalaion ist nicht bereit, das Inventar ist nicht erreichbar: " + describeInventoryError(err)
		writeJSON(w, http.StatusOK, response)
		return
	}
	read, err := store.Read(r.Context(), inventory.DocName)
	if err != nil {
		response.Message = "Inventar nicht lesbar: " + describeInventoryError(err)
		writeJSON(w, http.StatusOK, response)
		return
	}
	if read.Kind != kephalaion.ReadKindDocument {
		response.Message = "Das Inventar wurde noch nicht erhoben. „Aktualisieren“ erhebt es und legt das Dokument in der Collection an."
		writeJSON(w, http.StatusOK, response)
		return
	}
	response.Present = true

	// Der Frontmatter-Block gehört nicht in die Ansicht: seine Werte stehen
	// bereits als Status daneben. Abgetrennt wird er von der Fachlogik, die
	// das Dokumentformat kennt.
	var rendered bytes.Buffer
	if err := markdown.Convert(inventory.Body([]byte(*read.Content)), &rendered); err != nil {
		response.Message = "Markdown konnte nicht gerendert werden: " + err.Error()
		writeJSON(w, http.StatusOK, response)
		return
	}
	response.HTML = rendered.String()
	writeJSON(w, http.StatusOK, response)
}
