package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/vscodeext"
)

// isoliertesHome gibt dem Test ein eigenes Heimatverzeichnis und ein
// LookPath, das nichts findet. Das echte VS Code dieses Rechners bleibt so
// unerreichbar: die Remote-CLI ignoriert --extensions-dir und würde in die
// laufende Umgebung schreiben.
func isoliertesHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("VSCODE_IPC_HOOK_CLI", "")
	vorher := vscodeext.LookPath
	vscodeext.LookPath = func(string) (string, error) { return "", errors.New("nicht im PATH") }
	t.Cleanup(func() { vscodeext.LookPath = vorher })
	return home
}

// fakeCodeServer legt ein ~/.vscode-server/bin/<commit>/bin/code-server an,
// das seine Argumente in marker schreibt.
func fakeCodeServer(t *testing.T, home string, marker string) {
	t.Helper()
	path := filepath.Join(home, ".vscode-server", "bin", "abc123", "bin", "code-server")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + marker + "\n" +
		"printf 'Extension was successfully installed.\\n'\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

func extensionsJSON(t *testing.T, home string, version string) {
	t.Helper()
	dir := filepath.Join(home, ".vscode-server", "extensions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `[{"identifier":{"id":"` + vscodeext.ID + `"},"version":"` + version + `"}]`
	if err := os.WriteFile(filepath.Join(dir, "extensions.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestVSCodeStatusOhneVSCode(t *testing.T) {
	isoliertesHome(t)

	var out bytes.Buffer
	if err := runVSCodeStatus(nil, &out); err != nil {
		t.Fatalf("status: %v", err)
	}
	text := out.String()
	for _, want := range []string{vscodeext.ID, "in keinem Verzeichnis", "CLI: keine", "Nachzuziehen: ja"} {
		if !strings.Contains(text, want) {
			t.Errorf("Ausgabe nennt %q nicht:\n%s", want, text)
		}
	}
}

// Installiert in der eingebetteten Fassung: nichts nachzuziehen.
func TestVSCodeStatusPassendInstalliert(t *testing.T) {
	home := isoliertesHome(t)
	version, ok, err := vscodeext.EmbeddedVersion()
	if !ok || err != nil {
		t.Fatalf("eingebettete Version: %v, %v", ok, err)
	}
	extensionsJSON(t, home, version)
	fakeCodeServer(t, home, filepath.Join(t.TempDir(), "argumente"))

	var out bytes.Buffer
	if err := runVSCodeStatus(nil, &out); err != nil {
		t.Fatalf("status: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "Nachzuziehen: nein") {
		t.Errorf("Ausgabe:\n%s", text)
	}
	if !strings.Contains(text, string(vscodeext.KindServer)) {
		t.Errorf("die gewählte CLI fehlt:\n%s", text)
	}
}

func TestVSCodeInstallRuftCodeServer(t *testing.T) {
	home := isoliertesHome(t)
	// Die Marke, die jeden Testlauf von der echten CLI abschneidet, ist hier
	// ausdrücklich aufgehoben: gerufen wird allein der Fake in diesem HOME.
	t.Setenv(vscodeext.EnvNoInstall, "")
	marker := filepath.Join(t.TempDir(), "argumente")
	fakeCodeServer(t, home, marker)

	var out bytes.Buffer
	if err := runVSCodeInstall(nil, &out); err != nil {
		t.Fatalf("install: %v", err)
	}
	if !strings.Contains(out.String(), "successfully installed") {
		t.Errorf("Ausgabe:\n%s", out.String())
	}

	data, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("Marker: %v", err)
	}
	argumente := strings.Fields(strings.TrimSpace(string(data)))
	if len(argumente) != 3 || argumente[0] != "--install-extension" || argumente[2] != "--force" {
		t.Errorf("Argumente = %v", argumente)
	}
}

// Ohne CLI nennt der Fehler den ausdrücklichen Weg über die VSIX.
func TestVSCodeInstallOhneCLI(t *testing.T) {
	isoliertesHome(t)

	err := runVSCodeInstall(nil, &bytes.Buffer{})
	if err == nil {
		t.Fatal("install ohne Fehler")
	}
	if !strings.Contains(err.Error(), "vscode vsix -o") {
		t.Errorf("Fehler = %v", err)
	}
}

func TestVSCodeVSIXSchreibtDatei(t *testing.T) {
	target := filepath.Join(t.TempDir(), "erweiterung.vsix")

	var out bytes.Buffer
	if err := runVSCodeVSIX([]string{"-o", target}, &out); err != nil {
		t.Fatalf("vsix: %v", err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("Datei lesen: %v", err)
	}
	version, err := vscodeext.Version(data)
	if err != nil {
		t.Fatalf("geschriebene VSIX: %v", err)
	}
	embedded, _, _ := vscodeext.EmbeddedVersion()
	if version != embedded {
		t.Errorf("Version = %q, eingebettet %q", version, embedded)
	}
	if !strings.Contains(out.String(), target) {
		t.Errorf("Ausgabe:\n%s", out.String())
	}
}

func TestVSCodeVSIXOhneZiel(t *testing.T) {
	if err := runVSCodeVSIX(nil, &bytes.Buffer{}); err == nil {
		t.Error("vsix ohne -o ohne Fehler")
	}
}

func TestVSCodeUnbekanntesKommando(t *testing.T) {
	if err := runVSCode([]string{"loeschen"}); err == nil {
		t.Error("unbekanntes Kommando ohne Fehler")
	}
	if err := runVSCode([]string{"status", "zuviel"}); err == nil {
		t.Error("überzähliges Argument ohne Fehler")
	}
}

// Liegen zwei Installationen in verschiedenen Fassungen, nennt die Ausgabe
// das Verzeichnis, in das ein Nachzug schreibt, und die abweichende Fassung
// daneben — „Nachzuziehen" bleibt trotzdem „nein", weil im Ziel die passende
// Fassung liegt. Sonst stünde dort dauerhaft „ja", und der Dienst installierte
// bei jedem Start erneut.
func TestVSCodeStatusNenntAbweichendeZweitfassung(t *testing.T) {
	home := isoliertesHome(t)
	version, ok, err := vscodeext.EmbeddedVersion()
	if !ok || err != nil {
		t.Fatalf("eingebettete Version: %v, %v", ok, err)
	}
	extensionsJSON(t, home, version)
	lokal := filepath.Join(home, ".vscode", "extensions")
	if err := os.MkdirAll(lokal, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `[{"identifier":{"id":"` + vscodeext.ID + `"},"version":"0.0.9"}]`
	if err := os.WriteFile(filepath.Join(lokal, "extensions.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	fakeCodeServer(t, home, filepath.Join(t.TempDir(), "argumente"))

	var out bytes.Buffer
	if err := runVSCodeStatus(nil, &out); err != nil {
		t.Fatalf("status: %v", err)
	}
	text := out.String()
	for _, want := range []string{
		"Nachzug schreibt nach: " + filepath.Join(home, ".vscode-server", "extensions"),
		"Abweichend, von dieser CLI nicht erreichbar: " + lokal + ": 0.0.9",
		"Nachzuziehen: nein",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("Ausgabe nennt %q nicht:\n%s", want, text)
		}
	}
}

// vscodeProjekt legt ein Projekt mit K-PLAYBOOK.yaml an und, wenn local nicht
// leer ist, k-playbook-local/K-PLAYBOOK.local.yaml mit diesem Inhalt.
func vscodeProjekt(t *testing.T, local string) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", root)
	if err := os.WriteFile(filepath.Join(root, "K-PLAYBOOK.yaml"), []byte("schema_version: 3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if local != "" {
		dir := filepath.Join(root, "k-playbook-local")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "K-PLAYBOOK.local.yaml"), []byte(local), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// Aus einem Unterverzeichnis heraus: Die Erweiterung ruft mit dem
// Projektordner als Arbeitsverzeichnis, ein Aufruf von Hand aber von überall.
func TestVSCodeCollectionsAusDerAnbindung(t *testing.T) {
	root := vscodeProjekt(t, "kephalaion:\n  hub: vm\n  collection: omni\n  account: kamran-wsl\n")
	sub := filepath.Join(root, "src")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	answer := collectWorkspaceFolders(sub)
	if !answer.Configured || answer.Message != "" {
		t.Fatalf("Antwort = %+v", answer)
	}
	if len(answer.Folders) != 2 ||
		answer.Folders[0].URI != "keph://vm/k-playbook" ||
		answer.Folders[1].URI != "keph://vm/omni" {
		t.Fatalf("Ordner = %+v", answer.Folders)
	}
}

// Ohne Anbindung und außerhalb eines Projekts ist das ein Zustand mit
// Begründung, kein Fehler, und folders bleibt eine leere Liste statt null:
// die Erweiterung liest beides als JSON.
func TestVSCodeCollectionsNichtEingerichtet(t *testing.T) {
	ohne := vscodeProjekt(t, "")
	draussen := t.TempDir()
	for name, dir := range map[string]string{"ohne Anbindung": ohne, "kein Projekt": draussen} {
		answer := collectWorkspaceFolders(dir)
		if answer.Configured || answer.Message == "" || answer.Folders == nil || len(answer.Folders) != 0 {
			t.Fatalf("%s: Antwort = %+v", name, answer)
		}
	}
	if answer := collectWorkspaceFolders(ohne); !strings.Contains(answer.Message, "K-PLAYBOOK.local.yaml") {
		t.Fatalf("Meldung nennt die Datei nicht: %s", answer.Message)
	}
}

func TestVSCodeCollectionsUnbekannteOption(t *testing.T) {
	var out bytes.Buffer
	if err := runVSCodeCollections([]string{"--yaml"}, &out); err == nil {
		t.Fatal("kein Fehler bei unbekannter Option")
	}
}
