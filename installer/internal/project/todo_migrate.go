package project

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Übernahme von data/todos.json in die Collection (`k-playbook todo
// migrate`, Task 097, Etappe 6). Allein dieser Weg liest die Datei.
//
// Erstlauf oder Nachzügler entscheidet der Inhalt (Task 112): Ein Lauf ist
// ein Nachzügler, wenn data/todos.json.migriert liegt oder unter todos/ bzw.
// todos/done/ ein Dokument liegt, das nicht zu todos.json passt — seine
// Nummer kommt dort nicht vor, oder sie kommt vor, aber mit anderem Text.
// Sonst ist es ein Erstlauf: bei leerer Collection oder wenn jedes Dokument
// passt, etwa nach einem abgebrochenen Erstlauf. Die Merkdatei allein reicht
// nicht, weil /k-upgrade 3.4 data/ samt den umbenannten Dateien abräumt.
//
// Erstlauf: Jeder Eintrag wird ein Dokument mit festem Namen <id
// dreistellig>-<kurzname>.md, nicht über die Nummernvergabe — so bleiben die
// bisherigen Nummern erhalten. Übersprungen wird ein Eintrag, dessen Nummer
// schon mit demselben Text als Dokument vorliegt; so setzt ein Lauf nach
// einem Abbruch den Erstlauf fort.
//
// Nachzügler (etwa weil ein älteres Binary nach der Übernahme wieder nach
// data/todos.json geschrieben hat, oder weil die Collection schon eigene
// Todos hat): Die Einträge werden über die normale Nummernvergabe angehängt,
// ihre alten Nummern bleiben nicht erhalten; übersprungen wird nur, wessen
// Text schon als Dokument vorliegt, damit nichts doppelt angelegt wird —
// auch nicht beim Wiederholen nach einem Abbruch. „Doppelt“ heißt gleicher
// Text: Wurde ein Dokument nach einem abgebrochenen Erstlauf bearbeitet,
// steht sein alter Text danach als neuer Eintrag daneben.
//
// Erst wenn alle Einträge angekommen sind, wird die Datei umbenannt: beim
// Erstlauf nach todos.json.migriert, ein Nachzügler nach
// todos.json.nachzuegler-<datum>[-<n>]. Ein vorhandener Name wird nie
// überschrieben; umbenannte Dateien liest niemand mehr.

// Die Arten eines Laufs von MigrateTodos.
const (
	TodoMigrationNone      = "none"
	TodoMigrationFirst     = "first"
	TodoMigrationLatecomer = "latecomer"
)

// legacyTodoDocument ist der Inhalt von data/todos.json (TodoSchemaVersion).
type legacyTodoDocument struct {
	SchemaVersion int          `json:"schemaVersion"`
	NextID        int          `json:"nextId"`
	MigratedOn    string       `json:"migratedOn,omitempty"`
	Todos         []legacyTodo `json:"todos"`
}

type legacyTodo struct {
	ID           int    `json:"id"`
	Text         string `json:"text"`
	Created      string `json:"created"`
	Done         string `json:"done,omitempty"`
	DoneMigrated bool   `json:"doneMigrated,omitempty"`
	Origin       string `json:"origin,omitempty"`
}

