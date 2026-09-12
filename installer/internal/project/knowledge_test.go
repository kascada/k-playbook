package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
	"testing"
	"time"
)

// knowledgeFixture legt ein Projekt mit einer Wissensablage an, die die drei
// Generatorordner, extracted/, manual/, findings/ und die README in der Wurzel
// trägt, und gibt das Hauptverzeichnis zurück.
func knowledgeFixture(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	files := map[string]string{
		"README.md": "---\ntitle: Index\ndescription: Der Einstieg\n---\n\n# k-playbook – Dokumentation\n\n" +
			"Diese Dokumentation beschreibt das Setup.\n\n## Übersicht der Dokumente\n\nTabelle der Dateien.\n",
		"code/links.md":         "# Verlinkung\n\nWie Symlinks entstehen.\n\n## Ablauf\n\nErster Ablauf.\n\n## Ablauf\n\nZweiter Ablauf.\n\n## Ablauf\n\nDritter Ablauf.\n",
		"libs/goldmark.md":      "# Goldmark\n\nFallen beim Rendern.\n\n## Überschriften\n\nUmlaute fallen aus der Id.\n",
		"extracted/sitzung.md":  "# Sitzung\n\nAus einem Mitschnitt extrahiert.\n\n## Codeblock\n\n```md\n# Keine Überschrift\n## Auch keine\n```\n\nDanach Text.\n",
		"versions/inventory.md": "---\ntitle: Versionsinventar\nsubject: Versionen\norigin: /k-doc-inventory\nstate: condensed\nformat: markdown\nupdated: 2026-09-10\n---\n\n# Versionsinventar\n\nStand heute.\n\n## Laufzeiten\n\nGo 1.26.\n",
		"manual/release.md":     "# Release\n\nWie ein Release entsteht.\n\n## Der Weg\n\nTag setzen, CI abwarten.\n",
		"findings/notiz.md":     "---\ntitle: Gelernt\nsubject: Release\norigin: Sitzung 42\nstate: reviewed\nformat: markdown\nupdated: 2026-09-10\n---\n\n# Gelernt\n\nDer Befehl war make sichern.\n",
	}
	for rel, content := range files {
		writeKnowledgeFile(t, root, rel, content)
	}
	return root
}

func writeKnowledgeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(KnowledgeDir(root), filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("%s anlegen: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("%s schreiben: %v", rel, err)
	}
}

func chunkFixtureFile(t *testing.T, root, rel string) (knowledgeFileEntry, []Chunk) {
	t.Helper()
	entry, chunks, err := chunkKnowledgeFile(KnowledgeDir(root), rel)
	if err != nil {
		t.Fatalf("%s chunken: %v", rel, err)
	}
	return entry, chunks
}

// Die Art ist das erste Pfadsegment; eine Datei flach in der Wurzel bekommt
// ausdrücklich root — der Leerwert wäre ein Zufall, kein Vertrag. Die README
// gibt keine Chunks in den Suchindex, alle anderen schon.
func TestKnowledgeKindJeEigentuemerordner(t *testing.T) {
	root := knowledgeFixture(t)

	want := map[string]string{
		"README.md":             "root",
		"code/links.md":         "code",
		"libs/goldmark.md":      "libs",
		"extracted/sitzung.md":  "extracted",
		"versions/inventory.md": "versions",
		"manual/release.md":     "manual",
		"findings/notiz.md":     "findings",
	}
	paths, err := scanKnowledgeTree(KnowledgeDir(root))
	if err != nil {
		t.Fatalf("Baum lesen: %v", err)
	}
	if len(paths) != len(want) {
		t.Fatalf("Baum: %v", paths)
	}
	for _, rel := range paths {
		entry, chunks := chunkFixtureFile(t, root, rel)
		if entry.Kind != want[rel] {
			t.Errorf("%s: Kind = %q, erwartet %q", rel, entry.Kind, want[rel])
		}
		if rel == "README.md" {
			if len(chunks) != 0 {
				t.Errorf("README.md gibt Chunks in den Suchindex: %+v", chunks)
			}
			continue
		}
		if len(chunks) == 0 {
			t.Errorf("%s: keine Chunks", rel)
		}
		for _, chunk := range chunks {
			if chunk.Kind != want[rel] || chunk.Path != rel {
				t.Errorf("%s: Chunk %+v", rel, chunk)
			}
		}
	}
}

// Der Anker kommt aus Goldmark: Nicht-ASCII fällt ersatzlos weg.
func TestKnowledgeAnkerVerwirftUmlaute(t *testing.T) {
	root := knowledgeFixture(t)
	_, chunks := chunkFixtureFile(t, root, "libs/goldmark.md")

	found := false
	for _, chunk := range chunks {
		if chunk.Heading == "Überschriften" {
			found = true
			if chunk.Anchor != "berschriften" {
				t.Errorf("Anchor = %q, erwartet berschriften", chunk.Anchor)
			}
			if chunk.Text != "Umlaute fallen aus der Id." {
				t.Errorf("Text = %q", chunk.Text)
			}
		}
	}
	if !found {
		t.Fatalf("Chunk „Überschriften“ fehlt: %+v", chunks)
	}
}

// Die Eindeutigkeit der Ids zählt über das ganze Dokument — nur, weil die
// Datei einmal ganz geparst wird.
func TestKnowledgeAnkerEindeutigUeberDasDokument(t *testing.T) {
	root := knowledgeFixture(t)
	_, chunks := chunkFixtureFile(t, root, "code/links.md")

	var anchors []string
	var texts []string
	for _, chunk := range chunks {
		if chunk.Heading == "Ablauf" {
			anchors = append(anchors, chunk.Anchor)
			texts = append(texts, chunk.Text)
		}
	}
	if strings.Join(anchors, ",") != "ablauf,ablauf-1,ablauf-2" {
		t.Errorf("Anker = %v", anchors)
	}
	if strings.Join(texts, "|") != "Erster Ablauf.|Zweiter Ablauf.|Dritter Ablauf." {
		t.Errorf("Texte = %v", texts)
	}
}

// Frontmatter ist Metadatum, kein Text: die Vertragsfelder landen am
// Eintrag, in keinem Chunk steht „title:".
func TestKnowledgeFrontmatterBleibtAusDemText(t *testing.T) {
	root := knowledgeFixture(t)
	entry, chunks := chunkFixtureFile(t, root, "versions/inventory.md")

	want := knowledgeFrontmatter{Title: "Versionsinventar", Subject: "Versionen", Origin: "/k-doc-inventory", State: "condensed", Format: "markdown", Updated: "2026-09-10"}
	if entry.Frontmatter != want {
		t.Errorf("Frontmatter = %+v, erwartet %+v", entry.Frontmatter, want)
	}
	if entry.Title != "Versionsinventar" {
		t.Errorf("Title = %q", entry.Title)
	}
	for _, chunk := range chunks {
		if strings.Contains(chunk.Text, "title:") || strings.Contains(chunk.Text, "---") {
			t.Errorf("Frontmatter im Chunk: %q", chunk.Text)
		}
	}
	if len(chunks) != 2 || chunks[0].Heading != "Versionsinventar" || chunks[0].Text != "Stand heute." {
		t.Errorf("Chunks = %+v", chunks)
	}
	// Ohne Kopf bleiben die Felder leer — und die Datei wird trotzdem indiziert.
	entry, _ = chunkFixtureFile(t, root, "manual/release.md")
	if entry.Frontmatter != (knowledgeFrontmatter{}) {
		t.Errorf("ohne Kopf: %+v", entry.Frontmatter)
	}
}

