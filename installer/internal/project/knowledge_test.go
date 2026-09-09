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

	"github.com/kascada/k-playbook/installer/internal/inventory"
)

// knowledgeFixture legt ein Projekt mit einem Wissensverzeichnis an, das alle
// fünf Herkunftsordner, learned/ und eine flache Wurzeldatei trägt, und gibt
// das Hauptverzeichnis zurück.
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
		"versions/inventory.md": "---\ntitle: Versionsinventar\n---\n\n# Versionsinventar\n\n## Laufzeiten\n\nGo 1.26.\n",
		"manual/release.md":     "# Release\n\nWie ein Release entsteht.\n\n## Der Weg\n\nTag setzen, CI abwarten.\n",
		"learned/notiz.md":      "---\nsource: sitzung 42\n---\n\n# Gelernt\n\nDer Befehl war make sichern.\n",
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

// Die Herkunft ist das erste Pfadsegment; eine Datei flach in der Wurzel
// bekommt ausdrücklich root — der Leerwert wäre ein Zufall, kein Vertrag.
func TestKnowledgeSourceJeHerkunftsordner(t *testing.T) {
	root := knowledgeFixture(t)

	want := map[string]string{
		"README.md":             "root",
		"code/links.md":         "code",
		"libs/goldmark.md":      "libs",
		"extracted/sitzung.md":  "extracted",
		"versions/inventory.md": "versions",
		"manual/release.md":     "manual",
		"learned/notiz.md":      "learned",
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
		if entry.Source != want[rel] {
			t.Errorf("%s: Source = %q, erwartet %q", rel, entry.Source, want[rel])
		}
		if len(chunks) == 0 {
			t.Errorf("%s: keine Chunks", rel)
		}
		for _, chunk := range chunks {
			if chunk.Source != want[rel] || chunk.Path != rel {
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

// Frontmatter ist Metadatum, kein Text: title und description landen am
// Eintrag, in keinem Chunk steht „title:".
func TestKnowledgeFrontmatterBleibtAusDemText(t *testing.T) {
	root := knowledgeFixture(t)
	entry, chunks := chunkFixtureFile(t, root, "README.md")

	if entry.Frontmatter.Title != "Index" || entry.Frontmatter.Description != "Der Einstieg" {
		t.Errorf("Frontmatter = %+v", entry.Frontmatter)
	}
	if entry.Title != "k-playbook – Dokumentation" {
		t.Errorf("Title = %q", entry.Title)
	}
	for _, chunk := range chunks {
		if strings.Contains(chunk.Text, "title:") || strings.Contains(chunk.Text, "---") {
			t.Errorf("Frontmatter im Chunk: %q", chunk.Text)
		}
	}
	if len(chunks) != 2 || chunks[0].Heading != "k-playbook – Dokumentation" || chunks[0].Text != "Diese Dokumentation beschreibt das Setup." {
		t.Errorf("Chunks = %+v", chunks)
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

// Kaputtes JSON und eine fremde Fassung führen kommentarlos zum Neubau.
func TestKnowledgeIndexNeubauBeiKaputterOderFremderDatei(t *testing.T) {
	for name, content := range map[string]string{
		"kaputt":          "{ das ist kein JSON",
		"indexVersion":    `{"indexVersion": 999, "goldmarkVersion": "` + knowledgeGoldmarkVersion + `", "stale": true, "staleFiles": 5, "files": {}, "chunks": []}`,
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
	want := []string{"anchor", "excerpt", "heading", "path", "rank", "source"}
	if strings.Join(keys, ",") != strings.Join(want, ",") {
		t.Errorf("Felder = %v, erwartet %v", keys, want)
	}
	keys = jsonKeys(t, KnowledgeEntry{})
	if strings.Join(keys, ",") != "path,source,title" {
		t.Errorf("List-Felder = %v", keys)
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
	if hit.Rank != 1 || hit.Path != "code/links.md" || hit.Heading != "Verlinkung" || hit.Anchor != "verlinkung" || hit.Source != "code" {
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

	if hits, err := knowledge.Search("Ablauf Release", KnowledgeFilter{Source: "manual"}, 0); err != nil || len(hits) != 1 || hits[0].Source != "manual" || hits[0].Rank != 1 {
		t.Errorf("Filter manual: %+v, %v", hits, err)
	}
	if hits, err := knowledge.Search("Ablauf", KnowledgeFilter{Source: "gibt-es-nicht"}, 0); err != nil || len(hits) != 0 {
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

// List: path, title nach der ListDocs-Regel, source; README vorn, dann
// alphabetisch; Filter nach Herkunft.
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
	want := "README.md,code/links.md,extracted/sitzung.md,learned/notiz.md,libs/goldmark.md,manual/ohne-titel.md,manual/release.md,versions/inventory.md"
	if strings.Join(paths, ",") != want {
		t.Errorf("Reihenfolge = %v", paths)
	}
	titles := map[string]string{}
	sources := map[string]string{}
	for _, entry := range entries {
		titles[entry.Path] = entry.Title
		sources[entry.Path] = entry.Source
	}
	if titles["README.md"] != "k-playbook – Dokumentation" || titles["manual/ohne-titel.md"] != "ohne-titel" || titles["learned/notiz.md"] != "Gelernt" {
		t.Errorf("Titel = %v", titles)
	}
	if sources["README.md"] != "root" || sources["learned/notiz.md"] != "learned" {
		t.Errorf("Herkunft = %v", sources)
	}

	entries, err = knowledge.List(KnowledgeFilter{Source: "manual"})
	if err != nil || len(entries) != 2 || entries[0].Path != "manual/ohne-titel.md" {
		t.Errorf("Filter manual: %+v, %v", entries, err)
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

func readLearned(t *testing.T, root, rel string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(KnowledgeLearnedDir(root), filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("%s lesen: %v", rel, err)
	}
	return string(content)
}

// Write schreibt nur unterhalb von learned/, legt Ordner an, setzt source ins
// Frontmatter und aktualisiert die Chunks, ohne dass das als Drift zählt.
func TestKnowledgeWriteVertrag(t *testing.T) {
	root := knowledgeFixture(t)
	knowledge := NewKnowledge(root)

	for _, path := range []string{"../manual/x.md", "../../x.md", filepath.Join(root, "x.md"), "notiz.txt", "", "tief/../../x.md"} {
		if rel, err := knowledge.Write(path, "# X\n", "sitzung"); err == nil || rel != "" {
			t.Errorf("Write(%q) nicht abgewiesen: %q, %v", path, rel, err)
		}
	}
	if _, err := knowledge.Write("x.md", "# X\n", "  "); err == nil {
		t.Error("leeres source nicht abgewiesen")
	}
	if _, err := knowledge.Write("x.md", "# X\n", "zwei\nzeilen"); err == nil {
		t.Error("mehrzeiliges source nicht abgewiesen")
	}
	if _, err := os.Stat(filepath.Join(KnowledgeDir(root), "manual", "x.md")); err == nil {
		t.Error("Ausbruch nach manual/ hat geschrieben")
	}

	// Ohne Frontmatter: Block voranstellen.
	if rel, err := knowledge.Write("sitzung/befund.md", "# Befund\n\nDer Wächter prüft VERSION.", "sitzung 42"); err != nil || rel != "learned/sitzung/befund.md" {
		t.Fatalf("Write: %q, %v", rel, err)
	}
	if got := readLearned(t, root, "sitzung/befund.md"); got != "---\nsource: sitzung 42\n---\n\n# Befund\n\nDer Wächter prüft VERSION.\n" {
		t.Errorf("ohne Frontmatter: %q", got)
	}

	// Mit Frontmatter ohne source: Feld ergänzen, title nicht anfassen.
	if _, err := knowledge.Write("mit.md", "---\ntitle: Mit\ndescription: Beschreibung\n---\n\n# Mit\n\nText.\n", "extern"); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if got := readLearned(t, root, "mit.md"); got != "---\ntitle: Mit\ndescription: Beschreibung\nsource: extern\n---\n\n# Mit\n\nText.\n" {
		t.Errorf("Feld ergänzen: %q", got)
	}

	// Mit source: ersetzen.
	if _, err := knowledge.Write("ersetzt.md", "---\nsource: alt\ntitle: E\n---\n# E\n", "neu"); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if got := readLearned(t, root, "ersetzt.md"); got != "---\nsource: neu\ntitle: E\n---\n# E\n" {
		t.Errorf("Feld ersetzen: %q", got)
	}

	// Die Chunks sind da, und die eigene Schreibung ist keine Drift.
	hits, err := knowledge.Search("Wächter", KnowledgeFilter{Source: "learned"}, 0)
	if err != nil || len(hits) != 1 || hits[0].Path != "learned/sitzung/befund.md" || hits[0].Heading != "Befund" {
		t.Errorf("Treffer nach Write: %+v, %v", hits, err)
	}
	status, err := knowledge.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.Stale || status.StaleFiles != 0 {
		t.Errorf("Write als Drift gemeldet: %+v", status)
	}
	if status.BySource["learned"].Files != 4 {
		t.Errorf("learned = %+v", status.BySource["learned"])
	}

	// Überschreiben ersetzt die Chunks, statt sie zu verdoppeln.
	if _, err := knowledge.Write("sitzung/befund.md", "# Befund\n\nJetzt ohne den Begriff.\n", "sitzung 43"); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if hits, err := knowledge.Search("Wächter", KnowledgeFilter{}, 0); err != nil || len(hits) != 0 {
		t.Errorf("alter Chunk geblieben: %+v, %v", hits, err)
	}
}

// Status: die Felder von morgen heute schon, bySource je Herkunft, builtAt als
// RFC3339.
func TestKnowledgeStatusVertrag(t *testing.T) {
	root := knowledgeFixture(t)
	status, err := NewKnowledge(root).Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}

	keys := jsonKeys(t, status)
	want := "builtAt,bySource,chunkCount,dims,fileCount,indexKind,indexVersion,model,stale,staleFiles"
	if strings.Join(keys, ",") != want {
		t.Errorf("Felder = %v", keys)
	}
	if status.IndexKind != "bm25" || status.Model != "" || status.Dims != 0 || status.IndexVersion != KnowledgeIndexVersion {
		t.Errorf("Status = %+v", status)
	}
	if status.FileCount != 7 || status.ChunkCount != 15 {
		t.Errorf("fileCount=%d chunkCount=%d", status.FileCount, status.ChunkCount)
	}
	if status.BySource["root"] != (KnowledgeSourceCount{Files: 1, Chunks: 2}) || status.BySource["code"] != (KnowledgeSourceCount{Files: 1, Chunks: 4}) {
		t.Errorf("bySource = %+v", status.BySource)
	}
	if len(status.BySource) != 7 {
		t.Errorf("bySource kennt %d Herkünfte: %+v", len(status.BySource), status.BySource)
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
	if string(raw["bySource"]) == "null" {
		t.Error("bySource ist null statt Objekt")
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
	if status.FileCount != 0 || status.ChunkCount != 0 || len(status.BySource) != 0 || status.Stale {
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
// gewährt. Auf dem Drift-Weg zählt sie als Drift.
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
		if !status.Stale || status.StaleFiles != 1 {
			t.Errorf("stale=%v staleFiles=%d, erwartet true/1", status.Stale, status.StaleFiles)
		}
		if _, there := status.BySource["libs"]; there {
			t.Errorf("libs noch gezählt: %+v", status.BySource)
		}
		if notes := strings.Join(knowledge.Notes(), "; "); !strings.Contains(notes, "libs/goldmark.md") {
			t.Errorf("Notizen = %q, erwartet die übersprungene Datei", notes)
		}
	})
}

// Ein führender Thematic Break ist kein Frontmatter. Der Vermerk kommt davor,
// statt vor das nächste „---" mitten im Dokument — sonst schnitte
// inventory.Body() Titel und ersten Absatz weg, und list und search
// widersprächen sich: die Liste nennte den Titel, die Suche fände den Text
// darunter nicht mehr.
func TestKnowledgeWriteFuehrenderThematicBreak(t *testing.T) {
	root := knowledgeFixture(t)
	knowledge := NewKnowledge(root)

	content := "---\n\n# Titel\n\nAbsatz mit Kennwort.\n\n---\n\nRest nach dem zweiten Strich.\n"
	rel, err := knowledge.Write("bruch.md", content, "sitzung 44")
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	got := readLearned(t, root, "bruch.md")
	if want := "---\nsource: sitzung 44\n---\n\n" + content; got != want {
		t.Errorf("geschrieben:\n%q\nerwartet:\n%q", got, want)
	}
	body := string(inventory.Body([]byte(got)))
	if !strings.Contains(body, "# Titel") || !strings.Contains(body, "Absatz mit Kennwort.") {
		t.Errorf("Rumpf hat Titel oder Absatz verloren: %q", body)
	}

	// list und search sehen danach dasselbe Dokument.
	entries, err := knowledge.List(KnowledgeFilter{Source: KnowledgeLearnedDirName})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	title := ""
	for _, entry := range entries {
		if entry.Path == rel {
			title = entry.Title
		}
	}
	if title != "Titel" {
		t.Errorf("List meldet Titel %q für %s", title, rel)
	}
	hits, err := knowledge.Search("Kennwort", KnowledgeFilter{}, 0)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 1 || hits[0].Path != rel || hits[0].Heading != title {
		t.Errorf("Search: %+v — erwartet einen Treffer in %s unter %q", hits, rel, title)
	}
}

// Leerraum gehört nicht zum Schlüssel: „source :" und „ source:" sind dasselbe
// Feld, und ein zweites daneben ergäbe einen doppelten Schlüssel im Kopf.
func TestKnowledgeWriteSchluesselMitLeerraum(t *testing.T) {
	root := knowledgeFixture(t)
	knowledge := NewKnowledge(root)

	for name, content := range map[string]string{
		"Leerzeichen vor dem Doppelpunkt": "---\nsource : alt\ntitle: E\n---\n\n# E\n",
		"eingerückter Schlüssel":          "---\n source: alt\ntitle: E\n---\n\n# E\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := knowledge.Write("schluessel.md", content, "neu"); err != nil {
				t.Fatalf("Write: %v", err)
			}
			got := readLearned(t, root, "schluessel.md")
			if strings.Count(got, "source") != 1 {
				t.Errorf("doppelter Schlüssel: %q", got)
			}
			if got != "---\nsource: neu\ntitle: E\n---\n\n# E\n" {
				t.Errorf("geschrieben: %q", got)
			}
		})
	}
}

// Write meldet den Ort, den es selbst berechnet hat, und genau dieser Ort ist
// der, unter dem Read, List und Search die Datei kennen. Nachgebaut wird er
// nirgends: zwei Rechnungen für denselben Pfad wären zwei Chancen, ihn
// verschieden zu buchstabieren.
func TestKnowledgeWriteMeldetDenEigenenPfad(t *testing.T) {
	root := knowledgeFixture(t)
	knowledge := NewKnowledge(root)

	for _, tc := range []struct{ path, want string }{
		{"notiz.md", "learned/notiz.md"},
		{"./notiz.md", "learned/notiz.md"},
		{"tief/unten/../notiz.md", "learned/tief/notiz.md"},
	} {
		rel, err := knowledge.Write(tc.path, "# Notiz\n\nInhalt.\n", "sitzung")
		if err != nil {
			t.Fatalf("Write(%q): %v", tc.path, err)
		}
		if rel != tc.want {
			t.Errorf("Write(%q) = %q, erwartet %q", tc.path, rel, tc.want)
		}
		if _, err := knowledge.Read(rel); err != nil {
			t.Errorf("Read(%q): %v", rel, err)
		}
		entries, err := knowledge.List(KnowledgeFilter{Source: KnowledgeLearnedDirName})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		found := false
		for _, entry := range entries {
			found = found || entry.Path == rel
		}
		if !found {
			t.Errorf("List kennt %q nicht: %+v", rel, entries)
		}
	}
}
