package github

import (
	"context"
	"errors"
	"strings"
)

// State ist der Zustand einer Abfrage. Feste Werte statt freier Texte: die
// Oberfläche erklärt jeden Zustand selbst, und kein Fehlertext von gh wird roh
// durchgereicht.
type State string

const (
	// StateOK: die Abfrage hat geantwortet.
	StateOK State = "ok"
	// StateNoProject: es gibt keine K-PLAYBOOK.yaml und damit kein Projekt,
	// auf das sich die Ansicht beziehen könnte.
	StateNoProject State = "no-project"
	// StateDisabled: das Projekt hat entschieden, gh nicht zu nutzen
	// (tools.gh.status = disabled). Der Server ruft gh dann gar nicht auf.
	StateDisabled State = "disabled"
	// StateUndecided: tools.gh.status steht auf unknown. Ausdrücklicher
	// Zustand, kein stillschweigendes Nein — die Entscheidung fällt auf der
	// Startseite.
	StateUndecided State = "undecided"
	// StateNotInstalled: gh liegt nicht im PATH.
	StateNotInstalled State = "not-installed"
	// StateNotLoggedIn: gh ist da, aber es ist kein Konto hinterlegt.
	StateNotLoggedIn State = "not-logged-in"
	// StateBadCredentials: gh ist konfiguriert, die API antwortet aber mit 401.
	// Getrennt von StateNotLoggedIn, weil project.DetectGH „angemeldet" allein
	// aus der gh-Konfiguration liest und den Token nicht prüft: ein
	// abgelaufener oder zurückgezogener Token sieht dort aus wie eine
	// Anmeldung.
	StateBadCredentials State = "bad-credentials"
	// StateNoRemote: das Verzeichnis ist kein Repo mit GitHub-Remote.
	StateNoRemote State = "no-remote"
	// StateNoAccess: 403 oder 404 auf das Repo — kein Zugriff, oder es gibt es
	// unter diesem Namen nicht.
	StateNoAccess State = "no-access"
	// StateRateLimited: das Kontingent der API ist aufgebraucht.
	StateRateLimited State = "rate-limited"
	// StateNetwork: kein Netz, DNS- oder TLS-Fehler.
	StateNetwork State = "network"
	// StateTimeout: die Frist ist abgelaufen.
	StateTimeout State = "timeout"
	// StateError: alles Übrige. Der Text sagt, was gh gemeldet hat, gekürzt
	// auf die erste Zeile.
	StateError State = "error"
)

// Result ist der gemeinsame Kopf jeder Antwort dieses Pakets: der Zustand und
// der Satz, der ihn erklärt. Leer bleibt Message nur bei StateOK.
type Result struct {
	State   State  `json:"state"`
	Message string `json:"message"`
}

// stateMessages ist der erklärende Satz je Zustand. Jeder Zustand ohne Daten
// ist erklärt statt leer — das ist die Zusage der Ansicht.
var stateMessages = map[State]string{
	StateNoProject:      "Keine K-PLAYBOOK.yaml gefunden. Ohne Projekt gibt es kein Repo, dessen Stand hier stehen könnte.",
	StateDisabled:       "Dieses Projekt nutzt gh nicht (tools.gh.status: disabled). Die Entscheidung steht auf der Startseite im Block „GitHub CLI\".",
	StateUndecided:      "Für dieses Projekt ist noch nicht entschieden, ob gh genutzt wird (tools.gh.status: unknown). Die Entscheidung steht auf der Startseite im Block „GitHub CLI\".",
	StateNotInstalled:   "gh liegt nicht im PATH. Ohne das Binary kann diese Seite nichts abfragen.",
	StateNotLoggedIn:    "gh ist installiert, aber kein Konto hinterlegt. Anmelden im Terminal mit „gh auth login\".",
	StateBadCredentials: "gh ist angemeldet, die API weist den Token aber ab (401). Er ist abgelaufen oder zurückgezogen; im Terminal mit „gh auth login\" neu anmelden.",
	StateNoRemote:       "Kein GitHub-Remote gefunden. Diese Seite zeigt den Stand des Repos, zu dem das Arbeitsverzeichnis gehört.",
	StateNoAccess:       "Kein Zugriff auf das Repo (403 oder 404). Entweder fehlt dem angemeldeten Konto das Recht, oder es gibt das Repo unter diesem Namen nicht.",
	StateRateLimited:    "Das API-Kontingent von GitHub ist aufgebraucht. Später erneut laden.",
	StateNetwork:        "GitHub war nicht erreichbar. Netzverbindung prüfen.",
	StateTimeout:        "Die Abfrage hat zu lange gedauert und wurde abgebrochen.",
	StateError:          "Die Abfrage ist fehlgeschlagen.",
}

