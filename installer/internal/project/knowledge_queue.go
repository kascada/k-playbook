package project

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/kascada/k-playbook/installer/internal/inventory"
	"github.com/kascada/k-playbook/installer/internal/yamllite"
)

// QueueDir ist die Warteschlange eines Projekts.
func QueueDir(projectDir string) string {
	return filepath.Join(LocalDir(projectDir), QueueDirName)
}

// QueueEntry ist ein Stück offener Arbeit: dieses Rohstück soll Wissen werden.
// Das Dateiformat dahinter — queue/<id>.md mit den vier Feldern im Kopf und
// Notizen im Rumpf — ist Innenleben und darf sich ändern, solange die
// Argumente von QueueAdd gleich bleiben.
type QueueEntry struct {
	ID string `json:"id"`
	// Origin ist ein Eingangspfad oder eine Adresse draußen.
	Origin string `json:"origin"`
	// Target ist das Zielverzeichnis relativ zu knowledge/, mit Schrägstrich.
	Target string `json:"target"`
	Reason string `json:"reason"`
	// Added ist der Zeitpunkt des Eintrags, RFC3339.
	Added time.Time `json:"added"`
	// Notes ist der Rumpf: was ein Lauf dazu festgehalten hat.
	Notes string `json:"notes,omitempty"`
}

// queueNow ist der Zeitstempel eines Eintrags — als Variable, damit ein Test
// ihn festhalten kann.
var queueNow = func() time.Time { return time.Now().Truncate(time.Second) }

// queueEntryFile ist die Datei eines Queue-Eintrags: queue/<id>.md. Die
// Kennung ist ein einzelner Dateiname ohne Endung — kein Pfad, nicht
// versteckt, nicht die README des Verzeichnisses.
func queueEntryFile(projectDir string, id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", InputErrorf("keine Queue-Kennung angegeben")
	}
	if !isCleanPathSegment(id) || strings.HasSuffix(strings.ToLower(id), ".md") || strings.EqualFold(id, "README") {
		return "", InputErrorf("Queue-Kennung %q ist kein gültiger Eintragsname", id)
	}
	return filepath.Join(QueueDir(projectDir), id+".md"), nil
}

// queueTarget prüft das Zielverzeichnis: relativ zu knowledge/, kein
// Ausbruch, nicht versteckt, nicht die Wurzel selbst. Gegen die
// Erzeugertabelle wird nicht geprüft — welcher Erzeuger die Übernahme
// schreibt, entscheidet erst der Lauf, und der nennt sich dann bei write.
func queueTarget(target string) (string, error) {
	trimmed := strings.TrimSpace(target)
	if trimmed == "" {
		return "", InputErrorf("kein target angegeben — erwartet wird ein Verzeichnis relativ zu %s/%s/, etwa extracted/", LocalDirName, KnowledgeDirName)
	}
	if filepath.IsAbs(trimmed) || strings.HasPrefix(trimmed, "/") || strings.HasPrefix(trimmed, "\\") {
		return "", InputErrorf("target %q führt aus %s/%s/ heraus: nur relative Pfade", target, LocalDirName, KnowledgeDirName)
	}
	cleaned := path.Clean(filepath.ToSlash(trimmed))
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", InputErrorf("target %q führt aus %s/%s/ heraus oder meint die Wurzel — erwartet wird ein Verzeichnis darunter", target, LocalDirName, KnowledgeDirName)
	}
	for _, segment := range strings.Split(cleaned, "/") {
		if strings.HasPrefix(segment, ".") {
			return "", InputErrorf("target %q: versteckte Einträge (%q) gehören nicht in die Wissensablage", target, segment)
		}
	}
	return cleaned + "/", nil
}

// queueSlug macht aus der Herkunft den lesbaren Teil der Kennung: Buchstaben
// und Ziffern bleiben, alles andere wird ein Strich, höchstens 40 Zeichen.
func queueSlug(origin string) string {
	var slug strings.Builder
	dash := false
	for _, r := range strings.ToLower(origin) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if r > unicode.MaxASCII {
				// Umlaute und Sonderbuchstaben fallen: die Kennung wird ein
				// Dateiname, und der soll überall gleich heißen.
				continue
			}
			slug.WriteRune(r)
			dash = false
			continue
		}
		if slug.Len() > 0 && !dash {
			slug.WriteByte('-')
			dash = true
		}
	}
	result := strings.Trim(slug.String(), "-")
	if len(result) > 40 {
		result = strings.Trim(result[:40], "-")
	}
	if result == "" {
		return "eintrag"
	}
	return result
}

