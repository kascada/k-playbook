package project

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// newGitInstallation baut ein Projekt, dessen Installation ein Git-Repo mit
// Upstream ist. Das Remote liegt als bare Repo daneben.
func newGitInstallation(t *testing.T) (projectDir string, remoteDir string) {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git nicht verfügbar")
	}

	base := t.TempDir()
	remoteDir = filepath.Join(base, "remote.git")
	run(t, base, "git", "init", "--bare", "-b", "main", remoteDir)

	projectDir = filepath.Join(base, "projekt")
	dir := PlaybookDir(projectDir)
	t.Cleanup(func() {
		_ = setInstallationWritable(projectDir)
	})
	if err := os.MkdirAll(filepath.Join(dir, "dist"), 0o755); err != nil {
		t.Fatalf("Verzeichnis anlegen: %v", err)
	}
	// Seit die Binaries Release-Assets sind, liegt hier keins mehr. Die Datei
	// bleibt als beliebige verfolgte Datei stehen — die Tests darunter prüfen
	// Schreibsperre und Sauberkeit, nicht den Inhalt.
	if err := os.WriteFile(filepath.Join(dir, "dist", "k-playbook-linux-amd64"), []byte("alt"), 0o755); err != nil {
		t.Fatalf("Datei anlegen: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, VersionFileName), []byte("v0.0.1\n"), 0o644); err != nil {
		t.Fatalf("VERSION anlegen: %v", err)
	}

	run(t, dir, "git", "init", "-b", "main")
	run(t, dir, "git", "config", "user.email", "test@example.com")
	run(t, dir, "git", "config", "user.name", "Test")
	run(t, dir, "git", "add", "-A")
	run(t, dir, "git", "commit", "-m", "erster Stand")
	run(t, dir, "git", "remote", "add", "origin", remoteDir)
	run(t, dir, "git", "push", "-u", "origin", "main")
	return projectDir, remoteDir
}

func run(t *testing.T, dir string, name string, args ...string) {
	t.Helper()

	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, output)
	}
}

func assertNotWritable(t *testing.T, path string) {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("%s stat: %v", path, err)
	}
	if info.Mode().Perm()&0o222 != 0 {
		t.Fatalf("%s ist beschreibbar: %o", path, info.Mode().Perm())
	}
}

func TestCheckUpdateOhneNeuerung(t *testing.T) {
	projectDir, _ := newGitInstallation(t)

	status, err := CheckUpdate(projectDir)
	if err != nil {
		t.Fatalf("CheckUpdate: %v", err)
	}
	if status.Available {
		t.Errorf("Update gemeldet, obwohl der Stand gleich ist: %+v", status)
	}
	if status.Branch != "main" {
		t.Errorf("Branch = %q, erwartet main", status.Branch)
	}
}

func TestCheckUpdateErkenntNeuenStand(t *testing.T) {
	projectDir, remoteDir := newGitInstallation(t)
	if err := SetInstallationReadOnly(projectDir); err != nil {
		t.Fatalf("Installation sperren: %v", err)
	}

	// Ein zweiter Clone schiebt einen Commit ins Remote.
	other := filepath.Join(t.TempDir(), "anderer")
	run(t, filepath.Dir(other), "git", "clone", remoteDir, other)
	run(t, other, "git", "config", "user.email", "test@example.com")
	run(t, other, "git", "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(other, "neu.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("Datei anlegen: %v", err)
	}
	run(t, other, "git", "add", "-A")
	run(t, other, "git", "commit", "-m", "zweiter Stand")
	run(t, other, "git", "push")

	status, err := CheckUpdate(projectDir)
	if err != nil {
		t.Fatalf("CheckUpdate: %v", err)
	}
	if !status.Available {
		t.Errorf("kein Update gemeldet: %+v", status)
	}
	if status.Local == status.Remote {
		t.Error("Local und Remote sind gleich, obwohl ein Commit dazukam")
	}
}

func TestSetInstallationReadOnlyEntziehtSchreibrechte(t *testing.T) {
	projectDir, _ := newGitInstallation(t)
	dir := PlaybookDir(projectDir)

	if err := SetInstallationReadOnly(projectDir); err != nil {
		t.Fatalf("SetInstallationReadOnly: %v", err)
	}

	assertNotWritable(t, filepath.Join(dir, "dist", "k-playbook-linux-amd64"))
	assertNotWritable(t, filepath.Join(dir, ".git", "config"))
}

