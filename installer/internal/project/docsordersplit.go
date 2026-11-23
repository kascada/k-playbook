package project

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
)

// Zerlegte Doku-Aufträge schreiben (Task 179): k-playbook docs orders split.
// Die Sitzung hat die rohen Aufträge zerlegt und der Nutzer neu und entfällt
// bestätigt; das Programm legt je Dokument einen zerlegten Auftrag an oder
// ergänzt den offenen, schließt danach die aufgegangenen rohen Aufträge und
// die abgelehnten neu/entfaellt. Der Ablauf ist „In die Queue schreiben“ in
// Schritt 4 von k-playbook:/commands/k-docs-code.md; die Köpfe stehen in
// k-playbook:/instructions/on-demand/doku-auftraege.md.

// Die Ergebnisse je Dokument von Split.
const (
	// DocsSplitFailed: der Auftrag konnte nicht geschrieben werden; Code und
	// Message nennen den Grund, die rohen Aufträge bleiben offen.
	DocsSplitFailed = "failed"
)

// Die Ergebnisse je roher Auftrag von Split.
const (
	// DocsRawClosed: der rohe Auftrag liegt jetzt unter queue/docs/done/.
	DocsRawClosed = "closed"
	// DocsRawGone: ein anderer Lauf hat ihn schon abgeschlossen (wrong_dir
	// oder not_found); nicht wiederholt.
	DocsRawGone = "gone"
	// DocsRawKept: bleibt offen, weil ein zerlegter Auftrag nicht
	// geschrieben werden konnte.
	DocsRawKept = "kept"
)

// DocsSplitDocument ist eine bestätigte Zeile der Liste: ein Dokument mit
// Aktion, den Pfaden aus den aufgegangenen Aufträgen, die es betreffen (im
// vollen Lauf nur .), und deren Anlässen. Abgelehnt heißt: Der Nutzer hat ein
// neu oder entfaellt abgelehnt — der Auftrag wird geschrieben und gleich
// geschlossen.
type DocsSplitDocument struct {
	Dokument   string          `json:"dokument"`
	Aktion     string          `json:"aktion"`
	Pfade      []string        `json:"pfade"`
	Anlaesse   []DocsOrderLine `json:"anlaesse"`
	Anlass     string          `json:"anlass,omitempty"`
	Entstanden string          `json:"entstanden,omitempty"`
	Abgelehnt  bool            `json:"abgelehnt,omitempty"`
}

// DocsSplitInput ist die Eingabe von Split: die Kennung der Sitzung des
// Verarbeiters, die bestätigten Zeilen und die aufgegangenen rohen Aufträge
// als keph:-Verweise.
type DocsSplitInput struct {
	Sitzung   string              `json:"sitzung"`
	Documents []DocsSplitDocument `json:"documents"`
	Raw       []string            `json:"raw"`
}

// DocsSplitDocResult ist das Ergebnis je Dokument. Closed ist wahr bei einem
// abgelehnten Auftrag, der gleich geschlossen wurde; DoneRef ist dann sein Ort
// unter done/.
type DocsSplitDocResult struct {
	Dokument string `json:"dokument"`
	Aktion   string `json:"aktion"`
	Status   string `json:"status"`
	Ref      string `json:"ref,omitempty"`
	Revision int64  `json:"revision,omitempty"`
	Reason   string `json:"reason,omitempty"`
	Code     string `json:"code,omitempty"`
	Message  string `json:"message,omitempty"`
	Closed   bool   `json:"closed,omitempty"`
	DoneRef  string `json:"doneRef,omitempty"`
}