// Eine Raute in einem Code-Block ist keine Überschrift — der Parser weiß das,
// ein Regex wüsste es nicht.
func TestKnowledgeHeadingImCodeBlockIstKeinChunk(t *testing.T) {
	root := knowledgeFixture(t)
	_, chunks := chunkFixtureFile(t, root, "extracted/sitzung.md")

	if len(chunks) != 2 {
		t.Fatalf("erwartet 2 Chunks, bekommen %d: %+v", len(chunks), chunks)
	}
	block := chunks[1]
	if block.Heading != "Codeblock" || block.Anchor != "codeblock" {
		t.Errorf("Chunk = %+v", block)
	}
	if !strings.Contains(block.Text, "# Keine Überschrift") || !strings.HasSuffix(block.Text, "Danach Text.") {
		t.Errorf("Code-Block nicht im Text des umgebenden Chunks: %q", block.Text)
	}
}

// Text vor der ersten Überschrift wird ein Chunk ohne Heading und Anker;
// beginnt die Datei mit ihrer Überschrift, gibt es keinen leeren Vorspann.
func TestKnowledgeVorspannOhneUeberschrift(t *testing.T) {
	root := knowledgeFixture(t)
	writeKnowledgeFile(t, root, "manual/vorspann.md", "Ein Satz vor allem.\n\n# Titel\n\nInhalt.\n")
	writeKnowledgeFile(t, root, "manual/ohne.md", "Nur Fließtext, keine Überschrift.\n")

	_, chunks := chunkFixtureFile(t, root, "manual/vorspann.md")
	if len(chunks) != 2 || chunks[0].Heading != "" || chunks[0].Anchor != "" || chunks[0].Text != "Ein Satz vor allem." {
		t.Errorf("Vorspann: %+v", chunks)
	}
	if chunks[1].Heading != "Titel" || chunks[1].Text != "Inhalt." {
		t.Errorf("Titel-Chunk: %+v", chunks[1])
	}

	_, chunks = chunkFixtureFile(t, root, "manual/ohne.md")
	if len(chunks) != 1 || chunks[0].Heading != "" || chunks[0].Text != "Nur Fließtext, keine Überschrift." {
		t.Errorf("ohne Überschrift: %+v", chunks)
	}

	_, chunks = chunkFixtureFile(t, root, "manual/release.md")
	if len(chunks) != 2 || chunks[0].Heading != "Release" {
		t.Errorf("kein leerer Vorspann erwartet: %+v", chunks)
	}
}

// Setext-Überschriften schneiden hinter der Unterstreichung; ATX-Überschriften
// mit schließenden Rauten tragen den Wortlaut ohne sie.
func TestKnowledgeSetextUndSchliessendeRauten(t *testing.T) {
	root := knowledgeFixture(t)
	writeKnowledgeFile(t, root, "manual/setext.md", "Titel\n=====\n\nUnter dem Titel.\n\nZweiter\n-------\n\nUnter dem Zweiten.\n\n## Dritter ##\n\nUnter dem Dritten.\n")

	_, chunks := chunkFixtureFile(t, root, "manual/setext.md")
	if len(chunks) != 3 {
		t.Fatalf("erwartet 3 Chunks: %+v", chunks)
	}
	want := []Chunk{
		{Heading: "Titel", Anchor: "titel", Text: "Unter dem Titel."},
		{Heading: "Zweiter", Anchor: "zweiter", Text: "Unter dem Zweiten."},
		{Heading: "Dritter", Anchor: "dritter", Text: "Unter dem Dritten."},
	}
	for index, chunk := range chunks {
		if chunk.Heading != want[index].Heading || chunk.Anchor != want[index].Anchor || chunk.Text != want[index].Text {
			t.Errorf("Chunk %d = %+v, erwartet %+v", index, chunk, want[index])
		}
	}
}

// Ein fehlendes Wissensverzeichnis ist ein leerer Baum, kein Fehler.
func TestKnowledgeFehlendesVerzeichnisIstLeer(t *testing.T) {
	paths, err := scanKnowledgeTree(filepath.Join(t.TempDir(), "gibt-es-nicht"))
	if err != nil || len(paths) != 0 {
		t.Errorf("paths = %v, err = %v", paths, err)
	}
}

func openFixtureIndex(t *testing.T, root string) *knowledgeIndex {
	t.Helper()
	index, err := NewKnowledge(root).open()
	if err != nil {
		t.Fatalf("Index öffnen: %v", err)
	}
	return index
}

func readIndexFile(t *testing.T, root string) *knowledgeIndex {
	t.Helper()
	index, ok := readKnowledgeIndex(KnowledgeIndexFile(root))
	if !ok {
		t.Fatalf("Indexdatei %s nicht lesbar", KnowledgeIndexFile(root))
	}
	return index
}

// Der erste Zugriff baut den Index unter cache/knowledge/ und legt den Ordner
// an; ein Neubau ist keine Drift.
func TestKnowledgeIndexEntstehtBeimErstenZugriff(t *testing.T) {
	root := knowledgeFixture(t)

	index := openFixtureIndex(t, root)
	if index.Stale || index.StaleFiles != 0 {
		t.Errorf("Neubau als Drift gemeldet: stale=%v staleFiles=%d", index.Stale, index.StaleFiles)
	}
	if len(index.Files) != 7 {
		t.Errorf("Files = %d", len(index.Files))
	}
	if index.IndexVersion != KnowledgeIndexVersion || index.GoldmarkVersion != knowledgeGoldmarkVersion {
		t.Errorf("Fassungen: %d / %q", index.IndexVersion, index.GoldmarkVersion)
	}
	if index.BuiltAt.IsZero() {
		t.Error("BuiltAt leer")
	}

	stored := readIndexFile(t, root)
	if len(stored.Chunks) != len(index.Chunks) || !stored.BuiltAt.Equal(index.BuiltAt) {
		t.Errorf("gespeichert: %d Chunks, builtAt %s; im Speicher: %d, %s", len(stored.Chunks), stored.BuiltAt, len(index.Chunks), index.BuiltAt)
	}
}

