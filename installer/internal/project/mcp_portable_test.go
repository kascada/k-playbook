package project

import (
	"os"
	"path/filepath"
	"testing"
)

// Task 062: Ersetzt ein selbsttätiger Weg einen veralteten Eintrag, wählt die
// Tracking-Messung je Zieldatei das Kommando — erfasst oder unbeantwortet der
// bloße Name, nicht erfasst der absolute Pfad.

// repairScopes sind die beiden selbsttätigen Wege: Clone-Update und Start.
var repairScopes = []struct {
	name  string
	scope MCPWriteScope
}{
	{"Update", MCPWriteOutdatedOnly},
	{"Start", MCPWriteOutdatedAndUnversioned},
}

// legacyWrappers sind die beiden Schreibweisen des abgelösten Wrappers.
var legacyWrappers = []struct {
	name    string
	command string
}{
	{"relativ", legacyWrapperCommand},
	{"absolut", "/home/wer/projekt/" + legacyWrapperCommand},
}

// outdatedContent ist ein veralteter Eintrag im Schema des Ziels.
func outdatedContent(schema MCPSchema, command string) string {
	if schema == MCPSchemaOpenCode {
		return `{"mcp":{"` + MCPServerKey + `":{"type":"local","command":["` + command + `","mcp"],"enabled":true}}}` + "\n"
	}
	return `{"mcpServers":{"` + MCPServerKey + `":{"command":"` + command + `","args":["mcp"]}}}` + "\n"
}

// targetPaths nennt die Pfade aller drei Ziele in der Reihenfolge von MCPTargets.
func targetPaths(root string) []string {
	paths := []string{}
	for _, target := range MCPTargets(root) {
		paths = append(paths, target.Path)
	}
	return paths
}

// entryCommand liest das eingetragene Kommando einer Zieldatei — auch durch
// einen Link hindurch.
func entryCommand(t *testing.T, root string, path string) string {
	t.Helper()

	for _, target := range MCPTargets(root) {
		if target.Path != path {
			continue
		}
		doc, exists, err := readJSONObject(filepath.Join(root, path))
		if err != nil || !exists {
			t.Fatalf("%s lesen: vorhanden %v, Fehler %v", path, exists, err)
		}
		section, ok := mcpSection(doc.content, target.Schema)
		if !ok {
			t.Fatalf("%s: Abschnitt %s ist kein Objekt", path, target.Schema)
		}
		command, _, ok := mcpEntryCommand(target.Schema, section[MCPServerKey])
		if !ok {
			t.Fatalf("%s: kein lesbarer Eintrag: %v", path, section[MCPServerKey])
		}
		return command
	}
	t.Fatalf("%s ist kein Ziel", path)
	return ""
}

// assertRepairWrote verlangt, dass genau paths geschrieben wurden, jeweils mit
// want — im Rückgabewert wie in der Datei.
func assertRepairWrote(t *testing.T, root string, writes []MCPWrite, paths []string, want string) {
	t.Helper()

	if len(writes) != len(paths) {
		t.Fatalf("geschrieben = %v, erwartet %v", writes, paths)
	}
	for i, write := range writes {
		if write.Path != paths[i] {
			t.Errorf("Schreibvorgang %d: Pfad %q, erwartet %q", i, write.Path, paths[i])
		}
		if write.Command != want {
			t.Errorf("%s: geschriebenes Kommando %q, erwartet %q", write.Path, write.Command, want)
		}
		if write.Portable() != (want == InstalledCommandName) {
			t.Errorf("%s: Portable() = %v bei Kommando %q", write.Path, write.Portable(), write.Command)
		}
		if got := entryCommand(t, root, write.Path); got != want {
			t.Errorf("%s: in der Datei steht %q, erwartet %q", write.Path, got, want)
		}
	}
}

