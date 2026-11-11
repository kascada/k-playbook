package kephalaion

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Die Auskunft für Clients (Task 138): k-playbook sucht config, Node-Adresse
// und Account nicht selbst, sondern fragt kephalaion node client. Dessen
// Vertrag steht in kephalaion node client --help und in docs/installation.md
// von Kephalaion; hier wird Format 1 gelesen, unbekannte Felder werden
// übergangen. Die Auskunft enthält nie ein Token, nur den Pfad der
// Token-Datei; gelesen wird sie hier nur, wenn sie genau dort liegt, wo
// Kephalaion sie ablegt (<tokensDir>/<hub>/<account>.token).

// ClientFormat ist die Fassung der Auskunft, die k-playbook liest; sie geht
// als --format mit, und eine Antwort mit anderer Fassung gilt als zu altes
// oder fremdes Binary.
const ClientFormat = 1

// ClientTimeout begrenzt den Aufruf von kephalaion node client. Mit
// --no-probe fragt die Auskunft keinen Node, sie liest nur Dateien; die
// Grenze greift nur, wenn das Binary hängt. Austauschbar für Tests.
var ClientTimeout = 5 * time.Second

// maxClientOutput begrenzt, was von stdout gelesen wird; maxClientStderr das
// von stderr — gebraucht wird dort nur die erste Zeile.
const (
	maxClientOutput = 1 << 20
	maxClientStderr = 64 << 10
)

// Choice sagt, woher der Account eines Hubs kommt — die Reihenfolge der Wahl
// in Kephalaion: die Überschreibung des Clients, der Standard (kephalaion
// node account default), die einzige Token-Datei; trifft nichts zu, fehlt
// die Wahl. k-playbook ruft ohne --account auf, eine Überschreibung kommt
// also nur von außen.
type Choice string

// Die Werte von choice in Format 1.
const (
	ChoiceOverride Choice = "override"
	ChoiceDefault  Choice = "default"
	ChoiceOnly     Choice = "only"
	ChoiceMissing  Choice = "missing"
)

// ClientInfo ist die Auskunft im Format 1, soweit k-playbook sie liest.
type ClientInfo struct {
	Format     int          `json:"format"`
	Kephalaion string       `json:"kephalaion"`
	Config     ClientConfig `json:"config"`
	Node       ClientNode   `json:"node"`
	TokensDir  string       `json:"tokensDir"`
	Hubs       []ClientHub  `json:"hubs"`
}

// ClientConfig sagt, welche config von Kephalaion gilt. source none ist kein
// Fehler: Dann gilt die Standardadresse des Nodes.
type ClientConfig struct {
	Path    *string `json:"path"`
	Source  string  `json:"source"`
	Exists  bool    `json:"exists"`
	Problem *string `json:"problem"`
}

// ClientNode ist der lokale Node dieses Rechners. Mit --no-probe sind
// reachable und version null.
type ClientNode struct {
	Listen    *string `json:"listen"`
	URL       string  `json:"url"`
	Source    string  `json:"source"`
	Reachable *bool   `json:"reachable"`
	Version   *string `json:"version"`
}

// ClientHub ist der Eintrag eines Hubs: der gewählte Account samt Herkunft,
// der Pfad seiner Token-Datei, die Kandidaten (Accounts mit Token-Datei) und
// die Accounts mit wartendem Token (.pending). notes sind nur zum Anzeigen;
// entschieden wird nach choice.
type ClientHub struct {
	Alias      string   `json:"alias"`
	Account    *string  `json:"account"`
	Choice     Choice   `json:"choice"`
	TokenFile  *string  `json:"tokenFile"`
	Candidates []string `json:"candidates"`
	Pending    []string `json:"pending"`
	Notes      []string `json:"notes"`
}

// ClientError ist ein Zustand, an dem die Anbindung scheitert, bevor ein Node
// gefragt wird: das Binary, die Auskunft oder was sie für den Hub nennt.
// Message ist ein Satz samt Abhilfe, ohne Token.
type ClientError struct {
	State   State
	Message string
}

func (e *ClientError) Error() string { return e.Message }

// clientCall ist der Aufruf, den eine Abhilfe zum Prüfen von Hand nennt.
func clientCall(hubs []string) string {
	call := "kephalaion node client --json --format " + strconv.Itoa(ClientFormat) + " --no-probe"
	for _, hub := range hubs {
		call += " --hub " + hub
	}
	return call
}

