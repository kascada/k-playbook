package project

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func readFile(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s lesen: %v", path, err)
	}
	return string(data)
}

func symlink(t *testing.T, path string, destination string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("%s anlegen: %v", filepath.Dir(path), err)
	}
	if err := os.Symlink(destination, path); err != nil {
		t.Fatalf("%s verlinken: %v", path, err)
	}
}

func mkdir(t *testing.T, path string) {
	t.Helper()

	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("%s anlegen: %v", path, err)
	}
}

// snapshotTree liest den ganzen Projektbaum als Vergleichswert: Verzeichnisse,
// Symlink-Ziele und Dateiinhalte. Symlinks werden nicht verfolgt.
func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()

	snapshot := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			destination, err := os.Readlink(path)
			if err != nil {
				return err
			}
			snapshot[relative] = "-> " + destination
		case entry.IsDir():
			snapshot[relative] = "<dir>"
		case info.Mode().IsRegular():
			snapshot[relative] = readFile(t, path)
		default:
			// Nicht lesen: an einer FIFO bliebe der Test hängen.
			snapshot[relative] = "<" + info.Mode().String() + ">"
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Projektbaum lesen: %v", err)
	}
	return snapshot
}

func assertUnchanged(t *testing.T, before map[string]string, after map[string]string) {
	t.Helper()

	if reflect.DeepEqual(before, after) {
		return
	}

	names := map[string]bool{}
	for name := range before {
		names[name] = true
	}
	for name := range after {
		names[name] = true
	}

	changed := []string{}
	for name := range names {
		if before[name] != after[name] {
			changed = append(changed, name+": "+before[name]+" -> "+after[name])
		}
	}
	sort.Strings(changed)
	t.Errorf("zweiter Lauf hat etwas verändert:\n%s", strings.Join(changed, "\n"))
}

func assertSymlink(t *testing.T, path string, want string) {
	t.Helper()

	destination, err := os.Readlink(path)
	if err != nil {
		t.Errorf("%s ist kein Symlink: %v", path, err)
		return
	}
	if destination != want {
		t.Errorf("%s zeigt auf %q, erwartet %q", path, destination, want)
	}
}

// assertInclude prüft den Sollzustand an CLAUDE.md: eine reguläre Datei, kein
// Symlink, mit wirksamer Import-Zeile.
func assertInclude(t *testing.T, path string) {
	t.Helper()

	info, err := os.Lstat(path)
	if err != nil {
		t.Errorf("%s fehlt: %v", path, err)
		return
	}
	if !info.Mode().IsRegular() {
		t.Errorf("%s ist keine reguläre Datei (%s)", path, info.Mode())
		return
	}
	if !hasEffectiveInclude(readFile(t, path)) {
		t.Errorf("%s trägt keine wirksame Zeile %s:\n%s", path, ClaudeIncludeLine, readFile(t, path))
	}
}

func assertMissing(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Lstat(path); err == nil {
		t.Errorf("%s ist vorhanden, sollte es nicht sein", path)
	}
}

func assertRegular(t *testing.T, path string) {
	t.Helper()

	info, err := os.Lstat(path)
	if err != nil {
		t.Errorf("%s fehlt: %v", path, err)
		return
	}
	if !info.Mode().IsRegular() {
		t.Errorf("%s ist keine echte Datei (%s)", path, info.Mode())
	}
}

func assertContains(t *testing.T, path string, phrase string) {
	t.Helper()

	if content := readFile(t, path); !strings.Contains(content, phrase) {
		t.Errorf("%s enthält %q nicht:\n%s", path, phrase, content)
	}
}