// Eine Datei, die nach dem Indexbau am Tor vorbei geändert wird, wird beim
// nächsten Zugriff neu gechunkt und gemeldet; der Zugriff danach meldet
// nichts mehr.
func TestKnowledgeIndexErkenntDrift(t *testing.T) {
	root := knowledgeFixture(t)
	openFixtureIndex(t, root)

	writeKnowledgeFile(t, root, "manual/release.md", "# Release\n\nNeu geschrieben.\n\n## Der Weg\n\nAnders.\n\n## Ganz neu\n\nDazu.\n")

	index := openFixtureIndex(t, root)
	if !index.Stale || index.StaleFiles != 1 {
		t.Errorf("stale=%v staleFiles=%d, erwartet true/1", index.Stale, index.StaleFiles)
	}
	headings := []string{}
	for _, chunk := range index.Chunks {
		if chunk.Path == "manual/release.md" {
			headings = append(headings, chunk.Heading)
		}
	}
	if strings.Join(headings, ",") != "Release,Der Weg,Ganz neu" {
		t.Errorf("Chunks nach Drift: %v", headings)
	}
	if stored := readIndexFile(t, root); !stored.Stale || stored.StaleFiles != 1 {
		t.Errorf("Drift nicht persistiert: %+v", stored)
	}

	index = openFixtureIndex(t, root)
	if index.Stale || index.StaleFiles != 0 {
		t.Errorf("zweiter Zugriff: stale=%v staleFiles=%d, erwartet false/0", index.Stale, index.StaleFiles)
	}
	if stored := readIndexFile(t, root); stored.Stale || stored.StaleFiles != 0 {
		t.Errorf("Rücksetzung nicht persistiert: %+v", stored)
	}
}

// Eine gelöschte und eine neue Datei zählen ebenfalls als Drift.
func TestKnowledgeIndexEntferntGeloeschteDateien(t *testing.T) {
	root := knowledgeFixture(t)
	openFixtureIndex(t, root)

	if err := os.Remove(filepath.Join(KnowledgeDir(root), "libs", "goldmark.md")); err != nil {
		t.Fatalf("löschen: %v", err)
	}
	writeKnowledgeFile(t, root, "code/neu.md", "# Neu\n\nDazu gekommen.\n")

	index := openFixtureIndex(t, root)
	if !index.Stale || index.StaleFiles != 2 {
		t.Errorf("stale=%v staleFiles=%d, erwartet true/2", index.Stale, index.StaleFiles)
	}
	if _, there := index.Files["libs/goldmark.md"]; there {
		t.Error("gelöschte Datei noch im Index")
	}
	if _, there := index.Files["code/neu.md"]; !there {
		t.Error("neue Datei fehlt im Index")
	}
	for _, chunk := range index.Chunks {
		if chunk.Path == "libs/goldmark.md" {
			t.Errorf("Chunk der gelöschten Datei geblieben: %+v", chunk)
		}
	}
}

// Kaputtes JSON und eine fremde Fassung führen kommentarlos zum Neubau. Die
// Fassung 2 ist der Index, den v0.7.0 über docs/ gebaut hat: er wird ersetzt,
// nicht fortgeführt — sonst zählte der erste Zugriff über knowledge/ jede
// seiner Dateien als Drift.
func TestKnowledgeIndexNeubauBeiKaputterOderFremderDatei(t *testing.T) {
	for name, content := range map[string]string{
		"kaputt":          "{ das ist kein JSON",
		"indexVersion":    `{"indexVersion": 999, "goldmarkVersion": "` + knowledgeGoldmarkVersion + `", "stale": true, "staleFiles": 5, "files": {}, "chunks": []}`,
		"docs-Fassung":    `{"indexVersion": 2, "goldmarkVersion": "` + knowledgeGoldmarkVersion + `", "stale": false, "staleFiles": 0, "files": {"manual/alt.md": {"hash": "x", "source": "manual", "title": "Alt"}}, "chunks": [{"path": "manual/alt.md", "heading": "Alt", "anchor": "alt", "text": "Aus docs/.", "source": "manual"}]}`,
		"goldmarkVersion": fmt.Sprintf(`{"indexVersion": %d, "goldmarkVersion": "v0.0.1", "stale": true, "staleFiles": 5, "files": {}, "chunks": []}`, KnowledgeIndexVersion),
	} {
		t.Run(name, func(t *testing.T) {
			root := knowledgeFixture(t)
			if err := os.MkdirAll(KnowledgeCacheDir(root), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(KnowledgeIndexFile(root), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}

			index := openFixtureIndex(t, root)
			if index.Stale || index.StaleFiles != 0 {
				t.Errorf("Neubau als Drift gemeldet: %+v", index)
			}
			if len(index.Files) != 7 || len(index.Chunks) == 0 {
				t.Errorf("Neubau unvollständig: %d Dateien, %d Chunks", len(index.Files), len(index.Chunks))
			}
			stored := readIndexFile(t, root)
			if stored.IndexVersion != KnowledgeIndexVersion || stored.GoldmarkVersion != knowledgeGoldmarkVersion {
				t.Errorf("Fassungen nach Neubau: %d / %q", stored.IndexVersion, stored.GoldmarkVersion)
			}
		})
	}
}

// Die Goldmark-Fassung im Index muss die aus go.mod sein. Im Testbinary
// fehlen die Modulabhängigkeiten in den Build-Informationen ganz (gemessen
// mit `go version -m` gegen `go test -c`; das gebaute k-playbook trägt sie),
// also greift hier der Rückfall — und genau der wird gegen go.mod geprüft,
// damit ein Bump dort nicht still eine alte Fassung im Index hinterlässt.
// Meldet das Binary die Fassung selbst, muss sie ebenfalls zu go.mod passen.
func TestKnowledgeGoldmarkVersionPasstZuGoMod(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "go.mod"))
	if err != nil {
		t.Fatalf("go.mod lesen: %v", err)
	}
	want := ""
	for _, line := range strings.Split(string(content), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == knowledgeGoldmarkModule {
			want = fields[1]
		}
	}
	if want == "" {
		t.Fatalf("%s steht nicht in go.mod", knowledgeGoldmarkModule)
	}
	if knowledgeGoldmarkFallbackVersion != want {
		t.Errorf("Rückfall %q, go.mod sagt %q — Konstante nachziehen", knowledgeGoldmarkFallbackVersion, want)
	}
	if knowledgeGoldmarkVersion != want {
		t.Errorf("erkannte Fassung %q, go.mod sagt %q", knowledgeGoldmarkVersion, want)
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, dep := range info.Deps {
			if dep.Path == knowledgeGoldmarkModule && dep.Version != want {
				t.Errorf("Build-Informationen melden %q, go.mod sagt %q", dep.Version, want)
			}
		}
	}
}

// Tokenisierung: Kleinschreibung, Unicode-Buchstaben und -Ziffern, sonst
// nichts.
func TestKnowledgeTokens(t *testing.T) {
	got := strings.Join(knowledgeTokens("Über-Schriften, Go 1.26: `k-playbook`!"), " ")
	if got != "über schriften go 1 26 k playbook" {
		t.Errorf("Tokens = %q", got)
	}
}

