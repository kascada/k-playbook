package project

import (
	"path/filepath"
	"testing"
)

func entryFor(t *testing.T, entries []RegistryEntry, name string) RegistryEntry {
	t.Helper()

	for _, entry := range entries {
		if entry.Name == name {
			return entry
		}
	}
	t.Fatalf("kein Eintrag %q in %+v", name, entries)
	return RegistryEntry{}
}

func TestResolveRegistryFuehrtQuellenZusammen(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(shipped(t), "commands", "k-a.md"), "mitgeliefert\n")
	writeFile(t, filepath.Join(shipped(t), "commands", "k-b.md"), "mitgeliefert\n")
	writeFile(t, filepath.Join(root, ProjectDirName, "commands", "k-b.md"), "projekteigen\n")
	writeFile(t, filepath.Join(root, ProjectDirName, "commands", "k-c.md"), "projekteigen\n")

	entries := ResolveRegistry(root, KindCommands)
	if len(entries) != 3 {
		t.Fatalf("%d Einträge, erwartet 3: %+v", len(entries), entries)
	}

	if got := entryFor(t, entries, "k-a.md").Origin; got != "dist" {
		t.Errorf("k-a.md Origin = %q, erwartet dist", got)
	}

	overridden := entryFor(t, entries, "k-b.md")
	if overridden.Origin != "override" {
		t.Errorf("k-b.md Origin = %q, erwartet override", overridden.Origin)
	}
	if want := filepath.Join(root, ProjectDirName, "commands", "k-b.md"); overridden.Path != want {
		t.Errorf("k-b.md Path = %q, erwartet die projekteigene Fassung", overridden.Path)
	}

	if got := entryFor(t, entries, "k-c.md").Origin; got != "local" {
		t.Errorf("k-c.md Origin = %q, erwartet local", got)
	}
}

// Namensräume werden Datei für Datei verrechnet: ein Projekt kann eine
// einzelne Datei aus _shared/ ersetzen, ohne den Rest zu kopieren.
func TestResolveRegistryLoestNamensraeumeAuf(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(shipped(t), "commands", "_shared", "a.md"), "mitgeliefert\n")
	writeFile(t, filepath.Join(shipped(t), "commands", "_shared", "b.md"), "mitgeliefert\n")
	writeFile(t, filepath.Join(root, ProjectDirName, "commands", "_shared", "b.md"), "projekteigen\n")

	entries := ResolveRegistry(root, KindCommands)
	if len(entries) != 2 {
		t.Fatalf("%d Einträge, erwartet 2: %+v", len(entries), entries)
	}
	if got := entryFor(t, entries, "_shared/a.md").Origin; got != "dist" {
		t.Errorf("_shared/a.md Origin = %q, erwartet dist", got)
	}
	if got := entryFor(t, entries, "_shared/b.md").Origin; got != "override" {
		t.Errorf("_shared/b.md Origin = %q, erwartet override", got)
	}
}

func TestResolveRegistryErhaeltAuditNamensraumUndOverlaySchaltetAb(t *testing.T) {
	root := t.TempDir()
	name := "_audit/review-scan-triage.md"
	writeFile(t, filepath.Join(shipped(t), "commands", filepath.FromSlash(name)), "# Modul\n")

	active := ActiveRegistry(root, KindCommands)
	if len(active) != 1 || active[0].Name != name {
		t.Fatalf("aktive Commands = %+v, erwartet %s", active, name)
	}

	writeFile(t, filepath.Join(root, ProjectDirName, "commands", filepath.FromSlash(name)), "# abgeschaltet\n")
	entry := entryFor(t, ResolveRegistry(root, KindCommands), name)
	if !entry.Disabled || entry.Origin != "override" {
		t.Fatalf("Overlay = %+v, erwartet abgeschalteten Override", entry)
	}
	if got := len(ActiveRegistry(root, KindCommands)); got != 0 {
		t.Fatalf("%d aktive Commands, erwartet 0", got)
	}
}

func TestResolveRegistryErhaeltDocsNamensraum(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(shipped(t), "commands", "k-docs.md"), "# k-docs\n")
	writeFile(t, filepath.Join(shipped(t), "commands", "k-docs-code.md"), "# k-docs-code\n")
	writeFile(t, filepath.Join(shipped(t), "commands", "_docs", "code.md"), "# Docs-Modul: Code\n")

	entries := ResolveRegistry(root, KindCommands)
	for _, name := range []string{"k-docs.md", "k-docs-code.md", "_docs/code.md"} {
		if got := entryFor(t, entries, name).Origin; got != "dist" {
			t.Errorf("%s Origin = %q, erwartet dist", name, got)
		}
	}
}

// README.md beschreibt das Verzeichnis und ist kein Command.
func TestResolveRegistryUebergehtReadme(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(shipped(t), "commands", "k-a.md"), "mitgeliefert\n")
	writeFile(t, filepath.Join(root, ProjectDirName, "commands", "README.md"), "# commands\n")

	entries := ResolveRegistry(root, KindCommands)
	if len(entries) != 1 {
		t.Errorf("%d Einträge, erwartet 1: %+v", len(entries), entries)
	}
}

