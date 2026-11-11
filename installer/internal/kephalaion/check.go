package kephalaion

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// State ist das Ergebnis einer Prüfung. Jeder Zustand hat seinen eigenen
// Satz (Status.Message); die Reihenfolge der Konstanten ist die Reihenfolge,
// in der Check prüft.
type State string

const (
	// StateNotConfigured: Der Block kephalaion: in K-PLAYBOOK.yaml fehlt.
	StateNotConfigured State = "not-configured"
	// StateInvalidEntry: Ein Name verstößt gegen die Namensregel, oder die
	// Projektkonfiguration lässt sich nicht lesen.
	StateInvalidEntry State = "invalid-entry"
	// StateNoKephalaion: Das Binary kephalaion fehlt oder ist zu alt für die
	// Auskunft im Format 1 (QueryClient).
	StateNoKephalaion State = "kephalaion-missing"
	// StateClientFailed: Die Auskunft kephalaion node client ist nicht
	// entstanden (Exit 1, Zeitgrenze, Absturz) oder nennt den Hub nicht.
	StateClientFailed State = "client-failed"
	// StateNodeAddress: Node-Adresse fehlt oder ist keine Loopback-Adresse.
	StateNodeAddress State = "node-address"
	// StateAccountChoiceMissing: Für den Hub gibt es mehrere Token-Dateien
	// und keinen Standard (choice missing mit Kandidaten).
	StateAccountChoiceMissing State = "account-choice-missing"
	// StateTokenMissing: Für den Hub gibt es keine brauchbare Token-Datei —
	// keine, nur ein wartendes Token, eine an falschem Ort oder eine
	// unbrauchbare.
	StateTokenMissing State = "token-missing"
	// StateNodeUnreachable: Der Node antwortet nicht oder nicht verständlich.
	StateNodeUnreachable State = "node-unreachable"
	// StateHubUnknown: Der Hub-Alias steht in unknown_hubs.
	StateHubUnknown State = "hub-unknown"
	// StateHubNeverSynced: login invalid mit sync.never_synced.
	StateHubNeverSynced State = "hub-never-synced"
	// StateLoginInvalid: login invalid ohne never_synced.
	StateLoginInvalid State = "login-invalid"
	// StateLoginMissing: login missing, Replica lesbar.
	StateLoginMissing State = "login-missing"
	// StateReplicaUnreadable: login missing mit „Replica nicht lesbar“.
	StateReplicaUnreadable State = "replica-unreadable"
	// StateCollectionUnreadable: angemeldet, aber die Collection ist nicht
	// unter den lesbaren.
	StateCollectionUnreadable State = "collection-unreadable"
	// StateOK: angemeldet, Collection lesbar.
	StateOK State = "ok"
)

// replicaUnreadable ist der feste Satz, den whoami in sync.last_error setzt,
// wenn sich die Replica nicht lesen lässt (ReplicaUnreadable in Kephalaion).
const replicaUnreadable = "Replica nicht lesbar"

// Sync ist der Stand des Abgleichs eines Hubs, wie whoami ihn meldet.
type Sync struct {
	NeverSynced bool   `json:"neverSynced,omitempty"`
	LastSuccess string `json:"lastSuccess,omitempty"`
	Revision    *int64 `json:"revision,omitempty"`
	LastError   string `json:"lastError,omitempty"`
}

// Status ist das Ergebnis einer Prüfung der Anbindung. Ein Token steht nie
// darin.
type Status struct {
	// Configured sagt, ob der Block kephalaion: dasteht.
	Configured bool   `json:"configured"`
	Hub        string `json:"hub"`
	Collection string `json:"collection"`
	Account    string `json:"account"`
	// Choice sagt, woher der Account kommt (Auskunft kephalaion node
	// client): override, default, only; leer, solange keiner gewählt ist.
	Choice Choice `json:"choice,omitempty"`
	// Address ist <hub>:<collection>, sobald beide Namen gültig sind.
	Address string `json:"address"`
	State   State  `json:"state"`
	// OK ist State == StateOK, für Leser, die nur das wissen wollen.
	OK bool `json:"ok"`
	// Message ist der Satz zum Zustand, deutsch, ohne Token.
	Message string `json:"message"`
	// Notes sind die Sätze, mit denen Message hinter dem Satz des Zustands
	// endet, ohne dass sie zu ihm gehören — ein offener Tausch des Tokens
	// (rotate), ein Problem der config von Kephalaion —, je samt eigener
	// Abhilfe. Die Kontextausgabe zeigt Message ganz; getrennt braucht sie
	// nur SplitRemedy.
	Notes []string `json:"-"`
	// Rights sind die Rechte auf die Collection, wie whoami sie nennt (read,
	// write, supersede, vendor/<name>, dir <pfad>/); nur bei ok.
	Rights []string `json:"rights,omitempty"`
	// Scopes sind die Scopes darunter: vendor/<name> und dir <pfad>/.
	Scopes []string `json:"scopes,omitempty"`
	// User ist der User hinter dem Account; nur bei ok.
	User string `json:"user,omitempty"`
	// Node ist der Name dieses Nodes am Hub.
	Node string `json:"node,omitempty"`
	// NodeVersion ist die Version des Nodes.
	NodeVersion string `json:"nodeVersion,omitempty"`
	// Sync ist der Stand des Abgleichs des Hubs, sobald whoami geantwortet hat.
	Sync *Sync `json:"sync,omitempty"`
}