// TestApplyAssistantSetupFallmatrix führt jede Zeile der Fallmatrix vollständig
// aus und lässt sie danach ein zweites Mal laufen — der zweite Lauf darf nichts
// mehr ändern.
//
// Die Zeilen 1 und 17 fehlen hier: „unlesbar" und ein Socket lassen sich nicht
// portabel herstellen. Sie stehen in instructions_layout_test.go.
func TestApplyAssistantSetupFallmatrix(t *testing.T) {
	fremd := filepath.Join("docs", "AGENTS.md")

	cases := []struct {
		name string
		row  int
		// resolved ist die Zeile des zweiten Durchgangs; 0 heißt „wie row".
		resolved int
		outcome  InstructionsOutcome
		linksOK  bool
		setup    func(t *testing.T, root string)
		after    func(t *testing.T, root string)
	}{
		{
			name: "2 — AGENTS.md ist ein Verzeichnis", row: 2, outcome: InstructionsBlocked,
			setup: func(t *testing.T, root string) {
				mkdir(t, filepath.Join(root, RootInstructionsFile))
			},
			after: func(t *testing.T, root string) {
				if !isDir(filepath.Join(root, RootInstructionsFile)) {
					t.Error("das Verzeichnis AGENTS.md wurde angefasst")
				}
				assertMissing(t, filepath.Join(root, ClaudeInstructionsFile))
			},
		},
		{
			name: "3 — CLAUDE.md ist ein Verzeichnis", row: 3, outcome: InstructionsBlocked,
			setup: func(t *testing.T, root string) {
				mkdir(t, filepath.Join(root, ClaudeInstructionsFile))
			},
			after: func(t *testing.T, root string) {
				if !isDir(filepath.Join(root, ClaudeInstructionsFile)) {
					t.Error("das Verzeichnis CLAUDE.md wurde angefasst")
				}
				// AGENTS.md wird normal behandelt.
				assertContains(t, filepath.Join(root, RootInstructionsFile), instructionsMarker)
			},
		},
		{
			name: "4 — CLAUDE.md zeigt auf ein fremdes Ziel", row: 4, outcome: InstructionsConflict,
			setup: func(t *testing.T, root string) {
				writeFile(t, filepath.Join(root, fremd), "# fremd\n")
				symlink(t, filepath.Join(root, ClaudeInstructionsFile), fremd)
			},
			after: func(t *testing.T, root string) {
				assertSymlink(t, filepath.Join(root, ClaudeInstructionsFile), fremd)
				assertMissing(t, filepath.Join(root, RootInstructionsFile))
				if content := readFile(t, filepath.Join(root, fremd)); content != "# fremd\n" {
					t.Errorf("das fremde Ziel wurde verändert: %q", content)
				}
			},
		},
		{
			name: "5 — verdrehte Richtung mit Inhalt", row: 5, outcome: InstructionsRenamed, linksOK: true,
			setup: func(t *testing.T, root string) {
				writeFile(t, filepath.Join(root, ClaudeInstructionsFile), "# eigen\n")
				symlink(t, filepath.Join(root, RootInstructionsFile), ClaudeInstructionsFile)
			},
			after: func(t *testing.T, root string) {
				assertRegular(t, filepath.Join(root, RootInstructionsFile))
				assertContains(t, filepath.Join(root, RootInstructionsFile), "# eigen")
				assertInclude(t, filepath.Join(root, ClaudeInstructionsFile))
			},
		},
		{
			name: "6 — verdrehte Richtung ohne Inhalt", row: 6, resolved: 12,
			outcome: InstructionsCleared, linksOK: true,
			setup: func(t *testing.T, root string) {
				symlink(t, filepath.Join(root, RootInstructionsFile), ClaudeInstructionsFile)
			},
			after: func(t *testing.T, root string) {
				assertRegular(t, filepath.Join(root, RootInstructionsFile))
				assertInclude(t, filepath.Join(root, ClaudeInstructionsFile))
			},
		},
		{
			name: "6 — verdrehte Richtung als Zyklus", row: 6, resolved: 15,
			outcome: InstructionsCleared, linksOK: true,
			setup: func(t *testing.T, root string) {
				symlink(t, filepath.Join(root, RootInstructionsFile), ClaudeInstructionsFile)
				symlink(t, filepath.Join(root, ClaudeInstructionsFile), RootInstructionsFile)
			},
			after: func(t *testing.T, root string) {
				assertRegular(t, filepath.Join(root, RootInstructionsFile))
				assertInclude(t, filepath.Join(root, ClaudeInstructionsFile))
			},
		},
		{
			// Die Include-Datei wird nicht umbenannt — sonst importierte
			// AGENTS.md sich selbst. Erst weicht der Symlink, dann entsteht
			// AGENTS.md aus der Vorlage.
			name: "6 — verdrehte Richtung mit Include-Datei", row: 6, resolved: 10,
			outcome: InstructionsCleared, linksOK: true,
			setup: func(t *testing.T, root string) {
				writeFile(t, filepath.Join(root, ClaudeInstructionsFile), claudeIncludeStub())
				symlink(t, filepath.Join(root, RootInstructionsFile), ClaudeInstructionsFile)
			},
			after: func(t *testing.T, root string) {
				assertRegular(t, filepath.Join(root, RootInstructionsFile))
				assertContains(t, filepath.Join(root, RootInstructionsFile), instructionsMarker)
				if strings.Contains(readFile(t, filepath.Join(root, RootInstructionsFile)), ClaudeIncludeLine) {
					t.Error("AGENTS.md importiert sich selbst")
				}
				if readFile(t, filepath.Join(root, ClaudeInstructionsFile)) != claudeIncludeStub() {
					t.Error("die Include-Datei wurde angefasst")
				}
			},
		},
		{
			name: "7 — AGENTS.md ist ein Rest-Link", row: 7, resolved: 10,
			outcome: InstructionsRenamed, linksOK: true,
			setup: func(t *testing.T, root string) {
				symlink(t, filepath.Join(root, RootInstructionsFile), "weg.md")
				writeFile(t, filepath.Join(root, ClaudeInstructionsFile), "# eigen\n")
			},
			after: func(t *testing.T, root string) {
				assertRegular(t, filepath.Join(root, RootInstructionsFile))
				assertContains(t, filepath.Join(root, RootInstructionsFile), "# eigen")
				assertMissing(t, filepath.Join(root, "weg.md"))
				assertInclude(t, filepath.Join(root, ClaudeInstructionsFile))
			},
		},
		{
			name: "8 — AGENTS.md verlinkt, CLAUDE.md echt", row: 8, outcome: InstructionsConflict,
			setup: func(t *testing.T, root string) {
				writeFile(t, filepath.Join(root, fremd), "# fremd\n")
				symlink(t, filepath.Join(root, RootInstructionsFile), fremd)
				writeFile(t, filepath.Join(root, ClaudeInstructionsFile), "# eigen\n")
			},
			after: func(t *testing.T, root string) {
				assertSymlink(t, filepath.Join(root, RootInstructionsFile), fremd)
				if content := readFile(t, filepath.Join(root, ClaudeInstructionsFile)); content != "# eigen\n" {
					t.Errorf("CLAUDE.md wurde verändert: %q", content)
				}
				// Angehängt wird der Anstoß dort, wo eine Instruktionsdatei steht.
				assertContains(t, filepath.Join(root, fremd), instructionsMarker)
			},
		},
		{
			name: "9 — AGENTS.md verlinkt, CLAUDE.md ohne Inhalt", row: 9,
			outcome: InstructionsUnchanged, linksOK: true,
			setup: func(t *testing.T, root string) {
				writeFile(t, filepath.Join(root, fremd), "# fremd\n")
				symlink(t, filepath.Join(root, RootInstructionsFile), fremd)
			},
			after: func(t *testing.T, root string) {
				assertSymlink(t, filepath.Join(root, RootInstructionsFile), fremd)
				assertInclude(t, filepath.Join(root, ClaudeInstructionsFile))
				assertContains(t, filepath.Join(root, fremd), instructionsMarker)
			},
		},
		{
			// Die Include-Datei, die das Einrichten schreibt, muss selbst zu
			// Zeile 9 gehören — sonst fiele sie beim nächsten Lauf in Zeile 8.
			name: "9 — AGENTS.md verlinkt, CLAUDE.md Include-Datei", row: 9,
			outcome: InstructionsUnchanged, linksOK: true,
			setup: func(t *testing.T, root string) {
				writeFile(t, filepath.Join(root, fremd), "# fremd\n")
				symlink(t, filepath.Join(root, RootInstructionsFile), fremd)
				writeFile(t, filepath.Join(root, ClaudeInstructionsFile), claudeIncludeStub())
			},
			after: func(t *testing.T, root string) {
				assertSymlink(t, filepath.Join(root, RootInstructionsFile), fremd)
				assertInclude(t, filepath.Join(root, ClaudeInstructionsFile))
				assertContains(t, filepath.Join(root, fremd), instructionsMarker)
			},
		},
		{
			name: "10 — nur CLAUDE.md", row: 10, outcome: InstructionsRenamed, linksOK: true,
			setup: func(t *testing.T, root string) {
				writeFile(t, filepath.Join(root, ClaudeInstructionsFile), "# eigen\n")
			},
			after: func(t *testing.T, root string) {
				assertRegular(t, filepath.Join(root, RootInstructionsFile))
				assertContains(t, filepath.Join(root, RootInstructionsFile), "# eigen")
				assertInclude(t, filepath.Join(root, ClaudeInstructionsFile))
			},
		},
		{
			// Der Stub ohne Ziel ist ein Rest, kein Inhalt: er bleibt liegen,
			// AGENTS.md entsteht aus der Vorlage.
			name: "10 — nur Include-Datei", row: 10, outcome: InstructionsUnchanged, linksOK: true,
			setup: func(t *testing.T, root string) {
				writeFile(t, filepath.Join(root, ClaudeInstructionsFile), claudeIncludeStub())
			},
			after: func(t *testing.T, root string) {
				assertRegular(t, filepath.Join(root, RootInstructionsFile))
				assertContains(t, filepath.Join(root, RootInstructionsFile), instructionsMarker)
				if strings.Contains(readFile(t, filepath.Join(root, RootInstructionsFile)), ClaudeIncludeLine) {
					t.Error("AGENTS.md importiert sich selbst")
				}
				if readFile(t, filepath.Join(root, ClaudeInstructionsFile)) != claudeIncludeStub() {
					t.Error("die Include-Datei wurde angefasst")
				}
			},
		},
		{
			// Der Sollzustand. Hausregeln neben dem Include gehören dem Projekt.
			name: "11 — Include-Datei neben AGENTS.md", row: 11, outcome: InstructionsUnchanged, linksOK: true,
			setup: func(t *testing.T, root string) {
				writeFile(t, filepath.Join(root, RootInstructionsFile), "# vorhanden\n")
				writeFile(t, filepath.Join(root, ClaudeInstructionsFile), ClaudeIncludeLine+"\n\n## Hausregeln\n\nNur für Claude Code.\n")
			},
			after: func(t *testing.T, root string) {
				assertContains(t, filepath.Join(root, RootInstructionsFile), "# vorhanden")
				assertContains(t, filepath.Join(root, RootInstructionsFile), instructionsMarker)
				if content := readFile(t, filepath.Join(root, ClaudeInstructionsFile)); content != ClaudeIncludeLine+"\n\n## Hausregeln\n\nNur für Claude Code.\n" {
					t.Errorf("CLAUDE.md wurde verändert: %q", content)
				}
			},
		},
		{
			name: "11 — beide echte Dateien ohne Include", row: 11, outcome: InstructionsConflict,
			setup: func(t *testing.T, root string) {
				writeFile(t, filepath.Join(root, RootInstructionsFile), "# A\n")
				writeFile(t, filepath.Join(root, ClaudeInstructionsFile), "# eigenständig\n")
			},
			after: func(t *testing.T, root string) {
				if content := readFile(t, filepath.Join(root, ClaudeInstructionsFile)); content != "# eigenständig\n" {
					t.Errorf("CLAUDE.md wurde verändert: %q", content)
				}
				assertContains(t, filepath.Join(root, RootInstructionsFile), "# A")
			},
		},
		{
			name: "12 — beides fehlt", row: 12, outcome: InstructionsUnchanged, linksOK: true,
			setup: func(t *testing.T, root string) {},
			after: func(t *testing.T, root string) {
				assertContains(t, filepath.Join(root, RootInstructionsFile), instructionsMarker)
				assertInclude(t, filepath.Join(root, ClaudeInstructionsFile))
			},
		},
		{
			name: "13 — nur AGENTS.md", row: 13, outcome: InstructionsUnchanged, linksOK: true,
			setup: func(t *testing.T, root string) {
				writeFile(t, filepath.Join(root, RootInstructionsFile), "# vorhanden\n")
			},
			after: func(t *testing.T, root string) {
				assertContains(t, filepath.Join(root, RootInstructionsFile), "# vorhanden")
				assertContains(t, filepath.Join(root, RootInstructionsFile), instructionsMarker)
				assertInclude(t, filepath.Join(root, ClaudeInstructionsFile))
			},
		},
		{
			// Die Migration: der Symlink aus einer älteren Fassung wird
			// verlustfrei durch die Include-Datei ersetzt.
			name: "14 — Symlink aus älterer Fassung", row: 14, outcome: InstructionsUnchanged, linksOK: true,
			setup: func(t *testing.T, root string) {
				writeFile(t, filepath.Join(root, RootInstructionsFile), "# vorhanden\n")
				symlink(t, filepath.Join(root, ClaudeInstructionsFile), RootInstructionsFile)
			},
			after: func(t *testing.T, root string) {
				assertContains(t, filepath.Join(root, RootInstructionsFile), "# vorhanden")
				assertInclude(t, filepath.Join(root, ClaudeInstructionsFile))
			},
		},
		{
			name: "15 — Link auf ein noch fehlendes AGENTS.md", row: 15,
			outcome: InstructionsUnchanged, linksOK: true,
			setup: func(t *testing.T, root string) {
				symlink(t, filepath.Join(root, ClaudeInstructionsFile), RootInstructionsFile)
			},
			after: func(t *testing.T, root string) {
				assertRegular(t, filepath.Join(root, RootInstructionsFile))
				assertInclude(t, filepath.Join(root, ClaudeInstructionsFile))
			},
		},
		{
			name: "16 — CLAUDE.md ist ein Rest-Link", row: 16,
			outcome: InstructionsUnchanged, linksOK: true,
			setup: func(t *testing.T, root string) {
				symlink(t, filepath.Join(root, ClaudeInstructionsFile), "weg.md")
			},
			after: func(t *testing.T, root string) {
				assertRegular(t, filepath.Join(root, RootInstructionsFile))
				assertInclude(t, filepath.Join(root, ClaudeInstructionsFile))
				assertMissing(t, filepath.Join(root, "weg.md"))
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root := newProject(t)
			testCase.setup(t, root)

			setup, err := ApplyAssistantSetup(root)
			if err != nil {
				t.Fatalf("ApplyAssistantSetup: %v", err)
			}
			if setup.Instructions.Row != testCase.row {
				t.Errorf("Zeile = %d, erwartet %d", setup.Instructions.Row, testCase.row)
			}

			resolved := testCase.resolved
			if resolved == 0 {
				resolved = testCase.row
			}
			if setup.Instructions.ResolvedRow != resolved {
				t.Errorf("entscheidende Zeile = %d, erwartet %d", setup.Instructions.ResolvedRow, resolved)
			}
			if setup.Instructions.Outcome != testCase.outcome {
				t.Errorf("Outcome = %q, erwartet %q — %s",
					setup.Instructions.Outcome, testCase.outcome, setup.Instructions.Detail)
			}
			if got := LinksOK(setup.Links); got != testCase.linksOK {
				t.Errorf("LinksOK = %v, erwartet %v", got, testCase.linksOK)
			}
			testCase.after(t, root)

			// Die Katalog-Links hängen nicht an der Wurzeldatei und stehen in
			// jedem Fall.
			assertRegistryLinks(t, root, setup.Links)

			// Zweiter Lauf: er darf nichts mehr ändern.
			before := snapshotTree(t, root)
			if _, err := ApplyAssistantSetup(root); err != nil {
				t.Fatalf("zweiter Lauf: %v", err)
			}
			assertUnchanged(t, before, snapshotTree(t, root))
		})
	}
}

