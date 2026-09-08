package project

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// legacyFixture legt ein Projekt mit einer zu migrierenden TODO.md an und gibt
// dessen Hauptverzeichnis zurück. Der Dateiname steht hier bewusst: die
// Markdown-Ablage lebt nur noch als Quelle von Migration und Import.
func legacyFixture(t *testing.T, content string) string {
	t.Helper()

	root := t.TempDir()
	if err := os.MkdirAll(LocalDir(root), 0o755); err != nil {
		t.Fatalf("k-playbook-local anlegen: %v", err)
	}
	if err := os.WriteFile(LegacyTodoFile(root), []byte(content), 0o644); err != nil {
		t.Fatalf("TODO.md schreiben: %v", err)
	}
	return root
}

// jsonFixture legt ein Projekt mit einem bestehenden data/todos.json an.
func jsonFixture(t *testing.T, document *TodoDocument) string {
	t.Helper()

	root := t.TempDir()
	if err := writeTodos(root, document); err != nil {
		t.Fatalf("todos.json schreiben: %v", err)
	}
	return root
}

func readDocument(t *testing.T, root string) *TodoDocument {
	t.Helper()

	content, err := os.ReadFile(TodoFile(root))
	if err != nil {
		t.Fatalf("todos.json lesen: %v", err)
	}
	document := &TodoDocument{}
	if err := json.Unmarshal(content, document); err != nil {
		t.Fatalf("todos.json parsen: %v — %s", err, content)
	}
	return document
}

// Die Migration übersetzt alle drei Zeilenformen in Dateireihenfolge und
// entfernt die Markdown-Datei danach.
func TestMigrationUebersetztAlleZeilenformen(t *testing.T) {
	root := legacyFixture(t, "# TODO\n\n"+
		"Offene Punkte des Projekts.\n"+
		"- kein Checkbox-Eintrag\n"+
		"[ ] auch keiner\n"+
		"- [ ] Erster Punkt\n"+
		"- [x] Klein abgehakt\n"+
		"- [X] Groß abgehakt\n"+
		"- [ ] Zweiter Punkt\n")

	todos, notice, err := ListAllTodos(root)
	if err != nil {
		t.Fatalf("ListAllTodos: %v", err)
	}
	if notice.Migrated != 4 {
		t.Fatalf("Migrated = %d, erwartet 4", notice.Migrated)
	}
	if notice.Hint != "" {
		t.Errorf("Hinweis obwohl migriert: %q", notice.Hint)
	}
	if len(todos) != 4 {
		t.Fatalf("erwartet 4 Einträge, bekommen %d: %+v", len(todos), todos)
	}

	wantText := []string{"Erster Punkt", "Klein abgehakt", "Groß abgehakt", "Zweiter Punkt"}
	for index, want := range wantText {
		if todos[index].Text != want {
			t.Errorf("Eintrag %d: Text = %q, erwartet %q", index, todos[index].Text, want)
		}
		if todos[index].ID != index+1 {
			t.Errorf("Eintrag %d: ID = %d, erwartet %d", index, todos[index].ID, index+1)
		}
	}

	if pathExists(LegacyTodoFile(root)) {
		t.Error("TODO.md liegt nach der Migration noch da")
	}
	document := readDocument(t, root)
	if document.MigratedOn != today() {
		t.Errorf("MigratedOn = %q, erwartet %q", document.MigratedOn, today())
	}
	if document.NextID != 5 {
		t.Errorf("NextID = %d, erwartet 5", document.NextID)
	}
}

// Erfunden wird nichts: created bleibt leer, abgehakte Einträge tragen das
// Migrationsdatum und weisen es über doneMigrated als solches aus.
func TestMigrationErfindetKeineZeitstempel(t *testing.T) {
	root := legacyFixture(t, "- [ ] Offen\n- [x] Erledigt\n")

	todos, _, err := ListAllTodos(root)
	if err != nil {
		t.Fatalf("ListAllTodos: %v", err)
	}
	for _, todo := range todos {
		if todo.Created != "" {
			t.Errorf("Created = %q, erwartet leer: %+v", todo.Created, todo)
		}
	}
	if todos[0].Done != "" || todos[0].DoneMigrated {
		t.Errorf("offener Eintrag trägt Abhak-Daten: %+v", todos[0])
	}
	if todos[1].Done != today() || !todos[1].DoneMigrated {
		t.Errorf("abgehakter Eintrag: Done = %q, DoneMigrated = %v", todos[1].Done, todos[1].DoneMigrated)
	}
}

