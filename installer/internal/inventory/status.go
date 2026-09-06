package inventory

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/yamllite"
)

// Status ist der maschinell lesbare Stand der Inventardatei.
//
// Er kommt ausschließlich aus dem YAML-Block zwischen den beiden
// `---`-Zeilen am Dateianfang. Der Markdown-Rumpf wird nie geparst — keine
// Tabelle wird rückwärts gelesen, keine Überschrift ausgewertet. Genau deshalb
// braucht es kein zweites Maschinenartefakt neben der Datei: es gäbe zwei
// Antworten auf dieselbe Frage, und sie könnten auseinanderlaufen.
//
// Diese Funktion ist die eine Statusquelle für den Sammler, für /k-docs, für
// das Subkommando und für die Oberfläche aus Task 043.
type Status struct {
	Path              string `json:"path"`
	Present           bool   `json:"present"`
	GeneratedBy       string `json:"generatedBy,omitempty"`
	GeneratedAt       string `json:"generatedAt,omitempty"`
	SourcesConfigured int    `json:"sourcesConfigured"`
	SourcesRead       int    `json:"sourcesRead"`
	Entries           int    `json:"entries"`
	Deviations        int    `json:"deviations"`
	Rejected          int    `json:"rejected"`
	// SourcesExcluded ist die Zahl der Quellen, die eine Ausschlussregel
	// übergangen hat — die Installation und, wenn konfiguriert, die Muster aus
	// `exclude:`.
	SourcesExcluded int `json:"sourcesExcluded"`
	// Problem ist ein sichtbarer Befund zum Bestand: die Datei ist da, ihr
	// Frontmatter aber unvollständig oder defekt. Ein stilles Nullergebnis gibt
	// es nicht.
	Problem string `json:"problem,omitempty"`
}

// ReadStatus liest den Stand. Fehlt die Datei, ist das ein definierter Zustand
// und kein Fehler.
func ReadStatus(path string) Status {
	status := Status{Path: path}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return status
	}
	if err != nil {
		status.Present = true
		status.Problem = "nicht lesbar: " + err.Error()
		return status
	}
	status.Present = true
	fillStatus(&status, data)
	return status
}

func fillStatus(status *Status, data []byte) {
	block, ok := frontmatterBlock(data)
	if !ok {
		status.Problem = "kein Frontmatter am Dateianfang"
		return
	}
	root, err := yamllite.Parse([]byte(block))
	if err != nil {
		status.Problem = fmt.Sprintf("Frontmatter nicht lesbar: %v", err)
		return
	}
	status.GeneratedBy = root.Get("generated", "by").Str()
	status.GeneratedAt = root.Get("generated", "at").Str()
	status.SourcesConfigured, _ = root.Get("inventory", "sources-configured").Int()
	status.SourcesRead, _ = root.Get("inventory", "sources-read").Int()
	status.Entries, _ = root.Get("inventory", "entries").Int()
	status.Deviations, _ = root.Get("inventory", "deviations").Int()
	status.Rejected, _ = root.Get("inventory", "rejected").Int()
	status.SourcesExcluded, _ = root.Get("inventory", "sources-excluded").Int()

	var missing []string
	for _, field := range []struct{ name, value string }{
		{"title", root.Get("title").Str()},
		{"description", root.Get("description").Str()},
		{"generated.by", status.GeneratedBy},
		{"generated.at", status.GeneratedAt},
	} {
		if strings.TrimSpace(field.value) == "" {
			missing = append(missing, field.name)
		}
	}
	if len(missing) > 0 {
		status.Problem = "unvollständiges Frontmatter, es fehlt: " + strings.Join(missing, ", ")
	}
}

// Body liefert den Markdown-Rumpf ohne den Frontmatter-Block — das, was eine
// Anzeige rendert. Ohne Frontmatter ist der Rumpf die ganze Datei. Der Rumpf
// wird dabei nicht gedeutet, nur abgetrennt: der Status kommt weiterhin
// ausschließlich aus dem Frontmatter.
func Body(data []byte) []byte {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return data
	}
	for index := 1; index < len(lines); index++ {
		if strings.TrimSpace(lines[index]) == "---" {
			return []byte(strings.TrimLeft(strings.Join(lines[index+1:], "\n"), "\n"))
		}
	}
	return data
}

// frontmatterBlock schneidet den YAML-Block zwischen den beiden `---`-Zeilen am
// Dateianfang heraus.
func frontmatterBlock(data []byte) (string, bool) {
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", false
	}
	for index := 1; index < len(lines); index++ {
		if strings.TrimSpace(lines[index]) == "---" {
			return strings.Join(lines[1:index], "\n"), true
		}
	}
	return "", false
}
