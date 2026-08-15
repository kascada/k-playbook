package project

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// privateRepo baut ein Projekt mit echtem Repository und angelegtem priv/ auf.
//
// core.excludesFile zeigt bewusst auf eine leere Datei im Repository: eine
// globale Ignore-Datei des Rechners würde sonst in die Messung hineinreichen
// und den Test vom Entwicklungsrechner abhängig machen.
func privateRepo(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	gitInit(t, root)
	writeVCSConfig(t, root, "git")
	writeFile(t, filepath.Join(root, ".git", "leere-excludes"), "")
	gitRun(t, root, "config", "core.excludesFile", filepath.Join(root, ".git", "leere-excludes"))
	gitRun(t, root, "config", "user.email", "test@example.invalid")
	gitRun(t, root, "config", "user.name", "Test")
	writeFile(t, filepath.Join(LocalDir(root), "priv", "README.md"), "# priv\n")
	return root
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()

	command := append([]string{"-C", dir}, args...)
	if output, err := exec.Command("git", command...).CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v — %s", strings.Join(args, " "), err, output)
	}
}

// privStatus misst priv/ — das Verzeichnis, um das es in allen Fällen geht.
func privStatus(t *testing.T, root string) PrivacyStatus {
	t.Helper()

	entry, ok := PrivateEntry("priv")
	if !ok {
		t.Fatal("priv steht nicht als privates Verzeichnis in der lokalen Struktur")
	}
	return PrivacyStatusFor(root, entry)
}

// writeManagedIgnore legt die verwaltete Datei an, so wie SetPrivate es täte.
func writeManagedIgnore(t *testing.T, root string) {
	t.Helper()

	writeFile(t, filepath.Join(LocalDir(root), "priv", PrivateIgnoreFile), managedIgnoreContent())
}

// Beide privaten Verzeichnisse stehen zur Wahl — und nur sie.
func TestPrivateEntriesNenntPrivUndMaterial(t *testing.T) {
	paths := []string{}
	for _, entry := range PrivateEntries() {
		paths = append(paths, entry.Path)
	}

	if len(paths) != 2 || paths[0] != "priv" || paths[1] != "material" {
		t.Errorf("PrivateEntries = %v, erwartet [priv material]", paths)
	}
	if _, ok := PrivateEntry("rules"); ok {
		t.Error("rules gilt als privates Verzeichnis")
	}
}

func TestPrivacyStatusOhneRegel(t *testing.T) {
	root := privateRepo(t)

	status := privStatus(t, root)
	if status.State != PrivacyPublic {
		t.Fatalf("State = %q, erwartet %q (%s)", status.State, PrivacyPublic, status.Reason)
	}
	if status.Rule != nil {
		t.Errorf("Regel gemeldet, obwohl keine greift: %+v", status.Rule)
	}
	if !status.CanToggle {
		t.Errorf("nicht umschaltbar, obwohl nichts im Weg steht: %s", status.Blocked)
	}
	if !samePath(status.RepoRoot, root) {
		t.Errorf("RepoRoot = %q, erwartet %q", status.RepoRoot, root)
	}
}

// Der Sollzustand: Regel greift, weder Index noch HEAD tragen erfasste Dateien.
// README.md und die .gitignore selbst zählen nicht mit — der verwaltete Inhalt
// lässt sie ausdrücklich drin.
func TestPrivacyStatusPrivat(t *testing.T) {
	root := privateRepo(t)
	writeManagedIgnore(t, root)
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-qm", "Struktur")
	writeFile(t, filepath.Join(LocalDir(root), "priv", "notiz.md"), "geheim\n")

	status := privStatus(t, root)
	if status.State != PrivacyPrivate {
		t.Fatalf("State = %q, erwartet %q (%s, tracked=%v, head=%v)",
			status.State, PrivacyPrivate, status.Reason, status.Tracked, status.InHead)
	}
	if !status.Managed || !status.CanToggle {
		t.Errorf("Managed = %v, CanToggle = %v — die Regel steht in der verwalteten Datei", status.Managed, status.CanToggle)
	}
	if status.Rule == nil || status.Rule.Pattern != "*" {
		t.Errorf("Regel = %+v, erwartet das Muster *", status.Rule)
	}
}

