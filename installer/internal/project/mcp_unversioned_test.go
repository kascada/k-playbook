package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fremderEintrag ist eine .mcp.json, die dem Projekt gehört: ein fremder
// Server, kein eigener Eintrag — der Zustand MCPStateMissingEntry.
const fremderEintrag = `{"mcpServers":{"fremd":{"command":"anderes","args":["dienen"]}}}` + "\n"

// unversionedRepo baut ein Projekt mit installiertem k-playbook, echtem
// Repository und project.vcs git — der Ausgangspunkt für die Messung „nicht
// von git erfasst".
func unversionedRepo(t *testing.T) (root string, installed string) {
	t.Helper()

	installed = installTestBinary(t)
	root = t.TempDir()
	gitInit(t, root)
	writeVCSConfig(t, root, "git")
	gitRun(t, root, "config", "user.email", "test@example.invalid")
	gitRun(t, root, "config", "user.name", "Test")
	return root, installed
}

func readRaw(t *testing.T, path string) string {
	t.Helper()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s lesen: %v", path, err)
	}
	return string(raw)
}

func hasOwnEntry(t *testing.T, path string) bool {
	t.Helper()

	servers, ok := readJSON(t, path)["mcpServers"].(map[string]any)
	if !ok {
		return false
	}
	_, ok = servers[MCPServerKey]
	return ok
}

// Eine von git erfasste Datei bleibt beim Start unberührt: ein Eintrag dort
// machte den Arbeitsbaum jedes Klons dreckig und schriebe einen
// $HOME-gebundenen Pfad in eine geteilte Datei.
func TestStartLaesstErfassteDateiUnberuehrt(t *testing.T) {
	root, _ := unversionedRepo(t)
	file := filepath.Join(root, ".mcp.json")
	writeFile(t, file, fremderEintrag)
	gitRun(t, root, "add", ".mcp.json")
	gitRun(t, root, "commit", "-qm", "Registrierung")

	repaired, err := RepairMCP(root, MCPWriteOutdatedAndUnversioned)
	if err != nil {
		t.Fatalf("RepairMCP: %v", err)
	}
	if len(repaired) != 0 {
		t.Errorf("geschrieben = %v, erwartet nichts", repaired)
	}
	if readRaw(t, file) != fremderEintrag {
		t.Errorf("die erfasste Datei wurde verändert:\n%s", readRaw(t, file))
	}
}

// ls-files sieht auch den Index: eine gestagete, noch nicht committete Datei
// ist bereits erfasst.
func TestStartLaesstGestageteDateiUnberuehrt(t *testing.T) {
	root, _ := unversionedRepo(t)
	file := filepath.Join(root, ".mcp.json")
	writeFile(t, file, fremderEintrag)
	gitRun(t, root, "add", ".mcp.json")

	repaired, err := RepairMCP(root, MCPWriteOutdatedAndUnversioned)
	if err != nil {
		t.Fatalf("RepairMCP: %v", err)
	}
	if len(repaired) != 0 {
		t.Errorf("geschrieben = %v, erwartet nichts", repaired)
	}
	if readRaw(t, file) != fremderEintrag {
		t.Errorf("die gestagete Datei wurde verändert:\n%s", readRaw(t, file))
	}
}

