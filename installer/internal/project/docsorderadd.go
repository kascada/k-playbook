package project

import (
	"context"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
)

// Einen Doku-Auftrag einreihen (Task 179): k-playbook docs order add. Die
// Regeln stehen in k-playbook:/instructions/on-demand/doku-auftraege.md —
// „Kein Doppel“, „Wann nicht eingereiht wird“ — und gelten hier für den
// rohen Auftrag nach einer Codeänderung. Ein Auftrag mit pfade: . ist der
// volle Lauf, den /k-docs einreiht: Für ihn gelten „Ein Auftrag je Sitzung“,
// „Kein Doppel“ und „Noch keine erzeugte Doku“ nicht; ein offener roher
// Auftrag mit pfade: . deckt ihn.

// Die Ergebnisse von Add.
const (
	// DocsOrderCreated: ein neuer Auftrag steht in der Queue.
	DocsOrderCreated = "created"
	// DocsOrderExtended: der offene rohe Auftrag derselben Sitzung wurde
	// ergänzt (Pfade vereinigt, Anlass als Zeile im Rumpf).
	DocsOrderExtended = "extended"
	// DocsOrderCovered: ein offener Auftrag deckt die Änderung schon; nichts
	// geschrieben, der Auftrag zählt als eingereiht.
	DocsOrderCovered = "covered"
	// DocsOrderSkipped: nichts eingereiht, Reason sagt warum.
	DocsOrderSkipped = "skipped"
)

// Die Gründe, aus denen nichts eingereiht wird.
const (
	// DocsReasonNoGeneratedDocs: unter keph:/docs/generated/<erzeuger>/ liegt
	// noch kein Dokument; es gibt nichts nachzuziehen.
	DocsReasonNoGeneratedDocs = "no_generated_docs"
	// DocsReasonKephalaionUnavailable: Ohne bereite Kephalaion-Anbindung wird
	// nicht eingereiht, auch nicht auf die Platte.
	DocsReasonKephalaionUnavailable = "kephalaion_unavailable"
	// DocsReasonNothingToAdd: ein zerlegter Auftrag trägt schon alles.
	DocsReasonNothingToAdd = "nothing_to_add"
)

// DocsOrderAddInput sind die Eingaben von Add. Sitzung ist die Kennung der
// Sitzung aus dem Briefing; Title ist leer, wenn der Anlass der Titel sein
// soll.
type DocsOrderAddInput struct {
	Producer string
	Pfade    []string
	Anlass   string
	Title    string
	Sitzung  string
}

// DocsOrderAddResult ist die Antwort von Add. Ref und Revision sind die des
// Auftrags, der eingereiht wurde oder deckt; bei skipped fehlen sie.
// Corrected nennt die berichtigten Eingabepfade (docsRepoPrefixFix), Missing
// die Pfade, die es auch danach unter project.repoRoot nicht gibt; sie
// ändern den Status nicht (Task 193).
type DocsOrderAddResult struct {
	Producer  string               `json:"producer"`
	Status    string               `json:"status"`
	Ref       string               `json:"ref,omitempty"`
	Revision  int64                `json:"revision,omitempty"`
	Pfade     []string             `json:"pfade,omitempty"`
	Corrected []DocsPathCorrection `json:"corrected,omitempty"`
	Missing   []string             `json:"missing,omitempty"`
	Reason    string               `json:"reason,omitempty"`
	Message   string               `json:"message,omitempty"`
}

// DocsOrderAddUnavailable ist die Antwort, wenn die Kephalaion-Anbindung nicht
// bereit ist: Es wird nicht eingereiht, das ist kein Fehler des Aufrufs.
func DocsOrderAddUnavailable(producer string, err error) DocsOrderAddResult {
	return DocsOrderAddResult{Producer: producer, Status: DocsOrderSkipped, Reason: DocsReasonKephalaionUnavailable, Message: err.Error()}
}

// docsOrderAttempts begrenzt, wie oft ein Ergänzen nach stale_revision neu
// liest.
const docsOrderAttempts = 3

// yamlQuoted setzt einen einzeiligen Wert in doppelte Anführungszeichen, ein
// \ wird \\, ein " wird \".
func yamlQuoted(value string) string {
	value = strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ", "\t", " ").Replace(value)
	value = strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(strings.TrimSpace(value))
	return `"` + value + `"`
}

var yamlHeaderPlain = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// yamlHeaderValue lässt einen schlichten Wert (Buchstaben, Ziffern, ., _, -) wie er
// ist, damit das Format bestehender Aufträge gleich bleibt, und setzt jeden
// anderen in Anführungszeichen.
func yamlHeaderValue(value string) string {
	if yamlHeaderPlain.MatchString(value) {
		return value
	}
	return yamlQuoted(value)
}