// TodoMigrationEntry ist ein Eintrag in der Ausgabe von MigrateTodos: die
// Kennung aus todos.json, der Name des Dokuments und bei übersprungenen der
// Grund.
type TodoMigrationEntry struct {
	SourceID int    `json:"sourceId"`
	ID       int    `json:"id,omitempty"`
	Name     string `json:"name,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// TodoMigration ist die Ausgabe von MigrateTodos.
type TodoMigration struct {
	// Mode ist none (keine data/todos.json), first oder latecomer.
	Mode    string               `json:"mode"`
	Source  string               `json:"source,omitempty"`
	Created []TodoMigrationEntry `json:"created"`
	Skipped []TodoMigrationEntry `json:"skipped"`
	// RenamedTo ist der neue Name der Datei, relativ zum Projekt; leer, wenn
	// nicht umbenannt wurde.
	RenamedTo string `json:"renamedTo,omitempty"`
}

// TodoMigratedFile ist der Name, unter dem der erste Lauf die Datei ablegt.
func TodoMigratedFile(projectDir string) string {
	return filepath.Join(TodoDataDir(projectDir), TodoMigratedFileName)
}

// todoRel ist ein Pfad unter data/, relativ zum Projekt geschrieben.
func todoRel(name string) string {
	return LocalDirName + "/" + TodoDataDirName + "/" + name
}

// readLegacyTodos liest data/todos.json und prüft die Einträge.
func readLegacyTodos(path string) ([]legacyTodo, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%s lesen: %w", path, err)
	}
	var document legacyTodoDocument
	if err := json.Unmarshal(content, &document); err != nil {
		return nil, fmt.Errorf("%s lesen: %w", path, err)
	}
	if document.SchemaVersion > TodoSchemaVersion {
		return nil, fmt.Errorf("%s hat die Fassung %d, gelesen wird bis %d", path, document.SchemaVersion, TodoSchemaVersion)
	}
	seen := map[int]bool{}
	for _, todo := range document.Todos {
		if todo.ID < 1 {
			return nil, fmt.Errorf("%s: ein Eintrag hat die Kennung %d", path, todo.ID)
		}
		if seen[todo.ID] {
			return nil, fmt.Errorf("%s: die Kennung %d steht zweimal da", path, todo.ID)
		}
		seen[todo.ID] = true
		if strings.TrimSpace(todo.Text) == "" {
			return nil, fmt.Errorf("%s: der Eintrag %d hat keinen Text", path, todo.ID)
		}
	}
	todos := append([]legacyTodo{}, document.Todos...)
	sort.SliceStable(todos, func(i, j int) bool { return todos[i].ID < todos[j].ID })
	return todos, nil
}

// legacyTodoDoc ist das Dokument eines Eintrags: created steht immer da,
// auch leer.
func legacyTodoDoc(todo legacyTodo) todoDoc {
	doc := todoDoc{Text: strings.TrimSpace(strings.ReplaceAll(todo.Text, "\r\n", "\n")), Origin: strings.TrimSpace(todo.Origin),
		Created: strings.TrimSpace(todo.Created), HasCreated: true}
	if strings.TrimSpace(todo.Done) != "" {
		doc.Done = strings.TrimSpace(todo.Done)
		doc.DoneMigrated = todo.DoneMigrated
	}
	return doc
}

// MigrateTodos übernimmt data/todos.json in die Collection, nach den Regeln
// oben. Bei einem Fehler steht in der Ausgabe, was bis dahin angelegt wurde;
// die Datei bleibt dann liegen, und ein neuer Lauf setzt fort.
func MigrateTodos(ctx context.Context, projectDir string) (TodoMigration, error) {
	result := TodoMigration{Mode: TodoMigrationNone, Created: []TodoMigrationEntry{}, Skipped: []TodoMigrationEntry{}}
	source := TodoFile(projectDir)
	if !fileExists(source) {
		return result, nil
	}
	result.Source = todoRel(TodoFileName)
	todos, err := readLegacyTodos(source)
	if err != nil {
		return result, err
	}
	access, err := openTodos(projectDir)
	if err != nil {
		return result, err
	}
	existing, err := access.loadAll(ctx)
	if err != nil {
		return result, err
	}

	if fileExists(TodoMigratedFile(projectDir)) || !matchesLegacyTodos(todos, existing) {
		result.Mode = TodoMigrationLatecomer
		err = access.migrateLatecomer(ctx, todos, existing, &result)
	} else {
		result.Mode = TodoMigrationFirst
		err = access.migrateFirst(ctx, todos, existing, &result)
	}
	if err != nil {
		return result, err
	}

	target, err := migrationTarget(projectDir, result.Mode)
	if err != nil {
		return result, err
	}
	if err := os.Rename(source, target); err != nil {
		return result, fmt.Errorf("alle Einträge sind übernommen, aber %s ließ sich nicht umbenennen: %w", result.Source, err)
	}
	result.RenamedTo = todoRel(filepath.Base(target))
	return result, nil
}

// migrationTarget ist der neue Name der Datei; ein vorhandener wird nie
// genommen.
func migrationTarget(projectDir, mode string) (string, error) {
	if mode == TodoMigrationFirst {
		target := TodoMigratedFile(projectDir)
		if _, err := os.Lstat(target); err == nil {
			return "", fmt.Errorf("%s gibt es schon; sie wird nie überschrieben", todoRel(TodoMigratedFileName))
		}
		return target, nil
	}
	base := TodoFileName + ".nachzuegler-" + today()
	for n := 1; n < 1000; n++ {
		name := base
		if n > 1 {
			name = fmt.Sprintf("%s-%d", base, n)
		}
		target := filepath.Join(TodoDataDir(projectDir), name)
		if _, err := os.Lstat(target); os.IsNotExist(err) {
			return target, nil
		}
	}
	return "", fmt.Errorf("kein freier Name für den Nachzügler unter %s", todoRel(""))
}

// matchesLegacyTodos sagt, ob jedes Dokument der Collection zu einem Eintrag
// aus todos.json passt: gleiche Nummer, gleicher Text. Nur dann ist ein Lauf
// ohne todos.json.migriert ein Erstlauf.
func matchesLegacyTodos(todos []legacyTodo, existing []todoEntry) bool {
	texts := map[int]string{}
	for _, todo := range todos {
		texts[todo.ID] = legacyTodoDoc(todo).Text
	}
	for _, entry := range existing {
		text, ok := texts[entry.todo.ID]
		if !ok || entry.todo.Text != text {
			return false
		}
	}
	return true
}

// migrateFirst ist der Erstlauf: Jedes vorhandene Dokument passt zu
// todos.json (matchesLegacyTodos); angelegt wird, was noch fehlt, unter der
// alten Nummer.
func (a *todoAccess) migrateFirst(ctx context.Context, todos []legacyTodo, existing []todoEntry, result *TodoMigration) error {
	byNumber := map[int]string{}
	for _, entry := range existing {
		if _, ok := byNumber[entry.todo.ID]; !ok {
			byNumber[entry.todo.ID] = entry.name
		}
	}

	var pending []legacyTodo
	for _, todo := range todos {
		name, ok := byNumber[todo.ID]
		if !ok {
			pending = append(pending, todo)
			continue
		}
		result.Skipped = append(result.Skipped, TodoMigrationEntry{SourceID: todo.ID, ID: todo.ID, Name: name,
			Reason: "liegt mit demselben Text schon vor"})
	}

	for _, todo := range pending {
		doc := legacyTodoDoc(todo)
		dir := a.kind.Dir
		if doc.Done != "" {
			dir += "/" + docDoneDir
		}
		file := docFileName(todo.ID, todoShortName(doc.Text))
		if !docNamePattern.MatchString(file) {
			return docRefErrorf(DocCodeInvalidInput, "Eintrag %d: Dateiname %q passt nicht in die Namensregel von Kephalaion", todo.ID, file)
		}
		name := dir + "/" + file
		if _, err := a.store.Create(ctx, name, composeTodoDoc(doc)); err != nil {
			return fmt.Errorf("Eintrag %d nach %s: %w", todo.ID, name, todoKephError(err))
		}
		result.Created = append(result.Created, TodoMigrationEntry{SourceID: todo.ID, ID: todo.ID, Name: name})
	}
	return nil
}

// migrateLatecomer hängt die Einträge eines Nachzüglers über die normale
// Nummernvergabe an.
func (a *todoAccess) migrateLatecomer(ctx context.Context, todos []legacyTodo, existing []todoEntry, result *TodoMigration) error {
	texts := map[string]string{}
	for _, entry := range existing {
		texts[entry.todo.Text] = entry.name
	}
	for _, todo := range todos {
		doc := legacyTodoDoc(todo)
		if name, ok := texts[doc.Text]; ok {
			result.Skipped = append(result.Skipped, TodoMigrationEntry{SourceID: todo.ID, Name: name,
				Reason: "der Text liegt schon als Dokument vor"})
			continue
		}
		dir := a.kind.Dir
		if doc.Done != "" {
			dir += "/" + docDoneDir
		}
		created, _, err := a.docs.createNumbered(ctx, a.kind, dir, todoShortName(doc.Text), composeTodoDoc(doc), todoKephError)
		if err != nil {
			return fmt.Errorf("Eintrag %d des Nachzüglers: %w", todo.ID, err)
		}
		name := dir + "/" + created.Name
		texts[doc.Text] = name
		result.Created = append(result.Created, TodoMigrationEntry{SourceID: todo.ID, ID: created.Number, Name: name})
	}
	return nil
}
