package project

import (
	"os"
	"testing"
)

// todosFixture legt ein Projekt mit einer TODO.md an und gibt dessen
// Hauptverzeichnis zurück.
func todosFixture(t *testing.T, content string) string {
	t.Helper()

	root := t.TempDir()
	if err := os.MkdirAll(LocalDir(root), 0o755); err != nil {
		t.Fatalf("k-playbook-local anlegen: %v", err)
	}
	if err := os.WriteFile(TodoFile(root), []byte(content), 0o644); err != nil {
		t.Fatalf("TODO.md schreiben: %v", err)
	}
	return root
}

func TestListTodosNimmtNurOffeneInDateireihenfolge(t *testing.T) {
	root := todosFixture(t, "# TODO\n\n"+
		"Offene Punkte des Projekts. Einträge kommen über /k-todo hinzu.\n"+
		"- [ ] Erster Punkt\n"+
		"- [x] Schon erledigt\n"+
		"- [ ] Zweiter Punkt\n")

	todos, err := ListTodos(root)
	if err != nil {
		t.Fatalf("ListTodos: %v", err)
	}
	if len(todos) != 2 {
		t.Fatalf("erwartet 2 offene Todos, bekommen %d: %+v", len(todos), todos)
	}
	if todos[0].Text != "Erster Punkt" || todos[1].Text != "Zweiter Punkt" {
		t.Errorf("Reihenfolge und Text: %+v", todos)
	}
	for _, todo := range todos {
		if todo.Done {
			t.Errorf("als offen gelistet, aber Done=true: %+v", todo)
		}
	}
}

func TestListDoneTodosNimmtNurAbgehakte(t *testing.T) {
	root := todosFixture(t, "# TODO\n\n"+
		"- [ ] Offen\n"+
		"- [x] Klein abgehakt\n"+
		"- [X] Groß abgehakt\n")

	todos, err := ListDoneTodos(root)
	if err != nil {
		t.Fatalf("ListDoneTodos: %v", err)
	}
	if len(todos) != 2 {
		t.Fatalf("erwartet 2 erledigte Todos, bekommen %d: %+v", len(todos), todos)
	}
	if todos[0].Text != "Klein abgehakt" || todos[1].Text != "Groß abgehakt" {
		t.Errorf("Text: %+v", todos)
	}
	for _, todo := range todos {
		if !todo.Done {
			t.Errorf("als erledigt gelistet, aber Done=false: %+v", todo)
		}
	}
}

// Überschrift und Fließtext sind keine Einträge, auch wenn sie mit "- " oder
// eckigen Klammern arbeiten.
func TestListTodosIgnoriertFliesstextUndUeberschrift(t *testing.T) {
	root := todosFixture(t, "# TODO\n\n"+
		"Offene Punkte des Projekts. Einträge kommen über /k-todo hinzu.\n"+
		"- kein Checkbox-Eintrag\n"+
		"[ ] auch keiner\n"+
		"- [ ] Echter Eintrag\n")

	todos, err := ListTodos(root)
	if err != nil {
		t.Fatalf("ListTodos: %v", err)
	}
	if len(todos) != 1 || todos[0].Text != "Echter Eintrag" {
		t.Fatalf("erwartet genau den echten Eintrag, bekommen %+v", todos)
	}
}

// Eine fehlende TODO.md ist kein Fehler, sondern die Zahl null — die Datei
// entsteht erst mit dem ersten /k-todo-Aufruf.
func TestListTodosOhneDatei(t *testing.T) {
	root := t.TempDir()

	todos, err := ListTodos(root)
	if err != nil {
		t.Fatalf("ListTodos: %v", err)
	}
	if len(todos) != 0 {
		t.Fatalf("erwartet keine Todos, bekommen %+v", todos)
	}

	done, err := ListDoneTodos(root)
	if err != nil {
		t.Fatalf("ListDoneTodos: %v", err)
	}
	if len(done) != 0 {
		t.Fatalf("erwartet keine erledigten Todos, bekommen %+v", done)
	}
}
