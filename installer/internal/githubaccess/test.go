package githubaccess

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/kascada/k-playbook/installer/internal/ghconfig"
)

// Der Test gegen GitHub (docs/github-access.md, Abschnitt Testing an access):
// Er fragt GitHub, ob ein Zugang wirklich funktioniert — Identität, lesen,
// schreiben, Ablauf. Er ist das Einzige in diesem Paket, das das Netz nutzt,
// und läuft nur auf ausdrücklichen Aufruf: der Befehl `k-playbook github
// test`, das MCP-Werkzeug k_playbook_github_test, der Knopf der Oberfläche.
// Weder das Briefing noch die Umgebungsprüfung noch check rufen ihn.
//
// Kein Token verlässt den Prozess. Für gh und token steht das Token nur im
// Authorization-Header des eigenen HTTP-Clients; der einzige Kindprozess, der
// mit einem Token zu tun hat, ist `gh auth token -u <login>` — er liefert
// eines und bekommt keines. ssh, git und public bekommen kein Token. Keine
// Kommandozeile, keine Meldung, kein Fehler trägt es; jede Meldung läuft
// zusätzlich durch redact, damit auch ein Fehlertext, der es wider Erwarten
// zitiert, es nicht verrät. Kindprozesse laufen ohne GH_TOKEN und
// GITHUB_TOKEN: gh gäbe sonst das Token der Umgebung zurück statt das des
// Logins.
//
// Der Test liest; er pusht nie, legt nichts an und ändert die Datei nicht.

// Outcome ist das Ergebnis eines getesteten Zugangs.
type Outcome string

const (
	// ResultOK: GitHub hat das Credential angenommen, und das Repository ist
	// damit erreichbar.
	ResultOK Outcome = "ok"
	// ResultFailed: GitHub hat das Credential abgewiesen, oder das Repository
	// ist damit nicht erreichbar; Message trägt GitHubs Antwort.
	ResultFailed Outcome = "failed"
	// ResultSkipped: nicht getestet — disabled, unavailable, oder ein
	// früherer Schritt desselben Zugangs ist schon gescheitert.
	ResultSkipped Outcome = "skipped"
)

// Die Werte von Read und Write.
const (
	AnswerYes     = "yes"
	AnswerNo      = "no"
	AnswerUnknown = "unknown"
	// ExpiresNone: kein Ablaufdatum — ssh, public, ein Token ohne den Header.
	ExpiresNone = "none"
	// IdentityAnonymous ist die Identität eines public-Zugangs.
	IdentityAnonymous = "anonymous"
)

// DefaultTimeout ist die Frist je Anfrage und je Kindprozess: zehn Sekunden,
// wie der Vertrag sie nennt.
const DefaultTimeout = 10 * time.Second

// DefaultAPIBase ist die GitHub-API; Tests ersetzen sie durch einen
// httptest.Server.
const DefaultAPIBase = "https://api.github.com"

// expiryWarningDays: Ein Ablauf innerhalb so vieler Tage ist eine Warnung am
// Ergebnis; ein Ablauf in der Vergangenheit ist failed.
const expiryWarningDays = 14

// AccessResult ist das Ergebnis eines Zugangs: was die Datei über ihn sagt,
// dazu result und — soweit erreicht — identity, read, write, expires, warning
// und message. State und Reason der Verfügbarkeit stehen nicht hier: ein
// übersprungener Zugang trägt seinen Grund in message.
type AccessResult struct {
	Type     string  `json:"type"`
	Login    string  `json:"login,omitempty"`
	File     string  `json:"file,omitempty"`
	Key      string  `json:"key,omitempty"`
	Host     string  `json:"host,omitempty"`
	Disabled bool    `json:"disabled,omitempty"`
	Result   Outcome `json:"result"`
	Identity string  `json:"identity,omitempty"`
	Read     string  `json:"read,omitempty"`
	Write    string  `json:"write,omitempty"`
	Expires  string  `json:"expires,omitempty"`
	Warning  string  `json:"warning,omitempty"`
	Message  string  `json:"message,omitempty"`
}

