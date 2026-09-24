package webui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// newProject baut ein Zielprojekt mit Installation darin auf.
func newProject(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	writeFile(t, filepath.Join(root, project.PlaybookDirName, "commands", "k-test.md"), "test\n")
	writeFile(t, filepath.Join(root, project.PlaybookDirName, "skills", "beispiel", "SKILL.md"), "# Beispiel\n")
	return root
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("%s anlegen: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("%s schreiben: %v", path, err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s lesen: %v", path, err)
	}
	return string(data)
}

// Das Aktualisieren bekommt denselben Ablauf wie das Einrichten. Ohne das
// bliebe ein Projekt mit nur echter CLAUDE.md über „Aktualisieren" für immer
// unverändert.
func TestRelinkAfterUpdateRichtetEin(t *testing.T) {
	root := newProject(t)
	writeFile(t, filepath.Join(root, "CLAUDE.md"), "# Unser Projekt\n")

	_, message := relinkAfterUpdate(root, "Aktualisiert.")

	agents := filepath.Join(root, project.RootInstructionsFile)
	content := readFile(t, agents)
	if !strings.Contains(content, "# Unser Projekt") {
		t.Errorf("der mitgebrachte Inhalt fehlt in AGENTS.md:\n%s", content)
	}
	if !strings.Contains(content, "\n    k-playbook context\n") {
		t.Errorf("der Anstoß fehlt in AGENTS.md:\n%s", content)
	}

	// CLAUDE.md ist danach die Include-Datei: regulär, mit der Import-Zeile,
	// ohne den umbenannten Inhalt.
	claude := filepath.Join(root, "CLAUDE.md")
	if info, err := os.Lstat(claude); err != nil || !info.Mode().IsRegular() {
		t.Errorf("CLAUDE.md ist keine reguläre Datei: %v", err)
	}
	if claudeContent := readFile(t, claude); !strings.Contains(claudeContent, "\n"+project.ClaudeIncludeLine+"\n") ||
		strings.Contains(claudeContent, "# Unser Projekt") {
		t.Errorf("CLAUDE.md ist kein reiner Include:\n%s", claudeContent)
	}
	if !project.LinksOK(project.CheckLinks(root)) {
		t.Errorf("nach dem Aktualisieren nicht eingerichtet: %+v", project.CheckLinks(root))
	}

	// Die Umbenennung gehört in den Antworttext des Updates, nicht nur in den
	// des Einrichtens.
	if !strings.Contains(message, "umbenannt") {
		t.Errorf("die Umbenennung fehlt im Antworttext: %s", message)
	}
}

// Ein Projekt ohne beide Dateien bekommt AGENTS.md erstmals aus der Vorlage.
// Das ist neues Verhalten für diesen Einstieg und deshalb gewollt benannt.
func TestRelinkAfterUpdateLegtWurzeldateiAn(t *testing.T) {
	root := newProject(t)

	_, message := relinkAfterUpdate(root, "Aktualisiert.")

	content := readFile(t, filepath.Join(root, project.RootInstructionsFile))
	if !strings.Contains(content, "\n    k-playbook context\n") {
		t.Errorf("der Anstoß fehlt in AGENTS.md:\n%s", content)
	}
	if !strings.Contains(message, project.RootInstructionsFile) {
		t.Errorf("die angelegte Wurzeldatei fehlt im Antworttext: %s", message)
	}

	// Ein zweiter Lauf ändert nichts mehr.
	relinkAfterUpdate(root, "Aktualisiert.")
	if readFile(t, filepath.Join(root, project.RootInstructionsFile)) != content {
		t.Error("der zweite Lauf hat AGENTS.md verändert")
	}
}

// Ein Konflikt gehört ebenfalls in den Antworttext — er bedeutet, dass Claude
// Code das Playbook bis zur Handarbeit nicht kennt.
func TestRelinkAfterUpdateMeldetKonflikt(t *testing.T) {
	root := newProject(t)
	writeFile(t, filepath.Join(root, project.RootInstructionsFile), "# A\n")
	writeFile(t, filepath.Join(root, "CLAUDE.md"), "# eigenständig\n")

	_, message := relinkAfterUpdate(root, "Aktualisiert.")

	if !strings.Contains(message, "sieht Claude Code den Anstoß nicht") {
		t.Errorf("der Konflikt fehlt im Antworttext: %s", message)
	}
	if content := readFile(t, filepath.Join(root, "CLAUDE.md")); content != "# eigenständig\n" {
		t.Errorf("CLAUDE.md wurde verändert: %q", content)
	}
}

