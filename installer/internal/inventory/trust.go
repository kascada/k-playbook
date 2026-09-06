package inventory

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// MaxFileSize ist die Obergrenze je gelesener Quelle. Wird sie überschritten,
// ist das eine sichtbare Ablehnung und kein Teil-Lesen: ein halb gelesenes
// Manifest erzeugt ein halbes Inventar, und das sähe aus wie ein vollständiges.
const MaxFileSize = 8 << 20

// Boundary ist die Vertrauensgrenze aus docs/versionsinventar.md. Sie ist die
// einzige Stelle, an der entschieden wird, ob eine Datei gelesen werden darf,
// und die einzige, die sie öffnet.
//
// Warum das hier und nicht in pathnorm liegt: `pathnorm.Normalize` beantwortet
// eine ganz andere Frage — ob zwei SARIF-Pfade auf dieselbe Stelle zeigen. Es
// schreibt dafür klein, wirft ein führendes `/` weg und macht damit aus einem
// absoluten Pfad einen relativen. Für eine Sicherheitsprüfung wäre genau das
// falsch: `/etc/passwd` und `etc/passwd` dürfen hier nicht dasselbe sein. Die
// beiden Funktionen sehen sich ähnlich und meinen Gegenteiliges; sie
// zusammenzulegen hieße, die Grenze von der Gruppierungslogik abhängig zu
// machen.
type Boundary struct {
	// roots sind die aufgelösten, erlaubten Wurzeln. Die Projektwurzel steht
	// immer an erster Stelle.
	roots []string
	// projectRoot ist die aufgelöste Projektwurzel; relative Pfade werden gegen
	// sie aufgelöst, nie gegen das Arbeitsverzeichnis des Prozesses.
	projectRoot string
	// requestedRoots hält die Wurzeln so, wie sie angefragt wurden, für die
	// Meldung.
	requestedRoots []string
}

// PathError ist eine Ablehnung der Vertrauensgrenze. Sie nennt angefragten
// Pfad, aufgelösten Pfad und Grund — die drei Angaben, die der Vertrag für
// jede Ablehnung verlangt.
type PathError struct {
	Requested string
	Resolved  string
	Reason    string
}

func (e *PathError) Error() string {
	if e.Resolved != "" && e.Resolved != e.Requested {
		return fmt.Sprintf("%s (aufgelöst: %s): %s", e.Requested, e.Resolved, e.Reason)
	}
	return fmt.Sprintf("%s: %s", e.Requested, e.Reason)
}

// Rejection macht aus der Ablehnung den Eintrag fürs Ergebnis.
func (e *PathError) Rejection() Rejection {
	return Rejection{Requested: e.Requested, Resolved: e.Resolved, Reason: e.Reason}
}

// NewBoundary baut die Grenze aus der Projektwurzel und den in der
// Quellenkonfiguration freigegebenen Wurzeln.
//
// Es gibt keine impliziten Wurzeln: ein absoluter Pfad in `sources:` gibt seine
// Wurzel nicht selbst frei. Wer außerhalb lesen will, schreibt die Wurzel hin —
// damit die Frage „was darf dieses Binary lesen" an einer Stelle und ohne
// Glob-Auswertung zu beantworten ist.
func NewBoundary(projectRoot string, extraRoots []string) (*Boundary, error) {
	if strings.TrimSpace(projectRoot) == "" {
		return nil, fmt.Errorf("keine Projektwurzel angegeben")
	}
	if !filepath.IsAbs(projectRoot) {
		absolute, err := filepath.Abs(projectRoot)
		if err != nil {
			return nil, fmt.Errorf("Projektwurzel %s auflösen: %w", projectRoot, err)
		}
		projectRoot = absolute
	}
	resolvedProject := resolveExisting(filepath.Clean(projectRoot))

	boundary := &Boundary{
		projectRoot:    resolvedProject,
		roots:          []string{resolvedProject},
		requestedRoots: []string{projectRoot},
	}
	for _, root := range extraRoots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		// Eine relative Wurzel wäre eine Wurzel, die vom Arbeitsverzeichnis des
		// Aufrufers abhinge — auf dem CLI-Weg etwas anderes als im
		// Webserver-Prozess. Genau das darf eine Vertrauensgrenze nicht.
		if !filepath.IsAbs(root) {
			return nil, fmt.Errorf("Wurzel %q ist nicht absolut; `roots:` nimmt nur absolute Pfade", root)
		}
		resolved := resolveExisting(filepath.Clean(root))
		boundary.roots = append(boundary.roots, resolved)
		boundary.requestedRoots = append(boundary.requestedRoots, root)
	}
	return boundary, nil
}

