package project

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
	"time"
	"unicode"
)

// CacheDirName ist das Verzeichnis unterhalb von k-playbook-local, dessen
// Inhalt aus dem Projekt abgeleitet ist und jederzeit neu entsteht. Es ist
// von Anfang an privat (siehe LocalStructure) und darf ohne Rückfrage
// gelöscht werden — der Wissensindex baut sich dann beim nächsten Zugriff neu.
const CacheDirName = "cache"

// KnowledgeCacheDirName ist der Ordner des Wissensindex unterhalb von cache/.
const KnowledgeCacheDirName = "knowledge"

// KnowledgeIndexFileName ist die Indexdatei: Chunks und Datei-Hashes als JSON.
const KnowledgeIndexFileName = "index.json"

// KnowledgeIndexVersion ist die Fassung des Index. Sie steht in der Datei und
// wächst, sobald sich Chunking-Regel oder Schema ändern: Datei-Hashes fangen
// geänderte Dateien, nicht geänderte Regeln. Weicht die Fassung ab, wird
// kommentarlos neu gebaut — nach dem Vorbild von TodoSchemaVersion.
//
// 2: der Hash je Chunk ist entfallen. Er hatte keinen Leser; ein Bestand ohne
// ihn antwortete zwar gleich, trüge das Feld aber ungenutzt bis in alle
// Ewigkeit weiter.
const KnowledgeIndexVersion = 2

// KnowledgeIndexKind benennt das Verfahren hinter dem Index. Es steht nur in
// status; kein Treffer trägt es.
const KnowledgeIndexKind = "bm25"

// knowledgeGoldmarkFallbackVersion ist die Goldmark-Fassung aus go.mod. Sie
// gilt nur, wenn das Binary keine Build-Informationen trägt; sonst kommt der
// Wert aus runtime/debug. Ein Test vergleicht beide, damit ein Bump in go.mod
// hier nicht unbemerkt vorbeigeht.
const knowledgeGoldmarkFallbackVersion = "v1.8.5"

// knowledgeGoldmarkModule ist der Modulpfad, dessen Fassung im Index steht.
const knowledgeGoldmarkModule = "github.com/yuin/goldmark"

// BM25-Parameter, die üblichen Werte. Kein Stemming, keine Stoppwörter.
const (
	bm25K1 = 1.2
	bm25B  = 0.75
)

// knowledgeIndex ist die Datei unter cache/knowledge/index.json.
//
// Gespeichert werden die Chunks und je Datei ein Hash; die Postings der Suche
// entstehen bei jedem Laden neu aus den Chunks — sie zu speichern brächte nur
// eine zweite Wahrheit. Stale und StaleFiles bleiben in der Datei, damit der
// nächste status-Aufruf noch sagt, was der letzte Zugriff behoben hat.
type knowledgeIndex struct {
	IndexVersion    int                           `json:"indexVersion"`
	GoldmarkVersion string                        `json:"goldmarkVersion"`
	BuiltAt         time.Time                     `json:"builtAt"`
	Stale           bool                          `json:"stale"`
	StaleFiles      int                           `json:"staleFiles"`
	Files           map[string]knowledgeFileEntry `json:"files"`
	Chunks          []Chunk                       `json:"chunks"`
}

// knowledgeGoldmarkVersion ist die Goldmark-Fassung, mit der die Anker im
// Index gebildet wurden. Sie hängt per Festlegung an genau einer
// Implementierung; eine andere Fassung macht den Cache still falsch.
var knowledgeGoldmarkVersion = detectGoldmarkVersion()

func detectGoldmarkVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return knowledgeGoldmarkFallbackVersion
	}
	for _, dep := range info.Deps {
		if dep.Path != knowledgeGoldmarkModule {
			continue
		}
		if dep.Replace != nil && dep.Replace.Version != "" {
			return dep.Replace.Version
		}
		if dep.Version != "" {
			return dep.Version
		}
	}
	return knowledgeGoldmarkFallbackVersion
}