// Der erste gefährliche Zustand: sieht privat aus, ist es nicht.
func TestPrivacyStatusTeilweisePrivat(t *testing.T) {
	root := privateRepo(t)
	writeFile(t, filepath.Join(LocalDir(root), "priv", "notiz.md"), "geheim\n")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-qm", "mit Notiz")
	writeManagedIgnore(t, root)

	status := privStatus(t, root)
	if status.State != PrivacyPartial {
		t.Fatalf("State = %q, erwartet %q (%s)", status.State, PrivacyPartial, status.Reason)
	}
	if len(status.Tracked) != 1 || status.Tracked[0] != "notiz.md" {
		t.Errorf("Tracked = %v, erwartet nur notiz.md — README.md gehört nicht dazu", status.Tracked)
	}
}

// Der zweite gefährliche Zustand: git rm --cached ist nur gestaget, ohne Commit
// trägt jeder Clone die Dateien weiter.
func TestPrivacyStatusPrivatErstNachCommit(t *testing.T) {
	root := privateRepo(t)
	writeFile(t, filepath.Join(LocalDir(root), "priv", "notiz.md"), "geheim\n")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-qm", "mit Notiz")
	writeManagedIgnore(t, root)
	gitRun(t, root, "rm", "--cached", "--quiet", filepath.Join(LocalDirName, "priv", "notiz.md"))

	status := privStatus(t, root)
	if status.State != PrivacyPendingCommit {
		t.Fatalf("State = %q, erwartet %q (%s)", status.State, PrivacyPendingCommit, status.Reason)
	}
	if len(status.Tracked) != 0 {
		t.Errorf("Tracked = %v, erwartet leer", status.Tracked)
	}
	if len(status.InHead) != 1 || status.InHead[0] != "notiz.md" {
		t.Errorf("InHead = %v, erwartet notiz.md", status.InHead)
	}
}

// Ein frisch initialisiertes Repository hat kein HEAD. Das ist kein Fehler,
// sondern „HEAD ist leer" — sonst stünde es als „nicht ermittelbar" da.
func TestPrivacyStatusRepoOhneCommits(t *testing.T) {
	root := privateRepo(t)
	writeManagedIgnore(t, root)

	status := privStatus(t, root)
	if status.State != PrivacyPrivate {
		t.Fatalf("State = %q, erwartet %q (%s)", status.State, PrivacyPrivate, status.Reason)
	}
}

// Stammt die Regel von woanders, wird der Zustand angezeigt, aber nicht
// umgeschaltet: geschrieben wird nur die eine verwaltete Datei.
func TestPrivacyStatusFremdeRegelquelle(t *testing.T) {
	root := privateRepo(t)
	writeFile(t, filepath.Join(root, ".gitignore"), LocalDirName+"/priv/\n")

	status := privStatus(t, root)
	if status.State != PrivacyPrivate {
		t.Fatalf("State = %q, erwartet %q (%s)", status.State, PrivacyPrivate, status.Reason)
	}
	if status.Managed || status.CanToggle {
		t.Errorf("Managed = %v, CanToggle = %v — die Regel stammt aus der Projekt-.gitignore", status.Managed, status.CanToggle)
	}
	if !strings.Contains(status.Blocked, ".gitignore") {
		t.Errorf("Blocked nennt die fremde Quelle nicht: %q", status.Blocked)
	}
}

// Dieselbe Datei mit fremdem Inhalt gehört ebenfalls dem Projekt.
func TestPrivacyStatusFremderInhaltInDerVerwaltetenDatei(t *testing.T) {
	root := privateRepo(t)
	writeFile(t, filepath.Join(LocalDir(root), "priv", PrivateIgnoreFile), "*.log\n")

	status := privStatus(t, root)
	if status.State != PrivacyPublic {
		t.Fatalf("State = %q, erwartet %q — *.log erfasst den Inhalt nicht", status.State, PrivacyPublic)
	}
	if status.CanToggle {
		t.Error("umschaltbar, obwohl die Datei eigenen Inhalt trägt")
	}
	if !strings.Contains(status.Blocked, PrivateIgnoreFile) {
		t.Errorf("Blocked nennt die Datei nicht: %q", status.Blocked)
	}
}

