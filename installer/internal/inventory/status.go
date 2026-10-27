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
	status.SourcesConfigured, _ = root.Get("inventory", "sources-configured").Int()
	status.SourcesRead, _ = root.Get("inventory", "sources-read").Int()
	status.Entries, _ = root.Get("inventory", "entries").Int()
	status.Deviations, _ = root.Get("inventory", "deviations").Int()
	status.Rejected, _ = root.Get("inventory", "rejected").Int()
	status.SourcesExcluded, _ = root.Get("inventory", "sources-excluded").Int()
	status.SourcesUnevaluable, _ = root.Get("inventory", "sources-unevaluable").Int()

	var missing []string
	for _, field := range []struct{ name, value string }{
		{"type", root.Get("type").Str()},
		{"title", root.Get("title").Str()},
		{"description", root.Get("description").Str()},
		{"status", root.Get("status").Str()},
		{"generated.by", status.GeneratedBy},
		{"generated.at", status.GeneratedAt},
	} {
		if strings.TrimSpace(field.value) == "" {
			missing = append(missing, field.name)
		}
	}
	if tags := root.Get("tags"); tags == nil || len(tags.List()) == 0 {
		missing = append(missing, "tags")
	}
	for _, field := range []struct {
		name string
		node *yamllite.Node
	}{
		{"inventory.sources-configured", root.Get("inventory", "sources-configured")},
		{"inventory.sources-read", root.Get("inventory", "sources-read")},
		{"inventory.entries", root.Get("inventory", "entries")},
		{"inventory.deviations", root.Get("inventory", "deviations")},
		{"inventory.rejected", root.Get("inventory", "rejected")},
		{"inventory.sources-excluded", root.Get("inventory", "sources-excluded")},
		{"inventory.sources-unevaluable", root.Get("inventory", "sources-unevaluable")},
	} {
		if field.node == nil || field.node.Str() == "" {
			missing = append(missing, field.name)
			continue
		}
		if _, ok := field.node.Int(); !ok {
			missing = append(missing, field.name+" (ungültig)")
		}
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
// Dateianfang heraus. Ohne Frontmatter ist das zweite Ergebnis false. Das
// Wissenstor in project/ benutzt denselben Schnitt für title und description,
// damit Frontmatter an genau einer Stelle erkannt wird.
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