// Eine nicht erfasste Datei bekommt den Eintrag — dieselbe Form wie über den
// Knopf, der fremde Eintrag bleibt stehen. Der zweite Lauf schreibt nicht mehr.
func TestStartErgaenztNichtErfassteDatei(t *testing.T) {
	root, installed := unversionedRepo(t)
	file := filepath.Join(root, ".mcp.json")
	writeFile(t, file, fremderEintrag)

	repaired, err := RepairMCP(root, MCPWriteOutdatedAndUnversioned)
	if err != nil {
		t.Fatalf("RepairMCP: %v", err)
	}
	if len(repaired) != 1 || repaired[0].Path != ".mcp.json" {
		t.Fatalf("geschrieben = %v, erwartet nur .mcp.json", repaired)
	}

	servers := readJSON(t, file)["mcpServers"].(map[string]any)
	entry, ok := servers[MCPServerKey].(map[string]any)
	if !ok || entry["command"] != installed {
		t.Errorf("eigener Eintrag = %v, erwartet den absoluten Pfad %s", servers[MCPServerKey], installed)
	}
	if fremd, ok := servers["fremd"].(map[string]any); !ok || fremd["command"] != "anderes" {
		t.Errorf("fremder Eintrag verändert oder verloren: %v", servers["fremd"])
	}
	// Die beiden anderen Dateien fehlten und haben keine Spur: sie entstehen nicht.
	for _, name := range []string{filepath.Join(".cursor", "mcp.json"), "opencode.json"} {
		if pathExists(filepath.Join(root, name)) {
			t.Errorf("%s wurde ohne Spur des Assistenten angelegt", name)
		}
	}

	first := readRaw(t, file)
	repaired, err = RepairMCP(root, MCPWriteOutdatedAndUnversioned)
	if err != nil {
		t.Fatalf("zweiter Lauf: %v", err)
	}
	if len(repaired) != 0 {
		t.Errorf("der zweite Lauf hat erneut geschrieben: %v", repaired)
	}
	if readRaw(t, file) != first {
		t.Errorf("der zweite Lauf hat die Datei verändert:\n%s", readRaw(t, file))
	}
}

// Ohne git ist nichts erfasst: project.vcs sagt es, und es wird geschrieben.
func TestStartSchreibtOhneVersionskontrolle(t *testing.T) {
	installTestBinary(t)
	root := t.TempDir()
	writeVCSConfig(t, root, "none")
	file := filepath.Join(root, ".mcp.json")
	writeFile(t, file, fremderEintrag)

	repaired, err := RepairMCP(root, MCPWriteOutdatedAndUnversioned)
	if err != nil {
		t.Fatalf("RepairMCP: %v", err)
	}
	if len(repaired) != 1 || repaired[0].Path != ".mcp.json" {
		t.Fatalf("geschrieben = %v, erwartet nur .mcp.json", repaired)
	}
	if !hasOwnEntry(t, file) {
		t.Error("eigener Eintrag fehlt")
	}
}

// project.vcs sagt git, aber das Hauptverzeichnis liegt in keinem Repository:
// an dieser Stelle ist nichts erfasst, es wird geschrieben.
func TestStartSchreibtAusserhalbDesRepos(t *testing.T) {
	installTestBinary(t)
	root := t.TempDir()
	skipIfRepositoryAround(t, root)
	writeVCSConfig(t, root, "git")
	file := filepath.Join(root, ".mcp.json")
	writeFile(t, file, fremderEintrag)

	repaired, err := RepairMCP(root, MCPWriteOutdatedAndUnversioned)
	if err != nil {
		t.Fatalf("RepairMCP: %v", err)
	}
	if len(repaired) != 1 || repaired[0].Path != ".mcp.json" {
		t.Fatalf("geschrieben = %v, erwartet nur .mcp.json", repaired)
	}
	if !hasOwnEntry(t, file) {
		t.Error("eigener Eintrag fehlt")
	}
}

// Ein git, das die Frage nicht beantwortet, sperrt: ls-files scheitert an
// einem defekten Index, und die Datei gilt als erfasst.
func TestStartSchreibtNichtBeiDefektemRepo(t *testing.T) {
	root, _ := unversionedRepo(t)
	writeFile(t, filepath.Join(root, ".git", "index"), "kein Index\n")
	if _, code, _ := runGit(t.Context(), root, "ls-files", "--error-unmatch", "--", ".mcp.json"); code <= 1 {
		t.Skip("git stört sich nicht am defekten Index")
	}
	file := filepath.Join(root, ".mcp.json")
	writeFile(t, file, fremderEintrag)

	repaired, err := RepairMCP(root, MCPWriteOutdatedAndUnversioned)
	if err != nil {
		t.Fatalf("RepairMCP: %v", err)
	}
	if len(repaired) != 0 {
		t.Errorf("geschrieben = %v, erwartet nichts", repaired)
	}
	if readRaw(t, file) != fremderEintrag {
		t.Errorf("trotz unbeantworteter Frage geschrieben:\n%s", readRaw(t, file))
	}
}

