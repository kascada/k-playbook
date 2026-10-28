package githubaccess

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeTools legt gh und ssh als Skripte in ein Verzeichnis und setzt PATH
// darauf. gh tut nichts (für die Verfügbarkeit zählt nur, dass es im PATH
// liegt). ssh beantwortet `-G <host>` wie OpenSSH: ein bekannter Alias
// (github-example) löst nach github.com auf, github.com selbst ebenso, ein
// unbekannter Alias zu sich selbst, der Alias kaputt scheitert. Ohne gh im
// Skriptsatz (withGH false) fehlt gh im PATH.
func fakeTools(t *testing.T, withGH bool) string {
	t.Helper()
	dir := t.TempDir()
	ssh := `#!/bin/sh
if [ "$1" != "-G" ]; then echo "unerwarteter Aufruf: $*" >&2; exit 2; fi
case "$2" in
  github-example|github.com) echo "user git"; echo "hostname github.com"; echo "port 22" ;;
  kaputt) echo "/home/x/.ssh/config: line 3: Bad configuration option: xyz" >&2; exit 255 ;;
  *) echo "user git"; echo "hostname $2" ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "ssh"), []byte(ssh), 0o755); err != nil {
		t.Fatal(err)
	}
	if withGH {
		if err := os.WriteFile(filepath.Join(dir, "gh"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
	return dir
}

// ghHosts gibt gh eine Konfiguration mit den genannten Accounts.
func ghHosts(t *testing.T, accounts ...string) {
	t.Helper()
	dir := t.TempDir()
	var b strings.Builder
	b.WriteString("github.com:\n")
	if len(accounts) > 0 {
		b.WriteString("    user: " + accounts[0] + "\n    oauth_token: gho_geheim\n    users:\n")
		for _, account := range accounts {
			b.WriteString("        " + account + ":\n            oauth_token: gho_geheim_" + account + "\n")
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "hosts.yml"), []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GH_CONFIG_DIR", dir)
}

func judgeOne(access Access) Access {
	services := Judge(Services{{Name: "s", Repo: "a/b", Access: []Access{access}}})
	return services[0].Access[0]
}

func TestJudgeGH(t *testing.T) {
	fakeTools(t, true)
	ghHosts(t, "alice", "bob")

	if got := judgeOne(Access{Type: "gh", Login: "bob"}); got.State != StateAvailable || got.Reason != "" {
		t.Errorf("bekannter Login: %+v", got)
	}
	got := judgeOne(Access{Type: "gh", Login: "carol"})
	if got.State != StateUnavailable || !strings.Contains(got.Reason, "carol") || !strings.Contains(got.Reason, "alice, bob") || !strings.Contains(got.Reason, "gh auth login") {
		t.Errorf("unbekannter Login: %+v", got)
	}

	ghHosts(t)
	if got := judgeOne(Access{Type: "gh", Login: "alice"}); got.State != StateUnavailable || !strings.Contains(got.Reason, "kein Konto hinterlegt") {
		t.Errorf("ohne Anmeldung: %+v", got)
	}

	fakeTools(t, false)
	ghHosts(t, "alice")
	if got := judgeOne(Access{Type: "gh", Login: "alice"}); got.State != StateUnavailable || !strings.Contains(got.Reason, "nicht im PATH") {
		t.Errorf("gh fehlt: %+v", got)
	}
}

func TestJudgeTokenFile(t *testing.T) {
	fakeTools(t, true)
	dir := t.TempDir()
	path := filepath.Join(dir, "acme.token")
	writeMode := func(content string, mode os.FileMode) {
		t.Helper()
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, mode); err != nil {
			t.Fatal(err)
		}
	}

	if got := judgeOne(Access{Type: "token", File: path}); got.State != StateUnavailable || !strings.Contains(got.Reason, "fehlt") || !strings.Contains(got.Reason, path) {
		t.Errorf("fehlende Datei: %+v", got)
	}
	writeMode("ghp_geheimes_token_xyz\n", 0o644)
	got := judgeOne(Access{Type: "token", File: path})
	if got.State != StateUnavailable || !strings.Contains(got.Reason, "0644") || !strings.Contains(got.Reason, "chmod 600") {
		t.Errorf("falscher Modus: %+v", got)
	}
	writeMode("ghp_geheimes_token_xyz\nzweite Zeile\n", 0o600)
	if got := judgeOne(Access{Type: "token", File: path}); got.State != StateUnavailable || !strings.Contains(got.Reason, "mehr als eine Zeile") {
		t.Errorf("zwei Zeilen: %+v", got)
	}
	writeMode("\n", 0o600)
	if got := judgeOne(Access{Type: "token", File: path}); got.State != StateUnavailable || !strings.Contains(got.Reason, "leer") {
		t.Errorf("leer: %+v", got)
	}
	writeMode("ghp_geheimes_token_xyz\n", 0o600)
	if got := judgeOne(Access{Type: "token", File: path}); got.State != StateAvailable || got.Reason != "" {
		t.Errorf("gültig mit Zeilenumbruch: %+v", got)
	}
	writeMode("ghp_geheimes_token_xyz", 0o600)
	if got := judgeOne(Access{Type: "token", File: path}); got.State != StateAvailable {
		t.Errorf("gültig ohne Zeilenumbruch: %+v", got)
	}
	if got := judgeOne(Access{Type: "token", File: dir}); got.State != StateUnavailable || !strings.Contains(got.Reason, "keine gewöhnliche Datei") {
		t.Errorf("Verzeichnis: %+v", got)
	}

	// Der Inhalt der Datei verlässt die Beurteilung nicht — in keinem Zustand.
	for _, content := range []string{"ghp_geheimes_token_xyz", "ghp_geheimes_token_xyz\nzweite", ""} {
		writeMode(content, 0o644)
		if got := judgeOne(Access{Type: "token", File: path}); strings.Contains(got.Reason, "ghp_") || strings.Contains(got.Reason, "zweite") {
			t.Errorf("Inhalt in der Begründung: %+v", got)
		}
	}
}

func TestJudgeSSHKey(t *testing.T) {
	fakeTools(t, true)
	path := filepath.Join(t.TempDir(), "id_ed25519")

	if got := judgeOne(Access{Type: "ssh", Key: path}); got.State != StateUnavailable || !strings.HasPrefix(got.Reason, "Die Schlüsseldatei fehlt") {
		t.Errorf("fehlender Schlüssel: %+v", got)
	}
	if err := os.WriteFile(path, []byte("-----BEGIN OPENSSH PRIVATE KEY-----\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := judgeOne(Access{Type: "ssh", Key: path}); got.State != StateUnavailable || !strings.Contains(got.Reason, "0644") {
		t.Errorf("0644: %+v", got)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	if got := judgeOne(Access{Type: "ssh", Key: path}); got.State != StateAvailable {
		t.Errorf("0600: %+v", got)
	}
	if err := os.Chmod(path, 0o400); err != nil {
		t.Fatal(err)
	}
	if got := judgeOne(Access{Type: "ssh", Key: path}); got.State != StateAvailable {
		t.Errorf("0400: %+v", got)
	}
}

func TestJudgeSSHHost(t *testing.T) {
	fakeTools(t, true)

	if got := judgeOne(Access{Type: "ssh", Host: "github-example"}); got.State != StateAvailable {
		t.Errorf("bekannter Alias: %+v", got)
	}
	if got := judgeOne(Access{Type: "ssh", Host: "github.com"}); got.State != StateAvailable {
		t.Errorf("github.com: %+v", got)
	}
	got := judgeOne(Access{Type: "ssh", Host: "github-unbekannt"})
	if got.State != StateUnavailable || !strings.Contains(got.Reason, "hostname github-unbekannt") || !strings.Contains(got.Reason, "HostName github.com") {
		t.Errorf("unbekannter Alias: %+v", got)
	}
	got = judgeOne(Access{Type: "ssh", Host: "kaputt"})
	if got.State != StateUnavailable || !strings.Contains(got.Reason, "ssh -G kaputt ist gescheitert: /home/x/.ssh/config: line 3") {
		t.Errorf("defekte Konfiguration: %+v", got)
	}
	if got := judgeOne(Access{Type: "ssh", Host: "-oProxyCommand=x"}); got.State != StateUnavailable || !strings.Contains(got.Reason, "Bindestrich") {
		t.Errorf("Option statt Host: %+v", got)
	}

	t.Setenv("PATH", t.TempDir())
	if got := judgeOne(Access{Type: "ssh", Host: "github-example"}); got.State != StateUnavailable || !strings.Contains(got.Reason, "ssh liegt nicht im PATH") {
		t.Errorf("ssh fehlt: %+v", got)
	}
}

// public ist immer verfügbar; disabled wird nicht beurteilt und trägt keinen
// Grund — auch dann nicht, wenn der Zugang unbrauchbar wäre.
func TestJudgePublicUndDisabled(t *testing.T) {
	fakeTools(t, false)
	if got := judgeOne(Access{Type: "public"}); got.State != StateAvailable || got.Reason != "" {
		t.Errorf("public: %+v", got)
	}
	if got := judgeOne(Access{Type: "gh", Login: "alice", Disabled: true}); got.State != StateDisabled || got.Reason != "" {
		t.Errorf("disabled: %+v", got)
	}
}

// Usable und FirstReason: ein Dienst ist nutzbar mit einem verfügbaren
// Zugang; sonst nennt FirstReason den Grund des ersten Zugangs.
func TestUsableUndFirstReason(t *testing.T) {
	fakeTools(t, false)
	ghHosts(t)
	services := Judge(Services{
		{Name: "a", Repo: "x/a", Access: []Access{{Type: "gh", Login: "alice"}, {Type: "public"}}},
		{Name: "b", Repo: "x/b", Access: []Access{{Type: "gh", Login: "alice"}, {Type: "ssh", Key: "/nicht/da"}}},
		{Name: "c", Repo: "x/c", Access: []Access{{Type: "gh", Login: "alice", Disabled: true}}},
	})
	if !Usable(services[0]) || Usable(services[1]) || Usable(services[2]) {
		t.Errorf("Usable: %v %v %v", Usable(services[0]), Usable(services[1]), Usable(services[2]))
	}
	if reason := FirstReason(services[1]); !strings.Contains(reason, "gh liegt nicht im PATH") {
		t.Errorf("FirstReason b: %q", reason)
	}
	if reason := FirstReason(services[2]); reason != "alle Zugänge sind deaktiviert" {
		t.Errorf("FirstReason c: %q", reason)
	}
}

// Load trägt keine Zustände ein: Das JSON eines ungeprüften Zugangs hat weder
// state noch reason, das eines beurteilten beide.
func TestStateFehltOhneJudge(t *testing.T) {
	fakeTools(t, true)
	ghHosts(t)
	t.Setenv("HOME", t.TempDir())
	result := Load(filepath.Join("testdata", "valid.yaml"))
	if result.HasErrors() {
		t.Fatalf("valid.yaml: %+v", result.Problems)
	}
	encoded := string(mustJSON(t, result.Services))
	if strings.Contains(encoded, `"state"`) || strings.Contains(encoded, `"reason"`) {
		t.Errorf("state ohne Judge: %s", encoded)
	}
	encoded = string(mustJSON(t, Judge(result.Services)))
	if !strings.Contains(encoded, `"state":"available"`) || !strings.Contains(encoded, `"state":"unavailable"`) {
		t.Errorf("state nach Judge fehlt: %s", encoded)
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}
