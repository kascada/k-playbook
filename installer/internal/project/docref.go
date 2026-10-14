package project

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"
	"syscall"

	"github.com/kascada/k-playbook/installer/internal/inventory"
)

// Verweise im Präfixschema (docs/verweisschema.md) und ihre Auflösung.
//
// Ein Verweis ist ein einziger Pfad mit Präfix: project:/…, k-playbook:/… oder
// keph:/…. Aufgelöst wird hier nur k-playbook:, und davon zunächst nur, was
// unter instructions/ liegt. Hinter k-playbook: stehen zwei Ebenen — die
// mitgelieferte und die lokale —, die über eine austauschbare Quelle gelesen
// werden: heute die Platte, später womöglich Kephalaion. Wer aufruft, sieht
// davon nichts; er bekommt den Inhalt mit lokal und mitgeliefert schon
// gegeneinander aufgelöst.
//
// Die Überlagerungsregel ist die der Kataloge (resolveCatalog): gleicher Name
// gewinnt lokal, ein leerer lokaler Eintrag schaltet ab. instructions ist
// aber bewusst keine Sorte in catalogKinds() und erscheint nicht unter
// catalogs der Kontextausgabe.

// Die Präfixe des Schemas.
const (
	DocSchemeProject   = "project"
	DocSchemeKPlaybook = "k-playbook"
	DocSchemeKeph      = "keph"
)

// docInstructionsRoot ist der einzige Bereich, den die Auflösung heute kennt.
const docInstructionsRoot = "instructions"

// Die Fehlercodes der Auflösung. Die Hüllen — MCP und Subkommando — melden sie
// unverändert; alles ohne Code ist die Umgebung (read_failed).
const (
	// DocCodeInvalidInput: der Verweis ist kein gültiger Verweis im Schema —
	// kein Präfix, kein „/" danach, ein leeres, verstecktes oder ..-Segment.
	DocCodeInvalidInput = "invalid_input"
	// DocCodeUnsupported: der Verweis ist gültig, wird vom Werkzeug aber noch
	// nicht aufgelöst — project:, keph: und k-playbook: außerhalb von
	// instructions/.
	DocCodeUnsupported = "unsupported"
	// DocCodeNotFound: es gibt ihn weder lokal noch mitgeliefert. Kein
	// Rückfall auf die Platte.
	DocCodeNotFound = "not_found"
)

// DocRefError ist ein Fehler mit Code aus der Auflösung eines Verweises.
type DocRefError struct {
	Code string
	err  error
}

func (e *DocRefError) Error() string { return e.err.Error() }

// Unwrap gibt den umhüllten Fehler frei.
func (e *DocRefError) Unwrap() error { return e.err }

func docRefErrorf(code string, format string, args ...any) error {
	return &DocRefError{Code: code, err: fmt.Errorf(format, args...)}
}

// DocRefErrorCode liefert den Code eines Auflösungsfehlers, leer bei allen
// anderen Fehlern.
func DocRefErrorCode(err error) string {
	var refErr *DocRefError
	if errors.As(err, &refErr) {
		return refErr.Code
	}
	return ""
}

// DocRef ist ein zerlegter Verweis.
type DocRef struct {
	// Raw ist der Verweis, wie er übergeben wurde, ohne umgebenden Leerraum.
	Raw string
	// Scheme ist das Präfix ohne Doppelpunkt: project, k-playbook oder keph.
	Scheme string
	// Path ist der Pfad ab der Wurzel des Raums, ohne führenden und ohne
	// abschließenden Schrägstrich; leer für die Wurzel selbst.
	Path string
	// Dir meldet einen Verweis, der auf „/" endet: ein Verzeichnis.
	Dir bool
}

