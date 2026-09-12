package project

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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

// KnowledgeDirName ist die Wissensablage unterhalb von k-playbook-local: die
// dritte Zone aus docs/knowledge-layout.md, das, was gilt. Nur was hier liegt,
// wird indiziert, durchsucht und gelesen (siehe den Eintrag knowledge in
// LocalStructure). Bis zur Migration ist sie in jedem Projekt leer — das
// heutige docs/ bleibt daneben bestehen und trägt weiter; ein Rückfall dorthin
// wird bewusst nicht gebaut.
const KnowledgeDirName = "knowledge"

// InboxDirName ist der Eingang unterhalb von k-playbook-local: Rohmaterial in
// jedem Format, nach Quelle unterteilt (inbox/<quelle>/…), nie indiziert. Ein
// Archiv, keine Warteschlange — was hier liegt, bleibt, bis eine Person es
// entfernt (siehe den Eintrag inbox in LocalStructure).
const InboxDirName = "inbox"

// QueueDirName ist die Warteschlange unterhalb von k-playbook-local: je ein
// Markdown-Eintrag für ein Rohstück, das Wissen werden soll. Ein Eintrag
// verschwindet, wenn sein Dokument steht — deshalb heißt ein leeres queue/:
// nichts offen (siehe den Eintrag queue in LocalStructure).
const QueueDirName = "queue"

// KnowledgeRootKind ist die Art einer Datei, die flach in der Wurzel der
// Wissensablage liegt — die README.md, der Index selbst, den /k-docs-index
// schreibt. Ohne eigenen Wert liefe ausgerechnet die wichtigste Datei mit
// einem Leerwert durch List().
const KnowledgeRootKind = "root"

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
	// Kind ist die Art: das erste Pfadsegment — der Eigentümer, den der Pfad
	// trägt —, oder KnowledgeRootKind für Dateien in der Wurzel. Sie kommt
	// aus dem Pfad und nie aus dem Dokument: ein Feld, das den Pfad
	// wiederholt, ist ein Feld, das ihm widersprechen kann.
	Kind string `json:"kind"`
}

// Ein Hash je Chunk gibt es nicht. Erkannt wird Drift je Datei
// (knowledgeFileEntry.Hash), und neu gechunkt wird immer die ganze Datei: die
// Anker hängen an einem Parser-Lauf über das ganze Dokument. Ein Chunk-Hash
// hätte damit keinen Leser und stünde nur als zweite, ungenutzte Wahrheit in
// jeder Zeile des Index.

// knowledgeFrontmatter sind die Metadaten aus dem Kopf einer Wissensdatei —
// der Vertrag aus docs/knowledge-layout.md, soweit der Index ihn liest. Sie
// gehören in keinen Chunk-Text. State hat Zähne: raw und superseded fehlen in
// Search standardmäßig; List und Read führen sie weiter.
type knowledgeFrontmatter struct {
	Title   string `json:"title,omitempty"`
	Subject string `json:"subject,omitempty"`
	Origin  string `json:"origin,omitempty"`
	State   string `json:"state,omitempty"`
	Format  string `json:"format,omitempty"`
	Updated string `json:"updated,omitempty"`
}