// Scheitert die Programmaktualisierung, nennt die Meldung den Bootstrap in der
// kanonischen Form. Geprüft wird der ganze Satz, nicht nur ein
// Wortbestandteil: die Meldung ist die Stelle, an der ein Nutzer erfährt, wie
// er das Programm von Hand nachholt.
//
// Ausdrücklich mitgeprüft wird, was **nicht** dastehen darf: ein Zielprojekt
// hat kein eigenes install-Target, `make install` liefe dort ins Leere.
func TestInstallationsfehlerNenntKanonischenBootstrap(t *testing.T) {
	message := installFailureMessage(errors.New("kein Netz"))

	for _, want := range []string{
		"kein Netz",
		"läuft mit dem bisherigen Programm weiter",
		"make -C " + project.PlaybookDirName + " install",
		project.PlaybookDirName + "/bin/install",
	} {
		if !strings.Contains(message, want) {
			t.Errorf("in der Meldung fehlt %q: %s", want, message)
		}
	}
	if strings.Contains(strings.ReplaceAll(message, "make -C "+project.PlaybookDirName+" install", ""), "make install") {
		t.Errorf("die Meldung nennt ein install-Target, das ein Zielprojekt nicht hat: %s", message)
	}
}

// Die Oberfläche zeigt denselben Bootstrap wie die Antwort des Servers. Beide
// Texte stehen an verschiedenen Stellen — Go-Meldung und service.js, wo der
// Update-Knopf der Statusseite und von /setup bedient wird —, und genau
// deshalb wird der Gleichlauf geprüft statt vorausgesetzt.
func TestOberflaecheNenntDenselbenBootstrap(t *testing.T) {
	source, err := staticFiles.ReadFile("static/service.js")
	if err != nil {
		t.Fatalf("service.js lesen: %v", err)
	}
	script := string(source)

	for _, want := range []string{
		"make -C " + project.PlaybookDirName + " install",
		project.PlaybookDirName + "/bin/install",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("in service.js fehlt %q", want)
		}
	}
}

// Ob nach dem Pull installiert und neu gestartet wird, entscheidet nicht der
// Wechsel der VERSION allein, sondern der semantische Vergleich mit dem
// laufenden Programm: nur „älter" löst aus.
func TestRestartAfterPull(t *testing.T) {
	tests := []struct {
		name    string
		running string
		result  project.UpdateResult
		want    bool
	}{
		{
			name:    "Zielprojekt: Clone zieht an, Programm bleibt zurück",
			running: "v0.3.0",
			result:  project.UpdateResult{VersionChanged: true, Version: "v0.4.0"},
			want:    true,
		},
		{
			// Lexikalisch stünde v0.10.0 vor v0.9.4.
			name:    "Minor-Sprung über 9 hinaus",
			running: "v0.9.4",
			result:  project.UpdateResult{VersionChanged: true, Version: "v0.10.0"},
			want:    true,
		},
		{
			// Der Entwicklungsfall: `make dev-install` hat das Programm des
			// neuen Standes schon eingespielt, der Clone holt ihn erst jetzt nach.
			name:    "Entwicklungsrepo: Programm trägt die neue Version bereits",
			running: "v0.4.0",
			result:  project.UpdateResult{VersionChanged: true, Version: "v0.4.0"},
		},
		{
			// Neuer als der Clone: nie herabstufen.
			name:    "Programm neuer als der neue Stand",
			running: "v0.5.0",
			result:  project.UpdateResult{VersionChanged: true, Version: "v0.4.0"},
		},
		{
			// Ohne Versionswechsel verhält sich ein Update wie immer.
			name:    "kein Versionswechsel, Programm älter",
			running: "v0.3.0",
			result:  project.UpdateResult{Version: "v0.4.0"},
		},
		{
			name:    "Installation ohne VERSION",
			running: "v0.4.0",
			result:  project.UpdateResult{VersionChanged: true},
		},
		{
			name:   "Programm ohne gestempelte Version",
			result: project.UpdateResult{VersionChanged: true, Version: "v0.4.0"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := restartAfterPull(test.running, test.result); got != test.want {
				t.Errorf("restartAfterPull = %v, erwartet %v", got, test.want)
			}
		})
	}
}

// Nach einem Clone-Update hängt der Dock/Finder-Hinweis an der geschriebenen
// Form: nur wo der bloße Name steht, wird er genannt.
func TestDescribeMCPRepairNenntDiePortableForm(t *testing.T) {
	absolute := describeMCPRepair(project.UpdateResult{MCPRepaired: []string{".mcp.json"}})
	if !strings.Contains(absolute, ".mcp.json") {
		t.Errorf("Meldung nennt die Datei nicht: %q", absolute)
	}
	if strings.Contains(absolute, "bloße Name") || strings.Contains(absolute, "Dock oder Finder") {
		t.Errorf("Meldung ohne portable Form nennt den Hinweis: %q", absolute)
	}

	portable := describeMCPRepair(project.UpdateResult{
		MCPRepaired:         []string{".mcp.json", "opencode.json"},
		MCPRepairedPortable: []string{".mcp.json"},
	})
	for _, want := range []string{"bloße Name " + project.InstalledCommandName, "In .mcp.json steht", "Dock oder Finder", "von Hand"} {
		if !strings.Contains(portable, want) {
			t.Errorf("Meldung enthält %q nicht: %q", want, portable)
		}
	}
}
