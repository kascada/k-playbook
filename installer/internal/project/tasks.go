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

// DoneDirName ist das Verzeichnis der erledigten Tasks unterhalb von
// TasksDirName. /k-run verschiebt eine Datei nach der Ausführung dorthin.
const DoneDirName = "done"

// Task ist eine Aufgabe, offen oder erledigt.
type Task struct {
	// Path ist der Name im Task-Verzeichnis. Offene Tasks liegen flach,
	// erledigte tragen das Präfix "done/".
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

// DoneDir ist das Verzeichnis der erledigten Tasks eines Projekts.
func DoneDir(projectDir string) string {
	return filepath.Join(TasksDir(projectDir), DoneDirName)
}

// ListTasks sammelt die offenen Tasks, nach ihrer Nummer sortiert.
//
// Erledigte liegen in done/ und bleiben draußen; Unterverzeichnisse fallen
// damit von selbst weg. Die README beschreibt das Verzeichnis und ist keine
// Aufgabe.
func ListTasks(projectDir string) ([]Task, error) {
	root := TasksDir(projectDir)
	return listTasksIn(root, root)
}

// ListDoneTasks sammelt die erledigten Tasks aus done/, die jüngste Nummer
// zuerst.
//
// Anders als bei den offenen zählt hier der letzte Stand: was zuletzt
// abgearbeitet wurde, steht oben.
func ListDoneTasks(projectDir string) ([]Task, error) {
	tasks, err := listTasksIn(TasksDir(projectDir), DoneDir(projectDir))
	if err != nil {
		return nil, err
	}
	sort.Slice(tasks, func(i int, j int) bool { return tasks[i].Path > tasks[j].Path })
	return tasks, nil
}

// listTasksIn liest die Markdown-Dateien unmittelbar in dir. Die Namen der
// gefundenen Tasks werden relativ zu root gebildet, damit erledigte ihr
// "done/" behalten und damit wieder angefragt werden können.
//
// Ein fehlendes Verzeichnis ist kein Fehler: die projekteigene Struktur wird
// erst angelegt, und "noch keine Tasks" ist dieselbe Auskunft wie ein leeres
// Verzeichnis.
func listTasksIn(root string, dir string) ([]Task, error) {
	if !isDir(dir) {
		return []Task{}, nil
	}

	entries, err := os.ReadDir(dir)
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
		tasks = append(tasks, readTaskFacts(root, filepath.Join(dir, name)))
	}

	// Die Nummer steht vorn, deshalb ordnet der Dateiname bereits richtig.
	sort.Slice(tasks, func(i int, j int) bool { return tasks[i].Path < tasks[j].Path })
	return tasks, nil
}

// ReadTask liefert eine einzelne Task als Rohtext. Der Name meint eine Datei
// unmittelbar im Task-Verzeichnis oder eine erledigte als "done/<datei>.md".
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
	return readTaskFacts(root, path), content, nil
}

// reviewLogHeading ist die Spur, die /k-task-refine in jeder geprüften Datei
// hinterlässt — auch dann, wenn nichts zu ändern war. /k-run erkennt daran
// dasselbe: ob ein Task jemals gegengelesen wurde.
const reviewLogHeading = "## Review-Log"

// readTaskFacts liest die Datei einmal und holt daraus alles, was die Liste
// über einen Task zeigt. Der Name ist der Weg ab root, in Schrägstrichen —
// derselbe Name, unter dem die Datei wieder angefragt wird.
func readTaskFacts(root string, path string) Task {
	name := filepath.Base(path)
	if relative, err := filepath.Rel(root, path); err == nil {
		name = filepath.ToSlash(relative)
	}

	task := Task{
		Path:  name,
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
// Verzeichnis oder in dessen done/ meinen, sonst wäre er ein Weg zu beliebigen
// Dateien des Rechners.
func taskFilePath(root string, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("kein Task angegeben")
	}

	// Genau eine Ebene tiefer, und genau dieses eine Verzeichnis: erledigte
	// Tasks sind der einzige Grund, den flachen Vergleich zu verlassen.
	dir := root
	file := name
	if rest, found := strings.CutPrefix(name, DoneDirName+"/"); found {
		dir = filepath.Join(root, DoneDirName)
		file = rest
	}

	if file != filepath.Base(file) || file == "." || file == ".." {
		return "", fmt.Errorf("nur Dateien in %s oder %s/%s", TasksDirName, TasksDirName, DoneDirName)
	}
	if !strings.EqualFold(filepath.Ext(file), ".md") {
		return "", fmt.Errorf("nur Markdown-Dateien")
	}
	return filepath.Join(dir, file), nil
}