// assertNoPingPong verlangt, dass ein Folgelauf nichts schreibt — in derselben
// Umgebung und in einer zweiten mit anderem HOME.
func assertNoPingPong(t *testing.T, root string, scope MCPWriteScope) {
	t.Helper()

	before := map[string]string{}
	for _, path := range targetPaths(root) {
		if file := filepath.Join(root, path); pathExists(file) {
			before[path] = readRaw(t, file)
		}
	}

	for _, environment := range []string{"dieselbe Umgebung", "zweite Umgebung"} {
		if environment == "zweite Umgebung" {
			installTestBinary(t)
		}
		repaired, err := RepairMCP(root, scope)
		if err != nil {
			t.Fatalf("%s: RepairMCP: %v", environment, err)
		}
		if len(repaired) != 0 {
			t.Errorf("%s: erneut geschrieben: %v", environment, repaired)
		}
	}

	for path, content := range before {
		if got := readRaw(t, filepath.Join(root, path)); got != content {
			t.Errorf("%s wurde vom Folgelauf verändert:\n%s", path, got)
		}
	}
}

// assertLsFilesExit verlangt den Exit-Code von ls-files --error-unmatch für
// path. Sonst trifft der Test die Regel nicht, die er belegen soll.
func assertLsFilesExit(t *testing.T, root string, path string, want int) {
	t.Helper()

	if _, code, reason := runGit(t.Context(), root, "ls-files", "--error-unmatch", "--", path); code != want {
		t.Fatalf("ls-files %s: Exit %d (%s), erwartet %d", path, code, reason, want)
	}
}

// Der Kern: eine erfasste Datei mit veraltetem Eintrag bekommt den bloßen
// Namen, in beiden Modi, für beide Wrapper-Formen und alle drei Schemata. Ein
// Folgelauf schreibt nichts, auch nicht aus einem anderen HOME.
func TestKorrekturSchreibtInErfassteDateiDenNamen(t *testing.T) {
	for _, scope := range repairScopes {
		for _, legacy := range legacyWrappers {
			t.Run(scope.name+"/"+legacy.name, func(t *testing.T) {
				root, _ := unversionedRepo(t)
				for _, target := range MCPTargets(root) {
					writeFile(t, filepath.Join(root, target.Path), outdatedContent(target.Schema, legacy.command))
				}
				gitRun(t, root, "add", "--all")
				gitRun(t, root, "commit", "-qm", "Registrierung")

				repaired, err := RepairMCP(root, scope.scope)
				if err != nil {
					t.Fatalf("RepairMCP: %v", err)
				}
				assertRepairWrote(t, root, repaired, targetPaths(root), InstalledCommandName)
				for _, status := range CheckMCP(root) {
					if !status.OK() {
						t.Errorf("%s nach der Korrektur nicht in Ordnung: %s (%s)", status.Path, status.State, status.Detail)
					}
				}
				assertNoPingPong(t, root, scope.scope)
			})
		}
	}
}

// Eine nicht erfasste Datei bekommt weiter den absoluten Pfad — belegt in einem
// echten Repository mit project.vcs git.
func TestKorrekturSchreibtInNichtErfassteDateiDenAbsolutenPfad(t *testing.T) {
	for _, scope := range repairScopes {
		for _, legacy := range legacyWrappers {
			t.Run(scope.name+"/"+legacy.name, func(t *testing.T) {
				root, installed := unversionedRepo(t)
				for _, target := range MCPTargets(root) {
					writeFile(t, filepath.Join(root, target.Path), outdatedContent(target.Schema, legacy.command))
				}
				assertLsFilesExit(t, root, ".mcp.json", 1)

				repaired, err := RepairMCP(root, scope.scope)
				if err != nil {
					t.Fatalf("RepairMCP: %v", err)
				}
				assertRepairWrote(t, root, repaired, targetPaths(root), installed)
				assertNoPingPong(t, root, scope.scope)
			})
		}
	}
}

// Ohne lesbare K-PLAYBOOK.yaml ist die Tracking-Frage unbeantwortet. Das gilt
// als erfasst, und beim Ersetzen heißt das: der bloße Name.
func TestKorrekturBeiUnbeantworteterTrackingFrageSchreibtDenNamen(t *testing.T) {
	for _, scope := range repairScopes {
		t.Run(scope.name, func(t *testing.T) {
			root, _ := unversionedRepo(t)
			if err := os.Remove(ConfigPath(root)); err != nil {
				t.Fatalf("Config entfernen: %v", err)
			}
			writeFile(t, filepath.Join(root, ".mcp.json"), veralteterEintrag)

			repaired, err := RepairMCP(root, scope.scope)
			if err != nil {
				t.Fatalf("RepairMCP: %v", err)
			}
			assertRepairWrote(t, root, repaired, []string{".mcp.json"}, InstalledCommandName)
		})
	}
}

