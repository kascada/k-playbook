package project

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DocsDirName ist das Doku-Verzeichnis innerhalb der Installation. Es gehoert
// zur mitgelieferten Ebene und wird bei jedem Update ersetzt; projekteigene
// Doku liegt getrennt davon unter k-playbook-local/docs.
const DocsDirName = "docs"

// Doc ist eine Markdown-Datei der mitgelieferten Doku.
type Doc struct {
	// Path ist der Ort relativ zum Doku-Verzeichnis, immer mit Schraegstrich.
	Path string `json:"path"`
	// Title ist die erste Ueberschrift der Datei, ersatzweise der Dateiname.
	Title string `json:"title"`
}

// DocsDir ist das Doku-Verzeichnis eines Projekts.
func DocsDir(projectDir string) string {
	return filepath.Join(PlaybookDir(projectDir), DocsDirName)
}

// ListDocs sammelt alle Markdown-Dateien der Doku, auch aus
// Unterverzeichnissen. Die README steht vorn, sie ist der Einstieg; der Rest
// folgt alphabetisch.
func ListDocs(projectDir string) ([]Doc, error) {
	root := DocsDir(projectDir)
	if !isDir(root) {
		return nil, fmt.Errorf("%s fehlt; die Installation ist unvollstaendig", root)
	}

	docs := []Doc{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			// Ein unlesbarer Teilbaum darf die uebrige Liste nicht verhindern.
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
		docs = append(docs, Doc{Path: filepath.ToSlash(rel), Title: docTitle(path)})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("Doku lesen: %w", err)
	}

	sort.Slice(docs, func(i int, j int) bool {
		if readme := docs[i].Path == "README.md"; readme != (docs[j].Path == "README.md") {
			return readme
		}
		return docs[i].Path < docs[j].Path
	})
	return docs, nil
}

// ReadDoc liefert eine einzelne Datei der Doku als Rohtext.
func ReadDoc(projectDir string, rel string) (Doc, []byte, error) {
	root := DocsDir(projectDir)
	path, err := docFilePath(root, rel)
	if err != nil {
		return Doc{}, nil, err
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return Doc{}, nil, fmt.Errorf("%s lesen: %w", rel, err)
	}

	cleaned, err := filepath.Rel(root, path)
	if err != nil {
		cleaned = rel
	}
	return Doc{Path: filepath.ToSlash(cleaned), Title: docTitle(path)}, content, nil
}

// docFilePath loest einen angefragten Pfad im Doku-Verzeichnis auf. Der Pfad
// kommt aus dem Browser: er muss relativ bleiben, im Verzeichnis liegen und
// eine Markdown-Datei meinen. Alles andere waere ein Weg, beliebige Dateien des
// Rechners zu lesen.
func docFilePath(root string, rel string) (string, error) {
	if strings.TrimSpace(rel) == "" {
		return "", fmt.Errorf("kein Pfad angegeben")
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("nur Pfade innerhalb von %s", DocsDirName)
	}

	path := filepath.Join(root, filepath.FromSlash(rel))
	if path != root && !strings.HasPrefix(path, root+string(filepath.Separator)) {
		return "", fmt.Errorf("nur Pfade innerhalb von %s", DocsDirName)
	}
	if !strings.EqualFold(filepath.Ext(path), ".md") {
		return "", fmt.Errorf("nur Markdown-Dateien")
	}
	return path, nil
}

// docTitle nimmt die erste Ueberschrift der Datei. Sie beschreibt den Inhalt
// besser als der Dateiname, der nur der Rueckfall ist.
func docTitle(path string) string {
	fallback := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	content, err := os.ReadFile(path)
	if err != nil {
		return fallback
	}
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return fallback
}
