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
	// results/ und cache/ tragen das.
	//
	// Begründung: Bei priv/ und material/ geht es um Geschmack, dort ist die
	// Zurückhaltung richtig. Ein Werkzeug, das gefundene Secrets im Klartext
	// ins Repository des Nutzers schreibt, ist dagegen ein Fehler von
	// k-playbook und keine Projektentscheidung — und die Rohausgaben sind nur
	// der schärfste Fall: ein Review ist aus dem Code wiederholbar, sein
	// Ergebnis ist ein Stand von einem Rechner. Bei cache/ ist der Grund ein
	// anderer, führt aber zum selben Ergebnis: der Inhalt ist aus dem Projekt
	// abgeleitet und entsteht jederzeit neu. Ableitbares gehört nicht ins
	// Repository — es veraltet dort, ohne dass jemand es merkt, und bläht die
	// Historie mit Ständen auf, die niemand liest.
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
		{
			Path: "data",
			Purpose: "Maschinendateien, die k-playbook selbst besitzt und die zum Projektstand gehören.\n" +
				"Sie werden ganz normal mitversioniert; dieses Verzeichnis bekommt keine .gitignore.\n\n" +
				"Erste Datei ist todos.json mit den Todos des Projekts. Geschrieben wird sie über\n" +
				"/k-todo, über das Subkommando `k-playbook todo` oder über die Oberfläche — nie von\n" +
				"Hand. Einzige Ausnahme ist die Auflösung eines Merge-Konflikts: treffen zwei Branches\n" +
				"aufeinander, die je ein Todo angelegt haben, kollidieren sie an nextId und am Ende des\n" +
				"Arrays. Die Auflösung ist mechanisch — beide Einträge behalten, nextId auf max(id)+1\n" +
				"setzen.",
		},
		{
			Path:             "cache",
			Private:          true,
			PrivateByDefault: true,
			Purpose: "Was diese Maschine aus dem Projekt ableitet und jederzeit neu bauen kann.\n\n" +
				"Alles hier ist wiederherstellbar. Wer etwas Unwiederbringliches ablegen will, braucht\n" +
				"ein anderes Verzeichnis — dieses darf ohne Rückfrage gelöscht werden, und sein Inhalt\n" +
				"bleibt aus der Versionskontrolle.\n\n" +
				"k-playbook legt dafür beim erstmaligen Anlegen dieses Verzeichnisses eine .gitignore mit\n" +
				"diesem Inhalt an:\n\n" +
				"    *\n    !.gitignore\n    !README.md\n\n" +
				"Der Block „Lokale Einstellungen\" in der Oberfläche zeigt den gemessenen Ist-Zustand und\n" +
				"schaltet ihn um — auch wieder zurück; einmal umgeschaltet, bleibt es dabei.\n\n" +
				"knowledge/index.json ist der Suchindex des Wissenstors über ../knowledge/ (`k-playbook\n" +
				"knowledge`, MCP-Werkzeuge k_playbook_knowledge_*). Fehlt er, baut ihn der nächste\n" +
				"Zugriff neu; Änderungen am Tor vorbei erkennt er über Datei-Hashes selbst.",
		},
		{Path: "docs", Purpose: "Projektwissen für AI-Sessions, nach Herkunft getrennt: code/ von /k-docs-code, libs/ von /k-docs-tools, extracted/ von /k-docs-extract, versions/ von /k-doc-inventory, manual/ von Hand. Die vier erzeugten Verzeichnisse legt jeweils ihr Erzeuger beim ersten Lauf an. Die README dieses Verzeichnisses ist der einzige Index; /k-docs-index schreibt sie neu über die Ordner und die flachen Wurzeldateien. Das Wissenstor (`k-playbook knowledge`, MCP-Werkzeuge k_playbook_knowledge_*) liest und schreibt nicht mehr hier, sondern in der Wissensablage ../knowledge/; dieses Verzeichnis bleibt bestehen und trägt weiter, bis die Migration seine Dokumente dorthin überführt."},
		{Path: filepath.Join("docs", "manual"), Purpose: "Von Hand gepflegte Dokumentation. Kein Command schreibt hier Doc-Dateien hinein; gelistet wird sie über den Index in ../README.md."},
		{
			Path:    InboxDirName,
			Private: true,
			Purpose: "Der Eingang: was ankommt, bevor jemand es gelesen hat. Chat-Mitschnitte, Notizen,\n" +
				"PDFs, Screenshots, HTML-Abzüge, Exporte — alles, was eine Person oder ein Connector\n" +
				"ablegt, in dem Format, in dem es kommt. Unterteilt allein nach Quelle: inbox/<quelle>/…,\n" +
				"die Quelle ist frei (confluence, chat, mail, scan).\n\n" +
				"Es gibt keine Namensregel, kein Frontmatter und keine Pflicht über das Ablegen hinaus.\n" +
				"Das ist die Bedingung dafür, dass überhaupt etwas abgelegt wird: ein Eingang, der\n" +
				"Vorbereitung verlangt, wird einmal benutzt. Abgelegt wird über `k-playbook knowledge\n" +
				"inbox put`, das MCP-Werkzeug k_playbook_knowledge_inbox_put oder schlicht mit dem\n" +
				"Dateimanager.\n\n" +
				"Der Eingang ist ein Archiv, keine Warteschlange. Etwas kann monatelang hier liegen,\n" +
				"ohne dass jemand hineinsieht, und nichts ist offen, nur weil es hier liegt. Er wird\n" +
				"nie indiziert und taucht in keiner Suche auf. Eine Verarbeitung verbraucht ihn nicht:\n" +
				"ein Rohstück bleibt, nachdem daraus Wissen gemacht wurde — nur so lässt sich ein\n" +
				"schlechter Extrakt aus derselben Quelle noch einmal ziehen. Gelöscht wird hier\n" +
				"ausschließlich von Hand, nie als Nebenwirkung eines Laufs. Was kein Markdown ist,\n" +
				"bleibt für immer hier; das Wissensdokument unter ../knowledge/ zeigt darauf.\n\n" +
				"Der Inhalt wird ganz normal mitversioniert. Rohmaterial enthält typischerweise\n" +
				"Tokens, Pfade und Namen; soll es nicht ins Repository, schaltet der Block\n" +
				"„Lokale Einstellungen\" in der Oberfläche dieses Verzeichnis um — er legt die\n" +
				".gitignore an und nimmt bereits versionierte Dateien aus dem Index.\n\n" +
				"Von Hand geht es genauso: eine .gitignore in diesem Verzeichnis mit diesem\n" +
				"Inhalt:\n\n    *\n    !.gitignore\n    !README.md\n\n" +
				"Was bereits committet ist, nimmt erst ein `git rm --cached` wieder heraus. Und\n" +
				"was schon gepusht wurde, bleibt in der Historie.",
		},
		{
			Path: QueueDirName,
			Purpose: "Die Warteschlange: was noch aussteht. Ein Eintrag ist ein Stück Arbeit — dieses\n" +
				"Rohstück soll Wissen werden. Er ist eine kleine Markdown-Datei, die auf ihre Quelle\n" +
				"im Eingang verweist statt sie zu kopieren, und das Zielverzeichnis unter ../knowledge/\n" +
				"und den Grund nennt. Angelegt wird er über `k-playbook knowledge queue add` oder das\n" +
				"MCP-Werkzeug k_playbook_knowledge_queue_add, das die Kennung selbst vergibt.\n\n" +
				"Nach der Übernahme in die Ablage wird der Eintrag gelöscht — nicht verschoben, nicht\n" +
				"archiviert, nicht abgehakt. Das erledigt `k-playbook knowledge write` selbst, wenn\n" +
				"ihm der Eintrag genannt wird: gelöscht wird, nachdem das Dokument steht, und nur dann.\n" +
				"Eine leere Warteschlange heißt deshalb: nichts offen. Das ist der ganze Zweck des\n" +
				"Verzeichnisses. Ein Rückstand, der sich nur durch Zählen oder Filtern feststellen\n" +
				"lässt, ist einer, den niemand liest.\n\n" +
				"Ein Lauf, der scheitert, lässt seinen Eintrag liegen. Das Verzeichnis hält Arbeit,\n" +
				"nicht Geschichte; was aus einer Quelle geworden ist, steht im origin des fertigen\n" +
				"Dokuments. Wer einen Eintrag ohne Übernahme loswerden will, nimmt `k-playbook\n" +
				"knowledge queue drop`. Indiziert wird hier nichts.",
		},
		{
			Path: KnowledgeDirName,
			Purpose: "Die Wissensablage: was gilt. Nur was hier liegt, wird indiziert, durchsucht, gelesen\n" +
				"und in einer Antwort zitiert. Immer Markdown mit YAML-Frontmatter; was in einem anderen\n" +
				"Format ankommt, wird am Eingang umgewandelt oder bleibt unter ../inbox/ liegen, mit\n" +
				"einem Markdown-Stub hier, der es beschreibt. Ein Dokument je Thema, fortgeschrieben —\n" +
				"nicht eines je Ereignis.\n\n" +
				"Der Pfad trägt den Eigentümer, das Frontmatter das Thema: code/ gehört /k-docs-code,\n" +
				"libs/ gehört /k-docs-tools, versions/ gehört /k-doc-inventory, extracted/ gehört\n" +
				"/k-docs-extract, external/<system>/ je einem Connector, findings/ der Sitzung,\n" +
				"pitfalls/ und manual/ einer Person, README.md dem Index-Command /k-docs-index. Genau ein\n" +
				"Eigentümer je Verzeichnis, und nur der schreibt dort: die drei Generatoren code/, libs/\n" +
				"und versions/ schreiben ihr Verzeichnis bei jedem Lauf als Ganzes neu, und was ein\n" +
				"anderer dort abgelegt hätte, wäre danach spurlos weg.\n\n" +
				"Geschrieben wird ausschließlich über das Werkzeug — `k-playbook knowledge write`,\n" +
				"`publish` und `supersede` oder die MCP-Werkzeuge k_playbook_knowledge_*. Jede\n" +
				"Schreibung nennt ihren Erzeuger, und ein Ziel außerhalb seines Verzeichnisses wird\n" +
				"abgewiesen. Das Frontmatter baut das Werkzeug aus den übergebenen Feldern; niemand\n" +
				"reicht einen fertigen Dateikopf durch. Gelöscht wird nichts: Überholtes bekommt\n" +
				"`state: superseded`, fällt aus der Suche und bleibt lesbar.\n\n" +
				"Der Suchindex darüber liegt unter ../cache/knowledge/ und ist jederzeit verwerfbar;\n" +
				"Änderungen am Werkzeug vorbei erkennt er selbst über Datei-Hashes.",
		},
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
			Purpose: "Rohmaterial als Quelle für Docs: Chat-Mitschnitte, Notizen, Zulieferungen.\nEs wird nie indiziert; gelesen wird es von /k-docs-extract, geschrieben nach\ndocs/extracted/.\n\nGeschrieben wird hier nur an einer einzigen Stelle: nach befunde/. Dort halten\nder Skill ks-befunde und der Command /k-danke fest, was eine Analyse oder\nFehlersuche ergeben hat — Belegtes, Widerlegtes und ausgeschlossene Sackgassen.\nDas Verzeichnis legt sein Erzeuger beim ersten Lauf an; die Form steht in der\nRegel rules/befunde.md. Alles andere in diesem Verzeichnis bleibt unangetastet.\n\nDer Inhalt wird ganz normal mitversioniert. Rohmaterial enthält typischerweise\nTokens, Pfade und Namen; soll es nicht ins Repository, schaltet der Block\n„Lokale Einstellungen\" in der Oberfläche dieses Verzeichnis um — er legt die\n.gitignore an und nimmt bereits versionierte Dateien aus dem Index.\n\nVon Hand geht es genauso: eine .gitignore in diesem Verzeichnis mit diesem\nInhalt:\n\n    *\n    !.gitignore\n    !README.md\n\nWas bereits committet ist, nimmt erst ein `git rm --cached` wieder heraus. Und\nwas schon gepusht wurde, bleibt in der Historie.",
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
				"ersten Lauf des Sammlers —, damit ihr Zustand in `k-playbook context` von Anfang an\n" +
				"eine Antwort hat. Vollständige Beschreibung: " + PlaybookDirName + "/docs/version-inventory.md.",
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
		if entry.Path == KnowledgeDirName {
			if err := createKnowledgeReadme(projectDir, readme, entry); err != nil {
				return CheckLocal(projectDir), err
			}
		} else if err := writeIfMissing(readme, readmeTemplate(entry)); err != nil {
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
// Das Wissenstor legt sein cache/knowledge/ beim ersten Zugriff selbst an und
// bringt cache/ damit nebenbei mit. Ohne diesen Weg entstünde es dann ohne
// .gitignore, und CreateLocal zöge sie nie nach: der schreibt sie nur, wenn
// das Verzeichnis in genau seinem Lauf entsteht. Der abgeleitete Index wäre
// damit committierbar — ein Release mit `git add -A` nähme den Chunk-Abzug
// der ganzen Doku mit.
//
// Nachträglich geschrieben wird nichts: ist cache/ schon da, bleibt es, wie
// es ist. Ein bewusstes Umschalten auf öffentlich (makePublic) darf dieser
// Weg so wenig zurückdrehen wie CreateLocal.
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

// createKnowledgeReadme legt die README der Wissensablage an, wenn sie fehlt —
// durch das Tor, als Erzeuger docs-index, dem sie gehört. So trägt sie
// Frontmatter wie jedes Dokument der Ablage, und der Index kennt sie: ein
// schon gebauter Index meldet danach keine Drift. state ist condensed, denn
// der Text ist maschinell erzeugte Navigation, von niemandem geprüft; die
// README ist ohnehin nie ein Suchtreffer. Eine vorhandene README bleibt, wie
// sie ist — auch eine ohne Kopf, bis /k-docs-index sie neu schreibt.
func createKnowledgeReadme(projectDir string, readme string, entry LocalEntry) error {
	if pathExists(readme) {
		return nil
	}
	_, err := NewKnowledge(projectDir).Write(string(ProducerDocsIndex), KnowledgeDocument{
		Path:    knowledgeReadmeName,
		Title:   "Wissensablage",
		Subject: "Index der Wissensablage",
		Origin:  "k-playbook, Einrichtung der Struktur",
		State:   KnowledgeStateCondensed,
		Body:    readmeTemplate(entry),
	}, "")
	if err != nil {
		// Write schreibt erst die Datei, dann den Index. Steht die README
		// danach auf der Platte, ist nur der Index gescheitert: der nächste
		// Zugriff nimmt sie über die Drift-Erkennung auf, und CreateLocal legt
		// die übrigen Einträge noch an (Task 064, Entscheidung 5). Einen
		// Hinweiskanal hat CreateLocal nicht. Fehlt die Datei, bleibt es beim
		// Fehler.
		if fileExists(readme) {
			return nil
		}
		return fmt.Errorf("%s anlegen: %w", filepath.Join(entry.Path, knowledgeReadmeName), err)
	}
	return nil
}

func readmeTemplate(entry LocalEntry) string {
	if entry.Path == "docs" {
		return docsReadmeTemplate()
	}
	return fmt.Sprintf("# %s\n\n%s\n\nDieses Verzeichnis gehört dem Projekt und wird von einem Update nie angefasst.\n",
		entry.Path, entry.Purpose)
}

func docsReadmeTemplate() string {
	return `# Projektdokumentation

Dieser Index ist der Einstieg für Projektwissen in AI-Sessions. Er enthält noch
keine fachlichen Dokumente.

Wenn Dokumentation entsteht, ergänzt /k-docs-index diese Datei um Übersicht,
Stichwort-Index und direkte Fragen. Bis dahin gilt: Erst diesen Index lesen,
dann nur bei Bedarf den Code untersuchen.

Die AI-Session-Regel steht in ../../AGENTS.md.
`
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