// BM25: der Chunk, der den Begriff trägt, steht vorn; Chunks ohne den Begriff
// sind keine Treffer; die Überschrift zählt mit.
func TestKnowledgeBM25FindetUeberschriftUndText(t *testing.T) {
	chunks := []Chunk{
		{Heading: "Release", Text: "Wie ein Release entsteht."},
		{Heading: "Der Weg", Text: "Tag setzen, CI abwarten."},
		{Heading: "Verlinkung", Text: "Symlinks für Claude Code."},
	}
	hits := buildBM25(chunks).search("Symlinks")
	if len(hits) != 1 || hits[0].chunk != 2 {
		t.Errorf("Symlinks: %+v", hits)
	}
	hits = buildBM25(chunks).search("weg")
	if len(hits) != 1 || hits[0].chunk != 1 {
		t.Errorf("Überschrift nicht gewichtet: %+v", hits)
	}
	hits = buildBM25(chunks).search("release entsteht")
	if len(hits) != 1 || hits[0].chunk != 0 {
		t.Errorf("release entsteht: %+v", hits)
	}
	if hits := buildBM25(chunks).search(""); len(hits) != 0 {
		t.Errorf("leere Anfrage: %+v", hits)
	}
}

func jsonKeys(t *testing.T, value any) []string {
	t.Helper()
	content, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	decoded := map[string]json.RawMessage{}
	if err := json.Unmarshal(content, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	keys := make([]string, 0, len(decoded))
	for key := range decoded {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// Der Vertrag eines Treffers: genau diese Felder, kein score.
func TestKnowledgeHitJSONFelder(t *testing.T) {
	keys := jsonKeys(t, Hit{Rank: 1})
	want := []string{"anchor", "excerpt", "heading", "kind", "path", "rank"}
	if strings.Join(keys, ",") != strings.Join(want, ",") {
		t.Errorf("Felder = %v, erwartet %v", keys, want)
	}
	keys = jsonKeys(t, Hit{Rank: 1, Origin: "Sitzung 42", State: "reviewed"})
	if strings.Join(keys, ",") != "anchor,excerpt,heading,kind,origin,path,rank,state" {
		t.Errorf("Felder mit Frontmatter = %v", keys)
	}
	keys = jsonKeys(t, KnowledgeEntry{})
	if strings.Join(keys, ",") != "kind,path,title" {
		t.Errorf("List-Felder = %v", keys)
	}
	keys = jsonKeys(t, KnowledgeEntry{Origin: "o", State: "s"})
	if strings.Join(keys, ",") != "kind,origin,path,state,title" {
		t.Errorf("List-Felder mit Frontmatter = %v", keys)
	}
}

// Der Auszug ist der Anfang des Chunks: an einer Wortgrenze auf höchstens 400
// Zeichen gekürzt, dann „…"; kürzere Texte unverändert und ohne „…".
func TestKnowledgeExcerptKuerzung(t *testing.T) {
	short := "Ein kurzer Absatz mit Umlauten: Größe und Maß."
	if got := knowledgeExcerpt(short); got != short {
		t.Errorf("kurz: %q", got)
	}

	word := "Überschrift "
	long := strings.Repeat(word, 60) // 720 Zeichen, Wortgrenzen alle 12
	got := knowledgeExcerpt(long)
	if !strings.HasSuffix(got, "…") {
		t.Errorf("kein Auslassungszeichen: %q", got)
	}
	body := strings.TrimSuffix(got, "…")
	if n := len([]rune(body)); n > 400 {
		t.Errorf("%d Zeichen vor dem …, erwartet höchstens 400", n)
	}
	if strings.HasSuffix(body, " ") || !strings.HasSuffix(body, "Überschrift") {
		t.Errorf("nicht an einer Wortgrenze geschnitten: %q", body)
	}

	// 401 Zeichen: das Zeichen an Position 400 ist das „b" von „ab", also
	// fällt das ganze Wort und der Schnitt liegt bei 398.
	exact := strings.TrimSpace(strings.Repeat("ab ", 134))
	got = knowledgeExcerpt(exact)
	if n := len([]rune(strings.TrimSuffix(got, "…"))); n != 398 || !strings.HasSuffix(got, "ab…") {
		t.Errorf("401 Zeichen: %d vor dem …, %q", n, got[len(got)-10:])
	}
	if got := knowledgeExcerpt(strings.Repeat("ab ", 133) + "c"); got != strings.Repeat("ab ", 133)+"c" {
		t.Errorf("400 Zeichen müssen ungekürzt bleiben: %q", got)
	}

	if got := knowledgeExcerpt("Zeile eins\n\n  Zeile   zwei\n"); got != "Zeile eins Zeile zwei" {
		t.Errorf("Leerraum: %q", got)
	}
}

// Suche: rank ab 1 in Trefferreihenfolge, der Auszug ohne die
// Überschriftenzeile, Filter nach Herkunft, Standardgrenze zehn.
func TestKnowledgeSearchVertrag(t *testing.T) {
	root := knowledgeFixture(t)
	knowledge := NewKnowledge(root)

	hits, err := knowledge.Search("Symlinks", KnowledgeFilter{}, 0)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("Treffer = %+v", hits)
	}
	hit := hits[0]
	if hit.Rank != 1 || hit.Path != "code/links.md" || hit.Heading != "Verlinkung" || hit.Anchor != "verlinkung" || hit.Kind != "code" || hit.Origin != "" || hit.State != "" {
		t.Errorf("Treffer = %+v", hit)
	}
	if hit.Excerpt != "Wie Symlinks entstehen." {
		t.Errorf("Excerpt = %q — die Überschriftenzeile gehört nicht hinein", hit.Excerpt)
	}

	hits, err = knowledge.Search("Ablauf", KnowledgeFilter{}, 0)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 3 {
		t.Fatalf("Ablauf: %+v", hits)
	}
	for index, hit := range hits {
		if hit.Rank != index+1 {
			t.Errorf("Rank %d an Position %d", hit.Rank, index)
		}
	}

	hits, err = knowledge.Search("Ablauf", KnowledgeFilter{}, 2)
	if err != nil || len(hits) != 2 {
		t.Errorf("limit 2: %d Treffer, %v", len(hits), err)
	}

	if hits, err := knowledge.Search("Ablauf Release", KnowledgeFilter{Kind: "manual"}, 0); err != nil || len(hits) != 1 || hits[0].Kind != "manual" || hits[0].Rank != 1 {
		t.Errorf("Filter manual: %+v, %v", hits, err)
	}
	if hits, err := knowledge.Search("Ablauf", KnowledgeFilter{Kind: "gibt-es-nicht"}, 0); err != nil || len(hits) != 0 {
		t.Errorf("Filter unbekannt: %+v, %v", hits, err)
	}
	if _, err := knowledge.Search("   ", KnowledgeFilter{}, 0); err == nil {
		t.Error("leere Anfrage nicht abgewiesen")
	}

	var many strings.Builder
	many.WriteString("# Viele\n\n")
	for index := 0; index < 12; index++ {
		fmt.Fprintf(&many, "## Abschnitt %d\n\nHier steht Zwölfmal derselbe Begriff.\n\n", index)
	}
	writeKnowledgeFile(t, root, "manual/viele.md", many.String())
	hits, err = knowledge.Search("Zwölfmal", KnowledgeFilter{}, 0)
	if err != nil || len(hits) != 10 {
		t.Errorf("Standardgrenze: %d Treffer, %v", len(hits), err)
	}
	hits, err = knowledge.Search("Zwölfmal", KnowledgeFilter{}, 50)
	if err != nil || len(hits) != 12 || hits[11].Rank != 12 {
		t.Errorf("limit 50: %d Treffer, %v", len(hits), err)
	}
}

// List: path, title nach der ListDocs-Regel, kind, dazu origin und state aus
// dem Frontmatter; README vorn, dann alphabetisch; Filter nach Art.
func TestKnowledgeListVertrag(t *testing.T) {
	root := knowledgeFixture(t)
	writeKnowledgeFile(t, root, "manual/ohne-titel.md", "Nur Text.\n")
	knowledge := NewKnowledge(root)

	entries, err := knowledge.List(KnowledgeFilter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	paths := []string{}
	for _, entry := range entries {
		paths = append(paths, entry.Path)
	}
	want := "README.md,code/links.md,extracted/sitzung.md,findings/notiz.md,libs/goldmark.md,manual/ohne-titel.md,manual/release.md,versions/inventory.md"
	if strings.Join(paths, ",") != want {
		t.Errorf("Reihenfolge = %v", paths)
	}
	titles := map[string]string{}
	kinds := map[string]string{}
	byPath := map[string]KnowledgeEntry{}
	for _, entry := range entries {
		titles[entry.Path] = entry.Title
		kinds[entry.Path] = entry.Kind
		byPath[entry.Path] = entry
	}
	// Die README trägt title: Index im Kopf — der gewinnt vor der Überschrift.
	if titles["README.md"] != "Index" || titles["manual/ohne-titel.md"] != "ohne-titel" || titles["findings/notiz.md"] != "Gelernt" || titles["manual/release.md"] != "Release" {
		t.Errorf("Titel = %v", titles)
	}
	if kinds["README.md"] != "root" || kinds["findings/notiz.md"] != "findings" {
		t.Errorf("Art = %v", kinds)
	}
	if entry := byPath["findings/notiz.md"]; entry.Origin != "Sitzung 42" || entry.State != "reviewed" {
		t.Errorf("Frontmatter in List: %+v", entry)
	}
	if entry := byPath["manual/release.md"]; entry.Origin != "" || entry.State != "" {
		t.Errorf("ohne Kopf: %+v", entry)
	}

	entries, err = knowledge.List(KnowledgeFilter{Kind: "manual"})
	if err != nil || len(entries) != 2 || entries[0].Path != "manual/ohne-titel.md" {
		t.Errorf("Filter manual: %+v, %v", entries, err)
	}
}

// Der Titel in List und im Index folgt einer Regel: der Frontmatter-Titel,
// wenn gesetzt — das Pflichtfeld, das ein Aufrufer bei write angeben musste,
// kommt beim Lesen auch zurück —, sonst die erste Überschrift, ersatzweise der
// Dateiname. Ein Dokument, dessen Rumpf mit „## " beginnt, hieß vorher nach
// seiner Datei, obwohl es einen Titel trägt.
func TestKnowledgeListTitelAusDemFrontmatter(t *testing.T) {
	root := knowledgeFixture(t)
	writeKnowledgeFile(t, root, "findings/wissenstor.md", "---\ntitle: Wie das Wissenstor Drift erkennt\nsubject: Index\norigin: Sitzung 43\nstate: reviewed\nformat: markdown\nupdated: 2026-09-12\n---\n\n## Der Abgleich\n\nHash je Datei.\n")
	writeKnowledgeFile(t, root, "findings/leerer-titel.md", "---\ntitle: \"\"\nstate: reviewed\n---\n\n# Aus der Überschrift\n\nText.\n")
	writeKnowledgeFile(t, root, "manual/nur-rumpf.md", "Kein Kopf, keine Überschrift.\n")

	entries, err := NewKnowledge(root).List(KnowledgeFilter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	titles := map[string]string{}
	for _, entry := range entries {
		titles[entry.Path] = entry.Title
	}
	for rel, want := range map[string]string{
		"findings/wissenstor.md":   "Wie das Wissenstor Drift erkennt",
		"findings/leerer-titel.md": "Aus der Überschrift",
		"manual/release.md":        "Release",
		"manual/nur-rumpf.md":      "nur-rumpf",
		"README.md":                "Index",
	} {
		if titles[rel] != want {
			t.Errorf("%s: Titel = %q, erwartet %q", rel, titles[rel], want)
		}
	}
}

// Read liefert Markdown samt Frontmatter und weist jeden Pfad ab, der nicht
// relativ, nicht im Verzeichnis oder keine Markdown-Datei ist.
func TestKnowledgeReadVertragUndPfadabwehr(t *testing.T) {
	root := knowledgeFixture(t)
	knowledge := NewKnowledge(root)

	content, err := knowledge.Read("versions/inventory.md")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !strings.HasPrefix(content, "---\ntitle: Versionsinventar\n") || !strings.Contains(content, "## Laufzeiten") {
		t.Errorf("Inhalt = %q", content)
	}

	for _, path := range []string{"../x.md", "../../k-playbook.md", filepath.Join(KnowledgeDir(root), "README.md"), "manual/release.txt", "manual", "", "manual/../../data/todos.json"} {
		if _, err := knowledge.Read(path); err == nil {
			t.Errorf("Read(%q) nicht abgewiesen", path)
		}
	}
	if _, err := knowledge.Read("manual/fehlt.md"); err == nil {
		t.Error("fehlende Datei ohne Fehler")
	}
}

// Status: die Felder von morgen heute schon, byKind je Art, builtAt als
// RFC3339. Die README zählt als Datei, gibt aber keine Chunks.
func TestKnowledgeStatusVertrag(t *testing.T) {
	root := knowledgeFixture(t)
	status, err := NewKnowledge(root).Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}

	keys := jsonKeys(t, status)
	want := "builtAt,byKind,chunkCount,dims,fileCount,indexKind,indexVersion,model,stale,staleFiles"
	if strings.Join(keys, ",") != want {
		t.Errorf("Felder = %v", keys)
	}
	if status.IndexKind != "bm25" || status.Model != "" || status.Dims != 0 || status.IndexVersion != KnowledgeIndexVersion {
		t.Errorf("Status = %+v", status)
	}
	if status.FileCount != 7 || status.ChunkCount != 13 {
		t.Errorf("fileCount=%d chunkCount=%d", status.FileCount, status.ChunkCount)
	}
	if status.ByKind["root"] != (KnowledgeKindCount{Files: 1, Chunks: 0}) || status.ByKind["code"] != (KnowledgeKindCount{Files: 1, Chunks: 4}) {
		t.Errorf("byKind = %+v", status.ByKind)
	}
	if len(status.ByKind) != 7 {
		t.Errorf("byKind kennt %d Arten: %+v", len(status.ByKind), status.ByKind)
	}

	content, err := json.Marshal(status)
	if err != nil {
		t.Fatal(err)
	}
	raw := map[string]json.RawMessage{}
	if err := json.Unmarshal(content, &raw); err != nil {
		t.Fatal(err)
	}
	var builtAt string
	if err := json.Unmarshal(raw["builtAt"], &builtAt); err != nil {
		t.Fatalf("builtAt ist kein String: %s", raw["builtAt"])
	}
	parsed, err := time.Parse(time.RFC3339, builtAt)
	if err != nil {
		t.Errorf("builtAt %q ist kein RFC3339: %v", builtAt, err)
	}
	if !parsed.Equal(status.BuiltAt) || time.Since(parsed) > time.Minute {
		t.Errorf("builtAt = %s", builtAt)
	}
	if string(raw["byKind"]) == "null" {
		t.Error("byKind ist null statt Objekt")
	}
}

// Ein leeres knowledge/ ist bis zur Migration der Normalfall, ein fehlendes
// der Zustand jeder bestehenden Installation, bis die Struktur erneut
// angewendet wird. Suche, Liste und Status antworten auf beides leer, nicht
// mit einem Fehler — und ein Rückfall auf docs/ findet nicht statt: was dort
// liegt, bleibt für das Tor unsichtbar.
func TestKnowledgeLeereOderFehlendeZoneAntwortetLeer(t *testing.T) {
	for name, prepare := range map[string]func(t *testing.T, root string){
		"leer": func(t *testing.T, root string) {
			if err := os.MkdirAll(KnowledgeDir(root), 0o755); err != nil {
				t.Fatal(err)
			}
		},
		"fehlt": func(t *testing.T, root string) {},
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.MkdirAll(LocalDir(root), 0o755); err != nil {
				t.Fatal(err)
			}
			// docs/ liegt voll daneben und darf nicht als Rückfall dienen.
			docs := filepath.Join(LocalDir(root), "docs", "manual")
			if err := os.MkdirAll(docs, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(docs, "alt.md"), []byte("# Alt\n\nKennwort aus docs.\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			prepare(t, root)

			knowledge := NewKnowledge(root)
			hits, err := knowledge.Search("Kennwort", KnowledgeFilter{}, 0)
			if err != nil || len(hits) != 0 {
				t.Errorf("Search: %+v, %v", hits, err)
			}
			entries, err := knowledge.List(KnowledgeFilter{})
			if err != nil || len(entries) != 0 {
				t.Errorf("List: %+v, %v", entries, err)
			}
			status, err := knowledge.Status()
			if err != nil {
				t.Fatalf("Status: %v", err)
			}
			if status.FileCount != 0 || status.ChunkCount != 0 || status.Stale {
				t.Errorf("Status = %+v", status)
			}
			if len(knowledge.Notes()) != 0 {
				t.Errorf("Notizen bei leerer Zone: %v", knowledge.Notes())
			}
		})
	}
}

// Ein Projekt ohne Wissensverzeichnis hat einen leeren Status, keinen Fehler.
func TestKnowledgeStatusOhneVerzeichnis(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(LocalDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	status, err := NewKnowledge(root).Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.FileCount != 0 || status.ChunkCount != 0 || len(status.ByKind) != 0 || status.Stale {
		t.Errorf("Status = %+v", status)
	}
}

// denyWrite nimmt einem Verzeichnis das Schreibrecht und gibt es am Ende des
// Tests zurück, damit t.TempDir() aufräumen kann. Greifen die Rechte nicht —
// als root, auf manchen Dateisystemen —, bewiese der Test nichts und wird
// übersprungen.
func denyWrite(t *testing.T, dir string) {
	t.Helper()
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("%s: %v", dir, err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("%s sperren: %v", dir, err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, info.Mode().Perm()) })

	probe := filepath.Join(dir, ".schreibprobe")
	if err := os.WriteFile(probe, []byte("x"), 0o644); err == nil {
		_ = os.Remove(probe)
		t.Skip("das entzogene Schreibrecht greift hier nicht")
	}
}

// denyRead nimmt einer Datei das Leserecht, mit derselben Vorsichtsmaßnahme.
func denyRead(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatalf("%s sperren: %v", path, err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, info.Mode().Perm()) })

	if _, err := os.ReadFile(path); err == nil {
		t.Skip("das entzogene Leserecht greift hier nicht")
	}
}

// Der erste Zugriff legt cache/ an — und dabei die verwaltete .gitignore.
// Ohne sie wäre der abgeleitete Index committierbar: CreateLocal schreibt sie
// nur für ein Verzeichnis, das in seinem eigenen Lauf entsteht, und zieht sie
// später nie nach. Ein Release mit `git add -A` nähme den Chunk-Abzug der
// ganzen Doku mit.
func TestKnowledgeIndexLegtGitignoreFuerCacheAn(t *testing.T) {
	root := knowledgeFixture(t)
	cache := filepath.Join(LocalDir(root), CacheDirName)
	if pathExists(cache) {
		t.Fatalf("%s lag schon vor dem Zugriff da", cache)
	}

	if _, err := NewKnowledge(root).Status(); err != nil {
		t.Fatalf("Status: %v", err)
	}

	ignore := filepath.Join(cache, PrivateIgnoreFile)
	content, err := os.ReadFile(ignore)
	if err != nil {
		t.Fatalf("%s fehlt nach dem Zugriff: %v", ignore, err)
	}
	if string(content) != managedIgnoreContent() {
		t.Errorf("%s = %q, erwartet den verwalteten Inhalt %q", ignore, content, managedIgnoreContent())
	}
}

// Ein vorhandenes cache/ ohne .gitignore bleibt, wie es ist: dass ein Projekt
// das Verzeichnis bewusst öffentlich gestellt hat, dreht kein Zugriff still
// zurück — dieselbe Zurückhaltung wie in CreateLocal.
func TestKnowledgeIndexBringtEntfernteGitignoreNichtZurueck(t *testing.T) {
	root := knowledgeFixture(t)
	cache := filepath.Join(LocalDir(root), CacheDirName)
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatalf("%s anlegen: %v", cache, err)
	}

	if _, err := NewKnowledge(root).Status(); err != nil {
		t.Fatalf("Status: %v", err)
	}

	if pathExists(filepath.Join(cache, PrivateIgnoreFile)) {
		t.Error(".gitignore nachträglich in ein vorhandenes cache/ geschrieben")
	}
}

