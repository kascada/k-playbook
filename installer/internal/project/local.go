package project

import (
	"fmt"
	"os"
	"path/filepath"
)

// LocalDirName ist das Verzeichnis für alles, was dem Projekt gehört. Es
// liegt neben der Installation, damit diese vollständig ersetzbar bleibt.
const LocalDirName = "k-playbook-local"

// LocalEntry ist ein Bestandteil der lokalen Struktur. Verzeichnisse bekommen
// eine README, weil Git leere Verzeichnisse nicht speichert — ohne sie wären
// sie nach einem Clone des Projekts verschwunden.
type LocalEntry struct {
	Path    string `json:"path"`
	IsFile  bool   `json:"isFile"`
	Purpose string `json:"purpose"`
	// Private markiert Verzeichnisse, deren Inhalt üblicherweise nicht ins
	// Repository gehört. k-playbook erzwingt das nicht: ob der Inhalt
	// versioniert wird, entscheidet das Projekt. Wer ihn heraushalten will,
	// legt eine .gitignore im Verzeichnis selbst an — die README sagt, wie.
	// Das Feld bleibt, weil es genau die Verzeichnisse benennt, für die diese
	// Wahl überhaupt zur Debatte steht.
	Private bool `json:"private"`
	// PrivateByDefault ist die eine Ausnahme davon: der Eintrag wird bei der
	// Installation schon privat angelegt, statt die Wahl offen zu lassen. Nur
	// results/ trägt das.
	//
	// Begründung: Bei priv/ und material/ geht es um Geschmack, dort ist die
	// Zurückhaltung richtig. Ein Werkzeug, das gefundene Secrets im Klartext
	// ins Repository des Nutzers schreibt, ist dagegen ein Fehler von
	// k-playbook und keine Projektentscheidung — und die Rohausgaben sind nur
	// der schärfste Fall: ein Review ist aus dem Code wiederholbar, sein
	// Ergebnis ist ein Stand von einem Rechner.
	//
	// Eine Erzwingung ist es trotzdem nicht: der Zustand bleibt in der
	// Oberfläche umschaltbar, und geschrieben wird die verwaltete .gitignore
	// nur beim erstmaligen Anlegen des Verzeichnisses (siehe CreateLocal).
	PrivateByDefault bool `json:"privateByDefault"`
}