// Liegt in k-playbook-local ein eigenes Repository, gilt die Aussage für dieses
// — deshalb gehört der Repo-Root in den Zustand.
func TestPrivacyStatusNenntDasNaechstgelegeneRepo(t *testing.T) {
	root := privateRepo(t)
	local := LocalDir(root)
	gitInit(t, local)
	writeManagedIgnore(t, root)

	status := privStatus(t, root)
	if !samePath(status.RepoRoot, local) {
		t.Errorf("RepoRoot = %q, erwartet das innere Repository %q", status.RepoRoot, local)
	}
	if status.State != PrivacyPrivate {
		t.Errorf("State = %q, erwartet %q (%s)", status.State, PrivacyPrivate, status.Reason)
	}
}

// Ohne git gibt es nichts zu messen — und keinen Fehler.
func TestPrivacyStatusOhneVersionskontrolle(t *testing.T) {
	root := t.TempDir()
	writeVCSConfig(t, root, "none")
	writeFile(t, filepath.Join(LocalDir(root), "priv", "README.md"), "# priv\n")

	status := privStatus(t, root)
	if status.State != PrivacyNoVCS {
		t.Fatalf("State = %q, erwartet %q", status.State, PrivacyNoVCS)
	}
	if status.Reason == "" || status.CanToggle {
		t.Errorf("Reason = %q, CanToggle = %v", status.Reason, status.CanToggle)
	}
}

// Ein noch nicht angelegtes Verzeichnis ist ein eigener Zustand, kein Fehler.
func TestPrivacyStatusOhneVerzeichnis(t *testing.T) {
	root := t.TempDir()
	writeVCSConfig(t, root, "git")

	status := privStatus(t, root)
	if status.State != PrivacyMissing {
		t.Fatalf("State = %q, erwartet %q", status.State, PrivacyMissing)
	}
	if status.CanToggle {
		t.Error("umschaltbar, obwohl das Verzeichnis fehlt")
	}
}

// git sagt „kein Repository" mit Exit 128. Das ist „nicht ermittelbar" mit
// Grund, kein Fehler an die Oberfläche.
func TestPrivacyStatusOhneRepository(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("kein git im Pfad")
	}

	root := t.TempDir()
	writeVCSConfig(t, root, "git")
	writeFile(t, filepath.Join(LocalDir(root), "priv", "README.md"), "# priv\n")

	status := privStatus(t, root)
	if status.State != PrivacyUnknown {
		t.Fatalf("State = %q, erwartet %q", status.State, PrivacyUnknown)
	}
	if status.Reason == "" {
		t.Error("Reason fehlt")
	}
	if status.CanToggle {
		t.Error("umschaltbar, obwohl es kein Repository gibt")
	}
}

// setPriv schaltet priv/ um.
func setPriv(t *testing.T, root string, private bool) (PrivacyChange, error) {
	t.Helper()

	entry, ok := PrivateEntry("priv")
	if !ok {
		t.Fatal("priv steht nicht als privates Verzeichnis in der lokalen Struktur")
	}
	return SetPrivate(root, entry, private)
}

// Der einfache Weg: nichts ist getrackt, es entsteht nur die verwaltete Datei.
func TestSetPrivateLegtVerwalteteDateiAn(t *testing.T) {
	root := privateRepo(t)

	change, err := setPriv(t, root, true)
	if err != nil {
		t.Fatalf("SetPrivate: %v", err)
	}
	if !change.Changed || change.Status.State != PrivacyPrivate {
		t.Fatalf("Changed = %v, State = %q — erwartet %q (%s)",
			change.Changed, change.Status.State, PrivacyPrivate, change.Message)
	}
	if len(change.Untracked) != 0 {
		t.Errorf("Untracked = %v, erwartet leer", change.Untracked)
	}
	if !hasManagedContent(filepath.Join(LocalDir(root), "priv", PrivateIgnoreFile)) {
		t.Error("die verwaltete .gitignore steht nicht mit ihrem Inhalt da")
	}
}