// ProjectRoot ist die aufgelöste Projektwurzel.
func (b *Boundary) ProjectRoot() string { return b.projectRoot }

// Roots sind die erlaubten Wurzeln, aufgelöst.
func (b *Boundary) Roots() []string { return append([]string(nil), b.roots...) }

// Check führt die fünf Schritte des Vertrags aus und liefert den geprüften,
// absoluten Pfad. Kein Leser öffnet eine Datei an dieser Funktion vorbei.
//
//  1. Keine Expansion: `~`, `$VAR` und `%VAR%` sind Bestandteil des Pfads.
//  2. Absolut machen, relativ zur Projektwurzel.
//  3. Lexikalisch säubern: `.` und `..` auflösen.
//  4. Symlinks auflösen, einschließlich aller Elternsegmente.
//  5. Prüfen: gleich einer Wurzel oder unterhalb einer Wurzel, segmentweise.
func (b *Boundary) Check(requested string) (string, error) {
	trimmed := strings.TrimSpace(requested)
	if trimmed == "" {
		return "", &PathError{Requested: requested, Reason: "leerer Pfad"}
	}

	// Schritt 1 und 2. `~` und `$VAR` bleiben stehen und werden dadurch zu
	// gewöhnlichen Segmenten — ein Pfad, dessen Bedeutung von der Umgebung des
	// Aufrufers abhängt, ist an einer Vertrauensgrenze nicht zu gebrauchen.
	absolute := trimmed
	if !filepath.IsAbs(absolute) {
		absolute = filepath.Join(b.projectRoot, absolute)
	}
	// Schritt 3.
	absolute = filepath.Clean(absolute)
	// Schritt 4.
	resolved := resolveExisting(absolute)
	// Schritt 5.
	if !b.inRoots(resolved) {
		return "", &PathError{
			Requested: requested,
			Resolved:  resolved,
			Reason:    "liegt außerhalb der erlaubten Wurzeln (" + strings.Join(b.requestedRoots, ", ") + ")",
		}
	}
	return resolved, nil
}

// ReadFile prüft und liest. Gelesen werden nur reguläre Dateien; Verzeichnisse,
// Gerätedateien, FIFOs und Sockets werden abgelehnt.
//
// exists ist false, wenn der Pfad erlaubt ist, aber nichts dort liegt. Das ist
// keine Ablehnung, sondern ein anderer Befund: eine konfigurierte, aber
// fehlende Quelle ist ein Hinweis, kein Grenzverstoß.
func (b *Boundary) ReadFile(requested string) (data []byte, resolved string, exists bool, err error) {
	resolved, err = b.Check(requested)
	if err != nil {
		return nil, "", false, err
	}
	info, statErr := os.Stat(resolved)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			return nil, resolved, false, nil
		}
		return nil, resolved, false, &PathError{Requested: requested, Resolved: resolved,
			Reason: "nicht lesbar: " + statErr.Error()}
	}
	if info.IsDir() {
		return nil, resolved, true, &PathError{Requested: requested, Resolved: resolved,
			Reason: "ist ein Verzeichnis, keine Datei"}
	}
	if !info.Mode().IsRegular() {
		return nil, resolved, true, &PathError{Requested: requested, Resolved: resolved,
			Reason: "ist keine reguläre Datei"}
	}
	if info.Size() > MaxFileSize {
		return nil, resolved, true, &PathError{Requested: requested, Resolved: resolved,
			Reason: fmt.Sprintf("ist %d Bytes groß, erlaubt sind %d", info.Size(), int64(MaxFileSize))}
	}
	content, readErr := os.ReadFile(resolved)
	if readErr != nil {
		return nil, resolved, true, &PathError{Requested: requested, Resolved: resolved,
			Reason: "nicht lesbar: " + readErr.Error()}
	}
	return content, resolved, true, nil
}

