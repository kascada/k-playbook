package project

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// linkTo baut einen synthetischen Symlink-Zustand für die Matrix-Tabelle.
func linkTo(kind pathKind, destination string) pathState {
	return pathState{kind: kind, link: destination}
}

// TestFallmatrixTrifftJedeZeile prüft die Reihenfolge der Fallmatrix: je Zeile
// eine Ausgangslage, die von keiner früheren Zeile eingefangen werden darf.
//
// Geprüft wird auf der Ebene der Einordnung und nicht am Dateisystem, weil sich
// zwei der sieben Zustände dort nicht portabel herstellen lassen: „unlesbar"
// bräuchte entzogene Rechte am Hauptverzeichnis, und ein Socket oder Gerät ist
// plattformabhängig. Die übrigen Zeilen laufen zusätzlich vollständig durch
// (setup_test.go).
func TestFallmatrixTrifftJedeZeile(t *testing.T) {
	dir := t.TempDir()

	cases := []struct {
		row    int
		name   string
		claude pathState
		agents pathState
		// resolved ist die Zeile des zweiten Durchgangs; 0 heißt „wie row".
		resolved int
	}{
		{row: 1, name: "CLAUDE.md unlesbar", claude: pathState{kind: kindUnreadable, err: os.ErrPermission}, agents: pathState{kind: kindMissing}},
		{row: 1, name: "AGENTS.md unlesbar", claude: pathState{kind: kindRegular}, agents: pathState{kind: kindUnreadable, err: os.ErrPermission}},
		{row: 2, name: "AGENTS.md ist ein Verzeichnis", claude: pathState{kind: kindRegular}, agents: pathState{kind: kindDir}},
		{row: 3, name: "CLAUDE.md ist ein Verzeichnis", claude: pathState{kind: kindDir}, agents: pathState{kind: kindMissing}},
		{row: 4, name: "CLAUDE.md zeigt woandershin", claude: linkTo(kindLinkForeign, "docs/AGENTS.md"), agents: pathState{kind: kindMissing}},
		{row: 5, name: "verdrehte Richtung mit Inhalt", claude: pathState{kind: kindRegular}, agents: linkTo(kindLinkToOther, "CLAUDE.md")},
		{row: 6, name: "verdrehte Richtung ohne Inhalt", claude: pathState{kind: kindMissing}, agents: linkTo(kindLinkToOther, "CLAUDE.md"), resolved: 12},
		{row: 6, name: "verdrehte Richtung als Zyklus", claude: linkTo(kindLinkToOther, "AGENTS.md"), agents: linkTo(kindLinkToOther, "CLAUDE.md"), resolved: 15},
		{row: 6, name: "verdrehte Richtung auf Rest-Link", claude: linkTo(kindLinkDangling, "weg.md"), agents: linkTo(kindLinkToOther, "CLAUDE.md"), resolved: 16},
		{row: 7, name: "AGENTS.md ist ein Rest-Link", claude: pathState{kind: kindMissing}, agents: linkTo(kindLinkDangling, "weg.md"), resolved: 12},
		{row: 7, name: "Rest-Link neben echter CLAUDE.md", claude: pathState{kind: kindRegular}, agents: linkTo(kindLinkDangling, "weg.md"), resolved: 10},
		{row: 8, name: "AGENTS.md verlinkt, CLAUDE.md echt", claude: pathState{kind: kindRegular}, agents: linkTo(kindLinkForeign, "docs/AGENTS.md")},
		{row: 9, name: "AGENTS.md verlinkt, CLAUDE.md leer", claude: pathState{kind: kindMissing}, agents: linkTo(kindLinkForeign, "docs/AGENTS.md")},
		{row: 10, name: "nur CLAUDE.md", claude: pathState{kind: kindRegular}, agents: pathState{kind: kindMissing}},
		{row: 11, name: "beide echt", claude: pathState{kind: kindRegular}, agents: pathState{kind: kindRegular}},
		{row: 12, name: "beides fehlt", claude: pathState{kind: kindMissing}, agents: pathState{kind: kindMissing}},
		{row: 13, name: "nur AGENTS.md", claude: pathState{kind: kindMissing}, agents: pathState{kind: kindRegular}},
		{row: 14, name: "Sollzustand", claude: linkTo(kindLinkToOther, "AGENTS.md"), agents: pathState{kind: kindRegular}},
		{row: 15, name: "Link auf fehlendes AGENTS.md", claude: linkTo(kindLinkToOther, "AGENTS.md"), agents: pathState{kind: kindMissing}},
		{row: 16, name: "CLAUDE.md ist ein Rest-Link", claude: linkTo(kindLinkDangling, "weg.md"), agents: pathState{kind: kindMissing}},
		{row: 17, name: "Auffangzweig", claude: pathState{kind: kindOther, mode: os.ModeNamedPipe}, agents: pathState{kind: kindMissing}},
	}

	seen := map[int]bool{}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			plan := planInstructions(dir, testCase.claude, testCase.agents)
			if plan.matched != testCase.row {
				t.Errorf("Zeile = %d, erwartet %d", plan.matched, testCase.row)
			}

			resolved := testCase.resolved
			if resolved == 0 {
				resolved = testCase.row
			}
			if plan.row != resolved {
				t.Errorf("entscheidende Zeile = %d, erwartet %d", plan.row, resolved)
			}
		})
		seen[testCase.row] = true
	}

	for row := 1; row <= 17; row++ {
		if !seen[row] {
			t.Errorf("Zeile %d hat keinen Fall", row)
		}
	}
}

