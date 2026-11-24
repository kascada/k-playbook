package project

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/kascada/k-playbook/installer/internal/health"
)

// Die Quelle frontmatter der Umgebungsprüfung: Sie meldet ein Frontmatter, das
// ein strenger YAML-Parser nicht liest. Ein solcher Kopf macht einen Command
// oder Skill in OpenCode 2 unsichtbar, ohne dass eine Meldung erscheint —
// etwa ein unquotiertes `[a|b] [c]` oder ein ": " im Wert.
//
// Geprüft wird die ungefilterte Auflösung ohne abgeschaltete Einträge, also
// auch Fragmente. Maßstab ist yaml.v3, nicht das Verhalten eines Assistenten;
// die schlichte Bibliothek yamllite liest zu großzügig und taugt hier nicht.
// Dateien ohne Frontmatter sind kein Fehler.
func init() {
	health.Register(healthFrontmatter)
}

func healthFrontmatter(_ context.Context, projectDir string) []health.Problem {
	var problems []health.Problem
	for _, kind := range []AssetKind{KindCommands, KindSkills} {
		for _, entry := range ResolveRegistry(projectDir, kind) {
			if entry.Disabled {
				continue
			}
			file := entry.Path
			if entry.IsDir {
				file = filepath.Join(entry.Path, skillFileName)
			}
			err := CheckFrontmatter(file)
			if err == nil {
				continue
			}
			problems = append(problems, health.Problem{
				Level:   health.LevelError,
				Area:    health.AreaFrontmatter,
				Message: "Das Frontmatter von " + displayPath(projectDir, file) + " ist kein gültiges YAML: " + err.Error() + ".",
				Hint: "Werte mit `: ` oder führendem `[` in Anführungszeichen setzen; ein ungültiges Frontmatter " +
					"macht den Command oder Skill in OpenCode 2 unsichtbar.",
			})
		}
	}
	return problems
}

// displayPath kürzt Pfade im Projekt auf den Teil ab der Projektwurzel.
func displayPath(projectDir string, path string) string {
	if rel, err := filepath.Rel(projectDir, path); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return path
}

// CheckFrontmatter liest das Frontmatter einer Datei mit einem strengen
// YAML-Parser. Ohne Frontmatter (die Datei beginnt nicht mit ---) ist es nil.
func CheckFrontmatter(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return nil
	}
	rest := text[len("---\n"):]
	var head string
	switch {
	case strings.HasPrefix(rest, "---\n") || rest == "---":
		return nil
	default:
		end := strings.Index(rest, "\n---\n")
		if end < 0 {
			if !strings.HasSuffix(rest, "\n---") {
				return fmt.Errorf("das Frontmatter hat keine schließende Zeile ---")
			}
			end = len(rest) - len("\n---")
		}
		head = rest[:end]
	}

	decoder := yaml.NewDecoder(bytes.NewReader([]byte(head)))
	var out map[string]any
	if err := decoder.Decode(&out); err != nil && err.Error() != "EOF" {
		return fmt.Errorf("%s", strings.TrimPrefix(err.Error(), "yaml: "))
	}
	return nil
}