// DocsSplitRawResult ist das Ergebnis je rohem Auftrag.
type DocsSplitRawResult struct {
	Ref     string `json:"ref"`
	Status  string `json:"status"`
	DoneRef string `json:"doneRef,omitempty"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// DocsSplitResult ist die Antwort von Split. ClosedRaw nennt die geschlossenen
// rohen Aufträge, wie sie vorher hießen.
type DocsSplitResult struct {
	Documents []DocsSplitDocResult `json:"documents"`
	Raw       []DocsSplitRawResult `json:"raw"`
	ClosedRaw []string             `json:"closedRaw"`
}

var docsSplitNamePattern = regexp.MustCompile(`^[a-z0-9._-]+\.md$`)

// normalizeDocsAction nimmt auch die Schreibweise mit Umlaut.
func normalizeDocsAction(raw string) (string, error) {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case DocsActionNew:
		return DocsActionNew, nil
	case DocsActionRewrite:
		return DocsActionRewrite, nil
	case DocsActionObsolete, "entfällt":
		return DocsActionObsolete, nil
	}
	return "", docRefErrorf(DocCodeInvalidInput, "aktion %q — erlaubt sind %s, %s, %s", raw, DocsActionNew, DocsActionRewrite, DocsActionObsolete)
}

// docsSplitPrepared ist eine geprüfte Zeile.
type docsSplitPrepared struct {
	input    DocsSplitDocument
	aktion   string
	pfade    []string
	lines    []DocsOrderLine
	anlass   string
	entsteht string
}

// Split schreibt die zerlegten Aufträge und schließt, was darin aufgegangen
// ist. Alle Eingaben werden geprüft, bevor etwas geschrieben wird.
func (o *DocsOrders) Split(ctx context.Context, in DocsSplitInput) (DocsSplitResult, error) {
	sitzung := oneLine(in.Sitzung)
	if sitzung == "" {
		return DocsSplitResult{}, docRefErrorf(DocCodeInvalidInput, "keine Sitzung angegeben — die Kennung des Verarbeiters aus dem Briefing")
	}
	date := now().Format("2006-01-02")
	prepared := make([]docsSplitPrepared, 0, len(in.Documents))
	seen := map[string]bool{}
	for _, doc := range in.Documents {
		name := strings.TrimSpace(doc.Dokument)
		if !docsSplitNamePattern.MatchString(name) || name == docsReadmeTemplateName || name == "index.md" {
			return DocsSplitResult{}, docRefErrorf(DocCodeInvalidInput,
				"dokument %q — erwartet <NN>-<slug>.md, klein, nur a–z, 0–9, ., _ und -, keine README.md und keine index.md", doc.Dokument)
		}
		if seen[name] {
			return DocsSplitResult{}, docRefErrorf(DocCodeInvalidInput, "dokument %q steht mehrfach in der Eingabe", name)
		}
		seen[name] = true
		action, err := normalizeDocsAction(doc.Aktion)
		if err != nil {
			return DocsSplitResult{}, fmt.Errorf("%s: %w", name, err)
		}
		if doc.Abgelehnt && action == DocsActionRewrite {
			return DocsSplitResult{}, docRefErrorf(DocCodeInvalidInput, "%s: umschreiben bestätigt der Nutzer nicht, abgelehnt gibt es nur für neu und entfaellt", name)
		}
		pfade, err := cleanDocsOrderPaths(doc.Pfade)
		if err != nil {
			return DocsSplitResult{}, fmt.Errorf("%s: %w", name, err)
		}
		if len(pfade) == 0 {
			return DocsSplitResult{}, docRefErrorf(DocCodeInvalidInput, "%s: kein Pfad angegeben", name)
		}
		var lines []DocsOrderLine
		for _, line := range doc.Anlaesse {
			text := oneLine(line.Anlass)
			if text == "" {
				continue
			}
			datum := strings.TrimSpace(line.Datum)
			if datum != "" {
				if _, err := time.Parse("2006-01-02", datum); err != nil {
					return DocsSplitResult{}, docRefErrorf(DocCodeInvalidInput, "%s: datum %q — erwartet JJJJ-MM-TT", name, line.Datum)
				}
			}
			lines = append(lines, DocsOrderLine{Datum: datum, Anlass: text})
		}
		anlass := oneLine(doc.Anlass)
		if anlass == "" {
			parts := []string{}
			for _, line := range lines {
				if !containsString(parts, line.Anlass) {
					parts = append(parts, line.Anlass)
				}
			}
			anlass = strings.Join(parts, "; ")
		}
		if anlass == "" {
			return DocsSplitResult{}, docRefErrorf(DocCodeInvalidInput, "%s: kein Anlass angegeben (anlass oder anlaesse)", name)
		}
		entsteht := strings.TrimSpace(doc.Entstanden)
		if entsteht != "" {
			if _, err := time.Parse("2006-01-02", entsteht); err != nil {
				return DocsSplitResult{}, docRefErrorf(DocCodeInvalidInput, "%s: entstanden %q — erwartet JJJJ-MM-TT", name, doc.Entstanden)
			}
		}
		if entsteht == "" {
			for _, line := range lines {
				if line.Datum != "" && (entsteht == "" || line.Datum < entsteht) {
					entsteht = line.Datum
				}
			}
		}
		if entsteht == "" {
			entsteht = date
		}
		prepared = append(prepared, docsSplitPrepared{input: doc, aktion: action, pfade: pfade, lines: lines, anlass: anlass, entsteht: entsteht})
	}

	raws := make([]string, 0, len(in.Raw))
	for _, raw := range in.Raw {
		ref, err := ParseDocRef(strings.TrimSpace(raw))
		if err != nil {
			return DocsSplitResult{}, err
		}
		prefix := docsOrdersDir + "/"
		if ref.Scheme != DocSchemeKeph || ref.Dir || !strings.HasPrefix(ref.Path, prefix) ||
			strings.HasPrefix(ref.Path, prefix+docDoneDir+"/") || strings.Contains(strings.TrimPrefix(ref.Path, prefix), "/") {
			return DocsSplitResult{}, docRefErrorf(DocCodeInvalidInput, "%s: erwartet ein offener Auftrag keph:/%s<datei>", raw, prefix)
		}
		raws = append(raws, DocSchemeKeph+":/"+ref.Path)
	}
	open, _, err := o.load(ctx)
	if err != nil {
		return DocsSplitResult{}, err
	}
	for _, ref := range raws {
		for _, order := range open {
			if order.Ref == ref && (order.Kind != DocsOrderKindRaw || order.Producer != DocsOrderCode) {
				return DocsSplitResult{}, docRefErrorf(DocCodeInvalidInput, "%s ist kein roher Auftrag für code und geht in keinem Auftrag auf", ref)
			}
		}
	}

	// Ein wartender roher Auftrag nimmt an nichts teil; nur ein voller Lauf
	// (Auftrag über .) unter den rohen der Eingabe nimmt ihn auf.
	var waiting []string
	full := false
	at := now()
	for _, ref := range raws {
		for _, order := range open {
			if order.Ref != ref {
				continue
			}
			if len(order.Pfade) == 1 && order.Pfade[0] == "." {
				full = true
			}
			if age, ok := docsOrderAge(order.Updated, at); ok && age < DocsOrderWait {
				waiting = append(waiting, ref)
			}
		}
	}
	if len(waiting) > 0 && !full {
		return DocsSplitResult{}, docRefErrorf(DocCodeInvalidInput,
			"%s wartet noch (letzte Änderung jünger als %d Minuten) und geht in keinem vollen Lauf auf — nichts geschrieben",
			strings.Join(waiting, ", "), int(DocsOrderWait/time.Minute))
	}

	result := DocsSplitResult{Documents: []DocsSplitDocResult{}, Raw: []DocsSplitRawResult{}, ClosedRaw: []string{}}
	failed := false
	for _, item := range prepared {
		doc := o.writeSplit(ctx, item, sitzung)
		if doc.Status == DocsSplitFailed {
			failed = true
		} else if item.input.Abgelehnt && doc.Ref != "" {
			// Ein abgelehntes neu oder entfaellt steht als Auftrag in der Queue
			// und wird gleich geschlossen, ohne publish.
			moved, err := o.docs.Done(ctx, doc.Ref)
			switch {
			case err == nil:
				doc.Closed, doc.DoneRef = true, moved.Ref
			case isDocsGone(err):
				doc.Closed, doc.Message = true, "schon von einem anderen Lauf abgeschlossen"
			default:
				doc.Code, doc.Message = docsErrorCode(err), "nicht geschlossen: "+err.Error()
			}
		}
		result.Documents = append(result.Documents, doc)
	}

	for _, ref := range raws {
		raw := DocsSplitRawResult{Ref: ref}
		if failed {
			raw.Status = DocsRawKept
			raw.Message = "ein zerlegter Auftrag wurde nicht geschrieben; der nächste Lauf zerlegt erneut und ergänzt, was schon dasteht"
			result.Raw = append(result.Raw, raw)
			continue
		}
		moved, err := o.docs.Done(ctx, ref)
		switch {
		case err == nil:
			raw.Status, raw.DoneRef = DocsRawClosed, moved.Ref
			result.ClosedRaw = append(result.ClosedRaw, ref)
		case isDocsGone(err):
			raw.Status, raw.Message = DocsRawGone, "ein anderer Lauf hat ihn schon abgeschlossen"
		default:
			raw.Status, raw.Code, raw.Message = DocsSplitFailed, docsErrorCode(err), err.Error()
		}
		result.Raw = append(result.Raw, raw)
	}
	return result, nil
}

// isDocsGone sagt, ob ein Verschieben mit wrong_dir oder not_found endete:
// Ein anderer Lauf hat den Auftrag schon abgeschlossen.
func isDocsGone(err error) bool {
	code := DocRefErrorCode(err)
	return code == DocCodeWrongDir || code == DocCodeNotFound
}

// docsErrorCode ist der Code eines Fehlers, sonst failed.
func docsErrorCode(err error) string {
	if code := DocRefErrorCode(err); code != "" {
		return code
	}
	return "failed"
}

// writeSplit legt den zerlegten Auftrag für ein Dokument an oder ergänzt den
// offenen.
func (o *DocsOrders) writeSplit(ctx context.Context, item docsSplitPrepared, sitzung string) DocsSplitDocResult {
	name := strings.TrimSpace(item.input.Dokument)
	doc := DocsSplitDocResult{Dokument: name, Aktion: item.aktion}
	fail := func(err error) DocsSplitDocResult {
		doc.Status, doc.Code, doc.Message = DocsSplitFailed, docsErrorCode(err), err.Error()
		return doc
	}
	for attempt := 0; attempt < docsOrderAttempts; attempt++ {
		open, _, err := o.load(ctx)
		if err != nil {
			return fail(err)
		}
		var existing *DocsOrder
		for i := range open {
			if open[i].Producer == DocsOrderCode && open[i].Kind == DocsOrderKindSplit && open[i].Dokument == name {
				existing = &open[i]
				break
			}
		}
		if existing == nil {
			created, err := o.createSplit(ctx, item, name, sitzung)
			if err != nil {
				return fail(err)
			}
			doc.Status, doc.Ref, doc.Revision = DocsOrderCreated, created.ref, created.revision
			return doc
		}
		doc.Ref = existing.Ref
		merged := append([]string{}, existing.Pfade...)
		for _, p := range item.pfade {
			if !containsString(merged, p) {
				merged = append(merged, p)
			}
		}
		if containsString(merged, ".") {
			merged = []string{"."}
		}
		var newLines []DocsOrderLine
		for _, line := range item.lines {
			if !containsDocsLine(existing.Lines, line) && !containsDocsLine(newLines, line) {
				newLines = append(newLines, line)
			}
		}
		entsteht := existing.Entstanden
		if entsteht == "" || item.entsteht < entsteht {
			entsteht = item.entsteht
		}
		if len(merged) == len(existing.Pfade) && sameStrings(merged, existing.Pfade) && len(newLines) == 0 &&
			existing.Aktion == item.aktion && entsteht == existing.Entstanden {
			doc.Status, doc.Revision, doc.Reason = "skipped", existing.Revision, DocsReasonNothingToAdd
			return doc
		}
		storeName := strings.TrimPrefix(existing.Ref, DocSchemeKeph+":/")
		read, err := o.docs.store.Read(ctx, storeName)
		if err != nil {
			return fail(kephError(err))
		}
		if read.Kind != kephalaion.ReadKindDocument || read.Content == nil {
			continue
		}
		content := spliceDocsOrderPaths(*read.Content, merged)
		content = setDocsOrderField(content, "aktion", item.aktion)
		content = setDocsOrderField(content, "entstanden", entsteht)
		for _, line := range newLines {
			content = appendDocsOrderLine(content, renderDocsOrderLine(line))
		}
		written, err := o.docs.store.Write(ctx, storeName, content, read.Revision)
		if err == nil {
			doc.Status, doc.Revision = DocsOrderExtended, written.Revision
			return doc
		}
		if !errIsStale(err) {
			return fail(kephError(err))
		}
	}
	return fail(docRefErrorf(DocCodeStaleRevision, "der Auftrag für %s wurde %d Mal nacheinander geändert — noch einmal versuchen", name, docsOrderAttempts))
}

type docsSplitCreated struct {
	ref      string
	revision int64
}

// createSplit legt den zerlegten Auftrag mit Kopf und Rumpf aus
// doku-auftraege.md an.
func (o *DocsOrders) createSplit(ctx context.Context, item docsSplitPrepared, name, sitzung string) (docsSplitCreated, error) {
	short, err := NormalizeDocName("code-" + strings.TrimSuffix(name, ".md"))
	if err != nil {
		return docsSplitCreated{}, err
	}
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "title: %s\n", yamlQuoted(strings.TrimSuffix(name, ".md")+" "+item.aktion))
	fmt.Fprintf(&b, "erzeuger: %s\n", DocsOrderCode)
	fmt.Fprintf(&b, "dokument: %s\n", name)
	fmt.Fprintf(&b, "aktion: %s\n", item.aktion)
	b.WriteString(renderDocsOrderPaths(item.pfade))
	fmt.Fprintf(&b, "anlass: %s\n", yamlQuoted(item.anlass))
	fmt.Fprintf(&b, "sitzung: %s\n", yamlHeaderValue(sitzung))
	fmt.Fprintf(&b, "entstanden: %s\n", item.entsteht)
	b.WriteString("---\n")
	if len(item.lines) > 0 {
		b.WriteString("\n")
		for _, line := range item.lines {
			b.WriteString(renderDocsOrderLine(line) + "\n")
		}
	}
	kind, _ := o.docs.anyKind("docs")
	created, doc, err := o.docs.createNumbered(ctx, kind, kind.dirFor(o.docs.user), short, b.String(), kephError)
	if err != nil {
		return docsSplitCreated{}, err
	}
	return docsSplitCreated{ref: created.Ref, revision: doc.Revision}, nil
}

// renderDocsOrderLine ist die Zeile des Rumpfs: Datum — Anlass.
func renderDocsOrderLine(line DocsOrderLine) string {
	if line.Datum == "" {
		return line.Anlass
	}
	return line.Datum + " — " + line.Anlass
}

// setDocsOrderField setzt ein einzeiliges Feld im Kopf: ersetzt die Zeile,
// oder fügt sie vor der schließenden Trennzeile ein. Alles andere bleibt.
func setDocsOrderField(content, key, value string) string {
	lines := strings.Split(content, "\n")
	_, end, ok := frontmatterBounds(lines)
	if !ok {
		return content
	}
	line := key + ": " + value
	for i := 1; i < end; i++ {
		if strings.HasPrefix(lines[i], key+":") {
			if lines[i] == line {
				return content
			}
			lines[i] = line
			return strings.Join(lines, "\n")
		}
	}
	out := append([]string{}, lines[:end]...)
	out = append(out, line)
	return strings.Join(append(out, lines[end:]...), "\n")
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
