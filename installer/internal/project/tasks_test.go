package project

import (
	"os"
	"path/filepath"
	"testing"
)

// tasksFixture legt ein Projekt mit Tasks an und gibt dessen Hauptverzeichnis
// zurück.
func tasksFixture(t *testing.T, files map[string]string) string {
	t.Helper()

	root := t.TempDir()
	for name, content := range files {
		path := filepath.Join(TasksDir(root), filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("%s anlegen: %v", name, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("%s schreiben: %v", name, err)
		}
	}
	return root
}

func TestListTasksNimmtNurOffeneNachNummer(t *testing.T) {
	root := tasksFixture(t, map[string]string{
		"002-zweiter.md":     "# Zweiter Task\n",
		"001-erster.md":      "Vorspann\n\n# Erster Task\n",
		"010-ohne-titel.md":  "nur Text\n",
		"README.md":          "# Tasks\n",
		"notiz.txt":          "keine Task\n",
		"done/000-fertig.md": "# Fertig\n",
	})

	tasks, err := ListTasks(root)
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(tasks) != 3 {
		t.Fatalf("erwartet 3 offene Tasks, bekommen %d: %+v", len(tasks), tasks)
	}

	// Die Nummer ordnet, nicht die Reihenfolge im Verzeichnis.
	if tasks[0].Path != "001-erster.md" || tasks[1].Path != "002-zweiter.md" {
		t.Errorf("Reihenfolge nach Nummer: %+v", tasks)
	}
	if tasks[0].Title != "Erster Task" {
		t.Errorf("Titel aus Überschrift: %q", tasks[0].Title)
	}
	// Ohne Überschrift bleibt nur der Dateiname.
	if tasks[2].Title != "010-ohne-titel" {
		t.Errorf("Rückfall auf den Dateinamen: %q", tasks[2].Title)
	}
}

// Ein Projekt, dessen Struktur noch nicht angelegt ist, hat keine Tasks — das
// ist kein Fehler, sondern die Zahl null.
func TestListTasksOhneVerzeichnis(t *testing.T) {
	tasks, err := ListTasks(t.TempDir())
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(tasks) != 0 {
		t.Fatalf("erwartet keine Tasks, bekommen %+v", tasks)
	}
}

// Der Review-Status hängt am Log, das /k-review-loop an jede geprüfte Datei
// anhängt — dieselbe Erkennung, die /k-run Step 1.2 benutzt.
func TestListTasksErkenntReviewLog(t *testing.T) {
	root := tasksFixture(t, map[string]string{
		"001-geprueft.md":   "# Geprüft\n\n---\n## Review-Log (2026-08-10)\n\nRunde 1\n\n---\n## Review-Log (2026-08-12)\n",
		"002-ohne-datum.md": "# Ohne Datum\n\n## Review-Log\n",
		"003-offen.md":      "# Offen\n\nText\n",
		// Eine Vorlage im Codeblock ist kein Nachweis einer Prüfung.
		"004-nur-zitiert.md": "# Nur zitiert\n\n```markdown\n## Review-Log (<date>)\n```\n",
	})

	tasks, err := ListTasks(root)
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}

	if !tasks[0].Reviewed || tasks[0].ReviewedAt != "2026-08-12" {
		t.Errorf("jüngstes Log erwartet: %+v", tasks[0])
	}
	if !tasks[1].Reviewed || tasks[1].ReviewedAt != "" {
		t.Errorf("Log ohne Datum zählt trotzdem: %+v", tasks[1])
	}
	if tasks[2].Reviewed {
		t.Errorf("ohne Log nicht gereviewt: %+v", tasks[2])
	}
	if tasks[3].Reviewed {
		t.Errorf("Codeblock ist kein Nachweis: %+v", tasks[3])
	}
}

func TestReadTaskLiefertInhalt(t *testing.T) {
	root := tasksFixture(t, map[string]string{"001-erster.md": "# Erster Task\n\nText\n"})

	task, content, err := ReadTask(root, "001-erster.md")
	if err != nil {
		t.Fatalf("ReadTask: %v", err)
	}
	if task.Title != "Erster Task" {
		t.Errorf("Titel: %q", task.Title)
	}
	if len(content) == 0 {
		t.Error("kein Inhalt gelesen")
	}
}

// Der Pfad kommt aus dem Browser: alles, was aus dem Task-Verzeichnis
// herausführt oder keine Markdown-Datei meint, muss abgewiesen werden.
func TestReadTaskWeistFremdePfadeAb(t *testing.T) {
	root := tasksFixture(t, map[string]string{"001-erster.md": "# Erster Task\n"})

	for _, name := range []string{"", "../../etc/passwd.md", "done/000-fertig.md", "/etc/passwd.md", "001-erster.txt"} {
		if _, _, err := ReadTask(root, name); err == nil {
			t.Errorf("%q wurde angenommen", name)
		}
	}
}