// Getrackte Dateien gehören zur selben Operation: sonst bliebe ein Zustand
// stehen, der privat aussieht und keiner ist.
func TestSetPrivateNimmtGetrackteDateienAusDemIndex(t *testing.T) {
	root := privateRepo(t)
	writeFile(t, filepath.Join(LocalDir(root), "priv", "notiz.md"), "geheim\n")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-qm", "mit Notiz")

	change, err := setPriv(t, root, true)
	if err != nil {
		t.Fatalf("SetPrivate: %v", err)
	}
	if len(change.Untracked) != 1 || change.Untracked[0] != "notiz.md" {
		t.Fatalf("Untracked = %v, erwartet notiz.md", change.Untracked)
	}
	if change.Status.State != PrivacyPendingCommit {
		t.Errorf("State = %q, erwartet %q — die Löschung ist nur gestaget", change.Status.State, PrivacyPendingCommit)
	}
	if !strings.Contains(change.Message, "Commit") {
		t.Errorf("Message nennt den ausstehenden Commit nicht: %q", change.Message)
	}

	// README.md bleibt versioniert — der verwaltete Inhalt lässt sie drin.
	if readFile(t, filepath.Join(LocalDir(root), "priv", "notiz.md")) != "geheim\n" {
		t.Error("die Datei wurde aus dem Arbeitsverzeichnis entfernt; --cached darf das nicht")
	}
	tracked, detail := listTracked(t, root)
	if detail != "" {
		t.Fatalf("ls-files: %s", detail)
	}
	if !contains(tracked, filepath.Join(LocalDirName, "priv", "README.md")) {
		t.Errorf("README.md ist nicht mehr versioniert: %v", tracked)
	}
	if contains(tracked, filepath.Join(LocalDirName, "priv", "notiz.md")) {
		t.Errorf("notiz.md steht weiterhin im Index: %v", tracked)
	}
}

// Nach dem Commit ist der Zustand privat, ohne dass etwas weiter geschieht.
func TestSetPrivateIstNachDemCommitPrivat(t *testing.T) {
	root := privateRepo(t)
	writeFile(t, filepath.Join(LocalDir(root), "priv", "notiz.md"), "geheim\n")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-qm", "mit Notiz")

	if _, err := setPriv(t, root, true); err != nil {
		t.Fatalf("SetPrivate: %v", err)
	}
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-qm", "privat")

	if state := privStatus(t, root).State; state != PrivacyPrivate {
		t.Errorf("State = %q, erwartet %q", state, PrivacyPrivate)
	}
}

// Zweimal einschalten ändert nichts und antwortet dasselbe.
func TestSetPrivateIstIdempotent(t *testing.T) {
	root := privateRepo(t)

	if _, err := setPriv(t, root, true); err != nil {
		t.Fatalf("erster Lauf: %v", err)
	}
	change, err := setPriv(t, root, true)
	if err != nil {
		t.Fatalf("zweiter Lauf: %v", err)
	}
	if change.Changed {
		t.Error("zweiter Lauf meldet eine Änderung")
	}
	if change.Status.State != PrivacyPrivate {
		t.Errorf("State = %q, erwartet %q", change.Status.State, PrivacyPrivate)
	}
}

func TestSetPrivateAusSchaltenEntferntNurDieVerwalteteDatei(t *testing.T) {
	root := privateRepo(t)
	if _, err := setPriv(t, root, true); err != nil {
		t.Fatalf("einschalten: %v", err)
	}

	change, err := setPriv(t, root, false)
	if err != nil {
		t.Fatalf("ausschalten: %v", err)
	}
	if !change.Changed || change.Status.State != PrivacyPublic {
		t.Fatalf("Changed = %v, State = %q, erwartet %q", change.Changed, change.Status.State, PrivacyPublic)
	}
	if pathExists(filepath.Join(LocalDir(root), "priv", PrivateIgnoreFile)) {
		t.Error("die verwaltete .gitignore steht noch da")
	}
	if !fileExists(filepath.Join(LocalDir(root), "priv", "README.md")) {
		t.Error("README.md wurde mit entfernt")
	}
}

