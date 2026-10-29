package webui

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// Die Anbindung an die Kephalaion-Collection in der Oberfläche: Anzeige auf
// der Statusseite und der Einrichtungsschritt, der fragt, testet und erst
// nach bestandenem Test einträgt.
//
// Sicherheitsvertrag der vier Endpunkte. Task 082 lässt einen Seitenaufruf
// nur dann messen, wenn kein Wert aus Anfrage oder Projektdatei in die
// Messung einfließt. Hier fließen Hub und Account aus
// K-PLAYBOOK.local.yaml oder aus der Anfrage in einen Dateipfad und in
// Header; die Bedingung ist deshalb so gefasst: Hub und Account bestehen die
// Namensregel und entsprechen genau einem Kandidaten aus dem
// Token-Verzeichnis; gelesen wird dessen Pfad, nie ein Pfad aus den rohen
// Werten. Die Collection besteht die Namensregel und wird nur mit der
// Antwort von whoami verglichen. Gesendet wird nur an die Loopback-Adresse
// aus der config von Kephalaion. Das alles erzwingt das Paket kephalaion.
//
// Die GET-Endpunkte nehmen keinen Wert aus der Anfrage; sie messen beim
// Öffnen der Seite, weil die Anzeige dort der Zweck ist. Werte aus der Anfrage
// gibt es nur per POST, hinter sameOrigin. Ein POST ohne Origin passiert
// sameOrigin (server.go): Jeder lokale Prozess des Nutzers erreicht diese
// Endpunkte. Das ist tragbar, weil ein solcher Prozess die Token-Dateien
// ohnehin selbst lesen kann; er erfährt hier nichts, was er nicht schon hat,
// und bekommt nie ein Token zu sehen.

// kephalaionCacheTTL ist, wie lange ein Ergebnis der Prüfung gilt. Die
// Statusseite fragt beim Öffnen; ohne Zwischenspeicher fragte jeder
// Seitenwechsel den Node.
const kephalaionCacheTTL = 30 * time.Second

// kephalaionEnv liefert die Kephalaion-Einrichtung dieses Rechners, in
// derselben Reihenfolge gesucht wie von Kephalaion selbst; austauschbar für
// Tests.
var kephalaionEnv = kephalaion.DefaultEnv

// kephalaionCache hält das letzte Ergebnis der Prüfung je Projekt.
var kephalaionCache struct {
	mu         sync.Mutex
	projectDir string
	status     kephalaion.Status
	at         time.Time
}

// cachedKephalaionStatus liefert das Ergebnis der Prüfung, höchstens
// kephalaionCacheTTL alt.
func cachedKephalaionStatus(projectDir string) (kephalaion.Status, time.Time) {
	kephalaionCache.mu.Lock()
	if kephalaionCache.projectDir == projectDir && time.Since(kephalaionCache.at) < kephalaionCacheTTL {
		status, at := kephalaionCache.status, kephalaionCache.at
		kephalaionCache.mu.Unlock()
		return status, at
	}
	kephalaionCache.mu.Unlock()

	status := kephalaion.Check(context.Background(), project.LocalDir(projectDir), kephalaionEnv())
	at := time.Now()
	kephalaionCache.mu.Lock()
	kephalaionCache.projectDir, kephalaionCache.status, kephalaionCache.at = projectDir, status, at
	kephalaionCache.mu.Unlock()
	return status, at
}

// dropKephalaionCache verwirft das zwischengespeicherte Ergebnis nach dem
// Eintragen.
func dropKephalaionCache() {
	kephalaionCache.mu.Lock()
	kephalaionCache.at = time.Time{}
	kephalaionCache.mu.Unlock()
}

type kephalaionResponse struct {
	Available bool               `json:"available"`
	Status    *kephalaion.Status `json:"status,omitempty"`
	// LocalFile ist der Ort von K-PLAYBOOK.local.yaml in Anzeigeform.
	LocalFile string `json:"localFile,omitempty"`
	// CheckedAt ist der Zeitpunkt der Prüfung (RFC 3339).
	CheckedAt string `json:"checkedAt,omitempty"`
	Message   string `json:"message,omitempty"`
}