func TestUpdateMachtReadOnlyInstallationTemporärBeschreibbar(t *testing.T) {
	projectDir, remoteDir := newGitInstallation(t)
	dir := PlaybookDir(projectDir)

	other := filepath.Join(t.TempDir(), "anderer")
	run(t, filepath.Dir(other), "git", "clone", remoteDir, other)
	run(t, other, "git", "config", "user.email", "test@example.com")
	run(t, other, "git", "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(other, "doku.md"), []byte("neu"), 0o644); err != nil {
		t.Fatalf("Datei anlegen: %v", err)
	}
	run(t, other, "git", "add", "-A")
	run(t, other, "git", "commit", "-m", "neuer Stand")
	run(t, other, "git", "push")

	if err := SetInstallationReadOnly(projectDir); err != nil {
		t.Fatalf("Installation sperren: %v", err)
	}
	result, err := Update(projectDir)
	if err != nil {
		t.Fatalf("Update: %v\n%s", err, result.Output)
	}

	if !fileExists(filepath.Join(dir, "doku.md")) {
		t.Error("Update hat den neuen Stand nicht eingespielt")
	}
	assertNotWritable(t, filepath.Join(dir, "doku.md"))
	assertNotWritable(t, filepath.Join(dir, ".git", "config"))
}

// Nur wenn VERSION wechselt, bringt ein Neustart eine andere Version.
func TestUpdateMeldetVersionswechsel(t *testing.T) {
	projectDir, remoteDir := newGitInstallation(t)

	other := filepath.Join(t.TempDir(), "anderer")
	run(t, filepath.Dir(other), "git", "clone", remoteDir, other)
	run(t, other, "git", "config", "user.email", "test@example.com")
	run(t, other, "git", "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(other, VersionFileName), []byte("v0.0.2\n"), 0o644); err != nil {
		t.Fatalf("VERSION ändern: %v", err)
	}
	run(t, other, "git", "add", "-A")
	run(t, other, "git", "commit", "-m", "neue Version")
	run(t, other, "git", "push")

	result, err := Update(projectDir)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !result.VersionChanged {
		t.Error("VersionChanged = false, obwohl VERSION gewechselt hat")
	}
	if result.Version != "v0.0.2" {
		t.Errorf("Version = %q, erwartet v0.0.2", result.Version)
	}

	status, err := CheckUpdate(projectDir)
	if err != nil {
		t.Fatalf("CheckUpdate nach Update: %v", err)
	}
	if status.Available {
		t.Error("nach dem Update wird weiterhin ein Update gemeldet")
	}
}

func TestUpdateOhneVersionswechsel(t *testing.T) {
	projectDir, remoteDir := newGitInstallation(t)

	other := filepath.Join(t.TempDir(), "anderer")
	run(t, filepath.Dir(other), "git", "clone", remoteDir, other)
	run(t, other, "git", "config", "user.email", "test@example.com")
	run(t, other, "git", "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(other, "doku.md"), []byte("nur Text"), 0o644); err != nil {
		t.Fatalf("Datei anlegen: %v", err)
	}
	run(t, other, "git", "add", "-A")
	run(t, other, "git", "commit", "-m", "nur Doku")
	run(t, other, "git", "push")

	result, err := Update(projectDir)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if result.VersionChanged {
		t.Error("VersionChanged = true, obwohl VERSION gleich geblieben ist")
	}
}

func TestCheckCleanlinessSauber(t *testing.T) {
	projectDir, _ := newGitInstallation(t)

	state := CheckCleanliness(projectDir)
	if !state.Clean || state.Blocking() {
		t.Errorf("frische Installation gilt als verschmutzt: %+v", state)
	}
}

// Der Fall, der ohne diese Prüfung nie auffällt: eine veränderte Datei, die
// sich upstream nicht mit ändert. `git pull` läuft sauber durch und lässt
// sie stehen.
func TestCheckCleanlinessErkenntVeraenderteDatei(t *testing.T) {
	projectDir, _ := newGitInstallation(t)

	binary := filepath.Join(PlaybookDir(projectDir), "dist", "k-playbook-linux-amd64")
	if err := os.WriteFile(binary, []byte("kaputt"), 0o755); err != nil {
		t.Fatalf("Binary ändern: %v", err)
	}

	state := CheckCleanliness(projectDir)
	if state.Clean || !state.Blocking() {
		t.Fatalf("veränderte Datei nicht als blockierend gemeldet: %+v", state)
	}
	if len(state.Modified) != 1 || state.Modified[0] != "dist/k-playbook-linux-amd64" {
		t.Errorf("Modified = %v, erwartet die geänderte Datei", state.Modified)
	}
	if state.Message == "" {
		t.Error("keine Meldung zum Zustand")
	}
}

