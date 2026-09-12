package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/kascada/k-playbook/installer/internal/inventory"
	"github.com/kascada/k-playbook/installer/internal/yamllite"
)

// Die Zustände eines Dokuments, geschlossen. write nimmt die ersten drei an;
// superseded entsteht ausschließlich über Supersede — ein Dokument, das als
// abgelöst geboren würde, hätte nie gegolten.
const (
	KnowledgeStateRaw        = "raw"
	KnowledgeStateCondensed  = "condensed"
	KnowledgeStateReviewed   = "reviewed"
	KnowledgeStateSuperseded = "superseded"
)

// knowledgeWriteStates sind die Zustände, die write annimmt.
var knowledgeWriteStates = []string{KnowledgeStateRaw, KnowledgeStateCondensed, KnowledgeStateReviewed}

// Die Formate, geschlossen: was das Original war, bevor es Markdown wurde.
// Standard ist markdown. Die Ablage selbst hält immer Markdown; das Feld sagt
// nur, woraus es entstand — ein Bild oder PDF liegt weiter im Eingang, das
// Dokument hier ist der Stub, der darauf zeigt.
const KnowledgeFormatMarkdown = "markdown"

var knowledgeFormats = []string{KnowledgeFormatMarkdown, "text", "html", "image", "pdf"}

// KnowledgeDocument ist ein Dokument, wie write und publish es entgegennehmen:
// die Frontmatter-Felder einzeln und der Rumpf als Markdown ohne Kopf. Das
// Werkzeug setzt den Kopf selbst zusammen und setzt updated. Ein Aufrufer, der
// einen fertigen Kopf übergeben könnte, könnte einen falschen übergeben — und
// der Index wäre der Ort, an dem das auffiele.
//
// Path ist relativ zu knowledge/ und muss im Verzeichnis des Erzeugers liegen
// (siehe ResolveProducerPath). Die JSON-Namen sind die Argumentnamen der
// MCP-Werkzeuge und der --json-Antworten.
type KnowledgeDocument struct {
	Path string `json:"path"`
	// Title ist die eigene Formulierung des Dokuments.
	Title string `json:"title"`
	// Subject ist das Thema — die Achse, nach der Index und Suche sortieren.
	// Frei, solange nichts danach filtert.
	Subject string `json:"subject"`
	// Origin ist die tatsächliche Herkunft: System, Kennung dort, Adresse,
	// Zeitpunkt des Abrufs — nicht das Verzeichnis, das trägt der Pfad.
	Origin string `json:"origin"`
	// State ist raw, condensed oder reviewed; superseded nur über Supersede.
	State string `json:"state"`
	// Format ist markdown (Standard), text, html, image oder pdf.
	Format string `json:"format,omitempty"`
	// Sources sind die Eingangspfade, aus denen das Dokument destilliert
	// wurde — wo es welche gibt.
	Sources []string `json:"sources,omitempty"`
	// Body ist der Rumpf als Markdown ohne Frontmatter.
	Body string `json:"body"`
}

// knowledgeUpdatedLayout ist das Datum in updated: der Tag, nicht der
// Zeitpunkt. Ein Dokument ändert sich an einem Tag, nicht um 14:03:17.
const knowledgeUpdatedLayout = "2006-01-02"

// knowledgeUpdatedNow ist der Wert für updated. Als Variable, damit ein Test
// ihn festhalten kann.
var knowledgeUpdatedNow = func() string { return time.Now().Format(knowledgeUpdatedLayout) }

