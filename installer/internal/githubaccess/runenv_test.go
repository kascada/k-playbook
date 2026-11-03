package githubaccess

import (
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// canary ist ein erkennbares Token: Wo es auftaucht, außer in GH_TOKEN und
// GITHUB_TOKEN, ist es durchgesickert.
const canary = "ghp_KANARIENVOGEL_0123456789abcdef"

// runService ist der Dienst der Tests: ein Repository, ein Alias, dazu
// ein deaktivierter zweiter Alias, der als Form trotzdem zählt.
func runService(accesses ...Access) Service {
	return Service{Name: "web", Repo: "acme/web", Access: accesses}
}

func envValue(env []string, name string) (string, bool) {
	return lookupEnv(env, name)
}

func TestGitConfigJeZugangsart(t *testing.T) {
	alias := Access{Type: "ssh", Host: "github-acme", State: StateAvailable}
	oldAlias := Access{Type: "ssh", Host: "github-alt", Disabled: true, State: StateDisabled}
	gh := Access{Type: "gh", Login: "alice", State: StateAvailable}
	key := Access{Type: "ssh", Key: "/k/id", State: StateAvailable}
	pub := Access{Type: "public", State: StateAvailable}

	text := func(entries []ConfigEntry) string {
		var lines []string
		for _, entry := range entries {
			value := entry.Value
			if value == CredentialHelper {
				value = "<helper>"
			}
			lines = append(lines, entry.Key+"="+value)
		}
		return strings.Join(lines, "\n")
	}

	// rewrites sind die Umlenkungen auf target: zuerst target auf sich
	// selbst, dann jede Quellform, je mit und ohne .git, je als insteadOf
	// und als pushInsteadOf.
	rewrites := func(target string, sources ...string) []string {
		var lines []string
		for _, source := range append([]string{target}, sources...) {
			for _, kind := range []string{"insteadOf", "pushInsteadOf"} {
				lines = append(lines,
					"url."+target+".git."+kind+"="+source+".git",
					"url."+target+"."+kind+"="+source)
			}
		}
		return lines
	}
	https := "https://github.com/acme/web"
	sshForms := []string{"git@github.com:acme/web", "ssh://git@github.com/acme/web", "git@github-acme:acme/web"}

	cases := []struct {
		name     string
		service  Service
		git      int
		expected []string
	}{
		{"gh: SSH-Formen auf https, Helper zurückgesetzt und eigener", runService(gh, alias, oldAlias), 0, append(
			rewrites(https, append(sshForms, "git@github-alt:acme/web")...),
			"credential.https://github.com/acme/web.git.helper=",
			"credential.https://github.com/acme/web.git.helper=<helper>",
			"credential.https://github.com/acme/web.helper=",
			"credential.https://github.com/acme/web.helper=<helper>",
		)},
		{"public: wie gh, aber nur zurückgesetzt", runService(pub), 0, append(
			rewrites(https, sshForms[:2]...),
			"credential.https://github.com/acme/web.git.helper=",
			"credential.https://github.com/acme/web.helper=",
		)},
		// Ein Fall ausgeschrieben, damit die Form der Einträge feststeht.
		{"ssh key: alle Formen auf git@github.com:", runService(key, alias), 0, []string{
			"url.git@github.com:acme/web.git.insteadOf=git@github.com:acme/web.git",
			"url.git@github.com:acme/web.insteadOf=git@github.com:acme/web",
			"url.git@github.com:acme/web.git.pushInsteadOf=git@github.com:acme/web.git",
			"url.git@github.com:acme/web.pushInsteadOf=git@github.com:acme/web",
			"url.git@github.com:acme/web.git.insteadOf=https://github.com/acme/web.git",
			"url.git@github.com:acme/web.insteadOf=https://github.com/acme/web",
			"url.git@github.com:acme/web.git.pushInsteadOf=https://github.com/acme/web.git",
			"url.git@github.com:acme/web.pushInsteadOf=https://github.com/acme/web",
			"url.git@github.com:acme/web.git.insteadOf=ssh://git@github.com/acme/web.git",
			"url.git@github.com:acme/web.insteadOf=ssh://git@github.com/acme/web",
			"url.git@github.com:acme/web.git.pushInsteadOf=ssh://git@github.com/acme/web.git",
			"url.git@github.com:acme/web.pushInsteadOf=ssh://git@github.com/acme/web",
			"url.git@github.com:acme/web.git.insteadOf=git@github-acme:acme/web.git",
			"url.git@github.com:acme/web.insteadOf=git@github-acme:acme/web",
			"url.git@github.com:acme/web.git.pushInsteadOf=git@github-acme:acme/web.git",
			"url.git@github.com:acme/web.pushInsteadOf=git@github-acme:acme/web",
		}},
		{"ssh host: alle Formen auf den Alias, auch der deaktivierte", runService(alias, oldAlias), 0,
			rewrites("git@github-acme:acme/web", https, "git@github.com:acme/web", "ssh://git@github.com/acme/web", "git@github-alt:acme/web")},
		{"ssh host github.com: die Zielform nur einmal", runService(Access{Type: "ssh", Host: "github.com", State: StateAvailable}), 0,
			rewrites("git@github.com:acme/web", https, "ssh://git@github.com/acme/web")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := text(GitConfig(c.service, &c.service.Access[c.git]))
			if expected := strings.Join(c.expected, "\n"); got != expected {
				t.Errorf("Einträge:\n%s\nerwartet:\n%s", got, expected)
			}
		})
	}
	if entries := GitConfig(runService(gh), nil); entries != nil {
		t.Errorf("ohne git-Zugang: %v", entries)
	}
}

