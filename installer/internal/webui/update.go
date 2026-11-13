package webui

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/program"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// updateResponse ist der Zustand der Aktualisierung.
type updateResponse struct {
	Available bool   `json:"available"`
	Branch    string `json:"branch"`
	Local     string `json:"local"`
	Remote    string `json:"remote"`
	Output    string `json:"output"`
	// Links nennt, was das Update an der Registrierung von Commands und Skills
	// geändert hat.
	Links   project.LinkChanges `json:"links"`
	Message string              `json:"message"`
	// Program vergleicht das laufende Programm mit dem neuesten Release-Tag.
	// Die Angabe hängt nicht am Clone und nicht an dessen Remote: sie steht
	// auch dann da, wenn es keinen Clone gibt oder dessen ls-remote scheitert.
	Program *programStatus `json:"program,omitempty"`

	// Die Felder darunter trägt nur die Antwort auf einen POST, der
	// installiert und neu gestartet hat oder es versucht hat.

	// Restarted: der Dienst läuft jetzt unter URL, dieser beendet sich nach
	// der Antwort. Die Seite wechselt selbst dorthin.
	Restarted bool   `json:"restarted,omitempty"`
	URL       string `json:"url,omitempty"`
	// Version ist die des neuen Dienstes.
	Version string `json:"version,omitempty"`
	// InstallFailed: Installation oder Neustart sind gescheitert, dieser
	// Dienst läuft weiter. Message nennt Fehler und Befehl zum Nachholen.
	InstallFailed bool `json:"installFailed,omitempty"`
	// InstallOutput nennt, was die Installation geladen und geprüft hat, und
	// bei einem gescheiterten Start das Log des neuen Dienstes.
	InstallOutput string `json:"installOutput,omitempty"`
	// Busy: es laufen Befehle oder Chats, und der Aufruf trug keine
	// Bestätigung. Message ist die Rückfrage.
	Busy bool `json:"busy,omitempty"`
}

// programStatus ist der Vergleich des laufenden Programms mit dem neuesten
// Release-Tag. Die Prüfung liegt in program.CheckStatus, weil
// `k-playbook upgrade` dieselbe braucht; hier bleibt der Name, den die Seite
// kennt.
type programStatus = program.Status

// checkProgram vergleicht die Version dieses Dienstes mit dem neuesten
// Release-Tag — die gemeinsame Prüfung für GET /api/update, POST /api/update
// und POST /api/update/program. Nur „älter" meldet; Begründung bei
// program.CheckStatus. Ohne eigene Version fragt sie nichts ab.
func checkProgram(ctx context.Context, running string) *programStatus {
	return program.CheckStatus(ctx, running)
}

// updateCheckHandler prüft den Remote-Stand des Clones und das laufende
// Programm. Rein lesend: heruntergeladen wird hier nie etwas.
//
// Beides steht nebeneinander in der Antwort. Trifft beides zu, hat das ältere
// Programm Vorrang in der Oberfläche: Der Knopf installiert dann das neueste
// Release, und der Pull dient nur noch den Inhalten des Clones.
func (state *serverState) updateCheckHandler(w http.ResponseWriter, r *http.Request) {
	environment := project.Detect()
	if !environment.Installed {
		writeJSON(w, http.StatusOK, updateResponse{})
		return
	}
	programState := checkProgram(r.Context(), state.version)

	status, err := project.CheckUpdate(environment.ProjectDir)
	if err != nil {
		writeJSON(w, http.StatusOK, updateResponse{Message: "Prüfung fehlgeschlagen: " + err.Error(), Program: programState})
		return
	}
	writeJSON(w, http.StatusOK, updateResponse{
		Available: status.Available,
		Branch:    status.Branch,
		Local:     shortCommit(status.Local),
		Remote:    shortCommit(status.Remote),
		Message:   status.Message,
		Program:   programState,
	})
}

