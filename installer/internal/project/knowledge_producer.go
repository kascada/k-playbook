package project

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
)

// Producer ist der Erzeuger einer Schreibung in die Wissensablage — ein Name
// aus einer geschlossenen Liste, den jeder Aufruf von write und publish
// mitbringt. Der Pfad trägt den Eigentümer: jedes Verzeichnis unter knowledge/
// gehört genau einem Erzeuger, und nur der schreibt dort. Das Werkzeug prüft
// den Namen gegen das Ziel und weist ab, was nicht zusammenpasst.
//
// Der Erzeuger ist eine Erklärung, kein Beweis: wer lügt, wird nicht
// erwischt. Was die Prüfung bringt, ist, dass ein falsches Ziel am Aufruf ein
// Fehler ist statt einen Monat später ein Rätsel — und dass die Zuordnung an
// genau einer Stelle steht, nicht in acht Aufrufern. Die Tabelle ist die aus
// docs/knowledge-layout.md, Abschnitt „Who may write where".
type Producer string

const (
	// ProducerDocsCode ist /k-docs-code; schreibt knowledge/code/ als Ganzes.
	ProducerDocsCode Producer = "docs-code"
	// ProducerDocsTools ist /k-docs-tools; schreibt knowledge/libs/ als Ganzes.
	ProducerDocsTools Producer = "docs-tools"
	// ProducerInventory ist /k-doc-inventory; schreibt knowledge/versions/ als
	// Ganzes.
	ProducerInventory Producer = "inventory"
	// ProducerDocsExtract ist /k-docs-extract; schreibt einzelne Dokumente
	// nach knowledge/extracted/.
	ProducerDocsExtract Producer = "docs-extract"
	// ProducerSession ist die Sitzung, die durch das Tor schreibt; ihr gehört
	// knowledge/findings/.
	ProducerSession Producer = "session"
	// ProducerPerson ist ein Mensch; ihm gehören knowledge/manual/ und
	// knowledge/pitfalls/.
	ProducerPerson Producer = "person"
	// ProducerDocsIndex ist /k-docs-index; ihm gehört genau eine Datei, die
	// knowledge/README.md — die Wurzel der Zone, sonst nichts.
	ProducerDocsIndex Producer = "docs-index"
)

// ProducerConnectorPrefix leitet den achten Erzeuger ein: connector:<system>
// schreibt nach knowledge/external/<system>/. Das System ist frei, muss aber
// ein einzelnes Pfadsegment sein — es wird zum Verzeichnisnamen.
const ProducerConnectorPrefix = "connector:"

// knowledgeExternalDirName ist der Ordner der Connector-Extrakte, je System
// ein Unterverzeichnis.
const knowledgeExternalDirName = "external"

// knowledgeReadmeName ist die Indexdatei in der Wurzel der Zone.
const knowledgeReadmeName = "README.md"

// producerDirs ist die Zuordnung der festen Erzeuger auf ihre Verzeichnisse,
// je mit Schrägstrich am Ende. docs-index steht nicht darin: er besitzt eine
// Datei, kein Verzeichnis (siehe owns).
var producerDirs = map[Producer][]string{
	ProducerDocsCode:    {"code/"},
	ProducerDocsTools:   {"libs/"},
	ProducerInventory:   {"versions/"},
	ProducerDocsExtract: {"extracted/"},
	ProducerSession:     {"findings/"},
	ProducerPerson:      {"manual/", "pitfalls/"},
}

// knownProducers nennt die Liste für Fehlermeldungen, in der Reihenfolge der
// Tabelle im Layout.
var knownProducers = []Producer{
	ProducerDocsCode, ProducerDocsTools, ProducerInventory, ProducerDocsExtract,
	Producer(ProducerConnectorPrefix + "<system>"), ProducerSession, ProducerPerson, ProducerDocsIndex,
}

