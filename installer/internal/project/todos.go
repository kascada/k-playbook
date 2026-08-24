package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// TodoFileName ist die Datei mit den offenen Punkten des Projekts, angelegt
// und ergänzt über /k-todo.
const TodoFileName = "TODO.md"

// Todo ist ein einzelner Eintrag aus TODO.md.
type Todo struct {
	// Text ist der Inhalt der Checkbox-Zeile, ohne "- [ ]" bzw. "- [x]".
	Text string `json:"text"`
	// Done: die Zeile trägt ein "x" statt eines Leerzeichens.
	Done bool `json:"done"`
}

// TodoFile ist die TODO.md eines Projekts.
func TodoFile(projectDir string) string {
	return filepath.Join(LocalDir(projectDir), TodoFileName)
}

// ListTodos sammelt die offenen Einträge aus TODO.md, in Dateireihenfolge —
// /k-todo hängt neue Einträge unten an, die älteste Nummer steht damit oben.
func ListTodos(projectDir string) ([]Todo, error) {
	todos, err := readTodos(projectDir)
	if err != nil {
		return nil, err
	}
	open := []Todo{}
	for _, todo := range todos {
		if !todo.Done {
			open = append(open, todo)
		}
	}
	return open, nil
}

// ListDoneTodos sammelt die abgehakten Einträge aus TODO.md, ebenfalls in
// Dateireihenfolge. /k-todo selbst hakt keinen Eintrag ab — das geschieht von
// Hand —, die Liste zeigt trotzdem, was schon erledigt ist.
func ListDoneTodos(projectDir string) ([]Todo, error) {
	todos, err := readTodos(projectDir)
	if err != nil {
		return nil, err
	}
	done := []Todo{}
	for _, todo := range todos {
		if todo.Done {
			done = append(done, todo)
		}
	}
	return done, nil
}

// readTodos liest TODO.md einmal und parst jede Checkbox-Zeile. Alles andere
// — Überschrift, Fließtext, Leerzeilen — ist kein Eintrag und wird
// übersprungen.
//
// Eine fehlende Datei ist kein Fehler: sie entsteht erst mit dem ersten
// /k-todo-Aufruf, und "noch keine Todos" ist dieselbe Auskunft wie eine leere
// Datei.
func readTodos(projectDir string) ([]Todo, error) {
	content, err := os.ReadFile(TodoFile(projectDir))
	if err != nil {
		if os.IsNotExist(err) {
			return []Todo{}, nil
		}
		return nil, fmt.Errorf("TODO.md lesen: %w", err)
	}

	todos := []Todo{}
	for line := range strings.SplitSeq(string(content), "\n") {
		if text, done, ok := parseTodoLine(strings.TrimSpace(line)); ok {
			todos = append(todos, Todo{Text: text, Done: done})
		}
	}
	return todos, nil
}

// parseTodoLine erkennt "- [ ] Text" und "- [x] Text"/"- [X] Text" —
// dieselbe Markdown-Checkliste, die /k-todo schreibt.
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
