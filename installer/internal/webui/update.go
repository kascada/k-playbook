package webui

import (
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
	// Program vergleicht das laufende Programm mit der VERSION des Clones.
	// Die Angabe hängt nicht am Remote: sie steht auch dann da, wenn
	// ls-remote scheitert.
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
	// InstallOutput ist die Ausgabe von bin/install, sofern es lief, und bei
	// einem gescheiterten Start das Log des neuen Dienstes.
	InstallOutput string `json:"installOutput,omitempty"`
	// Busy: es laufen Befehle oder Chats, und der Aufruf trug keine
	// Bestätigung. Message ist die Rückfrage.
	Busy bool `json:"busy,omitempty"`
}

// programStatus ist der Vergleich des laufenden Programms mit dem Clone.
type programStatus struct {
	// Running ist die Version dieses Dienstes, Clone die VERSION der
	// Installation.
	Running string `json:"running"`
	Clone   string `json:"clone"`
	// Order ist das Ergebnis des Vergleichs: älter, gleich, neuer, unbekannt.
	Order string `json:"order"`
	// Outdated: das laufende Programm ist älter als der Clone. Nur das löst
	// eine Meldung aus.
	Outdated bool `json:"outdated"`
	// Installable: `k-playbook` im PATH zeigt auf das Installationsziel. Nur
	// dann gibt es den Knopf; sonst steht Hint da, mit beiden Pfaden.
	Installable bool   `json:"installable"`
	Target      string `json:"target,omitempty"`
	Found       string `json:"found,omitempty"`
	Hint        string `json:"hint,omitempty"`
}

// checkProgram vergleicht die Version dieses Dienstes mit der VERSION des
// Clones — die gemeinsame Prüfung für GET /api/update, den Versionswechsel im
// POST und POST /api/update/program.
//
// Nur „älter" meldet. Gleich oder neuer ist im Entwicklungsrepo nach
// `make dev-install` der Normalfall, und eine nicht lesbare Version ist kein
// Nachweis. Den PATH prüft sie nur, wenn es etwas anzubieten gäbe.
func checkProgram(running string, playbookDir string) *programStatus {
	clone := project.InstalledVersion(playbookDir)
	order := program.Compare(running, clone)
	status := &programStatus{Running: running, Clone: clone, Order: order.String(), Outdated: order == program.Older}
	if !status.Outdated {
		return status
	}
	path := program.CheckPath()
	status.Target, status.Found, status.Installable = path.Target, path.Found, path.OK
	status.Hint = path.Hint()
	return status
}

// updateCheckHandler prüft den Remote-Stand und das laufende Programm. Rein
// lesend: heruntergeladen wird hier nie etwas.
//
// Beides steht nebeneinander in der Antwort. Trifft beides zu, hat das ältere
// Programm Vorrang in der Oberfläche, denn ein Pull allein hilft dann nicht —
// der Clone ist schon weiter als das Programm, das ihn bedient.
func (state *serverState) updateCheckHandler(w http.ResponseWriter, r *http.Request) {
	environment := project.Detect()
	if !environment.Installed {
		writeJSON(w, http.StatusOK, updateResponse{})
		return
	}
	programState := checkProgram(state.version, environment.PlaybookDir)

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

// applyUpdateHandler holt den neuen Stand.
//
// Bewegt der Pull die VERSION und ist das laufende Programm älter als die
// neue, installiert der Dienst das passende Programm über den Bootstrap des
// Clones und startet daraus neu — derselbe Ablauf wie
// POST /api/update/program. Ist das laufende Programm gleich alt oder neuer,
// läuft er weiter wie bei einem Update ohne Versionswechsel.
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

	result, err := project.Update(environment.ProjectDir)
	if err != nil {
		writeJSON(w, http.StatusConflict, updateResponse{
			Output:  result.Output,
			Message: err.Error(),
		})
		return
	}

	response := updateResponse{
		Output:  result.Output,
		Message: "Aktualisiert.",
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
	}
	response.Program = checkProgram(state.version, environment.PlaybookDir)

	if !restartAfterPull(state.version, result) {
		writeJSON(w, http.StatusOK, response)
		return
	}
	if !response.Program.Installable {
		response.Message += " Zum neuen Stand gehört ein neueres Programm. " + response.Program.Hint
		writeJSON(w, http.StatusOK, response)
		return
	}
	if activity := state.activeWork(); activity != "" && !confirmed(r) {
		response.Message += " Zum neuen Stand gehört ein neueres Programm. " + activity +
			" Deshalb startet der Dienst jetzt nicht neu; „Programm aktualisieren“ holt das nach."
		writeJSON(w, http.StatusOK, response)
		return
	}

	outcome := state.installAndRestart(environment.ProjectDir)
	outcome.applyTo(&response)
	writeJSON(w, http.StatusOK, response)
	if outcome.Restarted {
		state.shutdownAfterResponse()
	}
}

// restartAfterPull meldet, ob nach dem Pull installiert und neu gestartet
// wird: die VERSION hat gewechselt, und das laufende Programm ist älter als
// die neue.
//
// Beide Bedingungen werden gebraucht. Ohne Versionswechsel verhält sich ein
// Update wie immer — Pull, Verlinkung, der Dienst läuft weiter. Und nur
// „älter" löst aus, nie „ungleich": im Entwicklungsrepo ist das Programm
// regelmäßig neuer als der Clone, weil `make dev-install` es gebaut hat,
// während der Clone noch dem zuletzt gepushten Commit folgt. Holt er ihn
// nach, wechselt dort die VERSION; ein Vergleich auf Ungleichheit stufte das
// Programm dann herab. Fehlt eine Angabe oder ist sie nicht lesbar, wird
// nichts verlangt.
func restartAfterPull(running string, result project.UpdateResult) bool {
	return result.VersionChanged && program.Compare(running, result.Version) == program.Older
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
//
// Das Update ersetzt veraltete Einträge nach derselben Regel wie der Start und
// schreibt in einer erfassten Datei — oder bei unbeantworteter Tracking-Frage —
// den bloßen Kommandonamen. Dann gilt derselbe Hinweis wie beim Start: ein aus
// Dock oder Finder gestarteter Client findet den Namen nicht. Er hängt wie dort
// an der geschriebenen Form (UpdateResult.MCPRepairedPortable). Ohne ihn läse
// „auf das installierte k-playbook korrigiert" sich wie ein absoluter Pfad.
func describeMCPRepair(result project.UpdateResult) string {
	parts := []string{}
	if len(result.MCPRepaired) > 0 {
		parts = append(parts, "MCP-Registrierung auf das installierte k-playbook korrigiert: "+
			strings.Join(result.MCPRepaired, ", ")+".")
	}
	if len(result.MCPRepairedPortable) > 0 {
		parts = append(parts, "In "+strings.Join(result.MCPRepairedPortable, ", ")+
			" steht jetzt der bloße Name "+project.InstalledCommandName+
			", weil die Datei von git erfasst ist oder sich das nicht klären ließ. "+
			"Ein aus Dock oder Finder gestarteter Client findet ihn nicht und braucht den absoluten Pfad, von Hand eingetragen.")
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
