package kephalaion

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// fakeClient ist ein Ersatz-Binary für kephalaion node client: ein Skript,
// das seine Argumente nach args schreibt, stdout und stderr ausgibt und mit
// dem Code aus exit endet. (kephtest.FakeClient tut dasselbe für andere
// Pakete; hier ginge der Import im Kreis.)
type fakeClient struct {
	dir  string
	path string
}

func newFakeClient(t *testing.T) *fakeClient {
	t.Helper()
	dir := t.TempDir()
	f := &fakeClient{dir: dir, path: filepath.Join(dir, "kephalaion")}
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" > '" + dir + "/args'\n" +
		"cat '" + dir + "/stdout'\n" +
		"cat '" + dir + "/stderr' >&2\n" +
		"exit $(cat '" + dir + "/exit')\n"
	writeFile(t, f.path, script)
	if err := os.Chmod(f.path, 0o755); err != nil {
		t.Fatal(err)
	}
	f.set(t, "", "", 0)
	original := LookupBinary
	LookupBinary = func() (string, bool) { return f.path, true }
	t.Cleanup(func() { LookupBinary = original })
	return f
}

func (f *fakeClient) set(t *testing.T, stdout, stderr string, exit int) {
	t.Helper()
	writeFile(t, filepath.Join(f.dir, "stdout"), stdout)
	writeFile(t, filepath.Join(f.dir, "stderr"), stderr)
	writeFile(t, filepath.Join(f.dir, "exit"), strconv.Itoa(exit))
}

func (f *fakeClient) setInfo(t *testing.T, info ClientInfo) {
	t.Helper()
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	f.set(t, string(data)+"\n", "", 0)
}

// argsOrNil sind die Argumente des letzten Aufrufs, nil ohne Aufruf.
func (f *fakeClient) argsOrNil(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(f.dir, "args"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Fields(string(data))
}

func (f *fakeClient) args(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(f.dir, "args"))
	if err != nil {
		t.Fatal(err)
	}
	return strings.Join(strings.Fields(string(data)), " ")
}

func ptr[T any](v T) *T { return &v }

// infoFor baut eine Auskunft: config des Users, Node auf 127.0.0.1:7433, das
// Token-Verzeichnis tokensDir und die Hubs.
func infoFor(tokensDir string, hubs ...ClientHub) ClientInfo {
	if hubs == nil {
		hubs = []ClientHub{}
	}
	return ClientInfo{
		Format: 1, Kephalaion: "v0.4.0",
		Config:    ClientConfig{Path: ptr(filepath.Join(filepath.Dir(tokensDir), "config.yaml")), Source: "user", Exists: true},
		Node:      ClientNode{Listen: ptr("127.0.0.1:7433"), URL: "http://127.0.0.1:7433/mcp", Source: "config"},
		TokensDir: tokensDir,
		Hubs:      hubs,
	}
}

// hubWith ist ein Eintrag mit gewähltem Account und dessen Token-Datei.
func hubWith(tokensDir, hub, account string, choice Choice, candidates ...string) ClientHub {
	if candidates == nil {
		candidates = []string{account}
	}
	return ClientHub{Alias: hub, Account: ptr(account), Choice: choice,
		TokenFile:  ptr(filepath.Join(tokensDir, hub, account+".token")),
		Candidates: candidates, Pending: []string{}, Notes: []string{}}
}

// tokensWith legt ein Token-Verzeichnis mit einer Datei für vm/kamran-wsl an.
func tokensWith(t *testing.T) string {
	t.Helper()
	tokens := filepath.Join(t.TempDir(), "kephalaion", "tokens")
	writeFile(t, filepath.Join(tokens, "vm", "kamran-wsl.token"), testToken+"\n")
	return tokens
}

// clientState holt die Auskunft über das Ersatz-Binary und prüft sie.
func clientState(t *testing.T, hub string) (Access, *ClientError) {
	t.Helper()
	access, err := ResolveAccess(context.Background(), hub)
	if err == nil {
		return access, nil
	}
	var clientErr *ClientError
	if !errors.As(err, &clientErr) {
		t.Fatalf("kein ClientError: %T %v", err, err)
	}
	if strings.Contains(clientErr.Message, "TOKENLEAKCANARY") {
		t.Fatalf("Token in der Meldung: %s", clientErr.Message)
	}
	return Access{}, clientErr
}