func TestChildEnvTokenNurInDenTokenVariablen(t *testing.T) {
	for _, typ := range []string{"gh", "token"} {
		t.Run(typ, func(t *testing.T) {
			access := Access{Type: typ, Login: "alice", State: StateAvailable}
			if typ == "token" {
				access = Access{Type: typ, File: "/t/a.token", State: StateAvailable}
			}
			service := runService(access, Access{Type: "ssh", Host: "github-acme", State: StateAvailable})
			choice := Choose(service, ChooseOptions{})
			environ := []string{"PATH=/bin", "GH_TOKEN=geerbt", "GITHUB_TOKEN=geerbt", "GIT_TERMINAL_PROMPT=1", "GIT_SSH_COMMAND=ssh -v"}
			env, err := ChildEnv(environ, service, choice, canary)
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range env {
				name, value, _ := strings.Cut(entry, "=")
				if name == "GH_TOKEN" || name == "GITHUB_TOKEN" {
					if value != canary {
						t.Errorf("%s=%s", name, value)
					}
					continue
				}
				if strings.Contains(entry, canary) || strings.Contains(entry, canary[:12]) {
					t.Errorf("Token außerhalb der Token-Variablen: %s", name)
				}
			}
			for name, expected := range map[string]string{"GIT_TERMINAL_PROMPT": "0", "GIT_SSH_COMMAND": "ssh -v", "PATH": "/bin"} {
				if value, _ := envValue(env, name); value != expected {
					t.Errorf("%s=%q, erwartet %q", name, value, expected)
				}
			}
			counts := map[string]int{}
			for _, entry := range env {
				name, _, _ := strings.Cut(entry, "=")
				counts[name]++
			}
			for name, n := range counts {
				if n > 1 {
					t.Errorf("%s steht %d-mal in der Umgebung", name, n)
				}
			}
			if line := RunLine(service.Name, choice, "/home/a"); strings.Contains(line, canary) {
				t.Errorf("Zeile trägt das Token: %s", line)
			}
		})
	}
}

