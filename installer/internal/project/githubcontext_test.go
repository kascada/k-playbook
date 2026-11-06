package project

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/githubaccess"
	"github.com/kascada/k-playbook/installer/internal/kephalaion"
)

// stubSSHHostIsGitHub ersetzt den Blick über ssh -G: known sind die Aliase,
// die nach github.com führen. Zurück kommt der Zähler der Aufrufe.
func stubSSHHostIsGitHub(t *testing.T, known ...string) *int {
	t.Helper()
	calls := 0
	previous := githubSSHHostIsGitHub
	githubSSHHostIsGitHub = func(host string) bool {
		calls++
		for _, alias := range known {
			if alias == host {
				return true
			}
		}
		return false
	}
	t.Cleanup(func() { githubSSHHostIsGitHub = previous })
	return &calls
}

func TestGitHubRemoteRepo(t *testing.T) {
	calls := stubSSHHostIsGitHub(t, "github-privat")
	services := githubaccess.Services{
		{Name: "k-playbook", Repo: "kascada/k-playbook", Access: []githubaccess.Access{{Type: "ssh", Host: "github-kamranbycloud"}}},
	}
	cases := []struct {
		remote, want string
		sshCalls     int
	}{
		{"https://github.com/kascada/k-playbook.git", "kascada/k-playbook", 0},
		{"git@github.com:kascada/k-playbook.git", "kascada/k-playbook", 0},
		{"ssh://git@github.com/kascada/k-playbook", "kascada/k-playbook", 0},
		// Alias, den die Datei nennt: ohne ssh -G.
		{"git@github-kamranbycloud:kascada/k-playbook.git", "kascada/k-playbook", 0},
		{"git@GitHub-KamranByCloud:kascada/other.git", "kascada/other", 0},
		// Alias, den die Datei nicht nennt: ssh -G entscheidet.
		{"git@github-privat:kascada/foo.git", "kascada/foo", 1},
		{"git@gitlab.com:kascada/k-playbook.git", "", 1},
		// Keine Form von run: kein ssh -G.
		{"ssh://git@github-zweit:2222/kascada/k-playbook", "", 0},
		{"https://gitlab.com/kascada/k-playbook.git", "", 0},
		{"https://x-access-token:ghp_geheim@github.com/kascada/k-playbook.git", "kascada/k-playbook", 0},
		{"", "", 0},
	}
	for _, c := range cases {
		*calls = 0
		if got := githubRemoteRepo(c.remote, services); got != c.want {
			t.Errorf("githubRemoteRepo(%q) = %q, erwartet %q", c.remote, got, c.want)
		}
		if *calls != c.sshCalls {
			t.Errorf("githubRemoteRepo(%q): %d Aufrufe von ssh -G, erwartet %d", c.remote, *calls, c.sshCalls)
		}
	}
}

// Das Remote, das zählt: das Upstream-Remote des Branches, sonst origin —
// roh gelesen, ohne insteadOf.
func TestProjectRemoteURL(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git nicht verfügbar")
	}
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "repo")
	run(t, filepath.Dir(dir), "git", "init", "-b", "main", dir)

	if got := projectRemoteURL(ctx, dir); got != "" {
		t.Errorf("ohne Remote: %q", got)
	}
	if got := projectRemoteURL(ctx, t.TempDir()); got != "" {
		t.Errorf("ohne Repo: %q", got)
	}

	run(t, dir, "git", "remote", "add", "origin", "git@github.com:kascada/origin.git")
	// insteadOf des Rechners verfälscht den rohen Eintrag nicht.
	run(t, dir, "git", "config", "url.https://example.invalid/.insteadOf", "git@github.com:")
	if got := projectRemoteURL(ctx, dir); got != "git@github.com:kascada/origin.git" {
		t.Errorf("origin: %q", got)
	}

	run(t, dir, "git", "remote", "add", "upstream", "https://github.com/kascada/upstream.git")
	run(t, dir, "git", "config", "branch.main.remote", "upstream")
	if got := projectRemoteURL(ctx, dir); got != "https://github.com/kascada/upstream.git" {
		t.Errorf("Upstream des Branches: %q", got)
	}

	// Ein Upstream ohne Adresse (lokaler Branch) fällt auf origin zurück.
	run(t, dir, "git", "config", "branch.main.remote", ".")
	if got := projectRemoteURL(ctx, dir); got != "git@github.com:kascada/origin.git" {
		t.Errorf("Upstream „.“: %q", got)
	}
}