// Kommt git gar nicht zu Wort, ist die Frage ebenfalls unbeantwortet: nicht
// installiert heißt nicht „kein Repository".
func TestStartSchreibtNichtOhneGitImPfad(t *testing.T) {
	root, _ := unversionedRepo(t)
	file := filepath.Join(root, ".mcp.json")
	writeFile(t, file, fremderEintrag)
	t.Setenv("PATH", t.TempDir())

	repaired, err := RepairMCP(root, MCPWriteOutdatedAndUnversioned)
	if err != nil {
		t.Fatalf("RepairMCP: %v", err)
	}
	if len(repaired) != 0 {
		t.Errorf("geschrieben = %v, erwartet nichts", repaired)
	}
	if readRaw(t, file) != fremderEintrag {
		t.Errorf("ohne git im Pfad geschrieben:\n%s", readRaw(t, file))
	}
}

// Ohne lesbare K-PLAYBOOK.yaml ist die Frage nach der Versionierung
// unbeantwortet — auch in einem echten Repository, in dem die Datei nicht
// erfasst ist. Die sichere Richtung ist „erfasst".
func TestStartSchreibtNichtBeiUnlesbarerConfig(t *testing.T) {
	root, _ := unversionedRepo(t)
	if err := os.Remove(ConfigPath(root)); err != nil {
		t.Fatalf("Config entfernen: %v", err)
	}
	file := filepath.Join(root, ".mcp.json")
	writeFile(t, file, fremderEintrag)

	repaired, err := RepairMCP(root, MCPWriteOutdatedAndUnversioned)
	if err != nil {
		t.Fatalf("RepairMCP: %v", err)
	}
	if len(repaired) != 0 {
		t.Errorf("geschrieben = %v, erwartet nichts", repaired)
	}
	if readRaw(t, file) != fremderEintrag {
		t.Errorf("ohne Config geschrieben:\n%s", readRaw(t, file))
	}
}

// Eine fehlende Datei entsteht nur, wenn der Assistent eine eigene Spur im
// Projekt hat. Die Verzeichnisse und die von Links() verwalteten Einträge sind
// keine: die legt ApplyLinks in jedem Projekt an.
func TestStartLegtFehlendeDateiNurBeiSpurAn(t *testing.T) {
	installTestBinary(t)
	root := t.TempDir()
	writeVCSConfig(t, root, "none")
	for _, link := range Links() {
		if !link.IsInclude {
			if err := os.MkdirAll(filepath.Join(root, link.Path), 0o755); err != nil {
				t.Fatalf("%s anlegen: %v", link.Path, err)
			}
		}
	}

	repaired, err := RepairMCP(root, MCPWriteOutdatedAndUnversioned)
	if err != nil {
		t.Fatalf("RepairMCP: %v", err)
	}
	if len(repaired) != 0 {
		t.Errorf("ohne Spur geschrieben: %v", repaired)
	}
	for _, target := range MCPTargets(root) {
		if pathExists(filepath.Join(root, target.Path)) {
			t.Errorf("%s wurde ohne Spur des Assistenten angelegt", target.Path)
		}
	}

	// Jeder Assistent bekommt eine eigene Spur: alles entsteht.
	writeFile(t, filepath.Join(root, ".claude", "settings.json"), "{}\n")
	writeFile(t, filepath.Join(root, ".cursor", "rules", "eigene.mdc"), "# eigene\n")
	writeFile(t, filepath.Join(root, ".opencode", "eigene.md"), "# eigene\n")

	repaired, err = RepairMCP(root, MCPWriteOutdatedAndUnversioned)
	if err != nil {
		t.Fatalf("RepairMCP mit Spur: %v", err)
	}
	if len(repaired) != 3 {
		t.Errorf("geschrieben = %v, erwartet alle drei Ziele", repaired)
	}
	for _, status := range CheckMCP(root) {
		if !status.OK() {
			t.Errorf("%s nach dem Start nicht registriert: %s (%s)", status.Path, status.State, status.Detail)
		}
	}
}

