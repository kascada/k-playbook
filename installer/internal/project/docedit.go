package project

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
)

// Ändern eines Dokuments der Collection, ohne dass sein Inhalt durch den
// Kontext des Aufrufers läuft (Task 104): anhängen (Append), einen Text
// ersetzen (Replace) und ein Frontmatter-Feld setzen oder entfernen
// (SetField). Am Hub gibt es nur write mit dem ganzen Inhalt; jede Operation
// liest deshalb hier, ändert und schreibt mit base_revision zurück — alle drei
// über die eine Funktion edit.
//
// Geltungsbereich: Dokumente der Sorten in DocKinds ohne eigene Werkzeuge,
// offen oder unter done/, bei Sorten mit Userbereich nur die eigenen — und
// für Append und Replace die Bereiche in DocEditAreas, heute die Handdoku
// unter docs/manual/ (Entscheidung 2026-11-08).
//
// Ein Dokument hat zu einer Zeit einen Schreiber. since_revision ist die
// Revision, die der Aufrufer zuletzt gesehen hat — aus seinem letzten
// Schreiben oder Lesen. Weicht das Dokument davon ab, hat ein anderer
// geschrieben: Die Operation verweigert (foreign_change), statt still
// nachzuziehen, und der Aufrufer fragt den Nutzer, wer weitermacht.

// Die Fehlercodes der Änderungsoperationen über die der doc-Werkzeuge hinaus.
const (
	// DocCodeForeignChange: Das Dokument hat nicht die Revision
	// since_revision — ein anderer hat geschrieben. Nichts wurde geschrieben;
	// der Fehler trägt DocForeignChange.
	DocCodeForeignChange = "foreign_change"
	// DocCodeTextNotFound: Der zu ersetzende Text kommt nicht vor.
	DocCodeTextNotFound = "text_not_found"
	// DocCodeTextNotUnique: Der zu ersetzende Text kommt mehr als einmal vor;
	// die Meldung nennt die Anzahl.
	DocCodeTextNotUnique = "text_not_unique"
)

// docEditOp ist eine der Änderungsoperationen; der Geltungsbereich fragt
// danach, denn nicht jeder Bereich lässt jede zu.
type docEditOp string

const (
	docEditAppend   docEditOp = "append"
	docEditReplace  docEditOp = "replace"
	docEditSetField docEditOp = "set"
)

// DocEditArea ist ein Bereich der Collection, dessen Dokumente die
// Änderungsoperationen erreichen, ohne dass er eine Sorte ist: ohne Nummern,
// ohne create, list, done und reopen, ohne Userbereich. Was darin liegt,
// gehört allen; since_revision fängt zwei Schreiber ab.
type DocEditArea struct {
	// Dir ist das Verzeichnis in der Collection, ohne Schrägstrich am Ende.
	// Alles darunter zählt, auch in Unterverzeichnissen.
	Dir string
	// Label nennt den Bereich in Meldungen.
	Label string
	// Append, Replace und SetField sagen, welche Operationen den Bereich
	// erreichen.
	Append, Replace, SetField bool
}

// DocEditAreas sind die Bereiche (Entscheidung 2026-11-08, Anlass war eine
// Handbuchseite in einem Zielprojekt): die Handdoku unter docs/manual/, für
// append und replace. Bisher blieb für eine Handbuchseite nur Kephalaion write mit dem
// ganzen Inhalt, und bei 20 bis 60 KB läuft so die ganze Seite durch den
// Kontext des Aufrufers, um einen Absatz zu tauschen. set bleibt außen vor:
// Handdoku hat kein Pflicht-Frontmatter, und set legte eines an, wo keines
// hingehört. docs/generated/ gehört den Erzeugern über docs checkout/publish
// und ist kein Bereich.
var DocEditAreas = []DocEditArea{
	{Dir: "docs/manual", Label: "Handdoku", Append: true, Replace: true},
}

// DocEditAreaDirs sind die Verzeichnisse der Bereiche mit Schrägstrich am
// Ende, für Meldungen und Hilfetexte.
func DocEditAreaDirs(areas []DocEditArea) []string {
	dirs := make([]string, 0, len(areas))
	for _, area := range areas {
		dirs = append(dirs, area.Dir+"/")
	}
	return dirs
}