// ParseProducer prüft einen Erzeugernamen gegen die geschlossene Liste. Ein
// Connector wird mitsamt seinem System angenommen, wenn das System ein
// einzelnes, sauberes Pfadsegment ist.
func ParseProducer(name string) (Producer, error) {
	producer := Producer(strings.TrimSpace(name))
	if producer == "" {
		return "", InputErrorf("kein Erzeuger angegeben — erwartet wird einer aus %s", knownProducerList())
	}
	if _, ok := producerDirs[producer]; ok || producer == ProducerDocsIndex {
		return producer, nil
	}
	if system, ok := strings.CutPrefix(string(producer), ProducerConnectorPrefix); ok {
		if !isCleanPathSegment(system) {
			return "", InputErrorf("Erzeuger %q: das System hinter %q muss ein einzelner Verzeichnisname sein",
				producer, ProducerConnectorPrefix)
		}
		return producer, nil
	}
	return "", InputErrorf("unbekannter Erzeuger %q — bekannt sind %s", producer, knownProducerList())
}

func knownProducerList() string {
	names := make([]string, 0, len(knownProducers))
	for _, producer := range knownProducers {
		names = append(names, string(producer))
	}
	return strings.Join(names, ", ")
}

// IsGenerator meldet, ob der Erzeuger sein Verzeichnis bei jedem Lauf als
// Ganzes neu schreibt. Nur diese drei dürfen publish, und nur sie dürfen
// write nicht: ein Generator schreibt nie eine Einzeldatei, sonst hinge im
// Verzeichnis etwas, das der nächste Lauf spurlos entfernt.
func (p Producer) IsGenerator() bool {
	switch p {
	case ProducerDocsCode, ProducerDocsTools, ProducerInventory:
		return true
	}
	return false
}

// Dirs nennt die Verzeichnisse, die dem Erzeuger gehören, relativ zu
// knowledge/ und mit Schrägstrich am Ende; für docs-index leer, weil ihm eine
// Datei gehört und kein Verzeichnis.
func (p Producer) Dirs() []string {
	if dirs, ok := producerDirs[p]; ok {
		return append([]string(nil), dirs...)
	}
	if system, ok := strings.CutPrefix(string(p), ProducerConnectorPrefix); ok {
		return []string{knowledgeExternalDirName + "/" + system + "/"}
	}
	return nil
}

// owns prüft, ob ein bereinigter Pfad relativ zu knowledge/ dem Erzeuger
// gehört. Für docs-index ist das genau die README in der Wurzel.
func (p Producer) owns(rel string) bool {
	if p == ProducerDocsIndex {
		return rel == knowledgeReadmeName
	}
	for _, dir := range p.Dirs() {
		if strings.HasPrefix(rel, dir) && len(rel) > len(dir) {
			return true
		}
	}
	return false
}

// describeOwnership nennt in einer Fehlermeldung, wohin der Erzeuger darf.
func (p Producer) describeOwnership() string {
	if p == ProducerDocsIndex {
		return "genau " + knowledgeReadmeName + " in der Wurzel"
	}
	dirs := p.Dirs()
	return "unterhalb von " + strings.Join(dirs, " oder ")
}

