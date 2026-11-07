package inventory

import (
	"context"
	"fmt"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
	"github.com/kascada/k-playbook/installer/internal/yamllite"
)

// Status ist der maschinell lesbare Stand des Inventars.
//
// Er kommt ausschließlich aus dem YAML-Block zwischen den beiden
// `---`-Zeilen am Anfang des Dokuments. Der Markdown-Rumpf wird nie geparst —
// keine Tabelle wird rückwärts gelesen, keine Überschrift ausgewertet. Genau
// deshalb braucht es kein zweites Maschinenartefakt neben dem Dokument: es
// gäbe zwei Antworten auf dieselbe Frage, und sie könnten auseinanderlaufen.
//
// Diese Funktion ist die eine Statusquelle für den Sammler, für /k-docs, für
// das Subkommando und für die Oberfläche aus Task 043.
type Status struct {
	// Path ist der Ort des Inventars als Verweis: DocRef.
	Path    string `json:"path"`
	Present bool   `json:"present"`
	// Unavailable sagt, dass die Collection nicht erreichbar war: Ob das
	// Inventar da ist, weiß dann niemand — das ist ein eigener Zustand, kein
	// „fehlt“. Der Grund steht in Message.
	Unavailable bool   `json:"unavailable,omitempty"`
	Message     string `json:"message,omitempty"`
	// Revision ist die Revision des Dokuments in der Collection, bei
	// vorhandenem Bestand.
	Revision          int64  `json:"revision,omitempty"`
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
	// SourcesUnevaluable ist die Zahl der Quellen im Zustand „nicht
	// auswertbar": gelesen, aber als Ganzes nicht ausgewertet.
	SourcesUnevaluable int `json:"sourcesUnevaluable"`
	// Problem ist ein sichtbarer Befund zum Bestand: die Datei ist da, ihr
	// Frontmatter aber unvollständig oder defekt. Ein stilles Nullergebnis gibt
	// es nicht.
	Problem string `json:"problem,omitempty"`
}

// ReadStatus liest den Stand aus der Collection des Projekts. Fehlt das
// Dokument, ist das ein definierter Zustand und kein Fehler. Ist die
// Collection nicht erreichbar — kein Store, oder read scheitert —, ist das
// ein anderer Zustand (Unavailable) und wird nicht als „fehlt“ ausgegeben.
func ReadStatus(ctx context.Context, store Store) Status {
	if store == nil {
		return UnavailableStatus("kein Zugang zur Collection des Projekts")
	}
	read, err := store.Read(ctx, DocName)
	if err != nil {
		return UnavailableStatus(err.Error())
	}
	status := Status{Path: DocRef}
	switch read.Kind {
	case kephalaion.ReadKindNone:
		return status
	case kephalaion.ReadKindDirectory:
		status.Present = true
		status.Problem = "unter " + DocRef + " liegt ein Verzeichnis, kein Dokument"
		return status
	}
	status = StatusOf([]byte(*read.Content))
	status.Revision = read.Revision
	return status
}

// UnavailableStatus ist der Stand, wenn die Collection nicht erreichbar ist.
// Der Grund kommt vom Zugang (kephalaion_unavailable) oder vom Store.
func UnavailableStatus(reason string) Status {
	return Status{Path: DocRef, Unavailable: true, Message: reason}
}

// StatusOf ist der Stand eines vorliegenden Bestands: das Dokument ist da,
// und was sein Frontmatter sagt, steht im Ergebnis. Der Sammler benutzt es
// für den Vergleich, ReadStatus für die Auskunft.
func StatusOf(data []byte) Status {
	status := Status{Path: DocRef, Present: true}
	fillStatus(&status, data)
	return status
}

func fillStatus(status *Status, data []byte) {
	block, ok := FrontmatterBlock(data)
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

	// Die Zahlen stehen seit Task 120 unter generated.stats. Ein Bestand im
	// alten Kopf trägt sie im Block `inventory:`; er wird bis zum nächsten Lauf
	// weiter gelesen, der ihn im neuen Kopf schreibt.
	statsPath, stats := "generated.stats", root.Get("generated", "stats")
	if stats == nil {
		if legacy := root.Get("inventory"); legacy != nil {
			statsPath, stats = "inventory", legacy
		}
	}
	targets := map[string]*int{
		"sources-configured":  &status.SourcesConfigured,
		"sources-read":        &status.SourcesRead,
		"entries":             &status.Entries,
		"deviations":          &status.Deviations,
		"rejected":            &status.Rejected,
		"sources-excluded":    &status.SourcesExcluded,
		"sources-unevaluable": &status.SourcesUnevaluable,
	}

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
	for _, key := range statKeys {
		name, node := statsPath+"."+key, stats.Get(key)
		if node == nil || node.Str() == "" {
			missing = append(missing, name)
			continue
		}
		value, ok := node.Int()
		if !ok {
			missing = append(missing, name+" (ungültig)")
			continue
		}
		*targets[key] = value
	}
	if len(missing) > 0 {
		status.Problem = "unvollständiges Frontmatter, es fehlt: " + strings.Join(missing, ", ")
	}
}

// Body liefert den Markdown-Rumpf ohne den Frontmatter-Block — das, was eine
// Anzeige rendert. Ohne Frontmatter ist der Rumpf die ganze Datei. Der Rumpf
// wird dabei nicht gedeutet, nur abgetrennt: der Status kommt weiterhin
// ausschließlich aus dem Frontmatter. Zeilenenden kommen in jedem Fall als LF
// zurück, mit und ohne Frontmatter gleich.
func Body(data []byte) []byte {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return []byte(text)
	}
	for index := 1; index < len(lines); index++ {
		if strings.TrimSpace(lines[index]) == "---" {
			return []byte(strings.TrimLeft(strings.Join(lines[index+1:], "\n"), "\n"))
		}
	}
	return []byte(text)
}

// FrontmatterBlock schneidet den YAML-Block zwischen den beiden `---`-Zeilen am
// Dateianfang heraus. Ohne Frontmatter ist das zweite Ergebnis false. project/
// benutzt denselben Schnitt (Briefing, Erzeuger-Arbeitsverzeichnis), damit
// Frontmatter an genau einer Stelle erkannt wird.
func FrontmatterBlock(data []byte) (string, bool) {
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