// Explain ist der Satz zu einem Zustand.
func Explain(state State) string {
	if message, ok := stateMessages[state]; ok {
		return message
	}
	return ""
}

// Fail baut ein Result aus einem Zustand samt Erklärung.
func Fail(state State) Result {
	return Result{State: state, Message: Explain(state)}
}

// Classify ordnet einen Fehler von gh einem festen Zustand zu. Der Fehlertext
// wird gelesen, aber nicht durchgereicht: was die Seite zeigt, ist der Satz aus
// stateMessages. Nur bei StateError steht die erste Zeile von gh dahinter,
// weil es sonst nichts zu sagen gäbe.
func Classify(err error) Result {
	if err == nil {
		return Result{State: StateOK}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return Fail(StateTimeout)
	}

	text := errorText(err)
	lower := strings.ToLower(text)

	switch {
	case strings.Contains(lower, "executable file not found"), strings.Contains(lower, "exec: \"gh\""):
		return Fail(StateNotInstalled)
	case strings.Contains(lower, "signal: killed"), strings.Contains(lower, "context deadline exceeded"):
		return Fail(StateTimeout)
	case strings.Contains(lower, "bad credentials"), strings.Contains(lower, "http 401"):
		return Fail(StateBadCredentials)
	case strings.Contains(lower, "gh auth login"), strings.Contains(lower, "not logged into"), strings.Contains(lower, "no git remotes found"), strings.Contains(lower, "authentication token not found"):
		return authOrRemote(lower)
	case strings.Contains(lower, "api rate limit exceeded"), strings.Contains(lower, "secondary rate limit"), strings.Contains(lower, "was submitted too quickly"):
		return Fail(StateRateLimited)
	case strings.Contains(lower, "http 403"), strings.Contains(lower, "must have admin rights"), strings.Contains(lower, "resource not accessible"):
		return Fail(StateNoAccess)
	case strings.Contains(lower, "http 404"), strings.Contains(lower, "could not resolve to a repository"), strings.Contains(lower, "not found"):
		return Fail(StateNoAccess)
	case strings.Contains(lower, "none of the git remotes"), strings.Contains(lower, "no such remote"), strings.Contains(lower, "not a git repository"):
		return Fail(StateNoRemote)
	case strings.Contains(lower, "dial tcp"), strings.Contains(lower, "no such host"), strings.Contains(lower, "connection refused"),
		strings.Contains(lower, "network is unreachable"), strings.Contains(lower, "tls"), strings.Contains(lower, "i/o timeout"),
		strings.Contains(lower, "eof"):
		return Fail(StateNetwork)
	}

	result := Fail(StateError)
	if line := firstLine(text); line != "" {
		result.Message += " gh meldet: " + line
	}
	return result
}

// authOrRemote trennt die zwei Fälle, die gh mit demselben Hinweis auf
// `gh auth login` quittiert: fehlendes Remote und fehlende Anmeldung.
func authOrRemote(lower string) Result {
	if strings.Contains(lower, "no git remotes found") || strings.Contains(lower, "none of the git remotes") {
		return Fail(StateNoRemote)
	}
	return Fail(StateNotLoggedIn)
}

func errorText(err error) string {
	var commandErr *CommandError
	if errors.As(err, &commandErr) {
		if text := strings.TrimSpace(commandErr.Stderr); text != "" {
			return text
		}
	}
	return err.Error()
}

func firstLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// gh setzt seine Fehlerzeilen mit einem Kreuz voran; das gehört nicht
		// in die Oberfläche.
		line = strings.TrimSpace(strings.TrimPrefix(line, "X"))
		if line != "" {
			return line
		}
	}
	return ""
}