// Ein nicht beschreibbares cache/ legt die Lesewege nicht still: der
// vollständige Index steht im Speicher, und Search, List und Status antworten
// damit. Geprüft auf allen drei Wegen, auf denen open() zurückschreibt —
// Neubau, Drift, Rücksetzung. Verschluckt wird der Fehlschlag nicht: er steht
// in den Notizen.
func TestKnowledgeIndexUeberlebtGesperrtenCache(t *testing.T) {
	t.Run("Neubau", func(t *testing.T) {
		root := knowledgeFixture(t)
		cache := filepath.Join(LocalDir(root), CacheDirName)
		if err := os.MkdirAll(cache, 0o755); err != nil {
			t.Fatalf("%s anlegen: %v", cache, err)
		}
		denyWrite(t, cache)

		knowledge := NewKnowledge(root)
		hits, err := knowledge.Search("Symlinks", KnowledgeFilter{}, 0)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if len(hits) != 1 || hits[0].Path != "code/links.md" {
			t.Errorf("Treffer trotz vollständigem Index im Speicher: %+v", hits)
		}
		if pathExists(KnowledgeIndexFile(root)) {
			t.Error("Indexdatei trotz gesperrtem cache/ entstanden — der Test misst nichts")
		}
		if len(knowledge.Notes()) == 0 {
			t.Error("der Fehlschlag am Cache wurde still verschluckt")
		}

		entries, err := knowledge.List(KnowledgeFilter{})
		if err != nil || len(entries) != 7 {
			t.Errorf("List: %d Einträge, %v", len(entries), err)
		}
		status, err := NewKnowledge(root).Status()
		if err != nil || status.FileCount != 7 {
			t.Errorf("Status: %+v, %v", status, err)
		}
	})

	t.Run("Drift", func(t *testing.T) {
		root := knowledgeFixture(t)
		if _, err := NewKnowledge(root).Status(); err != nil {
			t.Fatalf("Index bauen: %v", err)
		}
		denyWrite(t, KnowledgeCacheDir(root))

		writeKnowledgeFile(t, root, "manual/release.md", "# Release\n\nJetzt mit Kennwort.\n")
		knowledge := NewKnowledge(root)
		hits, err := knowledge.Search("Kennwort", KnowledgeFilter{}, 0)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if len(hits) != 1 || hits[0].Path != "manual/release.md" {
			t.Errorf("Drift nicht im Speicher ausgeglichen: %+v", hits)
		}
		if len(knowledge.Notes()) == 0 {
			t.Error("der Fehlschlag am Cache wurde still verschluckt")
		}

		// Die Datei trägt weiter den alten Stand — sie ist verwerfbar, die
		// Antwort ist es nicht.
		stored := readIndexFile(t, root)
		for _, chunk := range stored.Chunks {
			if chunk.Path == "manual/release.md" && strings.Contains(chunk.Text, "Kennwort") {
				t.Fatal("die Indexdatei wurde doch geschrieben — der Test misst nichts")
			}
		}
	})

	t.Run("Rücksetzung", func(t *testing.T) {
		root := knowledgeFixture(t)
		if _, err := NewKnowledge(root).Status(); err != nil {
			t.Fatalf("Index bauen: %v", err)
		}
		writeKnowledgeFile(t, root, "manual/release.md", "# Release\n\nEinmal geändert.\n")
		if status, err := NewKnowledge(root).Status(); err != nil || !status.Stale {
			t.Fatalf("Drift fehlt: %+v, %v", status, err)
		}
		denyWrite(t, KnowledgeCacheDir(root))

		knowledge := NewKnowledge(root)
		status, err := knowledge.Status()
		if err != nil {
			t.Fatalf("Status: %v", err)
		}
		if status.Stale || status.StaleFiles != 0 {
			t.Errorf("Rücksetzung nicht ausgeführt: %+v", status)
		}
		if len(knowledge.Notes()) == 0 {
			t.Error("der Fehlschlag am Cache wurde still verschluckt")
		}
		if stored := readIndexFile(t, root); !stored.Stale {
			t.Fatal("die Indexdatei wurde doch geschrieben — der Test misst nichts")
		}
	})
}

