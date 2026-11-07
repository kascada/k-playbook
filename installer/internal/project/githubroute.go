package project

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/githubaccess"
)

// Der Weg der Oberfläche zu GitHub (Task 126, docs/github-access.md, Abschnitt
// In the interface): Hat das Repo einen Dienst in der persönlichen
// Zugangsdatei, laufen gh und git fetch über dessen Zugang — gewählt wie bei
// `k-playbook github run` und mit demselben Code: Load, Judge, Choose,
// ReadToken, ChildEnv, RunLine. Den Dienst bestimmt das Remote wie im Feld
// context.github (projectRemoteURL, githubRemoteRepo). Ohne Dienst gibt es
// keinen Weg, und die Oberfläche bleibt beim Bisherigen, dem aktiven
// gh-Konto.

// GitHubRouteState ist der Zustand eines Weges über einen Dienst.
type GitHubRouteState string

const (
	// GitHubRouteOK: Der Dienst hat für beide Kanäle einen Zugang; gh und
	// git fetch laufen über ihn.
	GitHubRouteOK GitHubRouteState = "ok"
	// GitHubRouteNoAPI: Der Dienst hat einen git-Zugang, aber keinen für api
	// (nur ssh oder public, oder kein token- oder gh-Zugang verfügbar). Es
	// startet kein gh — ChildEnv nähme GH_TOKEN und GITHUB_TOKEN heraus, und
	// gh liefe still über das aktive Konto. git fetch läuft.
	GitHubRouteNoAPI GitHubRouteState = "no-api"
	// GitHubRouteNoAccess: Ein Dienst passt, aber Choose liefert für keinen
	// Kanal einen Zugang. Weder gh noch git fetch, wie run, das dann gar
	// nicht startet.
	GitHubRouteNoAccess GitHubRouteState = "no-access"
	// GitHubRouteFileError: Die Zugangsdatei hat ein Problem der Stufe error,
	// oder ihr Ort ist unbekannt. Ob das Repo einen Dienst hat, ist damit
	// unbekannt; die Oberfläche fällt nicht auf das aktive Konto zurück und
	// startet weder gh noch git fetch.
	GitHubRouteFileError GitHubRouteState = "file-error"
)

// GitHubRoute ist der Weg zu GitHub über einen Dienst. Er trägt keinen
// Token-Wert: den holt erst Environment, und dann steht er nur in der
// Kindumgebung.
type GitHubRoute struct {
	State GitHubRouteState
	// Service ist der Dienst, über den gelesen wird; leer bei
	// GitHubRouteFileError.
	Service string
	// Repo ist <owner>/<repo> aus dem Remote; bei GitHubRouteFileError leer,
	// wenn das Remote ohne die Datei nicht zuzuordnen war.
	Repo string
	// Matching sind alle Dienste, deren repo zum Remote passt, in
	// Dateireihenfolge. Mehr als einer: Die Oberfläche kann nicht fragen,
	// nimmt den ersten und sagt das (Note).
	Matching []string
	// Choice ist die Wahl von run ohne Optionen; die Zeiger zeigen in
	// service.Access.
	Choice githubaccess.Choice
	// Line ist die Zeile von run (githubaccess.RunLine), mit der die Seite
	// Dienst und Zugang nennt; leer bei GitHubRouteFileError.
	Line string
	// Message erklärt jeden Zustand außer GitHubRouteOK in einem Satz mit
	// Abhilfe.
	Message string
	// Note ist der Satz zu mehreren passenden Diensten, sonst leer.
	Note string
	// Problems sind bei GitHubRouteFileError die Probleme des Lesers.
	Problems []githubaccess.Problem

	service githubaccess.Service
}

// GitReady sagt, ob git fetch über den Dienst laufen darf: bei OK und NoAPI.
func (r *GitHubRoute) GitReady() bool {
	return r.State == GitHubRouteOK || r.State == GitHubRouteNoAPI
}

// APIReady sagt, ob gh über den Dienst laufen darf: nur bei OK.
func (r *GitHubRoute) APIReady() bool {
	return r.State == GitHubRouteOK
}

// githubRouteReadToken holt das Token des api-Kanals; austauschbar, damit
// Tests ohne gh auskommen.
var githubRouteReadToken = githubaccess.ReadToken

// Environment baut die Kindumgebung des Dienstes aus der Umgebung des
// Servers, wie run es für seinen Befehl tut: das Token des api-Kanals
// (ReadToken, für gh mit `gh auth token`) und ChildEnv. Erst hier wird das
// Token gelesen; es steht danach nur in GH_TOKEN und GITHUB_TOKEN der
// Rückgabe. Die Fehlermeldung trägt es nie und sagt, was zu tun ist. Ohne
// git-Zugang (GitReady falsch) gibt es keine Umgebung.
func (r *GitHubRoute) Environment(ctx context.Context) ([]string, error) {
	if !r.GitReady() {
		return nil, errors.New(r.Message)
	}
	token := ""
	if r.Choice.API != nil {
		value, err := githubRouteReadToken(ctx, *r.Choice.API)
		if err != nil {
			return nil, errors.New("Das Token für den Dienst " + r.Service + " ist nicht zu bekommen: " + err.Error() + " — k-playbook github test " + r.Service + " prüft den Zugang.")
		}
		token = value
	}
	env, err := githubaccess.ChildEnv(os.Environ(), r.service, r.Choice, token)
	if err != nil {
		return nil, errors.New("Die Umgebung für den Dienst " + r.Service + " lässt sich nicht bauen: " + err.Error() + ".")
	}
	return env, nil
}