func TestChildEnvOhneApiEntferntGeerbteTokens(t *testing.T) {
	service := runService(Access{Type: "ssh", Key: "/home/a/.ssh/id acme", State: StateAvailable})
	env, err := ChildEnv([]string{"GH_TOKEN=geerbt", "GITHUB_TOKEN=geerbt", "GIT_SSH_COMMAND=ssh -v"}, service, Choose(service, ChooseOptions{}), "")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"GH_TOKEN", "GITHUB_TOKEN"} {
		if _, ok := envValue(env, name); ok {
			t.Errorf("%s ist geblieben", name)
		}
	}
	if value, _ := envValue(env, "GIT_SSH_COMMAND"); value != "ssh -i '/home/a/.ssh/id acme' -o IdentitiesOnly=yes" {
		t.Errorf("GIT_SSH_COMMAND=%s", value)
	}
}

func TestChildEnvHaengtAnVorhandeneGitConfigAn(t *testing.T) {
	service := runService(Access{Type: "public", State: StateAvailable})
	environ := []string{"GIT_CONFIG_COUNT=2", "GIT_CONFIG_KEY_0=core.eins", "GIT_CONFIG_VALUE_0=1", "GIT_CONFIG_KEY_1=core.zwei", "GIT_CONFIG_VALUE_1=2", "GIT_CONFIG_KEY_2=alt.rest", "GIT_CONFIG_VALUE_2=x"}
	env, err := ChildEnv(environ, service, Choose(service, ChooseOptions{}), "")
	if err != nil {
		t.Fatal(err)
	}
	// public: drei Formen (die Zielform und zwei SSH-Formen) mal zwei
	// Endungen mal insteadOf/pushInsteadOf, dazu zwei Helper.
	if value, _ := envValue(env, "GIT_CONFIG_COUNT"); value != "16" {
		t.Errorf("GIT_CONFIG_COUNT=%s, erwartet 16 (2 + 14)", value)
	}
	for name, expected := range map[string]string{"GIT_CONFIG_KEY_0": "core.eins", "GIT_CONFIG_VALUE_1": "2", "GIT_CONFIG_KEY_2": "url.https://github.com/acme/web.git.insteadOf", "GIT_CONFIG_VALUE_15": ""} {
		if value, ok := envValue(env, name); !ok || value != expected {
			t.Errorf("%s=%q, erwartet %q", name, value, expected)
		}
	}

	for _, bad := range []string{"zwei", "-1"} {
		if _, err := ChildEnv([]string{"GIT_CONFIG_COUNT=" + bad}, service, Choose(service, ChooseOptions{}), ""); err == nil {
			t.Errorf("GIT_CONFIG_COUNT=%s angenommen", bad)
		}
	}
	gh := runService(Access{Type: "gh", Login: "alice", State: StateAvailable})
	if _, err := ChildEnv(nil, gh, Choose(gh, ChooseOptions{}), ""); err == nil {
		t.Error("api über gh ohne Token angenommen")
	}
}

func TestRunLine(t *testing.T) {
	home := "/home/a"
	token := Access{Type: "token", File: "/home/a/.config/github-tokens/acme.token", State: StateAvailable}
	gh := Access{Type: "gh", Login: "kkl_kmde", State: StateAvailable}
	host := Access{Type: "ssh", Host: "github.com-km", State: StateAvailable}
	key := Access{Type: "ssh", Key: "/home/a/.ssh/id_acme", State: StateAvailable}
	pub := Access{Type: "public", State: StateAvailable}
	cases := []struct {
		service  Service
		options  ChooseOptions
		expected string
	}{
		{runService(token, host), ChooseOptions{}, "github run web: git über token ~/.config/github-tokens/acme.token, api über token"},
		{runService(host, gh), ChooseOptions{}, "github run web: git über ssh host github.com-km, api über gh kkl_kmde"},
		{runService(gh), ChooseOptions{}, "github run web: git über gh kkl_kmde, api über gh"},
		{runService(key), ChooseOptions{}, "github run web: git über ssh key ~/.ssh/id_acme, api: keiner"},
		{runService(pub, token), ChooseOptions{}, "github run web: git über public, api über token ~/.config/github-tokens/acme.token"},
		{runService(gh), ChooseOptions{Skip: []string{"gh"}}, "github run web: git: keiner, api: keiner"},
		{runService(Access{Type: "token", File: "/srv/t.token", State: StateAvailable}, host), ChooseOptions{Use: "ssh"}, "github run web: git über ssh host github.com-km, api: keiner"},
	}
	for _, c := range cases {
		if got := RunLine(c.service.Name, Choose(c.service, c.options), home); got != c.expected {
			t.Errorf("Zeile\n%s\nerwartet\n%s", got, c.expected)
		}
	}
	if got := tildePath("/srv/t.token", home); got != "/srv/t.token" {
		t.Errorf("tildePath außerhalb von home: %s", got)
	}
	if got := tildePath("/home/ab/t", home); got != "/home/ab/t" {
		t.Errorf("tildePath mit gleichem Anfang: %s", got)
	}
}