// Eine TODO.md ohne Einträge ergibt ein leeres Dokument — und wird trotzdem
// entfernt, sonst liefe die Migration bei jedem Zugriff erneut an.
func TestMigrationLeererDatei(t *testing.T) {
	root := legacyFixture(t, "# TODO\n\nNoch nichts.\n")

	todos, notice, err := ListTodos(root)
	if err != nil {
		t.Fatalf("ListTodos: %v", err)
	}
	if len(todos) != 0 {
		t.Errorf("erwartet keine Todos, bekommen %+v", todos)
	}
	if notice.Migrated != 0 {
		t.Errorf("Migrated = %d, erwartet 0", notice.Migrated)
	}
	if pathExists(LegacyTodoFile(root)) {
		t.Error("leere TODO.md liegt nach der Migration noch da")
	}
}

// Ohne jede Ablage ist die Antwort die Zahl null, kein Fehler — und es
// entsteht auch keine Datei.
func TestListTodosOhneAblage(t *testing.T) {
	root := t.TempDir()

	todos, _, err := ListTodos(root)
	if err != nil {
		t.Fatalf("ListTodos: %v", err)
	}
	if len(todos) != 0 {
		t.Fatalf("erwartet keine Todos, bekommen %+v", todos)
	}

	done, _, err := ListDoneTodos(root)
	if err != nil {
		t.Fatalf("ListDoneTodos: %v", err)
	}
	if len(done) != 0 {
		t.Fatalf("erwartet keine erledigten Todos, bekommen %+v", done)
	}
	if pathExists(TodoFile(root)) {
		t.Error("ein reines Lesen hat data/todos.json angelegt")
	}
}

// Liegen beide Dateien da, geht der Zugriff weiter: gelesen wird die
// JSON-Datei, die zurückgebliebene TODO.md ist ein Hinweis. Nur der
// Migrationsversuch selbst lehnt ab — und nennt den Ausweg.
func TestBeideDateienZugriffGehtWeiterMigrationLehntAb(t *testing.T) {
	root := jsonFixture(t, &TodoDocument{
		SchemaVersion: TodoSchemaVersion,
		NextID:        2,
		Todos:         []Todo{{ID: 1, Text: "Aus der JSON-Datei", Created: "2026-09-01"}},
	})
	if err := os.WriteFile(LegacyTodoFile(root), []byte("- [ ] Aus der Markdown-Datei\n"), 0o644); err != nil {
		t.Fatalf("TODO.md anlegen: %v", err)
	}

	todos, notice, err := ListTodos(root)
	if err != nil {
		t.Fatalf("ListTodos scheitert, obwohl der Zugriffsweg nicht scheitern darf: %v", err)
	}
	if len(todos) != 1 || todos[0].Text != "Aus der JSON-Datei" {
		t.Fatalf("erwartet den Eintrag aus der JSON-Datei, bekommen %+v", todos)
	}
	if notice.Hint == "" {
		t.Fatal("kein Hinweis auf die zurückgebliebene TODO.md")
	}
	for _, want := range []string{"TODO.md", "todos.json", "k-playbook todo import"} {
		if !strings.Contains(notice.Hint, want) {
			t.Errorf("Hinweis nennt %q nicht: %s", want, notice.Hint)
		}
	}

	_, err = migrateTodos(root)
	if err == nil {
		t.Fatal("Migration lief, obwohl beide Dateien da sind")
	}
	for _, want := range []string{"TODO.md", "todos.json", "k-playbook todo import k-playbook-local/TODO.md"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Ablehnung nennt %q nicht: %v", want, err)
		}
	}
}

