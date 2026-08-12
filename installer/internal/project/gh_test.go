package project

import (
	"os"
	"slices"
	"strings"
	"testing"
)

func TestParseGHStatus(t *testing.T) {
	cases := []struct {
		name       string
		content    string
		want       GHStatus
		configured bool
		wantErr    bool
	}{
		{
			name:       "ohne tools-Block gilt unknown",
			content:    "schema_version: 3\n\nproject:\n  repo_root: .\n",
			want:       GHUnknown,
			configured: false,
		},
		{
			name:       "gelesen wird tools.gh.status",
			content:    "tools:\n  gh:\n    status: enabled\n",
			want:       GHEnabled,
			configured: true,
		},
		{
			name: "ein Nachbarblock stört nicht",
			content: `tools:
  beispiel-tool:
    target: app
    report:
      status: enabled
  gh:
    status: disabled
`,
			want:       GHDisabled,
			configured: true,
		},
		{
			name: "status außerhalb von gh zählt nicht",
			content: `tools:
  beispiel-tool:
    report:
      status: enabled
`,
			want:       GHUnknown,
			configured: false,
		},
		{
			name:       "ein unbekannter Wert ist ein Fehler",
			content:    "tools:\n  gh:\n    status: vielleicht\n",
			configured: true,
			wantErr:    true,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			status, configured, err := parseGHStatus(testCase.content)
			if testCase.wantErr {
				if err == nil {
					t.Fatalf("kein Fehler für %q", testCase.content)
				}
				return
			}
			if err != nil {
				t.Fatalf("unerwarteter Fehler: %v", err)
			}
			if status != testCase.want {
				t.Errorf("status = %q, erwartet %q", status, testCase.want)
			}
			if configured != testCase.configured {
				t.Errorf("configured = %t, erwartet %t", configured, testCase.configured)
			}
		})
	}
}

func TestParseGHHosts(t *testing.T) {
	content := `github.com:
    user: zweitname
    oauth_token: gho_geheim
    git_protocol: https
    users:
        erstname:
            oauth_token: gho_auch_geheim
        zweitname:
            oauth_token: gho_ebenfalls_geheim
github.example.com:
    user: fremder
    users:
        fremder:
            oauth_token: gho_fremd
`

	active, accounts := parseGHHosts(content)
	if active != "zweitname" {
		t.Errorf("active = %q, erwartet zweitname", active)
	}
	// Der aktive Account gehört nach vorn, damit die Oberfläche ihn nicht
	// heraussuchen muss.
	want := []string{"zweitname", "erstname"}
	if !slices.Equal(accounts, want) {
		t.Errorf("accounts = %v, erwartet %v", accounts, want)
	}
	// Ein anderer Host darf sich nicht einmischen.
	if slices.Contains(accounts, "fremder") {
		t.Errorf("Account eines anderen Hosts übernommen: %v", accounts)
	}
}

// Ältere gh-Fassungen kennen keinen users-Block. Dort ist `user` der einzige
// Account und darf nicht verloren gehen.
func TestParseGHHostsOhneUsersBlock(t *testing.T) {
	active, accounts := parseGHHosts("github.com:\n    user: einziger\n    oauth_token: gho_geheim\n")
	if active != "einziger" {
		t.Errorf("active = %q, erwartet einziger", active)
	}
	if !slices.Equal(accounts, []string{"einziger"}) {
		t.Errorf("accounts = %v, erwartet [einziger]", accounts)
	}
}

func TestParseGHHostsOhneAnmeldung(t *testing.T) {
	active, accounts := parseGHHosts("")
	if active != "" || len(accounts) != 0 {
		t.Errorf("active = %q, accounts = %v, erwartet leer", active, accounts)
	}
}

