package project

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/inventory"
	"github.com/kascada/k-playbook/installer/internal/kephalaion"
)

// Nummerierte Dokumente in der Collection des Projekts: anlegen, auflisten,
// erledigen und wiederöffnen (docs/mcp.md, „doc-Werkzeuge“).
//
// Was eine Sorte ist — wo ihre Dokumente liegen, ob sie einen Userbereich hat,
// woraus die nächste Nummer gezählt wird, ob es done/ gibt und mit welcher
// Nummer sie beginnt —, steht allein in der Tabelle DocKinds. Der Code fragt
// nur die Spalten ab; eine neue Sorte ist ein neuer Eintrag.
//
// Nummer und Liste lesen die Replica des Nodes, Anlegen und Verschieben gehen
// über den Hub. Den Inhalt liest und schreibt die Sitzung direkt über
// Kephalaion; dafür gibt es hier nichts.

// DocCounting ist der Zählbereich einer Sorte: aus welchen Verzeichnissen die
// höchste Nummer stammt. done/ zählt immer mit.
type DocCounting string

const (
	// DocCountAllUsers: die Ablage aller User samt done/, projektweit. Nur bei
	// Sorten mit Userbereich.
	DocCountAllUsers DocCounting = "all-users"
	// DocCountDir: die Ablage selbst samt done/ — bei Sorten mit Userbereich
	// die des eigenen Users.
	DocCountDir DocCounting = "dir"
)

// docUserSegment steht in DocKind.Dir für den User, wie ~ im Verweisschema.
const docUserSegment = "~"

// docUserRoot ist das Verzeichnis der Userbereiche in der Collection.
const docUserRoot = "user"

// docQueueRoot ist das Verzeichnis der Queue. Alles darunter hat keinen
// Userbereich (Entscheidung 2026-10-14).
const docQueueRoot = "queue"

// docDoneDir ist das Unterverzeichnis der erledigten Dokumente.
const docDoneDir = "done"

// DocKind ist ein Eintrag der Sorten-Tabelle.
type DocKind struct {
	// Kind ist der Name der Sorte, der Wert von kind in Werkzeug und CLI.
	Kind string `json:"kind"`
	// Dir ist die Ablage in der Collection, ohne Schrägstrich am Ende. Bei
	// einer Sorte mit Userbereich beginnt sie mit user/~/; ~ steht für den
	// User.
	Dir string `json:"dir"`
	// UserArea sagt, ob die Sorte einen Userbereich hat.
	UserArea bool `json:"userArea"`
	// Counting ist der Zählbereich.
	Counting DocCounting `json:"counting"`
	// Done sagt, ob Dokumente nach done/ verschoben werden.
	Done bool `json:"done"`
	// Start ist die Startnummer: Ist der Zählbereich leer oder liegt sein
	// Höchstwert darunter, bekommt das Dokument sie.
	Start int `json:"start"`
	// OwnTools sagt, dass die Sorte eigene Werkzeuge hat und über die
	// doc-Werkzeuge (k_playbook_doc_*, k-playbook doc) nicht erreichbar ist:
	// Sie steht nicht in deren kind-Liste, und create, list, done und reopen
	// lehnen sie ab. Nummernvergabe und done/-Behandlung der Tabelle gelten
	// trotzdem — die eigenen Werkzeuge benutzen sie.
	OwnTools bool `json:"ownTools,omitempty"`
}

// DocKinds ist die Sorten-Tabelle (Task 088, Klärungen 2026-10-14). tasks
// beginnt bei 100, damit sich die Nummern mit den bisherigen Task-Dateien auf
// der Platte nicht überschneiden. todos (Task 097, Entscheidung 2026-10-19)
// ist projektweit, zählt über todos/ samt done/ und hat eigene Werkzeuge
// (todos.go): doc_create legte ein leeres Dokument an, doc_done verschöbe
// ohne das Feld done:.
var DocKinds = []DocKind{
	{Kind: "tasks", Dir: "user/~/tasks", UserArea: true, Counting: DocCountAllUsers, Done: true, Start: 100},
	{Kind: "workbench", Dir: "user/~/workbench", UserArea: true, Counting: DocCountAllUsers, Done: true, Start: 1},
	{Kind: "findings", Dir: "queue/findings", UserArea: false, Counting: DocCountDir, Done: false, Start: 1},
	{Kind: TodoKindName, Dir: "todos", UserArea: false, Counting: DocCountDir, Done: true, Start: 1, OwnTools: true},
}