// validateKnowledgeDocument prüft die Felder eines Dokuments — alles außer dem
// Pfad, den ResolveProducerPath prüft. Format wird auf den Standard gesetzt,
// wenn es fehlt, und die Sources werden bereinigt; deshalb ein Zeiger.
func validateKnowledgeDocument(doc *KnowledgeDocument) error {
	if err := requireLine("title", doc.Title); err != nil {
		return err
	}
	if err := requireLine("subject", doc.Subject); err != nil {
		return err
	}
	if err := requireLine("origin", doc.Origin); err != nil {
		return err
	}
	doc.Title, doc.Subject, doc.Origin = strings.TrimSpace(doc.Title), strings.TrimSpace(doc.Subject), strings.TrimSpace(doc.Origin)

	doc.State = strings.TrimSpace(doc.State)
	switch {
	case doc.State == "":
		return InputErrorf("kein state angegeben — erwartet wird %s", strings.Join(knowledgeWriteStates, ", "))
	case doc.State == KnowledgeStateSuperseded:
		return InputErrorf("state %q wird nicht geschrieben: abgelöst wird ein Dokument nur über supersede", doc.State)
	case !oneOf(knowledgeWriteStates, doc.State):
		return InputErrorf("unbekannter state %q — erwartet wird %s", doc.State, strings.Join(knowledgeWriteStates, ", "))
	}

	doc.Format = strings.TrimSpace(doc.Format)
	if doc.Format == "" {
		doc.Format = KnowledgeFormatMarkdown
	}
	if !oneOf(knowledgeFormats, doc.Format) {
		return InputErrorf("unbekanntes format %q — erwartet wird %s", doc.Format, strings.Join(knowledgeFormats, ", "))
	}

	sources := make([]string, 0, len(doc.Sources))
	for _, source := range doc.Sources {
		source = strings.TrimSpace(source)
		if source == "" {
			continue
		}
		if strings.ContainsAny(source, "\r\n") {
			return InputErrorf("sources: ein Eintrag umfasst mehrere Zeilen: %q", source)
		}
		sources = append(sources, source)
	}
	doc.Sources = sources

	if strings.TrimSpace(doc.Body) == "" {
		return InputErrorf("leerer body: der Rumpf muss Markdown enthalten")
	}
	if bodyCarriesFrontmatter(doc.Body) {
		return InputErrorf("der body trägt einen Dateikopf (---…---): das Frontmatter baut das Werkzeug aus den Feldern, ein übergebener Kopf wird nicht durchgereicht")
	}
	return nil
}

// requireLine verlangt einen nicht leeren, einzeiligen Wert.
func requireLine(field string, value string) error {
	if strings.TrimSpace(value) == "" {
		return InputErrorf("kein %s angegeben", field)
	}
	if strings.ContainsAny(value, "\r\n") {
		return InputErrorf("%s umfasst mehrere Zeilen: %q", field, value)
	}
	return nil
}

