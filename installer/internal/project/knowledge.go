package project

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	"github.com/kascada/k-playbook/installer/internal/inventory"
	"github.com/kascada/k-playbook/installer/internal/markdown"
	"github.com/kascada/k-playbook/installer/internal/yamllite"
)

// KnowledgeDirName ist das Wissensverzeichnis unterhalb von k-playbook-local:
// das Projektwissen für AI-Sessions, nach Herkunft getrennt (siehe den
// Eintrag docs in LocalStructure). Es ist dasselbe Verzeichnis, das
// /k-docs-index indiziert — ein zweites wird nicht angelegt.
const KnowledgeDirName = "docs"

// KnowledgeLearnedDirName ist der einzige Ordner, in den das Wissenstor
// schreibt. Die Herkunftsordner code/, libs/, extracted/, versions/ und
// manual/ gehören ihren Generatoren, die ganze Dateien neu schreiben; was
// dorthin geschrieben würde, wäre nach dem nächsten Lauf still weg.
const KnowledgeLearnedDirName = "learned"

// KnowledgeRootSource ist die Herkunft einer Datei, die flach in der Wurzel
// des Wissensverzeichnisses liegt — heute README.md, der Index selbst.
// /k-docs-index kennt sie als „unsorted flat files"; ohne eigenen Wert liefe
// ausgerechnet die wichtigste Datei mit einem Leerwert durch List().
const KnowledgeRootSource = "root"

// Chunk ist ein Abschnitt einer Wissensdatei: alles unter einer Überschrift
// bis zur nächsten. Die Suche findet Chunks, nicht Dateien.
type Chunk struct {
	// Path ist der Ort relativ zum Wissensverzeichnis, immer mit Schrägstrich.
	Path string `json:"path"`
	// Heading ist die Überschrift im Wortlaut, ohne die Rauten. Leer beim
	// Vorspann: Text, der vor der ersten Überschrift steht.
	Heading string `json:"heading"`
	// Anchor ist die Id, die Goldmark der Überschrift gibt — gebildet über
	// denselben Weg wie in der Oberfläche, nie nachgebaut. Für Menschen liest
	// sie sich verstümmelt (Überschriften → berschriften); sie dient allein
	// dem Sprung in der Anzeige.
	Anchor string `json:"anchor"`
	// Text ist der Abschnitt unterhalb der Überschrift, ohne die
	// Überschriftenzeile selbst und ohne Frontmatter; Leerraum an beiden Enden
	// ist abgeschnitten. Die Überschrift steht in Heading und wird beim
	// Suchen mitgewichtet.
	Text string `json:"text"`
	// Source ist der Herkunftsordner: das erste Pfadsegment, oder
	// KnowledgeRootSource für Dateien in der Wurzel.
	Source string `json:"source"`
}

// Ein Hash je Chunk gibt es nicht. Erkannt wird Drift je Datei
// (knowledgeFileEntry.Hash), und neu gechunkt wird immer die ganze Datei: die
// Anker hängen an einem Parser-Lauf über das ganze Dokument. Ein Chunk-Hash
// hätte damit keinen Leser und stünde nur als zweite, ungenutzte Wahrheit in
// jeder Zeile des Index.

// knowledgeFrontmatter sind die Metadaten aus dem Kopf einer Wissensdatei.
// Der Doku-Index verlangt beide Felder; sie gehören in keinen Chunk-Text.
type knowledgeFrontmatter struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
}

// knowledgeFileEntry ist, was der Index über eine Datei weiß.
type knowledgeFileEntry struct {
	// Hash ist SHA-256 über den Dateiinhalt, hexadezimal. Er fängt Dateien,
	// die am Tor vorbei geändert wurden.
	Hash   string `json:"hash"`
	Source string `json:"source"`
	// Title folgt der Regel von ListDocs: erste Überschrift, ersatzweise der
	// Dateiname.
	Title       string               `json:"title"`
	Frontmatter knowledgeFrontmatter `json:"frontmatter"`
}

// knowledgeMarkdown ist der Parser, der die Anker bildet — dieselbe Instanz
// wie die der Oberfläche, aus demselben Paket.
var knowledgeMarkdown = markdown.New()

// KnowledgeDir ist das Wissensverzeichnis eines Projekts.
func KnowledgeDir(projectDir string) string {
	return filepath.Join(LocalDir(projectDir), KnowledgeDirName)
}