func TestReadToken(t *testing.T) {
	dir := t.TempDir()
	// gh gibt das Token des Logins aus und protokolliert seine Argumente
	// und ob es GH_TOKEN geerbt hat; bei bob gibt es das Token aus und
	// scheitert trotzdem, mit einer Meldung, die das Token wiederholt.
	log := filepath.Join(dir, "gh.log")
	gh := "#!/bin/sh\necho \"$* GH_TOKEN=${GH_TOKEN:-}\" >> " + log + "\n" +
		"case \"$6\" in alice) echo " + canary + " ;; *) echo " + canary + "; echo \"kaputt: " + canary + "\" >&2; exit 1 ;; esac\n"
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(gh), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("GH_TOKEN", "geerbt")

	token, err := ReadToken(context.Background(), Access{Type: "gh", Login: "alice"})
	if err != nil || token != canary {
		t.Fatalf("gh alice: %q %v", token, err)
	}
	_, err = ReadToken(context.Background(), Access{Type: "gh", Login: "bob"})
	if err == nil || strings.Contains(err.Error(), canary[:12]) || !strings.Contains(err.Error(), "gh auth token für bob") {
		t.Errorf("gh bob: %v", err)
	}
	data, _ := os.ReadFile(log)
	if got := string(data); got != "auth token --hostname github.com --user alice GH_TOKEN=\nauth token --hostname github.com --user bob GH_TOKEN=\n" {
		t.Errorf("gh-Aufrufe:\n%s", got)
	}

	file := filepath.Join(dir, "a.token")
	if err := os.WriteFile(file, []byte(canary+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if token, err := ReadToken(context.Background(), Access{Type: "token", File: file}); err != nil || token != canary {
		t.Errorf("token: %q %v", token, err)
	}
	if _, err := ReadToken(context.Background(), Access{Type: "token", File: filepath.Join(dir, "fehlt")}); err == nil {
		t.Error("fehlende Datei angenommen")
	}
	if _, err := ReadToken(context.Background(), Access{Type: "ssh", Host: "x"}); err == nil {
		t.Error("ssh trägt ein Token")
	}
}

// realGit bereitet einen Test mit echtem git vor: HOME und
// XDG_CONFIG_HOME im Temp-Verzeichnis, keine Systemkonfiguration. Es gibt
// die Basisumgebung des Aufrufers zurück.
func realGit(t *testing.T) (home string, environ []string) {
	t.Helper()
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git liegt nicht im PATH")
	}
	home = t.TempDir()
	environ = []string{
		"PATH=" + filepath.Dir(gitPath) + ":/usr/bin:/bin",
		"HOME=" + home,
		"XDG_CONFIG_HOME=" + filepath.Join(home, ".config"),
		"GIT_CONFIG_NOSYSTEM=1",
		"LANG=C",
	}
	return home, environ
}

func gitIn(t *testing.T, env []string, stdin string, args ...string) (string, error) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Env = env
	command.Dir = t.TempDir()
	command.Stdin = strings.NewReader(stdin)
	out, err := command.CombinedOutput()
	return string(out), err
}

