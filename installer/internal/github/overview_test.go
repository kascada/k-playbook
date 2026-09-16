package github

import (
	"context"
	"strings"
	"testing"
)

const repoViewJSON = `{"defaultBranchRef":{"name":"main"},"isPrivate":false,` +
	`"nameWithOwner":"kascada/k-playbook","url":"https://github.com/kascada/k-playbook",` +
	`"viewerPermission":"READ"}`

const runListJSON = `[
 {"conclusion":"success","createdAt":"2026-09-15T17:39:55Z","databaseId":35002675524,
  "displayTitle":"Release v0.9.0","event":"push","headBranch":"main","startedAt":"2026-09-15T17:39:55Z",
  "status":"completed","updatedAt":"2026-09-15T17:40:04Z",
  "url":"https://github.com/kascada/k-playbook/actions/runs/35002675524","workflowName":"VERSION-Wächter"},
 {"conclusion":"failure","createdAt":"2026-09-15T17:39:55Z","databaseId":35002675449,
  "displayTitle":"Release v0.9.0","event":"push","headBranch":"main","startedAt":"2026-09-15T17:39:55Z",
  "status":"completed","updatedAt":"2026-09-15T17:40:46Z",
  "url":"https://github.com/kascada/k-playbook/actions/runs/35002675449","workflowName":"Tests"},
 {"conclusion":"failure","createdAt":"2026-09-14T09:00:00Z","databaseId":34000000000,
  "displayTitle":"Zwischenstand","event":"push","headBranch":"main","startedAt":"2026-09-14T09:00:00Z",
  "status":"completed","updatedAt":"2026-09-14T09:01:00Z",
  "url":"https://github.com/kascada/k-playbook/actions/runs/34000000000","workflowName":"Tests"}
]`

func healthyRunner() *fakeRunner {
	return &fakeRunner{answers: []fakeAnswer{
		{prefix: "gh repo view", out: repoViewJSON},
		{prefix: "gh api user", out: `{"login":"kkl_kmde","id":307552572}`},
		{prefix: "gh run list", out: runListJSON},
		{prefix: "git remote get-url", out: "git@github-kamranbycloud:kascada/k-playbook.git\n"},
		{prefix: "git describe", out: "v0.9.0\n"},
		{prefix: "git rev-list", out: "3\n"},
		{prefix: "git log", out: "2026-09-15T19:35:12+02:00\n"},
	}}
}

func TestOverviewLiestRepoKontoUndRecht(t *testing.T) {
	runner := healthyRunner()
	overview := runner.client().FetchOverview(context.Background())

	if overview.State != StateOK {
		t.Fatalf("Zustand %q, erwartet ok (%s)", overview.State, overview.Message)
	}
	if overview.Repo != "kascada/k-playbook" {
		t.Errorf("Repo %q, erwartet kascada/k-playbook", overview.Repo)
	}
	if overview.DefaultBranch != "main" {
		t.Errorf("Default-Branch %q, erwartet main", overview.DefaultBranch)
	}
	if overview.Account != "kkl_kmde" {
		t.Errorf("Konto %q, erwartet kkl_kmde", overview.Account)
	}
	// Leserecht ist nicht Schreibrecht: „angemeldet" allein sagt nichts
	// darüber, ob über gh gemergt werden kann.
	if overview.Permission != "READ" {
		t.Errorf("Recht %q, erwartet READ", overview.Permission)
	}
	if overview.LastTag != "v0.9.0" || overview.CommitsSinceTag != 3 {
		t.Errorf("Tag %q mit %d Commits, erwartet v0.9.0 mit 3", overview.LastTag, overview.CommitsSinceTag)
	}
}

func TestOverviewNenntDenSSHAliasOhneKonto(t *testing.T) {
	overview := healthyRunner().client().FetchOverview(context.Background())

	if overview.SSHAlias != "github-kamranbycloud" {
		t.Fatalf("SSH-Alias %q, erwartet github-kamranbycloud", overview.SSHAlias)
	}
	if !strings.Contains(overview.AliasHint, "github-kamranbycloud") {
		t.Errorf("Hinweis nennt den Alias nicht: %q", overview.AliasHint)
	}
	// Der Aliasname ist kein Kontoname. Welches Konto hinter seinem Schlüssel
	// steht, wäre nur über einen weiteren Netzaufruf zu erfahren — der Hinweis
	// behauptet deshalb keines.
	if strings.Contains(overview.AliasHint, "kkl_kmde") || strings.Contains(overview.AliasHint, "KamranByCloud") {
		t.Errorf("Hinweis behauptet ein Konto hinter dem Alias: %q", overview.AliasHint)
	}
}