// Ein vorhandener Eintrag, der weder veraltet noch akzeptiert ist, bleibt auch
// beim Start liegen: er kann aus einem fremden $HOME stammen.
func TestStartLaesstStaleLiegen(t *testing.T) {
	installTestBinary(t)
	root := t.TempDir()
	writeVCSConfig(t, root, "none")
	stale := `{"mcpServers":{"` + MCPServerKey + `":{"command":"/opt/fremd/dienst","args":["mcp"]}}}` + "\n"
	file := filepath.Join(root, ".mcp.json")
	writeFile(t, file, stale)

	repaired, err := RepairMCP(root, MCPWriteOutdatedAndUnversioned)
	if err != nil {
		t.Fatalf("RepairMCP: %v", err)
	}
	if len(repaired) != 0 {
		t.Errorf("ein fremder Stand wurde geschrieben: %v", repaired)
	}
	if readRaw(t, file) != stale {
		t.Errorf("ein fremder Stand wurde verändert:\n%s", readRaw(t, file))
	}
}

// Das Clone-Update behält den engen Modus: es zieht nur den abgelösten Wrapper
// nach und ergänzt keinen fehlenden Eintrag — auch nicht in einer nicht
// erfassten Datei mit Spur des Assistenten.
func TestUpdateErgaenztKeinenFehlendenEintrag(t *testing.T) {
	projectDir, _ := newGitInstallation(t)
	installTestBinary(t)
	writeVCSConfig(t, projectDir, "none")
	file := filepath.Join(projectDir, ".mcp.json")
	writeFile(t, file, fremderEintrag)
	writeFile(t, filepath.Join(projectDir, ".claude", "settings.json"), "{}\n")

	result, err := Update(projectDir)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(result.MCPRepaired) != 0 {
		t.Errorf("MCPRepaired = %v, erwartet nichts", result.MCPRepaired)
	}
	if readRaw(t, file) != fremderEintrag {
		t.Errorf("das Update hat den fehlenden Eintrag ergänzt:\n%s", readRaw(t, file))
	}
}

// veralteterEintrag trägt noch den abgelösten Wrapper — MCPStateOutdated.
const veralteterEintrag = `{"mcpServers":{"` + MCPServerKey + `":{"command":"k-playbook/bin/k-playbook","args":["mcp"]}}}` + "\n"

// sharedLinkTarget ist das Ziel des Links, den symlinkRepo anlegt.
var sharedLinkTarget = filepath.Join("..", ".mcp.json")

// symlinkRepo baut die Einrichtung aus dem Review nach v0.8.0: .mcp.json ist
// erfasst und trägt einen fremden Server, .cursor/ ist ignoriert, und
// .cursor/mcp.json ist ein Symlink auf die erfasste Datei. ls-files misst den
// Link (Exit 1), geschrieben würde ins Linkziel.
func symlinkRepo(t *testing.T) (root string, shared string, link string) {
	t.Helper()

	root, _ = unversionedRepo(t)
	writeFile(t, filepath.Join(root, ".gitignore"), ".cursor/\n")
	shared = filepath.Join(root, ".mcp.json")
	writeFile(t, shared, fremderEintrag)
	gitRun(t, root, "add", "--all")
	gitRun(t, root, "commit", "-qm", "Registrierung")

	link = filepath.Join(root, ".cursor", "mcp.json")
	makeSymlink(t, sharedLinkTarget, link)
	if _, code, reason := runGit(t.Context(), root, "ls-files", "--error-unmatch", "--", filepath.Join(".cursor", "mcp.json")); code != 1 {
		t.Fatalf("ls-files misst den Link nicht als unerfasst: Exit %d (%s)", code, reason)
	}
	return root, shared, link
}

// makeSymlink legt einen Link samt Elternverzeichnis an.
func makeSymlink(t *testing.T, target string, link string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatalf("%s anlegen: %v", filepath.Dir(link), err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Symlink %s anlegen: %v", link, err)
	}
}

