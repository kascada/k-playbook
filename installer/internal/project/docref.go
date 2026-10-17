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
	"syscall"

	"github.com/kascada/k-playbook/installer/internal/inventory"
)

// Verweise im Präfixschema (docs/verweisschema.md) und ihre Auflösung.
//
// Ein Verweis ist ein einziger Pfad mit Präfix: project:/…, k-playbook:/… oder
// keph:/…. Aufgelöst wird hier nur k-playbook:, und davon die Sorten in
// docRefKinds. Hinter k-playbook: stehen zwei Ebenen — die mitgelieferte und
// die lokale —, die über eine austauschbare Quelle gelesen werden: heute die
// Platte, später womöglich Kephalaion. Wer aufruft, sieht davon nichts; er
// bekommt den Inhalt mit lokal und mitgeliefert schon gegeneinander aufgelöst.
//
// Die Überlagerungsregel ist eine für alle: overlayDocDir legt die beiden
// Ebenen eines Verzeichnisses übereinander — gleicher Name gewinnt lokal, ein
// leerer lokaler Eintrag schaltet ab —, und sowohl die Kataloge der
// Kontextausgabe (resolveCatalog) als auch die Verweise hier und das Briefing
// gehen darüber. Dieselbe Sorte kommt so über jeden Weg gleich heraus.
// instructions und guidelines sind trotzdem keine Sorten in catalogKinds():
// instructions erscheint nicht unter catalogs, und das Feld guidelines der
// Kontextausgabe liest bis zu deren Umstellung weiter nur lokal — die eine
// benannte Ausnahme (Task 086).

// Die Präfixe des Schemas.
const (
	DocSchemeProject   = "project"
	DocSchemeKPlaybook = "k-playbook"
	DocSchemeKeph      = "keph"
)

// docInstructionsRoot ist die Sorte instructions/, die einzige mit einem
// zusammengebauten Verzeichnis.
const docInstructionsRoot = "instructions"

// docRefKinds sind die Sorten unter k-playbook:, die ResolveDocRef auflöst —
// die obersten Verzeichnisse beider Ebenen, deren Dateien Lesestoff für eine
// Sitzung sind. checks, commands und skills fehlen bewusst: Skripte laufen,
// Commands und Skills liegen beim Assistenten; beides liest niemand über
// einen Verweis.
var docRefKinds = []string{docInstructionsRoot, "rules", "reviews", "guidelines"}

// docAssembledDirs sind die Verzeichnisse, die ein Verweis mit „/" am Ende
// zusammenbaut — Pfade ab der Wurzel von k-playbook:, ohne Schrägstrich am
// Ende. Jedes andere Verzeichnis wird nur einzeln mit Dateinamen gelesen; ein
// Verweis darauf mit „/" am Ende ist unvollständig (invalid_input). Die Liste
// ist ein Wert im Code, keine Konfiguration: ein neuer Eintrag ist eine
// Vertragsänderung (docs/verweisschema.md).
var docAssembledDirs = []string{
	docInstructionsRoot + "/always",
}