// LocalStructure beschreibt, was ein Projekt braucht.
//
// rules, reviews, checks, commands und skills sind die Overlay-Sorten: ein
// gleichnamiger lokaler Eintrag ersetzt den mitgelieferten, ein leerer schaltet
// ihn ab. Aufgelöst wird das in context.go (rules, reviews, checks) und in
// registry.go (commands, skills).
func LocalStructure() []LocalEntry {
	return []LocalEntry{
		{Path: "rules", Purpose: "Projekteigene Enforcement-Regeln. Ergänzen die mitgelieferten aus " + PlaybookDirName + "/rules/; gleicher Dateiname ersetzt."},
		{Path: "reviews", Purpose: "Projekteigene Review-Rezepte, benannt als review-<name>.md."},
		{Path: "checks", Purpose: "Projekteigene Checks als *.sh, ausgeführt über " + PlaybookDirName + "/bin/k-check."},
		{Path: "commands", Purpose: "Projekteigene Commands als *.md. Ergänzen die mitgelieferten aus " + PlaybookDirName + "/commands/; gleicher Name ersetzt, eine leere Datei schaltet ab. Unterverzeichnisse bilden Namensräume und werden Datei für Datei verrechnet."},
		{Path: "skills", Purpose: "Projekteigene Skills, je ein Verzeichnis mit SKILL.md darin. Ergänzen die mitgelieferten aus " + PlaybookDirName + "/skills/; gleicher Verzeichnisname ersetzt den Skill als Ganzes, eine leere SKILL.md schaltet ihn ab."},
		{
			Path:             "results",
			Private:          true,
			PrivateByDefault: true,
			Purpose: "Alles, was Reviews erzeugen: Ergebnisse je Familie und Datum, dazu log.md.\n\n" +
				"Der Inhalt bleibt aus der Versionskontrolle. Ein Review ist aus dem Code wiederholbar —\n" +
				"sein Ergebnis ist ein Stand von diesem Rechner und kein Projektwissen; log.md sagt\n" +
				"außerdem, wer wann was gescannt hat. Was vom Ergebnis Projektwissen ist, wandert ohnehin\n" +
				"heraus: in known-decisions.md und in die Tasks einer Remediation.\n\n" +
				"k-playbook legt dafür beim erstmaligen Anlegen dieses Verzeichnisses eine .gitignore mit\n" +
				"diesem Inhalt an:\n\n" +
				"    *\n    !.gitignore\n    !README.md\n\n" +
				"Der Block „Lokale Einstellungen\" in der Oberfläche zeigt den gemessenen Ist-Zustand und\n" +
				"schaltet ihn um — auch wieder zurück; einmal umgeschaltet, bleibt es dabei. Was bereits\n" +
				"committet ist, nimmt erst ein `git rm --cached` wieder heraus — eine .gitignore allein\n" +
				"wirkt auf getrackte Dateien nicht. Und was schon gepusht wurde, bleibt in der Historie.",
		},
		{Path: "docs", Purpose: "Projektwissen für AI-Sessions, nach Herkunft getrennt: code/ von /k-code2docs, libs/ von /k-tools-scan, extracted/ von /k-docs-extract, manual/ von Hand. Die drei erzeugten Verzeichnisse legt jeweils ihr Erzeuger beim ersten Lauf an. Die README dieses Verzeichnisses ist der einzige Index; /k-docs-index schreibt sie neu."},
		{Path: filepath.Join("docs", "manual"), Purpose: "Von Hand gepflegte Dokumentation. Kein Command schreibt hier Doc-Dateien hinein; gelistet wird sie über den Index in ../README.md."},
		{Path: "guidelines", Purpose: "Projektvorgaben, auf die Commands und Reviews sich beziehen."},
		{Path: "tasks", Purpose: "Offene Tasks, nummeriert als <nummer>-<name>.md."},
		{Path: filepath.Join("tasks", "done"), Purpose: "Erledigte Tasks, nach der Ausführung hierher verschoben."},
		{
			Path:    "priv",
			Purpose: "Platz für eigene Notizen, Zwischenstände und alles, was nur dich angeht.\n\nDer Inhalt wird ganz normal mitversioniert. Ob er das soll, entscheidet das\nProjekt: Der Block „Lokale Einstellungen\" in der Oberfläche zeigt den gemessenen\nIst-Zustand dieses Verzeichnisses und schaltet ihn um — er legt die .gitignore\nan und nimmt bereits versionierte Dateien aus dem Index.\n\nVon Hand geht es genauso: eine .gitignore in diesem Verzeichnis mit diesem\nInhalt:\n\n    *\n    !.gitignore\n    !README.md\n\nDann bleibt der Inhalt draußen und das Verzeichnis selbst sichtbar. Was bereits\ncommittet ist, nimmt erst ein `git rm --cached` wieder heraus — eine .gitignore\nallein wirkt auf getrackte Dateien nicht. Und was schon gepusht wurde, bleibt in\nder Historie; das macht kein Schalter rückgängig.",
			Private: true,
		},
		{
			Path:    "material",
			Purpose: "Rohmaterial als Quelle für Docs: Chat-Mitschnitte, Notizen, Zulieferungen.\nEs wird nie indiziert und von keinem Command geschrieben — gelesen wird es von\n/k-docs-extract, geschrieben nach docs/extracted/.\n\nDer Inhalt wird ganz normal mitversioniert. Rohmaterial enthält typischerweise\nTokens, Pfade und Namen; soll es nicht ins Repository, schaltet der Block\n„Lokale Einstellungen\" in der Oberfläche dieses Verzeichnis um — er legt die\n.gitignore an und nimmt bereits versionierte Dateien aus dem Index.\n\nVon Hand geht es genauso: eine .gitignore in diesem Verzeichnis mit diesem\nInhalt:\n\n    *\n    !.gitignore\n    !README.md\n\nWas bereits committet ist, nimmt erst ein `git rm --cached` wieder heraus. Und\nwas schon gepusht wurde, bleibt in der Historie.",
			Private: true,
		},
		{Path: InstructionsFileName, IsFile: true},
		{Path: "TODO.md", IsFile: true},
	}
}

