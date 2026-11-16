package project

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
)

// Die Todos des Projekts liegen als je ein Dokument in der Collection des
// Projekts (Task 097, Entscheidung 2026-10-19): offen unter todos/, abgehakt
// unter todos/done/, projektweit, nicht im Userbereich. Die Nummer im
// Dateinamen ist die Kennung id. Das Dokumentformat steht in todo_doc.go.
//
// GUI, Subkommando und MCP-Hüllen rufen allein die Funktionen hier; sie
// gehen über den Kephalaion-Client. Listen und Lesen nehmen die Replica des
// Nodes, Anlegen, Ändern, Verschieben und Löschen gehen über den Hub. Ohne
// erreichbares Kephalaion gibt es keine Todos: Der Fehler trägt
// kephalaion_unavailable, ein Rückfall auf die Platte existiert nicht.
//
// Nummern werden wie bei den übrigen Sorten aus dem Verzeichnis selbst samt
// done/ gezählt. Eine Löschmarke sperrt keinen Namen und zählt nicht mit:
// Wird das Todo mit der höchsten Nummer gelöscht, bekommt das nächste neue
// dieselbe Nummer. Das ist hingenommen.

// todoShortNameWords ist die Zahl der Wörter, aus denen der Kurzname im
// Dateinamen gebildet wird, todoMaxShortName seine Höchstlänge.
const (
	todoShortNameWords = 6
	todoMaxShortName   = 40
)

// Todo ist ein einzelner Eintrag, so wie ihn alle Zugriffswege ausgeben.
//
// Done trägt das Datum des Abhakens und ist bei offenen Einträgen leer — ein
// eigenes Status-Feld gäbe es zweimal und könnte sich widersprechen.
type Todo struct {
	ID      int    `json:"id"`
	Text    string `json:"text"`
	Created string `json:"created"`
	Done    string `json:"done,omitempty"`
	// DoneMigrated steht nur an Einträgen, deren Done aus einer Migration
	// stammt, und weist es als Migrationsdatum aus — nicht als Tag, an dem
	// jemand den Punkt wirklich abgehakt hat.
	DoneMigrated bool `json:"doneMigrated,omitempty"`
	// Origin ist die optionale Herkunftsnotiz, etwa "Task 026".
	Origin string `json:"origin,omitempty"`
}

// TodoStore sind die Aufrufe an Kephalaion, die die Todos brauchen.
// *kephalaion.Client erfüllt sie; Tests setzen eine Fälschung ein. Die
// schreibenden tragen je Aufruf einen eigenen request_id und wiederholen nach
// offenem Ausgang selbst, genau einmal.
type TodoStore interface {
	DocStore
	Delete(ctx context.Context, name string, baseRevision int64) (kephalaion.Document, error)
}

// OpenTodoStore öffnet den Zugang zur Collection des Projekts; austauschbar,
// damit Tests den Node nicht fragen — auch die anderer Pakete (Hüllen,
// Subkommando, GUI), die dafür kephtest.Store einsetzen. Zurück kommen der Store und der User
// hinter dem Account. Ist kephalaion.ok nicht wahr, ist der Fehler
// DocCodeKephalaionUnavailable.
var OpenTodoStore = func(projectDir string) (TodoStore, string, error) {
	client, user, err := openKephClient(projectDir)
	if err != nil {
		return nil, "", err
	}
	return client, user, nil
}

// todoAccess ist der geöffnete Zugang: Store, Sorte und die Nummernvergabe
// der Sorten-Tabelle.
type todoAccess struct {
	docs  *UserDocs
	store TodoStore
	kind  DocKind
}

func openTodos(projectDir string) (*todoAccess, error) {
	store, user, err := OpenTodoStore(projectDir)
	if err != nil {
		return nil, err
	}
	docs, err := NewUserDocs(DocKinds, store, user)
	if err != nil {
		return nil, err
	}
	kind, ok := docs.anyKind(TodoKindName)
	if !ok {
		return nil, fmt.Errorf("Sorten-Tabelle ohne %s", TodoKindName)
	}
	return &todoAccess{docs: docs, store: store, kind: kind}, nil
}