// KnowledgeCacheDir ist der Ordner des Wissensindex eines Projekts.
func KnowledgeCacheDir(projectDir string) string {
	return filepath.Join(LocalDir(projectDir), CacheDirName, KnowledgeCacheDirName)
}

// KnowledgeIndexFile ist die Indexdatei eines Projekts.
func KnowledgeIndexFile(projectDir string) string {
	return filepath.Join(KnowledgeCacheDir(projectDir), KnowledgeIndexFileName)
}

func newKnowledgeIndex() *knowledgeIndex {
	return &knowledgeIndex{
		IndexVersion:    KnowledgeIndexVersion,
		GoldmarkVersion: knowledgeGoldmarkVersion,
		Files:           map[string]knowledgeFileEntry{},
		Chunks:          []Chunk{},
	}
}

// readKnowledgeIndex liest die Indexdatei. Das zweite Ergebnis ist false,
// wenn kommentarlos neu zu bauen ist: Datei fehlt, JSON kaputt, Fassung des
// Index oder der Goldmark weicht ab. Der Cache ist wiederherstellbar — das
// ist die Zusage des Verzeichnisses, also ist keiner dieser Fälle ein Fehler.
func readKnowledgeIndex(path string) (*knowledgeIndex, bool) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	index := &knowledgeIndex{}
	if err := json.Unmarshal(content, index); err != nil {
		return nil, false
	}
	if index.IndexVersion != KnowledgeIndexVersion || index.GoldmarkVersion != knowledgeGoldmarkVersion {
		return nil, false
	}
	if index.Files == nil {
		index.Files = map[string]knowledgeFileEntry{}
	}
	if index.Chunks == nil {
		index.Chunks = []Chunk{}
	}
	return index, true
}

// writeKnowledgeIndex schreibt den Index atomar (siehe writeJSONFileAtomic).
// cache/knowledge/ entsteht dabei, wenn es fehlt — und cache/ über
// ensureCacheDir samt der verwalteten .gitignore, damit der abgeleitete Index
// nicht committierbar wird.
func writeKnowledgeIndex(projectDir string, index *knowledgeIndex) error {
	if err := ensureCacheDir(projectDir); err != nil {
		return err
	}
	return writeJSONFileAtomic(KnowledgeIndexFile(projectDir), index, "Wissensindex schreiben")
}

// replaceFile ersetzt Eintrag und Chunks einer Datei. Die Chunks bleiben nach
// Pfad sortiert und innerhalb einer Datei in Dokumentreihenfolge.
func (index *knowledgeIndex) replaceFile(rel string, entry knowledgeFileEntry, chunks []Chunk) {
	index.removeFile(rel)
	index.Files[rel] = entry
	index.Chunks = append(index.Chunks, chunks...)
	sort.SliceStable(index.Chunks, func(i int, j int) bool { return index.Chunks[i].Path < index.Chunks[j].Path })
}

// removeFile nimmt eine Datei samt Chunks aus dem Index.
func (index *knowledgeIndex) removeFile(rel string) {
	delete(index.Files, rel)
	kept := index.Chunks[:0]
	for _, chunk := range index.Chunks {
		if chunk.Path != rel {
			kept = append(kept, chunk)
		}
	}
	index.Chunks = kept
}