func oneOf(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

// bodyCarriesFrontmatter erkennt einen Dateikopf am Anfang des Rumpfs: ein
// führender ---Block, der als YAML-Abbildung durchgeht. Ein führendes „---"
// allein ist kein Kopf, sondern ein Thematic Break — gültiges Markdown, und
// der Text dahinter bleibt Rumpf (belegt in material/befunde/wissenstor-mcp.md).
func bodyCarriesFrontmatter(body string) bool {
	block, ok := inventory.FrontmatterBlock([]byte(strings.TrimLeft(body, " \t")))
	return ok && isKnowledgeFrontmatter(block)
}

// isKnowledgeFrontmatter prüft, ob ein Block eine YAML-Abbildung ist —
// derselbe Leser, aus dem der Index die Felder holt. Ein leerer Block zählt
// mit. Eine Liste oder Fließtext zählt nicht.
//
// Die Prüfung ist absichtlich die schwächere Seite: ein Markdown-Absatz mit
// Doppelpunkt geht als Abbildung durch und würde für einen Kopf gehalten. Der
// teure Fall ist der umgekehrte — ein durchgereichter Kopf, der den erzeugten
// verdrängt —, und den fängt sie.
func isKnowledgeFrontmatter(block string) bool {
	root, err := yamllite.Parse([]byte(block))
	return err == nil && root != nil && root.Kind == yamllite.Mapping
}

// composeKnowledgeFile baut die Datei: Frontmatter in fester Reihenfolge, dann
// der Rumpf. Zeilenenden werden LF, die Datei endet mit genau einem
// Zeilenumbruch. extra sind Felder, die nur supersede setzt; sie stehen hinter
// den Pflichtfeldern und vor updated.
func composeKnowledgeFile(doc KnowledgeDocument, extra [][2]string, updated string) string {
	var head strings.Builder
	head.WriteString("---\n")
	head.WriteString("title: " + yamlScalar(doc.Title) + "\n")
	head.WriteString("subject: " + yamlScalar(doc.Subject) + "\n")
	head.WriteString("origin: " + yamlScalar(doc.Origin) + "\n")
	head.WriteString("state: " + doc.State + "\n")
	head.WriteString("format: " + doc.Format + "\n")
	if len(doc.Sources) > 0 {
		head.WriteString("sources:\n")
		for _, source := range doc.Sources {
			head.WriteString("  - " + yamlScalar(source) + "\n")
		}
	}
	for _, field := range extra {
		head.WriteString(field[0] + ": " + yamlScalar(field[1]) + "\n")
	}
	head.WriteString("updated: " + updated + "\n")
	head.WriteString("---\n\n")

	body := strings.ReplaceAll(doc.Body, "\r\n", "\n")
	body = strings.TrimRight(strings.TrimLeft(body, "\n"), "\n") + "\n"
	return head.String() + body
}

// yamlScalar schreibt einen Wert so, dass jeder YAML-Leser ihn als genau
// diese Zeichenkette liest. Ein schlichter Wert bleibt schlicht; was YAML
// anders deuten könnte — führende Sonderzeichen, „: " oder „ #" im Text,
// Leerraum an den Enden, Werte, die wie true, null oder eine Zahl aussehen —
// wird in doppelte Anführungszeichen gesetzt. yamllite liest solche Werte
// über strconv.Unquote zurück, deshalb strconv.Quote zum Schreiben.
func yamlScalar(value string) string {
	if yamlNeedsQuotes(value) {
		return strconv.Quote(value)
	}
	return value
}

func yamlNeedsQuotes(value string) bool {
	if value == "" || value != strings.TrimSpace(value) {
		return true
	}
	if strings.ContainsAny(value[:1], "-?:,[]{}#&*!|>'\"%@`") {
		return true
	}
	if strings.Contains(value, ": ") || strings.Contains(value, " #") || strings.HasSuffix(value, ":") || strings.ContainsAny(value, "\t\r\n") {
		return true
	}
	switch strings.ToLower(value) {
	case "true", "false", "yes", "no", "on", "off", "null", "~":
		return true
	}
	if _, err := strconv.ParseFloat(value, 64); err == nil {
		return true
	}
	return false
}

// Write legt ein Dokument in der Wissensablage an oder ersetzt es. Der
// Erzeuger nennt sich, der Pfad muss in seinem Verzeichnis liegen, und die
// drei Generatoren werden abgewiesen: die schreiben ihr Verzeichnis nur als
// Ganzes über Publish, nie eine Einzeldatei — sonst hinge dort etwas, das der
// nächste Lauf spurlos entfernt.
//
// queue nennt optional den Eintrag der Warteschlange, den dieses Dokument
// erledigt. Er wird gelöscht, nachdem das Dokument steht und der Index es
// kennt — und nur dann. Schlägt das Schreiben fehl, liegt der Eintrag noch da.
// Ein genannter Eintrag, den es nicht gibt, ist ein Fehler vor dem Schreiben:
// sonst stünde ein Dokument, dessen Erledigung nirgends verbucht ist.
//
// Das Ergebnis ist der geschriebene Ort relativ zu knowledge/ — so, wie Read,
// List und Search ihn nennen.
//
// Erst der Index, dann die Datei: der Zugriff gleicht zuerst den Baum ab, so
// dass die eigene Schreibung danach nicht als Drift zählt — das Tor selbst
// hat nicht am Tor vorbei geschrieben.
func (k *Knowledge) Write(producer string, doc KnowledgeDocument, queue string) (string, error) {
	parsed, rel, err := ResolveProducerPath(producer, doc.Path)
	if err != nil {
		return "", err
	}
	if parsed.IsGenerator() {
		return "", InputErrorf("Erzeuger %s schreibt keine Einzeldatei: ein Generator veröffentlicht sein Verzeichnis als Ganzes über publish", parsed)
	}
	if err := validateKnowledgeDocument(&doc); err != nil {
		return "", err
	}
	doc.Path = rel

	queueFile := ""
	if strings.TrimSpace(queue) != "" {
		queueFile, err = queueEntryFile(k.projectDir, queue)
		if err != nil {
			return "", err
		}
		if !fileExists(queueFile) {
			return "", InputErrorf("Queue-Eintrag %q gibt es nicht", strings.TrimSpace(queue))
		}
	}

	index, err := k.open()
	if err != nil {
		return "", err
	}
	if err := k.writeDocument(index, doc); err != nil {
		return "", err
	}
	index.BuiltAt = knowledgeNow()
	if err := writeKnowledgeIndex(k.projectDir, index); err != nil {
		return "", err
	}

	if queueFile != "" {
		if err := os.Remove(queueFile); err != nil {
			return "", fmt.Errorf("Dokument %s steht, aber der Queue-Eintrag ließ sich nicht löschen: %w", rel, err)
		}
	}
	return rel, nil
}

// writeDocument schreibt eine geprüfte Datei an ihren Ort und tauscht ihre
// Chunks im Index — ohne den Index zu speichern; das übernimmt der Aufrufer,
// nachdem alles geschrieben ist.
func (k *Knowledge) writeDocument(index *knowledgeIndex, doc KnowledgeDocument) error {
	root := KnowledgeDir(k.projectDir)
	full := filepath.Join(root, filepath.FromSlash(doc.Path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return fmt.Errorf("%s anlegen: %w", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(composeKnowledgeFile(doc, nil, knowledgeUpdatedNow())), 0o644); err != nil {
		return fmt.Errorf("%s schreiben: %w", doc.Path, err)
	}
	entry, chunks, err := chunkKnowledgeFile(root, doc.Path)
	if err != nil {
		return err
	}
	index.replaceFile(doc.Path, entry, chunks)
	return nil
}