// ServiceResult ist das Ergebnis eines Dienstes: Repository und je Zugang ein
// Ergebnis in Vorrangreihenfolge.
type ServiceResult struct {
	Name   string         `json:"-"`
	Repo   string         `json:"repo"`
	Access []AccessResult `json:"access"`
}

// ServiceResults sind die getesteten Dienste in Dateireihenfolge; im JSON ein
// Objekt in dieser Reihenfolge, wie Services.
type ServiceResults []ServiceResult

// MarshalJSON schreibt die Dienste als Objekt in Dateireihenfolge.
func (s ServiceResults) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, service := range s {
		if i > 0 {
			b.WriteByte(',')
		}
		name, err := json.Marshal(service.Name)
		if err != nil {
			return nil, err
		}
		body, err := json.Marshal(service)
		if err != nil {
			return nil, err
		}
		b.Write(name)
		b.WriteByte(':')
		b.Write(body)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// UnmarshalJSON liest das Objekt in seiner Reihenfolge zurück — für die
// Leser des Reports (Oberfläche, Tests), die ihn aus dem JSON holen.
func (s *ServiceResults) UnmarshalJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	if token == nil {
		*s = nil
		return nil
	}
	if delim, ok := token.(json.Delim); !ok || delim != '{' {
		return errors.New("services ist kein Objekt")
	}
	result := ServiceResults{}
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return err
		}
		name, _ := key.(string)
		var service ServiceResult
		if err := decoder.Decode(&service); err != nil {
			return err
		}
		service.Name = name
		result = append(result, service)
	}
	*s = result
	return nil
}

// Report ist das Ergebnis von Test: die Datei, die getesteten Dienste und
// OK, wahr, wenn kein getesteter Zugang failed ist — übersprungene zählen
// nicht.
type Report struct {
	File     string         `json:"file"`
	Services ServiceResults `json:"services"`
	OK       bool           `json:"ok"`
}

// Tester trägt, was der Test von außen braucht: die API-Adresse, die Frist
// je Aufruf und den HTTP-Client. Die Nullwerte sind die Standardwerte; Tests
// setzen APIBase auf einen httptest.Server und legen gh, ssh und git als
// Skripte in den PATH.
type Tester struct {
	APIBase string
	Timeout time.Duration
	Client  *http.Client
	// Now ist die Uhr für den Ablauf; nil heißt time.Now.
	Now func() time.Time
}

// NewTester ist der Standardfall: die echte API, zehn Sekunden je Aufruf.
func NewTester() *Tester {
	return &Tester{}
}

func (t *Tester) apiBase() string {
	if t.APIBase != "" {
		return strings.TrimRight(t.APIBase, "/")
	}
	return DefaultAPIBase
}

func (t *Tester) timeout() time.Duration {
	if t.Timeout > 0 {
		return t.Timeout
	}
	return DefaultTimeout
}

func (t *Tester) client() *http.Client {
	if t.Client != nil {
		return t.Client
	}
	return &http.Client{Timeout: t.timeout()}
}

func (t *Tester) now() time.Time {
	if t.Now != nil {
		return t.Now()
	}
	return time.Now()
}

// Test prüft jeden Zugang jedes Dienstes gegen GitHub. Die Verfügbarkeit
// wird vorher beurteilt (Judge): disabled und unavailable werden skipped
// mit ihrem Grund und nicht kontaktiert. file ist nur der Pfad für den
// Report; gelesen ist die Datei schon.
func (t *Tester) Test(ctx context.Context, file string, services Services) Report {
	Judge(services)
	report := Report{File: file, Services: ServiceResults{}, OK: true}
	for _, service := range services {
		result := ServiceResult{Name: service.Name, Repo: service.Repo, Access: []AccessResult{}}
		for _, access := range service.Access {
			outcome := t.TestAccess(ctx, service.Repo, access)
			if outcome.Result == ResultFailed {
				report.OK = false
			}
			result.Access = append(result.Access, outcome)
		}
		report.Services = append(report.Services, result)
	}
	return report
}