// DocKindNames sind die Namen der Sorten in der Reihenfolge der Tabelle, die
// die doc-Werkzeuge erreichen — die erlaubten Werte von kind. Sorten mit
// eigenen Werkzeugen (OwnTools) fehlen.
func DocKindNames(kinds []DocKind) []string {
	names := make([]string, 0, len(kinds))
	for _, kind := range kinds {
		if kind.OwnTools {
			continue
		}
		names = append(names, kind.Kind)
	}
	return names
}

// ownToolsError ist die Ablehnung einer Sorte mit eigenen Werkzeugen durch
// die doc-Werkzeuge.
func ownToolsError(kind DocKind) error {
	if kind.Kind == TodoKindName {
		return docRefErrorf(DocCodeInvalidInput, "die Sorte %s hat eigene Werkzeuge und ist über die doc-Werkzeuge nicht erreichbar — "+
			"Todos gehen über k_playbook_todo_* bzw. k-playbook todo", kind.Kind)
	}
	return docRefErrorf(DocCodeInvalidInput, "die Sorte %s hat eigene Werkzeuge und ist über die doc-Werkzeuge nicht erreichbar", kind.Kind)
}

// Die Fehlercodes der doc-Werkzeuge über die der Verweise hinaus
// (DocCodeInvalidInput, DocCodeNotFound).
const (
	// DocCodeKephalaionUnavailable: Beim Hub ist nichts angekommen — die
	// Anbindung ist nicht ok, der Node nicht erreichbar oder zu alt
	// (kephalaion.MinWriteNodeVersion), oder der Node meldet unreachable,
	// unsupported oder internal. Es gibt keinen Rückfall auf die Platte.
	DocCodeKephalaionUnavailable = "kephalaion_unavailable"
	// DocCodeOutcomeUnknown: Anlegen oder Verschieben kann beim Hub
	// angekommen sein, auch die Wiederholung mit demselben request_id hat den
	// Ausgang nicht geklärt. Nicht wiederholen, sondern mit doc list
	// nachsehen.
	DocCodeOutcomeUnknown = "outcome_unknown"
	// DocCodeForeignUser: Der Verweis liegt im Userbereich eines anderen.
	DocCodeForeignUser = "foreign_user"
	// DocCodeDoneUnsupported: Die Sorte hat kein done/.
	DocCodeDoneUnsupported = "done_unsupported"
	// DocCodeWrongDir: Der Verweis liegt nicht dort, wo der Vorgang beginnt —
	// done/ bei done, außerhalb von done/ bei reopen, oder in keiner Ablage.
	DocCodeWrongDir = "wrong_dir"
	// DocCodeNameTaken: name_taken von Kephalaion — das Ziel ist belegt, oder
	// beim Anlegen waren alle Versuche belegt.
	DocCodeNameTaken = "name_taken"
	// DocCodeStaleRevision: stale_revision von Kephalaion — das Dokument hat
	// sich seit dem Lesen geändert, etwa von einem anderen Rechner; nichts
	// wurde geschrieben. Neu lesen, dann noch einmal.
	DocCodeStaleRevision = "stale_revision"
	// DocCodeKephalaionError: ein anderer Fehler von Kephalaion; die Meldung
	// nennt dessen Code.
	DocCodeKephalaionError = "kephalaion_error"
)

// docCreateAttempts ist die Zahl der Nummern, die Create nacheinander
// versucht, bevor es aufgibt.
const docCreateAttempts = 10

// docMaxShortName begrenzt den Kurznamen, damit <nummer>-<name>.md auch mit
// einer vierstelligen Nummer in die Namensregel von Kephalaion passt (63
// Zeichen).
const docMaxShortName = 50

// docNumberPattern liest die Nummer vorn an einem Dateinamen.
var docNumberPattern = regexp.MustCompile(`^([0-9]+)-.+\.md$`)

