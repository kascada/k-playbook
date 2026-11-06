package githubaccess

import "strings"

// Die Zuordnung eines Remotes zu einem Repository (Task 119): aus dem rohen
// Eintrag remote.<name>.url — nicht aus `git remote get-url`, das insteadOf
// anwendet, auch das von run — wird <owner>/<repo>, wenn der Eintrag eine
// der Formen hat, die run umlenkt (formsOf). Den SSH-Host eines Eintrags
// erkennt der Aufrufer (internal/github.SSHAliasOf); dieses Paket hängt
// davon nicht ab.

// RemoteWithoutCredentials nimmt aus einem Remote-Eintrag mit Schema
// (https://, ssh://, jedes <schema>://) den Teil vor dem @ der Autorität
// ganz heraus — Benutzer und Passwort oder Token. Aus
// https://x-access-token:<token>@github.com/o/r.git wird
// https://github.com/o/r.git. Die scp-Form [user@]host:pfad bleibt, wie sie
// ist: Sie kann kein Passwort tragen.
func RemoteWithoutCredentials(remote string) string {
	remote = strings.TrimSpace(remote)
	scheme, rest, found := strings.Cut(remote, "://")
	if !found || scheme == "" || strings.ContainsAny(scheme, "/@:") {
		return remote
	}
	authority, path, hasPath := strings.Cut(rest, "/")
	// Das letzte @ der Autorität: Ein Token kann selbst eins tragen,
	// der Host nie.
	if at := strings.LastIndex(authority, "@"); at >= 0 {
		authority = authority[at+1:]
	}
	if !hasPath {
		return scheme + "://" + authority
	}
	return scheme + "://" + authority + "/" + path
}

// RepoOfRemote liest <owner>/<repo> aus einem rohen Remote-Eintrag, so wie
// er in der Datei steht. alias ist der SSH-Host des Eintrags, wenn er nicht
// github.com ist — leer bei https und bei github.com; der Aufrufer hat ihn
// schon als GitHub-Alias anerkannt. Gelten die Formen, die run umlenkt
// (formsOf): https://github.com/<owner>/<repo>, git@github.com:<owner>/<repo>,
// ssh://git@github.com/<owner>/<repo> und git@<alias>:<owner>/<repo>, jede
// mit oder ohne .git und abschließenden Schrägstrich; Groß- und
// Kleinschreibung zählen nicht. Zugangsdaten vor dem @ einer Form mit Schema
// fallen vor dem Vergleich weg, und der Benutzer einer SSH-Form zählt nicht
// (GitHub nimmt ohnehin nur git) — so trifft auch ein Alias, dessen Benutzer
// in ~/.ssh/config steht (<alias>:<owner>/<repo>). Jede andere Form, jeder
// fremde Host und ein Pfad, der nicht genau <owner>/<repo> ist, ergeben "".
// Zurück kommt owner/repo in der Schreibung des Eintrags.
func RepoOfRemote(remote string, alias string) string {
	normalized, candidate := normalizeRemote(remote)
	if !validRepo(candidate) {
		return ""
	}
	service := Service{Repo: candidate}
	if alias != "" {
		service.Access = []Access{{Type: "ssh", Host: alias}}
	}
	for _, form := range formsOf(service).all() {
		if strings.EqualFold(normalized, form) {
			return candidate
		}
	}
	return ""
}

// normalizeRemote bringt einen Eintrag in die Schreibweise von formsOf und
// nennt den Pfad dahinter: ohne Leerraum, abschließenden Schrägstrich und
// .git; bei https ohne Zugangsdaten; bei ssh:// und der scp-Form mit dem
// Benutzer git.
func normalizeRemote(remote string) (normalized string, path string) {
	remote = strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(remote), "/"), ".git")
	if strings.Contains(remote, "://") {
		remote = RemoteWithoutCredentials(remote)
		scheme, rest, _ := strings.Cut(remote, "://")
		host, path, _ := strings.Cut(rest, "/")
		if strings.EqualFold(scheme, "ssh") {
			return "ssh://git@" + host + "/" + path, path
		}
		return remote, path
	}
	host, path, found := strings.Cut(remote, ":")
	if !found {
		return remote, ""
	}
	if at := strings.LastIndex(host, "@"); at >= 0 {
		host = host[at+1:]
	}
	return "git@" + host + ":" + path, path
}

// validRepo sagt, ob path genau <owner>/<repo> ist: zwei nicht leere Teile
// ohne Leerraum.
func validRepo(path string) bool {
	owner, repo, found := strings.Cut(path, "/")
	return found && owner != "" && repo != "" && !strings.Contains(repo, "/") &&
		!strings.ContainsAny(path, " \t\r\n\\")
}

// NamesSSHHost sagt, ob ein Dienst host als SSH-Alias nennt — in einem
// Zugang vom Typ ssh mit host, auch einem deaktivierten. Ein Remote mit
// diesem Host ist dann eine Form, die run umlenkt (formsOf), und gilt ohne
// weiteren Blick als GitHub. Der Vergleich ignoriert Groß- und
// Kleinschreibung wie ssh.
func (s Services) NamesSSHHost(host string) bool {
	for _, service := range s {
		for _, access := range service.Access {
			if access.Type == "ssh" && access.Host != "" && strings.EqualFold(access.Host, host) {
				return true
			}
		}
	}
	return false
}

// SSHHostIsGitHub sagt, ob `ssh -G <host>` nach github.com auflöst — derselbe
// lokale Blick in die SSH-Konfiguration wie bei der Verfügbarkeit eines
// ssh-Zugangs (judgeSSHHost), mit derselben Frist sshResolveTimeout. Kein
// Netz; ein Host, der nicht auflöst, scheitert oder die Frist überschreitet,
// ist keiner.
func SSHHostIsGitHub(host string) bool {
	state, _ := judgeSSHHost(host)
	return state == StateAvailable
}
