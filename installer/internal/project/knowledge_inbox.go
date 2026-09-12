package project

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// InboxDir ist der Eingang eines Projekts.
func InboxDir(projectDir string) string {
	return filepath.Join(LocalDir(projectDir), InboxDirName)
}

// inboxNoteSuffix ist die Endung der Notiz zu einem Rohstück: <name>.note
// neben der Datei, nur wenn beim Ablegen eine Notiz übergeben wurde. Die
// Liste zeigt sie nicht als eigenen Eintrag, sondern hängt sie an den des
// Rohstücks.
const inboxNoteSuffix = ".note"

// inboxTextExtensions sind die Endungen, die InboxRead als Text liefert.
// Alles andere — Bilder, PDFs, Archive — bleibt im Eingang liegen und wird
// mit klarer Meldung abgewiesen: ein Textkanal für Binärdaten wäre Rauschen.
var inboxTextExtensions = []string{"md", "txt", "html", "htm", "json", "yaml", "yml", "csv", "xml", "log"}

// inboxFormats ordnet Endungen dem Format zu, das die Liste nennt; die
// Werte decken sich, wo es passt, mit dem format-Feld der Wissensablage.
var inboxFormats = map[string]string{
	"md": "markdown", "markdown": "markdown",
	"txt": "text", "log": "text", "csv": "text", "json": "text", "yaml": "text", "yml": "text", "xml": "text",
	"html": "html", "htm": "html",
	"png": "image", "jpg": "image", "jpeg": "image", "gif": "image", "webp": "image", "svg": "image",
	"pdf": "pdf",
}

// InboxEntry ist ein Rohstück in der Liste des Eingangs.
type InboxEntry struct {
	// Path ist der Ort relativ zu inbox/, mit Schrägstrich.
	Path   string `json:"path"`
	Source string `json:"source"`
	// Name ist der Rest hinter der Quelle.
	Name string `json:"name"`
	// Format leitet sich aus der Endung ab: markdown, text, html, image, pdf
	// — oder die Endung selbst, wenn keine Zuordnung passt.
	Format string `json:"format"`
	Size   int64  `json:"size"`
	// Modified ist der Zeitpunkt der letzten Änderung, RFC3339.
	Modified time.Time `json:"modified"`
	// Note ist der Inhalt der Notiz daneben, wo es eine gibt.
	Note string `json:"note,omitempty"`
}

// inboxRelPath prüft Quelle und Name und liefert den Pfad relativ zu inbox/.
// Die Quelle ist ein einzelnes Verzeichnis; der Name darf Unterverzeichnisse
// tragen, aber weder herausführen noch versteckt sein.
func inboxRelPath(source string, name string) (string, error) {
	source = strings.TrimSpace(source)
	if !isCleanPathSegment(source) {
		return "", InputErrorf("Quelle %q muss ein einzelner Verzeichnisname sein (confluence, chat, mail, scan …)", source)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", InputErrorf("kein Name angegeben")
	}
	if filepath.IsAbs(name) || strings.HasPrefix(name, "/") || strings.HasPrefix(name, "\\") {
		return "", InputErrorf("Name %q führt aus %s/%s/ heraus: nur relative Namen", name, LocalDirName, InboxDirName)
	}
	cleaned := path.Clean(filepath.ToSlash(name))
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", InputErrorf("Name %q führt aus %s/%s/ heraus", name, LocalDirName, InboxDirName)
	}
	for _, segment := range strings.Split(cleaned, "/") {
		if strings.HasPrefix(segment, ".") {
			return "", InputErrorf("Name %q: versteckte Einträge (%q) gehören nicht in den Eingang", name, segment)
		}
	}
	if strings.HasSuffix(cleaned, inboxNoteSuffix) {
		return "", InputErrorf("Name %q: die Endung %s ist der Notiz vorbehalten", name, inboxNoteSuffix)
	}
	return source + "/" + cleaned, nil
}