func expectState(t *testing.T, err *ClientError, state State, contains ...string) {
	t.Helper()
	if err == nil {
		t.Fatalf("Zustand %s erwartet, Zugang bekommen", state)
	}
	if err.State != state {
		t.Fatalf("Zustand %s, erwartet %s (%s)", err.State, state, err.Message)
	}
	for _, want := range contains {
		if !strings.Contains(err.Message, want) {
			t.Errorf("Meldung ohne %q:\n%s", want, err.Message)
		}
	}
	if state != StateInvalidEntry && !strings.Contains(err.Message, "Abhilfe") {
		t.Errorf("Meldung ohne Abhilfe:\n%s", err.Message)
	}
}

func TestLocateBinaryOrder(t *testing.T) {
	dir := t.TempDir()
	local := filepath.Join(dir, "home", ".local", "bin", BinaryName)
	global := filepath.Join(dir, "usr", "local", "bin", BinaryName)
	notInPath := func(string) (string, error) { return "", errors.New("nicht im PATH") }

	if _, ok := locateBinary(notInPath, []string{local, global}); ok {
		t.Fatal("ohne Datei gefunden")
	}
	writeFile(t, global, "#!/bin/sh\n")
	if _, ok := locateBinary(notInPath, []string{local, global}); ok {
		t.Fatal("nicht ausführbare Datei gefunden")
	}
	if err := os.Chmod(global, 0o755); err != nil {
		t.Fatal(err)
	}
	if path, ok := locateBinary(notInPath, []string{local, global}); !ok || path != global {
		t.Fatalf("global: %q %v", path, ok)
	}
	writeFile(t, local, "#!/bin/sh\n")
	if err := os.Chmod(local, 0o755); err != nil {
		t.Fatal(err)
	}
	if path, _ := locateBinary(notInPath, []string{local, global}); path != local {
		t.Fatalf("~/.local/bin vor global erwartet: %q", path)
	}
	inPath := func(string) (string, error) { return "/opt/bin/kephalaion", nil }
	if path, _ := locateBinary(inPath, []string{local, global}); path != "/opt/bin/kephalaion" {
		t.Fatalf("PATH zuerst erwartet: %q", path)
	}
	if got := binaryFallbacks("/home/u"); len(got) != 2 || got[0] != "/home/u/.local/bin/kephalaion" || got[1] != GlobalBinary {
		t.Fatalf("Orte: %v", got)
	}
	if got := binaryFallbacks(""); len(got) != 1 || got[0] != GlobalBinary {
		t.Fatalf("ohne Heimatverzeichnis: %v", got)
	}
}

// Die einzige Token-Datei: Account und Herkunft aus der Auskunft, der Aufruf
// mit --json --format 1 --no-probe --hub.
func TestClientOnlyTokenFile(t *testing.T) {
	fake := newFakeClient(t)
	tokens := tokensWith(t)
	fake.setInfo(t, infoFor(tokens, hubWith(tokens, "vm", "kamran-wsl", ChoiceOnly)))

	access, err := clientState(t, "vm")
	if err != nil {
		t.Fatalf("Fehler: %v", err)
	}
	if access.Account != "kamran-wsl" || access.Choice != ChoiceOnly || access.Endpoint != "http://127.0.0.1:7433/mcp" {
		t.Fatalf("Zugang: %+v", access)
	}
	if got := fake.args(t); got != "node client --json --format 1 --no-probe --hub vm" {
		t.Fatalf("Aufruf: %q", got)
	}
	if token, err := access.readToken(); err != nil || token != testToken {
		t.Fatalf("Token nicht gelesen: %v", err)
	}
	if access.Note != "" || access.ConfigProblem != "" {
		t.Fatalf("Hinweise ohne Anlass: %+v", access)
	}
}

func TestClientDefaultAccount(t *testing.T) {
	fake := newFakeClient(t)
	tokens := tokensWith(t)
	writeFile(t, filepath.Join(tokens, "vm", "zweit.token"), testToken+"\n")
	fake.setInfo(t, infoFor(tokens, hubWith(tokens, "vm", "kamran-wsl", ChoiceDefault, "kamran-wsl", "zweit")))

	access, err := clientState(t, "vm")
	if err != nil || access.Account != "kamran-wsl" || access.Choice != ChoiceDefault {
		t.Fatalf("Standard: %+v %v", access, err)
	}
}

func TestClientChoiceMissingWithCandidates(t *testing.T) {
	fake := newFakeClient(t)
	tokens := tokensWith(t)
	fake.setInfo(t, infoFor(tokens, ClientHub{Alias: "vm", Choice: ChoiceMissing,
		Candidates: []string{"kamran-wsl", "zweit"}, Pending: []string{}, Notes: []string{"Wahl fehlt"}}))

	_, err := clientState(t, "vm")
	expectState(t, err, StateAccountChoiceMissing,
		"kamran-wsl, zweit", "kephalaion node account default vm kamran-wsl",
		"nur lesend eingebunden, wie im DevContainer, wird der Standard auf dem Host gesetzt")
}

