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

// kephProject legt ein Projekt (eigenes Git-Repo) und eine
// Kephalaion-Einrichtung mit der Token-Datei vm/kamran-wsl an und lenkt die
// Endpunkte auf den gefälschten Node.
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

	configDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("node:\n  db: x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tokens := filepath.Join(configDir, "tokens", "vm")
	if err := os.MkdirAll(tokens, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tokens, "kamran-wsl.token"), []byte(kephToken+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	node := kephFakeNode(t)
	original := kephalaionEnv
	kephalaionEnv = func() kephalaion.Env {
		env := kephalaion.LoadEnv(configDir)
		env.Endpoint = node.URL + "/mcp"
		return env
	}
	t.Cleanup(func() { kephalaionEnv = original; dropKephalaionCache() })
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

const kephGood = `{"hub":"vm","collection":"k-playbook","account":"kamran-wsl"}`

func TestKephalaionSaveWritesOnlyAfterPassedTest(t *testing.T) {
	root := kephProject(t)
	local := filepath.Join(project.LocalDir(root), kephalaion.LocalFileName)

	code, body := kephCall(t, http.MethodGet, "/api/kephalaion", "", nil)
	if code != 200 || !strings.Contains(body, `"state":"not-configured"`) {
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
	// Das Projektverzeichnis heißt nicht k-playbook; vorgewählt ist dann
	// keine Collection, weil es zwei gibt.
	if candidates.Preselect.Hub != "vm" || candidates.Preselect.Account != "kamran-wsl" {
		t.Fatalf("Vorauswahl: %+v", candidates.Preselect)
	}

	code, body = kephCall(t, http.MethodPost, "/api/kephalaion/test", kephGood, nil)
	if code != 200 || !strings.Contains(body, `"state":"ok"`) {
		t.Fatalf("Test: %d %s", code, body)
	}
	if _, err := os.Stat(local); !os.IsNotExist(err) {
		t.Fatal("der Test hat geschrieben")
	}

	code, body = kephCall(t, http.MethodPost, "/api/kephalaion/save",
		`{"hub":"vm","collection":"fremd","account":"kamran-wsl"}`, nil)
	if code != http.StatusConflict || !strings.Contains(body, `"state":"collection-unreadable"`) {
		t.Fatalf("falsche Collection: %d %s", code, body)
	}
	if _, err := os.Stat(local); !os.IsNotExist(err) {
		t.Fatal("trotz gescheitertem Test geschrieben")
	}

	code, body = kephCall(t, http.MethodPost, "/api/kephalaion/save", kephGood, nil)
	if code != 200 || !strings.Contains(body, `"saved":true`) {
		t.Fatalf("Eintragen: %d %s", code, body)
	}
	if b, configured, err := kephalaion.ReadBinding(project.LocalDir(root)); err != nil || !configured || b.Address() != "vm:k-playbook" {
		t.Fatalf("eingetragen: %+v %v %v", b, configured, err)
	}
	gitignore, _ := os.ReadFile(filepath.Join(project.LocalDir(root), ".gitignore"))
	if !strings.Contains(string(gitignore), kephalaion.IgnoreLine) {
		t.Fatalf(".gitignore: %q", gitignore)
	}

	// Der Zwischenspeicher ist verworfen: die Anzeige zeigt sofort ok.
	code, body = kephCall(t, http.MethodGet, "/api/kephalaion", "", nil)
	if code != 200 || !strings.Contains(body, `"state":"ok"`) {
		t.Fatalf("nachher: %d %s", code, body)
	}
}

func TestKephalaionRejectsPairsOutsideCandidates(t *testing.T) {
	root := kephProject(t)
	local := filepath.Join(project.LocalDir(root), kephalaion.LocalFileName)
	for _, body := range []string{
		`{"hub":"vm","collection":"k-playbook","account":"bob"}`,
		`{"hub":"anders","collection":"k-playbook","account":"kamran-wsl"}`,
	} {
		code, text := kephCall(t, http.MethodPost, "/api/kephalaion/save", body, nil)
		if code != http.StatusConflict || !strings.Contains(text, `"state":"token-missing"`) {
			t.Fatalf("%s: %d %s", body, code, text)
		}
	}
	for _, body := range []string{
		`{"hub":"../vm","collection":"k-playbook","account":"kamran-wsl"}`,
		`{"hub":"vm","collection":"k-playbook","account":"../../kamran-wsl"}`,
		`{"hub":"vm","collection":"a/b","account":"kamran-wsl"}`,
		`{"hub":"vm","collection":"k-playbook","account":"` + kephToken + `"}`,
		`kein json`,
	} {
		for _, path := range []string{"/api/kephalaion/test", "/api/kephalaion/save"} {
			code, text := kephCall(t, http.MethodPost, path, body, nil)
			if code != http.StatusBadRequest {
				t.Fatalf("%s %s: %d %s", path, body, code, text)
			}
		}
	}
	if _, err := os.Stat(local); !os.IsNotExist(err) {
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

func TestKephalaionSaveRefusesTrackedFile(t *testing.T) {
	root := kephProject(t)
	local := filepath.Join(project.LocalDir(root), kephalaion.LocalFileName)
	if err := os.WriteFile(local, []byte("# versioniert\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", root, "add", "--", local).CombinedOutput(); err != nil {
		t.Fatalf("git add: %v %s", err, out)
	}
	code, body := kephCall(t, http.MethodPost, "/api/kephalaion/save", kephGood, nil)
	if code != http.StatusConflict || !strings.Contains(body, "git rm --cached") {
		t.Fatalf("%d %s", code, body)
	}
	data, _ := os.ReadFile(local)
	if string(data) != "# versioniert\n" {
		t.Fatalf("verändert: %q", data)
	}
}

func TestKephalaionGetIgnoresRequestValues(t *testing.T) {
	root := kephProject(t)
	// Ein falscher Account in der Anfrage ändert nichts: geprüft wird nur,
	// was in der Datei steht.
	if err := kephalaion.WriteBinding(project.LocalDir(root), kephalaion.Binding{Hub: "vm", Collection: "omni", Account: "kamran-wsl"}); err != nil {
		t.Fatal(err)
	}
	code, body := kephCall(t, http.MethodGet, "/api/kephalaion?hub=x&account=bob&collection=fremd", "", nil)
	if code != 200 || !strings.Contains(body, `"address":"vm:omni"`) || !strings.Contains(body, `"state":"ok"`) {
		t.Fatalf("%d %s", code, body)
	}
}

// Eine Zeile in K-PLAYBOOK.local.yaml, die nur aus einem Token besteht,
// erscheint weder in /api/kephalaion noch in /api/context (kephCall prüft
// jede Antwort auf das Token).
func TestKephalaionResponsesDoNotQuoteFileContent(t *testing.T) {
	root := kephProject(t)
	local := filepath.Join(project.LocalDir(root), kephalaion.LocalFileName)
	if err := os.WriteFile(local, []byte("kephalaion:\n  hub: vm\n"+kephToken+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, body := kephCall(t, http.MethodGet, "/api/kephalaion", "", nil)
	if code != 200 || !strings.Contains(body, `"state":"invalid-entry"`) || !strings.Contains(body, "Zeile 3") {
		t.Fatalf("/api/kephalaion: %d %s", code, body)
	}
	code, body = kephCall(t, http.MethodGet, "/api/context", "", nil)
	if code != 200 || !strings.Contains(body, `"state":"invalid-entry"`) {
		t.Fatalf("/api/context: %d %s", code, body)
	}
}