// TestAccess prüft einen Zugang gegen das Repository <owner>/<repo>. Der
// Zugang muss beurteilt sein (State gesetzt); ein nicht verfügbarer oder
// deaktivierter wird übersprungen.
func (t *Tester) TestAccess(ctx context.Context, repo string, access Access) AccessResult {
	result := AccessResult{Type: access.Type, Login: access.Login, File: access.File, Key: access.Key, Host: access.Host, Disabled: access.Disabled}
	switch access.State {
	case StateDisabled:
		result.Result, result.Message = ResultSkipped, "deaktiviert"
		return result
	case StateUnavailable:
		result.Result, result.Message = ResultSkipped, "nicht verfügbar — "+access.Reason
		return result
	case StateAvailable:
	default:
		result.Result, result.Message = ResultSkipped, "Verfügbarkeit nicht beurteilt"
		return result
	}
	switch access.Type {
	case "gh":
		t.testGH(ctx, repo, access.Login, &result)
	case "token":
		t.testTokenFile(ctx, repo, access.File, &result)
	case "ssh":
		t.testSSH(ctx, repo, access, &result)
	case "public":
		t.testPublic(ctx, repo, &result)
	default:
		result.Result, result.Message = ResultSkipped, "unbekannter Typ "+access.Type
	}
	return result
}

// testGH holt das Token des Logins aus gh — der einzige tokenbezogene
// Kindprozess: er liefert das Token auf stdout und bekommt keines — und
// prüft es wie ein Token aus einer Datei.
func (t *Tester) testGH(ctx context.Context, repo string, login string, result *AccessResult) {
	stdout, stderr, err := t.run(ctx, nil, "gh", "auth", "token", "--hostname", ghconfig.Host, "--user", login)
	token := strings.TrimSpace(stdout)
	if err != nil || token == "" {
		result.Result = ResultFailed
		if err == nil {
			err = errors.New("gh hat kein Token ausgegeben")
		}
		result.Message = redact(sshFailure("gh auth token für "+login, stderr, err, t.timeout()), token)
		return
	}
	t.testToken(ctx, repo, token, result)
}

// testTokenFile liest das Token aus der Datei — den Modus und die eine
// Zeile hat die Verfügbarkeit schon geprüft — und prüft es gegen die API.
func (t *Tester) testTokenFile(ctx context.Context, repo string, path string, result *AccessResult) {
	data, err := os.ReadFile(path)
	if err != nil {
		result.Result = ResultFailed
		result.Message = "Die Token-Datei ist nicht lesbar: " + describeReadError(err) + "."
		return
	}
	t.testToken(ctx, repo, strings.TrimSpace(string(data)), result)
}

// apiAnswer ist, was eine Anfrage an die API ergeben hat.
type apiAnswer struct {
	status  int
	header  http.Header
	body    map[string]json.RawMessage
	message string // GitHubs eigener Text aus dem Rumpf, falls vorhanden
	err     error  // Netz oder Frist; status ist dann 0
}

