package githubaccess

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Das erkennbare Token der Tests: Es darf in keiner Ausgabe, keinem Fehler
// und keiner Kommandozeile eines Kindprozesses stehen.
const testToken = "ghp_ERKENNBARES_TOKEN_1234567890abcdef"

// fakeGitHubAPI stellt /user und /repos/<owner>/<repo> nach, je nach Token
// im Authorization-Header. Nur Bearer-Tokens, sonst 401.
func fakeGitHubAPI(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		w.Header().Set("Content-Type", "application/json")
		writeJSON := func(status int, body string) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(body))
		}
		switch token {
		case "ghp_SLOW":
			time.Sleep(2 * time.Second)
			writeJSON(http.StatusOK, `{"login":"alice"}`)
			return
		case "ghp_BAD", "":
			// GitHub zitiert das Token nicht; dieser Server tut es absichtlich,
			// damit der Redaktionstest etwas zu redigieren hat.
			writeJSON(http.StatusUnauthorized, `{"message":"Bad credentials for `+token+`"}`)
			return
		}
		switch r.URL.Path {
		case "/user":
			switch token {
			case "ghp_SOON":
				w.Header().Set("github-authentication-token-expiration", "2026-11-06 10:00:00 UTC")
			case "ghp_EXPIRED":
				w.Header().Set("github-authentication-token-expiration", "2026-10-01 10:00:00 UTC")
			case testToken:
				w.Header().Set("github-authentication-token-expiration", "2027-03-01 10:00:00 UTC")
			}
			writeJSON(http.StatusOK, `{"login":"alice","id":1}`)
		case "/repos/acme/website":
			if token == "ghp_SSO" {
				writeJSON(http.StatusForbidden, `{"message":"Resource protected by organization SAML enforcement. You must grant your Personal Access token access to this organization."}`)
				return
			}
			writeJSON(http.StatusOK, `{"full_name":"acme/website","permissions":{"admin":false,"push":true,"pull":true}}`)
		case "/repos/acme/readonly":
			writeJSON(http.StatusOK, `{"full_name":"acme/readonly","permissions":{"admin":false,"push":false,"pull":true}}`)
		default:
			writeJSON(http.StatusNotFound, `{"message":"Not Found","documentation_url":"https://docs.github.com/rest"}`)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

// fakeTestTools legt gh, ssh und git als Skripte in den PATH und gibt die
// Datei zurück, in die jedes Skript seine Argumente und die Token-Variablen
// seiner Umgebung schreibt. gh kennt den Login alice (mit dem erkennbaren
// Token) und slow (schläft); ssh beantwortet -G wie in fakeTools und -T mit
// GitHubs Gruß: ein Deploy Key für github-example, alice für den Schlüssel am
// Standard-Host, Permission denied für github-denied, Schweigen für
// github-slow. git ls-remote scheitert für URLs mit „missing“, sonst Erfolg.
func fakeTestTools(t *testing.T) (calls string) {
	t.Helper()
	dir := t.TempDir()
	calls = filepath.Join(t.TempDir(), "aufrufe")
	record := `echo "$0 $* GH_TOKEN=${GH_TOKEN-unset} GITHUB_TOKEN=${GITHUB_TOKEN-unset} GIT_SSH_COMMAND=${GIT_SSH_COMMAND-unset}" >> "` + calls + `"
`
	scripts := map[string]string{
		"gh": record + `case "$6" in
  alice) echo "` + testToken + `" ;;
  slow) /bin/sleep 3; echo "ghp_SLOW" ;;
  soon) echo "ghp_SOON" ;;
  *) echo "no oauth token found for $6" >&2; exit 1 ;;
esac
`,
		"ssh": record + `if [ "$1" = "-G" ]; then
  case "$2" in github-example|github-denied|github-slow|github.com) echo "hostname github.com" ;; *) echo "hostname $2" ;; esac
  exit 0
fi
for last; do :; done
case "$last" in
  git@github-example) echo "Hi example/k-playbook! You've successfully authenticated, but GitHub does not provide shell access." >&2; exit 1 ;;
  git@github.com) echo "Hi alice! You've successfully authenticated, but GitHub does not provide shell access." >&2; exit 1 ;;
  git@github-denied) echo "git@github.com: Permission denied (publickey)." >&2; exit 255 ;;
  git@github-slow) /bin/sleep 3; exit 255 ;;
esac
exit 2
`,
		"git": record + `case "$*" in
  *missing*) echo "ERROR: Repository not found." >&2; echo "fatal: Could not read from remote repository." >&2; exit 128 ;;
  *slow*) /bin/sleep 3; exit 128 ;;
esac
echo "0123456789abcdef	HEAD"
`,
	}
	for name, body := range scripts {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
	return calls
}

// tokenFile legt eine Token-Datei mit Modus 0600 an.
func tokenFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "acme.token")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// fixedNow ist die Uhr der Tests: 2026-10-28.
func fixedNow() time.Time { return time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC) }