func TestClientOnlyPending(t *testing.T) {
	fake := newFakeClient(t)
	tokens := tokensWith(t)
	fake.setInfo(t, infoFor(tokens, ClientHub{Alias: "vm", Choice: ChoiceMissing,
		Candidates: []string{}, Pending: []string{"neu"}, Notes: []string{}}))

	_, err := clientState(t, "vm")
	expectState(t, err, StateTokenMissing, "wartet", "kephalaion node account setup vm neu",
		"kephalaion hub account token neu")
}

// Liegt die .pending neben der .token (rotate offen), trägt der Zugang, und
// der Hinweis nennt check mit --token-file.
func TestClientPendingNextToToken(t *testing.T) {
	fake := newFakeClient(t)
	tokens := tokensWith(t)
	hub := hubWith(tokens, "vm", "kamran-wsl", ChoiceOnly)
	hub.Pending = []string{"kamran-wsl"}
	fake.setInfo(t, infoFor(tokens, hub))

	access, err := clientState(t, "vm")
	if err != nil {
		t.Fatalf("Fehler: %v", err)
	}
	want := "kephalaion node account check vm kamran-wsl --token-file " + filepath.Join(tokens, "vm", "kamran-wsl.token")
	if !strings.Contains(access.Note, want) || !strings.Contains(access.Note, "Abhilfe") {
		t.Fatalf("Hinweis: %q", access.Note)
	}
}

func TestClientNoTokenDirectory(t *testing.T) {
	fake := newFakeClient(t)
	tokens := filepath.Join(t.TempDir(), "tokens")
	fake.setInfo(t, infoFor(tokens, ClientHub{Alias: "vm", Choice: ChoiceMissing,
		Candidates: []string{}, Pending: []string{}, Notes: []string{"keine Token-Datei"}}))

	_, err := clientState(t, "vm")
	expectState(t, err, StateTokenMissing, "kephalaion node account setup vm <account>",
		"kephalaion hub account add", "kephalaion node setup <code>")
}

func TestClientConfigSourceNoneIsNoError(t *testing.T) {
	fake := newFakeClient(t)
	tokens := tokensWith(t)
	info := infoFor(tokens, hubWith(tokens, "vm", "kamran-wsl", ChoiceOnly))
	info.Config = ClientConfig{Source: "none"}
	info.Node = ClientNode{URL: "http://127.0.0.1:7433/mcp", Source: "default"}
	fake.setInfo(t, info)

	access, err := clientState(t, "vm")
	if err != nil || access.Endpoint != "http://127.0.0.1:7433/mcp" || access.ConfigProblem != "" {
		t.Fatalf("ohne config: %+v %v", access, err)
	}
}

func TestClientConfigProblemInMessage(t *testing.T) {
	fake := newFakeClient(t)
	tokens := tokensWith(t)
	info := infoFor(tokens, hubWith(tokens, "vm", "kamran-wsl", ChoiceOnly))
	info.Config.Problem = ptr("Leserecht fehlt")
	info.Node = ClientNode{URL: "http://127.0.0.1:7433/mcp", Source: "default"}
	fake.setInfo(t, info)

	access, err := clientState(t, "vm")
	if err != nil || !strings.Contains(access.ConfigProblem, "Leserecht fehlt") || !strings.Contains(access.ConfigProblem, "Abhilfe") {
		t.Fatalf("Problem nicht genannt: %+v %v", access, err)
	}

	// Scheitert der Zugang, steht der Satz in der Meldung.
	info.Hubs = []ClientHub{{Alias: "vm", Choice: ChoiceMissing, Candidates: []string{}, Pending: []string{}}}
	fake.setInfo(t, info)
	_, failure := clientState(t, "vm")
	expectState(t, failure, StateTokenMissing, "Leserecht fehlt")
}

func TestClientBinaryMissing(t *testing.T) {
	original := LookupBinary
	LookupBinary = func() (string, bool) { return "", false }
	t.Cleanup(func() { LookupBinary = original })

	_, err := clientState(t, "vm")
	expectState(t, err, StateNoKephalaion, InstallCommand)
}