// githubFakes legt gh und ssh als Attrappen vor git in den PATH, wie in den
// Tests von githubaccess: ssh beantwortet `-G <host>`; github-kamranbycloud
// und github.com lösen nach github.com auf, jeder andere Host zu sich selbst.
// gh hat das Konto alice. Jeder Aufruf von ssh hinterlässt eine Zeile im
// Protokoll, dessen Pfad zurückkommt.
func githubFakes(t *testing.T) string {
	t.Helper()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git nicht verfügbar")
	}
	dir := t.TempDir()
	calls := filepath.Join(t.TempDir(), "ssh-aufrufe")
	ssh := `#!/bin/sh
echo "$*" >> ` + calls + `
if [ "$1" != "-G" ]; then echo "unerwarteter Aufruf: $*" >&2; exit 2; fi
case "$2" in
  github-kamranbycloud|github.com) echo "user git"; echo "hostname github.com" ;;
  *) echo "user git"; echo "hostname $2" ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "ssh"), []byte(ssh), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+filepath.Dir(git))
	ghDir := t.TempDir()
	hosts := "github.com:\n    user: alice\n    oauth_token: gho_geheim_gh\n    users:\n        alice:\n            oauth_token: gho_geheim_gh\n"
	if err := os.WriteFile(filepath.Join(ghDir, "hosts.yml"), []byte(hosts), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GH_CONFIG_DIR", ghDir)
	return calls
}

// githubRepoWith legt ein Repo mit origin an; ohne remote keins.
func githubRepoWith(t *testing.T, remote string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "repo")
	run(t, filepath.Dir(dir), "git", "init", "-b", "main", dir)
	if remote != "" {
		run(t, dir, "git", "remote", "add", "origin", remote)
	}
	return dir
}

// githubConfig schreibt die Zugangsdatei und setzt den Override darauf.
func githubConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), githubaccess.FileName)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(githubaccess.EnvConfig, path)
	return path
}

const githubTwoServices = `services:
  k-playbook:
    repo: kascada/k-playbook
    access:
      - type: ssh
        host: github-kamranbycloud
      - type: gh
        login: alice
  k-playbook-dev:
    repo: kascada/k-playbook-dev
    access:
      - type: gh
        login: alice
`

// Das Feld allein legt nichts an. Im Briefing legt die vorher laufende
// Umgebungsprüfung die Vorlage an; das belegt
// TestBriefingGitHubVorlageDurchUmgebungspruefung in cmd/k-playbook.
func TestContextGitHubDateiFehlt(t *testing.T) {
	githubFakes(t)
	path := filepath.Join(t.TempDir(), "fehlt", githubaccess.FileName)
	t.Setenv(githubaccess.EnvConfig, path)

	field := contextGitHub(githubRepoWith(t, "https://github.com/kascada/k-playbook.git"))
	if field.File != path || field.Present || len(field.Problems) != 0 || len(field.Services) != 0 || len(field.ProjectServices) != 0 {
		t.Errorf("Feld bei fehlender Datei: %+v", field)
	}
	if field.Repo != "kascada/k-playbook" {
		t.Errorf("repo auch ohne Datei: %q", field.Repo)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("Vorlage angelegt: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Errorf("Verzeichnis angelegt: %v", err)
	}
	data, err := json.Marshal(field)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"present":false`, `"problems":[]`, `"projectServices":[]`, `"services":{}`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("JSON ohne %s: %s", want, data)
		}
	}
}

func TestContextGitHubDateiMitError(t *testing.T) {
	githubFakes(t)
	githubConfig(t, "services:\n  kaputt:\n    access:\n      - type: gh\n        login: alice\n")

	field := contextGitHub(githubRepoWith(t, "git@github.com:kascada/k-playbook.git"))
	if !field.Present || len(field.Problems) == 0 || field.Problems[0].Level != githubaccess.LevelError {
		t.Fatalf("Probleme: %+v", field.Problems)
	}
	data, _ := json.Marshal(field)
	if !strings.Contains(string(data), `"services":{}`) || !strings.Contains(string(data), `"projectServices":[]`) {
		t.Errorf("bei einem error leer: %s", data)
	}
	if field.Repo != "kascada/k-playbook" {
		t.Errorf("repo: %q", field.Repo)
	}
}

func TestContextGitHubEinPassenderDienst(t *testing.T) {
	calls := githubFakes(t)
	githubConfig(t, githubTwoServices)

	field := contextGitHub(githubRepoWith(t, "git@github-kamranbycloud:Kascada/K-Playbook.git"))
	if field.Remote != "git@github-kamranbycloud:Kascada/K-Playbook.git" || field.Repo != "Kascada/K-Playbook" {
		t.Errorf("remote %q, repo %q", field.Remote, field.Repo)
	}
	if len(field.ProjectServices) != 1 || field.ProjectServices[0] != "k-playbook" {
		t.Errorf("projectServices: %v", field.ProjectServices)
	}
	// Die Dienste wie bei show --json: beurteilt, mit selected.
	if len(field.Services) != 2 {
		t.Fatalf("services: %+v", field.Services)
	}
	service := field.Services[0]
	if service.Access[0].State != githubaccess.StateAvailable || service.Selected == nil ||
		service.Selected.Read.Git == nil || *service.Selected.Read.Git != 0 || service.Selected.Read.API == nil || *service.Selected.Read.API != 1 {
		t.Errorf("k-playbook beurteilt: %+v, selected %+v", service.Access, service.Selected)
	}
	// ssh -G nur für den host der Datei (Judge), nicht noch einmal für das
	// Remote: Die Datei nennt den Alias.
	log, _ := os.ReadFile(calls)
	if string(log) != "-G github-kamranbycloud\n" {
		t.Errorf("ssh-Aufrufe:\n%s", log)
	}
}

