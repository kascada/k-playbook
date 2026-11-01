package project

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// newInstallation legt ein Projekt mit Anker und Installation an.
func newInstallation(t *testing.T, projectDir string) {
	t.Helper()

	if err := os.MkdirAll(PlaybookDir(projectDir), 0o755); err != nil {
		t.Fatalf("Installation anlegen: %v", err)
	}
	writeFile(t, ConfigPath(projectDir), "schema_version: 2\n")
}

// resolved gleicht Symlinks an, damit Vergleiche unter /tmp auf macOS greifen.
func resolved(t *testing.T, dir string) string {
	t.Helper()

	path, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("EvalSymlinks(%s): %v", dir, err)
	}
	return path
}

func TestDiscoverFindetAnkerImVerzeichnisSelbst(t *testing.T) {
	root := t.TempDir()
	newInstallation(t, root)

	found, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if want := resolved(t, root); found != want {
		t.Errorf("Discover = %q, erwartet %q", found, want)
	}
}

func TestDiscoverFindetAnkerAusTieferUnterebene(t *testing.T) {
	root := t.TempDir()
	newInstallation(t, root)

	deep := filepath.Join(root, "src", "paket", "unter")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatalf("Unterverzeichnis anlegen: %v", err)
	}

	found, err := Discover(deep)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if want := resolved(t, root); found != want {
		t.Errorf("Discover = %q, erwartet %q", found, want)
	}
}

// Die Installation ist ein eigener Clone und damit ein eigener Git-Worktree.
// Ein Abbruch am Worktree-Root würde den Anker eine Ebene darüber verdecken.
func TestDiscoverLaeuftUeberGitWorktreeHinaus(t *testing.T) {
	root := t.TempDir()
	newInstallation(t, root)

	playbook := PlaybookDir(root)
	if err := os.MkdirAll(filepath.Join(playbook, ".git"), 0o755); err != nil {
		t.Fatalf(".git in der Installation anlegen: %v", err)
	}

	found, err := Discover(playbook)
	if err != nil {
		t.Fatalf("Discover aus der Installation heraus: %v", err)
	}
	if want := resolved(t, root); found != want {
		t.Errorf("Discover = %q, erwartet %q", found, want)
	}
}

func TestDiscoverMeldetKeinenFund(t *testing.T) {
	root := t.TempDir()

	_, err := Discover(root)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Fehler = %v, erwartet %v", err, ErrNotFound)
	}
}

func TestDetectFromLiefertBeideVerzeichnisse(t *testing.T) {
	root := t.TempDir()
	newInstallation(t, root)

	environment := DetectFrom(root)
	if !environment.Installed {
		t.Fatal("Installed = false, erwartet true")
	}
	if want := resolved(t, root); environment.ProjectDir != want {
		t.Errorf("ProjectDir = %q, erwartet %q", environment.ProjectDir, want)
	}
	if want := PlaybookDir(resolved(t, root)); environment.PlaybookDir != want {
		t.Errorf("PlaybookDir = %q, erwartet %q", environment.PlaybookDir, want)
	}
	if !environment.PlaybookPresent {
		t.Error("PlaybookPresent = false, erwartet true")
	}
}

// Nach einem frischen Clone des Projekts ist die Config da, die Installation
// aber noch nicht.
func TestDetectFromMeldetFehlendeInstallation(t *testing.T) {
	root := t.TempDir()
	writeFile(t, ConfigPath(root), "schema_version: 2\n")

	environment := DetectFrom(root)
	if !environment.Installed {
		t.Fatal("Installed = false, erwartet true")
	}
	if environment.PlaybookPresent {
		t.Error("PlaybookPresent = true, erwartet false")
	}
}

func TestDetectFromOhneInstallation(t *testing.T) {
	environment := DetectFrom(t.TempDir())
	if environment.Installed {
		t.Error("Installed = true, erwartet false")
	}
	if environment.ProjectDir != "" {
		t.Errorf("ProjectDir = %q, erwartet leer", environment.ProjectDir)
	}
}

// Ein Verzeichnis wird als Anker nur anerkannt, wenn K-PLAYBOOK.yaml eine Datei
// ist; ein gleichnamiges Verzeichnis darf nicht zählen.
func TestDiscoverIgnoriertGleichnamigesVerzeichnis(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(ConfigPath(root), 0o755); err != nil {
		t.Fatalf("Verzeichnis anlegen: %v", err)
	}

	if _, err := Discover(root); !errors.Is(err, ErrNotFound) {
		t.Errorf("Fehler = %v, erwartet %v", err, ErrNotFound)
	}
}

// Jede Installation trägt .k-playbook/ aus dem Quell-Repo. Das Verzeichnis
// allein ist kein Anker, sonst endete die Suche in der Installation.
func TestDiscoverUebergehtKPlaybookVerzeichnisOhneKonfiguration(t *testing.T) {
	root := t.TempDir()
	newInstallation(t, root)

	playbook := PlaybookDir(root)
	writeFile(t, filepath.Join(playbook, ProjectDirName, ".gitignore"), "/share\n")

	found, err := Discover(playbook)
	if err != nil {
		t.Fatalf("Discover aus der Installation heraus: %v", err)
	}
	if want := resolved(t, root); found != want {
		t.Errorf("Discover = %q, erwartet %q", found, want)
	}
}

