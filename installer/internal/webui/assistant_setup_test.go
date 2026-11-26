package webui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// Die Tests hier prüfen setupMessage: den Ablauf des Einrichtens
// (project.ApplyAssistantSetup) samt dem Text, den die Assistenten-Karte dazu
// zeigt. newProject, writeFile und readFile stehen in update_test.go.

// Das Einrichten ordnet das Paar CLAUDE.md/AGENTS.md ein: Ein Projekt mit nur
// echter CLAUDE.md bekommt AGENTS.md mit dem mitgebrachten Inhalt und dem
// Anstoß, CLAUDE.md wird zur Include-Datei.
func TestEinrichtenOrdnetClaudeUndAgentsEin(t *testing.T) {
	root := newProject(t)
	writeFile(t, filepath.Join(root, "CLAUDE.md"), "# Unser Projekt\n")

	message := setupMessage(root)

	agents := filepath.Join(root, project.RootInstructionsFile)
	content := readFile(t, agents)
	if !strings.Contains(content, "# Unser Projekt") {
		t.Errorf("der mitgebrachte Inhalt fehlt in AGENTS.md:\n%s", content)
	}
	if !strings.Contains(content, "\n    k-playbook briefing\n") {
		t.Errorf("der Anstoß fehlt in AGENTS.md:\n%s", content)
	}

	// CLAUDE.md ist danach die Include-Datei: regulär, mit der Import-Zeile,
	// ohne den umbenannten Inhalt.
	claude := filepath.Join(root, "CLAUDE.md")
	if info, err := os.Lstat(claude); err != nil || !info.Mode().IsRegular() {
		t.Errorf("CLAUDE.md ist keine reguläre Datei: %v", err)
	}
	if claudeContent := readFile(t, claude); !strings.Contains("\n"+claudeContent, "\n"+project.ClaudeIncludeLine+"\n") ||
		strings.Contains(claudeContent, "# Unser Projekt") {
		t.Errorf("CLAUDE.md ist kein reiner Include:\n%s", claudeContent)
	}
	if !project.LinksOK(project.CheckLinks(root)) {
		t.Errorf("nach dem Einrichten nicht eingerichtet: %+v", project.CheckLinks(root))
	}

	// Die Umbenennung gehört in den Antworttext.
	if !strings.Contains(message, "umbenannt") {
		t.Errorf("die Umbenennung fehlt im Antworttext: %s", message)
	}
}

// Ein Projekt ohne beide Dateien bekommt AGENTS.md aus der Vorlage, und der
// Antworttext nennt die Datei.
func TestEinrichtenLegtWurzeldateiAn(t *testing.T) {
	root := newProject(t)

	message := setupMessage(root)

	content := readFile(t, filepath.Join(root, project.RootInstructionsFile))
	if !strings.Contains(content, "\n    k-playbook briefing\n") {
		t.Errorf("der Anstoß fehlt in AGENTS.md:\n%s", content)
	}
	if !strings.Contains(message, project.RootInstructionsFile) {
		t.Errorf("die angelegte Wurzeldatei fehlt im Antworttext: %s", message)
	}

	// Ein zweiter Lauf ändert nichts mehr.
	setupMessage(root)
	if readFile(t, filepath.Join(root, project.RootInstructionsFile)) != content {
		t.Error("der zweite Lauf hat AGENTS.md verändert")
	}
}

// Ein Konflikt gehört ebenfalls in den Antworttext — er bedeutet, dass Claude
// Code das Playbook bis zur Handarbeit nicht kennt.
func TestEinrichtenMeldetKonflikt(t *testing.T) {
	root := newProject(t)
	writeFile(t, filepath.Join(root, project.RootInstructionsFile), "# A\n")
	writeFile(t, filepath.Join(root, "CLAUDE.md"), "# eigenständig\n")

	message := setupMessage(root)

	if !strings.Contains(message, "sieht Claude Code den Anstoß nicht") {
		t.Errorf("der Konflikt fehlt im Antworttext: %s", message)
	}
	if content := readFile(t, filepath.Join(root, "CLAUDE.md")); content != "# eigenständig\n" {
		t.Errorf("CLAUDE.md wurde verändert: %q", content)
	}
}

// Bricht das Einrichten mit einem Fehler ab, nennt der Antworttext
// trotzdem den README-Schritt (Task 137): Er steht neben dem
// Fehlerwert im Ergebnis. Hier ohne Anbindung des Testprojekts als Hinweis.
func TestEinrichtenNenntReadmesAuchImFehlerzweig(t *testing.T) {
	root := newProject(t)
	// .claude ist eine Datei: Die Verlinkung kann darunter nichts anlegen.
	writeFile(t, filepath.Join(root, ".claude"), "kein Verzeichnis\n")

	message := setupMessage(root)
	if !strings.Contains(message, "Nicht vollständig eingerichtet") {
		t.Fatalf("kein Fehlerzweig: %s", message)
	}
	if !strings.Contains(message, "README unter keph:/docs/: nicht versucht") {
		t.Errorf("der README-Schritt fehlt im Antworttext: %s", message)
	}
}

// Im Erfolgszweig nennt describeSetup den README-Schritt.
func TestEinrichtenNenntReadmes(t *testing.T) {
	root := newProject(t)
	message := setupMessage(root)
	if !strings.Contains(message, "README unter keph:/docs/") {
		t.Errorf("der README-Schritt fehlt im Antworttext: %s", message)
	}
}

// Ergänzt das Einrichten den Block maintenance, nennt die Antwort den Schritt
// (Task 196, V8): Die Änderung an der versionierten K-PLAYBOOK.yaml will
// committet werden und darf nicht still geschehen.
func TestEinrichtenNenntMaintenanceBlock(t *testing.T) {
	root := newProject(t)
	config := "schema_version: " + project.SchemaVersion + "\n\nproject:\n  repo_root: .\n  vcs: git\n"
	writeFile(t, project.ConfigPath(root), config)

	message := setupMessage(root)

	if readFile(t, project.ConfigPath(root)) == config {
		t.Fatal("der Block maintenance wurde nicht ergänzt; der Test prüft nichts")
	}
	if !strings.Contains(message, "Block "+project.MaintenanceBlockName+" in "+project.ConfigFileName+" ergänzt") {
		t.Errorf("die Antwort nennt das Ergänzen nicht: %s", message)
	}
}
