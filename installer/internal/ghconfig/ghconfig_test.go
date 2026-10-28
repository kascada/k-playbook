package ghconfig

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestParseHosts(t *testing.T) {
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

	active, accounts := ParseHosts(content)
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
func TestParseHostsOhneUsersBlock(t *testing.T) {
	active, accounts := ParseHosts("github.com:\n    user: einziger\n    oauth_token: gho_geheim\n")
	if active != "einziger" {
		t.Errorf("active = %q, erwartet einziger", active)
	}
	if !slices.Equal(accounts, []string{"einziger"}) {
		t.Errorf("accounts = %v, erwartet [einziger]", accounts)
	}
}

func TestParseHostsOhneAnmeldung(t *testing.T) {
	active, accounts := ParseHosts("")
	if active != "" || len(accounts) != 0 {
		t.Errorf("active = %q, accounts = %v, erwartet leer", active, accounts)
	}
}

// ReadHosts liest hosts.yml aus dem Verzeichnis; fehlt sie, ist nichts
// hinterlegt und kein Fehler.
func TestReadHostsUndConfigDir(t *testing.T) {
	dir := t.TempDir()
	if active, accounts := ReadHosts(dir); active != "" || len(accounts) != 0 {
		t.Errorf("ohne hosts.yml: %q %v", active, accounts)
	}
	if err := os.WriteFile(filepath.Join(dir, "hosts.yml"), []byte("github.com:\n    user: alice\n    users:\n        alice:\n            oauth_token: gho_x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if active, accounts := ReadHosts(dir); active != "alice" || !slices.Equal(accounts, []string{"alice"}) {
		t.Errorf("mit hosts.yml: %q %v", active, accounts)
	}

	// GH_CONFIG_DIR sticht XDG_CONFIG_HOME, das sticht HOME.
	t.Setenv("GH_CONFIG_DIR", dir)
	t.Setenv("XDG_CONFIG_HOME", "/xdg")
	if got := ConfigDir(); got != dir {
		t.Errorf("ConfigDir = %q, erwartet %q", got, dir)
	}
	if !slices.Equal(Accounts(), []string{"alice"}) {
		t.Errorf("Accounts = %v", Accounts())
	}
	t.Setenv("GH_CONFIG_DIR", "")
	if got := ConfigDir(); got != filepath.Join("/xdg", "gh") {
		t.Errorf("ConfigDir = %q, erwartet /xdg/gh", got)
	}
	t.Setenv("XDG_CONFIG_HOME", "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	if got := ConfigDir(); got != filepath.Join(home, ".config", "gh") {
		t.Errorf("ConfigDir = %q, erwartet %s/.config/gh", got, home)
	}
}

func TestTokenFromEnv(t *testing.T) {
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	if TokenFromEnv() {
		t.Error("ohne Variablen true")
	}
	t.Setenv("GITHUB_TOKEN", "ghp_x")
	if !TokenFromEnv() {
		t.Error("mit GITHUB_TOKEN false")
	}
}