// Die Fehlercodes der Auflösung. Die Hüllen — MCP und Subkommando — melden sie
// unverändert; alles ohne Code ist die Umgebung (read_failed).
const (
	// DocCodeInvalidInput: der Verweis ist kein gültiger Verweis im Schema —
	// kein Präfix, kein „/" danach, ein leeres, verstecktes oder ..-Segment —
	// oder er ist unvollständig: ein Verzeichnis-Verweis (mit „/" am Ende) auf
	// ein Verzeichnis, das nicht in docAssembledDirs steht. Dort fehlt der
	// Dateiname; entschieden wird das, ohne zu lesen.
	DocCodeInvalidInput = "invalid_input"
	// DocCodeUnsupported: der Verweis ist gültig, wird vom Werkzeug aber noch
	// nicht aufgelöst — project:, keph: und k-playbook: außerhalb der Sorten
	// in docRefKinds.
	DocCodeUnsupported = "unsupported"
	// DocCodeNotFound: der Verweis ist vollständig — eine Datei oder ein
	// zusammengebautes Verzeichnis —, aber es gibt ihn weder lokal noch
	// mitgeliefert. Kein Rückfall auf die Platte.
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
// Installation k-playbook/ (siehe NewShippedDocSource) und k-playbook-local/.
func ProjectDocLayers(projectDir string) DocLayers {
	return DocLayers{
		Dist:  NewShippedDocSource(PlaybookDir(projectDir)),
		Local: NewDiskDocSource(LocalDir(projectDir)),
	}
}

// defaultsDirName ist das Verzeichnis der Installation, das den Lieferumfang
// für den Hub trägt; sein Inhalt wird zum Release gepackt und landet unter
// <hub>:k-playbook/defaults/.
const defaultsDirName = "defaults"

// NewShippedDocSource ist die mitgelieferte Ebene einer Installation. Die
// Sorte instructions/ liegt nur in defaults/instructions/ — einmal, für Hub
// und Installation gleichermaßen; eine zweite Fassung oben im Repo, die mit
// der Kopie gleichzuhalten wäre, gibt es seit 2026-10-17 nicht mehr. Die
// übrigen Sorten (rules/, reviews/, guidelines/) lesen bis zu ihrer
// Umstellung weiter die oberste Ebene der Installation, weil auch die
// Kataloge der Kontextausgabe dort lesen.
func NewShippedDocSource(playbookDir string) DocSource {
	return shippedDocSource{
		top:      NewDiskDocSource(playbookDir),
		defaults: NewDiskDocSource(filepath.Join(playbookDir, defaultsDirName)),
	}
}

type shippedDocSource struct {
	top      DocSource
	defaults DocSource
}

// pick wählt die Quelle nach dem ersten Segment des Pfads: instructions/
// aus defaults/, alles andere von oben. Der Pfad bleibt derselbe, weil
// defaults/ denselben Aufbau hat.
func (s shippedDocSource) pick(p string) DocSource {
	first, _, _ := strings.Cut(p, "/")
	if first == docInstructionsRoot {
		return s.defaults
	}
	return s.top
}

func (s shippedDocSource) List(dir string) ([]DocSourceEntry, error) { return s.pick(dir).List(dir) }
func (s shippedDocSource) Read(file string) ([]byte, error)          { return s.pick(file).Read(file) }

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
	if !isDocRefKind(ref.Path) {
		return DocResult{}, docRefErrorf(DocCodeUnsupported,
			"%s: k-playbook löst heute nur Verweise unter %s auf", ref.Raw, docRefKindsText())
	}
	if ref.Dir {
		if !isAssembledDocDir(ref.Path) {
			return DocResult{}, missingDocFileName(ref)
		}
		return resolveDocDir(layers, ref)
	}
	return resolveDocFile(layers, ref)
}

// isDocRefKind meldet, ob ein Pfad in einer Sorte aus docRefKinds liegt.
func isDocRefKind(refPath string) bool {
	for _, kind := range docRefKinds {
		if refPath == kind || strings.HasPrefix(refPath, kind+"/") {
			return true
		}
	}
	return false
}

// docRefKindsText nennt die Sorten als Verweise, für Fehlermeldungen.
func docRefKindsText() string {
	refs := make([]string, 0, len(docRefKinds))
	for _, kind := range docRefKinds {
		refs = append(refs, DocSchemeKPlaybook+":/"+kind+"/")
	}
	return strings.Join(refs, ", ")
}

// isAssembledDocDir meldet, ob ein Verzeichnis in docAssembledDirs steht.
func isAssembledDocDir(dir string) bool {
	for _, assembled := range docAssembledDirs {
		if dir == assembled {
			return true
		}
	}
	return false
}