// Auch der schreibende Weg migriert zuerst: sonst entstünde neben der TODO.md
// ein zweites, leeres Dokument — genau der Zustand, den die Migration danach
// dauerhaft ablehnt.
func TestAddMigriertZuerstUndLegtKeinZweitesDokumentAn(t *testing.T) {
	root := legacyFixture(t, "- [ ] Alter Punkt\n")

	todo, notice, err := AddTodo(root, "Neuer Punkt", "")
	if err != nil {
		t.Fatalf("AddTodo: %v", err)
	}
	if notice.Migrated != 1 {
		t.Errorf("Migrated = %d, erwartet 1", notice.Migrated)
	}
	if todo.ID != 2 {
		t.Errorf("ID = %d, erwartet 2", todo.ID)
	}
	if todo.Created != today() {
		t.Errorf("Created = %q, erwartet %q", todo.Created, today())
	}
	if pathExists(LegacyTodoFile(root)) {
		t.Error("TODO.md liegt nach dem Schreiben noch da")
	}

	todos, _, err := ListAllTodos(root)
	if err != nil {
		t.Fatalf("ListAllTodos: %v", err)
	}
	if len(todos) != 2 || todos[0].Text != "Alter Punkt" || todos[1].Text != "Neuer Punkt" {
		t.Fatalf("Bestand nach dem Anlegen: %+v", todos)
	}
}

func TestAddLehntLeerenTextAb(t *testing.T) {
	root := t.TempDir()

	if _, _, err := AddTodo(root, "   ", ""); err == nil {
		t.Fatal("leerer Text wurde angenommen")
	}
}

// Der Import hängt an, vergibt Kennungen ab nextId und lässt migratedOn in
// Ruhe: die Herkunftsnotiz gehört dem Dokument, nicht dem einzelnen Eintrag.
func TestImportHaengtAnUndLaesstMigratedOnInRuhe(t *testing.T) {
	root := jsonFixture(t, &TodoDocument{
		SchemaVersion: TodoSchemaVersion,
		NextID:        8,
		MigratedOn:    "2026-01-01",
		Todos: []Todo{
			{ID: 5, Text: "Bestehend offen", Created: "2026-08-01"},
			{ID: 7, Text: "Bestehend erledigt", Created: "2026-08-02", Done: "2026-08-30"},
		},
	})

	markdown := LegacyTodoFile(root)
	if err := os.WriteFile(markdown, []byte("- [ ] Import offen\n- [x] Import erledigt\n"), 0o644); err != nil {
		t.Fatalf("TODO.md anlegen: %v", err)
	}

	count, _, err := ImportTodos(root, markdown)
	if err != nil {
		t.Fatalf("ImportTodos: %v", err)
	}
	if count != 2 {
		t.Fatalf("count = %d, erwartet 2", count)
	}
	if pathExists(markdown) {
		t.Error("die importierte Markdown-Datei liegt noch da")
	}

	document := readDocument(t, root)
	if document.MigratedOn != "2026-01-01" {
		t.Errorf("MigratedOn = %q, der Import darf sie nicht anfassen", document.MigratedOn)
	}
	if document.NextID != 10 {
		t.Errorf("NextID = %d, erwartet 10", document.NextID)
	}

	wantIDs := []int{5, 7, 8, 9}
	wantText := []string{"Bestehend offen", "Bestehend erledigt", "Import offen", "Import erledigt"}
	if len(document.Todos) != 4 {
		t.Fatalf("erwartet 4 Einträge, bekommen %+v", document.Todos)
	}
	for index, todo := range document.Todos {
		if todo.ID != wantIDs[index] || todo.Text != wantText[index] {
			t.Errorf("Eintrag %d = %+v, erwartet ID %d / %q", index, todo, wantIDs[index], wantText[index])
		}
	}
	if document.Todos[1].DoneMigrated {
		t.Error("der bestehende erledigte Eintrag trägt doneMigrated")
	}
	if !document.Todos[3].DoneMigrated || document.Todos[3].Done != today() {
		t.Errorf("importierter erledigter Eintrag: %+v", document.Todos[3])
	}
}