// missingBinaryMessage ist der Satz, wenn kephalaion an keinem der Orte liegt.
func missingBinaryMessage() string {
	return "Kephalaion ist auf diesem Rechner nicht installiert: kein " + BinaryName +
		" im PATH, unter ~/.local/bin oder unter /usr/local/bin. Abhilfe: installieren mit " +
		InstallCommand + "; danach die Anbindung prüfen (k-playbook context)."
}

// tooOldMessage ist der Satz, wenn das Binary die Auskunft im Format 1 nicht
// liefert — Exit 2, keine JSON-Ausgabe oder eine andere Fassung.
func tooOldMessage(path, reason string) string {
	return fmt.Sprintf("Die Auskunft von %s (kephalaion node client im Format %d) ist nicht zu haben: %s. "+
		"k-playbook braucht sie, Kephalaion liefert sie ab v0.4.0. Abhilfe: kephalaion upgrade.",
		path, ClientFormat, reason)
}

// QueryClient fragt kephalaion node client --json --format 1 --no-probe nach
// der Auskunft, für die genannten Hubs (je --hub genau ein Eintrag, auch ohne
// Token-Verzeichnis) oder ohne Hub für alle mit Verzeichnis unter tokens/.
// Ein Fehler ist ein *ClientError mit Zustand und Satz samt Abhilfe:
//
//   - kein Binary an den Orten von FindBinary: StateNoKephalaion mit
//     InstallCommand;
//   - Exit 2, keine JSON-Ausgabe oder format ≠ 1: StateNoKephalaion mit
//     kephalaion upgrade und der ersten Zeile von stderr;
//   - Exit 1 (die Auskunft konnte nicht entstehen, etwa ohne
//     Heimatverzeichnis oder mit nicht lesbarem tokens/): StateClientFailed
//     mit der ersten Zeile von stderr als Grund, ohne kephalaion upgrade;
//   - Zeitgrenze, Absturz oder kein Start: StateClientFailed.
func QueryClient(ctx context.Context, hubs ...string) (ClientInfo, error) {
	for _, hub := range hubs {
		// Ein Hub wird ein Argument; die Namensregel schließt einen führenden
		// Bindestrich aus, also auch eine eingeschleuste Option.
		if err := CheckName("Hub", hub); err != nil {
			return ClientInfo{}, &ClientError{StateInvalidEntry, "Eintrag ungültig: " + err.Error() + "."}
		}
	}
	path, ok := FindBinary()
	if !ok {
		return ClientInfo{}, &ClientError{StateNoKephalaion, missingBinaryMessage()}
	}
	args := []string{"node", "client", "--json", "--format", strconv.Itoa(ClientFormat), "--no-probe"}
	for _, hub := range hubs {
		args = append(args, "--hub", hub)
	}
	call := clientCall(hubs)

	runCtx, cancel := context.WithTimeout(ctx, ClientTimeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, path, args...)
	stdout := &cappedBuffer{limit: maxClientOutput}
	stderr := &cappedBuffer{limit: maxClientStderr}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	// Ein Kind, das die Ausgabe offen hält, hält Wait sonst über die
	// Zeitgrenze hinaus fest.
	cmd.WaitDelay = time.Second
	err := cmd.Run()
	reason := firstLine(stderr.String())

	if err != nil {
		switch {
		case errors.Is(runCtx.Err(), context.DeadlineExceeded):
			return ClientInfo{}, &ClientError{StateClientFailed, fmt.Sprintf(
				"Die Auskunft kephalaion node client hat nicht innerhalb von %s geantwortet. "+
					"Abhilfe: %s von Hand aufrufen; hängt es dort auch, kephalaion neu installieren (kephalaion upgrade).",
				ClientTimeout, call)}
		case runCtx.Err() != nil:
			return ClientInfo{}, &ClientError{StateClientFailed,
				"Die Auskunft kephalaion node client wurde abgebrochen, bevor sie stand. Abhilfe: erneut prüfen (k-playbook context)."}
		}
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			return ClientInfo{}, &ClientError{StateClientFailed, fmt.Sprintf(
				"kephalaion unter %s lässt sich nicht starten (%s). Abhilfe: neu installieren mit %s.",
				path, sanitize(err.Error()), InstallCommand)}
		}
		switch code := exitErr.ExitCode(); code {
		case 2:
			return ClientInfo{}, &ClientError{StateNoKephalaion, tooOldMessage(path, withReason("Exit 2", reason))}
		case 1:
			if reason == "" {
				reason = "ohne Grund auf stderr"
			}
			return ClientInfo{}, &ClientError{StateClientFailed, fmt.Sprintf(
				"Die Auskunft kephalaion node client konnte nicht entstehen: %s. "+
					"Abhilfe: den Grund beheben (etwa das Heimatverzeichnis setzen oder die Rechte am Token-Verzeichnis prüfen) "+
					"und mit %s von Hand prüfen.", reason, call)}
		default:
			how := "Exit " + strconv.Itoa(code)
			if code < 0 {
				how = "beendet von außen"
			}
			return ClientInfo{}, &ClientError{StateClientFailed, fmt.Sprintf(
				"Die Auskunft kephalaion node client ist abgebrochen (%s). "+
					"Abhilfe: %s von Hand aufrufen; bricht es dort auch ab, kephalaion neu installieren (kephalaion upgrade).",
				withReason(how, reason), call)}
		}
	}

	info, problem := parseClientInfo(stdout)
	if problem != "" {
		return ClientInfo{}, &ClientError{StateNoKephalaion, tooOldMessage(path, problem)}
	}
	for _, hub := range hubs {
		if !slices.ContainsFunc(info.Hubs, func(h ClientHub) bool { return h.Alias == hub }) {
			return ClientInfo{}, &ClientError{StateClientFailed, fmt.Sprintf(
				"Die Auskunft kephalaion node client nennt den Hub %s nicht, obwohl er gefragt war. "+
					"Abhilfe: %s von Hand prüfen; fehlt er dort auch, kephalaion upgrade.", hub, call)}
		}
	}
	return info, nil
}

