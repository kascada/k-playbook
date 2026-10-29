package kephalaion

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

// State ist das Ergebnis einer Prüfung. Jeder Zustand hat seinen eigenen
// Satz (Status.Message); die Reihenfolge der Konstanten ist die Reihenfolge,
// in der Check prüft.
type State string

const (
	// StateNotConfigured: Datei oder Block kephalaion: fehlt.
	StateNotConfigured State = "not-configured"
	// StateInvalidEntry: Ein Name verstößt gegen die Namensregel, oder die
	// Datei lässt sich nicht lesen.
	StateInvalidEntry State = "invalid-entry"
	// StateNoKephalaion: Es gibt keine config von Kephalaion.
	StateNoKephalaion State = "kephalaion-missing"
	// StateNodeAddress: Node-Adresse fehlt oder ist keine Loopback-Adresse.
	StateNodeAddress State = "node-address"
	// StateTokenMissing: Kein Kandidat passt zu Hub und Account, oder seine
	// Datei ist unbrauchbar.
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
	// Address ist <hub>:<collection>, sobald beide Namen gültig sind.
	Address string `json:"address"`
	State   State  `json:"state"`
	// OK ist State == StateOK, für Leser, die nur das wissen wollen.
	OK bool `json:"ok"`
	// Message ist der Satz zum Zustand, deutsch, ohne Token.
	Message string `json:"message"`
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

// Check liest die Anbindung aus localDir und prüft sie gegen den Node, den
// env beschreibt. Check scheitert nie: Jeder Fehler wird ein Zustand. Die
// Zeitgrenze DefaultTimeout gilt für die ganze Prüfung.
func Check(ctx context.Context, localDir string, env Env) Status {
	binding, configured, err := ReadBinding(localDir)
	if err != nil {
		return Status{Configured: true, State: StateInvalidEntry, Message: err.Error() + "."}
	}
	if !configured {
		return Status{State: StateNotConfigured,
			Message: "Keine Collection eingetragen: In " + LocalFileName + " fehlt der Block kephalaion:."}
	}
	return CheckBinding(ctx, binding, env)
}

// CheckBinding prüft eine Anbindung — aus der Datei oder aus einer Anfrage —
// gegen den Node, den env beschreibt. Vertrag wie im Paketkommentar: Namen
// nach der Namensregel, das Paar genau ein Kandidat, das Token aus dessen
// Pfad, gesendet nur an die Loopback-Adresse aus env.
func CheckBinding(ctx context.Context, binding Binding, env Env) Status {
	status := Status{Configured: true}
	if err := ValidateBinding(binding); err != nil {
		status.State = StateInvalidEntry
		status.Message = "Eintrag ungültig: " + err.Error() + "."
		return status
	}
	status.Hub, status.Collection, status.Account = binding.Hub, binding.Collection, binding.Account
	status.Address = binding.Address()

	reply, early := probe(ctx, binding.Hub, binding.Account, env)
	if early != nil {
		status.State, status.Message = early.State, early.Message
		return status
	}
	evaluate(&status, binding, reply)
	return status
}

// failure ist ein Zustand, der vor der Auswertung von whoami feststeht.
type failure struct {
	State   State
	Message string
}

// probe führt die Schritte bis zur Antwort von whoami aus: config, Adresse,
// Kandidat, Token, Anfrage. Hub und Account müssen schon geprüft sein.
func probe(ctx context.Context, hub, account string, env Env) (*whoamiOutput, *failure) {
	if problem := env.ConfigProblem(); problem != "" {
		return nil, &failure{StateNoKephalaion, problem}
	}
	if env.Endpoint == "" {
		return nil, &failure{StateNodeAddress, "Keine zulässige Adresse des Nodes: " + env.AddressProblem}
	}
	candidate, ok := env.FindCandidate(hub, account)
	if !ok {
		return nil, &failure{StateTokenMissing, tokenMissingMessage(env, hub, account)}
	}
	token, err := candidate.readToken()
	if err != nil {
		return nil, &failure{StateTokenMissing, fmt.Sprintf(
			"Die Token-Datei für Account %s am Hub %s ist unbrauchbar: %v.", account, hub, err)}
	}

	ctx, cancel := context.WithTimeout(ctx, DefaultTimeout)
	defer cancel()
	reply, err := callWhoami(ctx, env.Endpoint, candidate, token)
	if err != nil {
		return nil, &failure{StateNodeUnreachable, redact(
			fmt.Sprintf("Der Node antwortet nicht unter %s: %v.", env.Endpoint, err), token)}
	}
	return reply, nil
}

// ConfigProblem sagt in einem Satz, warum die config von Kephalaion nicht
// trägt — keine da oder nicht lesbar —, mit den Pfaden, an denen gesucht
// bzw. gelesen wurde; leer, wenn sie gelesen ist. Ein Inhalt der Datei steht
// nie darin.
func (e Env) ConfigProblem() string {
	if !e.ConfigPresent {
		return "Kephalaion ist auf diesem Rechner nicht eingerichtet: " + e.missingConfig() + "."
	}
	if e.ConfigError != "" {
		return "Die config von Kephalaion unter " + e.ConfigPath + " ist nicht lesbar: " + e.ConfigError + "."
	}
	return ""
}

// missingConfig nennt die Orte, an denen die Suche keine config fand. Bei
// KEPHALAION_CONFIG ist das nur dieser Pfad: Die Variable schließt die
// übrigen Orte aus.
func (e Env) missingConfig() string {
	if e.ConfigSource == SourceEnv {
		return "keine config unter " + e.ConfigPath + " (Quelle " + EnvConfigVar + ")"
	}
	system := filepath.Clean(SystemConfigPath)
	if e.UserDir == "" {
		return "keine config unter " + system + ", und die des Users ist unbekannt, weil das Heimatverzeichnis unbekannt ist"
	}
	return "keine config unter " + filepath.Join(e.UserDir, "config.yaml") + " und keine unter " + system
}

// tokenMissingMessage nennt das Token-Verzeichnis des Users, in dem die
// Datei des Paars fehlt.
func tokenMissingMessage(env Env, hub, account string) string {
	dir := env.TokensDir()
	if dir == "" {
		return fmt.Sprintf("Keine Token-Datei für Account %s am Hub %s: Das Token-Verzeichnis ist unbekannt, weil das Heimatverzeichnis unbekannt ist.",
			account, hub)
	}
	return fmt.Sprintf("Keine Token-Datei für Account %s am Hub %s: %s/%s.token fehlt im Token-Verzeichnis %s.",
		account, hub, hub, account, dir)
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

// Collection ist eine lesbare Collection eines Paars, wie der
// Einrichtungsschritt sie anbietet.
type Collection struct {
	Collection string   `json:"collection"`
	Address    string   `json:"address"`
	Rights     []string `json:"rights"`
}

// Pair ist ein Kandidat mit dem, was der Node für ihn meldet.
type Pair struct {
	Hub     string `json:"hub"`
	Account string `json:"account"`
	// State ist StateOK, wenn der Account angemeldet ist; sonst der Zustand,
	// an dem es scheitert.
	State       State        `json:"state"`
	Message     string       `json:"message"`
	User        string       `json:"user,omitempty"`
	Collections []Collection `json:"collections"`
}

// ProbePairs fragt den Node für jeden Kandidaten nach den lesbaren
// Collections. Die Kandidaten kommen allein aus dem Token-Verzeichnis; aus
// einer Anfrage fließt nichts ein. Die Zeitgrenze gilt je Kandidat.
func ProbePairs(ctx context.Context, env Env) []Pair {
	candidates, _ := env.Candidates()
	pairs := make([]Pair, 0, len(candidates))
	// Antwortet der Node für einen Kandidaten nicht, antwortet er auch für die
	// übrigen nicht; sie bekommen denselben Zustand, ohne erneut zu warten.
	var unreachable *failure
	for _, c := range candidates {
		pair := Pair{Hub: c.Hub, Account: c.Account, Collections: []Collection{}}
		if unreachable != nil {
			pair.State, pair.Message = unreachable.State, unreachable.Message
			pairs = append(pairs, pair)
			continue
		}
		reply, early := probe(ctx, c.Hub, c.Account, env)
		if early != nil && early.State == StateNodeUnreachable {
			unreachable = early
		}
		if early != nil {
			pair.State, pair.Message = early.State, early.Message
			pairs = append(pairs, pair)
			continue
		}
		entry, state, message := hubState(c.Hub, reply)
		pair.State, pair.Message = state, message
		if state == StateOK {
			pair.User = entry.User
			pair.Message = fmt.Sprintf("Angemeldet als %s (User %s).", entry.Account, entry.User)
			for _, col := range entry.Collections {
				if !slices.Contains(col.Rights, "read") {
					continue
				}
				pair.Collections = append(pair.Collections, Collection{
					Collection: col.Collection, Address: c.Hub + ":" + col.Collection,
					Rights: append([]string{}, col.Rights...),
				})
			}
		}
		pairs = append(pairs, pair)
	}
	return pairs
}
