package project

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
)

// Der Block kephalaion steht in jeder Kontextausgabe und lässt sie nie
// scheitern — auch nicht bei unlesbarer Datei oder fehlendem Node.
func TestContextForDirTraegtKephalaion(t *testing.T) {
	isolateKephalaionConfig(t)
	root := newContextProject(t)

	built, err := ContextForDir(root)
	if err != nil {
		t.Fatalf("ContextForDir: %v", err)
	}
	if built.Kephalaion == nil || built.Kephalaion.State != kephalaion.StateNotConfigured || built.Kephalaion.Configured {
		t.Fatalf("ohne Eintrag: %+v", built.Kephalaion)
	}

	local := filepath.Join(LocalDir(root), kephalaion.LocalFileName)
	write(t, local, "kephalaion:\n  hub: vm\n  collection: k-playbook\n  account: kamran-wsl\n")
	built, err = ContextForDir(root)
	if err != nil {
		t.Fatalf("ContextForDir: %v", err)
	}
	if built.Kephalaion.State != kephalaion.StateNoKephalaion || built.Kephalaion.Address != "vm:k-playbook" {
		t.Fatalf("ohne Kephalaion: %+v", built.Kephalaion)
	}

	write(t, local, "kephalaion:\n  hub: [kaputt\n")
	built, err = ContextForDir(root)
	if err != nil {
		t.Fatalf("unlesbare Datei bricht den Kontext ab: %v", err)
	}
	if built.Kephalaion.State != kephalaion.StateInvalidEntry {
		t.Fatalf("unlesbar: %+v", built.Kephalaion)
	}
}

// BuildContext fragt den Node nicht; das Feld bleibt leer.
func TestBuildContextFragtKephalaionNicht(t *testing.T) {
	called := false
	original := kephalaionCheck
	kephalaionCheck = func(string) kephalaion.Status { called = true; return kephalaion.Status{} }
	t.Cleanup(func() { kephalaionCheck = original })

	built, err := BuildContext(newContextProject(t))
	if err != nil {
		t.Fatal(err)
	}
	if called || built.Kephalaion != nil {
		t.Fatal("BuildContext hat die Anbindung geprüft")
	}
}

// Eine Zeile in K-PLAYBOOK.local.yaml, die nur aus einem Token besteht,
// erscheint nicht in der Kontextausgabe — höchstens Zeilennummer und Art des
// Fehlers.
func TestContextForDirZitiertKeinenDateiinhalt(t *testing.T) {
	const token = "keph_CONTEXTLEAKCANARYabcdefghijklmnopqrstuvw1234"
	isolateKephalaionConfig(t)
	root := newContextProject(t)
	write(t, filepath.Join(LocalDir(root), kephalaion.LocalFileName),
		"kephalaion:\n  hub: vm\n  collection: k-playbook\n"+token+"\n  account: kamran-wsl\n")

	built, err := ContextForDir(root)
	if err != nil {
		t.Fatalf("ContextForDir: %v", err)
	}
	data, err := json.Marshal(built)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "CONTEXTLEAKCANARY") {
		t.Fatalf("Dateiinhalt in der Kontextausgabe: %s", built.Kephalaion.Message)
	}
	if built.Kephalaion.State != kephalaion.StateInvalidEntry || !strings.Contains(built.Kephalaion.Message, "Zeile 4") {
		t.Fatalf("Zustand: %+v", built.Kephalaion)
	}
}

// isolateKephalaionConfig lenkt die Suche nach der config von Kephalaion in
// temporäre Verzeichnisse: XDG_CONFIG_HOME, KEPHALAION_CONFIG (leer) und den
// Ort der globalen config. Zurück kommen das Verzeichnis des Users und der
// Ort der globalen config; angelegt ist keine.
func isolateKephalaionConfig(t *testing.T) (userDir, systemPath string) {
	t.Helper()
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv(kephalaion.EnvConfigVar, "")
	original := kephalaion.SystemConfigPath
	kephalaion.SystemConfigPath = filepath.Join(t.TempDir(), "config.yaml")
	t.Cleanup(func() { kephalaion.SystemConfigPath = original })
	return filepath.Join(xdg, "kephalaion"), kephalaion.SystemConfigPath
}

// Mit nur globaler config (wie bei einer globalen Installation) kommt der
// Node aus /etc/kephalaion/config.yaml, das Token aus dem Verzeichnis des
// Users — KephalaionStatus meldet ok statt „nicht eingerichtet“.
func TestKephalaionStatusMitNurGlobalerConfig(t *testing.T) {
	const token = "keph_GLOBALLEAKCANARYabcdefghijklmnopqrstuvw1234"
	userDir, systemPath := isolateKephalaionConfig(t)

	var gotAccount, gotToken string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		var msg struct {
			ID     *int   `json:"id"`
			Method string `json:"method"`
		}
		_ = json.Unmarshal(data, &msg)
		switch msg.Method {
		case "initialize":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":{"protocolVersion":"2025-06-18","capabilities":{}}}`, *msg.ID)
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
		case "tools/call":
			gotAccount, gotToken = r.Header.Get("X-Keph-Account-vm"), r.Header.Get("X-Keph-Token-vm")
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":{"content":[{"type":"text","text":"x"}],"structuredContent":`+
				`{"version":"v0.3.1","hubs":[{"hub":"vm","login":"ok","node":"vm-node","account":"kamran-vm","user":"kamran",`+
				`"sync":{"revision":1},"collections":[{"address":"vm:k-playbook","collection":"k-playbook","rights":["read","write"]}]}],`+
				`"unknown_hubs":[]}}}`, *msg.ID)
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	t.Cleanup(server.Close)

	if err := os.MkdirAll(filepath.Join(userDir, "tokens", "vm"), 0o700); err != nil {
		t.Fatal(err)
	}
	write(t, systemPath, "node:\n  listen: "+strings.TrimPrefix(server.URL, "http://")+"\n")
	write(t, filepath.Join(userDir, "tokens", "vm", "kamran-vm.token"), token+"\n")
	root := newContextProject(t)
	write(t, filepath.Join(LocalDir(root), kephalaion.LocalFileName),
		"kephalaion:\n  hub: vm\n  collection: k-playbook\n  account: kamran-vm\n")

	status := KephalaionStatus(root)
	if status.State != kephalaion.StateOK || status.User != "kamran" {
		t.Fatalf("nur globale config: %+v", status)
	}
	if gotAccount != "kamran-vm" || gotToken != token {
		t.Fatalf("Node hat Account %q und ein anderes Token bekommen", gotAccount)
	}
	if data, _ := json.Marshal(status); strings.Contains(string(data), "GLOBALLEAKCANARY") {
		t.Fatalf("Token im Ergebnis: %s", data)
	}
}
