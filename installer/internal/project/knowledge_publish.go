package project

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/inventory"
	"github.com/kascada/k-playbook/installer/internal/yamllite"
)

// KnowledgePublishResult ist die Antwort von Publish: wie viele Dokumente
// geschrieben und wie viele aus dem vorherigen Stand entfernt wurden.
type KnowledgePublishResult struct {
	Producer string `json:"producer"`
	// Dir ist das getauschte Verzeichnis relativ zu knowledge/, mit
	// Schrägstrich am Ende.
	Dir     string `json:"dir"`
	Written int    `json:"written"`
	Removed int    `json:"removed"`
}

// Publish ist der Weg, auf dem ein Generator schreibt — und der einzige. Er
// übergibt den vollständigen Satz seiner Dokumente; das Werkzeug schreibt sie
// und entfernt, was nicht im Satz ist. Die Pfade der Dokumente sind relativ
// zum Verzeichnis des Generators (code/, libs/, versions/).
//
// Ein leerer Satz — nil oder [] — wird abgewiesen, bevor irgendetwas
// angelegt wird: publish leert nie ein Verzeichnis.
//
// Atomar: alle Dateien entstehen in einem versteckten Verzeichnis neben dem
// Ziel, dann wird getauscht — der vorherige Stand beiseite, der neue an seinen
// Platz, der vorherige danach entfernt. Ein Lauf, der vor dem Tausch stirbt —
// eine ungültige Datei, ein voller Datenträger —, lässt den vorherigen Stand
// vollständig stehen. Ein clear mit anschließenden Einzelschreibungen ließe
// die Ablage für die Dauer des Laufs leer. Das versteckte Verzeichnis sieht
// der Index nie: scanKnowledgeTree überspringt Einträge mit führendem Punkt,
// also ist auch ein liegengebliebenes nach einem Absturz kein Treffer.
//
// Erst der Index, dann der Tausch: der Zugriff gleicht zuerst den Baum ab,
// dann werden alle Einträge des Verzeichnisses ersetzt — die eigene Schreibung
// zählt nicht als Drift.
func (k *Knowledge) Publish(producer string, documents []KnowledgeDocument) (KnowledgePublishResult, error) {
	parsed, err := ParseProducer(producer)
	if err != nil {
		return KnowledgePublishResult{}, err
	}
	if !parsed.IsGenerator() {
		return KnowledgePublishResult{}, InputErrorf("Erzeuger %s veröffentlicht kein Verzeichnis: publish ist den Generatoren %s, %s und %s vorbehalten, alle anderen schreiben einzelne Dokumente über write",
			parsed, ProducerDocsCode, ProducerDocsTools, ProducerInventory)
	}
	dir := parsed.Dirs()[0]
	result := KnowledgePublishResult{Producer: string(parsed), Dir: dir}

	// Der Wächter sitzt hier, nicht nur im Lader der Kommandozeile: hier
	// laufen beide Wege zusammen, und nur hier schützt er auch den
	// MCP-Aufruf, der documents: [] oder ein weggelassenes Feld (nil)
	// durchreicht. Ein Generator, der nichts erzeugt hat, leert sein
	// Verzeichnis nicht — was er hatte, bleibt stehen.
	if len(documents) == 0 {
		return result, InputErrorf("%s bringt kein Dokument mit — ein leerer Satz veröffentlicht nichts; publish leert nie ein Verzeichnis", parsed)
	}

	// Erst alles prüfen, dann irgendetwas schreiben.
	seen := map[string]bool{}
	prepared := make([]KnowledgeDocument, 0, len(documents))
	for _, doc := range documents {
		rel, err := KnowledgeRelPath(doc.Path)
		if err != nil {
			return result, err
		}
		full := dir + rel
		if _, err := KnowledgeRelPath(full); err != nil || !parsed.owns(full) {
			return result, InputErrorf("Ziel %q außerhalb des Erzeugerverzeichnisses: %s veröffentlicht nur unterhalb von %s", doc.Path, parsed, dir)
		}
		if seen[full] {
			return result, InputErrorf("Dokument %q steht zweimal im Satz", full)
		}
		seen[full] = true
		if err := validateKnowledgeDocument(&doc); err != nil {
			return result, fmt.Errorf("%s: %w", full, err)
		}
		doc.Path = full
		prepared = append(prepared, doc)
	}
	sort.Slice(prepared, func(i, j int) bool { return prepared[i].Path < prepared[j].Path })

	index, err := k.open()
	if err != nil {
		return result, err
	}

	root := KnowledgeDir(k.projectDir)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return result, fmt.Errorf("%s anlegen: %w", root, err)
	}
	target := filepath.Join(root, filepath.FromSlash(strings.TrimSuffix(dir, "/")))
	// Das Zwischenverzeichnis wird zum Generatorverzeichnis, also bekommt es
	// dieselben Rechte wie die Unterverzeichnisse darin: 0o755 unter der
	// umask. os.MkdirTemp legte es mit 0o700 an, und der Tausch benannte
	// genau dieses Verzeichnis um — für jeden anderen Benutzer unlesbar.
	staging, err := reserveSibling(root, "."+strings.TrimSuffix(dir, "/")+"-neu-*")
	if err != nil {
		return result, err
	}
	if err := os.Mkdir(staging, 0o755); err != nil {
		return result, fmt.Errorf("Zwischenverzeichnis anlegen: %w", err)
	}
	abort := func(err error) (KnowledgePublishResult, error) {
		os.RemoveAll(staging)
		return result, err
	}

	updated := knowledgeUpdatedNow()
	for _, doc := range prepared {
		full := filepath.Join(staging, filepath.FromSlash(strings.TrimPrefix(doc.Path, dir)))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return abort(fmt.Errorf("%s anlegen: %w", filepath.Dir(full), err))
		}
		if err := os.WriteFile(full, []byte(composeKnowledgeFile(doc, nil, updated)), 0o644); err != nil {
			return abort(fmt.Errorf("%s schreiben: %w", doc.Path, err))
		}
	}

	previous, err := scanKnowledgeTree(target)
	if err != nil {
		return abort(err)
	}
	for _, rel := range previous {
		if !seen[dir+rel] {
			result.Removed++
		}
	}

	// Der Tausch. Ein Fehlschlag im zweiten Schritt stellt den vorherigen
	// Stand zurück — sonst stünde die Ablage genau hier leer.
	retired := ""
	if pathExists(target) {
		retired, err = reserveSibling(root, "."+strings.TrimSuffix(dir, "/")+"-alt-*")
		if err != nil {
			return abort(err)
		}
		if err := os.Rename(target, retired); err != nil {
			return abort(fmt.Errorf("%s beiseitestellen: %w", dir, err))
		}
	}
	if err := os.Rename(staging, target); err != nil {
		if retired != "" {
			_ = os.Rename(retired, target)
		}
		return abort(fmt.Errorf("%s einsetzen: %w", dir, err))
	}
	if retired != "" {
		if err := os.RemoveAll(retired); err != nil {
			// Der neue Stand steht; das alte Verzeichnis ist versteckt und
			// für den Index unsichtbar. Gemeldet wird es trotzdem.
			k.note("vorheriger Stand von %s nicht entfernt: %v", dir, err)
		}
	}

	for rel := range index.Files {
		if strings.HasPrefix(rel, dir) {
			index.removeFile(rel)
		}
	}
	for _, doc := range prepared {
		entry, chunks, err := chunkKnowledgeFile(root, doc.Path)
		if err != nil {
			return result, err
		}
		index.replaceFile(doc.Path, entry, chunks)
		result.Written++
	}
	index.BuiltAt = knowledgeNow()
	if err := writeKnowledgeIndex(k.projectDir, index); err != nil {
		return result, err
	}
	return result, nil
}

