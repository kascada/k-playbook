package project

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"syscall"

	"github.com/kascada/k-playbook/installer/internal/inventory"
	"github.com/kascada/k-playbook/installer/internal/kephalaion"
)

// Verweise im Präfixschema (docs/verweisschema.md) und ihre Auflösung.
//
// Ein Verweis ist ein einziger Pfad mit Präfix: project:/…, k-playbook:/… oder
// keph:/…. ResolveDocRef löst alle drei auf — die eine Funktion hinter
// `k-playbook read` und k_playbook_doc_read. project: liest eine Datei des
// Projekts ab seiner Wurzel, keph: ein Dokument der Projekt-Collection über
// den Node; beide haben eine Ebene, ohne Überlagerung und Zusammenbau.
// k-playbook: löst die Sorten in docRefKinds auf; dahinter stehen zwei Ebenen
// — die mitgelieferte und die lokale —, die über eine austauschbare Quelle
// gelesen werden: heute die Platte, später womöglich Kephalaion. Wer aufruft,
// sieht davon nichts; er bekommt den Inhalt mit lokal und mitgeliefert schon
// gegeneinander aufgelöst.
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

// docCommandsRoot ist die Sorte commands/: der Text eines Commands. Beim
// Assistenten liegt nur noch eine Hülle (Frontmatter plus Verweis
// k-playbook:/commands/<name>.md), den Text löst ResolveDocRef auf — zuerst
// für /k-upgrade (Task 092).
const docCommandsRoot = "commands"

// docIncludesRoot ist die Sorte includes/: Bausteine für Include-Zeilen, aus
// einem Command oder einer Anweisung nur über den absoluten Verweis
// k-playbook:/includes/<name>.md erreichbar, weil kein relativer Pfad aus
// der eigenen Sorte führt. Eine lokale Fassung eines Bausteins gewinnt.
const docIncludesRoot = "includes"

// docTestdataRoot ist die Sorte testdata/: feste Testfälle, die auf jeder
// Installation gleich ausgehen — die Go-Tests lesen sie von der Platte, nach
// dem Release liegen sie auf jedem Hub (defaults/testdata/README.md).
const docTestdataRoot = "testdata"

// docReaderKinds sind die Sorten unter k-playbook:, deren Dateien Lesestoff
// für eine Sitzung sind — die, die Hilfetext und Werkzeugbeschreibung nennen.
// checks und skills fehlen bewusst: Skripte laufen, Skills liegen beim
// Assistenten; beides liest niemand über einen Verweis. Commands liegen beim
// Assistenten nur noch als Hülle; ihr Text ist Lesestoff.
var docReaderKinds = []string{docInstructionsRoot, "rules", "reviews", "guidelines", docCommandsRoot}

// docRefKinds sind alle Sorten, die ResolveDocRef auflöst: die Lesersorten
// und dazu includes/ und testdata/, die in keinem Katalog, keinem Briefing
// und keiner Hilfe erscheinen.
var docRefKinds = append(append([]string{}, docReaderKinds...), docIncludesRoot, docTestdataRoot)

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
	// DocCodeUnsupported: der Verweis ist gültig, wird vom Werkzeug aber nicht
	// aufgelöst — k-playbook: außerhalb der Sorten in docRefKinds.
	DocCodeUnsupported = "unsupported"
	// DocCodeNotFound: der Verweis ist vollständig — eine Datei oder ein
	// zusammengebautes Verzeichnis —, aber es gibt ihn nicht: weder lokal
	// noch mitgeliefert, nicht im Projekt, nicht in der Collection. Kein
	// Rückfall auf eine andere Quelle.
	DocCodeNotFound = "not_found"
	// DocCodeIncludeCycle: ein Include-Ziel ist in der Kette der gerade
	// offenen Dokumente schon enthalten — a bindet b ein, b wieder a.
	// Erkannt, sobald die Zeile gelesen ist, nicht erst an der Tiefengrenze.
	DocCodeIncludeCycle = "include_cycle"
	// DocCodeIncludeTooDeep: ein Include läge tiefer als docIncludeMaxDepth.
	DocCodeIncludeTooDeep = "include_too_deep"
)