// gitStatusShort liefert `git status --short`; leer heißt sauberer Arbeitsbaum.
func gitStatusShort(t *testing.T, root string) string {
	t.Helper()

	out, code, reason := runGit(t.Context(), root, "status", "--short")
	if code != 0 {
		t.Fatalf("git status: Exit %d (%s)", code, reason)
	}
	return out
}

// Ein nicht erfasster Symlink auf eine erfasste Datei: der Start übergeht ihn.
// Sonst änderte er über den Link die erfasste .mcp.json.
func TestStartUeberspringtSymlinkAufErfassteDatei(t *testing.T) {
	root, shared, link := symlinkRepo(t)

	repaired, err := RepairMCP(root, MCPWriteOutdatedAndUnversioned)
	if err != nil {
		t.Fatalf("RepairMCP: %v", err)
	}
	if len(repaired) != 0 {
		t.Errorf("geschrieben = %v, erwartet nichts", repaired)
	}
	if readRaw(t, shared) != fremderEintrag {
		t.Errorf("die erfasste Datei wurde über den Link verändert:\n%s", readRaw(t, shared))
	}
	if readRaw(t, link) != fremderEintrag {
		t.Errorf("der Link liefert einen anderen Inhalt:\n%s", readRaw(t, link))
	}
	assertSymlink(t, link, sharedLinkTarget)
	if status := gitStatusShort(t, root); status != "" {
		t.Errorf("git status nicht leer:\n%s", status)
	}
}

// Ein toter Link als Zieldatei ist ebenfalls ein Link: os.WriteFile legte die
// Datei sonst an seinem Ziel an, auch bei vorhandener Spur des Assistenten.
func TestStartLegtNichtsAmZielEinesTotenLinksAn(t *testing.T) {
	installTestBinary(t)
	root := t.TempDir()
	writeVCSConfig(t, root, "none")
	writeFile(t, filepath.Join(root, ".cursor", "rules", "eigene.mdc"), "# eigene\n")
	if err := os.MkdirAll(filepath.Join(root, "ziel"), 0o755); err != nil {
		t.Fatalf("ziel anlegen: %v", err)
	}
	link := filepath.Join(root, ".cursor", "mcp.json")
	makeSymlink(t, filepath.Join("..", "ziel", "mcp.json"), link)

	repaired, err := RepairMCP(root, MCPWriteOutdatedAndUnversioned)
	if err != nil {
		t.Fatalf("RepairMCP: %v", err)
	}
	if len(repaired) != 0 {
		t.Errorf("geschrieben = %v, erwartet nichts", repaired)
	}
	if pathExists(filepath.Join(root, "ziel", "mcp.json")) {
		t.Error("am Ziel des toten Links ist eine Datei entstanden")
	}
	assertSymlink(t, link, filepath.Join("..", "ziel", "mcp.json"))
}

// Ist das Assistenten-Verzeichnis selbst ein Link, führt der Weg ebenfalls über
// einen Symlink: es entsteht keine Datei im verlinkten Verzeichnis.
func TestStartSchreibtNichtInVerlinktesAssistentenVerzeichnis(t *testing.T) {
	installTestBinary(t)
	root := t.TempDir()
	writeVCSConfig(t, root, "none")
	sharedDir := filepath.Join(root, "geteilt-cursor")
	writeFile(t, filepath.Join(sharedDir, "rules", "eigene.mdc"), "# eigene\n")
	makeSymlink(t, "geteilt-cursor", filepath.Join(root, ".cursor"))

	repaired, err := RepairMCP(root, MCPWriteOutdatedAndUnversioned)
	if err != nil {
		t.Fatalf("RepairMCP: %v", err)
	}
	if len(repaired) != 0 {
		t.Errorf("geschrieben = %v, erwartet nichts", repaired)
	}
	if pathExists(filepath.Join(sharedDir, "mcp.json")) {
		t.Error("im verlinkten Assistenten-Verzeichnis ist mcp.json entstanden")
	}
}