// remedyMarker leitet in einer Meldung die Abhilfe ein.
const remedyMarker = "Abhilfe: "

// SplitRemedy teilt die Meldung für eine Anzeige, die Satz und Abhilfe
// getrennt führt (die Umgebungsprüfung): remedy ist die Abhilfe des
// Zustands — was in seinem Satz hinter „Abhilfe: “ steht, leer ohne —,
// sentence sein Satz davor, gefolgt von den Notes, jede samt ihrer eigenen
// Abhilfe. Geschnitten wird nur im Satz des Zustands: Die Abhilfe einer Note
// (rotate offen, config nicht lesbar) wird nie die Abhilfe eines Zustands,
// zu dem sie nicht gehört.
func (s Status) SplitRemedy() (sentence, remedy string) {
	state, notes := s.Message, []string(nil)
	for i := len(s.Notes) - 1; i >= 0; i-- {
		trimmed, ok := strings.CutSuffix(state, " "+s.Notes[i])
		if !ok {
			break
		}
		state, notes = trimmed, append([]string{s.Notes[i]}, notes...)
	}
	before, after, found := strings.Cut(state, remedyMarker)
	if !found {
		before = state
	}
	return strings.Join(append([]string{strings.TrimSpace(before)}, notes...), " "), strings.TrimSpace(after)
}

// addNote hängt einen Satz an Message an, der nicht zum Zustand gehört, und
// führt ihn in Notes.
func (s *Status) addNote(note string) {
	if note == "" {
		return
	}
	s.Message += " " + note
	s.Notes = append(s.Notes, note)
}

// Check prüft die Anbindung eines Projekts: Hub und Collection aus dem Block
// kephalaion: seiner Konfiguration (.k-playbook/K-PLAYBOOK.yaml, gelesen von
// project.ReadConfig). Sind beide leer, fehlt der Block: not-configured, die
// Meldung nennt den Block und configFile. Sonst CheckBinding. Check scheitert
// nie: Jeder Fehler wird ein Zustand.
func Check(ctx context.Context, binding Binding, configFile string) Status {
	if binding.Hub == "" && binding.Collection == "" {
		return Status{State: StateNotConfigured, Message: NotConfiguredMessage(configFile)}
	}
	return CheckBinding(ctx, binding)
}

// NotConfiguredMessage ist der Satz, wenn der Block kephalaion: in
// configFile fehlt, samt Abhilfe.
func NotConfiguredMessage(configFile string) string {
	return "Keine Collection eingetragen: In " + configFile + " fehlt der Block kephalaion: mit hub und collection. " +
		"Abhilfe: den Block dort eintragen (hub: <alias>, collection: <name>) und committen, " +
		"oder in der Oberfläche einrichten (k-playbook gui, Abschnitt Kephalaion)."
}

// CheckBinding prüft Hub und Collection gegen den Node, den die Auskunft
// kephalaion node client nennt, mit dem Account, den sie für den Hub wählt.
// Vertrag wie im Paketkommentar: Namen nach der Namensregel; Account,
// Token-Datei und Node-Adresse allein aus der Auskunft (ResolveAccess);
// gesendet nur an die Loopback-Adresse. Ein Account der Anbindung wird nicht
// gelesen. Die Zeitgrenze ClientTimeout gilt für die Auskunft,
// DefaultTimeout für die Anfrage am Node.
func CheckBinding(ctx context.Context, binding Binding) Status {
	if err := ValidateBinding(binding); err != nil {
		return Status{Configured: true, State: StateInvalidEntry, Message: "Eintrag ungültig: " + err.Error() + "."}
	}
	access, err := ResolveAccess(ctx, binding.Hub)
	if err != nil {
		status := Status{Configured: true, Hub: binding.Hub, Collection: binding.Collection, Address: binding.Address()}
		var clientErr *ClientError
		if errors.As(err, &clientErr) {
			status.State, status.Message = clientErr.State, clientErr.Message
			status.Notes = append([]string(nil), clientErr.Notes...)
		} else {
			status.State, status.Message = StateClientFailed, err.Error()
		}
		return status
	}
	return checkAccess(ctx, binding, access)
}