// Hinter einem Link wird das Linkziel gemessen. ls-files meldet die Linkpfade
// als nicht erfasst (Exit 1); ohne die Regel landete der absolute Pfad in den
// erfassten Zieldateien. Dieselbe Einrichtung wird auch über ein verlinktes
// Hauptverzeichnis aufgerufen: verglichen wird physisch mit physisch.
func TestKorrekturHinterSymlinkAufErfassteDateiSchreibtDenNamen(t *testing.T) {
	for _, scope := range repairScopes {
		for _, viaLink := range []bool{false, true} {
			name := scope.name + "/Hauptverzeichnis"
			if viaLink {
				name += " über Symlink"
			}
			t.Run(name, func(t *testing.T) {
				root, _ := unversionedRepo(t)
				writeFile(t, filepath.Join(root, "real", "claude.json"), outdatedContent(MCPSchemaServers, legacyWrapperCommand))
				writeFile(t, filepath.Join(root, "real", "cursor", "mcp.json"), outdatedContent(MCPSchemaServers, legacyWrapperCommand))
				writeFile(t, filepath.Join(root, "real", "opencode.json"), outdatedContent(MCPSchemaOpenCode, legacyWrapperCommand))
				gitRun(t, root, "add", "--all")
				gitRun(t, root, "commit", "-qm", "Registrierung")

				links := map[string]string{
					".mcp.json":     filepath.Join("real", "claude.json"),
					".cursor":       filepath.Join("real", "cursor"),
					"opencode.json": filepath.Join("real", "opencode.json"),
				}
				for link, target := range links {
					makeSymlink(t, target, filepath.Join(root, link))
				}
				for _, path := range targetPaths(root) {
					assertLsFilesExit(t, root, path, 1)
				}

				start := root
				if viaLink {
					start = filepath.Join(t.TempDir(), "projekt")
					makeSymlink(t, root, start)
				}

				repaired, err := RepairMCP(start, scope.scope)
				if err != nil {
					t.Fatalf("RepairMCP: %v", err)
				}
				assertRepairWrote(t, start, repaired, targetPaths(start), InstalledCommandName)
				for link, target := range links {
					assertSymlink(t, filepath.Join(root, link), target)
				}
				assertNoPingPong(t, start, scope.scope)
			})
		}
	}
}

// Ein Linkziel im selben Repository, das nicht erfasst ist, bekommt den
// absoluten Pfad.
func TestKorrekturHinterSymlinkAufNichtErfassteDateiSchreibtDenAbsolutenPfad(t *testing.T) {
	for _, scope := range repairScopes {
		t.Run(scope.name, func(t *testing.T) {
			root, installed := unversionedRepo(t)
			writeFile(t, filepath.Join(root, "geteilt.json"), veralteterEintrag)
			link := filepath.Join(root, ".cursor", "mcp.json")
			makeSymlink(t, filepath.Join("..", "geteilt.json"), link)
			assertLsFilesExit(t, root, "geteilt.json", 1)

			repaired, err := RepairMCP(root, scope.scope)
			if err != nil {
				t.Fatalf("RepairMCP: %v", err)
			}
			assertRepairWrote(t, root, repaired, []string{filepath.Join(".cursor", "mcp.json")}, installed)
			assertSymlink(t, link, filepath.Join("..", "geteilt.json"))
		})
	}
}