// Eine unlesbare Datei fällt aus dem Index, statt alle fünf Werkzeuge
// lahmzulegen — dieselbe Duldung, die scanKnowledgeTree unlesbaren Teilbäumen
// gewährt. Auf dem Drift-Weg zählt sie nicht als Drift: ihr Eintrag wird
// entfernt und gemeldet, stale bleibt dem vorbehalten, was am Index vorbei
// geändert wurde (siehe TestKnowledgeIndexUnlesbareDateiIstKeinDrift).
func TestKnowledgeIndexUeberspringtUnlesbareDatei(t *testing.T) {
	t.Run("Neubau", func(t *testing.T) {
		root := knowledgeFixture(t)
		denyRead(t, filepath.Join(KnowledgeDir(root), "libs", "goldmark.md"))

		knowledge := NewKnowledge(root)
		index, err := knowledge.open()
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		if _, there := index.Files["libs/goldmark.md"]; there {
			t.Error("unlesbare Datei im Index")
		}
		if len(index.Files) != 6 {
			t.Errorf("Files = %d, erwartet 6", len(index.Files))
		}
		if index.Stale || index.StaleFiles != 0 {
			t.Errorf("Neubau als Drift gemeldet: stale=%v staleFiles=%d", index.Stale, index.StaleFiles)
		}
		if notes := strings.Join(knowledge.Notes(), "; "); !strings.Contains(notes, "libs/goldmark.md") {
			t.Errorf("Notizen = %q, erwartet die übersprungene Datei", notes)
		}
		if hits, err := NewKnowledge(root).Search("Symlinks", KnowledgeFilter{}, 0); err != nil || len(hits) != 1 {
			t.Errorf("die übrigen Wege antworten nicht mehr: %+v, %v", hits, err)
		}
	})

	t.Run("Drift", func(t *testing.T) {
		root := knowledgeFixture(t)
		if _, err := NewKnowledge(root).Status(); err != nil {
			t.Fatalf("Index bauen: %v", err)
		}
		denyRead(t, filepath.Join(KnowledgeDir(root), "libs", "goldmark.md"))

		knowledge := NewKnowledge(root)
		status, err := knowledge.Status()
		if err != nil {
			t.Fatalf("Status: %v", err)
		}
		if status.FileCount != 6 {
			t.Errorf("fileCount = %d, erwartet 6", status.FileCount)
		}
		if status.Stale || status.StaleFiles != 0 {
			t.Errorf("stale=%v staleFiles=%d, erwartet false/0 — unlesbar ist nicht „am Tor vorbei“", status.Stale, status.StaleFiles)
		}
		if _, there := status.ByKind["libs"]; there {
			t.Errorf("libs noch gezählt: %+v", status.ByKind)
		}
		if notes := strings.Join(knowledge.Notes(), "; "); !strings.Contains(notes, "libs/goldmark.md") {
			t.Errorf("Notizen = %q, erwartet die übersprungene Datei", notes)
		}
	})
}

