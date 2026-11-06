package githubaccess

import "testing"

func TestRepoOfRemoteAlleFormen(t *testing.T) {
	cases := []struct {
		remote, alias, want string
	}{
		// https, mit und ohne .git, mit Schrägstrich am Ende
		{"https://github.com/kascada/k-playbook.git", "", "kascada/k-playbook"},
		{"https://github.com/kascada/k-playbook", "", "kascada/k-playbook"},
		{"https://github.com/kascada/k-playbook/", "", "kascada/k-playbook"},
		{"  https://github.com/kascada/k-playbook.git\n", "", "kascada/k-playbook"},
		// https mit Zugangsdaten: Sie zählen nicht, das Repo bleibt.
		{"https://x-access-token:ghp_geheim@github.com/kascada/k-playbook.git", "", "kascada/k-playbook"},
		{"https://kkl_kmde@github.com/kascada/k-playbook", "", "kascada/k-playbook"},
		// scp-Form auf github.com
		{"git@github.com:kascada/k-playbook.git", "", "kascada/k-playbook"},
		{"git@github.com:kascada/k-playbook", "", "kascada/k-playbook"},
		{"github.com:kascada/k-playbook.git", "", "kascada/k-playbook"},
		// ssh://
		{"ssh://git@github.com/kascada/k-playbook.git", "", "kascada/k-playbook"},
		{"ssh://git@github.com/kascada/k-playbook", "", "kascada/k-playbook"},
		{"ssh://github.com/kascada/k-playbook.git", "", "kascada/k-playbook"},
		// SSH-Alias, auch ohne Benutzer (der steht dann in ~/.ssh/config)
		{"git@github-kamranbycloud:kascada/k-playbook.git", "github-kamranbycloud", "kascada/k-playbook"},
		{"git@github-kamranbycloud:kascada/k-playbook", "github-kamranbycloud", "kascada/k-playbook"},
		{"github-kamranbycloud:kascada/k-playbook.git", "github-kamranbycloud", "kascada/k-playbook"},
		{"git@github.com-km:koelnmesse-IT/omni-gw.git", "github.com-km", "koelnmesse-IT/omni-gw"},
		// Groß- und Kleinschreibung: Host und Schema zählen nicht, owner/repo
		// kommt in der Schreibung des Eintrags zurück.
		{"HTTPS://GitHub.com/Kascada/K-Playbook.git", "", "Kascada/K-Playbook"},
		{"git@GITHUB.COM:Kascada/K-Playbook.git", "", "Kascada/K-Playbook"},
		{"git@GitHub-KamranByCloud:kascada/k-playbook.git", "github-kamranbycloud", "kascada/k-playbook"},

		// Fremde Hosts und andere Formen: leer.
		{"https://gitlab.com/kascada/k-playbook.git", "", ""},
		{"git@gitlab.com:kascada/k-playbook.git", "", ""},
		{"http://github.com/kascada/k-playbook.git", "", ""},
		{"https://github.com/kascada/k-playbook/tree/main", "", ""},
		{"https://github.com/kascada", "", ""},
		{"git@github.com:/kascada/k-playbook.git", "", ""},
		{"ssh://git@github-zweit:2222/kascada/k-playbook", "github-zweit", ""},
		{"/srv/git/k-playbook.git", "", ""},
		{"../remote.git", "", ""},
		{"file:///srv/kascada/k-playbook.git", "", ""},
		{"", "", ""},
		// Ein Alias-Remote ohne anerkannten Alias ist keine Form von run.
		{"git@github-kamranbycloud:kascada/k-playbook.git", "", ""},
	}
	for _, c := range cases {
		if got := RepoOfRemote(c.remote, c.alias); got != c.want {
			t.Errorf("RepoOfRemote(%q, %q) = %q, erwartet %q", c.remote, c.alias, got, c.want)
		}
	}
}

func TestRemoteWithoutCredentials(t *testing.T) {
	cases := map[string]string{
		"https://x-access-token:ghp_geheim@github.com/kascada/k-playbook.git": "https://github.com/kascada/k-playbook.git",
		"https://user:pa@ss@github.com/kascada/k-playbook.git":                "https://github.com/kascada/k-playbook.git",
		"https://kkl_kmde@github.com/kascada/k-playbook":                      "https://github.com/kascada/k-playbook",
		"ssh://git@github.com/kascada/k-playbook.git":                         "ssh://github.com/kascada/k-playbook.git",
		"https://github.com/kascada/k-playbook.git":                           "https://github.com/kascada/k-playbook.git",
		"https://token@github.com":                                            "https://github.com",
		// Die scp-Form bleibt: Sie kann kein Passwort tragen.
		"git@github-kamranbycloud:kascada/k-playbook.git": "git@github-kamranbycloud:kascada/k-playbook.git",
		"/srv/git/k-playbook.git":                         "/srv/git/k-playbook.git",
		"":                                                "",
	}
	for remote, want := range cases {
		if got := RemoteWithoutCredentials(remote); got != want {
			t.Errorf("RemoteWithoutCredentials(%q) = %q, erwartet %q", remote, got, want)
		}
	}
}

func TestNamesSSHHost(t *testing.T) {
	services := Services{
		{Name: "a", Repo: "o/a", Access: []Access{{Type: "gh", Login: "alice"}}},
		{Name: "b", Repo: "o/b", Access: []Access{{Type: "ssh", Key: "/k"}, {Type: "ssh", Host: "github-b", Disabled: true}}},
	}
	if !services.NamesSSHHost("github-b") || !services.NamesSSHHost("GitHub-B") {
		t.Error("der Alias eines deaktivierten Zugangs zählt auch")
	}
	if services.NamesSSHHost("github-a") || services.NamesSSHHost("") {
		t.Error("ein Alias, den kein Dienst nennt, zählt nicht")
	}
}

func TestSSHHostIsGitHub(t *testing.T) {
	fakeTools(t, false)
	if !SSHHostIsGitHub("github-example") {
		t.Error("github-example löst nach github.com auf")
	}
	for _, host := range []string{"gitlab.com", "kaputt", "-oProxyCommand=x"} {
		if SSHHostIsGitHub(host) {
			t.Errorf("%s ist kein GitHub-Alias", host)
		}
	}
}