// testToken: GET /user für die Identität und den Ablauf, dann
// GET /repos/<owner>/<repo> für lesen und schreiben. Das Token steht allein
// im Header der beiden Anfragen.
func (t *Tester) testToken(ctx context.Context, repo string, token string, result *AccessResult) {
	defer func() {
		result.Message = redact(result.Message, token)
		result.Warning = redact(result.Warning, token)
	}()

	user := t.get(ctx, "/user", token)
	if user.err != nil {
		result.Result, result.Message = ResultFailed, requestFailure("GET /user", user.err, t.timeout())
		return
	}
	if user.status != http.StatusOK {
		result.Result, result.Message = ResultFailed, apiFailure("GET /user", user)
		return
	}
	var login string
	_ = json.Unmarshal(user.body["login"], &login)
	result.Identity = login
	result.Expires = ExpiresNone

	// Der Ablauf kommt mit der Antwort auf /user; ein Token ohne den Header
	// (Classic ohne Ablauf) hat keinen.
	if raw := strings.TrimSpace(user.header.Get("github-authentication-token-expiration")); raw != "" {
		expires, ok := parseExpiry(raw)
		if !ok {
			result.Expires = raw
		} else {
			result.Expires = expires.Format("2006-01-02")
			days := calendarDays(t.now(), expires)
			switch {
			case expires.Before(t.now()):
				result.Result, result.Message = ResultFailed, "Das Token ist am "+result.Expires+" abgelaufen — ein neues Token ausstellen und hinterlegen."
				return
			case days < expiryWarningDays:
				result.Warning = expiryWarning(days)
			}
		}
	}

	repoAnswer := t.get(ctx, "/repos/"+repo, token)
	if repoAnswer.err != nil {
		result.Result, result.Message = ResultFailed, requestFailure("GET /repos/"+repo, repoAnswer.err, t.timeout())
		return
	}
	switch repoAnswer.status {
	case http.StatusOK:
		result.Read = AnswerYes
		result.Write = AnswerNo
		var permissions struct {
			Push bool `json:"push"`
		}
		if err := json.Unmarshal(repoAnswer.body["permissions"], &permissions); err == nil && permissions.Push {
			result.Write = AnswerYes
		}
		result.Result = ResultOK
	default:
		result.Read = AnswerNo
		result.Result, result.Message = ResultFailed, apiFailure("GET /repos/"+repo, repoAnswer)
	}
}

// get ruft die API mit dem Token im Header auf und liest die Antwort.
func (t *Tester) get(ctx context.Context, path string, token string) apiAnswer {
	callCtx, cancel := context.WithTimeout(ctx, t.timeout())
	defer cancel()
	request, err := http.NewRequestWithContext(callCtx, http.MethodGet, t.apiBase()+path, nil)
	if err != nil {
		return apiAnswer{err: err}
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	request.Header.Set("User-Agent", "k-playbook")
	response, err := t.client().Do(request)
	if err != nil {
		if callCtx.Err() != nil {
			err = callCtx.Err()
		}
		return apiAnswer{err: err}
	}
	defer response.Body.Close()
	answer := apiAnswer{status: response.StatusCode, header: response.Header, body: map[string]json.RawMessage{}}
	data, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	_ = json.Unmarshal(data, &answer.body)
	_ = json.Unmarshal(answer.body["message"], &answer.message)
	return answer
}

// requestFailure beschreibt eine Anfrage, die keine Antwort bekam: die Frist
// mit dem Wort Zeitüberschreitung, sonst der Fehler des Clients.
func requestFailure(what string, err error, timeout time.Duration) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "Zeitüberschreitung: " + what + " hat nicht innerhalb von " + timeout.String() + " geantwortet."
	}
	if errors.Is(err, context.Canceled) {
		return what + " wurde abgebrochen."
	}
	return what + " ist gescheitert: " + err.Error()
}

// apiFailure beschreibt eine Antwort, die kein 200 war: Statuscode und
// GitHubs eigener Text, dazu die Einordnung, die der Vertrag nennt.
func apiFailure(what string, answer apiAnswer) string {
	text := answer.message
	if text == "" {
		text = http.StatusText(answer.status)
	}
	message := fmt.Sprintf("HTTP %d bei %s: %s", answer.status, what, text)
	switch {
	case answer.status == http.StatusUnauthorized:
		message += " — das Token ist ungültig oder zurückgezogen."
	case answer.status == http.StatusForbidden && isSSOText(text):
		message += " — die Organisation verlangt die SAML/SSO-Autorisierung dieses Tokens."
	case answer.status == http.StatusNotFound && strings.HasPrefix(what, "GET /repos/"):
		message += " — das Repository ist mit diesem Zugang nicht erreichbar; ein fine-grained Token braucht dafür die Freigabe der Organisation."
	}
	return message
}