// parseClientInfo liest stdout als genau ein JSON-Objekt im Format 1. problem
// sagt, warum nicht — ohne den Text der Ausgabe zu zitieren.
func parseClientInfo(stdout *cappedBuffer) (ClientInfo, string) {
	if stdout.truncated {
		return ClientInfo{}, fmt.Sprintf("Ausgabe länger als %d Bytes", maxClientOutput)
	}
	var info ClientInfo
	decoder := json.NewDecoder(bytes.NewReader(stdout.Bytes()))
	if err := decoder.Decode(&info); err != nil {
		return ClientInfo{}, "keine JSON-Ausgabe"
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return ClientInfo{}, "mehr als ein JSON-Objekt auf stdout"
	}
	if info.Format != ClientFormat {
		return ClientInfo{}, fmt.Sprintf("Format %d statt %d", info.Format, ClientFormat)
	}
	return info, ""
}

// Access ist der Zugang zu einem Hub, wie die Auskunft ihn ergibt: der
// Account samt Herkunft, die Adresse von /mcp am Node (nur Loopback) und die
// Token-Datei. Das Token selbst liest erst readToken; der Pfad bleibt
// unexportiert und gelangt in kein Ergebnis.
type Access struct {
	Hub     string
	Account string
	Choice  Choice
	// Endpoint ist die URL von /mcp am Node, geprüft auf 127.0.0.1, [::1]
	// oder localhost.
	Endpoint string
	// TokensDir ist das Token-Verzeichnis aus der Auskunft.
	TokensDir string
	// ConfigProblem ist der Satz, wenn die config von Kephalaion nicht
	// lesbar ist (config.problem); leer sonst. Die Auskunft nimmt dann die
	// Standardadresse des Nodes.
	ConfigProblem string
	// Note ist ein Hinweis samt Abhilfe, der die Anbindung nicht verhindert:
	// neben der Token-Datei liegt eine .pending (rotate offen).
	Note string

	tokenFile string
}

// ResolveAccess fragt die Auskunft für hub und prüft, was sie nennt
// (ResolveInfo).
func ResolveAccess(ctx context.Context, hub string) (Access, error) {
	info, err := QueryClient(ctx, hub)
	if err != nil {
		return Access{}, err
	}
	return ResolveInfo(info, hub)
}

