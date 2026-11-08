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
// schreibt nichts nach GitHub.
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

// newGitHubClient baut den Client, nachdem die Vorprüfung durch ist; env ist
// die Kindumgebung eines Dienstes oder nil. Die Naht für Handler-Tests: sie
// setzen einen Runner ein, der blockiert oder langsam antwortet, und prüfen
// die Frist am Endpunkt — oder zählen, ob überhaupt ein Client entsteht.
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

// githubAccess ist das Ergebnis der Vorprüfung: der Client, wenn gh starten
// darf, sonst der Zustand; dazu, worüber gelesen wird.
type githubAccess struct {
	client *github.Client
	result github.Result
	via    *github.Via
	// viaAccount heißt: ohne Dienst, der bisherige Weg über das aktive
	// gh-Konto. Scheitert er, nennt die Seite die Zugangsdatei als Abhilfe.
	viaAccount bool
}

// accessFileHint ist der Satz, den die Fehlerzustände ohne Dienst tragen:
// not-installed, not-logged-in, bad-credentials. Die Seite setzt den Verweis
// auf AccessPage daneben.
const accessFileHint = " Abhilfe ohne das aktive gh-Konto: eine GitHub-Zugangsdatei mit einem Dienst für dieses Repo — dann liest die Seite über dessen Zugang. Auf der Seite GitHub-Zugänge legt „Anlegen“ die Vorlage an; beim Einrichten hilft eine KI-Sitzung."

// hint ergänzt den Zustand einer Antwort ohne Dienst um die Zugangsdatei als
// Abhilfe, wenn der bisherige Weg an gh oder seiner Anmeldung scheitert.
func (a githubAccess) hint(result *github.Result) {
	if !a.viaAccount {
		return
	}
	switch result.State {
	case github.StateNotInstalled, github.StateNotLoggedIn, github.StateBadCredentials:
		if result.AccessLink == "" {
			result.Message += accessFileHint
			result.AccessLink = github.AccessPage
		}
	}
}

// resolveGitHubRoute ist der Weg über einen Dienst der Zugangsdatei; eine
// Variable, damit Tests zählen können, wann er bestimmt wird.
var resolveGitHubRoute = project.ResolveGitHubRoute

// githubClient prüft die Voraussetzungen und liefert den Client dazu.
//
// Geprüft wird **vor** jedem Netzaufruf, und zwar aus Dateien: die
// Projektentscheidung aus tools.gh.status und der netzfreie Host-Befund aus
// project.DetectGH. Steht die Entscheidung nicht auf enabled, ruft der Server
// gh gar nicht auf — das ist die Zusage des Zustands „disabled".
//
// Danach der Weg (Task 126): Nennt ein Dienst der Zugangsdatei das Repo,
// läuft gh mit dessen Kindumgebung, gewählt wie bei `k-playbook github run`
// (project.ResolveGitHubRoute). Hat er keinen api-Zugang oder gar keinen
// verfügbaren, oder ist die Datei fehlerhaft, startet kein gh und es gibt
// keinen Rückfall auf das aktive Konto. Ohne Dienst gilt das Bisherige. ctx
// ist das Budget der Anfrage; das Token liest erst Environment darin.
func githubClient(ctx context.Context) githubAccess {
	return githubClientRoute(ctx, nil, false)
}

// githubClientRoute ist githubClient mit einem schon bestimmten Weg, wenn
// resolved wahr ist: Die Branches-Seite bestimmt ihn einmal je Anfrage und
// braucht ihn auch für den Fetch und die Zeile der Seite.
func githubClientRoute(ctx context.Context, route *project.GitHubRoute, resolved bool) githubAccess {
	environment := project.Detect()
	if !environment.Installed {
		return githubAccess{result: github.Fail(github.StateNoProject)}
	}

	state, err := project.GHState(environment.ProjectDir)
	if err != nil {
		// Der Fehlertext von gh wird eingeordnet statt durchgereicht; für einen
		// Fehler an der eigenen Konfiguration gilt dasselbe. Der Dateiname sagt,
		// wo nachzusehen ist — die rohe Meldung gehört ins Log, nicht in die Karte.
		return githubAccess{result: github.Result{
			State:   github.StateError,
			Message: "Die Datei " + project.ConfigFileName + " ließ sich nicht lesen. Dort steht, ob die Oberfläche gh nutzen darf.",
		}}
	}

	switch state.Status {
	case project.GHDisabled:
		return githubAccess{result: github.Fail(github.StateDisabled)}
	case project.GHUnknown:
		return githubAccess{result: github.Fail(github.StateUndecided)}
	}

	// Gearbeitet wird in der Repo-Wurzel, nicht im Hauptverzeichnis: aus ihr
	// löst gh das Remote auf, und die Tag-Abfragen sind git-Aufrufe.
	dir := environment.ProjectDir
	if config, err := project.ReadConfig(environment.ProjectDir); err == nil {
		dir = project.RepoRootDir(environment.ProjectDir, config)
	}

	if !resolved {
		route = resolveGitHubRoute(ctx, dir)
	}
	if route != nil {
		return serviceAccess(ctx, route, state, dir)
	}

	access := githubAccess{viaAccount: true, via: accountVia(state)}
	switch {
	case !state.Installed:
		access.result = github.Fail(github.StateNotInstalled)
	case !state.LoggedIn:
		access.result = github.Fail(github.StateNotLoggedIn)
	default:
		access.client = newGitHubClient(dir, nil)
		access.result = github.Result{State: github.StateOK}
		return access
	}
	access.hint(&access.result)
	return access
}