func newTester(t *testing.T) *Tester {
	t.Helper()
	server := fakeGitHubAPI(t)
	return &Tester{APIBase: server.URL, Timeout: 500 * time.Millisecond, Now: fixedNow}
}

func available(access Access) Access {
	access.State = StateAvailable
	return access
}

func TestTestTokenGegenDieAPI(t *testing.T) {
	fakeTestTools(t)
	tester := newTester(t)
	ctx := context.Background()

	// 200/200 mit push: ok, Identität, lesen ja, schreiben ja, Ablauf weit.
	got := tester.TestAccess(ctx, "acme/website", available(Access{Type: "token", File: tokenFile(t, testToken+"\n")}))
	if got.Result != ResultOK || got.Identity != "alice" || got.Read != AnswerYes || got.Write != AnswerYes || got.Expires != "2027-03-01" || got.Warning != "" || got.Message != "" {
		t.Errorf("ok: %+v", got)
	}
	// Ohne push: schreiben nein, trotzdem ok.
	if got := tester.TestAccess(ctx, "acme/readonly", available(Access{Type: "token", File: tokenFile(t, testToken)})); got.Result != ResultOK || got.Write != AnswerNo {
		t.Errorf("readonly: %+v", got)
	}
	// 404 am Repository: lesen nein, failed mit GitHubs Text und der Einordnung.
	got = tester.TestAccess(ctx, "acme/missing", available(Access{Type: "token", File: tokenFile(t, testToken)}))
	if got.Result != ResultFailed || got.Read != AnswerNo || got.Identity != "alice" || !strings.Contains(got.Message, "HTTP 404 bei GET /repos/acme/missing: Not Found") || !strings.Contains(got.Message, "Freigabe der Organisation") {
		t.Errorf("404: %+v", got)
	}
	// 401: das Token ist ungültig; nichts erreicht.
	got = tester.TestAccess(ctx, "acme/website", available(Access{Type: "token", File: tokenFile(t, "ghp_BAD")}))
	if got.Result != ResultFailed || got.Identity != "" || !strings.Contains(got.Message, "HTTP 401 bei GET /user") || !strings.Contains(got.Message, "ungültig oder zurückgezogen") {
		t.Errorf("401: %+v", got)
	}
	// 403 mit SSO-Text am Repository.
	got = tester.TestAccess(ctx, "acme/website", available(Access{Type: "token", File: tokenFile(t, "ghp_SSO")}))
	if got.Result != ResultFailed || !strings.Contains(got.Message, "HTTP 403") || !strings.Contains(got.Message, "SAML/SSO") {
		t.Errorf("403: %+v", got)
	}
	// Ablauf in 9 Tagen: ok mit Warnung; in der Vergangenheit: failed.
	got = tester.TestAccess(ctx, "acme/website", available(Access{Type: "token", File: tokenFile(t, "ghp_SOON")}))
	if got.Result != ResultOK || got.Expires != "2026-11-06" || got.Warning != "läuft in 9 Tagen ab" {
		t.Errorf("bald: %+v", got)
	}
	got = tester.TestAccess(ctx, "acme/website", available(Access{Type: "token", File: tokenFile(t, "ghp_EXPIRED")}))
	if got.Result != ResultFailed || got.Expires != "2026-10-01" || !strings.Contains(got.Message, "am 2026-10-01 abgelaufen") || got.Read != "" {
		t.Errorf("abgelaufen: %+v", got)
	}
	// Frist: failed mit dem Wort Zeitüberschreitung.
	got = tester.TestAccess(ctx, "acme/website", available(Access{Type: "token", File: tokenFile(t, "ghp_SLOW")}))
	if got.Result != ResultFailed || !strings.Contains(got.Message, "Zeitüberschreitung") || !strings.Contains(got.Message, "GET /user") {
		t.Errorf("Frist: %+v", got)
	}
}

