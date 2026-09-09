package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// TodoDataDirName ist das Verzeichnis der Maschinendateien, die k-playbook
// selbst besitzt. Es wird mitversioniert.
const TodoDataDirName = "data"

// TodoFileName ist die Ablage der Todos: eine JSON-Datei, die Go besitzt.
// Geschrieben wird sie über /k-todo, `k-playbook todo` oder die Oberfläche,
// nie von Hand.
const TodoFileName = "todos.json"

// LegacyTodoFileName ist die frühere Ablage als Markdown-Checkliste. Sie wird
// beim ersten Zugriff nach data/todos.json übersetzt und danach entfernt; der
// Name lebt nur noch in Migration und Import weiter.
const LegacyTodoFileName = "TODO.md"

// TodoSchemaVersion ist die Fassung des Dokuments. Sie steht in der Datei,
// damit ein späterer Formatwechsel den Bestand erkennen kann.
const TodoSchemaVersion = 1

// Todo ist ein einzelner Eintrag.
//
// Done trägt das Datum des Abhakens und ist bei offenen Einträgen leer — ein
// eigenes Status-Feld gäbe es zweimal und könnte sich widersprechen.
type Todo struct {
	ID      int    `json:"id"`
	Text    string `json:"text"`
	Created string `json:"created"`
	Done    string `json:"done,omitempty"`
	// DoneMigrated steht nur an Einträgen, deren Done aus einer Migration oder
	// einem Import stammt, und weist es als Migrationsdatum aus — nicht als
	// Tag, an dem jemand den Punkt wirklich abgehakt hat.
	DoneMigrated bool `json:"doneMigrated,omitempty"`
	// Origin ist die optionale Herkunftsnotiz, etwa "Task 026".
	Origin string `json:"origin,omitempty"`
}

// TodoDocument ist der gesamte Bestand.
//
// NextID sichert zu, dass eine einmal vergebene Kennung nie wieder vergeben
// wird — auch nicht nach einem Löschen. MigratedOn ist die Herkunftsnotiz des
// Dokuments: gesetzt, wenn es aus einer TODO.md entstanden ist. Über einzelne
// Einträge sagt es nichts, das tut DoneMigrated am Eintrag.
type TodoDocument struct {
	SchemaVersion int    `json:"schemaVersion"`
	NextID        int    `json:"nextId"`
	MigratedOn    string `json:"migratedOn,omitempty"`
	Todos         []Todo `json:"todos"`
}

// TodoNotice trägt, was ein Zugriff nebenbei zu melden hat. Beides ist kein
// Fehler: der Zugriffsweg scheitert nicht, weil eine Migration lief oder weil
// noch eine alte Datei danebenliegt.
type TodoNotice struct {
	// Migrated ist die Zahl der Einträge, die dieser Zugriff aus einer
	// TODO.md übersetzt hat. Null, wenn nichts zu migrieren war.
	Migrated int `json:"migrated,omitempty"`
	// Hint meldet eine zurückgebliebene TODO.md samt Ausweg. Er gehört nie
	// nach message: dort gilt er als Fehler und die Liste verschwände.
	Hint string `json:"hint,omitempty"`
}

// TodoDataDir ist das Verzeichnis der Maschinendateien eines Projekts.
func TodoDataDir(projectDir string) string {
	return filepath.Join(LocalDir(projectDir), TodoDataDirName)
}

// TodoFile ist die Todo-Ablage eines Projekts: data/todos.json.
func TodoFile(projectDir string) string {
	return filepath.Join(TodoDataDir(projectDir), TodoFileName)
}

// LegacyTodoFile ist die frühere Markdown-Ablage eines Projekts.
func LegacyTodoFile(projectDir string) string {
	return filepath.Join(LocalDir(projectDir), LegacyTodoFileName)
}

// ListTodos sammelt die offenen Einträge in Dokumentreihenfolge — neue
// Einträge kommen hinten an, der älteste steht damit oben.
func ListTodos(projectDir string) ([]Todo, TodoNotice, error) {
	document, notice, err := readTodos(projectDir)
	if err != nil {
		return nil, notice, err
	}
	return filterTodos(document, false), notice, nil
}

// ListDoneTodos sammelt die abgehakten Einträge, ebenfalls in
// Dokumentreihenfolge.
func ListDoneTodos(projectDir string) ([]Todo, TodoNotice, error) {
	document, notice, err := readTodos(projectDir)
	if err != nil {
		return nil, notice, err
	}
	return filterTodos(document, true), notice, nil
}