// docIncludeMaxDepth ist die größte Tiefe eines Includes; das Hauptdokument
// hat Tiefe 0, was es einbindet Tiefe 1. Ein Include auf Tiefe 11 ist
// include_too_deep.
const docIncludeMaxDepth = 10

// docIncludePrefix ist der Anfang einer Include-Zeile: ohne führenden
// Leerraum, danach genau ein Argument — ein Verweis mit Präfix oder ein
// relativer Pfad ohne — und sonst nichts. Kein HTML-Kommentar: Ein nicht
// eingesetztes Include muss im Rohtext lesbar bleiben. „@“ und „!“ am
// Zeilenanfang wären tabu, Claude Code deutet sie um.
const docIncludePrefix = "include:"

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

// DocLayers sind die Quellen hinter den drei Präfixen. Hinter k-playbook:
// stehen zwei Ebenen, Dist das Mitgelieferte und Local das Projekteigene;
// hinter project: steht das Projekt selbst, eine Ebene ohne Überlagerung,
// und hinter keph: die Collection des Projekts, geöffnet erst, wenn ein
// keph:-Verweis sie braucht — der Zugang fragt den Node.
type DocLayers struct {
	Dist  DocSource
	Local DocSource
	// Project ist die Wurzel des Projekts (das Verzeichnis von
	// K-PLAYBOOK.yaml); nil, wenn es keine gibt — dann ist jeder
	// project:-Verweis not_found.
	Project DocSource
	// Keph öffnet den Zugang zur Collection; nil, wenn es keinen gibt — dann
	// ist jeder keph:-Verweis kephalaion_unavailable.
	Keph func() (KephDocs, error)
}

// ProjectDocLayers sind die Quellen eines Projekts auf der Platte: die
// Installation k-playbook/ (siehe NewShippedDocSource), k-playbook-local/,
// das Projekt selbst ab seiner Wurzel und, auf Abruf, seine Collection
// (OpenKephDocs).
func ProjectDocLayers(projectDir string) DocLayers {
	return DocLayers{
		Dist:    NewShippedDocSource(PlaybookDir(projectDir)),
		Local:   NewDiskDocSource(LocalDir(projectDir)),
		Project: NewDiskDocSource(projectDir),
		Keph:    memoKephDocs(func() (KephDocs, error) { return OpenKephDocs(projectDir) }),
	}
}

// memoKephDocs öffnet den Zugang höchstens einmal je DocLayers — auch das
// Ergebnis „nicht bereit“ bleibt stehen. Der Zugang fragt den Node; mehrere
// keph:-Verweise in einer Auflösung (Includes) sollen das nicht wiederholen.
func memoKephDocs(open func() (KephDocs, error)) func() (KephDocs, error) {
	var once sync.Once
	var docs KephDocs
	var err error
	return func() (KephDocs, error) {
		once.Do(func() { docs, err = open() })
		return docs, err
	}
}

// defaultsDirName ist das Verzeichnis der Installation, das den Lieferumfang
// für den Hub trägt; sein Inhalt wird zum Release gepackt und landet unter
// <hub>:k-playbook/defaults/.
const defaultsDirName = "defaults"

// NewShippedDocSource ist die mitgelieferte Ebene einer Installation. Die
// Sorte instructions/ liegt nur in defaults/instructions/ — einmal, für Hub
// und Installation gleichermaßen; eine zweite Fassung oben im Repo, die mit
// der Kopie gleichzuhalten wäre, gibt es seit 2026-10-17 nicht mehr. Ebenso
// liest die Sorte commands/ aus defaults/commands/: dort liegt der Text eines
// Commands, oben in commands/ nur seine Hülle für den Assistenten — zwei
// verschiedene Dateien, keine Kopie. includes/ und testdata/ gibt es nur in
// defaults/. Die übrigen Sorten (rules/, reviews/, guidelines/) lesen bis zu
// ihrer Umstellung weiter die oberste Ebene der Installation, weil auch die
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