// KnowledgeRelPath bereinigt einen Pfad relativ zu knowledge/ und weist ab,
// was aus der Zone herausführt: leer, absolut, mit „..", oder keine
// Markdown-Datei. Das Ergebnis ist der Pfad mit Schrägstrichen, so wie read,
// list und search ihn nennen.
//
// Das ist der erste der drei Fehler der Pfadprüfung — der Zone-Ausbruch. Er
// hängt an keinem Erzeuger: ein Pfad, der die Zone verlässt, ist für jeden
// falsch.
func KnowledgeRelPath(rel string) (string, error) {
	trimmed := strings.TrimSpace(rel)
	if trimmed == "" {
		return "", InputErrorf("kein Pfad angegeben — erwartet wird ein Pfad relativ zu %s/%s/", LocalDirName, KnowledgeDirName)
	}
	if filepath.IsAbs(trimmed) || strings.HasPrefix(trimmed, "/") || strings.HasPrefix(trimmed, "\\") {
		return "", InputErrorf("Pfad %q führt aus %s/%s/ heraus: nur relative Pfade", rel, LocalDirName, KnowledgeDirName)
	}
	cleaned := path.Clean(filepath.ToSlash(trimmed))
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", InputErrorf("Pfad %q führt aus %s/%s/ heraus", rel, LocalDirName, KnowledgeDirName)
	}
	for _, segment := range strings.Split(cleaned, "/") {
		if strings.HasPrefix(segment, ".") {
			// Versteckte Einträge überspringt der Index; eine Datei, die er
			// nie sähe, soll auch niemand hineinschreiben.
			return "", InputErrorf("Pfad %q: versteckte Einträge (%q) gehören nicht in die Wissensablage", rel, segment)
		}
	}
	if !strings.EqualFold(path.Ext(cleaned), ".md") {
		return "", InputErrorf("Pfad %q: die Wissensablage hält nur Markdown-Dateien (*.md)", rel)
	}
	return cleaned, nil
}

// ResolveProducerPath prüft Erzeuger und Ziel zusammen und liefert den
// bereinigten Pfad relativ zu knowledge/. Drei Fehler, drei Meldungen:
//
//   - unbekannter Erzeuger (ParseProducer),
//   - Pfad, der aus der Zone herausführt (KnowledgeRelPath),
//   - Ziel innerhalb der Zone, aber außerhalb des Erzeugerverzeichnisses.
//
// Die Reihenfolge ist Absicht: erst der Erzeuger, dann die Zone, dann die
// Zuordnung. So nennt die Meldung immer den Fehler, der zuerst behoben werden
// muss, und ein Aufrufer mit falschem Erzeuger erfährt nicht zuerst, dass
// sein Pfad für diesen falschen Erzeuger unpassend wäre.
func ResolveProducerPath(producer string, rel string) (Producer, string, error) {
	parsed, err := ParseProducer(producer)
	if err != nil {
		return "", "", err
	}
	cleaned, err := KnowledgeRelPath(rel)
	if err != nil {
		return "", "", err
	}
	if !parsed.owns(cleaned) {
		return "", "", InputErrorf("Ziel %q außerhalb des Erzeugerverzeichnisses: %s schreibt %s",
			cleaned, parsed, parsed.describeOwnership())
	}
	return parsed, cleaned, nil
}

// isCleanPathSegment prüft einen einzelnen Verzeichnis- oder Dateinamen:
// nicht leer, kein Trenner, nicht versteckt, kein Leerraum an den Enden.
func isCleanPathSegment(segment string) bool {
	if segment == "" || segment != strings.TrimSpace(segment) {
		return false
	}
	if strings.ContainsAny(segment, `/\`) || strings.HasPrefix(segment, ".") {
		return false
	}
	return segment != "." && segment != ".."
}

// ProducerTable ist die Erzeugertabelle als Zeilen für die Kurzhilfe des
// Subkommandos — Name und Verzeichnis je Zeile, in der Reihenfolge des
// Layouts, damit Hilfe und Prüfung dieselbe Quelle haben.
func ProducerTable() []string {
	rows := []string{}
	for _, producer := range knownProducers {
		target := ""
		switch {
		case producer == ProducerDocsIndex:
			target = knowledgeReadmeName
		case strings.HasPrefix(string(producer), ProducerConnectorPrefix):
			target = knowledgeExternalDirName + "/<system>/"
		default:
			target = strings.Join(producer.Dirs(), ", ")
		}
		suffix := ""
		if producer.IsGenerator() {
			suffix = " (Generator: nur publish)"
		}
		rows = append(rows, fmt.Sprintf("%-20s %s%s", producer, target, suffix))
	}
	return rows
}