// assertRegistryLinks prüft, dass alle Katalog-Links stehen.
func assertRegistryLinks(t *testing.T, root string, statuses []LinkStatus) {
	t.Helper()

	registry := 0
	for _, status := range statuses {
		if status.IsInclude {
			continue
		}
		registry++
		if status.State != StateOK {
			t.Errorf("%s: State = %q (%s), erwartet %q", status.Path, status.State, status.Detail, StateOK)
		}
	}
	if registry != 4 {
		t.Errorf("%d Katalog-Links geprüft, erwartet 4", registry)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "commands", "k-test.md")); err != nil {
		t.Errorf("Command nicht registriert: %v", err)
	}
}

// Der Inhalt der mitgebrachten CLAUDE.md muss nach dem Einrichten in AGENTS.md
// stehen, und der Anstoß genau einmal darin.
func TestEinrichtenErhaeltInhaltUndAnstoss(t *testing.T) {
	cases := []struct {
		name  string
		row   int
		setup func(t *testing.T, root string)
	}{
		{
			name: "Zeile 10 — nur CLAUDE.md", row: 10,
			setup: func(t *testing.T, root string) {
				writeFile(t, filepath.Join(root, ClaudeInstructionsFile), "# Unser Projekt\n\nEigene Regeln.\n")
			},
		},
		{
			name: "Zeile 5 — AGENTS.md zeigt auf CLAUDE.md", row: 5,
			setup: func(t *testing.T, root string) {
				writeFile(t, filepath.Join(root, ClaudeInstructionsFile), "# Unser Projekt\n\nEigene Regeln.\n")
				symlink(t, filepath.Join(root, RootInstructionsFile), ClaudeInstructionsFile)
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root := newProject(t)
			testCase.setup(t, root)

			setup, err := ApplyAssistantSetup(root)
			if err != nil {
				t.Fatalf("ApplyAssistantSetup: %v", err)
			}
			if setup.Instructions.Row != testCase.row || !setup.Instructions.Renamed {
				t.Fatalf("Zeile = %d, Renamed = %v — %s",
					setup.Instructions.Row, setup.Instructions.Renamed, setup.Instructions.Detail)
			}

			content := readFile(t, filepath.Join(root, RootInstructionsFile))
			if !strings.Contains(content, "Eigene Regeln.") {
				t.Errorf("der mitgebrachte Inhalt fehlt:\n%s", content)
			}
			if got := strings.Count(content, instructionsMarker); got != 1 {
				t.Errorf("Anstoß steht %dmal, erwartet genau einmal:\n%s", got, content)
			}

			// CLAUDE.md ist danach nur noch der Include; der Inhalt steht
			// einmal, in AGENTS.md.
			assertInclude(t, filepath.Join(root, ClaudeInstructionsFile))
			if strings.Contains(readFile(t, filepath.Join(root, ClaudeInstructionsFile)), "Eigene Regeln.") {
				t.Error("der Inhalt steht doppelt, auch in CLAUDE.md")
			}

			// Ein zweiter Lauf hängt den Anstoß nicht erneut an.
			if _, err := ApplyAssistantSetup(root); err != nil {
				t.Fatalf("zweiter Lauf: %v", err)
			}
			if readFile(t, filepath.Join(root, RootInstructionsFile)) != content {
				t.Error("zweiter Lauf hat AGENTS.md verändert")
			}
		})
	}
}

