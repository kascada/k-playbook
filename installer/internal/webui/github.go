package webui

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/kascada/k-playbook/installer/internal/github"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// Die GitHub-Ansicht liest den Stand des Repos über das gh-Binary. Sie
// schreibt nichts nach GitHub: Approve, Merge und Kommentare bleiben bei
// /k-pr-review, das die Seite nur als Befehl zum Kopieren nennt.
//
// Je Karte ein eigener Endpunkt. Der Grund ist derselbe wie bei
// /api/mcp/tools: dahinter steht ein Subprozess mit Netzzugriff, und eine
// langsame Abfrage soll die übrigen Karten nicht aufhalten. Ausgelöst werden
// sie allein von der Seite /github — weder die Statusseite noch das Menü
// fragen sie. Einen Cache gibt es nicht; wie überall in der Oberfläche liest
// jede Anfrage neu.

// githubBudgets ist die zugesagte Frist je Endpunkt: nach ihr ist die Anfrage
// beendet, egal wie viele Subprozesse sie dafür startet. Die Frist je
// Subprozess allein reichte nicht — die Kopfzeile ruft sieben nacheinander auf,
// und jeder hätte seine eigenen 20 s bekommen.
//
// Das Budget steht hier und nicht im Paket github: es ist eine Zusage an die
// HTTP-Anfrage, und nur der Handler hat deren Kontext. Die Fetch-Funktionen
// nehmen jeden Kontext und bleiben von der Oberfläche unabhängig; jeder andere
// Aufrufer setzt seine eigene Frist. Die Werte selbst sind die dokumentierten
// Fristen des Pakets, damit Doku, Paket und Endpunkt dieselbe Zahl nennen.
//
// Eine Variable, keine Konstante — wie mcpProbeTimeout: nur so kann ein Test
// den Weg über die abgelaufene Frist gehen, ohne 20 s zu warten.
var githubBudgets = struct {
	Overview time.Duration
	Pulls    time.Duration
	Runs     time.Duration
	Failure  time.Duration
}{
	Overview: github.DefaultTimeout,
	Pulls:    github.DefaultTimeout,
	Runs:     github.DefaultTimeout,
	Failure:  github.LogTimeout,
}

// newGitHubClient baut den Client, nachdem die Vorprüfung durch ist. Die
// einzige Naht für Handler-Tests: sie setzen einen Runner ein, der blockiert
// oder langsam antwortet, und prüfen die Frist am Endpunkt.
var newGitHubClient = github.NewClient