// checkAccess prüft Hub und Collection mit einem Zugang aus der Auskunft.
// Ein Hinweis des Zugangs (rotate offen) und ein Problem der config stehen
// hinter der Meldung, gleich welcher Zustand, und in Notes.
func checkAccess(ctx context.Context, binding Binding, access Access) Status {
	status := Status{Configured: true, Hub: binding.Hub, Collection: binding.Collection, Address: binding.Address(),
		Account: access.Account, Choice: access.Choice}
	reply, early := probe(ctx, access)
	if early != nil {
		status.State, status.Message = early.State, early.Message
	} else {
		evaluate(&status, binding, reply)
	}
	status.addNote(access.Note)
	status.addNote(access.ConfigProblem)
	return status
}

// failure ist ein Zustand, der vor der Auswertung von whoami feststeht.
type failure struct {
	State   State
	Message string
}

// probe liest das Token des Zugangs und fragt den Node nach whoami.
func probe(ctx context.Context, access Access) (*whoamiOutput, *failure) {
	token, err := access.readToken()
	if err != nil {
		return nil, &failure{StateTokenMissing, fmt.Sprintf(
			"Die Token-Datei %s für Account %s am Hub %s ist unbrauchbar: %v. "+
				"Abhilfe: kephalaion node account list zeigt ihren Zustand; ist sie kaputt, den Account neu einrichten "+
				"(kephalaion node account setup %s %s mit einem Einrichtungstoken vom Admin).",
			access.tokenFile, access.Account, access.Hub, err, access.Hub, access.Account)}
	}

	ctx, cancel := context.WithTimeout(ctx, DefaultTimeout)
	defer cancel()
	reply, err := callWhoami(ctx, access.Endpoint, access.Hub, access.Account, token)
	if err != nil {
		return nil, &failure{StateNodeUnreachable, redact(
			fmt.Sprintf("Der Node antwortet nicht unter %s: %v.", access.Endpoint, err), token)}
	}
	return reply, nil
}

// hubState wertet den Eintrag eines Hubs in der Antwort aus: alles bis zur
// Anmeldung. ok heißt angemeldet.
func hubState(hub string, reply *whoamiOutput) (entry *whoamiHub, state State, message string) {
	if slices.Contains(reply.UnknownHubs, hub) {
		return nil, StateHubUnknown, fmt.Sprintf(
			"Der Node kennt keinen Hub mit dem Alias %s (kephalaion status nennt die eingetragenen).", hub)
	}
	for i := range reply.Hubs {
		if reply.Hubs[i].Hub == hub {
			entry = &reply.Hubs[i]
			break
		}
	}
	if entry == nil {
		if reply.Hidden {
			return nil, StateLoginInvalid, "Anmeldung ungültig: Der Node hat die Anfrage an keinem Hub angenommen."
		}
		return nil, StateHubUnknown, fmt.Sprintf(
			"Der Node kennt keinen Hub mit dem Alias %s (kephalaion status nennt die eingetragenen).", hub)
	}
	switch entry.Login {
	case "ok":
		return entry, StateOK, ""
	case "invalid":
		if entry.Sync.NeverSynced {
			return entry, StateHubNeverSynced, fmt.Sprintf(
				"Der Node hat den Hub %s noch nie abgeglichen; die Anmeldung lässt sich erst danach prüfen (kephalaion node sync).", hub)
		}
		return entry, StateLoginInvalid, fmt.Sprintf(
			"Anmeldung ungültig: Der Hub %s nimmt Account und Token nicht an (gesperrt, ersetzt oder abgelaufen).", hub)
	default:
		if entry.Sync.LastError == replicaUnreadable {
			return entry, StateReplicaUnreadable, fmt.Sprintf(
				"Die Replica des Hubs %s lässt sich am Node nicht lesen; die Anmeldung ist nicht prüfbar.", hub)
		}
		return entry, StateLoginMissing, fmt.Sprintf(
			"Anmeldung fehlt: Der Node kennt den Account am Hub %s nicht (noch nicht abgeglichen oder nicht angelegt).", hub)
	}
}

// evaluate füllt status aus der Antwort von whoami.
func evaluate(status *Status, binding Binding, reply *whoamiOutput) {
	status.NodeVersion = reply.Version
	entry, state, message := hubState(binding.Hub, reply)
	if entry != nil {
		status.Node = entry.Node
		status.Sync = &Sync{
			NeverSynced: entry.Sync.NeverSynced,
			LastSuccess: entry.Sync.LastSuccess,
			Revision:    entry.Sync.Revision,
			LastError:   entry.Sync.LastError,
		}
	}
	if state != StateOK {
		status.State, status.Message = state, message
		return
	}
	status.User = entry.User
	for _, c := range entry.Collections {
		if c.Collection != binding.Collection || !slices.Contains(c.Rights, "read") {
			continue
		}
		status.State, status.OK = StateOK, true
		status.Rights = append([]string{}, c.Rights...)
		status.Scopes = scopes(c.Rights)
		status.Message = fmt.Sprintf("Angemeldet als %s (User %s); %s ist lesbar.", entry.Account, entry.User, status.Address)
		return
	}
	status.State = StateCollectionUnreadable
	status.Message = fmt.Sprintf("Account %s kann die Collection %s nicht lesen; lesbar sind: %s.",
		entry.Account, status.Address, readableList(binding.Hub, entry.Collections))
}