// Exit 1: Die Auskunft konnte nicht entstehen; der Grund kommt aus stderr,
// kephalaion upgrade hilft dort nicht.
func TestClientExit1NamesReasonWithoutUpgrade(t *testing.T) {
	fake := newFakeClient(t)
	fake.set(t, "", "node client: Heimatverzeichnis unbekannt\nzweite Zeile\n", 1)

	_, err := clientState(t, "vm")
	expectState(t, err, StateClientFailed, "Heimatverzeichnis unbekannt", "kephalaion node client --json --format 1 --no-probe --hub vm")
	if strings.Contains(err.Message, "kephalaion upgrade") || strings.Contains(err.Message, "zweite Zeile") {
		t.Fatalf("Exit 1: %s", err.Message)
	}
}

func TestClientExit2NeedsUpgrade(t *testing.T) {
	fake := newFakeClient(t)
	fake.set(t, "", "node client: --format 1: dieses Binary trägt die Fassungen 2\n", 2)

	_, err := clientState(t, "vm")
	expectState(t, err, StateNoKephalaion, "kephalaion upgrade", "dieses Binary trägt die Fassungen 2")
}

// Ein Binary ohne node client (vor v0.4.0) endet ebenfalls mit Exit 2. Eine
// andere Fassung bei Exit 0 nennt wie Exit 2 die erste Zeile von stderr.
func TestClientWrongFormat(t *testing.T) {
	fake := newFakeClient(t)
	tokens := tokensWith(t)
	info := infoFor(tokens, hubWith(tokens, "vm", "kamran-wsl", ChoiceOnly))
	info.Format = 2
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	fake.set(t, string(data)+"\n", "node client: Fassung 1 veraltet, gebe Fassung 2 aus\nzweite Zeile\n", 0)

	_, failure := clientState(t, "vm")
	expectState(t, failure, StateNoKephalaion, "kephalaion upgrade", "Format 2 statt 1",
		"node client: Fassung 1 veraltet, gebe Fassung 2 aus")
	if strings.Contains(failure.Message, "zweite Zeile") {
		t.Fatalf("mehr als die erste Zeile von stderr: %s", failure.Message)
	}
}

// Keine JSON-Ausgabe bei Exit 0: kephalaion upgrade und die erste Zeile von
// stderr, wie bei Exit 2.
func TestClientNoJSON(t *testing.T) {
	fake := newFakeClient(t)
	fake.set(t, "Hub vm: kamran-wsl (einzige Token-Datei)\n", "\nunbekannte Option --json, Ausgabe als Text\nzweite Zeile\n", 0)

	_, err := clientState(t, "vm")
	expectState(t, err, StateNoKephalaion, "kephalaion upgrade", "keine JSON-Ausgabe",
		"unbekannte Option --json, Ausgabe als Text")
	if strings.Contains(err.Message, "zweite Zeile") {
		t.Fatalf("mehr als die erste Zeile von stderr: %s", err.Message)
	}
}

func TestClientCrashAndTimeout(t *testing.T) {
	fake := newFakeClient(t)
	fake.set(t, "", "panic: kaputt\n", 3)
	_, err := clientState(t, "vm")
	expectState(t, err, StateClientFailed, "Exit 3", "panic: kaputt")

	original := ClientTimeout
	ClientTimeout = 200 * time.Millisecond
	t.Cleanup(func() { ClientTimeout = original })
	writeFile(t, fake.path, "#!/bin/sh\nexec sleep 5\n")
	start := time.Now()
	_, err = clientState(t, "vm")
	expectState(t, err, StateClientFailed, "nicht innerhalb von 200ms")
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("Zeitgrenze nicht eingehalten: %s", elapsed)
	}
}

func TestClientTokenFileElsewhere(t *testing.T) {
	fake := newFakeClient(t)
	tokens := tokensWith(t)
	hub := hubWith(tokens, "vm", "kamran-wsl", ChoiceOnly)
	elsewhere := filepath.Join(t.TempDir(), "kamran-wsl.token")
	writeFile(t, elsewhere, testToken+"\n")
	hub.TokenFile = &elsewhere
	fake.setInfo(t, infoFor(tokens, hub))

	_, err := clientState(t, "vm")
	expectState(t, err, StateTokenMissing, elsewhere, filepath.Join(tokens, "vm", "kamran-wsl.token"),
		"kephalaion node account setup vm kamran-wsl")

	// Ohne Token-Datei, etwa nach einer Überschreibung ohne Datei.
	hub.TokenFile = nil
	hub.Choice = ChoiceOverride
	fake.setInfo(t, infoFor(tokens, hub))
	_, err = clientState(t, "vm")
	expectState(t, err, StateTokenMissing, "fehlt die Token-Datei", "kephalaion node account setup vm kamran-wsl")
}