// KnowledgeLearnedDir ist der Schreibordner des Wissenstors.
func KnowledgeLearnedDir(projectDir string) string {
	return filepath.Join(KnowledgeDir(projectDir), KnowledgeLearnedDirName)
}

// knowledgeSource leitet die Herkunft aus dem Pfad ab: das erste Segment,
// oder root für eine Datei ohne Verzeichnis.
func knowledgeSource(rel string) string {
	rel = filepath.ToSlash(rel)
	if index := strings.IndexByte(rel, '/'); index >= 0 {
		return rel[:index]
	}
	return KnowledgeRootSource
}

// scanKnowledgeTree sammelt alle Markdown-Dateien unterhalb der Wurzel,
// relativ und mit Schrägstrich, alphabetisch. Eine fehlende Wurzel ist kein
// Fehler: „noch kein Wissen" ist dieselbe Auskunft wie ein leerer Baum.
func scanKnowledgeTree(root string) ([]string, error) {
	if !isDir(root) {
		return []string{}, nil
	}

	paths := []string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			// Ein unlesbarer Teilbaum darf den übrigen Baum nicht verhindern.
			return nil
		}
		if entry.IsDir() {
			if path != root && strings.HasPrefix(entry.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.EqualFold(filepath.Ext(path), ".md") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		paths = append(paths, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("%s lesen: %w", root, err)
	}
	sort.Strings(paths)
	return paths, nil
}

// knowledgeFileHash ist der Datei-Hash, gegen den der Index den Baum prüft.
func knowledgeFileHash(root, rel string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return "", fmt.Errorf("%s lesen: %w", rel, err)
	}
	return sha256Hex(data), nil
}

// chunkKnowledgeFile liest eine Datei und zerlegt sie in Chunks.
//
// Reihenfolge ist zwingend: erst das Frontmatter abtrennen, dann den Rumpf
// einmal ganz parsen und die Anker den Überschriften entnehmen, erst danach
// entlang der Überschriften schneiden. Je Chunk zu parsen bräche die
// dokumentweite Eindeutigkeit der Ids — drei „## Ablauf" bekämen dreimal
// „ablauf", und der Sprung aus einem Treffer landete auf der ersten.
func chunkKnowledgeFile(root, rel string) (knowledgeFileEntry, []Chunk, error) {
	full := filepath.Join(root, filepath.FromSlash(rel))
	data, err := os.ReadFile(full)
	if err != nil {
		return knowledgeFileEntry{}, nil, fmt.Errorf("%s lesen: %w", rel, err)
	}

	entry := knowledgeFileEntry{
		Hash:        sha256Hex(data),
		Source:      knowledgeSource(rel),
		Title:       docTitle(full),
		Frontmatter: parseKnowledgeFrontmatter(data),
	}
	chunks := chunkMarkdown(filepath.ToSlash(rel), entry.Source, inventory.Body(data))
	return entry, chunks, nil
}

// parseKnowledgeFrontmatter liest title und description aus dem Kopf. Ein
// Kopf, den yamllite nicht versteht, ist kein Fehler des Wissenstors: die
// Metadaten bleiben dann leer, die Datei wird trotzdem indiziert.
func parseKnowledgeFrontmatter(data []byte) knowledgeFrontmatter {
	block, ok := inventory.FrontmatterBlock(data)
	if !ok {
		return knowledgeFrontmatter{}
	}
	root, err := yamllite.Parse([]byte(block))
	if err != nil {
		return knowledgeFrontmatter{}
	}
	return knowledgeFrontmatter{
		Title:       strings.TrimSpace(root.Get("title").Str()),
		Description: strings.TrimSpace(root.Get("description").Str()),
	}
}

// headingMark ist eine Überschrift im geparsten Rumpf mit den Offsets, an
// denen geschnitten wird.
type headingMark struct {
	heading string
	anchor  string
	// lineStart ist der Beginn der Überschriftenzeile: hier endet der
	// vorherige Chunk.
	lineStart int
	// bodyStart ist der Beginn der Zeile nach der Überschrift — bei einer
	// Setext-Überschrift nach der Unterstreichung: hier beginnt der Text des
	// Chunks.
	bodyStart int
}