// todoEntry ist ein gelesenes Todo-Dokument.
type todoEntry struct {
	todo     Todo
	name     string
	done     bool
	revision int64
	doc      todoDoc
	// fallbackDone ist das Datum, das ein Dokument in done/ ohne done:
	// bekommt: der Stempel updated.
	fallbackDone string
}

// loadAll liest alle Todos, offene und erledigte, sortiert nach Kennung.
// Dokumente ohne Nummer vorn im Namen gehören nicht dazu.
func (a *todoAccess) loadAll(ctx context.Context) ([]todoEntry, error) {
	entries, matches, err := a.docs.listKind(ctx, a.kind, a.docs.User(), false)
	if err != nil {
		return nil, err
	}
	var found []todoEntry
	for i, entry := range entries {
		number, ok := docNumber(matches[i].file)
		if !ok {
			continue
		}
		read, err := a.store.Read(ctx, entry.Name)
		if err != nil {
			return nil, todoKephError(err)
		}
		if read.Kind != kephalaion.ReadKindDocument || read.Content == nil {
			// Zwischen list und read gelöscht oder verschoben.
			continue
		}
		found = append(found, newTodoEntry(number, entry.Name, matches[i].done, read))
	}
	sort.SliceStable(found, func(i, j int) bool {
		if found[i].todo.ID != found[j].todo.ID {
			return found[i].todo.ID < found[j].todo.ID
		}
		return found[i].name < found[j].name
	})
	return found, nil
}

// newTodoEntry baut den Eintrag nach den Regeln des Formats: Ob offen oder
// erledigt, sagt das Verzeichnis; ein übrig gebliebenes done: unter todos/
// wird übergangen, ein fehlendes unter done/ durch den Stempel updated
// ersetzt.
func newTodoEntry(number int, name string, done bool, read kephalaion.ReadResult) todoEntry {
	doc := parseTodoDoc(*read.Content)
	entry := todoEntry{name: name, done: done, revision: read.Revision, doc: doc}
	todo := Todo{ID: number, Text: doc.Text, Origin: doc.Origin}
	switch {
	case doc.HasCreated:
		todo.Created = doc.Created
	case read.Created != nil:
		todo.Created = stampDate(read.Created.At)
	}
	if read.Updated != nil {
		entry.fallbackDone = stampDate(read.Updated.At)
	}
	if done {
		todo.Done = doc.Done
		if todo.Done == "" {
			todo.Done = entry.fallbackDone
		}
		todo.DoneMigrated = doc.DoneMigrated
	}
	entry.todo = todo
	return entry
}

// todoKephError übersetzt einen Fehler von Kephalaion wie kephError, mit
// dem Ausweg der Todo-Werkzeuge bei offenem Ausgang.
func todoKephError(err error) error {
	var refErr *DocRefError
	if errors.As(err, &refErr) {
		return err
	}
	if kephalaion.ErrorCode(err) == kephalaion.CodeOutcomeUnknown {
		return &DocRefError{Code: DocCodeOutcomeUnknown, err: fmt.Errorf(
			"%w Nachsehen mit k_playbook_todo_list (includeDone) bzw. k-playbook todo list --all, ob der Vorgang geschehen ist; "+
				"nicht noch einmal ausführen", err)}
	}
	return kephError(err)
}

// listTodos ist der gemeinsame Weg der Listen: Hinweis von der Platte,
// Einträge aus der Collection, gefiltert.
func listTodos(ctx context.Context, projectDir string, keep func(todoEntry) bool) ([]Todo, error) {
	access, err := openTodos(projectDir)
	if err != nil {
		return nil, err
	}
	entries, err := access.loadAll(ctx)
	if err != nil {
		return nil, err
	}
	todos := []Todo{}
	for _, entry := range entries {
		if keep(entry) {
			todos = append(todos, entry.todo)
		}
	}
	return todos, nil
}

// ListTodos sammelt die offenen Einträge nach Kennung — neue Einträge kommen
// hinten an, der älteste steht oben.
func ListTodos(ctx context.Context, projectDir string) ([]Todo, error) {
	return listTodos(ctx, projectDir, func(e todoEntry) bool { return !e.done })
}

// ListDoneTodos sammelt die abgehakten Einträge, ebenfalls nach Kennung.
func ListDoneTodos(ctx context.Context, projectDir string) ([]Todo, error) {
	return listTodos(ctx, projectDir, func(e todoEntry) bool { return e.done })
}