// Ohne bestehendes Dokument verhält sich der Import wie die Migration.
func TestImportOhneBestehendesDokument(t *testing.T) {
	root := legacyFixture(t, "- [ ] Nur dieser\n")

	count, _, err := ImportTodos(root, LegacyTodoFile(root))
	if err != nil {
		t.Fatalf("ImportTodos: %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, erwartet 1", count)
	}
	todos, _, err := ListAllTodos(root)
	if err != nil {
		t.Fatalf("ListAllTodos: %v", err)
	}
	if len(todos) != 1 || todos[0].ID != 1 || todos[0].Text != "Nur dieser" {
		t.Fatalf("Bestand nach dem Import: %+v", todos)
	}
}

// Abhaken behält den Eintrag und setzt kein Migrationsdatum; Wiederöffnen
// leert das Feld wieder.
func TestUpdateHaktAbUndOeffnetWieder(t *testing.T) {
	root := legacyFixture(t, "- [ ] Erster\n- [x] Zweiter\n")

	todo, _, err := UpdateTodo(root, 1, nil, boolPtr(true))
	if err != nil {
		t.Fatalf("UpdateTodo: %v", err)
	}
	if todo.Done != today() || todo.DoneMigrated {
		t.Fatalf("abgehakt: %+v", todo)
	}

	todo, _, err = UpdateTodo(root, 2, nil, boolPtr(false))
	if err != nil {
		t.Fatalf("UpdateTodo: %v", err)
	}
	if todo.Done != "" || todo.DoneMigrated {
		t.Fatalf("wieder geöffnet: %+v", todo)
	}

	neu := "Neuer Text"
	todo, _, err = UpdateTodo(root, 1, &neu, nil)
	if err != nil {
		t.Fatalf("UpdateTodo: %v", err)
	}
	if todo.Text != neu || todo.Done == "" {
		t.Fatalf("Textänderung hat den Erledigt-Zustand angefasst: %+v", todo)
	}
}

func TestUpdateUndDeleteMeldenUnbekannteKennung(t *testing.T) {
	root := legacyFixture(t, "- [ ] Erster\n")

	if _, _, err := UpdateTodo(root, 99, nil, boolPtr(true)); err == nil {
		t.Error("UpdateTodo nahm eine unbekannte Kennung an")
	}
	if _, _, err := DeleteTodo(root, 99); err == nil {
		t.Error("DeleteTodo nahm eine unbekannte Kennung an")
	}
}

// Löschen entfernt den Eintrag physisch, nextId bleibt stehen: eine einmal
// vergebene Kennung wird nie wieder vergeben.
func TestDeleteHaeltNextIdFest(t *testing.T) {
	root := legacyFixture(t, "- [ ] Erster\n- [ ] Zweiter\n")

	if _, _, err := DeleteTodo(root, 2); err != nil {
		t.Fatalf("DeleteTodo: %v", err)
	}
	document := readDocument(t, root)
	if len(document.Todos) != 1 || document.Todos[0].ID != 1 {
		t.Fatalf("Bestand nach dem Löschen: %+v", document.Todos)
	}
	if document.NextID != 3 {
		t.Fatalf("NextID = %d, erwartet 3", document.NextID)
	}

	todo, _, err := AddTodo(root, "Nach dem Löschen", "")
	if err != nil {
		t.Fatalf("AddTodo: %v", err)
	}
	if todo.ID != 3 {
		t.Errorf("ID = %d, erwartet 3 — eine gelöschte Kennung wurde neu vergeben", todo.ID)
	}
}

func TestListTrenntOffeneUndErledigte(t *testing.T) {
	root := legacyFixture(t, "- [ ] Offen\n- [x] Erledigt\n- [ ] Auch offen\n")

	open, _, err := ListTodos(root)
	if err != nil {
		t.Fatalf("ListTodos: %v", err)
	}
	if len(open) != 2 || open[0].Text != "Offen" || open[1].Text != "Auch offen" {
		t.Errorf("offene Einträge: %+v", open)
	}

	done, _, err := ListDoneTodos(root)
	if err != nil {
		t.Fatalf("ListDoneTodos: %v", err)
	}
	if len(done) != 1 || done[0].Text != "Erledigt" {
		t.Errorf("erledigte Einträge: %+v", done)
	}
}

func boolPtr(value bool) *bool {
	return &value
}