// knowledgeHeadings parst den Rumpf einmal ganz und liest die Überschriften
// samt Anker aus dem Baum. Überschriften in Code-Blöcken sind für den Parser
// keine und tauchen hier nicht auf.
func knowledgeHeadings(source []byte) []headingMark {
	document := knowledgeMarkdown.Parser().Parse(text.NewReader(source))
	marks := []headingMark{}
	_ = ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		heading, ok := node.(*ast.Heading)
		if !ok {
			return ast.WalkContinue, nil
		}
		lines := heading.Lines()
		if lines.Len() == 0 {
			// Eine leere Überschrift („#" allein) trägt keinen Wortlaut und
			// bleibt Teil des laufenden Chunks.
			return ast.WalkContinue, nil
		}
		first := lines.At(0)
		last := lines.At(lines.Len() - 1)
		lineStart := bytes.LastIndexByte(source[:first.Start], '\n') + 1

		var bodyStart int
		if bytes.IndexByte(source[lineStart:first.Start], '#') >= 0 {
			// ATX: die Überschrift ist genau diese eine Zeile.
			bodyStart = nextLineStart(source, first.Start)
		} else {
			// Setext: auf die Textzeilen folgt die Unterstreichung. Ob das
			// Segment den Zeilenumbruch schon enthält, hängt vom Parser ab;
			// beide Fälle landen am Beginn der Unterstreichung.
			underline := last.Stop
			if underline > 0 && underline <= len(source) && source[underline-1] != '\n' {
				underline = nextLineStart(source, underline)
			}
			bodyStart = nextLineStart(source, underline)
		}

		marks = append(marks, headingMark{
			heading:   strings.Join(strings.Fields(string(lines.Value(source))), " "),
			anchor:    headingAnchor(heading),
			lineStart: lineStart,
			bodyStart: bodyStart,
		})
		return ast.WalkContinue, nil
	})
	sort.SliceStable(marks, func(i int, j int) bool { return marks[i].lineStart < marks[j].lineStart })
	return marks
}

// headingAnchor liest die Id, die parser.WithAutoHeadingID vergeben hat.
func headingAnchor(heading *ast.Heading) string {
	value, ok := heading.AttributeString("id")
	if !ok {
		return ""
	}
	switch id := value.(type) {
	case []byte:
		return string(id)
	case string:
		return id
	}
	return ""
}

// nextLineStart liefert den Beginn der Zeile nach der, in der pos liegt —
// oder das Ende der Quelle.
func nextLineStart(source []byte, pos int) int {
	if pos >= len(source) {
		return len(source)
	}
	index := bytes.IndexByte(source[pos:], '\n')
	if index < 0 {
		return len(source)
	}
	return pos + index + 1
}