// ListAllTodos liefert den ganzen Bestand, offene und erledigte.
func ListAllTodos(ctx context.Context, projectDir string) ([]Todo, error) {
	return listTodos(ctx, projectDir, func(todoEntry) bool { return true })
}

// SearchTodos sucht über offene und erledigte Einträge: Ein Eintrag passt,
// wenn sein Text query enthält, ohne Rücksicht auf Groß- und
// Kleinschreibung. Kephalaion hat keine Suche; gefiltert wird hier, für alle
// Zugriffswege gleich. Ein leerer query passt auf alles.
func SearchTodos(ctx context.Context, projectDir, query string) ([]Todo, error) {
	needle := strings.ToLower(strings.TrimSpace(query))
	return listTodos(ctx, projectDir, func(e todoEntry) bool {
		return strings.Contains(strings.ToLower(e.todo.Text), needle)
	})
}

// todoShortName ist der Kurzname im Dateinamen: die ersten Wörter des
// Textes, normalisiert wie bei doc_create. Bleibt nichts übrig, heißt er
// todo.
func todoShortName(text string) string {
	words := strings.Fields(text)
	if len(words) > todoShortNameWords {
		words = words[:todoShortNameWords]
	}
	name, err := NormalizeDocName(strings.Join(words, " "))
	if err != nil {
		return "todo"
	}
	if len(name) > todoMaxShortName {
		name = strings.TrimRight(name[:todoMaxShortName], "-")
	}
	return name
}

// AddTodo legt ein Todo an, mit der nächsten Nummer aus todos/ samt done/.
// Ist der Name belegt, versucht es die nächste Nummer (wie doc_create); ein
// offener Ausgang zieht nie die nächste Nummer.
func AddTodo(ctx context.Context, projectDir, text, origin string) (Todo, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return Todo{}, docRefErrorf(DocCodeInvalidInput, "leerer Text: ein Todo braucht einen Inhalt")
	}
	access, err := openTodos(projectDir)
	if err != nil {
		return Todo{}, err
	}
	doc := todoDoc{Text: text, Origin: strings.TrimSpace(origin)}
	created, hubDoc, err := access.docs.createNumbered(ctx, access.kind, access.kind.Dir, todoShortName(text),
		composeTodoDoc(doc), todoKephError)
	if err != nil {
		return Todo{}, err
	}
	todo := Todo{ID: created.Number, Text: doc.Text, Origin: doc.Origin, Created: today()}
	if hubDoc.Created != nil {
		todo.Created = stampDate(hubDoc.Created.At)
	}
	return todo, nil
}

// find sucht ein Todo nach Kennung, offen oder erledigt.
func (a *todoAccess) find(ctx context.Context, id int) (todoEntry, error) {
	entries, err := a.loadAll(ctx)
	if err != nil {
		return todoEntry{}, err
	}
	var hits []todoEntry
	for _, entry := range entries {
		if entry.todo.ID == id {
			hits = append(hits, entry)
		}
	}
	switch len(hits) {
	case 0:
		return todoEntry{}, docRefErrorf(DocCodeNotFound, "kein Todo mit der Kennung %d", id)
	case 1:
		return hits[0], nil
	}
	names := make([]string, 0, len(hits))
	for _, hit := range hits {
		names = append(names, hit.name)
	}
	return todoEntry{}, docRefErrorf(DocCodeInvalidInput, "die Kennung %d ist mehrdeutig: %s — von Hand in Kephalaion auflösen",
		id, strings.Join(names, ", "))
}

// todoNames sind der Name eines Todos unter todos/ und unter todos/done/.
func (a *todoAccess) todoNames(name string) (open, done string) {
	file := name[strings.LastIndex(name, "/")+1:]
	return a.kind.Dir + "/" + file, a.kind.Dir + "/" + docDoneDir + "/" + file
}