// knowledgeFileEntry ist, was der Index über eine Datei weiß.
type knowledgeFileEntry struct {
	// Hash ist SHA-256 über den Dateiinhalt, hexadezimal. Er fängt Dateien,
	// die am Tor vorbei geändert wurden.
	Hash string `json:"hash"`
	Kind string `json:"kind"`
	// Title folgt knowledgeTitle: Frontmatter-Titel, sonst erste Überschrift,
	// ersatzweise der Dateiname.
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

// knowledgeKind leitet die Art aus dem Pfad ab: das erste Segment, oder root
// für eine Datei ohne Verzeichnis.
func knowledgeKind(rel string) string {
	rel = filepath.ToSlash(rel)
	if index := strings.IndexByte(rel, '/'); index >= 0 {
		return rel[:index]
	}
	return KnowledgeRootKind
}

// knowledgeSearchable meldet, ob eine Datei Chunks in den Suchindex gibt.
// Die README in der Wurzel gibt keine: sie ist erzeugte Navigation ohne
// eigenen Inhalt, und ihr Stichwortindex — kurz, begriffsdicht, genau die
// Form, die BM25 doppelt belohnt — verdrängte die Dokumente, auf die er
// zeigt (belegt in material/befunde/wissenstor-mcp.md). List führt sie
// weiter; Navigation ist dort besser aufgehoben als im Ranking.
func knowledgeSearchable(rel string) bool {
	return filepath.ToSlash(rel) != knowledgeReadmeName
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

	frontmatter := parseKnowledgeFrontmatter(data)
	entry := knowledgeFileEntry{
		Hash:        sha256Hex(data),
		Kind:        knowledgeKind(rel),
		Title:       knowledgeTitle(full, frontmatter),
		Frontmatter: frontmatter,
	}
	if !knowledgeSearchable(rel) {
		return entry, []Chunk{}, nil
	}
	chunks := chunkMarkdown(filepath.ToSlash(rel), entry.Kind, inventory.Body(data))
	return entry, chunks, nil
}

// knowledgeTitle ist der Titel, unter dem List und Index eine Datei führen:
// der Frontmatter-Titel, wenn gesetzt — was ein Aufrufer bei write als
// Pflichtfeld angeben musste, kommt beim Lesen auch zurück —, sonst wie bei
// ListDocs die erste Überschrift, ersatzweise der Dateiname. So bleiben die
// Wurzel-README und fremde Dateien ohne Kopf lesbar benannt.
func knowledgeTitle(full string, frontmatter knowledgeFrontmatter) string {
	if frontmatter.Title != "" {
		return frontmatter.Title
	}
	return docTitle(full)
}

// parseKnowledgeFrontmatter liest die Vertragsfelder aus dem Kopf. Ein Kopf,
// den yamllite nicht versteht, ist kein Fehler des Wissenstors: die Metadaten
// bleiben dann leer, die Datei wird trotzdem indiziert — und ohne state gilt
// sie als suchbar; eine Datei am Tor vorbei soll nicht unsichtbar werden.
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
		Title:   strings.TrimSpace(root.Get("title").Str()),
		Subject: strings.TrimSpace(root.Get("subject").Str()),
		Origin:  strings.TrimSpace(root.Get("origin").Str()),
		State:   strings.TrimSpace(root.Get("state").Str()),
		Format:  strings.TrimSpace(root.Get("format").Str()),
		Updated: strings.TrimSpace(root.Get("updated").Str()),
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
func chunkMarkdown(rel, kind string, body []byte) []Chunk {
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
			Kind:    kind,
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
// dazu, ohne dass ein Aufrufer bricht. Ein Filter auf state ist bewusst
// keiner: das ist Leseseite.
type KnowledgeFilter struct {
	// Kind ist die Art aus dem Pfad (code, libs, versions, extracted,
	// external, findings, pitfalls, manual, root) oder leer für alle.
	Kind string `json:"kind,omitempty"`
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
	// Kind ist die Art aus dem Pfad; Origin und State kommen aus dem
	// Frontmatter und fehlen, wo das Dokument keines trägt.
	Kind   string `json:"kind"`
	Origin string `json:"origin,omitempty"`
	State  string `json:"state,omitempty"`
	Rank   int    `json:"rank"`
	Anchor string `json:"anchor"`
}

// KnowledgeEntry ist eine Datei in List.
type KnowledgeEntry struct {
	Path string `json:"path"`
	// Title ist der Frontmatter-Titel, sonst die erste Überschrift,
	// ersatzweise der Dateiname (knowledgeTitle) — eine Liste aus Dateinamen
	// liest sich schlecht.
	Title  string `json:"title"`
	Kind   string `json:"kind"`
	Origin string `json:"origin,omitempty"`
	State  string `json:"state,omitempty"`
}

// KnowledgeKindCount ist die Größe einer Art in Status.
type KnowledgeKindCount struct {
	Files  int `json:"files"`
	Chunks int `json:"chunks"`
}

// KnowledgeStatus trägt heute schon die Felder von morgen: Model und Dims
// bleiben leer, bis ein Vektorindex sie füllt — dann sind sie die Stelle, an
// der ein mit dem einen Modell gebauter und mit einem anderen befragter Index
// auffällt. FileCount und ByKind sind die Messgrundlage für die
// RAG-Entscheidung, ausdrücklich als Untergrenze.
type KnowledgeStatus struct {
	IndexKind  string                        `json:"indexKind"`
	Model      string                        `json:"model"`
	Dims       int                           `json:"dims"`
	FileCount  int                           `json:"fileCount"`
	ChunkCount int                           `json:"chunkCount"`
	ByKind     map[string]KnowledgeKindCount `json:"byKind"`
	// BuiltAt ist der Stand des Index als RFC3339-Zeitstempel.
	BuiltAt      time.Time `json:"builtAt"`
	IndexVersion int       `json:"indexVersion"`
	// Stale ist gesetzt, wenn der letzte Zugriff Drift erkannt und behoben
	// hat — jemand hat am Tor vorbei geschrieben. StaleFiles nennt die Zahl
	// der dabei neu gechunkten oder entfernten Dateien.
	Stale      bool `json:"stale"`
	StaleFiles int  `json:"staleFiles"`
}

// Search durchsucht die Chunks. limit 0 heißt knowledgeDefaultLimit. Chunks
// aus Dokumenten mit state raw oder superseded bleiben draußen: Rohes würde
// die verdichtete Fassung seiner selbst überdecken, Abgelöstes hat aufgehört
// zu gelten. Beides bleibt über Read und List erreichbar.
func (k *Knowledge) Search(query string, filter KnowledgeFilter, limit int) ([]Hit, error) {
	if strings.TrimSpace(query) == "" {
		return nil, InputErrorf("leere Suchanfrage")
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
		if filter.Kind != "" && chunk.Kind != filter.Kind {
			continue
		}
		file := index.Files[chunk.Path]
		if knowledgeHiddenState(file.Frontmatter.State) {
			continue
		}
		hits = append(hits, Hit{
			Path:    chunk.Path,
			Heading: chunk.Heading,
			Excerpt: knowledgeExcerpt(chunk.Text),
			Kind:    chunk.Kind,
			Origin:  file.Frontmatter.Origin,
			State:   file.Frontmatter.State,
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
		if filter.Kind != "" && file.Kind != filter.Kind {
			continue
		}
		entries = append(entries, KnowledgeEntry{
			Path: rel, Title: file.Title, Kind: file.Kind,
			Origin: file.Frontmatter.Origin, State: file.Frontmatter.State,
		})
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
		if errors.Is(err, fs.ErrNotExist) {
			// Nicht vorhanden ist ein Eingabefehler: der Aufrufer hat einen
			// Pfad genannt, den es nicht gibt, und kann ihn korrigieren.
			// Vorhanden, aber unlesbar, ist die Umgebung.
			return "", InputErrorf("%s gibt es nicht in der Wissensablage — list nennt, was dort liegt", path)
		}
		return "", fmt.Errorf("%s lesen: %w", path, err)
	}
	return string(content), nil
}

// Status liefert Art und Größe des Index samt der Drift-Meldung des letzten
// Zugriffs. Der Aufruf ist selbst ein Zugriff: er gleicht den Baum ab.
func (k *Knowledge) Status() (KnowledgeStatus, error) {
	index, err := k.open()
	if err != nil {
		return KnowledgeStatus{}, err
	}

	byKind := map[string]KnowledgeKindCount{}
	for _, file := range index.Files {
		count := byKind[file.Kind]
		count.Files++
		byKind[file.Kind] = count
	}
	for _, chunk := range index.Chunks {
		count := byKind[chunk.Kind]
		count.Chunks++
		byKind[chunk.Kind] = count
	}

	return KnowledgeStatus{
		IndexKind:    KnowledgeIndexKind,
		FileCount:    len(index.Files),
		ChunkCount:   len(index.Chunks),
		ByKind:       byKind,
		BuiltAt:      index.BuiltAt,
		IndexVersion: index.IndexVersion,
		Stale:        index.Stale,
		StaleFiles:   index.StaleFiles,
	}, nil
}

// knowledgeHiddenState meldet, ob ein state ein Dokument aus der Suche hält.
func knowledgeHiddenState(state string) bool {
	return state == KnowledgeStateRaw || state == KnowledgeStateSuperseded
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
