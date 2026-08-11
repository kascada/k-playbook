package webui

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// updateResponse ist der Zustand der Aktualisierung.
type updateResponse struct {
	Available bool   `json:"available"`
	Branch    string `json:"branch"`
	Local     string `json:"local"`
	Remote    string `json:"remote"`
	Output    string `json:"output"`
	// RestartRequired: die Binaries wurden ersetzt, der laufende Prozess
	// arbeitet aber weiter mit dem alten Code.
	RestartRequired bool `json:"restartRequired"`
	// Links nennt, was das Update an der Registrierung von Commands und Skills
	// geaendert hat.
	Links   project.LinkChanges `json:"links"`
	Message string              `json:"message"`
}

// updateCheckHandler prueft den Remote-Stand. Rein lesend.
func updateCheckHandler(w http.ResponseWriter, r *http.Request) {
	environment := project.Detect()
	if !environment.Installed {
		writeJSON(w, http.StatusOK, updateResponse{})
		return
	}

	status, err := project.CheckUpdate(environment.ProjectDir)
	if err != nil {
		writeJSON(w, http.StatusOK, updateResponse{Message: "Pruefung fehlgeschlagen: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, updateResponse{
		Available: status.Available,
		Branch:    status.Branch,
		Local:     shortCommit(status.Local),
		Remote:    shortCommit(status.Remote),
		Message:   status.Message,
	})
}

// applyUpdateHandler holt den neuen Stand.
func applyUpdateHandler(w http.ResponseWriter, r *http.Request) {
	environment := project.Detect()
	if !environment.Installed {
		writeJSON(w, http.StatusConflict, updateResponse{
			Message: "Keine " + project.ConfigFileName + " gefunden.",
		})
		return
	}

	result, err := project.Update(environment.ProjectDir)
	if err != nil {
		writeJSON(w, http.StatusConflict, updateResponse{
			Output:  result.Output,
			Message: err.Error(),
		})
		return
	}

	response := updateResponse{
		Output:          result.Output,
		RestartRequired: result.BinaryChanged,
		Message:         "Aktualisiert.",
	}
	if result.BinaryChanged {
		response.Message = "Aktualisiert. Das Programm wurde ersetzt und laeuft bis zum Neustart mit dem bisherigen Stand."
	}
	response.Links, response.Message = relinkAfterUpdate(environment.ProjectDir, response.Message)

	// Nach dem Pull erneut pruefen, damit der Button den neuen Zustand zeigt.
	if status, err := project.CheckUpdate(environment.ProjectDir); err == nil {
		response.Available = status.Available
		response.Branch = status.Branch
		response.Local = shortCommit(status.Local)
		response.Remote = shortCommit(status.Remote)
	}
	writeJSON(w, http.StatusOK, response)
}

// relinkAfterUpdate zieht die Assistenten-Verlinkung auf den neuen Stand nach
// und meldet, was sich dabei geaendert hat.
//
// Das gehoert zum Update, nicht in einen zweiten Schritt: seit Commands und
// Skills einzeln verlinkt werden, kommt ein neu mitgelieferter Command nicht
// mehr von selbst an. Ein Update, das den Katalog aendert, ihn aber nicht
// registriert, waere halb erledigt — und zwar unsichtbar.
//
// Ein Fehler dabei laesst das Update selbst gueltig: der Pull ist durch, und
// die Verlinkung kann ueber die Assistenten-Karte nachgeholt werden.
func relinkAfterUpdate(projectDir string, message string) (project.LinkChanges, string) {
	changes := project.PendingLinkChanges(project.CheckLinks(projectDir))

	if _, err := project.ApplyLinks(projectDir); err != nil {
		return changes, message + " Die Verlinkung konnte nicht nachgezogen werden: " + err.Error()
	}
	if changes.Empty() {
		return changes, message
	}
	return changes, message + " " + describeLinkChanges(changes)
}

// describeLinkChanges formuliert die Bilanz als Satz.
func describeLinkChanges(changes project.LinkChanges) string {
	parts := []string{}
	for _, part := range []struct {
		names []string
		label string
	}{
		{changes.Added, "dazugekommen"},
		{changes.Removed, "entfernt"},
		{changes.Repointed, "auf eine andere Quelle umgesetzt"},
	} {
		if len(part.names) > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", len(part.names), part.label))
		}
	}
	return "Verlinkung nachgezogen: " + strings.Join(parts, ", ") + "."
}

// shortCommit kuerzt einen Commit-Hash auf die uebliche Anzeigelaenge.
func shortCommit(hash string) string {
	if len(hash) > 7 {
		return hash[:7]
	}
	return hash
}