// ParseDocRef zerlegt einen Verweis. Geprüft wird nur die Form, nicht ob es
// das Ziel gibt: ein gültiger project:- oder keph:-Verweis ist hier kein
// Fehler, auch wenn die Auflösung ihn noch nicht unterstützt.
func ParseDocRef(raw string) (DocRef, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return DocRef{}, docRefErrorf(DocCodeInvalidInput, "kein Verweis angegeben")
	}
	scheme, rest, found := strings.Cut(raw, ":")
	if !found {
		return DocRef{}, docRefErrorf(DocCodeInvalidInput,
			"%s ist kein Verweis: es fehlt das Präfix project:, k-playbook: oder keph:", raw)
	}
	switch scheme {
	case DocSchemeProject, DocSchemeKPlaybook, DocSchemeKeph:
	default:
		return DocRef{}, docRefErrorf(DocCodeInvalidInput,
			"%s: unbekanntes Präfix %q — erlaubt sind project:, k-playbook: und keph:", raw, scheme+":")
	}
	if !strings.HasPrefix(rest, "/") {
		return DocRef{}, docRefErrorf(DocCodeInvalidInput,
			"%s: nach dem Präfix muss „/“ stehen, der Pfad beginnt an der Wurzel", raw)
	}
	if strings.Contains(rest, `\`) {
		return DocRef{}, docRefErrorf(DocCodeInvalidInput, "%s: Pfade trennen mit „/“, nicht mit „\\“", raw)
	}

	ref := DocRef{Raw: raw, Scheme: scheme}
	body := strings.TrimPrefix(rest, "/")
	if strings.HasSuffix(body, "/") || body == "" {
		ref.Dir = true
		body = strings.TrimSuffix(body, "/")
	}
	if body != "" {
		for _, segment := range strings.Split(body, "/") {
			switch {
			case segment == "":
				return DocRef{}, docRefErrorf(DocCodeInvalidInput, "%s: leeres Pfadsegment", raw)
			case segment == "." || segment == "..":
				return DocRef{}, docRefErrorf(DocCodeInvalidInput, "%s: %q ist im Pfad nicht erlaubt", raw, segment)
			case strings.HasPrefix(segment, "."):
				return DocRef{}, docRefErrorf(DocCodeInvalidInput, "%s: versteckte Segmente wie %q sind nicht erlaubt", raw, segment)
			}
		}
	}
	ref.Path = body
	return ref, nil
}

// String gibt den Verweis in seiner kanonischen Form zurück.
func (r DocRef) String() string {
	text := r.Scheme + ":/" + r.Path
	if r.Dir && r.Path != "" {
		text += "/"
	}
	return text
}

// ErrDocIsDir meldet eine DocSource, wenn unter einem Dateipfad ein
// Verzeichnis liegt.
var ErrDocIsDir = errors.New("ist ein Verzeichnis")

// DocSourceEntry ist ein Eintrag eines Verzeichnisses in einer DocSource.
type DocSourceEntry struct {
	Name string
	Dir  bool
}

// DocSource ist eine Ebene hinter k-playbook:-Verweisen. Pfade sind relativ,
// mit Schrägstrich getrennt und schon geprüft (ParseDocRef).
//
// List nennt die Einträge eines Verzeichnisses; Read liest eine Datei. Gibt es
// das Verzeichnis oder die Datei nicht, ist der Fehler fs.ErrNotExist (über
// errors.Is erkennbar) — auch dann, wenn der Pfad eine Datei wie ein
// Verzeichnis behandelt (…/a.md/x.md, …/a.md/); liegt unter dem Dateipfad ein
// Verzeichnis, ist er ErrDocIsDir.
type DocSource interface {
	List(dir string) ([]DocSourceEntry, error)
	Read(file string) ([]byte, error)
}

// DocLayers sind die beiden Ebenen: Dist ist das Mitgelieferte, Local das
// Projekteigene.
type DocLayers struct {
	Dist  DocSource
	Local DocSource
}

// ProjectDocLayers sind die Ebenen eines Projekts auf der Platte: die
// Installation k-playbook/ (deren oberste Ebene, nicht defaults/) und
// k-playbook-local/.
func ProjectDocLayers(projectDir string) DocLayers {
	return DocLayers{
		Dist:  NewDiskDocSource(PlaybookDir(projectDir)),
		Local: NewDiskDocSource(LocalDir(projectDir)),
	}
}

// NewDiskDocSource ist eine DocSource über ein Verzeichnis auf der Platte.
// Gelesen wird über fs.FS: Pfade, die aus der Wurzel herausführen, weist das
// schon von sich aus ab.
func NewDiskDocSource(root string) DocSource {
	return fsDocSource{fsys: os.DirFS(root)}
}

type fsDocSource struct {
	fsys fs.FS
}

func (s fsDocSource) List(dir string) ([]DocSourceEntry, error) {
	if dir == "" {
		dir = "."
	}
	info, err := fs.Stat(s.fsys, dir)
	if err != nil {
		return nil, notExistForFileAsDir(err)
	}
	if !info.IsDir() {
		// Ein Verzeichnis-Verweis auf eine Datei: das Verzeichnis gibt es
		// in dieser Ebene nicht.
		return nil, &fs.PathError{Op: "readdir", Path: dir, Err: fs.ErrNotExist}
	}
	entries, err := fs.ReadDir(s.fsys, dir)
	if err != nil {
		return nil, notExistForFileAsDir(err)
	}
	listed := make([]DocSourceEntry, 0, len(entries))
	for _, entry := range entries {
		isDir := entry.IsDir()
		if entry.Type()&fs.ModeSymlink != 0 {
			// Ein verlinkter Eintrag zählt als das, worauf er zeigt.
			if info, err := fs.Stat(s.fsys, path.Join(dir, entry.Name())); err == nil {
				isDir = info.IsDir()
			}
		}
		listed = append(listed, DocSourceEntry{Name: entry.Name(), Dir: isDir})
	}
	return listed, nil
}

func (s fsDocSource) Read(file string) ([]byte, error) {
	info, err := fs.Stat(s.fsys, file)
	if err != nil {
		return nil, notExistForFileAsDir(err)
	}
	if info.IsDir() {
		return nil, ErrDocIsDir
	}
	data, err := fs.ReadFile(s.fsys, file)
	if err != nil {
		return nil, notExistForFileAsDir(err)
	}
	return data, nil
}

// notExistForFileAsDir macht aus dem Fehler eines Pfads, der eine Datei wie
// ein Verzeichnis behandelt (…/a.md/x.md), fs.ErrNotExist: den Eintrag gibt es
// in dieser Ebene nicht. Unter Linux meldet das System dafür ENOTDIR, das
// errors.Is nicht als fs.ErrNotExist erkennt; Windows meldet schon
// ERROR_PATH_NOT_FOUND. Alle anderen Fehler bleiben unverändert.
func notExistForFileAsDir(err error) error {
	if !errors.Is(err, syscall.ENOTDIR) {
		return err
	}
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return &fs.PathError{Op: pathErr.Op, Path: pathErr.Path, Err: fs.ErrNotExist}
	}
	return fs.ErrNotExist
}

// DocPart ist ein Teil einer Antwort: bei einer Einzeldatei genau sie, bei
// einem Verzeichnis jeder Teil, der zusammengebaut wurde oder abgeschaltet
// ist.
type DocPart struct {
	Name     string `json:"name"`
	Origin   string `json:"origin"`
	Disabled bool   `json:"disabled,omitempty"`
}

// DocResult ist ein aufgelöster Verweis.
type DocResult struct {
	// Ref ist der Verweis in kanonischer Form.
	Ref string `json:"ref"`
	// Directory meldet einen zusammengebauten Verzeichnis-Verweis.
	Directory bool `json:"directory"`
	// Content ist der Inhalt: die Einzeldatei unverändert mit Frontmatter,
	// beim Verzeichnis die Teile ohne Frontmatter, durch eine Leerzeile
	// getrennt. Leer bei einer abgeschalteten Datei und einem Verzeichnis
	// ohne aufgenommene Teile.
	Content string `json:"content"`
	// Disabled meldet eine lokal abgeschaltete Einzeldatei. Kein Fehler.
	Disabled bool `json:"disabled"`
	// Hint erklärt Disabled.
	Hint string `json:"hint,omitempty"`
	// Parts nennt die beteiligten Teile in der Reihenfolge des Inhalts,
	// abgeschaltete eingeschlossen.
	Parts []DocPart `json:"parts"`
	// Problems nennt, was übergangen wurde — etwa eine Datei ohne Ziffer vorn
	// in einem zusammengebauten Verzeichnis.
	Problems []string `json:"problems"`
}

// ResolveDocRef löst einen Verweis über die beiden Ebenen auf.
func ResolveDocRef(layers DocLayers, raw string) (DocResult, error) {
	ref, err := ParseDocRef(raw)
	if err != nil {
		return DocResult{}, err
	}
	switch ref.Scheme {
	case DocSchemeProject:
		return DocResult{}, docRefErrorf(DocCodeUnsupported,
			"%s: project:-Verweise löst k-playbook noch nicht auf — die Datei liegt im Projekt und wird direkt von der Platte gelesen", ref.Raw)
	case DocSchemeKeph:
		return DocResult{}, docRefErrorf(DocCodeUnsupported,
			"%s: keph:-Verweise löst k-playbook noch nicht auf — gelesen wird über Kephalaion read mit collection = kephalaion.address aus k-playbook context", ref.Raw)
	}
	if ref.Path != docInstructionsRoot && !strings.HasPrefix(ref.Path, docInstructionsRoot+"/") {
		return DocResult{}, docRefErrorf(DocCodeUnsupported,
			"%s: k-playbook löst heute nur Verweise unter k-playbook:/%s/ auf", ref.Raw, docInstructionsRoot)
	}
	if ref.Dir {
		return resolveDocDir(layers, ref)
	}
	return resolveDocFile(layers, ref)
}

// resolveDocFile liefert eine Einzeldatei: lokal vor mitgeliefert, Inhalt
// unverändert mit Frontmatter. Eine leere lokale Datei schaltet ab — kein
// Fehler, sondern leerer Inhalt mit disabled.
func resolveDocFile(layers DocLayers, ref DocRef) (DocResult, error) {
	result := DocResult{Ref: ref.String(), Parts: []DocPart{}, Problems: []string{}}
	name := path.Base(ref.Path)

	local, localErr := readDocLayer(layers.Local, ref)
	if localErr != nil && !errors.Is(localErr, fs.ErrNotExist) {
		return DocResult{}, localErr
	}
	dist, distErr := readDocLayer(layers.Dist, ref)
	if distErr != nil && !errors.Is(distErr, fs.ErrNotExist) {
		return DocResult{}, distErr
	}
	localFound := localErr == nil
	distFound := distErr == nil

	switch {
	case localFound:
		part := DocPart{Name: name, Origin: "local"}
		if distFound {
			part.Origin = "override"
		}
		if isEmptyContent(local) {
			part.Disabled = true
			result.Disabled = true
			result.Hint = fmt.Sprintf("%s ist lokal abgeschaltet: der lokale Eintrag ist leer", result.Ref)
		} else {
			result.Content = string(local)
		}
		result.Parts = append(result.Parts, part)
	case distFound:
		result.Content = string(dist)
		result.Parts = append(result.Parts, DocPart{Name: name, Origin: "dist"})
	default:
		return DocResult{}, docRefErrorf(DocCodeNotFound,
			"%s ist unbekannt: weder lokal noch mitgeliefert vorhanden", result.Ref)
	}
	return result, nil
}

// readDocLayer liest die Datei eines Verweises aus einer Ebene. Ein
// Verzeichnis unter dem Dateipfad ist ein Eingabefehler: der Verweis hätte
// auf „/" enden müssen.
func readDocLayer(source DocSource, ref DocRef) ([]byte, error) {
	if source == nil {
		return nil, fs.ErrNotExist
	}
	data, err := source.Read(ref.Path)
	if errors.Is(err, ErrDocIsDir) {
		return nil, docRefErrorf(DocCodeInvalidInput,
			"%s ist ein Verzeichnis — ein Verzeichnis-Verweis endet auf „/“: %s/", ref.Raw, ref.String())
	}
	return data, err
}

// docDirPart ist ein Teil beim Zusammenbauen, vor dem Sortieren.
type docDirPart struct {
	DocPart
	number  string
	content []byte
}

// resolveDocDir baut ein Verzeichnis aus beiden Ebenen zusammen: nur Dateien
// mit Ziffer vorn, ohne Unterverzeichnisse und README.md, gleichnamig lokal
// ersetzt mitgeliefert, leer lokal schaltet ab, numerisch sortiert, ohne
// Frontmatter zusammengefügt.
func resolveDocDir(layers DocLayers, ref DocRef) (DocResult, error) {
	result := DocResult{Ref: ref.String(), Directory: true, Parts: []DocPart{}, Problems: []string{}}

	local, localFound, err := listDocLayer(layers.Local, ref.Path)
	if err != nil {
		return DocResult{}, err
	}
	dist, distFound, err := listDocLayer(layers.Dist, ref.Path)
	if err != nil {
		return DocResult{}, err
	}
	if !localFound && !distFound {
		return DocResult{}, docRefErrorf(DocCodeNotFound,
			"%s ist unbekannt: das Verzeichnis gibt es weder lokal noch mitgeliefert", result.Ref)
	}

	names := map[string]bool{}
	for _, name := range append(append([]string{}, local...), dist...) {
		names[name] = true
	}
	sortedNames := make([]string, 0, len(names))
	for name := range names {
		sortedNames = append(sortedNames, name)
	}
	sort.Strings(sortedNames)

	inLocal := toSet(local)
	inDist := toSet(dist)
	parts := make([]docDirPart, 0, len(sortedNames))
	for _, name := range sortedNames {
		number := leadingDigits(name)
		if number == "" {
			layer := "mitgeliefert"
			switch {
			case inLocal[name] && inDist[name]:
				layer = "lokal und mitgeliefert"
			case inLocal[name]:
				layer = "lokal"
			}
			result.Problems = append(result.Problems, fmt.Sprintf(
				"%s%s (%s): der Name beginnt nicht mit einer Ziffer — nicht aufgenommen", result.Ref, name, layer))
			continue
		}
		file := path.Join(ref.Path, name)
		part := docDirPart{DocPart: DocPart{Name: name}, number: number}
		if inLocal[name] {
			data, err := layers.Local.Read(file)
			if err != nil {
				return DocResult{}, fmt.Errorf("%s%s lokal lesen: %w", result.Ref, name, err)
			}
			part.Origin = "local"
			if inDist[name] {
				part.Origin = "override"
			}
			if isEmptyContent(data) {
				part.Disabled = true
			} else {
				part.content = data
			}
		} else {
			data, err := layers.Dist.Read(file)
			if err != nil {
				return DocResult{}, fmt.Errorf("%s%s mitgeliefert lesen: %w", result.Ref, name, err)
			}
			part.Origin = "dist"
			part.content = data
		}
		parts = append(parts, part)
	}

	sort.SliceStable(parts, func(i, j int) bool {
		if cmp := compareNumbers(parts[i].number, parts[j].number); cmp != 0 {
			return cmp < 0
		}
		return parts[i].Name < parts[j].Name
	})

	pieces := []string{}
	for _, part := range parts {
		result.Parts = append(result.Parts, part.DocPart)
		if part.Disabled {
			continue
		}
		body := strings.Trim(string(inventory.Body(part.content)), "\n")
		if strings.TrimSpace(body) == "" {
			continue
		}
		pieces = append(pieces, body)
	}
	if len(pieces) > 0 {
		result.Content = strings.Join(pieces, "\n\n") + "\n"
	}
	return result, nil
}

// listDocLayer nennt die Dateien eines Verzeichnisses einer Ebene, die beim
// Zusammenbauen in Frage kommen: keine Unterverzeichnisse, keine README.md,
// keine Dotfiles. Das zweite Ergebnis meldet, ob es das Verzeichnis gibt.
func listDocLayer(source DocSource, dir string) ([]string, bool, error) {
	if source == nil {
		return nil, false, nil
	}
	entries, err := source.List(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("k-playbook:/%s/ auflisten: %w", dir, err)
	}
	names := []string{}
	for _, entry := range entries {
		if entry.Dir || entry.Name == "README.md" || strings.HasPrefix(entry.Name, ".") {
			continue
		}
		names = append(names, entry.Name)
	}
	return names, true, nil
}

func toSet(names []string) map[string]bool {
	set := make(map[string]bool, len(names))
	for _, name := range names {
		set[name] = true
	}
	return set
}

// leadingDigits liefert die Ziffern am Anfang eines Namens, leer ohne.
func leadingDigits(name string) string {
	end := 0
	for end < len(name) && name[end] >= '0' && name[end] <= '9' {
		end++
	}
	return name[:end]
}

// compareNumbers vergleicht zwei Ziffernfolgen numerisch, ohne sie in eine
// Zahl zu wandeln — so gibt es keinen Überlauf: führende Nullen fallen weg,
// dann entscheidet die Länge, dann der Text.
func compareNumbers(a string, b string) int {
	a = strings.TrimLeft(a, "0")
	b = strings.TrimLeft(b, "0")
	if len(a) != len(b) {
		if len(a) < len(b) {
			return -1
		}
		return 1
	}
	return strings.Compare(a, b)
}