// UpdateTodo ändert Text und Erledigt-Zustand eines Eintrags. Beide Argumente
// sind optional: nil lässt das Feld, wie es ist.
//
// Abhaken schreibt done: mit dem heutigen Datum und verschiebt danach nach
// done/ — in dieser Reihenfolge, weil rename kein Datum festhält.
// Wiederöffnen verschiebt zurück und entfernt das Feld danach. Beides
// entfernt doneMigrated: Ein von Hand abgehakter Eintrag trägt kein
// Migrationsdatum mehr. Scheitert der zweite Schritt, kommt dessen Code
// (etwa outcome_unknown); der erste bleibt stehen und wird beim Lesen
// übergangen. Jedes Schreiben bereinigt dabei ein übrig gebliebenes oder
// fehlendes done:.
func UpdateTodo(ctx context.Context, projectDir string, id int, text *string, done *bool) (Todo, error) {
	var newText string
	if text != nil {
		newText = strings.TrimSpace(*text)
		if newText == "" {
			return Todo{}, docRefErrorf(DocCodeInvalidInput, "leerer Text: ein Todo braucht einen Inhalt")
		}
	}
	if text == nil && done == nil {
		return Todo{}, docRefErrorf(DocCodeInvalidInput, "nichts zu ändern: erwartet wird text oder done")
	}
	access, err := openTodos(projectDir)
	if err != nil {
		return Todo{}, err
	}
	entry, err := access.find(ctx, id)
	if err != nil {
		return Todo{}, err
	}

	doc := entry.doc
	if text != nil {
		doc.Text = newText
	}
	toDone := entry.done
	if done != nil {
		toDone = *done
		doc.DoneMigrated = false
	}
	switch {
	case !toDone:
		doc.Done = ""
		doc.DoneMigrated = false
	case done != nil && *done:
		doc.Done = today()
	case doc.Done == "":
		// In done/ ohne done: — das Datum, das die Liste gezeigt hat.
		doc.Done = entry.fallbackDone
	}
	content := composeTodoDoc(doc)
	openName, doneName := access.todoNames(entry.name)

	writeIfChanged := func() error {
		if content == composeTodoDoc(entry.doc) {
			return nil
		}
		if _, err := access.store.Write(ctx, entry.name, content, entry.revision); err != nil {
			return todoKephError(err)
		}
		return nil
	}

	switch {
	case toDone && !entry.done:
		// Erst das Datum, dann verschieben.
		if err := writeIfChanged(); err != nil {
			return Todo{}, err
		}
		if _, err := access.store.Rename(ctx, entry.name, doneName); err != nil {
			return Todo{}, todoMoveError(err, "done: ist geschrieben, das Verschieben nach done/ nicht")
		}
	case !toDone && entry.done:
		// Erst zurück, dann das Feld entfernen.
		moved, err := access.store.Rename(ctx, entry.name, openName)
		if err != nil {
			return Todo{}, todoKephError(err)
		}
		if content != composeTodoDoc(entry.doc) {
			if _, err := access.store.Write(ctx, openName, content, moved.Revision); err != nil {
				return Todo{}, todoMoveError(err, "das Todo ist zurück unter todos/, done: ist noch nicht entfernt (wird beim Lesen übergangen)")
			}
		}
	default:
		if err := writeIfChanged(); err != nil {
			return Todo{}, err
		}
	}

	todo := Todo{ID: entry.todo.ID, Text: doc.Text, Origin: doc.Origin, Created: entry.todo.Created}
	if toDone {
		todo.Done = doc.Done
		todo.DoneMigrated = doc.DoneMigrated
	}
	return todo, nil
}

// todoMoveError ist der Fehler des zweiten Schritts von Abhaken oder
// Wiederöffnen: der Code wie sonst, die Meldung sagt, was schon geschehen
// ist.
func todoMoveError(err error, state string) error {
	mapped := todoKephError(err)
	return &DocRefError{Code: DocRefErrorCode(mapped), err: fmt.Errorf("%s: %w", state, mapped)}
}

// DeleteTodo entfernt einen Eintrag (Löschmarke in Kephalaion). Die Nummer
// kann danach wieder vergeben werden, wenn sie die höchste war.
func DeleteTodo(ctx context.Context, projectDir string, id int) (Todo, error) {
	access, err := openTodos(projectDir)
	if err != nil {
		return Todo{}, err
	}
	entry, err := access.find(ctx, id)
	if err != nil {
		return Todo{}, err
	}
	if _, err := access.store.Delete(ctx, entry.name, entry.revision); err != nil {
		return Todo{}, todoKephError(err)
	}
	return entry.todo, nil
}

func today() string {
	return time.Now().Format("2006-01-02")
}
