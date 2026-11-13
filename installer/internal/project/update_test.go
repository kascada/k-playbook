package project

import (
	"encoding/json"
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
	if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte("v0.0.1\n"), 0o644); err != nil {
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

// Der Pull holt den neuen Stand des Clones — auch eine bewegte VERSION —,
// sagt aber nichts mehr über Version und Installation des Programms: Die
// entscheiden sich am neuesten Release-Tag, nicht am Clone.
func TestUpdateHoltBewegteVersionOhneSieZuLesen(t *testing.T) {
	projectDir, remoteDir := newGitInstallation(t)

	other := filepath.Join(t.TempDir(), "anderer")
	run(t, filepath.Dir(other), "git", "clone", remoteDir, other)
	run(t, other, "git", "config", "user.email", "test@example.com")
	run(t, other, "git", "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(other, "VERSION"), []byte("v0.0.2\n"), 0o644); err != nil {
		t.Fatalf("VERSION ändern: %v", err)
	}
	run(t, other, "git", "add", "-A")
	run(t, other, "git", "commit", "-m", "neue Version")
	run(t, other, "git", "push")

	result, err := Update(projectDir)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if content := readRaw(t, filepath.Join(PlaybookDir(projectDir), "VERSION")); content != "v0.0.2\n" {
		t.Errorf("VERSION im Clone = %q, der Pull hat den neuen Stand nicht geholt", content)
	}
	// Die Antwort trägt nur, was der Pull selbst war.
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	for _, gone := range []string{"versionChanged", `"version"`, "mcpRepaired"} {
		if strings.Contains(string(encoded), gone) {
			t.Errorf("UpdateResult trägt noch %s: %s", gone, encoded)
		}
	}

	status, err := CheckUpdate(projectDir)
	if err != nil {
		t.Fatalf("CheckUpdate nach Update: %v", err)
	}
	if status.Available {
		t.Error("nach dem Update wird weiterhin ein Update gemeldet")
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

// Kein Bestandsprojekt darf mit einer kaputten MCP-Registrierung
// zurückbleiben. Der Pull erreicht die Datei nicht — sie liegt im
// Hauptverzeichnis, nicht im Clone —, und er repariert sie auch nicht mehr
// selbst: Das tun die Aufrufer über RepairMCPReport, ob der Pull gelang oder
// nicht.
//
// Das Hauptverzeichnis hat keine K-PLAYBOOK.yaml: die Tracking-Frage ist
// unbeantwortet, geschrieben wird seit Task 062 der bloße Name.
func TestRepairMCPReportKorrigiertVeralteteMCPRegistrierung(t *testing.T) {
	projectDir, _ := newGitInstallation(t)

	installTestBinary(t)
	stale := `{"mcpServers":{"` + MCPServerKey + `":{"command":"k-playbook/bin/k-playbook","args":["mcp"]}}}` + "\n"
	writeFile(t, filepath.Join(projectDir, ".mcp.json"), stale)

	if _, err := Update(projectDir); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if readRaw(t, filepath.Join(projectDir, ".mcp.json")) != stale {
		t.Fatal("der Pull hat die MCP-Registrierung selbst repariert")
	}

	report := RepairMCPReport(projectDir, MCPWriteOutdatedOnly)
	if len(report.Repaired) != 1 || report.Repaired[0] != ".mcp.json" || report.Message != "" {
		t.Fatalf("Repaired = %v, Message %q, erwartet [.mcp.json]", report.Repaired, report.Message)
	}

	status := mcpStatusFor(t, CheckMCP(projectDir), ".mcp.json")
	if !status.OK() {
		t.Fatalf("nach der Reparatur nicht korrigiert: %+v", status)
	}
	if status.Detail != "-> "+InstalledCommandName {
		t.Errorf("Detail = %q, erwartet den bloßen Namen %s", status.Detail, InstalledCommandName)
	}
	if len(report.Portable) != 1 || report.Portable[0] != ".mcp.json" {
		t.Errorf("Portable = %v, erwartet [.mcp.json]", report.Portable)
	}

	// Der zweite Lauf hat nichts zu tun.
	if again := RepairMCPReport(projectDir, MCPWriteOutdatedOnly); len(again.Repaired) != 0 || len(again.Portable) != 0 {
		t.Errorf("zweiter Lauf: %+v", again)
	}
}

func TestPublicFetchURL(t *testing.T) {
	cases := map[string]bool{
		"git@github.com:kascada/k-playbook.git":           true,
		"git@github.com:kascada/k-playbook":               true,
		"git@github.com:Kascada/K-Playbook.git":           true,
		"ssh://git@github.com/kascada/k-playbook.git":     true,
		"ssh://git@github.com/kascada/k-playbook/":        true,
		"https://github.com/kascada/k-playbook.git":       false,
		"git@github-kamranbycloud:kascada/k-playbook.git": false,
		"git@github.com:jemand/k-playbook.git":            false,
		"git@github.com:kascada/k-playbook-fork.git":      false,
		"/tmp/remote.git":                                 false,
	}
	for url, want := range cases {
		got, ok := publicFetchURL(url)
		if ok != want {
			t.Errorf("publicFetchURL(%q) ok = %v, erwartet %v", url, ok, want)
		}
		if ok && got != PublicRepoURL {
			t.Errorf("publicFetchURL(%q) = %q, erwartet %q", url, got, PublicRepoURL)
		}
	}
}

// withSSHRemote trägt im Clone die SSH-Adresse des offiziellen Repos ein und
// biegt die HTTPS-Adresse per insteadOf auf das lokale Remote um. So bleibt
// der Test ohne Netz, und ein Zugriff über SSH schlüge fehl.
func withSSHRemote(t *testing.T, projectDir string, remoteDir string) {
	t.Helper()

	dir := PlaybookDir(projectDir)
	run(t, dir, "git", "remote", "set-url", "origin", "git@github.com:kascada/k-playbook.git")
	run(t, dir, "git", "config", "url."+remoteDir+".insteadOf", PublicRepoURL)
}

func pushSecondCommit(t *testing.T, remoteDir string) {
	t.Helper()

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
}

func remoteURL(t *testing.T, projectDir string) string {
	t.Helper()

	cmd := exec.Command("git", "config", "--get", "remote.origin.url")
	cmd.Dir = PlaybookDir(projectDir)
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("remote.origin.url lesen: %v", err)
	}
	return string(output[:len(output)-1])
}

func TestUpdateStelltSSHRemoteAufHTTPSUm(t *testing.T) {
	projectDir, remoteDir := newGitInstallation(t)
	withSSHRemote(t, projectDir, remoteDir)
	pushSecondCommit(t, remoteDir)
	if err := SetInstallationReadOnly(projectDir); err != nil {
		t.Fatalf("Installation sperren: %v", err)
	}

	result, err := Update(projectDir)
	if err != nil {
		t.Fatalf("Update: %v\n%s", err, result.Output)
	}
	if result.RemoteSwitched != PublicRepoURL {
		t.Errorf("RemoteSwitched = %q, erwartet %q", result.RemoteSwitched, PublicRepoURL)
	}
	if got := remoteURL(t, projectDir); got != PublicRepoURL {
		t.Errorf("remote.origin.url = %q, erwartet %q", got, PublicRepoURL)
	}
	if !fileExists(filepath.Join(PlaybookDir(projectDir), "neu.txt")) {
		t.Error("Update hat den neuen Stand nicht eingespielt")
	}
	assertNotWritable(t, filepath.Join(PlaybookDir(projectDir), ".git", "config"))
}

func TestUpdateLaesstAndereRemotesStehen(t *testing.T) {
	projectDir, remoteDir := newGitInstallation(t)

	result, err := Update(projectDir)
	if err != nil {
		t.Fatalf("Update: %v\n%s", err, result.Output)
	}
	if result.RemoteSwitched != "" {
		t.Errorf("RemoteSwitched = %q, erwartet leer", result.RemoteSwitched)
	}
	if got := remoteURL(t, projectDir); got != remoteDir {
		t.Errorf("remote.origin.url = %q, erwartet %q", got, remoteDir)
	}
}

func TestCheckUpdateFragtSSHRemoteUeberHTTPS(t *testing.T) {
	projectDir, remoteDir := newGitInstallation(t)
	withSSHRemote(t, projectDir, remoteDir)

	// Gleicher Stand ist nur über das umgebogene lokale Remote zu haben: über
	// SSH wäre es nicht erreichbar oder das echte GitHub mit anderem Stand.
	status, err := CheckUpdate(projectDir)
	if err != nil {
		t.Fatalf("CheckUpdate: %v", err)
	}
	if status.Available || status.Remote == "" || status.Remote != status.Local {
		t.Errorf("nicht über das lokale Remote gefragt: %+v", status)
	}
	// Die Prüfung fasst den Clone nicht an.
	if got := remoteURL(t, projectDir); got != "git@github.com:kascada/k-playbook.git" {
		t.Errorf("remote.origin.url = %q, CheckUpdate hat umgestellt", got)
	}
}
