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
		t.Skip("git nicht verfuegbar")
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

// Nur wenn sich die Binaries aendern, bringt ein Neustart eine andere Version.
func TestUpdateMeldetGeaenderteBinaries(t *testing.T) {
	projectDir, remoteDir := newGitInstallation(t)

	other := filepath.Join(t.TempDir(), "anderer")
	run(t, filepath.Dir(other), "git", "clone", remoteDir, other)
	run(t, other, "git", "config", "user.email", "test@example.com")
	run(t, other, "git", "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(other, "dist", "k-playbook-linux-amd64"), []byte("neu"), 0o755); err != nil {
		t.Fatalf("Binary aendern: %v", err)
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
		t.Error("BinaryChanged = true, obwohl sich nur Text geaendert hat")
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