// chunkMarkdown schneidet den Rumpf entlang der Überschriften. Text vor der
// ersten Überschrift wird ein Chunk mit leerem Heading und Anker — aber nur,
// wenn dort tatsächlich etwas steht; die meisten Dateien beginnen mit ihrer
// Überschrift, und ein leerer Vorspann wäre ein Treffer ohne Inhalt.
func chunkMarkdown(rel, source string, body []byte) []Chunk {
	chunks := []Chunk{}
	add := func(heading, anchor string, from, to int) {
		if from > to {
			from = to
		}
		content := strings.TrimSpace(string(body[from:to]))
		if heading == "" && content == "" {
			return
		}
		chunks = append(chunks, Chunk{
			Path:    rel,
			Heading: heading,
			Anchor:  anchor,
			Text:    content,
			Source:  source,
		})
	}

	marks := knowledgeHeadings(body)
	end := len(body)
	if len(marks) == 0 {
		add("", "", 0, end)
		return chunks
	}
	add("", "", 0, marks[0].lineStart)
	for index, mark := range marks {
		to := end
		if index+1 < len(marks) {
			to = marks[index+1].lineStart
		}
		add(mark.heading, mark.anchor, mark.bodyStart, to)
	}
	return chunks
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Knowledge ist der Zugriffsweg auf das Wissensverzeichnis eines Projekts —
// derselbe für Subkommando und MCP-Hüllen. Er kennt nur das Hauptverzeichnis;
// alles Weitere leitet er daraus ab.
type Knowledge struct {
	projectDir string
	notes      []string
}

// NewKnowledge öffnet den Zugriffsweg für ein Projekt. Gelesen oder
// geschrieben wird dabei noch nichts.
func NewKnowledge(projectDir string) *Knowledge {
	return &Knowledge{projectDir: projectDir}
}

// Notes nennt, was die Aufrufe an diesem Zugriffsweg übergehen mussten, ohne
// daran zu scheitern: ein nicht geschriebener Cache, eine unlesbare Datei. Die
// Antwort steht trotzdem, aber der Aufrufer soll es sagen — das Subkommando
// auf stderr, die MCP-Hülle als hint im Umschlag. Stilles Verschlucken wäre
// die unangenehmste Fehlerart, die dieser Weg haben kann.
func (k *Knowledge) Notes() []string {
	return k.notes
}

func (k *Knowledge) note(format string, args ...any) {
	k.notes = append(k.notes, fmt.Sprintf(format, args...))
}

// knowledgeDefaultLimit ist die Trefferzahl von Search, wenn der Aufrufer
// keine nennt.
const knowledgeDefaultLimit = 10

// knowledgeExcerptRunes ist die Länge, auf die ein Auszug gekürzt wird.
const knowledgeExcerptRunes = 400

// KnowledgeFilter grenzt Search und List ein. Heute ein Feld; weitere kommen
// dazu, ohne dass ein Aufrufer bricht.
type KnowledgeFilter struct {
	// Source ist ein Herkunftswert (code, libs, extracted, versions, manual,
	// learned, root) oder leer für alle.
	Source string `json:"source,omitempty"`
}

// Hit ist ein Suchtreffer — der Vertrag, der später nicht mehr wackeln darf.
//
// Kein Feld verrät das Verfahren: BM25-Werte und Kosinus-Abstände sind nicht
// ineinander übersetzbar, deshalb gibt es kein score. Die Reihenfolge ist die
// Aussage; Rank zählt sie ab 1 mit. Führend ist Heading, die Überschrift im
// Wortlaut; Anchor ist das renderer-gebundene Hilfsfeld für den Sprung.
type Hit struct {
	Path    string `json:"path"`
	Heading string `json:"heading"`
	// Excerpt ist der Anfang des Chunks — nicht ein Fenster um den Treffer,
	// das Term-Positionen voraussetzte, die ein Vektorindex nicht hat.
	Excerpt string `json:"excerpt"`
	Source  string `json:"source"`
	Rank    int    `json:"rank"`
	Anchor  string `json:"anchor"`
}

// KnowledgeEntry ist eine Datei in List.
type KnowledgeEntry struct {
	Path string `json:"path"`
	// Title folgt der Regel von ListDocs: erste Überschrift, ersatzweise der
	// Dateiname — eine Liste aus Dateinamen liest sich schlecht.
	Title  string `json:"title"`
	Source string `json:"source"`
}

// KnowledgeSourceCount ist die Größe einer Herkunft in Status.
type KnowledgeSourceCount struct {
	Files  int `json:"files"`
	Chunks int `json:"chunks"`
}

// KnowledgeStatus trägt heute schon die Felder von morgen: Model und Dims
// bleiben leer, bis ein Vektorindex sie füllt — dann sind sie die Stelle, an
// der ein mit dem einen Modell gebauter und mit einem anderen befragter Index
// auffällt. FileCount und BySource sind die Messgrundlage für die
// RAG-Entscheidung, ausdrücklich als Untergrenze.
type KnowledgeStatus struct {
	IndexKind  string                          `json:"indexKind"`
	Model      string                          `json:"model"`
	Dims       int                             `json:"dims"`
	FileCount  int                             `json:"fileCount"`
	ChunkCount int                             `json:"chunkCount"`
	BySource   map[string]KnowledgeSourceCount `json:"bySource"`
	// BuiltAt ist der Stand des Index als RFC3339-Zeitstempel.
	BuiltAt      time.Time `json:"builtAt"`
	IndexVersion int       `json:"indexVersion"`
	// Stale ist gesetzt, wenn der letzte Zugriff Drift erkannt und behoben
	// hat — jemand hat am Tor vorbei geschrieben. StaleFiles nennt die Zahl
	// der dabei neu gechunkten oder entfernten Dateien.
	Stale      bool `json:"stale"`
	StaleFiles int  `json:"staleFiles"`
}

// Search durchsucht die Chunks. limit 0 heißt knowledgeDefaultLimit.
func (k *Knowledge) Search(query string, filter KnowledgeFilter, limit int) ([]Hit, error) {
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("leere Suchanfrage")
	}
	if limit <= 0 {
		limit = knowledgeDefaultLimit
	}

	index, err := k.open()
	if err != nil {
		return nil, err
	}

	hits := []Hit{}
	for _, scored := range buildBM25(index.Chunks).search(query) {
		chunk := index.Chunks[scored.chunk]
		if filter.Source != "" && chunk.Source != filter.Source {
			continue
		}
		hits = append(hits, Hit{
			Path:    chunk.Path,
			Heading: chunk.Heading,
			Excerpt: knowledgeExcerpt(chunk.Text),
			Source:  chunk.Source,
			Rank:    len(hits) + 1,
			Anchor:  chunk.Anchor,
		})
		if len(hits) == limit {
			break
		}
	}
	return hits, nil
}