// Im Konfliktfall wird nichts angelegt — sonst stünde neben dem echten Inhalt
// eine zweite, fast leere Instruktionsquelle.
func TestKonfliktLegtNichtsAn(t *testing.T) {
	fremd := filepath.Join("docs", "AGENTS.md")

	t.Run("ohne AGENTS.md", func(t *testing.T) {
		root := newProject(t)
		writeFile(t, filepath.Join(root, fremd), "# fremd\n")
		symlink(t, filepath.Join(root, ClaudeInstructionsFile), fremd)

		if _, err := ApplyAssistantSetup(root); err != nil {
			t.Fatalf("ApplyAssistantSetup: %v", err)
		}
		assertMissing(t, filepath.Join(root, RootInstructionsFile))
	})

	t.Run("mit Rest-Link an AGENTS.md", func(t *testing.T) {
		root := newProject(t)
		writeFile(t, filepath.Join(root, fremd), "# fremd\n")
		symlink(t, filepath.Join(root, ClaudeInstructionsFile), fremd)
		symlink(t, filepath.Join(root, RootInstructionsFile), "weg.md")

		setup, err := ApplyAssistantSetup(root)
		if err != nil {
			t.Fatalf("ApplyAssistantSetup: %v", err)
		}
		if setup.Instructions.Row != 4 {
			t.Errorf("Zeile = %d, erwartet 4", setup.Instructions.Row)
		}

		// os.WriteFile folgte dem toten Link und legte die Datei an seinem Ziel an.
		assertMissing(t, filepath.Join(root, "weg.md"))
		assertSymlink(t, filepath.Join(root, RootInstructionsFile), "weg.md")
	})
}

