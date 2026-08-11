package project

import (
	"fmt"
	"os"
	"path/filepath"
)

// LocalDirName ist das Verzeichnis fuer alles, was dem Projekt gehoert. Es
// liegt neben der Installation, damit diese vollstaendig ersetzbar bleibt.
const LocalDirName = "k-playbook-local"

// LocalEntry ist ein Bestandteil der lokalen Struktur. Verzeichnisse bekommen
// eine README, weil Git leere Verzeichnisse nicht speichert — ohne sie waeren
// sie nach einem Clone des Projekts verschwunden.
type LocalEntry struct {
	Path    string `json:"path"`
	IsFile  bool   `json:"isFile"`
	Purpose string `json:"purpose"`
	// Private haelt den Inhalt aus der Versionskontrolle heraus, ueber eine
	// .gitignore im Verzeichnis selbst. Das Verzeichnis bleibt dadurch
	// auffindbar, ohne dass die Projekt-.gitignore angefasst werden muss.
	Private bool `json:"private"`
}

// LocalStructure beschreibt, was ein Projekt braucht.
//
// rules, reviews und checks sind die drei Overlay-Sorten: eine gleichnamige
// lokale Datei ersetzt die mitgelieferte, overlay.<kind>.disabled schaltet ab.
func LocalStructure() []LocalEntry {
	return []LocalEntry{
		{Path: "rules", Purpose: "Projekteigene Enforcement-Regeln. Ergaenzen die mitgelieferten aus " + PlaybookDirName + "/rules/; gleicher Dateiname ersetzt."},
		{Path: "reviews", Purpose: "Projekteigene Review-Rezepte, benannt als review-<name>.md."},
		{Path: "checks", Purpose: "Projekteigene Checks als *.sh, ausgefuehrt ueber " + PlaybookDirName + "/bin/k-check."},
		{Path: "results", Purpose: "Alles, was Reviews erzeugen: Ergebnisse je Familie und Datum, dazu log.md und known-decisions.md."},
		{Path: "docs", Purpose: "Projektwissen fuer AI-Sessions, erzeugt von /k-code2docs; Tool-Steckbriefe unter libs/."},
		{Path: "guidelines", Purpose: "Projektvorgaben, auf die Commands und Reviews sich beziehen."},
		{Path: "tasks", Purpose: "Offene Tasks, nummeriert als <nummer>-<name>.md."},
		{Path: filepath.Join("tasks", "done"), Purpose: "Erledigte Tasks, nach der Ausfuehrung hierher verschoben."},
		{
			Path:    "priv",
			Purpose: "Platz fuer eigene Notizen, Zwischenstaende und alles, was nur dich angeht.\n\nDer Inhalt bleibt aus der Versionskontrolle heraus: die .gitignore in diesem\nVerzeichnis schliesst alles aus, ausser sich selbst und dieser README. Du kannst\nhier also ablegen, was du willst, ohne es aus Versehen zu committen.",
			Private: true,
		},
		{Path: InstructionsFileName, IsFile: true},
		{Path: "TODO.md", IsFile: true},
	}
}

// LocalEntryStatus ist der gepruefte Zustand eines Eintrags.
type LocalEntryStatus struct {
	LocalEntry
	Present bool `json:"present"`
}

// LocalDir ist das lokale Verzeichnis eines Projekts.
func LocalDir(projectDir string) string {
	return filepath.Join(projectDir, LocalDirName)
}

// CheckLocal prueft die Struktur, ohne etwas zu veraendern.
func CheckLocal(projectDir string) []LocalEntryStatus {
	root := LocalDir(projectDir)

	statuses := make([]LocalEntryStatus, 0, len(LocalStructure()))
	for _, entry := range LocalStructure() {
		path := filepath.Join(root, entry.Path)
		present := isDir(path)
		if entry.IsFile {
			present = fileExists(path)
		}
		statuses = append(statuses, LocalEntryStatus{LocalEntry: entry, Present: present})
	}
	return statuses
}

// LocalOK meldet, ob die Struktur vollstaendig ist.
func LocalOK(statuses []LocalEntryStatus) bool {
	for _, status := range statuses {
		if !status.Present {
			return false
		}
	}
	return len(statuses) > 0
}

// CreateLocal legt fehlende Teile der Struktur an. Vorhandenes bleibt
// unberuehrt, auch READMEs mit eigenem Text.
func CreateLocal(projectDir string) ([]LocalEntryStatus, error) {
	root := LocalDir(projectDir)

	for _, entry := range LocalStructure() {
		path := filepath.Join(root, entry.Path)

		if entry.IsFile {
			if err := writeIfMissing(path, fileTemplate(entry)); err != nil {
				return CheckLocal(projectDir), err
			}
			continue
		}

		if err := os.MkdirAll(path, 0o755); err != nil {
			return CheckLocal(projectDir), fmt.Errorf("%s anlegen: %w", entry.Path, err)
		}
		readme := filepath.Join(path, "README.md")
		if err := writeIfMissing(readme, readmeTemplate(entry)); err != nil {
			return CheckLocal(projectDir), err
		}

		if entry.Private {
			if err := writeIfMissing(filepath.Join(path, ".gitignore"), privateGitignore()); err != nil {
				return CheckLocal(projectDir), err
			}
		}
	}

	return CheckLocal(projectDir), nil
}

// writeIfMissing schreibt nur, wenn nichts da ist. Projektinhalte werden nie
// ueberschrieben.
func writeIfMissing(path string, content string) error {
	if pathExists(path) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("%s anlegen: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("%s schreiben: %w", path, err)
	}
	return nil
}

func readmeTemplate(entry LocalEntry) string {
	return fmt.Sprintf("# %s\n\n%s\n\nDieses Verzeichnis gehoert dem Projekt und wird von einem Update nie angefasst.\n",
		entry.Path, entry.Purpose)
}

// privateGitignore schliesst den gesamten Inhalt aus, laesst das Verzeichnis
// selbst aber im Repository sichtbar.
func privateGitignore() string {
	return "# Inhalt bleibt privat; das Verzeichnis selbst bleibt versioniert.\n*\n!.gitignore\n!README.md\n"
}

// fileTemplate liefert den Erstinhalt eines Datei-Eintrags.
func fileTemplate(entry LocalEntry) string {
	if entry.Path == InstructionsFileName {
		return instructionsTemplate()
	}
	return todoTemplate()
}

// instructionsTemplate ist die projekteigene Instruktionsebene. Sie wird von
// jedem Assistenten gelesen, der `k-playbook context` folgt — nach der
// mitgelieferten Ebene, deren Aussagen sie ergaenzen oder ueberstimmen kann.
func instructionsTemplate() string {
	return `# Projektregeln

Diese Datei gilt nur fuer dieses Projekt. Sie wird nach der mitgelieferten
Ebene gelesen und kann deren Aussagen ergaenzen oder ueberstimmen.

Was hier hineingehoert: Aufbau und Besonderheiten des Projekts, Konventionen,
wiederkehrende Ablaeufe, alles was ein Assistent in jeder Sitzung wissen sollte.

Was nicht: allgemeine k-playbook-Regeln — die stehen in der mitgelieferten
Ebene und werden bei jedem Update aktualisiert.
`
}

func todoTemplate() string {
	return "# TODO\n\nOffene Punkte des Projekts. Eintraege kommen ueber /k-todo hinzu.\n"
}