// List nennt die Dateien des Wissensverzeichnisses. Die README steht vorn,
// sie ist der Einstieg; der Rest folgt alphabetisch.
func (k *Knowledge) List(filter KnowledgeFilter) ([]KnowledgeEntry, error) {
	index, err := k.open()
	if err != nil {
		return nil, err
	}

	entries := []KnowledgeEntry{}
	for rel, file := range index.Files {
		if filter.Source != "" && file.Source != filter.Source {
			continue
		}
		entries = append(entries, KnowledgeEntry{Path: rel, Title: file.Title, Source: file.Source})
	}
	sort.Slice(entries, func(i int, j int) bool {
		if readme := entries[i].Path == "README.md"; readme != (entries[j].Path == "README.md") {
			return readme
		}
		return entries[i].Path < entries[j].Path
	})
	return entries, nil
}

// Read liefert eine Datei als Markdown, nicht als HTML: gerendert wird an
// einer Stelle, in der Oberfläche. Der Pfad wird geprüft wie bei der
// mitgelieferten Doku — relativ, kein Ausbruch, Markdown-Datei.
func (k *Knowledge) Read(path string) (string, error) {
	full, err := docFilePath(KnowledgeDir(k.projectDir), path)
	if err != nil {
		return "", err
	}
	content, err := os.ReadFile(full)
	if err != nil {
		return "", fmt.Errorf("%s lesen: %w", path, err)
	}
	return string(content), nil
}

// Write legt eine Datei unterhalb von docs/learned/ an oder ersetzt sie.
// path ist relativ zu diesem Ordner; jeder Pfad, der herausführt, wird
// abgewiesen. source geht als Herkunftsvermerk ins Frontmatter, und die
// Chunks der Datei werden im selben Zug aktualisiert. Löschen und Umbenennen
// gibt es nicht.
//
// Das erste Ergebnis ist der geschriebene Ort relativ zum Wissensverzeichnis,
// also mit learned/ davor — so, wie Read, List und Search ihn nennen. Er
// entsteht hier ohnehin; ihn wegzuwerfen und in jedem Aufrufer nachzubauen
// hieße, dieselbe Rechnung dreimal zu führen und sie zweimal falsch haben zu
// können.
//
// Erst der Index, dann die Datei: der Zugriff gleicht zuerst den Baum ab, so
// dass die eigene Schreibung danach nicht als Drift zählt — das Tor selbst
// hat nicht am Tor vorbei geschrieben.
func (k *Knowledge) Write(path, content, source string) (string, error) {
	source = strings.TrimSpace(source)
	if source == "" || strings.ContainsAny(source, "\r\n") {
		return "", fmt.Errorf("kein Herkunftsvermerk: Write braucht ein einzeiliges source")
	}

	learned := KnowledgeLearnedDir(k.projectDir)
	full, err := docFilePath(learned, path)
	if err != nil {
		return "", fmt.Errorf("Pfad %q abgelehnt — geschrieben wird nur unterhalb von %s/%s/%s: %w",
			path, LocalDirName, KnowledgeDirName, KnowledgeLearnedDirName, err)
	}

	index, err := k.open()
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return "", fmt.Errorf("%s anlegen: %w", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(knowledgeWithSource(content, source)), 0o644); err != nil {
		return "", fmt.Errorf("%s schreiben: %w", path, err)
	}

	root := KnowledgeDir(k.projectDir)
	rel, err := filepath.Rel(root, full)
	if err != nil {
		return "", fmt.Errorf("%s einordnen: %w", path, err)
	}
	rel = filepath.ToSlash(rel)
	entry, chunks, err := chunkKnowledgeFile(root, rel)
	if err != nil {
		return "", err
	}
	index.replaceFile(rel, entry, chunks)
	index.BuiltAt = knowledgeNow()
	if err := writeKnowledgeIndex(k.projectDir, index); err != nil {
		return "", err
	}
	return rel, nil
}