// Jede Zugangsart lenkt jede Form des Repositorys auf ihre Zielform, auch
// die Form eines zweiten host-Alias desselben Dienstes.
func TestChildEnvUmlenkungMitEchtemGit(t *testing.T) {
	_, environ := realGit(t)
	alias := Access{Type: "ssh", Host: "github-acme", State: StateAvailable}
	second := Access{Type: "ssh", Host: "github-alt", State: StateAvailable}
	cases := []struct {
		name    string
		service Service
		target  string
	}{
		{"gh", runService(Access{Type: "gh", Login: "alice", State: StateAvailable}, alias, second), "https://github.com/acme/web"},
		{"public", runService(Access{Type: "public", State: StateAvailable}, alias, second), "https://github.com/acme/web"},
		{"ssh key", runService(Access{Type: "ssh", Key: "/k/id", State: StateAvailable}, alias, second), "git@github.com:acme/web"},
		{"ssh host", runService(alias, second), "git@github-acme:acme/web"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			choice := Choose(c.service, ChooseOptions{})
			token := ""
			if choice.API != nil {
				token = canary
			}
			env, err := ChildEnv(environ, c.service, choice, token)
			if err != nil {
				t.Fatal(err)
			}
			for _, form := range []string{"https://github.com/acme/web", "git@github.com:acme/web", "ssh://git@github.com/acme/web", "git@github-acme:acme/web", "git@github-alt:acme/web"} {
				for _, suffix := range []string{".git", ""} {
					got, err := gitIn(t, env, "", "ls-remote", "--get-url", form+suffix)
					if err != nil {
						t.Fatalf("git ls-remote --get-url: %v %s", err, got)
					}
					expected := c.target + suffix
					if strings.TrimSpace(got) != expected {
						t.Errorf("%s → %s, erwartet %s", form+suffix, strings.TrimSpace(got), expected)
					}
				}
			}
			// Ein fremdes Repository bleibt, wie es ist.
			for _, foreign := range []string{"git@github.com:acme/other.git", "https://github.com/other/web.git", "ssh://git@github.com/acme/other.git", "git@github-acme:acme/other.git", "git@github-alt:acme/other.git"} {
				got, _ := gitIn(t, env, "", "ls-remote", "--get-url", foreign)
				if strings.TrimSpace(got) != foreign {
					t.Errorf("fremdes Repo %s → %s", foreign, strings.TrimSpace(got))
				}
			}
		})
	}
}