// Kein Paar darf durchfallen, und im Konfliktfall darf nichts geplant sein, was
// etwas anfasst oder anlegt.
func TestFallmatrixFaengtJedesPaarAuf(t *testing.T) {
	dir := t.TempDir()

	kinds := []pathKind{
		kindMissing, kindRegular, kindDir, kindLinkToOther,
		kindLinkForeign, kindLinkDangling, kindUnreadable, kindOther,
	}

	for _, claude := range kinds {
		for _, agents := range kinds {
			plan := planInstructions(dir,
				pathState{kind: claude, link: "irgendwohin", err: os.ErrPermission},
				pathState{kind: agents, link: "irgendwohin", err: os.ErrPermission})

			if plan.matched < 1 || plan.matched > 17 {
				t.Errorf("(%d,%d): Zeile %d liegt außerhalb der Matrix", claude, agents, plan.matched)
			}
			if plan.conflict && (plan.rename || plan.removeAgentsLink || plan.mayCreate) {
				t.Errorf("(%d,%d): Konflikt in Zeile %d fasst etwas an", claude, agents, plan.matched)
			}
			if plan.conflict && plan.detail == "" {
				t.Errorf("(%d,%d): Konflikt ohne Detailtext", claude, agents)
			}
			if plan.blocked && plan.detail == "" {
				t.Errorf("(%d,%d): Blockade ohne Detailtext", claude, agents)
			}
		}
	}
}

// Der Detailtext der Zeile 17 muss beide Zustände nennen, sonst sagt er nicht,
// was zu tun ist.
func TestZeile17NenntBeideZustaende(t *testing.T) {
	plan := planInstructions(t.TempDir(),
		pathState{kind: kindOther, mode: os.ModeSocket},
		linkTo(kindLinkToOther, "CLAUDE.md"))

	if plan.matched != 17 || !plan.conflict {
		t.Fatalf("Zeile = %d, conflict = %v", plan.matched, plan.conflict)
	}
	for _, phrase := range []string{ClaudeInstructionsFile, RootInstructionsFile, "Symlink", conflictHint} {
		if !strings.Contains(plan.detail, phrase) {
			t.Errorf("Detailtext nennt %q nicht: %s", phrase, plan.detail)
		}
	}
}

