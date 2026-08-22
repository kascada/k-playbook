package project

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// TasksDirName ist das Verzeichnis der Tasks unterhalb von LocalDirName.
const TasksDirName = "tasks"

// Task ist eine offene Aufgabe.
type Task struct {
	// Path ist der Dateiname im Task-Verzeichnis. Tasks liegen flach, deshalb
	// ohne Verzeichnisanteil.
	Path string `json:"path"`
	// Title ist die erste Überschrift der Datei, ersatzweise der Dateiname.
	Title string `json:"title"`
	// Reviewed: die Datei ist durch /k-task-refine gegangen.
	Reviewed bool `json:"reviewed"`
	// ReviewedAt ist das Datum des jüngsten Logs, sofern es eines nennt.
	ReviewedAt string `json:"reviewedAt"`
}

// TasksDir ist das Task-Verzeichnis eines Projekts.
func TasksDir(projectDir string) string {
	return filepath.Join(LocalDir(projectDir), TasksDirName)
}

// ListTasks sammelt die offenen Tasks, nach ihrer Nummer sortiert.
//
// Erledigte liegen in done/ und bleiben draußen; Unterverzeichnisse fallen
// damit von selbst weg. Die README beschreibt das Verzeichnis und ist keine
// Aufgabe.
//
// Ein fehlendes Verzeichnis ist kein Fehler: die projekteigene Struktur wird
// erst angelegt, und "noch keine Tasks" ist dieselbe Auskunft wie ein leeres
// Verzeichnis.
func ListTasks(projectDir string) ([]Task, error) {
	root := TasksDir(projectDir)
	if !isDir(root) {
		return []Task{}, nil
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("Tasks lesen: %w", err)
	}

	tasks := []Task{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || strings.HasPrefix(name, ".") {
			continue
		}
		if !strings.EqualFold(filepath.Ext(name), ".md") || strings.EqualFold(name, "README.md") {
			continue
		}
		tasks = append(tasks, readTaskFacts(filepath.Join(root, name)))
	}

	// Die Nummer steht vorn, deshalb ordnet der Dateiname bereits richtig.
	sort.Slice(tasks, func(i int, j int) bool { return tasks[i].Path < tasks[j].Path })
	return tasks, nil
}

// ReadTask liefert eine einzelne Task als Rohtext.
func ReadTask(projectDir string, name string) (Task, []byte, error) {
	root := TasksDir(projectDir)
	path, err := taskFilePath(root, name)
	if err != nil {
		return Task{}, nil, err
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return Task{}, nil, fmt.Errorf("%s lesen: %w", name, err)
	}
	return readTaskFacts(path), content, nil
}

// reviewLogHeading ist die Spur, die /k-task-refine in jeder geprüften Datei
// hinterlässt — auch dann, wenn nichts zu ändern war. /k-run erkennt daran
// dasselbe: ob ein Task jemals gegengelesen wurde.
const reviewLogHeading = "## Review-Log"

// readTaskFacts liest die Datei einmal und holt daraus alles, was die Liste
// über einen Task zeigt.
func readTaskFacts(path string) Task {
	task := Task{
		Path:  filepath.Base(path),
		Title: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return task
	}

	// Codeblöcke bleiben außen vor: eine Vorlage im Text ist keine Überschrift
	// und kein Nachweis einer Prüfung.
	fenced := false
	titleFound := false
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "```") {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}

		if !titleFound && strings.HasPrefix(line, "# ") {
			task.Title = strings.TrimSpace(strings.TrimPrefix(line, "# "))
			titleFound = true
			continue
		}
		if strings.HasPrefix(line, reviewLogHeading) {
			task.Reviewed = true
			// Jede Runde hängt ihr Log an; das jüngste steht deshalb unten.
			if date := headingDate(line); date != "" {
				task.ReviewedAt = date
			}
		}
	}
	return task
}

// headingDate holt das Datum aus einer Überschrift der Form
// "## Review-Log (2026-08-12)". Fehlt die Klammer, bleibt es leer — das Log
// zählt trotzdem.
func headingDate(line string) string {
	open := strings.Index(line, "(")
	closing := strings.LastIndex(line, ")")
	if open < 0 || closing < open {
		return ""
	}
	return strings.TrimSpace(line[open+1 : closing])
}

// taskFilePath löst einen angefragten Namen im Task-Verzeichnis auf. Der Name
// kommt aus dem Browser: er darf nur eine Datei unmittelbar in diesem
// Verzeichnis meinen, sonst wäre er ein Weg zu beliebigen Dateien des Rechners.
func taskFilePath(root string, name string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("kein Task angegeben")
	}
	if name != filepath.Base(name) || name == "." || name == ".." {
		return "", fmt.Errorf("nur Dateien unmittelbar in %s", TasksDirName)
	}
	if !strings.EqualFold(filepath.Ext(name), ".md") {
		return "", fmt.Errorf("nur Markdown-Dateien")
	}
	return filepath.Join(root, name), nil
}
