package webui

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/program"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// updateResponse ist der Zustand der Aktualisierung.
type updateResponse struct {
	Message string `json:"message"`
	// Program vergleicht das laufende Programm mit dem neuesten Release-Tag.
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
// Release-Tag — die gemeinsame Prüfung für GET /api/update und
// POST /api/update/program. Nur „älter" meldet; Begründung bei
// program.CheckStatus. Ohne eigene Version fragt sie nichts ab.
//
// lookup sagt, ob der Cache gilt: POST /api/update/program installiert und
// fragt frisch, GET /api/update ist ein Hinweis beim Laden der Seite und nimmt den
// Cache — außer mit fresh=1, dem ausdrücklichen Prüfen (Knopf „Update
// prüfen“).
func checkProgram(ctx context.Context, running string, lookup program.Lookup) *programStatus {
	return program.CheckStatus(ctx, running, lookup)
}

// updateCheckLookup: GET /api/update nimmt den Cache, mit fresh=1 fragt es
// frisch.
func updateCheckLookup(r *http.Request) program.Lookup {
	if r.URL.Query().Get("fresh") == "1" {
		return program.Fresh
	}
	return program.Cached
}

// updateCheckHandler prüft das laufende Programm gegen den neuesten
// Release-Tag. Rein lesend: heruntergeladen wird hier nie etwas. Den Tag nimmt
// er aus dem Cache, mit fresh=1 fragt er frisch (updateCheckLookup).
func (state *serverState) updateCheckHandler(w http.ResponseWriter, r *http.Request) {
	environment := project.Detect()
	if !environment.Installed {
		writeJSON(w, http.StatusOK, updateResponse{})
		return
	}
	writeJSON(w, http.StatusOK, updateResponse{
		Program: checkProgram(r.Context(), state.version, updateCheckLookup(r)),
	})
}

// updateBusyMessage ist die Antwort auf einen zweiten Aufruf, während einer
// läuft.
const updateBusyMessage = "Aktualisierung läuft bereits."

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
