package project

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/githubaccess"
)

// routeToken ist ein Fake-Token: Er darf nur in GH_TOKEN und GITHUB_TOKEN der
// Kindumgebung stehen, nie in Zeile, Meldung oder Hinweis des Weges.
const routeToken = "ghp_fake_route_token_126"

// routeTokenFile legt eine Token-Datei mit routeToken an (Modus 0600).
func routeTokenFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "k-playbook.token")
	if err := os.WriteFile(path, []byte(routeToken+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// stubRouteToken ersetzt ReadToken und zählt die Aufrufe.
func stubRouteToken(t *testing.T, token string, err error) *int {
	t.Helper()
	calls := 0
	previous := githubRouteReadToken
	githubRouteReadToken = func(context.Context, githubaccess.Access) (string, error) {
		calls++
		return token, err
	}
	t.Cleanup(func() { githubRouteReadToken = previous })
	return &calls
}

// assertNoToken prüft, dass der Weg den Token-Wert in keinem Text trägt.
func assertNoToken(t *testing.T, route *GitHubRoute) {
	t.Helper()
	for name, text := range map[string]string{"Line": route.Line, "Message": route.Message, "Note": route.Note} {
		if strings.Contains(text, routeToken) {
			t.Errorf("%s trägt den Token: %s", name, text)
		}
	}
}

func envValue(env []string, name string) (string, bool) {
	value, found := "", false
	for _, entry := range env {
		if key, rest, ok := strings.Cut(entry, "="); ok && key == name {
			value, found = rest, true
		}
	}
	return value, found
}

func TestGitHubRouteMitDienst(t *testing.T) {
	githubFakes(t)
	tokenFile := routeTokenFile(t)
	githubConfig(t, "services:\n  k-playbook:\n    repo: kascada/k-playbook\n    access:\n      - type: ssh\n        host: github-kamranbycloud\n      - type: token\n        file: "+tokenFile+"\n")
	t.Setenv("GH_TOKEN", "vom-server")

	route := ResolveGitHubRoute(context.Background(), githubRepoWith(t, "git@github-kamranbycloud:kascada/k-playbook.git"))
	if route == nil {
		t.Fatal("kein Weg, obwohl ein Dienst das Repo nennt")
	}
	if route.State != GitHubRouteOK || route.Service != "k-playbook" || route.Repo != "kascada/k-playbook" || route.Note != "" {
		t.Fatalf("Weg: %+v", route)
	}
	if !route.GitReady() || !route.APIReady() {
		t.Errorf("GitReady/APIReady: %v/%v", route.GitReady(), route.APIReady())
	}
	if want := "github run k-playbook: git über ssh host github-kamranbycloud, api über token " + tokenFile; route.Line != want {
		t.Errorf("Zeile %q, erwartet %q", route.Line, want)
	}
	assertNoToken(t, route)

	env, err := route.Environment(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"GH_TOKEN", "GITHUB_TOKEN"} {
		if value, _ := envValue(env, name); value != routeToken {
			t.Errorf("%s = %q, erwartet das Token des Dienstes", name, value)
		}
	}
	if value, _ := envValue(env, "GIT_TERMINAL_PROMPT"); value != "0" {
		t.Errorf("GIT_TERMINAL_PROMPT = %q", value)
	}
	if count, ok := envValue(env, "GIT_CONFIG_COUNT"); !ok || count == "0" {
		t.Errorf("GIT_CONFIG_COUNT = %q: keine Umlenkung des git-Kanals", count)
	}
}

func TestGitHubRouteOhneDienst(t *testing.T) {
	githubFakes(t)
	repo := githubRepoWith(t, "https://github.com/kascada/k-playbook.git")

	t.Run("Datei fehlt", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "fehlt", githubaccess.FileName)
		t.Setenv(githubaccess.EnvConfig, path)
		if route := ResolveGitHubRoute(context.Background(), repo); route != nil {
			t.Errorf("Weg ohne Datei: %+v", route)
		}
		if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
			t.Errorf("Verzeichnis angelegt: %v", err)
		}
	})
	t.Run("Repo nicht genannt", func(t *testing.T) {
		githubConfig(t, "services:\n  anderes:\n    repo: kascada/anderes\n    access:\n      - type: public\n")
		if route := ResolveGitHubRoute(context.Background(), repo); route != nil {
			t.Errorf("Weg für ein Repo, das kein Dienst nennt: %+v", route)
		}
	})
	t.Run("kein GitHub-Remote", func(t *testing.T) {
		githubConfig(t, githubTwoServices)
		if route := ResolveGitHubRoute(context.Background(), githubRepoWith(t, "")); route != nil {
			t.Errorf("Weg ohne Remote: %+v", route)
		}
	})
}

func TestGitHubRouteMehrereDienste(t *testing.T) {
	githubFakes(t)
	tokenFile := routeTokenFile(t)
	githubConfig(t, "services:\n  erster:\n    repo: kascada/k-playbook\n    access:\n      - type: token\n        file: "+tokenFile+"\n  zweiter:\n    repo: Kascada/K-Playbook\n    access:\n      - type: public\n")

	route := ResolveGitHubRoute(context.Background(), githubRepoWith(t, "https://github.com/kascada/k-playbook.git"))
	if route == nil || route.Service != "erster" || len(route.Matching) != 2 || route.Matching[1] != "zweiter" {
		t.Fatalf("Weg: %+v", route)
	}
	if !strings.Contains(route.Note, "erster, zweiter") || !strings.Contains(route.Note, "nimmt den ersten") {
		t.Errorf("Hinweis zu mehreren Diensten: %q", route.Note)
	}
}