// writeGitHubJSON schreibt die Antwort eines Endpunkts — außer, der Browser
// hat die Anfrage abgebrochen. Dann hört niemand mehr zu, und die Antwort
// trüge ohnehin keinen Stand, sondern nur den Abbruch. Still heißt hier: keine
// Antwort und kein Eintrag im Log; ein geschlossener Tab ist kein Fehler.
// Geprüft wird der Kontext der Anfrage selbst, nicht der abgeleitete: dessen
// abgelaufenes Budget ist ein Zeitfehler und wird beantwortet.
func writeGitHubJSON(w http.ResponseWriter, r *http.Request, payload any) {
	if errors.Is(r.Context().Err(), context.Canceled) {
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

// githubClient prüft die Voraussetzungen und liefert den Client dazu.
//
// Geprüft wird **vor** jedem Netzaufruf, und zwar aus Dateien: die
// Projektentscheidung aus tools.gh.status und der netzfreie Host-Befund aus
// project.DetectGH. Steht die Entscheidung nicht auf enabled, ruft der Server
// gh gar nicht auf — das ist die Zusage des Zustands „disabled".
func githubClient() (*github.Client, github.Result, bool) {
	environment := project.Detect()
	if !environment.Installed {
		return nil, github.Fail(github.StateNoProject), false
	}

	state, err := project.GHState(environment.ProjectDir)
	if err != nil {
		// Der Fehlertext von gh wird eingeordnet statt durchgereicht; für einen
		// Fehler an der eigenen Konfiguration gilt dasselbe. Der Dateiname sagt,
		// wo nachzusehen ist — die rohe Meldung gehört ins Log, nicht in die Karte.
		return nil, github.Result{
			State:   github.StateError,
			Message: "Die Datei " + project.ConfigFileName + " ließ sich nicht lesen. Dort steht, ob die Oberfläche gh nutzen darf.",
		}, false
	}

	switch state.Status {
	case project.GHDisabled:
		return nil, github.Fail(github.StateDisabled), false
	case project.GHUnknown:
		return nil, github.Fail(github.StateUndecided), false
	}
	if !state.Installed {
		return nil, github.Fail(github.StateNotInstalled), false
	}
	if !state.LoggedIn {
		return nil, github.Fail(github.StateNotLoggedIn), false
	}

	// Gearbeitet wird in der Repo-Wurzel, nicht im Hauptverzeichnis: aus ihr
	// löst gh das Remote auf, und die Tag-Abfragen sind git-Aufrufe.
	dir := environment.ProjectDir
	if config, err := project.ReadConfig(environment.ProjectDir); err == nil {
		dir = project.RepoRootDir(environment.ProjectDir, config)
	}
	return newGitHubClient(dir), github.Result{State: github.StateOK}, true
}

// githubOverviewHandler ist die Kopfzeile: Repo, Konto, Recht, CI-Stand des
// Default-Branchs und der letzte Tag.
func githubOverviewHandler(w http.ResponseWriter, r *http.Request) {
	client, result, ok := githubClient()
	if !ok {
		writeJSON(w, http.StatusOK, github.Overview{Result: result})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), githubBudgets.Overview)
	defer cancel()
	writeGitHubJSON(w, r, client.FetchOverview(ctx))
}

// githubPullsHandler ist die PR-Karte: offene oben, die letzten
// geschlossenen und gemergten darunter.
//
// `repo` reicht die Seite aus der Kopfzeile durch — sie hat es dort schon
// aufgelöst, und ein zweiter `gh repo view` wäre ein zweiter Netzaufruf für
// dieselbe Auskunft. Fehlt der Parameter oder hat er nicht die Form
// `owner/name`, löst der Server selbst auf: der Endpunkt muss auch ohne die
// Seite antworten.
func githubPullsHandler(w http.ResponseWriter, r *http.Request) {
	client, result, ok := githubClient()
	if !ok {
		writeJSON(w, http.StatusOK, github.Pulls{Result: result, Open: []github.PullRequest{}, Closed: []github.PullRequest{}, ClosedLimit: github.ClosedPullRequestLimit})
		return
	}

	// Auflösen und Abfrage teilen sich ein Budget: die Frist gilt für den
	// Endpunkt, nicht je Aufruf.
	ctx, cancel := context.WithTimeout(r.Context(), githubBudgets.Pulls)
	defer cancel()

	repo := r.URL.Query().Get("repo")
	if !github.ValidRepo(repo) {
		resolved, resolveResult := client.Repo(ctx)
		if resolveResult.State != github.StateOK {
			writeGitHubJSON(w, r, github.Pulls{Result: resolveResult, Open: []github.PullRequest{}, Closed: []github.PullRequest{}, ClosedLimit: github.ClosedPullRequestLimit})
			return
		}
		repo = resolved
	}
	writeGitHubJSON(w, r, client.FetchPulls(ctx, repo))
}

// githubRunsHandler ist die Lauf-Karte: die letzten Läufe mit Branch oder Tag,
// Ereignis, Dauer und Ergebnis — **ohne** Logs. Die Ursache holt erst
// githubRunFailureHandler.
func githubRunsHandler(w http.ResponseWriter, r *http.Request) {
	client, result, ok := githubClient()
	if !ok {
		writeJSON(w, http.StatusOK, github.Runs{Result: result, Runs: []github.Run{}, Limit: github.RunLimit})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), githubBudgets.Runs)
	defer cancel()
	writeGitHubJSON(w, r, client.FetchRuns(ctx))
}

// githubRunFailureHandler ist die Ursache eines roten Laufs.
//
// Eigener Endpunkt, weil das Log die teuerste Abfrage der Seite ist: gh lädt
// das Archiv der fehlgeschlagenen Jobs. Die Seite ruft ihn erst beim
// Aufklappen eines roten Laufs, nie für alle Läufe beim Laden.
func githubRunFailureHandler(w http.ResponseWriter, r *http.Request) {
	client, result, ok := githubClient()
	if !ok {
		writeJSON(w, http.StatusOK, github.Failure{Result: result, Groups: []github.FailureGroup{}, Lines: []string{}})
		return
	}

	runID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || runID <= 0 {
		// Eine Kennung, die keine ist, geht nicht als Argument in einen
		// Subprozess.
		http.NotFound(w, r)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), githubBudgets.Failure)
	defer cancel()
	writeGitHubJSON(w, r, client.FetchFailure(ctx, runID))
}