// ListAllTodos liefert den ganzen Bestand, offene und erledigte.
func ListAllTodos(projectDir string) ([]Todo, TodoNotice, error) {
	document, notice, err := readTodos(projectDir)
	if err != nil {
		return nil, notice, err
	}
	todos := make([]Todo, len(document.Todos))
	copy(todos, document.Todos)
	return todos, notice, nil
}

func filterTodos(document *TodoDocument, done bool) []Todo {
	found := []Todo{}
	for _, todo := range document.Todos {
		if (todo.Done != "") == done {
			found = append(found, todo)
		}
	}
	return found
}

// AddTodo hängt einen neuen Eintrag hinten an und vergibt die nächste Kennung.
func AddTodo(projectDir, text, origin string) (Todo, TodoNotice, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return Todo{}, TodoNotice{}, fmt.Errorf("leerer Text: ein Todo braucht einen Inhalt")
	}

	document, notice, err := readTodos(projectDir)
	if err != nil {
		return Todo{}, notice, err
	}

	todo := Todo{
		ID:      document.NextID,
		Text:    text,
		Created: today(),
		Origin:  strings.TrimSpace(origin),
	}
	document.Todos = append(document.Todos, todo)
	document.NextID++

	if err := writeTodos(projectDir, document); err != nil {
		return Todo{}, notice, err
	}
	return todo, notice, nil
}

// UpdateTodo ändert Text und Erledigt-Zustand eines Eintrags. Beide Argumente
// sind optional: nil lässt das Feld, wie es ist.
//
// Abhaken setzt das heutige Datum und entfernt DoneMigrated — ein von Hand
// abgehakter Eintrag trägt kein Migrationsdatum mehr. Wiederöffnen leert Done.
func UpdateTodo(projectDir string, id int, text *string, done *bool) (Todo, TodoNotice, error) {
	document, notice, err := readTodos(projectDir)
	if err != nil {
		return Todo{}, notice, err
	}

	index := indexOfTodo(document, id)
	if index < 0 {
		return Todo{}, notice, unknownTodoError(id)
	}

	if text != nil {
		trimmed := strings.TrimSpace(*text)
		if trimmed == "" {
			return Todo{}, notice, fmt.Errorf("leerer Text: ein Todo braucht einen Inhalt")
		}
		document.Todos[index].Text = trimmed
	}
	if done != nil {
		if *done {
			document.Todos[index].Done = today()
		} else {
			document.Todos[index].Done = ""
		}
		document.Todos[index].DoneMigrated = false
	}

	if err := writeTodos(projectDir, document); err != nil {
		return Todo{}, notice, err
	}
	return document.Todos[index], notice, nil
}

// DeleteTodo entfernt einen Eintrag physisch. NextID bleibt stehen, damit die
// Kennung nie wieder vergeben wird.
func DeleteTodo(projectDir string, id int) (Todo, TodoNotice, error) {
	document, notice, err := readTodos(projectDir)
	if err != nil {
		return Todo{}, notice, err
	}

	index := indexOfTodo(document, id)
	if index < 0 {
		return Todo{}, notice, unknownTodoError(id)
	}

	removed := document.Todos[index]
	document.Todos = append(document.Todos[:index], document.Todos[index+1:]...)

	if err := writeTodos(projectDir, document); err != nil {
		return Todo{}, notice, err
	}
	return removed, notice, nil
}