// reserveSibling liefert einen freien, versteckten Namen im Verzeichnis — für
// das Zwischenverzeichnis, das mit eigenen Rechten neu angelegt wird, und für
// den Ausweichnamen, an den ein Rename gehen kann. MkdirTemp findet den Namen;
// das Verzeichnis selbst wird gleich wieder entfernt, damit Mkdir oder Rename
// es besetzen kann.
func reserveSibling(root string, pattern string) (string, error) {
	name, err := os.MkdirTemp(root, pattern)
	if err != nil {
		return "", fmt.Errorf("Ausweichnamen anlegen: %w", err)
	}
	if err := os.Remove(name); err != nil {
		return "", fmt.Errorf("Ausweichnamen freigeben: %w", err)
	}
	return name, nil
}

// ParseKnowledgeDocumentFile liest ein Dokument, wie `publish --from <dir>`
// es vorfindet: Frontmatter mit den Feldern von write, danach der Rumpf. Das
// Werkzeug prüft die Felder und setzt den Kopf neu zusammen — an der Prüfung
// vorbei wird keine Datei durchgereicht, und updated setzt es selbst. rel ist
// der Pfad der Datei relativ zum gelesenen Verzeichnis und wird zum Pfad des
// Dokuments.
func ParseKnowledgeDocumentFile(rel string, data []byte) (KnowledgeDocument, error) {
	block, ok := inventory.FrontmatterBlock(data)
	if !ok {
		return KnowledgeDocument{}, fmt.Errorf("%s: kein Frontmatter — erwartet werden title, subject, origin und state im Kopf", rel)
	}
	head, err := yamllite.Parse([]byte(block))
	if err != nil {
		return KnowledgeDocument{}, fmt.Errorf("%s: Frontmatter nicht lesbar: %w", rel, err)
	}
	if head == nil || head.Kind != yamllite.Mapping {
		return KnowledgeDocument{}, fmt.Errorf("%s: Frontmatter ist keine Abbildung", rel)
	}
	doc := KnowledgeDocument{
		Path:    filepath.ToSlash(rel),
		Title:   strings.TrimSpace(head.Get("title").Str()),
		Subject: strings.TrimSpace(head.Get("subject").Str()),
		Origin:  strings.TrimSpace(head.Get("origin").Str()),
		State:   strings.TrimSpace(head.Get("state").Str()),
		Format:  strings.TrimSpace(head.Get("format").Str()),
		Body:    string(inventory.Body(data)),
	}
	for _, item := range head.Get("sources").List() {
		doc.Sources = append(doc.Sources, item.Str())
	}
	return doc, nil
}

