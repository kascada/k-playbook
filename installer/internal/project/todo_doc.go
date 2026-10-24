package project

import (
	"regexp"
	"strings"
	"time"

	"github.com/kascada/k-playbook/installer/internal/yamllite"
)

// Das Dokumentformat eines Todos in der Collection (Task 097, „Kontext“):
// Frontmatter mit origin (optional), done (Datum, nur in done/),
// doneMigrated (optional, true) und created (nur bei übernommenen Einträgen);
// der Text des Todos ist der Rumpf. Gelesen und geschrieben wird es allein
// hier.
//
// Für created gilt: Steht created: im Frontmatter, gilt es, auch wenn es leer
// ist — übernommene Einträge ohne Datum bleiben leer, ein Zeitstempel wird
// nicht erfunden. Sonst gilt der Stempel created von Kephalaion.
//
// Ob ein Todo offen oder erledigt ist, sagt allein das Verzeichnis; done: im
// Frontmatter trägt nur das Datum, weil rename keines festhält und updated
// bei jeder Textänderung mitwandert.

// TodoKindName ist die Sorte der Todos in der Sorten-Tabelle DocKinds.
const TodoKindName = "todos"

// todoDoc ist ein Todo-Dokument, zerlegt.
type todoDoc struct {
	// Text ist der Rumpf ohne Leerraum an den Rändern.
	Text   string
	Origin string
	// Done ist das Feld done: — ein Datum oder leer.
	Done         string
	DoneMigrated bool
	// Created ist das Feld created:, HasCreated sagt, ob es dasteht — auch
	// leer.
	Created    string
	HasCreated bool
	// extra sind die Zeilen anderer Felder des Frontmatters, unverändert und
	// in ihrer Reihenfolge; sie gehen beim Schreiben nicht verloren.
	extra []string
}

// todoFieldOrder sind die Felder des Formats in der Reihenfolge, in der sie
// geschrieben werden.
var todoFieldOrder = []string{"origin", "created", "done", "doneMigrated"}

// todoKeyLine erkennt eine Zeile, die ein Feld der obersten Ebene beginnt.
var todoKeyLine = regexp.MustCompile(`^([A-Za-z0-9_][A-Za-z0-9_.-]*)\s*:`)

// parseTodoDoc zerlegt den Inhalt eines Todo-Dokuments. Ein Kopf, den
// yamllite nicht versteht, ist kein Fehler: Seine Zeilen bleiben als fremde
// Felder erhalten, die Felder des Formats gelten als leer.
func parseTodoDoc(content string) todoDoc {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return todoDoc{Text: strings.TrimSpace(content)}
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		// Kein schließender Strich: kein Frontmatter, alles ist Text.
		return todoDoc{Text: strings.TrimSpace(content)}
	}
	doc := todoDoc{Text: strings.TrimSpace(strings.Join(lines[end+1:], "\n"))}
	block := lines[1:end]

	root, err := yamllite.Parse([]byte(strings.Join(block, "\n")))
	if err != nil || root == nil || root.Kind != yamllite.Mapping {
		for _, line := range block {
			if strings.TrimSpace(line) != "" {
				doc.extra = append(doc.extra, line)
			}
		}
		return doc
	}
	doc.Origin = strings.TrimSpace(root.Get("origin").Str())
	doc.Done = strings.TrimSpace(root.Get("done").Str())
	doc.DoneMigrated = strings.EqualFold(strings.TrimSpace(root.Get("doneMigrated").Str()), "true")
	if node := root.Get("created"); node != nil {
		doc.HasCreated = true
		doc.Created = strings.TrimSpace(node.Str())
	}

	// Fremde Felder samt ihrer eingerückten Folgezeilen behalten.
	known := map[string]bool{}
	for _, key := range todoFieldOrder {
		known[key] = true
	}
	keep := false
	for _, line := range block {
		if m := todoKeyLine.FindStringSubmatch(line); m != nil {
			keep = !known[m[1]]
		} else if strings.TrimSpace(line) == "" {
			continue
		}
		// Folgezeilen (eingerückt, Listen, Kommentare) gehören zum Feld davor.
		if keep {
			doc.extra = append(doc.extra, line)
		}
	}
	return doc
}

// composeTodoDoc baut den Inhalt eines Todo-Dokuments. Ein Frontmatter steht
// nur da, wenn es ein Feld trägt oder der Text selbst mit einem Strich ---
// beginnt (sonst läse ihn parseTodoDoc als Kopf). Zeilenenden werden LF, die
// Datei endet mit genau einem Zeilenumbruch.
func composeTodoDoc(doc todoDoc) string {
	text := strings.TrimSpace(strings.ReplaceAll(doc.Text, "\r\n", "\n"))
	var head strings.Builder
	if doc.Origin != "" {
		head.WriteString("origin: " + yamlScalar(doc.Origin) + "\n")
	}
	if doc.HasCreated {
		head.WriteString("created: " + yamlScalar(doc.Created) + "\n")
	}
	if doc.Done != "" {
		head.WriteString("done: " + yamlScalar(doc.Done) + "\n")
	}
	if doc.DoneMigrated {
		head.WriteString("doneMigrated: true\n")
	}
	for _, line := range doc.extra {
		head.WriteString(line + "\n")
	}
	firstLine, _, _ := strings.Cut(text, "\n")
	if head.Len() == 0 && strings.TrimSpace(firstLine) != "---" {
		return text + "\n"
	}
	return "---\n" + head.String() + "---\n\n" + text + "\n"
}

// stampDate macht aus einem Stempel von Kephalaion (RFC 3339, UTC) das Datum
// YYYY-MM-DD in der Ortszeit — dieselbe Form wie today(). Lässt er sich
// nicht lesen, gelten seine ersten zehn Zeichen, sonst leer.
func stampDate(at string) string {
	if t, err := time.Parse(time.RFC3339Nano, at); err == nil {
		return t.Local().Format("2006-01-02")
	}
	if len(at) >= 10 {
		return at[:10]
	}
	return ""
}
