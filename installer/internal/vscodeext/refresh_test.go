package vscodeext

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// vorbereiten gibt dem Test ein eigenes HOME, ein LookPath, das nichts
// findet, einen zurückgesetzten Nachzug-Speicher und eine aufgehobene
// Install-Marke. Das echte VS Code dieses Rechners bleibt unerreichbar.
func vorbereiten(t *testing.T) (home string, lockDir string) {
	t.Helper()
	home = t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("VSCODE_IPC_HOOK_CLI", "")

	// Die Marke ist aufgehoben: die Tests hier prüfen den Nachzug selbst,
	// und zwar ausschließlich gegen eine Fake-CLI in diesem HOME.
	erlaubeInstall(t)

	vorher := LookPath
	LookPath = func(string) (string, error) { return "", errors.New("nicht im PATH") }
	lastMu.Lock()
	last = nil
	lastMu.Unlock()
	t.Cleanup(func() {
		LookPath = vorher
		lastMu.Lock()
		last = nil
		lastMu.Unlock()
	})
	return home, t.TempDir()
}

// codeServer legt ein ~/.vscode-server/bin/<commit>/bin/code-server an, das
// mit exitCode endet und message ausgibt.
func codeServer(t *testing.T, home string, exitCode int, message string) {
	t.Helper()
	path := filepath.Join(home, ".vscode-server", "bin", "abc123", "bin", "code-server")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nprintf '" + message + "\\n'\nexit " + strconv.Itoa(exitCode) + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

func eingebettet(t *testing.T) string {
	t.Helper()
	version, ok, err := EmbeddedVersion()
	if !ok || err != nil {
		t.Fatalf("eingebettete Version: %v, %v", ok, err)
	}
	return version
}

// Ohne VS Code in der Umgebung: nichts tun, nichts melden, nichts vermerken.
func TestEnsureInstalledOhneVSCode(t *testing.T) {
	_, lockDir := vorbereiten(t)

	var log bytes.Buffer
	EnsureInstalled(context.Background(), lockDir, &log)

	if log.Len() != 0 {
		t.Errorf("Log = %q, erwartet still", log.String())
	}
	if LastRefresh() != nil {
		t.Errorf("Nachzug vermerkt: %+v", LastRefresh())
	}
}

// Liegt die passende Fassung, wird nichts getan und nichts gemeldet — der
// Vermerk hält es trotzdem fest, damit die Statusanzeige es nennen kann.
func TestEnsureInstalledNichtsZuTun(t *testing.T) {
	home, lockDir := vorbereiten(t)
	codeServer(t, home, 0, "sollte nicht aufgerufen werden")
	extensions := filepath.Join(home, ".vscode-server", "extensions")
	schreibe(t, filepath.Join(extensions, "extensions.json"),
		manifest(eintrag(ID, eingebettet(t))))

	var log bytes.Buffer
	EnsureInstalled(context.Background(), lockDir, &log)

	if log.Len() != 0 {
		t.Errorf("Log = %q, erwartet still", log.String())
	}
	refresh := LastRefresh()
	if refresh == nil || refresh.Action != ActionNothing {
		t.Fatalf("Nachzug = %+v", refresh)
	}
}

// Fehlt die Erweiterung, wird installiert: eine Zeile im Log, Vermerk mit der
// gewählten CLI.
func TestEnsureInstalledInstalliert(t *testing.T) {
	home, lockDir := vorbereiten(t)
	codeServer(t, home, 0, "Extension was successfully installed.")

	var log bytes.Buffer
	EnsureInstalled(context.Background(), lockDir, &log)

	if lines := strings.Count(strings.TrimSpace(log.String()), "\n"); lines != 0 {
		t.Errorf("Log ist nicht eine Zeile: %q", log.String())
	}
	if !strings.Contains(log.String(), eingebettet(t)) {
		t.Errorf("Log = %q", log.String())
	}
	refresh := LastRefresh()
	if refresh == nil || refresh.Action != ActionInstalled {
		t.Fatalf("Nachzug = %+v", refresh)
	}
	if refresh.CLI == nil || refresh.CLI.Kind != KindServer {
		t.Errorf("CLI = %+v", refresh.CLI)
	}
	if refresh.At.IsZero() {
		t.Error("kein Zeitpunkt vermerkt")
	}
}

// Liegt die Erweiterung in beiden Verzeichnissen in verschiedenen Fassungen,
// ist nach dem Nachzug nichts mehr zu tun: gefragt wird nach dem Verzeichnis,
// das die gewählte CLI beschreibt, nicht nach „irgendwo". Fragte es
// „irgendwo", installierte der Dienst bei jedem Start erneut und schriebe
// jedes Mal „installiert" ins Log, obwohl nichts zu tun ist.
func TestEnsureInstalledZweiVerzeichnisseVerschiedenerFassung(t *testing.T) {
	home, lockDir := vorbereiten(t)
	codeServer(t, home, 0, "sollte nicht aufgerufen werden")
	version := eingebettet(t)
	// code-server bedient ~/.vscode-server/extensions — dort steht die
	// passende Fassung. Die lokale Installation daneben trägt eine alte und
	// ist über diese CLI nicht erreichbar.
	schreibe(t, filepath.Join(home, ".vscode-server", "extensions", "extensions.json"),
		manifest(eintrag(ID, version)))
	schreibe(t, filepath.Join(home, ".vscode", "extensions", "extensions.json"),
		manifest(eintrag(ID, "0.0.9")))

	var log bytes.Buffer
	EnsureInstalled(context.Background(), lockDir, &log)

	if log.Len() != 0 {
		t.Errorf("Log = %q, erwartet still", log.String())
	}
	refresh := LastRefresh()
	if refresh == nil || refresh.Action != ActionNothing {
		t.Fatalf("Nachzug = %+v, erwartet %q", refresh, ActionNothing)
	}
}

// Eine scheiternde CLI bleibt nicht still: der Grund steht im Log und im
// Vermerk, damit die Statusanzeige ihn zeigen kann.
func TestEnsureInstalledScheiterndeCLI(t *testing.T) {
	home, lockDir := vorbereiten(t)
	codeServer(t, home, 1, "Unable to install extension")

	var log bytes.Buffer
	EnsureInstalled(context.Background(), lockDir, &log)

	if !strings.Contains(log.String(), "nicht installiert") {
		t.Errorf("Log = %q", log.String())
	}
	if !strings.Contains(log.String(), "Unable to install extension") {
		t.Errorf("die Ausgabe der CLI fehlt im Log: %q", log.String())
	}
	refresh := LastRefresh()
	if refresh == nil || refresh.Action != ActionFailed {
		t.Fatalf("Nachzug = %+v", refresh)
	}
	if refresh.Error == "" || refresh.Output == "" {
		t.Errorf("Vermerk ohne Grund: %+v", refresh)
	}
}

// Die Sperre ist ausschließend: zwei gleichzeitig startende Projekt-Dienste
// schreiben nicht gleichzeitig in dasselbe Erweiterungsverzeichnis. Der
// Zweite wartet und prüft danach neu.
func TestSperreSchliesstAus(t *testing.T) {
	dir := t.TempDir()

	release, err := lock(dir)
	if err != nil {
		t.Fatalf("erste Sperre: %v", err)
	}

	genommen := make(chan func(), 1)
	go func() {
		zweite, err := lock(dir)
		if err != nil {
			close(genommen)
			return
		}
		genommen <- zweite
	}()

	select {
	case <-genommen:
		t.Fatal("die zweite Sperre kam durch, während die erste liegt")
	case <-time.After(150 * time.Millisecond):
	}

	release()
	select {
	case zweite, ok := <-genommen:
		if !ok {
			t.Fatal("die zweite Sperre scheiterte")
		}
		zweite()
	case <-time.After(2 * time.Second):
		t.Fatal("die zweite Sperre kam nach der Freigabe nicht durch")
	}
}

func TestSperreOhneVerzeichnis(t *testing.T) {
	if _, err := lock(""); err == nil {
		t.Error("lock(\"\") ohne Fehler")
	}
	if _, err := lock(filepath.Join(t.TempDir(), "gibt-es-nicht")); err == nil {
		t.Error("lock in fehlendem Verzeichnis ohne Fehler")
	}
}