func TestDiscoverZiehtAlteKonfigurationUm(t *testing.T) {
	root := t.TempDir()
	writeFile(t, legacyConfigPath(root), "schema_version: 3\n# alt\n")

	found, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if want := resolved(t, root); found != want {
		t.Errorf("Discover = %q, erwartet %q", found, want)
	}
	if got := readFile(t, ConfigPath(root)); got != "schema_version: 3\n# alt\n" {
		t.Errorf("Inhalt am neuen Ort = %q", got)
	}
	if _, err := os.Lstat(legacyConfigPath(root)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("alte Datei liegt noch: %v", err)
	}
}

// Auch aus einer Unterebene heraus wird umgezogen, bevor gelesen wird.
func TestDiscoverZiehtAlteKonfigurationAusUnterebeneUm(t *testing.T) {
	root := t.TempDir()
	writeFile(t, legacyConfigPath(root), "schema_version: 3\n")
	deep := filepath.Join(root, "src", "paket")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatalf("Unterverzeichnis anlegen: %v", err)
	}

	if _, err := Discover(deep); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if !fileExists(ConfigPath(root)) {
		t.Error("Konfiguration liegt nicht am neuen Ort")
	}
}

// Liegt am neuen Ort schon eine, gilt sie; die alte wird gelöscht.
func TestDiscoverLoeschtAlteKonfigurationNebenNeuer(t *testing.T) {
	root := t.TempDir()
	writeFile(t, ConfigPath(root), "schema_version: 3\n# neu\n")
	writeFile(t, legacyConfigPath(root), "schema_version: 3\n# alt\n")

	if _, err := Discover(root); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if got := readFile(t, ConfigPath(root)); got != "schema_version: 3\n# neu\n" {
		t.Errorf("Inhalt am neuen Ort = %q, erwartet die neue Datei", got)
	}
	if _, err := os.Lstat(legacyConfigPath(root)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("alte Datei liegt noch: %v", err)
	}
}

// Im Quell-Repo ist die alte Datei ein relativer Symlink nach
// k-playbook-local/. Umziehen muss die Datei dahinter; der verschobene Link
// zeigte ins Leere.
func TestDiscoverZiehtZielEinesSymlinksUm(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, LocalDirName, ConfigFileName)
	writeFile(t, real, "schema_version: 3\n# hinter dem Link\n")
	if err := os.Symlink(filepath.Join(LocalDirName, ConfigFileName), legacyConfigPath(root)); err != nil {
		t.Fatalf("Symlink anlegen: %v", err)
	}

	if _, err := Discover(root); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	info, err := os.Lstat(ConfigPath(root))
	if err != nil {
		t.Fatalf("neuer Ort: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Error("am neuen Ort liegt ein Link, erwartet die Datei selbst")
	}
	if got := readFile(t, ConfigPath(root)); got != "schema_version: 3\n# hinter dem Link\n" {
		t.Errorf("Inhalt am neuen Ort = %q", got)
	}
	for _, path := range []string{legacyConfigPath(root), real} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s liegt noch: %v", path, err)
		}
	}
}

// Neben einer neuen Datei wird nur der Link gelöscht, nie sein Ziel.
func TestDiscoverLoeschtNebenNeuerNurDenLink(t *testing.T) {
	root := t.TempDir()
	writeFile(t, ConfigPath(root), "schema_version: 3\n")
	real := filepath.Join(root, "anderswo.yaml")
	writeFile(t, real, "schema_version: 3\n# fremd\n")
	if err := os.Symlink(real, legacyConfigPath(root)); err != nil {
		t.Fatalf("Symlink anlegen: %v", err)
	}

	if _, err := Discover(root); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if _, err := os.Lstat(legacyConfigPath(root)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Link liegt noch: %v", err)
	}
	if !fileExists(real) {
		t.Error("Ziel des Links wurde gelöscht")
	}
}

// Scheitert der Umzug, meldet Discover das, statt „nicht gefunden“ zu sagen.
func TestDiscoverMeldetGescheitertenUmzug(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("als root greifen Dateirechte nicht")
	}
	root := t.TempDir()
	writeFile(t, legacyConfigPath(root), "schema_version: 3\n")
	if err := os.Chmod(root, 0o555); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(root, 0o755) })

	_, err := Discover(root)
	if err == nil || errors.Is(err, ErrNotFound) {
		t.Fatalf("Fehler = %v, erwartet einen gescheiterten Umzug", err)
	}
	if !fileExists(legacyConfigPath(root)) {
		t.Error("alte Datei ist weg, obwohl der Umzug scheiterte")
	}
}