// allows sagt, ob die Operation den Bereich erreicht.
func (a DocEditArea) allows(op docEditOp) bool {
	switch op {
	case docEditAppend:
		return a.Append
	case docEditReplace:
		return a.Replace
	case docEditSetField:
		return a.SetField
	}
	return false
}

// allowed nennt die Operationen, die den Bereich erreichen, für Meldungen.
func (a DocEditArea) allowed() string {
	var ops []string
	for _, op := range []docEditOp{docEditAppend, docEditReplace, docEditSetField} {
		if a.allows(op) {
			ops = append(ops, string(op))
		}
	}
	return strings.Join(ops, " und ")
}

// contains sagt, ob name unter dem Bereich liegt, beliebig tief.
func (a DocEditArea) contains(name string) bool {
	return strings.HasPrefix(name, a.Dir+"/") && len(name) > len(a.Dir)+1
}

// validateDocEditAreas prüft die Bereiche: gültige Pfade, mindestens eine
// Operation, und keiner überschneidet sich mit einem anderen oder mit der
// Ablage einer Sorte — sonst entschiede die Reihenfolge der Prüfung, welche
// Regeln gelten.
func validateDocEditAreas(areas []DocEditArea, kinds []DocKind) error {
	for i, area := range areas {
		label := fmt.Sprintf("Bereich %d (%q)", i+1, area.Dir)
		for _, segment := range strings.Split(area.Dir, "/") {
			if segment == "" || segment == "." || segment == ".." || strings.HasPrefix(segment, ".") || segment == docUserSegment {
				return fmt.Errorf("%s: kein Pfad aus gültigen Segmenten", label)
			}
		}
		if !area.Append && !area.Replace && !area.SetField {
			return fmt.Errorf("%s: keine Operation erlaubt", label)
		}
		for j, other := range areas {
			if i != j && (area.Dir == other.Dir || strings.HasPrefix(other.Dir, area.Dir+"/")) {
				return fmt.Errorf("Bereiche %q und %q überschneiden sich", area.Dir, other.Dir)
			}
		}
		for _, kind := range kinds {
			if area.Dir == kind.Dir || strings.HasPrefix(kind.Dir, area.Dir+"/") || strings.HasPrefix(area.Dir, kind.Dir+"/") {
				return fmt.Errorf("%s überschneidet sich mit der Ablage %q der Sorte %s", label, kind.Dir, kind.Kind)
			}
		}
	}
	return nil
}

// docEditRetries ist die Zahl der Wiederholungen: nach stale_revision ohne
// since_revision neu lesen und noch einmal schreiben, und mit since_revision
// neu lesen, solange die Replica noch unter der erwarteten Revision steht.
const docEditRetries = 3

// docEditTailLines ist die Zahl der Zeilen am Ende des Dokuments, die ein
// foreign_change mitbringt — das, was der Nutzer für die Rückfrage braucht.
const docEditTailLines = 20

// docEditWait ist die Wartezeit vor dem n-ten neuen Lesen (n ab 1): Die
// Replica des Nodes läuft dem Hub nach. Tests setzen sie auf null.
var docEditWait = func(n int) time.Duration { return time.Duration(n) * 300 * time.Millisecond }

// docFieldKeyPattern ist die Form eines Frontmatter-Felds, das SetField
// setzt.
var docFieldKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

// DocEditOptions sind die gemeinsamen Eingaben der Änderungsoperationen.
type DocEditOptions struct {
	// SinceRevision ist die Revision, die der Aufrufer zuletzt gesehen hat;
	// 0 heißt ohne Vorbedingung.
	SinceRevision int64
}

// DocEdited ist die Antwort einer Änderungsoperation.
type DocEdited struct {
	// Ref ist der keph:-Verweis mit dem echten Usernamen.
	Ref string `json:"ref"`
	// Revision ist die Revision nach dem Schreiben; ohne Änderung die
	// gelesene.
	Revision int64 `json:"revision"`
	// Size ist die Größe des Inhalts in Bytes.
	Size int64 `json:"size"`
	// Changed sagt, ob geschrieben wurde. Ein unveränderter Inhalt schreibt
	// nichts und erzeugt keine Revision.
	Changed bool `json:"changed"`
}