// missingDocFileName ist der Fehler für einen Verzeichnis-Verweis außerhalb
// von docAssembledDirs: es fehlt der Dateiname. Das Beispiel passt zum Fall —
// eine Datei mit „/" am Ende, ein Pfad unter einer Datei, die Wurzel
// instructions/ oder ein gewöhnliches Verzeichnis.
func missingDocFileName(ref DocRef) error {
	segments := strings.Split(ref.Path, "/")
	assembled := assembledDocDirsText()
	for i, segment := range segments {
		if !strings.HasSuffix(segment, ".md") {
			continue
		}
		file := DocRef{Scheme: ref.Scheme, Path: strings.Join(segments[:i+1], "/")}.String()
		if i == len(segments)-1 {
			return docRefErrorf(DocCodeInvalidInput,
				"%s: Dateiname fehlt — %s ist ein Dateiname, kein Verzeichnis; ohne „/“ am Ende: %s",
				ref.Raw, segment, file)
		}
		return docRefErrorf(DocCodeInvalidInput,
			"%s: Dateiname fehlt — unter der Datei %s liegt kein Verzeichnis; gemeint ist etwa %s",
			ref.Raw, segment, file)
	}
	if ref.Path == docInstructionsRoot {
		return docRefErrorf(DocCodeInvalidInput,
			"%s: Dateiname fehlt — %s/ wird nicht zusammengebaut; gemeint ist etwa %s:/%s/on-demand/<name>.md, zusammengebaut wird nur %s",
			ref.Raw, ref.Path, ref.Scheme, docInstructionsRoot, assembled)
	}
	return docRefErrorf(DocCodeInvalidInput,
		"%s: Dateiname fehlt — %s/ wird nicht zusammengebaut; gemeint ist etwa %s:/%s/<name>.md, zusammengebaut wird nur %s",
		ref.Raw, ref.Path, ref.Scheme, ref.Path, assembled)
}

