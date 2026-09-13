package project

import (
	"os"
	"path/filepath"
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
	if len(repaired) != 1 || repaired[0] != ".mcp.json" {
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
	if len(repaired) != 1 || repaired[0] != ".mcp.json" {
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
	if _, code, _ := runGit(t.Context(), root, "rev-parse", "--show-toplevel"); code == 0 {
		t.Skip("das Testverzeichnis liegt in einem Git-Repository")
	}
	writeVCSConfig(t, root, "git")
	file := filepath.Join(root, ".mcp.json")
	writeFile(t, file, fremderEintrag)

	repaired, err := RepairMCP(root, MCPWriteOutdatedAndUnversioned)
	if err != nil {
		t.Fatalf("RepairMCP: %v", err)
	}
	if len(repaired) != 1 || repaired[0] != ".mcp.json" {
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