// Ausschalten ohne eingeschalteten Zustand ist ebenfalls idempotent.
func TestSetPrivateAusSchaltenOhneRegel(t *testing.T) {
	root := privateRepo(t)

	change, err := setPriv(t, root, false)
	if err != nil {
		t.Fatalf("SetPrivate: %v", err)
	}
	if change.Changed {
		t.Error("es wurde etwas geändert, obwohl keine Regel greift")
	}
}

// Stammt die Regel von woanders, wird nichts geschrieben — weder beim Ein- noch
// beim Ausschalten.
func TestSetPrivateSchreibtNichtBeiFremderQuelle(t *testing.T) {
	root := privateRepo(t)
	writeFile(t, filepath.Join(root, ".gitignore"), LocalDirName+"/priv/\n")

	change, err := setPriv(t, root, false)
	if err == nil {
		t.Fatalf("kein Fehler, obwohl die Regel von woanders stammt: %+v", change)
	}
	if !strings.Contains(err.Error(), ".gitignore") {
		t.Errorf("Fehlertext nennt die fremde Quelle nicht: %v", err)
	}
	if !fileExists(filepath.Join(root, ".gitignore")) {
		t.Error("die fremde .gitignore wurde entfernt")
	}
}

// Eine Datei mit eigenem Inhalt gehört dem Projekt und wird nicht überschrieben.
func TestSetPrivateUeberschreibtKeineFremdeDatei(t *testing.T) {
	root := privateRepo(t)
	eigen := filepath.Join(LocalDir(root), "priv", PrivateIgnoreFile)
	writeFile(t, eigen, "*.log\n")

	if _, err := setPriv(t, root, true); err == nil {
		t.Fatal("kein Fehler, obwohl die Datei eigenen Inhalt trägt")
	}
	if readFile(t, eigen) != "*.log\n" {
		t.Errorf("die Datei wurde verändert: %q", readFile(t, eigen))
	}
}

// Ohne git gibt es nichts umzuschalten.
func TestSetPrivateOhneVersionskontrolle(t *testing.T) {
	root := t.TempDir()
	writeVCSConfig(t, root, "none")
	writeFile(t, filepath.Join(LocalDir(root), "priv", "README.md"), "# priv\n")

	if _, err := setPriv(t, root, true); err == nil {
		t.Fatal("kein Fehler, obwohl das Projekt kein git benutzt")
	}
	if pathExists(filepath.Join(LocalDir(root), "priv", PrivateIgnoreFile)) {
		t.Error("es wurde trotzdem eine .gitignore geschrieben")
	}
}

// Andere Verzeichnisse stehen nicht zur Wahl, auch nicht auf Zuruf.
func TestSetPrivateLehntFremdenEintragAb(t *testing.T) {
	root := privateRepo(t)

	if _, err := SetPrivate(root, LocalEntry{Path: "rules"}, true); err == nil {
		t.Fatal("kein Fehler für ein Verzeichnis ohne Private-Markierung")
	}
	if pathExists(filepath.Join(LocalDir(root), "rules", PrivateIgnoreFile)) {
		t.Error("es wurde trotzdem eine .gitignore geschrieben")
	}
}

func listTracked(t *testing.T, root string) ([]string, string) {
	t.Helper()

	out, code, detail := runGit(context.Background(), root, "ls-files", "-z")
	if code != 0 {
		return nil, detail
	}
	return splitNUL(out), ""
}

func contains(paths []string, want string) bool {
	for _, path := range paths {
		if path == want {
			return true
		}
	}
	return false
}

// Nur genau der verwaltete Inhalt zählt als verwaltet.
func TestHasManagedContent(t *testing.T) {
	root := t.TempDir()

	cases := map[string]bool{
		"*\n!.gitignore\n!README.md\n":         true,
		"*\n\n!.gitignore\n!README.md":         true,
		"*\n!.gitignore\n!README.md\n!eigen\n": false,
		"*\n!README.md\n!.gitignore\n":         false,
		"*\n":                                  false,
	}
	for content, want := range cases {
		path := filepath.Join(root, "probe")
		writeFile(t, path, content)
		if got := hasManagedContent(path); got != want {
			t.Errorf("hasManagedContent(%q) = %v, erwartet %v", content, got, want)
		}
	}
}
