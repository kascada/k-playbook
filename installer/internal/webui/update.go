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
//
// Wechselt dabei die VERSION, gehört zum neuen Stand ein anderes Binary. Der
// Hintergrunddienst beendet sich dann nach der Antwort, wie bei /api/shutdown:
// ein weiterlaufender alter Daemon würde vom nächsten Aufruf zwar an der
// Version erkannt und ersetzt, hier ist der Wechsel aber schon bekannt.
func (state *serverState) applyUpdateHandler(w http.ResponseWriter, r *http.Request) {
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

	restartRequired := binaryOutdated(state.version, result)
	response := updateResponse{
		Output:          result.Output,
		RestartRequired: restartRequired,
		Message:         "Aktualisiert.",
	}
	if restartRequired {
		response.Message = versionChangeMessage()
	}
	if note := describeMCPRepair(result); note != "" {
		response.Message += " " + note
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
	state.completeUpdate(restartRequired)
}

// binaryOutdated meldet, ob zum aktualisierten Stand ein anderes Binary gehört
// als das laufende.
//
// Zwei Bedingungen, und beide werden gebraucht:
//
//   - Der Pull muss die VERSION bewegt haben. Ohne das ist der Stand derselbe
//     wie vorher, und ein Binary, das schon vorher passte, passt weiter.
//   - Das laufende Binary darf die neue Version nicht schon tragen. Genau hier
//     lag der Fehler: im Entwicklungsrepo steht unter ~/.local/bin längst das
//     Binary des neuen Standes, weil `make dev-install` es gebaut hat, während
//     der Clone noch dem zuletzt gepushten Commit folgt. Holt der ihn nach,
//     wechselt dort die VERSION — und der Dienst beendete sich, obwohl nichts
//     zu tun war, und verwies auf den Bootstrap. Der lädt das Release-Asset:
//     im Entwicklungsrepo der falsche Weg, und vor dem Release gibt es das
//     Asset nicht einmal.
//
// Warum nicht schlicht „laufende Version ≠ Version der Installation": im
// Entwicklungsrepo ist das Binary regelmäßig **neuer** als der Clone. Jeder
// Pull ohne Versionswechsel schlüge dann in eine Aufforderung um, ein älteres
// Binary zu installieren.
//
// Fehlt eine der beiden Angaben, wird nichts verlangt: ohne Vergleichsgrundlage
// ist ein selbsttätiges Ende des Dienstes das schlechtere Ergebnis.
func binaryOutdated(running string, result project.UpdateResult) bool {
	if !result.VersionChanged || running == "" || result.Version == "" {
		return false
	}
	return running != result.Version
}

// versionChangeMessage ist die Meldung des Versionswechsels.
//
// Sie sagt zwei Dinge, die zusammengehören: der Dienst endet jetzt, und das
// neue Binary kommt nicht von selbst — es wird über den Bootstrap installiert.
// Der steht hier nicht als Literal, sondern als project.BootstrapHint, damit
// Meldung, README und docs/installation.md dieselbe kanonische Form nennen. Ein
// bloßes `make install` gibt es in einem Zielprojekt nicht.
func versionChangeMessage() string {
	return "Aktualisiert. Zum neuen Stand gehört ein anderes Binary; der Dienst beendet sich jetzt. " +
		"Neu installieren mit: " + project.BootstrapHint + ". Danach k-playbook erneut aufrufen."
}

// completeUpdate beendet den Dienst nach einem Update, das ein neues Binary
// verlangt. Bei unveränderter VERSION läuft er weiter: der neue Stand liegt
// auf der Platte, und jeder Handler liest ihn bei der nächsten Anfrage.
func (state *serverState) completeUpdate(binaryChanged bool) {
	if !binaryChanged {
		return
	}
	state.shutdownAfterResponse()
}

// relinkAfterUpdate zieht die Assistenten-Einrichtung auf den neuen Stand nach
// und meldet, was sich dabei geändert hat.
//
// Das gehört zum Update, nicht in einen zweiten Schritt: seit Commands und
// Skills einzeln verlinkt werden, kommt ein neu mitgelieferter Command nicht
// mehr von selbst an. Ein Update, das den Katalog ändert, ihn aber nicht
// registriert, wäre halb erledigt — und zwar unsichtbar.
//
// Aufgerufen wird derselbe Ablauf wie beim Einrichten, nicht bloß ApplyLinks.
// Zwei Änderungen an diesem Einstieg sind gewollt und stehen deshalb im
// Antworttext: das Aktualisieren bringt jetzt den Anstoß mit — der Marker macht
// das idempotent und überschreibt vorhandenen Inhalt nie —, und in einem
// Projekt ohne AGENTS.md legt es die Datei erstmals aus der Vorlage an. Sonst
// bliebe ein Projekt mit nur echter CLAUDE.md über „Aktualisieren" für immer
// unverändert.
//
// Ein Fehler dabei lässt das Update selbst gültig: der Pull ist durch, und
// die Verlinkung kann über die Assistenten-Karte nachgeholt werden.
func relinkAfterUpdate(projectDir string, message string) (project.LinkChanges, string) {
	changes := project.PendingLinkChanges(project.CheckLinks(projectDir))

	setup, err := project.ApplyAssistantSetup(projectDir)
	if err != nil {
		return changes, message + " Die Verlinkung konnte nicht nachgezogen werden: " + err.Error()
	}

	parts := []string{message}
	if note := describeSetup(setup); note != "" {
		parts = append(parts, note)
	}
	if !changes.Empty() {
		parts = append(parts, describeLinkChanges(changes))
	}
	return changes, strings.Join(parts, " ")
}

// describeMCPRepair meldet, was das Update an veralteten MCP-Einträgen
// richtiggestellt hat.
//
// Das gehört in die Antwort und nicht ins Log: die Registrierung liegt im
// Hauptverzeichnis und ist damit eine Änderung an einer Projektdatei — sie
// stillschweigend vorzunehmen wäre genau die Art Nebenwirkung, die niemand
// erwartet.
func describeMCPRepair(result project.UpdateResult) string {
	parts := []string{}
	if len(result.MCPRepaired) > 0 {
		parts = append(parts, "MCP-Registrierung auf das installierte k-playbook korrigiert: "+
			strings.Join(result.MCPRepaired, ", ")+".")
	}
	if result.Message != "" {
		parts = append(parts, result.Message)
	}
	return strings.Join(parts, " ")
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
