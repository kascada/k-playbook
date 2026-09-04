// Package legacy räumt weg, was das abgelöste host-globale Installationsmodell
// auf einem Rechner hinterlassen hat.
//
// Früher hat der Installer die Commands und Skills einer zentralen
// Basisinstallation in die User-Konfiguration der Assistenten verlinkt:
// Symlinks unter ~/.claude/commands, ~/.claude/skills und
// ~/.config/opencode/command sowie ein skills.paths-Eintrag in der
// OpenCode-User-Config. Seit der Umstellung auf die projektlokale Installation
// wird ausschließlich innerhalb des Projekts verlinkt. Bleiben die alten
// globalen Links liegen, sieht ein Assistent in jedem Projekt zusätzlich die
// Commands eines fremden Standes.
package legacy

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// playbookSegment ist das Pfadsegment, an dem eine alte Verlinkung erkannt
// wird. Der Repo-Pfad war frei wählbar, das Verzeichnis hieß aber immer so.
const playbookSegment = "k-playbook"

// Removal ist eine entfernte Altlast.
type Removal struct {
	// Path ist der entfernte Pfad, absolut.
	Path string
	// Detail sagt in einem Halbsatz, was dort stand.
	Detail string
}

func (r Removal) String() string {
	return r.Path + " (" + r.Detail + ")"
}

// legacyLinkDirs sind die Verzeichnisse, in denen das alte Modell verlinkt hat.
func legacyLinkDirs(home string) []string {
	return []string{
		filepath.Join(home, ".claude", "commands"),
		filepath.Join(home, ".claude", "skills"),
		filepath.Join(home, ".config", "opencode", "command"),
	}
}

// legacyConfigs sind die OpenCode-User-Configs, die einen skills.paths-Eintrag
// auf eine zentrale Basisinstallation tragen können.
func legacyConfigs(home string) []string {
	return []string{
		filepath.Join(home, ".config", "opencode", "opencode.jsonc"),
		filepath.Join(home, ".config", "opencode", "opencode.json"),
	}
}

// RemoveGlobalLinks entfernt die host-globale Assistenten-Registrierung des
// alten Modells und meldet, was entfernt wurde.
//
// Angefasst wird nur, was nachweislich zu einem k-playbook gehört: Symlinks,
// deren Ziel ein Pfadsegment k-playbook enthält, und der skills.paths-Eintrag,
// der auf ein solches Verzeichnis zeigt. Echte Dateien und fremde Symlinks
// bleiben liegen. Ein Fehler an einer Stelle stoppt den Rest nicht; die bis
// dahin entfernten Einträge werden zusammen mit dem Fehler gemeldet.
func RemoveGlobalLinks() ([]Removal, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("Home-Verzeichnis ermitteln: %w", err)
	}
	return removeGlobalLinks(home)
}

// RemoveFormerHostInstall räumt die alte Wrapper-Spiegelung und ihren
// Standard-Cache weg. Ein direkt installiertes Binary bleibt erhalten; nur ein
// Symlink auf die alte Spiegelung fällt weg.
func RemoveFormerHostInstall() ([]Removal, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("Home-Verzeichnis ermitteln: %w", err)
	}
	return removeFormerHostInstall(home)
}

func removeFormerHostInstall(home string) ([]Removal, error) {
	installation := filepath.Join(home, ".local", "share", "k-playbook", "installation")
	cache := filepath.Join(home, ".cache", "k-playbook")
	launcher := filepath.Join(home, ".local", "bin", "k-playbook")
	removals := []Removal{}
	problems := []string{}

	if target, ok := symlinkTarget(launcher); ok && isWithin(target, installation) {
		if err := os.Remove(launcher); err != nil {
			problems = append(problems, fmt.Sprintf("%s entfernen: %v", launcher, err))
		} else {
			removals = append(removals, Removal{Path: launcher, Detail: "Symlink auf alte Host-Installation"})
		}
	}
	for _, path := range []string{installation, cache} {
		if !pathExists(path) {
			continue
		}
		if err := os.RemoveAll(path); err != nil {
			problems = append(problems, fmt.Sprintf("%s entfernen: %v", path, err))
		} else {
			removals = append(removals, Removal{Path: path, Detail: "abgelöstes Wrapper-Modell"})
		}
	}
	if len(problems) > 0 {
		return removals, errors.New(strings.Join(problems, "; "))
	}
	return removals, nil
}