// ImportTodos hängt die Einträge einer Markdown-Checkliste hinten an das
// bestehende Dokument an und entfernt die Datei danach.
//
// Die Kennungen werden fortlaufend ab NextID vergeben, vorhandene bleiben
// unberührt und die Reihenfolge der bestehenden Einträge bleibt. Created bleibt
// leer, Done trägt bei "[x]" das heutige Datum und DoneMigrated. MigratedOn
// fasst der Import nicht an: es ist die Herkunftsnotiz des Dokuments und wird
// allein beim Anlegen gesetzt. Gibt es noch kein Dokument, verhält sich der
// Import wie die Migration.
func ImportTodos(projectDir, markdownPath string) (int, TodoNotice, error) {
	// Erst der gemeinsame Zugriffsweg, dann die Quelldatei: gab es noch kein
	// Dokument, hat die Migration genau diese TODO.md eben übersetzt und
	// entfernt — der Import ist dann schon geschehen.
	document, notice, err := readTodos(projectDir)
	if err != nil {
		return 0, notice, err
	}

	content, err := os.ReadFile(markdownPath)
	if err != nil {
		if os.IsNotExist(err) && notice.Migrated > 0 && sameFilePath(markdownPath, LegacyTodoFile(projectDir)) {
			return notice.Migrated, notice, nil
		}
		return 0, notice, fmt.Errorf("%s lesen: %w", markdownPath, err)
	}

	parsed := parseTodoMarkdown(string(content))
	stamp := today()
	for _, entry := range parsed {
		todo := Todo{ID: document.NextID, Text: entry.text}
		if entry.done {
			todo.Done = stamp
			todo.DoneMigrated = true
		}
		document.Todos = append(document.Todos, todo)
		document.NextID++
	}

	if err := writeTodos(projectDir, document); err != nil {
		return 0, notice, err
	}
	if err := os.Remove(markdownPath); err != nil && !os.IsNotExist(err) {
		return len(parsed), notice, fmt.Errorf("%s entfernen: %w", markdownPath, err)
	}
	// Die importierte Datei ist weg; ein Hinweis auf sie wäre jetzt falsch.
	notice.Hint = ""
	return len(parsed), notice, nil
}

// readTodos ist der gemeinsame Zugriffsweg von GUI, Subkommando und
// MCP-Hüllen. Jeder Zugriff läuft darüber, auch jeder schreibende: lesen,
// migrieren, ändern, schreiben, in dieser Reihenfolge.
//
// Ohne diese Festlegung legte ein `add` in einem Projekt mit vorhandener
// TODO.md ein leeres data/todos.json daneben an — genau der Zustand, den die
// Migration danach dauerhaft ablehnt.
//
// Eine fehlende Ablage ist kein Fehler: "noch keine Todos" ist dieselbe
// Auskunft wie ein leeres Dokument.
func readTodos(projectDir string) (*TodoDocument, TodoNotice, error) {
	notice := TodoNotice{}

	jsonPath := TodoFile(projectDir)
	legacyPath := LegacyTodoFile(projectDir)
	jsonThere := fileExists(jsonPath)
	legacyThere := fileExists(legacyPath)

	switch {
	case !jsonThere && legacyThere:
		count, err := migrateTodos(projectDir)
		if err != nil {
			return nil, notice, err
		}
		notice.Migrated = count
	case jsonThere && legacyThere:
		// Der Zugriff scheitert nicht: gelesen und geschrieben wird
		// data/todos.json, die zurückgebliebene Datei ist ein Hinweis.
		notice.Hint = legacyTodoHint(projectDir)
	}

	document, err := readTodoDocument(jsonPath)
	if err != nil {
		return nil, notice, err
	}
	return document, notice, nil
}

// readTodoDocument liest data/todos.json. Fehlt die Datei, ist das Dokument
// leer.
func readTodoDocument(path string) (*TodoDocument, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return emptyTodoDocument(), nil
		}
		return nil, fmt.Errorf("%s lesen: %w", path, err)
	}

	document := &TodoDocument{}
	if err := json.Unmarshal(content, document); err != nil {
		return nil, fmt.Errorf("%s lesen: %w", path, err)
	}
	if document.Todos == nil {
		document.Todos = []Todo{}
	}
	if document.SchemaVersion == 0 {
		document.SchemaVersion = TodoSchemaVersion
	}
	if document.NextID < 1 {
		document.NextID = nextTodoID(document.Todos)
	}
	return document, nil
}

// writeTodos schreibt das Dokument atomar (siehe writeJSONFileAtomic) und
// zieht dabei Schemafassung und nächste Kennung nach.
func writeTodos(projectDir string, document *TodoDocument) error {
	document.SchemaVersion = TodoSchemaVersion
	if document.NextID < nextTodoID(document.Todos) {
		document.NextID = nextTodoID(document.Todos)
	}
	return writeJSONFileAtomic(TodoFile(projectDir), document, "Todos schreiben")
}