// scopes sind die Rechte, die einen Bereich eingrenzen: vendor/<name> und
// dir <pfad>/.
func scopes(rights []string) []string {
	var out []string
	for _, r := range rights {
		if strings.HasPrefix(r, "vendor/") || strings.HasPrefix(r, "dir ") {
			out = append(out, r)
		}
	}
	return out
}

func readableList(hub string, collections []whoamiCollection) string {
	var names []string
	for _, c := range collections {
		if slices.Contains(c.Rights, "read") {
			names = append(names, hub+":"+c.Collection)
		}
	}
	if len(names) == 0 {
		return "keine"
	}
	return strings.Join(names, ", ")
}

// Collection ist eine lesbare Collection eines Hubs, wie der
// Einrichtungsschritt sie anbietet.
type Collection struct {
	Collection string   `json:"collection"`
	Address    string   `json:"address"`
	Rights     []string `json:"rights"`
}

// HubProbe ist ein Hub aus der Auskunft mit dem Account, den sie für ihn
// wählt, und dem, was der Node für diesen Account meldet.
type HubProbe struct {
	Hub     string `json:"hub"`
	Account string `json:"account,omitempty"`
	Choice  Choice `json:"choice,omitempty"`
	// State ist StateOK, wenn der Account angemeldet ist; sonst der Zustand,
	// an dem es scheitert — auch schon an der Auskunft (Wahl fehlt, kein
	// Token).
	State       State        `json:"state"`
	Message     string       `json:"message"`
	User        string       `json:"user,omitempty"`
	Collections []Collection `json:"collections"`
}

// ProbeHubs fragt die Auskunft ohne --hub — jeder Hub mit Verzeichnis unter
// tokens/ — und für jeden Hub, für den sie einen Account mit Token-Datei
// nennt, den Node nach den lesbaren Collections. Aus einer Anfrage fließt
// nichts ein. Ein Fehler ist der *ClientError der Auskunft; info trägt dann
// nichts. Die Zeitgrenze DefaultTimeout gilt je Hub; antwortet der Node für
// einen nicht, bekommen die übrigen denselben Zustand, ohne erneut zu warten.
func ProbeHubs(ctx context.Context) (ClientInfo, []HubProbe, error) {
	info, err := QueryClient(ctx)
	if err != nil {
		return ClientInfo{}, nil, err
	}
	probes := make([]HubProbe, 0, len(info.Hubs))
	var unreachable *failure
	for _, entry := range info.Hubs {
		if CheckName("Hub", entry.Alias) != nil {
			continue
		}
		hub := HubProbe{Hub: entry.Alias, Collections: []Collection{}}
		access, err := ResolveInfo(info, entry.Alias)
		if err != nil {
			hub.State, hub.Message = StateClientFailed, err.Error()
			var clientErr *ClientError
			if errors.As(err, &clientErr) {
				hub.State = clientErr.State
			}
			probes = append(probes, hub)
			continue
		}
		hub.Account, hub.Choice = access.Account, access.Choice
		if unreachable != nil {
			hub.State, hub.Message = unreachable.State, unreachable.Message
			probes = append(probes, hub)
			continue
		}
		reply, early := probe(ctx, access)
		if early != nil && early.State == StateNodeUnreachable {
			unreachable = early
		}
		if early != nil {
			hub.State, hub.Message = early.State, early.Message
			probes = append(probes, hub)
			continue
		}
		whoami, state, message := hubState(entry.Alias, reply)
		hub.State, hub.Message = state, message
		if state == StateOK {
			hub.User = whoami.User
			hub.Message = fmt.Sprintf("Angemeldet als %s (User %s).", whoami.Account, whoami.User)
			for _, col := range whoami.Collections {
				if !slices.Contains(col.Rights, "read") {
					continue
				}
				hub.Collections = append(hub.Collections, Collection{
					Collection: col.Collection, Address: entry.Alias + ":" + col.Collection,
					Rights: append([]string{}, col.Rights...),
				})
			}
		}
		if access.Note != "" {
			hub.Message += " " + access.Note
		}
		probes = append(probes, hub)
	}
	return info, probes, nil
}
