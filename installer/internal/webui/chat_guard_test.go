package webui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func setContainer(t *testing.T, inside bool) {
	t.Helper()
	before := chatInContainer
	chatInContainer = func() bool { return inside }
	t.Cleanup(func() { chatInContainer = before })
}

func setProcRoot(t *testing.T, root string) {
	t.Helper()
	before := procRoot
	procRoot = root
	t.Cleanup(func() { procRoot = before })
}

// fakeProc baut ein proc-Dateisystem nach: ein lauschender Socket auf port mit
// der Inode socketInode, und ein Prozess 42, dessen fd 3 auf ownedInode zeigt.
func fakeProc(t *testing.T, port int, socketInode string, ownedInode string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "net"), 0o755); err != nil {
		t.Fatal(err)
	}
	tcp := "  sl  local_address                         remote_address                        st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode\n" +
		fmt.Sprintf("   0: 0100007F:%04X 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 %s 1 0000000000000000 100 0 0 10 0\n", port, socketInode)
	if err := os.WriteFile(filepath.Join(root, "net", "tcp"), []byte(tcp), 0o644); err != nil {
		t.Fatal(err)
	}
	fdDir := filepath.Join(root, "42", "fd")
	if err := os.MkdirAll(fdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("socket:["+ownedInode+"]", filepath.Join(fdDir, "3")); err != nil {
		t.Fatal(err)
	}
	return root
}

// Im Container zählt nur ein Dienst, dessen Socket einem Prozess des
// Containers gehört. Eine Loopback-Adresse allein reicht nicht — mit
// --network host wäre sie der Host.
func TestContainerNutztNurEigenenDienst(t *testing.T) {
	local := openCodeTarget{baseURL: "http://127.0.0.1:4096"}

	t.Run("außerhalb eines Containers keine Prüfung", func(t *testing.T) {
		setContainer(t, false)
		setProcRoot(t, t.TempDir())
		if message := local.containerGuard(); message != "" {
			t.Errorf("Meldung = %q, erwartet keine", message)
		}
	})

	tests := []struct {
		name   string
		target openCodeTarget
		proc   func(t *testing.T) string
		want   string
	}{
		{"eigener Prozess besitzt den Port", local, func(t *testing.T) string { return fakeProc(t, 4096, "555", "555") }, ""},
		{"localhost zählt als Loopback", openCodeTarget{baseURL: "http://localhost:4096"}, func(t *testing.T) string { return fakeProc(t, 4096, "555", "555") }, ""},
		{"Socket ohne Prozess im Container", local, func(t *testing.T) string { return fakeProc(t, 4096, "555", "777") }, "kein Prozess dieses Containers"},
		{"anderer Port", local, func(t *testing.T) string { return fakeProc(t, 4097, "555", "555") }, "kein Prozess dieses Containers"},
		{"Adresse nach außen", openCodeTarget{baseURL: "http://host.docker.internal:4096"}, func(t *testing.T) string { return fakeProc(t, 4096, "555", "555") }, "zeigt nach außen"},
		{"proc nicht lesbar", local, func(t *testing.T) string { return t.TempDir() }, "ließ sich nicht prüfen"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setContainer(t, true)
			setProcRoot(t, test.proc(t))
			message := test.target.containerGuard()
			if test.want == "" && message != "" {
				t.Errorf("Meldung = %q, erwartet keine", message)
			}
			if test.want != "" && !strings.Contains(message, test.want) {
				t.Errorf("Meldung = %q, erwartet %q", message, test.want)
			}
		})
	}
}

// Ein gesperrter Dienst wird weder für Anfragen noch für den Ereignisstrom
// benutzt, und der Status sagt, warum und was zu tun ist.
func TestChatSperrtFremdenDienstImContainer(t *testing.T) {
	chatProject(t)
	called := false
	fakeOpenCode(t, func(w http.ResponseWriter, r *http.Request) { called = true })
	setContainer(t, true)
	setProcRoot(t, fakeProc(t, 1, "555", "777"))

	for _, path := range []string{"/api/chat/sessions", "/api/chat/sessions/ses_abc", "/api/chat/events"} {
		if recorder := serveChat(t, http.MethodGet, path, ""); recorder.Code != http.StatusForbidden {
			t.Errorf("%s: Status = %d, erwartet 403", path, recorder.Code)
		}
	}
	if recorder := serveChat(t, http.MethodPost, "/api/chat/sessions/ses_abc/prompt", `{"text":"Hallo"}`); recorder.Code != http.StatusForbidden {
		t.Errorf("prompt: Status = %d, erwartet 403", recorder.Code)
	}
	if called {
		t.Error("eine Anfrage hat den gesperrten Dienst erreicht")
	}

	var status chatStatusResponse
	if err := json.Unmarshal(serveChat(t, http.MethodGet, "/api/chat/status", "").Body.Bytes(), &status); err != nil {
		t.Fatalf("Status nicht lesbar: %v", err)
	}
	if !status.Blocked || status.Available || !status.Container || status.Hint == nil || !strings.Contains(status.Message, "Container") {
		t.Errorf("Status = %+v, erwartet gesperrt mit Meldung und Hinweis", status)
	}
}

// Gegen das echte /proc: der Stellvertreter lauscht in diesem Prozess und gilt
// deshalb auch im Container als eigener Dienst.
func TestChatErlaubtEigenenDienstImContainer(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("braucht /proc")
	}
	chatProject(t)
	fakeOpenCode(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[]`)
	})
	setContainer(t, true)

	if recorder := serveChat(t, http.MethodGet, "/api/chat/sessions", ""); recorder.Code != http.StatusOK {
		t.Errorf("Status = %d, erwartet 200: %s", recorder.Code, recorder.Body.String())
	}
}

// Ohne erreichbaren Dienst sagt der Status, ob OpenCode fehlt oder nur nicht
// läuft, und nennt die Befehle dazu.
func TestChatStatusNenntInstallationsweg(t *testing.T) {
	chatProject(t)
	server := fakeOpenCode(t, func(w http.ResponseWriter, r *http.Request) {})
	server.Close()
	t.Setenv("HOME", t.TempDir())
	bin := t.TempDir()
	t.Setenv("PATH", bin)

	readStatus := func() chatStatusResponse {
		var status chatStatusResponse
		if err := json.Unmarshal(serveChat(t, http.MethodGet, "/api/chat/status", "").Body.Bytes(), &status); err != nil {
			t.Fatalf("Status nicht lesbar: %v", err)
		}
		return status
	}

	status := readStatus()
	if status.Hint == nil || !strings.Contains(status.Hint.Text, "nicht installiert") || !strings.Contains(strings.Join(status.Hint.Commands, "\n"), "opencode.ai/install") {
		t.Errorf("Hinweis ohne OpenCode = %+v", status.Hint)
	}

	if err := os.WriteFile(filepath.Join(bin, "opencode"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	status = readStatus()
	if status.Hint == nil || !strings.Contains(status.Hint.Text, "installiert (") || !strings.Contains(strings.Join(status.Hint.Commands, "\n"), "opencode serve --port") {
		t.Errorf("Hinweis mit OpenCode = %+v", status.Hint)
	}
}