// open ist der gemeinsame Zugriffsweg auf den Index: laden oder neu bauen,
// gegen den Baum prüfen, Abweichungen beheben, zurückschreiben.
//
// Der Index darf sich nicht auf das Tor verlassen — git pull, ein Editor und
// die /k-docs-*-Commands schreiben daran vorbei. Deshalb wird bei jedem
// Zugriff der Hash jeder Datei gegen den Baum geprüft; was abweicht, wird
// neu gechunkt, was fehlt, entfernt. Stale meldet dabei nicht einen
// veralteten Zustand — den behebt derselbe Zugriff —, sondern den zuletzt
// behobenen: gesetzt, wenn dieser Zugriff Drift gefunden hat, mit StaleFiles
// als Zahl der betroffenen Dateien; ein Zugriff ohne Drift setzt beides
// zurück. Ein Neubau von Grund auf zählt nicht als Drift: dort gab es keinen
// Stand, an dem jemand vorbeigeschrieben haben könnte.
//
// Scheitert etwas, das nur den Cache betrifft — eine einzelne unlesbare Datei,
// ein nicht beschreibbares cache/ —, antwortet der Zugriff trotzdem: der
// vollständige Index steht im Speicher. Was dabei übergangen wurde, geht als
// Notiz an den Aufrufer (siehe note).
func (k *Knowledge) open() (*knowledgeIndex, error) {
	root := KnowledgeDir(k.projectDir)
	paths, err := scanKnowledgeTree(root)
	if err != nil {
		return nil, err
	}

	index, ok := readKnowledgeIndex(KnowledgeIndexFile(k.projectDir))
	if !ok {
		index = newKnowledgeIndex()
		for _, rel := range paths {
			entry, chunks, err := chunkKnowledgeFile(root, rel)
			if err != nil {
				k.skip(rel, err)
				continue
			}
			index.replaceFile(rel, entry, chunks)
		}
		index.BuiltAt = knowledgeNow()
		k.cache(index)
		return index, nil
	}

	changed := 0
	seen := map[string]bool{}
	for _, rel := range paths {
		entry, chunks, err := reindexKnowledgeFile(index, root, rel)
		if err != nil {
			// Eine unlesbare oder zwischen Walk und Zugriff verschwundene
			// Datei darf die anderen nicht mitreißen: scanKnowledgeTree
			// überspringt unlesbare Teilbäume, hier gilt dasselbe. Sie fällt
			// aus dem Index und zählt als Drift — der Baum sieht anders aus
			// als der Stand, den der Index beschreibt.
			k.skip(rel, err)
			index.removeFile(rel)
			changed++
			continue
		}
		seen[rel] = true
		if chunks == nil {
			continue
		}
		index.replaceFile(rel, entry, chunks)
		changed++
	}
	for rel := range index.Files {
		if !seen[rel] {
			index.removeFile(rel)
			changed++
		}
	}

	switch {
	case changed > 0:
		index.Stale = true
		index.StaleFiles = changed
		index.BuiltAt = knowledgeNow()
	case index.Stale || index.StaleFiles != 0:
		index.Stale = false
		index.StaleFiles = 0
	default:
		return index, nil
	}
	k.cache(index)
	return index, nil
}

// reindexKnowledgeFile prüft eine Datei gegen den Index. Stimmt ihr Hash mit
// dem gespeicherten überein, sind beide Ergebnisse leer: nichts zu tun. Sonst
// kommen Eintrag und Chunks der neu gelesenen Datei zurück.
func reindexKnowledgeFile(index *knowledgeIndex, root, rel string) (knowledgeFileEntry, []Chunk, error) {
	hash, err := knowledgeFileHash(root, rel)
	if err != nil {
		return knowledgeFileEntry{}, nil, err
	}
	if entry, exists := index.Files[rel]; exists && entry.Hash == hash {
		return knowledgeFileEntry{}, nil, nil
	}
	return chunkKnowledgeFile(root, rel)
}

// cache schreibt den Index zurück.
//
// Ein Fehlschlag ist kein Fehler des Zugriffs: der vollständige Index steht im
// Speicher, und Search, List und Status können damit antworten. Ein nicht
// beschreibbares cache/ — ein zeitweise read-only gesetzter Baum, ein fremder
// Eigentümer — darf die Lesewege nicht stilllegen; das Verzeichnis ist
// ausdrücklich verwerfbar, also ist ein fehlender Cache kein Datenverlust.
// Still verschluckt wird der Fehlschlag trotzdem nicht: er geht als Notiz an
// den Aufrufer. Write dagegen scheitert hart, wenn es nicht schreiben kann —
// dort ist der Index nicht Beiwerk, sondern Teil der Zusage.
func (k *Knowledge) cache(index *knowledgeIndex) {
	if err := writeKnowledgeIndex(k.projectDir, index); err != nil {
		k.note("Index nicht zwischengespeichert: %v — gesucht wird trotzdem, der nächste Zugriff baut ihn neu", err)
	}
}