func TestOverviewMachtKeinenWeiterenNetzaufrufFuerDenAlias(t *testing.T) {
	runner := healthyRunner()
	runner.client().FetchOverview(context.Background())

	for _, call := range runner.calls {
		if strings.HasPrefix(call, "ssh ") {
			t.Errorf("Aufruf %q: der Alias wird erkannt, nicht aufgelöst", call)
		}
	}
}

func TestOverviewNimmtJeWorkflowDenJuengstenLauf(t *testing.T) {
	overview := healthyRunner().client().FetchOverview(context.Background())

	if len(overview.Workflows) != 2 {
		t.Fatalf("%d Workflows, erwartet 2: %+v", len(overview.Workflows), overview.Workflows)
	}
	if overview.Workflows[1].Name != "Tests" || overview.Workflows[1].RunID != 35002675449 {
		t.Errorf("zweiter Workflow %+v, erwartet Tests mit Lauf 35002675449", overview.Workflows[1])
	}
	// Ein einziger roter Lauf macht die CI rot.
	if overview.CI != "error" {
		t.Errorf("CI-Stand %q, erwartet error", overview.CI)
	}
}

func TestOverviewOhneTagErklaertDieLuecke(t *testing.T) {
	runner := healthyRunner()
	runner.answers = append([]fakeAnswer{
		{prefix: "git describe", err: ghError("fatal: No names found, cannot describe anything.")},
	}, runner.answers...)

	overview := runner.client().FetchOverview(context.Background())
	if overview.State != StateOK {
		t.Fatalf("Zustand %q, erwartet ok: ein fehlender Tag macht die Repo-Angabe nicht falsch", overview.State)
	}
	if overview.TagNote == "" {
		t.Error("kein Hinweis zum fehlenden Tag; jeder Zustand ohne Daten ist erklärt statt leer")
	}
}

// Je Zustand aus dem Intent ein Fall. Geprüft wird die Einordnung, nicht der
// rohe Fehlertext von gh: der wird gelesen und nicht durchgereicht.
func TestOverviewOrdnetJedenFehlerEin(t *testing.T) {
	cases := []struct {
		name   string
		stderr string
		want   State
	}{
		{"nicht angemeldet", "To get started with GitHub CLI, please run:  gh auth login", StateNotLoggedIn},
		{"Anmeldung ungültig", "HTTP 401: Bad credentials (https://api.github.com/user)", StateBadCredentials},
		{"kein Remote", "none of the git remotes configured for this repository point to a known GitHub host.", StateNoRemote},
		{"kein Zugriff 403", "HTTP 403: Resource not accessible by integration", StateNoAccess},
		{"kein Zugriff 404", "GraphQL: Could not resolve to a Repository with the name 'kascada/geheim'.", StateNoAccess},
		{"Rate-Limit", "HTTP 403: API rate limit exceeded for user ID 1.", StateRateLimited},
		{"kein Netz", "dial tcp: lookup api.github.com: no such host", StateNetwork},
		{"kein gh", `exec: "gh": executable file not found in $PATH`, StateNotInstalled},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			runner := &fakeRunner{answers: []fakeAnswer{{prefix: "gh repo view", err: ghError(testCase.stderr)}}}
			overview := runner.client().FetchOverview(context.Background())

			if overview.State != testCase.want {
				t.Fatalf("Zustand %q, erwartet %q", overview.State, testCase.want)
			}
			if overview.Message == "" {
				t.Error("Zustand ohne Erklärung; jeder Leerzustand ist erklärt statt leer")
			}
			if strings.Contains(overview.Message, testCase.stderr) {
				t.Errorf("Fehlertext von gh roh durchgereicht: %q", overview.Message)
			}
		})
	}
}

func TestOverviewLaeuftInDieFristStattZuHaengen(t *testing.T) {
	runner := &fakeRunner{answers: []fakeAnswer{
		{prefix: "gh repo view", err: ghError("signal: killed")},
	}}
	if state := runner.client().FetchOverview(context.Background()).State; state != StateTimeout {
		t.Errorf("Zustand %q, erwartet timeout", state)
	}
}

func TestSSHAliasOfErkenntNurEchteAliase(t *testing.T) {
	cases := map[string]string{
		"git@github-kamranbycloud:kascada/k-playbook.git": "github-kamranbycloud",
		"git@github.com:kascada/k-playbook.git":           "",
		"ssh://git@github.com/kascada/k-playbook.git":     "",
		"ssh://git@github-zweit:2222/kascada/k-playbook":  "github-zweit",
		"https://github.com/kascada/k-playbook.git":       "",
		"https://kkl_kmde@github.com/kascada/k-playbook":  "",
		"": "",
	}
	for remote, want := range cases {
		if got := SSHAliasOf(remote); got != want {
			t.Errorf("SSHAliasOf(%q) = %q, erwartet %q", remote, got, want)
		}
	}
}
