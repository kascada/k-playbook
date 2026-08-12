package project

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	if err := os.MkdirAll(filepath.Join(dir, "dist"), 0o755); err != nil {
		t.Fatalf("Verzeichnis anlegen: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "dist", "k-playbook-linux-amd64"), []byte("alt"), 0o755); err != nil {
		t.Fatalf("Binary anlegen: %v", err)
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

// Nur wenn sich die Binaries ändern, bringt ein Neustart eine andere Version.
func TestUpdateMeldetGeaenderteBinaries(t *testing.T) {
	projectDir, remoteDir := newGitInstallation(t)

	other := filepath.Join(t.TempDir(), "anderer")
	run(t, filepath.Dir(other), "git", "clone", remoteDir, other)
	run(t, other, "git", "config", "user.email", "test@example.com")
	run(t, other, "git", "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(other, "dist", "k-playbook-linux-amd64"), []byte("neu"), 0o755); err != nil {
		t.Fatalf("Binary ändern: %v", err)
	}
	run(t, other, "git", "add", "-A")
	run(t, other, "git", "commit", "-m", "neues Binary")
	run(t, other, "git", "push")

	result, err := Update(projectDir)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !result.BinaryChanged {
		t.Error("BinaryChanged = false, obwohl das Binary ersetzt wurde")
	}

	status, err := CheckUpdate(projectDir)
	if err != nil {
		t.Fatalf("CheckUpdate nach Update: %v", err)
	}
	if status.Available {
		t.Error("nach dem Update wird weiterhin ein Update gemeldet")
	}
}

func TestUpdateOhneBinaeraenderung(t *testing.T) {
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
	if result.BinaryChanged {
		t.Error("BinaryChanged = true, obwohl sich nur Text geändert hat")
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

// Ein eingespielter Arbeitsstand ist gewollt. Er blockiert das Update trotzdem —
// aber er darf nicht als Handarbeit im Clone gemeldet werden, sonst stünde der
// Alarm dauerhaft da und würde wertlos.
func TestCheckCleanlinessErkenntEntwicklungsstand(t *testing.T) {
	projectDir, _ := newGitInstallation(t)

	// Zusätzlich eine echte Änderung: die Markierung muss sie überstimmen,
	// nicht neben ihr stehen.
	binary := filepath.Join(PlaybookDir(projectDir), "dist", "k-playbook-linux-amd64")
	if err := os.WriteFile(binary, []byte("eingespielt"), 0o755); err != nil {
		t.Fatalf("Binary ändern: %v", err)
	}
	marker := filepath.Join(PlaybookDir(projectDir), DevSyncMarker)
	if err := os.WriteFile(marker, []byte("Arbeitsstand\n"), 0o644); err != nil {
		t.Fatalf("Markierung anlegen: %v", err)
	}

	state := CheckCleanliness(projectDir)
	if !state.DevSync {
		t.Fatalf("Entwicklungsstand nicht erkannt: %+v", state)
	}
	if !state.Blocking() {
		t.Error("Entwicklungsstand blockiert das Update nicht")
	}
	if len(state.Modified) > 0 || len(state.Untracked) > 0 {
		t.Errorf("einzelne Dateien gemeldet statt des Zustands: %+v", state)
	}
	if !strings.Contains(state.Message, "Arbeitsstand verwerfen") {
		t.Errorf("Message nennt den Rückweg nicht: %q", state.Message)
	}
}

// Ohne Markierung bleibt es beim bisherigen Verhalten.
func TestUpdateLehntEntwicklungsstandAb(t *testing.T) {
	projectDir, _ := newGitInstallation(t)
	marker := filepath.Join(PlaybookDir(projectDir), DevSyncMarker)
	if err := os.WriteFile(marker, []byte("Arbeitsstand\n"), 0o644); err != nil {
		t.Fatalf("Markierung anlegen: %v", err)
	}

	result, err := Update(projectDir)
	if err == nil {
		t.Fatal("Update lief trotz Entwicklungsstand")
	}
	if !result.Cleanliness.DevSync {
		t.Errorf("Grund nicht durchgereicht: %+v", result.Cleanliness)
	}
}

func TestDiscardDevSyncStelltDenCloneWiederHer(t *testing.T) {
	projectDir, _ := newGitInstallation(t)
	playbookDir := PlaybookDir(projectDir)

	binary := filepath.Join(playbookDir, "dist", "k-playbook-linux-amd64")
	original, err := os.ReadFile(binary)
	if err != nil {
		t.Fatalf("Binary lesen: %v", err)
	}
	if err := os.WriteFile(binary, []byte("eingespielt"), 0o755); err != nil {
		t.Fatalf("Binary ändern: %v", err)
	}
	// Auch das Danebengelegte muss weg: bei einem eingespielten Arbeitsstand
	// gehört dort nichts hin, was nicht aus dem Arbeitsstand kommt.
	extra := filepath.Join(playbookDir, "notiz.txt")
	if err := os.WriteFile(extra, []byte("x"), 0o644); err != nil {
		t.Fatalf("Datei anlegen: %v", err)
	}
	marker := filepath.Join(playbookDir, DevSyncMarker)
	if err := os.WriteFile(marker, []byte("Arbeitsstand\n"), 0o644); err != nil {
		t.Fatalf("Markierung anlegen: %v", err)
	}

	if err := DiscardDevSync(projectDir); err != nil {
		t.Fatalf("DiscardDevSync: %v", err)
	}

	if restored, err := os.ReadFile(binary); err != nil || string(restored) != string(original) {
		t.Errorf("Binary nicht zurückgesetzt: %q, erwartet %q", restored, original)
	}
	if fileExists(extra) {
		t.Error("danebengelegte Datei blieb liegen")
	}
	if fileExists(marker) {
		t.Error("Markierung blieb liegen")
	}

	state := CheckCleanliness(projectDir)
	if !state.Clean || state.Blocking() {
		t.Errorf("Installation nach dem Verwerfen nicht sauber: %+v", state)
	}
}

// Ohne Markierung darf nichts verworfen werden: dann laesst sich nicht wissen,
// ob dort jemand absichtlich gearbeitet hat.
func TestDiscardDevSyncOhneMarkierung(t *testing.T) {
	projectDir, _ := newGitInstallation(t)
	binary := filepath.Join(PlaybookDir(projectDir), "dist", "k-playbook-linux-amd64")
	if err := os.WriteFile(binary, []byte("handarbeit"), 0o755); err != nil {
		t.Fatalf("Binary ändern: %v", err)
	}

	if err := DiscardDevSync(projectDir); err == nil {
		t.Fatal("ohne Markierung wurde verworfen")
	}

	content, err := os.ReadFile(binary)
	if err != nil || string(content) != "handarbeit" {
		t.Errorf("Handarbeit wurde angetastet: %q", content)
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