func TestResolveRegistryLeereDateiSchaltetAb(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(shipped(t), "commands", "k-a.md"), "mitgeliefert\n")
	writeFile(t, filepath.Join(root, ProjectDirName, "commands", "k-a.md"), "# hier nicht gebraucht\n")

	if !entryFor(t, ResolveRegistry(root, KindCommands), "k-a.md").Disabled {
		t.Error("leere projekteigene Datei hat den Eintrag nicht abgeschaltet")
	}
	if got := len(ActiveRegistry(root, KindCommands)); got != 0 {
		t.Errorf("%d aktive Einträge, erwartet 0", got)
	}
}

// Ein Skill ist eine Einheit: erst die SKILL.md macht ein Verzeichnis dazu, und
// überlagert wird das ganze Verzeichnis.
func TestResolveRegistrySkills(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(shipped(t), "skills", "alpha", skillFileName), "# Alpha\n")
	writeFile(t, filepath.Join(shipped(t), "skills", "beta", skillFileName), "# Beta\n")
	writeFile(t, filepath.Join(shipped(t), "skills", "kein-skill", "NOTIZ.md"), "kein Skill\n")
	writeFile(t, filepath.Join(root, ProjectDirName, "skills", "beta", skillFileName), "# Beta projekteigen\n")
	writeFile(t, filepath.Join(root, ProjectDirName, "skills", "gamma", skillFileName), "# Gamma\n")

	entries := ResolveRegistry(root, KindSkills)
	if len(entries) != 3 {
		t.Fatalf("%d Skills, erwartet 3 (kein-skill zählt nicht): %+v", len(entries), entries)
	}

	beta := entryFor(t, entries, "beta")
	if beta.Origin != "override" {
		t.Errorf("beta Origin = %q, erwartet override", beta.Origin)
	}
	if !beta.IsDir {
		t.Error("beta ist nicht als Verzeichnis markiert")
	}
	if got := entryFor(t, entries, "gamma").Origin; got != "local" {
		t.Errorf("gamma Origin = %q, erwartet local", got)
	}
}

func TestResolveRegistrySkillLeereSkillDateiSchaltetAb(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(shipped(t), "skills", "alpha", skillFileName), "# Alpha\n")
	writeFile(t, filepath.Join(root, ProjectDirName, "skills", "alpha", skillFileName), "# hier nicht gebraucht\n")

	if !entryFor(t, ResolveRegistry(root, KindSkills), "alpha").Disabled {
		t.Error("leere SKILL.md hat den Skill nicht abgeschaltet")
	}
}

func TestRegistrySourcePresent(t *testing.T) {
	root := t.TempDir()
	if RegistrySourcePresent(root, KindCommands) {
		t.Error("ohne Quellverzeichnis darf nichts gemeldet werden")
	}

	writeFile(t, filepath.Join(root, ProjectDirName, "commands", "k-a.md"), "projekteigen\n")
	if !RegistrySourcePresent(root, KindCommands) {
		t.Error("das lokale Verzeichnis allein muss als Quelle zählen")
	}
}

// Der frühere Ort k-playbook-local/<sorte>/ wird seit Task 129 nicht mehr
// gelesen: weder ein neuer Name noch eine abschaltende leere Datei wirkt dort.
func TestResolveRegistryLiestAltenOrtNicht(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(shipped(t), "commands", "k-a.md"), "mitgeliefert\n")
	writeFile(t, filepath.Join(shipped(t), "skills", "alpha", skillFileName), "# Alpha\n")
	writeFile(t, filepath.Join(root, LocalDirName, "commands", "k-a.md"), "")
	writeFile(t, filepath.Join(root, LocalDirName, "commands", "k-alt.md"), "alt\n")
	writeFile(t, filepath.Join(root, LocalDirName, "skills", "alpha", skillFileName), "")
	writeFile(t, filepath.Join(root, LocalDirName, "skills", "alt", skillFileName), "# Alt\n\nText.\n")

	commands := ResolveRegistry(root, KindCommands)
	if len(commands) != 1 || commands[0].Name != "k-a.md" || commands[0].Origin != "dist" || commands[0].Disabled {
		t.Errorf("Commands: %+v, erwartet nur k-a.md mitgeliefert", commands)
	}
	skills := ResolveRegistry(root, KindSkills)
	if len(skills) != 1 || skills[0].Name != "alpha" || skills[0].Origin != "dist" || skills[0].Disabled {
		t.Errorf("Skills: %+v, erwartet nur alpha mitgeliefert", skills)
	}
	if want := filepath.Join(root, ProjectDirName, "commands"); ProjectAssetDir(root, KindCommands) != want {
		t.Errorf("ProjectAssetDir = %q, erwartet %q", ProjectAssetDir(root, KindCommands), want)
	}
}

// Namensräume am neuen Ort werden Datei für Datei verrechnet.
func TestResolveRegistryNamensraumAmNeuenOrt(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(shipped(t), "commands", "_x", "eins.md"), "mitgeliefert\n")
	writeFile(t, filepath.Join(shipped(t), "commands", "_x", "zwei.md"), "mitgeliefert\n")
	writeFile(t, filepath.Join(root, ProjectDirName, "commands", "_x", "eins.md"), "projekteigen\n")

	entries := ResolveRegistry(root, KindCommands)
	if got := entryFor(t, entries, "_x/eins.md"); got.Origin != "override" || got.Path != filepath.Join(root, ProjectDirName, "commands", "_x", "eins.md") {
		t.Errorf("_x/eins.md: %+v", got)
	}
	if got := entryFor(t, entries, "_x/zwei.md").Origin; got != "dist" {
		t.Errorf("_x/zwei.md Origin = %q, erwartet dist", got)
	}
}
