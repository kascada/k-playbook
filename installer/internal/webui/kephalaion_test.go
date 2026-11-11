package webui

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
	"github.com/kascada/k-playbook/installer/internal/kephalaion/kephtest"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// kephToken hat die Form eines Kephalaion-Tokens und ist leicht
// wiederzufinden; es darf in keiner Antwort der Endpunkte stehen.
const kephToken = "keph_WEBUILEAKCANARYabcdefghijklmnopqrstuvw12345"

// kephFakeNode beantwortet initialize und whoami wie ein Node mit dem Hub vm,
// an dem kamran-wsl die Collections k-playbook und omni lesen darf — aber nur
// mit dem richtigen Token. Jede Antwort spiegelt das empfangene Token in
// unknown_hubs zurück, damit ein Durchreichen auffiele.
func kephFakeNode(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		var msg struct {
			ID     *int   `json:"id"`
			Method string `json:"method"`
		}
		_ = json.Unmarshal(data, &msg)
		w.Header().Set("Content-Type", "application/json")
		switch msg.Method {
		case "initialize":
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":{"protocolVersion":"2025-06-18","capabilities":{},"serverInfo":{"name":"kephalaion","version":"v0.2.4"}}}`, *msg.ID)
		case "tools/call":
			hub := `{"hub":"vm","login":"invalid","node":"n","sync":{"revision":1}}`
			if r.Header.Get("X-Keph-Account-vm") == "kamran-wsl" && r.Header.Get("X-Keph-Token-vm") == kephToken {
				hub = `{"hub":"vm","login":"ok","node":"n","account":"kamran-wsl","user":"kamran","sync":{"revision":1},"collections":[` +
					`{"collection":"k-playbook","address":"vm:k-playbook","rights":["read","write"],"dirs":[]},` +
					`{"collection":"omni","address":"vm:omni","rights":["read"],"dirs":[]}]}`
			}
			echo, _ := json.Marshal(r.Header.Get("X-Keph-Token-vm"))
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":{"content":[{"type":"text","text":"x"}],"structuredContent":{"version":"v0.2.4","hubs":[%s],"unknown_hubs":[%s]}}}`, *msg.ID, hub, echo)
		default:
			w.WriteHeader(http.StatusAccepted)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

// kephProject legt ein Projekt (eigenes Git-Repo) an, ein Token-Verzeichnis
// mit der Token-Datei vm/kamran-wsl und ein Ersatz-Binary, dessen Auskunft
// den gefälschten Node und die Hubs vm (einzige Token-Datei) und anders (kein
// Token) nennt.
func kephProject(t *testing.T) (root string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git fehlt")
	}
	root = t.TempDir()
	if err := project.CreateConfig(root, "."); err != nil {
		t.Fatalf("Konfiguration anlegen: %v", err)
	}
	if err := os.MkdirAll(project.LocalDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	chdir(t, root)

	tokensDir := kephtest.Token(t, filepath.Join(t.TempDir(), "kephalaion", "tokens"), "vm", "kamran-wsl", kephToken)
	node := kephFakeNode(t)
	anders := kephalaion.ClientHub{Alias: "anders", Choice: kephalaion.ChoiceMissing, Candidates: []string{}, Pending: []string{}, Notes: []string{}}
	kephtest.NewFakeClient(t).SetInfo(t, kephtest.Info(node.URL+"/mcp", tokensDir, anders, kephtest.Hub(tokensDir, "vm", "kamran-wsl")))
	t.Cleanup(dropKephalaionCache)
	dropKephalaionCache()
	return root
}

// kephCall ruft einen Endpunkt über routes() und prüft, dass das Token in
// der Antwort nicht vorkommt.
func kephCall(t *testing.T, method, path, body string, header map[string]string) (int, string) {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	for k, v := range header {
		request.Header.Set(k, v)
	}
	recorder := httptest.NewRecorder()
	routes(&serverState{}).ServeHTTP(recorder, request)
	text := recorder.Body.String()
	if strings.Contains(text, kephToken) || strings.Contains(text, "WEBUILEAKCANARY") {
		t.Fatalf("%s %s: Token in der Antwort: %s", method, path, text)
	}
	return recorder.Code, text
}

const kephGood = `{"hub":"vm","collection":"k-playbook"}`

// configText liest K-PLAYBOOK.yaml des Projekts.
func configText(t *testing.T, root string) string {
	t.Helper()
	data, err := os.ReadFile(project.ConfigPath(root))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// appendKephalaionBlock hängt einen Block an K-PLAYBOOK.yaml an.
func appendKephalaionBlock(t *testing.T, root, block string) {
	t.Helper()
	if err := os.WriteFile(project.ConfigPath(root), []byte(configText(t, root)+"\n"+block), 0o644); err != nil {
		t.Fatal(err)
	}
	dropKephalaionCache()
}

func TestKephalaionSaveWritesOnlyAfterPassedTest(t *testing.T) {
	root := kephProject(t)
	before := configText(t, root)

	code, body := kephCall(t, http.MethodGet, "/api/kephalaion", "", nil)
	if code != 200 || !strings.Contains(body, `"state":"not-configured"`) || !strings.Contains(body, `"configFile":`) {
		t.Fatalf("vorher: %d %s", code, body)
	}

	code, body = kephCall(t, http.MethodGet, "/api/kephalaion/candidates", "", nil)
	if code != 200 || !strings.Contains(body, `"vm:k-playbook"`) {
		t.Fatalf("Kandidaten: %d %s", code, body)
	}
	var candidates kephalaionCandidatesResponse
	if err := json.Unmarshal([]byte(body), &candidates); err != nil {
		t.Fatal(err)
	}
	// config, Node und Token-Verzeichnis kommen aus der Auskunft.
	if candidates.ConfigSource != "user" || filepath.Base(candidates.ConfigPath) != "config.yaml" ||
		filepath.Base(candidates.TokensDir) != "tokens" || !strings.HasSuffix(candidates.NodeURL, "/mcp") || candidates.ClientProblem != "" {
		t.Fatalf("Auskunft: %+v", candidates)
	}
	if len(candidates.Hubs) != 2 || candidates.Hubs[0].Hub != "anders" || candidates.Hubs[0].State != kephalaion.StateTokenMissing ||
		candidates.Hubs[1].Account != "kamran-wsl" || candidates.Hubs[1].Choice != kephalaion.ChoiceOnly {
		t.Fatalf("Hubs: %+v", candidates.Hubs)
	}
	// Das Projektverzeichnis heißt nicht k-playbook; vorgewählt ist dann
	// keine Collection, weil es zwei gibt.
	if candidates.Preselect.Hub != "vm" || candidates.Preselect.Collection != "" {
		t.Fatalf("Vorauswahl: %+v", candidates.Preselect)
	}

	code, body = kephCall(t, http.MethodPost, "/api/kephalaion/test", kephGood, nil)
	if code != 200 || !strings.Contains(body, `"state":"ok"`) || !strings.Contains(body, `"account":"kamran-wsl"`) {
		t.Fatalf("Test: %d %s", code, body)
	}
	if configText(t, root) != before {
		t.Fatal("der Test hat geschrieben")
	}

	code, body = kephCall(t, http.MethodPost, "/api/kephalaion/save", `{"hub":"vm","collection":"fremd"}`, nil)
	if code != http.StatusConflict || !strings.Contains(body, `"state":"collection-unreadable"`) {
		t.Fatalf("falsche Collection: %d %s", code, body)
	}
	if configText(t, root) != before {
		t.Fatal("trotz gescheitertem Test geschrieben")
	}

	// Ein Account in der Anfrage wird nicht gelesen und nirgends geschrieben.
	code, body = kephCall(t, http.MethodPost, "/api/kephalaion/save", `{"hub":"vm","collection":"k-playbook","account":"bob"}`, nil)
	if code != 200 || !strings.Contains(body, `"saved":true`) || !strings.Contains(body, project.KephalaionCommitHint) {
		t.Fatalf("Eintragen: %d %s", code, body)
	}
	after := configText(t, root)
	if !strings.HasPrefix(after, before) || strings.Contains(after, "account:") || strings.Contains(after, "bob") {
		t.Fatalf("eingetragen:\n%s", after)
	}
	if config, _ := project.ReadConfig(root); config.Kephalaion.Binding().Address() != "vm:k-playbook" {
		t.Fatalf("gelesen: %+v", config.Kephalaion)
	}
	if _, err := os.Stat(filepath.Join(project.LocalDir(root), kephalaion.LegacyFileName)); !os.IsNotExist(err) {
		t.Fatal("alte lokale Datei angelegt")
	}

	// Der Zwischenspeicher ist verworfen: die Anzeige zeigt sofort ok, mit
	// der Herkunft des Accounts.
	code, body = kephCall(t, http.MethodGet, "/api/kephalaion", "", nil)
	if code != 200 || !strings.Contains(body, `"state":"ok"`) || !strings.Contains(body, `"choice":"only"`) {
		t.Fatalf("nachher: %d %s", code, body)
	}

	// Ein vorhandener Block: nur hub und collection ändern sich.
	code, body = kephCall(t, http.MethodPost, "/api/kephalaion/save", `{"hub":"vm","collection":"omni"}`, nil)
	if code != 200 || !strings.Contains(body, `"changed":true`) {
		t.Fatalf("Ändern: %d %s", code, body)
	}
	if got := configText(t, root); got != strings.Replace(after, "collection: k-playbook", "collection: omni", 1) {
		t.Fatalf("geändert:\n%s", got)
	}
	code, body = kephCall(t, http.MethodPost, "/api/kephalaion/save", `{"hub":"vm","collection":"omni"}`, nil)
	if code != 200 || strings.Contains(body, `"changed":true`) || !strings.Contains(body, "nichts geändert") {
		t.Fatalf("unverändert: %d %s", code, body)
	}
}

// Andere Blöcke und Kommentare in K-PLAYBOOK.yaml bleiben beim Eintragen
// byte-gleich.
func TestKephalaionSaveKeepsOtherBlocksAndComments(t *testing.T) {
	root := kephProject(t)
	content := "# Kopf\nschema_version: 3\n\nproject:\n  # Ort\n  repo_root: .\n  vcs: git\n\n# Anbindung\nkephalaion:\n" +
		"  # eigener Kommentar\n  hub: alt\n  collection: alt\n\n# danach\ndocs:\n  exclude:\n    - x/\n"
	if err := os.WriteFile(project.ConfigPath(root), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	code, body := kephCall(t, http.MethodPost, "/api/kephalaion/save", kephGood, nil)
	if code != 200 || !strings.Contains(body, `"saved":true`) {
		t.Fatalf("%d %s", code, body)
	}
	want := strings.Replace(strings.Replace(content, "hub: alt", "hub: vm", 1), "collection: alt", "collection: k-playbook", 1)
	if got := configText(t, root); got != want {
		t.Fatalf("Ergebnis:\n%s", got)
	}
}

func TestKephalaionRejectsHubsWithoutAccount(t *testing.T) {
	root := kephProject(t)
	before := configText(t, root)
	code, text := kephCall(t, http.MethodPost, "/api/kephalaion/save", `{"hub":"anders","collection":"k-playbook"}`, nil)
	if code != http.StatusConflict || !strings.Contains(text, `"state":"token-missing"`) || !strings.Contains(text, "kephalaion node account setup anders") {
		t.Fatalf("%d %s", code, text)
	}
	for _, body := range []string{
		`{"hub":"../vm","collection":"k-playbook"}`,
		`{"hub":"--config","collection":"k-playbook"}`,
		`{"hub":"vm","collection":"a/b"}`,
		`{"hub":"vm","collection":"` + kephToken + `"}`,
		`kein json`,
	} {
		for _, path := range []string{"/api/kephalaion/test", "/api/kephalaion/save"} {
			code, text := kephCall(t, http.MethodPost, path, body, nil)
			if code != http.StatusBadRequest {
				t.Fatalf("%s %s: %d %s", path, body, code, text)
			}
		}
	}
	if configText(t, root) != before {
		t.Fatal("geschrieben")
	}
}

func TestKephalaionPostsRequireSameOrigin(t *testing.T) {
	kephProject(t)
	for _, path := range []string{"/api/kephalaion/test", "/api/kephalaion/save"} {
		code, _ := kephCall(t, http.MethodPost, path, kephGood, map[string]string{"Origin": "http://evil.example"})
		if code != http.StatusForbidden {
			t.Fatalf("%s: %d", path, code)
		}
	}
}

func TestKephalaionGetIgnoresRequestValues(t *testing.T) {
	root := kephProject(t)
	// Werte in der Anfrage ändern nichts: geprüft wird nur, was in der Datei
	// steht.
	appendKephalaionBlock(t, root, "kephalaion:\n  hub: vm\n  collection: omni\n")
	code, body := kephCall(t, http.MethodGet, "/api/kephalaion?hub=x&account=bob&collection=fremd", "", nil)
	if code != 200 || !strings.Contains(body, `"address":"vm:omni"`) || !strings.Contains(body, `"state":"ok"`) {
		t.Fatalf("%d %s", code, body)
	}
}

// Ein Token im Block kephalaion: von K-PLAYBOOK.yaml — als nackte Zeile oder
// als Wert — erscheint weder in /api/kephalaion noch in /api/context
// (kephCall prüft jede Antwort auf das Token).
func TestKephalaionResponsesDoNotQuoteFileContent(t *testing.T) {
	root := kephProject(t)
	appendKephalaionBlock(t, root, "kephalaion:\n  hub: vm\n"+kephToken+"\n  collection: "+kephToken+"\n")
	code, body := kephCall(t, http.MethodGet, "/api/kephalaion", "", nil)
	if code != 200 || !strings.Contains(body, `"state":"invalid-entry"`) {
		t.Fatalf("/api/kephalaion: %d %s", code, body)
	}
	code, body = kephCall(t, http.MethodGet, "/api/context", "", nil)
	if code != 200 || !strings.Contains(body, `"state":"invalid-entry"`) {
		t.Fatalf("/api/context: %d %s", code, body)
	}
}

// Ohne Auskunft — Binary fehlt oder ist zu alt — nennen die Kandidaten den
// Zustand samt Abhilfe statt Hubs.
func TestKephalaionCandidatesWithoutClientInfo(t *testing.T) {
	kephProject(t)
	kephtest.NoBinary(t)
	code, body := kephCall(t, http.MethodGet, "/api/kephalaion/candidates", "", nil)
	var response kephalaionCandidatesResponse
	if code != 200 || json.Unmarshal([]byte(body), &response) != nil {
		t.Fatalf("%d %s", code, body)
	}
	if response.ClientState != kephalaion.StateNoKephalaion || !strings.Contains(response.ClientProblem, kephalaion.InstallCommand) ||
		len(response.Hubs) != 0 {
		t.Fatalf("ohne Binary: %+v", response)
	}

	fake := kephtest.NewFakeClient(t)
	fake.Set(t, "", "node client: unbekanntes Kommando\n", 2)
	code, body = kephCall(t, http.MethodGet, "/api/kephalaion/candidates", "", nil)
	if code != 200 || !strings.Contains(body, "kephalaion upgrade") {
		t.Fatalf("zu alt: %d %s", code, body)
	}
}