func TestSetGHStatusLegtBlockAn(t *testing.T) {
	dir := writeConfig(t, `schema_version: 3

project:
  repo_root: .
  vcs: git

remediation:
  mode: task-first
`)

	if err := SetGHStatus(dir, GHEnabled); err != nil {
		t.Fatalf("SetGHStatus: %v", err)
	}

	updated := readConfig(t, dir)
	if !strings.Contains(updated, "tools:\n  gh:\n") {
		t.Errorf("tools.gh fehlt:\n%s", updated)
	}
	// Was vorher dastand, bleibt stehen.
	if !strings.Contains(updated, "mode: task-first") {
		t.Errorf("remediation verloren:\n%s", updated)
	}

	status, configured, err := parseGHStatus(updated)
	if err != nil {
		t.Fatalf("parseGHStatus: %v", err)
	}
	if status != GHEnabled || !configured {
		t.Errorf("status = %q, configured = %t", status, configured)
	}
}

// Der Nachbar im tools-Block gehört einem anderen Tool und darf beim Schreiben
// nicht verlorengehen.
func TestSetGHStatusLaesstNachbarblockStehen(t *testing.T) {
	dir := writeConfig(t, `schema_version: 3

tools:
  beispiel-tool:
    target: app
    languages:
      - python
    report:
      status: disabled
  gh:
    status: enabled
`)

	if err := SetGHStatus(dir, GHDisabled); err != nil {
		t.Fatalf("SetGHStatus: %v", err)
	}

	updated := readConfig(t, dir)
	for _, want := range []string{"beispiel-tool:", "target: app", "- python", "report:"} {
		if !strings.Contains(updated, want) {
			t.Errorf("%q verloren:\n%s", want, updated)
		}
	}

	status, _, err := parseGHStatus(updated)
	if err != nil {
		t.Fatalf("parseGHStatus: %v", err)
	}
	if status != GHDisabled {
		t.Errorf("status = %q, erwartet disabled", status)
	}
}

// Zweimal schreiben darf keinen zweiten Block hinterlassen.
func TestSetGHStatusIstWiederholbar(t *testing.T) {
	dir := writeConfig(t, "schema_version: 3\n\ntools:\n  gh:\n    status: unknown\n")

	for _, status := range []GHStatus{GHEnabled, GHDisabled, GHEnabled} {
		if err := SetGHStatus(dir, status); err != nil {
			t.Fatalf("SetGHStatus(%q): %v", status, err)
		}
	}

	updated := readConfig(t, dir)
	if count := strings.Count(updated, "  gh:\n"); count != 1 {
		t.Errorf("gh-Block %d-mal vorhanden:\n%s", count, updated)
	}
}

func TestSetGHStatusLehntUnbekanntesAb(t *testing.T) {
	dir := writeConfig(t, "schema_version: 3\n")

	if err := SetGHStatus(dir, GHStatus("vielleicht")); err == nil {
		t.Fatal("unbekannter Status wurde geschrieben")
	}
}

func TestDetectGHOhneAnmeldung(t *testing.T) {
	// Ein leeres Konfigurationsverzeichnis: gh mag installiert sein, angemeldet
	// ist hier niemand.
	t.Setenv("GH_CONFIG_DIR", t.TempDir())
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")

	state := DetectGH()
	if state.LoggedIn {
		t.Errorf("LoggedIn = true ohne hosts.yml")
	}
	if state.Ready {
		t.Errorf("Ready = true ohne Anmeldung")
	}
	if state.Host != GHHost {
		t.Errorf("Host = %q, erwartet %q", state.Host, GHHost)
	}
}

// Ein Token in der Umgebung sticht die Konfigurationsdatei; ohne diesen Fall
// meldete die Oberfläche „nicht angemeldet", während gh läuft.
func TestDetectGHMitTokenAusDerUmgebung(t *testing.T) {
	t.Setenv("GH_CONFIG_DIR", t.TempDir())
	t.Setenv("GH_TOKEN", "gho_geheim")

	state := DetectGH()
	if !state.LoggedIn {
		t.Errorf("LoggedIn = false trotz GH_TOKEN")
	}
	if !state.TokenFromEnv {
		t.Errorf("TokenFromEnv = false trotz GH_TOKEN")
	}
	if state.Account != "" {
		t.Errorf("Account = %q, erwartet leer: die Umgebung kennt keinen Namen", state.Account)
	}
}

func readConfig(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(ConfigPath(dir))
	if err != nil {
		t.Fatalf("Konfiguration lesen: %v", err)
	}
	return string(data)
}