// InboxPut legt ein Rohstück unter inbox/<source>/<name> ab, so wie es kommt —
// keine Prüfung des Inhalts, kein Frontmatter, keine Umwandlung. Eine Notiz
// wird daneben als <name>.note abgelegt, nur wenn eine übergeben wurde. Ein
// Name, der schon belegt ist, wird abgewiesen: der Eingang ist ein Archiv,
// und Überschreiben wäre ein Löschen ohne Absicht.
//
// Das Ergebnis ist der Pfad relativ zu inbox/.
func (k *Knowledge) InboxPut(source string, name string, content []byte, note string) (string, error) {
	rel, err := inboxRelPath(source, name)
	if err != nil {
		return "", err
	}
	if len(content) == 0 {
		return "", InputErrorf("leerer Inhalt: ein Rohstück ohne Inhalt gehört nicht in den Eingang")
	}
	full := filepath.Join(InboxDir(k.projectDir), filepath.FromSlash(rel))
	if pathExists(full) {
		return "", InputErrorf("%s liegt schon im Eingang — der Eingang überschreibt nicht, ein anderer Name oder Löschen von Hand", rel)
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return "", fmt.Errorf("%s anlegen: %w", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, content, 0o644); err != nil {
		return "", fmt.Errorf("%s schreiben: %w", rel, err)
	}
	if strings.TrimSpace(note) != "" {
		text := strings.TrimSpace(note) + "\n"
		if err := os.WriteFile(full+inboxNoteSuffix, []byte(text), 0o644); err != nil {
			return "", fmt.Errorf("Notiz zu %s schreiben: %w", rel, err)
		}
	}
	return rel, nil
}

// InboxList nennt die Rohstücke des Eingangs, alle oder die einer Quelle,
// alphabetisch nach Pfad. Notizdateien erscheinen nicht selbst, sondern am
// Eintrag ihres Rohstücks; die README in der Wurzel und versteckte Einträge
// gehören nicht dazu. Ein fehlender Eingang ist leer, kein Fehler.
func (k *Knowledge) InboxList(source string) ([]InboxEntry, error) {
	root := InboxDir(k.projectDir)
	source = strings.TrimSpace(source)
	if source != "" && !isCleanPathSegment(source) {
		return nil, InputErrorf("Quelle %q muss ein einzelner Verzeichnisname sein", source)
	}
	entries := []InboxEntry{}
	if !isDir(root) {
		return entries, nil
	}

	notes := map[string]string{}
	err := filepath.WalkDir(root, func(full string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if entry.IsDir() {
			if full != root && strings.HasPrefix(entry.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, full)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(entry.Name(), ".") || !strings.Contains(rel, "/") {
			// Versteckt oder flach in der Wurzel (README.md): kein Rohstück.
			return nil
		}
		entrySource := rel[:strings.IndexByte(rel, '/')]
		if source != "" && entrySource != source {
			return nil
		}
		if strings.HasSuffix(rel, inboxNoteSuffix) {
			if content, err := os.ReadFile(full); err == nil {
				notes[strings.TrimSuffix(rel, inboxNoteSuffix)] = strings.TrimSpace(string(content))
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		entries = append(entries, InboxEntry{
			Path:     rel,
			Source:   entrySource,
			Name:     rel[len(entrySource)+1:],
			Format:   inboxFormat(rel),
			Size:     info.Size(),
			Modified: info.ModTime().Truncate(time.Second),
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("%s lesen: %w", root, err)
	}
	for index := range entries {
		entries[index].Note = notes[entries[index].Path]
	}
	sort.Slice(entries, func(i int, j int) bool { return entries[i].Path < entries[j].Path })
	return entries, nil
}

// inboxFormat leitet das Format aus der Endung ab.
func inboxFormat(rel string) string {
	ext := strings.ToLower(strings.TrimPrefix(path.Ext(rel), "."))
	if format, ok := inboxFormats[ext]; ok {
		return format
	}
	if ext == "" {
		return "unbekannt"
	}
	return ext
}

// InboxRead liefert ein Rohstück als Text — nur für Textformate, bestimmt
// über die Endung. Der Pfad ist relativ zu inbox/, so wie InboxList ihn
// nennt.
func (k *Knowledge) InboxRead(rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	slash := strings.IndexByte(filepath.ToSlash(rel), '/')
	if rel == "" || slash <= 0 {
		return "", InputErrorf("Pfad %q: erwartet wird <quelle>/<name>, so wie inbox list ihn nennt", rel)
	}
	cleaned, err := inboxRelPath(rel[:slash], rel[slash+1:])
	if err != nil {
		return "", err
	}
	ext := strings.ToLower(strings.TrimPrefix(path.Ext(cleaned), "."))
	if !oneOf(inboxTextExtensions, ext) {
		return "", InputErrorf("%s ist kein Textformat (%s) — gelesen werden nur %s; Bilder und PDFs bleiben im Eingang und werden über ein Markdown-Dokument in der Wissensablage beschrieben",
			cleaned, inboxFormat(cleaned), strings.Join(inboxTextExtensions, ", "))
	}
	content, err := os.ReadFile(filepath.Join(InboxDir(k.projectDir), filepath.FromSlash(cleaned)))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// Nicht vorhanden ist ein Eingabefehler: der Pfad ist falsch,
			// nicht der Eingang.
			return "", InputErrorf("%s gibt es nicht im Eingang — inbox list nennt, was dort liegt", cleaned)
		}
		return "", fmt.Errorf("%s lesen: %w", cleaned, err)
	}
	return string(content), nil
}