// ResolveInfo prüft den Eintrag von hub in einer Auskunft. Ein Fehler ist
// ein *ClientError:
//
//   - node.url ist keine Loopback-Adresse: StateNodeAddress;
//   - choice missing mit Kandidaten: StateAccountChoiceMissing, mit den
//     Kandidaten und kephalaion node account default;
//   - choice missing ohne Kandidaten: StateTokenMissing, mit kephalaion node
//     account setup — oder, wenn nur ein wartendes Token liegt, mit dem Weg,
//     es zu klären;
//   - ein Account ohne Token-Datei, eine Token-Datei an einem anderen Ort
//     als <tokensDir>/<hub>/<account>.token oder ein Name außerhalb der
//     Namensregel: StateTokenMissing.
//
// Ist die config nicht lesbar (config.problem), steht der Satz dazu in
// ConfigProblem und hinter jeder Meldung.
func ResolveInfo(info ClientInfo, hub string) (Access, error) {
	configProblem := configProblemSentence(info.Config)
	fail := func(state State, message string) (Access, error) {
		if configProblem != "" {
			message += " " + configProblem
		}
		return Access{}, &ClientError{State: state, Message: message}
	}
	if err := CheckName("Hub", hub); err != nil {
		return fail(StateInvalidEntry, "Eintrag ungültig: "+err.Error()+".")
	}
	var entry *ClientHub
	for i := range info.Hubs {
		if info.Hubs[i].Alias == hub {
			entry = &info.Hubs[i]
			break
		}
	}
	if entry == nil {
		return fail(StateClientFailed, fmt.Sprintf(
			"Die Auskunft kephalaion node client nennt den Hub %s nicht. Abhilfe: %s von Hand prüfen; fehlt er dort auch, kephalaion upgrade.",
			hub, clientCall([]string{hub})))
	}

	endpoint, problem := loopbackURL(info.Node.URL)
	if problem != "" {
		return fail(StateNodeAddress, problem+" "+nodeAddressRemedy(info.Config))
	}

	tokensDir := info.TokensDir
	if tokensDir == "" || !filepath.IsAbs(tokensDir) || filepath.Clean(tokensDir) != tokensDir {
		return fail(StateClientFailed, fmt.Sprintf(
			"Die Auskunft kephalaion node client nennt kein gültiges Token-Verzeichnis. Abhilfe: %s von Hand prüfen; "+
				"fehlt es dort auch, kephalaion upgrade.", clientCall([]string{hub})))
	}

	switch entry.Choice {
	case ChoiceMissing:
		candidates := validNames(entry.Candidates)
		if len(candidates) > 0 {
			return fail(StateAccountChoiceMissing, choiceMissingMessage(hub, candidates))
		}
		if waiting := waitingOnly(entry); len(waiting) > 0 {
			return fail(StateTokenMissing, pendingOnlyMessage(hub, waiting))
		}
		return fail(StateTokenMissing, noTokenMessage(hub, tokensDir))
	case ChoiceOverride, ChoiceDefault, ChoiceOnly:
	default:
		return fail(StateClientFailed, fmt.Sprintf(
			"Die Auskunft kephalaion node client nennt für den Hub %s eine unbekannte Wahl des Accounts. "+
				"Abhilfe: %s von Hand prüfen; kephalaion upgrade.", hub, clientCall([]string{hub})))
	}

	if entry.Account == nil {
		return fail(StateTokenMissing, noTokenMessage(hub, tokensDir))
	}
	account := *entry.Account
	if err := checkAccountName(account); err != nil {
		return fail(StateTokenMissing, fmt.Sprintf(
			"Die Auskunft nennt für den Hub %s einen Account, den k-playbook nicht nimmt (%v). "+
				"Abhilfe: einen gültigen Account als Standard setzen: kephalaion node account default %s <account>.",
			hub, err, hub))
	}
	expected := filepath.Join(tokensDir, hub, account+".token")
	if entry.TokenFile == nil {
		return fail(StateTokenMissing, fmt.Sprintf(
			"Für Account %s am Hub %s fehlt die Token-Datei %s. Abhilfe: mit einem Einrichtungstoken vom Admin "+
				"(kephalaion hub account add bzw. kephalaion hub account token %s) einrichten: kephalaion node account setup %s %s.",
			account, hub, expected, account, hub, account))
	}
	if *entry.TokenFile != expected {
		return fail(StateTokenMissing, fmt.Sprintf(
			"Die Auskunft nennt für Account %s am Hub %s die Token-Datei %s; k-playbook liest ein Token nur aus %s. "+
				"Abhilfe: kephalaion node account list zeigt die Token-Dateien; liegt das Token woanders, den Account "+
				"dort einrichten: kephalaion node account setup %s %s.",
			account, hub, sanitize(*entry.TokenFile), expected, hub, account))
	}

	access := Access{
		Hub: hub, Account: account, Choice: entry.Choice, Endpoint: endpoint,
		TokensDir: tokensDir, ConfigProblem: configProblem, tokenFile: expected,
	}
	if slices.Contains(entry.Pending, account) {
		access.Note = fmt.Sprintf(
			"Neben der Token-Datei von Account %s am Hub %s liegt eine .pending: Ein Tausch des Tokens (rotate) ist offen. "+
				"Abhilfe: kephalaion node account check %s %s --token-file %s räumt auf.",
			account, hub, hub, account, expected)
	}
	return access, nil
}

