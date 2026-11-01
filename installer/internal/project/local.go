package project

import (
	"fmt"
	"os"
	"path/filepath"
)

// LocalDirName ist das Verzeichnis für alles, was dem Projekt gehört. Es
// liegt neben der Installation, damit diese vollständig ersetzbar bleibt.
const LocalDirName = "k-playbook-local"

// VersionSourcesFileName ist die Quellenkonfiguration des Versionsinventars.
// Sie liegt neben der Instruktionsdatei in k-playbook-local/ und wird von einem
// Update nie überschrieben. Vertrag: docs/version-inventory.md.
const VersionSourcesFileName = "version-sources.yaml"

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
	// einem Rechner. cache/ steht nicht mehr in der Struktur (Task 107), bekommt
	// aus einem verwandten Grund aber dieselbe Datei, siehe ensureCacheDir.
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
//
// docs, tasks, tasks/done, data, material, inbox, queue, knowledge und cache
// stehen nicht mehr hier (Task 107): ihr Inhalt liegt in der Projekt-Collection
// oder zieht als Nächstes dorthin. Was die Wissenswerkzeuge bis dahin noch auf
// die Platte schreiben, legt sein Verzeichnis beim ersten Schreiben selbst an;
// /k-upgrade räumt die leeren Verzeichnisse eines Bestandsprojekts ab.
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
		{Path: "guidelines", Purpose: "Projektvorgaben, auf die Commands und Reviews sich beziehen."},
		{
			Path:    "priv",
			Purpose: "Platz für eigene Notizen, Zwischenstände und alles, was nur dich angeht.\n\nDer Inhalt wird ganz normal mitversioniert. Ob er das soll, entscheidet das\nProjekt: Der Block „Lokale Einstellungen\" in der Oberfläche zeigt den gemessenen\nIst-Zustand dieses Verzeichnisses und schaltet ihn um — er legt die .gitignore\nan und nimmt bereits versionierte Dateien aus dem Index.\n\nVon Hand geht es genauso: eine .gitignore in diesem Verzeichnis mit diesem\nInhalt:\n\n    *\n    !.gitignore\n    !README.md\n\nDann bleibt der Inhalt draußen und das Verzeichnis selbst sichtbar. Was bereits\ncommittet ist, nimmt erst ein `git rm --cached` wieder heraus — eine .gitignore\nallein wirkt auf getrackte Dateien nicht. Und was schon gepusht wurde, bleibt in\nder Historie; das macht kein Schalter rückgängig.",
			Private: true,
		},
		{Path: InstructionsFileName, IsFile: true},
		{
			Path:   VersionSourcesFileName,
			IsFile: true,
			Purpose: "Versionsquellen für `k-playbook inventory`: zusätzlich lesbare Wurzeln außerhalb\n" +
				"des Projekts und zusätzliche Quellen über die Standarderkennung hinaus.\n\n" +
				"Die Datei ist handgepflegt. k-playbook schreibt nur nach ausdrücklicher Bestätigung\n" +
				"in sie, und dann ausschließlich ergänzend: bestehende Einträge, Kommentare und\n" +
				"Reihenfolge bleiben erhalten. Ohne Einträge gelten die Standardquellen unterhalb\n" +
				"der Projektwurzel.\n\n" +
				"Sie wird beim Einrichten als gültige, leere Konfiguration angelegt — nicht erst beim\n" +
				"ersten Lauf des Sammlers —, damit ihr Zustand im Briefing (`k-playbook briefing`,\n" +
				"Feld `versionSources`) von Anfang an eine Antwort hat. Vollständige Beschreibung:\n" +
				PlaybookDirName + "/docs/version-inventory.md.",
		},
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