func TestClientNodeURLNotLoopback(t *testing.T) {
	fake := newFakeClient(t)
	tokens := tokensWith(t)
	info := infoFor(tokens, hubWith(tokens, "vm", "kamran-wsl", ChoiceOnly))
	info.Node.URL = "http://10.0.0.5:7433/mcp"
	fake.setInfo(t, info)

	_, err := clientState(t, "vm")
	expectState(t, err, StateNodeAddress, `"http://10.0.0.5:7433/mcp"`, "keine Loopback-Adresse", "listen auf eine Loopback-Adresse")
}

func TestLoopbackURL(t *testing.T) {
	cases := map[string]string{
		"http://127.0.0.1:7433/mcp": "http://127.0.0.1:7433/mcp",
		"http://localhost:7433/mcp": "http://127.0.0.1:7433/mcp",
		"http://[::1]:7433/mcp":     "http://[::1]:7433/mcp",
		"http://127.0.0.1:7433":     "http://127.0.0.1:7433/mcp",
	}
	for raw, want := range cases {
		if got, problem := loopbackURL(raw); got != want || problem != "" {
			t.Errorf("%s: %q %q", raw, got, problem)
		}
	}
	for _, raw := range []string{"", "https://127.0.0.1:7433/mcp", "http://0.0.0.0:7433/mcp", "http://example.org:7433/mcp",
		"http://user:pw@127.0.0.1:7433/mcp", "http://127.0.0.1:7433/mcp?x=1", "http://127.0.0.1/mcp", "http://127.0.0.1:99999/mcp"} {
		if got, problem := loopbackURL(raw); got != "" || problem == "" {
			t.Errorf("%s angenommen: %q", raw, got)
		}
	}
}

// Kein Token in einer Meldung — auch nicht, wenn stderr, config.problem,
// notes oder die Adresse des Nodes eines enthalten.
func TestClientMessagesCarryNoToken(t *testing.T) {
	fake := newFakeClient(t)
	fake.set(t, "", "Fehler mit "+testToken+"\n", 1)
	if _, err := clientState(t, "vm"); err == nil || !strings.Contains(err.Message, "keph_***") {
		t.Fatalf("stderr nicht bereinigt: %+v", err)
	}
	fake.set(t, "", testToken+"\n", 2)
	clientState(t, "vm")

	tokens := tokensWith(t)
	info := infoFor(tokens, ClientHub{Alias: "vm", Choice: ChoiceMissing, Candidates: []string{testToken, "kamran-wsl", "zweit"},
		Pending: []string{testToken}, Notes: []string{testToken}})
	info.Config.Problem = ptr("Zeile 3: " + testToken)
	fake.setInfo(t, info)
	clientState(t, "vm")

	info.Node.URL = "http://" + testToken + ":7433/mcp"
	fake.setInfo(t, info)
	_, err := clientState(t, "vm")
	expectState(t, err, StateNodeAddress, "(unzulässiger Wert)")

	hub := hubWith(tokens, "vm", "kamran-wsl", ChoiceOnly)
	hub.TokenFile = ptr(filepath.Join(tokens, testToken))
	info = infoFor(tokens, hub)
	fake.setInfo(t, info)
	clientState(t, "vm")
}

// Ein Hub außerhalb der Namensregel wird kein Argument.
func TestClientRejectsHubOutsideTheRule(t *testing.T) {
	fake := newFakeClient(t)
	_, err := clientState(t, "--config")
	expectState(t, err, StateInvalidEntry)
	if _, statErr := os.Stat(filepath.Join(fake.dir, "args")); statErr == nil {
		t.Fatal("Binary trotzdem aufgerufen")
	}
}

// readTokenFile nimmt nur eine reguläre Datei, keinen Symlink, und nur die
// Form eines Tokens.
func TestAccessReadTokenChecks(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "vm", "a.token")
	writeFile(t, file, testToken+"\n")
	link := filepath.Join(dir, "vm", "b.token")
	if err := os.Symlink(file, link); err != nil {
		t.Fatal(err)
	}
	if _, err := (Access{tokenFile: link}).readToken(); err == nil {
		t.Fatal("Symlink gelesen")
	}
	writeFile(t, file, "kein Token\n")
	if _, err := (Access{tokenFile: file}).readToken(); err == nil || strings.Contains(err.Error(), "kein Token") {
		t.Fatalf("falsche Form: %v", err)
	}
}