// choiceMissingMessage nennt die Kandidaten und den Weg, einen Standard zu
// setzen. Der letzte Satz ist fest: Ob das Token-Verzeichnis nur lesend
// eingebunden ist, wird nicht erkannt.
func choiceMissingMessage(hub string, candidates []string) string {
	return fmt.Sprintf(
		"Für den Hub %s ist auf diesem Rechner kein Account gewählt: Es gibt mehrere Token-Dateien (%s) und keinen Standard. "+
			"Abhilfe: einen davon als Standard setzen, etwa kephalaion node account default %s %s. "+
			"Ist das Token-Verzeichnis nur lesend eingebunden, wie im DevContainer, wird der Standard auf dem Host gesetzt.",
		hub, strings.Join(candidates, ", "), hub, candidates[0])
}

// noTokenMessage ist der Satz, wenn es für den Hub keine Token-Datei gibt.
func noTokenMessage(hub, tokensDir string) string {
	return fmt.Sprintf(
		"Für den Hub %s gibt es auf diesem Rechner keine Token-Datei (Token-Verzeichnis %s). "+
			"Abhilfe: mit einem Einrichtungstoken vom Admin (kephalaion hub account add bzw. kephalaion hub account token) "+
			"einrichten: kephalaion node account setup %s <account>; ist Kephalaion auf diesem Rechner noch gar nicht "+
			"eingerichtet, zuerst kephalaion node setup <code>.",
		hub, tokensDir, hub)
}

// pendingOnlyMessage ist der Satz, wenn für den Hub nur wartende Tokens
// liegen: Der Ausgang von setup ist unklar.
func pendingOnlyMessage(hub string, waiting []string) string {
	account := waiting[0]
	return fmt.Sprintf(
		"Für den Hub %s wartet das Token von Account %s noch auf den Tausch (nur eine .pending, keine Token-Datei). "+
			"Abhilfe: derselbe Aufruf kephalaion node account setup %s %s klärt sie; gilt keins der Tokens, "+
			"erzeugt der Admin ein neues Einrichtungstoken (kephalaion hub account token %s).",
		hub, strings.Join(waiting, ", "), hub, account, account)
}

// waitingOnly sind die Accounts mit wartendem Token ohne Token-Datei.
func waitingOnly(entry *ClientHub) []string {
	var out []string
	for _, account := range validNames(entry.Pending) {
		if !slices.Contains(entry.Candidates, account) {
			out = append(out, account)
		}
	}
	return out
}

// validNames sind die Accounts, die die Namensregel bestehen; nur sie kommen
// in eine Meldung.
func validNames(names []string) []string {
	var out []string
	for _, name := range names {
		if checkAccountName(name) == nil && !slices.Contains(out, name) {
			out = append(out, name)
		}
	}
	return out
}

// configProblemSentence ist der Satz zu config.problem; leer ohne Problem.
func configProblemSentence(config ClientConfig) string {
	if config.Problem == nil || strings.TrimSpace(*config.Problem) == "" {
		return ""
	}
	where := ""
	if config.Path != nil && *config.Path != "" {
		where = " " + sanitize(*config.Path)
	}
	return fmt.Sprintf("Die config von Kephalaion%s ist nicht lesbar (%s); für den Node gilt die Standardadresse. "+
		"Abhilfe: die config prüfen (kephalaion status).", where, sanitize(*config.Problem))
}

// nodeAddressRemedy nennt den Weg zu einer Loopback-Adresse.
func nodeAddressRemedy(config ClientConfig) string {
	where := "der config von Kephalaion"
	if config.Path != nil && *config.Path != "" {
		where += " (" + sanitize(*config.Path) + ")"
	}
	return "Abhilfe: in " + where + " unter node: listen auf eine Loopback-Adresse setzen, etwa 127.0.0.1:7433, " +
		"und den Node neu starten (kephalaion status)."
}