func TestTestGHHoltDasTokenAusGh(t *testing.T) {
	calls := fakeTestTools(t)
	tester := newTester(t)
	ctx := context.Background()

	got := tester.TestAccess(ctx, "acme/website", available(Access{Type: "gh", Login: "alice"}))
	if got.Result != ResultOK || got.Identity != "alice" || got.Read != AnswerYes || got.Write != AnswerYes || got.Login != "alice" {
		t.Errorf("alice: %+v", got)
	}
	got = tester.TestAccess(ctx, "acme/website", available(Access{Type: "gh", Login: "nobody"}))
	if got.Result != ResultFailed || !strings.Contains(got.Message, "gh auth token für nobody: no oauth token found for nobody") {
		t.Errorf("nobody: %+v", got)
	}
	got = tester.TestAccess(ctx, "acme/website", available(Access{Type: "gh", Login: "slow"}))
	if got.Result != ResultFailed || !strings.Contains(got.Message, "Zeitüberschreitung") {
		t.Errorf("slow: %+v", got)
	}

	// gh wurde mit --hostname github.com --user <login> gerufen, ohne
	// Token-Variablen in seiner Umgebung — sonst gäbe es das Token der
	// Umgebung zurück statt das des Logins.
	log, err := os.ReadFile(calls)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(log), "auth token --hostname github.com --user alice GH_TOKEN=unset GITHUB_TOKEN=unset") {
		t.Errorf("Aufrufe:\n%s", log)
	}
}