// LoadKnowledgeDocuments liest alle Markdown-Dateien unterhalb eines
// Verzeichnisses als Dokumente für Publish; die Pfade sind relativ zu dir.
// Versteckte Einträge werden übersprungen wie im Index. Ein Verzeichnis ohne
// eine einzige Datei ist ein Fehler: ein Generator, der nichts erzeugt hat,
// soll nicht still das ganze Verzeichnis leeren.
func LoadKnowledgeDocuments(dir string) ([]KnowledgeDocument, error) {
	if !isDir(dir) {
		return nil, fmt.Errorf("%s ist kein Verzeichnis", dir)
	}
	documents := []KnowledgeDocument{}
	err := filepath.WalkDir(dir, func(full string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if full != dir && strings.HasPrefix(entry.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(entry.Name(), ".") || !strings.EqualFold(path.Ext(entry.Name()), ".md") {
			return nil
		}
		rel, err := filepath.Rel(dir, full)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(full)
		if err != nil {
			return fmt.Errorf("%s lesen: %w", rel, err)
		}
		doc, err := ParseKnowledgeDocumentFile(rel, data)
		if err != nil {
			return err
		}
		documents = append(documents, doc)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(documents) == 0 {
		return nil, fmt.Errorf("%s enthält keine Markdown-Datei — ein leerer Satz veröffentlicht nichts", dir)
	}
	return documents, nil
}

// Die Felder, die Supersede setzt. successor und superseded_reason sind
// Felder des Frontmatter-Vertrags (docs/knowledge-layout.md).
const (
	knowledgeFieldState      = "state"
	knowledgeFieldSuccessor  = "successor"
	knowledgeFieldSupersedes = "superseded_reason"
	knowledgeFieldUpdated    = "updated"
)

// Supersede löst ein Dokument ab: state wird superseded, der Nachfolger und
// der Grund kommen ins Frontmatter, updated wird neu gesetzt. Der Rumpf bleibt
// unverändert, gelöscht wird nichts — eine Ablage, die löscht, kann nicht
// sagen, was sie früher behauptet hat. Der Nachfolger muss in der Ablage
// liegen: erst schreiben, dann ablösen; ein Nachfolger, den es nicht gibt,
// wäre ein Verweis ins Leere, und genau der soll am Aufruf auffallen.
//
// Das Ergebnis ist der bereinigte Pfad des abgelösten Dokuments.
func (k *Knowledge) Supersede(docPath string, successor string, reason string) (string, error) {
	rel, err := KnowledgeRelPath(docPath)
	if err != nil {
		return "", err
	}
	next, err := KnowledgeRelPath(successor)
	if err != nil {
		return "", InputErrorf("Nachfolger: %w", err)
	}
	if next == rel {
		return "", InputErrorf("ein Dokument kann nicht sein eigener Nachfolger sein: %s", rel)
	}
	if err := requireLine("reason", reason); err != nil {
		return "", err
	}
	root := KnowledgeDir(k.projectDir)
	full := filepath.Join(root, filepath.FromSlash(rel))
	if !fileExists(full) {
		return "", InputErrorf("Dokument %s gibt es nicht", rel)
	}
	if !fileExists(filepath.Join(root, filepath.FromSlash(next))) {
		return "", InputErrorf("Nachfolger %s gibt es nicht — erst schreiben, dann ablösen", next)
	}

	data, err := os.ReadFile(full)
	if err != nil {
		return "", fmt.Errorf("%s lesen: %w", rel, err)
	}
	updated := supersedeFrontmatter(string(data), [][2]string{
		{knowledgeFieldState, KnowledgeStateSuperseded},
		{knowledgeFieldSuccessor, next},
		{knowledgeFieldSupersedes, strings.TrimSpace(reason)},
		{knowledgeFieldUpdated, knowledgeUpdatedNow()},
	})

	index, err := k.open()
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(full, []byte(updated), 0o644); err != nil {
		return "", fmt.Errorf("%s schreiben: %w", rel, err)
	}
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

// supersedeFrontmatter setzt Felder im Kopf einer Datei: vorhandene Schlüssel
// werden an Ort und Stelle ersetzt, fehlende vor dem schließenden „---"
// ergänzt, der Rumpf bleibt Byte für Byte. Eine Datei ohne Kopf — an der
// Prüfung vorbei entstanden — bekommt einen vorangestellt; ihr Inhalt ist
// dann der Rumpf.
func supersedeFrontmatter(content string, fields [][2]string) string {
	text := strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(text, "\n")
	block, ok := inventory.FrontmatterBlock([]byte(text))
	if !ok || !isKnowledgeFrontmatter(block) {
		var head strings.Builder
		head.WriteString("---\n")
		for _, field := range fields {
			head.WriteString(field[0] + ": " + yamlScalar(field[1]) + "\n")
		}
		head.WriteString("---\n\n")
		return head.String() + strings.TrimLeft(text, "\n")
	}

	end := 1
	for end < len(lines) && strings.TrimSpace(lines[end]) != "---" {
		end++
	}
	done := map[string]bool{}
	// Fehlende Felder kommen vor updated, damit die Reihenfolge von write
	// erhalten bleibt: updated steht zuletzt. Fehlt auch updated, vor das
	// schließende „---".
	insertAt := end
	for position := 1; position < end; position++ {
		line := lines[position]
		if line == "" || line[0] == ' ' || line[0] == '\t' || line[0] == '-' || line[0] == '#' {
			continue
		}
		key, _, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		if key == knowledgeFieldUpdated {
			insertAt = position
		}
		for _, field := range fields {
			if key == field[0] {
				lines[position] = field[0] + ": " + yamlScalar(field[1])
				done[key] = true
			}
		}
	}
	missing := []string{}
	for _, field := range fields {
		if !done[field[0]] {
			missing = append(missing, field[0]+": "+yamlScalar(field[1]))
		}
	}
	if len(missing) > 0 {
		rest := append([]string(nil), lines[insertAt:]...)
		lines = append(append(lines[:insertAt], missing...), rest...)
	}
	return strings.Join(lines, "\n")
}