// serviceAccess ist die Vorprüfung mit einem Dienst: kein gh ohne api-Zugang,
// ohne verfügbaren Zugang und bei fehlerhafter Datei; sonst ein Client mit
// der Kindumgebung des Dienstes.
func serviceAccess(ctx context.Context, route *project.GitHubRoute, state project.GH, dir string) githubAccess {
	failed := func(githubState github.State, message string) githubAccess {
		access := githubAccess{result: github.Result{State: githubState, Message: message, AccessLink: github.AccessPage}}
		if route.State != project.GitHubRouteFileError {
			access.via = serviceVia(route)
		}
		return access
	}
	switch route.State {
	case project.GitHubRouteFileError:
		return failed(github.StateAccessFileError, route.Message)
	case project.GitHubRouteNoAccess:
		return failed(github.StateServiceNoAccess, route.Message)
	case project.GitHubRouteNoAPI:
		return failed(github.StateServiceNoAPI, route.Message)
	}
	if !state.Installed {
		access := githubAccess{result: github.Fail(github.StateNotInstalled), via: serviceVia(route)}
		return access
	}
	env, err := route.Environment(ctx)
	if err != nil {
		return failed(github.StateServiceSetup, err.Error())
	}
	return githubAccess{client: newGitHubClient(dir, env), result: github.Result{State: github.StateOK}, via: serviceVia(route)}
}

// serviceVia nennt Dienst und Zugang in der Form der Zeile von run.
func serviceVia(route *project.GitHubRoute) *github.Via {
	return &github.Via{Kind: github.ViaService, Service: route.Service, Line: route.Line, Note: route.Note}
}

// accountVia nennt ohne Dienst das aktive gh-Konto — oder, wenn gh den Token
// aus der Umgebung des Servers nimmt, das.
func accountVia(state project.GH) *github.Via {
	line := "aktives gh-Konto " + state.Account
	switch {
	case state.TokenFromEnv:
		line = "GH_TOKEN/GITHUB_TOKEN aus der Umgebung des Servers"
	case state.Account == "":
		line = "aktives gh-Konto (keins hinterlegt)"
	}
	return &github.Via{Kind: github.ViaAccount, Line: line, Note: accountNote}
}

// accountNote ist der Satz zum Weg ohne Dienst.
const accountNote = "Kein Dienst der GitHub-Zugangsdatei nennt dieses Repo; gelesen wird wie bisher über gh selbst."

// githubOverviewHandler ist die Kopfzeile: Repo, Konto, Recht, CI-Stand des
// Default-Branchs und der letzte Tag.
func githubOverviewHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), githubBudgets.Overview)
	defer cancel()
	access := githubClient(ctx)
	if access.client == nil {
		writeGitHubJSON(w, r, github.Overview{Result: access.result, Via: access.via})
		return
	}
	overview := access.client.FetchOverview(ctx)
	overview.Via = access.via
	access.hint(&overview.Result)
	writeGitHubJSON(w, r, overview)
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
	// Vorprüfung, Auflösen und Abfrage teilen sich ein Budget: die Frist gilt
	// für den Endpunkt, nicht je Aufruf.
	ctx, cancel := context.WithTimeout(r.Context(), githubBudgets.Pulls)
	defer cancel()
	access := githubClient(ctx)
	if access.client == nil {
		writeGitHubJSON(w, r, github.Pulls{Result: access.result, Open: []github.PullRequest{}, Closed: []github.PullRequest{}, ClosedLimit: github.ClosedPullRequestLimit})
		return
	}

	repo := r.URL.Query().Get("repo")
	if !github.ValidRepo(repo) {
		resolved, resolveResult := access.client.Repo(ctx)
		if resolveResult.State != github.StateOK {
			access.hint(&resolveResult)
			writeGitHubJSON(w, r, github.Pulls{Result: resolveResult, Open: []github.PullRequest{}, Closed: []github.PullRequest{}, ClosedLimit: github.ClosedPullRequestLimit})
			return
		}
		repo = resolved
	}
	pulls := access.client.FetchPulls(ctx, repo)
	access.hint(&pulls.Result)
	writeGitHubJSON(w, r, pulls)
}

// githubRunsHandler ist die Lauf-Karte: die letzten Läufe mit Branch oder Tag,
// Ereignis, Dauer und Ergebnis — **ohne** Logs. Die Ursache holt erst
// githubRunFailureHandler.
func githubRunsHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), githubBudgets.Runs)
	defer cancel()
	access := githubClient(ctx)
	if access.client == nil {
		writeGitHubJSON(w, r, github.Runs{Result: access.result, Runs: []github.Run{}, Limit: github.RunLimit})
		return
	}
	runs := access.client.FetchRuns(ctx)
	access.hint(&runs.Result)
	writeGitHubJSON(w, r, runs)
}

// githubRunFailureHandler ist die Ursache eines roten Laufs.
//
// Eigener Endpunkt, weil das Log die teuerste Abfrage der Seite ist: gh lädt
// das Archiv der fehlgeschlagenen Jobs. Die Seite ruft ihn erst beim
// Aufklappen eines roten Laufs, nie für alle Läufe beim Laden.
func githubRunFailureHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), githubBudgets.Failure)
	defer cancel()
	access := githubClient(ctx)
	if access.client == nil {
		writeGitHubJSON(w, r, github.Failure{Result: access.result, Groups: []github.FailureGroup{}, Lines: []string{}})
		return
	}

	runID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || runID <= 0 {
		// Eine Kennung, die keine ist, geht nicht als Argument in einen
		// Subprozess.
		http.NotFound(w, r)
		return
	}
	failure := access.client.FetchFailure(ctx, runID)
	access.hint(&failure.Result)
	writeGitHubJSON(w, r, failure)
}