// Ein Konflikt ist ein offener Punkt, auch am optionalen Link CLAUDE.md.
func TestKonfliktZaehltTrotzOptional(t *testing.T) {
	root := newProject(t)
	fremd := filepath.Join("docs", "AGENTS.md")
	writeFile(t, filepath.Join(root, fremd), "# fremd\n")
	symlink(t, filepath.Join(root, ClaudeInstructionsFile), fremd)

	statuses := CheckLinks(root)
	status := statusFor(t, statuses, ClaudeInstructionsFile)
	if !status.Optional {
		t.Fatal("der Link gilt nicht mehr als optional; der Test prüft dann nichts")
	}
	if status.State != StateConflict {
		t.Errorf("State = %q, erwartet %q", status.State, StateConflict)
	}
	if !strings.Contains(status.Detail, fremd) {
		t.Errorf("Detailtext nennt das Ziel nicht: %s", status.Detail)
	}
	if !strings.Contains(status.Detail, "sieht Claude Code den Anstoß nicht") {
		t.Errorf("Detailtext verschweigt die Folge: %s", status.Detail)
	}
	if !status.NeedsAction() {
		t.Error("der Konflikt zählt nicht als offener Punkt")
	}
	if LinksOK(statuses) {
		t.Error("LinksOK ist trotz Konflikt true")
	}
}