func symlinkTarget(path string) (string, bool) {
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		return "", false
	}
	target, err := os.Readlink(path)
	if err != nil {
		return "", false
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(path), target)
	}
	return filepath.Clean(target), true
}

func isWithin(path string, parent string) bool {
	relative, err := filepath.Rel(parent, path)
	return err == nil && relative != ".." && !filepath.IsAbs(relative)
}

func pathExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func removeGlobalLinks(home string) ([]Removal, error) {
	removals := []Removal{}
	problems := []string{}

	for _, dir := range legacyLinkDirs(home) {
		found, err := removeLinkDir(dir)
		removals = append(removals, found...)
		if err != nil {
			problems = append(problems, err.Error())
		}
	}

	for _, config := range legacyConfigs(home) {
		found, err := removeSkillsPath(config)
		if found != nil {
			removals = append(removals, *found)
		}
		if err != nil {
			problems = append(problems, err.Error())
		}
	}

	if len(problems) > 0 {
		return removals, errors.New(strings.Join(problems, "; "))
	}
	return removals, nil
}

// removeLinkDir räumt ein Verzeichnis der alten Registrierung.
//
// Ist das Verzeichnis selbst ein Symlink in ein k-playbook, fällt es als
// Ganzes weg. Ist es ein echtes Verzeichnis, fallen nur die Symlinks weg, die
// dorthin zeigen — projekteigene Dateien des Nutzers bleiben. Bleibt danach
// nichts übrig, verschwindet auch das leere Verzeichnis.
func removeLinkDir(dir string) ([]Removal, error) {
	info, err := os.Lstat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("%s prüfen: %w", dir, err)
	}

	if info.Mode()&os.ModeSymlink != 0 {
		destination, ok := playbookLink(dir)
		if !ok {
			return nil, nil
		}
		if err := os.Remove(dir); err != nil {
			return nil, fmt.Errorf("%s entfernen: %w", dir, err)
		}
		return []Removal{{Path: dir, Detail: "Verzeichnis-Symlink auf " + destination}}, nil
	}

	if !info.IsDir() {
		return nil, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("%s lesen: %w", dir, err)
	}

	removals := []Removal{}
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		destination, ok := playbookLink(path)
		if !ok {
			continue
		}
		if err := os.Remove(path); err != nil {
			return removals, fmt.Errorf("%s entfernen: %w", path, err)
		}
		removals = append(removals, Removal{Path: path, Detail: "Symlink auf " + destination})
	}

	if len(removals) > 0 && len(removals) == len(entries) {
		if err := os.Remove(dir); err == nil {
			removals = append(removals, Removal{Path: dir, Detail: "leeres Verzeichnis"})
		}
	}
	return removals, nil
}

// playbookLink meldet das Ziel eines Symlinks, wenn er in ein k-playbook zeigt.
// Ob das Ziel noch existiert, spielt keine Rolle: ein toter Link ist genauso
// eine Altlast wie ein lebender.
func playbookLink(path string) (string, bool) {
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		return "", false
	}

	destination, err := os.Readlink(path)
	if err != nil {
		return "", false
	}

	resolved := destination
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(filepath.Dir(path), resolved)
	}
	return destination, hasPlaybookSegment(filepath.Clean(resolved))
}

// hasPlaybookSegment prüft, ob ein Pfad ein Verzeichnis k-playbook durchläuft.
func hasPlaybookSegment(path string) bool {
	return slices.Contains(strings.Split(filepath.ToSlash(path), "/"), playbookSegment)
}

// removeSkillsPath entfernt aus einer OpenCode-User-Config den Top-Level-Key
// skills, sofern sein Wert auf ein k-playbook zeigt. Alles andere in der Datei
// bleibt Zeichen für Zeichen erhalten, Kommentare eingeschlossen.
func removeSkillsPath(path string) (*Removal, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("%s lesen: %w", path, err)
	}

	updated, changed := stripSkillsBlock(string(data))
	if !changed {
		return nil, nil
	}

	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	if err := os.WriteFile(path, []byte(updated), mode); err != nil {
		return nil, fmt.Errorf("%s schreiben: %w", path, err)
	}
	return &Removal{Path: path, Detail: "skills.paths auf ein k-playbook"}, nil
}

// stripSkillsBlock schneidet den skills-Eintrag samt zugehörigem Komma heraus.
func stripSkillsBlock(content string) (string, bool) {
	start, end, ok := findSkillsEntry(content)
	if !ok {
		return content, false
	}
	return cutEntry(content, start, end), true
}

