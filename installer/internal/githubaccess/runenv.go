package githubaccess

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/ghconfig"
)

// Die Einrichtung für `k-playbook github run` (docs/github-access.md, What is
// set up): alles nur in der Umgebung des Befehls. Keine Datei, keine
// git-Konfiguration außerhalb, kein gh auth switch. git bekommt seine
// Einträge über GIT_CONFIG_COUNT/GIT_CONFIG_KEY_n/GIT_CONFIG_VALUE_n hinter
// denen des Aufrufers; das Token steht nur in GH_TOKEN und GITHUB_TOKEN.

// CredentialHelper ist der Credential-Helper für die URLs des Repositorys,
// wenn git über token oder gh läuft. Er liest das Token aus der Umgebung des
// Prozesses, den git startet — der Wert in der git-Konfiguration trägt es
// nie. Er antwortet nur auf get; store und erase ignoriert er, denn es gibt
// nichts abzulegen. git hängt die Aktion als Argument an.
const CredentialHelper = `!f() { test "$1" = get || return 0; echo username=x-access-token; echo "password=${GH_TOKEN:-$GITHUB_TOKEN}"; }; f`

// ConfigEntry ist ein Eintrag der git-Konfiguration für den Befehl.
type ConfigEntry struct {
	Key   string
	Value string
}

// repoForms sind die Formen der URL eines Repositorys, auf die sich
// Umlenkungen und Helper beschränken, jeweils ohne .git; die Form mit .git
// entsteht beim Eintragen.
type repoForms struct {
	https   string   // https://github.com/<owner>/<repo>
	scp     string   // git@github.com:<owner>/<repo>
	sshURL  string   // ssh://git@github.com/<owner>/<repo>
	aliases []string // git@<host>:<owner>/<repo> je host-Alias des Dienstes
}

func formsOf(service Service) repoForms {
	forms := repoForms{
		https:  "https://" + ghconfig.Host + "/" + service.Repo,
		scp:    "git@" + ghconfig.Host + ":" + service.Repo,
		sshURL: "ssh://git@" + ghconfig.Host + "/" + service.Repo,
	}
	seen := map[string]bool{}
	for _, access := range service.Access {
		// Jeder Alias, den der Dienst nennt, auch ein deaktivierter: Ein
		// Remote in dieser Form meint dasselbe Repository.
		if access.Type != "ssh" || access.Host == "" || seen[access.Host] {
			continue
		}
		seen[access.Host] = true
		if access.Host == ghconfig.Host {
			continue // ist schon die scp-Form
		}
		forms.aliases = append(forms.aliases, "git@"+access.Host+":"+service.Repo)
	}
	return forms
}

// all sind alle Formen des Repositorys: https, git@github.com:,
// ssh://git@github.com/ und die Aliase.
func (f repoForms) all() []string {
	return append([]string{f.https, f.scp, f.sshURL}, f.aliases...)
}