// DocForeignChange beschreibt ein foreign_change: wie das Dokument jetzt
// steht. Es steckt im Fehler mit DocCodeForeignChange; DocForeignChangeOf
// holt es heraus.
type DocForeignChange struct {
	Ref string `json:"ref"`
	// SinceRevision ist die erwartete Revision.
	SinceRevision int64 `json:"sinceRevision"`
	// Revision ist die Revision, die das Dokument jetzt hat (aus der
	// Replica).
	Revision int64 `json:"revision"`
	// UpdatedBy und UpdatedAt sagen, wer zuletzt geschrieben hat.
	UpdatedBy string `json:"updatedBy,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
	// Tail sind die letzten docEditTailLines Zeilen des Dokuments.
	Tail string `json:"tail"`
}

func (e *DocForeignChange) Error() string {
	who := ""
	if e.UpdatedBy != "" {
		who = ", zuletzt geschrieben von " + e.UpdatedBy
		if e.UpdatedAt != "" {
			who += " am " + e.UpdatedAt
		}
	}
	return fmt.Sprintf("%s hat seit Revision %d jemand anderes geändert (jetzt Revision %d%s); nichts geschrieben — "+
		"nicht still nachziehen, sondern den Nutzer fragen, wer weitermacht", e.Ref, e.SinceRevision, e.Revision, who)
}

// DocForeignChangeOf holt die Angaben eines foreign_change aus einem Fehler.
func DocForeignChangeOf(err error) (*DocForeignChange, bool) {
	var change *DocForeignChange
	if errors.As(err, &change) {
		return change, true
	}
	return nil, false
}

// Append hängt text ans Ende des Dokuments: Endet der Inhalt nicht mit einem
// Zeilenumbruch, kommt einer dazu, dann eine Leerzeile, dann der Text, der
// mit genau einem Zeilenumbruch endet. Ein leeres Dokument bekommt nur den
// Text. So beginnt jeder Eintrag am Zeilenanfang. Leerer Text ist
// invalid_input.
func (u *UserDocs) Append(ctx context.Context, ref, text string, opts DocEditOptions) (DocEdited, error) {
	if strings.TrimSpace(text) == "" {
		return DocEdited{}, docRefErrorf(DocCodeInvalidInput, "kein Text zum Anhängen angegeben")
	}
	return u.edit(ctx, ref, docEditAppend, opts, func(content string) (string, error) {
		return appendDocText(content, text), nil
	})
}

// appendDocText ist die Regel von Append.
func appendDocText(content, text string) string {
	entry := strings.TrimRight(text, "\r\n") + "\n"
	if content == "" {
		return entry
	}
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return content + "\n" + entry
}

// Replace ersetzt old, das genau einmal vorkommen muss, durch new. Kein
// Treffer ist text_not_found, mehrere sind text_not_unique. Leeres old ist
// invalid_input; leeres new löscht.
func (u *UserDocs) Replace(ctx context.Context, ref, old, new string, opts DocEditOptions) (DocEdited, error) {
	if old == "" {
		return DocEdited{}, docRefErrorf(DocCodeInvalidInput, "kein Text zum Ersetzen angegeben (old ist leer)")
	}
	return u.edit(ctx, ref, docEditReplace, opts, func(content string) (string, error) {
		switch n := countOverlapping(content, old); {
		case n == 0:
			return "", docRefErrorf(DocCodeTextNotFound, "der zu ersetzende Text kommt im Dokument nicht vor")
		case n > 1:
			return "", docRefErrorf(DocCodeTextNotUnique,
				"der zu ersetzende Text kommt %d-mal vor, erwartet genau einmal — mehr Umgebung mitgeben", n)
		}
		return strings.Replace(content, old, new, 1), nil
	})
}

// countOverlapping zählt die Vorkommen von sub in s, auch überlappende:
// Nach einem Treffer geht die Suche eine Stelle weiter, nicht um len(sub).
// strings.Count würde "x\nx\n" in "x\nx\nx\n" nur einmal zählen, obwohl der
// Text an zwei Stellen steht und ein Ersetzen damit nicht eindeutig ist.
// sub darf nicht leer sein.
func countOverlapping(s, sub string) int {
	n := 0
	for {
		i := strings.Index(s, sub)
		if i < 0 {
			return n
		}
		n++
		s = s[i+1:]
	}
}

// SetField setzt das Frontmatter-Feld key auf value oder entfernt es, wenn
// value nil ist. Fehlt das Frontmatter, wird beim Setzen eines angelegt.
// Andere Felder, ihre Reihenfolge und der Rumpf bleiben Byte für Byte; der
// Wert ist ein YAML-Skalar und wird quotiert, wenn nötig. key folgt
// [a-z][a-z0-9_-]*.
func (u *UserDocs) SetField(ctx context.Context, ref, key string, value *string, opts DocEditOptions) (DocEdited, error) {
	if !docFieldKeyPattern.MatchString(key) {
		return DocEdited{}, docRefErrorf(DocCodeInvalidInput, "Feld %q passt nicht zur Form [a-z][a-z0-9_-]*", key)
	}
	return u.edit(ctx, ref, docEditSetField, opts, func(content string) (string, error) {
		return setDocField(content, key, value), nil
	})
}

// setDocField ist die Regel von SetField. Ein Feld reicht von seiner Zeile
// bis vor die nächste Zeile, die weder eingerückt noch ein Listeneintrag ist;
// Leerzeilen gehören nur dazu, wenn danach noch eine Folgezeile kommt.
func setDocField(content, key string, value *string) string {
	lines := strings.SplitAfter(content, "\n")
	end := frontmatterLines(lines)
	if end == 0 {
		if value == nil {
			return content
		}
		head := "---\n" + key + ": " + yamlScalar(*value) + "\n---\n"
		if content == "" {
			return head
		}
		return head + "\n" + content
	}
	keyLine := regexp.MustCompile(`^` + regexp.QuoteMeta(key) + `[ \t]*:`)
	closing := end - 1
	start, stop := -1, -1
	for i := 1; i < closing; i++ {
		if !keyLine.MatchString(lines[i]) {
			continue
		}
		start, stop = i, i+1
		for j := i + 1; j < closing; j++ {
			line := strings.TrimRight(lines[j], "\r\n")
			if strings.TrimSpace(line) == "" {
				continue
			}
			if !isDocFieldContinuation(line) {
				break
			}
			stop = j + 1
		}
		break
	}
	var replacement []string
	if value != nil {
		replacement = []string{key + ": " + yamlScalar(*value) + "\n"}
	}
	var out []string
	switch {
	case start >= 0:
		out = append(append(append(out, lines[:start]...), replacement...), lines[stop:]...)
	case value == nil:
		return content
	default:
		out = append(append(append(out, lines[:closing]...), replacement...), lines[closing:]...)
	}
	return strings.Join(out, "")
}

// isDocFieldContinuation sagt, ob eine Zeile des Frontmatters zum Feld davor
// gehört: eingerückt oder ein Listeneintrag ohne Einrückung.
func isDocFieldContinuation(line string) bool {
	return strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") || line == "-" || strings.HasPrefix(line, "- ")
}

// editTarget löst den Verweis einer Änderungsoperation auf: ein keph:-Verweis
// auf ein Dokument einer Sorte ohne eigene Werkzeuge, offen oder unter done/,
// bei Sorten mit Userbereich ein eigenes — oder auf ein Dokument eines
// Bereichs (DocEditAreas), der die Operation op zulässt. Zurück kommt der
// volle Name in der Collection. Entschieden wird ohne zu lesen.
func (u *UserDocs) editTarget(raw string, op docEditOp) (string, error) {
	ref, err := ParseDocRef(raw)
	if err != nil {
		return "", err
	}
	kinds := strings.Join(DocKindNames(u.kinds), ", ")
	areas := strings.Join(DocEditAreaDirs(u.areas), ", ")
	if ref.Scheme != DocSchemeKeph {
		return "", docRefErrorf(DocCodeInvalidInput,
			"%s: geändert werden nur keph:-Verweise auf Dokumente der Sorten %s oder unter %s", ref.Raw, kinds, areas)
	}
	if ref.Dir || ref.Path == "" {
		return "", docRefErrorf(DocCodeInvalidInput, "%s: ein Verzeichnis lässt sich nicht ändern, erwartet ein Dokument", ref.Raw)
	}
	name := u.expandUser(ref.Path)
	if area, ok := u.matchArea(name); ok {
		if !area.allows(op) {
			return "", docRefErrorf(DocCodeInvalidInput,
				"%s liegt in der %s (%s/); dort gilt %s nicht, nur %s", ref.Raw, area.Label, area.Dir, op, area.allowed())
		}
		return name, nil
	}
	kind, m, found := u.matchKind(name)
	if !found || kind.OwnTools {
		return "", docRefErrorf(DocCodeInvalidInput,
			"%s liegt in keiner Ablage der Sorten %s, nicht in deren done/ und nicht unter %s — andere Dokumente ändern diese Werkzeuge nicht",
			ref.Raw, kinds, areas)
	}
	if kind.UserArea && m.user != u.user {
		return "", docRefErrorf(DocCodeForeignUser, "%s gehört %s, nicht %s; geändert werden nur eigene Dokumente",
			ref.Raw, m.user, u.user)
	}
	return name, nil
}

// expandUser ersetzt jeden ganzen Pfadteil ~ durch den eigenen User.
func (u *UserDocs) expandUser(refPath string) string {
	segments := strings.Split(refPath, "/")
	for i, segment := range segments {
		if segment == docUserSegment {
			segments[i] = u.user
		}
	}
	return strings.Join(segments, "/")
}

// matchArea sucht den Bereich, unter dem name liegt.
func (u *UserDocs) matchArea(name string) (DocEditArea, bool) {
	for _, area := range u.areas {
		if area.contains(name) {
			return area, true
		}
	}
	return DocEditArea{}, false
}

// matchKind sucht die Sorte, in deren Ablage oder done/ name liegt.
func (u *UserDocs) matchKind(name string) (DocKind, docMatch, bool) {
	for _, candidate := range u.kinds {
		if m, ok := candidate.match(name); ok {
			return candidate, m, true
		}
	}
	return DocKind{}, docMatch{}, false
}

// edit ist der Kern der Änderungsoperationen: Verweis auflösen, lesen,
// since_revision prüfen, change anwenden und mit base_revision schreiben.
// change bekommt den gelesenen Inhalt und liefert den neuen; ein Fehler von
// change beendet die Operation ohne Schreiben.
//
// Ohne since_revision wird nach stale_revision neu gelesen, change neu
// angewandt und bis zu docEditRetries Mal wiederholt. Mit since_revision ist
// stale_revision ein foreign_change. request_id und Wiederholung nach offenem
// Ausgang macht der Client (kephalaion.Client.Write); was dann offen bleibt,
// ist outcome_unknown.
func (u *UserDocs) edit(ctx context.Context, raw string, op docEditOp, opts DocEditOptions, change func(string) (string, error)) (DocEdited, error) {
	if opts.SinceRevision < 0 {
		return DocEdited{}, docRefErrorf(DocCodeInvalidInput, "since_revision %d ist keine Revision", opts.SinceRevision)
	}
	name, err := u.editTarget(raw, op)
	if err != nil {
		return DocEdited{}, err
	}
	ref := DocSchemeKeph + ":/" + name
	read, err := u.readForEdit(ctx, name, ref, opts.SinceRevision)
	if err != nil {
		return DocEdited{}, err
	}
	for attempt := 0; ; attempt++ {
		content := *read.Content
		updated, err := change(content)
		if err != nil {
			return DocEdited{}, err
		}
		if updated == content {
			return DocEdited{Ref: ref, Revision: read.Revision, Size: int64(len(content)), Changed: false}, nil
		}
		doc, err := u.store.Write(ctx, name, updated, read.Revision)
		if err == nil {
			size := int64(len(updated))
			if doc.Size != nil {
				size = *doc.Size
			}
			return DocEdited{Ref: ref, Revision: doc.Revision, Size: size, Changed: true}, nil
		}
		switch kephalaion.ErrorCode(err) {
		case kephalaion.CodeStaleRevision:
			if opts.SinceRevision > 0 {
				return DocEdited{}, u.foreignChangeAfterStale(ctx, name, ref, opts.SinceRevision, read)
			}
			if attempt >= docEditRetries {
				return DocEdited{}, &DocRefError{Code: DocCodeStaleRevision, err: fmt.Errorf(
					"%s: nach %d Wiederholungen weiter stale_revision — das Dokument ändert sich laufend: %w", ref, docEditRetries, err)}
			}
			sleepCtx(ctx, docEditWait(attempt+1))
			if read, err = u.readEditDoc(ctx, name, ref); err != nil {
				return DocEdited{}, err
			}
		case kephalaion.CodeOutcomeUnknown:
			return DocEdited{}, &DocRefError{Code: DocCodeOutcomeUnknown, err: fmt.Errorf(
				"%w Nicht noch einmal ändern oder anhängen, sondern nachlesen (k_playbook_doc_read bzw. k-playbook read, "+
					"oder k_playbook_doc_grep mit count_only für die Revision), ob die Änderung angekommen ist", err)}
		default:
			return DocEdited{}, kephError(err)
		}
	}
}

// readForEdit liest das Dokument und prüft since_revision. Liegt die
// gelesene Revision darunter, läuft die Replica nach dem eigenen Schreiben
// noch nach: warten und neu lesen, bis zu docEditRetries Mal. Eine höhere
// Revision, oder eine nach den Versuchen weiter abweichende, ist
// foreign_change.
func (u *UserDocs) readForEdit(ctx context.Context, name, ref string, since int64) (kephalaion.ReadResult, error) {
	read, err := u.readEditDoc(ctx, name, ref)
	if err != nil || since == 0 {
		return read, err
	}
	for n := 1; read.Revision < since && n <= docEditRetries; n++ {
		sleepCtx(ctx, docEditWait(n))
		if read, err = u.readEditDoc(ctx, name, ref); err != nil {
			return read, err
		}
	}
	if read.Revision != since {
		return kephalaion.ReadResult{}, foreignChange(ref, since, read)
	}
	return read, nil
}

// foreignChangeAfterStale ist das foreign_change nach stale_revision mit
// since_revision: Der Hub kennt eine neuere Revision als die gelesene. Neu
// gelesen wird nur für die Angaben; nachgezogen wird nichts.
func (u *UserDocs) foreignChangeAfterStale(ctx context.Context, name, ref string, since int64, last kephalaion.ReadResult) error {
	sleepCtx(ctx, docEditWait(1))
	if read, err := u.readEditDoc(ctx, name, ref); err == nil {
		last = read
	}
	return foreignChange(ref, since, last)
}

// foreignChange baut den Fehler aus dem gelesenen Stand.
func foreignChange(ref string, since int64, read kephalaion.ReadResult) error {
	change := &DocForeignChange{Ref: ref, SinceRevision: since, Revision: read.Revision}
	if read.Updated != nil {
		change.UpdatedBy, change.UpdatedAt = read.Updated.By, read.Updated.At
	}
	if read.Content != nil {
		change.Tail = docTail(*read.Content, docEditTailLines)
	}
	return &DocRefError{Code: DocCodeForeignChange, err: change}
}

// docTail sind die letzten n Zeilen eines Inhalts.
func docTail(content string, n int) string {
	lines := strings.SplitAfter(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "")
}

// readEditDoc liest ein Dokument zum Ändern. Gibt es es nicht, ist das
// not_found; ein Verzeichnis ist invalid_input; ohne Revision ist es ein
// Fehler, denn ohne sie gäbe es keine Prüfung am Hub.
func (u *UserDocs) readEditDoc(ctx context.Context, name, ref string) (kephalaion.ReadResult, error) {
	read, err := u.store.Read(ctx, name)
	if err != nil {
		return kephalaion.ReadResult{}, kephError(err)
	}
	switch {
	case read.Kind == kephalaion.ReadKindNone:
		return kephalaion.ReadResult{}, docRefErrorf(DocCodeNotFound, "%s ist unbekannt: in der Collection gibt es das Dokument nicht", ref)
	case read.Kind == kephalaion.ReadKindDirectory:
		return kephalaion.ReadResult{}, docRefErrorf(DocCodeInvalidInput, "%s ist ein Verzeichnis, erwartet ein Dokument", ref)
	case read.Content == nil:
		return kephalaion.ReadResult{}, docRefErrorf(DocCodeKephalaionError, "%s: read liefert keinen Inhalt", ref)
	case read.Revision <= 0:
		return kephalaion.ReadResult{}, docRefErrorf(DocCodeKephalaionError, "%s: read nennt keine Revision", ref)
	}
	return read, nil
}

// sleepCtx wartet d, höchstens bis ctx abläuft.
func sleepCtx(ctx context.Context, d time.Duration) {
	if d <= 0 {
		return
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}