// Unlesbar ist nicht „am Tor vorbei": eine Datei, deren Hash nicht gebildet
// werden kann, wird gemeldet, nicht als Drift gezählt. War sie im Index, fällt
// ihr Eintrag beim ersten Hash-Fehler einmal heraus und der Index wird einmal
// geschrieben; danach ist sie für den Abgleich unsichtbar — kein Drift, kein
// weiteres Neuschreiben —, bis sie wieder lesbar ist. Vorher hielt sie stale
// für immer auf true und der Index wurde bei jedem Zugriff neu geschrieben.
// Ihre Rückkehr zählt wie jede neu erscheinende Datei als Drift.
func TestKnowledgeIndexUnlesbareDateiIstKeinDrift(t *testing.T) {
	root := knowledgeFixture(t)
	if _, err := NewKnowledge(root).Status(); err != nil {
		t.Fatalf("Index bauen: %v", err)
	}
	locked := filepath.Join(KnowledgeDir(root), "libs", "goldmark.md")
	denyRead(t, locked)

	var stored []byte
	for round := 1; round <= 3; round++ {
		knowledge := NewKnowledge(root)
		status, err := knowledge.Status()
		if err != nil {
			t.Fatalf("Status %d: %v", round, err)
		}
		if status.Stale || status.StaleFiles != 0 {
			t.Errorf("Status %d: stale=%v staleFiles=%d, erwartet false/0", round, status.Stale, status.StaleFiles)
		}
		if status.FileCount != 6 {
			t.Errorf("Status %d: fileCount = %d, erwartet 6", round, status.FileCount)
		}
		if notes := strings.Join(knowledge.Notes(), "; "); !strings.Contains(notes, "libs/goldmark.md") {
			t.Errorf("Status %d: Notizen = %q, erwartet die unlesbare Datei", round, notes)
		}
		hits, err := NewKnowledge(root).Search("Rendern", KnowledgeFilter{}, 0)
		if err != nil {
			t.Fatalf("Search %d: %v", round, err)
		}
		for _, hit := range hits {
			if hit.Path == "libs/goldmark.md" {
				t.Errorf("Status %d: alter Chunk der unlesbaren Datei ist noch ein Treffer: %+v", round, hit)
			}
		}

		content, err := os.ReadFile(KnowledgeIndexFile(root))
		if err != nil {
			t.Fatalf("Index lesen: %v", err)
		}
		if round > 1 && string(content) != string(stored) {
			t.Errorf("Status %d hat den Index neu geschrieben, obwohl sich nichts geändert hat", round)
		}
		stored = content
	}

	if err := os.Chmod(locked, 0o644); err != nil {
		t.Fatal(err)
	}
	status, err := NewKnowledge(root).Status()
	if err != nil {
		t.Fatalf("Status nach Wiederherstellung: %v", err)
	}
	if status.FileCount != 7 || !status.Stale || status.StaleFiles != 1 {
		t.Errorf("nach Wiederherstellung: %+v, erwartet 7 Dateien und die Rückkehr als Drift", status)
	}
	if hits, err := NewKnowledge(root).Search("Rendern", KnowledgeFilter{}, 0); err != nil || len(hits) != 1 || hits[0].Path != "libs/goldmark.md" {
		t.Errorf("nach Wiederherstellung nicht im Index: %+v, %v", hits, err)
	}
}