// docNamePattern ist die Namensregel von Kephalaion, die auch für den
// Dateinamen eines Dokuments gelten soll.
var docNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,62}$`)

// ValidateDocKinds prüft eine Sorten-Tabelle.
func ValidateDocKinds(kinds []DocKind) error {
	seen := map[string]bool{}
	for i, kind := range kinds {
		label := fmt.Sprintf("Sorte %d (%q)", i+1, kind.Kind)
		if !docNamePattern.MatchString(kind.Kind) {
			return fmt.Errorf("%s: ungültiger Name", label)
		}
		if seen[kind.Kind] {
			return fmt.Errorf("%s: doppelt", label)
		}
		seen[kind.Kind] = true
		segments := strings.Split(kind.Dir, "/")
		for _, segment := range segments {
			if segment == "" || segment == "." || segment == ".." || strings.HasPrefix(segment, ".") {
				return fmt.Errorf("%s: Ablage %q ist kein Pfad aus gültigen Segmenten", label, kind.Dir)
			}
		}
		if segments[0] == docQueueRoot && kind.UserArea {
			return fmt.Errorf("%s: Ablage %q liegt unter %s/ — dort gibt es keinen Userbereich", label, kind.Dir, docQueueRoot)
		}
		inUserArea := len(segments) > 2 && segments[0] == docUserRoot && segments[1] == docUserSegment
		if kind.UserArea != inUserArea {
			return fmt.Errorf("%s: Ablage %q passt nicht zum Userbereich %v — mit Userbereich beginnt sie mit %s/%s/, sonst nicht",
				label, kind.Dir, kind.UserArea, docUserRoot, docUserSegment)
		}
		if strings.Count(kind.Dir, docUserSegment) > 1 || (!kind.UserArea && strings.Contains(kind.Dir, docUserSegment)) {
			return fmt.Errorf("%s: %s darf in der Ablage nur als User-Segment stehen", label, docUserSegment)
		}
		if segments[len(segments)-1] == docDoneDir {
			return fmt.Errorf("%s: Ablage %q endet auf %s", label, kind.Dir, docDoneDir)
		}
		switch kind.Counting {
		case DocCountDir:
		case DocCountAllUsers:
			if !kind.UserArea {
				return fmt.Errorf("%s: Zählbereich %s nur mit Userbereich", label, kind.Counting)
			}
		default:
			return fmt.Errorf("%s: unbekannter Zählbereich %q", label, kind.Counting)
		}
		if kind.Start < 1 {
			return fmt.Errorf("%s: Startnummer %d, erwartet ab 1", label, kind.Start)
		}
	}
	for i, a := range kinds {
		for j, b := range kinds {
			if i != j && (a.Dir == b.Dir || strings.HasPrefix(b.Dir, a.Dir+"/")) {
				return fmt.Errorf("Sorten %q und %q: Ablage %q enthält %q", a.Kind, b.Kind, a.Dir, b.Dir)
			}
		}
	}
	return nil
}

// dirFor ist die Ablage für einen User.
func (k DocKind) dirFor(user string) string {
	return strings.Replace(k.Dir, docUserSegment, user, 1)
}

// listRoot ist das Verzeichnis, das ein Listing über alle User liest: der
// Teil der Ablage vor ~. Ohne Userbereich die Ablage selbst.
func (k DocKind) listRoot() string {
	before, _, found := strings.Cut(k.Dir, "/"+docUserSegment)
	if !found {
		return k.Dir
	}
	return before
}

// docMatch ist ein Dokument, zerlegt nach einer Sorte.
type docMatch struct {
	user string
	done bool
	file string
}

// match prüft, ob name direkt in der Ablage der Sorte oder in deren done/
// liegt, für irgendeinen User. Tiefer Liegendes gehört nicht dazu.
func (k DocKind) match(name string) (docMatch, bool) {
	pattern := strings.Split(k.Dir, "/")
	segments := strings.Split(name, "/")
	var m docMatch
	switch {
	case len(segments) == len(pattern)+1:
	case len(segments) == len(pattern)+2 && segments[len(pattern)] == docDoneDir:
		m.done = true
	default:
		return docMatch{}, false
	}
	for i, want := range pattern {
		if want == docUserSegment {
			if segments[i] == "" {
				return docMatch{}, false
			}
			m.user = segments[i]
			continue
		}
		if segments[i] != want {
			return docMatch{}, false
		}
	}
	m.file = segments[len(segments)-1]
	if m.file == "" {
		return docMatch{}, false
	}
	return m, true
}

// DocStore sind die Aufrufe an Kephalaion, die die doc-Werkzeuge brauchen.
// *kephalaion.Client erfüllt sie; Tests setzen eine Fälschung ein. Create
// und Rename tragen je Aufruf einen eigenen request_id und wiederholen nach
// offenem Ausgang selbst, genau einmal mit demselben Schlüssel; was dann noch
// offen ist, kommt als kephalaion.CodeOutcomeUnknown.
type DocStore interface {
	List(ctx context.Context, req kephalaion.ListRequest) ([]kephalaion.Entry, error)
	Create(ctx context.Context, name, content string) (kephalaion.Document, error)
	Rename(ctx context.Context, name, newName string) (kephalaion.Document, error)
}

// UserDocs ist der Zugang zu den nummerierten Dokumenten eines Users.
type UserDocs struct {
	kinds []DocKind
	store DocStore
	user  string
}

// NewUserDocs baut den Zugang. user ist kephalaion.user, nicht der Account.
func NewUserDocs(kinds []DocKind, store DocStore, user string) (*UserDocs, error) {
	if err := ValidateDocKinds(kinds); err != nil {
		return nil, fmt.Errorf("Sorten-Tabelle ungültig: %w", err)
	}
	if !docNamePattern.MatchString(user) {
		return nil, docRefErrorf(DocCodeKephalaionUnavailable, "Kephalaion nennt keinen gültigen User (%q)", user)
	}
	return &UserDocs{kinds: kinds, store: store, user: user}, nil
}

// User ist der eigene User.
func (u *UserDocs) User() string { return u.user }

// OpenUserDocs prüft die Anbindung des Projekts und baut den Zugang mit der
// Sorten-Tabelle DocKinds. Ist kephalaion.ok nicht wahr, ist das ein Fehler
// mit DocCodeKephalaionUnavailable.
func OpenUserDocs(projectDir string) (*UserDocs, error) {
	client, user, err := openKephClient(projectDir)
	if err != nil {
		return nil, err
	}
	return NewUserDocs(DocKinds, client, user)
}

// KephDocStore sind die Aufrufe an Kephalaion, die keph:-Verweise brauchen.
// *kephalaion.Client erfüllt sie; Tests setzen eine Fälschung ein.
type KephDocStore interface {
	Read(ctx context.Context, name string) (kephalaion.ReadResult, error)
}

// KephDocs ist der Zugang zur Collection des Projekts für keph:-Verweise:
// der Store und der eigene User, für den ~ im Pfad steht.
type KephDocs struct {
	Store KephDocStore
	User  string
}

// OpenKephDocs prüft die Anbindung des Projekts und baut den Zugang für
// keph:-Verweise — derselbe Weg wie OpenUserDocs, Fehler mit
// DocCodeKephalaionUnavailable.
func OpenKephDocs(projectDir string) (KephDocs, error) {
	client, user, err := openKephClient(projectDir)
	if err != nil {
		return KephDocs{}, err
	}
	return KephDocs{Store: client, User: user}, nil
}

// OpenInventoryStore öffnet den Zugang zur Collection des Projekts für das
// Versionsinventar (inventory.DocName) — derselbe Weg wie OpenTodoStore und
// OpenKephDocs, Fehler mit DocCodeKephalaionUnavailable. Austauschbar, damit
// Tests den Node nicht fragen: Subkommando und Oberfläche setzen dann
// kephtest.Store ein. Was der Store meldet, trägt die Codes der doc-Werkzeuge
// (kephError): kephalaion_unavailable, stale_revision, outcome_unknown.
// Ohne bereite Anbindung gibt es kein Inventar; einen Rückfall auf die
// Platte gibt es nicht.
var OpenInventoryStore = func(projectDir string) (inventory.Store, error) {
	client, _, err := openKephClient(projectDir)
	if err != nil {
		return nil, err
	}
	return inventoryStore{client: client}, nil
}

// inventoryStore ist der Kephalaion-Client hinter inventory.Store; er
// übersetzt jeden Fehler in einen Code der doc-Werkzeuge, damit das
// Inventar-Paket die Codes nicht kennen muss.
type inventoryStore struct {
	client *kephalaion.Client
}

func (s inventoryStore) Read(ctx context.Context, name string) (kephalaion.ReadResult, error) {
	read, err := s.client.Read(ctx, name)
	if err != nil {
		return kephalaion.ReadResult{}, kephError(err)
	}
	return read, nil
}

func (s inventoryStore) Create(ctx context.Context, name, content string) (kephalaion.Document, error) {
	doc, err := s.client.Create(ctx, name, content)
	if err != nil {
		return kephalaion.Document{}, kephError(err)
	}
	return doc, nil
}

func (s inventoryStore) Write(ctx context.Context, name, content string, baseRevision int64) (kephalaion.Document, error) {
	doc, err := s.client.Write(ctx, name, content, baseRevision)
	if err != nil {
		return kephalaion.Document{}, kephError(err)
	}
	return doc, nil
}

// openKephClient ist der Zugang je Projekt: Status prüfen (kephalaion.ok),
// Anbindung lesen, Umgebung laden, Client bauen. Zurück kommen der Client
// und der User hinter dem Account.
func openKephClient(projectDir string) (*kephalaion.Client, string, error) {
	status := KephalaionStatus(projectDir)
	if !status.OK {
		return nil, "", docRefErrorf(DocCodeKephalaionUnavailable,
			"Kephalaion ist für dieses Projekt nicht bereit (%s): %s", status.State, status.Message)
	}
	binding, _, err := kephalaion.ReadBinding(LocalDir(projectDir))
	if err != nil {
		return nil, "", docRefErrorf(DocCodeKephalaionUnavailable, "Anbindung nicht lesbar: %v", err)
	}
	client, err := kephalaion.NewClient(binding, kephalaion.DefaultEnv())
	if err != nil {
		return nil, "", docRefErrorf(DocCodeKephalaionUnavailable, "%v", err)
	}
	return client, status.User, nil
}

// kind sucht eine Sorte nach Namen, für die doc-Werkzeuge: Eine Sorte mit
// eigenen Werkzeugen ist abgelehnt.
func (u *UserDocs) kind(name string) (DocKind, error) {
	kind, ok := u.anyKind(name)
	if ok && kind.OwnTools {
		return DocKind{}, ownToolsError(kind)
	}
	if ok {
		return kind, nil
	}
	return DocKind{}, docRefErrorf(DocCodeInvalidInput, "unbekannte Sorte %q — erlaubt sind %s",
		name, strings.Join(DocKindNames(u.kinds), ", "))
}

// anyKind sucht eine Sorte nach Namen, auch eine mit eigenen Werkzeugen.
func (u *UserDocs) anyKind(name string) (DocKind, bool) {
	for _, kind := range u.kinds {
		if kind.Kind == name {
			return kind, true
		}
	}
	return DocKind{}, false
}

// kephError übersetzt einen Fehler von Kephalaion in einen Code der
// doc-Werkzeuge. kephalaion_unavailable heißt: Beim Hub ist nichts
// angekommen. Ein offener Ausgang wird outcome_unknown, nie
// kephalaion_unavailable.
func kephError(err error) error {
	code := kephalaion.ErrorCode(err)
	switch code {
	case kephalaion.CodeNodeUnreachable:
		return &DocRefError{Code: DocCodeKephalaionUnavailable, err: err}
	case kephalaion.CodeUnreachable, kephalaion.CodeUnsupported, kephalaion.CodeInternal:
		return &DocRefError{Code: DocCodeKephalaionUnavailable,
			err: fmt.Errorf("%w (Code von Kephalaion: %s; nichts gespeichert)", err, code)}
	case kephalaion.CodeOutcomeUnknown:
		return &DocRefError{Code: DocCodeOutcomeUnknown, err: fmt.Errorf(
			"%w Nachsehen mit k_playbook_doc_list bzw. k-playbook doc list, ob der Vorgang geschehen ist; nicht noch einmal anlegen oder verschieben", err)}
	case kephalaion.CodeNameTaken:
		return &DocRefError{Code: DocCodeNameTaken, err: err}
	case kephalaion.CodeNotFound:
		return &DocRefError{Code: DocCodeNotFound, err: err}
	case kephalaion.CodeStaleRevision:
		return &DocRefError{Code: DocCodeStaleRevision, err: err}
	case "":
		return &DocRefError{Code: DocCodeKephalaionError, err: err}
	}
	return &DocRefError{Code: DocCodeKephalaionError, err: fmt.Errorf("%w (Code von Kephalaion: %s)", err, code)}
}

// NormalizeDocName macht aus dem Kurznamen des Aufrufers den Namensteil
// eines Dokuments: Kleinbuchstaben, Umlaute und ß umschrieben, alles andere
// als a–z und 0–9 zu einem Bindestrich, keine Bindestriche am Rand, höchstens
// docMaxShortName Zeichen. Ein „.md“ am Ende fällt weg.
func NormalizeDocName(raw string) (string, error) {
	text := strings.TrimSpace(raw)
	if len(text) >= 3 && strings.EqualFold(text[len(text)-3:], ".md") {
		text = text[:len(text)-3]
	}
	text = strings.ToLower(text)
	replacer := strings.NewReplacer("ä", "ae", "ö", "oe", "ü", "ue", "ß", "ss")
	text = replacer.Replace(text)
	var b strings.Builder
	dash := false
	for _, r := range text {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			dash = false
			continue
		}
		if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	name := strings.TrimRight(b.String(), "-")
	if len(name) > docMaxShortName {
		name = strings.TrimRight(name[:docMaxShortName], "-")
	}
	if name == "" {
		return "", docRefErrorf(DocCodeInvalidInput,
			"Kurzname %q enthält keinen Buchstaben und keine Ziffer", raw)
	}
	return name, nil
}

// docFileName ist <nummer>-<name>.md, die Nummer mindestens dreistellig.
func docFileName(number int, name string) string {
	return fmt.Sprintf("%03d-%s.md", number, name)
}

// docNumber liest die Nummer vorn an einem Dateinamen.
func docNumber(file string) (int, bool) {
	m := docNumberPattern.FindStringSubmatch(file)
	if m == nil {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	return n, err == nil
}

// listKind liest die Dokumente einer Sorte aus der Replica: für user einen
// User, für "*" alle. Zurück kommen nur die, die direkt in der Ablage oder in
// done/ liegen.
func (u *UserDocs) listKind(ctx context.Context, kind DocKind, user string, frontmatter bool) ([]kephalaion.Entry, []docMatch, error) {
	path := kind.dirFor(user)
	if kind.UserArea && user == "*" {
		path = kind.listRoot()
	}
	entries, err := u.store.List(ctx, kephalaion.ListRequest{Path: path, Recursive: true, Frontmatter: frontmatter})
	if err != nil {
		return nil, nil, kephError(err)
	}
	var kept []kephalaion.Entry
	var matches []docMatch
	for _, entry := range entries {
		m, ok := kind.match(entry.Name)
		if !ok || (kind.UserArea && user != "*" && m.user != user) {
			continue
		}
		kept = append(kept, entry)
		matches = append(matches, m)
	}
	return kept, matches, nil
}

// nextNumber bestimmt die Nummer aus dem Zählbereich: Höchstwert plus eins,
// mindestens die Startnummer.
func (u *UserDocs) nextNumber(ctx context.Context, kind DocKind) (int, error) {
	user := u.user
	if kind.Counting == DocCountAllUsers {
		user = "*"
	}
	_, matches, err := u.listKind(ctx, kind, user, false)
	if err != nil {
		return 0, err
	}
	highest := 0
	for _, m := range matches {
		if n, ok := docNumber(m.file); ok && n > highest {
			highest = n
		}
	}
	if highest+1 < kind.Start {
		return kind.Start, nil
	}
	return highest + 1, nil
}

// DocCreated ist die Antwort von Create.
type DocCreated struct {
	// Ref ist der keph:-Verweis mit dem echten Usernamen.
	Ref    string `json:"ref"`
	Kind   string `json:"kind"`
	Number int    `json:"number"`
	// Name ist der Dateiname, <nummer>-<name>.md.
	Name string `json:"name"`
	// User ist der User des Userbereichs; leer bei Sorten ohne.
	User string `json:"user,omitempty"`
}

// Create legt ein leeres Dokument der Sorte mit der nächsten Nummer an. Ist
// der Name belegt (name_taken), versucht es die nächste Nummer, bis zu
// docCreateAttempts Mal; jede Nummer ist ein eigener Aufruf von
// DocStore.Create und bekommt damit einen eigenen request_id. Jeder andere
// Fehler beendet die Schleife — ein offener Ausgang (outcome_unknown) zieht
// nie die nächste Nummer, denn das erste Dokument kann schon angelegt sein.
func (u *UserDocs) Create(ctx context.Context, kindName, shortName string) (DocCreated, error) {
	kind, err := u.kind(kindName)
	if err != nil {
		return DocCreated{}, err
	}
	name, err := NormalizeDocName(shortName)
	if err != nil {
		return DocCreated{}, err
	}
	created, _, err := u.createNumbered(ctx, kind, kind.dirFor(u.user), name, "", kephError)
	return created, err
}

// createNumbered legt ein Dokument mit content unter dir an, mit der nächsten
// Nummer aus dem Zählbereich der Sorte — der Weg von Create, auch für Sorten
// mit eigenen Werkzeugen. dir ist die Ablage oder deren done/. Zurück kommen
// die Angaben wie bei Create und die Antwort des Hubs. mapErr übersetzt einen
// Fehler von create außer name_taken in den Code des Aufrufers.
func (u *UserDocs) createNumbered(ctx context.Context, kind DocKind, dir, name, content string,
	mapErr func(error) error) (DocCreated, kephalaion.Document, error) {
	number, err := u.nextNumber(ctx, kind)
	if err != nil {
		return DocCreated{}, kephalaion.Document{}, err
	}
	var lastErr error
	for attempt := 0; attempt < docCreateAttempts; attempt++ {
		file := docFileName(number, name)
		if !docNamePattern.MatchString(file) {
			return DocCreated{}, kephalaion.Document{}, docRefErrorf(DocCodeInvalidInput, "Dateiname %q passt nicht in die Namensregel von Kephalaion", file)
		}
		doc, err := u.store.Create(ctx, dir+"/"+file, content)
		if err == nil {
			created := DocCreated{Ref: DocSchemeKeph + ":/" + dir + "/" + file, Kind: kind.Kind, Number: number, Name: file}
			if kind.UserArea {
				created.User = u.user
			}
			return created, doc, nil
		}
		if kephalaion.ErrorCode(err) != kephalaion.CodeNameTaken {
			return DocCreated{}, kephalaion.Document{}, mapErr(err)
		}
		lastErr = err
		number++
	}
	return DocCreated{}, kephalaion.Document{}, &DocRefError{Code: DocCodeNameTaken, err: fmt.Errorf(
		"%d Nummern nacheinander belegt, zuletzt %s: %w", docCreateAttempts, docFileName(number-1, name), lastErr)}
}

// DocListOptions sind die Eingaben von List.
type DocListOptions struct {
	Kind string
	// User ist leer für den eigenen, "*" für alle oder ein Username. Greift
	// nur bei Sorten mit Userbereich.
	User string
	// Done nimmt die Erledigten mit auf.
	Done bool
}

// DocListItem ist ein Dokument in der Antwort von List.
type DocListItem struct {
	Ref   string `json:"ref"`
	Title string `json:"title"`
	// User ist bei Sorten mit Userbereich der aus dem Pfad, sonst der, der
	// das Dokument angelegt hat (Autor-Metadaten von Kephalaion), oder leer.
	User    string `json:"user"`
	Created string `json:"created"`
	Updated string `json:"updated"`
	Done    bool   `json:"done"`
}

// DocList ist die Antwort von List.
type DocList struct {
	Kind string `json:"kind"`
	// User ist der angewandte Filter: ein Username oder "*"; leer bei Sorten
	// ohne Userbereich.
	User  string        `json:"user"`
	Done  bool          `json:"done"`
	Items []DocListItem `json:"items"`
	// Hint erklärt, warum ein Filter nicht gegriffen hat.
	Hint string `json:"hint,omitempty"`
}

// List listet die Dokumente einer Sorte aus der Replica.
func (u *UserDocs) List(ctx context.Context, opts DocListOptions) (DocList, error) {
	kind, err := u.kind(opts.Kind)
	if err != nil {
		return DocList{}, err
	}
	result := DocList{Kind: kind.Kind, Done: opts.Done, Items: []DocListItem{}}
	user := strings.TrimSpace(opts.User)
	if kind.UserArea {
		switch {
		case user == "":
			user = u.user
		case user == docUserSegment:
			user = u.user
		case user == "*":
		case !docNamePattern.MatchString(user):
			return DocList{}, docRefErrorf(DocCodeInvalidInput, "user %q ist kein gültiger Username; erlaubt sind ein Name, * oder leer", user)
		}
		result.User = user
	} else {
		if user != "" {
			result.Hint = fmt.Sprintf("Die Sorte %s hat keinen Userbereich; user greift nicht, gelistet sind alle.", kind.Kind)
		}
		user = u.user
	}
	entries, matches, err := u.listKind(ctx, kind, user, true)
	if err != nil {
		return DocList{}, err
	}
	for i, entry := range entries {
		m := matches[i]
		if m.done && !opts.Done {
			continue
		}
		item := DocListItem{Ref: DocSchemeKeph + ":/" + entry.Name, Title: m.file, User: m.user, Done: m.done}
		if title, ok := entry.Frontmatter["title"].(string); ok && strings.TrimSpace(title) != "" {
			item.Title = strings.TrimSpace(title)
		}
		if entry.Created != nil {
			item.Created = entry.Created.At
			if !kind.UserArea {
				item.User = entry.Created.By
			}
		}
		if entry.Updated != nil {
			item.Updated = entry.Updated.At
		}
		result.Items = append(result.Items, item)
	}
	sort.SliceStable(result.Items, func(i, j int) bool { return result.Items[i].Ref < result.Items[j].Ref })
	return result, nil
}

// DocMoved ist die Antwort von Done und Reopen.
type DocMoved struct {
	// Ref ist der neue Verweis mit dem echten Usernamen.
	Ref string `json:"ref"`
	// From ist der alte Verweis, ~ schon ersetzt.
	From string `json:"from"`
	Kind string `json:"kind"`
}

// Done verschiebt ein eigenes Dokument aus der Ablage nach done/.
func (u *UserDocs) Done(ctx context.Context, ref string) (DocMoved, error) {
	return u.move(ctx, ref, true)
}

// Reopen holt ein eigenes Dokument aus done/ zurück in die Ablage.
func (u *UserDocs) Reopen(ctx context.Context, ref string) (DocMoved, error) {
	return u.move(ctx, ref, false)
}

func (u *UserDocs) move(ctx context.Context, raw string, toDone bool) (DocMoved, error) {
	ref, err := ParseDocRef(raw)
	if err != nil {
		return DocMoved{}, err
	}
	if ref.Scheme != DocSchemeKeph {
		return DocMoved{}, docRefErrorf(DocCodeInvalidInput, "%s: erwartet ein keph:-Verweis auf ein Dokument", ref.Raw)
	}
	if ref.Dir || ref.Path == "" {
		return DocMoved{}, docRefErrorf(DocCodeInvalidInput, "%s: ein Verzeichnis lässt sich nicht verschieben, erwartet ein Dokument", ref.Raw)
	}
	segments := strings.Split(ref.Path, "/")
	for i, segment := range segments {
		if segment == docUserSegment {
			segments[i] = u.user
		}
	}
	name := strings.Join(segments, "/")

	var kind DocKind
	var m docMatch
	found := false
	for _, candidate := range u.kinds {
		if cm, ok := candidate.match(name); ok {
			kind, m, found = candidate, cm, true
			break
		}
	}
	if found && kind.OwnTools {
		return DocMoved{}, ownToolsError(kind)
	}
	if !found {
		return DocMoved{}, docRefErrorf(DocCodeWrongDir, "%s liegt in keiner Ablage einer Sorte (%s) und nicht in deren done/",
			ref.Raw, strings.Join(DocKindNames(u.kinds), ", "))
	}
	if !kind.Done {
		return DocMoved{}, docRefErrorf(DocCodeDoneUnsupported, "die Sorte %s hat kein %s/", kind.Kind, docDoneDir)
	}
	if kind.UserArea && m.user != u.user {
		return DocMoved{}, docRefErrorf(DocCodeForeignUser, "%s gehört %s, nicht %s; verschoben werden nur eigene Dokumente",
			ref.Raw, m.user, u.user)
	}
	dir := kind.dirFor(m.user)
	var from, to string
	switch {
	case toDone && m.done:
		return DocMoved{}, docRefErrorf(DocCodeWrongDir, "%s liegt schon in %s/", ref.Raw, docDoneDir)
	case !toDone && !m.done:
		return DocMoved{}, docRefErrorf(DocCodeWrongDir, "%s liegt nicht in %s/", ref.Raw, docDoneDir)
	case toDone:
		from, to = dir+"/"+m.file, dir+"/"+docDoneDir+"/"+m.file
	default:
		from, to = dir+"/"+docDoneDir+"/"+m.file, dir+"/"+m.file
	}
	doc, err := u.store.Rename(ctx, from, to)
	if err != nil {
		return DocMoved{}, kephError(err)
	}
	if doc.Name != "" {
		to = doc.Name
	}
	return DocMoved{Ref: DocSchemeKeph + ":/" + to, From: DocSchemeKeph + ":/" + from, Kind: kind.Kind}, nil
}