// oneLine macht aus einem Text eine Zeile.
func oneLine(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

// Add reiht einen rohen Auftrag ein oder ergänzt den vorhandenen derselben
// Sitzung.
func (o *DocsOrders) Add(ctx context.Context, in DocsOrderAddInput) (DocsOrderAddResult, error) {
	if err := ValidDocsOrderProducer(in.Producer); err != nil {
		return DocsOrderAddResult{}, err
	}
	anlass := oneLine(in.Anlass)
	if anlass == "" {
		return DocsOrderAddResult{}, docRefErrorf(DocCodeInvalidInput, "kein Anlass angegeben")
	}
	sitzung := oneLine(in.Sitzung)
	if sitzung == "" {
		return DocsOrderAddResult{}, docRefErrorf(DocCodeInvalidInput, "keine Sitzung angegeben — die Kennung aus dem Briefing (Zeile „Sitzung:“)")
	}
	if len(in.Pfade) == 0 {
		return DocsOrderAddResult{}, docRefErrorf(DocCodeInvalidInput, "kein Pfad angegeben")
	}
	pfade, err := cleanDocsOrderPaths(in.Pfade)
	if err != nil {
		return DocsOrderAddResult{}, err
	}
	pfade, corrected := o.correctPaths(pfade)
	result := DocsOrderAddResult{Producer: in.Producer, Pfade: pfade, Corrected: corrected, Missing: o.missingPaths(pfade)}
	full := len(pfade) == 1 && pfade[0] == "."

	if !full {
		count, err := o.generatedDocCount(ctx, in.Producer)
		if err != nil {
			return DocsOrderAddResult{}, err
		}
		if count == 0 {
			result.Status, result.Reason = DocsOrderSkipped, DocsReasonNoGeneratedDocs
			result.Message = fmt.Sprintf("unter keph:/%s/ liegt noch kein Dokument — es gibt nichts nachzuziehen", docsOrderGeneratedDir(in.Producer))
			return result, nil
		}
	}

	title := oneLine(in.Title)
	if title == "" {
		title = anlass
	}
	date := now().Format("2006-01-02")

	for attempt := 0; attempt < docsOrderAttempts; attempt++ {
		orders, _, err := o.load(ctx)
		if err != nil {
			return DocsOrderAddResult{}, err
		}
		existing, found := findRawOrder(orders, in.Producer, sitzung, full)
		if !found {
			return o.createRaw(ctx, result, title, anlass, sitzung, date)
		}
		if full || in.Producer == DocsOrderTools || docsPathsCovered(existing.Pfade, pfade) {
			result.Status, result.Ref, result.Revision = DocsOrderCovered, existing.Ref, existing.Revision
			result.Pfade = existing.Pfade
			return result, nil
		}
		merged := append([]string{}, existing.Pfade...)
		for _, p := range pfade {
			if !containsString(merged, p) {
				merged = append(merged, p)
			}
		}
		if containsString(merged, ".") {
			merged = []string{"."}
		}
		name := strings.TrimPrefix(existing.Ref, DocSchemeKeph+":/")
		read, err := o.docs.store.Read(ctx, name)
		if err != nil {
			return DocsOrderAddResult{}, kephError(err)
		}
		if read.Kind != kephalaion.ReadKindDocument || read.Content == nil {
			continue
		}
		content := spliceDocsOrderPaths(*read.Content, merged)
		content = appendDocsOrderLine(content, fmt.Sprintf("%s — %s", date, anlass))
		doc, err := o.docs.store.Write(ctx, name, content, read.Revision)
		if err == nil {
			result.Status, result.Ref, result.Revision, result.Pfade = DocsOrderExtended, existing.Ref, doc.Revision, merged
			return result, nil
		}
		if kephalaion.ErrorCode(err) != kephalaion.CodeStaleRevision {
			return DocsOrderAddResult{}, kephError(err)
		}
	}
	return DocsOrderAddResult{}, docRefErrorf(DocCodeStaleRevision, "der Auftrag wurde %d Mal nacheinander geändert, während er ergänzt wurde — noch einmal versuchen", docsOrderAttempts)
}

// findRawOrder sucht den offenen rohen Auftrag, der einen neuen deckt oder
// ergänzt wird: für den vollen Lauf irgendeinen mit pfade: ., sonst einen der
// Sitzung für denselben Erzeuger. Ein zerlegter zählt nie.
func findRawOrder(orders []DocsOrder, producer, sitzung string, full bool) (DocsOrder, bool) {
	for _, order := range orders {
		if order.Producer != producer || order.Kind != DocsOrderKindRaw {
			continue
		}
		if full {
			if len(order.Pfade) == 1 && order.Pfade[0] == "." {
				return order, true
			}
			continue
		}
		if order.Sitzung == sitzung {
			return order, true
		}
	}
	return DocsOrder{}, false
}

// createRaw legt einen neuen rohen Auftrag an.
func (o *DocsOrders) createRaw(ctx context.Context, result DocsOrderAddResult, title, anlass, sitzung, date string) (DocsOrderAddResult, error) {
	short := result.Producer + " " + strings.Join(firstWords(anlass, 4), " ")
	name, err := NormalizeDocName(short)
	if err != nil {
		name, err = NormalizeDocName(result.Producer)
		if err != nil {
			return DocsOrderAddResult{}, err
		}
	}
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "title: %s\n", yamlQuoted(title))
	fmt.Fprintf(&b, "erzeuger: %s\n", result.Producer)
	b.WriteString(renderDocsOrderPaths(result.Pfade))
	fmt.Fprintf(&b, "anlass: %s\n", yamlQuoted(anlass))
	fmt.Fprintf(&b, "sitzung: %s\n", yamlHeaderValue(sitzung))
	fmt.Fprintf(&b, "entstanden: %s\n", date)
	b.WriteString("---\n")
	kind, _ := o.docs.anyKind("docs")
	created, doc, err := o.docs.createNumbered(ctx, kind, kind.dirFor(o.docs.user), name, b.String(), kephError)
	if err != nil {
		return DocsOrderAddResult{}, err
	}
	result.Status, result.Ref, result.Revision = DocsOrderCreated, created.Ref, doc.Revision
	return result, nil
}