// Expand löst ein Glob auf. Der statische Anteil wird zuerst geprüft, damit ein
// Muster nicht erst außerhalb jeder Wurzel gesucht und dann verworfen wird;
// jedes einzelne Ergebnis durchläuft anschließend Check. Ein Glob ist damit nie
// ein Weg an der Prüfung vorbei.
//
// Enthält das Muster kein Metazeichen, ist das Ergebnis der Pfad selbst — auch
// wenn dort nichts liegt. Ob eine Datei fehlt, entscheidet der Aufrufer, weil
// nur er weiß, ob der Eintrag `optional` trägt.
func (b *Boundary) Expand(pattern string) ([]string, []Rejection) {
	trimmed := strings.TrimSpace(pattern)
	if !hasMeta(trimmed) {
		if _, err := b.Check(trimmed); err != nil {
			return nil, []Rejection{err.(*PathError).Rejection()}
		}
		return []string{trimmed}, nil
	}

	absolute := trimmed
	if !filepath.IsAbs(absolute) {
		absolute = filepath.Join(b.projectRoot, absolute)
	}
	absolute = filepath.Clean(absolute)

	base, rest := splitStatic(absolute)
	if _, err := b.Check(base); err != nil {
		rejection := err.(*PathError).Rejection()
		rejection.Requested = pattern
		rejection.Reason = "der feste Anteil des Musters " + rejection.Reason
		return nil, []Rejection{rejection}
	}

	matches := walkGlob(base, rest)
	sort.Strings(matches)

	var paths []string
	var rejections []Rejection
	for _, match := range matches {
		if _, err := b.Check(match); err != nil {
			rejections = append(rejections, err.(*PathError).Rejection())
			continue
		}
		paths = append(paths, match)
	}
	return paths, rejections
}

func (b *Boundary) inRoots(path string) bool {
	for _, root := range b.roots {
		if underRoot(path, root) {
			return true
		}
	}
	return false
}

// underRoot vergleicht segmentweise, nicht als Zeichenketten-Präfix:
// /srv/deploy erlaubt /srv/deploy/x, aber nicht /srv/deploy-alt/x.
func underRoot(path string, root string) bool {
	if path == root {
		return true
	}
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	if relative == "." {
		return true
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return false
	}
	return !filepath.IsAbs(relative)
}

// resolveExisting löst Symlinks so weit auf, wie der Pfad existiert, und hängt
// den nicht existierenden Rest wieder an.
//
// filepath.EvalSymlinks allein genügt nicht: es scheitert, sobald das letzte
// Segment fehlt — und genau das ist der Normalfall bei einer konfigurierten,
// aber noch nicht angelegten Quelle. Ohne den Rückfall wäre ein solcher Pfad
// ungeprüft, und die Prüfung fände auf dem ungesäuberten Original statt.
func resolveExisting(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	parent := filepath.Dir(path)
	if parent == path {
		return path
	}
	return filepath.Join(resolveExisting(parent), filepath.Base(path))
}

func hasMeta(pattern string) bool {
	return strings.ContainsAny(pattern, "*?[")
}

// splitStatic trennt den festen Anfang eines Musters von seinem Rest.
func splitStatic(pattern string) (string, []string) {
	segments := strings.Split(filepath.ToSlash(pattern), "/")
	static := []string{}
	index := 0
	for ; index < len(segments); index++ {
		if hasMeta(segments[index]) {
			break
		}
		static = append(static, segments[index])
	}
	base := strings.Join(static, "/")
	if base == "" {
		base = "/"
	}
	return filepath.FromSlash(base), segments[index:]
}

// walkGlob läuft die restlichen Mustersegmente ab. `**` steht für beliebig
// viele Verzeichnisebenen; filepath.Glob kennt das nicht, die Standardquellen
// des Vertrags brauchen es aber (`.devcontainer/**/devcontainer.json`).
//
// Symlinks auf Verzeichnisse werden beim Laufen nicht verfolgt; ein Treffer
// muss trotzdem einzeln durch Check.
func walkGlob(base string, segments []string) []string {
	if len(segments) == 0 {
		return []string{base}
	}
	segment := segments[0]
	rest := segments[1:]

	if segment == "**" {
		results := walkGlob(base, rest)
		for _, dir := range subdirectories(base) {
			results = append(results, walkGlob(dir, segments)...)
		}
		return results
	}

	entries, err := os.ReadDir(base)
	if err != nil {
		return nil
	}
	var results []string
	for _, entry := range entries {
		matched, err := filepath.Match(segment, entry.Name())
		if err != nil || !matched {
			continue
		}
		child := filepath.Join(base, entry.Name())
		if len(rest) == 0 {
			results = append(results, child)
			continue
		}
		if entry.IsDir() {
			results = append(results, walkGlob(child, rest)...)
		}
	}
	return results
}

func subdirectories(base string) []string {
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil
	}
	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() && !skippedDir(entry.Name()) {
			dirs = append(dirs, filepath.Join(base, entry.Name()))
		}
	}
	return dirs
}