func isSSOText(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, "saml") || strings.Contains(lower, "sso") || strings.Contains(lower, "single sign-on")
}

// expiryLayouts sind die Formen, in denen GitHub den Ablauf schreibt.
var expiryLayouts = []string{
	"2006-01-02 15:04:05 MST",
	"2006-01-02 15:04:05 -0700",
	"2006-01-02 15:04:05 -07:00",
	time.RFC3339,
	"2006-01-02",
}

func parseExpiry(raw string) (time.Time, bool) {
	for _, layout := range expiryLayouts {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

// calendarDays zählt die Kalendertage von now bis until in UTC: Ein Ablauf
// am 6. November ist am 28. Oktober „in 9 Tagen“, auch wenn es bis zur
// Stunde des Ablaufs noch keine neun volle Tage sind.
func calendarDays(now time.Time, until time.Time) int {
	day := func(t time.Time) time.Time {
		t = t.UTC()
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	}
	return int(day(until).Sub(day(now)).Hours() / 24)
}

func expiryWarning(days int) string {
	switch {
	case days <= 0:
		return "läuft heute ab"
	case days == 1:
		return "läuft in 1 Tag ab"
	}
	return fmt.Sprintf("läuft in %d Tagen ab", days)
}

// sshGreeting ist GitHubs Antwort auf ssh -T: "Hi <name>! You've successfully
// authenticated …" — ein Nutzer, oder <owner>/<repo> für einen Deploy Key.
var sshGreeting = regexp.MustCompile(`Hi ([^!\s]+)!`)

// testSSH: ssh -T für die Identität, git ls-remote für das Leserecht.
// Schreiben lässt sich über SSH ohne Push nicht feststellen — unknown.
func (t *Tester) testSSH(ctx context.Context, repo string, access Access, result *AccessResult) {
	target := access.Host
	sshOptions := []string{"-o", "BatchMode=yes"}
	if access.Key != "" {
		target = ghconfig.Host
		sshOptions = append(sshOptions, "-i", access.Key, "-o", "IdentitiesOnly=yes")
	}
	args := append([]string{"-T"}, sshOptions...)
	args = append(args, "git@"+target)
	_, stderr, err := t.run(ctx, nil, "ssh", args...)
	match := sshGreeting.FindStringSubmatch(stderr)
	if match == nil {
		result.Result = ResultFailed
		result.Message = sshFailure("ssh -T git@"+target, stderr, err, t.timeout())
		return
	}
	result.Identity = match[1]
	result.Write = AnswerUnknown
	result.Expires = ExpiresNone

	// git ruft ssh über GIT_SSH_COMMAND; die Optionen stehen dort, damit
	// derselbe Schlüssel und BatchMode gelten wie beim Gruß.
	gitSSH := "ssh"
	for _, option := range sshOptions {
		gitSSH += " " + shellQuote(option)
	}
	url := "git@" + target + ":" + repo + ".git"
	_, stderr, err = t.run(ctx, []string{"GIT_SSH_COMMAND=" + gitSSH, "GIT_TERMINAL_PROMPT=0"}, "git", "ls-remote", url, "HEAD")
	if err != nil {
		result.Read = AnswerNo
		result.Result = ResultFailed
		result.Message = sshFailure("git ls-remote "+url, stderr, err, t.timeout())
		return
	}
	result.Read = AnswerYes
	result.Result = ResultOK
}

// testPublic: git ls-remote über HTTPS ohne Anmeldung. Der leere
// credential.helper verwirft die konfigurierten Helfer, damit kein
// gespeichertes Credential den anonymen Zugriff vortäuscht.
func (t *Tester) testPublic(ctx context.Context, repo string, result *AccessResult) {
	result.Identity = IdentityAnonymous
	result.Write = AnswerNo
	result.Expires = ExpiresNone
	url := "https://" + ghconfig.Host + "/" + repo + ".git"
	_, stderr, err := t.run(ctx, []string{"GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=/bin/true"}, "git", "-c", "credential.helper=", "ls-remote", url, "HEAD")
	if err != nil {
		result.Read = AnswerNo
		result.Result = ResultFailed
		result.Message = sshFailure("git ls-remote "+url, stderr, err, t.timeout())
		return
	}
	result.Read = AnswerYes
	result.Result = ResultOK
}

// sshFailure beschreibt einen gescheiterten Kindprozess (ssh, git, gh): die
// Frist mit dem Wort Zeitüberschreitung, sonst die Zeilen auf stderr, wie
// GitHub und SSH sie schreiben — sie enthalten nie ein Credential.
func sshFailure(what string, stderr string, err error, timeout time.Duration) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "Zeitüberschreitung: " + what + " hat nicht innerhalb von " + timeout.String() + " geantwortet."
	}
	detail := firstLines(stderr)
	if detail == "" && err != nil {
		detail = err.Error()
	}
	if detail == "" {
		detail = "keine Ausgabe"
	}
	return what + ": " + detail
}