// Liegt nur das Hauptverzeichnis unter einem Symlink-Pfad, ist das kein Grund
// zum Überspringen: geprüft werden die Bestandteile unterhalb davon.
func TestStartErgaenztUnterVerlinktemHauptverzeichnis(t *testing.T) {
	root, installed := unversionedRepo(t)
	writeFile(t, filepath.Join(root, ".mcp.json"), fremderEintrag)
	linkedRoot := filepath.Join(t.TempDir(), "projekt")
	makeSymlink(t, root, linkedRoot)

	repaired, err := RepairMCP(linkedRoot, MCPWriteOutdatedAndUnversioned)
	if err != nil {
		t.Fatalf("RepairMCP: %v", err)
	}
	if len(repaired) != 1 || repaired[0].Path != ".mcp.json" {
		t.Fatalf("geschrieben = %v, erwartet nur .mcp.json", repaired)
	}
	servers := readJSON(t, filepath.Join(root, ".mcp.json"))["mcpServers"].(map[string]any)
	if entry, ok := servers[MCPServerKey].(map[string]any); !ok || entry["command"] != installed {
		t.Errorf("eigener Eintrag = %v, erwartet %s", servers[MCPServerKey], installed)
	}
}

// Der Knopf ist eine ausdrückliche Handlung und schreibt weiter durch den Link.
func TestKnopfSchreibtDurchSymlink(t *testing.T) {
	root, shared, link := symlinkRepo(t)

	if _, err := ApplyMCP(root); err != nil {
		t.Fatalf("ApplyMCP: %v", err)
	}
	if !hasOwnEntry(t, link) {
		t.Error("über den Link fehlt der eigene Eintrag")
	}
	if !hasOwnEntry(t, shared) {
		t.Error("im Linkziel fehlt der eigene Eintrag")
	}
	assertSymlink(t, link, sharedLinkTarget)
}

// Das Clone-Update ergänzt auch über einen Link keinen fehlenden Eintrag.
// Update() ruft RepairMCP mit genau diesem Modus (update.go).
func TestUpdateLaesstSymlinkEinrichtungUnberuehrt(t *testing.T) {
	root, shared, link := symlinkRepo(t)

	repaired, err := RepairMCP(root, MCPWriteOutdatedOnly)
	if err != nil {
		t.Fatalf("RepairMCP: %v", err)
	}
	if len(repaired) != 0 {
		t.Errorf("geschrieben = %v, erwartet nichts", repaired)
	}
	if readRaw(t, shared) != fremderEintrag || readRaw(t, link) != fremderEintrag {
		t.Errorf("das Update hat die Symlink-Einrichtung verändert:\n%s", readRaw(t, shared))
	}
	assertSymlink(t, link, sharedLinkTarget)
	if status := gitStatusShort(t, root); status != "" {
		t.Errorf("git status nicht leer:\n%s", status)
	}
}

// Der veraltete Wrapper-Eintrag ist eigener Inhalt und wird auch hinter einem
// Link korrigiert — wie in v0.8.0.
func TestStartKorrigiertVeraltetenEintragHinterSymlink(t *testing.T) {
	installed := installTestBinary(t)
	root := t.TempDir()
	writeVCSConfig(t, root, "none")
	shared := filepath.Join(root, "geteilt.json")
	writeFile(t, shared, veralteterEintrag)
	link := filepath.Join(root, ".cursor", "mcp.json")
	makeSymlink(t, filepath.Join("..", "geteilt.json"), link)

	repaired, err := RepairMCP(root, MCPWriteOutdatedAndUnversioned)
	if err != nil {
		t.Fatalf("RepairMCP: %v", err)
	}
	if len(repaired) != 1 || repaired[0].Path != filepath.Join(".cursor", "mcp.json") {
		t.Fatalf("geschrieben = %v, erwartet nur .cursor/mcp.json", repaired)
	}
	servers := readJSON(t, shared)["mcpServers"].(map[string]any)
	if entry, ok := servers[MCPServerKey].(map[string]any); !ok || entry["command"] != installed {
		t.Errorf("Eintrag im Linkziel = %v, erwartet %s", servers[MCPServerKey], installed)
	}
	assertSymlink(t, link, filepath.Join("..", "geteilt.json"))
}