// GitConfig nennt die Einträge der git-Konfiguration, die run für den
// git-Kanal einrichtet, in dieser Reihenfolge. Ohne Zugang für git: keine.
// Jede Zugangsart lenkt alle Formen des Repositorys auf ihre Zielform um;
// sie unterscheiden sich nur in der Zielform und in den Helpern.
func GitConfig(service Service, git *Access) []ConfigEntry {
	if git == nil {
		return nil
	}
	forms := formsOf(service)
	var target string
	switch {
	case git.Type == "token" || git.Type == "gh" || git.Type == "public":
		target = forms.https
	case git.Type == "ssh" && git.Key != "":
		target = forms.scp
	case git.Type == "ssh":
		target = "git@" + git.Host + ":" + service.Repo
	default:
		return nil
	}
	var entries []ConfigEntry
	// rewrite lenkt jede Quellform auf target um, mit und ohne .git: git
	// nimmt das längste passende insteadOf, die Form mit .git trifft also
	// genau, die ohne auch einen Nachbarn mit gleichem Anfang (hingenommen,
	// siehe Vertrag). Zuerst wird target auf sich selbst gelenkt: Eine
	// Umlenkung des Rechners wie url.git@github.com:.insteadOf
	// https://github.com/ ist kürzer und kommt deshalb nicht mehr zum Zug.
	// Jede Umlenkung steht zweimal, als insteadOf und als pushInsteadOf: git
	// sucht die Push-URL zuerst unter pushInsteadOf, und ein pushInsteadOf
	// des Rechners ginge sonst jedem insteadOf von run vor.
	rewrite := func(sources ...string) {
		for i, source := range append([]string{target}, sources...) {
			if i > 0 && source == target {
				continue
			}
			for _, kind := range []string{"insteadOf", "pushInsteadOf"} {
				entries = append(entries,
					ConfigEntry{"url." + target + ".git." + kind, source + ".git"},
					ConfigEntry{"url." + target + "." + kind, source})
			}
		}
	}
	// helpers setzt die Helper des Rechners für die https-Formen zurück
	// (leerer Eintrag) und trägt danach, wenn gewünscht, den eigenen ein.
	// Das Zurücksetzen verhindert, dass git nach einer erfolgreichen
	// Anmeldung store bei einem Helper des Rechners aufruft und das Token
	// außerhalb des Befehls ablegt — oder bei public ein gespeichertes
	// Credential nimmt.
	helpers := func(own bool) {
		for _, url := range []string{forms.https + ".git", forms.https} {
			entries = append(entries, ConfigEntry{"credential." + url + ".helper", ""})
			if own {
				entries = append(entries, ConfigEntry{"credential." + url + ".helper", CredentialHelper})
			}
		}
	}

	rewrite(forms.all()...)
	switch git.Type {
	case "token", "gh":
		helpers(true)
	case "public":
		helpers(false)
	}
	return entries
}

// SSHCommand ist GIT_SSH_COMMAND für einen ssh-Zugang mit key.
func SSHCommand(key string) string {
	return "ssh -i " + shellQuote(key) + " -o IdentitiesOnly=yes"
}

// ChildEnv baut aus der Umgebung des Aufrufers (environ, wie os.Environ)
// die des Befehls: die git-Einträge hinter vorhandenen GIT_CONFIG_*,
// GIT_SSH_COMMAND für einen ssh-Zugang mit key, GH_TOKEN und GITHUB_TOKEN
// gesetzt (api über token oder gh) oder entfernt, GIT_TERMINAL_PROMPT=0.
// Alles andere bleibt, wie es ist. token ist das Token des api-Kanals; es
// erscheint nur in GH_TOKEN und GITHUB_TOKEN.
func ChildEnv(environ []string, service Service, choice Choice, token string) ([]string, error) {
	if choice.API != nil && token == "" {
		return nil, errors.New("für den api-Kanal fehlt das Token")
	}
	count := 0
	if raw, ok := lookupEnv(environ, "GIT_CONFIG_COUNT"); ok && raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			return nil, fmt.Errorf("GIT_CONFIG_COUNT=%s ist keine Zahl ab 0 — die Variable korrigieren oder entfernen", raw)
		}
		count = n
	}
	entries := GitConfig(service, choice.Git)

	// Was run selbst setzt, wird aus der geerbten Umgebung genommen: die
	// eigenen Indizes von GIT_CONFIG_*, die Token-Variablen,
	// GIT_TERMINAL_PROMPT und bei key GIT_SSH_COMMAND.
	drop := map[string]bool{"GIT_CONFIG_COUNT": true, "GH_TOKEN": true, "GITHUB_TOKEN": true, "GIT_TERMINAL_PROMPT": true}
	for i := range entries {
		drop[fmt.Sprintf("GIT_CONFIG_KEY_%d", count+i)] = true
		drop[fmt.Sprintf("GIT_CONFIG_VALUE_%d", count+i)] = true
	}
	sshKey := choice.Git != nil && choice.Git.Type == "ssh" && choice.Git.Key != ""
	if sshKey {
		drop["GIT_SSH_COMMAND"] = true
	}
	env := make([]string, 0, len(environ)+2*len(entries)+5)
	for _, entry := range environ {
		name, _, _ := strings.Cut(entry, "=")
		if !drop[name] {
			env = append(env, entry)
		}
	}

	for i, entry := range entries {
		env = append(env,
			fmt.Sprintf("GIT_CONFIG_KEY_%d=%s", count+i, entry.Key),
			fmt.Sprintf("GIT_CONFIG_VALUE_%d=%s", count+i, entry.Value))
	}
	if count+len(entries) > 0 {
		env = append(env, "GIT_CONFIG_COUNT="+strconv.Itoa(count+len(entries)))
	}
	if sshKey {
		env = append(env, "GIT_SSH_COMMAND="+SSHCommand(choice.Git.Key))
	}
	if choice.API != nil {
		env = append(env, "GH_TOKEN="+token, "GITHUB_TOKEN="+token)
	}
	env = append(env, "GIT_TERMINAL_PROMPT=0")
	return env, nil
}

