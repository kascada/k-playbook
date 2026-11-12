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
	// PrivateByDefault ist die Ausnahme davon: der Eintrag wird bei der
	// Installation schon privat angelegt, statt die Wahl offen zu lassen.
	// results/ trägt das.
	//
	// Begründung: Bei priv/ geht es um Geschmack, dort ist die Zurückhaltung
	// richtig. Ein Werkzeug, das gefundene Secrets im Klartext ins Repository
	// des Nutzers schreibt, ist dagegen ein Fehler von k-playbook und keine
	// Projektentscheidung — und die Rohausgaben sind nur der schärfste Fall:
	// ein Review ist aus dem Code wiederholbar, sein Ergebnis ist ein Stand von
	// einem Rechner.
	//
	// Eine Erzwingung ist es trotzdem nicht: der Zustand bleibt in der
	// Oberfläche umschaltbar, und geschrieben wird die verwaltete .gitignore
	// nur beim erstmaligen Anlegen des Verzeichnisses (siehe CreateLocal).
	PrivateByDefault bool `json:"privateByDefault"`
}

// LocalStructure beschreibt, was ein Projekt braucht.
//
// docs, tasks, tasks/done, data und material stehen nicht mehr hier (Task
// 107): ihr Inhalt liegt in der Projekt-Collection. commands und skills
// ebenso nicht (Task 129): projekteigene Commands und Skills liegen in
// .k-playbook/commands/ und .k-playbook/skills/ (registry.go), die das
// Einrichten nicht anlegt. rules hat Task 135 entfernt, checks Task 134:
// projekteigene Checks gibt es bis zu ihrer neuen Form nicht, k-check führt
// nur die mitgelieferten aus. guidelines ebenso nicht (Todo #40):
// projekteigene Guidelines liegen unter keph:/docs/guidelines/, /k-upgrade
// löscht das alte Verzeichnis auch mit Inhalt. reviews ebenso nicht (Task
// 143): projekteigene Review-Rezepte liegen wie projekteigene Instructions
// und Includes in der Collection des Projekts unter keph:/k-playbook/<sorte>/
// (projectlayer.go). Die Quellenkonfiguration ebenso nicht (Task 144): sie
// liegt als Quellenkonfiguration des Versionsinventars im Dokument
// keph:/k-playbook/inventory.md (versionsources.go), angelegt von
// /k-doc-inventory bei der ersten bestätigten Ergänzung; eine alte Datei
// meldet die Gesundheitsprüfung als Altbestand, /k-upgrade zieht sie um.
// /k-upgrade räumt die leeren Verzeichnisse eines
// Bestandsprojekts ab, dazu die Reste der früheren Wissensablage auf der
// Platte (Task 123).
func LocalStructure() []LocalEntry {
	return []LocalEntry{
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
		{
			Path:    "priv",
			Purpose: "Platz für eigene Notizen, Zwischenstände und alles, was nur dich angeht.\n\nDer Inhalt wird ganz normal mitversioniert. Ob er das soll, entscheidet das\nProjekt: Der Block „Lokale Einstellungen\" in der Oberfläche zeigt den gemessenen\nIst-Zustand dieses Verzeichnisses und schaltet ihn um — er legt die .gitignore\nan und nimmt bereits versionierte Dateien aus dem Index.\n\nVon Hand geht es genauso: eine .gitignore in diesem Verzeichnis mit diesem\nInhalt:\n\n    *\n    !.gitignore\n    !README.md\n\nDann bleibt der Inhalt draußen und das Verzeichnis selbst sichtbar. Was bereits\ncommittet ist, nimmt erst ein `git rm --cached` wieder heraus — eine .gitignore\nallein wirkt auf getrackte Dateien nicht. Und was schon gepusht wurde, bleibt in\nder Historie; das macht kein Schalter rückgängig.",
			Private: true,
		},
		{Path: InstructionsFileName, IsFile: true},
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
// unberührt, auch READMEs mit eigenem Text. Das ist der **ausdrückliche**
// Weg: der Knopf „Anlegen" (POST /api/local). Beim Start läuft er nicht —
// dort greift EnsureLocal, und das nur, wenn das Verzeichnis ganz fehlt.
//
// Für Einträge mit PrivateByDefault schreibt CreateLocal zusätzlich die
// verwaltete .gitignore — aber nur, wenn das Verzeichnis in genau diesem Lauf
// entsteht. Deshalb wird vor os.MkdirAll geprüft, ob es schon da ist; MkdirAll
// selbst meldet das nicht. Zwei Gründe:
//
//   - makePublic() entfernt die verwaltete Datei bewusst. Ein späterer
//     CreateLocal()-Lauf — jedes „Struktur anlegen" — brächte sie sonst
//     still zurück und überginge die Entscheidung des Projekts.
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

// EnsureLocal legt die Struktur an, wenn k-playbook-local/ ganz fehlt — der
// **selbsttätige** Weg beim Start. Das zweite Ergebnis meldet, ob angelegt
// wurde; die Zustände sind die aus CreateLocal.
//
// Nur das ganz fehlende Verzeichnis, nicht der fehlende Teil: CreateLocal ist
// rein additiv, und additiv heißt auch, dass eine bewusst gelöschte
// Strukturdatei bei jedem Start zurückkäme. Existiert das Verzeichnis — auch
// unvollständig —, bleibt es deshalb beim Knopf. Damit braucht es hier keinen
// Vorher-nachher-Vergleich über CheckLocal: entweder entsteht alles, oder
// nichts wird angefasst. Eine bewusste Umschaltung auf öffentlich bleibt so
// ebenfalls stehen — sie setzt ein vorhandenes Verzeichnis voraus.
func EnsureLocal(projectDir string) ([]LocalEntryStatus, bool, error) {
	if pathExists(LocalDir(projectDir)) {
		return nil, false, nil
	}
	statuses, err := CreateLocal(projectDir)
	return statuses, true, err
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
//
// Jeder Datei-Eintrag mit einem eigenen Format braucht hier einen eigenen
// Zweig. Der default:-Zweig baut aus dem Zweck des Eintrags einen neutralen
// Rumpf — Überschrift plus Zweck —, damit ein neuer Eintrag ohne Zweig nicht
// still das Format eines fremden bekommt.
func fileTemplate(entry LocalEntry) string {
	switch entry.Path {
	case InstructionsFileName:
		return instructionsTemplate()
	default:
		return genericFileTemplate(entry)
	}
}

// genericFileTemplate ist der Rumpf für Datei-Einträge ohne eigenes Format.
func genericFileTemplate(entry LocalEntry) string {
	if entry.Purpose == "" {
		return fmt.Sprintf("# %s\n", entry.Path)
	}
	return fmt.Sprintf("# %s\n\n%s\n", entry.Path, entry.Purpose)
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