// pick wählt die Quelle nach dem ersten Segment des Pfads: instructions/,
// commands/, includes/ und testdata/ aus defaults/, alles andere von oben.
// Der Pfad bleibt derselbe, weil defaults/ denselben Aufbau hat.
func (s shippedDocSource) pick(p string) DocSource {
	first, _, _ := strings.Cut(p, "/")
	switch first {
	case docInstructionsRoot, docCommandsRoot, docIncludesRoot, docTestdataRoot:
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
	// Problems nennt, was übergangen wurde — eine Datei ohne Ziffer vorn in
	// einem zusammengebauten Verzeichnis, oder ein Teil davon, dessen Include
	// scheiterte (mit Kette).
	Problems []string `json:"problems"`
	// Includes nennt die eingesetzten Ziele in Lesereihenfolge, mit Tiefe;
	// leer mit DocReadOptions.Raw. Woraus ein Text zusammengesetzt wurde.
	Includes []DocInclude `json:"includes"`
}

// DocInclude ist ein eingesetztes Include-Ziel.
type DocInclude struct {
	// Ref ist das Ziel in kanonischer Form, aufgelöst — relativ gegen das
	// einbindende Dokument, ~ durch den User ersetzt.
	Ref string `json:"ref"`
	// Origin ist die Herkunft des Ziels: dist, local, override, project oder
	// keph. Leer bei einem zusammengebauten Verzeichnis als Ziel; seine
	// Teile stehen nicht einzeln hier.
	Origin string `json:"origin,omitempty"`
	// Depth ist die Tiefe: 1 für ein Include des Hauptdokuments.
	Depth int `json:"depth"`
	// Disabled meldet ein lokal abgeschaltetes Ziel: nichts eingesetzt, kein
	// Fehler.
	Disabled bool `json:"disabled,omitempty"`
}

// DocReadOptions steuern die Auflösung. Der Nullwert ist der Standard.
type DocReadOptions struct {
	// Raw liefert den Rohtext: Include-Zeilen bleiben stehen, nichts wird
	// eingesetzt. Standard ist einsetzen.
	Raw bool
}

// ResolveDocRef löst einen Verweis mit den Standardoptionen auf: Includes
// werden eingesetzt. ctx ist der Hintergrund — für Aufrufer, die keinen
// haben (Briefing, Subkommando).
func ResolveDocRef(layers DocLayers, raw string) (DocResult, error) {
	return ResolveDocRefWith(context.Background(), layers, raw, DocReadOptions{})
}

// ResolveDocRefWith löst einen Verweis auf — jedes der drei Präfixe — und
// setzt Includes ein, wenn opts es nicht abschaltet. ctx begrenzt nur den
// Weg zur Collection (keph:); die Platte kennt keinen Abbruch.
//
// Zwei Fehlerebenen bei Includes: Bei einer Datei ist ein gescheitertes
// Include der Fehler des ganzen Lesevorgangs, ohne Teilergebnis. In einem
// zusammengebauten Verzeichnis landet der Fehler eines Teils mit Kette unter
// problems, der Teil wird ausgelassen, die übrigen bleiben — eine fehlerhafte
// Include-Zeile in einer lokalen Always-Anweisung legt den Sitzungsstart
// nicht lahm, und nichts scheitert still.
func ResolveDocRefWith(ctx context.Context, layers DocLayers, raw string, opts DocReadOptions) (DocResult, error) {
	ref, err := ParseDocRef(raw)
	if err != nil {
		return DocResult{}, err
	}
	x := &docIncluder{ctx: ctx, layers: layers, opts: opts}
	var result DocResult
	switch ref.Scheme {
	case DocSchemeProject:
		result, err = resolveProjectDoc(layers, ref)
	case DocSchemeKeph:
		result, err = resolveKephDoc(ctx, layers, ref)
	default:
		if !isDocRefKind(ref.Path) {
			return DocResult{}, docRefErrorf(DocCodeUnsupported,
				"%s: k-playbook löst nur Verweise unter %s auf", ref.Raw, docRefKindsText())
		}
		if ref.Dir {
			if !isAssembledDocDir(ref.Path) {
				return DocResult{}, missingDocFileName(ref)
			}
			return x.resolveDir(ref)
		}
		result, err = resolveDocFile(layers, ref)
	}
	if err != nil {
		return DocResult{}, err
	}
	return x.finishFile(result, ref)
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

// docRefKindsText nennt die Lesersorten als Verweise, für Fehlermeldungen —
// die Meldung ist Hilfe und nennt nur, was eine Sitzung liest; includes/ und
// testdata/ löst isDocRefKind trotzdem auf.
func docRefKindsText() string {
	refs := make([]string, 0, len(docReaderKinds))
	for _, kind := range docReaderKinds {
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

// resolveProjectDoc liest eine Datei des Projekts ab seiner Wurzel: eine
// Ebene, keine Überlagerung, kein Zusammenbau. Eine leere Datei ist leerer
// Inhalt, nicht abgeschaltet — es gibt nichts, was sie abschalten könnte. Ein
// Verweis mit „/" am Ende ist unvollständig: project: kennt keine
// zusammengebauten Verzeichnisse.
func resolveProjectDoc(layers DocLayers, ref DocRef) (DocResult, error) {
	if ref.Dir {
		return DocResult{}, docRefErrorf(DocCodeInvalidInput,
			"%s: Dateiname fehlt — project: liest einzelne Dateien, kein Verzeichnis wird zusammengebaut; gemeint ist etwa %s<name>.md",
			ref.Raw, ref.String())
	}
	data, err := readDocLayer(layers.Project, ref)
	if errors.Is(err, fs.ErrNotExist) {
		return DocResult{}, docRefErrorf(DocCodeNotFound,
			"%s ist unbekannt: die Datei gibt es im Projekt nicht", ref.String())
	}
	if err != nil {
		return DocResult{}, err
	}
	return DocResult{
		Ref:      ref.String(),
		Content:  string(data),
		Parts:    []DocPart{{Name: path.Base(ref.Path), Origin: DocOriginProject}},
		Problems: []string{},
	}, nil
}

// resolveKephDoc liest ein Dokument der Projekt-Collection über den Node:
// eine Ebene, keine Überlagerung, kein Zusammenbau. Der Zugang kommt aus
// layers.Keph (Status, Anbindung, Client — wie OpenUserDocs); ist er nicht
// zu haben, ist das kephalaion_unavailable. Ein ganzer Pfadteil ~ steht für
// kephalaion.user; der Verweis in der Antwort trägt den echten Namen. Was
// read als none meldet, ist not_found; ein Verzeichnis ist ein Eingabefehler.
// Fehler des Clients gehen durch kephError.
func resolveKephDoc(ctx context.Context, layers DocLayers, ref DocRef) (DocResult, error) {
	if ref.Dir {
		return DocResult{}, docRefErrorf(DocCodeInvalidInput,
			"%s: Dateiname fehlt — keph: liest einzelne Dokumente, kein Verzeichnis wird zusammengebaut; gemeint ist etwa %s<name>.md",
			ref.Raw, ref.String())
	}
	if layers.Keph == nil {
		return DocResult{}, docRefErrorf(DocCodeKephalaionUnavailable,
			"%s: in dieser Auflösung gibt es keinen Zugang zur Collection", ref.Raw)
	}
	docs, err := layers.Keph()
	if err != nil {
		return DocResult{}, err
	}
	segments := strings.Split(ref.Path, "/")
	for i, segment := range segments {
		if segment != docUserSegment {
			continue
		}
		if !docNamePattern.MatchString(docs.User) {
			return DocResult{}, docRefErrorf(DocCodeKephalaionUnavailable,
				"%s: ~ steht für den eigenen User, aber Kephalaion nennt keinen gültigen (%q)", ref.Raw, docs.User)
		}
		segments[i] = docs.User
	}
	name := strings.Join(segments, "/")
	canonical := DocRef{Scheme: DocSchemeKeph, Path: name}.String()

	read, err := docs.Store.Read(ctx, name)
	if err != nil {
		return DocResult{}, kephError(err)
	}
	switch read.Kind {
	case kephalaion.ReadKindNone:
		return DocResult{}, docRefErrorf(DocCodeNotFound,
			"%s ist unbekannt: in der Collection %s gibt es das Dokument nicht", canonical, read.Address)
	case kephalaion.ReadKindDirectory:
		return DocResult{}, docRefErrorf(DocCodeInvalidInput,
			"%s ist ein Verzeichnis — keph: liest einzelne Dokumente; gemeint ist etwa %s/<name>.md", canonical, canonical)
	}
	content := ""
	if read.Content != nil {
		content = *read.Content
	}
	return DocResult{
		Ref:      canonical,
		Content:  content,
		Parts:    []DocPart{{Name: path.Base(name), Origin: DocOriginKeph}},
		Problems: []string{},
	}, nil
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
// gleichnamigen mitgelieferten; dazu für die anderen Präfixe das Projekt und
// die Collection, die beide nur eine Ebene haben.
const (
	DocOriginDist     = "dist"
	DocOriginLocal    = "local"
	DocOriginOverride = "override"
	DocOriginProject  = "project"
	DocOriginKeph     = "keph"
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

// docDirPart ist ein Teil beim Zusammenbauen, vor dem Sortieren. content ist
// die Datei, wie sie in der Ebene liegt (mit Frontmatter — das Briefing liest
// daraus Titel und Beschreibung), body ihr Rumpf ohne Frontmatter, Includes
// schon eingesetzt; leer bei einem abgeschalteten Teil.
type docDirPart struct {
	DocPart
	number  string
	content []byte
	body    string
}

// assembleDocDir ordnet ein Verzeichnis aus docAssembledDirs zum Zusammenbau:
// überlagert wie jedes Verzeichnis (overlayDocDir), dann nur Dateien mit
// Ziffer vorn, numerisch sortiert über beide Ebenen, bei gleicher Ziffer nach
// Namen, in jedem Teil die Includes eingesetzt. Dateien ohne Ziffer und Teile,
// deren Include scheiterte, kommen als Problem zurück und fehlen in den
// Teilen. Das vierte Ergebnis meldet, ob es das Verzeichnis in einer Ebene
// gibt. Wer den Text braucht, ruft ResolveDocRef; das Briefing braucht die
// Teile mit Inhalt und Herkunft und baut denselben Text (joinDocDirParts).
func assembleDocDir(ctx context.Context, layers DocLayers, dir string, opts DocReadOptions) ([]docDirPart, []DocInclude, []string, bool, error) {
	x := &docIncluder{ctx: ctx, layers: layers, opts: opts}
	parts, found, err := x.assembleDir(dir, nil, 0)
	return parts, x.includes, x.problems, found, err
}

// joinDocDirParts fügt die Teile eines zusammengebauten Verzeichnisses zu dem
// einen Text: die Rümpfe ohne Frontmatter, abgeschaltete und leere Teile
// übergangen, durch eine Leerzeile getrennt. Leer ohne aufgenommene Teile.
// Die eine Stelle für diesen Text — `k-playbook read` und das Briefing
// liefern denselben.
func joinDocDirParts(parts []docDirPart) string {
	pieces := []string{}
	for _, part := range parts {
		if part.Disabled || strings.TrimSpace(part.body) == "" {
			continue
		}
		pieces = append(pieces, part.body)
	}
	if len(pieces) == 0 {
		return ""
	}
	return strings.Join(pieces, "\n\n") + "\n"
}

// docIncluder setzt Include-Zeilen ein. Eine Instanz je Lesevorgang: sie
// sammelt die eingesetzten Ziele (includes) und die Probleme zusammengebauter
// Verzeichnisse (problems) über alle Tiefen hinweg.
type docIncluder struct {
	ctx      context.Context
	layers   DocLayers
	opts     DocReadOptions
	includes []DocInclude
	problems []string
}

// finishFile schließt das Lesen einer Einzeldatei ab: Includes einsetzen,
// außer mit Raw oder bei einer abgeschalteten Datei. Ein gescheitertes Include
// ist der Fehler des ganzen Lesevorgangs.
func (x *docIncluder) finishFile(result DocResult, ref DocRef) (DocResult, error) {
	result.Includes = []DocInclude{}
	if x.opts.Raw || result.Disabled || result.Content == "" {
		return result, nil
	}
	content, err := x.expand(result.Content, ref, []string{result.Ref}, 0, true)
	if err != nil {
		return DocResult{}, err
	}
	result.Content = content
	result.Includes = append(result.Includes, x.includes...)
	result.Problems = append(result.Problems, x.problems...)
	return result, nil
}

// resolveDir baut ein Verzeichnis aus docAssembledDirs aus beiden Ebenen
// zusammen (geprüft hat das ResolveDocRefWith): die Teile aus assembleDir,
// ohne Frontmatter, durch eine Leerzeile getrennt.
func (x *docIncluder) resolveDir(ref DocRef) (DocResult, error) {
	result := DocResult{Ref: ref.String(), Directory: true, Parts: []DocPart{}, Problems: []string{}, Includes: []DocInclude{}}
	parts, found, err := x.assembleDir(ref.Path, nil, 0)
	if err != nil {
		return DocResult{}, err
	}
	if !found {
		return DocResult{}, docRefErrorf(DocCodeNotFound,
			"%s ist unbekannt: das Verzeichnis gibt es weder lokal noch mitgeliefert", result.Ref)
	}
	for _, part := range parts {
		result.Parts = append(result.Parts, part.DocPart)
	}
	result.Content = joinDocDirParts(parts)
	result.Problems = append(result.Problems, x.problems...)
	result.Includes = append(result.Includes, x.includes...)
	return result, nil
}

// assembleDir ist der Zusammenbau hinter assembleDocDir: chain sind die
// gerade offenen Verweise (leer beim Hauptverzeichnis), depth die Tiefe des
// Verzeichnisses. Jeder Teil setzt seine Includes für sich ein; scheitert
// eines, wird der Teil mit Kette unter problems gemeldet und ausgelassen.
func (x *docIncluder) assembleDir(dir string, chain []string, depth int) ([]docDirPart, bool, error) {
	ref := DocSchemeKPlaybook + ":/" + dir + "/"
	entries, found, err := overlayDocDir(x.layers, dir, nil)
	if err != nil || !found {
		return nil, found, err
	}

	parts := make([]docDirPart, 0, len(entries))
	for _, entry := range entries {
		number := leadingDigits(entry.Name)
		if number == "" {
			x.problems = append(x.problems, fmt.Sprintf(
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

	kept := make([]docDirPart, 0, len(parts))
	dirChain := append(append([]string{}, chain...), ref)
	for _, part := range parts {
		if part.Disabled {
			kept = append(kept, part)
			continue
		}
		part.body = docBody(part.content)
		if !x.opts.Raw {
			partRef := DocRef{Scheme: DocSchemeKPlaybook, Path: dir + "/" + part.Name}
			body, err := x.expand(part.body, partRef, append(append([]string{}, dirChain...), partRef.String()), depth, false)
			if err != nil {
				x.problems = append(x.problems, fmt.Sprintf("%s%s (%s): Include nicht aufgelöst — nicht aufgenommen: %v",
					ref, part.Name, docOriginText(part.Origin), err))
				continue
			}
			part.body = body
		}
		kept = append(kept, part)
	}
	return kept, true, nil
}

// docBody ist der Rumpf eines Dokuments ohne Frontmatter und ohne Leerzeilen
// am Rand — das, was an Stelle einer Include-Zeile eingesetzt wird und woraus
// ein zusammengebautes Verzeichnis besteht.
func docBody(content []byte) string {
	return strings.Trim(string(inventory.Body(content)), "\n")
}

// expand setzt die Include-Zeilen eines Texts ein. ref ist das Dokument, zu
// dem der Text gehört — relative Ziele zählen ab seinem Verzeichnis im selben
// Präfix —, chain die gerade offenen Verweise einschließlich ref, depth die
// Tiefe des Dokuments. withFrontmatter überspringt einen Frontmatter-Block am
// Anfang (der Text einer Einzeldatei kommt mit ihm; ein Rumpf nicht, und ein
// Rumpf darf mit einer Trennlinie beginnen). Innerhalb eines Code-Blocks wird
// nichts eingesetzt. Ein Fehler trägt die Kette.
func (x *docIncluder) expand(text string, ref DocRef, chain []string, depth int, withFrontmatter bool) (string, error) {
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	start := 0
	if withFrontmatter {
		start = frontmatterLines(lines)
		out = append(out, lines[:start]...)
	}
	var fenceChar byte
	fenceLen := 0
	for _, line := range lines[start:] {
		if char, n, rest := fenceMarker(line); n > 0 {
			switch {
			case fenceLen == 0:
				fenceChar, fenceLen = char, n
			case char == fenceChar && n >= fenceLen && strings.TrimSpace(rest) == "":
				fenceLen = 0
			}
			out = append(out, line)
			continue
		}
		if fenceLen > 0 || !strings.HasPrefix(line, docIncludePrefix) {
			out = append(out, line)
			continue
		}
		inserted, err := x.includeLine(strings.TrimSpace(line[len(docIncludePrefix):]), ref, chain, depth)
		if err != nil {
			return "", err
		}
		if inserted == "" {
			// Nichts einzusetzen (abgeschaltet oder leer): die Zeile fällt
			// weg, keine Leerzeile bleibt zurück.
			continue
		}
		out = append(out, strings.Split(inserted, "\n")...)
	}
	return strings.Join(out, "\n"), nil
}

// includeLine löst das Argument einer Include-Zeile auf und liefert den
// Text, der an ihre Stelle tritt. Jeder Fehler trägt die Kette bis zum Ziel.
func (x *docIncluder) includeLine(arg string, from DocRef, chain []string, depth int) (string, error) {
	fail := func(target string, err error) error {
		return chainIncludeError(err, append(append([]string{}, chain...), target))
	}
	if arg == "" || strings.ContainsAny(arg, " \t") {
		return "", fail(arg, docRefErrorf(DocCodeInvalidInput,
			"%s: eine Include-Zeile trägt genau ein Argument, einen Verweis mit Präfix oder einen relativen Pfad: include: %q",
			from.String(), arg))
	}
	target, err := ParseDocRef(includeTarget(arg, from))
	if err != nil {
		return "", fail(arg, err)
	}
	canonical := target.String()
	if slices.Contains(chain, canonical) {
		return "", fail(canonical, docRefErrorf(DocCodeIncludeCycle,
			"%s bindet %s ein, das schon offen ist — ein Include-Zyklus", from.String(), canonical))
	}
	if depth+1 > docIncludeMaxDepth {
		return "", fail(canonical, docRefErrorf(DocCodeIncludeTooDeep,
			"%s bindet %s auf Tiefe %d ein; erlaubt ist bis Tiefe %d", from.String(), canonical, depth+1, docIncludeMaxDepth))
	}
	inserted, err := x.include(target, append(append([]string{}, chain...), canonical), depth+1)
	if err != nil {
		return "", fail(canonical, err)
	}
	return inserted, nil
}

// include liest ein Include-Ziel und setzt dessen eigene Includes ein. chain
// endet mit dem Ziel, depth ist seine Tiefe. Zurück kommt der Rumpf ohne
// Frontmatter; leer bei einem abgeschalteten Ziel.
func (x *docIncluder) include(target DocRef, chain []string, depth int) (string, error) {
	var result DocResult
	var err error
	switch target.Scheme {
	case DocSchemeProject:
		result, err = resolveProjectDoc(x.layers, target)
	case DocSchemeKeph:
		result, err = resolveKephDoc(x.ctx, x.layers, target)
	default:
		if !isDocRefKind(target.Path) {
			return "", docRefErrorf(DocCodeUnsupported,
				"%s: k-playbook löst nur Verweise unter %s auf", target.String(), docRefKindsText())
		}
		if target.Dir {
			if !isAssembledDocDir(target.Path) {
				return "", missingDocFileName(target)
			}
			x.includes = append(x.includes, DocInclude{Ref: target.String(), Depth: depth})
			parts, found, err := x.assembleDir(target.Path, chain[:len(chain)-1], depth)
			if err != nil {
				return "", err
			}
			if !found {
				return "", docRefErrorf(DocCodeNotFound,
					"%s ist unbekannt: das Verzeichnis gibt es weder lokal noch mitgeliefert", target.String())
			}
			return strings.Trim(joinDocDirParts(parts), "\n"), nil
		}
		result, err = resolveDocFile(x.layers, target)
	}
	if err != nil {
		return "", err
	}
	if result.Ref != target.String() {
		// keph: mit ~ — der echte Name kann schon offen sein.
		if slices.Contains(chain, result.Ref) {
			return "", docRefErrorf(DocCodeIncludeCycle, "%s ist schon offen — ein Include-Zyklus", result.Ref)
		}
		chain = append(append([]string{}, chain...), result.Ref)
	}
	x.includes = append(x.includes, DocInclude{Ref: result.Ref, Origin: result.Parts[0].Origin, Depth: depth, Disabled: result.Disabled})
	if result.Disabled {
		return "", nil
	}
	body := docBody([]byte(result.Content))
	if body == "" {
		return "", nil
	}
	return x.expand(body, target, chain, depth, false)
}

// includeTarget macht aus dem Argument einer Include-Zeile einen Verweis:
// mit Präfix bleibt es, wie es ist; ohne zählt es ab dem Verzeichnis des
// einbindenden Dokuments im selben Präfix. Nicht bereinigt — „..“ weist
// ParseDocRef ab, so führt kein relativer Pfad aus der Sorte heraus.
func includeTarget(arg string, from DocRef) string {
	if i := strings.IndexAny(arg, ":/"); i >= 0 && arg[i] == ':' {
		return arg
	}
	dir := path.Dir(from.Path)
	if dir == "." {
		return from.Scheme + ":/" + arg
	}
	return from.Scheme + ":/" + dir + "/" + arg
}

// includeChainError ist ein Fehler aus einem Include mit der Kette der
// Dokumente, über die es erreicht wurde. Der Code bleibt der des Fehlers
// (errors.As findet ihn durch Unwrap); die Kette wird nur einmal angehängt,
// am Ort des Scheiterns.
type includeChainError struct {
	err   error
	chain []string
}

func (e *includeChainError) Error() string {
	return e.err.Error() + " — Include-Kette: " + strings.Join(e.chain, " → ")
}

// Unwrap gibt den Fehler ohne Kette frei.
func (e *includeChainError) Unwrap() error { return e.err }

func chainIncludeError(err error, chain []string) error {
	var chained *includeChainError
	if errors.As(err, &chained) {
		return err
	}
	return &includeChainError{err: err, chain: chain}
}

// frontmatterLines zählt die Zeilen eines Frontmatter-Blocks am Anfang,
// einschließlich der schließenden Trennzeile; 0 ohne Block. Derselbe Schnitt
// wie inventory.FrontmatterBlock.
func frontmatterLines(lines []string) int {
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return 0
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return i + 1
		}
	}
	return 0
}

// fenceMarker erkennt den Zaun eines Code-Blocks: nach höchstens drei
// Leerzeichen mindestens drei „`“ oder „~“. Zurück kommen das Zeichen, seine
// Anzahl (0 ohne Zaun) und der Rest der Zeile.
func fenceMarker(line string) (byte, int, string) {
	trimmed := strings.TrimLeft(line, " ")
	if len(line)-len(trimmed) > 3 || trimmed == "" || (trimmed[0] != '`' && trimmed[0] != '~') {
		return 0, 0, ""
	}
	char := trimmed[0]
	n := 0
	for n < len(trimmed) && trimmed[n] == char {
		n++
	}
	if n < 3 {
		return 0, 0, ""
	}
	return char, n, trimmed[n:]
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