// kephalaionHandler zeigt die Anbindung samt Ergebnis der Prüfung. GET ohne
// Werte aus der Anfrage; Vertrag siehe oben.
func kephalaionHandler(w http.ResponseWriter, r *http.Request) {
	environment := project.Detect()
	if !environment.Installed {
		writeJSON(w, http.StatusOK, kephalaionResponse{Message: "Keine " + project.ConfigFileName + " gefunden."})
		return
	}
	status, at := cachedKephalaionStatus(environment.ProjectDir)
	writeJSON(w, http.StatusOK, kephalaionResponse{
		Available: true,
		Status:    &status,
		LocalFile: project.DisplayPath(kephalaion.LocalFilePath(project.LocalDir(environment.ProjectDir))),
		CheckedAt: at.Format(time.RFC3339),
	})
}

// kephalaionSelection ist die Vorauswahl des Einrichtungsschritts.
type kephalaionSelection struct {
	Hub        string `json:"hub"`
	Account    string `json:"account"`
	Collection string `json:"collection"`
}

type kephalaionCandidatesResponse struct {
	Available bool `json:"available"`
	// ConfigPresent sagt, ob es eine config von Kephalaion gibt.
	ConfigPresent bool `json:"configPresent"`
	// ConfigPath ist die config, die gilt, in Anzeigeform; ohne config der
	// Ort beim User, an dem kephalaion init sie anlegt.
	ConfigPath string `json:"configPath"`
	// ConfigSource sagt, woher sie kommt: env (KEPHALAION_CONFIG), user
	// oder system (/etc/kephalaion/config.yaml).
	ConfigSource kephalaion.ConfigSource `json:"configSource"`
	// ConfigProblem ist der Satz, wenn die config fehlt oder nicht lesbar
	// ist; er nennt die gesuchten bzw. gelesenen Pfade.
	ConfigProblem string `json:"configProblem,omitempty"`
	// TokensDir ist das Token-Verzeichnis des Users in Anzeigeform — auch
	// bei globaler config.
	TokensDir string `json:"tokensDir"`
	// Endpoint ist die Adresse von /mcp am Node; leer, wenn unzulässig.
	Endpoint string `json:"endpoint"`
	// AddressProblem erklärt einen fehlenden Endpoint.
	AddressProblem string `json:"addressProblem,omitempty"`
	// Pairs sind die Kandidaten mit ihren lesbaren Collections.
	Pairs []kephalaion.Pair `json:"pairs"`
	// Pending sind Paare, deren Token noch auf den Tausch am Node wartet.
	Pending []kephalaion.Pending `json:"pending"`
	// ProjectName ist der Name des Projektverzeichnisses; die gleichnamige
	// Collection wird vorgewählt, wenn es sie gibt.
	ProjectName string              `json:"projectName"`
	Preselect   kephalaionSelection `json:"preselect"`
	// AdminSteps sind die Schritte am Hub, in der festgelegten Reihenfolge,
	// für den Fall, dass kein Paar trägt.
	AdminSteps []string `json:"adminSteps"`
	Message    string   `json:"message,omitempty"`
}

// kephalaionAdminSteps ist der Ablauf für ein neues Projekt (Entscheidung
// vom 2026-10-09): Das macht ein Admin in Kephalaion, danach trägt
// k-playbook den Namen der Collection ein.
func kephalaionAdminSteps(projectName string) []string {
	return []string{
		"Einen Kephalaion-Hub finden und auf diesem Rechner einen Node einrichten, der ihn abgleicht (kephalaion status zeigt den Alias).",
		"Am Hub die Collection des Projekts anlegen, meist nach dem Projekt benannt: " + projectName + ".",
		"Am Hub für diesen Rechner einen eigenen Account anlegen und ihm die Collection geben (mindestens read; write zum Schreiben).",
		"Den User eintragen, der diesen Rechner bedient, und das Token des Accounts am Node ablegen (tokens/<hub>/<account>.token).",
		"Danach hier die Collection wählen, testen und eintragen.",
	}
}