// skipIfRepositoryAround steigt aus, wenn das Testverzeichnis in einem
// Repository liegt oder im Verzeichnis oder darüber ein .git-Eintrag steht —
// über dieselben zwei Elternketten wie mcpTargetTracked. Dann gilt die Datei
// zu Recht als erfasst, und ein Test auf „außerhalb des Repos" belegt nichts.
func skipIfRepositoryAround(t *testing.T, root string) {
	t.Helper()

	if _, code, _ := runGitUntranslated(t.Context(), root, "rev-parse", "--show-toplevel"); code == 0 {
		t.Skip("das Testverzeichnis liegt in einem Git-Repository")
	}
	if gitEntryInAncestors(root) {
		t.Skip("im Testverzeichnis oder darüber liegt ein .git-Eintrag; die Datei gilt dann zu Recht als erfasst")
	}
}

// revParseFails verlangt, dass rev-parse im Verzeichnis mit Exit 128 scheitert,
// und liefert die erste stderr-Zeile. Sonst trifft der Test die Regel nicht.
func revParseFails(t *testing.T, dir string) string {
	t.Helper()

	_, code, reason := runGitUntranslated(t.Context(), dir, "rev-parse", "--show-toplevel")
	if code != 128 {
		t.Skipf("rev-parse endet hier nicht mit Exit 128, sondern mit %d (%s)", code, reason)
	}
	return reason
}

// assertNothingWritten führt den Start aus und verlangt, dass .mcp.json
// unverändert bleibt.
func assertNothingWritten(t *testing.T, root string, file string) {
	t.Helper()

	repaired, err := RepairMCP(root, MCPWriteOutdatedAndUnversioned)
	if err != nil {
		t.Fatalf("RepairMCP: %v", err)
	}
	if len(repaired) != 0 {
		t.Errorf("geschrieben = %v, erwartet nichts", repaired)
	}
	if readRaw(t, file) != fremderEintrag {
		t.Errorf("die Datei wurde verändert:\n%s", readRaw(t, file))
	}
}

// Nur die beiden ausdrücklichen Formen von „kein Repository" zählen. Der
// Wortlaut der Mount-Point-Form steht im Binary von git 2.53.0 (setup.c) und
// ist auf diesem Rechner nicht messbar — deshalb ein Test ohne git.
func TestGitReportsNoRepository(t *testing.T) {
	cases := []struct {
		line string
		want bool
	}{
		{"fatal: not a git repository (or any of the parent directories): .git", true},
		{"fatal: not a git repository (or any of the parent directories)", true},
		{"fatal: not a git repository (or any parent up to mount point /mnt/daten)", true},
		{"fatal: not a git repository: /pfad/fehlt", false},
		{"fatal: detected dubious ownership in repository at '/pfad'", false},
		{"", false},
	}
	for _, c := range cases {
		if got := gitReportsNoRepository(c.line); got != c.want {
			t.Errorf("gitReportsNoRepository(%q) = %v, erwartet %v", c.line, got, c.want)
		}
	}
}

// „dubious ownership" endet wie „kein Repository" mit Exit 128, ist aber ein
// Repository, dem git nur nicht traut. Die nicht erfasste Datei bleibt liegen:
// ob sie erfasst ist, ist unbeantwortet.
func TestStartSchreibtNichtBeiFremdemBesitzer(t *testing.T) {
	root, _ := unversionedRepo(t)
	file := filepath.Join(root, ".mcp.json")
	writeFile(t, file, fremderEintrag)
	t.Setenv("GIT_TEST_ASSUME_DIFFERENT_OWNER", "1")

	_, code, reason := runGitUntranslated(t.Context(), root, "rev-parse", "--show-toplevel")
	if code != 128 || !strings.Contains(reason, "dubious") {
		t.Skipf("git beachtet GIT_TEST_ASSUME_DIFFERENT_OWNER nicht: Exit %d (%s); Lücke 2 ist damit hier nicht belegt", code, reason)
	}

	assertNothingWritten(t, root, file)
}