// ensureCacheDir legt cache/ an und schreibt dabei dieselbe verwaltete
// .gitignore, die CreateLocal für einen Eintrag mit PrivateByDefault schreibt.
//
// cache/ steht nicht in LocalStructure() (Task 107): es entsteht nur hier, wenn
// das Wissenstor einen Index schreibt — und das tut es nur, wenn die
// Wissensablage etwas zu indizieren hat (siehe open in knowledge_index.go).
// Ohne die .gitignore wäre der abgeleitete Index committierbar — ein Release
// mit `git add -A` nähme den Chunk-Abzug der ganzen Doku mit. Der Inhalt ist
// aus dem Projekt abgeleitet und entsteht jederzeit neu; Ableitbares gehört
// nicht ins Repository.
//
// Nachträglich geschrieben wird nichts: ist cache/ schon da, bleibt es, wie
// es ist — auch eine von Hand entfernte oder geänderte .gitignore.
func ensureCacheDir(projectDir string) error {
	dir := filepath.Join(LocalDir(projectDir), CacheDirName)
	if pathExists(dir) {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("%s anlegen: %w", dir, err)
	}
	return writeIfMissing(filepath.Join(dir, PrivateIgnoreFile), managedIgnoreContent())
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
	case VersionSourcesFileName:
		return versionSourcesTemplate()
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

// versionSourcesTemplate ist die gültige, leere Quellenkonfiguration des
// Versionsinventars. Der Inhalt ist wortgleich die Vorlage aus
// docs/version-inventory.md, Abschnitt „Source Configuration → Template"; wer ihn
// ändert, ändert ihn dort und zieht hier nach.
func versionSourcesTemplate() string {
	return "# Versionsquellen für `k-playbook inventory`\n" + `#
# Diese Datei ist handgepflegt. k-playbook schreibt nur nach ausdrücklicher
# Bestätigung in sie, und dann ausschließlich ergänzend: bestehende Einträge,
# Kommentare und Reihenfolge bleiben erhalten.
#
` + "# Ohne Einträge erhebt `k-playbook inventory` die Standardquellen unterhalb der\n" + `# Projektwurzel — Manifeste, Lockfiles, Dockerfiles, Compose, DevContainer,
# Helm und CI. Hier stehen nur zusätzliche Quellen und die Wurzeln außerhalb
# des Projekts, die dafür gelesen werden dürfen.
#
# Vollständige Beschreibung: k-playbook/docs/version-inventory.md

schema_version: 1

# Zusätzlich lesbare Wurzeln, je ein absoluter Pfad. Die Projektwurzel ist
# immer erlaubt und gehört nicht hierher. Was nicht unter einer dieser Wurzeln
# liegt, wird abgelehnt — sichtbar gemeldet, nicht stillschweigend übergangen.
#
#   roots:
#     - /srv/deploy
roots: []

# Zusätzliche Quellen. Je Eintrag:
#   path: Datei oder Glob, relativ zur Projektwurzel oder absolut
#   kind: auto, python, go, node, rust, ruby, php, java, elixir, dockerfile,
#         compose, devcontainer, helm, ci, tool-versions
#   env:  lokal, dev, devcontainer, ci, deployment
#   note: optionaler Anzeigetext
#
#   sources:
#     - path: /srv/deploy/values-prod.yaml
#       kind: helm
#       env: deployment
#       note: Produktionswerte aus dem Deployment-Repo
sources: []

# Bereiche, in denen die Standarderkennung nicht suchen soll — je ein Muster
` + "# relativ zur Projektwurzel, `*` für ein Segment, `**` für beliebig viele.\n" + `# Gedacht für Testfixtures und Beispielprojekte: gepflegter Inhalt, dessen
# Versionen nichts über dieses Projekt aussagen.
#
# Gesperrt ist damit nichts. Jeder Ausschluss steht mit der Zahl der
` + "# übergangenen Quellen im Inventar, und eine Quelle daraus kommt wieder hinein,\n" +
		"# sobald sie unter `sources:` steht.\n" + `#
` + "# Die Installation `k-playbook/` ist immer ausgenommen und gehört nicht hierher:\n" + `# sie ist ein Clone des Werkzeugs und sagt nichts über dieses Projekt.
#
#   exclude:
#     - tests/fixtures/**
exclude: []

# Versionen in Helm-values, die keine Image-Referenz sind — etwa ein Subchart,
# das nur einen Tag entgegennimmt. Je Eintrag:
#   path: values-Datei oder Glob; sie muss ohnehin als Helm-values gelesen werden
#   key:  Punktpfad zum Wert, Listenindex in eckigen Klammern
#   item: Gegenstand als container/<name>
#
` + "# Der Abschnitt verlangt `schema_version: 2`; unter 1 wird er abgelehnt. Jeder\n" + `# Eintrag, der keine Zeile ergibt, steht als Hinweis mit Grund im Inventar.
#
#   helm_values:
#     - path: helm/values.yaml
#       key: redis.standalone.tag
#       item: container/redis
`
}