// firstLines fasst stderr zu einem Satz zusammen: die nicht-leeren Zeilen,
// höchstens vier, mit „ / “ verbunden und auf 400 Zeichen gekürzt.
func firstLines(stderr string) string {
	var lines []string
	for _, line := range strings.Split(stderr, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line != "" {
			lines = append(lines, line)
		}
		if len(lines) == 4 {
			break
		}
	}
	text := strings.Join(lines, " / ")
	if len(text) > 400 {
		text = text[:400] + "…"
	}
	return text
}

// shellQuote setzt ein Argument für GIT_SSH_COMMAND in einfache
// Anführungszeichen; git reicht die Variable an die Shell.
func shellQuote(value string) string {
	if value != "" && !strings.ContainsAny(value, " \t'\"\\$`") {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

// redact entfernt das Token aus einem Text, auch Präfixe davon ab acht
// Zeichen: Keine Meldung soll es tragen, auch keine, die ein fremdes
// Programm geschrieben hat.
func redact(text string, token string) string {
	if token == "" || text == "" {
		return text
	}
	if strings.Contains(text, token) {
		text = strings.ReplaceAll(text, token, "[token]")
	}
	for length := len(token); length >= 8; length-- {
		prefix := token[:length]
		if strings.Contains(text, prefix) {
			text = strings.ReplaceAll(text, prefix, "[token]")
			break
		}
	}
	return text
}

// run startet einen Kindprozess mit der Frist des Testers. Die Umgebung ist
// die eigene ohne GH_TOKEN und GITHUB_TOKEN, dazu extra; stdout und stderr
// kommen getrennt zurück. Ein Prozess, den die Frist beendet hat, meldet den
// Kontextfehler, nicht den Exit-Status.
func (t *Tester) run(ctx context.Context, extra []string, name string, args ...string) (stdout string, stderr string, err error) {
	if err := ctx.Err(); err != nil {
		return "", "", err
	}
	callCtx, cancel := context.WithTimeout(ctx, t.timeout())
	defer cancel()
	command := exec.CommandContext(callCtx, name, args...)
	command.WaitDelay = time.Second
	command.Env = append(childEnv(), extra...)
	command.Stdin = nil
	var out, errOut bytes.Buffer
	command.Stdout, command.Stderr = &out, &errOut
	err = command.Run()
	if err != nil && callCtx.Err() != nil {
		err = callCtx.Err()
	}
	return out.String(), errOut.String(), err
}

// childEnv ist die eigene Umgebung ohne die Token-Variablen von gh.
func childEnv() []string {
	env := make([]string, 0, len(os.Environ()))
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "GH_TOKEN=") || strings.HasPrefix(entry, "GITHUB_TOKEN=") {
			continue
		}
		env = append(env, entry)
	}
	return env
}