func TestTestSSHUndPublic(t *testing.T) {
	calls := fakeTestTools(t)
	tester := newTester(t)
	ctx := context.Background()
	key := filepath.Join(t.TempDir(), "id mit leerzeichen")
	if err := os.WriteFile(key, []byte("key"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Alias: Deploy Key, Identität <owner>/<repo>, lesen ja, schreiben unknown.
	got := tester.TestAccess(ctx, "example/k-playbook", available(Access{Type: "ssh", Host: "github-example"}))
	if got.Result != ResultOK || got.Identity != "example/k-playbook" || got.Read != AnswerYes || got.Write != AnswerUnknown || got.Expires != ExpiresNone || got.Host != "github-example" {
		t.Errorf("Alias: %+v", got)
	}
	// Schlüssel: -i und IdentitiesOnly, Ziel github.com, Nutzer alice.
	got = tester.TestAccess(ctx, "acme/website", available(Access{Type: "ssh", Key: key}))
	if got.Result != ResultOK || got.Identity != "alice" || got.Read != AnswerYes {
		t.Errorf("Schlüssel: %+v", got)
	}
	// Permission denied: failed mit SSHs stderr.
	got = tester.TestAccess(ctx, "acme/website", available(Access{Type: "ssh", Host: "github-denied"}))
	if got.Result != ResultFailed || got.Identity != "" || got.Message != "ssh -T git@github-denied: git@github.com: Permission denied (publickey)." {
		t.Errorf("denied: %+v", got)
	}
	// Gruß ja, ls-remote nein: lesen nein, failed.
	got = tester.TestAccess(ctx, "acme/missing", available(Access{Type: "ssh", Host: "github-example"}))
	if got.Result != ResultFailed || got.Identity != "example/k-playbook" || got.Read != AnswerNo || !strings.Contains(got.Message, "git ls-remote git@github-example:acme/missing.git: ERROR: Repository not found. / fatal:") {
		t.Errorf("ls-remote: %+v", got)
	}
	// Frist beim Gruß.
	got = tester.TestAccess(ctx, "acme/website", available(Access{Type: "ssh", Host: "github-slow"}))
	if got.Result != ResultFailed || !strings.Contains(got.Message, "Zeitüberschreitung: ssh -T git@github-slow") {
		t.Errorf("slow: %+v", got)
	}
	// public: anonym über HTTPS, schreiben nein.
	got = tester.TestAccess(ctx, "acme/website", available(Access{Type: "public"}))
	if got.Result != ResultOK || got.Identity != IdentityAnonymous || got.Read != AnswerYes || got.Write != AnswerNo || got.Expires != ExpiresNone {
		t.Errorf("public: %+v", got)
	}
	got = tester.TestAccess(ctx, "acme/missing", available(Access{Type: "public"}))
	if got.Result != ResultFailed || got.Read != AnswerNo || !strings.Contains(got.Message, "git ls-remote https://github.com/acme/missing.git: ERROR: Repository not found.") {
		t.Errorf("public 404: %+v", got)
	}

	log, err := os.ReadFile(calls)
	if err != nil {
		t.Fatal(err)
	}
	text := string(log)
	// Der Schlüssel steht an ssh (-i, IdentitiesOnly) und in GIT_SSH_COMMAND,
	// dort in Anführungszeichen; BatchMode überall; public ohne Helfer.
	for _, want := range []string{
		"ssh -T -o BatchMode=yes -i " + key + " -o IdentitiesOnly=yes git@github.com",
		"ls-remote git@github.com:acme/website.git HEAD GH_TOKEN=unset GITHUB_TOKEN=unset GIT_SSH_COMMAND=ssh -o BatchMode=yes -i '" + key + "' -o IdentitiesOnly=yes",
		"ls-remote git@github-example:example/k-playbook.git HEAD GH_TOKEN=unset GITHUB_TOKEN=unset GIT_SSH_COMMAND=ssh -o BatchMode=yes",
		"git -c credential.helper= ls-remote https://github.com/acme/website.git HEAD",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("Aufruf fehlt: %s\n%s", want, text)
		}
	}
}

// skipped: disabled und unavailable werden nicht kontaktiert und tragen
// ihren Grund in message; ein unbeurteilter Zugang ebenso.
func TestTestUeberspringtDeaktivierteUndNichtVerfuegbare(t *testing.T) {
	calls := fakeTestTools(t)
	tester := newTester(t)
	ctx := context.Background()

	got := tester.TestAccess(ctx, "acme/website", Access{Type: "gh", Login: "alice", Disabled: true, State: StateDisabled})
	if got.Result != ResultSkipped || got.Message != "deaktiviert" || !got.Disabled || got.Identity != "" {
		t.Errorf("disabled: %+v", got)
	}
	got = tester.TestAccess(ctx, "acme/website", Access{Type: "ssh", Key: "/nicht/da", State: StateUnavailable, Reason: "Die Schlüsseldatei fehlt."})
	if got.Result != ResultSkipped || got.Message != "nicht verfügbar — Die Schlüsseldatei fehlt." {
		t.Errorf("unavailable: %+v", got)
	}
	if got := tester.TestAccess(ctx, "acme/website", Access{Type: "public"}); got.Result != ResultSkipped {
		t.Errorf("unbeurteilt: %+v", got)
	}
	if _, err := os.Stat(calls); !os.IsNotExist(err) {
		log, _ := os.ReadFile(calls)
		t.Errorf("ein übersprungener Zugang hat einen Prozess gestartet:\n%s", log)
	}
}

// Test über alle Dienste: Judge läuft vorher, der Report trägt Datei,
// Dienste in Reihenfolge und ok; das JSON hat die Form des Vertrags.
func TestTestReportUndJSON(t *testing.T) {
	fakeTestTools(t)
	ghHosts(t, "alice")
	tester := newTester(t)
	token := tokenFile(t, testToken)
	services := Services{
		{Name: "website", Repo: "acme/website", Access: []Access{{Type: "gh", Login: "alice"}, {Type: "token", File: token, Disabled: true}}},
		{Name: "playbook", Repo: "example/k-playbook", Access: []Access{{Type: "ssh", Host: "github-example"}, {Type: "ssh", Key: "/nicht/da"}}},
	}
	report := tester.Test(context.Background(), "/pfad/github.yaml", services)
	if !report.OK || report.File != "/pfad/github.yaml" || len(report.Services) != 2 {
		t.Fatalf("Report: %+v", report)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	for _, want := range []string{
		`"file":"/pfad/github.yaml"`,
		`"services":{"website":{"repo":"acme/website","access":[{"type":"gh","login":"alice","result":"ok","identity":"alice","read":"yes","write":"yes","expires":"2027-03-01"},{"type":"token","file":"` + token + `","disabled":true,"result":"skipped","message":"deaktiviert"}]},"playbook":`,
		`{"type":"ssh","host":"github-example","result":"ok","identity":"example/k-playbook","read":"yes","write":"unknown","expires":"none"}`,
		`{"type":"ssh","key":"/nicht/da","result":"skipped","message":"nicht verfügbar — Die Schlüsseldatei fehlt`,
		`"ok":true}`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("JSON ohne %s:\n%s", want, text)
		}
	}
	if strings.Contains(text, `"state"`) || strings.Contains(text, `"reason"`) {
		t.Errorf("state/reason im Testreport: %s", text)
	}

	// Ein failed macht ok falsch; skipped nicht.
	services = Services{{Name: "x", Repo: "acme/missing", Access: []Access{{Type: "public"}}}}
	if report := tester.Test(context.Background(), "/pfad", services); report.OK {
		t.Errorf("ok trotz failed: %+v", report)
	}
}

// Der Redaktionstest: ein erkennbares Token aus gh und aus der Datei, GitHub
// antwortet mit einem Text, der es zitiert — in keinem Ergebnis, keinem JSON
// und keiner Kommandozeile eines Kindprozesses steht es, nicht einmal ein
// Präfix.
func TestTestVerraetDasTokenNicht(t *testing.T) {
	calls := fakeTestTools(t)
	ghHosts(t, "alice")
	server := fakeGitHubAPI(t)
	// Jede Antwort 401 mit dem Token im Text: der Weg, auf dem es durchkäme.
	echo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Bad credentials: ` + token + ` and prefix ` + token[:12] + `"}`))
	}))
	defer echo.Close()

	for _, base := range []string{server.URL, echo.URL} {
		tester := &Tester{APIBase: base, Timeout: 500 * time.Millisecond, Now: fixedNow}
		services := Services{{Name: "s", Repo: "acme/missing", Access: []Access{
			{Type: "gh", Login: "alice"},
			{Type: "token", File: tokenFile(t, testToken)},
		}}}
		report := tester.Test(context.Background(), "/pfad", services)
		encoded, err := json.Marshal(report)
		if err != nil {
			t.Fatal(err)
		}
		for _, needle := range []string{testToken, testToken[:12], "ERKENNBARES"} {
			if strings.Contains(string(encoded), needle) {
				t.Errorf("Token im Report (%s): %s", base, encoded)
			}
			for _, access := range report.Services[0].Access {
				if strings.Contains(access.Message, needle) || strings.Contains(access.Warning, needle) || strings.Contains(access.Identity, needle) {
					t.Errorf("Token im Ergebnis: %+v", access)
				}
			}
		}
		if base == echo.URL && !strings.Contains(report.Services[0].Access[1].Message, "[token]") {
			t.Errorf("zitiertes Token nicht redigiert: %+v", report.Services[0].Access[1])
		}
	}
	log, err := os.ReadFile(calls)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(log), "ghp_") {
		t.Errorf("Token in einer Kommandozeile oder Umgebung eines Kindprozesses:\n%s", log)
	}
}

func TestRedact(t *testing.T) {
	if got := redact("Bad credentials for ghp_ERKENNBAR_1234", "ghp_ERKENNBAR_1234"); got != "Bad credentials for [token]" {
		t.Errorf("ganz: %q", got)
	}
	if got := redact("ghp_ERKENNBAR_12 …", "ghp_ERKENNBAR_1234"); got != "[token] …" {
		t.Errorf("Präfix: %q", got)
	}
	if got := redact("ghp_ERK", "ghp_ERKENNBAR_1234"); got != "ghp_ERK" {
		t.Errorf("zu kurz für ein Präfix: %q", got)
	}
	if got := redact("unverändert", ""); got != "unverändert" {
		t.Errorf("leer: %q", got)
	}
}