// Liegt das Linkziel außerhalb des Repositorys des Hauptverzeichnisses — in
// keinem oder in einem verschachtelten —, ist die Frage unbeantwortet: der
// bloße Name. Im verschachtelten Repository meldete ls-files des äußeren
// sonst Exit 1, und der absolute Pfad landete in einem fremden Repository.
func TestKorrekturHinterSymlinkAusDemRepositorySchreibtDenNamen(t *testing.T) {
	for _, scope := range repairScopes {
		for _, place := range []string{"außerhalb", "verschachteltes Repository"} {
			t.Run(scope.name+"/"+place, func(t *testing.T) {
				root, _ := unversionedRepo(t)
				var shared string
				if place == "außerhalb" {
					shared = filepath.Join(t.TempDir(), "mcp.json")
				} else {
					nested := filepath.Join(root, "verschachtelt")
					if err := os.MkdirAll(nested, 0o755); err != nil {
						t.Fatalf("%s anlegen: %v", nested, err)
					}
					gitInit(t, nested)
					shared = filepath.Join(nested, "mcp.json")
				}
				writeFile(t, shared, veralteterEintrag)
				makeSymlink(t, shared, filepath.Join(root, ".cursor", "mcp.json"))

				repaired, err := RepairMCP(root, scope.scope)
				if err != nil {
					t.Fatalf("RepairMCP: %v", err)
				}
				assertRepairWrote(t, root, repaired, []string{filepath.Join(".cursor", "mcp.json")}, InstalledCommandName)
			})
		}
	}
}

// Die Symlink-Regel greift nur, wenn das Projekt git nutzt: bei project.vcs
// none bleibt die Datei auch hinter einem Link „nicht erfasst". Den Start
// belegt TestStartKorrigiertVeraltetenEintragHinterSymlink; hier beide Modi.
func TestKorrekturOhneVersionskontrolleHinterSymlinkSchreibtDenAbsolutenPfad(t *testing.T) {
	for _, scope := range repairScopes {
		t.Run(scope.name, func(t *testing.T) {
			installed := installTestBinary(t)
			root := t.TempDir()
			writeVCSConfig(t, root, "none")
			writeFile(t, filepath.Join(root, "geteilt.json"), veralteterEintrag)
			makeSymlink(t, filepath.Join("..", "geteilt.json"), filepath.Join(root, ".cursor", "mcp.json"))

			repaired, err := RepairMCP(root, scope.scope)
			if err != nil {
				t.Fatalf("RepairMCP: %v", err)
			}
			assertRepairWrote(t, root, repaired, []string{filepath.Join(".cursor", "mcp.json")}, installed)
		})
	}
}

// Wird das Hauptverzeichnis über einen Symlink-Pfad aufgerufen, ist eine
// reguläre, nicht erfasste Datei weiter nicht erfasst: absoluter Pfad.
func TestKorrekturUnterVerlinktemHauptverzeichnisSchreibtDenAbsolutenPfad(t *testing.T) {
	for _, scope := range repairScopes {
		t.Run(scope.name, func(t *testing.T) {
			root, installed := unversionedRepo(t)
			writeFile(t, filepath.Join(root, ".mcp.json"), veralteterEintrag)
			linkedRoot := filepath.Join(t.TempDir(), "projekt")
			makeSymlink(t, root, linkedRoot)

			repaired, err := RepairMCP(linkedRoot, scope.scope)
			if err != nil {
				t.Fatalf("RepairMCP: %v", err)
			}
			assertRepairWrote(t, linkedRoot, repaired, []string{".mcp.json"}, installed)
		})
	}
}

// Ergänzen bleibt, wie es war: ein fehlender Eintrag in einer nicht erfassten
// Datei bekommt den absoluten Pfad, auch wenn daneben eine erfasste Datei den
// bloßen Namen bekommt.
func TestErgaenzenBleibtBeimAbsolutenPfad(t *testing.T) {
	root, installed := unversionedRepo(t)
	writeFile(t, filepath.Join(root, ".mcp.json"), veralteterEintrag)
	gitRun(t, root, "add", ".mcp.json")
	gitRun(t, root, "commit", "-qm", "Registrierung")
	writeFile(t, filepath.Join(root, ".cursor", "mcp.json"), fremderEintrag)

	repaired, err := RepairMCP(root, MCPWriteOutdatedAndUnversioned)
	if err != nil {
		t.Fatalf("RepairMCP: %v", err)
	}
	if len(repaired) != 2 {
		t.Fatalf("geschrieben = %v, erwartet .mcp.json und .cursor/mcp.json", repaired)
	}
	assertRepairWrote(t, root, repaired[:1], []string{".mcp.json"}, InstalledCommandName)
	assertRepairWrote(t, root, repaired[1:], []string{filepath.Join(".cursor", "mcp.json")}, installed)
}