func TestGitHubRouteDateiFehlerhaft(t *testing.T) {
	githubFakes(t)
	path := githubConfig(t, "services:\n  kaputt:\n    access:\n      - type: gh\n        login: alice\n")
	calls := stubRouteToken(t, routeToken, nil)

	route := ResolveGitHubRoute(context.Background(), githubRepoWith(t, "git@github.com:kascada/k-playbook.git"))
	if route == nil || route.State != GitHubRouteFileError {
		t.Fatalf("Weg bei fehlerhafter Datei: %+v", route)
	}
	if len(route.Problems) == 0 || route.Problems[0].Level != githubaccess.LevelError {
		t.Errorf("Probleme: %+v", route.Problems)
	}
	if !strings.Contains(route.Message, path) || !strings.Contains(route.Message, "k-playbook github check") {
		t.Errorf("Meldung: %q", route.Message)
	}
	if route.GitReady() || route.APIReady() {
		t.Error("fehlerhafte Datei: weder git noch api")
	}
	if _, err := route.Environment(context.Background()); err == nil {
		t.Error("Umgebung trotz fehlerhafter Datei")
	}
	if *calls != 0 {
		t.Errorf("Token gelesen: %d", *calls)
	}
}

func TestGitHubRouteOrtUnbekannt(t *testing.T) {
	githubFakes(t)
	t.Setenv(githubaccess.EnvConfig, "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "")

	route := ResolveGitHubRoute(context.Background(), githubRepoWith(t, "git@github.com:kascada/k-playbook.git"))
	if route == nil || route.State != GitHubRouteFileError || len(route.Problems) != 1 {
		t.Fatalf("Weg bei unbekanntem Ort: %+v", route)
	}
	if !strings.Contains(route.Message, "Ort der GitHub-Zugangsdatei ist unbekannt") {
		t.Errorf("Meldung: %q", route.Message)
	}
}

func TestGitHubRouteKeinVerfuegbarerZugang(t *testing.T) {
	githubFakes(t)
	missing := filepath.Join(t.TempDir(), "fehlt.token")
	githubConfig(t, "services:\n  k-playbook:\n    repo: kascada/k-playbook\n    access:\n      - type: token\n        file: "+missing+"\n        disabled: true\n      - type: token\n        file: "+missing+"\n")
	calls := stubRouteToken(t, routeToken, nil)

	route := ResolveGitHubRoute(context.Background(), githubRepoWith(t, "https://github.com/kascada/k-playbook.git"))
	if route == nil || route.State != GitHubRouteNoAccess || route.Service != "k-playbook" {
		t.Fatalf("Weg: %+v", route)
	}
	if !strings.Contains(route.Message, "Die Token-Datei fehlt") || !strings.Contains(route.Message, "deaktiviert") {
		t.Errorf("Meldung nennt den Grund nicht: %q", route.Message)
	}
	if want := "github run k-playbook: git: keiner, api: keiner"; route.Line != want {
		t.Errorf("Zeile %q, erwartet %q", route.Line, want)
	}
	if route.GitReady() || route.APIReady() {
		t.Error("ohne Zugang: weder git noch api")
	}
	if _, err := route.Environment(context.Background()); err == nil {
		t.Error("Umgebung ohne Zugang")
	}
	if *calls != 0 {
		t.Errorf("Token gelesen: %d", *calls)
	}
}

func TestGitHubRouteKeinAPIKanal(t *testing.T) {
	githubFakes(t)
	githubConfig(t, "services:\n  k-playbook:\n    repo: kascada/k-playbook\n    access:\n      - type: ssh\n        host: github-kamranbycloud\n      - type: public\n")
	calls := stubRouteToken(t, routeToken, nil)
	t.Setenv("GH_TOKEN", "vom-server")

	route := ResolveGitHubRoute(context.Background(), githubRepoWith(t, "git@github-kamranbycloud:kascada/k-playbook.git"))
	if route == nil || route.State != GitHubRouteNoAPI {
		t.Fatalf("Weg: %+v", route)
	}
	if !route.GitReady() || route.APIReady() {
		t.Errorf("ohne api: git ja, api nein — %v/%v", route.GitReady(), route.APIReady())
	}
	if !strings.Contains(route.Message, "Der Dienst k-playbook hat keinen api-Zugang") {
		t.Errorf("Meldung nennt den Dienst nicht: %q", route.Message)
	}
	env, err := route.Environment(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if *calls != 0 {
		t.Errorf("Token gelesen, obwohl kein api-Kanal: %d", *calls)
	}
	for _, name := range []string{"GH_TOKEN", "GITHUB_TOKEN"} {
		if _, found := envValue(env, name); found {
			t.Errorf("%s steht in der Umgebung ohne api-Kanal", name)
		}
	}
}

// Scheitert das Token, gibt es keine Umgebung; die Meldung nennt den Dienst
// und den Test, aber keinen Token-Wert.
func TestGitHubRouteTokenNichtZuBekommen(t *testing.T) {
	githubFakes(t)
	githubConfig(t, "services:\n  k-playbook:\n    repo: kascada/k-playbook\n    access:\n      - type: gh\n        login: alice\n")
	stubRouteToken(t, "", errors.New("gh auth token für alice ist gescheitert"))

	route := ResolveGitHubRoute(context.Background(), githubRepoWith(t, "https://github.com/kascada/k-playbook.git"))
	if route == nil || route.State != GitHubRouteOK {
		t.Fatalf("Weg: %+v", route)
	}
	_, err := route.Environment(context.Background())
	if err == nil || !strings.Contains(err.Error(), "k-playbook github test k-playbook") {
		t.Errorf("Fehler: %v", err)
	}
}