func TestContextGitHubZweiPassendeDienste(t *testing.T) {
	githubFakes(t)
	githubConfig(t, githubTwoServices+`  zweit:
    repo: KASCADA/k-playbook-dev
    access:
      - type: public
`)
	field := contextGitHub(githubRepoWith(t, "https://github.com/kascada/k-playbook-dev"))
	if strings.Join(field.ProjectServices, ",") != "k-playbook-dev,zweit" {
		t.Errorf("projectServices: %v", field.ProjectServices)
	}
}

func TestContextGitHubKeinPassenderDienst(t *testing.T) {
	githubFakes(t)
	githubConfig(t, githubTwoServices)
	field := contextGitHub(githubRepoWith(t, "https://github.com/kascada/foo.git"))
	if field.Repo != "kascada/foo" || len(field.ProjectServices) != 0 || len(field.Services) != 2 {
		t.Errorf("kein passender Dienst: %+v", field)
	}
}

func TestContextGitHubOhneRemote(t *testing.T) {
	githubFakes(t)
	githubConfig(t, githubTwoServices)
	for name, dir := range map[string]string{
		"ohne Remote":       githubRepoWith(t, ""),
		"ohne Repo":         t.TempDir(),
		"fremder Host":      githubRepoWith(t, "git@gitlab.com:kascada/k-playbook.git"),
		"unbekannter Alias": githubRepoWith(t, "git@github-fremd:kascada/k-playbook.git"),
	} {
		field := contextGitHub(dir)
		if field.Remote != "" || field.Repo != "" || len(field.ProjectServices) != 0 || len(field.Services) != 2 {
			t.Errorf("%s: %+v", name, field)
		}
	}
}

// Kein Token-Wert im Kontext: weder aus der Zugangsdatei (Token-Datei, gh)
// noch aus einem Remote mit eingebettetem Token. repo ist trotzdem gefüllt.
func TestContextGitHubOhneTokenWert(t *testing.T) {
	githubFakes(t)
	tokenFile := filepath.Join(t.TempDir(), "x.token")
	if err := os.WriteFile(tokenFile, []byte("ghp_dateigeheim\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	githubConfig(t, "services:\n  k-playbook:\n    repo: kascada/k-playbook\n    access:\n      - type: token\n        file: "+tokenFile+"\n      - type: gh\n        login: alice\n")
	field := contextGitHub(githubRepoWith(t, "https://x-access-token:ghp_remotegeheim@github.com/kascada/k-playbook.git"))
	data, err := json.Marshal(field)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"ghp_remotegeheim", "x-access-token", "ghp_dateigeheim", "gho_geheim"} {
		if strings.Contains(string(data), secret) {
			t.Errorf("JSON trägt %s: %s", secret, data)
		}
	}
	if field.Remote != "https://github.com/kascada/k-playbook.git" || field.Repo != "kascada/k-playbook" ||
		len(field.ProjectServices) != 1 {
		t.Errorf("remote %q, repo %q, projectServices %v", field.Remote, field.Repo, field.ProjectServices)
	}
}

// ContextForDir füllt das Feld, BuildContext nicht; das Remote kommt ab dem
// Startverzeichnis, auch aus einem eigenen Repo unterhalb des Projekts.
func TestContextForDirFuelltGitHub(t *testing.T) {
	githubFakes(t)
	githubConfig(t, githubTwoServices)
	root := newBriefingProject(t)
	stubKephalaionCheck(t, kephalaion.Status{State: kephalaion.StateNotConfigured})
	run(t, root, "git", "init", "-b", "main")
	run(t, root, "git", "remote", "add", "origin", "git@github-kamranbycloud:kascada/k-playbook.git")
	nested := filepath.Join(root, "k-playbook-local")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	run(t, nested, "git", "init", "-b", "main")
	run(t, nested, "git", "remote", "add", "origin", "https://github.com/kascada/k-playbook-dev.git")

	built, err := BuildContext(root)
	if err != nil {
		t.Fatal(err)
	}
	if built.GitHub != nil {
		t.Errorf("BuildContext füllt github: %+v", built.GitHub)
	}
	for dir, want := range map[string]string{root: "k-playbook", nested: "k-playbook-dev"} {
		full, err := ContextForDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		if full.GitHub == nil || strings.Join(full.GitHub.ProjectServices, ",") != want {
			t.Errorf("ContextForDir(%s): %+v", dir, full.GitHub)
		}
	}
}