// assembledDocDirsText nennt die zusammengebauten Verzeichnisse als Verweise.
func assembledDocDirsText() string {
	refs := make([]string, 0, len(docAssembledDirs))
	for _, dir := range docAssembledDirs {
		refs = append(refs, DocSchemeKPlaybook+":/"+dir+"/")
	}
	return strings.Join(refs, ", ")
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
		part := DocPart{Name: name, Origin: DocOriginLocal}
		if distFound {
			part.Origin = DocOriginOverride
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
		result.Parts = append(result.Parts, DocPart{Name: name, Origin: DocOriginDist})
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

// Die Herkunft eines Eintrags, wie sie in Katalogen, Verweisen und Briefing
// steht: nur mitgeliefert, nur lokal, oder lokal an Stelle eines
// gleichnamigen mitgelieferten.
const (
	DocOriginDist     = "dist"
	DocOriginLocal    = "local"
	DocOriginOverride = "override"
)

// docDirEntry ist ein Eintrag eines Verzeichnisses, über beide Ebenen
// aufgelöst: Name, Herkunft, ob lokal abgeschaltet, und der Inhalt der
// wirksamen Ebene — nil, wenn abgeschaltet.
type docDirEntry struct {
	Name     string
	Origin   string
	Disabled bool
	Content  []byte
}

// overlayDocDir legt die beiden Ebenen eines Verzeichnisses übereinander. Das
// ist die eine Überlagerungsregel für die Kataloge der Kontextausgabe
// (resolveCatalog), die zusammengebauten Verzeichnisse (resolveDocDir) und
// das Briefing: Vergleichseinheit ist der Dateiname, ein gleichnamiger lokaler
// Eintrag ersetzt den mitgelieferten vollständig (override), ein leerer
// lokaler schaltet ihn ab, Unterverzeichnisse, README.md und Dotfiles zählen
// nie. keep filtert die Namen weiter — etwa auf das Muster einer Sorte —; nil
// nimmt alle. Das zweite Ergebnis meldet, ob es das Verzeichnis in mindestens
// einer Ebene gibt. Die Einträge sind nach Namen sortiert.
func overlayDocDir(layers DocLayers, dir string, keep func(name string) bool) ([]docDirEntry, bool, error) {
	local, localFound, err := listDocLayer(layers.Local, dir)
	if err != nil {
		return nil, false, err
	}
	dist, distFound, err := listDocLayer(layers.Dist, dir)
	if err != nil {
		return nil, false, err
	}
	if !localFound && !distFound {
		return nil, false, nil
	}

	inLocal := toSet(local)
	inDist := toSet(dist)
	names := make([]string, 0, len(inLocal)+len(inDist))
	for name := range inLocal {
		names = append(names, name)
	}
	for name := range inDist {
		if !inLocal[name] {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	entries := make([]docDirEntry, 0, len(names))
	for _, name := range names {
		if keep != nil && !keep(name) {
			continue
		}
		file := path.Join(dir, name)
		entry := docDirEntry{Name: name}
		if inLocal[name] {
			data, err := layers.Local.Read(file)
			if err != nil {
				return nil, true, fmt.Errorf("%s:/%s lokal lesen: %w", DocSchemeKPlaybook, file, err)
			}
			entry.Origin = DocOriginLocal
			if inDist[name] {
				entry.Origin = DocOriginOverride
			}
			if isEmptyContent(data) {
				entry.Disabled = true
			} else {
				entry.Content = data
			}
		} else {
			data, err := layers.Dist.Read(file)
			if err != nil {
				return nil, true, fmt.Errorf("%s:/%s mitgeliefert lesen: %w", DocSchemeKPlaybook, file, err)
			}
			entry.Origin = DocOriginDist
			entry.Content = data
		}
		entries = append(entries, entry)
	}
	return entries, true, nil
}

// docOriginText nennt die Ebenen einer Herkunft im Klartext, für Meldungen.
func docOriginText(origin string) string {
	switch origin {
	case DocOriginOverride:
		return "lokal und mitgeliefert"
	case DocOriginLocal:
		return "lokal"
	}
	return "mitgeliefert"
}

// docDirPart ist ein Teil beim Zusammenbauen, vor dem Sortieren.
type docDirPart struct {
	DocPart
	number  string
	content []byte
}

// assembleDocDir ordnet ein Verzeichnis aus docAssembledDirs zum Zusammenbau:
// überlagert wie jedes Verzeichnis (overlayDocDir), dann nur Dateien mit
// Ziffer vorn, numerisch sortiert über beide Ebenen, bei gleicher Ziffer nach
// Namen. Dateien ohne Ziffer kommen als Problem zurück. Das dritte Ergebnis
// meldet, ob es das Verzeichnis in einer Ebene gibt. Wer den Text braucht,
// ruft resolveDocDir; das Briefing braucht die Teile mit Inhalt und Herkunft.
func assembleDocDir(layers DocLayers, dir string) ([]docDirPart, []string, bool, error) {
	ref := DocSchemeKPlaybook + ":/" + dir + "/"
	entries, found, err := overlayDocDir(layers, dir, nil)
	if err != nil || !found {
		return nil, nil, found, err
	}

	problems := []string{}
	parts := make([]docDirPart, 0, len(entries))
	for _, entry := range entries {
		number := leadingDigits(entry.Name)
		if number == "" {
			problems = append(problems, fmt.Sprintf(
				"%s%s (%s): der Name beginnt nicht mit einer Ziffer — nicht aufgenommen",
				ref, entry.Name, docOriginText(entry.Origin)))
			continue
		}
		parts = append(parts, docDirPart{
			DocPart: DocPart{Name: entry.Name, Origin: entry.Origin, Disabled: entry.Disabled},
			number:  number,
			content: entry.Content,
		})
	}

	sort.SliceStable(parts, func(i, j int) bool {
		if cmp := compareNumbers(parts[i].number, parts[j].number); cmp != 0 {
			return cmp < 0
		}
		return parts[i].Name < parts[j].Name
	})
	return parts, problems, true, nil
}

// resolveDocDir baut ein Verzeichnis aus docAssembledDirs aus beiden Ebenen
// zusammen (geprüft hat das ResolveDocRef): die Teile aus assembleDocDir, ohne
// Frontmatter, durch eine Leerzeile getrennt.
func resolveDocDir(layers DocLayers, ref DocRef) (DocResult, error) {
	result := DocResult{Ref: ref.String(), Directory: true, Parts: []DocPart{}, Problems: []string{}}

	parts, problems, found, err := assembleDocDir(layers, ref.Path)
	if err != nil {
		return DocResult{}, err
	}
	if !found {
		return DocResult{}, docRefErrorf(DocCodeNotFound,
			"%s ist unbekannt: das Verzeichnis gibt es weder lokal noch mitgeliefert", result.Ref)
	}
	result.Problems = append(result.Problems, problems...)

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