// kephalaionCandidatesHandler listet die Paare aus dem Token-Verzeichnis mit
// ihren lesbaren Collections. GET ohne Werte aus der Anfrage; die Paare
// kommen allein aus Dateinamen, Vertrag siehe oben.
func kephalaionCandidatesHandler(w http.ResponseWriter, r *http.Request) {
	environment := project.Detect()
	if !environment.Installed {
		writeJSON(w, http.StatusOK, kephalaionCandidatesResponse{
			Pairs: []kephalaion.Pair{}, Pending: []kephalaion.Pending{},
			Message: "Keine " + project.ConfigFileName + " gefunden.",
		})
		return
	}
	env := kephalaionEnv()
	_, pending := env.Candidates()
	if pending == nil {
		pending = []kephalaion.Pending{}
	}
	projectName := filepath.Base(environment.ProjectDir)
	response := kephalaionCandidatesResponse{
		Available:      true,
		ConfigPresent:  env.ConfigPresent,
		ConfigPath:     project.DisplayPath(env.ConfigPath),
		ConfigSource:   env.ConfigSource,
		ConfigProblem:  env.ConfigProblem(),
		TokensDir:      project.DisplayPath(env.TokensDir()),
		Endpoint:       env.Endpoint,
		AddressProblem: env.AddressProblem,
		Pairs:          kephalaion.ProbePairs(r.Context(), env),
		Pending:        pending,
		ProjectName:    projectName,
		AdminSteps:     kephalaionAdminSteps(projectName),
	}
	current, _, _ := kephalaion.ReadBinding(project.LocalDir(environment.ProjectDir))
	response.Preselect = preselect(response.Pairs, current, projectName)
	writeJSON(w, http.StatusOK, response)
}

// preselect wählt Paar und Collection vor: das eingetragene Paar, wenn es
// unter den Kandidaten ist, sonst das erste angemeldete; als Collection die
// eingetragene, sonst die nach dem Projekt benannte, sonst die einzige.
func preselect(pairs []kephalaion.Pair, current kephalaion.Binding, projectName string) kephalaionSelection {
	var chosen *kephalaion.Pair
	for i := range pairs {
		if pairs[i].Hub == current.Hub && pairs[i].Account == current.Account {
			chosen = &pairs[i]
			break
		}
	}
	if chosen == nil {
		for i := range pairs {
			if pairs[i].State == kephalaion.StateOK {
				chosen = &pairs[i]
				break
			}
		}
	}
	if chosen == nil && len(pairs) > 0 {
		chosen = &pairs[0]
	}
	if chosen == nil {
		return kephalaionSelection{}
	}
	sel := kephalaionSelection{Hub: chosen.Hub, Account: chosen.Account}
	has := func(name string) bool {
		for _, c := range chosen.Collections {
			if c.Collection == name {
				return true
			}
		}
		return false
	}
	switch {
	case current.Collection != "" && has(current.Collection):
		sel.Collection = current.Collection
	case has(projectName):
		sel.Collection = projectName
	case len(chosen.Collections) == 1:
		sel.Collection = chosen.Collections[0].Collection
	}
	return sel
}

// maxKephalaionRequest begrenzt den Rumpf eines POST: drei kurze Namen.
const maxKephalaionRequest = 4 << 10

type kephalaionRequest struct {
	Hub        string `json:"hub"`
	Collection string `json:"collection"`
	Account    string `json:"account"`
}

type kephalaionTestResponse struct {
	Status  kephalaion.Status `json:"status"`
	Saved   bool              `json:"saved"`
	Message string            `json:"message,omitempty"`
}