// updateBusyMessage ist die Antwort auf einen zweiten Aufruf, während einer
// läuft.
const updateBusyMessage = "Aktualisierung läuft bereits."

// updatePull und updateRepairMCP sind austauschbar, damit die Abfolge nach
// einem gescheiterten Pull ohne Clone und ohne Netz prüfbar ist.
var (
	updatePull      = project.Update
	updateRepairMCP = project.RepairMCPReport
)

// applyUpdateHandler holt den neuen Stand des Clones und prüft danach das
// Programm.
//
// Der Pull dient nur noch den Inhalten des Clones; über Version und
// Installation des Programms entscheidet er nicht. Deshalb kehrt der Handler
// nach einem gescheiterten Pull nicht zurück, bevor zwei Dinge erledigt sind:
//
//   - die MCP-Registrierung: veraltete Einträge werden korrigiert
//     (MCPWriteOutdatedOnly), ob der Pull gelang oder nicht — sie liegt im
//     Hauptverzeichnis und hängt nicht am Clone;
//   - das Programm: Ist das laufende älter als der neueste Release-Tag,
//     installiert der Dienst das Release-Binary und startet daraus neu —
//     derselbe Ablauf wie POST /api/update/program. Knopf und Neustart hängen
//     am Tag, nicht daran, ob der Pull etwas bewegt hat. Ist das laufende
//     Programm gleich alt oder neuer, läuft der Dienst weiter.
//
// Ein gescheiterter Pull wird weiter gemeldet, mit 409 wie bisher — außer der
// Dienst hat neu gestartet: dann muss die Seite der neuen Adresse folgen, und
// die Meldung nennt den Pull trotzdem.
//
// Zeigt der PATH auf ein anderes Programm, bleibt es beim Pull: der Dienst
// läuft weiter, und die Antwort trägt den Hinweis statt eines Neustarts. Laufen
// Befehle oder Chats, wird ebenfalls nicht neu gestartet; die Prüfung meldet
// danach „Programm älter", und der Knopf fragt vor dem Neustart nach.
func (state *serverState) applyUpdateHandler(w http.ResponseWriter, r *http.Request) {
	environment := project.Detect()
	if !environment.Installed {
		writeJSON(w, http.StatusConflict, updateResponse{
			Message: "Keine " + project.ConfigFileName + " gefunden.",
		})
		return
	}
	if !state.updateMu.TryLock() {
		writeJSON(w, http.StatusConflict, updateResponse{Message: updateBusyMessage})
		return
	}
	defer state.updateMu.Unlock()

	var result project.UpdateResult
	var pullErr error
	if environment.PlaybookPresent {
		result, pullErr = updatePull(environment.ProjectDir)
	} else {
		pullErr = errors.New("Die Installation " + project.PlaybookDirName + "/ fehlt; der Pull entfällt")
	}
	response := updateResponse{Output: result.Output, Message: "Aktualisiert."}
	status := http.StatusOK
	if pullErr != nil {
		response.Message = strings.TrimSuffix(pullErr.Error(), ".") + "."
		status = http.StatusConflict
	}
	if note := describeMCPRepair(updateRepairMCP(environment.ProjectDir, project.MCPWriteOutdatedOnly)); note != "" {
		response.Message += " " + note
	}
	if pullErr == nil {
		response.Links, response.Message = relinkAfterUpdate(environment.ProjectDir, response.Message)

		// Nach dem Pull erneut prüfen, damit Button und Karte den neuen Zustand
		// zeigen. Der Pull selbst kann den lokalen Zustand verändert haben.
		if update, err := project.CheckUpdate(environment.ProjectDir); err == nil {
			response.Available = update.Available
			response.Branch = update.Branch
			response.Local = shortCommit(update.Local)
			response.Remote = shortCommit(update.Remote)
		}
	}
	response.Program = checkProgram(r.Context(), state.version)

	if !response.Program.Outdated {
		writeJSON(w, status, response)
		return
	}
	newer := fmt.Sprintf(" Das laufende Programm (%s) ist älter als das neueste Release (%s).",
		displayVersion(response.Program.Running), displayVersion(response.Program.Latest))
	if !response.Program.Installable {
		response.Message += newer + " " + response.Program.Hint
		writeJSON(w, status, response)
		return
	}
	if activity := state.activeWork(); activity != "" && !confirmed(r) {
		response.Message += newer + " " + activity +
			" Deshalb startet der Dienst jetzt nicht neu; „Programm aktualisieren“ holt das nach."
		writeJSON(w, status, response)
		return
	}

	outcome := state.installAndRestart(response.Program.Latest)
	outcome.applyTo(&response)
	if outcome.Restarted {
		// Die Seite folgt der neuen Adresse; ein 409 hielte sie davon ab.
		writeJSON(w, http.StatusOK, response)
		state.shutdownAfterResponse()
		return
	}
	writeJSON(w, status, response)
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
// Drei Änderungen an diesem Einstieg sind gewollt und stehen deshalb im
// Antworttext: das Aktualisieren bringt jetzt den Anstoß mit — der Marker macht
// das idempotent und überschreibt vorhandenen Inhalt nie —, es zieht einen
// veralteten Anstoß nach und entfernt die abgelösten Zusatzblöcke
// (describeSetup), und in einem Projekt ohne AGENTS.md legt es die Datei
// erstmals aus der Vorlage an. Sonst bliebe ein Projekt mit nur echter
// CLAUDE.md über „Aktualisieren" für immer unverändert.
//
// Ein Fehler dabei lässt das Update selbst gültig: der Pull ist durch, und
// die Verlinkung kann über die Assistenten-Karte nachgeholt werden. Den
// README-Schritt (Task 137) meldet auch dieser Zweig: Er steht neben dem
// Fehlerwert im Ergebnis und ist auch dann gelaufen.
func relinkAfterUpdate(projectDir string, message string) (project.LinkChanges, string) {
	changes := project.PendingLinkChanges(project.CheckLinks(projectDir))

	setup, err := project.ApplyAssistantSetup(projectDir)
	if err != nil {
		text := message + " Die Verlinkung konnte nicht nachgezogen werden: " + err.Error()
		if note := project.DescribeSetupReadmes(setup.Readmes); note != "" {
			text += " " + note
		}
		return changes, text
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
//
// Das Update ersetzt veraltete Einträge nach derselben Regel wie der Start und
// schreibt in einer erfassten Datei — oder bei unbeantworteter Tracking-Frage —
// den bloßen Kommandonamen. Dann gilt derselbe Hinweis wie beim Start: ein aus
// Dock oder Finder gestarteter Client findet den Namen nicht. Er hängt wie dort
// an der geschriebenen Form (MCPRepairReport.Portable). Ohne ihn läse
// „auf das installierte k-playbook korrigiert" sich wie ein absoluter Pfad.
//
// Repariert wird nach dem Pull, auch wenn er gescheitert ist
// (applyUpdateHandler): die Reparatur hängt nicht mehr am Pull.
func describeMCPRepair(repair project.MCPRepairReport) string {
	parts := []string{}
	if len(repair.Repaired) > 0 {
		parts = append(parts, "MCP-Registrierung auf das installierte k-playbook korrigiert: "+
			strings.Join(repair.Repaired, ", ")+".")
	}
	if len(repair.Portable) > 0 {
		parts = append(parts, "In "+strings.Join(repair.Portable, ", ")+
			" steht jetzt der bloße Name "+project.InstalledCommandName+
			", weil die Datei von git erfasst ist oder sich das nicht klären ließ. "+
			"Ein aus Dock oder Finder gestarteter Client findet ihn nicht und braucht den absoluten Pfad, von Hand eingetragen.")
	}
	if repair.Message != "" {
		parts = append(parts, repair.Message)
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