// skip hält fest, dass eine Datei übergangen wurde.
func (k *Knowledge) skip(rel string, err error) {
	k.note("%s übersprungen: %v", rel, err)
}

// knowledgeNow ist der Zeitstempel des Index, sekundengenau und ohne
// monotone Uhr, damit er als RFC3339 in die Datei geht und identisch
// zurückkommt.
func knowledgeNow() time.Time {
	return time.Now().Truncate(time.Second)
}

// bm25Index ist die Suchstruktur über den Chunks eines Index. Sie entsteht
// bei jedem Laden neu; gespeichert wird sie nicht.
type bm25Index struct {
	frequencies []map[string]int
	lengths     []int
	documents   map[string]int
	average     float64
}

type bm25Hit struct {
	chunk int
	score float64
}

// buildBM25 tokenisiert Überschrift und Text jedes Chunks. Die Überschrift
// fließt mit ein: wer nach ihr sucht, soll den Abschnitt darunter finden.
func buildBM25(chunks []Chunk) *bm25Index {
	index := &bm25Index{
		frequencies: make([]map[string]int, len(chunks)),
		lengths:     make([]int, len(chunks)),
		documents:   map[string]int{},
	}
	total := 0
	for position, chunk := range chunks {
		tokens := knowledgeTokens(chunk.Heading + " " + chunk.Text)
		frequency := map[string]int{}
		for _, token := range tokens {
			frequency[token]++
		}
		for token := range frequency {
			index.documents[token]++
		}
		index.frequencies[position] = frequency
		index.lengths[position] = len(tokens)
		total += len(tokens)
	}
	if len(chunks) > 0 {
		index.average = float64(total) / float64(len(chunks))
	}
	return index
}

// search bewertet jeden Chunk gegen die Anfrage und liefert die Treffer mit
// positivem Gewicht, bestes zuerst; bei gleichem Gewicht entscheidet die
// Reihenfolge im Index.
func (index *bm25Index) search(query string) []bm25Hit {
	terms := knowledgeTokens(query)
	if len(terms) == 0 || len(index.frequencies) == 0 {
		return nil
	}
	count := float64(len(index.frequencies))
	hits := []bm25Hit{}
	for position, frequency := range index.frequencies {
		score := 0.0
		for _, term := range terms {
			tf := frequency[term]
			if tf == 0 {
				continue
			}
			df := float64(index.documents[term])
			idf := math.Log(1 + (count-df+0.5)/(df+0.5))
			length := float64(index.lengths[position])
			norm := 1 - bm25B + bm25B*length/math.Max(index.average, 1)
			score += idf * float64(tf) * (bm25K1 + 1) / (float64(tf) + bm25K1*norm)
		}
		if score > 0 {
			hits = append(hits, bm25Hit{chunk: position, score: score})
		}
	}
	sort.SliceStable(hits, func(i int, j int) bool { return hits[i].score > hits[j].score })
	return hits
}

// knowledgeTokens zerlegt Text in Kleinbuchstaben-Wörter aus
// Unicode-Buchstaben und -Ziffern. Kein Stemming: „Anker" und „Ankern" sind
// zwei Terme, und das ist für einen Index dieser Größe die ehrlichere Wahl.
func knowledgeTokens(value string) []string {
	tokens := []string{}
	var current strings.Builder
	flush := func() {
		if current.Len() > 0 {
			tokens = append(tokens, current.String())
			current.Reset()
		}
	}
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			current.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	return tokens
}