// Umlenkungen des Rechners für ganz github.com, als insteadOf und als
// pushInsteadOf, in der globalen Konfiguration des Temp-HOME: https auf SSH
// und jede SSH-Form (ssh://, beide Aliase) woandershin. Unter run geht jede
// Form des Repositorys bei jeder Zugangsart auf die gewählte Form, beim
// Abholen wie beim Push; fremde Repositorys bleiben, wie der Rechner sie hat.
func TestChildEnvGegenUmlenkungenDesRechners(t *testing.T) {
	_, environ := realGit(t)
	for _, kind := range []string{"insteadOf", "pushInsteadOf"} {
		for target, source := range map[string]string{
			"git@github.com:":      "https://github.com/",
			"https://github.com/":  "git@github.com:",
			"https://example.org/": "git@github-acme:",
			"https://example.net/": "ssh://git@github.com/",
			"https://example.com/": "git@github-alt:",
		} {
			if out, err := gitIn(t, environ, "", "config", "--global", "--add", "url."+target+"."+kind, source); err != nil {
				t.Fatalf("git config: %v %s", err, out)
			}
		}
	}
	repo := t.TempDir()
	if out, err := gitIn(t, environ, "", "init", "-q", repo); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	// urls nennt die Fetch- und die Push-URL von origin = url.
	urls := func(env []string, url string) (string, string) {
		t.Helper()
		set := exec.Command("git", "-C", repo, "config", "remote.origin.url", url)
		set.Env = environ
		if out, err := set.CombinedOutput(); err != nil {
			t.Fatalf("git config remote.origin.url: %v %s", err, out)
		}
		var got [2]string
		for i, args := range [][]string{{"remote", "get-url", "origin"}, {"remote", "get-url", "--push", "origin"}} {
			command := exec.Command("git", append([]string{"-C", repo}, args...)...)
			command.Env = env
			out, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("git %v: %v %s", args, err, out)
			}
			got[i] = strings.TrimSpace(string(out))
		}
		return got[0], got[1]
	}
	// Ohne run lenkt der Rechner wirklich um, beim Abholen wie beim Push.
	if fetch, push := urls(environ, "https://github.com/acme/web.git"); fetch != "git@github.com:acme/web.git" || push != "git@github.com:acme/web.git" {
		t.Fatalf("Umlenkung des Rechners greift nicht: %s %s", fetch, push)
	}

	alias := Access{Type: "ssh", Host: "github-acme", State: StateAvailable}
	second := Access{Type: "ssh", Host: "github-alt", State: StateAvailable}
	cases := []struct {
		name    string
		service Service
		target  string
	}{
		{"token", runService(Access{Type: "token", File: "/t/a.token", State: StateAvailable}, alias, second), "https://github.com/acme/web"},
		{"gh", runService(Access{Type: "gh", Login: "alice", State: StateAvailable}, alias, second), "https://github.com/acme/web"},
		{"public", runService(Access{Type: "public", State: StateAvailable}, alias, second), "https://github.com/acme/web"},
		{"ssh key", runService(Access{Type: "ssh", Key: "/k/id", State: StateAvailable}, alias, second), "git@github.com:acme/web"},
		{"ssh host", runService(alias, second), "git@github-acme:acme/web"},
	}
	all := []string{"https://github.com/acme/web", "git@github.com:acme/web", "ssh://git@github.com/acme/web", "git@github-acme:acme/web", "git@github-alt:acme/web"}
	foreign := []string{"https://github.com/other/web.git", "git@github.com:acme/other.git", "ssh://git@github.com/acme/other.git", "git@github-acme:acme/other.git", "git@github-alt:acme/other.git"}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			choice := Choose(c.service, ChooseOptions{})
			token := ""
			if choice.API != nil {
				token = canary
			}
			env, err := ChildEnv(environ, c.service, choice, token)
			if err != nil {
				t.Fatal(err)
			}
			for _, form := range all {
				for _, suffix := range []string{".git", ""} {
					fetch, push := urls(env, form+suffix)
					if expected := c.target + suffix; fetch != expected || push != expected {
						t.Errorf("%s → fetch %s, push %s; erwartet %s", form+suffix, fetch, push, expected)
					}
				}
			}
			for _, url := range foreign {
				fetch, push := urls(env, url)
				machineFetch, machinePush := urls(environ, url)
				if fetch != machineFetch || push != machinePush || fetch == url {
					t.Errorf("fremdes Repo %s → %s, %s; der Rechner hat %s, %s", url, fetch, push, machineFetch, machinePush)
				}
			}
		})
	}
}