// firstWords nimmt höchstens n Wörter.
func firstWords(text string, n int) []string {
	words := strings.Fields(text)
	if len(words) > n {
		words = words[:n]
	}
	return words
}

// renderDocsOrderPaths schreibt den Block pfade: im Kopf, ein Pfad je Zeile.
func renderDocsOrderPaths(paths []string) string {
	var b strings.Builder
	b.WriteString("pfade:\n")
	for _, p := range paths {
		fmt.Fprintf(&b, "  - %s\n", docsYAMLScalar(p))
	}
	return b.String()
}

// frontmatterBounds nennt die Zeilen von --- bis --- (Indizes der beiden
// Trennzeilen); ok ist false ohne Frontmatter.
func frontmatterBounds(lines []string) (int, int, bool) {
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return 0, 0, false
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return 0, i, true
		}
	}
	return 0, 0, false
}

// spliceDocsOrderPaths ersetzt den Block pfade: im Kopf durch paths; fehlt er,
// kommt er vor die schließende Trennzeile. Alles andere bleibt Byte für Byte.
func spliceDocsOrderPaths(content string, paths []string) string {
	lines := strings.Split(content, "\n")
	_, end, ok := frontmatterBounds(lines)
	if !ok {
		return content
	}
	block := strings.Split(strings.TrimRight(renderDocsOrderPaths(paths), "\n"), "\n")
	start := -1
	for i := 1; i < end; i++ {
		if strings.HasPrefix(lines[i], "pfade:") {
			start = i
			break
		}
	}
	if start < 0 {
		out := append([]string{}, lines[:end]...)
		out = append(out, block...)
		return strings.Join(append(out, lines[end:]...), "\n")
	}
	stop := start + 1
	if strings.TrimSpace(strings.TrimPrefix(lines[start], "pfade:")) == "" {
		for stop < end && (strings.HasPrefix(lines[stop], " ") || strings.HasPrefix(lines[stop], "\t") || strings.HasPrefix(lines[stop], "- ")) {
			stop++
		}
	}
	out := append([]string{}, lines[:start]...)
	out = append(out, block...)
	return strings.Join(append(out, lines[stop:]...), "\n")
}

// appendDocsOrderLine hängt eine Zeile an den Rumpf: nach einer Leerzeile,
// wenn der Rumpf noch leer ist, sonst als nächste Zeile.
func appendDocsOrderLine(content, line string) string {
	lines := strings.Split(content, "\n")
	_, end, ok := frontmatterBounds(lines)
	bodyEmpty := true
	if ok {
		for _, l := range lines[end+1:] {
			if strings.TrimSpace(l) != "" {
				bodyEmpty = false
				break
			}
		}
	}
	text := strings.TrimRight(content, "\n") + "\n"
	if bodyEmpty && ok {
		text += "\n"
	}
	return text + line + "\n"
}

// generatedDocCount zählt die Dokumente unter keph:/docs/generated/<erzeuger>/
// außer README.md und index.md.
func (o *DocsOrders) generatedDocCount(ctx context.Context, producer string) (int, error) {
	dir := docsOrderGeneratedDir(producer)
	entries, err := o.docs.store.List(ctx, kephalaion.ListRequest{Path: dir, Recursive: true})
	if err != nil {
		return 0, kephError(err)
	}
	count := 0
	for _, entry := range entries {
		if entry.Kind != "" && entry.Kind != "document" {
			continue
		}
		if !strings.HasSuffix(entry.Name, ".md") {
			continue
		}
		base := path.Base(entry.Name)
		if base == docsReadmeTemplateName || base == "index.md" {
			continue
		}
		count++
	}
	return count, nil
}

// errIsStale sagt, ob ein Fehler von Kephalaion stale_revision meldet.
func errIsStale(err error) bool {
	var coded *DocRefError
	if errors.As(err, &coded) {
		return coded.Code == DocCodeStaleRevision
	}
	return kephalaion.ErrorCode(err) == kephalaion.CodeStaleRevision
}

// DocsOrderOpenError ist der Fehler von OpenDocsOrders, wenn die Anbindung
// nicht bereit ist (DocCodeKephalaionUnavailable). Tests der Aufrufer setzen
// ihn ein, ohne den Node zu fragen.
func DocsOrderOpenError(message string) error {
	return docRefErrorf(DocCodeKephalaionUnavailable, "%s", message)
}