// Verglichen wird das aufgelöste Ziel, nicht der Rohstring aus Readlink. Sonst
// fiele ./CLAUDE.md in den Fremdlink-Zweig und bliebe dauerhaft blockiert.
func TestKlassifikationLoestLinkzieleAuf(t *testing.T) {
	for _, destination := range []string{"CLAUDE.md", "./CLAUDE.md", "sub/../CLAUDE.md", ""} {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, ClaudeInstructionsFile), "# eigen\n")

		target := destination
		if target == "" {
			// Der absolute Pfad auf dieselbe Datei ist derselbe Zustand.
			target = filepath.Join(root, ClaudeInstructionsFile)
		}
		if err := os.Symlink(target, filepath.Join(root, RootInstructionsFile)); err != nil {
			t.Fatalf("Symlink anlegen: %v", err)
		}

		state := classifyPath(root, RootInstructionsFile, ClaudeInstructionsFile)
		if state.kind != kindLinkToOther {
			t.Errorf("%q: kind = %d, erwartet kindLinkToOther", target, state.kind)
		}
	}
}

// Ein Link ins Leere ist Rest, kein Fremdlink — auch wenn er auf die andere
// Datei zeigt, bleibt er die verdrehte Richtung.
func TestKlassifikationUnterscheidetRestLink(t *testing.T) {
	root := t.TempDir()
	if err := os.Symlink("weg.md", filepath.Join(root, RootInstructionsFile)); err != nil {
		t.Fatalf("Symlink anlegen: %v", err)
	}
	if got := classifyPath(root, RootInstructionsFile, ClaudeInstructionsFile).kind; got != kindLinkDangling {
		t.Errorf("kind = %d, erwartet kindLinkDangling", got)
	}

	// Zeigt der Link auf die fehlende Gegendatei, hat „andere Datei" Vorrang.
	if err := os.Remove(filepath.Join(root, RootInstructionsFile)); err != nil {
		t.Fatalf("Symlink entfernen: %v", err)
	}
	if err := os.Symlink(ClaudeInstructionsFile, filepath.Join(root, RootInstructionsFile)); err != nil {
		t.Fatalf("Symlink anlegen: %v", err)
	}
	if got := classifyPath(root, RootInstructionsFile, ClaudeInstructionsFile).kind; got != kindLinkToOther {
		t.Errorf("kind = %d, erwartet kindLinkToOther", got)
	}
}

// writeVCSConfig legt eine K-PLAYBOOK.yaml mit der gewünschten Versionskontrolle an.
func writeVCSConfig(t *testing.T, root string, vcs string) {
	t.Helper()

	writeFile(t, ConfigPath(root), "schema_version: "+SchemaVersion+"\n\nproject:\n  repo_root: .\n  vcs: "+vcs+"\n")
}

// gitInit legt ein echtes Repository an. Ohne git im Pfad hat der Test keine
// Grundlage.
func gitInit(t *testing.T, root string) {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("kein git im Pfad")
	}
	if output, err := exec.Command("git", "-C", root, "init", "--quiet").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v — %s", err, output)
	}
}