// urlPattern ist die Form, in der node.url in einer Meldung zitiert wird —
// ohne Unterstrich, damit ein Token (keph_…) nie passt.
var urlPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]{0,15}://[A-Za-z0-9.:\[\]-]{1,260}(/[A-Za-z0-9./-]{0,200})?$`)

// quoteURL zitiert node.url nur, wenn sie wie eine Adresse aussieht.
func quoteURL(raw string) string {
	if urlPattern.MatchString(raw) {
		return fmt.Sprintf("%q", raw)
	}
	return "(unzulässiger Wert)"
}

// loopbackURL prüft node.url: http, Host 127.0.0.1, [::1] oder localhost
// (localhost wird nicht aufgelöst, sondern 127.0.0.1), Port 1 bis 65535,
// ohne Anmeldedaten, Anfrage oder Fragment. Jede andere Adresse ist ein
// Problem: Ein Token geht nie an eine Adresse außerhalb dieses Rechners.
func loopbackURL(raw string) (endpoint string, problem string) {
	if raw == "" {
		return "", "Die Auskunft nennt keine Adresse des Nodes."
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "http" || u.Opaque != "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Host == "" {
		return "", fmt.Sprintf("Die Adresse des Nodes %s ist keine http-Adresse der Form http://127.0.0.1:<port>/mcp.", quoteURL(raw))
	}
	n, err := strconv.Atoi(u.Port())
	if err != nil || n < 1 || n > 65535 {
		return "", fmt.Sprintf("Die Adresse des Nodes %s hat keinen Port von 1 bis 65535.", quoteURL(raw))
	}
	host := u.Hostname()
	switch host {
	case "127.0.0.1", "localhost":
		host = "127.0.0.1"
	case "::1":
		host = "[::1]"
	default:
		return "", fmt.Sprintf("Die Adresse des Nodes %s ist keine Loopback-Adresse; k-playbook schickt ein Token nur an 127.0.0.1, [::1] oder localhost.", quoteURL(raw))
	}
	path := u.EscapedPath()
	if path == "" {
		path = "/mcp"
	}
	return "http://" + host + ":" + strconv.Itoa(n) + path, ""
}

// readToken liest das Token aus der Token-Datei des Zugangs, mit den
// Prüfungen von readTokenFile.
func (a Access) readToken() (string, error) {
	return readTokenFile(a.tokenFile)
}

// tokenPattern erkennt ein Token in einem fremden Text (keph_ und
// base64url).
var tokenPattern = regexp.MustCompile(`keph_[A-Za-z0-9_-]{8,}`)

// redactTokens ersetzt alles, was wie ein Token aussieht — die Sicherung für
// Text, der von außen kommt (stderr, config.problem, Pfade aus der Auskunft).
func redactTokens(text string) string {
	return tokenPattern.ReplaceAllString(text, "keph_***")
}

// sanitize macht aus fremdem Text einen Teil einer Meldung: die erste Zeile,
// ohne Steuerzeichen, höchstens 300 Zeichen, ohne Token.
func sanitize(text string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(text), "\n")
	line = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == ' ' || r == ' ' {
			return ' '
		}
		return r
	}, line)
	line = strings.TrimSpace(line)
	if runes := []rune(line); len(runes) > 300 {
		line = string(runes[:300]) + " …"
	}
	return redactTokens(line)
}

// firstLine ist die erste nicht leere Zeile von stderr, bereinigt.
func firstLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) != "" {
			return sanitize(line)
		}
	}
	return ""
}

// withReason hängt einen Grund in Klammern an, wenn es einen gibt.
func withReason(what, reason string) string {
	if reason == "" {
		return what
	}
	return what + ": " + reason
}

// cappedBuffer nimmt höchstens limit Bytes und verwirft den Rest, ohne den
// Schreiber aufzuhalten.
type cappedBuffer struct {
	bytes.Buffer
	limit     int
	truncated bool
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	if room := b.limit - b.Len(); room < len(p) {
		b.truncated = true
		if room > 0 {
			b.Buffer.Write(p[:room])
		}
		return len(p), nil
	}
	return b.Buffer.Write(p)
}
