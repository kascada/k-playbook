package webui

import (
	"net/http"
	"strconv"

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
// sie allein von der Seite /github — weder die Startseite noch das Menü
// fragen sie. Einen Cache gibt es nicht; wie überall in der Oberfläche liest
// jede Anfrage neu.

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
	return github.NewClient(dir), github.Result{State: github.StateOK}, true
}

// githubOverviewHandler ist die Kopfzeile: Repo, Konto, Recht, CI-Stand des
// Default-Branchs und der letzte Tag.
func githubOverviewHandler(w http.ResponseWriter, r *http.Request) {
	client, result, ok := githubClient()
	if !ok {
		writeJSON(w, http.StatusOK, github.Overview{Result: result})
		return
	}
	writeJSON(w, http.StatusOK, client.FetchOverview(r.Context()))
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

	repo := r.URL.Query().Get("repo")
	if !github.ValidRepo(repo) {
		resolved, resolveResult := client.Repo(r.Context())
		if resolveResult.State != github.StateOK {
			writeJSON(w, http.StatusOK, github.Pulls{Result: resolveResult, Open: []github.PullRequest{}, Closed: []github.PullRequest{}, ClosedLimit: github.ClosedPullRequestLimit})
			return
		}
		repo = resolved
	}
	writeJSON(w, http.StatusOK, client.FetchPulls(r.Context(), repo))
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
	writeJSON(w, http.StatusOK, client.FetchRuns(r.Context()))
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
	writeJSON(w, http.StatusOK, client.FetchFailure(r.Context(), runID))
}