// Ist AGENTS.md ignoriert, nähme die Umbenennung versionierten Inhalt still aus
// der Versionskontrolle. Das ist der eine Fall, der blockiert.
func TestUmbenennenBlocktBeiIgnoriertemAgents(t *testing.T) {
	root := newProject(t)
	gitInit(t, root)
	writeVCSConfig(t, root, "git")
	writeFile(t, filepath.Join(root, ".gitignore"), RootInstructionsFile+"\n")
	writeFile(t, filepath.Join(root, ClaudeInstructionsFile), "# eigen\n")

	setup, err := ApplyAssistantSetup(root)
	if err != nil {
		t.Fatalf("ApplyAssistantSetup: %v", err)
	}
	if setup.Instructions.Outcome != InstructionsConflict {
		t.Errorf("Outcome = %q, erwartet %q", setup.Instructions.Outcome, InstructionsConflict)
	}
	if setup.Instructions.Row != 10 {
		t.Errorf("Zeile = %d, erwartet 10", setup.Instructions.Row)
	}
	for _, phrase := range []string{"ignoriert", "Ignore-Regel", "sieht Claude Code den Anstoß nicht"} {
		if !strings.Contains(setup.Instructions.Detail, phrase) {
			t.Errorf("Detailtext nennt %q nicht: %s", phrase, setup.Instructions.Detail)
		}
	}

	content, err := os.ReadFile(filepath.Join(root, ClaudeInstructionsFile))
	if err != nil || string(content) != "# eigen\n" {
		t.Errorf("CLAUDE.md = %q, %v — sie durfte nicht angefasst werden", content, err)
	}
	if _, err := os.Lstat(filepath.Join(root, RootInstructionsFile)); err == nil {
		t.Error("AGENTS.md wurde im Konfliktfall angelegt")
	}
}

// Ohne git gibt es nichts zu ignorieren; die Umbenennung läuft wie sonst.
func TestUmbenennenLaeuftOhneGit(t *testing.T) {
	root := newProject(t)
	writeVCSConfig(t, root, "none")
	writeFile(t, filepath.Join(root, ".gitignore"), RootInstructionsFile+"\n")
	writeFile(t, filepath.Join(root, ClaudeInstructionsFile), "# eigen\n")

	setup, err := ApplyAssistantSetup(root)
	if err != nil {
		t.Fatalf("ApplyAssistantSetup: %v", err)
	}
	if setup.Instructions.Outcome != InstructionsRenamed {
		t.Fatalf("Outcome = %q, erwartet %q — %s",
			setup.Instructions.Outcome, InstructionsRenamed, setup.Instructions.Detail)
	}
	if !strings.HasPrefix(readFile(t, filepath.Join(root, RootInstructionsFile)), "# eigen\n") {
		t.Error("der erhaltene Inhalt steht nicht in AGENTS.md")
	}
}

// git check-ignore läuft ohne --no-index: eine bereits getrackte Datei gilt als
// nicht ignoriert. Dann geht durch die Umbenennung nichts verloren, und der
// Vorbehalt soll nicht greifen.
func TestUmbenennenLaeuftBeiGetracktemAgents(t *testing.T) {
	root := newProject(t)
	gitInit(t, root)
	writeVCSConfig(t, root, "git")
	writeFile(t, filepath.Join(root, ".gitignore"), RootInstructionsFile+"\n")

	// AGENTS.md ist trotz der Ignore-Regel im Index — der übliche Weg dahin ist
	// ein git add -f aus der Zeit vor der Regel.
	writeFile(t, filepath.Join(root, RootInstructionsFile), "# versioniert\n")
	if output, err := exec.Command("git", "-C", root, "add", "-f", RootInstructionsFile).CombinedOutput(); err != nil {
		t.Fatalf("git add: %v — %s", err, output)
	}
	if err := os.Remove(filepath.Join(root, RootInstructionsFile)); err != nil {
		t.Fatalf("AGENTS.md entfernen: %v", err)
	}
	writeFile(t, filepath.Join(root, ClaudeInstructionsFile), "# eigen\n")

	setup, err := ApplyAssistantSetup(root)
	if err != nil {
		t.Fatalf("ApplyAssistantSetup: %v", err)
	}
	if setup.Instructions.Outcome != InstructionsRenamed {
		t.Fatalf("Outcome = %q, erwartet %q — %s",
			setup.Instructions.Outcome, InstructionsRenamed, setup.Instructions.Detail)
	}
	if !strings.HasPrefix(readFile(t, filepath.Join(root, RootInstructionsFile)), "# eigen\n") {
		t.Error("der erhaltene Inhalt steht nicht in AGENTS.md")
	}
}