// ResolveGitHubRoute bestimmt zum Repo in dir den Weg über einen Dienst. nil
// heißt ohne Dienst: Die Zugangsdatei fehlt, oder sie ist fehlerfrei und kein
// Dienst nennt das Repo (auch ohne GitHub-Remote). Dann gilt das Bisherige.
//
// Die Kosten (Task 126, Punkt 6): Load der Datei, drei bis vier lokale
// git-Aufrufe für das Remote (Frist githubContextTimeout, höchstens ein
// `ssh -G` für einen Alias, den die Datei nicht nennt) und Judge für den
// einen Dienst, der genommen wird — wie run, nicht für alle Dienste der
// Datei; `ssh -G` darin hat je Alias seine Frist von drei Sekunden. Kein
// Netz, kein Token: das liest erst Environment. Der Aufrufer ruft das einmal
// je Anfrage, innerhalb ihres Budgets.
func ResolveGitHubRoute(ctx context.Context, dir string) *GitHubRoute {
	path, err := githubaccess.Path()
	if err != nil {
		return &GitHubRoute{
			State:   GitHubRouteFileError,
			Message: "Der Ort der GitHub-Zugangsdatei ist unbekannt: " + err.Error() + ". Ob dieses Repo einen Dienst hat, ist damit offen; die Oberfläche liest GitHub deshalb nicht. HOME oder XDG_CONFIG_HOME setzen oder den Pfad in " + githubaccess.EnvConfig + " nennen.",
			Problems: []githubaccess.Problem{{
				Level: githubaccess.LevelError, Path: githubaccess.PathFile,
				Message: "Der Ort der GitHub-Zugangsdatei ist unbekannt: " + err.Error() + ".",
				Hint:    "HOME oder XDG_CONFIG_HOME setzen oder den Pfad in " + githubaccess.EnvConfig + " nennen.",
			}},
		}
	}
	result := githubaccess.Load(path)
	if !result.Present {
		return nil
	}
	if result.HasErrors() {
		return &GitHubRoute{
			State:    GitHubRouteFileError,
			Message:  fileErrorMessage(path, result.Problems),
			Problems: result.Problems,
		}
	}

	gitCtx, cancel := context.WithTimeout(ctx, githubContextTimeout)
	defer cancel()
	repo := githubRemoteRepo(projectRemoteURL(gitCtx, dir), result.Services)
	if repo == "" {
		return nil
	}
	var matching githubaccess.Services
	for _, service := range result.Services {
		if strings.EqualFold(service.Repo, repo) {
			matching = append(matching, service)
		}
	}
	if len(matching) == 0 {
		return nil
	}

	// Nur der genommene Dienst wird beurteilt, wie bei run: ssh -G kann je
	// Alias bis zu drei Sekunden brauchen.
	service := githubaccess.Judge(githubaccess.Services{matching[0]})[0]
	choice := githubaccess.Choose(service, githubaccess.ChooseOptions{})
	home, _ := os.UserHomeDir()
	route := &GitHubRoute{
		Service:  service.Name,
		Repo:     repo,
		Matching: matching.Names(),
		Choice:   choice,
		Line:     githubaccess.RunLine(service.Name, choice, home),
		service:  service,
	}
	if len(matching) > 1 {
		route.Note = "Mehrere Dienste der Zugangsdatei nennen " + repo + ": " + strings.Join(route.Matching, ", ") + ". Die Oberfläche kann nicht fragen und nimmt den ersten in Dateireihenfolge, " + service.Name + "."
	}
	switch {
	case choice.Empty():
		route.State = GitHubRouteNoAccess
		route.Message = "Der Dienst " + service.Name + " hat keinen verfügbaren Zugang (" + githubaccess.NoAccessMessage(service, githubaccess.ChooseOptions{}) + "). Die Oberfläche liest GitHub deshalb nicht, auch nicht über das aktive gh-Konto. k-playbook github check " + service.Name + " nennt die Gründe je Zugang."
	case choice.API == nil:
		route.State = GitHubRouteNoAPI
		route.Message = "Der Dienst " + service.Name + " hat keinen api-Zugang (kein verfügbarer Zugang vom Typ token oder gh). Die Oberfläche startet deshalb kein gh — es liefe sonst still über das aktive gh-Konto. git fetch läuft über den git-Zugang des Dienstes."
	default:
		route.State = GitHubRouteOK
	}
	return route
}

// fileErrorMessage ist der Satz zu einer fehlerhaften Zugangsdatei: Pfad,
// die Probleme der Stufe error und was zu tun ist.
func fileErrorMessage(path string, problems []githubaccess.Problem) string {
	var parts []string
	for _, problem := range problems {
		if problem.Level == githubaccess.LevelError {
			parts = append(parts, problem.Path+": "+problem.Message)
		}
	}
	return "Die GitHub-Zugangsdatei " + path + " ist fehlerhaft (" + strings.Join(parts, " ") + "). Ob dieses Repo einen Dienst hat, ist damit offen; die Oberfläche liest GitHub deshalb nicht, auch nicht über das aktive gh-Konto. k-playbook github check nennt alle Probleme mit Abhilfe."
}