// LocalEntryStatus ist der geprüfte Zustand eines Eintrags.
type LocalEntryStatus struct {
	LocalEntry
	Present bool `json:"present"`
}

// LocalDir ist das lokale Verzeichnis eines Projekts.
func LocalDir(projectDir string) string {
	return filepath.Join(projectDir, LocalDirName)
}

// CheckLocal prüft die Struktur, ohne etwas zu verändern.
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

// LocalOK meldet, ob die Struktur vollständig ist.
func LocalOK(statuses []LocalEntryStatus) bool {
	for _, status := range statuses {
		if !status.Present {
			return false
		}
	}
	return len(statuses) > 0
}

// CreateLocal legt fehlende Teile der Struktur an. Vorhandenes bleibt
// unberührt, auch READMEs mit eigenem Text.
//
// Für Einträge mit PrivateByDefault schreibt CreateLocal zusätzlich die
// verwaltete .gitignore — aber nur, wenn das Verzeichnis in genau diesem Lauf
// entsteht. Deshalb wird vor os.MkdirAll geprüft, ob es schon da ist; MkdirAll
// selbst meldet das nicht. Zwei Gründe:
//
//   - makePublic() entfernt die verwaltete Datei bewusst. Ein späterer
//     CreateLocal()-Lauf — jeder /k-gui-Start, jedes „Struktur anlegen" —
//     brächte sie sonst still zurück und überginge die Entscheidung des
//     Projekts.
//   - Bestandsprojekte mit getrackten Dateien unter results/ landeten sonst im
//     Zustand PrivacyPartial: Regel greift, Dateien stehen im Index. Wer nur
//     aktualisiert, soll davon nichts merken.
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

		fresh := !pathExists(path)
		if err := os.MkdirAll(path, 0o755); err != nil {
			return CheckLocal(projectDir), fmt.Errorf("%s anlegen: %w", entry.Path, err)
		}
		readme := filepath.Join(path, "README.md")
		if err := writeIfMissing(readme, readmeTemplate(entry)); err != nil {
			return CheckLocal(projectDir), err
		}
		if fresh && entry.PrivateByDefault {
			ignore := filepath.Join(path, PrivateIgnoreFile)
			if err := writeIfMissing(ignore, managedIgnoreContent()); err != nil {
				return CheckLocal(projectDir), err
			}
		}
	}

	return CheckLocal(projectDir), nil
}

// writeIfMissing schreibt nur, wenn nichts da ist. Projektinhalte werden nie
// überschrieben.
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
	return fmt.Sprintf("# %s\n\n%s\n\nDieses Verzeichnis gehört dem Projekt und wird von einem Update nie angefasst.\n",
		entry.Path, entry.Purpose)
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
// mitgelieferten Ebene, deren Aussagen sie ergänzen oder überstimmen kann.
func instructionsTemplate() string {
	return `# Projektregeln

Diese Datei gilt nur für dieses Projekt. Sie wird nach der mitgelieferten
Ebene gelesen und kann deren Aussagen ergänzen oder überstimmen.

Was hier hineingehört: Aufbau und Besonderheiten des Projekts, Konventionen,
wiederkehrende Abläufe, alles was ein Assistent in jeder Sitzung wissen sollte.

Was nicht: allgemeine k-playbook-Regeln — die stehen in der mitgelieferten
Ebene und werden bei jedem Update aktualisiert.
`
}

func todoTemplate() string {
	return "# TODO\n\nOffene Punkte des Projekts. Einträge kommen über /k-todo hinzu.\n"
}