// readKephalaionRequest liest und prüft den Rumpf. Ein Name außerhalb der
// Namensregel endet hier mit 400; ob das Paar ein Kandidat ist, prüft
// CheckBinding.
func readKephalaionRequest(w http.ResponseWriter, r *http.Request) (kephalaion.Binding, bool) {
	var request kephalaionRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxKephalaionRequest)).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, kephalaionTestResponse{Message: "Anfrage nicht lesbar."})
		return kephalaion.Binding{}, false
	}
	binding := kephalaion.Binding{Hub: request.Hub, Collection: request.Collection, Account: request.Account}
	if err := kephalaion.ValidateBinding(binding); err != nil {
		writeJSON(w, http.StatusBadRequest, kephalaionTestResponse{
			Status:  kephalaion.Status{Configured: true, State: kephalaion.StateInvalidEntry, Message: "Eintrag ungültig: " + err.Error() + "."},
			Message: "Eintrag ungültig: " + err.Error() + ".",
		})
		return kephalaion.Binding{}, false
	}
	return binding, true
}

// kephalaionTestHandler testet Paar und Collection aus der Anfrage und
// schreibt nichts. Messender Endpunkt mit Werten aus der Anfrage: nur per
// POST hinter sameOrigin; Hub und Account müssen genau einem Kandidaten
// entsprechen, die Collection wird nur mit whoami verglichen, gesendet wird
// nur an Loopback. Ein POST ohne Origin kommt durch — von einem lokalen
// Prozess, der die Token-Dateien ohnehin lesen kann. Vertrag siehe oben.
func kephalaionTestHandler(w http.ResponseWriter, r *http.Request) {
	if !project.Detect().Installed {
		writeJSON(w, http.StatusConflict, kephalaionTestResponse{Message: "Keine " + project.ConfigFileName + " gefunden."})
		return
	}
	binding, ok := readKephalaionRequest(w, r)
	if !ok {
		return
	}
	status := kephalaion.CheckBinding(r.Context(), binding, kephalaionEnv())
	writeJSON(w, http.StatusOK, kephalaionTestResponse{Status: status})
}

// kephalaionSaveHandler testet erneut und trägt nur bei ok ein: Er ersetzt
// den Block kephalaion: in K-PLAYBOOK.local.yaml und ergänzt, falls Git die
// Datei sonst nicht ignorierte, k-playbook-local/.gitignore. Schreibender
// Endpunkt mit Werten aus der Anfrage: derselbe Vertrag wie beim Test, dazu
// schreibt er nur Namen, die die Namensregel bestehen und gerade getestet
// wurden. Ein POST ohne Origin kommt durch — siehe oben, warum das tragbar ist.
func kephalaionSaveHandler(w http.ResponseWriter, r *http.Request) {
	environment := project.Detect()
	if !environment.Installed {
		writeJSON(w, http.StatusConflict, kephalaionTestResponse{Message: "Keine " + project.ConfigFileName + " gefunden."})
		return
	}
	binding, ok := readKephalaionRequest(w, r)
	if !ok {
		return
	}
	status := kephalaion.CheckBinding(r.Context(), binding, kephalaionEnv())
	if !status.OK {
		writeJSON(w, http.StatusConflict, kephalaionTestResponse{
			Status:  status,
			Message: "Nicht eingetragen: Der Test ist nicht bestanden.",
		})
		return
	}
	if err := kephalaion.WriteBinding(project.LocalDir(environment.ProjectDir), binding); err != nil {
		writeJSON(w, http.StatusConflict, kephalaionTestResponse{Status: status, Message: "Nicht eingetragen: " + err.Error() + "."})
		return
	}
	dropKephalaionCache()
	writeJSON(w, http.StatusOK, kephalaionTestResponse{
		Status:  status,
		Saved:   true,
		Message: "Eingetragen in " + project.DisplayPath(kephalaion.LocalFilePath(project.LocalDir(environment.ProjectDir))) + ".",
	})
}
