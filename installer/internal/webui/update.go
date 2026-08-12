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
	// geändert hat.
	Links project.LinkChanges `json:"links"`
	// Cleanliness ist der lokale Zustand der Installation. Er wird bei jeder
	// Prüfung mitgeliefert, auch ohne anstehendes Update: die Verschmutzung
	// entsteht unabhängig davon, und wer nie aktualisiert, bekäme sie sonst
	// nie zu sehen.
	Cleanliness project.Cleanliness `json:"cleanliness"`
	Message     string              `json:"message"`
}

// updateCheckHandler prüft den Remote-Stand. Rein lesend.
func updateCheckHandler(w http.ResponseWriter, r *http.Request) {
	environment := project.Detect()
	if !environment.Installed {
		writeJSON(w, http.StatusOK, updateResponse{})
		return
	}

	status, err := project.CheckUpdate(environment.ProjectDir)
	if err != nil {
		writeJSON(w, http.StatusOK, updateResponse{Message: "Prüfung fehlgeschlagen: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, updateResponse{
		Available:   status.Available,
		Branch:      status.Branch,
		Local:       shortCommit(status.Local),
		Remote:      shortCommit(status.Remote),
		Cleanliness: status.Cleanliness,
		Message:     status.Message,
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
			Output:      result.Output,
			Cleanliness: result.Cleanliness,
			Message:     err.Error(),
		})
		return
	}

	response := updateResponse{
		Output:          result.Output,
		RestartRequired: result.BinaryChanged,
		Message:         "Aktualisiert.",
	}
	if result.BinaryChanged {
		response.Message = "Aktualisiert. Das Programm wurde ersetzt und läuft bis zum Neustart mit dem bisherigen Stand."
	}
	response.Links, response.Message = relinkAfterUpdate(environment.ProjectDir, response.Message)

	// Nach dem Pull erneut prüfen, damit Button und Karte den neuen Zustand
	// zeigen. Der Pull selbst kann den lokalen Zustand verändert haben.
	if status, err := project.CheckUpdate(environment.ProjectDir); err == nil {
		response.Available = status.Available
		response.Branch = status.Branch
		response.Local = shortCommit(status.Local)
		response.Remote = shortCommit(status.Remote)
		response.Cleanliness = status.Cleanliness
	}
	writeJSON(w, http.StatusOK, response)
}

// relinkAfterUpdate zieht die Assistenten-Verlinkung auf den neuen Stand nach
// und meldet, was sich dabei geändert hat.
//
// Das gehört zum Update, nicht in einen zweiten Schritt: seit Commands und
// Skills einzeln verlinkt werden, kommt ein neu mitgelieferter Command nicht
// mehr von selbst an. Ein Update, das den Katalog ändert, ihn aber nicht
// registriert, wäre halb erledigt — und zwar unsichtbar.
//
// Ein Fehler dabei lässt das Update selbst gültig: der Pull ist durch, und
// die Verlinkung kann über die Assistenten-Karte nachgeholt werden.
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

// shortCommit kürzt einen Commit-Hash auf die übliche Anzeigelänge.
func shortCommit(hash string) string {
	if len(hash) > 7 {
		return hash[:7]
	}
	return hash
}