// Zusätzliche Dateien sind auffällig, stehen einem Fast-Forward aber nicht im
// Weg. Sie dürfen das Update deshalb nicht verhindern.
func TestCheckCleanlinessUntrackedBlockiertNicht(t *testing.T) {
	projectDir, _ := newGitInstallation(t)

	extra := filepath.Join(PlaybookDir(projectDir), "notiz.txt")
	if err := os.WriteFile(extra, []byte("x"), 0o644); err != nil {
		t.Fatalf("Datei anlegen: %v", err)
	}

	state := CheckCleanliness(projectDir)
	if state.Clean {
		t.Error("zusätzliche Datei nicht gemeldet")
	}
	if state.Blocking() {
		t.Errorf("zusätzliche Datei blockiert das Update: %+v", state)
	}
	if len(state.Untracked) != 1 || state.Untracked[0] != "notiz.txt" {
		t.Errorf("Untracked = %v, erwartet notiz.txt", state.Untracked)
	}
}

func TestCheckCleanlinessErkenntLokaleCommits(t *testing.T) {
	projectDir, _ := newGitInstallation(t)
	dir := PlaybookDir(projectDir)

	if err := os.WriteFile(filepath.Join(dir, "eigen.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("Datei anlegen: %v", err)
	}
	run(t, dir, "git", "add", "-A")
	run(t, dir, "git", "commit", "-m", "lokal")

	state := CheckCleanliness(projectDir)
	if state.Ahead != 1 || !state.Blocking() {
		t.Errorf("lokaler Commit nicht als blockierend gemeldet: %+v", state)
	}
}

// Vorher prüfen statt hinterher stolpern: das Update läuft gar nicht erst an.
func TestUpdateBrichtBeiVerschmutzterInstallationAb(t *testing.T) {
	projectDir, remoteDir := newGitInstallation(t)

	other := filepath.Join(t.TempDir(), "anderer")
	run(t, filepath.Dir(other), "git", "clone", remoteDir, other)
	run(t, other, "git", "config", "user.email", "test@example.com")
	run(t, other, "git", "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(other, "doku.md"), []byte("neu"), 0o644); err != nil {
		t.Fatalf("Datei anlegen: %v", err)
	}
	run(t, other, "git", "add", "-A")
	run(t, other, "git", "commit", "-m", "neuer Stand")
	run(t, other, "git", "push")

	// Eine lokal veränderte Datei, die mit dem neuen Stand gar nicht
	// kollidiert. Ohne Vorabprüfung liefe der Pull hier sauber durch.
	binary := filepath.Join(PlaybookDir(projectDir), "dist", "k-playbook-linux-amd64")
	if err := os.WriteFile(binary, []byte("kaputt"), 0o755); err != nil {
		t.Fatalf("Binary ändern: %v", err)
	}

	result, err := Update(projectDir)
	if err == nil {
		t.Fatal("Update lief trotz veränderter Datei durch")
	}
	if !result.Cleanliness.Blocking() {
		t.Errorf("Cleanliness nicht mitgeliefert: %+v", result.Cleanliness)
	}

	// Der Pull darf nicht stattgefunden haben.
	if data, readErr := os.ReadFile(binary); readErr != nil || string(data) != "kaputt" {
		t.Errorf("die lokale Änderung wurde angetastet: %q, %v", data, readErr)
	}
}

func TestCheckUpdateOhneGit(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(PlaybookDir(root), 0o755); err != nil {
		t.Fatalf("Verzeichnis anlegen: %v", err)
	}

	status, err := CheckUpdate(root)
	if err != nil {
		t.Fatalf("CheckUpdate: %v", err)
	}
	if status.Available || status.Message == "" {
		t.Errorf("erwartet wurde eine Meldung ohne Update: %+v", status)
	}
}

// Nach dem Clone-Update darf kein Bestandsprojekt mit einer kaputten
// MCP-Registrierung zurückbleiben. Der Pull erreicht die Datei nicht — sie
// liegt im Hauptverzeichnis, nicht im Clone —, also korrigiert das Update sie
// selbst.
func TestUpdateKorrigiertVeralteteMCPRegistrierung(t *testing.T) {
	projectDir, _ := newGitInstallation(t)

	installed := installTestBinary(t)
	writeFile(t, filepath.Join(projectDir, ".mcp.json"),
		`{"mcpServers":{"`+MCPServerKey+`":{"command":"k-playbook/bin/k-playbook","args":["mcp"]}}}`+"\n")

	result, err := Update(projectDir)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(result.MCPRepaired) != 1 || result.MCPRepaired[0] != ".mcp.json" {
		t.Fatalf("MCPRepaired = %v, erwartet [.mcp.json]", result.MCPRepaired)
	}

	status := mcpStatusFor(t, CheckMCP(projectDir), ".mcp.json")
	if !status.OK() {
		t.Fatalf("nach dem Update nicht korrigiert: %+v", status)
	}
	if status.Detail != "-> "+installed {
		t.Errorf("Detail = %q, erwartet den absoluten Pfad %s", status.Detail, installed)
	}
}