func lookupEnv(environ []string, name string) (string, bool) {
	value, found := "", false
	for _, entry := range environ {
		if key, rest, ok := strings.Cut(entry, "="); ok && key == name {
			value, found = rest, true // wie exec: der letzte gilt
		}
	}
	return value, found
}

// ReadToken holt das Token eines Zugangs vom Typ gh oder token: für gh mit
// `gh auth token --hostname github.com --user <login>` (ohne GH_TOKEN und
// GITHUB_TOKEN in dessen Umgebung, sonst gäbe gh das der Umgebung zurück),
// für token aus der Datei. Eine Fehlermeldung trägt das Token nie.
func ReadToken(ctx context.Context, access Access) (string, error) {
	switch access.Type {
	case "gh":
		tester := NewTester()
		stdout, stderr, err := tester.run(ctx, nil, "gh", "auth", "token", "--hostname", ghconfig.Host, "--user", access.Login)
		token := strings.TrimSpace(stdout)
		if err != nil || token == "" {
			if err == nil {
				err = errors.New("gh hat kein Token ausgegeben")
			}
			return "", errors.New(redact(sshFailure("gh auth token für "+access.Login, stderr, err, tester.timeout()), token))
		}
		return token, nil
	case "token":
		data, err := os.ReadFile(access.File)
		if err != nil {
			return "", errors.New("Die Token-Datei " + access.File + " ist nicht lesbar: " + describeReadError(err) + ".")
		}
		token := strings.TrimSpace(string(data))
		if token == "" {
			return "", errors.New("Die Token-Datei " + access.File + " ist leer.")
		}
		return token, nil
	}
	return "", errors.New("Zugang vom Typ " + access.Type + " trägt kein Token")
}

// RunLine ist die Zeile, die run vor dem Start auf stderr schreibt, etwa
// „github run OMNI-GW: git über ssh host github.com-km, api über gh kkl_kmde“.
// Der git-Teil nennt Typ und Schlüssel des Zugangs (Pfade mit ~ für home);
// der api-Teil nur den Typ, wenn es derselbe Zugang wie für git ist, sonst
// ebenfalls Typ und Schlüssel. Ein leerer Kanal heißt „git: keiner“ bzw.
// „api: keiner“.
func RunLine(name string, choice Choice, home string) string {
	part := func(channel Channel, access *Access, short bool) string {
		if access == nil {
			return string(channel) + ": keiner"
		}
		if short {
			return string(channel) + " über " + access.Type
		}
		return string(channel) + " über " + describeForLine(*access, home)
	}
	return "github run " + name + ": " + part(ChannelGit, choice.Git, false) + ", " + part(ChannelAPI, choice.API, choice.SameAccess())
}

// describeForLine nennt einen Zugang wie in den Beispielen des Vertrags:
// „token ~/…/x.token“, „gh alice“, „ssh host github-example“,
// „ssh key ~/.ssh/id“, „public“.
func describeForLine(access Access, home string) string {
	switch {
	case access.Type == "gh":
		return "gh " + access.Login
	case access.Type == "token":
		return "token " + tildePath(access.File, home)
	case access.Type == "ssh" && access.Key != "":
		return "ssh key " + tildePath(access.Key, home)
	case access.Type == "ssh":
		return "ssh host " + access.Host
	}
	return access.Type
}

// tildePath kürzt einen Pfad unter home auf ~/….
func tildePath(path string, home string) string {
	if home == "" || !filepath.IsAbs(home) {
		return path
	}
	home = filepath.Clean(home)
	if rest, ok := strings.CutPrefix(path, home+string(filepath.Separator)); ok {
		return "~/" + rest
	}
	return path
}