// Status liefert Art und Größe des Index samt der Drift-Meldung des letzten
// Zugriffs. Der Aufruf ist selbst ein Zugriff: er gleicht den Baum ab.
func (k *Knowledge) Status() (KnowledgeStatus, error) {
	index, err := k.open()
	if err != nil {
		return KnowledgeStatus{}, err
	}

	bySource := map[string]KnowledgeSourceCount{}
	for _, file := range index.Files {
		count := bySource[file.Source]
		count.Files++
		bySource[file.Source] = count
	}
	for _, chunk := range index.Chunks {
		count := bySource[chunk.Source]
		count.Chunks++
		bySource[chunk.Source] = count
	}

	return KnowledgeStatus{
		IndexKind:    KnowledgeIndexKind,
		FileCount:    len(index.Files),
		ChunkCount:   len(index.Chunks),
		BySource:     bySource,
		BuiltAt:      index.BuiltAt,
		IndexVersion: index.IndexVersion,
		Stale:        index.Stale,
		StaleFiles:   index.StaleFiles,
	}, nil
}

// knowledgeExcerpt bildet den Auszug: der Anfang des Chunk-Texts, Leerraum
// zu einzelnen Leerzeichen zusammengezogen, an einer Wortgrenze auf höchstens
// knowledgeExcerptRunes Zeichen gekürzt und dann mit „…" beendet. Kürzere
// Texte kommen unverändert und ohne Auslassungszeichen zurück.
func knowledgeExcerpt(content string) string {
	flat := strings.Join(strings.Fields(content), " ")
	runes := []rune(flat)
	if len(runes) <= knowledgeExcerptRunes {
		return flat
	}
	cut := knowledgeExcerptRunes
	for cut > 0 && !unicode.IsSpace(runes[cut]) {
		cut--
	}
	if cut == 0 {
		// Ein einziges Wort länger als die Grenze: dann hart schneiden.
		cut = knowledgeExcerptRunes
	}
	return strings.TrimRightFunc(string(runes[:cut]), unicode.IsSpace) + "…"
}

// knowledgeWithSource setzt den Herkunftsvermerk ins Frontmatter: ein
// vorhandenes source-Feld wird ersetzt, ein fehlendes ergänzt, ein fehlender
// Block vorangestellt. Ein title wird nicht erfunden. Zeilenenden werden LF,
// die Datei endet mit einem Zeilenumbruch.
//
// Ein führender ---Block gilt nur als Frontmatter, wenn er als YAML-Abbildung
// durchgeht. „---" als erste Zeile ist gültiges Markdown — ein Thematic
// Break —, und wer den Text dahinter für Frontmatter hielte, schöbe den
// Vermerk vor das nächste „---" mitten im Dokument. inventory.Body() schnitte
// danach alles davor weg: Titel und erste Absätze fehlten in Chunks, Suche und
// Anzeige, während docTitle den Titel weiterhin meldete — list und search
// widersprächen sich.
func knowledgeWithSource(content, source string) string {
	text := strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	line := "source: " + source

	lines := strings.Split(text, "\n")
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		for end := 1; end < len(lines); end++ {
			if strings.TrimSpace(lines[end]) != "---" {
				continue
			}
			if !isKnowledgeFrontmatter(strings.Join(lines[1:end], "\n")) {
				break
			}
			for position := 1; position < end; position++ {
				key, _, found := strings.Cut(lines[position], ":")
				// Leerraum um den Schlüssel gehört nicht dazu: „ source:"
				// und „source :" sind dasselbe Feld, und ein zweites
				// daneben zu schreiben ergäbe einen doppelten Schlüssel.
				if found && strings.TrimSpace(key) == "source" {
					lines[position] = line
					return strings.Join(lines, "\n")
				}
			}
			lines = slices.Insert(lines, end, line)
			return strings.Join(lines, "\n")
		}
	}
	return "---\n" + line + "\n---\n\n" + text
}

// isKnowledgeFrontmatter prüft, ob der führende Block eine YAML-Abbildung ist.
// Geprüft wird mit yamllite — demselben Leser, aus dem parseKnowledgeFrontmatter
// title und description holt. Ein leerer Block zählt mit: dort kann nichts
// verrutschen. Eine Liste oder Fließtext zählt nicht.
//
// Die Prüfung ist absichtlich die schwächere Seite: ein Markdown-Absatz mit
// Doppelpunkt geht als Abbildung durch und würde weiterhin für Frontmatter
// gehalten. Der teure Fall ist der umgekehrte — Inhalt, der nach dem
// Schreiben aus dem Rumpf verschwindet —, und den fängt sie.
func isKnowledgeFrontmatter(block string) bool {
	root, err := yamllite.Parse([]byte(block))
	return err == nil && root != nil && root.Kind == yamllite.Mapping
}