// state hat Zähne: ein rohes und ein abgelöstes Dokument fehlen in den
// Treffern, List führt beide mit ihrem state, Read liefert sie. Kein
// Filterargument hebt das auf — das ist Leseseite.
func TestKnowledgeSearchLaesstRawUndSupersededAus(t *testing.T) {
	root := knowledgeFixture(t)
	knowledge := NewKnowledge(root)

	raw := sessionDoc("findings/roh.md")
	raw.State = KnowledgeStateRaw
	raw.Body = "# Roh\n\nDas Rohwort steht nur hier.\n"
	if _, err := knowledge.Write("session", raw, ""); err != nil {
		t.Fatal(err)
	}
	if hits, err := knowledge.Search("Rohwort", KnowledgeFilter{}, 0); err != nil || len(hits) != 0 {
		t.Errorf("rohes Dokument in den Treffern: %+v, %v", hits, err)
	}
	if hits, err := knowledge.Search("Rohwort", KnowledgeFilter{Kind: "findings"}, 0); err != nil || len(hits) != 0 {
		t.Errorf("rohes Dokument trotz Filter in den Treffern: %+v, %v", hits, err)
	}

	// Vorher gefunden, nach supersede nicht mehr — read und list kennen es weiter.
	if hits, err := knowledge.Search("sichern", KnowledgeFilter{}, 0); err != nil || len(hits) != 1 || hits[0].Path != "findings/notiz.md" || hits[0].Origin != "Sitzung 42" || hits[0].State != "reviewed" {
		t.Fatalf("vor supersede: %+v, %v", hits, err)
	}
	if _, err := knowledge.Supersede("findings/notiz.md", "findings/roh.md", "Ersetzt"); err != nil {
		t.Fatal(err)
	}
	if hits, err := knowledge.Search("sichern", KnowledgeFilter{}, 0); err != nil || len(hits) != 0 {
		t.Errorf("abgelöstes Dokument in den Treffern: %+v, %v", hits, err)
	}
	content, err := knowledge.Read("findings/notiz.md")
	if err != nil || !strings.Contains(content, "make sichern") || !strings.Contains(content, "state: superseded") {
		t.Errorf("Read nach supersede: %v", err)
	}
	entries, err := knowledge.List(KnowledgeFilter{Kind: "findings"})
	if err != nil {
		t.Fatal(err)
	}
	states := map[string]string{}
	for _, entry := range entries {
		states[entry.Path] = entry.State
	}
	if states["findings/notiz.md"] != "superseded" || states["findings/roh.md"] != "raw" {
		t.Errorf("List nach supersede: %v", states)
	}

	// Der Status zählt beide weiter — sie stehen im Index, nur nicht in den Treffern.
	status, err := knowledge.Status()
	if err != nil || status.ByKind["findings"].Files != 2 || status.ByKind["findings"].Chunks == 0 {
		t.Errorf("Status: %+v, %v", status, err)
	}
}

// Die README fällt aus dem Suchindex: ihr Text ist kein Treffer, List führt
// sie vorn wie bisher, Read liefert sie.
func TestKnowledgeReadmeFaelltAusDemSuchindex(t *testing.T) {
	root := knowledgeFixture(t)
	knowledge := NewKnowledge(root)

	if hits, err := knowledge.Search("Setup", KnowledgeFilter{}, 0); err != nil || len(hits) != 0 {
		t.Errorf("README in den Treffern: %+v, %v", hits, err)
	}
	if hits, err := knowledge.Search("Tabelle", KnowledgeFilter{Kind: "root"}, 0); err != nil || len(hits) != 0 {
		t.Errorf("README trotz Filter root in den Treffern: %+v, %v", hits, err)
	}
	entries, err := knowledge.List(KnowledgeFilter{})
	if err != nil || len(entries) == 0 || entries[0].Path != "README.md" || entries[0].Kind != "root" {
		t.Errorf("List ohne README vorn: %+v, %v", entries, err)
	}
	if content, err := knowledge.Read("README.md"); err != nil || !strings.Contains(content, "Setup") {
		t.Errorf("Read README: %v", err)
	}
	// Eine README in einem Unterverzeichnis ist ein gewöhnliches Dokument.
	writeKnowledgeFile(t, root, "manual/README.md", "# Handbuch\n\nDas Handbuchwort.\n")
	if hits, err := knowledge.Search("Handbuchwort", KnowledgeFilter{}, 0); err != nil || len(hits) != 1 || hits[0].Path != "manual/README.md" {
		t.Errorf("manual/README.md nicht gefunden: %+v, %v", hits, err)
	}
}