// Eine .git-Datei, die ins Leere zeigt, ist ein kaputtes Repository, kein
// fehlendes: git meldet die kurze Form „not a git repository: <pfad>".
func TestStartSchreibtNichtBeiVerwaisterGitDatei(t *testing.T) {
	installTestBinary(t)
	root := t.TempDir()
	writeVCSConfig(t, root, "git")
	writeFile(t, filepath.Join(root, ".git"), "gitdir: "+filepath.Join(root, "fehlt")+"\n")
	file := filepath.Join(root, ".mcp.json")
	writeFile(t, file, fremderEintrag)
	revParseFails(t, root)

	assertNothingWritten(t, root, file)
}

// Ein .git-Verzeichnis ohne HEAD meldet wörtlich dieselbe Zeile wie ein
// Verzeichnis ohne Repository. Hier trägt allein der .git-Eintrag.
func TestStartSchreibtNichtBeiGitVerzeichnisOhneHead(t *testing.T) {
	installTestBinary(t)
	root := t.TempDir()
	skipIfRepositoryAround(t, root)
	writeVCSConfig(t, root, "git")
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf(".git anlegen: %v", err)
	}
	file := filepath.Join(root, ".mcp.json")
	writeFile(t, file, fremderEintrag)
	if reason := revParseFails(t, root); !gitReportsNoRepository(reason) {
		t.Skipf("git meldet hier nicht ausdrücklich „kein Repository“: %s", reason)
	}

	assertNothingWritten(t, root, file)
}

// git sucht über den physischen Pfad. Liegt ein .git ohne HEAD nur im
// physischen Elternverzeichnis und nicht in der Kette des Linkpfads, muss die
// Prüfung auch die aufgelöste Kette sehen.
func TestStartSchreibtNichtBeiGitEintragImPhysischenElternpfad(t *testing.T) {
	installTestBinary(t)
	base := t.TempDir()
	skipIfRepositoryAround(t, base)

	physical := filepath.Join(base, "physisch", "projekt")
	writeVCSConfig(t, physical, "git")
	file := filepath.Join(physical, ".mcp.json")
	writeFile(t, file, fremderEintrag)
	if err := os.MkdirAll(filepath.Join(base, "physisch", ".git"), 0o755); err != nil {
		t.Fatalf(".git anlegen: %v", err)
	}
	linked := filepath.Join(base, "logisch", "projekt")
	makeSymlink(t, physical, linked)
	if reason := revParseFails(t, linked); !gitReportsNoRepository(reason) {
		t.Skipf("git meldet hier nicht ausdrücklich „kein Repository“: %s", reason)
	}

	assertNothingWritten(t, linked, file)
}

// Rückschrittschutz: Mit deutscher Sprache in der Umgebung wird außerhalb des
// Repos weiter geschrieben, weil der Aufruf ohne Übersetzung läuft. Auf einem
// Rechner ohne deutsche git-Übersetzung belegt der Test nichts; das Log sagt,
// ob git übersetzt geantwortet hat.
func TestStartSchreibtAusserhalbDesReposMitDeutscherSprache(t *testing.T) {
	installTestBinary(t)
	root := t.TempDir()
	skipIfRepositoryAround(t, root)
	t.Setenv("LANG", "de_DE.UTF-8")
	t.Setenv("LANGUAGE", "de")
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_MESSAGES", "")
	writeVCSConfig(t, root, "git")
	file := filepath.Join(root, ".mcp.json")
	writeFile(t, file, fremderEintrag)

	_, _, translated := runGit(t.Context(), root, "rev-parse", "--show-toplevel")
	_, _, untranslated := runGitUntranslated(t.Context(), root, "rev-parse", "--show-toplevel")
	t.Logf("git übersetzt: %v (Sprache des Nutzers: %q, ohne Übersetzung: %q)", translated != untranslated, translated, untranslated)

	repaired, err := RepairMCP(root, MCPWriteOutdatedAndUnversioned)
	if err != nil {
		t.Fatalf("RepairMCP: %v", err)
	}
	if len(repaired) != 1 || repaired[0].Path != ".mcp.json" {
		t.Fatalf("geschrieben = %v, erwartet nur .mcp.json", repaired)
	}
	if !hasOwnEntry(t, file) {
		t.Error("eigener Eintrag fehlt")
	}
}
