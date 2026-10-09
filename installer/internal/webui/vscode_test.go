package webui

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kascada/k-playbook/installer/internal/buildinfo"
	"github.com/kascada/k-playbook/installer/internal/guiproc"
	"github.com/kascada/k-playbook/installer/internal/vscodeext"
)

// GET /api/vscode antwortet mit der Auskunft, aus der die Karte lebt: die
// eingebettete Fassung, die Installationen aus extensions.json und die CLI.
// Rein lesend — ein Aufruf darf nichts installieren.
func TestVSCodeHandler(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("VSCODE_IPC_HOOK_CLI", "")

	// Ohne das echte `code` dieses Rechners: die Remote-CLI ignoriert
	// --extensions-dir und schriebe in die laufende Umgebung.
	vorher := vscodeext.LookPath
	vscodeext.LookPath = func(string) (string, error) { return "", errors.New("nicht im PATH") }
	t.Cleanup(func() { vscodeext.LookPath = vorher })

	dir := filepath.Join(home, ".vscode-server", "extensions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `[{"identifier":{"id":"` + vscodeext.ID + `"},"version":"0.0.1"}]`
	if err := os.WriteFile(filepath.Join(dir, "extensions.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	vscodeHandler(recorder, httptest.NewRequest(http.MethodGet, "/api/vscode", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("Status = %d", recorder.Code)
	}

	var status vscodeext.Status
	if err := json.Unmarshal(recorder.Body.Bytes(), &status); err != nil {
		t.Fatalf("Antwort lesen: %v", err)
	}
	if status.ID != vscodeext.ID {
		t.Errorf("ID = %q", status.ID)
	}
	embedded, _, _ := vscodeext.EmbeddedVersion()
	if status.Embedded != embedded {
		t.Errorf("Embedded = %q, erwartet %q", status.Embedded, embedded)
	}
	if len(status.Installed) != 1 || status.Installed[0].Version != "0.0.1" {
		t.Fatalf("Installed = %+v", status.Installed)
	}
	if !status.NeedsInstall {
		t.Error("eine abweichende Fassung gilt nicht als nachzuziehen")
	}
	if status.CLI != nil {
		t.Errorf("CLI = %+v, erwartet keine", status.CLI)
	}
	if status.Hint == "" {
		t.Error("ohne CLI fehlt der ausdrückliche Weg")
	}
}

// Der Dienst zieht die Erweiterung beim Start nach — und zwar nur aus einem
// gestempelten Programm. Ohne Version ist der Prozess ein Ad-hoc-Build oder
// ein Test-Binary; dass ein Testlauf wirklich in das VS Code des Rechners
// installiert, ist genau der Fehler, den dieser Test verhindert.
func TestCareForVSCodeExtension(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	t.Setenv("VSCODE_IPC_HOOK_CLI", "")

	vorher := vscodeext.LookPath
	vscodeext.LookPath = func(string) (string, error) { return "", errors.New("nicht im PATH") }
	t.Cleanup(func() { vscodeext.LookPath = vorher })

	// Ein code-server, der nur bestätigt, statt wirklich zu installieren.
	cli := filepath.Join(home, ".vscode-server", "bin", "abc123", "bin", "code-server")
	if err := os.MkdirAll(filepath.Dir(cli), 0o755); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nprintf 'Extension was successfully installed.\\n'\n"
	if err := os.WriteFile(cli, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	// Die Marke, die jeden Testlauf von der echten CLI abschneidet, ist hier
	// ausdrücklich aufgehoben: gerufen wird allein der Fake in diesem HOME.
	// Dass sie ohne dieses Aufheben wirklich greift, belegt
	// TestDienstRuehrtImTestlaufKeineCLIAn — dort gegen ein echtes,
	// gestempeltes Programm als Dienst.
	t.Setenv(vscodeext.EnvNoInstall, "")

	versionBefore := buildinfo.Version
	t.Cleanup(func() { buildinfo.Version = versionBefore })

	buildinfo.Version = ""
	careForVSCodeExtension(context.Background())
	if refresh := vscodeext.LastRefresh(); refresh != nil {
		t.Fatalf("ohne gestempelte Version wurde nachgezogen: %+v", refresh)
	}

	buildinfo.Version = "v0.9.4"
	careForVSCodeExtension(context.Background())
	deadline := time.Now().Add(5 * time.Second)
	for vscodeext.LastRefresh() == nil && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	refresh := vscodeext.LastRefresh()
	if refresh == nil {
		t.Fatal("der Dienst hat nicht nachgezogen")
	}
	if refresh.Action != vscodeext.ActionInstalled {
		t.Errorf("Nachzug = %+v", refresh)
	}
}

// Gegenprobe zur Abdichtung des Testlaufs.
//
// Dieser Test ist der Grund, warum die Abdichtung in der Umgebung sitzt und
// nicht im PATH einzelner Tests: Er startet ein echtes, über -ldflags
// gestempeltes Programm als Dienst — denselben Weg, den restart_test.go und
// update_program_test.go gehen —, legt ihm ein `code` in den PATH und belegt,
// dass der Dienst es nicht anfasst. Gestempelt passiert das Programm den
// Wächter `guiproc.OwnVersion() != ""`; nur die Marke in der Umgebung, die das
// Kind von diesem Testprozess erbt, hält es auf.
//
// Die Probe hat Zähne: derselbe Dienst mit aufgehobener Marke ruft die
// (gefälschte) CLI wirklich. Ohne diesen zweiten Teil wäre nicht zu
// unterscheiden, ob die Abdichtung greift oder die Probe nur nichts sieht.
func TestDienstRuehrtImTestlaufKeineCLIAn(t *testing.T) {
	asset := buildProgram(t, "v0.9.93")
	isolateHome(t)
	home := os.Getenv("HOME")
	chdir(t, t.TempDir())
	// Ohne lebenden IPC-Hook: sonst nähme die Wahl Rang 1 und fände im
	// isolierten HOME keine Remote-CLI.
	t.Setenv("VSCODE_IPC_HOOK_CLI", "")

	// Ein `code` im PATH des Dienstes — nicht das echte. Es schreibt nur auf,
	// dass und wie es gerufen wurde. Genau diese Datei wäre auf einem Rechner
	// mit `code` in /usr/bin eine echte Installation.
	marker := filepath.Join(t.TempDir(), "code-gerufen")
	writeExecutable(t, filepath.Join(home, ".local", "bin", "code"),
		"#!/bin/sh\nprintf '%s\\n' \"$@\" >> "+marker+"\n"+
			"printf 'Extension was successfully installed.\\n'\n")

	refresh := func(extraEnv ...string) *vscodeext.Refresh {
		t.Helper()

		key, err := guiproc.Key()
		if err != nil {
			t.Fatal(err)
		}
		location, err := guiproc.Locate(key)
		if err != nil {
			t.Fatal(err)
		}
		child, err := guiproc.Spawn(asset, filepath.Join(t.TempDir(), "dienst.log"), extraEnv...)
		if err != nil {
			t.Fatalf("Dienst starten: %v", err)
		}
		defer func() {
			child.Terminate()
			guiproc.WaitForExit(location.File, child.Process.Pid, 5*time.Second)
		}()
		record, err := child.Await(key, location.File, 20*time.Second, guiproc.ProbeHealth)
		if err != nil {
			t.Fatalf("Dienst antwortet nicht: %v\n%s", err, child.Log())
		}

		// Der Nachzug läuft in einer Goroutine; gewartet wird auf seinen
		// Vermerk, nicht auf eine Zeitspanne. Jeder Ausgang hinterlässt
		// einen — auch der gesperrte, und genau deshalb ist dieser Test
		// nicht auf ein Zeitfenster angewiesen.
		deadline := time.Now().Add(20 * time.Second)
		for {
			status := fetchVSCodeStatus(t, record.Addr)
			if status.LastRefresh != nil {
				return status.LastRefresh
			}
			if time.Now().After(deadline) {
				t.Fatalf("kein Nachzug vermerkt\n%s", child.Log())
			}
			time.Sleep(20 * time.Millisecond)
		}
	}

	// Mit Marke: gesperrt, keine CLI gewählt, nichts gerufen.
	if got := refresh(); got.Action != vscodeext.ActionBlocked {
		t.Fatalf("Nachzug = %+v, erwartet %q", got, vscodeext.ActionBlocked)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		data, _ := os.ReadFile(marker)
		t.Fatalf("der Dienst hat eine CLI gerufen: %s", data)
	}

	// Ohne Marke: derselbe Dienst ruft die Fake-CLI wirklich — die Probe
	// würde ein Leck also sehen.
	if got := refresh(vscodeext.EnvNoInstall + "="); got.Action != vscodeext.ActionInstalled {
		t.Fatalf("mit aufgehobener Marke = %+v, erwartet %q", got, vscodeext.ActionInstalled)
	}
	data, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("die Gegenprobe hätte ein Leck nicht gesehen: %v", err)
	}
	if !strings.Contains(string(data), "--install-extension") || !strings.Contains(string(data), "--force") {
		t.Errorf("Aufruf = %q", data)
	}
}

// fetchVSCodeStatus liest GET /api/vscode eines laufenden Dienstes.
func fetchVSCodeStatus(t *testing.T, addr string) vscodeext.Status {
	t.Helper()

	response, err := http.Get("http://" + addr + "/api/vscode")
	if err != nil {
		t.Fatalf("GET /api/vscode: %v", err)
	}
	defer response.Body.Close()
	var status vscodeext.Status
	if err := json.NewDecoder(response.Body).Decode(&status); err != nil {
		t.Fatalf("Antwort lesen: %v", err)
	}
	return status
}