// Die Einträge des Aufrufers bleiben wirksam, auch mit echtem Transport:
// Sein insteadOf für ein fremdes Repository auf ein file://-Ziel greift im
// Kindprozess, ls-remote erreicht das Ziel, und der Helper des Dienstes
// passt nur auf dessen URL.
func TestChildEnvBehaeltEintraegeDesAufrufers(t *testing.T) {
	_, environ := realGit(t)
	bare := filepath.Join(t.TempDir(), "fremd.git")
	if out, err := gitIn(t, environ, "", "init", "--bare", "-q", bare); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	environ = append(environ, "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=url.file://"+bare+".insteadOf", "GIT_CONFIG_VALUE_0=https://github.com/other/fremd.git")
	service := runService(Access{Type: "token", File: "/t/a.token", State: StateAvailable})
	env, err := ChildEnv(environ, service, Choose(service, ChooseOptions{}), canary)
	if err != nil {
		t.Fatal(err)
	}
	if out, err := gitIn(t, env, "", "ls-remote", "https://github.com/other/fremd.git"); err != nil {
		t.Errorf("ls-remote über das insteadOf des Aufrufers: %v %s", err, out)
	}
	got, err := gitIn(t, env, "", "config", "--get-urlmatch", "credential.helper", "https://github.com/acme/web.git")
	if err != nil || strings.TrimSpace(got) != CredentialHelper {
		t.Errorf("Helper für das Repo: %q %v", got, err)
	}
	if got, err := gitIn(t, env, "", "config", "--get-urlmatch", "credential.helper", "https://github.com/acme/webseite.git"); err == nil {
		t.Errorf("Helper greift beim Nachbarn acme/webseite.git: %q", got)
	}
}

// Der Helper des Dienstes ersetzt den store-Helper des Rechners für die
// URLs des Repositorys: fill liefert das Token, approve legt nichts ab. Für
// ein fremdes Repository bleibt der store-Helper, wie er ist.
func TestChildEnvCredentialHelperGegenStore(t *testing.T) {
	home, environ := realGit(t)
	if out, err := gitIn(t, environ, "", "config", "--global", "credential.helper", "store"); err != nil {
		t.Fatalf("git config: %v %s", err, out)
	}
	stored := "https://gespeichert:STOREDPW@github.com\n"
	if err := os.WriteFile(filepath.Join(home, ".git-credentials"), []byte(stored), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, typ := range []string{"gh", "token"} {
		t.Run(typ, func(t *testing.T) {
			service := runService(Access{Type: typ, Login: "alice", File: "/t/a.token", State: StateAvailable})
			env, err := ChildEnv(environ, service, Choose(service, ChooseOptions{}), canary)
			if err != nil {
				t.Fatal(err)
			}
			for _, url := range []string{"https://github.com/acme/web.git", "https://github.com/acme/web"} {
				out, err := gitIn(t, env, "url="+url+"\n\n", "credential", "fill")
				if err != nil || !strings.Contains(out, "password="+canary+"\n") || strings.Contains(out, "STOREDPW") {
					t.Errorf("fill %s: %v\n%s", url, err, strings.ReplaceAll(out, canary, "[token]"))
				}
				if out, err := gitIn(t, env, "url="+url+"\nusername=x-access-token\npassword="+canary+"\n\n", "credential", "approve"); err != nil {
					t.Errorf("approve %s: %v %s", url, err, out)
				}
			}
			out, err := gitIn(t, env, "url=https://github.com/other/x.git\n\n", "credential", "fill")
			if err != nil || !strings.Contains(out, "password=STOREDPW\n") {
				t.Errorf("fill fremdes Repo: %v\n%s", err, out)
			}
		})
	}
	// public setzt den store-Helper ebenfalls zurück: kein gespeichertes
	// Credential für das Repo, also scheitert fill ohne Terminal.
	service := runService(Access{Type: "public", State: StateAvailable})
	env, err := ChildEnv(environ, service, Choose(service, ChooseOptions{}), "")
	if err != nil {
		t.Fatal(err)
	}
	if out, err := gitIn(t, env, "url=https://github.com/acme/web.git\n\n", "credential", "fill"); err == nil || strings.Contains(out, "STOREDPW") {
		t.Errorf("public fill: %v\n%s", err, out)
	}

	// Danach steht das Token in keiner Datei unter HOME, und
	// .git-credentials ist unverändert.
	err = filepath.WalkDir(home, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, readErr := os.ReadFile(path)
		if readErr == nil && strings.Contains(string(data), canary) {
			t.Errorf("Token steht in %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(filepath.Join(home, ".git-credentials")); string(data) != stored {
		t.Errorf(".git-credentials verändert: %q", data)
	}
}