// Zeile 7: der Rest-Link verschwindet, und die Datei liegt danach an AGENTS.md,
// nicht am alten Ziel des Links.
func TestZeile7EntferntRestLink(t *testing.T) {
	root := newProject(t)
	symlink(t, filepath.Join(root, RootInstructionsFile), filepath.Join("docs", "weg.md"))

	setup, err := ApplyAssistantSetup(root)
	if err != nil {
		t.Fatalf("ApplyAssistantSetup: %v", err)
	}
	if setup.Instructions.Row != 7 || !setup.Instructions.RemovedLink {
		t.Fatalf("Zeile = %d, RemovedLink = %v", setup.Instructions.Row, setup.Instructions.RemovedLink)
	}

	assertRegular(t, filepath.Join(root, RootInstructionsFile))
	assertContains(t, filepath.Join(root, RootInstructionsFile), instructionsMarker)
	assertMissing(t, filepath.Join(root, "docs", "weg.md"))
	assertInclude(t, filepath.Join(root, ClaudeInstructionsFile))
}

// Zeile 9 schreibt die Include-Datei neben ein bewusst fremdverlinktes
// AGENTS.md. Der zweite Lauf darf an genau dieser Datei keinen Konflikt melden
// — sie ist eine reguläre Datei, und ohne den Include-Zweig in Zeile 9 fiele
// sie in Zeile 8.
func TestZeile9ZweiterLaufOhneKonflikt(t *testing.T) {
	root := newProject(t)
	fremd := filepath.Join("docs", "AGENTS.md")
	writeFile(t, filepath.Join(root, fremd), "# fremd\n")
	symlink(t, filepath.Join(root, RootInstructionsFile), fremd)

	first, err := ApplyAssistantSetup(root)
	if err != nil {
		t.Fatalf("erster Lauf: %v", err)
	}
	if first.Instructions.Row != 9 {
		t.Fatalf("Zeile = %d, erwartet 9", first.Instructions.Row)
	}
	assertInclude(t, filepath.Join(root, ClaudeInstructionsFile))

	second, err := ApplyAssistantSetup(root)
	if err != nil {
		t.Fatalf("zweiter Lauf: %v", err)
	}
	if second.Instructions.Row != 9 || second.Instructions.Outcome != InstructionsUnchanged {
		t.Errorf("zweiter Lauf: Zeile = %d, Outcome = %q — %s",
			second.Instructions.Row, second.Instructions.Outcome, second.Instructions.Detail)
	}
	status := statusFor(t, second.Links, ClaudeInstructionsFile)
	if status.State != StateOK {
		t.Errorf("State = %q (%s), erwartet %q", status.State, status.Detail, StateOK)
	}
	if !LinksOK(second.Links) {
		t.Errorf("zweiter Lauf nicht eingerichtet: %+v", second.Links)
	}
}

// Zeile 2: das Verzeichnis an AGENTS.md darf die Katalog-Links nicht mit
// ausfallen lassen, und die Prüfung muss die richtige Ursache nennen.
func TestZeile2SetztKatalogLinksTrotzdem(t *testing.T) {
	root := newProject(t)
	mkdir(t, filepath.Join(root, RootInstructionsFile))

	setup, err := ApplyAssistantSetup(root)
	if err != nil {
		t.Fatalf("ApplyAssistantSetup: %v", err)
	}
	assertRegistryLinks(t, root, setup.Links)

	status := statusFor(t, setup.Links, ClaudeInstructionsFile)
	if status.State != StateBlocked {
		t.Errorf("State = %q, erwartet %q", status.State, StateBlocked)
	}
	if !strings.Contains(status.Detail, "Verzeichnis") {
		t.Errorf("Detailtext nennt das Verzeichnis nicht: %s", status.Detail)
	}
	if strings.Contains(status.Detail, "fehlt im Projekt") {
		t.Errorf("Detailtext nennt die falsche Ursache: %s", status.Detail)
	}
}