// migrateTodos übersetzt eine vorhandene TODO.md nach data/todos.json und
// entfernt sie danach.
//
// Erfunden wird dabei nichts: Created bleibt leer, weil in der Markdown-Datei
// nicht steht, wann ein Eintrag entstand. Done kann nicht leer bleiben, weil
// leer "offen" heißt — abgehakte Einträge bekommen deshalb das Migrationsdatum,
// und DoneMigrated weist es als solches aus.
func migrateTodos(projectDir string) (int, error) {
	legacyPath := LegacyTodoFile(projectDir)
	if !fileExists(legacyPath) {
		return 0, nil
	}
	if fileExists(TodoFile(projectDir)) {
		return 0, fmt.Errorf("Migration abgelehnt: %s gibt es schon und %s liegt noch daneben. "+
			"`k-playbook todo import %s/%s` hängt die Einträge der Markdown-Datei an das "+
			"bestehende Dokument an und entfernt sie danach",
			TodoFile(projectDir), legacyPath, LocalDirName, LegacyTodoFileName)
	}

	content, err := os.ReadFile(legacyPath)
	if err != nil {
		return 0, fmt.Errorf("%s lesen: %w", legacyPath, err)
	}

	stamp := today()
	document := emptyTodoDocument()
	document.MigratedOn = stamp
	for _, entry := range parseTodoMarkdown(string(content)) {
		todo := Todo{ID: document.NextID, Text: entry.text}
		if entry.done {
			todo.Done = stamp
			todo.DoneMigrated = true
		}
		document.Todos = append(document.Todos, todo)
		document.NextID++
	}

	if err := writeTodos(projectDir, document); err != nil {
		return 0, err
	}
	if err := os.Remove(legacyPath); err != nil && !os.IsNotExist(err) {
		return len(document.Todos), fmt.Errorf("%s entfernen: %w", legacyPath, err)
	}
	return len(document.Todos), nil
}

// legacyTodoHint beschreibt den Zustand "beide Dateien da" und nennt den
// Ausweg wörtlich.
func legacyTodoHint(projectDir string) string {
	return fmt.Sprintf("%s liegt noch neben %s. Gelesen und geschrieben wird die JSON-Datei; "+
		"`k-playbook todo import %s/%s` hängt die Einträge der Markdown-Datei an und entfernt sie danach.",
		LegacyTodoFile(projectDir), TodoFile(projectDir), LocalDirName, LegacyTodoFileName)
}

// sameFilePath vergleicht zwei Pfade, ohne dass die Dateien existieren müssen.
func sameFilePath(left, right string) bool {
	leftAbs, err := filepath.Abs(left)
	if err != nil {
		return false
	}
	rightAbs, err := filepath.Abs(right)
	if err != nil {
		return false
	}
	return filepath.Clean(leftAbs) == filepath.Clean(rightAbs)
}

func emptyTodoDocument() *TodoDocument {
	return &TodoDocument{SchemaVersion: TodoSchemaVersion, NextID: 1, Todos: []Todo{}}
}

func indexOfTodo(document *TodoDocument, id int) int {
	for index, todo := range document.Todos {
		if todo.ID == id {
			return index
		}
	}
	return -1
}

func unknownTodoError(id int) error {
	return fmt.Errorf("kein Todo mit der Kennung %d", id)
}

func nextTodoID(todos []Todo) int {
	next := 1
	for _, todo := range todos {
		if todo.ID >= next {
			next = todo.ID + 1
		}
	}
	return next
}

func today() string {
	return time.Now().Format("2006-01-02")
}

// markdownTodo ist eine geparste Checkbox-Zeile.
type markdownTodo struct {
	text string
	done bool
}

// parseTodoMarkdown liest eine Markdown-Checkliste in Dateireihenfolge. Alles
// andere — Überschrift, Fließtext, Leerzeilen — ist kein Eintrag.
//
// Der Parser bleibt allein für Migration und Import; der laufende Betrieb
// kennt nur noch data/todos.json.
func parseTodoMarkdown(content string) []markdownTodo {
	todos := []markdownTodo{}
	for line := range strings.SplitSeq(content, "\n") {
		if text, done, ok := parseTodoLine(strings.TrimSpace(line)); ok {
			todos = append(todos, markdownTodo{text: text, done: done})
		}
	}
	return todos
}

// parseTodoLine erkennt "- [ ] Text" und "- [x] Text"/"- [X] Text" — dieselbe
// Markdown-Checkliste, die /k-todo früher geschrieben hat.
func parseTodoLine(line string) (text string, done bool, ok bool) {
	if rest, found := strings.CutPrefix(line, "- [ ] "); found {
		return strings.TrimSpace(rest), false, true
	}
	if rest, found := strings.CutPrefix(line, "- [x] "); found {
		return strings.TrimSpace(rest), true, true
	}
	if rest, found := strings.CutPrefix(line, "- [X] "); found {
		return strings.TrimSpace(rest), true, true
	}
	return "", false, false
}