// QueueAdd legt einen Eintrag an und vergibt die Kennung selbst: Zeitstempel
// plus ein Slug aus der Herkunft, bei Kollision mit laufender Nummer. Das
// Ergebnis ist die Kennung.
func (k *Knowledge) QueueAdd(origin string, target string, reason string) (string, error) {
	if err := requireLine("origin", origin); err != nil {
		return "", err
	}
	cleanedTarget, err := queueTarget(target)
	if err != nil {
		return "", err
	}
	if err := requireLine("reason", reason); err != nil {
		return "", err
	}
	origin, reason = strings.TrimSpace(origin), strings.TrimSpace(reason)

	dir := QueueDir(k.projectDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("%s anlegen: %w", dir, err)
	}
	now := queueNow()
	base := now.Format("20060102-150405") + "-" + queueSlug(origin)
	id := base
	for attempt := 2; pathExists(filepath.Join(dir, id+".md")); attempt++ {
		id = fmt.Sprintf("%s-%d", base, attempt)
	}

	content := "---\n" +
		"origin: " + yamlScalar(origin) + "\n" +
		"target: " + yamlScalar(cleanedTarget) + "\n" +
		"reason: " + yamlScalar(reason) + "\n" +
		"added: " + yamlScalar(now.Format(time.RFC3339)) + "\n" +
		"---\n"
	if err := os.WriteFile(filepath.Join(dir, id+".md"), []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("Queue-Eintrag %s schreiben: %w", id, err)
	}
	return id, nil
}

// QueueList nennt den Rückstand, älteste Kennung zuerst — die Kennung beginnt
// mit dem Zeitstempel, also ist alphabetisch chronologisch. Eine fehlende
// Warteschlange ist leer, kein Fehler; eine Datei, die kein Eintrag ist, wird
// mit Notiz übersprungen statt die Liste zu verhindern.
func (k *Knowledge) QueueList() ([]QueueEntry, error) {
	dir := QueueDir(k.projectDir)
	entries := []QueueEntry{}
	if !isDir(dir) {
		return entries, nil
	}
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("%s lesen: %w", dir, err)
	}
	for _, file := range files {
		name := file.Name()
		if file.IsDir() || strings.HasPrefix(name, ".") || !strings.HasSuffix(name, ".md") || name == knowledgeReadmeName {
			continue
		}
		entry, err := readQueueEntry(filepath.Join(dir, name))
		if err != nil {
			k.note("Queue-Eintrag %s übersprungen: %v", name, err)
			continue
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i int, j int) bool { return entries[i].ID < entries[j].ID })
	return entries, nil
}

// readQueueEntry liest eine Eintragsdatei.
func readQueueEntry(full string) (QueueEntry, error) {
	data, err := os.ReadFile(full)
	if err != nil {
		return QueueEntry{}, err
	}
	block, ok := inventory.FrontmatterBlock(data)
	if !ok {
		return QueueEntry{}, fmt.Errorf("kein Frontmatter")
	}
	head, err := yamllite.Parse([]byte(block))
	if err != nil {
		return QueueEntry{}, err
	}
	entry := QueueEntry{
		ID:     strings.TrimSuffix(filepath.Base(full), ".md"),
		Origin: strings.TrimSpace(head.Get("origin").Str()),
		Target: strings.TrimSpace(head.Get("target").Str()),
		Reason: strings.TrimSpace(head.Get("reason").Str()),
		Notes:  strings.TrimSpace(string(inventory.Body(data))),
	}
	if added, err := time.Parse(time.RFC3339, strings.TrimSpace(head.Get("added").Str())); err == nil {
		entry.Added = added
	}
	return entry, nil
}

// QueueDrop löscht einen Eintrag ohne Übernahme. Der Grund wird
// entgegengenommen und nirgends festgehalten: die Zone hält Arbeit, nicht
// Geschichte — wer den Grund braucht, schreibt ihn dorthin, wo Geschichte
// hingehört. Ein Eintrag, den es nicht gibt, ist ein Fehler.
func (k *Knowledge) QueueDrop(id string, reason string) error {
	full, err := queueEntryFile(k.projectDir, id)
	if err != nil {
		return err
	}
	if !fileExists(full) {
		return InputErrorf("Queue-Eintrag %q gibt es nicht", strings.TrimSpace(id))
	}
	_ = reason
	if err := os.Remove(full); err != nil {
		return fmt.Errorf("Queue-Eintrag %s löschen: %w", strings.TrimSpace(id), err)
	}
	return nil
}