// findSkillsEntry sucht den Top-Level-Key skills, dessen Wert ein k-playbook
// nennt, und liefert Anfang des Keys und Ende des Werts.
func findSkillsEntry(content string) (int, int, bool) {
	depth := 0
	i := 0
	for i < len(content) {
		switch {
		case strings.HasPrefix(content[i:], "//"):
			i = skipLineComment(content, i)
		case strings.HasPrefix(content[i:], "/*"):
			i = skipBlockComment(content, i)
		case content[i] == '"':
			end := skipString(content, i)
			if depth != 1 || content[i:end] != `"skills"` {
				i = end
				continue
			}
			value := valueStart(content, end)
			if value < 0 {
				i = end
				continue
			}
			stop := skipValue(content, value)
			if strings.Contains(content[value:stop], playbookSegment) {
				return i, stop, true
			}
			i = stop
		case content[i] == '{' || content[i] == '[':
			depth++
			i++
		case content[i] == '}' || content[i] == ']':
			depth--
			i++
		default:
			i++
		}
	}
	return 0, 0, false
}

// cutEntry entfernt den Bereich und das Komma, das den Eintrag an seine
// Nachbarn bindet: bevorzugt das nachfolgende, sonst das vorangehende. Steht in
// den Randzeilen sonst nichts, fallen sie ganz weg, damit keine Zeile mit
// Restweißraum stehen bleibt.
func cutEntry(content string, start int, end int) string {
	comma := -1

	after := end
	for after < len(content) && isSpace(content[after]) {
		after++
	}
	if after < len(content) && content[after] == ',' {
		end = after + 1
	} else {
		before := start
		for before > 0 && isSpace(content[before-1]) {
			before--
		}
		if before > 0 && content[before-1] == ',' {
			comma = before - 1
		}
	}

	lineStart := strings.LastIndexByte(content[:start], '\n') + 1
	prefix := content[lineStart:start]
	if comma >= lineStart {
		prefix = content[lineStart:comma] + content[comma+1:start]
	}
	if strings.TrimSpace(prefix) == "" {
		if comma >= lineStart {
			comma = -1
		}
		start = lineStart

		lineEnd := strings.IndexByte(content[end:], '\n')
		if lineEnd >= 0 && strings.TrimSpace(content[end:end+lineEnd]) == "" {
			end += lineEnd + 1
		}
	}

	if comma >= 0 {
		return content[:comma] + content[comma+1:start] + content[end:]
	}
	return content[:start] + content[end:]
}

// valueStart liefert den Beginn des Werts hinter einem Key, oder -1, wenn dort
// kein Doppelpunkt steht.
func valueStart(content string, i int) int {
	i = skipGap(content, i)
	if i >= len(content) || content[i] != ':' {
		return -1
	}
	return skipGap(content, i+1)
}

// skipValue liest über einen Wert hinweg und liefert die Position dahinter.
func skipValue(content string, i int) int {
	depth := 0
	for i < len(content) {
		switch {
		case strings.HasPrefix(content[i:], "//"):
			i = skipLineComment(content, i)
		case strings.HasPrefix(content[i:], "/*"):
			i = skipBlockComment(content, i)
		case content[i] == '"':
			i = skipString(content, i)
			if depth == 0 {
				return i
			}
		case content[i] == '{' || content[i] == '[':
			depth++
			i++
		case content[i] == '}' || content[i] == ']':
			if depth == 0 {
				return i
			}
			depth--
			i++
			if depth == 0 {
				return i
			}
		case content[i] == ',' && depth == 0:
			return i
		default:
			i++
		}
	}
	return i
}

// skipGap überliest Weißraum und Kommentare.
func skipGap(content string, i int) int {
	for i < len(content) {
		switch {
		case isSpace(content[i]):
			i++
		case strings.HasPrefix(content[i:], "//"):
			i = skipLineComment(content, i)
		case strings.HasPrefix(content[i:], "/*"):
			i = skipBlockComment(content, i)
		default:
			return i
		}
	}
	return i
}

func skipString(content string, i int) int {
	for j := i + 1; j < len(content); j++ {
		switch content[j] {
		case '\\':
			j++
		case '"':
			return j + 1
		}
	}
	return len(content)
}

func skipLineComment(content string, i int) int {
	if end := strings.IndexByte(content[i:], '\n'); end >= 0 {
		return i + end
	}
	return len(content)
}

func skipBlockComment(content string, i int) int {
	if end := strings.Index(content[i+2:], "*/"); end >= 0 {
		return i + 2 + end + 2
	}
	return len(content)
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}
